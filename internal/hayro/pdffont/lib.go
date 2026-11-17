// Copyright (c) The Hayro Authors.
//
// Use of this source code is governed by the Apache-2.0 or MIT license that
// can be found in the LICENSE-APACHE and LICENSE-MIT files.

// This file ports hayro-font src/lib.rs.

// Package pdffont is a port of the Rust crate hayro-font, which reads the
// fonts of PDF files.
//
// The package turns a font dictionary of PDF, with its embedded font
// program, into a Font. A Font maps the character codes of text to glyphs,
// and it gives the outline of each glyph. A font program is the data of a
// font file: TrueType, OpenType, CFF or Type 1. A Type 3 font has no font
// program. Its glyphs are content streams of the PDF file.
//
// The package reads TrueType, OpenType and CFF with go-text, and Type 1 with
// its own reader. It embeds substitutes for the 14 standard fonts of PDF.
package pdffont

import (
	"strconv"
	"strings"
	"sync"
	"unicode/utf8"

	"github.com/xo/okage/kurbo"

	"github.com/xo/ivy/internal/hayro/cmap"
	"github.com/xo/ivy/internal/hayro/kurboext"
	"github.com/xo/ivy/internal/hayro/syntax"
)

// FontResolverFn is FontResolverFn in hayro-font src/lib.rs. It resolves a
// query for a font to the data of a font file and the index of the font in
// a collection. It returns false if it has no font for the query.
type FontResolverFn func(query *FontQuery) (FontData, uint32, bool)

// CMapResolverFn is CMapResolverFn in hayro-font src/lib.rs. It returns the
// data of a CMap file by its name, and false if it does not know the name.
// cmap.LoadEmbedded is such a function.
type CMapResolverFn func(name cmap.Name) ([]byte, bool)

// FontData is FontData in hayro-font src/lib.rs. It holds the bytes of a
// font file.
//
// Port: upstream shares the bytes through an Arc. Go uses a slice. A caller
// must not change it.
type FontData = []byte

// UnitsPerEm is UNITS_PER_EM in hayro-font src/lib.rs. All outlines and
// metrics are scaled to it.
const UnitsPerEm float32 = 1000

// GlyphID is GlyphId in hayro-font src/lib.rs. It is the ID of a glyph in a
// font.
type GlyphID uint32

// GlyphIDNotdef is GlyphId::NOTDEF in hayro-font src/lib.rs. It is the ID of
// the glyph .notdef.
const GlyphIDNotdef GlyphID = 0

// NewGlyphID is GlyphId::new in hayro-font src/lib.rs.
func NewGlyphID(id uint32) GlyphID {
	return GlyphID(id)
}

// ToU32 is GlyphId::to_u32 in hayro-font src/lib.rs.
func (g GlyphID) ToU32() uint32 {
	return uint32(g)
}

// Font is Font in hayro-font src/lib.rs. It is a PDF font.
//
// Port: upstream wraps the enum FontType. Go sets one of the four fields.
type Font struct {
	type1    *type1Font
	trueType *trueTypeFont
	type0    *type0Font
	type3    *Type3Font
}

// MapCode is Font::map_code in hayro-font src/lib.rs. It maps a character
// code to a glyph.
//
// Port: upstream asserts in debug builds that the code of a simple font is
// a byte. Go uses the low byte.
func (f *Font) MapCode(code uint32) GlyphID {
	switch {
	case f.type1 != nil:
		return f.type1.mapCode(uint8(code))
	case f.trueType != nil:
		return f.trueType.mapCode(uint8(code))
	case f.type0 != nil:
		return f.type0.mapCode(code)
	}
	return f.type3.mapCode(uint8(code))
}

// Glyph is Font::glyph in hayro-font src/lib.rs. It returns the glyph that a
// glyph ID refers to: an *OutlineGlyph, or a *Type3Glyph for a Type 3 font.
func (f *Font) Glyph(glyph GlyphID, charCode uint32) FontGlyph {
	var font outlineFont
	switch {
	case f.type1 != nil:
		font = outlineFont{type1: f.type1}
	case f.trueType != nil:
		font = outlineFont{trueType: f.trueType}
	case f.type0 != nil:
		font = outlineFont{type0: f.type0}
	default:
		return newType3Glyph(f.type3, glyph, charCode)
	}
	return newOutlineGlyph(font, glyph, charCode)
}

