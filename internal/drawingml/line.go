package drawingml

import (
	"math"

	"github.com/xo/okage"
	"github.com/xo/okage/path"
	"github.com/xo/okage/roxmltree"

	"github.com/xo/ivy/internal/display"
)

// Line is the outline of a shape or of a connector, from a:ln.
type Line struct {
	Paint Paint
	// Width is the width in points. Zero draws the thinnest line.
	Width float64
	// Dash is the pattern of the dashes, in multiples of the width, or nil
	// for a solid line.
	Dash []float64
	Cap  okage.LineCap
	Join okage.LineJoin
	// Compound is the kind of a compound line, such as "dbl", which draws
	// as two or three lines in the width.
	Compound string
	// Head and Tail are the ends of the line.
	Head, Tail Arrow
}

// Arrow is an end of a line, from a:headEnd or a:tailEnd.
type Arrow struct {
	// Type is "none", "triangle", "stealth", "diamond", "oval" or "arrow".
	Type string
	// W and L are the width and the length, from 0 for small to 2 for
	// large.
	W, L int
}

// DefaultLine is a line without paint, with the width of 0.75 points that
// PowerPoint gives a line without a width.
func DefaultLine() Line {
	return Line{Paint: Paint{Kind: PaintNone}, Width: 0.75, Cap: okage.LineCapButt, Join: okage.LineJoinRound}
}

// presetDashes holds the dashes of a:prstDash, in multiples of the width,
// as LibreOffice draws them.
var presetDashes = map[string][]float64{
	"dot":           {1, 3},
	"sysDot":        {1, 1},
	"dash":          {4, 3},
	"sysDash":       {3, 1},
	"lgDash":        {8, 3},
	"dashDot":       {4, 3, 1, 3},
	"sysDashDot":    {3, 1, 1, 1},
	"lgDashDot":     {8, 3, 1, 3},
	"lgDashDotDot":  {8, 3, 1, 3, 1, 3},
	"sysDashDotDot": {3, 1, 1, 1, 1, 1},
}

// ParseLine reads the element n, a:ln, over the line base, and returns the
// line. An attribute or a child that n does not give keeps the value of
// base, so that a line of a placeholder or of a style can be the base.
func (c *Colors) ParseLine(n roxmltree.Node, base Line, pics Pictures) Line {
	l := base
	if w, ok := n.Attribute(roxmltree.ExpandedNameFromStr("w")); ok {
		if v, ok := number(w); ok && v >= 0 {
			l.Width = min(emu(v), 1000)
		}
	}
	switch attr(n, "cap", "") {
	case "rnd":
		l.Cap = okage.LineCapRound
	case "sq":
		l.Cap = okage.LineCapSquare
	case "flat":
		l.Cap = okage.LineCapButt
	}
	if s := attr(n, "cmpd", ""); s != "" {
		l.Compound = s
	}
	for e := range n.Children() {
		if !e.IsElement() {
			continue
		}
		if pt, ok := c.PaintOf(e, pics); ok {
			l.Paint = pt
			continue
		}
		switch e.TagName().Name() {
		case "prstDash":
			l.Dash = presetDashes[attr(e, "val", "solid")]
		case "custDash":
			var dash []float64
			for ds := range e.Children() {
				if !ds.IsElement() || ds.TagName().Name() != "ds" || len(dash) >= 64 {
					continue
				}
				dash = append(dash, max(pct(numberAttr(ds, "d", 0)), 0), max(pct(numberAttr(ds, "sp", 0)), 0))
			}
			l.Dash = dash
		case "round":
			l.Join = okage.LineJoinRound
		case "bevel":
			l.Join = okage.LineJoinBevel
		case "miter":
			l.Join = okage.LineJoinMiter
		case "headEnd":
			l.Head = parseArrow(e)
		case "tailEnd":
			l.Tail = parseArrow(e)
		}
	}
	return l
}

