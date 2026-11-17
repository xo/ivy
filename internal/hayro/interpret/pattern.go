// Copyright (c) The Hayro Authors.
//
// Use of this source code is governed by the Apache-2.0 or MIT license that
// can be found in the LICENSE-APACHE and LICENSE-MIT files.

// This file ports hayro-interpret src/pattern.rs.

package interpret

import (
	"github.com/xo/okage/kurbo"

	"github.com/xo/ivy/internal/hayro/kurboext"
	"github.com/xo/ivy/internal/hayro/syntax"
)

// Pattern is Pattern in hayro-interpret src/pattern.rs. It is a
// *ShadingPattern or a *TilingPattern.
//
// Port: upstream is an enum. Go uses an interface that only the two types
// implement. Each Pattern is also a Paint.
type Pattern interface {
	Paint
	isPattern()
}

func (*ShadingPattern) isPattern() {}
func (*TilingPattern) isPattern()  {}

// newPattern is Pattern::new in hayro-interpret src/pattern.rs. A
// dictionary is a shading pattern, and a stream is a tiling pattern.
func newPattern(object syntax.Object, ctx *Context) (Pattern, bool) {
	switch o := object.(type) {
	case syntax.Dict:
		p, ok := newShadingPattern(o, ctx.interpreterCache.objectCache)
		if !ok {
			return nil, false
		}
		p.run = ctx.run
		return p, true
	case syntax.Stream:
		p, ok := newTilingPattern(o, ctx)
		if !ok {
			return nil, false
		}
		return p, true
	}
	return nil, false
}

// clonePattern is the derived Clone of Pattern in hayro-interpret
// src/pattern.rs. The shading stays shared, as the Arc of upstream does.
func clonePattern(p Pattern) Pattern {
	switch v := p.(type) {
	case *ShadingPattern:
		c := *v
		return &c
	case *TilingPattern:
		c := *v
		return &c
	}
	return p
}

// patternPreConcatTransform is Pattern::pre_concat_transform in
// hayro-interpret src/pattern.rs. The clip path of a shading is copied
// before it changes, as Arc::make_mut of upstream does.
func patternPreConcatTransform(p Pattern, transform kurbo.Affine) {
	switch v := p.(type) {
	case *ShadingPattern:
		v.Matrix = transform.Mul(v.Matrix)
		shading := *v.Shading
		if shading.ClipPath != nil {
			clip := kurboext.MulBezPath(v.Matrix, *shading.ClipPath)
			shading.ClipPath = &clip
		}
		v.Shading = &shading
	case *TilingPattern:
		v.Matrix = transform.Mul(v.Matrix)
	}
}

// patternSetOpacity is Pattern::set_opacity in hayro-interpret
// src/pattern.rs.
func patternSetOpacity(p Pattern, opacity float32) {
	switch v := p.(type) {
	case *ShadingPattern:
		v.Opacity = opacity
	case *TilingPattern:
		v.Opacity = opacity
	}
}

// patternSetTransferFunction is Pattern::set_transfer_function in
// hayro-interpret src/pattern.rs. A tiling pattern ignores it.
func patternSetTransferFunction(p Pattern, tf *ActiveTransferFunction) {
	if v, ok := p.(*ShadingPattern); ok {
		v.TransferFunction = tf
	}
}

// ShadingPattern is ShadingPattern in hayro-interpret src/pattern.rs. It is
// a pattern that paints a shading.
type ShadingPattern struct {
	// Shading is the shading of the pattern. It is shared, and a caller
	// must not change it.
	Shading *Shading
	// Matrix is the transform to apply before the drawing.
	Matrix kurbo.Affine
	// Opacity is an opacity to apply to the shading.
	Opacity float32
	// TransferFunction is a transfer function for the colors of the
	// shading, or nil.
	TransferFunction *ActiveTransferFunction

	// run lets the drawing of the shading stop with the interpretation.
	run *runState
}

// newShadingPattern is ShadingPattern::new in hayro-interpret
// src/pattern.rs.
func newShadingPattern(dict syntax.Dict, cache *Cache) (*ShadingPattern, bool) {
	obj, ok := dict.Get[syntax.Object](syntax.KeyShading)
	if !ok {
		return nil, false
	}
	sd, ss, ok := syntax.DictOrStream(obj)
	if !ok {
		return nil, false
	}
	shading, ok := newShading(sd, ss, cache)
	if !ok {
		return nil, false
	}
	matrix := kurbo.IdentityAffine
	if m, ok := dict.Get[[6]float64](syntax.KeyMatrix); ok {
		matrix = kurbo.NewAffine(m)
	}
	return &ShadingPattern{Shading: shading, Opacity: 1, Matrix: matrix}, true
}

// CacheKey is the CacheKey implementation of ShadingPattern in
// hayro-interpret src/pattern.rs.
func (p *ShadingPattern) CacheKey() Key128 {
	return newKeyHasher().key(p.Shading.CacheKey()).key(affineCacheKey(p.Matrix)).f32(p.Opacity).sum()
}

