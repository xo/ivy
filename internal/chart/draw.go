package chart

import (
	"context"
	"fmt"
	"image/color"
	"math"
	"strconv"

	"github.com/xo/okage"
	"github.com/xo/okage/svg"

	"github.com/xo/ivy"
	"github.com/xo/ivy/internal/display"
	"github.com/xo/ivy/internal/fontdb"
)

// This file draws a chart: the chart area, the title and the legend. The
// plot area draws in plane.go for the charts with axes, in pie.go for a pie
// and in radar.go for a radar chart. The sizes and the gaps are the ones of
// Office, in points.

// The bounds of the size of the layout of a chart, in points. A chart of
// another size lays out at the nearest size in the bounds, and scales to its
// rectangle.
const (
	minSize = 24
	maxSize = 4096
)

// The gaps of the layout of a chart, in points.
const (
	// pad is the gap between the side of the chart and its parts.
	pad = 7
	// gapPart is the gap between the title, the legend and the plot area.
	gapPart = 6
	// tickLen is the length of a major tick mark, and minorLen of a minor
	// one.
	tickLen  = 4
	minorLen = 2
	// gapLabel is the gap between an axis and its labels.
	gapLabel = 3
	// thin is the width of the lines of the axes and of the gridlines.
	thin = 0.75
)

// Draw draws the chart onto dev, into the rectangle rect of the user space of
// dev. The chart lays out at the size of Options, in points, and scales to
// rect. Without a size, it lays out at the size of rect, as points. Draw
// saves and restores the state of dev. It returns an *ivy.PartialError for a
// chart that has more series or points than the limits, and draws the first
// ones.
func (c *Chart) Draw(ctx context.Context, dev display.Device, rect okage.Rect) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = &ivy.PartialError{Err: fmt.Errorf("drawing chart: panic: %v: %w", r, ivy.ErrFormat)}
		}
	}()
	if err := ctx.Err(); err != nil {
		return err
	}
	rw, rh := float64(rect.Width()), float64(rect.Height())
	if !(rw > 0) || !(rh > 0) || math.IsInf(rw, 0) || math.IsInf(rh, 0) {
		return nil
	}
	w, h := c.width, c.height
	if !(w > 0) || !(h > 0) {
		w, h = rw, rh
	}
	if k := maxSize / max(w, h); k < 1 {
		w, h = w*k, h*k
	}
	w, h = max(w, minSize), max(h, minSize)
	db := c.fontDatabase()
	cv := &canvas{c: c, dev: dev, t: newTexter(db, c.font), w: w, h: h}
	dev.Save()
	defer dev.Restore()
	dev.Concat(okage.TransformFromRow(float32(rw/w), 0, 0, float32(rh/h), rect.X(), rect.Y()))
	cv.draw()
	if c.truncated {
		return &ivy.PartialError{Err: fmt.Errorf("chart with more than %d series or %d points: %w", maxSeries, maxPoints, ivy.ErrLimit)}
	}
	return nil
}

// fontDatabase returns the fonts of the text of the chart: the fonts of
// Options, or the fonts of the program.
func (c *Chart) fontDatabase() *svg.FontDatabase {
	if c.fonts != nil {
		return c.fonts
	}
	return fontdb.Of(c.cfg)
}

// family returns the family that draws the text of the chart: its font if
// db has it, else the font with the same metrics if db has it, else the
// sans serif family of db, else the first family of db.
func (c *Chart) family(db *svg.FontDatabase) string {
	return familyOf(db, c.font)
}

// box is a rectangle of the layout, by its sides.
type box struct {
	x0, y0, x1, y1 float64
}

// w returns the width of a box.
func (b box) w() float64 { return b.x1 - b.x0 }

// h returns the height of a box.
func (b box) h() float64 { return b.y1 - b.y0 }

// ok says whether a box has an area.
func (b box) ok() bool {
	return b.x1 > b.x0 && b.y1 > b.y0 && !math.IsNaN(b.x0+b.x1+b.y0+b.y1) && !math.IsInf(b.x0+b.x1+b.y0+b.y1, 0)
}

// canvas holds the state of one drawing of a chart.
type canvas struct {
	c   *Chart
	dev display.Device
	t   *texter
	// w and h are the size of the layout, in points.
	w, h float64
}

