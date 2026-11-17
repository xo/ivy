// Copyright (c) The Hayro Authors.
//
// Use of this source code is governed by the Apache-2.0 or MIT license that
// can be found in the LICENSE-APACHE and LICENSE-MIT files.

// This file ports hayro-interpret src/font/type3.rs.

package interpret

import (
	"github.com/xo/okage/kurbo"

	"github.com/xo/ivy/internal/hayro/kurboext"
	"github.com/xo/ivy/internal/hayro/pdffont"
	"github.com/xo/ivy/internal/hayro/syntax"
)

// renderType3Glyph is render_glyph in hayro-interpret src/font/type3.rs. It
// runs the content stream of the glyph. A glyph that starts with d1 is a
// shape, and the paint of the text paints it.
func renderType3Glyph(glyph *Type3Glyph, transform, glyphTransform kurbo.Affine, paint Paint, device Device) {
	// Port: upstream does not bound the nesting of glyphs of Type 3, so a
	// glyph that shows itself recurses without end (D18).
	if glyph.nestingDepth >= maxNestedInterpretationDepth {
		return
	}
	s := glyph.state
	font := glyph.glyph.Font()
	rootTransform := transform.Mul(glyphTransform).Mul(font.Matrix()).Mul(kurbo.AffineScale(float64(pdffont.UnitsPerEm)))
	s.ctm = rootTransform
	// The text state is reset for the procedures of glyphs. The graphics
	// state is kept.
	s.textState = defaultTextState()

	program, ok := glyph.glyph.Program()
	if !ok {
		return
	}
	decoded, err := program.DecodedData()
	if err != nil {
		return
	}
	isShapeGlyph := true
	scan := syntax.NewTypedIter(decoded)
	for {
		op, ok := scan.Next()
		if !ok {
			break
		}
		if !glyph.run.step(1) {
			return
		}
		if _, ok := op.(syntax.ShapeGlyph); ok {
			break
		}
		if _, ok := op.(syntax.ColorGlyph); ok {
			isShapeGlyph = false
			break
		}
	}

	ctx := newContextWith(s.ctm, glyph.bbox, glyph.cache, glyph.xref, glyph.settings, s, glyph.nestingDepth, glyph.run)

	resources := glyph.parentResources
	if d, ok := program.Dict().Get[syntax.Dict](syntax.KeyResources); ok {
		resources = syntax.NewResources(d)
	} else if d, ok := font.Resources(); ok {
		resources = syntax.NewResources(d)
	}
	if resources == nil {
		resources = syntax.NewResources(syntax.EmptyDict())
	}

	iter := syntax.NewTypedIter(decoded)
	if isShapeGlyph {
		Interpret(iter, resources, ctx, &type3ShapeGlyphDevice{inner: device, paint: paint})
	} else {
		Interpret(iter, resources, ctx, device)
	}
}

// type3ShapeGlyphDevice is Type3ShapeGlyphDevice in hayro-interpret
// src/font/type3.rs. It lets only the fills and the strokes of paths and the
// stencil masks through, with the paint of the text.
type type3ShapeGlyphDevice struct {
	inner Device
	paint Paint
}

// DrawPath draws the path with the paint of the text.
func (d *type3ShapeGlyphDevice) DrawPath(path *kurboext.BezPath, props DrawProps, mode DrawMode) {
	props.Paint = d.paint
	d.inner.DrawPath(path, props, mode)
}

// PushClipPath forwards the clip.
func (d *type3ShapeGlyphDevice) PushClipPath(clip *ClipPath) {
	d.inner.PushClipPath(clip)
}

// PushTransparencyGroup does nothing.
func (d *type3ShapeGlyphDevice) PushTransparencyGroup(float32, *SoftMask, BlendMode, bool, bool) {}

// DrawGlyphRun forwards the run.
func (d *type3ShapeGlyphDevice) DrawGlyphRun(run *GlyphRun, props DrawProps, mode DrawMode) {
	d.inner.DrawGlyphRun(run, props, mode)
}

// PopClip forwards the pop.
func (d *type3ShapeGlyphDevice) PopClip() {
	d.inner.PopClip()
}

// PopTransparencyGroup does nothing.
func (d *type3ShapeGlyphDevice) PopTransparencyGroup() {}

// DrawImage draws a stencil mask with the paint of the text, and drops any
// other image.
func (d *type3ShapeGlyphDevice) DrawImage(image Image, props ImageDrawProps) {
	if s, ok := image.(*StencilImage); ok {
		c := *s
		c.paint = d.paint
		d.inner.DrawImage(&c, props)
	}
}