// GlyphStretch is Font::glyph_stretch in hayro-font src/lib.rs. It returns
// the horizontal scale factor that fits a glyph to its expected width.
func (f *Font) GlyphStretch(glyph GlyphID, code uint32) float64 {
	switch {
	case f.type1 != nil:
		return f.type1.glyphStretch(glyph, uint8(code))
	case f.trueType != nil:
		return f.trueType.glyphStretch(glyph, uint8(code))
	case f.type0 != nil:
		return f.type0.glyphStretch(glyph, code)
	}
	return 1
}

// CodeAdvance is Font::code_advance in hayro-font src/lib.rs. It returns the
// advance of a character code.
func (f *Font) CodeAdvance(code uint32) kurbo.Vec2 {
	switch {
	case f.type1 != nil:
		w, _ := f.type1.glyphWidth(uint8(code))
		return kurbo.NewVec2(float64(w), 0)
	case f.trueType != nil:
		return kurbo.NewVec2(float64(f.trueType.glyphWidth(uint8(code))), 0)
	case f.type0 != nil:
		return f.type0.codeAdvance(code)
	}
	return kurbo.NewVec2(float64(f.type3.glyphWidth(code)), 0)
}

// OriginDisplacement is Font::origin_displacement in hayro-font
// src/lib.rs. It returns the displacement of the glyph origin of a
// character code.
func (f *Font) OriginDisplacement(code uint32) kurbo.Vec2 {
	if f.type0 != nil {
		return f.type0.originDisplacement(code)
	}
	return kurbo.Vec2{}
}

// ReadCode is Font::read_code in hayro-font src/lib.rs. It reads the next
// character code from a string, and returns the code and its length.
//
// Port: upstream panics if offset is not inside bytes. Go returns the code 0
// with the length 1.
func (f *Font) ReadCode(bytes []byte, offset int) (uint32, int) {
	if offset < 0 || offset >= len(bytes) {
		return 0, 1
	}
	if f.type0 != nil {
		return f.type0.readCode(bytes, offset)
	}
	return uint32(bytes[offset]), 1
}

// IsHorizontal is Font::is_horizontal in hayro-font src/lib.rs. It reports
// whether the font is written horizontally.
func (f *Font) IsHorizontal() bool {
	if f.type0 != nil {
		return f.type0.isHorizontal()
	}
	return true
}

// newFont is Font::new in hayro-font src/lib.rs.
func newFont(dict syntax.Dict, loader *fontLoader) (*Font, bool) {
	subtype, ok := dict.Get[syntax.Name](syntax.KeySubtype)
	if !ok {
		return nil, false
	}
	switch subtype {
	case syntax.KeyType1, syntax.KeyMMType1:
		t, ok := newType1Font(dict, loader)
		if !ok {
			return nil, false
		}
		return &Font{type1: t}, true
	// PDFBOX-5463: PDF viewers seem to accept OpenType as well.
	case syntax.KeyTrueType, syntax.KeyOpenType:
		t, ok := newTrueTypeFont(dict, loader)
		if !ok {
			return nil, false
		}
		return &Font{trueType: t}, true
	case syntax.KeyType0:
		t, ok := newType0Font(dict, loader)
		if !ok {
			return nil, false
		}
		return &Font{type0: t}, true
	case syntax.KeyType3:
		t, ok := newType3Font(dict, loader.cmapResolver)
		if !ok {
			return nil, false
		}
		return &Font{type3: t}, true
	}
	return nil, false
}

// newStandardFont is Font::new_standard in hayro-font src/lib.rs.
func newStandardFont(standardFont StandardFont, loader *programLoader) (*Font, bool) {
	font, ok := newStandardType1Font(standardFont, loader)
	if !ok {
		return nil, false
	}
	return &Font{type1: font}, true
}

// FontCache is FontCache in hayro-font src/lib.rs. It is a cache for the
// fonts of a document and their font programs.
//
// Port: upstream shares the cache through an Rc and a RefCell, so one thread
// uses it. Go guards the maps with mutexes, so that the methods are safe for
// concurrent use. A font is built without a lock, so two goroutines can
// build the same font, and the first one to finish wins.
type FontCache struct {
	fontsMu sync.Mutex
	fonts   map[Key128]*Font

	programsMu sync.Mutex
	programs   map[Key128]*fontProgram
}

