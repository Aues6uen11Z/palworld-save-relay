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


## Task 5: FArchive primitives, UUID, GVAS header, ordered PropertyList

**Files:**
- Create: `internal/sav/archive.go`
- Create: `internal/sav/gvas.go`
- Create: `internal/sav/gvas_test.go`

> Why ordered: Go `map` is unordered, but UE properties are an ordered sequence. Byte-identical round-trip (the correctness proof) requires preserving order, so property sequences use `PropertyList` (an ordered slice). Struct/array/map value dicts have fixed write order, so they stay `map[string]any`.
> Why raw UUID: Palworld GUIDs are 16 raw bytes. Host swap compares by raw bytes (format-agnostic); the mixed-endian `String()` is display-only.

- [ ] **Step 1: Write the failing tests**

`internal/sav/gvas_test.go`:
```go
package sav_test

import (
	"bytes"
	"os"
	"testing"

	"palworld-save-relay/internal/sav"
)

func TestFArchivePrimitives_RoundTrip(t *testing.T) {
	w := sav.NewFArchiveWriter(nil)
	w.FString("hello")
	w.FString("你好") // utf-16
	w.I32(-12345)
	w.U64(0x0102030405060708)
	g := sav.UUID{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16}
	w.Guid(&g)
	w.OptionalGuid(nil)
	w.OptionalGuid(&g)

	r := sav.NewFArchiveReader(w.Bytes(), nil, nil)
	if v := r.FString(); v != "hello" {
		t.Errorf("ascii fstring = %q", v)
	}
	if v := r.FString(); v != "你好" {
		t.Errorf("utf16 fstring = %q", v)
	}
	if v := r.I32(); v != -12345 {
		t.Errorf("i32 = %d", v)
	}
	if v := r.U64(); v != 0x0102030405060708 {
		t.Errorf("u64 = %x", v)
	}
	if v := r.Guid(); *v != g {
		t.Errorf("guid = %v", v)
	}
	if r.OptionalGuid() != nil {
		t.Errorf("optional guid nil expected")
	}
	if v := r.OptionalGuid(); *v != g {
		t.Errorf("optional guid = %v", v)
	}
}

func TestGvasHeader_RoundTrip(t *testing.T) {
	data, err := os.ReadFile("testdata/player_plz.sav")
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	gvas, _, err := sav.Decompress(data)
	if err != nil {
		t.Fatalf("decompress: %v", err)
	}
	r := sav.NewFArchiveReader(gvas, nil, nil)
	hdr, err := sav.ReadGvasHeader(r)
	if err != nil {
		t.Fatalf("read header: %v", err)
	}
	headerEnd := r.Pos()

	w := sav.NewFArchiveWriter(nil)
	sav.WriteGvasHeader(w, hdr)
	if !bytes.Equal(w.Bytes(), gvas[:headerEnd]) {
		t.Fatalf("header bytes differ (got %d bytes, want %d)", len(w.Bytes()), headerEnd)
	}
	if hdr.Magic != 0x53415647 {
		t.Errorf("magic = %x, want GVAS", hdr.Magic)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/sav/ -run "TestFArchivePrimitives|TestGvasHeader" -v`
Expected: FAIL - `undefined: sav.NewFArchiveWriter` etc.

- [ ] **Step 3: Write minimal implementation**

