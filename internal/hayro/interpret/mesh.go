// Copyright (c) The Hayro Authors.
//
// Use of this source code is governed by the Apache-2.0 or MIT license that
// can be found in the LICENSE-APACHE and LICENSE-MIT files.

// This file ports hayro-interpret src/mesh.rs.

package interpret

import (
	"math"

	"github.com/xo/okage/kurbo"

	"github.com/xo/ivy/internal/hayro/kurboext"
)

// The constants of hayro-interpret src/mesh.rs.
const (
	meshSegment   = 8.0
	meshMaxDepth  = 10
	meshMaxPieces = float64(1 << 20)
)

// drawMesh is draw_mesh in hayro-interpret src/mesh.rs. It draws the
// triangles or the patches of a mesh into the canvas.
func drawMesh(c *canvas, shadingType ShadingType, transform kurbo.Affine, pattern *EncodedShadingPattern) {
	switch st := shadingType.(type) {
	case *TriangleMeshShading:
		triangles := make([]canvasTriangle, len(st.Triangles))
		for i := range st.Triangles {
			triangles[i] = newCanvasTriangle(&st.Triangles[i], transform)
		}
		if st.Function != nil {
			var corners []ColorComponents
			for i := range triangles {
				corners = append(corners, triangles[i].colors[:]...)
			}
			colors := newParameter(st.Function, corners, pattern)
			drawTriangles(c, triangles, 0, colors)
			return
		}
		colors := &direct{pattern: pattern}
		depth := uint32(0)
		if !colors.isLinear() {
			pieces := make([]meshPiece, len(triangles))
			for i := range triangles {
				pieces[i] = meshPiece{extent: triangles[i].extent(), bounds: triangles[i].bounds()}
			}
			depth = meshDepth(c.bounds(), pieces)
		}
		drawTriangles(c, triangles, depth, colors)
	case *PatchMeshShading:
		patches := make([]canvasPatch, len(st.Patches))
		pieces := make([]meshPiece, len(st.Patches))
		for i := range st.Patches {
			patches[i] = newCanvasPatch(&st.Patches[i], transform)
			pieces[i] = meshPiece{extent: patches[i].extent(), bounds: patches[i].bounds()}
		}
		depth := meshDepth(c.bounds(), pieces)
		if st.Function != nil {
			var corners []ColorComponents
			for i := range patches {
				for _, col := range patches[i].colors {
					corners = append(corners, col[:]...)
				}
			}
			drawPatches(c, patches, depth, newParameter(st.Function, corners, pattern))
			return
		}
		drawPatches(c, patches, depth, &direct{pattern: pattern})
	}
}

// meshPiece is the extent and the bounds of a triangle or a patch, as
// mesh_depth in hayro-interpret src/mesh.rs takes them.
type meshPiece struct {
	extent float64
	bounds kurbo.Rect
}

// meshDepth is mesh_depth in hayro-interpret src/mesh.rs. It returns the
// depth of the subdivision, so that a piece is about 8 pixels long, and the
// pieces in the canvas stay under a limit.
func meshDepth(canvasRect kurbo.Rect, pieces []meshPiece) uint32 {
	maxExtent := 0.0
	visibleArea := 0.0
	for _, p := range pieces {
		if !kurboext.RectOverlaps(p.bounds, canvasRect) {
			continue
		}
		area := kurboext.RectArea(p.bounds)
		maxExtent = rustMax(maxExtent, p.extent)
		if area > 0 {
			visibleArea += kurboext.RectArea(kurboext.RectIntersect(p.bounds, canvasRect)) / area
		} else {
			visibleArea++
		}
	}
	depth := asU32(rustClamp(math.Ceil(math.Log2(maxExtent/meshSegment)), 0, meshMaxDepth))
	for depth > 0 && visibleArea*math.Pow(4, float64(depth)) > meshMaxPieces {
		depth--
	}
	return depth
}

// meshColors is the trait MeshColors in hayro-interpret src/mesh.rs. It
// maps the components of a point to values that vary linearly, and the
// values to a color.
//
// Port: upstream has a constant N for the number of values. Go asks n.
type meshColors interface {
	n() int
	values(components []float32, out []float32)
	shade(values []float32) [4]float32
}

