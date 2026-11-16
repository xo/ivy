package drawingml

import (
	"fmt"
	"strings"

	"github.com/xo/okage"
	"github.com/xo/okage/roxmltree"
)

// The limits of a table.
const (
	// maxTableCells is the number of cells of a table that the package
	// draws.
	maxTableCells = 1 << 14
	// maxSpan is the largest span of a cell.
	maxSpan = 1024
)

// The parts of a table style, in the order that they apply.
var tableParts = [...]string{"wholeTbl", "band1H", "band2H", "band1V", "band2V", "lastCol", "firstCol", "lastRow", "firstRow"}

// cellStyle is the style of a cell: its fill, its borders and the style of
// its text.
type cellStyle struct {
	fill Opt[Paint]
	// borders are the left, top, right and bottom borders.
	borders [4]Opt[Line]
	text    RunStyle
}

// tablePart is a part of a table style, such as a:firstRow.
type tablePart struct {
	fill Opt[Paint]
	// borders are the left, top, right, bottom, inside horizontal and
	// inside vertical borders.
	borders [6]Opt[Line]
	text    RunStyle
}

// tableFlags are the parts of the table style that a table uses, from
// a:tblPr.
type tableFlags struct {
	firstRow, lastRow, firstCol, lastCol, bandRow, bandCol bool
}

// TableStyles gives the table styles of a package, such as the a:tblStyle
// elements of ppt/tableStyles.xml, by their ID.
type TableStyles func(id string) (roxmltree.Node, bool)

// table draws a:tbl in the box of w by h points of the space m.
func (t *Tree) table(tbl roxmltree.Node, m okage.Transform, w, h float64, pics Pictures) {
	var cols []float64
	var flags tableFlags
	var parts map[string]*tablePart
	var tableFill Opt[Paint]
	if pr, ok := child(tbl, "tblPr"); ok {
		flags.firstRow, _ = boolAttr(pr, "firstRow")
		flags.lastRow, _ = boolAttr(pr, "lastRow")
		flags.firstCol, _ = boolAttr(pr, "firstCol")
		flags.lastCol, _ = boolAttr(pr, "lastCol")
		flags.bandRow, _ = boolAttr(pr, "bandRow")
		flags.bandCol, _ = boolAttr(pr, "bandCol")
		if id, ok := child(pr, "tableStyleId"); ok && t.TableStyles != nil {
			if st, ok := t.TableStyles(strings.TrimSpace(nodeText(id))); ok {
				parts = t.tableStyle(st, pics)
			}
		} else if st, ok := child(pr, "tableStyle"); ok {
			parts = t.tableStyle(st, pics)
		}
		if pt, ok := t.Colors.PaintIn(pr, pics); ok {
			tableFill.Set(pt)
		}
	}
	if g, ok := child(tbl, "tblGrid"); ok {
		for c := range g.Children() {
			if c.IsElement() && c.TagName().Name() == "gridCol" && len(cols) < maxSpan {
				cols = append(cols, max(emu(numberAttr(c, "w", 0)), 0))
			}
		}
	}
	var rows []roxmltree.Node
	for r := range tbl.Children() {
		if r.IsElement() && r.TagName().Name() == "tr" {
			rows = append(rows, r)
		}
	}
	if len(cols) == 0 || len(rows) == 0 {
		return
	}
	var list ListStyle
	if t.TextStyle != nil {
		if s := t.TextStyle(nil); s != nil {
			list = *s
		}
	}
	tab := &Table{Cols: cols}
	if tableFill.OK {
		tab.Fill = tableFill.V
	}
	cells := 0
	for ri, r := range rows {
		row := TableRow{Height: max(emu(numberAttr(r, "h", 0)), 0)}
		ci := 0
		for c := range r.Children() {
			if !c.IsElement() || c.TagName().Name() != "tc" {
				continue
			}
			col := ci
			ci++
			if b, _ := boolAttr(c, "hMerge"); b {
				continue
			}
			if b, _ := boolAttr(c, "vMerge"); b {
				continue
			}
			cells++
			if cells > maxTableCells {
				break
			}
			style := cellStyleAt(parts, flags, ri, col, len(rows), len(cols))
			row.Cells = append(row.Cells, t.cell(c, &style, &list, pics))
		}
		tab.Rows = append(tab.Rows, row)
	}
	t.Scene.AddTable(tab, w, h, m, t.Colors.Theme)
}

