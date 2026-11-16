// Copyright (c) 2011-2021, Simon Sapin and contributors.
//
// Use of this source code is governed by the BSD-3-Clause license that can be
// found in the LICENSE file.

// This file ports the tests of tests/test_text.py that split lines without
// the layout, and the tests of text-transform, which read the boxes of
// parse_all instead of the boxes of render_pages. It also tests the line
// breaks with the font weasyprint.otf of the resources of upstream, whose
// glyphs are 1em wide, and the strut and the ratios of the characters.
//
// The fonts are the Go fonts of golang.org/x/image and weasyprint.otf, not
// the fonts of the machine. SANS_FONTS and MONO_FONTS of upstream are Go and
// Go Mono. The widths of upstream for DejaVu Sans give the same breaks with
// Go, so the expected texts are the same.

package layout

import (
	"os"
	"strings"
	"testing"
)

// sansFonts and monoFonts are SANS_FONTS and MONO_FONTS in
// tests/testing_utils.py, with the fonts of the tests.
var (
	sansFonts = tupleOf("Go", "sans-serif")
	monoFonts = tupleOf("Go Mono", "monospace")
)

// textStyle returns an InitialStyle with the fonts of the tests and the
// values of style.
func textStyle(t *testing.T, style map[string]Value) *Style {
	t.Helper()
	fc := testFontConfig(nil)
	data, err := os.ReadFile("testdata/weasyprint.otf")
	if err != nil {
		t.Fatal(err)
	}
	ids := fc.db.LoadFontData(data)
	for _, id := range ids {
		info, _ := fc.db.Face(id)
		info.Families = append(info.Families, info.Families[0])
		info.Families[0].Name = "weasyprint"
	}
	newStyle := newInitialStyle(&StyleContext{FontConfig: fc, Cache: map[any]any{}})
	newStyle.Set("font_family", monoFonts)
	for key, value := range style {
		newStyle.Set(key, value)
	}
	return newStyle
}

// makeText is make_text in tests/test_text.py: split_first_line with a
// style that has the values of style. A negative width is None.
func makeText(t *testing.T, text string, width float64, style map[string]Value) FirstLineMetrics {
	t.Helper()
	var maxWidth *float64
	if width >= 0 {
		maxWidth = &width
	}
	return SplitFirstLine(text, textStyle(t, style), maxWidth, 0, true, false)
}

// TestLineContent is test_line_content.
func TestLineContent(t *testing.T) {
	for _, c := range []struct {
		width     float64
		remaining string
	}{{100, "text for test"}, {45, "is a text for test"}} {
		text := "This is a text for test"
		m := makeText(t, text, c.width, map[string]Value{"font_family": sansFonts, "font_size": Float(19)})
		if !m.HasResume || text[m.ResumeIndex:] != c.remaining {
			t.Errorf("width %v: remaining %q", c.width, text[m.ResumeIndex:])
		}
		if m.Length+1 != m.ResumeIndex { // +1 for the removed trailing space
			t.Errorf("width %v: length %d, resume %d", c.width, m.Length, m.ResumeIndex)
		}
	}
}

// TestLineWithAnyWidth is test_line_with_any_width.
func TestLineWithAnyWidth(t *testing.T) {
	width1 := makeText(t, "some text", -1, nil).Width
	width2 := makeText(t, "some text some text", -1, nil).Width
	if width1 >= width2 {
		t.Errorf("widths %v %v", width1, width2)
	}
}

// TestLineBreaking is test_line_breaking and test_line_breaking_rtl.
func TestLineBreaking(t *testing.T) {
	for _, c := range []struct {
		text      string
		style     map[string]Value
		remaining string
	}{
		// These two tests do not really rely on installed fonts
		{"Thïs is a text for test", map[string]Value{"font_size": Float(1)}, ""},
		{"Thïs is a text for test", map[string]Value{"font_size": Float(100)}, "is a text for test"},
		{"Thïs is a text for test", map[string]Value{"font_family": sansFonts, "font_size": Float(19)}, "text for test"},
		// test_line_breaking_rtl
		{"لوريم ايبسوم دولا", map[string]Value{"font_size": Float(1)}, ""},
		{"لوريم ايبسوم دولا", map[string]Value{"font_size": Float(100)}, "ايبسوم دولا"},
	} {
		m := makeText(t, c.text, map[bool]float64{true: 90, false: 100}[c.remaining != "text for test"], c.style)
		got := ""
		if m.HasResume {
			got = c.text[m.ResumeIndex:]
		}
		if got != c.remaining {
			t.Errorf("%q %v: remaining %q, want %q", c.text, c.style, got, c.remaining)
		}
	}
}

