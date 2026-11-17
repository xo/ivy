// This file is not a port. It tests the drawing of the bitmap glyphs of a
// font, such as the color emojis of CBDT and sbix, which upstream draws
// with Cairo.

package layout

import (
	"encoding/base64"
	"encoding/binary"
	"slices"
	"testing"

	"golang.org/x/image/font/gofont/goregular"

	ivyfont "github.com/xo/ivy/internal/font"
)

// withTables returns the TrueType font data with the tables added to its
// table directory, in the order of their tags. It does not compute the
// checksums, which go-text does not read.
func fontWithTables(data []byte, tables map[string][]byte) []byte {
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
func fontBitmapStrike(gid uint16) (eblc, ebdt []byte) {
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

func TestBitmapGlyph(t *testing.T) {
	// The space of the Go font gets a bitmap of 8 by 8 pixels at 12
	// pixels to the em: a frame, which the text of 12px draws with its
	// color, one pixel of the bitmap for a pixel of the page.
	plain, err := ivyfont.Load(goregular.TTF, 0)
	if err != nil {
		t.Fatal(err)
	}
	space, _ := plain.Glyph(' ')
	eblc, ebdt := fontBitmapStrike(uint16(space))
	redPixels := func(font []byte) int {
		data := base64.StdEncoding.EncodeToString(font)
		_, _, pixels := htmlToPixels(t, `
      <style>
        @font-face { src: url(data:font/ttf;base64,`+data+`); font-family: bitmap }
        @page { size: 40px 20px }
        body { margin: 0; color: red; font: 12px/20px bitmap; white-space: pre }
      </style>. .`)
		n := 0
		for _, p := range pixels {
			if p[0] > 200 && p[1] < 100 {
				n++
			}
		}
		return n
	}
	red := redPixels(fontWithTables(goregular.TTF, map[string][]byte{"EBLC": eblc, "EBDT": ebdt})) - redPixels(goregular.TTF)
	// The frame has 28 pixels.
	if red < 24 || red > 32 {
		t.Errorf("expected the 28 pixels of the frame, got %d", red)
	}
}
