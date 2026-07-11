package palworld

import (
	"bytes"
	"testing"

	"palworld-save-relay/internal/sav"
)

func readLevelFixture(t *testing.T) []byte {
	t.Helper()
	return readSavFixtureX(t, "level_plm.sav")
}

func TestSwapLevelSav_Reversible(t *testing.T) {
	gvas, _, _ := sav.Decompress(readLevelFixture(t))
	hostInst, guestInst, guestUID := findHostGuest(t, gvas)
	hints, custom := sav.PalWorldConfig() // character decoded for deepSwap

	gf1, _ := sav.ReadGvasFile(gvas, hints, custom)
	SwapLevelSav(gf1, hostInst, guestInst, HostUUID, *guestUID)
	out1 := gf1.Write(custom)

	gf2, _ := sav.ReadGvasFile(out1, hints, custom)
	SwapLevelSav(gf2, guestInst, hostInst, HostUUID, *guestUID)
	out2 := gf2.Write(custom)

	if !bytes.Equal(gvas, out2) {
		for i := 0; i < len(gvas) && i < len(out2); i++ {
			if gvas[i] != out2[i] {
				t.Fatalf("Level.sav not reversible @ %d", i)
			}
		}
		t.Fatalf("Level.sav not reversible (len %d vs %d)", len(gvas), len(out2))
	}
}

func TestSwapPlayerSav_Reversible(t *testing.T) {
	orig := readSavFixtureX(t, "player_plm.sav")
	gvas, _, err := sav.Decompress(orig)
	if err != nil {
		t.Fatalf("decompress: %v", err)
	}
	hints, custom := sav.PalWorldConfig()
	a := HostUUID
	b := sav.UUID{0x91, 0x23, 0x03, 0x69, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0}

	gf1, _ := sav.ReadGvasFile(gvas, hints, custom)
	SwapPlayerSav(gf1, a, b)
	out1 := gf1.Write(custom)

	gf2, _ := sav.ReadGvasFile(out1, hints, custom)
	SwapPlayerSav(gf2, a, b)
	out2 := gf2.Write(custom)

	if !bytes.Equal(gvas, out2) {
		t.Fatalf("player .sav not reversible")
	}
}
