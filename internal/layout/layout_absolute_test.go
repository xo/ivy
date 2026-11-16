// Copyright (c) 2011-2021, Simon Sapin and contributors.
//
// Use of this source code is governed by the BSD-3-Clause license that can be
// found in the LICENSE file.

// This file ports tests/layout/test_position.py: the tests of the property
// position, for the relative, the absolute and the fixed boxes.

package layout

import (
	"fmt"
	"testing"
)

// blockAssertXY checks the position of a box: (box.position_x,
// box.position_y) of upstream.
func blockAssertXY(t *testing.T, name string, box *Box, x, y float64) {
	t.Helper()
	if box.PositionX != x || box.PositionY != y {
		t.Errorf("(%s.position_x, %s.position_y) = (%v, %v), want (%v, %v)",
			name, name, box.PositionX, box.PositionY, x, y)
	}
}

// blockAssertWH checks the size of a box: (box.width, box.height) of
// upstream.
func blockAssertWH(t *testing.T, name string, box *Box, width, height float64) {
	t.Helper()
	if box.Width != width || box.Height != height {
		t.Errorf("(%s.width, %s.height) = (%v, %v), want (%v, %v)",
			name, name, box.Width, box.Height, width, height)
	}
}

// blockAssertTags checks the element tags of the children of a box.
func blockAssertTags(t *testing.T, box *Box, want ...string) {
	t.Helper()
	tags := make([]string, 0, len(box.Children))
	for _, child := range box.Children {
		tags = append(tags, child.ElementTag)
	}
	if fmt.Sprint(tags) != fmt.Sprint(want) {
		t.Errorf("tags of %v = %q, want %q", box, tags, want)
	}
}

func TestPositionRelativePositioning1(t *testing.T) {
	body := blockBody(t, `
      <style>
        p { height: 20px }
      </style>
      <p>1</p>
      <div style="position: relative; top: 10px">
        <p>2</p>
        <p style="position: relative; top: -5px; left: 5px">3</p>
        <p>4</p>
        <p style="position: relative; bottom: 5px; right: 5px">5</p>
        <p style="position: relative">6</p>
        <p>7</p>
      </div>
      <p>8</p>
    `)
	children := unpack(t, body, 3)
	p1, div, p8 := children[0], children[1], children[2]
	ps := unpack(t, div, 6)
	blockAssertXY(t, "p1", p1, 0, 0)
	blockAssertXY(t, "div", div, 0, 30)
	blockAssertXY(t, "p2", ps[0], 0, 30)
	blockAssertXY(t, "p3", ps[1], 5, 45) // (0 + 5, 50 - 5)
	blockAssertXY(t, "p4", ps[2], 0, 70)
	blockAssertXY(t, "p5", ps[3], -5, 85) // (0 - 5, 90 - 5)
	blockAssertXY(t, "p6", ps[4], 0, 110)
	blockAssertXY(t, "p7", ps[5], 0, 130)
	blockAssertXY(t, "p8", p8, 0, 140)
	assertApprox(t, "div.height", div.Height, 120)
}

func TestPositionRelativePositioning2(t *testing.T) {
	body := blockBody(t, `
      <style>
        img { width: 20px }
        body { font-size: 0 } /* Remove spaces */
      </style>
      <body>
      <span><img src=pattern.png></span>
      <span style="position: relative; left: 10px">
        <img src=pattern.png>
        <img src=pattern.png
             style="position: relative; left: -5px; top: 5px">
        <img src=pattern.png>
        <img src=pattern.png
             style="position: relative; right: 5px; bottom: 5px">
        <img src=pattern.png style="position: relative">
        <img src=pattern.png>
      </span>
      <span><img src=pattern.png></span>
    `)
	spans := unpack(t, only(t, body), 3)
	img1 := only(t, spans[0])
	imgs := unpack(t, spans[1], 6)
	img8 := only(t, spans[2])
	blockAssertXY(t, "img1", img1, 0, 0)
	// Don't test the span2.position_y because it depends on fonts
	assertApprox(t, "span2.position_x", spans[1].PositionX, 30)
	blockAssertXY(t, "img2", imgs[0], 30, 0)
	blockAssertXY(t, "img3", imgs[1], 45, 5) // (50 - 5, y + 5)
	blockAssertXY(t, "img4", imgs[2], 70, 0)
	blockAssertXY(t, "img5", imgs[3], 85, -5) // (90 - 5, y - 5)
	blockAssertXY(t, "img6", imgs[4], 110, 0)
	blockAssertXY(t, "img7", imgs[5], 130, 0)
	blockAssertXY(t, "img8", img8, 140, 0)
	assertApprox(t, "span2.width", spans[1].Width, 120)
}

