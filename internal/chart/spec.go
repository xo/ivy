package chart

import (
	"encoding/json"
	"fmt"
	"image/color"
	"maps"
	"math"
	"strconv"
	"strings"
)

// This file converts a chart into a spec of Vega-Lite for zuzu. The spec is
// built as JSON, because the config and most parts of the spec types of
// zuzu are raw JSON.
//
// The rows of the data have short fields: c is the category, s the series,
// v the value, x the value of x of a scatter chart, z the size of a bubble,
// i the number of the category or the point, k the number of the series, p
// the number of the plot, and l the text of the label on the point, which
// the drawing adds after the layout.

// schema is the $schema of the spec, which names the dialect for zuzu.
const schema = "https://vega.github.io/schema/vega-lite/v5.json"

// zws is the zero width space that makes a repeated or an empty category
// name unique, so that each category keeps its own band.
const zws = "\u200b"

// Spec returns the spec of Vega-Lite of the chart, at a size in points, as
// JSON. family is the family of the font of the text.
func (c *Chart) Spec(width, height float64, family string) ([]byte, error) {
	b, err := json.Marshal(c.spec(width, height, family))
	if err != nil {
		return nil, fmt.Errorf("writing chart spec: %w", err)
	}
	return b, nil
}

// obj is an object of JSON.
type obj = map[string]any

// spec returns the spec as values of JSON.
func (c *Chart) spec(width, height float64, family string) obj {
	s := obj{
		"$schema":  schema,
		"width":    round2(width),
		"height":   round2(height),
		"autosize": obj{"type": "fit", "contains": "padding"},
		"padding":  round2(min(8, max(2, min(width, height)/40))),
		"config":   c.config(family),
	}
	if len(c.title) > 0 {
		t := obj{"text": c.title, "anchor": "middle", "fontSize": c.titleText.size,
			"color": css(c.titleText.color), "fontWeight": "normal", "offset": round2(c.titleText.size / 2)}
		if len(c.title) == 1 {
			t["text"] = c.title[0]
		}
		if c.titleText.bold {
			t["fontWeight"] = "bold"
		}
		s["title"] = t
	}
	var layers []obj
	switch {
	case c.unsupported != "" || len(c.plots) == 0:
		layers = []obj{c.frame()}
	case c.plots[0].kind == kindPie:
		layers = []obj{c.pie(c.plots[0], 0)}
	default:
		dom := c.colorDomain()
		for i, p := range c.plots {
			if p.kind == kindPie {
				continue
			}
			layers = append(layers, c.layersOf(p, i, dom, width, height)...)
		}
	}
	if len(layers) == 1 {
		for k, v := range layers[0] {
			s[k] = v
		}
		return s
	}
	list := make([]any, len(layers))
	for i, l := range layers {
		list[i] = l
	}
	s["layer"] = list
	return s
}

// config returns the config of the spec: the fonts and the colors of
// Office, and the rules of the look of zuzu turned off.
func (c *Chart) config(family string) obj {
	axisText := func(a axis) obj {
		return obj{
			"labelFont": family, "labelFontSize": a.text.size, "labelColor": css(a.text.color),
			"titleFont": family, "titleFontSize": a.titleText.size, "titleColor": css(a.titleText.color),
			"titleFontWeight": "normal", "gridColor": css(gridGray), "domainColor": css(axisGray),
			"tickColor": css(axisGray),
		}
	}
	noCorners := obj{"zuzu.cornerRadiusEnd": 0, "zuzu.stackGap": 0, "zuzu.hierarchy": false}
	return obj{
		"font":       family,
		"background": "transparent",
		"view":       obj{"stroke": nil},
		"axis":       axisText(c.val),
		"axisBand":   axisText(c.cat),
		"legend": obj{
			"labelFont": family, "labelFontSize": c.legendText.size, "labelColor": css(c.legendText.color),
			"titleFont": family, "symbolType": "square", "symbolSize": round2(c.legendText.size * c.legendText.size * 0.8),
		},
		"title": obj{"font": family, "subtitleFont": family},
		"style": obj{
			"line":                obj{"zuzu.endLabel": false},
			"arc":                 obj{"zuzu.total": false, "zuzu.hierarchy": false},
			"bar":                 obj{"zuzu.hierarchy": false},
			"rect":                obj{"zuzu.hierarchy": false, "zuzu.sankey": false},
			"point":               obj{"zuzu.hierarchy": false},
			"zuzu.bar.vertical":   noCorners,
			"zuzu.bar.horizontal": noCorners,
			"zuzu.point":          obj{"strokeWidth": 0},
			"zuzu.area":           obj{"zuzu.gradient": false, "zuzu.topLine": 0, "zuzu.markHigh": false},
			"zuzu.area.stacked":   obj{"fillOpacity": 1, "strokeWidth": 0},
			"zuzu.curve":          obj{"interpolate": "linear"},
			"text":                obj{"font": family},
		},
	}
}

