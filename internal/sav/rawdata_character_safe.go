package sav

import "fmt"

// characterDecodeSafe runs characterDecode, falling back to an opaque skip on
// any error (matching cheahjs' _make_decode_safe). This keeps round-tripping
// robust for malformed/unexpected entries; __skip__ marks fallback results.
func characterDecodeSafe(r *FArchiveReader, typeName string, size int, path string) (val map[string]any) {
	start := r.pos
	defer func() {
		if rec := recover(); rec != nil {
			r.pos = start
			val = skipDecode(r, typeName, size, path)
			val["custom_type"] = path
			val["__skip__"] = true
		}
	}()
	val = characterDecode(r, typeName, size, path)
	val["__skip__"] = false
	return
}

func characterEncodeSafe(w *FArchiveWriter, propertyType string, p map[string]any) int {
	if skip, ok := p["__skip__"].(bool); ok && skip {
		delete(p, "custom_type")
		delete(p, "__skip__")
		return skipEncode(w, propertyType, p)
	}
	delete(p, "__skip__")
	return characterEncode(w, propertyType, p)
}

var _ = fmt.Sprintf
