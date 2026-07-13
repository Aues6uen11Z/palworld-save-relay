package palworld

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// ---- ValidateWorldZip ----

func TestValidateWorldZip_ValidZip(t *testing.T) {
	dir := t.TempDir()
	data := readSavFixtureX(t, "level_plm.sav")
	if len(data) == 0 {
		return // fixture skipped
	}
	if err := os.WriteFile(filepath.Join(dir, "Level.sav"), data, 0o644); err != nil {
		t.Fatal(err)
	}
	zipBytes, err := PackWorld(dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := ValidateWorldZip(zipBytes); err != nil {
		t.Fatalf("ValidateWorldZip failed on valid zip: %v", err)
	}
}

func TestValidateWorldZip_NotAZip(t *testing.T) {
	if err := ValidateWorldZip([]byte("not a zip file at all")); err == nil {
		t.Fatal("expected error for non-zip data")
	}
}

func TestValidateWorldZip_Truncated(t *testing.T) {
	dir := t.TempDir()
	data := readSavFixtureX(t, "level_plm.sav")
	if len(data) == 0 {
		return
	}
	os.WriteFile(filepath.Join(dir, "Level.sav"), data, 0o644)
	zipBytes, err := PackWorld(dir)
	if err != nil {
		t.Fatal(err)
	}
	truncated := zipBytes[:len(zipBytes)/2]
	if err := ValidateWorldZip(truncated); err == nil {
		t.Fatal("expected error for truncated zip")
	}
}

func TestValidateWorldZip_NoLevelSav(t *testing.T) {
	dir := t.TempDir()
	data := readSavFixtureX(t, "player_plm.sav")
	if len(data) == 0 {
		return
	}
	os.WriteFile(filepath.Join(dir, "someplayer.sav"), data, 0o644)
	zipBytes, err := PackWorld(dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := ValidateWorldZip(zipBytes); err == nil {
		t.Fatal("expected error for zip without Level.sav")
	}
}

// ---- ReplaceWorld (temp-first, full replace) ----

func TestReplaceWorld_CorruptZipDoesNotTouchWorld(t *testing.T) {
	worldDir := t.TempDir()
	original := []byte("original-level-data")
	if err := os.WriteFile(filepath.Join(worldDir, "Level.sav"), original, 0o644); err != nil {
		t.Fatal(err)
	}

	// Corrupt zip must fail and must NOT modify the live world.
	if err := ReplaceWorld(worldDir, []byte("not a zip")); err == nil {
		t.Fatal("expected error for corrupt zip")
	}

	got, err := os.ReadFile(filepath.Join(worldDir, "Level.sav"))
	if err != nil {
		t.Fatalf("original Level.sav missing after failed replace: %v", err)
	}
	if string(got) != string(original) {
		t.Fatalf("original Level.sav was modified: got %q, want %q", string(got), string(original))
	}
}

func TestReplaceWorld_FullReplaceRemovesOldFiles(t *testing.T) {
	worldDir := t.TempDir()
	os.MkdirAll(filepath.Join(worldDir, "Players"), 0o755)
	os.WriteFile(filepath.Join(worldDir, "LocalData.sav"), []byte("old-local"), 0o644)
	os.WriteFile(filepath.Join(worldDir, "Level.sav"), []byte("old-level"), 0o644)
	os.WriteFile(filepath.Join(worldDir, "Players", "OLDPLAYER.sav"), []byte("old-player"), 0o644)

	srcDir := t.TempDir()
	os.WriteFile(filepath.Join(srcDir, "Level.sav"), []byte("new-level"), 0o644)
	zipBytes, err := PackWorld(srcDir)
	if err != nil {
		t.Fatal(err)
	}

	if err := ReplaceWorld(worldDir, zipBytes); err != nil {
		t.Fatalf("ReplaceWorld: %v", err)
	}

	// Old Level.sav replaced.
	got, _ := os.ReadFile(filepath.Join(worldDir, "Level.sav"))
	if string(got) != "new-level" {
		t.Fatalf("Level.sav not replaced: %q", string(got))
	}
	// LocalData.sav removed (full replace does not keep it).
	if _, err := os.Stat(filepath.Join(worldDir, "LocalData.sav")); err == nil {
		t.Fatal("LocalData.sav should have been removed by full replace")
	}
	// Old player file removed (clean replace, not overlay).
	if _, err := os.Stat(filepath.Join(worldDir, "Players", "OLDPLAYER.sav")); err == nil {
		t.Fatal("OLDPLAYER.sav should have been removed (not an overlay)")
	}
}

// ---- ReplaceWorldKeepLocalData ----

func TestReplaceWorldKeepLocalData_PreservesLocalData(t *testing.T) {
	worldDir := t.TempDir()
	os.MkdirAll(filepath.Join(worldDir, "Players"), 0o755)
	os.WriteFile(filepath.Join(worldDir, "LocalData.sav"), []byte("my-personal-progress"), 0o644)
	os.WriteFile(filepath.Join(worldDir, "Level.sav"), []byte("old-level"), 0o644)
	os.WriteFile(filepath.Join(worldDir, "Players", "OLDPLAYER.sav"), []byte("old-player"), 0o644)

	// New zip has Level.sav + new player, no LocalData.sav.
	srcDir := t.TempDir()
	os.MkdirAll(filepath.Join(srcDir, "Players"), 0o755)
	os.WriteFile(filepath.Join(srcDir, "Level.sav"), []byte("new-level"), 0o644)
	os.WriteFile(filepath.Join(srcDir, "Players", "NEWPLAYER.sav"), []byte("new-player"), 0o644)
	zipBytes, err := PackWorld(srcDir)
	if err != nil {
		t.Fatal(err)
	}

	if err := ReplaceWorldKeepLocalData(worldDir, zipBytes); err != nil {
		t.Fatalf("ReplaceWorldKeepLocalData: %v", err)
	}

	// LocalData.sav preserved with original content.
	got, err := os.ReadFile(filepath.Join(worldDir, "LocalData.sav"))
	if err != nil {
		t.Fatalf("LocalData.sav should exist: %v", err)
	}
	if string(got) != "my-personal-progress" {
		t.Fatalf("LocalData.sav content changed: %q", string(got))
	}
	// Level.sav replaced.
	got, _ = os.ReadFile(filepath.Join(worldDir, "Level.sav"))
	if string(got) != "new-level" {
		t.Fatalf("Level.sav not replaced: %q", string(got))
	}
	// Old player removed (clean replace).
	if _, err := os.Stat(filepath.Join(worldDir, "Players", "OLDPLAYER.sav")); err == nil {
		t.Fatal("OLDPLAYER.sav should have been removed")
	}
	// New player exists.
	got, _ = os.ReadFile(filepath.Join(worldDir, "Players", "NEWPLAYER.sav"))
	if string(got) != "new-player" {
		t.Fatalf("NEWPLAYER.sav not written: %q", string(got))
	}
}

func TestReplaceWorldKeepLocalData_DropsLocalDataFromZip(t *testing.T) {
	worldDir := t.TempDir()
	os.WriteFile(filepath.Join(worldDir, "LocalData.sav"), []byte("keep-me"), 0o644)

	// Zip includes a LocalData.sav that must NOT overwrite the local one.
	srcDir := t.TempDir()
	os.WriteFile(filepath.Join(srcDir, "Level.sav"), []byte("level"), 0o644)
	os.WriteFile(filepath.Join(srcDir, "LocalData.sav"), []byte("from-zip-must-be-ignored"), 0o644)
	zipBytes, err := PackWorld(srcDir)
	if err != nil {
		t.Fatal(err)
	}

	if err := ReplaceWorldKeepLocalData(worldDir, zipBytes); err != nil {
		t.Fatalf("ReplaceWorldKeepLocalData: %v", err)
	}

	got, _ := os.ReadFile(filepath.Join(worldDir, "LocalData.sav"))
	if string(got) != "keep-me" {
		t.Fatalf("LocalData.sav was overwritten by zip: got %q, want %q", string(got), "keep-me")
	}
}

func TestReplaceWorldKeepLocalData_CorruptZipDoesNotTouchWorld(t *testing.T) {
	worldDir := t.TempDir()
	original := []byte("original")
	os.WriteFile(filepath.Join(worldDir, "Level.sav"), original, 0o644)
	os.WriteFile(filepath.Join(worldDir, "LocalData.sav"), []byte("local"), 0o644)

	if err := ReplaceWorldKeepLocalData(worldDir, []byte("corrupt")); err == nil {
		t.Fatal("expected error for corrupt zip")
	}

	got, _ := os.ReadFile(filepath.Join(worldDir, "Level.sav"))
	if string(got) != string(original) {
		t.Fatalf("original Level.sav was modified: %q", string(got))
	}
}

// ---- PruneBackups ----

func TestPruneBackups_KeepsNewest(t *testing.T) {
	oldAppData := os.Getenv("APPDATA")
	defer os.Setenv("APPDATA", oldAppData)
	os.Setenv("APPDATA", t.TempDir())

	guid := "TESTGUID"
	worldDir := filepath.Join(t.TempDir(), guid)
	os.MkdirAll(worldDir, 0o755)

	root, err := BackupDir()
	if err != nil {
		t.Fatal(err)
	}
	bdir := filepath.Join(root, guid)
	os.MkdirAll(bdir, 0o755)

	// Create 5 backups with strictly increasing mod times.
	for i := 0; i < 5; i++ {
		name := fmt.Sprintf("2026-01-0%d_12000%d.zip", i+1, i)
		path := filepath.Join(bdir, name)
		os.WriteFile(path, []byte("backup"), 0o644)
		mt := time.Date(2026, 1, i+1, 12, 0, i, 0, time.UTC)
		os.Chtimes(path, mt, mt)
	}

	// Keep 3.
	if err := PruneBackups(worldDir, 3); err != nil {
		t.Fatalf("PruneBackups: %v", err)
	}

	entries, _ := os.ReadDir(bdir)
	if len(entries) != 3 {
		t.Fatalf("expected 3 backups after prune, got %d", len(entries))
	}

	// The 3 newest (indices 2, 3, 4 -> Jan 3, 4, 5) should survive.
	for _, e := range entries {
		day := e.Name()[len("2026-01-0"):len("2026-01-0")+1]
		if day == "1" || day == "2" {
			t.Fatalf("oldest backup %s should have been pruned", e.Name())
		}
	}
}

func TestPruneBackups_KeepZeroIsNoop(t *testing.T) {
	oldAppData := os.Getenv("APPDATA")
	defer os.Setenv("APPDATA", oldAppData)
	os.Setenv("APPDATA", t.TempDir())

	guid := "TESTGUID2"
	worldDir := filepath.Join(t.TempDir(), guid)
	os.MkdirAll(worldDir, 0o755)

	root, _ := BackupDir()
	bdir := filepath.Join(root, guid)
	os.MkdirAll(bdir, 0o755)

	for i := 0; i < 3; i++ {
		os.WriteFile(filepath.Join(bdir, fmt.Sprintf("backup%d.zip", i)), []byte("x"), 0o644)
	}

	// keep <= 0 disables pruning.
	PruneBackups(worldDir, 0)

	entries, _ := os.ReadDir(bdir)
	if len(entries) != 3 {
		t.Fatalf("expected 3 backups (no pruning), got %d", len(entries))
	}
}

func TestPruneBackups_NoBackupsIsNoop(t *testing.T) {
	oldAppData := os.Getenv("APPDATA")
	defer os.Setenv("APPDATA", oldAppData)
	os.Setenv("APPDATA", t.TempDir())

	worldDir := filepath.Join(t.TempDir(), "NOBACKUP")
	os.MkdirAll(worldDir, 0o755)

	if err := PruneBackups(worldDir, 5); err != nil {
		t.Fatalf("PruneBackups with no backups should not error: %v", err)
	}
}
