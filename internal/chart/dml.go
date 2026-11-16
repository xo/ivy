package chart

import (
	"fmt"
	"image/color"
	"iter"
	"math"
	"strconv"
	"strings"

	"github.com/xo/okage/roxmltree"

	"github.com/xo/ivy"
	"github.com/xo/ivy/internal/drawingml"
	"github.com/xo/ivy/internal/ooxml"
)

// Parse reads a chart part of DrawingML, the part with the root
// c:chartSpace (ECMA-376 Part 1, 21.2.2.29). It returns an error that wraps
// ivy.ErrFormat for a part that is not a chart, and one that wraps
// ivy.ErrLimit for a part with more nodes than the limits of
// opts.Config.
func Parse(buf []byte, opts Options) (c *Chart, err error) {
	defer func() {
		if r := recover(); r != nil {
			c, err = nil, fmt.Errorf("reading chart: panic: %v: %w", r, ivy.ErrFormat)
		}
	}()
	lim := opts.Config.Limits.WithDefaults()
	doc, err := ooxml.ParseXML(buf, lim)
	if err != nil {
		return nil, fmt.Errorf("reading chart: %w", err)
	}
	root := doc.RootElement()
	if name := root.TagName().Name(); name != "chartSpace" {
		return nil, fmt.Errorf("chart with the root %q: %w", name, ivy.ErrFormat)
	}
	chart, ok := child(root, "chart")
	if !ok {
		return nil, fmt.Errorf("chart space without c:chart: %w", ivy.ErrFormat)
	}
	c = newChart(opts)
	theme := withOffice(opts.Theme)
	p := &dmlParser{c: c, theme: theme, colors: &drawingml.Colors{Theme: theme}, budget: min(lim.MaxObjects, maxPoints)}
	c.font = p.fontName("+mn-lt")
	if txPr, ok := child(root, "txPr"); ok {
		c.text = p.bodyStyle(txPr).merge(c.text)
	}
	if c.text.font != "" {
		c.font = c.text.font
	}
	if spPr, ok := child(root, "spPr"); ok {
		c.fill, c.border = p.shapeColors(spPr)
	}
	for i := range 36 {
		c.colors = append(c.colors, accent(theme, i))
	}
	p.chart(chart)
	c.finish()
	return c, nil
}

// newChart returns a chart with the defaults of Office.
func newChart(opts Options) *Chart {
	return &Chart{
		cat: newAxis(), val: newAxis(),
		blanks: "gap",
		text:   textStyle{size: 10, color: textGray, hasColor: true},
		width:  max(0, opts.Width), height: max(0, opts.Height),
		fonts: opts.Fonts,
		cfg:   opts.Config,
	}
}

// dmlParser holds the state of one call of Parse.
type dmlParser struct {
	c      *Chart
	theme  *drawingml.Theme
	colors *drawingml.Colors
	// budget is the number of points that the chart can still read.
	budget int
}

// chart reads c:chart.
func (p *dmlParser) chart(n roxmltree.Node) {
	c := p.c
	autoDeleted := false
	var title roxmltree.Node
	hasTitle := false
	for e := range elements(n) {
		switch e.TagName().Name() {
		case "title":
			title, hasTitle = e, true
		case "autoTitleDeleted":
			autoDeleted = boolVal(e)
		case "plotArea":
			p.plotArea(e)
		case "legend":
			c.legend = "right"
			for l := range elements(e) {
				switch l.TagName().Name() {
				case "legendPos":
					c.legend = map[string]string{
						"r": "right", "l": "left", "t": "top", "b": "bottom", "tr": "top-right",
					}[ooxml.Attr(l, "val")]
					if c.legend == "" {
						c.legend = "right"
					}
				case "txPr":
					c.legendText = p.bodyStyle(l)
				}
			}
		case "dispBlanksAs":
			if v := ooxml.Attr(e, "val"); v == "gap" || v == "zero" || v == "span" {
				c.blanks = v
			}
		}
	}
	if hasTitle {
		c.title, c.titleText = p.title(title)
		if c.title == nil && !autoDeleted {
			c.title = []string{"Chart Title"}
			if len(c.plots) == 1 && len(c.plots[0].series) == 1 && c.plots[0].series[0].name != "" {
				c.title = []string{c.plots[0].series[0].name}
			}
		}
	}
	if c.titleText.size <= 0 {
		c.titleText.size = math.Round(c.text.size * 1.4)
	}
	c.titleText = c.titleText.merge(c.text)
	c.legendText = c.legendText.merge(textStyle{size: math.Round(c.text.size * 0.9)}).merge(c.text)
}

