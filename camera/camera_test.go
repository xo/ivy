package camera

import (
	"bytes"
	"encoding/binary"
	"errors"
	"image"
	"image/color"
	"slices"
	"testing"
	"time"

	"github.com/xo/ivy"
	"github.com/xo/ivy/internal/fixture"
)

// entry is one entry of an IFD that tiffFile builds. The value is the
// offset of blob, or of the IFD ifd, when one of them is not negative.
type entry struct {
	tag, typ uint16
	count    uint32
	value    uint32
	blob     int
	ifd      int
}

// val returns an entry with an inline value.
func val(tag, typ uint16, count, value uint32) entry {
	return entry{tag: tag, typ: typ, count: count, value: value, blob: -1, ifd: -1}
}

// blob returns an entry whose value is the offset of blob i, which is count
// values long.
func blobRef(tag, typ uint16, count uint32, i int) entry {
	return entry{tag: tag, typ: typ, count: count, blob: i, ifd: -1}
}

// tiffFile builds a little endian TIFF file: the header, then the IFDs, each
// pointing to the next one when chain is true, then the blobs.
func tiffFile(header []byte, chain bool, ifds [][]entry, blobs [][]byte) []byte {
	le := binary.LittleEndian
	ifdOff := make([]uint32, len(ifds))
	off := uint32(len(header))
	for i, ifd := range ifds {
		ifdOff[i] = off
		off += 2 + 12*uint32(len(ifd)) + 4
	}
	blobOff := make([]uint32, len(blobs))
	for i, b := range blobs {
		blobOff[i] = off
		off += uint32(len(b))
	}
	out := bytes.Clone(header)
	for i, ifd := range ifds {
		out = le.AppendUint16(out, uint16(len(ifd)))
		for _, e := range ifd {
			v := e.value
			switch {
			case e.blob >= 0:
				v = blobOff[e.blob]
			case e.ifd >= 0:
				v = ifdOff[e.ifd]
			}
			out = le.AppendUint16(out, e.tag)
			out = le.AppendUint16(out, e.typ)
			out = le.AppendUint32(out, e.count)
			out = le.AppendUint32(out, v)
		}
		next := uint32(0)
		if chain && i+1 < len(ifds) {
			next = ifdOff[i+1]
		}
		out = le.AppendUint32(out, next)
	}
	for _, b := range blobs {
		out = append(out, b...)
	}
	return out
}

// nef returns a TIFF file of a camera: IFD0 holds the facts of the camera
// and a small JPEG, and its SubIFD holds a large JPEG as one strip. A third
// region holds data that is not a JPEG.
func nef(header []byte) []byte {
	small := fixture.JPEG(64, 48, color.White)
	large := fixture.JPEG(320, 240, color.NRGBA{255, 0, 0, 255})
	blobs := [][]byte{
		[]byte("NIKON CORPORATION\x00"),
		[]byte("NIKON Z 8\x00"),
		[]byte("2026:11:16 10:20:30\x00"),
		small,
		large,
		[]byte("not a jpeg at all"),
	}
	ifd0 := []entry{
		blobRef(tagMake, typeASCII, uint32(len(blobs[0])), 0),
		blobRef(tagModel, typeASCII, uint32(len(blobs[1])), 1),
		blobRef(tagDateTime, typeASCII, uint32(len(blobs[2])), 2),
		{tag: tagSubIFDs, typ: typeLong, count: 1, blob: -1, ifd: 1},
		blobRef(tagJPEGOffset, typeLong, 1, 3),
		val(tagJPEGLength, typeLong, 1, uint32(len(small))),
	}
	ifd1 := []entry{
		val(tagCompression, typeShort, 1, 6),
		blobRef(tagStripOffsets, typeLong, 1, 4),
		val(tagStripByteCounts, typeLong, 1, uint32(len(large))),
		blobRef(tagJpgFromRaw, typeUndefined, uint32(len(blobs[5])), 5),
	}
	return tiffFile(header, false, [][]entry{ifd0, ifd1}, blobs)
}