// parameter is Parameter in hayro-interpret src/mesh.rs. It shades a mesh
// whose colors are the input of a function.
type parameter struct {
	lut   *colorLut
	t0    float32
	scale float32
}

// newParameter is Parameter::new in hayro-interpret src/mesh.rs.
func newParameter(function *ShadingFunction, colors []ColorComponents, pattern *EncodedShadingPattern) *parameter {
	t0, t1 := float32(math.Inf(1)), float32(math.Inf(-1))
	for _, c := range colors {
		if len(c) > 0 {
			t0, t1 = rustMin(t0, c[0]), rustMax(t1, c[0])
		}
	}
	if !(t0 <= t1) {
		t0, t1 = 0, 0
	}
	var scale float32
	if t1 > t0 {
		scale = 1 / (t1 - t0)
	}
	return &parameter{lut: functionLut(function, [2]float32{t0, t1}, pattern), t0: t0, scale: scale}
}

func (p *parameter) n() int { return 1 }

func (p *parameter) values(components []float32, out []float32) {
	var c float32
	if len(components) > 0 {
		c = components[0]
	}
	out[0] = (c - p.t0) * p.scale
}

func (p *parameter) shade(values []float32) [4]float32 {
	return p.lut.get(values[0])
}

// direct is Direct in hayro-interpret src/mesh.rs. It shades a mesh whose
// colors are in the color space of the shading.
type direct struct {
	pattern *EncodedShadingPattern
}

// isLinear is Direct::is_linear in hayro-interpret src/mesh.rs.
func (d *direct) isLinear() bool {
	return d.pattern.colorSpace.isDeviceGrayOrRGB() && d.pattern.transferFunction == nil
}

func (d *direct) n() int { return 4 }

func (d *direct) values(components []float32, out []float32) {
	v := d.pattern.convert(components)
	copy(out, v[:])
}

func (d *direct) shade(values []float32) [4]float32 {
	return [4]float32(values)
}

// mix is mix in hayro-interpret src/mesh.rs. It returns the weighted sum of
// the colors, in out.
func mix(colors []ColorComponents, weights []float32, out *[maxComponents]float32) []float32 {
	n := maxComponents
	if len(colors) == 0 {
		n = 0
	}
	for _, c := range colors {
		n = min(n, len(c))
	}
	for i := range n {
		var sum float32
		for j, c := range colors {
			sum += c[i] * weights[j]
		}
		out[i] = sum
	}
	return out[:n]
}

// canvasTriangle is CanvasTriangle in hayro-interpret src/mesh.rs.
type canvasTriangle struct {
	points [3]kurbo.Point
	colors [3]ColorComponents
}

// newCanvasTriangle is CanvasTriangle::new in hayro-interpret src/mesh.rs.
func newCanvasTriangle(t *Triangle, transform kurbo.Affine) canvasTriangle {
	var ct canvasTriangle
	for i, v := range t.Vertices {
		ct.points[i] = transform.MulPoint(v.Point)
		ct.colors[i] = v.Colors
	}
	return ct
}

// extent is CanvasTriangle::extent in hayro-interpret src/mesh.rs.
func (t *canvasTriangle) extent() float64 {
	a, b, c := t.points[0], t.points[1], t.points[2]
	return rustMax(rustMax(a.Sub(b).Hypot(), b.Sub(c).Hypot()), c.Sub(a).Hypot())
}

// bounds is CanvasTriangle::bounds in hayro-interpret src/mesh.rs.
func (t *canvasTriangle) bounds() kurbo.Rect {
	return pointsBounds(t.points[:])
}

// drawTriangles is draw_triangles in hayro-interpret src/mesh.rs.
func drawTriangles(c *canvas, triangles []canvasTriangle, depth uint32, colors meshColors) {
	corners := [3][3]float32{{1, 0, 0}, {0, 1, 0}, {0, 0, 1}}
	for i := range triangles {
		if c.stopped() {
			return
		}
		drawTrianglePiece(c, &triangles[i], triangles[i].points, corners, depth, colors)
	}
}

