// Copyright (c) 2011-2021, Simon Sapin and contributors.
//
// Use of this source code is governed by the BSD-3-Clause license that can be
// found in the LICENSE file.

// This file ports tests/layout/test_block.py: the tests of the layout of
// the blocks.

package layout

import (
	"fmt"
	"log/slog"
	"strings"
	"testing"
)

// blockPages is render_pages with assert_no_logs: it lays out a document,
// and fails the test for each warning that the layout logs.
func blockPages(t *testing.T, source string) []*Box {
	t.Helper()
	log, logs := captureLogs(slog.LevelWarn)
	pages := renderPagesLog(t, source, testBaseURL, log)
	assertNoLogs(t, logs)
	return pages
}

// blockPagesN is blockPages for a document of n pages.
func blockPagesN(t *testing.T, source string, n int) []*Box {
	t.Helper()
	pages := blockPages(t, source)
	if len(pages) != n {
		t.Fatalf("got %d pages, want %d", len(pages), n)
	}
	return pages
}

// blockPage is blockPages for a document of one page: `page, =
// render_pages(...)`.
func blockPage(t *testing.T, source string) *Box {
	t.Helper()
	return blockPagesN(t, source, 1)[0]
}

// blockBody returns the box of the body of the only page of a document.
func blockBody(t *testing.T, source string) *Box {
	t.Helper()
	_, body := pageBody(t, blockPage(t, source))
	return body
}

// blockDescendants returns the descendants of box whose element tag is
// tag, in the order of the tree.
func blockDescendants(box *Box, tag string) []*Box {
	var result []*Box
	box.Descendants(func(d *Box) bool {
		if d.ElementTag == tag {
			result = append(result, d)
		}
		return true
	})
	return result
}

func TestBlockWidths(t *testing.T) {
	page := blockPage(t, `
      <style>
        @page { margin: 0; size: 120px 2000px }
        body { margin: 0 }
        div { margin: 10px }
        p { padding: 2px; border-width: 1px; border-style: solid }
      </style>
      <div>
        <p></p>
        <p style="width: 50px"></p>
      </div>
      <div style="direction: rtl">
        <p style="width: 50px; direction: rtl"></p>
      </div>
      <div>
        <p style="margin: 0 10px 0 20px"></p>
        <p style="width: 50px; margin-left: 20px; margin-right: auto"></p>
        <p style="width: 50px; margin-left: auto; margin-right: 20px"></p>
        <p style="width: 50px; margin: auto"></p>

        <p style="margin-left: 20px; margin-right: auto"></p>
        <p style="margin-left: auto; margin-right: 20px"></p>
        <p style="margin: auto"></p>

        <p style="width: 200px; margin: auto"></p>

        <p style="min-width: 200px; margin: auto"></p>
        <p style="max-width: 50px; margin: auto"></p>
        <p style="min-width: 50px; margin: auto"></p>

        <p style="width: 70%"></p>
      </div>
    `)
	html, body := pageBody(t, page)
	if html.ElementTag != "html" || body.ElementTag != "body" {
		t.Fatalf("tags %q %q", html.ElementTag, body.ElementTag)
	}
	assertApprox(t, "body.width", body.Width, 120)

	var paragraphs []*Box
	for _, div := range body.Children {
		if !div.Is(KindBlock) || div.ElementTag != "div" {
			t.Fatalf("div is %v", div)
		}
		assertApprox(t, "div.width", div.Width, 100)
		for _, paragraph := range div.Children {
			if !paragraph.Is(KindBlock) || paragraph.ElementTag != "p" {
				t.Fatalf("paragraph is %v", paragraph)
			}
			assertApprox(t, "padding_left", paragraph.PaddingLeft, 2)
			assertApprox(t, "padding_right", paragraph.PaddingRight, 2)
			assertApprox(t, "border_left_width", paragraph.BorderLeftWidth, 1)
			assertApprox(t, "border_right_width", paragraph.BorderRightWidth, 1)
			paragraphs = append(paragraphs, paragraph)
		}
	}
	if len(paragraphs) != 15 {
		t.Fatalf("%d paragraphs", len(paragraphs))
	}
	for i, want := range [][2]float64{
		{94, 0}, {50, 0}, {50, -1}, {64, 20}, {50, 20}, {50, 24}, {50, 22},
		{74, 20}, {74, 0}, {94, 0}, {200, 0}, {200, 0}, {50, 22}, {94, 0},
		{70, 0},
	} {
		p := paragraphs[i]
		assertApprox(t, fmt.Sprintf("paragraphs[%d].width", i), p.Width, want[0])
		if i == 2 {
			// No 'auto', over-constrained equation with rtl, the initial
			// 'margin-left: 0' was ignored.
			assertApprox(t, "paragraphs[2].margin_right", p.MarginRight, 0)
			continue
		}
		assertApprox(t, fmt.Sprintf("paragraphs[%d].margin_left", i), p.MarginLeft, want[1])
		if i == 0 {
			assertApprox(t, "paragraphs[0].margin_right", p.MarginRight, 0)
		}
	}
}

