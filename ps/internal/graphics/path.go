package graphics

import (
	"math"
	"slices"

	"github.com/xo/ivy/ps/internal/interp"
)

// point is a point of the device space.
type point struct{ x, y float64 }

// The kinds of the segments of a path.
const (
	segMove = iota
	segLine
	segCurve
	segClose
)

// seg is a segment of a path, in the device space. A curve uses the three
// points, a move and a line the first.
type seg struct {
	op uint8
	p  [3]point
}

// Path is the current path of PostScript, in the device space (PLRM 4.4).
type Path struct {
	segs   []seg
	cur    point
	start  point
	hasCur bool
}

// clone returns a copy of the path that shares no memory that an append
// changes.
func (p Path) clone() Path {
	p.segs = slices.Clip(p.segs)
	return p
}

// reset empties the path.
func (p *Path) reset() {
	*p = Path{}
}

// moveTo starts a subpath.
func (p *Path) moveTo(q point) {
	if n := len(p.segs); n > 0 && p.segs[n-1].op == segMove {
		p.segs[n-1].p[0] = q
	} else {
		p.segs = append(p.segs, seg{op: segMove, p: [3]point{q}})
	}
	p.cur, p.start, p.hasCur = q, q, true
}

// lineTo adds a line.
func (p *Path) lineTo(q point) {
	p.segs = append(p.segs, seg{op: segLine, p: [3]point{q}})
	p.cur = q
}

// curveTo adds a cubic Bezier curve.
func (p *Path) curveTo(a, b, c point) {
	p.segs = append(p.segs, seg{op: segCurve, p: [3]point{a, b, c}})
	p.cur = c
}

// closePath closes the subpath.
func (p *Path) closePath() {
	if !p.hasCur {
		return
	}
	if n := len(p.segs); n > 0 && (p.segs[n-1].op == segClose || p.segs[n-1].op == segMove) {
		return
	}
	p.segs = append(p.segs, seg{op: segClose})
	p.cur = p.start
}

// empty reports whether the path has no segment that draws.
func (p *Path) empty() bool {
	for _, s := range p.segs {
		if s.op != segMove {
			return false
		}
	}
	return true
}

// bbox returns the bounds of the points of the path, and false for an
// empty path.
func (p *Path) bbox() (x0, y0, x1, y1 float64, ok bool) {
	x0, y0, x1, y1 = math.Inf(1), math.Inf(1), math.Inf(-1), math.Inf(-1)
	for _, s := range p.segs {
		n := 0
		switch s.op {
		case segMove, segLine:
			n = 1
		case segCurve:
			n = 3
		}
		for _, q := range s.p[:n] {
			x0, y0 = math.Min(x0, q.x), math.Min(y0, q.y)
			x1, y1 = math.Max(x1, q.x), math.Max(y1, q.y)
			ok = true
		}
	}
	return x0, y0, x1, y1, ok
}

// transform returns the path with each point transformed by t.
func (p *Path) transform(t Matrix) Path {
	out := Path{segs: make([]seg, len(p.segs)), hasCur: p.hasCur}
	for i, s := range p.segs {
		out.segs[i].op = s.op
		for j := range s.p {
			x, y := t.Apply(s.p[j].x, s.p[j].y)
			out.segs[i].p[j] = point{x, y}
		}
	}
	out.cur.x, out.cur.y = t.Apply(p.cur.x, p.cur.y)
	out.start.x, out.start.y = t.Apply(p.start.x, p.start.y)
	return out
}

// appendPath adds the segments of q.
func (p *Path) appendPath(q *Path) {
	if len(q.segs) == 0 {
		return
	}
	p.segs = append(p.segs, q.segs...)
	p.cur, p.start, p.hasCur = q.cur, q.start, q.hasCur
}

// flatten returns the path with each curve replaced by lines, within tol
// in the device space.
func (p *Path) flatten(tol float64) Path {
	out := Path{hasCur: p.hasCur, cur: p.cur, start: p.start}
	var cur point
	for _, s := range p.segs {
		switch s.op {
		case segMove:
			out.segs = append(out.segs, s)
			cur = s.p[0]
		case segLine:
			out.segs = append(out.segs, s)
			cur = s.p[0]
		case segCurve:
			n := curveSteps(cur, s.p[0], s.p[1], s.p[2], tol)
			for i := 1; i <= n; i++ {
				t := float64(i) / float64(n)
				out.segs = append(out.segs, seg{op: segLine, p: [3]point{bezier(cur, s.p[0], s.p[1], s.p[2], t)}})
			}
			cur = s.p[2]
		case segClose:
			out.segs = append(out.segs, s)
		}
	}
	return out
}

