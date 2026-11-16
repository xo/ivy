// Copyright (c) 2011-2021, Simon Sapin and contributors.
//
// Use of this source code is governed by the BSD-3-Clause license that can be
// found in the LICENSE file.

// This file ports tests/layout/test_inline_block.py, the tests of the
// layout of the inline blocks, tests/layout/test_list.py, the tests of the
// layout of the lists, and tests/layout/test_logical.py, the tests of the
// logical properties.

package layout

import (
	"fmt"
	"testing"
)

func TestInlineBlockSizes(t *testing.T) {
	page := blockPage(t, `
      <style>
        @page { margin: 0; size: 200px 2000px }
        body { margin: 0 }
        div { display: inline-block; }
      </style>
      <div> </div>
      <div>a</div>
      <div style="margin: 10px; height: 100px"></div>
      <div style="margin-left: 10px; margin-top: -50px;
                  padding-right: 20px;"></div>
      <div>
        Ipsum dolor sit amet,
        consectetur adipiscing elit.
        Sed sollicitudin nibh
        et turpis molestie tristique.
      </div>
      <div style="width: 100px; height: 100px;
                  padding-left: 10px; margin-right: 10px;
                  margin-top: -10px; margin-bottom: 50px"></div>
      <div style="font-size: 0">
        <div style="min-width: 10px; height: 10px"></div>
        <div style="width: 10%">
          <div style="width: 10px; height: 10px"></div>
        </div>
      </div>
      <div style="min-width: 150px">foo</div>
      <div style="max-width: 10px
        ">Supercalifragilisticexpialidocious</div>`)
	html, body := pageBody(t, page)
	if html.ElementTag != "html" || body.ElementTag != "body" {
		t.Fatalf("tags %q %q", html.ElementTag, body.ElementTag)
	}
	assertApprox(t, "body.width", body.Width, 200)
	lines := unpack(t, body, 4)

	// First line:
	// White space in-between divs ends up preserved in TextBoxes
	children := unpack(t, lines[0], 8)
	div1, div2, div3, div4 := children[0], children[2], children[4], children[6]
	for _, div := range []*Box{div1, div2, div3, div4} {
		if div.ElementTag != "div" {
			t.Errorf("%v is not a div", div)
		}
	}
	// First div, one ignored space collapsing with next space
	assertApprox(t, "div_1.width", div1.Width, 0)
	// Second div, one letter
	if !(0 < div2.Width && div2.Width < 20) {
		t.Errorf("div_2.width = %v", div2.Width)
	}
	// Third div, empty with margin
	assertApprox(t, "div_3.width", div3.Width, 0)
	assertApprox(t, "div_3.margin_width", div3.MarginWidth(), 20)
	assertApprox(t, "div_3.height", div3.Height, 100)
	// Fourth div, empty with margin and padding
	assertApprox(t, "div_4.width", div4.Width, 0)
	assertApprox(t, "div_4.margin_width", div4.MarginWidth(), 30)

	// Second line:
	div5 := unpack(t, lines[1], 2)[0]
	// Fifth div, long text, full-width div
	if div5.ElementTag != "div" || len(div5.Children) <= 1 {
		t.Errorf("div_5 is %v with %d children", div5, len(div5.Children))
	}
	assertApprox(t, "div_5.width", div5.Width, 200)

	// Third line:
	children = unpack(t, lines[2], 4)
	div6, div7 := children[0], children[2]
	// Sixth div, empty div with fixed width and height
	assertApprox(t, "div_6.width", div6.Width, 100)
	assertApprox(t, "div_6.margin_width", div6.MarginWidth(), 120)
	assertApprox(t, "div_6.height", div6.Height, 100)
	assertApprox(t, "div_6.margin_height", div6.MarginHeight(), 140)
	// Seventh div
	assertApprox(t, "div_7.width", div7.Width, 20)
	childLine := only(t, div7)
	// Spaces have font-size: 0, they get removed
	childDivs := unpack(t, childLine, 2)
	assertApprox(t, "child_div_1.width", childDivs[0].Width, 10)
	assertApprox(t, "child_div_2.width", childDivs[1].Width, 2)
	grandchild := only(t, childDivs[1])
	if grandchild.ElementTag != "div" {
		t.Errorf("grandchild is %v", grandchild)
	}
	assertApprox(t, "grandchild.width", grandchild.Width, 10)

	children = unpack(t, lines[3], 3)
	assertApprox(t, "div_8.width", children[0].Width, 150)
	assertApprox(t, "div_9.width", children[2].Width, 10)
}

