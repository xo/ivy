package drawingml

import (
	"context"
	"errors"
	"math"

	"github.com/xo/okage"

	"github.com/xo/ivy/internal/display"
)

// Figure is a shape with its properties resolved: its paths, its fill, its
// outline, its shadow and its text. A format makes it from the shapes of its
// pages, and a Scene draws it.
type Figure struct {
	// Transform maps the box of the shape, from (0, 0) to (W, H), onto the
	// space of the scene. It holds the position, the rotation and the
	// flips.
	Transform okage.Transform
	// W and H are the size of the box.
	W, H float64
	// Paths are the paths of the shape in its box, from Evaluate or from
	// the paths of the format.
	Paths []Path
	// Fill is the fill of the paths.
	Fill Paint
	// Line is the outline of the paths.
	Line Line
	// Shadow is the outer shadow of the shape, or nil.
	Shadow *Shadow
	// InnerShadow is the inner shadow of the shape, or nil. It draws over
	// the fill and under the outline.
	InnerShadow *Shadow
	// Glow is the glow of the shape, or nil. It draws under the shape.
	Glow *Glow
	// SoftEdge is the radius of the soft edge of the shape, in points, or
	// 0. The fill and the outline fade out at the outline.
	SoftEdge float64
	// Reflection is the reflection of the shape and its text, or nil.
	Reflection *Reflection
	// Text is the text of the shape, or nil.
	Text *TextBody
	// TextRect is the box of the text in the box of the shape. An empty
	// rectangle is the box of the shape.
	TextRect okage.Rect
	// TextTransform maps the box of the shape onto the space of the scene
	// for the text. The text does not mirror with the shape, so it is the
	// transform of the shape without its flips.
	TextTransform okage.Transform
	// Theme gives the fonts of the theme to the text, or is nil.
	Theme *Theme
}

// Shadow is the outer shadow (a:outerShdw) or the inner shadow
// (a:innerShdw) of a shape.
type Shadow struct {
	Color RGBA
	// DX and DY are the offset of the shadow, in points.
	DX, DY float64
	// Blur is the radius of the blur, in points. The shadow draws with a
	// blur whose standard deviation is half the radius.
	Blur float64
	// SX and SY scale the shadow, from 1.
	SX, SY float64
}

// ShapeTransform returns the transform of a shape of the box at (x, y) of
// the size w by h, rotated by rot degrees about its center and flipped. It
// maps the box from the origin onto its place.
func ShapeTransform(x, y, w, h, rot float64, flipH, flipV bool) okage.Transform {
	cx, cy := float32(x+w/2), float32(y+h/2)
	t := okage.TransformFromTranslate(cx, cy)
	if rot != 0 && !math.IsNaN(rot) && !math.IsInf(rot, 0) {
		t = t.PreRotate(float32(rot))
	}
	sx, sy := float32(1), float32(1)
	if flipH {
		sx = -1
	}
	if flipV {
		sy = -1
	}
	return t.PreScale(sx, sy).PreTranslate(float32(-w/2), float32(-h/2))
}

// TextTransform returns the transform of the text of a shape, with the same
// arguments as ShapeTransform without the horizontal flip, which leaves the
// text as it is. The text of a shape that flips vertically turns by 180
// degrees.
func TextTransform(x, y, w, h, rot float64, flipV bool) okage.Transform {
	if flipV {
		rot += 180
	}
	return ShapeTransform(x, y, w, h, rot, false, false)
}

