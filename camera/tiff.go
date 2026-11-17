package camera

import (
	"encoding/binary"
	"fmt"
	"strings"
	"time"

	"github.com/xo/ivy"
)

// The tags of TIFF and of EXIF that the package reads.
const (
	tagCompression     = 0x0103
	tagMake            = 0x010f
	tagModel           = 0x0110
	tagStripOffsets    = 0x0111
	tagStripByteCounts = 0x0117
	tagDateTime        = 0x0132
	tagArtist          = 0x013b
	tagSubIFDs         = 0x014a
	tagJPEGOffset      = 0x0201
	tagJPEGLength      = 0x0202
	tagExifIFD         = 0x8769
	tagDateOriginal    = 0x9003
	tagMakerNote       = 0x927c
	// tagJpgFromRaw holds the whole JPEG of an RW2 file of Panasonic.
	tagJpgFromRaw = 0x002e
)

// The types of a TIFF field.
const (
	typeByte      = 1
	typeASCII     = 2
	typeShort     = 3
	typeLong      = 4
	typeUndefined = 7
	typeIFD       = 13
)

// maxEntries is the number of entries of one IFD that the walk reads.
const maxEntries = 4096

// tiffReader walks the IFDs of a TIFF file, and finds its JPEG images and the
// facts of the camera.
type tiffReader struct {
	src   ivy.Source
	base  int64
	size  int64
	order binary.ByteOrder
	lim   ivy.Limits

	seen    map[int64]bool
	first   int64
	regions []region
	info    ivy.Info
}

// newTIFFReader reads the header of the TIFF file that starts at base in src
// and is size bytes long. The offsets of the file count from base.
func newTIFFReader(src ivy.Source, base, size int64, lim ivy.Limits) (*tiffReader, error) {
	head := make([]byte, 8)
	if n, _ := src.ReadAt(head, base); n < len(head) {
		return nil, fmt.Errorf("short tiff header: %w", ivy.ErrFormat)
	}
	t := &tiffReader{src: src, base: base, size: size, lim: lim, seen: make(map[int64]bool)}
	switch string(head[:2]) {
	case "II":
		t.order = binary.LittleEndian
	case "MM":
		t.order = binary.BigEndian
	default:
		return nil, fmt.Errorf("tiff byte order %q: %w", head[:2], ivy.ErrFormat)
	}
	// 42 is TIFF, 0x55 is RW2 of Panasonic, and RO and RS are ORF of Olympus
	switch t.order.Uint16(head[2:]) {
	case 42, 0x55, 0x4f52, 0x5352, 0x524f, 0x5253:
	default:
		return nil, fmt.Errorf("tiff magic %x: %w", head[2:4], ivy.ErrFormat)
	}
	t.first = int64(t.order.Uint32(head[4:]))
	return t, nil
}

// walk walks the chain of IFDs that starts at the first IFD, with the IFDs
// that they point to.
func (t *tiffReader) walk() error {
	return t.walkChain(t.first, 0)
}

// walkChain walks the IFD at off and the IFDs that follow it.
func (t *tiffReader) walkChain(off int64, depth int) error {
	for off != 0 {
		next, err := t.walkIFD(off, depth)
		if err != nil {
			return err
		}
		off = next
	}
	return nil
}

// field is one entry of an IFD.
type field struct {
	typ   uint16
	count uint32
	// value holds the 4 bytes of the value, or of its offset.
	value []byte
}

// walkIFD reads the IFD at off, and returns the offset of the next IFD.
func (t *tiffReader) walkIFD(off int64, depth int) (int64, error) {
	switch {
	case depth > t.lim.MaxDepth:
		return 0, fmt.Errorf("tiff ifds nested deeper than %d: %w", t.lim.MaxDepth, ivy.ErrLimit)
	case t.seen[off]:
		// a loop of IFDs ends the walk
		return 0, nil
	case len(t.seen) >= t.lim.MaxObjects:
		return 0, fmt.Errorf("more than %d tiff ifds: %w", t.lim.MaxObjects, ivy.ErrLimit)
	}
	t.seen[off] = true
	fields, next, err := t.fields(off)
	if err != nil || fields == nil {
		return 0, err
	}
	t.readFields(fields)
	if f, ok := fields[tagMakerNote]; ok && f.typ == typeUndefined {
		t.makerNote(f)
	}
	for _, tag := range []uint16{tagSubIFDs, tagExifIFD} {
		f, ok := fields[tag]
		if !ok {
			continue
		}
		for _, sub := range t.uints(f) {
			if err := t.walkChain(int64(sub), depth+1); err != nil {
				return 0, err
			}
		}
	}
	return next, nil
}

