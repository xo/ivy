package chart

import (
	"image/color"
	"math"
	"strconv"

	"github.com/xo/okage"
	"github.com/xo/okage/roxmltree"

	"github.com/xo/ivy/internal/ooxml"
)

// This file holds the lines that a chart of DrawingML adds to its series:
// the trendlines (c:trendline), the error bars (c:errBars) and the drop
// lines of a line or an area chart (c:dropLines), ECMA-376 Part 1, 21.2.2.

// The limits of the extras of a series.
const (
	// maxTrends is the number of trendlines or of error bars of a series.
	maxTrends = 4
	// maxOrder is the highest order of a polynomial trendline, as in
	// Excel.
	maxOrder = 6
	// trendSteps is the number of segments of a curved trendline.
	trendSteps = 128
	// capSize is the length of the end cap of an error bar, in points.
	capSize = 6.0
)

// trendline is a trendline of a series.
type trendline struct {
	// kind is linear, exp, log, poly, power or movingAvg.
	kind string
	// order is the order of a polynomial, and period the number of points
	// of a moving average.
	order, period int
	// forward and backward extend the line past the last and before the
	// first value of x.
	forward, backward float64
	// intercept is the value of y at x 0 of a linear, a polynomial or an
	// exponential trendline, if hasIntercept.
	intercept    float64
	hasIntercept bool
	stroke
}

// errBars are the error bars of a series.
type errBars struct {
	// dir is x or y. The bars of a chart other than a scatter chart are y.
	dir string
	// kind is both, plus or minus.
	kind string
	// valType is fixedVal, percentage, stdDev, stdErr or cust, and val the
	// value of the first three.
	valType string
	val     float64
	// plus and minus are the values of cust.
	plus, minus []float64
	noEndCap    bool
	stroke
}

// stroke is the line of a trendline, of error bars or of drop lines.
type stroke struct {
	color    color.NRGBA
	hasColor bool
	noLine   bool
	width    float64
	dash     string
}

// strokeOf reads the line of c:spPr of n.
func (p *dmlParser) strokeOf(n roxmltree.Node) stroke {
	var st stroke
	if spPr, ok := child(n, "spPr"); ok {
		st.color, st.hasColor, st.noLine, st.width, st.dash = p.lineOf(spPr)
	}
	return st
}

// trendline reads c:trendline.
func (p *dmlParser) trendline(n roxmltree.Node) trendline {
	t := trendline{kind: "linear", order: 2, period: 2, stroke: p.strokeOf(n)}
	for e := range elements(n) {
		v := ooxml.Attr(e, "val")
		switch e.TagName().Name() {
		case "trendlineType":
			t.kind = v
		case "order":
			if o, err := strconv.Atoi(v); err == nil && o >= 2 && o <= maxOrder {
				t.order = o
			}
		case "period":
			if o, err := strconv.Atoi(v); err == nil && o >= 2 && o <= maxPoints {
				t.period = o
			}
		case "forward", "backward":
			if f, ok := parseNumber(v); ok && f >= 0 && f < 1e9 {
				if e.TagName().Name() == "forward" {
					t.forward = f
				} else {
					t.backward = f
				}
			}
		case "intercept":
			if f, ok := parseNumber(v); ok {
				t.intercept, t.hasIntercept = f, true
			}
		}
	}
	return t
}

// errBars reads c:errBars.
func (p *dmlParser) errBars(n roxmltree.Node) errBars {
	b := errBars{dir: "y", kind: "both", valType: "fixedVal", stroke: p.strokeOf(n)}
	for e := range elements(n) {
		v := ooxml.Attr(e, "val")
		switch e.TagName().Name() {
		case "errDir":
			b.dir = v
		case "errBarType":
			b.kind = v
		case "errValType":
			b.valType = v
		case "val":
			if f, ok := parseNumber(v); ok && f >= 0 {
				b.val = f
			}
		case "noEndCap":
			b.noEndCap = boolVal(e)
		case "plus":
			_, b.plus, _, _ = p.data(e)
		case "minus":
			_, b.minus, _, _ = p.data(e)
		}
	}
	return b
}

