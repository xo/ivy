package chart

import (
	"math"
	"slices"
	"strconv"
	"strings"

	"github.com/xo/okage/roxmltree"

	"github.com/xo/ivy/internal/ooxml"
)

// This file reads the charts of Office 2016, the part with the root
// cx:chartSpace ([MS-ODRAWXML] 2.24): the funnel, the waterfall, the
// histogram, the Pareto chart, the box and whisker chart, the treemap, the
// sunburst and the region map. The data of these charts is in
// cx:chartData, apart from the series.

// exData is one cx:data: the categories and the values of a series.
// levels are the levels of the categories of a tree, from the innermost:
// levels[0] holds the names of the points, and cats is levels[0].
type exData struct {
	cats   []string
	levels [][]string
	vals   []float64
	format string
}

// maxBins is the number of the bins of a histogram.
const maxBins = 1000

// maxLevels is the number of the levels of the categories of a chart of
// Office 2016.
const maxLevels = 16

// chartEx reads cx:chartSpace, whose cx:chart is chart.
func (p *dmlParser) chartEx(root, chart roxmltree.Node) {
	c := p.c
	data := map[string]exData{}
	if cd, ok := child(root, "chartData"); ok {
		for d := range elements(cd) {
			if d.TagName().Name() == "data" && len(data) < maxSeries {
				data[ooxml.Attr(d, "id")] = p.exData(d)
			}
		}
	}
	for e := range elements(chart) {
		switch e.TagName().Name() {
		case "title":
			c.title, c.titleText = p.title(e)
		case "legend":
			c.legend = legendPositions[ooxml.Attr(e, "pos")]
			if c.legend == "" {
				c.legend = "top"
			}
			if txPr, ok := child(e, "txPr"); ok {
				c.legendText = p.bodyStyle(txPr)
			}
		case "plotArea":
			p.plotAreaEx(e, data)
		}
	}
}

// exData reads cx:data.
func (p *dmlParser) exData(n roxmltree.Node) exData {
	var d exData
	for dim := range elements(n) {
		name := dim.TagName().Name()
		texts, nums, format := p.exLevels(dim)
		if len(texts) == 0 {
			continue
		}
		switch typ := ooxml.Attr(dim, "type"); {
		case name == "strDim" && d.cats == nil:
			d.cats, d.levels = texts[0], texts
		case name == "numDim" && (typ == "val" || typ == "size" || typ == "y") && d.vals == nil:
			d.format, d.vals = format, nums[0]
		case name == "numDim" && d.cats == nil:
			d.cats = texts[0]
		}
	}
	return d
}

// exLevels returns the levels of a cx:strDim or a cx:numDim, from the
// innermost: the texts of their points, their numbers, or NaN, and the
// format of the numbers. The levels come from the caches, cx:lvl, or from
// the cells of the reference cx:f when the dimension has no cache.
func (p *dmlParser) exLevels(dim roxmltree.Node) (texts [][]string, nums [][]float64, format string) {
	for lvl := range elements(dim) {
		if lvl.TagName().Name() != "lvl" || len(texts) >= maxLevels {
			continue
		}
		count := p.exCount(ooxml.Attr(lvl, "ptCount"))
		level := make([]string, count)
		for pt := range elements(lvl) {
			if pt.TagName().Name() != "pt" {
				continue
			}
			i, err := strconv.Atoi(ooxml.Attr(pt, "idx"))
			if err != nil || i < 0 || i >= count {
				continue
			}
			level[i] = text(pt)
		}
		if format == "" {
			format = ooxml.Attr(lvl, "formatCode")
		}
		values := make([]float64, count)
		for i, t := range level {
			values[i] = math.NaN()
			if v, ok := parseNumber(t); ok {
				values[i] = v
			}
		}
		texts, nums = append(texts, level), append(nums, values)
	}
	if len(texts) > 0 || p.srcCells == nil {
		return texts, nums, format
	}
	f, ok := child(dim, "f")
	if !ok {
		return nil, nil, ""
	}
	ref := strings.TrimSpace(text(f))
	if p.srcFormat != nil {
		format = p.srcFormat(ref)
	}
	cells := p.boundCells(p.srcCells(ref))
	return cellLevels(cells), cellValues(cells), format
}

