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

// nsChartEx is the namespace of the charts of Office 2016, cx:chartSpace.
const nsChartEx = "http://schemas.microsoft.com/office/drawing/2014/chartex"

// Parse reads a chart part of DrawingML, the part with the root
// c:chartSpace (ECMA-376 Part 1, 21.2.2.29), or the part of a chart of
// Office 2016 with the root cx:chartSpace. It returns an error that wraps
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
	p := &dmlParser{c: c, theme: theme, colors: &drawingml.Colors{Theme: theme}, budget: min(lim.MaxObjects, maxPoints), srcFormat: opts.SourceFormat, srcCells: opts.SourceCells}
	c.font = p.fontName("+mn-lt")
	if txPr, ok := child(root, "txPr"); ok {
		c.text = p.bodyStyle(txPr).merge(c.text)
	}
	if c.text.font != "" {
		c.font = c.text.font
	}
	ns, _ := root.TagName().Namespace()
	if spPr, ok := child(root, "spPr"); ok {
		c.fill, c.border = p.shapeColors(spPr)
	} else if ns == nsChartEx && opts.Style != nil {
		// Word and Excel draw the chart area of the style part.
		c.fill, c.border = p.styleArea(opts.Style(), lim)
	} else if _, ok := child(root, "printSettings"); ok {
		// A chart of a sheet has the settings of its printing, and a white
		// chart area with a border.
		c.fill, c.border = color.NRGBA{255, 255, 255, 255}, gridGray
	}
	for i := range 36 {
		c.colors = append(c.colors, accent(theme, i))
	}
	if ns == nsChartEx {
		c.look = lookModern
		p.chartEx(root, chart)
	} else {
		p.chart(chart)
	}
	c.placeDates()
	c.finish()
	c.styleText()
	return c, nil
}

// dmlParser holds the state of one call of Parse.
type dmlParser struct {
	c      *Chart
	theme  *drawingml.Theme
	colors *drawingml.Colors
	// budget is the number of points that the chart can still read.
	budget int
	// srcFormat is Options.SourceFormat, and srcCells Options.SourceCells.
	srcFormat func(ref string) string
	srcCells  func(ref string) [][]Cell
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
			p.legend(e)
		case "dispBlanksAs":
			if v := ooxml.Attr(e, "val"); v == "gap" || v == "zero" || v == "span" {
				c.blanks = v
			}
		case "view3D":
			if rx, ok := child(e, "rotX"); ok {
				if v, ok := parseNumber(ooxml.Attr(rx, "val")); ok && v >= -90 && v <= 90 {
					c.rotX = math.Abs(v)
				}
			}
		}
	}
	if hasTitle {
		c.title, c.titleText = p.title(title)
		if o, ok := child(title, "overlay"); ok {
			c.titleOver = boolVal(o)
		}
		if box, ok := p.layout(title); ok {
			c.titleAt = [2]float64{box[0], box[1]}
		}
		if c.title == nil && !autoDeleted {
			c.title = []string{"Chart Title"}
			if len(c.plots) == 1 && len(c.plots[0].series) == 1 && c.plots[0].series[0].name != "" {
				c.title = []string{c.plots[0].series[0].name}
			}
		}
	}
}

// legend reads c:legend.
func (p *dmlParser) legend(n roxmltree.Node) {
	c := p.c
	c.legend = "right"
	for l := range elements(n) {
		switch l.TagName().Name() {
		case "legendPos":
			c.legend = legendPositions[ooxml.Attr(l, "val")]
			if c.legend == "" {
				c.legend = "right"
			}
		case "overlay":
			c.legendOver = boolVal(l)
		case "legendEntry":
			idx, del := -1, false
			for e := range elements(l) {
				switch e.TagName().Name() {
				case "idx":
					if v, err := strconv.Atoi(ooxml.Attr(e, "val")); err == nil && v >= 0 && v < maxPoints {
						idx = v
					}
				case "delete":
					del = boolVal(e)
				}
			}
			if idx >= 0 && del && len(c.legendHidden) < maxSeries {
				if c.legendHidden == nil {
					c.legendHidden = map[int]bool{}
				}
				c.legendHidden[idx] = true
			}
		case "layout":
			if box, _, ok := p.manualLayout(l); ok {
				c.legendAt = box
			}
		case "txPr":
			c.legendText = p.bodyStyle(l)
		}
	}
}

