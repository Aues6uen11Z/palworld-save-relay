package palworld

import (
	"testing"

	"palworld-save-relay/internal/sav"
)

func TestCityHash64(t *testing.T) {
	cases := []struct {
		in   string
		want uint64
	}{
		{"", 0x9ae16a3b2f90404f},
	}
	for _, c := range cases {
		if got := CityHash64([]byte(c.in)); got != c.want {
			t.Errorf("CityHash64(%q) = %x, want %x", c.in, got, c.want)
		}
	}
}

func TestSteamIDToPlayerUUID(t *testing.T) {
	// 76561198986886742 -> cityhash64(...) = 0x8494729d0c5356be -> result
	// u32 = 0xf5a9a2d9 -> LE bytes d9 a2 a9 f5 (verified vs reference Python).
	u := SteamIDToPlayerUUID(76561198986886742)
	want := sav.UUID{0xd9, 0xa2, 0xa9, 0xf5, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0}
	if u != want {
		t.Errorf("SteamIDToPlayerUUID = %x, want %x", u, want)
	}
	// Guest UIDs in the test save are SteamID-derived (4 nonzero bytes + zeros).
	guests := []sav.UUID{
		{0x69, 0x03, 0x23, 0x91, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0}, // 91230369
		{0x21, 0x31, 0xe0, 0x16, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0}, // 16e03121
	}
	for _, g := range guests {
		if !isSteamIDDerived(g) {
			t.Errorf("guest %x should be SteamID-derived", g)
		}
	}
}

// isSteamIDDerived reports whether a UID has the SteamID-derived shape:
// nonzero only in the first 4 bytes.
func isSteamIDDerived(u sav.UUID) bool {
	for _, b := range u[4:] {
		if b != 0 {
			return false
		}
	}
	return true
}
