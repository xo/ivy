// Copyright (c) 2011-2021, Simon Sapin and contributors.
//
// Use of this source code is governed by the BSD-3-Clause license that can be
// found in the LICENSE file.

// This file ports tests/css/test_variables.py.
//
// Upstream renders the pages and reads the boxes. Most tests here read the
// computed styles of the elements instead, because the boxes wait for a
// later stage of the port. Such a test says so in its comment.

package layout

import (
	"fmt"
	"log/slog"
	"slices"
	"strings"
	"testing"
)

// cascade parses an HTML document with the style sheet of the tests as the
// style sheet of the user agent, computes its styles, and captures the log
// at the level INFO, as capture_logs does.
func cascade(t *testing.T, source string) (*HTML, *StyleFor, *logCapture) {
	t.Helper()
	log, capture := captureLogs(slog.LevelInfo)
	document := fakeHTML(t, source, testBaseURL, log)
	return document, computedStyles(t, document, ComputedStylesOptions{}), capture
}

// elementsByTag returns the elements with a tag, in the order of the
// document.
func elementsByTag(root *Element, tag string) []*Element {
	return slices.Collect(root.IterTag(tag))
}

// styleOf returns the computed style of the first element with a tag.
func styleOf(t *testing.T, document *HTML, sf *StyleFor, tag string) *Style {
	t.Helper()
	el := findElement(document.EtreeElement, tag)
	if el == nil {
		t.Fatalf("no element %s", tag)
	}
	style := sf.Get(el, "")
	if style == nil {
		t.Fatalf("no style for %s", tag)
	}
	return style
}

// assertValue fails the test when the computed value of a property is not
// want.
func assertValue(t *testing.T, style *Style, name string, want Value) {
	t.Helper()
	if got := style.Get(name); !Equal(got, want) {
		t.Errorf("%s: got %s, want %s", name, ValueString(got), ValueString(want))
	}
}

// px is a Dimension in pixels.
func px(v float64) Dimension {
	return Dimension{v, "px"}
}

// TestVariableWidth ports the tests of upstream that check the width of a
// paragraph set by var(). They read the computed width instead of the width
// of the box.
func TestVariableWidth(t *testing.T) {
	for _, c := range []struct {
		name, css, html string
		want            Value
	}{
		// test_variable_simple
		{"simple", `p { --var: 10px; width: var(--var) }`, "<p></p>", px(10)},
		// test_variable_not_computed
		{"not_computed", `p { --var: 1rem; width: var(--var) }`, "<p></p>", px(16)},
		// test_variable_inherit
		{"inherit", `html { --var: 10px } p { width: var(--var) }`, "<p></p>", px(10)},
		// test_variable_inherit_override
		{"inherit_override", `html { --var: 20px } p { width: var(--var); --var: 10px }`, "<p></p>", px(10)},
		// test_variable_default_unknown
		{"default_unknown", `p { width: var(--x, 10px) }`, "<p></p>", px(10)},
		// test_variable_default_var
		{"default_var", `p { --var: 10px; width: var(--x, var(--var)) }`, "<p></p>", px(10)},
		// test_variable_case_sensitive
		{"case_sensitive", `html { --var: 20px } body { --VAR: 10px } p { width: var(--VAR) }`, "<p></p>", px(10)},
		// test_variable_chain
		{"chain", `html { --foo: 10px } body { --var: var(--foo) } p { width: var(--var) }`, "<p></p>", px(10)},
		// test_variable_initial: the used width is the width of the body,
		// as the computed width is auto.
		{"initial", `html { --var: initial } p { width: var(--var) }`, "<p></p>", String("auto")},
		// test_variable_initial_default
		{"initial_default", `p { --var: initial; width: var(--var, 10px) }`, "<p></p>", String("auto")},
		// test_variable_initial_default_var
		{"initial_default_var", `p { --var: initial; width: var(--var, var(--var)) }`, "<p></p>", String("auto")},
	} {
		t.Run(c.name, func(t *testing.T) {
			document, sf, capture := cascade(t, "<style>"+c.css+"</style>"+c.html)
			assertValue(t, styleOf(t, document, sf, "p"), "width", c.want)
			assertNoLogs(t, capture)
		})
	}
}