// extraLine returns the color and the width of the line st, or the
// defaults: the color def and the width width.
func (cv *canvas) extraLine(st stroke, def color.NRGBA, width float64) (color.NRGBA, float64) {
	col := def
	if st.hasColor {
		col = st.color
	}
	if st.width > 0 {
		width = st.width
	}
	return col, width
}

// fit returns the function of the trendline t through the points of xs and
// ys, and false when the points do not give one.
func (t trendline) fit(xs, ys []float64) (func(float64) float64, bool) {
	var px, py []float64
	for i := range min(len(xs), len(ys)) {
		x, y := xs[i], ys[i]
		if math.IsNaN(x) || math.IsNaN(y) || math.IsInf(x, 0) || math.IsInf(y, 0) {
			continue
		}
		switch t.kind {
		case "exp":
			if y <= 0 {
				continue
			}
			y = math.Log(y)
		case "log":
			if x <= 0 {
				continue
			}
			x = math.Log(x)
		case "power":
			if x <= 0 || y <= 0 {
				continue
			}
			x, y = math.Log(x), math.Log(y)
		}
		px, py = append(px, x), append(py, y)
	}
	order := 1
	if t.kind == "poly" {
		order = t.order
	}
	if len(px) < 2 {
		return nil, false
	}
	var fixed *float64
	if t.hasIntercept && (t.kind == "linear" || t.kind == "poly" || t.kind == "exp") {
		v := t.intercept
		if t.kind == "exp" {
			if v <= 0 {
				return nil, false
			}
			v = math.Log(v)
		}
		fixed = &v
	}
	coef, ok := leastSquares(px, py, min(order, len(px)-1), fixed)
	if !ok {
		return nil, false
	}
	poly := func(x float64) float64 {
		y := 0.0
		for k := len(coef) - 1; k >= 0; k-- {
			y = y*x + coef[k]
		}
		return y
	}
	switch t.kind {
	case "exp":
		return func(x float64) float64 { return math.Exp(poly(x)) }, true
	case "log":
		return func(x float64) float64 {
			if x <= 0 {
				return math.NaN()
			}
			return poly(math.Log(x))
		}, true
	case "power":
		return func(x float64) float64 {
			if x <= 0 {
				return math.NaN()
			}
			return math.Exp(poly(math.Log(x)))
		}, true
	}
	return poly, true
}

// leastSquares returns the coefficients c0 to c[order] of the polynomial
// that fits the points best, with c0 fixed when fixed is not nil. It
// solves the normal equations with Gaussian elimination.
func leastSquares(xs, ys []float64, order int, fixed *float64) ([]float64, bool) {
	first := 0
	if fixed != nil {
		first = 1
	}
	n := order + 1 - first
	if n <= 0 {
		return nil, false
	}
	// the matrix of the normal equations, with the right side in its last
	// column
	m := make([][]float64, n)
	for i := range m {
		m[i] = make([]float64, n+1)
	}
	for k, x := range xs {
		y := ys[k]
		if fixed != nil {
			y -= *fixed
		}
		pows := make([]float64, 2*order+1)
		pows[0] = 1
		for j := 1; j < len(pows); j++ {
			pows[j] = pows[j-1] * x
		}
		for i := range n {
			for j := range n {
				m[i][j] += pows[i+first+j+first]
			}
			m[i][n] += pows[i+first] * y
		}
	}
	for c := range n {
		pivot := c
		for r := c + 1; r < n; r++ {
			if math.Abs(m[r][c]) > math.Abs(m[pivot][c]) {
				pivot = r
			}
		}
		if math.Abs(m[pivot][c]) < 1e-300 {
			return nil, false
		}
		m[c], m[pivot] = m[pivot], m[c]
		for r := range n {
			if r == c {
				continue
			}
			f := m[r][c] / m[c][c]
			for j := c; j <= n; j++ {
				m[r][j] -= f * m[c][j]
			}
		}
	}
	coef := make([]float64, order+1)
	if fixed != nil {
		coef[0] = *fixed
	}
	for i := range n {
		coef[i+first] = m[i][n] / m[i][i]
		if math.IsNaN(coef[i+first]) || math.IsInf(coef[i+first], 0) {
			return nil, false
		}
	}
	return coef, true
}

