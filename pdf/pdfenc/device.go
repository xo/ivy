package pdfenc

import (
	"image"
	"math"
	"reflect"

	"github.com/xo/okage"
	"github.com/xo/okage/path"
	"github.com/xo/okage/pdf"

	"github.com/xo/ivy/internal/display"
)

// maxCoordinate is the largest magnitude of a coordinate of a path, in its
// user space or on the page, that the device writes. The writer of okage
// records an error for a stroke past 2^60, and a viewer loses the precision
// of a path long before.
const maxCoordinate = 1 << 40

// maxDashes is the largest number of dashes of a stroke. okage, as
// tiny-skia, draws no stroke with more dashes, so the device writes none.
const maxDashes = 1000000

// The bytes that the device counts for the commands of a path, for each
// point and for each command.
const (
	pointBytes   = 24
	commandBytes = 64
)

// device maps the commands of the display list onto a surface of the PDF
// writer of okage (D58). It draws with okage, through Surface.DrawRaster, a
// part that PDF cannot hold.
type device struct {
	enc *encoder
	s   *pdf.Surface
	// page is the page of the file, or nil for the device of a soft mask.
	// index is the index of the page in the file, from 0, and w and h are
	// its size in points.
	page  *pdf.Page
	index int
	w, h  float32
	// res is the resolution of the raster fallback, in pixels for each
	// point.
	res float32

	state  state
	stack  []state
	groups []frame
	// pushes is the number of the pushes of the surface that are not
	// popped.
	pushes int

	// rec records the content of a group that the device draws with the
	// raster fallback, from its BeginGroup to its EndGroup, or is nil. depth
	// counts the groups that begin inside it.
	rec      *display.List
	recGroup display.Group
	depth    int
	// level is the number of the groups around the device: the groups
	// around the soft mask that it draws.
	level int
}

// state is the part of the device that Save pushes.
type state struct {
	// pushes is the number of the pushes of the surface at the Save.
	pushes int
	// clip holds the bounds of the clip on the page.
	clip okage.Rect
	// hidden is true when the clip is empty, or when the transform cannot
	// draw, so that the device draws nothing.
	hidden bool
}

// frame is a group that BeginGroup started.
type frame struct {
	// depth is the length of the stack at BeginGroup. A Restore inside the
	// group does not pop below it.
	depth int
	// state is the state at BeginGroup, which EndGroup restores. Its
	// pushes are the pushes before the group of the surface.
	state state
}

// The interfaces that a device implements.
var (
	_ display.Device         = (*device)(nil)
	_ display.TextFiller     = (*device)(nil)
	_ display.StrokeAdjuster = (*device)(nil)
)

// newDevice returns a device that draws onto s, the surface of page, the
// page of index n in the file, of w by h points.
func newDevice(enc *encoder, page *pdf.Page, s *pdf.Surface, n int, w, h float32) *device {
	clip, ok := okage.RectFromXYWH(0, 0, w, h)
	return &device{
		enc:   enc,
		s:     s,
		page:  page,
		index: n,
		w:     w,
		h:     h,
		res:   enc.o.dpi / 72,
		state: state{clip: clip, hidden: !ok},
	}
}

// subDevice returns a device that draws onto s, the surface of a soft mask,
// with the clip of d.
func (d *device) subDevice(s *pdf.Surface) *device {
	return &device{enc: d.enc, s: s, res: d.res, state: state{clip: d.state.clip}}
}

// finish ends the groups that are open and pops each push, so that the
// surface can finish.
func (d *device) finish() {
	if d.rec != nil {
		d.endRecording(d.depth + 1)
	}
	for len(d.groups) > 0 {
		d.EndGroup()
	}
	d.popTo(0)
}

// popTo pops the pushes of the surface until n are left.
func (d *device) popTo(n int) {
	for d.pushes > n {
		d.s.Pop()
		d.pushes--
	}
}

// push counts a push of the surface.
func (d *device) push() {
	d.pushes++
}

// skip reports whether the device draws nothing for a command: after an
// error of a limit, or when the state hides what it draws.
func (d *device) skip() bool {
	return d.enc.err != nil || d.state.hidden
}

