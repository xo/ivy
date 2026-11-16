// Copyright (c) The Hayro Authors.
//
// Use of this source code is governed by the Apache-2.0 or MIT license that
// can be found in the LICENSE-APACHE and LICENSE-MIT files.

// This file ports hayro-font src/type3.rs.

package pdffont

import (
	"slices"
	"sort"

	"github.com/xo/okage/kurbo"

	"github.com/xo/ivy/pdf/internal/cmap"
	"github.com/xo/ivy/pdf/internal/syntax"
)

// Type3Glyph is Type3Glyph in hayro-font src/type3.rs. It is a glyph of a
// Type 3 font, defined by a content stream of the font.
type Type3Glyph struct {
	font     *Type3Font
	id       GlyphID
	charCode uint32
}

// Program is Type3Glyph::program in hayro-font src/type3.rs. It returns the
// content stream that draws the glyph, and false if the font has none.
func (g *Type3Glyph) Program() (syntax.Stream, bool) {
	return g.font.glyphProgram(g.id)
}

// Font is Type3Glyph::font in hayro-font src/type3.rs. It returns the font
// of the glyph.
func (g *Type3Glyph) Font() *Type3Font {
	return g.font
}

// AsUnicode is Type3Glyph::as_unicode in hayro-font src/type3.rs. It returns
// the Unicode text of the glyph, and false if it is not known.
func (g *Type3Glyph) AsUnicode() (cmap.BfString, bool) {
	return g.font.charCodeToUnicode(g.charCode)
}

// GlyphID is Type3Glyph::glyph_id in hayro-font src/type3.rs. It returns the
// glyph ID within the font.
func (g *Type3Glyph) GlyphID() GlyphID {
	return g.id
}

// CharCode is Type3Glyph::char_code in hayro-font src/type3.rs. It returns
// the character code that selected this glyph.
func (g *Type3Glyph) CharCode() uint32 {
	return g.charCode
}

// AdvanceWidth is Type3Glyph::advance_width in hayro-font src/type3.rs. It
// returns the advance width of the glyph, scaled to UnitsPerEm.
func (g *Type3Glyph) AdvanceWidth() float32 {
	return g.font.glyphWidth(g.charCode)
}

// CacheKey is Type3Glyph::cache_key in hayro-font src/type3.rs. It returns
// the cache key of the glyph.
func (g *Type3Glyph) CacheKey() Key128 {
	return newKeyHasher().key(g.font.cacheKey()).u64(uint64(g.id)).sum()
}

// newType3Glyph is Type3Glyph::new in hayro-font src/type3.rs.
func newType3Glyph(font *Type3Font, id GlyphID, charCode uint32) *Type3Glyph {
	return &Type3Glyph{font: font, id: id, charCode: charCode}
}

// Type3Font is Type3Font in hayro-font src/type3.rs. It is a Type 3 font.
type Type3Font struct {
	widths       []width
	missingWidth float32
	encoding     encoding
	encodings    map[uint8]string
	dict         syntax.Dict
	charProcs    map[string]syntax.Stream
	glyphNames   []string
	matrix       kurbo.Affine
	toUnicode    *cmap.CMap
}

// Matrix is Type3Font::matrix in hayro-font src/type3.rs. It returns the
// font matrix.
func (t *Type3Font) Matrix() kurbo.Affine {
	return t.matrix
}

// Resources is Type3Font::resources in hayro-font src/type3.rs. It returns
// the resources of the font, and false if it has none.
func (t *Type3Font) Resources() (syntax.Dict, bool) {
	return t.dict.Get[syntax.Dict](syntax.KeyResources)
}

// newType3Font is Type3Font::new in hayro-font src/type3.rs.
func newType3Font(dict syntax.Dict, cmapResolver CMapResolverFn) (*Type3Font, bool) {
	enc, encodings := readEncoding(dict)
	widths, missingWidth, ok := readWidths(dict, dict)
	if !ok {
		return nil, false
	}
	m, ok := dict.Get[[6]float64](syntax.KeyFontMatrix)
	if !ok {
		m = [6]float64{0.001, 0, 0, 0.001, 0, 0}
	}

	procs := make(map[string]syntax.Stream)
	procDict, _ := dict.Get[syntax.Dict](syntax.KeyCharProcs)
	for _, name := range procDict.Keys() {
		if prog, ok := procDict.Get[syntax.Stream](name); ok {
			procs[name.AsStr()] = prog
		}
	}

	toUnicode, _ := readToUnicode(dict, cmapResolver)

	// Glyph IDs are indices into `glyph_names` plus one, keeping 0 for
	// `.notdef`.
	glyphNames := make([]string, 0, len(procs))
	for name := range procs {
		glyphNames = append(glyphNames, name)
	}
	slices.Sort(glyphNames)

	return &Type3Font{
		glyphNames:   glyphNames,
		encoding:     enc,
		charProcs:    procs,
		widths:       widths,
		missingWidth: missingWidth,
		encodings:    encodings,
		matrix:       kurbo.NewAffine(m),
		dict:         dict,
		toUnicode:    toUnicode,
	}, true
}

// glyphProgram is Type3Font::glyph_program in hayro-font src/type3.rs.
func (t *Type3Font) glyphProgram(glyph GlyphID) (syntax.Stream, bool) {
	if glyph == 0 || int(glyph-1) >= len(t.glyphNames) {
		return syntax.Stream{}, false
	}
	s, ok := t.charProcs[t.glyphNames[glyph-1]]
	return s, ok
}

// glyphWidth is Type3Font::glyph_width in hayro-font src/type3.rs.
func (t *Type3Font) glyphWidth(code uint32) float32 {
	w := t.missingWidth
	if uint64(code) < uint64(len(t.widths)) && !t.widths[code].missing {
		w = t.widths[code].value
	}
	return (w * float32(t.matrix[0])) * UnitsPerEm
}

// charCodeToUnicode is Type3Font::char_code_to_unicode in hayro-font
// src/type3.rs.
func (t *Type3Font) charCodeToUnicode(charCode uint32) (cmap.BfString, bool) {
	if t.toUnicode != nil {
		// Some PDFs wrongly map codes to U+0000 in `ToUnicode`.
		if c, ok := t.toUnicode.LookupBfString(charCode); ok && c != cmap.NewBfChar(0) {
			return c, true
		}
	}
	if charCode > 255 {
		return cmap.BfString{}, false
	}
	name, ok := t.codeToPSName(uint8(charCode))
	if !ok {
		return cmap.BfString{}, false
	}
	return glyphNameToUnicode(name, false)
}

// cacheKey is Type3Font::cache_key in hayro-font src/type3.rs.
func (t *Type3Font) cacheKey() Key128 {
	return dictCacheKey(t.dict)
}

// mapCode is Type3Font::map_code in hayro-font src/type3.rs.
func (t *Type3Font) mapCode(code uint8) GlyphID {
	name, ok := t.codeToPSName(code)
	if !ok {
		return GlyphIDNotdef
	}
	index := sort.SearchStrings(t.glyphNames, name)
	if index >= len(t.glyphNames) || t.glyphNames[index] != name {
		return GlyphIDNotdef
	}
	return NewGlyphID(uint32(index) + 1)
}

// codeToPSName is Type3Font::code_to_ps_name in hayro-font src/type3.rs.
func (t *Type3Font) codeToPSName(code uint8) (string, bool) {
	if s, ok := t.encodings[code]; ok {
		return s, true
	}
	return t.encoding.mapCode(code)
}