// curveSteps returns the number of lines that flatten a curve within tol.
func curveSteps(p0, p1, p2, p3 point, tol float64) int {
	dd := math.Max(
		math.Hypot(p0.x-2*p1.x+p2.x, p0.y-2*p1.y+p2.y),
		math.Hypot(p1.x-2*p2.x+p3.x, p1.y-2*p2.y+p3.y))
	if tol <= 0 {
		tol = 0.5
	}
	n := int(math.Ceil(math.Sqrt(0.75 * dd / tol)))
	return min(max(n, 1), 256)
}

// bezier returns the point of a cubic Bezier curve at t.
func bezier(p0, p1, p2, p3 point, t float64) point {
	u := 1 - t
	a, b, c, d := u*u*u, 3*u*u*t, 3*u*t*t, t*t*t
	return point{a*p0.x + b*p1.x + c*p2.x + d*p3.x, a*p0.y + b*p1.y + c*p2.y + d*p3.y}
}

// reverse returns the path with each subpath reversed.
func (p *Path) reverse() Path {
	out := Path{hasCur: p.hasCur, cur: p.cur, start: p.start}
	i := 0
	for i < len(p.segs) {
		if p.segs[i].op != segMove {
			i++
			continue
		}
		j := i + 1
		for j < len(p.segs) && p.segs[j].op != segMove {
			j++
		}
		sub := p.segs[i:j]
		closed := sub[len(sub)-1].op == segClose
		if closed {
			sub = sub[:len(sub)-1]
		}
		// starts holds the start point of each segment
		starts := make([]point, len(sub))
		cur := sub[0].p[0]
		for k := 1; k < len(sub); k++ {
			starts[k] = cur
			switch sub[k].op {
			case segLine:
				cur = sub[k].p[0]
			case segCurve:
				cur = sub[k].p[2]
			}
		}
		out.segs = append(out.segs, seg{op: segMove, p: [3]point{cur}})
		last := cur
		for k := len(sub) - 1; k >= 1; k-- {
			switch s := sub[k]; s.op {
			case segLine:
				out.segs = append(out.segs, seg{op: segLine, p: [3]point{starts[k]}})
				last = starts[k]
			case segCurve:
				out.segs = append(out.segs, seg{op: segCurve, p: [3]point{s.p[1], s.p[0], starts[k]}})
				last = starts[k]
			}
		}
		out.start = cur
		out.cur = last
		if closed {
			out.segs = append(out.segs, seg{op: segClose})
			out.cur = cur
		}
		i = j
	}
	return out
}

// requireCurrent returns nocurrentpoint when the path has no current point.
func (m *Machine) requireCurrent() error {
	if !m.gs.path.hasCur {
		return interp.ErrNoCurrentPoint
	}
	return nil
}

// device returns the device point of a point of the user space.
func (m *Machine) device(x, y float64) point {
	dx, dy := m.gs.ctm.Apply(x, y)
	return point{dx, dy}
}

// user returns the point of the user space of a device point.
func (m *Machine) user(q point) (float64, float64, error) {
	inv, ok := m.gs.ctm.Invert()
	if !ok {
		return 0, 0, interp.ErrUndefinedResult
	}
	x, y := inv.Apply(q.x, q.y)
	return x, y, nil
}

// arc adds an arc of a circle of the user space, from angle a1 to a2, in
// degrees, counterclockwise or clockwise. It draws a line from the current
// point to the start of the arc.
func (m *Machine) arc(cx, cy, r, a1, a2 float64, clockwise bool) {
	if clockwise {
		for a2 > a1 {
			a2 -= 360
		}
	} else {
		for a2 < a1 {
			a2 += 360
		}
	}
	sweep := a2 - a1
	if math.Abs(sweep) > 3600 {
		sweep = math.Copysign(3600, sweep)
	}
	s, c := sinCos(a1)
	start := m.device(cx+r*c, cy+r*s)
	if m.gs.path.hasCur {
		m.gs.path.lineTo(start)
	} else {
		m.gs.path.moveTo(start)
	}
	n := int(math.Ceil(math.Abs(sweep) / 90))
	if n == 0 {
		return
	}
	step := sweep / float64(n) * math.Pi / 180
	k := 4.0 / 3 * math.Tan(step/4)
	a := a1 * math.Pi / 180
	for range n {
		b := a + step
		sa, ca := math.Sincos(a)
		sb, cb := math.Sincos(b)
		p1 := m.device(cx+r*(ca-k*sa), cy+r*(sa+k*ca))
		p2 := m.device(cx+r*(cb+k*sb), cy+r*(sb-k*cb))
		p3 := m.device(cx+r*cb, cy+r*sb)
		m.gs.path.curveTo(p1, p2, p3)
		a = b
	}
}

