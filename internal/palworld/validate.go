package palworld

import (
	"archive/zip"
	"bytes"
	"fmt"
	"io"
	"path/filepath"
	"strings"

	"palworld-save-relay/internal/sav"
)

// ValidateWorldZip checks that a world zip is well-formed without touching the
// filesystem. It verifies the zip can be read, and that every .sav file inside
// can be decompressed and parsed as a valid GVAS document. A zip without
// Level.sav is rejected (a relay intermediate must contain the world data).
//
// Use this after a cloud download or file import, before writing to the live
// world, so corrupt or truncated data never reaches the save folder.
func ValidateWorldZip(zipBytes []byte) error {
	zr, err := zip.NewReader(bytes.NewReader(zipBytes), int64(len(zipBytes)))
	if err != nil {
		return fmt.Errorf("validate: invalid zip: %w", err)
	}
	hints, custom := sav.PalWorldConfig()
	foundLevel := false
	for _, f := range zr.File {
		name := filepath.ToSlash(f.Name)
		if strings.HasPrefix(name, "_") {
			continue // skip metadata files
		}
		if filepath.Ext(name) != ".sav" {
			continue
		}
		if name == "Level.sav" {
			foundLevel = true
		}
		if err := validateSAVEntry(f, hints, custom); err != nil {
			return err
		}
	}
	if !foundLevel {
		return fmt.Errorf("validate: Level.sav not found in zip")
	}
	return nil
}

// validateSAVEntry decompresses and parses a single .sav zip entry.
func validateSAVEntry(f *zip.File, hints map[string]string, custom map[string]sav.CustomProperty) error {
	name := filepath.ToSlash(f.Name)
	rc, err := f.Open()
	if err != nil {
		return fmt.Errorf("validate: open %s: %w", name, err)
	}
	data, err := io.ReadAll(rc)
	rc.Close()
	if err != nil {
		return fmt.Errorf("validate: read %s: %w", name, err)
	}
	gvas, _, err := sav.Decompress(data)
	if err != nil {
		return fmt.Errorf("validate: decompress %s: %w", name, err)
	}
	if _, err := sav.ReadGvasFile(gvas, hints, custom); err != nil {
		return fmt.Errorf("validate: parse %s: %w", name, err)
	}
	return nil
}
