# Pal Save Relay Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build a Windows single-exe Palworld co-op save-relay desktop tool (Wails v3 + Go + React) that swaps hosts by UID and syncs saves via Qiniu cloud storage.

**Architecture:** Pure-Go save engine (ported from `cheahjs/palworld-save-tools`) parses SAV/GVAS; Oodle compression via `go-oodle` (purego, no CGo) + embedded DLL; Qiniu object storage for peer-to-peer relay with no central server; Wails v3 binds Go methods to a React frontend.

**Tech Stack:** Go 1.22+, Wails v3 (pinned alpha), React 18 + TypeScript + Vite + shadcn/ui + Tailwind + Zustand, `github.com/qiniu/go-sdk/v7`, `github.com/new-world-tools/go-oodle`.

**Spec:** `docs/superpowers/specs/2026-07-11-palworld-save-relay-design.md`

---

## File Structure

```
palworld-save-relay/
  main.go                          # Wails app entry, service/tray registration (Phase 4)
  app.go                           # Wails bindings layer (Phase 4)
  go.mod
  internal/
    sav/                           # Save engine (Phase 1) - pure Go, testable standalone
      container.go                 # SAV container header + decompress/compress
      oodle.go                     # Oodle DLL embed/extract + go-oodle wrapper
      gvas.go                      # GVAS header + file read/write (Phase 1)
      archive.go                   # FArchiveReader/Writer primitives (Phase 1)
      properties.go                # UE property type read/write (Phase 1)
      rawdata.go                   # RawData dispatch + raw passthrough (Phase 1)
      rawdata_character.go         # CharacterSaveParameterMap parser (Phase 1)
      rawdata_group.go             # GroupSaveDataMap (guild) parser (Phase 1)
      paltypes.go                  # type hints / custom property registry (Phase 1)
      assets/oo2core_9_win64.dll   # embedded Oodle DLL
      testdata/*.sav               # real save fixtures
    palworld/                      # Domain logic (Phase 2)
      detect.go hostswap.go backup.go pack.go
    storage/                       # Cloud storage (Phase 3)
      storage.go qiniu.go lock.go
    config/config.go               # Config (Phase 4)
  frontend/                        # React app (Phase 5)
  resources/                       # app icon etc.
  docs/superpowers/{specs,plans}/
```

## Phase Roadmap

- **Phase 1 — Save engine (`internal/sav/`)**: SAV container, Oodle, GVAS, UE property system, RawData, gold round-trip tests. Independently testable with `go test`. *(current)*
- **Phase 2 — Palworld domain (`internal/palworld/`)**: detect worlds/players, host swap, backup, pack/unpack. Depends on Phase 1.
- **Phase 3 — Storage (`internal/storage/`)**: Storage interface, Qiniu impl, play lock. Independent of Phase 1/2.
- **Phase 4 — App shell**: config, Wails v3 scaffold, `app.go` bindings, game-running guard, Oodle DLL runtime extract.
- **Phase 5 — Frontend (React)**: all pages wired to bindings.
- **Phase 6 — Integration & packaging**: single-exe build, embed DLL, e2e relay flow, docs.

> Test fixtures come from this machine's real saves (both PlZ and PlM formats). Stable snapshots are read from game `backup/` folders to avoid file-lock issues while the game runs.

---

# Phase 1: Save Engine

## Task 1: Project init + test fixtures

**Files:**
- Create: `go.mod`
- Create: `internal/sav/testdata/level_plz.sav`
- Create: `internal/sav/testdata/player_plz.sav`
- Create: `internal/sav/testdata/level_plm.sav`
- Create: `internal/sav/testdata/player_plm.sav`
- Create: `internal/sav/testdata/localdata_plm.sav`

- [ ] **Step 1: Init Go module**

Run:
```bash
cd D:\Code\palworld-save-relay
go mod init palworld-save-relay
```
Expected: creates `go.mod` with `module palworld-save-relay`.

- [ ] **Step 2: Copy real save fixtures into testdata**

These are real saves from this machine. Copy PlZ (zlib) fixtures from the old single-player world and PlM (Oodle) fixtures from a stable backup snapshot of the multiplayer world:

