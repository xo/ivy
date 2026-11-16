package metafile

import (
	"context"
	"fmt"
	"image"
	"image/color"
	"math"

	"github.com/xo/okage"

	"github.com/xo/ivy/internal/display"
	"github.com/xo/ivy/internal/gowemf"
	"github.com/xo/ivy/internal/raster"
)

// backend receives the drawing operations of gowemf, in the coordinates of
// the user space of the device, and sends them to the device.
type backend struct {
	ctx context.Context
	d   display.Device
	pic *Picture
	// surface is the rectangle that the clip of GDI covers by default: the
	// target of the picture, with a margin of its size on each side.
	surface okage.Rect
	// far is the distance from the target past which a point is garbage. A
	// path with such a point is left out, because okage fails on a
	// coordinate near 1e12 pixels.
	far float64
	// cx and cy are the center of the target.
	cx, cy float64
	// drew is set after the first operation that reached the device.
	drew bool
	// skipped counts the records that the backend left out, and firstSkip
	// is the reason for the first one.
	skipped   int
	firstSkip string
	// patterns holds the pixmaps of the images of the patterns.
	patterns map[image.Image]*okage.Pixmap
}

// The interfaces that a backend implements.
var (
	_ gowemf.Backend         = (*backend)(nil)
	_ gowemf.TextBackend     = (*backend)(nil)
	_ gowemf.GradientBackend = (*backend)(nil)
)

// newBackend returns the backend that draws p onto d, into dst.
func newBackend(ctx context.Context, d display.Device, dst okage.Rect, p *Picture) *backend {
	surface, ok := okage.RectFromXYWH(dst.X()-dst.Width(), dst.Y()-dst.Height(), 3*dst.Width(), 3*dst.Height())
	if !ok {
		surface = dst
	}
	return &backend{
		ctx:     ctx,
		d:       d,
		pic:     p,
		surface: surface,
		far:     maxFar * max(float64(dst.Width()), float64(dst.Height()), 1),
		cx:      float64(dst.X() + dst.Width()/2),
		cy:      float64(dst.Y() + dst.Height()/2),
	}
}

// maxFar is the distance from the target, in sizes of the target, past which
// a point is garbage.
const maxFar = 1e6

// near reports whether the point (x, y) is not garbage.
func (b *backend) near(x, y float64) bool {
	return math.Abs(x-b.cx) <= b.far && math.Abs(y-b.cy) <= b.far
}

// close releases the caches of the backend.
func (b *backend) close() {
	b.patterns = nil
}

// unsupported is the callback of gowemf for a record that it cannot draw.
// The backend logs the record and goes on, unless the context ended.
func (b *backend) unsupported(op gowemf.UnsupportedOperation) error {
	if err := b.ctx.Err(); err != nil {
		return fmt.Errorf("drawing metafile: %w", err)
	}
	b.skip(op.Reason)
	b.pic.log.Debug("metafile: record left out", "offset", op.Source.Offset, "reason", op.Reason)
	return nil
}

// skip counts an operation that the backend left out.
func (b *backend) skip(reason string) {
	if b.skipped == 0 {
		b.firstSkip = reason
	}
	b.skipped++
}

// begin checks the context before an operation, and marks the picture as
// drawn.
func (b *backend) begin() error {
	if err := b.ctx.Err(); err != nil {
		return fmt.Errorf("drawing metafile: %w", err)
	}
	b.drew = true
	return nil
}

// transform converts a matrix of gowemf to a transform of okage.
func transform(m gowemf.Matrix) okage.Transform {
	return okage.TransformFromRow(float32(m.M11), float32(m.M12), float32(m.M21), float32(m.M22), float32(m.Dx), float32(m.Dy))
}

// path converts a path of gowemf to a path of okage. It returns nil for an
// empty path, and for a path with a point that is garbage.
func (b *backend) path(p gowemf.Path) *okage.Path {
	pb := okage.NewPathBuilderWithCapacity(len(p.Verbs), len(p.Points))
	pts := p.Points
	i := 0
	pt := func() (float32, float32, bool) {
		if i >= len(pts) {
			return 0, 0, false
		}
		v := pts[i]
		i++
		return float32(v.X), float32(v.Y), b.near(v.X, v.Y)
	}
	for _, v := range p.Verbs {
		switch v {
		case gowemf.PathMoveTo:
			x, y, ok := pt()
			if !ok {
				return nil
			}
			pb.MoveTo(x, y)
		case gowemf.PathLineTo:
			x, y, ok := pt()
			if !ok {
				return nil
			}
			pb.LineTo(x, y)
		case gowemf.PathCubicTo:
			x1, y1, ok1 := pt()
			x2, y2, ok2 := pt()
			x, y, ok3 := pt()
			if !ok1 || !ok2 || !ok3 {
				return nil
			}
			pb.CubicTo(x1, y1, x2, y2, x, y)
		case gowemf.PathClose:
			pb.Close()
		}
	}
	out, ok := pb.Finish()
	if !ok {
		return nil
	}
	return out
}

