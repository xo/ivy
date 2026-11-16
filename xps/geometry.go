package xps

import (
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/xo/ivy"
	"github.com/xo/okage"
)

// geometry is a path and its fill rule.
type geometry struct {
	path *okage.Path
	rule okage.FillRule
}

// parseMatrix parses a matrix of XPS, "m11,m12,m21,m22,offsetX,offsetY".
func parseMatrix(s string) (okage.Transform, error) {
	v, err := numbers(s)
	if err != nil || len(v) != 6 {
		return okage.Transform{}, fmt.Errorf("matrix %q: %w", s, ivy.ErrFormat)
	}
	return okage.TransformFromRow(v[0], v[1], v[2], v[3], v[4], v[5]), nil
}

// numbers parses numbers that commas or white space separate.
func numbers(s string) ([]float32, error) {
	fields := strings.FieldsFunc(s, func(r rune) bool {
		return r == ',' || r == ' ' || r == '\t' || r == '\n' || r == '\r'
	})
	out := make([]float32, len(fields))
	for i, f := range fields {
		v, err := strconv.ParseFloat(f, 32)
		if err != nil {
			return nil, fmt.Errorf("number %q: %w", f, ivy.ErrFormat)
		}
		out[i] = float32(v)
	}
	return out, nil
}

// parseRect parses a rectangle of XPS, "x,y,width,height".
func parseRect(s string) (okage.Rect, bool) {
	v, err := numbers(s)
	if err != nil || len(v) != 4 {
		return okage.Rect{}, false
	}
	return okage.RectFromXYWH(v[0], v[1], v[2], v[3])
}

// pathBuilder builds a path, with the current point and the control points
// that the smooth curves of the syntax reflect.
type pathBuilder struct {
	pb           *okage.PathBuilder
	x, y         float32
	startX       float32
	startY       float32
	lastCubic    [2]float32
	lastQuad     [2]float32
	hasLastCubic bool
	hasLastQuad  bool
}

func newPathBuilder() *pathBuilder {
	return &pathBuilder{pb: okage.NewPathBuilder()}
}

func (b *pathBuilder) moveTo(x, y float32) {
	b.pb.MoveTo(x, y)
	b.x, b.y, b.startX, b.startY = x, y, x, y
	b.hasLastCubic, b.hasLastQuad = false, false
}

func (b *pathBuilder) lineTo(x, y float32) {
	b.pb.LineTo(x, y)
	b.x, b.y = x, y
	b.hasLastCubic, b.hasLastQuad = false, false
}

func (b *pathBuilder) cubicTo(x1, y1, x2, y2, x, y float32) {
	b.pb.CubicTo(x1, y1, x2, y2, x, y)
	b.x, b.y = x, y
	b.lastCubic, b.hasLastCubic, b.hasLastQuad = [2]float32{x2, y2}, true, false
}

func (b *pathBuilder) quadTo(x1, y1, x, y float32) {
	b.pb.QuadTo(x1, y1, x, y)
	b.x, b.y = x, y
	b.lastQuad, b.hasLastQuad, b.hasLastCubic = [2]float32{x1, y1}, true, false
}

func (b *pathBuilder) close() {
	b.pb.Close()
	b.x, b.y = b.startX, b.startY
	b.hasLastCubic, b.hasLastQuad = false, false
}

