// This file ports no upstream file. It tests the fonts that a PDF file does
// not embed, which Go resolves by their names and by their CJK collections
// where upstream picks a standard font.

package pdffont

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"os"
	"sync"
	"testing"

	"golang.org/x/image/font/gofont/goregular"

	"github.com/xo/ivy/internal/hayro/syntax"
)

// dict reads a font dictionary.
func dict(t *testing.T, s string) syntax.Dict {
	t.Helper()
	d, ok := syntax.FromBytes[syntax.Dict]([]byte(s))
	if !ok {
		t.Fatalf("cannot read %s", s)
	}
	return d
}

// queryLog is a font resolver that gives Go Regular for a Named query of
// one name, and the substitutes of the standard fonts else. It keeps the
// queries.
type queryLog struct {
	name string

	mu      sync.Mutex
	queries []FontQuery
}

func (l *queryLog) resolve(q *FontQuery) (FontData, uint32, bool) {
	l.mu.Lock()
	l.queries = append(l.queries, *q)
	l.mu.Unlock()
	if q.Named {
		if q.Fallback.PostScriptName == l.name {
			return goregular.TTF, 0, true
		}
		return nil, 0, false
	}
	return defaultResolver(q)
}

// named returns the names of the Named queries.
func (l *queryLog) named() []string {
	l.mu.Lock()
	defer l.mu.Unlock()
	var out []string
	for _, q := range l.queries {
		if q.Named {
			out = append(out, q.Fallback.PostScriptName)
		}
	}
	return out
}

// TestNamedFont checks that a font that the file does not embed draws with
// the font that the resolver gives for its name, fitted to its widths, and
// that a standard font keeps its substitute.
func TestNamedFont(t *testing.T) {
	t.Parallel()
	widths := "/FirstChar 65 /LastChar 65 /Widths [500] /Encoding /WinAnsiEncoding"
	for _, c := range []struct {
		dict  string
		named []string
		kind  string
	}{
		{"<</Type /Font /Subtype /TrueType /BaseFont /ArialMT " + widths + ">>", []string{"ArialMT"}, "sfnt"},
		{"<</Type /Font /Subtype /Type1 /BaseFont /ABCDEF+ArialMT " + widths + ">>", []string{"ArialMT"}, "sfnt"},
		{"<</Type /Font /Subtype /TrueType /BaseFont /Calibri " + widths + ">>", []string{"Calibri"}, "cff"},
		{"<</Type /Font /Subtype /Type1 /BaseFont /Helvetica>>", nil, "cff"},
		{"<</Type /Font /Subtype /Type1 /BaseFont /Symbol>>", nil, "cff"},
	} {
		log := &queryLog{name: "ArialMT"}
		font, ok := NewFontCache().Font(dict(t, c.dict), log.resolve, cmapResolver)
		if !ok {
			t.Fatalf("%s: cannot read the font", c.dict)
		}
		if got := log.named(); len(got) != len(c.named) || len(got) > 0 && got[0] != c.named[0] {
			t.Errorf("%s: Named queries %q, expected %q", c.dict, got, c.named)
		}
		if got := programKindOf(font); got != c.kind {
			t.Errorf("%s: program %s, expected %s", c.dict, got, c.kind)
		}
		if c.kind != "sfnt" {
			continue
		}
		gid := font.MapCode('A')
		g, ok := font.Glyph(gid, 'A').(*OutlineGlyph)
		if gid == GlyphIDNotdef || !ok || g.Outline().IsEmpty() {
			t.Errorf("%s: expected the glyph A of Go Regular, got %d", c.dict, gid)
		}
		// The A of Go Regular is wider than the width of the file, and the
		// glyph fits that width.
		if s := font.GlyphStretch(gid, 'A'); s >= 1 || s <= 0.5 {
			t.Errorf("%s: expected a stretch of A below 1, got %g", c.dict, s)
		}
	}
}

