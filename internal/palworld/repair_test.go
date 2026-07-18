package palworld

import (
	"os"
	"path/filepath"
	"testing"

	"palworld-save-relay/internal/sav"
)

func readLevelFixture(t *testing.T) []byte {
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

func writeWorld(t *testing.T, levelData []byte, addHostSave bool) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "Level.sav"), levelData, 0o644); err != nil {
		t.Fatal(err)
	}
	if addHostSave {
		players := filepath.Join(dir, "Players")
		os.MkdirAll(players, 0o755)
		if err := os.WriteFile(filepath.Join(players, uidFilename(HostUUID)), []byte("host"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func guildSnapshot(t *testing.T, dir string) (map[string]any, int) {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(dir, "Level.sav"))
	if err != nil {
		t.Fatal(err)
	}
	gvas, _, err := sav.Decompress(data)
	if err != nil {
		t.Fatalf("decompress: %v", err)
	}
	hints, custom := sav.PalWorldConfig()
	gf, err := sav.ReadGvasFile(gvas, hints, custom)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	guild, err := findGuild(gf)
	if err != nil {
		t.Fatalf("findGuild: %v", err)
	}
	return guild, cspmCount(gf)
}

// corruptICH truncates the guild ICH to 2 entries (the early-tool truncation),
// leaving everything else (incl. group_name) untouched.
func corruptICH(t *testing.T, dir string) {
	t.Helper()
	path := filepath.Join(dir, "Level.sav")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	gvas, hdr, err := sav.Decompress(data)
	if err != nil {
		t.Fatal(err)
	}
	hints, custom := sav.PalWorldConfig()
	gf, err := sav.ReadGvasFile(gvas, hints, custom)
	if err != nil {
		t.Fatal(err)
	}
	guild, err := findGuild(gf)
	if err != nil {
		t.Fatal(err)
	}
	if ich, ok := guild["individual_character_handle_ids"].([]any); ok && len(ich) > 2 {
		guild["individual_character_handle_ids"] = ich[:2]
	}
	out, err := sav.Compress(gf.Write(custom), hdr)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, out, 0o644); err != nil {
		t.Fatal(err)
	}
}

func expectedICHCount(t *testing.T, dir string) int {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(dir, "Level.sav"))
	if err != nil {
		t.Fatal(err)
	}
	gvas, _, err := sav.Decompress(data)
	if err != nil {
		t.Fatal(err)
	}
	hints, custom := sav.PalWorldConfig()
	gf, err := sav.ReadGvasFile(gvas, hints, custom)
	if err != nil {
		t.Fatal(err)
	}
	guild, err := findGuild(gf)
	if err != nil {
		t.Fatal(err)
	}
	return len(buildICHForGroup(gf, guild))
}

func TestRepairIntermediate_IdempotentOnHealthySave(t *testing.T) {
	dir := writeWorld(t, readLevelFixture(t), false)
	guildBefore, _ := guildSnapshot(t, dir)
	gnBefore, _ := guildBefore["group_name"].(string)
	ichBefore, _ := guildBefore["individual_character_handle_ids"].([]any)

	rep, err := RepairIntermediate(dir)
	if err != nil {
		t.Fatalf("RepairIntermediate: %v", err)
	}
	if rep.RebuiltICH || rep.ConsolidatedPals || rep.ConvertedOpaque {
		t.Errorf("healthy save was modified: %+v", rep)
	}
	// group_name must be left exactly as-is (repair no longer touches it).
	guildAfter, _ := guildSnapshot(t, dir)
	gnAfter, _ := guildAfter["group_name"].(string)
	ichAfter, _ := guildAfter["individual_character_handle_ids"].([]any)
	if gnAfter != gnBefore {
		t.Errorf("group_name changed on healthy save: %q -> %q", gnBefore, gnAfter)
	}
	if len(ichAfter) != len(ichBefore) {
		t.Errorf("ICH changed on healthy save: %d -> %d", len(ichBefore), len(ichAfter))
	}
}

// TestRepairIntermediate_RebuildsTruncatedICH: a host world (0001.sav present)
// with a truncated ICH. Repair rebuilds the ICH per group_id; group_name is NOT
// touched and the sentinel conversion is skipped (host world).
func TestRepairIntermediate_RebuildsTruncatedICH(t *testing.T) {
	dir := writeWorld(t, readLevelFixture(t), true)
	want := expectedICHCount(t, dir)
	corruptICH(t, dir)
	guild0, _ := guildSnapshot(t, dir)
	gnBefore, _ := guild0["group_name"].(string)

	rep, err := RepairIntermediate(dir)
	if err != nil {
		t.Fatalf("RepairIntermediate: %v", err)
	}
	if !rep.RebuiltICH {
		t.Error("expected RebuiltICH=true")
	}
	guild, _ := guildSnapshot(t, dir)
	ich, _ := guild["individual_character_handle_ids"].([]any)
	gnAfter, _ := guild["group_name"].(string)
	if len(ich) != want {
		t.Errorf("ICH=%d, want %d (per-group_id CSPM count)", len(ich), want)
	}
	if gnAfter != gnBefore {
		t.Errorf("group_name changed: %q -> %q (must be untouched)", gnBefore, gnAfter)
	}
}

func TestRepairIntermediate_RoundTripPreservesGVAS(t *testing.T) {
	dir := writeWorld(t, readLevelFixture(t), true)
	corruptICH(t, dir)
	if _, err := RepairIntermediate(dir); err != nil {
		t.Fatalf("RepairIntermediate: %v", err)
	}
	hints, custom := sav.PalWorldConfig()
	data, err := os.ReadFile(filepath.Join(dir, "Level.sav"))
	if err != nil {
		t.Fatalf("read Level.sav: %v", err)
	}
	gvas, _, err := sav.Decompress(data)
	if err != nil {
		t.Fatalf("decompress: %v", err)
	}
	if _, err := sav.ReadGvasFile(gvas, hints, custom); err != nil {
		t.Fatalf("parse Level.sav: %v", err)
	}
}