// exCount returns the number of points of a level, from its ptCount,
// within the budget of the chart.
func (p *dmlParser) exCount(s string) int {
	count := 0
	if v, err := strconv.Atoi(s); err == nil && v > 0 {
		count = v
	}
	limit := max(0, min(p.budget, maxCategories*4))
	if count > limit {
		p.c.truncated = true
		count = limit
	}
	p.budget -= count
	return count
}

// cellGrid returns the cells of a reference as levels of points, from the
// innermost. A range with at least as many rows as columns holds a point
// in each row, and a level in each column, the last column innermost, as
// Excel reads it. A wider range holds a point in each column.
func cellGrid[T any](rows [][]Cell, of func(Cell) T) [][]T {
	if len(rows) == 0 {
		return nil
	}
	cols := 0
	for _, r := range rows {
		cols = max(cols, len(r))
	}
	at := func(r, c int) Cell {
		if c < len(rows[r]) {
			return rows[r][c]
		}
		return Cell{}
	}
	var out [][]T
	if len(rows) >= cols {
		for c := cols - 1; c >= 0 && len(out) < maxLevels; c-- {
			level := make([]T, len(rows))
			for r := range rows {
				level[r] = of(at(r, c))
			}
			out = append(out, level)
		}
		return out
	}
	for r := len(rows) - 1; r >= 0 && len(out) < maxLevels; r-- {
		level := make([]T, cols)
		for c := range cols {
			level[c] = of(at(r, c))
		}
		out = append(out, level)
	}
	return out
}

// cellLevels returns the texts of the cells of a reference, as levels.
func cellLevels(rows [][]Cell) [][]string {
	return cellGrid(rows, func(c Cell) string {
		if len(c.Text) > maxText {
			return c.Text[:maxText]
		}
		return c.Text
	})
}

// cellValues returns the numbers of the cells of a reference, or NaN, as
// levels.
func cellValues(rows [][]Cell) [][]float64 {
	return cellGrid(rows, func(c Cell) float64 {
		if c.Number && !math.IsNaN(c.Value) && !math.IsInf(c.Value, 0) {
			return c.Value
		}
		if v, ok := parseNumber(c.Text); ok {
			return v
		}
		return math.NaN()
	})
}

// boundCells bounds the cells of a reference by the limits of the chart,
// and takes their number from its budget.
func (p *dmlParser) boundCells(rows [][]Cell) [][]Cell {
	limit := max(0, min(p.budget, maxCategories*4))
	if len(rows) > limit {
		p.c.truncated = true
		rows = rows[:limit]
	}
	out := make([][]Cell, 0, len(rows))
	for _, r := range rows {
		if p.budget <= 0 {
			p.c.truncated = true
			break
		}
		if len(r) > limit {
			p.c.truncated = true
			r = r[:limit]
		}
		out = append(out, r)
		p.budget -= max(1, len(r))
	}
	return out
}

// plotAreaEx reads cx:plotArea: its series and its axes.
func (p *dmlParser) plotAreaEx(n roxmltree.Node, data map[string]exData) {
	c := p.c
	for e := range elements(n) {
		switch e.TagName().Name() {
		case "plotAreaRegion":
			for s := range elements(e) {
				if s.TagName().Name() == "series" {
					p.seriesEx(s, data)
				}
			}
		case "axis":
			a := p.axisEx(e)
			if _, ok := child(e, "valScaling"); ok {
				c.val = a
			} else if !c.cat.present {
				c.cat = a
			}
		}
	}
	for _, pl := range c.plots {
		if pl.kind == kindBar {
			if gap, ok := p.catGap(n); ok {
				pl.gap = gap
			}
		}
		if pl.special == "funnel" {
			// A funnel draws its first category at the top.
			c.cat.reverse = true
		}
	}
	if c.cat.betweenSet || len(c.plots) == 0 {
		return
	}
	c.cat.between, c.cat.betweenSet = true, true
}