// title reads the text of c:title, from rich text or from the cache of a
// reference, and its style. It returns nil for a title without text.
func (p *dmlParser) title(n roxmltree.Node) ([]string, textStyle) {
	var lines []string
	var style textStyle
	if txPr, ok := child(n, "txPr"); ok {
		style = p.bodyStyle(txPr)
	}
	tx, ok := child(n, "tx")
	if !ok {
		return nil, style
	}
	if rich, ok := child(tx, "rich"); ok {
		style = p.bodyStyle(rich).merge(style)
		for para := range elements(rich) {
			if para.TagName().Name() != "p" {
				continue
			}
			var b strings.Builder
			for r := range elements(para) {
				switch r.TagName().Name() {
				case "r", "fld":
					if rPr, ok := child(r, "rPr"); ok && style.size <= 0 {
						style = p.runStyle(rPr).merge(style)
					}
					if t, ok := child(r, "t"); ok {
						b.WriteString(text(t))
					}
				case "br":
					lines = append(lines, b.String())
					b.Reset()
				}
			}
			lines = append(lines, b.String())
		}
	} else {
		lines = []string{p.seriesName(tx)}
	}
	for _, l := range lines {
		if strings.TrimSpace(l) != "" {
			return clip(lines), style
		}
	}
	return nil, style
}

// clip bounds the length of each line of a text.
func clip(lines []string) []string {
	for i, l := range lines {
		if len(l) > maxText {
			lines[i] = l[:maxText]
		}
	}
	if len(lines) > 16 {
		lines = lines[:16]
	}
	return lines
}

// bodyStyle reads the style of the first paragraph of a text body, such as
// c:txPr or c:rich: a:p/a:pPr/a:defRPr.
func (p *dmlParser) bodyStyle(n roxmltree.Node) textStyle {
	for para := range elements(n) {
		if para.TagName().Name() != "p" {
			continue
		}
		if pPr, ok := child(para, "pPr"); ok {
			if def, ok := child(pPr, "defRPr"); ok {
				return p.runStyle(def)
			}
		}
		return textStyle{}
	}
	return textStyle{}
}

// runStyle reads the properties of a run, a:rPr or a:defRPr.
func (p *dmlParser) runStyle(n roxmltree.Node) textStyle {
	var s textStyle
	if sz, ok := parseNumber(ooxml.Attr(n, "sz")); ok && sz >= 100 && sz <= 400000 {
		s.size = sz / 100
	}
	s.bold = ooxml.Attr(n, "b") == "1" || ooxml.Attr(n, "b") == "true"
	if c, ok, _ := fillColor(n, p.colors); ok {
		s.color, s.hasColor = c, true
	}
	if latin, ok := child(n, "latin"); ok {
		s.font = p.fontName(ooxml.Attr(latin, "typeface"))
	}
	return s
}

// fontName returns the family of a typeface. The names +mn-lt and +mj-lt
// are the minor and the major fonts of the theme.
func (p *dmlParser) fontName(typeface string) string {
	var f string
	switch typeface {
	case "+mn-lt", "+mn-ea", "+mn-cs":
		if p.theme != nil {
			f = p.theme.Minor.Latin
		}
		if f == "" {
			f = "Calibri"
		}
	case "+mj-lt", "+mj-ea", "+mj-cs":
		if p.theme != nil {
			f = p.theme.Major.Latin
		}
		if f == "" {
			f = "Calibri Light"
		}
	default:
		f = typeface
	}
	if len(f) > 128 {
		return ""
	}
	return f
}

// shapeColors reads the fill and the line of c:spPr. A missing or a noFill
// color is transparent.
func (p *dmlParser) shapeColors(n roxmltree.Node) (fill, line color.NRGBA) {
	if c, ok, _ := fillColor(n, p.colors); ok {
		fill = c
	}
	if ln, ok := child(n, "ln"); ok {
		if c, ok, _ := fillColor(ln, p.colors); ok {
			line = c
		}
	}
	return fill, line
}

