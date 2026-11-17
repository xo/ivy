// Copyright (c) The Hayro Authors.
//
// Use of this source code is governed by the Apache-2.0 or MIT license that
// can be found in the LICENSE-APACHE and LICENSE-MIT files.

// This file ports hayro-font src/standard_font.rs.

package pdffont

import (
	"embed"
	"strings"
	"unicode/utf8"

	"github.com/xo/ivy/internal/hayro/cmap"
	"github.com/xo/ivy/internal/hayro/syntax"
)

// StandardFont is StandardFont in hayro-font src/standard_font.rs. It is
// one of the 14 standard fonts of PDF.
type StandardFont uint8

// The 14 standard fonts of PDF.
const (
	// Helvetica is Helvetica.
	Helvetica StandardFont = iota
	// HelveticaBold is Helvetica Bold.
	HelveticaBold
	// HelveticaOblique is Helvetica Oblique.
	HelveticaOblique
	// HelveticaBoldOblique is Helvetica Bold Oblique.
	HelveticaBoldOblique
	// Courier is Courier.
	Courier
	// CourierBold is Courier Bold.
	CourierBold
	// CourierOblique is Courier Oblique.
	CourierOblique
	// CourierBoldOblique is Courier Bold Oblique.
	CourierBoldOblique
	// TimesRoman is Times Roman.
	TimesRoman
	// TimesBold is Times Bold.
	TimesBold
	// TimesItalic is Times Italic.
	TimesItalic
	// TimesBoldItalic is Times Bold Italic.
	TimesBoldItalic
	// ZapfDingBats is Zapf Dingbats.
	ZapfDingBats
	// Symbol is Symbol.
	Symbol
)

// PostscriptName is StandardFont::postscript_name in hayro-font
// src/standard_font.rs. It returns the PostScript name of the font.
func (s StandardFont) PostscriptName() string {
	switch s {
	case Helvetica:
		return "Helvetica"
	case HelveticaBold:
		return "Helvetica-Bold"
	case HelveticaOblique:
		return "Helvetica-Oblique"
	case HelveticaBoldOblique:
		return "Helvetica-BoldOblique"
	case Courier:
		return "Courier"
	case CourierBold:
		return "Courier-Bold"
	case CourierOblique:
		return "Courier-Oblique"
	case CourierBoldOblique:
		return "Courier-BoldOblique"
	case TimesRoman:
		return "Times-Roman"
	case TimesBold:
		return "Times-Bold"
	case TimesItalic:
		return "Times-Italic"
	case TimesBoldItalic:
		return "Times-BoldItalic"
	case ZapfDingBats:
		return "ZapfDingbats"
	}
	return "Symbol"
}

// assets holds the substitutes of the 14 standard fonts, from PDFium, under
// the license in assets/LICENSE_FOXIT. The names of the files end in .pfb,
// but each file holds a bare CFF font.
//
//go:embed assets/*.pfb
var assets embed.FS

// GetFontData is StandardFont::get_font_data in hayro-font
// src/standard_font.rs. It returns the embedded substitute font data and its
// collection index for the standard font.
//
// Port: upstream embeds the fonts with the feature embed-fonts. Go always
// embeds them.
func (s StandardFont) GetFontData() (FontData, uint32) {
	var name string
	switch s {
	case Helvetica:
		name = "FoxitSans.pfb"
	case HelveticaBold:
		name = "FoxitSansBold.pfb"
	case HelveticaOblique:
		name = "FoxitSansItalic.pfb"
	case HelveticaBoldOblique:
		name = "FoxitSansBoldItalic.pfb"
	case Courier:
		name = "FoxitFixed.pfb"
	case CourierBold:
		name = "FoxitFixedBold.pfb"
	case CourierOblique:
		name = "FoxitFixedItalic.pfb"
	case CourierBoldOblique:
		name = "FoxitFixedBoldItalic.pfb"
	case TimesRoman:
		name = "FoxitSerif.pfb"
	case TimesBold:
		name = "FoxitSerifBold.pfb"
	case TimesItalic:
		name = "FoxitSerifItalic.pfb"
	case TimesBoldItalic:
		name = "FoxitSerifBoldItalic.pfb"
	case ZapfDingBats:
		name = "FoxitDingbats.pfb"
	default:
		name = "FoxitSymbol.pfb"
	}
	// The files are part of the package, so a read cannot fail.
	data, _ := assets.ReadFile("assets/" + name)
	return data, 0
}

// codeToName is StandardFont::code_to_name in hayro-font
// src/standard_font.rs.
func (s StandardFont) codeToName(code uint8) (string, bool) {
	switch s {
	case Symbol:
		return symbolGet(code)
	case ZapfDingBats:
		return zapfDingsGet(code)
	}
	return standardGet(code)
}

