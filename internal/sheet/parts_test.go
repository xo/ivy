package sheet

import (
	"context"
	"crypto/sha256"
	enchex "encoding/hex"
	"image"
	"image/color"
	"strconv"
	"strings"
	"testing"

	"github.com/xo/ivy"
	"github.com/xo/ivy/internal/layout"
)

// pageHashes returns the hash of the pixels of each page of the book of
// src, at 72 dots per inch. order gives the pages that it renders first,
// before PageCount, such as the first page or a page far into the book.
func pageHashes(t *testing.T, src layout.Source, order ...int) []string {
	t.Helper()
	ctx := t.Context()
	book := layout.NewBook(src, layout.BookOptions{FontFiles: officeFonts(t)})
	early := map[int]string{}
	for _, n := range order {
		early[n] = pageHash(ctx, t, book, n)
	}
	n, err := book.PageCount(ctx)
	if err != nil {
		t.Fatal(err)
	}
	hashes := make([]string, n)
	for i := range n {
		hashes[i] = pageHash(ctx, t, book, i)
		if h, ok := early[i]; ok && h != hashes[i] {
			t.Errorf("page %d: expected the same pixels before and after PageCount", i+1)
		}
	}
	return hashes
}

// pageHash returns the hash of the pixels of page n of book.
func pageHash(ctx context.Context, t *testing.T, book *layout.Book, n int) string {
	t.Helper()
	page, err := book.Page(ctx, n)
	if err != nil {
		t.Fatalf("page %d: %v", n+1, err)
	}
	img, err := ivy.RenderImage(ctx, page, 72, color.White)
	if err != nil {
		t.Fatalf("page %d: %v", n+1, err)
	}
	return hashImage(img)
}

// hashImage returns a hash of the size and the pixels of img.
func hashImage(img image.Image) string {
	h := sha256.New()
	b := img.Bounds()
	h.Write([]byte(b.String()))
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			c := nrgba(img.At(x, y))
			h.Write([]byte{c.R, c.G, c.B, c.A})
		}
	}
	return enchex.EncodeToString(h.Sum(nil)[:8])
}

// partsMatch lays out wb in parts of n pages, with opts, and reports a
// page that differs from the hashes want of the whole sheet. It returns the
// splitter of the parts.
func partsMatch(t *testing.T, wb *Workbook, want []string, n int, opts Options) *splitter {
	t.Helper()
	ctx := t.Context()
	src, err := wb.Source(WithPartPages(ctx, n), opts)
	if err != nil {
		t.Fatal(err)
	}
	s, _ := src.Splitter.(*splitter)
	if s == nil {
		t.Fatal("expected parts")
	}
	for _, order := range [][]int{{0}, {len(want) - 1, 0}} {
		got := pageHashes(t, src, order...)
		if len(got) != len(want) {
			t.Fatalf("parts of %d pages: expected %d pages, got: %d", n, len(want), len(got))
		}
		for i := range want {
			if got[i] != want[i] {
				t.Errorf("parts of %d pages: page %d of %d differs", n, i+1, len(want))
			}
		}
	}
	return s
}

// partsOf returns the number of parts of the sheets of s, the number of
// cuts inside a table, and the number of cuts with lead rows or tail rows
// of a merged cell.
func partsOf(s *splitter) (parts, inner, merged int) {
	for _, p := range s.sheets {
		parts += len(p.parts)
		for _, c := range p.parts {
			if c.r == 0 {
				continue
			}
			inner++
			lead, tail := p.tables[c.t].edges(c.r)
			if lead < c.r-1 || tail > c.r {
				merged++
			}
		}
	}
	return parts, inner, merged
}

// checkParts lays out the sheets of wb whole and in parts of 1 page and of
// 4 pages, and reports a page that differs. It returns the splitters of the
// parts of 1 page and of 4 pages.
func checkParts(t *testing.T, wb *Workbook, opts Options) [2]*splitter {
	t.Helper()
	whole, err := wb.Source(WithPartPages(t.Context(), 0), opts)
	if err != nil {
		t.Fatal(err)
	}
	if whole.Splitter != nil {
		t.Fatal("expected no parts")
	}
	want := pageHashes(t, whole)
	var out [2]*splitter
	for k, n := range []int{1, 4} {
		if raceDetector && n == 1 {
			continue
		}
		s := partsMatch(t, wb, want, n, opts)
		parts, inner, merged := partsOf(s)
		t.Logf("parts of %d pages: %d parts, %d cuts in a table, %d with merged cells, %d recuts",
			n, parts, inner, merged, s.recuts.Load())
		out[k] = s
	}
	return out
}

