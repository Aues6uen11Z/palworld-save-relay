package palworld

import (
	"fmt"
	"os"
	"path/filepath"

	"palworld-save-relay/internal/sav"
)

// RepairReport summarizes what RepairIntermediate changed.
type RepairReport struct {
	HostUID          string // detected old-host UID whose bucket the pals were scattered into
	RebuiltICH       bool   // a guild's ICH was truncated -> rebuilt from CSPM by group_id
	ConsolidatedPals bool   // pals were moved back onto the host sentinel slot (0001)
	ConvertedOpaque  bool   // old-host UIDs in opaque blobs moved back to the host slot
}

// RepairIntermediate repairs a cloud/imported intermediate that may have been
// corrupted by an early version of the host-swap tool, and normalizes it to the
// clean "official host world" structure: all pals live under the host sentinel
// slot (0001), not dragged into a former host's personal bucket. Idempotent: a
// healthy/clean save is left untouched.
//
// What it does (only when a guild's ICH is incomplete - the truncation marker):
//   - Consolidates every pal's CSPM bucket back onto the host slot (0001). The
//     old uniform conversion dragged the whole world's pals into the stepping
//     -down host's real UID; this gathers them back so the next host inherits
//     them, matching an official host world. Player entries stay on their UIDs.
//   - Moves the old host's UIDs in opaque RawData blobs (MapObject/WorkSaveData
//     ownership) back to the host slot.
//   - Rebuilds each truncated guild ICH from the (now consolidated) CSPM, per
//     group_id - the actual fix for the "can't lift base pal" bug.
//
// group_name is deliberately not touched (verified unrelated to the lift bug).
func RepairIntermediate(worldDir string) (*RepairReport, error) {
	levelPath := filepath.Join(worldDir, "Level.sav")
	data, err := os.ReadFile(levelPath)
	if err != nil {
		return nil, fmt.Errorf("repair: read Level.sav: %w", err)
	}
	gvas, hdr, err := sav.Decompress(data)
	if err != nil {
		return nil, fmt.Errorf("repair: decompress: %w", err)
	}
	hints, custom := sav.PalWorldConfig()
	gf, err := sav.ReadGvasFile(gvas, hints, custom)
	if err != nil {
		return nil, fmt.Errorf("repair: parse: %w", err)
	}

	rep := &RepairReport{}
	guilds := findAllGuilds(gf)
	if len(guilds) == 0 {
		return rep, nil
	}

	// Only act when a guild's ICH is incomplete (the early-tool truncation
	// marker). A clean save (ICH complete) is left untouched.
	needFix := false
	for _, inner := range guilds {
		ich, _ := inner["individual_character_handle_ids"].([]any)
		if len(ich) != len(buildICHForGroup(gf, inner)) {
			needFix = true
			break
		}
	}
	if !needFix {
		return rep, nil
	}

	// Detect the old host: the non-sentinel UID holding the most CSPM entries.
	// In a legacy intermediate the world's pals were dragged into this bucket.
	oldHost, oldHostCount := detectHostUID(gf)

	// 1. Consolidate every pal onto the host sentinel slot (0001). Player
	//    entries (IsPlayer=true) stay on their own UIDs.
	rep.ConsolidatedPals = consolidatePalsToHostSlot(gf)

	// 2. Move the old host's objects back to the host slot in opaque blobs.
	//    Only when a real old host was detected (count > 1: more than a lone
	//    guest player entry), so a clean save's guest UIDs aren't touched.
	if oldHost != nil && *oldHost != HostUUID && oldHostCount > 1 {
		replaceOpaqueGUIDs(gf.Properties, *oldHost, HostUUID)
		rep.ConvertedOpaque = true
		rep.HostUID = oldHost.String()
	}

	// 3. Rebuild each truncated guild ICH from the consolidated CSPM (per
	//    group_id). Pals are now under 0001, players under their own UIDs.
	for _, inner := range guilds {
		ich, _ := inner["individual_character_handle_ids"].([]any)
		if len(ich) == len(buildICHForGroup(gf, inner)) {
			continue
		}
		inner["individual_character_handle_ids"] = buildICHForGroup(gf, inner)
		rep.RebuiltICH = true
	}

	out, err := sav.Compress(gf.Write(custom), hdr)
	if err != nil {
		return nil, fmt.Errorf("repair: compress: %w", err)
	}
	if err := writeLevelAtomic(levelPath, out); err != nil {
		return nil, fmt.Errorf("repair: write: %w", err)
	}
	return rep, nil
}

// findAllGuilds returns the parsed inner maps of every EPalGroupType::Guild.
func findAllGuilds(gf *sav.GvasFile) []map[string]any {
	wsd := gf.Properties.Get("worldSaveData")
	if wsd == nil {
		return nil
	}
	gsdm := wsd["value"].(sav.PropertyList).Get("GroupSaveDataMap")
	if gsdm == nil {
		return nil
	}
	groups, _ := gsdm["value"].([]map[string]any)
	var out []map[string]any
	for _, g := range groups {
		gv, _ := g["value"].(sav.PropertyList)
		if gv == nil {
			continue
		}
		raw := gv.Get("RawData")
		if raw == nil {
			continue
		}
		inner, _ := raw["value"].(map[string]any)
		if inner == nil {
			continue
		}
		if gtype, _ := inner["group_type"].(string); gtype == "EPalGroupType::Guild" {
			out = append(out, inner)
		}
	}
	return out
}

