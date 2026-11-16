// Copyright (c) 2011-2021, Simon Sapin and contributors.
//
// Use of this source code is governed by the BSD-3-Clause license that can be
// found in the LICENSE file.

// This file ports tests/css/test_math.py.
//
// Upstream renders the pages and reads the used width of a box. The tests
// here read the computed width of the element instead, and resolve its
// percentages and its math functions against the width of the containing
// block, as percentage in weasyprint/layout/percent.py does. The tests that
// only check that a document renders compute each property of each element
// instead.

package layout

import (
	"fmt"
	"math"
	"slices"
	"strings"
	"testing"

	"github.com/xo/okage/tinycss2"
)

// testPercentage is percentage in weasyprint/layout/percent.py, for the tests:
// it returns the used length of a computed length, with referTo as the
// length of 100%, and false for auto or a value it cannot resolve.
func testPercentage(t *testing.T, value Value, computed *Style, referTo float64) (float64, bool) {
	t.Helper()
	switch v := value.(type) {
	case Dimension:
		if v.Unit == "%" {
			return referTo * v.Value / 100, true
		}
		return v.Value, true
	case Token:
		resolved, err := resolveMath(v.Node, computed, "", &referTo)
		if err != nil || resolved == nil {
			t.Errorf("resolving %s: %v", v.Node.Serialize(), err)
			return 0, false
		}
		n, _ := tokenNumber(resolved)
		if resolved.Type() == tinycss2.TypePercentage {
			return referTo * n / 100, true
		}
		return n, true
	}
	return 0, false
}

// mathDocument computes the styles of the document of test_math_functions:
// a page of 400px by 500px with margins of 100px, and a body of 200px.
func mathDocument(t *testing.T, width string) (*Style, *logCapture) {
	t.Helper()
	document, sf, capture := cascade(t, fmt.Sprintf(`
      <style>
        @page { size: 400px 500px; margin: 100px }
        body { font-size: 10px; width: 200px }
      </style>
      <div style="--one: 1; height: 1px; width: %s"></div>`, width))
	// The layout sets the sizes of the first page before it reads a style.
	sf.Context.InitialPageSizes.Box = &[2]float64{400, 500}
	sf.Context.InitialPageSizes.Area = &[2]float64{200, 300}
	return styleOf(t, document, sf, "div"), capture
}

// TestMathFunctions is test_math_functions. It checks that the used width
// of the div is 100px.
func TestMathFunctions(t *testing.T) {
	for _, width := range []string{
		"calc(100px)",
		"calc(10em)",
		"calc(50vw)",
		"calc(20pvh)",
		"calc(50%)",
		"calc(10px + 90px)",
		"calc(5em + 50px)",
		"calc(2 * 5em)",
		"calc(2 * (3em + 20px))",
		"calc(25% * (1 + 1))",
		"calc(20% * (1 + 1) + 20px)",
		"calc(100px",
		"max(100px)",
		"max(30%, 2em, 100px)",
		"max(-30%, -2em, 10em)",
		"calc(max(-1, 1, 2) * 50px)",
		"min(100px)",
		"min(100%, 20em, 100px)",
		"calc(min(4, 2) * 50px)",
		"calc(sqrt(4) * 50px)",
		"calc(pow(2, 2) * 25px)",
		"calc(hypot(2) * 50px)",
		"calc(hypot(3, 4) * 20px)",
		"calc(hypot(2px) * 50)",
		"calc(hypot(3px, 4px) * 20)",
		"calc(log(e) * 100px)",
		"calc(log(100, 10) * 50px)",
		"calc(exp(1) / e * 100px)",
		"abs(-100px)",
		"calc(abs(-100) * 1px)",
		"calc(sign(-100) * -100px)",
		"calc(sign(-100px) * -100px)",
		"calc(sqrt(16) * min(25px, 100%))",
		"clamp(calc(-infinity * 1px), 10em, calc(infinity * 1px))",
		"clamp(50px, 10em, 500px)",
		"clamp(100px, 2em, 500px)",
		"clamp(10px, 100em, 10em)",
		"clamp(10px, 100%, 10em)",
		"round(100.4px)",
		"round(145.4px, 100px)",
		"round(nearest, 100px)",
		"round(down, 195px, 100px)",
		"round(up, 5px, 100px)",
		"round(to-zero, 195px, 100px)",
		"mod(300px, 200px)",
		"calc(mod(300px, -200px) * -1)",
		"calc(mod(-300px, -200px) * -1)",
		"rem(300px, 200px)",
		"rem(300px, -200px)",
		"calc(rem(-300px, -200px) * -1)",
		"calc(sin(30deg) * 200px)",
		"calc(cos(60deg) * 200px)",
		"calc(tan(45deg) * 100px)",
		"calc(tan(calc(pi / 4)) * 100px)",
		"calc(sin(asin(0.5)) * 200px)",
		"calc(cos(acos(0.5)) * 200px)",
		"calc(tan(atan(1)) * 100px)",
		"calc(tan(atan2(1, 1)) * 100px)",
		"calc(100px * var(--one))",
		"calc(50% * var(--one))",
		"calc(100px * sqrt(var(--one)))",
	} {
		t.Run(width, func(t *testing.T) {
			div, capture := mathDocument(t, width)
			got, ok := testPercentage(t, div.Get("width"), div, 200)
			if !ok || !isClose(got, 100) {
				t.Errorf("width: got %v (%s), want 100", got, ValueString(div.Get("width")))
			}
			assertNoLogs(t, capture)
		})
	}
}

