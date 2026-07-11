package sav

import (
	_ "embed"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	oodle "github.com/new-world-tools/go-oodle"
)

//go:embed assets/oo2core_9_win64.dll
var oodleDLL []byte

var (
	oodleOnce    sync.Once
	oodleInitErr error
)

// ensureOodle extracts the embedded Oodle DLL to the directory go-oodle
// searches (os.TempDir/go-oodle) on first use. It is safe for concurrent use.
func ensureOodle() error {
	oodleOnce.Do(func() {
		dir := filepath.Join(os.TempDir(), "go-oodle")
		path := filepath.Join(dir, "oo2core_9_win64.dll")
		if _, err := os.Stat(path); os.IsNotExist(err) {
			if err = os.MkdirAll(dir, 0o755); err != nil {
				oodleInitErr = err
				return
			}
			if err = os.WriteFile(path, oodleDLL, 0o644); err != nil {
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
		return nil, fmt.Errorf("ensure oodle: %w", err)
	}
	out, err := oodle.Decompress(comp, int64(outLen))
	if err != nil {
		return nil, fmt.Errorf("oodle decompress: %w", err)
	}
	return out, nil
}

// OodleCompress compresses data with Oodle Kraken at the Normal level,
// matching what Palworld uses for PlM saves.
func OodleCompress(data []byte) ([]byte, error) {
	if err := ensureOodle(); err != nil {
		return nil, fmt.Errorf("ensure oodle: %w", err)
	}
	out, err := oodle.Compress(data, oodle.CompressorKraken, oodle.CompressionLevelNormal)
	if err != nil {
		return nil, fmt.Errorf("oodle compress: %w", err)
	}
	return out, nil
}
