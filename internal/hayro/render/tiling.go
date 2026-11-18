// Copyright (c) The Hayro Authors.
//
// Use of this source code is governed by the Apache-2.0 or MIT license that
// can be found in the LICENSE-APACHE and LICENSE-MIT files.

// This file ports hayro src/tiling.rs.

package render

import (
	"math"

	"github.com/xo/okage"
	"github.com/xo/okage/kurbo"

	"github.com/xo/ivy/internal/display"
	"github.com/xo/ivy/internal/hayro/interpret"
)

// The bounds of a cell of a tiling pattern (D18).
const (
	// maxCellSide is the width or the height of the largest pixmap of a
	// cell. Upstream stores the size in a u16.
	maxCellSide = 1 << 14
	// maxCellPixels is the number of pixels of the largest pixmap of a
	// cell.
	maxCellPixels = 1 << 22
	// maxCopiesPerAxis is the number of copies of a cell in each direction
	// that the pixmap of the cell draws.
	maxCopiesPerAxis = 256
	// maxCopies is the number of copies of a cell that the pixmap of the
	// cell draws.
	maxCopies = 4096
)

// tilingPaint is Renderer::tiling_paint in hayro src/tiling.rs. It returns
// the pixmap of a cell of the pattern, its mode of repetition, and the
// transform from the pixmap to the space of the pixels. area is the area
// to cover, in the space of the pixels. It returns false when the pattern
// draws nothing.
//
// Port: upstream returns an image of vello_cpu, which repeats or pads each
// direction on its own. A pattern of okage has one mode. The pixmap covers
// the whole area in a direction that pads, so a repetition in that direction
// does not show, and the port repeats when one direction repeats. The
// pixmap is smaller than upstream when it would pass maxCellSide or
// maxCellPixels, and the cell then draws at a lower resolution. When the
// matrix keeps the axes, the pixels of a direction that pads are the pixels
// of the device. See alignedAxes.
func (r *renderer) tilingPaint(t *interpret.TilingPattern, area kurbo.Rect, isStroke bool) (*okage.Pixmap, okage.SpreadMode, kurbo.Affine, bool) {
	if !invertible(t.Matrix) {
		return nil, 0, kurbo.Affine{}, false
	}
	area = t.Matrix.Inverse().TransformRectBbox(area)
	xAdvance, yAdvance := interpret.XYAdvances(t.Matrix)
	xScale, yScale := xAdvance.Hypot(), yAdvance.Hypot()
	alignX, alignY, aligned := alignedAxes(t.Matrix)
	var x, y tileAxis
	for i := range 8 {
		if i > 0 {
			// A smaller pixmap cannot have the pixels of the device.
			aligned = false
		}
		if aligned {
			x = newAlignedTileAxis(xScale, t.BBox.X0, t.BBox.X1, t.XStep, area.X0, area.X1, alignX)
			y = newAlignedTileAxis(yScale, t.BBox.Y0, t.BBox.Y1, t.YStep, area.Y0, area.Y1, alignY)
		} else {
			x = newTileAxis(xScale, t.BBox.X0, t.BBox.X1, t.XStep, area.X0, area.X1)
			y = newTileAxis(yScale, t.BBox.Y0, t.BBox.Y1, t.YStep, area.Y0, area.Y1)
		}
		// Port: see maxCellSide and maxCellPixels.
		fx := math.Min(1, maxCellSide/float64(x.size))
		fy := math.Min(1, maxCellSide/float64(y.size))
		if pixels := float64(x.size) * float64(y.size); pixels > maxCellPixels {
			f := math.Sqrt(maxCellPixels / pixels)
			fx, fy = math.Min(fx, f), math.Min(fy, f)
		}
		if fx >= 1 && fy >= 1 {
			break
		}
		xScale, yScale = xScale*fx*0.99, yScale*fy*0.99
	}
	if x.size == 0 || y.size == 0 || x.size > maxCellSide || y.size > maxCellSide || x.size*y.size > maxCellPixels {
		return nil, 0, kurbo.Affine{}, false
	}
	pix := r.renderTiling(t, &x, &y, isStroke)
	if pix == nil {
		return nil, 0, kurbo.Affine{}, false
	}
	spread := okage.SpreadModePad
	if x.repeat || y.repeat {
		spread = okage.SpreadModeRepeat
	}
	transform := t.Matrix.
		Mul(kurbo.AffineTranslate(kurbo.NewVec2(x.start, y.start))).
		Mul(kurbo.AffineScaleNonUniform(1/x.scale, 1/y.scale))
	return pix, spread, transform, true
}

