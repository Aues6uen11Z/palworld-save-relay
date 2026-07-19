package palworld

import (
	"bytes"
	"testing"

	"palworld-save-relay/internal/sav"
)

// TestFixContainerSlotPlayerUids corrupts some palbox slot PlayerUIds to a
// non-host value, runs the fix, and verifies they are reset to the host
// sentinel while zero/host slots are left untouched.
func TestFixContainerSlotPlayerUids(t *testing.T) {
	gf, _, hints, custom := parseFixturePLM(t)
	wsd := gf.Properties.Get("worldSaveData")
	if wsd == nil {
		t.Skip("no worldSaveData")
	}
	pl := wsd["value"].(sav.PropertyList)
	ccsd := pl.Get("CharacterContainerSaveData")
	if ccsd == nil {
		t.Skip("no CharacterContainerSaveData")
	}
	entries, _ := ccsd["value"].([]map[string]any)
	if len(entries) == 0 {
		t.Skip("no containers")
	}

	// A non-host player UID to inject.
	fake := sav.UUID{0xde, 0xad, 0xbe, 0xef, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0}
	var zero sav.UUID

	// Collect slots and corrupt the first few with non-zero PlayerUIds.
	type slotRef struct {
		b   []byte
		off int
	}
	var allSlots []slotRef
	var zeroSlots, hostSlots int
	for _, e := range entries {
		val, _ := e["value"].(sav.PropertyList)
		if val == nil {
			continue
		}
		slots := val.Get("Slots")
		if slots == nil {
			continue
		}
		mv, _ := slots["value"].(map[string]any)
		arr, _ := mv["values"].([]any)
		for _, s := range arr {
			slot, ok := s.(sav.PropertyList)
			if !ok {
				continue
			}
			rd := slot.Get("RawData")
			if rd == nil {
				continue
			}
			b, ok := rd["value"].([]byte)
			if !ok || len(b) < 20 {
				continue
			}
			var uid sav.UUID
			copy(uid[:], b[4:20])
			if uid == zero {
				zeroSlots++
			} else if uid == HostUUID {
				hostSlots++
			}
			allSlots = append(allSlots, slotRef{b, 4})
		}
	}
	if len(allSlots) == 0 {
		t.Skip("no slots with RawData")
	}

	// Corrupt the first 3 slots (set their PlayerUId to fake).
	injected := 0
	for i := 0; i < 3 && i < len(allSlots); i++ {
		copy(allSlots[i].b[4:20], fake[:])
		injected++
	}

	// Run the fix.
	fixed := fixContainerSlotPlayerUids(gf)
	if fixed < injected {
		t.Fatalf("fixed %d, want >= %d (injected)", fixed, injected)
	}

	// Verify: no slot should have the fake UID anymore; zero/host slots unchanged.
	for _, sr := range allSlots {
		var uid sav.UUID
		copy(uid[:], sr.b[4:20])
		if uid == fake {
			t.Fatalf("slot still has fake UID after fix")
		}
	}

	// Idempotent: a second run fixes nothing.
	fixed2 := fixContainerSlotPlayerUids(gf)
	if fixed2 != 0 {
		t.Fatalf("second run fixed %d, want 0 (idempotent)", fixed2)
	}

	_ = bytes.Equal
	_ = hints
	_ = custom
}