```bash
mkdir internal\sav\testdata
copy "C:\Users\CZ\AppData\Local\Pal\Saved\SaveGames\76561198986886742\6E8DEA2A4D670B932592E49D63B013FC\Level.sav" internal\sav\testdata\level_plz.sav
copy "C:\Users\CZ\AppData\Local\Pal\Saved\SaveGames\76561198986886742\6E8DEA2A4D670B932592E49D63B013FC\Players\00000000000000000000000000000001.sav" internal\sav\testdata\player_plz.sav
copy "C:\Users\CZ\AppData\Local\Pal\Saved\SaveGames\76561198986886742\47CB2F8945C9CFC0FEF94F8FF89EDE0E\backup\world\2026.07.11-15.47.46\Level.sav" internal\sav\testdata\level_plm.sav
copy "C:\Users\CZ\AppData\Local\Pal\Saved\SaveGames\76561198986886742\47CB2F8945C9CFC0FEF94F8FF89EDE0E\backup\world\2026.07.11-15.47.46\Players\00000000000000000000000000000001.sav" internal\sav\testdata\player_plm.sav
copy "C:\Users\CZ\AppData\Local\Pal\Saved\SaveGames\76561198986886742\6424B6CA4FED14984B336CB98155AC7B\backup\local\2026.07.11-16.12.28\LocalData.sav" internal\sav\testdata\localdata_plm.sav
```
Expected: 5 files in `internal/sav/testdata/`. Verify formats:
```bash
go run ./internal/sav/cmd/peek@tmp 2>nul || powershell -c "Get-ChildItem internal\sav\testdata\*.sav | %%{ $b=[IO.File]::ReadAllBytes($_.FullName)[8..10]; '{0}: {1}' -f $_.Name ([Text.Encoding]::ASCII.GetString($b)) }"
```
Expected output shows `level_plz.sav: PlZ`, `player_plz.sav: PlZ`, `level_plm.sav: PlM`, `player_plm.sav: PlM`, `localdata_plm.sav: PlM`.

- [ ] **Step 3: Add a .gitignore for the DLL asset (commit fixtures)**

Append to `.gitignore`:
```
# temp peek helper
internal/sav/cmd/
```
Commit fixtures:
```bash
git add go.mod internal/sav/testdata/
git -c user.name=codex -c user.email=codex@local commit -m "chore(sav): init module and real save fixtures"
```

---

## Task 2: SAV container header parsing

**Files:**
- Create: `internal/sav/container.go`
- Create: `internal/sav/container_test.go`

- [ ] **Step 1: Write the failing test**

`internal/sav/container_test.go`:
```go
package sav_test

import (
	"os"
	"testing"

	"palworld-save-relay/internal/sav"
)

func TestParseSAVHeader_PlZ(t *testing.T) {
	data, err := os.ReadFile("testdata/level_plz.sav")
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	h, err := sav.ParseSAVHeader(data)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if string(h.Magic[:]) != "PlZ" {
		t.Errorf("magic = %q, want PlZ", h.Magic[:])
	}
	if h.SaveType != 50 {
		t.Errorf("saveType = %d, want 50", h.SaveType)
	}
	if h.UncompressedLen == 0 || h.CompressedLen == 0 {
		t.Errorf("lengths zero: uncomp=%d comp=%d", h.UncompressedLen, h.CompressedLen)
	}
	if h.DataOffset != 12 {
		t.Errorf("offset = %d, want 12", h.DataOffset)
	}
}

func TestParseSAVHeader_PlM(t *testing.T) {
	data, err := os.ReadFile("testdata/player_plm.sav")
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	h, err := sav.ParseSAVHeader(data)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if string(h.Magic[:]) != "PlM" {
		t.Errorf("magic = %q, want PlM", h.Magic[:])
	}
	if h.SaveType != 49 {
		t.Errorf("saveType = %d, want 49", h.SaveType)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/sav/ -run TestParseSAVHeader -v`
Expected: FAIL — `undefined: sav.ParseSAVHeader`.

- [ ] **Step 3: Write minimal implementation**

