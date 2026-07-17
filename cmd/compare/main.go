package main

import (
	"encoding/json"
	"fmt"
	"os"

	"palworld-save-relay/internal/palworld"
	"palworld-save-relay/internal/sav"
)

func main() {
	zipPath := `D:\Code\palworld-save-relay\6424B6CA4FED14984B336CB98155AC7B.palrelay.zip`
	tmpDir, _ := os.MkdirTemp("", "fulldiff-*")
	defer os.RemoveAll(tmpDir)
	worldDir := tmpDir + `\world`
	os.MkdirAll(worldDir, 0755)

	zipData, _ := os.ReadFile(zipPath)
	palworld.UnpackWorld(zipData, worldDir)
	data, _ := os.ReadFile(worldDir + `\Level.sav`)
	gvas, _, _ := sav.Decompress(data)
	hints, custom := sav.PalWorldConfig()
	gf, _ := sav.ReadGvasFile(gvas, hints, custom)

	wsd := gf.Properties.Get("worldSaveData")
	cspm := wsd["value"].(sav.PropertyList).Get("CharacterSaveParameterMap")
	entries, _ := cspm["value"].([]map[string]any)

	// ---- Find ALL SheepBall entries ----
	type PalEntry struct {
		Index int
		Key   map[string]any
		SP    sav.PropertyList
	}
	var sheepBalls []PalEntry
	for i, entry := range entries {
		val, _ := entry["value"].(sav.PropertyList)
		raw := val.Get("RawData")
		rv, _ := raw["value"].(map[string]any)
		obj, _ := rv["object"].(sav.PropertyList)
		sp := obj.Get("SaveParameter")
		inner, _ := sp["value"].(sav.PropertyList)

		if cid := inner.Get("CharacterID"); cid != nil && fmt.Sprintf("%v", cid["value"]) == "SheepBall" {
			sheepBalls = append(sheepBalls, PalEntry{i, entry["key"].(map[string]any), inner})
		}
	}

	fmt.Printf("=== Found %d SheepBall entries ===\n\n", len(sheepBalls))

	// ---- Key + OwnerPlayerUId for each ----
	for idx, sb := range sheepBalls {
		ks, _ := sb.Key["value"].(map[string]any)
		fmt.Printf("### SheepBall #%d (CSPM index %d)\n", idx+1, sb.Index)
		fmt.Printf("  PlayerUId:     %v\n", ks["PlayerUId"])
		fmt.Printf("  InstanceId:    %v\n", ks["InstanceId"])

		for _, name := range []string{
			"OwnerPlayerUId", "CharacterID", "NickName", "Level",
			"SlotId", "FullStomach", "FriendshipActiveOtomoSec", "FriendshipBasecampSec",
			"OldOwnerPlayerUIds", "LastNickNameModifierPlayerUid",
		} {
			if p := sb.SP.Get(name); p != nil {
				v := fmt.Sprintf("%v", p["value"])
				if len(v) > 200 {
					v = v[:200] + "..."
				}
				fmt.Printf("  %-35s = %s\n", name, v)
			} else {
				fmt.Printf("  %-35s = (absent)\n", name)
			}
		}
		fmt.Println()
	}

	// ---- Full JSON dump of each SheepBall's SaveParameter ----
	for idx, sb := range sheepBalls {
		b, _ := json.MarshalIndent(sb.SP, "", "  ")
		outPath := tmpDir + fmt.Sprintf(`\sheepball_%d.json`, idx+1)
		os.WriteFile(outPath, b, 0644)
		fmt.Printf("JSON: %s\n", outPath)
	}

	// ---- RawData level keys for each SheepBall ----
	fmt.Println("\n=== RawData level keys ===")
	for idx, sb := range sheepBalls {
		val := entries[sb.Index]["value"].(sav.PropertyList)
		raw := val.Get("RawData")
		rv, _ := raw["value"].(map[string]any)
		fmt.Printf("SheepBall #%d: ", idx+1)
		for k, v := range rv {
			if k == "object" {
				continue
			}
			fmt.Printf("%s=%v  ", k, v)
		}
		fmt.Println()
	}

	// ---- GroupSaveDataMap ----
	fmt.Println("\n=== GroupSaveDataMap ===")
	gsdm := wsd["value"].(sav.PropertyList).Get("GroupSaveDataMap")
	gsdEntries, _ := gsdm["value"].([]map[string]any)

	for i, ge := range gsdEntries {
		gv, _ := ge["value"].(sav.PropertyList)
		raw := gv.Get("RawData")
		if raw == nil {
			continue
		}
		inner, _ := raw["value"].(map[string]any)
		if inner == nil {
			continue
		}

		gtype, _ := inner["group_type"].(string)
		gname, _ := inner["guild_name"].(string)
		gid := inner["group_id"]
		admin := inner["admin_player_uid"]

		fmt.Printf("\n  Group #%d: type=%s name=%q gid=%v admin=%v\n", i, gtype, gname, gid, admin)

		if ich, ok := inner["individual_character_handle_ids"].([]any); ok {
			fmt.Printf("    individual_character_handle_ids (%d):\n", len(ich))
			for j, h := range ich {
				m, _ := h.(map[string]any)
				fmt.Printf("      [%d] guid=%v instance_id=%v\n", j, m["guid"], m["instance_id"])
			}
		}

		if players, ok := inner["players"].([]map[string]any); ok {
			fmt.Printf("    players (%d):\n", len(players))
			for _, p := range players {
				pi, _ := p["player_info"].(map[string]any)
				fmt.Printf("      uid=%v name=%q\n", p["player_uid"], pi["player_name"])
			}
		}

		if puid, ok := inner["player_uid"]; ok {
			fmt.Printf("    player_uid: %v\n", puid)
		}
	}

	// ---- Full JSON dump of GroupSaveDataMap ----
	gsdmJSON, _ := json.MarshalIndent(gsdm, "", "  ")
	gsdmPath := tmpDir + `\GroupSaveDataMap.json`
	os.WriteFile(gsdmPath, gsdmJSON, 0644)
	fmt.Printf("\nGroupSaveDataMap JSON: %s\n", gsdmPath)
}

func trunc(s string, n int) string {
	if len(s) > n {
		return s[:n] + "..."
	}
	return s
}
