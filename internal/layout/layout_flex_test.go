// Copyright (c) 2011-2021, Simon Sapin and contributors.
//
// Use of this source code is governed by the BSD-3-Clause license that can be
// found in the LICENSE file.

// This file ports tests/layout/test_flex.py: the tests of the layout of the
// flex containers.

package layout

import (
	"fmt"
	"log/slog"
	"testing"
)

// flexPages renders a document, as render_pages does with assert_no_logs:
// the layout must write no log. It fails the test when the document does
// not make n pages.
func flexPages(t *testing.T, source string, n int) []*Box {
	t.Helper()
	log, c := captureLogs(slog.LevelInfo)
	pages := renderPagesLog(t, source, testBaseURL, log)
	assertNoLogs(t, c)
	if len(pages) != n {
		t.Fatalf("got %d pages, want %d", len(pages), n)
	}
	return pages
}

// flexBody renders a document of one page and returns the box of its body.
func flexBody(t *testing.T, source string) *Box {
	t.Helper()
	_, body := pageBody(t, flexPages(t, source, 1)[0])
	return body
}

// flexText returns the text of the first child of the child of index i of
// box: box.children[i].children[0].text of upstream.
func flexText(t *testing.T, box *Box, i int) string {
	t.Helper()
	if i >= len(box.Children) || len(box.Children[i].Children) == 0 {
		t.Fatalf("%s has no text at %d", box, i)
	}
	return box.Children[i].Children[0].Text
}

// flexTexts checks the texts of the first line of each box: the text of
// box.children[0].children[0] of upstream.
func flexTexts(t *testing.T, boxes []*Box, want ...string) {
	t.Helper()
	for i, box := range boxes {
		if got := flexText(t, box, 0); got != want[i] {
			t.Errorf("text %d = %q, want %q", i, got, want[i])
		}
	}
}

// flexEq checks that each value is want, as the operator == does.
func flexEq(t *testing.T, name string, want float64, got ...float64) {
	t.Helper()
	for i, value := range got {
		if value != want {
			t.Errorf("%s: value %d = %v, want %v", name, i, value, want)
		}
	}
}

// flexLess checks that the values increase strictly, as a < b < c does.
func flexLess(t *testing.T, name string, values ...float64) {
	t.Helper()
	for i := 1; i < len(values); i++ {
		if !(values[i-1] < values[i]) {
			t.Errorf("%s: %v is not less than %v", name, values[i-1], values[i])
		}
	}
}

// flexGreater checks that the values decrease strictly, as a > b > c does.
func flexGreater(t *testing.T, name string, values ...float64) {
	t.Helper()
	for i := 1; i < len(values); i++ {
		if !(values[i-1] > values[i]) {
			t.Errorf("%s: %v is not greater than %v", name, values[i-1], values[i])
		}
	}
}

// flexThree renders a document of one page whose body holds one flex
// container of three children, and returns the container and its children.
func flexThree(t *testing.T, source string) (article, div1, div2, div3 *Box) {
	t.Helper()
	article = only(t, flexBody(t, source))
	divs := unpack(t, article, 3)
	return article, divs[0], divs[1], divs[2]
}

func TestFlexDirectionRow(t *testing.T) {
	article, div1, div2, div3 := flexThree(t, `
      <article style="display: flex">
        <div>A</div>
        <div>B</div>
        <div>C</div>
      </article>
    `)
	flexTexts(t, []*Box{div1, div2, div3}, "A", "B", "C")
	flexEq(t, "position_y", article.PositionY, div1.PositionY, div2.PositionY, div3.PositionY)
	flexEq(t, "div1.position_x", article.PositionX, div1.PositionX)
	flexLess(t, "position_x", div1.PositionX, div2.PositionX, div3.PositionX)
}

func TestFlexDirectionRowMaxWidth(t *testing.T) {
	article := only(t, flexBody(t, `
      <article style="display: flex; max-width: 100px">
        <div></div>
      </article>
    `))
	flexEq(t, "article.width", 100, article.Width)
}

func TestFlexDirectionRowMinHeight(t *testing.T) {
	article := only(t, flexBody(t, `
      <article style="display: flex; min-height: 100px">
        <div></div>
      </article>
    `))
	flexEq(t, "article.height", 100, article.Height)
}

func TestFlexDirectionRowRTL(t *testing.T) {
	article, div1, div2, div3 := flexThree(t, `
      <article style="display: flex; direction: rtl">
        <div>A</div>
        <div>B</div>
        <div>C</div>
      </article>
    `)
	flexTexts(t, []*Box{div1, div2, div3}, "A", "B", "C")
	flexEq(t, "position_y", article.PositionY, div1.PositionY, div2.PositionY, div3.PositionY)
	flexEq(t, "div1 right", article.PositionX+article.Width, div1.PositionX+div1.Width)
	flexGreater(t, "position_x", div1.PositionX, div2.PositionX, div3.PositionX)
}

func TestFlexDirectionRowRTLGap(t *testing.T) {
	article, div1, div2, div3 := flexThree(t, `
      <article style="display: flex; direction: rtl; gap: 10px">
        <div>A</div>
        <div>B</div>
        <div>C</div>
      </article>
    `)
	flexTexts(t, []*Box{div1, div2, div3}, "A", "B", "C")
	flexEq(t, "position_y", article.PositionY, div1.PositionY, div2.PositionY, div3.PositionY)
	flexEq(t, "div1 right", article.PositionX+article.Width, div1.PositionX+div1.Width)
	flexEq(t, "div1.position_x", div2.PositionX+div2.Width+10, div1.PositionX)
	flexEq(t, "div2.position_x", div3.PositionX+div3.Width+10, div2.PositionX)
}

func TestFlexDirectionRowRTLGapMultiline(t *testing.T) {
	// Regression test for issue #3014.
	article := only(t, flexBody(t, `
      <article style="display: flex; flex-wrap: wrap; direction: rtl;
                      gap: 10px; width: 75px">
        <div style="width: 30px; height: 40px">A</div>
        <div style="width: 30px; height: 40px">B</div>
        <div style="width: 30px; height: 40px">C</div>
        <div style="width: 30px; height: 40px">D</div>
      </article>
    `))
	divs := unpack(t, article, 4)
	div1, div2, div3, div4 := divs[0], divs[1], divs[2], divs[3]
	flexTexts(t, divs, "A", "B", "C", "D")
	flexEq(t, "article.width", 75, article.Width)
	flexEq(t, "position_y 1", article.PositionY, div1.PositionY, div2.PositionY)
	flexEq(t, "position_y 2", article.PositionY+40+10, div3.PositionY, div4.PositionY)
	flexEq(t, "position_x 1", article.PositionX+article.Width-30, div1.PositionX, div3.PositionX)
	flexEq(t, "position_x 2", article.PositionX+article.Width-70, div2.PositionX, div4.PositionX)
}

func TestFlexDirectionRowReverse(t *testing.T) {
	article, div1, div2, div3 := flexThree(t, `
      <article style="display: flex; flex-direction: row-reverse">
        <div>A</div>
        <div>B</div>
        <div>C</div>
      </article>
    `)
	flexTexts(t, []*Box{div1, div2, div3}, "C", "B", "A")
	flexEq(t, "position_y", article.PositionY, div1.PositionY, div2.PositionY, div3.PositionY)
	flexEq(t, "div3 right", article.PositionX+article.Width, div3.PositionX+div3.Width)
	flexLess(t, "position_x", div1.PositionX, div2.PositionX, div3.PositionX)
}

func TestFlexDirectionRowReverseRTL(t *testing.T) {
	article, div1, div2, div3 := flexThree(t, `
      <article style="display: flex; flex-direction: row-reverse;
      direction: rtl">
        <div>A</div>
        <div>B</div>
        <div>C</div>
      </article>
    `)
	flexTexts(t, []*Box{div1, div2, div3}, "C", "B", "A")
	flexEq(t, "position_y", article.PositionY, div1.PositionY, div2.PositionY, div3.PositionY)
	flexEq(t, "div3.position_x", article.PositionX, div3.PositionX)
	flexGreater(t, "position_x", div1.PositionX, div2.PositionX, div3.PositionX)
}

func TestFlexDirectionColumn(t *testing.T) {
	article, div1, div2, div3 := flexThree(t, `
      <article style="display: flex; flex-direction: column">
        <div>A</div>
        <div>B</div>
        <div>C</div>
      </article>
    `)
	flexTexts(t, []*Box{div1, div2, div3}, "A", "B", "C")
	flexEq(t, "position_x", article.PositionX, div1.PositionX, div2.PositionX, div3.PositionX)
	flexEq(t, "div1.position_y", article.PositionY, div1.PositionY)
	flexLess(t, "position_y", div1.PositionY, div2.PositionY, div3.PositionY)
}

func TestFlexDirectionColumnMinWidth(t *testing.T) {
	article := only(t, flexBody(t, `
      <article style="display: flex; flex-direction: column; min-height: 100px">
        <div></div>
      </article>
    `))
	flexEq(t, "article.height", 100, article.Height)
}

