// Copyright (c) The Hayro Authors.
//
// Use of this source code is governed by the Apache-2.0 or MIT license that
// can be found in the LICENSE-APACHE and LICENSE-MIT files.

// This file ports hayro-font src/program/mod.rs.
//
// Port: upstream reads each font program with skrifa. Go reads TrueType,
// OpenType and CFF with go-text, through skrifa_sfnt.go and skrifa_cff.go,
// and Type 1 with skrifa_type1.go.

package pdffont

import (
	"bytes"
	"fmt"
	"math"

	"github.com/go-text/typesetting/font/opentype/tables"

	"github.com/xo/ivy"
	"github.com/xo/ivy/pdf/internal/kurboext"
)

// programKind is ProgramKind in hayro-font src/program/mod.rs.
type programKind uint8

// The kinds of a font program.
const (
	programSfnt programKind = iota
	programCff
	programType1
)

// cmapPlatform is CmapPlatform in hayro-font src/program/mod.rs.
type cmapPlatform uint8

// The platforms of a subtable of a cmap table.
const (
	cmapPlatformUnicode cmapPlatform = iota
	cmapPlatformMacintosh
	cmapPlatformWindows
	cmapPlatformOther
)

// cmapSubtable is CmapSubtable in hayro-font src/program/mod.rs. It is one
// subtable of the cmap table of a TrueType font.
type cmapSubtable struct {
	platform   cmapPlatform
	encodingID uint16
	sub        tables.CmapSubtable
}

// mapCode is CmapSubtable::map in hayro-font src/program/mod.rs. It returns
// the glyph of a code.
func (c cmapSubtable) mapCode(code uint32) (GlyphID, bool) {
	return cmapMap(c.sub, code)
}

// fontProgram is FontProgram in hayro-font src/program/mod.rs. It is a
// parsed font program, which fonts share.
//
// Port: upstream holds a font of skrifa. Go holds the reader of the kind of
// the program. cff is the font of the kind Cff, or the CFF table of an
// OpenType font.
type fontProgram struct {
	hash Key128
	data FontData
	kind programKind

	sfnt  *sfntProgram
	cff   *cffProgram
	type1 *type1Program
}

// newFontProgram is FontProgram::new in hayro-font src/program/mod.rs.
//
// Port: upstream returns None for each error. Go returns an error that wraps
// ivy.ErrFormat.
func newFontProgram(data FontData, index uint32) (*fontProgram, error) {
	p := &fontProgram{hash: contentHash(data, index), data: data}
	switch {
	case isSfnt(data):
		s, err := newSfntProgram(data, index)
		// Some embedded OpenType fonts lack `maxp`, so skrifa finds no
		// glyphs.
		//
		// Port: go-text fails on a font without maxp, where skrifa reads
		// a font without glyphs. Go tries the CFF table after either.
		if err != nil || s.numGlyphs == 0 {
			if raw := sfntCFFTable(data, index); raw != nil {
				if c, cerr := newCFFProgram(raw); cerr == nil {
					p.kind, p.cff = programCff, c
					return p, nil
				}
			}
			if err != nil {
				return nil, err
			}
		}
		// Reject invalid `post` versions, which skrifa accepts (pdf.js
		// issue 9462).
		if v, ok := s.postVersion(); ok && len(s.post) >= 32 {
			switch v {
			case 0x00010000, 0x00020000, 0x00025000, 0x00030000:
			default:
				return nil, fmt.Errorf("post table of version %#x: %w", v, ivy.ErrFormat)
			}
		}
		p.kind, p.sfnt, p.cff = programSfnt, s, s.cff
	case isType1(data):
		t, err := newType1Program(data)
		if err != nil {
			return nil, err
		}
		p.kind, p.type1 = programType1, t
	default:
		c, err := newCFFProgram(data)
		if err != nil {
			return nil, err
		}
		p.kind, p.cff = programCff, c
	}
	return p, nil
}

// isSfnt reports whether data starts with the tag of a TrueType or OpenType
// font, or of a collection.
func isSfnt(data []byte) bool {
	if len(data) < 4 {
		return false
	}
	switch string(data[:4]) {
	case "\x00\x01\x00\x00", "OTTO", "true", "ttcf", "typ1":
		return true
	}
	return false
}

// isType1 reports whether data starts as a Type 1 font: PostScript text or
// the first segment of a PFB file.
func isType1(data []byte) bool {
	return bytes.HasPrefix(data, []byte("%!")) || len(data) >= 2 && data[0] == 0x80 && data[1] == 0x01
}

// programKind is FontProgram::kind in hayro-font src/program/mod.rs.
func (p *fontProgram) programKind() programKind {
	return p.kind
}

// contentHash is FontProgram::content_hash in hayro-font
// src/program/mod.rs.
func (p *fontProgram) contentHash() Key128 {
	return p.hash
}

// fontData is FontProgram::data in hayro-font src/program/mod.rs.
func (p *fontProgram) fontData() FontData {
	return p.data
}

// hasCffOutlines is FontProgram::has_cff_outlines in hayro-font
// src/program/mod.rs.
func (p *fontProgram) hasCffOutlines() bool {
	return p.cff != nil
}

