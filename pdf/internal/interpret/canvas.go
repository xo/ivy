// Copyright (c) The Hayro Authors.
//
// Use of this source code is governed by the Apache-2.0 or MIT license that
// can be found in the LICENSE-APACHE and LICENSE-MIT files.

// This file ports hayro-interpret src/canvas.rs.

package interpret

import (
	"math"
	"slices"

	"github.com/xo/okage/kurbo"

	"github.com/xo/ivy/pdf/internal/kurboext"
)

// canvas is Canvas in hayro-interpret src/canvas.rs. It is a grid of pixels
// of RGBA in f32, into which the shadings are sampled.
//
// Port: Go adds the state of the run, so that a long drawing can stop
// with the interpretation.
type canvas struct {
	width  int
	height int
	pixels [][4]float32
	run    *runState
	work   int
}

// newCanvas is Canvas::new in hayro-interpret src/canvas.rs.
func newCanvas(width, height uint32, background [4]float32) *canvas {
	w, h := int(width), int(height)
	pixels := make([][4]float32, w*h)
	for i := range pixels {
		pixels[i] = background
	}
	return &canvas{width: w, height: h, pixels: pixels}
}

// stopped reports whether the drawing must stop. It calls the Cancel
// function of the interpretation once for each 1024 calls.
func (c *canvas) stopped() bool {
	c.work++
	if c.work%cancelInterval != 0 {
		return c.run != nil && c.run.err != nil
	}
	return c.run.canceled()
}

// bounds is Canvas::bounds in hayro-interpret src/canvas.rs.
func (c *canvas) bounds() kurbo.Rect {
	return kurbo.NewRect(0, 0, float64(c.width), float64(c.height))
}

// isVisible is Canvas::is_visible in hayro-interpret src/canvas.rs.
func (c *canvas) isVisible(points []kurbo.Point) bool {
	return kurboext.RectOverlaps(pointsBounds(points), c.bounds())
}

// fillTriangle is Canvas::fill_triangle in hayro-interpret src/canvas.rs.
// It fills a triangle, with values that vary linearly between the corners,
// through shade.
//
// Port: upstream takes arrays of a constant length N. Go takes slices of
// the same length.
func (c *canvas) fillTriangle(points [3]kurbo.Point, values [3][]float32, shade func([]float32) [4]float32) {
	p0, p1, p2 := points[0], points[1], points[2]
	e1, e2 := p1.Sub(p0), p2.Sub(p0)
	det := e1.X*e2.Y - e1.Y*e2.X
	if det == 0 || math.IsInf(det, 0) || math.IsNaN(det) {
		return
	}
	n := len(values[0])
	dx := make([]float32, n)
	dy := make([]float32, n)
	for i := range n {
		d1 := float64(values[1][i] - values[0][i])
		d2 := float64(values[2][i] - values[0][i])
		dx[i] = float32((d1*e2.Y - d2*e1.Y) / det)
		dy[i] = float32((d2*e1.X - d1*e2.X) / det)
	}
	sorted := points
	slices.SortFunc(sorted[:], func(a, b kurbo.Point) int {
		if c := totalCmpF64(a.Y, b.Y); c != 0 {
			return c
		}
		return totalCmpF64(a.X, b.X)
	})
	top, middle, bottom := sorted[0], sorted[1], sorted[2]
	firstRow := pixelIndex(top.Y, c.height)
	endRow := pixelIndex(bottom.Y, c.height)
	value := make([]float32, n)
	for y := firstRow; y < endRow; y++ {
		centerY := float64(y) + 0.5
		long := edgeX(top, bottom, centerY)
		var short float64
		if centerY < middle.Y {
			short = edgeX(top, middle, centerY)
		} else {
			short = edgeX(middle, bottom, centerY)
		}
		start := pixelIndex(rustMin(long, short), c.width)
		end := pixelIndex(rustMax(long, short), c.width)
		if start >= end {
			continue
		}
		offsetX, offsetY := float64(start)+0.5-p0.X, centerY-p0.Y
		for i := range n {
			value[i] = values[0][i] + float32(float64(dx[i])*offsetX+float64(dy[i])*offsetY)
		}
		row := c.pixels[y*c.width+start : y*c.width+end]
		for p := range row {
			row[p] = shade(value)
			for i := range n {
				value[i] += dx[i]
			}
		}
	}
}

// pixelIndex is pixel_index in hayro-interpret src/canvas.rs.
func pixelIndex(coordinate float64, n int) int {
	return asUsize(rustClamp(math.Ceil(coordinate-0.5), 0, float64(n)))
}

// edgeX is edge_x in hayro-interpret src/canvas.rs. It returns the x of the
// edge from a to b at y.
func edgeX(a, b kurbo.Point, y float64) float64 {
	if !(a.Y < b.Y || (a.Y == b.Y && a.X <= b.X)) {
		a, b = b, a
	}
	return a.X + (y-a.Y)*(b.X-a.X)/(b.Y-a.Y)
}

// pointsBounds is bounds in hayro-interpret src/canvas.rs.
func pointsBounds(points []kurbo.Point) kurbo.Rect {
	b := kurbo.RectFromPoints(points[0], points[0])
	for _, p := range points {
		b = b.UnionPt(p)
	}
	return b
}
