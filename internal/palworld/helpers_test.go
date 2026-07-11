package palworld

import (
	"os"
	"testing"
)

func readSavFixtureX(t *testing.T, name string) []byte {
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
