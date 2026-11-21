// Copyright (c) The Hayro Authors.
//
// Use of this source code is governed by the Apache-2.0 or MIT license that
// can be found in the LICENSE-APACHE and LICENSE-MIT files.

// This file ports no file of hayro-font, and it does not port skrifa. It
// reads the TrueType and OpenType fonts that hayro-font reads with skrifa in
// src/program/mod.rs, with go-text. go-text parses the font, draws the
// outlines, and gives the advances, the names of the glyphs and the glyph of
// a character. This file adds the parts that go-text v0.3.5 does not give:
//
//  1. Each subtable of the cmap table, by platform and encoding. hayro-font
//     looks up a code in the subtables (3,1), (3,0) and (1,0) one by one,
//     without a change of the code. go-text exposes one Cmap that it picks,
//     and it changes the codes of the subtables (3,0) and (1,0). Its lookup
//     of a subtable is unexported, so cmapMap reads the parsed subtables of
//     tables.ParseCmap.
//  2. The version of the post table, the weight class and the style bits of
//     the OS/2 table, and the fixed pitch flag of the post table. go-text
//     keeps them in unexported fields. Font.Describe guesses from the names
//     of the font too, which skrifa does not do.
//  3. A font without a cmap table. font.NewFont fails on it, where skrifa
//     reads it. This file adds an empty cmap table and reads it again.
//  4. A table that lies outside the data. Loader.RawTable reserves the
//     length that the directory gives before it reads, so a small file can
//     make it reserve 4 GB. sfntTables reads the directory first.

package pdffont

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"slices"
	"sync"

	"github.com/xo/okage/kurbo"
	gofont "github.com/xo/okage/typesetting/font"
	ot "github.com/xo/okage/typesetting/font/opentype"
	"github.com/xo/okage/typesetting/font/opentype/tables"

	"github.com/xo/ivy"
)

// sfntProgram is a TrueType or OpenType font that go-text reads.
type sfntProgram struct {
	face      *gofont.Face
	numGlyphs int
	upem      float32

	// The raw tables that go-text does not expose.
	os2, head, post []byte
	cmap            tables.Cmap
	cff             *cffProgram

	namesOnce sync.Once
	names     map[string]GlyphID
}

// Tags of the tables that this file reads.
var (
	tagCmap = ot.MustNewTag("cmap")
	tagCFF  = ot.MustNewTag("CFF ")
	tagHead = ot.MustNewTag("head")
	tagMaxp = ot.MustNewTag("maxp")
	tagOS2  = ot.MustNewTag("OS/2")
	tagPost = ot.MustNewTag("post")
)

// sfntTables reads the table directory of the font at index in data, which
// can be a collection. It returns the tables that start inside data, each cut
// to the end of data.
//
// Port: go-text reserves the memory of the length that the directory gives
// for a table before it reads the table, so that a table of a declared
// length of 4 GB in a small file makes it reserve 4 GB. go-text gets only
// the tables that this function returns, rebuilt into a new font file.
// skrifa skips a table that runs past the end of the data. This function
// cuts it, as internal/font does, because a writer of fonts can declare a
// last table a few bytes longer than the file, and that table can hold the
// outlines.
func sfntTables(data []byte, index uint32) (map[ot.Tag][]byte, error) {
	errDir := fmt.Errorf("reading OpenType font: bad table directory: %w", ivy.ErrFormat)
	if len(data) < 12 {
		return nil, errDir
	}
	off := uint64(0)
	if string(data[:4]) == "ttcf" {
		n := binary.BigEndian.Uint32(data[8:])
		pos := 12 + 4*uint64(index)
		if index >= n || pos+4 > uint64(len(data)) {
			return nil, fmt.Errorf("font %d of a collection of %d: %w", index, n, ivy.ErrFormat)
		}
		off = uint64(binary.BigEndian.Uint32(data[pos:]))
	} else if index != 0 {
		return nil, fmt.Errorf("font %d of a single font: %w", index, ivy.ErrFormat)
	}
	if off+12 > uint64(len(data)) {
		return nil, errDir
	}
	numTables := uint64(binary.BigEndian.Uint16(data[off+4:]))
	tabs := make(map[ot.Tag][]byte)
	for i := range numTables {
		rec := off + 12 + 16*i
		if rec+16 > uint64(len(data)) {
			break
		}
		tag := ot.Tag(binary.BigEndian.Uint32(data[rec:]))
		start := uint64(binary.BigEndian.Uint32(data[rec+8:]))
		length := uint64(binary.BigEndian.Uint32(data[rec+12:]))
		if start > uint64(len(data)) {
			continue
		}
		end := min(start+length, uint64(len(data)))
		if _, ok := tabs[tag]; !ok {
			// The capacity ends with the table. ot.WriteTTF appends the
			// padding of the checksum to the table, which would else
			// write over the start of the next table in data.
			tabs[tag] = data[start:end:end]
		}
	}
	return tabs, nil
}

