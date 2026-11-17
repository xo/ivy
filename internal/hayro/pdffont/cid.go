// Copyright (c) The Hayro Authors.
//
// Use of this source code is governed by the Apache-2.0 or MIT license that
// can be found in the LICENSE-APACHE and LICENSE-MIT files.

// This file ports hayro-font src/cid.rs.

package pdffont

import (
	"container/heap"
	"encoding/binary"
	"math"
	"slices"
	"sort"
	"strings"
	utf16pkg "unicode/utf16"
	"unicode/utf8"

	"github.com/xo/okage/kurbo"

	"github.com/xo/ivy/internal/hayro/cmap"
	"github.com/xo/ivy/internal/hayro/syntax"
)

// type0Font is Type0Font in hayro-font src/cid.rs. It is a composite font
// with a CID font as its descendant.
type type0Font struct {
	prog                  *fontProgram
	horizontal            bool
	dw                    float32
	dw2                   [2]float32
	widths                widths[float32]
	encoding              *cmap.CMap
	toUnicode             *cmap.CMap
	widths2               widths[[3]float32]
	cidToGIDMap           cidToGIDMap
	psName                string
	fontFlags             fontFlags
	hasFlags              bool
	fallback              bool
	toUnicodeIsCIDIndexed bool

	// latin is the substitute of a standard font that draws the characters
	// that a fallback font of a CJK collection does not draw, or nil.
	//
	// Port: upstream has no such font.
	latin *fontProgram
	// unicode is the Unicode encoding of the codes, for an encoding CMap of
	// Unicode such as UniJIS-UCS2-H.
	//
	// Port: upstream does not read it.
	unicode unicodeEncoding
}

// unicodeEncoding is the Unicode encoding of the codes of a predefined CMap
// of Unicode.
type unicodeEncoding uint8

// The encodings of unicodeEncoding.
const (
	// notUnicode is a CMap that is not of Unicode.
	notUnicode unicodeEncoding = iota
	// utf16 is the encoding of a CMap whose name holds UCS2 or UTF16. A
	// code of 2 bytes is a character of the BMP, and a code of 4 bytes is
	// a surrogate pair.
	utf16
	// utf32 is the encoding of a CMap whose name holds UTF32.
	utf32
)

// unicodeEncodingOf returns the Unicode encoding of the CMap of the entry
// Encoding of a Type 0 font.
func unicodeEncodingOf(enc syntax.Object) unicodeEncoding {
	name, ok := enc.(syntax.Name)
	if !ok || !strings.HasPrefix(string(name), "Uni") {
		return notUnicode
	}
	switch {
	case strings.Contains(string(name), "-UCS2-"), strings.Contains(string(name), "-UTF16-"):
		return utf16
	case strings.Contains(string(name), "-UTF32-"):
		return utf32
	}
	return notUnicode
}

// char returns the character of a code of the encoding, and false if the
// code is not a valid character.
func (u unicodeEncoding) char(code uint32) (rune, bool) {
	var r rune
	switch u {
	case utf16:
		if code > 0xFFFF {
			r = utf16pkg.DecodeRune(rune(code>>16), rune(code&0xFFFF))
		} else {
			r = rune(code)
		}
	case utf32:
		r = rune(code)
	default:
		return 0, false
	}
	return r, r > 0 && validChar(uint32(r)) && r != utf8.RuneError
}

// latinGlyph marks a glyph of type0Font.latin. No font has a glyph of an
// ID this large.
const latinGlyph GlyphID = 1 << 31

// isCJK reports whether cc is one of the four CJK collections of Adobe:
// Japan1, GB1, CNS1 and Korea1.
func isCJK(cc *cmap.CharacterCollection) bool {
	if cc == nil {
		return false
	}
	switch cc.Family.Kind {
	case cmap.AdobeJapan1, cmap.AdobeGB1, cmap.AdobeCNS1, cmap.AdobeKorea1:
		return true
	}
	return false
}

