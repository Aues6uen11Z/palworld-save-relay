package main

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sort"
	"time"

	"palworld-save-relay/internal/config"
	"palworld-save-relay/internal/logger"
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
		logger.Warnf("NewApp: config load failed, using defaults: %v", err)
		cfg = config.Defaults()
	} else {
		logger.Infof("NewApp: config loaded (save_root=%q uploader=%q qiniu_bucket=%q)", cfg.SaveRoot, cfg.Uploader, cfg.Qiniu.Bucket)
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
	logger.Infof("SaveConfig: save_root=%q uploader=%q qiniu_bucket=%q", c.SaveRoot, c.Uploader, c.Qiniu.Bucket)
	if err := config.Save(c); err != nil {
		logger.Errorf("SaveConfig: persist failed: %v", err)
		return err
	}
	a.cfg = c
	logger.Info("SaveConfig: persisted")
	return nil
}

// ---------- detection ----------

// World is a world save with display metadata (alias/hidden) merged from config.
type World struct {
	palworld.World
	Alias  string `json:"alias"`
	Hidden bool   `json:"hidden"`
}

// ResolvedSaveRoot returns the save root the app uses (config override or auto).
func (a *App) ResolvedSaveRoot() (string, error) {
	if a.cfg.SaveRoot != "" {
		return a.cfg.SaveRoot, nil
	}
	return palworld.SaveRoot()
}

// OpenWorldFolder opens the world save folder in the system file explorer.
func (a *App) OpenWorldFolder(worldPath string) error {
	guid := filepath.Base(worldPath)
	logger.Infof("OpenWorldFolder: world=%s", guid)
	cmd := exec.Command("explorer", worldPath)
	if err := cmd.Start(); err != nil {
		logger.Errorf("OpenWorldFolder: world=%s failed: %v", guid, err)
		return fmt.Errorf("failed to open folder: %w", err)
	}
	return nil
}

// DetectWorlds lists local world saves under the configured (or auto) root.
func (a *App) DetectWorlds() ([]World, error) {
	root, err := a.ResolvedSaveRoot()
	if err != nil {
		logger.Errorf("DetectWorlds: resolve save root failed: %v", err)
		return nil, err
	}
	ws, err := palworld.ListWorlds(root)
	if err != nil {
		logger.Errorf("DetectWorlds: root=%s err=%v", root, err)
		return nil, err
	}
	logger.Infof("DetectWorlds: root=%s worlds=%d", root, len(ws))
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
	logger.Infof("SetWorldMeta: guid=%s alias=%q hidden=%v", guid, alias, hidden)
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
	if err := config.Save(a.cfg); err != nil {
		logger.Errorf("SetWorldMeta: persist failed: %v", err)
		return err
	}
	return nil
}

// ListPlayers returns the players in a world (host flagged).
func (a *App) ListPlayers(worldPath string) ([]palworld.Player, error) {
	players, err := palworld.ListPlayers(worldPath)
	if err != nil {
		logger.Errorf("ListPlayers: world=%s err=%v", worldPath, err)
		return nil, err
	}
	logger.Infof("ListPlayers: world=%s players=%d", filepath.Base(worldPath), len(players))
	return players, nil
}

// LocalSteamID returns the local player's SteamID64 (from the save folder).
func (a *App) LocalSteamID() (uint64, error) {
	root := a.cfg.SaveRoot
	if root == "" {
		r, err := palworld.SaveRoot()
		if err != nil {
			logger.Errorf("LocalSteamID: resolve root failed: %v", err)
			return 0, err
		}
		root = r
	}
	sid, err := palworld.LocalSteamID(root)
	if err != nil {
		logger.Errorf("LocalSteamID: root=%s err=%v", root, err)
		return 0, err
	}
	logger.Infof("LocalSteamID: root=%s steamid=%d", root, sid)
	return sid, nil
}

// ---------- host conversion ----------