func TestPositionRelativePositioning3(t *testing.T) {
	body := blockBody(t, `
      <style>
        img { width: 20px }
        body { font-size: 0 } /* Remove spaces */
      </style>
      <body>
      <span><img src=pattern.png></span>
      <span style="position: relative; left: 10px; right: 5px
        "><img src=pattern.png></span>
      <span><img src=pattern.png></span>
    `)
	spans := unpack(t, only(t, body), 3)
	assertApprox(t, "span2.position_x", spans[1].PositionX, 20+10)
}

func TestPositionRelativePositioning4(t *testing.T) {
	body := blockBody(t, `
      <style>
        img { width: 20px }
        body { direction: rtl; width: 100px;
               font-size: 0 } /* Remove spaces */
      </style>
      <body>
      <span><img src=pattern.png></span>
      <span style="position: relative; left: 10px; right: 5px
        "><img src=pattern.png></span>
      <span><img src=pattern.png></span>
    `)
	spans := unpack(t, only(t, body), 3)
	assertApprox(t, "span2.position_x", spans[1].PositionX, 100-20-5-20)
}

func TestPositionAbsolutePositioning1(t *testing.T) {
	body := blockBody(t, `
      <div style="margin: 3px">
        <div style="height: 20px; width: 20px; position: absolute"></div>
        <div style="height: 20px; width: 20px; position: absolute;
                    left: 0"></div>
        <div style="height: 20px; width: 20px; position: absolute;
                    top: 0"></div>
      </div>
    `)
	div1 := only(t, body)
	divs := unpack(t, div1, 3)
	assertApprox(t, "div1.height", div1.Height, 0)
	blockAssertXY(t, "div1", div1, 0, 0)
	for i, xy := range [][2]float64{{3, 3}, {0, 3}, {3, 0}} {
		name := fmt.Sprintf("div%d", i+2)
		blockAssertWH(t, name, divs[i], 20, 20)
		blockAssertXY(t, name, divs[i], xy[0], xy[1])
	}
}

func TestPositionAbsolutePositioning2(t *testing.T) {
	body := blockBody(t, `
      <div style="position: relative; width: 20px">
        <div style="height: 20px; width: 20px; position: absolute"></div>
        <div style="height: 20px; width: 20px"></div>
      </div>
    `)
	div1 := only(t, body)
	divs := unpack(t, div1, 2)
	for i, div := range []*Box{div1, divs[0], divs[1]} {
		name := fmt.Sprintf("div%d", i+1)
		blockAssertXY(t, name, div, 0, 0)
		blockAssertWH(t, name, div, 20, 20)
	}
}

func TestPositionAbsolutePositioning3(t *testing.T) {
	body := blockBody(t, `
      <body style="font-size: 0">
        <img src=pattern.png>
        <span style="position: relative">
          <span style="position: absolute">2</span>
          <span style="position: absolute">3</span>
          <span>4</span>
        </span>
    `)
	children := unpack(t, only(t, body), 2)
	span1 := children[1]
	spans := unpack(t, span1, 3)
	assertApprox(t, "span1.position_x", span1.PositionX, 4)
	blockAssertXY(t, "span2", spans[0], 4, 0)
	blockAssertXY(t, "span3", spans[1], 4, 0)
	assertApprox(t, "span4.position_x", spans[2].PositionX, 4)
}

