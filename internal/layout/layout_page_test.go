// Copyright (c) 2011-2021, Simon Sapin and contributors.
//
// Use of this source code is governed by the BSD-3-Clause license that can be
// found in the LICENSE file.

// This file ports tests/layout/test_page.py: the tests of the layout of the
// pages. It also holds the tests of tests/test_boxes.py that need the
// layout of the pages (the margin boxes, string-set in the margins and the
// page counters), test_page_style of tests/css/test_pages.py, and the tests
// of the lazy Document of the port.

package layout

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"testing"
)

// pageRender is render_pages with assert_no_logs: it lays out a document,
// fails the test on a warning or an error of the log, and checks that the
// document has n pages, or any number of pages for a negative n.
func pageRender(t *testing.T, source string, n int) []*Box {
	t.Helper()
	log, capture := captureLogs(slog.LevelWarn)
	pages := renderPagesLog(t, source, testBaseURL, log)
	assertNoLogs(t, capture)
	if n >= 0 && len(pages) != n {
		t.Fatalf("%d pages, want %d", len(pages), n)
	}
	return pages
}

// pageEqual fails the test when got is not want, with the tolerance of
// assertApprox.
func pageEqual(t *testing.T, name string, got, want float64) {
	t.Helper()
	assertApprox(t, name, got, want)
}

// pageTag fails the test when the tag of box is not tag.
func pageTag(t *testing.T, box *Box, tag string) {
	t.Helper()
	if box.ElementTag != tag {
		t.Errorf("tag %q, want %q", box.ElementTag, tag)
	}
}

// pageTexts returns the text of the text boxes of the descendants of box.
func pageTexts(box *Box) string {
	var b strings.Builder
	box.Descendants(func(d *Box) bool {
		if d.Kind == KindText {
			b.WriteString(d.Text)
		}
		return true
	})
	return b.String()
}

// TestPageSizeBasic is test_page_size_basic.
func TestPageSizeBasic(t *testing.T) {
	for _, c := range []struct {
		size          string
		width, height int
	}{
		{"auto", 793, 1122},
		{"2in 10in", 192, 960},
		{"242px", 242, 242},
		{"letter", 816, 1056},
		{"letter portrait", 816, 1056},
		{"letter landscape", 1056, 816},
		{"portrait", 793, 1122},
		{"landscape", 1122, 793},
	} {
		t.Run(c.size, func(t *testing.T) {
			page := pageRender(t, "<style>@page { size: "+c.size+"; }</style>", 1)[0]
			if int(page.MarginWidth()) != c.width || int(page.MarginHeight()) != c.height {
				t.Errorf("size %v×%v, want %d×%d", page.MarginWidth(), page.MarginHeight(), c.width, c.height)
			}
		})
	}
}

// TestPageSizeWithMargin is test_page_size_with_margin.
func TestPageSizeWithMargin(t *testing.T) {
	page := pageRender(t, `<style>
      @page { size: 200px 300px; margin: 10px 10% 20% 1in }
      body { margin: 8px }
    </style>
    <p style="margin: 0">`, 1)[0]
	pageEqual(t, "margin width", page.MarginWidth(), 200)
	pageEqual(t, "margin height", page.MarginHeight(), 300)
	pageEqual(t, "x", page.PositionX, 0)
	pageEqual(t, "y", page.PositionY, 0)
	pageEqual(t, "width", page.Width, 84)    // 200px - 10% - 1 inch
	pageEqual(t, "height", page.Height, 230) // 300px - 10px - 20%

	html := only(t, page)
	pageTag(t, html, "html")
	pageEqual(t, "html x", html.PositionX, 96) // 1in
	pageEqual(t, "html y", html.PositionY, 10) // root element’s margins do not collapse
	pageEqual(t, "html width", html.Width, 84)

	body := only(t, html)
	pageTag(t, body, "body")
	pageEqual(t, "body x", body.PositionX, 96) // 1in
	pageEqual(t, "body y", body.PositionY, 10)
	// body has margins in the UA stylesheet
	pageEqual(t, "body margin left", body.MarginLeft, 8)
	pageEqual(t, "body margin right", body.MarginRight, 8)
	pageEqual(t, "body margin top", body.MarginTop, 8)
	pageEqual(t, "body margin bottom", body.MarginBottom, 8)
	pageEqual(t, "body width", body.Width, 68)

	paragraph := only(t, body)
	pageTag(t, paragraph, "p")
	pageEqual(t, "p x", paragraph.PositionX, 104) // 1in + 8px
	pageEqual(t, "p y", paragraph.PositionY, 18)  // 10px + 8px
	pageEqual(t, "p width", paragraph.Width, 68)
}

// TestPageSizeWithMarginBorderPadding is
// test_page_size_with_margin_border_padding.
func TestPageSizeWithMarginBorderPadding(t *testing.T) {
	page := pageRender(t, `<style> @page {
      size: 100px; margin: 1px 2px; padding: 4px 8px;
      border-width: 16px 32px; border-style: solid;
    }</style>`, 1)[0]
	pageEqual(t, "width", page.Width, 16)   // 100 - 2 * 42
	pageEqual(t, "height", page.Height, 58) // 100 - 2 * 21
	html := only(t, page)
	pageTag(t, html, "html")
	pageEqual(t, "html x", html.PositionX, 42) // 2 + 8 + 32
	pageEqual(t, "html y", html.PositionY, 21) // 1 + 4 + 16
}

// TestPageSizeMargins is test_page_size_margins.
func TestPageSizeMargins(t *testing.T) {
	for _, c := range []struct {
		margin                   string
		top, right, bottom, left float64
	}{
		{"auto", 15, 10, 15, 10},
		{"5px 5px auto auto", 5, 5, 25, 15},
	} {
		t.Run(c.margin, func(t *testing.T) {
			page := pageRender(t, `<style>@page {
      size: 106px 206px; width: 80px; height: 170px;
      padding: 1px; border: 2px solid; margin: `+c.margin+` }</style>`, 1)[0]
			pageEqual(t, "top", page.MarginTop, c.top)
			pageEqual(t, "right", page.MarginRight, c.right)
			pageEqual(t, "bottom", page.MarginBottom, c.bottom)
			pageEqual(t, "left", page.MarginLeft, c.left)
		})
	}
}

// TestPageSizeOverConstrained is test_page_size_over_constrained.
func TestPageSizeOverConstrained(t *testing.T) {
	for _, c := range []struct {
		style         string
		width, height float64
	}{
		{"size: 4px 10000px; width: 100px; height: 100px;" +
			"padding: 1px; border: 2px solid; margin: 3px", 112, 112},
		{"size: 1000px; margin: 100px; max-width: 500px; min-height: 1500px", 700, 1700},
		{"size: 1000px; margin: 100px; min-width: 1500px; max-height: 500px", 1700, 700},
	} {
		t.Run(c.style, func(t *testing.T) {
			page := pageRender(t, "<style>@page { "+c.style+" }</style>", 1)[0]
			pageEqual(t, "margin width", page.MarginWidth(), c.width)
			pageEqual(t, "margin height", page.MarginHeight(), c.height)
		})
	}
}

// TestPageBreaks is test_page_breaks.
func TestPageBreaks(t *testing.T) {
	for _, html := range []string{
		"<div>1</div>",
		"<div></div>",
		"<img src=pattern.png>",
	} {
		t.Run(html, func(t *testing.T) {
			pages := pageRender(t, `
      <style>
        @page { size: 100px; margin: 10px }
        body { margin: 0 }
        div { height: 30px; font-size: 20px }
        img { height: 30px; display: block }
      </style>
      `+strings.Repeat(html, 5), -1)
			var got [][]float64
			for _, page := range pages {
				_, body := pageBody(t, page)
				var ys []float64
				for _, child := range body.Children {
					if child.ElementTag != "div" && child.ElementTag != "img" {
						t.Errorf("child %s", child)
					}
					if child.PositionX != 10 {
						t.Errorf("child x %v", child.PositionX)
					}
					ys = append(ys, child.PositionY)
				}
				got = append(got, ys)
			}
			if fmt.Sprint(got) != "[[10 40] [10 40] [10]]" {
				t.Errorf("positions %v", got)
			}
		})
	}
}

// TestPageBreaksBoxSplit is test_page_breaks_box_split.
func TestPageBreaksBoxSplit(t *testing.T) {
	// If floats round the wrong way, a block that gets filled to the end of
	// a page due to breaking over the page may be forced onto the next page
	// because it is slightly taller than can fit on the previous page, even
	// if it wouldn't have been without being filled. These numbers aren't
	// ideal, but they do seem to trigger the issue.
	pages := pageRender(t, `
      <style>
        @page { size: 982.4146981627297px; margin: 0 }
        div { font-size: 5px; height: 200.0123456789px; margin: 0; padding: 0 }
        figure { margin: 0; padding: 0 }
      </style>
      <div>text</div>
      <div>text</div><!-- no page break here -->
      <section>
        <div>line1</div>
        <div>line2</div><!-- page break here -->
        <div>line3</div>
        <div>line4</div>
      </section>
    `, 2)
	_, body := pageBody(t, pages[0])
	section := unpack(t, body, 3)[2]
	unpack(t, section, 2)

	_, body = pageBody(t, pages[1])
	section = only(t, body)
	unpack(t, section, 2)
}

