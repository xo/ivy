// Copyright (c) The Hayro Authors.
//
// Use of this source code is governed by the Apache-2.0 or MIT license that
// can be found in the LICENSE-APACHE and LICENSE-MIT files.

// This file ports hayro-interpret src/shading.rs.

package interpret

import (
	"github.com/xo/okage/kurbo"

	"github.com/xo/ivy/pdf/internal/kurboext"
	"github.com/xo/ivy/pdf/internal/syntax"
)

// maxMeshPieces is the largest number of triangles or patches of one mesh.
//
// Port: upstream reads the triangles and the patches up to the end of the
// data, which can be 512 MiB. The cap keeps the memory bounded (D18).
const maxMeshPieces = 1 << 20

// ShadingFunction is ShadingFunction in hayro-interpret src/shading.rs. It
// is one function for all the components, or one function of one output
// for each component.
//
// Port: upstream is an enum. Go sets Single, or Multiple.
type ShadingFunction struct {
	// Single is the function of ShadingFunction::Single, or nil.
	Single *Function
	// Multiple holds the functions of ShadingFunction::Multiple.
	Multiple []*Function
}

// NumOutputs is ShadingFunction::num_outputs in hayro-interpret
// src/shading.rs.
func (f *ShadingFunction) NumOutputs() int {
	if f.Single != nil {
		return f.Single.NumOutputs()
	}
	return len(f.Multiple)
}

// Eval is ShadingFunction::eval in hayro-interpret src/shading.rs. It
// evaluates the function on one sample.
func (f *ShadingFunction) Eval(input, output []float32) bool {
	if f.Single != nil {
		return f.Single.Eval(input, output)
	}
	if len(f.Multiple) != len(output) {
		return false
	}
	for i, fn := range f.Multiple {
		if !fn.Eval(input, output[i:i+1]) {
			return false
		}
	}
	return true
}

// stitchingBounds is ShadingFunction::stitching_bounds in hayro-interpret
// src/shading.rs.
func (f *ShadingFunction) stitchingBounds() []float32 {
	var bounds []float32
	if f.Single != nil {
		bounds = append(bounds, f.Single.stitchingBounds()...)
	} else {
		for _, fn := range f.Multiple {
			bounds = append(bounds, fn.stitchingBounds()...)
		}
	}
	return sortDedupBounds(bounds)
}

// ShadingType is ShadingType in hayro-interpret src/shading.rs. It is a
// *FunctionBasedShading, a *RadialAxialShading, a *TriangleMeshShading, a
// *PatchMeshShading or a *DummyShading.
//
// Port: upstream is an enum. Go uses an interface that only the five types
// implement.
type ShadingType interface {
	isShadingType()
}

// FunctionBasedShading is ShadingType::FunctionBased in hayro-interpret
// src/shading.rs.
type FunctionBasedShading struct {
	// Domain is the domain of the function.
	Domain [4]float32
	// Matrix is a transform to apply to the shading.
	Matrix kurbo.Affine
	// Function evaluates the shading.
	Function ShadingFunction
}

// RadialAxialShading is ShadingType::RadialAxial in hayro-interpret
// src/shading.rs.
type RadialAxialShading struct {
	// Coords holds x0 y0 x1 y1 for an axial shading, and x0 y0 r0 x1 y1 r1
	// for a radial one.
	Coords [6]float32
	// Domain is the domain of the shading.
	Domain [2]float32
	// Function is the base of the shading.
	Function ShadingFunction
	// Extend holds the extends at the start and at the end.
	Extend [2]bool
	// Axial is true for an axial shading and false for a radial one.
	Axial bool
}

// TriangleMeshShading is ShadingType::TriangleMesh in hayro-interpret
// src/shading.rs.
type TriangleMeshShading struct {
	// Triangles are the triangles of the shading.
	Triangles []Triangle
	// Function computes the colors of the samples, or is nil.
	Function *ShadingFunction
}

