package palworld

import (
	"testing"

	"palworld-save-relay/internal/sav"
)

// TestFixOwnerlessPals removes OwnerPlayerUId from some pals, runs the fix,
// and verifies both OwnerPlayerUId and OwnedTime are restored.
func TestFixOwnerlessPals(t *testing.T) {
	gf, _, _, _ := parseFixturePLM(t)
	wsd := gf.Properties.Get("worldSaveData")
	if wsd == nil {
		t.Skip("no worldSaveData")
	}
	pl := wsd["value"].(sav.PropertyList)
	cspm := pl.Get("CharacterSaveParameterMap")
	if cspm == nil {
		t.Skip("no CSPM")
	}
	entries, _ := cspm["value"].([]map[string]any)

	// Remove OwnerPlayerUId and OwnedTime from the first 3 non-player pals.
	removed := 0
	for _, e := range entries {
		val, _ := e["value"].(sav.PropertyList)
		if val == nil {
			continue
		}
		raw := val.Get("RawData")
		rv, _ := raw["value"].(map[string]any)
		obj, _ := rv["object"].(sav.PropertyList)
		sp := obj.Get("SaveParameter")
		inner, _ := sp["value"].(sav.PropertyList)
		isPlayer, _ := inner.Get("IsPlayer")["value"].(bool)
		if isPlayer {
			continue
		}
		if inner.Get("OwnerPlayerUId") == nil {
			continue
		}
		if removed >= 3 {
			break
		}
		var newInner sav.PropertyList
		for _, fe := range inner {
			if fe.Name == "OwnerPlayerUId" || fe.Name == "OwnedTime" {
				continue
			}
			newInner = append(newInner, fe)
		}
		sp["value"] = newInner
		removed++
	}
	if removed == 0 {
		t.Skip("no pals with OwnerPlayerUId to remove")
	}

	// Run the fix.
	fixed := fixOwnerlessPals(gf)
	if fixed < removed {
		t.Fatalf("fixed %d, want >= %d", fixed, removed)
	}

	// Verify: no pal should be missing OwnerPlayerUId or OwnedTime.
	missingOwner := 0
	for _, e := range entries {
		val, _ := e["value"].(sav.PropertyList)
		if val == nil {
			continue
		}
		raw := val.Get("RawData")
		rv, _ := raw["value"].(map[string]any)
		obj, _ := rv["object"].(sav.PropertyList)
		sp := obj.Get("SaveParameter")
		inner, _ := sp["value"].(sav.PropertyList)
		isPlayer, _ := inner.Get("IsPlayer")["value"].(bool)
		if isPlayer {
			continue
		}
		if inner.Get("OwnerPlayerUId") == nil {
			missingOwner++
		}
		if inner.Get("OwnedTime") == nil {
			missingOwner++
		}
	}
	if missingOwner > 0 {
		t.Fatalf("%d pals still missing OwnerPlayerUId or OwnedTime after fix", missingOwner)
	}

	// Idempotent: second run fixes nothing.
	fixed2 := fixOwnerlessPals(gf)
	if fixed2 != 0 {
		t.Fatalf("second run fixed %d, want 0 (idempotent)", fixed2)
	}
}