// skipRace skips a test of the parts when the race detector runs, which
// makes it too slow. TestPartsPlain, TestPartsMerged and TestPartsWrap still
// check the goroutines of the parts and the parts that the book cuts again.
func skipRace(t *testing.T) {
	t.Helper()
	if raceDetector {
		t.Skip("the race detector makes the layout of the parts too slow")
	}
}

// Styles of the workbooks of the tests of the parts.
const (
	stylePlain = iota
	styleWide
	styleDouble
	styleWrap
	styleSides
	styleBold
)

// partStyles returns the styles of the workbooks of the tests of the parts:
// a wide border at the bottom, a double border at the top, text that
// wraps, wide borders at the sides and a bold font.
func partStyles() []Style {
	font := Font{Family: "Calibri", Size: 11}
	return []Style{
		stylePlain:  {Font: font},
		styleWide:   {Font: font, Borders: [4]Border{2: {Width: 3, Line: "solid"}}},
		styleDouble: {Font: font, Borders: [4]Border{0: {Width: 2.25, Line: "double"}}},
		styleWrap:   {Font: font, Wrap: true, VAlign: "top"},
		styleSides:  {Font: font, Borders: [4]Border{1: {Width: 3, Line: "solid"}, 3: {Width: 3, Line: "solid"}}},
		styleBold:   {Font: Font{Family: "Calibri", Size: 11, Bold: true}, Fill: color.NRGBA{R: 0xdd, G: 0xee, B: 0xff, A: 0xff}},
	}
}

// mergedSheet returns a sheet of n rows with merged cells of 7 rows and of
// 3 rows by 2 columns, wide and double borders, text that wraps, rows of
// 30 points, print titles, a footer with the page and the number of pages,
// and a first row with wider side borders than the other rows.
func mergedSheet(n int) *Sheet {
	sh := &Sheet{Name: "Merged", DefaultColWidth: 80, DefaultRowHeight: 15, Page: DefaultPage, Gridlines: true}
	sh.TitleRows = &Span{First: 0, Last: 1}
	sh.Footer = ParseHeaderFooter("&CPage &P of &N")
	sh.Rows = append(sh.Rows,
		Row{Index: 0, Cells: []Cell{{Col: 0, Text: "Title", Style: styleBold}, {Col: 1, Text: "B", Style: styleBold}, {Col: 4, Text: "E", Style: styleBold}}},
		Row{Index: 1, Cells: []Cell{{Col: 0, Text: "Sub", Style: styleSides}, {Col: 2, Text: "C", Style: styleSides}}},
	)
	for i := 2; i < n; i++ {
		row := Row{Index: i}
		if i%11 == 0 {
			row.Height = 30
		}
		row.Cells = append(row.Cells, Cell{Col: 0, Text: "r" + strconv.Itoa(i)})
		switch i % 9 {
		case 0:
			row.Cells = append(row.Cells, Cell{Col: 1, Text: "wide", Style: styleWide})
		case 4:
			row.Cells = append(row.Cells, Cell{Col: 1, Text: "double", Style: styleDouble})
		}
		if i%13 == 2 {
			row.Cells = append(row.Cells, Cell{Col: 2, Text: "merged 3x2", Style: styleWide})
		}
		if i%17 == 0 {
			row.Cells = append(row.Cells, Cell{Col: 4, Text: "a long text that wraps in its cell, so that the row grows", Style: styleWrap})
		}
		sh.Rows = append(sh.Rows, row)
		if i%10 == 3 && i+6 < n {
			sh.Merges = append(sh.Merges, Range{Row0: i, Col0: 0, Row1: i + 6, Col1: 0})
		}
		if i%13 == 2 && i+2 < n {
			sh.Merges = append(sh.Merges, Range{Row0: i, Col0: 2, Row1: i + 2, Col1: 3})
		}
	}
	return sh
}