// isCIDKeyed is FontProgram::is_cid_keyed in hayro-font src/program/mod.rs.
func (p *fontProgram) isCIDKeyed() bool {
	return p.cff != nil && p.cff.isCID
}

// glyphByName is FontProgram::glyph_by_name in hayro-font
// src/program/mod.rs.
func (p *fontProgram) glyphByName(name string) (GlyphID, bool) {
	switch p.kind {
	case programSfnt:
		return p.sfnt.glyphByName(name)
	case programCff:
		return p.cff.glyphByName(name)
	}
	return p.type1.glyphByName(name)
}

// glyphByCID is FontProgram::glyph_by_cid in hayro-font
// src/program/mod.rs.
func (p *fontProgram) glyphByCID(cid uint32) (GlyphID, bool) {
	if p.cff == nil {
		return 0, false
	}
	return p.cff.glyphByCID(cid)
}

// builtinEncodingGlyph is FontProgram::builtin_encoding_glyph in hayro-font
// src/program/mod.rs.
func (p *fontProgram) builtinEncodingGlyph(code uint8) (GlyphID, bool) {
	if p.kind == programType1 {
		return p.type1.encodingGlyph(code)
	}
	if p.cff == nil {
		return 0, false
	}
	return p.cff.encodingGlyph(code)
}

// builtinEncodingGlyphName is FontProgram::builtin_encoding_glyph_name in
// hayro-font src/program/mod.rs.
func (p *fontProgram) builtinEncodingGlyphName(code uint8) (string, bool) {
	if p.kind == programType1 {
		return p.type1.encodingName(code)
	}
	if p.cff == nil {
		return "", false
	}
	g, ok := p.cff.encodingGlyph(code)
	if !ok {
		return "", false
	}
	return p.cff.glyphName(g)
}

// cmapGlyph is FontProgram::cmap_glyph in hayro-font src/program/mod.rs.
//
// Port: go-text picks the subtable of the cmap table, where skrifa picks it
// with its own rules. Both prefer a Unicode subtable.
func (p *fontProgram) cmapGlyph(c rune) (GlyphID, bool) {
	if p.kind != programSfnt {
		return 0, false
	}
	return p.sfnt.unicodeGlyph(c)
}

// cmapSubtables is FontProgram::cmap_subtables in hayro-font
// src/program/mod.rs.
func (p *fontProgram) cmapSubtables() []cmapSubtable {
	if p.kind != programSfnt {
		return nil
	}
	out := make([]cmapSubtable, 0, len(p.sfnt.cmap.Records))
	for _, r := range p.sfnt.cmap.Records {
		platform := cmapPlatformOther
		switch r.PlatformID {
		case 0:
			platform = cmapPlatformUnicode
		case 1:
			platform = cmapPlatformMacintosh
		case 3:
			platform = cmapPlatformWindows
		}
		out = append(out, cmapSubtable{platform: platform, encodingID: uint16(r.EncodingID), sub: r.Subtable})
	}
	return out
}

// advanceWidth is FontProgram::advance_width in hayro-font
// src/program/mod.rs.
func (p *fontProgram) advanceWidth(glyph GlyphID) (float32, bool) {
	if p.kind != programSfnt {
		return 0, false
	}
	adv, ok := p.sfnt.advance(glyph)
	if !ok {
		return 0, false
	}
	return adv * UnitsPerEm / p.sfnt.upem, true
}

// weight is FontProgram::weight in hayro-font src/program/mod.rs.
func (p *fontProgram) weight() (uint32, bool) {
	if p.kind != programSfnt {
		return 0, false
	}
	w := uint32(math.Round(float64(p.sfnt.weight())))
	return w, w > 0
}

// isItalic is FontProgram::is_italic in hayro-font src/program/mod.rs.
func (p *fontProgram) isItalic() bool {
	return p.kind == programSfnt && p.sfnt.isItalic()
}

// isMonospace is FontProgram::is_monospace in hayro-font
// src/program/mod.rs.
func (p *fontProgram) isMonospace() bool {
	return p.kind == programSfnt && p.sfnt.isFixedPitch()
}

// hintedOutline is FontProgram::hinted_outline in hayro-font
// src/program/mod.rs.
//
// Port: go-text has no hinter, so no font can be hinted. The function always
// returns false, which upstream returns for a font that it cannot hint.
func (p *fontProgram) hintedOutline(GlyphID, float32, *HintingCache) (kurboext.BezPath, bool) {
	return kurboext.BezPath{}, false
}

// outline is FontProgram::outline in hayro-font src/program/mod.rs. It
// returns the outline of a glyph, scaled to UnitsPerEm.
func (p *fontProgram) outline(glyph GlyphID) kurboext.BezPath {
	pen := newOutlinePath()
	switch p.kind {
	case programSfnt:
		p.sfnt.outline(glyph, pen)
	case programCff:
		p.cff.outline(glyph, pen)
	case programType1:
		p.type1.outline(glyph, pen)
	}
	return pen.take()
}

// contentHash is content_hash in hayro-font src/program/mod.rs.
func contentHash(data []byte, index uint32) Key128 {
	return newKeyHasher().bytes(data).u64(uint64(index)).sum()
}