// legendPositions maps c:legendPos and the position of a legend of Office
// 2016 to a position of the legend.
var legendPositions = map[string]string{
	"r": "right", "l": "left", "t": "top", "b": "bottom", "tr": "top-right",
}

// layout reads the c:layout of n, and returns its box.
func (p *dmlParser) layout(n roxmltree.Node) ([4]float64, bool) {
	if l, ok := child(n, "layout"); ok {
		box, _, ok := p.manualLayout(l)
		return box, ok
	}
	return nan4(), false
}

// manualLayout reads c:layout/c:manualLayout, the box of an element as
// fractions of the size of the chart. A position of the mode factor moves
// the element from its automatic place, which the drawing does not do, so
// it returns false for it. A box without a size has NaN as its width and
// height. inner says that the box is the inner box of a plot area, without
// the labels of its axes.
func (p *dmlParser) manualLayout(n roxmltree.Node) (box [4]float64, inner, ok bool) {
	ml, found := child(n, "manualLayout")
	if !found {
		return nan4(), false, false
	}
	box = nan4()
	modes := map[string]string{}
	for e := range elements(ml) {
		name := e.TagName().Name()
		v := ooxml.Attr(e, "val")
		switch name {
		case "xMode", "yMode", "wMode", "hMode", "layoutTarget":
			modes[name] = v
		case "x", "y", "w", "h":
			f, ok := parseNumber(v)
			if !ok || f < -1 || f > 2 {
				continue
			}
			box[strings.Index("xywh", name)] = f
		}
	}
	if modes["xMode"] == "factor" || modes["yMode"] == "factor" || math.IsNaN(box[0]) || math.IsNaN(box[1]) {
		return nan4(), false, false
	}
	if modes["wMode"] == "edge" {
		box[2] -= box[0]
	}
	if modes["hMode"] == "edge" {
		box[3] -= box[1]
	}
	return box, modes["layoutTarget"] == "inner", true
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
		first := true
		for para := range elements(rich) {
			if para.TagName().Name() != "p" {
				continue
			}
			var b strings.Builder
			for r := range elements(para) {
				switch r.TagName().Name() {
				case "r", "fld":
					if rPr, ok := child(r, "rPr"); ok && first {
						style = p.runStyle(rPr).merge(style)
						first = false
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
// c:txPr or c:rich: a:p/a:pPr/a:defRPr, and the rotation of a:bodyPr.
func (p *dmlParser) bodyStyle(n roxmltree.Node) textStyle {
	var s textStyle
	if bodyPr, ok := child(n, "bodyPr"); ok {
		if rot, ok := parseNumber(ooxml.Attr(bodyPr, "rot")); ok && rot >= -5400000 && rot <= 5400000 {
			s.rotation, s.hasRotation = -rot/60000, true
		}
	}
	for para := range elements(n) {
		if para.TagName().Name() != "p" {
			continue
		}
		if pPr, ok := child(para, "pPr"); ok {
			if def, ok := child(pPr, "defRPr"); ok {
				r := p.runStyle(def)
				r.rotation, r.hasRotation = s.rotation, s.hasRotation
				return r
			}
		}
		return s
	}
	return s
}

// runStyle reads the properties of a run, a:rPr or a:defRPr.
func (p *dmlParser) runStyle(n roxmltree.Node) textStyle {
	var s textStyle
	if sz, ok := parseNumber(ooxml.Attr(n, "sz")); ok && sz >= 100 && sz <= 400000 {
		s.size = sz / 100
	}
	if b := ooxml.Attr(n, "b"); b != "" {
		s.bold, s.hasBold = b == "1" || b == "true", true
	}
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

// lineOf reads a:ln of c:spPr: its color, whether it is a:noFill, its
// width in points, and its preset dash.
func (p *dmlParser) lineOf(spPr roxmltree.Node) (c color.NRGBA, ok, none bool, width float64, dash string) {
	ln, found := child(spPr, "ln")
	if !found {
		return c, false, false, 0, ""
	}
	if w, wok := parseNumber(ooxml.Attr(ln, "w")); wok && w >= 0 && w <= 20116800 {
		width = w / 12700
	}
	if d, found := child(ln, "prstDash"); found {
		dash = ooxml.Attr(d, "val")
	}
	c, ok, none = fillColor(ln, p.colors)
	return c, ok, none, width, dash
}

// plotArea reads c:plotArea: its axes, then its charts, so that the
// categories take the number format of their axis.
func (p *dmlParser) plotArea(n roxmltree.Node) {
	c := p.c
	axes := map[string]axis{}
	var first []string
	for e := range elements(n) {
		switch name := e.TagName().Name(); {
		case name == "catAx" || name == "valAx" || name == "dateAx" || name == "serAx":
			a := p.axis(e)
			if _, ok := axes[a.id]; !ok && len(axes) < 16 {
				axes[a.id] = a
			}
		case plotKinds[name] != 0 && first == nil:
			first = axisIDs(e)
		case name == "spPr":
			c.plotFill, c.plotBorder = p.shapeColors(e)
		case name == "dTable":
			c.table = p.dataTable(e)
		case name == "layout":
			if box, inner, ok := p.manualLayout(e); ok {
				c.plotAt, c.plotInner = box, inner
			}
		}
	}
	pair := func(ids []string) (axis, axis, bool) {
		if len(ids) < 2 {
			return axis{}, axis{}, false
		}
		a, ok0 := axes[ids[0]]
		b, ok1 := axes[ids[1]]
		return a, b, ok0 && ok1
	}
	if a, b, ok := pair(first); ok {
		c.cat, c.val = a, b
	}
	for e := range elements(n) {
		pl, unsupported := p.plot(e)
		if unsupported != "" && c.unsupported == "" && len(c.plots) == 0 {
			c.unsupported = unsupported
		}
		if pl == nil || len(c.plots) >= 16 {
			continue
		}
		ids := axisIDs(e)
		if len(first) >= 2 && len(ids) >= 2 && (ids[0] != first[0] || ids[1] != first[1]) {
			if a, b, ok := pair(ids); ok {
				pl.secondary = true
				if !c.val2.present {
					c.cat2, c.val2 = a, b
				}
			}
		}
		c.plots = append(c.plots, pl)
	}
	if len(c.plots) > 0 {
		c.unsupported = ""
	}
	// The axis of values tells how it crosses the axis of categories.
	if c.val.betweenSet {
		c.cat.between, c.cat.betweenSet = c.val.between, true
	}
	if c.val2.betweenSet {
		c.cat2.between, c.cat2.betweenSet = c.val2.between, true
	}
}

// axisIDs returns the c:axId of a chart type.
func axisIDs(n roxmltree.Node) []string {
	ids := []string{}
	for a := range elements(n) {
		if a.TagName().Name() == "axId" && len(ids) < 4 {
			ids = append(ids, ooxml.Attr(a, "val"))
		}
	}
	return ids
}

// dispUnits are the built-in units of an axis, c:builtInUnit.
var dispUnits = map[string]float64{
	"hundreds": 100, "thousands": 1e3, "tenThousands": 1e4, "hundredThousands": 1e5,
	"millions": 1e6, "tenMillions": 1e7, "hundredMillions": 1e8, "billions": 1e9, "trillions": 1e12,
}

// axis reads c:catAx, c:valAx, c:dateAx or c:serAx.
func (p *dmlParser) axis(n roxmltree.Node) axis {
	a := newAxis()
	a.present = true
	a.date = n.TagName().Name() == "dateAx"
	for e := range elements(n) {
		v := ooxml.Attr(e, "val")
		switch e.TagName().Name() {
		case "axId":
			a.id = v
		case "baseTimeUnit":
			a.baseUnit = v
		case "crossAx":
			a.cross = v
		case "delete":
			a.deleted = boolVal(e)
		case "axPos":
			a.position = v
		case "title":
			lines, style := p.title(e)
			a.title = strings.Join(lines, " ")
			a.titleText = style
		case "numFmt":
			if ooxml.Attr(e, "sourceLinked") != "1" && ooxml.Attr(e, "sourceLinked") != "true" {
				a.format = ooxml.Attr(e, "formatCode")
			}
		case "majorGridlines", "minorGridlines":
			major := e.TagName().Name() == "majorGridlines"
			var col color.NRGBA
			ok, none := false, false
			if spPr, found := child(e, "spPr"); found {
				col, ok, none, _, _ = p.lineOf(spPr)
			}
			if major {
				a.grid = !none
				a.gridColor, a.hasGridColor = col, ok
			} else {
				a.minorGrid = !none
				a.minorColor, a.hasMinorColor = col, ok
			}
		case "majorTickMark":
			a.majorTick = v
		case "minorTickMark":
			a.minorTick = v
		case "tickLblPos":
			a.labelPos = v
		case "majorUnit":
			if f, ok := parseNumber(v); ok && f > 0 {
				a.major = f
			}
		case "minorUnit":
			if f, ok := parseNumber(v); ok && f > 0 {
				a.minor = f
			}
		case "crosses":
			a.crosses = v
		case "crossesAt":
			if f, ok := parseNumber(v); ok {
				a.crossesAt = f
			}
		case "crossBetween":
			a.between, a.betweenSet = v != "midCat", true
		case "tickLblSkip":
			if k, err := strconv.Atoi(v); err == nil && k > 0 && k <= maxCategories {
				a.skip = k
			}
		case "dispUnits":
			for u := range elements(e) {
				switch u.TagName().Name() {
				case "builtInUnit":
					a.unit = dispUnits[ooxml.Attr(u, "val")]
				case "custUnit":
					if f, ok := parseNumber(ooxml.Attr(u, "val")); ok && f > 0 {
						a.unit = f
					}
				}
			}
		case "scaling":
			for s := range elements(e) {
				f, ok := parseNumber(ooxml.Attr(s, "val"))
				switch s.TagName().Name() {
				case "orientation":
					a.reverse = ooxml.Attr(s, "val") == "maxMin"
				case "min":
					if ok {
						a.min = f
					}
				case "max":
					if ok {
						a.max = f
					}
				case "logBase":
					if ok && f >= 2 && f <= 1000 {
						a.logBase = f
					}
				}
			}
		case "spPr":
			col, ok, none, _, _ := p.lineOf(e)
			a.line, a.hasLine, a.noLine = col, ok, none
		case "txPr":
			a.text = p.bodyStyle(e)
			a.rotation, a.hasRotation = a.text.rotation, a.text.hasRotation
		}
	}
	return a
}

// plotKinds maps the element of a chart type to the kind of plot that draws
// it.
var plotKinds = map[string]kind{
	"barChart": kindBar, "bar3DChart": kindBar,
	"lineChart": kindLine, "line3DChart": kindLine, "stockChart": kindLine,
	"radarChart": kindRadar,
	"areaChart":  kindArea, "area3DChart": kindArea,
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
	pl := &plot{kind: k, gap: 150, overlap: math.NaN(), markers: true, varyColors: k == kindPie, bubbleScale: 100}
	switch name {
	case "barChart", "bar3DChart":
		pl.grouping = "clustered"
	case "line3DChart":
		pl.markers = false
	case "doughnutChart":
		pl.hole = 0.5
	case "pie3DChart":
		pl.flat3D = true
	case "bubbleChart":
		pl.bubbles = true
	case "stockChart":
		pl.grouping = "standard"
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
		case "overlap":
			if o, ok := parseNumber(v); ok && o >= -100 && o <= 100 {
				pl.overlap = o
			}
		case "holeSize":
			if h, ok := parseNumber(v); ok && h >= 1 && h <= 90 {
				pl.hole = h / 100
			}
		case "firstSliceAng":
			if a, ok := parseNumber(v); ok && a >= 0 && a <= 360 {
				pl.firstAngle = a
			}
		case "bubbleScale":
			if s, ok := parseNumber(v); ok && s >= 0 && s <= 300 {
				pl.bubbleScale = s
			}
		case "marker":
			pl.markers = boolVal(e)
		case "scatterStyle":
			pl.lines = strings.Contains(strings.ToLower(v), "line") || strings.HasPrefix(v, "smooth")
			pl.smooth = strings.HasPrefix(v, "smooth")
			pl.markers = v != "line" && v != "smooth"
		case "radarStyle":
			pl.filled = v == "filled"
			pl.markers = v == "marker"
		case "hiLowLines":
			pl.hiLow = true
		case "upDownBars":
			pl.upDown = true
		case "dropLines":
			drop := p.strokeOf(e)
			pl.drop = &drop
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
		case "dLbl":
			idx, pt := p.pointLabel(e)
			if idx >= 0 && len(l.points) < maxPoints {
				if l.points == nil {
					l.points = map[int]*pointLabel{}
				}
				l.points[idx] = pt
			}
		default:
			p.labelPart(e, &l)
		}
	}
	return l
}

// labelPart reads a part of c:dLbls or c:dLbl into l: what the labels show,
// their format, their place and their style.
func (p *dmlParser) labelPart(e roxmltree.Node, l *labels) {
	switch e.TagName().Name() {
	case "showVal":
		l.value = boolVal(e)
	case "showPercent":
		l.percent = boolVal(e)
	case "showCatName":
		l.category = boolVal(e)
	case "showSerName":
		l.seriesOf = boolVal(e)
	case "separator":
		l.separator, l.hasSep = text(e), true
	case "numFmt":
		l.format = ooxml.Attr(e, "formatCode")
		if ooxml.Attr(e, "sourceLinked") == "1" {
			l.format = ""
		}
	case "dLblPos":
		l.position = ooxml.Attr(e, "val")
	case "txPr":
		l.text = p.bodyStyle(e).merge(l.text)
	}
}

// pointLabel reads c:dLbl, the label of one point, and returns the number
// of the point, or -1.
func (p *dmlParser) pointLabel(n roxmltree.Node) (int, *pointLabel) {
	pt := &pointLabel{labels: labels{set: true}}
	idx := -1
	for e := range elements(n) {
		switch e.TagName().Name() {
		case "idx":
			if v, err := strconv.Atoi(ooxml.Attr(e, "val")); err == nil && v >= 0 && v < maxPoints {
				idx = v
			}
		case "delete":
			pt.deleted = boolVal(e)
		case "layout":
			if ml, ok := child(e, "manualLayout"); ok {
				for m := range elements(ml) {
					v, ok := parseNumber(ooxml.Attr(m, "val"))
					if !ok || v < -2 || v > 2 {
						continue
					}
					switch m.TagName().Name() {
					case "x":
						pt.dx = v
					case "y":
						pt.dy = v
					}
				}
			}
		case "tx":
			lines, style := p.title(n)
			for _, l := range lines {
				pt.lines = append(pt.lines, strings.Split(l, "\n")...)
			}
			pt.hasText = len(pt.lines) > 0
			pt.text = style.merge(pt.text)
		default:
			p.labelPart(e, &pt.labels)
		}
	}
	return idx, pt
}

// series reads c:ser.
func (p *dmlParser) series(n roxmltree.Node, pl *plot) *series {
	s := &series{index: -1}
	var markerFill color.NRGBA
	hasMarker := false
	for e := range elements(n) {
		switch e.TagName().Name() {
		case "idx":
			if v, err := strconv.Atoi(ooxml.Attr(e, "val")); err == nil && v >= 0 {
				s.index = min(v, maxPoints)
			}
		case "tx":
			s.name = p.seriesName(e)
		case "spPr":
			c, ok, none := fillColor(e, p.colors)
			s.noFill = none
			if ok {
				s.color, s.hasColor = c, true
			}
			s.line, s.hasLine, s.noLine, s.lineWidth, s.dash = p.lineOf(e)
		case "marker":
			for m := range elements(e) {
				switch m.TagName().Name() {
				case "symbol":
					s.marker = ooxml.Attr(m, "val")
					s.noMarker = s.marker == "none"
				case "size":
					if v, ok := parseNumber(ooxml.Attr(m, "val")); ok && v >= 2 && v <= 72 {
						s.markerSize = v
					}
				case "spPr":
					if c, ok, _ := fillColor(m, p.colors); ok {
						markerFill, hasMarker = c, true
						s.markerFill, s.hasMarkerFill = c, true
					}
					if c, ok, _, _, _ := p.lineOf(m); ok {
						s.markerLine, s.hasMarkerLine = c, true
					}
				}
			}
		case "explosion":
			if v, ok := parseNumber(ooxml.Attr(e, "val")); ok && v > 0 && v <= 400 {
				s.explode = v / 100
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
			if isNum {
				s.dates, s.dateFormat = nums, format
			}
		case "val", "yVal":
			_, s.vals, s.format, _ = p.data(e)
		case "bubbleSize":
			_, s.sizes, _, _ = p.data(e)
		case "smooth":
			s.smooth = boolVal(e)
		case "trendline":
			if len(s.trends) < maxTrends {
				s.trends = append(s.trends, p.trendline(e))
			}
		case "errBars":
			if len(s.errs) < maxTrends {
				s.errs = append(s.errs, p.errBars(e))
			}
		}
	}
	if pl.kind != kindScatter {
		// the error bars of a chart of categories go along the values
		for i := range s.errs {
			s.errs[i].dir = "y"
		}
	}
	if s.index < 0 {
		s.index = len(pl.series)
	}
	lined := pl.kind == kindLine || pl.kind == kindScatter || (pl.kind == kindRadar && !pl.filled)
	if lined && s.hasLine {
		s.color, s.hasColor = s.line, true
	}
	if !s.hasColor && hasMarker {
		s.color, s.hasColor = markerFill, true
	}
	if !s.hasColor && !lined && s.hasLine && s.noFill {
		s.color, s.hasColor = s.line, true
	}
	if len(s.name) > maxText {
		s.name = s.name[:maxText]
	}
	p.budget -= max(len(s.vals), len(s.cats), len(s.xs))
	return s
}

// point reads c:dPt, the color and the explosion of one point of a series.
func (p *dmlParser) point(n roxmltree.Node, s *series) {
	idx := -1
	var c color.NRGBA
	ok := false
	explode := math.NaN()
	for e := range elements(n) {
		switch e.TagName().Name() {
		case "idx":
			if v, err := strconv.Atoi(ooxml.Attr(e, "val")); err == nil && v >= 0 && v < maxPoints {
				idx = v
			}
		case "explosion":
			if v, vok := parseNumber(ooxml.Attr(e, "val")); vok && v >= 0 && v <= 400 {
				explode = v / 100
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
	if idx < 0 || len(s.points) >= maxPoints || len(s.explodes) >= maxPoints {
		return
	}
	if ok {
		if s.points == nil {
			s.points = map[int]color.NRGBA{}
		}
		s.points[idx] = c
	}
	if !math.IsNaN(explode) {
		if s.explodes == nil {
			s.explodes = map[int]float64{}
		}
		s.explodes[idx] = explode
	}
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
		for i := range nums {
			if f, ok := formats[i]; ok {
				format = f
				break
			}
		}
	}
	if ref, ok := child(n, "numRef"); ok && p.srcFormat != nil {
		if f, ok := child(ref, "f"); ok {
			if code := p.srcFormat(text(f)); code != "" {
				format = code
			}
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
