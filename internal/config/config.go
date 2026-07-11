// Package config manages the Pal Save Relay application config (Qiniu creds,
// world aliases, hidden saves, relay preferences) stored as JSON in AppData.
package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"
)

// Config is the persisted application configuration.
type Config struct {
	Qiniu        Qiniu             `json:"qiniu"`
	Uploader     string            `json:"uploader"`      // default uploader name
	SaveRoot     string            `json:"save_root"`     // override save root (auto if empty)
	WorldAliases map[string]string `json:"world_aliases"` // worldGUID -> alias
	HiddenWorlds map[string]bool   `json:"hidden_worlds"` // worldGUID -> hidden
	BackupKeep   int               `json:"backup_keep"`   // local backups to keep (default 5)
	LockTTL      time.Duration     `json:"lock_ttl"`      // play-lock TTL (default 6h)
}

// Qiniu holds Qiniu Kodo credentials/settings.
type Qiniu struct {
	AccessKey string `json:"access_key"`
	SecretKey string `json:"secret_key"`
	Bucket    string `json:"bucket"`
	Region    string `json:"region"`
	Domain    string `json:"domain"`
}

// Path returns the config file path (%APPDATA%/PalSaveRelay/config.json).
func Path() (string, error) {
	appData := os.Getenv("APPDATA")
	if appData == "" {
		return "", errStr("APPDATA not set")
	}
	dir := filepath.Join(appData, "PalSaveRelay")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	return filepath.Join(dir, "config.json"), nil
}

// Load reads the config, returning defaults if missing.
func Load() (*Config, error) {
	p, err := Path()
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(p)
	if os.IsNotExist(err) {
		return Defaults(), nil
	}
	if err != nil {
		return nil, err
	}
	c := Defaults()
	if err := json.Unmarshal(data, c); err != nil {
		return nil, err
	}
	return c, nil
}

// Save writes the config to disk.
func Save(c *Config) error {
	p, err := Path()
	if err != nil {
		return err
	}
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(p, data, 0o644)
}

// Defaults returns the default configuration.
func Defaults() *Config {
	return &Config{
		WorldAliases: map[string]string{},
		HiddenWorlds: map[string]bool{},
		BackupKeep:   5,
		LockTTL:      6 * time.Hour,
	}
}

type errStr string

func (e errStr) Error() string { return string(e) }
