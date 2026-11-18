package sheet

import (
	"context"
	"image"
	"image/color"
	"io/fs"
	"math"
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
		// the rows of the merged cell keep their own heights
		`<tr><td class="c" colspan="3" rowspan="2">merged text</td></tr><tr></tr>`,
		`td.s1{font-family:"Calibri", sans-serif;font-size:11pt;font-weight:bold;background-color:#ddebf7;border-bottom:0.75pt solid #000000;text-align:center;}`,
		// the empty cell at the end of a row is left out
		`<td class="c">alpha</td><td class="n c">1.50</td></tr>`,
	} {
		if !strings.Contains(doc, want) {
			t.Errorf("expected %q in the document:\n%s", want, doc)
		}
	}
}

func TestMergedRows(t *testing.T) {
	// 60 cells of 3 rows of 15 points each, merged across the band: the
	// rows in which no cell ends keep their heights in the layout. 2700
	// points of rows give 4 pages of 684 points, as without the merges.
	sheet := func(merged bool) *Workbook {
		sh := &Sheet{Name: "Merged", DefaultColWidth: 48, DefaultRowHeight: 15, Page: DefaultPage}
		for i := range 60 {
			sh.Rows = append(sh.Rows, Row{Index: 3 * i, Cells: []Cell{{Col: 0, Text: "a"}, {Col: 1, Text: "b"}}})
			if merged {
				sh.Merges = append(sh.Merges, Range{Row0: 3 * i, Col0: 0, Row1: 3*i + 2, Col1: 1})
			} else {
				sh.Rows = append(sh.Rows, Row{Index: 3*i + 2, Cells: []Cell{{Col: 0, Text: "c"}}})
			}
		}
		return &Workbook{Sheets: []*Sheet{sh}}
	}
	src, err := sheet(true).Source(t.Context(), Options{})
	if err != nil {
		t.Fatal(err)
	}
	doc := document(t, src, 0)
	if want := `<tr><td class="c" colspan="2" rowspan="3">a</td></tr><tr></tr><tr></tr><tr>`; !strings.Contains(doc, want) {
		t.Errorf("expected %q in the document:\n%s", want, doc)
	}
	plain, err := sheet(false).Source(t.Context(), Options{})
	if err != nil {
		t.Fatal(err)
	}
	if got, exp := pageCount(t, src), pageCount(t, plain); got != exp || got != 4 {
		t.Errorf("expected 4 pages with and without the merges, got: %d and %d", got, exp)
	}
}

func TestDefaultStyle(t *testing.T) {
	// the default style centers its cells and fills them: a cell of the
	// default style takes it, and a cell of another style undoes it
	wb := &Workbook{
		Styles: []Style{
			{Font: Font{Family: "Calibri", Size: 11}, Fill: color.NRGBA{0xff, 0xff, 0, 255}, HAlign: "center", VAlign: "center"},
			{Font: Font{Family: "Calibri", Size: 11}},
		},
	}
	sh := &Sheet{Name: "Default", DefaultColWidth: 48, DefaultRowHeight: 15, Page: DefaultPage}
	sh.Rows = []Row{{Index: 0, Cells: []Cell{{Col: 0, Text: "a"}, {Col: 1, Text: "b", Style: 1}, {Col: 2, Text: "1", Style: 1, Kind: KindNumber}}}}
	wb.Sheets = []*Sheet{sh}
	src, err := wb.Source(t.Context(), Options{})
	if err != nil {
		t.Fatal(err)
	}
	doc := document(t, src, 0)
	for _, want := range []string{
		"line-height:normal;background-color:#ffff00;text-align:center;vertical-align:middle;}",
		"td.s1{font-family:\"Calibri\", sans-serif;font-size:11pt;background-color:transparent;text-align:left;vertical-align:bottom;}",
		// the alignment General of a number comes after the styles
		"td.s1{", "}\ntd.n{text-align:right}",
		`<td class="c">a</td><td class="s1 c">b</td><td class="s1 n c">1</td>`,
	} {
		if !strings.Contains(doc, want) {
			t.Errorf("expected %q in the document:\n%s", want, doc)
		}
	}
}

func TestFontFamily(t *testing.T) {
	def := Font{Family: "Times New Roman", Generic: "serif"}
	for _, test := range []struct {
		font Font
		exp  string
	}{
		// a family of its own is sans-serif without a generic family
		{Font{Family: "Arial"}, `"Arial", sans-serif`},
		{Font{Family: "Courier New", Generic: "monospace"}, `"Courier New", monospace`},
		// no family takes the default font
		{Font{}, `"Times New Roman", serif`},
	} {
		if got := fontFamily(test.font, def); got != test.exp {
			t.Errorf("expected %s for %+v, got: %s", test.exp, test.font, got)
		}
	}
}