// catGap returns the gap of cx:catScaling of the axes of n, in percent.
func (p *dmlParser) catGap(n roxmltree.Node) (float64, bool) {
	for e := range elements(n) {
		if e.TagName().Name() != "axis" {
			continue
		}
		if cs, ok := child(e, "catScaling"); ok {
			if g, ok := parseNumber(ooxml.Attr(cs, "gapWidth")); ok && g >= 0 && g <= 5 {
				return g * 100, true
			}
		}
	}
	return 0, false
}

// axisEx reads cx:axis.
func (p *dmlParser) axisEx(n roxmltree.Node) axis {
	a := newAxis()
	a.present = true
	a.labelPos = "none"
	a.deleted = ooxml.Attr(n, "hidden") == "1" || ooxml.Attr(n, "hidden") == "true"
	for e := range elements(n) {
		switch e.TagName().Name() {
		case "valScaling":
			if v, ok := parseNumber(ooxml.Attr(e, "min")); ok {
				a.min = v
			}
			if v, ok := parseNumber(ooxml.Attr(e, "max")); ok {
				a.max = v
			}
			if v, ok := parseNumber(ooxml.Attr(e, "majorUnit")); ok && v > 0 {
				a.major = v
			}
		case "title":
			lines, style := p.title(e)
			a.title = strings.Join(lines, " ")
			a.titleText = style
		case "majorGridlines":
			a.grid = true
		case "minorGridlines":
			a.minorGrid = true
		case "tickLabels":
			a.labelPos = "low"
		case "numFmt":
			if ooxml.Attr(e, "sourceLinked") != "1" {
				a.format = ooxml.Attr(e, "formatCode")
			}
		case "spPr":
			col, ok, none, _, _ := p.lineOf(e)
			a.line, a.hasLine, a.noLine = col, ok, none
		case "txPr":
			a.text = p.bodyStyle(e)
		}
	}
	return a
}