// arct adds the arc of arct and arcto, and returns its tangent points in
// the user space.
func (m *Machine) arct(x1, y1, x2, y2, r float64) (float64, float64, float64, float64, error) {
	if err := m.requireCurrent(); err != nil {
		return 0, 0, 0, 0, err
	}
	x0, y0, err := m.user(m.gs.path.cur)
	if err != nil {
		return 0, 0, 0, 0, err
	}
	// the unit vectors from the corner to the two points
	ux, uy := x0-x1, y0-y1
	vx, vy := x2-x1, y2-y1
	lu, lv := math.Hypot(ux, uy), math.Hypot(vx, vy)
	if lu == 0 || lv == 0 || r == 0 {
		m.gs.path.lineTo(m.device(x1, y1))
		return x1, y1, x1, y1, nil
	}
	ux, uy, vx, vy = ux/lu, uy/lu, vx/lv, vy/lv
	cross := ux*vy - uy*vx
	if math.Abs(cross) < 1e-9 {
		m.gs.path.lineTo(m.device(x1, y1))
		return x1, y1, x1, y1, nil
	}
	half := math.Acos(math.Max(-1, math.Min(1, ux*vx+uy*vy))) / 2
	d := r / math.Tan(half)
	t1x, t1y := x1+ux*d, y1+uy*d
	t2x, t2y := x1+vx*d, y1+vy*d
	// the center is on the bisector, at r/sin(half) from the corner
	bx, by := ux+vx, uy+vy
	lb := math.Hypot(bx, by)
	h := r / math.Sin(half)
	cx, cy := x1+bx/lb*h, y1+by/lb*h
	a1 := math.Atan2(t1y-cy, t1x-cx) * 180 / math.Pi
	a2 := math.Atan2(t2y-cy, t2x-cx) * 180 / math.Pi
	m.gs.path.lineTo(m.device(t1x, t1y))
	m.arc(cx, cy, r, a1, a2, cross > 0)
	return t1x, t1y, t2x, t2y, nil
}