// rule converts a fill rule of gowemf.
func rule(r gowemf.FillRule) okage.FillRule {
	if r == gowemf.NonZero {
		return okage.FillRuleWinding
	}
	return okage.FillRuleEvenOdd
}

// rgba converts a color of gowemf.
func rgba(c color.NRGBA) okage.Color {
	return okage.ColorFromRGBA8(c.R, c.G, c.B, c.A)
}

// FillPath satisfies the gowemf.Backend interface.
func (b *backend) FillPath(p gowemf.Path, r gowemf.FillRule, paint gowemf.Paint, clip gowemf.Clip) error {
	if err := b.begin(); err != nil {
		return err
	}
	op := b.path(p)
	shader := b.shader(paint)
	if op == nil || shader == nil {
		return nil
	}
	b.clipped(clip, func() {
		b.d.FillPath(op, rule(r), shader)
	})
	return nil
}

// StrokePath satisfies the gowemf.Backend interface.
func (b *backend) StrokePath(p gowemf.Path, s gowemf.Stroke, clip gowemf.Clip) error {
	if err := b.begin(); err != nil {
		return err
	}
	op := b.path(p)
	shader := b.shader(s.Paint)
	if op == nil || shader == nil {
		return nil
	}
	b.clipped(clip, func() {
		b.stroke(op, s, shader)
	})
	return nil
}

// stroke strokes p with the pen s. A hairline is one pixel of the device
// wide. A geometric pen strokes in the space of the pen, which its
// transform maps to the user space.
func (b *backend) stroke(p *okage.Path, s gowemf.Stroke, shader okage.Shader) {
	b.d.Save()
	defer b.d.Restore()
	// GDI draws a line through the centers of the pixels of its device
	b.d.Concat(okage.TransformFromTranslate(float32(s.PixelCenter.X), float32(s.PixelCenter.Y)))
	st := okage.DefaultStroke()
	st.Width = 0
	unit := float32(math.Abs(s.PixelCenter.X) + math.Abs(s.PixelCenter.Y))
	if !s.Hairline {
		t := transform(s.Transform)
		inv, ok := t.Invert()
		m := s.Transform
		scale := math.Max(math.Hypot(m.M11, m.M12), math.Hypot(m.M21, m.M22))
		if !ok || !(s.Width > 0) || !(s.Width*scale <= b.far) {
			// a pen wider than the distance of garbage is garbage
			return
		}
		pen, ok := display.Clone(p).Transform(inv)
		if !ok {
			return
		}
		b.d.Concat(t)
		p, shader = pen, shader.Transform(inv)
		st.Width = float32(s.Width)
		unit = st.Width
	}
	if unit <= 0 {
		unit = 1
	}
	st.LineCap = lineCap(s.Cap)
	st.LineJoin = lineJoin(s.Join)
	if s.MiterLimit >= 1 {
		st.MiterLimit = float32(s.MiterLimit)
	}
	if len(s.Compound) >= 2 && !s.Hairline {
		b.compound(p, st, s.Compound, shader)
		return
	}
	if dashes := dashArray(s, unit); dashes != nil {
		if s.Gap != nil {
			if gap := b.shader(*s.Gap); gap != nil {
				if !s.Hairline {
					gap = gap.Transform(okInverse(transform(s.Transform)))
				}
				b.d.StrokePath(p, st, gap)
			}
		}
		if dash, ok := okage.NewStrokeDash(dashes, float32(s.DashOffset)*unitScale(s, unit)); ok {
			st.Dash = dash
		}
	}
	b.d.StrokePath(p, st, shader)
}

// okInverse returns the inverse of t, or the identity.
func okInverse(t okage.Transform) okage.Transform {
	inv, ok := t.Invert()
	if !ok {
		return okage.IdentityTransform()
	}
	return inv
}

// unitScale returns the scale of the lengths of the dashes of s: one device
// pixel for a hairline, and one unit of the space of the pen for a
// geometric pen.
func unitScale(s gowemf.Stroke, unit float32) float32 {
	if s.Hairline {
		return unit
	}
	return 1
}