// TestMergedWrap checks that a merged cell whose text wraps into more lines
// than its rows hold keeps the heights of its rows, as Excel does: 45 rows
// of 15 points fill one page of US Letter.
func TestMergedWrap(t *testing.T) {
	wb := &Workbook{Styles: []Style{
		{Font: Font{Family: "Arial", Size: 10}},
		{Font: Font{Family: "Arial", Size: 10}, Wrap: true},
	}}
	sh := &Sheet{
		Name: "Merged", DefaultColWidth: 48, DefaultRowHeight: 15, Page: DefaultPage, FixedRows: true,
		Merges: []Range{{Row0: 0, Col0: 0, Row1: 1, Col1: 0}},
		Rows:   []Row{{Index: 0, Cells: []Cell{{Col: 0, Text: strings.Repeat("word ", 40), Style: 1}}}},
	}
	for i := 2; i < 45; i++ {
		sh.Rows = append(sh.Rows, Row{Index: i, Cells: []Cell{{Col: 0, Text: "x"}}})
	}
	wb.Sheets = []*Sheet{sh}
	src, err := wb.Source(t.Context(), Options{})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(document(t, src, 0), `<div class="wc" style="max-height:30pt">word`) {
		t.Error("expected the text of the merged cell in a block of 30 points")
	}
	if n := pageCount(t, src); n != 1 {
		t.Errorf("expected 1 page, got: %d", n)
	}
}

func TestFixedRows(t *testing.T) {
	wb := &Workbook{Styles: []Style{
		{Font: Font{Family: "Arial", Size: 10}},
		{Font: Font{Family: "Arial", Size: 20}},
		{Font: Font{Family: "Arial", Size: 10}, Wrap: true},
	}}
	sh := &Sheet{
		Name: "Fixed", DefaultColWidth: 48, DefaultRowHeight: 12.75, Page: DefaultPage, FixedRows: true,
		Rows: []Row{
			// a line of 20 points in a row of 12.75 points
			{Index: 0, Cells: []Cell{{Col: 0, Text: "big", Style: 1}, {Col: 1, Text: "small"}}},
			// a text that wraps in a row of one line, and in a row of two
			{Index: 1, Cells: []Cell{{Col: 0, Text: "one\ntwo", Style: 2}}},
			{Index: 2, Height: 30, Cells: []Cell{{Col: 0, Text: "one\ntwo", Style: 2}}},
		},
	}
	wb.Sheets = []*Sheet{sh}
	src, err := wb.Source(t.Context(), Options{})
	if err != nil {
		t.Fatal(err)
	}
	doc := document(t, src, 0)
	for _, want := range []string{
		`<td class="s1 c" style="line-height:12.75pt">big</td><td>small</td>`,
		`<td class="s2 c" style="white-space:pre;overflow-wrap:normal">one two</td>`,
		`<tr style="height:30pt"><td class="s2 c"><div class="wc" style="max-height:30pt">one` + "\n" + `two</div></td>`,
	} {
		if !strings.Contains(doc, want) {
			t.Errorf("expected %q in the document:\n%s", want, doc)
		}
	}
	// without FixedRows, the rows grow for their text
	sh.FixedRows = false
	if src, err = wb.Source(t.Context(), Options{}); err != nil {
		t.Fatal(err)
	}
	if doc := document(t, src, 0); strings.Contains(doc, "line-height:12.75pt") || strings.Contains(doc, "white-space:pre;overflow-wrap:normal\"") {
		t.Errorf("expected the rows to grow:\n%s", doc)
	}
}

