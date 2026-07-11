package sav

import "encoding/binary"

// PropertiesUntilEnd reads the ordered property sequence terminated by "None".
func (r *FArchiveReader) PropertiesUntilEnd(path string) PropertyList {
	var pl PropertyList
	for {
		name := r.FString()
		if name == "None" {
			break
		}
		typeName := r.FString()
		size := int(r.U64())
		pl = append(pl, PropertyEntry{Name: name, Value: r.Property(typeName, size, path+"."+name)})
	}
	return pl
}

// Property reads one property value, dispatching to a registered custom
// handler (by path) when present, else to the built-in type reader.
func (r *FArchiveReader) Property(typeName string, size int, path string) map[string]any {
	return r.propertyWithNested(typeName, size, path, "")
}

// propertyWithNested is Property with a nested-caller path: when path equals
// nested, the custom handler is skipped so a decoder can read its own container
// (e.g. an ArrayProperty of bytes) without re-entering itself.
func (r *FArchiveReader) propertyWithNested(typeName string, size int, path, nested string) map[string]any {
	var value map[string]any
	if cp, ok := r.customProperties[path]; ok && path != nested {
		value = cp.Decode(r, typeName, size, path)
		value["custom_type"] = path
	} else {
		value = r.readPropertyByType(typeName, size, path)
	}
	value["type"] = typeName
	return value
}

// propertyInnerNoCustom writes a property via the built-in dispatch, ignoring
// any custom_type. Used by custom encoders to write their container normally.
func (w *FArchiveWriter) propertyInnerNoCustom(propertyType string, p map[string]any) int {
	return w.propertyInner(propertyType, p)
}

func (r *FArchiveReader) readPropertyByType(typeName string, size int, path string) map[string]any {
	switch typeName {
	case "StructProperty":
		return r.readStruct(path)
	case "IntProperty":
		return map[string]any{"id": r.OptionalGuid(), "value": r.I32()}
	case "UInt16Property":
		return map[string]any{"id": r.OptionalGuid(), "value": r.U16()}
	case "UInt32Property":
		return map[string]any{"id": r.OptionalGuid(), "value": r.U32()}
	case "UInt64Property":
		return map[string]any{"id": r.OptionalGuid(), "value": r.U64()}
	case "Int64Property":
		return map[string]any{"id": r.OptionalGuid(), "value": r.I64()}
	case "FixedPoint64Property":
		return map[string]any{"id": r.OptionalGuid(), "value": r.I32()}
	case "FloatProperty":
		return map[string]any{"id": r.OptionalGuid(), "value": r.Float()}
	case "StrProperty", "NameProperty":
		return map[string]any{"id": r.OptionalGuid(), "value": r.FString()}
	case "EnumProperty":
		enumType := r.FString()
		id := r.OptionalGuid()
		return map[string]any{"id": id, "value": map[string]any{"type": enumType, "value": r.FString()}}
	case "BoolProperty":
		return map[string]any{"value": r.Bool(), "id": r.OptionalGuid()}
	case "ByteProperty":
		enumType := r.FString()
		id := r.OptionalGuid()
		var ev any
		if enumType == "None" {
			ev = r.Byte()
		} else {
			ev = r.FString()
		}
		return map[string]any{"id": id, "value": map[string]any{"type": enumType, "value": ev}}
	case "ArrayProperty":
		arrayType := r.FString()
		id := r.OptionalGuid()
		return map[string]any{"array_type": arrayType, "id": id, "value": r.arrayProperty(arrayType, size, path)}
	case "MapProperty":
		return r.readMapProperty(size, path)
	case "SetProperty":
		return r.readSetProperty(size, path)
	default:
		panic("sav: unknown property type " + typeName + " (" + path + ")")
	}
}

func (r *FArchiveReader) readStruct(path string) map[string]any {
	structType := r.FString()
	structID := r.Guid()
	id := r.OptionalGuid()
	return map[string]any{
		"struct_type": structType,
		"struct_id":   structID,
		"id":          id,
		"value":       r.StructValue(structType, path),
	}
}

