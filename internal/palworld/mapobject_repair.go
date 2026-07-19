package palworld

import (
	"bytes"
	"encoding/binary"
	"fmt"

	"palworld-save-relay/internal/sav"
)

// mapObjectBuilderOffset is the byte offset of the builder (placer) player UID
// within each map object's Model.RawData blob. Verified against live saves:
// Model.RawData is a fixed 261-byte layout (4 Guids + 2 int32 + transform +
// 3 optional Guids + builder UID at offset 200 + trailing data). The builder
// field is always at offset 200 across all facility types.
const mapObjectBuilderOffset = 200

// findMapObjectSkipMap locates the opaque skip map for
// .worldSaveData.MapObjectSaveData in the parsed property tree and returns it
// (so callers can read/replace its "value" []byte slot directly). Returns nil
// if not present.
func findMapObjectSkipMap(props sav.PropertyList) map[string]any {
	var found map[string]any
	var rec func(v any)
	rec = func(v any) {
		switch x := v.(type) {
		case sav.PropertyList:
			for _, e := range x {
				rec(e.Value)
			}
		case map[string]any:
			if ct, ok := x["custom_type"].(string); ok && ct == ".worldSaveData.MapObjectSaveData" {
				if _, ok := x["value"].([]byte); ok {
					found = x
					return
				}
			}
			for _, val := range x {
				rec(val)
			}
		case []map[string]any:
			for _, m := range x {
				rec(m)
			}
		case []any:
			for _, m := range x {
				rec(m)
			}
		}
	}
	rec(props)
	return found
}

// parseMapObjectArray parses an opaque MapObjectSaveData blob (the ArrayProperty
// content: count + StructProperty header + elements) into a structured form.
func parseMapObjectArray(blob []byte, hints map[string]string, custom map[string]sav.CustomProperty) (map[string]any, error) {
	r := sav.NewFArchiveReader(blob, hints, custom)
	count := r.U32()
	propName := r.FString()
	propType := r.FString()
	r.U64()
	typeName := r.FString()
	id := r.Guid()
	r.Byte()
	basePath := ".worldSaveData.MapObjectSaveData." + propName
	values := make([]any, 0, count)
	for i := uint32(0); i < count; i++ {
		values = append(values, r.PropertiesUntilEnd(basePath))
	}
	if !r.EOF() {
		return nil, fmt.Errorf("palworld: mapobject parse: %d trailing bytes", len(blob)-r.Pos())
	}
	return map[string]any{
		"prop_name": propName, "prop_type": propType, "type_name": typeName,
		"id": id, "values": values,
	}, nil
}

// encodeMapObjectArray encodes a parsed MapObjectSaveData array back to its blob
// bytes (inverse of parseMapObjectArray). Round-trips byte-identically.
func encodeMapObjectArray(parsed map[string]any, custom map[string]sav.CustomProperty) []byte {
	w := sav.NewFArchiveWriter(custom)
	values := parsed["values"].([]any)
	w.U32(uint32(len(values)))
	w.FString(parsed["prop_name"].(string))
	w.FString(parsed["prop_type"].(string))
	sizePos := w.Len()
	w.Write(make([]byte, 8))
	w.FString(parsed["type_name"].(string))
	w.Guid(parsed["id"].(*sav.UUID))
	w.Byte(0)
	dataStart := w.Len()
	for _, v := range values {
		w.StructValue(parsed["type_name"].(string), v)
	}
	binary.LittleEndian.PutUint64(w.Bytes()[sizePos:sizePos+8], uint64(w.Len()-dataStart))
	return w.Bytes()
}

// findModelRawDataBytes walks a single map object's properties to Model > RawData
// and returns its byte values (the 261-byte layout containing the builder UID).
func findModelRawDataBytes(fac sav.PropertyList) []byte {
	model := fac.Get("Model")
	if model == nil {
		return nil
	}
	modelPL, ok := model["value"].(sav.PropertyList)
	if !ok {
		return nil
	}
	rd := modelPL.Get("RawData")
	if rd == nil {
		return nil
	}
	rv, _ := rd["value"].(map[string]any)
	if rv == nil {
		return nil
	}
	b, _ := rv["values"].([]byte)
	return b
}

// setModelRawDataBytes replaces the Model > RawData byte values for a facility.
func setModelRawDataBytes(fac sav.PropertyList, newBytes []byte) bool {
	model := fac.Get("Model")
	if model == nil {
		return false
	}
	modelPL, ok := model["value"].(sav.PropertyList)
	if !ok {
		return false
	}
	rd := modelPL.Get("RawData")
	if rd == nil {
		return false
	}
	rv, _ := rd["value"].(map[string]any)
	if rv == nil {
		return false
	}
	rv["values"] = newBytes
	return true
}

