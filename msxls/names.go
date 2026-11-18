package msxls

import (
	"github.com/xo/ivy/internal/sheet"
)

// This file reads the print areas and the print titles of the sheets: the
// built-in names Print_Area and Print_Titles, in the records NAME of the
// globals of a workbook of BIFF8 (MS-XLS 2.4.150), and of BIFF5. Each name
// of a sheet holds a formula of areas of cells.

// The flag fBuiltin of a record NAME, and the characters of the built-in
// names that the package reads (MS-XLS 2.4.150).
const (
	nameBuiltin     = 0x0020
	namePrintArea   = 0x06
	namePrintTitles = 0x07
)

// The tokens of a formula that a print area or print titles use (MS-XLS
// 2.5.198), in each of their classes.
const (
	ptgUnion   = 0x10
	ptgRange   = 0x11
	ptgParen   = 0x15
	ptgArea    = 0x25
	ptgMemFunc = 0x29
	ptgRef3d   = 0x3A
	ptgArea3d  = 0x3B
)

// maxPrintAreas bounds the areas of one name.
const maxPrintAreas = 256

// printNames are the print areas and the print titles of a sheet.
type printNames struct {
	areas []sheet.Range
	rows  *sheet.Span
	cols  *sheet.Span
}

// readName reads a record NAME of BIFF8 or of BIFF5, and keeps the print
// areas and the print titles of a sheet. A name of BIFF5 has no byte of
// flags before its characters, which take one byte each, and its formula
// has the tokens of BIFF5.
func (w *workbook) readName(b []byte) {
	if len(b) < 15 {
		return
	}
	grbit := le.Uint16(b)
	cch := int(b[3])
	cce := int(le.Uint16(b[4:]))
	itab := int(le.Uint16(b[8:]))
	if grbit&nameBuiltin == 0 || itab == 0 || cch != 1 {
		return
	}
	// the name: a byte of flags in BIFF8, and one character of 1 or 2
	// bytes
	at, size := 14, 1
	if w.version >= biff8 {
		if len(b) < 16 {
			return
		}
		if b[14]&1 != 0 {
			size = 2
		}
		at = 15
	}
	if at+size+cce > len(b) {
		return
	}
	kind := b[at]
	if kind != namePrintArea && kind != namePrintTitles {
		return
	}
	rgce := b[at+size : at+size+cce]
	areas := formulaAreas(rgce)
	if w.version < biff8 {
		areas = formulaAreas5(rgce)
	}
	if len(areas) == 0 {
		return
	}
	if w.prints == nil {
		w.prints = make(map[int]*printNames)
	}
	p := w.prints[itab-1]
	if p == nil {
		if len(w.prints) >= 4096 {
			return
		}
		p = &printNames{}
		w.prints[itab-1] = p
	}
	if kind == namePrintArea {
		p.areas = areas
		return
	}
	// the print titles: whole rows, whole columns, or both
	for _, a := range areas {
		switch {
		case a.Col0 == 0 && a.Col1 >= maxCol:
			p.rows = &sheet.Span{First: a.Row0, Last: a.Row1}
		case a.Row0 == 0 && a.Row1 >= maxRow:
			p.cols = &sheet.Span{First: a.Col0, Last: a.Col1}
		}
	}
}

// formulaAreas returns the areas of the formula rgce of a name: areas and
// references of cells, joined by unions, in a PtgMemFunc or not. It
// returns nil for a formula of another kind.
func formulaAreas(rgce []byte) []sheet.Range {
	var out []sheet.Range
	add := func(r0, r1, c0, c1 int) bool {
		c0, c1 = c0&0x3FFF, c1&0x3FFF
		if r0 > r1 {
			r0, r1 = r1, r0
		}
		if c0 > c1 {
			c0, c1 = c1, c0
		}
		if r1 > maxRow || c0 > maxCol || len(out) >= maxPrintAreas {
			return false
		}
		out = append(out, sheet.Range{Row0: r0, Col0: c0, Row1: r1, Col1: min(c1, maxCol)})
		return true
	}
	for i := 0; i < len(rgce); {
		ptg := rgce[i]
		// the class of a token is in its bits 5 and 6
		base := ptg
		if ptg >= 0x20 && ptg < 0x80 {
			base = ptg&0x1F | 0x20
		}
		switch {
		case ptg == ptgUnion, ptg == ptgRange, ptg == ptgParen:
			i++
		case base == ptgMemFunc:
			i += 3
		case base == ptgArea3d && i+11 <= len(rgce):
			v := rgce[i+3:]
			if !add(int(le.Uint16(v)), int(le.Uint16(v[2:])), int(le.Uint16(v[4:])), int(le.Uint16(v[6:]))) {
				return nil
			}
			i += 11
		case base == ptgRef3d && i+7 <= len(rgce):
			v := rgce[i+3:]
			r, c := int(le.Uint16(v)), int(le.Uint16(v[2:]))
			if !add(r, r, c, c) {
				return nil
			}
			i += 7
		case base == ptgArea && i+9 <= len(rgce):
			v := rgce[i+1:]
			if !add(int(le.Uint16(v)), int(le.Uint16(v[2:])), int(le.Uint16(v[4:])), int(le.Uint16(v[6:]))) {
				return nil
			}
			i += 9
		default:
			return nil
		}
	}
	return out
}

// formulaAreas5 returns the areas of the formula rgce of a name of BIFF5,
// as formulaAreas does for BIFF8. A row of BIFF5 holds the flags of a
// relative reference in its two high bits, and a column takes one byte. A
// 3D token names its sheets by itabFirst and itabLast after 10 bytes. Row
// 16384, the last row of BIFF5, is the last row of a sheet.
func formulaAreas5(rgce []byte) []sheet.Range {
	var out []sheet.Range
	add := func(r0, r1, c0, c1 int) bool {
		r0, r1 = r0&0x3FFF, r1&0x3FFF
		if r1 == 0x3FFF {
			// the last row of BIFF5 stands for every row
			r1 = maxRow
		}
		if r0 > r1 {
			r0, r1 = r1, r0
		}
		if c0 > c1 {
			c0, c1 = c1, c0
		}
		if r1 > maxRow || c1 > maxCol || len(out) >= maxPrintAreas {
			return false
		}
		out = append(out, sheet.Range{Row0: r0, Col0: c0, Row1: r1, Col1: c1})
		return true
	}
	for i := 0; i < len(rgce); {
		ptg := rgce[i]
		base := ptg
		if ptg >= 0x20 && ptg < 0x80 {
			base = ptg&0x1F | 0x20
		}
		switch {
		case ptg == ptgUnion, ptg == ptgRange, ptg == ptgParen:
			i++
		case base == ptgMemFunc:
			i += 3
		case base == ptgArea3d && i+21 <= len(rgce):
			v := rgce[i+15:]
			if !add(int(le.Uint16(v)), int(le.Uint16(v[2:])), int(v[4]), int(v[5])) {
				return nil
			}
			i += 21
		case base == ptgRef3d && i+18 <= len(rgce):
			v := rgce[i+15:]
			r := int(le.Uint16(v))
			if !add(r, r, int(v[2]), int(v[2])) {
				return nil
			}
			i += 18
		case base == ptgArea && i+7 <= len(rgce):
			v := rgce[i+1:]
			if !add(int(le.Uint16(v)), int(le.Uint16(v[2:])), int(v[4]), int(v[5])) {
				return nil
			}
			i += 7
		default:
			return nil
		}
	}
	return out
}