func TestBlockHeightsP(t *testing.T) {
	body := blockBody(t, `
      <style>
        @page { margin: 0; size: 100px 20000px }
        html, body { margin: 0 }
        div { margin: 4px; border: 2px solid; padding: 4px }
        /* Use top margins so that margin collapsing doesn't change result */
        p { margin: 16px 0 0; border: 4px solid; padding: 8px; height: 50px }
      </style>
      <div>
        <p></p>
        <!-- Not in normal flow: don't contribute to the parent’s height -->
        <p style="position: absolute"></p>
        <p style="float: left"></p>
      </div>
      <div> <p></p> <p></p> <p></p> </div>
      <div style="height: 20px"> <p></p> </div>
      <div style="height: 120px"> <p></p> </div>
      <div style="max-height: 20px"> <p></p> </div>
      <div style="min-height: 120px"> <p></p> </div>
      <div style="min-height: 20px"> <p></p> </div>
      <div style="max-height: 120px"> <p></p> </div>
    `)
	blockAssertHeights(t, body, []float64{90, 90 * 3, 20, 120, 20, 120, 90, 90})
}

// blockAssertHeights checks the heights of the children of body.
func blockAssertHeights(t *testing.T, body *Box, want []float64) {
	t.Helper()
	heights := make([]float64, 0, len(body.Children))
	for _, div := range body.Children {
		heights = append(heights, div.Height)
	}
	if fmt.Sprint(heights) != fmt.Sprint(want) {
		t.Errorf("heights = %v, want %v", heights, want)
	}
}

func TestBlockHeightsImg(t *testing.T) {
	body := blockBody(t, `
      <style>
        body { height: 200px; font-size: 0 }
      </style>
      <div>
        <img src=pattern.png style="height: 40px">
      </div>
      <div style="height: 10%">
        <img src=pattern.png style="height: 40px">
      </div>
      <div style="max-height: 20px">
        <img src=pattern.png style="height: 40px">
      </div>
      <div style="max-height: 10%">
        <img src=pattern.png style="height: 40px">
      </div>
      <div style="min-height: 20px"></div>
      <div style="min-height: 10%"></div>
    `)
	blockAssertHeights(t, body, []float64{40, 20, 20, 20, 20, 20})
}

func TestBlockHeightsImgNoBodyHeight(t *testing.T) {
	// Same but with no height on body: percentage *-height is ignored
	body := blockBody(t, `
      <style>
        body { font-size: 0 }
      </style>
        <div>
          <img src=pattern.png style="height: 40px">
        </div>
        <div style="height: 10%">
          <img src=pattern.png style="height: 40px">
        </div>
        <div style="max-height: 20px">
          <img src=pattern.png style="height: 40px">
        </div>
        <div style="max-height: 10%">
          <img src=pattern.png style="height: 40px">
        </div>
        <div style="min-height: 20px"></div>
        <div style="min-height: 10%"></div>
    `)
	blockAssertHeights(t, body, []float64{40, 40, 20, 40, 20, 0})
}

func TestBlockPercentageHeightsNoHTMLHeight(t *testing.T) {
	body := blockBody(t, `
      <style>
        html, body { margin: 0 }
        body { height: 50% }
      </style>
    `)
	// Since html’s height depend on body’s, body’s 50% means 'auto'
	assertApprox(t, "body.height", body.Height, 0)
}

func TestBlockPercentageHeights(t *testing.T) {
	body := blockBody(t, `
      <style>
        html, body { margin: 0 }
        html { height: 300px }
        body { height: 50% }
      </style>
    `)
	// This time the percentage makes sense
	assertApprox(t, "body.height", body.Height, 150)
}

