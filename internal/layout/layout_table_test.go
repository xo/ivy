// Copyright (c) 2011-2021, Simon Sapin and contributors.
//
// Use of this source code is governed by the BSD-3-Clause license that can be
// found in the LICENSE file.

// This file ports the first half of tests/layout/test_table.py: the tests
// of the layout of the tables. layout_table2_test.go holds the second half.

package layout

import (
	"fmt"
	"log/slog"
	"math"
	"strings"
	"testing"
)

// tablePages lays out a document, fails the test for each logged warning
// or error, and returns the pages. It is render_pages in a test decorated
// with assert_no_logs.
func tablePages(t *testing.T, source string) []*Box {
	t.Helper()
	log, c := captureLogs(slog.LevelWarn)
	pages := renderPagesLog(t, source, testBaseURL, log)
	assertNoLogs(t, c)
	return pages
}

// tablePage lays out a document of one page and returns the page, as
// `page, = render_pages(...)`.
func tablePage(t *testing.T, source string) *Box {
	t.Helper()
	pages := tablePages(t, source)
	if len(pages) != 1 {
		t.Fatalf("got %d pages, want 1", len(pages))
	}
	return pages[0]
}

// tableRender lays out a document of one page whose body holds only a table,
// and returns the table wrapper and the table.
func tableRender(t *testing.T, source string) (wrapper, table *Box) {
	t.Helper()
	_, body := pageBody(t, tablePage(t, source))
	wrapper = only(t, body)
	return wrapper, only(t, wrapper)
}

// tableEq fails the test when got is not want. It is the `==` of the tests
// of upstream, with the tolerance of assertApprox.
func tableEq(t *testing.T, name string, got, want float64) {
	t.Helper()
	assertApprox(t, name, got, want)
}

// tableFloats fails the test when the lists are not equal.
func tableFloats(t *testing.T, name string, got, want []float64) {
	t.Helper()
	if len(got) != len(want) {
		t.Errorf("%s = %v, want %v", name, got, want)
		return
	}
	for i := range got {
		assertApprox(t, name, got[i], want[i])
	}
}

func TestInlineTable(t *testing.T) {
	page := tablePage(t, `
      <table style="display: inline-table; border-spacing: 10px; margin: 5px">
        <tr>
          <td><img src=pattern.png style="width: 20px"></td>
          <td><img src=pattern.png style="width: 30px"></td>
        </tr>
      </table>
      foo
    `)
	_, body := pageBody(t, page)
	line := only(t, body)
	c := unpack(t, line, 2)
	tableWrapper, text := c[0], c[1]
	table := only(t, tableWrapper)
	rowGroup := only(t, table)
	row := only(t, rowGroup)
	td := unpack(t, row, 2)
	tableEq(t, "table_wrapper.position_x", tableWrapper.PositionX, 0)
	tableEq(t, "table.position_x", table.PositionX, 5) // 0 + margin-left
	tableEq(t, "td_1.position_x", td[0].PositionX, 15) // 0 + border-spacing
	tableEq(t, "td_1.width", td[0].Width, 20)
	tableEq(t, "td_2.position_x", td[1].PositionX, 45) // 15 + 20 + border-spacing
	tableEq(t, "td_2.width", td[1].Width, 30)
	tableEq(t, "table.width", table.Width, 80)                                 // 20 + 30 + 3 * border-spacing
	tableEq(t, "table_wrapper.margin_width()", tableWrapper.MarginWidth(), 90) // 80 + 2 * margin
	tableEq(t, "text.position_x", text.PositionX, 90)
}

func TestInlineTableWidth(t *testing.T) {
	page := tablePage(t, `
      <div style="font: 2px weasyprint; width: 20px">
        <table style="display: inline-table; width: 20%"><tr><td>A</tr></td></table>
        <table style="display: inline-table; width: 20%"><tr><td>B</tr></td></table>
      </div>`)
	_, body := pageBody(t, page)
	div := only(t, body)
	line := only(t, div)
	c := unpack(t, line, 4)
	tableWrapper1, tableWrapper2 := c[0], c[2]
	tableEq(t, "table_wrapper1.width", tableWrapper1.Width, 4)
	tableEq(t, "table_wrapper2.width", tableWrapper2.Width, 4)
	tableEq(t, "table_wrapper1.position_x", tableWrapper1.PositionX, 0)
	tableEq(t, "table_wrapper2.position_x", tableWrapper2.PositionX, 6)
}

func TestImplicitWidthTableColPercent(t *testing.T) {
	// Regression test for #169.
	_, table := tableRender(t, `
      <table>
        <col style="width:25%"></col>
        <col></col>
        <tr>
          <td></td>
          <td></td>
        </tr>
      </table>
    `)
	rowGroup := only(t, table)
	row := only(t, rowGroup)
	unpack(t, row, 2)
}

func TestImplicitWidthTableTdPercent(t *testing.T) {
	_, table := tableRender(t, `
      <table>
        <tr>
          <td style="width:25%"></td>
          <td></td>
        </tr>
      </table>
    `)
	rowGroup := only(t, table)
	row := only(t, rowGroup)
	unpack(t, row, 2)
}

func TestLayoutTableFixed1(t *testing.T) {
	tableWrapper, table := tableRender(t, `
      <table style="table-layout: fixed; border-spacing: 10px; margin: 5px">
        <colgroup>
          <col style="width: 20px" />
        </colgroup>
        <tr>
          <td></td>
          <td style="width: 40px">a</td>
        </tr>
      </table>
    `)
	td := unpack(t, only(t, only(t, table)), 2)
	tableEq(t, "table_wrapper.position_x", tableWrapper.PositionX, 0)
	tableEq(t, "table.position_x", table.PositionX, 5) // 0 + margin-left
	tableEq(t, "td_1.position_x", td[0].PositionX, 15) // 5 + border-spacing
	tableEq(t, "td_1.width", td[0].Width, 20)
	tableEq(t, "td_2.position_x", td[1].PositionX, 45) // 15 + 20 + border-spacing
	tableEq(t, "td_2.width", td[1].Width, 40)
	tableEq(t, "table.width", table.Width, 90) // 20 + 40 + 3 * border-spacing
}

func TestLayoutTableFixed2(t *testing.T) {
	tableWrapper, table := tableRender(t, `
      <table style="table-layout: fixed; border-spacing: 10px; width: 200px;
                    margin: 5px">
        <tr>
          <td style="width: 20px">a</td>
          <td style="width: 40px"></td>
        </tr>
      </table>
    `)
	td := unpack(t, only(t, only(t, table)), 2)
	tableEq(t, "table_wrapper.position_x", tableWrapper.PositionX, 0)
	tableEq(t, "table.position_x", table.PositionX, 5)  // 0 + margin-left
	tableEq(t, "td_1.position_x", td[0].PositionX, 15)  // 5 + border-spacing
	tableEq(t, "td_1.width", td[0].Width, 75)           // 20 + ((200 - 20 - 40 - 3 * border-spacing) / 2)
	tableEq(t, "td_2.position_x", td[1].PositionX, 100) // 15 + 75 + border-spacing
	tableEq(t, "td_2.width", td[1].Width, 95)           // 40 + ((200 - 20 - 40 - 3 * border-spacing) / 2)
	tableEq(t, "table.width", table.Width, 200)
}

func TestLayoutTableFixed3(t *testing.T) {
	tableWrapper, table := tableRender(t, `
      <table style="table-layout: fixed; border-spacing: 10px;
                    width: 110px; margin: 5px">
        <tr>
          <td style="width: 40px">a</td>
          <td>b</td>
        </tr>
        <tr>
          <td style="width: 50px">a</td>
          <td style="width: 30px">b</td>
        </tr>
      </table>
    `)
	rows := unpack(t, only(t, table), 2)
	td12 := unpack(t, rows[0], 2)
	td34 := unpack(t, rows[1], 2)
	tableEq(t, "table_wrapper.position_x", tableWrapper.PositionX, 0)
	tableEq(t, "table.position_x", table.PositionX, 5)   // 0 + margin-left
	tableEq(t, "td_1.position_x", td12[0].PositionX, 15) // 0 + border-spacing
	tableEq(t, "td_3.position_x", td34[0].PositionX, 15)
	tableEq(t, "td_1.width", td12[0].Width, 40)
	tableEq(t, "td_2.width", td12[1].Width, 40)
	tableEq(t, "td_2.position_x", td12[1].PositionX, 65) // 15 + 40 + border-spacing
	tableEq(t, "td_4.position_x", td34[1].PositionX, 65)
	tableEq(t, "td_3.width", td34[0].Width, 40)
	tableEq(t, "td_4.width", td34[1].Width, 40)
	tableEq(t, "table.width", table.Width, 110) // 20 + 40 + 3 * border-spacing
}