// frame returns the layer of a chart that draws as a frame: the name of its
// type in the middle of the plot.
func (c *Chart) frame() obj {
	name := c.unsupported
	if name == "" {
		name = "empty chart"
	}
	return obj{
		"data": obj{"values": []any{obj{"t": name}}},
		"mark": obj{"type": "text", "color": css(c.text.color), "fontSize": c.text.size},
		"encoding": obj{
			"text": obj{"field": "t", "type": "nominal"},
		},
		"view": obj{"stroke": css(gridGray)},
	}
}

// categories returns the names of the categories of the plots: the longest
// list of category names of a series, with a number for each category
// that has no name. Each name is unique.
func (c *Chart) categories() []string {
	var names []string
	n := 0
	for _, p := range c.plots {
		for _, s := range p.series {
			n = max(n, len(s.vals), len(s.cats))
			if len(s.cats) > len(names) {
				names = s.cats
			}
		}
	}
	n = min(n, maxCategories)
	out := make([]string, n)
	for i := range out {
		if i < len(names) {
			out[i] = names[i]
		} else {
			out[i] = strconv.Itoa(i + 1)
		}
	}
	return unique(out)
}

// unique makes each name of a list unique: it adds zero width spaces to a
// name that an earlier one has, which draw nothing.
func unique(names []string) []string {
	seen := make(map[string]bool, len(names))
	out := make([]string, len(names))
	for i, n := range names {
		for seen[n] {
			n += zws
		}
		seen[n] = true
		out[i] = n
	}
	return out
}

// seriesNames returns the unique names of the series of the plots, with
// "Series n" for a series without a name. A pie has none.
func (c *Chart) seriesNames() map[*series]string {
	out := map[*series]string{}
	var all []string
	var order []*series
	for _, p := range c.plots {
		for _, s := range p.series {
			name := s.name
			if name == "" {
				name = "Series " + strconv.Itoa(s.index+1)
			}
			all = append(all, name)
			order = append(order, s)
		}
	}
	for i, n := range unique(all) {
		out[order[i]] = n
	}
	return out
}

// domain is the scale of the colors of a chart: its names and their colors.
type domain struct {
	names  []string
	colors []string
	// byCategory colors the points of a single series by category.
	byCategory bool
	series     map[*series]string
	cats       []string
}

// colorDomain returns the scale of the colors of the plots of a chart
// that is not a pie.
func (c *Chart) colorDomain() domain {
	d := domain{series: c.seriesNames(), cats: c.categories()}
	p := c.plots[0]
	if len(c.plots) == 1 && p.varyColors && len(p.series) == 1 && p.kind == kindBar {
		d.byCategory = true
		s := p.series[0]
		for i, name := range d.cats {
			d.names = append(d.names, name)
			d.colors = append(d.colors, css(c.pointColor(s, i, true)))
		}
		return d
	}
	for _, pl := range c.plots {
		if pl.kind == kindPie {
			continue
		}
		for _, s := range pl.series {
			d.names = append(d.names, d.series[s])
			d.colors = append(d.colors, css(s.color))
		}
	}
	return d
}

// pointColor returns the color of the point i of a series. With vary, a
// point without a color of its own takes the color of its number.
func (c *Chart) pointColor(s *series, i int, vary bool) color.NRGBA {
	if pc, ok := s.points[i]; ok {
		return pc
	}
	if vary {
		return c.palette(i)
	}
	return s.color
}

// palette returns the default color of the series or the point with the
// number i.
func (c *Chart) palette(i int) color.NRGBA {
	return c.colors[max(0, i)%len(c.colors)]
}

