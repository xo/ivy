// Copyright (c) The Hayro Authors.
//
// Use of this source code is governed by the Apache-2.0 or MIT license that
// can be found in the LICENSE-APACHE and LICENSE-MIT files.

// This file ports hayro-interpret src/mesh.rs.

package interpret

import (
	"math"
	"sort"

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
//
// Port: upstream draws into a Canvas. The port draws into a meshTarget: a
// canvas, or a meshEmitter that gives the triangles to okage.
func drawMesh(c meshTarget, shadingType ShadingType, transform kurbo.Affine, pattern *EncodedShadingPattern) {
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
			depth = meshDepth(c.bounds(), pieces, c.depthLimits())
		}
		drawTriangles(c, triangles, depth, colors)
	case *PatchMeshShading:
		patches := make([]canvasPatch, len(st.Patches))
		pieces := make([]meshPiece, len(st.Patches))
		for i := range st.Patches {
			patches[i] = newCanvasPatch(&st.Patches[i], transform)
			pieces[i] = meshPiece{extent: patches[i].extent(), bounds: patches[i].bounds()}
		}
		depth := meshDepth(c.bounds(), pieces, c.depthLimits())
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
// depth of the subdivision, so that a piece is about limits.segment pixels
// long, and the pieces in the canvas stay under limits.maxPieces.
//
// Port: upstream has the constants MESH_SEGMENT and MAX_PIECES. The port
// takes them from the target, because a meshEmitter needs fewer pieces.
func meshDepth(canvasRect kurbo.Rect, pieces []meshPiece, limits meshLimits) uint32 {
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
	depth := asU32(rustClamp(math.Ceil(math.Log2(maxExtent/limits.segment)), 0, meshMaxDepth))
	for depth > 0 && visibleArea*math.Pow(4, float64(depth)) > limits.maxPieces {
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
	lut *colorLut
	// stops are the stops of lut.
	//
	// Port: upstream keeps the table only. A meshEmitter cuts the triangles
	// at the stops.
	stops []SVGGradientStop
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
	stops := functionStops(function, [2]float32{t0, t1}, pattern)
	return &parameter{lut: newColorLut(stops), stops: stops, t0: t0, scale: scale}
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
func drawTriangles(c meshTarget, triangles []canvasTriangle, depth uint32, colors meshColors) {
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
func drawTrianglePiece(c meshTarget, triangle *canvasTriangle, points [3]kurbo.Point, weights [3][3]float32, depth uint32, colors meshColors) {
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
		c.fillPiece(points, values, colors)
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
func drawPatches(c meshTarget, patches []canvasPatch, depth uint32, colors meshColors) {
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
func drawPatchPiece(c meshTarget, patch *canvasPatch, points [4][4]kurbo.Point, ranges [2][2]float32, depths [2]uint32, colors meshColors) {
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
		c.fillPiece([3]kurbo.Point{p00, p10, p01}, [3][]float32{c00, c10, c01}, colors)
		c.fillPiece([3]kurbo.Point{p10, p11, p01}, [3][]float32{c10, c11, c01}, colors)
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

// meshTarget is where drawMesh puts the pieces of a mesh.
//
// Port: upstream draws a mesh into a Canvas only. A meshEmitter takes the
// triangles for a MeshGradient of okage instead (okage D55).
type meshTarget interface {
	// bounds returns the area of the target.
	bounds() kurbo.Rect
	// isVisible reports whether the bounds of the points overlap the area
	// of the target.
	isVisible(points []kurbo.Point) bool
	// stopped reports whether the drawing must stop.
	stopped() bool
	// depthLimits returns the limits of meshDepth.
	depthLimits() meshLimits
	// fillPiece fills a triangle with the values of colors at its corners.
	fillPiece(points [3]kurbo.Point, values [3][]float32, colors meshColors)
}

// meshLimits are the limits of the subdivision of meshDepth: the length of
// a piece, and the largest number of pieces.
type meshLimits struct {
	segment, maxPieces float64
}

func (c *canvas) depthLimits() meshLimits {
	return meshLimits{segment: meshSegment, maxPieces: meshMaxPieces}
}

func (c *canvas) fillPiece(points [3]kurbo.Point, values [3][]float32, colors meshColors) {
	c.fillTriangle(points, values, colors.shade)
}

// maxMeshTriangles is the largest number of triangles that MeshTriangles
// gives for one shading. okage keeps about 250 bytes for each triangle, so
// the triangles of one shading take at most about 64 MiB.
const maxMeshTriangles = 1 << 18

// maxMeshParts is the largest number of parts that cutParameter cuts for
// one shading. A part outside the area gives no triangle, so maxMeshTriangles
// alone does not bound the work.
const maxMeshParts = 1 << 22

// meshEmitterSegment is the length in pixels of a piece of a mesh for a
// meshEmitter.
const meshEmitterSegment = 16.0

// meshClipMargin is the distance in pixels that a triangle can reach past
// the area of a meshEmitter before the emitter cuts it. okage computes in
// float32, and the cut keeps the coordinates small.
const meshClipMargin = 1 << 12

// MeshTriangle is a triangle of a mesh shading in the space of the device.
//
// Port: upstream has no such type. See MeshTriangles.
type MeshTriangle struct {
	// Points are the corners of the triangle.
	Points [3]kurbo.Point
	// Colors are the straight colors of RGBA at the corners. The color
	// varies linearly in the triangle.
	Colors [3][4]float32
}

// MeshTriangles calls emit with the triangles of a mesh shading, a shading
// of type 4 to 7, in the space of the device. The triangles cover the part
// of the shading that overlaps bbox, and a later triangle paints over an
// earlier one. If the shading has a background, the first two triangles
// cover bbox with its color. MeshTriangles returns false when the shading
// is not a mesh.
//
// A function of the colors is not linear in a triangle. MeshTriangles thus
// cuts each triangle at the stops of the function, where the colors change
// from one linear segment to the next. It gives at most maxMeshTriangles
// triangles.
//
// Port: upstream samples a mesh into a texture with sample_texture. okage
// fills the triangles with MeshGradient (okage D55).
func (p *EncodedShadingPattern) MeshTriangles(bbox kurbo.Rect, emit func(MeshTriangle)) bool {
	s, ok := p.ShadingType.(*EncodedMeshShading)
	if !ok {
		return false
	}
	if !(bbox.Width() > 0 && bbox.Height() > 0) {
		return true
	}
	e := &meshEmitter{
		area:  bbox,
		limit: kurboext.RectInflate(bbox, meshClipMargin, meshClipMargin),
		emit:  emit,
		run:   p.run,
	}
	if bg := p.backgroundColor.Components(); bg[3] > 0 {
		e.fillArea(p.finish(bg))
	}
	drawMesh(e, s.shadingType, p.BaseTransform.Inverse(), p)
	return true
}

// meshEmitter is a meshTarget that gives the triangles of a mesh to a
// function, for MeshTriangles.
type meshEmitter struct {
	// area is the area of the target, and limit is the area that the
	// triangles stay in.
	area, limit kurbo.Rect
	emit        func(MeshTriangle)
	count       int
	run         *runState
	work        int
	// cuts holds the values at which cutParameter cuts a triangle, and
	// parts counts the parts that it cut.
	cuts  []float64
	parts int
}

func (e *meshEmitter) bounds() kurbo.Rect { return e.area }

func (e *meshEmitter) isVisible(points []kurbo.Point) bool {
	return kurboext.RectOverlaps(pointsBounds(points), e.area)
}

// depthLimits gives pieces of meshEmitterSegment pixels. okage interpolates
// the colors in each triangle, so a piece only has to follow the curves of
// a patch and the colors of a color space that is not linear. A piece of a
// patch is two triangles.
func (e *meshEmitter) depthLimits() meshLimits {
	return meshLimits{segment: meshEmitterSegment, maxPieces: maxMeshTriangles / 2}
}

// stopped works as canvas.stopped does, and also stops at
// maxMeshTriangles triangles.
func (e *meshEmitter) stopped() bool {
	if e.count >= maxMeshTriangles {
		return true
	}
	e.work++
	if e.work%cancelInterval != 0 {
		return e.run != nil && e.run.err != nil
	}
	return e.run.canceled()
}

func (e *meshEmitter) fillPiece(points [3]kurbo.Point, values [3][]float32, colors meshColors) {
	if p, ok := colors.(*parameter); ok {
		e.cutParameter(points, values, p)
		return
	}
	var c [3][4]float32
	for i := range values {
		c[i] = colors.shade(values[i])
	}
	e.add(points, c)
}

// fillArea gives two triangles that cover the area with color.
func (e *meshEmitter) fillArea(color [4]float32) {
	a := e.area
	p00, p10 := kurbo.NewPoint(a.X0, a.Y0), kurbo.NewPoint(a.X1, a.Y0)
	p01, p11 := kurbo.NewPoint(a.X0, a.Y1), kurbo.NewPoint(a.X1, a.Y1)
	c := [3][4]float32{color, color, color}
	e.add([3]kurbo.Point{p00, p10, p01}, c)
	e.add([3]kurbo.Point{p10, p11, p01}, c)
}

// cutParameter gives a triangle whose corners hold the values of p. The
// value varies linearly in the triangle, and the color is linear between
// two stops of p. cutParameter thus cuts the triangle along the lines where
// the value is at a stop, and gives the triangles of each part.
func (e *meshEmitter) cutParameter(points [3]kurbo.Point, values [3][]float32, p *parameter) {
	stops := p.stops
	if len(stops) == 0 {
		var c [3][4]float32
		for i := range values {
			c[i] = p.shade(values[i])
		}
		e.add(points, c)
		return
	}
	var tri [3]cutVertex
	lo, hi := math.Inf(1), math.Inf(-1)
	for i := range tri {
		t := float64(values[i][0])
		switch {
		case !(t > 0):
			t = 0
		case t > 1:
			t = 1
		}
		tri[i] = cutVertex{p: points[i], v: [4]float64{t}}
		lo, hi = math.Min(lo, t), math.Max(hi, t)
	}
	e.cuts = append(e.cuts[:0], lo)
	for i := stopAfter(stops, lo); i < len(stops) && float64(stops[i].Offset) < hi; i++ {
		if o := float64(stops[i].Offset); o > e.cuts[len(e.cuts)-1] {
			e.cuts = append(e.cuts, o)
		}
	}
	e.cuts = append(e.cuts, hi)
	cuts := e.cuts
	var buf, scratch [8]cutVertex
	for k := 0; k+1 < len(cuts) && e.parts < maxMeshParts && !e.stopped(); k++ {
		e.parts++
		a, b := cuts[k], cuts[k+1]
		poly, other := append(buf[:0], tri[:]...), scratch[:0]
		if k > 0 {
			poly, other = cutPolygon(poly, other, func(v *cutVertex) float64 { return v.v[0] - a }), poly[:0]
		}
		if k+2 < len(cuts) {
			poly = cutPolygon(poly, other, func(v *cutVertex) float64 { return b - v.v[0] })
		}
		i := stopAfter(stops, 0.5*(a+b)) - 1
		for j := 1; j+1 < len(poly); j++ {
			e.add([3]kurbo.Point{poly[0].p, poly[j].p, poly[j+1].p}, [3][4]float32{
				stopsColor(stops, i, poly[0].v[0]),
				stopsColor(stops, i, poly[j].v[0]),
				stopsColor(stops, i, poly[j+1].v[0]),
			})
		}
	}
}

// stopAfter returns the index of the first stop whose offset is above t.
func stopAfter(stops []SVGGradientStop, t float64) int {
	return sort.Search(len(stops), func(i int) bool { return float64(stops[i].Offset) > t })
}

// stopsColor returns the color at t on the segment of the stops from the
// stop i to the stop i+1. Out of the stops, the color is the color of the
// nearest stop.
func stopsColor(stops []SVGGradientStop, i int, t float64) [4]float32 {
	switch {
	case i < 0:
		return stops[0].Color
	case i+1 >= len(stops):
		return stops[len(stops)-1].Color
	}
	s0, s1 := stops[i], stops[i+1]
	width := float64(s1.Offset - s0.Offset)
	if !(width > 0) {
		return s0.Color
	}
	return lerpColor(s0.Color, s1.Color, float32(rustClamp((t-float64(s0.Offset))/width, 0, 1)))
}

// add gives a triangle that overlaps the area. It cuts a triangle that
// reaches past the limit.
func (e *meshEmitter) add(points [3]kurbo.Point, colors [3][4]float32) {
	b := pointsBounds(points[:])
	if !kurboext.RectOverlaps(b, e.area) {
		return
	}
	l := e.limit
	if b.X0 >= l.X0 && b.Y0 >= l.Y0 && b.X1 <= l.X1 && b.Y1 <= l.Y1 {
		e.put(points, colors)
		return
	}
	var buf, scratch [8]cutVertex
	poly := buf[:0]
	for i := range points {
		v := cutVertex{p: points[i]}
		for k := range v.v {
			v.v[k] = float64(colors[i][k])
		}
		poly = append(poly, v)
	}
	poly = cutPolygon(poly, scratch[:0], func(v *cutVertex) float64 { return v.p.X - l.X0 })
	poly = cutPolygon(poly, buf[:0], func(v *cutVertex) float64 { return l.X1 - v.p.X })
	poly = cutPolygon(poly, scratch[:0], func(v *cutVertex) float64 { return v.p.Y - l.Y0 })
	poly = cutPolygon(poly, buf[:0], func(v *cutVertex) float64 { return l.Y1 - v.p.Y })
	color := func(v *cutVertex) [4]float32 {
		return [4]float32{float32(v.v[0]), float32(v.v[1]), float32(v.v[2]), float32(v.v[3])}
	}
	for j := 1; j+1 < len(poly); j++ {
		e.put([3]kurbo.Point{poly[0].p, poly[j].p, poly[j+1].p}, [3][4]float32{color(&poly[0]), color(&poly[j]), color(&poly[j+1])})
	}
}

// put gives a triangle to the function of the emitter, up to
// maxMeshTriangles triangles.
func (e *meshEmitter) put(points [3]kurbo.Point, colors [3][4]float32) {
	if e.count >= maxMeshTriangles {
		return
	}
	e.count++
	e.emit(MeshTriangle{Points: points, Colors: colors})
}

// cutVertex is a corner of a polygon that cutPolygon cuts, with values
// that vary linearly in the polygon.
type cutVertex struct {
	p kurbo.Point
	v [4]float64
}

// cutPolygon appends to out the part of the convex polygon in where f is 0
// or more, and returns it. f must be linear. out must not share memory
// with in.
//
// A point on an edge depends only on the two ends of the edge, and not on
// their order. Two polygons that share an edge thus cut it at the same
// point, and the triangles meet with no gap in okage.
func cutPolygon(in, out []cutVertex, f func(*cutVertex) float64) []cutVertex {
	if len(in) == 0 {
		return out
	}
	prev := in[len(in)-1]
	fPrev := f(&prev)
	for _, cur := range in {
		fCur := f(&cur)
		if (fCur >= 0) != (fPrev >= 0) {
			out = append(out, cutEdge(prev, cur, fPrev, fCur))
		}
		if fCur >= 0 {
			out = append(out, cur)
		}
		prev, fPrev = cur, fCur
	}
	return out
}

// cutEdge returns the point where f is 0 on the edge from a to b, where f
// is fa at a and fb at b. It orders the ends by x and then by y, so that
// the point does not depend on the direction of the edge.
func cutEdge(a, b cutVertex, fa, fb float64) cutVertex {
	if b.p.X < a.p.X || (b.p.X == a.p.X && b.p.Y < a.p.Y) {
		a, b, fa, fb = b, a, fb, fa
	}
	t := fa / (fa - fb)
	out := cutVertex{p: kurbo.NewPoint(a.p.X+t*(b.p.X-a.p.X), a.p.Y+t*(b.p.Y-a.p.Y))}
	for i := range out.v {
		out.v[i] = a.v[i] + t*(b.v[i]-a.v[i])
	}
	return out
}