func TestPositionAbsolutePositioning4(t *testing.T) {
	body := blockBody(t, `
      <style> img { width: 5px; height: 20px} </style>
      <body style="font-size: 0">
        <img src=pattern.png>
        <span style="position: absolute">2</span>
        <img src=pattern.png>
    `)
	children := unpack(t, only(t, body), 3)
	blockAssertXY(t, "img1", children[0], 0, 0)
	blockAssertXY(t, "span", children[1], 5, 0)
	blockAssertXY(t, "img2", children[2], 5, 0)
}

func TestPositionAbsolutePositioning5(t *testing.T) {
	body := blockBody(t, `
      <style> img { width: 5px; height: 20px} </style>
      <body style="font-size: 0">
        <img src=pattern.png>
        <span style="position: absolute; display: block">2</span>
        <img src=pattern.png>
    `)
	children := unpack(t, only(t, body), 3)
	blockAssertXY(t, "img1", children[0], 0, 0)
	blockAssertXY(t, "span", children[1], 0, 20)
	blockAssertXY(t, "img2", children[2], 5, 0)
}

func TestPositionAbsolutePositioning6(t *testing.T) {
	body := blockBody(t, `
      <div style="position: relative; width: 20px; height: 60px;
                  border: 10px solid; padding-top: 6px; top: 5px; left: 1px">
        <div style="height: 20px; width: 20px; position: absolute;
                    bottom: 50%"></div>
        <div style="height: 20px; width: 20px; position: absolute;
                    top: 13px"></div>
      </div>
    `)
	div1 := only(t, body)
	divs := unpack(t, div1, 2)
	blockAssertXY(t, "div1", div1, 1, 5)
	blockAssertWH(t, "div1", div1, 20, 60)
	assertApprox(t, "div1.border_width", div1.BorderWidth(), 40)
	assertApprox(t, "div1.border_height", div1.BorderHeight(), 86)
	blockAssertXY(t, "div2", divs[0], 11, 28)
	blockAssertWH(t, "div2", divs[0], 20, 20)
	blockAssertXY(t, "div3", divs[1], 11, 28)
	blockAssertWH(t, "div3", divs[1], 20, 20)
}

func TestPositionAbsolutePositioning7(t *testing.T) {
	body := blockBody(t, `
      <style>
        @page { size: 1000px 2000px }
        html { font-size: 0 }
        p { height: 20px }
      </style>
      <p>1</p>
      <div style="width: 100px">
        <p>2</p>
        <p style="position: absolute; top: -5px; left: 5px">3</p>
        <p style="margin: 3px">4</p>
        <p style="position: absolute; bottom: 5px; right: 15px;
                  width: 50px; height: 10%;
                  padding: 3px; margin: 7px">5
          <span>
            <img src="pattern.png">
            <span style="position: absolute"></span>
            <span style="position: absolute; top: -10px; right: 5px;
                         width: 20px; height: 15px"></span>
          </span>
        </p>
        <p style="margin-top: 8px">6</p>
      </div>
      <p>7</p>
    `)
	children := unpack(t, body, 3)
	p1, div, p7 := children[0], children[1], children[2]
	ps := unpack(t, div, 5)
	spans := unpack(t, only(t, only(t, ps[3])), 3)
	blockAssertXY(t, "p1", p1, 0, 0)
	blockAssertXY(t, "div", div, 0, 20)
	blockAssertXY(t, "p2", ps[0], 0, 20)
	blockAssertXY(t, "p3", ps[1], 5, -5)
	blockAssertXY(t, "p4", ps[2], 0, 40)
	// p5 x = page width - right - margin/padding/border - width
	//      = 1000       - 15    - 2 * 10                - 50
	//      = 915
	// p5 y = page height - bottom - margin/padding/border - height
	//      = 2000        - 5      - 2 * 10                - 200
	//      = 1775
	blockAssertXY(t, "p5", ps[3], 915, 1775)
	blockAssertXY(t, "img", spans[0], 925, 1785)
	blockAssertXY(t, "span2", spans[1], 929, 1785)
	// span3 x = p5 right - p5 margin - span width - span right
	//         = 985      - 7         - 20         - 5
	//         = 953
	// span3 y = p5 y + p5 margin top + span top
	//         = 1775 + 7             + -10
	//         = 1772
	blockAssertXY(t, "span3", spans[2], 953, 1772)
	// p6 y = p4 y + p4 margin height - margin collapsing
	//      = 40   + 26               - 3
	//      = 63
	blockAssertXY(t, "p6", ps[4], 0, 63)
	assertApprox(t, "div.height", div.Height, 71) // 20*3 + 2*3 + 8 - 3
	blockAssertXY(t, "p7", p7, 0, 91)
}