// tiffHeader returns the header of a little endian TIFF file whose first IFD
// follows it.
func tiffHeader() []byte {
	return []byte{'I', 'I', 42, 0, 8, 0, 0, 0}
}

func TestTIFF(t *testing.T) {
	d, err := Open(t.Context(), bytes.NewReader(nef(tiffHeader())))
	if err != nil {
		t.Fatal(err)
	}
	var sizes []image.Point
	for p, err := range d.Previews(t.Context()) {
		if err != nil {
			t.Fatal(err)
		}
		sizes = append(sizes, p.Size)
	}
	if len(sizes) != 2 || sizes[0] != image.Pt(320, 240) || sizes[1] != image.Pt(64, 48) {
		t.Fatalf("expected the two JPEG images, largest first, got: %v", sizes)
	}
	img, err := ivy.BestPreview(t.Context(), d, image.Point{})
	if err != nil {
		t.Fatal(err)
	}
	r, _, _, _ := img.At(10, 10).RGBA()
	if img.Bounds().Size() != image.Pt(320, 240) || r < 0xf000 {
		t.Errorf("expected the large red JPEG, got: %v", img.Bounds().Size())
	}
	info, err := d.Describe(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if info.Producer != "NIKON CORPORATION NIKON Z 8" {
		t.Errorf("expected the make and the model, got: %q", info.Producer)
	}
	if !info.Created.Equal(time.Date(2026, 11, 16, 10, 20, 30, 0, time.UTC)) {
		t.Errorf("expected the date, got: %v", info.Created)
	}
}

func TestCR2ImageDecode(t *testing.T) {
	header := []byte{'I', 'I', 42, 0, 16, 0, 0, 0, 'C', 'R', 2, 0, 0, 0, 0, 0}
	img, name, err := image.Decode(bytes.NewReader(nef(header)))
	if err != nil {
		t.Fatal(err)
	}
	if name != "cr2" || img.Bounds().Size() != image.Pt(320, 240) {
		t.Errorf("expected the large preview of a cr2, got: %s %v", name, img.Bounds().Size())
	}
	cfg, _, err := image.DecodeConfig(bytes.NewReader(nef(header)))
	if err != nil || cfg.Width != 320 {
		t.Errorf("expected a width of 320, got: %+v %v", cfg, err)
	}
}

// box returns an ISO BMFF box.
func box(typ string, body ...[]byte) []byte {
	b := bytes.Join(body, nil)
	out := binary.BigEndian.AppendUint32(nil, uint32(8+len(b)))
	return append(append(out, typ...), b...)
}

func TestCR3(t *testing.T) {
	thumb := fixture.JPEG(160, 120, color.White)
	prvw := fixture.JPEG(1620, 1080, color.Black)
	cmt1 := tiffFile(tiffHeader(), false, [][]entry{{
		blobRef(tagMake, typeASCII, 6, 0),
		blobRef(tagModel, typeASCII, 9, 1),
	}}, [][]byte{[]byte("Canon\x00"), []byte("Canon R5\x00")})
	file := bytes.Join([][]byte{
		box("ftyp", []byte("crx \x00\x00\x00\x01crx isom")),
		box("moov", box("uuid", make([]byte, 16), box(boxMetadata, cmt1), box(boxThumbnail, make([]byte, 16), thumb))),
		box("uuid", make([]byte, 16), make([]byte, 8), box(boxPreview, make([]byte, 16), prvw)),
		box("mdat", []byte("sensor data")),
	}, nil)
	d, err := Open(t.Context(), bytes.NewReader(file))
	if err != nil {
		t.Fatal(err)
	}
	var sizes []image.Point
	for p, err := range d.Previews(t.Context()) {
		if err != nil {
			t.Fatal(err)
		}
		sizes = append(sizes, p.Size)
	}
	if len(sizes) != 2 || sizes[0] != image.Pt(1620, 1080) || sizes[1] != image.Pt(160, 120) {
		t.Errorf("expected PRVW and THMB, got: %v", sizes)
	}
	if info, _ := d.Describe(t.Context()); info.Producer != "Canon R5" {
		t.Errorf("expected Canon R5, got: %q", info.Producer)
	}
	if _, name, err := image.DecodeConfig(bytes.NewReader(file)); err != nil || name != "cr3" {
		t.Errorf("expected cr3, got: %q %v", name, err)
	}
}

func TestRAF(t *testing.T) {
	jpg := fixture.JPEG(48, 32, color.White)
	head := make([]byte, 100)
	copy(head, rafMagic)
	copy(head[28:], "X-T5")
	binary.BigEndian.PutUint32(head[84:], 100)
	binary.BigEndian.PutUint32(head[88:], uint32(len(jpg)))
	file := slices.Concat(head, jpg)
	d, err := Open(t.Context(), bytes.NewReader(file))
	if err != nil {
		t.Fatal(err)
	}
	img, err := ivy.BestPreview(t.Context(), d, image.Point{})
	if err != nil || img.Bounds().Size() != image.Pt(48, 32) {
		t.Fatalf("expected 48x32, got: %v", err)
	}
	if info, _ := d.Describe(t.Context()); info.Producer != "X-T5" {
		t.Errorf("expected X-T5, got: %q", info.Producer)
	}
}

func TestErrors(t *testing.T) {
	tests := []struct {
		name string
		buf  []byte
		want error
	}{
		{"short", []byte("II*"), ivy.ErrFormat},
		{"unknown", []byte("0123456789abcdef"), ivy.ErrFormat},
		{"bad magic", append([]byte{'I', 'I', 7, 0, 8, 0, 0, 0}, make([]byte, 8)...), ivy.ErrFormat},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if _, err := Open(t.Context(), bytes.NewReader(test.buf)); !errors.Is(err, test.want) {
				t.Errorf("expected %v, got: %v", test.want, err)
			}
		})
	}
}