// getWidth is StandardFont::get_width in hayro-font src/standard_font.rs.
func (s StandardFont) getWidth(name string) (float32, bool) {
	// <https://github.com/apache/pdfbox/blob/129aafe26548c1ff935af9c55cb40a996186c35f/pdfbox/src/main/java/org/apache/pdfbox/pdmodel/font/PDSimpleFont.java#L340>
	if name == ".notdef" {
		return 250, true
	}
	name = normalizedGlyphName(name)
	var m map[string]float32
	switch s {
	case Helvetica:
		m = metricsHelvetica
	case HelveticaBold:
		m = metricsHelveticaBold
	case HelveticaOblique:
		m = metricsHelveticaOblique
	case HelveticaBoldOblique:
		m = metricsHelveticaBoldOblique
	case Courier:
		m = metricsCourier
	case CourierBold:
		m = metricsCourierBold
	case CourierOblique:
		m = metricsCourierOblique
	case CourierBoldOblique:
		m = metricsCourierBoldOblique
	case TimesRoman:
		m = metricsTimesRoman
	case TimesBold:
		m = metricsTimesBold
	case TimesItalic:
		m = metricsTimesItalic
	case TimesBoldItalic:
		m = metricsTimesBoldItalic
	case ZapfDingBats:
		m = metricsZapfDingBats
	default:
		m = metricsSymbol
	}
	w, ok := m[name]
	return w, ok
}

// isBold is StandardFont::is_bold in hayro-font src/standard_font.rs.
func (s StandardFont) isBold() bool {
	switch s {
	case HelveticaBold, HelveticaBoldOblique, CourierBold, CourierBoldOblique, TimesBold, TimesBoldItalic:
		return true
	}
	return false
}

// isItalic is StandardFont::is_italic in hayro-font src/standard_font.rs.
func (s StandardFont) isItalic() bool {
	switch s {
	case HelveticaOblique, HelveticaBoldOblique, CourierOblique, CourierBoldOblique, TimesItalic, TimesBoldItalic:
		return true
	}
	return false
}

// isSerif is StandardFont::is_serif in hayro-font src/standard_font.rs.
func (s StandardFont) isSerif() bool {
	switch s {
	case TimesRoman, TimesBold, TimesItalic, TimesBoldItalic:
		return true
	}
	return false
}

// isMonospace is StandardFont::is_monospace in hayro-font
// src/standard_font.rs.
func (s StandardFont) isMonospace() bool {
	switch s {
	case Courier, CourierBold, CourierOblique, CourierBoldOblique:
		return true
	}
	return false
}

// standardKind is StandardKind in hayro-font src/standard_font.rs.
type standardKind struct {
	baseFont     StandardFont
	program      *fontProgram
	encoding     encoding
	widths       []width
	missingWidth float32
	fallback     bool
	codeToGlyph  [256]GlyphID
	encodings    map[uint8]string
	// symbolic is the (3,0) cmap subtable of a symbolic TrueType font that
	// the file names and does not embed, or nil.
	//
	// Port: upstream draws such a font with a standard font.
	symbolic *cmapSubtable
}

// newStandardKind is StandardKind::new in hayro-font src/standard_font.rs.
func newStandardKind(dict syntax.Dict, loader *programLoader) (*standardKind, bool) {
	descriptor, _ := dict.Get[syntax.Dict](syntax.KeyFontDescriptor)
	font, exact, ok := selectStandardFont(dict, descriptor)
	if !ok {
		return nil, false
	}
	return newStandardKindWith(dict, font, !exact, loader)
}

// newStandardKindWith is StandardKind::new_with_standard in hayro-font
// src/standard_font.rs.
func newStandardKindWith(dict syntax.Dict, baseFont StandardFont, fallback bool, loader *programLoader) (*standardKind, bool) {
	descriptor, _ := dict.Get[syntax.Dict](syntax.KeyFontDescriptor)
	widths, missingWidth, ok := readWidths(dict, descriptor)
	if !ok {
		return nil, false
	}
	enc, encodingMap := readEncoding(dict)
	// See PDFJS-16464: Ignore encodings for non-embedded Type1 symbol fonts.
	if baseFont == Symbol || baseFont == ZapfDingBats {
		enc = encodingBuiltIn
	}
	// Port: upstream draws the font with the substitute of baseFont. Go
	// first asks for the font that dict names, and fits its glyphs to the
	// widths of dict, as it fits those of a substitute. Symbol and
	// ZapfDingbats keep their substitutes, which hold their glyphs.
	program, named := (*fontProgram)(nil), false
	if baseFont != Symbol && baseFont != ZapfDingBats {
		program, named = loader.resolveNamed(dict)
	}
	if !named {
		var ok bool
		if program, ok = loader.resolve(&FontQuery{IsStandard: true, Standard: baseFont}); !ok {
			return nil, false
		}
	}
	fallback = fallback || named
	k := &standardKind{
		baseFont:     baseFont,
		program:      program,
		widths:       widths,
		missingWidth: missingWidth,
		encodings:    encodingMap,
		fallback:     fallback,
		encoding:     enc,
	}
	if named {
		k.symbolic = symbolicSubtable(dict, descriptor, program)
	}
	for code := range 256 {
		k.codeToGlyph[code] = k.lookupGlyph(uint8(code))
	}
	return k, true
}

