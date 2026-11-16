package docx

import (
	"strconv"

	"github.com/xo/okage/roxmltree"
)

// maxGridColumns bounds the columns of a table.
const maxGridColumns = 1024

// tableCell is a cell of a table, w:tc.
type tableCell struct {
	node roxmltree.Node
	tc   props
	// col is the first column of the grid of the cell, and span the number
	// of its columns.
	col, span int
	// rowSpan is the number of rows of a cell that w:vMerge merges, or 0 for
	// a cell that a cell above covers.
	rowSpan int
}

// tableRow is a row of a table, w:tr.
type tableRow struct {
	tr     props
	cells  []*tableCell
	header bool
}

// rowsOf returns the rows of a table, with those in a content control or a
// change.
func (c *converter) rowsOf(n roxmltree.Node, out []roxmltree.Node, depth int) []roxmltree.Node {
	if depth > 4 {
		return out
	}
	for e := range n.Children() {
		if !e.IsElement() {
			continue
		}
		switch e.TagName().Name() {
		case "tr":
			out = append(out, e)
		case "sdt":
			if content, ok := child(e, "sdtContent"); ok {
				out = c.rowsOf(content, out, depth+1)
			}
		case "customXml", "ins", "moveTo":
			out = c.rowsOf(e, out, depth+1)
		}
	}
	return out
}

// cellsOf returns the cells of a row, with those in a content control.
func cellsOf(n roxmltree.Node, out []roxmltree.Node, depth int) []roxmltree.Node {
	if depth > 4 {
		return out
	}
	for e := range n.Children() {
		if !e.IsElement() {
			continue
		}
		switch e.TagName().Name() {
		case "tc":
			out = append(out, e)
		case "sdt":
			if content, ok := child(e, "sdtContent"); ok {
				out = cellsOf(content, out, depth+1)
			}
		case "customXml":
			out = cellsOf(e, out, depth+1)
		}
	}
	return out
}

// table converts a w:tbl element.
func (c *converter) table(t roxmltree.Node) {
	if !c.enter() {
		return
	}
	defer c.leave()
	direct := props{}
	if pr, ok := child(t, "tblPr"); ok {
		direct = parseTblPr(pr)
	}
	st := c.styles.get(direct["tblStyle"], "table")
	if st == nil {
		st = &style{ppr: props{}, rpr: props{}, tbl: props{}, tc: props{}}
	}
	tp := st.tbl.over(direct)
	if _, ok := tp["look.firstRow"]; !ok {
		tp = tp.over(props{"look.firstRow": "1", "look.firstColumn": "1", "look.noVBand": "1"})
	}
	var grid []float64
	if g, ok := child(t, "tblGrid"); ok {
		for col := range g.Children() {
			if col.IsElement() && col.TagName().Name() == "gridCol" && len(grid) < maxGridColumns {
				grid = append(grid, max(number(attrOf(col, "w"), 0)/20, 0))
			}
		}
	}
	rows := c.readRows(t, &grid)
	if len(rows) == 0 {
		return
	}
	cols := len(grid)

	var s css
	total := 0.0
	for _, w := range grid {
		total += w
	}
	switch w := tp.num("tblW", 0); {
	case tp["tblW.type"] == "pct" && w > 0:
		s.set("width", num(min(w/50, 100))+"%")
	case tp["tblW.type"] == "dxa" && w > 0:
		s.set("width", pt(w/20))
	case total > 0:
		s.set("width", pt(total))
	}
	if total > 0 {
		s.set("table-layout", "fixed")
	}
	switch tp["jc"] {
	case "center":
		s.set("margin-left", "auto")
		s.set("margin-right", "auto")
	case "right", "end":
		s.set("margin-left", "auto")
	default:
		// The text of the first cell lines up with the text around the
		// table, so the table starts a cell margin to the left.
		ind := tp.num("tblInd", 0)/20 - tp.num("mar.left", 108)/20
		if tp["tblInd.type"] != "dxa" && tp["tblInd.type"] != "" {
			ind = -tp.num("mar.left", 108) / 20
		}
		if ind != 0 {
			s.set("margin-left", pt(ind))
		}
	}
	if v, ok := hexColor(tp["shd"]); ok {
		s.set("background-color", v)
	}
	c.w.WriteString(`<table style="` + attrText(s.String()) + `">`)
	if total > 0 {
		c.w.WriteString("<colgroup>")
		for _, w := range grid {
			c.w.WriteString(`<col style="width: ` + pt(w) + `"/>`)
		}
		c.w.WriteString("</colgroup>")
	}
	inHead := rows[0].header
	if inHead {
		c.w.WriteString("<thead>")
	} else {
		c.w.WriteString("<tbody>")
	}
	for r, row := range rows {
		if c.check() != nil {
			return
		}
		if inHead && !row.header {
			c.w.WriteString("</thead><tbody>")
			inHead = false
		}
		c.row(st, tp, rows, r, cols)
	}
	if inHead {
		c.w.WriteString("</thead></table>")
	} else {
		c.w.WriteString("</tbody></table>")
	}
}

