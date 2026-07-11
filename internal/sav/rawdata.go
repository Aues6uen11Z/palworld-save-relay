package sav

// skipDecode reads a RawData payload opaquely, preserving the type-specific
// header (array_type+id / key+value+id / struct_type+struct_id+id) and storing
// the remaining `size` bytes verbatim. This lets unknown custom properties
// round-trip byte-identically without a dedicated parser.
//
// The u64 property size excludes this type-specific header.
func skipDecode(r *FArchiveReader, typeName string, size int, path string) map[string]any {
	switch typeName {
	case "ArrayProperty":
		arrayType := r.FString()
		id := r.OptionalGuid()
		return map[string]any{"skip_type": typeName, "array_type": arrayType, "id": id, "value": r.Read(size)}
	case "MapProperty":
		keyType := r.FString()
		valueType := r.FString()
		id := r.OptionalGuid()
		return map[string]any{"skip_type": typeName, "key_type": keyType, "value_type": valueType, "id": id, "value": r.Read(size)}
	case "StructProperty":
		structType := r.FString()
		structID := r.Guid()
		id := r.OptionalGuid()
		return map[string]any{"skip_type": typeName, "struct_type": structType, "struct_id": structID, "id": id, "value": r.Read(size)}
	default:
		panic("sav: skip expects Array/Map/StructProperty, got " + typeName + " (" + path + ")")
	}
}

// skipEncode writes an opaque RawData payload back verbatim, returning the
// value byte length (the property size, which excludes the type-specific header).
func skipEncode(w *FArchiveWriter, propertyType string, p map[string]any) int {
	raw := p["value"].([]byte)
	switch propertyType {
	case "ArrayProperty":
		w.FString(p["array_type"].(string))
		w.OptionalGuid(asUUID(p["id"]))
	case "MapProperty":
		w.FString(p["key_type"].(string))
		w.FString(p["value_type"].(string))
		w.OptionalGuid(asUUID(p["id"]))
	case "StructProperty":
		w.FString(p["struct_type"].(string))
		w.Guid(asUUID(p["struct_id"]))
		w.OptionalGuid(asUUID(p["id"]))
	default:
		panic("sav: skip encode expects Array/Map/StructProperty, got " + propertyType)
	}
	w.Write(raw)
	return len(raw)
}
