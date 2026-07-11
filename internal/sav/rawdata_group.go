package sav

import "bytes"

// groupDecode parses GroupSaveDataMap (guilds) RawData. Ported from
// cheahjs palsav/rawdata/group.py. The MapProperty is read normally (nested
// guard), then each group's RawData byte blob is decoded per group type.
func groupDecode(r *FArchiveReader, typeName string, size int, path string) map[string]any {
	value := r.propertyWithNested(typeName, size, path, path)
	entries, _ := value["value"].([]map[string]any)
	for _, e := range entries {
		gv, _ := e["value"].(PropertyList) // group value struct is a property sequence
		if gv == nil {
			continue
		}
		raw := gv.Get("RawData")
		if raw == nil {
			continue
		}
		inner, _ := raw["value"].(map[string]any)
		if inner == nil {
			continue
		}
		bytes, _ := inner["values"].([]byte)
		raw["value"] = groupDecodeBytes(r, bytes, groupType(gv))
	}
	return value
}

// groupType reads GroupType from a group's value PropertyList.
func groupType(gv PropertyList) string {
	gt := gv.Get("GroupType")
	if gt == nil {
		return ""
	}
	v, _ := gt["value"].(map[string]any)
	if v == nil {
		return ""
	}
	s, _ := v["value"].(string)
	return s
}

var v1Marker = []byte{0x02, 0x00, 0x00, 0x00, 0x02, 0x03, 0x00, 0x00, 0x00, 0x00}

func groupDecodeBytes(parent *FArchiveReader, b []byte, groupType string) map[string]any {
	r := parent.InternalCopy(b)
	d := map[string]any{"group_type": groupType}
	d["group_id"] = r.Guid()
	d["group_name"] = r.FString()
	d["individual_character_handle_ids"] = r.TArray(func() any {
		return map[string]any{"guid": r.Guid(), "instance_id": r.Guid()}
	})

	switch groupType {
	case "EPalGroupType::Organization":
		d["org_type"] = r.Byte()
		d["trailing_bytes"] = r.ByteList(12)
		if !r.EOF() {
			d["unknown_bytes"] = r.ReadToEnd()
		}
		return d
	case "EPalGroupType::IndependentGuild":
		d["org_type"] = r.Byte()
		d["base_camp_level"] = r.I32()
		d["map_object_instance_ids_base_camp_points"] = r.TArray(func() any { return r.Guid() })
		d["guild_name"] = r.FString()
		d["player_uid"] = r.Guid()
		d["guild_name_2"] = r.FString()
		d["player_info"] = map[string]any{
			"last_online_real_time": r.I64(),
			"player_name":           r.FString(),
		}
		if !r.EOF() {
			d["unknown_bytes"] = r.ReadToEnd()
		}
		return d
	case "EPalGroupType::Guild":
		d["org_type"] = r.Byte()
		d["leading_bytes"] = r.ByteList(4)
		d["base_ids"] = r.TArray(func() any { return r.Guid() })
		d["unknown_1"] = r.I32()
		d["base_camp_level"] = r.I32()
		d["map_object_instance_ids_base_camp_points"] = r.TArray(func() any { return r.Guid() })
		d["guild_name"] = r.FString()
		d["last_guild_name_modifier_player_uid"] = r.Guid()
		d["unknown_2"] = r.ByteList(4)
		tail := r.ReadToEnd()
		originalTail := tail
		post := tail
		if vi := bytes.Index(post, v1Marker); vi >= 0 {
			d["_has_v1_marker"] = true
			if pre := post[:vi]; len(pre) > 0 {
				d["_pre_v1_bytes"] = pre
			}
			post = post[vi+len(v1Marker):]
		}
		if !groupDecodeGuildTail(parent, post, d) {
			d["_raw_tail"] = originalTail
		}
		if _, ok := d["players"]; !ok {
			d["players"] = []map[string]any{}
		}
		return d
	default:
		if !r.EOF() {
			d["unknown_bytes"] = r.ReadToEnd()
		}
		return d
	}
}

// groupDecodeGuildTail parses admin_player_uid + players from the guild tail.
// Returns false on failure (caller stores _raw_tail).
func groupDecodeGuildTail(parent *FArchiveReader, post []byte, d map[string]any) bool {
	defer func() bool { recover(); return false }()
	sub := parent.InternalCopy(post)
	admin := sub.Guid()
	count := int(sub.I32())
	hasV1, _ := d["_has_v1_marker"].(bool)
	players := make([]map[string]any, 0, count)
	for i := 0; i < count; i++ {
		puid := sub.Guid()
		lt := sub.I64()
		nm := sub.FString()
		p := map[string]any{
			"player_uid":  puid,
			"player_info": map[string]any{"last_online_real_time": lt, "player_name": nm},
		}
		if hasV1 && !sub.EOF() {
			p["_u8_flag"] = sub.Byte()
		}
		players = append(players, p)
	}
	d["admin_player_uid"] = admin
	d["players"] = players
	if !sub.EOF() {
		d["_trailing_bytes"] = sub.ReadToEnd()
	}
	return true
}

