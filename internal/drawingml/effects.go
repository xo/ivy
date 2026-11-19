package drawingml

import (
	"math"

	"github.com/xo/okage"
	"github.com/xo/okage/roxmltree"

	"github.com/xo/ivy/internal/display"
)

// Effects are the effects of a shape that a:effectLst gives (ECMA-376
// Part 1, 20.1.8.26). The blur, the fill overlay and the preset shadow of
// the list are left out (D5).
type Effects struct {
	// Shadow is the outer shadow, or nil.
	Shadow *Shadow
	// InnerShadow is the inner shadow, or nil. Its scale is not used.
	InnerShadow *Shadow
	// Glow is the glow, or nil.
	Glow *Glow
	// SoftEdge is the radius of the soft edge, in points, or 0.
	SoftEdge float64
	// Reflection is the reflection, or nil.
	Reflection *Reflection
}

// Glow is the glow of a shape (a:glow): its color spreads out of the
// outline of the shape and fades out at the radius.
type Glow struct {
	Color RGBA
	// Rad is the radius, in points.
	Rad float64
}

// Reflection is the reflection of a shape (a:reflection): a copy of the
// shape and its text, scaled about a point of its box, often upside down
// below it, that fades out. The blur of the reflection is left out, and the
// reflection turns with the shape whatever rotWithShape says.
type Reflection struct {
	// StartA and EndA are the alphas of the start and the end of the fade,
	// from 0 to 1. StartPos and EndPos are their places, as fractions of
	// the reflection along the direction of the fade.
	StartA, StartPos, EndA, EndPos float64
	// FadeDir is the direction of the fade, in degrees clockwise from the
	// x axis.
	FadeDir float64
	// DX and DY are the offset of the reflection, in points.
	DX, DY float64
	// SX and SY scale the reflection, from 1. A negative SY turns it upside
	// down.
	SX, SY float64
	// KX and KY skew the reflection, in degrees.
	KX, KY float64
	// Align is the point of the box of the shape that the scale keeps, as
	// the attribute algn gives it, such as "b" or "bl".
	Align string
}

// maxEffectRadius is the largest radius of an effect, in points.
const maxEffectRadius = 1000

// ParseEffects reads the effects of the element n, an a:effectLst or an
// a:effectStyle of the theme. It returns false if n holds no list of
// effects. It logs nothing: a format logs the effects that it leaves out
// with LeftOut.
func (c *Colors) ParseEffects(n roxmltree.Node) (Effects, bool) {
	if n.TagName().Name() == "effectStyle" {
		e, ok := child(n, "effectLst")
		if !ok {
			return Effects{}, false
		}
		n = e
	}
	if n.TagName().Name() != "effectLst" {
		return Effects{}, false
	}
	var fx Effects
	if e, ok := child(n, "outerShdw"); ok {
		fx.Shadow = c.shadow(e)
		if fx.Shadow != nil {
			fx.Shadow.SX = pct(numberAttr(e, "sx", 100000))
			fx.Shadow.SY = pct(numberAttr(e, "sy", 100000))
		}
	}
	if e, ok := child(n, "innerShdw"); ok {
		fx.InnerShadow = c.shadow(e)
	}
	if e, ok := child(n, "glow"); ok {
		col, ok := c.In(e)
		if rad := radius(e, "rad"); ok && rad > 0 && col.A > 0 {
			fx.Glow = &Glow{Color: col, Rad: rad}
		}
	}
	if e, ok := child(n, "softEdge"); ok {
		fx.SoftEdge = radius(e, "rad")
	}
	if e, ok := child(n, "reflection"); ok {
		fx.Reflection = parseReflection(e)
	}
	return fx, true
}

// radius returns the length of the attribute name of n, in points, from 0
// to maxEffectRadius.
func radius(n roxmltree.Node, name string) float64 {
	v := emu(numberAttr(n, name, 0))
	if math.IsNaN(v) {
		return 0
	}
	return max(0, min(v, maxEffectRadius))
}