// TestCalcDivisionByZero is test_calc_division_by_zero. A division by zero
// in calc() resolves to an infinity or to NaN, as CSS Values and Units 4
// requires. Upstream compares the used widths, and the port compares the
// computed widths, with NaN equal to NaN.
func TestCalcDivisionByZero(t *testing.T) {
	width := func(t *testing.T, value string) float64 {
		t.Helper()
		document, sf, capture := cascade(t, fmt.Sprintf(
			`<div style="font-size: 10px; height: 1px; width: %s"></div>`, value))
		div := styleOf(t, document, sf, "div")
		got, ok := testPercentage(t, div.Get("width"), div, 0)
		if !ok {
			t.Errorf("%s: no width, got %s", value, ValueString(div.Get("width")))
		}
		assertNoLogs(t, capture)
		return got
	}
	for _, c := range []struct{ width, reference string }{
		// A positive numerator over zero is +infinity, ...
		{"calc(100px / 0)", "calc(infinity * 1px)"},
		{"calc(50px / 0)", "calc(infinity * 1px)"},
		{"calc(10em / 0)", "calc(infinity * 1px)"},
		// ... a negative numerator is -infinity, ...
		{"calc(-100px / 0)", "calc(-infinity * 1px)"},
		// ... and zero over zero is NaN.
		{"calc(0px / 0)", "calc(nan * 1px)"},
		// The divisor may evaluate to zero rather than being a literal zero.
		{"calc(100px / (1 - 1))", "calc(infinity * 1px)"},
		// The resulting infinity keeps propagating through later operations.
		{"calc(100px / 0 + 100px)", "calc(infinity * 1px)"},
		{"calc(-1px / 0 * 100)", "calc(-infinity * 1px)"},
	} {
		t.Run(c.width, func(t *testing.T) {
			got, want := width(t, c.width), width(t, c.reference)
			if got != want && !(math.IsNaN(got) && math.IsNaN(want)) {
				t.Errorf("got %v, want %v", got, want)
			}
		})
	}
}

