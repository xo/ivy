// Copyright (c) 2011-2021, Simon Sapin and contributors.
//
// Use of this source code is governed by the BSD-3-Clause license that can be
// found in the LICENSE file.

// This file ports tests/layout/test_column.py: the tests of the layout of
// the multi-column boxes.

package layout

import (
	"fmt"
	"testing"
)

// columnChild returns the descendant of box at the path of indexes of
// children, such as box.children[0].children[1] for 0, 1.
func columnChild(t *testing.T, box *Box, path ...int) *Box {
	t.Helper()
	for _, i := range path {
		if i < 0 || i >= len(box.Children) {
			t.Fatalf("%s has %d children, no child %d", box, len(box.Children), i)
		}
		box = box.Children[i]
	}
	return box
}

// columnText fails the test when the text of the descendant of box at
// path is not text.
func columnText(t *testing.T, box *Box, text string, path ...int) {
	t.Helper()
	if got := columnChild(t, box, path...).Text; got != text {
		t.Errorf("text %q, want %q", got, text)
	}
}

// columnPosition fails the test when the position of box is not (x, y).
func columnPosition(t *testing.T, box *Box, x, y float64) {
	t.Helper()
	if box.PositionX != x || box.PositionY != y {
		t.Errorf("%s at (%v, %v), want (%v, %v)", box, box.PositionX, box.PositionY, x, y)
	}
}

// columnContentPosition fails the test when the position of the content
// box of box is not (x, y).
func columnContentPosition(t *testing.T, box *Box, x, y float64) {
	t.Helper()
	if box.ContentBoxX() != x || box.ContentBoxY() != y {
		t.Errorf("%s content at (%v, %v), want (%v, %v)", box, box.ContentBoxX(), box.ContentBoxY(), x, y)
	}
}

// columnDiv renders a page and returns the children of the only child of
// its body.
func columnDiv(t *testing.T, page *Box, n int) []*Box {
	t.Helper()
	_, body := pageBody(t, page)
	return unpack(t, only(t, body), n)
}

// TestColumns is test_columns.
func TestColumns(t *testing.T) {
	for _, css := range []string{
		"columns: 4",
		"columns: 100px",
		"columns: 4 100px",
		"columns: 100px 4",
		"column-width: 100px",
		"column-count: 4",
	} {
		t.Run(css, func(t *testing.T) {
			page := pageRender(t, `
      <style>
        div { `+css+`; column-gap: 0 }
        body { margin: 0; font-family: weasyprint }
        @page { margin: 0; size: 400px 1000px }
      </style>
      <div>
        Ipsum dolor sit amet,
        consectetur adipiscing elit.
        Sed sollicitudin nibh
        et turpis molestie tristique.
      </div>
    `, 1)[0]
			columns := columnDiv(t, page, 4)
			for i, column := range columns {
				pageEqual(t, "width", column.Width, 100)
				columnPosition(t, column, float64(100*i), 0)
			}
		})
	}
}

// TestColumnGap is test_column_gap.
func TestColumnGap(t *testing.T) {
	for _, c := range []struct {
		value string
		width float64
	}{
		{"normal", 16},  // "normal" is 1em = 16px
		{"unknown", 16}, // default value is normal
		{"15px", 15},
		{"5%", 15},
		{"-1em", 16}, // negative values are not allowed
	} {
		t.Run(c.value, func(t *testing.T) {
			page := renderPages(t, `
      <style>
        div { columns: 3; column-gap: `+c.value+` }
        body { margin: 0; font-family: weasyprint }
        @page { margin: 0; size: 300px 1000px }
      </style>
      <div>
        Ipsum dolor sit amet,
        consectetur adipiscing elit.
        Sed sollicitudin nibh
        et turpis molestie tristique.
      </div>
    `)[0]
			columns := columnDiv(t, page, 3)
			for i, column := range columns {
				pageEqual(t, "width", column.Width, 100-2*c.width/3)
				pageEqual(t, "x", column.PositionX, float64(100*i)+float64(i)*c.width/3)
				pageEqual(t, "y", column.PositionY, 0)
			}
		})
	}
}

// TestColumnSpan1 is test_column_span_1.
func TestColumnSpan1(t *testing.T) {
	page := pageRender(t, `
      <style>
        body { margin: 0; font-family: weasyprint; line-height: 1 }
        div { columns: 2; width: 10em; column-gap: 0 }
        section { column-span: all; margin: 1em 0 }
      </style>

      <div>
        abc def
        <section>test</section>
        <section>test</section>
        ghi jkl
      </div>
    `, 1)[0]
	children := columnDiv(t, page, 6)
	column1, column2, section1, section2, column3, column4 := children[0], children[1], children[2], children[3], children[4], children[5]
	columnPosition(t, column1, 0, 0)
	columnPosition(t, column2, 5*16, 0)
	columnContentPosition(t, section1, 0, 32)
	columnContentPosition(t, section2, 0, 64)
	columnPosition(t, column3, 0, 96)
	columnPosition(t, column4, 5*16, 96)

	pageEqual(t, "column 1 height", column1.Height, 16)
}