// newType0Font is Type0Font::new in hayro-font src/cid.rs.
func newType0Font(dict syntax.Dict, loader *fontLoader) (*type0Font, bool) {
	cmapResolver := loader.cmapResolver
	encObj, ok := dict.Get[syntax.Object](syntax.KeyEncoding)
	if !ok {
		return nil, false
	}
	cm, ok := readCIDEncoding(encObj, cmapResolver)
	if !ok {
		return nil, false
	}
	wm := cm.Metadata().WritingMode
	horizontal := wm == nil || *wm != cmap.Vertical

	descendants, ok := dict.Get[syntax.Array](syntax.KeyDescendantFonts)
	if !ok {
		return nil, false
	}
	descendantFont, ok := descendants.Iter[syntax.Dict]().Next()
	if !ok {
		return nil, false
	}
	fontDescriptor, _ := descendantFont.Get[syntax.Dict](syntax.KeyFontDescriptor)

	var characterCollection *cmap.CharacterCollection
	if cc := cm.Metadata().CharacterCollection; cc != nil && cc.Family != (cmap.CIDFamily{Kind: cmap.AdobeIdentity}) {
		c := *cc
		characterCollection = &c
	} else if cc, ok := readCIDSystemInfo(descendantFont); ok {
		characterCollection = &cc
	}

	program, ok := loadCIDProgram(fontDescriptor, loader)
	fallback := !ok
	var latin *fontProgram
	if !ok {
		// Port: upstream does not ask for the font that dict names.
		program, ok = loader.programs.resolveNamed(dict)
	}
	if !ok {
		var query FontQuery
		if standard, _, ok := selectStandardFont(dict, fontDescriptor); ok {
			query = FontQuery{IsStandard: true, Standard: standard}
		} else {
			q := newFallbackFontQuery(dict)
			q.CharacterCollection = characterCollection
			query = FontQuery{Fallback: q}
			if isCJK(characterCollection) {
				// Port: upstream draws the text of a CJK collection with
				// the font of the resolver only. A CJK font, such as Droid
				// Sans Fallback, can lack the Latin letters. Go draws them
				// with the substitute of a standard font.
				latin, _ = loader.programs.resolve(&FontQuery{IsStandard: true, Standard: q.PickStandardFont()})
			}
		}
		if program, ok = loader.programs.resolve(&query); !ok {
			return nil, false
		}
	}

	defaultWidth, ok := descendantFont.Get[float32](syntax.KeyDW)
	if !ok {
		defaultWidth = 1000
	}
	dw2, ok := descendantFont.Get[[2]float32](syntax.KeyDW2)
	if !ok {
		dw2 = [2]float32{880, -1000}
	}

	var w widths[float32]
	if a, ok := descendantFont.Get[syntax.Array](syntax.KeyW); ok {
		w, _ = readCIDWidths(a)
	}
	var w2 widths[[3]float32]
	if a, ok := descendantFont.Get[syntax.Array](syntax.KeyW2); ok {
		w2, _ = readCIDWidths2(a)
	}
	needsInverse := program.programKind() == programCff && program.isCIDKeyed()
	cidToGID, _ := newCIDToGIDMap(descendantFont, needsInverse)
	toUnicode, hasToUnicode := readToUnicode(dict, cmapResolver)
	toUnicodeIsCIDIndexed := false

	if !hasToUnicode && characterCollection != nil && cmapResolver != nil {
		if ucs2Name, ok := characterCollection.Family.UCS2CMap(); ok {
			if data, ok := cmapResolver(ucs2Name); ok {
				if ucs2, err := cmap.Parse(data, resolverOf(cmapResolver)); err == nil {
					toUnicode = ucs2
					toUnicodeIsCIDIndexed = true
				}
			}
		}
	}

	var postscriptName string
	if n, ok := dict.Get[syntax.Name](syntax.KeyBaseFont); ok {
		postscriptName = stripSubsetPrefix(n.AsStr())
	}
	var flags fontFlags
	hasFlags := false
	if bits, ok := fontDescriptor.Get[uint32](syntax.KeyFlags); ok {
		flags, hasFlags = fontFlagsFromBits(bits)
	}

	return &type0Font{
		horizontal:            horizontal,
		encoding:              cm,
		toUnicode:             toUnicode,
		prog:                  program,
		dw:                    defaultWidth,
		dw2:                   dw2,
		widths:                w,
		widths2:               w2,
		cidToGIDMap:           cidToGID,
		psName:                postscriptName,
		fontFlags:             flags,
		hasFlags:              hasFlags,
		fallback:              fallback,
		latin:                 latin,
		unicode:               unicodeEncodingOf(encObj),
		toUnicodeIsCIDIndexed: toUnicodeIsCIDIndexed,
	}, true
}