func TestLayoutTableFixed4(t *testing.T) {
	tableWrapper, table := tableRender(t, `
      <table style="table-layout: fixed; border-spacing: 0;
                    width: 100px; margin: 10px">
        <colgroup>
          <col />
          <col style="width: 20px" />
        </colgroup>
        <tr>
          <td></td>
          <td style="width: 40px">a</td>
        </tr>
      </table>
    `)
	td := unpack(t, only(t, only(t, table)), 2)
	tableEq(t, "table_wrapper.position_x", tableWrapper.PositionX, 0)
	tableEq(t, "table.position_x", table.PositionX, 10) // 0 + margin-left
	tableEq(t, "td_1.position_x", td[0].PositionX, 10)
	tableEq(t, "td_1.width", td[0].Width, 80)          // 100 - 20
	tableEq(t, "td_2.position_x", td[1].PositionX, 90) // 10 + 80
	tableEq(t, "td_2.width", td[1].Width, 20)
	tableEq(t, "table.width", table.Width, 100)
}

func TestLayoutTableFixed5(t *testing.T) {
	// With border-collapse
	tableWrapper, table := tableRender(t, `
      <style>
        /* Do not apply: */
        colgroup, col, tbody, tr, td { margin: 1000px }
      </style>
      <table style="table-layout: fixed;
                    border-collapse: collapse; border: 10px solid;
                    /* ignored with collapsed borders: */
                    border-spacing: 10000px; padding: 1000px">
        <colgroup>
          <col style="width: 30px" />
        </colgroup>
        <tbody>
          <tr>
            <td style="padding: 2px"></td>
            <td style="width: 34px; padding: 10px; border: 2px solid"></td>
          </tr>
        </tbody>
      </table>
    `)
	td := unpack(t, only(t, only(t, table)), 2)
	tableEq(t, "table_wrapper.position_x", tableWrapper.PositionX, 0)
	tableEq(t, "table.position_x", table.PositionX, 0)
	tableEq(t, "table.border_left_width", table.BorderLeftWidth, 5) // half of the collapsed 10px border
	tableEq(t, "td_1.position_x", td[0].PositionX, 5)               // border-spacing is ignored
	tableEq(t, "td_1.margin_width()", td[0].MarginWidth(), 30)      // as <col>
	tableEq(t, "td_1.width", td[0].Width, 20)                       // 30 - 5 (border-left) - 1 (border-right) - 2*2
	tableEq(t, "td_2.position_x", td[1].PositionX, 35)
	tableEq(t, "td_2.width", td[1].Width, 34)
	tableEq(t, "td_2.margin_width()", td[1].MarginWidth(), 60)   // 34 + 2*10 + 5 + 1
	tableEq(t, "table.width", table.Width, 90)                   // 30 + 60
	tableEq(t, "table.margin_width()", table.MarginWidth(), 100) // 90 + 2*5 (border)
}

func TestLayoutTableAuto1(t *testing.T) {
	tableWrapper, table := tableRender(t, `
      <body style="width: 100px">
      <table style="border-spacing: 10px; margin: auto">
        <tr>
          <td><img src=pattern.png></td>
          <td><img src=pattern.png></td>
        </tr>
      </table>
    `)
	td := unpack(t, only(t, only(t, table)), 2)
	tableEq(t, "table_wrapper.position_x", tableWrapper.PositionX, 0)
	tableEq(t, "table_wrapper.width", tableWrapper.Width, 38)            // Same as table, see below
	tableEq(t, "table_wrapper.margin_left", tableWrapper.MarginLeft, 31) // 0 + margin-left = (100 - 38) / 2
	tableEq(t, "table_wrapper.margin_right", tableWrapper.MarginRight, 31)
	tableEq(t, "table.position_x", table.PositionX, 31)
	tableEq(t, "td_1.position_x", td[0].PositionX, 41) // 31 + spacing
	tableEq(t, "td_1.width", td[0].Width, 4)
	tableEq(t, "td_2.position_x", td[1].PositionX, 55) // 31 + 4 + spacing
	tableEq(t, "td_2.width", td[1].Width, 4)
	tableEq(t, "table.width", table.Width, 38) // 3 * spacing + 2 * 4
}

func TestLayoutTableAuto2(t *testing.T) {
	tableWrapper, table := tableRender(t, `
      <body style="width: 50px">
      <table style="border-spacing: 1px; margin: 10%">
        <tr>
          <td style="border: 3px solid black"><img src=pattern.png></td>
          <td style="border: 3px solid black">
            <img src=pattern.png><img src=pattern.png>
          </td>
        </tr>
      </table>
    `)
	td := unpack(t, only(t, only(t, table)), 2)
	tableEq(t, "table_wrapper.position_x", tableWrapper.PositionX, 0)
	tableEq(t, "table.position_x", table.PositionX, 5) // 0 + margin-left
	tableEq(t, "td_1.position_x", td[0].PositionX, 6)  // 5 + border-spacing
	tableEq(t, "td_1.width", td[0].Width, 4)
	tableEq(t, "td_2.position_x", td[1].PositionX, 17) // 6 + 4 + spacing + 2 * border
	tableEq(t, "td_2.width", td[1].Width, 8)
	tableEq(t, "table.width", table.Width, 27) // 3 * spacing + 4 + 8 + 4 * border
}

func TestLayoutTableAuto3(t *testing.T) {
	tableWrapper, table := tableRender(t, `
      <table style="border-spacing: 1px; margin: 5px; font-size: 0">
        <tr>
          <td></td>
          <td><img src=pattern.png><img src=pattern.png></td>
        </tr>
        <tr>
          <td>
            <img src=pattern.png>
            <img src=pattern.png>
            <img src=pattern.png>
          </td>
          <td><img src=pattern.png></td>
        </tr>
      </table>
    `)
	rows := unpack(t, only(t, table), 2)
	r1 := unpack(t, rows[0], 2)
	r2 := unpack(t, rows[1], 2)
	tableEq(t, "table_wrapper.position_x", tableWrapper.PositionX, 0)
	tableEq(t, "table.position_x", table.PositionX, 5) // 0 + margin-left
	tableEq(t, "td_11.position_x", r1[0].PositionX, 6) // 5 + spacing
	tableEq(t, "td_21.position_x", r2[0].PositionX, 6)
	tableEq(t, "td_11.width", r1[0].Width, 12)
	tableEq(t, "td_21.width", r2[0].Width, 12)
	tableEq(t, "td_12.position_x", r1[1].PositionX, 19) // 6 + 12 + spacing
	tableEq(t, "td_22.position_x", r2[1].PositionX, 19)
	tableEq(t, "td_12.width", r1[1].Width, 8)
	tableEq(t, "td_22.width", r2[1].Width, 8)
	tableEq(t, "table.width", table.Width, 23) // 3 * spacing + 12 + 8
}

func TestLayoutTableAuto4(t *testing.T) {
	tableWrapper, table := tableRender(t, `
      <table style="border-spacing: 1px; margin: 5px">
        <tr>
          <td style="border: 1px solid black"><img src=pattern.png></td>
          <td style="border: 2px solid black; padding: 1px">
            <img src=pattern.png>
          </td>
        </tr>
        <tr>
          <td style="border: 5px solid black"><img src=pattern.png></td>
          <td><img src=pattern.png></td>
        </tr>
      </table>
    `)
	rows := unpack(t, only(t, table), 2)
	r1 := unpack(t, rows[0], 2)
	r2 := unpack(t, rows[1], 2)
	tableEq(t, "table_wrapper.position_x", tableWrapper.PositionX, 0)
	tableEq(t, "table.position_x", table.PositionX, 5) // 0 + margin-left
	tableEq(t, "td_11.position_x", r1[0].PositionX, 6) // 5 + spacing
	tableEq(t, "td_21.position_x", r2[0].PositionX, 6)
	tableEq(t, "td_11.width", r1[0].Width, 12) // 4 + 2 * 5 - 2 * 1
	tableEq(t, "td_21.width", r2[0].Width, 4)
	tableEq(t, "td_12.position_x", r1[1].PositionX, 21) // 6 + 4 + 2 * b1 + sp
	tableEq(t, "td_22.position_x", r2[1].PositionX, 21)
	tableEq(t, "td_12.width", r1[1].Width, 4)
	tableEq(t, "td_22.width", r2[1].Width, 10) // 4 + 2 * 3
	tableEq(t, "table.width", table.Width, 27) // 3 * spacing + 4 + 4 + 2 * b1 + 2 * b2
}

func TestLayoutTableAuto5(t *testing.T) {
	_, table := tableRender(t, `
      <table style="width: 1000px; font-family: weasyprint">
        <tr>
          <td style="width: 40px">aa aa aa aa</td>
          <td style="width: 40px">aaaaaaaaaaa</td>
          <td>This will take the rest of the width</td>
        </tr>
      </table>
    `)
	td := unpack(t, only(t, only(t, table)), 3)
	tableEq(t, "table.width", table.Width, 1000)
	tableEq(t, "td_1.width", td[0].Width, 40)
	tableEq(t, "td_2.width", td[1].Width, 11*16)
	tableEq(t, "td_3.width", td[2].Width, 1000-40-11*16)
}

