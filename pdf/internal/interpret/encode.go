// Copyright (c) The Hayro Authors.
//
// Use of this source code is governed by the Apache-2.0 or MIT license that
// can be found in the LICENSE-APACHE and LICENSE-MIT files.

// This file ports hayro-interpret src/encode.rs.

package interpret

import (
	"math"

	"github.com/xo/okage/kurbo"
)

// maxTexturePixels is the largest number of pixels of a texture of a
// shading.
//
// Port: upstream allocates the texture for any box that the device asks
// for. The cap keeps the memory bounded (D18).
const maxTexturePixels = 1 << 24

// EncodedShadingPattern is EncodedShadingPattern in hayro-interpret
// src/encode.rs. It is a shading pattern that is ready to be sampled.
type EncodedShadingPattern struct {
	// BaseTransform is the base transform of the shading pattern.
	BaseTransform kurbo.Affine
	// ShadingType is the encoded type of the shading.
	ShadingType EncodedShadingType

	colorSpace       ColorSpace
	backgroundColor  AlphaColor
	opacity          float32
	transferFunction *ActiveTransferFunction
	run              *runState
}

// EncodedShadingType is EncodedShadingType in hayro-interpret
// src/encode.rs. It is an *EncodedFunctionBasedShading, an
// *EncodedRadialAxialShading, an *EncodedMeshShading or an
// *EncodedDummyShading.
//
// Port: upstream is an enum. Go uses an interface that only the four types
// implement.
type EncodedShadingType interface {
	isEncodedShadingType()
}

// EncodedFunctionBasedShading is EncodedFunctionBasedShading in
// hayro-interpret src/encode.rs.
type EncodedFunctionBasedShading struct {
	domain   kurbo.Rect
	function ShadingFunction
}

// EncodedRadialAxialShading is EncodedRadialAxialShading in
// hayro-interpret src/encode.rs.
type EncodedRadialAxialShading struct {
	function ShadingFunction
	params   radialAxialParams
	domain   [2]float32
	extend   [2]bool
}

// EncodedMeshShading is EncodedMeshShading in hayro-interpret
// src/encode.rs.
type EncodedMeshShading struct {
	shadingType ShadingType
}

// EncodedDummyShading is EncodedDummyShading in hayro-interpret
// src/encode.rs.
type EncodedDummyShading struct{}

func (*EncodedFunctionBasedShading) isEncodedShadingType() {}
func (*EncodedRadialAxialShading) isEncodedShadingType()   {}
func (*EncodedMeshShading) isEncodedShadingType()          {}
func (*EncodedDummyShading) isEncodedShadingType()         {}

// ShadingTexture is ShadingTexture in hayro-interpret src/encode.rs. It is a
// shading that was sampled into a texture.
type ShadingTexture struct {
	// Data holds the pixels as straight RGBA8, row by row.
	Data []byte
	// Width is the width of the texture in pixels.
	Width uint32
	// Height is the height of the texture in pixels.
	Height uint32
	// Transform maps the space of the texture to the space of the box.
	Transform kurbo.Affine
	// HasTransparency is true when a pixel is not fully opaque.
	HasTransparency bool
}

// TextureDimensions is texture_dimensions in hayro-interpret src/encode.rs.
// It returns the size of the texture of bbox at scale pixels for each
// unit.
func TextureDimensions(bbox kurbo.Rect, scale float32) (uint32, uint32) {
	return asU32(math.Ceil(float64(rustMax(float32(bbox.Width())*scale, 1)))),
		asU32(math.Ceil(float64(rustMax(float32(bbox.Height())*scale, 1))))
}

// finish is EncodedShadingPattern::finish in hayro-interpret src/encode.rs.
// It applies the opacity and the transfer function.
func (p *EncodedShadingPattern) finish(color [4]float32) [4]float32 {
	color[3] *= p.opacity
	if p.transferFunction != nil {
		return p.transferFunction.Apply(NewAlphaColor(color)).Components()
	}
	return color
}

// convert is EncodedShadingPattern::convert in hayro-interpret
// src/encode.rs.
func (p *EncodedShadingPattern) convert(components []float32) [4]float32 {
	return p.finish(p.colorSpace.ToRGBA(components, 1).Components())
}

// eval is EncodedShadingPattern::eval in hayro-interpret src/encode.rs.
func (p *EncodedShadingPattern) eval(function *ShadingFunction, input []float32) [4]float32 {
	var buf [maxComponents]float32
	n := function.NumOutputs()
	if n > maxComponents {
		return [4]float32{}
	}
	out := buf[:n]
	if !function.Eval(input, out) {
		return [4]float32{}
	}
	return p.convert(out)
}