// TestVariableDoubleChain is test_variable_double_chain. It reads the
// computed background-image instead of the box.
func TestVariableDoubleChain(t *testing.T) {
	document, sf, capture := cascade(t, `
      <style>
        html { --foo: red }
        body { --var: var(--foo), var(--foo) }
        div { background-image: linear-gradient(var(--var)) }
      </style>
      <div></dib>`)
	assertRedGradient(t, styleOf(t, document, sf, "div"), 2)
	assertNoLogs(t, capture)
}

// assertRedGradient checks that the first background image is a gradient
// whose first n colors are red.
func assertRedGradient(t *testing.T, style *Style, n int) {
	t.Helper()
	images, ok := style.Get("background_image").(Tuple)
	if !ok || len(images) == 0 {
		t.Fatalf("background_image: got %s", ValueString(style.Get("background_image")))
	}
	image, _ := images[0].(Tuple)
	if len(image) != 2 {
		t.Fatalf("image: got %s", ValueString(images[0]))
	}
	gradient, ok := image[1].(*LinearGradient)
	if !ok || len(gradient.Colors) < n {
		t.Fatalf("gradient: got %s", ValueString(image[1]))
	}
	for i := range n {
		if !Equal(gradient.Colors[i], mustParseColor("red")) {
			t.Errorf("color %d: got %s, want red", i, ValueString(gradient.Colors[i]))
		}
	}
}

// TestVariableChainRoot is test_variable_chain_root. It reads the computed
// width of the root element instead of the box.
func TestVariableChainRoot(t *testing.T) {
	// Regression test for #1656.
	document, sf, capture := cascade(t, `
      <style>
        html { --var2: 10px; --var1: var(--var2); width: var(--var1) }
      </style>`)
	assertValue(t, sf.Get(document.EtreeElement, ""), "width", px(10))
	assertNoLogs(t, capture)
}

// TestVariableLoops ports the tests of upstream that only check that a
// document renders: the self references, the loops and the missing
// variables. They compute each property of each element instead of
// rendering, and they must end.
func TestVariableLoops(t *testing.T) {
	for _, c := range []struct{ name, css string }{
		// test_variable_self
		{"self", `html { --var1: var(--var1) }`},
		// test_variable_self_used
		{"self_used", `html { --var1: var(--var1); width: var(--var1) }`},
		// test_variable_loop
		{"loop", `html { --var1: var(--var2); --var2: var(--var1); padding: var(--var1) }`},
		// test_variable_chain_root_missing, regression test for #1656.
		{"chain_root_missing", `html { --var1: var(--var-missing); width: var(--var1) }`},
		// test_variable_chain_root_missing_inherited, regression test for
		// #2164.
		{"chain_root_missing_inherited", `html { --var1: var(--var-missing); font: var(--var1) }`},
	} {
		t.Run(c.name, func(t *testing.T) {
			document, sf, _ := cascade(t, "<style>"+c.css+"</style>a")
			computeAllStyles(document, sf)
		})
	}
}

// computeAllStyles reads each known property of the style of each element
// and of its ::before and ::after, as a rendering would.
func computeAllStyles(document *HTML, sf *StyleFor) {
	for el := range document.EtreeElement.Iter() {
		for _, pseudo := range []string{"", "before", "after", "marker"} {
			style := sf.Get(el, pseudo)
			if style == nil {
				continue
			}
			for name := range InitialValues {
				style.Get(name)
			}
		}
	}
}