`internal/sav/container.go`:
```go
package sav

import (
	"bytes"
	"encoding/binary"
	"fmt"
)

// SaveType byte values (offset 11 of the SAV header).
const (
	SaveTypeCNK = 48 // 0x30
	SaveTypePLM = 49 // 0x31
	SaveTypePLZ = 50 // 0x32
)

var (
	magicPLZ = []byte("PlZ")
	magicPLM = []byte("PlM")
	magicCNK = []byte("CNK")
)

// SAVHeader is the parsed SAV container header.
type SAVHeader struct {
	UncompressedLen uint32
	CompressedLen   uint32
	Magic           [3]byte
	SaveType        byte
	DataOffset      int
}

// ParseSAVHeader parses the SAV container header. CNK has a double header.
func ParseSAVHeader(data []byte) (SAVHeader, error) {
	if len(data) < 12 {
		return SAVHeader{}, fmt.Errorf("file too small to parse header: %d bytes", len(data))
	}
	h := SAVHeader{
		UncompressedLen: binary.LittleEndian.Uint32(data[0:4]),
		CompressedLen:   binary.LittleEndian.Uint32(data[4:8]),
		DataOffset:      12,
	}
	copy(h.Magic[:], data[8:11])
	h.SaveType = data[11]

	if bytes.Equal(h.Magic[:], magicCNK) {
		if len(data) < 24 {
			return SAVHeader{}, fmt.Errorf("CNK file too small: %d bytes", len(data))
		}
		h.UncompressedLen = binary.LittleEndian.Uint32(data[12:16])
		h.CompressedLen = binary.LittleEndian.Uint32(data[16:20])
		copy(h.Magic[:], data[20:23])
		h.SaveType = data[23]
		h.DataOffset = 24
	}

	switch {
	case bytes.Equal(h.Magic[:], magicPLZ), bytes.Equal(h.Magic[:], magicPLM), bytes.Equal(h.Magic[:], magicCNK):
		return h, nil
	default:
		return SAVHeader{}, fmt.Errorf("unknown magic bytes: %q", string(h.Magic[:]))
	}
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/sav/ -run TestParseSAVHeader -v`
Expected: PASS (both subtests).

- [ ] **Step 5: Commit**

```bash
git add internal/sav/container.go internal/sav/container_test.go
git -c user.name=codex -c user.email=codex@local commit -m "feat(sav): SAV container header parsing"
```

---

## Task 3: Oodle integration (go-oodle, embedded DLL)

**Files:**
- Create: `internal/sav/assets/oo2core_9_win64.dll`
- Create: `internal/sav/oodle.go`
- Create: `internal/sav/oodle_test.go`
- Modify: `go.mod` (add `github.com/new-world-tools/go-oodle`)

- [ ] **Step 1: Obtain the Oodle DLL and place it for embedding**

Use go-oodle's built-in downloader (fetches `oo2core_9_win64.dll` from its GitHub release). Run this one-off program:
```bash
mkdir internal\sav\assets
cd internal\sav && go mod edit -require=github.com/new-world-tools/go-oodle@latest && cd ..\..
go run github.com/new-world-tools/go-oodle@latest -download 2>nul
```
If the downloader subcommand is unavailable, run this helper once:
```go
// tmp_dl/main.go
package main

import (
	"os"
	"path/filepath"
	oodle "github.com/new-world-tools/go-oodle"
)

func main() {
	if !oodle.IsLibExists() {
		if err := oodle.Download(); err != nil { panic(err) }
	}
	src := filepath.Join(os.TempDir(), "go-oodle", "oo2core_9_win64.dll")
	data, _ := os.ReadFile(src)
	os.MkdirAll("internal/sav/assets", 0o755)
	os.WriteFile("internal/sav/assets/oo2core_9_win64.dll", data, 0o644)
}
```
Run: `go run ./tmp_dl && rmdir /s /q tmp_dl`
Expected: `internal/sav/assets/oo2core_9_win64.dll` exists (~2-3 MB).

- [ ] **Step 2: Write the failing test**

`internal/sav/oodle_test.go`:
```go
package sav_test

import (
	"encoding/binary"
	"os"
	"testing"

	"palworld-save-relay/internal/sav"
)

func TestOodleDecompress_PlM(t *testing.T) {
	data, err := os.ReadFile("testdata/player_plm.sav")
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	h, err := sav.ParseSAVHeader(data)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	comp := data[h.DataOffset : h.DataOffset+int(h.CompressedLen)]
	out, err := sav.OodleDecompress(comp, int(h.UncompressedLen))
	if err != nil {
		t.Fatalf("oodle decompress: %v", err)
	}
	if len(out) != int(h.UncompressedLen) {
		t.Fatalf("len = %d, want %d", len(out), h.UncompressedLen)
	}
	// Decompressed payload must start with the GVAS magic (0x53415647, "GVAS" LE).
	if binary.LittleEndian.Uint32(out[0:4]) != 0x53415647 {
		t.Fatalf("not GVAS magic: %x", out[0:4])
	}
}
```

- [ ] **Step 3: Run test to verify it fails**

Run: `go test ./internal/sav/ -run TestOodleDecompress -v`
Expected: FAIL — `undefined: sav.OodleDecompress`.

