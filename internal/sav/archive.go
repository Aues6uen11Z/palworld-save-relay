package sav

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"math"
	"unicode/utf16"
)

// UUID is a Palworld GUID: 16 raw bytes as stored on disk.
//
// Host-swap compares and swaps raw bytes (format-agnostic). String uses
// Palworld's mixed-endian layout for display only; it does not affect parsing
// or round-tripping.
type UUID [16]byte

// Equal reports whether two GUIDs are byte-identical. Nil-safe.
func (u *UUID) Equal(o *UUID) bool {
	if u == nil || o == nil {
		return u == o
	}
	return *u == *o
}

// String returns the Palworld mixed-endian formatted GUID, for display only.
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

// PropertyList is an ordered list of named properties. UE property sequences
// are ordered, and Go maps are not, so this preserves read order for faithful
// byte-identical round-trips.
type PropertyList []PropertyEntry

// Get returns the value of the first property named name, or nil.
func (pl PropertyList) Get(name string) map[string]any {
	for _, e := range pl {
		if e.Name == name {
			return e.Value
		}
	}
	return nil
}

// CustomProperty is a (decode, encode) pair registered for a property path,
// used to parse opaque RawData blobs (e.g. guild / character save data).
type CustomProperty struct {
	Decode func(r *FArchiveReader, typeName string, size int, path string) map[string]any
	Encode func(w *FArchiveWriter, propertyType string, p map[string]any) int
}

// ---- FArchiveReader ----

// FArchiveReader reads little-endian UE archive primitives from a byte slice.
type FArchiveReader struct {
	data             []byte
	pos              int
	typeHints        map[string]string
	customProperties map[string]CustomProperty
}

// NewFArchiveReader creates a reader over data with optional type hints and
// custom-property handlers.
func NewFArchiveReader(data []byte, typeHints map[string]string, custom map[string]CustomProperty) *FArchiveReader {
	return &FArchiveReader{data: data, typeHints: typeHints, customProperties: custom}
}

// Pos returns the current read offset.
func (r *FArchiveReader) Pos() int { return r.pos }

// EOF reports whether the reader is at end of data.
func (r *FArchiveReader) EOF() bool { return r.pos >= len(r.data) }

// Read consumes and returns the next n bytes.
func (r *FArchiveReader) Read(n int) []byte {
	if r.pos+n > len(r.data) {
		panic(fmt.Sprintf("sav: read past end: pos=%d n=%d len=%d", r.pos, n, len(r.data)))
	}
	b := r.data[r.pos : r.pos+n]
	r.pos += n
	return b
}

// ReadToEnd consumes and returns all remaining bytes.
func (r *FArchiveReader) ReadToEnd() []byte {
	b := r.data[r.pos:]
	r.pos = len(r.data)
	return b
}

// Skip advances the read position by n bytes.
func (r *FArchiveReader) Skip(n int) { r.pos += n }

func (r *FArchiveReader) Byte() byte  { return r.Read(1)[0] }
func (r *FArchiveReader) Bool() bool  { return r.Byte() > 0 }
func (r *FArchiveReader) I16() int16  { return int16(binary.LittleEndian.Uint16(r.Read(2))) }
func (r *FArchiveReader) U16() uint16 { return binary.LittleEndian.Uint16(r.Read(2)) }
func (r *FArchiveReader) I32() int32  { return int32(binary.LittleEndian.Uint32(r.Read(4))) }
func (r *FArchiveReader) U32() uint32 { return binary.LittleEndian.Uint32(r.Read(4)) }
func (r *FArchiveReader) I64() int64  { return int64(binary.LittleEndian.Uint64(r.Read(8))) }
func (r *FArchiveReader) U64() uint64 { return binary.LittleEndian.Uint64(r.Read(8)) }
func (r *FArchiveReader) Float() float32 {
	return math.Float32frombits(binary.LittleEndian.Uint32(r.Read(4)))
}
func (r *FArchiveReader) Double() float64 {
	return math.Float64frombits(binary.LittleEndian.Uint64(r.Read(8)))
}

// Guid reads a 16-byte GUID.
func (r *FArchiveReader) Guid() *UUID {
	var u UUID
	copy(u[:], r.Read(16))
	return &u
}

