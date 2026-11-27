package chart

import (
	"image/color"
	"math"
	"strconv"
)

// This file lays out and draws the plot area of a chart with axes: a chart
// of categories, such as a column, a bar, a line or an area chart, and a
// scatter chart. The axis of values is at the left and the axis of
// categories at the bottom, or the other way for the bars of c:barDir bar.
// The layout measures the labels of the axes, and gives the rest of the
// box to the plot, as Office does.

// plane is the layout of a chart with axes.
type plane struct {
	cv *canvas
	c  *Chart
	// out is the box of the plot area with its axes, and in the inner box
	// of the plot.
	out, in box
	// horizontal draws the categories up the vertical axis.
	horizontal bool
	// scatter says that the axis of categories is an axis of values.
	scatter bool
	cats    []string
	n       int
	// between puts the points between the tick marks of the categories.
	between bool
	// xs is the scale of x of a scatter chart, and vs and vs2 the scales
	// of the primary and the secondary values.
	xs, vs, vs2 scale
	has2        bool
	// formats are the number formats of the labels of x, of the primary
	// and of the secondary values.
	fmtX, fmt1, fmt2 string
	catL             catLabels
	// labels are the labels on the data, which draw last, and rich the
	// labels of the trendlines, which draw after them.
	labels []placed
	rich   []richLabel
}

// placed is a label on the data at its place.
type placed struct {
	b      block
	x, y   float64
	ha, va int
}

// catLabels is the layout of the labels of the axis of categories.
type catLabels struct {
	texts [][]string
	angle float64
	skip  int
	// depth is the size of the labels across the axis.
	depth float64
}

// plane draws the plot area of a chart with axes into the box b. With
// inner, b is the inner box, and the labels draw outside it.
func (cv *canvas) plane(b box, inner bool) {
	c := cv.c
	pl := &plane{cv: cv, c: c, out: b}
	first := c.plots[0]
	pl.scatter = first.kind == kindScatter
	for _, p := range c.plots {
		if p.kind == kindBar || p.kind == kindBox {
			pl.horizontal = p.horizontal
			break
		}
	}
	for _, p := range c.plots {
		pl.has2 = pl.has2 || p.secondary
	}
	pl.cats = c.categories()
	pl.n = len(pl.cats)
	pl.between = pl.defaultBetween()
	if c.cat.betweenSet {
		pl.between = c.cat.between
	}
	pl.formats()
	pl.layout(b, inner)
	pl.back()
	pl.series()
	pl.axes()
	for _, l := range pl.labels {
		cv.t.draw(cv.dev, l.b, l.x, l.y, l.ha, l.va, 0)
	}
	for _, l := range pl.rich {
		cv.drawRich(l)
	}
}

// defaultBetween says whether the points of a chart whose file does not
// say it are between the tick marks: for the bars and the lines, and not
// for an area chart.
func (pl *plane) defaultBetween() bool {
	for _, p := range pl.c.plots {
		if p.kind != kindArea {
			return true
		}
	}
	return false
}