// TestPageBreaksComplex1 is test_page_breaks_complex_1.
func TestPageBreaksComplex1(t *testing.T) {
	pages := pageRender(t, `
      <style>
        @page { margin: 10px }
        @page :left { margin-left: 50px }
        @page :right { margin-right: 50px }
        html { page-break-before: left }
        div { page-break-after: left }
        ul { page-break-before: always }
      </style>
      <div>1</div>
      <p>2</p>
      <p>3</p>
      <article>
        <section>
          <ul><li>4</li></ul>
        </section>
      </article>
    `, 4)

	// The first page is a right page on rtl, but not here because of
	// page-break-before on the root element.
	pageEqual(t, "page 1 margin left", pages[0].MarginLeft, 50) // left page
	pageEqual(t, "page 1 margin right", pages[0].MarginRight, 10)
	_, body := pageBody(t, pages[0])
	div := only(t, body)
	line := only(t, div)
	text := only(t, line)
	pageTag(t, div, "div")
	if text.Text != "1" {
		t.Errorf("text %q", text.Text)
	}

	html := only(t, pages[1])
	pageEqual(t, "page 2 margin left", pages[1].MarginLeft, 10)
	pageEqual(t, "page 2 margin right", pages[1].MarginRight, 50) // right page
	unpack(t, html, 0)                                            // empty page to get to a left page

	pageEqual(t, "page 3 margin left", pages[2].MarginLeft, 50) // left page
	pageEqual(t, "page 3 margin right", pages[2].MarginRight, 10)
	_, body = pageBody(t, pages[2])
	ps := unpack(t, body, 2)
	pageTag(t, ps[0], "p")
	pageTag(t, ps[1], "p")

	pageEqual(t, "page 4 margin left", pages[3].MarginLeft, 10)
	pageEqual(t, "page 4 margin right", pages[3].MarginRight, 50) // right page
	_, body = pageBody(t, pages[3])
	article := only(t, body)
	section := only(t, article)
	ulist := only(t, section)
	pageTag(t, ulist, "ul")
}

// TestPageBreaksComplex2 is test_page_breaks_complex_2.
func TestPageBreaksComplex2(t *testing.T) {
	// Reference for the following test:
	// Without any 'avoid', this breaks after the <div>
	pages := pageRender(t, `
      <style>
        @page { size: 140px; margin: 0 }
        img { height: 25px; vertical-align: top }
      </style>
      <img src=pattern.png>
      <div>
        <p><img src=pattern.png><br/><img src=pattern.png><p>
        <p><img src=pattern.png><br/><img src=pattern.png><p>
      </div><!-- page break here -->
      <img src=pattern.png>
    `, 2)
	_, body := pageBody(t, pages[0])
	children := unpack(t, body, 2)
	img1, div := children[0], children[1]
	pageEqual(t, "img 1 y", img1.PositionY, 0)
	pageEqual(t, "img 1 height", img1.Height, 25)
	pageEqual(t, "div y", div.PositionY, 25)
	pageEqual(t, "div height", div.Height, 100)

	_, body = pageBody(t, pages[1])
	img2 := only(t, body)
	pageEqual(t, "img 2 y", img2.PositionY, 0)
	pageEqual(t, "img 2 height", img2.Height, 25)
}

// TestPageBreaksComplex3 is test_page_breaks_complex_3.
func TestPageBreaksComplex3(t *testing.T) {
	// Adding a few page-break-*: avoid, the only legal break is
	// before the <div>
	pages := pageRender(t, `
      <style>
        @page { size: 140px; margin: 0 }
        img { height: 25px; vertical-align: top }
      </style>
      <img src=pattern.png><!-- page break here -->
      <div>
        <p style="page-break-inside: avoid">
          <img src=pattern.png><br/><img src=pattern.png></p>
        <p style="page-break-before: avoid; page-break-after: avoid; widows: 2"
          ><img src=pattern.png><br/><img src=pattern.png></p>
      </div>
      <img src=pattern.png>
    `, 2)
	_, body := pageBody(t, pages[0])
	img1 := only(t, body)
	pageEqual(t, "img 1 y", img1.PositionY, 0)
	pageEqual(t, "img 1 height", img1.Height, 25)

	_, body = pageBody(t, pages[1])
	children := unpack(t, body, 2)
	div, img2 := children[0], children[1]
	pageEqual(t, "div y", div.PositionY, 0)
	pageEqual(t, "div height", div.Height, 100)
	pageEqual(t, "img 2 y", img2.PositionY, 100)
	pageEqual(t, "img 2 height", img2.Height, 25)
}

// TestPageBreaksComplex4 is test_page_breaks_complex_4.
func TestPageBreaksComplex4(t *testing.T) {
	pages := pageRender(t, `
      <style>
        @page { size: 140px; margin: 0 }
        img { height: 25px; vertical-align: top }
      </style>
      <img src=pattern.png><!-- page break here -->
      <div>
        <div>
          <p style="page-break-inside: avoid">
            <img src=pattern.png><br/><img src=pattern.png></p>
          <p style="page-break-before:avoid; page-break-after:avoid; widows:2"
            ><img src=pattern.png><br/><img src=pattern.png></p>
        </div>
        <img src=pattern.png>
      </div>
    `, 2)
	_, body := pageBody(t, pages[0])
	img1 := only(t, body)
	pageEqual(t, "img 1 y", img1.PositionY, 0)
	pageEqual(t, "img 1 height", img1.Height, 25)

	_, body = pageBody(t, pages[1])
	outerDiv := only(t, body)
	children := unpack(t, outerDiv, 2)
	innerDiv, img2 := children[0], children[1]
	pageEqual(t, "inner div y", innerDiv.PositionY, 0)
	pageEqual(t, "inner div height", innerDiv.Height, 100)
	pageEqual(t, "img 2 y", img2.PositionY, 100)
	pageEqual(t, "img 2 height", img2.Height, 25)
}

// TestPageBreaksComplex5 is test_page_breaks_complex_5.
func TestPageBreaksComplex5(t *testing.T) {
	// Reference for the next test
	pages := pageRender(t, `
      <style>
        @page { size: 100px; margin: 0 }
        img { height: 30px; display: block; }
      </style>
      <div>
        <img src=pattern.png style="page-break-after: always">
        <section>
          <img src=pattern.png>
          <img src=pattern.png>
        </section>
      </div>
      <img src=pattern.png><!-- page break here -->
      <img src=pattern.png>
    `, 3)
	_, body := pageBody(t, pages[0])
	div := only(t, body)
	pageEqual(t, "div height", div.Height, 100)
	_, body = pageBody(t, pages[1])
	children := unpack(t, body, 2)
	pageEqual(t, "div height", children[0].Height, 60)
	pageEqual(t, "img 4 height", children[1].Height, 30)
	_, body = pageBody(t, pages[2])
	pageEqual(t, "img 5 height", only(t, body).Height, 30)
}

// TestPageBreaksComplex6 is test_page_breaks_complex_6.
func TestPageBreaksComplex6(t *testing.T) {
	pages := pageRender(t, `
      <style>
        @page { size: 100px; margin: 0 }
        img { height: 30px; display: block; }
      </style>
      <div>
        <img src=pattern.png style="page-break-after: always">
        <section>
          <img src=pattern.png><!-- page break here -->
          <img src=pattern.png style="page-break-after: avoid">
        </section>
      </div>
      <img src=pattern.png style="page-break-after: avoid">
      <img src=pattern.png>
    `, 3)
	_, body := pageBody(t, pages[0])
	div := only(t, body)
	pageEqual(t, "div height", div.Height, 100)
	_, body = pageBody(t, pages[1])
	div = only(t, body)
	section := only(t, div)
	img2 := only(t, section)
	pageEqual(t, "img 2 height", img2.Height, 30)
	// TODO: currently this is 60: we do not increase the used height of
	// blocks to make them fill the blank space at the end of the age when
	// we remove children from them for some break-*: avoid.
	// See TODOs in blocks.block_container_layout
	// assert div.height == 100
	_, body = pageBody(t, pages[2])
	children := unpack(t, body, 3)
	pageEqual(t, "div height", children[0].Height, 30)
	pageEqual(t, "img 4 height", children[1].Height, 30)
	pageEqual(t, "img 5 height", children[2].Height, 30)
}

// TestPageBreaksComplex7 is test_page_breaks_complex_7.
func TestPageBreaksComplex7(t *testing.T) {
	pages := pageRender(t, `
      <style>
        @page { @bottom-center { content: counter(page) } }
        @page:blank { @bottom-center { content: none } }
      </style>
      <p style="page-break-after: right">foo</p>
      <p>bar</p>
    `, 3)
	unpack(t, pages[0], 2) // content and @bottom-center
	unpack(t, pages[1], 1) // content only
	unpack(t, pages[2], 2) // content and @bottom-center
}

// TestPageBreaksComplex8 is test_page_breaks_complex_8.
func TestPageBreaksComplex8(t *testing.T) {
	pages := pageRender(t, `
      <style>
        @page { size: 75px; margin: 0 }
        div { height: 20px }
      </style>
      <div></div>
      <section>
        <div></div>
        <div style="page-break-after: avoid">
          <div style="position: absolute"></div>
          <div style="position: fixed"></div>
        </div>
      </section>
      <div></div>
    `, 2)
	html := only(t, pages[0])
	body := unpack(t, html, 2)[0]
	children := unpack(t, body, 2)
	div1, section := children[0], children[1]
	div2 := only(t, section)
	pageEqual(t, "div 1 y", div1.PositionY, 0)
	pageEqual(t, "div 2 y", div2.PositionY, 20)
	pageEqual(t, "div 1 height", div1.Height, 20)
	pageEqual(t, "div 2 height", div2.Height, 20)
	_, body = pageBody(t, pages[1])
	children = unpack(t, body, 2)
	section, div4 := children[0], children[1]
	div3 := only(t, section)
	unpack(t, div3, 2) // absolute, fixed
	pageEqual(t, "div 3 y", div3.PositionY, 0)
	pageEqual(t, "div 4 y", div4.PositionY, 20)
	pageEqual(t, "div 3 height", div3.Height, 20)
	pageEqual(t, "div 4 height", div4.Height, 20)
}