func TestFlexDirectionColumnMaxHeight(t *testing.T) {
	article, div1, div2, div3 := flexThree(t, `
      <article style="display: flex; flex-flow: column wrap; max-height: 100px">
        <div style="height: 40px">A</div>
        <div style="height: 40px">B</div>
        <div style="height: 40px">C</div>
      </article>
    `)
	flexEq(t, "article.height", 100, article.Height)
	flexEq(t, "height", 40, div1.Height, div2.Height, div3.Height)
	flexEq(t, "div1.position_x", 0, div1.PositionX)
	flexEq(t, "div1.position_y", 0, div1.PositionY)
	flexEq(t, "div2.position_x", 0, div2.PositionX)
	flexEq(t, "div2.position_y", 40, div2.PositionY)
	flexEq(t, "div3.position_x", div1.Width, div3.PositionX)
	flexEq(t, "div3.position_y", 0, div3.PositionY)
}

func TestFlexDirectionColumnRTL(t *testing.T) {
	article, div1, div2, div3 := flexThree(t, `
      <article style="display: flex; flex-direction: column;
      direction: rtl">
        <div>A</div>
        <div>B</div>
        <div>C</div>
      </article>
    `)
	flexTexts(t, []*Box{div1, div2, div3}, "A", "B", "C")
	flexEq(t, "position_x", article.PositionX, div1.PositionX, div2.PositionX, div3.PositionX)
	flexEq(t, "div1.position_y", article.PositionY, div1.PositionY)
	flexLess(t, "position_y", div1.PositionY, div2.PositionY, div3.PositionY)
}

func TestFlexDirectionColumnReverse(t *testing.T) {
	article, div1, div2, div3 := flexThree(t, `
      <article style="display: flex; flex-direction: column-reverse">
        <div>A</div>
        <div>B</div>
        <div>C</div>
      </article>
    `)
	flexTexts(t, []*Box{div1, div2, div3}, "C", "B", "A")
	flexEq(t, "position_x", article.PositionX, div1.PositionX, div2.PositionX, div3.PositionX)
	flexEq(t, "div3 bottom", article.PositionY+article.Height, div3.PositionY+div3.Height)
	flexLess(t, "position_y", div1.PositionY, div2.PositionY, div3.PositionY)
}

func TestFlexDirectionColumnReverseRTL(t *testing.T) {
	article, div1, div2, div3 := flexThree(t, `
      <article style="display: flex; flex-direction: column-reverse;
      direction: rtl">
        <div>A</div>
        <div>B</div>
        <div>C</div>
      </article>
    `)
	flexTexts(t, []*Box{div1, div2, div3}, "C", "B", "A")
	flexEq(t, "position_x", article.PositionX, div1.PositionX, div2.PositionX, div3.PositionX)
	flexEq(t, "div3 bottom", article.PositionY+article.Height, div3.PositionY+div3.Height)
	flexLess(t, "position_y", div1.PositionY, div2.PositionY, div3.PositionY)
}

func TestFlexDirectionColumnBoxSizing(t *testing.T) {
	article := only(t, flexBody(t, `
      <style>
        article {
          box-sizing: border-box;
          display: flex;
          flex-direction: column;
          height: 10px;
          padding-top: 5px;
          width: 10px;
        }
      </style>
      <article></article>
    `))
	flexEq(t, "article.width", 10, article.Width)
	flexEq(t, "article.height", 5, article.Height)
	flexEq(t, "article.padding_top", 5, article.PaddingTop)
}

func TestFlexRowWrap(t *testing.T) {
	article, div1, div2, div3 := flexThree(t, `
      <article style="display: flex; flex-flow: wrap; width: 50px">
        <div style="width: 20px">A</div>
        <div style="width: 20px">B</div>
        <div style="width: 20px">C</div>
      </article>
    `)
	flexTexts(t, []*Box{div1, div2, div3}, "A", "B", "C")
	flexEq(t, "position_y 1", article.PositionY, div1.PositionY, div2.PositionY)
	flexEq(t, "div3.position_y", article.PositionY+div2.Height, div3.PositionY)
	flexEq(t, "position_x", article.PositionX, div1.PositionX, div3.PositionX)
	flexLess(t, "position_x", div1.PositionX, div2.PositionX)
}

func TestFlexColumnWrap(t *testing.T) {
	article, div1, div2, div3 := flexThree(t, `
      <article style="display: flex; flex-flow: column wrap; height: 50px">
        <div style="height: 20px">A</div>
        <div style="height: 20px">B</div>
        <div style="height: 20px">C</div>
      </article>
    `)
	flexTexts(t, []*Box{div1, div2, div3}, "A", "B", "C")
	flexEq(t, "position_x 1", article.PositionX, div1.PositionX, div2.PositionX)
	flexEq(t, "div3.position_x", article.PositionX+div2.Width, div3.PositionX)
	flexEq(t, "position_y", article.PositionY, div1.PositionY, div3.PositionY)
	flexLess(t, "position_y", div1.PositionY, div2.PositionY)
}

func TestFlexRowWrapReverse(t *testing.T) {
	article, div1, div2, div3 := flexThree(t, `
      <article style="display: flex; flex-flow: wrap-reverse; width: 50px">
        <div style="width: 20px">A</div>
        <div style="width: 20px">B</div>
        <div style="width: 20px">C</div>
      </article>
    `)
	flexTexts(t, []*Box{div1, div2, div3}, "C", "A", "B")
	flexEq(t, "div1.position_y", article.PositionY, div1.PositionY)
	flexEq(t, "position_y", article.PositionY+div1.Height, div2.PositionY, div3.PositionY)
	flexEq(t, "position_x", article.PositionX, div1.PositionX, div2.PositionX)
	flexLess(t, "position_x", div2.PositionX, div3.PositionX)
}

func TestFlexColumnWrapReverse(t *testing.T) {
	article, div1, div2, div3 := flexThree(t, `
      <article style="display: flex; flex-flow: column wrap-reverse;
                      height: 50px">
        <div style="height: 20px">A</div>
        <div style="height: 20px">B</div>
        <div style="height: 20px">C</div>
      </article>
    `)
	flexTexts(t, []*Box{div1, div2, div3}, "C", "A", "B")
	flexEq(t, "div1.position_x", article.PositionX, div1.PositionX)
	flexEq(t, "position_x", article.PositionX+div1.Width, div2.PositionX, div3.PositionX)
	flexEq(t, "position_y", article.PositionY, div1.PositionY, div2.PositionY)
	flexLess(t, "position_y", div2.PositionY, div3.PositionY)
}

func TestFlexDirectionColumnFixedHeightContainer(t *testing.T) {
	section := only(t, flexBody(t, `
      <section style="height: 10px">
        <article style="display: flex; flex-direction: column">
          <div>A</div>
          <div>B</div>
          <div>C</div>
        </article>
      </section>
    `))
	article := only(t, section)
	divs := unpack(t, article, 3)
	div1, div2, div3 := divs[0], divs[1], divs[2]
	flexTexts(t, divs, "A", "B", "C")
	flexEq(t, "position_x", article.PositionX, div1.PositionX, div2.PositionX, div3.PositionX)
	flexEq(t, "div1.position_y", article.PositionY, div1.PositionY)
	flexLess(t, "position_y", div1.PositionY, div2.PositionY, div3.PositionY)
	flexEq(t, "section.height", 10, section.Height)
	flexLess(t, "article.height", 10, article.Height)
}

func TestFlexDirectionColumnFixedHeight(t *testing.T) {
	article, div1, div2, div3 := flexThree(t, `
      <article style="display: flex; flex-direction: column; height: 10px">
        <div>A</div>
        <div>B</div>
        <div>C</div>
      </article>
    `)
	flexTexts(t, []*Box{div1, div2, div3}, "A", "B", "C")
	flexEq(t, "position_x", article.PositionX, div1.PositionX, div2.PositionX, div3.PositionX)
	flexEq(t, "div1.position_y", article.PositionY, div1.PositionY)
	flexLess(t, "position_y", div1.PositionY, div2.PositionY, div3.PositionY)
	flexEq(t, "article.height", 10, article.Height)
	flexLess(t, "div3.position_y", 10, div3.PositionY)
}

func TestFlexDirectionColumnFixedHeightWrap(t *testing.T) {
	article, div1, div2, div3 := flexThree(t, `
      <article style="display: flex; flex-direction: column; height: 10px;
                      flex-wrap: wrap">
        <div>A</div>
        <div>B</div>
        <div>C</div>
      </article>
    `)
	flexTexts(t, []*Box{div1, div2, div3}, "A", "B", "C")
	flexLess(t, "position_x", div1.PositionX, div2.PositionX, div3.PositionX)
	flexEq(t, "position_y", article.PositionY, div1.PositionY, div2.PositionY, div3.PositionY)
	flexEq(t, "article.height", 10, article.Height)
}

