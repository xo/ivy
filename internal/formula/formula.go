// Package formula lays out the equations of the formats with the layout of
// math of okage, the package texmath (okage D78), and draws them on the
// display list. A format builds a tree of texmath from its equation, such as
// an equation of Office Math with OMML. A Typesetter lays out the tree, and
// the result is an Object that the layout of HTML and CSS places as an
// object of a Source, as it places a chart (D62).
//
// The equations take STIX Two Math, the math font that okage holds. The
// glyphs are drawn as runs of glyphs with their text, so that a writer of
// PDF keeps the text of an equation.
package formula

import (
	"context"
	"fmt"
	"sync"

	"github.com/xo/okage"
	"github.com/xo/okage/texmath"
	"github.com/xo/okage/texmath/stixtwomath"
	gofont "github.com/xo/okage/typesetting/font"

	"github.com/xo/ivy"
	"github.com/xo/ivy/internal/display"
	"github.com/xo/ivy/internal/font"
)

// The limits of one equation. A document holds many equations, so an
// equation takes less than the limits of texmath.
const (
	// maxDepth is the depth of a tree that Layout lays out.
	maxDepth = 64
	// maxItems is the number of items of the box of one equation.
	maxItems = 200000
)

// drawFace returns the face of ivy that draws the glyphs of STIX Two Math,
// loaded once. It is nil when the font cannot be read.
var drawFace = sync.OnceValue(func() *font.Face {
	f, err := font.Load(stixtwomath.Data(), 0)
	if err != nil {
		return nil
	}
	return f
})

// Typesetter lays out equations with STIX Two Math. It is not safe for
// concurrent use, because the face of okage that it reads is not. The
// Objects that it returns are safe for concurrent use.
type Typesetter struct {
	face *gofont.Face
}

// New returns a Typesetter. It returns an error that wraps ivy.ErrFormat if
// the math font cannot be read.
func New() (*Typesetter, error) {
	f, err := stixtwomath.Font()
	if err != nil {
		return nil, fmt.Errorf("reading the math font: %w: %w", ivy.ErrFormat, err)
	}
	return &Typesetter{face: gofont.NewFace(f)}, nil
}

// Layout lays out the tree n at the size, in points, in the style, such as
// texmath.Display for an equation on a line of its own. color is the color
// of the items that the tree does not color. It returns an error that wraps
// ivy.ErrLimit when the tree goes past a limit.
func (t *Typesetter) Layout(n texmath.Node, size float64, style texmath.Style, color okage.Color) (*Object, error) {
	if !(size > 0) || size > 1638 {
		size = 10
	}
	box, err := texmath.Layout(n, t.face, size, style,
		texmath.WithMaxDepth(maxDepth),
		texmath.WithColor(color),
	)
	if err != nil {
		return nil, fmt.Errorf("laying out an equation: %w: %w", ivy.ErrLimit, err)
	}
	if len(box.Items) > maxItems {
		return nil, fmt.Errorf("equation of %d items: %w", len(box.Items), ivy.ErrLimit)
	}
	o := &Object{box: box, draw: drawFace()}
	// A glyph that the face of ivy cannot draw is drawn as its outline,
	// which is read here, because Draw must not read the face of okage.
	for i := range box.Items {
		it := &box.Items[i]
		if it.Kind == texmath.ItemGlyph && (o.draw == nil || it.Face != t.face) {
			if o.paths == nil {
				o.paths = make(map[int]*okage.Path)
			}
			o.paths[i] = box.ItemPath(i)
		}
	}
	return o, nil
}

// Object is an equation laid out. It is a picture that a format draws
// itself, which satisfies the interface Object of the package layout. Its
// methods are safe for concurrent use.
type Object struct {
	box *texmath.Box
	// draw is the face that draws the glyphs of the math font.
	draw *font.Face
	// paths holds the outlines of the glyphs that draw cannot draw, by the
	// index of their item.
	paths map[int]*okage.Path
}

// Size returns the width of the equation, and its height above the baseline
// with its depth below it, in points.
func (o *Object) Size() (width, height float64) {
	return o.box.Width, o.box.Height + o.box.Depth
}

// Depth returns the depth of the equation below its baseline, in points. A
// format moves the object down by its depth, so that the baseline of the
// equation sits on the baseline of the line.
func (o *Object) Depth() float64 {
	return o.box.Depth
}

// Draw draws the equation onto dev, scaled to fill rect.
func (o *Object) Draw(ctx context.Context, dev display.Device, rect okage.Rect) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	w, h := o.Size()
	if !(w > 0) || !(h > 0) {
		return nil
	}
	sx := rect.Width() / float32(w)
	sy := rect.Height() / float32(h)
	dev.Save()
	defer dev.Restore()
	dev.Concat(okage.TransformFromRow(sx, 0, 0, sy, rect.Left(), rect.Top()+float32(o.box.Height)*sy))
	for i := range o.box.Items {
		o.item(dev, i)
	}
	return nil
}

// item draws the item i of the box.
func (o *Object) item(dev display.Device, i int) {
	it := &o.box.Items[i]
	shader := okage.SolidColor(it.Color)
	if !it.Transform.IsIdentity() {
		dev.Save()
		defer dev.Restore()
		dev.Concat(it.Transform)
	}
	switch it.Kind {
	case texmath.ItemGlyph:
		if p, ok := o.paths[i]; ok {
			if p != nil {
				dev.FillPath(p, okage.FillRuleWinding, shader)
			}
			return
		}
		dev.FillGlyphs(display.GlyphRun{
			Face:      o.draw,
			Size:      float32(it.Size),
			Glyphs:    []display.Glyph{{ID: font.GID(it.Glyph), X: float32(it.X), Y: float32(it.Y), End: len(it.Text)}},
			Transform: okage.IdentityTransform(),
			Text:      it.Text,
		}, shader)
	case texmath.ItemRule:
		if r, ok := okage.RectFromXYWH(float32(it.X), float32(it.Y), float32(it.W), float32(it.H)); ok {
			dev.FillPath(okage.PathFromRect(r), okage.FillRuleWinding, shader)
		}
	case texmath.ItemPath:
		if it.Path != nil {
			dev.Save()
			dev.Concat(okage.TransformFromTranslate(float32(it.X), float32(it.Y)))
			dev.FillPath(it.Path, okage.FillRuleWinding, shader)
			dev.Restore()
		}
	}
}