// SampleTexture is EncodedShadingPattern::sample_texture in
// hayro-interpret src/encode.rs. It calls f with the straight RGBA color of
// each pixel of a grid that covers bbox, row by row. It returns the size of
// the grid and its transform.
func (p *EncodedShadingPattern) SampleTexture(bbox kurbo.Rect, scale float32, f func([4]float32)) (uint32, uint32, kurbo.Affine) {
	width, height := TextureDimensions(bbox, scale)
	gridTransform := kurbo.AffineTranslate(kurbo.NewVec2(bbox.X0, bbox.Y0)).Mul(kurbo.AffineScale(1 / float64(scale)))
	// Port: see maxTexturePixels.
	if uint64(width)*uint64(height) > maxTexturePixels {
		return 0, 0, gridTransform
	}
	background := p.finish(p.backgroundColor.Components())
	c := newCanvas(width, height, background)
	c.run = p.run
	transform := gridTransform.Inverse().Mul(p.BaseTransform.Inverse())
	switch s := p.ShadingType.(type) {
	case *EncodedFunctionBasedShading:
		drawFunction(c, s.domain, &s.function, transform, p)
	case *EncodedRadialAxialShading:
		s.draw(c, transform, p)
	case *EncodedMeshShading:
		drawMesh(c, s.shadingType, transform, p)
	}
	for _, px := range c.pixels {
		f(px)
	}
	return width, height, gridTransform
}

// RenderTexture is EncodedShadingPattern::render_texture in hayro-interpret
// src/encode.rs. It samples the shading into a straight RGBA8 texture that
// covers bbox.
func (p *EncodedShadingPattern) RenderTexture(bbox kurbo.Rect, scale float32) ShadingTexture {
	width, height := TextureDimensions(bbox, scale)
	var data []byte
	if uint64(width)*uint64(height) <= maxTexturePixels {
		data = make([]byte, 0, int(width)*int(height)*4)
	}
	hasTransparency := false
	w, h, transform := p.SampleTexture(bbox, scale, func(s [4]float32) {
		converted := [4]byte{asU8(s[0]*255 + 0.5), asU8(s[1]*255 + 0.5), asU8(s[2]*255 + 0.5), asU8(s[3]*255 + 0.5)}
		hasTransparency = hasTransparency || converted[3] != 255
		data = append(data, converted[:]...)
	})
	return ShadingTexture{Data: data, Width: w, Height: h, Transform: transform, HasTransparency: hasTransparency}
}

// Encode is ShadingPattern::encode in hayro-interpret src/encode.rs. It
// prepares the shading pattern for sampling.
func (p *ShadingPattern) Encode() *EncodedShadingPattern {
	var baseTransform kurbo.Affine
	var shadingType EncodedShadingType
	switch s := p.Shading.ShadingType.(type) {
	case *FunctionBasedShading:
		baseTransform = p.Matrix.Mul(s.Matrix).Inverse()
		shadingType = encodeFunctionShading(s.Domain, &s.Function)
	case *RadialAxialShading:
		encoded, initialTransform := encodeAxialShading(s.Coords, s.Domain, &s.Function, s.Extend, s.Axial)
		baseTransform = initialTransform.Mul(p.Matrix.Inverse())
		shadingType = encoded
	case *TriangleMeshShading, *PatchMeshShading:
		baseTransform = p.Matrix.Inverse()
		shadingType = &EncodedMeshShading{shadingType: s}
	default:
		baseTransform = kurbo.IdentityAffine
		shadingType = &EncodedDummyShading{}
	}
	colorSpace := p.Shading.ColorSpace
	backgroundColor := AlphaColorTransparent
	if _, dummy := shadingType.(*EncodedDummyShading); !dummy && p.Shading.Background != nil {
		backgroundColor = colorSpace.ToRGBA(p.Shading.Background, 1)
	}
	return &EncodedShadingPattern{
		colorSpace:       colorSpace,
		backgroundColor:  backgroundColor,
		ShadingType:      shadingType,
		BaseTransform:    baseTransform,
		opacity:          p.Opacity,
		transferFunction: p.TransferFunction,
		run:              p.run,
	}
}

// encodeAxialShading is encode_axial_shading in hayro-interpret
// src/encode.rs.
func encodeAxialShading(coords [6]float32, domain [2]float32, function *ShadingFunction, extend [2]bool, isAxial bool) (*EncodedRadialAxialShading, kurbo.Affine) {
	var initialTransform kurbo.Affine
	var params radialAxialParams
	if isAxial {
		x0, y0, x1, y1 := coords[0], coords[1], coords[2], coords[3]
		initialTransform = tsFromLineToLine(
			kurbo.NewPoint(float64(x0), float64(y0)),
			kurbo.NewPoint(float64(x1), float64(y1)),
			kurbo.Point{},
			kurbo.NewPoint(1, 0),
		)
	} else {
		x0, y0, r0, x1, y1, r1 := coords[0], coords[1], coords[2], coords[3], coords[4], coords[5]
		initialTransform = kurbo.AffineTranslate(kurbo.NewVec2(float64(-x0), float64(-y0)))
		params = radialAxialParams{
			radial: true,
			p1:     kurbo.NewPoint(float64(x1-x0), float64(y1-y0)),
			r:      kurbo.NewPoint(float64(r0), float64(r1)),
		}
	}
	return &EncodedRadialAxialShading{function: *function, params: params, domain: domain, extend: extend}, initialTransform
}