// newSfntProgram reads the font at index in data.
//
// go-text fails on a font without a cmap table, which skrifa reads. Such a
// font gets a cmap table that maps nothing, and go-text reads it again.
func newSfntProgram(data []byte, index uint32) (*sfntProgram, error) {
	tabs, err := sfntTables(data, index)
	if err != nil {
		return nil, err
	}
	f := &sfntProgram{os2: tabs[tagOS2], head: tabs[tagHead], post: tabs[tagPost]}
	if raw, ok := tabs[tagCmap]; ok {
		_ = safely(func() error {
			var err error
			f.cmap, _, err = tables.ParseCmap(raw)
			return goTextError(err)
		})
	}
	if raw := tabs[tagMaxp]; len(raw) >= 6 {
		f.numGlyphs = int(binary.BigEndian.Uint16(raw[4:]))
	}
	font, err := goTextFont(tabs)
	if err != nil {
		// A cmap table with one subtable of format 0 for the platform
		// Unicode, which maps each code to glyph 0.
		cmap := make([]byte, 12+262)
		binary.BigEndian.PutUint16(cmap[2:], 1)
		binary.BigEndian.PutUint32(cmap[8:], 12)
		binary.BigEndian.PutUint16(cmap[14:], 262)
		tabs[tagCmap] = cmap
		font, err = goTextFont(tabs)
	}
	if err != nil {
		return nil, fmt.Errorf("reading OpenType font: %w", err)
	}
	f.face = gofont.NewFace(font)
	f.upem = float32(font.Upem())
	if f.upem <= 0 {
		// Port: skrifa divides by a zero units per em. Go uses 1000.
		f.upem = UnitsPerEm
	}
	if raw, ok := tabs[tagCFF]; ok {
		f.cff, _ = newCFFProgram(raw)
	}
	return f, nil
}

// goTextFont builds a font file of the tables and reads it with go-text.
func goTextFont(tabs map[ot.Tag][]byte) (*gofont.Font, error) {
	list := make([]ot.Table, 0, len(tabs))
	for tag, content := range tabs {
		list = append(list, ot.Table{Tag: tag, Content: content})
	}
	slices.SortFunc(list, func(a, b ot.Table) int {
		switch {
		case a.Tag < b.Tag:
			return -1
		case a.Tag > b.Tag:
			return 1
		}
		return 0
	})
	var font *gofont.Font
	err := safely(func() error {
		ld, err := ot.NewLoader(bytes.NewReader(ot.WriteOpentype(list, ot.TrueType)))
		if err != nil {
			return goTextError(err)
		}
		font, err = gofont.NewFont(ld)
		return goTextError(err)
	})
	return font, err
}

// sfntCFFTable returns the CFF table of an OpenType font, or nil.
func sfntCFFTable(data []byte, index uint32) []byte {
	tabs, err := sfntTables(data, index)
	if err != nil {
		return nil
	}
	return tabs[tagCFF]
}

// postVersion returns the version of the post table, and false if the font
// has no such table.
func (f *sfntProgram) postVersion() (uint32, bool) {
	if len(f.post) < 4 {
		return 0, false
	}
	return binary.BigEndian.Uint32(f.post), true
}

// outline draws a glyph into the pen, scaled to UnitsPerEm. typesetting
// draws the glyphs of a CFF table with the FontMatrix of the table and with
// seac, as skrifa does (okage D59).
func (f *sfntProgram) outline(gid GlyphID, pen *outlinePath) {
	if int(gid) >= f.numGlyphs {
		return
	}
	var out gofont.GlyphOutline
	_ = safely(func() error {
		out, _ = f.face.GlyphDataOutline(gofont.GID(gid))
		return nil
	})
	drawSegments(out.Segments, kurbo.AffineScale(float64(UnitsPerEm/f.upem)), pen)
}

// advance returns the advance of a glyph in font units.
func (f *sfntProgram) advance(gid GlyphID) (float32, bool) {
	if int(gid) >= f.numGlyphs {
		return 0, false
	}
	var adv float32
	err := safely(func() error {
		adv = f.face.HorizontalAdvance(gofont.GID(gid))
		return nil
	})
	return adv, err == nil
}