func TestInlineBlockWithMargin(t *testing.T) {
	// Regression test for #1235.
	body := blockBody(t, `
      <style>
        @page { size: 100px }
        span { font-family: weasyprint; display: inline-block; margin: 0 30px }
      </style>
      <span>a b c d e f g h i j k l</span>`)
	span := only(t, only(t, body))
	assertApprox(t, "span.width", span.Width, 40) // 100 - 2 * 30
}

func TestListsStyle(t *testing.T) {
	for _, inside := range []string{"inside", ""} {
		for _, c := range []struct{ style, character string }{
			{"circle", "◦ "},
			{"disc", "• "},
			{"square", "▪ "},
		} {
			t.Run(inside+" "+c.style, func(t *testing.T) {
				body := blockBody(t, fmt.Sprintf(`
      <style>
        body { margin: 0 }
        ul { margin-left: 50px; list-style: %s %s }
      </style>
      <ul>
        <li>abc</li>
      </ul>
    `, inside, c.style))
				listItem := only(t, only(t, body))
				var markerText, content *Box
				if inside != "" {
					children := unpack(t, only(t, listItem), 2)
					markerText = only(t, children[0])
					content = children[1]
				} else {
					children := unpack(t, listItem, 2)
					marker, lineContainer := children[0], children[1]
					assertApprox(t, "marker.position_x", marker.PositionX, listItem.PositionX-marker.Width)
					assertApprox(t, "marker.position_y", marker.PositionY, listItem.PositionY)
					content = only(t, only(t, lineContainer))
					markerText = only(t, only(t, marker))
				}
				blockAssertText(t, "marker_text", markerText, c.character)
				blockAssertText(t, "content", content, "abc")
			})
		}
	}
}

func TestListsEmptyItem(t *testing.T) {
	// Regression test for #873.
	body := blockBody(t, `
      <ul>
        <li>a</li>
        <li></li>
        <li>a</li>
      </ul>
    `)
	lis := unpack(t, only(t, body), 3)
	if lis[0].PositionY == lis[1].PositionY || lis[1].PositionY == lis[2].PositionY {
		t.Errorf("positions_y %v %v %v", lis[0].PositionY, lis[1].PositionY, lis[2].PositionY)
	}
}

func TestListsWhitespaceItem(t *testing.T) {
	t.Skip("upstream marks test_lists_whitespace_item as an expected failure")
}

func TestListsPageBreak(t *testing.T) {
	// Regression test for #945.
	pages := blockPagesN(t, `
      <style>
        @page { size: 300px 100px }
        ul { font-size: 30px; font-family: weasyprint; margin: 0 }
      </style>
      <ul>
        <li>a</li>
        <li>a</li>
        <li>a</li>
        <li>a</li>
      </ul>
    `, 2)
	for i, n := range []int{3, 1} {
		_, body := pageBody(t, pages[i])
		for _, li := range unpack(t, only(t, body), n) {
			unpack(t, li, 2)
		}
	}
}

func TestListsPageBreakMargin(t *testing.T) {
	// Regression test for #1058.
	pages := blockPagesN(t, `
      <style>
        @page { size: 300px 100px }
        ul { font-size: 30px; font-family: weasyprint; margin: 0 }
        p { margin: 10px 0 }
      </style>
      <ul>
        <li><p>a</p></li>
        <li><p>a</p></li>
        <li><p>a</p></li>
        <li><p>a</p></li>
      </ul>
    `, 2)
	for _, page := range pages {
		_, body := pageBody(t, page)
		for _, li := range unpack(t, only(t, body), 2) {
			children := unpack(t, li, 2)
			if len(children[1].Children) == 0 {
				t.Fatalf("%v has no child", children[1])
			}
			assertApprox(t, "marker.position_y", children[0].PositionY, children[1].Children[0].PositionY)
		}
	}
}

func TestListsMarkerContent(t *testing.T) {
	// Regression test for #2918.
	body := blockBody(t, `
      <style>
        ::marker { content: 'a' }
      </style>
      <ul>
        <li>b</li>
      </ul>
    `)
	children := unpack(t, only(t, only(t, body)), 2)
	blockAssertText(t, "marker", children[0].Children[0].Children[0], "a")
	blockAssertText(t, "li", children[1].Children[0].Children[0], "b")
}