// mutateMapObjectBuilders parses the MapObjectSaveData blob, applies mutate to
// each facility's builder UID (16 bytes at offset 200 of Model.RawData), and
// re-encodes. mutate receives the current builder bytes and returns the
// replacement (or nil to leave unchanged). Returns the count of changed
// facilities. If there is no MapObjectSaveData, returns 0.
func mutateMapObjectBuilders(gf *sav.GvasFile, hints map[string]string, custom map[string]sav.CustomProperty, mutate func(builder []byte) []byte) (int, error) {
	skipMap := findMapObjectSkipMap(gf.Properties)
	if skipMap == nil {
		return 0, nil
	}
	blob, _ := skipMap["value"].([]byte)
	if len(blob) == 0 {
		return 0, nil
	}
	parsed, err := parseMapObjectArray(blob, hints, custom)
	if err != nil {
		return 0, fmt.Errorf("palworld: mapobject parse: %w", err)
	}
	values := parsed["values"].([]any)
	changed := 0
	for _, v := range values {
		fac, ok := v.(sav.PropertyList)
		if !ok {
			continue
		}
		rd := findModelRawDataBytes(fac)
		if len(rd) < mapObjectBuilderOffset+16 {
			continue
		}
		cur := rd[mapObjectBuilderOffset : mapObjectBuilderOffset+16]
		rep := mutate(cur)
		if rep == nil {
			continue
		}
		newRD := make([]byte, len(rd))
		copy(newRD, rd)
		copy(newRD[mapObjectBuilderOffset:mapObjectBuilderOffset+16], rep)
		setModelRawDataBytes(fac, newRD)
		changed++
	}
	if changed == 0 {
		return 0, nil
	}
	skipMap["value"] = encodeMapObjectArray(parsed, custom)
	return changed, nil
}

// RemapMapObjectBuilders replaces every facility builder UID equal to from with
// to (exact 16-byte match, at the fixed builder offset). Used by the host
// activation step so a player's guest-built facilities follow them onto the host
// slot, preventing orphaned builder references.
func RemapMapObjectBuilders(gf *sav.GvasFile, hints map[string]string, custom map[string]sav.CustomProperty, from, to sav.UUID) (int, error) {
	return mutateMapObjectBuilders(gf, hints, custom, func(b []byte) []byte {
		if bytes.Equal(b, from[:]) {
			return to[:]
		}
		return nil
	})
}

// RepairMapObjectBuilders repairs facility builder UIDs that no longer resolve to
// a current player. A builder is left untouched if it is all-zero (natural
// objects) or matches a current player UID (the host sentinel or any CSPM
// IsPlayer entry). Anything else - an orphaned former-player UID or a corrupted
// (non-player-shaped) value - is reset to the host sentinel (0001) so the
// current host can work the facility and the inspect name resolves. Returns the
// number of facilities repaired.
func RepairMapObjectBuilders(gf *sav.GvasFile, hints map[string]string, custom map[string]sav.CustomProperty) (int, error) {
	valid := collectCurrentPlayerUIDs(gf)
	var zero sav.UUID
	return mutateMapObjectBuilders(gf, hints, custom, func(b []byte) []byte {
		if bytes.Equal(b, zero[:]) {
			return nil // all-zero: natural object, leave
		}
		var u sav.UUID
		copy(u[:], b)
		if valid[u] {
			return nil // current player, leave
		}
		return HostUUID[:] // orphan/corrupt -> host
	})
}

// collectCurrentPlayerUIDs returns the set of player UIDs present in the save:
// the host sentinel (0001) plus every CSPM IsPlayer entry.
func collectCurrentPlayerUIDs(gf *sav.GvasFile) map[sav.UUID]bool {
	out := map[sav.UUID]bool{HostUUID: true}
	wsd := gf.Properties.Get("worldSaveData")
	if wsd == nil {
		return out
	}
	pl, ok := wsd["value"].(sav.PropertyList)
	if !ok {
		return out
	}
	cspm := pl.Get("CharacterSaveParameterMap")
	if cspm == nil {
		return out
	}
	entries, _ := cspm["value"].([]map[string]any)
	for _, e := range entries {
		val, _ := e["value"].(sav.PropertyList)
		if val == nil {
			continue
		}
		raw := val.Get("RawData")
		if raw == nil {
			continue
		}
		rv, _ := raw["value"].(map[string]any)
		obj, _ := rv["object"].(sav.PropertyList)
		sp := obj.Get("SaveParameter")
		inner, _ := sp["value"].(sav.PropertyList)
		ip := inner.Get("IsPlayer")
		if ip == nil {
			continue
		}
		b, _ := ip["value"].(bool)
		if !b {
			continue
		}
		key, _ := e["key"].(sav.PropertyList)
		if key == nil {
			continue
		}
		if p := key.Get("PlayerUId"); p != nil {
			if u, ok := p["value"].(*sav.UUID); ok && u != nil {
				out[*u] = true
			}
		}
	}
	return out
}
