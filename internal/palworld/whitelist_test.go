package palworld

import (
	"archive/zip"
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestIsWorldSaveFile(t *testing.T) {
	cases := []struct {
		path string
		want bool
	}{
		{"Level.sav", true},
		{"LevelMeta.sav", true},
		{"WorldOption.sav", true},
		{"LocalData.sav", true},
		{"Players/ABCDEF0123456789.sav", true},
		{"Players/ABCDEF0123456789_dps.sav", true},
		{"backup/Level.sav", false},
		{"world_save_bak/Level.sav", false},
		{"world_save_bak/LevelMeta.sav", false},
		{"stray.zip", false},
		{"some/relay.zip", false},
		{"Players/sub/x.sav", false},
		{"_relay_log.jsonl", false},
		{"Unknown.sav", false},
		{"Players/ABCDEF.txt", false},
		{"", false},
	}
	for _, c := range cases {
		if got := IsWorldSaveFile(c.path); got != c.want {
			t.Errorf("IsWorldSaveFile(%q) = %v, want %v", c.path, got, c.want)
		}
	}
}

func TestPackWorld_ExcludesStrays(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "Players"), 0o755)
	os.MkdirAll(filepath.Join(dir, "world_save_bak"), 0o755)
	// Legit world save files.
	os.WriteFile(filepath.Join(dir, "Level.sav"), []byte("level"), 0o644)
	os.WriteFile(filepath.Join(dir, "LevelMeta.sav"), []byte("meta"), 0o644)
	os.WriteFile(filepath.Join(dir, "WorldOption.sav"), []byte("opt"), 0o644)
	os.WriteFile(filepath.Join(dir, "LocalData.sav"), []byte("local"), 0o644)
	os.WriteFile(filepath.Join(dir, "Players", "ABCDEF0123456789.sav"), []byte("p"), 0o644)
	os.WriteFile(filepath.Join(dir, "Players", "ABCDEF0123456789_dps.sav"), []byte("dps"), 0o644)
	// Strays that must NOT be packed.
	os.WriteFile(filepath.Join(dir, "world_save_bak", "Level.sav"), []byte("bak-level"), 0o644)
	os.WriteFile(filepath.Join(dir, "world_save_bak", "LevelMeta.sav"), []byte("bak-meta"), 0o644)
	os.WriteFile(filepath.Join(dir, "stray.zip"), []byte("PK"), 0o644)

	zipBytes, err := PackWorld(dir)
	if err != nil {
		t.Fatalf("PackWorld: %v", err)
	}
	have := map[string]bool{}
	for _, n := range ListZipFiles(zipBytes) {
		have[filepath.ToSlash(n)] = true
	}
	for _, want := range []string{"Level.sav", "LevelMeta.sav", "WorldOption.sav", "LocalData.sav", "Players/ABCDEF0123456789.sav", "Players/ABCDEF0123456789_dps.sav"} {
		if !have[want] {
			t.Errorf("expected %s in zip; got %v", want, have)
		}
	}
	for _, bad := range []string{"world_save_bak/Level.sav", "world_save_bak/LevelMeta.sav", "stray.zip"} {
		if have[bad] {
			t.Errorf("stray %s must not be packed", bad)
		}
	}
}

func TestUnpackWorld_SkipsStrays(t *testing.T) {
	// Build a raw zip that includes strays (bypassing PackWorld's filter) to
	// exercise UnpackWorld's own whitelist.
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	files := map[string]string{
		"Level.sav":                "level",
		"Players/ABCDEF0123456789.sav": "p",
		"world_save_bak/Level.sav": "bak",
		"stray.zip":                "PK",
		"_relay_log.jsonl":         "{}",
		"Unknown.sav":              "x",
	}
	for name, content := range files {
		w, err := zw.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		w.Write([]byte(content))
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	dst := t.TempDir()
	if err := UnpackWorld(buf.Bytes(), dst); err != nil {
		t.Fatalf("UnpackWorld: %v", err)
	}
	for _, f := range []string{"Level.sav", "Players/ABCDEF0123456789.sav"} {
		if _, err := os.Stat(filepath.Join(dst, filepath.FromSlash(f))); err != nil {
			t.Errorf("expected %s unpacked: %v", f, err)
		}
	}
	for _, f := range []string{"world_save_bak/Level.sav", "stray.zip", "_relay_log.jsonl", "Unknown.sav"} {
		if _, err := os.Stat(filepath.Join(dst, filepath.FromSlash(f))); err == nil {
			t.Errorf("stray %s must not be unpacked", f)
		}
	}
}
