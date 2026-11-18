package msxls

import (
	"bytes"
	"encoding/binary"
	"image/color"
	"testing"

	"github.com/xo/ivy/internal/fixture"
	"github.com/xo/ivy/internal/sheet"
)

// sheetWorkbook returns a workbook of one sheet with the numbers 1 to 5 in
// A1:A5 and the records recs after them.
func sheetWorkbook(recs ...[]byte) []byte {
	var b biff
	b.record(recBOF, u16(biff8, bofGlobals, 0, 0), u32(0, 0))
	b.record(recFont, u16(200, 0, 0x7FFF, 400, 0), []byte{0, 0, 0, 0}, []byte{5, 0}, []byte("Arial"))
	b.record(recXF, u16(0, 0), make([]byte, 16))
	pos := b.record(recBoundSheet, u32(0), []byte{0, 0}, []byte{5, 0}, []byte("Conds"))
	b.record(recEOF)
	binary.LittleEndian.PutUint32(b.buf[pos+4:], uint32(len(b.buf)))
	b.record(recBOF, u16(biff8, bofWorksheet, 0, 0), u32(0, 0))
	for i := range 5 {
		b.record(recNumber, u16(i, 0, 0), float(float64(i+1)))
	}
	for _, r := range recs {
		b.record(le.Uint16(r), r[2:])
	}
	b.record(recEOF)
	return fixture.CFB(fixture.CFBEntry{Name: "Workbook", Data: b.buf})
}

// biffRec returns a record of the type typ for sheetWorkbook.
func biffRec(typ int, data ...[]byte) []byte {
	return append(u16(typ), bytes.Join(data, nil)...)
}

// condFmtRec returns a record CONDFMT of the id over A1:A5, for n rules.
func condFmtRec(n, id int) []byte {
	return biffRec(recCondFmt, u16(n, id<<1, 0, 4, 0, 0, 1, 0, 4, 0, 0))
}

// frtHeader is the FrtRefHeader of a record of Excel 2007.
func frtHeader(typ int) []byte {
	return append(u16(typ, 0), make([]byte, 8)...)
}

// condWorkbook returns a workbook of one sheet with a rule of Excel 97,
// cellIs greater than 3 with a red fill, and a data bar and an icon set of
// Excel 2007.
func condWorkbook() []byte {
	// a DXFN with a solid fill of the color 10 of the palette in back, red
	dxfn := append(u32(0xA03BFFFF&^(1<<16|1<<18)|1<<29), u16(0x8000)...)
	dxfn = append(dxfn, u16(1<<10, 10<<7|64)...)
	// a rule of a cell value greater than 3: PtgInt
	cf := biffRec(recCF, []byte{ctCellValue, 5}, u16(3, 0), dxfn, []byte{ptgInt}, u16(3))
	// a data bar of Excel 2007 from the lowest to the highest value, in RGB,
	// that hides the values
	bar := append(u16(0), 0, 0x02, 10, 90)
	bar = append(bar, u32(2)...)
	bar = append(bar, 0x11, 0x22, 0x33, 0xFF)
	bar = append(bar, float(0)...)
	bar = append(bar, 2, 0, 0, 3, 0, 0)
	cf12 := func(ct, template int, ext []byte) []byte {
		b := append(frtHeader(recCF12), byte(ct), 0)
		b = append(b, u16(0, 0)...)
		b = append(b, u32(0)...)
		b = append(b, u16(0, 0)...)
		b = append(b, 0)
		b = append(b, u16(2, template)...)
		b = append(b, 16)
		b = append(b, make([]byte, 16)...)
		return biffRec(recCF12, b, ext)
	}
	// an icon set of 3 arrows over percents, with fIconOnly
	icons := append(u16(0), 0, 3, 0, 0x01)
	for _, v := range []float64{0, 33, 67} {
		icons = append(icons, 4, 0, 0)
		icons = append(icons, float(v)...)
		icons = append(icons, 1, 0, 0, 0, 0)
	}
	condFmt12 := biffRec(recCondFmt12, frtHeader(recCondFmt12), u16(2, 2<<1, 0, 4, 0, 0, 1, 0, 4, 0, 0))
	return sheetWorkbook(
		condFmtRec(1, 1), cf,
		condFmt12, cf12(ctDataBar, 3, bar), cf12(ctIconSet, 4, icons),
	)
}