// TestColumnSpan2 is test_column_span_2.
func TestColumnSpan2(t *testing.T) {
	page := pageRender(t, `
      <style>
        body { margin: 0; font-family: weasyprint; line-height: 1 }
        div { columns: 2; width: 10em; column-gap: 0 }
        section { column-span: all; margin: 1em 0 }
      </style>

      <div>
        <section>test</section>
        abc def
        ghi jkl
        mno pqr
        stu vwx
      </div>
    `, 1)[0]
	children := columnDiv(t, page, 3)
	section1, column1, column2 := children[0], children[1], children[2]
	columnContentPosition(t, section1, 0, 16)
	columnPosition(t, column1, 0, 3*16)
	columnPosition(t, column2, 5*16, 3*16)

	pageEqual(t, "column 1 height", column1.Height, 16*4)
	pageEqual(t, "column 2 height", column2.Height, 16*4)
}

// columnSpanStyle is the style of test_column_span_3 to
// test_column_span_9, with the size of the page.
func columnSpanStyle(size, section string) string {
	return `
      <style>
        @page { margin: 0; size: ` + size + ` }
        body { font-family: weasyprint; font-size: 1px }
        div { columns: 2; column-gap: 0; line-height: 1 }
        section { column-span: all` + section + ` }
      </style>`
}

// TestColumnSpan3 is test_column_span_3.
func TestColumnSpan3(t *testing.T) {
	pages := pageRender(t, columnSpanStyle("8px 3px", "")+`
      <div>
        abc def
        ghi jkl
        <section>line1 line2</section>
        mno pqr
      </div>
    `, 2)
	children := columnDiv(t, pages[0], 3)
	column1, column2, section := children[0], children[1], children[2]
	columnPosition(t, column1, 0, 0)
	columnPosition(t, column2, 4, 0)
	columnPosition(t, section, 0, 2)

	columnText(t, column1, "abc", 0, 0, 0)
	columnText(t, column1, "def", 0, 1, 0)
	columnText(t, column2, "ghi", 0, 0, 0)
	columnText(t, column2, "jkl", 0, 1, 0)
	columnText(t, section, "line1", 0, 0)

	children = columnDiv(t, pages[1], 3)
	section, column1, column2 = children[0], children[1], children[2]
	columnPosition(t, section, 0, 0)
	columnPosition(t, column1, 0, 1)
	columnPosition(t, column2, 4, 1)

	columnText(t, section, "line2", 0, 0)
	columnText(t, column1, "mno", 0, 0, 0)
	columnText(t, column2, "pqr", 0, 0, 0)
}

// TestColumnSpan4 is test_column_span_4.
func TestColumnSpan4(t *testing.T) {
	pages := pageRender(t, columnSpanStyle("8px 3px", "")+`
      <div>
        abc def
        <section>line1</section>
        ghi jkl
        mno pqr
      </div>
    `, 2)
	children := columnDiv(t, pages[0], 5)
	column1, column2, section, column3, column4 := children[0], children[1], children[2], children[3], children[4]
	columnPosition(t, column1, 0, 0)
	columnPosition(t, column2, 4, 0)
	columnPosition(t, section, 0, 1)
	columnPosition(t, column3, 0, 2)
	columnPosition(t, column4, 4, 2)

	columnText(t, column1, "abc", 0, 0, 0)
	columnText(t, column2, "def", 0, 0, 0)
	columnText(t, section, "line1", 0, 0)
	columnText(t, column3, "ghi", 0, 0, 0)
	columnText(t, column4, "jkl", 0, 0, 0)

	children = columnDiv(t, pages[1], 2)
	columnPosition(t, children[0], 0, 0)
	columnPosition(t, children[1], 4, 0)
	columnText(t, children[0], "mno", 0, 0, 0)
	columnText(t, children[1], "pqr", 0, 0, 0)
}

// TestColumnSpan5 is test_column_span_5.
func TestColumnSpan5(t *testing.T) {
	pages := pageRender(t, columnSpanStyle("8px 3px", "")+`
      <div>
        abc def
        ghi jkl
        <section>line1</section>
        mno pqr
      </div>
    `, 2)
	children := columnDiv(t, pages[0], 3)
	column1, column2, section := children[0], children[1], children[2]
	columnPosition(t, column1, 0, 0)
	columnPosition(t, column2, 4, 0)
	columnPosition(t, section, 0, 2)

	columnText(t, column1, "abc", 0, 0, 0)
	columnText(t, column1, "def", 0, 1, 0)
	columnText(t, column2, "ghi", 0, 0, 0)
	columnText(t, column2, "jkl", 0, 1, 0)
	columnText(t, section, "line1", 0, 0)

	children = columnDiv(t, pages[1], 2)
	columnPosition(t, children[0], 0, 0)
	columnPosition(t, children[1], 4, 0)
	columnText(t, children[0], "mno", 0, 0, 0)
	columnText(t, children[1], "pqr", 0, 0, 0)
}