// findGuild returns the first Guild inner map (used by tests).
func findGuild(gf *sav.GvasFile) (map[string]any, error) {
	if g := findAllGuilds(gf); len(g) > 0 {
		return g[0], nil
	}
	return nil, fmt.Errorf("no EPalGroupType::Guild group found")
}

// cspmCount returns the number of CharacterSaveParameterMap entries.
func cspmCount(gf *sav.GvasFile) int {
	wsd := gf.Properties.Get("worldSaveData")
	if wsd == nil {
		return 0
	}
	cspm := wsd["value"].(sav.PropertyList).Get("CharacterSaveParameterMap")
	if cspm == nil {
		return 0
	}
	entries, _ := cspm["value"].([]map[string]any)
	return len(entries)
}

// buildICHForGroup rebuilds a guild's individual_character_handle_ids from the
// CSPM entries whose RawData group_id matches the guild's group_id. A healthy
// guild's ICH lists exactly its own members (by group_id), NOT every CSPM
// character - verified against single-guild and two-guild saves.
func buildICHForGroup(gf *sav.GvasFile, guild map[string]any) []any {
	gid, _ := guild["group_id"].(*sav.UUID)
	if gid == nil {
		return nil
	}
	wsd := gf.Properties.Get("worldSaveData")
	if wsd == nil {
		return nil
	}
	cspm := wsd["value"].(sav.PropertyList).Get("CharacterSaveParameterMap")
	if cspm == nil {
		return nil
	}
	entries, _ := cspm["value"].([]map[string]any)
	ich := make([]any, 0, len(entries))
	for _, e := range entries {
		val, _ := e["value"].(sav.PropertyList)
		if val == nil {
			continue
		}
		raw := val.Get("RawData")
		if raw == nil {
			continue
		}
		rv, _ := raw["value"].(map[string]any)
		if rv == nil {
			continue
		}
		eg, _ := rv["group_id"].(*sav.UUID)
		if eg == nil || !eg.Equal(gid) {
			continue
		}
		key, _ := e["key"].(sav.PropertyList)
		if key == nil {
			continue
		}
		var puid, inst *sav.UUID
		if p := key.Get("PlayerUId"); p != nil {
			puid, _ = p["value"].(*sav.UUID)
		}
		if p := key.Get("InstanceId"); p != nil {
			inst, _ = p["value"].(*sav.UUID)
		}
		if puid == nil || inst == nil {
			continue
		}
		ich = append(ich, map[string]any{"guid": puid, "instance_id": inst})
	}
	return ich
}

// detectHostUID returns the non-sentinel player UID holding the most CSPM
// entries, plus that count. In a legacy intermediate this is the former host
// whose bucket the world's pals were dragged into. Returns (nil, 0) if none.
func detectHostUID(gf *sav.GvasFile) (*sav.UUID, int) {
	wsd := gf.Properties.Get("worldSaveData")
	if wsd == nil {
		return nil, 0
	}
	cspm := wsd["value"].(sav.PropertyList).Get("CharacterSaveParameterMap")
	if cspm == nil {
		return nil, 0
	}
	entries, _ := cspm["value"].([]map[string]any)
	cnt := map[string]int{}
	var best *sav.UUID
	bestN := 0
	for _, e := range entries {
		key, _ := e["key"].(sav.PropertyList)
		if key == nil {
			continue
		}
		p := key.Get("PlayerUId")
		if p == nil {
			continue
		}
		uid, ok := p["value"].(*sav.UUID)
		if !ok || uid == nil || *uid == HostUUID {
			continue
		}
		s := uid.String()
		cnt[s]++
		if cnt[s] > bestN {
			bestN = cnt[s]
			best = uid
		}
	}
	return best, bestN
}

// consolidatePalsToHostSlot moves every non-player CSPM entry's key.PlayerUId
// onto the host sentinel slot (0001). In a clean host world all pals live under
// the host slot; the legacy conversion scattered them across former-host
// buckets, so this gathers them back. Player entries (IsPlayer=true) keep their
// own UID. Returns true if any pal was moved.
func consolidatePalsToHostSlot(gf *sav.GvasFile) bool {
	wsd := gf.Properties.Get("worldSaveData")
	if wsd == nil {
		return false
	}
	cspm := wsd["value"].(sav.PropertyList).Get("CharacterSaveParameterMap")
	if cspm == nil {
		return false
	}
	entries, _ := cspm["value"].([]map[string]any)
	moved := false
	for _, e := range entries {
		if cspmEntryIsPlayer(e) {
			continue
		}
		key, _ := e["key"].(sav.PropertyList)
		if key == nil {
			continue
		}
		puid := key.Get("PlayerUId")
		if puid == nil {
			continue
		}
		if g, ok := puid["value"].(*sav.UUID); ok && g != nil && *g != HostUUID {
			puid["value"] = uidPtr(HostUUID)
			moved = true
		}
	}
	return moved
}

// hostSaveExists reports whether the host sentinel player save
// (0000...0001.sav) exists - i.e. this world currently has a host.
func hostSaveExists(worldDir string) bool {
	_, err := os.Stat(filepath.Join(worldDir, "Players", uidFilename(HostUUID)))
	return err == nil
}

// writeLevelAtomic writes data to path via a temp file + rename.
func writeLevelAtomic(path string, data []byte) error {
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}