// cellStyleAt returns the style of the cell in the row r and the column c
// of a table of rows by cols, from the parts of its table style.
func cellStyleAt(parts map[string]*tablePart, f tableFlags, r, c, rows, cols int) cellStyle {
	var s cellStyle
	if parts == nil {
		return s
	}
	// the rows and the columns of the bands do not count the first row
	// and the first column
	br, bc := r, c
	if f.firstRow {
		br--
	}
	if f.firstCol {
		bc--
	}
	applies := map[string]bool{
		"wholeTbl": true,
		"band1H":   f.bandRow && br >= 0 && br%2 == 0,
		"band2H":   f.bandRow && br >= 0 && br%2 == 1,
		"band1V":   f.bandCol && bc >= 0 && bc%2 == 0,
		"band2V":   f.bandCol && bc >= 0 && bc%2 == 1,
		"lastCol":  f.lastCol && c == cols-1,
		"firstCol": f.firstCol && c == 0,
		"lastRow":  f.lastRow && r == rows-1,
		"firstRow": f.firstRow && r == 0,
	}
	for _, name := range tableParts {
		p, ok := parts[name]
		if !ok || !applies[name] {
			continue
		}
		s.fill.over(p.fill)
		s.text.Over(&p.text)
		// a border of the table is an outside border at the edge, and an
		// inside border between two cells
		left, top, right, bottom := p.borders[0], p.borders[1], p.borders[2], p.borders[3]
		if c > 0 {
			left = p.borders[5]
		}
		if c < cols-1 {
			right = p.borders[5]
		}
		if r > 0 {
			top = p.borders[4]
		}
		if r < rows-1 {
			bottom = p.borders[4]
		}
		s.borders[0].over(left)
		s.borders[1].over(top)
		s.borders[2].over(right)
		s.borders[3].over(bottom)
	}
	return s
}

// tableStyle reads a:tblStyle.
func (t *Tree) tableStyle(n roxmltree.Node, pics Pictures) map[string]*tablePart {
	parts := make(map[string]*tablePart)
	for e := range n.Children() {
		if !e.IsElement() {
			continue
		}
		p := &tablePart{}
		for c := range e.Children() {
			if !c.IsElement() {
				continue
			}
			switch c.TagName().Name() {
			case "tcTxStyle":
				p.text = t.cellText(c)
			case "tcStyle":
				t.cellPart(c, p, pics)
			}
		}
		parts[e.TagName().Name()] = p
	}
	return parts
}

// cellText reads a:tcTxStyle.
func (t *Tree) cellText(n roxmltree.Node) RunStyle {
	var s RunStyle
	switch attr(n, "b", "") {
	case "on":
		s.Bold.Set(true)
	case "off":
		s.Bold.Set(false)
	}
	switch attr(n, "i", "") {
	case "on":
		s.Italic.Set(true)
	case "off":
		s.Italic.Set(false)
	}
	for e := range n.Children() {
		if !e.IsElement() {
			continue
		}
		switch e.TagName().Name() {
		case "font":
			if l, ok := child(e, "latin"); ok {
				s.Latin.Set(attr(l, "typeface", ""))
			}
		case "fontRef":
			switch attr(e, "idx", "") {
			case "major":
				s.Latin.Set("+mj-lt")
			case "minor":
				s.Latin.Set("+mn-lt")
			}
			if v, ok := t.Colors.In(e); ok {
				s.Paint.Set(Solid(v))
			}
		default:
			if v, ok := t.Colors.Of(e); ok {
				s.Paint.Set(Solid(v))
			}
		}
	}
	return s
}

