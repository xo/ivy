// Copyright (c) 2011-2021, Simon Sapin and contributors.
//
// Use of this source code is governed by the BSD-3-Clause license that can be
// found in the LICENSE file.

// This file ports the second half of tests/layout/test_table.py: the page
// breaks of the tables, the captions, the baselines and the borders.

package layout

import (
	"fmt"
	"reflect"
	"testing"
)

// tableGroupTexts returns the text of the only cell of each row of each
// group of a table. It fails the test for an empty group, and for a cell
// that does not hold one line of one text box.
func tableGroupTexts(t *testing.T, table *Box) [][]string {
	t.Helper()
	groups := make([][]string, 0, len(table.Children))
	for _, group := range table.Children {
		if len(group.Children) == 0 {
			t.Fatal("found an empty table group")
		}
		rows := []string{}
		for _, row := range group.Children {
			cell := only(t, row)
			line := only(t, cell)
			text := only(t, line)
			rows = append(rows, text.Text)
		}
		groups = append(groups, rows)
	}
	return groups
}

// tableCheckTexts fails the test when the texts of the pages are not want.
func tableCheckTexts(t *testing.T, got, want [][][]string) {
	t.Helper()
	if fmt.Sprint(got) != fmt.Sprint(want) {
		t.Errorf("rows per page = %v, want %v", got, want)
	}
}

func TestTablePageBreaks(t *testing.T) {
	for i, c := range []struct {
		html      string
		rows      []int
		positions []float64
	}{
		{`
      <style>
        @page { size: 120px }
        table { table-layout: fixed; width: 100% }
        h1 { height: 30px }
        td { height: 40px }
      </style>
      <h1>Dummy title</h1>
      <table>
        <tr><td>row 1</td></tr>
        <tr><td>row 2</td></tr>

        <tr><td>row 3</td></tr>
        <tr><td>row 4</td></tr>
        <tr><td>row 5</td></tr>

        <tr><td style="height: 300px"> <!-- overflow the page -->
          row 6</td></tr>
        <tr><td>row 7</td></tr>
        <tr><td>row 8</td></tr>
      </table>
     `,
			[]int{2, 3, 1, 2},
			[]float64{30, 70, 0, 40, 80, 0, 0, 40}},
		{`
      <style>
        @page { size: 120px }
        h1 { height: 30px}
        td { height: 40px }
        table { table-layout: fixed; width: 100%;
                page-break-inside: avoid }
      </style>
      <h1>Dummy title</h1>
      <table>
        <tr><td>row 1</td></tr>
        <tr><td>row 2</td></tr>
        <tr><td>row 3</td></tr>
        <tr><td>row 4</td></tr>
     </table>
     `,
			[]int{0, 3, 1},
			[]float64{0, 40, 80, 0}},
		{`
      <style>
        @page { size: 120px }
        h1 { height: 30px}
        td { height: 40px }
        table { table-layout: fixed; width: 100%;
                page-break-inside: avoid }
      </style>
      <h1>Dummy title</h1>
      <table>
        <tbody>
          <tr><td>row 1</td></tr>
          <tr><td>row 2</td></tr>
          <tr><td>row 3</td></tr>
        </tbody>

        <tr><td>row 4</td></tr>
      </table>
     `,
			[]int{0, 3, 1},
			[]float64{0, 40, 80, 0}},
		{`
      <style>
        @page { size: 120px }
        h1 { height: 30px}
        td { height: 40px }
        table { table-layout: fixed; width: 100% }
      </style>
      <h1>Dummy title</h1>
      <table>
        <tr><td>row 1</td></tr>

        <tbody style="page-break-inside: avoid">
          <tr><td>row 2</td></tr>
          <tr><td>row 3</td></tr>
        </tbody>
      </table>
     `,
			[]int{1, 2},
			[]float64{30, 0, 40}},
		{`
      <style>
        @page { size: 120px }
        h1 { height: 30px}
        td { line-height: 40px }
        table { table-layout: fixed; width: 100% }
      </style>
      <h1>Dummy title</h1>
      <table>
        <tr><td>r1l1</td></tr>
        <tr style="break-inside: avoid"><td>r2l1<br>r2l2</td></tr>
        <tr><td>r3l1</td></tr>
      </table>
     `,
			[]int{1, 2},
			[]float64{30, 0, 80}},
		{`
      <style>
        @page { size: 120px }
        h1 { height: 30px}
        td { line-height: 40px }
        table { table-layout: fixed; width: 100% }
      </style>
      <h1>Dummy title</h1>
      <table>
        <tbody>
          <tr><td>r1l1</td></tr>
          <tr style="break-inside: avoid"><td>r2l1<br>r2l2</td></tr>
        </tbody>
        <tr><td>r3l1</td></tr>
      </table>
     `,
			[]int{1, 2},
			[]float64{30, 0, 80}},
		{`
      <style>
        @page { size: 100px }
        h1 { height: 30px }
        td { line-height: 40px }
        table { table-layout: fixed; width: 100% }
        thead { display: table-row-group; break-after: avoid }
        thead tr { break-after: avoid }
      </style>
      <h1>Dummy title</h1>
      <table>
        <thead>
          <tr><td>r1l1</td></tr>
        </thead>
        <tbody>
          <tr><td>r2l1</td></tr>
        </tbody>
      </table>
     `,
			[]int{0, 2},
			[]float64{0, 40}},
	} {
		t.Run(fmt.Sprint(i), func(t *testing.T) {
			pages := tablePages(t, c.html)
			var rowsPerPage []int
			var rowsPositionY []float64
			for i, page := range pages {
				_, body := pageBody(t, page)
				bodyChildren := body.Children
				if i == 0 {
					bodyChildren = bodyChildren[1:] // skip h1
				}
				if len(bodyChildren) == 0 {
					rowsPerPage = append(rowsPerPage, 0)
					continue
				}
				if len(bodyChildren) != 1 {
					t.Fatalf("the body of page %d holds %d boxes, want 1", i, len(bodyChildren))
				}
				table := only(t, bodyChildren[0])
				rowsInThisPage := 0
				for _, group := range table.Children {
					if len(group.Children) == 0 {
						t.Fatal("found an empty table group")
					}
					for _, row := range group.Children {
						rowsInThisPage++
						rowsPositionY = append(rowsPositionY, row.PositionY)
						only(t, row)
					}
				}
				rowsPerPage = append(rowsPerPage, rowsInThisPage)
			}
			if !reflect.DeepEqual(rowsPerPage, c.rows) {
				t.Errorf("rows per page = %v, want %v", rowsPerPage, c.rows)
			}
			tableFloats(t, "rows_position_y", rowsPositionY, c.positions)
		})
	}
}

