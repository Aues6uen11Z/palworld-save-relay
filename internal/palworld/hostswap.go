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
	// Player .sav filenames use the GUID's mixed-endian String() form, uppercased,
	// with dashes removed (e.g. 00000000-...-000000000001 -> 0000...0001.sav).
	return strings.ToUpper(strings.ReplaceAll(u.String(), "-", "")) + ".sav"
}

// SwapHost makes the player targetUUID the host of the world at worldDir.
// It swaps the host slot (HostUUID) <-> targetUUID throughout the save, then
// renames the two player .sav files. Safe: backs up first, validates writes,
// rolls back on any error.
func SwapHost(worldDir string, targetUUID sav.UUID) error {
	if err := assertGameNotRunning(); err != nil {
		return err
	}
	backupPath, err := BackupWorld(worldDir)
	if err != nil {
		return fmt.Errorf("palworld: backup: %w", err)
	}
	if err := swapHostImpl(worldDir, targetUUID); err != nil {
		_ = restoreFromBackup(worldDir, backupPath)
		return err
	}
	return nil
}

func swapHostImpl(worldDir string, targetUUID sav.UUID) error {
	playersDir := filepath.Join(worldDir, "Players")
	hostFile := filepath.Join(playersDir, uidFilename(HostUUID))
	targetFile := filepath.Join(playersDir, uidFilename(targetUUID))

	hostGvas, _, err := readDecompress(hostFile)
	if err != nil {
		return fmt.Errorf("read host player: %w", err)
	}
	targetGvas, _, err := readDecompress(targetFile)
	if err != nil {
		return fmt.Errorf("read target player: %w", err)
	}
	hints, custom := sav.PalWorldConfig()
	hostGF, err := sav.ReadGvasFile(hostGvas, hints, custom)
	if err != nil {
		return err
	}
	targetGF, err := sav.ReadGvasFile(targetGvas, hints, custom)
	if err != nil {
		return err
	}
	hostInst := playerInstanceID(hostGF)
	targetInst := playerInstanceID(targetGF)
	if hostInst == nil || targetInst == nil {
		return fmt.Errorf("palworld: could not read InstanceId from player saves")
	}

	// Swap player saves (PlayerUId + IndividualId.PlayerUId).
	SwapPlayerSav(hostGF, HostUUID, targetUUID)
	SwapPlayerSav(targetGF, targetUUID, HostUUID)

	// Swap Level.sav.
	levelPath := filepath.Join(worldDir, "Level.sav")
	levelGvas, _, err := readDecompress(levelPath)
	if err != nil {
		return fmt.Errorf("read Level.sav: %w", err)
	}
	levelGF, err := sav.ReadGvasFile(levelGvas, hints, custom)
	if err != nil {
		return err
	}
	SwapLevelSav(levelGF, hostInst, targetInst, HostUUID, targetUUID)

	// Write all three atomically (tmp -> validate -> rename).
	if err := writeValidatedSav(hostFile, hostGF, custom); err != nil {
		return fmt.Errorf("write host player: %w", err)
	}
	if err := writeValidatedSav(targetFile, targetGF, custom); err != nil {
		return fmt.Errorf("write target player: %w", err)
	}
	if err := writeValidatedSav(levelPath, levelGF, custom); err != nil {
		return fmt.Errorf("write Level.sav: %w", err)
	}

	// Rename player files: host slot <-> target slot.
	tmpHost := hostFile + ".swap"
	tmpTarget := targetFile + ".swap"
	if err := os.Rename(hostFile, tmpHost); err != nil {
		return err
	}
	if err := os.Rename(targetFile, tmpTarget); err != nil {
		return err
	}
	if err := os.Rename(tmpTarget, hostFile); err != nil {
		return err
	}
	if err := os.Rename(tmpHost, targetFile); err != nil {
		return err
	}
	return nil
}

// readDecompress reads a .sav file and decompresses it to GVAS bytes.
func readDecompress(path string) ([]byte, sav.SAVHeader, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, sav.SAVHeader{}, err
	}
	return sav.Decompress(data)
}

// writeValidatedSav serializes+compresses gf to a .tmp file, re-parses it to
// validate integrity, then atomically renames over the target path.
func writeValidatedSav(path string, gf *sav.GvasFile, custom map[string]sav.CustomProperty) error {
	orig, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	_, hdr, err := sav.Decompress(orig)
	if err != nil {
		return err
	}
	out, _ := sav.Compress(gf.Write(custom), hdr)
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, out, 0o644); err != nil {
		return err
	}
	// validate: re-decompress + parse.
	check, _, err := sav.Decompress(out)
	if err != nil {
		os.Remove(tmp)
		return fmt.Errorf("validation decompress: %w", err)
	}
	if _, err := sav.ReadGvasFile(check, sav.PalWorldTypeHints, custom); err != nil {
		os.Remove(tmp)
		return fmt.Errorf("validation parse: %w", err)
	}
	return os.Rename(tmp, path)
}

func playerInstanceID(gf *sav.GvasFile) *sav.UUID {
	sd := gf.Properties.Get("SaveData")
	if sd == nil {
		return nil
	}
	ind := sd["value"].(sav.PropertyList).Get("IndividualId")
	if ind == nil {
		return nil
	}
	inst := ind["value"].(sav.PropertyList).Get("InstanceId")
	if inst == nil {
		return nil
	}
	g, _ := inst["value"].(*sav.UUID)
	return g
}

func restoreFromBackup(worldDir, backupZip string) error {
	data, err := os.ReadFile(backupZip)
	if err != nil {
		return err
	}
	// Clear current world contents (except backup/) then unpack.
	entries, _ := os.ReadDir(worldDir)
	for _, e := range entries {
		if e.IsDir() && e.Name() == "backup" {
			continue
		}
		os.RemoveAll(filepath.Join(worldDir, e.Name()))
	}
	return UnpackWorld(data, worldDir)
}