// StructValue reads a struct value, dispatching on struct type. Unknown types
// are read as an ordered property sequence (used by Palworld struct payloads).
func (r *FArchiveReader) StructValue(structType, path string) any {
	switch structType {
	case "Vector":
		return map[string]any{"x": r.Double(), "y": r.Double(), "z": r.Double()}
	case "DateTime":
		return r.U64()
	case "Guid":
		return r.Guid()
	case "Quat":
		return map[string]any{"x": r.Double(), "y": r.Double(), "z": r.Double(), "w": r.Double()}
	case "LinearColor":
		return map[string]any{"r": r.Float(), "g": r.Float(), "b": r.Float(), "a": r.Float()}
	case "Color":
		return map[string]any{"b": r.Byte(), "g": r.Byte(), "r": r.Byte(), "a": r.Byte()}
	default:
		return r.PropertiesUntilEnd(path)
	}
}

func (r *FArchiveReader) arrayProperty(arrayType string, size int, path string) map[string]any {
	count := r.U32()
	if arrayType == "StructProperty" {
		propName := r.FString()
		propType := r.FString()
		r.U64() // reserved size slot
		typeName := r.FString()
		id := r.Guid()
		r.Byte() // reserved
		values := make([]any, 0, count)
		for i := uint32(0); i < count; i++ {
			values = append(values, r.StructValue(typeName, path+"."+propName))
		}
		return map[string]any{"prop_name": propName, "prop_type": propType, "values": values, "type_name": typeName, "id": id}
	}
	return map[string]any{"values": r.arrayValue(arrayType, int(count), size-4, path)}
}

func (r *FArchiveReader) arrayValue(arrayType string, count, size int, path string) any {
	switch arrayType {
	case "EnumProperty", "NameProperty":
		out := make([]any, 0, count)
		for i := 0; i < count; i++ {
			out = append(out, r.FString())
		}
		return out
	case "Guid":
		out := make([]any, 0, count)
		for i := 0; i < count; i++ {
			out = append(out, r.Guid())
		}
		return out
	case "ByteProperty":
		if size == count {
			return r.ByteList(count)
		}
		panic("sav: labelled ByteProperty array not implemented: " + path)
	default:
		panic("sav: unknown array type " + arrayType + " (" + path + ")")
	}
}

func (r *FArchiveReader) readMapProperty(size int, path string) map[string]any {
	keyType := r.FString()
	valueType := r.FString()
	id := r.OptionalGuid()
	start := r.pos
	r.U32() // reserved 0
	count := r.U32()
	keyStructType, valueStructType := "", ""
	if keyType == "StructProperty" {
		keyStructType = r.getTypeOr(path+".Key", "Guid")
	}
	if valueType == "StructProperty" {
		valueStructType = r.getTypeOr(path+".Value", "StructProperty")
	}
	values := make([]map[string]any, 0, count)
	for i := uint32(0); i < count; i++ {
		values = append(values, map[string]any{
			"key":   r.propValue(keyType, keyStructType, path+".Key"),
			"value": r.propValue(valueType, valueStructType, path+".Value"),
		})
	}
	if rem := size - (r.pos - start); rem > 0 {
		r.Skip(rem)
	}
	return map[string]any{
		"key_type": keyType, "value_type": valueType,
		"key_struct_type": keyStructType, "value_struct_type": valueStructType,
		"id": id, "value": values,
	}
}

func (r *FArchiveReader) readSetProperty(size int, path string) map[string]any {
	setType := r.FString()
	id := r.OptionalGuid()
	start := r.pos
	r.U32()
	count := r.U32()
	values := make([]PropertyList, 0, count)
	for i := uint32(0); i < count; i++ {
		values = append(values, r.PropertiesUntilEnd(path))
	}
	if rem := size - (r.pos - start); rem > 0 {
		r.Skip(rem)
	}
	return map[string]any{"set_type": setType, "id": id, "value": values}
}

func (r *FArchiveReader) propValue(typeName, structType, path string) any {
	switch typeName {
	case "StructProperty":
		return r.StructValue(structType, path)
	case "EnumProperty", "NameProperty", "StrProperty":
		return r.FString()
	case "IntProperty":
		return r.I32()
	case "BoolProperty":
		return r.Bool()
	case "UInt32Property":
		return r.U32()
	case "Int64Property":
		return r.I64()
	default:
		panic("sav: unknown prop_value type " + typeName + " (" + path + ")")
	}
}

// ---- Writer ----

// Properties writes an ordered property sequence plus the "None" terminator.
func (w *FArchiveWriter) Properties(pl PropertyList) {
	for _, e := range pl {
		w.FString(e.Name)
		w.Property(e.Value)
	}
	w.FString("None")
}

