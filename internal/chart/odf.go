package chart

import (
	"fmt"
	"image/color"
	"math"
	"strconv"
	"strings"

	"github.com/xo/okage/roxmltree"

	"github.com/xo/ivy"
	"github.com/xo/ivy/internal/drawingml"
	"github.com/xo/ivy/internal/ooxml"
)

// The namespaces of OpenDocument that a chart uses.
const (
	nsChart = "urn:oasis:names:tc:opendocument:xmlns:chart:1.0"
	nsTable = "urn:oasis:names:tc:opendocument:xmlns:table:1.0"
	nsStyle = "urn:oasis:names:tc:opendocument:xmlns:style:1.0"
)

// libreColors are the default colors of the series of LibreOffice.
var libreColors = []color.NRGBA{
	{0x00, 0x45, 0x86, 255}, {0xff, 0x42, 0x0e, 255}, {0xff, 0xd3, 0x20, 255}, {0x57, 0x9d, 0x1c, 255},
	{0x7e, 0x00, 0x21, 255}, {0x83, 0xca, 0xff, 255}, {0x31, 0x40, 0x04, 255}, {0xae, 0xcf, 0x00, 255},
	{0x4b, 0x1f, 0x6f, 255}, {0xff, 0x95, 0x0e, 255}, {0xc5, 0x00, 0x0b, 255}, {0x00, 0x84, 0xd1, 255},
}

// odfKinds maps the class of a chart of OpenDocument to the kind of plot
// that draws it.
var odfKinds = map[string]kind{
	"bar": kindBar, "line": kindLine, "area": kindArea, "circle": kindPie, "ring": kindPie,
	"scatter": kindScatter, "bubble": kindScatter, "radar": kindLine, "filled-radar": kindArea,
	"stock": kindLine,
}

// ParseODF reads a chart of OpenDocument: the content.xml of an embedded
// object whose body is office:chart (ODF 1.3 Part 3, 11). The values come
// from the table of the chart. It returns an error that wraps
// ivy.ErrFormat for a part without a chart.
func ParseODF(buf []byte, opts Options) (c *Chart, err error) {
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
	var chart roxmltree.Node
	found := false
	styles := map[string]roxmltree.Node{}
	for n := range doc.Descendants() {
		if !n.IsElement() {
			continue
		}
		switch {
		case isElem(n, nsChart, "chart") && !found:
			chart, found = n, true
		case isElem(n, nsStyle, "style") && len(styles) < 1<<12:
			if name := ooxml.Attr(n, "name"); name != "" {
				styles[name] = n
			}
		}
	}
	if !found {
		return nil, fmt.Errorf("chart object without chart:chart: %w", ivy.ErrFormat)
	}
	c = newChart(opts)
	c.colors = libreColors
	c.font = "Liberation Sans"
	c.text.color = color.NRGBA{0, 0, 0, 255}
	if c.width <= 0 || c.height <= 0 {
		c.width, c.height = length(ooxml.Attr(chart, "width")), length(ooxml.Attr(chart, "height"))
	}
	p := &odfParser{c: c, styles: styles, budget: min(lim.MaxObjects, maxPoints)}
	p.chart(chart)
	c.finish()
	return c, nil
}

// odfParser holds the state of one call of ParseODF.
type odfParser struct {
	c      *Chart
	styles map[string]roxmltree.Node
	table  [][]odfCell
	budget int
}

// odfCell is a cell of the table of a chart.
type odfCell struct {
	text  string
	num   float64
	isNum bool
}

// isElem reports whether n is the element local in the namespace ns.
func isElem(n roxmltree.Node, ns, local string) bool {
	if n.TagName().Name() != local {
		return false
	}
	got, _ := n.TagName().Namespace()
	return got == ns
}

