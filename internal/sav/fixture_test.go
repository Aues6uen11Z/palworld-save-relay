package sav

import (
	"os"
	"testing"
)

// readFixture loads a testdata file, skipping the test if it is absent
// (real save fixtures are gitignored for privacy).
func readFixture(t *testing.T, name string) []byte {
	t.Helper()
	b, err := os.ReadFile("testdata/" + name)
	if os.IsNotExist(err) {
		t.Skipf("fixture missing: testdata/%s (run testdata/fetch.ps1)", name)
	}
	if err != nil {
		t.Fatalf("read fixture %s: %v", name, err)
	}
	return b
}
