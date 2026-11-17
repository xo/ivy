package vsdx

import (
	"math"
	"slices"
	"sort"
	"strconv"
	"strings"

	"github.com/xo/okage"
)

// pt is a point in the local coordinates of a shape, in inches, with the y
// axis up.
type pt struct {
	x, y float64
}

// sub returns p minus q.
func (p pt) sub(q pt) pt {
	return pt{p.x - q.x, p.y - q.y}
}

// near reports whether p and q are the same point.
func (p pt) near(q pt) bool {
	return math.Abs(p.x-q.x) < 1e-9 && math.Abs(p.y-q.y) < 1e-9
}

// finite reports whether both coordinates are finite and of a size that a
// drawing can hold.
func (p pt) finite() bool {
	return !math.IsNaN(p.x) && !math.IsNaN(p.y) && math.Abs(p.x) < 1e7 && math.Abs(p.y) < 1e7
}

// seg is a segment of a subpath: a line to p[0], a quadratic curve through
// p[0] to p[1], or a cubic curve through p[0] and p[1] to p[2].
type seg struct {
	op byte
	p  [3]pt
}

// The operators of a segment.
const (
	opLine  = 'L'
	opQuad  = 'Q'
	opCubic = 'C'
)

// end returns the last point of the segment.
func (s seg) end() pt {
	switch s.op {
	case opQuad:
		return s.p[1]
	case opCubic:
		return s.p[2]
	}
	return s.p[0]
}

// subpath is a subpath of a geometry: a start and segments.
type subpath struct {
	start pt
	segs  []seg
	// closed reports a subpath that ends where it starts, or an ellipse.
	closed bool
}

// geometry builds the subpaths of the Geometry sections of a shape.
type geometry struct {
	b *builder
	// w and h are the width and the height of the shape, for the rows of
	// relative coordinates.
	w, h float64
	// extent is the length of an infinite line in each direction.
	extent float64
	subs   []subpath
	cur    pt
	open   bool
	// over reports that the shape has more segments than the limits.
	over bool
}

// maxSplinePoints bounds the lines that one spline or NURBS curve becomes.
const maxSplinePoints = 1024

// move starts a subpath at p.
func (g *geometry) move(p pt) {
	g.finish()
	g.subs = append(g.subs, subpath{start: p})
	g.cur, g.open = p, true
}

// ensure starts a subpath at the current point if no subpath is open.
func (g *geometry) ensure() {
	if !g.open {
		g.move(g.cur)
	}
}

// add adds a segment to the open subpath.
func (g *geometry) add(s seg) {
	for i := range s.p {
		if !s.p[i].finite() {
			return
		}
	}
	if !g.b.segs.add(1) {
		g.over = true
		return
	}
	g.ensure()
	sp := &g.subs[len(g.subs)-1]
	sp.segs = append(sp.segs, s)
	g.cur = s.end()
}

// finish closes the open subpath if it ends where it starts.
func (g *geometry) finish() {
	if !g.open || len(g.subs) == 0 {
		return
	}
	sp := &g.subs[len(g.subs)-1]
	if len(sp.segs) > 0 && sp.segs[len(sp.segs)-1].end().near(sp.start) {
		sp.closed = true
	}
	g.open = false
}

// line adds a line to p.
func (g *geometry) line(p pt) {
	g.add(seg{op: opLine, p: [3]pt{p}})
}

