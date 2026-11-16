// Copyright (c) The Hayro Authors.
//
// Use of this source code is governed by the Apache-2.0 or MIT license that
// can be found in the LICENSE-APACHE and LICENSE-MIT files.

// This file ports hayro-font src/true_type.rs.

package pdffont

import (
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/xo/ivy/pdf/internal/cmap"
	"github.com/xo/ivy/pdf/internal/syntax"
)

// trueTypeFont is TrueTypeFont in hayro-font src/true_type.rs.
//
// Port: upstream holds the enum Kind. Go sets embedded or standard.
type trueTypeFont struct {
	embedded  *trueTypeEmbedded
	standard  *standardKind
	toUnicode *cmap.CMap
}

// newTrueTypeFont is TrueTypeFont::new in hayro-font src/true_type.rs.
func newTrueTypeFont(dict syntax.Dict, loader *fontLoader) (*trueTypeFont, bool) {
	toUnicode, _ := readToUnicode(dict, loader.cmapResolver)
	if embedded, ok := newTrueTypeEmbedded(dict, loader); ok {
		return &trueTypeFont{embedded: embedded, toUnicode: toUnicode}, true
	}
	fallback := func() (*trueTypeFont, bool) {
		query := newFallbackFontQuery(dict)
		standardFont := query.PickStandardFont()
		s, ok := newStandardKindWith(dict, standardFont, true, loader.programs)
		if !ok {
			return nil, false
		}
		return &trueTypeFont{standard: s, toUnicode: toUnicode}, true
	}
	if s, ok := newStandardKind(dict, loader.programs); ok {
		return &trueTypeFont{standard: s, toUnicode: toUnicode}, true
	}
	return fallback()
}

// program is TrueTypeFont::program in hayro-font src/true_type.rs.
func (t *trueTypeFont) program() *fontProgram {
	if t.embedded != nil {
		return t.embedded.baseFont
	}
	return t.standard.program
}

// isStandard is TrueTypeFont::is_standard in hayro-font src/true_type.rs.
func (t *trueTypeFont) isStandard() bool {
	return t.standard != nil
}

// glyphStretch is TrueTypeFont::glyph_stretch in hayro-font
// src/true_type.rs.
func (t *trueTypeFont) glyphStretch(glyph GlyphID, code uint8) float64 {
	if t.embedded != nil {
		return 1
	}
	return t.standard.glyphStretch(glyph, code)
}

// fontData is TrueTypeFont::font_data in hayro-font src/true_type.rs.
func (t *trueTypeFont) fontData() (FontData, bool) {
	if t.embedded != nil {
		return t.embedded.baseFont.fontData(), true
	}
	return nil, false
}

// postscriptName is TrueTypeFont::postscript_name in hayro-font
// src/true_type.rs. It returns the empty string for None.
func (t *trueTypeFont) postscriptName() string {
	if t.embedded != nil {
		return t.embedded.postscriptName
	}
	return ""
}

// weight is TrueTypeFont::weight in hayro-font src/true_type.rs.
func (t *trueTypeFont) weight() (uint32, bool) {
	if t.embedded != nil {
		return t.embedded.baseFont.weight()
	}
	if t.standard.isBold() {
		return 700, true
	}
	return 400, true
}

// isItalic is TrueTypeFont::is_italic in hayro-font src/true_type.rs.
func (t *trueTypeFont) isItalic() bool {
	if e := t.embedded; e != nil {
		if e.hasFlags && e.fontFlags.contains(fontFlagItalic) {
			return true
		}
		return e.baseFont.isItalic()
	}
	return t.standard.isItalic()
}

// isSerif is TrueTypeFont::is_serif in hayro-font src/true_type.rs.
func (t *trueTypeFont) isSerif() bool {
	if e := t.embedded; e != nil {
		return e.hasFlags && e.fontFlags.contains(fontFlagSerif)
	}
	return t.standard.isSerif()
}