// fields reads the entries of the IFD at off, and the offset of the next
// IFD. It returns nil if the IFD is outside the file.
func (t *tiffReader) fields(off int64) (map[uint16]field, int64, error) {
	buf, ok := t.read(off, 2)
	if !ok {
		return nil, 0, nil
	}
	n := int(t.order.Uint16(buf))
	if n > maxEntries {
		return nil, 0, fmt.Errorf("tiff ifd of %d entries: %w", n, ivy.ErrFormat)
	}
	buf, ok = t.read(off+2, int64(n)*12+4)
	if !ok {
		return nil, 0, nil
	}
	fields := make(map[uint16]field, n)
	for i := range n {
		e := buf[i*12 : i*12+12]
		fields[t.order.Uint16(e)] = field{typ: t.order.Uint16(e[2:]), count: t.order.Uint32(e[4:]), value: e[8:12]}
	}
	return fields, int64(t.order.Uint32(buf[n*12:])), nil
}

// readFields keeps the JPEG images and the facts of the camera that the fields
// of one IFD hold.
func (t *tiffReader) readFields(fields map[uint16]field) {
	if off, ok := t.uint(fields, tagJPEGOffset); ok {
		if n, ok := t.uint(fields, tagJPEGLength); ok {
			t.regions = append(t.regions, region{off: t.base + int64(off), n: int64(n)})
		}
	}
	// a single strip that JPEG compresses: 6 is the old form of JPEG of TIFF,
	// and 7 is JPEG, which can also be lossless
	if c, ok := t.uint(fields, tagCompression); ok && (c == 6 || c == 7) {
		offs, counts := t.uints(fields[tagStripOffsets]), t.uints(fields[tagStripByteCounts])
		if len(offs) == 1 && len(counts) == 1 {
			t.regions = append(t.regions, region{off: t.base + int64(offs[0]), n: int64(counts[0])})
		}
	}
	if f, ok := fields[tagJpgFromRaw]; ok && (f.typ == typeUndefined || f.typ == typeByte) {
		t.regions = append(t.regions, region{off: t.base + int64(t.order.Uint32(f.value)), n: int64(f.count)})
	}
	maker, model := t.ascii(fields, tagMake), t.ascii(fields, tagModel)
	if model != "" && !strings.HasPrefix(model, maker) {
		model = strings.TrimSpace(maker + " " + model)
	}
	if model == "" {
		model = maker
	}
	if model != "" && t.info.Producer == "" {
		t.info.Producer = model
	}
	if artist := t.ascii(fields, tagArtist); artist != "" && t.info.Authors == nil {
		t.info.Authors = []string{artist}
	}
	for _, tag := range []uint16{tagDateOriginal, tagDateTime} {
		if d, err := time.Parse("2006:01:02 15:04:05", t.ascii(fields, tag)); err == nil {
			if t.info.Created.IsZero() || tag == tagDateOriginal {
				t.info.Created = d
			}
			break
		}
	}
}

// read reads n bytes at the offset off of the TIFF file. It returns false if
// they are outside the file.
func (t *tiffReader) read(off, n int64) ([]byte, bool) {
	if off < 0 || n < 0 || off > t.size || n > t.size-off || n > t.lim.MaxMemory {
		return nil, false
	}
	buf := make([]byte, n)
	if _, err := t.src.ReadAt(buf, t.base+off); err != nil {
		return nil, false
	}
	return buf, true
}

// uint returns the first value of the field tag of fields, which is a short
// or a long.
func (t *tiffReader) uint(fields map[uint16]field, tag uint16) (uint32, bool) {
	vs := t.uints(fields[tag])
	if len(vs) == 0 {
		return 0, false
	}
	return vs[0], true
}

// uints returns the values of a field of shorts, longs or offsets of IFDs. It
// returns nil for a field of another type, or one outside the file.
func (t *tiffReader) uints(f field) []uint32 {
	var size int64
	switch f.typ {
	case typeShort:
		size = 2
	case typeLong, typeIFD:
		size = 4
	default:
		return nil
	}
	if f.count == 0 || int64(f.count) > int64(t.lim.MaxObjects) {
		return nil
	}
	data := f.value
	if n := size * int64(f.count); n > 4 {
		var ok bool
		if data, ok = t.read(int64(t.order.Uint32(f.value)), n); !ok {
			return nil
		}
	}
	out := make([]uint32, f.count)
	for i := range out {
		if size == 2 {
			out[i] = uint32(t.order.Uint16(data[i*2:]))
		} else {
			out[i] = t.order.Uint32(data[i*4:])
		}
	}
	return out
}

// ascii returns the text of the field tag of fields, which is ascii.
func (t *tiffReader) ascii(fields map[uint16]field, tag uint16) string {
	f, ok := fields[tag]
	if !ok || f.typ != typeASCII || f.count == 0 || f.count > 1024 {
		return ""
	}
	data := f.value[:min(f.count, 4)]
	if f.count > 4 {
		var ok bool
		if data, ok = t.read(int64(t.order.Uint32(f.value)), int64(f.count)); !ok {
			return ""
		}
	}
	s, _, _ := strings.Cut(string(data), "\x00")
	return strings.TrimSpace(s)
}