// AddFigure adds the figure to the scene: its shadow, its glow, its
// reflection, its fill, its inner shadow, its outline and its text, in
// that order. The soft edge fades the fill, the inner shadow and the
// outline.
func (s *Scene) AddFigure(f *Figure) {
	box, ok := okage.RectFromXYWH(0, 0, float32(max(f.W, 1e-3)), float32(max(f.H, 1e-3)))
	if !ok || !f.Transform.IsFinite() {
		return
	}
	paths := f.Paths
	visible := f.Fill.Visible() || f.Line.Visible()
	if f.Shadow != nil && f.Shadow.Color.A > 0 && visible {
		sh := *f.Shadow
		m := okage.TransformFromTranslate(float32(sh.DX), float32(sh.DY)).PreConcat(f.Transform)
		if sh.SX > 0 && sh.SY > 0 && (sh.SX != 1 || sh.SY != 1) {
			m = m.PreScale(float32(sh.SX), float32(sh.SY))
		}
		s.Add(m, func(_ context.Context, dev display.Device, scale float64) error {
			col := sh.Color
			if sh.Blur > 0 {
				var shape []*okage.Path
				var stroke []bool
				for _, p := range paths {
					switch {
					case f.Fill.Visible() && p.Fill != FillNone:
						shape, stroke = append(shape, p.Path), append(stroke, false)
					case f.Line.Visible() && p.Stroke:
						shape, stroke = append(shape, p.Path), append(stroke, true)
					}
				}
				if blurredShadow(dev, shape, stroke, f.Line.stroke(), col, sh.Blur, scale) {
					return nil
				}
				// without the mask, a lighter shadow stands for the blur
				col = col.WithAlpha(0.7)
			}
			shader := okage.SolidColor(col.Okage())
			for _, p := range paths {
				switch {
				case f.Fill.Visible() && p.Fill != FillNone:
					dev.FillPath(p.Path, okage.FillRuleWinding, shader)
				case f.Line.Visible() && p.Stroke:
					st := f.Line.stroke()
					dev.StrokePath(p.Path, st, shader)
				}
			}
			return nil
		})
	}
	if f.Glow != nil && visible {
		s.Add(f.Transform, func(_ context.Context, dev display.Device, scale float64) error {
			drawGlow(dev, f, scale)
			return nil
		})
	}
	var ops []sceneOp
	if visible {
		ops = append(ops, drawOp(f.Transform, func(ctx context.Context, dev display.Device, scale float64) error {
			if f.SoftEdge > 0 {
				if mask := softEdgeMask(f, scale); mask != nil {
					dev.BeginGroup(display.Group{Opacity: 1, Isolated: true, Mask: mask})
					defer dev.EndGroup()
				}
			}
			var errs []error
			for _, p := range paths {
				if p.Fill == FillNone || !f.Fill.Visible() {
					continue
				}
				pt := shadeFill(f.Fill, p.Fill)
				if err := pt.FillPath(ctx, dev, p.Path, okage.FillRuleEvenOdd, box, scale); err != nil {
					errs = append(errs, err)
				}
			}
			if f.InnerShadow != nil && f.InnerShadow.Color.A > 0 {
				drawInnerShadow(dev, f, f.Transform, scale)
			}
			for _, p := range paths {
				if !p.Stroke {
					continue
				}
				f.Line.StrokePath(dev, p.Path, box)
			}
			return errors.Join(errs...)
		}))
	}
	if f.Text != nil && !f.Text.Empty() {
		r := f.TextRect
		if r.Width() <= 0 || r.Height() <= 0 {
			r = box
		}
		tb, ok := f.Text.Box(f.Theme, float64(r.Width()), float64(r.Height()))
		if ok {
			m := f.TextTransform.PreTranslate(r.X(), r.Y()).PreConcat(tb.Transform)
			if op := s.htmlOp(tb.HTML, tb.Width, tb.Height, m); op != nil {
				ops = append(ops, op)
			}
		}
	}
	if f.Reflection != nil && len(ops) > 0 {
		s.ops = append(s.ops, s.reflect(f, ops)...)
	}
	s.ops = append(s.ops, ops...)
}

// shadeFill returns the paint of a path with the fill mode mode: lighter or
// darker than the paint of the shape, such as the side of a cube.
func shadeFill(pt Paint, mode Fill) Paint {
	var f func(RGBA) RGBA
	switch mode {
	case FillLighten:
		f = func(c RGBA) RGBA { return c.Mix(White, 0.4).WithAlpha(1) }
	case FillLightenLess:
		f = func(c RGBA) RGBA { return c.Mix(White, 0.2).WithAlpha(1) }
	case FillDarken:
		f = func(c RGBA) RGBA { return c.Mix(Black, 0.4).WithAlpha(1) }
	case FillDarkenLess:
		f = func(c RGBA) RGBA { return c.Mix(Black, 0.2).WithAlpha(1) }
	default:
		return pt
	}
	switch pt.Kind {
	case PaintSolid:
		a := pt.Color.A
		pt.Color = f(pt.Color)
		pt.Color.A = a
	case PaintGradient:
		stops := make([]Stop, len(pt.Gradient.Stops))
		for i, st := range pt.Gradient.Stops {
			a := st.Color.A
			st.Color = f(st.Color)
			st.Color.A = a
			stops[i] = st
		}
		pt.Gradient.Stops = stops
	}
	return pt
}
