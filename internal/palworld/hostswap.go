package palworld

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"palworld-save-relay/internal/logger"

	"palworld-save-relay/internal/sav"
)

// uidFilename returns the .sav filename for a UID (formatted, no dashes, upper).
func uidFilename(u sav.UUID) string {
	return strings.ToUpper(strings.ReplaceAll(u.String(), "-", "")) + ".sav"
}

// ConvertHost replaces fromUID -> toUID throughout the world save (Level.sav +
// the player .sav named fromUID) and renames that player file to toUID.
//
// Relay flow:
//   - current host uploads: ConvertHost(world, HostUUID, hostRealUID)
//   - successor hosts:      ConvertHost(world, myRealUID, HostUUID)
//
// Safe: backs up first, validates writes, rolls back on any error.
func ConvertHost(worldDir string, fromUID, toUID sav.UUID) error {
	guid := filepath.Base(worldDir)
	logger.Infof("ConvertHost: world=%s %s -> %s", guid, fromUID.String(), toUID.String())
	if err := assertGameNotRunning(); err != nil {
		logger.Errorf("ConvertHost: world=%s game running: %v", guid, err)
		return err
	}
	backupPath, err := BackupWorld(worldDir)
	if err != nil {
		logger.Errorf("ConvertHost: world=%s backup failed: %v", guid, err)
		return fmt.Errorf("palworld: backup: %w", err)
	}
	if err := convertHostImpl(worldDir, fromUID, toUID); err != nil {
		logger.Errorf("ConvertHost: world=%s impl failed, rolling back from %s: %v", guid, backupPath, err)
		_ = restoreFromBackup(worldDir, backupPath)
		return err
	}
	logger.Infof("ConvertHost: world=%s done (backup=%s)", guid, backupPath)
	return nil
}

// PackIntermediate produces the cloud/manual-transfer intermediate WITHOUT
// modifying the original world: it packs worldDir, unpacks into a temp dir,
// converts the host slot (0001) to realUID in the temp copy, and repacks. The
// original worldDir is only ever read. Use this for upload/export so the local
// save is left untouched (the uploader keeps being host).
func PackIntermediate(worldDir string, realUID sav.UUID) ([]byte, error) {
	if err := assertGameNotRunning(); err != nil {
		return nil, err
	}
	data, err := PackWorld(worldDir)
	if err != nil {
		return nil, fmt.Errorf("pack original: %w", err)
	}
	tmp, err := os.MkdirTemp("", "palrelay-int-*")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(tmp)
	dst := filepath.Join(tmp, "world")
	if err := UnpackWorld(data, dst); err != nil {
		return nil, fmt.Errorf("unpack to temp: %w", err)
	}
	// LocalData.sav is personal local progress (quests/map/etc.); it must never
	// be transferred. Drop it from the intermediate so a downloader keeps their
	// own. (Local backups still include it via PackWorld for full rollback.)
	if err := os.Remove(filepath.Join(dst, "LocalData.sav")); err == nil {
		logger.Info("PackIntermediate: dropped LocalData.sav from intermediate (personal, not transferred)")
	}
	if err := convertHostImpl(dst, HostUUID, realUID); err != nil {
		return nil, fmt.Errorf("convert temp copy: %w", err)
	}
	return PackWorld(dst)
}

func convertHostImpl(worldDir string, fromUID, toUID sav.UUID) error {
	playersDir := filepath.Join(worldDir, "Players")
	fromFile := filepath.Join(playersDir, uidFilename(fromUID))
	toFile := filepath.Join(playersDir, uidFilename(toUID))
	hints, custom := sav.PalWorldConfig()

	// Convert Level.sav in place.
	levelPath := filepath.Join(worldDir, "Level.sav")
	if err := convertFile(levelPath, levelPath, fromUID, toUID, hints, custom); err != nil {
		return fmt.Errorf("Level.sav: %w", err)
	}
	// Convert the player save fromUID.sav -> toUID.sav.
	if _, err := os.Stat(fromFile); err != nil {
		return fmt.Errorf("player save %s: %w", filepath.Base(fromFile), err)
	}
	if err := convertFile(fromFile, toFile, fromUID, toUID, hints, custom); err != nil {
		return fmt.Errorf("player save: %w", err)
	}
	if err := os.Remove(fromFile); err != nil {
		return fmt.Errorf("remove old player save: %w", err)
	}
	return nil
}

// convertFile reads path, replaces fromUID -> toUID, validates, and writes
// atomically to outPath.
func convertFile(path, outPath string, from, to sav.UUID, hints map[string]string, custom map[string]sav.CustomProperty) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	gvas, hdr, err := sav.Decompress(data)
	if err != nil {
		return err
	}
	gf, err := sav.ReadGvasFile(gvas, hints, custom)
	if err != nil {
		return err
	}
	ConvertGvas(gf, from, to)
	out, err := sav.Compress(gf.Write(custom), hdr)
	if err != nil {
		return err
	}
	// Validate: re-decompress + re-parse the output before committing.
	check, _, err := sav.Decompress(out)
	if err != nil {
		return fmt.Errorf("validation decompress: %w", err)
	}
	if _, err := sav.ReadGvasFile(check, hints, custom); err != nil {
		return fmt.Errorf("validation parse: %w", err)
	}
	tmp := outPath + ".tmp"
	if err := os.WriteFile(tmp, out, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, outPath)
}

func restoreFromBackup(worldDir, backupZip string) error {
	data, err := os.ReadFile(backupZip)
	if err != nil {
		return err
	}
	entries, _ := os.ReadDir(worldDir)
	for _, e := range entries {
		if e.IsDir() && e.Name() == "backup" {
			continue
		}
		os.RemoveAll(filepath.Join(worldDir, e.Name()))
	}
	return UnpackWorld(data, worldDir)
}