// TestPageBreaksComplex9 is test_page_breaks_complex_9.
func TestPageBreaksComplex9(t *testing.T) {
	// Regression test for #1979.
	pages := pageRender(t, `
      <style>
        @page { size: 75px; margin: 0 }
        div { height: 20px; margin: 10px }
      </style>
      <div style="height: 40px"></div>
      <div></div>
      <div style="break-before: left"></div>
      <div style="break-before: right"></div>
    `, 5)
	check := func(page *Box, x, y float64) {
		t.Helper()
		_, body := pageBody(t, page)
		div := only(t, body)
		pageEqual(t, "x", div.ContentBoxX(), x)
		pageEqual(t, "y", div.ContentBoxY(), y)
	}
	check(pages[0], 10, 10)
	check(pages[1], 10, 0) // Unforced page break
	unpack(t, only(t, pages[2]), 0)
	check(pages[3], 10, 10) // Forced page break
	check(pages[4], 10, 10) // Forced page break
}

// TestMarginBreak is test_margin_break.
func TestMarginBreak(t *testing.T) {
	for _, c := range []struct {
		breakAfter, marginBreak string
		marginTop               float64
	}{
		{"page", "auto", 5},
		{"auto", "auto", 0},
		{"page", "keep", 5},
		{"auto", "keep", 5},
		{"page", "discard", 0},
		{"auto", "discard", 0},
	} {
		t.Run(c.breakAfter+" "+c.marginBreak, func(t *testing.T) {
			pages := pageRender(t, `
      <style>
        @page { size: 70px; margin: 0 }
        div { height: 63px; margin: 5px 0 8px;
              break-after: `+c.breakAfter+`; margin-break: `+c.marginBreak+` }
      </style>
      <section>
        <div></div>
      </section>
      <section>
        <div></div>
      </section>
    `, 2)
			_, body := pageBody(t, pages[0])
			pageEqual(t, "page 1 margin top", only(t, only(t, body)).MarginTop, 5)
			_, body = pageBody(t, pages[1])
			pageEqual(t, "page 2 margin top", only(t, only(t, body)).MarginTop, c.marginTop)
		})
	}
}

// TestMarginBreakClearance is test_margin_break_clearance.
func TestMarginBreakClearance(t *testing.T) {
	t.Skip("upstream marks the test as an expected failure")
}

// TestRectoVersoBreak is test_recto_verso_break.
func TestRectoVersoBreak(t *testing.T) {
	for _, c := range []struct {
		direction, pageBreak string
		pagesNumber          int
	}{
		{"ltr", "recto", 3},
		{"ltr", "verso", 2},
		{"rtl", "recto", 3},
		{"rtl", "verso", 2},
		{"ltr", "right", 3},
		{"ltr", "left", 2},
		{"rtl", "right", 2},
		{"rtl", "left", 3},
	} {
		t.Run(c.direction+" "+c.pageBreak, func(t *testing.T) {
			pageRender(t, `
      <style>
        html { direction: `+c.direction+` }
        p { break-before: `+c.pageBreak+` }
      </style>
      abc
      <p>def</p>
    `, c.pagesNumber)
		})
	}
}

// TestRectoVersoBreakRoot is test_recto_verso_break_root.
func TestRectoVersoBreakRoot(t *testing.T) {
	for _, c := range []struct {
		direction, pageBreak, firstPage string
	}{
		{"ltr", "recto", "right"},
		{"ltr", "verso", "left"},
		{"rtl", "recto", "left"},
		{"rtl", "verso", "right"},
		{"ltr", "right", "right"},
		{"ltr", "left", "left"},
		{"rtl", "right", "right"},
		{"rtl", "left", "left"},
	} {
		t.Run(c.direction+" "+c.pageBreak, func(t *testing.T) {
			page := pageRender(t, `
      <style>
        @page:left { size: 4px /* len('left') */ }
        @page:right { size: 5px /* len('right') */ }
        html { direction: `+c.direction+`; break-before: `+c.pageBreak+` }
      </style>
      abc
    `, 1)[0]
			pageEqual(t, "width", page.Width, float64(len(c.firstPage)))
		})
	}
}

// pageSize fails the test when the size of page is not width × height.
func pageSize(t *testing.T, page *Box, width, height float64) {
	t.Helper()
	if page.Width != width || page.Height != height {
		t.Errorf("page size %v×%v, want %v×%v", page.Width, page.Height, width, height)
	}
}

// TestPageNames1 is test_page_names_1.
func TestPageNames1(t *testing.T) {
	pages := pageRender(t, `
      <style>
        @page { size: 100px 100px }
        section { page: small }
      </style>
      <div>
        <section>large</section>
      </div>
    `, 1)
	pageSize(t, pages[0], 100, 100)
}

// TestPageNames2 is test_page_names_2.
func TestPageNames2(t *testing.T) {
	pages := pageRender(t, `
      <style>
        @page { size: 100px 100px }
        @page narrow { margin: 1px }
        section { page: small }
      </style>
      <div>
        <section>large</section>
      </div>
    `, 1)
	pageSize(t, pages[0], 100, 100)
}

// TestPageNames3 is test_page_names_3.
func TestPageNames3(t *testing.T) {
	pages := pageRender(t, `
      <style>
        @page { margin: 0 }
        @page narrow { size: 100px 200px }
        @page large { size: 200px 100px }
        div { page: narrow }
        section { page: large }
      </style>
      <div>
        <section>large</section>
        <section>large</section>
        <p>narrow</p>
      </div>
    `, 2)

	pageSize(t, pages[0], 200, 100)
	_, body := pageBody(t, pages[0])
	div := only(t, body)
	sections := unpack(t, div, 2)
	pageTag(t, sections[0], "section")
	pageTag(t, sections[1], "section")

	pageSize(t, pages[1], 100, 200)
	_, body = pageBody(t, pages[1])
	div = only(t, body)
	pageTag(t, only(t, div), "p")
}

// TestPageNames4 is test_page_names_4.
func TestPageNames4(t *testing.T) {
	pages := pageRender(t, `
      <style>
        @page { size: 200px 200px; margin: 0 }
        @page small { size: 100px 100px }
        p { page: small }
      </style>
      <section>normal</section>
      <section>normal</section>
      <p>small</p>
      <section>small</section>
    `, 2)

	pageSize(t, pages[0], 200, 200)
	_, body := pageBody(t, pages[0])
	sections := unpack(t, body, 2)
	pageTag(t, sections[0], "section")
	pageTag(t, sections[1], "section")

	pageSize(t, pages[1], 100, 100)
	_, body = pageBody(t, pages[1])
	children := unpack(t, body, 2)
	pageTag(t, children[0], "p")
	pageTag(t, children[1], "section")
}

// TestPageNames5 is test_page_names_5.
func TestPageNames5(t *testing.T) {
	pages := pageRender(t, `
      <style>
        @page { size: 200px 200px; margin: 0 }
        @page small { size: 100px 100px }
        div { page: small }
      </style>
      <section><p>a</p>b</section>
      <section>c<div>d</div></section>
    `, 2)

	pageSize(t, pages[0], 200, 200)
	_, body := pageBody(t, pages[0])
	sections := unpack(t, body, 2)
	pageTag(t, sections[0], "section")
	pageTag(t, sections[1], "section")
	unpack(t, sections[0], 2) // p, line
	only(t, sections[1])      // line

	pageSize(t, pages[1], 100, 100)
	_, body = pageBody(t, pages[1])
	section2 := only(t, body)
	only(t, section2) // div
}

// TestPageNames6 is test_page_names_6.
func TestPageNames6(t *testing.T) {
	pages := pageRender(t, `
      <style>
        @page { margin: 0 }
        @page large { size: 200px 200px }
        @page small { size: 100px 100px }
        section { page: large }
        div { page: small }
      </style>
      <section>a<p>b</p>c</section>
      <section>d<div>e</div>f</section>
    `, 3)

	pageSize(t, pages[0], 200, 200)
	_, body := pageBody(t, pages[0])
	sections := unpack(t, body, 2)
	pageTag(t, sections[0], "section")
	pageTag(t, sections[1], "section")
	unpack(t, sections[0], 3) // line1, p, line2
	only(t, sections[1])      // line

	pageSize(t, pages[1], 100, 100)
	_, body = pageBody(t, pages[1])
	only(t, only(t, body)) // section2, div

	pageSize(t, pages[2], 200, 200)
	_, body = pageBody(t, pages[2])
	only(t, only(t, body)) // section2, line
}

// TestPageNames7 is test_page_names_7.
func TestPageNames7(t *testing.T) {
	pages := pageRender(t, `
      <style>
        @page { size: 200px 200px; margin: 0 }
        @page small { size: 100px 100px }
        p { page: small; break-before: right }
      </style>
      <section>normal</section>
      <section>normal</section>
      <p>small</p>
      <section>small</section>
    `, 3)

	pageSize(t, pages[0], 200, 200)
	_, body := pageBody(t, pages[0])
	sections := unpack(t, body, 2)
	pageTag(t, sections[0], "section")
	pageTag(t, sections[1], "section")

	pageSize(t, pages[1], 200, 200)
	unpack(t, only(t, pages[1]), 0)

	pageSize(t, pages[2], 100, 100)
	_, body = pageBody(t, pages[2])
	children := unpack(t, body, 2)
	pageTag(t, children[0], "p")
	pageTag(t, children[1], "section")
}

// TestPageNames8 is test_page_names_8.
func TestPageNames8(t *testing.T) {
	pages := pageRender(t, `
      <style>
        @page small { size: 100px 100px }
        section { page: small }
        p { line-height: 80px }
      </style>
      <section>
        <p>small</p>
        <p>small</p>
      </section>
    `, 2)
	for _, page := range pages {
		pageSize(t, page, 100, 100)
		_, body := pageBody(t, page)
		section := only(t, body)
		p := only(t, section)
		pageTag(t, section, "section")
		pageTag(t, p, "p")
	}
}