`internal/sav/archive.go`:
```go
package sav

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"math"
	"unicode/utf16"
)

// UUID is a Palworld GUID: 16 raw bytes. Comparisons and host-swap use raw
// bytes; String() uses Palworld's mixed-endian layout for display only.
type UUID [16]byte

func (u *UUID) Equal(o *UUID) bool {
	if u == nil || o == nil {
		return u == o
	}
	return *u == *o
}

func (u UUID) String() string {
	b := u[:]
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%04x%08x",
		uint32(b[3])<<24|uint32(b[2])<<16|uint32(b[1])<<8|uint32(b[0]),
		uint32(b[7])<<8|uint32(b[6]),
		uint32(b[5])<<8|uint32(b[4]),
		uint32(b[11])<<8|uint32(b[10]),
		uint32(b[9])<<8|uint32(b[8]),
		uint32(b[15])<<24|uint32(b[14])<<16|uint32(b[13])<<8|uint32(b[12]),
	)
}

// PropertyEntry is one named property in an ordered property sequence.
type PropertyEntry struct {
	Name  string
	Value map[string]any
}

// PropertyList is an ordered list of named properties (preserves read order).
type PropertyList []PropertyEntry

func (pl PropertyList) Get(name string) map[string]any {
	for _, e := range pl {
		if e.Name == name {
			return e.Value
		}
	}
	return nil
}

// CustomProperty is a (decode, encode) pair registered for a property path.
type CustomProperty struct {
	Decode func(r *FArchiveReader, typeName string, size int, path string) map[string]any
	Encode func(w *FArchiveWriter, propertyType string, p map[string]any) int
}

// ---- Reader ----

type FArchiveReader struct {
	data             []byte
	pos              int
	typeHints        map[string]string
	customProperties map[string]CustomProperty
}

func NewFArchiveReader(data []byte, typeHints map[string]string, custom map[string]CustomProperty) *FArchiveReader {
	return &FArchiveReader{data: data, typeHints: typeHints, customProperties: custom}
}

func (r *FArchiveReader) Pos() int      { return r.pos }
func (r *FArchiveReader) EOF() bool     { return r.pos >= len(r.data) }
func (r *FArchiveReader) Read(n int) []byte {
	if r.pos+n > len(r.data) {
		panic(fmt.Sprintf("read past end: pos=%d n=%d len=%d", r.pos, n, len(r.data)))
	}
	b := r.data[r.pos : r.pos+n]
	r.pos += n
	return b
}
func (r *FArchiveReader) ReadToEnd() []byte {
	b := r.data[r.pos:]
	r.pos = len(r.data)
	return b
}
func (r *FArchiveReader) Skip(n int) { r.pos += n }

func (r *FArchiveReader) Byte() byte      { return r.Read(1)[0] }
func (r *FArchiveReader) Bool() bool      { return r.Byte() > 0 }
func (r *FArchiveReader) I16() int16      { return int16(binary.LittleEndian.Uint16(r.Read(2))) }
func (r *FArchiveReader) U16() uint16     { return binary.LittleEndian.Uint16(r.Read(2)) }
func (r *FArchiveReader) I32() int32      { return int32(binary.LittleEndian.Uint32(r.Read(4))) }
func (r *FArchiveReader) U32() uint32     { return binary.LittleEndian.Uint32(r.Read(4)) }
func (r *FArchiveReader) I64() int64      { return int64(binary.LittleEndian.Uint64(r.Read(8))) }
func (r *FArchiveReader) U64() uint64     { return binary.LittleEndian.Uint64(r.Read(8)) }
func (r *FArchiveReader) Float() float32  { return math.Float32frombits(binary.LittleEndian.Uint32(r.Read(4))) }
func (r *FArchiveReader) Double() float64 { return math.Float64frombits(binary.LittleEndian.Uint64(r.Read(8))) }

func (r *FArchiveReader) Guid() *UUID {
	var u UUID
	copy(u[:], r.Read(16))
	return &u
}
func (r *FArchiveReader) OptionalGuid() *UUID {
	if r.Byte() != 0 {
		return r.Guid()
	}
	return nil
}

func (r *FArchiveReader) FString() string {
	size := int(r.I32())
	if size == 0 {
		return ""
	}
	if size < 0 {
		size = -size
		b := r.Read(size * 2)
		return decodeUTF16LE(b[:len(b)-2]) // drop 2-byte null terminator
	}
	b := r.Read(size)
	return string(b[:len(b)-1]) // drop 1-byte null terminator (ascii)
}

func (r *FArchiveReader) TArray(readElem func() any) []any {
	count := int(r.U32())
	arr := make([]any, 0, count)
	for i := 0; i < count; i++ {
		arr = append(arr, readElem())
	}
	return arr
}

func (r *FArchiveReader) ByteList(n int) []byte {
	return r.Read(n)
}

func (r *FArchiveReader) getTypeOr(path, def string) string {
	if t, ok := r.typeHints[path]; ok {
		return t
	}
	return def
}

// InternalCopy returns a new reader over a sub-buffer sharing config.
func (r *FArchiveReader) InternalCopy(data []byte) *FArchiveReader {
	return &FArchiveReader{data: data, typeHints: r.typeHints, customProperties: r.customProperties}
}

func decodeUTF16LE(b []byte) string {
	u := make([]uint16, len(b)/2)
	for i := range u {
		u[i] = binary.LittleEndian.Uint16(b[i*2:])
	}
	return string(utf16.Decode(u))
}

func isASCII(s string) bool {
	for _, c := range s {
		if c > 127 {
			return false
		}
	}
	return true
}

// ---- Writer ----

type FArchiveWriter struct {
	buf              *bytes.Buffer
	customProperties map[string]CustomProperty
}

func NewFArchiveWriter(custom map[string]CustomProperty) *FArchiveWriter {
	return &FArchiveWriter{buf: new(bytes.Buffer), customProperties: custom}
}

func (w *FArchiveWriter) Bytes() []byte        { return w.buf.Bytes() }
func (w *FArchiveWriter) WriteByteRaw(b byte)  { w.buf.WriteByte(b) }
func (w *FArchiveWriter) Write(b []byte)       { w.buf.Write(b) }

func (w *FArchiveWriter) Byte(b byte) {
	w.buf.WriteByte(b)
}
func (w *FArchiveWriter) Bool(b bool) {
	if b {
		w.buf.WriteByte(1)
	} else {
		w.buf.WriteByte(0)
	}
}
func (w *FArchiveWriter) I16(v int16)  { var b [2]byte; binary.LittleEndian.PutUint16(b[:], uint16(v)); w.buf.Write(b[:]) }
func (w *FArchiveWriter) U16(v uint16) { var b [2]byte; binary.LittleEndian.PutUint16(b[:], v); w.buf.Write(b[:]) }
func (w *FArchiveWriter) I32(v int32)  { var b [4]byte; binary.LittleEndian.PutUint32(b[:], uint32(v)); w.buf.Write(b[:]) }
func (w *FArchiveWriter) U32(v uint32) { var b [4]byte; binary.LittleEndian.PutUint32(b[:], v); w.buf.Write(b[:]) }
func (w *FArchiveWriter) I64(v int64)  { var b [8]byte; binary.LittleEndian.PutUint64(b[:], uint64(v)); w.buf.Write(b[:]) }
func (w *FArchiveWriter) U64(v uint64) { var b [8]byte; binary.LittleEndian.PutUint64(b[:], v); w.buf.Write(b[:]) }
func (w *FArchiveWriter) Float(v float32)  { var b [4]byte; binary.LittleEndian.PutUint32(b[:], math.Float32bits(v)); w.buf.Write(b[:]) }
func (w *FArchiveWriter) Double(v float64) { var b [8]byte; binary.LittleEndian.PutUint64(b[:], math.Float64bits(v)); w.buf.Write(b[:]) }

func (w *FArchiveWriter) Guid(u *UUID) {
	if u == nil {
		var z UUID
		w.buf.Write(z[:])
	} else {
		w.buf.Write(u[:])
	}
}
func (w *FArchiveWriter) OptionalGuid(u *UUID) {
	if u == nil {
		w.Bool(false)
	} else {
		w.Bool(true)
		w.Guid(u)
	}
}

func (w *FArchiveWriter) FString(s string) {
	if s == "" {
		w.I32(0)
		return
	}
	if isASCII(s) {
		b := []byte(s)
		w.I32(int32(len(b) + 1))
		w.buf.Write(b)
		w.buf.WriteByte(0)
		return
	}
	b := encodeUTF16LE(s)
	w.I32(int32(-(len(b)/2 + 1)))
	w.buf.Write(b)
	w.buf.WriteByte(0)
	w.buf.WriteByte(0)
}

func (w *FArchiveWriter) TArray(writeElem func(v any), arr []any) {
	w.U32(uint32(len(arr)))
	for _, v := range arr {
		writeElem(v)
	}
}

func encodeUTF16LE(s string) []byte {
	u := utf16.Encode([]rune(s))
	b := make([]byte, len(u)*2)
	for i, v := range u {
		binary.LittleEndian.PutUint16(b[i*2:], v)
	}
	return b
}
```

