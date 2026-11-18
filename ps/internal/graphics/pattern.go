package graphics

import (
	"math"

	"github.com/xo/okage"

	"github.com/xo/ivy/internal/display"
	"github.com/xo/ivy/ps/internal/interp"
)

// maxTileSide bounds the side of the tile of a pattern, in pixels.
const maxTileSide = 1024

// pattern is a pattern that makepattern made (PLRM 4.9).
type pattern struct {
	dict interp.Object
	typ  int
	// matrix maps the pattern space onto the device space.
	matrix Matrix

	paintType  int
	bbox       [4]float64
	xstep      float64
	ystep      float64
	proc       interp.Object
	rendered   bool
	tile       *okage.Pixmap
	tileMatrix Matrix
	colored    map[[3]uint8]*okage.Pixmap

	shading *shading
}

// makePattern makes the pattern of a dictionary with a matrix.
func (m *Machine) makePattern(d *interp.Dict, mat Matrix) (interp.Object, error) {
	pat := &pattern{matrix: mat.Mul(m.gs.ctm)}
	if v, ok := d.GetName("PatternType"); ok && v.IsNumber() {
		pat.typ = int(v.Float())
	}
	switch pat.typ {
	case 1:
		if v, ok := d.GetName("PaintType"); ok && v.IsNumber() {
			pat.paintType = int(v.Float())
		}
		b, ok := numbers(d, "BBox", 4)
		if !ok {
			return interp.Null, interp.ErrRangecheck
		}
		pat.bbox = [4]float64{math.Min(b[0], b[2]), math.Min(b[1], b[3]), math.Max(b[0], b[2]), math.Max(b[1], b[3])}
		xs, ok1 := d.GetName("XStep")
		ys, ok2 := d.GetName("YStep")
		if !ok1 || !ok2 || !xs.IsNumber() || !ys.IsNumber() || xs.Float() == 0 || ys.Float() == 0 {
			return interp.Null, interp.ErrRangecheck
		}
		pat.xstep, pat.ystep = math.Abs(xs.Float()), math.Abs(ys.Float())
		proc, ok := d.GetName("PaintProc")
		if !ok {
			return interp.Null, interp.ErrRangecheck
		}
		pat.proc = proc
	case 2:
		so, ok := d.GetName("Shading")
		if !ok {
			return interp.Null, interp.ErrRangecheck
		}
		s, err := m.parseShading(so)
		if err != nil {
			return interp.Null, err
		}
		pat.shading = s
	default:
		return interp.Null, interp.ErrRangecheck
	}
	cp := m.in.NewDict(d.Len() + 1)
	d.Each(func(k, v interp.Object) bool {
		cp.Dict().Put(k, v)
		return true
	})
	cp.Dict().PutName("Implementation", interp.GState(pat))
	pat.dict = cp
	return cp, nil
}

// patternOf returns the pattern of a dictionary that makepattern made.
func patternOf(o interp.Object) *pattern {
	d := o.Dict()
	if d == nil {
		return nil
	}
	v, ok := d.GetName("Implementation")
	if !ok {
		return nil
	}
	p, _ := v.Ref().(*pattern)
	return p
}

// setPatternColor sets a pattern as the color, for setcolor in the space
// Pattern.
func (m *Machine) setPatternColor(cs *colorSpace) error {
	in := m.in
	o, err := in.Peek(0)
	if err != nil {
		return err
	}
	pat := patternOf(o)
	if pat == nil {
		return interp.ErrTypecheck
	}
	_, _ = in.Pop()
	var comps []float64
	if pat.typ == 1 && pat.paintType == 2 {
		n := 1
		if cs.base != nil {
			n = cs.base.n
		}
		if comps, err = in.PopNums(n); err != nil {
			return err
		}
	}
	m.gs.color = paint{space: cs, comps: comps, pattern: pat}
	return nil
}

// patternShader returns the shader of a pattern paint in the device space,
// and the path in the device space that a fill with it clips to, or nil.
func (m *Machine) patternShader(p paint) (okage.Shader, *okage.Path) {
	pat := p.pattern
	if pat.typ == 2 {
		return m.shadingShader(pat)
	}
	if !pat.rendered {
		pat.rendered = true
		m.renderTile(pat)
	}
	if pat.tile == nil {
		return nil, nil
	}
	pix := pat.tile
	if pat.paintType == 2 {
		rgb, ok := m.toRGB(p.space.base, p.comps, 0)
		if !ok {
			return nil, nil
		}
		pix = pat.colorize(rgb)
	}
	return okage.NewPattern(pix.AsRef(), okage.SpreadModeRepeat, okage.FilterQualityBilinear, 1, toTransform(pat.tileMatrix)), nil
}

// colorize returns the tile of an uncolored pattern in a color.
func (pat *pattern) colorize(rgb [3]float64) *okage.Pixmap {
	key := [3]uint8{uint8(clamp01(rgb[0]) * 255), uint8(clamp01(rgb[1]) * 255), uint8(clamp01(rgb[2]) * 255)}
	if pix, ok := pat.colored[key]; ok {
		return pix
	}
	pix := pat.tile.Clone()
	data := pix.Data()
	for i := 0; i+3 < len(data); i += 4 {
		a := uint32(data[i+3])
		data[i] = uint8(uint32(key[0]) * a / 255)
		data[i+1] = uint8(uint32(key[1]) * a / 255)
		data[i+2] = uint8(uint32(key[2]) * a / 255)
	}
	if pat.colored == nil || len(pat.colored) > 64 {
		pat.colored = map[[3]uint8]*okage.Pixmap{}
	}
	pat.colored[key] = pix
	return pix
}

