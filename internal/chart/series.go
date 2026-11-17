package chart

import (
	"image/color"
	"math"
	"slices"
	"strconv"
	"strings"

	"github.com/xo/okage"
)

// This file draws the series of a chart with axes: the bars, the lines,
// the areas, the points of a scatter chart, the bubbles, the lines and the
// bars of a stock chart and the boxes of a box and whisker chart, with the
// labels on their data.

// The defaults of the series, in points: the width of a line, and of the
// outline of a bar.
const (
	lineWidth    = 2.25
	outlineWidth = 0.75
)

// series draws the series of the plots: the areas first, then the bars,
// then the lines and the points, and the plots of the secondary axis last,
// as Office does.
func (pl *plane) series() {
	cv := pl.cv
	cv.dev.Save()
	defer cv.dev.Restore()
	// The lines and the points can go a little past the plot.
	slack := 6.0
	if r, ok := okage.RectFromLTRB(float32(pl.in.x0-slack), float32(pl.in.y0-slack), float32(pl.in.x1+slack),
		float32(pl.in.y1+slack)); ok {
		cv.dev.ClipPath(okage.PathFromRect(r), okage.FillRuleWinding)
	}
	order := []kind{kindArea, kindBar, kindBox, kindLine, kindScatter}
	for _, secondary := range []bool{false, true} {
		for _, k := range order {
			for _, p := range pl.c.plots {
				if p.kind != k || p.secondary != secondary {
					continue
				}
				switch k {
				case kindArea:
					pl.areas(p)
				case kindBar:
					pl.bars(p)
				case kindBox:
					pl.boxes(p)
				case kindLine:
					pl.lines(p)
				case kindScatter:
					pl.scatterPlot(p)
				}
			}
		}
	}
}

// value returns the value i of a series, or NaN.
func value(s *series, i int) float64 {
	if i < len(s.vals) {
		return s.vals[i]
	}
	return math.NaN()
}

// seriesName returns the name of a series, or "Series n".
func seriesName(s *series) string {
	if s.name != "" {
		return s.name
	}
	return "Series " + strconv.Itoa(s.index+1)
}

// catName returns the name of the category i of a series.
func (pl *plane) catName(s *series, i int) string {
	if i < len(s.cats) {
		return s.cats[i]
	}
	if i < len(pl.cats) {
		return pl.cats[i]
	}
	return strconv.Itoa(i + 1)
}

// labelsOf returns the labels of a series: its own, or the labels of its
// plot.
func labelsOf(p *plot, s *series) labels {
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
		f := "0%"
		if l.format != "" && strings.Contains(l.format, "%") {
			f = l.format
		}
		parts = append(parts, formatValue(percent, f))
	}
	sep := ", "
	if l.hasSep {
		sep = l.separator
	}
	s := strings.Join(parts, sep)
	if len(s) > maxText {
		s = s[:maxText]
	}
	return s
}

// tag is the label of one point: its lines, its style, its position, such
// as outEnd, and its offset from its place, as fractions of the size of the
// chart.
type tag struct {
	lines  []string
	st     textStyle
	pos    string
	dx, dy float64
}

// tagOf returns the label of the point i of a series with the labels l, or
// false for none. text makes the text of the parts that a label shows.
func tagOf(l labels, i int, text func(l labels) string) (tag, bool) {
	t := tag{st: l.text, pos: l.position}
	src := l
	if pt, ok := l.points[i]; ok {
		if pt.deleted {
			return t, false
		}
		t.st = pt.text.merge(l.text)
		t.dx, t.dy = pt.dx, pt.dy
		if pt.position != "" {
			t.pos = pt.position
		}
		if pt.hasText {
			t.lines = pt.lines
			return t, true
		}
		src = pt.labels
		src.text = t.st
	}
	if !src.show() {
		return t, false
	}
	txt := text(src)
	if txt == "" {
		return t, false
	}
	t.lines = strings.Split(txt, "\n")
	return t, true
}

