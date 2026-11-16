package sheet

import (
	"context"
	"image"
	"image/color"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/xo/ivy"
	"github.com/xo/ivy/internal/layout"
)

// testWorkbook returns a workbook of one sheet with a header row, numbers,
// a merged cell and a column of its own width.
func testWorkbook() *Workbook {
	wb := &Workbook{
		Styles: []Style{
			{Font: Font{Family: "Calibri", Size: 11}},
			{Font: Font{Family: "Calibri", Size: 11, Bold: true}, Fill: color.NRGBA{0xdd, 0xeb, 0xf7, 255},
				Borders: [4]Border{{}, {}, {Width: 0.75, Line: "solid"}, {}}, HAlign: "center"},
		},
	}
	sh := &Sheet{
		Name:             "Data",
		DefaultColWidth:  48,
		DefaultRowHeight: 15,
		Cols:             []Col{{Min: 0, Max: 0, Width: 120}},
		Rows: []Row{
			{Index: 0, Cells: []Cell{{Col: 0, Text: "Name", Style: 1}, {Col: 1, Text: "Value", Style: 1}, {Col: 2, Text: "Total", Style: 1}}},
			{Index: 1, Cells: []Cell{{Col: 0, Text: "alpha"}, {Col: 1, Text: "1.50", Kind: KindNumber}}},
			{Index: 2, Cells: []Cell{{Col: 0, Text: "beta"}, {Col: 1, Text: "-2", Kind: KindNumber, Color: color.NRGBA{255, 0, 0, 255}}}},
			{Index: 3, Cells: []Cell{{Col: 0, Text: "merged text"}}},
		},
		Merges: []Range{{Row0: 3, Col0: 0, Row1: 4, Col1: 2}},
		Page:   DefaultPage,
	}
	wb.Sheets = []*Sheet{sh}
	return wb
}

// document returns the text of document i of src.
func document(t *testing.T, src layout.Source, i int) string {
	t.Helper()
	buf, err := fs.ReadFile(src.FS, src.Documents[i])
	if err != nil {
		t.Fatal(err)
	}
	return string(buf)
}

func TestSourceHTML(t *testing.T) {
	src, err := testWorkbook().Source(t.Context(), Options{})
	if err != nil {
		t.Fatal(err)
	}
	if len(src.Documents) != 1 {
		t.Fatalf("expected 1 document, got: %q", src.Documents)
	}
	doc := document(t, src, 0)
	for _, want := range []string{
		`@page{size:612pt 792pt;margin:54pt 50.4pt 54pt 50.4pt;`,
		`@top-left{content:"Data"`,
		`<col style="width:120pt"><col style="width:48pt"><col style="width:48pt">`,
		`<td class="s1 c">Name</td>`,
		`<td class="n c">1.50</td>`,
		`<td class="n c" style="color:#ff0000">-2</td>`,
		`<td class="c" colspan="3" rowspan="2" style="height:30pt">merged text</td>`,
		`td.s1{font-family:"Calibri", sans-serif;font-size:11pt;font-weight:bold;background-color:#ddebf7;border-bottom:0.75pt solid #000000;text-align:center;}`,
		`<td class="c">alpha</td><td class="n c">1.50</td><td></td>`,
	} {
		if !strings.Contains(doc, want) {
			t.Errorf("expected %q in the document:\n%s", want, doc)
		}
	}
	// the empty row under the merge has no cell of its own
	if !strings.Contains(doc, `rowspan="2" style="height:30pt">merged text</td></tr><tr></tr>`) {
		t.Errorf("expected the covered row without cells:\n%s", doc)
	}
}

func TestSourceEmpty(t *testing.T) {
	wb := &Workbook{Sheets: []*Sheet{{Name: "Empty", Page: DefaultPage}}}
	src, err := wb.Source(t.Context(), Options{})
	if err != nil {
		t.Fatal(err)
	}
	if len(src.Documents) != 1 || src.Documents[0] != "empty.html" {
		t.Fatalf("expected one empty page, got: %q", src.Documents)
	}
	book := layout.NewBook(src, layout.BookOptions{})
	if n, err := book.PageCount(t.Context()); err != nil || n != 1 {
		t.Errorf("expected 1 page, got: %d, %v", n, err)
	}
}