// Save satisfies the display.Device interface.
func (d *device) Save() {
	if d.rec != nil {
		d.rec.Save()
		return
	}
	st := d.state
	st.pushes = d.pushes
	d.stack = append(d.stack, st)
}

// Restore satisfies the display.Device interface. A Restore without a Save
// does nothing, and the content of a group cannot restore a state from
// before the group, as in display.OkageDevice.
func (d *device) Restore() {
	if d.rec != nil {
		d.rec.Restore()
		return
	}
	if len(d.stack) == 0 {
		return
	}
	if n := len(d.groups); n > 0 && len(d.stack) <= d.groups[n-1].depth {
		return
	}
	st := d.stack[len(d.stack)-1]
	d.stack = d.stack[:len(d.stack)-1]
	d.popTo(st.pushes)
	d.state = st
}

// Concat satisfies the display.Device interface. A transform that is not
// finite, or that cannot be inverted, hides what follows until the Restore.
func (d *device) Concat(t okage.Transform) {
	if d.rec != nil {
		d.rec.Concat(t)
		return
	}
	if d.state.hidden || t.IsIdentity() {
		return
	}
	if _, ok := t.Invert(); !ok || !t.IsFinite() {
		d.state.hidden = true
		return
	}
	if ctm := d.s.CTM().PreConcat(t); !ctm.IsFinite() || !finiteScale(ctm) {
		d.state.hidden = true
		return
	}
	d.s.PushTransform(t)
	d.push()
}

// finiteScale reports whether the scale of t is within maxCoordinate.
func finiteScale(t okage.Transform) bool {
	for _, v := range []float32{t.SX, t.KX, t.KY, t.SY, t.TX, t.TY} {
		if !(math.Abs(float64(v)) < maxCoordinate) {
			return false
		}
	}
	return true
}

// ClipPath satisfies the display.Device interface. An empty path clips
// everything.
func (d *device) ClipPath(p *okage.Path, rule okage.FillRule) {
	if d.rec != nil {
		d.rec.ClipPath(p, rule)
		return
	}
	if d.state.hidden {
		return
	}
	if p == nil {
		d.state.hidden = true
		return
	}
	b, ok := pageBounds(p.Bounds(), d.s.CTM())
	if !ok {
		// a clip that the page cannot hold covers more than the page
		return
	}
	clip, ok := d.state.clip.Intersect(b)
	if !ok || clip.Width() <= 0 || clip.Height() <= 0 {
		d.state.hidden = true
		return
	}
	if !d.enc.charge(pathBytes(p)) {
		return
	}
	d.state.clip = clip
	d.s.PushClipPath(p, fillRule(rule))
	d.push()
}

// pageBounds returns r, in a user space, on the page that t maps it onto.
// It returns false when the page cannot hold it.
func pageBounds(r okage.Rect, t okage.Transform) (okage.Rect, bool) {
	if !inRange(r) {
		return okage.Rect{}, false
	}
	b, ok := r.Transform(t)
	if !ok || !inRange(b) {
		return okage.Rect{}, false
	}
	return b, true
}

// inRange reports whether each side of r is within maxCoordinate.
func inRange(r okage.Rect) bool {
	return math.Abs(float64(r.Left())) < maxCoordinate && math.Abs(float64(r.Top())) < maxCoordinate &&
		math.Abs(float64(r.Right())) < maxCoordinate && math.Abs(float64(r.Bottom())) < maxCoordinate
}

// pathBytes returns the bytes that the commands of p take in the file,
// about.
func pathBytes(p *okage.Path) int64 {
	return int64(len(p.Points()))*pointBytes + commandBytes
}

// fillRule returns the fill rule of PDF for rule.
func fillRule(rule okage.FillRule) pdf.FillRule {
	if rule == okage.FillRuleEvenOdd {
		return pdf.FillRuleEvenOdd
	}
	return pdf.FillRuleNonZero
}

// FillPath satisfies the display.Device interface.
func (d *device) FillPath(p *okage.Path, rule okage.FillRule, shader okage.Shader) {
	if d.rec != nil {
		d.rec.FillPath(p, rule, shader)
		return
	}
	d.fill(p, rule, shader)
}