// TestVariableExponentialChain checks the guard against a chain of custom
// properties where each one uses the previous one twice. Upstream builds a
// value of 2^30 tokens. The port stops past maxVarTokens and the value is
// not valid, so the width keeps its initial value.
func TestVariableExponentialChain(t *testing.T) {
	var css strings.Builder
	css.WriteString("html { --v0: 1px")
	for i := 1; i <= 30; i++ {
		fmt.Fprintf(&css, "; --v%d: var(--v%d) var(--v%d)", i, i-1, i-1)
	}
	css.WriteString("; width: var(--v30) }")
	document, sf, _ := cascade(t, "<style>"+css.String()+"</style>")
	assertValue(t, sf.Get(document.EtreeElement, ""), "width", String("auto"))
}

// TestVariableShorthandMargin ports test_variable_shorthand_margin and
// test_variable_shorthand_margin_multiple. They read the computed margins
// instead of the box.
func TestVariableShorthandMargin(t *testing.T) {
	for _, c := range []struct {
		name, css                string
		top, right, bottom, left float64
	}{
		{"single", `html { --var: 10px } div { margin: 0 0 0 var(--var) }`, 0, 0, 0, 10},
		{"multiple", `html { --var1: 10px; --var2: 20px } div { margin: var(--var2) 0 0 var(--var1) }`, 20, 0, 0, 10},
	} {
		t.Run(c.name, func(t *testing.T) {
			document, sf, capture := cascade(t, "<style>"+c.css+"</style><div></div>")
			div := styleOf(t, document, sf, "div")
			assertValue(t, div, "margin_top", px(c.top))
			assertValue(t, div, "margin_right", px(c.right))
			assertValue(t, div, "margin_bottom", px(c.bottom))
			assertValue(t, div, "margin_left", px(c.left))
			assertNoLogs(t, capture)
		})
	}
}

// TestVariableShorthandMarginInvalid is
// test_variable_shorthand_margin_invalid. It reads the computed margins
// instead of the box.
func TestVariableShorthandMarginInvalid(t *testing.T) {
	document, sf, capture := cascade(t, `
      <style>
        html { --var: blue }
        div { margin: 0 0 0 var(--var) }
      </style>
      <div></div>`)
	div := styleOf(t, document, sf, "div")
	for _, side := range []string{"top", "right", "bottom", "left"} {
		assertValue(t, div, "margin_"+side, px(0))
	}
	assertLog(t, capture, "invalid value")
}

// TestVariableShorthandBorder ports the tests of upstream on border with
// var(). They read the computed border widths instead of the box.
func TestVariableShorthandBorder(t *testing.T) {
	for _, c := range []struct {
		name, css                string
		top, right, bottom, left float64
		log                      string
	}{
		// test_variable_shorthand_border
		{"border", `html { --var: 1px solid blue } div { border: var(--var) }`, 1, 1, 1, 1, ""},
		// test_variable_shorthand_border_side
		{"border_side", `html { --var: 1px solid blue } div { border-top: var(--var) }`, 1, 0, 0, 0, ""},
		// test_variable_shorthand_border_mixed
		{"border_mixed", `html { --var: 1px solid } div { border: blue var(--var) }`, 1, 1, 1, 1, ""},
		// test_variable_shorthand_border_mixed_invalid
		{"border_mixed_invalid", `html { --var: 1px solid blue } div { border: blue var(--var) }`, 0, 0, 0, 0, "multiple color values"},
	} {
		t.Run(c.name, func(t *testing.T) {
			document, sf, capture := cascade(t, "<style>"+c.css+"</style><div></div>")
			div := styleOf(t, document, sf, "div")
			for i, side := range []string{"top", "right", "bottom", "left"} {
				want := []float64{c.top, c.right, c.bottom, c.left}[i]
				if got := num(div.Get("border_" + side + "_width")); got != want {
					t.Errorf("border_%s_width: got %v, want %v", side, got, want)
				}
			}
			if c.log == "" {
				assertNoLogs(t, capture)
				return
			}
			// TODO of upstream: we should only get one warning here.
			if logs := capture.logs(); len(logs) == 0 || !strings.Contains(logs[0], c.log) {
				t.Errorf("expected a log with %q, got: %q", c.log, logs)
			}
		})
	}
}

