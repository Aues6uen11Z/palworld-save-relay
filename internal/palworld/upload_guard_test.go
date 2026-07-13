package palworld

import (
	"os"
	"path/filepath"
	"testing"

	"palworld-save-relay/internal/sav"
)

// helper: create a minimal world folder with the given player .sav files.
func makeWorldDir(t *testing.T, players ...sav.UUID) string {
	t.Helper()
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "Players"), 0o755)
	os.WriteFile(filepath.Join(dir, "Level.sav"), []byte("fake-level"), 0o644)
	for _, uid := range players {
		os.WriteFile(filepath.Join(dir, "Players", uidFilename(uid)), []byte("fake-player"), 0o644)
	}
	return dir
}

func TestAssertUploadReady_OriginalHost(t *testing.T) {
	// Original host: only HostUUID.sav exists, no realUID.sav.
	dir := makeWorldDir(t, HostUUID)
	if err := assertUploadReady(dir, fakeUID); err != nil {
		t.Fatalf("original host should pass: %v", err)
	}
}

func TestAssertUploadReady_ActivatedSuccessor(t *testing.T) {
	// Downloaded + activated: HostUUID.sav exists, realUID.sav does not.
	dir := makeWorldDir(t, HostUUID)
	if err := assertUploadReady(dir, fakeUID); err != nil {
		t.Fatalf("activated successor should pass: %v", err)
	}
}

func TestAssertUploadReady_NoLevelSav(t *testing.T) {
	// Guest-only folder (stripped after upload): no Level.sav.
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "LocalData.sav"), []byte("local"), 0o644)
	if err := assertUploadReady(dir, fakeUID); err == nil {
		t.Fatal("should fail when Level.sav is missing")
	}
}

func TestAssertUploadReady_NoHostSave(t *testing.T) {
	// Downloaded but not activated, not played: no HostUUID.sav.
	dir := makeWorldDir(t, fakeUID) // only realUID player save
	if err := assertUploadReady(dir, fakeUID); err == nil {
		t.Fatal("should fail when host save is missing (not activated)")
	}
}

func TestAssertUploadReady_DuplicateData(t *testing.T) {
	// Downloaded, played without activating: both HostUUID.sav and realUID.sav exist.
	dir := makeWorldDir(t, HostUUID, fakeUID)
	err := assertUploadReady(dir, fakeUID)
	if err == nil {
		t.Fatal("should fail when both host and guest saves exist (played without activating)")
	}
}

func TestAssertUploadReady_OtherPlayersOK(t *testing.T) {
	// Host with other guest players: HostUUID + otherUID, but NOT fakeUID.
	otherUID := sav.UUID{0xAA, 0xBB, 0xCC, 0xDD, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0}
	dir := makeWorldDir(t, HostUUID, otherUID)
	if err := assertUploadReady(dir, fakeUID); err != nil {
		t.Fatalf("host with other guests should pass: %v", err)
	}
}