// mapCode is Type0Font::map_code in hayro-font src/cid.rs.
func (t *type0Font) mapCode(code uint32) GlyphID {
	cid, ok := t.codeToCID(code)
	if t.latin != nil && (!ok || cid == 0) {
		// Port: upstream draws nothing for a code that the collection does
		// not hold. The code of a Unicode CMap, such as UniJIS-UCS2-H, is
		// a character, so Go draws it with the fallback font.
		if r, ok := t.unicode.char(code); ok {
			return t.fallbackGlyph(r)
		}
	}
	if !ok {
		return GlyphIDNotdef
	}
	toUnicodeKey := code
	if t.toUnicodeIsCIDIndexed {
		toUnicodeKey = cid
	}
	if t.fallback && t.latin != nil {
		// Port: the glyph ID of a CID is a glyph of the font that the file
		// does not embed, so Go draws nothing for a character that neither
		// font draws.
		if r, ok := t.unicodeOf(toUnicodeKey); ok {
			return t.fallbackGlyph(r)
		}
		return GlyphIDNotdef
	}
	if t.fallback {
		if glyph, ok := t.mapViaUnicode(toUnicodeKey); ok {
			return glyph
		}
	}
	switch t.prog.programKind() {
	case programSfnt:
		return t.cidToGIDMap.mapCode(uint16(cid))
	case programCff:
		if t.prog.isCIDKeyed() {
			// Acrobat applies `CIDToGIDMap`s to CFF fonts inversely (pdf.js
			// PR 15563).
			if t.cidToGIDMap.identity {
				if g, ok := t.prog.glyphByCID(cid); ok {
					return g
				}
				return GlyphIDNotdef
			}
			return NewGlyphID(uint32(t.cidToGIDMap.inverseMap(NewGlyphID(cid))))
		}
		return t.cidToGIDMap.mapCode(uint16(cid))
	}
	return NewGlyphID(cid)
}

// mapViaUnicode is Type0Font::map_via_unicode in hayro-font src/cid.rs.
func (t *type0Font) mapViaUnicode(key uint32) (GlyphID, bool) {
	r, ok := t.unicodeOf(key)
	if !ok {
		return 0, false
	}
	return glyphOfChar(t.prog, r)
}

// unicodeOf returns the character of a key of the ToUnicode CMap. It is the
// first half of Type0Font::map_via_unicode in hayro-font src/cid.rs.
func (t *type0Font) unicodeOf(key uint32) (rune, bool) {
	if t.toUnicode == nil {
		return 0, false
	}
	bf, found := t.toUnicode.LookupBfString(key)
	for length := uint8(0); !found && length < 4; length++ {
		if code, ok := t.toUnicode.LookupCIDCode(key, length); ok && validChar(code) {
			bf, found = cmap.NewBfChar(rune(code)), true
		}
	}
	if !found || bf.IsString {
		return 0, false
	}
	return bf.Char, true
}

// glyphOfChar returns the glyph of a character in prog. It is the second
// half of Type0Font::map_via_unicode in hayro-font src/cid.rs.
func glyphOfChar(prog *fontProgram, r rune) (GlyphID, bool) {
	if prog.programKind() == programSfnt {
		return prog.cmapGlyph(r)
	}
	name, ok := glyphNamesGetReverse(r)
	if !ok {
		return 0, false
	}
	return prog.glyphByName(name)
}

// fallbackGlyph returns the glyph of a character in the program of a font
// that has a latin font, then in the latin font, or .notdef.
//
// Port: upstream has no latin font.
func (t *type0Font) fallbackGlyph(r rune) GlyphID {
	if g, ok := glyphOfChar(t.prog, r); ok {
		return g
	}
	if g, ok := glyphOfChar(t.latin, r); ok {
		return g | latinGlyph
	}
	return GlyphIDNotdef
}

// programOf returns the program that draws glyph, and the ID of the glyph
// in that program.
//
// Port: upstream has one program for each font.
func (t *type0Font) programOf(glyph GlyphID) (*fontProgram, GlyphID) {
	if t.latin != nil && glyph&latinGlyph != 0 {
		return t.latin, glyph &^ latinGlyph
	}
	return t.prog, glyph
}

