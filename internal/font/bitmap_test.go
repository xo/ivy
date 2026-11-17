package font

import (
	"encoding/binary"
	"image"
	"slices"
	"testing"

	gofont "github.com/xo/okage/typesetting/font"
	"github.com/xo/okage/typesetting/font/opentype"
	"golang.org/x/image/font/gofont/goregular"
)

// withTables returns the TrueType font data with the tables added to its
// table directory, in the order of their tags. It does not compute the
// checksums, which go-text does not read.
func withTables(data []byte, tables map[string][]byte) []byte {
	type rec struct {
		tag  string
		body []byte
	}
	n := int(binary.BigEndian.Uint16(data[4:]))
	recs := make([]rec, 0, n+len(tables))
	for i := range n {
		r := data[12+16*i:]
		off, length := binary.BigEndian.Uint32(r[8:]), binary.BigEndian.Uint32(r[12:])
		recs = append(recs, rec{string(r[:4]), data[off : off+length]})
	}
	for tag, body := range tables {
		recs = append(recs, rec{tag, body})
	}
	slices.SortFunc(recs, func(a, b rec) int {
		if a.tag < b.tag {
			return -1
		}
		return 1
	})
	out := binary.BigEndian.AppendUint32(nil, binary.BigEndian.Uint32(data))
	out = binary.BigEndian.AppendUint16(out, uint16(len(recs)))
	out = append(out, 0, 0, 0, 0, 0, 0)
	off := 12 + 16*len(recs)
	var bodies []byte
	for _, r := range recs {
		out = append(out, r.tag...)
		out = binary.BigEndian.AppendUint32(out, 0)
		out = binary.BigEndian.AppendUint32(out, uint32(off+len(bodies)))
		out = binary.BigEndian.AppendUint32(out, uint32(len(r.body)))
		bodies = append(bodies, r.body...)
		for len(bodies)%4 != 0 {
			bodies = append(bodies, 0)
		}
	}
	return append(out, bodies...)
}

// bitmapStrike returns the tables EBLC and EBDT of one strike of 12 pixels
// to the em, with a bitmap of 8 by 8 pixels for the glyph gid, as the
// strikes of Calibri and Cambria hold.
func bitmapStrike(gid uint16) (eblc, ebdt []byte) {
	ebdt = binary.BigEndian.AppendUint32(nil, 0x00020000)
	// SmallGlyphMetrics: height, width, bearingX, bearingY, advance, then
	// the rows of the bitmap, one byte each.
	ebdt = append(ebdt, 8, 8, 0, 8, 8)
	ebdt = append(ebdt, 0xff, 0x81, 0x81, 0x81, 0x81, 0x81, 0x81, 0xff)
	lineMetrics := []byte{10, 254, 8, 0, 0, 0, 0, 0, 0, 0, 0, 0}
	eblc = binary.BigEndian.AppendUint32(nil, 0x00020000)
	eblc = binary.BigEndian.AppendUint32(eblc, 1)
	// BitmapSize: the subtable array follows it, at 8+48.
	eblc = binary.BigEndian.AppendUint32(eblc, 8+48)
	eblc = binary.BigEndian.AppendUint32(eblc, 8+20)
	eblc = binary.BigEndian.AppendUint32(eblc, 1)
	eblc = binary.BigEndian.AppendUint32(eblc, 0)
	eblc = append(eblc, lineMetrics...)
	eblc = append(eblc, lineMetrics...)
	eblc = binary.BigEndian.AppendUint16(eblc, gid)
	eblc = binary.BigEndian.AppendUint16(eblc, gid)
	eblc = append(eblc, 12, 12, 1, 1)
	// IndexSubTableArray, then IndexSubTable of format 1 with the image
	// format 1: the glyph data starts at 4 in EBDT.
	eblc = binary.BigEndian.AppendUint16(eblc, gid)
	eblc = binary.BigEndian.AppendUint16(eblc, gid)
	eblc = binary.BigEndian.AppendUint32(eblc, 8)
	eblc = binary.BigEndian.AppendUint16(eblc, 1)
	eblc = binary.BigEndian.AppendUint16(eblc, 1)
	eblc = binary.BigEndian.AppendUint32(eblc, 4)
	eblc = binary.BigEndian.AppendUint32(eblc, 0)
	eblc = binary.BigEndian.AppendUint32(eblc, 13)
	return eblc, ebdt
}

