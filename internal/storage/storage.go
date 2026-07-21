// Package storage abstracts the cloud object storage used for save relay
// (Qiniu Kodo by default; the interface allows OSS/COS backends later).
package storage

import (
	"context"
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"
	"time"

	"palworld-save-relay/internal/logger"
)

// Object is a cloud object listing entry.
type Object struct {
	Key          string
	Size         int64
	LastModified time.Time
	Uploader     string // parsed from the key, if it follows the version scheme
}

// Storage is the cloud object storage backend interface.
type Storage interface {
	// Upload stores size bytes from r under key (resumable where supported).
	Upload(ctx context.Context, key string, r io.Reader, size int64) error
	// Download writes the object at key to w, calling prog(done,total) for progress.
	Download(ctx context.Context, key string, w io.Writer, prog func(done, total int64)) error
	// List returns objects under prefix.
	List(ctx context.Context, prefix string) ([]Object, error)
	// Delete removes the object at key.
	Delete(ctx context.Context, key string) error
	// Get reads a small object fully (used for the play lock).
	Get(ctx context.Context, key string) ([]byte, error)
	// Put writes a small object fully (used for the play lock).
	Put(ctx context.Context, key string, data []byte) error
}

// VersionKey builds a cloud key for a save version: saves/<worldGUID>/<ms>__<uploader>.zip
func VersionKey(worldGUID, uploader string, t time.Time) string {
	return fmt.Sprintf("saves/%s/%d__%s.zip", worldGUID, t.UnixMilli(), uploader)
}

// ParseVersionKey extracts the timestamp and uploader from a version key.
func ParseVersionKey(key string) (t time.Time, uploader string, ok bool) {
	base := key
	if i := strings.LastIndex(base, "/"); i >= 0 {
		base = base[i+1:]
	}
	base = strings.TrimSuffix(base, ".zip")
	parts := strings.SplitN(base, "__", 2)
	if len(parts) != 2 {
		return time.Time{}, "", false
	}
	ms, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		return time.Time{}, "", false
	}
	return time.UnixMilli(ms), parts[1], true
}

// IsVersionKey reports whether key follows the saves/<worldGUID>/<ms>__<u>.zip scheme.
func IsVersionKey(key string) bool {
	_, _, ok := ParseVersionKey(key)
	return ok
}

// ListVersions returns the save versions for a world, newest first.
func ListVersions(ctx context.Context, s Storage, worldGUID string) ([]Object, error) {
	prefix := "saves/" + worldGUID + "/"
	objs, err := s.List(ctx, prefix)
	if err != nil {
		logger.Errorf("ListVersions: world=%s list failed: %v", worldGUID, err)
		return nil, err
	}
	var out []Object
	for _, o := range objs {
		if IsVersionKey(o.Key) {
			_, up, _ := ParseVersionKey(o.Key)
			o.Uploader = up
			out = append(out, o)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		ti, _, _ := ParseVersionKey(out[i].Key)
		tj, _, _ := ParseVersionKey(out[j].Key)
		return ti.After(tj)
	})
	return out, nil
}

// LatestVersion returns the newest version key, or "" if none.
func LatestVersion(ctx context.Context, s Storage, worldGUID string) (string, error) {
	versions, err := ListVersions(ctx, s, worldGUID)
	if err != nil {
		return "", err
	}
	if len(versions) == 0 {
		return "", nil
	}
	return versions[0].Key, nil
}

// UploadVersion uploads a save zip as a new version under worldGUID.
func UploadVersion(ctx context.Context, s Storage, worldGUID, uploader string, r io.Reader, size int64) (string, error) {
	key := VersionKey(worldGUID, uploader, time.Now())
	logger.Infof("UploadVersion: world=%s uploader=%s key=%s size=%d", worldGUID, uploader, key, size)
	if err := s.Upload(ctx, key, r, size); err != nil {
		logger.Errorf("UploadVersion: world=%s key=%s upload failed: %v", worldGUID, key, err)
		return "", err
	}
	return key, nil
}