func TestBlockBoxSizing(t *testing.T) {
	for _, size := range []string{
		"width: 10%; height: 1000px",
		"max-width: 10%; max-height: 1000px; height: 2000px",
		"width: 5%; min-width: 10%; min-height: 1000px",
		"width: 10%; height: 1000px; min-width: auto; max-height: none",
	} {
		t.Run(size, func(t *testing.T) {
			// https://www.w3.org/TR/css-ui-3/#box-sizing
			body := blockBody(t, fmt.Sprintf(`
      <style>
        @page { size: 100000px }
        body { width: 10000px; margin: 0 }
        div { %s; margin: 100px; padding: 10px; border: 1px solid }
      </style>
      <div></div>

      <div style="box-sizing: content-box"></div>
      <div style="box-sizing: padding-box"></div>
      <div style="box-sizing: border-box"></div>
    `, size))
			divs := unpack(t, body, 4)
			for i, c := range []struct {
				sizing                              string
				size, padding, border, marginHeight float64
			}{
				{"content-box", 1000, 1020, 1022, 1222},
				{"content-box", 1000, 1020, 1022, 1222},
				{"padding-box", 980, 1000, 1002, 1202},
				{"border-box", 978, 998, 1000, 1200},
			} {
				div := divs[i]
				if !isStr(div.Style.Get("box_sizing"), c.sizing) {
					t.Errorf("div %d: box_sizing %v", i, div.Style.Get("box_sizing"))
				}
				assertApprox(t, "width", div.Width, c.size)
				assertApprox(t, "height", div.Height, c.size)
				assertApprox(t, "padding_width", div.PaddingWidth(), c.padding)
				assertApprox(t, "padding_height", div.PaddingHeight(), c.padding)
				assertApprox(t, "border_width", div.BorderWidth(), c.border)
				assertApprox(t, "border_height", div.BorderHeight(), c.border)
				assertApprox(t, "margin_height", div.MarginHeight(), c.marginHeight)
				// margin_width() is the width of the containing block
			}
		})
	}
}

func TestBlockBoxSizingZero(t *testing.T) {
	for _, size := range []string{
		"width: 0; height: 0",
		"max-width: 0; max-height: 0",
		"min-width: 0; min-height: 0; width: 0; height: 0",
	} {
		t.Run(size, func(t *testing.T) {
			// https://www.w3.org/TR/css-ui-3/#box-sizing
			body := blockBody(t, fmt.Sprintf(`
      <style>
        @page { size: 100000px }
        body { width: 10000px; margin: 0 }
        div { %s; margin: 100px; padding: 10px; border: 1px solid }
      </style>
      <div></div>

      <div style="box-sizing: content-box"></div>
      <div style="box-sizing: padding-box"></div>
      <div style="box-sizing: border-box"></div>
    `, size))
			for _, div := range body.Children {
				assertApprox(t, "width", div.Width, 0)
				assertApprox(t, "height", div.Height, 0)
				assertApprox(t, "padding_width", div.PaddingWidth(), 20)
				assertApprox(t, "padding_height", div.PaddingHeight(), 20)
				assertApprox(t, "border_width", div.BorderWidth(), 22)
				assertApprox(t, "border_height", div.BorderHeight(), 22)
				assertApprox(t, "margin_height", div.MarginHeight(), 222)
				// margin_width() is the width of the containing block
			}
		})
	}
}

// blockMargins are the cases COLLAPSING and NOT_COLLAPSING of the tests of
// the vertical space: two margins, and the space with and without
// collapsing.
var blockMargins = []struct {
	margin1, margin2          string
	collapsing, notCollapsing float64
}{
	{"10px", "15px", 15, 25}, // not 25
	// "The maximum of the absolute values of the negative adjoining
	// margins is deducted from the maximum of the positive adjoining
	// margins"
	{"-10px", "15px", 5, 5},
	{"10px", "-15px", -5, -5},
	{"-10px", "-15px", -15, -25},
	{"10px", "auto", 10, 10}, // 'auto' is 0
}