// label adds a label on the data at (x, y), moved by its offset.
func (pl *plane) label(t tag, x, y float64, ha, va int) {
	if len(t.lines) == 0 || math.IsNaN(x) || math.IsNaN(y) || len(pl.labels) >= maxPoints {
		return
	}
	x += t.dx * pl.cv.w
	y += t.dy * pl.cv.h
	pl.labels = append(pl.labels, placed{b: block{lines: t.lines, st: t.st}, x: x, y: y, ha: ha, va: va})
}

// bars draws the bars of a plot.
func (pl *plane) bars(p *plot) {
	c := pl.c
	cv := pl.cv
	stacked := p.grouping == "stacked" || p.grouping == "percentStacked"
	groups := len(p.series)
	if stacked || p.special != "" || groups == 0 {
		groups = 1
	}
	o := p.overlap
	if math.IsNaN(o) {
		o = 0
		if stacked {
			o = 100
		}
	}
	ov, g := o/100, p.gap/100
	slot := pl.slot()
	bw := slot / (float64(groups) - float64(groups-1)*ov + g)
	var lows, highs [][]float64
	if stacked {
		lows, highs = pl.stack(p)
	}
	origin := pl.origin(p.secondary)
	sc := pl.scaleOf(p.secondary)
	vary := p.varyColors && len(p.series) == 1
	maxAbs := 0.0
	if p.special == "funnel" {
		for _, s := range p.series {
			for _, v := range s.vals {
				if !math.IsNaN(v) {
					maxAbs = max(maxAbs, math.Abs(v))
				}
			}
		}
	}
	for k, s := range p.series {
		gi := k
		if groups == 1 {
			gi = 0
		}
		var wf [][2]float64
		if p.special == "waterfall" {
			wf = waterfall(s)
		}
		lab := labelsOf(p, s)
		n := min(pl.n, max(len(s.vals), len(s.cats)))
		for i := range n {
			v := value(s, i)
			lo, hi := origin, v
			switch {
			case stacked:
				if k >= len(lows) || i >= len(lows[k]) {
					continue
				}
				lo, hi = lows[k][i], highs[k][i]
			case wf != nil:
				if i >= len(wf) {
					continue
				}
				lo, hi = wf[i][0], wf[i][1]
			}
			if math.IsNaN(lo) || math.IsNaN(hi) {
				continue
			}
			base := pl.center(i) - 0.5
			p0 := base + (g*bw/2+float64(gi)*bw*(1-ov))/slot
			p1 := p0 + bw/slot
			c0, c1 := pl.catCoord(p0), pl.catCoord(p1)
			v0, v1 := pl.valCoord(sc.clamp(lo), p.secondary), pl.valCoord(sc.clamp(hi), p.secondary)
			if p.special == "funnel" && maxAbs > 0 {
				mid := (pl.in.x0 + pl.in.x1) / 2
				half := math.Abs(v) / maxAbs * pl.in.w() / 2
				v0, v1 = mid-half, mid+half
			}
			var b box
			if pl.horizontal {
				b = box{min(v0, v1), min(c0, c1), max(v0, v1), max(c0, c1)}
			} else {
				b = box{min(c0, c1), min(v0, v1), max(c0, c1), max(v0, v1)}
			}
			fill := c.pointColor(s, i, vary)
			if wf != nil {
				switch {
				case s.totals[i]:
					fill = c.palette(2)
				case v < 0:
					fill = c.palette(1)
				default:
					fill = c.palette(0)
				}
			}
			if s.noFill {
				fill = color.NRGBA{}
			}
			width := s.lineWidth
			if !(width > 0) {
				width = outlineWidth
			}
			cv.rect(b, fill, cv.outline(s), width)
			if math.IsNaN(v) {
				continue
			}
			if len(s.errs) > 0 && wf == nil && p.special == "" {
				mid := (p0 + p1) / 2
				pl.errorBars(s, "y", s.vals, i, mid, hi, func(x, y float64) (float64, float64) {
					return pl.pt(x, y, p.secondary)
				})
			}
			t, ok := tagOf(lab, i, func(l labels) string {
				return labelText(l, seriesName(s), pl.catName(s, i), v, math.NaN(), s.format)
			})
			if ok {
				if t.pos == "" {
					t.pos = "outEnd"
					if stacked || p.special == "funnel" {
						t.pos = "ctr"
					}
				}
				pl.barLabel(t, b, hi >= lo)
			}
		}
	}
	for _, s := range p.series {
		if len(s.trends) > 0 && p.special == "" {
			n := min(pl.n, len(s.vals))
			pl.trendlines(s, catXs(n), s.vals[:n], pl.catAt(p))
		}
	}
}