// OptionalGuid reads a GUID preceded by a presence flag.
func (r *FArchiveReader) OptionalGuid() *UUID {
	if r.Byte() != 0 {
		return r.Guid()
	}
	return nil
}

// FString reads a length-prefixed string (ascii or UTF-16-LE).
func (r *FArchiveReader) FString() string {
	size := int(r.I32())
	if size == 0 {
		return ""
	}
	if size < 0 { // UTF-16-LE
		size = -size
		b := r.Read(size * 2)
		return decodeUTF16LE(b[:len(b)-2]) // drop 2-byte null terminator
	}
	b := r.Read(size)
	return string(b[:len(b)-1]) // drop 1-byte null terminator
}

// TArray reads a uint32-counted array using readElem for each element.
func (r *FArchiveReader) TArray(readElem func() any) []any {
	count := int(r.U32())
	arr := make([]any, 0, count)
	for i := 0; i < count; i++ {
		arr = append(arr, readElem())
	}
	return arr
}

// ByteList reads n raw bytes.
func (r *FArchiveReader) ByteList(n int) []byte { return r.Read(n) }

func (r *FArchiveReader) getTypeOr(path, def string) string {
	if t, ok := r.typeHints[path]; ok {
		return t
	}
	return def
}

// InternalCopy returns a new reader over a sub-buffer, sharing config.
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

// ---- FArchiveWriter ----

// FArchiveWriter writes little-endian UE archive primitives to a buffer.
type FArchiveWriter struct {
	buf              *bytes.Buffer
	customProperties map[string]CustomProperty
}

// NewFArchiveWriter creates a writer with optional custom-property handlers.
func NewFArchiveWriter(custom map[string]CustomProperty) *FArchiveWriter {
	return &FArchiveWriter{buf: new(bytes.Buffer), customProperties: custom}
}

// Bytes returns the accumulated output.
func (w *FArchiveWriter) Bytes() []byte { return w.buf.Bytes() }

// Len returns the current output length.
func (w *FArchiveWriter) Len() int { return w.buf.Len() }

// Write appends raw bytes.
func (w *FArchiveWriter) Write(b []byte) { w.buf.Write(b) }

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
func (w *FArchiveWriter) I16(v int16) {
	var b [2]byte
	binary.LittleEndian.PutUint16(b[:], uint16(v))
	w.buf.Write(b[:])
}
func (w *FArchiveWriter) U16(v uint16) {
	var b [2]byte
	binary.LittleEndian.PutUint16(b[:], v)
	w.buf.Write(b[:])
}
func (w *FArchiveWriter) I32(v int32) {
	var b [4]byte
	binary.LittleEndian.PutUint32(b[:], uint32(v))
	w.buf.Write(b[:])
}
func (w *FArchiveWriter) U32(v uint32) {
	var b [4]byte
	binary.LittleEndian.PutUint32(b[:], v)
	w.buf.Write(b[:])
}
func (w *FArchiveWriter) I64(v int64) {
	var b [8]byte
	binary.LittleEndian.PutUint64(b[:], uint64(v))
	w.buf.Write(b[:])
}
func (w *FArchiveWriter) U64(v uint64) {
	var b [8]byte
	binary.LittleEndian.PutUint64(b[:], v)
	w.buf.Write(b[:])
}
func (w *FArchiveWriter) Float(v float32) {
	var b [4]byte
	binary.LittleEndian.PutUint32(b[:], math.Float32bits(v))
	w.buf.Write(b[:])
}
func (w *FArchiveWriter) Double(v float64) {
	var b [8]byte
	binary.LittleEndian.PutUint64(b[:], math.Float64bits(v))
	w.buf.Write(b[:])
}

// Guid writes a 16-byte GUID (zero if nil).
func (w *FArchiveWriter) Guid(u *UUID) {
	if u == nil {
		var z UUID
		w.buf.Write(z[:])
	} else {
		w.buf.Write(u[:])
	}
}

// OptionalGuid writes a presence flag then, if present, the GUID.
func (w *FArchiveWriter) OptionalGuid(u *UUID) {
	if u == nil {
		w.Bool(false)
	} else {
		w.Bool(true)
		w.Guid(u)
	}
}

// FString writes a length-prefixed string (ascii or UTF-16-LE).
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

// TArray writes a uint32 count then each element via writeElem.
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