// NewFontCache is FontCache::new in hayro-font src/lib.rs. It returns an
// empty cache.
func NewFontCache() *FontCache {
	return &FontCache{
		fonts:    make(map[Key128]*Font),
		programs: make(map[Key128]*fontProgram),
	}
}

// Font is FontCache::font in hayro-font src/lib.rs. It returns the font that
// a font dictionary describes, and false if it cannot read the font.
// fontResolver and cmapResolver can be nil.
func (c *FontCache) Font(dict syntax.Dict, fontResolver FontResolverFn, cmapResolver CMapResolverFn) (*Font, bool) {
	key := dictCacheKey(dict)
	c.fontsMu.Lock()
	font, ok := c.fonts[key]
	c.fontsMu.Unlock()
	if ok {
		return font, font != nil
	}
	loader := &fontLoader{programs: c.programLoader(fontResolver), cmapResolver: cmapResolver}
	font, _ = newFont(dict, loader)
	c.fontsMu.Lock()
	if prev, ok := c.fonts[key]; ok {
		font = prev
	} else {
		c.fonts[key] = font
	}
	c.fontsMu.Unlock()
	return font, font != nil
}

// StandardFont is FontCache::standard_font in hayro-font src/lib.rs. It
// returns one of the 14 standard fonts.
func (c *FontCache) StandardFont(font StandardFont, fontResolver FontResolverFn) (*Font, bool) {
	return newStandardFont(font, c.programLoader(fontResolver))
}

// programLoader is FontCache::program_loader in hayro-font src/lib.rs.
func (c *FontCache) programLoader(fontResolver FontResolverFn) *programLoader {
	return &programLoader{fontResolver: fontResolver, cache: c}
}

// HintingCache is HintingCache in hayro-font src/lib.rs. It is a cache for
// hinting font programs.
//
// Port: go-text has no hinter, so the cache holds nothing. It keeps the
// signature of OutlineGlyph.HintedOutline.
type HintingCache struct{}

// NewHintingCache is HintingCache::new in hayro-font src/lib.rs. It returns
// an empty cache.
func NewHintingCache() *HintingCache {
	return &HintingCache{}
}

// FontGlyph is FontGlyph in hayro-font src/lib.rs. It is a glyph of a Font:
// an *OutlineGlyph, defined by an outline, or a *Type3Glyph, defined by a
// content stream of the font.
//
// Port: upstream is an enum. Go uses an interface that only the two types
// implement. A caller tells them apart with a type switch.
type FontGlyph interface {
	isFontGlyph()
}

func (*OutlineGlyph) isFontGlyph() {}
func (*Type3Glyph) isFontGlyph()   {}

// OutlineGlyph is OutlineGlyph in hayro-font src/lib.rs. It is a glyph
// defined by an outline.
type OutlineGlyph struct {
	id       GlyphID
	program  *fontProgram
	hidden   bool
	font     outlineFont
	charCode uint32
}

// Outline is OutlineGlyph::outline in hayro-font src/lib.rs. It returns the
// outline of the glyph, for an upem value of 1000.
func (o *OutlineGlyph) Outline() kurboext.BezPath {
	if o.hidden {
		return kurboext.NewBezPath()
	}
	return o.program.outline(o.id)
}

// HintedOutline is OutlineGlyph::hinted_outline in hayro-font src/lib.rs.
// It returns the outline of the glyph hinted at ppem, and false if its font
// cannot be hinted.
func (o *OutlineGlyph) HintedOutline(ppem float32, cache *HintingCache) (kurboext.BezPath, bool) {
	if o.hidden {
		return kurboext.NewBezPath(), true
	}
	return o.program.hintedOutline(o.id, ppem, cache)
}

// Identifier is OutlineGlyph::identifier in hayro-font src/lib.rs. It
// returns the identifier of the glyph.
func (o *OutlineGlyph) Identifier() GlyphIdentifier {
	return GlyphIdentifier{program: o.program, glyph: o.id, hasGlyph: !o.hidden}
}

// AsUnicode is OutlineGlyph::as_unicode in hayro-font src/lib.rs. It
// returns the Unicode text of the glyph, and false if it is not known.
func (o *OutlineGlyph) AsUnicode() (cmap.BfString, bool) {
	return o.font.charCodeToUnicode(o.charCode)
}

// FontData is OutlineGlyph::font_data in hayro-font src/lib.rs. It returns
// the raw font bytes and metadata, and false if they are not available.
func (o *OutlineGlyph) FontData() (OutlineFontData, bool) {
	return o.font.fontData()
}

