package palworld_test

import (
	"os"
	"testing"

	"palworld-save-relay/internal/palworld"
	"palworld-save-relay/internal/sav"
)

func readSavFixture(t *testing.T, name string) []byte {
	t.Helper()
	b, err := os.ReadFile("../sav/testdata/" + name)
	if os.IsNotExist(err) {
		t.Skipf("fixture missing: %s", name)
	}
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	return b
}

func TestPlayersFromLevel(t *testing.T) {
	data := readSavFixture(t, "level_plm.sav")
	gvas, _, err := sav.Decompress(data)
	if err != nil {
		t.Fatalf("Decompress: %v", err)
	}
	players, err := palworld.PlayersFromLevel(gvas)
	if err != nil {
		t.Fatalf("PlayersFromLevel: %v", err)
	}
	if len(players) != 3 {
		t.Fatalf("players = %d, want 3 (got %+v)", len(players), players)
	}
	var host int
	for _, p := range players {
		if p.IsHost {
			host++
		}
		if p.NickName == "" {
			t.Errorf("player %s has empty NickName", p.UID)
		}
		if p.UID == "" || p.InstanceID == "" {
			t.Errorf("player missing UID/InstanceID: %+v", p)
		}
	}
	if host != 1 {
		t.Errorf("host count = %d, want 1", host)
	}
	t.Logf("players: %+v", players)
}