// seriesEx reads cx:series, and adds its plot to the chart.
func (p *dmlParser) seriesEx(n roxmltree.Node, data map[string]exData) {
	c := p.c
	layout := ooxml.Attr(n, "layoutId")
	if hidden := ooxml.Attr(n, "hidden"); hidden == "1" || hidden == "true" {
		// a hidden series draws nothing, and the legend leaves it out
		return
	}
	s := &series{}
	for _, pl := range c.plots {
		s.index += len(pl.series)
	}
	var d exData
	var layoutPr roxmltree.Node
	hasLayout := false
	for e := range elements(n) {
		switch e.TagName().Name() {
		case "tx":
			s.name = p.seriesName(e)
		case "dataId":
			d = data[ooxml.Attr(e, "val")]
		case "spPr":
			if col, ok, none := fillColor(e, p.colors); ok {
				s.color, s.hasColor = col, true
			} else {
				s.noFill = none
			}
			s.line, s.hasLine, s.noLine, s.lineWidth, s.dash = p.lineOf(e)
		case "dataPt":
			p.point(e, s)
		case "dataLabels":
			s.labels = p.labelsEx(e)
		case "layoutPr":
			layoutPr, hasLayout = e, true
		}
	}
	if len(s.name) > maxText {
		s.name = s.name[:maxText]
	}
	s.cats, s.vals, s.format = d.cats, d.vals, d.format
	pl := &plot{kind: kindBar, grouping: "clustered", gap: 150, overlap: math.NaN(), bubbleScale: 100}
	switch layout {
	case "funnel":
		// A funnel draws its first series only, as Excel and PowerPoint
		// do, with the labels that the series asks for.
		for _, other := range c.plots {
			if other.special == "funnel" {
				return
			}
		}
		pl.horizontal, pl.special, pl.gap = true, "funnel", 6
	case "waterfall":
		pl.special, pl.gap = "waterfall", 50
		if hasLayout {
			for st := range layoutPr.Descendants() {
				if st.IsElement() && st.TagName().Name() == "idx" && len(s.totals) < maxPoints {
					if v, err := strconv.Atoi(ooxml.Attr(st, "val")); err == nil && v >= 0 {
						if s.totals == nil {
							s.totals = map[int]bool{}
						}
						s.totals[v] = true
					}
				}
			}
		}
	case "clusteredColumn":
		if hasLayout {
			if bin, ok := child(layoutPr, "binning"); ok {
				s.cats, s.vals = histogram(d.vals, bin)
				s.format = "General"
				pl.gap = 0
			} else if _, ok := child(layoutPr, "aggregation"); ok {
				s.cats, s.vals = aggregate(s.cats, s.vals)
			}
		}
	case "paretoLine":
		owner := ooxml.Attr(n, "ownerIdx")
		i, err := strconv.Atoi(owner)
		if err != nil || i < 0 || i >= len(c.plots) || len(c.plots[i].series) == 0 {
			return
		}
		bars := c.plots[i]
		pareto(bars.series[0])
		bars.special = "pareto"
		line := &series{index: s.index, name: s.name, cats: bars.series[0].cats, format: "0%"}
		line.vals = cumulative(bars.series[0].vals)
		line.color, line.hasColor = s.color, s.hasColor
		line.noMarker = true
		c.val2 = newAxis()
		c.val2.present, c.val2.min, c.val2.max, c.val2.format, c.val2.position = true, 0, 1, "0%", "r"
		c.val2.labelPos = "nextTo"
		c.plots = append(c.plots, &plot{kind: kindLine, grouping: "standard", gap: 150, overlap: math.NaN(),
			secondary: true, series: []*series{line}, bubbleScale: 100})
		return
	case "boxWhisker":
		pl.kind, pl.gap = kindBox, 100
		if len(c.plots) > 0 && c.plots[len(c.plots)-1].kind == kindBox {
			last := c.plots[len(c.plots)-1]
			if len(last.series) < maxSeries {
				last.series = append(last.series, s)
			}
			return
		}
	case "treemap", "sunburst":
		// A tree draws its first series only, as Excel does.
		if len(c.plots) > 0 {
			return
		}
		pl.kind, pl.special, pl.parents = kindTree, layout, "overlapping"
		pl.tree = buildTree(d.levels, d.vals)
		pl.labelColor = s.labels.text.hasColor
		if hasLayout {
			if pll, ok := child(layoutPr, "parentLabelLayout"); ok {
				switch v := ooxml.Attr(pll, "val"); v {
				case "banner", "overlapping", "none":
					pl.parents = v
				}
			}
		}
	case "regionMap":
		if c.unsupported == "" {
			c.unsupported = "region map"
		}
		return
	default:
		if c.unsupported == "" && layout != "" {
			c.unsupported = layout + " chart"
		}
		return
	}
	if len(c.plots) >= 16 {
		return
	}
	pl.series = []*series{s}
	c.plots = append(c.plots, pl)
	c.unsupported = ""
}

// labelsEx reads cx:dataLabels.
func (p *dmlParser) labelsEx(n roxmltree.Node) labels {
	l := labels{set: true, value: true, position: ooxml.Attr(n, "pos")}
	for e := range elements(n) {
		switch e.TagName().Name() {
		case "visibility":
			l.value = ooxml.Attr(e, "value") != "0"
			l.seriesOf = ooxml.Attr(e, "seriesName") == "1"
			l.category = ooxml.Attr(e, "categoryName") == "1"
		case "numFmt":
			l.format = ooxml.Attr(e, "formatCode")
		case "txPr":
			l.text = p.bodyStyle(e)
		case "separator":
			l.separator, l.hasSep = text(e), true
		}
	}
	return l
}

