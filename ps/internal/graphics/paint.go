package graphics

import (
	"math"

	"github.com/xo/okage"

	"github.com/xo/ivy/ps/internal/interp"
)

// okagePath returns the path of okage of p, or nil for a path that draws
// nothing.
func okagePath(p *Path) *okage.Path {
	pb := okage.NewPathBuilderWithCapacity(len(p.segs), len(p.segs)*3)
	for _, s := range p.segs {
		switch s.op {
		case segMove:
			pb.MoveTo(float32(s.p[0].x), float32(s.p[0].y))
		case segLine:
			pb.LineTo(float32(s.p[0].x), float32(s.p[0].y))
		case segCurve:
			pb.CubicTo(float32(s.p[0].x), float32(s.p[0].y), float32(s.p[1].x), float32(s.p[1].y),
				float32(s.p[2].x), float32(s.p[2].y))
		case segClose:
			pb.Close()
		}
	}
	out, ok := pb.Finish()
	if !ok {
		return nil
	}
	return out
}

// fromOkage returns the path of an okage path.
func fromOkage(p *okage.Path) Path {
	var out Path
	if p == nil {
		return out
	}
	it := p.Segments()
	for {
		s, ok := it.Next()
		if !ok {
			break
		}
		switch s.Kind {
		case okage.PathSegmentMoveTo:
			out.moveTo(point{float64(s.Points[0].X), float64(s.Points[0].Y)})
		case okage.PathSegmentLineTo:
			out.lineTo(point{float64(s.Points[0].X), float64(s.Points[0].Y)})
		case okage.PathSegmentQuadTo:
			c := out.cur
			q := point{float64(s.Points[0].X), float64(s.Points[0].Y)}
			e := point{float64(s.Points[1].X), float64(s.Points[1].Y)}
			out.curveTo(point{c.x + 2.0/3*(q.x-c.x), c.y + 2.0/3*(q.y-c.y)},
				point{e.x + 2.0/3*(q.x-e.x), e.y + 2.0/3*(q.y-e.y)}, e)
		case okage.PathSegmentCubicTo:
			out.curveTo(point{float64(s.Points[0].X), float64(s.Points[0].Y)},
				point{float64(s.Points[1].X), float64(s.Points[1].Y)},
				point{float64(s.Points[2].X), float64(s.Points[2].Y)})
		case okage.PathSegmentClose:
			out.closePath()
		}
	}
	return out
}

// toTransform returns the transform of okage of a matrix.
func toTransform(m Matrix) okage.Transform {
	return okage.TransformFromRow(float32(m[0]), float32(m[1]), float32(m[2]), float32(m[3]), float32(m[4]), float32(m[5]))
}

// mark notes that the current page holds a mark, and makes the device hold
// the clip of the graphics state.
func (m *Machine) mark() {
	m.marked = true
	m.syncClip(m.gs.clip)
}

// fill fills the current path and clears it.
func (m *Machine) fill(evenOdd bool) error {
	defer m.gs.path.reset()
	return m.fillPath(&m.gs.path, evenOdd)
}

// fillPath fills a path of the device space with the color of the
// graphics state.
func (m *Machine) fillPath(p *Path, evenOdd bool) error {
	if p.empty() {
		return nil
	}
	if m.capture != nil && m.capture.ignoreColor() {
		// the glyph of a Type 3 font with setcachedevice takes the color of
		// show
		return m.capture.fill(m, p, evenOdd)
	}
	shader, clip := m.paintShader(m.gs.color)
	if shader == nil {
		return nil
	}
	op := okagePath(p)
	if op == nil {
		return nil
	}
	rule := okage.FillRuleWinding
	if evenOdd {
		rule = okage.FillRuleEvenOdd
	}
	m.mark()
	if clip != nil {
		m.dev.Save()
		defer m.dev.Restore()
		m.dev.ClipPath(clip, okage.FillRuleWinding)
	}
	m.dev.FillPath(op, rule, shader)
	return nil
}

