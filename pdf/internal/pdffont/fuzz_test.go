// Copyright (c) The Hayro Authors.
//
// Use of this source code is governed by the Apache-2.0 or MIT license that
// can be found in the LICENSE-APACHE and LICENSE-MIT files.

// This file holds the fuzz tests of the readers of font programs. Upstream
// has no such test.

package pdffont

import (
	"errors"
	"testing"

	"github.com/xo/ivy"
	"github.com/xo/ivy/pdf/internal/syntax"
)

// embeddedPrograms returns the decoded font files of a test file.
func embeddedPrograms(f *testing.F, name string) [][]byte {
	f.Helper()
	var out [][]byte
	for _, d := range fontDicts(f, name) {
		if sub, _ := d.Get[syntax.Name](syntax.KeySubtype); sub == syntax.KeyType0 {
			descendants, _ := d.Get[syntax.Array](syntax.KeyDescendantFonts)
			if dd, ok := descendants.Iter[syntax.Dict]().Next(); ok {
				d = dd
			}
		}
		desc, _ := d.Get[syntax.Dict](syntax.KeyFontDescriptor)
		for _, key := range []syntax.Name{syntax.KeyFontFile, syntax.KeyFontFile2, syntax.KeyFontFile3} {
			if s, ok := desc.Get[syntax.Stream](key); ok {
				if data, err := s.DecodedData(); err == nil {
					out = append(out, data)
				}
			}
		}
	}
	return out
}

// checkFontError fails if err does not wrap a sentinel error of ivy.
func checkFontError(t *testing.T, err error) {
	t.Helper()
	if !errors.Is(err, ivy.ErrFormat) && !errors.Is(err, ivy.ErrLimit) {
		t.Fatalf("error %v wraps no sentinel error of ivy", err)
	}
}

// FuzzType1Program reads the input as a Type 1 font program and draws its
// glyphs. The reader must not panic or hang, and each error must wrap a
// sentinel error of ivy.
func FuzzType1Program(f *testing.F) {
	glyphs, order := testGlyphs()
	for _, form := range []string{"binary", "hex", "pfb"} {
		f.Add(type1Fixture(fixtureSubrs(), glyphs, order, form))
	}
	for _, data := range embeddedPrograms(f, "font_type1_1.pdf") {
		f.Add(data)
	}
	f.Add([]byte("%!PS-AdobeFont-1.0\n/Encoding StandardEncoding def\ncurrentfile eexec\n"))
	f.Add([]byte{0x80, 0x01, 0x05, 0, 0, 0, '%', '!', 'P', 'S', '\n', 0x80, 0x03})

	f.Fuzz(func(t *testing.T, data []byte) {
		p, err := newType1Program(data)
		if err != nil {
			checkFontError(t, err)
			return
		}
		for gid := range min(len(p.glyphs), 64) {
			pen := newOutlinePath()
			p.outline(GlyphID(gid), pen)
		}
		for code := range 256 {
			p.encodingGlyph(uint8(code))
		}
	})
}

// FuzzFontProgram reads the input as any font program: TrueType, OpenType,
// CFF or Type 1. It checks the readers of go-text through the guard of this
// package.
func FuzzFontProgram(f *testing.F) {
	for _, name := range []string{"font_type1_cff_1.pdf", "font_truetype_3.pdf", "font_type1_opentype_builtin_encoding.pdf"} {
		for _, data := range embeddedPrograms(f, name) {
			f.Add(data)
		}
	}
	data, _ := Helvetica.GetFontData()
	f.Add(data)

	f.Fuzz(func(t *testing.T, data []byte) {
		p, err := newFontProgram(data, 0)
		if err != nil {
			checkFontError(t, err)
			return
		}
		for gid := range GlyphID(64) {
			p.outline(gid)
			p.advanceWidth(gid)
		}
		for code := range 256 {
			p.builtinEncodingGlyph(uint8(code))
			p.builtinEncodingGlyphName(uint8(code))
			p.cmapGlyph(rune(code))
		}
		for _, sub := range p.cmapSubtables() {
			sub.mapCode(65)
		}
		p.glyphByName("A")
		p.glyphByCID(1)
		p.weight()
	})
}