// symbolicSubtable returns the (3,0) cmap subtable of program, for a
// symbolic TrueType font of dict, or nil. A symbolic TrueType font maps a
// code through that subtable (ISO 32000-1, 9.6.6.4), not through the names
// of the glyphs.
//
// Port: upstream has no such function.
func symbolicSubtable(dict, descriptor syntax.Dict, program *fontProgram) *cmapSubtable {
	if subtype, _ := dict.Get[syntax.Name](syntax.KeySubtype); subtype != syntax.KeyTrueType {
		return nil
	}
	bits, ok := descriptor.Get[uint32](syntax.KeyFlags)
	if !ok {
		return nil
	}
	flags, _ := fontFlagsFromBits(bits)
	if !flags.contains(fontFlagSymbolic) || flags.contains(fontFlagNonSymbolic) {
		return nil
	}
	for _, sub := range program.cmapSubtables() {
		if sub.platform == cmapPlatformWindows && sub.encodingID == 0 {
			return &sub
		}
	}
	return nil
}

// codeToPSName is StandardKind::code_to_ps_name in hayro-font
// src/standard_font.rs.
func (k *standardKind) codeToPSName(code uint8) (string, bool) {
	if s, ok := k.encodings[code]; ok {
		return s, true
	}
	if k.encoding == encodingBuiltIn {
		return k.baseFont.codeToName(code)
	}
	return k.encoding.mapCode(code)
}

// mapCode is StandardKind::map_code in hayro-font src/standard_font.rs.
func (k *standardKind) mapCode(code uint8) GlyphID {
	return k.codeToGlyph[code]
}

// lookupGlyph is StandardKind::lookup_glyph in hayro-font
// src/standard_font.rs.
//
// Port: a font with a symbolic subtable maps the code through that subtable
// first, as trueTypeEmbedded.lookupGlyph does for an embedded font.
func (k *standardKind) lookupGlyph(code uint8) GlyphID {
	if k.symbolic != nil {
		for _, offset := range [...]uint32{0x0000, 0xF000, 0xF100, 0xF200} {
			if g, ok := k.symbolic.mapCode(uint32(code) + offset); ok && g != GlyphIDNotdef {
				return g
			}
		}
	}
	c, ok := k.codeToPSName(code)
	if !ok {
		return GlyphIDNotdef
	}
	if g, ok := k.program.glyphByName(c); ok {
		return g
	}
	if s, ok := glyphNamesGet(c); ok && s != "" {
		r, _ := utf8.DecodeRuneInString(s)
		if g, ok := k.program.cmapGlyph(r); ok {
			return g
		}
	}
	if g, ok := k.program.glyphByName(normalizedGlyphName(c)); ok {
		return g
	}
	return GlyphIDNotdef
}

// glyphStretch is StandardKind::glyph_stretch in hayro-font
// src/standard_font.rs.
func (k *standardKind) glyphStretch(glyph GlyphID, code uint8) float64 {
	actualWidth, ok := k.program.advanceWidth(glyph)
	if !ok {
		name, nok := k.codeToPSName(code)
		if !nok {
			return 1
		}
		if actualWidth, ok = k.baseFont.getWidth(name); !ok {
			return 1
		}
	}
	// Viewers ignore `Widths` for recognised standard fonts, so only
	// substitutes use it.
	var expectedWidth float32
	if k.fallback {
		if int(code) >= len(k.widths) || k.widths[code].missing {
			return 1
		}
		expectedWidth = k.widths[code].value
	} else {
		name, ok := k.codeToPSName(code)
		if !ok {
			return 1
		}
		if expectedWidth, ok = k.baseFont.getWidth(name); !ok {
			return 1
		}
	}
	return stretchFactor(expectedWidth, actualWidth)
}

