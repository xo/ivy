package msxls

import (
	"bytes"
	"encoding/binary"
	"testing"

	"github.com/xo/ivy"
	"github.com/xo/ivy/internal/fixture"
	"github.com/xo/ivy/internal/sheet"
)

// printWorkbook returns a workbook of two sheets with a number each, with
// the records names in its globals and the records hf in its first sheet.
func printWorkbook(names [][]byte, hf ...[]byte) []byte {
	var b biff
	b.record(recBOF, u16(biff8, bofGlobals, 0, 0), u32(0, 0))
	b.record(recFont, u16(200, 0, 0x7FFF, 400, 0), []byte{0, 0, 0, 0}, []byte{5, 0}, []byte("Arial"))
	b.record(recXF, u16(0, 0), make([]byte, 16))
	pos1 := b.record(recBoundSheet, u32(0), []byte{0, 0}, []byte{3, 0}, []byte("One"))
	pos2 := b.record(recBoundSheet, u32(0), []byte{0, 0}, []byte{3, 0}, []byte("Two"))
	for _, n := range names {
		b.record(recName, n)
	}
	b.record(recEOF)
	for i, pos := range []int{pos1, pos2} {
		binary.LittleEndian.PutUint32(b.buf[pos+4:], uint32(len(b.buf)))
		b.record(recBOF, u16(biff8, bofWorksheet, 0, 0), u32(0, 0))
		b.record(recNumber, u16(0, 0, 0), float(1))
		if i == 0 {
			for _, r := range hf {
				b.record(recHeaderFooter, r)
			}
		}
		b.record(recEOF)
	}
	return fixture.CFB(fixture.CFBEntry{Name: "Workbook", Data: b.buf})
}

// builtinName returns a record NAME of the built-in name kind of the sheet
// itab, from 1, with the formula rgce.
func builtinName(kind byte, itab int, rgce []byte) []byte {
	b := u16(nameBuiltin)
	b = append(b, 0, 1)
	b = append(b, u16(len(rgce), 0, itab)...)
	b = append(b, 0, 0, 0, 0, 0, kind)
	return append(b, rgce...)
}

// area3d returns a PtgArea3d of the class ref.
func area3d(r0, r1, c0, c1 int) []byte {
	return append([]byte{ptgArea3d}, u16(0, r0, r1, c0|0xC000, c1|0xC000)...)
}

// bookOf returns the model of the sheets of the workbook buf.
func bookOf(t *testing.T, buf []byte) *sheet.Workbook {
	t.Helper()
	d, err := Open(t.Context(), bytes.NewReader(buf))
	if err != nil {
		t.Fatal(err)
	}
	book, err := d.wb.book(t.Context(), "", d.cfg)
	if err != nil {
		t.Fatal(err)
	}
	return book
}

// TestPrintNames checks the print area and the print titles of the second
// sheet: two areas in a PtgMemFunc with a union, the rows 1 to 2 and the
// column A.
func TestPrintNames(t *testing.T) {
	areas := append(append(area3d(2, 9, 1, 3), area3d(20, 29, 0, 0)...), ptgUnion)
	memFunc := append(append([]byte{0x49}, u16(len(areas))...), areas...)
	titles := append(append(area3d(0, 1, 0, 255), area3d(0, 65535, 0, 0)...), ptgUnion)
	book := bookOf(t, printWorkbook([][]byte{
		builtinName(namePrintArea, 2, memFunc),
		builtinName(namePrintTitles, 2, append(append([]byte{0x29}, u16(len(titles))...), titles...)),
		// a name of a formula that is not an area is left out
		builtinName(namePrintArea, 1, []byte{0x1E, 1, 0}),
	}))
	one, two := book.Sheets[0], book.Sheets[1]
	if one.PrintAreas != nil || one.TitleRows != nil {
		t.Errorf("expected no print area on the first sheet, got: %+v", one.PrintAreas)
	}
	want := []sheet.Range{{Row0: 2, Col0: 1, Row1: 9, Col1: 3}, {Row0: 20, Col0: 0, Row1: 29, Col1: 0}}
	if len(two.PrintAreas) != 2 || two.PrintAreas[0] != want[0] || two.PrintAreas[1] != want[1] {
		t.Errorf("expected the areas %+v, got: %+v", want, two.PrintAreas)
	}
	if two.TitleRows == nil || *two.TitleRows != (sheet.Span{First: 0, Last: 1}) {
		t.Errorf("expected the title rows 0 to 1, got: %+v", two.TitleRows)
	}
	if two.TitleCols == nil || *two.TitleCols != (sheet.Span{First: 0, Last: 0}) {
		t.Errorf("expected the title column 0, got: %+v", two.TitleCols)
	}
}