func TestCondFormats(t *testing.T) {
	book := bookOf(t, condWorkbook())
	sh := book.Sheets[0]
	if len(sh.CondFormats) != 2 {
		t.Fatalf("expected 2 conditional formats, got: %d", len(sh.CondFormats))
	}
	if r := sh.Rows[0].Cells[0]; r.Kind != sheet.KindNumber || r.Value != 1 {
		t.Errorf("expected the value 1 in A1, got: %+v", r)
	}
	f := sh.CondFormats[0]
	if exp := (sheet.Range{Row0: 0, Col0: 0, Row1: 4, Col1: 0}); len(f.Ranges) != 1 || f.Ranges[0] != exp {
		t.Errorf("expected the range %+v, got: %+v", exp, f.Ranges)
	}
	if len(f.Rules) != 1 {
		t.Fatalf("expected 1 rule, got: %d", len(f.Rules))
	}
	r := f.Rules[0]
	if r.Kind != "cellIs" || r.Operator != "greaterThan" || len(r.Operands) != 1 || r.Operands[0].Number != 3 {
		t.Errorf("expected cellIs greaterThan 3, got: %+v", r)
	}
	if r.Diff == nil || r.Diff.Fill != (color.NRGBA{255, 0, 0, 255}) {
		t.Errorf("expected a red fill, got: %+v", r.Diff)
	}
	rules := sh.CondFormats[1].Rules
	if len(rules) != 2 {
		t.Fatalf("expected 2 rules of Excel 2007, got: %d", len(rules))
	}
	if b := rules[0].Bar; rules[0].Kind != "dataBar" || b == nil || b.Min.Kind != "min" || b.Max.Kind != "max" ||
		b.Color != (color.NRGBA{0x11, 0x22, 0x33, 255}) || !b.HideValue || rules[0].Priority != 2 {
		t.Errorf("expected a data bar, got: %+v %+v", rules[0], b)
	}
	if is := rules[1].Icons; rules[1].Kind != "iconSet" || is == nil || is.Name != "3Arrows" || !is.HideValue ||
		len(is.Thresholds) != 3 || is.Thresholds[2] != (sheet.Threshold{Kind: "percent", Value: 67, GTE: true}) {
		t.Errorf("expected an icon set, got: %+v %+v", rules[1], is)
	}
}

func TestOperand(t *testing.T) {
	for _, test := range []struct {
		rgce []byte
		exp  sheet.Operand
		ok   bool
	}{
		{[]byte{ptgInt, 7, 0}, sheet.Operand{Number: 7}, true},
		{append(append([]byte{ptgNum}, float(2.5)...), ptgUminus), sheet.Operand{Number: -2.5}, true},
		{[]byte{ptgStr, 2, 0, 'h', 'i'}, sheet.Operand{Text: "hi", IsText: true}, true},
		{[]byte{ptgStr, 1, 1, 'z', 0}, sheet.Operand{Text: "z", IsText: true}, true},
		// a reference that moves with the cell needs a formula
		{[]byte{ptgRef, 0, 0, 0, 0xC0}, sheet.Operand{}, false},
		{[]byte{ptgInt, 1}, sheet.Operand{}, false},
		{nil, sheet.Operand{}, false},
	} {
		o, ok := operand(nil, test.rgce)
		if ok != test.ok || (ok && o != test.exp) {
			t.Errorf("operand(%x): expected %+v %v, got: %+v %v", test.rgce, test.exp, test.ok, o, ok)
		}
	}
}
