package xlsx

import (
	"github.com/xo/ivy/internal/sheet"
)

// This file reads the pivot tables of a worksheet (ECMA-376 Part 1,
// 18.10), for the style of their cells. The worksheet holds the cells that
// Excel computed, and a pivot table gives them the font of its style. The
// styles of the pivot tables that Excel gives, such as PivotStyleLight16,
// make the header rows and the row of the grand totals bold. The package
// does not draw the fills and the borders of the styles.

// readPivotTables makes bold the cells of the header rows and of the row
// of the grand totals of each pivot table of the worksheet part that has a
// style. The rows of sh must be sorted.
func (b *book) readPivotTables(sh *sheet.Sheet, part string) {
	// sheetRel reads the relationships of the part
	b.sheetRel(part, "")
	n := 0
	for _, r := range b.partRels[part] {
		if r.External || !r.TypeIs("pivotTable") {
			continue
		}
		if n++; n > maxPivotTables {
			break
		}
		doc, err := b.parsePart(r.Target)
		if err != nil {
			continue
		}
		root := doc.RootElement()
		info, ok := child(root, "pivotTableStyleInfo")
		if !ok || attr(info, "name") == "" {
			continue
		}
		loc, ok := child(root, "location")
		if !ok {
			continue
		}
		ref, ok := parseRange(attr(loc, "ref"))
		if !ok {
			continue
		}
		// the header rows are the rows before the first row of data
		head := min(ref.Row0+max(0, integer(loc, "firstDataRow", 0)), ref.Row1+1)
		grand := -1
		if items, ok := child(root, "rowItems"); ok && flag(root, "rowGrandTotals", true) {
			var last string
			for i := range children(items, "i") {
				last = attr(i, "t")
			}
			if last == "grand" {
				grand = ref.Row1
			}
		}
		b.boldRows(sh, ref, head, grand)
	}
}

// maxPivotTables bounds the pivot tables of a worksheet.
const maxPivotTables = 256

// boldRows makes bold the cells of the range r in the rows before the row
// head and in the row grand.
func (b *book) boldRows(sh *sheet.Sheet, r sheet.Range, head, grand int) {
	for i := range sh.Rows {
		row := &sh.Rows[i]
		if row.Index < r.Row0 || row.Index > r.Row1 || (row.Index >= head && row.Index != grand) {
			continue
		}
		for k := range row.Cells {
			if c := &row.Cells[k]; c.Col >= r.Col0 && c.Col <= r.Col1 {
				c.Style = b.boldStyle(c.Style)
			}
		}
	}
}

// boldStyle returns the index of the style i with a bold font, which it
// adds to the styles of the cells once.
func (b *book) boldStyle(i int) int {
	s := b.styles
	if i < 0 || i >= len(s.cells) || s.cells[i].Font.Bold {
		return i
	}
	if j, ok := b.bold[i]; ok {
		return j
	}
	st := s.cells[i]
	st.Font.Bold = true
	s.cells = append(s.cells, st)
	s.formats = append(s.formats, s.format(i))
	if i < len(s.codes) {
		s.codes = append(s.codes, s.codes[i])
	}
	if b.bold == nil {
		b.bold = make(map[int]int)
	}
	b.bold[i] = len(s.cells) - 1
	return len(s.cells) - 1
}