// cellPart reads a:tcStyle into the part p.
func (t *Tree) cellPart(n roxmltree.Node, p *tablePart, pics Pictures) {
	for e := range n.Children() {
		if !e.IsElement() {
			continue
		}
		switch e.TagName().Name() {
		case "fill":
			if pt, ok := t.Colors.PaintIn(e, pics); ok {
				p.fill.Set(pt)
			}
		case "fillRef":
			idx := int(min(max(numberAttr(e, "idx", 0), 0), 2000))
			colors := t.Colors
			if v, ok := t.Colors.In(e); ok {
				colors = t.Colors.WithPlaceholder(v)
			}
			if f, ok := t.Colors.Theme.FillStyle(idx); ok {
				if pt, ok := colors.PaintOf(f, t.Colors.Theme.StylePictures()); ok {
					p.fill.Set(pt)
				}
			}
		case "tcBdr":
			for b := range e.Children() {
				if !b.IsElement() {
					continue
				}
				i := -1
				switch b.TagName().Name() {
				case "left":
					i = 0
				case "top":
					i = 1
				case "right":
					i = 2
				case "bottom":
					i = 3
				case "insideH":
					i = 4
				case "insideV":
					i = 5
				}
				if i < 0 {
					continue
				}
				if l, ok := t.borderLine(b, pics); ok {
					p.borders[i].Set(l)
				}
			}
		}
	}
}

// borderLine reads a border of a table style, which holds a:ln or a:lnRef.
func (t *Tree) borderLine(n roxmltree.Node, pics Pictures) (Line, bool) {
	if ln, ok := child(n, "ln"); ok {
		return t.Colors.ParseLine(ln, DefaultLine(), pics), true
	}
	if ref, ok := child(n, "lnRef"); ok {
		idx := int(min(max(numberAttr(ref, "idx", 0), 0), 2000))
		colors := t.Colors
		if v, ok := t.Colors.In(ref); ok {
			colors = t.Colors.WithPlaceholder(v)
		}
		if e, ok := t.Colors.Theme.LineStyle(idx); ok {
			return colors.ParseLine(e, DefaultLine(), pics), true
		}
	}
	return Line{}, false
}

// cell reads a:tc.
func (t *Tree) cell(c roxmltree.Node, style *cellStyle, list *ListStyle, pics Pictures) TableCell {
	cell := TableCell{
		ColSpan: 1,
		RowSpan: 1,
		Insets:  [4]float64{defaultInsetX, defaultInsetY, defaultInsetX, defaultInsetY},
		Anchor:  "t",
		Fill:    style.fill.Or(Paint{Kind: PaintNone}),
	}
	if v, ok := number(attr(c, "gridSpan", "")); ok && v > 1 {
		cell.ColSpan = int(min(v, maxSpan))
	}
	if v, ok := number(attr(c, "rowSpan", "")); ok && v > 1 {
		cell.RowSpan = int(min(v, maxSpan))
	}
	borders := style.borders
	if pr, ok := child(c, "tcPr"); ok {
		for i, name := range [...]string{"marL", "marT", "marR", "marB"} {
			if v, ok := number(attr(pr, name, "")); ok {
				cell.Insets[i] = min(max(emu(v), 0), 1000)
			}
		}
		cell.Anchor = attr(pr, "anchor", "t")
		if pt, ok := t.Colors.PaintIn(pr, pics); ok {
			cell.Fill = pt
		}
		for i, name := range [...]string{"lnL", "lnT", "lnR", "lnB"} {
			if e, ok := child(pr, name); ok {
				base := borders[i].Or(DefaultLine())
				borders[i].Set(t.Colors.ParseLine(e, base, pics))
			}
		}
	}
	for i := range borders {
		if borders[i].OK {
			l := borders[i].V
			cell.Borders[i] = &l
		}
	}
	if body, ok := child(c, "txBody"); ok {
		run := style.text
		cell.Text = t.Colors.ParseTextBody(body, list, &run, BodyStyle{}, t.Fields)
	}
	return cell
}

// Table is a table of cells with text, which a scene lays out as a table of
// HTML.
type Table struct {
	// Cols are the widths of the columns, in points.
	Cols []float64
	Rows []TableRow
	// Fill is the fill behind the cells.
	Fill Paint
}

// TableRow is a row of a table.
type TableRow struct {
	// Height is the least height of the row, in points.
	Height float64
	Cells  []TableCell
}

