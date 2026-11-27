package drawingml

import (
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/xo/ivy"
	"github.com/xo/okage"
)

// The limits of the evaluation.
const (
	// maxSize is the largest width or height of a shape. It is the largest
	// value of ST_PositiveCoordinate, in EMU.
	maxSize = 27273042316900
	// maxValue is the largest magnitude of a guide. A larger result becomes
	// maxValue, so that a product of two guides is finite. It is larger
	// than the product of the squares of two lengths of the largest shape,
	// which the circular arrow computes.
	maxValue = 1e60
	// boxes is how far a point can be from the box of the shape, in units of
	// the larger side of the box. A point farther away moves to that limit.
	// A callout can point outside its box.
	boxes = 64
)

// angle is the number of units of an angle of DrawingML in a radian. An angle
// is in 60000ths of a degree.
const angle = 60000 * 180 / math.Pi

// Shape is a geometry that Evaluate computed for the size of a shape.
type Shape struct {
	// Paths holds the paths that are not empty, in the order of the
	// geometry.
	Paths []Path
	// Text is the text rectangle.
	Text okage.Rect
}

// Path is a path of a shape, from one a:path. It can hold more than one
// subpath.
type Path struct {
	Path *okage.Path
	Fill Fill
	// Stroke reports whether the outline of the shape draws the path.
	Stroke bool
	// ExtrusionOK reports whether a 3D extrusion of the shape uses the path.
	ExtrusionOK bool
}

// Evaluate computes the guides of the geometry for a shape box of the width
// w and the height h, and returns the paths and the text rectangle. w and h
// can be in any unit, such as EMU or points. The paths and the text
// rectangle use the same unit, with the origin at the top left corner of the
// box. The adjust values replace those of the geometry with the same names,
// as ParseAdjust returns them for the a:avLst of a:prstGeom.
//
// The guides are in the order of the geometry, and a guide can use the
// guides before it. A name that no guide defines and that is not a built-in
// variable has the value 0. A division by zero gives 0.
func (g *Geometry) Evaluate(w, h float64, adjust []Guide) (*Shape, error) {
	if !(w >= 0 && w <= maxSize && h >= 0 && h <= maxSize) {
		return nil, fmt.Errorf("drawingml: shape size %gx%g: %w", w, h, ivy.ErrFormat)
	}
	e := &evaluator{
		w:    w,
		h:    h,
		vars: make(map[string]float64, len(g.adjust)+len(adjust)+len(g.guides)),
	}
	for _, a := range g.adjust {
		for _, b := range adjust {
			if b.name == a.name {
				a = b
			}
		}
		e.vars[a.name] = e.eval(a)
	}
	for _, b := range adjust {
		if _, ok := e.vars[b.name]; !ok {
			e.vars[b.name] = e.eval(b)
		}
	}
	for _, gd := range g.guides {
		e.vars[gd.name] = e.eval(gd)
	}
	limit := boxes * max(w, h, 1)
	s := &Shape{Paths: make([]Path, 0, len(g.paths))}
	for _, p := range g.paths {
		path, ok := e.path(p, limit)
		if !ok {
			continue
		}
		s.Paths = append(s.Paths, Path{
			Path:        path,
			Fill:        p.fill,
			Stroke:      p.stroke,
			ExtrusionOK: p.extrusion,
		})
	}
	s.Text = e.text(g.text, limit)
	return s, nil
}

// evaluator holds the values of the guides of a shape.
type evaluator struct {
	w, h float64
	vars map[string]float64
}

// value returns the value of the argument s: a number, a guide or a built-in
// variable.
func (e *evaluator) value(s string) float64 {
	if s == "" {
		return 0
	}
	// A number starts with a digit, a sign or a point. So do the built-in
	// variables such as 3cd4, which are not numbers.
	if c := s[0]; c == '-' || c == '+' || c == '.' || '0' <= c && c <= '9' {
		v, err := strconv.ParseFloat(s, 64)
		switch {
		case err == nil && math.IsInf(v, 0):
			// "Inf" and "-Inf"
			return 0
		case err == nil || errors.Is(err, strconv.ErrRange):
			return limitValue(v)
		}
	}
	if v, ok := e.vars[s]; ok {
		return v
	}
	v, _ := builtin(s, e.w, e.h)
	return v
}