// TestPageNames9 is test_page_names_9.
func TestPageNames9(t *testing.T) {
	pages := pageRender(t, `
      <style>
        @page { size: 200px 200px }
        @page small { size: 100px 100px }
        section { break-after: page; page: small }
        article { page: small }
      </style>
      <section>
        <div>big</div>
        <div>big</div>
      </section>
      <article>
        <div>small</div>
        <div>small</div>
      </article>
    `, 2)
	pageSize(t, pages[0], 100, 100)
	_, body := pageBody(t, pages[0])
	pageTag(t, only(t, body), "section")
	pageSize(t, pages[1], 100, 100)
	_, body = pageBody(t, pages[1])
	pageTag(t, only(t, body), "article")
}

// TestPageNames10 is test_page_names_10.
func TestPageNames10(t *testing.T) {
	pages := pageRender(t, `
      <style>
        #running { position: running(running); }
        #fixed { position: fixed; }
        @page { size: 200px 200px; @top-center { content: element(header); }}
        section { page: small; }
        @page small { size: 100px 100px; }
        .pagebreak { break-after: page; }
      </style>
      <div id="running">running</div>
      <div id="fixed">fixed</div>
      <section>
        <h1>text</h1>
        <div class="pagebreak"></div>
        <article>text</article>
      </section>
    `, 2)

	pageSize(t, pages[0], 100, 100)
	html := unpack(t, pages[0], 2)[0]
	body := only(t, html)
	section := unpack(t, body, 2)[1]
	h1 := unpack(t, section, 2)[0]
	pageTag(t, h1, "h1")

	pageSize(t, pages[1], 100, 100)
	html = unpack(t, pages[1], 2)[0]
	body = unpack(t, html, 2)[1]
	section = only(t, body)
	pageTag(t, only(t, section), "article")
}

// pageFirst returns page.children[0].children[0].children: the children of
// the body of a page.
func pageFirst(t *testing.T, page *Box) []*Box {
	t.Helper()
	if len(page.Children) == 0 || len(page.Children[0].Children) == 0 {
		t.Fatalf("page without a body: %v", page.Children)
	}
	return page.Children[0].Children[0].Children
}

// TestPageGroups is test_page_groups.
func TestPageGroups(t *testing.T) {
	pages := pageRender(t, `
      <style>
        @page { size: 200px 200px }
        @page small { size: 100px 100px }
        @page :nth(1 of small) { size: 50px 50px }
        section { page: small }
        div, div section { break-after: page }
      </style>
      <div></div>
      <article></article>
      <section>
        <div></div>
        <div></div>
      </section>
      <section>
      </section>
      <div></div>
      <div></div>
      <section>
        <div></div>
      </section>
      <div>
        <section></section>
        <section></section>
      </div>
    `, 9)
	tags := func(page *Box, want ...string) []*Box {
		t.Helper()
		children := pageFirst(t, page)
		if len(children) != len(want) {
			t.Fatalf("children %v, want %v", children, want)
		}
		for i, child := range children {
			pageTag(t, child, want[i])
		}
		return children
	}

	pageSize(t, pages[0], 200, 200)
	tags(pages[0], "div")
	pageSize(t, pages[1], 200, 200)
	tags(pages[1], "article")
	pageSize(t, pages[2], 50, 50)
	pageTag(t, only(t, tags(pages[2], "section")[0]), "div")
	pageSize(t, pages[3], 100, 100)
	pageTag(t, only(t, tags(pages[3], "section")[0]), "div")
	pageSize(t, pages[4], 50, 50)
	tags(pages[4], "section", "div")
	pageSize(t, pages[5], 200, 200)
	tags(pages[5], "div")
	pageSize(t, pages[6], 50, 50)
	pageTag(t, only(t, tags(pages[6], "section")[0]), "div")
	pageSize(t, pages[7], 50, 50)
	pageTag(t, only(t, tags(pages[7], "div")[0]), "section")
	pageSize(t, pages[8], 50, 50)
	pageTag(t, only(t, tags(pages[8], "div")[0]), "section")
}

// TestPageGroupsBlankInside is test_page_groups_blank_inside.
func TestPageGroupsBlankInside(t *testing.T) {
	// Regression test for #1076.
	pages := pageRender(t, `
      <style>
        @page { size: 100px }
        @page div { size: 50px }
        div { page: div }
        p { break-before: right }
      </style>
      <div>
        <p>1</p>
        <p>2</p>
      </div>
    `, 3)
	for _, page := range pages {
		pageSize(t, page, 50, 50)
	}
}

// TestPageGroupsBlankOutside is test_page_groups_blank_outside.
func TestPageGroupsBlankOutside(t *testing.T) {
	pages := pageRender(t, `
      <style>
        @page { size: 100px }
        @page p { size: 50px }
        p { page: p; break-before: right }
      </style>
      <div>
        <p>1</p>
        <p>2</p>
      </div>
    `, 3)
	pageSize(t, pages[0], 50, 50)
	pageSize(t, pages[2], 50, 50)
	pageSize(t, pages[1], 100, 100)
}

// TestPageGroupsFirstNth is test_page_groups_first_nth.
func TestPageGroupsFirstNth(t *testing.T) {
	// Regression test for #2429.
	pages := pageRender(t, `
      <style>
        @page { size: 100px }
        @page div { size: 50px }
        @page :nth(2n+1 of div) { size: 30px }
        div { page: div; break-before: right }
        p { break-before: page }
      </style>
      <div>
        <p>1</p>
        <p>2</p>
        <p>3</p>
      </div>
      <div>
        <p>4</p>
        <p>5</p>
      </div>
    `, 6)
	for _, i := range []int{0, 2, 4} {
		pageSize(t, pages[i], 30, 30)
	}
	for _, i := range []int{1, 5} {
		pageSize(t, pages[i], 50, 50)
	}
	pageSize(t, pages[3], 100, 100)
}

// TestPageGroupsCounters is test_page_groups_counters.
func TestPageGroupsCounters(t *testing.T) {
	// Regression test for #2485.
	pages := pageRender(t, `
      <style>
        html { counter-reset: h1-counter }
        h1 { page-break-before: always; counter-increment: h1-counter; }
        div.main-content { page: main-content-page-group; break-before: page; }
        @page { size: 1000px }
        @page :nth(1 of main-content-page-group) { counter-reset: page 1; size: 500px; }
        a::after {
          content: target-counter(attr(href), h1-counter) '.'
          target-counter(attr(href), page);
        }
      </style>
      <html>
      <body>
        INTRO PAGE
        <div class="main-content">
        <h1 id="t2">First title</h1>
        <h1>Title3</h1>
        <a href="#t2">Second title</a>
        </div>
      </body>
      </html>
    `, 3)
	pageSize(t, pages[0], 1000, 1000)
	pageSize(t, pages[1], 500, 500)
	pageSize(t, pages[2], 1000, 1000)

	pages = pageRender(t, `
      <!DOCTYPE html>
      <html>
        <head>
          <style>
            @page { size: 1000px }
            div { page: group }
            h1 { break-before: page }
            p.counter::after { content: counter(pages) }
            @page :nth(1 of group) { background-color: blue; size: 500px; }
          </style>
        </head>
        <body>
          INTRO PAGE
          <div>
            <h1>Title</h1>
            <p>content</p>
            <h1>Title</h1>
            <p class="counter">content</p>
          </div>
          <div>
            <h1>Title</h1>
            <p>content</p>
            <h1>Title</h1>
            <p class="counter">content</p>
          </div>
        </body>
      </html>
    `, 5)
	for i, size := range []float64{1000, 500, 1000, 500, 1000} {
		pageSize(t, pages[i], size, size)
	}
}

// TestOrphansWidowsAvoid is test_orphans_widows_avoid.
func TestOrphansWidowsAvoid(t *testing.T) {
	for _, c := range []struct {
		style      string
		lineCounts []int
	}{
		{"orphans: 2; widows: 2", []int{4, 3}},
		{"orphans: 5; widows: 2", []int{0, 7}},
		{"orphans: 2; widows: 4", []int{3, 4}},
		{"orphans: 4; widows: 4", []int{0, 7}},
		{"orphans: 2; widows: 2; page-break-inside: avoid", []int{0, 7}},
	} {
		t.Run(c.style, func(t *testing.T) {
			pages := pageRender(t, `
      <style>
        @page { size: 200px }
        h1 { height: 120px }
        p { line-height: 20px;
            width: 1px; /* line break at each word */
            `+c.style+` }
      </style>
      <h1>Tasty test</h1>
      <!-- There is room for 4 lines after h1 on the fist page -->
      <p>one two three four five six seven</p>
    `, len(c.lineCounts))
			for i, page := range pages {
				_, body := pageBody(t, page)
				bodyChildren := body.Children
				if i == 0 {
					bodyChildren = bodyChildren[1:] // skip h1
				}
				count := 0
				if len(bodyChildren) > 0 {
					count = len(bodyChildren[0].Children)
				}
				if count != c.lineCounts[i] {
					t.Errorf("page %d: %d lines, want %d", i, count, c.lineCounts[i])
				}
			}
		})
	}
}

// TestPageAndLineboxBreaking is test_page_and_linebox_breaking.
func TestPageAndLineboxBreaking(t *testing.T) {
	// Empty <span/> tests a corner case in skip_first_whitespace()
	pages := pageRender(t, `
      <style>
        @page { size: 100px; margin: 2px; border: 1px solid }
        body { margin: 0 }
        div { font-family: weasyprint; font-size: 20px }
      </style>
      <div><span/>1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15</div>
    `, 4)
	var texts strings.Builder
	for _, page := range pages {
		_, body := pageBody(t, page)
		div := only(t, body)
		for _, line := range div.Children {
			texts.WriteString(pageTexts(line))
		}
	}
	if texts.String() != "1,2,3,4,5,6,7,8,9,10,11,12,13,14,15" {
		t.Errorf("texts %q", texts.String())
	}
}

