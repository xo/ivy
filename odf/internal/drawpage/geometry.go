package drawpage

import (
	"errors"
	"math"
	"strconv"
	"strings"

	"github.com/xo/okage"
	"github.com/xo/okage/roxmltree"
	"github.com/xo/okage/svg"

	"github.com/xo/ivy/internal/drawingml"
)

// The limits of a geometry.
const (
	// maxEquations is the number of equations of an enhanced geometry.
	maxEquations = 2048
	// maxPathTokens is the number of tokens of an enhanced path or of a
	// list of points.
	maxPathTokens = 1 << 16
	// maxExprDepth is the depth of nesting of a formula, and of the
	// equations that a formula reads.
	maxExprDepth = 64
	// maxCoord is the largest magnitude of a coordinate, in the units of
	// the viewBox.
	maxCoord = 1e9
)

// errFormula reports a formula that cannot be read.
var errFormula = errors.New("malformed formula")

// geometry is a draw:enhanced-geometry that is ready to evaluate.
type geometry struct {
	vx, vy, vw, vh float64
	modifiers      []float64
	names          map[string]int
	formulas       []expr
	values         []float64
	state          []uint8
	path           string
	textAreas      string
	// w and h are the size of the shape, in points.
	w, h float64
}

// parseGeometry reads draw:enhanced-geometry for a shape of w by h points.
func parseGeometry(g roxmltree.Node, w, h float64) *geometry {
	e := &geometry{vw: 21600, vh: 21600, names: map[string]int{}, w: w, h: h}
	v := numbers(at(g, nsSVG, "viewBox"))
	switch {
	case len(v) == 4 && v[2] > 0 && v[3] > 0:
		e.vx, e.vy, e.vw, e.vh = v[0], v[1], v[2], v[3]
	case len(v) == 4:
		// an empty viewBox is the size of the shape in 1/100 mm, which a
		// shape of DrawingML that LibreOffice writes uses
		e.vx, e.vy = v[0], v[1]
		e.vw, e.vh = max(e.variable("logwidth"), 1e-3), max(e.variable("logheight"), 1e-3)
	}
	e.modifiers = numbers(at(g, nsDraw, "modifiers"))
	// LibreOffice writes the whole path of a shape of DrawingML in its own
	// namespace, and a simpler path in draw:enhanced-path for the other
	// programs
	e.path = at(g, nsDrawOOo, "enhanced-path")
	if e.path == "" {
		e.path = at(g, nsDraw, "enhanced-path")
	}
	e.textAreas = at(g, nsDraw, "text-areas")
	for c := range g.Children() {
		if !is(c, nsDraw, "equation") || len(e.formulas) >= maxEquations {
			continue
		}
		x, err := parseExpr(at(c, nsDraw, "formula"))
		if err != nil {
			x = constExpr(0)
		}
		e.names[at(c, nsDraw, "name")] = len(e.formulas)
		e.formulas = append(e.formulas, x)
	}
	e.values = make([]float64, len(e.formulas))
	e.state = make([]uint8, len(e.formulas))
	return e
}

// numbers parses the numbers of s, which spaces or commas part.
func numbers(s string) []float64 {
	var v []float64
	for f := range strings.FieldsFuncSeq(s, func(r rune) bool { return r == ' ' || r == ',' || r == '\t' || r == '\n' }) {
		if len(v) >= maxPathTokens {
			break
		}
		x, err := strconv.ParseFloat(f, 64)
		if err != nil || math.IsNaN(x) || math.IsInf(x, 0) {
			return v
		}
		v = append(v, min(max(x, -maxCoord), maxCoord))
	}
	return v
}

// equation returns the value of the equation with the index i.
func (e *geometry) equation(i, depth int) float64 {
	switch e.state[i] {
	case 2:
		return e.values[i]
	case 1:
		// a cycle of equations
		return 0
	}
	e.state[i] = 1
	v := e.formulas[i].eval(e, depth+1)
	e.values[i], e.state[i] = clampValue(v), 2
	return e.values[i]
}

// clampValue limits a value to maxCoord, and returns 0 for NaN.
func clampValue(v float64) float64 {
	if math.IsNaN(v) {
		return 0
	}
	return min(max(v, -maxCoord), maxCoord)
}