// barLabel places the label of a bar: past its end, inside its end, in its
// middle, or inside its base.
func (pl *plane) barLabel(l tag, b box, up bool) {
	gap := l.st.size / 4
	pos := l.pos
	cx, cy := (b.x0+b.x1)/2, (b.y0+b.y1)/2
	if pl.horizontal {
		end, start := b.x1, b.x0
		ha, hb := alignStart, alignEnd
		if !up {
			end, start, ha, hb = b.x0, b.x1, alignEnd, alignStart
			gap = -gap
		}
		switch pos {
		case "inEnd":
			pl.label(l, end-gap, cy, hb, alignCenter)
		case "inBase":
			pl.label(l, start+gap, cy, ha, alignCenter)
		case "ctr":
			pl.label(l, cx, cy, alignCenter, alignCenter)
		default:
			pl.label(l, end+gap, cy, ha, alignCenter)
		}
		return
	}
	end, start := b.y0, b.y1
	va, vb := alignEnd, alignStart
	if !up {
		end, start, va, vb = b.y1, b.y0, alignStart, alignEnd
		gap = -gap
	}
	switch pos {
	case "inEnd":
		pl.label(l, cx, end+gap, alignCenter, vb)
	case "inBase":
		pl.label(l, cx, start-gap, alignCenter, va)
	case "ctr":
		pl.label(l, cx, cy, alignCenter, alignCenter)
	default:
		pl.label(l, cx, end-gap, alignCenter, va)
	}
}

// waterfall returns the bottom and the top of each bar of a waterfall: a
// total starts at zero, and the other bars at the running total.
func waterfall(s *series) [][2]float64 {
	out := make([][2]float64, len(s.vals))
	run := 0.0
	for i, v := range s.vals {
		switch {
		case math.IsNaN(v):
			out[i] = [2]float64{math.NaN(), math.NaN()}
		case s.totals[i]:
			out[i] = [2]float64{0, v}
			run = v
		default:
			out[i] = [2]float64{run, run + v}
			run += v
		}
	}
	return out
}

// curve returns the path of a line through points. A point with a NaN
// coordinate breaks the line. smooth draws a curve through the points, and
// step draws steps: start, end or center.
func curve(points [][2]float64, smooth bool, step string) *okage.Path {
	pb := okage.NewPathBuilder()
	var seg [][2]float64
	flush := func() {
		if len(seg) < 2 {
			seg = seg[:0]
			return
		}
		pb.MoveTo(float32(seg[0][0]), float32(seg[0][1]))
		for i := 1; i < len(seg); i++ {
			a, b := seg[i-1], seg[i]
			switch {
			case step == "start":
				pb.LineTo(float32(a[0]), float32(b[1]))
				pb.LineTo(float32(b[0]), float32(b[1]))
			case step == "end":
				pb.LineTo(float32(b[0]), float32(a[1]))
				pb.LineTo(float32(b[0]), float32(b[1]))
			case step == "center":
				m := (a[0] + b[0]) / 2
				pb.LineTo(float32(m), float32(a[1]))
				pb.LineTo(float32(m), float32(b[1]))
				pb.LineTo(float32(b[0]), float32(b[1]))
			case smooth:
				p0 := seg[max(0, i-2)]
				p3 := seg[min(len(seg)-1, i+1)]
				c1x, c1y := a[0]+(b[0]-p0[0])/6, a[1]+(b[1]-p0[1])/6
				c2x, c2y := b[0]-(p3[0]-a[0])/6, b[1]-(p3[1]-a[1])/6
				pb.CubicTo(float32(c1x), float32(c1y), float32(c2x), float32(c2y), float32(b[0]), float32(b[1]))
			default:
				pb.LineTo(float32(b[0]), float32(b[1]))
			}
		}
		seg = seg[:0]
	}
	for _, pt := range points {
		if math.IsNaN(pt[0]) || math.IsNaN(pt[1]) || math.IsInf(pt[0], 0) || math.IsInf(pt[1], 0) {
			flush()
			continue
		}
		seg = append(seg, pt)
	}
	flush()
	p, ok := pb.Finish()
	if !ok {
		return nil
	}
	return p
}