// GlyphID is OutlineGlyph::glyph_id in hayro-font src/lib.rs. It returns the
// glyph ID within the font.
func (o *OutlineGlyph) GlyphID() GlyphID {
	return o.id
}

// CharCode is OutlineGlyph::char_code in hayro-font src/lib.rs. It returns
// the character code that selected this glyph.
func (o *OutlineGlyph) CharCode() uint32 {
	return o.charCode
}

// AdvanceWidth is OutlineGlyph::advance_width in hayro-font src/lib.rs. It
// returns the advance width of the glyph.
func (o *OutlineGlyph) AdvanceWidth() (float32, bool) {
	return o.font.glyphAdvanceWidth(o.charCode)
}

// newOutlineGlyph is OutlineGlyph::new in hayro-font src/lib.rs.
func newOutlineGlyph(font outlineFont, id GlyphID, charCode uint32) *OutlineGlyph {
	hidden := id == GlyphIDNotdef && font.hidesNotdef()
	program := font.program()
	if font.type0 != nil {
		// Port: a glyph of a CID font can come from its latin font.
		program, id = font.type0.programOf(id)
	}
	return &OutlineGlyph{
		id:       id,
		program:  program,
		hidden:   hidden,
		font:     font,
		charCode: charCode,
	}
}

// GlyphIdentifier is GlyphIdentifier in hayro-font src/lib.rs. It
// identifies the outline of a glyph and the font program it belongs to.
type GlyphIdentifier struct {
	program  *fontProgram
	glyph    GlyphID
	hasGlyph bool
}

// Equal is the derived PartialEq of GlyphIdentifier in hayro-font
// src/lib.rs. Two font programs are equal if their content is.
func (g GlyphIdentifier) Equal(other GlyphIdentifier) bool {
	return g.program.contentHash() == other.program.contentHash() &&
		g.hasGlyph == other.hasGlyph && g.glyph == other.glyph
}

// GlyphKey is GlyphIdentifier::glyph_key in hayro-font src/lib.rs. It
// returns a key that identifies the outline of the glyph.
func (g GlyphIdentifier) GlyphKey() Key128 {
	return newKeyHasher().key(g.program.contentHash()).boolean(g.hasGlyph).u64(uint64(g.glyph)).sum()
}

// FontKey is GlyphIdentifier::font_key in hayro-font src/lib.rs. It returns
// a key that identifies the font program of the glyph.
func (g GlyphIdentifier) FontKey() Key128 {
	return g.program.contentHash()
}

// FontQuery is FontQuery in hayro-font src/lib.rs. It is a query for a font:
// one of the 14 standard fonts, or a font that the PDF file does not embed.
//
// Port: upstream is an enum. Go holds both cases. IsStandard tells which
// one is set. Named marks a third case that upstream does not have.
type FontQuery struct {
	// IsStandard is true for FontQuery::Standard and false for
	// FontQuery::Fallback.
	IsStandard bool
	// Standard is the font of FontQuery::Standard.
	Standard StandardFont
	// Fallback is the query of FontQuery::Fallback.
	Fallback FallbackFontQuery
	// Named is true for a query of the font that a PDF file names and does
	// not embed, by its name, such as Arial or MS-Mincho. Fallback holds
	// the name and the style. The font of a Named query is a font that a
	// program gives, such as a font of ivy.Config.Fonts or of the system.
	// A resolver returns false when it has no font of the name. The font
	// then is the substitute of a standard font, as upstream does.
	//
	// Port: upstream has no such query.
	Named bool
}

// cacheKey is the derived Hash of FontQuery in hayro-font src/lib.rs.
func (q *FontQuery) cacheKey() Key128 {
	h := newKeyHasher().boolean(q.IsStandard)
	if q.IsStandard {
		return h.u64(uint64(q.Standard)).sum()
	}
	f := &q.Fallback
	h.boolean(q.Named).str(f.PostScriptName).str(f.FontName).str(f.FontFamily).u64(uint64(f.FontStretch)).
		u64(uint64(f.FontWeight)).boolean(f.IsFixedPitch).boolean(f.IsSerif).
		boolean(f.IsItalic).boolean(f.IsBold).boolean(f.IsSmallCap)
	if cc := f.CharacterCollection; cc != nil {
		h.boolean(true).u64(uint64(cc.Family.Kind)).str(cc.Family.Registry).
			str(cc.Family.Ordering).u64(uint64(uint32(cc.Supplement)))
	} else {
		h.boolean(false)
	}
	return h.sum()
}