// glyphByName returns the first glyph with the name, from the post table or
// the CFF table.
func (f *sfntProgram) glyphByName(name string) (GlyphID, bool) {
	f.namesOnce.Do(func() {
		f.names = make(map[string]GlyphID)
		_ = safely(func() error {
			for gid := range f.numGlyphs {
				n := f.face.GlyphName(gofont.GID(gid))
				if n == "" {
					continue
				}
				if _, ok := f.names[n]; !ok {
					f.names[n] = GlyphID(gid)
				}
			}
			return nil
		})
	})
	gid, ok := f.names[name]
	return gid, ok
}

// unicodeGlyph returns the glyph of a character through the cmap table that
// go-text picks.
func (f *sfntProgram) unicodeGlyph(r rune) (GlyphID, bool) {
	var gid gofont.GID
	var ok bool
	_ = safely(func() error {
		gid, ok = f.face.NominalGlyph(r)
		return nil
	})
	return GlyphID(gid), ok
}

// weight is the weight of default_attributes of skrifa. It reads the class
// of the weight in the OS/2 table, or else the bold bit of the head table.
func (f *sfntProgram) weight() float32 {
	if len(f.os2) >= 6 {
		return float32(binary.BigEndian.Uint16(f.os2[4:]))
	}
	if len(f.head) >= 46 && binary.BigEndian.Uint16(f.head[44:])&1 != 0 {
		return 700
	}
	return 400
}

// isItalic reports whether the style of default_attributes of skrifa is not
// normal. It reads the italic and oblique bits of the OS/2 table, or else
// the italic bit of the head table.
func (f *sfntProgram) isItalic() bool {
	if len(f.os2) >= 64 {
		sel := binary.BigEndian.Uint16(f.os2[62:])
		return sel&1 != 0 || sel&(1<<9) != 0
	}
	if len(f.os2) == 0 && len(f.head) >= 46 {
		return binary.BigEndian.Uint16(f.head[44:])&2 != 0
	}
	return false
}

// isFixedPitch is is_fixed_pitch of style_metrics of skrifa. It reads the
// post table, and it reports false if the font has none.
func (f *sfntProgram) isFixedPitch() bool {
	return len(f.post) >= 16 && binary.BigEndian.Uint32(f.post[12:]) != 0
}

// cmapMap is map_code of the charmap of skrifa for one subtable. It returns
// the glyph of a code, and false if the subtable does not map the code.
//
// Port: go-text picks one subtable and changes the codes of some subtables.
// This function reads the subtables that go-text parses, without a change.
func cmapMap(sub tables.CmapSubtable, code uint32) (GlyphID, bool) {
	switch s := sub.(type) {
	case tables.CmapSubtable0:
		if code < 256 {
			return GlyphID(s.GlyphIdArray[code]), true
		}
	case tables.CmapSubtable4:
		return cmap4Map(s, code)
	case tables.CmapSubtable6:
		if code >= uint32(s.FirstCode) && code-uint32(s.FirstCode) < uint32(len(s.GlyphIdArray)) {
			return GlyphID(s.GlyphIdArray[code-uint32(s.FirstCode)]), true
		}
	case tables.CmapSubtable10:
		if code >= s.StartCharCode && code-s.StartCharCode < uint32(len(s.GlyphIdArray)) {
			return GlyphID(s.GlyphIdArray[code-s.StartCharCode]), true
		}
	case tables.CmapSubtable12:
		for _, g := range s.Groups {
			if code >= g.StartCharCode && code <= g.EndCharCode {
				return GlyphID(g.StartGlyphID + (code - g.StartCharCode)), true
			}
		}
	case tables.CmapSubtable13:
		for _, g := range s.Groups {
			if code >= g.StartCharCode && code <= g.EndCharCode {
				return GlyphID(g.StartGlyphID), true
			}
		}
	}
	return 0, false
}

// cmap4Map maps a code through a subtable of format 4.
func cmap4Map(s tables.CmapSubtable4, code uint32) (GlyphID, bool) {
	if code > 0xFFFF {
		return 0, false
	}
	c := uint16(code)
	n := min(len(s.EndCode), len(s.StartCode), len(s.IdDelta), len(s.IdRangeOffsets))
	for i := range n {
		if s.EndCode[i] < c {
			continue
		}
		if s.StartCode[i] > c {
			return 0, false
		}
		if s.IdRangeOffsets[i] == 0 {
			return GlyphID(c + s.IdDelta[i]), true
		}
		idx := int(s.IdRangeOffsets[i])/2 + int(c-s.StartCode[i]) + i - n
		if idx < 0 || 2*idx+2 > len(s.GlyphIDArray) {
			return 0, false
		}
		g := binary.BigEndian.Uint16(s.GlyphIDArray[2*idx:])
		if g == 0 {
			return 0, true
		}
		return GlyphID(g + s.IdDelta[i]), true
	}
	return 0, false
}