// FillText satisfies the display.TextFiller interface. The outlines of the
// glyphs fill as a path.
func (d *device) FillText(p *okage.Path, rule okage.FillRule, shader okage.Shader) {
	if d.rec != nil {
		d.rec.FillText(p, rule, shader)
		return
	}
	d.fill(p, rule, shader)
}

// FillGlyphs satisfies the display.Device interface. A run with a text
// fills as text of an embedded font, where the font allows it (D58). Other
// runs fill as paths, with the nonzero rule.
func (d *device) FillGlyphs(run display.GlyphRun, shader okage.Shader) {
	if d.rec != nil {
		d.rec.FillGlyphs(run, shader)
		return
	}
	if d.skip() || run.Face == nil || shader == nil {
		return
	}
	if d.text(run, shader) {
		return
	}
	d.fill(run.Path(), okage.FillRuleWinding, shader)
}

// fill fills p with the shader.
func (d *device) fill(p *okage.Path, rule okage.FillRule, shader okage.Shader) {
	if d.skip() || p == nil || shader == nil {
		return
	}
	if _, ok := pageBounds(p.Bounds(), d.s.CTM()); !ok {
		return
	}
	if !d.enc.charge(pathBytes(p)) {
		return
	}
	paint, opacity, ok := d.paint(shader)
	if !ok {
		d.raster(p.Bounds(), func(dev *display.OkageDevice) { dev.FillPath(p, rule, shader) })
		return
	}
	if opacity == 0 {
		return
	}
	d.s.SetStroke(nil)
	d.s.SetFill(&pdf.Fill{Paint: paint, Opacity: opacity, Rule: fillRule(rule)})
	d.s.DrawPath(p)
	d.s.SetFill(nil)
}

// paint converts the shader, in the current user space, into a paint of
// the surface. It returns false for a shader that a paint cannot hold, or
// that the writer refuses.
func (d *device) paint(shader okage.Shader) (pdf.Paint, pdf.NormalizedF32, bool) {
	paint, opacity, ok := d.s.PaintFromShader(shader)
	if !ok {
		return nil, 0, false
	}
	var t okage.Transform
	switch g := paint.(type) {
	case pdf.LinearGradient:
		t = g.Transform
		if !gradientFits(len(g.Stops), g.X1, g.Y1, g.X2, g.Y2) {
			return nil, 0, false
		}
	case pdf.RadialGradient:
		t = g.Transform
		if !gradientFits(len(g.Stops), g.Fx, g.Fy, g.Fr, g.Cx, g.Cy, g.Cr) {
			return nil, 0, false
		}
	case pdf.SweepGradient:
		t = g.Transform
		if !gradientFits(len(g.Stops), g.Cx, g.Cy, g.StartAngle, g.EndAngle) {
			return nil, 0, false
		}
	case pdf.MeshGradient:
		t = g.Transform
		for _, tri := range g.Triangles {
			for _, v := range tri {
				if !finitePoint(v.Point) {
					return nil, 0, false
				}
			}
		}
	case pdf.Pattern:
		t = g.Transform
	default:
		return paint, opacity, true
	}
	// the writer inverts the transform to cover the bounds of the path
	inv, ok := t.Invert()
	if !ok || !finiteScale(t) || !finiteScale(inv) {
		return nil, 0, false
	}
	return paint, opacity, true
}

// gradientFits reports whether the writer can write a gradient of n stops
// with the parameters params: at least one stop, and each parameter within
// maxCoordinate. The writer of okage puts the functions of many stops into
// a tree that MuPDF reads (okage D69).
func gradientFits(n int, params ...float32) bool {
	if n == 0 {
		return false
	}
	for _, v := range params {
		if !(math.Abs(float64(v)) < maxCoordinate) {
			return false
		}
	}
	return true
}

// finitePoints reports whether each point of p is within maxCoordinate.
// The bounds of a path leave out a point that is NaN, and the writer of
// okage does not stroke a path with such a point.
func finitePoints(p *okage.Path) bool {
	for _, pt := range p.Points() {
		if !finitePoint(pt) {
			return false
		}
	}
	return true
}

// finitePoint reports whether p is within maxCoordinate.
func finitePoint(p okage.Point) bool {
	return math.Abs(float64(p.X)) < maxCoordinate && math.Abs(float64(p.Y)) < maxCoordinate
}

