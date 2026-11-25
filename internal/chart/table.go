package chart

import (
	"image/color"
	"math"
	"slices"

	"github.com/xo/okage/roxmltree"
)

// This file draws the data table of a chart, c:dTable of DrawingML
// (ECMA-376 Part 1, 21.2.2.54): a table below the plot, with a column for
// each category and a row for each series, as Excel draws it. The table
// takes the place of the labels of an axis of categories along the bottom.
// Below the bars that go across, it goes below the labels of the values,
// with the categories from the left in their order.

// dataTable is the data table of a chart.
type dataTable struct {
	// horz and vert draw the lines between the rows and between the
	// columns, outline the lines around the table, and keys the symbols of
	// the legend before the names of the series.
	horz, vert, outline, keys bool
	// line is the color of the lines, if hasLine. noLine draws no line.
	line    color.NRGBA
	hasLine bool
	noLine  bool
	text    textStyle
}

// The sizes of a data table: the height of a row and the gaps of a cell,
// in ems of its text.
const (
	tableRow = 1.6
	tablePad = 0.35
)

// dataTable reads c:dTable.
func (p *dmlParser) dataTable(n roxmltree.Node) *dataTable {
	t := &dataTable{}
	for e := range elements(n) {
		switch e.TagName().Name() {
		case "showHorzBorder":
			t.horz = boolVal(e)
		case "showVertBorder":
			t.vert = boolVal(e)
		case "showOutline":
			t.outline = boolVal(e)
		case "showKeys":
			t.keys = boolVal(e)
		case "spPr":
			t.line, t.hasLine, t.noLine, _, _ = p.lineOf(e)
		case "txPr":
			t.text = p.bodyStyle(e)
		}
	}
	return t
}

// hasTable says whether the plot draws a data table: a chart of
// categories.
func (pl *plane) hasTable() bool {
	return pl.c.table != nil && !pl.scatter && pl.n > 0
}

// tableOnCats says whether the data table takes the place of the labels of
// the axis of categories, which is along the bottom.
func (pl *plane) tableOnCats() bool {
	return pl.hasTable() && !pl.horizontal
}

// tableTop returns the top of the data table: the bottom of the inner box
// of the plot, or of the labels of the values below it.
func (pl *plane) tableTop() float64 {
	c := pl.c
	if !pl.horizontal || !c.val.shown() || pl.valueEnd() {
		return pl.in.y1
	}
	return pl.in.y1 + c.val.text.size*lineHeight + gapLabel + pl.tickOut(c.val)
}

// tableEdges returns the bounds of the columns of the categories, from the
// left.
func (pl *plane) tableEdges() []float64 {
	edges := make([]float64, pl.n+1)
	if pl.horizontal {
		for i := range edges {
			edges[i] = pl.in.x0 + float64(i)*pl.in.w()/float64(pl.n)
		}
		return edges
	}
	for i := range pl.n {
		mid := pl.catCoord(pl.center(i))
		edges[i] = mid - pl.slot()/2
		edges[i+1] = mid + pl.slot()/2
	}
	if pl.c.cat.reverse {
		slices.Reverse(edges)
	}
	return edges
}

// tableSeries returns the series of the rows of the data table, in the
// order of the legend.
func (pl *plane) tableSeries() []*series {
	var out []*series
	for _, p := range pl.c.plots {
		out = append(out, p.series...)
	}
	return out[:min(len(out), maxSeries)]
}

// tableHeight returns the height of the data table.
func (pl *plane) tableHeight() float64 {
	st := pl.c.table.text
	return float64(len(pl.tableSeries())+1) * st.size * tableRow
}

// tableKey returns the width of the symbol of a series in the data table,
// or zero.
func (pl *plane) tableKey() float64 {
	if !pl.c.table.keys {
		return 0
	}
	for _, p := range pl.c.plots {
		if p.kind == kindLine {
			return pl.c.table.text.size * 1.8
		}
	}
	return pl.c.table.text.size * 0.6
}

// tableNames returns the width of the column of the names of the series,
// left of the plot.
func (pl *plane) tableNames() float64 {
	st := pl.c.table.text
	w := 0.0
	for _, s := range pl.tableSeries() {
		w = max(w, pl.cv.t.width(seriesName(s), st))
	}
	if k := pl.tableKey(); k > 0 {
		w += k + st.size*tablePad
	}
	return w + 2*st.size*tablePad
}

// table draws the data table below the inner box of the plot.
func (pl *plane) table() {
	c := pl.c
	cv := pl.cv
	tb := c.table
	st := tb.text
	rowH := st.size * tableRow
	rows := pl.tableSeries()
	in := pl.in
	left := in.x0 - pl.tableNames()
	top := pl.tableTop()
	bottom := top + float64(len(rows)+1)*rowH
	col := gridGray
	switch {
	case tb.noLine:
		col = color.NRGBA{}
	case tb.hasLine:
		col = tb.line
	}
	edges := pl.tableEdges()
	x0, x1 := min(edges[0], edges[pl.n]), max(edges[0], edges[pl.n])
	// The lines.
	if tb.horz {
		for r := 1; r <= len(rows); r++ {
			y := top + float64(r)*rowH
			cv.line(left, y, x1, y, col, thin)
		}
	}
	if tb.vert {
		for i, x := range edges {
			if i == 0 || i == pl.n {
				continue
			}
			cv.line(x, top, x, bottom, col, thin)
		}
		cv.line(x0, top, x0, bottom, col, thin)
	}
	if tb.outline {
		cv.line(left, top+rowH, left, bottom, col, thin)
		cv.line(left, bottom, x1, bottom, col, thin)
		cv.line(x1, top, x1, bottom, col, thin)
		cv.line(x0, top, x0, top+rowH, col, thin)
		cv.line(left, top+rowH, x0, top+rowH, col, thin)
		cv.line(x0, top, x1, top, col, thin)
	}
	// The names of the categories.
	pad := st.size * tablePad
	for i := range pl.n {
		a, b := edges[i], edges[i+1]
		lines, ok := cv.fitLines(pl.cats[i], st, max(a, b)-min(a, b)-pad, rowH)
		if !ok || len(lines) == 0 {
			continue
		}
		cv.t.draw(cv.dev, block{lines: lines[:1], st: st}, (a+b)/2, top+rowH/2, alignCenter, alignCenter, 0)
	}
	// The rows of the series.
	var entries []entry
	if tb.keys {
		entries = cv.entries()
	}
	for r, s := range rows {
		y := top + (float64(r)+1.5)*rowH
		x := left + pad
		if k := pl.tableKey(); k > 0 {
			for _, e := range entries {
				if e.s == s {
					cv.symbol(e, x, y, k, st.size)
					break
				}
			}
			x += k + pad
		}
		cv.t.draw(cv.dev, block{lines: []string{seriesName(s)}, st: st}, x, y, alignStart, alignCenter, 0)
		for i := range pl.n {
			v := value(s, i)
			if math.IsNaN(v) {
				continue
			}
			f := s.format
			if f == "" {
				f = "General"
			}
			a, b := edges[i], edges[i+1]
			cv.t.draw(cv.dev, block{lines: []string{formatValue(v, f)}, st: st}, (a+b)/2, y, alignCenter, alignCenter, 0)
		}
	}
}