// TestTextDimension is test_text_dimension.
func TestTextDimension(t *testing.T) {
	text := "This is a text for test. This is a test for text.py"
	m1 := makeText(t, text, 200, map[string]Value{"font_size": Float(12)})
	m2 := makeText(t, text, 200, map[string]Value{"font_size": Float(20)})
	if m1.Width*m1.Height >= m2.Width*m2.Height {
		t.Errorf("%v×%v, %v×%v", m1.Width, m1.Height, m2.Width, m2.Height)
	}
}

// weasyprint returns the values of a style with the font weasyprint.otf at
// 16px, and more values.
func weasyprint(more map[string]Value) map[string]Value {
	style := map[string]Value{"font_family": tupleOf("weasyprint"), "font_size": Float(16)}
	for key, value := range more {
		style[key] = value
	}
	return style
}

// TestSplitFirstLine checks the line breaks with the font weasyprint.otf,
// whose glyphs are 1em wide. It follows tests of tests/test_text.py that
// upstream runs with render_pages.
func TestSplitFirstLine(t *testing.T) {
	for _, c := range []struct {
		name        string
		text        string
		width       float64
		style       map[string]Value
		layoutText  string
		length      int
		resume      int
		hasResume   bool
		lineWidth   float64
		lineHeight  float64
		baseline    float64
		firstLetter string
	}{
		{name: "fits", text: "abc def", width: 200, layoutText: "abc def", length: 7, lineWidth: 112},
		{name: "break at space", text: "abc def", width: 100, layoutText: "abc", length: 3, resume: 4, hasResume: true, lineWidth: 48},
		{name: "space hangs", text: "abc def", width: 50, layoutText: "abc", length: 3, resume: 4, hasResume: true, lineWidth: 48},
		// Words that fill the width exactly stay on the line, because the
		// final space of a wrapped line has no width, as in Pango.
		{name: "exact fit", text: "c d e", width: 48, layoutText: "c d", length: 3, resume: 4, hasResume: true, lineWidth: 48},
		// test_line_breaking_nbsp: no break at a no-break space.
		{name: "nbsp", text: "a b c d ef", width: 120, layoutText: "a b c", length: 5, resume: 6, hasResume: true, lineWidth: 80},
		{name: "overflow", text: "abcdef gh", width: 32, layoutText: "abcdef", length: 6, resume: 7, hasResume: true, lineWidth: 96},
		// test_overflow_wrap: break-word breaks inside the word.
		{name: "break-word", text: "abcdef gh", width: 32, style: map[string]Value{"overflow_wrap": String("break-word")},
			layoutText: "ab", length: 2, resume: 2, hasResume: true, lineWidth: 32},
		{name: "break-all", text: "abcdef gh", width: 40, style: map[string]Value{"word_break": String("break-all")},
			layoutText: "ab", length: 2, resume: 2, hasResume: true, lineWidth: 32},
		// test_hyphenate_manual_1: a soft hyphen breaks a word, with the
		// hyphenate character.
		{name: "soft hyphen", text: "hy\u00adphénation", width: 48, layoutText: "hy\u00ad‐", length: 4, resume: 4, hasResume: true, lineWidth: 48},
		// A preserved new line ends the line.
		{name: "pre", text: "ab\ncd", width: 200, style: map[string]Value{"white_space": String("pre")},
			layoutText: "ab", length: 2, resume: 3, hasResume: true, lineWidth: 32},
		{name: "pre-wrap", text: "ab cd\nef", width: 40, style: map[string]Value{"white_space": String("pre-wrap")},
			layoutText: "ab ", length: 3, resume: 3, hasResume: true, lineWidth: 48},
		{name: "nowrap", text: "ab cd ef", width: 40, style: map[string]Value{"white_space": String("nowrap")},
			layoutText: "ab cd ef", length: 8, lineWidth: 128},
		{name: "letter-spacing", text: "ab", width: -1, style: map[string]Value{"letter_spacing": Float(2)},
			layoutText: "ab", length: 2, lineWidth: 36},
		{name: "word-spacing", text: "a b", width: -1, style: map[string]Value{"word_spacing": Float(5)},
			layoutText: "a b", length: 3, lineWidth: 53},
		{name: "tab", text: "a\tb", width: -1, style: map[string]Value{"white_space": String("pre"), "tab_size": Int(4)},
			layoutText: "a\tb", length: 3, lineWidth: 80},
	} {
		t.Run(c.name, func(t *testing.T) {
			m := makeText(t, c.text, c.width, weasyprint(c.style))
			if m.Layout.Text != c.layoutText || m.Length != c.length || m.HasResume != c.hasResume ||
				(c.hasResume && m.ResumeIndex != c.resume) || m.Width != c.lineWidth {
				t.Errorf("got text %q, length %d, resume %d %v, width %v", m.Layout.Text, m.Length, m.ResumeIndex, m.HasResume, m.Width)
			}
			if m.Height != 16 || m.Baseline != 12.796875 {
				t.Errorf("got height %v, baseline %v", m.Height, m.Baseline)
			}
		})
	}
}

