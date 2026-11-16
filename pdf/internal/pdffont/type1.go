// Copyright (c) The Hayro Authors.
//
// Use of this source code is governed by the Apache-2.0 or MIT license that
// can be found in the LICENSE-APACHE and LICENSE-MIT files.

// This file ports hayro-font src/type1.rs.

package pdffont

import (
	"github.com/xo/ivy/pdf/internal/cmap"
	"github.com/xo/ivy/pdf/internal/syntax"
)

// type1Font is Type1Font in hayro-font src/type1.rs. It is a simple font of
// the subtype Type1 or MMType1, with an embedded program or a standard font.
//
// Port: upstream holds the enum Kind. Go sets standard or embedded.
type type1Font struct {
	standard  *standardKind
	embedded  *type1Embedded
	toUnicode *cmap.CMap
}

// newType1Font is Type1Font::new in hayro-font src/type1.rs.
func newType1Font(dict syntax.Dict, loader *fontLoader) (*type1Font, bool) {
	toUnicode, _ := readToUnicode(dict, loader.cmapResolver)

	fallback := func() (*type1Font, bool) {
		// Upstream notes that it does not use the fallback fonts yet.
		query := newFallbackFontQuery(dict)
		standardFont := query.PickStandardFont()
		s, ok := newStandardKindWith(dict, standardFont, true, loader.programs)
		if !ok {
			return nil, false
		}
		return &type1Font{standard: s, toUnicode: toUnicode}, true
	}

	if embedded, ok := newType1Embedded(dict, loader); ok {
		return &type1Font{embedded: embedded, toUnicode: toUnicode}, true
	}
	if hasFontFile(dict) {
		return fallback()
	}
	if s, ok := newStandardKind(dict, loader.programs); ok {
		return &type1Font{standard: s, toUnicode: toUnicode}, true
	}
	return fallback()
}

// newStandardType1Font is Type1Font::new_standard in hayro-font
// src/type1.rs.
func newStandardType1Font(font StandardFont, loader *programLoader) (*type1Font, bool) {
	s, ok := newStandardKindWith(syntax.EmptyDict(), font, true, loader)
	if !ok {
		return nil, false
	}
	return &type1Font{standard: s}, true
}

// mapCode is Type1Font::map_code in hayro-font src/type1.rs.
func (t *type1Font) mapCode(code uint8) GlyphID {
	if t.standard != nil {
		return t.standard.mapCode(code)
	}
	return t.embedded.mapCode(code)
}

// program is Type1Font::program in hayro-font src/type1.rs.
func (t *type1Font) program() *fontProgram {
	if t.standard != nil {
		return t.standard.program
	}
	return t.embedded.program
}

// isStandard is Type1Font::is_standard in hayro-font src/type1.rs.
func (t *type1Font) isStandard() bool {
	return t.standard != nil
}

// glyphStretch is Type1Font::glyph_stretch in hayro-font src/type1.rs.
func (t *type1Font) glyphStretch(glyph GlyphID, code uint8) float64 {
	if t.standard != nil {
		return t.standard.glyphStretch(glyph, code)
	}
	return 1
}

// glyphWidth is Type1Font::glyph_width in hayro-font src/type1.rs.
func (t *type1Font) glyphWidth(code uint8) (float32, bool) {
	if t.standard != nil {
		return t.standard.glyphWidth(code)
	}
	return t.embedded.glyphWidth(code)
}

// charCodeToUnicode is Type1Font::char_code_to_unicode in hayro-font
// src/type1.rs.
func (t *type1Font) charCodeToUnicode(charCode uint32) (cmap.BfString, bool) {
	if t.toUnicode != nil {
		// Some PDFs wrongly map codes to U+0000 in `ToUnicode`.
		if c, ok := t.toUnicode.LookupBfString(charCode); ok && c != cmap.NewBfChar(0) {
			return c, true
		}
	}
	code := uint8(charCode)
	if t.standard != nil {
		return t.standard.charCodeToUnicode(code)
	}
	return t.embedded.charCodeToUnicode(code)
}