- [ ] **Step 4: Write minimal implementation**

`internal/sav/oodle.go`:
```go
package sav

import (
	_ "embed"
	"os"
	"path/filepath"
	"sync"

	oodle "github.com/new-world-tools/go-oodle"
)

//go:embed assets/oo2core_9_win64.dll
var oodleDLL []byte

var (
	oodleOnce   sync.Once
	oodleInitErr error
)

// ensureOodle extracts the embedded DLL to the location go-oodle checks
// (os.TempDir()/go-oodle/oo2core_9_win64.dll) on first use.
func ensureOodle() error {
	oodleOnce.Do(func() {
		dir := filepath.Join(os.TempDir(), "go-oodle")
		path := filepath.Join(dir, "oo2core_9_win64.dll")
		if _, err := os.Stat(path); os.IsNotExist(err) {
			if err := os.MkdirAll(dir, 0o755); err != nil {
				oodleInitErr = err
				return
			}
			if err := os.WriteFile(path, oodleDLL, 0o644); err != nil {
				oodleInitErr = err
				return
			}
		}
	})
	return oodleInitErr
}

// OodleDecompress decompresses an Oodle buffer to exactly outLen bytes.
func OodleDecompress(comp []byte, outLen int) ([]byte, error) {
	if err := ensureOodle(); err != nil {
		return nil, err
	}
	return oodle.Decompress(comp, int64(outLen))
}

// OodleCompress compresses data with Oodle Kraken / Normal level.
func OodleCompress(data []byte) ([]byte, error) {
	if err := ensureOodle(); err != nil {
		return nil, err
	}
	return oodle.Compress(data, oodle.CompressorKraken, oodle.CompressionLevelNormal)
}
```

Run: `go mod tidy`
Expected: `go.mod` gains `github.com/new-world-tools/go-oodle` and `github.com/ebitengine/purego`.

- [ ] **Step 5: Run test to verify it passes**

Run: `go test ./internal/sav/ -run TestOodleDecompress -v`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add go.mod go.sum internal/sav/oodle.go internal/sav/oodle_test.go internal/sav/assets/oo2core_9_win64.dll
git -c user.name=codex -c user.email=codex@local commit -m "feat(sav): Oodle decompress/compress via go-oodle + embedded DLL"
```

---

## Task 4: SAV decompress + compress (PlZ and PlM round-trip)

**Files:**
- Modify: `internal/sav/container.go`
- Modify: `internal/sav/container_test.go`

- [ ] **Step 1: Write the failing tests**

Append to `internal/sav/container_test.go`:
```go
import (
	"bytes"
	// ...existing imports...
)

func TestDecompressCompressRoundTrip_PlZ(t *testing.T) {
	for _, name := range []string{"testdata/level_plz.sav", "testdata/player_plz.sav"} {
		t.Run(name, func(t *testing.T) {
			data, err := os.ReadFile(name)
			if err != nil {
				t.Fatalf("read: %v", err)
			}
			gvas, h, err := sav.Decompress(data)
			if err != nil {
				t.Fatalf("decompress: %v", err)
			}
			sav2, err := sav.Compress(gvas, h)
			if err != nil {
				t.Fatalf("compress: %v", err)
			}
			gvas2, _, err := sav.Decompress(sav2)
			if err != nil {
				t.Fatalf("re-decompress: %v", err)
			}
			if !bytes.Equal(gvas, gvas2) {
				t.Fatal("GVAS bytes differ after compress round-trip")
			}
		})
	}
}

func TestDecompressCompressRoundTrip_PlM(t *testing.T) {
	for _, name := range []string{"testdata/player_plm.sav", "testdata/localdata_plm.sav", "testdata/level_plm.sav"} {
		t.Run(name, func(t *testing.T) {
			data, err := os.ReadFile(name)
			if err != nil {
				t.Fatalf("read: %v", err)
			}
			gvas, h, err := sav.Decompress(data)
			if err != nil {
				t.Fatalf("decompress: %v", err)
			}
			sav2, err := sav.Compress(gvas, h)
			if err != nil {
				t.Fatalf("compress: %v", err)
			}
			gvas2, _, err := sav.Decompress(sav2)
			if err != nil {
				t.Fatalf("re-decompress: %v", err)
			}
			if !bytes.Equal(gvas, gvas2) {
				t.Fatal("GVAS bytes differ after compress round-trip")
			}
		})
	}
}
```
(Merge the `bytes` import into the existing import block.)

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/sav/ -run TestDecompressCompressRoundTrip -v`
Expected: FAIL — `undefined: sav.Decompress` / `sav.Compress`.