func TestFlexDirectionColumnBreak(t *testing.T) {
	// Regression test for issue #2066.
	pages := flexPages(t, `
      <style>
        @page { size: 4px 5px }
      </style>
      <article style="display: flex; flex-direction: column; font: 2px weasyprint">
        <div>A<br>B<br>C</div>
      </article>
    `, 2)
	_, body := pageBody(t, pages[0])
	div := only(t, only(t, body))
	if flexText(t, div, 0) != "A" || flexText(t, div, 1) != "B" {
		t.Errorf("page 1: texts %q %q", flexText(t, div, 0), flexText(t, div, 1))
	}
	flexEq(t, "div.height 1", 5, div.Height)
	_, body = pageBody(t, pages[1])
	div = only(t, only(t, body))
	if got := flexText(t, div, 0); got != "C" {
		t.Errorf("page 2: text %q", got)
	}
	flexEq(t, "div.height 2", 2, div.Height)
}

func TestFlexDirectionColumnBreakMargin(t *testing.T) {
	// Regression test for issue #1967.
	pages := flexPages(t, `
      <style>
        @page { size: 4px 7px }
      </style>
      <body style="font: 2px weasyprint">
        <p style="margin: 1px 0">a</p>
        <article style="display: flex; flex-direction: column">
          <div>A<br>B<br>C</div>
        </article>
      </body>
    `, 2)
	_, body := pageBody(t, pages[0])
	children := unpack(t, body, 2)
	p, article := children[0], children[1]
	flexEq(t, "p.margin_height", 4, p.MarginHeight())
	flexEq(t, "article.position_y", 4, article.PositionY)
	div := only(t, article)
	if got := flexText(t, div, 0); got != "A" {
		t.Errorf("page 1: text %q", got)
	}
	flexEq(t, "div.height 1", 3, div.Height)
	_, body = pageBody(t, pages[1])
	div = only(t, only(t, body))
	if flexText(t, div, 0) != "B" || flexText(t, div, 1) != "C" {
		t.Errorf("page 2: texts %q %q", flexText(t, div, 0), flexText(t, div, 1))
	}
	flexEq(t, "div.height 2", 4, div.Height)
}

func TestFlexDirectionColumnBreakBorder(t *testing.T) {
	pages := flexPages(t, `
      <style>
        @page { size: 8px 7px }
        article, div { border: 1px solid black }
      </style>
      <article style="display: flex; flex-direction: column; font: 2px weasyprint">
        <div>A B C</div>
      </article>
    `, 2)
	_, body := pageBody(t, pages[0])
	article := only(t, body)
	flexEq(t, "article.border_height 1", 7, article.BorderHeight())
	flexEq(t, "article.border_top_width 1", 1, article.BorderTopWidth)
	flexEq(t, "article.border_bottom_width 1", 0, article.BorderBottomWidth)
	div := only(t, article)
	if flexText(t, div, 0) != "A" || flexText(t, div, 1) != "B" {
		t.Errorf("page 1: texts %q %q", flexText(t, div, 0), flexText(t, div, 1))
	}
	flexEq(t, "div.border_height 1", 6, div.BorderHeight())
	flexEq(t, "div.border_top_width 1", 1, div.BorderTopWidth)
	flexEq(t, "div.border_bottom_width 1", 0, div.BorderBottomWidth)
	_, body = pageBody(t, pages[1])
	article = only(t, body)
	flexEq(t, "article.border_height 2", 4, article.BorderHeight())
	flexEq(t, "article.border_top_width 2", 0, article.BorderTopWidth)
	flexEq(t, "article.border_bottom_width 2", 1, article.BorderBottomWidth)
	div = only(t, article)
	if got := flexText(t, div, 0); got != "C" {
		t.Errorf("page 2: text %q", got)
	}
	flexEq(t, "div.border_height 2", 3, div.BorderHeight())
	flexEq(t, "div.border_top_width 2", 0, div.BorderTopWidth)
	flexEq(t, "div.border_bottom_width 2", 1, div.BorderBottomWidth)
}

func TestFlexDirectionColumnBreakMultipleChildren(t *testing.T) {
	pages := flexPages(t, `
      <style>
        @page { size: 4px 5px }
      </style>
      <article style="display: flex; flex-direction: column; font: 2px weasyprint">
        <div>A</div>
        <div>B<br>C</div>
      </article>
    `, 2)
	_, body := pageBody(t, pages[0])
	divs := unpack(t, only(t, body), 2)
	div1, div2 := divs[0], divs[1]
	flexTexts(t, divs, "A", "B")
	flexEq(t, "div1.height", 2, div1.Height)
	flexEq(t, "div2.height 1", 3, div2.Height)
	_, body = pageBody(t, pages[1])
	div2 = only(t, only(t, body))
	if got := flexText(t, div2, 0); got != "C" {
		t.Errorf("page 2: text %q", got)
	}
	flexEq(t, "div2.height 2", 2, div2.Height)
}

func TestFlexItemMinWidth(t *testing.T) {
	article, div1, div2, div3 := flexThree(t, `
      <article style="display: flex">
        <div style="min-width: 30px">A</div>
        <div style="min-width: 50px">B</div>
        <div style="min-width: 5px">C</div>
      </article>
    `)
	flexTexts(t, []*Box{div1, div2, div3}, "A", "B", "C")
	flexEq(t, "div1.position_x", 0, div1.PositionX)
	flexEq(t, "div1.width", 30, div1.Width)
	flexEq(t, "div2.position_x", 30, div2.PositionX)
	flexEq(t, "div2.width", 50, div2.Width)
	flexEq(t, "div3.position_x", 80, div3.PositionX)
	flexLess(t, "div3.width", 5, div3.Width)
	flexEq(t, "position_y", article.PositionY, div1.PositionY, div2.PositionY, div3.PositionY)
}

func TestFlexItemMinHeight(t *testing.T) {
	article, div1, div2, div3 := flexThree(t, `
      <article style="display: flex">
        <div style="min-height: 30px">A</div>
        <div style="min-height: 50px">B</div>
        <div style="min-height: 5px">C</div>
      </article>
    `)
	flexTexts(t, []*Box{div1, div2, div3}, "A", "B", "C")
	flexEq(t, "height", 50, div1.Height, div2.Height, div3.Height, article.Height)
}

func TestFlexAutoMargin(t *testing.T) {
	// Regression test for issue #800.
	flexPages(t, `<div style="display: flex; margin: auto"><div>`, 1)
	flexPages(t, `<div style="display: flex; flex-direction: column; margin: auto"><div>`, 1)
}

func TestFlexItemAutoMarginSized(t *testing.T) {
	// Regression test for issue #2054.
	article := only(t, flexBody(t, `
      <style>
        div {
          margin: auto;
          display: flex;
          width: 160px;
          height: 160px;
        }
      </style>
      <article>
        <div></div>
      </article>
    `))
	div := only(t, article)
	if div.MarginLeft == 0 {
		t.Errorf("div.margin_left = 0")
	}
}

func TestFlexNoBaseline(t *testing.T) {
	// Regression test for issue #765.
	flexPages(t, `
      <div class="references" style="display: flex; align-items: baseline;">
        <div></div>
      </div>`, 1)
}

func TestFlexAlignContent(t *testing.T) {
	for _, c := range []struct {
		align          string
		height, y1, y2 float64
	}{
		{"flex-start", 50, 0, 10},
		{"flex-end", 50, 30, 40},
		{"space-around", 60, 10, 40},
		{"space-between", 50, 0, 40},
		{"space-evenly", 50, 10, 30},
	} {
		t.Run(c.align, func(t *testing.T) {
			// Regression test for issue #811.
			article := only(t, flexBody(t, fmt.Sprintf(`
      <style>
        article {
          align-content: %s;
          display: flex;
          flex-wrap: wrap;
          font-family: weasyprint;
          font-size: 10px;
          height: %gpx;
          line-height: 1;
        }
        section {
          width: 100%%;
        }
      </style>
      <article>
        <section><span>Lorem</span></section>
        <section><span>Lorem</span></section>
      </article>
    `, c.align, c.height)))
			sections := unpack(t, article, 2)
			section1, section2 := sections[0], sections[1]
			span1 := only(t, only(t, section1))
			span2 := only(t, only(t, section2))
			flexEq(t, "position_x 1", 0, section1.PositionX, span1.PositionX)
			flexEq(t, "position_y 1", c.y1, section1.PositionY, span1.PositionY)
			flexEq(t, "position_x 2", 0, section2.PositionX, span2.PositionX)
			flexEq(t, "position_y 2", c.y2, section2.PositionY, span2.PositionY)
		})
	}
}

func TestFlexItemPercentage(t *testing.T) {
	// Regression test for issue #885.
	flex := only(t, flexBody(t, `
      <div style="display: flex; font-size: 15px; line-height: 1">
        <div style="height: 100%">a</div>
      </div>`))
	flexEq(t, "flex_item.height", 15, only(t, flex).Height)
}

func TestFlexUndefinedPercentageHeightMultipleLines(t *testing.T) {
	// Regression test for issue #1204.
	flexPages(t, `
      <div style="display: flex; flex-wrap: wrap; height: 100%">
        <div style="width: 100%">a</div>
        <div style="width: 100%">b</div>
      </div>`, 1)
}