// StrokePath satisfies the display.Device interface.
func (d *device) StrokePath(p *okage.Path, stroke okage.Stroke, shader okage.Shader) {
	if d.rec != nil {
		d.rec.StrokePath(p, stroke, shader)
		return
	}
	d.stroke(p, stroke, shader, false)
}

// StrokeAdjusted satisfies the display.StrokeAdjuster interface. It strokes
// p with the entry SA of PDF.
func (d *device) StrokeAdjusted(p *okage.Path, stroke okage.Stroke, shader okage.Shader) {
	if d.rec != nil {
		d.rec.StrokeAdjusted(p, stroke, shader)
		return
	}
	d.stroke(p, stroke, shader, true)
}

// stroke strokes p with the shader, and with the stroke adjustment when
// adjust is true. The dash of the stroke is the dash of PDF, so a viewer
// draws the caps and the joins of each dash (okage D69).
func (d *device) stroke(p *okage.Path, stroke okage.Stroke, shader okage.Shader, adjust bool) {
	if d.skip() || p == nil || shader == nil {
		return
	}
	w := stroke.Width
	if !(w >= 0) || !(float64(w) < maxCoordinate) {
		return
	}
	ctm := d.s.CTM()
	pad := w/2*max(stroke.MiterLimit, math.Sqrt2) + 1
	if !(float64(pad) < maxCoordinate) {
		pad = w/2 + 1
	}
	bounds, ok := outset(p.Bounds(), pad)
	if !ok || !finitePoints(p) {
		return
	}
	if _, ok := pageBounds(bounds, ctm); !ok {
		return
	}
	var dash *pdf.StrokeDash
	if stroke.Dash != nil {
		sx, sy := ctm.GetScale()
		if !dashFits(p, stroke.Dash, max(sx, sy)*d.res) {
			return
		}
		dash = &pdf.StrokeDash{Array: stroke.Dash.Array(), Offset: stroke.Dash.Offset()}
	}
	if !d.enc.charge(pathBytes(p)) {
		return
	}
	paint, opacity, ok := d.paint(shader)
	if !ok {
		d.raster(bounds, func(dev *display.OkageDevice) {
			if adjust {
				dev.StrokeAdjusted(p, stroke, shader)
				return
			}
			dev.StrokePath(p, stroke, shader)
		})
		return
	}
	if opacity == 0 {
		return
	}
	if adjust {
		d.s.PushStrokeAdjustment(true)
	}
	d.s.SetFill(nil)
	d.s.SetStroke(&pdf.Stroke{
		Paint:      paint,
		Width:      w,
		MiterLimit: max(stroke.MiterLimit, 1),
		LineCap:    lineCap(stroke.LineCap),
		LineJoin:   lineJoin(stroke.LineJoin),
		Opacity:    opacity,
		Dash:       dash,
	})
	d.s.DrawPath(p)
	d.s.SetStroke(nil)
	if adjust {
		d.s.Pop()
	}
}

// dashFits reports whether okage draws the stroke of p with the dash, at
// the resolution scale res: when the dash gives at most maxDashes dashes.
// The length of the lines between the points of p is at least the length of
// its curves, and twice it covers the lines that close its contours. When
// that bound gives too many dashes, the dash of okage decides.
func dashFits(p *okage.Path, dash *okage.StrokeDash, res float32) bool {
	pts := p.Points()
	var length float64
	for i := 1; i < len(pts); i++ {
		length += float64(pts[i].Distance(pts[i-1]))
	}
	array := dash.Array()
	var sum float64
	for _, v := range array {
		sum += float64(v)
	}
	if 2*length*float64(len(array)/2)/sum <= maxDashes {
		return true
	}
	_, ok := p.Dash(dash, res)
	return ok
}

// outset returns r with a margin of pad on each side.
func outset(r okage.Rect, pad float32) (okage.Rect, bool) {
	return okage.RectFromLTRB(r.Left()-pad, r.Top()-pad, r.Right()+pad, r.Bottom()+pad)
}

// lineCap returns the line cap of PDF for c.
func lineCap(c okage.LineCap) pdf.LineCap {
	switch c {
	case okage.LineCapRound:
		return pdf.LineCapRound
	case okage.LineCapSquare:
		return pdf.LineCapSquare
	}
	return pdf.LineCapButt
}

