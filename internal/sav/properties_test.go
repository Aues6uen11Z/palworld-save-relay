package sav

import (
	"bytes"
	"testing"
)

func TestPropertyRoundTripSynthetic(t *testing.T) {
	g := &UUID{0xA, 0xB, 0xC, 0xD, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12}
	original := PropertyList{
		{Name: "IntField", Value: map[string]any{"type": "IntProperty", "id": (*UUID)(nil), "value": int32(42)}},
		{Name: "StrField", Value: map[string]any{"type": "StrProperty", "id": (*UUID)(nil), "value": "player"}},
		{Name: "NameField", Value: map[string]any{"type": "NameProperty", "id": (*UUID)(nil), "value": "EPal::Foo"}},
		{Name: "BoolField", Value: map[string]any{"type": "BoolProperty", "value": true, "id": (*UUID)(nil)}},
		{Name: "EnumField", Value: map[string]any{"type": "EnumProperty", "id": (*UUID)(nil), "value": map[string]any{"type": "EPalX", "value": "A"}}},
		{Name: "GuidField", Value: map[string]any{"type": "StructProperty", "struct_type": "Guid", "struct_id": &UUID{}, "id": (*UUID)(nil), "value": g}},
		{Name: "FloatField", Value: map[string]any{"type": "FloatProperty", "id": (*UUID)(nil), "value": float32(1.5)}},
		{Name: "Int64Field", Value: map[string]any{"type": "Int64Property", "id": (*UUID)(nil), "value": int64(9000000000)}},
	}

	w := NewFArchiveWriter(nil)
	w.Properties(original)
	encoded := w.Bytes()

	r := NewFArchiveReader(encoded, nil, nil)
	parsed := r.PropertiesUntilEnd("")
	if !r.EOF() {
		t.Fatalf("trailing bytes: %d", len(r.ReadToEnd()))
	}

	w2 := NewFArchiveWriter(nil)
	w2.Properties(parsed)
	if !bytes.Equal(encoded, w2.Bytes()) {
		t.Fatalf("property round-trip bytes differ")
	}
	if parsed.Get("IntField")["value"].(int32) != 42 {
		t.Errorf("IntField value lost")
	}
}

func TestArrayAndMapRoundTrip(t *testing.T) {
	g1 := &UUID{1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1}
	g2 := &UUID{2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2}
	original := PropertyList{
		{Name: "GuidArray", Value: map[string]any{
			"type": "ArrayProperty", "array_type": "StructProperty",
			"id": (*UUID)(nil),
			"value": map[string]any{
				"prop_name": "Element", "prop_type": "StructProperty",
				"type_name": "Guid", "id": &UUID{},
				"values": []any{g1, g2},
			},
		}},
		{Name: "StrToInt", Value: map[string]any{
			"type": "MapProperty", "key_type": "StrProperty", "value_type": "IntProperty",
			"key_struct_type": "", "value_struct_type": "",
			"id": (*UUID)(nil),
			"value": []map[string]any{
				{"key": "a", "value": int32(1)},
				{"key": "b", "value": int32(2)},
			},
		}},
	}

	w := NewFArchiveWriter(nil)
	w.Properties(original)
	encoded := w.Bytes()

	r := NewFArchiveReader(encoded, nil, nil)
	parsed := r.PropertiesUntilEnd("")
	if !r.EOF() {
		t.Fatalf("trailing bytes: %d", len(r.ReadToEnd()))
	}
	w2 := NewFArchiveWriter(nil)
	w2.Properties(parsed)
	if !bytes.Equal(encoded, w2.Bytes()) {
		t.Fatalf("array/map round-trip bytes differ")
	}
}
