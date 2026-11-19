package layout

import (
	"fmt"
	"os"
	"slices"
	"strings"
	"testing"
)

// shapeSliceStyle returns a style of the fonts of the tests with Liberation
// Sans, Liberation Mono and Droid Sans Fallback of the module fonts, and
// the values of style.
func shapeSliceStyle(t *testing.T, fc *FontConfiguration, style map[string]Value) *Style {
	t.Helper()
	for _, name := range []string{
		"liberation/LiberationSans-Regular.ttf", "liberation/LiberationMono-Regular.ttf",
		"droid/DroidSansFallback.ttf",
	} {
		data, err := os.ReadFile("../../fonts/" + name)
		if err != nil {
			t.Fatal(err)
		}
		fc.db.LoadFontData(data)
	}
	s := newInitialStyle(&StyleContext{FontConfig: fc, Cache: map[any]any{}})
	s.Set("font_family", tupleOf("Liberation Sans", "Droid Sans Fallback"))
	for key, value := range style {
		s.Set(key, value)
	}
	return s
}

// TestShapeSlice shapes texts as parents, and each start and each end of
// them on its own. When a part takes the glyphs of the parent, they must be
// the glyphs that the shaper gives for the part.
func TestShapeSlice(t *testing.T) {
	texts := []string{
		"AVATAR WAVE Type To Yo “quoted” office fit fluffy, Tolkien’s LTA. P.J. Wavy",
		"The quick brown fox jumps over the lazy dog. AV AV AV TT Te Ty Vo We",
		"第一回　甄士隱夢幻識通靈\"賈雨村\"風塵怀閨秀，作者自云：因曾歷過一番夢幻之后",
		"abc אבג דהו def (ghi) 123 שלום, עולם! xyz",
		"Mixed, punctuation... — and   spaces; with: colons? “Yes!” said he.",
		"été café näive ﬁ ﬂ x̣̂ Ω≈ç√",
	}
	styles := []map[string]Value{
		nil,
		{"font_kerning": String("none")},
		{"direction": String("rtl")},
		{"font_family": tupleOf("Liberation Mono", "Droid Sans Fallback")},
		{"font_variant_ligatures": String("none")},
	}
	taken, cuts := 0, 0
	for i, values := range styles {
		fc := testFontConfig(nil)
		style := shapeSliceStyle(t, fc, values)
		rtl := isStr(style.Get("direction"), "rtl")
		key := parentKey{fontStyleCacheKey(style, false), ValueString(style.Get("font_kerning")), rtl}
		for _, text := range texts {
			fc.shape(text, style)
			p := fc.parents[key]
			if p == nil || p.text != text {
				t.Errorf("style %d: %q is not a parent", i, text)
				continue
			}
			for b := range text {
				if b == 0 {
					continue
				}
				cuts++
				for _, part := range []string{text[:b], text[b:]} {
					got := fc.shapeSlice(key, part, style)
					if got == nil {
						continue
					}
					taken++
					want, _ := fc.shapeText(part, style, rtl)
					if why := diffShaped(got, want); why != "" {
						t.Errorf("style %d: %q of %q: %s", i, part, text, why)
					}
				}
			}
		}
	}
	// Most cuts are safe, at the start and at the end of a text.
	if taken < cuts {
		t.Errorf("expected at least %d slices, got: %d", cuts, taken)
	}
}

// TestShapeSliceKerning checks that a part does not take the glyphs of a
// parent when it cuts a pair that the font kerns.
func TestShapeSliceKerning(t *testing.T) {
	fc := testFontConfig(nil)
	style := shapeSliceStyle(t, fc, nil)
	key := parentKey{fontStyleCacheKey(style, false), ValueString(style.Get("font_kerning")), false}
	text := strings.Repeat("AV", 20)
	fc.shape(text, style)
	if fc.parents[key] == nil {
		t.Fatalf("%q is not a parent", text)
	}
	if s := fc.shapeSlice(key, text[:3], style); s != nil {
		t.Errorf("AVA takes the glyphs of the parent")
	}
	if s := fc.shapeSlice(key, text[:4], style); s != nil {
		t.Errorf("AVAV takes the glyphs of the parent")
	}
	style = shapeSliceStyle(t, testFontConfig(nil), map[string]Value{"font_kerning": String("none")})
	fc = fontConfigOf(style)
	key.kerning = ValueString(style.Get("font_kerning"))
	fc.shape(text, style)
	if s := fc.shapeSlice(key, text[:3], style); s == nil {
		t.Errorf("AVA without kerning does not take the glyphs of the parent")
	}
}

// TestShapeSliceScript checks that a part of only common characters does
// not take the glyphs of the parent, since the segmenter gives the part
// another script.
func TestShapeSliceScript(t *testing.T) {
	fc := testFontConfig(nil)
	style := shapeSliceStyle(t, fc, nil)
	key := parentKey{fontStyleCacheKey(style, false), ValueString(style.Get("font_kerning")), false}
	text := "abcdefghijklmnopqrstuvwxyz, 1234567890!"
	fc.shape(text, style)
	part := text[strings.IndexByte(text, ','):]
	if _, ok := fc.parents[key].cut(len(text) - len(part)); !ok {
		t.Fatalf("the cut before %q is not safe", part)
	}
	if s := fc.shapeSlice(key, part, style); s != nil {
		t.Errorf("%q takes the glyphs of the parent", part)
	}
}

// diffShaped returns how two shaped texts differ, or "" when they are the
// same. It does not compare the flags of the clusters, since the flags of a
// part depend on the parent.
func diffShaped(a, b *shapedText) string {
	if a.ascent != b.ascent || a.descent != b.descent {
		return "the metrics differ"
	}
	if len(a.clusters) != len(b.clusters) {
		return fmt.Sprintf("%d clusters, not %d", len(a.clusters), len(b.clusters))
	}
	for i := range a.clusters {
		x, y := a.clusters[i], b.clusters[i]
		x.unsafe, y.unsafe = false, false
		if x != y {
			return fmt.Sprintf("cluster %d is %+v, not %+v", i, x, y)
		}
	}
	if len(a.runs) != len(b.runs) {
		return fmt.Sprintf("%d runs, not %d", len(a.runs), len(b.runs))
	}
	for i := range a.runs {
		x, y := a.runs[i], b.runs[i]
		if x.id != y.id || x.hasID != y.hasID || x.ascent != y.ascent || x.descent != y.descent ||
			!slices.Equal(x.glyphs, y.glyphs) {
			return fmt.Sprintf("run %d differs", i)
		}
	}
	return ""
}