func TestFlexAbsolute(t *testing.T) {
	// Regression test for issue #1536.
	flexPages(t, `
      <div style="display: flex; position: absolute">
        <div>a</div>
      </div>`, 1)
}

func TestFlexPercentHeight(t *testing.T) {
	a := only(t, flexBody(t, `
      <style>
        .a { height: 10px; width: 10px; }
        .b { height: 10%; width: 100%; display: flex; flex-direction: column; }
      </style>
      <div class="a"">
        <div class="b"></div>
      </div>`))
	b := only(t, a)
	flexEq(t, "a.height", 10, a.Height)
	flexEq(t, "b.height", 1, b.Height)
}

func TestFlexPercentHeightAuto(t *testing.T) {
	// Regression test for issue #2146.
	flexPages(t, `
      <style>
        .a { width: 10px; }
        .b { height: 10%; width: 100%; display: flex; flex-direction: column; }
      </style>
      <div class="a"">
        <div class="b"></div>
      </div>`, 1)
}

func TestFlexBreakInsideAvoid(t *testing.T) {
	// Regression test for issue #2183.
	pages := flexPages(t, `
      <style>
        @page { size: 6px 4px }
        html { font-family: weasyprint; font-size: 2px }
      </style>
      <article style="display: flex; flex-wrap: wrap">
        <div>ABC</div>
        <div style="break-inside: avoid">abc def</div>
      </article>`, 2)
	for _, page := range pages {
		_, body := pageBody(t, page)
		only(t, only(t, body))
	}
}

func TestFlexAbsoluteContent(t *testing.T) {
	// Regression test for issue #996.
	section := only(t, flexBody(t, `
      <section style="display: flex; position: relative">
         <h1 style="position: absolute; top: 0; right: 0">TEST</h1>
         <p>Hello world!</p>
      </section>`))
	children := unpack(t, section, 2)
	h1, p := children[0], children[1]
	if h1.PositionX == 0 {
		t.Errorf("h1.position_x = 0")
	}
	flexEq(t, "h1.position_y", 0, h1.PositionY)
	flexEq(t, "p.position_x", 0, p.PositionX)
	flexEq(t, "p.position_y", 0, p.PositionY)
}

func TestFlexColumnHeight(t *testing.T) {
	// Regression test for issue #2222.
	section := only(t, flexBody(t, `
      <section style="display: flex; width: 10em">
        <article style="display: flex; flex-direction: column">
          <div>
            Lorem ipsum dolor sit amet
          </div>
        </article>
        <article style="display: flex; flex-direction: column">
          <div>
            Lorem ipsum dolor sit amet
          </div>
        </article>
      </section>
    `))
	articles := unpack(t, section, 2)
	flexEq(t, "article.height", section.Height, articles[0].Height, articles[1].Height)
}

func TestFlexColumnHeightMargin(t *testing.T) {
	// Regression test for issue #2222.
	section := only(t, flexBody(t, `
      <section style="display: flex; flex-direction: column; width: 10em">
        <article style="margin: 5px">
          Lorem ipsum dolor sit amet
        </article>
        <article style="margin: 10px">
          Lorem ipsum dolor sit amet
        </article>
      </section>
    `))
	articles := unpack(t, section, 2)
	flexEq(t, "section.height", articles[0].MarginHeight()+articles[1].MarginHeight(), section.Height)
}

func TestFlexColumnWidth(t *testing.T) {
	// Regression test for issue #1171.
	main := only(t, flexBody(t, `
      <main style="display: flex; flex-direction: column;
                   width: 40px; height: 50px; font: 2px weasyprint">
        <section style="width: 100%; height: 5px">a</section>
        <section style="display: flex; flex: auto; flex-direction: column;
                        justify-content: space-between; width: 100%">
          <div>b</div>
          <div>c</div>
        </section>
      </main>
    `))
	sections := unpack(t, main, 2)
	section1, section2 := sections[0], sections[1]
	divs := unpack(t, section2, 2)
	div1, div2 := divs[0], divs[1]
	flexEq(t, "section.width", main.Width, section1.Width, section2.Width)
	flexEq(t, "div2.width", div1.Width, div2.Width)
	flexEq(t, "div1.position_y", 5, div1.PositionY)
	flexEq(t, "div2.position_y", 48, div2.PositionY)
}

func TestFlexColumnInFlexRow(t *testing.T) {
	_, body := pageBody(t, flexPages(t, `
      <body style="display: flex; flex-wrap: wrap; font: 2px weasyprint">
        <article>1</article>
        <section style="display: flex; flex-direction: column">
          <div>2</div>
        </section>
      </body>
    `, 1)[0])
	children := unpack(t, body, 2)
	article, section := children[0], children[1]
	flexEq(t, "position_y", 0, article.PositionY, section.PositionY)
	flexEq(t, "article.position_x", 0, article.PositionX)
	flexEq(t, "section.position_x", 2, section.PositionX)
}

func TestFlexOverflow(t *testing.T) {
	// Regression test for issue #2292.
	article := only(t, flexBody(t, `
      <style>
        article {
          display: flex;
        }
        section {
          overflow: hidden;
          width: 5px;
        }
      </style>
      <article>
        <section>A</section>
        <section>B</section>
      </article>
    `))
	if len(article.Children) < 2 {
		t.Fatalf("article has %d children", len(article.Children))
	}
	flexEq(t, "section_1.position_x", 0, article.Children[0].PositionX)
	flexEq(t, "section_2.position_x", 5, article.Children[1].PositionX)
}

func TestFlexColumnOverflow(t *testing.T) {
	// Regression test for issue #2304.
	log, c := captureLogs(slog.LevelInfo)
	renderPagesLog(t, `
      <style>
        @page {size: 20px}
      </style>
      <div style="display: flex; flex-direction: column">
        <div></div>
        <div><div style="height: 40px"></div></div>
        <div><div style="height: 5px"></div></div>
      </div>
    `, testBaseURL, log)
	assertNoLogs(t, c)
}

func TestInlineFlex(t *testing.T) {
	flexPages(t, `
      <style>
        @page {size: 20px}
      </style>
      <div style="display: inline-flex; flex-direction: column">
        <div>test</div>
      </div>
    `, 1)
}

func TestInlineFlexEmptyChild(t *testing.T) {
	flexPages(t, `
      <style>
        @page {size: 20px}
      </style>
      <div style="display: inline-flex; flex-direction: column">
        <div></div>
      </div>
    `, 1)
}

func TestInlineFlexAbsoluteBaseline(t *testing.T) {
	flexPages(t, `
      <style>
        @page {size: 20px}
      </style>
      <div style="display: inline-flex; flex-direction: column">
        <div style="position: absolute">abs</div>
      </div>
    `, 1)
}

func TestFlexItemOverflow(t *testing.T) {
	// Regression test for issue #2359.
	flex := only(t, flexBody(t, `
      <div style="display: flex; font: 2px weasyprint; width: 12px">
        <div>ab</div>
        <div>c d e</div>
        <div>f</div>
      </div>`))
	divs := unpack(t, flex, 3)
	div1, div2, div3 := divs[0], divs[1], divs[2]
	flexEq(t, "div1.width", 4, div1.Width)
	flexEq(t, "div2.width", 6, div2.Width)
	flexEq(t, "div3.width", 2, div3.Width)
	lines := unpack(t, div2, 2)
	if got := only(t, lines[0]).Text; got != "c d" {
		t.Errorf("text1 = %q", got)
	}
	if got := only(t, lines[1]).Text; got != "e" {
		t.Errorf("text2 = %q", got)
	}
}

func TestFlexItemChildBottomMargin(t *testing.T) {
	for _, direction := range []string{"row", "column"} {
		t.Run(direction, func(t *testing.T) {
			// Regression test for issue #2449.
			flex := only(t, flexBody(t, fmt.Sprintf(`
      <div style="display: flex; font: 2px weasyprint; flex-direction: %s">
        <section>
          <div style="margin: 2px 0">ab</div>
        </section>
      </div>`, direction)))
			flexEq(t, "flex.content_box_y", 0, flex.ContentBoxY())
			flexEq(t, "flex.height", 6, flex.Height)
			section := only(t, flex)
			flexEq(t, "section.content_box_y", 0, section.ContentBoxY())
			flexEq(t, "section.height", 6, section.Height)
			div := only(t, section)
			flexEq(t, "div.content_box_y", 2, div.ContentBoxY())
			flexEq(t, "div.height", 2, div.Height)
		})
	}
}

func TestFlexDirectionRowInlineBlock(t *testing.T) {
	// Regression test for issue #1652.
	article := only(t, flexBody(t, `
      <article style="display: flex; font: 2px weasyprint; width: 14px">
        <div style="display: inline-block">A B C D E F</div>
      </article>
    `))
	div := only(t, article)
	flexEq(t, "div.width", 14, div.Width)
	if flexText(t, div, 0) != "A B C D" || flexText(t, div, 1) != "E F" {
		t.Errorf("texts %q %q", flexText(t, div, 0), flexText(t, div, 1))
	}
}

