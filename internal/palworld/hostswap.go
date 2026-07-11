package palworld

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

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
	if err := assertGameNotRunning(); err != nil {
		return err
	}
	backupPath, err := BackupWorld(worldDir)
	if err != nil {
		return fmt.Errorf("palworld: backup: %w", err)
	}
	if err := convertHostImpl(worldDir, fromUID, toUID); err != nil {
		_ = restoreFromBackup(worldDir, backupPath)
		return err
	}
	return nil
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
