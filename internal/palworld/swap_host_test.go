package palworld

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"palworld-save-relay/internal/sav"
)

// findSnapshotWorld returns a Palworld world dir (Level.sav + Players/ with
// >=2 .sav files) for end-to-end SwapHost testing, preferring a stable backup
// snapshot. Returns "" if none.
func findSnapshotWorld() string {
	root := filepath.Join(os.Getenv("LOCALAPPDATA"), "Pal", "Saved", "SaveGames")
	var backup, live string
	filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil || !info.IsDir() {
			return nil
		}
		if _, err := os.Stat(filepath.Join(path, "Level.sav")); err != nil {
			return nil
		}
		players, _ := os.ReadDir(filepath.Join(path, "Players"))
		if countSav(players) < 2 {
			return nil
		}
		if containsBackup(path) {
			backup = path
		} else if live == "" {
			live = path
		}
		return nil
	})
	if backup != "" {
		return backup
	}
	return live
}

func countSav(entries []os.DirEntry) int {
	n := 0
	for _, e := range entries {
		if !e.IsDir() && filepath.Ext(e.Name()) == ".sav" {
			n++
		}
	}
	return n
}

func containsBackup(path string) bool {
	for p := path; p != "" && p != filepath.Dir(p); p = filepath.Dir(p) {
		if filepath.Base(p) == "backup" {
			return true
		}
	}
	return false
}

// gvasSet returns the set of decompressed GVAS byte-slices for every .sav under
// dir (path-agnostic, since SwapHost renames player files).
func gvasSet(t *testing.T, dir string) [][]byte {
	t.Helper()
	var out [][]byte
	filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() || filepath.Ext(path) != ".sav" {
			return nil
		}
		b, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		g, _, err := sav.Decompress(b)
		if err != nil {
			t.Fatalf("decompress %s: %v", path, err)
		}
		out = append(out, g)
		return nil
	})
	return out
}

func sameGvasSet(a, b [][]byte) bool {
	if len(a) != len(b) {
		return false
	}
	used := make([]bool, len(b))
	for _, x := range a {
		found := false
		for i, y := range b {
			if !used[i] && bytes.Equal(x, y) {
				used[i] = true
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}

func TestSwapHost_FilesystemReversible(t *testing.T) {
	src := findSnapshotWorld()
	if src == "" {
		t.Skip("no Palworld world snapshot found on this machine")
	}
	levelData, err := os.ReadFile(filepath.Join(src, "Level.sav"))
	if err != nil {
		t.Skipf("read Level.sav: %v", err)
	}
	gvas, _, err := sav.Decompress(levelData)
	if err != nil {
		t.Fatalf("decompress: %v", err)
	}
	_, guestInst, guestUID := findHostGuest(t, gvas)
	if guestUID == nil || guestInst == nil {
		t.Skip("no guest player found")
	}

	origSet := gvasSet(t, src)

	packed, err := PackWorld(src)
	if err != nil {
		t.Fatalf("PackWorld: %v", err)
	}
	tmp, err := os.MkdirTemp("", "palrelay-swap-*")
	if err != nil {
		t.Fatalf("tempdir: %v", err)
	}
	defer os.RemoveAll(tmp)
	if err := UnpackWorld(packed, tmp); err != nil {
		t.Fatalf("UnpackWorld: %v", err)
	}

	if err := swapHostImpl(tmp, *guestUID); err != nil {
		t.Fatalf("swap 1: %v", err)
	}
	if err := swapHostImpl(tmp, *guestUID); err != nil {
		t.Fatalf("swap 2: %v", err)
	}

	afterSet := gvasSet(t, tmp)
	if !sameGvasSet(origSet, afterSet) {
		t.Fatalf("GVAS set changed after double swap: orig=%d files, after=%d", len(origSet), len(afterSet))
	}
	t.Logf("double swap preserved %d save files' GVAS content", len(origSet))
}