// variable returns the value of a variable of a formula, such as "width".
func (e *geometry) variable(name string) float64 {
	switch name {
	case "pi":
		return math.Pi
	case "left":
		return e.vx
	case "top":
		return e.vy
	case "right":
		return e.vx + e.vw
	case "bottom":
		return e.vy + e.vh
	case "width":
		return e.vw
	case "height":
		return e.vh
	case "logwidth":
		// the width of the shape in 1/100 mm
		return e.w * 2540 / 72
	case "logheight":
		return e.h * 2540 / 72
	case "hasstroke", "hasfill":
		return 1
	}
	return 0
}

// param returns the value of a parameter of a path: a number, an equation
// such as "?f1", a modifier such as "$0", or a variable.
func (e *geometry) param(s string) float64 {
	if s == "" {
		return 0
	}
	switch s[0] {
	case '?':
		if i, ok := e.names[s[1:]]; ok {
			return e.equation(i, 0)
		}
		return 0
	case '$':
		i, err := strconv.Atoi(s[1:])
		if err == nil && i >= 0 && i < len(e.modifiers) {
			return e.modifiers[i]
		}
		return 0
	}
	if v, err := strconv.ParseFloat(s, 64); err == nil {
		return clampValue(v)
	}
	return e.variable(s)
}

// expr is a formula of draw:equation.
type expr interface {
	eval(e *geometry, depth int) float64
}

// constExpr is a number.
type constExpr float64

func (c constExpr) eval(*geometry, int) float64 { return float64(c) }

// refExpr is a variable, an equation or a modifier.
type refExpr string

func (r refExpr) eval(e *geometry, depth int) float64 {
	if depth > maxExprDepth {
		return 0
	}
	if len(r) > 0 && r[0] == '?' {
		if i, ok := e.names[string(r[1:])]; ok {
			return e.equation(i, depth)
		}
		return 0
	}
	return e.param(string(r))
}

// unaryExpr is a negation.
type unaryExpr struct{ x expr }

func (u unaryExpr) eval(e *geometry, depth int) float64 { return -u.x.eval(e, depth) }

// binaryExpr is an operation of two operands.
type binaryExpr struct {
	op   byte
	x, y expr
}

func (b binaryExpr) eval(e *geometry, depth int) float64 {
	x, y := b.x.eval(e, depth), b.y.eval(e, depth)
	switch b.op {
	case '+':
		return x + y
	case '-':
		return x - y
	case '*':
		return x * y
	case '/':
		if y == 0 {
			return 0
		}
		return x / y
	}
	return 0
}

// callExpr is a call of a function.
type callExpr struct {
	name string
	args []expr
}

func (c callExpr) eval(e *geometry, depth int) float64 {
	a := make([]float64, 3)
	for i, x := range c.args {
		if i < len(a) {
			a[i] = x.eval(e, depth)
		}
	}
	switch c.name {
	case "abs":
		return math.Abs(a[0])
	case "sqrt":
		return math.Sqrt(max(a[0], 0))
	case "sin":
		return math.Sin(a[0])
	case "cos":
		return math.Cos(a[0])
	case "tan":
		return math.Tan(a[0])
	case "atan":
		return math.Atan(a[0])
	case "atan2":
		return math.Atan2(a[0], a[1])
	case "min":
		return min(a[0], a[1])
	case "max":
		return max(a[0], a[1])
	case "if":
		if a[0] > 0 {
			return a[1]
		}
		return a[2]
	}
	return 0
}

// exprParser parses a formula.
type exprParser struct {
	s     string
	i     int
	depth int
}

// parseExpr parses the formula s.
func parseExpr(s string) (expr, error) {
	p := &exprParser{s: s}
	x, err := p.sum()
	if err != nil {
		return nil, err
	}
	p.space()
	if p.i != len(p.s) {
		return nil, errFormula
	}
	return x, nil
}

func (p *exprParser) space() {
	for p.i < len(p.s) && (p.s[p.i] == ' ' || p.s[p.i] == '\t') {
		p.i++
	}
}

func (p *exprParser) sum() (expr, error) {
	p.depth++
	defer func() { p.depth-- }()
	if p.depth > maxExprDepth {
		return nil, errFormula
	}
	x, err := p.product()
	for err == nil {
		p.space()
		if p.i >= len(p.s) || p.s[p.i] != '+' && p.s[p.i] != '-' {
			break
		}
		op := p.s[p.i]
		p.i++
		var y expr
		y, err = p.product()
		x = binaryExpr{op, x, y}
	}
	return x, err
}