// picturesSheet returns a sheet of n rows that is wider than its page, with
// a title column in each band, fit to the width of one page, 19 frames of
// pictures in slices, and a header with the page.
func picturesSheet(n int) *Sheet {
	sh := &Sheet{Name: "Pictures", DefaultColWidth: 70, DefaultRowHeight: 15, Page: DefaultPage, Gridlines: true}
	sh.Page.Fit, sh.Page.FitWidth = true, 2
	sh.TitleCols = &Span{First: 0, Last: 0}
	sh.Header = ParseHeaderFooter("&L&P&R&A")
	for i := range n {
		row := Row{Index: i}
		for c := range 18 {
			if (i+c)%5 == 0 {
				row.Cells = append(row.Cells, Cell{Col: c, Text: strconv.Itoa(i*100 + c), Kind: KindNumber})
			}
		}
		sh.Rows = append(sh.Rows, row)
	}
	for k := range 19 {
		sh.Images = append(sh.Images, Image{Row: 5 + k*35, Col: 2 + k%9, DX: 4, DY: 3, Width: 150, Height: 15 * float64(10+k%25), Label: "Picture " + strconv.Itoa(k+1)})
	}
	return sh
}

// wrapSheet returns a sheet of n rows of narrow columns whose text wraps,
// so that the rows are taller than their heights, and the cuts of the
// parts are not at the page breaks.
func wrapSheet(n int) *Sheet {
	sh := &Sheet{Name: "Wrap", DefaultColWidth: 40, DefaultRowHeight: 15, Page: DefaultPage}
	for i := range n {
		row := Row{Index: i, Cells: []Cell{{Col: 0, Text: strconv.Itoa(i)}}}
		if i%7 == 3 || i%5 == 1 {
			row.Cells = append(row.Cells, Cell{Col: 1, Text: "text that wraps in a narrow column " + strconv.Itoa(i), Style: styleWrap})
		}
		if i%23 == 0 {
			row.Cells = append(row.Cells, Cell{Col: 2, Text: "border", Style: styleWide})
		}
		sh.Rows = append(sh.Rows, row)
		if i%19 == 4 && i+3 < n {
			sh.Merges = append(sh.Merges, Range{Row0: i, Col0: 3, Row1: i + 3, Col1: 4})
		}
	}
	return sh
}

func TestPartsPlain(t *testing.T) {
	sh := &Sheet{Name: "Plain", DefaultColWidth: 100, DefaultRowHeight: 14.52, Page: DefaultPage, Gridlines: true}
	for i := range 600 {
		sh.Rows = append(sh.Rows, Row{Index: i, Cells: []Cell{{Col: 0, Text: "row " + strconv.Itoa(i)}, {Col: 1, Text: "x"}}})
	}
	checkParts(t, &Workbook{Sheets: []*Sheet{sh}}, Options{})
}

func TestPartsMerged(t *testing.T) {
	checkParts(t, &Workbook{Styles: partStyles(), Sheets: []*Sheet{mergedSheet(700)}}, Options{})
}

func TestPartsPictures(t *testing.T) {
	skipRace(t)
	checkParts(t, &Workbook{Styles: partStyles(), Sheets: []*Sheet{picturesSheet(700)}}, Options{})
}

func TestPartsWrap(t *testing.T) {
	// the rows that wrap are taller than the heights that predict the
	// cuts, so probe rows fail and the book cuts again
	wb := &Workbook{Styles: partStyles(), Sheets: []*Sheet{wrapSheet(700)}}
	for k, s := range checkParts(t, wb, Options{}) {
		if s != nil && s.recuts.Load() == 0 {
			t.Errorf("parts %d: expected a probe row that fails", k)
		}
	}
}

// plainSheet returns a sheet of n rows of two columns.
func plainSheet(n int) *Sheet {
	sh := &Sheet{Name: "Sheet", DefaultColWidth: 100, DefaultRowHeight: 15, Page: DefaultPage, Gridlines: true}
	for i := range n {
		sh.Rows = append(sh.Rows, Row{Index: i, Cells: []Cell{{Col: 0, Text: "row " + strconv.Itoa(i)}, {Col: 1, Text: strconv.Itoa(i * 7), Kind: KindNumber}}})
	}
	return sh
}

func TestPartsTallPicture(t *testing.T) {
	skipRace(t)
	// a picture of 1100 rows shows whole in its first cell: no part
	// starts inside its rows
	sh := plainSheet(1400)
	sh.Images = []Image{{Row: 100, Col: 2, Width: 200, Height: 15 * 1100, Label: "Tall"}}
	wb := &Workbook{Sheets: []*Sheet{sh}}
	for k, s := range checkParts(t, wb, Options{}) {
		p := s.sheets["sheet1.html"]
		for _, c := range p.parts {
			if c.r > 100 && c.r <= 1200 {
				t.Errorf("parts %d: a part starts at the row %d, inside the picture", k, c.r)
			}
		}
		if len(p.parts) < 3 {
			t.Errorf("parts %d: expected parts before and after the picture, got %d parts", k, len(p.parts))
		}
	}
}