// droid returns Droid Sans Fallback of the module github.com/xo/ivy/fonts,
// from its folder in the repository. It skips the test when the folder is
// not there.
func droid(t *testing.T) []byte {
	t.Helper()
	data, err := os.ReadFile("../../../fonts/droid/DroidSansFallback.ttf")
	if err != nil {
		t.Skipf("no fonts module: %v", err)
	}
	return data
}

// cjkFont is a Type 0 font of the collection Japan1 that the file does not
// embed, with a Unicode encoding.
const cjkFont = `<</Type /Font /Subtype /Type0 /BaseFont /KozMinPr6N-Regular /Encoding /UniJIS-UCS2-H
	/DescendantFonts [<</Type /Font /Subtype /CIDFontType0 /BaseFont /KozMinPr6N-Regular
	/CIDSystemInfo <</Registry (Adobe) /Ordering (Japan1) /Supplement 6>> /DW 1000 /W [1 95 500]>>]>>`

// TestCJKFallback checks that the text of a CJK font that the file does not
// embed draws with the font of its collection, and its Latin letters with
// the substitute of a standard font.
func TestCJKFallback(t *testing.T) {
	t.Parallel()
	data := droid(t)
	var queries []FontQuery
	resolve := func(q *FontQuery) (FontData, uint32, bool) {
		queries = append(queries, *q)
		if cc := q.Fallback.CharacterCollection; !q.IsStandard && !q.Named && isCJK(cc) {
			return data, 0, true
		}
		return defaultResolver(q)
	}
	font, ok := NewFontCache().Font(dict(t, cjkFont), resolve, cmapResolver)
	if !ok {
		t.Fatal("cannot read the font")
	}
	if got := programKindOf(font); got != "sfnt" {
		t.Errorf("expected the program of Droid Sans Fallback, got %s", got)
	}
	if len(queries) == 0 || !queries[0].Named || queries[0].Fallback.PostScriptName != "KozMinPr6N-Regular" {
		t.Errorf("expected a Named query first, got %+v", queries)
	}
	outline := func(code uint32) (GlyphID, bool) {
		gid := font.MapCode(code)
		g, ok := font.Glyph(gid, code).(*OutlineGlyph)
		return gid, gid != GlyphIDNotdef && ok && !g.Outline().IsEmpty()
	}
	// あ and 漢 are in Japan1. A code of UniJIS-UCS2-H is the character.
	for _, code := range []uint32{0x3042, 0x6F22} {
		if gid, ok := outline(code); !ok || gid&latinGlyph != 0 {
			t.Errorf("%#x: expected a glyph of Droid Sans Fallback, got %#x", code, gid)
		}
	}
	// 偞 is not in Japan1, so UniJIS-UCS2-H does not map it. The font draws
	// nothing for it and does not advance, as MuPDF does.
	if gid := font.MapCode(0x505E); gid != GlyphIDNotdef {
		t.Errorf("0x505e: expected .notdef, got %#x", gid)
	}
	if g := font.Glyph(GlyphIDNotdef, 0x505E).(*OutlineGlyph); !g.Outline().IsEmpty() {
		t.Error("0x505e: expected no outline")
	}
	if adv := font.CodeAdvance(0x505E); adv.X != 0 || adv.Y != 0 {
		t.Errorf("0x505e: expected no advance, got %v", adv)
	}
	if adv := font.CodeAdvance(0x3042); adv.X != 1000 {
		t.Errorf("0x3042: expected the advance DW, got %v", adv)
	}
	gid, ok := outline('A')
	if !ok || gid&latinGlyph == 0 {
		t.Errorf("A: expected a glyph of the latin font, got %#x", gid)
	}
	if s := font.GlyphStretch(gid, 'A'); s <= 0 || s > 1 {
		t.Errorf("A: expected a stretch to the width 500, got %g", s)
	}
	// A code that the CMap does not map reads as two bytes, by the
	// codespace of the CMap.
	if code, n := font.ReadCode([]byte{0x50, 0x5E, 0x30, 0x42}, 0); code != 0x505E || n != 2 {
		t.Errorf("expected the code 0x505e of 2 bytes, got %#x of %d", code, n)
	}
	// Without a CJK font, the CJK characters draw nothing, and the Latin
	// letters draw with the substitute of a standard font.
	font, ok = NewFontCache().Font(dict(t, cjkFont), defaultResolver, cmapResolver)
	if !ok {
		t.Fatal("cannot read the font")
	}
	if gid := font.MapCode(0x3042); gid != GlyphIDNotdef {
		t.Errorf("expected .notdef for あ, got %#x", gid)
	}
	if _, ok := outline('A'); !ok {
		t.Error("expected a glyph of the standard font for A")
	}
}

