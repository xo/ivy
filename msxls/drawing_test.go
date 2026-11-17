package msxls

import (
	"bytes"
	"encoding/binary"
	"image/color"
	"regexp"
	"strings"
	"testing"

	"github.com/xo/ivy"
	"github.com/xo/ivy/internal/drawingml"
	"github.com/xo/ivy/internal/fixture"
	"github.com/xo/ivy/internal/odraw"
)

// rec returns a record of Office Drawing: a container when ver is 0xF.
func rec(ver, inst, typ int, data ...[]byte) []byte {
	body := bytes.Join(data, nil)
	out := u16(ver|inst<<4, typ)
	out = binary.LittleEndian.AppendUint32(out, uint32(len(body)))
	return append(out, body...)
}

// fopt returns an OfficeArtFOPT with the properties, by id and value.
func fopt(props ...int) []byte {
	var b []byte
	for i := 0; i+1 < len(props); i += 2 {
		b = append(b, u16(props[i])...)
		b = append(b, u32(props[i+1])...)
	}
	return rec(3, len(props)/2, odraw.RecFOPT, b)
}

// anchorShape returns the container of a shape of the type typ with a cell
// anchor from (col0, row0) to (col1, row1), and the properties.
func anchorShape(typ, col0, row0, col1, row1 int, props ...int) []byte {
	return rec(0xF, 0, odraw.RecSpContainer,
		rec(2, typ, odraw.RecFSP, u32(1025, 0xA00)),
		fopt(props...),
		rec(0, 0, odraw.RecClientAnchor, u16(2, col0, 0, row0, 0, col1, 0, row1, 0)),
		rec(0, 0, odraw.RecClientData),
	)
}

// patriarch returns the container of the shape of the group of a drawing.
func patriarch() []byte {
	return rec(0xF, 0, odraw.RecSpContainer,
		rec(1, 0, odraw.RecFSPGR, make([]byte, 16)),
		rec(2, 0, odraw.RecFSP, u32(1024, 5)),
	)
}

// obj returns a record OBJ of the type ot, which prints.
func obj(ot int) []byte {
	return append(u16(0x15, 0x12, ot, 1, cmoPrint), make([]byte, 12)...)
}

// The fill colors of the shapes of the test: red, and the color 10 of the
// palette, green.
const (
	testRed   = 0x0000FF
	testGreen = odraw.ColorSchemeIndex | 11
)

// testWMF returns a WMF file without a placeable header, of a magenta
// rectangle.
func testWMF() []byte {
	w := fixture.NewWMF(100, 50, false)
	w.Select(w.Brush(color.NRGBA{255, 0, 255, 255}))
	w.Select(w.NullPen())
	w.Rectangle(0, 0, 100, 50)
	return w.Bytes()
}