// section adds the rows of a Geometry section.
func (g *geometry) section(rows []*rowView, num func(r *rowView, name string) float64, formula func(r *rowView, name string) string) {
	g.open = false
	g.cur = pt{}
	for i := 0; i < len(rows) && !g.over; i++ {
		r := rows[i]
		x, y := num(r, "X"), num(r, "Y")
		switch r.t {
		case "MoveTo":
			g.move(pt{x, y})
		case "RelMoveTo":
			g.move(pt{x * g.w, y * g.h})
		case "LineTo":
			g.line(pt{x, y})
		case "RelLineTo":
			g.line(pt{x * g.w, y * g.h})
		case "ArcTo":
			g.arcTo(pt{x, y}, num(r, "A"))
		case "EllipticalArcTo":
			g.ellipticalArcTo(pt{x, y}, pt{num(r, "A"), num(r, "B")}, num(r, "C"), num(r, "D"))
		case "RelEllipticalArcTo":
			g.ellipticalArcTo(pt{x * g.w, y * g.h}, pt{num(r, "A") * g.w, num(r, "B") * g.h}, num(r, "C"), num(r, "D"))
		case "RelCubBezTo":
			g.add(seg{op: opCubic, p: [3]pt{
				{num(r, "A") * g.w, num(r, "B") * g.h},
				{num(r, "C") * g.w, num(r, "D") * g.h},
				{x * g.w, y * g.h},
			}})
		case "RelQuadBezTo":
			g.add(seg{op: opQuad, p: [3]pt{{num(r, "A") * g.w, num(r, "B") * g.h}, {x * g.w, y * g.h}}})
		case "Ellipse":
			g.ellipse(pt{x, y}, pt{num(r, "A"), num(r, "B")}, pt{num(r, "C"), num(r, "D")})
		case "InfiniteLine":
			g.infiniteLine(pt{x, y}, pt{num(r, "A"), num(r, "B")})
		case "PolylineTo":
			g.polyline(pt{x, y}, formula(r, "A"))
		case "NURBSTo":
			g.nurbs(pt{x, y}, num(r, "A"), num(r, "B"), num(r, "C"), num(r, "D"), formula(r, "E"))
		case "SplineStart":
			// the knots of the spline follow in the rows of SplineKnot
			ctrl := []pt{g.cur, {x, y}}
			knots := []float64{num(r, "B"), num(r, "A")}
			for i+1 < len(rows) && rows[i+1].t == "SplineKnot" && len(ctrl) < maxSplinePoints {
				i++
				k := rows[i]
				ctrl = append(ctrl, pt{num(k, "X"), num(k, "Y")})
				knots = append(knots, num(k, "A"))
			}
			g.spline(ctrl, knots, nil, num(r, "C"), int(num(r, "D")))
		case "SplineKnot":
			// a knot without a start is a line
			g.line(pt{x, y})
		}
	}
	g.finish()
}

// arcTo adds a circular arc to p, whose middle is the distance bow from the
// middle of its chord. A positive bow draws the arc counterclockwise.
func (g *geometry) arcTo(p pt, bow float64) {
	p0 := g.cur
	d := p.sub(p0)
	c := math.Hypot(d.x, d.y)
	if c == 0 || math.Abs(bow) < 1e-9*c || math.IsNaN(bow) {
		g.line(p)
		return
	}
	mid := pt{(p0.x + p.x) / 2, (p0.y + p.y) / 2}
	q := pt{mid.x + bow*d.y/c, mid.y - bow*d.x/c}
	g.arc3(p0, q, p, affine{a: 1, d: 1})
}

// ellipticalArcTo adds an elliptical arc to p through the point q, on an
// ellipse whose major axis is at the angle angle, in radians, and is ratio
// times its minor axis.
func (g *geometry) ellipticalArcTo(p, q pt, angle, ratio float64) {
	if !(ratio > 1e-6) || math.IsInf(ratio, 0) || math.IsNaN(angle) {
		g.line(p)
		return
	}
	// the arc is a circular arc in the space where the ellipse is a circle
	back := rotate(angle).mul(scale(ratio, 1))
	to := scale(1/ratio, 1).mul(rotate(-angle))
	g.arc3(to.apply(g.cur), to.apply(q), to.apply(p), back)
}

// arc3 adds the circular arc from p0 through q to p1, in the space that m
// maps onto the coordinates of the shape. Three points on a line make a
// line.
func (g *geometry) arc3(p0, q, p1 pt, m affine) {
	ax, ay := p0.x, p0.y
	bx, by := q.x, q.y
	cx, cy := p1.x, p1.y
	det := 2 * (ax*(by-cy) + bx*(cy-ay) + cx*(ay-by))
	scale := math.Max(math.Hypot(cx-ax, cy-ay), math.Hypot(bx-ax, by-ay))
	if math.Abs(det) < 1e-12*scale*scale || p0.near(p1) {
		g.line(m.apply(p1))
		return
	}
	a2, b2, c2 := ax*ax+ay*ay, bx*bx+by*by, cx*cx+cy*cy
	ox := (a2*(by-cy) + b2*(cy-ay) + c2*(ay-by)) / det
	oy := (a2*(cx-bx) + b2*(ax-cx) + c2*(bx-ax)) / det
	r := math.Hypot(ax-ox, ay-oy)
	a0 := math.Atan2(ay-oy, ax-ox)
	aq := norm(math.Atan2(by-oy, bx-ox) - a0)
	sweep := norm(math.Atan2(cy-oy, cx-ox) - a0)
	if aq > sweep {
		// the arc goes clockwise through q
		sweep -= 2 * math.Pi
	}
	n := int(math.Ceil(math.Abs(sweep) / (math.Pi / 2)))
	n = min(max(n, 1), 4)
	step := sweep / float64(n)
	k := 4.0 / 3 * math.Tan(step/4)
	for i := range n {
		t0, t1 := a0+step*float64(i), a0+step*float64(i+1)
		s0, c0 := math.Sincos(t0)
		s1, c1 := math.Sincos(t1)
		g.add(seg{op: opCubic, p: [3]pt{
			m.apply(pt{ox + r*(c0-k*s0), oy + r*(s0+k*c0)}),
			m.apply(pt{ox + r*(c1+k*s1), oy + r*(s1-k*c1)}),
			m.apply(pt{ox + r*c1, oy + r*s1}),
		}})
	}
}

