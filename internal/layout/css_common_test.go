// Copyright (c) 2011-2021, Simon Sapin and contributors.
//
// Use of this source code is governed by the BSD-3-Clause license that can be
// found in the LICENSE file.

// This file ports tests/css/test_common.py: the parsing of CSS, the
// cascade, and the inherited and computed values.
//
// Upstream renders the pages of some tests and reads the boxes. Those tests
// here read the computed styles of the elements instead, because the boxes
// wait for a later stage of the port. Such a test says so in its comment.

package layout

import (
	"log/slog"
	"math"
	"os"
	"strings"
	"testing"
)

// assertRGBA fails the test when a value is not a color with the channels
// of sRGB.
func assertRGBA(t *testing.T, name string, v Value, r, g, b, a float64) {
	t.Helper()
	c, ok := v.(Color)
	if !ok || c.Color == nil {
		t.Errorf("%s: got %s, want a color", name, ValueString(v))
		return
	}
	srgb, ok := c.To("srgb")
	if !ok {
		t.Errorf("%s: cannot convert %s to srgb", name, ValueString(v))
		return
	}
	want := []float64{r, g, b}
	for i, w := range want {
		if math.Abs(srgb.Coordinates[i]-w) > 1e-6 {
			t.Errorf("%s: got %s, want (%v, %v, %v, %v)", name, ValueString(v), r, g, b, a)
			return
		}
	}
	if math.Abs(srgb.Alpha-a) > 1e-6 {
		t.Errorf("%s: got alpha %v, want %v", name, srgb.Alpha, a)
	}
}