// arcTo draws an elliptical arc from the current point to (x, y), as SVG and
// XPS define it, with cubic curves.
func (b *pathBuilder) arcTo(rx, ry, rotation float32, large, sweep bool, x, y float32) {
	x0, y0 := float64(b.x), float64(b.y)
	x1, y1 := float64(x), float64(y)
	if x0 == x1 && y0 == y1 {
		return
	}
	frx, fry := math.Abs(float64(rx)), math.Abs(float64(ry))
	if frx == 0 || fry == 0 {
		b.lineTo(x, y)
		return
	}
	phi := float64(rotation) * math.Pi / 180
	sin, cos := math.Sincos(phi)
	// the endpoint parameterization to the center parameterization, as in
	// appendix F.6 of SVG 1.1
	dx, dy := (x0-x1)/2, (y0-y1)/2
	x1p := cos*dx + sin*dy
	y1p := -sin*dx + cos*dy
	if l := x1p*x1p/(frx*frx) + y1p*y1p/(fry*fry); l > 1 {
		s := math.Sqrt(l)
		frx, fry = frx*s, fry*s
	}
	num := frx*frx*fry*fry - frx*frx*y1p*y1p - fry*fry*x1p*x1p
	den := frx*frx*y1p*y1p + fry*fry*x1p*x1p
	coef := 0.0
	if den != 0 && num > 0 {
		coef = math.Sqrt(num / den)
	}
	if large == sweep {
		coef = -coef
	}
	cxp, cyp := coef*frx*y1p/fry, -coef*fry*x1p/frx
	cx := cos*cxp - sin*cyp + (x0+x1)/2
	cy := sin*cxp + cos*cyp + (y0+y1)/2
	angle := func(ux, uy, vx, vy float64) float64 {
		return math.Atan2(ux*vy-uy*vx, ux*vx+uy*vy)
	}
	theta := angle(1, 0, (x1p-cxp)/frx, (y1p-cyp)/fry)
	delta := angle((x1p-cxp)/frx, (y1p-cyp)/fry, (-x1p-cxp)/frx, (-y1p-cyp)/fry)
	switch {
	case !sweep && delta > 0:
		delta -= 2 * math.Pi
	case sweep && delta < 0:
		delta += 2 * math.Pi
	}
	segments := int(math.Ceil(math.Abs(delta) / (math.Pi / 2)))
	step := delta / float64(segments)
	k := 4.0 / 3.0 * math.Tan(step/4)
	point := func(t float64) (float64, float64) {
		s, c := math.Sincos(t)
		return cx + frx*c*cos - fry*s*sin, cy + frx*c*sin + fry*s*cos
	}
	deriv := func(t float64) (float64, float64) {
		s, c := math.Sincos(t)
		return -frx*s*cos - fry*c*sin, -frx*s*sin + fry*c*cos
	}
	for i := range segments {
		t0, t1 := theta+float64(i)*step, theta+float64(i+1)*step
		px0, py0 := point(t0)
		px1, py1 := point(t1)
		d0x, d0y := deriv(t0)
		d1x, d1y := deriv(t1)
		b.pb.CubicTo(
			float32(px0+k*d0x), float32(py0+k*d0y),
			float32(px1-k*d1x), float32(py1-k*d1y),
			float32(px1), float32(py1),
		)
	}
	b.x, b.y = x, y
	b.hasLastCubic, b.hasLastQuad = false, false
}

// finish returns the path, or nil for an empty path.
func (b *pathBuilder) finish() *okage.Path {
	p, ok := b.pb.Finish()
	if !ok {
		return nil
	}
	return p
}

// parseAbbreviated parses the abbreviated syntax of a geometry, such as
// "F1 M 0,0 L 10,0 10,10 Z". The fill rule is even-odd unless the data starts
// with F1.
func parseAbbreviated(s string) (geometry, error) {
	g := geometry{rule: okage.FillRuleEvenOdd}
	sc := &scanner{s: s}
	b := newPathBuilder()
	var cmd byte
	for {
		sc.skipSpace()
		if sc.done() {
			break
		}
		if c := sc.peek(); isCommand(c) {
			cmd = c
			sc.i++
		} else if cmd == 0 {
			return g, fmt.Errorf("geometry %q: %w", s, ivy.ErrFormat)
		}
		rel := cmd >= 'a' && cmd <= 'z'
		var ox, oy float32
		if rel {
			ox, oy = b.x, b.y
		}
		var err error
		switch cmd {
		case 'F':
			var v float32
			if v, err = sc.number(); err == nil && v == 1 {
				g.rule = okage.FillRuleWinding
			}
			cmd = 0
		case 'M', 'm':
			var p [2]float32
			if p, err = sc.point(); err == nil {
				b.moveTo(ox+p[0], oy+p[1])
				// the points after the first of a move are lines
				cmd = map[bool]byte{true: 'l', false: 'L'}[rel]
			}
		case 'L', 'l':
			var p [2]float32
			if p, err = sc.point(); err == nil {
				b.lineTo(ox+p[0], oy+p[1])
			}
		case 'H', 'h':
			var v float32
			if v, err = sc.number(); err == nil {
				b.lineTo(ox+v, b.y)
			}
		case 'V', 'v':
			var v float32
			if v, err = sc.number(); err == nil {
				b.lineTo(b.x, oy+v)
			}
		case 'C', 'c':
			var v [6]float32
			if v, err = sc.numbers6(); err == nil {
				b.cubicTo(ox+v[0], oy+v[1], ox+v[2], oy+v[3], ox+v[4], oy+v[5])
			}
		case 'S', 's':
			var v [4]float32
			if v, err = sc.numbers4(); err == nil {
				x1, y1 := b.x, b.y
				if b.hasLastCubic {
					x1, y1 = 2*b.x-b.lastCubic[0], 2*b.y-b.lastCubic[1]
				}
				b.cubicTo(x1, y1, ox+v[0], oy+v[1], ox+v[2], oy+v[3])
			}
		case 'Q', 'q':
			var v [4]float32
			if v, err = sc.numbers4(); err == nil {
				b.quadTo(ox+v[0], oy+v[1], ox+v[2], oy+v[3])
			}
		case 'A', 'a':
			var v [7]float32
			for i := range v {
				if v[i], err = sc.number(); err != nil {
					break
				}
			}
			if err == nil {
				b.arcTo(v[0], v[1], v[2], v[3] != 0, v[4] != 0, ox+v[5], oy+v[6])
			}
		case 'Z', 'z':
			b.close()
			cmd = 0
		default:
			return g, fmt.Errorf("geometry %q: command %q: %w", s, cmd, ivy.ErrFormat)
		}
		if err != nil {
			return g, fmt.Errorf("geometry %q: %w", s, err)
		}
	}
	g.path = b.finish()
	return g, nil
}