`internal/sav/gvas.go`:
```go
package sav

const GvasMagic uint32 = 0x53415647 // "GVAS" little-endian

type CustomVersion struct {
	ID      *UUID
	Version int32
}

type GvasHeader struct {
	Magic                   uint32
	SaveGameVersion         int32
	PackageFileVersionUE4   int32
	PackageFileVersionUE5   int32
	EngineVersionMajor      uint16
	EngineVersionMinor      uint16
	EngineVersionPatch      uint16
	EngineVersionChangelist uint32
	EngineVersionBranch     string
	CustomVersionFormat     int32
	CustomVersions          []CustomVersion
	SaveGameClassName       string
}

type GvasFile struct {
	Header     GvasHeader
	Properties PropertyList
	Trailer    []byte
}

func ReadGvasHeader(r *FArchiveReader) (GvasHeader, error) {
	h := GvasHeader{}
	h.Magic = r.U32()
	if h.Magic != GvasMagic {
		return h, fmt.Errorf("invalid GVAS magic: %x", h.Magic)
	}
	h.SaveGameVersion = r.I32()
	h.PackageFileVersionUE4 = r.I32()
	h.PackageFileVersionUE5 = r.I32()
	h.EngineVersionMajor = r.U16()
	h.EngineVersionMinor = r.U16()
	h.EngineVersionPatch = r.U16()
	h.EngineVersionChangelist = r.U32()
	h.EngineVersionBranch = r.FString()
	h.CustomVersionFormat = r.I32()
	count := int(r.U32())
	h.CustomVersions = make([]CustomVersion, count)
	for i := 0; i < count; i++ {
		h.CustomVersions[i] = CustomVersion{ID: r.Guid(), Version: r.I32()}
	}
	h.SaveGameClassName = r.FString()
	return h, nil
}

func WriteGvasHeader(w *FArchiveWriter, h GvasHeader) {
	w.U32(h.Magic)
	w.I32(h.SaveGameVersion)
	w.I32(h.PackageFileVersionUE4)
	w.I32(h.PackageFileVersionUE5)
	w.U16(h.EngineVersionMajor)
	w.U16(h.EngineVersionMinor)
	w.U16(h.EngineVersionPatch)
	w.U32(h.EngineVersionChangelist)
	w.FString(h.EngineVersionBranch)
	w.I32(h.CustomVersionFormat)
	w.U32(uint32(len(h.CustomVersions)))
	for _, cv := range h.CustomVersions {
		w.Guid(cv.ID)
		w.I32(cv.Version)
	}
	w.FString(h.SaveGameClassName)
}
```
(Add `"fmt"` to gvas.go imports.)

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/sav/ -run "TestFArchivePrimitives|TestGvasHeader" -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/sav/archive.go internal/sav/gvas.go internal/sav/gvas_test.go
git -c user.name=codex -c user.email=codex@local commit -m "feat(sav): FArchive primitives, UUID, GVAS header, ordered PropertyList"
```

---

## Task 6: UE property system (read + write dispatch)

**Files:**
- Create: `internal/sav/properties.go`
- Create: `internal/sav/properties_test.go`
- Modify: `internal/sav/gvas.go` (add `ReadGvasFile` / `WriteGvasFile`)

> Ports `FArchiveReader.property/properties_until_end/struct/array_property/map_property` and the writer counterparts from `archive.py`. Properties are read into `PropertyList` (ordered) + `map[string]any` values. Custom-property dispatch (rawdata) is wired but the registry is empty until Task 7; this task's test uses synthetic data with no rawdata.

- [ ] **Step 1: Write the failing test**

`internal/sav/properties_test.go`:
```go
package sav_test