// backgroundProperties are the properties of the shorthand background.
var backgroundProperties = []string{
	"background_color", "background_image", "background_repeat",
	"background_attachment", "background_position", "background_size",
	"background_clip", "background_origin",
}

// TestVariableShorthandBackground is test_variable_shorthand_background. It
// computes the background properties instead of rendering.
func TestVariableShorthandBackground(t *testing.T) {
	for _, c := range []struct{ v, background string }{
		{"blue", "var(--v)"},
		{"padding-box url(pattern.png)", "var(--v)"},
		{"padding-box url(pattern.png)", "white var(--v) center"},
		{"100%", "url(pattern.png) var(--v) var(--v) / var(--v) var(--v)"},
		{"left / 100%", "url(pattern.png) top var(--v) 100%"},
	} {
		t.Run(c.v+" "+c.background, func(t *testing.T) {
			document, sf, capture := cascade(t, fmt.Sprintf(
				"<style>html { --v: %s } div { background: %s }</style><div></div>", c.v, c.background))
			div := styleOf(t, document, sf, "div")
			for _, name := range backgroundProperties {
				div.Get(name)
			}
			assertNoLogs(t, capture)
		})
	}
}

// TestVariableShorthandBackgroundInvalid is
// test_variable_shorthand_background_invalid. It computes the background
// properties instead of rendering.
func TestVariableShorthandBackgroundInvalid(t *testing.T) {
	for _, c := range []struct{ v, background string }{
		{"invalid", "var(--v)"},
		{"blue", "var(--v) var(--v)"},
		{"100%", "url(pattern.png) var(--v) var(--v) var(--v)"},
	} {
		t.Run(c.v+" "+c.background, func(t *testing.T) {
			document, sf, capture := cascade(t, fmt.Sprintf(
				"<style>html { --v: %s } div { background: %s }</style><div></div>", c.v, c.background))
			div := styleOf(t, document, sf, "div")
			for _, name := range backgroundProperties {
				div.Get(name)
			}
			assertLog(t, capture, "invalid value")
		})
	}
}

// TestVariableFallback is test_variable_fallback. It computes the property
// instead of rendering, for each known property, and checks that nothing
// panics.
func TestVariableFallback(t *testing.T) {
	names := make([]string, 0, len(knownProperties))
	for name := range knownProperties {
		names = append(names, name)
	}
	slices.Sort(names)
	for _, name := range names {
		document, sf, _ := cascade(t, fmt.Sprintf(
			"<style>div { --var: improperValue; %s: var(--var) }</style><div></div>", name))
		styleOf(t, document, sf, "div").Get(strings.ReplaceAll(name, "-", "_"))
	}
}

// TestVariableListContent is test_variable_list_content. It reads the
// computed content of div::before. The text "Page 1/1" waits for the boxes.
func TestVariableListContent(t *testing.T) {
	// Regression test for #1287.
	document, sf, capture := cascade(t, `
      <style>
        :root { --var: "Page " counter(page) "/" counter(pages) }
        div::before { content: var(--var) }
      </style>
      <div></div>`)
	before := sf.Get(findElement(document.EtreeElement, "div"), "before")
	want := Tuple{
		Tuple{String("string"), String("Page ")},
		Tuple{String("counter()"), Tuple{String("page"), String("decimal")}},
		Tuple{String("string"), String("/")},
		Tuple{String("counter()"), Tuple{String("pages"), String("decimal")}},
	}
	assertValue(t, before, "content", want)
	assertNoLogs(t, capture)
}