- [ ] **Step 3: Write minimal implementation**

Append to `internal/sav/container.go` (merge imports: add `compress/zlib`, `io`):
```go
// Decompress decompresses a SAV file to GVAS bytes, returning the bytes and the
// original header (so the caller preserves format on recompression).
func Decompress(data []byte) ([]byte, SAVHeader, error) {
	h, err := ParseSAVHeader(data)
	if err != nil {
		return nil, SAVHeader{}, err
	}
	payload := data[h.DataOffset:]
	switch {
	case bytes.Equal(h.Magic[:], magicPLZ):
		out, err := zlibDecompress(payload)
		if err != nil {
			return nil, h, err
		}
		if h.SaveType == SaveTypePLZ {
			// double zlib
			out, err = zlibDecompress(out)
			if err != nil {
				return nil, h, err
			}
		}
		if len(out) != int(h.UncompressedLen) {
			return nil, h, fmt.Errorf("uncompressed len mismatch: got %d want %d", len(out), h.UncompressedLen)
		}
		return out, h, nil
	case bytes.Equal(h.Magic[:], magicPLM):
		comp := payload[:h.CompressedLen]
		out, err := OodleDecompress(comp, int(h.UncompressedLen))
		if err != nil {
			return nil, h, fmt.Errorf("oodle decompress: %w", err)
		}
		if len(out) != int(h.UncompressedLen) {
			return nil, h, fmt.Errorf("uncompressed len mismatch: got %d want %d", len(out), h.UncompressedLen)
		}
		return out, h, nil
	case bytes.Equal(h.Magic[:], magicCNK):
		return nil, h, fmt.Errorf("CNK inner decompress not yet supported")
	default:
		return nil, h, fmt.Errorf("unsupported magic: %q", string(h.Magic[:]))
	}
}

// Compress compresses GVAS bytes back into a SAV container, preserving the
// original format recorded in h (magic + save type).
func Compress(gvas []byte, h SAVHeader) ([]byte, error) {
	var comp []byte
	var err error
	switch {
	case bytes.Equal(h.Magic[:], magicPLZ):
		if h.SaveType == SaveTypePLZ {
			comp = zlibCompress(zlibCompress(gvas))
		} else {
			comp = zlibCompress(gvas)
		}
	case bytes.Equal(h.Magic[:], magicPLM):
		comp, err = OodleCompress(gvas)
		if err != nil {
			return nil, fmt.Errorf("oodle compress: %w", err)
		}
	default:
		return nil, fmt.Errorf("unsupported magic for compress: %q", string(h.Magic[:]))
	}
	return buildSAV(comp, uint32(len(gvas)), uint32(len(comp)), h.Magic[:], h.SaveType), nil
}

func buildSAV(comp []byte, uncompLen, compLen uint32, magic []byte, saveType byte) []byte {
	out := make([]byte, 12+len(comp))
	binary.LittleEndian.PutUint32(out[0:4], uncompLen)
	binary.LittleEndian.PutUint32(out[4:8], compLen)
	copy(out[8:11], magic)
	out[11] = saveType
	copy(out[12:], comp)
	return out
}

func zlibDecompress(data []byte) ([]byte, error) {
	r, err := zlib.NewReader(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	defer r.Close()
	return io.ReadAll(r)
}

func zlibCompress(data []byte) []byte {
	var buf bytes.Buffer
	w := zlib.NewWriter(&buf)
	_, _ = w.Write(data)
	_ = w.Close()
	return buf.Bytes()
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/sav/ -v`
Expected: PASS for header, oodle, and both round-trip tests.

- [ ] **Step 5: Commit**

```bash
git add internal/sav/container.go internal/sav/container_test.go
git -c user.name=codex -c user.email=codex@local commit -m "feat(sav): SAV decompress/compress with PlZ+PlM round-trip"
```

---

<!-- Phase 1 continues: Task 5 (GVAS header + FArchive primitives), Task 6 (UE property system), Task 7 (RawData parsers), Task 8 (gold GVAS round-trip test). Written in next increment. -->
<!-- Phase 2: palworld/detect.go, hostswap.go, backup.go, pack.go -->
<!-- Phase 3: storage/ (qiniu, lock) -->
<!-- Phase 4: config + Wails v3 scaffold + app.go bindings -->
<!-- Phase 5: React frontend -->
<!-- Phase 6: integration + single-exe packaging -->