// FallbackFontQuery is FallbackFontQuery in hayro-font src/lib.rs. It is a
// query for a font with specific properties.
//
// Port: upstream holds the names in an Option. Go uses the empty string for
// None.
type FallbackFontQuery struct {
	// PostScriptName is the postscript name of the font.
	PostScriptName string
	// FontName is the name of the font.
	FontName string
	// FontFamily is the family of the font.
	FontFamily string
	// FontStretch is the stretch of the font.
	FontStretch FontStretch
	// FontWeight is the weight of the font.
	FontWeight uint32
	// IsFixedPitch is whether the font is monospaced.
	IsFixedPitch bool
	// IsSerif is whether the font is serif.
	IsSerif bool
	// IsItalic is whether the font is italic.
	IsItalic bool
	// IsBold is whether the font is bold.
	IsBold bool
	// IsSmallCap is whether the font is small cap.
	IsSmallCap bool
	// CharacterCollection is the character collection of a CID font, or
	// nil.
	CharacterCollection *cmap.CharacterCollection
}

// DefaultFallbackFontQuery is the Default implementation of
// FallbackFontQuery in hayro-font src/lib.rs.
func DefaultFallbackFontQuery() FallbackFontQuery {
	return FallbackFontQuery{FontStretch: FontStretchNormal, FontWeight: 400}
}

// newFallbackFontQuery is FallbackFontQuery::new in hayro-font src/lib.rs.
// It keeps the faults of upstream: it reads FontName from the font
// dictionary, not from the descriptor, and it never sets IsFixedPitch.
func newFallbackFontQuery(dict syntax.Dict) FallbackFontQuery {
	data := DefaultFallbackFontQuery()
	if n, ok := dict.Get[syntax.Name](syntax.KeyBaseFont); ok {
		data.PostScriptName = stripSubsetPrefix(n.AsStr())
	}
	if descriptor, ok := dict.Get[syntax.Dict](syntax.KeyFontDescriptor); ok {
		if n, ok := dict.Get[syntax.Name](syntax.KeyFontName); ok {
			data.FontName = stripSubsetPrefix(n.AsStr())
		}
		if n, ok := descriptor.Get[syntax.Name](syntax.KeyFontFamily); ok {
			data.FontFamily = n.AsStr()
		}
		data.FontStretch = FontStretchNormal
		if n, ok := descriptor.Get[syntax.Name](syntax.KeyFontStretch); ok {
			data.FontStretch = fontStretchFromString(n.AsStr())
		}
		data.FontWeight = 400
		if w, ok := descriptor.Get[uint32](syntax.KeyFontWeight); ok {
			data.FontWeight = w
		}
		if bits, ok := descriptor.Get[uint32](syntax.KeyFlags); ok {
			flags := fontFlagsFromBitsTruncate(bits)
			data.IsSerif = flags.contains(fontFlagSerif)
			data.IsItalic = flags.contains(fontFlagItalic)
			data.IsSmallCap = flags.contains(fontFlagSmallCap)
		}
	}
	data.IsItalic = data.IsItalic || strings.Contains(data.PostScriptName, "Italic")
	data.IsBold = data.IsBold || strings.Contains(data.PostScriptName, "Bold")
	return data
}

// PickStandardFont is FallbackFontQuery::pick_standard_font in hayro-font
// src/lib.rs. It returns the standard font that best matches the query.
func (q *FallbackFontQuery) PickStandardFont() StandardFont {
	switch {
	case q.IsFixedPitch:
		switch {
		case q.IsBold && q.IsItalic:
			return CourierBoldOblique
		case q.IsBold:
			return CourierBold
		case q.IsItalic:
			return CourierOblique
		}
		return Courier
	case !q.IsSerif:
		switch {
		case q.IsBold && q.IsItalic:
			return HelveticaBoldOblique
		case q.IsBold:
			return HelveticaBold
		case q.IsItalic:
			return HelveticaOblique
		}
		return Helvetica
	}
	switch {
	case q.IsBold && q.IsItalic:
		return TimesBoldItalic
	case q.IsBold:
		return TimesBold
	case q.IsItalic:
		return TimesItalic
	}
	return TimesRoman
}