// categories returns the names of the categories of the plots: the longest
// list of category names of a series, with a number for each category
// that has no name.
func (c *Chart) categories() []string {
	var names []string
	n := 0
	for _, p := range c.plots {
		if p.kind == kindScatter {
			continue
		}
		for _, s := range p.series {
			n = max(n, len(s.vals), len(s.cats))
			if len(s.cats) > len(names) {
				names = s.cats
			}
		}
	}
	if len(c.plots) > 0 && c.plots[0].kind == kindBox {
		return boxCategories(c.plots[0])
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
	return out
}

// formats sets the number formats of the labels of the axes.
func (pl *plane) formats() {
	c := pl.c
	of := func(a axis, secondary bool) string {
		if a.format != "" {
			return a.format
		}
		for _, p := range c.plots {
			if p.secondary != secondary {
				continue
			}
			if p.grouping == "percentStacked" {
				return "0%"
			}
			for _, s := range p.series {
				if s.format != "" {
					return s.format
				}
			}
		}
		return "General"
	}
	pl.fmt1, pl.fmt2 = of(c.val, false), of(c.val2, true)
	pl.fmtX = c.cat.format
	if pl.fmtX == "" {
		pl.fmtX = "General"
	}
}

// layout places the inner box of the plot in b: it measures the labels of
// the axes, which depend on the scales, which depend on the size of the
// box, twice.
func (pl *plane) layout(b box, inner bool) {
	pl.in = b
	if inner {
		pl.scales()
		pl.catL = pl.categoryLabels()
		return
	}
	in := b
	for range 2 {
		pl.in = in
		pl.scales()
		pl.catL = pl.categoryLabels()
		l, t, r, bt := pl.margins()
		next := box{b.x0 + l, b.y0 + t, b.x1 - r, b.y1 - bt}
		if next.w() < b.w()*0.2 || next.h() < b.h()*0.2 {
			next = box{b.x0 + b.w()*0.1, b.y0 + b.h()*0.1, b.x1 - b.w()*0.1, b.y1 - b.h()*0.1}
		}
		in = next
	}
	pl.in = in
	pl.scales()
	pl.catL = pl.categoryLabels()
}

// valueLength returns the length of the axis of values, and catLength the
// one of the axis of categories, in the inner box.
func (pl *plane) valueLength() float64 {
	if pl.horizontal {
		return pl.in.w()
	}
	return pl.in.h()
}

// catLength returns the length of the axis of categories.
func (pl *plane) catLength() float64 {
	if pl.horizontal {
		return pl.in.h()
	}
	return pl.in.w()
}

// intervals returns the number of major intervals that an axis of a length
// holds, for labels of the style st and of the width w along the axis.
// LibreOffice puts the labels of a vertical axis closer than Office, which
// dense says.
func intervals(length float64, st textStyle, along bool, w float64, dense bool) int {
	if along {
		return int(length / (w + st.size*1.2))
	}
	k := 1.6
	if dense {
		k = 1.15
	}
	return int(length / (st.size * lineHeight * k))
}

// scales computes the scales of the axes for the inner box.
func (pl *plane) scales() {
	c := pl.c
	lo, hi := pl.extent(false)
	a := percentAxis(c.val, c.plots, false, lo)
	n := intervals(pl.valueLength(), c.val.text, pl.horizontal, pl.cv.t.width(formatValue(rough(lo, hi), pl.fmt1), c.val.text), c.look == lookLibre)
	pl.vs = autoScale(lo, hi, a, n)
	if pl.has2 {
		lo2, hi2 := pl.extent(true)
		a2 := c.val2
		if !a2.present {
			a2 = newAxis()
		}
		pl.vs2 = autoScale(lo2, hi2, percentAxis(a2, c.plots, true, lo2), n)
	}
	if pl.scatter {
		xlo, xhi := pl.extentX()
		n := intervals(pl.in.w(), c.cat.text, true, pl.cv.t.width(formatValue(rough(xlo, xhi), pl.fmtX), c.cat.text), false)
		pl.xs = autoScale(xlo, xhi, c.cat, n)
	}
}

// percentAxis returns the axis a of the plots of percentages, which ends at
// 100 percent, and at -100 percent for negative values, unless the file
// gives its limits.
func percentAxis(a axis, plots []*plot, secondary bool, lo float64) axis {
	seen := false
	for _, p := range plots {
		if p.secondary != secondary || p.kind == kindPie || p.kind == kindRadar {
			continue
		}
		if p.grouping != "percentStacked" {
			return a
		}
		seen = true
	}
	if !seen {
		return a
	}
	if math.IsNaN(a.max) {
		a.max = 1
	}
	if math.IsNaN(a.min) && lo < 0 {
		a.min = -1
	}
	return a
}

// rough returns the value of lo and hi farthest from zero, rounded to one
// digit, as the widest label of an axis of values is near it.
func rough(lo, hi float64) float64 {
	v := max(math.Abs(lo), math.Abs(hi))
	if v == 0 || math.IsNaN(v) || math.IsInf(v, 0) {
		return v
	}
	k := math.Pow(10, math.Floor(math.Log10(v)))
	v = math.Round(v/k) * k
	if lo < 0 && math.Abs(lo) > hi {
		v = -v
	}
	return v
}

// extent returns the smallest and the largest value of the plots of the
// primary or the secondary axis, as their bars and their lines reach them.
func (pl *plane) extent(secondary bool) (lo, hi float64) {
	lo, hi = math.Inf(1), math.Inf(-1)
	add := func(v float64) {
		if !math.IsNaN(v) && !math.IsInf(v, 0) {
			lo, hi = min(lo, v), max(hi, v)
		}
	}
	for _, p := range pl.c.plots {
		if p.secondary != secondary || p.kind == kindPie || p.kind == kindRadar {
			continue
		}
		switch {
		case p.kind == kindBox:
			for _, s := range p.series {
				for _, v := range s.vals {
					add(v)
				}
			}
		case p.special == "waterfall":
			for _, s := range p.series {
				for _, b := range waterfall(s) {
					add(b[0])
					add(b[1])
				}
			}
		case p.grouping == "stacked" || p.grouping == "percentStacked":
			lows, highs := pl.stack(p)
			for k := range lows {
				for i := range lows[k] {
					add(lows[k][i])
					add(highs[k][i])
				}
			}
		default:
			for _, s := range p.series {
				for _, v := range s.vals {
					add(v)
				}
				if p.special == "" {
					trendExtent(p, s, add)
				}
			}
		}
	}
	if math.IsInf(lo, 0) {
		return 0, 1
	}
	return lo, hi
}

// trendExtent calls add with the values of the trendlines of a series, as
// Excel takes them into the scale of the axis of values.
func trendExtent(p *plot, s *series, add func(float64)) {
	if len(s.trends) == 0 {
		return
	}
	xs := catXs(len(s.vals))
	if p.kind == kindScatter {
		for i := range xs {
			xs[i] = xOf(s, i)
		}
	}
	for _, t := range s.trends {
		if t.noLine || t.kind == "movingAvg" {
			continue
		}
		f, ok := t.fit(xs, s.vals)
		if !ok {
			continue
		}
		lo, hi := math.Inf(1), math.Inf(-1)
		for i, x := range xs {
			if !math.IsNaN(x) && !math.IsNaN(s.vals[i]) {
				lo, hi = min(lo, x), max(hi, x)
			}
		}
		if math.IsInf(lo, 0) {
			continue
		}
		lo, hi = lo-t.backward, hi+t.forward
		for k := range trendSteps + 1 {
			add(f(lo + (hi-lo)*float64(k)/trendSteps))
		}
	}
}

// extentX returns the smallest and the largest value of x of a scatter
// chart.
func (pl *plane) extentX() (lo, hi float64) {
	lo, hi = math.Inf(1), math.Inf(-1)
	for _, p := range pl.c.plots {
		for _, s := range p.series {
			for i := range s.vals {
				x := xOf(s, i)
				if !math.IsNaN(x) {
					lo, hi = min(lo, x), max(hi, x)
				}
			}
		}
	}
	if math.IsInf(lo, 0) {
		return 0, 1
	}
	return lo, hi
}

// xOf returns the value of x of the point i of a series of a scatter
// chart: its value of x, or its number from 1.
func xOf(s *series, i int) float64 {
	if len(s.xs) == 0 {
		return float64(i + 1)
	}
	if i < len(s.xs) {
		return s.xs[i]
	}
	return math.NaN()
}

// stack returns the bottom and the top of each point of the series of a
// stacked plot. The positive values stack up from zero, and the negative
// ones down. A plot of percentages stacks the shares of the total of each
// category.
func (pl *plane) stack(p *plot) (lows, highs [][]float64) {
	n := 0
	for _, s := range p.series {
		n = max(n, len(s.vals))
	}
	n = min(n, maxCategories)
	pos, neg, total := make([]float64, n), make([]float64, n), make([]float64, n)
	if p.grouping == "percentStacked" {
		for _, s := range p.series {
			for i, v := range s.vals[:min(len(s.vals), n)] {
				if !math.IsNaN(v) {
					total[i] += math.Abs(v)
				}
			}
		}
	}
	for _, s := range p.series {
		lo, hi := make([]float64, n), make([]float64, n)
		for i := range n {
			v := math.NaN()
			if i < len(s.vals) {
				v = s.vals[i]
			}
			if math.IsNaN(v) && pl.c.blanks == "zero" {
				v = 0
			}
			if p.grouping == "percentStacked" {
				if total[i] == 0 {
					v = math.NaN()
				} else {
					v /= total[i]
				}
			}
			if math.IsNaN(v) {
				lo[i], hi[i] = math.NaN(), math.NaN()
				continue
			}
			if v >= 0 {
				lo[i], hi[i] = pos[i], pos[i]+v
				pos[i] += v
			} else {
				lo[i], hi[i] = neg[i], neg[i]+v
				neg[i] += v
			}
		}
		lows, highs = append(lows, lo), append(highs, hi)
	}
	return lows, highs
}

// tickOut returns the length of a tick mark outside of the plot.
func (pl *plane) tickOut(a axis) float64 {
	switch pl.tick(a.majorTick) {
	case "out", "cross":
		return tickLen
	}
	return 0
}

// tick returns the tick marks of an axis, with the default of the look.
func (pl *plane) tick(t string) string {
	if t != "" {
		return t
	}
	if pl.c.look == lookModern {
		return "none"
	}
	return "out"
}

// valueTexts returns the labels of the major ticks of a scale.
func valueTexts(s scale, a axis, format string) ([]float64, []string) {
	ticks := s.ticks(false)
	texts := make([]string, len(ticks))
	for i, v := range ticks {
		if a.unit > 0 {
			v /= a.unit
		}
		texts[i] = formatValue(v, format)
	}
	return ticks, texts
}

// widest returns the width of the widest text.
func (pl *plane) widest(texts []string, st textStyle) float64 {
	w := 0.0
	for _, t := range texts {
		w = max(w, pl.cv.t.width(t, st))
	}
	return w
}

// margins returns the space of the labels and the titles of the axes on
// each side of the inner box.
func (pl *plane) margins() (left, top, right, bottom float64) {
	c := pl.c
	t := pl.cv.t
	// The value axis.
	var valDepth, valHalf float64
	if c.val.shown() {
		_, texts := valueTexts(pl.vs, c.val, pl.fmt1)
		st := c.val.text
		if pl.horizontal {
			valDepth = st.size*lineHeight + gapLabel + pl.tickOut(c.val)
			if len(texts) > 0 {
				valHalf = t.width(texts[len(texts)-1], st) / 2
			}
		} else {
			valDepth = pl.widest(texts, st) + gapLabel + pl.tickOut(c.val)
			valHalf = st.size * lineHeight / 2
		}
	}
	if c.val.title != "" && !c.val.deleted {
		valDepth += c.val.titleBlock().lineSize(0)*lineHeight + gapLabel
	}
	// The axis of categories, or of x.
	var catDepth, catHalf float64
	if pl.tableOnCats() {
		catDepth = pl.catL.depth
	} else if c.cat.shown() {
		catDepth = pl.catL.depth + gapLabel + pl.tickOut(c.cat)
		if pl.scatter || !pl.between {
			st := c.cat.text
			if pl.horizontal {
				catHalf = st.size * lineHeight / 2
			} else if len(pl.catL.texts) > 0 && pl.catL.angle == 0 {
				catHalf = pl.widest(pl.catL.texts[len(pl.catL.texts)-1], st) / 2
			}
		}
	}
	if c.cat.title != "" && !c.cat.deleted {
		catDepth += c.cat.titleBlock().lineSize(0)*lineHeight + gapLabel
	}
	// The secondary axis of values.
	var depth2 float64
	if pl.has2 && c.val2.shown() {
		_, texts := valueTexts(pl.vs2, c.val2, pl.fmt2)
		st := c.val2.text
		if pl.horizontal {
			depth2 = st.size*lineHeight + gapLabel + pl.tickOut(c.val2)
		} else {
			depth2 = pl.widest(texts, st) + gapLabel + pl.tickOut(c.val2)
		}
		if c.val2.title != "" {
			depth2 += c.val2.titleBlock().lineSize(0)*lineHeight + gapLabel
		}
	}
	valRight := pl.valueEnd()
	if pl.horizontal {
		left, bottom = catDepth, valDepth
		top, right = catHalf, max(valHalf, 2)
		if valRight {
			top, bottom = max(valDepth, catHalf), 2
		}
		if depth2 > 0 {
			top = max(top, depth2)
		}
		if pl.hasTable() {
			// The data table goes below the labels of the values.
			left = max(left, pl.tableNames())
			bottom += pl.tableHeight()
		}
		return left, max(top, 2), right, bottom
	}
	left, bottom, top = valDepth, catDepth, max(valHalf, 2)
	right = max(catHalf, 2)
	if pl.hasTable() {
		left = max(left, pl.tableNames())
		right = max(right, 2)
	}
	if valRight {
		left, right = max(catHalf, 2), valDepth
	}
	if depth2 > 0 {
		right = max(right, depth2)
	}
	return left, top, right, bottom
}

// categoryLabels lays out the labels of the axis of categories: in one
// line, in wrapped lines, or turned by 45 degrees, and one of skip when
// they do not fit.
func (pl *plane) categoryLabels() catLabels {
	c := pl.c
	t := pl.cv.t
	st := c.cat.text
	lh := st.size * lineHeight
	var out catLabels
	if pl.tableOnCats() {
		// the data table holds the names of the categories
		out.depth = pl.tableHeight()
		return out
	}
	if !c.cat.shown() {
		return out
	}
	if pl.scatter {
		_, texts := valueTexts(pl.xs, c.cat, pl.fmtX)
		for _, s := range texts {
			out.texts = append(out.texts, []string{s})
		}
		out.skip = 1
		out.depth = lh
		return out
	}
	out.texts = make([][]string, len(pl.cats))
	for i, s := range pl.cats {
		out.texts[i] = []string{s}
	}
	if pl.n == 0 {
		return out
	}
	slots := float64(pl.n)
	if !pl.between && pl.n > 1 {
		slots = float64(pl.n - 1)
	}
	slot := pl.catLength() / slots
	out.skip = max(1, c.cat.skip)
	if pl.horizontal {
		maxW := pl.cv.w * 0.3
		w := 0.0
		lines := 1
		for i, s := range pl.cats {
			if t.width(s, st) > maxW {
				out.texts[i] = t.wrap(s, st, maxW)
			}
			lines = max(lines, len(out.texts[i]))
			w = max(w, pl.widest(out.texts[i], st))
		}
		if c.cat.skip == 0 {
			out.skip = max(1, int(math.Ceil(float64(lines)*lh/slot-1e-9)))
		}
		out.depth = w
		return out
	}
	if c.cat.hasRotation && c.cat.rotation != 0 {
		out.angle = c.cat.rotation
	}
	widest := 0.0
	for _, s := range pl.cats {
		widest = max(widest, t.width(s, st))
	}
	if out.angle == 0 && widest <= slot*0.98*float64(out.skip) {
		out.depth = lh
		return out
	}
	if out.angle == 0 {
		// Wrap the labels at their spaces when each word fits.
		lines, fits := 1, true
		wrapped := make([][]string, len(pl.cats))
		for i, s := range pl.cats {
			wrapped[i] = t.wrap(s, st, slot*0.98)
			lines = max(lines, len(wrapped[i]))
			if pl.widest(wrapped[i], st) > slot*0.98 {
				fits = false
			}
		}
		if fits && lines <= 3 {
			out.texts = wrapped
			out.depth = float64(lines) * lh
			return out
		}
		out.angle = 45
	}
	a := math.Abs(out.angle) * math.Pi / 180
	out.depth = min(widest*math.Sin(a)+lh*math.Cos(a), pl.cv.h*0.4)
	if c.cat.skip == 0 && math.Sin(a) > 0 {
		out.skip = max(1, int(math.Ceil(lh/math.Sin(a)/slot-1e-9)))
	}
	return out
}

// catPos returns the place of the category position pos on the axis, from
// 0 at its start to 1 at its end. The middle of the category i is at i+0.5
// between the tick marks, and at i on them.
func (pl *plane) catFrac(pos float64) float64 {
	var f float64
	switch {
	case pl.between:
		f = pos / float64(max(1, pl.n))
	case pl.n > 1:
		f = pos / float64(pl.n-1)
	default:
		f = 0.5
	}
	if pl.c.cat.reverse {
		f = 1 - f
	}
	return f
}

// center returns the position of the middle of the category i.
func (pl *plane) center(i int) float64 {
	if pl.between {
		return float64(i) + 0.5
	}
	return float64(i)
}

// catCoord returns the coordinate of a category position along its axis.
func (pl *plane) catCoord(pos float64) float64 {
	f := pl.catFrac(pos)
	if pl.horizontal {
		return pl.in.y1 - f*pl.in.h()
	}
	return pl.in.x0 + f*pl.in.w()
}

// slot returns the length of one category along its axis.
func (pl *plane) slot() float64 {
	slots := float64(max(1, pl.n))
	if !pl.between && pl.n > 1 {
		slots = float64(pl.n - 1)
	}
	return pl.catLength() / slots
}

// scaleOf returns the scale of the values of a plot.
func (pl *plane) scaleOf(secondary bool) scale {
	if secondary && pl.has2 {
		return pl.vs2
	}
	return pl.vs
}

// valCoord returns the coordinate of a value along the axis of values.
func (pl *plane) valCoord(v float64, secondary bool) float64 {
	f := pl.scaleOf(secondary).frac(v)
	if pl.horizontal {
		return pl.in.x0 + f*pl.in.w()
	}
	return pl.in.y1 - f*pl.in.h()
}

// pt returns the point of a value at a category position.
func (pl *plane) pt(pos, v float64, secondary bool) (x, y float64) {
	if pl.horizontal {
		return pl.valCoord(v, secondary), pl.catCoord(pos)
	}
	return pl.catCoord(pos), pl.valCoord(v, secondary)
}

// xy returns the point of a scatter chart.
func (pl *plane) xy(x, v float64, secondary bool) (float64, float64) {
	return pl.in.x0 + pl.xs.frac(x)*pl.in.w(), pl.in.y1 - pl.scaleOf(secondary).frac(v)*pl.in.h()
}

// origin returns the value where the axis of categories crosses the axis
// of values, where the bars start.
func (pl *plane) origin(secondary bool) float64 {
	a := pl.c.cat
	if secondary && pl.c.cat2.present {
		a = pl.c.cat2
	}
	return pl.scaleOf(secondary).cross(a.crosses, a.crossesAt)
}

// colors of the lines of the axes and the gridlines of each look.
func (pl *plane) gridColor(minor bool) color.NRGBA {
	switch pl.c.look {
	case lookModern:
		if minor {
			return color.NRGBA{0xf2, 0xf2, 0xf2, 255}
		}
		return gridGray
	case lookLibre:
		if minor {
			return color.NRGBA{0xdd, 0xdd, 0xdd, 255}
		}
		return libreGray
	}
	if minor {
		return gridGray
	}
	return officeGray
}

// axisColor returns the color of the line of an axis, or transparent for
// none.
func (pl *plane) axisColor(a axis, values bool) color.NRGBA {
	switch {
	case a.noLine:
		return color.NRGBA{}
	case a.hasLine:
		return a.line
	}
	switch pl.c.look {
	case lookModern:
		if values {
			return color.NRGBA{}
		}
		return gridGray
	case lookLibre:
		return libreGray
	}
	return officeGray
}

// back draws the plot area and its gridlines.
func (pl *plane) back() {
	c := pl.c
	cv := pl.cv
	border := c.plotBorder
	cv.rect(pl.in, c.plotFill, color.NRGBA{}, 0)
	grid := func(a axis, s scale, secondary bool) {
		for _, minor := range []bool{true, false} {
			on, col, set := a.grid, a.gridColor, a.hasGridColor
			if minor {
				on, col, set = a.minorGrid, a.minorColor, a.hasMinorColor
			}
			if !on {
				continue
			}
			if !set {
				col = pl.gridColor(minor)
			}
			for _, v := range s.ticks(minor) {
				if pl.horizontal {
					x := pl.in.x0 + s.frac(v)*pl.in.w()
					cv.line(x, pl.in.y0, x, pl.in.y1, col, thin)
				} else {
					y := pl.in.y1 - s.frac(v)*pl.in.h()
					cv.line(pl.in.x0, y, pl.in.x1, y, col, thin)
				}
			}
		}
		_ = secondary
	}
	grid(c.val, pl.vs, false)
	if pl.has2 && c.val2.present {
		grid(c.val2, pl.vs2, true)
	}
	// The gridlines of the categories, or of x.
	for _, minor := range []bool{true, false} {
		on, col, set := c.cat.grid, c.cat.gridColor, c.cat.hasGridColor
		if minor {
			on, col, set = c.cat.minorGrid, c.cat.minorColor, c.cat.hasMinorColor
		}
		if !on {
			continue
		}
		if !set {
			col = pl.gridColor(minor)
		}
		if pl.scatter {
			for _, v := range pl.xs.ticks(minor) {
				x := pl.in.x0 + pl.xs.frac(v)*pl.in.w()
				cv.line(x, pl.in.y0, x, pl.in.y1, col, thin)
			}
			continue
		}
		if minor {
			continue
		}
		for _, pos := range pl.catTicks() {
			if pl.horizontal {
				y := pl.catCoord(pos)
				cv.line(pl.in.x0, y, pl.in.x1, y, col, thin)
			} else {
				x := pl.catCoord(pos)
				cv.line(x, pl.in.y0, x, pl.in.y1, col, thin)
			}
		}
	}
	cv.rect(pl.in, color.NRGBA{}, border, thin)
}

// catTicks returns the category positions of the tick marks of the axis of
// categories: between the categories, or at them.
func (pl *plane) catTicks() []float64 {
	var out []float64
	if pl.between {
		for i := 0; i <= pl.n && len(out) <= maxCategories; i++ {
			out = append(out, float64(i))
		}
		return out
	}
	for i := 0; i < pl.n && len(out) <= maxCategories; i++ {
		out = append(out, float64(i))
	}
	return out
}

// axes draws the lines, the tick marks, the labels and the titles of the
// axes.
func (pl *plane) axes() {
	c := pl.c
	cv := pl.cv
	t := cv.t
	in := pl.in
	// The axis of categories crosses the axis of values at the origin. Office
	// draws the axis of categories of a box and whisker chart at the bottom
	// of the plot, so that its line does not cross the boxes at zero.
	o := pl.vs.cross(c.cat.crosses, c.cat.crossesAt)
	if c.plots[0].kind == kindBox {
		o = pl.vs.cross("min", math.NaN())
	}
	catAt := pl.valCoord(o, false)
	if pl.scatter {
		catAt = in.y1 - pl.vs.frac(o)*in.h()
	}
	// The axis of values crosses the axis of categories at its start, or at
	// its end.
	valAt := in.x0
	if pl.horizontal {
		valAt = in.y1
	}
	valEnd := pl.valueEnd()
	if valEnd {
		valAt = in.x1
		if pl.horizontal {
			valAt = in.y0
		}
	}
	if pl.scatter {
		x := pl.xs.cross(c.val.crosses, c.val.crossesAt)
		valAt = in.x0 + pl.xs.frac(x)*in.w()
	}
	if c.cat.present && !c.cat.deleted {
		pl.catAxis(catAt)
	}
	if pl.hasTable() {
		pl.table()
	}
	if c.val.present && !c.val.deleted {
		pl.valueAxis(c.val, pl.vs, pl.fmt1, valAt, valEnd)
	}
	if pl.has2 && c.val2.present && !c.val2.deleted {
		end := in.x1
		if pl.horizontal {
			end = in.y0
		}
		pl.valueAxis(c.val2, pl.vs2, pl.fmt2, end, true)
	}
	// The titles of the axes.
	if c.cat.title != "" && !c.cat.deleted {
		b := c.cat.titleBlock()
		if pl.horizontal {
			t.draw(cv.dev, b, pl.out.x0, (in.y0+in.y1)/2, alignCenter, alignStart, 90)
		} else {
			t.draw(cv.dev, b, (in.x0+in.x1)/2, pl.out.y1, alignCenter, alignEnd, 0)
		}
	}
	if c.val.title != "" && !c.val.deleted {
		b := c.val.titleBlock()
		if pl.horizontal {
			t.draw(cv.dev, b, (in.x0+in.x1)/2, pl.out.y1, alignCenter, alignEnd, 0)
		} else {
			t.draw(cv.dev, b, pl.out.x0, (in.y0+in.y1)/2, alignCenter, alignStart, 90)
		}
	}
	if pl.has2 && c.val2.title != "" && !c.val2.deleted && !pl.horizontal {
		b := c.val2.titleBlock()
		t.draw(cv.dev, b, pl.out.x1, (in.y0+in.y1)/2, alignCenter, alignStart, -90)
	}
}

// valueEnd says whether the axis of values is at the end of the axis of
// categories: at the right, or at the top for the bars. It crosses the
// axis of categories at its maximum, or at its minimum when that is
// reversed.
func (pl *plane) valueEnd() bool {
	return (pl.c.val.crosses == "max") != pl.c.cat.reverse
}

// catAxis draws the axis of categories, or of x, whose line is at the
// coordinate at across the plot.
func (pl *plane) catAxis(at float64) {
	c := pl.c
	cv := pl.cv
	in := pl.in
	a := c.cat
	col := pl.axisColor(a, pl.scatter)
	if pl.horizontal {
		cv.line(at, in.y0, at, in.y1, col, thin)
	} else {
		cv.line(in.x0, at, in.x1, at, col, thin)
	}
	// The tick marks.
	var ticks []float64
	if pl.scatter {
		for _, v := range pl.xs.ticks(false) {
			ticks = append(ticks, in.x0+pl.xs.frac(v)*in.w())
		}
	} else {
		for _, pos := range pl.catTicks() {
			ticks = append(ticks, pl.catCoord(pos))
		}
	}
	tickCol := col
	if tickCol.A == 0 {
		tickCol = pl.axisColor(axis{}, false)
	}
	if pl.tableOnCats() {
		// the lines of the data table take the place of the tick marks
		return
	}
	pl.ticks(ticks, at, pl.tick(a.majorTick), tickLen, !pl.horizontal, false, tickCol)
	if !a.shown() {
		return
	}
	// The labels go next to the line of the axis, or at the low or the
	// high side of the plot.
	labelAt := at
	switch a.labelPos {
	case "low":
		labelAt = in.y1
		if pl.horizontal {
			labelAt = in.x0
		}
	case "high":
		labelAt = in.y0
		if pl.horizontal {
			labelAt = in.x1
		}
	}
	off := pl.tickOut(a) + gapLabel
	st := a.text
	l := pl.catL
	for i, lines := range l.texts {
		if l.skip > 1 && i%l.skip != 0 {
			continue
		}
		var pos float64
		if pl.scatter {
			pos = math.NaN()
		} else {
			pos = pl.catCoord(pl.center(i))
		}
		b := block{lines: lines, st: st}
		switch {
		case pl.scatter:
			ts := pl.xs.ticks(false)
			if i >= len(ts) {
				continue
			}
			x := in.x0 + pl.xs.frac(ts[i])*in.w()
			cv.t.draw(cv.dev, b, x, labelAt+off, alignCenter, alignStart, 0)
		case pl.horizontal:
			cv.t.draw(cv.dev, b, labelAt-off, pos, alignEnd, alignCenter, 0)
		case l.angle > 0:
			cv.t.draw(cv.dev, b, pos, labelAt+off, alignEnd, alignStart, l.angle)
		case l.angle < 0:
			cv.t.draw(cv.dev, b, pos, labelAt+off, alignStart, alignStart, l.angle)
		default:
			cv.t.draw(cv.dev, b, pos, labelAt+off, alignCenter, alignStart, 0)
		}
	}
}

// ticks draws tick marks at the coordinates of ticks, on a line of an axis
// at the coordinate at. across says that the axis is horizontal, so that
// its tick marks are vertical. flip puts the outside of the axis up or to
// the right.
func (pl *plane) ticks(ticks []float64, at float64, kind string, length float64, across, flip bool, col color.NRGBA) {
	var a, b float64
	switch kind {
	case "out":
		a, b = 0, length
	case "in":
		a, b = -length, 0
	case "cross":
		a, b = -length, length
	default:
		return
	}
	if !across {
		a, b = -b, -a
	}
	if flip {
		a, b = -b, -a
	}
	for _, v := range ticks {
		if across {
			pl.cv.line(v, at+a, v, at+b, col, thin)
		} else {
			pl.cv.line(at+a, v, at+b, v, col, thin)
		}
	}
}

// valueAxis draws an axis of values whose line is at the coordinate at.
// end says that its labels are on the outer side of the end of the plot:
// right, or top for the bars.
func (pl *plane) valueAxis(a axis, s scale, format string, at float64, end bool) {
	cv := pl.cv
	in := pl.in
	col := pl.axisColor(a, true)
	if pl.horizontal {
		cv.line(in.x0, at, in.x1, at, col, thin)
	} else {
		cv.line(at, in.y0, at, in.y1, col, thin)
	}
	values, texts := valueTexts(s, a, format)
	coords := make([]float64, len(values))
	for i, v := range values {
		if pl.horizontal {
			coords[i] = in.x0 + s.frac(v)*in.w()
		} else {
			coords[i] = in.y1 - s.frac(v)*in.h()
		}
	}
	tickCol := col
	if tickCol.A == 0 {
		tickCol = pl.axisColor(axis{}, false)
	}
	pl.ticks(coords, at, pl.tick(a.majorTick), tickLen, pl.horizontal, end, tickCol)
	if mt := pl.tick(a.minorTick); a.minorTick != "" && mt != "none" {
		var minor []float64
		for _, v := range s.ticks(true) {
			if pl.horizontal {
				minor = append(minor, in.x0+s.frac(v)*in.w())
			} else {
				minor = append(minor, in.y1-s.frac(v)*in.h())
			}
		}
		pl.ticks(minor, at, mt, minorLen, pl.horizontal, end, tickCol)
	}
	if !a.shown() {
		return
	}
	labelAt := at
	switch a.labelPos {
	case "low":
		labelAt, end = in.x0, false
		if pl.horizontal {
			labelAt = in.y1
		}
	case "high":
		labelAt, end = in.x1, true
		if pl.horizontal {
			labelAt = in.y0
		}
	}
	off := pl.tickOut(a) + gapLabel
	for i, txt := range texts {
		b := block{lines: []string{txt}, st: a.text}
		switch {
		case pl.horizontal && end:
			cv.t.draw(cv.dev, b, coords[i], labelAt-off, alignCenter, alignEnd, 0)
		case pl.horizontal:
			cv.t.draw(cv.dev, b, coords[i], labelAt+off, alignCenter, alignStart, 0)
		case end:
			cv.t.draw(cv.dev, b, labelAt+off, coords[i], alignStart, alignCenter, 0)
		default:
			cv.t.draw(cv.dev, b, labelAt-off, coords[i], alignEnd, alignCenter, 0)
		}
	}
}