// PatchMeshShading is ShadingType::PatchMesh in hayro-interpret
// src/shading.rs. It is a mesh of Coons patches or of tensor-product
// patches.
type PatchMeshShading struct {
	// Patches are the patches of the shading.
	Patches []Patch
	// Function computes the colors of the samples, or is nil.
	Function *ShadingFunction
}

// DummyShading is ShadingType::Dummy in hayro-interpret src/shading.rs. It
// is a shading that draws nothing.
type DummyShading struct{}

func (*FunctionBasedShading) isShadingType() {}
func (*RadialAxialShading) isShadingType()   {}
func (*TriangleMeshShading) isShadingType()  {}
func (*PatchMeshShading) isShadingType()     {}
func (*DummyShading) isShadingType()         {}

// Shading is Shading in hayro-interpret src/shading.rs. It is a shading of
// PDF.
type Shading struct {
	cacheKey Key128
	// ShadingType is the type of the shading. It is shared, and a caller
	// must not change it.
	ShadingType ShadingType
	// ColorSpace is the color space of the shading.
	ColorSpace ColorSpace
	// ClipPath is a path that clips the shading, or nil.
	ClipPath *kurboext.BezPath
	// Background is the background color of the shading, or nil.
	Background []float32

	meshBounds *kurbo.Rect
}

// newShading is Shading::new in hayro-interpret src/shading.rs. It reads a
// shading through the cache.
func newShading(dict syntax.Dict, stream *syntax.Stream, cache *Cache) (*Shading, bool) {
	key := dictCacheKey(dict)
	if stream != nil {
		key = streamCacheKey(*stream)
	}
	return cacheGetOrInsertWith(cache, key, func() (*Shading, bool) {
		return parseShading(dict, stream, cache, key)
	})
}

// meshParams reads the entries of a shading of type 4 to 7.
func meshParams(dict syntax.Dict, stream *syntax.Stream, flag bool) (data []byte, bpCoord, bpComp, bpf uint8, ok bool) {
	if stream == nil {
		return nil, 0, 0, 0, false
	}
	data, err := stream.DecodedData()
	if err != nil {
		return nil, 0, 0, 0, false
	}
	if bpCoord, ok = dict.Get[uint8](syntax.KeyBitsPerCoordinate); !ok {
		return nil, 0, 0, 0, false
	}
	if bpComp, ok = dict.Get[uint8](syntax.KeyBitsPerComponent); !ok {
		return nil, 0, 0, 0, false
	}
	if flag {
		if bpf, ok = dict.Get[uint8](syntax.KeyBitsPerFlag); !ok {
			return nil, 0, 0, 0, false
		}
	}
	return data, bpCoord, bpComp, bpf, true
}

// meshDecode reads the decode array of a mesh.
func meshDecode(dict syntax.Dict) ([]float32, bool) {
	arr, ok := dict.Get[syntax.Array](syntax.KeyDecode)
	if !ok {
		return nil, false
	}
	return arr.Iter[float32]().Collect(), true
}