// attrOf returns the attribute of n with the local name name.
func attrOf(n roxmltree.Node, name string) string {
	for a := range n.Attributes() {
		if a.Name() == name {
			return a.Value()
		}
	}
	return ""
}

// readRows reads the rows and the cells of a table, places each cell on the
// grid, and resolves the vertical merges. It widens the grid for a row
// with more columns.
func (c *converter) readRows(t roxmltree.Node, grid *[]float64) []*tableRow {
	var rows []*tableRow
	for _, tr := range c.rowsOf(t, nil, 0) {
		if len(rows) >= c.lim.MaxObjects {
			break
		}
		row := &tableRow{tr: props{}}
		col := 0
		if pr, ok := child(tr, "trPr"); ok {
			for e := range pr.Children() {
				if !e.IsElement() {
					continue
				}
				switch e.TagName().Name() {
				case "trHeight":
					row.tr["height"] = attrOf(e, "val")
					row.tr["hRule"] = attrOf(e, "hRule")
				case "tblHeader":
					v := attrOf(e, "val")
					row.header = v == "" || v == "1" || v == "true" || v == "on"
				case "gridBefore":
					if v, err := strconv.Atoi(attrOf(e, "val")); err == nil && v > 0 {
						col = min(v, maxGridColumns)
					}
				case "cantSplit":
					row.tr["cantSplit"] = "1"
				}
			}
		}
		for _, tc := range cellsOf(tr, nil, 0) {
			cell := &tableCell{node: tc, tc: props{}, col: col, span: 1, rowSpan: 1}
			if pr, ok := child(tc, "tcPr"); ok {
				cell.tc = parseTcPr(pr)
			}
			if v, err := strconv.Atoi(cell.tc["gridSpan"]); err == nil && v > 1 {
				cell.span = min(v, maxGridColumns)
			}
			if cell.tc["hMerge"] == "continue" && len(row.cells) > 0 {
				row.cells[len(row.cells)-1].span += cell.span
				col += cell.span
				continue
			}
			col += cell.span
			if col > maxGridColumns {
				break
			}
			row.cells = append(row.cells, cell)
		}
		for len(*grid) < col {
			*grid = append(*grid, 0)
		}
		rows = append(rows, row)
	}
	// A cell with w:vMerge restart spans the cells with w:vMerge continue
	// below it, in the same column.
	for r, row := range rows {
		for _, cell := range row.cells {
			if cell.tc["vMerge"] == "continue" {
				if r > 0 && covered(rows[r-1], cell.col) {
					cell.rowSpan = 0
				}
				continue
			}
			if cell.tc["vMerge"] != "restart" {
				continue
			}
			for below := r + 1; below < len(rows); below++ {
				next := cellAt(rows[below], cell.col)
				if next == nil || next.tc["vMerge"] != "continue" {
					break
				}
				cell.rowSpan++
			}
		}
	}
	return rows
}

// cellAt returns the cell of a row that starts at the column col, or nil.
func cellAt(row *tableRow, col int) *tableCell {
	for _, cell := range row.cells {
		if cell.col == col {
			return cell
		}
	}
	return nil
}

// covered reports whether a row has a merged cell at the column col.
func covered(row *tableRow, col int) bool {
	cell := cellAt(row, col)
	return cell != nil && (cell.tc["vMerge"] == "restart" || cell.tc["vMerge"] == "continue")
}