func TestTablePageBreaksInCell(t *testing.T) {
	pages := tablePages(t, `
      <style>
        @page { size: 120px }
        h1 { height: 30px}
        td { line-height: 40px }
        table { table-layout: fixed; width: 100% }
      </style>
      <h1>Dummy title</h1>
      <table>
        <tr><td>r1c1l1</td><td>r1c2l1</td></tr>
        <tr><td>r2c1l1</td><td style="break-inside: avoid">r2c2l1<br>r2c2l2</td></tr>
        <tr><td>r3c1l1</td><td>r3l1</td></tr>
      </table>
    `)
	if len(pages) != 2 {
		t.Fatalf("got %d pages, want 2", len(pages))
	}
	_, body := pageBody(t, pages[0])
	tableWrapper := unpack(t, body, 2)[1]
	row := only(t, only(t, only(t, tableWrapper)))
	unpack(t, row, 2)

	_, body = pageBody(t, pages[1])
	rows := unpack(t, only(t, only(t, only(t, body))), 2)
	unpack(t, rows[0], 2)
	unpack(t, rows[1], 2)
}

func TestTablePageBreaksComplex1(t *testing.T) {
	pages := tablePages(t, `
      <style>
        @page { size: 100px }
      </style>
      <h1 style="margin: 0; height: 30px">Lipsum</h1>
      <!-- Leave 70px on the first page: enough for the header or row1
           but not both.  -->
      <table style="border-spacing: 0; font-size: 5px">
        <thead>
          <tr><td style="height: 20px">Header</td></tr>
        </thead>
        <tbody>
          <tr><td style="height: 60px">Row 1</td></tr>
          <tr><td style="height: 10px">Row 2</td></tr>
          <tr><td style="height: 50px">Row 3</td></tr>
          <tr><td style="height: 61px">Row 4</td></tr>
          <tr><td style="height: 90px">Row 5</td></tr>
        </tbody>
        <tfoot>
          <tr><td style="height: 20px">Footer</td></tr>
        </tfoot>
      </table>
    `)
	var rowsPerPage [][][]string
	for i, page := range pages {
		_, body := pageBody(t, page)
		tableWrapper := only(t, body)
		if i == 0 {
			if tableWrapper.ElementTag != "h1" {
				t.Errorf("the first page holds %s, want h1", tableWrapper.ElementTag)
			}
			rowsPerPage = append(rowsPerPage, nil)
			continue
		}
		rowsPerPage = append(rowsPerPage, tableGroupTexts(t, only(t, tableWrapper)))
	}
	tableCheckTexts(t, rowsPerPage, [][][]string{
		{},
		{{"Header"}, {"Row 1"}, {"Footer"}},
		{{"Header"}, {"Row 2", "Row 3"}, {"Footer"}},
		{{"Header"}, {"Row 4"}},
		{{"Row 5"}},
	})
}

func TestTablePageBreaksComplex2(t *testing.T) {
	pages := tablePages(t, `
      <style>
        @page { size: 250px }
        td { height: 40px }
        table { table-layout: fixed; width: 100%; break-before: avoid }
      </style>
      <table>
        <thead>
          <tr><td>head 1</td></tr>
        </thead>
        <tbody>
          <tr><td>row 1 1</td></tr>
          <tr><td>row 1 2</td></tr>
          <tr><td>row 1 3</td></tr>
        </tbody>
        <tfoot>
          <tr><td>foot 1</td></tr>
        </tfoot>
      </table>
      <table>
        <thead>
          <tr><td>head 2</td></tr>
        </thead>
        <tbody>
          <tr><td>row 2 1</td></tr>
          <tr><td>row 2 2</td></tr>
          <tr><td>row 2 3</td></tr>
        </tbody>
        <tfoot>
          <tr><td>foot 2</td></tr>
        </tfoot>
      </table>
     `)
	rowsPerPage := make([][][]string, 0, len(pages))
	for _, page := range pages {
		_, body := pageBody(t, page)
		var groups [][]string
		for _, tableWrapper := range body.Children {
			groups = append(groups, tableGroupTexts(t, only(t, tableWrapper))...)
		}
		rowsPerPage = append(rowsPerPage, groups)
	}
	tableCheckTexts(t, rowsPerPage, [][][]string{
		{{"head 1"}, {"row 1 1", "row 1 2"}, {"foot 1"}},
		{{"head 1"}, {"row 1 3"}, {"foot 1"},
			{"head 2"}, {"row 2 1"}, {"foot 2"}},
		{{"head 2"}, {"row 2 2", "row 2 3"}, {"foot 2"}},
	})
	// TODO: test positions, the place of footer on the first page is
	// wrong.
}

// tablePagesN lays out a document and fails the test when it does not
// have n pages.
func tablePagesN(t *testing.T, source string, n int) []*Box {
	t.Helper()
	pages := tablePages(t, source)
	if len(pages) != n {
		t.Fatalf("got %d pages, want %d", len(pages), n)
	}
	return pages
}

// tableGroupSizes returns the number of rows of each group of the table of
// the table wrapper.
func tableGroupSizes(t *testing.T, tableWrapper *Box) []int {
	t.Helper()
	table := only(t, tableWrapper)
	sizes := make([]int, 0, len(table.Children))
	for _, group := range table.Children {
		sizes = append(sizes, len(group.Children))
	}
	return sizes
}

// tableCheckSizes fails the test when the sizes are not want.
func tableCheckSizes(t *testing.T, name string, got []int, want ...int) {
	t.Helper()
	if !reflect.DeepEqual(got, want) {
		t.Errorf("%s = %v, want %v", name, got, want)
	}
}

func TestTablePageBreakAfter(t *testing.T) {
	pages := tablePagesN(t, `
      <style>
        @page { size: 1000px }
        h1 { height: 30px}
        td { height: 40px }
        table { table-layout: fixed; width: 100% }
      </style>
      <h1>Dummy title</h1>
      <table>

        <tbody>
          <tr><td>row 1</td></tr>
          <tr><td>row 2</td></tr>
          <tr><td>row 3</td></tr>
        </tbody>
        <tbody>
          <tr style="break-after: page"><td>row 1</td></tr>
          <tr><td>row 2</td></tr>
          <tr><td>row 3</td></tr>
        </tbody>
        <tbody>
          <tr><td>row 1</td></tr>
          <tr><td>row 2</td></tr>
          <tr style="break-after: page"><td>row 3</td></tr>
        </tbody>
        <tbody style="break-after: right">
          <tr><td>row 1</td></tr>
          <tr><td>row 2</td></tr>
          <tr><td>row 3</td></tr>
        </tbody>
        <tbody style="break-after: page">
          <tr><td>row 1</td></tr>
          <tr><td>row 2</td></tr>
          <tr><td>row 3</td></tr>
        </tbody>

      </table>
      <p>bla bla</p>
     `, 6)
	_, body := pageBody(t, pages[0])
	tableCheckSizes(t, "page 1", tableGroupSizes(t, unpack(t, body, 2)[1]), 3, 1)
	_, body = pageBody(t, pages[1])
	tableCheckSizes(t, "page 2", tableGroupSizes(t, only(t, body)), 2, 3)
	_, body = pageBody(t, pages[2])
	tableCheckSizes(t, "page 3", tableGroupSizes(t, only(t, body)), 3)
	html := only(t, pages[3])
	unpack(t, html, 0)
	_, body = pageBody(t, pages[4])
	tableCheckSizes(t, "page 5", tableGroupSizes(t, only(t, body)), 3)
	_, body = pageBody(t, pages[5])
	if p := only(t, body); p.ElementTag != "p" {
		t.Errorf("page 6 holds %s, want p", p.ElementTag)
	}
}

