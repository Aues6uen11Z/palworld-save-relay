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

// assertUploadReady verifies the world is in a safe state for producing a relay
// intermediate (upload or export). It prevents the most dangerous multi-player
// misuse: downloading without activating, playing, then uploading — which would
// create duplicate UID entries and corrupt the cloud save for everyone.
//
// The world must satisfy all three conditions:
//  1. Level.sav exists (this is a host world, not a guest-only folder).
//  2. The host player save (HostUUID.sav) exists (the player has activated or
//     is the original host).
//  3. The uploader's own realUID player save does NOT exist (no stale guest
//     copy that would duplicate on conversion). If it does, the player played
//     without activating; they must restore from backup and activate first.
func assertUploadReady(worldDir string, realUID sav.UUID) error {
	levelPath := filepath.Join(worldDir, "Level.sav")
	if _, err := os.Stat(levelPath); err != nil {
		return fmt.Errorf("not a host world (Level.sav missing); download the latest cloud save and activate as host first")
	}
	playersDir := filepath.Join(worldDir, "Players")
	hostFile := filepath.Join(playersDir, uidFilename(HostUUID))
	if _, err := os.Stat(hostFile); err != nil {
		return fmt.Errorf("not the host (host player save %s missing); activate as host first", filepath.Base(hostFile))
	}
	realFile := filepath.Join(playersDir, uidFilename(realUID))
	if _, err := os.Stat(realFile); err == nil {
		return fmt.Errorf("duplicate player data: both the host save and your guest save exist. This happens when you play without activating after downloading. Please restore from a backup, then activate as host before uploading")
	}
	return nil
}

// ConvertHost replaces fromUID -> toUID throughout the world save (Level.sav +
// the player .sav named fromUID) and renames that player file to toUID.
//
// Relay flow:
//   - current host uploads: ConvertHost(world, HostUUID, hostRealUID)
//   - successor hosts:      ConvertHost(world, myRealUID, HostUUID)
//
// Safe: backs up first, validates writes, rolls back on any error. If the
// rollback itself fails, the returned error names the backup path so the user
// can recover manually.
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
		if rbErr := RestoreFromBackup(worldDir, backupPath); rbErr != nil {
			return fmt.Errorf("convert failed: %v; rollback also failed: %v; backup at %s, please restore manually", err, rbErr, backupPath)
		}
		return fmt.Errorf("convert failed (rolled back from %s): %w", backupPath, err)
	}
	logger.Infof("ConvertHost: world=%s done (backup=%s)", guid, backupPath)
	return nil
}


