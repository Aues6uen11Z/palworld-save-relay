// Package palworld implements Palworld save directory detection, host
// switching, backup, and packing on top of the sav engine.
package palworld

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"palworld-save-relay/internal/logger"
	"palworld-save-relay/internal/sav"
)

// HostUID is the GUID Palworld uses for the co-op host player slot.
var HostUUID = sav.UUID{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1, 0, 0, 0} // 00000000-...-000000000001 (mixed-endian: byte 12 set)

// World is a detected Palworld world save folder (host world with Level.sav, or guest-only with just LocalData.sav).
type World struct {
	GUID        string // world folder name
	Path        string // absolute path to the world folder
	ModTime     time.Time
	PlayerCount int    // number of files in Players/
	IsHost      bool   // true when Level.sav exists (this machine hosts the world)
	WorldName   string // in-game world name from LevelMeta.sav (" if unavailable, e.g. guest-only)
	SteamID     string // SteamID64 folder name (which Steam account owns this world)
}

// Player is a detected player within a world.
type Player struct {
	UID        string // formatted PlayerUId
	InstanceID string // formatted InstanceId
	NickName   string
	Level      int
	IsHost     bool
	// IsGuildLeader is true when this player's UID is the admin (leader) of the
	// guild the world's host belongs to. Only meaningful for a host world.
	IsGuildLeader bool
}

// SaveRoot returns the Palworld save root (LocalAppData/Pal/Saved/SaveGames).
// It prefers %LOCALAPPDATA% and falls back to %USERPROFILE%\AppData\Local, so
// an empty LOCALAPPDATA never yields a relative path.
func SaveRoot() (string, error) {
	root := os.Getenv("LOCALAPPDATA")
	if root == "" {
		if home, err := os.UserHomeDir(); err == nil && home != "" {
			root = filepath.Join(home, "AppData", "Local")
			logger.Warnf("SaveRoot: LOCALAPPDATA empty, using fallback %s", root)
		}
	}
	if root == "" {
		return "", errors.New("cannot determine LocalAppData (LOCALAPPDATA and USERPROFILE are both unset)")
	}
	return filepath.Join(root, "Pal", "Saved", "SaveGames"), nil
}