func TestTablePageBreakBefore(t *testing.T) {
	pages := tablePagesN(t, `
      <style>
        @page { size: 1000px }
        h1 { height: 30px}
        td { height: 40px }
        table { table-layout: fixed; width: 100% }
      </style>
      <h1>Dummy title</h1>
      <table>

        <tbody>
          <tr style="break-before: page"><td>row 1</td></tr>
          <tr><td>row 2</td></tr>
          <tr><td>row 3</td></tr>
        </tbody>
        <tbody>
          <tr><td>row 1</td></tr>
          <tr style="break-before: page"><td>row 2</td></tr>
          <tr><td>row 3</td></tr>
        </tbody>
        <tbody>
          <tr style="break-before: page"><td>row 1</td></tr>
          <tr><td>row 2</td></tr>
          <tr><td>row 3</td></tr>
        </tbody>
        <tbody>
          <tr><td>row 1</td></tr>
          <tr><td>row 2</td></tr>
          <tr><td>row 3</td></tr>
        </tbody>
        <tbody style="break-before: left">
          <tr><td>row 1</td></tr>
          <tr><td>row 2</td></tr>
          <tr><td>row 3</td></tr>
        </tbody>

      </table>
      <p>bla bla</p>
     `, 6)
	_, body := pageBody(t, pages[0])
	if h1 := only(t, body); h1.ElementTag != "h1" {
		t.Errorf("page 1 holds %s, want h1", h1.ElementTag)
	}
	_, body = pageBody(t, pages[1])
	tableCheckSizes(t, "page 2", tableGroupSizes(t, only(t, body)), 3, 1)
	_, body = pageBody(t, pages[2])
	tableCheckSizes(t, "page 3", tableGroupSizes(t, only(t, body)), 2)
	_, body = pageBody(t, pages[3])
	tableCheckSizes(t, "page 4", tableGroupSizes(t, only(t, body)), 3, 3)
	html := only(t, pages[4])
	unpack(t, html, 0)
	_, body = pageBody(t, pages[5])
	c := unpack(t, body, 2)
	tableCheckSizes(t, "page 6", tableGroupSizes(t, c[0]), 3)
	if c[1].ElementTag != "p" {
		t.Errorf("page 6 holds %s, want p", c[1].ElementTag)
	}
}

func TestTablePageBreakAvoid(t *testing.T) {
	const style26 = `
      <style>
        @page { size: 100px }
        table { table-layout: fixed; width: 100% }
        tr { height: 26px }
      </style>`
	const style30 = `
      <style>
        @page { size: 100px }
        table { table-layout: fixed; width: 100% }
        tr { height: 30px }
      </style>`
	for i, c := range []struct {
		html string
		rows []int
	}{
		{style26 + `
      <table>
        <tbody>
          <tr><td>row 0</td></tr>
          <tr><td>row 1</td></tr>
          <tr style="break-before: avoid"><td>row 2</td></tr>
          <tr style="break-before: avoid"><td>row 3</td></tr>
        </tbody>
      </table>
    `, []int{1, 3}},
		{style26 + `
      <table>
        <tbody>
          <tr><td>row 0</td></tr>
          <tr style="break-after: avoid"><td>row 1</td></tr>
          <tr><td>row 2</td></tr>
          <tr style="break-before: avoid"><td>row 3</td></tr>
        </tbody>
      </table>
     `, []int{1, 3}},
		{style26 + `
      <table>
        <tbody>
          <tr><td>row 0</td></tr>
          <tr><td>row 1</td></tr>
          <tr style="break-after: avoid"><td>row 2</td></tr>
          <tr><td>row 3</td></tr>
        </tbody>
      </table>
     `, []int{2, 2}},
		{style26 + `
      <table>
        <tbody>
          <tr style="break-before: avoid"><td>row 0</td></tr>
          <tr style="break-before: avoid"><td>row 1</td></tr>
          <tr style="break-before: avoid"><td>row 2</td></tr>
          <tr style="break-before: avoid"><td>row 3</td></tr>
        </tbody>
      </table>
     `, []int{3, 1}},
		{style26 + `
      <table>
        <tbody>
          <tr style="break-after: avoid"><td>row 0</td></tr>
          <tr style="break-after: avoid"><td>row 1</td></tr>
          <tr style="break-after: avoid"><td>row 2</td></tr>
          <tr style="break-after: avoid"><td>row 3</td></tr>
        </tbody>
      </table>
     `, []int{3, 1}},
		{`
      <style>
        @page { size: 100px }
        table { table-layout: fixed; width: 100% }
        tr { height: 26px }
        p { height: 26px }
      </style>
      <p>wow p</p>
      <table>
        <tbody>
          <tr style="break-after: avoid"><td>row 0</td></tr>
          <tr style="break-after: avoid"><td>row 1</td></tr>
          <tr style="break-after: avoid"><td>row 2</td></tr>
          <tr style="break-after: avoid"><td>row 3</td></tr>
        </tbody>
      </table>
     `, []int{1, 3, 1}},
		{style30 + `
      <table>
        <tbody style="break-after: avoid">
          <tr><td>row 0</td></tr>
          <tr><td>row 1</td></tr>
          <tr><td>row 2</td></tr>
        </tbody>
        <tbody>
          <tr><td>row 0</td></tr>
          <tr><td>row 1</td></tr>
          <tr><td>row 2</td></tr>
        </tbody>
      </table>
     `, []int{2, 3, 1}},
		{style30 + `
      <table>
        <tbody>
          <tr><td>row 0</td></tr>
          <tr><td>row 1</td></tr>
          <tr><td>row 2</td></tr>
        </tbody>
        <tbody style="break-before: avoid">
          <tr><td>row 0</td></tr>
          <tr><td>row 1</td></tr>
          <tr><td>row 2</td></tr>
        </tbody>
      </table>
     `, []int{2, 3, 1}},
		{style30 + `
      <table>
        <tbody>
          <tr><td>row 0</td></tr>
          <tr><td>row 1</td></tr>
          <tr><td>row 2</td></tr>
        </tbody>
        <tbody>
          <tr style="break-before: avoid"><td>row 0</td></tr>
          <tr><td>row 1</td></tr>
          <tr><td>row 2</td></tr>
        </tbody>
      </table>
     `, []int{2, 3, 1}},
		{style30 + `
      <table>
        <tbody>
          <tr><td>row 0</td></tr>
          <tr><td>row 1</td></tr>
          <tr style="break-after: avoid"><td>row 2</td></tr>
        </tbody>
        <tbody>
          <tr><td>row 0</td></tr>
          <tr><td>row 1</td></tr>
          <tr><td>row 2</td></tr>
        </tbody>
      </table>
     `, []int{2, 3, 1}},
		{style30 + `
      <table>
        <tbody style="break-after: avoid">
          <tr><td>row 0</td></tr>
          <tr><td>row 1</td></tr>
          <tr style="break-after: page"><td>row 2</td></tr>
        </tbody>
        <tbody>
          <tr><td>row 0</td></tr>
          <tr><td>row 1</td></tr>
          <tr><td>row 2</td></tr>
        </tbody>
      </table>
     `, []int{3, 3}},
	} {
		t.Run(fmt.Sprint(i), func(t *testing.T) {
			pages := tablePagesN(t, c.html, len(c.rows))
			var rowsPerPage []int
			for _, page := range pages {
				_, body := pageBody(t, page)
				if len(body.Children) > 0 && body.Children[0].ElementTag == "p" {
					rowsPerPage = append(rowsPerPage, len(body.Children))
					continue
				}
				rows := 0
				for _, size := range tableGroupSizes(t, only(t, body)) {
					rows += size
				}
				rowsPerPage = append(rowsPerPage, rows)
			}
			tableCheckSizes(t, "rows per page", rowsPerPage, c.rows...)
		})
	}
}

