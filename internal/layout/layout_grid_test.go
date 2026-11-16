// Copyright (c) 2011-2021, Simon Sapin and contributors.
//
// Use of this source code is governed by the BSD-3-Clause license that can be
// found in the LICENSE file.

// This file ports tests/layout/testGrid.py: the tests of the layout of the
// grid containers. The tests read the positions and the sizes of the laid
// out boxes, as upstream does, and the unit tests of the helpers of
// layoutGrid.go follow them.

package layout

import (
	"errors"
	"fmt"
	"log/slog"
	"math"
	"testing"

	"github.com/xo/ivy"
)

// gridRenderPages is renderPages with assertNoLogs: it lays out a
// document, fails the test for each message of the log, and fails the test
// when the document does not have n pages.
func gridRenderPages(t *testing.T, source string, n int) []*Box {
	t.Helper()
	log, logs := captureLogs(slog.LevelWarn)
	pages := renderPagesLog(t, source, testBaseURL, log)
	assertNoLogs(t, logs)
	if len(pages) != n {
		t.Fatalf("the document has %d pages, want %d", len(pages), n)
	}
	return pages
}

// gridEqual fails the test when got is not want. The tolerance is the
// default of math.isclose of Python: 1e-9 times the larger value.
func gridEqual(t *testing.T, desc string, got, want float64) {
	t.Helper()
	if got != want && math.Abs(got-want) > 1e-9*max(math.Abs(got), math.Abs(want)) {
		t.Errorf("%s: %v != %v", desc, got, want)
	}
}

// gridLess fails the test when a is not less than b.
func gridLess(t *testing.T, desc string, a, b float64) {
	t.Helper()
	if !(a < b) {
		t.Errorf("%s: %v >= %v", desc, a, b)
	}
}

// gridEqualString fails the test when got is not want.
func gridEqualString(t *testing.T, desc, got, want string) {
	t.Helper()
	if got != want {
		t.Errorf("%s: %q != %q", desc, got, want)
	}
}

// gridAll fails the test when the boxes are none, or when value of one of
// them is not want: {value(box) for box in boxes} == {want}.
func gridAll(t *testing.T, desc string, boxes []*Box, value func(*Box) float64, want float64) {
	t.Helper()
	if len(boxes) == 0 {
		t.Errorf("%s: no box", desc)
	}
	for i, box := range boxes {
		gridEqual(t, fmt.Sprintf("%s, box %d", desc, i), value(box), want)
	}
}

// gridLast returns the last child of box: `*_, last = box.children`.
func gridLast(t *testing.T, box *Box) *Box {
	t.Helper()
	if len(box.Children) == 0 {
		t.Fatalf("%s has no child", box)
	}
	return box.Children[len(box.Children)-1]
}

// TestLayoutGridEmpty is testGridEmpty of tests/layout/testGrid.py.
func TestLayoutGridEmpty(t *testing.T) {
	pages := gridRenderPages(t, `
      <article style="display: grid">
      </article>
    `, 1)
	page := pages[0]
	html := only(t, page)
	body := only(t, html)
	article := only(t, body)
	gridEqual(t, `article.position_x == 0`, article.PositionX, 0)
	gridEqual(t, `article.position_y == 0`, article.PositionY, 0)
	gridEqual(t, `article.width == html.width`, article.Width, html.Width)
	gridEqual(t, `article.height == 0`, article.Height, 0)
}

// TestLayoutGridSingleItem is testGridSingleItem of tests/layout/testGrid.py.
func TestLayoutGridSingleItem(t *testing.T) {
	pages := gridRenderPages(t, `
      <article style="display: grid">
        <div>a</div>
      </article>
    `, 1)
	page := pages[0]
	html := only(t, page)
	body := only(t, html)
	article := only(t, body)
	div := only(t, article)
	gridEqual(t, `article.position_x == div.position_x`, article.PositionX, div.PositionX)
	gridEqual(t, `div.position_x == 0`, div.PositionX, 0)
	gridEqual(t, `article.position_y == div.position_y`, article.PositionY, div.PositionY)
	gridEqual(t, `div.position_y == 0`, div.PositionY, 0)
	gridEqual(t, `article.width == div.width`, article.Width, div.Width)
	gridEqual(t, `div.width == html.width`, div.Width, html.Width)
}

// TestLayoutGridSingleAutoWidth is testGridSingleAutoWidth of tests/layout/testGrid.py.
func TestLayoutGridSingleAutoWidth(t *testing.T) {
	pages := gridRenderPages(t, `
      <article style="display: grid">
        <div style="padding: 0 2px; justify-self: start"></div>
      </article>
    `, 1)
	page := pages[0]
	html := only(t, page)
	body := only(t, html)
	article := only(t, body)
	div := only(t, article)
	gridEqual(t, `article.position_x == div.position_x`, article.PositionX, div.PositionX)
	gridEqual(t, `div.position_x == 0`, div.PositionX, 0)
	gridEqual(t, `article.position_y == div.position_y`, article.PositionY, div.PositionY)
	gridEqual(t, `div.position_y == 0`, div.PositionY, 0)
	gridEqual(t, `article.width == html.width`, article.Width, html.Width)
	gridEqual(t, `div.width == 0`, div.Width, 0)
	gridEqual(t, `div.padding_width() == 4`, div.PaddingWidth(), 4)
}

// TestLayoutGridSinglePercentageWidth is testGridSinglePercentageWidth of tests/layout/testGrid.py.
func TestLayoutGridSinglePercentageWidth(t *testing.T) {
	pages := gridRenderPages(t, `
      <article style="display: grid">
        <div style="justify-self: start; width: 100%"></div>
      </article>
    `, 1)
	page := pages[0]
	html := only(t, page)
	body := only(t, html)
	article := only(t, body)
	div := only(t, article)
	gridEqual(t, `article.position_x == div.position_x`, article.PositionX, div.PositionX)
	gridEqual(t, `div.position_x == 0`, div.PositionX, 0)
	gridEqual(t, `article.position_y == div.position_y`, article.PositionY, div.PositionY)
	gridEqual(t, `div.position_y == 0`, div.PositionY, 0)
	gridEqual(t, `article.width == html.width`, article.Width, html.Width)
	gridEqual(t, `html.width == div.padding_width()`, html.Width, div.PaddingWidth())
}

// TestLayoutGridRows is testGridRows of tests/layout/testGrid.py.
func TestLayoutGridRows(t *testing.T) {
	pages := gridRenderPages(t, `
      <article style="display: grid">
        <div>a</div>
        <div>b</div>
        <div>c</div>
      </article>
    `, 1)
	page := pages[0]
	html := only(t, page)
	body := only(t, html)
	article := only(t, body)
	children2 := unpack(t, article, 3)
	divA := children2[0]
	divB := children2[1]
	divC := children2[2]
	gridEqual(t, `div_a.position_x == div_b.position_x`, divA.PositionX, divB.PositionX)
	gridEqual(t, `div_b.position_x == div_c.position_x`, divB.PositionX, divC.PositionX)
	gridEqual(t, `div_c.position_x == 0`, divC.PositionX, 0)
	gridLess(t, `div_a.position_y < div_b.position_y`, divA.PositionY, divB.PositionY)
	gridLess(t, `div_b.position_y < div_c.position_y`, divB.PositionY, divC.PositionY)
	gridEqual(t, `div_a.height == div_b.height`, divA.Height, divB.Height)
	gridEqual(t, `div_b.height == div_c.height`, divB.Height, divC.Height)
	gridEqual(t, `div_a.width == div_b.width`, divA.Width, divB.Width)
	gridEqual(t, `div_b.width == div_c.width`, divB.Width, divC.Width)
	gridEqual(t, `div_c.width == html.width`, divC.Width, html.Width)
	gridEqual(t, `html.width == article.width`, html.Width, article.Width)
}

// TestLayoutGridTemplateFr is testGridTemplateFr of tests/layout/testGrid.py.
func TestLayoutGridTemplateFr(t *testing.T) {
	pages := gridRenderPages(t, `
      <style>
        article {
          display: grid;
          font-family: weasyprint;
          font-size: 2px;
          grid-template-rows: auto 1fr;
          grid-template-columns: auto 1fr;
          line-height: 1;
          width: 10px;
        }
      </style>
      <article>
        <div>a</div> <div>b</div>
        <div>c</div> <div>d</div>
      </article>
    `, 1)
	page := pages[0]
	html := only(t, page)
	body := only(t, html)
	article := only(t, body)
	children2 := unpack(t, article, 4)
	divA := children2[0]
	divB := children2[1]
	divC := children2[2]
	divD := children2[3]
	gridEqual(t, `div_a.position_x == div_c.position_x`, divA.PositionX, divC.PositionX)
	gridEqual(t, `div_c.position_x == 0`, divC.PositionX, 0)
	gridEqual(t, `div_b.position_x == div_d.position_x`, divB.PositionX, divD.PositionX)
	gridEqual(t, `div_d.position_x == 2`, divD.PositionX, 2)
	gridEqual(t, `div_a.height == div_b.height`, divA.Height, divB.Height)
	gridEqual(t, `div_b.height == div_c.height`, divB.Height, divC.Height)
	gridEqual(t, `div_c.height == div_d.height`, divC.Height, divD.Height)
	gridEqual(t, `div_d.height == 2`, divD.Height, 2)
	gridEqual(t, `div_a.width == div_c.width`, divA.Width, divC.Width)
	gridEqual(t, `div_c.width == 2`, divC.Width, 2)
	gridEqual(t, `div_b.width == div_d.width`, divB.Width, divD.Width)
	gridEqual(t, `div_d.width == 8`, divD.Width, 8)
	gridEqual(t, `article.width == 10`, article.Width, 10)
}

// TestLayoutGridTemplateAreas is testGridTemplateAreas of tests/layout/testGrid.py.
func TestLayoutGridTemplateAreas(t *testing.T) {
	pages := gridRenderPages(t, `
      <style>
        article {
          display: grid;
          font-family: weasyprint;
          font-size: 2px;
          grid-template-areas: 'a b' 'c d';
          line-height: 1;
          width: 10px;
        }
      </style>
      <article>
        <div>a</div> <div>b</div>
        <div>c</div> <div>d</div>
      </article>
    `, 1)
	page := pages[0]
	html := only(t, page)
	body := only(t, html)
	article := only(t, body)
	children2 := unpack(t, article, 4)
	divA := children2[0]
	divB := children2[1]
	divC := children2[2]
	divD := children2[3]
	gridEqual(t, `div_a.position_x == div_c.position_x`, divA.PositionX, divC.PositionX)
	gridEqual(t, `div_c.position_x == 0`, divC.PositionX, 0)
	gridEqual(t, `div_b.position_x == div_d.position_x`, divB.PositionX, divD.PositionX)
	gridEqual(t, `div_d.position_x == 5`, divD.PositionX, 5)
	gridEqual(t, `div_a.position_y == div_b.position_y`, divA.PositionY, divB.PositionY)
	gridEqual(t, `div_b.position_y == 0`, divB.PositionY, 0)
	gridEqual(t, `div_c.position_y == div_d.position_y`, divC.PositionY, divD.PositionY)
	gridEqual(t, `div_d.position_y == 2`, divD.PositionY, 2)
	gridEqual(t, `div_a.height == div_b.height`, divA.Height, divB.Height)
	gridEqual(t, `div_b.height == div_c.height`, divB.Height, divC.Height)
	gridEqual(t, `div_c.height == div_d.height`, divC.Height, divD.Height)
	gridEqual(t, `div_d.height == 2`, divD.Height, 2)
	gridEqual(t, `div_a.width == div_b.width`, divA.Width, divB.Width)
	gridEqual(t, `div_b.width == div_c.width`, divB.Width, divC.Width)
	gridEqual(t, `div_c.width == div_d.width`, divC.Width, divD.Width)
	gridEqual(t, `div_d.width == 5`, divD.Width, 5)
	gridEqual(t, `article.width == 10`, article.Width, 10)
}

// TestLayoutGridTemplateAreasGridArea is testGridTemplateAreasGridArea of tests/layout/testGrid.py.
func TestLayoutGridTemplateAreasGridArea(t *testing.T) {
	pages := gridRenderPages(t, `
      <style>
        article {
          display: grid;
          font-family: weasyprint;
          font-size: 2px;
          grid-template-areas: 'b a' 'd c';
          line-height: 1;
          width: 10px;
        }
      </style>
      <article>
        <div style="grid-area: a">a</div> <div style="grid-area: b">b</div>
        <div style="grid-area: c">c</div> <div style="grid-area: d">d</div>
      </article>
    `, 1)
	page := pages[0]
	html := only(t, page)
	body := only(t, html)
	article := only(t, body)
	children2 := unpack(t, article, 4)
	divA := children2[0]
	divB := children2[1]
	divC := children2[2]
	divD := children2[3]
	gridEqual(t, `div_b.position_x == div_d.position_x`, divB.PositionX, divD.PositionX)
	gridEqual(t, `div_d.position_x == 0`, divD.PositionX, 0)
	gridEqual(t, `div_a.position_x == div_c.position_x`, divA.PositionX, divC.PositionX)
	gridEqual(t, `div_c.position_x == 5`, divC.PositionX, 5)
	gridEqual(t, `div_a.position_y == div_b.position_y`, divA.PositionY, divB.PositionY)
	gridEqual(t, `div_b.position_y == 0`, divB.PositionY, 0)
	gridEqual(t, `div_c.position_y == div_d.position_y`, divC.PositionY, divD.PositionY)
	gridEqual(t, `div_d.position_y == 2`, divD.PositionY, 2)
	gridEqual(t, `div_a.height == div_b.height`, divA.Height, divB.Height)
	gridEqual(t, `div_b.height == div_c.height`, divB.Height, divC.Height)
	gridEqual(t, `div_c.height == div_d.height`, divC.Height, divD.Height)
	gridEqual(t, `div_d.height == 2`, divD.Height, 2)
	gridEqual(t, `div_a.width == div_b.width`, divA.Width, divB.Width)
	gridEqual(t, `div_b.width == div_c.width`, divB.Width, divC.Width)
	gridEqual(t, `div_c.width == div_d.width`, divC.Width, divD.Width)
	gridEqual(t, `div_d.width == 5`, divD.Width, 5)
	gridEqual(t, `article.width == 10`, article.Width, 10)
}

// TestLayoutGridTemplateAreasEmptyRow is testGridTemplateAreasEmptyRow of tests/layout/testGrid.py.
func TestLayoutGridTemplateAreasEmptyRow(t *testing.T) {
	pages := gridRenderPages(t, `
      <style>
        article {
          display: grid;
          font-family: weasyprint;
          font-size: 2px;
          grid-template-areas: 'b a' 'd a' 'd c';
          line-height: 1;
          width: 10px;
        }
      </style>
      <article>
        <div style="grid-area: a">a</div> <div style="grid-area: b">b</div>
        <div style="grid-area: c">c</div> <div style="grid-area: d">d</div>
      </article>
    `, 1)
	page := pages[0]
	html := only(t, page)
	body := only(t, html)
	article := only(t, body)
	children2 := unpack(t, article, 4)
	divA := children2[0]
	divB := children2[1]
	divC := children2[2]
	divD := children2[3]
	gridEqual(t, `div_b.position_x == div_d.position_x`, divB.PositionX, divD.PositionX)
	gridEqual(t, `div_d.position_x == 0`, divD.PositionX, 0)
	gridEqual(t, `div_a.position_x == div_c.position_x`, divA.PositionX, divC.PositionX)
	gridEqual(t, `div_c.position_x == 5`, divC.PositionX, 5)
	gridEqual(t, `div_a.position_y == div_b.position_y`, divA.PositionY, divB.PositionY)
	gridEqual(t, `div_b.position_y == 0`, divB.PositionY, 0)
	gridEqual(t, `div_c.position_y == div_d.position_y`, divC.PositionY, divD.PositionY)
	gridEqual(t, `div_d.position_y == 2`, divD.PositionY, 2)
	gridEqual(t, `div_a.height == div_b.height`, divA.Height, divB.Height)
	gridEqual(t, `div_b.height == div_c.height`, divB.Height, divC.Height)
	gridEqual(t, `div_c.height == div_d.height`, divC.Height, divD.Height)
	gridEqual(t, `div_d.height == 2`, divD.Height, 2)
	gridEqual(t, `div_a.width == div_b.width`, divA.Width, divB.Width)
	gridEqual(t, `div_b.width == div_c.width`, divB.Width, divC.Width)
	gridEqual(t, `div_c.width == div_d.width`, divC.Width, divD.Width)
	gridEqual(t, `div_d.width == 5`, divD.Width, 5)
	gridEqual(t, `article.width == 10`, article.Width, 10)
}

// TestLayoutGridTemplateAreasMultipleRows is testGridTemplateAreasMultipleRows of tests/layout/testGrid.py.
func TestLayoutGridTemplateAreasMultipleRows(t *testing.T) {
	pages := gridRenderPages(t, `
      <style>
        article {
          display: grid;
          font-family: weasyprint;
          font-size: 2px;
          grid-template-areas: 'b a' 'd a' '. c';
          line-height: 1;
          width: 10px;
        }
      </style>
      <article>
        <div style="grid-area: a">a</div> <div style="grid-area: b">b</div>
        <div style="grid-area: c">c</div> <div style="grid-area: d">d</div>
      </article>
    `, 1)
	page := pages[0]
	html := only(t, page)
	body := only(t, html)
	article := only(t, body)
	children2 := unpack(t, article, 4)
	divA := children2[0]
	divB := children2[1]
	divC := children2[2]
	divD := children2[3]
	gridEqual(t, `div_b.position_x == div_d.position_x`, divB.PositionX, divD.PositionX)
	gridEqual(t, `div_d.position_x == 0`, divD.PositionX, 0)
	gridEqual(t, `div_a.position_x == div_c.position_x`, divA.PositionX, divC.PositionX)
	gridEqual(t, `div_c.position_x == 5`, divC.PositionX, 5)
	gridEqual(t, `div_a.position_y == div_b.position_y`, divA.PositionY, divB.PositionY)
	gridEqual(t, `div_b.position_y == 0`, divB.PositionY, 0)
	gridEqual(t, `div_c.position_y == 4`, divC.PositionY, 4)
	gridEqual(t, `div_d.position_y == 2`, divD.PositionY, 2)
	gridEqual(t, `div_a.height == 4`, divA.Height, 4)
	gridEqual(t, `div_b.height == div_c.height`, divB.Height, divC.Height)
	gridEqual(t, `div_c.height == div_d.height`, divC.Height, divD.Height)
	gridEqual(t, `div_d.height == 2`, divD.Height, 2)
	gridEqual(t, `div_a.width == div_b.width`, divA.Width, divB.Width)
	gridEqual(t, `div_b.width == div_c.width`, divB.Width, divC.Width)
	gridEqual(t, `div_c.width == div_d.width`, divC.Width, divD.Width)
	gridEqual(t, `div_d.width == 5`, divD.Width, 5)
	gridEqual(t, `article.width == 10`, article.Width, 10)
}