func (p *exprParser) product() (expr, error) {
	x, err := p.unary()
	for err == nil {
		p.space()
		if p.i >= len(p.s) || p.s[p.i] != '*' && p.s[p.i] != '/' {
			break
		}
		op := p.s[p.i]
		p.i++
		var y expr
		y, err = p.unary()
		x = binaryExpr{op, x, y}
	}
	return x, err
}

func (p *exprParser) unary() (expr, error) {
	p.space()
	if p.i < len(p.s) && (p.s[p.i] == '-' || p.s[p.i] == '+') {
		neg := p.s[p.i] == '-'
		p.i++
		p.depth++
		defer func() { p.depth-- }()
		if p.depth > maxExprDepth {
			return nil, errFormula
		}
		x, err := p.unary()
		if err != nil || !neg {
			return x, err
		}
		return unaryExpr{x}, nil
	}
	return p.primary()
}

func (p *exprParser) primary() (expr, error) {
	p.space()
	if p.i >= len(p.s) {
		return nil, errFormula
	}
	c := p.s[p.i]
	switch {
	case c == '(':
		p.i++
		x, err := p.sum()
		if err != nil {
			return nil, err
		}
		p.space()
		if p.i >= len(p.s) || p.s[p.i] != ')' {
			return nil, errFormula
		}
		p.i++
		return x, nil
	case c >= '0' && c <= '9' || c == '.':
		start := p.i
		for p.i < len(p.s) && (p.s[p.i] >= '0' && p.s[p.i] <= '9' || p.s[p.i] == '.' || p.s[p.i] == 'e' || p.s[p.i] == 'E') {
			p.i++
		}
		v, err := strconv.ParseFloat(p.s[start:p.i], 64)
		if err != nil {
			return nil, errFormula
		}
		return constExpr(clampValue(v)), nil
	case c == '?' || c == '$':
		start := p.i
		p.i++
		for p.i < len(p.s) && isName(p.s[p.i]) {
			p.i++
		}
		return refExpr(p.s[start:p.i]), nil
	case isName(c):
		start := p.i
		for p.i < len(p.s) && isName(p.s[p.i]) {
			p.i++
		}
		name := p.s[start:p.i]
		p.space()
		if p.i < len(p.s) && p.s[p.i] == '(' {
			p.i++
			var args []expr
			for {
				x, err := p.sum()
				if err != nil {
					return nil, err
				}
				args = append(args, x)
				p.space()
				if p.i < len(p.s) && p.s[p.i] == ',' && len(args) < 3 {
					p.i++
					continue
				}
				break
			}
			if p.i >= len(p.s) || p.s[p.i] != ')' {
				return nil, errFormula
			}
			p.i++
			return callExpr{name: name, args: args}, nil
		}
		return refExpr(name), nil
	}
	return nil, errFormula
}

// isName reports whether c can be in a name of a formula.
func isName(c byte) bool {
	return c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '_'
}

// pathBuilder builds the paths of an enhanced path, in the box of the
// shape. Each part between two N commands is one path, with its own flags.
type pathBuilder struct {
	e *geometry
	// sx and sy scale the viewBox to the box.
	sx, sy float64
	pb     *okage.PathBuilder
	fill   bool
	stroke bool
	paths  []drawingml.Path
	// x and y are the current point, and x0 and y0 the start of the
	// subpath, in the units of the viewBox.
	x, y, x0, y0 float64
	open         bool
}

// pt returns the point (x, y) of the viewBox in the box.
func (b *pathBuilder) pt(x, y float64) (float32, float32) {
	return float32((x - b.e.vx) * b.sx), float32((y - b.e.vy) * b.sy)
}

func (b *pathBuilder) moveTo(x, y float64) {
	b.pb.MoveTo(b.pt(x, y))
	b.x, b.y, b.x0, b.y0, b.open = x, y, x, y, true
}

func (b *pathBuilder) lineTo(x, y float64) {
	if !b.open {
		b.moveTo(b.x, b.y)
	}
	b.pb.LineTo(b.pt(x, y))
	b.x, b.y = x, y
}

func (b *pathBuilder) cubicTo(x1, y1, x2, y2, x, y float64) {
	if !b.open {
		b.moveTo(b.x, b.y)
	}
	ax, ay := b.pt(x1, y1)
	bx, by := b.pt(x2, y2)
	cx, cy := b.pt(x, y)
	b.pb.CubicTo(ax, ay, bx, by, cx, cy)
	b.x, b.y = x, y
}