// parseShading is Shading::parse in hayro-interpret src/shading.rs.
func parseShading(dict syntax.Dict, stream *syntax.Stream, cache *Cache, cacheKey Key128) (*Shading, bool) {
	shadingNum, ok := dict.Get[uint8](syntax.KeyShadingType)
	if !ok {
		return nil, false
	}
	csObj, ok := dict.Get[syntax.Object](syntax.KeyColorSpace)
	if !ok {
		return nil, false
	}
	colorSpace, ok := newColorSpace(csObj, cache)
	if !ok {
		return nil, false
	}
	var meshBounds *kurbo.Rect
	var shadingType ShadingType
	switch shadingNum {
	case 1:
		domain, ok := dict.Get[[4]float32](syntax.KeyDomain)
		if !ok {
			domain = [4]float32{0, 1, 0, 1}
		}
		matrix := kurbo.IdentityAffine
		if m, ok := dict.Get[[6]float64](syntax.KeyMatrix); ok {
			matrix = kurbo.NewAffine(m)
		}
		function, ok := readShadingFunction(dict, colorSpace)
		if !ok {
			return nil, false
		}
		shadingType = &FunctionBasedShading{Domain: domain, Matrix: matrix, Function: *function}
	case 2, 3:
		domain, ok := dict.Get[[2]float32](syntax.KeyDomain)
		if !ok {
			domain = [2]float32{0, 1}
		}
		function, ok := readShadingFunction(dict, colorSpace)
		if !ok {
			return nil, false
		}
		extend, _ := dict.Get[[2]bool](syntax.KeyExtend)
		var coords [6]float32
		var invalid bool
		if shadingNum == 2 {
			read, ok := dict.Get[[4]float32](syntax.KeyCoords)
			if !ok {
				return nil, false
			}
			invalid = IsNearlyZero(read[0]-read[2]) && IsNearlyZero(read[1]-read[3])
			coords = [6]float32{read[0], read[1], read[2], read[3], 0, 0}
		} else {
			read, ok := dict.Get[[6]float32](syntax.KeyCoords)
			if !ok {
				return nil, false
			}
			invalid = IsNearlyZero(read[0]-read[3]) && IsNearlyZero(read[1]-read[4]) && IsNearlyZero(read[2]-read[5])
			coords = read
		}
		if invalid {
			shadingType = &DummyShading{}
		} else {
			shadingType = &RadialAxialShading{Domain: domain, Function: *function, Extend: extend, Coords: coords, Axial: shadingNum == 2}
		}
	case 4, 5:
		data, bpCoord, bpComp, bpf, ok := meshParams(dict, stream, shadingNum == 4)
		if !ok {
			return nil, false
		}
		function, _ := readShadingFunction(dict, colorSpace)
		decode, ok := meshDecode(dict)
		if !ok {
			return nil, false
		}
		var triangles []Triangle
		var bounds *kurbo.Rect
		if shadingNum == 4 {
			triangles, bounds, ok = readFreeFormTriangles(data, bpf, bpCoord, bpComp, function != nil, decode)
		} else {
			verticesPerRow, vok := dict.Get[uint32](syntax.KeyVerticesPerRow)
			if !vok {
				return nil, false
			}
			triangles, bounds, ok = readLatticeTriangles(data, bpCoord, bpComp, function != nil, verticesPerRow, decode)
		}
		if !ok {
			return nil, false
		}
		meshBounds = bounds
		shadingType = &TriangleMeshShading{Triangles: triangles, Function: function}
	case 6, 7:
		data, bpCoord, bpComp, bpf, ok := meshParams(dict, stream, true)
		if !ok {
			return nil, false
		}
		function, _ := readShadingFunction(dict, colorSpace)
		decode, ok := meshDecode(dict)
		if !ok {
			return nil, false
		}
		kind := patchKindCoons
		if shadingNum == 7 {
			kind = patchKindTensorProduct
		}
		patches, bounds, ok := readPatchMesh(data, bpf, bpCoord, bpComp, function != nil, decode, kind)
		if !ok {
			return nil, false
		}
		meshBounds = bounds
		shadingType = &PatchMeshShading{Patches: patches, Function: function}
	default:
		return nil, false
	}
	s := &Shading{cacheKey: cacheKey, ShadingType: shadingType, ColorSpace: colorSpace, meshBounds: meshBounds}
	if r, ok := dict.Get[syntax.Rect](syntax.KeyBBox); ok {
		p := kurboext.RectToPath(RectToKurbo(r), 0.1)
		s.ClipPath = &p
	}
	if arr, ok := dict.Get[syntax.Array](syntax.KeyBackground); ok {
		s.Background = arr.Iter[float32]().Collect()
		if s.Background == nil {
			s.Background = []float32{}
		}
	}
	return s, true
}

// CacheKey is the CacheKey implementation of Shading in hayro-interpret
// src/shading.rs.
func (s *Shading) CacheKey() Key128 {
	return s.cacheKey
}

// Triangle is Triangle in hayro-interpret src/shading.rs. It is a triangle
// of a mesh.
type Triangle struct {
	// Vertices are the corners of the triangle.
	Vertices [3]TriangleVertex
}