// TestLayoutGridTemplateAreasMultipleColumns is testGridTemplateAreasMultipleColumns of tests/layout/testGrid.py.
func TestLayoutGridTemplateAreasMultipleColumns(t *testing.T) {
	pages := gridRenderPages(t, `
      <style>
        article {
          display: grid;
          font-family: weasyprint;
          font-size: 2px;
          grid-template-areas: 'b b' 'c a';
          line-height: 1;
          width: 10px;
        }
      </style>
      <article>
        <div style="grid-area: a">a</div>
        <div style="grid-area: b">b</div>
        <div style="grid-area: c">c</div>
      </article>
    `, 1)
	page := pages[0]
	html := only(t, page)
	body := only(t, html)
	article := only(t, body)
	children2 := unpack(t, article, 3)
	divA := children2[0]
	divB := children2[1]
	divC := children2[2]
	gridEqual(t, `div_b.position_x == div_c.position_x`, divB.PositionX, divC.PositionX)
	gridEqual(t, `div_c.position_x == 0`, divC.PositionX, 0)
	gridEqual(t, `div_a.position_x == 5`, divA.PositionX, 5)
	gridEqual(t, `div_a.position_y == div_c.position_y`, divA.PositionY, divC.PositionY)
	gridEqual(t, `div_c.position_y == 2`, divC.PositionY, 2)
	gridEqual(t, `div_b.position_y == 0`, divB.PositionY, 0)
	gridEqual(t, `div_a.height == div_b.height`, divA.Height, divB.Height)
	gridEqual(t, `div_b.height == div_c.height`, divB.Height, divC.Height)
	gridEqual(t, `div_c.height == 2`, divC.Height, 2)
	gridEqual(t, `div_a.width == div_c.width`, divA.Width, divC.Width)
	gridEqual(t, `div_c.width == 5`, divC.Width, 5)
	gridEqual(t, `div_b.width == 10`, divB.Width, 10)
	gridEqual(t, `article.width == 10`, article.Width, 10)
}

// TestLayoutGridTemplateAreasOverlap is testGridTemplateAreasOverlap of tests/layout/testGrid.py.
func TestLayoutGridTemplateAreasOverlap(t *testing.T) {
	pages := gridRenderPages(t, `
      <style>
        article {
          display: grid;
          font-family: weasyprint;
          font-size: 2px;
          grid-template-areas: 'a b' 'c d';
          line-height: 1;
          width: 10px;
        }
      </style>
      <article>
        <div style="grid-area: a">a</div>
        <div style="grid-area: a">a</div>
        <div style="grid-area: a">a</div>
      </article>
    `, 1)
	page := pages[0]
	html := only(t, page)
	body := only(t, html)
	article := only(t, body)
	children2 := unpack(t, article, 3)
	divA1 := children2[0]
	divA2 := children2[1]
	divA3 := children2[2]
	gridEqual(t, `div_a1.position_x == div_a2.position_x`, divA1.PositionX, divA2.PositionX)
	gridEqual(t, `div_a2.position_x == div_a3.position_x`, divA2.PositionX, divA3.PositionX)
	gridEqual(t, `div_a3.position_x == 0`, divA3.PositionX, 0)
	gridEqual(t, `div_a1.position_y == div_a2.position_y`, divA1.PositionY, divA2.PositionY)
	gridEqual(t, `div_a2.position_y == div_a3.position_y`, divA2.PositionY, divA3.PositionY)
	gridEqual(t, `div_a3.position_y == 0`, divA3.PositionY, 0)
	gridEqual(t, `div_a1.width == div_a2.width`, divA1.Width, divA2.Width)
	gridEqual(t, `div_a2.width == div_a3.width`, divA2.Width, divA3.Width)
	gridEqual(t, `div_a3.width == 6`, divA3.Width, 6)
	gridEqual(t, `div_a1.height == div_a2.height`, divA1.Height, divA2.Height)
	gridEqual(t, `div_a2.height == div_a3.height`, divA2.Height, divA3.Height)
	gridEqual(t, `div_a3.height == 2`, divA3.Height, 2)
	gridEqual(t, `article.width == 10`, article.Width, 10)
}

// TestLayoutGridTemplateBigSpan is testGridTemplateBigSpan of tests/layout/testGrid.py.
func TestLayoutGridTemplateBigSpan(t *testing.T) {
	pages := gridRenderPages(t, `
      <style>
        article {
          display: grid;
          font-family: weasyprint;
          font-size: 2px;
          grid-template-columns: 50% 50%;
          line-height: 1;
          width: 6px;
        }
      </style>
      <article>
        <div>a</div>
        <div style="grid-row: span 2">b b b b</div>
        <div>a</div>
      </article>
    `, 1)
	page := pages[0]
	html := only(t, page)
	body := only(t, html)
	article := only(t, body)
	children2 := unpack(t, article, 3)
	div1 := children2[0]
	div2 := children2[1]
	div3 := children2[2]
	gridEqual(t, `div_1.position_x == div_3.position_x`, div1.PositionX, div3.PositionX)
	gridEqual(t, `div_3.position_x == 0`, div3.PositionX, 0)
	gridEqual(t, `div_2.position_x == 3`, div2.PositionX, 3)
	gridEqual(t, `div_1.position_y == div_2.position_y`, div1.PositionY, div2.PositionY)
	gridEqual(t, `div_2.position_y == 0`, div2.PositionY, 0)
	gridEqual(t, `div_3.position_y == 4`, div3.PositionY, 4)
	gridEqual(t, `div_1.width == div_2.width`, div1.Width, div2.Width)
	gridEqual(t, `div_2.width == div_3.width`, div2.Width, div3.Width)
	gridEqual(t, `div_3.width == 3`, div3.Width, 3)
	gridEqual(t, `div_1.height == div_3.height`, div1.Height, div3.Height)
	gridEqual(t, `div_3.height == 4`, div3.Height, 4)
	gridEqual(t, `div_2.height == article.height`, div2.Height, article.Height)
	gridEqual(t, `article.height == 8`, article.Height, 8)
	gridEqual(t, `article.width == 6`, article.Width, 6)
}

// TestLayoutGridTemplateMultipleBigSpan is testGridTemplateMultipleBigSpan of tests/layout/testGrid.py.
func TestLayoutGridTemplateMultipleBigSpan(t *testing.T) {
	pages := gridRenderPages(t, `
      <style>
        article {
          display: grid;
          font-family: weasyprint;
          font-size: 2px;
          grid-template-columns: 50% 50%;
          line-height: 1;
          width: 6px;
        }
      </style>
      <article>
        <div>a</div>
        <div style="grid-row: span 3">b b b b b b</div>
        <div>a</div>
        <div>a</div>
        <div style="grid-row: span 2">c</div>
      </article>
    `, 1)
	page := pages[0]
	html := only(t, page)
	body := only(t, html)
	article := only(t, body)
	children2 := unpack(t, article, 5)
	div1 := children2[0]
	div2 := children2[1]
	div3 := children2[2]
	div4 := children2[3]
	div5 := children2[4]
	gridEqual(t, `div_1.position_x == div_3.position_x`, div1.PositionX, div3.PositionX)
	gridEqual(t, `div_3.position_x == div_4.position_x`, div3.PositionX, div4.PositionX)
	gridEqual(t, `div_4.position_x == 0`, div4.PositionX, 0)
	gridEqual(t, `div_5.position_x == 0`, div5.PositionX, 0)
	gridEqual(t, `div_2.position_x == 3`, div2.PositionX, 3)
	gridEqual(t, `div_1.position_y == div_2.position_y`, div1.PositionY, div2.PositionY)
	gridEqual(t, `div_2.position_y == 0`, div2.PositionY, 0)
	gridEqual(t, `div_3.position_y == 4`, div3.PositionY, 4)
	gridEqual(t, `div_4.position_y == 8`, div4.PositionY, 8)
	gridEqual(t, `div_5.position_y == 12`, div5.PositionY, 12)
	gridEqual(t, `div_1.width == div_2.width`, div1.Width, div2.Width)
	gridEqual(t, `div_2.width == div_3.width`, div2.Width, div3.Width)
	gridEqual(t, `div_3.width == div_5.width`, div3.Width, div5.Width)
	gridEqual(t, `div_5.width == 3`, div5.Width, 3)
	gridEqual(t, `div_4.width == 3`, div4.Width, 3)
	gridEqual(t, `div_1.height == div_3.height`, div1.Height, div3.Height)
	gridEqual(t, `div_3.height == div_4.height`, div3.Height, div4.Height)
	gridEqual(t, `div_4.height == 4`, div4.Height, 4)
	gridEqual(t, `div_2.height == 12`, div2.Height, 12)
	gridEqual(t, `div_5.height == 2`, div5.Height, 2)
	gridEqual(t, `article.height == 14`, article.Height, 14)
	gridEqual(t, `article.width == 6`, article.Width, 6)
}

// TestLayoutGridTemplateAreasSpanOverflow is testGridTemplateAreasSpanOverflow of tests/layout/testGrid.py.
func TestLayoutGridTemplateAreasSpanOverflow(t *testing.T) {
	pages := gridRenderPages(t, `
      <style>
        article {
          display: grid;
          font-family: weasyprint;
          font-size: 2px;
          grid-template-columns: 50% 50%;
          line-height: 1;
          width: 10px;
        }
      </style>
      <article>
        <div style="">a</div>
        <div style="grid-column: span 2">a</div>
        <div style="">a</div>
      </article>
    `, 1)
	page := pages[0]
	html := only(t, page)
	body := only(t, html)
	article := only(t, body)
	children2 := unpack(t, article, 3)
	divA1 := children2[0]
	divA2 := children2[1]
	divA3 := children2[2]
	gridEqual(t, `div_a1.position_x == div_a2.position_x`, divA1.PositionX, divA2.PositionX)
	gridEqual(t, `div_a2.position_x == div_a3.position_x`, divA2.PositionX, divA3.PositionX)
	gridEqual(t, `div_a3.position_x == 0`, divA3.PositionX, 0)
	gridEqual(t, `div_a1.position_y == 0`, divA1.PositionY, 0)
	gridEqual(t, `div_a2.position_y == 2`, divA2.PositionY, 2)
	gridEqual(t, `div_a3.position_y == 4`, divA3.PositionY, 4)
	gridEqual(t, `div_a1.width == div_a3.width`, divA1.Width, divA3.Width)
	gridEqual(t, `div_a3.width == 5`, divA3.Width, 5)
	gridEqual(t, `div_a2.width == 10`, divA2.Width, 10)
	gridEqual(t, `div_a1.height == div_a2.height`, divA1.Height, divA2.Height)
	gridEqual(t, `div_a2.height == div_a3.height`, divA2.Height, divA3.Height)
	gridEqual(t, `div_a3.height == 2`, divA3.Height, 2)
	gridEqual(t, `article.width == 10`, article.Width, 10)
}

// TestLayoutGridTemplateAreasExtraSpan is testGridTemplateAreasExtraSpan of tests/layout/testGrid.py.
func TestLayoutGridTemplateAreasExtraSpan(t *testing.T) {
	pages := gridRenderPages(t, `
      <style>
        article {
          display: grid;
          font-family: weasyprint;
          font-size: 2px;
          grid-template-areas: 'a . b' 'c d d';
          line-height: 1;
          width: 10px;
        }
      </style>
      <article>
        <div style="grid-area: a">a</div>
        <div style="grid-area: b">b</div>
        <div style="grid-area: c">c</div>
        <div style="grid-area: d">d</div>
        <div style="grid-row: span 2; grid-column: span 2">e</div>
        <div>f</div>
      </article>
    `, 1)
	page := pages[0]
	html := only(t, page)
	body := only(t, html)
	article := only(t, body)
	children2 := unpack(t, article, 6)
	divA := children2[0]
	divB := children2[1]
	divC := children2[2]
	divD := children2[3]
	divE := children2[4]
	divF := children2[5]
	gridEqual(t, `div_a.position_x == div_c.position_x`, divA.PositionX, divC.PositionX)
	gridEqual(t, `div_c.position_x == div_e.position_x`, divC.PositionX, divE.PositionX)
	gridEqual(t, `div_e.position_x == 0`, divE.PositionX, 0)
	gridEqual(t, `div_d.position_x == 4`, divD.PositionX, 4)
	gridEqual(t, `div_b.position_x == div_f.position_x`, divB.PositionX, divF.PositionX)
	gridEqual(t, `div_f.position_x == 6`, divF.PositionX, 6)
	gridEqual(t, `div_a.position_y == div_b.position_y`, divA.PositionY, divB.PositionY)
	gridEqual(t, `div_b.position_y == 0`, divB.PositionY, 0)
	gridEqual(t, `div_c.position_y == div_d.position_y`, divC.PositionY, divD.PositionY)
	gridEqual(t, `div_d.position_y == 2`, divD.PositionY, 2)
	gridEqual(t, `div_e.position_y == div_f.position_y`, divE.PositionY, divF.PositionY)
	gridEqual(t, `div_f.position_y == 4`, divF.PositionY, 4)
	gridEqual(t, `div_a.width == div_b.width`, divA.Width, divB.Width)
	gridEqual(t, `div_b.width == div_c.width`, divB.Width, divC.Width)
	gridEqual(t, `div_c.width == div_f.width`, divC.Width, divF.Width)
	gridEqual(t, `div_f.width == 4`, divF.Width, 4)
	gridEqual(t, `div_d.width == div_e.width`, divD.Width, divE.Width)
	gridEqual(t, `div_e.width == 6`, divE.Width, 6)
	gridAll(t, `{div.height for div in article.children} == {2}`, article.Children, func(div *Box) float64 { return div.Height }, 2)
	gridEqual(t, `article.width == 10`, article.Width, 10)
}

// TestLayoutGridTemplateAreasExtraSpanDense is testGridTemplateAreasExtraSpanDense of tests/layout/testGrid.py.
func TestLayoutGridTemplateAreasExtraSpanDense(t *testing.T) {
	pages := gridRenderPages(t, `
      <style>
        article {
          display: grid;
          font-family: weasyprint;
          font-size: 2px;
          grid-auto-flow: dense;
          grid-template-areas: 'a . b' 'c d d';
          line-height: 1;
          width: 9px;
        }
      </style>
      <article>
        <div style="grid-area: a">a</div>
        <div style="grid-area: b">b</div>
        <div style="grid-area: c">c</div>
        <div style="grid-area: d">d</div>
        <div style="grid-row: span 2; grid-column: span 2">e</div>
        <div>f</div>
      </article>
    `, 1)
	page := pages[0]
	html := only(t, page)
	body := only(t, html)
	article := only(t, body)
	children2 := unpack(t, article, 6)
	divA := children2[0]
	divB := children2[1]
	divC := children2[2]
	divD := children2[3]
	divE := children2[4]
	divF := children2[5]
	gridEqual(t, `div_a.position_x == div_c.position_x`, divA.PositionX, divC.PositionX)
	gridEqual(t, `div_c.position_x == div_e.position_x`, divC.PositionX, divE.PositionX)
	gridEqual(t, `div_e.position_x == 0`, divE.PositionX, 0)
	gridEqual(t, `div_d.position_x == div_f.position_x`, divD.PositionX, divF.PositionX)
	gridEqual(t, `div_f.position_x == 3`, divF.PositionX, 3)
	gridEqual(t, `div_b.position_x == 6`, divB.PositionX, 6)
	gridEqual(t, `div_a.position_y == div_b.position_y`, divA.PositionY, divB.PositionY)
	gridEqual(t, `div_b.position_y == div_f.position_y`, divB.PositionY, divF.PositionY)
	gridEqual(t, `div_f.position_y == 0`, divF.PositionY, 0)
	gridEqual(t, `div_c.position_y == div_d.position_y`, divC.PositionY, divD.PositionY)
	gridEqual(t, `div_d.position_y == 2`, divD.PositionY, 2)
	gridEqual(t, `div_e.position_y == 4`, divE.PositionY, 4)
	gridEqual(t, `div_a.width == div_b.width`, divA.Width, divB.Width)
	gridEqual(t, `div_b.width == div_c.width`, divB.Width, divC.Width)
	gridEqual(t, `div_c.width == div_f.width`, divC.Width, divF.Width)
	gridEqual(t, `div_f.width == 3`, divF.Width, 3)
	gridEqual(t, `div_d.width == div_e.width`, divD.Width, divE.Width)
	gridEqual(t, `div_e.width == 6`, divE.Width, 6)
	gridAll(t, `{div.height for div in article.children} == {2}`, article.Children, func(div *Box) float64 { return div.Height }, 2)
	gridEqual(t, `article.height == 6`, article.Height, 6)
	gridEqual(t, `article.width == 9`, article.Width, 9)
}