// pageMarginBox checks the position and the margin size of a margin box.
func pageMarginBox(t *testing.T, box *Box, x, y, width, height float64) {
	t.Helper()
	pageEqual(t, box.AtKeyword+" x", box.PositionX, x)
	pageEqual(t, box.AtKeyword+" y", box.PositionY, y)
	pageEqual(t, box.AtKeyword+" margin width", box.MarginWidth(), width)
	pageEqual(t, box.AtKeyword+" margin height", box.MarginHeight(), height)
}

// TestMarginBoxesFixedDimension1 is test_margin_boxes_fixed_dimension_1.
func TestMarginBoxesFixedDimension1(t *testing.T) {
	// Corner boxes
	page := pageRender(t, `
      <style>
        @page {
          @top-left-corner {
            content: 'top_left';
            padding: 10px;
          }
          @top-right-corner {
            content: 'top_right';
            padding: 10px;
          }
          @bottom-left-corner {
            content: 'bottom_left';
            padding: 10px;
          }
          @bottom-right-corner {
            content: 'bottom_right';
            padding: 10px;
          }
          size: 1000px;
          margin-top: 10%;
          margin-bottom: 40%;
          margin-left: 20%;
          margin-right: 30%;
        }
      </style>
    `, 1)[0]
	children := unpack(t, page, 5)
	topLeft, topRight, bottomLeft, bottomRight := children[1], children[2], children[3], children[4]
	for i, text := range []string{"top_left", "top_right", "bottom_left", "bottom_right"} {
		line := only(t, children[i+1])
		if got := only(t, line).Text; got != text {
			t.Errorf("text %q, want %q", got, text)
		}
	}

	// Check positioning and Rule 1 for fixed dimensions
	pageMarginBox(t, topLeft, 0, 0, 200, 100)         // margin-left, margin-top
	pageMarginBox(t, topRight, 700, 0, 300, 100)      // size-x - margin-right
	pageMarginBox(t, bottomLeft, 0, 600, 200, 400)    // size-y - margin-bottom
	pageMarginBox(t, bottomRight, 700, 600, 300, 400) // size-x - margin-right, size-y - margin-bottom
}

// pageOnlyMarginBox renders a page with one margin box and returns it.
func pageOnlyMarginBox(t *testing.T, source string) *Box {
	t.Helper()
	page := pageRender(t, source, 1)[0]
	return unpack(t, page, 2)[1]
}

// TestMarginBoxesFixedDimension2 is test_margin_boxes_fixed_dimension_2.
func TestMarginBoxesFixedDimension2(t *testing.T) {
	// Test rules 2 and 3
	box := pageOnlyMarginBox(t, `
      <style>
        @page {
          margin: 100px 200px;
          @bottom-left-corner { content: ""; margin: 60px }
        }
      </style>
    `)
	pageEqual(t, "margin width", box.MarginWidth(), 200)
	pageEqual(t, "margin left", box.MarginLeft, 60)
	pageEqual(t, "margin right", box.MarginRight, 60)
	pageEqual(t, "width", box.Width, 80) // 200 - 60 - 60

	pageEqual(t, "margin height", box.MarginHeight(), 100)
	// total was too big, the outside margin was ignored:
	pageEqual(t, "margin top", box.MarginTop, 60)
	pageEqual(t, "margin bottom", box.MarginBottom, 40) // Not 60
	pageEqual(t, "height", box.Height, 0)               // But not negative
}

// TestMarginBoxesFixedDimension3 is test_margin_boxes_fixed_dimension_3.
func TestMarginBoxesFixedDimension3(t *testing.T) {
	// Test rule 3 with a non-auto inner dimension
	box := pageOnlyMarginBox(t, `
      <style>
        @page {
          margin: 100px;
          @left-middle { content: ""; margin: 10px; width: 130px }
        }
      </style>
    `)
	pageEqual(t, "margin width", box.MarginWidth(), 100)
	pageEqual(t, "margin left", box.MarginLeft, -40) // Not 10px
	pageEqual(t, "margin right", box.MarginRight, 10)
	pageEqual(t, "width", box.Width, 130) // As specified
}

// TestMarginBoxesFixedDimension4 is test_margin_boxes_fixed_dimension_4.
func TestMarginBoxesFixedDimension4(t *testing.T) {
	// Test rule 4
	box := pageOnlyMarginBox(t, `
      <style>
        @page {
          margin: 100px;
          @left-bottom {
            content: "";
            margin-left: 10px;
            margin-right: auto;
            width: 70px;
          }
        }
      </style>
    `)
	pageEqual(t, "margin width", box.MarginWidth(), 100)
	pageEqual(t, "margin left", box.MarginLeft, 10) // 10px this time, no over-constrain
	pageEqual(t, "margin right", box.MarginRight, 20)
	pageEqual(t, "width", box.Width, 70) // As specified
}

// TestMarginBoxesFixedDimension5 is test_margin_boxes_fixed_dimension_5.
func TestMarginBoxesFixedDimension5(t *testing.T) {
	// Test rules 2, 3 and 4
	box := pageOnlyMarginBox(t, `
      <style>
        @page {
          margin: 100px;
          @right-top {
            content: "";
            margin-right: 10px;
            margin-left: auto;
            width: 130px;
          }
        }
      </style>
    `)
	pageEqual(t, "margin width", box.MarginWidth(), 100)
	pageEqual(t, "margin left", box.MarginLeft, 0)     // rule 2
	pageEqual(t, "margin right", box.MarginRight, -30) // rule 3, after rule 2
	pageEqual(t, "width", box.Width, 130)              // As specified
}

// TestMarginBoxesFixedDimension6 is test_margin_boxes_fixed_dimension_6.
func TestMarginBoxesFixedDimension6(t *testing.T) {
	// Test rule 5
	box := pageOnlyMarginBox(t, `
      <style>
        @page {
          margin: 100px;
          @top-left { content: ""; margin-top: 10px; margin-bottom: auto }
        }
      </style>
    `)
	pageEqual(t, "margin height", box.MarginHeight(), 100)
	pageEqual(t, "margin top", box.MarginTop, 10)
	pageEqual(t, "margin bottom", box.MarginBottom, 0)
	pageEqual(t, "height", box.Height, 90)
}

// TestMarginBoxesFixedDimension7 is test_margin_boxes_fixed_dimension_7.
func TestMarginBoxesFixedDimension7(t *testing.T) {
	// Test rule 5
	box := pageOnlyMarginBox(t, `
      <style>
        @page {
          margin: 100px;
          @top-center { content: ""; margin: auto 0 }
        }
      </style>
    `)
	pageEqual(t, "margin height", box.MarginHeight(), 100)
	pageEqual(t, "margin top", box.MarginTop, 0)
	pageEqual(t, "margin bottom", box.MarginBottom, 0)
	pageEqual(t, "height", box.Height, 100)
}

// TestMarginBoxesFixedDimension8 is test_margin_boxes_fixed_dimension_8.
func TestMarginBoxesFixedDimension8(t *testing.T) {
	// Test rule 6
	box := pageOnlyMarginBox(t, `
      <style>
        @page {
          margin: 100px;
          @bottom-right { content: ""; margin: auto; height: 70px }
        }
      </style>
    `)
	pageEqual(t, "margin height", box.MarginHeight(), 100)
	pageEqual(t, "margin top", box.MarginTop, 15)
	pageEqual(t, "margin bottom", box.MarginBottom, 15)
	pageEqual(t, "height", box.Height, 70)
}

// TestMarginBoxesFixedDimension9 is test_margin_boxes_fixed_dimension_9.
func TestMarginBoxesFixedDimension9(t *testing.T) {
	// Rule 2 inhibits rule 6
	box := pageOnlyMarginBox(t, `
      <style>
        @page {
          margin: 100px;
          @bottom-center { content: ""; margin: auto 0; height: 150px }
        }
      </style>
    `)
	pageEqual(t, "margin height", box.MarginHeight(), 100)
	pageEqual(t, "margin top", box.MarginTop, 0)
	pageEqual(t, "margin bottom", box.MarginBottom, -50) // outside
	pageEqual(t, "height", box.Height, 150)
}

// pageImages is images of test_page.py: the CSS of SVG images of the
// widths, 10px high.
func pageImages(widths ...int) string {
	urls := make([]string, len(widths))
	for i, width := range widths {
		urls[i] = fmt.Sprintf(`url('data:image/svg+xml,<svg width="%d" height="10"></svg>')`, width)
	}
	return strings.Join(urls, " ")
}