// trendlines draws the trendlines of a series. xs are the values of x of
// its points: the numbers of the categories from 1, or the values of x of
// a scatter chart, and ys its values. at maps a point to the canvas.
func (pl *plane) trendlines(s *series, xs, ys []float64, at func(x, y float64) (float64, float64)) {
	for _, t := range s.trends {
		if t.noLine {
			continue
		}
		col, width := pl.cv.extraLine(t.stroke, s.color, 1.5)
		dash := t.dash
		if !t.hasColor && dash == "" && pl.c.look == lookModern {
			dash = "sysDot"
		}
		var points [][2]float64
		if t.kind == "movingAvg" {
			points = movingAverage(xs, ys, t.period, at)
		} else {
			f, ok := t.fit(xs, ys)
			if !ok {
				continue
			}
			lo, hi := math.Inf(1), math.Inf(-1)
			for i, x := range xs {
				if i < len(ys) && !math.IsNaN(x) && !math.IsNaN(ys[i]) {
					lo, hi = min(lo, x), max(hi, x)
				}
			}
			if math.IsInf(lo, 0) {
				continue
			}
			lo, hi = lo-t.backward, hi+t.forward
			steps := trendSteps
			if t.kind == "linear" {
				steps = 1
			}
			for k := range steps + 1 {
				x := lo + (hi-lo)*float64(k)/float64(steps)
				cx, cy := at(x, f(x))
				points = append(points, [2]float64{cx, cy})
			}
		}
		if path := curve(points, false, ""); path != nil {
			pl.cv.stroke(path, col, width, dash, true)
		}
	}
}

// movingAverage returns the points of a moving average of period points.
func movingAverage(xs, ys []float64, period int, at func(x, y float64) (float64, float64)) [][2]float64 {
	var points [][2]float64
	n := min(len(xs), len(ys))
	for i := period - 1; i < n; i++ {
		sum, count := 0.0, 0
		for k := i - period + 1; k <= i; k++ {
			if !math.IsNaN(ys[k]) {
				sum += ys[k]
				count++
			}
		}
		if count == 0 || math.IsNaN(xs[i]) {
			points = append(points, [2]float64{math.NaN(), math.NaN()})
			continue
		}
		x, y := at(xs[i], sum/float64(count))
		points = append(points, [2]float64{x, y})
	}
	return points
}

// errorRange returns the low and the high end of the error bar of the
// value v of the point i of vals, and false when it has none. Excel draws
// the bars of the standard deviation from the mean of the values, and the
// others from the value.
func (b errBars) errorRange(vals []float64, i int, v float64) (lo, hi float64, ok bool) {
	if math.IsNaN(v) {
		return 0, 0, false
	}
	plus, minus := 0.0, 0.0
	at := v
	switch b.valType {
	case "fixedVal":
		plus, minus = b.val, b.val
	case "percentage":
		plus = math.Abs(v) * b.val / 100
		minus = plus
	case "stdDev", "stdErr":
		mean, sd, n := stats(vals)
		if n < 2 {
			return 0, 0, false
		}
		if b.valType == "stdDev" {
			at = mean
			plus = sd * b.val
		} else {
			plus = sd / math.Sqrt(float64(n))
		}
		minus = plus
	case "cust":
		if i < len(b.plus) && !math.IsNaN(b.plus[i]) {
			plus = b.plus[i]
		}
		if i < len(b.minus) && !math.IsNaN(b.minus[i]) {
			minus = b.minus[i]
		}
	default:
		return 0, 0, false
	}
	lo, hi = at-minus, at+plus
	switch b.kind {
	case "plus":
		lo = at
	case "minus":
		hi = at
	}
	if b.valType == "stdDev" {
		// the bar of the standard deviation goes through the mean
		return lo, hi, true
	}
	return lo, hi, lo != hi
}