func TestHeaderScale(t *testing.T) {
	wb := &Workbook{Styles: []Style{{Font: Font{Family: "Calibri", Size: 10}}}}
	sh := &Sheet{
		Name: "Scaled", DefaultColWidth: 48, DefaultRowHeight: 15, Page: DefaultPage,
		Rows:   []Row{{Index: 0, Cells: []Cell{{Col: 0, Text: "x"}}}},
		Header: ParseHeaderFooter("&CTitle"),
	}
	sh.Page.Scale = 0.5
	wb.Sheets = []*Sheet{sh}
	for _, test := range []struct {
		scale bool
		exp   string
	}{
		{false, "font-size:10pt"},
		{true, "font-size:5pt"},
	} {
		sh.Page.ScaleHeaderFooter = test.scale
		src, err := wb.Source(t.Context(), Options{})
		if err != nil {
			t.Fatal(err)
		}
		if doc := document(t, src, 0); !strings.Contains(doc, `@top-center{content:"Title";vertical-align:top;padding-top:21.6pt;white-space:pre;font-family:"Calibri", sans-serif;`+test.exp+";}") {
			t.Errorf("expected the header with %s:\n%s", test.exp, doc)
		}
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

func TestSetPaper(t *testing.T) {
	page := DefaultPage
	page.SetPaper(ivy.Paper{})
	if page != DefaultPage {
		t.Errorf("expected the zero paper to keep the page, got: %+v", page)
	}
	page.SetPaper(ivy.A4.Landscape())
	exp := DefaultPage
	exp.Width, exp.Height, exp.Landscape = float64(ivy.A4.Width), float64(ivy.A4.Height), true
	if page != exp {
		t.Errorf("expected %+v, got: %+v", exp, page)
	}
	// a workbook without sheets takes the paper of the options
	src, err := (&Workbook{}).Source(t.Context(), Options{Paper: ivy.A4.Landscape()})
	if err != nil {
		t.Fatal(err)
	}
	if doc := document(t, src, 0); !strings.Contains(doc, "@page{size:841.89pt 595.28pt;margin:54pt 50.4pt 54pt 50.4pt;") {
		t.Errorf("expected A4 in landscape with the margins of Excel, got:\n%s", doc)
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
	// 511.2 / 1200 is 42.6%, and Excel fits at a whole percent
	if !strings.Contains(doc, `<col style="width:42pt">`) {
		t.Errorf("expected the columns at the scale 0.42:\n%s", doc)
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
	if n := strings.Count(doc, "<tr"); n > 90 {
		t.Errorf("expected at most 90 rows, got: %d", n)
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
	// Calibri of 11 points has a digit of 7 pixels on the screen, and of
	// 47 dots at 600 dots per inch. The default column of 8.43 characters
	// is 64 pixels, and Excel prints it 51.6 points wide.
	calibri := DigitOf(Font{Family: "Calibri", Size: 11}, 10)
	if calibri != (Digit{Pixels: 7, Print: 5.64}) {
		t.Errorf("expected a digit of 7 pixels and 5.64 points, got: %+v", calibri)
	}
	if got := calibri.Column(9.140625); math.Abs(got-51.566) > 0.01 {
		t.Errorf("expected the default column of 51.57 points, got: %v", got)
	}
	if got := calibri.DefaultColumn(8); math.Abs(got-51.566) > 0.01 {
		t.Errorf("expected the default column of 51.57 points from the base width, got: %v", got)
	}
	// a font without a size takes the default size: Arial of 10 points
	// has a digit of 7 pixels and of 46 dots, and Excel prints a column
	// of 80 pixels 63.1 points wide
	arial := DigitOf(Font{Family: "Arial"}, 10)
	if arial != (Digit{Pixels: 7, Print: 5.52}) {
		t.Errorf("expected a digit of 7 pixels and 5.52 points for Arial of 10 points, got: %+v", arial)
	}
	if got := arial.DefaultColumn(10); math.Abs(got-63.086) > 0.01 {
		t.Errorf("expected a column of 63.09 points, got: %v", got)
	}
	// Century Gothic of 9 points has a digit of 6.65 pixels, which Excel
	// rounds to 7, and of 42 dots: Excel prints a column of 22.43
	// characters 113.04 points wide
	gothic := DigitOf(Font{Family: "Century Gothic", Size: 9}, 11)
	if gothic != (Digit{Pixels: 7, Print: 5.04}) {
		t.Errorf("expected a digit of 7 pixels and 5.04 points for Century Gothic of 9 points, got: %+v", gothic)
	}
	if got := gothic.Column(22.42578125); math.Abs(got-113.04) > 0.01 {
		t.Errorf("expected a column of 113.04 points, got: %v", got)
	}
	for _, chars := range []float64{0, -1, math.NaN()} {
		if got := arial.Column(chars); got != 0 {
			t.Errorf("expected no width for %v characters, got: %v", chars, got)
		}
	}
	if w, h, ok := PaperSize(9); !ok || w != 595.28 || h != 841.89 {
		t.Errorf("expected A4, got: %v by %v, %v", w, h, ok)
	}
	if _, _, ok := PaperSize(1000); ok {
		t.Error("expected no paper 1000")
	}
}
