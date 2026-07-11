package palworld

import (
	"archive/zip"
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// PackWorld zips a world save folder (excluding the game's own backup/ subdir)
// into a byte slice. Used by cloud sync and import/export.
func PackWorld(worldDir string) ([]byte, error) {
	tmp, err := os.CreateTemp("", "palrelay-pack-*.zip")
	if err != nil {
		return nil, err
	}
	defer os.Remove(tmp.Name())
	zw := zip.NewWriter(tmp)

	err = filepath.Walk(worldDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			if strings.EqualFold(filepath.Base(path), "backup") {
				return filepath.SkipDir // exclude game backups
			}
			return nil
		}
		rel, err := filepath.Rel(worldDir, path)
		if err != nil {
			return err
		}
		w, err := zw.Create(filepath.ToSlash(rel))
		if err != nil {
			return err
		}
		f, err := os.Open(path)
		if err != nil {
			return err
		}
		defer f.Close()
		_, err = io.Copy(w, f)
		return err
	})
	if err != nil {
		zw.Close()
		return nil, err
	}
	if err := zw.Close(); err != nil {
		return nil, err
	}
	if err := tmp.Close(); err != nil {
		return nil, err
	}
	return os.ReadFile(tmp.Name())
}

// UnpackWorld extracts a world zip into destDir (created if needed).
func UnpackWorld(zipBytes []byte, destDir string) error {
	if err := os.MkdirAll(destDir, 0o755); err != nil {
		return err
	}
	zr, err := zip.NewReader(bytes.NewReader(zipBytes), int64(len(zipBytes)))
	if err != nil {
		return err
	}
	for _, f := range zr.File {
		outPath := filepath.Join(destDir, filepath.FromSlash(f.Name))
		if err := os.MkdirAll(filepath.Dir(outPath), 0o755); err != nil {
			return err
		}
		rc, err := f.Open()
		if err != nil {
			return err
		}
		out, err := os.OpenFile(outPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o644)
		if err != nil {
			rc.Close()
			return err
		}
		if _, err := io.Copy(out, rc); err != nil {
			rc.Close()
			out.Close()
			return err
		}
		rc.Close()
		out.Close()
	}
	return nil
}

func bytesReader(b []byte) *bytesReaderImpl { return &bytesReaderImpl{b: b} }

type bytesReaderImpl struct{ b []byte; i int }

func (r *bytesReaderImpl) Read(p []byte) (int, error) {
	if r.i >= len(r.b) {
		return 0, io.EOF
	}
	n := copy(p, r.b[r.i:])
	r.i += n
	return n, nil
}