// blockVerticalSpace runs a test of the vertical space for each case of
// blockMargins. format is the document, with the two margins, and space
// returns the space to check in the body.
func blockVerticalSpace(t *testing.T, format string, collapsing bool, space func(t *testing.T, body *Box) float64) {
	for _, c := range blockMargins {
		t.Run(c.margin1+" "+c.margin2, func(t *testing.T) {
			var source string
			if strings.Count(format, "%s") == 4 {
				source = fmt.Sprintf(format, c.margin1, c.margin2, c.margin1, c.margin2)
			} else {
				source = fmt.Sprintf(format, c.margin1, c.margin2)
			}
			_, body := pageBody(t, renderPages(t, source)[0])
			want := c.notCollapsing
			if collapsing {
				want = c.collapsing
			}
			assertApprox(t, "space", space(t, body), want)
		})
	}
}

// blockSpaceBetween returns the space between the content of p1 and of p2.
func blockSpaceBetween(p1, p2 *Box) float64 {
	return p2.ContentBoxY() - (p1.ContentBoxY() + p1.Height)
}

func TestBlockVerticalSpace1(t *testing.T) {
	// Siblings
	blockVerticalSpace(t, `
      <style>
        p { font: 20px/1 serif } /* block height == 20px */
        #p1 { margin-bottom: %s }
        #p2 { margin-top: %s }
      </style>
      <p id=p1>Lorem ipsum
      <p id=p2>dolor sit amet
    `, true, func(t *testing.T, body *Box) float64 {
		children := unpack(t, body, 2)
		return blockSpaceBetween(children[0], children[1])
	})
}

func TestBlockVerticalSpace2(t *testing.T) {
	// Not siblings, first is nested
	blockVerticalSpace(t, `
      <style>
        p { font: 20px/1 serif } /* block height == 20px */
        #p1 { margin-bottom: %s }
        #p2 { margin-top: %s }
      </style>
      <div>
        <p id=p1>Lorem ipsum
      </div>
      <p id=p2>dolor sit amet
    `, true, func(t *testing.T, body *Box) float64 {
		children := unpack(t, body, 2)
		return blockSpaceBetween(only(t, children[0]), children[1])
	})
}

func TestBlockVerticalSpace3(t *testing.T) {
	// Not siblings, second is nested
	blockVerticalSpace(t, `
      <style>
        p { font: 20px/1 serif } /* block height == 20px */
        #p1 { margin-bottom: %s }
        #p2 { margin-top: %s }
      </style>
      <p id=p1>Lorem ipsum
      <div>
        <p id=p2>dolor sit amet
      </div>
    `, true, func(t *testing.T, body *Box) float64 {
		children := unpack(t, body, 2)
		return blockSpaceBetween(children[0], only(t, children[1]))
	})
}

func TestBlockVerticalSpace4(t *testing.T) {
	// Not siblings, second is doubly nested
	blockVerticalSpace(t, `
      <style>
        p { font: 20px/1 serif } /* block height == 20px */
        #p1 { margin-bottom: %s }
        #p2 { margin-top: %s }
      </style>
      <p id=p1>Lorem ipsum
      <div>
        <div>
            <p id=p2>dolor sit amet
        </div>
      </div>
    `, true, func(t *testing.T, body *Box) float64 {
		children := unpack(t, body, 2)
		return blockSpaceBetween(children[0], only(t, only(t, children[1])))
	})
}

func TestBlockVerticalSpace5(t *testing.T) {
	// Collapsing with children
	blockVerticalSpace(t, `
      <style>
        p { font: 20px/1 serif } /* block height == 20px */
        #div1 { margin-top: %s }
        #div2 { margin-top: %s }
      </style>
      <p>Lorem ipsum
      <div id=div1>
        <div id=div2>
          <p id=p2>dolor sit amet
        </div>
      </div>
    `, true, func(t *testing.T, body *Box) float64 {
		children := unpack(t, body, 2)
		p1, div1 := children[0], children[1]
		div2 := only(t, div1)
		p2 := only(t, div2)
		// Parent and element edge are the same:
		assertApprox(t, "div1.border_box_y", div1.BorderBoxY(), p2.BorderBoxY())
		assertApprox(t, "div2.border_box_y", div2.BorderBoxY(), p2.BorderBoxY())
		return blockSpaceBetween(p1, p2)
	})
}

