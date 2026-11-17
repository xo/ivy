package xps

import (
	"fmt"
	"math"

	"github.com/xo/ivy"
	"github.com/xo/ivy/internal/display"
	"github.com/xo/okage"
	"github.com/xo/okage/roxmltree"
)

// The bounds of the visuals and the tiles of the brushes.
const (
	// maxVisuals is the depth of the VisualBrush elements that a page can
	// nest, one inside the visual of another. Limits.MaxDepth bounds it
	// too. A visual that fills itself with its own brush stops at this depth.
	maxVisuals = 4
	// maxTile is the number of pixels of the largest tile of a brush. A tile
	// at the resolution of the target that is larger draws at a lower
	// resolution.
	maxTile = 1 << 24
)

// contentFunc draws the content of a brush onto d, in the space of the
// brush. ctm is the transform of d, area is the target of d in its pixels,
// and opacity multiplies the alpha of the content.
type contentFunc func(d display.Device, ctm okage.Transform, area okage.Rect, opacity float32)

// once returns a brush that draws its content one time, clipped to the
// viewport, for the TileMode None. t maps the space of the brush onto the
// user space of the fill.
func (r *renderer) once(viewport okage.Rect, t okage.Transform, opacity float32, content contentFunc) *brush {
	area := r.area
	clip := okage.PathFromRect(viewport)
	return &brush{draw: func(d display.Device, ctm okage.Transform) {
		if opacity <= 0 {
			return
		}
		d.Concat(t)
		d.ClipPath(clip, okage.FillRuleWinding)
		content(d, ctm.PreConcat(t), area, opacity)
	}}
}

// tile returns a brush that repeats its content, the tile, over the plane.
// The viewport is the tile in the space of the brush, and t maps the space of
// the brush onto the user space of the fill. base holds the pixels of the
// tile, or is nil. Without base, or when base is too large, the content
// draws into a tile at the resolution of the target.
//
// The modes FlipX, FlipY and FlipXY mirror every other column, row, or both,
// of the tiles. The pattern of okage has one spread mode for both axes, so
// the brush repeats a tile of 2 by 1, 1 by 2 or 2 by 2 copies of the base,
// each one mirrored as ECMA-388 says.
func (r *renderer) tile(viewport okage.Rect, t okage.Transform, mode string, opacity float32, base *okage.Pixmap, content contentFunc) (*brush, error) {
	nx, ny := 1, 1
	switch mode {
	case "FlipX":
		nx = 2
	case "FlipY":
		ny = 2
	case "FlipXY":
		nx, ny = 2, 2
	}
	limit := max(min(r.doc.lim.MaxPixels, maxTile)/int64(nx*ny), 1)
	if base == nil || int64(base.Width())*int64(base.Height()) > limit {
		w, h, ok := r.tileSize(viewport, t, limit)
		if !ok {
			// the transform hides the brush
			return nil, nil
		}
		if err := r.charge(int64(w) * int64(h) * int64(nx*ny)); err != nil {
			return nil, err
		}
		pix, ok := okage.NewPixmap(w, h)
		if !ok {
			return nil, fmt.Errorf("tile of %dx%d pixels: %w", w, h, ivy.ErrLimit)
		}
		ctm := okage.TransformFromScale(float32(w)/viewport.Width(), float32(h)/viewport.Height()).
			PreTranslate(-viewport.X(), -viewport.Y())
		area, _ := okage.RectFromXYWH(0, 0, float32(w), float32(h))
		content(display.NewOkage(pix, ctm), ctm, area, 1)
		base = pix
	}
	pix := base
	if nx*ny > 1 {
		var ok bool
		if pix, ok = mirror(base, nx, ny); !ok {
			return nil, fmt.Errorf("tile of %dx%d pixels: %w", nx*int(base.Width()), ny*int(base.Height()), ivy.ErrLimit)
		}
	}
	// the pattern maps the pixels of the base onto the viewport, and repeats
	pattern := t.PreTranslate(viewport.X(), viewport.Y()).
		PreScale(viewport.Width()/float32(base.Width()), viewport.Height()/float32(base.Height()))
	shader := okage.NewPattern(pix.AsRef(), okage.SpreadModeRepeat, okage.FilterQualityBilinear, opacity, pattern)
	return &brush{shader: shader, pix: pix}, nil
}

// tileSize returns the size in pixels of a tile for the viewport, at the
// resolution that the current transform and t give, and at most limit
// pixels. It returns false if the transform hides the tile.
func (r *renderer) tileSize(viewport okage.Rect, t okage.Transform, limit int64) (uint32, uint32, bool) {
	m := r.ctm.PreConcat(t)
	// the lengths in pixels of a unit along each axis of the brush
	w := float64(viewport.Width()) * math.Hypot(float64(m.SX), float64(m.KY))
	h := float64(viewport.Height()) * math.Hypot(float64(m.KX), float64(m.SY))
	if !(w > 0 && h > 0) || math.IsInf(w, 0) || math.IsInf(h, 0) {
		return 0, 0, false
	}
	n := float64(limit)
	w, h = max(w, 1), max(h, 1)
	if w*h > n {
		f := math.Sqrt(n / (w * h))
		w, h = max(w*f, 1), max(h*f, 1)
	}
	pw := min(math.Ceil(w), n)
	ph := min(math.Ceil(h), math.Floor(n/pw))
	return uint32(pw), uint32(ph), true
}