// parseArrow reads a:headEnd or a:tailEnd.
func parseArrow(n roxmltree.Node) Arrow {
	size := func(name string) int {
		switch attr(n, name, "med") {
		case "sm":
			return 0
		case "lg":
			return 2
		}
		return 1
	}
	return Arrow{Type: attr(n, "type", "none"), W: size("w"), L: size("len")}
}

// Visible reports whether the line draws anything.
func (l *Line) Visible() bool {
	return l.Paint.Visible()
}

// stroke returns the stroke of okage for the line.
func (l *Line) stroke() okage.Stroke {
	s := okage.DefaultStroke()
	s.Width = float32(l.Width)
	s.LineCap = l.Cap
	s.LineJoin = l.Join
	s.MiterLimit = 8
	if len(l.Dash) >= 2 {
		w := max(l.Width, 0.75)
		dash := make([]float32, 0, len(l.Dash)+1)
		for _, d := range l.Dash {
			dash = append(dash, float32(max(d*w, 0.1)))
		}
		if len(dash)%2 == 1 {
			dash = append(dash, dash...)
		}
		if d, ok := okage.NewStrokeDash(dash, 0); ok {
			s.Dash = d
		}
	}
	return s
}

// StrokePath strokes the path p, in the user space of dev, with the line,
// and draws its arrows. box is the box of the shape, over which a gradient
// stretches.
func (l *Line) StrokePath(dev display.Device, p *okage.Path, box okage.Rect) {
	if p == nil || !l.Visible() {
		return
	}
	sh, ok := l.shader(box)
	if !ok {
		return
	}
	if bands := compoundBands[l.Compound]; bands != nil && l.Width > 0 {
		l.compound(dev, p, bands, sh)
	} else {
		dev.StrokePath(p, l.stroke(), sh)
	}
	l.arrows(dev, p, sh)
}

// compoundBands holds the lines of each kind of a compound line (ECMA-376
// Part 1, 20.1.10.15), as fractions of the width from one side of the line
// to the other: the start and the end of each line. The lines are
// symmetric about the middle of the line. A thick line and a thin line
// (thickThin and thinThick) draw as two lines of 2/5 and 1/5 of the width
// on each side, not as one thick line on one side and a thin line on the
// other, because the side of a stroke is not known.
var compoundBands = map[string][]float64{
	"dbl":       {0, 1.0 / 3, 2.0 / 3, 1},
	"tri":       {0, 1.0 / 6, 2.0 / 6, 4.0 / 6, 5.0 / 6, 1},
	"thickThin": {0, 2.0 / 5, 3.0 / 5, 1},
	"thinThick": {0, 2.0 / 5, 3.0 / 5, 1},
}

// compound strokes p as the lines of a compound line. Each line of a
// symmetric pair is the stroke of its outer width less the stroke of its
// inner width, which a group erases.
func (l *Line) compound(dev display.Device, p *okage.Path, bands []float64, sh okage.Shader) {
	st := l.stroke()
	width := st.Width
	for i := 0; i+1 < len(bands); i += 2 {
		a, c := bands[i], bands[i+1]
		if a >= 0.5 {
			break
		}
		outer := float32(1-2*a) * width
		inner := float32(0)
		if c < 0.5 {
			inner = float32(1-2*c) * width
		}
		dev.BeginGroup(display.Group{Opacity: 1, Isolated: true})
		st.Width = outer
		dev.StrokePath(p, st, sh)
		if inner > 0 {
			dev.BeginGroup(display.Group{Opacity: 1, Blend: okage.BlendModeDestinationOut})
			st.Width = inner
			dev.StrokePath(p, st, okage.SolidColor(okage.ColorBlack))
			dev.EndGroup()
		}
		dev.EndGroup()
	}
}

// shader returns the shader of the paint of the line. A picture draws as
// its mean color, gray.
func (l *Line) shader(box okage.Rect) (okage.Shader, bool) {
	switch l.Paint.Kind {
	case PaintSolid:
		return okage.SolidColor(l.Paint.Color.Okage()), true
	case PaintGradient:
		return l.Paint.Gradient.shader(box)
	case PaintPattern:
		return l.Paint.patternShader(), true
	case PaintPicture:
		return okage.SolidColor(RGBA{0.5, 0.5, 0.5, 1}.Okage()), true
	}
	return nil, false
}