// chart reads chart:chart.
func (p *odfParser) chart(n roxmltree.Node) {
	c := p.c
	class := strings.TrimPrefix(ooxml.Attr(n, "class"), "chart:")
	if st := p.props(ooxml.Attr(n, "style-name")); st.text.size > 0 || st.text.font != "" {
		c.text = st.text.merge(c.text)
		if st.text.font != "" {
			c.font = st.text.font
		}
	}
	if fill, ok := p.props(ooxml.Attr(n, "style-name")).fill(); ok {
		c.fill = fill
	}
	for e := range elements(n) {
		if isElem(e, nsTable, "table") && p.table == nil {
			p.readTable(e)
		}
	}
	for e := range elements(n) {
		switch e.TagName().Name() {
		case "title":
			if isElem(e, nsChart, "title") {
				c.title = clip(paragraphs(e))
				c.titleText = p.props(ooxml.Attr(e, "style-name")).text
			}
		case "legend":
			if isElem(e, nsChart, "legend") {
				c.legend = map[string]string{
					"start": "left", "end": "right", "top": "top", "bottom": "bottom",
					"top-start": "top", "top-end": "top-right", "bottom-start": "bottom", "bottom-end": "bottom",
				}[ooxml.Attr(e, "legend-position")]
				if c.legend == "" {
					c.legend = "right"
				}
				c.legendText = p.props(ooxml.Attr(e, "style-name")).text
			}
		case "plot-area":
			if isElem(e, nsChart, "plot-area") {
				p.plotArea(e, class)
			}
		}
	}
	if len(c.plots) == 0 && odfKinds[class] == 0 {
		c.unsupported = class + " chart"
	}
	if c.titleText.size <= 0 {
		c.titleText.size = math.Round(c.text.size * 1.3)
	}
	c.titleText = c.titleText.merge(c.text)
	c.legendText = c.legendText.merge(c.text)
	c.cat.text = c.cat.text.merge(c.text)
	c.val.text = c.val.text.merge(c.text)
	c.cat.titleText = c.cat.titleText.merge(c.text)
	c.val.titleText = c.val.titleText.merge(c.text)
}

// paragraphs returns the text of each text:p of n.
func paragraphs(n roxmltree.Node) []string {
	var lines []string
	for e := range elements(n) {
		if e.TagName().Name() == "p" {
			lines = append(lines, text(e))
		}
	}
	return lines
}

// plotArea reads chart:plot-area: its axes and its series.
func (p *odfParser) plotArea(n roxmltree.Node, class string) {
	c := p.c
	st := p.props(ooxml.Attr(n, "style-name"))
	var cats []string
	for e := range elements(n) {
		if !isElem(e, nsChart, "axis") {
			continue
		}
		a := p.axis(e)
		switch ooxml.Attr(e, "dimension") {
		case "x":
			if !c.cat.present {
				c.cat = a
			}
			for cat := range elements(e) {
				if isElem(cat, nsChart, "categories") {
					for _, cell := range p.cells(ooxml.Attr(cat, "cell-range-address")) {
						cats = append(cats, cell.label())
					}
				}
			}
		case "y":
			if !c.val.present {
				c.val = a
			}
		}
	}
	plots := map[kind]*plot{}
	addSeries := func(k kind, s *series) {
		pl, ok := plots[k]
		if !ok {
			pl = p.newPlot(k, class, st)
			plots[k] = pl
			c.plots = append(c.plots, pl)
		}
		s.index = len(pl.series)
		if len(pl.series) >= maxSeries || p.budget <= 0 {
			c.truncated = true
			return
		}
		pl.series = append(pl.series, s)
		p.budget -= max(len(s.vals), len(s.cats))
	}
	defaultKind := odfKinds[class]
	seen := false
	for e := range elements(n) {
		if !isElem(e, nsChart, "series") {
			continue
		}
		seen = true
		k := defaultKind
		if sc := strings.TrimPrefix(ooxml.Attr(e, "class"), "chart:"); sc != "" {
			if kk, ok := odfKinds[sc]; ok {
				k = kk
			}
		}
		if k == 0 {
			continue
		}
		addSeries(k, p.series(e, cats, k))
	}
	if !seen && defaultKind != 0 {
		// A chart without series draws the columns of its table.
		for col := 1; len(p.table) > 0 && col < len(p.table[0]); col++ {
			s := &series{}
			if len(p.table[0]) > col {
				s.name = p.table[0][col].text
			}
			for r := 1; r < len(p.table) && r <= maxCategories; r++ {
				row := p.table[r]
				v := math.NaN()
				if col < len(row) && row[col].isNum {
					v = row[col].num
				}
				s.vals = append(s.vals, v)
				label := ""
				if len(row) > 0 {
					label = row[0].label()
				}
				s.cats = append(s.cats, label)
			}
			addSeries(defaultKind, s)
		}
	}
	for _, pl := range c.plots {
		for i, s := range pl.series {
			if !s.hasColor {
				s.color, s.hasColor = c.palette(i), true
			}
		}
	}
}