func TestPositionAbsolutePositioning8(t *testing.T) {
	// Regression test for #1264.
	body := blockBody(t, `
      <style>@page{ width: 50px; height: 50px }</style>
      <body style="font-size: 0">
        <div style="position: absolute; margin: auto;
                    left: 0; right: 10px;
                    top: 0; bottom: 10px;
                    width: 10px; height: 20px">
    `)
	div := only(t, body)
	assertApprox(t, "div.content_box_x", div.ContentBoxX(), 15)
	assertApprox(t, "div.content_box_y", div.ContentBoxY(), 10)
	blockAssertWH(t, "div", div, 10, 20)
}

func TestPositionAbsoluteImages(t *testing.T) {
	body := blockBody(t, `
      <style>
        @page { size: 50px; }
        img { display: block; position: absolute }
      </style>
      <div style="margin: 10px">
        <img src=pattern.png />
        <img src=pattern.png style="left: 15px" />
        <img src=pattern.png style="top: 15px" />
        <img src=pattern.png style="bottom: 25px" />
      </div>
    `)
	div := only(t, body)
	imgs := unpack(t, div, 4)
	assertApprox(t, "div.height", div.Height, 0)
	blockAssertXY(t, "div", div, 0, 0)
	for i, xy := range [][2]float64{{10, 10}, {15, 10}, {10, 15}, {10, 21}} { // (50 - 4) - 25
		name := fmt.Sprintf("img%d", i+1)
		blockAssertXY(t, name, imgs[i], xy[0], xy[1])
		blockAssertWH(t, name, imgs[i], 4, 4)
	}

	// TODO: test the various cases in absolute_replaced()
}

func TestPositionAbsoluteInAbsoluteNextPage(t *testing.T) {
	// Regression test for #2714.
	pages := blockPagesN(t, `
      <style>
        @page { size: 3px 9px }
        body { font: 2px/1 weasyprint }
        article { height: 8px }
        div { position: absolute }
      </style>
      <article>a</article>
      <article style="position: relative"><div><div>b<br>c
    `, 2)
	_, body := pageBody(t, pages[0])
	line := only(t, only(t, body))
	blockAssertText(t, "line", line.Children[0], "a")

	_, body = pageBody(t, pages[1])
	div2 := only(t, only(t, only(t, body)))
	lines := unpack(t, div2, 2)
	blockAssertText(t, "line_1", lines[0].Children[0], "b")
	blockAssertText(t, "line_2", lines[1].Children[0], "c")
}

func TestPositionFixedPositioning(t *testing.T) {
	// TODO:test page-break-before: left/right
	pages := blockPagesN(t, `
      a
      <div style="page-break-before: always; page-break-after: always">
        <p style="position: fixed">b</p>
      </div>
      c
    `, 3)
	blockAssertTags(t, only(t, pages[0]), "body", "p")
	_, body := pageBody(t, pages[1])
	blockAssertTags(t, only(t, body), "p")
	blockAssertTags(t, only(t, pages[2]), "p", "body")
}