// strokeStyle returns the stroke of okage of the graphics state, in the
// user space.
func (m *Machine) strokeStyle() okage.Stroke {
	st := okage.DefaultStroke()
	// a width of 0 is the thinnest line that the device draws, which okage
	// draws as a hairline. The page widens a thinner line to minLinePixels when
	// it replays the stroke.
	st.Width = float32(m.gs.lineWidth)
	st.MiterLimit = float32(m.gs.miterLimit)
	switch m.gs.lineCap {
	case 1:
		st.LineCap = okage.LineCapRound
	case 2:
		st.LineCap = okage.LineCapSquare
	}
	switch m.gs.lineJoin {
	case 1:
		st.LineJoin = okage.LineJoinRound
	case 2:
		st.LineJoin = okage.LineJoinBevel
	}
	if len(m.gs.dash) > 0 {
		dash := make([]float32, 0, 2*len(m.gs.dash))
		for _, v := range m.gs.dash {
			dash = append(dash, float32(v))
		}
		if len(dash)%2 == 1 {
			dash = append(dash, dash...)
		}
		if d, ok := okage.NewStrokeDash(dash, float32(m.gs.dashOffset)); ok {
			st.Dash = d
		}
	}
	return st
}

// stroke strokes the current path and clears it.
func (m *Machine) stroke() error {
	defer m.gs.path.reset()
	if m.gs.path.empty() && !m.gs.path.hasCur {
		return nil
	}
	inv, ok := m.gs.ctm.Invert()
	if !ok {
		return nil
	}
	if m.capture != nil && m.capture.ignoreColor() {
		outline := m.strokeOutline()
		return m.capture.fill(m, &outline, false)
	}
	shader, clip := m.paintShader(m.gs.color)
	if shader == nil {
		return nil
	}
	user := m.gs.path.transform(inv)
	p := okagePath(&user)
	if p == nil {
		p = dotPath(&user)
		if p == nil {
			return nil
		}
	}
	m.mark()
	m.dev.Save()
	if clip != nil {
		m.dev.ClipPath(clip, okage.FillRuleWinding)
	}
	m.dev.Concat(toTransform(m.gs.ctm))
	m.dev.strokeMin(p, m.strokeStyle(), m.gs.strokeAdjust, m.userShader(shader, inv), m.gs.ctm.Scale())
	m.dev.Restore()
	return nil
}

// dotPath returns a path for a subpath of one point, which a round cap
// draws as a dot, or nil.
func dotPath(p *Path) *okage.Path {
	if len(p.segs) == 0 {
		return nil
	}
	q := p.segs[0].p[0]
	pb := okage.NewPathBuilder()
	pb.MoveTo(float32(q.x), float32(q.y))
	pb.LineTo(float32(q.x), float32(q.y))
	out, _ := pb.Finish()
	return out
}

// userShader returns a shader of the device space for drawing in a user
// space: a pattern takes the inverse of the transform to the device.
func (m *Machine) userShader(s okage.Shader, inv Matrix) okage.Shader {
	if _, ok := s.(okage.SolidColor); ok {
		return s
	}
	return s.Transform(toTransform(inv))
}

// strokeOutline returns the outline of the stroke of the current path, in
// the device space, for strokepath.
func (m *Machine) strokeOutline() Path {
	inv, ok := m.gs.ctm.Invert()
	if !ok {
		return Path{}
	}
	user := m.gs.path.transform(inv)
	p := okagePath(&user)
	if p == nil {
		return Path{}
	}
	st := m.strokeStyle()
	if st.Width == 0 {
		st.Width = float32(1 / math.Max(m.gs.ctm.Scale(), 1e-9))
	}
	res := float32(math.Max(m.gs.ctm.Scale(), 1e-3))
	if st.Dash != nil {
		if d, ok := p.Dash(st.Dash, res); ok {
			p = d
		}
		st.Dash = nil
	}
	out, ok := p.Stroke(&st, res)
	if !ok {
		return Path{}
	}
	stroked := fromOkage(out)
	return stroked.transform(m.gs.ctm)
}

// clip intersects the clip with the current path.
func (m *Machine) clip(evenOdd bool) {
	rule := okage.FillRuleWinding
	if evenOdd {
		rule = okage.FillRuleEvenOdd
	}
	m.clipWith(&m.gs.path, rule)
}

// clipWith intersects the clip with a path of the device space.
func (m *Machine) clipWith(p *Path, rule okage.FillRule) {
	n := &clipNode{parent: m.gs.clip, path: okagePath(p), rule: rule}
	if m.gs.clip != nil {
		n.depth = m.gs.clip.depth + 1
	}
	x0, y0, x1, y1, ok := p.bbox()
	if !ok {
		x0, y0, x1, y1 = 0, 0, 0, 0
	}
	if pc := m.gs.clip; pc != nil {
		x0, y0 = math.Max(x0, pc.bbox[0]), math.Max(y0, pc.bbox[1])
		x1, y1 = math.Min(x1, pc.bbox[2]), math.Min(y1, pc.bbox[3])
	}
	n.bbox = [4]float64{x0, y0, x1, y1}
	m.gs.clip = n
}

