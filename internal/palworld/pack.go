package palworld

import (
	"archive/zip"
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"

	"palworld-save-relay/internal/logger"
)

// isWorldSaveFile reports whether relpath (forward-slash, relative to the
// world dir) is a legitimate Palworld world save file: a top-level Level.sav,
// LevelMeta.sav, WorldOption.sav, or LocalData.sav, or a .sav directly under
// Players/. Everything else - backup/ and world_save_bak/ dirs, stray zip
// files, nested relay packages, _-prefixed metadata - is excluded so it
// neither bloats transfers nor pollutes the target world.
func isWorldSaveFile(relpath string) bool {
	relpath = filepath.ToSlash(relpath)
	switch relpath {
	case "Level.sav", "LevelMeta.sav", "WorldOption.sav", "LocalData.sav":
		return true
	}
	if strings.HasPrefix(relpath, "Players/") {
		rest := relpath[len("Players/"):]
		// Flat .sav only (uid.sav / uid_dps.sav); no subfolders.
		return !strings.Contains(rest, "/") && filepath.Ext(rest) == ".sav"
	}
	return false
}

// PackWorld zips a world save folder (excluding the game's own backup/ subdir)
// into a byte slice. Used by cloud sync and import/export.
func PackWorld(worldDir string) ([]byte, error) {
	guid := filepath.Base(worldDir)
	tmp, err := os.CreateTemp("", "palrelay-pack-*.zip")
	if err != nil {
		return nil, err
	}
	defer os.Remove(tmp.Name())
	zw := zip.NewWriter(tmp)
	count := 0

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
		// Whitelist: pack only world save files + relay metadata (_-prefixed).
		// Strays (world_save_bak/, nested zips, etc.) never enter a transfer.
		rel = filepath.ToSlash(rel)
		if !isWorldSaveFile(rel) && !strings.HasPrefix(rel, "_") {
			return nil
		}
		w, err := zw.Create(rel)
		if err != nil {
			return err
		}
		f, err := os.Open(path)
		if err != nil {
			return err
		}
		defer f.Close()
		if _, err := io.Copy(w, f); err != nil {
			return err
		}
		count++
		return nil
	})
	if err != nil {
		zw.Close()
		logger.Errorf("PackWorld: world=%s walk failed: %v", guid, err)
		return nil, err
	}
	if err := zw.Close(); err != nil {
		return nil, err
	}
	if err := tmp.Close(); err != nil {
		return nil, err
	}
	data, err := os.ReadFile(tmp.Name())
	if err != nil {
		return nil, err
	}
	logger.Infof("PackWorld: world=%s files=%d -> %d bytes", guid, count, len(data))
	return data, nil
}

// UnpackWorld extracts a world zip into destDir (created if needed).
func UnpackWorld(zipBytes []byte, destDir string) error {
	guid := filepath.Base(destDir)
	if err := os.MkdirAll(destDir, 0o755); err != nil {
		return err
	}
	zr, err := zip.NewReader(bytes.NewReader(zipBytes), int64(len(zipBytes)))
	if err != nil {
		logger.Errorf("UnpackWorld: world=%s zip read failed: %v", guid, err)
		return err
	}
	count := 0
	for _, f := range zr.File {
		name := filepath.ToSlash(f.Name)
		// Whitelist: unpack only world save files. Relay metadata (_-prefixed)
		// and strays (world_save_bak/, nested zips) are skipped.
		if !isWorldSaveFile(name) {
			continue
		}
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
		count++
	}
	logger.Infof("UnpackWorld: world=%s files=%d (%d bytes)", guid, count, len(zipBytes))
	return nil
}

func bytesReader(b []byte) *bytesReaderImpl { return &bytesReaderImpl{b: b} }

type bytesReaderImpl struct {
	b []byte
	i int
}

func (r *bytesReaderImpl) Read(p []byte) (int, error) {
	if r.i >= len(r.b) {
		return 0, io.EOF
	}
	n := copy(p, r.b[r.i:])
	r.i += n
	return n, nil
}

// ReadFileFromZip reads a single file from a zip byte slice. Returns the file
// data and true if found, or nil and false if not.
func ReadFileFromZip(zipBytes []byte, name string) ([]byte, bool) {
	zr, err := zip.NewReader(bytes.NewReader(zipBytes), int64(len(zipBytes)))
	if err != nil {
		return nil, false
	}
	for _, f := range zr.File {
		if filepath.ToSlash(f.Name) == name {
			rc, err := f.Open()
			if err != nil {
				return nil, false
			}
			defer rc.Close()
			data, err := io.ReadAll(rc)
			if err != nil {
				return nil, false
			}
			return data, true
		}
	}
	return nil, false
}

// ListZipFiles returns the names of all files in a zip byte slice.
func ListZipFiles(zipBytes []byte) []string {
	zr, err := zip.NewReader(bytes.NewReader(zipBytes), int64(len(zipBytes)))
	if err != nil {
		return nil
	}
	var names []string
	for _, f := range zr.File {
		names = append(names, filepath.ToSlash(f.Name))
	}
	return names
}
