// Copyright (c) The Hayro Authors.
//
// Use of this source code is governed by the Apache-2.0 or MIT license that
// can be found in the LICENSE-APACHE and LICENSE-MIT files.

// This file holds the tests of the parts of the port that upstream does not
// test: the readers that replace skrifa, the substitutes of the standard
// fonts, and the fonts of a few test files of hayro.

package pdffont

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/xo/ivy/internal/hayro/cmap"
	"github.com/xo/ivy/internal/hayro/syntax"
)

// defaultResolver is the default font resolver of hayro-interpret
// src/interpret/mod.rs: the embedded substitutes of the standard fonts. It
// has no font for a Named query.
func defaultResolver(q *FontQuery) (FontData, uint32, bool) {
	if q.Named {
		return nil, 0, false
	}
	if q.IsStandard {
		data, index := q.Standard.GetFontData()
		return data, index, true
	}
	data, index := q.Fallback.PickStandardFont().GetFontData()
	return data, index, true
}

// cmapResolver resolves the predefined CMaps.
func cmapResolver(name cmap.Name) ([]byte, bool) {
	return cmap.LoadEmbedded(name)
}

// fontDicts returns the font dictionaries of a test file, without the
// descendant CID fonts.
func fontDicts(t testing.TB, name string) []syntax.Dict {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatal(err)
	}
	pdf, err := syntax.NewPdf(data)
	if err != nil {
		t.Fatal(err)
	}
	var out []syntax.Dict
	for obj := range pdf.Objects() {
		d, ok := obj.(syntax.Dict)
		if !ok {
			continue
		}
		if typ, _ := d.Get[syntax.Name](syntax.KeyType); typ != "Font" {
			continue
		}
		switch sub, _ := d.Get[syntax.Name](syntax.KeySubtype); sub {
		case syntax.KeyType1, syntax.KeyMMType1, syntax.KeyTrueType, syntax.KeyType0, syntax.KeyType3:
			out = append(out, d)
		}
	}
	if len(out) == 0 {
		t.Fatalf("%s: no font", name)
	}
	return out
}

// TestStandardFontPrograms reads each substitute of a standard font. The
// files of PDFium end in .pfb, but they hold bare CFF fonts.
func TestStandardFontPrograms(t *testing.T) {
	t.Parallel()
	for sf := Helvetica; sf <= Symbol; sf++ {
		data, _ := sf.GetFontData()
		p, err := newFontProgram(data, 0)
		if err != nil {
			t.Fatalf("%s: %v", sf.PostscriptName(), err)
		}
		if p.programKind() != programCff {
			t.Fatalf("%s: kind %d, want CFF", sf.PostscriptName(), p.programKind())
		}
		if name, _ := p.cff.glyphName(0); name != ".notdef" {
			t.Errorf("%s: glyph 0 is %q", sf.PostscriptName(), name)
		}
		name := "A"
		switch sf {
		case Symbol:
			name = "Alpha"
		case ZapfDingBats:
			name = "a1"
		}
		g, ok := p.glyphByName(name)
		if !ok {
			t.Fatalf("%s: no glyph %s", sf.PostscriptName(), name)
		}
		if p.outline(g).IsEmpty() {
			t.Errorf("%s: empty outline of %s", sf.PostscriptName(), name)
		}
		// Each glyph must draw without a panic.
		for gid := range p.cff.numGlyphs {
			p.outline(GlyphID(gid))
		}
	}
}

// TestStandardFontHelvetica maps text through the standard font Helvetica.
func TestStandardFontHelvetica(t *testing.T) {
	t.Parallel()
	cache := NewFontCache()
	font, ok := cache.StandardFont(Helvetica, defaultResolver)
	if !ok {
		t.Fatal("no Helvetica")
	}
	gid := font.MapCode('A')
	if gid == GlyphIDNotdef {
		t.Fatal("A maps to .notdef")
	}
	if adv := font.CodeAdvance('A'); adv.X != 667 || adv.Y != 0 {
		t.Errorf("advance of A = %v, want 667", adv)
	}
	g, ok := font.Glyph(gid, 'A').(*OutlineGlyph)
	if !ok {
		t.Fatal("glyph of A is not an outline")
	}
	if g.Outline().IsEmpty() {
		t.Error("empty outline of A")
	}
	if u, ok := g.AsUnicode(); !ok || u != cmap.NewBfChar('A') {
		t.Errorf("unicode of A = %v, %v", u, ok)
	}
	// A standard font hides .notdef.
	if notdef := font.Glyph(GlyphIDNotdef, 0).(*OutlineGlyph); !notdef.Outline().IsEmpty() {
		t.Error(".notdef of a standard font draws")
	}
	if _, ok := g.FontData(); ok {
		t.Error("a Type 1 font gives font data")
	}
	if _, ok := g.HintedOutline(12, NewHintingCache()); ok {
		t.Error("a hinted outline without a hinter")
	}
	if id1, id2 := g.Identifier(), font.Glyph(gid, 'A').(*OutlineGlyph).Identifier(); !id1.Equal(id2) || id1.GlyphKey() != id2.GlyphKey() {
		t.Error("two identifiers of one glyph differ")
	}
}