// ListWorlds enumerates world save folders under root (SteamID/WorldGUID).
// A folder with Level.sav is a full host world (IsHost=true). A guest-only
// folder (just LocalData.sav, no Level.sav) is also listed with IsHost=false so
// a non-host player can still pick it to download the cloud save and take over
// hosting. Folders with neither file are skipped (not a world folder).
func ListWorlds(root string) ([]World, error) {
	var worlds []World
	steamDirs, err := os.ReadDir(root)
	if err != nil {
		logger.Errorf("ListWorlds: read root %s: %v", root, err)
		return nil, err
	}
	for _, steamDir := range steamDirs {
		if !steamDir.IsDir() {
			continue
		}
		worldDirs, err := os.ReadDir(filepath.Join(root, steamDir.Name()))
		if err != nil {
			logger.Warnf("ListWorlds: read %s: %v", steamDir.Name(), err)
			continue
		}
		for _, wd := range worldDirs {
			if !wd.IsDir() {
				continue
			}
			worldPath := filepath.Join(root, steamDir.Name(), wd.Name())
			hasLevel := fileExists(filepath.Join(worldPath, "Level.sav"))
			if !hasLevel && !fileExists(filepath.Join(worldPath, "LocalData.sav")) {
				continue // not a world folder
			}
			info, _ := wd.Info()
			worlds = append(worlds, World{
				GUID:        wd.Name(),
				Path:        worldPath,
				ModTime:     info.ModTime(),
				PlayerCount: countPlayers(filepath.Join(worldPath, "Players")),
				IsHost:      hasLevel,
				WorldName:   worldNameFromLevelMeta(worldPath),
				SteamID:     steamDir.Name(),
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
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if filepath.Ext(name) != ".sav" {
			continue
		}
		// Exclude Pal Dimension Storage (_dps.sav); count only player saves.
		if strings.HasSuffix(name, "_dps.sav") {
			continue
		}
		n++
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
// If the host's CSPM key is the sentinel (0001), it resolves the real UID
// from the SteamID folder name via SteamIDToPlayerUUID.
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
	players, err := PlayersFromLevel(gvas)
	if err != nil {
		return nil, err
	}
	hints, custom := sav.PalWorldConfig()

	// Resolve host's real UID: worldDir is SteamID/WorldGUID,
	// so filepath.Dir(worldDir) is the SteamID folder.
	var hostReal *sav.UUID
	for i := range players {
		if !players[i].IsHost {
			continue
		}
		steamIDFolder := filepath.Base(filepath.Dir(worldDir))
		var sid uint64
		if _, err := fmt.Sscanf(steamIDFolder, "%d", &sid); err == nil && sid > 0 {
			uid := SteamIDToPlayerUUID(sid)
			players[i].UID = sav.UUID(uid).String()
			u := sav.UUID(uid)
			hostReal = &u
		}
		break
	}

	// Flag the guild leader within the host's guild, so the UI can mark it.
	if gf, err := sav.ReadGvasFile(gvas, hints, custom); err == nil {
		markGuildLeader(gf, players, hostReal)
	}
	return players, nil
}

// markGuildLeader sets IsGuildLeader on the player who holds the admin (leader)
// role of the guild the host belongs to. hostReal is the host's resolved real
// UID (nil if the SteamID could not be derived); the host is matched in the
// guild by either the host sentinel (0001) or hostReal.
func markGuildLeader(gf *sav.GvasFile, players []Player, hostReal *sav.UUID) {
	g := findHostGuild(gf, hostReal)
	if g == nil {
		return
	}
	admin, _ := g["admin_player_uid"].(*sav.UUID)
	if admin == nil {
		return
	}
	leader := admin.String()
	if admin.Equal(&HostUUID) && hostReal != nil {
		// Leader is the host sentinel; the UI shows the host under its real UID.
		leader = hostReal.String()
	}
	for i := range players {
		if players[i].UID == leader {
			players[i].IsGuildLeader = true
		}
	}
}

// findHostGuild returns the first EPalGroupType::Guild whose members include the
// host sentinel (0001) or hostReal, or nil if the host is not in any guild.
func findHostGuild(gf *sav.GvasFile, hostReal *sav.UUID) map[string]any {
	for _, g := range findAllGuilds(gf) {
		players, _ := g["players"].([]map[string]any)
		for _, p := range players {
			pu, _ := p["player_uid"].(*sav.UUID)
			if pu == nil {
				continue
			}
			if pu.Equal(&HostUUID) || (hostReal != nil && pu.Equal(hostReal)) {
				return g
			}
		}
	}
	return nil
}

// PalCount returns the total number of character entries (pals + players) in
// Level.sav's CharacterSaveParameterMap. Used for relay history diagnostics.
func PalCount(worldDir string) (int, error) {
	levelPath := filepath.Join(worldDir, "Level.sav")
	data, err := os.ReadFile(levelPath)
	if err != nil {
		return 0, fmt.Errorf("palworld: read Level.sav: %w", err)
	}
	gvas, _, err := sav.Decompress(data)
	if err != nil {
		return 0, err
	}
	hints, custom := sav.PalWorldConfig()
	gf, err := sav.ReadGvasFile(gvas, hints, custom)
	if err != nil {
		return 0, err
	}
	wsd := gf.Properties.Get("worldSaveData")
	if wsd == nil {
		return 0, errors.New("palworld: worldSaveData not found")
	}
	cspm := wsd["value"].(sav.PropertyList).Get("CharacterSaveParameterMap")
	if cspm == nil {
		return 0, nil
	}
	entries, ok := cspm["value"].([]map[string]any)
	if !ok {
		return 0, nil
	}
	return len(entries), nil
}

// LocalSteamID returns the SteamID64 of the local player (the save folder name
// under root), used to derive the host's real UID via SteamIDToPlayerUUID.
func LocalSteamID(root string) (uint64, error) {
	entries, err := os.ReadDir(root)
	if err != nil {
		return 0, err
	}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		var sid uint64
		if _, err := fmt.Sscanf(e.Name(), "%d", &sid); err == nil && sid > 0 {
			return sid, nil
		}
	}
	return 0, fmt.Errorf("palworld: no SteamID folder under %s", root)
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// worldNameFromLevelMeta reads the in-game world name from LevelMeta.sav's
// SaveData.WorldName. Returns "" if LevelMeta.sav is missing or unparseable
// (e.g. guest-only worlds stripped to LocalData.sav). Best-effort: any error
// or panic is swallowed so the world list still loads.
func worldNameFromLevelMeta(worldDir string) (name string) {
	defer func() { recover() }()
	data, err := os.ReadFile(filepath.Join(worldDir, "LevelMeta.sav"))
	if err != nil {
		return ""
	}
	gvas, _, err := sav.Decompress(data)
	if err != nil {
		return ""
	}
	hints, custom := sav.PalWorldConfig()
	gf, err := sav.ReadGvasFile(gvas, hints, custom)
	if err != nil {
		return ""
	}
	sd := gf.Properties.Get("SaveData")
	if sd == nil {
		return ""
	}
	pl, ok := sd["value"].(sav.PropertyList)
	if !ok {
		return ""
	}
	wn := pl.Get("WorldName")
	if wn == nil {
		return ""
	}
	name, _ = wn["value"].(string)
	return name
}