// pageRect returns the rectangle of the page in the device space.
func (m *Machine) pageRect() Path {
	var p Path
	w, h := m.width*resolutionScale, m.height*resolutionScale
	p.moveTo(point{0, 0})
	p.lineTo(point{w, 0})
	p.lineTo(point{w, h})
	p.lineTo(point{0, h})
	p.closePath()
	return p
}

// clipPath returns the path of the clip, for clippath. The clip of
// several paths gives the innermost one.
func (m *Machine) clipPath() Path {
	if m.gs.clip == nil {
		return m.pageRect()
	}
	return fromOkage(m.gs.clip.path)
}

// rects pops the operands of rectfill, rectstroke and rectclip: four
// numbers, or an array of numbers, and returns the rectangles.
func (m *Machine) rects() ([][4]float64, error) {
	in := m.in
	top, err := in.Peek(0)
	if err != nil {
		return nil, err
	}
	if top.IsArray() {
		_, _ = in.Pop()
		a := top.Array()
		var out [][4]float64
		for i := 0; i+3 < len(a); i += 4 {
			var r [4]float64
			for j := range 4 {
				if !a[i+j].IsNumber() {
					return nil, interp.ErrTypecheck
				}
				r[j] = a[i+j].Float()
			}
			out = append(out, r)
		}
		return out, nil
	}
	if top.Type() == interp.TString {
		_, _ = in.Pop()
		return nil, nil
	}
	v, err := in.PopNums(4)
	if err != nil {
		return nil, err
	}
	return [][4]float64{{v[0], v[1], v[2], v[3]}}, nil
}

// rectPath returns the path of rectangles in the user space.
func (m *Machine) rectPath(rs [][4]float64) Path {
	var p Path
	for _, r := range rs {
		x, y, w, h := r[0], r[1], r[2], r[3]
		p.moveTo(m.device(x, y))
		p.lineTo(m.device(x+w, y))
		p.lineTo(m.device(x+w, y+h))
		p.lineTo(m.device(x, y+h))
		p.closePath()
	}
	return p
}

// opsPaint are the operators of painting and clipping (PLRM 8.1).
func (m *Machine) opsPaint() {
	in := m.in
	in.Define("fill", func(*interp.Interp) error { return m.fill(false) })
	in.Define("eofill", func(*interp.Interp) error { return m.fill(true) })
	in.Define("stroke", func(*interp.Interp) error { return m.stroke() })
	in.Define("clip", func(*interp.Interp) error {
		m.clip(false)
		return nil
	})
	in.Define("eoclip", func(*interp.Interp) error {
		m.clip(true)
		return nil
	})
	in.Define("initclip", func(*interp.Interp) error {
		m.gs.clip = nil
		return nil
	})
	in.Define("clipsave", func(*interp.Interp) error {
		m.gsave()
		return nil
	})
	in.Define("cliprestore", func(*interp.Interp) error {
		c := m.gs.clip
		m.grestore()
		_ = c
		return nil
	})
	in.Define("rectfill", func(*interp.Interp) error {
		rs, err := m.rects()
		if err != nil {
			return err
		}
		p := m.rectPath(rs)
		return m.fillPath(&p, false)
	})
	in.Define("rectstroke", func(*interp.Interp) error {
		top, err := in.Peek(0)
		if err != nil {
			return err
		}
		var mat *Matrix
		if top.IsArray() && top.Len() == 6 {
			if next, err := in.Peek(1); err == nil && (next.IsArray() || next.IsNumber() || next.Type() == interp.TString) {
				mt, err := matrixOf(top)
				if err != nil {
					return err
				}
				_, _ = in.Pop()
				mat = &mt
			}
		}
		rs, err := m.rects()
		if err != nil {
			return err
		}
		saved := m.gs.path
		defer func() { m.gs.path = saved }()
		m.gs.path = m.rectPath(rs)
		if mat != nil {
			ctm := m.gs.ctm
			defer func() { m.gs.ctm = ctm }()
			m.gs.ctm = mat.Mul(ctm)
		}
		return m.stroke()
	})
	in.Define("rectclip", func(*interp.Interp) error {
		rs, err := m.rects()
		if err != nil {
			return err
		}
		p := m.rectPath(rs)
		m.clipWith(&p, okage.FillRuleWinding)
		m.gs.path.reset()
		return nil
	})
}
