// Package palworld implements Palworld save directory detection, host
// switching, backup, and packing on top of the sav engine.
package palworld

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"palworld-save-relay/internal/sav"
)

// HostUID is the GUID Palworld uses for the co-op host player slot.
var HostUUID = sav.UUID{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1, 0, 0, 0} // 00000000-...-000000000001 (mixed-endian: byte 12 set)

// World is a detected Palworld world save.
type World struct {
	GUID        string // world folder name
	Path        string // absolute path to the world folder
	ModTime     time.Time
	PlayerCount int // number of files in Players/
}

// Player is a detected player within a world.
type Player struct {
	UID        string // formatted PlayerUId
	InstanceID string // formatted InstanceId
	NickName   string
	Level      int
	IsHost     bool
}

// SaveRoot returns the Palworld save root (LocalAppData/Pal/Saved/SaveGames).
func SaveRoot() (string, error) {
	root := os.Getenv("LOCALAPPDATA")
	if root == "" {
		return "", errors.New("LOCALAPPDATA not set")
	}
	return filepath.Join(root, "Pal", "Saved", "SaveGames"), nil
}

// ListWorlds enumerates world save folders under root (SteamID/WorldGUID).
func ListWorlds(root string) ([]World, error) {
	var worlds []World
	steamDirs, err := os.ReadDir(root)
	if err != nil {
		return nil, err
	}
	for _, steamDir := range steamDirs {
		if !steamDir.IsDir() {
			continue
		}
		worldDirs, err := os.ReadDir(filepath.Join(root, steamDir.Name()))
		if err != nil {
			continue
		}
		for _, wd := range worldDirs {
			if !wd.IsDir() {
				continue
			}
			worldPath := filepath.Join(root, steamDir.Name(), wd.Name())
			if _, err := os.Stat(filepath.Join(worldPath, "Level.sav")); err != nil {
				continue // not a world folder
			}
			info, _ := wd.Info()
			worlds = append(worlds, World{
				GUID:        wd.Name(),
				Path:        worldPath,
				ModTime:     info.ModTime(),
				PlayerCount: countPlayers(filepath.Join(worldPath, "Players")),
			})
		}
	}
	return worlds, nil
}

func countPlayers(playersDir string) int {
	entries, err := os.ReadDir(playersDir)
	if err != nil {
		return 0
	}
	n := 0
	for _, e := range entries {
		if !e.IsDir() && filepath.Ext(e.Name()) == ".sav" {
			n++
		}
	}
	return n
}

// PlayersFromLevel extracts the player list (IsPlayer=true) from a Level.sav's
// GVAS bytes. This is the core, testable logic; ListPlayers wraps it with IO.
func PlayersFromLevel(gvas []byte) ([]Player, error) {
	hints, custom := sav.PalWorldConfig()
	gf, err := sav.ReadGvasFile(gvas, hints, custom)
	if err != nil {
		return nil, err
	}
	wsd := gf.Properties.Get("worldSaveData")
	if wsd == nil {
		return nil, errors.New("palworld: worldSaveData not found")
	}
	cspm := wsd["value"].(sav.PropertyList).Get("CharacterSaveParameterMap")
	if cspm == nil {
		return nil, errors.New("palworld: CharacterSaveParameterMap not found")
	}
	entries, ok := cspm["value"].([]map[string]any)
	if !ok {
		return nil, errors.New("palworld: CharacterSaveParameterMap has unexpected shape")
	}
	var players []Player
	for _, e := range entries {
		if p, ok := playerFromEntry(e); ok {
			players = append(players, p)
		}
	}
	return players, nil
}

// playerFromEntry reads one CSPM entry, returning the player if IsPlayer=true.
// Malformed entries are skipped rather than aborting the whole list.
func playerFromEntry(e map[string]any) (Player, bool) {
	defer func() { recover() }() // skip malformed entries gracefully

	key, _ := e["key"].(sav.PropertyList)
	val, _ := e["value"].(sav.PropertyList)
	if key == nil || val == nil {
		return Player{}, false
	}
	raw := val.Get("RawData")
	if raw == nil {
		return Player{}, false
	}
	rv, _ := raw["value"].(map[string]any)
	obj, _ := rv["object"].(sav.PropertyList)
	sp := obj.Get("SaveParameter")
	if sp == nil {
		return Player{}, false
	}
	inner, _ := sp["value"].(sav.PropertyList)
	if inner == nil {
		return Player{}, false
	}
	ip := inner.Get("IsPlayer")
	if ip == nil {
		return Player{}, false
	}
	isPlayer, _ := ip["value"].(bool)
	if !isPlayer {
		return Player{}, false
	}

	p := Player{}
	if v := key.Get("PlayerUId"); v != nil {
		if g, ok := v["value"].(*sav.UUID); ok {
			p.UID = g.String()
			p.IsHost = *g == HostUUID
		}
	}
	if v := key.Get("InstanceId"); v != nil {
		if g, ok := v["value"].(*sav.UUID); ok {
			p.InstanceID = g.String()
		}
	}
	if v := inner.Get("NickName"); v != nil {
		p.NickName, _ = v["value"].(string)
	}
	if v := inner.Get("Level"); v != nil {
		if inner2, ok := v["value"].(map[string]any); ok {
			if b, ok := inner2["value"].(byte); ok {
				p.Level = int(b)
			}
		}
	}
	return p, true
}

// ListPlayers reads a world's Level.sav and returns its players.
func ListPlayers(worldDir string) ([]Player, error) {
	levelPath := filepath.Join(worldDir, "Level.sav")
	data, err := os.ReadFile(levelPath)
	if err != nil {
		return nil, fmt.Errorf("palworld: read Level.sav: %w", err)
	}
	gvas, _, err := sav.Decompress(data)
	if err != nil {
		return nil, err
	}
	return PlayersFromLevel(gvas)
}