// validChar reports whether char::from_u32 of Rust accepts v.
func validChar(v uint32) bool {
	return v <= 0x10FFFF && (v < 0xD800 || v > 0xDFFF)
}

// codeToCID is Type0Font::code_to_cid in hayro-font src/cid.rs.
func (t *type0Font) codeToCID(code uint32) (uint32, bool) {
	for byteLen := uint8(1); byteLen <= 4; byteLen++ {
		if cid, ok := t.encoding.LookupCIDCode(code, byteLen); ok {
			return cid, true
		}
	}
	return 0, false
}

// program is Type0Font::program in hayro-font src/cid.rs.
func (t *type0Font) program() *fontProgram {
	return t.prog
}

// glyphStretch is Type0Font::glyph_stretch in hayro-font src/cid.rs.
func (t *type0Font) glyphStretch(glyph GlyphID, code uint32) float64 {
	if !t.fallback {
		return 1
	}
	cid, ok := t.codeToCID(code)
	if !ok {
		return 1
	}
	prog, glyph := t.programOf(glyph)
	actualWidth, ok := prog.advanceWidth(glyph)
	if !ok {
		return 1
	}
	// Stretching to `DW` gives bad results, so only explicit `W` entries
	// count.
	expectedWidth, ok := t.widths.get(cid)
	if !ok {
		return 1
	}
	return stretchFactor(expectedWidth, actualWidth)
}

// fontData is Type0Font::font_data in hayro-font src/cid.rs.
func (t *type0Font) fontData() (FontData, bool) {
	switch t.prog.programKind() {
	case programSfnt, programCff:
		return t.prog.fontData(), true
	}
	return nil, false
}

// postscriptName is Type0Font::postscript_name in hayro-font src/cid.rs. It
// returns the empty string for None.
func (t *type0Font) postscriptName() string {
	return t.psName
}

// weight is Type0Font::weight in hayro-font src/cid.rs.
func (t *type0Font) weight() (uint32, bool) {
	return t.prog.weight()
}

// isItalic is Type0Font::is_italic in hayro-font src/cid.rs.
func (t *type0Font) isItalic() bool {
	if t.hasFlags && t.fontFlags.contains(fontFlagItalic) {
		return true
	}
	return t.prog.isItalic()
}

// isSerif is Type0Font::is_serif in hayro-font src/cid.rs.
func (t *type0Font) isSerif() bool {
	return t.hasFlags && t.fontFlags.contains(fontFlagSerif)
}

// isMonospace is Type0Font::is_monospace in hayro-font src/cid.rs.
func (t *type0Font) isMonospace() bool {
	if t.hasFlags && t.fontFlags.contains(fontFlagFixedPitch) {
		return true
	}
	return t.prog.isMonospace()
}

// codeAdvance is Type0Font::code_advance in hayro-font src/cid.rs.
func (t *type0Font) codeAdvance(code uint32) kurbo.Vec2 {
	cid, _ := t.codeToCID(code)
	if t.horizontal {
		return kurbo.NewVec2(float64(t.horizontalWidth(cid)), 0)
	}
	if w, ok := t.widths2.get(cid); ok {
		return kurbo.NewVec2(0, float64(w[0]))
	}
	return kurbo.NewVec2(0, float64(t.dw2[1]))
}

// horizontalWidth is Type0Font::horizontal_width in hayro-font src/cid.rs.
func (t *type0Font) horizontalWidth(cid uint32) float32 {
	if w, ok := t.widths.get(cid); ok {
		return w
	}
	return t.dw
}

// isHorizontal is Type0Font::is_horizontal in hayro-font src/cid.rs.
func (t *type0Font) isHorizontal() bool {
	return t.horizontal
}

// readCode is Type0Font::read_code in hayro-font src/cid.rs.
func (t *type0Font) readCode(bytes []byte, offset int) (uint32, int) {
	var code uint32
	remaining := len(bytes) - offset
	for n := range min(4, remaining) {
		code = code<<8 | uint32(bytes[offset+n])
		if _, ok := t.encoding.LookupCIDCode(code, uint8(n+1)); ok {
			return code, n + 1
		}
	}
	// Port: upstream reads a code that the CMap does not map as one byte,
	// so that a code of two bytes advances twice. Go reads the length of
	// the code from the codespace ranges of the CMap, as pdf.js does.
	code = 0
	for n := range min(4, remaining) {
		code = code<<8 | uint32(bytes[offset+n])
		if inCodespace(t.encoding, code, uint8(n+1)) {
			return code, n + 1
		}
	}
	return 0, 1
}