// draw draws the whole chart.
func (cv *canvas) draw() {
	c := cv.c
	cv.rect(box{0, 0, cv.w, cv.h}, c.fill, c.border, thin)
	area := box{pad, pad, cv.w - pad, cv.h - pad}
	area = cv.title(area)
	area = cv.legend(area)
	plotBox, inner := area, false
	if b := c.plotAt; !math.IsNaN(b[0]) && !math.IsNaN(b[2]) {
		pb := box{b[0] * cv.w, b[1] * cv.h, (b[0] + b[2]) * cv.w, (b[1] + b[3]) * cv.h}
		if pb.ok() && pb.w() > 4 && pb.h() > 4 {
			plotBox, inner = pb, c.plotInner
		}
	}
	if !plotBox.ok() {
		return
	}
	switch {
	case c.unsupported != "" || len(c.plots) == 0:
		cv.frame(plotBox)
	case c.plots[0].kind == kindPie:
		cv.pie(plotBox, inner)
	case c.plots[0].special == "treemap":
		cv.treemap(plotBox)
	case c.plots[0].special == "sunburst":
		cv.sunburst(plotBox)
	case c.plots[0].kind == kindRadar:
		cv.radar(plotBox)
	default:
		cv.plane(plotBox, inner)
	}
}

// rect fills and strokes a box. A transparent color draws nothing.
func (cv *canvas) rect(b box, fill, line color.NRGBA, width float64) {
	r, ok := okage.RectFromLTRB(float32(b.x0), float32(b.y0), float32(b.x1), float32(b.y1))
	if !ok {
		return
	}
	p := okage.PathFromRect(r)
	if fill.A > 0 {
		cv.dev.FillPath(p, okage.FillRuleWinding, okage.SolidColor(rgba(fill)))
	}
	if line.A > 0 && width > 0 {
		cv.dev.StrokePath(p, okage.Stroke{Width: float32(width), MiterLimit: 4}, okage.SolidColor(rgba(line)))
	}
}

// line strokes a line from (x0, y0) to (x1, y1).
func (cv *canvas) line(x0, y0, x1, y1 float64, col color.NRGBA, width float64) {
	if col.A == 0 || !(width > 0) {
		return
	}
	pb := okage.NewPathBuilder()
	pb.MoveTo(float32(x0), float32(y0))
	pb.LineTo(float32(x1), float32(y1))
	if p, ok := pb.Finish(); ok {
		cv.dev.StrokePath(p, okage.Stroke{Width: float32(width), MiterLimit: 4}, okage.SolidColor(rgba(col)))
	}
}

// title draws the title of the chart, and returns the area below it.
func (cv *canvas) title(area box) box {
	c := cv.c
	if len(c.title) == 0 {
		return area
	}
	st := c.titleText
	var lines []string
	for _, l := range c.title {
		lines = append(lines, cv.t.wrap(l, st, cv.w*0.8)...)
	}
	b := block{lines: lines, st: st}
	_, h := cv.t.size(b)
	if at := c.titleAt; !math.IsNaN(at[0]) && !math.IsNaN(at[1]) {
		cv.t.draw(cv.dev, b, at[0]*cv.w, at[1]*cv.h, alignStart, alignStart, 0)
		if at[1]*cv.h < cv.h/3 {
			area.y0 = max(area.y0, at[1]*cv.h+h+gapPart)
		}
		return area
	}
	cv.t.draw(cv.dev, b, cv.w/2, area.y0, alignCenter, alignStart, 0)
	if !c.titleOver {
		area.y0 += h + gapPart
	}
	return area
}

// entry is an entry of the legend.
type entry struct {
	text string
	// line draws the symbol as a line with a marker, and not as a square.
	line bool
	fill color.NRGBA
	// noFill draws the square without a fill.
	noFill bool
	// stroke is the color of the outline of the square, or of the line.
	stroke    color.NRGBA
	lineWidth float64
	dash      string
	// marker is the marker on the line, or empty.
	marker string
	s      *series
	p      *plot
}

// entries returns the entries of the legend: one for each category of a
// pie or of a single series with a color for each point, else one for each
// series.
func (cv *canvas) entries() []entry {
	c := cv.c
	var out []entry
	add := func(e entry) {
		if !c.legendHidden[len(out)] || len(out) >= maxSeries {
			out = append(out, e)
			return
		}
		out = append(out, entry{text: "\x00"})
	}
	first := c.plots[0]
	if len(c.plots) == 1 && first.varyColors && len(first.series) == 1 && (first.kind == kindPie || first.kind == kindBar) {
		s := first.series[0]
		cats := c.categories()
		for i := range min(len(cats), max(len(s.vals), len(s.cats)), maxSeries) {
			add(entry{text: cats[i], fill: c.pointColor(s, i, true), stroke: cv.outline(s), p: first, s: s})
		}
		return visible(out)
	}
	if first.kind == kindTree && first.tree != nil && len(first.series) > 0 {
		for _, n := range first.tree.children {
			add(entry{text: n.name, fill: cv.treeColor(first.series[0], n)})
		}
		return visible(out)
	}
	if first.special == "waterfall" {
		for i, name := range []string{"Increase", "Decrease", "Total"} {
			add(entry{text: name, fill: c.palette(i)})
		}
		return visible(out)
	}
	for _, p := range c.plots {
		for _, s := range p.series {
			name := s.name
			if name == "" {
				name = "Series " + strconv.Itoa(s.index+1)
			}
			e := entry{text: name, fill: s.color, noFill: s.noFill, stroke: cv.outline(s), p: p, s: s}
			if p.kind == kindLine || p.kind == kindScatter && !p.bubbles || p.kind == kindRadar && !p.filled {
				e.line = cv.drawsLine(p, s)
				e.stroke, e.lineWidth, e.dash = s.color, s.lineWidth, s.dash
				e.marker = cv.markerOf(p, s)
				if !e.line && e.marker == "" {
					e.marker = "square"
				}
			}
			add(e)
		}
	}
	// The trendlines follow the series, as in Excel.
	for _, p := range c.plots {
		for _, s := range p.series {
			for _, t := range s.trends {
				col, width, dash := cv.trendLine(s, t)
				if t.noLine {
					col = color.NRGBA{}
				}
				add(entry{text: trendName(s, t), line: true, stroke: col, lineWidth: width, dash: dash})
			}
		}
	}
	return visible(out)
}