func TestLayoutTableAuto6(t *testing.T) {
	tableWrapper, table := tableRender(t, `
      <style>
        @page { size: 100px 1000px; }
      </style>
      <table style="border-spacing: 1px; margin-right: 79px; font-size: 0">
        <tr>
          <td><img src=pattern.png></td>
          <td>
            <img src=pattern.png> <img src=pattern.png>
            <img src=pattern.png> <img src=pattern.png>
            <img src=pattern.png> <img src=pattern.png>
            <img src=pattern.png> <img src=pattern.png>
            <img src=pattern.png>
          </td>
        </tr>
        <tr>
          <td></td>
        </tr>
      </table>
    `)
	// Preferred minimum width is 2 * 4 + 3 * 1 = 11
	rows := unpack(t, only(t, table), 2)
	r1 := unpack(t, rows[0], 2)
	td21 := only(t, rows[1])
	tableEq(t, "table_wrapper.position_x", tableWrapper.PositionX, 0)
	tableEq(t, "table.position_x", table.PositionX, 0)
	tableEq(t, "td_11.position_x", r1[0].PositionX, 1) // spacing
	tableEq(t, "td_21.position_x", td21.PositionX, 1)
	tableEq(t, "td_11.width", r1[0].Width, 4) // minimum width
	tableEq(t, "td_21.width", td21.Width, 4)
	tableEq(t, "td_12.position_x", r1[1].PositionX, 6) // 1 + 5 + sp
	tableEq(t, "td_12.width", r1[1].Width, 14)         // available width
	tableEq(t, "table.width", table.Width, 21)
}

func TestLayoutTableAuto7(t *testing.T) {
	tableWrapper, table := tableRender(t, `
      <table style="border-spacing: 10px; margin: 5px">
        <colgroup>
          <col style="width: 20px" />
        </colgroup>
        <tr>
          <td></td>
          <td style="width: 40px">a</td>
        </tr>
      </table>
    `)
	td := unpack(t, only(t, only(t, table)), 2)
	tableEq(t, "table_wrapper.position_x", tableWrapper.PositionX, 0)
	tableEq(t, "table.position_x", table.PositionX, 5) // 0 + margin-left
	tableEq(t, "td_1.position_x", td[0].PositionX, 15) // 0 + border-spacing
	tableEq(t, "td_1.width", td[0].Width, 20)
	tableEq(t, "td_2.position_x", td[1].PositionX, 45) // 15 + 20 + border-spacing
	tableEq(t, "td_2.width", td[1].Width, 40)
	tableEq(t, "table.width", table.Width, 90) // 20 + 40 + 3 * border-spacing
}

func TestLayoutTableAuto8(t *testing.T) {
	tableWrapper, table := tableRender(t, `
      <table style="border-spacing: 10px; width: 120px; margin: 5px;
                    font-size: 0">
        <tr>
          <td style="width: 20px"><img src=pattern.png></td>
          <td><img src=pattern.png style="width: 40px"></td>
        </tr>
      </table>
    `)
	td := unpack(t, only(t, only(t, table)), 2)
	tableEq(t, "table_wrapper.position_x", tableWrapper.PositionX, 0)
	tableEq(t, "table.position_x", table.PositionX, 5) // 0 + margin-left
	tableEq(t, "td_1.position_x", td[0].PositionX, 15) // 5 + border-spacing
	tableEq(t, "td_1.width", td[0].Width, 20)          // fixed
	tableEq(t, "td_2.position_x", td[1].PositionX, 45) // 15 + 20 + border-spacing
	tableEq(t, "td_2.width", td[1].Width, 70)          // 120 - 3 * border-spacing - 20
	tableEq(t, "table.width", table.Width, 120)
}

func TestLayoutTableAuto9(t *testing.T) {
	tableWrapper, table := tableRender(t, `
      <table style="border-spacing: 10px; width: 120px; margin: 5px">
        <tr>
          <td style="width: 60px"></td>
          <td></td>
        </tr>
        <tr>
          <td style="width: 50px"></td>
          <td style="width: 30px"></td>
        </tr>
      </table>
    `)
	rows := unpack(t, only(t, table), 2)
	td12 := unpack(t, rows[0], 2)
	td34 := unpack(t, rows[1], 2)
	tableEq(t, "table_wrapper.position_x", tableWrapper.PositionX, 0)
	tableEq(t, "table.position_x", table.PositionX, 5)   // 0 + margin-left
	tableEq(t, "td_1.position_x", td12[0].PositionX, 15) // 0 + border-spacing
	tableEq(t, "td_3.position_x", td34[0].PositionX, 15)
	tableEq(t, "td_1.width", td12[0].Width, 60)
	tableEq(t, "td_2.width", td12[1].Width, 30)
	tableEq(t, "td_2.position_x", td12[1].PositionX, 85) // 15 + 60 + border-spacing
	tableEq(t, "td_4.position_x", td34[1].PositionX, 85)
	tableEq(t, "td_3.width", td34[0].Width, 60)
	tableEq(t, "td_4.width", td34[1].Width, 30)
	tableEq(t, "table.width", table.Width, 120) // 60 + 30 + 3 * border-spacing
}

func TestLayoutTableAuto10(t *testing.T) {
	tableWrapper, table := tableRender(t, `
      <table style="border-spacing: 0; width: 14px; margin: 10px">
        <colgroup>
          <col />
          <col style="width: 6px" />
        </colgroup>
        <tr>
          <td><img src=pattern.png><img src=pattern.png></td>
          <td style="width: 8px"></td>
        </tr>
      </table>
    `)
	td := unpack(t, only(t, only(t, table)), 2)
	tableEq(t, "table_wrapper.position_x", tableWrapper.PositionX, 0)
	tableEq(t, "table.position_x", table.PositionX, 10) // 0 + margin-left
	tableEq(t, "td_1.position_x", td[0].PositionX, 10)
	tableEq(t, "td_1.width", td[0].Width, 6)           // 14 - 8
	tableEq(t, "td_2.position_x", td[1].PositionX, 16) // 10 + 6
	tableEq(t, "td_2.width", td[1].Width, 8)           // maximum of the minimum widths for the column
	tableEq(t, "table.width", table.Width, 14)
}

// tableRowCells returns the cells of the only row of the only row group of
// table, and fails the test when the row does not have n cells.
func tableRowCells(t *testing.T, table *Box, n int) []*Box {
	t.Helper()
	return unpack(t, only(t, only(t, table)), n)
}

// tableWidths checks the widths of boxes, td_1.width == want[0] and so on.
func tableCheckWidths(t *testing.T, boxes []*Box, want ...float64) {
	t.Helper()
	for i, w := range want {
		tableEq(t, "td_"+string(rune('1'+i))+".width", boxes[i].Width, w)
	}
}

func TestLayoutTableAuto11(t *testing.T) {
	tableWrapper, table := tableRender(t, `
      <table style="border-spacing: 0">
        <tr>
          <td style="width: 10px"></td>
          <td colspan="3"></td>
        </tr>
        <tr>
          <td colspan="2" style="width: 22px"></td>
          <td style="width: 8px"></td>
          <td style="width: 8px"></td>
        </tr>
        <tr>
          <td></td>
          <td></td>
          <td colspan="2"></td>
        </tr>
      </table>
    `)
	rows := unpack(t, only(t, table), 3)
	r1 := unpack(t, rows[0], 2)
	r2 := unpack(t, rows[1], 3)
	r3 := unpack(t, rows[2], 3)
	tableEq(t, "table_wrapper.position_x", tableWrapper.PositionX, 0)
	tableEq(t, "table.position_x", table.PositionX, 0)
	tableEq(t, "td_11.width", r1[0].Width, 10) // fixed
	tableEq(t, "td_12.width", r1[1].Width, 28) // 38 - 10
	tableEq(t, "td_21.width", r2[0].Width, 22) // fixed
	tableEq(t, "td_22.width", r2[1].Width, 8)  // fixed
	tableEq(t, "td_23.width", r2[2].Width, 8)  // fixed
	tableEq(t, "td_31.width", r3[0].Width, 10) // same as first line
	tableEq(t, "td_32.width", r3[1].Width, 12) // 22 - 10
	tableEq(t, "td_33.width", r3[2].Width, 16) // 8 + 8 from second line
	tableEq(t, "table.width", table.Width, 38)
}

