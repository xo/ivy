// Copyright (c) 2011-2021, Simon Sapin and contributors.
//
// Use of this source code is governed by the BSD-3-Clause license that can be
// found in the LICENSE file.

// This file ports tests/css/test_fonts.py, the tests of the properties of
// the fonts.

package layout

import (
	"fmt"
	"log/slog"
	"math"
	"testing"
)

// isClose is math.isclose of Python with its default tolerance.
func isClose(a, b float64) bool {
	return math.Abs(a-b) <= 1e-9*max(math.Abs(a), math.Abs(b))
}

func TestFontSize(t *testing.T) {
	for _, c := range []struct {
		parentCSS  string
		parentSize float64
		childCSS   string
		childSize  float64
	}{
		{"10px", 10, "10px", 10},
		{"x-small", 12, "xx-large", 32},
		{"x-large", 24, "2em", 48},
		{"1em", 16, "1em", 16},
		{"1em", 16, "larger", 6.0 / 5 * 16},
		{"medium", 16, "larger", 6.0 / 5 * 16},
		{"x-large", 24, "larger", 32},
		{"xx-large", 32, "larger", 1.2 * 32},
		{"1px", 1, "larger", 3.0 / 5 * 16},
		{"28px", 28, "larger", 32},
		{"100px", 100, "larger", 120},
		{"xx-small", 3.0 / 5 * 16, "larger", 12},
		{"1em", 16, "smaller", 8.0 / 9 * 16},
		{"medium", 16, "smaller", 8.0 / 9 * 16},
		{"x-large", 24, "smaller", 6.0 / 5 * 16},
		{"xx-large", 32, "smaller", 24},
		{"xx-small", 3.0 / 5 * 16, "smaller", 0.8 * 3 / 5 * 16},
		{"1px", 1, "smaller", 0.8},
		{"28px", 28, "smaller", 24},
		{"100px", 100, "smaller", 32},
	} {
		log, capture := captureLogs(slog.LevelInfo)
		document := fakeHTML(t, "<p>a<span>b", testBaseURL, log)
		styleFor := computedStyles(t, document, ComputedStylesOptions{
			UserStylesheets: []CSSSource{{Text: fmt.Sprintf("p{font-size:%s}span{font-size:%s}", c.parentCSS, c.childCSS)}},
		})
		p := findElement(document.EtreeElement, "p")
		span := findElement(document.EtreeElement, "span")
		if got := num(styleFor.Get(p, "").Get("font_size")); !isClose(got, c.parentSize) {
			t.Errorf("%s: p font size %v, want %v", c.parentCSS, got, c.parentSize)
		}
		if got := num(styleFor.Get(span, "").Get("font_size")); !isClose(got, c.childSize) {
			t.Errorf("%s %s: span font size %v, want %v", c.parentCSS, c.childCSS, got, c.childSize)
		}
		assertNoLogs(t, capture)
	}
}

// TestLineHeightInheritance is test_line_height_inheritance. Upstream reads
// the styles of the boxes of a rendered page. The port reads the computed
// styles of the elements, as the boxes wait for a later stage, and measures
// the struts with the default metrics.
func TestLineHeightInheritance(t *testing.T) {
	log, capture := captureLogs(slog.LevelInfo)
	document := fakeHTML(t, `
      <style>
        html { font-size: 10px; line-height: 140% }
        section { font-size: 10px; line-height: 1.4 }
        div, p { font-size: 20px; vertical-align: 50% }
      </style>
      <body><div><section><p></p></section></div></body>
    `, testBaseURL, log)
	styleFor := computedStyles(t, document, ComputedStylesOptions{})
	root := document.EtreeElement
	html := styleFor.Get(root, "")
	div := styleFor.Get(findElement(root, "div"), "")
	paragraph := styleFor.Get(findElement(root, "p"), "")

	check := func(name string, got, want float64) {
		t.Helper()
		if got != want {
			t.Errorf("%s: got %v, want %v", name, got, want)
		}
	}
	check("html font_size", num(html.Get("font_size")), 10)
	check("div font_size", num(div.Get("font_size")), 20)
	// 140% of 10px = 14px is inherited from html.
	lineHeight, _ := strut(div)
	check("div strut", lineHeight, 14)
	check("div vertical_align", num(div.Get("vertical_align")), 7) // 50 % of 14px

	check("p font_size", num(paragraph.Get("font_size")), 20)
	// 1.4 is inherited from p, 1.4 * 20px on em = 28px.
	lineHeight, _ = strut(paragraph)
	check("p strut", lineHeight, 28)
	check("p vertical_align", num(paragraph.Get("vertical_align")), 14) // 50% of 28px
	assertNoLogs(t, capture)
}