// tableCheckTags fails the test when the tags of boxes are not want.
func tableCheckTags(t *testing.T, boxes []*Box, want ...string) {
	t.Helper()
	for i, tag := range want {
		if boxes[i].ElementTag != tag {
			t.Errorf("box %d is %s, want %s", i, boxes[i].ElementTag, tag)
		}
	}
}

func TestTablePageBreakAvoidBeforeTable(t *testing.T) {
	pages := tablePagesN(t, `
      <style>
        @page { size: 100px }
        h1 { height: 30px }
        td, p { font: 20px / 1 weasyprint }
        tr { break-after: avoid }
        table { table-layout: fixed; width: 100%; break-before: avoid }
      </style>
      <h1>Dummy title</h1>
      <p>paragraph</p>
      <table>
        <tbody>
          <tr><td>row 1</td></tr>
        </tbody>
        <tbody>
          <tr><td>row 2</td></tr>
          <tr><td>row 3</td></tr>
        </tbody>
      </table>
     `, 2)
	_, body := pageBody(t, pages[0])
	tableCheckTags(t, unpack(t, body, 1), "h1")
	_, body = pageBody(t, pages[1])
	tableCheckTags(t, unpack(t, body, 2), "p", "table")
}

func TestTablePageBreakAvoidBeforeTbody(t *testing.T) {
	pages := tablePagesN(t, `
      <style>
        @page { size: 100px }
        h1 { height: 30px }
        td, p { height: 20px }
        tr { break-after: avoid }
        tbody { break-before: avoid }
      </style>
      <h1>Dummy title</h1>
      <p>paragraph</p>
      <table>
        <tbody>
          <tr><td>row 1</td></tr>
        </tbody>
        <tbody>
          <tr><td>row 2</td></tr>
          <tr><td>row 3</td></tr>
        </tbody>
      </table>
     `, 2)
	_, body := pageBody(t, pages[0])
	tableCheckTags(t, unpack(t, body, 1), "h1")
	_, body = pageBody(t, pages[1])
	tableCheckTags(t, unpack(t, body, 2), "p", "table")
}

func TestTablePageBreakAvoidAfter(t *testing.T) {
	pages := tablePagesN(t, `
      <style>
        @page { size: 7px }
        table { font: 2px/1 weasyprint; border-collapse: collapse }
        tr { break-inside: avoid }
      </style>
      <table>
        <tbody>
          <tr><td>1</td></tr>
          <tr><td>2</td></tr>
          <tr style="break-after: avoid"><td>3</td></tr>
          <tr><td>4</td></tr>
          <tr><td>5</td></tr>
        </tbody>
      </table>
     `, 2)
	_, body := pageBody(t, pages[0])
	tableCheckSizes(t, "page 1", tableGroupSizes(t, only(t, body)), 2)
	_, body = pageBody(t, pages[1])
	tableCheckSizes(t, "page 2", tableGroupSizes(t, only(t, body)), 3)
}

func TestInlineTableBaseline(t *testing.T) {
	// Check that inline table's baseline is its first row's baseline.
	//
	// Div text's baseline is at 18px from the top (10px because of the
	// line-height, 8px as it's weasyprint.otf's baseline position).
	//
	// When a row has vertical-align: baseline cells, its baseline is its
	// cell's baseline. The position of the table is thus 10px above the
	// text's baseline.
	//
	// When a row has another value for vertical-align, the baseline is
	// the bottom of the row. The first cell's text is aligned with the
	// div's text, and the top of the table is thus 8px above the baseline.
	for _, c := range []struct {
		verticalAlign  string
		tablePositionY float64
	}{
		{"top", 8},
		{"bottom", 8},
		{"baseline", 10},
	} {
		t.Run(c.verticalAlign, func(t *testing.T) {
			page := tablePage(t, `
      <div style="font-family: weasyprint; font-size: 10px; line-height: 30px">
        abc
        <table style="display: inline-table; border-collapse: collapse;
                      line-height: 10px">
          <tr><td style="vertical-align: `+c.verticalAlign+`">a</td></tr>
          <tr><td>a</td></tr>
        </table>
        abc
      </div>
    `)
			_, body := pageBody(t, page)
			line := only(t, only(t, body))
			boxes := unpack(t, line, 3)
			table := only(t, boxes[1])
			tableEq(t, "text1.position_y", boxes[0].PositionY, 0)
			tableEq(t, "text2.position_y", boxes[2].PositionY, 0)
			tableEq(t, "table.height", table.Height, 10*2)
			if d := table.PositionY - c.tablePositionY; d <= -0.1 || d >= 0.1 {
				t.Errorf("table.position_y = %v, want %v", table.PositionY, c.tablePositionY)
			}
		})
	}
}

// tableCheckContentBox fails the test when the position of the content
// box of box is not (x, y).
func tableCheckContentBox(t *testing.T, name string, box *Box, x, y float64) {
	t.Helper()
	tableEq(t, name+".content_box_x()", box.ContentBoxX(), x)
	tableEq(t, name+".content_box_y()", box.ContentBoxY(), y)
}