// drawingWorkbook returns a workbook of a worksheet with a picture, a text
// box, a chart, a comment, a red rectangle, a green rectangle that does not
// print, and a group, and of a chart sheet.
func drawingWorkbook(png []byte) []byte {
	var b biff
	b.record(recBOF, u16(biff8, bofGlobals, 0, 0), u32(0, 0))
	b.record(recFont, u16(200, 0, 0x7FFF, 400, 0), []byte{0, 0, 0, 0}, []byte{5, 0}, []byte("Arial"))
	b.record(recXF, u16(0, 0), make([]byte, 16))
	// the store of the pictures: one PNG image, which the record cuts in
	// two, as Excel does for a large group
	blip := rec(0, 0x6E0, odraw.RecBlipPNG, make([]byte, 16), []byte{0xFF}, png)
	fbse := rec(2, 6, odraw.RecFBSE, []byte{6, 6}, make([]byte, 16), u16(0xFF), u32(len(blip), 1, 0), []byte{0, 0, 0, 0}, blip)
	// a WMF file without a placeable header, stored as it is: its
	// OfficeArtMetafileHeader gives the size, the bounds, the size in EMU,
	// the saved size, the compression 0xFE and the filter
	wmf := testWMF()
	mh := append(u32(len(wmf), 0, 0, 100, 50, 1270000, 635000, len(wmf)), 0xFE, 0xFE)
	blip2 := rec(0, 0x216, odraw.RecBlipWMF, make([]byte, 16), mh, wmf)
	fbse2 := rec(2, 3, odraw.RecFBSE, []byte{3, 3}, make([]byte, 16), u16(0xFF), u32(len(blip2), 1, 0), []byte{0, 0, 0, 0}, blip2)
	group := rec(0xF, 0, odraw.RecDggContainer, rec(0xF, 2, odraw.RecBStoreContainer, fbse, fbse2))
	half := len(group) / 2
	b.record(recMsoDrawingGroup, group[:half])
	b.record(recMsoDrawingGroup, group[half:])
	sheet1 := b.record(recBoundSheet, u32(0), []byte{0, 0}, []byte{6, 0}, []byte("Drawn"))
	chart := b.record(recBoundSheet, u32(0), []byte{0, 2}, []byte{5, 0}, []byte("Chart"))
	b.record(recEOF)
	binary.LittleEndian.PutUint32(b.buf[sheet1+4:], uint32(len(b.buf)))
	b.record(recBOF, u16(biff8, bofWorksheet, 0, 0), u32(0, 0))
	b.record(recNumber, u16(0, 0, 0), float(1))
	child := func(typ, l, t, r, bottom, fill int) []byte {
		return rec(0xF, 0, odraw.RecSpContainer,
			rec(2, typ, odraw.RecFSP, u32(1030, 0xA02)),
			fopt(odraw.PropFillColor, fill),
			rec(0, 0, odraw.RecChildAnchor, u32(l, t, r, bottom)),
			rec(0, 0, odraw.RecClientData),
		)
	}
	grp := rec(0xF, 0, odraw.RecSpgrContainer,
		rec(0xF, 0, odraw.RecSpContainer,
			rec(1, 0, odraw.RecFSPGR, u32(0, 0, 100, 100)),
			rec(2, 0, odraw.RecFSP, u32(1029, 0x201)),
			rec(0, 0, odraw.RecClientAnchor, u16(2, 7, 0, 4, 0, 9, 0, 8, 0)),
			rec(0, 0, odraw.RecClientData),
		),
		child(1, 0, 0, 50, 100, testRed),
		child(3, 50, 0, 100, 100, testRed),
	)
	shapes := [][]byte{
		anchorShape(75, 1, 1, 3, 5, odraw.PropPib, 1),
		anchorShape(202, 4, 1, 6, 3),
		anchorShape(1, 1, 6, 5, 12),
		anchorShape(1, 7, 0, 8, 2),
		anchorShape(1, 1, 14, 3, 16, odraw.PropFillColor, testRed),
		anchorShape(1, 4, 14, 6, 16, odraw.PropFillColor, testGreen),
		grp,
		anchorShape(75, 7, 10, 9, 13, odraw.PropPib, 2),
	}
	// the container of the drawing goes over all the records
	all := bytes.Join(append([][]byte{patriarch()}, shapes...), nil)
	dg := rec(0xF, 0, odraw.RecDgContainer, rec(0xF, 0, odraw.RecSpgrContainer, all))
	head := len(dg) - len(all) + len(patriarch())
	b.record(recMsoDrawing, dg[:head+len(shapes[0])])
	b.record(recObj, obj(objPicture))
	// the drawing of the next shape in a CONTINUE record after the OBJ
	b.record(recMsoDrawing, shapes[1])
	b.record(recObj, obj(objText))
	// a centered text with a run of the font 0 and the last run
	b.record(recTxo, u16(2<<1|1<<4, 0, 0, 0, 0, 5, 16), make([]byte, 4))
	b.record(recContinue, []byte{0}, []byte("Hello"))
	b.record(recContinue, u16(0, 0, 0, 0, 5, 0, 0, 0))
	b.record(recContinue, shapes[2])
	b.record(recObj, obj(objChart))
	b.record(recBOF, u16(biff8, 0x20, 0, 0), u32(0, 0))
	b.record(recEOF)
	b.record(recMsoDrawing, shapes[3])
	b.record(recObj, obj(objNote))
	b.record(recMsoDrawing, shapes[4])
	b.record(recObj, obj(2))
	b.record(recMsoDrawing, shapes[5])
	b.record(recObj, append(u16(0x15, 0x12, 2, 1, 0), make([]byte, 12)...))
	// the group, then its two children, each with its record OBJ
	cut := len(grp) - len(child(1, 0, 0, 50, 100, testRed))*2
	b.record(recMsoDrawing, grp[:cut])
	b.record(recObj, obj(objGroup))
	b.record(recMsoDrawing, grp[cut:cut+(len(grp)-cut)/2])
	b.record(recObj, obj(2))
	b.record(recMsoDrawing, grp[cut+(len(grp)-cut)/2:])
	b.record(recObj, obj(3))
	b.record(recMsoDrawing, shapes[7])
	b.record(recObj, obj(objPicture))
	b.record(recEOF)
	binary.LittleEndian.PutUint32(b.buf[chart+4:], uint32(len(b.buf)))
	b.record(recBOF, u16(biff8, 0x20, 0, 0), u32(0, 0))
	b.record(recEOF)
	return fixture.CFB(fixture.CFBEntry{Name: "Workbook", Data: b.buf})
}