// FontStretch is FontStretch in hayro-font src/lib.rs. It is the stretch of
// a font.
type FontStretch uint8

// The values of FontStretch.
const (
	// FontStretchNormal is normal.
	FontStretchNormal FontStretch = iota
	// FontStretchUltraCondensed is ultra condensed.
	FontStretchUltraCondensed
	// FontStretchExtraCondensed is extra condensed.
	FontStretchExtraCondensed
	// FontStretchCondensed is condensed.
	FontStretchCondensed
	// FontStretchSemiCondensed is semi condensed.
	FontStretchSemiCondensed
	// FontStretchSemiExpanded is semi expanded.
	FontStretchSemiExpanded
	// FontStretchExpanded is expanded.
	FontStretchExpanded
	// FontStretchExtraExpanded is extra expanded.
	FontStretchExtraExpanded
	// FontStretchUltraExpanded is ultra expanded.
	FontStretchUltraExpanded
)

// fontStretchFromString is FontStretch::from_string in hayro-font
// src/lib.rs.
func fontStretchFromString(s string) FontStretch {
	switch s {
	case "UltraCondensed":
		return FontStretchUltraCondensed
	case "ExtraCondensed":
		return FontStretchExtraCondensed
	case "Condensed":
		return FontStretchCondensed
	case "SemiCondensed":
		return FontStretchSemiCondensed
	case "SemiExpanded":
		return FontStretchSemiExpanded
	case "Expanded":
		return FontStretchExpanded
	case "ExtraExpanded":
		return FontStretchExtraExpanded
	case "UltraExpanded":
		return FontStretchUltraExpanded
	}
	return FontStretchNormal
}

// fontLoader is FontLoader in hayro-font src/lib.rs.
type fontLoader struct {
	programs     *programLoader
	cmapResolver CMapResolverFn
}

// programLoader is ProgramLoader in hayro-font src/lib.rs.
type programLoader struct {
	fontResolver FontResolverFn
	cache        *FontCache
}

// load is ProgramLoader::load in hayro-font src/lib.rs. It returns the font
// program of an embedded font file.
func (l *programLoader) load(stream syntax.Stream) (*fontProgram, bool) {
	key := newKeyHasher().str("font program").key(streamCacheKey(stream)).sum()
	return l.getOrInsertWith(key, func() *fontProgram {
		data, err := stream.DecodedData()
		if err != nil {
			return nil
		}
		p, err := newFontProgram(data, 0)
		if err != nil {
			return nil
		}
		return p
	})
}

// resolve is ProgramLoader::resolve in hayro-font src/lib.rs. It returns the
// font program that the font resolver gives for a query.
func (l *programLoader) resolve(query *FontQuery) (*fontProgram, bool) {
	key := newKeyHasher().str("resolved font program").key(query.cacheKey()).sum()
	return l.getOrInsertWith(key, func() *fontProgram {
		if l.fontResolver == nil {
			return nil
		}
		data, index, ok := l.fontResolver(query)
		if !ok {
			return nil
		}
		p, err := newFontProgram(data, index)
		if err != nil {
			return nil
		}
		return p
	})
}

// resolveNamed returns the font program that the font resolver gives for
// the font that dict names, with a Named query, and false if it gives none.
// A font that names one of the 14 standard fonts gets no Named query, so
// that it keeps the substitute of the standard font.
//
// Port: upstream has no such query. It draws a font that the file does not
// embed with the substitute of a standard font.
func (l *programLoader) resolveNamed(dict syntax.Dict) (*fontProgram, bool) {
	q := newFallbackFontQuery(dict)
	if q.PostScriptName == "" || isStandardName(q.PostScriptName) {
		return nil, false
	}
	return l.resolve(&FontQuery{Named: true, Fallback: q})
}

// isStandardName reports whether name is the PostScript name of one of
// the 14 standard fonts.
func isStandardName(name string) bool {
	for f := Helvetica; f <= Symbol; f++ {
		if f.PostscriptName() == name {
			return true
		}
	}
	return false
}

