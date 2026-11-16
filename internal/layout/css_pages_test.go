// Copyright (c) 2011-2021, Simon Sapin and contributors.
//
// Use of this source code is governed by the BSD-3-Clause license that can be
// found in the LICENSE file.

// This file ports tests/css/test_pages.py.

package layout

import (
	"log/slog"
	"testing"

	"github.com/xo/okage/cssselect2"
	"github.com/xo/okage/tinycss2"
)

// TestPage is test_page.
func TestPage(t *testing.T) {
	log, capture := captureLogs(slog.LevelInfo)
	document := fakeHTMLFile(t, "doc1.html", log)
	styleFor := computedStyles(t, document, ComputedStylesOptions{
		UserStylesheets: []CSSSource{{Text: `
          html { color: red }
          @page { margin: 10px }
          @page :right {
            color: blue;
            margin-bottom: 12pt;
            font-size: 20px;
            @top-left { width: 10em }
            @top-right { font-size: 10px}
          }
        `}},
	})

	for _, c := range []struct {
		side             string
		index            int
		top, bottom      float64
		r, g, b          float64
		marginLeftPixels float64
	}{
		{"left", 0, 5, 10, 1, 0, 0, 10},  // red, inherited from html
		{"right", 0, 5, 16, 0, 0, 1, 10}, // blue
		{"left", 1, 10, 10, 1, 0, 0, 10}, // red, inherited from html
		{"right", 1, 10, 16, 0, 0, 1, 10},
	} {
		pageType := PageType{Side: c.side, Index: c.index}
		setPageTypeComputedStyles(t, pageType, document, styleFor)
		style := styleFor.Get(pageType, "")
		assertValue(t, style, "margin_top", Dimension{c.top, "px"})
		assertValue(t, style, "margin_left", Dimension{c.marginLeftPixels, "px"})
		assertValue(t, style, "margin_bottom", Dimension{c.bottom, "px"})
		assertRGBA(t, "color", style.Get("color"), c.r, c.g, c.b, 1)
	}

	pageType := PageType{Side: "left", Index: 0}
	setPageTypeComputedStyles(t, pageType, document, styleFor)
	if style := styleFor.Get(pageType, "@top-left"); style != nil {
		t.Error("expected no style for @top-left of a left page")
	}

	pageType = PageType{Side: "right", Index: 0}
	setPageTypeComputedStyles(t, pageType, document, styleFor)
	style := styleFor.Get(pageType, "@top-left")
	assertValue(t, style, "font_size", Int(20)) // inherited from @page
	assertValue(t, style, "width", Dimension{200, "px"})

	style = styleFor.Get(pageType, "@top-right")
	assertValue(t, style, "font_size", Int(10))

	assertNoLogs(t, capture)
}