// isMonospace is TrueTypeFont::is_monospace in hayro-font src/true_type.rs.
func (t *trueTypeFont) isMonospace() bool {
	if e := t.embedded; e != nil {
		if e.hasFlags && e.fontFlags.contains(fontFlagFixedPitch) {
			return true
		}
		return e.baseFont.isMonospace()
	}
	return t.standard.isMonospace()
}

// mapCode is TrueTypeFont::map_code in hayro-font src/true_type.rs.
func (t *trueTypeFont) mapCode(code uint8) GlyphID {
	if t.embedded != nil {
		return t.embedded.mapCode(code)
	}
	return t.standard.mapCode(code)
}

// glyphWidth is TrueTypeFont::glyph_width in hayro-font src/true_type.rs.
func (t *trueTypeFont) glyphWidth(code uint8) float32 {
	if t.embedded != nil {
		return t.embedded.glyphWidth(code)
	}
	w, _ := t.standard.glyphWidth(code)
	return w
}

// charCodeToUnicode is TrueTypeFont::char_code_to_unicode in hayro-font
// src/true_type.rs.
func (t *trueTypeFont) charCodeToUnicode(code uint32) (cmap.BfString, bool) {
	if t.toUnicode != nil {
		if c, ok := t.toUnicode.LookupBfString(code); ok {
			return c, true
		}
	}
	if e := t.embedded; e != nil {
		name, ok := e.codeToName(uint8(code))
		if !ok {
			return cmap.BfString{}, false
		}
		return glyphNameToUnicode(name, e.isZapfDingbats)
	}
	return t.standard.charCodeToUnicode(uint8(code))
	// Upstream notes that the test PDFs below fail, but mutool can render
	// them correctly. There is likely some other strategy that requires
	// processing the font tables.
	// tests/pdfs/custom/font_truetype_7.pdf
	// tests/pdfs/custom/font_truetype_6.pdf
}

// trueTypeEmbedded is EmbeddedKind in hayro-font src/true_type.rs.
type trueTypeEmbedded struct {
	baseFont       *fontProgram
	widths         []width
	missingWidth   float32
	fontFlags      fontFlags
	hasFlags       bool
	encoding       encoding
	differences    map[uint8]string
	codeToGlyph    [256]GlyphID
	postscriptName string
	isZapfDingbats bool
}

// newTrueTypeEmbedded is EmbeddedKind::new in hayro-font src/true_type.rs.
func newTrueTypeEmbedded(dict syntax.Dict, loader *fontLoader) (*trueTypeEmbedded, bool) {
	descriptor, _ := dict.Get[syntax.Dict](syntax.KeyFontDescriptor)
	var flags fontFlags
	hasFlags := false
	if bits, ok := descriptor.Get[uint32](syntax.KeyFlags); ok {
		flags, hasFlags = fontFlagsFromBits(bits)
	}
	widths, missingWidth, ok := readWidths(dict, descriptor)
	if !ok {
		return nil, false
	}
	enc, differences := readEncoding(dict)
	stream, ok := descriptor.Get[syntax.Stream](syntax.KeyFontFile2)
	if !ok {
		return nil, false
	}
	baseFont, ok := loader.programs.load(stream)
	if !ok || baseFont.programKind() != programSfnt {
		return nil, false
	}
	var postscriptName string
	if n, ok := dict.Get[syntax.Name](syntax.KeyBaseFont); ok {
		postscriptName = stripSubsetPrefix(n.AsStr())
	}
	sf, _, ok := selectStandardFont(dict, descriptor)
	k := &trueTypeEmbedded{
		baseFont:       baseFont,
		differences:    differences,
		widths:         widths,
		missingWidth:   missingWidth,
		fontFlags:      flags,
		hasFlags:       hasFlags,
		encoding:       enc,
		postscriptName: postscriptName,
		isZapfDingbats: ok && sf == ZapfDingBats,
	}
	subtables := baseFont.cmapSubtables()
	for code := range 256 {
		k.codeToGlyph[code] = k.lookupGlyph(uint8(code), subtables)
	}
	return k, true
}