// drawTrianglePiece is draw_triangle_piece in hayro-interpret src/mesh.rs.
// It splits a triangle into four until the depth is 0, and fills each
// piece.
func drawTrianglePiece(c *canvas, triangle *canvasTriangle, points [3]kurbo.Point, weights [3][3]float32, depth uint32, colors meshColors) {
	if !c.isVisible(points[:]) || c.stopped() {
		return
	}
	if depth == 0 {
		var buffer [maxComponents]float32
		var values [3][]float32
		for i, w := range weights {
			values[i] = make([]float32, colors.n())
			colors.values(mix(triangle.colors[:], w[:], &buffer), values[i])
		}
		c.fillTriangle(points, values, colors.shade)
		return
	}
	p0, p1, p2 := points[0], points[1], points[2]
	w0, w1, w2 := weights[0], weights[1], weights[2]
	m01, m12, m20 := p0.Midpoint(p1), p1.Midpoint(p2), p2.Midpoint(p0)
	mid := func(a, b [3]float32) [3]float32 {
		return [3]float32{0.5 * (a[0] + b[0]), 0.5 * (a[1] + b[1]), 0.5 * (a[2] + b[2])}
	}
	n01, n12, n20 := mid(w0, w1), mid(w1, w2), mid(w2, w0)
	pieces := [4]struct {
		points  [3]kurbo.Point
		weights [3][3]float32
	}{
		{[3]kurbo.Point{p0, m01, m20}, [3][3]float32{w0, n01, n20}},
		{[3]kurbo.Point{m01, p1, m12}, [3][3]float32{n01, w1, n12}},
		{[3]kurbo.Point{m20, m12, p2}, [3][3]float32{n20, n12, w2}},
		{[3]kurbo.Point{m01, m12, m20}, [3][3]float32{n01, n12, n20}},
	}
	for _, p := range pieces {
		drawTrianglePiece(c, triangle, p.points, p.weights, depth-1, colors)
	}
}

// canvasPatch is CanvasPatch in hayro-interpret src/mesh.rs.
type canvasPatch struct {
	points [4][4]kurbo.Point
	colors *[2][2]ColorComponents
}

// newCanvasPatch is CanvasPatch::new in hayro-interpret src/mesh.rs.
func newCanvasPatch(p *Patch, transform kurbo.Affine) canvasPatch {
	var cp canvasPatch
	for i, column := range p.ControlPoints {
		for j, pt := range column {
			cp.points[i][j] = transform.MulPoint(pt)
		}
	}
	cp.colors = &p.Colors
	return cp
}

// extent is CanvasPatch::extent in hayro-interpret src/mesh.rs. It returns
// the length of the longest column or row of control points.
func (p *canvasPatch) extent() float64 {
	length := func(points [4]kurbo.Point) float64 {
		var l float64
		for i := range 3 {
			l += points[i+1].Sub(points[i]).Hypot()
		}
		return l
	}
	e := 0.0
	for _, column := range p.points {
		e = rustMax(e, length(column))
	}
	for j := range 4 {
		e = rustMax(e, length([4]kurbo.Point{p.points[0][j], p.points[1][j], p.points[2][j], p.points[3][j]}))
	}
	return e
}

// bounds is CanvasPatch::bounds in hayro-interpret src/mesh.rs.
func (p *canvasPatch) bounds() kurbo.Rect {
	return pointsBounds(flattenPoints(&p.points))
}

// flattenPoints returns the 16 points of a patch.
func flattenPoints(points *[4][4]kurbo.Point) []kurbo.Point {
	out := make([]kurbo.Point, 0, 16)
	for _, column := range points {
		out = append(out, column[:]...)
	}
	return out
}

// components is CanvasPatch::components in hayro-interpret src/mesh.rs. It
// returns the color at (u, v), interpolated between the corners.
func (p *canvasPatch) components(u, v float32, out *[maxComponents]float32) []float32 {
	c := p.colors
	weights := []float32{(1 - u) * (1 - v), (1 - u) * v, u * (1 - v), u * v}
	return mix([]ColorComponents{c[0][0], c[0][1], c[1][0], c[1][1]}, weights, out)
}