func TestLayoutTableAuto12(t *testing.T) {
	tableWrapper, table := tableRender(t, `
      <table style="border-spacing: 10px">
        <tr>
          <td style="width: 10px"></td>
          <td colspan="3"></td>
        </tr>
        <tr>
          <td colspan="2" style="width: 32px"></td>
          <td style="width: 8px"></td>
          <td style="width: 8px"></td>
        </tr>
        <tr>
          <td></td>
          <td></td>
          <td colspan="2"></td>
        </tr>
      </table>
    `)
	rows := unpack(t, only(t, table), 3)
	r1 := unpack(t, rows[0], 2)
	r2 := unpack(t, rows[1], 3)
	r3 := unpack(t, rows[2], 3)
	tableEq(t, "table_wrapper.position_x", tableWrapper.PositionX, 0)
	tableEq(t, "table.position_x", table.PositionX, 0)
	tableEq(t, "td_11.width", r1[0].Width, 10) // fixed
	tableEq(t, "td_12.width", r1[1].Width, 48) // 32 - 10 - sp + 2 * 8 + 2 * sp
	tableEq(t, "td_21.width", r2[0].Width, 32) // fixed
	tableEq(t, "td_22.width", r2[1].Width, 8)  // fixed
	tableEq(t, "td_23.width", r2[2].Width, 8)  // fixed
	tableEq(t, "td_31.width", r3[0].Width, 10) // same as first line
	tableEq(t, "td_32.width", r3[1].Width, 12) // 32 - 10 - sp
	tableEq(t, "td_33.width", r3[2].Width, 26) // 2 * 8 + sp
	tableEq(t, "table.width", table.Width, 88)
}

func TestLayoutTableAuto13(t *testing.T) {
	// Regression test.
	_, table := tableRender(t, `
      <table style="width: 30px">
        <tr>
          <td colspan=2></td>
          <td></td>
        </tr>
      </table>
    `)
	td := tableRowCells(t, table, 2)
	tableCheckWidths(t, td, 20, 10) // 2 / 3 * 30, 1 / 3 * 30
	tableEq(t, "table.width", table.Width, 30)
}

func TestLayoutTableAuto14(t *testing.T) {
	_, table := tableRender(t, `
      <table style="width: 20px">
        <col />
        <col />
        <tr>
          <td></td>
        </tr>
      </table>
    `)
	td := tableRowCells(t, table, 1)
	tableCheckWidths(t, td, 20)
	tableEq(t, "table.width", table.Width, 20)
}

func TestLayoutTableAuto15(t *testing.T) {
	_, table := tableRender(t, `
      <table style="width: 20px">
        <col />
        <col />
      </table>
    `)
	if len(table.ColumnGroups) != 1 {
		t.Fatalf("got %d column groups, want 1", len(table.ColumnGroups))
	}
	columns := unpack(t, table.ColumnGroups[0], 2)
	tableEq(t, "column_1.width", columns[0].Width, 0)
	tableEq(t, "column_2.width", columns[1].Width, 0)
}

func TestLayoutTableAuto16(t *testing.T) {
	// Absolute table
	_, table := tableRender(t, `
      <table style="width: 30px; position: absolute">
        <tr>
          <td colspan=2></td>
          <td></td>
        </tr>
      </table>
    `)
	td := tableRowCells(t, table, 2)
	tableCheckWidths(t, td, 20, 10) // 2 / 3 * 30, 1 / 3 * 30
	tableEq(t, "table.width", table.Width, 30)
}

func TestLayoutTableAuto17(t *testing.T) {
	// With border-collapse
	tableWrapper, table := tableRender(t, `
      <style>
        /* Do not apply: */
        colgroup, col, tbody, tr, td { margin: 1000px }
      </style>
      <table style="border-collapse: collapse; border: 10px solid;
                    /* ignored with collapsed borders: */
                    border-spacing: 10000px; padding: 1000px">
        <colgroup>
          <col style="width: 30px" />
        </colgroup>
        <tbody>
          <tr>
            <td style="padding: 2px"></td>
            <td style="width: 34px; padding: 10px; border: 2px solid"></td>
          </tr>
        </tbody>
      </table>
    `)
	td := tableRowCells(t, table, 2)
	tableEq(t, "table_wrapper.position_x", tableWrapper.PositionX, 0)
	tableEq(t, "table.position_x", table.PositionX, 0)
	tableEq(t, "table.border_left_width", table.BorderLeftWidth, 5) // half of the collapsed 10px border
	tableEq(t, "td_1.position_x", td[0].PositionX, 5)               // border-spacing is ignored
	tableEq(t, "td_1.margin_width()", td[0].MarginWidth(), 30)      // as <col>
	tableEq(t, "td_1.width", td[0].Width, 20)                       // 30 - 5 (border-left) - 1 (border-right) - 2*2
	tableEq(t, "td_2.position_x", td[1].PositionX, 35)
	tableEq(t, "td_2.width", td[1].Width, 34)
	tableEq(t, "td_2.margin_width()", td[1].MarginWidth(), 60)   // 34 + 2*10 + 5 + 1
	tableEq(t, "table.width", table.Width, 90)                   // 30 + 60
	tableEq(t, "table.margin_width()", table.MarginWidth(), 100) // 90 + 2*5 (border)
}

func TestLayoutTableAuto18(t *testing.T) {
	// Column widths as percentage
	_, table := tableRender(t, `
      <table style="width: 200px">
        <colgroup>
          <col style="width: 70%" />
          <col style="width: 30%" />
        </colgroup>
        <tbody>
          <tr>
            <td>a</td>
            <td>abc</td>
          </tr>
        </tbody>
      </table>
    `)
	tableCheckWidths(t, tableRowCells(t, table, 2), 140, 60)
	tableEq(t, "table.width", table.Width, 200)
}

func TestLayoutTableAuto19(t *testing.T) {
	// Column group width
	_, table := tableRender(t, `
      <table style="width: 200px">
        <colgroup style="width: 100px">
          <col />
          <col />
        </colgroup>
        <col style="width: 100px" />
        <tbody>
          <tr>
            <td>a</td>
            <td>a</td>
            <td>abc</td>
          </tr>
        </tbody>
      </table>
    `)
	tableCheckWidths(t, tableRowCells(t, table, 3), 100, 100, 100)
	tableEq(t, "table.width", table.Width, 300)
}

func TestLayoutTableAuto20(t *testing.T) {
	// Multiple column width
	_, table := tableRender(t, `
      <table style="width: 200px">
        <colgroup>
          <col style="width: 50px" />
          <col style="width: 30px" />
          <col />
        </colgroup>
        <tbody>
          <tr>
            <td>a</td>
            <td>a</td>
            <td>abc</td>
          </tr>
        </tbody>
      </table>
    `)
	tableCheckWidths(t, tableRowCells(t, table, 3), 50, 30, 120)
	tableEq(t, "table.width", table.Width, 200)
}

func TestLayoutTableAuto21(t *testing.T) {
	// Fixed-width table with column group with widths as percentages and
	// pixels
	_, table := tableRender(t, `
      <table style="width: 500px">
        <colgroup style="width: 100px">
          <col />
          <col />
        </colgroup>
        <colgroup style="width: 30%">
          <col />
          <col />
        </colgroup>
        <tbody>
          <tr>
            <td>a</td>
            <td>a</td>
            <td>abc</td>
            <td>abc</td>
          </tr>
        </tbody>
      </table>
    `)
	tableCheckWidths(t, tableRowCells(t, table, 4), 100, 100, 150, 150)
	tableEq(t, "table.width", table.Width, 500)
}

func TestLayoutTableAuto22(t *testing.T) {
	// Auto-width table with column group with widths as percentages and
	// pixels
	_, table := tableRender(t, `
      <table>
        <colgroup style="width: 10%">
          <col />
          <col />
        </colgroup>
        <colgroup style="width: 200px">
          <col />
          <col />
        </colgroup>
        <tbody>
          <tr>
            <td>a a</td>
            <td>a b</td>
            <td>a c</td>
            <td>a d</td>
          </tr>
        </tbody>
      </table>
    `)
	tableCheckWidths(t, tableRowCells(t, table, 4), 50, 50, 200, 200)
	tableEq(t, "table.width", table.Width, 500)
}

func TestLayoutTableAuto23(t *testing.T) {
	// Wrong column group width
	_, table := tableRender(t, `
      <table style="width: 200px">
        <colgroup style="width: 20%">
          <col />
          <col />
        </colgroup>
        <tbody>
          <tr>
            <td>a</td>
            <td>a</td>
          </tr>
        </tbody>
      </table>
    `)
	tableCheckWidths(t, tableRowCells(t, table, 2), 100, 100)
	tableEq(t, "table.width", table.Width, 200)
}

func TestLayoutTableAuto24(t *testing.T) {
	// Column width as percentage and cell width in pixels
	_, table := tableRender(t, `
      <table style="width: 200px">
        <colgroup>
          <col style="width: 70%" />
          <col />
        </colgroup>
        <tbody>
          <tr>
            <td>a</td>
            <td style="width: 60px">abc</td>
          </tr>
        </tbody>
      </table>
    `)
	tableCheckWidths(t, tableRowCells(t, table, 2), 140, 60)
	tableEq(t, "table.width", table.Width, 200)
}

func TestLayoutTableAuto25(t *testing.T) {
	// Column width and cell width as percentage
	page := tablePage(t, `
      <div style="width: 400px">
        <table style="width: 50%">
          <colgroup>
            <col style="width: 70%" />
            <col />
          </colgroup>
          <tbody>
            <tr>
              <td>a</td>
              <td style="width: 30%">abc</td>
            </tr>
          </tbody>
        </table>
      </div>
    `)
	_, body := pageBody(t, page)
	div := only(t, body)
	table := only(t, only(t, div))
	tableCheckWidths(t, tableRowCells(t, table, 2), 140, 60)
	tableEq(t, "table.width", table.Width, 200)
}

