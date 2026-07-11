package sav

// characterDecode parses a CharacterSaveParameterMap entry's RawData blob.
//
// The RawData is an ArrayProperty of bytes; we read it normally (via the nested
// guard so we don't re-enter this handler), then decode the byte payload as:
//
//	{object: property sequence (the PalIndividualCharacterSaveParameter),
//	 unknown_bytes[4], group_id: Guid, trailing_bytes[4], trailing_unknown_bytes?}
//
// Ported from cheahjs palsav/rawdata/character.py.
func characterDecode(r *FArchiveReader, typeName string, size int, path string) map[string]any {
	value := r.propertyWithNested(typeName, size, path, path)
	charBytes := value["value"].(map[string]any)["values"].([]byte)
	value["value"] = characterDecodeBytes(r, charBytes)
	return value
}

func characterDecodeBytes(parent *FArchiveReader, b []byte) map[string]any {
	r := parent.InternalCopy(b)
	data := map[string]any{
		"object":         r.PropertiesUntilEnd(""),
		"unknown_bytes":  r.ByteList(4),
		"group_id":       r.Guid(),
		"trailing_bytes": r.ByteList(4),
	}
	if !r.EOF() {
		data["trailing_unknown_bytes"] = r.ReadToEnd()
	}
	return data
}

func characterEncode(w *FArchiveWriter, propertyType string, p map[string]any) int {
	delete(p, "custom_type")
	encoded := characterEncodeBytes(p["value"].(map[string]any))
	p["value"] = map[string]any{"values": encoded}
	return w.propertyInnerNoCustom(propertyType, p)
}

func characterEncodeBytes(d map[string]any) []byte {
	w := NewFArchiveWriter(nil)
	w.Properties(d["object"].(PropertyList))
	w.Write(d["unknown_bytes"].([]byte))
	w.Guid(asUUID(d["group_id"]))
	w.Write(d["trailing_bytes"].([]byte))
	if tb, ok := d["trailing_unknown_bytes"]; ok {
		w.Write(tb.([]byte))
	}
	return w.Bytes()
}
