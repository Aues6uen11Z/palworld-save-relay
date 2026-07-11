package storage

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/qiniu/go-sdk/v7/auth"
	"github.com/qiniu/go-sdk/v7/storage"
)

// QiniuConfig holds Qiniu Kodo credentials and bucket settings.
type QiniuConfig struct {
	AccessKey string
	SecretKey string
	Bucket    string
	Region    string // huadong/z0, huabei/z1, huanan/z2, beimei/na0
	Domain    string // download domain (auto-detected if empty)
}

// Qiniu is a Storage backed by Qiniu Kodo.
type Qiniu struct {
	cfg      QiniuConfig
	mac      *auth.Credentials
	bm       *storage.BucketManager
	upCfg    storage.Config
	domain   string
	uploader *storage.ResumeUploader
	formUp   *storage.FormUploader
	client   *http.Client
}

// NewQiniu creates a Qiniu Storage, auto-detecting the download domain if unset.
func NewQiniu(c QiniuConfig) (*Qiniu, error) {
	if c.AccessKey == "" || c.SecretKey == "" || c.Bucket == "" {
		return nil, errors.New("storage: qiniu AccessKey/SecretKey/Bucket required")
	}
	mac := auth.New(c.AccessKey, c.SecretKey)
	upCfg := storage.Config{UseHTTPS: true, Zone: zoneFor(c.Region)}
	bm := storage.NewBucketManager(mac, &upCfg)

	q := &Qiniu{
		cfg: c, mac: mac, bm: bm, upCfg: upCfg,
		uploader: storage.NewResumeUploader(&upCfg),
		formUp:   storage.NewFormUploader(&upCfg),
		client:   &http.Client{Timeout: 30 * time.Minute},
	}
	if c.Domain != "" {
		q.domain = c.Domain
	} else if doms, err := bm.ListBucketDomains(c.Bucket); err == nil && len(doms) > 0 {
		q.domain = doms[0].Domain
	}
	if q.domain == "" {
		return nil, errors.New("storage: no download domain for bucket; bind one or set Domain")
	}
	return q, nil
}

func zoneFor(region string) *storage.Zone {
	switch strings.ToLower(region) {
	case "z0", "huadong", "east", "华东":
		return &storage.ZoneHuadong
	case "z1", "huabei", "north", "华北":
		return &storage.ZoneHuabei
	case "z2", "huanan", "south", "华南":
		return &storage.ZoneHuanan
	case "na0", "beimei", "us", "北美":
		return &storage.ZoneBeimei
	default:
		return nil
	}
}

func (q *Qiniu) putToken(key string) string {
	scope := q.cfg.Bucket
	if key != "" {
		scope = q.cfg.Bucket + ":" + key
	}
	pp := storage.PutPolicy{Scope: scope, Expires: 3600}
	return pp.UploadToken(q.mac)
}

// Upload stores size bytes from r under key (resumable upload; the input is
// buffered since Qiniu's resumable uploader requires io.ReaderAt).
func (q *Qiniu) Upload(ctx context.Context, key string, r io.Reader, size int64) error {
	data, err := io.ReadAll(r)
	if err != nil {
		return err
	}
	var ret storage.PutRet
	if err := q.uploader.Put(ctx, &ret, q.putToken(key), key, bytes.NewReader(data), int64(len(data)), &storage.RputExtra{}); err != nil {
		return fmt.Errorf("storage: qiniu upload: %w", err)
	}
	return nil
}

// Put writes a small object fully.
func (q *Qiniu) Put(ctx context.Context, key string, data []byte) error {
	var ret storage.PutRet
	if err := q.formUp.Put(ctx, &ret, q.putToken(key), key, bytes.NewReader(data), int64(len(data)), &storage.PutExtra{}); err != nil {
		return fmt.Errorf("storage: qiniu put: %w", err)
	}
	return nil
}

func (q *Qiniu) downloadURL(key string) string {
	deadline := time.Now().Add(1 * time.Hour).Unix()
	return storage.MakePrivateURL(q.mac, q.domain, key, deadline)
}

// Download writes the object at key to w in 2 MiB ranged chunks with progress.
func (q *Qiniu) Download(ctx context.Context, key string, w io.Writer, prog func(done, total int64)) error {
	total, err := q.size(ctx, key)
	if err != nil {
		return err
	}
	if prog != nil {
		prog(0, total)
	}
	const chunk = 2 << 20
	for off := int64(0); off < total; off += chunk {
		end := off + chunk - 1
		if end > total-1 {
			end = total - 1
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, q.downloadURL(key), nil)
		if err != nil {
			return err
		}
		req.Header.Set("Range", fmt.Sprintf("bytes=%d-%d", off, end))
		resp, err := q.client.Do(req)
		if err != nil {
			return fmt.Errorf("storage: qiniu download: %w", err)
		}
		if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusPartialContent {
			resp.Body.Close()
			if resp.StatusCode == http.StatusNotFound {
				return ErrNotFound
			}
			return fmt.Errorf("storage: qiniu download status %s", resp.Status)
		}
		n, err := io.Copy(w, resp.Body)
		resp.Body.Close()
		if err != nil {
			return err
		}
		if prog != nil {
			prog(off+n, total)
		}
	}
	return nil
}

func (q *Qiniu) size(ctx context.Context, key string) (int64, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodHead, q.downloadURL(key), nil)
	if err != nil {
		return 0, err
	}
	resp, err := q.client.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return 0, ErrNotFound
	}
	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("storage: qiniu head status %s", resp.Status)
	}
	return resp.ContentLength, nil
}

// Get reads a small object fully.
func (q *Qiniu) Get(ctx context.Context, key string) ([]byte, error) {
	var buf bytes.Buffer
	if err := q.Download(ctx, key, &buf, nil); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// List returns objects under prefix.
func (q *Qiniu) List(ctx context.Context, prefix string) ([]Object, error) {
	var out []Object
	marker := ""
	for {
		entries, _, next, hasNext, err := q.bm.ListFiles(q.cfg.Bucket, prefix, "", marker, 1000)
		if err != nil {
			return nil, fmt.Errorf("storage: qiniu list: %w", err)
		}
		for _, e := range entries {
			t := time.Unix(0, e.PutTime*100) // Qiniu PutTime: 100-ns since epoch
			out = append(out, Object{Key: e.Key, Size: e.Fsize, LastModified: t})
		}
		if !hasNext {
			break
		}
		marker = next
	}
	return out, nil
}

// Delete removes the object at key.
func (q *Qiniu) Delete(ctx context.Context, key string) error {
	if err := q.bm.Delete(q.cfg.Bucket, key); err != nil {
		s := err.Error()
		if strings.Contains(s, "no such file") || strings.Contains(s, "612") {
			return ErrNotFound
		}
		return fmt.Errorf("storage: qiniu delete: %w", err)
	}
	return nil
}
