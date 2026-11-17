package camera

import (
	"bytes"
	"encoding/binary"
)

// The tags of the maker notes of Olympus that the package reads (the tags
// of Olympus in ExifTool).
const (
	tagOlympusThumbnail      = 0x0100
	tagOlympusPreviewStart   = 0x0088
	tagOlympusPreviewLength  = 0x0089
	tagOlympusCameraSettings = 0x2020
	tagSettingsPreviewStart  = 0x0101
	tagSettingsPreviewLength = 0x0102
)

// makerNote reads the maker notes of Olympus in the field f, and keeps the
// JPEG images that they hold. An ORF file keeps its large preview there, in
// the camera settings, and not in an IFD of the file. The maker notes of
// other makers are left out.
//
// The maker notes start with a header. "OLYMP\\0" has an IFD after 8 bytes,
// whose offsets count from the start of the TIFF file. "OLYMPUS\\0" and "OM
// SYSTEM\\0" have their own byte order, and an IFD after 12 or 16 bytes,
// whose offsets count from the start of the maker notes.
func (t *tiffReader) makerNote(f field) {
	start := int64(t.order.Uint32(f.value))
	head, ok := t.read(start, min(16, int64(f.count)))
	if !ok {
		return
	}
	notes := &tiffReader{src: t.src, size: t.size, order: t.order, lim: t.lim, seen: make(map[int64]bool)}
	var ifd int64
	switch {
	case bytes.HasPrefix(head, []byte("OLYMPUS\x00")) && len(head) >= 12:
		notes.base, ifd = t.base+start, 12
		notes.size = t.size - start
		if notes.order, ok = byteOrder(head[8:10]); !ok {
			return
		}
	case bytes.HasPrefix(head, []byte("OM SYSTEM\x00\x00\x00")) && len(head) >= 16:
		notes.base, ifd = t.base+start, 16
		notes.size = t.size - start
		if notes.order, ok = byteOrder(head[12:14]); !ok {
			return
		}
	case bytes.HasPrefix(head, []byte("OLYMP\x00")):
		notes.base, ifd = t.base, start+8
	default:
		return
	}
	fields, _, err := notes.fields(ifd)
	if err != nil || fields == nil {
		return
	}
	if th, ok := fields[tagOlympusThumbnail]; ok && th.typ == typeUndefined {
		t.regions = append(t.regions, region{off: notes.base + int64(notes.order.Uint32(th.value)), n: int64(th.count)})
	}
	if off, ok := notes.uint(fields, tagOlympusPreviewStart); ok {
		if n, ok := notes.uint(fields, tagOlympusPreviewLength); ok {
			t.regions = append(t.regions, region{off: notes.base + int64(off), n: int64(n)})
		}
	}
	settings, ok := fields[tagOlympusCameraSettings]
	if !ok {
		return
	}
	// the camera settings are an IFD, or a block of the type undefined
	// that holds one
	var at uint32
	switch settings.typ {
	case typeIFD, typeLong:
		at, ok = notes.uint(fields, tagOlympusCameraSettings)
	case typeUndefined:
		at = notes.order.Uint32(settings.value)
	default:
		ok = false
	}
	if !ok {
		return
	}
	sf, _, err := notes.fields(int64(at))
	if err != nil || sf == nil {
		return
	}
	if off, ok := notes.uint(sf, tagSettingsPreviewStart); ok {
		if n, ok := notes.uint(sf, tagSettingsPreviewLength); ok {
			t.regions = append(t.regions, region{off: notes.base + int64(off), n: int64(n)})
		}
	}
}

// byteOrder returns the byte order of the mark "II" or "MM".
func byteOrder(mark []byte) (binary.ByteOrder, bool) {
	switch string(mark) {
	case "II":
		return binary.LittleEndian, true
	case "MM":
		return binary.BigEndian, true
	}
	return nil, false
}