// colorChannel returns the channel color of a domain.
func (c *Chart) colorChannel(d domain) obj {
	field := "s"
	if d.byCategory {
		field = "c"
	}
	ch := obj{
		"field": field, "type": "nominal",
		"scale":  obj{"domain": d.names, "range": d.colors},
		"legend": c.legendOf(),
	}
	return ch
}

// legendOf returns the legend of the color channel, or nil for none.
func (c *Chart) legendOf() any {
	if c.legend == "" {
		return nil
	}
	return obj{"orient": c.legend, "title": nil}
}

// layersOf returns the layers of a plot of categories or of a scatter
// chart.
func (c *Chart) layersOf(p *plot, index int, d domain, width, height float64) []obj {
	if p.kind == kindScatter {
		return c.scatter(p, index, d, width, height)
	}
	rows := c.categoryRows(p, index, d)
	catCh, valCh := "x", "y"
	if p.kind == kindBar && p.horizontal {
		catCh, valCh = "y", "x"
	}
	// zuzu puts the first category of a band scale of y at the bottom, as
	// a chart of bars of Office does.
	cats := d.cats
	if c.cat.reverse {
		cats = reversed(cats)
	}
	enc := obj{
		catCh:   c.categoryChannel(p, cats, catCh, width),
		valCh:   c.valueChannel(p, rows),
		"color": c.colorChannel(d),
		"order": obj{"field": "k", "type": "quantitative"},
	}
	var mark obj
	switch p.kind {
	case kindBar:
		mark = obj{"type": "bar"}
		if p.grouping == "clustered" || p.grouping == "standard" || p.grouping == "" {
			var names []string
			for _, s := range p.series {
				names = append(names, d.series[s])
			}
			if len(names) > 1 {
				enc[catCh+"Offset"] = obj{"field": "s", "type": "nominal", "scale": obj{"domain": names, "paddingInner": 0}}
			}
		}
	case kindLine:
		mark = obj{"type": "line", "strokeWidth": 2.25, "strokeCap": "round", "strokeJoin": "round", "interpolate": "linear"}
		if p.smooth {
			mark["interpolate"] = "monotone"
		}
		if c.blanks == "gap" {
			mark["invalid"] = nil
		}
	case kindArea:
		mark = obj{"type": "area", "line": false, "interpolate": "linear"}
		if p.grouping == "standard" {
			mark["opacity"] = 0.85
		}
	}
	var layers []obj
	if p.kind != kindLine || !allNoLine(p) {
		layers = append(layers, obj{"data": obj{"values": rows}, "mark": mark, "encoding": enc})
	}
	if p.kind == kindLine && (c.markers(p) || allNoLine(p)) {
		// The points are a layer of their own, because zuzu gives the
		// points of a line an opacity of 0.7.
		points := maps.Clone(enc)
		points["opacity"] = obj{"value": 1}
		layers = append(layers, obj{
			"data":     obj{"values": rows},
			"mark":     obj{"type": "point", "filled": true, "size": 30},
			"encoding": points,
		})
	}
	return layers
}

// markers reports whether a line plot draws its points.
func (c *Chart) markers(p *plot) bool {
	if !p.markers {
		return false
	}
	for _, s := range p.series {
		if !s.noMarker {
			return true
		}
	}
	return false
}

// allNoLine reports whether no series of a plot draws its line.
func allNoLine(p *plot) bool {
	for _, s := range p.series {
		if !s.noLine {
			return false
		}
	}
	return len(p.series) > 0
}

// categoryChannel returns the channel of the categories.
func (c *Chart) categoryChannel(p *plot, cats []string, ch string, width float64) obj {
	n := 1.0
	if p.kind == kindBar && (p.grouping == "clustered" || p.grouping == "standard") {
		n = float64(max(1, len(p.series)))
	}
	g := p.gap / 100
	inner := g / (n + g)
	out := obj{
		"field": "c", "type": "nominal", "title": nil,
		"scale": obj{"domain": cats, "paddingInner": round4(inner), "paddingOuter": round4(inner / 2)},
	}
	a := c.cat
	if !a.present || a.deleted {
		out["axis"] = nil
		return out
	}
	if a.title != "" {
		out["title"] = a.title
	}
	ax := obj{"grid": a.grid, "ticks": true, "domain": true, "labelPadding": 3}
	if pos := orient(a.position); pos != "" && orientFits(pos, ch) {
		ax["orient"] = pos
	}
	if ch == "x" {
		ax["labelAngle"] = 0
		longest := 0
		for _, name := range cats {
			longest = max(longest, len([]rune(name)))
		}
		if len(cats) > 0 && float64(longest)*a.text.size*0.55*float64(len(cats)) > width*0.85 {
			ax["labelAngle"] = -45
		}
	}
	out["axis"] = ax
	return out
}

