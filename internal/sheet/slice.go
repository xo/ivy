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

// objectSlice writes the slice p of an object, which shows the part of the
// object in its row. The slice reaches 1 point into the next row, which
// draws its own slice over it, so that no seam shows between the rows.
func (w *writer) objectSlice(p placed) {
	img := p.img
	obj := w.wb.Objects[img.Object]
	if obj == nil {
		return
	}
	if w.slices == nil {
		w.slices = make(map[string]layout.Object)
	}
	name := img.Object + "~" + w.id + "~" + strconv.Itoa(len(w.slices)+1)
	s := &slice{obj: obj, w: p.clipW, h: p.clipH + 1, x: p.dx, y: p.dy, ow: img.Width, oh: img.Height}
	w.slices[name] = s
	sc := w.scale
	bw, bh := s.Size()
	w.body.WriteString(`<img style="position:absolute;left:` + pt(-sliceMargin*sc) + ";top:" + pt(-sliceMargin*sc) +
		";width:" + pt(bw*sc) + ";height:" + pt(bh*sc) + `" src="` + html.EscapeString(layout.ObjectURL(name)) + `" alt="">`)
}

// slice is the part of an object in a row: the box of w by h points at the
// corner of the cell. The object takes the box of ow by oh points at (x, y)
// from the corner of the cell.
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
