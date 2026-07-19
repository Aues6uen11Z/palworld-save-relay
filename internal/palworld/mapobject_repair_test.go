package palworld

import (
	"os"
	"testing"

	"palworld-save-relay/internal/sav"
)

func readLevelFixturePLM(t *testing.T) []byte {
	t.Helper()
	b, err := os.ReadFile("../sav/testdata/level_plm.sav")
	if os.IsNotExist(err) {
		t.Skipf("fixture missing: level_plm.sav")
	}
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	return b
}

func parseFixturePLM(t *testing.T) (*sav.GvasFile, sav.SAVHeader, map[string]string, map[string]sav.CustomProperty) {
	t.Helper()
	data := readLevelFixturePLM(t)
	gvas, hdr, err := sav.Decompress(data)
	if err != nil {
		t.Fatalf("decompress: %v", err)
	}
	hints, custom := sav.PalWorldConfig()
	gf, err := sav.ReadGvasFile(gvas, hints, custom)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	return gf, hdr, hints, custom
}

// TestRepairMapObjectBuilders corrupts two facility builders (one mashup, one
// orphan), repairs, and verifies: corrupt/orphan -> host sentinel; valid player
// builders and all-zero builders preserved; round-trip intact; idempotent.
func TestRepairMapObjectBuilders(t *testing.T) {
	gf, hdr, hints, custom := parseFixturePLM(t)
	skipMap := findMapObjectSkipMap(gf.Properties)
	if skipMap == nil {
		t.Skip("fixture has no MapObjectSaveData")
	}
	blob, _ := skipMap["value"].([]byte)
	parsed, err := parseMapObjectArray(blob, hints, custom)
	if err != nil {
		t.Fatal(err)
	}
	values := parsed["values"].([]any)
	if len(values) < 3 {
		t.Skip("fixture has too few facilities")
	}
	valid := collectCurrentPlayerUIDs(gf)

	// A corrupted mashup (8 nonzero bytes) and a player-shaped orphan.
	mashup := sav.UUID{0xaa, 0xbb, 0xcc, 0xdd, 0x11, 0x22, 0x33, 0x44, 0, 0, 0, 0, 0, 0, 0, 0}
	orphan := sav.UUID{0x99, 0x88, 0x77, 0x66, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0}
	corruptBuilder(t, values[0].(sav.PropertyList), mashup)
	corruptBuilder(t, values[1].(sav.PropertyList), orphan)
	skipMap["value"] = encodeMapObjectArray(parsed, custom)

	// Find a facility with a valid (current-player, non-zero) builder to verify
	// it is preserved. Also find an all-zero builder if present.
	var preserveIdx int = -1
	var preserveUID sav.UUID
	var zeroIdx int = -1
	var zero sav.UUID
	for i, v := range values {
		if i < 2 {
			continue
		}
		rd := findModelRawDataBytes(v.(sav.PropertyList))
		if len(rd) < mapObjectBuilderOffset+16 {
			continue
		}
		var u sav.UUID
		copy(u[:], rd[mapObjectBuilderOffset:mapObjectBuilderOffset+16])
		if u == zero {
			if zeroIdx < 0 {
				zeroIdx = i
			}
			continue
		}
		if valid[u] && preserveIdx < 0 {
			preserveIdx = i
			preserveUID = u
		}
	}

	n, err := RepairMapObjectBuilders(gf, hints, custom)
	if err != nil {
		t.Fatalf("RepairMapObjectBuilders: %v", err)
	}
	if n < 2 {
		t.Fatalf("repaired %d, want >= 2 (the two corrupted)", n)
	}

	// Re-read the repaired blob.
	parsed2 := mustReparseMapObject(t, gf, hints, custom)
	values2 := parsed2["values"].([]any)
	checkBuilder(t, values2[0].(sav.PropertyList), HostUUID, "mashup facility")
	checkBuilder(t, values2[1].(sav.PropertyList), HostUUID, "orphan facility")
	if preserveIdx >= 0 {
		checkBuilder(t, values2[preserveIdx].(sav.PropertyList), preserveUID, "valid-player facility")
	}
	if zeroIdx >= 0 {
		checkBuilder(t, values2[zeroIdx].(sav.PropertyList), zero, "all-zero facility")
	}

	// Idempotent: a second repair changes nothing.
	n2, err := RepairMapObjectBuilders(gf, hints, custom)
	if err != nil {
		t.Fatalf("second repair: %v", err)
	}
	if n2 != 0 {
		t.Fatalf("second repair changed %d, want 0 (idempotent)", n2)
	}

	// Round-trip: compress + reparse preserves facility count and the repairs.
	out, err := sav.Compress(gf.Write(custom), hdr)
	if err != nil {
		t.Fatalf("compress: %v", err)
	}
	gvas2, _, err := sav.Decompress(out)
	if err != nil {
		t.Fatalf("re-decompress: %v", err)
	}
	gf2, err := sav.ReadGvasFile(gvas2, hints, custom)
	if err != nil {
		t.Fatalf("re-parse: %v", err)
	}
	parsed3 := mustReparseMapObject(t, gf2, hints, custom)
	values3 := parsed3["values"].([]any)
	if len(values3) != len(values) {
		t.Fatalf("facility count changed: %d -> %d", len(values), len(values3))
	}
	checkBuilder(t, values3[0].(sav.PropertyList), HostUUID, "mashup facility after round-trip")
}