// TestMathFunctionsError is test_math_functions_error. It computes the width
// of the div instead of rendering, and checks that one message is logged.
func TestMathFunctionsError(t *testing.T) {
	for _, width := range []string{
		"calc",
		"(calc)",
		"calc(",
		"calc()",
		`calc("100px")`,
		"calc(100)",
		"calc(100px 100px)",
		"calc(100px, 100px)",
		"calc(100px * 100px)",
		"calc(100 * 100)",
		"calc(calc(100unknown))",
		"calc(0.1)",
		"calc(-1)",
		"min()",
		"min(10)",
		`min("10px")`,
		"min(10, 5px)",
		"calc(min(1, 5px) * 10px)",
		"max()",
		"max(10)",
		`max("10px")`,
		"max(10, 50px)",
		"calc(max(100, 5px) * 10px)",
		"calc(100* - max(56px, 1rem)",
		"clamp()",
		"clamp(10px)",
		"clamp(10px, 50px)",
		"clamp(10px, 50px, 100px, 200px)",
		`clamp(10px, "50px", 100px)`,
		"round()",
		"round(100)",
		"round(100, 10)",
		"round(nearest, 100, 10)",
		"round(100px, 10)",
		`round(100px, "10px")`,
		"round(nearest, 100px, 10)",
		"round(100px, 10px, 1)",
		"round(nearest, 100px, 10px, 1)",
		"round(unknown, 100px)",
		"round(unknown, 100px, 10px)",
		"mod()",
		"mod(10px)",
		"mod(100px, 10)",
		`mod(100px, "10px")`,
		"calc(mod(300px, 200) * -1)",
		"mod(100px, 10px, 1px)",
		"rem()",
		"rem(10px)",
		"rem(100px, 10)",
		`rem(100px, "10px")`,
		"calc(rem(300px, 200) * -1)",
		"rem(100px, 10px, 1px)",
		"sin()",
		"sin(10)",
		"sin(10%)",
		"sin(10deg)",
		"calc(sin(10) * 1)",
		"cos()",
		"cos(10)",
		"cos(10%)",
		"cos(10deg)",
		"calc(cos(10) * 1)",
		"tan()",
		"tan(10)",
		"tan(10%)",
		"tan(10deg)",
		"calc(tan(10) * 1)",
		"asin()",
		"asin(0)",
		"asin(0.5)",
		"asin(50deg)",
		"calc(sin(asin(50deg)) * 200px)",
		"calc(sin(asin(0.5)) * 200)",
		"calc(sin(asin(0.5, 2)) * 200px)",
		"calc(sin(asin(5)) * 200px)",
		"acos()",
		"acos(0)",
		"acos(0.5)",
		"acos(50deg)",
		"calc(cos(acos(50deg)) * 200px)",
		"calc(cos(acos(0.5)) * 200)",
		"calc(cos(acos(0.5, 2)) * 200px)",
		"calc(cos(acos(5)) * 200px)",
		"atan()",
		"atan(0)",
		"atan(0.5)",
		"atan(50deg)",
		"calc(tan(atan(50deg)) * 200px)",
		"calc(tan(atan(0.5)) * 200)",
		"calc(tan(atan(0.5, 2)) * 200px)",
		"atan2()",
		"atan2(0.5)",
		"atan2(0.5, 1)",
		"atan2(50deg, 1)",
		"calc(tan(atan2(50deg, 1)) * 200px)",
		"calc(tan(atan2(0.5, 1)) * 200)",
		"pow()",
		"pow(4, 3)",
		"pow(4px, 3)",
		"pow(4, 3, 4)",
		"sqrt()",
		"sqrt(4)",
		"sqrt(4px)",
		"sqrt(4, 2)",
		"hypoth()",
		"hypoth(3)",
		"hypoth(3, 4)",
		"log()",
		"log(10)",
		"log(10px)",
		"log(10, 10)",
		"log(10px, 10)",
		"log(10, 10, 10)",
		"exp()",
		"exp(10)",
		"exp(10px)",
		"exp(10, 10)",
		"exp(10px, 10)",
		"exp(10, 10, 10)",
		"abs()",
		"abs(10)",
		"abs(10px, 100)",
		"sign()",
		"sign(10)",
		"sign(10px)",
		"sign(10px, 10)",
	} {
		t.Run(width, func(t *testing.T) {
			document, sf, capture := cascade(t, fmt.Sprintf(`
              <style>body { font-size: 10px; width: 200px }</style>
              <div style="--one: 1; height: 1px; width: %s"></div>`, width))
			styleOf(t, document, sf, "div").Get("width")
			if logs := capture.logs(); len(logs) != 1 {
				t.Errorf("expected 1 log, got: %q", logs)
			}
		})
	}
}