// TestPDFFonts reads the fonts of the test files of hayro, and checks the
// kind of each font program.
func TestPDFFonts(t *testing.T) {
	t.Parallel()
	tests := []struct {
		file string
		kind func(*Font) string
		want string
	}{
		{"font_type1_1.pdf", programKindOf, "type1"},
		{"font_type1_cff_1.pdf", programKindOf, "cff"},
		// A TrueType font that the file does not embed falls back to
		// Courier.
		{"font_truetype_1.pdf", programKindOf, "cff"},
		{"font_truetype_3.pdf", programKindOf, "sfnt"},
		{"font_cid_1.pdf", programKindOf, "cff"},
		{"font_standard_1.pdf", programKindOf, "cff"},
		{"font_type1_opentype_builtin_encoding.pdf", programKindOf, "sfnt"},
		{"font_type3_widths_with_matrix.pdf", programKindOf, "type3"},
	}
	for _, tt := range tests {
		t.Run(tt.file, func(t *testing.T) {
			t.Parallel()
			cache := NewFontCache()
			for _, d := range fontDicts(t, tt.file) {
				font, ok := cache.Font(d, defaultResolver, cmapResolver)
				if !ok {
					t.Fatalf("cannot read font %s", d.Data())
				}
				if got := tt.kind(font); got != tt.want {
					t.Errorf("kind %s, want %s", got, tt.want)
				}
				drawn := 0
				for code := range uint32(256) {
					gid := font.MapCode(code)
					switch g := font.Glyph(gid, code).(type) {
					case *OutlineGlyph:
						if gid != GlyphIDNotdef && !g.Outline().IsEmpty() {
							drawn++
						}
					case *Type3Glyph:
						if _, ok := g.Program(); ok {
							drawn++
						}
					}
					font.CodeAdvance(code)
					font.GlyphStretch(gid, code)
				}
				if drawn == 0 {
					t.Errorf("no code draws a glyph in font %s", d.Data())
				}
			}
		})
	}
}

// programKindOf names the kind of the font program of a font.
func programKindOf(f *Font) string {
	var p *fontProgram
	switch {
	case f.type1 != nil:
		p = f.type1.program()
	case f.trueType != nil:
		p = f.trueType.program()
	case f.type0 != nil:
		p = f.type0.program()
	default:
		return "type3"
	}
	return [...]string{"sfnt", "cff", "type1"}[p.programKind()]
}

// TestWidthsLaterEntryWins tests that a later entry of a W array wins over
// an earlier one, as the BTreeMap of upstream gives.
func TestWidthsLaterEntryWins(t *testing.T) {
	t.Parallel()
	w := newWidths([]widthRange[float32]{
		{first: 0, last: 100, value: 1},
		{first: 10, last: 20, value: 2},
		{first: 15, last: 15, value: 3},
		{first: 90, last: 200, value: 4},
		{first: 300, last: 300, value: 5},
	})
	for _, c := range []struct {
		cid  uint32
		want float32
		ok   bool
	}{
		{0, 1, true}, {9, 1, true}, {10, 2, true}, {14, 2, true}, {15, 3, true},
		{16, 2, true}, {20, 2, true}, {21, 1, true}, {89, 1, true}, {90, 4, true},
		{200, 4, true}, {201, 0, false}, {300, 5, true}, {301, 0, false},
	} {
		got, ok := w.get(c.cid)
		if got != c.want || ok != c.ok {
			t.Errorf("get(%d) = %v, %v, want %v, %v", c.cid, got, ok, c.want, c.ok)
		}
	}
}

// TestSelectStandardFont tests the guess of a standard font from its name.
func TestSelectStandardFont(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		dict  string
		want  StandardFont
		exact bool
	}{
		{"<</BaseFont /Helvetica>>", Helvetica, true},
		{"<</BaseFont /ABCDEF+Times-BoldItalic>>", TimesBoldItalic, true},
		{"<</BaseFont /Arial,Bold>>", HelveticaBold, false},
		{"<</BaseFont /CourierNew-Italic>>", CourierOblique, true},
		{"<</BaseFont /DejaVuSerif>>", TimesRoman, false},
	} {
		d, ok := syntax.FromBytes[syntax.Dict]([]byte(c.dict))
		if !ok {
			t.Fatalf("cannot read %s", c.dict)
		}
		got, exact, ok := selectStandardFont(d, syntax.EmptyDict())
		if !ok || got != c.want || exact != c.exact {
			t.Errorf("%s: %s, %v, %v", c.dict, got.PostscriptName(), exact, ok)
		}
	}
}