func TestBlockVerticalSpace6(t *testing.T) {
	// Block formatting context: Not collapsing with children
	blockVerticalSpace(t, `
      <style>
        p { font: 20px/1 serif } /* block height == 20px */
        #div1 { margin-top: %s; overflow: hidden }
        #div2 { margin-top: %s }
      </style>
      <p>Lorem ipsum
      <div id=div1>
        <div id=div2>
          <p id=p2>dolor sit amet
        </div>
      </div>
    `, false, func(t *testing.T, body *Box) float64 {
		children := unpack(t, body, 2)
		return blockSpaceBetween(children[0], only(t, only(t, children[1])))
	})
}

func TestBlockVerticalSpace7(t *testing.T) {
	// Collapsing through an empty div
	blockVerticalSpace(t, `
      <style>
        p { font: 20px/1 serif } /* block height == 20px */
        #p1 { margin-bottom: %s }
        #p2 { margin-top: %s }
        div { margin-bottom: %s; margin-top: %s }
      </style>
      <p id=p1>Lorem ipsum
      <div></div>
      <p id=p2>dolor sit amet
    `, true, func(t *testing.T, body *Box) float64 {
		children := unpack(t, body, 3)
		return blockSpaceBetween(children[0], children[2])
	})
}

func TestBlockVerticalSpace8(t *testing.T) {
	// The root element does not collapse
	blockVerticalSpace(t, `
      <style>
        html { margin-top: %s }
        body { margin-top: %s }
      </style>
      <p>Lorem ipsum
    `, false, func(t *testing.T, body *Box) float64 {
		// Vertical space from y=0
		return only(t, body).ContentBoxY()
	})
}

func TestBlockVerticalSpace9(t *testing.T) {
	// <body> DOES collapse
	blockVerticalSpace(t, `
      <style>
        body { margin-top: %s }
        div { margin-top: %s }
      </style>
      <div>
        <p>Lorem ipsum
    `, true, func(t *testing.T, body *Box) float64 {
		// Vertical space from y=0
		return only(t, only(t, body)).ContentBoxY()
	})
}

// blockFields checks the fields of a box: each name is a field or a method
// of the box model, and each value is its expected value.
func blockFields(t *testing.T, name string, box *Box, fields map[string]float64) {
	t.Helper()
	values := map[string]float64{
		"position_x":          box.PositionX,
		"position_y":          box.PositionY,
		"width":               box.Width,
		"height":              box.Height,
		"margin_top":          box.MarginTop,
		"margin_right":        box.MarginRight,
		"margin_bottom":       box.MarginBottom,
		"margin_left":         box.MarginLeft,
		"padding_top":         box.PaddingTop,
		"padding_right":       box.PaddingRight,
		"padding_bottom":      box.PaddingBottom,
		"padding_left":        box.PaddingLeft,
		"border_top_width":    box.BorderTopWidth,
		"border_right_width":  box.BorderRightWidth,
		"border_bottom_width": box.BorderBottomWidth,
		"border_left_width":   box.BorderLeftWidth,
		"content_box_x":       box.ContentBoxX(),
		"content_box_y":       box.ContentBoxY(),
		"padding_box_y":       box.PaddingBoxY(),
		"border_box_y":        box.BorderBoxY(),
		"margin_height":       box.MarginHeight(),
		"margin_width":        box.MarginWidth(),
		"border_height":       box.BorderHeight(),
		"border_width":        box.BorderWidth(),
		"top":                 box.extra().Top,
		"left":                box.extra().Left,
		"right":               box.extra().Right,
		"bottom":              box.extra().Bottom,
		"baseline":            box.Baseline,
	}
	for field, want := range fields {
		got, ok := values[field]
		if !ok {
			t.Fatalf("unknown field %s", field)
		}
		assertApprox(t, name+"."+field, got, want)
	}
}