// TestMathFunctionsPercentageAndFontUnit is
// test_math_functions_percentage_and_font_unit. For each property, a value
// that mixes a percentage and a font unit in calc() logs as much as a
// percentage alone, or as a font unit alone when the property accepts
// percentages but not lengths. The port computes the property instead of
// rendering.
func TestMathFunctionsPercentageAndFontUnit(t *testing.T) {
	logCount := func(t *testing.T, name, value string) int {
		t.Helper()
		document, sf, capture := cascade(t, fmt.Sprintf(`<div style="%s: %s"></div>`, name, value))
		styleOf(t, document, sf, "div").Get(strings.ReplaceAll(name, "-", "_"))
		return len(capture.logs())
	}
	names := make([]string, 0, len(properties))
	for name := range properties {
		names = append(names, name)
	}
	slices.Sort(names)
	// The port computes the property, where upstream reads only the
	// properties that the layout needs.
	differences := map[string]string{
		"font-size": "calc(50% + 1em) in font-size needs the font size itself: upstream recurses " +
			"until RecursionError, and the port stops at once with one warning",
		"transform-origin": "upstream reads transform-origin only for an element with a transform, " +
			"and its computed value gives the same warning as in the port",
	}
	for _, name := range names {
		t.Run(name, func(t *testing.T) {
			if reason, ok := differences[name]; ok {
				t.Skip(reason)
			}
			mathLogs := logCount(t, name, "calc(50% + 1em)")
			logs := logCount(t, name, "50%")
			if logs == 0 {
				// Happens when property accepts percentages but not lengths.
				logs = logCount(t, name, "1em")
			}
			if mathLogs != logs {
				t.Errorf("got %d logs with calc(), want %d", mathLogs, logs)
			}
		})
	}
}

// TestMathFunctionsComputed ports the tests of upstream that only check
// that a document with math functions renders without a log. They compute
// each property of each element instead of rendering, as the layout waits
// for a later stage.
func TestMathFunctionsComputed(t *testing.T) {
	cases := make([]struct{ name, html string }, 0, 15)
	// test_math_functions_line_height, regression test for #2812.
	for _, lineHeight := range []string{
		"calc(50% + 1em)", "calc(100% + 2px)", "calc(1.2em + 2px)", "calc(1.5 * 1)",
	} {
		cases = append(cases, struct{ name, html string }{
			"line_height " + lineHeight,
			fmt.Sprintf(`<p style="line-height: %s">Hello</p>`, lineHeight),
		})
	}
	cases = append(cases, []struct{ name, html string }{
		{"hyphenate", `
          <div lang="en"
            style="hyphens: auto; hyphenate-limit-zone: calc(1em + 100%); width: 2em">
            absolute
          </div>`},
		{"gradient", `
          <div style="width: 10px; height: 10px; background: linear-gradient(
            blue calc(20% + 1em),
            red calc(80% + 1em))"></div>`},
		{"image_min_content_calc", `
          <table><td><img src="pattern.png" style="
            height: calc(10% + 1em); width: calc(10% + 1em);
            max-height: calc(10% + 1em); max-width: calc(10% + 1em);
            min-height: calc(10% + 1em); min-width: calc(10% + 1em);">`},
		{"image_min_content_auto_width_calc", `
          <table><td><img src="pattern.png" style="
            height: calc(10% + 1em);
            max-height: calc(10% + 1em); max-width: calc(10% + 1em);
            min-height: calc(10% + 1em); min-width: calc(10% + 1em);">`},
		{"image_min_content_auto_width_height_calc", `
          <table><td><img src="pattern.png" style="
            max-height: calc(10% + 1em); max-width: calc(10% + 1em);
            min-height: calc(10% + 1em); min-width: calc(10% + 1em);">`},
		{"table_margin", `<table style="margin: calc(1em + 10%)">`},
		{"grid_padding", `
          <article style="display: grid">
            <div style="box-sizing: border-box; border: 1px solid;
                        padding: calc(2px + 10%); width: 7px">a</div>
          </article>`},
		{"table_column", `
          <table style="width: 200px">
            <colgroup style="width: calc(1em + 10%)"><col /></colgroup>
            <col style="width: calc(1em + 10%)" />
            <tbody><tr><td>a</td><td>a</td></tr></tbody>
          </table>`},
		{"border_spacing_em", `
          <table style="
              --spacing: 1em; --border-spacing: calc(var(--spacing) * 5);
              border-spacing: var(--border-spacing) var(--border-spacing)">
            <tbody><tr><td>a</td><td>a</td></tr></tbody>
          </table>`},
		{"vertical_align_table_percent", `<table><tr><td style="vertical-align: calc(1em + 1%)">abc`},
		{"vertical_align_inline_percent", `<span style="vertical-align: calc(1em + 1%)">abc`},
	}...)
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			document, sf, capture := cascade(t, c.html)
			computeAllStyles(document, sf)
			assertNoLogs(t, capture)
		})
	}
}