// Property writes one property: type name, an 8-byte size placeholder, the
// value, then the size patched in.
func (w *FArchiveWriter) Property(p map[string]any) {
	w.FString(p["type"].(string))
	sizePos := w.buf.Len()
	w.buf.Write(make([]byte, 8))
	size := w.propertyInner(p["type"].(string), p)
	binary.LittleEndian.PutUint64(w.buf.Bytes()[sizePos:sizePos+8], uint64(size))
}

func (w *FArchiveWriter) propertyInner(propertyType string, p map[string]any) int {
	if ct, ok := p["custom_type"].(string); ok {
		cp, ok := w.customProperties[ct]
		if !ok {
			panic("sav: unknown custom property type " + ct)
		}
		return cp.Encode(w, propertyType, p)
	}
	switch propertyType {
	case "StructProperty":
		return w.writeStruct(p)
	case "IntProperty":
		w.OptionalGuid(asUUID(p["id"]))
		w.I32(asI32(p["value"]))
		return 4
	case "UInt16Property":
		w.OptionalGuid(asUUID(p["id"]))
		w.U16(asU16(p["value"]))
		return 2
	case "UInt32Property":
		w.OptionalGuid(asUUID(p["id"]))
		w.U32(asU32(p["value"]))
		return 4
	case "UInt64Property":
		w.OptionalGuid(asUUID(p["id"]))
		w.U64(asU64(p["value"]))
		return 8
	case "Int64Property":
		w.OptionalGuid(asUUID(p["id"]))
		w.I64(asI64(p["value"]))
		return 8
	case "FixedPoint64Property":
		w.OptionalGuid(asUUID(p["id"]))
		w.I32(asI32(p["value"]))
		return 4
	case "FloatProperty":
		w.OptionalGuid(asUUID(p["id"]))
		w.Float(asF32(p["value"]))
		return 4
	case "StrProperty", "NameProperty":
		w.OptionalGuid(asUUID(p["id"]))
		return w.fstringLen(p["value"].(string))
	case "EnumProperty":
		ev := p["value"].(map[string]any)
		w.FString(ev["type"].(string))
		w.OptionalGuid(asUUID(p["id"]))
		return w.fstringLen(ev["value"].(string))
	case "BoolProperty":
		w.Bool(p["value"].(bool))
		w.OptionalGuid(asUUID(p["id"]))
		return 0
	case "ByteProperty":
		ev := p["value"].(map[string]any)
		w.FString(ev["type"].(string))
		w.OptionalGuid(asUUID(p["id"]))
		if ev["type"] == "None" {
			w.Byte(ev["value"].(byte))
			return 1
		}
		return w.fstringLen(ev["value"].(string))
	case "ArrayProperty":
		w.FString(p["array_type"].(string))
		w.OptionalGuid(asUUID(p["id"]))
		start := w.buf.Len()
		w.writeArrayProperty(p["array_type"].(string), p["value"].(map[string]any))
		return w.buf.Len() - start
	case "MapProperty":
		return w.writeMapProperty(p)
	case "SetProperty":
		return w.writeSetProperty(p)
	default:
		panic("sav: unknown property type (write) " + propertyType)
	}
}

func (w *FArchiveWriter) writeStruct(p map[string]any) int {
	w.FString(p["struct_type"].(string))
	w.Guid(asUUID(p["struct_id"]))
	w.OptionalGuid(asUUID(p["id"]))
	start := w.buf.Len()
	w.StructValue(p["struct_type"].(string), p["value"])
	return w.buf.Len() - start
}

// StructValue writes a struct value, dispatching on struct type.
func (w *FArchiveWriter) StructValue(structType string, value any) {
	switch structType {
	case "Vector":
		v := value.(map[string]any)
		w.Double(asF64(v["x"]))
		w.Double(asF64(v["y"]))
		w.Double(asF64(v["z"]))
	case "DateTime":
		w.U64(asU64(value))
	case "Guid":
		w.Guid(asUUID(value))
	case "Quat":
		v := value.(map[string]any)
		w.Double(asF64(v["x"]))
		w.Double(asF64(v["y"]))
		w.Double(asF64(v["z"]))
		w.Double(asF64(v["w"]))
	case "LinearColor":
		v := value.(map[string]any)
		w.Float(asF32(v["r"]))
		w.Float(asF32(v["g"]))
		w.Float(asF32(v["b"]))
		w.Float(asF32(v["a"]))
	case "Color":
		v := value.(map[string]any)
		w.Byte(v["b"].(byte))
		w.Byte(v["g"].(byte))
		w.Byte(v["r"].(byte))
		w.Byte(v["a"].(byte))
	default:
		w.Properties(value.(PropertyList))
	}
}

