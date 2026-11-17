package msdoc

import (
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
)

// This file reads the properties of the rows of the tables, the TAP of
// MS-DOC 2.9.297, from the modifiers of the mark of the end of a row, and
// writes the rows as HTML.

// The sprms of the tables that the package reads.
const (
	sprmTJc90             = 0x5400
	sprmTTableHeader      = 0x3404
	sprmTJc               = 0x548A
	sprmTDxaLeft          = 0x9601
	sprmTDxaGapHalf       = 0x9602
	sprmTDyaRowHeight     = 0x9407
	sprmTTableBorders80   = 0xD605
	sprmTDefTableShd80    = 0xD609
	sprmTDefTableShd3rd   = 0xD60C
	sprmTDefTableShd      = 0xD612
	sprmTTableBorders     = 0xD613
	sprmTDefTableShd2nd   = 0xD616
	sprmTSetBrc80         = 0xD620
	sprmTVertMerge        = 0xD62B
	sprmTVertAlign        = 0xD62C
	sprmTSetBrc           = 0xD62F
	sprmTCellPadding      = 0xD632
	sprmTCellPaddingDflt  = 0xD634
	sprmTDefTableMaxCells = 63
)

// The values of the vertMerge of a TCGRF (MS-DOC 2.9.329): a cell that a
// cell above covers, and the first cell of the cells that merge.
const (
	vertMergeContinue = 1
	vertMergeRestart  = 3
)

// brc is a border: its width in points, its type, and its color as
// 0xRRGGBB. A brc that is not set gives no border of its own.
type brc struct {
	width float64
	typ   byte
	color uint32
	set   bool
}

// none reports whether the border draws nothing.
func (b brc) none() bool {
	return b.typ == 0 || b.typ == 0xFF || b.width <= 0
}

// css returns the border as a value of CSS.
func (b brc) css() string {
	if b.none() {
		return "none"
	}
	style := "solid"
	width := b.width
	switch b.typ {
	case 3, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20, 21:
		style = "double"
		width = max(width*3, 1.5)
	case 6:
		style = "dotted"
	case 7, 8, 9, 22, 23:
		style = "dashed"
	case 5:
		width = max(width, 1.5)
	}
	return fmt.Sprintf("%spt %s #%06x", num(min(width, 6)), style, b.color)
}

// num returns v as a short decimal number.
func num(v float64) string {
	r := math.Round(v*100) / 100
	if r == 0 {
		// no negative zero
		r = 0
	}
	return strconv.FormatFloat(r, 'f', -1, 64)
}

// brc80 reads a Brc80 of 4 bytes (MS-DOC 2.9.17). Four bytes of 0xFF are
// the border brcNil, which is set and draws nothing, and four bytes of 0
// are not set.
func brc80(b []byte) brc {
	if len(b) < 4 {
		return brc{}
	}
	if b[0] == 0xFF && b[1] == 0xFF && b[2] == 0xFF && b[3] == 0xFF {
		return brc{set: true}
	}
	if b[0] == 0 && b[1] == 0 && b[2] == 0 && b[3] == 0 {
		return brc{}
	}
	out := brc{width: float64(b[0]) / 8, typ: b[1], set: true}
	if int(b[2]) < len(icoColors) {
		out.color = icoColors[b[2]]
	}
	return out
}

// brc97 reads a Brc of 8 bytes (MS-DOC 2.9.16): a COLORREF, the width in
// eighths of a point and the type.
func brc97(b []byte) brc {
	if len(b) < 8 {
		return brc{}
	}
	if b[4] == 0xFF && b[5] == 0xFF {
		return brc{set: true}
	}
	out := brc{width: float64(b[4]) / 8, typ: b[5], set: true}
	if b[3] == 0 {
		out.color = uint32(b[0])<<16 | uint32(b[1])<<8 | uint32(b[2])
	}
	return out
}

// shd is a shading: the color of a cell or of a paragraph, as 0xRRGGBB,
// valid when set.
type shd struct {
	color uint32
	set   bool
}