// TestColumnSpan6 is test_column_span_6.
func TestColumnSpan6(t *testing.T) {
	pages := pageRender(t, columnSpanStyle("8px 3px", "")+`
      <div>
        abc def
        ghi jkl
        mno pqr
        <section>line1</section>
      </div>
    `, 2)
	children := columnDiv(t, pages[0], 2)
	column1, column2 := children[0], children[1]
	columnPosition(t, column1, 0, 0)
	columnPosition(t, column2, 4, 0)

	columnText(t, column1, "abc", 0, 0, 0)
	columnText(t, column1, "def", 0, 1, 0)
	columnText(t, column1, "ghi", 0, 2, 0)
	columnText(t, column2, "jkl", 0, 0, 0)
	columnText(t, column2, "mno", 0, 1, 0)
	columnText(t, column2, "pqr", 0, 2, 0)

	section := columnDiv(t, pages[1], 1)[0]
	columnText(t, section, "line1", 0, 0)
	columnPosition(t, section, 0, 0)
}

// TestColumnSpan7 is test_column_span_7.
func TestColumnSpan7(t *testing.T) {
	pages := pageRender(t, columnSpanStyle("8px 3px", "; font-size: 2px")+`
      <div>
        abc def
        ghi jkl
        <section>l1</section>
        mno pqr
      </div>
    `, 2)
	children := columnDiv(t, pages[0], 2)
	column1, column2 := children[0], children[1]
	columnPosition(t, column1, 0, 0)
	columnPosition(t, column2, 4, 0)

	columnText(t, column1, "abc", 0, 0, 0)
	columnText(t, column1, "def", 0, 1, 0)
	columnText(t, column2, "ghi", 0, 0, 0)
	columnText(t, column2, "jkl", 0, 1, 0)

	children = columnDiv(t, pages[1], 3)
	section, column1, column2 := children[0], children[1], children[2]
	columnPosition(t, section, 0, 0)
	columnPosition(t, column1, 0, 2)
	columnPosition(t, column2, 4, 2)

	columnText(t, section, "l1", 0, 0)
	columnText(t, column1, "mno", 0, 0, 0)
	columnText(t, column2, "pqr", 0, 0, 0)
}

// TestColumnSpan8 is test_column_span_8.
func TestColumnSpan8(t *testing.T) {
	pages := pageRender(t, columnSpanStyle("8px 2px", "")+`
      <div>
        abc def
        ghi jkl
        mno pqr
        <section>line1</section>
      </div>
    `, 2)
	children := columnDiv(t, pages[0], 2)
	column1, column2 := children[0], children[1]
	columnPosition(t, column1, 0, 0)
	columnPosition(t, column2, 4, 0)

	columnText(t, column1, "abc", 0, 0, 0)
	columnText(t, column1, "def", 0, 1, 0)
	columnText(t, column2, "ghi", 0, 0, 0)
	columnText(t, column2, "jkl", 0, 1, 0)

	children = columnDiv(t, pages[1], 3)
	column1, column2, section := children[0], children[1], children[2]
	columnPosition(t, column1, 0, 0)
	columnPosition(t, column2, 4, 0)
	columnPosition(t, section, 0, 1)

	columnText(t, column1, "mno", 0, 0, 0)
	columnText(t, column2, "pqr", 0, 0, 0)
	columnText(t, section, "line1", 0, 0)
}

// TestColumnSpan9 is test_column_span_9.
func TestColumnSpan9(t *testing.T) {
	page := pageRender(t, columnSpanStyle("8px 3px", "")+`
      <div>
        abc
        <section>line1</section>
        def ghi
      </div>
    `, 1)[0]
	children := columnDiv(t, page, 4)
	column1, section, column2, column3 := children[0], children[1], children[2], children[3]
	columnPosition(t, column1, 0, 0)
	columnPosition(t, section, 0, 1)
	columnPosition(t, column2, 0, 2)
	columnPosition(t, column3, 4, 2)

	columnText(t, column1, "abc", 0, 0, 0)
	columnText(t, section, "line1", 0, 0)
	columnText(t, column2, "def", 0, 0, 0)
	columnText(t, column3, "ghi", 0, 0, 0)
}

// TestColumnSpanBalance is test_column_span_balance.
func TestColumnSpanBalance(t *testing.T) {
	page := pageRender(t, `
      <style>
        @page { margin: 0; size: 8px 5px }
        body { font-family: weasyprint; font-size: 1px }
        div { columns: 2; column-gap: 0; line-height: 1; column-fill: auto }
        section { column-span: all }
      </style>
      <div>
        abc def
        <section>line1</section>
        ghi jkl
      </div>
    `, 1)[0]
	children := columnDiv(t, page, 4)
	column1, column2, section, column3 := children[0], children[1], children[2], children[3]
	columnPosition(t, column1, 0, 0)
	columnPosition(t, column2, 4, 0)
	columnPosition(t, section, 0, 1)
	columnPosition(t, column3, 0, 2)

	columnText(t, column1, "abc", 0, 0, 0)
	columnText(t, column2, "def", 0, 0, 0)
	columnText(t, section, "line1", 0, 0)
	columnText(t, column3, "ghi", 0, 0, 0)
	columnText(t, column3, "jkl", 0, 1, 0)
}