// TableCell is a cell of a table.
type TableCell struct {
	// ColSpan and RowSpan are the columns and the rows that the cell
	// covers, from 1.
	ColSpan, RowSpan int
	Fill             Paint
	// Borders are the left, top, right and bottom borders, or nil.
	Borders [4]*Line
	// Insets are the left, top, right and bottom insets, in points.
	Insets [4]float64
	// Anchor is "t", "ctr" or "b".
	Anchor string
	// Text is the text of the cell, or nil.
	Text *TextBody
}

// AddTable adds a table in the box of w by h points, with the transform m
// from the box onto the space of the scene. theme gives the fonts of the
// theme to the text, and can be nil.
func (s *Scene) AddTable(tab *Table, w, h float64, m okage.Transform, theme *Theme) {
	if len(tab.Cols) == 0 || len(tab.Rows) == 0 {
		return
	}
	tableW := 0.0
	for _, c := range tab.Cols {
		tableW += c
	}
	pageW := max(tableW, w, 1)
	page := max(h*overflowLines, h+720)
	var sb strings.Builder
	fmt.Fprintf(&sb, `<!DOCTYPE html><html><head><meta charset="utf-8"><style>
@page { size: %spt %spt; margin: 0 }
html, body { margin: 0; padding: 0 }
table { border-collapse: collapse; border-spacing: 0; table-layout: fixed }
td { overflow-wrap: break-word; padding: 0 }
p { margin: 0; white-space: pre-wrap; overflow-wrap: break-word; hyphens: none }
span.b { display: inline-block; text-indent: 0; white-space: pre }
</style></head><body><table style="width:%spt`, num(pageW), num(page), num(tableW))
	if c := tab.Fill.Mean(); tab.Fill.Visible() && c.A > 0 {
		sb.WriteString(";background-color:" + c.CSS())
	}
	sb.WriteString(`"><colgroup>`)
	for _, c := range tab.Cols {
		fmt.Fprintf(&sb, `<col style="width:%spt">`, num(c))
	}
	sb.WriteString("</colgroup>")
	for _, r := range tab.Rows {
		fmt.Fprintf(&sb, `<tr style="height:%spt">`, num(r.Height))
		for i := range r.Cells {
			writeCell(&sb, &r.Cells[i], theme)
		}
		sb.WriteString("</tr>")
	}
	sb.WriteString("</table></body></html>")
	s.AddHTML(sb.String(), pageW, page, m)
}

// writeCell writes a cell as an element td.
func writeCell(sb *strings.Builder, c *TableCell, theme *Theme) {
	sb.WriteString("<td")
	if c.ColSpan > 1 {
		fmt.Fprintf(sb, ` colspan="%d"`, min(c.ColSpan, maxSpan))
	}
	if c.RowSpan > 1 {
		fmt.Fprintf(sb, ` rowspan="%d"`, min(c.RowSpan, maxSpan))
	}
	anchor := "top"
	switch c.Anchor {
	case "ctr":
		anchor = "middle"
	case "b":
		anchor = "bottom"
	}
	in := c.Insets
	fmt.Fprintf(sb, ` style="vertical-align:%s;padding:%spt %spt %spt %spt`, anchor, num(in[1]), num(in[2]), num(in[3]), num(in[0]))
	if c.Fill.Visible() {
		sb.WriteString(";background-color:" + c.Fill.Mean().CSS())
	}
	for i, side := range [...]string{"left", "top", "right", "bottom"} {
		l := c.Borders[i]
		if l == nil || !l.Visible() {
			continue
		}
		kind := "solid"
		if len(l.Dash) >= 2 {
			kind = "dashed"
			if l.Dash[0] <= 1 {
				kind = "dotted"
			}
		}
		fmt.Fprintf(sb, ";border-%s:%spt %s %s", side, num(max(l.Width, 0.5)), kind, l.Paint.Mean().CSS())
	}
	sb.WriteString(`">`)
	if c.Text != nil {
		for i := range c.Text.Paragraphs {
			writeParagraph(sb, &c.Text.Paragraphs[i], theme, 1, 0, i == 0)
		}
	}
	sb.WriteString("</td>")
}