// plotArea reads c:plotArea: its axes, then its charts, so that the
// categories take the number format of their axis.
func (p *dmlParser) plotArea(n roxmltree.Node) {
	c := p.c
	axes := map[string]axis{}
	var axisIDs []string
	for e := range elements(n) {
		switch name := e.TagName().Name(); {
		case name == "catAx" || name == "valAx" || name == "dateAx" || name == "serAx":
			id := ""
			if a, ok := child(e, "axId"); ok {
				id = ooxml.Attr(a, "val")
			}
			if _, ok := axes[id]; !ok && len(axes) < 16 {
				axes[id] = p.axis(e)
			}
		case plotKinds[name] != 0 && axisIDs == nil:
			axisIDs = []string{}
			for a := range elements(e) {
				if a.TagName().Name() == "axId" {
					axisIDs = append(axisIDs, ooxml.Attr(a, "val"))
				}
			}
		}
	}
	if len(axisIDs) > 0 {
		if a, ok := axes[axisIDs[0]]; ok {
			c.cat = a
		}
	}
	if len(axisIDs) > 1 {
		if a, ok := axes[axisIDs[1]]; ok {
			c.val = a
		}
	}
	for e := range elements(n) {
		pl, unsupported := p.plot(e)
		if unsupported != "" && c.unsupported == "" && len(c.plots) == 0 {
			c.unsupported = unsupported
		}
		if pl == nil || len(c.plots) >= 16 {
			continue
		}
		c.plots = append(c.plots, pl)
	}
	if len(c.plots) > 0 {
		c.unsupported = ""
	}
	c.cat.text = c.cat.text.merge(textStyle{size: math.Round(c.text.size * 0.9)}).merge(c.text)
	c.val.text = c.val.text.merge(textStyle{size: math.Round(c.text.size * 0.9)}).merge(c.text)
	c.cat.titleText = c.cat.titleText.merge(c.text)
	c.val.titleText = c.val.titleText.merge(c.text)
}

// axis reads c:catAx, c:valAx, c:dateAx or c:serAx.
func (p *dmlParser) axis(n roxmltree.Node) axis {
	a := newAxis()
	a.present = true
	a.date = n.TagName().Name() == "dateAx"
	for e := range elements(n) {
		switch e.TagName().Name() {
		case "delete":
			a.deleted = boolVal(e)
		case "axPos":
			a.position = ooxml.Attr(e, "val")
		case "title":
			lines, style := p.title(e)
			a.title = strings.Join(lines, " ")
			a.titleText = style
		case "numFmt":
			if ooxml.Attr(e, "sourceLinked") != "1" && ooxml.Attr(e, "sourceLinked") != "true" {
				a.format = ooxml.Attr(e, "formatCode")
			}
		case "majorGridlines":
			a.grid = true
		case "majorUnit":
			if v, ok := parseNumber(ooxml.Attr(e, "val")); ok && v > 0 {
				a.major = v
			}
		case "scaling":
			for s := range elements(e) {
				v, ok := parseNumber(ooxml.Attr(s, "val"))
				switch s.TagName().Name() {
				case "orientation":
					a.reverse = ooxml.Attr(s, "val") == "maxMin"
				case "min":
					if ok {
						a.min = v
					}
				case "max":
					if ok {
						a.max = v
					}
				}
			}
		case "txPr":
			a.text = p.bodyStyle(e)
		}
	}
	return a
}

// plotKinds maps the element of a chart type to the kind of plot that draws
// it.
var plotKinds = map[string]kind{
	"barChart": kindBar, "bar3DChart": kindBar,
	"lineChart": kindLine, "line3DChart": kindLine, "stockChart": kindLine, "radarChart": kindLine,
	"areaChart": kindArea, "area3DChart": kindArea,
	"pieChart": kindPie, "pie3DChart": kindPie, "doughnutChart": kindPie, "ofPieChart": kindPie,
	"scatterChart": kindScatter, "bubbleChart": kindScatter,
}