// isNonSymbolic is EmbeddedKind::is_non_symbolic in hayro-font
// src/true_type.rs.
func (k *trueTypeEmbedded) isNonSymbolic() bool {
	return k.hasFlags && k.fontFlags.contains(fontFlagNonSymbolic)
}

// codeToName is EmbeddedKind::code_to_name in hayro-font src/true_type.rs.
func (k *trueTypeEmbedded) codeToName(code uint8) (string, bool) {
	if s, ok := k.differences[code]; ok {
		return s, true
	}
	if s, ok := k.encoding.mapCode(code); ok {
		return s, true
	}
	// Fall back to StandardEncoding when there is no base encoding
	// (PDFJS-6410).
	return standardGet(code)
}

// mapCode is EmbeddedKind::map_code in hayro-font src/true_type.rs.
func (k *trueTypeEmbedded) mapCode(code uint8) GlyphID {
	return k.codeToGlyph[code]
}

// lookupGlyph is EmbeddedKind::lookup_glyph in hayro-font src/true_type.rs.
func (k *trueTypeEmbedded) lookupGlyph(code uint8, subtables []cmapSubtable) GlyphID {
	// Some PDFs mistakenly embed an OpenType font with CFF outlines.
	if k.baseFont.hasCffOutlines() {
		if name, ok := k.codeToName(code); ok {
			if g, ok := k.baseFont.glyphByName(name); ok {
				return g
			}
		}
		return GlyphIDNotdef
	}

	var glyph GlyphID
	found := false
	// try sets glyph to the result of f if no glyph is found yet and the
	// result is not .notdef. This is or_else followed by a filter.
	try := func(f func() (GlyphID, bool)) {
		if found {
			return
		}
		if g, ok := f(); ok && g != GlyphIDNotdef {
			glyph, found = g, true
		}
	}

	if k.isNonSymbolic() {
		lookup, ok := k.codeToName(code)
		if !ok {
			return GlyphIDNotdef
		}
		for _, sub := range subtables {
			if sub.platform == cmapPlatformWindows && sub.encodingID == 1 {
				try(func() (GlyphID, bool) {
					var s string
					if n, ok := glyphNamesGet(lookup); ok {
						s = n
					} else if r, ok := unicodeFromName(lookup); ok {
						s = string(r)
					} else {
						return 0, false
					}
					c, _ := utf8.DecodeRuneInString(s)
					if s == "" {
						return 0, false
					}
					return sub.mapCode(uint32(c))
				})
			}
		}
		for _, sub := range subtables {
			if sub.platform == cmapPlatformMacintosh && sub.encodingID == 0 {
				try(func() (GlyphID, bool) {
					c, ok := macOSRomanGetInverse(lookup)
					if !ok {
						c, ok = macRomanGetInverse(lookup)
					}
					if !ok {
						return 0, false
					}
					return sub.mapCode(uint32(c))
				})
			}
		}
		if !found {
			if gid, ok := k.baseFont.glyphByName(lookup); ok {
				glyph, found = gid, true
			} else if gid, ok := glyphNumString(lookup); ok {
				glyph, found = NewGlyphID(gid), true
			}
		}
	} else {
		for _, sub := range subtables {
			if sub.platform == cmapPlatformWindows && (sub.encodingID == 0 || sub.encodingID == 1) {
				for _, offset := range [...]uint32{0x0000, 0xF000, 0xF100, 0xF200} {
					try(func() (GlyphID, bool) { return sub.mapCode(uint32(code) + offset) })
				}
			} else if (sub.platform == cmapPlatformMacintosh || sub.platform == cmapPlatformUnicode) && sub.encodingID == 0 {
				try(func() (GlyphID, bool) { return sub.mapCode(uint32(code)) })
			}
		}
	}
	if !found {
		return GlyphIDNotdef
	}
	return glyph
}

// glyphWidth is EmbeddedKind::glyph_width in hayro-font src/true_type.rs.
func (k *trueTypeEmbedded) glyphWidth(code uint8) float32 {
	if int(code) < len(k.widths) {
		w := k.widths[code]
		if w.missing {
			return k.missingWidth
		}
		return w.value
	}
	w, _ := k.baseFont.advanceWidth(k.mapCode(code))
	return w
}