// TriangleVertex is TriangleVertex in hayro-interpret src/shading.rs.
type TriangleVertex struct {
	flag uint32
	// Point is the position of the vertex.
	Point kurbo.Point
	// Colors holds the components of the color of the vertex.
	Colors ColorComponents
}

// Patch is Patch in hayro-interpret src/shading.rs. It is a patch of a
// mesh, in the form of a tensor product. A Coons patch is converted.
type Patch struct {
	// ControlPoints holds the control points by column (u) and then by row
	// (v).
	ControlPoints [4][4]kurbo.Point
	// Colors holds the colors of the corners by u and then by v.
	Colors [2][2]ColorComponents
}

// readFreeFormTriangles is read_free_form_triangles in hayro-interpret
// src/shading.rs.
func readFreeFormTriangles(data []byte, bpf, bpCoord, bpComp uint8, hasFunction bool, decode []float32) ([]Triangle, *kurbo.Rect, bool) {
	var triangles []Triangle
	var bounds *kurbo.Rect
	xr, decode, ok := splitDecode(decode)
	if !ok {
		return nil, nil, false
	}
	reader := syntax.NewBitReader(data)
	helpers := newInterpolationHelpers(bpCoord, bpComp, xr)
	readSingle := func() (TriangleVertex, bool) {
		v, ok := helpers.readTriangleVertex(reader, bpf, hasFunction, decode)
		if !ok {
			return TriangleVertex{}, false
		}
		extendBounds(&bounds, kurbo.RectFromPoints(v.Point, v.Point))
		return v, true
	}
	var a, b, c *TriangleVertex
	// Port: see maxMeshPieces.
	for len(triangles) < maxMeshPieces {
		first, ok := readSingle()
		if !ok {
			break
		}
		switch first.flag {
		case 0:
			second, ok := readSingle()
			if !ok {
				return nil, nil, false
			}
			third, ok := readSingle()
			if !ok {
				return nil, nil, false
			}
			a, b, c = &first, &second, &third
		case 1:
			if b == nil || c == nil {
				return nil, nil, false
			}
			a, b, c = b, c, &first
		case 2:
			if c == nil {
				return nil, nil, false
			}
			b, c = c, &first
		}
		if a == nil || b == nil || c == nil {
			return nil, nil, false
		}
		if pointNearlySame(a.Point, b.Point) || pointNearlySame(b.Point, c.Point) {
			continue
		}
		triangles = append(triangles, Triangle{Vertices: [3]TriangleVertex{*a, *b, *c}})
	}
	return triangles, bounds, true
}

// interpolationHelpers is InterpolationHelpers in hayro-interpret
// src/shading.rs.
type interpolationHelpers struct {
	bpCoord, bpComp   uint8
	coordMax, compMax float32
	xMin, xMax        float32
	yMin, yMax        float32
}

// newInterpolationHelpers is InterpolationHelpers::new in hayro-interpret
// src/shading.rs. xr holds x_min, x_max, y_min and y_max.
func newInterpolationHelpers(bpCoord, bpComp uint8, xr [4]float32) *interpolationHelpers {
	return &interpolationHelpers{
		bpCoord:  bpCoord,
		bpComp:   bpComp,
		coordMax: powf32(2, float32(bpCoord)) - 1,
		compMax:  powf32(2, float32(bpComp)) - 1,
		xMin:     xr[0], xMax: xr[1], yMin: xr[2], yMax: xr[3],
	}
}

// interpolateCoord is InterpolationHelpers::interpolate_coord in
// hayro-interpret src/shading.rs.
func (h *interpolationHelpers) interpolateCoord(n uint32, dMin, dMax float32) float32 {
	return interpolate(float32(n), 0, h.coordMax, dMin, dMax)
}

// interpolateComp is InterpolationHelpers::interpolate_comp in
// hayro-interpret src/shading.rs.
func (h *interpolationHelpers) interpolateComp(n uint32, dMin, dMax float32) float32 {
	return interpolate(float32(n), 0, h.compMax, dMin, dMax)
}