// conditions returns the conditional parts of a table style that apply to
// the cell at the row r and the column col, in their order of priority,
// the last one first.
func conditions(tp props, rows []*tableRow, r, col, span, cols int) []string {
	var out []string
	look := func(name string) bool { return tp.on("look." + name) }
	first, last := look("firstRow"), look("lastRow")
	firstCol, lastCol := look("firstColumn"), look("lastColumn")
	size := max(int(tp.num("tblStyleRowBandSize", 1)), 1)
	if !look("noVBand") {
		c := col
		if firstCol {
			c--
		}
		if c >= 0 {
			out = append(out, []string{"band1Vert", "band2Vert"}[(c/size)%2])
		}
	}
	if !look("noHBand") {
		i := r
		if first {
			i--
		}
		if i >= 0 && (!last || r != len(rows)-1) {
			out = append(out, []string{"band1Horz", "band2Horz"}[(i/size)%2])
		}
	}
	if firstCol && col == 0 {
		out = append(out, "firstCol")
	}
	if lastCol && col+span == cols {
		out = append(out, "lastCol")
	}
	if first && r == 0 {
		out = append(out, "firstRow")
	}
	if last && r == len(rows)-1 {
		out = append(out, "lastRow")
	}
	top, bottom := r == 0, r == len(rows)-1
	left, right := col == 0, col+span == cols
	switch {
	case top && left:
		out = append(out, "nwCell")
	case top && right:
		out = append(out, "neCell")
	case bottom && left:
		out = append(out, "swCell")
	case bottom && right:
		out = append(out, "seCell")
	}
	return out
}

// row converts row r of a table.
func (c *converter) row(st *style, tp props, rows []*tableRow, r, cols int) {
	row := rows[r]
	var s css
	if h := number(row.tr["height"], 0) / 20; h > 0 {
		s.set("height", pt(h))
	}
	if row.tr["cantSplit"] != "" {
		s.set("break-inside", "avoid")
	}
	if s.b.Len() > 0 {
		c.w.WriteString(`<tr style="` + attrText(s.String()) + `">`)
	} else {
		c.w.WriteString("<tr>")
	}
	for _, cell := range row.cells {
		if cell.rowSpan == 0 {
			continue
		}
		c.cell(st, tp, rows, r, cols, cell)
	}
	c.w.WriteString("</tr>")
}

// cell converts a cell of row r of a table.
func (c *converter) cell(st *style, tp props, rows []*tableRow, r, cols int, cell *tableCell) {
	cs := &cellStyle{ppr: st.ppr, rpr: st.rpr}
	tc := st.tc
	for _, name := range conditions(tp, rows, r, cell.col, cell.span, cols) {
		if cond, ok := st.conds[name]; ok {
			cs.ppr = cs.ppr.over(cond.ppr)
			cs.rpr = cs.rpr.over(cond.rpr)
			tc = tc.over(cond.tc)
		}
	}
	tc = tc.over(cell.tc)
	var s css
	last := r+cell.rowSpan-1 >= len(rows)-1
	sides := map[string]string{
		"top":    pick(r == 0, tp["tbl.top"], tp["tbl.insideH"]),
		"bottom": pick(last, tp["tbl.bottom"], tp["tbl.insideH"]),
		"left":   pick(cell.col == 0, tp["tbl.left"], tp["tbl.insideV"]),
		"right":  pick(cell.col+cell.span >= cols, tp["tbl.right"], tp["tbl.insideV"]),
	}
	for _, side := range [...]string{"top", "right", "bottom", "left"} {
		v := sides[side]
		if o, ok := tc["tc."+side]; ok {
			v = o
		}
		if b, _ := border(v); b != "" {
			s.set("border-"+side, b)
		}
	}
	for _, side := range [...]string{"top", "right", "bottom", "left"} {
		def := 0.0
		if side == "left" || side == "right" {
			def = 108
		}
		m := tp.num("mar."+side, def)
		m = tc.num("mar."+side, m)
		if m > 0 {
			s.set("padding-"+side, pt(m/20))
		}
	}
	if v, ok := hexColor(tc["shd"]); ok {
		s.set("background-color", v)
	}
	switch tc["vAlign"] {
	case "center":
		s.set("vertical-align", "middle")
	case "bottom":
		s.set("vertical-align", "bottom")
	}
	c.w.WriteString(`<td`)
	if cell.span > 1 {
		c.w.WriteString(` colspan="` + strconv.Itoa(cell.span) + `"`)
	}
	if cell.rowSpan > 1 {
		c.w.WriteString(` rowspan="` + strconv.Itoa(cell.rowSpan) + `"`)
	}
	c.w.WriteString(` style="` + attrText(s.String()) + `">`)
	mark := c.w.Len()
	c.blocks(cell.node, cs)
	if c.w.Len() == mark {
		c.w.WriteString("<p><br/></p>")
	}
	c.w.WriteString("</td>")
}

// pick returns a if cond is true, else b.
func pick(cond bool, a, b string) string {
	if cond {
		return a
	}
	return b
}