// The parts of the foreground of the patterns of shading, the Ipat of
// MS-DOC 2.9.121, in thousandths. A pattern that is not in the table, such
// as a hatch, takes half.
var ipatShares = map[uint16]int{
	1: 1000, 2: 50, 3: 100, 4: 200, 5: 250, 6: 300, 7: 400, 8: 500, 9: 600,
	10: 700, 11: 750, 12: 800, 13: 900, 35: 25, 36: 75, 37: 125, 38: 150,
	39: 175, 40: 225, 41: 275, 42: 325, 43: 350, 44: 375, 45: 425, 46: 450,
	47: 475, 48: 525, 49: 550, 50: 575, 51: 625, 52: 650, 53: 675, 54: 725,
	55: 775, 56: 825, 57: 850, 58: 875, 59: 925, 60: 950, 61: 975, 62: 970,
}

// mixShading returns the shading of the pattern ipat of the color fore
// over the color back. An automatic foreground is black, and an automatic
// background is no shading.
func mixShading(fore, back uint32, foreAuto, backAuto bool, ipat uint16) shd {
	if ipat == 0 {
		if backAuto {
			return shd{}
		}
		return shd{color: back, set: true}
	}
	if ipat == 0xFFFF {
		return shd{}
	}
	if foreAuto {
		fore = 0
	}
	if backAuto {
		back = 0xFFFFFF
	}
	share, ok := ipatShares[ipat]
	if !ok {
		share = 500
	}
	mix := func(shift uint) uint32 {
		f, b := int(fore>>shift&0xFF), int(back>>shift&0xFF)
		return uint32((f*share + b*(1000-share) + 500) / 1000)
	}
	return shd{color: mix(16)<<16 | mix(8)<<8 | mix(0), set: true}
}

// shd80 reads a SHD80 of 2 bytes (MS-DOC 2.9.249).
func shd80(v uint16) shd {
	fore, back, ipat := v&0x1F, v>>5&0x1F, v>>10
	ico := func(i uint16) uint32 {
		if int(i) < len(icoColors) {
			return icoColors[i]
		}
		return 0
	}
	return mixShading(ico(fore), ico(back), fore == 0, back == 0, ipat)
}

// shd97 reads a SHD of 10 bytes (MS-DOC 2.9.248): two COLORREFs and an
// Ipat.
func shd97(b []byte) shd {
	if len(b) < 10 {
		return shd{}
	}
	color := func(c []byte) (uint32, bool) {
		return uint32(c[0])<<16 | uint32(c[1])<<8 | uint32(c[2]), c[3] == 0xFF
	}
	fore, foreAuto := color(b)
	back, backAuto := color(b[4:])
	return mixShading(fore, back, foreAuto, backAuto, le.Uint16(b[8:]))
}

// tableCell holds the properties of a cell of a row.
type tableCell struct {
	// top, left, bottom and right are the borders of the cell.
	borders   [4]brc
	vertMerge byte
	// vertAlign is 0 for the top, 1 for the middle and 2 for the bottom.
	vertAlign byte
	shading   shd
}

// tap holds the properties of a row of a table.
type tap struct {
	// centers are the edges of the cells, in twips from the left margin,
	// one more than the cells.
	centers []int
	cells   []tableCell
	// borders are the borders of the table: top, left, bottom, right,
	// inside horizontal and inside vertical.
	borders [6]brc
	// gapHalf is half the space between the text of two cells, in twips,
	// which is the padding at the left and the right of a cell, unless
	// padding sets it.
	gapHalf int
	// padding is the padding of the cells at the top, the left, the bottom
	// and the right in twips, where hasPadding is set.
	padding    [4]int
	hasPadding [4]bool
	// rowHeight is the height of the row in twips: at least that height,
	// or exactly when it is negative.
	rowHeight int
	// jc is the alignment of the row: 0 left, 1 center, 2 right.
	jc int
	// header is set for a row that repeats at the top of each page.
	header bool
}

// cellRange returns the cells from first to lim of the row, within its
// cells.
func (t *tap) cellRange(first, lim byte) []tableCell {
	a, b := min(int(first), len(t.cells)), min(int(lim), len(t.cells))
	if a >= b {
		return nil
	}
	return t.cells[a:b]
}