// TestPageStyleLayout is test_page_style of tests/layout/test_page.py. The
// Go name differs from the name of upstream, because test_page_style of
// tests/test_boxes.py is TestPageStyle.
func TestPageStyleLayout(t *testing.T) {
	img := pageImages
	for i, c := range []struct {
		css    string
		widths []float64
	}{
		{`@top-left { content: ` + img(50, 50) + ` }
        @top-center { content: ` + img(50, 50) + ` }
        @top-right { content: ` + img(50, 50) + ` }
     `, []float64{100, 100, 100}}, // Use preferred widths if they fit
		{`@top-left { content: ` + img(50, 50) + `; margin: auto }
        @top-center { content: ` + img(50, 50) + ` }
        @top-right { content: ` + img(50, 50) + ` }
     `, []float64{100, 100, 100}}, // 'auto' margins are set to 0
		{`@top-left { content: ` + img(100, 50) + ` }
        @top-center { content: ` + img(300, 150) + ` }
        @top-right { content: 'foo'; width: 200px }
     `, []float64{150, 300, 200}}, // Use at least minimum widths, even if boxes overlap
		{`@top-left { content: ` + img(150, 150) + ` }
        @top-center { content: ` + img(150, 150) + ` }
        @top-right { content: ` + img(150, 150) + ` }
     `, []float64{200, 200, 200}}, // Distribute remaining space proportionally
		{`@top-left { content: ` + img(100, 100, 100) + ` }
        @top-center { content: ` + img(100, 100) + ` }
        @top-right { content: ` + img(10) + ` }
     `, []float64{220, 160, 10}},
		{`@top-left { content: ` + img(100, 100, 100) + `; width: 205px }
        @top-center { content: ` + img(100, 100) + ` }
        @top-right { content: ` + img(10) + ` }
     `, []float64{205, 190, 10}},
		{`@top-left { width: 1000px; margin: 1000px; padding: 1000px;
                    border: 1000px solid }
        @top-center { content: ` + img(100, 100) + ` }
        @top-right { content: ` + img(10) + ` }
     `, []float64{200, 10}}, // 'width' and other have no effect without 'content'
		{`@top-left { content: ''; width: 200px }
        @top-center { content: ''; width: 300px }
        @top-right { content: ` + img(50, 50) + ` }
     `, []float64{200, 300, 100}}, // This leaves 150px for @top-right’s shrink-to-fit
		{`@top-left { content: ''; width: 200px }
        @top-center { content: ''; width: 300px }
        @top-right { content: ` + img(100, 100, 100) + ` }
     `, []float64{200, 300, 150}},
		{`@top-left { content: ''; width: 200px }
        @top-center { content: ''; width: 300px }
        @top-right { content: ` + img(170, 175) + ` }
     `, []float64{200, 300, 175}},
		{`@top-left { content: ''; width: 200px }
        @top-right { content: ''; width: 500px }
     `, []float64{200, 500}},
		{`@top-left { content: ''; width: 200px }
        @top-right { content: ` + img(150, 50, 150) + ` }
     `, []float64{200, 350}},
		{`@top-left { content: ''; width: 200px }
        @top-right { content: ` + img(150, 50, 150, 200) + ` }
     `, []float64{200, 400}},
		{`@top-left { content: ` + img(150, 50, 450) + ` }
        @top-right { content: ''; width: 200px }
     `, []float64{450, 200}},
		{`@top-left { content: ` + img(150, 100) + ` }
        @top-right { content: ` + img(10, 120) + ` }
     `, []float64{250, 130}},
		{`@top-left { content: ` + img(550, 100) + ` }
        @top-right { content: ` + img(10, 120) + ` }
     `, []float64{550, 120}},
		{`@top-left { content: ` + img(250, 60) + ` }
        @top-right { content: ` + img(250, 180) + ` }
     `, []float64{275, 325}}, // 250 + (100 * 1 / 4), 250 + (100 * 3 / 4)
	} {
		t.Run(fmt.Sprint(i), func(t *testing.T) {
			var expected []string
			for _, atKeyword := range []string{"@top-left", "@top-center", "@top-right"} {
				if strings.Contains(c.css, atKeyword+" { content: ") {
					expected = append(expected, atKeyword)
				}
			}
			page := pageRender(t, `
      <style>
        @page {
          size: 800px;
          margin: 100px;
          padding: 42px;
          border: 7px solid;
          `+c.css+`
        }
      </style>
    `, 1)[0]
			pageTag(t, page.Children[0], "html")
			marginBoxes := page.Children[1:]
			var atKeywords []string
			var widths []float64
			offsets := map[string]float64{"@top-left": 0, "@top-center": 0.5, "@top-right": 1}
			for _, box := range marginBoxes {
				atKeywords = append(atKeywords, box.AtKeyword)
				widths = append(widths, box.MarginWidth())
				pageEqual(t, box.AtKeyword+" x", box.PositionX, 100+offsets[box.AtKeyword]*(600-box.MarginWidth()))
			}
			if fmt.Sprint(atKeywords) != fmt.Sprint(expected) {
				t.Errorf("at-keywords %v, want %v", atKeywords, expected)
			}
			if fmt.Sprint(widths) != fmt.Sprint(c.widths) {
				t.Errorf("widths %v, want %v", widths, c.widths)
			}
		})
	}
}

// TestMarginBoxesVerticalAlign is test_margin_boxes_vertical_align.
func TestMarginBoxesVerticalAlign(t *testing.T) {
	// 3 px ->    +-----+
	//            |  1  |
	//            +-----+
	//
	//        43 px ->   +-----+
	//        53 px ->   |  2  |
	//                   +-----+
	//
	//               83 px ->   +-----+
	//                          |  3  |
	//               103px ->   +-----+
	page := pageRender(t, `
      <style>
        @page {
          size: 800px;
          margin: 106px;  /* margin boxes’ content height is 100px */

          @top-left {
            content: "foo"; line-height: 20px; border: 3px solid;
            vertical-align: top;
          }
          @top-center {
            content: "foo"; line-height: 20px; border: 3px solid;
            vertical-align: middle;
          }
          @top-right {
            content: "foo"; line-height: 20px; border: 3px solid;
            vertical-align: bottom;
          }
        }
      </style>
    `, 1)[0]
	children := unpack(t, page, 4)
	pageEqual(t, "line 1 y", only(t, children[1]).PositionY, 3)
	pageEqual(t, "line 2 y", only(t, children[2]).PositionY, 43)
	pageEqual(t, "line 3 y", only(t, children[3]).PositionY, 83)
}

// TestMarginBoxesElement is test_margin_boxes_element.
func TestMarginBoxesElement(t *testing.T) {
	pages := pageRender(t, `
      <style>
        @page {
          counter-increment: count;
          counter-reset: page pages;
          margin: 50px;
          size: 200px;
          @bottom-center {
            content: counter(page) ' of ' counter(pages)
                     ' (' counter(count) ')';
          }
        }
        h1 {
          height: 40px;
        }
      </style>
      <h1>test1</h1>
      <h1>test2</h1>
      <h1>test3</h1>
      <h1>test4</h1>
      <h1>test5</h1>
      <h1>test6</h1>
    `, 3)
	for i, want := range []string{"0 of 3 (1)", "0 of 3 (2)", "0 of 3 (3)"} {
		if got := pageTexts(pages[i].Children[1]); got != want {
			t.Errorf("footer %d %q, want %q", i+1, got, want)
		}
	}
}

// TestMarginBoxesRunningElement is test_margin_boxes_running_element.
func TestMarginBoxesRunningElement(t *testing.T) {
	pages := pageRender(t, `
      <style>
        footer {
          position: running(footer);
        }
        @page {
          margin: 50px;
          size: 200px;
          @bottom-center {
            content: element(footer);
          }
        }
        body {
          font-size: 1px
        }
        h1 {
          height: 40px;
        }
        .pages:before {
          content: counter(page);
        }
        .pages:after {
          content: counter(pages);
        }
      </style>
      <footer class="pages"> of </footer>
      <h1>test1</h1>
      <h1>test2</h1>
      <h1>test3</h1>
      <h1>test4</h1>
      <h1>test5</h1>
      <h1>test6</h1>
      <footer>Static</footer>
    `, 3)
	for i, want := range []string{"1 of 3", "2 of 3", "Static"} {
		if got := pageTexts(pages[i].Children[1]); got != want {
			t.Errorf("footer %d %q, want %q", i+1, got, want)
		}
	}
}

// TestRunningElements is test_running_elements.
func TestRunningElements(t *testing.T) {
	for _, c := range []struct {
		argument string
		texts    []string
	}{
		// TODO: start doesn’t work because running elements are removed
		// from the original tree, and the current implentation in
		// layout.get_running_element_for uses the tree to know if it’s at
		// the beginning of the page

		// ('start', ('', '2-first', '2-last', '3-last', '5')),

		{"first", []string{"", "2-first", "3-first", "3-last", "5"}},
		{"last", []string{"", "2-last", "3-last", "3-last", "5"}},
		{"first-except", []string{"", "", "", "3-last", ""}},
	} {
		t.Run(c.argument, func(t *testing.T) {
			pages := pageRender(t, `
      <style>
        @page {
          margin: 50px;
          size: 200px;
          @bottom-center { content: element(title, `+c.argument+`) }
        }
        article { break-after: page }
        h1 { position: running(title) }
      </style>
      <article>
        <div>1</div>
      </article>
      <article>
        <h1>2-first</h1>
        <h1>2-last</h1>
      </article>
      <article>
        <p>3</p>
        <h1>3-first</h1>
        <h1>3-last</h1>
      </article>
      <article>
      </article>
      <article>
        <h1>5</h1>
      </article>
    `, 5)
			for i, page := range pages {
				margin := unpack(t, page, 2)[1]
				if len(margin.Children) > 0 {
					h1 := only(t, margin)
					line := only(t, h1)
					if got := only(t, line).Text; got != c.texts[i] {
						t.Errorf("page %d: %q, want %q", i+1, got, c.texts[i])
					}
				} else if c.texts[i] != "" {
					t.Errorf("page %d: no text, want %q", i+1, c.texts[i])
				}
			}
		})
	}
}

// TestRunningElementsDisplay is test_running_elements_display.
func TestRunningElementsDisplay(t *testing.T) {
	page := pageRender(t, `
      <style>
        @page {
          margin: 50px;
          size: 200px;
          @bottom-left { content: element(inline) }
          @bottom-center { content: element(block) }
          @bottom-right { content: element(table) }
        }
        table { position: running(table) }
        div { position: running(block) }
        span { position: running(inline) }
      </style>
      text
      <table><tr><td>table</td></tr></table>
      <div>block</div>
      <span>inline</span>
    `, 1)[0]
	children := unpack(t, page, 4)
	for i, want := range []string{"inline", "block", "table"} {
		if got := pageTexts(children[i+1]); got != want {
			t.Errorf("margin box %d %q, want %q", i+1, got, want)
		}
	}
}