// lines draws the lines of a plot, with their points, and the lines and
// the bars of a stock chart.
func (pl *plane) lines(p *plot) {
	cv := pl.cv
	stacked := p.grouping == "stacked" || p.grouping == "percentStacked"
	var highs [][]float64
	if stacked {
		_, highs = pl.stack(p)
	}
	if p.hiLow {
		pl.hiLow(p)
	}
	if p.upDown {
		pl.upDown(p)
	}
	pl.dropLines(p, highs)
	for k, s := range p.series {
		n := min(pl.n, max(len(s.vals), len(s.cats)))
		points := make([][2]float64, 0, n)
		vals := make([]float64, n)
		for i := range n {
			v := value(s, i)
			if stacked && k < len(highs) && i < len(highs[k]) {
				v = highs[k][i]
			}
			if math.IsNaN(v) {
				switch {
				case s.span || pl.c.blanks == "span":
					vals[i] = math.NaN()
					continue
				case pl.c.blanks == "zero":
					v = 0
				}
			}
			vals[i] = v
			x, y := pl.pt(pl.center(i), v, p.secondary)
			points = append(points, [2]float64{x, y})
		}
		if cv.drawsLine(p, s) {
			width := s.lineWidth
			if !(width > 0) {
				width = lineWidth
			}
			cv.stroke(curve(points, p.smooth || s.smooth, p.step), s.color, width, s.dash, true)
		}
		at := pl.catAt(p)
		pl.trendlines(s, catXs(n), s.vals[:min(n, len(s.vals))], at)
		marker := cv.markerOf(p, s)
		size := cv.markerSize(p, s)
		lab := labelsOf(p, s)
		for i, v := range vals {
			if math.IsNaN(v) {
				continue
			}
			pl.errorBars(s, "y", s.vals, i, float64(i+1), v, at)
			x, y := pl.pt(pl.center(i), v, p.secondary)
			cv.marker(marker, x, y, size, s, s.color)
			t, ok := tagOf(lab, i, func(l labels) string {
				return labelText(l, seriesName(s), pl.catName(s, i), value(s, i), math.NaN(), s.format)
			})
			if ok {
				r := 0.0
				if marker != "" {
					r = size / 2
				}
				pl.pointLabel(t, x, y, r)
			}
		}
	}
}

// pointLabel places the label of a point of a line: at its right by
// default, as Office does.
func (pl *plane) pointLabel(l tag, x, y, r float64) {
	gap := r + l.st.size/4
	switch l.pos {
	case "t":
		pl.label(l, x, y-gap, alignCenter, alignEnd)
	case "b":
		pl.label(l, x, y+gap, alignCenter, alignStart)
	case "l":
		pl.label(l, x-gap, y, alignEnd, alignCenter)
	case "ctr":
		pl.label(l, x, y, alignCenter, alignCenter)
	default:
		pl.label(l, x+gap, y, alignStart, alignCenter)
	}
}

// hiLow draws the lines from the lowest to the highest value of each
// category of a stock chart.
func (pl *plane) hiLow(p *plot) {
	col := pl.c.defaultTextColor()
	for i := range pl.n {
		lo, hi := math.Inf(1), math.Inf(-1)
		for _, s := range p.series {
			if v := value(s, i); !math.IsNaN(v) {
				lo, hi = min(lo, v), max(hi, v)
			}
		}
		if math.IsInf(lo, 0) || lo == hi {
			continue
		}
		x0, y0 := pl.pt(pl.center(i), lo, p.secondary)
		x1, y1 := pl.pt(pl.center(i), hi, p.secondary)
		pl.cv.line(x0, y0, x1, y1, col, thin)
	}
}