// shadow reads a:outerShdw or a:innerShdw: its color, its offset and its
// blur. It returns nil for an offset that is not finite or too large.
func (c *Colors) shadow(e roxmltree.Node) *Shadow {
	col, ok := c.In(e)
	if !ok {
		col = Black.WithAlpha(0.5)
	}
	dist := emu(numberAttr(e, "dist", 0))
	dir := numberAttr(e, "dir", 0) / 60000 * math.Pi / 180
	s := &Shadow{
		Color: col,
		DX:    dist * math.Cos(dir),
		DY:    dist * math.Sin(dir),
		Blur:  radius(e, "blurRad"),
		SX:    1,
		SY:    1,
	}
	if math.IsNaN(s.DX) || math.IsNaN(s.DY) || math.Abs(s.DX) > 1e4 || math.Abs(s.DY) > 1e4 {
		return nil
	}
	return s
}

// parseReflection reads a:reflection, with the defaults of ECMA-376 Part
// 1, 20.1.8.50. It returns nil for a reflection that does not show.
func parseReflection(e roxmltree.Node) *Reflection {
	dist := emu(numberAttr(e, "dist", 0))
	dir := numberAttr(e, "dir", 0) / 60000 * math.Pi / 180
	r := &Reflection{
		StartA:   unit(pct(numberAttr(e, "stA", 100000))),
		StartPos: unit(pct(numberAttr(e, "stPos", 0))),
		EndA:     unit(pct(numberAttr(e, "endA", 0))),
		EndPos:   unit(pct(numberAttr(e, "endPos", 100000))),
		FadeDir:  numberAttr(e, "fadeDir", 5400000) / 60000,
		DX:       dist * math.Cos(dir),
		DY:       dist * math.Sin(dir),
		SX:       pct(numberAttr(e, "sx", 100000)),
		SY:       pct(numberAttr(e, "sy", 100000)),
		KX:       numberAttr(e, "kx", 0) / 60000,
		KY:       numberAttr(e, "ky", 0) / 60000,
		Align:    attr(e, "algn", "b"),
	}
	for _, v := range [...]float64{r.DX, r.DY, r.SX, r.SY, r.KX, r.KY, r.FadeDir} {
		if math.IsNaN(v) || math.Abs(v) > 1e4 {
			return nil
		}
	}
	if r.StartA == 0 && r.EndA == 0 || r.SX == 0 || r.SY == 0 {
		return nil
	}
	return r
}

// LeftOut returns the names of the effects of the a:effectLst n that a
// Figure leaves out, for the log.
func LeftOut(n roxmltree.Node) []string {
	var names []string
	for e := range n.Children() {
		if !e.IsElement() {
			continue
		}
		switch name := e.TagName().Name(); name {
		case "outerShdw", "innerShdw", "glow", "softEdge", "reflection":
		default:
			names = append(names, name)
		}
	}
	return names
}

// SetEffects gives the figure the effects fx.
func (f *Figure) SetEffects(fx Effects) {
	f.Shadow = fx.Shadow
	f.InnerShadow = fx.InnerShadow
	f.Glow = fx.Glow
	f.SoftEdge = fx.SoftEdge
	f.Reflection = fx.Reflection
}

// figureSilhouette returns the silhouette of the paths of f that draw:
// those that fill when fill is set, and those that stroke when line is
// set.
func figureSilhouette(f *Figure, fill, line bool, scale float64) (silhouette, bool) {
	var shape []*okage.Path
	var stroke []bool
	for _, p := range f.Paths {
		switch {
		case fill && f.Fill.Visible() && p.Fill != FillNone:
			shape, stroke = append(shape, p.Path), append(stroke, false)
		case line && f.Line.Visible() && p.Stroke:
			shape, stroke = append(shape, p.Path), append(stroke, true)
		}
	}
	return newSilhouette(shape, stroke, f.Line.stroke(), scale)
}

// drawGlow fills the glow of the figure f onto dev, in the box of the
// figure. The glow is the silhouette of the figure grown by half the radius,
// blurred with a standard deviation of a sixth of the radius. So it is full
// near the outline, half at half the radius, and it fades out at the
// radius, as PowerPoint 2024 draws it.
func drawGlow(dev display.Device, f *Figure, scale float64) {
	g := f.Glow
	s, ok := figureSilhouette(f, true, true, scale)
	if !ok {
		return
	}
	fr, ok := s.frame(g.Rad*1.5, scale)
	if !ok {
		return
	}
	alpha := s.mask(fr, 0, 0, g.Rad/2, g.Rad/6)
	if alpha == nil {
		return
	}
	dev.FillMask(alpha, fr.toUser(), display.ImageOptions{Opacity: 1, Smooth: true}, okage.SolidColor(g.Color.Okage()))
}