// plot reads one chart type of a plot area. It returns nil for an element
// that is not a chart, and the name of a chart type that draws as a frame.
func (p *dmlParser) plot(n roxmltree.Node) (*plot, string) {
	name := n.TagName().Name()
	k, ok := plotKinds[name]
	if !ok {
		if strings.HasPrefix(name, "surface") {
			return nil, "surface chart"
		}
		return nil, ""
	}
	pl := &plot{kind: k, gap: 150, markers: true, varyColors: k == kindPie}
	switch name {
	case "barChart", "bar3DChart":
		pl.grouping = "clustered"
	case "line3DChart":
		pl.markers = false
	case "doughnutChart":
		pl.hole = 0.5
	case "bubbleChart":
		pl.bubbles = true
	}
	for e := range elements(n) {
		v := ooxml.Attr(e, "val")
		switch e.TagName().Name() {
		case "barDir":
			pl.horizontal = v == "bar"
		case "grouping":
			pl.grouping = v
		case "varyColors":
			pl.varyColors = boolVal(e)
		case "gapWidth":
			if g, ok := parseNumber(v); ok && g >= 0 && g <= 500 {
				pl.gap = g
			}
		case "holeSize":
			if h, ok := parseNumber(v); ok && h >= 1 && h <= 90 {
				pl.hole = h / 100
			}
		case "firstSliceAng":
			if a, ok := parseNumber(v); ok && a >= 0 && a <= 360 {
				pl.firstAngle = a
			}
		case "marker":
			pl.markers = boolVal(e)
		case "scatterStyle":
			pl.lines = strings.Contains(strings.ToLower(v), "line") || strings.HasPrefix(v, "smooth")
			pl.smooth = strings.HasPrefix(v, "smooth")
			pl.markers = v != "line" && v != "smooth"
		case "radarStyle":
			if v == "filled" {
				pl.kind = kindArea
			}
		case "dLbls":
			pl.labels = p.labels(e)
		case "ser":
			if len(pl.series) >= maxSeries || p.budget <= 0 {
				p.c.truncated = true
				continue
			}
			pl.series = append(pl.series, p.series(e, pl))
		}
	}
	if pl.kind == kindArea && pl.grouping == "" {
		pl.grouping = "standard"
	}
	if name == "scatterChart" && !pl.lines {
		pl.markers = true
	}
	if name == "bubbleChart" {
		pl.lines = false
	}
	return pl, ""
}

// labels reads c:dLbls.
func (p *dmlParser) labels(n roxmltree.Node) labels {
	l := labels{set: true}
	for e := range elements(n) {
		switch e.TagName().Name() {
		case "delete":
			if boolVal(e) {
				return labels{set: true}
			}
		case "showVal":
			l.value = boolVal(e)
		case "showPercent":
			l.percent = boolVal(e)
		case "showCatName":
			l.category = boolVal(e)
		case "showSerName":
			l.seriesOf = boolVal(e)
		case "numFmt":
			l.format = ooxml.Attr(e, "formatCode")
			if ooxml.Attr(e, "sourceLinked") == "1" {
				l.format = ""
			}
		case "dLblPos":
			l.position = ooxml.Attr(e, "val")
		case "txPr":
			l.text = p.bodyStyle(e)
		}
	}
	return l
}

