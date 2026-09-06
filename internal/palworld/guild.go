package palworld

import (
	"fmt"
	"os"
	"path/filepath"

	"palworld-save-relay/internal/apperr"
	"palworld-save-relay/internal/logger"
	"palworld-save-relay/internal/sav"
)

// BecomeGuildLeader makes the current host the leader of the guild they belong
// to in worldDir. It is the minimal "claim leadership" operation: it sets the
// guild's admin_player_uid to the host's in-save UID and swaps the per-member
// leader flag (_u8_flag, 1 = leader) between the new and old leader. The world
// is backed up first and restored on any failure.
func BecomeGuildLeader(worldDir string) error {
	guid := filepath.Base(worldDir)
	logger.Infof("BecomeGuildLeader: world=%s", guid)
	if err := assertGameNotRunning(); err != nil {
		logger.Errorf("BecomeGuildLeader: world=%s game running: %v", guid, err)
		return err
	}
	backupPath, err := BackupWorld(worldDir)
	if err != nil {
		logger.Errorf("BecomeGuildLeader: world=%s backup failed: %v", guid, err)
		return fmt.Errorf("palworld: backup: %w", err)
	}
	if err := becomeGuildLeaderImpl(worldDir); err != nil {
		logger.Errorf("BecomeGuildLeader: world=%s impl failed, rolling back from %s: %v", guid, backupPath, err)
		if rbErr := RestoreFromBackup(worldDir, backupPath); rbErr != nil {
			return fmt.Errorf("become leader failed: %v; rollback also failed: %v; backup at %s, please restore manually", err, rbErr, backupPath)
		}
		return fmt.Errorf("become leader failed (rolled back from %s): %w", backupPath, err)
	}
	logger.Infof("BecomeGuildLeader: world=%s done (backup=%s)", guid, backupPath)
	return nil
}

func becomeGuildLeaderImpl(worldDir string) error {
	levelPath := filepath.Join(worldDir, "Level.sav")
	data, err := os.ReadFile(levelPath)
	if err != nil {
		return apperr.Wrap(apperr.FileRead, err)
	}
	gvas, hdr, err := sav.Decompress(data)
	if err != nil {
		return err
	}
	hints, custom := sav.PalWorldConfig()
	gf, err := sav.ReadGvasFile(gvas, hints, custom)
	if err != nil {
		return err
	}

	hostReal := hostServerUID(worldDir)
	guild := findHostGuild(gf, hostReal)
	if guild == nil {
		return apperr.New(apperr.NotInGuild, "")
	}
	selfUID := selfGuildUID(guild, hostReal)
	if selfUID == nil {
		return apperr.New(apperr.NotInGuild, "")
	}
	if admin, ok := guild["admin_player_uid"].(*sav.UUID); ok && admin.Equal(selfUID) {
		return apperr.New(apperr.AlreadyLeader, "")
	}
	setGuildLeader(guild, selfUID)

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
		return apperr.Wrap(apperr.FileWrite, err)
	}
	return os.Rename(tmp, levelPath)
}

// hostServerUID resolves the host's real UID from the SteamID folder name, or
// nil if it cannot be derived. Used to match the host inside guild data.
func hostServerUID(worldDir string) *sav.UUID {
	steamIDFolder := filepath.Base(filepath.Dir(worldDir))
	var sid uint64
	if _, err := fmt.Sscanf(steamIDFolder, "%d", &sid); err == nil && sid > 0 {
		u := sav.UUID(SteamIDToPlayerUUID(sid))
		return &u
	}
	return nil
}

// selfGuildUID returns the host's player_uid as recorded in the given guild
// (the host sentinel, or hostReal if the guild stores the real UID). Returns
// nil if the host is not a member of this guild.
func selfGuildUID(guild map[string]any, hostReal *sav.UUID) *sav.UUID {
	players, _ := guild["players"].([]map[string]any)
	for _, p := range players {
		pu, _ := p["player_uid"].(*sav.UUID)
		if pu == nil {
			continue
		}
		if pu.Equal(&HostUUID) || (hostReal != nil && pu.Equal(hostReal)) {
			return pu
		}
	}
	return nil
}

// setGuildLeader promotes newUID to leader of the guild: it sets admin_player_uid
// and, when the guild uses the v1 member flag, promotes the new leader's
// _u8_flag to 1 and demotes every current leader (flag == 1) to the value the
// new leader previously held. In the legacy format (no per-member flag) only
// admin_player_uid is updated.
func setGuildLeader(guild map[string]any, newUID *sav.UUID) {
	players, _ := guild["players"].([]map[string]any)
	newIndex := -1
	var oldFlag byte
	hasFlag := false
	for i, p := range players {
		pu, _ := p["player_uid"].(*sav.UUID)
		if pu != nil && pu.Equal(newUID) {
			newIndex = i
			if f, ok := p["_u8_flag"].(byte); ok {
				oldFlag = f
				hasFlag = true
			}
		}
	}
	if newIndex < 0 {
		return
	}
	if hasFlag {
		players[newIndex]["_u8_flag"] = byte(1)
		for i, p := range players {
			if i == newIndex {
				continue
			}
			if f, ok := p["_u8_flag"].(byte); ok && f == 1 {
				p["_u8_flag"] = oldFlag
			}
		}
	}
	guild["admin_player_uid"] = uidPtr(*newUID)
}