// ActivateHost converts the local player's real UID to the host slot, making
// this machine the host. Call after downloading the intermediate.
func (a *App) ActivateHost(worldPath string) error {
	guid := filepath.Base(worldPath)
	sid, err := a.LocalSteamID()
	if err != nil {
		return err
	}
	fromUID := palworld.SteamIDToPlayerUUID(sid)
	logger.Infof("ActivateHost: world=%s steamid=%d -> %s (real->host)", guid, sid, fromUID)
	if err := palworld.ConvertHostWithoutBackup(worldPath, fromUID, palworld.HostUUID); err != nil {
		logger.Errorf("ActivateHost: world=%s convert failed: %v", guid, err)
		return err
	}
	if err := palworld.PruneBackups(worldPath, a.cfg.BackupKeep); err != nil {
		logger.Warnf("ActivateHost: prune backups failed: %v", err)
	}
	logger.Infof("ActivateHost: world=%s done", guid)
	return nil
}

// ---------- cloud ----------

func (a *App) newStorage() (storage.Storage, error) {
	q := a.cfg.Qiniu
	if q.AccessKey == "" || q.Bucket == "" {
		return nil, fmt.Errorf("qiniu config incomplete")
	}
	s, err := storage.NewQiniu(storage.QiniuConfig{
		AccessKey: q.AccessKey, SecretKey: q.SecretKey,
		Bucket: q.Bucket, Region: q.Region, Domain: q.Domain,
	})
	if err != nil {
		logger.Errorf("newStorage: qiniu init failed (bucket=%s region=%s): %v", q.Bucket, q.Region, err)
		return nil, err
	}
	return s, nil
}

func (a *App) lockManager() (*storage.LockManager, error) {
	s, err := a.newStorage()
	if err != nil {
		return nil, err
	}

	return &storage.LockManager{Store: s, TTL: a.cfg.LockTTL}, nil
}
// packAndStripToGuest packs the world as the transfer intermediate, backs up
// the full world, then strips the local world to guest-only. Returns the
// intermediate data for the caller to send to its final destination (cloud
// upload, file export, etc.).
func (a *App) packAndStripToGuest(worldPath string) ([]byte, error) {
	guid := filepath.Base(worldPath)

	sid, err := a.LocalSteamID()
	if err != nil {
		return nil, err
	}

	logger.Infof("packAndStripToGuest: world=%s packing intermediate", guid)
	data, err := palworld.PackIntermediate(worldPath, palworld.SteamIDToPlayerUUID(sid))
	if err != nil {
		logger.Errorf("packAndStripToGuest: world=%s pack intermediate failed: %v", guid, err)
		return nil, err
	}

	// Back up the full world, then strip to guest-only so the former host
	// cannot keep a conflicting host copy. They can restore from the backup
	// (Backups page) to play again.
	backupPath, err := palworld.BackupWorld(worldPath)
	if err != nil {
		logger.Errorf("packAndStripToGuest: world=%s backup failed: %v", guid, err)
		return nil, err
	}
	if err := palworld.StripToGuest(worldPath); err != nil {
		logger.Errorf("packAndStripToGuest: world=%s strip to guest failed: %v", guid, err)
		if rbErr := palworld.RestoreFromBackup(worldPath, backupPath); rbErr != nil {
			return nil, fmt.Errorf("strip to guest failed: %v; rollback also failed: %v; backup at %s, please restore manually", err, rbErr, backupPath)
		}
		return nil, fmt.Errorf("strip to guest failed (rolled back from %s): %w", backupPath, err)
	}
	if err := palworld.PruneBackups(worldPath, a.cfg.BackupKeep); err != nil {
		logger.Warnf("packAndStripToGuest: prune backups failed: %v", err)
	}

	logger.Infof("packAndStripToGuest: world=%s done (%d bytes)", guid, len(data))
	return data, nil
}