// TestRunningImg is test_running_img.
func TestRunningImg(t *testing.T) {
	// Regression test.
	pageRender(t, `
      <style>
        img {
          position: running(img);
        }
        @page {
          @bottom-center {
            content: element(img);
          }
        }
      </style>
      <img src="pattern.png" />
    `, -1)
}

// TestRunningAbsolute is test_running_absolute.
func TestRunningAbsolute(t *testing.T) {
	// Regression test for #1540.
	pageRender(t, `
      <style>
        footer {
          position: running(footer);
        }
        p {
          position: absolute;
        }
        @page {
          @bottom-center {
            content: element(footer);
          }
        }
      </style>
      <footer>Hello!<p>Bonjour!</p></footer>
    `, -1)
}

// TestRunningAbsoluteOverflow is test_running_absolute_overflow.
func TestRunningAbsoluteOverflow(t *testing.T) {
	// Regression test for #2857.
	pageRender(t, `
      <style>
        header {
          position: running(header);
        }
        p {
          font: 2px/1 weasyprint;
          position: absolute;
        }
        div {
          break-before: page;
        }
        @page {
          size: 10px;
          margin: 2px;
          @top-center {
            content: element(header);
          }
        }
      </style>
      <header>
        <p>
          abc<br>
          abc<br>
          abc<br>
          abc<br>
          abc<br>
        </p>
      </header>
      <div>a</div>
      <div>a</div>
      <div>a</div>
      <div>a</div>
    `, -1)
}

// TestRunningFlex is test_running_flex.
func TestRunningFlex(t *testing.T) {
	// Regression test.
	pageRender(t, `
      <style>
        footer {
          display: flex;
          position: running(footer);
        }
        @page {
          @bottom-center {
            content: element(footer);
          }
        }
      </style>
      <footer>
        Hello!
      </footer>
    `, -1)
}

// TestRunningFloat is test_running_float.
func TestRunningFloat(t *testing.T) {
	// Regression test.
	pageRender(t, `
      <style>
        footer {
          float: left;
          position: running(footer);
        }
        @page {
          @bottom-center {
            content: element(footer);
          }
        }
      </style>
      <footer>
        Hello!
      </footer>
    `, -1)
}

// pageMarginText returns the text of the only text box of the only line of
// a margin box.
func pageMarginText(t *testing.T, box *Box) string {
	t.Helper()
	line := only(t, box)
	return only(t, line).Text
}

// TestPageStyle is test_page_style of tests/test_boxes.py.
func TestPageStyle(t *testing.T) {
	for _, c := range []struct {
		pageType                 PageType
		top, right, bottom, left float64
	}{
		{PageType{Side: "left", Index: 0}, 20, 3, 3, 10},
		{PageType{Side: "right", Index: 0}, 20, 10, 3, 3},
		{PageType{Side: "left", Index: 1}, 10, 3, 3, 10},
		{PageType{Side: "right", Index: 1}, 10, 10, 3, 3},
		{PageType{Side: "right", Name: "name", Index: 1, Groups: []PageGroup{{"name", 0}}}, 5, 10, 3, 15},
		{PageType{Side: "right", Name: "name", Index: 1, Groups: []PageGroup{{"name", 1}}}, 5, 10, 4, 15},
		{PageType{Side: "right", Name: "name", Index: 2, Groups: []PageGroup{{"name", 0}}}, 5, 10, 1, 15},
		{PageType{Side: "right", Name: "name", Index: 8, Groups: []PageGroup{{"name", 8}}}, 5, 10, 2, 15},
	} {
		t.Run(c.pageType.key(), func(t *testing.T) {
			log, capture := captureLogs(slog.LevelWarn)
			document := fakeHTML(t, `
      <style>
        @page { margin: 3px }
        @page name { margin-left: 15px; margin-top: 5px }
        @page :nth(3) { margin-bottom: 1px }
        @page :nth(5n+4) { margin-bottom: 2px }
        @page :nth(2 of name) { margin-bottom: 4px }
        @page :first { margin-top: 20px }
        @page :right { margin-right: 10px; margin-top: 10px }
        @page :left { margin-left: 10px; margin-top: 10px }
      </style>
    `, testBaseURL, log)
			styleFor := computedStyles(t, document, ComputedStylesOptions{})

			// Force the generation of the style for this page type as it's
			// generally only done during the rendering.
			setPageTypeStyles(&layoutContext{styleFor: styleFor}, c.pageType, document)

			style := styleFor.Get(c.pageType, "")
			for _, side := range []struct {
				name string
				want float64
			}{
				{"margin_top", c.top}, {"margin_right", c.right},
				{"margin_bottom", c.bottom}, {"margin_left", c.left},
			} {
				if got := style.Get(side.name); !Equal(got, Dimension{side.want, "px"}) {
					t.Errorf("%s = %s, want %vpx", side.name, ValueString(got), side.want)
				}
			}
			assertNoLogs(t, capture)
		})
	}
}

// TestMarginBoxes is test_margin_boxes of tests/test_boxes.py.
func TestMarginBoxes(t *testing.T) {
	pages := pageRender(t, `
      <style>
        @page {
          /* Make the page content area only 10px high and wide,
             so every word in <p> end up on a page of its own. */
          size: 30px;
          margin: 10px;
          @top-center { content: "Title" }
        }
        @page :first {
          @bottom-left { content: "foo" }
          @bottom-left-corner { content: "baz" }
        }
      </style>
      <p>lorem ipsum
    `, 2)
	pageTag(t, pages[0].Children[0], "html")
	pageTag(t, pages[1].Children[0], "html")

	atKeywords := func(page *Box) string {
		keywords := make([]string, 0, len(page.Children)-1)
		for _, box := range page.Children[1:] {
			keywords = append(keywords, box.AtKeyword)
		}
		return strings.Join(keywords, " ")
	}
	if got := atKeywords(pages[0]); got != "@top-center @bottom-left @bottom-left-corner" {
		t.Errorf("page 1 margin boxes %s", got)
	}
	if got := atKeywords(pages[1]); got != "@top-center" {
		t.Errorf("page 2 margin boxes %s", got)
	}

	topCenter := unpack(t, pages[1], 2)[1]
	if got := pageMarginText(t, topCenter); got != "Title" {
		t.Errorf("text %q", got)
	}
}

// pageMarginTexts renders a document, and checks the text of the second
// child of each page, a margin box.
func pageMarginTexts(t *testing.T, source string, texts ...string) {
	t.Helper()
	pages := pageRender(t, source, len(texts))
	for i, page := range pages {
		box := unpack(t, page, 2)[1]
		if got := pageMarginText(t, box); got != texts[i] {
			t.Errorf("page %d: %q, want %q", i+1, got, texts[i])
		}
	}
}

// TestMarginBoxStringSet1 is test_margin_box_string_set_1.
func TestMarginBoxStringSet1(t *testing.T) {
	// Test that both pages get string in the `bottom-center` margin box
	pageMarginTexts(t, `
      <style>
        @page {
          @bottom-center { content: string(text_header) }
        }
        p {
          string-set: text_header content();
        }
        .page {
          page-break-before: always;
        }
      </style>
      <p>first assignment</p>
      <div class="page"></div>
    `, "first assignment", "first assignment")
}

// TestMarginBoxStringSet2 is test_margin_box_string_set_2.
func TestMarginBoxStringSet2(t *testing.T) {
	// Test each accepted value of `content()` as an arguemnt to
	// `string-set`
	for _, value := range []string{"", "text", "before", "after"} {
		t.Run(value, func(t *testing.T) {
			extraStyle := ""
			want := "first assignment"
			if value == "before" || value == "after" {
				extraStyle = "p:" + value + `{content: "pseudo"}`
				want = "pseudo"
			}
			pageMarginTexts(t, `
          <style>
            @page {
              @top-center { content: string(text_header) }
            }
            p {
              string-set: text_header content(`+value+`);
            }
            `+extraStyle+`
          </style>
          <p>first assignment</p>
        `, want)
		})
	}
}

// TestMarginBoxStringSet3 is test_margin_box_string_set_3.
func TestMarginBoxStringSet3(t *testing.T) {
	// Test `first` (default value) ie. use the first assignment on the page
	pageMarginTexts(t, `
      <style>
        @page {
          @top-center { content: string(text_header, first) }
        }
        p {
          string-set: text_header content();
        }
      </style>
      <p>first assignment</p>
      <p>Second assignment</p>
    `, "first assignment")
}

// TestMarginBoxStringSet4 is test_margin_box_string_set_4.
func TestMarginBoxStringSet4(t *testing.T) {
	// test `first-except` ie. exclude from page on which value is assigned
	pages := pageRender(t, `
      <style>
        @page {
          @top-center { content: string(header_nofirst, first-except) }
        }
        p{
          string-set: header_nofirst content();
        }
        .page{
          page-break-before: always;
        }
      </style>
      <p>first_excepted</p>
      <div class="page"></div>
    `, 2)
	unpack(t, unpack(t, pages[0], 2)[1], 0)
	if got := pageMarginText(t, unpack(t, pages[1], 2)[1]); got != "first_excepted" {
		t.Errorf("text %q", got)
	}
}

// TestMarginBoxStringSet5 is test_margin_box_string_set_5.
func TestMarginBoxStringSet5(t *testing.T) {
	// Test `last` ie. use the most-recent assignment
	page := pageRender(t, `
      <style>
        @page {
          @top-center { content: string(header_last, last) }
        }
        p {
          string-set: header_last content();
        }
      </style>
      <p>String set</p>
      <p>Second assignment</p>
    `, 1)[0]
	if len(page.Children) < 2 {
		t.Fatalf("children %v", page.Children)
	}
	if got := pageMarginText(t, page.Children[1]); got != "Second assignment" {
		t.Errorf("text %q", got)
	}
}