// compound strokes a compound pen of EMF+. Each band of the pen is the
// stroke of its outer width less the stroke of its inner width. gowemf
// passes bands that are symmetric about the center, so the backend draws
// the bands of the first half.
func (b *backend) compound(p *okage.Path, st okage.Stroke, bands []float64, shader okage.Shader) {
	width := st.Width
	st.LineCap = okage.LineCapButt
	st.Dash = nil
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
		if outer <= 0 {
			continue
		}
		b.d.BeginGroup(display.Group{Opacity: 1, Isolated: true})
		st.Width = outer
		b.d.StrokePath(p, st, shader)
		if inner > 0 {
			b.d.BeginGroup(display.Group{Opacity: 1, Blend: okage.BlendModeDestinationOut})
			st.Width = inner
			b.d.StrokePath(p, st, okage.SolidColor(okage.ColorBlack))
			b.d.EndGroup()
		}
		b.d.EndGroup()
	}
}

// lineCap converts a line cap of gowemf.
func lineCap(c gowemf.LineCap) okage.LineCap {
	switch c {
	case gowemf.CapRound:
		return okage.LineCapRound
	case gowemf.CapSquare:
		return okage.LineCapSquare
	}
	return okage.LineCapButt
}

// lineJoin converts a line join of gowemf.
func lineJoin(j gowemf.LineJoin) okage.LineJoin {
	switch j {
	case gowemf.JoinRound:
		return okage.LineJoinRound
	case gowemf.JoinBevel:
		return okage.LineJoinBevel
	}
	return okage.LineJoinMiter
}

// The dashes of the styles of the pens of GDI, in pixels for a hairline and
// in widths of the pen for a geometric pen. GDI leaves them to the device,
// so these are the lengths that Windows draws on a screen.
var (
	hairlineDashes = map[gowemf.DashStyle][]float32{
		gowemf.DashDash:       {18, 6},
		gowemf.DashDot:        {3, 3},
		gowemf.DashDashDot:    {9, 6, 3, 6},
		gowemf.DashDashDotDot: {9, 3, 3, 3, 3, 3},
		gowemf.DashAlternate:  {1, 1},
	}
	geometricDashes = map[gowemf.DashStyle][]float32{
		gowemf.DashDash:       {3, 1},
		gowemf.DashDot:        {1, 1},
		gowemf.DashDashDot:    {3, 1, 1, 1},
		gowemf.DashDashDotDot: {3, 1, 1, 1, 1, 1},
		gowemf.DashAlternate:  {1, 1},
	}
)

// dashArray returns the dashes of s in the units of the path, or nil for a
// solid pen. unit is the width of the pen, or a pixel of the device for a
// hairline.
func dashArray(s gowemf.Stroke, unit float32) []float32 {
	var base []float32
	scale := unit
	switch {
	case s.Dash == gowemf.DashSolid || s.Dash == 0:
		return nil
	case s.Dash == gowemf.DashUser:
		for _, v := range s.Dashes {
			base = append(base, float32(v))
		}
		scale = unitScale(s, unit)
	case s.Hairline:
		base = hairlineDashes[s.Dash]
	default:
		base = geometricDashes[s.Dash]
	}
	if len(base) == 0 || len(base) > 1<<10 {
		return nil
	}
	out := make([]float32, 0, 2*len(base))
	var sum float32
	for _, v := range base {
		v *= scale
		if !(v >= 0) || math.IsInf(float64(v), 0) {
			return nil
		}
		sum += v
		out = append(out, v)
	}
	if !(sum > 0) {
		return nil
	}
	if len(out)%2 == 1 {
		out = append(out, out...)
	}
	return out
}

// DrawImage satisfies the gowemf.Backend interface.
func (b *backend) DrawImage(im gowemf.ImageDraw, clip gowemf.Clip) error {
	if err := b.begin(); err != nil {
		return err
	}
	img := im.Image
	if img == nil {
		return nil
	}
	src := im.Source.Intersect(img.Bounds())
	if src.Empty() {
		return nil
	}
	if src != img.Bounds() {
		sub, ok := img.(interface {
			SubImage(image.Rectangle) image.Image
		})
		if !ok {
			return nil
		}
		img = sub.SubImage(src)
	}
	for _, c := range []image.Point{src.Min, {src.Max.X, src.Min.Y}, {src.Min.X, src.Max.Y}, src.Max} {
		if q := im.Transform.Apply(gowemf.Point{X: float64(c.X), Y: float64(c.Y)}); !b.near(q.X, q.Y) {
			return nil
		}
	}
	t := transform(im.Transform).PreTranslate(float32(src.Min.X), float32(src.Min.Y))
	opacity := float32(im.Opacity)
	b.clipped(clip, func() {
		b.d.DrawImage(img, t, display.ImageOptions{Opacity: opacity, Smooth: im.Smooth})
	})
	return nil
}