// upDown draws the bars of a stock chart from the first value of each
// category to the last: white when the last is higher, else black.
func (pl *plane) upDown(p *plot) {
	if len(p.series) < 2 {
		return
	}
	first, last := p.series[0], p.series[len(p.series)-1]
	black := color.NRGBA{0, 0, 0, 255}
	white := color.NRGBA{255, 255, 255, 255}
	g := p.gap / 100
	half := pl.slot() / (1 + g) / 2 / pl.slot()
	for i := range pl.n {
		o, c := value(first, i), value(last, i)
		if math.IsNaN(o) || math.IsNaN(c) {
			continue
		}
		x0, y0 := pl.pt(pl.center(i)-half, o, p.secondary)
		x1, y1 := pl.pt(pl.center(i)+half, c, p.secondary)
		b := box{min(x0, x1), min(y0, y1), max(x0, x1), max(y0, y1)}
		if c >= o {
			pl.cv.rect(b, white, black, thin)
		} else {
			pl.cv.rect(b, black, black, thin)
		}
	}
}

// areas draws the areas of a plot: from the axis of categories, or from the
// series below in a stack, up to the values.
func (pl *plane) areas(p *plot) {
	cv := pl.cv
	stacked := p.grouping == "stacked" || p.grouping == "percentStacked"
	var lows, highs [][]float64
	if stacked {
		lows, highs = pl.stack(p)
	}
	origin := pl.origin(p.secondary)
	defer pl.dropLines(p, highs)
	for k, s := range p.series {
		n := min(pl.n, max(len(s.vals), len(s.cats)))
		if n == 0 {
			continue
		}
		pb := okage.NewPathBuilder()
		bottom := make([][2]float64, n)
		for i := range n {
			v, b := value(s, i), origin
			if stacked && k < len(highs) && i < len(highs[k]) {
				v, b = highs[k][i], lows[k][i]
				if math.IsNaN(v) {
					v, b = 0, 0
					for j := k - 1; j >= 0; j-- {
						if i < len(highs[j]) && !math.IsNaN(highs[j][i]) {
							v, b = highs[j][i], highs[j][i]
							break
						}
					}
				}
			}
			if math.IsNaN(v) {
				v = origin
			}
			x, y := pl.pt(pl.center(i), v, p.secondary)
			if i == 0 {
				pb.MoveTo(float32(x), float32(y))
			} else {
				pb.LineTo(float32(x), float32(y))
			}
			bx, by := pl.pt(pl.center(i), b, p.secondary)
			bottom[i] = [2]float64{bx, by}
		}
		for i := n - 1; i >= 0; i-- {
			pb.LineTo(float32(bottom[i][0]), float32(bottom[i][1]))
		}
		pb.Close()
		path, ok := pb.Finish()
		if !ok {
			continue
		}
		if !s.noFill {
			cv.fill(path, s.color)
		}
		if line := cv.outline(s); line.A > 0 {
			width := s.lineWidth
			if !(width > 0) {
				width = outlineWidth
			}
			cv.stroke(path, line, width, s.dash, true)
		}
		pl.trendlines(s, catXs(n), s.vals[:min(n, len(s.vals))], pl.catAt(p))
		lab := labelsOf(p, s)
		if !lab.any() {
			continue
		}
		for i := range n {
			v := value(s, i)
			if math.IsNaN(v) {
				continue
			}
			t, ok := tagOf(lab, i, func(l labels) string {
				return labelText(l, seriesName(s), pl.catName(s, i), v, math.NaN(), s.format)
			})
			if !ok {
				continue
			}
			top := v
			if stacked && k < len(highs) && i < len(highs[k]) {
				top = (highs[k][i] + lows[k][i]) / 2
			}
			x, y := pl.pt(pl.center(i), top, p.secondary)
			pl.label(t, x, y, alignCenter, alignCenter)
		}
	}
}