func TestTableCaptionMarginTop(t *testing.T) {
	page := tablePage(t, `
      <style>
        table { margin: 20px; }
        caption, h1, h2 { margin: 20px; height: 10px }
        td { height: 10px }
      </style>
      <h1></h1>
      <table>
        <caption></caption>
        <tr>
          <td></td>
        </tr>
      </table>
      <h2></h2>
    `)
	_, body := pageBody(t, page)
	c := unpack(t, body, 3)
	h1, wrapper, h2 := c[0], c[1], c[2]
	c = unpack(t, wrapper, 2)
	caption, table := c[0], c[1]
	tbody := only(t, table)
	tableCheckContentBox(t, "h1", h1, 20, 20)
	tableCheckContentBox(t, "wrapper", wrapper, 20, 50)
	tableCheckContentBox(t, "caption", caption, 40, 70)
	tableCheckContentBox(t, "tbody", tbody, 20, 100)
	tableCheckContentBox(t, "h2", h2, 20, 130)
}

func TestTableCaptionMarginBottom(t *testing.T) {
	page := tablePage(t, `
      <style>
        table { margin: 20px; }
        caption, h1, h2 { margin: 20px; height: 10px; caption-side: bottom }
        td { height: 10px }
      </style>
      <h1></h1>
      <table>
        <caption></caption>
        <tr>
          <td></td>
        </tr>
      </table>
      <h2></h2>
    `)
	_, body := pageBody(t, page)
	c := unpack(t, body, 3)
	h1, wrapper, h2 := c[0], c[1], c[2]
	c = unpack(t, wrapper, 2)
	table, caption := c[0], c[1]
	tbody := only(t, table)
	tableCheckContentBox(t, "h1", h1, 20, 20)
	tableCheckContentBox(t, "wrapper", wrapper, 20, 50)
	tableCheckContentBox(t, "tbody", tbody, 20, 50)
	tableCheckContentBox(t, "caption", caption, 40, 80)
	tableCheckContentBox(t, "h2", h2, 20, 130)
}

func TestTableEmptyBody(t *testing.T) {
	for i, c := range []struct {
		rowsExpected [][]string
		thead, tfoot int
		content      string
	}{
		{[][]string{{}, {"Header", "Footer"}}, 45, 45, "<p>content</p>"},
		{[][]string{{}, {"Header", "Footer"}}, 85, 5, "<p>content</p>"},
		{[][]string{{"Header", "Footer"}}, 30, 30, "<p>content</p>"},
		{[][]string{{}, {"Header"}}, 30, 110, "<p>content</p>"},
		{[][]string{{}, {"Header", "Footer"}}, 30, 60, "<p>content</p>"},
		{[][]string{{}, {"Footer"}}, 110, 30, "<p>content</p>"},

		// We try to render the header and footer on the same page, but it
		// does not fit. So we try to render the header or the footer on
		// the next one, but nothing fit either.
		{[][]string{{}, {}}, 110, 110, "<p>content</p>"},

		{[][]string{{"Header", "Footer"}}, 30, 30, ""},
		{[][]string{{"Header"}}, 30, 110, ""},
		{[][]string{{"Header", "Footer"}}, 30, 60, ""},
		{[][]string{{"Footer"}}, 110, 30, ""},
		{[][]string{{}}, 110, 110, ""},
	} {
		t.Run(fmt.Sprint(i), func(t *testing.T) {
			html := fmt.Sprintf(`
      <style>
        @page { size: 100px }
        p { height: 20px }
        thead th { height: %dpx }
        tfoot th { height: %dpx }
      </style>
      %s
      <table>
        <thead><tr><th>Header</th></tr></thead>
        <tfoot><tr><th>Footer</th></tr></tfoot>
      </table>
    `, c.thead, c.tfoot, c.content)
			pages := tablePagesN(t, html, len(c.rowsExpected))
			for i, page := range pages {
				rows := []string{}
				_, body := pageBody(t, page)
				if len(body.Children) > 0 {
					tableWrapper := body.Children[len(body.Children)-1]
					if tableWrapper.IsTableWrapper {
						for _, group := range tableGroupTexts(t, only(t, tableWrapper)) {
							rows = append(rows, group...)
						}
					}
				}
				if !reflect.DeepEqual(rows, c.rowsExpected[i]) {
					t.Errorf("rows of page %d = %q, want %q", i, rows, c.rowsExpected[i])
				}
			}
		})
	}
}

// tableBodyGroups returns the row groups of the table of the only table
// wrapper of the body of page.
func tableBodyGroups(t *testing.T, page *Box) []*Box {
	t.Helper()
	_, body := pageBody(t, page)
	return only(t, only(t, body)).Children
}

func TestTableGroupBreakInsideAvoidAbsolute(t *testing.T) {
	// Regression test for #2134.
	pages := renderPages(t, `
      <style>
        @page { size: 5cm }
        tbody { break-inside: avoid; line-height: 2cm }
        div { position: absolute }
      </style>
      <table>
        <tbody><tr><td>a</td></tr></tbody>
        <tbody><tr>
          <td><div>a<br>b</div></td>
          <td>a<br>b</td>
        </tr></tbody>
      </table>
    `)
	if len(pages) != 2 {
		t.Fatalf("got %d pages, want 2", len(pages))
	}
	if groups := tableBodyGroups(t, pages[0]); len(groups) != 1 { // Only first group
		t.Errorf("page 1 holds %d groups, want 1", len(groups))
	}
	// No absolute div here
	if groups := tableBodyGroups(t, pages[1]); len(groups) != 1 { // Only second group
		t.Errorf("page 2 holds %d groups, want 1", len(groups))
	}
}

func TestTableRowBreakInsideAvoidAbsolute(t *testing.T) {
	// Regression test for #2134.
	pages := renderPages(t, `
      <style>
        @page { size: 5cm }
        tr { break-inside: avoid; line-height: 2cm }
        div { position: absolute }
      </style>
      <table>
        <tr><td>a</td></tr>
        <tr>
          <td><div>a<br>b</div></td>
          <td>a<br>b</td>
        </tr>
      </table>
    `)
	if len(pages) != 2 {
		t.Fatalf("got %d pages, want 2", len(pages))
	}
	groups := tableBodyGroups(t, pages[0])
	if len(groups) != 1 || len(groups[0].Children) != 1 { // Only first row
		t.Errorf("page 1 does not hold one group of one row")
	}
	// No absolute div here
	groups = tableBodyGroups(t, pages[1])
	if len(groups) != 1 || len(groups[0].Children) != 1 { // Only second row
		t.Errorf("page 2 does not hold one group of one row")
	}
}

func TestTableBreakInsideAvoidAbsolute(t *testing.T) {
	// Regression test for #2134.
	pages := renderPages(t, `
      <style>
        @page { size: 5cm }
        body { line-height: 2cm }
        table { break-inside: avoid }
        div { position: absolute }
      </style>
      <p>text</p>
      <table>
        <tr>
          <td><div>a<br>b</div></td>
          <td>a<br>b</td>
        </tr>
      </table>
    `)
	if len(pages) != 2 {
		t.Fatalf("got %d pages, want 2", len(pages))
	}
	_, body := pageBody(t, pages[0])
	text := only(t, only(t, only(t, body)))
	if text.Text != "text" {
		t.Errorf("text = %q, want %q", text.Text, "text")
	}
	pageBody(t, pages[1]) // No absolute div here
}