// applyTap applies the table modifier s to t.
func applyTap(t *tap, s sprm) {
	b := s.operand
	switch s.op {
	case sprmTDefTable:
		// cb, the number of cells, the n+1 edges of the cells, and a TC80
		// of 20 bytes for each cell (MS-DOC 2.9.302)
		if len(b) < 3 {
			return
		}
		n := min(int(b[2]), sprmTDefTableMaxCells)
		if 3+2*(n+1) > len(b) {
			return
		}
		t.centers = make([]int, n+1)
		for i := range n + 1 {
			t.centers[i] = int(int16(le.Uint16(b[3+2*i:])))
		}
		t.cells = make([]tableCell, n)
		tcs := b[3+2*(n+1):]
		for i := range n {
			if len(tcs) < 20*(i+1) {
				break
			}
			tc := tcs[20*i:]
			grf := le.Uint16(tc)
			c := &t.cells[i]
			c.vertMerge = byte(grf >> 5 & 3)
			c.vertAlign = byte(grf >> 7 & 3)
			for k := range 4 {
				c.borders[k] = brc80(tc[4+4*k:])
			}
		}
	case sprmTTableBorders80:
		if len(b) < 25 {
			return
		}
		for k := range 6 {
			t.borders[k] = brc80(b[1+4*k:])
		}
	case sprmTTableBorders:
		if len(b) < 49 {
			return
		}
		for k := range 6 {
			t.borders[k] = brc97(b[1+8*k:])
		}
	case sprmTSetBrc80, sprmTSetBrc:
		// cb, the first cell, the cell after the last, the sides, the
		// border
		if len(b) < 5 {
			return
		}
		var br brc
		if s.op == sprmTSetBrc80 {
			br = brc80(b[4:])
		} else {
			br = brc97(b[4:])
		}
		for i := range t.cellRange(b[1], b[2]) {
			c := &t.cellRange(b[1], b[2])[i]
			for k := range 4 {
				if b[3]&(1<<k) != 0 {
					c.borders[k] = br
				}
			}
		}
	case sprmTDefTableShd80:
		for i := 0; 1+2*i+2 <= len(b) && i < len(t.cells); i++ {
			t.cells[i].shading = shd80(le.Uint16(b[1+2*i:]))
		}
	case sprmTDefTableShd, sprmTDefTableShd2nd, sprmTDefTableShd3rd:
		first := map[uint16]int{sprmTDefTableShd: 0, sprmTDefTableShd2nd: 22, sprmTDefTableShd3rd: 44}[s.op]
		for i := 0; 1+10*i+10 <= len(b) && first+i < len(t.cells); i++ {
			t.cells[first+i].shading = shd97(b[1+10*i:])
		}
	case sprmTVertMerge:
		if len(b) >= 3 && int(b[1]) < len(t.cells) {
			t.cells[b[1]].vertMerge = b[2]
		}
	case sprmTVertAlign:
		if len(b) >= 4 {
			for i := range t.cellRange(b[1], b[2]) {
				t.cellRange(b[1], b[2])[i].vertAlign = b[3]
			}
		}
	case sprmTCellPaddingDflt, sprmTCellPadding:
		// cb, the first cell, the cell after the last, the sides, the
		// unit, and the width. The padding of the cells of a range takes
		// the padding of all the cells here.
		if len(b) < 7 || b[4] != 3 {
			return
		}
		v := int(le.Uint16(b[5:]))
		for k := range 4 {
			if b[3]&(1<<k) != 0 {
				t.padding[k], t.hasPadding[k] = v, true
			}
		}
	case sprmTDxaGapHalf:
		t.gapHalf = int(s.value())
	case sprmTDyaRowHeight:
		t.rowHeight = int(s.value())
	case sprmTJc, sprmTJc90:
		t.jc = int(s.value())
	case sprmTTableHeader:
		t.header = s.byteValue() != 0
	}
}

