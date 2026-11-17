// Copyright (c) The Hayro Authors.
//
// Use of this source code is governed by the Apache-2.0 or MIT license that
// can be found in the LICENSE-APACHE and LICENSE-MIT files.

// This file ports hayro-font src/outline.rs.

package pdffont

import "github.com/xo/ivy/internal/hayro/cmap"

// OutlineFontData is OutlineFontData in hayro-font src/outline.rs. It holds
// the data and the metadata of an outline font.
//
// Port: upstream holds PostscriptName and Weight in an Option. Go uses the
// empty string and 0 for None.
type OutlineFontData struct {
	// Data holds the raw font bytes.
	Data FontData
	// CacheKey is a key that identifies the font data.
	CacheKey Key128
	// PostscriptName is the PostScript name.
	PostscriptName string
	// Weight is the font weight.
	Weight uint32
	// IsItalic is whether the font is italic.
	IsItalic bool
	// IsSerif is whether the font is serif.
	IsSerif bool
	// IsMonospace is whether the font is monospace.
	IsMonospace bool
}

// outlineFont is OutlineFont in hayro-font src/outline.rs.
//
// Port: upstream is an enum. Go sets one of the three fields.
type outlineFont struct {
	type1    *type1Font
	trueType *trueTypeFont
	type0    *type0Font
}

// program is OutlineFont::program in hayro-font src/outline.rs.
func (o outlineFont) program() *fontProgram {
	switch {
	case o.type1 != nil:
		return o.type1.program()
	case o.trueType != nil:
		return o.trueType.program()
	}
	return o.type0.program()
}

// hidesNotdef is OutlineFont::hides_notdef in hayro-font src/outline.rs.
// Standard fonts have an empty `.notdef`. Their substitutes can draw a
// glyph there.
func (o outlineFont) hidesNotdef() bool {
	switch {
	case o.type1 != nil:
		return o.type1.isStandard()
	case o.trueType != nil:
		return o.trueType.isStandard()
	}
	return false
}

// charCodeToUnicode is OutlineFont::char_code_to_unicode in hayro-font
// src/outline.rs.
func (o outlineFont) charCodeToUnicode(charCode uint32) (cmap.BfString, bool) {
	switch {
	case o.type1 != nil:
		return o.type1.charCodeToUnicode(charCode)
	case o.trueType != nil:
		return o.trueType.charCodeToUnicode(charCode)
	}
	return o.type0.charCodeToUnicode(charCode)
}

// glyphAdvanceWidth is OutlineFont::glyph_advance_width in hayro-font
// src/outline.rs.
func (o outlineFont) glyphAdvanceWidth(charCode uint32) (float32, bool) {
	switch {
	case o.type1 != nil:
		return o.type1.glyphWidth(uint8(charCode))
	case o.trueType != nil:
		return o.trueType.glyphWidth(uint8(charCode)), true
	}
	return float32(o.type0.codeAdvance(charCode).X), true
}

// fontData is OutlineFont::font_data in hayro-font src/outline.rs.
func (o outlineFont) fontData() (OutlineFontData, bool) {
	cacheKey := o.program().contentHash()
	switch {
	case o.type1 != nil:
		return OutlineFontData{}, false
	case o.trueType != nil:
		t := o.trueType
		data, ok := t.fontData()
		if !ok {
			return OutlineFontData{}, false
		}
		weight, _ := t.weight()
		return OutlineFontData{
			Data:           data,
			CacheKey:       cacheKey,
			PostscriptName: t.postscriptName(),
			Weight:         weight,
			IsItalic:       t.isItalic(),
			IsSerif:        t.isSerif(),
			IsMonospace:    t.isMonospace(),
		}, true
	}
	t := o.type0
	data, ok := t.fontData()
	if !ok {
		return OutlineFontData{}, false
	}
	weight, _ := t.weight()
	return OutlineFontData{
		Data:           data,
		CacheKey:       cacheKey,
		PostscriptName: t.postscriptName(),
		Weight:         weight,
		IsItalic:       t.isItalic(),
		IsSerif:        t.isSerif(),
		IsMonospace:    t.isMonospace(),
	}, true
}
