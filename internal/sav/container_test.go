package sav

import (
	"bytes"
	"encoding/binary"
	"testing"
)

func TestParseSAVHeader(t *testing.T) {
	cases := []struct {
		name     string
		fixture  string
		magic    string
		saveType byte
	}{
		{"PlZ_level", "level_plz.sav", "PlZ", 50},
		{"PlZ_player", "player_plz.sav", "PlZ", 49},
		{"PlM_level", "level_plm.sav", "PlM", 49},
		{"PlM_player", "player_plm.sav", "PlM", 49},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			h, err := ParseSAVHeader(readFixture(t, c.fixture))
			if err != nil {
				t.Fatalf("ParseSAVHeader: %v", err)
			}
			if got := string(h.Magic[:]); got != c.magic {
				t.Errorf("magic = %q, want %q", got, c.magic)
			}
			if h.SaveType != c.saveType {
				t.Errorf("saveType = %d, want %d", h.SaveType, c.saveType)
			}
			if h.UncompressedLen == 0 || h.CompressedLen == 0 {
				t.Errorf("zero lengths: uncomp=%d comp=%d", h.UncompressedLen, h.CompressedLen)
			}
		})
	}
}

func TestOodleDecompress(t *testing.T) {
	data := readFixture(t, "player_plm.sav")
	h, err := ParseSAVHeader(data)
	if err != nil {
		t.Fatalf("ParseSAVHeader: %v", err)
	}
	comp := data[h.DataOffset : h.DataOffset+int(h.CompressedLen)]
	out, err := OodleDecompress(comp, int(h.UncompressedLen))
	if err != nil {
		t.Fatalf("OodleDecompress: %v", err)
	}
	if len(out) != int(h.UncompressedLen) {
		t.Fatalf("len = %d, want %d", len(out), h.UncompressedLen)
	}
	// Decompressed payload must start with the GVAS magic ("GVAS" LE = 0x53415647).
	if got := binary.LittleEndian.Uint32(out[0:4]); got != 0x53415647 {
		t.Fatalf("not GVAS magic: %x", got)
	}
}

func TestDecompressCompressRoundTrip(t *testing.T) {
	fixtures := []string{"level_plz.sav", "player_plz.sav", "level_plm.sav", "player_plm.sav", "localdata_plm.sav"}
	for _, name := range fixtures {
		t.Run(name, func(t *testing.T) {
			data := readFixture(t, name)
			gvas, h, err := Decompress(data)
			if err != nil {
				t.Fatalf("Decompress: %v", err)
			}
			sav2, err := Compress(gvas, h)
			if err != nil {
				t.Fatalf("Compress: %v", err)
			}
			gvas2, _, err := Decompress(sav2)
			if err != nil {
				t.Fatalf("re-Decompress: %v", err)
			}
			if !bytes.Equal(gvas, gvas2) {
				t.Fatalf("GVAS bytes differ after compress round-trip")
			}
		})
	}
}