// newPlot returns a plot of a kind with the properties of the style of the
// plot area.
func (p *odfParser) newPlot(k kind, class string, st odfProps) *plot {
	pl := &plot{kind: k, gap: 100, grouping: "clustered", markers: k == kindLine && class != "radar"}
	if k != kindBar {
		pl.grouping = "standard"
	}
	switch {
	case st.attr("percentage") == "true":
		pl.grouping = "percentStacked"
	case st.attr("stacked") == "true":
		pl.grouping = "stacked"
	}
	pl.horizontal = st.attr("vertical") == "true"
	if g, ok := parseNumber(st.attr("gap-width")); ok && g >= 0 && g <= 500 {
		pl.gap = g
	}
	switch class {
	case "circle":
		pl.varyColors = true
	case "ring":
		pl.varyColors, pl.hole = true, 0.5
	case "bubble":
		pl.bubbles = true
	case "scatter":
		pl.lines = st.attr("lines") != "false"
		pl.markers = st.attr("symbol-type") != "none"
	}
	if st.attr("symbol-type") == "none" {
		pl.markers = false
	}
	if in := st.attr("interpolation"); in != "" && in != "none" {
		pl.smooth = true
	}
	return pl
}

// axis reads chart:axis.
func (p *odfParser) axis(n roxmltree.Node) axis {
	a := newAxis()
	a.present = true
	st := p.props(ooxml.Attr(n, "style-name"))
	a.deleted = st.attr("display-label") == "false" && st.attr("visible") == "false"
	a.reverse = st.attr("reverse-direction") == "true"
	if v, ok := parseNumber(st.attr("minimum")); ok {
		a.min = v
	}
	if v, ok := parseNumber(st.attr("maximum")); ok {
		a.max = v
	}
	if v, ok := parseNumber(st.attr("interval-major")); ok && v > 0 {
		a.major = v
	}
	a.text = st.text
	for e := range elements(n) {
		switch {
		case isElem(e, nsChart, "grid"):
			a.grid = a.grid || ooxml.Attr(e, "class") != "minor"
		case isElem(e, nsChart, "title"):
			a.title = strings.Join(paragraphs(e), " ")
			a.titleText = p.props(ooxml.Attr(e, "style-name")).text
		}
	}
	return a
}

// series reads chart:series.
func (p *odfParser) series(n roxmltree.Node, cats []string, k kind) *series {
	s := &series{}
	st := p.props(ooxml.Attr(n, "style-name"))
	if c, ok := st.fill(); ok && k != kindLine && k != kindScatter {
		s.color, s.hasColor = c, true
	} else if c, ok := st.stroke(); ok {
		s.color, s.hasColor = c, true
	} else if c, ok := st.fill(); ok {
		s.color, s.hasColor = c, true
	}
	s.noLine = st.graphic("stroke") == "none"
	s.noMarker = st.attr("symbol-type") == "none"
	if cells := p.cells(ooxml.Attr(n, "label-cell-address")); len(cells) > 0 {
		s.name = cells[0].label()
	}
	for _, cell := range p.cells(ooxml.Attr(n, "values-cell-range-address")) {
		v := math.NaN()
		if cell.isNum {
			v = cell.num
		}
		s.vals = append(s.vals, v)
	}
	s.cats = cats
	if len(s.cats) > len(s.vals) {
		s.cats = s.cats[:len(s.vals)]
	}
	var domains [][]odfCell
	i := 0
	for e := range elements(n) {
		switch {
		case isElem(e, nsChart, "domain"):
			domains = append(domains, p.cells(ooxml.Attr(e, "cell-range-address")))
		case isElem(e, nsChart, "data-point"):
			repeat := 1
			if r, err := strconv.Atoi(ooxml.Attr(e, "repeated")); err == nil && r > 0 {
				repeat = min(r, maxPoints)
			}
			ps := p.props(ooxml.Attr(e, "style-name"))
			if c, ok := ps.fill(); ok {
				for j := i; j < i+repeat && j < len(s.vals); j++ {
					if s.points == nil {
						s.points = map[int]color.NRGBA{}
					}
					s.points[j] = c
				}
			}
			i += repeat
		}
	}
	if k == kindScatter {
		// The first domain is x, and the values are y. A bubble chart has
		// the sizes as its values and y as its second domain.
		if len(domains) > 0 {
			s.xs = numbers(domains[0])
		}
		if len(domains) > 1 {
			s.sizes = s.vals
			s.vals = numbers(domains[1])
		}
	}
	s.labels = st.labels()
	return s
}