// inCodespace reports whether a code of n bytes is inside a codespace range
// of c or of the CMaps that it uses.
func inCodespace(c *cmap.CMap, code uint32, n uint8) bool {
	for depth := 0; c != nil && depth < 16; depth++ {
		for _, r := range c.CodespaceRanges() {
			if r.ByteLen() == n && code >= r.Low() && code <= r.High() {
				return true
			}
		}
		c = c.Base()
	}
	return false
}

// originDisplacement is Type0Font::origin_displacement in hayro-font
// src/cid.rs.
func (t *type0Font) originDisplacement(code uint32) kurbo.Vec2 {
	cid, _ := t.codeToCID(code)
	if t.isHorizontal() {
		return kurbo.Vec2{}
	}
	if w, ok := t.widths2.get(cid); ok {
		return kurbo.NewVec2(-float64(w[1]), -float64(w[2]))
	}
	return kurbo.NewVec2(-float64(t.horizontalWidth(cid))/2, -float64(t.dw2[0]))
}

// charCodeToUnicode is Type0Font::char_code_to_unicode in hayro-font
// src/cid.rs.
func (t *type0Font) charCodeToUnicode(code uint32) (cmap.BfString, bool) {
	if t.toUnicode == nil {
		return cmap.BfString{}, false
	}
	if t.toUnicodeIsCIDIndexed {
		cid, _ := t.codeToCID(code)
		return t.toUnicode.LookupBfString(cid)
	}
	if bf, ok := t.toUnicode.LookupBfString(code); ok {
		return bf, true
	}
	// Some `ToUnicode` maps use `cidrange` instead of `bfrange`
	// (pdfjs_issue9367).
	for length := uint8(1); length <= 4; length++ {
		if v, ok := t.toUnicode.LookupCIDCode(code, length); ok {
			if !validChar(v) {
				return cmap.BfString{}, false
			}
			return cmap.NewBfChar(rune(v)), true
		}
	}
	return cmap.BfString{}, false
}

// cidToGIDMap is CidToGIdMap in hayro-font src/cid.rs. The zero value is
// the case Identity.
//
// Port: upstream is an enum. identity marks the case Identity, and forward
// and inverse hold the case Mapped.
type cidToGIDMap struct {
	identity bool
	forward  []uint16
	inverse  map[GlyphID]uint16
}

// newCIDToGIDMap is CidToGIdMap::new in hayro-font src/cid.rs. It returns
// false where upstream returns None. The caller then uses the default,
// Identity.
func newCIDToGIDMap(dict syntax.Dict, withInverse bool) (cidToGIDMap, bool) {
	identity := cidToGIDMap{identity: true}
	if name, ok := dict.Get[syntax.Name](syntax.KeyCIDToGIDMap); ok {
		return identity, name == syntax.KeyIdentity
	}
	stream, ok := dict.Get[syntax.Stream](syntax.KeyCIDToGIDMap)
	if !ok {
		return identity, false
	}
	decoded, err := stream.DecodedData()
	if err != nil {
		return identity, false
	}
	n := min(len(decoded)/2, math.MaxUint16+1)
	forward := make([]uint16, n)
	for i := range forward {
		forward[i] = binary.BigEndian.Uint16(decoded[2*i:])
	}
	inverse := make(map[GlyphID]uint16)
	if withInverse {
		for cid, gid := range forward {
			inverse[NewGlyphID(uint32(gid))] = uint16(cid)
		}
	}
	return cidToGIDMap{forward: forward, inverse: inverse}, true
}

// mapCode is CidToGIdMap::map in hayro-font src/cid.rs.
func (m cidToGIDMap) mapCode(code uint16) GlyphID {
	if m.identity {
		return NewGlyphID(uint32(code))
	}
	if int(code) < len(m.forward) {
		return NewGlyphID(uint32(m.forward[code]))
	}
	return GlyphIDNotdef
}