// norm returns the angle a in the range from 0 to 2π.
func norm(a float64) float64 {
	a = math.Mod(a, 2*math.Pi)
	if a < 0 {
		a += 2 * math.Pi
	}
	return a
}

// ellipse adds a closed ellipse with the center c, and the ends a and b of
// its two axes.
func (g *geometry) ellipse(c, a, b pt) {
	m := affine{a: a.x - c.x, b: a.y - c.y, c: b.x - c.x, d: b.y - c.y, e: c.x, f: c.y}
	g.move(m.apply(pt{1, 0}))
	const k = 0.5522847498
	quarter := [4][3]pt{
		{{1, k}, {k, 1}, {0, 1}},
		{{-k, 1}, {-1, k}, {-1, 0}},
		{{-1, -k}, {-k, -1}, {0, -1}},
		{{k, -1}, {1, -k}, {1, 0}},
	}
	for _, q := range quarter {
		g.add(seg{op: opCubic, p: [3]pt{m.apply(q[0]), m.apply(q[1]), m.apply(q[2])}})
	}
	g.finish()
	if n := len(g.subs); n > 0 {
		g.subs[n-1].closed = true
	}
}

// infiniteLine adds the line through a and b, long enough to cross the
// page.
func (g *geometry) infiniteLine(a, b pt) {
	d := b.sub(a)
	l := math.Hypot(d.x, d.y)
	if l == 0 {
		return
	}
	e := g.extent + math.Abs(a.x) + math.Abs(a.y)
	ux, uy := d.x/l*e, d.y/l*e
	g.move(pt{a.x - ux, a.y - uy})
	g.line(pt{a.x + ux, a.y + uy})
	g.finish()
}

// funcArgs returns the numbers of the arguments of the function name in the
// formula f, such as "POLYLINE(0, 0, 1,2)".
func funcArgs(f, name string) ([]float64, bool) {
	f = strings.TrimSpace(f)
	if len(f) < len(name)+2 || !strings.EqualFold(f[:len(name)], name) {
		return nil, false
	}
	rest := strings.TrimSpace(f[len(name):])
	if !strings.HasPrefix(rest, "(") || !strings.HasSuffix(rest, ")") {
		return nil, false
	}
	parts := strings.Split(rest[1:len(rest)-1], ",")
	args := make([]float64, 0, len(parts))
	for _, s := range parts {
		v, err := strconv.ParseFloat(strings.TrimSpace(s), 64)
		if err != nil {
			return nil, false
		}
		args = append(args, v)
	}
	return args, true
}

// coord returns the coordinate v of a POLYLINE or a NURBS: a fraction of
// size for the type 0, and an absolute coordinate for the type 1.
func coord(v float64, typ, size float64) float64 {
	if typ == 0 {
		return v * size
	}
	return v
}

// polyline adds the lines of the formula POLYLINE(xType, yType, x1, y1,
// ...), then a line to the end p.
func (g *geometry) polyline(p pt, f string) {
	args, ok := funcArgs(f, "POLYLINE")
	if ok && len(args) >= 2 {
		xt, yt := args[0], args[1]
		for i := 2; i+1 < len(args) && !g.over; i += 2 {
			g.line(pt{coord(args[i], xt, g.w), coord(args[i+1], yt, g.h)})
		}
	}
	g.line(p)
}

