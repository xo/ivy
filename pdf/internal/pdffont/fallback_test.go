// This file ports no upstream file. It tests the fonts that a PDF file does
// not embed, which Go resolves by their names and by their CJK collections
// where upstream picks a standard font.

package pdffont

import (
	"os"
	"sync"
	"testing"

	"golang.org/x/image/font/gofont/goregular"

	"github.com/xo/ivy/pdf/internal/syntax"
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
	// あ is in Japan1, and 偞 is not. A code of UniJIS-UCS2-H is the
	// character.
	for _, code := range []uint32{0x3042, 0x6F22, 0x505E} {
		if gid, ok := outline(code); !ok || gid&latinGlyph != 0 {
			t.Errorf("%#x: expected a glyph of Droid Sans Fallback, got %#x", code, gid)
		}
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