// valueChannel returns the channel of the values of a plot of categories.
func (c *Chart) valueChannel(p *plot, rows []any) obj {
	out := obj{"field": "v", "type": "quantitative", "title": nil}
	switch p.grouping {
	case "stacked":
		out["stack"] = "zero"
	case "percentStacked":
		out["stack"] = "normalize"
	default:
		out["stack"] = nil
	}
	lo, hi := extent(rows, p.grouping)
	zero := p.kind != kindLine || excelZero(lo, hi)
	out["scale"], out["axis"] = c.valueAxis(c.val, lo, hi, p.grouping == "percentStacked", zero, p.series)
	if c.val.title != "" {
		out["title"] = c.val.title
	}
	return out
}

// valueAxis returns the scale and the axis of an axis of values, for data
// from lo to hi.
func (c *Chart) valueAxis(a axis, lo, hi float64, percent, zero bool, series []*series) (obj, any) {
	scale := obj{"zero": zero}
	if a.reverse {
		scale["reverse"] = true
	}
	if !math.IsNaN(a.min) || !math.IsNaN(a.max) || !math.IsNaN(a.major) {
		if percent {
			lo, hi = 0, 1
		}
		if zero {
			lo, hi = min(lo, 0), max(hi, 0)
		}
		minV, maxV := a.min, a.max
		if !math.IsNaN(a.major) {
			if math.IsNaN(minV) {
				minV = math.Floor(lo/a.major) * a.major
			}
			if math.IsNaN(maxV) {
				maxV = math.Ceil(hi/a.major) * a.major
			}
		}
		if !math.IsNaN(minV) && !math.IsNaN(maxV) && maxV > minV {
			scale["domain"] = []float64{minV, maxV}
			scale["nice"] = false
			scale["zero"] = false
		} else {
			if !math.IsNaN(minV) {
				scale["domainMin"] = minV
			}
			if !math.IsNaN(maxV) {
				scale["domainMax"] = maxV
			}
		}
		if !math.IsNaN(a.major) && !math.IsNaN(minV) && !math.IsNaN(maxV) && (maxV-minV)/a.major <= 100 {
			var values []float64
			for v := minV; v <= maxV+a.major/1e6; v += a.major {
				values = append(values, round4(v))
			}
			scale["values"] = values
		}
	}
	if !a.present || a.deleted {
		delete(scale, "values")
		return scale, nil
	}
	ax := obj{"grid": a.grid, "ticks": false, "domain": false, "labelPadding": 4}
	if v, ok := scale["values"]; ok {
		ax["values"] = v
		delete(scale, "values")
	}
	format := a.format
	if format == "" && len(series) > 0 {
		format = series[0].format
	}
	if f := d3Format(format); f != "" {
		ax["format"] = f
	}
	if percent && a.format == "" {
		ax["format"] = ".0%"
	}
	if pos := orient(a.position); pos != "" {
		ax["orient"] = pos
	}
	return scale, ax
}

// orient returns the orient of Vega-Lite of an axis position of DrawingML.
func orient(pos string) string {
	return map[string]string{"b": "bottom", "t": "top", "l": "left", "r": "right"}[pos]
}

// orientFits reports whether an orient fits the channel of an axis.
func orientFits(o, ch string) bool {
	if ch == "x" {
		return o == "bottom" || o == "top"
	}
	return o == "left" || o == "right"
}