func TestTableBreakChildrenMargin(t *testing.T) {
	// Regression test for #1254.
	pages := renderPages(t, `
      <style>
        @page { size: 100px }
        p { height: 20px; margin-top: 50px }
      </style>
      <table>
        <tr><td><p>Page1</p></td></tr>
        <tr><td><p>Page2</p></td></tr>
        <tr><td><p>Page3</p></td></tr>
      </table>
    `)
	if len(pages) != 3 {
		t.Errorf("got %d pages, want 3", len(pages))
	}
}

func TestTableBreakInPaddedBlock(t *testing.T) {
	// Regression test for #2580.
	pages := renderPages(t, `
      <style>
        @page { size: 100px; margin: 0 }
        body { margin: 0; font: 2px/1 weasyprint }
        div.previous { height: 20px }
        div.wrapper { padding-bottom: 6px }
        table {
          border-collapse: collapse;
          border-spacing: 0;
          margin: 20px 0;
          table-layout: fixed;
          width: 100%;
        }
        td { height: 10px; padding: 0 }
        tr { break-inside: avoid }
      </style>
      <div class="previous">Before</div>
      <div class="wrapper">
        <table>
          <thead><tr><td>Header</td></tr></thead>
          <tbody><tr><td>Row</td></tr></tbody>
          <tfoot><tr><td>Footer</td></tr></tfoot>
        </table>
      </div>
    `)
	if len(pages) != 1 {
		t.Fatalf("got %d pages, want 1", len(pages))
	}
	_, body := pageBody(t, pages[0])
	c := unpack(t, body, 2)
	previous, wrapper := c[0], c[1]
	table := only(t, only(t, wrapper))
	groups := unpack(t, table, 3)
	if previous.ElementTag != "div" {
		t.Errorf("previous is %s, want div", previous.ElementTag)
	}
	for i, group := range groups {
		if len(group.Children) != 1 {
			t.Errorf("group %d holds %d rows, want 1", i, len(group.Children))
		}
	}
}

func TestTableTdBreakInsideAvoid(t *testing.T) {
	// Regression test for #1547.
	pages := renderPages(t, `
      <style>
        @page { size: 4cm }
        td { break-inside: avoid; line-height: 3cm }
      </style>
      <table>
        <tr>
          <td>
            a<br>a
          </td>
        </tr>
      </table>
    `)
	if len(pages) != 2 {
		t.Errorf("got %d pages, want 2", len(pages))
	}
}

func TestTableBadIntTdThSpan(t *testing.T) {
	_, table := tableRender(t, `
      <table>
        <tr>
          <td colspan="bad"></td>
          <td rowspan="23.4"></td>
        </tr>
        <tr>
          <th colspan="x" rowspan="-2"></th>
          <th></th>
        </tr>
      </table>
    `)
	rows := unpack(t, only(t, table), 2)
	td := unpack(t, rows[0], 2)
	// colspan="bad" falls back to 1
	if td[0].table().Colspan != 1 {
		t.Errorf("td_1.colspan = %d, want 1", td[0].table().Colspan)
	}
	tableEq(t, "td_1.width", td[0].Width, td[1].Width)
	// rowspan="23.4" is parsed as 23 per HTML spec (stops at "."), then
	// clamped to actual row count by the layout engine
	if td[1].table().Rowspan != 2 {
		t.Errorf("td_2.rowspan = %d, want 2", td[1].table().Rowspan)
	}
	th := unpack(t, rows[1], 2)
	// colspan="x" falls back to 1
	if th[0].table().Colspan != 1 {
		t.Errorf("th_1.colspan = %d, want 1", th[0].table().Colspan)
	}
	tableEq(t, "th_1.width", th[0].Width, th[1].Width)
}

func TestTableBadIntColSpan(t *testing.T) {
	_, table := tableRender(t, `
      <table>
        <colgroup>
          <col span="bad" style="width:25px" />
        </colgroup>
        <tr>
          <td>a</td>
          <td>a</td>
        </tr>
      </table>
    `)
	td := tableRowCells(t, table, 2)
	tableEq(t, "td_1.width", td[0].Width, 25)
}

func TestTableBadIntColgroupSpan(t *testing.T) {
	_, table := tableRender(t, `
      <table>
        <colgroup span="bad" style="width:25px">
          <col />
        </colgroup>
        <tr>
          <td>a</td>
          <td>a</td>
        </tr>
      </table>
    `)
	td := tableRowCells(t, table, 2)
	tableEq(t, "td_1.width", td[0].Width, 25)
}

func TestTableIntWithTrailingTextTdThSpan(t *testing.T) {
	// https://github.com/Kozea/WeasyPrint/issues/2636
	// Browsers ignore text after digits in HTML integer attributes.
	_, table := tableRender(t, `
      <table>
        <tr>
          <td colspan="100%">a</td>
        </tr>
        <tr>
          <td rowspan="2px">a</td>
          <td>a</td>
        </tr>
        <tr>
          <td>a</td>
        </tr>
      </table>
    `)
	rows := unpack(t, only(t, table), 3)
	// colspan="100%" should be parsed as colspan="100"
	if td1 := only(t, rows[0]); td1.table().Colspan != 100 {
		t.Errorf("td_1.colspan = %d, want 100", td1.table().Colspan)
	}
	// rowspan="2px" should be parsed as rowspan="2"
	if td2 := unpack(t, rows[1], 2)[0]; td2.table().Rowspan != 2 {
		t.Errorf("td_2.rowspan = %d, want 2", td2.table().Rowspan)
	}
}

func TestTableIntWithTrailingTextColSpan(t *testing.T) {
	// https://github.com/Kozea/WeasyPrint/issues/2636
	_, table := tableRender(t, `
      <table>
        <colgroup>
          <col span="2px" style="width:25px" />
        </colgroup>
        <tr>
          <td>a</td>
          <td>a</td>
        </tr>
      </table>
    `)
	// span="2px" should be parsed as span="2", applying width to both
	// columns
	tableCheckWidths(t, tableRowCells(t, table, 2), 25, 25)
}

func TestTableColspanRoundingError(t *testing.T) {
	// Regression test for #2842.
	tablePage(t, `
      <table style="width:136.66666666666487px">
        <tr>
          <th style="width:40%"></th>
          <th colspan="2"><div style="width:82px"></span></th>
        </tr>
        <tr>
          <th style="width:40%"></th>
          <th><div style="width:12px"></div></th>
          <th><span style="display:inline-block;width:8px"></span><span
                    style="display:inline-block;width:8px"></span></th>
        </tr>
      </table>
    `)
}