func TestFlexFloat(t *testing.T) {
	article := only(t, flexBody(t, `
      <article style="display: flex">
        <div style="float: left">A</div>
      </article>
    `))
	flexTexts(t, []*Box{only(t, article)}, "A")
}

func TestFlexFloatInFlexItem(t *testing.T) {
	// Regression test for issue #1356.
	article := only(t, flexBody(t, `
      <article style="display: flex; font: 2px weasyprint">
        <div style="width: 10px"><span style="float: right">abc</span></div>
        <div style="width: 10px"><span style="float: right">abc</span></div>
      </article>
    `))
	divs := unpack(t, article, 2)
	span1 := only(t, divs[0])
	flexEq(t, "span1.position_y", 0, span1.PositionY)
	flexEq(t, "span1 right", 10, span1.PositionX+span1.Width)
	span2 := only(t, divs[1])
	flexEq(t, "span2.position_y", 0, span2.PositionY)
	flexEq(t, "span2 right", 20, span2.PositionX+span2.Width)
}

func TestFlexDirectionRowDefinedMain(t *testing.T) {
	div := only(t, only(t, flexBody(t, `
      <article style="display: flex">
        <div style="width: 10px; padding: 1px"></div>
      </article>
    `)))
	flexEq(t, "div.width", 10, div.Width)
	flexEq(t, "div.margin_width", 12, div.MarginWidth())
}

func TestFlexDirectionRowDefinedMainBorderBox(t *testing.T) {
	div := only(t, only(t, flexBody(t, `
      <article style="display: flex">
        <div style="box-sizing: border-box; width: 10px; padding: 1px"></div>
      </article>
    `)))
	flexEq(t, "div.width", 8, div.Width)
	flexEq(t, "div.margin_width", 10, div.MarginWidth())
}

func TestFlexDirectionColumnDefinedMain(t *testing.T) {
	div := only(t, only(t, flexBody(t, `
      <article style="display: flex; flex-direction: column">
        <div style="height: 10px; padding: 1px"></div>
      </article>
    `)))
	flexEq(t, "div.height", 10, div.Height)
	flexEq(t, "div.margin_height", 12, div.MarginHeight())
}

func TestFlexDirectionColumnDefinedMainBorderBox(t *testing.T) {
	div := only(t, only(t, flexBody(t, `
      <article style="display: flex; flex-direction: column">
        <div style="box-sizing: border-box; height: 10px; padding: 1px"></div>
      </article>
    `)))
	flexEq(t, "div.height", 8, div.Height)
	flexEq(t, "div.margin_height", 10, div.MarginHeight())
}

func TestFlexItemNegativeMargin(t *testing.T) {
	div := only(t, only(t, flexBody(t, `
      <article style="display: flex">
        <div style="margin-left: -20px; height: 10px; width: 10px"></div>
      </article>
    `)))
	flexEq(t, "div.height", 10, div.Height)
	flexEq(t, "div.width", 10, div.Width)
}

func TestFlexItemAutoMarginMain(t *testing.T) {
	div := only(t, only(t, flexBody(t, `
      <article style="display: flex; width: 100px">
        <div style="margin-left: auto; height: 10px; width: 10px"></div>
      </article>
    `)))
	flexEq(t, "div.height", 10, div.Height)
	flexEq(t, "div.width", 10, div.Width)
	flexEq(t, "div.margin_left", 90, div.MarginLeft)
}

func TestFlexItemAutoMarginCross(t *testing.T) {
	div := only(t, only(t, flexBody(t, `
      <article style="display: flex; height: 100px">
        <div style="margin-top: auto; height: 10px; width: 10px"></div>
      </article>
    `)))
	flexEq(t, "div.height", 10, div.Height)
	flexEq(t, "div.width", 10, div.Width)
	flexEq(t, "div.margin_top", 90, div.MarginTop)
}

func TestFlexDirectionColumnItemAutoMargin(t *testing.T) {
	article := only(t, only(t, flexBody(t, `
      <div style="font: 2px weasyprint; width: 30px; display: flex;
                  flex-direction: column; align-items: flex-start">
          <article style="margin: 0 auto">XXXX</article>
      </div>
    `)))
	flexEq(t, "article.width", 8, article.Width)
	flexEq(t, "article.margin_left", 11, article.MarginLeft)
}

func TestFlexItemAutoMarginFlexBasis(t *testing.T) {
	div := only(t, only(t, flexBody(t, `
      <article style="display: flex">
        <div style="margin-left: auto; height: 10px; flex-basis: 10px"></div>
      </article>
    `)))
	flexEq(t, "div.height", 10, div.Height)
	flexEq(t, "div.width", 10, div.Width)
}

// flexJustifyCase is a case of the tests of justify-content: the value
// and the positions of the three items.
type flexJustifyCase struct {
	align      string
	p1, p2, p3 float64
}

// flexJustifyCases are the cases of test_flex_direction_row_justify and
// test_flex_direction_column_justify.
var flexJustifyCases = []flexJustifyCase{
	{"start", 0, 2, 4},
	{"flex-start", 0, 2, 4},
	{"left", 0, 2, 4},
	{"end", 6, 8, 10},
	{"flex-end", 6, 8, 10},
	{"right", 6, 8, 10},
	{"center", 3, 5, 7},
	{"space-between", 0, 5, 10},
	{"space-around", 1, 5, 9},
	{"space-evenly", 1.5, 5, 8.5},
}

// flexJustifyGapCases are the cases of the tests of justify-content with a
// gap.
var flexJustifyGapCases = []flexJustifyCase{
	{"start", 0, 4, 8},
	{"flex-start", 0, 4, 8},
	{"left", 0, 4, 8},
	{"end", 6, 10, 14},
	{"flex-end", 6, 10, 14},
	{"right", 6, 10, 14},
	{"center", 3, 7, 11},
	{"space-between", 0, 7, 14},
	{"space-around", 1, 7, 13},
	{"space-evenly", 1.5, 7, 12.5},
}

// flexJustifyGapWrapCases are the cases of the tests of justify-content
// with a gap and a wrap.
var flexJustifyGapWrapCases = []flexJustifyCase{
	{"start", 0, 4, 0},
	{"flex-start", 0, 4, 0},
	{"left", 0, 4, 0},
	{"end", 3, 7, 7},
	{"flex-end", 3, 7, 7},
	{"right", 3, 7, 7},
	{"center", 1.5, 5.5, 3.5},
	{"space-between", 0, 7, 0},
	{"space-around", 0.75, 6.25, 3.5},
	{"space-evenly", 1, 6, 3.5},
}

func TestFlexDirectionRowJustify(t *testing.T) {
	for _, c := range flexJustifyCases {
		t.Run(c.align, func(t *testing.T) {
			article, div1, div2, div3 := flexThree(t, fmt.Sprintf(`
      <article style="width: 12px; font: 2px weasyprint;
                      display: flex; justify-content: %s">
        <div>A</div>
        <div>B</div>
        <div>C</div>
      </article>
    `, c.align))
			flexTexts(t, []*Box{div1, div2, div3}, "A", "B", "C")
			flexEq(t, "position_y", article.PositionY, div1.PositionY, div2.PositionY, div3.PositionY)
			flexEq(t, "article.position_x", 0, article.PositionX)
			flexEq(t, "div1.position_x", c.p1, div1.PositionX)
			flexEq(t, "div2.position_x", c.p2, div2.PositionX)
			flexEq(t, "div3.position_x", c.p3, div3.PositionX)
		})
	}
}

func TestFlexDirectionColumnJustify(t *testing.T) {
	for _, c := range flexJustifyCases {
		t.Run(c.align, func(t *testing.T) {
			article, div1, div2, div3 := flexThree(t, fmt.Sprintf(`
      <article style="height: 12px; font: 2px weasyprint;
                      display: flex; flex-direction: column; justify-content: %s">
        <div>A</div>
        <div>B</div>
        <div>C</div>
      </article>
    `, c.align))
			flexTexts(t, []*Box{div1, div2, div3}, "A", "B", "C")
			flexEq(t, "position_x", article.PositionX, div1.PositionX, div2.PositionX, div3.PositionX)
			flexEq(t, "article.position_y", 0, article.PositionY)
			flexEq(t, "div1.position_y", c.p1, div1.PositionY)
			flexEq(t, "div2.position_y", c.p2, div2.PositionY)
			flexEq(t, "div3.position_y", c.p3, div3.PositionY)
		})
	}
}

func TestFlexDirectionRowJustifyGap(t *testing.T) {
	for _, c := range flexJustifyGapCases {
		t.Run(c.align, func(t *testing.T) {
			article, div1, div2, div3 := flexThree(t, fmt.Sprintf(`
      <article style="width: 16px; font: 2px weasyprint; gap: 2px;
                      display: flex; justify-content: %s">
        <div>A</div>
        <div>B</div>
        <div>C</div>
      </article>
    `, c.align))
			flexTexts(t, []*Box{div1, div2, div3}, "A", "B", "C")
			flexEq(t, "position_y", article.PositionY, div1.PositionY, div2.PositionY, div3.PositionY)
			flexEq(t, "article.position_x", 0, article.PositionX)
			flexEq(t, "div1.position_x", c.p1, div1.PositionX)
			flexEq(t, "div2.position_x", c.p2, div2.PositionX)
			flexEq(t, "div3.position_x", c.p3, div3.PositionX)
		})
	}
}