// TestVariableListDisplay is test_variable_list_display. Upstream checks
// that the div is in a line box. The port checks that its computed display
// is inline-level.
func TestVariableListDisplay(t *testing.T) {
	for _, c := range []struct{ v, display string }{
		{"inline", "var(--var)"},
		{"inline-block", "var(--var)"},
		{"inline flow", "var(--var)"},
		{"inline", "var(--var) flow"},
		{"flow", "inline var(--var)"},
	} {
		t.Run(c.v+" "+c.display, func(t *testing.T) {
			document, sf, capture := cascade(t, fmt.Sprintf(
				"<style>html { --var: %s } div { display: %s }</style><section><div></div></section>", c.v, c.display))
			display, _ := styleOf(t, document, sf, "div").Get("display").(Tuple)
			if len(display) == 0 || !isStr(display[0], "inline") {
				t.Errorf("display: got %s, want an inline display", ValueString(display))
			}
			assertNoLogs(t, capture)
		})
	}
}

// TestVariableListFont is test_variable_list_font. Upstream measures the
// text with the font weasyprint. The port checks the computed font
// families.
func TestVariableListFont(t *testing.T) {
	for _, c := range []struct {
		v, font string
		want    Tuple
	}{
		{"weasyprint", "var(--var)", tupleOf("weasyprint")},
		{`"weasyprint"`, "var(--var)", tupleOf("weasyprint")},
		{"weasyprint", "var(--var), monospace", tupleOf("weasyprint", "monospace")},
		{"weasyprint, monospace", "var(--var)", tupleOf("weasyprint", "monospace")},
		{"monospace", "weasyprint, var(--var)", tupleOf("weasyprint", "monospace")},
	} {
		t.Run(c.v+" "+c.font, func(t *testing.T) {
			document, sf, capture := cascade(t, fmt.Sprintf(
				"<style>html { font-size: 2px; --var: %s } div { font-family: %s }</style><div>aa</div>", c.v, c.font))
			assertValue(t, styleOf(t, document, sf, "div"), "font_family", c.want)
			assertNoLogs(t, capture)
		})
	}
}

// counterContent is the computed content counter(name, style).
func counterContent(name, style string) Tuple {
	return Tuple{Tuple{String("counter()"), Tuple{String(name), String(style)}}}
}

// TestVariableInFunction ports the tests of upstream that use var() in
// counter functions. Upstream checks the text of the counters. The port
// checks the computed counter-increment of the h1 elements and the computed
// content of each div::before, as the text waits for the boxes.
func TestVariableInFunction(t *testing.T) {
	for _, c := range []struct {
		name, css, divs string
		want            []Tuple
	}{
		// test_variable_in_function
		{"in_function", `
        html { --var: title }
        h1 { counter-increment: var(--var) }
        div::before { content: counter(var(--var)) }`,
			"<h1></h1><div></div><h1></h1><div></div>",
			[]Tuple{counterContent("title", "decimal"), counterContent("title", "decimal")}},
		// test_variable_in_function_multiple_values
		{"multiple_values", `
        html { --name: title; --counter: title, upper-roman }
        h1 { counter-increment: var(--name) }
        div::before { content: counter(var(--counter)) }`,
			`<h1></h1><div></div><h1></h1><div></div><h1></h1><div style="--counter: var(--name), lower-roman"></div>`,
			[]Tuple{
				counterContent("title", "upper-roman"), counterContent("title", "upper-roman"),
				counterContent("title", "lower-roman"),
			}},
		// test_variable_in_variable_in_function
		{"variable_in_variable", `
        html { --name: title; --counter: var(--name), upper-roman }
        h1 { counter-increment: var(--name) }
        div::before { content: counter(var(--counter)) }`,
			`<h1></h1><div></div><h1></h1><div></div><h1></h1><div style="--counter: var(--name), lower-roman"></div>`,
			[]Tuple{
				counterContent("title", "upper-roman"), counterContent("title", "upper-roman"),
				counterContent("title", "lower-roman"),
			}},
		// test_variable_in_function_in_variable
		{"function_in_variable", `
        html { --name: title; --counter: counter(var(--name), upper-roman) }
        h1 { counter-increment: var(--name) }
        div::before { content: var(--counter) }`,
			`<h1></h1><div></div><h1></h1><div></div><h1></h1><div style="--counter: counter(var(--name), lower-roman)"></div>`,
			[]Tuple{
				counterContent("title", "upper-roman"), counterContent("title", "upper-roman"),
				counterContent("title", "lower-roman"),
			}},
	} {
		t.Run(c.name, func(t *testing.T) {
			document, sf, capture := cascade(t, "<style>"+c.css+"</style><section>"+c.divs+"</section>")
			root := document.EtreeElement
			for _, h1 := range elementsByTag(root, "h1") {
				assertValue(t, sf.Get(h1, ""), "counter_increment", Tuple{Tuple{String("title"), Int(1)}})
			}
			divs := elementsByTag(root, "div")
			if len(divs) != len(c.want) {
				t.Fatalf("got %d divs, want %d", len(divs), len(c.want))
			}
			for i, div := range divs {
				before := sf.Get(div, "before")
				if before == nil {
					t.Fatalf("div %d has no ::before", i)
				}
				assertValue(t, before, "content", c.want[i])
			}
			assertNoLogs(t, capture)
		})
	}
}