// inverseMap is CidToGIdMap::inverse_map in hayro-font src/cid.rs.
func (m cidToGIDMap) inverseMap(gid GlyphID) uint16 {
	if m.identity {
		return uint16(gid.ToU32())
	}
	if cid, ok := m.inverse[gid]; ok {
		return cid
	}
	return uint16(gid.ToU32())
}

// loadCIDProgram is load_program in hayro-font src/cid.rs.
func loadCIDProgram(descriptor syntax.Dict, loader *fontLoader) (*fontProgram, bool) {
	// Ignore the CIDFont subtype, as some PDFs pair it with the wrong font
	// file.
	stream, ok := descriptor.Get[syntax.Stream](syntax.KeyFontFile2)
	if !ok {
		stream, ok = descriptor.Get[syntax.Stream](syntax.KeyFontFile3)
	}
	if !ok {
		// `FontFile` is not allowed for CIDFonts, but Acrobat accepts it
		// (PDFBOX-2599).
		stream, ok = descriptor.Get[syntax.Stream](syntax.KeyFontFile)
	}
	if !ok {
		return nil, false
	}
	return loader.programs.load(stream)
}

// widthRange is one entry of the map of Widths in hayro-font src/cid.rs: the
// value of the CIDs first to last.
type widthRange[T any] struct {
	first, last uint32
	value       T
}

// widths is Widths in hayro-font src/cid.rs. It maps ranges of CIDs to
// values. A later entry wins over an earlier one where they overlap.
//
// Port: upstream splits the ranges in a BTreeMap as it inserts each entry.
// Go sweeps over the ends of the entries once, with a heap of the entries
// that cover the current CID, and keeps the disjoint ranges in a sorted
// slice. A lookup gives the same value, and the time grows with n log n, so
// that a large W array cannot take quadratic time.
type widths[T any] struct {
	ranges []widthRange[T]
}

// newWidths is Widths::new in hayro-font src/cid.rs.
func newWidths[T any](entries []widthRange[T]) widths[T] {
	// Each entry starts at first and ends before last+1. The sweep visits
	// these points in order. Between two points, the set of the entries
	// that cover a CID does not change.
	type event struct {
		pos   uint64
		index int
	}
	events := make([]event, 0, 2*len(entries))
	for i, e := range entries {
		events = append(events, event{uint64(e.first), i}, event{uint64(e.last) + 1, -1})
	}
	slices.SortFunc(events, func(a, b event) int {
		switch {
		case a.pos < b.pos:
			return -1
		case a.pos > b.pos:
			return 1
		}
		return 0
	})
	active := &indexHeap{}
	var out []widthRange[T]
	for i := 0; i < len(events); {
		pos := events[i].pos
		for ; i < len(events) && events[i].pos == pos; i++ {
			if idx := events[i].index; idx >= 0 {
				heap.Push(active, idx)
			}
		}
		// Drop the entries that end before pos.
		for active.Len() > 0 && uint64(entries[(*active)[0]].last) < pos {
			heap.Pop(active)
		}
		if active.Len() == 0 || i >= len(events) {
			continue
		}
		top := entries[(*active)[0]]
		out = append(out, widthRange[T]{first: uint32(pos), last: uint32(events[i].pos - 1), value: top.value})
	}
	return widths[T]{ranges: out}
}

// get is Widths::get in hayro-font src/cid.rs.
func (w widths[T]) get(cid uint32) (T, bool) {
	i := sort.Search(len(w.ranges), func(i int) bool { return w.ranges[i].first > cid }) - 1
	if i < 0 || cid > w.ranges[i].last {
		var zero T
		return zero, false
	}
	return w.ranges[i].value, true
}

// indexHeap is a max-heap of the indices of entries, so that the latest
// entry is on top.
type indexHeap []int

func (h indexHeap) Len() int           { return len(h) }
func (h indexHeap) Less(i, j int) bool { return h[i] > h[j] }
func (h indexHeap) Swap(i, j int)      { h[i], h[j] = h[j], h[i] }
func (h *indexHeap) Push(x any)        { *h = append(*h, x.(int)) }
func (h *indexHeap) Pop() any {
	old := *h
	x := old[len(old)-1]
	*h = old[:len(old)-1]
	return x
}