// eval returns the value of the formula of the guide g.
func (e *evaluator) eval(g Guide) float64 {
	x := e.value(g.args[0])
	y := e.value(g.args[1])
	z := e.value(g.args[2])
	var v float64
	switch g.op {
	case opVal:
		v = x
	case opMulDiv:
		v = div(x*y, z)
	case opAddSub:
		v = x + y - z
	case opAddDiv:
		v = div(x+y, z)
	case opIfElse:
		v = z
		if x > 0 {
			v = y
		}
	case opAbs:
		v = math.Abs(x)
	case opAt2:
		v = math.Atan2(y, x) * angle
	case opCat2:
		v = x * math.Cos(math.Atan2(z, y))
	case opCos:
		v = x * math.Cos(y/angle)
	case opMax:
		v = max(x, y)
	case opMin:
		v = min(x, y)
	case opMod:
		v = math.Sqrt(x*x + y*y + z*z)
	case opPin:
		switch {
		case y < x:
			v = x
		case y > z:
			v = z
		default:
			v = y
		}
	case opSat2:
		v = x * math.Sin(math.Atan2(z, y))
	case opSin:
		v = x * math.Sin(y/angle)
	case opSqrt:
		v = math.Sqrt(max(x, 0))
	case opTan:
		v = x * math.Tan(y/angle)
	}
	return limitValue(v)
}

// builtin returns the value of the built-in variable with the name for a box
// of the width w and the height h, and false for an unknown name. The
// variables are those of ECMA-376 Part 1, 20.1.9.11, and a few more
// divisions of the sides.
func builtin(name string, w, h float64) (float64, bool) {
	ss, ls := min(w, h), max(w, h)
	switch name {
	case "l", "t":
		return 0, true
	case "r", "w":
		return w, true
	case "b", "h":
		return h, true
	case "hc":
		return w / 2, true
	case "vc":
		return h / 2, true
	case "ss":
		return ss, true
	case "ls":
		return ls, true
	case "cd2":
		return 10800000, true
	case "cd4":
		return 5400000, true
	case "cd8":
		return 2700000, true
	case "3cd4":
		return 16200000, true
	case "3cd8":
		return 8100000, true
	case "5cd8":
		return 13500000, true
	case "7cd8":
		return 18900000, true
	}
	var v float64
	var s string
	switch {
	case strings.HasPrefix(name, "ssd"):
		v, s = ss, name[3:]
	case strings.HasPrefix(name, "wd"):
		v, s = w, name[2:]
	case strings.HasPrefix(name, "hd"):
		v, s = h, name[2:]
	default:
		return 0, false
	}
	switch s {
	case "2", "3", "4", "5", "6", "8", "10", "12", "16", "32":
		d, _ := strconv.Atoi(s)
		return v / float64(d), true
	}
	return 0, false
}

// div returns x / y, or 0 if y is 0.
func div(x, y float64) float64 {
	if y == 0 {
		return 0
	}
	return x / y
}

// limitValue returns v limited to maxValue, and 0 for NaN.
func limitValue(v float64) float64 {
	if math.IsNaN(v) {
		return 0
	}
	return min(max(v, -maxValue), maxValue)
}

// number returns the number s, and false if s is not a finite number.
func number(s string) (float64, bool) {
	v, err := strconv.ParseFloat(s, 64)
	if err != nil || math.IsNaN(v) || math.IsInf(v, 0) {
		return 0, false
	}
	return v, true
}

// path returns the okage path of the path p of the geometry. The points are
// at most limit away from the origin. It returns false for an empty path.
func (e *evaluator) path(p pathDef, limit float64) (*okage.Path, bool) {
	b := builder{
		pb:    okage.NewPathBuilderWithCapacity(len(p.commands)+1, 3*len(p.commands)+1),
		sx:    1,
		sy:    1,
		limit: limit,
	}
	if p.w > 0 {
		b.sx = e.w / p.w
	}
	if p.h > 0 {
		b.sy = e.h / p.h
	}
	for _, c := range p.commands {
		a := c.args
		switch c.verb {
		case verbMoveTo:
			b.moveTo(e.value(a[0]), e.value(a[1]))
		case verbLineTo:
			b.lineTo(e.value(a[0]), e.value(a[1]))
		case verbQuadTo:
			b.start()
			b.quadTo(e.value(a[0]), e.value(a[1]), e.value(a[2]), e.value(a[3]))
		case verbCubicTo:
			b.start()
			b.cubicTo(e.value(a[0]), e.value(a[1]), e.value(a[2]), e.value(a[3]), e.value(a[4]), e.value(a[5]))
		case verbArcTo:
			b.arcTo(e.value(a[0]), e.value(a[1]), e.value(a[2]), e.value(a[3]))
		case verbClose:
			b.close()
		}
	}
	return b.pb.Finish()
}

// text returns the text rectangle t, or the box of the shape if the geometry
// has none.
func (e *evaluator) text(t textRect, limit float64) okage.Rect {
	box, _ := okage.RectFromLTRB(0, 0, float32(e.w), float32(e.h))
	if t.l == "" {
		return box
	}
	l, r := e.value(t.l), e.value(t.r)
	top, bottom := e.value(t.t), e.value(t.b)
	l, r = clamp(min(l, r), limit), clamp(max(l, r), limit)
	top, bottom = clamp(min(top, bottom), limit), clamp(max(top, bottom), limit)
	rect, ok := okage.RectFromLTRB(float32(l), float32(top), float32(r), float32(bottom))
	if !ok {
		return box
	}
	return rect
}