func TestLayoutTableAuto26(t *testing.T) {
	// Regression test for #307.
	// Table with a cell larger than the table's max-width.
	tablePage(t, `
      <table style="max-width: 300px">
        <td style="width: 400px"></td>
      </table>
    `)
}

func TestLayoutTableAuto27(t *testing.T) {
	// Table with a cell larger than the table's width.
	tablePage(t, `
      <table style="width: 300px">
        <td style="width: 400px"></td>
      </table>
    `)
}

func TestLayoutTableAuto28(t *testing.T) {
	// Table with a cell larger than the table's width and max-width.
	tablePage(t, `
      <table style="width: 300px; max-width: 350px">
        <td style="width: 400px"></td>
      </table>
    `)
}

func TestLayoutTableAuto29(t *testing.T) {
	// Table with a cell larger than the table's width and max-width.
	tablePage(t, `
      <table style="width: 300px; max-width: 350px">
        <td style="padding: 50px">
          <div style="width: 300px"></div>
        </td>
      </table>
    `)
}

func TestLayoutTableAuto30(t *testing.T) {
	// Table with a cell larger than the table's max-width.
	tablePage(t, `
      <table style="max-width: 300px; margin: 100px">
        <td style="width: 400px"></td>
      </table>
    `)
}

func TestLayoutTableAuto31(t *testing.T) {
	// Test a table with column widths < table width < column width +
	// spacing.
	tablePage(t, `
      <table style="width: 300px; border-spacing: 2px">
        <td style="width: 299px"></td>
      </table>
    `)
}

func TestLayoutTableAuto32(t *testing.T) {
	// Table with a cell larger than the table's width.
	tableWrapper, _ := tableRender(t, `
      <table style="width: 400px; margin: 100px">
        <td style="width: 500px"></td>
      </table>
    `)
	tableEq(t, "table_wrapper.margin_width()", tableWrapper.MarginWidth(), 600) // 400 + 2 * 100
}

// tableInDiv lays out a document of one page whose body holds a div that
// holds a table, and returns the div and the table wrapper.
func tableInDiv(t *testing.T, source string) (div, wrapper *Box) {
	t.Helper()
	_, body := pageBody(t, tablePage(t, source))
	div = only(t, body)
	return div, only(t, div)
}

func TestLayoutTableAuto33(t *testing.T) {
	// Div with auto width containing a table with a min-width.
	div, tableWrapper := tableInDiv(t, `
      <div style="float: left">
        <table style="min-width: 400px; margin: 100px">
          <td></td>
        </table>
      </div>
    `)
	tableEq(t, "div.margin_width()", div.MarginWidth(), 600)                    // 400 + 2 * 100
	tableEq(t, "table_wrapper.margin_width()", tableWrapper.MarginWidth(), 600) // 400 + 2 * 100
}

func TestLayoutTableAuto34(t *testing.T) {
	// Div with auto width containing an empty table with a min-width.
	div, tableWrapper := tableInDiv(t, `
      <div style="float: left">
        <table style="min-width: 400px; margin: 100px"></table>
      </div>
    `)
	tableEq(t, "div.margin_width()", div.MarginWidth(), 600)                    // 400 + 2 * 100
	tableEq(t, "table_wrapper.margin_width()", tableWrapper.MarginWidth(), 600) // 400 + 2 * 100
}

func TestLayoutTableAuto35(t *testing.T) {
	// Div with auto width containing a table with a cell larger than the
	// table's max-width.
	div, tableWrapper := tableInDiv(t, `
      <div style="float: left">
        <table style="max-width: 300px; margin: 100px">
          <td style="width: 400px"></td>
        </table>
      </div>
    `)
	tableEq(t, "div.margin_width()", div.MarginWidth(), 600)                    // 400 + 2 * 100
	tableEq(t, "table_wrapper.margin_width()", tableWrapper.MarginWidth(), 600) // 400 + 2 * 100
}

func TestLayoutTableAuto36(t *testing.T) {
	// Regression test for #152.
	tablePage(t, `
      <table>
        <td style="width: 50%">
      </table>
    `)
}

func TestLayoutTableAuto37(t *testing.T) {
	// Regression test for #305.
	tablePage(t, `
      <table>
        <tr>
          <td>
            <table>
              <tr>
                <th>Test</th>
              </tr>
              <tr>
                <td style="min-width: 100%;"></td>
                <td style="width: 48px;"></td>
              </tr>
            </table>
          </td>
        </tr>
      </table>
    `)
}

func TestLayoutTableAuto38(t *testing.T) {
	tablePage(t, `
      <table>
        <tr>
          <td>
            <table>
              <tr>
                <td style="width: 100%;"></td>
                <td style="width: 48px;">
                  <img src="icon.png">
                </td>
              </tr>
            </table>
          </td>
        </tr>
      </table>
    `)
}

func TestLayoutTableAuto39(t *testing.T) {
	tablePage(t, `
      <table>
        <tr>
          <td>
            <table style="display: inline-table">
              <tr>
                <td style="width: 100%;"></td>
                <td></td>
              </tr>
            </table>
          </td>
        </tr>
      </table>
    `)
}

func TestLayoutTableAuto40(t *testing.T) {
	// Regression test for #368.
	// Check that white-space is used for the shrink-to-fit algorithm.
	_, table := tableRender(t, `
      <table style="width: 0">
        <td style="font-family: weasyprint; white-space: nowrap">a a</td>
      </table>
    `)
	tableEq(t, "table.width", table.Width, 16*3)
}

func TestLayoutTableAuto41(t *testing.T) {
	// Cell width as percentage in auto-width table.
	_, tableWrapper := tableInDiv(t, `
      <div style="width: 100px">
        <table>
          <tbody>
            <tr>
              <td>a a a a a a a a</td>
              <td style="width: 30%">a a a a a a a a</td>
            </tr>
          </tbody>
        </table>
      </div>
    `)
	table := only(t, tableWrapper)
	tableCheckWidths(t, tableRowCells(t, table, 2), 70, 30)
	tableEq(t, "table.width", table.Width, 100)
}

func TestLayoutTableAuto42(t *testing.T) {
	// Cell width as percentage in auto-width table.
	_, table := tableRender(t, `
      <table style="font-family: weasyprint">
        <tbody>
            <tr>
              <td style="width: 70px">aaa</td>
              <td style="width: 25%">aaa</td>
            </tr>
        </tbody>
      </table>
    `)
	td := tableRowCells(t, table, 2)
	tableEq(t, "td_2.width", td[1].Width, 16*3)     // Percentage column is set to max-width
	tableEq(t, "td_1.width", td[0].Width, (16*3)*3) // Pixel column constraint is ignored
	tableEq(t, "table.width", table.Width, (16*3)*4)
}

func TestLayoutTableAuto43(t *testing.T) {
	// Cell width as percentage on colspan cell in auto-width table.
	_, tableWrapper := tableInDiv(t, `
      <div style="width: 100px">
        <table>
          <tbody>
            <tr>
              <td>a a a a a a a a</td>
              <td style="width: 30%" colspan=2>a a a a a a a a</td>
              <td>a a a a a a a a</td>
            </tr>
          </tbody>
        </table>
      </div>
    `)
	table := only(t, tableWrapper)
	tableCheckWidths(t, tableRowCells(t, table, 3), 35, 30, 35)
	tableEq(t, "table.width", table.Width, 100)
}

func TestLayoutTableAuto44(t *testing.T) {
	// Cells widths as percentages on normal and colspan cells.
	_, tableWrapper := tableInDiv(t, `
      <div style="width: 100px">
        <table>
          <tbody>
            <tr>
              <td>a a a a a a a a</td>
              <td style="width: 30%" colspan=2>a a a a a a a a</td>
              <td>a a a a a a a a</td>
              <td style="width: 40%">a a a a a a a a</td>
              <td>a a a a a a a a</td>
            </tr>
          </tbody>
        </table>
      </div>
    `)
	table := only(t, tableWrapper)
	tableCheckWidths(t, tableRowCells(t, table, 5), 10, 30, 10, 40, 10)
	tableEq(t, "table.width", table.Width, 100)
}

func TestLayoutTableAuto45(t *testing.T) {
	// Cells widths as percentage on multiple lines.
	_, tableWrapper := tableInDiv(t, `
      <div style="width: 1000px">
        <table>
          <tbody>
            <tr>
              <td>a a a a a a a a</td>
              <td style="width: 30%">a a a a a a a a</td>
              <td>a a a a a a a a</td>
              <td style="width: 40%">a a a a a a a a</td>
              <td>a a a a a a a a</td>
            </tr>
            <tr>
              <td style="width: 31%" colspan=2>a a a a a a a a</td>
              <td>a a a a a a a a</td>
              <td style="width: 42%" colspan=2>a a a a a a a a</td>
            </tr>
          </tbody>
        </table>
      </div>
    `)
	table := only(t, tableWrapper)
	rows := unpack(t, only(t, table), 2)
	// 31% - 30%, 30%, 1000 - 31% - 42%, 40%, 42% - 2%
	tableCheckWidths(t, unpack(t, rows[0], 5), 10, 300, 270, 400, 20)
	// 31%, 1000 - 31% - 42%, 42%
	tableCheckWidths(t, unpack(t, rows[1], 3), 310, 270, 420)
	tableEq(t, "table.width", table.Width, 1000)
}