func TestLoopAndLimits(t *testing.T) {
	// IFD0 points to itself as its next IFD and as its SubIFD
	loop := tiffFile(tiffHeader(), true, [][]entry{{
		{tag: tagSubIFDs, typ: typeLong, count: 1, blob: -1, ifd: 0},
	}}, nil)
	binary.LittleEndian.PutUint32(loop[8+2+12:], 8)
	d, err := Open(t.Context(), bytes.NewReader(loop))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ivy.BestPreview(t.Context(), d, image.Point{}); !errors.Is(err, ivy.ErrNoPreview) {
		t.Errorf("expected ErrNoPreview, got: %v", err)
	}
	cfg := ivy.Config{Limits: ivy.Limits{MaxObjects: 1}}
	if _, err := Open(t.Context(), bytes.NewReader(nef(tiffHeader())), WithConfig(cfg)); !errors.Is(err, ivy.ErrLimit) {
		t.Errorf("expected ErrLimit, got: %v", err)
	}
}

func FuzzOpen(f *testing.F) {
	f.Add(nef(tiffHeader()))
	f.Add(bytes.Join([][]byte{box("ftyp", []byte("crx ")), box("moov", box(boxThumbnail, fixture.JPEG(8, 8, color.White)))}, nil))
	cfg := ivy.Config{Limits: ivy.Limits{MaxMemory: 1 << 20, MaxPixels: 1 << 16, MaxObjects: 256, MaxDepth: 8}}
	f.Fuzz(func(t *testing.T, buf []byte) {
		d, err := Open(t.Context(), bytes.NewReader(buf), WithConfig(cfg))
		if err != nil {
			return
		}
		for p, err := range d.Previews(t.Context()) {
			if err == nil {
				_, _ = p.Decode(t.Context())
			}
		}
	})
}