// visible returns the entries that the legend does not leave out.
func visible(list []entry) []entry {
	out := list[:0]
	for _, e := range list {
		if e.text != "\x00" {
			out = append(out, e)
		}
	}
	return out
}

// outline returns the color of the outline of a bar, a slice or a square of
// the legend of a series, or transparent.
func (cv *canvas) outline(s *series) color.NRGBA {
	if s.hasLine && !s.noLine {
		return s.line
	}
	return color.NRGBA{}
}

// legendLayout is the layout of the legend: its box, and the place of each
// entry.
type legendLayout struct {
	b       box
	entries []entry
	// at are the places of the entries: the left side and the middle of
	// the line of each.
	at [][2]float64
	// symW is the width of the symbol of an entry.
	symW float64
}

// legend draws the legend, and returns the area that it leaves to the plot.
func (cv *canvas) legend(area box) box {
	c := cv.c
	if c.legend == "" || len(c.plots) == 0 || c.unsupported != "" {
		return area
	}
	es := cv.entries()
	if len(es) == 0 {
		return area
	}
	keep := area
	st := c.legendText
	em := st.size
	line := false
	for _, e := range es {
		line = line || e.line
	}
	symW := em * 0.6
	if line {
		symW = em * 2.2
	}
	symGap := em * 0.35
	widths := make([]float64, len(es))
	maxW := 0.0
	for i, e := range es {
		widths[i] = symW + symGap + cv.t.width(e.text, st)
		maxW = max(maxW, widths[i])
	}
	l := legendLayout{entries: es, symW: symW}
	vertical := c.legend == "right" || c.legend == "left" || c.legend == "top-right"
	if vertical {
		rowH := em * 1.35
		maxW = min(maxW, cv.w*0.45)
		n := max(1, min(len(es), int((area.h())/rowH)))
		lh := float64(n) * rowH
		x := area.x1 - maxW
		if c.legend == "left" {
			x = area.x0
		}
		y := (cv.h - lh) / 2
		if c.legend == "top-right" {
			y = area.y0
		}
		y = max(area.y0, y)
		l.b = box{x, y, x + maxW, y + lh}
		for i := range n {
			l.at = append(l.at, [2]float64{x, y + (float64(i)+0.5)*rowH})
		}
		switch c.legend {
		case "left":
			area.x0 = l.b.x1 + gapPart
		default:
			area.x1 = l.b.x0 - gapPart
		}
	} else {
		rowH := em * 1.3
		space := em
		limit := area.w()
		var rows [][]int
		var rowW []float64
		cur, curW := []int{}, 0.0
		for i, wi := range widths {
			add := wi
			if len(cur) > 0 {
				add += space
			}
			if len(cur) > 0 && curW+add > limit {
				rows, rowW = append(rows, cur), append(rowW, curW)
				cur, curW, add = []int{}, 0, wi
			}
			cur, curW = append(cur, i), curW+add
		}
		rows, rowW = append(rows, cur), append(rowW, curW)
		rows = rows[:min(len(rows), max(1, int(area.h()/2/rowH)))]
		lh := float64(len(rows)) * rowH
		y := area.y1 - lh
		if c.legend == "top" {
			y = area.y0
		}
		widest := 0.0
		for r, row := range rows {
			x := (cv.w - rowW[r]) / 2
			widest = max(widest, rowW[r])
			for _, i := range row {
				l.at = append(l.at, [2]float64{x, y + (float64(r)+0.5)*rowH})
				x += widths[i] + space
			}
		}
		l.entries = nil
		for _, row := range rows {
			for _, i := range row {
				l.entries = append(l.entries, es[i])
			}
		}
		l.b = box{(cv.w - widest) / 2, y, (cv.w + widest) / 2, y + lh}
		if c.legend == "top" {
			area.y0 = l.b.y1 + gapPart
		} else {
			area.y1 = l.b.y0 - gapPart
		}
	}
	if at := c.legendAt; !math.IsNaN(at[0]) && !math.IsNaN(at[1]) {
		dx, dy := at[0]*cv.w-l.b.x0, at[1]*cv.h-l.b.y0
		for i := range l.at {
			l.at[i][0] += dx
			l.at[i][1] += dy
		}
	}
	cv.drawLegend(l)
	if c.legendOver {
		return keep
	}
	return area
}