// type1Embedded is EmbeddedKind in hayro-font src/type1.rs.
type type1Embedded struct {
	program         *fontProgram
	encoding        encoding
	widths          []width
	missingWidth    float32
	encodings       map[uint8]string
	standardFont    StandardFont
	hasStandardFont bool
}

// newType1Embedded is EmbeddedKind::new in hayro-font src/type1.rs.
func newType1Embedded(dict syntax.Dict, loader *fontLoader) (*type1Embedded, bool) {
	descriptor, ok := dict.Get[syntax.Dict](syntax.KeyFontDescriptor)
	if !ok {
		return nil, false
	}
	data, ok := descriptor.Get[syntax.Stream](syntax.KeyFontFile3)
	if !ok {
		data, ok = descriptor.Get[syntax.Stream](syntax.KeyFontFile)
	}
	if !ok {
		return nil, false
	}
	program, ok := loader.programs.load(data)
	if !ok {
		return nil, false
	}
	enc, encodings := readEncoding(dict)
	widths, missingWidth, ok := readWidths(dict, descriptor)
	if !ok {
		return nil, false
	}
	sf, _, hasSF := selectStandardFont(dict, descriptor)
	return &type1Embedded{
		program:         program,
		encoding:        enc,
		widths:          widths,
		missingWidth:    missingWidth,
		encodings:       encodings,
		standardFont:    sf,
		hasStandardFont: hasSF,
	}, true
}

// glyphByName is EmbeddedKind::glyph_by_name in hayro-font src/type1.rs.
func (e *type1Embedded) glyphByName(name string) (GlyphID, bool) {
	if g, ok := e.program.glyphByName(name); ok {
		return g, true
	}
	return e.program.glyphByName(normalizedGlyphName(name))
}

// mapCode is EmbeddedKind::map_code in hayro-font src/type1.rs.
func (e *type1Embedded) mapCode(code uint8) GlyphID {
	var g GlyphID
	var ok bool
	if entry, eok := e.encodings[code]; eok {
		g, ok = e.glyphByName(entry)
	} else if e.encoding == encodingBuiltIn {
		// Deliberately uses the CFF encoding of OpenType programs instead
		// of their `cmap`.
		g, ok = e.program.builtinEncodingGlyph(code)
	} else if name, nok := e.encoding.mapCode(code); nok {
		g, ok = e.glyphByName(name)
	}
	if !ok {
		return GlyphIDNotdef
	}
	return g
}

// codeToPSName is EmbeddedKind::code_to_ps_name in hayro-font
// src/type1.rs.
func (e *type1Embedded) codeToPSName(code uint8) (string, bool) {
	if entry, ok := e.encodings[code]; ok {
		return entry, true
	}
	if e.encoding == encodingBuiltIn {
		return e.program.builtinEncodingGlyphName(code)
	}
	return e.encoding.mapCode(code)
}

// glyphWidth is EmbeddedKind::glyph_width in hayro-font src/type1.rs.
func (e *type1Embedded) glyphWidth(code uint8) (float32, bool) {
	if int(code) < len(e.widths) {
		if e.widths[code].missing {
			return e.missingWidth, true
		}
		return e.widths[code].value, true
	}
	if !e.hasStandardFont {
		return 0, false
	}
	name, ok := e.codeToPSName(code)
	if !ok {
		return 0, false
	}
	return e.standardFont.getWidth(name)
}

// charCodeToUnicode is EmbeddedKind::char_code_to_unicode in hayro-font
// src/type1.rs.
func (e *type1Embedded) charCodeToUnicode(code uint8) (cmap.BfString, bool) {
	isZapfDingbats := e.hasStandardFont && e.standardFont == ZapfDingBats
	name, ok := e.codeToPSName(code)
	if !ok {
		return cmap.BfString{}, false
	}
	return glyphNameToUnicode(name, isZapfDingbats)
}

// hasFontFile is has_font_file in hayro-font src/type1.rs.
func hasFontFile(dict syntax.Dict) bool {
	descriptor, ok := dict.Get[syntax.Dict](syntax.KeyFontDescriptor)
	return ok && (descriptor.ContainsKey(syntax.KeyFontFile) || descriptor.ContainsKey(syntax.KeyFontFile3))
}