import (
	"bytes"
	"testing"

	"palworld-save-relay/internal/sav"
)

func TestPropertyRoundTrip_Synthetic(t *testing.T) {
	g := sav.UUID{0xA, 0xB, 0xC, 0xD, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12}
	original := sav.PropertyList{
		{Name: "IntField", Value: map[string]any{"type": "IntProperty", "id": (*sav.UUID)(nil), "value": int32(42)}},
		{Name: "StrField", Value: map[string]any{"type": "StrProperty", "id": (*sav.UUID)(nil), "value": "player"}},
		{Name: "BoolField", Value: map[string]any{"type": "BoolProperty", "value": true, "id": (*sav.UUID)(nil)}},
		{Name: "GuidField", Value: map[string]any{
			"type": "StructProperty",
			"struct_type": "Guid", "struct_id": &sav.UUID{}, "id": (*sav.UUID)(nil),
			"value": &g,
		}},
	}

	w := sav.NewFArchiveWriter(nil)
	w.Properties(original)
	encoded := w.Bytes()

	r := sav.NewFArchiveReader(encoded, nil, nil)
	parsed := r.PropertiesUntilEnd("")
	if !r.EOF() {
		t.Fatalf("trailing bytes: %d", len(r.ReadToEnd()))
	}

	w2 := sav.NewFArchiveWriter(nil)
	w2.Properties(parsed)
	if !bytes.Equal(encoded, w2.Bytes()) {
		t.Fatalf("property round-trip bytes differ")
	}
	// Spot-check a value.
	if parsed.Get("IntField")["value"].(int32) != int32(42) {
		t.Errorf("IntField value lost")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/sav/ -run TestPropertyRoundTrip_Synthetic -v`
Expected: FAIL - `undefined: sav.NewFArchiveWriter.Properties` / `PropertiesUntilEnd`.

- [ ] **Step 3: Write minimal implementation**

`internal/sav/properties.go`:
```go
package sav

// ---- Reader property dispatch (ports archive.py FArchiveReader) ----

func (r *FArchiveReader) PropertiesUntilEnd(path string) PropertyList {
	var pl PropertyList
	for {
		name := r.FString()
		if name == "None" {
			break
		}
		typeName := r.FString()
		size := int(r.U64())
		pl = append(pl, PropertyEntry{Name: name, Value: r.Property(typeName, size, path+"."+name)})
	}
	return pl
}

func (r *FArchiveReader) Property(typeName string, size int, path string) map[string]any {
	var value map[string]any
	if cp, ok := r.customProperties[path]; ok {
		value = cp.Decode(r, typeName, size, path)
		value["custom_type"] = path
	} else {
		value = r.readPropertyByType(typeName, size, path)
	}
	value["type"] = typeName
	return value
}

func (r *FArchiveReader) readPropertyByType(typeName string, size int, path string) map[string]any {
	switch typeName {
	case "StructProperty":
		return r.readStruct(path)
	case "IntProperty":
		return map[string]any{"id": r.OptionalGuid(), "value": r.I32()}
	case "UInt16Property":
		return map[string]any{"id": r.OptionalGuid(), "value": r.U16()}
	case "UInt32Property":
		return map[string]any{"id": r.OptionalGuid(), "value": r.U32()}
	case "UInt64Property":
		return map[string]any{"id": r.OptionalGuid(), "value": r.U64()}
	case "Int64Property":
		return map[string]any{"id": r.OptionalGuid(), "value": r.I64()}
	case "FixedPoint64Property":
		return map[string]any{"id": r.OptionalGuid(), "value": r.I32()}
	case "FloatProperty":
		return map[string]any{"id": r.OptionalGuid(), "value": r.Float()}
	case "StrProperty":
		return map[string]any{"id": r.OptionalGuid(), "value": r.FString()}
	case "NameProperty":
		return map[string]any{"id": r.OptionalGuid(), "value": r.FString()}
	case "EnumProperty":
		enumType := r.FString()
		id := r.OptionalGuid()
		enumValue := r.FString()
		return map[string]any{"id": id, "value": map[string]any{"type": enumType, "value": enumValue}}
	case "BoolProperty":
		return map[string]any{"value": r.Bool(), "id": r.OptionalGuid()}
	case "ByteProperty":
		enumType := r.FString()
		id := r.OptionalGuid()
		var ev any
		if enumType == "None" {
			ev = r.Byte()
		} else {
			ev = r.FString()
		}
		return map[string]any{"id": id, "value": map[string]any{"type": enumType, "value": ev}}
	case "ArrayProperty":
		arrayType := r.FString()
		id := r.OptionalGuid()
		return map[string]any{"array_type": arrayType, "id": id, "value": r.arrayProperty(arrayType, size, path)}
	case "MapProperty":
		return r.readMapProperty(size, path)
	case "SetProperty":
		return r.readSetProperty(size, path)
	default:
		panic("unknown property type: " + typeName + " (" + path + ")")
	}
}

func (r *FArchiveReader) readStruct(path string) map[string]any {
	structType := r.FString()
	structID := r.Guid()
	id := r.OptionalGuid()
	value := r.StructValue(structType, path)
	return map[string]any{"struct_type": structType, "struct_id": structID, "id": id, "value": value}
}

func (r *FArchiveReader) StructValue(structType, path string) any {
	switch structType {
	case "Vector":
		return map[string]any{"x": r.Double(), "y": r.Double(), "z": r.Double()}
	case "DateTime":
		return r.U64()
	case "Guid":
		return r.Guid()
	case "Quat":
		return map[string]any{"x": r.Double(), "y": r.Double(), "z": r.Double(), "w": r.Double()}
	case "LinearColor":
		return map[string]any{"r": r.Float(), "g": r.Float(), "b": r.Float(), "a": r.Float()}
	case "Color":
		return map[string]any{"b": r.Byte(), "g": r.Byte(), "r": r.Byte(), "a": r.Byte()}
	default:
		return r.PropertiesUntilEnd(path)
	}
}

func (r *FArchiveReader) arrayProperty(arrayType string, size int, path string) map[string]any {
	count := r.U32()
	if arrayType == "StructProperty" {
		propName := r.FString()
		propType := r.FString()
		r.U64() // reserved size slot
		typeName := r.FString()
		id := r.Guid()
		r.Byte() // skip 1
		values := make([]any, 0, count)
		for i := uint32(0); i < count; i++ {
			values = append(values, r.StructValue(typeName, path+"."+propName))
		}
		return map[string]any{"prop_name": propName, "prop_type": propType, "values": values, "type_name": typeName, "id": id}
	}
	return map[string]any{"values": r.arrayValue(arrayType, int(count), size-4, path)}
}

func (r *FArchiveReader) arrayValue(arrayType string, count, size int, path string) any {
	switch arrayType {
	case "EnumProperty", "NameProperty":
		out := make([]any, 0, count)
		for i := 0; i < count; i++ {
			out = append(out, r.FString())
		}
		return out
	case "Guid":
		out := make([]any, 0, count)
		for i := 0; i < count; i++ {
			out = append(out, r.Guid())
		}
		return out
	case "ByteProperty":
		if size == count {
			return r.ByteList(count) // []byte
		}
		panic("labelled ByteProperty array not implemented: " + path)
	default:
		panic("unknown array type: " + arrayType + " (" + path + ")")
	}
}

func (r *FArchiveReader) readMapProperty(size int, path string) map[string]any {
	keyType := r.FString()
	valueType := r.FString()
	id := r.OptionalGuid()
	start := r.pos
	r.U32() // reserved 0
	count := r.U32()
	keyStructType := ""
	valueStructType := ""
	if keyType == "StructProperty" {
		keyStructType = r.getTypeOr(path+".Key", "Guid")
	}
	if valueType == "StructProperty" {
		valueStructType = r.getTypeOr(path+".Value", "StructProperty")
	}
	values := make([]map[string]any, 0, count)
	for i := uint32(0); i < count; i++ {
		k := r.propValue(keyType, keyStructType, path+".Key")
		v := r.propValue(valueType, valueStructType, path+".Value")
		values = append(values, map[string]any{"key": k, "value": v})
	}
	remaining := size - (r.pos - start)
	if remaining > 0 {
		r.Skip(remaining)
	}
	return map[string]any{
		"key_type": keyType, "value_type": valueType,
		"key_struct_type": keyStructType, "value_struct_type": valueStructType,
		"id": id, "value": values,
	}
}

func (r *FArchiveReader) readSetProperty(size int, path string) map[string]any {
	setType := r.FString()
	id := r.OptionalGuid()
	start := r.pos
	r.U32()
	count := r.U32()
	values := make([]PropertyList, 0, count)
	for i := uint32(0); i < count; i++ {
		values = append(values, r.PropertiesUntilEnd(path))
	}
	remaining := size - (r.pos - start)
	if remaining > 0 {
		r.Skip(remaining)
	}
	return map[string]any{"set_type": setType, "id": id, "value": values}
}

func (r *FArchiveReader) propValue(typeName, structType, path string) any {
	switch typeName {
	case "StructProperty":
		return r.StructValue(structType, path)
	case "EnumProperty", "NameProperty":
		return r.FString()
	case "IntProperty":
		return r.I32()
	case "BoolProperty":
		return r.Bool()
	case "UInt32Property":
		return r.U32()
	case "StrProperty":
		return r.FString()
	case "Int64Property":
		return r.I64()
	default:
		panic("unknown prop_value type: " + typeName + " (" + path + ")")
	}
}

// ---- Writer property dispatch (ports archive.py FArchiveWriter) ----

func (w *FArchiveWriter) Properties(pl PropertyList) {
	for _, e := range pl {
		w.FString(e.Name)
		w.Property(e.Value)
	}
	w.FString("None")
}

func (w *FArchiveWriter) Property(p map[string]any) {
	w.FString(p["type"].(string))
	sizePos := w.buf.Len()
	w.buf.Write(make([]byte, 8)) // placeholder for size
	size := w.propertyInner(p["type"].(string), p)
	endPos := w.buf.Len()
	sizeBytes := make([]byte, 8)
	binary.LittleEndian.PutUint64(sizeBytes, uint64(size))
	copy(w.buf.Bytes()[sizePos:sizePos+8], sizeBytes)
	_ = endPos
}

func (w *FArchiveWriter) propertyInner(propertyType string, p map[string]any) int {
	if ct, ok := p["custom_type"].(string); ok {
		cp, ok := w.customProperties[ct]
		if !ok {
			panic("unknown custom property type: " + ct)
		}
		return cp.Encode(w, propertyType, p)
	}
	switch propertyType {
	case "StructProperty":
		return w.writeStruct(p)
	case "IntProperty":
		w.OptionalGuid(asUUID(p["id"]))
		w.I32(asI32(p["value"]))
		return 4
	case "UInt16Property":
		w.OptionalGuid(asUUID(p["id"]))
		w.U16(asU16(p["value"]))
		return 2
	case "UInt32Property":
		w.OptionalGuid(asUUID(p["id"]))
		w.U32(asU32(p["value"]))
		return 4
	case "UInt64Property":
		w.OptionalGuid(asUUID(p["id"]))
		w.U64(asU64(p["value"]))
		return 8
	case "Int64Property":
		w.OptionalGuid(asUUID(p["id"]))
		w.I64(asI64(p["value"]))
		return 8
	case "FixedPoint64Property":
		w.OptionalGuid(asUUID(p["id"]))
		w.I32(asI32(p["value"]))
		return 4
	case "FloatProperty":
		w.OptionalGuid(asUUID(p["id"]))
		w.Float(asF32(p["value"]))
		return 4
	case "StrProperty", "NameProperty":
		w.OptionalGuid(asUUID(p["id"]))
		return w.fstringLen(p["value"].(string))
	case "EnumProperty":
		ev := p["value"].(map[string]any)
		w.FString(ev["type"].(string))
		w.OptionalGuid(asUUID(p["id"]))
		return w.fstringLen(ev["value"].(string))
	case "BoolProperty":
		w.Bool(p["value"].(bool))
		w.OptionalGuid(asUUID(p["id"]))
		return 0
	case "ByteProperty":
		ev := p["value"].(map[string]any)
		w.FString(ev["type"].(string))
		w.OptionalGuid(asUUID(p["id"]))
		if ev["type"] == "None" {
			w.Byte(ev["value"].(byte))
			return 1
		}
		return w.fstringLen(ev["value"].(string))
	case "ArrayProperty":
		w.FString(p["array_type"].(string))
		w.OptionalGuid(asUUID(p["id"]))
		start := w.buf.Len()
		w.writeArrayProperty(p["array_type"].(string), p["value"])
		return w.buf.Len() - start
	case "MapProperty":
		return w.writeMapProperty(p)
	case "SetProperty":
		return w.writeSetProperty(p)
	default:
		panic("unknown property type (write): " + propertyType)
	}
}

func (w *FArchiveWriter) writeStruct(p map[string]any) int {
	w.FString(p["struct_type"].(string))
	w.Guid(asUUID(p["struct_id"]))
	w.OptionalGuid(asUUID(p["id"]))
	start := w.buf.Len()
	w.StructValue(p["struct_type"].(string), p["value"])
	return w.buf.Len() - start
}

func (w *FArchiveWriter) StructValue(structType string, value any) {
	switch structType {
	case "Vector":
		v := value.(map[string]any)
		w.Double(asF64(v["x"]))
		w.Double(asF64(v["y"]))
		w.Double(asF64(v["z"]))
	case "DateTime":
		w.U64(asU64(value))
	case "Guid":
		w.Guid(asUUID(value))
	case "Quat":
		v := value.(map[string]any)
		w.Double(asF64(v["x"]))
		w.Double(asF64(v["y"]))
		w.Double(asF64(v["z"]))
		w.Double(asF64(v["w"]))
	case "LinearColor":
		v := value.(map[string]any)
		w.Float(asF32(v["r"]))
		w.Float(asF32(v["g"]))
		w.Float(asF32(v["b"]))
		w.Float(asF32(v["a"]))
	case "Color":
		v := value.(map[string]any)
		w.Byte(v["b"].(byte))
		w.Byte(v["g"].(byte))
		w.Byte(v["r"].(byte))
		w.Byte(v["a"].(byte))
	default:
		w.Properties(value.(PropertyList))
	}
}

func (w *FArchiveWriter) writeArrayProperty(arrayType string, value map[string]any) {
	if arrayType == "ByteProperty" {
		buf := value["values"].([]byte)
		w.U32(uint32(len(buf)))
		w.Write(buf)
		return
	}
	values := value["values"].([]any)
	w.U32(uint32(len(values)))
	if arrayType == "StructProperty" {
		w.FString(value["prop_name"].(string))
		w.FString(value["prop_type"].(string))
		sizePos := w.buf.Len()
		w.buf.Write(make([]byte, 8))
		w.FString(value["type_name"].(string))
		w.Guid(asUUID(value["id"]))
		w.Byte(0)
		dataStart := w.buf.Len()
		for _, v := range values {
			w.StructValue(value["type_name"].(string), v)
		}
		sizeBytes := make([]byte, 8)
		binary.LittleEndian.PutUint64(sizeBytes, uint64(w.buf.Len()-dataStart))
		copy(w.buf.Bytes()[sizePos:sizePos+8], sizeBytes)
		return
	}
	for _, v := range values {
		w.arrayValueWrite(arrayType, v)
	}
}

func (w *FArchiveWriter) arrayValueWrite(arrayType string, v any) {
	switch arrayType {
	case "IntProperty":
		w.I32(asI32(v))
	case "UInt32Property":
		w.U32(asU32(v))
	case "Int64Property":
		w.I64(asI64(v))
	case "FloatProperty":
		w.Float(asF32(v))
	case "StrProperty", "NameProperty", "EnumProperty":
		w.FString(v.(string))
	case "BoolProperty":
		w.Bool(v.(bool))
	case "ByteProperty":
		w.Byte(v.(byte))
	case "Guid":
		w.Guid(asUUID(v))
	default:
		panic("unknown array value type (write): " + arrayType)
	}
}

func (w *FArchiveWriter) writeMapProperty(p map[string]any) int {
	w.FString(p["key_type"].(string))
	w.FString(p["value_type"].(string))
	w.OptionalGuid(asUUID(p["id"]))
	start := w.buf.Len()
	w.U32(0)
	entries := p["value"].([]map[string]any)
	w.U32(uint32(len(entries)))
	for _, e := range entries {
		w.propValueWrite(p["key_type"].(string), p["key_struct_type"].(string), e["key"])
		w.propValueWrite(p["value_type"].(string), p["value_struct_type"].(string), e["value"])
	}
	return w.buf.Len() - start
}

func (w *FArchiveWriter) writeSetProperty(p map[string]any) int {
	w.FString(p["set_type"].(string))
	w.OptionalGuid(asUUID(p["id"]))
	start := w.buf.Len()
	w.U32(0)
	values := p["value"].([]PropertyList)
	w.U32(uint32(len(values)))
	for _, pl := range values {
		w.Properties(pl)
	}
	return w.buf.Len() - start
}

func (w *FArchiveWriter) propValueWrite(typeName, structType string, value any) {
	switch typeName {
	case "StructProperty":
		w.StructValue(structType, value)
	case "EnumProperty", "NameProperty":
		w.FString(value.(string))
	case "IntProperty":
		w.I32(asI32(value))
	case "BoolProperty":
		w.Bool(value.(bool))
	case "UInt32Property":
		w.U32(asU32(value))
	case "StrProperty":
		w.FString(value.(string))
	case "Int64Property":
		w.I64(asI64(value))
	default:
		panic("unknown prop_value type (write): " + typeName)
	}
}

// fstringLen writes an FString and returns its byte length in the payload.
func (w *FArchiveWriter) fstringLen(s string) int {
	start := w.buf.Len()
	w.FString(s)
	return w.buf.Len() - start
}

// ---- typed accessors (handle int/uint width from JSON-like any) ----

func asUUID(v any) *UUID {
	if v == nil {
		return nil
	}
	u, ok := v.(*UUID)
	if !ok {
		return nil
	}
	return u
}
func asI32(v any) int32 { return int32(v.(int32)) }
func asU16(v any) uint16 {
	switch x := v.(type) {
	case uint16:
		return x
	case int32:
		return uint16(x)
	}
	return 0
}
func asU32(v any) uint32 { return v.(uint32) }
func asU64(v any) uint64 {
	switch x := v.(type) {
	case uint64:
		return x
	case int64:
		return uint64(x)
	}
	return 0
}
func asI64(v any) int64 { return v.(int64) }
func asF32(v any) float32 { return v.(float32) }
func asF64(v any) float64 { return v.(float64) }
```
(Add `"encoding/binary"` import to properties.go.)

- [ ] **Step 4: Add GvasFile read/write to gvas.go**

Append to `internal/sav/gvas.go`:
```go
func ReadGvasFile(data []byte, typeHints map[string]string, custom map[string]CustomProperty) (*GvasFile, error) {
	r := NewFArchiveReader(data, typeHints, custom)
	hdr, err := ReadGvasHeader(r)
	if err != nil {
		return nil, err
	}
	props := r.PropertiesUntilEnd("")
	trailer := r.ReadToEnd()
	return &GvasFile{Header: hdr, Properties: props, Trailer: trailer}, nil
}

func (g *GvasFile) Write(custom map[string]CustomProperty) []byte {
	w := NewFArchiveWriter(custom)
	WriteGvasHeader(w, g.Header)
	w.Properties(g.Properties)
	w.Write(g.Trailer)
	return w.Bytes()
}
```

- [ ] **Step 5: Run tests to verify they pass**

Run: `go test ./internal/sav/ -v`
Expected: PASS for primitives, header, and synthetic property round-trip.

- [ ] **Step 6: Commit**

```bash
git add internal/sav/properties.go internal/sav/properties_test.go internal/sav/gvas.go
git -c user.name=codex -c user.email=codex@local commit -m "feat(sav): UE property system read/write dispatch + GvasFile"
```

---

<!-- Phase 1 continues: Task 7 (RawData: skip passthrough + character + group parsers + type hints + registry), Task 8 (gold GVAS byte-identical round-trip on all fixtures). Written in next increment. -->
<!-- Phase 2: palworld/detect.go, hostswap.go, backup.go, pack.go -->
<!-- Phase 3: storage/ (qiniu, lock) -->
<!-- Phase 4: config + Wails v3 scaffold + app.go bindings -->
<!-- Phase 5: React frontend -->
<!-- Phase 6: integration + single-exe packaging -->