// drawInnerShadow fills the inner shadow of the figure f onto dev, in the
// box of the figure: the inside of the fill that the fill moved against the
// offset of the shadow, blurred, does not cover. So the shadow lies along
// the edges that the offset points to, as PowerPoint 2024 draws it. The
// offset is in the space of the scene, which m maps the box onto.
func drawInnerShadow(dev display.Device, f *Figure, m okage.Transform, scale float64) {
	sh := f.InnerShadow
	s, ok := figureSilhouette(f, true, false, scale)
	if !ok {
		return
	}
	// the offset in the box of the shape
	inv, ok := m.Invert()
	if !ok {
		return
	}
	inv.TX, inv.TY = 0, 0
	d := okage.Point{X: float32(sh.DX), Y: float32(sh.DY)}
	inv.MapPoint(&d)
	dx, dy := float64(d.X), float64(d.Y)
	if math.IsNaN(dx) || math.IsNaN(dy) || math.IsInf(dx, 0) || math.IsInf(dy, 0) {
		return
	}
	fr, ok := s.frame(sh.Blur*1.5+math.Max(math.Abs(dx), math.Abs(dy)), scale)
	if !ok {
		return
	}
	inside := s.mask(fr, 0, 0, 0, 0)
	moved := s.mask(fr, -dx, -dy, 0, sh.Blur/2)
	if inside == nil || moved == nil {
		return
	}
	for i, v := range inside.Pix {
		inside.Pix[i] = uint8((int(v)*(255-int(moved.Pix[i])) + 127) / 255)
	}
	dev.FillMask(inside, fr.toUser(), display.ImageOptions{Opacity: 1, Smooth: true}, okage.SolidColor(sh.Color.Okage()))
}

// softEdgeMask returns the soft mask of the soft edge of the figure f, in
// the box of the figure, or nil. The mask is the silhouette of the figure
// made smaller by 0.8 times the radius, and blurred with a standard
// deviation of a third of the radius. So it is empty at the outline, half
// at 0.8 times the radius inside it, and full a little past the radius, as
// PowerPoint 2024 draws it.
func softEdgeMask(f *Figure, scale float64) *display.Mask {
	s, ok := figureSilhouette(f, true, true, scale)
	if !ok {
		return nil
	}
	fr, ok := s.frame(f.SoftEdge, scale)
	if !ok {
		return nil
	}
	alpha := s.mask(fr, 0, 0, -0.8*f.SoftEdge, f.SoftEdge/3)
	if alpha == nil {
		return nil
	}
	t := fr.toUser()
	return &display.Mask{
		Type: okage.MaskTypeAlpha,
		Content: func(d display.Device) {
			d.FillMask(alpha, t, display.ImageOptions{Opacity: 1, Smooth: true}, okage.SolidColor(okage.ColorWhite))
		},
	}
}

// transform returns the transform of the reflection r in the box of a shape
// of w by h points: the scale and the skew about the point of the box of
// Align.
func (r *Reflection) transform(w, h float64) okage.Transform {
	ax, ay := w/2, h/2
	switch r.Align {
	case "tl", "l", "bl":
		ax = 0
	case "tr", "r", "br":
		ax = w
	}
	switch r.Align {
	case "tl", "t", "tr":
		ay = 0
	case "bl", "b", "br":
		ay = h
	}
	// a skew of 90 degrees has no tangent
	kx := math.Tan(max(-80, min(r.KX, 80)) * math.Pi / 180)
	ky := math.Tan(max(-80, min(r.KY, 80)) * math.Pi / 180)
	m := okage.TransformFromRow(float32(r.SX), float32(ky*r.SY), float32(kx*r.SX), float32(r.SY), 0, 0)
	return okage.TransformFromTranslate(float32(ax), float32(ay)).PreConcat(m).PreTranslate(float32(-ax), float32(-ay))
}