// columnMultipageStyle is the style of the tests of the columns on
// several pages, with the size of the page and more style.
func columnMultipageStyle(size, more string) string {
	return `
      <style>
        div { columns: 2; column-gap: 1px }
        body { margin: 0; font-family: weasyprint;
               font-size: 1px; line-height: 1px }
        @page { margin: 0; size: ` + size + ` }
        ` + more + `
      </style>`
}

// TestColumnsMultipage is test_columns_multipage.
func TestColumnsMultipage(t *testing.T) {
	pages := pageRender(t, columnMultipageStyle("3px 2px", "")+`
      <div>a b c d e f g</div>
    `, 2)
	columns := columnDiv(t, pages[0], 2)
	unpack(t, columns[0], 2)
	unpack(t, columns[1], 2)
	columnText(t, columns[0], "a", 0, 0)
	columnText(t, columns[0], "b", 1, 0)
	columnText(t, columns[1], "c", 0, 0)
	columnText(t, columns[1], "d", 1, 0)

	columns = columnDiv(t, pages[1], 2)
	unpack(t, columns[0], 2)
	unpack(t, columns[1], 1)
	columnText(t, columns[0], "e", 0, 0)
	columnText(t, columns[0], "f", 1, 0)
	columnText(t, columns[1], "g", 0, 0)
}

// TestColumnsMultipageSpan is test_columns_multipage_span.
func TestColumnsMultipageSpan(t *testing.T) {
	pages := pageRender(t, columnMultipageStyle("3px 2px", "section { column-span: all }")+`
      <div>
        a b
        <section>cc dd</section>
        e
      </div>
    `, 2)
	children := columnDiv(t, pages[0], 3)
	column1, column2, section := children[0], children[1], children[2]
	unpack(t, column1, 1)
	unpack(t, column2, 1)
	columnText(t, column1, "a", 0, 0, 0)
	columnText(t, column2, "b", 0, 0, 0)
	columnText(t, section, "cc", 0, 0)

	children = columnDiv(t, pages[1], 2)
	section, column1 = children[0], children[1]
	columnText(t, section, "dd", 0, 0)
	unpack(t, column1, 1)
	columnText(t, column1, "e", 0, 0, 0)
}

// TestColumnsMultipageSpanNested is test_columns_multipage_span_nested.
func TestColumnsMultipageSpanNested(t *testing.T) {
	pages := renderPages(t, columnMultipageStyle("3px 2px", "section { column-span: all }")+`
      <div>
        <section>
          <p>aa</p>
          <p>bb cc</p>
        </section>
        d e
      </div>
    `)
	if len(pages) != 2 {
		t.Fatalf("%d pages", len(pages))
	}
	section := columnDiv(t, pages[0], 1)[0]
	columnText(t, section, "aa", 0, 0, 0)
	columnText(t, section, "bb", 1, 0, 0)

	children := columnDiv(t, pages[1], 3)
	section, column1, column2 := children[0], children[1], children[2]
	columnText(t, section, "cc", 0, 0, 0)
	unpack(t, column1, 1)
	unpack(t, column2, 1)
	columnText(t, column1, "d", 0, 0, 0)
	columnText(t, column2, "e", 0, 0, 0)
}

// TestColumnsBreaks is test_columns_breaks.
func TestColumnsBreaks(t *testing.T) {
	pages := pageRender(t, columnMultipageStyle("3px 2px", "section { break-before: always; }")+`
      <div>a<section>b</section><section>c</section></div>
    `, 2)
	columns := columnDiv(t, pages[0], 2)
	unpack(t, columns[0], 1)
	unpack(t, columns[1], 1)
	columnText(t, columns[0], "a", 0, 0, 0)
	columnText(t, columns[1], "b", 0, 0, 0)

	columns = columnDiv(t, pages[1], 1)
	unpack(t, columns[0], 1)
	columnText(t, columns[0], "c", 0, 0, 0)
}

// columnBreakTest renders a page of two columns with a style for the
// section, and checks the texts at the paths of the columns.
func columnBreakTest(t *testing.T, section, html string, checks ...any) {
	t.Helper()
	page := pageRender(t, columnMultipageStyle("3px 10px", "section { "+section+" }")+html, 1)[0]
	columns := columnDiv(t, page, 2)
	columnChecks(t, columns, checks...)
}

// columnChecks checks texts in columns. checks are triples of the index
// of a column, the text and the path in the column.
func columnChecks(t *testing.T, columns []*Box, checks ...any) {
	t.Helper()
	for i := 0; i+2 < len(checks); i += 3 {
		columnText(t, columns[checks[i].(int)], checks[i+1].(string), checks[i+2].([]int)...)
	}
}