// TestLayoutGridAreaMultipleValues is testGridAreaMultipleValues of tests/layout/testGrid.py.
func TestLayoutGridAreaMultipleValues(t *testing.T) {
	pages := gridRenderPages(t, `
      <style>
        article {
          display: grid;
          font-family: weasyprint;
          font-size: 2px;
          grid-template-columns: 5px 5px;
          grid-template-rows: 2px 2px;
          line-height: 1;
          width: 10px;
        }
      </style>
      <article>
        <div style="grid-area: 2 / 1 / 3 / 2">a</div>
        <div style="grid-area: 1 / 1 / 2 / 3">b</div>
        <div style="grid-area: 2 / 2 / 3 / 3">c</div>
      </article>
    `, 1)
	page := pages[0]
	html := only(t, page)
	body := only(t, html)
	article := only(t, body)
	children2 := unpack(t, article, 3)
	divA := children2[0]
	divB := children2[1]
	divC := children2[2]
	gridEqual(t, `div_a.position_x == div_b.position_x`, divA.PositionX, divB.PositionX)
	gridEqual(t, `div_b.position_x == 0`, divB.PositionX, 0)
	gridEqual(t, `div_c.position_x == 5`, divC.PositionX, 5)
	gridEqual(t, `div_b.position_y == 0`, divB.PositionY, 0)
	gridEqual(t, `div_a.position_y == div_c.position_y`, divA.PositionY, divC.PositionY)
	gridEqual(t, `div_c.position_y == 2`, divC.PositionY, 2)
	gridEqual(t, `div_a.height == div_b.height`, divA.Height, divB.Height)
	gridEqual(t, `div_b.height == div_c.height`, divB.Height, divC.Height)
	gridEqual(t, `div_c.height == 2`, divC.Height, 2)
	gridEqual(t, `div_a.width == div_c.width`, divA.Width, divC.Width)
	gridEqual(t, `div_c.width == 5`, divC.Width, 5)
	gridEqual(t, `div_b.width == 10`, divB.Width, 10)
	gridEqual(t, `article.width == 10`, article.Width, 10)
}

// TestLayoutGridTemplateRepeatFr is testGridTemplateRepeatFr of tests/layout/testGrid.py.
func TestLayoutGridTemplateRepeatFr(t *testing.T) {
	pages := gridRenderPages(t, `
      <style>
        article {
          display: grid;
          font-family: weasyprint;
          font-size: 2px;
          grid-template-columns: repeat(2, 1fr 2fr);
          line-height: 1;
          width: 12px;
        }
      </style>
      <article>
        <div>a</div>
        <div>b</div>
        <div>c</div>
        <div>d</div>
        <div>e</div>
        <div>f</div>
        <div>g</div>
        <div>h</div>
      </article>
    `, 1)
	page := pages[0]
	html := only(t, page)
	body := only(t, html)
	article := only(t, body)
	children2 := unpack(t, article, 8)
	divA := children2[0]
	divB := children2[1]
	divC := children2[2]
	divD := children2[3]
	divE := children2[4]
	divF := children2[5]
	divG := children2[6]
	divH := children2[7]
	gridEqual(t, `div_a.position_x == div_e.position_x`, divA.PositionX, divE.PositionX)
	gridEqual(t, `div_e.position_x == 0`, divE.PositionX, 0)
	gridEqual(t, `div_b.position_x == div_f.position_x`, divB.PositionX, divF.PositionX)
	gridEqual(t, `div_f.position_x == 2`, divF.PositionX, 2)
	gridEqual(t, `div_c.position_x == div_g.position_x`, divC.PositionX, divG.PositionX)
	gridEqual(t, `div_g.position_x == 6`, divG.PositionX, 6)
	gridEqual(t, `div_d.position_x == div_h.position_x`, divD.PositionX, divH.PositionX)
	gridEqual(t, `div_h.position_x == 8`, divH.PositionX, 8)
	gridEqual(t, `div_a.position_y == div_b.position_y`, divA.PositionY, divB.PositionY)
	gridEqual(t, `div_b.position_y == 0`, divB.PositionY, 0)
	gridEqual(t, `div_c.position_y == div_d.position_y`, divC.PositionY, divD.PositionY)
	gridEqual(t, `div_d.position_y == 0`, divD.PositionY, 0)
	gridEqual(t, `div_e.position_y == div_f.position_y`, divE.PositionY, divF.PositionY)
	gridEqual(t, `div_f.position_y == 2`, divF.PositionY, 2)
	gridEqual(t, `div_g.position_y == div_h.position_y`, divG.PositionY, divH.PositionY)
	gridEqual(t, `div_h.position_y == 2`, divH.PositionY, 2)
	gridEqual(t, `div_a.width == div_c.width`, divA.Width, divC.Width)
	gridEqual(t, `div_c.width == div_e.width`, divC.Width, divE.Width)
	gridEqual(t, `div_e.width == div_g.width`, divE.Width, divG.Width)
	gridEqual(t, `div_g.width == 2`, divG.Width, 2)
	gridEqual(t, `div_b.width == div_d.width`, divB.Width, divD.Width)
	gridEqual(t, `div_d.width == div_f.width`, divD.Width, divF.Width)
	gridEqual(t, `div_f.width == div_h.width`, divF.Width, divH.Width)
	gridEqual(t, `div_h.width == 4`, divH.Width, 4)
	gridAll(t, `{div.height for div in article.children} == {2}`, article.Children, func(div *Box) float64 { return div.Height }, 2)
	gridEqual(t, `article.width == 12`, article.Width, 12)
}

// TestLayoutGridTemplateShorthandFr is testGridTemplateShorthandFr of tests/layout/testGrid.py.
func TestLayoutGridTemplateShorthandFr(t *testing.T) {
	pages := gridRenderPages(t, `
      <style>
        article {
          display: grid;
          font-family: weasyprint;
          font-size: 2px;
          grid-template: auto 1fr / auto 1fr auto;
          line-height: 1;
          width: 10px;
        }
      </style>
      <article>
        <div>a</div>
        <div>b</div>
        <div>c</div>
        <div>d</div>
        <div>e</div>
        <div>f</div>
      </article>
    `, 1)
	page := pages[0]
	html := only(t, page)
	body := only(t, html)
	article := only(t, body)
	children2 := unpack(t, article, 6)
	divA := children2[0]
	divB := children2[1]
	divC := children2[2]
	divD := children2[3]
	divE := children2[4]
	divF := children2[5]
	gridEqual(t, `div_a.position_x == div_d.position_x`, divA.PositionX, divD.PositionX)
	gridEqual(t, `div_d.position_x == 0`, divD.PositionX, 0)
	gridEqual(t, `div_b.position_x == div_e.position_x`, divB.PositionX, divE.PositionX)
	gridEqual(t, `div_e.position_x == 2`, divE.PositionX, 2)
	gridEqual(t, `div_c.position_x == div_f.position_x`, divC.PositionX, divF.PositionX)
	gridEqual(t, `div_f.position_x == 8`, divF.PositionX, 8)
	gridEqual(t, `div_a.position_y == div_b.position_y`, divA.PositionY, divB.PositionY)
	gridEqual(t, `div_b.position_y == div_c.position_y`, divB.PositionY, divC.PositionY)
	gridEqual(t, `div_c.position_y == 0`, divC.PositionY, 0)
	gridEqual(t, `div_d.position_y == div_e.position_y`, divD.PositionY, divE.PositionY)
	gridEqual(t, `div_e.position_y == div_f.position_y`, divE.PositionY, divF.PositionY)
	gridEqual(t, `div_f.position_y == 2`, divF.PositionY, 2)
	gridEqual(t, `div_a.width == div_c.width`, divA.Width, divC.Width)
	gridEqual(t, `div_c.width == div_d.width`, divC.Width, divD.Width)
	gridEqual(t, `div_d.width == div_f.width`, divD.Width, divF.Width)
	gridEqual(t, `div_f.width == 2`, divF.Width, 2)
	gridEqual(t, `div_b.width == div_e.width`, divB.Width, divE.Width)
	gridEqual(t, `div_e.width == 6`, divE.Width, 6)
	gridAll(t, `{div.height for div in article.children} == {2}`, article.Children, func(div *Box) float64 { return div.Height }, 2)
	gridEqual(t, `article.width == 10`, article.Width, 10)
}

// TestLayoutGridShorthandAutoFlowRowsFrSize is testGridShorthandAutoFlowRowsFrSize of tests/layout/testGrid.py.
func TestLayoutGridShorthandAutoFlowRowsFrSize(t *testing.T) {
	pages := gridRenderPages(t, `
      <style>
        article {
          display: grid;
          font-family: weasyprint;
          font-size: 2px;
          grid: auto-flow 1fr / 6px;
          line-height: 1;
          width: 10px;
        }
      </style>
      <article>
        <div>a</div>
        <div>b</div>
        <div>c</div>
      </article>
    `, 1)
	page := pages[0]
	html := only(t, page)
	body := only(t, html)
	article := only(t, body)
	children2 := unpack(t, article, 3)
	divA := children2[0]
	divB := children2[1]
	divC := children2[2]
	gridEqual(t, `div_a.position_x == div_b.position_x`, divA.PositionX, divB.PositionX)
	gridEqual(t, `div_b.position_x == div_c.position_x`, divB.PositionX, divC.PositionX)
	gridEqual(t, `div_c.position_x == 0`, divC.PositionX, 0)
	gridEqual(t, `div_a.position_y == 0`, divA.PositionY, 0)
	gridEqual(t, `div_b.position_y == 2`, divB.PositionY, 2)
	gridEqual(t, `div_c.position_y == 4`, divC.PositionY, 4)
	gridEqual(t, `div_a.width == div_b.width`, divA.Width, divB.Width)
	gridEqual(t, `div_b.width == div_c.width`, divB.Width, divC.Width)
	gridEqual(t, `div_c.width == 6`, divC.Width, 6)
	gridAll(t, `{div.height for div in article.children} == {2}`, article.Children, func(div *Box) float64 { return div.Height }, 2)
	gridEqual(t, `article.width == 10`, article.Width, 10)
}

// TestLayoutGridTemplateFrTooLarge is testGridTemplateFrTooLarge of tests/layout/testGrid.py.
func TestLayoutGridTemplateFrTooLarge(t *testing.T) {
	pages := gridRenderPages(t, `
      <style>
        article {
          display: grid;
          font-family: weasyprint;
          font-size: 2px;
          grid-template-columns: 1fr 1fr;
          line-height: 1;
          width: 10px;
        }
      </style>
      <article>
        <div>a</div><div>bbb</div>
      </article>
    `, 1)
	page := pages[0]
	html := only(t, page)
	body := only(t, html)
	article := only(t, body)
	children2 := unpack(t, article, 2)
	divA := children2[0]
	divB := children2[1]
	gridEqual(t, `div_a.position_x == 0`, divA.PositionX, 0)
	gridEqual(t, `div_b.position_x == 4`, divB.PositionX, 4)
	gridEqual(t, `div_a.position_y == div_b.position_y`, divA.PositionY, divB.PositionY)
	gridEqual(t, `div_b.position_y == 0`, divB.PositionY, 0)
	gridEqual(t, `div_a.height == div_b.height`, divA.Height, divB.Height)
	gridEqual(t, `div_b.height == 2`, divB.Height, 2)
	gridEqual(t, `div_a.width == 4`, divA.Width, 4)
	gridEqual(t, `div_b.width == 6`, divB.Width, 6)
	gridEqual(t, `article.width == 10`, article.Width, 10)
}

// TestLayoutGridShorthandAutoFlowColumnsNoneDense is testGridShorthandAutoFlowColumnsNoneDense of tests/layout/testGrid.py.
func TestLayoutGridShorthandAutoFlowColumnsNoneDense(t *testing.T) {
	pages := gridRenderPages(t, `
      <style>
        article {
          display: grid;
          font-family: weasyprint;
          font-size: 2px;
          grid: none / auto-flow dense 1fr;
          line-height: 1;
          width: 12px;
        }
      </style>
      <article>
        <div>a</div>
        <div>b</div>
        <div>c</div>
      </article>
    `, 1)
	page := pages[0]
	html := only(t, page)
	body := only(t, html)
	article := only(t, body)
	children2 := unpack(t, article, 3)
	divA := children2[0]
	divB := children2[1]
	divC := children2[2]
	gridEqual(t, `div_a.position_x == 0`, divA.PositionX, 0)
	gridEqual(t, `div_b.position_x == 4`, divB.PositionX, 4)
	gridEqual(t, `div_c.position_x == 8`, divC.PositionX, 8)
	gridEqual(t, `div_a.position_y == div_b.position_y`, divA.PositionY, divB.PositionY)
	gridEqual(t, `div_b.position_y == div_c.position_y`, divB.PositionY, divC.PositionY)
	gridEqual(t, `div_c.position_y == 0`, divC.PositionY, 0)
	gridEqual(t, `div_a.height == div_b.height`, divA.Height, divB.Height)
	gridEqual(t, `div_b.height == div_c.height`, divB.Height, divC.Height)
	gridEqual(t, `div_c.height == 2`, divC.Height, 2)
	gridAll(t, `{div.width for div in article.children} == {4}`, article.Children, func(div *Box) float64 { return div.Width }, 4)
	gridEqual(t, `article.width == 12`, article.Width, 12)
}

// TestLayoutGridTemplateFrUndefinedFreeSpace is testGridTemplateFrUndefinedFreeSpace of tests/layout/testGrid.py.
func TestLayoutGridTemplateFrUndefinedFreeSpace(t *testing.T) {
	pages := gridRenderPages(t, `
      <style>
        article {
          display: grid;
          font-family: weasyprint;
          font-size: 2px;
          grid-template-rows: 1fr 1fr;
          grid-template-columns: 1fr 1fr;
          line-height: 1;
          width: 10px;
        }
      </style>
      <article>
        <div>a</div> <div>b<br>b<br>b<br>b</div>
        <div>c</div> <div>d</div>
      </article>
    `, 1)
	page := pages[0]
	html := only(t, page)
	body := only(t, html)
	article := only(t, body)
	children2 := unpack(t, article, 4)
	divA := children2[0]
	divB := children2[1]
	divC := children2[2]
	divD := children2[3]
	gridEqual(t, `div_a.position_x == div_c.position_x`, divA.PositionX, divC.PositionX)
	gridEqual(t, `div_c.position_x == 0`, divC.PositionX, 0)
	gridEqual(t, `div_b.position_x == div_d.position_x`, divB.PositionX, divD.PositionX)
	gridEqual(t, `div_d.position_x == 5`, divD.PositionX, 5)
	gridEqual(t, `div_a.height == div_b.height`, divA.Height, divB.Height)
	gridEqual(t, `div_b.height == div_c.height`, divB.Height, divC.Height)
	gridEqual(t, `div_c.height == div_d.height`, divC.Height, divD.Height)
	gridEqual(t, `div_d.height == 8`, divD.Height, 8)
	gridEqual(t, `div_a.width == div_c.width`, divA.Width, divC.Width)
	gridEqual(t, `div_c.width == 5`, divC.Width, 5)
	gridEqual(t, `div_b.width == div_d.width`, divB.Width, divD.Width)
	gridEqual(t, `div_d.width == 5`, divD.Width, 5)
	gridEqual(t, `article.width == 10`, article.Width, 10)
	gridEqual(t, `article.height == 16`, article.Height, 16)
}

// TestLayoutGridColumnStart is testGridColumnStart of tests/layout/testGrid.py.
func TestLayoutGridColumnStart(t *testing.T) {
	pages := gridRenderPages(t, `
      <style>
        dl {
          display: grid;
          font-family: weasyprint;
          font-size: 2px;
          grid-template-columns: max-content auto;
          line-height: 1;
          width: 10px;
        }
        dt {
          display: block;
          grid-column-start: 1;
        }
        dd {
          display: block;
          grid-column-start: 2;
        }
      </style>
      <dl>
        <dt>A</dt>
        <dd>A1</dd>
        <dd>A2</dd>
        <dt>B</dt>
        <dd>B1</dd>
        <dd>B2</dd>
      </dl>
    `, 1)
	page := pages[0]
	html := only(t, page)
	body := only(t, html)
	dl := only(t, body)
	children2 := unpack(t, dl, 6)
	dtA := children2[0]
	ddA1 := children2[1]
	ddA2 := children2[2]
	dtB := children2[3]
	ddB1 := children2[4]
	ddB2 := children2[5]
	gridEqual(t, `dt_a.position_y == dd_a1.position_y`, dtA.PositionY, ddA1.PositionY)
	gridEqual(t, `dd_a1.position_y == 0`, ddA1.PositionY, 0)
	gridEqual(t, `dd_a2.position_y == 2`, ddA2.PositionY, 2)
	gridEqual(t, `dt_b.position_y == dd_b1.position_y`, dtB.PositionY, ddB1.PositionY)
	gridEqual(t, `dd_b1.position_y == 4`, ddB1.PositionY, 4)
	gridEqual(t, `dd_b2.position_y == 6`, ddB2.PositionY, 6)
	gridEqual(t, `dt_a.position_x == dt_b.position_x`, dtA.PositionX, dtB.PositionX)
	gridEqual(t, `dt_b.position_x == 0`, dtB.PositionX, 0)
	gridEqual(t, `dd_a1.position_x == dd_a2.position_x`, ddA1.PositionX, ddA2.PositionX)
	gridEqual(t, `dd_a2.position_x == 2`, ddA2.PositionX, 2)
	gridEqual(t, `dd_b1.position_x == dd_b2.position_x`, ddB1.PositionX, ddB2.PositionX)
	gridEqual(t, `dd_b2.position_x == 2`, ddB2.PositionX, 2)
}

// TestLayoutGridColumnStartBlockified is testGridColumnStartBlockified of tests/layout/testGrid.py.
func TestLayoutGridColumnStartBlockified(t *testing.T) {
	pages := gridRenderPages(t, `
      <style>
        dl {
          display: grid;
          font-family: weasyprint;
          font-size: 2px;
          grid-template-columns: max-content auto;
          line-height: 1;
          width: 10px;
        }
        dt {
          display: inline;
          grid-column-start: 1;
        }
        dd {
          display: inline;
          grid-column-start: 2;
        }
      </style>
      <dl>
        <dt>A</dt>
        <dd>A1</dd>
        <dd>A2</dd>
        <dt>B</dt>
        <dd>B1</dd>
        <dd>B2</dd>
      </dl>
    `, 1)
	page := pages[0]
	html := only(t, page)
	body := only(t, html)
	dl := only(t, body)
	children2 := unpack(t, dl, 6)
	dtA := children2[0]
	ddA1 := children2[1]
	ddA2 := children2[2]
	dtB := children2[3]
	ddB1 := children2[4]
	ddB2 := children2[5]
	gridEqual(t, `dt_a.position_y == dd_a1.position_y`, dtA.PositionY, ddA1.PositionY)
	gridEqual(t, `dd_a1.position_y == 0`, ddA1.PositionY, 0)
	gridEqual(t, `dd_a2.position_y == 2`, ddA2.PositionY, 2)
	gridEqual(t, `dt_b.position_y == dd_b1.position_y`, dtB.PositionY, ddB1.PositionY)
	gridEqual(t, `dd_b1.position_y == 4`, ddB1.PositionY, 4)
	gridEqual(t, `dd_b2.position_y == 6`, ddB2.PositionY, 6)
	gridEqual(t, `dt_a.position_x == dt_b.position_x`, dtA.PositionX, dtB.PositionX)
	gridEqual(t, `dt_b.position_x == 0`, dtB.PositionX, 0)
	gridEqual(t, `dd_a1.position_x == dd_a2.position_x`, ddA1.PositionX, ddA2.PositionX)
	gridEqual(t, `dd_a2.position_x == 2`, ddA2.PositionX, 2)
	gridEqual(t, `dd_b1.position_x == dd_b2.position_x`, ddB1.PositionX, ddB2.PositionX)
	gridEqual(t, `dd_b2.position_x == 2`, ddB2.PositionX, 2)
}