func TestLogical(t *testing.T) {
	baseStyle := `
    <style>
      @page { size: 10px }
      div { background: pink; background-clip: padding-box; height: 5px }
    </style>
    `
	for i, documents := range [][]string{
		{
			`<div style="border-top: 1px solid">`,
			`<div style="border-block-start: 1px solid">`,
			`<div style="--size: 1px; border-block-start: var(--size) solid">`,
		}, {
			`<div style="border-left: 1px solid">`,
			`<div style="border-inline-start: 1px solid">`,
			`<div style="--size: 1px; border-inline-start: var(--size) solid">`,
		}, {
			`<div style="border-inline-start: 1px solid">`,
			`<article style="direction: rtl">` +
				`  <div style="border-inline-end: 1px solid">`,
		}, {
			`<div style="border-style: solid; border-width: 1px 2px">`,
			`<div style="border-style: solid; border-width: logical 1px 2px">`,
			// TODO: handle this case.
			// '<div style="border-style: solid;'
			// '  --size: 1px; border-width: logical var(--size) 2px">',
		}, {
			`<div style="padding-top: 1px">`,
			`<div style="padding-block-start: 1px">`,
		}, {
			`<div style="padding: 1px 2px 3px 4px">`,
			`<div style="padding: logical 1px 4px 3px 2px">`,
			`<div style="padding: 1px 2px;` +
				`            padding-inline-start: 4px; padding-block-end: 3px">`,
		}, {
			`<div style="margin-right: 1px">`,
			`<div style="margin-inline-end: 1px">`,
			`<article style="direction: rtl">` +
				`  <div style="margin-inline-start: 1px">`,
			`<article style="direction: rtl">` +
				`  <div style="margin: logical 0 1px 0 0">`,
		}, {
			`<div style="border-radius: 0 0 0 4px">`,
			`<div style="border-bottom-left-radius: 4px">`,
			`<div style="border-end-start-radius: 4px">`,
			`<article style="direction: rtl">` +
				`  <div style="border-end-end-radius: 4px">`,
		}, {
			`<div style="width: 5px">`,
			`<div style="inline-size: 5px">`,
			`<div style="max-inline-size: 5px">`,
			`<div style="max-inline-size: 5px; min-block-size: 2px">`,
		}, {
			`<div style="height: 6px">`,
			`<div style="block-size: 6px">`,
			`<div style="min-block-size: 6px">`,
			`<div style="min-block-size: 6px; max-inline-size: 10px">`,
		}, {
			`<div style="position: absolute; width: 5px; inset: auto 1px auto auto">`,
			`<div style="position: absolute; width: 5px; right: 1px">`,
			`<div style="position: absolute; width: 5px; inset-inline-end: 1px">`,
			`<div style="position: absolute; width: 5px; inset-inline: auto 1px">`,
			`<div style="position: absolute; width: 5px;` +
				`  --size: 1px; inset-inline: auto var(--size)">`,
		}, {
			`<div style="float: left; width: 5px">`,
			`<div style="float: inline-start; width: 5px">`,
			`<article style="direction: rtl">` +
				`  <div style="float: inline-end; width: 5px">`,
		}, {
			`<div style="float: left; width: 5px"></div>` +
				`<div style="float: left; width: 3px"></div>`,
			`<div style="float: left; width: 5px"></div>` +
				`<div style="float: left; width: 3px; clear: right"></div>`,
			`<div style="float: inline-start; width: 5px"></div>` +
				`<div style="float: left; width: 3px; clear: inline-end"></div>`,
			`<div style="float: left; width: 5px"></div>` +
				`<div style="float: inline-start; width: 3px; clear: inline-end"></div>`,
			`<article style="direction: rtl">` +
				`  <div style="float: left; width: 5px"></div>` +
				`  <div style="float: inline-end; width: 3px; clear: inline-start"></div>`,
		}, {
			`<div style="float: left; width: 5px"></div>` +
				`<div style="float: left; width: 3px; clear: left"></div>`,
			`<div style="float: inline-start; width: 5px"></div>` +
				`<div style="float: left; width: 3px; clear: inline-start"></div>`,
			`<div style="float: inline-start; width: 5px"></div>` +
				`<div style="float: inline-start; width: 3px; clear: left"></div>`,
			`<div style="float: inline-start; width: 5px"></div>` +
				`<div style="float: inline-start; width: 3px; clear: both"></div>`,
			`<article style="direction: rtl">` +
				`  <div style="float: inline-end; width: 5px"></div>` +
				`  <div style="float: inline-end; width: 3px; clear: left"></div>`,
			`<article style="direction: rtl">` +
				`  <div style="float: left; width: 5px"></div>` +
				`  <div style="float: inline-end; width: 3px; clear: inline-end"></div>`,
		},
	} {
		t.Run(fmt.Sprint(i), func(t *testing.T) {
			sources := make([]string, len(documents))
			for i, document := range documents {
				sources[i] = baseStyle + document
			}
			assertSameRenderings(t, "logical", 0, sources...)
		})
	}
}
