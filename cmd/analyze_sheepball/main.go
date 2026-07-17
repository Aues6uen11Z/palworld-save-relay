package main

import (
	"archive/zip"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"palworld-save-relay/internal/sav"
)

func main() {
	hints, custom := sav.PalWorldConfig()

	type saveInfo struct {
		name string
		path string
		isZip bool
	}

	saves := []saveInfo{
		{"正常存档(原始)", `D:\Code\palworld-save-relay\debug\正常存档.zip`, true},
		{"中间态v5", `D:\Code\palworld-save-relay\debug\中间态_修复v5.zip`, true},
		{"v5导入", `D:\Code\palworld-save-relay\debug\v5导入.zip`, true},
		{"v5进游戏", `D:\Code\palworld-save-relay\debug\v5进游戏.zip`, true},
		{"云存档(GYF)", `D:\Code\palworld-save-relay\debug\1784288806413__GYF.zip`, true},
	}

	for _, s := range saves {
		fmt.Printf("\n========================================\n")
		fmt.Printf("=== %s ===\n", s.name)
		fmt.Printf("========================================\n")
		analyzeSave(s.path, hints, custom, s.isZip)
	}
}

func analyzeSave(path string, hints map[string]string, custom map[string]sav.CustomProperty, isZip bool) {
	var levelData []byte
	var playerFiles map[string][]byte

	if isZip {
		r, _ := zip.OpenReader(path)
		playerFiles = make(map[string][]byte)
		for _, f := range r.File {
			if f.FileInfo().IsDir() {
				continue
			}
			rc, _ := f.Open()
			data, _ := io.ReadAll(rc)
			rc.Close()
			base := filepath.Base(f.Name)
			dir := filepath.Dir(f.Name)
			if base == "Level.sav" && levelData == nil {
				levelData = data
			}
			// Player files: .sav files directly in "Players/" (not backup/)
			if filepath.Ext(f.Name) == ".sav" && dir == "Players" {
				playerFiles[base] = data
			}
		}
		r.Close()
	} else {
		filepath.Walk(path, func(p string, info os.FileInfo, err error) error {
			if err != nil || info.IsDir() {
				return nil
			}
			data, _ := os.ReadFile(p)
			if filepath.Base(p) == "Level.sav" {
				levelData = data
			}
			if filepath.Ext(p) == ".sav" && filepath.Dir(p) != path {
				rel, _ := filepath.Rel(path, p)
				if playerFiles == nil {
					playerFiles = make(map[string][]byte)
				}
				playerFiles[rel] = data
			}
			return nil
		})
	}

	if levelData == nil {
		fmt.Println("  Level.sav NOT FOUND!")
		return
	}

	gvas, _, err := sav.Decompress(levelData)
	if err != nil {
		fmt.Printf("  Decompress error: %v\n", err)
		return
	}
	gf, err := sav.ReadGvasFile(gvas, hints, custom)
	if err != nil {
		fmt.Printf("  Parse error: %v\n", err)
		return
	}
	wsd := gf.Properties.Get("worldSaveData")
	pl := wsd["value"].(sav.PropertyList)

	dpoi := sav.UUID{0xb8, 0x82, 0x0d, 0x7a, 0, 0, 0, 0, 0, 1, 0, 0, 0, 0, 0, 0}
	dpoiPrefix := fmt.Sprintf("%v", dpoi)[:8]
	sentinelPrefix := "00000000"

	// === CSPM ===
	fmt.Println("  --- CSPM Key PlayerUId 分布 ---")
	cspm := pl.Get("CharacterSaveParameterMap")
	cspmEntries, _ := cspm["value"].([]map[string]any)
	keyDist := map[string]int{}
	totalCSPM := 0
	for _, e := range cspmEntries {
		key, _ := e["key"].(sav.PropertyList)
		if p := key.Get("PlayerUId"); p != nil {
			uid := fmt.Sprintf("%v", p["value"])[:8]
			keyDist[uid]++
			totalCSPM++
		}
	}
	fmt.Printf("  Total CSPM: %d\n", totalCSPM)
	for k, v := range keyDist {
		label := k
		if k == dpoiPrefix {
			label = k + " (Dpoi原房主)"
		} else if k == sentinelPrefix {
			label = k + " (哨兵/新房主)"
		}
		fmt.Printf("    %s: %d\n", label, v)
	}

	// === Guild ===
	fmt.Println("  --- Guild ---")
	gsdm := pl.Get("GroupSaveDataMap")
	gsdEntries, _ := gsdm["value"].([]map[string]any)
	for _, ge := range gsdEntries {
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
		if gtype != "EPalGroupType::Guild" {
			continue
		}
		gn, _ := inner["group_name"].(string)
		fmt.Printf("  group_name=%s\n", gn)
		fmt.Printf("  admin_player_uid=%v\n", inner["admin_player_uid"])
		players, _ := inner["players"].([]map[string]any)
		fmt.Printf("  Players=%d:\n", len(players))
		for j, p := range players {
			fmt.Printf("    [%d] uid=%v flag=%v\n", j, p["player_uid"], p["_u8_flag"])
		}
		ich, _ := inner["individual_character_handle_ids"].([]any)
		guidCnt := map[string]int{}
		for _, h := range ich {
			m, _ := h.(map[string]any)
			guid := fmt.Sprintf("%v", m["guid"])[:8]
			guidCnt[guid]++
		}
		fmt.Printf("  ICH=%d: %v\n", len(ich), guidCnt)
	}

	// === Player files ===
	fmt.Printf("  --- Player files: %d ---\n", len(playerFiles))
	for name := range playerFiles {
		fmt.Printf("    %s\n", name)
	}
}

func extractToTemp(zipPath string) string {
	tmpDir, _ := os.MkdirTemp("", "palrelay-cmp-*")
	r, _ := zip.OpenReader(zipPath)
	defer r.Close()
	for _, f := range r.File {
		if f.FileInfo().IsDir() {
			continue
		}
		rc, _ := f.Open()
		data, _ := io.ReadAll(rc)
		rc.Close()
		outPath := filepath.Join(tmpDir, filepath.FromSlash(f.Name))
		os.MkdirAll(filepath.Dir(outPath), 0o755)
		os.WriteFile(outPath, data, 0o644)
	}
	return tmpDir
}