// TestMathFunctionsDisplaySize is test_math_functions_display_size, a
// regression test for #2673. It computes each property of each element
// instead of rendering.
func TestMathFunctionsDisplaySize(t *testing.T) {
	const sizes = `min-width: calc(50% + 1em); max-width: calc(50% + 1em); width: calc(50% + 1em);
      min-height: calc(50% + 1em); max-height: calc(50% + 1em); height: calc(50% + 1em)`
	for _, display := range []string{
		"block", "inline", "flex", "grid",
		"list", "list-item",
		"table", "table-row-group", "table-cell",
		"inline-block", "inline-table", "inline-flex", "inline-grid",
	} {
		t.Run(display, func(t *testing.T) {
			document, sf, _ := cascade(t, fmt.Sprintf(
				`<div style="display: %s; %s"><div style="%s"></div></div>`, display, sizes, sizes))
			computeAllStyles(document, sf)
		})
	}
}

// TestMathFunctionsColor is test_math_functions_color, which upstream marks
// as an expected failure.
func TestMathFunctionsColor(t *testing.T) {
	t.Skip("upstream marks the test as an expected failure: math functions in colors are not supported")
}

// TestMathFunctionsGradientColor is test_math_functions_gradient_color,
// which upstream marks as an expected failure.
func TestMathFunctionsGradientColor(t *testing.T) {
	t.Skip("upstream marks the test as an expected failure: math functions in colors are not supported")
}

// TestMathVerticalAlignPagePercent is
// test_math_vertical_align_page_percent. It computes the style of the
// margin box @top-left of the first page instead of rendering.
func TestMathVerticalAlignPagePercent(t *testing.T) {
	document, sf, capture := cascade(t, `
      <style>@page{@top-left{content: "a"; vertical-align: calc(1em + 1%)}}</style>
      <body>abc`)
	root := document.EtreeElement
	pageType := PageType{Side: "right", Name: "", Index: 0}
	sf.AddPageDeclarations(pageType)
	if err := sf.SetComputedStyles(pageType, root, root, "", document.BaseURL, nil); err != nil {
		t.Fatal(err)
	}
	if err := sf.SetComputedStyles(pageType, pageType, root, "@top-left", document.BaseURL, nil); err != nil {
		t.Fatal(err)
	}
	margin := sf.Get(pageType, "@top-left")
	if margin == nil {
		t.Fatal("no style for @top-left")
	}
	for name := range InitialValues {
		margin.Get(name)
	}
	assertNoLogs(t, capture)
}