// TestGlyphRuns checks the glyphs of a line for the drawing.
func TestGlyphRuns(t *testing.T) {
	style := textStyle(t, weasyprint(map[string]Value{"letter_spacing": Float(2)}))
	layout := CreateLayout("ab c", style, nil, 0)
	line, _, ok := layout.FirstLine()
	if ok {
		t.Fatal("second line")
	}
	runs := line.GlyphRuns()
	if len(runs) != 1 || !runs[0].HasFont || runs[0].Size != 16 {
		t.Fatalf("runs: %+v", runs)
	}
	xs := make([]float64, 0, len(runs[0].Glyphs))
	for _, g := range runs[0].Glyphs {
		xs = append(xs, g.X)
	}
	if want := []float64{0, 18, 36, 54}; !floatsEqual(xs, want) {
		t.Errorf("x: %v, want %v", xs, want)
	}
	if runs[0].Glyphs[3].Cluster != 3 {
		t.Errorf("cluster: %d", runs[0].Glyphs[3].Cluster)
	}
}

// floatsEqual reports whether two lists of numbers are equal.
func floatsEqual(a, b []float64) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// TestStrut checks strut and character_ratio with weasyprint.otf, whose
// ascent is 0.7998em and descent 0.2002em.
func TestStrut(t *testing.T) {
	style := textStyle(t, weasyprint(nil))
	fc := fontConfigOf(style)
	if lineHeight, baseline := fc.Strut(style); lineHeight != 16 || baseline != 12.796875 {
		t.Errorf("normal: %v %v", lineHeight, baseline)
	}
	style = textStyle(t, weasyprint(map[string]Value{"line_height": Float(2)}))
	if lineHeight, baseline := fc.Strut(style); lineHeight != 32 || baseline != 12.796875+8 {
		t.Errorf("2: %v %v", lineHeight, baseline)
	}
	style = textStyle(t, weasyprint(map[string]Value{"line_height": Dimension{20, "px"}}))
	if lineHeight, baseline := fc.Strut(style); lineHeight != 20 || baseline != 12.796875+2 {
		t.Errorf("20px: %v %v", lineHeight, baseline)
	}
	style = textStyle(t, weasyprint(map[string]Value{"font_size": Int(0)}))
	if lineHeight, baseline := fc.Strut(style); lineHeight != 0 || baseline != 0 {
		t.Errorf("font size 0: %v %v", lineHeight, baseline)
	}
	style = textStyle(t, weasyprint(nil))
	for unit, want := range map[string]float64{"ex": 0.7998, "ch": 1, "cap": 1} {
		if got := fc.CharacterRatio(style, unit); got != want {
			t.Errorf("%s: %v", unit, got)
		}
	}
}

// TestNoFonts checks the layout of a text without fonts: each character is
// half the font size, as the default metrics say.
func TestNoFonts(t *testing.T) {
	style := newInitialStyle(&StyleContext{Cache: map[any]any{}})
	style.Set("font_size", Float(10))
	width := 25.0
	m := SplitFirstLine("abc def", style, &width, 0, true, false)
	if m.Layout.Text != "abc" || m.Width != 15 || m.Height != 12 || m.Baseline != 9 {
		t.Errorf("got %q %v %v %v", m.Layout.Text, m.Width, m.Height, m.Baseline)
	}
}

// TestLogAttrs checks can_break_text, get_next_word_boundaries and
// get_last_word_end.
func TestLogAttrs(t *testing.T) {
	for _, c := range []struct {
		text string
		want bool
	}{{"ab", false}, {"a b", true}, {"a b", false}, {"a", false}, {"a-b", true}, {"水水", true}} {
		if got := CanBreakText(c.text, ""); got != c.want {
			t.Errorf("can_break_text(%q): %v", c.text, got)
		}
	}
	if start, end, ok := GetNextWordBoundaries("  hello world", ""); !ok || start != 2 || end != 7 {
		t.Errorf("get_next_word_boundaries: %d %d %v", start, end, ok)
	}
	if end, ok := GetLastWordEnd("hello world!", ""); !ok || end != 11 {
		t.Errorf("get_last_word_end: %d %v", end, ok)
	}
}