// TestLayoutGridUndefinedFreeSpace is testGridUndefinedFreeSpace of tests/layout/testGrid.py.
func TestLayoutGridUndefinedFreeSpace(t *testing.T) {
	pages := gridRenderPages(t, `
      <style>
        body {
          font-family: weasyprint;
          font-size: 2px;
          line-height: 1;
        }
        .columns {
          display: grid;
          grid-template-columns: 1fr 1fr;
          width: 8px;
        }
        .rows {
          display: grid;
          grid-template-rows: 1fr 1fr;
        }
      </style>
      <div class="columns">
        <div class="rows">
          <div>aa</div>
          <div>b</div>
        </div>
        <div class="rows">
          <div>c<br>c</div>
          <div>d</div>
        </div>
      </div>
    `, 1)
	page := pages[0]
	html := only(t, page)
	body := only(t, html)
	divC := only(t, body)
	children2 := unpack(t, divC, 2)
	divC1 := children2[0]
	divC2 := children2[1]
	children3 := unpack(t, divC1, 2)
	divR11 := children3[0]
	divR12 := children3[1]
	children4 := unpack(t, divC2, 2)
	divR21 := children4[0]
	divR22 := children4[1]
	gridEqual(t, `div_r11.position_x == div_r12.position_x`, divR11.PositionX, divR12.PositionX)
	gridEqual(t, `div_r12.position_x == 0`, divR12.PositionX, 0)
	gridEqual(t, `div_r21.position_x == div_r22.position_x`, divR21.PositionX, divR22.PositionX)
	gridEqual(t, `div_r22.position_x == 4`, divR22.PositionX, 4)
	gridEqual(t, `div_r11.position_y == div_r21.position_y`, divR11.PositionY, divR21.PositionY)
	gridEqual(t, `div_r21.position_y == 0`, divR21.PositionY, 0)
	gridEqual(t, `div_r12.position_y == div_r22.position_y`, divR12.PositionY, divR22.PositionY)
	gridEqual(t, `div_r22.position_y == 4`, divR22.PositionY, 4)
	gridEqual(t, `div_r11.height == div_r12.height`, divR11.Height, divR12.Height)
	gridEqual(t, `div_r12.height == div_r21.height`, divR12.Height, divR21.Height)
	gridEqual(t, `div_r21.height == div_r22.height`, divR21.Height, divR22.Height)
	gridEqual(t, `div_r22.height == 4`, divR22.Height, 4)
	gridEqual(t, `div_r11.width == div_r12.width`, divR11.Width, divR12.Width)
	gridEqual(t, `div_r12.width == div_r21.width`, divR12.Width, divR21.Width)
	gridEqual(t, `div_r21.width == div_r22.width`, divR21.Width, divR22.Width)
	gridEqual(t, `div_r22.width == 4`, divR22.Width, 4)
	gridEqual(t, `div_c.width == 8`, divC.Width, 8)
}

// TestLayoutGridPadding is testGridPadding of tests/layout/testGrid.py.
func TestLayoutGridPadding(t *testing.T) {
	pages := gridRenderPages(t, `
      <style>
        article {
          display: grid;
          font-family: weasyprint;
          font-size: 2px;
          grid-template-rows: auto 1fr;
          grid-template-columns: auto 1fr;
          line-height: 1;
          width: 14px;
        }
      </style>
      <article>
        <div style="padding: 1px">a</div> <div>b</div>
        <div>c</div> <div style="padding: 2px">d</div>
      </article>
    `, 1)
	page := pages[0]
	html := only(t, page)
	body := only(t, html)
	article := only(t, body)
	children2 := unpack(t, article, 4)
	divA := children2[0]
	divB := children2[1]
	divC := children2[2]
	divD := children2[3]
	gridEqual(t, `div_a.position_x == div_c.position_x`, divA.PositionX, divC.PositionX)
	gridEqual(t, `div_c.position_x == div_c.content_box_x()`, divC.PositionX, divC.ContentBoxX())
	gridEqual(t, `div_c.content_box_x() == 0`, divC.ContentBoxX(), 0)
	gridEqual(t, `div_a.content_box_x() == 1`, divA.ContentBoxX(), 1)
	gridEqual(t, `div_b.position_x == div_b.content_box_x()`, divB.PositionX, divB.ContentBoxX())
	gridEqual(t, `div_b.content_box_x() == div_d.position_x`, divB.ContentBoxX(), divD.PositionX)
	gridEqual(t, `div_d.position_x == 4`, divD.PositionX, 4)
	gridEqual(t, `div_d.content_box_x() == 6`, divD.ContentBoxX(), 6)
	gridEqual(t, `div_a.width == 2`, divA.Width, 2)
	gridEqual(t, `div_b.width == 10`, divB.Width, 10)
	gridEqual(t, `div_c.width == 4`, divC.Width, 4)
	gridEqual(t, `div_d.width == 6`, divD.Width, 6)
	gridEqual(t, `article.width == 14`, article.Width, 14)
	gridEqual(t, `div_a.position_y == div_b.position_y`, divA.PositionY, divB.PositionY)
	gridEqual(t, `div_b.position_y == div_b.content_box_y()`, divB.PositionY, divB.ContentBoxY())
	gridEqual(t, `div_b.content_box_y() == 0`, divB.ContentBoxY(), 0)
	gridEqual(t, `div_a.content_box_y() == 1`, divA.ContentBoxY(), 1)
	gridEqual(t, `div_c.position_y == div_c.content_box_y()`, divC.PositionY, divC.ContentBoxY())
	gridEqual(t, `div_c.content_box_y() == div_d.position_y`, divC.ContentBoxY(), divD.PositionY)
	gridEqual(t, `div_d.position_y == 4`, divD.PositionY, 4)
	gridEqual(t, `div_d.content_box_y() == 6`, divD.ContentBoxY(), 6)
	gridEqual(t, `div_a.height == div_d.height`, divA.Height, divD.Height)
	gridEqual(t, `div_d.height == 2`, divD.Height, 2)
	gridEqual(t, `div_b.height == 4`, divB.Height, 4)
	gridEqual(t, `div_c.height == 6`, divC.Height, 6)
	gridEqual(t, `article.height == 10`, article.Height, 10)
}

// TestLayoutGridBorder is testGridBorder of tests/layout/testGrid.py.
func TestLayoutGridBorder(t *testing.T) {
	pages := gridRenderPages(t, `
      <style>
        article {
          display: grid;
          font-family: weasyprint;
          font-size: 2px;
          grid-template-rows: auto 1fr;
          grid-template-columns: auto 1fr;
          line-height: 1;
          width: 14px;
        }
      </style>
      <article>
        <div style="border: 1px solid">a</div> <div>b</div>
        <div>c</div> <div style="border: 2px solid">d</div>
      </article>
    `, 1)
	page := pages[0]
	html := only(t, page)
	body := only(t, html)
	article := only(t, body)
	children2 := unpack(t, article, 4)
	divA := children2[0]
	divB := children2[1]
	divC := children2[2]
	divD := children2[3]
	gridEqual(t, `div_a.position_x == div_c.position_x`, divA.PositionX, divC.PositionX)
	gridEqual(t, `div_c.position_x == div_c.padding_box_x()`, divC.PositionX, divC.PaddingBoxX())
	gridEqual(t, `div_c.padding_box_x() == 0`, divC.PaddingBoxX(), 0)
	gridEqual(t, `div_a.padding_box_x() == 1`, divA.PaddingBoxX(), 1)
	gridEqual(t, `div_b.position_x == div_b.padding_box_x()`, divB.PositionX, divB.PaddingBoxX())
	gridEqual(t, `div_b.padding_box_x() == div_d.position_x`, divB.PaddingBoxX(), divD.PositionX)
	gridEqual(t, `div_d.position_x == 4`, divD.PositionX, 4)
	gridEqual(t, `div_d.padding_box_x() == 6`, divD.PaddingBoxX(), 6)
	gridEqual(t, `div_a.width == 2`, divA.Width, 2)
	gridEqual(t, `div_b.width == 10`, divB.Width, 10)
	gridEqual(t, `div_c.width == 4`, divC.Width, 4)
	gridEqual(t, `div_d.width == 6`, divD.Width, 6)
	gridEqual(t, `article.width == 14`, article.Width, 14)
	gridEqual(t, `div_a.position_y == div_b.position_y`, divA.PositionY, divB.PositionY)
	gridEqual(t, `div_b.position_y == div_b.padding_box_y()`, divB.PositionY, divB.PaddingBoxY())
	gridEqual(t, `div_b.padding_box_y() == 0`, divB.PaddingBoxY(), 0)
	gridEqual(t, `div_a.padding_box_y() == 1`, divA.PaddingBoxY(), 1)
	gridEqual(t, `div_c.position_y == div_c.padding_box_y()`, divC.PositionY, divC.PaddingBoxY())
	gridEqual(t, `div_c.padding_box_y() == div_d.position_y`, divC.PaddingBoxY(), divD.PositionY)
	gridEqual(t, `div_d.position_y == 4`, divD.PositionY, 4)
	gridEqual(t, `div_d.padding_box_y() == 6`, divD.PaddingBoxY(), 6)
	gridEqual(t, `div_a.height == div_d.height`, divA.Height, divD.Height)
	gridEqual(t, `div_d.height == 2`, divD.Height, 2)
	gridEqual(t, `div_b.height == 4`, divB.Height, 4)
	gridEqual(t, `div_c.height == 6`, divC.Height, 6)
	gridEqual(t, `article.height == 10`, article.Height, 10)
}

// TestLayoutGridBorderBox is testGridBorderBox of tests/layout/testGrid.py.
func TestLayoutGridBorderBox(t *testing.T) {
	pages := gridRenderPages(t, `
      <style>
        article {
          display: grid;
          font-family: weasyprint;
          font-size: 2px;
          grid-template-rows: auto 1fr;
          grid-template-columns: auto 1fr;
          line-height: 1;
          width: 14px;
        }
        div { box-sizing: border-box }
      </style>
      <article>
        <div style="border: 1px solid; width: 7px">a</div> <div>b</div>
        <div>c</div> <div style="padding: 2px; height: 7px">d</div>
      </article>
    `, 1)
	page := pages[0]
	html := only(t, page)
	body := only(t, html)
	article := only(t, body)
	children2 := unpack(t, article, 4)
	divA := children2[0]
	divB := children2[1]
	divC := children2[2]
	divD := children2[3]
	gridEqual(t, `div_a.position_x == div_c.position_x`, divA.PositionX, divC.PositionX)
	gridEqual(t, `div_c.position_x == div_c.padding_box_x()`, divC.PositionX, divC.PaddingBoxX())
	gridEqual(t, `div_c.padding_box_x() == 0`, divC.PaddingBoxX(), 0)
	gridEqual(t, `div_a.padding_box_x() == 1`, divA.PaddingBoxX(), 1)
	gridEqual(t, `div_b.position_x == div_b.padding_box_x()`, divB.PositionX, divB.PaddingBoxX())
	gridEqual(t, `div_b.padding_box_x() == div_d.position_x`, divB.PaddingBoxX(), divD.PositionX)
	gridEqual(t, `div_d.position_x == 7`, divD.PositionX, 7)
	gridEqual(t, `div_d.content_box_x() == 9`, divD.ContentBoxX(), 9)
	gridEqual(t, `div_a.width == 5`, divA.Width, 5)
	gridEqual(t, `div_b.width == 7`, divB.Width, 7)
	gridEqual(t, `div_c.width == 7`, divC.Width, 7)
	gridEqual(t, `div_d.width == 3`, divD.Width, 3)
	gridEqual(t, `article.width == 14`, article.Width, 14)
	gridEqual(t, `div_a.position_y == div_b.position_y`, divA.PositionY, divB.PositionY)
	gridEqual(t, `div_b.position_y == div_b.padding_box_y()`, divB.PositionY, divB.PaddingBoxY())
	gridEqual(t, `div_b.padding_box_y() == 0`, divB.PaddingBoxY(), 0)
	gridEqual(t, `div_a.padding_box_y() == 1`, divA.PaddingBoxY(), 1)
	gridEqual(t, `div_c.position_y == div_c.padding_box_y()`, divC.PositionY, divC.PaddingBoxY())
	gridEqual(t, `div_c.padding_box_y() == div_d.position_y`, divC.PaddingBoxY(), divD.PositionY)
	gridEqual(t, `div_d.position_y == 4`, divD.PositionY, 4)
	gridEqual(t, `div_d.content_box_y() == 6`, divD.ContentBoxY(), 6)
	gridEqual(t, `div_a.height == 2`, divA.Height, 2)
	gridEqual(t, `div_b.height == 4`, divB.Height, 4)
	gridEqual(t, `div_c.height == 7`, divC.Height, 7)
	gridEqual(t, `div_d.height == 3`, divD.Height, 3)
	gridEqual(t, `article.height == 11`, article.Height, 11)
}

// TestLayoutGridItemBorderBox is testGridItemBorderBox of tests/layout/testGrid.py.
func TestLayoutGridItemBorderBox(t *testing.T) {
	pages := gridRenderPages(t, `
      <style>
        @page { size: 50px }
      </style>
      <article style="display: grid">
        <section style="box-sizing: border-box; padding: 5px">
          <div>
    `, 1)
	page := pages[0]
	html := only(t, page)
	body := only(t, html)
	article := only(t, body)
	section := only(t, article)
	gridEqual(t, `section.margin_width() == 50`, section.MarginWidth(), 50)
	gridEqual(t, `section.width == 40`, section.Width, 40)
	gridEqual(t, `section.position_x == section.position_y`, section.PositionX, section.PositionY)
	gridEqual(t, `section.position_y == 0`, section.PositionY, 0)
	div := only(t, section)
	gridEqual(t, `div.width == div.margin_width()`, div.Width, div.MarginWidth())
	gridEqual(t, `div.margin_width() == 40`, div.MarginWidth(), 40)
	gridEqual(t, `div.position_x == div.position_y`, div.PositionX, div.PositionY)
	gridEqual(t, `div.position_y == 5`, div.PositionY, 5)
}

// TestLayoutGridBorderSplit is testGridBorderSplit of tests/layout/testGrid.py.
func TestLayoutGridBorderSplit(t *testing.T) {
	pages := gridRenderPages(t, `
      <style>
        @page { size: 5px }
        body { font: 2px / 1 weasyprint }
      </style>
      <article style="display: grid; border-top: 1px solid; border-bottom: 1px solid;
                      margin: 1px 0; padding: 1px 0">
        <div>a<br>b</div>
      </article>
    `, 2)
	page1 := pages[0]
	page2 := pages[1]
	html := only(t, page1)
	body := only(t, html)
	article := only(t, body)
	gridEqual(t, `article.border_top_width == 1`, article.BorderTopWidth, 1)
	gridEqual(t, `article.border_bottom_width == 0`, article.BorderBottomWidth, 0)
	gridEqual(t, `article.margin_height() == 5`, article.MarginHeight(), 5)
	section := only(t, article)
	gridEqual(t, `section.position_x == 0`, section.PositionX, 0)
	gridEqual(t, `section.position_y == 3`, section.PositionY, 3)
	gridEqual(t, `section.width == html.width`, section.Width, html.Width)
	gridEqual(t, `section.height == 2`, section.Height, 2)
	html = only(t, page2)
	body = only(t, html)
	article = only(t, body)
	gridEqual(t, `article.border_top_width == 0`, article.BorderTopWidth, 0)
	gridEqual(t, `article.border_bottom_width == 1`, article.BorderBottomWidth, 1)
	gridEqual(t, `article.margin_height() == 5`, article.MarginHeight(), 5)
	section = only(t, article)
	gridEqual(t, `section.position_x == 0`, section.PositionX, 0)
	gridEqual(t, `section.position_y == 0`, section.PositionY, 0)
	gridEqual(t, `section.width == html.width`, section.Width, html.Width)
	gridEqual(t, `section.height == 2`, section.Height, 2)
}

// TestLayoutGridMargin is testGridMargin of tests/layout/testGrid.py.
func TestLayoutGridMargin(t *testing.T) {
	pages := gridRenderPages(t, `
      <style>
        article {
          display: grid;
          font-family: weasyprint;
          font-size: 2px;
          grid-template-rows: auto 1fr;
          grid-template-columns: auto 1fr;
          line-height: 1;
          width: 14px;
        }
      </style>
      <article>
        <div style="margin: 1px">a</div> <div>b</div>
        <div>c</div> <div style="margin: 2px">d</div>
      </article>
    `, 1)
	page := pages[0]
	html := only(t, page)
	body := only(t, html)
	article := only(t, body)
	children2 := unpack(t, article, 4)
	divA := children2[0]
	divB := children2[1]
	divC := children2[2]
	divD := children2[3]
	gridEqual(t, `div_a.position_x == div_c.position_x`, divA.PositionX, divC.PositionX)
	gridEqual(t, `div_c.position_x == div_c.border_box_x()`, divC.PositionX, divC.BorderBoxX())
	gridEqual(t, `div_c.border_box_x() == 0`, divC.BorderBoxX(), 0)
	gridEqual(t, `div_a.border_box_x() == 1`, divA.BorderBoxX(), 1)
	gridEqual(t, `div_b.position_x == div_b.border_box_x()`, divB.PositionX, divB.BorderBoxX())
	gridEqual(t, `div_b.border_box_x() == div_d.position_x`, divB.BorderBoxX(), divD.PositionX)
	gridEqual(t, `div_d.position_x == 4`, divD.PositionX, 4)
	gridEqual(t, `div_d.border_box_x() == 6`, divD.BorderBoxX(), 6)
	gridEqual(t, `div_a.width == 2`, divA.Width, 2)
	gridEqual(t, `div_b.width == 10`, divB.Width, 10)
	gridEqual(t, `div_c.width == 4`, divC.Width, 4)
	gridEqual(t, `div_d.width == 6`, divD.Width, 6)
	gridEqual(t, `article.width == 14`, article.Width, 14)
	gridEqual(t, `div_a.position_y == div_b.position_y`, divA.PositionY, divB.PositionY)
	gridEqual(t, `div_b.position_y == div_b.border_box_y()`, divB.PositionY, divB.BorderBoxY())
	gridEqual(t, `div_b.border_box_y() == 0`, divB.BorderBoxY(), 0)
	gridEqual(t, `div_a.border_box_y() == 1`, divA.BorderBoxY(), 1)
	gridEqual(t, `div_c.position_y == div_c.border_box_y()`, divC.PositionY, divC.BorderBoxY())
	gridEqual(t, `div_c.border_box_y() == div_d.position_y`, divC.BorderBoxY(), divD.PositionY)
	gridEqual(t, `div_d.position_y == 4`, divD.PositionY, 4)
	gridEqual(t, `div_d.border_box_y() == 6`, divD.BorderBoxY(), 6)
	gridEqual(t, `div_a.height == div_d.height`, divA.Height, divD.Height)
	gridEqual(t, `div_d.height == 2`, divD.Height, 2)
	gridEqual(t, `div_b.height == 4`, divB.Height, 4)
	gridEqual(t, `div_c.height == 6`, divC.Height, 6)
	gridEqual(t, `article.height == 10`, article.Height, 10)
}

