// Copyright (c) The Hayro Authors.
//
// Use of this source code is governed by the Apache-2.0 or MIT license that
// can be found in the LICENSE-APACHE and LICENSE-MIT files.

// This file ports hayro-interpret src/font/mod.rs.

package interpret

import (
	"github.com/xo/okage/kurbo"

	"github.com/xo/ivy/internal/hayro/cmap"
	"github.com/xo/ivy/internal/hayro/pdffont"
	"github.com/xo/ivy/internal/hayro/syntax"
)

// The types of hayro-font that hayro-interpret src/font/mod.rs exports.
type (
	// FallbackFontQuery is FallbackFontQuery of hayro-font.
	FallbackFontQuery = pdffont.FallbackFontQuery
	// FontData is FontData of hayro-font.
	FontData = pdffont.FontData
	// FontQuery is FontQuery of hayro-font.
	FontQuery = pdffont.FontQuery
	// FontStretch is FontStretch of hayro-font.
	FontStretch = pdffont.FontStretch
	// GlyphID is GlyphId of hayro-font.
	GlyphID = pdffont.GlyphID
	// GlyphIdentifier is GlyphIdentifier of hayro-font.
	GlyphIdentifier = pdffont.GlyphIdentifier
	// HintingCache is HintingCache of hayro-font.
	HintingCache = pdffont.HintingCache
	// OutlineFontData is OutlineFontData of hayro-font.
	OutlineFontData = pdffont.OutlineFontData
	// OutlineGlyph is OutlineGlyph of hayro-font.
	OutlineGlyph = pdffont.OutlineGlyph
	// StandardFont is StandardFont of hayro-font.
	StandardFont = pdffont.StandardFont
)

// Glyph is Glyph in hayro-interpret src/font/mod.rs. It is a glyph that can
// be drawn: an *OutlineGlyph, defined by an outline, or a *Type3Glyph,
// defined by the operators of PDF.
//
// Port: upstream is an enum. Go uses an interface that only the two types
// implement. A device tells them apart with a type switch.
type Glyph interface {
	// AsUnicode is Glyph::as_unicode. It returns the Unicode text of the
	// glyph, and false if it is not known.
	AsUnicode() (cmap.BfString, bool)
	// CharCode is Glyph::char_code. It returns the character code, not
	// the CID, that selected the glyph.
	CharCode() uint32
}

// PositionedGlyph is PositionedGlyph in hayro-interpret src/font/mod.rs. It
// is a glyph and its place in a GlyphRun.
type PositionedGlyph struct {
	Glyph
	transform kurbo.Affine
}

// Transform is PositionedGlyph::transform in hayro-interpret
// src/font/mod.rs. It returns the transform that places the glyph in the
// space of the text.
func (p *PositionedGlyph) Transform() kurbo.Affine {
	return p.transform
}

// GlyphRun is GlyphRun in hayro-interpret src/font/mod.rs. It is a run of
// glyphs with their places. The run is valid only during the call of the
// device that receives it.
type GlyphRun struct {
	glyphs      []PositionedGlyph
	writingMode cmap.WritingMode
}

// Glyphs is GlyphRun::glyphs in hayro-interpret src/font/mod.rs. The caller
// must not change them.
func (r *GlyphRun) Glyphs() []PositionedGlyph {
	return r.glyphs
}

// WritingMode is GlyphRun::writing_mode in hayro-interpret src/font/mod.rs.
// It returns the writing mode of the font of the run.
func (r *GlyphRun) WritingMode() cmap.WritingMode {
	return r.writingMode
}

// Type3Glyph is Type3Glyph in hayro-interpret src/font/mod.rs. It is a
// glyph of a Type 3 font, with the state that its content stream needs.
type Type3Glyph struct {
	glyph           *pdffont.Type3Glyph
	state           state
	bbox            kurbo.Rect
	parentResources *syntax.Resources
	cache           *InterpreterCache
	xref            *syntax.XRef
	settings        InterpreterSettings
	nestingDepth    uint32
	run             *runState
}

// Interpret is Type3Glyph::interpret in hayro-interpret src/font/mod.rs. It
// runs the content stream of the glyph into the device. transform is the
// transform of the run, and glyphTransform is the transform of the glyph
// in the run.
func (g *Type3Glyph) Interpret(device Device, transform, glyphTransform kurbo.Affine, paint Paint) {
	defer recoverInto(g.run)
	renderType3Glyph(g, transform, glyphTransform, paint, device)
}

// AsUnicode is Type3Glyph::as_unicode in hayro-interpret src/font/mod.rs.
func (g *Type3Glyph) AsUnicode() (cmap.BfString, bool) {
	return g.glyph.AsUnicode()
}

// CharCode is Type3Glyph::char_code in hayro-interpret src/font/mod.rs.
func (g *Type3Glyph) CharCode() uint32 {
	return g.glyph.CharCode()
}

// AdvanceWidth is Type3Glyph::advance_width in hayro-interpret
// src/font/mod.rs.
func (g *Type3Glyph) AdvanceWidth() (float32, bool) {
	return g.glyph.AdvanceWidth(), true
}

// CacheKey is the CacheKey implementation of Type3Glyph in hayro-interpret
// src/font/mod.rs.
func (g *Type3Glyph) CacheKey() Key128 {
	return pairCacheKey(g.glyph.CacheKey(), rectCacheKey(g.bbox))
}

// GlyphIdentifierCacheKey is the CacheKey implementation of
// GlyphIdentifier in hayro-interpret src/font/mod.rs.
func GlyphIdentifierCacheKey(g GlyphIdentifier) Key128 {
	return g.GlyphKey()
}

// getGlyph is get_glyph in hayro-interpret src/font/mod.rs.
func getGlyph(font *pdffont.Font, glyph pdffont.GlyphID, charCode uint32, ctx *Context, resources *syntax.Resources) Glyph {
	switch g := font.Glyph(glyph, charCode).(type) {
	case *pdffont.OutlineGlyph:
		return g
	case *pdffont.Type3Glyph:
		return &Type3Glyph{
			glyph:           g,
			state:           ctx.snapshot(),
			bbox:            ctx.currentBbox(),
			parentResources: resources,
			cache:           ctx.interpreterCache,
			xref:            ctx.xref,
			settings:        ctx.settings,
			nestingDepth:    ctx.nestingDepth + 1,
			run:             ctx.run,
		}
	}
	return nil
}
