package drawpage

import (
	"context"
	"fmt"
	"math"

	"github.com/xo/okage"
	"github.com/xo/okage/roxmltree"

	"github.com/xo/ivy"
	"github.com/xo/ivy/internal/display"
	"github.com/xo/ivy/internal/drawingml"
)

// shapeScale is the number of pixels for a point that a shape of a text
// takes for its patterns and its text. The layout does not give the
// resolution of the page to an object, so a shape takes the resolution of
// 144 dots per inch.
const shapeScale = 2

// Shapes draws the shapes of a text of Writer, such as draw:rect or
// draw:custom-shape in a paragraph, each one in a box of its own, which the
// layout places in the text. It is safe for concurrent use.
type Shapes struct {
	d *Document
}

// NewShapes returns the shapes of a text whose content.xml and styles.xml
// the caller parsed. styles can be nil.
func NewShapes(opts Options, content, styles *roxmltree.Document) *Shapes {
	d := &Document{
		opts:     opts,
		lim:      opts.Config.Limits.WithDefaults(),
		log:      opts.Config.Log(),
		st:       newStyles(),
		pictures: map[string]picture{},
	}
	d.st.add(content.RootElement())
	if styles != nil {
		d.st.add(styles.RootElement())
	}
	return &Shapes{d: d}
}

// Shape is a shape of a text, or a group of shapes, in its box. It
// satisfies the interface Object of the layout.
type Shape struct {
	scene *drawingml.Scene
	// X and Y are the corner of the box of the shape in the space of its
	// anchor, and W and H its size, in points.
	X, Y, W, H float64
}

// Shape returns the shape n of the text, which is a shape element of the
// namespace draw. Without text, the shape draws its geometry only, and the
// caller lays out its text. It returns false if n draws nothing.
func (s *Shapes) Shape(ctx context.Context, n roxmltree.Node, text bool) (sh *Shape, ok bool) {
	defer func() {
		if r := recover(); r != nil {
			s.d.log.Warn("odf: shape left out", "err", fmt.Errorf("panic: %v: %w", r, ivy.ErrFormat))
			sh, ok = nil, false
		}
	}()
	uri, _ := n.TagName().Namespace()
	if uri != nsDraw {
		return nil, false
	}
	box, ok := bounds(n, 0, min(s.d.lim.MaxDepth, 256))
	if !ok {
		return nil, false
	}
	b := &builder{
		d:      s.d,
		ctx:    ctx,
		st:     s.d.st,
		scene:  drawingml.NewScene(s.d.opts.Config),
		number: 1,
		log:    s.d.log,
		noText: !text,
	}
	// The box takes the half of the lines and the markers that go past
	// it, since the layout clips an object to its box.
	pad := b.pad(n, 0, min(s.d.lim.MaxDepth, 256))
	box = rectangle{box.x0 - pad, box.y0 - pad, box.x1 + pad, box.y1 + pad}
	// A line across or down has a box of no height or no width, which the
	// layout would not draw: the box takes 1 point around it.
	if box.x1-box.x0 < 1 {
		box.x0, box.x1 = (box.x0+box.x1)/2-0.5, (box.x0+box.x1)/2+0.5
	}
	if box.y1-box.y0 < 1 {
		box.y0, box.y1 = (box.y0+box.y1)/2-0.5, (box.y0+box.y1)/2+0.5
	}
	b.w, b.h = box.x1-box.x0, box.y1-box.y0
	b.shape(n, 0, false)
	if b.scene.Len() == 0 {
		return nil, false
	}
	return &Shape{scene: b.scene, X: box.x0, Y: box.y0, W: b.w, H: b.h}, true
}

// Gradient returns the fill of the frame n of the text, of w by h points,
// when its graphic style fills it with a gradient. CSS does not have the
// gradients of OpenDocument, so the caller draws the shape behind the
// content of the frame. It returns false for another fill.
func (s *Shapes) Gradient(ctx context.Context, n roxmltree.Node, w, h float64) (sh *Shape, ok bool) {
	defer func() {
		if r := recover(); r != nil {
			s.d.log.Warn("odf: fill of a frame left out", "err", fmt.Errorf("panic: %v: %w", r, ivy.ErrFormat))
			sh, ok = nil, false
		}
	}()
	if !(w > 0) || !(h > 0) || math.IsInf(w, 0) || math.IsInf(h, 0) {
		return nil, false
	}
	b := &builder{
		d:     s.d,
		ctx:   ctx,
		st:    s.d.st,
		scene: drawingml.NewScene(s.d.opts.Config),
		log:   s.d.log,
		w:     w,
		h:     h,
	}
	gp := b.st.props(b.shapeStyle(n), "graphic-properties", nil)
	if gp.get("draw:fill") != "gradient" {
		return nil, false
	}
	b.background(gp)
	if b.scene.Len() == 0 {
		return nil, false
	}
	return &Shape{scene: b.scene, W: w, H: h}, true
}

// Size satisfies the interface Object of the layout.
func (sh *Shape) Size() (float64, float64) {
	return sh.W, sh.H
}