// TestTextTransform is test_text_transform. Upstream reads the boxes of
// render_pages, the port the boxes of parse_all.
func TestTextTransform(t *testing.T) {
	html := parseAll(t, `
      <style>
        p { text-transform: capitalize }
        p+p { text-transform: uppercase }
        p+p+p { text-transform: lowercase }
        p+p+p+p { text-transform: full-width }
        p+p+p+p+p { text-transform: none }
      </style>
<p>hé lO1</p><p>hé lO1</p><p>hé lO1</p><p>hé lO1</p><p>hé lO1</p>
    `)
	body := html.Children[0]
	got := make([]string, 0, len(body.Children))
	for _, p := range body.Children {
		got = append(got, p.Children[0].Children[0].Text)
	}
	want := []string{"Hé LO1", "HÉ LO1", "hé lo1", "ｈé　ｌＯ１", "hé lO1"}
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Errorf("got %q", got)
	}
}

// TestTextTransformLang is test_text_transform_lang_uppercase and
// test_text_transform_lang_lowercase.
func TestTextTransformLang(t *testing.T) {
	for _, c := range []struct {
		lang, transform, text, want string
	}{
		{"az", "uppercase", "ı i", "I İ"},
		{"tr", "uppercase", "ı i", "I İ"},
		{"el", "uppercase", "καλημέρα αύριο θεϊκό ευφυΐα Νεράιδα", "ΚΑΛΗΜΕΡΑ ΑΥΡΙΟ ΘΕΪΚΟ ΕΥΦΥΪΑ ΝΕΡΑΪΔΑ"},
		{"az", "lowercase", "İ I", "i ı"},
		{"tr", "lowercase", "İ I", "i ı"},
		{"lt", "lowercase", "Ì Í Ĩ", "i̇̀ i̇́ i̇̃"},
	} {
		html := parseAll(t, `
      <html lang="`+c.lang+`">
        <p style="text-transform: `+c.transform+`">`+c.text)
		p := html.Children[0].Children[0]
		if got := p.Children[0].Children[0].Text; got != c.want {
			t.Errorf("%s %s: %q, want %q", c.lang, c.transform, got, c.want)
		}
	}
}

// TestTextTransformCapitalize is test_text_transform_capitalize.
func TestTextTransformCapitalize(t *testing.T) {
	for _, c := range []struct {
		original, transformed, langCode string
	}{
		{"abc def ghi", "Abc Def Ghi", ""},
		{"AbC def ghi", "AbC Def Ghi", ""},
		{"I’m SO cool", "I’m SO Cool", ""},
		{"Wow.wow!wow", "Wow.wow!wow", ""},
		{"!now not tomorrow", "!Now Not Tomorrow", ""},
		{"SUPER cool", "SUPER Cool", ""},
		{"i 😻 non‑breaking characters", "I 😻 Non‑breaking Characters", ""},
		{"3lite 3lite", "3lite 3lite", ""},
		{"one/two/three", "One/two/three", ""},
		{"supernatural,super", "Supernatural,super", ""},
		{"éternel αιώνια", "Éternel Αιώνια", ""},
		{"great ijland", "Great Ijland", "fr"},
		{"great ijland", "Great IJland", "nl"},
		{"great ijland", "Great İjland", "az"},
	} {
		// Results are different for different browsers, we almost get the
		// same results as Firefox, that’s good enough!
		if got := capitalize(c.original, c.langCode); got != c.transformed {
			t.Errorf("capitalize(%q, %q): %q", c.original, c.langCode, got)
		}
	}
}

// TestTailorSpaceBreaks checks the breaks after a space and before a closing
// punctuation, an infix separator or a slash, which Pango allows and rule
// LB13 of UAX #14 does not.
func TestTailorSpaceBreaks(t *testing.T) {
	for _, c := range []struct {
		text string
		exp  bool
	}{
		{" /", true},
		{" )", true},
		{"a ,", true},
		{"a /b", true},
		{"a)", false},
		{"( )", false},
		{"(  ]", false},
		{"a !", false},
	} {
		if got := CanBreakText(c.text, ""); got != c.exp {
			t.Errorf("CanBreakText(%q): expected %v, got: %v", c.text, c.exp, got)
		}
	}
}