func TestBlockBoxDecorationBreakBlockSlice(t *testing.T) {
	// https://www.w3.org/TR/css-backgrounds-3/#the-box-decoration-break
	pages := blockPagesN(t, `
      <style>
        @page { size: 100px }
        p { padding: 2px; border: 3px solid; margin: 5px }
        img { display: block; height: 40px }
      </style>
      <p>
        <img src=pattern.png>
        <img src=pattern.png>
        <img src=pattern.png>
        <img src=pattern.png>`, 2)
	_, body := pageBody(t, pages[0])
	paragraph := only(t, body)
	imgs := unpack(t, paragraph, 2)
	blockFields(t, "paragraph", paragraph, map[string]float64{
		"position_y": 0, "margin_top": 5, "border_top_width": 3,
		"padding_top": 2, "content_box_y": 10, "height": 90,
		"margin_bottom": 0, "border_bottom_width": 0, "padding_bottom": 0,
		"margin_height": 100,
	})
	assertApprox(t, "img_1.position_y", imgs[0].PositionY, 10)
	assertApprox(t, "img_2.position_y", imgs[1].PositionY, 50)

	_, body = pageBody(t, pages[1])
	paragraph = only(t, body)
	imgs = unpack(t, paragraph, 2)
	blockFields(t, "paragraph", paragraph, map[string]float64{
		"position_y": 0, "margin_top": 0, "border_top_width": 0,
		"padding_top": 0, "content_box_y": 0, "height": 80,
		"padding_bottom": 2, "border_bottom_width": 3, "margin_bottom": 5,
		"margin_height": 90,
	})
	assertApprox(t, "img_1.position_y", imgs[0].PositionY, 0)
	assertApprox(t, "img_2.position_y", imgs[1].PositionY, 40)
}

func TestBlockBoxDecorationBreakBlockClone(t *testing.T) {
	// https://www.w3.org/TR/css-backgrounds-3/#the-box-decoration-break
	pages := blockPagesN(t, `
      <style>
        @page { size: 100px }
        p { padding: 2px; border: 3px solid; margin: 5px;
            box-decoration-break: clone }
        img { display: block; height: 40px }
      </style>
      <p>
        <img src=pattern.png>
        <img src=pattern.png>
        <img src=pattern.png>
        <img src=pattern.png>`, 2)
	_, body := pageBody(t, pages[0])
	paragraph := only(t, body)
	imgs := unpack(t, paragraph, 2)
	// TODO: bottom margin should be 0
	// https://www.w3.org/TR/css-break-3/#valdef-box-decoration-break-clone
	// "Cloned margins are truncated on block-level boxes."
	// See issue #115.
	blockFields(t, "paragraph", paragraph, map[string]float64{
		"position_y": 0, "margin_top": 5, "border_top_width": 3,
		"padding_top": 2, "content_box_y": 10, "height": 80,
		"margin_bottom": 5, "border_bottom_width": 3, "padding_bottom": 2,
		"margin_height": 100,
	})
	assertApprox(t, "img_1.position_y", imgs[0].PositionY, 10)
	assertApprox(t, "img_2.position_y", imgs[1].PositionY, 50)

	_, body = pageBody(t, pages[1])
	paragraph = only(t, body)
	imgs = unpack(t, paragraph, 2)
	blockFields(t, "paragraph", paragraph, map[string]float64{
		"position_y": 0, "margin_top": 0, "border_top_width": 3,
		"padding_top": 2, "content_box_y": 5, "height": 80,
		"padding_bottom": 2, "border_bottom_width": 3, "margin_bottom": 5,
		"margin_height": 95,
	})
	assertApprox(t, "img_1.position_y", imgs[0].PositionY, 5)
	assertApprox(t, "img_2.position_y", imgs[1].PositionY, 45)
}

func TestBlockBoxDecorationBreakCloneBottomPadding(t *testing.T) {
	pages := blockPagesN(t, `
      <style>
        @page { size: 80px; margin: 0 }
        div { height: 20px }
        article { padding: 12px; box-decoration-break: clone }
      </style>
      <article>
        <div>a</div>
        <div>b</div>
        <div>c</div>
      </article>`, 2)
	_, body := pageBody(t, pages[0])
	article := only(t, body)
	assertApprox(t, "article.height", article.Height, 80-2*12)
	divs := unpack(t, article, 2)
	assertApprox(t, "div_1.position_y", divs[0].PositionY, 12)
	assertApprox(t, "div_2.position_y", divs[1].PositionY, 12+20)

	_, body = pageBody(t, pages[1])
	article = only(t, body)
	assertApprox(t, "article.height", article.Height, 20)
	assertApprox(t, "div.position_y", only(t, article).PositionY, 12)
}

func TestBlockBoxDecorationBreakSliceBottomPadding(t *testing.T) {
	t.Skip("upstream marks test_box_decoration_break_slice_bottom_padding as an expected failure")
}

func TestBlockNestedBlocksPaddingBorder(t *testing.T) {
	t.Skip("upstream marks test_nested_blocks_padding_border as an expected failure")
}