// readTestdata returns the text of a file of testdata.
func readTestdata(t *testing.T, name string) string {
	t.Helper()
	data, err := os.ReadFile("testdata/" + name)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

// TestFindStylesheets is test_find_stylesheets. Upstream also counts the
// rules indexed by local name in the matchers, which the matcher of okage
// does not show. The port leaves that count out.
func TestFindStylesheets(t *testing.T) {
	log, capture := captureLogs(slog.LevelInfo)
	html := fakeHTMLFile(t, "doc1.html", log)
	ctx := &cssContext{
		log:           log,
		fetcher:       html.URLFetcher,
		counterStyle:  CounterStyle{},
		colorProfiles: map[string]*ColorProfile{},
		budget:        newCascadeBudget(html.Limits),
	}
	sheets := findStylesheets(ctx, html.WrapperElement, "print", html.BaseURL)
	if len(sheets) != 2 {
		t.Fatalf("expected 2 sheets, got: %d", len(sheets))
	}
	// Also test that stylesheets are in tree order.
	names := make([]string, 0, len(sheets))
	for _, sheet := range sheets {
		name := sheet.BaseURL[strings.LastIndex(sheet.BaseURL, "/")+1:]
		name = name[strings.LastIndex(name, ",")+1:]
		names = append(names, name)
	}
	if want := []string{"a%7Bcolor%3AcurrentColor%7D", "doc1.html"}; strings.Join(names, " ") != strings.Join(want, " ") {
		t.Errorf("expected %q, got: %q", want, names)
	}
	if len(*sheets[1].PageRules) != 1 {
		t.Errorf("expected 1 page rule, got: %d", len(*sheets[1].PageRules))
	}
	assertNoLogs(t, capture)
}

// TestAnnotateDocument is test_annotate_document.
func TestAnnotateDocument(t *testing.T) {
	log, capture := captureLogs(slog.LevelInfo)
	document := fakeHTMLFile(t, "doc1.html", log)
	styleFor := computedStyles(t, document, ComputedStylesOptions{
		UserAgentStylesheets: []CSSSource{{Text: readTestdata(t, "mini_ua.css"), BaseURL: pathToURL("mini_ua.css")}},
		UserStylesheets:      []CSSSource{{Text: readTestdata(t, "user.css"), BaseURL: pathToURL("user.css")}},
	})

	// Element objects behave as lists of their children.
	body := document.EtreeElement.Children[1]
	h1, p, ul, div := body.Children[0], body.Children[1], body.Children[2], body.Children[3]
	li0 := ul.Children[0]
	a := li0.Children[0]
	span1 := div.Children[0]
	span2 := span1.Children[0]

	h1Style := styleFor.Get(h1, "")
	pStyle := styleFor.Get(p, "")
	ulStyle := styleFor.Get(ul, "")
	li0Style := styleFor.Get(li0, "")
	divStyle := styleFor.Get(div, "")
	after := styleFor.Get(a, "after")
	aStyle := styleFor.Get(a, "")
	span1Style := styleFor.Get(span1, "")
	span2Style := styleFor.Get(span2, "")

	assertValue(t, h1Style, "background_image", Tuple{Tuple{String("url"), String(pathToURL("logo_small.png"))}})

	assertValue(t, h1Style, "font_weight", Int(700))
	assertValue(t, h1Style, "font_size", Int(40)) // 2em

	// x-large * initial = 3/2 * 16 = 24
	assertValue(t, pStyle, "margin_top", Dimension{24, "px"})
	assertValue(t, pStyle, "margin_right", Dimension{0, "px"})
	assertValue(t, pStyle, "margin_bottom", Dimension{24, "px"})
	assertValue(t, pStyle, "margin_left", Dimension{0, "px"})
	assertValue(t, pStyle, "background_color", String("currentcolor"))

	// The margins of ul use ex, which upstream does not test either.

	assertValue(t, ulStyle, "font_weight", Int(400))
	// thick = 5px, 0.25 inches = 96*.25 = 24px
	assertValue(t, ulStyle, "border_top_width", Int(0))
	assertValue(t, ulStyle, "border_right_width", Int(5))
	assertValue(t, ulStyle, "border_bottom_width", Int(0))
	assertValue(t, ulStyle, "border_left_width", Int(24))

	assertValue(t, li0Style, "font_weight", Int(700))
	assertValue(t, li0Style, "font_size", Int(8)) // 6pt
	assertValue(t, li0Style, "margin_top", Dimension{16, "px"})
	assertValue(t, li0Style, "margin_right", Dimension{0, "px"})
	assertValue(t, li0Style, "margin_bottom", Dimension{16, "px"})
	assertValue(t, li0Style, "margin_left", Dimension{32, "px"}) // 4em

	assertValue(t, aStyle, "text_decoration_line", newStringSet("underline"))
	assertValue(t, aStyle, "font_weight", Int(900))
	assertValue(t, aStyle, "font_size", Int(24)) // 300% of 8px
	assertValue(t, aStyle, "padding_top", Dimension{1, "px"})
	assertValue(t, aStyle, "padding_right", Dimension{2, "px"})
	assertValue(t, aStyle, "padding_bottom", Dimension{3, "px"})
	assertValue(t, aStyle, "padding_left", Dimension{4, "px"})
	assertValue(t, aStyle, "border_top_width", Int(42))
	assertValue(t, aStyle, "border_bottom_width", Int(42))

	assertRGBA(t, "color", aStyle.Get("color"), 1, 0, 0, 1)
	assertValue(t, aStyle, "border_top_color", String("currentcolor"))

	assertValue(t, divStyle, "font_size", Int(40))             // 2 * 20px
	assertValue(t, span1Style, "width", Dimension{160, "px"})  // 10 * 16px (root default is 16px)
	assertValue(t, span1Style, "height", Dimension{400, "px"}) // 10 * (2 * 20px)
	assertValue(t, span2Style, "font_size", Int(32))

	// The href attr should be as in the source, not made absolute.
	assertValue(t, after, "content", Tuple{
		Tuple{String("string"), String(" [")},
		Tuple{String("attr()"), Tuple{String("href"), String("string"), String("")}},
		Tuple{String("string"), String("]")},
	})
	assertRGBA(t, "background_color", after.Get("background_color"), 1, 0, 0, 1)
	assertValue(t, after, "border_top_width", Int(42))
	assertValue(t, after, "border_bottom_width", Int(3))

	assertNoLogs(t, capture)
}

// TestImportant is test_important. Upstream reads the styles of the boxes
// of a rendered page. The port reads the computed styles of the elements.
func TestImportant(t *testing.T) {
	log, capture := captureLogs(slog.LevelInfo)
	document := fakeHTML(t, `
      <style>
        p:nth-child(1) { color: lime }
        body p:nth-child(2) { color: red }

        p:nth-child(3) { color: lime !important }
        body p:nth-child(3) { color: red }

        body p:nth-child(5) { color: lime }
        p:nth-child(5) { color: red }

        p:nth-child(6) { color: red }
        p:nth-child(6) { color: lime }
      </style>
      <p></p>
      <p></p>
      <p></p>
      <p></p>
      <p></p>
      <p></p>
    `, testBaseURL, log)
	styleFor := computedStyles(t, document, ComputedStylesOptions{
		UserStylesheets: []CSSSource{{Text: `
      body p:nth-child(1) { color: red }
      p:nth-child(2) { color: lime !important }

      p:nth-child(4) { color: lime !important }
      body p:nth-child(4) { color: red }
    `}},
	})
	for i, paragraph := range elementsByTag(document.EtreeElement, "p") {
		// lime (light green)
		assertRGBA(t, "color of p "+string(rune('1'+i)), styleFor.Get(paragraph, "").Get("color"), 0, 1, 0, 1)
	}
	assertNoLogs(t, capture)
}

// testMetrics is the TextMetrics of the font weasyprint.otf of the tests of
// upstream: its x is 0.8 em high, its 0 and its O are 1 em wide and high,
// and 水 is 2 em wide.
type testMetrics struct{}

// Strut satisfies TextMetrics.
func (testMetrics) Strut(style *Style) (lineHeight, baseline float64) {
	return defaultMetrics{}.Strut(style)
}

// CharacterRatio satisfies TextMetrics.
func (testMetrics) CharacterRatio(_ *Style, unit string) float64 {
	switch unit {
	case "ex":
		return 0.8
	case "ic":
		return 2
	}
	return 1
}

// TestUnits is test_units. Upstream reads the used left margins of the
// boxes. The port reads the computed left margins, with the metrics of the
// font of the tests and the sizes of the page that the layout sets.
func TestUnits(t *testing.T) {
	for _, c := range []struct {
		value string
		width float64
	}{
		// Absolute units.
		{"96px", 96},
		{"1in", 96},
		{"72pt", 96},
		{"6pc", 96},
		{"2.54cm", 96},
		{"25.4mm", 96},
		{"101.6q", 96},
		// Font-relative units.
		{"1.1em", 11},
		{"1.1rem", 17.6},
		{"1.1ch", 11},
		{"1.1rch", 17.6},
		{"1.1cap", 11},
		{"1.1rcap", 17.6},
		{"1.5ex", 12},
		{"2rex", 25.6},
		{"1ic", 20},
		{"1ric", 32},
		{"1.1lh", 13.2},
		{"1.1rlh", 26.4},
		// Viewport-relative units.
		{"50vh", 50},
		{"50vw", 100},
		{"25vb", 25},
		{"1.25vi", 2.5},
		{"10vmin", 10},
		{"20vmax", 40},
		{"50lvh", 50},
		{"50lvw", 100},
		{"25lvb", 25},
		{"1.25lvi", 2.5},
		{"10lvmin", 10},
		{"20lvmax", 40},
		{"50svh", 50},
		{"50svw", 100},
		{"25svb", 25},
		{"1.25svi", 2.5},
		{"10svmin", 10},
		{"20svmax", 40},
		{"50dvh", 50},
		{"50dvw", 100},
		{"25dvb", 25},
		{"1.25dvi", 2.5},
		{"10dvmin", 10},
		{"20dvmax", 40},
		{"50pvh", 100},
		{"50pvw", 150},
		{"25pvb", 50},
		{"1.5pvi", 4.5},
		{"10pvmin", 20},
		{"20pvmax", 60},
	} {
		log, capture := captureLogs(slog.LevelInfo)
		document := fakeHTML(t, `
      <html style="font: 16px / 1.5 weasyprint">
      <style>@page { size: 300px 200px; margin: 50px }</style>
      <body style="font: 10px / 1.2 weasyprint">
      <p style="margin-left: `+c.value+`"></p>
      <p style="margin-left: `+strings.ToUpper(c.value)+`"></p>
      `, testBaseURL, log)
		styleFor := computedStyles(t, document, ComputedStylesOptions{Metrics: testMetrics{}})
		styleFor.Context.InitialPageSizes = InitialPageSizes{Box: &[2]float64{300, 200}, Area: &[2]float64{200, 100}}
		ps := elementsByTag(document.EtreeElement, "p")
		m1, ok1 := styleFor.Get(ps[0], "").Get("margin_left").(Dimension)
		m2, ok2 := styleFor.Get(ps[1], "").Get("margin_left").(Dimension)
		if !ok1 || !ok2 || m1 != m2 {
			t.Errorf("%s: margins %v and %v differ", c.value, m1, m2)
		}
		if math.Abs(m1.Value-c.width) > 0.01*c.width {
			t.Errorf("%s: got %v, want %v", c.value, m1.Value, c.width)
		}
		assertNoLogs(t, capture)
	}
}

// TestRecursiveLH is test_recursive_lh. Upstream renders the pages. The port
// computes the styles that the rendering reads, and checks that they end.
func TestRecursiveLH(t *testing.T) {
	for _, property := range []string{"line-height", "font-size"} {
		for _, pair := range [][2]string{{"1lh", "1rlh"}, {"1rlh", "1lh"}} {
			log, capture := captureLogs(slog.LevelInfo)
			document := fakeHTML(t, `
      <html style="`+property+`: `+pair[0]+`">
      <body style="`+property+`: `+pair[1]+`">a`, testBaseURL, log)
			styleFor := computedStyles(t, document, ComputedStylesOptions{})
			for el := range document.EtreeElement.Iter() {
				style := styleFor.Get(el, "")
				style.Get("font_size")
				style.Get("line_height")
				strut(style)
			}
			assertNoLogs(t, capture)
		}
	}
}

// setPageTypeComputedStyles is set_page_type_computed_styles in
// weasyprint/layout/page.py, which the tests of the pages call.
func setPageTypeComputedStyles(t *testing.T, pageType PageType, html *HTML, styleFor *StyleFor) {
	t.Helper()
	styleFor.AddPageDeclarations(pageType)

	// Apply style for page. @page inherits from the root element:
	// https://lists.w3.org/Archives/Public/www-style/2012Jan/1164.html
	root := html.EtreeElement
	if err := styleFor.SetComputedStyles(pageType, root, root, "", html.BaseURL, nil); err != nil {
		t.Fatal(err)
	}

	// Apply style for page pseudo-elements (margin boxes).
	for _, key := range styleFor.CascadedKeys() {
		if key.Pseudo != "" && key.Owner.ownerKey() == pageType.ownerKey() {
			// The pseudo-element inherits from the element.
			if err := styleFor.SetComputedStyles(key.Owner, key.Owner, root, key.Pseudo, html.BaseURL, nil); err != nil {
				t.Fatal(err)
			}
		}
	}
}

// TestMediaQueries is test_media_queries. Upstream reads the width of the
// root box of a rendered page. The port reads the size of the page.
func TestMediaQueries(t *testing.T) {
	for _, c := range []struct {
		media   string
		width   float64
		warning bool
	}{
		{"@media screen { @page { size: 10px } }", 20, false},
		{"@media print { @page { size: 10px } }", 10, false},
		{`@media ("unknown content") { @page { size: 10px } }`, 20, true},
	} {
		log, capture := captureLogs(slog.LevelInfo)
		document := fakeHTML(t, "<p>a<span>b", testBaseURL, log)
		styleFor := computedStyles(t, document, ComputedStylesOptions{
			UserStylesheets: []CSSSource{{Text: "@page{size:20px}" + c.media}},
		})
		pageType := PageType{Side: "right"}
		setPageTypeComputedStyles(t, pageType, document, styleFor)
		size, _ := styleFor.Get(pageType, "").Get("size").(Tuple)
		if len(size) != 2 || num(size[0]) != c.width {
			t.Errorf("%s: got size %s, want width %v", c.media, ValueString(size), c.width)
		}
		if logs := capture.logs(); (len(logs) > 0) != c.warning {
			t.Errorf("%s: unexpected logs %q", c.media, logs)
		}
	}
}

// TestStringToCSS is test_string_to_css.
func TestStringToCSS(t *testing.T) {
	for _, c := range []struct{ s, want string }{
		{"a", `"a"`},
		{"a\nb", `"a\Ab"`},
		{"a\rb", `"a\Db"`},
		{"\n\na", `"\A\Aa"`},
		{`a"b`, `"a\"b"`},
	} {
		if got := stringToCSS(c.s); got != c.want {
			t.Errorf("stringToCSS(%q): got %q, want %q", c.s, got, c.want)
		}
	}
}

// TestURLToCSS is test_url_to_css.
func TestURLToCSS(t *testing.T) {
	for _, c := range []struct{ s, want string }{
		{"a", `url("a")`},
		{"a\nb", `url("a\Ab")`},
		{"a\rb", `url("a\Db")`},
		{"\n\na", `url("\A\Aa")`},
		{`a"b`, `url("a\"b")`},
	} {
		if got := urlToCSS(c.s); got != c.want {
			t.Errorf("urlToCSS(%q): got %q, want %q", c.s, got, c.want)
		}
	}
}