// clamp returns v limited to the range from -limit to limit, and 0 for NaN.
func clamp(v, limit float64) float64 {
	if math.IsNaN(v) {
		return 0
	}
	return min(max(v, -limit), limit)
}

// builder builds the okage path of a path of a geometry. It keeps the
// current point in the coordinate space of the path, and it scales each
// point to the shape when it adds it.
type builder struct {
	pb *okage.PathBuilder
	// sx and sy scale the coordinate space of the path to the shape.
	sx, sy float64
	limit  float64
	// x and y are the current point, and x0 and y0 the start of the
	// subpath.
	x, y   float64
	x0, y0 float64
	// open reports whether the path has a current point.
	open bool
}

// pt returns the point (x, y) of the path in the shape.
func (b *builder) pt(x, y float64) (float32, float32) {
	return float32(clamp(x*b.sx, b.limit)), float32(clamp(y*b.sy, b.limit))
}

// start adds a move to the current point if the path has no current point.
// The current point is then the origin.
func (b *builder) start() {
	if !b.open {
		b.moveTo(b.x, b.y)
	}
}

func (b *builder) moveTo(x, y float64) {
	b.pb.MoveTo(b.pt(x, y))
	b.x, b.y, b.x0, b.y0 = x, y, x, y
	b.open = true
}

func (b *builder) lineTo(x, y float64) {
	b.start()
	px, py := b.pt(x, y)
	b.pb.LineTo(px, py)
	b.x, b.y = x, y
}

func (b *builder) quadTo(x1, y1, x, y float64) {
	px1, py1 := b.pt(x1, y1)
	px, py := b.pt(x, y)
	b.pb.QuadTo(px1, py1, px, py)
	b.x, b.y = x, y
}

func (b *builder) cubicTo(x1, y1, x2, y2, x, y float64) {
	px1, py1 := b.pt(x1, y1)
	px2, py2 := b.pt(x2, y2)
	px, py := b.pt(x, y)
	b.pb.CubicTo(px1, py1, px2, py2, px, py)
	b.x, b.y = x, y
}

func (b *builder) close() {
	if b.open {
		b.pb.Close()
	}
	b.x, b.y = b.x0, b.y0
}

// arcTo adds an elliptical arc from the current point, as ECMA-376 Part 1,
// 20.1.9.3 defines it. The arc is on the ellipse with the radii wr and hr. It
// starts at the angle st and sweeps the angle sw, clockwise for a positive
// sw. The current point is the point of the ellipse at the angle st, which
// fixes the center of the ellipse.
//
// The angles are the directions from the center of the ellipse, and not the
// parameters of the points, as in Office. A sweep of more than a full turn
// is a full turn.
func (b *builder) arcTo(wr, hr, st, sw float64) {
	b.start()
	wr, hr = math.Abs(wr), math.Abs(hr)
	sw = min(max(sw, -21600000), 21600000)
	t0 := param(st/angle, wr, hr)
	t1 := param((st+sw)/angle, wr, hr)
	cx := b.x - wr*math.Cos(t0)
	cy := b.y - hr*math.Sin(t0)
	if wr == 0 || hr == 0 {
		b.lineTo(cx+wr*math.Cos(t1), cy+hr*math.Sin(t1))
		return
	}
	// Each cubic curve draws at most a quarter of the ellipse.
	n := int(math.Ceil(math.Abs(t1-t0)/(math.Pi/2) - 1e-9))
	if n <= 0 {
		return
	}
	step := (t1 - t0) / float64(n)
	k := 4.0 / 3 * math.Tan(step/4)
	for i := range n {
		ta := t0 + float64(i)*step
		sa, ca := math.Sincos(ta)
		sb, cb := math.Sincos(ta + step)
		b.cubicTo(
			cx+wr*(ca-k*sa), cy+hr*(sa+k*ca),
			cx+wr*(cb+k*sb), cy+hr*(sb-k*cb),
			cx+wr*cb, cy+hr*sb,
		)
	}
}

// param returns the parameter of the point of the ellipse with the radii rx
// and ry in the direction a from its center. The point is (rx cos t, ry sin
// t) for the parameter t. param keeps the number of whole turns of a, so that
// it grows with a.
func param(a, rx, ry float64) float64 {
	if rx == 0 || ry == 0 {
		return a
	}
	turns := math.Round(a / (2 * math.Pi))
	a -= turns * 2 * math.Pi
	return turns*2*math.Pi + math.Atan2(rx*math.Sin(a), ry*math.Cos(a))
}