// TestColumnsBreakAfterColumn1 is test_columns_break_after_column_1.
func TestColumnsBreakAfterColumn1(t *testing.T) {
	columnBreakTest(t, "break-after: column", `<div>a b <section>c</section> d</div>`,
		0, "a", []int{0, 0, 0}, 0, "b", []int{0, 1, 0},
		0, "c", []int{1, 0, 0}, 1, "d", []int{0, 0, 0})
}

// TestColumnsBreakAfterColumn2 is test_columns_break_after_column_2.
func TestColumnsBreakAfterColumn2(t *testing.T) {
	columnBreakTest(t, "break-after: column", `<div><section>a</section> b c d</div>`,
		0, "a", []int{0, 0, 0}, 1, "b", []int{0, 0, 0},
		1, "c", []int{0, 1, 0}, 1, "d", []int{0, 2, 0})
}

// TestColumnsBreakAfterAvoidColumn is test_columns_break_after_avoid_column.
func TestColumnsBreakAfterAvoidColumn(t *testing.T) {
	columnBreakTest(t, "break-after: avoid-column", `<div>a <section>b</section> c d</div>`,
		0, "a", []int{0, 0, 0}, 0, "b", []int{1, 0, 0},
		0, "c", []int{2, 0, 0}, 1, "d", []int{0, 0, 0})
}

// TestColumnsBreakBeforeColumn1 is test_columns_break_before_column_1.
func TestColumnsBreakBeforeColumn1(t *testing.T) {
	columnBreakTest(t, "break-before: column", `<div>a b c <section>d</section></div>`,
		0, "a", []int{0, 0, 0}, 0, "b", []int{0, 1, 0},
		0, "c", []int{0, 2, 0}, 1, "d", []int{0, 0, 0})
}

// TestColumnsBreakBeforeColumn2 is test_columns_break_before_column_2.
func TestColumnsBreakBeforeColumn2(t *testing.T) {
	columnBreakTest(t, "break-before: column", `<div>a <section>b</section> c d</div>`,
		0, "a", []int{0, 0, 0}, 1, "b", []int{0, 0, 0},
		1, "c", []int{1, 0, 0}, 1, "d", []int{1, 1, 0})
}

// TestColumnsBreakBeforeAvoidColumn is
// test_columns_break_before_avoid_column.
func TestColumnsBreakBeforeAvoidColumn(t *testing.T) {
	columnBreakTest(t, "break-before: avoid-column", `<div>a b <section>c</section> d</div>`,
		0, "a", []int{0, 0, 0}, 0, "b", []int{0, 1, 0},
		0, "c", []int{1, 0, 0}, 1, "d", []int{0, 0, 0})
}

// TestColumnsBreakInsideColumn1 is test_columns_break_inside_column_1.
func TestColumnsBreakInsideColumn1(t *testing.T) {
	columnBreakTest(t, "break-inside: avoid-column", `<div><section>a b c</section> d</div>`,
		0, "a", []int{0, 0, 0}, 0, "b", []int{0, 1, 0},
		0, "c", []int{0, 2, 0}, 1, "d", []int{0, 0, 0})
}

// TestColumnsBreakInsideColumn2 is test_columns_break_inside_column_2.
func TestColumnsBreakInsideColumn2(t *testing.T) {
	columnBreakTest(t, "break-inside: avoid-column", `<div>a <section>b c d</section></div>`,
		0, "a", []int{0, 0, 0}, 1, "b", []int{0, 0, 0},
		1, "c", []int{0, 1, 0}, 1, "d", []int{0, 2, 0})
}

// TestColumnsBreakInsideColumnNotEmptyPage is
// test_columns_break_inside_column_not_empty_page.
func TestColumnsBreakInsideColumnNotEmptyPage(t *testing.T) {
	page := pageRender(t, columnMultipageStyle("3px 10px", "section { break-inside: avoid-column }")+`
      <p>p</p>
      <div><section>a b c</section> d</div>
    `, 1)[0]
	_, body := pageBody(t, page)
	children := unpack(t, body, 2)
	columnText(t, children[0], "p", 0, 0)
	columns := unpack(t, children[1], 2)
	columnChecks(t, columns,
		0, "a", []int{0, 0, 0}, 0, "b", []int{0, 1, 0},
		0, "c", []int{0, 2, 0}, 1, "d", []int{0, 0, 0})
}

// columnGeometry checks the widths, the heights and the positions of
// columns. A nil list is not checked.
func columnGeometry(t *testing.T, columns []*Box, widths, heights, xs, ys []float64) {
	t.Helper()
	for i, column := range columns {
		if widths != nil {
			pageEqual(t, "width", column.Width, widths[i])
		}
		if heights != nil {
			pageEqual(t, "height", column.Height, heights[i])
		}
		if xs != nil {
			pageEqual(t, "x", column.PositionX, xs[i])
		}
		if ys != nil {
			pageEqual(t, "y", column.PositionY, ys[i])
		}
	}
}