// histogram returns the bins of the values of a histogram and the number
// of the values in each bin, as cx:binning gives them: a width, a number of
// bins, or the width of the rule of Scott.
func histogram(vals []float64, bin roxmltree.Node) ([]string, []float64) {
	var xs []float64
	for _, v := range vals {
		if !math.IsNaN(v) {
			xs = append(xs, v)
		}
	}
	if len(xs) == 0 {
		return nil, nil
	}
	slices.Sort(xs)
	lo, hi := xs[0], xs[len(xs)-1]
	width := 0.0
	for e := range elements(bin) {
		v, ok := parseNumber(ooxml.Attr(e, "val"))
		switch e.TagName().Name() {
		case "binSize":
			if ok && v > 0 {
				width = v
			}
		case "binCount":
			if ok && v >= 1 && v <= maxBins {
				width = (hi - lo) / math.Round(v)
			}
		}
	}
	if !(width > 0) {
		mean, sd := 0.0, 0.0
		for _, x := range xs {
			mean += x
		}
		mean /= float64(len(xs))
		for _, x := range xs {
			sd += (x - mean) * (x - mean)
		}
		sd = math.Sqrt(sd / float64(len(xs)))
		width = 3.5 * sd / math.Cbrt(float64(len(xs)))
	}
	n := 1
	if width > 0 && hi > lo {
		n = int(math.Ceil((hi - lo) / width))
	}
	n = max(1, min(n, maxBins))
	if hi <= lo || !(width > 0) {
		width = 1
	}
	counts := make([]float64, n)
	for _, x := range xs {
		i := int(math.Ceil((x-lo)/width)) - 1
		counts[max(0, min(i, n-1))]++
	}
	cats := make([]string, n)
	for i := range n {
		a, b := lo+float64(i)*width, lo+float64(i+1)*width
		open := "("
		if i == 0 {
			open = "["
		}
		cats[i] = open + formatValue(round(a), "General") + ", " + formatValue(round(b), "General") + "]"
	}
	return cats, counts
}

// round rounds v to six significant digits, so that a bin of a histogram
// has a short label.
func round(v float64) float64 {
	if v == 0 {
		return 0
	}
	k := math.Pow(10, 5-math.Floor(math.Log10(math.Abs(v))))
	return math.Round(v*k) / k
}

// aggregate sums the values of each category, in the order of their first
// value.
func aggregate(cats []string, vals []float64) ([]string, []float64) {
	var outCats []string
	var outVals []float64
	at := map[string]int{}
	for i, v := range vals {
		name := ""
		if i < len(cats) {
			name = cats[i]
		}
		j, ok := at[name]
		if !ok {
			j = len(outCats)
			at[name] = j
			outCats = append(outCats, name)
			outVals = append(outVals, 0)
		}
		if !math.IsNaN(v) {
			outVals[j] += v
		}
	}
	return outCats, outVals
}

// pareto sorts the categories of a series from the largest value to the
// smallest, as the bars of a Pareto chart.
func pareto(s *series) {
	idx := make([]int, len(s.vals))
	for i := range idx {
		idx[i] = i
	}
	slices.SortStableFunc(idx, func(a, b int) int {
		va, vb := s.vals[a], s.vals[b]
		switch {
		case va > vb || (!math.IsNaN(va) && math.IsNaN(vb)):
			return -1
		case va < vb || (math.IsNaN(va) && !math.IsNaN(vb)):
			return 1
		}
		return 0
	})
	vals := make([]float64, len(idx))
	cats := make([]string, len(idx))
	for i, j := range idx {
		vals[i] = s.vals[j]
		if j < len(s.cats) {
			cats[i] = s.cats[j]
		}
	}
	s.vals, s.cats = vals, cats
}

// cumulative returns the running share of the total of the values.
func cumulative(vals []float64) []float64 {
	total := 0.0
	for _, v := range vals {
		if !math.IsNaN(v) {
			total += v
		}
	}
	out := make([]float64, len(vals))
	sum := 0.0
	for i, v := range vals {
		if !math.IsNaN(v) {
			sum += v
		}
		out[i] = math.NaN()
		if total != 0 {
			out[i] = sum / total
		}
	}
	return out
}
