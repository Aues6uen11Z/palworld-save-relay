# UID Replacement Bug — Final Resolution

## Root Cause
The `convert.go` UID replacement had two types of gaps:

### 1. Missing field names in `replaceFields`
Added: `OldOwnerPlayerUIds`, `LastNickNameModifierPlayerUid`, `guid`

### 2. `replaceGUIDValue` didn't handle all UUID storage formats
The original function only checked `m["value"]` for:
- `*sav.UUID` (pointer — PropertyList standard)
- `sav.UUID` (value type — decoded maps like group/guild data)

**Missing formats** (now handled):
- `[]any` array of UUIDs at `m["value"].([]any)`
- `map[string]any{"values": []any{...}}` — ArrayProperty nested inside decoded data (e.g. `OldOwnerPlayerUIds`)
- `map[string]any` wrapped UUIDs inside arrays (e.g. `{"value": *sav.UUID{...}}`)
- `sav.UUID` value type in `deepReplace` map iteration

### 3. `replaceOpaqueGUIDs` didn't process bare `[]byte` values
Added `[]byte` case to handle raw byte arrays found during tree walk.

## Verification
Test with `1784026577265__GYF.zip` against player UID `f5a9a2d9-0000-0000-0000-000000000000`:

```
BEFORE: 467 total (461 structured UUIDs + 6 byte-level matches)
AFTER:  0 total — ZERO LEAKS
```

## Final `replaceFields`:
```
PlayerUId                        — CSPM key + SaveData
OwnerPlayerUId                   — Pal ownership
OldOwnerPlayerUIds               — Pal ownership history (array)
LastNickNameModifierPlayerUid    — Last pal nickname modifier
guid                             — Individual character handle (player UID in group data)
admin_player_uid                 — Guild admin (value-based — only replaces if equals fromUID)
player_uid                       — Guild member list (value-based)
last_guild_name_modifier_player_uid — Guild name modifier (value-based)
```