// groupEncode reverses groupDecode.
func groupEncode(w *FArchiveWriter, propertyType string, p map[string]any) int {
	delete(p, "custom_type")
	entries, _ := p["value"].([]map[string]any)
	for _, e := range entries {
		gv, _ := e["value"].(PropertyList)
		if gv == nil {
			continue
		}
		raw := gv.Get("RawData")
		if raw == nil {
			continue
		}
		inner, ok := raw["value"].(map[string]any)
		if !ok {
			continue
		}
		if _, isBytes := inner["values"]; isBytes {
			continue
		}
		raw["value"] = map[string]any{"values": groupEncodeBytes(inner)}
	}
	return w.propertyInnerNoCustom(propertyType, p)
}

func groupEncodeBytes(d map[string]any) []byte {
	w := NewFArchiveWriter(nil)
	w.Guid(asUUID(d["group_id"]))
	w.FString(d["group_name"].(string))
	w.TArray(func(v any) {
		m := v.(map[string]any)
		w.Guid(asUUID(m["guid"]))
		w.Guid(asUUID(m["instance_id"]))
	}, d["individual_character_handle_ids"].([]any))

	gt, _ := d["group_type"].(string)
	switch gt {
	case "EPalGroupType::Organization":
		w.Byte(d["org_type"].(byte))
		w.Write(d["trailing_bytes"].([]byte))
		if ub, ok := d["unknown_bytes"]; ok {
			w.Write(ub.([]byte))
		}
	case "EPalGroupType::IndependentGuild":
		w.Byte(d["org_type"].(byte))
		w.I32(d["base_camp_level"].(int32))
		w.TArray(func(v any) { w.Guid(asUUID(v)) }, d["map_object_instance_ids_base_camp_points"].([]any))
		w.FString(d["guild_name"].(string))
		w.Guid(asUUID(d["player_uid"]))
		w.FString(d["guild_name_2"].(string))
		pi := d["player_info"].(map[string]any)
		w.I64(pi["last_online_real_time"].(int64))
		w.FString(pi["player_name"].(string))
		if ub, ok := d["unknown_bytes"]; ok {
			w.Write(ub.([]byte))
		}
	case "EPalGroupType::Guild":
		w.Byte(d["org_type"].(byte))
		w.Write(d["leading_bytes"].([]byte))
		w.TArray(func(v any) { w.Guid(asUUID(v)) }, d["base_ids"].([]any))
		w.I32(d["unknown_1"].(int32))
		w.I32(d["base_camp_level"].(int32))
		w.TArray(func(v any) { w.Guid(asUUID(v)) }, d["map_object_instance_ids_base_camp_points"].([]any))
		w.FString(d["guild_name"].(string))
		w.Guid(asUUID(d["last_guild_name_modifier_player_uid"]))
		w.Write(d["unknown_2"].([]byte))
		if rt, ok := d["_raw_tail"]; ok {
			w.Write(rt.([]byte))
		} else {
			if hasV1, _ := d["_has_v1_marker"].(bool); hasV1 {
				if pre, ok := d["_pre_v1_bytes"]; ok {
					w.Write(pre.([]byte))
				}
				w.Write(v1Marker)
			}
			w.Guid(asUUID(d["admin_player_uid"]))
			players, _ := d["players"].([]map[string]any)
			w.TArray(func(v any) {
				m := v.(map[string]any)
				w.Guid(asUUID(m["player_uid"]))
				pi := m["player_info"].(map[string]any)
				w.I64(pi["last_online_real_time"].(int64))
				w.FString(pi["player_name"].(string))
				if f, ok := m["_u8_flag"]; ok {
					w.Byte(f.(byte))
				}
			}, toAnySlice(players))
			if tb, ok := d["_trailing_bytes"]; ok {
				w.Write(tb.([]byte))
			}
		}
		if ub, ok := d["unknown_bytes"]; ok {
			w.Write(ub.([]byte))
		}
	default:
		if ub, ok := d["unknown_bytes"]; ok {
			w.Write(ub.([]byte))
		}
	}
	return w.Bytes()
}

func toAnySlice(ms []map[string]any) []any {
	out := make([]any, len(ms))
	for i, m := range ms {
		out[i] = m
	}
	return out
}

// groupDecodeSafe wraps groupDecode with an opaque-skip fallback.
func groupDecodeSafe(r *FArchiveReader, typeName string, size int, path string) (val map[string]any) {
	start := r.pos
	defer func() {
		if rec := recover(); rec != nil {
			r.pos = start
			val = skipDecode(r, typeName, size, path)
			val["custom_type"] = path
			val["__skip__"] = true
		}
	}()
	val = groupDecode(r, typeName, size, path)
	val["__skip__"] = false
	return
}

func groupEncodeSafe(w *FArchiveWriter, propertyType string, p map[string]any) int {
	if skip, ok := p["__skip__"].(bool); ok && skip {
		delete(p, "custom_type")
		delete(p, "__skip__")
		return skipEncode(w, propertyType, p)
	}
	delete(p, "__skip__")
	return groupEncode(w, propertyType, p)
}