// opsPath are the operators of paths (PLRM 8.1).
func (m *Machine) opsPath() {
	in := m.in
	in.Define("newpath", func(*interp.Interp) error {
		m.gs.path.reset()
		return nil
	})
	in.Define("currentpoint", func(*interp.Interp) error {
		if err := m.requireCurrent(); err != nil {
			return err
		}
		x, y, err := m.user(m.gs.path.cur)
		if err != nil {
			return err
		}
		in.PushNum(x)
		in.PushNum(y)
		return nil
	})
	in.Define("moveto", func(*interp.Interp) error {
		v, err := in.PopNums(2)
		if err != nil {
			return err
		}
		m.gs.path.moveTo(m.device(v[0], v[1]))
		return nil
	})
	in.Define("rmoveto", func(*interp.Interp) error {
		if err := m.requireCurrent(); err != nil {
			return err
		}
		v, err := in.PopNums(2)
		if err != nil {
			return err
		}
		dx, dy := m.gs.ctm.ApplyDist(v[0], v[1])
		c := m.gs.path.cur
		m.gs.path.moveTo(point{c.x + dx, c.y + dy})
		return nil
	})
	in.Define("lineto", func(*interp.Interp) error {
		if err := m.requireCurrent(); err != nil {
			return err
		}
		v, err := in.PopNums(2)
		if err != nil {
			return err
		}
		m.gs.path.lineTo(m.device(v[0], v[1]))
		return nil
	})
	in.Define("rlineto", func(*interp.Interp) error {
		if err := m.requireCurrent(); err != nil {
			return err
		}
		v, err := in.PopNums(2)
		if err != nil {
			return err
		}
		dx, dy := m.gs.ctm.ApplyDist(v[0], v[1])
		c := m.gs.path.cur
		m.gs.path.lineTo(point{c.x + dx, c.y + dy})
		return nil
	})
	in.Define("curveto", func(*interp.Interp) error {
		if err := m.requireCurrent(); err != nil {
			return err
		}
		v, err := in.PopNums(6)
		if err != nil {
			return err
		}
		m.gs.path.curveTo(m.device(v[0], v[1]), m.device(v[2], v[3]), m.device(v[4], v[5]))
		return nil
	})
	in.Define("rcurveto", func(*interp.Interp) error {
		if err := m.requireCurrent(); err != nil {
			return err
		}
		v, err := in.PopNums(6)
		if err != nil {
			return err
		}
		c := m.gs.path.cur
		rel := func(x, y float64) point {
			dx, dy := m.gs.ctm.ApplyDist(x, y)
			return point{c.x + dx, c.y + dy}
		}
		m.gs.path.curveTo(rel(v[0], v[1]), rel(v[2], v[3]), rel(v[4], v[5]))
		return nil
	})
	in.Define("arc", func(*interp.Interp) error {
		v, err := in.PopNums(5)
		if err != nil {
			return err
		}
		m.arc(v[0], v[1], v[2], v[3], v[4], false)
		return nil
	})
	in.Define("arcn", func(*interp.Interp) error {
		v, err := in.PopNums(5)
		if err != nil {
			return err
		}
		m.arc(v[0], v[1], v[2], v[3], v[4], true)
		return nil
	})
	in.Define("arct", func(*interp.Interp) error {
		v, err := in.PopNums(5)
		if err != nil {
			return err
		}
		_, _, _, _, err = m.arct(v[0], v[1], v[2], v[3], v[4])
		return err
	})
	in.Define("arcto", func(*interp.Interp) error {
		v, err := in.PopNums(5)
		if err != nil {
			return err
		}
		a, b, c, d, err := m.arct(v[0], v[1], v[2], v[3], v[4])
		if err != nil {
			return err
		}
		for _, x := range []float64{a, b, c, d} {
			in.PushNum(x)
		}
		return nil
	})
	in.Define("closepath", func(*interp.Interp) error {
		m.gs.path.closePath()
		return nil
	})
	in.Define("flattenpath", func(*interp.Interp) error {
		m.gs.path = m.gs.path.flatten(m.flatTolerance())
		return nil
	})
	in.Define("reversepath", func(*interp.Interp) error {
		m.gs.path = m.gs.path.reverse()
		return nil
	})
	in.Define("strokepath", func(*interp.Interp) error {
		m.gs.path = m.strokeOutline()
		return nil
	})
	in.Define("pathbbox", func(*interp.Interp) error {
		x0, y0, x1, y1, err := m.pathBBox(&m.gs.path)
		if err != nil {
			return err
		}
		for _, v := range []float64{x0, y0, x1, y1} {
			in.PushNum(v)
		}
		return nil
	})
	in.Define("pathforall", m.pathForall)
	in.Define("clippath", func(*interp.Interp) error {
		m.gs.path = m.clipPath()
		return nil
	})
	in.Define("setbbox", func(*interp.Interp) error {
		_, err := in.PopNums(4)
		return err
	})
	in.Define("ucache", func(*interp.Interp) error { return nil })
	in.Define("uappend", func(*interp.Interp) error {
		o, err := in.Pop()
		if err != nil {
			return err
		}
		return m.uappend(o)
	})
	in.Define("upath", func(*interp.Interp) error {
		if _, err := in.PopBool(); err != nil {
			return err
		}
		return m.upath()
	})
	for _, op := range []string{"ufill", "ueofill", "ustroke", "ustrokepath"} {
		in.Define(op, func(*interp.Interp) error {
			o, err := in.Pop()
			if err != nil {
				return err
			}
			if o.IsArray() && !o.IsProc() && op != "ustrokepath" {
				// a matrix after the user path of ustroke
				if top, err := in.Peek(0); err == nil && top.IsArray() && o.Len() == 6 {
					mat, err := matrixOf(o)
					if err != nil {
						return err
					}
					o, _ = in.Pop()
					saved := m.gs.ctm
					defer func() { m.gs.ctm = saved }()
					m.gsave()
					defer m.grestore()
					m.gs.path.reset()
					if err := m.uappend(o); err != nil {
						return err
					}
					m.gs.ctm = mat.Mul(m.gs.ctm)
					return m.paintOp(op)
				}
			}
			if op == "ustrokepath" {
				m.gs.path.reset()
				if err := m.uappend(o); err != nil {
					return err
				}
				m.gs.path = m.strokeOutline()
				return nil
			}
			m.gsave()
			defer m.grestore()
			m.gs.path.reset()
			if err := m.uappend(o); err != nil {
				return err
			}
			return m.paintOp(op)
		})
	}
	for _, op := range []string{"infill", "ineofill", "instroke", "inufill", "inueofill", "inustroke"} {
		in.Define(op, func(*interp.Interp) error {
			top, err := in.Pop()
			if err != nil {
				return err
			}
			if top.IsNumber() {
				_, _ = in.Pop()
			}
			if op == "inufill" || op == "inueofill" || op == "inustroke" {
				_, _ = in.Pop()
			}
			in.Push(interp.Bool(false))
			return nil
		})
	}
}

