// Copyright (c) The Hayro Authors.
//
// Use of this source code is governed by the Apache-2.0 or MIT license that
// can be found in the LICENSE-APACHE and LICENSE-MIT files.

// This file ports hayro-interpret src/function_shading.rs.

package interpret

import (
	"math"

	"github.com/xo/okage/kurbo"

	"github.com/xo/ivy/pdf/internal/kurboext"
)

// The constants of hayro-interpret src/function_shading.rs.
const (
	functionCell      = 16
	functionTolerance = 0.01
	functionPixelCell = 4
)

// drawFunction is draw_function in hayro-interpret src/function_shading.rs.
// It samples a function-based shading on a grid of cells, and splits a
// cell where the colors do not vary linearly.
func drawFunction(c *canvas, domain kurbo.Rect, function *ShadingFunction, transform kurbo.Affine, pattern *EncodedShadingPattern) {
	area := kurboext.RectIntersect(transform.TransformRectBbox(domain), c.bounds())
	if area.Width() <= 0 || area.Height() <= 0 {
		return
	}
	sampler := &functionSampler{function: function, inverse: transform.Inverse(), domain: domain, pattern: pattern}
	x0, y0 := asUsize(math.Floor(area.X0)), asUsize(math.Floor(area.Y0))
	columns := (asUsize(math.Ceil(area.X1)) - x0 + functionCell - 1) / functionCell
	rows := (asUsize(math.Ceil(area.Y1)) - y0 + functionCell - 1) / functionCell
	corner := func(column, row int) [2]int {
		return [2]int{x0 + column*functionCell, y0 + row*functionCell}
	}
	sampleRow := func(row int) [][4]float32 {
		out := make([][4]float32, columns+1)
		for column := range out {
			out[column] = sampler.sample(corner(column, row))
		}
		return out
	}
	previous := sampleRow(0)
	for row := 1; row <= rows; row++ {
		if c.stopped() {
			return
		}
		current := sampleRow(row)
		for column := range columns {
			corners := [4][4]float32{previous[column], previous[column+1], current[column], current[column+1]}
			drawFunctionCell(c, sampler, corner(column, row-1), functionCell, corners)
		}
		previous = current
	}
}

// functionSampler is FunctionSampler in hayro-interpret
// src/function_shading.rs.
type functionSampler struct {
	function *ShadingFunction
	inverse  kurbo.Affine
	domain   kurbo.Rect
	pattern  *EncodedShadingPattern
}

// sample is FunctionSampler::sample in hayro-interpret
// src/function_shading.rs.
func (s *functionSampler) sample(p [2]int) [4]float32 {
	return s.sampleAt(kurbo.NewPoint(float64(p[0]), float64(p[1])))
}

// sampleAt is FunctionSampler::sample_at in hayro-interpret
// src/function_shading.rs.
func (s *functionSampler) sampleAt(point kurbo.Point) [4]float32 {
	p := s.inverse.MulPoint(point)
	return s.pattern.eval(s.function, []float32{float32(p.X), float32(p.Y)})
}

// contains is FunctionSampler::contains in hayro-interpret
// src/function_shading.rs.
func (s *functionSampler) contains(point kurbo.Point) bool {
	return kurboext.RectContains(s.domain, s.inverse.MulPoint(point))
}

// drawFunctionCell is draw_function_cell in hayro-interpret
// src/function_shading.rs.
func drawFunctionCell(c *canvas, sampler *functionSampler, origin [2]int, size int, corners [4][4]float32) {
	x, y := origin[0], origin[1]
	var inDomain [4]kurbo.Point
	for i, d := range [4][2]int{{0, 0}, {size, 0}, {0, size}, {size, size}} {
		inDomain[i] = sampler.inverse.MulPoint(kurbo.NewPoint(float64(x+d[0]), float64(y+d[1])))
	}
	if x >= c.width || y >= c.height || !kurboext.RectOverlaps(pointsBounds(inDomain[:]), sampler.domain) {
		return
	}
	inside := true
	for _, p := range inDomain {
		if !kurboext.RectContains(sampler.domain, p) {
			inside = false
		}
	}
	if size <= functionPixelCell {
		fillCell(c, sampler, origin, size, inside, func(center kurbo.Point, _ [2]float32) [4]float32 {
			return sampler.sampleAt(center)
		})
		return
	}
	half := size / 2
	c00, c10, c01, c11 := corners[0], corners[1], corners[2], corners[3]
	top := sampler.sample([2]int{x + half, y})
	left := sampler.sample([2]int{x, y + half})
	middle := sampler.sample([2]int{x + half, y + half})
	right := sampler.sample([2]int{x + size, y + half})
	bottom := sampler.sample([2]int{x + half, y + size})
	errs := [5]float32{
		colorError(top, lerpColor(c00, c10, 0.5)),
		colorError(left, lerpColor(c00, c01, 0.5)),
		colorError(right, lerpColor(c10, c11, 0.5)),
		colorError(bottom, lerpColor(c01, c11, 0.5)),
		colorError(middle, lerpColor(lerpColor(c00, c11, 0.5), lerpColor(c10, c01, 0.5), 0.5)),
	}
	var e float32
	for _, v := range errs {
		e = rustMax(e, v)
	}
	quarters := [4]struct {
		origin  [2]int
		corners [4][4]float32
	}{
		{[2]int{x, y}, [4][4]float32{c00, top, left, middle}},
		{[2]int{x + half, y}, [4][4]float32{top, c10, middle, right}},
		{[2]int{x, y + half}, [4][4]float32{left, middle, c01, bottom}},
		{[2]int{x + half, y + half}, [4][4]float32{middle, right, bottom, c11}},
	}
	for _, q := range quarters {
		if e > functionTolerance {
			drawFunctionCell(c, sampler, q.origin, half, q.corners)
			continue
		}
		q00, q10, q01, q11 := q.corners[0], q.corners[1], q.corners[2], q.corners[3]
		fillCell(c, sampler, q.origin, half, inside, func(_ kurbo.Point, uv [2]float32) [4]float32 {
			return lerpColor(lerpColor(q00, q10, uv[0]), lerpColor(q01, q11, uv[0]), uv[1])
		})
	}
}

// fillCell is fill_cell in hayro-interpret src/function_shading.rs. It sets
// each pixel of the cell that is in the domain.
func fillCell(c *canvas, sampler *functionSampler, origin [2]int, size int, inside bool, color func(kurbo.Point, [2]float32) [4]float32) {
	x, y := origin[0], origin[1]
	for row := y; row < min(y+size, c.height); row++ {
		for column := x; column < min(x+size, c.width); column++ {
			center := kurbo.NewPoint(float64(column)+0.5, float64(row)+0.5)
			if inside || sampler.contains(center) {
				position := [2]float32{
					(float32(column-x) + 0.5) / float32(size),
					(float32(row-y) + 0.5) / float32(size),
				}
				c.pixels[row*c.width+column] = color(center, position)
			}
		}
	}
}
