package palworld

import (
	"os"
	"testing"

	"palworld-save-relay/internal/sav"
)

func readSavFixtureX(t *testing.T, name string) []byte {
	t.Helper()
	b, err := os.ReadFile("../sav/testdata/" + name)
	if os.IsNotExist(err) {
		t.Skipf("fixture missing: %s", name)
	}
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	return b
}

// findHostGuest returns host InstanceId and a guest (InstanceId, PlayerUId).
func findHostGuest(t *testing.T, gvas []byte) (hostInst, guestInst, guestUID *sav.UUID) {
	t.Helper()
	hints, custom := sav.PalWorldConfig()
	gf, err := sav.ReadGvasFile(gvas, hints, custom)
	if err != nil {
		t.Fatalf("ReadGvasFile: %v", err)
	}
	wsd := gf.Properties.Get("worldSaveData")
	cspm := wsd["value"].(sav.PropertyList).Get("CharacterSaveParameterMap")
	for _, e := range cspm["value"].([]map[string]any) {
		key := e["key"].(sav.PropertyList)
		val := e["value"].(sav.PropertyList)
		raw := val.Get("RawData")
		if raw == nil {
			continue
		}
		rv, _ := raw["value"].(map[string]any)
		obj, _ := rv["object"].(sav.PropertyList)
		sp := obj.Get("SaveParameter")
		if sp == nil {
			continue
		}
		inner, _ := sp["value"].(sav.PropertyList)
		if inner == nil {
			continue
		}
		ip := inner.Get("IsPlayer")
		if ip == nil {
			continue
		}
		isPlayer, _ := ip["value"].(bool)
		if !isPlayer {
			continue
		}
		inst, _ := key.Get("InstanceId")["value"].(*sav.UUID)
		puid, _ := key.Get("PlayerUId")["value"].(*sav.UUID)
		if puid.Equal(&HostUUID) {
			hostInst = inst
		} else if guestInst == nil {
			guestInst = inst
			guestUID = puid
		}
	}
	if hostInst == nil || guestInst == nil {
		t.Fatalf("host/guest not found")
	}
	return
}