// arrows draws the arrows at the ends of the path p: the head at its first
// point and the tail at its last point.
func (l *Line) arrows(dev display.Device, p *okage.Path, sh okage.Shader) {
	if l.Head.Type == "none" && l.Tail.Type == "none" || l.Head.Type == "" && l.Tail.Type == "" {
		return
	}
	pts := p.Points()
	if len(pts) < 2 {
		return
	}
	// a closed path, such as the outline of a rectangle, has no ends
	if verbs := p.Verbs(); len(verbs) > 0 && verbs[len(verbs)-1] == path.PathVerbClose {
		return
	}
	if a := l.Head; a.Type != "" && a.Type != "none" {
		tip, from := pts[0], pts[1]
		for i := 1; i < len(pts) && samePoint(tip, from); i++ {
			from = pts[i]
		}
		l.arrow(dev, a, tip, from, sh)
	}
	if a := l.Tail; a.Type != "" && a.Type != "none" {
		tip, from := pts[len(pts)-1], pts[len(pts)-2]
		for i := len(pts) - 2; i >= 0 && samePoint(tip, from); i-- {
			from = pts[i]
		}
		l.arrow(dev, a, tip, from, sh)
	}
}

// samePoint reports whether two points are the same, for the direction of an
// arrow.
func samePoint(a, b okage.Point) bool {
	return math.Abs(float64(a.X-b.X)) < 1e-4 && math.Abs(float64(a.Y-b.Y)) < 1e-4
}

// arrow draws the arrow a at the point tip, which points away from the
// point from.
func (l *Line) arrow(dev display.Device, a Arrow, tip, from okage.Point, sh okage.Shader) {
	w := max(l.Width, 0.75)
	factor := [...]float64{2, 3, 5}
	aw, al := w*factor[a.W], w*factor[a.L]
	dx, dy := float64(tip.X-from.X), float64(tip.Y-from.Y)
	d := math.Hypot(dx, dy)
	if d == 0 {
		return
	}
	ux, uy := dx/d, dy/d
	// the point at the distance along back from the tip, and across to the
	// left
	at := func(along, across float64) (float32, float32) {
		return float32(float64(tip.X) - ux*along - uy*across), float32(float64(tip.Y) - uy*along + ux*across)
	}
	pb := okage.NewPathBuilder()
	fill := true
	switch a.Type {
	case "triangle":
		pb.MoveTo(at(0, 0))
		pb.LineTo(at(al, aw/2))
		pb.LineTo(at(al, -aw/2))
		pb.Close()
	case "stealth":
		pb.MoveTo(at(0, 0))
		pb.LineTo(at(al, aw/2))
		pb.LineTo(at(al*0.6, 0))
		pb.LineTo(at(al, -aw/2))
		pb.Close()
	case "diamond":
		// a diamond and an oval are centered on the end
		pb.MoveTo(at(-al/2, 0))
		pb.LineTo(at(0, aw/2))
		pb.LineTo(at(al/2, 0))
		pb.LineTo(at(0, -aw/2))
		pb.Close()
	case "oval":
		r, ok := okage.RectFromXYWH(tip.X-float32(al/2), tip.Y-float32(aw/2), float32(al), float32(aw))
		if !ok {
			return
		}
		if p, ok := okage.PathFromOval(r); ok {
			dev.FillPath(p, okage.FillRuleWinding, sh)
		}
		return
	case "arrow":
		pb.MoveTo(at(al, aw/2))
		pb.LineTo(at(0, 0))
		pb.LineTo(at(al, -aw/2))
		fill = false
	default:
		return
	}
	p, ok := pb.Finish()
	if !ok {
		return
	}
	if fill {
		dev.FillPath(p, okage.FillRuleWinding, sh)
		return
	}
	s := okage.DefaultStroke()
	s.Width = float32(w)
	s.LineJoin = okage.LineJoinMiter
	dev.StrokePath(p, s, sh)
}