// TestMarginBoxStringSet6 is test_margin_box_string_set_6.
func TestMarginBoxStringSet6(t *testing.T) {
	// Test multiple complex string-set values
	page := pageRender(t, `
      <style>
        @page {
          @top-center { content: string(text_header, first) }
          @bottom-center { content: string(text_footer, last) }
        }
        html { counter-reset: a }
        body { counter-increment: a }
        ul { counter-reset: b }
        li {
          counter-increment: b;
          string-set:
            text_header content(before) "-" content() "-" content(after)
                        counter(a, upper-roman) '.' counters(b, '|'),
            text_footer content(before) '-' attr(class)
                        counters(b, '|') "/" counter(a, upper-roman);
        }
        li:before { content: 'before!' }
        li:after { content: 'after!' }
        li:last-child:before { content: 'before!last' }
        li:last-child:after { content: 'after!last' }
      </style>
      <ul>
        <li class="firstclass">first
        <li>
          <ul>
            <li class="secondclass">second
    `, 1)[0]
	children := unpack(t, page, 3)
	if got := pageMarginText(t, children[1]); got != "before!-first-after!I.1" {
		t.Errorf("top text %q", got)
	}
	if got := pageMarginText(t, children[2]); got != "before!last-secondclass2|1/I" {
		t.Errorf("bottom text %q", got)
	}
}

// TestMarginBoxStringSet7 is test_margin_box_string_set_7.
func TestMarginBoxStringSet7(t *testing.T) {
	// Regression test for #722.
	page := renderPages(t, `
      <style>
        img { string-set: left attr(alt) }
        img + img { string-set: right attr(alt) }
        @page { @top-left  { content: '[' string(left)  ']' }
                @top-right { content: '{' string(right) '}' } }
      </style>
      <img src=pattern.png alt="Chocolate">
      <img src=no_such_file.png alt="Cake">
    `)
	if len(page) != 1 {
		t.Fatalf("%d pages", len(page))
	}
	children := unpack(t, page[0], 3)
	if got := pageMarginText(t, children[1]); got != "[Chocolate]" {
		t.Errorf("left text %q", got)
	}
	if got := pageMarginText(t, children[2]); got != "{Cake}" {
		t.Errorf("right text %q", got)
	}
}

// TestMarginBoxStringSet8 is test_margin_box_string_set_8.
func TestMarginBoxStringSet8(t *testing.T) {
	// Regression test for #726.
	pageMarginTexts(t, `
      <style>
        @page { @top-left  { content: '[' string(left) ']' } }
        p { page-break-before: always }
        .initial { string-set: left 'initial' }
        .empty   { string-set: left ''        }
        .space   { string-set: left ' '       }
      </style>

      <p class="initial">Initial</p>
      <p class="empty">Empty</p>
      <p class="space">Space</p>
    `, "[initial]", "[]", "[ ]")
}

// TestMarginBoxStringSet9 is test_margin_box_string_set_9.
func TestMarginBoxStringSet9(t *testing.T) {
	// Regression test for #827.
	// Test that named strings are case-sensitive.
	pageMarginTexts(t, `
      <style>
        @page {
          @top-center {
            content: string(text_header, first)
                     ' ' string(TEXT_header, first)
          }
        }
        p { string-set: text_header content() }
        div { string-set: TEXT_header content() }
      </style>
      <p>first assignment</p>
      <div>second assignment</div>
    `, "first assignment second assignment")
}

// TestMarginBoxStringSet10 is test_margin_box_string_set_10.
func TestMarginBoxStringSet10(t *testing.T) {
	pageMarginTexts(t, `
      <style>
        @page { @top-left  { content: '[' string(p, start) ']' } }
        p { string-set: p content(); page-break-after: always }
      </style>
      <article></article>
      <p>1</p>
      <article></article>
      <p>2</p>
      <p>3</p>
      <article></article>
    `, "[]", "[1]", "[3]", "[3]")
}

// TestPageCounters is test_page_counters.
func TestPageCounters(t *testing.T) {
	// Test page-based counters. The spaces of the content are no-break
	// spaces, as in upstream, so the text stays on one line.
	pages := pageRender(t, `
      <style>
        @page {
          /* Make the page content area only 10px high and wide,
             so every word in <p> end up on a page of its own. */
          size: 30px;
          margin: 10px;
          @bottom-center {
            content: "Page " counter(page) " of " counter(pages) ".";
          }
        }
      </style>
      <p>lorem ipsum dolor
    `, 3)
	for i, page := range pages {
		bottomCenter := unpack(t, page, 2)[1]
		lineBox := only(t, bottomCenter)
		textBox := only(t, lineBox)
		want := fmt.Sprintf("Page\u00a0%d\u00a0of\u00a03.", i+1)
		if textBox.Text != want {
			t.Errorf("text %q, want %q", textBox.Text, want)
		}
	}
}

// TestPageCountersInPageContent is test_page_counters_in_page_content.
func TestPageCountersInPageContent(t *testing.T) {
	// Test page-based counters in page_content.
	pages := pageRender(t, `
      <style>
        div { break-after: page }
        div::before { content: counter(pages) }
      </style>
      <div></div>
      <div></div>
      <div></div>
    `, -1)
	for _, page := range pages {
		_, body := pageBody(t, page)
		div := only(t, body)
		line := only(t, div)
		before := only(t, line)
		if got := only(t, before).Text; got != "3" {
			t.Errorf("text %q", got)
		}
	}
}

// pageDocument returns the Document of some HTML, with nothing laid out.
func pageDocument(t *testing.T, source string) *Document {
	t.Helper()
	p := parseBase(t, source, testBaseURL, nil)
	root, err := p.builder.buildFormattingStructure(p.html.EtreeElement)
	if err != nil {
		t.Fatal(err)
	}
	return NewDocument(p.html, root, p.styleFor, DocumentOptions{
		FontConfig:      p.fc,
		TargetCollector: p.builder.targetCollector,
		CounterStyle:    p.builder.counterStyle,
		Footnotes:       p.builder.footnotes,
	})
}

// pageLongDocument is a document of 50 pages, with the margin boxes of
// margins.
func pageLongDocument(margins string) string {
	return `<style>
        @page { size: 100px; margin: 20px; ` + margins + ` }
        body { margin: 0 }
        div { height: 60px }
      </style>` + strings.Repeat("<div></div>", 50)
}

// pageCountingContext is a context whose method Err reports the
// cancellation after n calls.
type pageCountingContext struct {
	context.Context
	n int
}

// Err satisfies context.Context.
func (c *pageCountingContext) Err() error {
	c.n--
	if c.n < 0 {
		return context.Canceled
	}
	return nil
}

// TestDocumentLazy checks that Document.Page lays out the pages up to the
// page that it returns, and not the rest of the document.
func TestDocumentLazy(t *testing.T) {
	ctx := context.Background()
	d := pageDocument(t, pageLongDocument(""))
	page, err := d.Page(ctx, 0)
	if err != nil {
		t.Fatal(err)
	}
	if n := len(d.pages); n > 2 {
		t.Errorf("Page(0) laid out %d pages", n)
	}
	_, body := pageBody(t, page)
	only(t, body)

	page, err = d.Page(ctx, 9)
	if err != nil {
		t.Fatal(err)
	}
	if n := len(d.pages); n > 11 {
		t.Errorf("Page(9) laid out %d pages", n)
	}
	_, body = pageBody(t, page)
	pageEqual(t, "div y", only(t, body).PositionY, 20)

	n, err := d.Count(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if n != 50 {
		t.Errorf("Count = %d, want 50", n)
	}
	if _, err := d.Page(ctx, 49); err != nil {
		t.Error(err)
	}
	if _, err := d.Page(ctx, 50); !errors.Is(err, ErrNoPage) {
		t.Errorf("Page(50) error %v, want ErrNoPage", err)
	}
	if _, err := d.Page(ctx, -1); !errors.Is(err, ErrNoPage) {
		t.Errorf("Page(-1) error %v, want ErrNoPage", err)
	}
	pages, err := d.All(ctx)
	if err != nil || len(pages) != 50 {
		t.Errorf("All gives %d pages and %v", len(pages), err)
	}
}

// TestDocumentLazyPagesCounter checks that Document.Page gives the number
// of pages to the margin boxes of the first page, without a call to Count.
func TestDocumentLazyPagesCounter(t *testing.T) {
	d := pageDocument(t, pageLongDocument(
		`@bottom-center { content: counter(page) "/" counter(pages) }`))
	page, err := d.Page(context.Background(), 0)
	if err != nil {
		t.Fatal(err)
	}
	if got := pageTexts(unpack(t, page, 2)[1]); got != "1/50" {
		t.Errorf("margin box text %q, want 1/50", got)
	}
	page, err = d.Page(context.Background(), 49)
	if err != nil {
		t.Fatal(err)
	}
	if got := pageTexts(unpack(t, page, 2)[1]); got != "50/50" {
		t.Errorf("margin box text %q, want 50/50", got)
	}
}

// TestDocumentCancel checks that a cancelled context stops the layout with
// its error.
func TestDocumentCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	d := pageDocument(t, pageLongDocument(""))
	if _, err := d.Page(ctx, 0); !errors.Is(err, context.Canceled) {
		t.Errorf("Page error %v, want context.Canceled", err)
	}

	// The context is cancelled in the middle of the layout.
	d = pageDocument(t, pageLongDocument(""))
	_, err := d.Count(&pageCountingContext{Context: context.Background(), n: 3})
	if !errors.Is(err, context.Canceled) {
		t.Errorf("Count error %v, want context.Canceled", err)
	}
}