func TestTableDifferentDisplay(t *testing.T) {
	// Test display attribute set on different table elements.
	tablePages(t, `
      <table style="font-size: 1px">
        <colgroup style="display: block"><div>a</div></colgroup>
        <col style="display: block"><div>a</div></col>
        <tr style="display: block"><div>a</div></tr>
        <td style="display: block"><div>a</div></td>
        <th style="display: block"><div>a</div></th>
        <thead>
          <colgroup style="display: block"><div>a</div></colgroup>
          <col style="display: block"><div>a</div></col>
          <tr style="display: block"><div>a</div></tr>
          <td style="display: block"><div>a</div></td>
          <th style="display: block"><div>a</div></th>
        </thead>
        <tbody>
          <colgroup style="display: block"><div>a</div></colgroup>
          <col style="display: block"><div>a</div></col>
          <tr style="display: block"><div>a</div></tr>
          <td style="display: block"><div>a</div></td>
          <th style="display: block"><div>a</div></th>
        </tbody>
        <tr>
          <colgroup style="display: block"><div>a</div></colgroup>
          <col style="display: block"><div>a</div></col>
          <tr style="display: block"><div>a</div></tr>
          <td style="display: block"><div>a</div></td>
          <th style="display: block"><div>a</div></th>
        </tr>
        <td>
          <colgroup style="display: block"><div>a</div></colgroup>
          <col style="display: block"><div>a</div></col>
          <tr style="display: block"><div>a</div></tr>
          <td style="display: block"><div>a</div></td>
          <th style="display: block"><div>a</div></th>
        </td>
      </table>
    `)
}

func TestMinWidthWithOverflow(t *testing.T) {
	// Regression test for #1383.
	page := tablePage(t, `
      <style>
        table td { border: 1px solid black }
        table.key-val tr td:nth-child(1) { min-width: 13em }
      </style>

      <table class="key-val">
        <tbody>
          <tr>
            <td>Normal Key 1</td>
            <td>Normal Value 1</td>
          </tr>
          <tr>
            <td>Normal Key 2</td>
            <td>Normal Value 2</td>
          </tr>
        </tbody>
      </table>

      <table class="key-val">
        <tbody>
          <tr>
            <td>Short value</td>
            <td>Works as expected</td>
          </tr>
          <tr>
            <td>Long Value</td>
            <td>Annoyingly breaks my table layout: Sed ut perspiciatis
                unde omnis iste natus error sit voluptatem
                accusantium doloremque laudantium, totam rem aperiam,
                eaque ipsa quae ab illo inventore veritatis et quasi
                architecto beatae vitae dicta sunt explicabo.
            </td>
          </tr>
        </tbody>
      </table>
    `)
	_, body := pageBody(t, page)
	wrappers := unpack(t, body, 2)
	rows1 := unpack(t, only(t, only(t, wrappers[0])), 2)
	table1Td := unpack(t, rows1[0], 2)
	rows2 := unpack(t, only(t, only(t, wrappers[1])), 2)
	table2Td := unpack(t, rows2[0], 2)
	tableEq(t, "td1.min_width", table1Td[0].MinWidth, table2Td[0].MinWidth)
	tableEq(t, "td1.width", table1Td[0].Width, table2Td[0].Width)
}

func TestTableCellMaxWidth(t *testing.T) {
	_, table := tableRender(t, `
      <style>
        td {
          text-overflow: ellipsis;
          white-space: nowrap;
          overflow: hidden;
          max-width: 45px;
        }
      </style>
      <table>
        <tr>
          <td>abcdefghijkl</td>
        </tr>
      </table>
    `)
	td := only(t, only(t, only(t, table)))
	tableEq(t, "td.max_width", td.MaxWidth, 45)
	tableEq(t, "td.width", td.Width, 45)
}

// tableGrid is _get_grid of tests/layout/test_table.py: it parses a
// document whose body holds a table, resolves its collapsed borders and
// returns them, as "style width (r, g, b, a)" or "None" for a border of
// no width.
func tableGrid(t *testing.T, html string, gridWidth, gridHeight int) (vertical, horizontal [][]string) {
	t.Helper()
	box := parseAll(t, html)
	body := only(t, box)
	table := only(t, only(t, body))
	grid := collapseTableBorders(table, gridWidth, gridHeight)
	convert := func(borders [][]CollapsedBorder) [][]string {
		result := make([][]string, 0, len(borders))
		for _, list := range borders {
			row := []string{}
			for _, border := range list {
				if border.Width == 0 {
					row = append(row, "None")
					continue
				}
				r, g, b, a := srgba(border.Color)
				row = append(row, fmt.Sprintf("%s %v (%v, %v, %v, %v)", border.Style, border.Width, r, g, b, a))
			}
			result = append(result, row)
		}
		return result
	}
	return convert(grid.Vertical), convert(grid.Horizontal)
}

// The borders of the tests of the collapsed borders.
const (
	tableBlack3      = "solid 3 (0, 0, 0, 1)"
	tableRed1        = "solid 1 (1, 0, 0, 1)"
	tableYellow5     = "solid 5 (1, 1, 0, 1)"
	tableGreen5      = "solid 5 (0, 1, 0, 1)" // lime in CSS
	tableDashedBlue5 = "dashed 5 (0, 0, 1, 1)"
	tableNone        = "None"
)

// tableCheckBorders fails the test when the borders are not want.
func tableCheckBorders(t *testing.T, name string, got, want [][]string) {
	t.Helper()
	if !reflect.DeepEqual(got, want) {
		t.Errorf("%s = %q, want %q", name, got, want)
	}
}

func TestBorderCollapse1(t *testing.T) {
	html := parseAll(t, "<table></table>")
	body := only(t, html)
	table := only(t, only(t, body))
	if !table.IsTable() {
		t.Errorf("the table is a %s box", table.Kind)
	}
	if table.table().CollapsedBorderGrid != nil {
		t.Error("a table with separated borders has a grid of collapsed borders")
	}

	vertical, horizontal := tableGrid(t, `<table style="border-collapse: collapse"></table>`, 0, 0)
	tableCheckBorders(t, "vertical_borders", vertical, [][]string{})
	tableCheckBorders(t, "horizontal_borders", horizontal, [][]string{})
}

func TestBorderCollapse2(t *testing.T) {
	vertical, horizontal := tableGrid(t, `
      <style>td { border: 1px solid red }</style>
      <table style="border-collapse: collapse; border: 3px solid black">
        <tr> <td>A</td> <td>B</td> </tr>
        <tr> <td>C</td> <td>D</td> </tr>
      </table>
    `, 2, 2)
	tableCheckBorders(t, "vertical_borders", vertical, [][]string{
		{tableBlack3, tableRed1, tableBlack3},
		{tableBlack3, tableRed1, tableBlack3},
	})
	tableCheckBorders(t, "horizontal_borders", horizontal, [][]string{
		{tableBlack3, tableBlack3},
		{tableRed1, tableRed1},
		{tableBlack3, tableBlack3},
	})
}

