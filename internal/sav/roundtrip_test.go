package sav

import (
	"bytes"
	"testing"
)

// TestGoldRoundTrip is the correctness proof for the save engine: a real save
// must survive sav -> GVAS -> parse -> serialize -> GVAS byte-identically,
// across both compression formats (PlZ zlib and PlM Oodle).
func TestGoldRoundTrip(t *testing.T) {
	fixtures := []string{"level_plz.sav", "player_plz.sav", "level_plm.sav", "player_plm.sav", "localdata_plm.sav"}
	hints, custom := PalWorldConfig()
	for _, name := range fixtures {
		t.Run(name, func(t *testing.T) {
			data := readFixture(t, name)
			gvas, _, err := Decompress(data)
			if err != nil {
				t.Fatalf("Decompress: %v", err)
			}
			gf, err := ReadGvasFile(gvas, hints, custom)
			if err != nil {
				t.Fatalf("ReadGvasFile: %v", err)
			}
			gvas2 := gf.Write(custom)
			if !bytes.Equal(gvas, gvas2) {
				t.Fatalf("GVAS bytes differ: orig=%d new=%d", len(gvas), len(gvas2))
			}
		})
	}
}

// TestTwoHopStability verifies a save survives decompress -> parse -> serialize
// -> compress -> decompress -> parse -> serialize (two hops) unchanged.
func TestTwoHopStability(t *testing.T) {
	hints, custom := PalWorldConfig()
	for _, name := range []string{"player_plz.sav", "player_plm.sav"} {
		t.Run(name, func(t *testing.T) {
			data := readFixture(t, name)
			gvas, h, err := Decompress(data)
			if err != nil {
				t.Fatalf("Decompress: %v", err)
			}
			gf, _ := ReadGvasFile(gvas, hints, custom)
			sav2, _ := Compress(gf.Write(custom), h)
			gvas2, _, err := Decompress(sav2)
			if err != nil {
				t.Fatalf("re-Decompress: %v", err)
			}
			if !bytes.Equal(gvas, gvas2) {
				t.Fatalf("GVAS differs after two-hop")
			}
		})
	}
}

