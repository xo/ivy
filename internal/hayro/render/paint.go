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

// paintClips holds the clip paths of a paint, in the space of the pixels.
//
// Port: upstream has no such type. setPaint returns the clip path of the
// BBox of a shading, and the port adds the outline of a mesh.
type paintClips struct {
	// shading is the clip path of the BBox of a shading, or nil.
	shading *kurboext.BezPath
	// outline is the outline of the triangles of a mesh, or nil. It clips
	// with the nonzero rule.
	outline *kurboext.BezPath
}

// pushPaintClips pushes the clip paths of a paint. The clip path of the BBox
// clips with fill, as upstream does. It returns the number of clips that
// popPaintClips must pop.
func (r *renderer) pushPaintClips(c paintClips, fill interpret.FillRule) int {
	n := 0
	if c.shading != nil {
		r.pushClipPathInner(c.shading, fill)
		n++
	}
	if c.outline != nil {
		r.pushClipPathInner(c.outline, interpret.FillRuleNonZero)
		n++
	}
	return n
}

// popPaintClips pops n clips that pushPaintClips pushed.
func (r *renderer) popPaintClips(n int) {
	for range n {
		r.popClip()
	}
}

// setPaint is Renderer::set_paint in hayro src/paint.rs. It returns the
// shader of paint in the user space of transform, the transform of the
// drawing, and the clip paths of a shading. pathBBox returns the bounds of
// the drawing in the user space. strokeWidth is the width of the stroke in
// the user space, and it counts only when isStroke is true. A nil shader
// draws nothing.
//
// Port: upstream sets the paint and the transform of the paint on the
// context of vello_cpu, which holds the transform of the drawing. The port
// returns a shader of okage that holds the transform of the paint.
func (r *renderer) setPaint(paint interpret.Paint, transform kurbo.Affine, pathBBox func() kurbo.Rect, isStroke bool, strokeWidth float64) (okage.Shader, paintClips) {
	switch p := paint.(type) {
	case interpret.Color:
		return colorShader(p), paintClips{}
	case *interpret.ShadingPattern, *interpret.TilingPattern:
	default:
		return nil, paintClips{}
	}
	if !invertible(transform) {
		return nil, paintClips{}
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
		clips := paintClips{shading: p.Shading.ClipPath}
		encoded := p.Encode()
		if gradient, ok := encoded.ShadingType.(*interpret.EncodedRadialAxialShading); ok {
			if native, ok := gradient.AsSVGGradient(encoded, nativeGradientTolerance); ok {
				if shader, ok := nativeGradient(native, inverse.Mul(native.Transform)); ok {
					return shader, clips
				}
			}
		}
		bbox = kurboext.RectExpand(bbox)
		if _, ok := encoded.ShadingType.(*interpret.EncodedMeshShading); ok {
			shader, outline := meshGradient(encoded, bbox, inverse)
			clips.outline = outline
			return shader, clips
		}
		w, h := interpret.TextureDimensions(bbox, 1)
		var data []byte
		if uint64(w)*uint64(h) <= maxTexturePixels {
			data = make([]byte, 0, int(w)*int(h)*4)
		}
		width, height, t := encoded.SampleTexture(bbox, 1, func(sample [4]float32) {
			data = append(data, premultiply(sample)...)
		})
		if width == 0 || height == 0 || width != w || height != h {
			return nil, clips
		}
		size, ok := okage.IntSizeFromWH(width, height)
		if !ok {
			return nil, clips
		}
		pix, ok := okage.PixmapFromVec(data, size)
		if !ok {
			return nil, clips
		}
		return okage.NewPattern(pix.AsRef(), okage.SpreadModePad, okage.FilterQualityBilinear, 1, toTransform(inverse.Mul(t))), clips
	case *interpret.TilingPattern:
		pix, spread, t, ok := r.tilingPaint(p, bbox, isStroke)
		if !ok {
			return nil, paintClips{}
		}
		return okage.NewPattern(pix.AsRef(), spread, okage.FilterQualityBilinear, p.Opacity, toTransform(inverse.Mul(t))), paintClips{}
	}
	return nil, paintClips{}
}

// meshGradient returns the MeshGradient of okage of a mesh shading, a
// shading of type 4 to 7, over bbox in the space of the device, and the
// outline of its triangles in the space of the pixels. inverse maps the
// space of the device to the user space. It returns a nil shader when the
// mesh holds no triangle in bbox.
//
// Port: upstream samples every shading that is not a gradient into a
// texture. okage fills the triangles of a mesh with Gouraud shading (okage
// D55), which is faster and sharp at any scale. okage paints a pixel at a
// free edge of the mesh in full, so the caller clips the fill with the
// outline, which gives the antialiasing at that edge. The outline holds
// each triangle, turned the same way, so that the nonzero rule fills their
// union.
func meshGradient(encoded *interpret.EncodedShadingPattern, bbox kurbo.Rect, inverse kurbo.Affine) (okage.Shader, *kurboext.BezPath) {
	if !inverse.IsFinite() {
		return nil, nil
	}
	var triangles []okage.MeshTriangle
	outline := kurboext.NewBezPath()
	encoded.MeshTriangles(bbox, func(t interpret.MeshTriangle) {
		var mt okage.MeshTriangle
		for i := range mt {
			c := t.Colors[i]
			color, _ := okage.ColorFromRGBA(clamp01(c[0]), clamp01(c[1]), clamp01(c[2]), clamp01(c[3]))
			mt[i] = okage.MeshVertex{
				Point: okage.PointFromXY(float32(t.Points[i].X), float32(t.Points[i].Y)),
				Color: color,
			}
		}
		triangles = append(triangles, mt)
		addTriangle(&outline, t.Points)
	})
	shader, ok := okage.NewMeshGradient(triangles, toTransform(inverse))
	if !ok {
		return nil, nil
	}
	return shader, &outline
}

// addTriangle adds the triangle p to path, counterclockwise in the space
// of the pixels. It skips a triangle with no area or with a point that is
// not finite, as okage.NewMeshGradient does.
func addTriangle(path *kurboext.BezPath, p [3]kurbo.Point) {
	cross := (p[1].X-p[0].X)*(p[2].Y-p[0].Y) - (p[1].Y-p[0].Y)*(p[2].X-p[0].X)
	if cross == 0 || math.IsNaN(cross) || math.IsInf(cross, 0) {
		return
	}
	if cross < 0 {
		p[1], p[2] = p[2], p[1]
	}
	path.MoveTo(p[0])
	path.LineTo(p[1])
	path.LineTo(p[2])
	path.ClosePath()
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
// Renderer::set_paint. A radial gradient paints with the rule of PDF and
// the flags of its Extend (okage D64), so that the circles need not nest.
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
	spread := okage.SpreadModePad
	if native.Decal {
		spread = okage.SpreadModeDecal
	}
	switch k := native.Kind.(type) {
	case interpret.SVGGradientLinear:
		return okage.NewLinearGradient(pt(k.Start), pt(k.End), stops, spread, toTransform(t))
	case interpret.SVGGradientRadial:
		return okage.NewRadialGradientExtend(pt(k.StartCenter), k.StartRadius, pt(k.EndCenter), k.EndRadius, stops, k.Extend[0], k.Extend[1], toTransform(t))
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