// scatterPlot draws the series of a scatter or a bubble chart.
func (pl *plane) scatterPlot(p *plot) {
	cv := pl.cv
	if p.bubbles {
		pl.bubbles(p)
		return
	}
	for _, s := range p.series {
		points := make([][2]float64, 0, len(s.vals))
		for i, v := range s.vals {
			x := xOf(s, i)
			if math.IsNaN(v) || math.IsNaN(x) {
				if pl.c.blanks != "span" {
					points = append(points, [2]float64{math.NaN(), math.NaN()})
				}
				continue
			}
			px, py := pl.xy(x, v, p.secondary)
			points = append(points, [2]float64{px, py})
		}
		if cv.drawsLine(p, s) {
			width := s.lineWidth
			if !(width > 0) {
				width = lineWidth
			}
			cv.stroke(curve(points, p.smooth || s.smooth, p.step), s.color, width, s.dash, true)
		}
		at := func(x, y float64) (float64, float64) { return pl.xy(x, y, p.secondary) }
		if len(s.trends) > 0 {
			xs := make([]float64, len(s.vals))
			for i := range xs {
				xs[i] = xOf(s, i)
			}
			pl.trendlines(s, xs, s.vals, at)
		}
		marker := cv.markerOf(p, s)
		if !p.lines && marker == "" && !s.noMarker {
			marker = autoMarkers[max(0, s.index)%len(autoMarkers)]
		}
		size := cv.markerSize(p, s)
		lab := labelsOf(p, s)
		for i, v := range s.vals {
			x := xOf(s, i)
			if math.IsNaN(v) || math.IsNaN(x) {
				continue
			}
			if len(s.errs) > 0 {
				pl.errorBars(s, "y", s.vals, i, x, v, at)
				pl.errorBars(s, "x", s.xs, i, x, v, at)
			}
			px, py := pl.xy(x, v, p.secondary)
			cv.marker(marker, px, py, size, s, s.color)
			t, ok := tagOf(lab, i, func(l labels) string {
				return labelText(l, seriesName(s), formatValue(x, pl.fmtX), v, math.NaN(), s.format)
			})
			if ok {
				pl.pointLabel(t, px, py, size/2)
			}
		}
	}
}

// bubbles draws the bubbles of a bubble chart. The area of a bubble is in
// proportion to its size, and the largest bubble is a quarter of the
// smaller side of the plot wide, at the scale of 100 percent.
func (pl *plane) bubbles(p *plot) {
	cv := pl.cv
	biggest := 0.0
	for _, s := range p.series {
		for _, z := range s.sizes {
			if !math.IsNaN(z) {
				biggest = max(biggest, z)
			}
		}
	}
	if !(biggest > 0) {
		biggest = 1
	}
	rmax := min(pl.in.w(), pl.in.h()) / 8 * p.bubbleScale / 100
	vary := p.varyColors && len(p.series) == 1
	for _, s := range p.series {
		lab := labelsOf(p, s)
		for i, v := range s.vals {
			x := xOf(s, i)
			z := 1.0
			if len(s.sizes) > 0 {
				z = math.NaN()
				if i < len(s.sizes) {
					z = s.sizes[i]
				}
			}
			if math.IsNaN(v) || math.IsNaN(x) || math.IsNaN(z) || z <= 0 {
				continue
			}
			px, py := pl.xy(x, v, p.secondary)
			r := rmax * math.Sqrt(z/biggest)
			path, ok := okage.PathFromCircle(float32(px), float32(py), float32(max(r, 0.5)))
			if !ok {
				continue
			}
			if !s.noFill {
				cv.fill(path, pl.c.pointColor(s, i, vary))
			}
			if line := cv.outline(s); line.A > 0 {
				cv.stroke(path, line, outlineWidth, "", false)
			}
			t, ok := tagOf(lab, i, func(l labels) string {
				return labelText(l, seriesName(s), formatValue(x, pl.fmtX), v, math.NaN(), s.format)
			})
			if ok {
				pl.label(t, px, py, alignCenter, alignCenter)
			}
		}
	}
}

// boxCategories returns the categories of a box and whisker chart: each
// name once, in the order of its first value.
func boxCategories(p *plot) []string {
	var out []string
	seen := map[string]bool{}
	for _, s := range p.series {
		for i := range s.vals {
			name := ""
			if i < len(s.cats) {
				name = s.cats[i]
			}
			if !seen[name] && len(out) < maxCategories {
				seen[name] = true
				out = append(out, name)
			}
		}
	}
	return out
}