// TestPageSelectors is test_page_selectors.
func TestPageSelectors(t *testing.T) {
	type sel struct {
		side        string
		blank       bool
		first       bool
		name        string
		index       *PageIndex
		specificity cssselect2.Specificity
	}
	for _, c := range []struct {
		style     string
		selectors []sel
	}{
		{"@page {}", []sel{{}}},
		{"@page :left {}", []sel{{side: "left", specificity: cssselect2.Specificity{0, 0, 1}}}},
		{"@page:first:left {}", []sel{{side: "left", first: true, specificity: cssselect2.Specificity{0, 1, 1}}}},
		{"@page pagename {}", []sel{{name: "pagename", specificity: cssselect2.Specificity{1, 0, 0}}}},
		{"@page pagename:first:right:blank {}", []sel{{
			side: "right", blank: true, first: true, name: "pagename",
			specificity: cssselect2.Specificity{1, 2, 1},
		}}},
		{"@page pagename, :first {}", []sel{
			{name: "pagename", specificity: cssselect2.Specificity{1, 0, 0}},
			{first: true, specificity: cssselect2.Specificity{0, 1, 0}},
		}},
		{"@page :first:first {}", []sel{{first: true, specificity: cssselect2.Specificity{0, 2, 0}}}},
		{"@page :left:left {}", []sel{{side: "left", specificity: cssselect2.Specificity{0, 0, 2}}}},
		{"@page :nth(2) {}", []sel{{index: &PageIndex{0, 2, ""}, specificity: cssselect2.Specificity{0, 1, 0}}}},
		{"@page :nth(2n + 4) {}", []sel{{index: &PageIndex{2, 4, ""}, specificity: cssselect2.Specificity{0, 1, 0}}}},
		{"@page :nth(3n) {}", []sel{{index: &PageIndex{3, 0, ""}, specificity: cssselect2.Specificity{0, 1, 0}}}},
		{"@page :nth( n+2 ) {}", []sel{{index: &PageIndex{1, 2, ""}, specificity: cssselect2.Specificity{0, 1, 0}}}},
		{"@page :nth(even) {}", []sel{{index: &PageIndex{2, 0, ""}, specificity: cssselect2.Specificity{0, 1, 0}}}},
		{"@page pagename:nth(2) {}", []sel{{
			name: "pagename", index: &PageIndex{0, 2, ""}, specificity: cssselect2.Specificity{1, 1, 0},
		}}},
		{"@page page page {}", nil},
		{"@page :left page {}", nil},
		{"@page :left, {}", nil},
		{"@page , {}", nil},
		{"@page :left, test, {}", nil},
		{"@page :wrong {}", nil},
		{"@page :left:wrong {}", nil},
		{"@page :left:right {}", nil},
	} {
		rules := tinycss2.ParseStylesheet(c.style)
		if len(rules) != 1 {
			t.Fatalf("%s: expected 1 rule, got %d", c.style, len(rules))
		}
		data, ok := parsePageSelectors(rules[0].(*tinycss2.AtRule))
		if c.selectors == nil {
			if ok {
				t.Errorf("%s: expected None, got %+v", c.style, data)
			}
			continue
		}
		if !ok || len(data) != len(c.selectors) {
			t.Errorf("%s: got %+v, %v", c.style, data, ok)
			continue
		}
		for i, want := range c.selectors {
			got := data[i]
			sameIndex := (got.types.Index == nil) == (want.index == nil)
			if sameIndex && want.index != nil {
				sameIndex = *got.types.Index == *want.index
			}
			if got.types.Side != want.side || got.types.Blank != want.blank || got.types.First != want.first ||
				got.types.Name != want.name || !sameIndex || got.specificity != want.specificity {
				t.Errorf("%s: selector %d: got %+v %v, want %+v", c.style, i, got.types, got.specificity, want)
			}
		}
	}
}

// TestNamedPages is test_named_pages. Upstream reads the styles of the
// boxes of a rendered page. The port reads the computed styles of the
// elements.
func TestNamedPages(t *testing.T) {
	document, styleFor, capture := cascade(t, `
      <style>
        @page NARRow { size: landscape }
        div { page: AUTO }
        p { page: NARRow }
      </style>
      <div><p><span>a</span></p></div>
    `)
	for _, c := range []struct{ tag, page string }{
		{"html", ""},
		{"body", ""},
		{"div", ""},
		{"p", "NARRow"},
		{"span", "NARRow"},
	} {
		assertValue(t, styleOf(t, document, styleFor, c.tag), "page", String(c.page))
	}
	assertNoLogs(t, capture)
}

// TestPageTypeMatch checks the matching of page types by the selectors of
// @page, which test_page does not cover: :blank, :nth() and the page
// groups.
func TestPageTypeMatch(t *testing.T) {
	for _, c := range []struct {
		selector PageSelectorType
		page     PageType
		want     bool
	}{
		{PageSelectorType{}, PageType{Side: "left"}, true},
		{PageSelectorType{Blank: true}, PageType{Side: "left"}, false},
		{PageSelectorType{Blank: true}, PageType{Side: "left", Blank: true}, true},
		{PageSelectorType{Name: "a"}, PageType{Name: "b"}, false},
		{PageSelectorType{Index: &PageIndex{2, 1, ""}}, PageType{Index: 0}, true},
		{PageSelectorType{Index: &PageIndex{2, 1, ""}}, PageType{Index: 1}, false},
		{PageSelectorType{Index: &PageIndex{0, 3, ""}}, PageType{Index: 2}, true},
		{PageSelectorType{Index: &PageIndex{0, 2, "g"}}, PageType{Name: "g", Groups: []PageGroup{{"g", 1}}}, true},
		{PageSelectorType{Index: &PageIndex{0, 2, "g"}}, PageType{Name: "g", Groups: []PageGroup{{"h", 1}}}, false},
		{PageSelectorType{Index: &PageIndex{0, 2, "g"}}, PageType{Name: "h", Groups: []PageGroup{{"g", 1}}}, false},
	} {
		if got := pageTypeMatch(c.selector, c.page); got != c.want {
			t.Errorf("%+v %+v: got %v, want %v", c.selector, c.page, got, c.want)
		}
	}
}