// TestOutlineOfBitmapGlyph checks that a glyph with an embedded bitmap, as
// in the strikes of Calibri, Cambria and Lucida Console, has its outline.
func TestOutlineOfBitmapGlyph(t *testing.T) {
	plain, err := Load(goregular.TTF, 0)
	if err != nil {
		t.Fatal(err)
	}
	gid, ok := plain.Glyph('A')
	if !ok {
		t.Fatal("no glyph for A")
	}
	eblc, ebdt := bitmapStrike(uint16(gid))
	f, err := Load(withTables(goregular.TTF, map[string][]byte{"EBLC": eblc, "EBDT": ebdt}), 0)
	if err != nil {
		t.Fatal(err)
	}
	// The test needs a font whose GlyphData gives the bitmap of the glyph.
	if _, ok := f.face.GlyphData(opentype.GID(gid)).(gofont.GlyphBitmap); !ok {
		t.Fatalf("expected the bitmap of the glyph, got %T", f.face.GlyphData(opentype.GID(gid)))
	}
	p := f.Outline(gid)
	if p == nil {
		t.Fatal("expected the outline of a glyph with a bitmap")
	}
	if p.Bounds() != plain.Outline(gid).Bounds() {
		t.Errorf("expected the outline of glyf, %v, got %v", plain.Outline(gid).Bounds(), p.Bounds())
	}
}

// TestNoHorizontalMetrics checks that a font whose hhea table gives no
// horizontal metrics does not panic. go-text reads the advance at the index
// -1 of the table hmtx.
func TestNoHorizontalMetrics(t *testing.T) {
	data := slices.Clone(goregular.TTF)
	n := int(binary.BigEndian.Uint16(data[4:]))
	for i := range n {
		r := data[12+16*i:]
		if string(r[:4]) == "hhea" {
			// numberOfHMetrics is the last field, at 34.
			binary.BigEndian.PutUint16(data[binary.BigEndian.Uint32(r[8:])+34:], 0)
		}
	}
	f, err := Load(data, 0)
	if err != nil {
		t.Skipf("the font does not load: %v", err)
	}
	gid, _ := f.Glyph('A')
	if adv := f.Advance(gid); adv != 0 {
		t.Errorf("expected no advance, got %v", adv)
	}
}

// TestBitmap checks the bitmap of a glyph without an outline: the space of
// the Go font, with a bitmap of 8 by 8 pixels at 12 pixels to the em.
func TestBitmap(t *testing.T) {
	plain, err := Load(goregular.TTF, 0)
	if err != nil {
		t.Fatal(err)
	}
	space, _ := plain.Glyph(' ')
	letter, _ := plain.Glyph('A')
	eblc, ebdt := bitmapStrike(uint16(space))
	f, err := Load(withTables(goregular.TTF, map[string][]byte{"EBLC": eblc, "EBDT": ebdt}), 0)
	if err != nil {
		t.Fatal(err)
	}
	if f.Outline(space) != nil {
		t.Fatal("expected no outline for the space")
	}
	b, ok := f.Bitmap(space)
	if !ok {
		t.Fatal("expected the bitmap of the space")
	}
	const em = float32(8) / 12
	if !b.Mask || b.X != 0 || b.Y != -em || b.W != em || b.H != em {
		t.Errorf("expected a mask of %v em above the baseline, got %+v", em, b)
	}
	if got := b.Image.Bounds(); got != image.Rect(0, 0, 8, 8) {
		t.Fatalf("expected 8 by 8 pixels, got %v", got)
	}
	// The bitmap is a frame: a full first row, and the edges of the
	// second row.
	for x, want := range []uint8{0xff, 0, 0, 0, 0, 0, 0, 0xff} {
		if got := b.Image.(*image.Alpha).AlphaAt(x, 1).A; got != want {
			t.Errorf("pixel (%d, 1) = %d, want %d", x, got, want)
		}
	}
	if got := b.Image.(*image.Alpha).AlphaAt(3, 0).A; got != 0xff {
		t.Errorf("pixel (3, 0) = %d, want 255", got)
	}
	if _, ok := f.Bitmap(letter); ok {
		t.Error("expected no bitmap for a glyph without one")
	}
}
