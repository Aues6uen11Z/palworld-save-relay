package sav

import "fmt"

// GvasMagic is the GVAS file magic ("GVAS" little-endian).
const GvasMagic uint32 = 0x53415647

// CustomVersion is a (GUID, version) pair in the GVAS header.
type CustomVersion struct {
	ID      *UUID
	Version int32
}

// GvasHeader is the header of a GVAS file.
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

// GvasFile is a parsed GVAS file: header, ordered properties, and trailer.
type GvasFile struct {
	Header     GvasHeader
	Properties PropertyList
	Trailer    []byte
}

// ReadGvasHeader reads a GVAS header from r.
func ReadGvasHeader(r *FArchiveReader) (GvasHeader, error) {
	h := GvasHeader{Magic: r.U32()}
	if h.Magic != GvasMagic {
		return h, fmt.Errorf("sav: invalid GVAS magic %x", h.Magic)
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

// WriteGvasHeader writes a GVAS header to w.
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

// ReadGvasFile parses GVAS bytes into a GvasFile, applying type hints and
// custom-property handlers.
func ReadGvasFile(data []byte, typeHints map[string]string, custom map[string]CustomProperty) (*GvasFile, error) {
	r := NewFArchiveReader(data, typeHints, custom)
	hdr, err := ReadGvasHeader(r)
	if err != nil {
		return nil, err
	}
	props := r.PropertiesUntilEnd("")
	return &GvasFile{Header: hdr, Properties: props, Trailer: r.ReadToEnd()}, nil
}

// Write serializes the GvasFile back to GVAS bytes.
func (g *GvasFile) Write(custom map[string]CustomProperty) []byte {
	w := NewFArchiveWriter(custom)
	WriteGvasHeader(w, g.Header)
	w.Properties(g.Properties)
	w.Write(g.Trailer)
	return w.Bytes()
}