// encodeFunctionShading is encode_function_shading in hayro-interpret
// src/encode.rs.
func encodeFunctionShading(domain [4]float32, function *ShadingFunction) *EncodedFunctionBasedShading {
	return &EncodedFunctionBasedShading{
		domain:   kurbo.NewRect(float64(domain[0]), float64(domain[2]), float64(domain[1]), float64(domain[3])),
		function: *function,
	}
}

// radialAxialParams is RadialAxialParams in hayro-interpret src/encode.rs.
//
// Port: upstream is an enum. radial selects the case Radial, with p1 and r.
type radialAxialParams struct {
	radial bool
	p1     kurbo.Point
	r      kurbo.Point
}

// draw is EncodedRadialAxialShading::draw in hayro-interpret src/encode.rs.
func (s *EncodedRadialAxialShading) draw(c *canvas, transform kurbo.Affine, pattern *EncodedShadingPattern) {
	lut := functionLut(&s.function, s.domain, pattern)
	toShading := transform.Inverse()
	xAdvance, yAdvance := XYAdvances(toShading)
	rowStart := toShading.MulPoint(kurbo.NewPoint(0.5, 0.5))
	for y := range c.height {
		if c.stopped() {
			return
		}
		point := rowStart
		row := c.pixels[y*c.width : (y+1)*c.width]
		for i := range row {
			if t, ok := s.position(point); ok {
				row[i] = lut.get(t)
			}
			point = point.Add(xAdvance)
		}
		rowStart = rowStart.Add(yAdvance)
	}
}

// position is EncodedRadialAxialShading::position in hayro-interpret
// src/encode.rs. It returns the parameter t of a point, and false for a
// point that the shading does not cover.
func (s *EncodedRadialAxialShading) position(pos kurbo.Point) (float32, bool) {
	var t float32
	if s.params.radial {
		var ok bool
		if t, ok = radialPos(pos, s.params.p1, s.params.r, s.extend[0], s.extend[1]); !ok {
			return 0, false
		}
	} else {
		t = float32(pos.X)
	}
	switch {
	case t < 0:
		return 0, s.extend[0]
	case t > 1:
		return 1, s.extend[1]
	}
	return t, true
}

// tsFromLineToLine is ts_from_line_to_line in hayro-interpret
// src/encode.rs.
func tsFromLineToLine(src1, src2, dst1, dst2 kurbo.Point) kurbo.Affine {
	unitToLine1 := unitToLine(src1, src2)
	line1ToUnit := unitToLine1.Inverse()
	unitToLine2 := unitToLine(dst1, dst2)
	return unitToLine2.Mul(line1ToUnit)
}

// unitToLine is unit_to_line in hayro-interpret src/encode.rs.
func unitToLine(p0, p1 kurbo.Point) kurbo.Affine {
	return kurbo.NewAffine([6]float64{p1.Y - p0.Y, p0.X - p1.X, p1.X - p0.X, p1.Y - p0.Y, p0.X, p0.Y})
}

// radialPos is radial_pos in hayro-interpret src/encode.rs. It solves for
// the largest t whose circle holds the point.
func radialPos(pos, p1, r kurbo.Point, minExtend, maxExtend bool) (float32, bool) {
	r0 := float32(r.X)
	dx := float32(p1.X)
	dy := float32(p1.Y)
	dr := float32(r.Y) - r0
	px := float32(pos.X)
	py := float32(pos.Y)
	a := dx*dx + dy*dy - dr*dr
	b := -2 * (px*dx + py*dy + r0*dr)
	cc := px*px + py*py - r0*r0
	discriminant := b*b - 4*a*cc
	if discriminant < 0 {
		return 0, false
	}
	if abs32(a) < 1e-6 {
		if abs32(b) < 1e-6 {
			return 0, false
		}
		t := -cc / b
		if (!minExtend && t < 0) || (!maxExtend && t > 1) {
			return 0, false
		}
		if r0+dr*t < 0 {
			return 0, false
		}
		return t, true
	}
	sqrtD := float32(math.Sqrt(float64(discriminant)))
	t1 := (-b - sqrtD) / (2 * a)
	t2 := (-b + sqrtD) / (2 * a)
	maxT := rustMax(t1, t2)
	minT := rustMin(t1, t2)
	takeMax, takeMin := true, true
	if (!minExtend && minT < 0) || r0+dr*minT < 0 {
		takeMin = false
	}
	if (!maxExtend && maxT > 1) || r0+dr*maxT < 0 {
		takeMax = false
	}
	switch {
	case takeMax:
		return maxT, true
	case takeMin:
		return minT, true
	}
	return 0, false
}

// abs32 is f32::abs of Rust.
func abs32(x float32) float32 {
	return float32(math.Abs(float64(x)))
}