func TestBands(t *testing.T) {
	wb := &Workbook{}
	sh := &Sheet{Name: "Wide", DefaultColWidth: 100, DefaultRowHeight: 15, Page: DefaultPage}
	cells := make([]Cell, 0, 12)
	for c := range 12 {
		cells = append(cells, Cell{Col: c, Text: "x"})
	}
	sh.Rows = []Row{{Index: 0, Cells: cells}}
	sh.TitleCols = &Span{First: 0, Last: 0}
	wb.Sheets = []*Sheet{sh}
	src, err := wb.Source(t.Context(), Options{})
	if err != nil {
		t.Fatal(err)
	}
	doc := document(t, src, 0)
	// The page is 511.2 points wide: 5 columns of 100 points in a band,
	// then the title column and 4 more.
	if n := strings.Count(doc, `<table class="sh"`); n != 3 {
		t.Errorf("expected 3 bands, got: %d", n)
	}
	pages := pageCount(t, src)
	if pages != 3 {
		t.Errorf("expected 3 pages, got: %d", pages)
	}

	sh.Page.Fit, sh.Page.FitWidth = true, 1
	src, err = wb.Source(t.Context(), Options{})
	if err != nil {
		t.Fatal(err)
	}
	doc = document(t, src, 0)
	if n := strings.Count(doc, `<table class="sh"`); n != 1 {
		t.Errorf("expected 1 band when the sheet fits one page across, got: %d", n)
	}
	if !strings.Contains(doc, `<col style="width:42.6pt">`) {
		t.Errorf("expected the columns at the scale 0.426:\n%s", doc)
	}
}

func TestLimit(t *testing.T) {
	wb := &Workbook{}
	sh := &Sheet{Name: "Long", Page: DefaultPage}
	for r := range 1000 {
		sh.Rows = append(sh.Rows, Row{Index: r, Cells: []Cell{{Col: 0, Text: "a"}, {Col: 1, Text: "b"}}})
	}
	wb.Sheets = []*Sheet{sh}
	src, err := wb.Source(t.Context(), Options{Limits: ivy.Limits{MaxObjects: 1000}})
	if err != nil {
		t.Fatal(err)
	}
	doc := document(t, src, 0)
	if !strings.Contains(doc, "past the limit of 250 cells") {
		t.Error("expected the note of the limit")
	}
	if n := strings.Count(doc, "<tr"); n > 60 {
		t.Errorf("expected at most 60 rows, got: %d", n)
	}
}

func TestCanceled(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if _, err := testWorkbook().Source(ctx, Options{}); err == nil {
		t.Error("expected the error of the context")
	}
}