func (w *FArchiveWriter) writeArrayProperty(arrayType string, value map[string]any) {
	if arrayType == "ByteProperty" {
		buf := value["values"].([]byte)
		w.U32(uint32(len(buf)))
		w.Write(buf)
		return
	}
	values := value["values"].([]any)
	w.U32(uint32(len(values)))
	if arrayType == "StructProperty" {
		w.FString(value["prop_name"].(string))
		w.FString(value["prop_type"].(string))
		sizePos := w.buf.Len()
		w.buf.Write(make([]byte, 8))
		w.FString(value["type_name"].(string))
		w.Guid(asUUID(value["id"]))
		w.Byte(0)
		dataStart := w.buf.Len()
		for _, v := range values {
			w.StructValue(value["type_name"].(string), v)
		}
		binary.LittleEndian.PutUint64(w.buf.Bytes()[sizePos:sizePos+8], uint64(w.buf.Len()-dataStart))
		return
	}
	for _, v := range values {
		w.arrayValueWrite(arrayType, v)
	}
}

func (w *FArchiveWriter) arrayValueWrite(arrayType string, v any) {
	switch arrayType {
	case "IntProperty":
		w.I32(asI32(v))
	case "UInt32Property":
		w.U32(asU32(v))
	case "Int64Property":
		w.I64(asI64(v))
	case "FloatProperty":
		w.Float(asF32(v))
	case "StrProperty", "NameProperty", "EnumProperty":
		w.FString(v.(string))
	case "BoolProperty":
		w.Bool(v.(bool))
	case "ByteProperty":
		w.Byte(v.(byte))
	case "Guid":
		w.Guid(asUUID(v))
	default:
		panic("sav: unknown array value type (write) " + arrayType)
	}
}

func (w *FArchiveWriter) writeMapProperty(p map[string]any) int {
	w.FString(p["key_type"].(string))
	w.FString(p["value_type"].(string))
	w.OptionalGuid(asUUID(p["id"]))
	start := w.buf.Len()
	w.U32(0)
	entries := p["value"].([]map[string]any)
	w.U32(uint32(len(entries)))
	for _, e := range entries {
		w.propValueWrite(p["key_type"].(string), p["key_struct_type"].(string), e["key"])
		w.propValueWrite(p["value_type"].(string), p["value_struct_type"].(string), e["value"])
	}
	return w.buf.Len() - start
}

func (w *FArchiveWriter) writeSetProperty(p map[string]any) int {
	w.FString(p["set_type"].(string))
	w.OptionalGuid(asUUID(p["id"]))
	start := w.buf.Len()
	w.U32(0)
	values := p["value"].([]PropertyList)
	w.U32(uint32(len(values)))
	for _, pl := range values {
		w.Properties(pl)
	}
	return w.buf.Len() - start
}

func (w *FArchiveWriter) propValueWrite(typeName, structType string, value any) {
	switch typeName {
	case "StructProperty":
		w.StructValue(structType, value)
	case "EnumProperty", "NameProperty", "StrProperty":
		w.FString(value.(string))
	case "IntProperty":
		w.I32(asI32(value))
	case "BoolProperty":
		w.Bool(value.(bool))
	case "UInt32Property":
		w.U32(asU32(value))
	case "Int64Property":
		w.I64(asI64(value))
	default:
		panic("sav: unknown prop_value type (write) " + typeName)
	}
}

// fstringLen writes an FString and returns its serialized byte length.
func (w *FArchiveWriter) fstringLen(s string) int {
	start := w.buf.Len()
	w.FString(s)
	return w.buf.Len() - start
}

// ---- typed accessors over JSON-like any values ----

func asUUID(v any) *UUID {
	if v == nil {
		return nil
	}
	if u, ok := v.(*UUID); ok {
		return u
	}
	return nil
}
func asI32(v any) int32 { return v.(int32) }
func asU16(v any) uint16 {
	switch x := v.(type) {
	case uint16:
		return x
	case int32:
		return uint16(x)
	}
	return v.(uint16)
}
func asU32(v any) uint32 { return v.(uint32) }
func asU64(v any) uint64 {
	switch x := v.(type) {
	case uint64:
		return x
	case int64:
		return uint64(x)
	}
	return v.(uint64)
}
func asI64(v any) int64   { return v.(int64) }
func asF32(v any) float32 { return v.(float32) }
func asF64(v any) float64 { return v.(float64) }
