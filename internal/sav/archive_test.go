package sav

import (
	"bytes"
	"testing"
)

func TestFArchivePrimitivesRoundTrip(t *testing.T) {
	w := NewFArchiveWriter(nil)
	w.FString("hello")
	w.FString("你好") // UTF-16
	w.I32(-12345)
	w.U64(0x0102030405060708)
	g := UUID{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16}
	w.Guid(&g)
	w.OptionalGuid(nil)
	w.OptionalGuid(&g)

	r := NewFArchiveReader(w.Bytes(), nil, nil)
	if v := r.FString(); v != "hello" {
		t.Errorf("ascii fstring = %q", v)
	}
	if v := r.FString(); v != "你好" {
		t.Errorf("utf16 fstring = %q", v)
	}
	if v := r.I32(); v != -12345 {
		t.Errorf("i32 = %d", v)
	}
	if v := r.U64(); v != 0x0102030405060708 {
		t.Errorf("u64 = %x", v)
	}
	if v := r.Guid(); *v != g {
		t.Errorf("guid = %v", v)
	}
	if r.OptionalGuid() != nil {
		t.Error("optional guid should be nil")
	}
	if v := r.OptionalGuid(); *v != g {
		t.Errorf("optional guid = %v", v)
	}
}

func TestUUIDString(t *testing.T) {
	// Sanity: 16 zero bytes format to all-zero GUID segments.
	var z UUID
	if z.String() != "00000000-0000-0000-0000-000000000000" {
		t.Errorf("zero uuid = %q", z.String())
	}
}

func TestGvasHeaderRoundTrip(t *testing.T) {
	data := readFixture(t, "player_plz.sav")
	gvas, _, err := Decompress(data)
	if err != nil {
		t.Fatalf("Decompress: %v", err)
	}
	r := NewFArchiveReader(gvas, nil, nil)
	hdr, err := ReadGvasHeader(r)
	if err != nil {
		t.Fatalf("ReadGvasHeader: %v", err)
	}
	headerEnd := r.Pos()

	w := NewFArchiveWriter(nil)
	WriteGvasHeader(w, hdr)
	if !bytes.Equal(w.Bytes(), gvas[:headerEnd]) {
		t.Fatalf("header bytes differ (got %d, want %d)", len(w.Bytes()), headerEnd)
	}
	if hdr.Magic != GvasMagic {
		t.Errorf("magic = %x, want %x", hdr.Magic, GvasMagic)
	}
}
