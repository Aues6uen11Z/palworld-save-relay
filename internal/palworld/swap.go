package palworld

import (
	"palworld-save-relay/internal/sav"
)

// uidPtr returns a pointer to a copy of u (so each swap target is independent).
func uidPtr(u sav.UUID) *sav.UUID { v := u; return &v }

// swapFields are the UID-bearing fields deepSwap swaps (ports fix_save's
// deep_swap). Only these named fields are touched, so there are no false
// positives from byte-pattern matching.
var swapFields = map[string]bool{
	"OwnerPlayerUId":          true,
	"owner_player_uid":        true,
	"build_player_uid":        true,
	"private_lock_player_uid": true,
}

// SwapLevelSav swaps host<->target in a Level.sav GvasFile:
//   - CharacterSaveParameterMap keys: the entry whose InstanceId == oldInst gets
//     PlayerUId = newUID; whose InstanceId == newInst gets PlayerUId = oldUID.
//   - deepSwap: recurses the parsed property tree swapping the UID value of the
//     named ownership fields (OwnerPlayerUId etc.) oldUID <-> newUID.
//
// Use the character-decoded config so character SaveParameters are parsed and
// deepSwap reaches pal ownership.
func SwapLevelSav(gf *sav.GvasFile, oldInst, newInst *sav.UUID, oldUID, newUID sav.UUID) {
	swapCSPMKeys(gf, oldInst, newInst, oldUID, newUID)
	deepSwap(gf.Properties, oldUID, newUID)
}

// swapCSPMKeys swaps the PlayerUId on the two CharacterSaveParameterMap entries
// identified by InstanceId.
func swapCSPMKeys(gf *sav.GvasFile, oldInst, newInst *sav.UUID, oldUID, newUID sav.UUID) {
	wsd := gf.Properties.Get("worldSaveData")
	if wsd == nil {
		return
	}
	cspm := wsd["value"].(sav.PropertyList).Get("CharacterSaveParameterMap")
	if cspm == nil {
		return
	}
	entries, ok := cspm["value"].([]map[string]any)
	if !ok {
		return
	}
	for _, e := range entries {
		key, _ := e["key"].(sav.PropertyList)
		if key == nil {
			continue
		}
		inst := key.Get("InstanceId")
		if inst == nil {
			continue
		}
		g, ok := inst["value"].(*sav.UUID)
		if !ok {
			continue
		}
		puid := key.Get("PlayerUId")
		if puid == nil {
			continue
		}
		switch {
		case g.Equal(oldInst):
			puid["value"] = uidPtr(newUID)
		case g.Equal(newInst):
			puid["value"] = uidPtr(oldUID)
		}
	}
}

// deepSwap recurses the property tree, swapping the GUID value of named
// ownership fields oldUID <-> newUID. An involution (swap twice = identity).
func deepSwap(v any, a, b sav.UUID) {
	switch x := v.(type) {
	case sav.PropertyList:
		for _, e := range x {
			if swapFields[e.Name] {
				swapGUIDValue(e.Value, a, b)
			}
			deepSwap(e.Value, a, b)
		}
	case map[string]any:
		for _, val := range x {
			deepSwap(val, a, b)
		}
	case []any:
		for _, item := range x {
			deepSwap(item, a, b)
		}
	case []map[string]any:
		for _, item := range x {
			deepSwap(item, a, b)
		}
	case []sav.PropertyList:
		for _, item := range x {
			deepSwap(item, a, b)
		}
	}
}

// swapGUIDValue swaps a *UUID "value" in a property map a <-> b.
func swapGUIDValue(m map[string]any, a, b sav.UUID) {
	g, ok := m["value"].(*sav.UUID)
	if !ok {
		return
	}
	switch {
	case g.Equal(&a):
		m["value"] = uidPtr(b)
	case g.Equal(&b):
		m["value"] = uidPtr(a)
	}
}

// SwapPlayerSav swaps PlayerUId a <-> b in a player .sav GvasFile
// (SaveData.PlayerUId and SaveData.IndividualId.PlayerUId).
func SwapPlayerSav(gf *sav.GvasFile, a, b sav.UUID) {
	sd := gf.Properties.Get("SaveData")
	if sd == nil {
		return
	}
	sdPL, _ := sd["value"].(sav.PropertyList)
	if sdPL == nil {
		return
	}
	swapGUIDField(sdPL, "PlayerUId", a, b)
	if ind := sdPL.Get("IndividualId"); ind != nil {
		if indPL, ok := ind["value"].(sav.PropertyList); ok {
			swapGUIDField(indPL, "PlayerUId", a, b)
		}
	}
}

func swapGUIDField(pl sav.PropertyList, name string, a, b sav.UUID) {
	f := pl.Get(name)
	if f == nil {
		return
	}
	swapGUIDValue(f, a, b)
}