// isTableSprm reports whether op is a modifier of a table that applyTap
// reads.
func isTableSprm(op uint16) bool {
	switch op {
	case sprmTDefTable, sprmTTableBorders80, sprmTTableBorders, sprmTSetBrc80, sprmTSetBrc,
		sprmTDefTableShd80, sprmTDefTableShd, sprmTDefTableShd2nd, sprmTDefTableShd3rd,
		sprmTVertMerge, sprmTVertAlign, sprmTCellPaddingDflt, sprmTCellPadding,
		sprmTDxaGapHalf, sprmTDyaRowHeight, sprmTJc, sprmTJc90, sprmTTableHeader:
		return true
	}
	return false
}

// tableRow is a row of a table that the conversion holds until the end of
// the table: the HTML of its cells and its properties.
type tableRow struct {
	cells []string
	tap   *tap
}

// writeTable writes the rows of a table. The edges of the cells of all the
// rows make the columns of the table, so that each cell covers the columns
// between its edges, as in Word, where each row has cells of its own. A
// cell that the cells above cover becomes part of the cell where they
// start.
func writeTable(b *strings.Builder, rows []tableRow) {
	if len(rows) == 0 {
		return
	}
	for i := range rows {
		if rows[i].tap == nil {
			rows[i].tap = &tap{}
		}
	}
	edges := tableEdges(rows)
	first := rows[0].tap
	var style []string
	if len(edges) > 1 {
		style = append(style, "table-layout: fixed", "width: "+pt(edges[len(edges)-1]-edges[0]))
		switch first.jc {
		case 1:
			style = append(style, "margin-left: auto", "margin-right: auto")
		case 2:
			style = append(style, "margin-left: auto", "margin-right: 0")
		default:
			if v := edges[0]; v != 0 && abs(v) < 14400 {
				style = append(style, "margin-left: "+pt(v))
			}
		}
	}
	b.WriteString("<table")
	if len(style) > 0 {
		b.WriteString(` style="` + strings.Join(style, "; ") + `"`)
	}
	b.WriteString(">\n")
	if len(edges) > 1 {
		b.WriteString("<colgroup>")
		for k := 1; k < len(edges); k++ {
			b.WriteString(`<col style="width: ` + pt(edges[k]-edges[k-1]) + `"/>`)
		}
		b.WriteString("</colgroup>\n")
	}
	// column returns the column of the grid at the edge x
	column := func(x int) int {
		k := sort.SearchInts(edges, x)
		if k > 0 && (k == len(edges) || x-edges[k-1] < edges[k]-x) {
			k--
		}
		return k
	}
	// the header rows at the start of the table repeat at the top of each
	// page
	head := 0
	for head < len(rows) && rows[head].tap.header {
		head++
	}
	if head == len(rows) {
		head = 0
	}
	for r, row := range rows {
		t := row.tap
		switch {
		case head > 0 && r == 0:
			b.WriteString("<thead>")
		case head > 0 && r == head:
			b.WriteString("</thead><tbody>")
		}
		b.WriteString("<tr")
		if h := abs(t.rowHeight); h > 0 && h < 31680 {
			b.WriteString(` style="height: ` + pt(h) + `"`)
		}
		b.WriteString(">")
		at := 0
		if len(t.centers) > 0 && len(edges) > 1 {
			if c := column(t.centers[0]); c > 0 {
				b.WriteString(`<td colspan="` + strconv.Itoa(c) + `" style="border: none"></td>`)
				at = c
			}
		}
		for i, html := range row.cells {
			var c tableCell
			if i < len(t.cells) {
				c = t.cells[i]
			}
			span := 1
			if i+1 < len(t.centers) && len(edges) > 1 {
				end := column(t.centers[i+1])
				span = max(end-at, 1)
			}
			at += span
			if c.vertMerge == vertMergeContinue && r > 0 {
				continue
			}
			rowSpan := 1
			if c.vertMerge == vertMergeRestart {
				// the cells of the header rows merge among themselves
				limit := len(rows)
				if r < head {
					limit = head
				}
				for k := r + 1; k < limit; k++ {
					next := rows[k].tap
					if i >= len(next.cells) || i >= len(rows[k].cells) || next.cells[i].vertMerge != vertMergeContinue {
						break
					}
					rowSpan++
				}
			}
			b.WriteString("<td")
			if span > 1 {
				b.WriteString(` colspan="` + strconv.Itoa(span) + `"`)
			}
			if rowSpan > 1 {
				b.WriteString(` rowspan="` + strconv.Itoa(rowSpan) + `"`)
			}
			b.WriteString(` style="` + cellStyle(t, c, i, r == 0, r+rowSpan >= len(rows)) + `">`)
			if h := exactHeight(t); h > 0 && rowSpan == 1 {
				// a row of an exact height does not grow with the text of
				// its cells, such as the empty paragraph after the text of
				// a cell. The text that does not fit shows, and is not cut,
				// so that the shapes that the cell anchors show
				b.WriteString(`<div style="height: ` + num(h) + `pt">` + html + "</div>")
			} else {
				b.WriteString(html)
			}
			b.WriteString("</td>")
		}
		b.WriteString("</tr>\n")
	}
	if head > 0 {
		b.WriteString("</tbody>")
	}
	b.WriteString("</table>\n")
}