func TestFlexDirectionColumnJustifyGap(t *testing.T) {
	for _, c := range flexJustifyGapCases {
		t.Run(c.align, func(t *testing.T) {
			article, div1, div2, div3 := flexThree(t, fmt.Sprintf(`
      <article style="height: 16px; font: 2px weasyprint; gap: 2px;
                      display: flex; flex-direction: column; justify-content: %s">
        <div>A</div>
        <div>B</div>
        <div>C</div>
      </article>
    `, c.align))
			flexTexts(t, []*Box{div1, div2, div3}, "A", "B", "C")
			flexEq(t, "position_x", article.PositionX, div1.PositionX, div2.PositionX, div3.PositionX)
			flexEq(t, "article.position_y", 0, article.PositionY)
			flexEq(t, "div1.position_y", c.p1, div1.PositionY)
			flexEq(t, "div2.position_y", c.p2, div2.PositionY)
			flexEq(t, "div3.position_y", c.p3, div3.PositionY)
		})
	}
}

func TestFlexDirectionRowJustifyGapWrap(t *testing.T) {
	for _, c := range flexJustifyGapWrapCases {
		t.Run(c.align, func(t *testing.T) {
			article, div1, div2, div3 := flexThree(t, fmt.Sprintf(`
      <article style="width: 9px; font: 2px weasyprint; gap: 1px 2px;
                      display: flex; flex-wrap: wrap; justify-content: %s">
        <div>A</div>
        <div>B</div>
        <div>C</div>
      </article>
    `, c.align))
			flexTexts(t, []*Box{div1, div2, div3}, "A", "B", "C")
			flexEq(t, "position_y", 0, div1.PositionY, div2.PositionY, article.PositionY)
			flexEq(t, "div3.position_y", 3, div3.PositionY)
			flexEq(t, "article.position_x", 0, article.PositionX)
			flexEq(t, "div1.position_x", c.p1, div1.PositionX)
			flexEq(t, "div2.position_x", c.p2, div2.PositionX)
			flexEq(t, "div3.position_x", c.p3, div3.PositionX)
		})
	}
}

func TestFlexDirectionColumnJustifyGapWrap(t *testing.T) {
	for _, c := range flexJustifyGapWrapCases {
		t.Run(c.align, func(t *testing.T) {
			article, div1, div2, div3 := flexThree(t, fmt.Sprintf(`
      <article style="height: 9px; width: 9px; font: 2px weasyprint; gap: 2px 1px;
                      display: flex; flex-wrap: wrap; flex-direction: column;
                      justify-content: %s">
        <div>A</div>
        <div>B</div>
        <div>C</div>
      </article>
    `, c.align))
			flexTexts(t, []*Box{div1, div2, div3}, "A", "B", "C")
			flexEq(t, "position_x", 0, div1.PositionX, div2.PositionX, article.PositionX)
			flexEq(t, "div3.position_x", 5, div3.PositionX)
			flexEq(t, "article.position_y", 0, article.PositionY)
			flexEq(t, "div1.position_y", c.p1, div1.PositionY)
			flexEq(t, "div2.position_y", c.p2, div2.PositionY)
			flexEq(t, "div3.position_y", c.p3, div3.PositionY)
		})
	}
}

func TestFlexDirectionRowStretchNoGrow(t *testing.T) {
	article, div1, div2, div3 := flexThree(t, `
      <article style="font: 2px weasyprint; width: 10px;
                      display: flex; justify-content: stretch">
        <div>A</div>
        <div>B</div>
        <div>C</div>
      </article>
    `)
	flexTexts(t, []*Box{div1, div2, div3}, "A", "B", "C")
	flexEq(t, "position_y", article.PositionY, div1.PositionY, div2.PositionY, div3.PositionY)
	flexEq(t, "width", 2, div1.Width, div2.Width, div3.Width)
	flexEq(t, "position_x 1", 0, div1.PositionX, article.PositionX)
	flexEq(t, "div2.position_x", 2, div2.PositionX)
	flexEq(t, "div3.position_x", 4, div3.PositionX)
}

func TestFlexDirectionRowStretchGrow(t *testing.T) {
	article, div1, div2, div3 := flexThree(t, `
      <article style="font: 2px weasyprint; width: 10px;
                      display: flex; justify-content: stretch">
        <div>A</div>
        <div style="flex-grow: 3">B</div>
        <div style="flex-grow: 1">C</div>
      </article>
    `)
	flexTexts(t, []*Box{div1, div2, div3}, "A", "B", "C")
	flexEq(t, "position_y", article.PositionY, div1.PositionY, div2.PositionY, div3.PositionY)
	flexEq(t, "div1.width", 2, div1.Width)
	flexEq(t, "div2.width", 5, div2.Width)
	flexEq(t, "div3.width", 3, div3.Width)
	flexEq(t, "position_x 1", 0, div1.PositionX, article.PositionX)
	flexEq(t, "div2.position_x", 2, div2.PositionX)
	flexEq(t, "div3.position_x", 7, div3.PositionX)
}

// flexJustifyMarginPaddingCases are the cases of the tests of
// justify-content with margins and paddings.
var flexJustifyMarginPaddingCases = []flexJustifyCase{
	{"start", 0, 6, 12},
	{"flex-start", 0, 6, 12},
	{"left", 0, 6, 12},
	{"end", 6, 12, 18},
	{"flex-end", 6, 12, 18},
	{"right", 6, 12, 18},
	{"center", 3, 9, 15},
	{"space-between", 0, 9, 18},
	{"space-around", 1, 9, 17},
	{"space-evenly", 1.5, 9, 16.5},
}

func TestFlexDirectionRowJustifyMarginPadding(t *testing.T) {
	for _, c := range flexJustifyMarginPaddingCases {
		t.Run(c.align, func(t *testing.T) {
			article, div1, div2, div3 := flexThree(t, fmt.Sprintf(`
      <article style="width: 20px; font: 2px weasyprint;
                      display: flex; justify-content: %s">
        <div style="margin: 0 1em">A</div>
        <div style="padding: 0 1em">B</div>
        <div>C</div>
      </article>
    `, c.align))
			flexTexts(t, []*Box{div1, div2, div3}, "A", "B", "C")
			flexEq(t, "position_y", article.PositionY, div1.PositionY, div2.PositionY, div3.PositionY)
			flexEq(t, "article.position_x", 0, article.PositionX)
			flexEq(t, "article.width", 20, article.Width)
			flexEq(t, "div1.position_x", c.p1, div1.PositionX)
			flexEq(t, "div2.position_x", c.p2, div2.PositionX)
			flexEq(t, "div3.position_x", c.p3, div3.PositionX)
			flexEq(t, "margin_width", 6, div1.MarginWidth(), div2.MarginWidth())
			flexEq(t, "div3.margin_width", 2, div3.MarginWidth())
		})
	}
}

func TestFlexDirectionColumnJustifyMarginPadding(t *testing.T) {
	for _, c := range flexJustifyMarginPaddingCases {
		t.Run(c.align, func(t *testing.T) {
			article, div1, div2, div3 := flexThree(t, fmt.Sprintf(`
      <article style="height: 20px; font: 2px weasyprint;
                      display: flex; flex-direction: column; justify-content: %s">
        <div style="margin: 1em 0">A</div>
        <div style="padding: 1em 0">B</div>
        <div>C</div>
      </article>
    `, c.align))
			flexTexts(t, []*Box{div1, div2, div3}, "A", "B", "C")
			flexEq(t, "position_x", article.PositionX, div1.PositionX, div2.PositionX, div3.PositionX)
			flexEq(t, "article.position_y", 0, article.PositionY)
			flexEq(t, "article.height", 20, article.Height)
			flexEq(t, "div1.position_y", c.p1, div1.PositionY)
			flexEq(t, "div2.position_y", c.p2, div2.PositionY)
			flexEq(t, "div3.position_y", c.p3, div3.PositionY)
			flexEq(t, "margin_height", 6, div1.MarginHeight(), div2.MarginHeight())
			flexEq(t, "div3.margin_height", 2, div3.MarginHeight())
		})
	}
}

func TestFlexItemTable(t *testing.T) {
	// Regression test for issue #1805.
	article := only(t, flexBody(t, `
      <article style="display: flex; font: 2px weasyprint">
        <table><tr><td>A</tr></td></table>
        <table><tr><td>B</tr></td></table>
      </article>
    `))
	wrappers := unpack(t, article, 2)
	flexEq(t, "width", 2, wrappers[0].Width, wrappers[1].Width)
	flexEq(t, "table_wrapper1.position_x", 0, wrappers[0].PositionX)
	flexEq(t, "table_wrapper2.position_x", 2, wrappers[1].PositionX)
}