// UploadWorld packs the world as the transfer intermediate and uploads it to
// the cloud. After a successful upload the local world is stripped to guest -
// the owner can restore from Backups to play again.
func (a *App) UploadWorld(worldPath string) error {
	guid := filepath.Base(worldPath)
	s, err := a.newStorage()
	if err != nil {
		return err
	}

	data, err := a.packAndStripToGuest(worldPath)
	if err != nil {
		return err
	}

	up := a.cfg.Uploader
	if up == "" {
		up = "player"
	}
	key, err := storage.UploadVersion(context.Background(), s, guid, up, bytes.NewReader(data), int64(len(data)))
	if err != nil {
		logger.Errorf("UploadWorld: world=%s upload failed (%d bytes): %v", guid, len(data), err)
		return err
	}

	logger.Infof("UploadWorld: world=%s uploaded key=%s (%d bytes), stripped to guest", guid, key, len(data))
	return nil
}
// DownloadLatest downloads the newest cloud version and writes it to worldPath
// (after backing up the current world).
func (a *App) DownloadLatest(worldPath string) error {
	return a.DownloadVersion(worldPath, "")
}

// DownloadVersion downloads a specific version (or the latest if key is empty).
// The downloaded zip is validated before the local world is touched, then the
// world is backed up and cleanly replaced (preserving LocalData.sav).
func (a *App) DownloadVersion(worldPath, key string) error {
	guid := filepath.Base(worldPath)
	if err := palworld.AssertGameNotRunning(); err != nil {
		logger.Errorf("DownloadVersion: world=%s game running: %v", guid, err)
		return err
	}
	s, err := a.newStorage()
	if err != nil {
		return err
	}
	if key == "" {
		key, err = storage.LatestVersion(context.Background(), s, guid)
		if err != nil {
			logger.Errorf("DownloadVersion: world=%s list latest failed: %v", guid, err)
			return err
		}
		if key == "" {
			logger.Warnf("DownloadVersion: world=%s no cloud versions", guid)
			return fmt.Errorf("no cloud versions for world %s", guid)
		}
	}
	logger.Infof("DownloadVersion: world=%s key=%s downloading", guid, key)
	var buf bytes.Buffer
	if err := s.Download(context.Background(), key, &buf, nil); err != nil {
		logger.Errorf("DownloadVersion: world=%s key=%s download failed: %v", guid, key, err)
		return err
	}
	// Validate the downloaded zip before touching the local world so corrupt or
	// truncated data never reaches the save folder.
	if err := palworld.ValidateWorldZip(buf.Bytes()); err != nil {
		logger.Errorf("DownloadVersion: world=%s key=%s validation failed: %v", guid, key, err)
		return fmt.Errorf("downloaded save failed validation: %w", err)
	}
	logger.Infof("DownloadVersion: world=%s key=%s backing up", guid, key)
	backupPath, err := palworld.BackupWorld(worldPath)
	if err != nil {
		logger.Errorf("DownloadVersion: world=%s backup failed: %v", guid, err)
		return err
	}
	// Clean replace: clear the world folder (preserving LocalData.sav and the
	// game's backup/) then move the fresh files in. Not an overlay - stale
	// files from a previous state are removed.
	if err := palworld.ReplaceWorldKeepLocalData(worldPath, buf.Bytes()); err != nil {
		logger.Errorf("DownloadVersion: world=%s replace failed: %v", guid, err)
		if rbErr := palworld.RestoreFromBackup(worldPath, backupPath); rbErr != nil {
			return fmt.Errorf("replace failed: %v; rollback also failed: %v; backup at %s, please restore manually", err, rbErr, backupPath)
		}
		return fmt.Errorf("replace failed (rolled back from %s): %w", backupPath, err)
	}
	if err := palworld.PruneBackups(worldPath, a.cfg.BackupKeep); err != nil {
		logger.Warnf("DownloadVersion: prune backups failed: %v", err)
	}
	logger.Infof("DownloadVersion: world=%s key=%s done (%d bytes)", guid, key, buf.Len())
	return nil
}

// ListVersions returns the cloud version history for a world.
func (a *App) ListVersions(worldGUID string) ([]storage.Object, error) {
	s, err := a.newStorage()
	if err != nil {
		return nil, err
	}
	versions, err := storage.ListVersions(context.Background(), s, worldGUID)
	if err != nil {
		logger.Errorf("ListVersions: world=%s err=%v", worldGUID, err)
		return nil, err
	}
	logger.Infof("ListVersions: world=%s versions=%d", worldGUID, len(versions))
	return versions, nil
}