// pageCount lays out src and returns the number of its pages.
func pageCount(t *testing.T, src layout.Source) int {
	t.Helper()
	book := layout.NewBook(src, layout.BookOptions{FontFiles: officeFonts(t)})
	n, err := book.PageCount(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	return n
}

// officeFonts returns the folder of the module github.com/xo/ivy/fonts,
// or nil when it is missing.
func officeFonts(t *testing.T) fs.FS {
	t.Helper()
	dir := filepath.Join("..", "..", "fonts")
	if _, err := os.Stat(dir); err != nil {
		return nil
	}
	return os.DirFS(dir)
}

func TestRender(t *testing.T) {
	src, err := testWorkbook().Source(t.Context(), Options{})
	if err != nil {
		t.Fatal(err)
	}
	book := layout.NewBook(src, layout.BookOptions{FontFiles: officeFonts(t)})
	page, err := book.Page(t.Context(), 0)
	if err != nil {
		t.Fatal(err)
	}
	if w, h := page.Size(); w != 612 || h != 792 {
		t.Errorf("expected a page of 612x792 points, got: %vx%v", w, h)
	}
	img, err := ivy.RenderImage(t.Context(), page, 72, color.White)
	if err != nil {
		t.Fatal(err)
	}
	if out := os.Getenv("IVY_TEST_OUT"); out != "" {
		writePNG(t, filepath.Join(out, "sheet.png"), img)
	}
	// The fill of the header row is at the top left of the page, inside
	// the margins: 50.4 points from the left, 54 from the top.
	fill := color.NRGBA{0xdd, 0xeb, 0xf7, 255}
	if got := nrgba(img.At(60, 60)); !near(got, fill) {
		t.Errorf("expected the fill %v at (60, 60), got: %v", fill, got)
	}
	// the text of the first row is drawn
	if !hasInk(img, image.Rect(50, 54, 170, 69)) {
		t.Error("expected the text of the header row")
	}
	// nothing past the three columns
	if hasInk(img, image.Rect(270, 54, 560, 300)) {
		t.Error("expected no ink right of the columns")
	}
}

func TestNumberTooWide(t *testing.T) {
	wb := &Workbook{Sheets: []*Sheet{{
		Name: "Narrow", DefaultColWidth: 48, DefaultRowHeight: 15, Page: DefaultPage,
		Rows: []Row{{Index: 0, Cells: []Cell{
			{Col: 0, Text: "12345678901234", Kind: KindNumber},
			{Col: 1, Text: "1,234.50", Kind: KindNumber},
			{Col: 2, Text: "a text that is too wide"},
		}}},
	}}}
	src, err := wb.Source(t.Context(), Options{})
	if err != nil {
		t.Fatal(err)
	}
	doc := document(t, src, 0)
	// a number that is too wide shows signs #, as in Excel, and text
	// does not
	if !strings.Contains(doc, `<td class="n c">#######</td><td class="n c">1,234.50</td><td>a text that is too wide</td>`) {
		t.Errorf("expected the signs # of the wide number:\n%s", doc)
	}
}

func TestParseHeaderFooter(t *testing.T) {
	hf := ParseHeaderFooter(`&CPage &P of &N&&&R&D &T&KFF0000x`)
	if len(hf.Center) != 5 || hf.Center[2].Text != " of " || hf.Center[3].Kind != FieldPages || hf.Center[4].Text != "&" {
		t.Errorf("unexpected center: %+v", hf.Center)
	}
	if len(hf.Right) != 1 || hf.Right[0].Text != " x" {
		t.Errorf("unexpected right: %+v", hf.Right)
	}
	if hf.Font != nil {
		t.Errorf("expected no font, got: %+v", hf.Font)
	}
	hf = ParseHeaderFooter(`&L&"Arial,Bold"&12&A`)
	if len(hf.Left) != 1 || hf.Left[0].Kind != FieldSheet {
		t.Errorf("unexpected left: %+v", hf.Left)
	}
	if f := hf.Font; f == nil || f.Family != "Arial" || !f.Bold || f.Size != 12 {
		t.Errorf("expected Arial, bold, 12 points, got: %+v", f)
	}
}

func TestColumnWidths(t *testing.T) {
	// Calibri of 11 points has a digit of 7 pixels, and the default
	// column of 8.43 characters is 64 pixels, 48 points
	mdw := MaxDigitWidth(Font{Family: "Calibri", Size: 11}, 10)
	if mdw != 7 {
		t.Errorf("expected a digit of 7 pixels, got: %v", mdw)
	}
	if got := WidthToPoints(9.140625, mdw); got != 48 {
		t.Errorf("expected the default column of 64 pixels, got: %v", got)
	}
	// a font without a size takes the default size
	if got := MaxDigitWidth(Font{Family: "Arial"}, 10); got != 7 {
		t.Errorf("expected a digit of 7 pixels for Arial of 10 points, got: %v", got)
	}
	if w, h, ok := PaperSize(9); !ok || w != 595.28 || h != 841.89 {
		t.Errorf("expected A4, got: %v by %v, %v", w, h, ok)
	}
	if _, _, ok := PaperSize(1000); ok {
		t.Error("expected no paper 1000")
	}
}