// lineJoin returns the line join of PDF for j. PDF has no join that clips
// the miter, so it is a miter join.
func lineJoin(j okage.LineJoin) pdf.LineJoin {
	switch j {
	case okage.LineJoinRound:
		return pdf.LineJoinRound
	case okage.LineJoinBevel:
		return pdf.LineJoinBevel
	}
	return pdf.LineJoinMiter
}

// imageKey identifies an image that the device converted.
type imageKey struct {
	img    image.Image
	smooth bool
}

// image returns the image of the writer for img. The encoder keeps the
// image of each img that is a pointer, so that an image that many pages
// share is one object of the file.
func (d *device) image(img image.Image, smooth bool) *pdf.Image {
	key := imageKey{img: img, smooth: smooth}
	keep := reflect.ValueOf(img).Kind() == reflect.Pointer
	if keep {
		if pi, ok := d.enc.images[key]; ok {
			return pi
		}
	}
	size := img.Bounds().Size()
	if !d.enc.charge(4 * int64(size.X) * int64(size.Y)) {
		return nil
	}
	pi, err := pdf.ImageFromImage(img, smooth)
	if err != nil {
		d.enc.log.Debug("pdfenc: image left out", "size", size, "err", err)
		return nil
	}
	if keep && len(d.enc.images) < d.enc.lim.MaxObjects {
		d.enc.images[key] = pi
	}
	return pi
}

// DrawImage satisfies the display.Device interface.
func (d *device) DrawImage(img image.Image, t okage.Transform, opts display.ImageOptions) {
	if d.rec != nil {
		d.rec.DrawImage(img, t, opts)
		return
	}
	if d.skip() || img == nil || img.Bounds().Empty() || !(opts.Opacity > 0) {
		return
	}
	size := img.Bounds().Size()
	r, ok := okage.RectFromXYWH(0, 0, float32(size.X), float32(size.Y))
	if !ok || !t.IsFinite() {
		return
	}
	if _, ok := t.Invert(); !ok {
		return
	}
	if _, ok := pageBounds(r, d.s.CTM().PreConcat(t)); !ok {
		return
	}
	pi := d.image(img, opts.Smooth)
	if pi == nil {
		return
	}
	d.s.PushTransform(t)
	if opts.Opacity < 1 {
		d.s.PushOpacity(path.NormalizedF32Clamped(opts.Opacity))
	}
	d.s.DrawImage(pi, sizeOf(r))
	if opts.Opacity < 1 {
		d.s.Pop()
	}
	d.s.Pop()
}

// FillMask satisfies the display.Device interface. The surface fills the
// stencil with the paint of the shader, through FillStencil.
func (d *device) FillMask(mask *image.Alpha, t okage.Transform, opts display.ImageOptions, shader okage.Shader) {
	if d.rec != nil {
		d.rec.FillMask(mask, t, opts, shader)
		return
	}
	if d.skip() || mask == nil || shader == nil || mask.Rect.Empty() || !(opts.Opacity > 0) {
		return
	}
	size := mask.Rect.Size()
	r, ok := okage.RectFromXYWH(0, 0, float32(size.X), float32(size.Y))
	if !ok || !t.IsFinite() {
		return
	}
	inv, ok := t.Invert()
	if !ok {
		return
	}
	if _, ok := pageBounds(r, d.s.CTM().PreConcat(t)); !ok {
		return
	}
	if !d.enc.charge(int64(size.X) * int64(size.Y)) {
		return
	}
	// The shader is in the user space of the call, and the fill of the
	// stencil in the space of its pixels.
	paint, opacity, ok := d.paint(shader.Transform(inv))
	if !ok {
		bounds, ok := r.Transform(t)
		if !ok {
			return
		}
		d.raster(bounds, func(dev *display.OkageDevice) { dev.FillMask(mask, t, opts, shader) })
		return
	}
	if opacity == 0 {
		return
	}
	d.s.PushTransform(t)
	if opts.Opacity < 1 {
		d.s.PushOpacity(path.NormalizedF32Clamped(opts.Opacity))
	}
	d.s.SetStroke(nil)
	d.s.SetFill(&pdf.Fill{Paint: paint, Opacity: opacity, Rule: pdf.FillRuleNonZero})
	d.s.FillStencil(mask, sizeOf(r), opts.Smooth)
	d.s.SetFill(nil)
	if opts.Opacity < 1 {
		d.s.Pop()
	}
	d.s.Pop()
}