func TestBlockOverflowNonCollapsingParent(t *testing.T) {
	pages := blockPagesN(t, `
      <style>
        @page { size: 6px 10px; margin: 0 }
        html { font: 2px/1 weasyprint }
        div { border-bottom: 3px solid }
        p { margin-bottom: 2px }
      </style>
      abc
      def
      <div>
        <p>
          aaa
        </p>
      </div>
      ghi`, 2)
	_, body := pageBody(t, pages[0])
	unpack(t, only(t, body), 2)
	_, body = pageBody(t, pages[1])
	unpack(t, body, 2)
}

func TestBlockOverflowAuto(t *testing.T) {
	body := blockBody(t, `
      <article style="overflow: auto">
        <div style="float: left; height: 50px; margin: 10px">bla bla bla</div>
          toto toto`)
	assertApprox(t, "article.height", only(t, body).Height, 50+10+10)
}

func TestBlockOverflowHiddenInFlowLayout(t *testing.T) {
	body := blockBody(t, `
      <div style="overflow: hidden; height: 3px;">
        <div>abc</div>
        <div style="height: 100px; margin: 50px;">def</div>
      </div>
    `)
	assertApprox(t, "parent_div.height", only(t, body).Height, 3)
}

func TestBlockOverflowHiddenOutOfFlowLayout(t *testing.T) {
	body := blockBody(t, `
      <div style="overflow: hidden; height: 3px;">
        <div style="float: left;">abc</div>
        <div style="float: right; height: 100px; margin: 50px;">def</div>
      </div>
    `)
	assertApprox(t, "parent_div.height", only(t, body).Height, 3)
}

func TestBlockBoxMarginTopRepagination(t *testing.T) {
	// Regression test for #943.
	pages := blockPagesN(t, `
      <style>
        @page { size: 50px }
        :root { line-height: 1; font-size: 10px }
        a::before { content: target-counter(attr(href), page) }
        div { margin: 20px 0 0; background: yellow }
      </style>
      <p><a href="#title"></a></p>
      <div>1<br/>1<br/>2<br/>2</div>
      <h1 id="title">title</h1>
    `, 2)
	_, body := pageBody(t, pages[0])
	div := unpack(t, body, 2)[1]
	assertApprox(t, "div.margin_top", div.MarginTop, 20)
	assertApprox(t, "div.padding_box_y", div.PaddingBoxY(), 10+20)

	_, body = pageBody(t, pages[1])
	div = unpack(t, body, 2)[0]
	assertApprox(t, "div.margin_top", div.MarginTop, 0)
	assertApprox(t, "div.padding_box_y", div.PaddingBoxY(), 0)
}

func TestBlockContinueDiscard(t *testing.T) {
	body := blockBody(t, `
      <style>
        @page { size: 80px; margin: 0 }
        div { display: inline-block; width: 100%; height: 25px }
        article { continue: discard; border: 1px solid; line-height: 1 }
      </style>
      <article>
        <div>a</div>
        <div>b</div>
        <div>c</div>
        <div>d</div>
        <div>e</div>
        <div>f</div>
      </article>`)
	article := only(t, body)
	assertApprox(t, "article.height", article.Height, 3*25)
	divs := unpack(t, article, 3)
	for i, div := range divs {
		assertApprox(t, fmt.Sprintf("div_%d.position_y", i+1), div.PositionY, 1+25*float64(i))
	}
	assertApprox(t, "article.border_bottom_width", article.BorderBottomWidth, 1)
}

func TestBlockContinueDiscardChildren(t *testing.T) {
	body := blockBody(t, `
      <style>
        @page { size: 80px; margin: 0 }
        div { display: inline-block; width: 100%; height: 25px }
        section { border: 1px solid }
        article { continue: discard; border: 1px solid; line-height: 1 }
      </style>
      <article>
        <section>
          <div>a</div>
          <div>b</div>
          <div>c</div>
          <div>d</div>
          <div>e</div>
          <div>f</div>
        </section>
      </article>`)
	article := only(t, body)
	assertApprox(t, "article.height", article.Height, 2+3*25)
	section := only(t, article)
	assertApprox(t, "section.height", section.Height, 3*25)
	divs := unpack(t, section, 3)
	for i, div := range divs {
		assertApprox(t, fmt.Sprintf("div_%d.position_y", i+1), div.PositionY, 2+25*float64(i))
	}
	assertApprox(t, "article.border_bottom_width", article.BorderBottomWidth, 1)
}