// paintOp paints the current path for a user path operator.
func (m *Machine) paintOp(op string) error {
	switch op {
	case "ufill":
		return m.fill(false)
	case "ueofill":
		return m.fill(true)
	}
	return m.stroke()
}

// uappend adds a user path: a procedure or an array of operands and the
// names of the operators of paths (PLRM 4.6).
func (m *Machine) uappend(o interp.Object) error {
	if !o.IsArray() {
		return interp.ErrTypecheck
	}
	return m.in.Call(o.Exec())
}

// upath pushes a user path of the current path.
func (m *Machine) upath() error {
	inv, ok := m.gs.ctm.Invert()
	if !ok {
		return interp.ErrUndefinedResult
	}
	user := m.gs.path.transform(inv)
	var elems []interp.Object
	x0, y0, x1, y1, ok := user.bbox()
	if !ok {
		x0, y0, x1, y1 = 0, 0, 0, 0
	}
	elems = append(elems, interp.Real(x0), interp.Real(y0), interp.Real(x1), interp.Real(y1), interp.ExecName("setbbox"))
	for _, s := range user.segs {
		switch s.op {
		case segMove:
			elems = append(elems, interp.Real(s.p[0].x), interp.Real(s.p[0].y), interp.ExecName("moveto"))
		case segLine:
			elems = append(elems, interp.Real(s.p[0].x), interp.Real(s.p[0].y), interp.ExecName("lineto"))
		case segCurve:
			for _, q := range s.p {
				elems = append(elems, interp.Real(q.x), interp.Real(q.y))
			}
			elems = append(elems, interp.ExecName("curveto"))
		case segClose:
			elems = append(elems, interp.ExecName("closepath"))
		}
	}
	o, err := m.in.NewArray(elems)
	if err != nil {
		return err
	}
	m.in.Push(o.Exec())
	return nil
}

// pathBBox returns the bounds of a path in the user space.
func (m *Machine) pathBBox(p *Path) (x0, y0, x1, y1 float64, err error) {
	if !p.hasCur {
		return 0, 0, 0, 0, interp.ErrNoCurrentPoint
	}
	inv, ok := m.gs.ctm.Invert()
	if !ok {
		return 0, 0, 0, 0, interp.ErrUndefinedResult
	}
	user := p.transform(inv)
	x0, y0, x1, y1, ok = user.bbox()
	if !ok {
		x, y := inv.Apply(p.cur.x, p.cur.y)
		return x, y, x, y, nil
	}
	return x0, y0, x1, y1, nil
}

// pathForall runs pathforall (PLRM 8.2).
func (m *Machine) pathForall(in *interp.Interp) error {
	if in.Count() < 4 {
		return interp.ErrStackUnderflow
	}
	var procs [4]interp.Object
	for i := 3; i >= 0; i-- {
		o, _ := in.Peek(3 - i)
		if !o.IsArray() {
			return interp.ErrTypecheck
		}
		procs[i] = o
	}
	inv, ok := m.gs.ctm.Invert()
	if !ok {
		return interp.ErrUndefinedResult
	}
	for range 4 {
		_, _ = in.Pop()
	}
	user := m.gs.path.transform(inv)
	for _, s := range user.segs {
		var n int
		switch s.op {
		case segMove, segLine:
			n = 1
		case segCurve:
			n = 3
		}
		for _, q := range s.p[:n] {
			in.PushNum(q.x)
			in.PushNum(q.y)
		}
		if err := in.Call(procs[s.op]); err != nil {
			if interp.IsExit(err) {
				return nil
			}
			return err
		}
	}
	return nil
}

// flatTolerance returns the tolerance of flattening in the device space.
func (m *Machine) flatTolerance() float64 {
	return math.Max(0.2, m.gs.flat*resolutionScale/4)
}
