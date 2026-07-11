package palworld

import (
	"os"
	"path/filepath"
	"time"
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

// BackupWorld zips the world folder to backups/<worldGUID>/<timestamp>.zip and
// returns the zip path.
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
		return "", err
	}
	name := time.Now().Format("2006-01-02_150405") + ".zip"
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return "", err
	}
	return path, nil
}