// TestLayoutGridItemMargin is testGridItemMargin of tests/layout/testGrid.py.
func TestLayoutGridItemMargin(t *testing.T) {
	pages := gridRenderPages(t, `
      <style>
        article { display: grid }
        div { margin: auto }
      </style>
      <article>
        <div>a</div>
        <div>b</div>
      </article>
    `, 1)
	page := pages[0]
	html := only(t, page)
	body := only(t, html)
	article := only(t, body)
	unpack(t, article, 2)
}

// TestLayoutGridAutoFlowColumn is testGridAutoFlowColumn of tests/layout/testGrid.py.
func TestLayoutGridAutoFlowColumn(t *testing.T) {
	pages := gridRenderPages(t, `
      <article style="display: grid; grid-auto-flow: column">
        <div>a</div>
        <div>a</div>
        <div>a</div>
      </article>
    `, 1)
	page := pages[0]
	html := only(t, page)
	body := only(t, html)
	article := only(t, body)
	children2 := unpack(t, article, 3)
	divA := children2[0]
	divB := children2[1]
	divC := children2[2]
	gridLess(t, `div_a.position_x < div_b.position_x`, divA.PositionX, divB.PositionX)
	gridLess(t, `div_b.position_x < div_c.position_x`, divB.PositionX, divC.PositionX)
	gridEqual(t, `div_a.position_y == div_b.position_y`, divA.PositionY, divB.PositionY)
	gridEqual(t, `div_b.position_y == div_c.position_y`, divB.PositionY, divC.PositionY)
	gridEqual(t, `div_c.position_y == 0`, divC.PositionY, 0)
	gridEqual(t, `div_a.width == div_b.width`, divA.Width, divB.Width)
	gridEqual(t, `div_b.width == div_c.width`, divB.Width, divC.Width)
	gridEqual(t, `div_a.height == div_b.height`, divA.Height, divB.Height)
	gridEqual(t, `div_b.height == div_c.height`, divB.Height, divC.Height)
	gridEqual(t, `div_c.height == html.height`, divC.Height, html.Height)
	gridEqual(t, `html.height == article.height`, html.Height, article.Height)
}

// TestLayoutGridTemplateAreasExtraSpanColumnDense is testGridTemplateAreasExtraSpanColumnDense of tests/layout/testGrid.py.
func TestLayoutGridTemplateAreasExtraSpanColumnDense(t *testing.T) {
	pages := gridRenderPages(t, `
      <style>
        article {
          display: grid;
          font-family: weasyprint;
          font-size: 2px;
          grid-auto-flow: column dense;
          grid-template-areas: 'a . b' 'c d d';
          line-height: 1;
          width: 12px;
        }
      </style>
      <article>
        <div style="grid-area: a">a</div>
        <div style="grid-area: b">b</div>
        <div style="grid-area: c">c</div>
        <div style="grid-area: d">d</div>
        <div style="grid-row: span 2">e</div>
        <div>f</div>
      </article>
    `, 1)
	page := pages[0]
	html := only(t, page)
	body := only(t, html)
	article := only(t, body)
	children2 := unpack(t, article, 6)
	divA := children2[0]
	divB := children2[1]
	divC := children2[2]
	divD := children2[3]
	divE := children2[4]
	divF := children2[5]
	gridEqual(t, `div_a.position_x == div_c.position_x`, divA.PositionX, divC.PositionX)
	gridEqual(t, `div_c.position_x == 0`, divC.PositionX, 0)
	gridEqual(t, `div_d.position_x == div_f.position_x`, divD.PositionX, divF.PositionX)
	gridEqual(t, `div_f.position_x == 3`, divF.PositionX, 3)
	gridEqual(t, `div_b.position_x == 6`, divB.PositionX, 6)
	gridEqual(t, `div_e.position_x == 9`, divE.PositionX, 9)
	gridEqual(t, `div_a.position_y == div_b.position_y`, divA.PositionY, divB.PositionY)
	gridEqual(t, `div_b.position_y == div_e.position_y`, divB.PositionY, divE.PositionY)
	gridEqual(t, `div_e.position_y == div_f.position_y`, divE.PositionY, divF.PositionY)
	gridEqual(t, `div_f.position_y == 0`, divF.PositionY, 0)
	gridEqual(t, `div_c.position_y == div_d.position_y`, divC.PositionY, divD.PositionY)
	gridEqual(t, `div_d.position_y == 2`, divD.PositionY, 2)
	gridEqual(t, `div_a.width == div_b.width`, divA.Width, divB.Width)
	gridEqual(t, `div_b.width == div_c.width`, divB.Width, divC.Width)
	gridEqual(t, `div_c.width == div_e.width`, divC.Width, divE.Width)
	gridEqual(t, `div_e.width == div_f.width`, divE.Width, divF.Width)
	gridEqual(t, `div_f.width == 3`, divF.Width, 3)
	gridEqual(t, `div_d.width == 6`, divD.Width, 6)
	gridEqual(t, `div_a.height == div_b.height`, divA.Height, divB.Height)
	gridEqual(t, `div_b.height == div_c.height`, divB.Height, divC.Height)
	gridEqual(t, `div_c.height == div_d.height`, divC.Height, divD.Height)
	gridEqual(t, `div_d.height == div_f.height`, divD.Height, divF.Height)
	gridEqual(t, `div_f.height == 2`, divF.Height, 2)
	gridEqual(t, `div_e.height == 4`, divE.Height, 4)
	gridEqual(t, `article.height == 4`, article.Height, 4)
	gridEqual(t, `article.width == 12`, article.Width, 12)
}

// TestLayoutGridGapExplicitGridColumn is testGridGapExplicitGridColumn of tests/layout/testGrid.py.
func TestLayoutGridGapExplicitGridColumn(t *testing.T) {
	pages := gridRenderPages(t, `
      <style>
        article {
          display: grid;
          font-family: weasyprint;
          font-size: 2px;
          gap: 2px;
          grid-template-columns: 1fr;
          line-height: 1;
          width: 12px;
        }
      </style>
      <article>
        <div>a</div>
        <div style="grid-column: 1">b</div>
      </article>
    `, 1)
	page := pages[0]
	html := only(t, page)
	body := only(t, html)
	article := only(t, body)
	children2 := unpack(t, article, 2)
	divA := children2[0]
	divB := children2[1]
	gridEqual(t, `div_a.position_x == div_b.position_x`, divA.PositionX, divB.PositionX)
	gridEqual(t, `div_b.position_x == 0`, divB.PositionX, 0)
	gridEqual(t, `div_a.position_y == 0`, divA.PositionY, 0)
	gridEqual(t, `div_b.position_y == 4`, divB.PositionY, 4)
	gridEqual(t, `article.height == 6`, article.Height, 6)
	gridEqual(t, `article.width == 12`, article.Width, 12)
}

// TestLayoutGridBreak is testGridBreak of tests/layout/testGrid.py.
func TestLayoutGridBreak(t *testing.T) {
	pages := gridRenderPages(t, `
      <style>
        @page { size: 10px 11px }
        body { font: 2px / 1 weasyprint }
        section { display: grid; grid-template-columns: 1fr 1fr }
      </style>
      <section>
        <div>1</div>
        <div>2</div>
        <div>3</div>
        <div>4</div>
        <div>5</div>
        <div>6</div>
        <div>7a<br>7b<br>7c</div>
        <div>8</div>
        <div>9</div>
        <div>10</div>
      </section>
    `, 2)
	page1 := pages[0]
	page2 := pages[1]
	html := only(t, page1)
	body := only(t, html)
	section := only(t, body)
	gridEqual(t, `section.height == 11`, section.Height, 11)
	children2 := unpack(t, section, 8)
	div1 := children2[0]
	div2 := children2[1]
	div3 := children2[2]
	div4 := children2[3]
	div5 := children2[4]
	div6 := children2[5]
	div7 := children2[6]
	div8 := children2[7]
	gridEqual(t, `div1.position_x == div3.position_x`, div1.PositionX, div3.PositionX)
	gridEqual(t, `div3.position_x == div5.position_x`, div3.PositionX, div5.PositionX)
	gridEqual(t, `div5.position_x == div7.position_x`, div5.PositionX, div7.PositionX)
	gridEqual(t, `div7.position_x == 0`, div7.PositionX, 0)
	gridEqual(t, `div2.position_x == div4.position_x`, div2.PositionX, div4.PositionX)
	gridEqual(t, `div4.position_x == div6.position_x`, div4.PositionX, div6.PositionX)
	gridEqual(t, `div6.position_x == div8.position_x`, div6.PositionX, div8.PositionX)
	gridEqual(t, `div8.position_x == 5`, div8.PositionX, 5)
	gridEqual(t, `div1.position_y == div2.position_y`, div1.PositionY, div2.PositionY)
	gridEqual(t, `div2.position_y == 0`, div2.PositionY, 0)
	gridEqual(t, `div1.height == div2.height`, div1.Height, div2.Height)
	gridEqual(t, `div2.height == 2`, div2.Height, 2)
	gridEqual(t, `div3.position_y == div4.position_y`, div3.PositionY, div4.PositionY)
	gridEqual(t, `div4.position_y == 2`, div4.PositionY, 2)
	gridEqual(t, `div3.height == div4.height`, div3.Height, div4.Height)
	gridEqual(t, `div4.height == 2`, div4.Height, 2)
	gridEqual(t, `div5.position_y == div6.position_y`, div5.PositionY, div6.PositionY)
	gridEqual(t, `div6.position_y == 4`, div6.PositionY, 4)
	gridEqual(t, `div5.height == div6.height`, div5.Height, div6.Height)
	gridEqual(t, `div6.height == 2`, div6.Height, 2)
	gridEqual(t, `div7.position_y == div8.position_y`, div7.PositionY, div8.PositionY)
	gridEqual(t, `div8.position_y == 6`, div8.PositionY, 6)
	gridEqual(t, `div7.height == div8.height`, div7.Height, div8.Height)
	gridEqual(t, `div8.height == 5`, div8.Height, 5)
	children3 := unpack(t, div7, 2)
	line1 := children3[0]
	line2 := children3[1]
	children4 := unpack(t, line1, 2)
	text := children4[0]
	gridEqualString(t, `text.text == '7a'`, text.Text, `7a`)
	children5 := unpack(t, line2, 2)
	text = children5[0]
	gridEqualString(t, `text.text == '7b'`, text.Text, `7b`)
	line := only(t, div8)
	text = only(t, line)
	gridEqualString(t, `text.text == '8'`, text.Text, `8`)
	html = only(t, page2)
	body = only(t, html)
	section = only(t, body)
	gridEqual(t, `section.height == 4`, section.Height, 4)
	children6 := unpack(t, section, 4)
	div7 = children6[0]
	div8 = children6[1]
	div9 := children6[2]
	div10 := children6[3]
	gridEqual(t, `div7.position_x == div9.position_x`, div7.PositionX, div9.PositionX)
	gridEqual(t, `div9.position_x == 0`, div9.PositionX, 0)
	gridEqual(t, `div8.position_x == div10.position_x`, div8.PositionX, div10.PositionX)
	gridEqual(t, `div10.position_x == 5`, div10.PositionX, 5)
	gridEqual(t, `div7.position_y == div8.position_y`, div7.PositionY, div8.PositionY)
	gridEqual(t, `div8.position_y == 0`, div8.PositionY, 0)
	gridEqual(t, `div9.position_y == div10.position_y`, div9.PositionY, div10.PositionY)
	gridEqual(t, `div10.position_y == 2`, div10.PositionY, 2)
	gridEqual(t, `div7.height == div8.height`, div7.Height, div8.Height)
	gridEqual(t, `div8.height == div9.height`, div8.Height, div9.Height)
	gridEqual(t, `div9.height == div10.height`, div9.Height, div10.Height)
	gridEqual(t, `div10.height == 2`, div10.Height, 2)
	line = only(t, div7)
	text = only(t, line)
	gridEqualString(t, `text.text == '7c'`, text.Text, `7c`)
	gridEqual(t, `not div8.children`, float64(len(div8.Children)), 0)
}

// TestLayoutGridBreakOrderNegative is testGridBreakOrderNegative of tests/layout/testGrid.py.
func TestLayoutGridBreakOrderNegative(t *testing.T) {
	pages := gridRenderPages(t, `
      <style>
        @page { size: 10px 11px }
        body { font: 2px / 1 weasyprint }
        section { display: grid; grid-template-columns: 1fr 1fr }
      </style>
      <section>
        <div>3</div>
        <div>4</div>
        <div>5</div>
        <div>6</div>
        <div>7a<br>7b<br>7c</div>
        <div>8</div>
        <div>9</div>
        <div>10</div>
        <div style="order: -2">1</div>
        <div style="order: -1">2</div>
      </section>
    `, 2)
	page1 := pages[0]
	page2 := pages[1]
	html := only(t, page1)
	body := only(t, html)
	section := only(t, body)
	gridEqual(t, `section.height == 11`, section.Height, 11)
	children2 := unpack(t, section, 8)
	div1 := children2[0]
	div2 := children2[1]
	div3 := children2[2]
	div4 := children2[3]
	div5 := children2[4]
	div6 := children2[5]
	div7 := children2[6]
	div8 := children2[7]
	gridEqual(t, `div1.position_x == div3.position_x`, div1.PositionX, div3.PositionX)
	gridEqual(t, `div3.position_x == div5.position_x`, div3.PositionX, div5.PositionX)
	gridEqual(t, `div5.position_x == div7.position_x`, div5.PositionX, div7.PositionX)
	gridEqual(t, `div7.position_x == 0`, div7.PositionX, 0)
	gridEqual(t, `div2.position_x == div4.position_x`, div2.PositionX, div4.PositionX)
	gridEqual(t, `div4.position_x == div6.position_x`, div4.PositionX, div6.PositionX)
	gridEqual(t, `div6.position_x == div8.position_x`, div6.PositionX, div8.PositionX)
	gridEqual(t, `div8.position_x == 5`, div8.PositionX, 5)
	gridEqual(t, `div1.position_y == div2.position_y`, div1.PositionY, div2.PositionY)
	gridEqual(t, `div2.position_y == 0`, div2.PositionY, 0)
	gridEqual(t, `div1.height == div2.height`, div1.Height, div2.Height)
	gridEqual(t, `div2.height == 2`, div2.Height, 2)
	gridEqual(t, `div3.position_y == div4.position_y`, div3.PositionY, div4.PositionY)
	gridEqual(t, `div4.position_y == 2`, div4.PositionY, 2)
	gridEqual(t, `div3.height == div4.height`, div3.Height, div4.Height)
	gridEqual(t, `div4.height == 2`, div4.Height, 2)
	gridEqual(t, `div5.position_y == div6.position_y`, div5.PositionY, div6.PositionY)
	gridEqual(t, `div6.position_y == 4`, div6.PositionY, 4)
	gridEqual(t, `div5.height == div6.height`, div5.Height, div6.Height)
	gridEqual(t, `div6.height == 2`, div6.Height, 2)
	gridEqual(t, `div7.position_y == div8.position_y`, div7.PositionY, div8.PositionY)
	gridEqual(t, `div8.position_y == 6`, div8.PositionY, 6)
	gridEqual(t, `div7.height == div8.height`, div7.Height, div8.Height)
	gridEqual(t, `div8.height == 5`, div8.Height, 5)
	children3 := unpack(t, div7, 2)
	line1 := children3[0]
	line2 := children3[1]
	children4 := unpack(t, line1, 2)
	text := children4[0]
	gridEqualString(t, `text.text == '7a'`, text.Text, `7a`)
	children5 := unpack(t, line2, 2)
	text = children5[0]
	gridEqualString(t, `text.text == '7b'`, text.Text, `7b`)
	line := only(t, div8)
	text = only(t, line)
	gridEqualString(t, `text.text == '8'`, text.Text, `8`)
	html = only(t, page2)
	body = only(t, html)
	section = only(t, body)
	gridEqual(t, `section.height == 4`, section.Height, 4)
	children6 := unpack(t, section, 4)
	div7 = children6[0]
	div8 = children6[1]
	div9 := children6[2]
	div10 := children6[3]
	gridEqual(t, `div7.position_x == div9.position_x`, div7.PositionX, div9.PositionX)
	gridEqual(t, `div9.position_x == 0`, div9.PositionX, 0)
	gridEqual(t, `div8.position_x == div10.position_x`, div8.PositionX, div10.PositionX)
	gridEqual(t, `div10.position_x == 5`, div10.PositionX, 5)
	gridEqual(t, `div7.position_y == div8.position_y`, div7.PositionY, div8.PositionY)
	gridEqual(t, `div8.position_y == 0`, div8.PositionY, 0)
	gridEqual(t, `div9.position_y == div10.position_y`, div9.PositionY, div10.PositionY)
	gridEqual(t, `div10.position_y == 2`, div10.PositionY, 2)
	gridEqual(t, `div7.height == div8.height`, div7.Height, div8.Height)
	gridEqual(t, `div8.height == div9.height`, div8.Height, div9.Height)
	gridEqual(t, `div9.height == div10.height`, div9.Height, div10.Height)
	gridEqual(t, `div10.height == 2`, div10.Height, 2)
	line = only(t, div7)
	text = only(t, line)
	gridEqualString(t, `text.text == '7c'`, text.Text, `7c`)
	gridEqual(t, `not div8.children`, float64(len(div8.Children)), 0)
}