func TestPositionFixedPositioningRegression1(t *testing.T) {
	// Regression test for #641.
	pages := blockPagesN(t, `
      <style>
        @page:first { size: 100px 200px }
        @page { size: 200px 100px; margin: 0 }
        article { break-after: page }
        .fixed { position: fixed; top: 10px; width: 20px }
      </style>
      <ul class="fixed" style="right: 0"><li>a</li></ul>
      <img class="fixed" style="right: 20px" src="pattern.png" />
      <div class="fixed" style="right: 40px">b</div>
      <article>page1</article>
      <article>page2</article>
    `, 2)

	_, body := pageBody(t, pages[0])
	children := unpack(t, body, 4)
	ul, img, div, article := children[0], children[1], children[2], children[3]
	blockAssertXY(t, "ul", ul, 80, 10)
	blockAssertXY(t, "img", img, 60, 10)
	blockAssertXY(t, "div", div, 40, 10)
	blockAssertXY(t, "article", article, 0, 0)
	assertApprox(t, "marker.position_x", ul.Children[0].PositionX, ul.PositionX)

	children = unpack(t, only(t, pages[1]), 4)
	ul, img, div = children[0], children[1], children[2]
	blockAssertXY(t, "ul", ul, 180, 10)
	blockAssertXY(t, "img", img, 160, 10)
	blockAssertXY(t, "div", div, 140, 10)
	// Upstream checks the article of the first page again.
	blockAssertXY(t, "article", article, 0, 0)
	assertApprox(t, "marker.position_x", ul.Children[0].PositionX, ul.PositionX)
}

func TestPositionFixedPositioningRegression2(t *testing.T) {
	// Regression test for #728.
	pages := blockPagesN(t, `
      <style>
        @page { size: 100px 100px }
        section { break-after: page }
        .fixed { position: fixed; top: 10px; left: 15px; width: 20px }
      </style>
      <div class="fixed">
        <article class="fixed" style="top: 20px">
          <header class="fixed" style="left: 5px"></header>
        </article>
      </div>
      <section></section>
      <pre></pre>
    `, 2)
	check := func(div *Box) {
		t.Helper()
		blockAssertXY(t, "div", div, 15, 10)
		article := only(t, div)
		blockAssertXY(t, "article", article, 15, 20)
		blockAssertXY(t, "header", only(t, article), 5, 10)
	}
	_, body := pageBody(t, pages[0])
	check(unpack(t, body, 2)[0])
	check(unpack(t, only(t, pages[1]), 2)[0])
}

func TestPositionFlexRelativePositioning(t *testing.T) {
	body := blockBody(t, `
      <style>
        @page { size: 100px 100px }
        article { display: flex }
        .box { width: 20px; height: 20px }
        .relative { position: relative; top: 10px; left: 10px }
      </style>
      <article>
        <div class="box"></div>
        <div class="box relative"></div>
      </article>
    `)
	divs := unpack(t, only(t, body), 2)
	blockAssertXY(t, "div2", divs[1], 30, 10)
}

func TestPositionGridRelativePositioning(t *testing.T) {
	body := blockBody(t, `
      <style>
        @page { size: 100px 100px }
        article { display: grid; grid-template-columns: 20px 20px }
        .box { width: 20px; height: 20px }
        .relative { position: relative; top: 10px; left: 10px }
      </style>
      <article>
        <div class="box"></div>
        <div class="box relative"></div>
      </article>
    `)
	divs := unpack(t, only(t, body), 2)
	blockAssertXY(t, "div2", divs[1], 30, 10)
}

func TestPositionFlexAbsolutePositioning(t *testing.T) {
	t.Skip("upstream marks test_flex_absolute_positioning as an expected failure")
}

func TestPositionGridAbsolutePositioning(t *testing.T) {
	t.Skip("upstream marks test_grid_absolute_positioning as an expected failure")
}