// readPoint is InterpolationHelpers::read_point in hayro-interpret
// src/shading.rs.
func (h *interpolationHelpers) readPoint(reader *syntax.BitReader) (kurbo.Point, bool) {
	xv, ok := reader.Read(h.bpCoord)
	if !ok {
		return kurbo.Point{}, false
	}
	x := h.interpolateCoord(xv, h.xMin, h.xMax)
	yv, ok := reader.Read(h.bpCoord)
	if !ok {
		return kurbo.Point{}, false
	}
	y := h.interpolateCoord(yv, h.yMin, h.yMax)
	return kurbo.NewPoint(float64(x), float64(y)), true
}

// readColors is InterpolationHelpers::read_colors in hayro-interpret
// src/shading.rs.
func (h *interpolationHelpers) readColors(reader *syntax.BitReader, hasFunction bool, decode []float32) (ColorComponents, bool) {
	if hasFunction {
		if len(decode) < 2 {
			return nil, false
		}
		v, ok := reader.Read(h.bpComp)
		if !ok {
			return nil, false
		}
		return ColorComponents{h.interpolateComp(v, decode[0], decode[1])}, true
	}
	numComponents := len(decode) / 2
	colors := make(ColorComponents, 0, numComponents)
	for i := range numComponents {
		v, ok := reader.Read(h.bpComp)
		if !ok {
			return nil, false
		}
		colors = append(colors, h.interpolateComp(v, decode[2*i], decode[2*i+1]))
	}
	return colors, true
}

// readTriangleVertex is InterpolationHelpers::read_triangle_vertex in
// hayro-interpret src/shading.rs.
func (h *interpolationHelpers) readTriangleVertex(reader *syntax.BitReader, bpf uint8, hasFunction bool, decode []float32) (TriangleVertex, bool) {
	flag, ok := reader.Read(bpf)
	if !ok {
		return TriangleVertex{}, false
	}
	point, ok := h.readPoint(reader)
	if !ok {
		return TriangleVertex{}, false
	}
	colors, ok := h.readColors(reader, hasFunction, decode)
	if !ok {
		return TriangleVertex{}, false
	}
	reader.Align()
	return TriangleVertex{flag: flag, Point: point, Colors: colors}, true
}

// splitDecode is split_decode in hayro-interpret src/shading.rs. It splits
// the ranges of x and y from the ranges of the colors.
//
// Port: a mesh with more colors than any color space has fails, because
// upstream reads them all for each vertex (D18).
func splitDecode(decode []float32) ([4]float32, []float32, bool) {
	if len(decode) < 4 || len(decode)-4 > 2*maxComponents {
		return [4]float32{}, nil, false
	}
	return [4]float32(decode[:4]), decode[4:], true
}

// readLatticeTriangles is read_lattice_triangles in hayro-interpret
// src/shading.rs.
func readLatticeTriangles(data []byte, bpCoord, bpComp uint8, hasFunction bool, verticesPerRow uint32, decode []float32) ([]Triangle, *kurbo.Rect, bool) {
	if verticesPerRow < 2 {
		return nil, nil, false
	}
	var bounds *kurbo.Rect
	xr, decode, ok := splitDecode(decode)
	if !ok {
		return nil, nil, false
	}
	reader := syntax.NewBitReader(data)
	helpers := newInterpolationHelpers(bpCoord, bpComp, xr)
	readSingle := func() (TriangleVertex, bool) {
		point, ok := helpers.readPoint(reader)
		if !ok {
			return TriangleVertex{}, false
		}
		colors, ok := helpers.readColors(reader, hasFunction, decode)
		if !ok {
			return TriangleVertex{}, false
		}
		reader.Align()
		extendBounds(&bounds, kurbo.RectFromPoints(point, point))
		return TriangleVertex{Point: point, Colors: colors}, true
	}
	var lattices [][]TriangleVertex
	vertices := 0
outer:
	for {
		var row []TriangleVertex
		for range verticesPerRow {
			// Port: see maxMeshPieces. Each vertex makes at most two
			// triangles.
			if vertices >= maxMeshPieces/2 {
				break outer
			}
			next, ok := readSingle()
			if !ok {
				break outer
			}
			row = append(row, next)
			vertices++
		}
		lattices = append(lattices, row)
	}
	var triangles []Triangle
	for i := 0; i+1 < len(lattices); i++ {
		for j := 0; j+1 < int(verticesPerRow); j++ {
			triangles = append(triangles,
				Triangle{Vertices: [3]TriangleVertex{lattices[i][j], lattices[i+1][j], lattices[i][j+1]}},
				Triangle{Vertices: [3]TriangleVertex{lattices[i+1][j+1], lattices[i+1][j], lattices[i][j+1]}},
			)
		}
	}
	return triangles, bounds, true
}

