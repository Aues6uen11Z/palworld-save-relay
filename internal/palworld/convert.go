package palworld

import (
	"bytes"

	"palworld-save-relay/internal/sav"
)

// rawByteReplaceGUIDs replaces every 16-byte occurrence of from -> to in data.
// Used on opaque RawData blobs (map objects / base camps / item containers)
// whose ownership UIDs aren't individually parsed.
func rawByteReplaceGUIDs(data []byte, from, to sav.UUID) {
	for i := 0; i+16 <= len(data); {
		if bytes.Equal(data[i:i+16], from[:]) {
			copy(data[i:i+16], to[:])
			i += 16
		} else {
			i++
		}
	}
}

// replaceOpaqueGUIDs walks the property tree and raw-byte-replaces from -> to
// inside every opaque RawData blob (maps carrying a "skip_type" marker).
func replaceOpaqueGUIDs(v any, from, to sav.UUID) {
	switch x := v.(type) {
	case sav.PropertyList:
		for _, e := range x {
			replaceOpaqueGUIDs(e.Value, from, to)
		}
	case map[string]any:
		if _, ok := x["skip_type"]; ok {
			if raw, ok := x["value"].([]byte); ok {
				rawByteReplaceGUIDs(raw, from, to)
			}
		}
		for _, val := range x {
			replaceOpaqueGUIDs(val, from, to)
		}
	case []any:
		for _, item := range x {
			replaceOpaqueGUIDs(item, from, to)
		}
	case []map[string]any:
		for _, item := range x {
			replaceOpaqueGUIDs(item, from, to)
		}
	case []sav.PropertyList:
		for _, item := range x {
			replaceOpaqueGUIDs(item, from, to)
		}
	}
}

// replaceFields are the parsed UID-bearing fields deepReplace rewrites.
var replaceFields = map[string]bool{
	"PlayerUId":                          true, // CSPM key + player SaveData + IndividualId
	"OwnerPlayerUId":                     true, // character (pal) ownership
	"guid":                               true, // individual character handle (player UID)
	"OldOwnerPlayerUIds":                 true, // old pal owner history (array)
	"LastNickNameModifierPlayerUid":      true, // last pal nickname modifier
	"admin_player_uid":                   true, // guild admin
	"player_uid":                         true, // guild member / independent guild owner
	"last_guild_name_modifier_player_uid": true, // guild name modifier
}

// hostIdentityFields is the restricted field set used by a host step-down: it
// rewrites the host player's identity and pal ownership, but NOT the pal CSPM
// bucket (PlayerUId) or ICH guid (guid) - those stay on the host sentinel slot
// so the next host inherits the world's pals, matching an official host world.
var hostIdentityFields = map[string]bool{
	"OwnerPlayerUId":                      true,
	"OldOwnerPlayerUIds":                  true,
	"LastNickNameModifierPlayerUid":       true,
	"admin_player_uid":                    true,
	"player_uid":                          true,
	"last_guild_name_modifier_player_uid": true,
}

func uidPtr(u sav.UUID) *sav.UUID { v := u; return &v }

// deepReplace recurses the parsed property tree, rewriting named UID fields
// from -> to (by value). InstanceId / struct_id / id are left untouched.
func deepReplace(v any, from, to sav.UUID) {
	deepReplaceFields(v, from, to, replaceFields)
}

// deepReplaceFields is deepReplace with a caller-supplied field set, so a host
// step-down can rewrite only identity/ownership fields and leave world data
// (pal CSPM buckets, ICH guids) on the host slot.
func deepReplaceFields(v any, from, to sav.UUID, fields map[string]bool) {
	switch x := v.(type) {
	case sav.PropertyList:
		for _, e := range x {
			if fields[e.Name] {
				replaceGUIDValue(e.Value, from, to)
			}
			deepReplaceFields(e.Value, from, to, fields)
		}
	case map[string]any:
		for k, val := range x {
			// Structured RawData (e.g. group/guild decode) stores UID fields as
			// bare *UUID values keyed by name; replace those in-place.
			if fields[k] {
				if g, ok := val.(*sav.UUID); ok && g.Equal(&from) {
					x[k] = uidPtr(to)
					continue
				}
				// Value-type UUID from decoded blobs
				if g, ok := val.(sav.UUID); ok && g.Equal(&from) {
					x[k] = to
					continue
				}
			}
			deepReplaceFields(val, from, to, fields)
		}
	case []any:
		for _, item := range x {
			deepReplaceFields(item, from, to, fields)
		}
	case []map[string]any:
		for _, item := range x {
			deepReplaceFields(item, from, to, fields)
		}
	case []sav.PropertyList:
		for _, item := range x {
			deepReplaceFields(item, from, to, fields)
		}
	}
}

func replaceGUIDValue(m map[string]any, from, to sav.UUID) {
	if g, ok := m["value"].(*sav.UUID); ok {
		if g.Equal(&from) {
			m["value"] = uidPtr(to)
		}
		return
	}
	// Value-type UUID from decoded blobs (groupDecode, characterDecode, etc.)
	if g, ok := m["value"].(sav.UUID); ok {
		if g.Equal(&from) {
			m["value"] = to
		}
		return
	}
	// ArrayProperty stored as map[string]any{"values": []any{...}} (e.g. OldOwnerPlayerUIds)
	if nested, ok := m["value"].(map[string]any); ok {
		if arr, ok := nested["values"].([]any); ok {
			for i, item := range arr {
				if g, ok := item.(*sav.UUID); ok && g.Equal(&from) {
					arr[i] = uidPtr(to)
				} else if g, ok := item.(sav.UUID); ok && g.Equal(&from) {
					arr[i] = to
				}
			}
		}
	}
	// Array of UUIDs from decoded blobs (e.g. OldOwnerPlayerUIds)
	if arr, ok := m["value"].([]any); ok {
		for i, item := range arr {
			if g, ok := item.(*sav.UUID); ok && g.Equal(&from) {
				arr[i] = uidPtr(to)
			} else if g, ok := item.(sav.UUID); ok && g.Equal(&from) {
				arr[i] = to
			} else if wrapped, ok := item.(map[string]any); ok {
				if g, ok := wrapped["value"].(*sav.UUID); ok && g.Equal(&from) {
					wrapped["value"] = uidPtr(to)
				} else if g, ok := wrapped["value"].(sav.UUID); ok && g.Equal(&from) {
					wrapped["value"] = to
				}
			}
		}
	}
}

// ConvertGvas does a uniform from -> to UID swap across the whole GVAS tree
// (named fields + opaque blobs). Used for player .sav files (personal data).
func ConvertGvas(gf *sav.GvasFile, fromUID, toUID sav.UUID) {
	deepReplace(gf.Properties, fromUID, toUID)
	replaceOpaqueGUIDs(gf.Properties, fromUID, toUID)
}