// isCommand reports whether c is a command of the abbreviated syntax.
func isCommand(c byte) bool {
	return strings.IndexByte("FMmLlHhVvCcSsQqAaZz", c) >= 0
}

// scanner reads the numbers of the abbreviated syntax.
type scanner struct {
	s string
	i int
}

func (sc *scanner) done() bool {
	return sc.i >= len(sc.s)
}

func (sc *scanner) peek() byte {
	return sc.s[sc.i]
}

func (sc *scanner) skipSpace() {
	for !sc.done() && strings.IndexByte(" \t\r\n,", sc.s[sc.i]) >= 0 {
		sc.i++
	}
}

// number reads one number.
func (sc *scanner) number() (float32, error) {
	sc.skipSpace()
	start := sc.i
	if !sc.done() && (sc.peek() == '-' || sc.peek() == '+') {
		sc.i++
	}
	seenExp := false
loop:
	for !sc.done() {
		c := sc.peek()
		switch {
		case c >= '0' && c <= '9', c == '.':
		case (c == 'e' || c == 'E') && !seenExp:
			seenExp = true
			if sc.i+1 < len(sc.s) && (sc.s[sc.i+1] == '-' || sc.s[sc.i+1] == '+') {
				sc.i++
			}
		default:
			break loop
		}
		sc.i++
	}
	v, err := strconv.ParseFloat(sc.s[start:sc.i], 32)
	if err != nil {
		return 0, fmt.Errorf("number at %d: %w", start, ivy.ErrFormat)
	}
	return float32(v), nil
}

func (sc *scanner) point() ([2]float32, error) {
	var p [2]float32
	var err error
	for i := range p {
		if p[i], err = sc.number(); err != nil {
			return p, err
		}
	}
	return p, nil
}

func (sc *scanner) numbers4() ([4]float32, error) {
	var v [4]float32
	var err error
	for i := range v {
		if v[i], err = sc.number(); err != nil {
			return v, err
		}
	}
	return v, nil
}

func (sc *scanner) numbers6() ([6]float32, error) {
	var v [6]float32
	var err error
	for i := range v {
		if v[i], err = sc.number(); err != nil {
			return v, err
		}
	}
	return v, nil
}

// points parses the points of a segment, such as "1,2 3,4".
func points(s string) ([][2]float32, error) {
	v, err := numbers(s)
	if err != nil || len(v)%2 != 0 {
		return nil, fmt.Errorf("points %q: %w", s, ivy.ErrFormat)
	}
	out := make([][2]float32, len(v)/2)
	for i := range out {
		out[i] = [2]float32{v[2*i], v[2*i+1]}
	}
	return out, nil
}