func (b *pathBuilder) quadTo(x1, y1, x, y float64) {
	if !b.open {
		b.moveTo(b.x, b.y)
	}
	ax, ay := b.pt(x1, y1)
	cx, cy := b.pt(x, y)
	b.pb.QuadTo(ax, ay, cx, cy)
	b.x, b.y = x, y
}

func (b *pathBuilder) close() {
	if b.open {
		b.pb.Close()
	}
	b.x, b.y, b.open = b.x0, b.y0, false
}

// end ends the path of the part, with its flags.
func (b *pathBuilder) end() {
	if p, ok := b.pb.Finish(); ok {
		f := drawingml.FillNorm
		if !b.fill {
			f = drawingml.FillNone
		}
		b.paths = append(b.paths, drawingml.Path{Path: p, Fill: f, Stroke: b.stroke})
	}
	b.pb = okage.NewPathBuilder()
	b.fill, b.stroke, b.open = true, true, false
}

// arc adds the arc of the ellipse of the center (cx, cy) and the radii rx
// and ry, from the parameter t0 over the sweep, in radians. The parameter
// t is the point (cx + rx cos t, cy + ry sin t), so a positive sweep turns
// clockwise on the page. move starts a new subpath at the start of the arc.
func (b *pathBuilder) arc(cx, cy, rx, ry, t0, sweep float64, move bool) {
	sx, sy := cx+rx*math.Cos(t0), cy+ry*math.Sin(t0)
	if move || !b.open {
		b.moveTo(sx, sy)
	} else {
		b.lineTo(sx, sy)
	}
	sweep = min(max(sweep, -2*math.Pi), 2*math.Pi)
	n := int(math.Ceil(math.Abs(sweep)/(math.Pi/2) - 1e-9))
	if n <= 0 || rx == 0 || ry == 0 {
		return
	}
	step := sweep / float64(n)
	k := 4.0 / 3 * math.Tan(step/4)
	for i := range n {
		ta := t0 + float64(i)*step
		sa, ca := math.Sincos(ta)
		sb, cb := math.Sincos(ta + step)
		b.cubicTo(
			cx+rx*(ca-k*sa), cy+ry*(sa+k*ca),
			cx+rx*(cb+k*sb), cy+ry*(sb-k*cb),
			cx+rx*cb, cy+ry*sb,
		)
	}
}

// pathTokens splits an enhanced path into commands and parameters. A
// command is an upper case letter, which can touch its first parameter.
func pathTokens(s string) []string {
	var tokens []string
	for f := range strings.FieldsFuncSeq(s, func(r rune) bool { return r == ' ' || r == ',' || r == '\t' || r == '\n' || r == '\r' }) {
		if len(tokens) >= maxPathTokens {
			break
		}
		if f[0] >= 'A' && f[0] <= 'Z' && len(f) > 1 {
			tokens = append(tokens, f[:1], f[1:])
			continue
		}
		tokens = append(tokens, f)
	}
	return tokens
}