// LockStatus reports the play lock for a world.
func (a *App) LockStatus(worldGUID string) (storage.LockStatus, error) {
	lm, err := a.lockManager()
	if err != nil {
		return storage.LockStatus{}, err
	}
	st, err := lm.Status(context.Background(), worldGUID)
	if err != nil {
		logger.Errorf("LockStatus: world=%s err=%v", worldGUID, err)
		return storage.LockStatus{}, err
	}
	return st, nil
}

// AcquireLock claims the play lock.
func (a *App) AcquireLock(worldGUID, player string) error {
	logger.Infof("AcquireLock: world=%s player=%s", worldGUID, player)
	lm, err := a.lockManager()
	if err != nil {
		return err
	}
	if err := lm.Acquire(context.Background(), worldGUID, player); err != nil {
		logger.Errorf("AcquireLock: world=%s player=%s err=%v", worldGUID, player, err)
		return err
	}
	return nil
}

// ReleaseLock releases the play lock.
func (a *App) ReleaseLock(worldGUID string) error {
	logger.Infof("ReleaseLock: world=%s", worldGUID)
	lm, err := a.lockManager()
	if err != nil {
		return err
	}
	if err := lm.Release(context.Background(), worldGUID); err != nil {
		logger.Errorf("ReleaseLock: world=%s err=%v", worldGUID, err)
		return err
	}
	return nil
}

// ---------- backups ----------

// BackupRecord is a local backup entry.
type BackupRecord struct {
	Name string    `json:"name"`
	Size int64     `json:"size"`
	Time time.Time `json:"time"`
	IsHost bool      `json:"isHost"`  // true = host save, false = guest save
}

// ListBackups returns local backups for a world.
func (a *App) ListBackups(worldPath string) ([]BackupRecord, error) {
	guid := filepath.Base(worldPath)
	dir, err := palworld.BackupDir()
	if err != nil {
		return nil, err
	}
	bdir := filepath.Join(dir, guid)
	entries, err := os.ReadDir(bdir)
	if err != nil {
		if os.IsNotExist(err) {
			return []BackupRecord{}, nil
		}
		logger.Errorf("ListBackups: world=%s err=%v", guid, err)
		return nil, err
	}
	var out []BackupRecord
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".zip" {
			continue
		}
		info, _ := e.Info()
			isHost := !strings.HasSuffix(e.Name(), "_guest.zip")
			out = append(out, BackupRecord{Name: e.Name(), Size: info.Size(), Time: info.ModTime(), IsHost: isHost})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name > out[j].Name })
	logger.Infof("ListBackups: world=%s backups=%d", guid, len(out))
	return out, nil
}

// OpenBackupFolder opens the backup directory for a world in the system file explorer.
func (a *App) OpenBackupFolder(worldPath string) error {
	guid := filepath.Base(worldPath)
	dir, err := palworld.BackupDir()
	if err != nil {
		return err
	}
	bdir := filepath.Join(dir, guid)
	logger.Infof("OpenBackupFolder: world=%s dir=%s", guid, bdir)
	cmd := exec.Command("explorer", bdir)
	if err := cmd.Start(); err != nil {
		logger.Errorf("OpenBackupFolder: world=%s failed: %v", guid, err)
		return fmt.Errorf("failed to open folder: %w", err)
	}
	return nil
}