// TestColumnsNotEnoughContent is test_columns_not_enough_content.
func TestColumnsNotEnoughContent(t *testing.T) {
	page := pageRender(t, `
      <style>
        div { columns: 5; column-gap: 0 }
        body { margin: 0; font-family: weasyprint; font-size: 1px }
        @page { margin: 0; size: 5px }
      </style>
      <div>a b c</div>
    `, 1)[0]
	_, body := pageBody(t, page)
	div := only(t, body)
	pageEqual(t, "div width", div.Width, 5)
	columns := unpack(t, div, 3)
	columnGeometry(t, columns, []float64{1, 1, 1}, nil, []float64{0, 1, 2}, []float64{0, 0, 0})
}

// TestColumnsHigherThanPage is test_columns_higher_than_page.
func TestColumnsHigherThanPage(t *testing.T) {
	pages := pageRender(t, `
      <style>
        div { columns: 5; column-gap: 0 }
        body { margin: 0; font-family: weasyprint; font-size: 2px }
        @page { margin: 0; size: 5px 1px }
      </style>
      <div>a b c d e f g h</div>
    `, 2)
	_, body := pageBody(t, pages[0])
	div := only(t, body)
	pageEqual(t, "div width", div.Width, 5)
	columns := unpack(t, div, 5)
	for i, text := range []string{"a", "b", "c", "d", "e"} {
		columnText(t, columns[i], text, 0, 0)
	}

	_, body = pageBody(t, pages[1])
	div = only(t, body)
	pageEqual(t, "div width", div.Width, 5)
	columns = unpack(t, div, 3)
	for i, text := range []string{"f", "g", "h"} {
		columnText(t, columns[i], text, 0, 0)
	}
}

// TestColumnsEmpty is test_columns_empty.
func TestColumnsEmpty(t *testing.T) {
	page := pageRender(t, `
      <style>
        div { columns: 3 }
        body { margin: 0; font-family: weasyprint }
        @page { margin: 0; size: 3px; font-size: 1px }
      </style>
      <div></div>
    `, 1)[0]
	_, body := pageBody(t, page)
	div := only(t, body)
	pageEqual(t, "div width", div.Width, 3)
	pageEqual(t, "div height", div.Height, 0)
	unpack(t, div, 0)
}

// TestColumnsFixedHeight is test_columns_fixed_height.
func TestColumnsFixedHeight(t *testing.T) {
	// TODO: we should test when the height is too small
	for _, prop := range []string{"height", "min-height"} {
		t.Run(prop, func(t *testing.T) {
			page := pageRender(t, `
      <style>
        div { columns: 4; column-gap: 0; `+prop+`: 10px }
        body { margin: 0; font-family: weasyprint; line-height: 1px }
        @page { margin: 0; size: 4px 50px; font-size: 1px }
      </style>
      <div>a b c</div>
    `, 1)[0]
			_, body := pageBody(t, page)
			div := only(t, body)
			pageEqual(t, "div width", div.Width, 4)
			columns := unpack(t, div, 3)
			columnGeometry(t, columns, []float64{1, 1, 1}, []float64{10, 10, 10},
				[]float64{0, 1, 2}, []float64{0, 0, 0})
		})
	}
}

// TestColumnsPadding is test_columns_padding.
func TestColumnsPadding(t *testing.T) {
	page := pageRender(t, `
      <style>
        div { columns: 4; column-gap: 0; padding: 1px }
        body { margin: 0; font-family: weasyprint; line-height: 1px }
        @page { margin: 0; size: 6px 50px; font-size: 1px }
      </style>
      <div>a b c</div>
    `, 1)[0]
	_, body := pageBody(t, page)
	div := only(t, body)
	pageEqual(t, "div width", div.Width, 4)
	pageEqual(t, "div height", div.Height, 1)
	pageEqual(t, "div padding width", div.PaddingWidth(), 6)
	pageEqual(t, "div padding height", div.PaddingHeight(), 3)
	columns := unpack(t, div, 3)
	columnGeometry(t, columns, []float64{1, 1, 1}, []float64{1, 1, 1},
		[]float64{1, 2, 3}, []float64{1, 1, 1})
}

// TestColumnsBottomMargin is test_columns_bottom_margin.
func TestColumnsBottomMargin(t *testing.T) {
	page := pageRender(t, `
      <style>
        div { columns: 2; column-gap: 1px }
        body { margin: 0; font: 2px / 1 weasyprint }
        p { margin-bottom: 1em }
        @page { margin: 0; size: 5px 7px; font-size: 1px }
      </style>
      <div><p>a b c</p></div>
    `, 1)[0]
	columns := columnDiv(t, page, 2)
	p := only(t, columns[0])
	lines := unpack(t, p, 2)
	columnText(t, lines[0], "a", 0)
	columnText(t, lines[1], "b", 0)
	p = only(t, columns[1])
	columnText(t, only(t, p), "c", 0)
}