// TestVariableInFunctionMissing is test_variable_in_function_missing. It
// reads the computed values instead of the boxes: a missing variable makes
// the counter-increment and the content not valid, so they keep their
// initial values, and the ::before makes no box.
func TestVariableInFunctionMissing(t *testing.T) {
	document, sf, capture := cascade(t, `
      <style>
        h1 { counter-increment: var(--var) }
        div::before { content: counter(var(--var)) }
      </style>
      <section><h1></h1><div></div><h1></h1><div></div></section>`)
	root := document.EtreeElement
	for _, h1 := range elementsByTag(root, "h1") {
		assertValue(t, sf.Get(h1, ""), "counter_increment", String("auto"))
	}
	for _, div := range elementsByTag(root, "div") {
		assertValue(t, sf.Get(div, "before"), "content", String("normal"))
	}
	logs := capture.logs()
	if len(logs) != 2 || !strings.Contains(logs[0], "no value") || !strings.Contains(logs[1], "invalid value") {
		t.Errorf("expected the logs 'no value' and 'invalid value', got: %q", logs)
	}
}

// TestSameVariableInFunction is test_same_variable_in_function. It reads
// the computed background-image instead of the box.
func TestSameVariableInFunction(t *testing.T) {
	document, sf, capture := cascade(t, `
      <style>
        body { --var: red }
        div { background-image: linear-gradient(var(--var), var(--var)) }
      </style>
      <div></div>`)
	assertRedGradient(t, styleOf(t, document, sf, "div"), 2)
	assertNoLogs(t, capture)
}

// TestVariableInNestedFunction is test_variable_in_nested_function. It
// reads the computed background-image instead of the box.
func TestVariableInNestedFunction(t *testing.T) {
	document, sf, capture := cascade(t, `
      <style>
        body { --var: 255 0 0 }
        div { background-image: linear-gradient(rgba(var(--var))) }
      </style>
      <div></div>`)
	assertRedGradient(t, styleOf(t, document, sf, "div"), 1)
	assertNoLogs(t, capture)
}

// TestVariableAndFunctionInFunction is
// test_variable_and_function_in_function. Upstream checks the page number
// of the target. The port checks the computed content of a::after, as the
// value of target-counter() waits for the pagination.
func TestVariableAndFunctionInFunction(t *testing.T) {
	document, sf, capture := cascade(t, `
      <style>
        html { --counter-name: page }
        a::after { content: target-counter(attr(href), var(--counter-name)) }
      </style>
      <a id="link" href="#link"></a>`)
	after := sf.Get(findElement(document.EtreeElement, "a"), "after")
	want := Tuple{Tuple{String("target-counter()"), Tuple{
		Tuple{String("attr()"), Tuple{String("href"), String("string"), String("")}},
		String("page"),
		String("decimal"),
	}}}
	assertValue(t, after, "content", want)
	assertNoLogs(t, capture)
}
