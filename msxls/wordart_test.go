package msxls

import (
	"bytes"
	"encoding/binary"
	"image/color"
	"testing"

	"github.com/xo/ivy"
	"github.com/xo/ivy/internal/fixture"
	"github.com/xo/ivy/internal/odraw"
)

// wordArtWorkbook returns a workbook of a worksheet with a red shape of
// WordArt with the text "WordArt", from the cell B2 to the cell F11.
func wordArtWorkbook() []byte {
	var text []byte
	for _, r := range "WordArt" {
		text = binary.LittleEndian.AppendUint16(text, uint16(r))
	}
	// the fill color, then the complex property of the text, whose data
	// follows the table of the properties
	props := append(u16(odraw.PropFillColor), u32(testRed)...)
	props = append(props, u16(odraw.PropGtextUnicode|0x8000)...)
	props = append(props, u32(len(text))...)
	opt := rec(3, 2, odraw.RecFOPT, props, text)
	shape := rec(0xF, 0, odraw.RecSpContainer,
		rec(2, odraw.SptTextEffectMin, odraw.RecFSP, u32(1025, 0xA00)),
		opt,
		rec(0, 0, odraw.RecClientAnchor, u16(2, 1, 0, 1, 0, 5, 0, 10, 0)),
		rec(0, 0, odraw.RecClientData),
	)
	var b biff
	b.record(recBOF, u16(biff8, bofGlobals, 0, 0), u32(0, 0))
	b.record(recFont, u16(200, 0, 0x7FFF, 400, 0), []byte{0, 0, 0, 0}, []byte{5, 0}, []byte("Arial"))
	b.record(recXF, u16(0, 0), make([]byte, 16))
	b.record(recMsoDrawingGroup, rec(0xF, 0, odraw.RecDggContainer))
	sheet := b.record(recBoundSheet, u32(0), []byte{0, 0}, []byte{4, 0}, []byte("Word"))
	b.record(recEOF)
	binary.LittleEndian.PutUint32(b.buf[sheet+4:], uint32(len(b.buf)))
	b.record(recBOF, u16(biff8, bofWorksheet, 0, 0), u32(0, 0))
	b.record(recNumber, u16(0, 0, 0), float(1))
	all := bytes.Join([][]byte{patriarch(), shape}, nil)
	b.record(recMsoDrawing, rec(0xF, 0, odraw.RecDgContainer, rec(0xF, 0, odraw.RecSpgrContainer, all)))
	// a rectangle, of the type 2 of the record OBJ
	b.record(recObj, obj(2))
	b.record(recEOF)
	return fixture.CFB(fixture.CFBEntry{Name: "Workbook", Data: b.buf})
}

// TestWordArt checks that a shape of WordArt draws the outlines of its
// text with its fill, and not the rectangle of its type.
func TestWordArt(t *testing.T) {
	d, err := Open(t.Context(), bytes.NewReader(wordArtWorkbook()))
	if err != nil {
		t.Fatal(err)
	}
	p, err := d.Page(t.Context(), 0)
	if err != nil {
		t.Fatal(err)
	}
	img, err := ivy.RenderImage(t.Context(), p, 72, color.White)
	if err != nil {
		t.Fatal(err)
	}
	// the box of the shape from B2 to F11, inside the margins of 50.4 and
	// 54 points, with the columns of 50.47 points and the rows of 13.07
	// points
	red, all := 0, 0
	col, row := 50.47, 13.07
	x0, x1 := int(50.4+col)+1, int(50.4+5*col)-1
	y0, y1 := int(54+row)+1, int(54+10*row)-1
	for y := y0; y < y1; y++ {
		for x := x0; x < x1; x++ {
			all++
			if c := color.NRGBAModel.Convert(img.At(x, y)).(color.NRGBA); c.R > 200 && c.G < 60 && c.B < 60 {
				red++
			}
		}
	}
	if red < all/10 || red > all*3/4 {
		t.Errorf("expected the red glyphs to cover a part of the box, got %d of %d pixels", red, all)
	}
}