func TestLayoutTableAuto46(t *testing.T) {
	div, tableWrapper := tableInDiv(t, `
      <div style="position: absolute">
        <table style="margin: 50px; border: 20px solid black">
          <tr>
            <td style="width: 200px; height: 200px"></td>
          </tr>
        </table>
      </div>
    `)
	table := only(t, tableWrapper)
	tableCheckWidths(t, tableRowCells(t, table, 1), 200)
	tableEq(t, "table.width", table.Width, 200)
	tableEq(t, "div.width", div.Width, 340) // 200 + 2 * 50 + 2 * 20
}

func TestLayoutTableAuto47(t *testing.T) {
	// Regression test for #666.
	_, table := tableRender(t, `
      <table style="font-family: weasyprint">
        <tr>
          <td colspan=5>aaa</td>
        </tr>
      </table>
    `)
	tableCheckWidths(t, tableRowCells(t, table, 1), 48) // 3 * font-size
}

func TestLayoutTableAuto48(t *testing.T) {
	// Regression test for #685.
	_, table := tableRender(t, `
      <table style="font-family: weasyprint; border-spacing: 100px;
                    border-collapse: collapse">
        <tr>
          <td colspan=5>aaa</td>
        </tr>
      </table>
    `)
	tableCheckWidths(t, tableRowCells(t, table, 1), 48) // 3 * font-size
}

func TestLayoutTableAuto49(t *testing.T) {
	t.Skip("upstream marks it as an expected failure, see the TODO in table_layout.group_layout")
	// Regression test for #685.
	_, table := tableRender(t, `
      <table style="font-family: weasyprint; border-spacing: 100px">
        <tr>
          <td colspan=5>aaa</td>
        </tr>
      </table>
    `)
	tableCheckWidths(t, tableRowCells(t, table, 1), 48) // 3 * font-size
}

func TestLayoutTableAuto50(t *testing.T) {
	// Regression test for #685.
	_, table := tableRender(t, `
      <table style="font-family: weasyprint; border-spacing: 5px">
       <tr><td>a</td><td>a</td><td>a</td><td>a</td><td>a</td></tr>
       <tr>
         <td colspan='5'>aaa aaa aaa aaa</td>
       </tr>
      </table>
    `)
	rows := unpack(t, only(t, table), 2)
	for _, td := range rows[0].Children {
		tableEq(t, "td.width", td.Width, 44) // (15 * font_size - 4 * sp) / 5
	}
	td21 := only(t, rows[1])
	tableEq(t, "td_21.width", td21.Width, 240) // 15 * font_size
}

func TestLayoutTableAuto51(t *testing.T) {
	// Regression test for #2174.
	_, table := tableRender(t, `
      <table style="font-family: weasyprint; width: 100px">
        <tr>
          <td style="width: 29.9999%">a</td>
          <td style="width: 70%">a</td>
        </tr>
      </table>
    `)
	td := tableRowCells(t, table, 2)
	if math.Abs(td[0].Width-30) >= 0.1 {
		t.Errorf("td_1.width = %v, want about 30", td[0].Width)
	}
	if math.Abs(td[1].Width-70) >= 0.1 {
		t.Errorf("td_2.width = %v, want about 70", td[1].Width)
	}
}

func TestLayoutTableAuto52(t *testing.T) {
	// Regression test for #2325.
	tableWrapper, table := tableRender(t, `
      <style>
        @page { size: 20px }
      </style>
      <table style="font-family: weasyprint; border-spacing: 1px;
                    font-size: 2px; line-height: 1">
        <tr>
          <td><img src=pattern.png></td>
          <td>
            <span>foo</span>,
            <span>foo</span>,
            <span>foo</span>
          </td>
        </tr>
      </table>
    `)
	td := tableRowCells(t, table, 2)
	tableEq(t, "table.width", table.Width, 20)
	tableEq(t, "table_wrapper.position_x", tableWrapper.PositionX, 0)
	tableEq(t, "table.position_x", table.PositionX, 0)
	tableEq(t, "td_1.position_x", td[0].PositionX, 1)                  // spacing
	tableEq(t, "td_1.width", td[0].Width, 4)                           // image width
	tableEq(t, "td_2.position_x", td[1].PositionX, td[0].Width+2*1)    // 2 * spacing
	tableEq(t, "td_2.width", td[1].Width, table.Width-td[0].Width-3*1) // 3 * spacing
	tableEq(t, "td_2.height", td[1].Height, 3*2)                       // 3 lines * line height
}

func TestExplicitWidthTablePercentRtl(t *testing.T) {
	for _, c := range []struct {
		bodyWidth, tableWidth string
		checkWidth            float64
		positions, widths     []float64
	}{
		{"500px", "230px", 220, []float64{170, 5}, []float64{45, 155}},
		{"530px", "100%", 520, []float64{395, 5}, []float64{120, 380}},
	} {
		t.Run(c.bodyWidth, func(t *testing.T) {
			page := tablePage(t, `
      <style>
        body { width: `+c.bodyWidth+` }
        table { width: `+c.tableWidth+`; table-layout: fixed; direction: rtl;
                border-collapse: collapse; font-size: 1px }
        td, th { border: 10px solid }
      </style>
      <table style="">
        <col style="width: 25%"></col>
        <col></col>
        <tr>
          <th>الاسم</th>
          <th>العائلة</th>
        </tr>
        <tr>
          <td>محمد يوسف</td>
          <td>29</td>
        </tr>
      </table>
    `)
			_, body := pageBody(t, page)
			table := only(t, only(t, body))
			rows := unpack(t, only(t, table), 2)
			tableEq(t, "table.position_x", table.PositionX, 0)
			tableEq(t, "table.width", table.Width, c.checkWidth)
			for _, row := range rows {
				var positions, widths []float64
				for _, child := range row.Children {
					positions = append(positions, child.PositionX)
					widths = append(widths, child.Width)
				}
				tableFloats(t, "positions", positions, c.positions)
				tableFloats(t, "widths", widths, c.widths)
			}
		})
	}
}

func TestTableColumnWidth1(t *testing.T) {
	source := `
      <style>
        body { width: 20000px; margin: 0 }
        table {
          width: 10000px; margin: 0 auto; border-spacing: 100px 0;
          table-layout: fixed
        }
        td { border: 10px solid; padding: 1px }
      </style>
      <table>
        <col style="width: 10%">
        <tr>
          <td style="width: 30%" colspan=3>
          <td>
        </tr>
        <tr>
          <td>
          <td>
          <td>
          <td>
        </tr>
        <tr>
          <td>
          <td colspan=12>This cell will be truncated to grid width
          <td>This cell will be removed as it is beyond the grid width
        </tr>
      </table>
    `
	log, c := captureLogs(slog.LevelWarn)
	pages := renderPagesLog(t, source, testBaseURL, log)
	logs := c.logs()
	if len(logs) != 1 || !strings.HasPrefix(logs[0], "WARNING: This row has more columns") {
		t.Errorf("logs = %q, want one warning about the columns", logs)
	}
	if len(pages) != 1 {
		t.Fatalf("got %d pages, want 1", len(pages))
	}
	_, body := pageBody(t, pages[0])
	wrapper := only(t, body)
	table := only(t, wrapper)
	rowGroup := only(t, table)
	rows := unpack(t, rowGroup, 3)
	cells := [][]*Box{rows[0].Children, rows[1].Children, rows[2].Children}
	if len(cells[0]) != 2 || len(cells[1]) != 4 || len(cells[2]) != 2 {
		// Third cell here is completely removed.
		t.Fatalf("got rows of %d, %d and %d cells, want 2, 4 and 2", len(cells[0]), len(cells[1]), len(cells[2]))
	}

	tableEq(t, "body.position_x", body.PositionX, 0)
	tableEq(t, "wrapper.position_x", wrapper.PositionX, 0)
	tableEq(t, "wrapper.margin_left", wrapper.MarginLeft, 5000)
	tableEq(t, "wrapper.content_box_x()", wrapper.ContentBoxX(), 5000) // auto margin-left
	tableEq(t, "wrapper.width", wrapper.Width, 10000)
	tableEq(t, "table.position_x", table.PositionX, 5000)
	tableEq(t, "table.width", table.Width, 10000)
	tableEq(t, "row_group.position_x", rowGroup.PositionX, 5100) // 5000 + border_spacing
	tableEq(t, "row_group.width", rowGroup.Width, 9800)          // 10000 - 2*border-spacing
	tableEq(t, "first_row.position_x", rows[0].PositionX, rowGroup.PositionX)
	tableEq(t, "first_row.width", rows[0].Width, rowGroup.Width)

	check := func(name string, cell *Box, positionX, borderWidth, width float64) {
		t.Helper()
		tableEq(t, name+".position_x", cell.PositionX, positionX)
		tableEq(t, name+".border_width()", cell.BorderWidth(), borderWidth)
		tableEq(t, name+".width", cell.Width, width)
	}
	// This cell has colspan=3. `width` on a cell sets the content width.
	// 5000 + border-spacing, 3000 + borders + padding, 30% of 10000px
	check("cells[0][0]", cells[0][0], 5100, 3022, 3000)
	// Second cell of the first line, but on the fourth and last column.
	// 5100 + 3022 + border-spacing, 10000 - 3022 - 3*100, 6678 - borders
	// - padding
	check("cells[0][1]", cells[0][1], 8222, 6678, 6656)
	// `width` on a column sets the border width of cells.
	// 5000 + border-spacing, 10% of 10000px, 1000 - borders - padding
	check("cells[1][0]", cells[1][0], 5100, 1000, 978)
	// 5100 + 1000 + border-spacing, (3022 - 1000 - 2*100) / 2, 911 -
	// borders - padding
	check("cells[1][1]", cells[1][1], 6200, 911, 889)
	// 6200 + 911 + border-spacing
	check("cells[1][2]", cells[1][2], 7211, 911, 889)
	// Same as cells[0][1].
	check("cells[1][3]", cells[1][3], 8222, 6678, 6656)
	// Same as cells[1][0].
	check("cells[2][0]", cells[2][0], 5100, 1000, 978)
	// Same as cells[1][1], 1000 - 1000 - 3*border-spacing, 8700 - borders
	// - padding
	check("cells[2][1]", cells[2][1], 6200, 8700, 8678)
	if cells[2][1].Colspan != 3 { // truncated to grid width
		t.Errorf("cells[2][1].colspan = %d, want 3", cells[2][1].Colspan)
	}
}