// fade returns the shader of the fade of the reflection r, in the box of the
// shape of w by h points, after the transform of the reflection: white,
// with the alphas of r along the direction of the fade over the box of the
// reflection. It returns false for a fade that okage cannot make.
func (r *Reflection) fade(w, h float64) (okage.Shader, okage.Rect, bool) {
	pts := []okage.Point{{X: 0, Y: 0}, {X: float32(w), Y: 0}, {X: float32(w), Y: float32(h)}, {X: 0, Y: float32(h)}}
	r.transform(w, h).MapPoints(pts)
	minX, minY := math.Inf(1), math.Inf(1)
	maxX, maxY := math.Inf(-1), math.Inf(-1)
	for _, p := range pts {
		minX, maxX = math.Min(minX, float64(p.X)), math.Max(maxX, float64(p.X))
		minY, maxY = math.Min(minY, float64(p.Y)), math.Max(maxY, float64(p.Y))
	}
	box, ok := rectLTRB(minX, minY, maxX, maxY)
	if !ok {
		return nil, okage.Rect{}, false
	}
	// the fade runs over the box along its direction, from the corner that
	// the direction leaves to the corner that it reaches
	a := r.FadeDir * math.Pi / 180
	ux, uy := math.Cos(a), math.Sin(a)
	cx, cy := (minX+maxX)/2, (minY+maxY)/2
	half := (math.Abs(ux)*(maxX-minX) + math.Abs(uy)*(maxY-minY)) / 2
	p0 := okage.Point{X: float32(cx - ux*half), Y: float32(cy - uy*half)}
	p1 := okage.Point{X: float32(cx + ux*half), Y: float32(cy + uy*half)}
	white := func(a float64) okage.Color {
		return okage.ColorFromRGBAUnchecked(1, 1, 1, float32(unit(a)))
	}
	stops := []okage.GradientStop{
		okage.NewGradientStop(float32(r.StartPos), white(r.StartA)),
		okage.NewGradientStop(float32(max(r.EndPos, r.StartPos)), white(r.EndA)),
	}
	sh, ok := okage.NewLinearGradient(p0, p1, stops, okage.SpreadModePad, okage.IdentityTransform())
	if !ok {
		// a fade of no length is one alpha
		sh, ok = okage.SolidColor(white(r.StartA)), true
	}
	return sh, box, ok
}

// reflect wraps the operations ops of the figure f, which draw the shape and
// its text, into a group that draws them again as the reflection of the
// shape, with the fade as its soft mask. It returns the operations of the
// reflection.
func (s *Scene) reflect(f *Figure, ops []sceneOp) []sceneOp {
	r := f.Reflection
	shader, box, ok := r.fade(f.W, f.H)
	if !ok {
		return nil
	}
	// the mask covers the box of the reflection and the lines around it
	pad := float32(max(f.W, f.H))
	area, ok := okage.RectFromLTRB(box.Left()-pad, box.Top()-pad, box.Right()+pad, box.Bottom()+pad)
	if !ok {
		return nil
	}
	place := okage.TransformFromTranslate(float32(r.DX), float32(r.DY)).PreConcat(f.Transform)
	inv, ok := f.Transform.Invert()
	if !ok {
		return nil
	}
	// m maps the shape onto its reflection, in the space of the scene
	m := place.PreConcat(r.transform(f.W, f.H)).PreConcat(inv)
	mask := &display.Mask{
		Type: okage.MaskTypeAlpha,
		Content: func(d display.Device) {
			d.Save()
			d.Concat(place)
			d.FillPath(okage.PathFromRect(area), okage.FillRuleWinding, shader)
			d.Restore()
		},
	}
	out := make([]sceneOp, 0, len(ops)+2)
	out = append(out, func(c *Canvas) error {
		c.dev.Save()
		c.dev.BeginGroup(display.Group{Opacity: 1, Isolated: true, Mask: mask})
		c.dev.Concat(m)
		return nil
	})
	out = append(out, ops...)
	return append(out, func(c *Canvas) error {
		c.dev.EndGroup()
		c.dev.Restore()
		return nil
	})
}
