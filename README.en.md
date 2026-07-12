# Palworld Save Relay

[中文](README.md) | **English**

A desktop tool for **swapping the host** of a Palworld co-op save. Lets multiple players take turns being the host while sharing the same world progress — via cloud sync or manual import/export. **Single-file exe, zero install.**

> In co-op mode the save lives only on the host's machine; when the host goes offline nobody else can continue. This tool swaps the host by converting the host UID and relaying the save through the cloud or a file.

## How host-swapping works

Inside the save, the host's own UID is the sentinel `00000000-0000-0000-0000-000000000001`; everyone else (guests) has a real UID derived from their SteamID (`cityhash64`).

- **Intermediate state**: replace the host sentinel `0000…0001` with the host's real UID, yielding "everyone is a real UID, nobody is the host." This is the transfer format.
- **Upload / Export**: the conversion is done on a **temporary copy** that is then packaged — **your local save is untouched** (you remain host after uploading).
- **Download / Import**: overwrite your local save with the intermediate state (after backing up), then swap your real UID to `0000…0001`, making you the new host.

The core operation ConvertHost(fromUID -> toUID) is a **one-way global UID replacement** (not a two-way swap), combined with the intermediate state to achieve host-swapping. Each person's save data always stays under their own real UID and is never lost across transfers.

> LocalData.sav holds personal quest/map progress and is local — it is **not transferred** with the world (neither upload nor download includes it); each person keeps their own. Local backups do include it for a complete rollback.

## Features

- Auto-detects the local host save (a complete world with Level.sav), lists worlds/players, identifies the host by UID; guest saves (LocalData.sav only) are ignored
- One-click host swap (SteamID -> UID via cityhash64), with automatic pre-operation backup and rollback
- Cloud sync: upload / download / version history via Qiniu Kodo
- Manual import/export: works without cloud — export to a single file, send it, the recipient imports and becomes the host
- Local backup management (auto-backup before every host swap / download / import)
- Full-chain logging (`%APPDATA%\PalSaveRelay\app.log`) — startup / config / detection / host-swap / cloud / backup / import-export are all recorded
- Single Windows exe (Oodle DLL embedded)
- Bilingual UI (中文 / English) with a one-click language toggle

## Tech stack

Go 1.25 · Wails v3 (alpha2) · React 18 + TypeScript + Vite + Tailwind · Qiniu Kodo · go-oodle (Oodle, CGO calling oo2core DLL, requires mingw gcc)

## Project structure

```
internal/
  sav/        Save engine (ported from cheahjs/palworld-save-tools: SAV container / Oodle / GVAS / properties / RawData)
  palworld/   Domain logic: detect, ConvertHost, PackIntermediate, SteamID->UID, backup, packaging
  storage/    Cloud storage abstraction + Qiniu implementation
  config/     App config
  logger/     Process-level file logger
main.go app.go   Wails v3 entry & bindings
frontend/        React UI (bilingual: src/i18n.tsx)
docs/superpowers/{specs,plans}/   Design docs & implementation plans
```

## Build

Requires Go 1.25+, Node.js, [Wails v3 CLI](https://wails.io), mingw-w64 gcc (go-oodle depends on CGO):

```bash
wails3 task build          # production build (npm build + go build), outputs bin/palworld-save-relay.exe
# or manually:
cd frontend && npm install && npm run build && cd ..
go build -o bin/palworld-save-relay.exe .
```

> Note: `@wailsio/runtime` in frontend/package.json tracks the Wails version — don't pin an old version manually, or the front/back-end protocol will mismatch and report "Invalid runtime call".

## Development

```bash
wails3 dev                 # hot-reload dev
go test ./internal/...     # backend tests (incl. real save round-trip)
```

Test fixtures are real Palworld saves (covering both PlZ/zlib and PlM/Oodle), gitignored for privacy. To fetch them on first run:

```powershell
cd internal/sav/testdata; ./fetch.ps1
```

## Usage

1. (Optional) Configure cloud service in Settings (Qiniu AccessKey/SecretKey/Bucket; region is auto-detected, download domain is auto-fetched if left blank). Import/export works without it.
2. Under Host Swap, select the world whose host you want to swap.
3. **Cloud sync**: the current host clicks Upload Save (local save stays, you remain host); the person taking over clicks Download Latest, then Take Over as Host, and launches the game.
4. **Manual**: the host clicks Export Save to save a single file and sends it; the recipient clicks Import Save, picks that file, and automatically becomes the new host.

## Known limitations / Notes

- The host's real UID may already exist in the save (e.g. OldOwnerPlayerUIds). A single "handoff" can create duplicate references — **this does not affect host-swapping** (one-way flow; a few extra references in the intermediate state are harmless).
- Guild `individual_character_handle_ids` guids are not currently included in the replacement; patch if guild-member ownership issues are observed.
- Upload/export only touch a temporary copy and never modify your local save; download/import overwrite the local world (after backup); Take Over as Host modifies the local save (after backup).
- Tray icon / auto-detect game exit / auto-upload are planned for later.

## Acknowledgements

- [cheahjs/palworld-save-tools](https://github.com/cheahjs/palworld-save-tools) — upstream save format parsing (SAV/GVAS/property system).
- [deafdudecomputers/PalworldSaveTools](https://github.com/deafdudecomputers/PalworldSaveTools) — reference for host swapping & guild/character parsing.
- [new-world-tools/go-oodle](https://github.com/new-world-tools/go-oodle) — Go Oodle bindings.

## License

MIT