// paths evaluates the enhanced path of the geometry, and returns its paths
// in the box of the shape. It returns false for a geometry without a path.
func (e *geometry) paths() ([]drawingml.Path, bool) {
	tokens := pathTokens(e.path)
	if len(tokens) == 0 {
		return nil, false
	}
	b := &pathBuilder{e: e, sx: e.w / e.vw, sy: e.h / e.vh, pb: okage.NewPathBuilder(), fill: true, stroke: true}
	cmd := byte('M')
	i := 0
	// args returns the next n parameters, or false at a command or the
	// end.
	args := func(n int) ([]float64, bool) {
		if i+n > len(tokens) {
			return nil, false
		}
		v := make([]float64, n)
		for k := range n {
			t := tokens[i+k]
			if len(t) == 1 && t[0] >= 'A' && t[0] <= 'Z' {
				return nil, false
			}
			v[k] = e.param(t)
		}
		i += n
		return v, true
	}
	quadrant := false
	for i < len(tokens) {
		t := tokens[i]
		if len(t) == 1 && t[0] >= 'A' && t[0] <= 'Z' {
			cmd = t[0]
			i++
			quadrant = cmd == 'Y'
			switch cmd {
			case 'Z':
				b.close()
				continue
			case 'N':
				b.end()
				continue
			case 'F':
				b.fill = false
				continue
			case 'S':
				b.stroke = false
				continue
			}
		}
		before := i
		switch cmd {
		case 'M':
			if v, ok := args(2); ok {
				b.moveTo(v[0], v[1])
				cmd = 'L'
			}
		case 'L':
			if v, ok := args(2); ok {
				b.lineTo(v[0], v[1])
			}
		case 'C':
			if v, ok := args(6); ok {
				b.cubicTo(v[0], v[1], v[2], v[3], v[4], v[5])
			}
		case 'Q':
			if v, ok := args(4); ok {
				b.quadTo(v[0], v[1], v[2], v[3])
			}
		case 'T', 'U':
			if v, ok := args(6); ok {
				// the angles are in degrees, counterclockwise from the x
				// axis
				t0 := -v[4] * math.Pi / 180
				t1 := -v[5] * math.Pi / 180
				sweep := t1 - t0
				for sweep >= 0 {
					sweep -= 2 * math.Pi
				}
				if math.Mod(math.Abs(v[4]-v[5]), 360) == 0 {
					sweep = -2 * math.Pi
				}
				rx, ry := math.Abs(v[2]), math.Abs(v[3])
				b.arc(v[0], v[1], rx, ry, t0, sweep, cmd == 'U')
			}
		case 'A', 'B', 'W', 'V':
			if v, ok := args(8); ok {
				l, r := min(v[0], v[2]), max(v[0], v[2])
				tp, bt := min(v[1], v[3]), max(v[1], v[3])
				cx, cy := (l+r)/2, (tp+bt)/2
				rx, ry := (r-l)/2, (bt-tp)/2
				if rx == 0 || ry == 0 {
					b.lineTo(v[6], v[7])
					break
				}
				t0 := math.Atan2((v[5]-cy)/ry, (v[4]-cx)/rx)
				t1 := math.Atan2((v[7]-cy)/ry, (v[6]-cx)/rx)
				sweep := t1 - t0
				if cmd == 'A' || cmd == 'B' {
					// counterclockwise on the page
					for sweep >= 0 {
						sweep -= 2 * math.Pi
					}
				} else {
					for sweep <= 0 {
						sweep += 2 * math.Pi
					}
				}
				b.arc(cx, cy, rx, ry, t0, sweep, cmd == 'B' || cmd == 'V')
			}
		case 'X', 'Y':
			if v, ok := args(2); ok {
				const k = 0.5522847498
				x0, y0 := b.x, b.y
				if !quadrant {
					// the tangent at the start is horizontal
					b.cubicTo(x0+(v[0]-x0)*k, y0, v[0], v[1]+(y0-v[1])*k, v[0], v[1])
				} else {
					b.cubicTo(x0, y0+(v[1]-y0)*k, v[0]+(x0-v[0])*k, v[1], v[0], v[1])
				}
				quadrant = !quadrant
			}
		case 'G':
			if v, ok := args(4); ok {
				b.arcAngle(v[0], v[1], v[2], v[3])
			}
		}
		if i == before {
			// a parameter that the command does not take
			i++
		}
	}
	b.end()
	return b.paths, len(b.paths) > 0
}

// arcAngle adds the arc of arcangleto, which is a:arcTo of DrawingML: the
// radii, the start angle and the sweep, in degrees, clockwise.
func (b *pathBuilder) arcAngle(wr, hr, st, sw float64) {
	wr, hr = math.Abs(wr), math.Abs(hr)
	sw = min(max(sw, -360), 360)
	param := func(a float64) float64 {
		a = a * math.Pi / 180
		turns := math.Round(a / (2 * math.Pi))
		a -= turns * 2 * math.Pi
		return turns*2*math.Pi + math.Atan2(wr*math.Sin(a), hr*math.Cos(a))
	}
	t0, t1 := param(st), param(st+sw)
	cx, cy := b.x-wr*math.Cos(t0), b.y-hr*math.Sin(t0)
	b.arc(cx, cy, wr, hr, t0, t1-t0, false)
}

// textRect returns the first text area of the geometry in the box of the
// shape, or false if it has none.
func (e *geometry) textRect() (okage.Rect, bool) {
	tokens := pathTokens(e.textAreas)
	if len(tokens) < 4 {
		return okage.Rect{}, false
	}
	l, t := e.param(tokens[0]), e.param(tokens[1])
	r, bt := e.param(tokens[2]), e.param(tokens[3])
	sx, sy := e.w/e.vw, e.h/e.vh
	return okage.RectFromLTRB(float32((min(l, r)-e.vx)*sx), float32((min(t, bt)-e.vy)*sy),
		float32((max(l, r)-e.vx)*sx), float32((max(t, bt)-e.vy)*sy))
}