// TestLayoutGridBreakOrderPositive is testGridBreakOrderPositive of tests/layout/testGrid.py.
func TestLayoutGridBreakOrderPositive(t *testing.T) {
	pages := gridRenderPages(t, `
      <style>
        @page { size: 10px 11px }
        body { font: 2px / 1 weasyprint }
        section { display: grid; grid-template-columns: 1fr 1fr }
      </style>
      <section>
        <div>1</div>
        <div>2</div>
        <div style="order: 9">9</div>
        <div style="order: 10">10</div>
        <div>3</div>
        <div>4</div>
        <div>5</div>
        <div>6</div>
        <div>7a<br>7b<br>7c</div>
        <div>8</div>
      </section>
    `, 2)
	page1 := pages[0]
	page2 := pages[1]
	html := only(t, page1)
	body := only(t, html)
	section := only(t, body)
	gridEqual(t, `section.height == 11`, section.Height, 11)
	children2 := unpack(t, section, 8)
	div1 := children2[0]
	div2 := children2[1]
	div3 := children2[2]
	div4 := children2[3]
	div5 := children2[4]
	div6 := children2[5]
	div7 := children2[6]
	div8 := children2[7]
	gridEqual(t, `div1.position_x == div3.position_x`, div1.PositionX, div3.PositionX)
	gridEqual(t, `div3.position_x == div5.position_x`, div3.PositionX, div5.PositionX)
	gridEqual(t, `div5.position_x == div7.position_x`, div5.PositionX, div7.PositionX)
	gridEqual(t, `div7.position_x == 0`, div7.PositionX, 0)
	gridEqual(t, `div2.position_x == div4.position_x`, div2.PositionX, div4.PositionX)
	gridEqual(t, `div4.position_x == div6.position_x`, div4.PositionX, div6.PositionX)
	gridEqual(t, `div6.position_x == div8.position_x`, div6.PositionX, div8.PositionX)
	gridEqual(t, `div8.position_x == 5`, div8.PositionX, 5)
	gridEqual(t, `div1.position_y == div2.position_y`, div1.PositionY, div2.PositionY)
	gridEqual(t, `div2.position_y == 0`, div2.PositionY, 0)
	gridEqual(t, `div1.height == div2.height`, div1.Height, div2.Height)
	gridEqual(t, `div2.height == 2`, div2.Height, 2)
	gridEqual(t, `div3.position_y == div4.position_y`, div3.PositionY, div4.PositionY)
	gridEqual(t, `div4.position_y == 2`, div4.PositionY, 2)
	gridEqual(t, `div3.height == div4.height`, div3.Height, div4.Height)
	gridEqual(t, `div4.height == 2`, div4.Height, 2)
	gridEqual(t, `div5.position_y == div6.position_y`, div5.PositionY, div6.PositionY)
	gridEqual(t, `div6.position_y == 4`, div6.PositionY, 4)
	gridEqual(t, `div5.height == div6.height`, div5.Height, div6.Height)
	gridEqual(t, `div6.height == 2`, div6.Height, 2)
	gridEqual(t, `div7.position_y == div8.position_y`, div7.PositionY, div8.PositionY)
	gridEqual(t, `div8.position_y == 6`, div8.PositionY, 6)
	gridEqual(t, `div7.height == div8.height`, div7.Height, div8.Height)
	gridEqual(t, `div8.height == 5`, div8.Height, 5)
	children3 := unpack(t, div7, 2)
	line1 := children3[0]
	line2 := children3[1]
	children4 := unpack(t, line1, 2)
	text := children4[0]
	gridEqualString(t, `text.text == '7a'`, text.Text, `7a`)
	children5 := unpack(t, line2, 2)
	text = children5[0]
	gridEqualString(t, `text.text == '7b'`, text.Text, `7b`)
	line := only(t, div8)
	text = only(t, line)
	gridEqualString(t, `text.text == '8'`, text.Text, `8`)
	html = only(t, page2)
	body = only(t, html)
	section = only(t, body)
	gridEqual(t, `section.height == 4`, section.Height, 4)
	children6 := unpack(t, section, 4)
	div7 = children6[0]
	div8 = children6[1]
	div9 := children6[2]
	div10 := children6[3]
	gridEqual(t, `div7.position_x == div9.position_x`, div7.PositionX, div9.PositionX)
	gridEqual(t, `div9.position_x == 0`, div9.PositionX, 0)
	gridEqual(t, `div8.position_x == div10.position_x`, div8.PositionX, div10.PositionX)
	gridEqual(t, `div10.position_x == 5`, div10.PositionX, 5)
	gridEqual(t, `div7.position_y == div8.position_y`, div7.PositionY, div8.PositionY)
	gridEqual(t, `div8.position_y == 0`, div8.PositionY, 0)
	gridEqual(t, `div9.position_y == div10.position_y`, div9.PositionY, div10.PositionY)
	gridEqual(t, `div10.position_y == 2`, div10.PositionY, 2)
	gridEqual(t, `div7.height == div8.height`, div7.Height, div8.Height)
	gridEqual(t, `div8.height == div9.height`, div8.Height, div9.Height)
	gridEqual(t, `div9.height == div10.height`, div9.Height, div10.Height)
	gridEqual(t, `div10.height == 2`, div10.Height, 2)
	line = only(t, div7)
	text = only(t, line)
	gridEqualString(t, `text.text == '7c'`, text.Text, `7c`)
	gridEqual(t, `not div8.children`, float64(len(div8.Children)), 0)
	line = only(t, div9)
	text = only(t, line)
	gridEqualString(t, `text.text == '9'`, text.Text, `9`)
	line = only(t, div10)
	text = only(t, line)
	gridEqualString(t, `text.text == '10'`, text.Text, `10`)
}

// TestLayoutGridBreakBorder is testGridBreakBorder of tests/layout/testGrid.py.
func TestLayoutGridBreakBorder(t *testing.T) {
	pages := gridRenderPages(t, `
      <style>
        @page { size: 12px 18px }
        body { font: 2px / 1 weasyprint }
        section { display: grid; grid-template-columns: 1fr 1fr }
        div { border: 1px solid }
      </style>
      <section>
        <div>1</div>
        <div>2</div>
        <div>3</div>
        <div>4</div>
        <div>5</div>
        <div>6</div>
        <div>7a<br>7b<br>7c</div>
        <div>8</div>
        <div>9</div>
        <div>10</div>
      </section>
    `, 2)
	page1 := pages[0]
	page2 := pages[1]
	html := only(t, page1)
	body := only(t, html)
	section := only(t, body)
	gridEqual(t, `section.height == 18`, section.Height, 18)
	children2 := unpack(t, section, 8)
	div1 := children2[0]
	div2 := children2[1]
	div3 := children2[2]
	div4 := children2[3]
	div5 := children2[4]
	div6 := children2[5]
	div7 := children2[6]
	div8 := children2[7]
	for _, div := range section.Children {
		gridEqual(t, `div.border_top_width == 1`, div.BorderTopWidth, 1)
		gridEqual(t, `div.border_left_width == 1`, div.BorderLeftWidth, 1)
		gridEqual(t, `div.border_right_width == 1`, div.BorderRightWidth, 1)
	}
	for _, div := range []*Box{div1, div2, div3, div4, div5, div6} {
		gridEqual(t, `div.border_bottom_width == 1`, div.BorderBottomWidth, 1)
	}
	for _, div := range []*Box{div7, div8} {
		gridEqual(t, `div.border_bottom_width == 0`, div.BorderBottomWidth, 0)
	}
	gridEqual(t, `div1.position_x == div3.position_x`, div1.PositionX, div3.PositionX)
	gridEqual(t, `div3.position_x == div5.position_x`, div3.PositionX, div5.PositionX)
	gridEqual(t, `div5.position_x == div7.position_x`, div5.PositionX, div7.PositionX)
	gridEqual(t, `div7.position_x == 0`, div7.PositionX, 0)
	gridEqual(t, `div2.position_x == div4.position_x`, div2.PositionX, div4.PositionX)
	gridEqual(t, `div4.position_x == div6.position_x`, div4.PositionX, div6.PositionX)
	gridEqual(t, `div6.position_x == div8.position_x`, div6.PositionX, div8.PositionX)
	gridEqual(t, `div8.position_x == 6`, div8.PositionX, 6)
	gridEqual(t, `div1.position_y == div2.position_y`, div1.PositionY, div2.PositionY)
	gridEqual(t, `div2.position_y == 0`, div2.PositionY, 0)
	gridEqual(t, `div1.height == div2.height`, div1.Height, div2.Height)
	gridEqual(t, `div2.height == 2`, div2.Height, 2)
	gridEqual(t, `div1.width == div2.width`, div1.Width, div2.Width)
	gridEqual(t, `div2.width == 4`, div2.Width, 4)
	gridEqual(t, `div3.position_y == div4.position_y`, div3.PositionY, div4.PositionY)
	gridEqual(t, `div4.position_y == 4`, div4.PositionY, 4)
	gridEqual(t, `div3.height == div4.height`, div3.Height, div4.Height)
	gridEqual(t, `div4.height == 2`, div4.Height, 2)
	gridEqual(t, `div5.position_y == div6.position_y`, div5.PositionY, div6.PositionY)
	gridEqual(t, `div6.position_y == 8`, div6.PositionY, 8)
	gridEqual(t, `div5.height == div6.height`, div5.Height, div6.Height)
	gridEqual(t, `div6.height == 2`, div6.Height, 2)
	gridEqual(t, `div7.position_y == div8.position_y`, div7.PositionY, div8.PositionY)
	gridEqual(t, `div8.position_y == 12`, div8.PositionY, 12)
	gridEqual(t, `div7.height == div8.height`, div7.Height, div8.Height)
	gridEqual(t, `div8.height == 5`, div8.Height, 5)
	children3 := unpack(t, div7, 2)
	line1 := children3[0]
	line2 := children3[1]
	children4 := unpack(t, line1, 2)
	text := children4[0]
	gridEqualString(t, `text.text == '7a'`, text.Text, `7a`)
	children5 := unpack(t, line2, 2)
	text = children5[0]
	gridEqualString(t, `text.text == '7b'`, text.Text, `7b`)
	line := only(t, div8)
	text = only(t, line)
	gridEqualString(t, `text.text == '8'`, text.Text, `8`)
	html = only(t, page2)
	body = only(t, html)
	section = only(t, body)
	// assert section.height == 7
	children6 := unpack(t, section, 4)
	div7 = children6[0]
	div8 = children6[1]
	div9 := children6[2]
	div10 := children6[3]
	for _, div := range section.Children {
		gridEqual(t, `div.border_bottom_width == 1`, div.BorderBottomWidth, 1)
		gridEqual(t, `div.border_left_width == 1`, div.BorderLeftWidth, 1)
		gridEqual(t, `div.border_right_width == 1`, div.BorderRightWidth, 1)
	}
	for _, div := range []*Box{div7, div8} {
		gridEqual(t, `div.border_top_width == 0`, div.BorderTopWidth, 0)
	}
	for _, div := range []*Box{div9, div10} {
		gridEqual(t, `div.border_top_width == 1`, div.BorderTopWidth, 1)
	}
	gridEqual(t, `div7.position_x == div9.position_x`, div7.PositionX, div9.PositionX)
	gridEqual(t, `div9.position_x == 0`, div9.PositionX, 0)
	gridEqual(t, `div8.position_x == div10.position_x`, div8.PositionX, div10.PositionX)
	gridEqual(t, `div10.position_x == 6`, div10.PositionX, 6)
	gridEqual(t, `div7.position_y == div8.position_y`, div7.PositionY, div8.PositionY)
	gridEqual(t, `div8.position_y == 0`, div8.PositionY, 0)
	gridEqual(t, `div9.position_y == div10.position_y`, div9.PositionY, div10.PositionY)
	gridEqual(t, `div10.position_y == 3`, div10.PositionY, 3)
	gridEqual(t, `div7.height == div8.height`, div7.Height, div8.Height)
	gridEqual(t, `div8.height == div9.height`, div8.Height, div9.Height)
	gridEqual(t, `div9.height == div10.height`, div9.Height, div10.Height)
	gridEqual(t, `div10.height == 2`, div10.Height, 2)
	gridEqual(t, `div7.width == div8.width`, div7.Width, div8.Width)
	gridEqual(t, `div8.width == div9.width`, div8.Width, div9.Width)
	gridEqual(t, `div9.width == div10.width`, div9.Width, div10.Width)
	gridEqual(t, `div10.width == 4`, div10.Width, 4)
	line = only(t, div7)
	text = only(t, line)
	gridEqualString(t, `text.text == '7c'`, text.Text, `7c`)
	gridEqual(t, `not div8.children`, float64(len(div8.Children)), 0)
}

// TestLayoutGridBreakMultiple is testGridBreakMultiple of tests/layout/testGrid.py.
func TestLayoutGridBreakMultiple(t *testing.T) {
	pages := gridRenderPages(t, `
      <style>
        @page { size: 10px 11px }
        body { font: 2px / 1 weasyprint }
        section { display: grid; grid-template-columns: 1fr 1fr }
      </style>
      <section>
        <div>1</div>
        <div>2</div>
        <div>3</div>
        <div>4</div>
        <div>5</div>
        <div>6</div>
        <div>7a<br>7b<br>7c<br>7d<br>7e<br>7f<br>7g<br>7h</div>
        <div>8</div>
        <div>9</div>
        <div>10</div>
      </section>
    `, 3)
	page1 := pages[0]
	page2 := pages[1]
	page3 := pages[2]
	html := only(t, page1)
	body := only(t, html)
	section := only(t, body)
	gridEqual(t, `section.height == 11`, section.Height, 11)
	children2 := unpack(t, section, 8)
	div1 := children2[0]
	div2 := children2[1]
	div3 := children2[2]
	div4 := children2[3]
	div5 := children2[4]
	div6 := children2[5]
	div7 := children2[6]
	div8 := children2[7]
	gridEqual(t, `div1.position_x == div3.position_x`, div1.PositionX, div3.PositionX)
	gridEqual(t, `div3.position_x == div5.position_x`, div3.PositionX, div5.PositionX)
	gridEqual(t, `div5.position_x == div7.position_x`, div5.PositionX, div7.PositionX)
	gridEqual(t, `div7.position_x == 0`, div7.PositionX, 0)
	gridEqual(t, `div2.position_x == div4.position_x`, div2.PositionX, div4.PositionX)
	gridEqual(t, `div4.position_x == div6.position_x`, div4.PositionX, div6.PositionX)
	gridEqual(t, `div6.position_x == div8.position_x`, div6.PositionX, div8.PositionX)
	gridEqual(t, `div8.position_x == 5`, div8.PositionX, 5)
	gridEqual(t, `div1.position_y == div2.position_y`, div1.PositionY, div2.PositionY)
	gridEqual(t, `div2.position_y == 0`, div2.PositionY, 0)
	gridEqual(t, `div1.height == div2.height`, div1.Height, div2.Height)
	gridEqual(t, `div2.height == 2`, div2.Height, 2)
	gridEqual(t, `div3.position_y == div4.position_y`, div3.PositionY, div4.PositionY)
	gridEqual(t, `div4.position_y == 2`, div4.PositionY, 2)
	gridEqual(t, `div3.height == div4.height`, div3.Height, div4.Height)
	gridEqual(t, `div4.height == 2`, div4.Height, 2)
	gridEqual(t, `div5.position_y == div6.position_y`, div5.PositionY, div6.PositionY)
	gridEqual(t, `div6.position_y == 4`, div6.PositionY, 4)
	gridEqual(t, `div5.height == div6.height`, div5.Height, div6.Height)
	gridEqual(t, `div6.height == 2`, div6.Height, 2)
	gridEqual(t, `div7.position_y == div8.position_y`, div7.PositionY, div8.PositionY)
	gridEqual(t, `div8.position_y == 6`, div8.PositionY, 6)
	gridEqual(t, `div7.height == div8.height`, div7.Height, div8.Height)
	gridEqual(t, `div8.height == 5`, div8.Height, 5)
	children3 := unpack(t, div7, 2)
	line1 := children3[0]
	line2 := children3[1]
	children4 := unpack(t, line1, 2)
	text := children4[0]
	gridEqualString(t, `text.text == '7a'`, text.Text, `7a`)
	children5 := unpack(t, line2, 2)
	text = children5[0]
	gridEqualString(t, `text.text == '7b'`, text.Text, `7b`)
	line := only(t, div8)
	text = only(t, line)
	gridEqualString(t, `text.text == '8'`, text.Text, `8`)
	html = only(t, page2)
	body = only(t, html)
	section = only(t, body)
	gridEqual(t, `section.height == 11`, section.Height, 11)
	children6 := unpack(t, section, 2)
	div7 = children6[0]
	div8 = children6[1]
	gridEqual(t, `div7.position_x == 0`, div7.PositionX, 0)
	gridEqual(t, `div8.position_x == 5`, div8.PositionX, 5)
	gridEqual(t, `div7.height == div8.height`, div7.Height, div8.Height)
	gridEqual(t, `div8.height == 11`, div8.Height, 11)
	children7 := unpack(t, div7, 5)
	line1 = children7[0]
	line2 = children7[1]
	line3 := children7[2]
	line4 := children7[3]
	line5 := children7[4]
	children8 := unpack(t, line1, 2)
	text = children8[0]
	gridEqualString(t, `text.text == '7c'`, text.Text, `7c`)
	children9 := unpack(t, line2, 2)
	text = children9[0]
	gridEqualString(t, `text.text == '7d'`, text.Text, `7d`)
	children10 := unpack(t, line3, 2)
	text = children10[0]
	gridEqualString(t, `text.text == '7e'`, text.Text, `7e`)
	children11 := unpack(t, line4, 2)
	text = children11[0]
	gridEqualString(t, `text.text == '7f'`, text.Text, `7f`)
	children12 := unpack(t, line5, 2)
	text = children12[0]
	gridEqualString(t, `text.text == '7g'`, text.Text, `7g`)
	gridEqual(t, `not div8.children`, float64(len(div8.Children)), 0)
	html = only(t, page3)
	body = only(t, html)
	section = only(t, body)
	gridEqual(t, `section.height == 4`, section.Height, 4)
	children13 := unpack(t, section, 4)
	div7 = children13[0]
	div8 = children13[1]
	div9 := children13[2]
	div10 := children13[3]
	gridEqual(t, `div7.position_x == div9.position_x`, div7.PositionX, div9.PositionX)
	gridEqual(t, `div9.position_x == 0`, div9.PositionX, 0)
	gridEqual(t, `div8.position_x == div10.position_x`, div8.PositionX, div10.PositionX)
	gridEqual(t, `div10.position_x == 5`, div10.PositionX, 5)
	gridEqual(t, `div7.position_y == div8.position_y`, div7.PositionY, div8.PositionY)
	gridEqual(t, `div8.position_y == 0`, div8.PositionY, 0)
	gridEqual(t, `div9.position_y == div10.position_y`, div9.PositionY, div10.PositionY)
	gridEqual(t, `div10.position_y == 2`, div10.PositionY, 2)
	gridEqual(t, `div7.height == div8.height`, div7.Height, div8.Height)
	gridEqual(t, `div8.height == div9.height`, div8.Height, div9.Height)
	gridEqual(t, `div9.height == div10.height`, div9.Height, div10.Height)
	gridEqual(t, `div10.height == 2`, div10.Height, 2)
	line = only(t, div7)
	text = only(t, line)
	gridEqualString(t, `text.text == '7h'`, text.Text, `7h`)
	gridEqual(t, `not div8.children`, float64(len(div8.Children)), 0)
}