// patchKind is PatchKind in hayro-interpret src/shading.rs.
type patchKind uint8

// The cases of PatchKind.
const (
	patchKindCoons patchKind = iota
	patchKindTensorProduct
)

// readPatchMesh is read_patch_mesh in hayro-interpret src/shading.rs.
func readPatchMesh(data []byte, bpf, bpCoord, bpComp uint8, hasFunction bool, decode []float32, kind patchKind) ([]Patch, *kurbo.Rect, bool) {
	controlPointsCount := 12
	if kind == patchKindTensorProduct {
		controlPointsCount = 16
	}
	var bounds *kurbo.Rect
	xr, decode, ok := splitDecode(decode)
	if !ok {
		return nil, nil, false
	}
	reader := syntax.NewBitReader(data)
	helpers := newInterpolationHelpers(bpCoord, bpComp, xr)
	readColors := func() (ColorComponents, bool) {
		return helpers.readColors(reader, hasFunction, decode)
	}
	var prevPoints *[16]kurbo.Point
	var prevColors *[4]ColorComponents
	var patches []Patch
	// Port: see maxMeshPieces.
	for len(patches) < maxMeshPieces {
		flag, ok := reader.Read(bpf)
		if !ok {
			break
		}
		var controlPoints [16]kurbo.Point
		var colors [4]ColorComponents
		switch flag {
		case 0:
			for i := range controlPointsCount {
				if controlPoints[i], ok = helpers.readPoint(reader); !ok {
					return nil, nil, false
				}
			}
			for i := range 4 {
				if colors[i], ok = readColors(); !ok {
					return nil, nil, false
				}
			}
		case 1, 2, 3:
			if prevPoints == nil || prevColors == nil {
				return nil, nil, false
			}
			copyPatchControlPoints(flag, prevPoints, &controlPoints)
			switch flag {
			case 1:
				colors[0], colors[1] = prevColors[1], prevColors[2]
			case 2:
				colors[0], colors[1] = prevColors[2], prevColors[3]
			case 3:
				colors[0], colors[1] = prevColors[3], prevColors[0]
			}
			for i := 4; i < controlPointsCount; i++ {
				if controlPoints[i], ok = helpers.readPoint(reader); !ok {
					return nil, nil, false
				}
			}
			if colors[2], ok = readColors(); !ok {
				return nil, nil, false
			}
			if colors[3], ok = readColors(); !ok {
				return nil, nil, false
			}
		default:
			return patches, bounds, true
		}
		pp, pc := controlPoints, colors
		prevPoints, prevColors = &pp, &pc
		patch := patchFromStream(&controlPoints, colors, kind)
		extendBounds(&bounds, pointsBbox(patch.ControlPoints[:]))
		patches = append(patches, patch)
	}
	return patches, bounds, true
}

// streamOrder is STREAM_ORDER in hayro-interpret src/shading.rs.
var streamOrder = [4][4]int{{0, 1, 2, 3}, {11, 12, 13, 4}, {10, 15, 14, 5}, {9, 8, 7, 6}}