// TestColumnsRelative is test_columns_relative.
func TestColumnsRelative(t *testing.T) {
	page := pageRender(t, `
      <style>
        article { position: absolute; top: 3px }
        div { columns: 4; column-gap: 0; position: relative;
              top: 1px; left: 2px }
        body { margin: 0; font-family: weasyprint; line-height: 1px }
        @page { margin: 0; size: 4px 50px; font-size: 1px }
      </style>
      <div>a b c d<article>e</article></div>
    `, 1)[0]
	_, body := pageBody(t, page)
	div := only(t, body)
	pageEqual(t, "div width", div.Width, 4)
	columns := div.Children
	if len(columns) != 4 {
		t.Fatalf("%d columns", len(columns))
	}
	columnGeometry(t, columns, []float64{1, 1, 1, 1}, nil,
		[]float64{2, 3, 4, 5}, []float64{1, 1, 1, 1})
	column4 := columns[3]
	columnLine := only(t, column4)
	absoluteArticle := unpack(t, columnLine, 2)[1]
	absoluteLine := only(t, absoluteArticle)
	span := only(t, absoluteLine)
	pageEqual(t, "span x", span.PositionX, 5) // Default position of the 4th column
	pageEqual(t, "span y", span.PositionY, 4) // div's 1px + span's 3px
}

// TestColumnsRegression1 is test_columns_regression_1.
func TestColumnsRegression1(t *testing.T) {
	// Regression test for #659.
	pages := pageRender(t, `
      <style>
        @page {margin: 0; width: 100px; height: 100px}
        body {margin: 0; font-size: 1px}
      </style>
      <div style="height:95px">A</div>
      <div style="column-count:2">
        <div style="height:20px">B1</div>
        <div style="height:20px">B2</div>
        <div style="height:20px">B3</div>
      </div>
      <div style="height:95px">C</div>
    `, 3)

	_, body := pageBody(t, pages[0])
	div := only(t, body)
	pageEqual(t, "div y", div.PositionY, 0)
	columnText(t, div, "A", 0, 0)

	_, body = pageBody(t, pages[1])
	div = only(t, body)
	pageEqual(t, "div y", div.PositionY, 0)
	columns := unpack(t, div, 2)
	pageEqual(t, "column 1 y", columns[0].PositionY, 0)
	pageEqual(t, "column 2 y", columns[1].PositionY, 0)
	divs := unpack(t, columns[0], 2)
	div3 := only(t, columns[1])
	pageEqual(t, "div 1 y", divs[0].PositionY, 0)
	pageEqual(t, "div 3 y", div3.PositionY, 0)
	pageEqual(t, "div 2 y", divs[1].PositionY, 20)
	columnText(t, divs[0], "B1", 0, 0)
	columnText(t, divs[1], "B2", 0, 0)
	columnText(t, div3, "B3", 0, 0)

	_, body = pageBody(t, pages[2])
	div = only(t, body)
	pageEqual(t, "div y", div.PositionY, 0)
	columnText(t, div, "C", 0, 0)
}

// TestColumnsRegression2 is test_columns_regression_2.
func TestColumnsRegression2(t *testing.T) {
	// Regression test for #659.
	pages := pageRender(t, `
      <style>
        @page {margin: 0; width: 100px; height: 100px}
        body {margin: 0; font-size: 1px}
      </style>
      <div style="column-count:2">
        <div style="height:20px">B1</div>
        <div style="height:60px">B2</div>
        <div style="height:60px">B3</div>
        <div style="height:60px">B4</div>
      </div>
    `, 2)

	_, body := pageBody(t, pages[0])
	div := only(t, body)
	pageEqual(t, "div y", div.PositionY, 0)
	columns := unpack(t, div, 2)
	pageEqual(t, "column 1 y", columns[0].PositionY, 0)
	pageEqual(t, "column 2 y", columns[1].PositionY, 0)
	divs := unpack(t, columns[0], 2)
	div3 := only(t, columns[1])
	pageEqual(t, "div 1 y", divs[0].PositionY, 0)
	pageEqual(t, "div 3 y", div3.PositionY, 0)
	pageEqual(t, "div 2 y", divs[1].PositionY, 20)
	columnText(t, divs[0], "B1", 0, 0)
	columnText(t, divs[1], "B2", 0, 0)
	columnText(t, div3, "B3", 0, 0)

	_, body = pageBody(t, pages[1])
	div = only(t, body)
	pageEqual(t, "div y", div.PositionY, 0)
	column1 := only(t, div)
	pageEqual(t, "column 1 y", column1.PositionY, 0)
	div1 := only(t, column1)
	pageEqual(t, "div 1 y", div1.PositionY, 0)
	columnText(t, div1, "B4", 0, 0)
}