// ConvertHostWithoutBackup is like ConvertHost but skips the backup step. Use
// this when the caller has already backed up the world (e.g. DownloadVersion
// or ImportWorld backs up before calling ActivateHost). If the convert fails
// the error is returned directly; the caller must manage rollback.
func ConvertHostWithoutBackup(worldDir string, fromUID, toUID sav.UUID) error {
	guid := filepath.Base(worldDir)
	logger.Infof("ConvertHostWithoutBackup: world=%s %s -> %s", guid, fromUID.String(), toUID.String())
	if err := assertGameNotRunning(); err != nil {
		logger.Errorf("ConvertHostWithoutBackup: world=%s game running: %v", guid, err)
		return err
	}
	if err := convertHostImpl(worldDir, fromUID, toUID); err != nil {
		logger.Errorf("ConvertHostWithoutBackup: world=%s convert failed: %v", guid, err)
		return fmt.Errorf("palworld: convert host: %w", err)
	}
	logger.Infof("ConvertHostWithoutBackup: world=%s done", guid)
	return nil
}
// PackIntermediate produces the cloud/manual-transfer intermediate WITHOUT
// modifying the original world: it packs worldDir, unpacks into a temp dir,
// converts the host slot (0001) to realUID in the temp copy, and repacks. The
// original worldDir is only ever read. Use this for upload/export so the local
// save is left untouched (the uploader keeps being host).
//
// Before packing, assertUploadReady is called to prevent producing a corrupt
// intermediate from a world that was played without activating after download.
func PackIntermediate(worldDir string, realUID sav.UUID) ([]byte, error) {
	if err := assertGameNotRunning(); err != nil {
		return nil, err
	}
	if err := assertUploadReady(worldDir, realUID); err != nil {
		logger.Errorf("PackIntermediate: world=%s upload-ready check failed: %v", filepath.Base(worldDir), err)
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
	if err := convertHostLevel(levelPath, fromUID, toUID, hints, custom); err != nil {
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

// StripToGuest removes all world data except LocalData.sav (and the game's own
// backup/ subdir), turning a host world into a guest-only folder. Used after
// uploading so the former host cannot keep a conflicting host copy; they can
// restore from a backup to play again. Asserts the game is not running.
func StripToGuest(worldDir string) error {
	if err := assertGameNotRunning(); err != nil {
		return err
	}
	guid := filepath.Base(worldDir)
	entries, err := os.ReadDir(worldDir)
	if err != nil {
		return err
	}
	for _, e := range entries {
		name := e.Name()
		if name == "LocalData.sav" {
			continue
		}
		if e.IsDir() && name == "backup" {
			continue
		}
		if err := os.RemoveAll(filepath.Join(worldDir, name)); err != nil {
			return fmt.Errorf("remove %s: %w", name, err)
		}
	}
	logger.Infof("StripToGuest: world=%s stripped to LocalData.sav only", guid)
	return nil
}

// ReplaceWorld clears worldDir (keeping the game's own backup/ subdir) then
// extracts zipBytes into it. Unpacks to a temp directory first so a corrupt or
// truncated zip never touches the live world; only after a successful unpack
// does it clear and move files in.
func ReplaceWorld(worldDir string, zipBytes []byte) error {
	return replaceWorld(worldDir, zipBytes, false)
}

// ReplaceWorldKeepLocalData is like ReplaceWorld but preserves LocalData.sav
// (personal local progress) and the game's backup/ subdir. Used for cloud
// download / import where the incoming zip is a relay intermediate that
// intentionally omits LocalData.sav; the local player keeps their own. Any
// LocalData.sav present in the zip is discarded.
func ReplaceWorldKeepLocalData(worldDir string, zipBytes []byte) error {
	return replaceWorld(worldDir, zipBytes, true)
}

// replaceWorld is the shared implementation. When keepLocalData is true,
// LocalData.sav in the live world is preserved and any LocalData.sav in the zip
// is dropped so it never overwrites the local player's personal progress.
func replaceWorld(worldDir string, zipBytes []byte, keepLocalData bool) error {
	guid := filepath.Base(worldDir)
	parent := filepath.Dir(worldDir)

	// Unpack to a temp dir on the same volume as worldDir so os.Rename is
	// atomic. If the zip is corrupt/truncated, this fails *before* the live
	// world is touched.
	tmp, err := os.MkdirTemp(parent, ".palrelay-tmp-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmp)

	if err := UnpackWorld(zipBytes, tmp); err != nil {
		return fmt.Errorf("unpack: %w", err)
	}

	// When preserving LocalData.sav, drop it from the temp copy so it will not
	// overwrite the local player's personal progress.
	if keepLocalData {
		_ = os.Remove(filepath.Join(tmp, "LocalData.sav"))
	}

	// Ensure worldDir exists (first download to a new world folder).
	if err := os.MkdirAll(worldDir, 0o755); err != nil {
		return err
	}

	// Clear worldDir (keep the game's own backup/ subdir; keep LocalData.sav if
	// requested). This is a full clean replace, not an overlay - stale files
	// from a previous state are removed.
	entries, _ := os.ReadDir(worldDir)
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() && name == "backup" {
			continue
		}
		if keepLocalData && name == "LocalData.sav" {
			continue
		}
		if err := os.RemoveAll(filepath.Join(worldDir, name)); err != nil {
			return fmt.Errorf("clear %s: %w", name, err)
		}
	}

	// Move all files from temp into worldDir (same-volume rename = atomic per
	// file).
	entries, _ = os.ReadDir(tmp)
	for _, e := range entries {
		src := filepath.Join(tmp, e.Name())
		dst := filepath.Join(worldDir, e.Name())
		if err := os.Rename(src, dst); err != nil {
			return fmt.Errorf("move %s: %w", e.Name(), err)
		}
	}

	logger.Infof("replaceWorld: world=%s replaced (%d bytes, keepLocalData=%v)", guid, len(zipBytes), keepLocalData)
	return nil
}

// RestoreFromBackup reads a backup zip and fully replaces worldDir with its
// contents (keeping the game's own backup/ subdir). Exported so the app layer
// can roll back after a failed operation.
func RestoreFromBackup(worldDir, backupZip string) error {
	data, err := os.ReadFile(backupZip)
	if err != nil {
		return err
	}
	// Use KeepLocalData so the player's personal LocalData.sav is never
	// overwritten by a backup's copy - even during auto-rollback.
	return ReplaceWorldKeepLocalData(worldDir, data)
}








// convertHostLevel converts the host slot in Level.sav for a host step-down or
// step-up. Unlike a uniform UID swap (ConvertGvas), it moves ONLY the host
// player's identity - the CSPM player entry key, the ICH handle guid, guild
// membership, and pal ownership (OwnerPlayerUId) - from fromUID to toUID.
//
// World data stays put: every pal's CSPM bucket (key.PlayerUId), every ICH
// handle guid, and all opaque RawData blobs remain on whatever slot they're
// already on. In a host world all pals live under the host sentinel slot
// (0001), so after a step-down (0001 -> realUID) the pals stay on 0001 and the
// next host inherits them - matching an official host world, instead of
// dragging the whole world's pals into the old host's personal bucket.
func convertHostLevel(levelPath string, fromUID, toUID sav.UUID, hints map[string]string, custom map[string]sav.CustomProperty) error {
	data, err := os.ReadFile(levelPath)
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

	hostInst := findHostPlayerInstance(gf, fromUID)
	if hostInst == nil {
		logger.Warnf("convertHostLevel: host player entry for %s not found; moving identity only via deepReplace", fromUID.String())
	}

	// 1. Host identity + pal ownership, but NOT pal CSPM buckets or ICH guids.
	deepReplaceFields(gf.Properties, fromUID, toUID, hostIdentityFields)

	// 2. Move the host player's own CSPM key and ICH handle guid.
	moveHostPlayerSlot(gf, fromUID, toUID, hostInst)

	// (pal CSPM keys and opaque blobs are deliberately left untouched.)

	out, err := sav.Compress(gf.Write(custom), hdr)
	if err != nil {
		return err
	}
	// Validate: re-decompress + re-parse before committing.
	check, _, err := sav.Decompress(out)
	if err != nil {
		return fmt.Errorf("validation decompress: %w", err)
	}
	if _, err := sav.ReadGvasFile(check, hints, custom); err != nil {
		return fmt.Errorf("validation parse: %w", err)
	}
	tmp := levelPath + ".tmp"
	if err := os.WriteFile(tmp, out, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, levelPath)
}

// findHostPlayerInstance returns the InstanceId of the host player's CSPM entry
// (the IsPlayer=true entry whose key.PlayerUId == hostUID), or nil if not found.
func findHostPlayerInstance(gf *sav.GvasFile, hostUID sav.UUID) *sav.UUID {
	wsd := gf.Properties.Get("worldSaveData")
	if wsd == nil {
		return nil
	}
	cspm := wsd["value"].(sav.PropertyList).Get("CharacterSaveParameterMap")
	if cspm == nil {
		return nil
	}
	entries, _ := cspm["value"].([]map[string]any)
	for _, e := range entries {
		key, _ := e["key"].(sav.PropertyList)
		if key == nil {
			continue
		}
		p := key.Get("PlayerUId")
		if p == nil {
			continue
		}
		puid, ok := p["value"].(*sav.UUID)
		if !ok || puid == nil || *puid != hostUID {
			continue
		}
		if !cspmEntryIsPlayer(e) {
			continue
		}
		inst := key.Get("InstanceId")
		if inst == nil {
			return nil
		}
		if iv, ok := inst["value"].(*sav.UUID); ok && iv != nil {
			return iv
		}
		return nil
	}
	return nil
}

// cspmEntryIsPlayer reports whether a CSPM entry's SaveParameter has IsPlayer=true.
func cspmEntryIsPlayer(e map[string]any) bool {
	val, _ := e["value"].(sav.PropertyList)
	if val == nil {
		return false
	}
	raw := val.Get("RawData")
	if raw == nil {
		return false
	}
	rv, _ := raw["value"].(map[string]any)
	obj, _ := rv["object"].(sav.PropertyList)
	sp := obj.Get("SaveParameter")
	inner, _ := sp["value"].(sav.PropertyList)
	ip := inner.Get("IsPlayer")
	if ip == nil {
		return false
	}
	b, _ := ip["value"].(bool)
	return b
}

// moveHostPlayerSlot moves the host player's CSPM key.PlayerUId and ICH handle
// guid from fromUID to toUID (identified by the host player's InstanceId). Pal
// entries are left on their existing slot.
func moveHostPlayerSlot(gf *sav.GvasFile, fromUID, toUID sav.UUID, hostInst *sav.UUID) {
	if hostInst == nil {
		return
	}
	wsd := gf.Properties.Get("worldSaveData")
	if wsd == nil {
		return
	}
	pl := wsd["value"].(sav.PropertyList)

	// CSPM key: the host player's entry.
	if cspm := pl.Get("CharacterSaveParameterMap"); cspm != nil {
		entries, _ := cspm["value"].([]map[string]any)
		for _, e := range entries {
			key, _ := e["key"].(sav.PropertyList)
			if key == nil {
				continue
			}
			inst := key.Get("InstanceId")
			if inst == nil {
				continue
			}
			iv, ok := inst["value"].(*sav.UUID)
			if !ok || iv == nil || !iv.Equal(hostInst) {
				continue
			}
			puid := key.Get("PlayerUId")
			if puid == nil {
				continue
			}
			if pv, ok := puid["value"].(*sav.UUID); ok && pv != nil && *pv == fromUID {
				puid["value"] = uidPtr(toUID)
			}
		}
	}

	// ICH guid: the host player's handle, in every guild.
	if gsdm := pl.Get("GroupSaveDataMap"); gsdm != nil {
		groups, _ := gsdm["value"].([]map[string]any)
		for _, g := range groups {
			gv, _ := g["value"].(sav.PropertyList)
			raw := gv.Get("RawData")
			inner, _ := raw["value"].(map[string]any)
			if inner == nil {
				continue
			}
			ich, _ := inner["individual_character_handle_ids"].([]any)
			for _, h := range ich {
				m, _ := h.(map[string]any)
				if m == nil {
					continue
				}
			inst, _ := m["instance_id"].(*sav.UUID)
				if inst == nil || !inst.Equal(hostInst) {
					continue
				}
				guid, _ := m["guid"].(*sav.UUID)
				if guid != nil && *guid == fromUID {
					m["guid"] = uidPtr(toUID)
				}
			}
		}
	}
}
