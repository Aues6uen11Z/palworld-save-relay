package palworld

import (
	"bytes"
	"os"
	"path/filepath"
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


// TestConvertHostLevel_PalsStayOnHostSlot verifies the refactored host
// step-down moves ONLY the host player's identity off the sentinel slot; every
// pal's CSPM bucket and ICH guid stays on 0001 so the next host inherits them
// (matching an official host world), instead of dragging all pals into the old
// host's personal bucket.
func TestConvertHostLevel_PalsStayOnHostSlot(t *testing.T) {
	levelData := readSavFixtureX(t, "level_plm.sav")
	if len(levelData) == 0 {
		return
	}
	dir := t.TempDir()
	levelPath := filepath.Join(dir, "Level.sav")
	if err := os.WriteFile(levelPath, levelData, 0o644); err != nil {
		t.Fatal(err)
	}
	hints, custom := sav.PalWorldConfig()

	// Snapshot the CSPM bucket distribution before conversion.
	before := cspmBucketDist(t, levelPath, hints, custom)
	hostBefore := before[HostUUID.String()]

	if err := convertHostLevel(levelPath, HostUUID, fakeUID, hints, custom); err != nil {
		t.Fatalf("convertHostLevel: %v", err)
	}
	after := cspmBucketDist(t, levelPath, hints, custom)

	// Host player (1 entry) moved 0001 -> fakeUID.
	if after[fakeUID.String()] != 1 {
		t.Errorf("host player count under fakeUID = %d, want 1", after[fakeUID.String()])
	}
	if after[HostUUID.String()] != hostBefore-1 {
		t.Errorf("0001 bucket = %d, want %d (only the host player should have left)",
			after[HostUUID.String()], hostBefore-1)
	}
	// No bucket other than 0001/fakeUID changed (pals did not scatter).
	for uid, n := range before {
		if uid == HostUUID.String() {
			continue
		}
		if after[uid] != n {
			t.Errorf("bucket %s changed: %d -> %d (pals should not move)", uid, n, after[uid])
		}
	}
	t.Logf("before 0001=%d after 0001=%d fakeUID=%d", hostBefore, after[HostUUID.String()], after[fakeUID.String()])
}

func cspmBucketDist(t *testing.T, path string, hints map[string]string, custom map[string]sav.CustomProperty) map[string]int {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	gvas, _, err := sav.Decompress(data)
	if err != nil {
		t.Fatal(err)
	}
	gf, err := sav.ReadGvasFile(gvas, hints, custom)
	if err != nil {
		t.Fatal(err)
	}
	wsd := gf.Properties.Get("worldSaveData")
	pl := wsd["value"].(sav.PropertyList)
	cspm := pl.Get("CharacterSaveParameterMap")
	entries, _ := cspm["value"].([]map[string]any)
	dist := map[string]int{}
	for _, e := range entries {
		key, _ := e["key"].(sav.PropertyList)
		if p := key.Get("PlayerUId"); p != nil {
			if g, ok := p["value"].(*sav.UUID); ok && g != nil {
				dist[g.String()]++
			}
		}
	}
	return dist
}