// TilingPattern is TilingPattern in hayro-interpret src/pattern.rs. It is a
// pattern that repeats a cell of content.
type TilingPattern struct {
	cacheKey Key128
	// BBox is the box of the cell.
	BBox kurbo.Rect
	// XStep is the step in the direction x.
	XStep float32
	// YStep is the step in the direction y.
	YStep float32
	// Matrix is the transform to apply before the drawing.
	Matrix kurbo.Affine
	// Opacity is an opacity to apply to the cell as a whole.
	Opacity float32

	stream           syntax.Stream
	isColor          bool
	strokePaint      Color
	nonStrokingPaint Color
	resources        *syntax.Resources
	cache            *InterpreterCache
	settings         InterpreterSettings
	xref             *syntax.XRef
	nestingDepth     uint32
	run              *runState
}

// String is the Debug implementation of TilingPattern in hayro-interpret
// src/pattern.rs.
func (p *TilingPattern) String() string {
	return "TilingPattern"
}

// newTilingPattern is TilingPattern::new in hayro-interpret src/pattern.rs.
func newTilingPattern(stream syntax.Stream, ctx *Context) (*TilingPattern, bool) {
	dict := stream.Dict()
	r, ok := dict.Get[syntax.Rect](syntax.KeyBBox)
	if !ok {
		return nil, false
	}
	bbox := RectToKurbo(r)
	xStep, ok := dict.Get[float32](syntax.KeyXStep)
	if !ok {
		return nil, false
	}
	yStep, ok := dict.Get[float32](syntax.KeyYStep)
	if !ok {
		return nil, false
	}
	if IsNearlyZero(xStep) || IsNearlyZero(yStep) || kurboext.RectIsZeroArea(bbox) {
		return nil, false
	}
	paintType, ok := dict.Get[uint8](syntax.KeyPaintType)
	if !ok {
		return nil, false
	}
	matrix := kurbo.IdentityAffine
	if m, ok := dict.Get[[6]float64](syntax.KeyMatrix); ok {
		matrix = kurbo.NewAffine(m)
	}
	fill := ctx.fillPaint()
	fillCS, ok := fill.colorSpace.patternCS()
	if !ok {
		fillCS = DeviceGray()
	}
	nonStrokingPaint := newColor(fillCS, fill.color, 1)
	stroke := ctx.strokePaint()
	strokeCS, ok := stroke.colorSpace.patternCS()
	if !ok {
		strokeCS = DeviceGray()
	}
	strokePaint := newColor(strokeCS, stroke.color, 1)
	resDict, ok := dict.Get[syntax.Dict](syntax.KeyResources)
	if !ok {
		resDict = syntax.EmptyDict()
	}
	return &TilingPattern{
		cacheKey:         streamCacheKey(stream),
		BBox:             bbox,
		XStep:            xStep,
		YStep:            yStep,
		Matrix:           matrix,
		Opacity:          1,
		isColor:          paintType == 1,
		stream:           stream,
		strokePaint:      strokePaint,
		nonStrokingPaint: nonStrokingPaint,
		settings:         ctx.settings,
		resources:        syntax.NewResources(resDict),
		cache:            ctx.interpreterCache,
		xref:             ctx.xref,
		nestingDepth:     ctx.nestingDepth + 1,
		run:              ctx.run,
	}, true
}

// Interpret is TilingPattern::interpret in hayro-interpret src/pattern.rs.
// It runs the content of one cell into the device, clipped to the box of
// the cell. It returns false when the content cannot be read.
func (p *TilingPattern) Interpret(device Device, initialTransform kurbo.Affine, isStroke bool) bool {
	defer recoverInto(p.run)
	// Port: upstream does not bound the nesting of patterns, so a pattern
	// that paints with itself recurses without end (D18).
	if p.nestingDepth >= maxNestedInterpretationDepth {
		return false
	}
	s := newState(initialTransform)
	ctx := newContextWith(s.ctm, initialTransform.TransformRectBbox(p.BBox), p.cache, p.xref, p.settings, s, p.nestingDepth, p.run)
	decoded, err := p.stream.DecodedData()
	if err != nil {
		return false
	}
	iter := syntax.NewTypedIter(decoded)
	device.PushClipPath(&ClipPath{
		Path: kurboext.MulBezPath(initialTransform, kurboext.RectToPath(p.BBox, 0.1)),
		Fill: FillRuleNonZero,
	})
	if p.isColor {
		Interpret(iter, p.resources, ctx, device)
	} else {
		var paint Paint = p.nonStrokingPaint
		if isStroke {
			paint = p.strokePaint
		}
		Interpret(iter, p.resources, ctx, &stencilPatternDevice{type3ShapeGlyphDevice{inner: device, paint: paint}})
	}
	device.PopClip()
	return true
}

// CacheKey is the CacheKey implementation of TilingPattern in
// hayro-interpret src/pattern.rs. The colors of an uncolored pattern are
// part of the key.
func (p *TilingPattern) CacheKey() Key128 {
	h := newKeyHasher().key(p.cacheKey).key(affineCacheKey(p.Matrix)).f32(p.Opacity)
	if !p.isColor {
		ns := p.nonStrokingPaint.ToRGBA().ToRGBA8()
		st := p.strokePaint.ToRGBA().ToRGBA8()
		h.boolean(true).bytes(ns[:]).bytes(st[:])
	} else {
		h.boolean(false)
	}
	return h.sum()
}

// stencilPatternDevice is StencilPatternDevice in hayro-interpret
// src/pattern.rs. It lets only the fills and the strokes of paths and the
// stencil masks through, with the paint of the pattern.
//
// Port: upstream repeats the code of Type3ShapeGlyphDevice of
// src/font/type3.rs. Go embeds that type.
type stencilPatternDevice struct {
	type3ShapeGlyphDevice
}