func TestFlexItemTableWidth(t *testing.T) {
	// Regression test for issue #1805.
	article := only(t, flexBody(t, `
      <article style="display: flex; font: 2px weasyprint; width: 40px">
        <table style="width: 25%"><tr><td>A</tr></td></table>
        <table style="width: 25%"><tr><td>B</tr></td></table>
      </article>
    `))
	wrappers := unpack(t, article, 2)
	flexEq(t, "width", 10, wrappers[0].Width, wrappers[1].Width)
	flexEq(t, "table_wrapper1.position_x", 0, wrappers[0].PositionX)
	flexEq(t, "table_wrapper2.position_x", 10, wrappers[1].PositionX)
}

func TestFlexWidthOnParent(t *testing.T) {
	article := only(t, only(t, flexBody(t, `
      <div style="font: 2px weasyprint; width: 30px; display: flex;
                  flex-direction: column; align-items: flex-start">
          <article>XXXX</article>
      </div>
    `)))
	flexEq(t, "article.width", 8, article.Width)
}

func TestFlexColumnItemFlex1(t *testing.T) {
	div := only(t, flexBody(t, `
      <div style="font: 2px weasyprint; display: flex; flex-direction: column">
          <article>XXXX</article>
          <article style="flex: 1">XXXX</article>
      </div>
    `))
	flexEq(t, "div.height", 4, div.Height)
}

func TestFlexRowItemFlex0(t *testing.T) {
	div := only(t, flexBody(t, `
      <div style="font: 2px weasyprint; display: flex">
          <article style="flex: 0">XXXX</article>
          <article style="flex: 0">XXXX</article>
      </div>
    `))
	articles := unpack(t, div, 2)
	flexEq(t, "article1.position_x", 0, articles[0].PositionX)
	flexEq(t, "article1.width", 8, articles[0].Width)
	flexEq(t, "article2.position_x", 8, articles[1].PositionX)
	flexEq(t, "article2.width", 8, articles[1].Width)
}

func TestFlexItemIntrinsicWidth(t *testing.T) {
	svg := only(t, only(t, flexBody(t, `
      <div style="width: 100px; height: 100px;
                  display: flex; flex-direction: column; align-items: center">
        <svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 200 100">
          <rect width="200" height="100" fill="red" />
        </svg>
      </div>
    `)))
	flexEq(t, "svg.width", 100, svg.Width)
}

func TestFlexAlignContentNegative(t *testing.T) {
	div := only(t, flexBody(t, `
      <div style="height: 6px; width: 20px;
                  display: flex; flex-wrap: wrap; align-content: center">
        <span style="height: 2px; flex: none; margin: 1px; width: 8px"></span>
        <span style="height: 2px; flex: none; margin: 1px; width: 8px"></span>
        <span style="height: 2px; flex: none; margin: 1px; width: 8px"></span>
        <span style="height: 2px; flex: none; margin: 1px; width: 8px"></span>
      </div>
    `))
	spans := unpack(t, div, 4)
	// Upstream compares the heights and the widths of the spans without
	// assert, so the test does not check them.
	flexEq(t, "position_x 1", 0, spans[0].PositionX, spans[2].PositionX)
	flexEq(t, "position_x 2", 10, spans[1].PositionX, spans[3].PositionX)
	flexEq(t, "position_y 1", -1, spans[0].PositionY, spans[1].PositionY)
	flexEq(t, "position_y 2", 3, spans[2].PositionY, spans[3].PositionY)
}

func TestFlexShrink(t *testing.T) {
	article := only(t, flexBody(t, `
      <article style="display: flex; width: 300px">
        <div style="flex: 0 2 auto; width: 300px"></div>
        <div style="width: 200px"></div>
      </article>
    `))
	divs := unpack(t, article, 2)
	flexEq(t, "width", 150, divs[0].Width, divs[1].Width)
}

func TestFlexItemIntrinsicWidthShrink(t *testing.T) {
	svg := only(t, only(t, flexBody(t, `
      <div style="width: 10px; height: 100px; display: flex; flex-direction: column">
        <svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 100 100" width="100">
          <rect width="100" height="100" fill="red" />
        </svg>
      </div>
    `)))
	flexEq(t, "svg.width", 100, svg.Width)
}

func TestFlexItemIntrinsicHeightShrink(t *testing.T) {
	svg := only(t, only(t, flexBody(t, `
      <div style="width: 100px; height: 10px; display: flex; line-height: 0">
        <svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 100 100" height="100">
          <rect width="100" height="100" fill="red" />
        </svg>
      </div>
    `)))
	flexEq(t, "svg.height", 100, svg.Height)
}

func TestFlexWrapInFlex(t *testing.T) {
	div := only(t, only(t, flexBody(t, `
      <main style="display: flex; font: 2px weasyprint">
        <div style="display: flex; flex-wrap: wrap">
          <section style="width: 25%">A</section>
          <section style="flex: 1 75%">B</section>
        </div>
      </main>
    `)))
	sections := unpack(t, div, 2)
	flexEq(t, "position_y", 0, sections[0].PositionY, sections[1].PositionY)
	flexEq(t, "section1.position_x", 0, sections[0].PositionX)
	flexEq(t, "section2.position_x", 1, sections[1].PositionX) // 25% * 4
}

func TestFlexAutoBreakBefore(t *testing.T) {
	pages := flexPages(t, `
      <style>
        @page { size: 4px 5px }
        body { font: 2px weasyprint }
      </style>
      <p>A<br>B</p>
      <article style="display: flex">
        <div>A</div>
      </article>
    `, 2)
	_, body := pageBody(t, pages[0])
	flexEq(t, "p.height", 4, only(t, body).Height)
	_, body = pageBody(t, pages[1])
	flexEq(t, "article.height", 2, only(t, body).Height)
}

func TestFlexGrowInFlexColumn(t *testing.T) {
	_, body := pageBody(t, flexPages(t, `
      <html style="width: 14px">
        <body style="display: flex; flex-direction: column;
                     border: 1px solid; padding: 1px">
          <main style="flex: 1 1 auto; min-height: 0">
            <div style="height: 5px">
    `, 1)[0])
	div := only(t, only(t, body))
	flexEq(t, "height", 5, body.Height, div.Height)
	flexEq(t, "width", 10, body.Width, div.Width)
	flexEq(t, "body.margin_width", 14, body.MarginWidth())
	flexEq(t, "body.margin_height", 9, body.MarginHeight())
}

func TestFlexCollapsingMargin(t *testing.T) {
	body := flexBody(t, `
      <p style="margin-bottom: 20px; height: 100px">ABC</p>
      <article style="display: flex; margin-top: 10px">
        <div>A</div>
      </article>
    `)
	children := unpack(t, body, 2)
	p, article := children[0], children[1]
	div := only(t, article)
	flexEq(t, "p.position_y", 0, p.PositionY)
	flexEq(t, "p.height", 100, p.Height)
	flexEq(t, "article.position_y", 110, article.PositionY)
	flexEq(t, "div.position_y", 120, div.PositionY)
}

// flexRenderNoLogs renders a document of any number of pages, as
// render_pages does with assert_no_logs.
func flexRenderNoLogs(t *testing.T, source string) []*Box {
	t.Helper()
	log, c := captureLogs(slog.LevelInfo)
	pages := renderPagesLog(t, source, testBaseURL, log)
	assertNoLogs(t, c)
	return pages
}

// flexLineText returns box.children[i].children[0]: the first box of the
// line of index i of box, a text box.
func flexLineText(t *testing.T, box *Box, i int) *Box {
	t.Helper()
	if i >= len(box.Children) || len(box.Children[i].Children) == 0 {
		t.Fatalf("%s has no line %d with a child", box, i)
	}
	return box.Children[i].Children[0]
}

// flexChild returns the child of index i of box.
func flexChild(t *testing.T, box *Box, i int) *Box {
	t.Helper()
	if i >= len(box.Children) {
		t.Fatalf("%s has no child %d", box, i)
	}
	return box.Children[i]
}

// flexCheckText checks the text and the position_y of a text box.
func flexCheckText(t *testing.T, box *Box, text string, positionY float64) {
	t.Helper()
	if box.Text != text {
		t.Errorf("text = %q, want %q", box.Text, text)
	}
	flexEq(t, text+".position_y", positionY, box.PositionY)
}

func TestFlexDirectionColumnNextPage(t *testing.T) {
	// Regression test for issue #2414.
	pages := flexPages(t, `
      <style>
        @page { size: 4px 5px }
        html { font: 2px/1 weasyprint }
      </style>
      <div>1</div>
      <article style="display: flex; flex-direction: column">
        <div>A</div>
        <div>B</div>
        <div>C</div>
      </article>
    `, 2)
	_, body := pageBody(t, pages[0])
	children := unpack(t, body, 2)
	div, article := children[0], children[1]
	flexCheckText(t, flexLineText(t, div, 0), "1", 0)
	flexCheckText(t, flexLineText(t, flexChild(t, article, 0), 0), "A", 2)
	_, body = pageBody(t, pages[1])
	article = only(t, body)
	flexCheckText(t, flexLineText(t, flexChild(t, article, 0), 0), "B", 0)
	flexCheckText(t, flexLineText(t, flexChild(t, article, 1), 0), "C", 2)
}