// patchFromStream is patch_from_stream in hayro-interpret src/shading.rs. It
// orders the points of the stream into a grid, and computes the inner
// points of a Coons patch.
func patchFromStream(points *[16]kurbo.Point, colors [4]ColorComponents, kind patchKind) Patch {
	var p [4][4]kurbo.Point
	for i, column := range streamOrder {
		for j, idx := range column {
			p[i][j] = points[idx]
		}
	}
	if kind == patchKindCoons {
		v := func(pt kurbo.Point) kurbo.Vec2 { return pt.ToVec2() }
		p00, p01, p02, p03 := v(p[0][0]), v(p[0][1]), v(p[0][2]), v(p[0][3])
		p10, p13 := v(p[1][0]), v(p[1][3])
		p20, p23 := v(p[2][0]), v(p[2][3])
		p30, p31, p32, p33 := v(p[3][0]), v(p[3][1]), v(p[3][2]), v(p[3][3])
		inner := func(a, b1, b2, c1, c2, d1, d2, e kurbo.Vec2) kurbo.Point {
			r := a.Mul(-4).Add(b1.Add(b2).Mul(6)).Sub(c1.Add(c2).Mul(2)).Add(d1.Add(d2).Mul(3)).Sub(e)
			return kurbo.NewPoint(r.X/9, r.Y/9)
		}
		p[1][1] = inner(p00, p01, p10, p03, p30, p31, p13, p33)
		p[1][2] = inner(p03, p02, p13, p00, p33, p32, p10, p30)
		p[2][1] = inner(p30, p31, p20, p33, p00, p01, p23, p03)
		p[2][2] = inner(p33, p32, p23, p30, p03, p02, p20, p00)
	}
	return Patch{ControlPoints: p, Colors: [2][2]ColorComponents{{colors[0], colors[1]}, {colors[3], colors[2]}}}
}

// copyPatchControlPoints is copy_patch_control_points in hayro-interpret
// src/shading.rs.
func copyPatchControlPoints(flag uint32, prev, cur *[16]kurbo.Point) {
	switch flag {
	case 1:
		cur[0], cur[1], cur[2], cur[3] = prev[3], prev[4], prev[5], prev[6]
	case 2:
		cur[0], cur[1], cur[2], cur[3] = prev[6], prev[7], prev[8], prev[9]
	case 3:
		cur[0], cur[1], cur[2], cur[3] = prev[9], prev[10], prev[11], prev[0]
	}
}

// readShadingFunction is read_function in hayro-interpret src/shading.rs.
func readShadingFunction(dict syntax.Dict, colorSpace ColorSpace) (*ShadingFunction, bool) {
	if arr, ok := dict.Get[syntax.Array](syntax.KeyFunction); ok {
		var functions []*Function
		for o := range arr.Iter[syntax.Object]().All() {
			f, ok := NewFunction(o)
			if !ok {
				return nil, false
			}
			functions = append(functions, f)
		}
		if len(functions) != int(colorSpace.NumComponents()) {
			return nil, false
		}
		for _, f := range functions {
			if f.NumOutputs() != 1 {
				return nil, false
			}
		}
		return &ShadingFunction{Multiple: functions}, true
	}
	if obj, ok := dict.Get[syntax.Object](syntax.KeyFunction); ok {
		f, ok := NewFunction(obj)
		if !ok {
			return nil, false
		}
		return &ShadingFunction{Single: f}, true
	}
	return nil, false
}

// extendBounds is extend_bounds in hayro-interpret src/shading.rs.
func extendBounds(bounds **kurbo.Rect, rect kurbo.Rect) {
	if *bounds == nil {
		*bounds = &rect
		return
	}
	u := (*bounds).Union(rect)
	*bounds = &u
}

// pointsBbox is points_bbox in hayro-interpret src/shading.rs.
func pointsBbox(columns [][4]kurbo.Point) kurbo.Rect {
	bbox := kurbo.RectFromPoints(columns[0][0], columns[0][0])
	for _, column := range columns {
		for _, p := range column {
			bbox = bbox.UnionPt(p)
		}
	}
	return bbox
}