// stats returns the mean, the standard deviation of a sample and the
// count of the values that are numbers.
func stats(vals []float64) (mean, sd float64, n int) {
	sum := 0.0
	for _, v := range vals {
		if !math.IsNaN(v) {
			sum += v
			n++
		}
	}
	if n == 0 {
		return 0, 0, 0
	}
	mean = sum / float64(n)
	ss := 0.0
	for _, v := range vals {
		if !math.IsNaN(v) {
			ss += (v - mean) * (v - mean)
		}
	}
	if n > 1 {
		sd = math.Sqrt(ss / float64(n-1))
	}
	return mean, sd, n
}

// errorBars draws the error bars of the point i of a series, whose value
// is v at x. at maps a point to the canvas. dir is the direction of the
// bars to draw: x or y.
func (pl *plane) errorBars(s *series, dir string, vals []float64, i int, x, v float64, at func(x, y float64) (float64, float64)) {
	for _, b := range s.errs {
		if b.noLine || b.dir != dir {
			continue
		}
		col, width := pl.cv.extraLine(b.stroke, pl.c.defaultTextColor(), thin)
		base := v
		if dir == "x" {
			base = x
		}
		lo, hi, ok := b.errorRange(vals, i, base)
		if !ok {
			continue
		}
		var x0, y0, x1, y1 float64
		if dir == "x" {
			x0, y0 = at(lo, v)
			x1, y1 = at(hi, v)
		} else {
			x0, y0 = at(x, lo)
			x1, y1 = at(x, hi)
		}
		pl.cv.line(x0, y0, x1, y1, col, width)
		if b.noEndCap {
			continue
		}
		// the caps are across the bar
		dx, dy := 0.0, capSize/2
		if math.Abs(x1-x0) < math.Abs(y1-y0) {
			dx, dy = capSize/2, 0
		}
		ends := [][2]float64{{x0, y0}, {x1, y1}}
		switch b.kind {
		case "plus":
			ends = ends[1:]
		case "minus":
			ends = ends[:1]
		}
		for _, e := range ends {
			pl.cv.line(e[0]-dx, e[1]-dy, e[0]+dx, e[1]+dy, col, width)
		}
	}
}

// dropLines draws the drop lines of a line or an area chart: from the
// highest point of each category down to the axis of categories.
func (pl *plane) dropLines(p *plot, highs [][]float64) {
	if p.drop == nil || p.drop.noLine {
		return
	}
	col, width := pl.cv.extraLine(*p.drop, pl.c.defaultTextColor(), thin)
	origin := pl.origin(p.secondary)
	for i := range pl.n {
		top := math.Inf(-1)
		for k, s := range p.series {
			v := value(s, i)
			if k < len(highs) && i < len(highs[k]) {
				v = highs[k][i]
			}
			if !math.IsNaN(v) {
				top = max(top, v)
			}
		}
		if math.IsInf(top, 0) {
			continue
		}
		x0, y0 := pl.pt(pl.center(i), origin, p.secondary)
		x1, y1 := pl.pt(pl.center(i), top, p.secondary)
		path := okage.NewPathBuilder()
		path.MoveTo(float32(x0), float32(y0))
		path.LineTo(float32(x1), float32(y1))
		if pp, ok := path.Finish(); ok {
			pl.cv.stroke(pp, col, width, p.drop.dash, false)
		}
	}
}

// catAt returns the function that maps a point of a series of categories,
// whose x is the number of its category from 1, to the canvas.
func (pl *plane) catAt(p *plot) func(x, y float64) (float64, float64) {
	return func(x, y float64) (float64, float64) {
		return pl.pt(pl.center(0)+x-1, y, p.secondary)
	}
}

// catXs returns the numbers of the categories of a series from 1, as Excel
// takes them for a trendline.
func catXs(n int) []float64 {
	xs := make([]float64, n)
	for i := range xs {
		xs[i] = float64(i + 1)
	}
	return xs
}