// tableEdges returns the edges of the cells of the rows, in order, with
// the edges closer than 2 points as one.
func tableEdges(rows []tableRow) []int {
	var all []int
	for _, row := range rows {
		for _, x := range row.tap.centers {
			if abs(x) < 31680 {
				all = append(all, x)
			}
		}
	}
	sort.Ints(all)
	var edges []int
	for _, x := range all {
		if len(edges) == 0 || x-edges[len(edges)-1] >= 40 {
			edges = append(edges, x)
		}
		if len(edges) > 4*sprmTDefTableMaxCells {
			break
		}
	}
	return edges
}

// cellStyle returns the style of the cell c, the cell i of the row t. top
// and bottom are set for a cell of the first and of the last row of the
// table.
func cellStyle(t *tap, c tableCell, i int, top, bottom bool) string {
	var style []string
	// the border of the cell, else the border of the table at that side
	sides := [4]struct {
		name  string
		table brc
	}{
		{"top", pick(top, t.borders[0], t.borders[4])},
		{"left", pick(i == 0, t.borders[1], t.borders[5])},
		{"bottom", pick(bottom, t.borders[2], t.borders[4])},
		{"right", pick(i+1 >= len(t.cells), t.borders[3], t.borders[5])},
	}
	for k, side := range sides {
		br := c.borders[k]
		if !br.set {
			br = side.table
		}
		style = append(style, "border-"+side.name+": "+br.css())
	}
	pad := [4]int{0, t.gapHalf, 0, t.gapHalf}
	for k := range 4 {
		if t.hasPadding[k] {
			pad[k] = t.padding[k]
		}
		pad[k] = max(0, min(pad[k], 1440))
	}
	style = append(style, "padding: "+pt(pad[0])+" "+pt(pad[3])+" "+pt(pad[2])+" "+pt(pad[1]))
	switch c.vertAlign {
	case 1:
		style = append(style, "vertical-align: middle")
	case 2:
		style = append(style, "vertical-align: bottom")
	}
	if c.shading.set {
		style = append(style, fmt.Sprintf("background-color: #%06x", c.shading.color))
	}
	return strings.Join(style, "; ")
}

// exactHeight returns the height of the content of the cells of the row
// t in points, when the row has an exact height, or 0. The height of the
// row holds the padding of the cells at the top and the bottom.
func exactHeight(t *tap) float64 {
	if t.rowHeight >= 0 || t.rowHeight <= -31680 {
		return 0
	}
	h := -t.rowHeight
	for _, k := range [...]int{0, 2} {
		if t.hasPadding[k] {
			h -= max(0, min(t.padding[k], 1440))
		}
	}
	return max(float64(h)/20, 0.05)
}

// pick returns a if ok, else b.
func pick[T any](ok bool, a, b T) T {
	if ok {
		return a
	}
	return b
}
