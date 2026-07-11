// Package sav reads and writes Palworld save files.
//
// A Palworld save is a SAV container holding GVAS bytes. The container is
// compressed with one of two algorithms, indicated by a 3-byte magic:
//
//   - PlZ: zlib (double-zlib when the save-type byte is 50, single when 49)
//   - PlM: Oodle (Epic's proprietary codec, loaded via the embedded DLL)
//
// Format choice is per-save, not per-file-type: the same file can be PlZ in one
// world and PlM in another. Decompress/Compress always preserve the original
// format so the game never receives a format it did not write.
package sav

import (
	"bytes"
	"compress/zlib"
	"encoding/binary"
	"fmt"
	"io"
)

// Save-type byte values (offset 11 of the SAV header).
const (
	saveTypeCNK = 48 // 0x30 - double-header container
	saveTypePLM = 49 // 0x31 - Oodle
	saveTypePLZ = 50 // 0x32 - double zlib
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
	Magic           [3]byte // "PlZ", "PlM", or "CNK"
	SaveType        byte
	DataOffset      int
}

// ParseSAVHeader parses the SAV container header. CNK wraps a second header.
func ParseSAVHeader(data []byte) (SAVHeader, error) {
	if len(data) < 12 {
		return SAVHeader{}, fmt.Errorf("sav: file too small for header: %d bytes", len(data))
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
			return SAVHeader{}, fmt.Errorf("sav: CNK file too small: %d bytes", len(data))
		}
		h.UncompressedLen = binary.LittleEndian.Uint32(data[12:16])
		h.CompressedLen = binary.LittleEndian.Uint32(data[16:20])
		copy(h.Magic[:], data[20:23])
		h.SaveType = data[23]
		h.DataOffset = 24
	}

	switch string(h.Magic[:]) {
	case "PlZ", "PlM", "CNK":
		return h, nil
	default:
		return SAVHeader{}, fmt.Errorf("sav: unknown magic %q", string(h.Magic[:]))
	}
}

// Decompress decodes a SAV file to its GVAS payload, returning the payload and
// the original header (so callers preserve the format on recompression).
func Decompress(data []byte) ([]byte, SAVHeader, error) {
	h, err := ParseSAVHeader(data)
	if err != nil {
		return nil, SAVHeader{}, err
	}
	payload := data[h.DataOffset:]
	switch string(h.Magic[:]) {
	case "PlZ":
		out, err := zlibDecompress(payload)
		if err != nil {
			return nil, h, fmt.Errorf("sav: zlib: %w", err)
		}
		if h.SaveType == saveTypePLZ {
			if out, err = zlibDecompress(out); err != nil {
				return nil, h, fmt.Errorf("sav: zlib (2nd pass): %w", err)
			}
		}
		if len(out) != int(h.UncompressedLen) {
			return nil, h, fmt.Errorf("sav: uncompressed len %d, want %d", len(out), h.UncompressedLen)
		}
		return out, h, nil
	case "PlM":
		comp := payload[:h.CompressedLen]
		out, err := OodleDecompress(comp, int(h.UncompressedLen))
		if err != nil {
			return nil, h, err
		}
		if len(out) != int(h.UncompressedLen) {
			return nil, h, fmt.Errorf("sav: uncompressed len %d, want %d", len(out), h.UncompressedLen)
		}
		return out, h, nil
	case "CNK":
		return nil, h, fmt.Errorf("sav: CNK inner decompress not supported")
	default:
		return nil, h, fmt.Errorf("sav: unsupported magic %q", string(h.Magic[:]))
	}
}

// Compress encodes GVAS bytes back into a SAV container, preserving the
// format recorded in h (magic + save type).
func Compress(gvas []byte, h SAVHeader) ([]byte, error) {
	var comp []byte
	var err error
	switch string(h.Magic[:]) {
	case "PlZ":
		if h.SaveType == saveTypePLZ {
			comp = zlibCompress(zlibCompress(gvas))
		} else {
			comp = zlibCompress(gvas)
		}
	case "PlM":
		comp, err = OodleCompress(gvas)
		if err != nil {
			return nil, err
		}
	case "CNK":
		return nil, fmt.Errorf("sav: CNK compress not supported")
	default:
		return nil, fmt.Errorf("sav: unsupported magic %q", string(h.Magic[:]))
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