// blendMode returns the blend mode of PDF for b, or false for a mode that
// PDF does not have. The zero mode, okage.BlendModeClear, is the normal
// one, as in display.Group.
func blendMode(b okage.BlendMode) (pdf.BlendMode, bool) {
	switch b {
	case okage.BlendModeClear, okage.BlendModeSourceOver:
		return pdf.BlendModeNormal, true
	case okage.BlendModeMultiply:
		return pdf.BlendModeMultiply, true
	case okage.BlendModeScreen:
		return pdf.BlendModeScreen, true
	case okage.BlendModeOverlay:
		return pdf.BlendModeOverlay, true
	case okage.BlendModeDarken:
		return pdf.BlendModeDarken, true
	case okage.BlendModeLighten:
		return pdf.BlendModeLighten, true
	case okage.BlendModeColorDodge:
		return pdf.BlendModeColorDodge, true
	case okage.BlendModeColorBurn:
		return pdf.BlendModeColorBurn, true
	case okage.BlendModeHardLight:
		return pdf.BlendModeHardLight, true
	case okage.BlendModeSoftLight:
		return pdf.BlendModeSoftLight, true
	case okage.BlendModeDifference:
		return pdf.BlendModeDifference, true
	case okage.BlendModeExclusion:
		return pdf.BlendModeExclusion, true
	case okage.BlendModeHue:
		return pdf.BlendModeHue, true
	case okage.BlendModeSaturation:
		return pdf.BlendModeSaturation, true
	case okage.BlendModeColor:
		return pdf.BlendModeColor, true
	case okage.BlendModeLuminosity:
		return pdf.BlendModeLuminosity, true
	}
	return 0, false
}

// BeginGroup satisfies the display.Device interface. A group with a blend
// mode that PDF does not have goes to the raster fallback: the device
// records its content until its EndGroup.
func (d *device) BeginGroup(g display.Group) {
	if d.rec != nil {
		d.depth++
		d.rec.BeginGroup(g)
		return
	}
	f := frame{depth: len(d.stack), state: d.state}
	f.state.pushes = d.pushes
	blend, ok := blendMode(g.Blend)
	switch {
	case d.skip():
	case d.level+len(d.groups) >= d.enc.lim.MaxDepth:
		d.enc.log.Debug("pdfenc: group past the depth limit", "depth", d.level+len(d.groups))
		d.state.hidden = true
	case !ok:
		d.rec, d.recGroup, d.depth = display.NewRecording(), g, 0
	case !(g.Opacity > 0):
		d.state.hidden = true
	case g.Opacity >= 1 && blend == pdf.BlendModeNormal && !g.Isolated && !g.Knockout && g.Mask == nil:
		// the group draws straight onto its parent, as in OkageDevice
	default:
		pg := pdf.Group{
			Opacity:   path.NormalizedF32Clamped(g.Opacity),
			BlendMode: blend,
			Isolated:  g.Isolated,
			Knockout:  g.Knockout,
		}
		if g.Mask != nil {
			pg.Mask = d.mask(g.Mask)
		}
		d.s.PushGroup(pg)
		d.push()
	}
	d.groups = append(d.groups, f)
}

// EndGroup satisfies the display.Device interface.
func (d *device) EndGroup() {
	if d.rec != nil {
		if d.depth > 0 {
			d.depth--
			d.rec.EndGroup()
			return
		}
		d.endRecording(1)
		return
	}
	d.endGroup()
}

// endRecording ends the recording of a group for the raster fallback, and
// draws it. open is the number of the groups of the recording that are
// open, the group of the fallback too.
func (d *device) endRecording(open int) {
	rec, g := d.rec, d.recGroup
	d.rec, d.depth = nil, 0
	d.fallbackGroup(rec, g, open)
	d.endGroup()
}

