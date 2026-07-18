# Palworld Host Swap

[中文](README.md) | **English**

![license](https://img.shields.io/badge/license-MIT-blue) ![platform](https://img.shields.io/badge/platform-Windows-blue) ![go](https://img.shields.io/badge/Go-1.25-00ADD8)

> A desktop tool for **swapping the host** of a Palworld co-op save. Multiple players take turns being host while sharing one world's progress. **Single-file exe, zero install.**

## Download

Grab `palworld-save-relay.exe` from the [Releases](https://github.com/Aues6uen11Z/palworld-save-relay/releases) page and double-click to run.

> On first launch it auto-detects your save directory (`%LocalAppData%\Pal\Saved\SaveGames`).

> ⚠️ **Disable Steam Cloud saves for Palworld** (Steam -> Properties -> General -> uncheck "Keep games saves in the Steam Cloud"), or Steam Cloud will overwrite the swapped save with the old one.

## Usage

Pick a world on the **Host Swap** page, then choose one:

**Cloud Sync (recommended)**: current host clicks **Upload Save** (local becomes guest-only, auto-backed up); the person taking over clicks **Download Latest** -> **Take Over as Host** -> launch the game.

**Manual Transfer**: host clicks **Export Save** and sends the file; the other person clicks **Import Save** -> automatically becomes the new host -> launch the game.

> A local backup is made before every operation. Roll back anytime from the **Backups** page.

## Features

- Auto-detects save directory, worlds/players, identifies host by UID
- One-click host swap (SteamID -> UID, cityhash64)
- Cloud sync (Qiniu Kodo): upload/download/version history/play lock
- Manual import/export: single-file relay, no cloud dependency
- **Auto-repairs legacy corrupted saves**: on download/import, rebuilds truncated guild ICH and consolidates scattered pals back onto the host slot (fixes the "can't lift base pal" bug)
- Local backup management with one-click restore
- Bilingual UI, full-chain logging (`%AppData%\PalSaveRelay\app.log`)

## How It Works

In co-op the save lives only on the host's machine. The host's UID is the sentinel `0000…0001`; guests have real UIDs derived from their SteamID.

A host swap moves only the **host player's identity** (player entry, guild membership, pal ownership, player file) from `0001` to their real UID; **world data** (all pals, buildings, etc.) stays on the `0001` host slot. The next host activates by swapping their real UID to `0001`, inheriting the entire world -- the resulting save matches an official host-world structure.

- **Upload/Export**: conversion runs on a temporary copy that is packaged; after upload the local save is stripped to guest-only (personal data only, auto-backed up, restorable); export only touches the temporary copy.
- **Download/Import**: overwrites the local world (after backup), auto-repairs legacy corruption, then activates as host (real UID -> `0001`).
- **LocalData.sav** holds personal quest/map progress and is not transferred with the world; each person keeps their own.

## Build From Source

Requires Go 1.25+, Node.js, [Wails v3 CLI](https://wails.io), mingw-w64 gcc (CGO).

```powershell
.\build.ps1            # frontend + icons + syso + go build
```

## Development

```bash
wails3 dev              # hot-reload
go test ./internal/...  # backend tests (incl. real save round-trip)
```

```
internal/
  sav/        Save engine (ported from cheahjs/palworld-save-tools)
  palworld/   Domain logic: detect, host swap, SteamID->UID, backup, packaging, corruption repair
  storage/    Cloud storage abstraction + Qiniu implementation
  config/     App config
  logger/     Process-level file logger
frontend/     React UI (bilingual i18n)
```

Test fixtures are real saves (covering both PlZ/zlib and PlM/Oodle), gitignored. `cd internal/sav/testdata; ./fetch.ps1` to fetch.

## Known Limitations

- After upload the local save is stripped to guest-only (auto-backed up first, restorable); export doesn't touch the local save; download/import/rollback/host-takeover all back up before modifying the local world.
- The host's real UID may already exist in OldOwnerPlayerUIds and other history fields, creating duplicate references -- this does not affect host-swapping.

## Acknowledgements

- [cheahjs/palworld-save-tools](https://github.com/cheahjs/palworld-save-tools) - upstream save format parsing (SAV/GVAS/property system).
- [deafdudecomputers/PalworldSaveTools](https://github.com/deafdudecomputers/PalworldSaveTools) - reference for host swapping & guild/character parsing.
- [new-world-tools/go-oodle](https://github.com/new-world-tools/go-oodle) - Go Oodle bindings.

## License

MIT
