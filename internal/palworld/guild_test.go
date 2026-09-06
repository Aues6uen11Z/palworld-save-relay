package palworld

import (
	"testing"

	"palworld-save-relay/internal/sav"
)

func TestSetGuildLeader_SwapsFlags(t *testing.T) {
	a := sav.UUID{0x0A}
	b := sav.UUID{0x0B}
	c := sav.UUID{0x0C}
	guild := map[string]any{
		"admin_player_uid": &a,
		"players": []map[string]any{
			{"player_uid": &a, "_u8_flag": byte(1)}, // current leader
			{"player_uid": &b, "_u8_flag": byte(2)}, // promoted to leader
			{"player_uid": &c, "_u8_flag": byte(2)},
		},
	}

	setGuildLeader(guild, &b)

	admin, _ := guild["admin_player_uid"].(*sav.UUID)
	if admin == nil || *admin != b {
		t.Fatalf("admin = %v, want %v", admin, b)
	}
	players := guild["players"].([]map[string]any)
	got := map[string]byte{}
	for _, p := range players {
		uid := ""
		if v, _ := p["player_uid"].(*sav.UUID); v != nil {
			uid = v.String()
		}
		f, _ := p["_u8_flag"].(byte)
		got[uid] = f
	}
	if got[a.String()] != 2 {
		t.Errorf("old leader flag = %d, want 2 (demoted)", got[a.String()])
	}
	if got[b.String()] != 1 {
		t.Errorf("new leader flag = %d, want 1", got[b.String()])
	}
	if got[c.String()] != 2 {
		t.Errorf("unchanged member flag = %d, want 2", got[c.String()])
	}
}

func TestSetGuildLeader_NoFlagOnlyAdmin(t *testing.T) {
	a := sav.UUID{0x0A}
	b := sav.UUID{0x0B}
	guild := map[string]any{
		"admin_player_uid": &a,
		"players": []map[string]any{
			{"player_uid": &a}, // legacy format: no _u8_flag
			{"player_uid": &b},
		},
	}

	setGuildLeader(guild, &b)

	admin, _ := guild["admin_player_uid"].(*sav.UUID)
	if admin == nil || *admin != b {
		t.Fatalf("admin = %v, want %v", admin, b)
	}
	players := guild["players"].([]map[string]any)
	if _, ok := players[0]["_u8_flag"]; ok {
		t.Errorf("legacy member should not gain a _u8_flag, but it did")
	}
	if _, ok := players[1]["_u8_flag"]; ok {
		t.Errorf("legacy promoted member should not gain a _u8_flag, but it did")
	}
}