// renderTile runs the PaintProc of a tiling pattern into the pixmap of one
// tile.
func (m *Machine) renderTile(pat *pattern) {
	mat := pat.matrix
	kx := math.Hypot(mat[0], mat[1])
	ky := math.Hypot(mat[2], mat[3])
	pw := math.Ceil(pat.xstep * kx)
	ph := math.Ceil(pat.ystep * ky)
	if !(pw >= 1) {
		pw = 1
	}
	if !(ph >= 1) {
		ph = 1
	}
	pw, ph = math.Min(pw, maxTileSide), math.Min(ph, maxTileSide)
	// pixToPattern maps the pixels of the tile onto the pattern space
	pixToPattern := Matrix{pat.xstep / pw, 0, 0, pat.ystep / ph, pat.bbox[0], pat.bbox[1]}
	patternToPix, ok := pixToPattern.Invert()
	if !ok {
		return
	}
	pat.tileMatrix = pixToPattern.Mul(mat)
	rec := m.runPaintProc(pat, patternToPix)
	if rec == nil {
		return
	}
	pix, ok := okage.NewPixmap(uint32(pw), uint32(ph))
	if !ok {
		return
	}
	dev := display.NewOkage(pix, okage.TransformFromTranslate(0, 0))
	for i := -1; i <= 1; i++ {
		for j := -1; j <= 1; j++ {
			dev.Save()
			dev.Concat(okage.TransformFromTranslate(float32(float64(i)*pw), float32(float64(j)*ph)))
			rec.Replay(dev, 1)
			dev.Restore()
		}
	}
	pat.tile = pix
}

// runPaintProc runs the PaintProc of a pattern with the CTM ctm, and
// returns the commands that it drew.
func (m *Machine) runPaintProc(pat *pattern, ctm Matrix) *Recorder {
	if m.patternDepth >= 4 {
		return nil
	}
	m.patternDepth++
	defer func() { m.patternDepth-- }()
	dev, applied, marked, capture := m.dev, m.applied, m.marked, m.capture
	rec := newRecorder(m.lim)
	m.dev, m.applied, m.capture = rec, nil, nil
	m.gsave()
	m.gs.ctm = ctm
	m.gs.clip = nil
	m.gs.path.reset()
	if pat.paintType == 2 {
		m.gs.color = paint{space: deviceGray, comps: []float64{0}}
	}
	var bb Path
	bb.moveTo(m.device(pat.bbox[0], pat.bbox[1]))
	bb.lineTo(m.device(pat.bbox[2], pat.bbox[1]))
	bb.lineTo(m.device(pat.bbox[2], pat.bbox[3]))
	bb.lineTo(m.device(pat.bbox[0], pat.bbox[3]))
	bb.closePath()
	m.clipWith(&bb, okage.FillRuleWinding)
	m.in.Push(pat.dict)
	err := m.in.Call(pat.proc)
	m.clearClip()
	m.grestore()
	m.dev, m.applied, m.marked, m.capture = dev, applied, marked, capture
	if err != nil && !interp.IsSignal(err) && isFatal(err) {
		m.log.Debug("postscript: pattern", "error", err)
	}
	return rec
}

// execForm draws a form, as execform does.
func (m *Machine) execForm(d *interp.Dict, fo interp.Object) error {
	proc, ok := d.GetName("PaintProc")
	if !ok {
		return interp.ErrRangecheck
	}
	m.gsave()
	defer m.grestore()
	if v, ok := d.GetName("Matrix"); ok {
		if mat, err := matrixOf(v); err == nil {
			m.gs.ctm = mat.Mul(m.gs.ctm)
		}
	}
	if b, ok := numbers(d, "BBox", 4); ok {
		var p Path
		p.moveTo(m.device(b[0], b[1]))
		p.lineTo(m.device(b[2], b[1]))
		p.lineTo(m.device(b[2], b[3]))
		p.lineTo(m.device(b[0], b[3]))
		p.closePath()
		m.clipWith(&p, okage.FillRuleWinding)
	}
	m.gs.path.reset()
	m.in.Push(fo)
	return m.in.Call(proc)
}

// opsPattern are the operators of patterns, shadings and forms (PLRM 8.1).
func (m *Machine) opsPattern() {
	in := m.in
	in.Define("makepattern", func(*interp.Interp) error {
		mat, _, err := m.popMatrix()
		if err != nil {
			return err
		}
		d, err := in.PopDict()
		if err != nil {
			return err
		}
		o, err := m.makePattern(d, mat)
		if err != nil {
			return err
		}
		in.Push(o)
		return nil
	})
	in.Define("setpattern", func(*interp.Interp) error {
		o, err := in.Peek(0)
		if err != nil {
			return err
		}
		pat := patternOf(o)
		if pat == nil {
			return interp.ErrTypecheck
		}
		cs := m.gs.color.space
		if cs.family != "Pattern" {
			ncs := &colorSpace{family: "Pattern"}
			if pat.typ == 1 && pat.paintType == 2 {
				ncs.base, ncs.n = cs, cs.n
			}
			cs = ncs
		}
		return m.setPatternColor(cs)
	})
	in.Define("shfill", func(*interp.Interp) error {
		o, err := in.Peek(0)
		if err != nil {
			return err
		}
		s, err := m.parseShading(o)
		if err != nil {
			return err
		}
		_, _ = in.Pop()
		return m.shfill(s)
	})
	in.Define("execform", func(*interp.Interp) error {
		fo, err := in.PopType(interp.TDict)
		if err != nil {
			return err
		}
		return m.execForm(fo.Dict(), fo)
	})
}