func TestFlex1ItemPadding(t *testing.T) {
	article := only(t, flexBody(t, `
      <article style="display: flex; width: 100px; font: 2px weasyprint">
        <div>abc</div>
        <div style="flex: 1; padding-right: 5em">def</div>
      </article>
    `))
	divs := unpack(t, article, 2)
	flexEq(t, "border widths", article.Width, divs[0].BorderWidth()+divs[1].BorderWidth())
}

func TestFlex1ItemPaddingDirectionColumn(t *testing.T) {
	article := only(t, flexBody(t, `
      <article style="display: flex; flex-direction: column; height: 100px;
                      font: 2px weasyprint">
        <div>abc</div>
        <div style="flex: 1; padding-top: 5em">def</div>
      </article>
    `))
	divs := unpack(t, article, 2)
	flexEq(t, "border heights", article.Height, divs[0].BorderHeight()+divs[1].BorderHeight())
}

func TestFlexItemReplaced(t *testing.T) {
	svg := only(t, only(t, flexBody(t, `
      <div style="display: flex">
        <svg style="display: block" height="100" width="100" xmlns="http://www.w3.org/2000/svg">
          <circle r="45" cx="50" cy="50" fill="red" />
        </svg>
      </div>
    `)))
	flexEq(t, "svg size", 100, svg.Width, svg.Height)
}

func TestFlexNestedColumn(t *testing.T) {
	// Regression test for issue #2442.
	p := only(t, only(t, only(t, flexBody(t, `
      <section style="display: flex; flex-direction: column; width: 200px">
        <div style="display: flex; flex-direction: column">
          <p>
            A
          </p>
        </div>
      </section>
    `))))
	flexEq(t, "p.width", 200, p.Width)
}

func TestFlexBlockifyImage(t *testing.T) {
	article := only(t, flexBody(t, `
      <article style="display: flex; line-height: 2">
        <img src="pattern.png">
      </article>
    `))
	img := only(t, article)
	flexEq(t, "height", 4, article.Height, img.Height)
}

func TestFlexImageMaxWidth(t *testing.T) {
	article := only(t, flexBody(t, `
      <article style="display: flex">
        <img src="pattern.png" style="max-width: 2px">
      </article>
    `))
	img := only(t, article)
	flexEq(t, "size", 2, article.Height, img.Height, img.Width)
}

func TestFlexImageMaxHeight(t *testing.T) {
	article := only(t, flexBody(t, `
      <article style="display: flex">
        <img src="pattern.png" style="max-height: 2px">
      </article>
    `))
	img := only(t, article)
	flexEq(t, "size", 2, article.Height, img.Height, img.Width)
}

func TestFlexImageMinWidth(t *testing.T) {
	article := only(t, flexBody(t, `
      <article style="display: flex; width: 20px">
        <img style="min-width: 10px; flex: 1 0 auto" src="pattern.png">
        <div style="flex: 1 0 1px"></div>
      </article>
    `))
	children := unpack(t, article, 2)
	img, div := children[0], children[1]
	flexEq(t, "size", 10, article.Height, img.Height, img.Width, div.Height)
}

func TestFlexImageJustifyContent(t *testing.T) {
	article := only(t, flexBody(t, `
      <article style="width: 100px; display: flex; justify-content: end">
        <img src="pattern.png">
      </article>
    `))
	img := only(t, article)
	flexEq(t, "size", 4, article.Height, img.Height, img.Width)
	flexEq(t, "article.width", 100, article.Width)
	flexEq(t, "img.position_x", 96, img.PositionX)
}

func TestFlexRootFormattingContext(t *testing.T) {
	body := flexBody(t, `
      <html style="display: flex">
        <div>A</div>
      </html>
    `)
	div := only(t, body)
	flexTexts(t, []*Box{div}, "A")
	flexEq(t, "div.position_y", body.PositionY, div.PositionY)
	flexEq(t, "div.position_x", body.PositionX, div.PositionX)
}

func TestFlexHeightPageOverflow(t *testing.T) {
	// Regression test for #2689.
	div := only(t, only(t, flexBody(t, `
      <style>
        @page { size: 4px 6px }
      </style>
      <section style="display: flex; font: 2px weasyprint; height: 2px">
        <div style="width: 100%; display: flex; align-content: flex-start">a b c d</div>
      </section>
    `)))
	if got := len(flexChild(t, div, 0).Children); got != 3 {
		t.Errorf("len(div.children[0].children) = %d, want 3", got)
	}
}

func TestFlexBreakAfterPage(t *testing.T) {
	// Regression test for #2469.
	pages := flexPages(t, `
      <style>
        @page { size: 5px }
        body {
          display: flex; flex-direction: column;
          font: 2px/1 weasyprint; margin-top: 1px;
        }
      </style>
      <div style="height: 10px">a</div>
      <section>
        <div style="break-after: page">b</div>
        <div>c</div>
      </section>
    `, 3)
	_, body := pageBody(t, pages[0])
	div := only(t, body)
	flexEq(t, "div.position_y 1", 1, div.PositionY)
	flexEq(t, "div.height 1", 10, div.Height)
	flexTexts(t, []*Box{div}, "a")
	_, body = pageBody(t, pages[1])
	flexEq(t, "body.position_y 2", 0, body.PositionY)
	flexEq(t, "body.height 2", 5, body.Height)
	div = only(t, only(t, body))
	flexEq(t, "div.position_y 2", 0, div.PositionY)
	flexEq(t, "div.height 2", 2, div.Height)
	flexTexts(t, []*Box{div}, "b")
	_, body = pageBody(t, pages[2])
	flexEq(t, "body.position_y 3", 0, body.PositionY)
	flexEq(t, "body.height 3", 2, body.Height)
	div = only(t, only(t, body))
	flexEq(t, "div.position_y 3", 0, div.PositionY)
	flexEq(t, "div.height 3", 2, div.Height)
	flexTexts(t, []*Box{div}, "c")
}

func TestFlexBreakFloat(t *testing.T) {
	// Regression test for #2815.
	flexRenderNoLogs(t, `
      <style>
        @page { size: 3px }
        body { font: 2px/1 weasyprint }
      </style>
      <div></div>
      <div style="display: flex">
        <div style="overflow: hidden">
          <div style="float: left">
            <p>b
            <p>c
    `)
}

func TestFlexInlineTable(t *testing.T) {
	// Regression test for #2855.
	flexRenderNoLogs(t, `
      <style>
        @page { size: 10px }
        body { font: 2px/1 weasyprint; display: flex }
        div { display: inline-table }
      </style>
      <div><span>a</span></div>
    `)
}

func TestFlexInlineFlex(t *testing.T) {
	// Regression test for #2855.
	div := only(t, only(t, flexBody(t, `
      <style>
        @page { size: 10px }
        body { font: 2px/1 weasyprint }
        article { display: flex }
        div { display: inline-flex; padding: 1px; border: 1px solid }
      </style>
      <article><div><span>a</span></div></article>
    `)))
	flexEq(t, "div.position_x", 0, div.PositionX)
	flexEq(t, "div.position_y", 0, div.PositionY)
	flexEq(t, "div.width", 2, div.Width)
	span := only(t, div)
	flexEq(t, "span.position_x", 2, span.PositionX)
	flexEq(t, "span.position_y", 2, span.PositionY)
}

func TestFlexInlineGrid(t *testing.T) {
	// Regression test for #2855. Upstream does not check the logs here.
	pages := renderPages(t, `
      <style>
        @page { size: 10px }
        body { font: 2px/1 weasyprint }
        article { display: flex }
        div { display: inline-grid; padding: 1px; border: 1px solid }
      </style>
      <article><div><span>a</span></div></article>
    `)
	if len(pages) != 1 {
		t.Fatalf("got %d pages", len(pages))
	}
	_, body := pageBody(t, pages[0])
	div := only(t, only(t, body))
	flexEq(t, "div.position_x", 0, div.PositionX)
	flexEq(t, "div.position_y", 0, div.PositionY)
	flexEq(t, "div.width", 2, div.Width)
	span := only(t, div)
	flexEq(t, "span.position_x", 2, span.PositionX)
	flexEq(t, "span.position_y", 2, span.PositionY)
}

func TestFlexFloatGap(t *testing.T) {
	article := only(t, flexBody(t, `
      <article style="display: flex; float: left; font: 2px/1 weasyprint; gap: 1px">
        <div>a b</div>
        <div>c d</div>
      </article>
    `))
	flexEq(t, "article.width", 13, article.Width)
	divs := unpack(t, article, 2)
	flexEq(t, "width", 6, divs[0].Width, divs[1].Width)
	flexEq(t, "div1.position_x", 0, divs[0].PositionX)
	flexEq(t, "div2.position_x", 7, divs[1].PositionX)
}