// extent returns the smallest and the largest value of the rows, or of
// the sums of a stack.
func extent(rows []any, grouping string) (lo, hi float64) {
	lo, hi = math.Inf(1), math.Inf(-1)
	if grouping == "stacked" {
		pos, neg := map[int]float64{}, map[int]float64{}
		for _, r := range rows {
			row := r.(obj) //nolint:forcetypeassert // categoryRows makes each row an obj.
			v, ok := row["v"].(float64)
			if !ok {
				continue
			}
			i, _ := row["i"].(int)
			if v >= 0 {
				pos[i] += v
			} else {
				neg[i] += v
			}
		}
		for _, v := range pos {
			hi = max(hi, v)
			lo = min(lo, v)
		}
		for _, v := range neg {
			lo = min(lo, v)
			hi = max(hi, v)
		}
	} else {
		for _, r := range rows {
			if v, ok := r.(obj)["v"].(float64); ok {
				lo, hi = min(lo, v), max(hi, v)
			}
		}
	}
	if math.IsInf(lo, 0) || math.IsInf(hi, 0) {
		return 0, 1
	}
	return lo, hi
}

// categoryRows returns the rows of a plot of categories.
func (c *Chart) categoryRows(p *plot, index int, d domain) []any {
	var rows []any
	for k, s := range p.series {
		lab := c.labelsOf(p, s)
		n := min(max(len(s.vals), len(s.cats)), len(d.cats))
		for i := range n {
			v := math.NaN()
			if i < len(s.vals) {
				v = s.vals[i]
			}
			row := obj{"c": d.cats[i], "s": d.series[s], "i": i, "k": k, "p": index}
			switch {
			case !math.IsNaN(v):
				row["v"] = v
			case c.blanks == "zero" && p.kind != kindBar:
				row["v"] = 0.0
				v = 0
			case c.blanks == "span" && p.kind == kindLine:
				continue
			default:
				row["v"] = nil
			}
			if lab.show() && !math.IsNaN(v) {
				row["l"] = labelText(lab, d.series[s], categoryName(s, i), v, math.NaN(), s.format)
			}
			rows = append(rows, row)
		}
	}
	return rows
}

// categoryName returns the name of the category i of a series, without the
// characters that make it unique.
func categoryName(s *series, i int) string {
	if i < len(s.cats) {
		return s.cats[i]
	}
	return strconv.Itoa(i + 1)
}

// labelsOf returns the labels of a series: its own, or the labels of its
// plot.
func (c *Chart) labelsOf(p *plot, s *series) labels {
	if s.labels.set {
		return s.labels
	}
	return p.labels
}

// labelText returns the text of the label on a point. percent is the share
// of the point in a pie, or NaN.
func labelText(l labels, seriesName, category string, v, percent float64, format string) string {
	var parts []string
	if l.seriesOf {
		parts = append(parts, seriesName)
	}
	if l.category {
		parts = append(parts, category)
	}
	if l.value {
		f := l.format
		if f == "" {
			f = format
		}
		parts = append(parts, formatValue(v, f))
	}
	if l.percent && !math.IsNaN(percent) {
		parts = append(parts, formatValue(percent, "0%"))
	}
	s := strings.Join(parts, ", ")
	if len(s) > maxText {
		s = s[:maxText]
	}
	return s
}