// quartile returns the quartile q of sorted values, with the exclusive
// method of Excel, QUARTILE.EXC, or the inclusive one, QUARTILE.INC.
func quartile(xs []float64, q float64, inclusive bool) float64 {
	n := float64(len(xs))
	if n == 0 {
		return math.NaN()
	}
	pos := q * (n + 1)
	if inclusive {
		pos = q*(n-1) + 1
	}
	pos = max(1, min(n, pos))
	i := int(math.Floor(pos))
	f := pos - float64(i)
	if i >= len(xs) {
		return xs[len(xs)-1]
	}
	return xs[i-1] + f*(xs[i]-xs[i-1])
}

// boxes draws the boxes of a box and whisker chart: a box from the first
// to the third quartile with the median, whiskers to the smallest and the
// largest value within one and a half spans of the box, the points out of
// them, and the mean as a cross.
func (pl *plane) boxes(p *plot) {
	cv := pl.cv
	groups := max(1, len(p.series))
	g := p.gap / 100
	slot := pl.slot()
	bw := slot / (float64(groups) + g)
	for k, s := range p.series {
		edge := darker(s.color)
		groupsOf := map[string][]float64{}
		for i, v := range s.vals {
			if !math.IsNaN(v) && i < len(s.cats) {
				groupsOf[s.cats[i]] = append(groupsOf[s.cats[i]], v)
			}
		}
		for j, name := range pl.cats {
			xs := groupsOf[name]
			if len(xs) == 0 {
				continue
			}
			slices.Sort(xs)
			q1, med, q3 := quartile(xs, 0.25, false), quartile(xs, 0.5, false), quartile(xs, 0.75, false)
			iqr := q3 - q1
			lo, hi := q1, q3
			mean := 0.0
			for _, v := range xs {
				mean += v
				if v >= q1-1.5*iqr {
					lo = min(lo, v)
				}
				if v <= q3+1.5*iqr {
					hi = max(hi, v)
				}
			}
			mean /= float64(len(xs))
			base := pl.center(j) - 0.5
			p0 := base + (g*bw/2+float64(k)*bw)/slot
			p1 := p0 + bw/slot
			pm := (p0 + p1) / 2
			x0, y0 := pl.pt(p0, q3, p.secondary)
			x1, y1 := pl.pt(p1, q1, p.secondary)
			cv.rect(box{min(x0, x1), min(y0, y1), max(x0, x1), max(y0, y1)}, s.color, edge, thin)
			mx0, my0 := pl.pt(p0, med, p.secondary)
			mx1, my1 := pl.pt(p1, med, p.secondary)
			cv.line(mx0, my0, mx1, my1, edge, thin)
			wx0, wy0 := pl.pt(pm, lo, p.secondary)
			wx1, wy1 := pl.pt(pm, q1, p.secondary)
			cv.line(wx0, wy0, wx1, wy1, s.color, thin)
			wx0, wy0 = pl.pt(pm, q3, p.secondary)
			wx1, wy1 = pl.pt(pm, hi, p.secondary)
			cv.line(wx0, wy0, wx1, wy1, s.color, thin)
			for _, v := range []float64{lo, hi} {
				ax, ay := pl.pt(pm-0.25*(p1-p0), v, p.secondary)
				bx, by := pl.pt(pm+0.25*(p1-p0), v, p.secondary)
				cv.line(ax, ay, bx, by, s.color, thin)
			}
			mx, my := pl.pt(pm, mean, p.secondary)
			cv.marker("x", mx, my, 5, nil, edge)
			for _, v := range xs {
				if v < lo || v > hi {
					ox, oy := pl.pt(pm, v, p.secondary)
					cv.marker("circle", ox, oy, 4, s, s.color)
				}
			}
		}
	}
}

// darker returns a color darker by a quarter.
func darker(c color.NRGBA) color.NRGBA {
	return color.NRGBA{uint8(float64(c.R) * 0.75), uint8(float64(c.G) * 0.75), uint8(float64(c.B) * 0.75), c.A}
}