func TestBlockInBlockWithBottomPadding(t *testing.T) {
	// Regression test for #1476.
	pages := blockPagesN(t, `
      <style>
        @page { size: 8em 3.5em }
        body { line-height: 1; font-family: weasyprint }
        div { padding-bottom: 1em }
      </style>
      abc def
      <div>
        <p>
          ghi jkl
          mno pqr
        </p>
      </div>
      stu vwx`, 2)

	checkLine := func(line *Box, text string) {
		t.Helper()
		assertApprox(t, "line.height", line.Height, 16)
		if len(line.Children) == 0 || line.Children[0].Text != text {
			t.Errorf("line %v, want %q", line.Children, text)
		}
	}
	_, body := pageBody(t, pages[0])
	children := unpack(t, body, 2)
	checkLine(only(t, children[0]), "abc def")
	checkLine(only(t, only(t, children[1])), "ghi jkl")

	_, body = pageBody(t, pages[1])
	children = unpack(t, body, 2)
	checkLine(only(t, only(t, children[0])), "mno pqr")
	line := only(t, children[1])
	checkLine(line, "stu vwx")
	assertApprox(t, "line.content_box_y", line.ContentBoxY(), 16+16) // p content + div padding
}

// blockPageBreaks lays out the document of the tests of the page breaks,
// with the style of the small div, and checks the positions of the divs on
// each page.
func blockPageBreaks(t *testing.T, large, small string, want [][]float64) {
	t.Helper()
	pages := blockPagesN(t, fmt.Sprintf(`
      <style>
        @page{
          size: 110px;
          margin: 10px;
          padding: 0;
        }
        .large {
          width: 10px;
          %s
        }
        .small {
          width: 10px;
          %s
        }
      </style>
      <body>
        <div class="large"></div>
        <div class="small"></div>
        <div class="large"></div>
    `, large, small), len(want))
	positions := make([][]float64, 0, len(pages))
	for _, page := range pages {
		var y []float64
		for _, div := range blockDescendants(page, "div") {
			y = append(y, div.PositionY)
		}
		positions = append(positions, y)
	}
	if fmt.Sprint(positions) != fmt.Sprint(want) {
		t.Errorf("positions_y = %v, want %v", positions, want)
	}
}

func TestBlockPageBreaks1(t *testing.T) {
	// last div does not fit, pushed to next page
	blockPageBreaks(t, "height: 60px;", "height: 20px;", [][]float64{{10, 70}, {10}})
}

func TestBlockPageBreaks2(t *testing.T) {
	// last div does not fit, pushed to next page
	// center div must not
	blockPageBreaks(t, "height: 60px;", "height: 20px; page-break-after: avoid;", [][]float64{{10}, {10, 30}})
}

func TestBlockPageBreaks3(t *testing.T) {
	// center div must be the last element,
	// but div won't fit and will get pushed anyway
	blockPageBreaks(t, "height: 80px;", "height: 20px; page-break-after: avoid;", [][]float64{{10}, {10}, {10}})
}

func TestBlockPageBreakChildMarginNoCollapse(t *testing.T) {
	// Regression test for #2453.
	pages := blockPagesN(t, `
      <style>
        @page{ size: 6px }
      </style>
      <body>
        <div style="height: 2px"></div>
        <section>
          <div style="height: 3px; margin-bottom: 2px"></div>
          <div style="display: flex">
            <span style="font: 2px weasyprint">a</span>
          </div>
        </section>
      </body>
    `, 2)
	_, body := pageBody(t, pages[0])
	only(t, unpack(t, body, 2)[1])
	_, body = pageBody(t, pages[1])
	only(t, only(t, body))
}

func TestBlockMinMaxRTL(t *testing.T) {
	body := blockBody(t, `
      <style>
        @page{ size: 10px }
      </style>
      <body style="direction: rtl">
        <div style="height: 5px; width: 1px; max-height: 4px; min-width: 3px"></div>
        <div style="height: 1px; width: 5px; min-height: 4px; max-width: 3px"></div>
      </body>
    `)
	divs := unpack(t, body, 2)
	blockFields(t, "div1", divs[0], map[string]float64{"position_x": 7, "position_y": 0, "height": 4, "width": 3})
	blockFields(t, "div2", divs[1], map[string]float64{"position_x": 7, "position_y": 4, "height": 4, "width": 3})
}