// TestLayoutGridBreakSpan is testGridBreakSpan of tests/layout/testGrid.py.
func TestLayoutGridBreakSpan(t *testing.T) {
	pages := gridRenderPages(t, `
      <style>
        @page { size: 10px 11px }
        body { font: 2px / 1 weasyprint }
        section { display: grid; grid-template-columns: 1fr 1fr }
      </style>
      <section>
        <div>1</div>
        <div>2</div>
        <div>3</div>
        <div>4</div>
        <div>5</div>
        <div>6</div>
        <div style="grid-row: span 3">7a<br>7b<br>7c<br>7d</div>
        <div>8</div>
        <div>9</div>
        <div>10</div>
        <div>11</div>
        <div>12</div>
      </section>
    `, 2)
	page1 := pages[0]
	page2 := pages[1]
	html := only(t, page1)
	body := only(t, html)
	section := only(t, body)
	gridEqual(t, `section.height == 11`, section.Height, 11)
	children2 := unpack(t, section, 9)
	div1 := children2[0]
	div2 := children2[1]
	div3 := children2[2]
	div4 := children2[3]
	div5 := children2[4]
	div6 := children2[5]
	div7 := children2[6]
	div8 := children2[7]
	div9 := children2[8]
	gridEqual(t, `div1.position_x == div3.position_x`, div1.PositionX, div3.PositionX)
	gridEqual(t, `div3.position_x == div5.position_x`, div3.PositionX, div5.PositionX)
	gridEqual(t, `div5.position_x == div7.position_x`, div5.PositionX, div7.PositionX)
	gridEqual(t, `div7.position_x == 0`, div7.PositionX, 0)
	gridEqual(t, `div2.position_x == div4.position_x`, div2.PositionX, div4.PositionX)
	gridEqual(t, `div4.position_x == div6.position_x`, div4.PositionX, div6.PositionX)
	gridEqual(t, `div6.position_x == div8.position_x`, div6.PositionX, div8.PositionX)
	gridEqual(t, `div8.position_x == 5`, div8.PositionX, 5)
	gridEqual(t, `div1.position_y == div2.position_y`, div1.PositionY, div2.PositionY)
	gridEqual(t, `div2.position_y == 0`, div2.PositionY, 0)
	gridEqual(t, `div1.height == div2.height`, div1.Height, div2.Height)
	gridEqual(t, `div2.height == 2`, div2.Height, 2)
	gridEqual(t, `div3.position_y == div4.position_y`, div3.PositionY, div4.PositionY)
	gridEqual(t, `div4.position_y == 2`, div4.PositionY, 2)
	gridEqual(t, `div3.height == div4.height`, div3.Height, div4.Height)
	gridEqual(t, `div4.height == 2`, div4.Height, 2)
	gridEqual(t, `div5.position_y == div6.position_y`, div5.PositionY, div6.PositionY)
	gridEqual(t, `div6.position_y == 4`, div6.PositionY, 4)
	gridEqual(t, `div5.height == div6.height`, div5.Height, div6.Height)
	gridEqual(t, `div6.height == 2`, div6.Height, 2)
	gridEqual(t, `div7.position_y == div8.position_y`, div7.PositionY, div8.PositionY)
	gridEqual(t, `div8.position_y == 6`, div8.PositionY, 6)
	gridEqual(t, `div7.height == 5`, div7.Height, 5)
	gridEqual(t, `isclose(div8.height, 8 / 3)`, div8.Height, (8.0 / 3))
	gridEqual(t, `isclose(div9.height, 7 / 3)`, div9.Height, (7.0 / 3))
	children3 := unpack(t, div7, 2)
	line1 := children3[0]
	line2 := children3[1]
	children4 := unpack(t, line1, 2)
	text := children4[0]
	gridEqualString(t, `text.text == '7a'`, text.Text, `7a`)
	children5 := unpack(t, line2, 2)
	text = children5[0]
	gridEqualString(t, `text.text == '7b'`, text.Text, `7b`)
	line := only(t, div8)
	text = only(t, line)
	gridEqualString(t, `text.text == '8'`, text.Text, `8`)
	line = only(t, div9)
	text = only(t, line)
	gridEqualString(t, `text.text == '9'`, text.Text, `9`)
	html = only(t, page2)
	body = only(t, html)
	section = only(t, body)
	gridEqual(t, `isclose(section.height, 6)`, section.Height, 6)
	children6 := unpack(t, section, 5)
	div7 = children6[0]
	div9 = children6[1]
	div10 := children6[2]
	div11 := children6[3]
	div12 := children6[4]
	gridEqual(t, `div7.position_x == 0`, div7.PositionX, 0)
	gridEqual(t, `div8.position_x == 5`, div8.PositionX, 5)
	gridEqual(t, `div7.height == 4`, div7.Height, 4)
	children7 := unpack(t, div7, 2)
	line1 = children7[0]
	line2 = children7[1]
	children8 := unpack(t, line1, 2)
	text = children8[0]
	gridEqualString(t, `text.text == '7c'`, text.Text, `7c`)
	text = only(t, line2)
	gridEqualString(t, `text.text == '7d'`, text.Text, `7d`)
	gridEqual(t, `not div9.children`, float64(len(div9.Children)), 0)
	gridEqual(t, `div10.position_x == 5`, div10.PositionX, 5)
	gridEqual(t, `isclose(div10.position_y, 4 / 3)`, div10.PositionY, (4.0 / 3))
	line = only(t, div10)
	text = only(t, line)
	gridEqualString(t, `text.text == '10'`, text.Text, `10`)
	gridEqual(t, `div11.position_x == 0`, div11.PositionX, 0)
	gridEqual(t, `div12.position_x == 5`, div12.PositionX, 5)
	gridEqual(t, `div11.position_y == div12.position_y`, div11.PositionY, div12.PositionY)
	gridEqual(t, `isclose(div11.position_y, 4)`, div11.PositionY, 4)
	line = only(t, div11)
	text = only(t, line)
	gridEqualString(t, `text.text == '11'`, text.Text, `11`)
	line = only(t, div12)
	text = only(t, line)
	gridEqualString(t, `text.text == '12'`, text.Text, `12`)
}

// TestLayoutGridBreakItemAvoid is testGridBreakItemAvoid of tests/layout/testGrid.py.
func TestLayoutGridBreakItemAvoid(t *testing.T) {
	pages := gridRenderPages(t, `
      <style>
        @page { size: 10px 11px }
        body { font: 2px / 1 weasyprint }
        section { display: grid; grid-template-columns: 1fr 1fr }
      </style>
      <section>
        <div>1</div>
        <div>2</div>
        <div>3</div>
        <div>4</div>
        <div>5</div>
        <div>6</div>
        <div>7a<br>7b<br>7c</div>
        <div style="break-inside: avoid">8</div>
        <div>9</div>
        <div>10</div>
      </section>
    `, 2)
	page1 := pages[0]
	page2 := pages[1]
	html := only(t, page1)
	body := only(t, html)
	section := only(t, body)
	gridEqual(t, `section.height == 11`, section.Height, 11)
	children2 := unpack(t, section, 6)
	div1 := children2[0]
	div2 := children2[1]
	div3 := children2[2]
	div4 := children2[3]
	div5 := children2[4]
	div6 := children2[5]
	gridEqual(t, `div1.position_x == div3.position_x`, div1.PositionX, div3.PositionX)
	gridEqual(t, `div3.position_x == div5.position_x`, div3.PositionX, div5.PositionX)
	gridEqual(t, `div5.position_x == 0`, div5.PositionX, 0)
	gridEqual(t, `div2.position_x == div4.position_x`, div2.PositionX, div4.PositionX)
	gridEqual(t, `div4.position_x == div6.position_x`, div4.PositionX, div6.PositionX)
	gridEqual(t, `div6.position_x == 5`, div6.PositionX, 5)
	gridEqual(t, `div1.position_y == div2.position_y`, div1.PositionY, div2.PositionY)
	gridEqual(t, `div2.position_y == 0`, div2.PositionY, 0)
	gridEqual(t, `div1.height == div2.height`, div1.Height, div2.Height)
	gridEqual(t, `div2.height == 2`, div2.Height, 2)
	gridEqual(t, `div3.position_y == div4.position_y`, div3.PositionY, div4.PositionY)
	gridEqual(t, `div4.position_y == 2`, div4.PositionY, 2)
	gridEqual(t, `div3.height == div4.height`, div3.Height, div4.Height)
	gridEqual(t, `div4.height == 2`, div4.Height, 2)
	gridEqual(t, `div5.position_y == div6.position_y`, div5.PositionY, div6.PositionY)
	gridEqual(t, `div6.position_y == 4`, div6.PositionY, 4)
	gridEqual(t, `div5.height == div6.height`, div5.Height, div6.Height)
	gridEqual(t, `div6.height == 2`, div6.Height, 2)
	html = only(t, page2)
	body = only(t, html)
	section = only(t, body)
	gridEqual(t, `section.height == 8`, section.Height, 8)
	children3 := unpack(t, section, 4)
	div7 := children3[0]
	div8 := children3[1]
	div9 := children3[2]
	div10 := children3[3]
	gridEqual(t, `div7.position_x == div9.position_x`, div7.PositionX, div9.PositionX)
	gridEqual(t, `div9.position_x == 0`, div9.PositionX, 0)
	gridEqual(t, `div8.position_x == div10.position_x`, div8.PositionX, div10.PositionX)
	gridEqual(t, `div10.position_x == 5`, div10.PositionX, 5)
	gridEqual(t, `div7.position_y == div8.position_y`, div7.PositionY, div8.PositionY)
	gridEqual(t, `div8.position_y == 0`, div8.PositionY, 0)
	gridEqual(t, `div9.position_y == div10.position_y`, div9.PositionY, div10.PositionY)
	gridEqual(t, `div10.position_y == 6`, div10.PositionY, 6)
	gridEqual(t, `div7.height == div8.height`, div7.Height, div8.Height)
	gridEqual(t, `div8.height == 6`, div8.Height, 6)
	gridEqual(t, `div9.height == div10.height`, div9.Height, div10.Height)
	gridEqual(t, `div10.height == 2`, div10.Height, 2)
	gridEqual(t, `len(div7.children) == 3`, float64(len(div7.Children)), 3)
	gridEqual(t, `len(div8.children) == 1`, float64(len(div8.Children)), 1)
}

// TestLayoutGridBreakItemAvoidAuto is testGridBreakItemAvoidAuto of tests/layout/testGrid.py.
func TestLayoutGridBreakItemAvoidAuto(t *testing.T) {
	pages := gridRenderPages(t, `
      <style>
        @page { size: 10px 11px }
        body { font: 2px / 1 weasyprint }
        section { display: grid; grid-template-columns: 1fr 1fr }
      </style>
      <section>
        <div>1</div>
        <div>2</div>
        <div>3</div>
        <div>4</div>
        <div>5</div>
        <div>6</div>
        <div style="break-inside: avoid">7a<br>7b<br>7c</div>
        <div style="break-inside: auto">8</div>
        <div>9</div>
        <div>10</div>
      </section>
    `, 2)
	page1 := pages[0]
	page2 := pages[1]
	html := only(t, page1)
	body := only(t, html)
	section := only(t, body)
	gridEqual(t, `section.height == 11`, section.Height, 11)
	children2 := unpack(t, section, 6)
	div1 := children2[0]
	div2 := children2[1]
	div3 := children2[2]
	div4 := children2[3]
	div5 := children2[4]
	div6 := children2[5]
	gridEqual(t, `div1.position_x == div3.position_x`, div1.PositionX, div3.PositionX)
	gridEqual(t, `div3.position_x == div5.position_x`, div3.PositionX, div5.PositionX)
	gridEqual(t, `div5.position_x == 0`, div5.PositionX, 0)
	gridEqual(t, `div2.position_x == div4.position_x`, div2.PositionX, div4.PositionX)
	gridEqual(t, `div4.position_x == div6.position_x`, div4.PositionX, div6.PositionX)
	gridEqual(t, `div6.position_x == 5`, div6.PositionX, 5)
	gridEqual(t, `div1.position_y == div2.position_y`, div1.PositionY, div2.PositionY)
	gridEqual(t, `div2.position_y == 0`, div2.PositionY, 0)
	gridEqual(t, `div1.height == div2.height`, div1.Height, div2.Height)
	gridEqual(t, `div2.height == 2`, div2.Height, 2)
	gridEqual(t, `div3.position_y == div4.position_y`, div3.PositionY, div4.PositionY)
	gridEqual(t, `div4.position_y == 2`, div4.PositionY, 2)
	gridEqual(t, `div3.height == div4.height`, div3.Height, div4.Height)
	gridEqual(t, `div4.height == 2`, div4.Height, 2)
	gridEqual(t, `div5.position_y == div6.position_y`, div5.PositionY, div6.PositionY)
	gridEqual(t, `div6.position_y == 4`, div6.PositionY, 4)
	gridEqual(t, `div5.height == div6.height`, div5.Height, div6.Height)
	gridEqual(t, `div6.height == 2`, div6.Height, 2)
	html = only(t, page2)
	body = only(t, html)
	section = only(t, body)
	gridEqual(t, `section.height == 8`, section.Height, 8)
	children3 := unpack(t, section, 4)
	div7 := children3[0]
	div8 := children3[1]
	div9 := children3[2]
	div10 := children3[3]
	gridEqual(t, `div7.position_x == div9.position_x`, div7.PositionX, div9.PositionX)
	gridEqual(t, `div9.position_x == 0`, div9.PositionX, 0)
	gridEqual(t, `div8.position_x == div10.position_x`, div8.PositionX, div10.PositionX)
	gridEqual(t, `div10.position_x == 5`, div10.PositionX, 5)
	gridEqual(t, `div7.position_y == div8.position_y`, div7.PositionY, div8.PositionY)
	gridEqual(t, `div8.position_y == 0`, div8.PositionY, 0)
	gridEqual(t, `div9.position_y == div10.position_y`, div9.PositionY, div10.PositionY)
	gridEqual(t, `div10.position_y == 6`, div10.PositionY, 6)
	gridEqual(t, `div7.height == div8.height`, div7.Height, div8.Height)
	gridEqual(t, `div8.height == 6`, div8.Height, 6)
	gridEqual(t, `div9.height == div10.height`, div9.Height, div10.Height)
	gridEqual(t, `div10.height == 2`, div10.Height, 2)
	gridEqual(t, `len(div7.children) == 3`, float64(len(div7.Children)), 3)
	gridEqual(t, `len(div8.children) == 1`, float64(len(div8.Children)), 1)
}

// TestLayoutGridBreakContainer is testGridBreakContainer of tests/layout/testGrid.py.
func TestLayoutGridBreakContainer(t *testing.T) {
	pages := gridRenderPages(t, `
      <style>
        @page { size: 10px 11px }
        body { font: 2px / 1 weasyprint }
        section { display: grid; grid-template-columns: 1fr 1fr }
      </style>
      <p>1<br>2<br>3<br>4</p>
      <section style="break-inside: avoid">
        <div>5</div>
        <div>6</div>
        <div>7</div>
        <div>8</div>
        <div>9</div>
        <div>10</div>
      </section>
    `, 2)
	page1 := pages[0]
	page2 := pages[1]
	html := only(t, page1)
	body := only(t, html)
	p := only(t, body)
	gridEqual(t, `len(p.children) == 4`, float64(len(p.Children)), 4)
	html = only(t, page2)
	body = only(t, html)
	section := only(t, body)
	gridEqual(t, `len(section.children) == 6`, float64(len(section.Children)), 6)
}

// TestLayoutGridBottomPage is testGridBottomPage of tests/layout/testGrid.py.
func TestLayoutGridBottomPage(t *testing.T) {
	pages := gridRenderPages(t, `
      <style>
        @page { size: 10px }
        body { font: 2px / 1 weasyprint }
        section { display: grid; grid-template-columns: 1fr 1fr }
      </style>
      <div style="height: 9px"></div>
      <section>
        <div>1</div>
        <div>2</div>
      </section>
    `, 2)
	page1 := pages[0]
	page2 := pages[1]
	html := only(t, page1)
	body := only(t, html)
	_ = only(t, body)
	html = only(t, page2)
	body = only(t, html)
	_ = only(t, body)
}

