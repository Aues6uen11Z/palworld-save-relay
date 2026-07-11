package main

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"palworld-save-relay/internal/config"
	"palworld-save-relay/internal/palworld"
	"palworld-save-relay/internal/storage"
)

// App is the Wails service exposing relay operations to the frontend.
type App struct {
	cfg *config.Config
}

// NewApp creates the App service, loading persisted config.
func NewApp() *App {
	cfg, err := config.Load()
	if err != nil {
		cfg = config.Defaults()
	}
	return &App{cfg: cfg}
}

// ---------- config ----------

// GetConfig returns the current config.
func (a *App) GetConfig() (*config.Config, error) {
	return a.cfg, nil
}

// SaveConfig updates and persists the config.
func (a *App) SaveConfig(c *config.Config) error {
	if err := config.Save(c); err != nil {
		return err
	}
	a.cfg = c
	return nil
}

// ---------- detection ----------

// World is a world save with display metadata (alias/hidden) merged from config.
type World struct {
	palworld.World
	Alias  string `json:"alias"`
	Hidden bool   `json:"hidden"`
}

// DetectWorlds lists local world saves under the configured (or auto) root.
func (a *App) DetectWorlds() ([]World, error) {
	root := a.cfg.SaveRoot
	if root == "" {
		r, err := palworld.SaveRoot()
		if err != nil {
			return nil, err
		}
		root = r
	}
	ws, err := palworld.ListWorlds(root)
	if err != nil {
		return nil, err
	}
	out := make([]World, 0, len(ws))
	for _, w := range ws {
		out = append(out, World{
			World:  w,
			Alias:  a.cfg.WorldAliases[w.GUID],
			Hidden: a.cfg.HiddenWorlds[w.GUID],
		})
	}
	return out, nil
}

// SetWorldMeta sets a world's alias and hidden flag (persisted).
func (a *App) SetWorldMeta(guid, alias string, hidden bool) error {
	if a.cfg.WorldAliases == nil {
		a.cfg.WorldAliases = map[string]string{}
	}
	if a.cfg.HiddenWorlds == nil {
		a.cfg.HiddenWorlds = map[string]bool{}
	}
	if alias != "" {
		a.cfg.WorldAliases[guid] = alias
	} else {
		delete(a.cfg.WorldAliases, guid)
	}
	if hidden {
		a.cfg.HiddenWorlds[guid] = true
	} else {
		delete(a.cfg.HiddenWorlds, guid)
	}
	return config.Save(a.cfg)
}

// ListPlayers returns the players in a world (host flagged).
func (a *App) ListPlayers(worldPath string) ([]palworld.Player, error) {
	return palworld.ListPlayers(worldPath)
}

// LocalSteamID returns the local player's SteamID64 (from the save folder).
func (a *App) LocalSteamID() (uint64, error) {
	root := a.cfg.SaveRoot
	if root == "" {
		r, err := palworld.SaveRoot()
		if err != nil {
			return 0, err
		}
		root = r
	}
	return palworld.LocalSteamID(root)
}

// ---------- host conversion ----------

// PrepareUpload converts the host (0000...0001) to the local player's real UID,
// producing the cloud intermediate. Call before uploading.
func (a *App) PrepareUpload(worldPath string) error {
	sid, err := a.LocalSteamID()
	if err != nil {
		return err
	}
	return palworld.ConvertHost(worldPath, palworld.HostUUID, palworld.SteamIDToPlayerUUID(sid))
}

// ActivateHost converts the local player's real UID to the host slot, making
// this machine the host. Call after downloading the intermediate.
func (a *App) ActivateHost(worldPath string) error {
	sid, err := a.LocalSteamID()
	if err != nil {
		return err
	}
	return palworld.ConvertHost(worldPath, palworld.SteamIDToPlayerUUID(sid), palworld.HostUUID)
}

// ---------- cloud ----------

func (a *App) newStorage() (storage.Storage, error) {
	q := a.cfg.Qiniu
	if q.AccessKey == "" || q.Bucket == "" {
		return nil, fmt.Errorf("qiniu config incomplete")
	}
	return storage.NewQiniu(storage.QiniuConfig{
		AccessKey: q.AccessKey, SecretKey: q.SecretKey,
		Bucket: q.Bucket, Region: q.Region, Domain: q.Domain,
	})
}

func (a *App) lockManager() (*storage.LockManager, error) {
	s, err := a.newStorage()
	if err != nil {
		return nil, err
	}
	return &storage.LockManager{Store: s, TTL: a.cfg.LockTTL}, nil
}