// drawLegend draws the entries of a legend.
func (cv *canvas) drawLegend(l legendLayout) {
	st := cv.c.legendText
	em := st.size
	for i, at := range l.at {
		if i >= len(l.entries) {
			break
		}
		e := l.entries[i]
		x, y := at[0], at[1]
		cv.symbol(e, x, y, l.symW, em)
		cv.t.draw(cv.dev, block{lines: []string{e.text}, st: st}, x+l.symW+em*0.35, y, alignStart, alignCenter, 0)
	}
}

// symbol draws the symbol of an entry of the legend, symW wide, from the
// left side x and the middle y of its line, for a text of the size em.
func (cv *canvas) symbol(e entry, x, y, symW, em float64) {
	if e.line {
		w := min(e.lineWidth, 3)
		if !(w > 0) {
			w = 2.25
		}
		cv.stroke(polyline([][2]float64{{x, y}, {x + symW, y}}), e.stroke, w, e.dash, false)
	}
	switch {
	case e.marker != "" && e.s != nil:
		cv.marker(e.marker, x+symW/2, y, min(cv.markerSize(e.p, e.s), em*0.8), e.s, e.stroke)
	case !e.line:
		q := em * 0.6
		b := box{x + (symW-q)/2, y - q/2, x + (symW+q)/2, y + q/2}
		fill := e.fill
		if e.noFill {
			fill = color.NRGBA{}
		}
		cv.rect(b, fill, e.stroke, thin)
	}
}

// frame draws the plot area of a chart that draws as a frame: a border and
// the name of its type.
func (cv *canvas) frame(b box) {
	c := cv.c
	cv.rect(b, color.NRGBA{}, gridGray, thin)
	name := c.unsupported
	if name == "" {
		name = "empty chart"
	}
	st := c.text
	cv.t.draw(cv.dev, block{lines: []string{name}, st: st}, (b.x0+b.x1)/2, (b.y0+b.y1)/2, alignCenter, alignCenter, 0)
}

// polyline returns an open path through points.
func polyline(points [][2]float64) *okage.Path {
	pb := okage.NewPathBuilder()
	for i, pt := range points {
		if i == 0 {
			pb.MoveTo(float32(pt[0]), float32(pt[1]))
		} else {
			pb.LineTo(float32(pt[0]), float32(pt[1]))
		}
	}
	p, ok := pb.Finish()
	if !ok {
		return nil
	}
	return p
}

// dashes maps the preset dashes of DrawingML to their dashes and gaps, in
// widths of the line.
var dashes = map[string][]float32{
	"dash": {4, 3}, "dashDot": {4, 3, 1, 3}, "dot": {1, 3}, "lgDash": {8, 3}, "lgDashDot": {8, 3, 1, 3},
	"lgDashDotDot": {8, 3, 1, 3, 1, 3}, "sysDash": {3, 1}, "sysDot": {1, 1}, "sysDashDot": {3, 1, 1, 1},
	"sysDashDotDot": {3, 1, 1, 1, 1, 1},
}

// stroke strokes a path with a color, a width, a preset dash, and round
// joins when round is set.
func (cv *canvas) stroke(p *okage.Path, col color.NRGBA, width float64, dash string, round bool) {
	if p == nil || col.A == 0 || !(width > 0) {
		return
	}
	st := okage.Stroke{Width: float32(width), MiterLimit: 4}
	if round {
		st.LineJoin, st.LineCap = okage.LineJoinRound, okage.LineCapRound
	}
	if d, ok := dashes[dash]; ok {
		arr := make([]float32, len(d))
		for i, v := range d {
			arr[i] = v * float32(width)
		}
		if sd, ok := okage.NewStrokeDash(arr, 0); ok {
			st.Dash = sd
			st.LineCap = okage.LineCapButt
		}
	}
	cv.dev.StrokePath(p, st, okage.SolidColor(rgba(col)))
}

// fill fills a path with a color.
func (cv *canvas) fill(p *okage.Path, col color.NRGBA) {
	if p == nil || col.A == 0 {
		return
	}
	cv.dev.FillPath(p, okage.FillRuleWinding, okage.SolidColor(rgba(col)))
}