// numbers returns the numbers of cells, with NaN for a cell of text.
func numbers(cells []odfCell) []float64 {
	out := make([]float64, len(cells))
	for i, c := range cells {
		out[i] = math.NaN()
		if c.isNum {
			out[i] = c.num
		}
	}
	return out
}

// label returns the text of a cell, or its number.
func (c odfCell) label() string {
	if c.text != "" || !c.isNum {
		return c.text
	}
	return formatValue(c.num, "")
}

// readTable reads the local table of a chart into a grid.
func (p *odfParser) readTable(n roxmltree.Node) {
	const maxColumns = 1 << 10
	var rows [][]odfCell
	var walk func(n roxmltree.Node, depth int)
	walk = func(n roxmltree.Node, depth int) {
		for e := range elements(n) {
			if len(rows) > maxCategories {
				return
			}
			switch e.TagName().Name() {
			case "table-header-rows", "table-rows", "table-row-group":
				if depth < 4 {
					walk(e, depth+1)
				}
			case "table-row":
				var row []odfCell
				for cell := range elements(e) {
					name := cell.TagName().Name()
					if name != "table-cell" && name != "covered-table-cell" {
						continue
					}
					repeat := 1
					if r, err := strconv.Atoi(ooxml.Attr(cell, "number-columns-repeated")); err == nil && r > 1 {
						repeat = r
					}
					v := odfCell{text: strings.Join(paragraphs(cell), " ")}
					switch ooxml.Attr(cell, "value-type") {
					case "float", "percentage", "currency":
						v.num, v.isNum = parseNumber(ooxml.Attr(cell, "value"))
					}
					for range min(repeat, maxColumns-len(row)) {
						row = append(row, v)
					}
				}
				repeat := 1
				if r, err := strconv.Atoi(ooxml.Attr(e, "number-rows-repeated")); err == nil && r > 1 {
					repeat = r
				}
				for range min(repeat, maxCategories+1-len(rows)) {
					rows = append(rows, row)
				}
			}
		}
	}
	walk(n, 0)
	p.table = rows
	if p.table == nil {
		p.table = [][]odfCell{}
	}
}

// cells returns the cells of a range of the local table, such as
// "local-table.$B$2:.$B$5", in the order of its rows, then its columns.
func (p *odfParser) cells(ref string) []odfCell {
	start, end, ok := strings.Cut(ref, ":")
	r0, c0, ok0 := cellRef(start)
	if !ok0 {
		return nil
	}
	r1, c1 := r0, c0
	if ok {
		var ok1 bool
		if r1, c1, ok1 = cellRef(end); !ok1 {
			return nil
		}
	}
	if r1 < r0 {
		r0, r1 = r1, r0
	}
	if c1 < c0 {
		c0, c1 = c1, c0
	}
	var out []odfCell
	for r := r0; r <= r1 && r < len(p.table); r++ {
		for col := c0; col <= c1; col++ {
			if len(out) >= maxCategories {
				return out
			}
			if col < len(p.table[r]) {
				out = append(out, p.table[r][col])
			} else {
				out = append(out, odfCell{})
			}
		}
	}
	return out
}