// series reads c:ser.
func (p *dmlParser) series(n roxmltree.Node, pl *plot) *series {
	s := &series{index: -1}
	var line, markerFill color.NRGBA
	hasLine, hasMarker := false, false
	for e := range elements(n) {
		switch e.TagName().Name() {
		case "idx":
			if v, err := strconv.Atoi(ooxml.Attr(e, "val")); err == nil && v >= 0 {
				s.index = v
			}
		case "tx":
			s.name = p.seriesName(e)
		case "spPr":
			if c, ok, _ := fillColor(e, p.colors); ok {
				s.color, s.hasColor = c, true
			}
			if ln, ok := child(e, "ln"); ok {
				c, ok, none := fillColor(ln, p.colors)
				s.noLine = none
				if ok {
					line, hasLine = c, true
				}
			}
		case "marker":
			for m := range elements(e) {
				switch m.TagName().Name() {
				case "symbol":
					s.noMarker = ooxml.Attr(m, "val") == "none"
				case "spPr":
					if c, ok, _ := fillColor(m, p.colors); ok {
						markerFill, hasMarker = c, true
					}
				}
			}
		case "dPt":
			p.point(e, s)
		case "dLbls":
			s.labels = p.labels(e)
		case "cat", "xVal":
			strs, nums, format, isNum := p.data(e)
			if e.TagName().Name() == "xVal" && isNum {
				s.xs = nums
				continue
			}
			s.cats = p.categories(strs, nums, format, isNum)
		case "val", "yVal":
			_, s.vals, s.format, _ = p.data(e)
		case "bubbleSize":
			_, s.sizes, _, _ = p.data(e)
		case "smooth":
			pl.smooth = pl.smooth || boolVal(e)
		}
	}
	if s.index < 0 {
		s.index = len(pl.series)
	}
	if (pl.kind == kindLine || pl.kind == kindScatter) && hasLine {
		s.color, s.hasColor = line, true
	}
	if !s.hasColor && hasMarker {
		s.color, s.hasColor = markerFill, true
	}
	if !s.hasColor && pl.kind != kindLine && pl.kind != kindScatter && hasLine {
		s.color, s.hasColor = line, true
	}
	if len(s.name) > maxText {
		s.name = s.name[:maxText]
	}
	p.budget -= max(len(s.vals), len(s.cats), len(s.xs))
	return s
}

// point reads c:dPt, the color of one point of a series.
func (p *dmlParser) point(n roxmltree.Node, s *series) {
	idx := -1
	var c color.NRGBA
	ok := false
	for e := range elements(n) {
		switch e.TagName().Name() {
		case "idx":
			if v, err := strconv.Atoi(ooxml.Attr(e, "val")); err == nil && v >= 0 && v < maxPoints {
				idx = v
			}
		case "spPr":
			c, ok, _ = fillColor(e, p.colors)
			if !ok {
				if ln, found := child(e, "ln"); found {
					c, ok, _ = fillColor(ln, p.colors)
				}
			}
		}
	}
	if idx < 0 || !ok || len(s.points) >= maxPoints {
		return
	}
	if s.points == nil {
		s.points = map[int]color.NRGBA{}
	}
	s.points[idx] = c
}

// seriesName reads the name of a series from c:tx: a reference with its
// cache, or c:v.
func (p *dmlParser) seriesName(n roxmltree.Node) string {
	for e := range n.Descendants() {
		if !e.IsElement() {
			continue
		}
		if name := e.TagName().Name(); name == "v" || name == "t" {
			return text(e)
		}
	}
	return ""
}

// data reads the cache of c:cat, c:val, c:xVal, c:yVal or c:bubbleSize:
// c:strRef, c:numRef or c:multiLvlStrRef with their caches, or c:strLit or
// c:numLit. It returns the texts, or the numbers with their format code.
func (p *dmlParser) data(n roxmltree.Node) (strs []string, nums []float64, format string, isNum bool) {
	cache, isNum, ok := p.cache(n)
	if !ok {
		return nil, nil, "", false
	}
	if cache.TagName().Name() == "multiLvlStrCache" {
		// The first level holds the innermost categories.
		count := p.count(cache)
		for lvl := range elements(cache) {
			if lvl.TagName().Name() == "lvl" {
				strs = make([]string, count)
				p.points(lvl, count, func(i int, v string, _ string) { strs[i] = v })
				return strs, nil, "", false
			}
		}
		return nil, nil, "", false
	}
	count := p.count(cache)
	if f, ok := child(cache, "formatCode"); ok {
		format = text(f)
	}
	if !isNum {
		strs = make([]string, count)
		p.points(cache, count, func(i int, v string, _ string) { strs[i] = v })
		return strs, nil, format, false
	}
	nums = make([]float64, count)
	for i := range nums {
		nums[i] = math.NaN()
	}
	formats := map[int]string{}
	p.points(cache, count, func(i int, v string, f string) {
		if x, ok := parseNumber(v); ok {
			nums[i] = x
		}
		if f != "" && len(formats) < 64 {
			formats[i] = f
		}
	})
	if format == "" {
		for _, f := range formats {
			format = f
			break
		}
	}
	return nil, nums, format, true
}