// scatter returns the layers of a scatter or a bubble chart: the lines
// and the points.
func (c *Chart) scatter(p *plot, index int, d domain, width, height float64) []obj {
	var rows []any
	xlo, xhi, ylo, yhi := math.Inf(1), math.Inf(-1), math.Inf(1), math.Inf(-1)
	for k, s := range p.series {
		lab := c.labelsOf(p, s)
		for i, v := range s.vals {
			x := float64(i + 1)
			if i < len(s.xs) {
				x = s.xs[i]
			}
			if math.IsNaN(v) || math.IsNaN(x) {
				continue
			}
			row := obj{"x": x, "v": v, "s": d.series[s], "i": i, "k": k, "p": index}
			if p.bubbles {
				z := 0.0
				if i < len(s.sizes) && !math.IsNaN(s.sizes[i]) {
					z = math.Abs(s.sizes[i])
				}
				row["z"] = z
			}
			if lab.show() {
				row["l"] = labelText(lab, d.series[s], formatValue(x, ""), v, math.NaN(), s.format)
			}
			xlo, xhi, ylo, yhi = min(xlo, x), max(xhi, x), min(ylo, v), max(yhi, v)
			rows = append(rows, row)
		}
	}
	xs, xa := c.valueAxis(c.cat, xlo, xhi, false, excelZero(xlo, xhi), nil)
	ys, ya := c.valueAxis(c.val, ylo, yhi, false, excelZero(ylo, yhi), p.series)
	x := obj{"field": "x", "type": "quantitative", "scale": xs, "axis": xa, "title": nil}
	y := obj{"field": "v", "type": "quantitative", "scale": ys, "axis": ya, "title": nil}
	if c.cat.title != "" {
		x["title"] = c.cat.title
	}
	if c.val.title != "" {
		y["title"] = c.val.title
	}
	if xa != nil {
		xa.(obj)["ticks"], xa.(obj)["domain"] = true, true //nolint:forcetypeassert // valueAxis returns an obj or nil.
	}
	color := c.colorChannel(d)
	var layers []obj
	if p.lines && !allNoLine(p) && !p.bubbles {
		mark := obj{"type": "line", "strokeWidth": 2.25, "interpolate": "linear"}
		if p.smooth {
			mark["interpolate"] = "monotone"
		}
		layers = append(layers, obj{
			"data": obj{"values": rows},
			"mark": mark,
			"encoding": obj{"x": x, "y": y, "color": color, "detail": obj{"field": "k", "type": "nominal"},
				"order": obj{"field": "i", "type": "quantitative"}},
		})
	}
	if p.markers || p.bubbles || !p.lines {
		mark := obj{"type": "point", "filled": true, "size": 36}
		// zuzu gives a point an opacity of 0.7, unless the channel opacity
		// sets it.
		enc := obj{"x": x, "y": y, "color": color, "opacity": obj{"value": 1}}
		if p.bubbles {
			big := math.Pow(min(width, height)*0.25, 2)
			enc["size"] = obj{"field": "z", "type": "quantitative", "legend": nil,
				"scale": obj{"range": []float64{round2(big / 100), round2(big)}, "zero": true}}
			enc["opacity"] = obj{"value": 0.75}
			mark["shape"] = "circle"
		}
		layers = append(layers, obj{"data": obj{"values": rows}, "mark": mark, "encoding": enc})
	}
	return layers
}

// excelZero reports whether an axis of values from lo to hi starts at 0,
// as the automatic axis of Excel does when the values are far from 0 by
// less than a sixth of their span.
func excelZero(lo, hi float64) bool {
	if math.IsInf(lo, 0) || math.IsInf(hi, 0) {
		return true
	}
	switch {
	case lo >= 0:
		return lo < hi*5/6
	case hi <= 0:
		return hi > lo*5/6
	}
	return true
}

// pie returns the layer of a pie or a doughnut: the first series.
func (c *Chart) pie(p *plot, index int) obj {
	var s *series
	if len(p.series) > 0 {
		s = p.series[0]
	} else {
		s = &series{}
	}
	cats := c.categories()
	total := 0.0
	for _, v := range s.vals {
		if !math.IsNaN(v) {
			total += math.Abs(v)
		}
	}
	lab := c.labelsOf(p, s)
	var rows []any
	var names, colors []string
	name := c.seriesNames()[s]
	for i := range min(len(s.vals), len(cats)) {
		v := math.Abs(s.vals[i])
		names = append(names, cats[i])
		colors = append(colors, css(c.pointColor(s, i, p.varyColors)))
		if math.IsNaN(v) || v == 0 {
			continue
		}
		row := obj{"c": cats[i], "v": v, "i": i, "k": 0, "p": index}
		if lab.show() {
			row["l"] = labelText(lab, name, categoryName(s, i), s.vals[i], v/total, s.format)
		}
		rows = append(rows, row)
	}
	return obj{
		"data": obj{"values": rows},
		"mark": obj{"type": "arc", "stroke": "#ffffff", "strokeWidth": 1, "padAngle": 0, "cornerRadius": 0},
		"encoding": obj{
			"theta": obj{"field": "v", "type": "quantitative", "stack": true},
			"order": obj{"field": "i", "type": "quantitative"},
			"color": obj{"field": "c", "type": "nominal",
				"scale":  obj{"domain": names, "range": colors},
				"legend": c.legendOf()},
		},
	}
}

// reversed returns a copy of a list in the reverse order.
func reversed[T any](list []T) []T {
	out := make([]T, len(list))
	for i, v := range list {
		out[len(list)-1-i] = v
	}
	return out
}

// round2 rounds v to two decimals, so that the spec is stable.
func round2(v float64) float64 {
	return math.Round(v*100) / 100
}

// round4 rounds v to four decimals.
func round4(v float64) float64 {
	return math.Round(v*1e4) / 1e4
}
