package palworld

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBackupWorld_HostLabel(t *testing.T) {
	oldAppData := os.Getenv("APPDATA")
	defer os.Setenv("APPDATA", oldAppData)
	os.Setenv("APPDATA", t.TempDir())

	worldDir := t.TempDir()
	os.MkdirAll(filepath.Join(worldDir, "Players"), 0o755)
	os.WriteFile(filepath.Join(worldDir, "Level.sav"), []byte("level"), 0o644)

	path, err := BackupWorld(worldDir)
	if err != nil {
		t.Fatalf("BackupWorld: %v", err)
	}
	if !strings.HasSuffix(filepath.Base(path), "_host.zip") {
		t.Fatalf("expected _host.zip suffix, got %s", filepath.Base(path))
	}
}

func TestBackupWorld_GuestLabel(t *testing.T) {
	oldAppData := os.Getenv("APPDATA")
	defer os.Setenv("APPDATA", oldAppData)
	os.Setenv("APPDATA", t.TempDir())

	worldDir := t.TempDir()
	// Guest-only: only LocalData.sav, no Level.sav
	os.WriteFile(filepath.Join(worldDir, "LocalData.sav"), []byte("local"), 0o644)

	path, err := BackupWorld(worldDir)
	if err != nil {
		t.Fatalf("BackupWorld: %v", err)
	}
	if !strings.HasSuffix(filepath.Base(path), "_guest.zip") {
		t.Fatalf("expected _guest.zip suffix, got %s", filepath.Base(path))
	}
}