// shader returns the shader of a paint, or nil if the paint draws nothing.
func (b *backend) shader(p gowemf.Paint) okage.Shader {
	switch p.Kind {
	case gowemf.PaintSolid:
		return okage.SolidColor(rgba(p.Color))
	case gowemf.PaintHatch:
		return b.hatch(p)
	case gowemf.PaintPattern:
		return b.pattern(p)
	case gowemf.PaintLinearGradient:
		return linearGradient(p.Gradient)
	}
	return nil
}

// hatch returns the shader of a hatch of GDI: an 8 by 8 tile of the pixels
// of the device, with lines of the color over the background.
func (b *backend) hatch(p gowemf.Paint) okage.Shader {
	img := image.NewNRGBA(image.Rect(0, 0, 8, 8))
	if p.Background != nil {
		for i := 0; i < len(img.Pix); i += 4 {
			img.Pix[i], img.Pix[i+1], img.Pix[i+2], img.Pix[i+3] = p.Background.R, p.Background.G, p.Background.B, p.Background.A
		}
	}
	horizontal := p.Hatch == 0 || p.Hatch == 4
	vertical := p.Hatch == 1 || p.Hatch == 4
	forward := p.Hatch == 2 || p.Hatch == 5
	backward := p.Hatch == 3 || p.Hatch == 5
	if p.Hatch > 5 {
		return okage.SolidColor(rgba(p.Color))
	}
	for y := range 8 {
		for x := range 8 {
			if (horizontal && y == 7) || (vertical && x == 7) || (forward && x == y) || (backward && x+y == 7) {
				img.SetNRGBA(x, y, p.Color)
			}
		}
	}
	pm, err := raster.Pixmap(img)
	if err != nil {
		return okage.SolidColor(rgba(p.Color))
	}
	return okage.NewPattern(pm.AsRef(), okage.SpreadModeRepeat, okage.FilterQualityNearest, 1, transform(p.PatternTransform))
}

// pattern returns the shader of a pattern or a texture.
func (b *backend) pattern(p gowemf.Paint) okage.Shader {
	img := p.Pattern
	if img == nil || img.Bounds().Empty() {
		return nil
	}
	clamp := p.Wrap == gowemf.WrapClamp
	key := img
	pm := b.patterns[key]
	if pm == nil || clamp {
		if clamp {
			// a clamped pattern paints nothing outside its tile, so the tile
			// gets a transparent border, which the pad of okage repeats
			bounds := img.Bounds()
			framed := image.NewNRGBA(bounds.Inset(-1))
			for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
				for x := bounds.Min.X; x < bounds.Max.X; x++ {
					framed.Set(x, y, img.At(x, y))
				}
			}
			img = framed
		}
		var err error
		pm, err = raster.Pixmap(img)
		if err != nil {
			return nil
		}
		if !clamp {
			if b.patterns == nil {
				b.patterns = make(map[image.Image]*okage.Pixmap)
			}
			b.patterns[key] = pm
		}
	}
	t := transform(p.PatternTransform)
	spread := okage.SpreadModeRepeat
	switch p.Wrap {
	case gowemf.WrapTileFlipX, gowemf.WrapTileFlipY, gowemf.WrapTileFlipXY:
		// okage mirrors in both directions
		spread = okage.SpreadModeReflect
	case gowemf.WrapClamp:
		spread = okage.SpreadModePad
		t = t.PreTranslate(-1, -1)
	}
	t = t.PreTranslate(float32(p.Pattern.Bounds().Min.X), float32(p.Pattern.Bounds().Min.Y))
	return okage.NewPattern(pm.AsRef(), spread, okage.FilterQualityBilinear, 1, t)
}

// linearGradient returns the shader of a linear gradient of EMF+.
func linearGradient(g *gowemf.LinearGradient) okage.Shader {
	if g == nil || len(g.Stops) == 0 {
		return nil
	}
	stops := make([]okage.GradientStop, 0, len(g.Stops))
	for _, s := range g.Stops {
		stops = append(stops, okage.NewGradientStop(float32(s.Offset), rgba(s.Color)))
	}
	inv, ok := transform(g.Transform).Invert()
	if !ok {
		return okage.SolidColor(rgba(g.Stops[0].Color))
	}
	spread := okage.SpreadModeRepeat
	switch g.Wrap {
	case gowemf.WrapTileFlipX, gowemf.WrapTileFlipXY:
		spread = okage.SpreadModeReflect
	case gowemf.WrapClamp:
		spread = okage.SpreadModePad
	}
	shader, ok := okage.NewLinearGradient(okage.PointFromXY(0, 0), okage.PointFromXY(1, 0), stops, spread, inv)
	if !ok {
		return okage.SolidColor(rgba(g.Stops[0].Color))
	}
	return shader
}