// getOrInsertWith is ProgramLoader::get_or_insert_with in hayro-font
// src/lib.rs. It keeps a nil program for a key that has none.
func (l *programLoader) getOrInsertWith(key Key128, f func() *fontProgram) (*fontProgram, bool) {
	c := l.cache
	c.programsMu.Lock()
	p, ok := c.programs[key]
	c.programsMu.Unlock()
	if ok {
		return p, p != nil
	}
	p = f()
	c.programsMu.Lock()
	if prev, ok := c.programs[key]; ok {
		p = prev
	} else {
		c.programs[key] = p
	}
	c.programsMu.Unlock()
	return p, p != nil
}

// encoding is Encoding in hayro-font src/lib.rs.
type encoding uint8

// The base encodings of a simple font.
const (
	encodingStandard encoding = iota
	encodingMacRoman
	encodingWinAnsi
	encodingMacExpert
	encodingBuiltIn
)

// mapCode is Encoding::map_code in hayro-font src/lib.rs. It returns the
// glyph name of a code.
func (e encoding) mapCode(code uint8) (string, bool) {
	if code == 0 {
		return ".notdef", true
	}
	switch e {
	case encodingStandard:
		return standardGet(code)
	case encodingMacRoman:
		if s, ok := macRomanGet(code); ok {
			return s, true
		}
		return macOSRomanGet(code)
	case encodingWinAnsi:
		return winAnsiGet(code)
	case encodingMacExpert:
		return macExpertGet(code)
	}
	return "", false
}

// fontFlags is FontFlags in hayro-font src/lib.rs. It holds the flags of a
// font descriptor.
type fontFlags uint32

// The flags of a font descriptor.
const (
	fontFlagFixedPitch  fontFlags = 1 << 0
	fontFlagSerif       fontFlags = 1 << 1
	fontFlagSymbolic    fontFlags = 1 << 2
	fontFlagScript      fontFlags = 1 << 3
	fontFlagNonSymbolic fontFlags = 1 << 5
	fontFlagItalic      fontFlags = 1 << 6
	fontFlagAllCap      fontFlags = 1 << 16
	fontFlagSmallCap    fontFlags = 1 << 17
	fontFlagForceBold   fontFlags = 1 << 18

	fontFlagsAll = fontFlagFixedPitch | fontFlagSerif | fontFlagSymbolic | fontFlagScript |
		fontFlagNonSymbolic | fontFlagItalic | fontFlagAllCap | fontFlagSmallCap | fontFlagForceBold
)

// fontFlagsFromBits is FontFlags::from_bits of bitflags. It returns false
// if bits holds a bit that is not a flag.
func fontFlagsFromBits(bits uint32) (fontFlags, bool) {
	f := fontFlags(bits)
	if f&^fontFlagsAll != 0 {
		return 0, false
	}
	return f, true
}

// fontFlagsFromBitsTruncate is FontFlags::from_bits_truncate of bitflags.
// It drops each bit that is not a flag.
func fontFlagsFromBitsTruncate(bits uint32) fontFlags {
	return fontFlags(bits) & fontFlagsAll
}

// contains is FontFlags::contains of bitflags.
func (f fontFlags) contains(flag fontFlags) bool {
	return f&flag == flag
}

// stretchFactor is stretch_factor in hayro-font src/lib.rs.
func stretchFactor(expectedWidth, actualWidth float32) float64 {
	if actualWidth != 0 && actualWidth != expectedWidth {
		return float64(expectedWidth / actualWidth)
	}
	return 1
}

// stripSubsetPrefix is strip_subset_prefix in hayro-font src/lib.rs. It
// removes a prefix of six bytes and a plus sign, such as ABCDEF+.
func stripSubsetPrefix(name string) string {
	if prefix, rest, ok := strings.Cut(name, "+"); ok && len(prefix) == 6 {
		return rest
	}
	return name
}

// glyphNameToUnicode is glyph_name_to_unicode in hayro-font src/lib.rs. It
// maps a glyph name to Unicode text, as the Adobe Glyph List specification
// says.
func glyphNameToUnicode(name string, isZapfDingbats bool) (cmap.BfString, bool) {
	base, _, _ := strings.Cut(name, ".")
	var out strings.Builder
	for component := range strings.SplitSeq(base, "_") {
		if c, ok := zapfDingsGlyphNameToUnicode(component); isZapfDingbats && ok {
			out.WriteRune(c)
		} else if s, ok := glyphNamesGet(component); ok {
			out.WriteString(s)
		} else if s, ok := texGlyphNamesGet(component); ok {
			out.WriteString(s)
		} else if s, ok := uniComponent(component); ok {
			out.WriteString(s)
		} else if c, ok := uComponent(component); ok {
			out.WriteRune(c)
		}
	}
	return bfStringFromStr(out.String())
}