// TestLayoutGridInGrid is testGridInGrid of tests/layout/testGrid.py.
func TestLayoutGridInGrid(t *testing.T) {
	pages := gridRenderPages(t, `
      <style>
        @page { size: 10px }
        body { font: 2px / 1 weasyprint }
        section { display: grid; grid-template-columns: 1fr 1fr }
      </style>
      <div style="height: 7px"></div>
      <section>
        <section>
          <div>1 2</div>
          <div>3 4</div>
        </section>
        <section>
          <div>5 6</div>
          <div>7 8</div>
        </section>
      </section>
    `, 2)
	page1 := pages[0]
	page2 := pages[1]
	html := only(t, page1)
	body := only(t, html)
	children2 := unpack(t, body, 2)
	section := children2[1]
	children3 := unpack(t, section, 2)
	subsection1 := children3[0]
	subsection2 := children3[1]
	children4 := unpack(t, subsection1, 2)
	div1 := children4[0]
	div2 := children4[1]
	gridEqualString(t, `div1.children[0].children[0].text == '1'`, div1.Children[0].Children[0].Text, `1`)
	gridEqualString(t, `div2.children[0].children[0].text == '3'`, div2.Children[0].Children[0].Text, `3`)
	children5 := unpack(t, subsection2, 2)
	div3 := children5[0]
	div4 := children5[1]
	gridEqualString(t, `div3.children[0].children[0].text == '5'`, div3.Children[0].Children[0].Text, `5`)
	gridEqualString(t, `div4.children[0].children[0].text == '7'`, div4.Children[0].Children[0].Text, `7`)
	html = only(t, page2)
	body = only(t, html)
	section = only(t, body)
	children6 := unpack(t, section, 2)
	subsection1 = children6[0]
	subsection2 = children6[1]
	children7 := unpack(t, subsection1, 2)
	div1 = children7[0]
	div2 = children7[1]
	gridEqualString(t, `div1.children[0].children[0].text == '2'`, div1.Children[0].Children[0].Text, `2`)
	gridEqualString(t, `div2.children[0].children[0].text == '4'`, div2.Children[0].Children[0].Text, `4`)
	children8 := unpack(t, subsection2, 2)
	div3 = children8[0]
	div4 = children8[1]
	gridEqualString(t, `div3.children[0].children[0].text == '6'`, div3.Children[0].Children[0].Text, `6`)
	gridEqualString(t, `div4.children[0].children[0].text == '8'`, div4.Children[0].Children[0].Text, `8`)
}

// TestLayoutGridInFlexAfterFullHeight is testGridInFlexAfterFullHeight of tests/layout/testGrid.py.
func TestLayoutGridInFlexAfterFullHeight(t *testing.T) {
	pages := gridRenderPages(t, `
      <style>
        @page { size: 10px }
      </style>
      <div style="height: 15px"></div>
      <div style="display: flex; flex-direction: column">
        <div style="display: grid">
          <div></div>
        </div>
      </div>
    `, 2)
	page1 := pages[0]
	page2 := pages[1]
	html := only(t, page1)
	body := only(t, html)
	div := only(t, body)
	gridEqual(t, `not div.children`, float64(len(div.Children)), 0)
	html = only(t, page2)
	body = only(t, html)
	flex := only(t, body)
	grid := only(t, flex)
	gridItem := only(t, grid)
	gridEqual(t, `not grid_item.children`, float64(len(gridItem.Children)), 0)
}

// TestLayoutGridInColumns is testGridInColumns of tests/layout/testGrid.py.
func TestLayoutGridInColumns(t *testing.T) {
	pages := gridRenderPages(t, `
      <style>
        @page { size: 12cm }
        section { height: 10cm }
      </style>
      <div style="columns: 2">
        <div style="display: grid">
          <div>
            <section></section>
            <section></section>
            <section></section>
          </div>
        </div>
      </div>
    `, 2)
	page1 := pages[0]
	page2 := pages[1]
	html := only(t, page1)
	body := only(t, html)
	div := only(t, body)
	children2 := unpack(t, div, 2)
	column1 := children2[0]
	column2 := children2[1]
	grid := only(t, column1)
	gridEqual(t, `len(grid.children) == 1`, float64(len(grid.Children)), 1)
	grid = only(t, column2)
	gridEqual(t, `len(grid.children) == 1`, float64(len(grid.Children)), 1)
	html = only(t, page2)
	body = only(t, html)
	div = only(t, body)
	children3 := unpack(t, div, 2)
	column1 = children3[0]
	column2 = children3[1]
	grid = only(t, column1)
	gridEqual(t, `len(grid.children) == 1`, float64(len(grid.Children)), 1)
	grid = only(t, column2)
	gridEqual(t, `len(grid.children) == 0`, float64(len(grid.Children)), 0)
}

// TestLayoutGridInColumnsWithBreak is testGridInColumnsWithBreak of tests/layout/testGrid.py.
func TestLayoutGridInColumnsWithBreak(t *testing.T) {
	t.Skip("upstream marks it as an expected failure")
	pages := gridRenderPages(t, `
      <style>
        @page { size: 12px }
        section { height: 4px }
      </style>
      <div style="columns: 1">
        <div style="display: grid; margin-top: 6px">
          <div>
            <section></section>
            <section></section>
            <section></section>
          </div>
        </div>
      </div>
    `, 2)
	page1 := pages[0]
	page2 := pages[1]
	html := only(t, page1)
	body := only(t, html)
	div := only(t, body)
	column1 := only(t, div)
	grid := only(t, column1)
	section1 := only(t, grid)
	gridEqual(t, `section1.position_y == 6`, section1.PositionY, 6)
	html = only(t, page2)
	body = only(t, html)
	div = only(t, body)
	column1 = only(t, div)
	grid = only(t, column1)
	children2 := unpack(t, grid, 2)
	section2 := children2[0]
	section3 := children2[1]
	gridEqual(t, `section2.position_y == 0`, section2.PositionY, 0)
	gridEqual(t, `section3.position_y == 4`, section3.PositionY, 4)
}

// TestLayoutGridMinmaxYSpan is testGridMinmaxYSpan of tests/layout/testGrid.py.
func TestLayoutGridMinmaxYSpan(t *testing.T) {
	pages := gridRenderPages(t, `
      <style>
        @page { size: 20px }
        body { font: 2px / 1 weasyprint }
        article { display: grid; grid-template-columns: 5px 5px;
                  grid-auto-rows: minmax(1px, auto) }
      </style>
      <article>
        <div style="grid-row: span 2">a<br>b<br>c<br>d</div>
        <div>e</div>
        <div>f</div>
        <div style="grid-column: 1">g</div>
      </article>
    `, 1)
	page := pages[0]
	html := only(t, page)
	body := only(t, html)
	article := only(t, body)
	divG := gridLast(t, article)
	gridEqual(t, `div_g.position_y == 8`, divG.PositionY, 8)
}

// TestLayoutGridAbsolutePageBreakAvoid is testGridAbsolutePageBreakAvoid of tests/layout/testGrid.py.
func TestLayoutGridAbsolutePageBreakAvoid(t *testing.T) {
	pages := gridRenderPages(t, `
      <style>
        @page { size: 5px }
        body { font: 2px / 1 weasyprint }
      </style>
      <article style="display: grid; position: absolute; break-inside: avoid">
        <div>a</div><div>b</div><div>c</div>
      </article>
    `, 1)
	page := pages[0]
	html := only(t, page)
	body := only(t, html)
	article := only(t, body)
	gridEqual(t, `article.position_x == 0`, article.PositionX, 0)
	gridEqual(t, `article.position_y == 0`, article.PositionY, 0)
	gridEqual(t, `article.height == 5`, article.Height, 5)
	gridEqual(t, `len(article.children) == 2`, float64(len(article.Children)), 2)
}

// TestLayoutGridSplitFloatChild is testGridSplitFloatChild of tests/layout/testGrid.py.
func TestLayoutGridSplitFloatChild(t *testing.T) {
	pages := gridRenderPages(t, `
      <style>
        @page { size: 4px 5px }
        body { font: 2px / 1 weasyprint }
      </style>
      <div>a</div>
      <article style="display: grid; grid-template-columns: 2px 2px">
        <div><span style="float: right">b</span></div>
        <div style="break-inside: avoid">c<br>c</div>
      </article>
    `, 2)
	page1 := pages[0]
	page2 := pages[1]
	html := only(t, page1)
	body := only(t, html)
	_ = only(t, body)
	html = only(t, page2)
	body = only(t, html)
	article := only(t, body)
	gridEqual(t, `article.position_x == 0`, article.PositionX, 0)
	gridEqual(t, `article.position_y == 0`, article.PositionY, 0)
	gridEqual(t, `article.height == 4`, article.Height, 4)
	unpack(t, article, 2)
}

// TestLayoutGridMinmaxX is testGridMinmaxX of tests/layout/testGrid.py.
func TestLayoutGridMinmaxX(t *testing.T) {
	for _, c := range []struct {
		value string
		width float64
	}{
		{"minmax(auto, auto)", 20},
		{"minmax(0, auto)", 20},
		{"minmax(1px, auto)", 20},
		{"minmax(10px, auto)", 20},
		{"minmax(auto, 10px)", 10},
		{"minmax(auto, 30px)", 20},
		{"minmax(10px, 10px)", 10},
		{"minmax(30px, 30px)", 30},
		{"1px", 1},
		{"30px", 30},
		{"minmax(0, 1px)", 1},
	} {
		t.Run(c.value, func(t *testing.T) {
			pages := gridRenderPages(t, fmt.Sprintf(`
      <style>
        @page { size: 20px }
        body { font: 2px / 1 weasyprint }
        article { display: grid; grid-auto-columns: %s }
      </style>
      <article style="display: grid">
        <div>ab</div>
      </article>
    `, c.value), 1)
			_, body := pageBody(t, pages[0])
			article := only(t, body)
			div := only(t, article)
			gridEqual(t, "div.width", div.Width, c.width)
		})
	}
}

// TestLayoutGridMinmaxY is testGridMinmaxY of tests/layout/testGrid.py.
func TestLayoutGridMinmaxY(t *testing.T) {
	for _, c := range []struct {
		value  string
		height float64
	}{
		{"minmax(auto, auto)", 2},
		{"minmax(0, auto)", 2},
		{"minmax(1px, auto)", 2},
		{"minmax(10px, auto)", 10},
		{"minmax(auto, 1px)", 1},
		{"minmax(auto, 10px)", 10},
		{"minmax(1px, 1px)", 1},
		{"minmax(10px, 10px)", 10},
		{"1px", 1},
		{"10px", 10},
		{"minmax(0, 1px)", 1},
	} {
		t.Run(c.value, func(t *testing.T) {
			pages := gridRenderPages(t, fmt.Sprintf(`
      <style>
        @page { size: 20px }
        body { font: 2px / 1 weasyprint }
        article { display: grid; grid-auto-rows: %s }
      </style>
      <article style="display: grid">
        <div>ab</div>
      </article>
    `, c.value), 1)
			_, body := pageBody(t, pages[0])
			article := only(t, body)
			div := only(t, article)
			gridEqual(t, "div.height", div.Height, c.height)
		})
	}
}

// TestLayoutGridAutoFillColumns is testGridAutoFillColumns of
// tests/layout/testGrid.py.
func TestLayoutGridAutoFillColumns(t *testing.T) {
	for _, c := range []struct {
		columns string
		gap     int
		x       []float64
	}{
		{"repeat(auto-fill, 3px)", 0, []float64{0, 3, 6, 0, 3, 6}},
		{"repeat(auto-fill, 3px)", 1, []float64{0, 4, 0, 4, 0, 4}},
		{"1px repeat(auto-fill, 3px)", 0, []float64{0, 1, 4, 7, 0, 1}},
		{"repeat(auto-fill, 3px 2px)", 0, []float64{0, 3, 5, 8, 0, 3}},
		{"repeat(auto-fill, minmax(2px, 1fr))", 0, []float64{0, 2, 4, 6, 8, 0}},
		{"repeat(auto-fill, 20px)", 0, []float64{0, 0, 0, 0, 0, 0}},
		{"repeat(auto-fill, minmax(4px, 3px))", 0, []float64{0, 4, 0, 4, 0, 4}},
	} {
		t.Run(fmt.Sprintf("%s gap %d", c.columns, c.gap), func(t *testing.T) {
			pages := gridRenderPages(t, fmt.Sprintf(`
      <style>
        @page { size: 20px }
        body { font: 2px / 1 weasyprint }
        article { display: grid; width: 10px; column-gap: %dpx;
                  grid-template-columns: %s }
      </style>
      <article>
        <div>a</div><div>b</div><div>c</div>
        <div>d</div><div>e</div><div>f</div>
      </article>
    `, c.gap, c.columns), 1)
			_, body := pageBody(t, pages[0])
			article := only(t, body)
			divs := unpack(t, article, len(c.x))
			for i, div := range divs {
				gridEqual(t, fmt.Sprintf("div %d position_x", i), div.PositionX, c.x[i])
			}
		})
	}
}

// TestLayoutGridAutoFillRows is testGridAutoFillRows of
// tests/layout/testGrid.py.
func TestLayoutGridAutoFillRows(t *testing.T) {
	for _, c := range []struct {
		height string
		y      []float64
	}{
		{"10px", []float64{0, 3, 6, 0, 3, 6}},
		{"auto", []float64{0, 0, 0, 0, 0, 0}},
	} {
		t.Run(c.height, func(t *testing.T) {
			pages := gridRenderPages(t, fmt.Sprintf(`
      <style>
        @page { size: 20px }
        body { font: 2px / 1 weasyprint }
        article { display: grid; height: %s; grid-auto-flow: column;
                  grid-template-rows: repeat(auto-fill, 3px) }
      </style>
      <article>
        <div>a</div><div>b</div><div>c</div>
        <div>d</div><div>e</div><div>f</div>
      </article>
    `, c.height), 1)
			_, body := pageBody(t, pages[0])
			article := only(t, body)
			divs := unpack(t, article, len(c.y))
			for i, div := range divs {
				gridEqual(t, fmt.Sprintf("div %d position_y", i), div.PositionY, c.y[i])
			}
		})
	}
}

// TestGridSpanStartDefiniteColumn tests two items that span two rows in
// the same column. Upstream fails on the first one, and searches without
// end for the second one. See the Port: comment of gridLayout.
func TestGridSpanStartDefiniteColumn(t *testing.T) {
	pages := gridRenderPages(t, `
      <style>
        body { font: 2px / 1 weasyprint }
        article { display: grid; grid-template-columns: 5px 5px; width: 10px }
        div { grid-row: span 2; grid-column: 2 }
      </style>
      <article>
        <div>a</div>
        <div>b</div>
      </article>
    `, 1)
	_, body := pageBody(t, pages[0])
	article := only(t, body)
	children := unpack(t, article, 2)
	gridEqual(t, "div_a.position_x", children[0].PositionX, 5)
	gridEqual(t, "div_b.position_x", children[1].PositionX, 5)
	gridEqual(t, "div_a.position_y", children[0].PositionY, 0)
	gridEqual(t, "div_b.position_y", children[1].PositionY, 2)
}

// TestGridHostile lays out grids of hostile styles. Each one must end,
// without a panic, with its pages or with an error that wraps ivy.ErrLimit.
func TestGridHostile(t *testing.T) {
	for _, style := range []string{
		"grid-template-columns: repeat(1000000000, 1px)",
		"grid-template-columns: repeat(auto-fill, 0.0001px)",
		"grid-auto-flow: dense; grid-template-columns: repeat(50, 1px)",
		"grid-template-areas: 'a b' 'c d'; grid-auto-columns: ",
	} {
		for _, item := range []string{
			"grid-row: 1000000",
			"grid-row: -1000000 / span 1000000",
			"grid-column: span 7 x / 3",
			"grid-row: span 3 foo; grid-column: 2",
			"grid-area: a / b / c / d",
		} {
			t.Run(style+"; "+item, func(t *testing.T) {
				_, err := renderPagesErr(t, fmt.Sprintf(`
      <style>article { display: grid; %s } div { %s }</style>
      <article><div>a</div><div>b</div><div style="grid-row: span 2">c</div></article>
    `, style, item), testBaseURL, nil)
				if err != nil && !errors.Is(err, ivy.ErrLimit) {
					t.Fatal(err)
				}
			})
		}
	}
}

// TestGridGetPlacement tests the placement of the lines of an item in a
// grid of three tracks.
func TestGridGetPlacement(t *testing.T) {
	lines := [][]string{{"a"}, nil, {"b"}, nil}
	auto := gridPlaceLine{auto: true}
	for _, c := range []struct {
		name        string
		start, end  gridPlaceLine
		coord, size int
		ok          bool
	}{
		{"auto", auto, auto, 0, 0, false},
		{"number", gridNumberLine(2), auto, 1, 1, true},
		{"numbers", gridNumberLine(1), gridNumberLine(3), 0, 2, true},
		{"reversed", gridNumberLine(3), gridNumberLine(1), 0, 2, true},
		{"span", gridPlaceLine{span: true, number: 2, hasNumber: true}, gridNumberLine(4), 1, 2, true},
		{"names", gridPlaceLine{ident: "a"}, gridPlaceLine{ident: "b"}, 0, 2, true},
		{"negative", gridNumberLine(-1), auto, -2, 1, true},
	} {
		coord, size, ok := gridGetPlacement(c.start, c.end, lines)
		if coord != c.coord || size != c.size || ok != c.ok {
			t.Errorf("%s: got (%d, %d, %t), want (%d, %d, %t)", c.name, coord, size, ok, c.coord, c.size, c.ok)
		}
	}
}

// TestGridGetSizingFunctions tests the sizing functions of the track
// sizes.
func TestGridGetSizingFunctions(t *testing.T) {
	fr := Dimension{Value: 1, Unit: "fr"}
	px := Dimension{Value: 10, Unit: "px"}
	for _, c := range []struct {
		size     Value
		min, max Value
	}{
		{String("auto"), String("auto"), String("auto")},
		{fr, String("auto"), fr},
		{Tuple{String("minmax()"), px, fr}, px, fr},
		{Tuple{String("fit-content()"), px}, String("auto"), Tuple{String("fit-content()"), px}},
	} {
		got := gridGetSizingFunctions(c.size)
		if !Equal(got[0], c.min) || !Equal(got[1], c.max) {
			t.Errorf("%v: got %v, want (%v, %v)", c.size, got, c.min, c.max)
		}
	}
}