// Draw satisfies the interface Object of the layout. It draws the box of
// the shape onto rect.
func (sh *Shape) Draw(ctx context.Context, dev display.Device, rect okage.Rect) error {
	rw, rh := float64(rect.Width()), float64(rect.Height())
	if !(rw > 0) || !(rh > 0) || math.IsInf(rw, 0) || math.IsInf(rh, 0) {
		return nil
	}
	m := okage.TransformFromRow(float32(rw/sh.W), 0, 0, float32(rh/sh.H), rect.X(), rect.Y()).
		PreTranslate(float32(-sh.X), float32(-sh.Y))
	dev.Save()
	defer dev.Restore()
	dev.Concat(m)
	return sh.scene.Draw(ctx, dev, okage.TransformFromScale(shapeScale, shapeScale).PreConcat(m))
}

// pad returns the width that the lines and the markers of the shape n, and
// of the shapes of a group, draw outside the box of the shape.
func (b *builder) pad(n roxmltree.Node, depth, maxDepth int) float64 {
	pad := 0.0
	switch n.TagName().Name() {
	case "g", "a":
		if depth >= maxDepth {
			return 0
		}
		for c := range n.Children() {
			if uri, _ := c.TagName().Namespace(); c.IsElement() && uri == nsDraw {
				pad = max(pad, b.pad(c, depth+1, maxDepth))
			}
		}
		return pad
	}
	gp := b.st.props(b.shapeStyle(n), "graphic-properties", nil)
	if gp.get("draw:stroke") == "none" {
		return 0
	}
	if w, ok := length(gp.get("svg:stroke-width")); ok {
		pad = w / 2
	}
	// A width counts only with a marker: a style can give the width of a
	// marker and no marker, as draw:marker-start="".
	for _, name := range [...]string{"draw:marker-start", "draw:marker-end"} {
		if gp.get(name) == "" {
			continue
		}
		if w, ok := length(gp.get(name + "-width")); ok {
			pad = max(pad, w)
		}
	}
	return min(max(pad, 0.5), 1000)
}

// rectangle is a box from (x0, y0) to (x1, y1).
type rectangle struct {
	x0, y0, x1, y1 float64
}

// add returns the box of r and the point (x, y).
func (r rectangle) add(x, y float64) rectangle {
	return rectangle{min(r.x0, x), min(r.y0, y), max(r.x1, x), max(r.y1, y)}
}

// union returns the box of r and o.
func (r rectangle) union(o rectangle) rectangle {
	return r.add(o.x0, o.y0).add(o.x1, o.y1)
}

// bounds returns the box of the shape n in the space of its anchor: the box
// of its corners after its transform, the box of the ends of a line, or the
// box of the shapes of a group.
func bounds(n roxmltree.Node, depth, maxDepth int) (rectangle, bool) {
	var pts [][2]float64
	m := okage.IdentityTransform()
	switch n.TagName().Name() {
	case "g", "a":
		if depth >= maxDepth {
			return rectangle{}, false
		}
		var box rectangle
		found := false
		for c := range n.Children() {
			if uri, _ := c.TagName().Namespace(); !c.IsElement() || uri != nsDraw {
				continue
			}
			if r, ok := bounds(c, depth+1, maxDepth); ok {
				if found {
					box = box.union(r)
				} else {
					box, found = r, true
				}
			}
		}
		return box, found
	case "line", "connector":
		x1, _ := length(at(n, nsSVG, "x1"))
		y1, _ := length(at(n, nsSVG, "y1"))
		x2, _ := length(at(n, nsSVG, "x2"))
		y2, _ := length(at(n, nsSVG, "y2"))
		pts = [][2]float64{{x1, y1}, {x2, y2}}
		if t := at(n, nsDraw, "transform"); t != "" {
			m = parseTransform(t)
		}
	case "rect", "caption", "text-box", "ellipse", "circle", "custom-shape", "polygon", "polyline", "path",
		"regular-polygon", "frame", "measure":
		x, _ := length(at(n, nsSVG, "x"))
		y, _ := length(at(n, nsSVG, "y"))
		w, _ := length(at(n, nsSVG, "width"))
		h, _ := length(at(n, nsSVG, "height"))
		w, h = max(w, 0), max(h, 0)
		pts = [][2]float64{{x, y}, {x + w, y}, {x, y + h}, {x + w, y + h}}
		if t := at(n, nsDraw, "transform"); t != "" {
			// box applies the transform to the box at (x, y).
			m = parseTransform(t)
		}
	default:
		return rectangle{}, false
	}
	var box rectangle
	for i, p := range pts {
		q := okage.Point{X: float32(p[0]), Y: float32(p[1])}
		m.MapPoint(&q)
		if fx, fy := float64(q.X), float64(q.Y); i == 0 {
			box = rectangle{fx, fy, fx, fy}
		} else {
			box = box.add(fx, fy)
		}
	}
	if math.IsNaN(box.x0+box.y0+box.x1+box.y1) || math.Abs(box.x0)+math.Abs(box.x1) > 1e6 || math.Abs(box.y0)+math.Abs(box.y1) > 1e6 {
		return rectangle{}, false
	}
	return box, true
}