// TestRemapMapObjectBuilders verifies the activation-path remap: a specific
// player UID in builder fields is remapped to the host slot, other UIDs untouched.
func TestRemapMapObjectBuilders(t *testing.T) {
	gf, _, hints, custom := parseFixturePLM(t)
	skipMap := findMapObjectSkipMap(gf.Properties)
	if skipMap == nil {
		t.Skip("fixture has no MapObjectSaveData")
	}
	parsed := mustReparseMapObject(t, gf, hints, custom)
	values := parsed["values"].([]any)
	if len(values) < 2 {
		t.Skip("fixture has too few facilities")
	}
	// Stamp facility 0 and 1 with a synthetic "guest" UID, facility 2 with host.
	guest := sav.UUID{0xde, 0xad, 0xbe, 0xef, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0}
	corruptBuilder(t, values[0].(sav.PropertyList), guest)
	corruptBuilder(t, values[1].(sav.PropertyList), guest)
	corruptBuilder(t, values[2].(sav.PropertyList), HostUUID)
	skipMap["value"] = encodeMapObjectArray(parsed, custom)

	n, err := RemapMapObjectBuilders(gf, hints, custom, guest, HostUUID)
	if err != nil {
		t.Fatalf("RemapMapObjectBuilders: %v", err)
	}
	if n != 2 {
		t.Fatalf("remapped %d, want 2", n)
	}
	parsed2 := mustReparseMapObject(t, gf, hints, custom)
	values2 := parsed2["values"].([]any)
	checkBuilder(t, values2[0].(sav.PropertyList), HostUUID, "guest facility 0")
	checkBuilder(t, values2[1].(sav.PropertyList), HostUUID, "guest facility 1")
	checkBuilder(t, values2[2].(sav.PropertyList), HostUUID, "host facility (unchanged)")
}

func corruptBuilder(t *testing.T, fac sav.PropertyList, u sav.UUID) {
	t.Helper()
	rd := findModelRawDataBytes(fac)
	if len(rd) < mapObjectBuilderOffset+16 {
		t.Fatal("RawData too short to corrupt")
	}
	newRD := make([]byte, len(rd))
	copy(newRD, rd)
	copy(newRD[mapObjectBuilderOffset:mapObjectBuilderOffset+16], u[:])
	if !setModelRawDataBytes(fac, newRD) {
		t.Fatal("setModelRawDataBytes failed")
	}
}

func mustReparseMapObject(t *testing.T, gf *sav.GvasFile, hints map[string]string, custom map[string]sav.CustomProperty) map[string]any {
	t.Helper()
	skipMap := findMapObjectSkipMap(gf.Properties)
	if skipMap == nil {
		t.Fatal("no MapObjectSaveData after mutation")
	}
	blob, _ := skipMap["value"].([]byte)
	parsed, err := parseMapObjectArray(blob, hints, custom)
	if err != nil {
		t.Fatalf("reparse: %v", err)
	}
	return parsed
}

func checkBuilder(t *testing.T, fac sav.PropertyList, want sav.UUID, label string) {
	t.Helper()
	rd := findModelRawDataBytes(fac)
	if len(rd) < mapObjectBuilderOffset+16 {
		t.Fatalf("%s: RawData too short", label)
	}
	var got sav.UUID
	copy(got[:], rd[mapObjectBuilderOffset:mapObjectBuilderOffset+16])
	if got != want {
		t.Fatalf("%s: builder=%s want=%s", label, got.String(), want.String())
	}
}