// TestColumnsRegression3 is test_columns_regression_3.
func TestColumnsRegression3(t *testing.T) {
	// Regression test for #659.
	page := pageRender(t, `
      <style>
        @page {margin: 0; width: 100px; height: 100px}
        body {margin: 0; font-size: 10px}
      </style>
      <div style="column-count:2">
        <div style="height:20px; margin:5px">B1</div>
        <div style="height:60px">B2</div>
        <div style="height:60px">B3</div>
      </div>
    `, 1)[0]
	_, body := pageBody(t, page)
	div := only(t, body)
	pageEqual(t, "div y", div.PositionY, 0)
	columns := unpack(t, div, 2)
	pageEqual(t, "column 1 y", columns[0].PositionY, 0)
	pageEqual(t, "column 2 y", columns[1].PositionY, 0)
	divs := unpack(t, columns[0], 2)
	div3 := only(t, columns[1])
	pageEqual(t, "div 1 y", divs[0].PositionY, 0)
	pageEqual(t, "div 3 y", div3.PositionY, 0)
	pageEqual(t, "div 2 y", divs[1].PositionY, 30)
	pageEqual(t, "div height", div.Height, 5+20+5+60)
	columnText(t, divs[0], "B1", 0, 0)
	columnText(t, divs[1], "B2", 0, 0)
	columnText(t, div3, "B3", 0, 0)
}

// TestColumnsRegression4 is test_columns_regression_4.
func TestColumnsRegression4(t *testing.T) {
	// Regression test for #897.
	page := pageRender(t, `
      <div style="position:absolute">
        <div style="column-count:2">
          <div>a</div>
        </div>
      </div>
    `, 1)[0]
	_, body := pageBody(t, page)
	div := only(t, body)
	pageEqual(t, "div y", div.PositionY, 0)
	column1 := only(t, div)
	pageEqual(t, "column 1 y", column1.PositionY, 0)
	pageEqual(t, "div 1 y", only(t, column1).PositionY, 0)
}

// TestColumnsRegression5 is test_columns_regression_5.
func TestColumnsRegression5(t *testing.T) {
	// Regression test for #1191.
	pageRender(t, `
      <style>
        @page {width: 100px; height: 100px}
      </style>
      <div style="height: 1px"></div>
      <div style="columns: 2">
        <div style="break-after: avoid">
          <div style="height: 50px"></div>
        </div>
        <div style="break-after: avoid">
          <div style="height: 50px"></div>
          <p>a</p>
        </div>
      </div>
      <div style="height: 50px"></div>
    `, -1)
}

// TestColumnsRegression6 is test_columns_regression_6.
func TestColumnsRegression6(t *testing.T) {
	// Regression test for #2103.
	pageRender(t, `
      <style>
        @page {width: 100px; height: 100px}
      </style>
      <div style="columns: 2; column-width: 100px; width: 10px">abc def</div>
    `, -1)
}

// TestColumnsRegression7 is test_columns_regression_7.
func TestColumnsRegression7(t *testing.T) {
	pages := pageRender(t, `
      <style>
        @page { size: 50px 10px }
        body { font-size: 2px; line-height: 1 }
      </style>
      <div style="height: 8px"></div>
      <div style="column-count: 2">
        <div>a</div>
        <div style="break-inside: avoid">b<br>c<br>d</div>
      </div>
    `, 2)
	_, body := pageBody(t, pages[0])
	div := only(t, body)
	pageEqual(t, "div y", div.PositionY, 0)
	unpack(t, div, 0)
	columns := columnDiv(t, pages[1], 2)
	pageEqual(t, "column 1 y", columns[0].PositionY, 0)
	pageEqual(t, "column 2 y", columns[1].PositionY, 0)
}

// TestColumnsMarginTop is test_columns_margin_top.
func TestColumnsMarginTop(t *testing.T) {
	page := pageRender(t, `
      <style>
        @page { size: 50px 10px }
        body { font-size: 2px; line-height: 1 }
      </style>
      <div style="column-count: 2; margin-top: 1em">
        a<br>b
      </div>
    `, 1)[0]
	_, body := pageBody(t, page)
	div := only(t, body)
	pageEqual(t, "div y", div.PositionY, 0)
	columns := unpack(t, div, 2)
	for i, column := range columns {
		pageEqual(t, fmt.Sprintf("column %d y", i+1), column.PositionY, 2)
		pageEqual(t, fmt.Sprintf("column %d child y", i+1), columnChild(t, column, 0).PositionY, 2)
	}
}

// TestColumnsGrid is test_columns_grid.
func TestColumnsGrid(t *testing.T) {
	// Regression test for #2680.
	page := pageRender(t, `
      <div style="columns: 2">
        <div>1</div>
        <div style="display: grid">2</div>
      </div>
    `, 1)[0]
	columnDiv(t, page, 2)
}

// TestColumnsFirstLetter is test_columns_first_letter.
func TestColumnsFirstLetter(t *testing.T) {
	// Regression test for #2717.
	page := pageRender(t, `
      <style>
        h1:first-letter { font-size: 120% }
        div { columns: 2 }
      </style>
      <div>
        <h1>test</h1>
        <h1>test</h1>
      </div>
    `, 1)[0]
	columns := columnDiv(t, page, 2)
	pageTag(t, only(t, columns[0]), "h1")
	pageTag(t, only(t, columns[1]), "h1")
}