// RestoreBackup restores a backup: backs up the current state (safety net),
// then fully replaces the world folder with the backup contents (deletes
// everything first, then extracts - not just an overlay). If the replace fails,
// the safety backup is used to roll back automatically.
func (a *App) RestoreBackup(worldPath, name string) error {
	guid := filepath.Base(worldPath)
	logger.Infof("RestoreBackup: world=%s backup=%s", guid, name)
	if err := palworld.AssertGameNotRunning(); err != nil {
		logger.Errorf("RestoreBackup: world=%s game running: %v", guid, err)
		return err
	}
	dir, err := palworld.BackupDir()
	if err != nil {
		return err
	}
	data, err := os.ReadFile(filepath.Join(dir, guid, name))
	if err != nil {
		logger.Errorf("RestoreBackup: world=%s read backup failed: %v", guid, err)
		return err
	}
	backupPath, err := palworld.BackupWorld(worldPath)
	if err != nil {
		logger.Errorf("RestoreBackup: world=%s pre-backup failed: %v", guid, err)
		return err
	}
	if err := palworld.ReplaceWorldKeepLocalData(worldPath, data); err != nil {
		logger.Errorf("RestoreBackup: world=%s replace failed: %v", guid, err)
		if rbErr := palworld.RestoreFromBackup(worldPath, backupPath); rbErr != nil {
			return fmt.Errorf("restore failed: %v; rollback also failed: %v; backup at %s, please restore manually", err, rbErr, backupPath)
		}
		return fmt.Errorf("restore failed (rolled back from %s): %w", backupPath, err)
	}
	if err := palworld.PruneBackups(worldPath, a.cfg.BackupKeep); err != nil {
		logger.Warnf("RestoreBackup: prune backups failed: %v", err)
	}
	logger.Infof("RestoreBackup: world=%s backup=%s done", guid, name)
	return nil
}

// ---------- import/export ----------

// ExportWorld packs the world as the transfer intermediate and writes it to a
// .palrelay.zip at outPath. After a successful export the local world is
// stripped to guest - identical semantics to cloud upload minus the network.
func (a *App) ExportWorld(worldPath, outPath string) error {
	guid := filepath.Base(worldPath)
	logger.Infof("ExportWorld: world=%s -> %s", guid, outPath)

	data, err := a.packAndStripToGuest(worldPath)
	if err != nil {
		return err
	}

	if err := os.WriteFile(outPath, data, 0o644); err != nil {
		logger.Errorf("ExportWorld: write %s failed: %v", outPath, err)
		return err
	}
	logger.Infof("ExportWorld: world=%s -> %s done (%d bytes), stripped to guest", guid, outPath, len(data))
	return nil
}

// ImportWorld unpacks a .palrelay.zip into worldPath (after backup). The zip is
// validated before the local world is touched, then the world is cleanly
// replaced (preserving LocalData.sav). If the replace fails, the backup is used
// to roll back automatically.
func (a *App) ImportWorld(zipPath, worldPath string) error {
	guid := filepath.Base(worldPath)
	logger.Infof("ImportWorld: %s -> world=%s", zipPath, guid)
	if err := palworld.AssertGameNotRunning(); err != nil {
		logger.Errorf("ImportWorld: world=%s game running: %v", guid, err)
		return err
	}
	data, err := os.ReadFile(zipPath)
	if err != nil {
		logger.Errorf("ImportWorld: read failed: %v", err)
		return err
	}
	// Validate the zip before touching the local world so corrupt data never
	// reaches the save folder.
	if err := palworld.ValidateWorldZip(data); err != nil {
		logger.Errorf("ImportWorld: world=%s validation failed: %v", guid, err)
		return fmt.Errorf("import save failed validation: %w", err)
	}
	backupPath, err := palworld.BackupWorld(worldPath)
	if err != nil {
		logger.Errorf("ImportWorld: world=%s backup failed: %v", guid, err)
		return err
	}
	// Clean replace: clear the world folder (preserving LocalData.sav and the
	// game's backup/) then move the fresh files in.
	if err := palworld.ReplaceWorldKeepLocalData(worldPath, data); err != nil {
		logger.Errorf("ImportWorld: world=%s replace failed: %v", guid, err)
		if rbErr := palworld.RestoreFromBackup(worldPath, backupPath); rbErr != nil {
			return fmt.Errorf("replace failed: %v; rollback also failed: %v; backup at %s, please restore manually", err, rbErr, backupPath)
		}
		return fmt.Errorf("replace failed (rolled back from %s): %w", backupPath, err)
	}
	if err := palworld.PruneBackups(worldPath, a.cfg.BackupKeep); err != nil {
		logger.Warnf("ImportWorld: prune backups failed: %v", err)
	}
	logger.Infof("ImportWorld: %s -> world=%s done (%d bytes)", zipPath, guid, len(data))
	return nil
}