func TestTableColumnWidth2(t *testing.T) {
	_, table := tableRender(t, `
      <style>
        table { width: 1000px; border-spacing: 100px; table-layout: fixed }
      </style>
      <table>
        <tr>
          <td style="width: 50%">
          <td style="width: 60%">
          <td>
        </tr>
      </table>
    `)
	row := only(t, only(t, table))
	tableCheckWidths(t, row.Children, 500, 600, 0)
	tableEq(t, "table.width", table.Width, 1500) // 500 + 600 + 4 * border-spacing
}

func TestTableColumnWidth3(t *testing.T) {
	// Sum of columns width larger that the table width: increase the table
	// width.
	wrapper, table := tableRender(t, `
      <style>
        table { width: 1000px; border-spacing: 100px; table-layout: fixed }
        td { width: 60% }
      </style>
      <table>
        <tr>
          <td>
          <td>
        </tr>
      </table>
    `)
	tableCheckWidths(t, tableRowCells(t, table, 2), 600, 600) // 60% of 1000px
	tableEq(t, "table.width", table.Width, 1500)              // 600 + 600 + 3*border-spacing
	tableEq(t, "wrapper.width", wrapper.Width, table.Width)
}

// tableCellValues returns a value of each cell of each row of rowGroup.
func tableCellValues(rowGroup *Box, value func(*Box) float64) [][]float64 {
	values := make([][]float64, len(rowGroup.Children))
	for i, row := range rowGroup.Children {
		values[i] = []float64{}
		for _, cell := range row.Children {
			values[i] = append(values[i], value(cell))
		}
	}
	return values
}

// tableCheckGrid fails the test when the lists of lists are not equal.
func tableCheckGrid(t *testing.T, name string, got, want [][]float64) {
	t.Helper()
	if len(got) != len(want) {
		t.Errorf("%s = %v, want %v", name, got, want)
		return
	}
	for i := range got {
		tableFloats(t, name, got[i], want[i])
	}
}

func TestTableRowHeight1(t *testing.T) {
	wrapper, table := tableRender(t, `
      <table style="width: 1000px; border-spacing: 0 100px;
                    font: 20px/1em serif; margin: 3px; table-layout: fixed">
        <tr>
          <td rowspan=0 style="height: 420px; vertical-align: top"></td>
          <td>X<br>X<br>X</td>
          <td><table style="margin-top: 20px; border-spacing: 0">
            <tr><td>X</td></tr></table></td>
          <td style="vertical-align: top">X</td>
          <td style="vertical-align: middle">X</td>
          <td style="vertical-align: bottom">X</td>
        </tr>
        <tr>
          <!-- cells with no text (no line boxes) is a corner case
               in cell baselines -->
          <td style="padding: 15px"></td>
          <td><div style="height: 10px"></div></td>
        </tr>
        <tr></tr>
        <tr>
            <td style="vertical-align: bottom"></td>
        </tr>
      </table>
    `)
	rowGroup := only(t, table)

	tableEq(t, "wrapper.position_y", wrapper.PositionY, 0)
	tableEq(t, "table.position_y", table.PositionY, 3) // 0 + margin-top
	tableEq(t, "table.height", table.Height, 620)      // sum of row heigths + 5*border-spacing
	tableEq(t, "wrapper.height", wrapper.Height, table.Height)
	tableEq(t, "row_group.position_y", rowGroup.PositionY, 103) // 3 + border-spacing
	tableEq(t, "row_group.height", rowGroup.Height, 420)        // 620 - 2*border-spacing
	heights := make([]float64, 0, len(rowGroup.Children))
	positions := make([]float64, 0, len(rowGroup.Children))
	for _, row := range rowGroup.Children {
		heights = append(heights, row.Height)
		positions = append(positions, row.PositionY)
	}
	tableFloats(t, "row heights", heights, []float64{80, 30, 0, 10})
	// cumulative sum of previous row heights and border-spacings
	tableFloats(t, "row positions", positions, []float64{103, 283, 413, 513})
	tableCheckGrid(t, "cell heights", tableCellValues(rowGroup, func(c *Box) float64 { return c.Height }),
		[][]float64{{420, 60, 40, 20, 20, 20}, {0, 10}, {}, {0}})
	tableCheckGrid(t, "cell border heights", tableCellValues(rowGroup, func(c *Box) float64 { return c.BorderHeight() }),
		[][]float64{{420, 80, 80, 80, 80, 80}, {30, 30}, {}, {10}})
	// The baseline of the first row is at 40px because of the third
	// column. The second column thus gets a top padding of 20px pushes the
	// bottom to 80px.The middle is at 40px.
	tableCheckGrid(t, "cell padding tops", tableCellValues(rowGroup, func(c *Box) float64 { return c.PaddingTop }),
		[][]float64{{0, 20, 0, 0, 30, 60}, {15, 5}, {}, {10}})
	tableCheckGrid(t, "cell padding bottoms", tableCellValues(rowGroup, func(c *Box) float64 { return c.PaddingBottom }),
		[][]float64{{0, 0, 40, 60, 30, 0}, {15, 15}, {}, {0}})
	tableCheckGrid(t, "cell positions", tableCellValues(rowGroup, func(c *Box) float64 { return c.PositionY }),
		[][]float64{{103, 103, 103, 103, 103, 103}, {283, 283}, {}, {513}})
}

func TestTableRowHeight2(t *testing.T) {
	// A cell box cannot extend beyond the last row box of a table.
	_, table := tableRender(t, `
      <table style="border-spacing: 0">
        <tr style="height: 10px">
          <td rowspan=5></td>
          <td></td>
        </tr>
        <tr style="height: 10px">
          <td></td>
        </tr>
      </table>
    `)
	only(t, table)
}

func TestTableRowHeight3(t *testing.T) {
	// Regression test for #937.
	_, table := tableRender(t, `
      <table style="border-spacing: 0; font-family: weasyprint;
                    line-height: 20px">
        <tr><td>Table</td><td rowspan="2"></td></tr>
        <tr></tr>
      </table>
    `)
	tableEq(t, "table.height", table.Height, 20)
	rowGroup := only(t, table)
	tableEq(t, "row_group.height", rowGroup.Height, 20)
	rows := unpack(t, rowGroup, 2)
	tableEq(t, "row1.height", rows[0].Height, 20)
	tableEq(t, "row2.height", rows[1].Height, 0)
}

// TestTableRowHeightSpan checks that the row with a height in which a cell
// that spans rows ends gets only the part of the cell past the rows before
// it. WeasyPrint gives the row the whole height of the cell. This port
// differs on purpose.
func TestTableRowHeightSpan(t *testing.T) {
	_, table := tableRender(t, `
      <table style="border-spacing: 0">
        <tr style="height: 30px"><td rowspan="2" style="vertical-align: top"><div style="height: 50px"></div></td><td style="vertical-align: top"></td></tr>
        <tr style="height: 10px"><td style="vertical-align: top"></td></tr>
      </table>
    `)
	rowGroup := only(t, table)
	rows := unpack(t, rowGroup, 2)
	tableEq(t, "row1.height", rows[0].Height, 30)
	tableEq(t, "row2.height", rows[1].Height, 20)
	tableEq(t, "row_group.height", rowGroup.Height, 50)
}