// presetTypes maps the types of the shapes of LibreOffice to the preset
// shapes of DrawingML that look the same, for a geometry without an enhanced
// path.
var presetTypes = map[string]string{
	"rectangle": "rect", "round-rectangle": "roundRect", "ellipse": "ellipse", "circle": "ellipse",
	"isosceles-triangle": "triangle", "right-triangle": "rtTriangle", "diamond": "diamond",
	"parallelogram": "parallelogram", "trapezoid": "trapezoid", "pentagon": "pentagon",
	"hexagon": "hexagon", "octagon": "octagon", "cross": "plus", "ring": "donut", "can": "can",
	"cube": "cube", "heart": "heart", "sun": "sun", "moon": "moon", "smiley": "smileyFace",
	"lightning": "lightningBolt", "star4": "star4", "star5": "star5", "star6": "star6",
	"star8": "star8", "star12": "star12", "star24": "star24", "right-arrow": "rightArrow",
	"left-arrow": "leftArrow", "up-arrow": "upArrow", "down-arrow": "downArrow",
	"left-right-arrow": "leftRightArrow", "up-down-arrow": "upDownArrow", "cloud": "cloud",
	"flowchart-process": "flowChartProcess", "flowchart-decision": "flowChartDecision",
	"flowchart-terminator": "flowChartTerminator", "flowchart-data": "flowChartInputOutput",
	"flowchart-connector": "flowChartConnector", "block-arc": "blockArc", "frame": "frame",
	"quad-bevel": "bevel", "octagon-bevel": "bevel", "paper": "foldedCorner", "col-60da8460": "homePlate",
	"chevron": "chevron", "notched-right-arrow": "notchedRightArrow", "mso-spt100": "rect",
}

// presetGeometry returns the preset shape of DrawingML for the type of an
// enhanced geometry: "ooxml-" and the name of the preset, or a type of
// LibreOffice that presetTypes holds. The modifiers become the adjust
// values of the preset, in order.
func presetGeometry(typ string, modifiers []float64) (*drawingml.Geometry, []drawingml.Guide, bool) {
	name, ooxml := strings.CutPrefix(typ, "ooxml-")
	if !ooxml {
		name = presetTypes[typ]
	}
	g, ok := drawingml.Preset(name)
	if !ok {
		return nil, nil, false
	}
	var adjust []drawingml.Guide
	if ooxml {
		for i, n := range g.AdjustNames() {
			if i < len(modifiers) {
				adjust = append(adjust, drawingml.AdjustValue(n, modifiers[i]))
			}
		}
	}
	return g, adjust, true
}

// svgPath returns the path of the path data d of SVG, which okage parses.
func svgPath(d string) (*okage.Path, bool) {
	if d == "" || len(d) > 1<<20 {
		return nil, false
	}
	var b strings.Builder
	b.WriteString(`<svg xmlns="http://www.w3.org/2000/svg"><path d="`)
	b.WriteString(htmlAttr(d))
	b.WriteString(`"/></svg>`)
	tree, err := svg.FromStr(b.String())
	if err != nil {
		return nil, false
	}
	return findPath(tree.Root(), 0)
}

// findPath returns the data of the first path of the group g.
func findPath(g *svg.Group, depth int) (*okage.Path, bool) {
	if depth > 8 {
		return nil, false
	}
	for _, n := range g.Children() {
		switch v := n.(type) {
		case *svg.Path:
			if p := v.Data(); p != nil {
				return p.Clone(), true
			}
		case *svg.Group:
			if p, ok := findPath(v, depth+1); ok {
				return p, true
			}
		}
	}
	return nil, false
}

// htmlAttr escapes s for an attribute of XML.
func htmlAttr(s string) string {
	r := strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;", `"`, "&quot;")
	return r.Replace(s)
}

// pointsPath returns the path of draw:points, a list of "x,y" pairs, closed
// for a polygon.
func pointsPath(points string, closed bool) (*okage.Path, bool) {
	v := numbers(points)
	if len(v) < 4 {
		return nil, false
	}
	pb := okage.NewPathBuilder()
	pb.MoveTo(float32(v[0]), float32(v[1]))
	for k := 2; k+1 < len(v); k += 2 {
		pb.LineTo(float32(v[k]), float32(v[k+1]))
	}
	if closed {
		pb.Close()
	}
	return pb.Finish()
}