// cache finds the cache of a data source: the cache of a reference, or a
// literal.
func (p *dmlParser) cache(n roxmltree.Node) (roxmltree.Node, bool, bool) {
	for e := range elements(n) {
		switch e.TagName().Name() {
		case "numRef":
			if c, ok := child(e, "numCache"); ok {
				return c, true, true
			}
		case "strRef":
			if c, ok := child(e, "strCache"); ok {
				return c, false, true
			}
		case "multiLvlStrRef":
			if c, ok := child(e, "multiLvlStrCache"); ok {
				return c, false, true
			}
		case "numLit":
			return e, true, true
		case "strLit":
			return e, false, true
		}
	}
	return roxmltree.Node{}, false, false
}

// count returns the number of points of a cache, c:ptCount, within the
// budget of the chart. Without c:ptCount, it is one more than the largest
// index.
func (p *dmlParser) count(cache roxmltree.Node) int {
	n := -1
	if pc, ok := child(cache, "ptCount"); ok {
		if v, err := strconv.Atoi(ooxml.Attr(pc, "val")); err == nil && v >= 0 {
			n = v
		}
	}
	if n < 0 {
		n = 0
		for pt := range cache.Descendants() {
			if pt.IsElement() && pt.TagName().Name() == "pt" {
				if v, err := strconv.Atoi(ooxml.Attr(pt, "idx")); err == nil && v >= n && v < maxPoints {
					n = v + 1
				}
			}
		}
	}
	limit := max(0, min(p.budget, maxCategories*4))
	if n > limit {
		p.c.truncated = true
		n = limit
	}
	return n
}

// points calls fn for each c:pt of n whose index is below count, with its
// value and its format code.
func (p *dmlParser) points(n roxmltree.Node, count int, fn func(i int, v, format string)) {
	for pt := range elements(n) {
		if pt.TagName().Name() != "pt" {
			continue
		}
		i, err := strconv.Atoi(ooxml.Attr(pt, "idx"))
		if err != nil || i < 0 || i >= count {
			continue
		}
		if v, ok := child(pt, "v"); ok {
			s := text(v)
			if len(s) > maxText {
				s = s[:maxText]
			}
			fn(i, s, ooxml.Attr(pt, "formatCode"))
		}
	}
}

// categories returns the labels of the categories: the texts, or the
// numbers in their format, such as the dates of a date axis.
func (p *dmlParser) categories(strs []string, nums []float64, format string, isNum bool) []string {
	if !isNum {
		return strs
	}
	if p.c.cat.format != "" {
		format = p.c.cat.format
	}
	if p.c.cat.date && !isDate(format) {
		format = "m/d/yyyy"
	}
	out := make([]string, len(nums))
	for i, v := range nums {
		out[i] = formatValue(v, format)
	}
	return out
}

// elements returns the child elements of n.
func elements(n roxmltree.Node) iter.Seq[roxmltree.Node] {
	return func(yield func(roxmltree.Node) bool) {
		for c := range n.Children() {
			if c.IsElement() && !yield(c) {
				return
			}
		}
	}
}

// child returns the first child element of n with the local name name.
func child(n roxmltree.Node, name string) (roxmltree.Node, bool) {
	for c := range elements(n) {
		if c.TagName().Name() == name {
			return c, true
		}
	}
	return roxmltree.Node{}, false
}

// text returns the text of the element n and of its descendants.
func text(n roxmltree.Node) string {
	var b strings.Builder
	for d := range n.Descendants() {
		if d.IsText() {
			if t, ok := d.Text(); ok {
				b.WriteString(t)
			}
		}
		if b.Len() > maxText {
			break
		}
	}
	return b.String()
}

// boolVal reads the attribute val of an element of type CT_Boolean, which
// is true when it is missing.
func boolVal(n roxmltree.Node) bool {
	switch ooxml.Attr(n, "val") {
	case "0", "false", "off":
		return false
	}
	return true
}

// parseNumber reads a finite number.
func parseNumber(s string) (float64, bool) {
	v, err := strconv.ParseFloat(strings.TrimSpace(s), 64)
	if err != nil || math.IsNaN(v) || math.IsInf(v, 0) {
		return 0, false
	}
	return v, true
}
