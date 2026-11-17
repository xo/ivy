package sheet

import (
	"image"
	"image/color"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/xo/ivy"
	"github.com/xo/ivy/internal/layout"
)

func TestBlankBand(t *testing.T) {
	// two bands of one column each: the second band shows something in
	// its first row only, so its table ends there and prints one page, as
	// Excel prints no blank page. The first band prints 3 pages.
	sh := &Sheet{Name: "Bands", DefaultColWidth: 48, DefaultRowHeight: 15, Page: DefaultPage,
		Cols: []Col{{Min: 0, Max: 1, Width: 400}}}
	sh.Rows = append(sh.Rows, Row{Index: 0, Cells: []Cell{{Col: 0, Text: "a"}, {Col: 1, Text: "b"}}})
	for i := 1; i < 100; i++ {
		sh.Rows = append(sh.Rows, Row{Index: i, Cells: []Cell{{Col: 0, Text: "a"}}})
	}
	src, err := (&Workbook{Sheets: []*Sheet{sh}}).Source(t.Context(), Options{})
	if err != nil {
		t.Fatal(err)
	}
	doc := document(t, src, 0)
	tables := strings.Split(doc, `<table class="sh"`)
	if len(tables) != 3 {
		t.Fatalf("expected 2 tables, got: %d", len(tables)-1)
	}
	if n := strings.Count(tables[1], "<tr"); n != 100 {
		t.Errorf("expected 100 rows in the first band, got: %d", n)
	}
	if n := strings.Count(tables[2], "<tr"); n != 1 {
		t.Errorf("expected 1 row in the second band, got: %d", n)
	}
	if n := pageCount(t, src); n != 4 {
		t.Errorf("expected 4 pages, got: %d", n)
	}
}

func TestSpill(t *testing.T) {
	// columns of 100 points from 50.4 points: the text of the column B
	// that is aligned right goes past the cell into the empty column A,
	// the centered text into the columns A and C, and the text aligned
	// right next to a cell with text stays in its cell
	long := "Spill spill spill spill spill spill spill spill"
	wb := &Workbook{Styles: []Style{DefaultStyle, {Font: DefaultStyle.Font, HAlign: "right"}, {Font: DefaultStyle.Font, HAlign: "center"}}}
	sh := &Sheet{Name: "Spill", DefaultColWidth: 100, DefaultRowHeight: 20, Page: DefaultPage,
		Rows: []Row{
			{Index: 0, Cells: []Cell{{Col: 1, Text: long, Style: 1}}},
			{Index: 1, Cells: []Cell{{Col: 1, Text: long, Style: 2}}},
			{Index: 2, Cells: []Cell{{Col: 0, Text: "x"}, {Col: 1, Text: long, Style: 1}}},
			{Index: 3, Cells: []Cell{{Col: 3, Text: "end"}}},
		}}
	wb.Sheets = []*Sheet{sh}
	src, err := wb.Source(t.Context(), Options{})
	if err != nil {
		t.Fatal(err)
	}
	doc := document(t, src, 0)
	for _, want := range []string{
		`<td class="s1"><span class="sl"><span>` + long + `</span></span></td>`,
		`<td class="s2"><span class="sb"><span>` + long + `</span></span></td>`,
		`<td class="s1 c"><span class="sl"><span>` + long + `</span></span></td>`,
	} {
		if !strings.Contains(doc, want) {
			t.Errorf("expected %q in the document:\n%s", want, doc)
		}
	}
	book := layout.NewBook(src, layout.BookOptions{FontFiles: officeFonts(t)})
	page, err := book.Page(t.Context(), 0)
	if err != nil {
		t.Fatal(err)
	}
	img, err := ivy.RenderImage(t.Context(), page, 72, color.White)
	if err != nil {
		t.Fatal(err)
	}
	if out := os.Getenv("IVY_TEST_OUT"); out != "" {
		writePNG(t, filepath.Join(out, "spill.png"), img)
		if err := os.WriteFile(filepath.Join(out, "spill.html"), []byte(doc), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	// the rows are 20 points high from 54 points
	row := func(i int) (int, int) { return 54 + 20*i, 54 + 20*(i+1) }
	y0, y1 := row(0)
	if !hasInk(img, image.Rect(51, y0, 148, y1)) || hasInk(img, image.Rect(252, y0, 400, y1)) {
		t.Error("expected the text aligned right in the columns A and B only")
	}
	y0, y1 = row(1)
	if !hasInk(img, image.Rect(51, y0, 148, y1)) || !hasInk(img, image.Rect(252, y0, 349, y1)) {
		t.Error("expected the centered text in the columns A, B and C")
	}
	y0, y1 = row(2)
	if hasInk(img, image.Rect(70, y0, 148, y1)) || !hasInk(img, image.Rect(240, y0, 250, y1)) {
		t.Error("expected the text aligned right to stay in the column B, at its right side")
	}
}

func TestOverThenDown(t *testing.T) {
	// two bands of one column, 100 rows of 15 points on pages of 684
	// points: 45 rows on a page. Over, then down: the first 45 rows of
	// the column A, then of the column B, then the next rows.
	sh := &Sheet{Name: "Order", DefaultColWidth: 48, DefaultRowHeight: 15, Page: DefaultPage,
		Cols: []Col{{Min: 0, Max: 1, Width: 400}}}
	sh.Page.OverThenDown = true
	for i := range 100 {
		sh.Rows = append(sh.Rows, Row{Index: i, Cells: []Cell{{Col: 0, Text: "a" + strconv.Itoa(i)}, {Col: 1, Text: "b" + strconv.Itoa(i)}}})
	}
	src, err := (&Workbook{Sheets: []*Sheet{sh}}).Source(t.Context(), Options{})
	if err != nil {
		t.Fatal(err)
	}
	tables := strings.Split(document(t, src, 0), `<table class="sh"`)[1:]
	if len(tables) != 6 {
		t.Fatalf("expected 6 tables, got: %d", len(tables))
	}
	for i, want := range []string{">a0<", ">b0<", ">a45<", ">b45<", ">a90<", ">b90<"} {
		if !strings.Contains(tables[i], want) {
			t.Errorf("expected %q in the table %d", want, i)
		}
	}
	if n := strings.Count(tables[0], "<tr"); n != 45 {
		t.Errorf("expected 45 rows on the first page, got: %d", n)
	}
	if n := pageCount(t, src); n != 6 {
		t.Errorf("expected 6 pages, got: %d", n)
	}
}

func TestCellName(t *testing.T) {
	for _, test := range []struct {
		row, col int
		exp      string
	}{{0, 0, "A1"}, {2, 27, "AB3"}, {9, 25, "Z10"}, {0, 16383, "XFD1"}} {
		if got := CellName(test.row, test.col); got != test.exp {
			t.Errorf("expected %s, got: %s", test.exp, got)
		}
	}
}