// TestTableRowHeightNoEnd checks that a row with a height in which no cell
// ends keeps its height, as in a browser. WeasyPrint gives it the height 0.
// The cells that span the rows end in the last row, which gets only the
// part of the cells past the rows before it. A row without a height in
// which no cell ends still gets the height 0, as TestTableRowHeight3
// checks.
func TestTableRowHeightNoEnd(t *testing.T) {
	_, table := tableRender(t, `
      <table style="border-spacing: 0">
        <tr style="height: 15px"><td rowspan="3" style="vertical-align: top"><div style="height: 10px"></div></td><td rowspan="3"></td></tr>
        <tr style="height: 15px"></tr>
        <tr style="height: 15px"></tr>
        <tr style="height: 15px"><td rowspan="3" style="vertical-align: top"><div style="height: 60px"></div></td></tr>
        <tr style="height: 15px"></tr>
        <tr style="height: 15px"></tr>
      </table>
    `)
	rowGroup := only(t, table)
	rows := unpack(t, rowGroup, 6)
	for i, want := range []float64{15, 15, 15, 15, 15, 30} {
		tableEq(t, fmt.Sprintf("row%d.height", i+1), rows[i].Height, want)
		tableEq(t, fmt.Sprintf("row%d.position_y", i+1), rows[i].PositionY, 15*float64(i))
	}
	tableEq(t, "row_group.height", rowGroup.Height, 105)
	cells := rows[0].Children
	tableEq(t, "cell1.border_height", cells[0].BorderHeight(), 45)
}

func TestTableRowHeight4(t *testing.T) {
	// A row cannot be shorter than the border-height of its tallest cell.
	_, table := tableRender(t, `
      <table style="border-spacing: 0;">
        <tr style="height: 4px;">
          <td style="border: 1px solid; padding: 5px;"></td>
        </tr>
      </table>
    `)
	rowGroup := only(t, table)
	tableEq(t, "row_group.height", rowGroup.Height, 12)
}

func TestTableVerticalAlign(t *testing.T) {
	assertPixels(t, "table_vertical_align", `
        rrrrrrrrrrrrrrrrrrrrrrrrrrrr
        rBBBBBBBBBBBBBBBBBBBBBBBBBBr
        rBrBB_BB_BB_BB_BBrrBBrrBB_Br
        rBrBB_BB_BBrBBrBBrrBBrrBBrBr
        rB_BBrBB_BBrBBrBBrrBBrrBBrBr
        rB_BBrBB_BB_BB_BBrrBBrrBB_Br
        rB_BB_BBrBB_BB_BB__BB__BB_Br
        rB_BB_BBrBB_BB_BB__BB__BB_Br
        rBBBBBBBBBBBBBBBBBBBBBBBBBBr
        rrrrrrrrrrrrrrrrrrrrrrrrrrrr
    `, `
      <style>
        @page { size: 28px 10px }
        html { font-size: 1px; color: red }
        body { margin: 0; width: 28px; height: 10px }
        td {
          width: 1em;
          padding: 0 !important;
          border: 1px solid blue;
          line-height: 1em;
          font-family: weasyprint;
        }
      </style>
      <table style="border: 1px solid red; border-spacing: 0">
        <tr>
          <!-- Test vertical-align: top, auto height -->
          <td style="vertical-align: top">o o</td>

          <!-- Test vertical-align: middle, auto height -->
          <td style="vertical-align: middle">o o</td>

          <!-- Test vertical-align: bottom, fixed useless height -->
          <td style="vertical-align: bottom; height: 2em">o o</td>

          <!-- Test default vertical-align value (baseline),
               fixed useless height -->
          <td style="height: 5em">o o</td>

          <!-- Test vertical-align: baseline with baseline set by next cell,
               auto height -->
          <td style="vertical-align: baseline">o o</td>

          <!-- Set baseline height to 2px, auto height -->
          <td style="vertical-align: baseline; font-size: 2em">o o</td>

          <!-- Test padding-bottom, fixed useless height,
               set the height of the cells to 2 lines * 2em + 2px = 6px -->
          <td style="vertical-align: baseline; height: 1em;
                     font-size: 2em; padding-bottom: 2px !important">
            o o
          </td>

          <!-- Test padding-top, auto height -->
          <td style="vertical-align: top; padding-top: 1em !important">
            o o
          </td>
        </tr>
      </table>
    `)
}

func TestTableVerticalAlignFloat(t *testing.T) {
	// Regression test for #2216 and #2293.
	_, table := tableRender(t, `
      <style>
        @page { size: 100px 400px }
        td { width: 50px; height: 100px; line-height: 0 }
      </style>
      <table>
        <tr>
          <td style="vertical-align: middle">
            <div style="float: left; height: 20px; width: 20px"></div>
            <div style="display: inline-block; height: 30px; width: 30px"></div>
          </td>
          <td style="vertical-align: bottom">
            <div style="display: inline-block; height: 30px; width: 30px"></div>
            <div style="float: right; height: 20px; width: 20px"></div>
          </td>
        </tr>
        <tr>
          <td style="vertical-align: middle">
            <div style="float: left; height: 20px; width: 20px"></div>
            <div style="display: inline-block; height: 10px; width: 10px"></div>
          </td>
          <td style="vertical-align: bottom">
            <div style="display: inline-block; height: 10px; width: 10px"></div>
            <div style="float: right; height: 20px; width: 20px"></div>
          </td>
        </tr>
        <tr>
          <td style="vertical-align: middle">
            <div style="display: inline-block; height: 10px; width: 10px"></div>
          </td>
          <td style="vertical-align: bottom">
            <div style="display: inline-block; height: 10px; width: 10px"></div>
          </td>
        </tr>
        <tr>
          <td style="vertical-align: middle">
            <div style="float: left; height: 20px; width: 20px"></div>
          </td>
          <td style="vertical-align: bottom">
            <div style="float: right; height: 20px; width: 20px"></div>
          </td>
        </tr>
      </table>
    `)
	rows := unpack(t, only(t, table), 4)
	pos := func(name string, box *Box, x, y float64) {
		t.Helper()
		tableEq(t, name+".position_x", box.PositionX, x)
		tableEq(t, name+".position_y", box.PositionY, y)
	}

	td := unpack(t, rows[0], 2)
	c := unpack(t, td[0], 2)
	pos("div_1", c[0], 0, (100-30)/2.)
	pos("div_2", c[1].Children[0].Children[0], 20, (100-30)/2.)
	c = unpack(t, only(t, td[1]), 3)
	pos("div_1", c[0], 50, 100-30)
	pos("div_2", c[2], 80, 100-30)

	td = unpack(t, rows[1], 2)
	c = unpack(t, td[0], 2)
	pos("div_1", c[0], 0, 100+(100-20)/2.)
	pos("div_2", c[1].Children[0].Children[0], 20, 100+(100-20)/2.)
	c = unpack(t, only(t, td[1]), 3)
	pos("div_1", c[0], 50, 100+(100-20))
	pos("div_2", c[2], 80, 100+(100-20))

	td = unpack(t, rows[2], 2)
	c = unpack(t, only(t, td[0]), 2)
	pos("div", c[0], 0, 200+(100-10)/2.)
	pos("div", only(t, td[1]), 50, 200+(100-10))

	td = unpack(t, rows[3], 2)
	pos("div", only(t, td[0]), 0, 300+(100-20)/2.)
	pos("div", only(t, td[1]), 80, 300+(100-20))
}

func TestTableWrapper(t *testing.T) {
	page := tablePage(t, `
      <style>
        @page { size: 1000px }
        table { width: 600px; height: 500px; table-layout: fixed;
                  padding: 1px; border: 10px solid; margin: 100px; }
      </style>
      <table></table>
    `)
	_, body := pageBody(t, page)
	wrapper := only(t, body)
	table := only(t, wrapper)
	tableEq(t, "body.width", body.Width, 1000)
	tableEq(t, "wrapper.width", wrapper.Width, 600) // not counting borders or padding
	tableEq(t, "wrapper.margin_left", wrapper.MarginLeft, 100)
	tableEq(t, "table.margin_width()", table.MarginWidth(), 600)
	tableEq(t, "table.width", table.Width, 578) // 600 - 2*10 - 2*1, no margin
	// box-sizing in the UA stylesheet makes `height: 500px` set this.
	tableEq(t, "table.border_height()", table.BorderHeight(), 500)
	tableEq(t, "table.height", table.Height, 478)                  // 500 - 2*10 - 2*1
	tableEq(t, "table.margin_height()", table.MarginHeight(), 500) // no margin
	tableEq(t, "wrapper.height", wrapper.Height, 500)
	tableEq(t, "wrapper.margin_height()", wrapper.MarginHeight(), 700) // 500 + 2*100
}

func TestTableHTMLTag(t *testing.T) {
	// Regression test.
	tablePages(t, `<html style="display: table">`)
}