// drawPatches is draw_patches in hayro-interpret src/mesh.rs.
func drawPatches(c *canvas, patches []canvasPatch, depth uint32, colors meshColors) {
	for i := range patches {
		if c.stopped() {
			return
		}
		drawPatchPiece(c, &patches[i], patches[i].points, [2][2]float32{{0, 1}, {0, 1}}, [2]uint32{depth, depth}, colors)
	}
}

// drawPatchPiece is draw_patch_piece in hayro-interpret src/mesh.rs. It
// splits a patch in v, then in u, until both depths are 0, and fills each
// piece as two triangles.
func drawPatchPiece(c *canvas, patch *canvasPatch, points [4][4]kurbo.Point, ranges [2][2]float32, depths [2]uint32, colors meshColors) {
	if !c.isVisible(flattenPoints(&points)) || c.stopped() {
		return
	}
	uRange, vRange := ranges[0], ranges[1]
	uDepth, vDepth := depths[0], depths[1]
	halves := func(r [2]float32) [2][2]float32 {
		m := 0.5 * (r[0] + r[1])
		return [2][2]float32{{r[0], m}, {m, r[1]}}
	}
	switch {
	case vDepth > 0:
		low, high := splitRows(points)
		h := halves(vRange)
		drawPatchPiece(c, patch, low, [2][2]float32{uRange, h[0]}, [2]uint32{uDepth, vDepth - 1}, colors)
		drawPatchPiece(c, patch, high, [2][2]float32{uRange, h[1]}, [2]uint32{uDepth, vDepth - 1}, colors)
	case uDepth > 0:
		low, high := splitColumns(points)
		h := halves(uRange)
		drawPatchPiece(c, patch, low, [2][2]float32{h[0], vRange}, [2]uint32{uDepth - 1, vDepth}, colors)
		drawPatchPiece(c, patch, high, [2][2]float32{h[1], vRange}, [2]uint32{uDepth - 1, vDepth}, colors)
	default:
		u0, u1, v0, v1 := uRange[0], uRange[1], vRange[0], vRange[1]
		var buffer [maxComponents]float32
		value := func(u, v float32) []float32 {
			out := make([]float32, colors.n())
			colors.values(patch.components(u, v, &buffer), out)
			return out
		}
		c00, c10, c01, c11 := value(u0, v0), value(u1, v0), value(u0, v1), value(u1, v1)
		p00, p10, p01, p11 := points[0][0], points[3][0], points[0][3], points[3][3]
		c.fillTriangle([3]kurbo.Point{p00, p10, p01}, [3][]float32{c00, c10, c01}, colors.shade)
		c.fillTriangle([3]kurbo.Point{p10, p11, p01}, [3][]float32{c10, c11, c01}, colors.shade)
	}
}

// splitRows is split_rows in hayro-interpret src/mesh.rs.
func splitRows(points [4][4]kurbo.Point) ([4][4]kurbo.Point, [4][4]kurbo.Point) {
	var low, high [4][4]kurbo.Point
	for i, column := range points {
		low[i], high[i] = splitCubic(column)
	}
	return low, high
}

// splitColumns is split_columns in hayro-interpret src/mesh.rs.
func splitColumns(points [4][4]kurbo.Point) ([4][4]kurbo.Point, [4][4]kurbo.Point) {
	low, high := points, points
	for j := range 4 {
		l, h := splitCubic([4]kurbo.Point{points[0][j], points[1][j], points[2][j], points[3][j]})
		for i := range 4 {
			low[i][j] = l[i]
			high[i][j] = h[i]
		}
	}
	return low, high
}

// splitCubic is split_cubic in hayro-interpret src/mesh.rs. It splits a
// cubic curve at its middle.
func splitCubic(p [4]kurbo.Point) ([4]kurbo.Point, [4]kurbo.Point) {
	p01, p12, p23 := p[0].Midpoint(p[1]), p[1].Midpoint(p[2]), p[2].Midpoint(p[3])
	p012, p123 := p01.Midpoint(p12), p12.Midpoint(p23)
	p0123 := p012.Midpoint(p123)
	return [4]kurbo.Point{p[0], p01, p012, p0123}, [4]kurbo.Point{p0123, p123, p23, p[3]}
}
