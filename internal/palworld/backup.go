package palworld

import (
	"os"
	"path/filepath"
	"sort"
	"time"

	"palworld-save-relay/internal/logger"
)

// BackupDir returns the app backup root (%APPDATA%/PalSaveRelay/backups).
func BackupDir() (string, error) {
	appData := os.Getenv("APPDATA")
	if appData == "" {
		return "", errNoAppData
	}
	dir := filepath.Join(appData, "PalSaveRelay", "backups")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	return dir, nil
}

var errNoAppData = errStr("APPDATA not set")

type errStr string

func (e errStr) Error() string { return string(e) }

// BackupWorld zips the world folder to backups/<worldGUID>/<timestamp>_<role>.zip
// and returns the zip path. The role suffix ("host" or "guest") records whether
// the world was a full host save (Level.sav present) or a guest-only folder at
// backup time, so the user can tell at a glance what each backup contains.
func BackupWorld(worldDir string) (string, error) {
	guid := filepath.Base(worldDir)
	root, err := BackupDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(root, guid)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	data, err := PackWorld(worldDir)
	if err != nil {
		logger.Errorf("BackupWorld: world=%s pack failed: %v", guid, err)
		return "", err
	}
	role := "guest"
	if fileExists(filepath.Join(worldDir, "Level.sav")) {
		role = "host"
	}
	name := time.Now().Format("2006-01-02_150405") + "_" + role + ".zip"
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, data, 0o644); err != nil {
		logger.Errorf("BackupWorld: world=%s write failed: %v", guid, err)
		return "", err
	}
	logger.Infof("BackupWorld: world=%s -> %s (%d bytes, %s)", guid, path, len(data), role)
	return path, nil
}

// PruneBackups removes old backups for a world, keeping only the most recent
// `keep` .zip files (sorted by modification time, newest first). A keep value
// of 0 or less disables pruning (all backups are retained).
func PruneBackups(worldDir string, keep int) error {
	if keep <= 0 {
		return nil
	}
	guid := filepath.Base(worldDir)
	root, err := BackupDir()
	if err != nil {
		return err
	}
	dir := filepath.Join(root, guid)
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	var zips []os.DirEntry
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".zip" {
			continue
		}
		zips = append(zips, e)
	}
	if len(zips) <= keep {
		return nil
	}
	// Sort newest first by modification time.
	sort.Slice(zips, func(i, j int) bool {
		fi, _ := zips[i].Info()
		fj, _ := zips[j].Info()
		return fi.ModTime().After(fj.ModTime())
	})
	for i := keep; i < len(zips); i++ {
		path := filepath.Join(dir, zips[i].Name())
		if err := os.Remove(path); err != nil {
			logger.Warnf("PruneBackups: remove %s failed: %v", path, err)
		} else {
			logger.Infof("PruneBackups: removed old backup %s", zips[i].Name())
		}
	}
	return nil
}