// nurbs adds the NURBS curve of a row of NURBSTo: from the current point,
// with the first knot first and its weight firstWeight, through the control
// points of the formula NURBS(knotLast, degree, xType, yType, x1, y1,
// knot1, weight1, ...), to the end p, with the knot lastKnot and the weight
// lastWeight.
func (g *geometry) nurbs(p pt, lastKnot, lastWeight, first, firstWeight float64, f string) {
	args, ok := funcArgs(f, "NURBS")
	if !ok || len(args) < 4 {
		g.line(p)
		return
	}
	knotLast, degree, xt, yt := args[0], int(args[1]), args[2], args[3]
	ctrl := []pt{g.cur}
	knots := []float64{first}
	weights := []float64{firstWeight}
	for i := 4; i+3 < len(args) && len(ctrl) < maxSplinePoints; i += 4 {
		ctrl = append(ctrl, pt{coord(args[i], xt, g.w), coord(args[i+1], yt, g.h)})
		knots = append(knots, args[i+2])
		weights = append(weights, args[i+3])
	}
	ctrl = append(ctrl, p)
	knots = append(knots, lastKnot)
	weights = append(weights, lastWeight)
	g.spline(ctrl, knots, weights, knotLast, degree)
}

// spline adds a B-spline of the degree with the control points ctrl, the
// knot of each control point, and the last knot. weights are the weights
// of a NURBS, or nil. The knot vector ends with the last knot as many times
// as the degree plus 1, so the curve ends at its last control point.
func (g *geometry) spline(ctrl []pt, knots, weights []float64, last float64, degree int) {
	n := len(ctrl)
	end := ctrl[n-1]
	degree = min(degree, n-1, 10)
	if degree < 1 {
		g.line(end)
		return
	}
	kv := make([]float64, 0, n+degree+1)
	kv = append(kv, knots...)
	for range degree + 1 {
		kv = append(kv, last)
	}
	// a knot vector must not decrease
	for i := 1; i < len(kv); i++ {
		if math.IsNaN(kv[i]) || kv[i] < kv[i-1] {
			kv[i] = kv[i-1]
		}
	}
	w := weights
	if len(w) != n {
		w = slices.Repeat([]float64{1}, n)
	}
	for i := range w {
		if !(w[i] > 0) || math.IsInf(w[i], 0) {
			w[i] = 1
		}
	}
	t0, t1 := kv[degree], kv[n]
	if !(t1 > t0) {
		g.line(end)
		return
	}
	steps := min(16*(n-degree), maxSplinePoints)
	for i := 1; i <= steps && !g.over; i++ {
		t := t0 + (t1-t0)*float64(i)/float64(steps)
		g.line(deBoor(ctrl, w, kv, degree, t))
	}
}

// deBoor evaluates the rational B-spline at the parameter t.
func deBoor(ctrl []pt, w, kv []float64, p int, t float64) pt {
	n := len(ctrl)
	// k is the last span whose knot is at most t, from p to n-1
	k := p + sort.Search(n-1-p, func(i int) bool { return t < kv[p+i+1] })
	type hp struct{ x, y, w float64 }
	d := make([]hp, p+1)
	for j := range p + 1 {
		i := j + k - p
		d[j] = hp{ctrl[i].x * w[i], ctrl[i].y * w[i], w[i]}
	}
	for r := 1; r <= p; r++ {
		for j := p; j >= r; j-- {
			i := j + k - p
			den := kv[i+p-r+1] - kv[i]
			a := 0.0
			if den > 0 {
				a = (t - kv[i]) / den
			}
			d[j] = hp{(1-a)*d[j-1].x + a*d[j].x, (1-a)*d[j-1].y + a*d[j].y, (1-a)*d[j-1].w + a*d[j].w}
		}
	}
	if d[p].w == 0 {
		return ctrl[k]
	}
	return pt{d[p].x / d[p].w, d[p].y / d[p].w}
}