func TestBorderCollapse3(t *testing.T) {
	// hidden vs. none
	vertical, horizontal := tableGrid(t, `
      <style>table, td { border: 3px solid }</style>
      <table style="border-collapse: collapse">
        <tr> <td>A</td> <td style="border-style: hidden">B</td> </tr>
        <tr> <td>C</td> <td style="border-style: none">D</td> </tr>
      </table>
    `, 2, 2)
	tableCheckBorders(t, "vertical_borders", vertical, [][]string{
		{tableBlack3, tableNone, tableNone},
		{tableBlack3, tableBlack3, tableBlack3},
	})
	tableCheckBorders(t, "horizontal_borders", horizontal, [][]string{
		{tableBlack3, tableNone},
		{tableBlack3, tableNone},
		{tableBlack3, tableBlack3},
	})
}

func TestBorderCollapse4(t *testing.T) {
	vertical, horizontal := tableGrid(t, `
      <style>td { border: 1px solid red }</style>
      <table style="border-collapse: collapse; border: 5px solid yellow">
        <col style="border: 3px solid black" />
        <tr> <td></td> <td></td> <td></td> </tr>
        <tr> <td></td> <td style="border: 5px dashed blue"></td>
          <td style="border: 5px solid lime"></td> </tr>
        <tr> <td></td> <td></td> <td></td> </tr>
        <tr> <td></td> <td></td> <td></td> </tr>
      </table>
    `, 3, 4)
	tableCheckBorders(t, "vertical_borders", vertical, [][]string{
		{tableYellow5, tableBlack3, tableRed1, tableYellow5},
		{tableYellow5, tableDashedBlue5, tableGreen5, tableGreen5},
		{tableYellow5, tableBlack3, tableRed1, tableYellow5},
		{tableYellow5, tableBlack3, tableRed1, tableYellow5},
	})
	tableCheckBorders(t, "horizontal_borders", horizontal, [][]string{
		{tableYellow5, tableYellow5, tableYellow5},
		{tableRed1, tableDashedBlue5, tableGreen5},
		{tableRed1, tableDashedBlue5, tableGreen5},
		{tableRed1, tableRed1, tableRed1},
		{tableYellow5, tableYellow5, tableYellow5},
	})
}

func TestBorderCollapse5(t *testing.T) {
	// rowspan and colspan
	vertical, horizontal := tableGrid(t, `
        <style>col, tr { border: 3px solid }</style>
        <table style="border-collapse: collapse">
            <col /><col /><col />
            <tr> <td rowspan=2></td> <td></td> <td></td> </tr>
            <tr>                     <td colspan=2></td> </tr>
        </table>
    `, 3, 2)
	tableCheckBorders(t, "vertical_borders", vertical, [][]string{
		{tableBlack3, tableBlack3, tableBlack3, tableBlack3},
		{tableBlack3, tableBlack3, tableNone, tableBlack3},
	})
	tableCheckBorders(t, "horizontal_borders", horizontal, [][]string{
		{tableBlack3, tableBlack3, tableBlack3},
		{tableNone, tableBlack3, tableBlack3},
		{tableBlack3, tableBlack3, tableBlack3},
	})
}

func TestTableZeroWidth(t *testing.T) {
	// Regression test for #2306.
	tablePage(t, `
      <table style="font-family: weasyprint">
        <thead>
          <tr>
            <th colspan="2">aaaaaaaaaaaaaaaaaa</th>
          </tr>
          <tr>
            <th style="width:80px"></th>
            <th style="width:80px">rrrrr</th>
          </tr>
        </thead>
      </table>
    `)
}

// TestTableSplitCellBorders tests the collapsed borders of a row that a
// page break splits. It is not a test of upstream. With
// box-decoration-break: clone on the table and the cells, each part of the
// row has its borders, as LibreOffice draws them. Without clone, the parts
// have no border at the split, as upstream.
func TestTableSplitCellBorders(t *testing.T) {
	for _, tt := range []struct {
		name  string
		clone bool
		width float64
		lines [2]int
	}{
		// The text of the first page starts at y = 2, below the halves of
		// the top border of the table and of the cell. 4 lines of 10px end
		// at y = 42. With clone, the cell keeps its half of the bottom
		// border, 1px, so the fourth line does not fit in the 42.5px of the
		// page.
		{"clone", true, 1, [2]int{3, 3}},
		{"slice", false, 0, [2]int{4, 2}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			clone := ""
			if tt.clone {
				clone = "table, td { box-decoration-break: clone }"
			}
			pages := tablePagesN(t, `
      <style>
        @page { size: 100px 42.5px }
        body { margin: 0 }
        table { font: 10px/1 weasyprint; border-collapse: collapse }
        td { border: 2px solid; padding: 0; orphans: 1; widows: 1 }
        `+clone+`
      </style>
      <table><tr><td>a<br>b<br>c<br>d<br>e<br>f</td></tr></table>`, 2)
			for i, page := range pages {
				var table, cell *Box
				page.Descendants(func(d *Box) bool {
					switch d.Kind {
					case KindTable:
						table = d
					case KindTableCell:
						cell = d
					}
					return true
				})
				if table == nil || cell == nil {
					t.Fatalf("page %d has no table or no cell", i+1)
				}
				if n := len(cell.Children); n != tt.lines[i] {
					t.Errorf("page %d: got %d lines, want %d", i+1, n, tt.lines[i])
				}
				if i == 0 {
					tableEq(t, "page 1: table.border_bottom_width", table.BorderBottomWidth, tt.width)
					tableEq(t, "page 1: cell.border_bottom_width", cell.BorderBottomWidth, tt.width)
					if table.table().SkipCellBorderBottom == tt.clone {
						t.Errorf("page 1: got skip_cell_border_bottom %v, want %v", table.table().SkipCellBorderBottom, !tt.clone)
					}
					continue
				}
				tableEq(t, "page 2: table.border_top_width", table.BorderTopWidth, tt.width)
				tableEq(t, "page 2: cell.border_top_width", cell.BorderTopWidth, tt.width)
				if table.table().SkipCellBorderTop == tt.clone {
					t.Errorf("page 2: got skip_cell_border_top %v, want %v", table.table().SkipCellBorderTop, !tt.clone)
				}
			}
		})
	}
}

func TestTableInlineAtomicNowrap(t *testing.T) {
	// Regression test for #2661.
	page := tablePage(t, `
      <div style="width: 0; font: 2px weasyprint; white-space: nowrap">
        <table>
          <tr>
            <td>a<img src=pattern.png></td>
    `)
	_, body := pageBody(t, page)
	div := only(t, body)
	tableWrapper := only(t, div)
	tableEq(t, "div.width", div.Width, 0)
	tableEq(t, "table_wrapper.width", tableWrapper.Width, 6)
}
