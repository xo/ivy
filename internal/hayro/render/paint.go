// Copyright (c) The Hayro Authors.
//
// Use of this source code is governed by the Apache-2.0 or MIT license that
// can be found in the LICENSE-APACHE and LICENSE-MIT files.

// This file ports hayro src/paint.rs.

package render

import (
	"math"

	"github.com/xo/okage"
	"github.com/xo/okage/kurbo"

	"github.com/xo/ivy/internal/hayro/interpret"
	"github.com/xo/ivy/internal/hayro/kurboext"
)

// nativeGradientTolerance is NATIVE_GRADIENT_TOLERANCE in hayro
// src/paint.rs.
const nativeGradientTolerance = 0.01

// maxTexturePixels is the size of the largest texture of a shading that
// interpret samples.
const maxTexturePixels = 1 << 24

// colorShader returns the shader of a color of interpret.
func colorShader(c interpret.Color) okage.Shader {
	rgba := c.ToRGBA().ToRGBA8()
	return okage.SolidColor(okage.ColorFromRGBA8(rgba[0], rgba[1], rgba[2], rgba[3]))
}

// setPaint is Renderer::set_paint in hayro src/paint.rs. It returns the
// shader of paint in the user space of transform, the transform of the
// drawing, and the clip path of a shading in the space of the pixels, or
// nil. pathBBox returns the bounds of the drawing in the user space.
// strokeWidth is the width of the stroke in the user space, and it counts
// only when isStroke is true. A nil shader draws nothing.
//
// Port: upstream sets the paint and the transform of the paint on the
// context of vello_cpu, which holds the transform of the drawing. The port
// returns a shader of okage that holds the transform of the paint.
func (r *renderer) setPaint(paint interpret.Paint, transform kurbo.Affine, pathBBox func() kurbo.Rect, isStroke bool, strokeWidth float64) (okage.Shader, *kurboext.BezPath) {
	switch p := paint.(type) {
	case interpret.Color:
		return colorShader(p), nil
	case *interpret.ShadingPattern, *interpret.TilingPattern:
	default:
		return nil, nil
	}
	if !invertible(transform) {
		return nil, nil
	}
	bbox := transform.TransformRectBbox(pathBBox())
	if isStroke {
		a1, a2 := interpret.XYAdvances(transform)
		factor := math.Max(a1.Hypot(), a2.Hypot()) * strokeWidth
		bbox = kurboext.RectInflate(bbox, factor, factor)
	}
	bbox = kurboext.RectIntersect(bbox, kurbo.NewRect(0, 0, r.width, r.height))
	inverse := transform.Inverse()
	switch p := paint.(type) {
	case *interpret.ShadingPattern:
		clipPath := p.Shading.ClipPath
		encoded := p.Encode()
		if gradient, ok := encoded.ShadingType.(*interpret.EncodedRadialAxialShading); ok {
			if native, ok := gradient.AsSVGGradient(encoded, nativeGradientTolerance); ok {
				if shader, ok := nativeGradient(native, inverse.Mul(native.Transform)); ok {
					return shader, clipPath
				}
			}
		}
		bbox = kurboext.RectExpand(bbox)
		w, h := interpret.TextureDimensions(bbox, 1)
		var data []byte
		if uint64(w)*uint64(h) <= maxTexturePixels {
			data = make([]byte, 0, int(w)*int(h)*4)
		}
		width, height, t := encoded.SampleTexture(bbox, 1, func(sample [4]float32) {
			data = append(data, premultiply(sample)...)
		})
		if width == 0 || height == 0 || width != w || height != h {
			return nil, clipPath
		}
		size, ok := okage.IntSizeFromWH(width, height)
		if !ok {
			return nil, clipPath
		}
		pix, ok := okage.PixmapFromVec(data, size)
		if !ok {
			return nil, clipPath
		}
		return okage.NewPattern(pix.AsRef(), okage.SpreadModePad, okage.FilterQualityBilinear, 1, toTransform(inverse.Mul(t))), clipPath
	case *interpret.TilingPattern:
		pix, spread, t, ok := r.tilingPaint(p, bbox, isStroke)
		if !ok {
			return nil, nil
		}
		return okage.NewPattern(pix.AsRef(), spread, okage.FilterQualityBilinear, p.Opacity, toTransform(inverse.Mul(t))), nil
	}
	return nil, nil
}

// premultiply returns the 4 bytes of a straight color of RGBA, premultiplied,
// as AlphaColor::premultiply and to_rgba8 of the color crate do.
func premultiply(c [4]float32) []byte {
	a := c[3]
	return []byte{unit8(c[0] * a), unit8(c[1] * a), unit8(c[2] * a), unit8(a)}
}

// unit8 converts a value from 0 to 1 to a byte, rounded.
func unit8(v float32) uint8 {
	switch {
	case !(v > 0):
		return 0
	case v >= 1:
		return 255
	}
	return uint8(v*255 + 0.5)
}

// nativeGradient returns the gradient of okage of an SVG gradient, with the
// transform t from the space of the gradient to the user space. It returns
// false when okage cannot make the gradient.
//
// Port: upstream makes a gradient of peniko in the same place of
// Renderer::set_paint.
func nativeGradient(native *interpret.SVGGradient, t kurbo.Affine) (okage.Shader, bool) {
	stops := make([]okage.GradientStop, 0, len(native.Stops))
	for _, s := range native.Stops {
		c := s.Color
		color, ok := okage.ColorFromRGBA(clamp01(c[0]), clamp01(c[1]), clamp01(c[2]), clamp01(c[3]))
		if !ok {
			return nil, false
		}
		stops = append(stops, okage.NewGradientStop(s.Offset, color))
	}
	if !t.IsFinite() {
		return nil, false
	}
	pt := func(p kurbo.Point) okage.Point { return okage.PointFromXY(float32(p.X), float32(p.Y)) }
	switch k := native.Kind.(type) {
	case interpret.SVGGradientLinear:
		return okage.NewLinearGradient(pt(k.Start), pt(k.End), stops, okage.SpreadModePad, toTransform(t))
	case interpret.SVGGradientRadial:
		return okage.NewRadialGradient(pt(k.StartCenter), k.StartRadius, pt(k.EndCenter), k.EndRadius, stops, okage.SpreadModePad, toTransform(t))
	}
	return nil, false
}

// clamp01 clamps v to the range from 0 to 1. NaN becomes 0.
func clamp01(v float32) float32 {
	switch {
	case !(v > 0):
		return 0
	case v > 1:
		return 1
	}
	return v
}
