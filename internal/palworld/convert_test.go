package palworld

import (
	"bytes"
	"testing"

	"palworld-save-relay/internal/sav"
)

var fakeUID = sav.UUID{0xDE, 0xAD, 0xBE, 0xEF, 0xCA, 0xFE, 0xBA, 0xBE, 0, 0, 0, 0, 0, 0, 0, 0}

func TestConvertGvas_Reversible(t *testing.T) {
	for _, name := range []string{"level_plm.sav", "player_plm.sav"} {
		t.Run(name, func(t *testing.T) {
			orig := readSavFixtureX(t, name)
			gvas, _, err := sav.Decompress(orig)
			if err != nil {
				t.Fatalf("decompress: %v", err)
			}
			hints, custom := sav.PalWorldConfig()

			gf1, _ := sav.ReadGvasFile(gvas, hints, custom)
			ConvertGvas(gf1, HostUUID, fakeUID)
			out1 := gf1.Write(custom)

			gf2, _ := sav.ReadGvasFile(out1, hints, custom)
			ConvertGvas(gf2, fakeUID, HostUUID)
			out2 := gf2.Write(custom)

			if !bytes.Equal(gvas, out2) {
				for i := 0; i < len(gvas) && i < len(out2); i++ {
					if gvas[i] != out2[i] {
						end := i + 16
						if end > len(gvas) {
							end = len(gvas)
						}
						end2 := i + 16
						if end2 > len(out2) {
							end2 = len(out2)
						}
						var a, b sav.UUID
						copy(a[:], gvas[i:end])
						copy(b[:], out2[i:end2])
						t.Fatalf("%s not reversible @ %d: orig=%s out2=%s", name, i, a.String(), b.String())
					}
				}
				t.Fatalf("%s not reversible (len %d vs %d)", name, len(gvas), len(out2))
			}
		})
	}
}