// TestHeaderFooterScale checks the flag fHFScaleWithDoc of the record
// HeaderFooter: without the record, and with the flag, the header and the
// footer scale with the cells. A record of a custom view does not count.
func TestHeaderFooterScale(t *testing.T) {
	record := func(view byte, flags int) []byte {
		b := append(u16(recHeaderFooter, 0), make([]byte, 8)...)
		guid := make([]byte, 16)
		guid[0] = view
		b = append(b, guid...)
		return append(b, u16(flags, 0, 0, 0, 0)...)
	}
	for _, tt := range []struct {
		name string
		hf   [][]byte
		want bool
	}{
		{"no record", nil, true},
		{"scale", [][]byte{record(0, hfScaleWithDoc)}, true},
		{"fixed", [][]byte{record(0, 0)}, false},
		{"custom view", [][]byte{record(1, 0)}, true},
	} {
		book := bookOf(t, printWorkbook(nil, tt.hf...))
		if got := book.Sheets[0].Page.ScaleHeaderFooter; got != tt.want {
			t.Errorf("%s: expected %v, got: %v", tt.name, tt.want, got)
		}
	}
}

// TestPrintAreaPages checks that the print area limits the cells on the
// pages.
func TestPrintAreaPages(t *testing.T) {
	d, err := Open(t.Context(), bytes.NewReader(printWorkbook([][]byte{builtinName(namePrintArea, 1, area3d(5, 5, 5, 5))})), WithConfig(ivy.Config{}))
	if err != nil {
		t.Fatal(err)
	}
	doc := sheets(t, d)[0]
	if bytes.Contains([]byte(doc), []byte(">1</td>")) {
		t.Errorf("expected the cell A1 out of the print area:\n%s", doc)
	}
}

// TestNameBIFF5 checks the records NAME of BIFF5: the print titles of the
// rows 1 to 2 and of the column A, in the 3D areas of BIFF5, and a print
// area.
func TestNameBIFF5(t *testing.T) {
	area5 := func(r0, r1, c0, c1 int) []byte {
		b := append([]byte{ptgArea3d}, make([]byte, 10)...)
		b = append(b, u16(0, 0, r0|0xC000, r1|0xC000)...)
		return append(b, byte(c0), byte(c1))
	}
	name5 := func(kind byte, itab int, rgce []byte) []byte {
		b := u16(nameBuiltin)
		b = append(b, 0, 1)
		b = append(b, u16(len(rgce), 0, itab)...)
		b = append(b, 0, 0, 0, 0, kind)
		return append(b, rgce...)
	}
	w := &workbook{version: biff5}
	titles := append(append(area5(0, 1, 0, 255), area5(0, 0x3FFF, 0, 0)...), ptgUnion)
	w.readName(name5(namePrintTitles, 2, titles))
	w.readName(name5(namePrintArea, 2, area5(2, 9, 1, 3)))
	p := w.prints[1]
	if p == nil {
		t.Fatal("expected the print names of the second sheet")
	}
	if p.rows == nil || *p.rows != (sheet.Span{First: 0, Last: 1}) {
		t.Errorf("expected the title rows 0 to 1, got: %+v", p.rows)
	}
	if p.cols == nil || *p.cols != (sheet.Span{First: 0, Last: 0}) {
		t.Errorf("expected the title column 0, got: %+v", p.cols)
	}
	if want := (sheet.Range{Row0: 2, Col0: 1, Row1: 9, Col1: 3}); len(p.areas) != 1 || p.areas[0] != want {
		t.Errorf("expected the area %+v, got: %+v", want, p.areas)
	}
}

// TestDefaultMargins checks the margins of a sheet without the records of
// the margins, the margins of the old versions of Excel, and of a sheet
// with a record of the left margin.
func TestDefaultMargins(t *testing.T) {
	for _, test := range []struct {
		name string
		recs [][]byte
		exp  [4]float64
	}{
		{"none", nil, [4]float64{72, 54, 72, 54}},
		{"left", [][]byte{float(0.25)}, [4]float64{72, 54, 72, 18}},
	} {
		t.Run(test.name, func(t *testing.T) {
			var b biff
			b.record(recBOF, u16(biff8, bofGlobals, 0, 0), u32(0, 0))
			b.record(recFont, u16(200, 0, 0x7FFF, 400, 0), []byte{0, 0, 0, 0}, []byte{5, 0}, []byte("Arial"))
			b.record(recXF, u16(0, 0), make([]byte, 16))
			pos := b.record(recBoundSheet, u32(0), []byte{0, 0}, []byte{6, 0}, []byte("Margin"))
			b.record(recEOF)
			binary.LittleEndian.PutUint32(b.buf[pos+4:], uint32(len(b.buf)))
			b.record(recBOF, u16(biff8, bofWorksheet, 0, 0), u32(0, 0))
			b.record(recNumber, u16(0, 0, 0), float(1))
			for _, r := range test.recs {
				b.record(recLeftMargin, r)
			}
			b.record(recEOF)
			book := bookOf(t, fixture.CFB(fixture.CFBEntry{Name: "Workbook", Data: b.buf}))
			p := book.Sheets[0].Page
			if got := [4]float64{p.Top, p.Right, p.Bottom, p.Left}; got != test.exp {
				t.Errorf("expected the margins %v, got: %v", test.exp, got)
			}
		})
	}
}