// cellRef reads a reference to a cell, such as "local-table.$B$2", as its
// row and its column, from 0.
func cellRef(s string) (row, col int, ok bool) {
	if i := strings.LastIndexByte(s, '.'); i >= 0 {
		s = s[i+1:]
	}
	s = strings.ReplaceAll(s, "$", "")
	i := 0
	for i < len(s) && s[i] >= 'A' && s[i] <= 'Z' {
		col = col*26 + int(s[i]-'A'+1)
		i++
		if col > 1<<14 {
			return 0, 0, false
		}
	}
	r, err := strconv.Atoi(s[i:])
	if i == 0 || err != nil || r < 1 || r > 1<<20 {
		return 0, 0, false
	}
	return r - 1, col - 1, true
}

// length reads a length of OpenDocument, such as "16cm", in points. It
// returns 0 for a length that does not read.
func length(s string) float64 {
	s = strings.TrimSpace(s)
	units := []struct {
		suffix string
		points float64
	}{{"cm", 72 / 2.54}, {"mm", 72 / 25.4}, {"in", 72}, {"pt", 1}, {"pc", 12}, {"px", 0.75}}
	for _, u := range units {
		if v, ok := strings.CutSuffix(s, u.suffix); ok {
			if n, ok := parseNumber(v); ok && n > 0 && n < 1e5 {
				return n * u.points
			}
			return 0
		}
	}
	return 0
}

// odfProps are the properties of a style of a chart.
type odfProps struct {
	chartNode, graphicNode roxmltree.Node
	hasChart, hasGraphic   bool
	text                   textStyle
}

// props returns the properties of the style with the name name.
func (p *odfParser) props(name string) odfProps {
	var out odfProps
	st, ok := p.styles[name]
	if !ok {
		return out
	}
	for e := range elements(st) {
		switch e.TagName().Name() {
		case "chart-properties":
			out.chartNode, out.hasChart = e, true
		case "graphic-properties":
			out.graphicNode, out.hasGraphic = e, true
		case "text-properties":
			if v := length(ooxml.Attr(e, "font-size")); v > 0 && v < 4000 {
				out.text.size = v
			}
			if c, ok := drawingml.ParseHex(ooxml.Attr(e, "color")); ok {
				out.text.color, out.text.hasColor = c, true
			}
			out.text.bold = ooxml.Attr(e, "font-weight") == "bold"
			if f := ooxml.Attr(e, "font-family"); f != "" && len(f) < 128 {
				out.text.font = strings.Trim(f, `'"`)
			} else if f := ooxml.Attr(e, "font-name"); f != "" && len(f) < 128 {
				out.text.font = f
			}
		}
	}
	return out
}

// attr returns an attribute of the chart properties.
func (o odfProps) attr(name string) string {
	if !o.hasChart {
		return ""
	}
	return ooxml.Attr(o.chartNode, name)
}

// graphic returns an attribute of the graphic properties.
func (o odfProps) graphic(name string) string {
	if !o.hasGraphic {
		return ""
	}
	return ooxml.Attr(o.graphicNode, name)
}

// fill returns the color of the fill.
func (o odfProps) fill() (color.NRGBA, bool) {
	if o.graphic("fill") == "none" {
		return color.NRGBA{}, false
	}
	return drawingml.ParseHex(o.graphic("fill-color"))
}

// stroke returns the color of the line.
func (o odfProps) stroke() (color.NRGBA, bool) {
	if o.graphic("stroke") == "none" {
		return color.NRGBA{}, false
	}
	return drawingml.ParseHex(o.graphic("stroke-color"))
}

// labels returns the labels on the data of a series.
func (o odfProps) labels() labels {
	l := labels{set: o.hasChart}
	switch o.attr("data-label-number") {
	case "value":
		l.value = true
	case "percentage":
		l.percent = true
	case "value-and-percentage":
		l.value, l.percent = true, true
	}
	l.category = o.attr("data-label-text") == "true"
	l.seriesOf = o.attr("data-label-series") == "true"
	l.text = o.text
	return l
}