// glyphWidth is StandardKind::glyph_width in hayro-font
// src/standard_font.rs.
func (k *standardKind) glyphWidth(code uint8) (float32, bool) {
	if int(code) < len(k.widths) {
		if k.widths[code].missing {
			return k.missingWidth, true
		}
		return k.widths[code].value, true
	}
	c, ok := k.codeToPSName(code)
	if !ok {
		return 0, false
	}
	return k.baseFont.getWidth(c)
}

// charCodeToUnicode is StandardKind::char_code_to_unicode in hayro-font
// src/standard_font.rs.
func (k *standardKind) charCodeToUnicode(code uint8) (cmap.BfString, bool) {
	name, ok := k.codeToPSName(code)
	if !ok {
		return cmap.BfString{}, false
	}
	return glyphNameToUnicode(name, k.baseFont == ZapfDingBats)
}

// isItalic is StandardKind::is_italic in hayro-font src/standard_font.rs.
func (k *standardKind) isItalic() bool { return k.baseFont.isItalic() }

// isBold is StandardKind::is_bold in hayro-font src/standard_font.rs.
func (k *standardKind) isBold() bool { return k.baseFont.isBold() }

// isSerif is StandardKind::is_serif in hayro-font src/standard_font.rs.
func (k *standardKind) isSerif() bool { return k.baseFont.isSerif() }

// isMonospace is StandardKind::is_monospace in hayro-font
// src/standard_font.rs.
func (k *standardKind) isMonospace() bool { return k.baseFont.isMonospace() }

// selectStandardFont is select_standard_font in hayro-font
// src/standard_font.rs. It returns the standard font that a font names, and
// whether the name is an exact match.
func selectStandardFont(dict, descriptor syntax.Dict) (StandardFont, bool, bool) {
	baseFont, ok := dict.Get[syntax.Name](syntax.KeyBaseFont)
	if !ok {
		return 0, false, false
	}
	name := stripSubsetPrefix(baseFont.AsStr())
	switch name {
	case "Helvetica":
		return Helvetica, true, true
	case "Helvetica-Bold":
		return HelveticaBold, true, true
	case "Helvetica-Oblique":
		return HelveticaOblique, true, true
	case "Helvetica-BoldOblique":
		return HelveticaBoldOblique, true, true
	case "Courier":
		return Courier, true, true
	case "Courier-Bold":
		return CourierBold, true, true
	case "Courier-Oblique":
		return CourierOblique, true, true
	case "Courier-BoldOblique":
		return CourierBoldOblique, true, true
	case "Times-Roman":
		return TimesRoman, true, true
	case "Times-Bold":
		return TimesBold, true, true
	case "Times-Italic":
		return TimesItalic, true, true
	case "Times-BoldItalic":
		return TimesBoldItalic, true, true
	case "Symbol":
		return Symbol, true, true
	case "ZapfDingbats":
		return ZapfDingBats, true, true
	}

	lower := asciiLower(name)
	w, wok := descriptor.Get[uint32](syntax.KeyFontWeight)
	isBold := wok && w >= 700 || strings.Contains(lower, "bold") || strings.Contains(lower, "demi")
	a, aok := descriptor.Get[float32](syntax.KeyItalicAngle)
	isItalic := aok && a != 0 || strings.Contains(lower, "italic") || strings.Contains(lower, "oblique")

	family := -1
	exact := false
	switch {
	case strings.Contains(lower, "helvetica"):
		family, exact = familyHelvetica, true
	case strings.Contains(lower, "arial") || strings.Contains(lower, "sans"):
		family = familyHelvetica
	case strings.Contains(lower, "courier"):
		family, exact = familyCourier, true
	case strings.Contains(lower, "mono"):
		family = familyCourier
	case strings.Contains(lower, "times"):
		family, exact = familyTimes, true
	case strings.Contains(lower, "serif"):
		family = familyTimes
	case strings.Contains(lower, "zapfdingbats") || strings.Contains(lower, "dingbats"):
		return ZapfDingBats, false, true
	}
	if family < 0 {
		return 0, false, false
	}
	// The fonts of each family are in the order regular, bold, oblique,
	// bold oblique.
	font := StandardFont(family * 4)
	switch {
	case isBold && isItalic:
		font += 3
	case isBold:
		font++
	case isItalic:
		font += 2
	}
	return font, exact, true
}

// The families of StandardFontFamily in hayro-font src/standard_font.rs, as
// the index of the first font of the family divided by 4.
const (
	familyHelvetica = 0
	familyCourier   = 1
	familyTimes     = 2
)

// asciiLower is str::to_ascii_lowercase of Rust. It changes only the ASCII
// letters.
func asciiLower(s string) string {
	b := []byte(s)
	for i, c := range b {
		if c >= 'A' && c <= 'Z' {
			b[i] = c + 'a' - 'A'
		}
	}
	return string(b)
}