// renderTiling is Renderer::render_tiling in hayro src/tiling.rs. It draws
// the copies of the cell that cover the pixmap.
func (r *renderer) renderTiling(t *interpret.TilingPattern, x, y *tileAxis, isStroke bool) *okage.Pixmap {
	toTexture := kurbo.AffineScaleNonUniform(x.scale, y.scale).
		Mul(kurbo.AffineTranslate(kurbo.NewVec2(-x.start, -y.start)))
	pix, ok := okage.NewPixmap(x.size, y.size)
	if !ok {
		return nil
	}
	renderer := newRenderer(display.NewOkage(pix, okage.IdentityTransform()), float64(x.size), float64(y.size), r.global)
	renderer.insidePattern = true
	// Port: see maxCopiesPerAxis and maxCopies. The copies nearest to the
	// end of each range stay.
	x.first = max(x.first, x.last-maxCopiesPerAxis+1)
	y.first = max(y.first, y.last-maxCopiesPerAxis+1)
	if n := (x.last - x.first + 1) * (y.last - y.first + 1); n > maxCopies {
		y.first = max(y.first, y.last-maxCopies/(x.last-x.first+1)+1)
	}
	for j := y.first; j <= y.last; j++ {
		for i := x.first; i <= x.last; i++ {
			offset := kurbo.AffineTranslate(kurbo.NewVec2(float64(i)*x.step, float64(j)*y.step))
			t.Interpret(renderer, toTexture.Mul(offset), isStroke)
		}
	}
	return pix
}

// tileAxis is TileAxis in hayro src/tiling.rs. It holds the layout of the
// pixmap of a cell in one direction.
//
// Port: upstream holds the copies in a RangeInclusive, and the extend of
// peniko. The port holds the first and the last copy, and whether the
// direction repeats.
type tileAxis struct {
	start       float64
	scale       float64
	size        uint32
	step        float64
	first, last int
	repeat      bool
}

// newTileAxis is TileAxis::new in hayro src/tiling.rs. The cell spans from
// cell0 to cell1, and the area from area0 to area1.
func newTileAxis(scale, cell0, cell1 float64, step32 float32, area0, area1 float64) tileAxis {
	step := math.Abs(float64(step32))
	width := cell1 - cell0
	visible := rustMax(area1-area0, 0)
	repeat := visible >= step
	scale = rustMax(math.Round(step*scale), 1) / step
	if repeat {
		return tileAxis{
			start:  cell0,
			scale:  scale,
			size:   uint32(asU16(float32(math.Round(step * scale)))),
			step:   step,
			first:  asI32(math.Floor(-width/step)) + 1,
			last:   0,
			repeat: true,
		}
	}
	start := cell0 + math.Floor((area0-cell0)*scale)/scale
	return tileAxis{
		start: start,
		scale: scale,
		size:  uint32(asU16(float32(rustMax(math.Ceil((area1-start)*scale), 1)))),
		step:  step,
		first: asI32(math.Ceil((area0 - cell1) / step)),
		last:  asI32(math.Floor((area1 - cell0) / step)),
	}
}

// deviceAxis maps one direction of the space of a pattern to one direction
// of the device. A coordinate p of the pattern is at k*p + offset on the
// device.
type deviceAxis struct {
	k, offset float64
}

// alignedAxes returns the directions of the device of the two directions of
// the space of a pattern. It returns false when m rotates or skews the
// pattern, other than by a multiple of 90 degrees.
func alignedAxes(m kurbo.Affine) (deviceAxis, deviceAxis, bool) {
	a, b, c, d, e, f := m[0], m[1], m[2], m[3], m[4], m[5]
	switch {
	case b == 0 && c == 0 && a != 0 && d != 0:
		return deviceAxis{a, e}, deviceAxis{d, f}, true
	case a == 0 && d == 0 && b != 0 && c != 0:
		return deviceAxis{b, f}, deviceAxis{c, e}, true
	}
	return deviceAxis{}, deviceAxis{}, false
}

// newAlignedTileAxis is newTileAxis for a direction whose pixels can be the
// pixels of the device, along dev.
//
// Port: upstream rounds the scale so that a step of the cell is a whole
// number of pixels, and it starts the pixmap on a pixel of the space of the
// pattern. The pixmap is then drawn with a part of a pixel of offset and a
// scale near 1, and its sampling blurs the thin lines of an image, such as
// those of pdfjs/pdf/a1-plans.pdf. When the direction pads, the steps of
// the cell do not repeat in the pixmap. The port then keeps the scale of
// the device, and starts the pixmap on a pixel of the device.
func newAlignedTileAxis(scale, cell0, cell1 float64, step32 float32, area0, area1 float64, dev deviceAxis) tileAxis {
	axis := newTileAxis(scale, cell0, cell1, step32, area0, area1)
	if axis.repeat || !(math.Abs(dev.k) > 0) {
		return axis
	}
	u := dev.k*area0 + dev.offset
	edge := math.Floor(u)
	if dev.k < 0 {
		edge = math.Ceil(u)
	}
	start := (edge - dev.offset) / dev.k
	scale = math.Abs(dev.k)
	size := math.Ceil((area1 - start) * scale)
	if !(start <= area0) || !(size >= 1) || size > maxCellSide {
		return axis
	}
	axis.start, axis.scale, axis.size = start, scale, uint32(size)
	return axis
}

// asI32 is the cast `as i32` of Rust: it saturates, and NaN becomes 0.
func asI32(x float64) int {
	switch {
	case math.IsNaN(x):
		return 0
	case x >= math.MaxInt32:
		return math.MaxInt32
	case x <= math.MinInt32:
		return math.MinInt32
	}
	return int(x)
}

// rustMax is f64::max of Rust: it returns the other value when one is NaN.
func rustMax(a, b float64) float64 {
	switch {
	case math.IsNaN(a):
		return b
	case math.IsNaN(b):
		return a
	}
	return math.Max(a, b)
}