// mirror returns a pixmap of nx by ny copies of base. The copy in the second
// column is mirrored left to right, and the copy in the second row is
// mirrored top to bottom.
func mirror(base *okage.Pixmap, nx, ny int) (*okage.Pixmap, bool) {
	w, h := int(base.Width()), int(base.Height())
	out, ok := okage.NewPixmap(uint32(w*nx), uint32(h*ny))
	if !ok {
		return nil, false
	}
	src, dst := base.Data(), out.DataMut()
	row := 4 * w
	stride := row * nx
	for ty := range ny {
		for y := range h {
			sy := y
			if ty == 1 {
				sy = h - 1 - y
			}
			s := src[sy*row : (sy+1)*row]
			d := dst[(ty*h+y)*stride:][:stride]
			copy(d, s)
			if nx == 2 {
				m := d[row:]
				for x := range w {
					copy(m[4*x:4*x+4], s[4*(w-1-x):4*(w-x)])
				}
			}
		}
	}
	return out, true
}

// charge adds pixels to the pixels that the visuals and the tiles of the
// page drew. It returns an error that wraps ivy.ErrLimit when the sum is more
// than Limits.MaxPixels.
func (r *renderer) charge(pixels int64) error {
	r.w.pixels += pixels
	if r.w.pixels > r.doc.lim.MaxPixels {
		return fmt.Errorf("visual brushes of more than %d pixels: %w", r.doc.lim.MaxPixels, ivy.ErrLimit)
	}
	return nil
}

// visualBrush returns the brush of a VisualBrush. Its Visual, a Canvas, a
// Path or a Glyphs element, draws in the Viewbox, which maps onto the
// Viewport in the space of the brush. ECMA-388 allows only the value
// Absolute for the ViewboxUnits and the ViewportUnits, so the brush ignores
// them.
func (r *renderer) visualBrush(n roxmltree.Node, res *resources, opacity float32) (*brush, error) {
	if limit := min(r.doc.lim.MaxDepth, maxVisuals); r.visuals >= limit {
		return nil, fmt.Errorf("visual brushes nested deeper than %d: %w", limit, ivy.ErrLimit)
	}
	visual, err := visualOf(n, res)
	if err != nil {
		return nil, err
	}
	viewbox, ok1 := parseRect(attr(n, "Viewbox"))
	viewport, ok2 := parseRect(attr(n, "Viewport"))
	if !ok1 || !ok2 {
		return nil, fmt.Errorf("visual brush without a viewbox or a viewport: %w", ivy.ErrFormat)
	}
	if !(viewbox.Width() > 0 && viewbox.Height() > 0 && viewport.Width() > 0 && viewport.Height() > 0) {
		// an empty viewbox or viewport draws nothing
		return nil, nil
	}
	sx, sy := viewport.Width()/viewbox.Width(), viewport.Height()/viewbox.Height()
	// v maps the viewbox onto the viewport
	v := okage.TransformFromTranslate(viewport.X()-viewbox.X()*sx, viewport.Y()-viewbox.Y()*sy).PreScale(sx, sy)
	t := r.brushTransform(n, res)
	content := func(d display.Device, ctm okage.Transform, area okage.Rect, opacity float32) {
		if opacity < 1 {
			d.BeginGroup(display.Group{Opacity: opacity})
			defer d.EndGroup()
		}
		d.Concat(v)
		r.walkVisual(d, ctm.PreConcat(v), area, visual, res)
	}
	mode := attr(n, "TileMode")
	if mode != "" && mode != "None" {
		return r.tile(viewport, t, mode, opacity, nil, content)
	}
	return r.once(viewport, t, opacity, func(d display.Device, ctm okage.Transform, area okage.Rect, opacity float32) {
		// a visual inside a visual draws through a clip of the size of the
		// target, which counts against the limit
		if r.visuals > 0 {
			if err := r.charge(int64(area.Width()) * int64(area.Height())); err != nil {
				r.fail(err)
				return
			}
		}
		content(d, ctm, area, opacity)
	}), nil
}

// visualOf returns the Visual of a VisualBrush, from its attribute, a static
// resource, or from its property element.
func visualOf(n roxmltree.Node, res *resources) (roxmltree.Node, error) {
	if hasAttr(n, "Visual") {
		s := attr(n, "Visual")
		node, ok, err := res.static(s)
		if err != nil {
			return node, err
		}
		if !ok {
			return node, fmt.Errorf("visual %q: %w", s, ivy.ErrFormat)
		}
		return node, nil
	}
	if node, ok := property(n, "Visual"); ok {
		return node, nil
	}
	return roxmltree.Node{}, fmt.Errorf("visual brush without a visual: %w", ivy.ErrFormat)
}

// walkVisual walks the visual n onto the device d, whose transform is ctm
// and whose target is area. The walk shares the state of the page, so its
// elements count against the limits of the page, and its errors make the
// page partial.
func (r *renderer) walkVisual(d display.Device, ctm okage.Transform, area okage.Rect, n roxmltree.Node, res *resources) {
	sub := &renderer{
		ctx:     r.ctx,
		doc:     r.doc,
		part:    r.part,
		dev:     d,
		ctm:     ctm,
		area:    area,
		visuals: r.visuals + 1,
		w:       r.w,
	}
	sub.element(n, res, 1)
}