// path returns the okage path of the subpaths that keep returns true for,
// with each point mapped by m. A closed subpath ends with a close when
// close is true. It returns nil for no subpaths.
func path(subs []subpath, m affine, keep func(*subpath) bool) *okage.Path {
	pb := okage.NewPathBuilder()
	for i := range subs {
		sp := &subs[i]
		if len(sp.segs) == 0 || !keep(sp) {
			continue
		}
		pb.MoveTo(m.apply32(sp.start))
		for _, s := range sp.segs {
			switch s.op {
			case opLine:
				pb.LineTo(m.apply32(s.p[0]))
			case opQuad:
				x1, y1 := m.apply32(s.p[0])
				x, y := m.apply32(s.p[1])
				pb.QuadTo(x1, y1, x, y)
			case opCubic:
				x1, y1 := m.apply32(s.p[0])
				x2, y2 := m.apply32(s.p[1])
				x, y := m.apply32(s.p[2])
				pb.CubicTo(x1, y1, x2, y2, x, y)
			}
		}
		if sp.closed {
			pb.Close()
		}
	}
	p, ok := pb.Finish()
	if !ok {
		return nil
	}
	return p
}

// ends returns the ends of the open subpaths for the arrows: the first
// point and the direction that the line leaves it, and the last point and
// the direction that the line comes to it. It returns false when the
// subpaths are closed or empty.
func ends(subs []subpath) (begin, beginFrom, end, endFrom pt, ok bool) {
	var first, last *subpath
	for i := range subs {
		if len(subs[i].segs) == 0 {
			continue
		}
		if first == nil {
			first = &subs[i]
		}
		last = &subs[i]
	}
	if first == nil || first.closed && last.closed {
		return pt{}, pt{}, pt{}, pt{}, false
	}
	begin = first.start
	beginFrom = begin
	for _, s := range first.segs {
		if q := s.p[0]; !q.near(begin) {
			beginFrom = q
			break
		}
		if q := s.end(); !q.near(begin) {
			beginFrom = q
			break
		}
	}
	ls := last.segs[len(last.segs)-1]
	end = ls.end()
	endFrom = last.start
	if len(last.segs) > 1 {
		endFrom = last.segs[len(last.segs)-2].end()
	}
	switch ls.op {
	case opQuad:
		endFrom = ls.p[0]
	case opCubic:
		endFrom = ls.p[1]
		if endFrom.near(end) {
			endFrom = ls.p[0]
		}
	}
	return begin, beginFrom, end, endFrom, true
}

// round returns the subpath with its corners rounded by the radius r, as
// the cell Rounding gives it. Only a subpath of lines has its corners
// rounded.
func (sp subpath) round(r float64) subpath {
	if !(r > 0) || len(sp.segs) < 2 {
		return sp
	}
	pts := []pt{sp.start}
	for _, s := range sp.segs {
		if s.op != opLine {
			return sp
		}
		if !s.p[0].near(pts[len(pts)-1]) {
			pts = append(pts, s.p[0])
		}
	}
	if sp.closed && len(pts) > 1 && pts[len(pts)-1].near(pts[0]) {
		pts = pts[:len(pts)-1]
	}
	n := len(pts)
	if n < 3 {
		return sp
	}
	// cut returns the point at the distance d from a toward b
	cut := func(a, b pt, d float64) pt {
		l := math.Hypot(b.x-a.x, b.y-a.y)
		if l == 0 {
			return a
		}
		return pt{a.x + (b.x-a.x)*d/l, a.y + (b.y-a.y)*d/l}
	}
	corner := func(prev, at, next pt) (pt, pt) {
		d := min(r, math.Hypot(at.x-prev.x, at.y-prev.y)/2, math.Hypot(next.x-at.x, next.y-at.y)/2)
		return cut(at, prev, d), cut(at, next, d)
	}
	out := subpath{closed: sp.closed}
	if sp.closed {
		in, start := corner(pts[n-1], pts[0], pts[1])
		out.start = start
		for i := 1; i <= n; i++ {
			at, next := pts[i%n], pts[(i+1)%n]
			a, b := corner(pts[i-1], at, next)
			if i == n {
				a, b = in, start
			}
			out.segs = append(out.segs, seg{op: opLine, p: [3]pt{a}}, seg{op: opQuad, p: [3]pt{at, b}})
		}
		return out
	}
	out.start = pts[0]
	for i := 1; i < n-1; i++ {
		a, b := corner(pts[i-1], pts[i], pts[i+1])
		out.segs = append(out.segs, seg{op: opLine, p: [3]pt{a}}, seg{op: opQuad, p: [3]pt{pts[i], b}})
	}
	out.segs = append(out.segs, seg{op: opLine, p: [3]pt{pts[n-1]}})
	return out
}
