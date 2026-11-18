package metafile

import (
	"testing"
	"unicode/utf16"

	"github.com/xo/ivy/internal/gowemf"
)

// shapeFont returns Liberation Sans at an em of 100 units.
func shapeFont(t *testing.T) textFont {
	t.Helper()
	b, _ := testBackend(t)
	b.pic.fonts = newFontSet(nil, officeFonts(t))
	tf, ok := b.font(gowemf.FontRequest{FaceName: "Liberation Sans", Height: -100})
	if !ok || tf.family != "Liberation Sans" {
		t.Fatal("no Liberation Sans")
	}
	return tf
}

func TestNeedsShaping(t *testing.T) {
	for s, want := range map[string]bool{
		"Hello, world":  false,
		"Ελληνικά":      false,
		"日本語":           false,
		"e\u0301":       true,
		"שלום":          true,
		"مرحبا":         true,
		"नमस्ते":        true,
		"ภาษาไทย":       true,
		"plain \u00e9t": false,
	} {
		if got := needsShaping([]rune(s)); got != want {
			t.Errorf("%q: %v, want %v", s, got, want)
		}
	}
}

// TestShapeMark checks that a combining mark joins the cluster of its base
// character, with no advance of its own.
func TestShapeMark(t *testing.T) {
	tf := shapeFont(t)
	run := gowemf.TextRun{Text: utf16.Encode([]rune("xe\u0301x"))}
	shaped, ok := tf.shape(run)
	if !ok {
		t.Fatal("not shaped")
	}
	if shaped.advances[2] != 0 || len(shaped.glyphs[2]) != 0 {
		t.Errorf("the mark has an advance %g and %d glyphs", shaped.advances[2], len(shaped.glyphs[2]))
	}
	if shaped.advances[1] <= 0 || len(shaped.glyphs[1]) == 0 {
		t.Errorf("the base has an advance %g and %d glyphs", shaped.advances[1], len(shaped.glyphs[1]))
	}
	gid, _ := tf.face.Glyph('x')
	if len(shaped.glyphs[0]) != 1 || shaped.glyphs[0][0].ID != gid || shaped.glyphs[0][0].X != 0 {
		t.Errorf("x: %+v", shaped.glyphs[0])
	}
	// the measure of the backend takes the shaped advances
	b, _ := testBackend(t)
	b.pic.fonts = newFontSet(nil, officeFonts(t))
	run.Font = gowemf.FontRequest{FaceName: "Liberation Sans", Height: -100}
	m, err := b.MeasureText(run)
	if err != nil {
		t.Fatal(err)
	}
	if m.Advances[2] != 0 || m.Advances[1] != shaped.advances[1] {
		t.Errorf("advances %v, want %v", m.Advances, shaped.advances)
	}
}

// TestShapeMixed checks that the Latin text of a run with Hebrew keeps a
// glyph for each character, as GDI draws it.
func TestShapeMixed(t *testing.T) {
	tf := shapeFont(t)
	text := []rune("fi של")
	run := gowemf.TextRun{Text: utf16.Encode(text), Levels: []uint8{0, 0, 0, 1, 1}}
	shaped, ok := tf.shape(run)
	if !ok {
		t.Fatal("not shaped")
	}
	for i, r := range text {
		gid, _ := tf.face.Glyph(r)
		if len(shaped.glyphs[i]) != 1 || shaped.glyphs[i][0].ID != gid {
			t.Errorf("element %d: %+v, want the glyph %d", i, shaped.glyphs[i], gid)
		}
	}
}

// TestShapeRightToLeft checks a run of Hebrew at level 1 with a point: the
// point goes with its letter, and the glyphs of each letter are at its
// origin.
func TestShapeRightToLeft(t *testing.T) {
	tf := shapeFont(t)
	text := []rune("\u05e9\u05b8\u05dc")
	run := gowemf.TextRun{Text: utf16.Encode(text), Levels: []uint8{1, 1, 1}}
	shaped, ok := tf.shape(run)
	if !ok {
		t.Fatal("not shaped")
	}
	if shaped.advances[1] != 0 || len(shaped.glyphs[1]) != 0 {
		t.Errorf("the point has an advance %g and %d glyphs", shaped.advances[1], len(shaped.glyphs[1]))
	}
	if len(shaped.glyphs[0]) != 2 || len(shaped.glyphs[2]) != 1 {
		t.Fatalf("glyphs %+v", shaped.glyphs)
	}
	for _, i := range []int{0, 2} {
		gid, _ := tf.face.Glyph(text[i])
		found := false
		for _, g := range shaped.glyphs[i] {
			found = found || (g.ID == gid && g.X == 0)
		}
		if !found {
			t.Errorf("element %d: no glyph %d at the origin in %+v", i, gid, shaped.glyphs[i])
		}
	}
}