// width is Width in hayro-font src/true_type.rs: a value, or a missing
// width.
//
// Port: upstream is an enum. Go uses a struct with the flag missing.
type width struct {
	value   float32
	missing bool
}

// maxSimpleWidths bounds the widths that readWidths keeps. A simple font
// reads the width of a code of one byte, so it never reads past this index.
const maxSimpleWidths = 257

// readWidths is read_widths in hayro-font src/true_type.rs. It reads the
// widths of a simple font and the missing width. It returns false if
// LastChar is less than FirstChar, as upstream does.
//
// Port: upstream fills one entry for each code before FirstChar, and one for
// each width. Go keeps at most maxSimpleWidths entries, so that a large
// FirstChar cannot use much memory. A code of one byte reads the same width.
func readWidths(dict, descriptor syntax.Dict) ([]width, float32, bool) {
	var widths []width
	firstChar, ok1 := dict.Get[int](syntax.KeyFirstChar)
	lastChar, ok2 := dict.Get[int](syntax.KeyLastChar)
	widthsArr, ok3 := dict.Get[syntax.Array](syntax.KeyWidths)
	missingWidth, ok := descriptor.Get[float32](syntax.KeyMissingWidth)
	if !ok {
		missingWidth = 0
	}
	if ok1 && ok2 && ok3 {
		if lastChar < firstChar {
			return nil, 0, false
		}
		take := lastChar - firstChar + 1
		for range min(firstChar, maxSimpleWidths) {
			widths = append(widths, width{missing: true})
		}
		it := widthsArr.Iter[float32]()
		for i := 0; i < take; i++ {
			w, ok := it.Next()
			if !ok || len(widths) >= maxSimpleWidths {
				break
			}
			widths = append(widths, width{value: w})
		}
		for len(widths) <= 256 {
			widths = append(widths, width{missing: true})
		}
	}
	return widths, missingWidth, true
}

// readEncoding is read_encoding in hayro-font src/true_type.rs. It reads the
// base encoding and the differences of a simple font.
func readEncoding(dict syntax.Dict) (encoding, map[uint8]string) {
	getEncodingBase := func(dict syntax.Dict, name syntax.Name) encoding {
		n, ok := dict.Get[syntax.Name](name)
		if !ok {
			return encodingBuiltIn
		}
		switch n {
		case syntax.KeyWinAnsiEncoding:
			return encodingWinAnsi
		case syntax.KeyMacRomanEncoding:
			return encodingMacRoman
		case syntax.KeyMacExpertEncoding:
			return encodingMacExpert
		case syntax.KeyStandardEncoding:
			return encodingStandard
		}
		return encodingBuiltIn
	}

	m := make(map[uint8]string)
	encodingDict, ok := dict.Get[syntax.Dict](syntax.KeyEncoding)
	if !ok {
		return getEncodingBase(dict, syntax.KeyEncoding), m
	}
	if differences, ok := encodingDict.Get[syntax.Array](syntax.KeyDifferences); ok {
		var code int64
		for obj := range differences.Iter[syntax.Object]().All() {
			switch o := obj.(type) {
			case syntax.Number:
				code = o.AsI64()
			case syntax.Name:
				if code >= 0 && code <= 255 {
					m[uint8(code)] = o.AsStr()
				}
				code++
			}
		}
	}
	return getEncodingBase(encodingDict, syntax.KeyBaseEncoding), m
}

// glyphNumString is glyph_num_string in hayro-font src/true_type.rs. It
// reads a glyph name of the form g123.
func glyphNumString(s string) (uint32, bool) {
	if !strings.HasPrefix(s, "g") || len(s) < 2 {
		return 0, false
	}
	rest := s[1:]
	// The parse of Rust accepts one leading plus sign. ParseUint does not
	// accept it, so the plus sign is removed first.
	rest = strings.TrimPrefix(rest, "+")
	v, err := strconv.ParseUint(rest, 10, 32)
	if err != nil {
		return 0, false
	}
	return uint32(v), true
}
