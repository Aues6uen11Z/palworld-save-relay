package palworld

import (
	"testing"

	"palworld-save-relay/internal/sav"
)

// TestDeepReplace_GuildUIDs verifies that UID fields inside the structured
// guild data produced by groupDecodeBytes (bare *UUID keyed by name in a
// map[string]any) are actually replaced by deepReplace. The old deepReplace
// only followed PropertyList entry names and silently skipped these, leaving
// guild admin/member UIDs stale after a host swap.
func TestDeepReplace_GuildUIDs(t *testing.T) {
	from := HostUUID
	to := fakeUID
	other := sav.UUID{0x11, 0x22, 0x33, 0x44, 0x55, 0x66, 0x77, 0x88,
		0, 0, 0, 0, 0, 0, 0, 0}

	// Mirror the shape groupDecodeBytes returns for EPalGroupType::Guild.
	guild := map[string]any{
		"group_type": "EPalGroupType::Guild",
		"group_id":   &sav.UUID{0xAA, 0xBB}, // not a player UID; must not change
		"guild_name": "Test Guild",
		"last_guild_name_modifier_player_uid": &from,
		"admin_player_uid":                    &from,
		"players": []map[string]any{
			{"player_uid": &from, "player_info": map[string]any{"player_name": "Host"}},
			{"player_uid": &other, "player_info": map[string]any{"player_name": "Guest"}},
		},
	}

	deepReplace(guild, from, to)

	want := func(label string, got *sav.UUID, exp sav.UUID) {
		t.Helper()
		if !got.Equal(&exp) {
			t.Errorf("%s = %s, want %s", label, got, exp)
		}
	}

	want("admin_player_uid", guild["admin_player_uid"].(*sav.UUID), to)
	want("last_guild_name_modifier_player_uid",
		guild["last_guild_name_modifier_player_uid"].(*sav.UUID), to)

	players := guild["players"].([]map[string]any)
	want("players[0].player_uid", players[0]["player_uid"].(*sav.UUID), to)
	want("players[1].player_uid", players[1]["player_uid"].(*sav.UUID), other) // unchanged

	// group_id is NOT a player UID — it must be left alone.
	if gid := guild["group_id"].(*sav.UUID); gid.Equal(&to) {
		t.Errorf("group_id should not be replaced")
	}
}

// TestDeepReplace_IndependentGuildUID covers the EPalGroupType::IndependentGuild
// shape, which stores the owner as a bare "player_uid" *UUID.
func TestDeepReplace_IndependentGuildUID(t *testing.T) {
	from := HostUUID
	to := fakeUID

	guild := map[string]any{
		"group_type":  "EPalGroupType::IndependentGuild",
		"player_uid":  &from,
		"player_info": map[string]any{"player_name": "Host"},
		"guild_name":  "Solo Guild",
	}

	deepReplace(guild, from, to)

	if g := guild["player_uid"].(*sav.UUID); !g.Equal(&to) {
		t.Errorf("player_uid not replaced: %s", g)
	}
}