// endGroup pops the group that BeginGroup started, and restores its state.
func (d *device) endGroup() {
	n := len(d.groups)
	if n == 0 {
		return
	}
	f := d.groups[n-1]
	d.groups = d.groups[:n-1]
	d.popTo(f.state.pushes)
	d.stack = d.stack[:min(f.depth, len(d.stack))]
	d.state = f.state
}

// fallbackGroup draws the recorded content of the group g with okage,
// inside the bounds of the clip, without the backdrop of the group. open is
// the number of the groups that the replay leaves open.
func (d *device) fallbackGroup(rec *display.List, g display.Group, open int) {
	if d.skip() {
		return
	}
	inv, ok := d.s.CTM().Invert()
	if !ok {
		return
	}
	// the links of the content stay links
	rec.Replay(&markDevice{d: d, t: okage.IdentityTransform()})
	bounds, ok := d.state.clip.Transform(inv)
	if !ok {
		return
	}
	d.raster(bounds, func(dev *display.OkageDevice) {
		dev.BeginGroup(g)
		rec.Replay(dev)
		for range open {
			dev.EndGroup()
		}
	})
}

// maskKey identifies a soft mask that the device drew.
type maskKey struct {
	key      any
	ctm      okage.Transform
	typ      okage.MaskType
	backdrop okage.Color
	transfer *[256]uint8
}

// mask returns the soft mask of the writer for m. The encoder keeps the
// mask of each m with a key, at its transform.
func (d *device) mask(m *display.Mask) *pdf.Mask {
	key := maskKey{key: m.Key, ctm: d.s.CTM(), typ: m.Type, backdrop: m.Backdrop, transfer: m.Transfer}
	keep := m.Key != nil && reflect.ValueOf(m.Key).Comparable()
	if keep {
		if pm, ok := d.enc.masks[key]; ok {
			return pm
		}
	}
	b := d.s.StreamBuilder()
	ms := b.Surface()
	if m.Content != nil {
		sub := d.subDevice(ms)
		sub.level = d.level + len(d.groups) + 1
		m.Content(sub)
		sub.finish()
	}
	ms.Finish()
	typ := pdf.MaskTypeLuminosity
	if m.Type == okage.MaskTypeAlpha {
		typ = pdf.MaskTypeAlpha
	}
	pm := pdf.NewMask(b.Finish(), typ)
	if m.Type == okage.MaskTypeLuminance && m.Backdrop.Alpha() > 0 {
		// the backdrop of PDF is opaque: the color over black
		u := m.Backdrop.Premultiply().ToColorU8()
		pm = pm.WithBackdrop(pdf.NewColorRGB(u.Red(), u.Green(), u.Blue()))
	}
	if m.Transfer != nil {
		pm = pm.WithTransfer(m.Transfer)
	}
	if keep && len(d.enc.masks) < d.enc.lim.MaxObjects {
		d.enc.masks[key] = pm
	}
	return pm
}

// raster draws a part with okage, through Surface.DrawRaster, inside the
// rectangle r of the user space and the bounds of the clip. The
// resolution is that of WithRasterDPI, lower when the pixmap gets
// more than MaxPixels pixels.
func (d *device) raster(r okage.Rect, draw func(dev *display.OkageDevice)) {
	if d.skip() {
		return
	}
	ctm := d.s.CTM()
	inv, ok := ctm.Invert()
	if !ok {
		return
	}
	clip, ok := d.state.clip.Transform(inv)
	if !ok {
		return
	}
	r, ok = r.Intersect(clip)
	if !ok || r.Width() <= 0 || r.Height() <= 0 {
		return
	}
	page, ok := pageBounds(r, ctm)
	if !ok {
		return
	}
	scale := d.res
	pixels := float64(page.Width()*scale) * float64(page.Height()*scale)
	if limit := float64(d.enc.lim.MaxPixels); pixels > limit {
		scale *= float32(math.Sqrt(limit / pixels))
		pixels = limit
	}
	if !d.enc.charge(4 * int64(math.Ceil(pixels))) {
		return
	}
	d.s.DrawRaster(r, scale, func(pm *okage.Pixmap, t okage.Transform) {
		draw(display.NewOkage(pm, t))
	})
}

// sizeOf returns the size of r.
func sizeOf(r okage.Rect) okage.Size {
	size, _ := path.SizeFromWH(r.Width(), r.Height())
	return size
}