// uniComponent is uni_component in hayro-font src/lib.rs. It decodes the
// groups of four uppercase hexadecimal digits after "uni".
func uniComponent(component string) (string, bool) {
	digits, ok := strings.CutPrefix(component, "uni")
	if !ok || len(digits) == 0 || len(digits)%4 != 0 {
		return "", false
	}
	var out strings.Builder
	for i := 0; i < len(digits); i += 4 {
		v, ok := parseUppercaseHex(digits[i : i+4])
		if !ok || !utf8.ValidRune(rune(v)) {
			return "", false
		}
		out.WriteRune(rune(v))
	}
	return out.String(), true
}

// uComponent is u_component in hayro-font src/lib.rs. It decodes four to
// six uppercase hexadecimal digits after "u".
func uComponent(component string) (rune, bool) {
	digits, ok := strings.CutPrefix(component, "u")
	if !ok || len(digits) < 4 || len(digits) > 6 {
		return 0, false
	}
	v, ok := parseUppercaseHex(digits)
	if !ok || v > utf8.MaxRune || !utf8.ValidRune(rune(v)) {
		return 0, false
	}
	return rune(v), true
}

// parseUppercaseHex is parse_uppercase_hex in hayro-font src/lib.rs.
func parseUppercaseHex(digits string) (uint32, bool) {
	var value uint32
	for i := range len(digits) {
		d := digits[i]
		switch {
		case d >= '0' && d <= '9':
			d -= '0'
		case d >= 'A' && d <= 'F':
			d = d - 'A' + 10
		default:
			return 0, false
		}
		value = value*16 + uint32(d)
	}
	return value, true
}

// bfStringFromStr is bf_string_from_str in hayro-font src/lib.rs.
func bfStringFromStr(s string) (cmap.BfString, bool) {
	r, size := utf8.DecodeRuneInString(s)
	switch {
	case s == "":
		return cmap.BfString{}, false
	case size == len(s):
		return cmap.NewBfChar(r), true
	}
	return cmap.NewBfString(s), true
}

// unicodeFromName is unicode_from_name in hayro-font src/lib.rs. It reads
// the hexadecimal digits after "uni" or "u".
func unicodeFromName(name string) (rune, bool) {
	convert := func(input string) (rune, bool) {
		v, err := strconv.ParseUint(input, 16, 32)
		if err != nil || !utf8.ValidRune(rune(v)) {
			return 0, false
		}
		return rune(v), true
	}
	if rest, ok := strings.CutPrefix(name, "uni"); ok {
		if r, ok := convert(rest); ok {
			return r, true
		}
	}
	if rest, ok := strings.CutPrefix(name, "u"); ok {
		return convert(rest)
	}
	return 0, false
}

// readToUnicode is read_to_unicode in hayro-font src/lib.rs. It reads the
// ToUnicode CMap of a font.
func readToUnicode(dict syntax.Dict, cmapResolver CMapResolverFn) (*cmap.CMap, bool) {
	var data []byte
	ok := false
	if s, sok := dict.Get[syntax.Stream](syntax.KeyToUnicode); sok {
		if d, err := s.DecodedData(); err == nil {
			data, ok = d, true
		}
	}
	// Some PDFs name a predefined CMap such as `Identity-H` as `ToUnicode`
	// (PDFJS-11915).
	if !ok && cmapResolver != nil {
		if name, nok := dict.Get[syntax.Name](syntax.KeyToUnicode); nok {
			data, ok = cmapResolver(cmap.NameFromBytes([]byte(name)))
		}
	}
	if !ok {
		return nil, false
	}
	c, err := cmap.Parse(data, resolverOf(cmapResolver))
	if err != nil {
		return nil, false
	}
	return c, true
}

// resolverOf returns the resolver of the cmap package for a CMapResolverFn,
// or nil.
func resolverOf(r CMapResolverFn) cmap.Resolver {
	if r == nil {
		return nil
	}
	return cmap.Resolver(r)
}

// normalizedGlyphName is normalized_glyph_name in hayro-font src/lib.rs.
func normalizedGlyphName(name string) string {
	if name == "nbspace" {
		name = "space"
	}
	if name == "sfthyphen" {
		name = "hyphen"
	}
	return name
}
