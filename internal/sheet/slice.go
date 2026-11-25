package sheet

import (
	"context"
	"html"
	"math"
	"strconv"

	"github.com/xo/okage"

	"github.com/xo/ivy/internal/display"
	"github.com/xo/ivy/internal/layout"
)

// This file draws the slice of an object, such as a chart, in a row of a
// sheet.
//
// An object draws more than one fill on a pixel: a chart fills its area,
// then its plot area, then its series. A clip applies to each fill on its
// own. On the edge of a clip that covers a part of a pixel, each fill covers
// the same part of the pixel, and the fills under the last one show through
// it. Where the slice of a row meets the slice of the next row, such an
// edge of the next slice draws a light seam across the chart. So a slice
// clips its object on the grid of the pixels (display.ClipAdjusted), and no
// edge of the clip covers a part of a pixel. A writer of PDF gets a plain
// clip, which a viewer draws as one.

// sliceMargin is the margin around a slice, in points. The layout clips
// the box of an image, and the box of a slice holds the slice and the
// margin, so that the clip cuts no pixel that the mask keeps, at 18 dots
// per inch and more.
const sliceMargin = 4

// sliceReach is the height of the part of the row above that a slice
// covers, in points. The row draws its cell, with the gridline at its top,
// after the row above. The slice draws over the gridline and over the
// edge of the slice of the row above, so that no seam shows between the
// rows.
const sliceReach = 1

// objectSlice writes the slice p of an object, which shows the part of the
// object in its row.
func (w *writer) objectSlice(p placed) {
	img := p.img
	obj := w.wb.Objects[img.Object]
	if obj == nil {
		return
	}
	w.slice(obj, img.Object, p.dx, p.dy, img.Width, img.Height, p)
}

// frameSlice writes the slice p of the frame of a picture that ivy does
// not draw, as objectSlice writes the slice of an object. The fill and the
// border of the frame are an object. The label goes over them, in a clip of
// the layout that reaches into the row above as the slice does.
func (w *writer) frameSlice(p placed) {
	img := p.img
	// the border of div.ph lies outside the box of the frame
	f := frame{w: img.Width + 2*frameBorder, h: img.Height + 2*frameBorder}
	w.slice(f, "frame", p.dx, p.dy, f.w, f.h, p)
	s := w.scale
	b := &w.body
	b.WriteString(`<div class="cl" style="top:` + pt(-sliceReach*s) + ";width:" + pt(p.clipW*s) + ";height:" + pt((p.clipH+sliceReach)*s) + `">`)
	b.WriteString(`<div class="ph pl" style="left:` + pt(p.dx*s) + ";top:" + pt((p.dy+sliceReach)*s) + ";width:" + pt(img.Width*s) +
		";height:" + pt(img.Height*s) + `">` + html.EscapeString(img.Label) + `</div>`)
	b.WriteString("</div>")
}

// slice writes the slice p of the object obj, which takes the box of ow by
// oh points at (x, y) from the corner of the cell. key names the object.
func (w *writer) slice(obj layout.Object, key string, x, y, ow, oh float64, p placed) {
	if w.slices == nil {
		w.slices = make(map[string]layout.Object)
	}
	name := key + "~" + w.id + "~" + strconv.Itoa(len(w.slices)+1)
	s := &slice{obj: obj, w: p.clipW, h: p.clipH + sliceReach, x: x, y: y + sliceReach, ow: ow, oh: oh}
	w.slices[name] = s
	sc := w.scale
	bw, bh := s.Size()
	w.body.WriteString(`<img style="position:absolute;left:` + pt(-sliceMargin*sc) + ";top:" + pt(-(sliceMargin+sliceReach)*sc) +
		";width:" + pt(bw*sc) + ";height:" + pt(bh*sc) + `" src="` + html.EscapeString(layout.ObjectURL(name)) + `" alt="">`)
}

// slice is the part of an object in a row: the box of w by h points from
// sliceReach above the corner of the cell. The object takes the box of ow by
// oh points at (x, y) from the corner of that box.
type slice struct {
	obj          layout.Object
	w, h         float64
	x, y, ow, oh float64
}

// Size satisfies the interface Object of the layout. The size holds the
// slice and its margin.
func (s *slice) Size() (float64, float64) {
	return s.w + 2*sliceMargin, s.h + 2*sliceMargin
}

// Draw satisfies the interface Object of the layout. It draws the part of
// the object inside the slice.
func (s *slice) Draw(ctx context.Context, dev display.Device, rect okage.Rect) error {
	bw, bh := s.Size()
	rw, rh := float64(rect.Width()), float64(rect.Height())
	if !(rw > 0) || !(rh > 0) || math.IsInf(rw, 0) || math.IsInf(rh, 0) {
		return nil
	}
	keep, ok := okage.RectFromXYWH(sliceMargin, sliceMargin, float32(s.w), float32(s.h))
	if !ok {
		return nil
	}
	box, ok := okage.RectFromXYWH(float32(s.x+sliceMargin), float32(s.y+sliceMargin), float32(s.ow), float32(s.oh))
	if !ok {
		return nil
	}
	dev.Save()
	defer dev.Restore()
	dev.Concat(okage.TransformFromRow(float32(rw/bw), 0, 0, float32(rh/bh), rect.X(), rect.Y()))
	display.ClipAdjusted(dev, keep)
	return s.obj.Draw(ctx, dev, box)
}

// frameBorder is the width of the border of a frame, in points.
const frameBorder = 0.75

// frame is the fill and the border of the frame of a picture that ivy does
// not draw, as div.ph draws them: a gray fill and a border of 0.75 points
// inside the box of w by h points.
type frame struct {
	w, h float64
}

// Size satisfies the interface Object of the layout.
func (f frame) Size() (float64, float64) {
	return f.w, f.h
}

// Draw satisfies the interface Object of the layout.
func (f frame) Draw(_ context.Context, dev display.Device, rect okage.Rect) error {
	if r, ok := okage.RectFromXYWH(rect.X(), rect.Y(), rect.Width(), rect.Height()); ok {
		dev.FillPath(okage.PathFromRect(r), okage.FillRuleWinding, okage.SolidColor(okage.ColorFromRGBA8(0xf2, 0xf2, 0xf2, 0xff)))
	}
	sx, sy := float64(rect.Width())/f.w, float64(rect.Height())/f.h
	in := frameBorder / 2
	r, ok := okage.RectFromXYWH(rect.X()+float32(in*sx), rect.Y()+float32(in*sy),
		rect.Width()-float32(2*in*sx), rect.Height()-float32(2*in*sy))
	if !ok {
		return nil
	}
	stroke := okage.Stroke{Width: float32(frameBorder * min(sx, sy)), MiterLimit: 4}
	dev.StrokePath(okage.PathFromRect(r), stroke, okage.SolidColor(okage.ColorFromRGBA8(0x80, 0x80, 0x80, 0xff)))
	return nil
}