// symbolicGoRegular returns Go Regular with its cmap subtables of the
// platform Windows marked as (3,0), the subtable of a symbolic font, and a
// post table of version 3, without the names of the glyphs.
func symbolicGoRegular(t *testing.T) []byte {
	t.Helper()
	data := bytes.Clone(goregular.TTF)
	// offset returns the offset of the table tag in the table directory.
	offset := func(tag string) int {
		for i := range int(binary.BigEndian.Uint16(data[4:])) {
			rec := data[12+16*i:]
			if string(rec[:4]) == tag {
				return int(binary.BigEndian.Uint32(rec[8:]))
			}
		}
		t.Fatalf("Go Regular has no table %s", tag)
		return 0
	}
	cmapAt := offset("cmap")
	marked := 0
	for i := range int(binary.BigEndian.Uint16(data[cmapAt+2:])) {
		rec := cmapAt + 4 + 8*i
		if binary.BigEndian.Uint16(data[rec:]) == 3 {
			binary.BigEndian.PutUint16(data[rec+2:], 0)
			marked++
		}
	}
	if marked == 0 {
		t.Fatal("Go Regular has no cmap subtable of the platform Windows")
	}
	binary.BigEndian.PutUint32(data[offset("post"):], 0x00030000)
	return data
}

// TestSymbolicNamedTrueType checks that a symbolic TrueType font that the
// file names and does not embed maps its codes through the (3,0) cmap
// subtable of the font that the resolver gives, and not through the names
// of the glyphs.
func TestSymbolicNamedTrueType(t *testing.T) {
	t.Parallel()
	symbolic := symbolicGoRegular(t)
	plain, err := newSfntProgram(goregular.TTF, 0)
	if err != nil {
		t.Fatal(err)
	}
	resolve := func(q *FontQuery) (FontData, uint32, bool) {
		if q.Named && q.Fallback.PostScriptName == "Wingdings" {
			return symbolic, 0, true
		}
		return defaultResolver(q)
	}
	const font = `<</Type /Font /Subtype /TrueType /BaseFont /Wingdings
		/FirstChar 65 /LastChar 160 /Widths [500]
		/FontDescriptor <</Type /FontDescriptor /FontName /Wingdings /Flags %d>>>>`
	for _, c := range []struct {
		flags  int
		mapped bool
	}{
		// symbolic
		{4, true},
		// nonsymbolic
		{32, false},
	} {
		f, ok := NewFontCache().Font(dict(t, fmt.Sprintf(font, c.flags)), resolve, cmapResolver)
		if !ok {
			t.Fatal("cannot read the font")
		}
		// StandardEncoding has no name for the code 0xA0, and the cmap of
		// Go Regular maps the no-break space U+00A0.
		for _, r := range []rune{'A', 0xA0} {
			exp, ok := plain.unicodeGlyph(r)
			if !ok {
				t.Fatalf("Go Regular has no glyph for %U", r)
			}
			got := f.MapCode(uint32(r))
			if c.mapped && got != exp {
				t.Errorf("flags %d: %U: expected the glyph %d of the (3,0) subtable, got %d", c.flags, r, exp, got)
			}
			if !c.mapped && r == 0xA0 && got != GlyphIDNotdef {
				t.Errorf("flags %d: %U: expected .notdef, got %d", c.flags, r, got)
			}
		}
	}
}