// UploadWorld packs and uploads the current world as a new cloud version.
func (a *App) UploadWorld(worldPath string) error {
	s, err := a.newStorage()
	if err != nil {
		return err
	}
	data, err := palworld.PackWorld(worldPath)
	if err != nil {
		return err
	}
	up := a.cfg.Uploader
	if up == "" {
		up = "player"
	}
	_, err = storage.UploadVersion(context.Background(), s, filepath.Base(worldPath), up, bytes.NewReader(data), int64(len(data)))
	return err
}

// DownloadLatest downloads the newest cloud version and writes it to worldPath
// (after backing up the current world).
func (a *App) DownloadLatest(worldPath string) error {
	return a.DownloadVersion(worldPath, "")
}

// DownloadVersion downloads a specific version (or the latest if key is empty).
func (a *App) DownloadVersion(worldPath, key string) error {
	s, err := a.newStorage()
	if err != nil {
		return err
	}
	guid := filepath.Base(worldPath)
	if key == "" {
		key, err = storage.LatestVersion(context.Background(), s, guid)
		if err != nil {
			return err
		}
		if key == "" {
			return fmt.Errorf("no cloud versions for world %s", guid)
		}
	}
	if _, err := palworld.BackupWorld(worldPath); err != nil {
		return err
	}
	var buf bytes.Buffer
	if err := s.Download(context.Background(), key, &buf, nil); err != nil {
		return err
	}
	return palworld.UnpackWorld(buf.Bytes(), worldPath)
}

// ListVersions returns the cloud version history for a world.
func (a *App) ListVersions(worldGUID string) ([]storage.Object, error) {
	s, err := a.newStorage()
	if err != nil {
		return nil, err
	}
	return storage.ListVersions(context.Background(), s, worldGUID)
}

// LockStatus reports the play lock for a world.
func (a *App) LockStatus(worldGUID string) (storage.LockStatus, error) {
	lm, err := a.lockManager()
	if err != nil {
		return storage.LockStatus{}, err
	}
	return lm.Status(context.Background(), worldGUID)
}

// AcquireLock claims the play lock.
func (a *App) AcquireLock(worldGUID, player string) error {
	lm, err := a.lockManager()
	if err != nil {
		return err
	}
	return lm.Acquire(context.Background(), worldGUID, player)
}

// ReleaseLock releases the play lock.
func (a *App) ReleaseLock(worldGUID string) error {
	lm, err := a.lockManager()
	if err != nil {
		return err
	}
	return lm.Release(context.Background(), worldGUID)
}

// ---------- backups ----------

// BackupRecord is a local backup entry.
type BackupRecord struct {
	Name string    `json:"name"`
	Size int64     `json:"size"`
	Time time.Time `json:"time"`
}

// ListBackups returns local backups for a world.
func (a *App) ListBackups(worldPath string) ([]BackupRecord, error) {
	dir, err := palworld.BackupDir()
	if err != nil {
		return nil, err
	}
	guid := filepath.Base(worldPath)
	bdir := filepath.Join(dir, guid)
	entries, err := os.ReadDir(bdir)
	if err != nil {
		if os.IsNotExist(err) {
			return []BackupRecord{}, nil
		}
		return nil, err
	}
	var out []BackupRecord
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".zip" {
			continue
		}
		info, _ := e.Info()
		out = append(out, BackupRecord{Name: e.Name(), Size: info.Size(), Time: info.ModTime()})
	}
	return out, nil
}

// RestoreBackup restores a backup (overwrites the current world after another backup).
func (a *App) RestoreBackup(worldPath, name string) error {
	dir, err := palworld.BackupDir()
	if err != nil {
		return err
	}
	data, err := os.ReadFile(filepath.Join(dir, filepath.Base(worldPath), name))
	if err != nil {
		return err
	}
	if _, err := palworld.BackupWorld(worldPath); err != nil {
		return err
	}
	return palworld.UnpackWorld(data, worldPath)
}

// ---------- import/export ----------

// ExportWorld packs a world to a single .palrelay.zip file.
func (a *App) ExportWorld(worldPath, outPath string) error {
	data, err := palworld.PackWorld(worldPath)
	if err != nil {
		return err
	}
	return os.WriteFile(outPath, data, 0o644)
}

// ImportWorld unpacks a .palrelay.zip into worldPath (after backup).
func (a *App) ImportWorld(zipPath, worldPath string) error {
	data, err := os.ReadFile(zipPath)
	if err != nil {
		return err
	}
	if _, err := palworld.BackupWorld(worldPath); err != nil {
		return err
	}
	return palworld.UnpackWorld(data, worldPath)
}