func TestPartsOverThenDown(t *testing.T) {
	skipRace(t)
	// two bands of columns in the order over, then down: each table is a
	// page, and a part starts at a table
	sh := plainSheet(500)
	sh.Cols = []Col{{Min: 0, Max: 1, Width: 400}}
	sh.Page.OverThenDown = true
	s := checkParts(t, &Workbook{Sheets: []*Sheet{sh}}, Options{})[0]
	for _, c := range s.sheets["sheet1.html"].parts {
		if c.r != 0 {
			t.Errorf("expected parts that start at a table, got the row %d of the table %d", c.r, c.t)
		}
	}
}

func TestPartsBandsDown(t *testing.T) {
	skipRace(t)
	// two bands of columns in the order down, then over, with a title
	// column, and print titles of rows: the second band is a table that
	// starts a page
	sh := plainSheet(400)
	sh.Cols = []Col{{Min: 0, Max: 0, Width: 150}, {Min: 1, Max: 1, Width: 400}}
	sh.TitleCols = &Span{First: 0, Last: 0}
	sh.TitleRows = &Span{First: 0, Last: 0}
	s := checkParts(t, &Workbook{Sheets: []*Sheet{sh}}, Options{})[1]
	tables := map[int]bool{}
	for _, c := range s.sheets["sheet1.html"].parts {
		tables[c.t] = true
	}
	if len(tables) != 2 {
		t.Errorf("expected parts in 2 tables, got %d", len(tables))
	}
}

func TestPartsBreaks(t *testing.T) {
	skipRace(t)
	// manual breaks at the rows where the parts start, and next to them
	sh := plainSheet(700)
	src, err := (&Workbook{Sheets: []*Sheet{sh}}).Source(WithPartPages(t.Context(), 4), Options{})
	if err != nil {
		t.Fatal(err)
	}
	p := src.Splitter.(*splitter).sheets["sheet1.html"]
	for k, c := range p.parts[1:] {
		switch k % 3 {
		case 0:
			sh.RowBreaks = append(sh.RowBreaks, c.r)
		case 1:
			sh.RowBreaks = append(sh.RowBreaks, c.r-1)
		case 2:
			sh.RowBreaks = append(sh.RowBreaks, c.r+1)
		}
	}
	checkParts(t, &Workbook{Sheets: []*Sheet{sh}}, Options{})
}

func TestPartsComments(t *testing.T) {
	skipRace(t)
	// the comments at the end and the note follow the last part
	sh := plainSheet(400)
	sh.Page.CommentsAtEnd = true
	for i := range 60 {
		sh.Comments = append(sh.Comments, Comment{Row: i * 6, Col: 1, Author: "Author", Text: "A comment on the row " + strconv.Itoa(i*6)})
	}
	sh.Note = "A note after the cells."
	checkParts(t, &Workbook{Sheets: []*Sheet{sh}}, Options{})
}

func TestPartsLimit(t *testing.T) {
	skipRace(t)
	// the sheet stops at the limit of the cells in the middle of its rows,
	// and its note says so
	wb := &Workbook{Sheets: []*Sheet{plainSheet(2000)}}
	opts := Options{Limits: ivy.Limits{MaxObjects: 8000}}
	src, err := wb.Source(t.Context(), opts)
	if err != nil {
		t.Fatal(err)
	}
	if doc := document(t, src, 0); !strings.Contains(doc, "The sheet continues past the limit") {
		t.Fatal("expected the note of the limit")
	}
	checkParts(t, wb, opts)
}

func TestPartsFitPage(t *testing.T) {
	skipRace(t)
	// the scale fits the rows on 10 pages, the first row has wider borders
	// at its sides than the other rows, and the table is centered
	sh := plainSheet(900)
	sh.Page.Fit, sh.Page.FitWidth, sh.Page.FitHeight = true, 1, 10
	sh.Page.CenterH = true
	sh.Rows[0].Cells[0].Style = styleSides
	sh.Rows[0].Cells[1].Style = styleSides
	sh.Header = ParseHeaderFooter("&L&P of &N&R&A")
	checkParts(t, &Workbook{Styles: partStyles(), Sheets: []*Sheet{sh}}, Options{})
}
