package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadSaveRoundTrip(t *testing.T) {
	dir, err := os.MkdirTemp("", "palrelay-cfg-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)
	os.Setenv("APPDATA", dir)
	defer os.Unsetenv("APPDATA")

	c := Defaults()
	c.Uploader = "tester"
	c.Qiniu = Qiniu{AccessKey: "ak", SecretKey: "sk", Bucket: "bkt", Region: "z0"}
	c.WorldAliases["W1"] = "My World"
	c.HiddenWorlds["W2"] = true

	if err := Save(c); err != nil {
		t.Fatalf("Save: %v", err)
	}

	p, _ := Path()
	if filepath.Base(p) != "config.json" {
		t.Errorf("path = %s", p)
	}
	if _, err := os.Stat(p); err != nil {
		t.Fatalf("config not written: %v", err)
	}

	got, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got.Uploader != "tester" || got.Qiniu.Bucket != "bkt" || got.WorldAliases["W1"] != "My World" {
		t.Errorf("round-trip mismatch: %+v", got)
	}
	if got.BackupKeep != 5 {
		t.Errorf("BackupKeep = %d", got.BackupKeep)
	}
}

func TestLoadMissingReturnsDefaults(t *testing.T) {
	dir, _ := os.MkdirTemp("", "palrelay-cfg-*")
	defer os.RemoveAll(dir)
	os.Setenv("APPDATA", dir)
	c, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if c.BackupKeep != 5 || c.WorldAliases == nil {
		t.Errorf("defaults wrong: %+v", c)
	}
}