// readCIDWidths is read_widths in hayro-font src/cid.rs. It reads the W
// array of a CID font.
func readCIDWidths(arr syntax.Array) (widths[float32], bool) {
	var ranges []widthRange[float32]
	it := arr.FlexIter()
	for {
		if t, ok := it.Next[syntax.Tuple2[uint32, syntax.Array]](); ok {
			first := t.V0
			for w := range t.V1.Iter[float32]().All() {
				ranges = append(ranges, widthRange[float32]{first, first, w})
				if first == math.MaxUint32 {
					return widths[float32]{}, false
				}
				first++
			}
		} else if t, ok := it.Next[syntax.Tuple3[uint32, uint32, float32]](); ok {
			if t.V0 <= t.V1 {
				ranges = append(ranges, widthRange[float32]{t.V0, t.V1, t.V2})
			}
		} else {
			break
		}
	}
	return newWidths(ranges), true
}

// readCIDWidths2 is read_widths2 in hayro-font src/cid.rs. It reads the W2
// array of a CID font.
func readCIDWidths2(arr syntax.Array) (widths[[3]float32], bool) {
	var ranges []widthRange[[3]float32]
	it := arr.FlexIter()
	for {
		if t, ok := it.Next[syntax.Tuple2[uint32, syntax.Array]](); ok {
			first := t.V0
			vals := t.V1.Iter[float32]()
			for {
				w, ok := vals.Next()
				if !ok {
					break
				}
				v1, ok1 := vals.Next()
				v2, ok2 := vals.Next()
				if !ok1 || !ok2 {
					return widths[[3]float32]{}, false
				}
				ranges = append(ranges, widthRange[[3]float32]{first, first, [3]float32{w, v1, v2}})
				if first == math.MaxUint32 {
					return widths[[3]float32]{}, false
				}
				first++
			}
		} else if t, ok := it.Next[syntax.Tuple5[uint32, uint32, float32, float32, float32]](); ok {
			if t.V0 <= t.V1 {
				ranges = append(ranges, widthRange[[3]float32]{t.V0, t.V1, [3]float32{t.V2, t.V3, t.V4}})
			}
		} else {
			break
		}
	}
	return newWidths(ranges), true
}

// readCIDSystemInfo is read_cid_system_info in hayro-font src/cid.rs.
func readCIDSystemInfo(descendantFont syntax.Dict) (cmap.CharacterCollection, bool) {
	info, ok := descendantFont.Get[syntax.Dict](syntax.KeyCIDSystemInfo)
	if !ok {
		return cmap.CharacterCollection{}, false
	}
	registry, ok := info.Get[syntax.String](syntax.KeyRegistry)
	if !ok {
		return cmap.CharacterCollection{}, false
	}
	ordering, ok := info.Get[syntax.String](syntax.KeyOrdering)
	if !ok {
		return cmap.CharacterCollection{}, false
	}
	supplement, ok := info.Get[int32](syntax.KeySupplement)
	if !ok {
		supplement = 0
	}
	family := cmap.CIDFamilyFromRegistryOrdering(registry.AsBytes(), ordering.AsBytes())
	return cmap.CharacterCollection{Family: family, Supplement: supplement}, true
}

// readCIDEncoding is read_encoding in hayro-font src/cid.rs. It reads the
// CMap of the Encoding of a Type 0 font: a name or a stream.
//
// Upstream notes that it does not fetch the CMaps that usecmap names in the
// PDF file yet.
func readCIDEncoding(object syntax.Object, cmapResolver CMapResolverFn) (*cmap.CMap, bool) {
	switch o := object.(type) {
	case syntax.Name:
		name := cmap.NameFromBytes([]byte(o))
		switch name {
		case cmap.IdentityH:
			return cmap.NewIdentityH(), true
		case cmap.IdentityV:
			return cmap.NewIdentityV(), true
		}
		if cmapResolver == nil {
			return nil, false
		}
		data, ok := cmapResolver(name)
		if !ok {
			return nil, false
		}
		c, err := cmap.Parse(data, resolverOf(cmapResolver))
		return c, err == nil
	case syntax.Stream:
		decoded, err := o.DecodedData()
		if err != nil {
			return nil, false
		}
		c, err := cmap.Parse(decoded, resolverOf(cmapResolver))
		return c, err == nil
	}
	return nil, false
}