func TestDrawings(t *testing.T) {
	png := fixture.PNG(4, 2, color.NRGBA{0, 0, 255, 255})
	d, err := Open(t.Context(), bytes.NewReader(drawingWorkbook(png)))
	if err != nil {
		t.Fatal(err)
	}
	src, err := d.source(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if len(src.Documents) != 2 {
		t.Fatalf("expected the worksheet and the chart sheet, got: %q", src.Documents)
	}
	// the text box, the red rectangle, the group and the metafile draw as
	// objects, and the PNG picture as an image; the comment and the shape
	// that does not print do not draw
	if len(src.Objects) != 4 {
		t.Errorf("expected 4 objects, got: %d", len(src.Objects))
	}
	docs := sheets(t, d)
	// the columns of 50.47 points and the rows of 13.07 points
	for _, want := range []string{
		`src="media/image1.png"`,
		`<div class="ph" style="left:0pt;top:0pt;width:201.87pt;height:78.41pt">Chart</div>`,
	} {
		if !strings.Contains(docs[0], want) {
			t.Errorf("expected %q in the sheet:\n%s", want, docs[0])
		}
	}
	if regexp.MustCompile(`class="ph"[^>]*></div>`).MatchString(docs[0]) {
		t.Errorf("expected no frame for the comment:\n%s", docs[0])
	}
	var text *drawingml.Figure
	for _, o := range src.Objects {
		for _, f := range o.(*shapeObject).figures {
			if f.Text != nil {
				text = f
			}
		}
	}
	if text == nil || len(text.Text.Paragraphs) != 1 || text.Text.Paragraphs[0].Runs[0].Text != "Hello" ||
		text.Text.Paragraphs[0].Style.Align.V != "ctr" || text.Text.Paragraphs[0].Runs[0].Style.Latin.V != "Arial" {
		t.Errorf("expected the centered text Hello in Arial, got: %+v", text.Text)
	}
	// the chart sheet fills a page of US Letter in landscape
	if !strings.Contains(docs[1], "size:792pt 612pt") || !strings.Contains(docs[1], ">Chart</div>") {
		t.Errorf("expected the chart sheet in landscape:\n%s", docs[1])
	}
	n, err := d.PageCount(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if n != 2 {
		t.Errorf("expected 2 pages, got: %d", n)
	}
	p, err := d.Page(t.Context(), 0)
	if err != nil {
		t.Fatal(err)
	}
	img, err := ivy.RenderImage(t.Context(), p, 72, color.White)
	if err != nil {
		t.Fatal(err)
	}
	// the page has the margins of 50.4 and 54 points: the picture, the red
	// rectangle and the two halves of the group, and no green
	at := func(col, row float64) color.NRGBA {
		return color.NRGBAModel.Convert(img.At(int(50.4+col*50.47), int(54+row*13.07))).(color.NRGBA)
	}
	for _, tt := range []struct {
		name     string
		col, row float64
		want     color.NRGBA
	}{
		{"picture", 2, 3, color.NRGBA{0, 0, 255, 255}},
		{"red rectangle", 2, 15, color.NRGBA{255, 0, 0, 255}},
		{"no green rectangle", 5, 15, color.NRGBA{255, 255, 255, 255}},
		{"rectangle of the group", 7.5, 6, color.NRGBA{255, 0, 0, 255}},
		{"oval of the group", 8.5, 6, color.NRGBA{255, 0, 0, 255}},
		{"corner outside the oval", 8.95, 4.1, color.NRGBA{255, 255, 255, 255}},
		{"metafile without a placeable header", 8, 11.5, color.NRGBA{255, 0, 255, 255}},
	} {
		if got := at(tt.col, tt.row); got != tt.want {
			t.Errorf("%s: expected %v, got: %v", tt.name, tt.want, got)
		}
	}
}
