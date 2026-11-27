// This file is not a port. It tests the properties of D56:
// -ivy-line-numbers, -ivy-line-multiple and -ivy-page-top-space.

package layout

import (
	"fmt"
	"strings"
	"testing"
)

// wordStyle is the style of the tests of D56. A page holds 5 lines of
// 20px, with their baselines 16px below their tops, and the content of a
// page starts at x = 40px.
const wordStyle = `
      <style>
        @font-face { src: url(weasyprint.otf); font-family: weasyprint }
        @page { size: 200px 100px; margin: 0 0 0 40px }
        body { margin: 0; font: 20px/20px weasyprint }
        p { margin: 0; orphans: 1; widows: 1 }
      </style>`

// lineNumber is a number of a line that the layout draws.
type lineNumber struct {
	text     string
	right    float64
	baseline float64
}

// pageLineNumbers returns the numbers of the lines of each page, from the
// block of the numbers, the last child of the root box.
func pageLineNumbers(t *testing.T, pages []*Box) [][]lineNumber {
	t.Helper()
	out := make([][]lineNumber, 0, len(pages))
	for _, page := range pages {
		html := page.Children[0]
		var numbers []lineNumber
		if n := len(html.Children); n > 0 {
			block := html.Children[n-1]
			for _, line := range block.Children {
				if line.Kind != KindLine || len(line.Children) != 1 || line.Children[0].Kind != KindText {
					continue
				}
				text := line.Children[0]
				numbers = append(numbers, lineNumber{text.Text, text.PositionX + text.Width, text.PositionY + text.Baseline})
			}
		}
		out = append(out, numbers)
	}
	return out
}

// numberTexts returns the texts of the numbers of each page.
func numberTexts(numbers [][]lineNumber) string {
	pages := make([]string, 0, len(numbers))
	for _, page := range numbers {
		texts := make([]string, 0, len(page))
		for _, n := range page {
			texts = append(texts, n.text)
		}
		pages = append(pages, strings.Join(texts, " "))
	}
	return strings.Join(pages, " | ")
}

func TestLineNumbers(t *testing.T) {
	seven := strings.Repeat("<p>a</p>", 7)
	for _, tt := range []struct {
		name  string
		value string
		body  string
		want  string
	}{
		{"page", "10px 1 1 page", `<div>` + seven + `</div>`, "1 2 3 4 5 | 1 2"},
		{"continuous", "10px 1 1 continuous", `<div>` + seven + `</div>`, "1 2 3 4 5 | 6 7"},
		{"first", "10px 1 5 continuous", `<div>` + seven + `</div>`, "5 6 7 8 9 | 10 11"},
		{"step", "10px 2 1 continuous", `<div>` + seven + `</div>`, "2 4 | 6"},
		// Each line of a paragraph counts.
		{"lines", "10px 1 1 page", `<div><p>a<br>b<br>c</p></div>`, "1 2 3"},
		// A section starts at the first number again, even with the same
		// value as the section before it.
		{"section", "10px 1 1 section", `<div><p>a</p><p>b</p></div><div><p>c</p></div>`, "1 2 1"},
		{"section continuous", "10px 1 1 continuous", `<div><p>a</p><p>b</p></div><div><p>c</p></div>`, "1 2 3"},
		// The lines of a table, a float and an absolutely positioned box
		// have no number, and neither has a paragraph with 'none'.
		{"table", "10px 1 1 page", `<div><p>a</p><table><tr><td>b</td></tr></table><p>c</p></div>`, "1 2"},
		{"float", "10px 1 1 page", `<div><p>a</p><div style="float: left">b</div><p>c</p></div>`, "1 2"},
		{"absolute", "10px 1 1 page", `<div><p>a</p><div style="position: absolute">b</div><p>c</p></div>`, "1 2"},
		{"none", "10px 1 1 page", `<div><p>a</p><p style="-ivy-line-numbers: none">b</p><p>c</p></div>`, "1 2"},
		// The lines of an in-flow block in a section count.
		{"nested", "10px 1 1 page", `<div><p>a</p><div><p>b</p></div></div>`, "1 2"},
		// The property is not valid without the restart.
		{"invalid", "10px 1 1", `<div>` + seven + `</div>`, " | "},
		{"unknown keyword", "10px 1 1 page center", `<div>` + seven + `</div>`, " | "},
		// With no-empty, an empty line does not count, and a line with an
		// image does.
		{"empty", "10px 1 1 page", `<div><p>a</p><p><br></p><p>b</p></div>`, "1 2 3"},
		{"no-empty", "10px 1 1 page no-empty", `<div><p>a</p><p><br></p><p> </p><p>b</p><p><img src="pattern.png"></p></div>`, "1 2 3"},
		// A box with reset gives the first number to its first line, and
		// the lines after it go on from there.
		{"reset", "10px 1 1 continuous", `<div><p>a</p><p style="-ivy-line-numbers: 10px 1 7 continuous reset">b<br>c</p><p>d</p></div>`, "1 7 8 9"},
		{"reset nested", "10px 1 1 continuous", `<div><p>a</p><div style="-ivy-line-numbers: 10px 1 7 continuous reset"><p>b</p></div><p>c</p></div>`, "1 7 8"},
		// The part of the box on the next page goes on.
		{"reset pages", "10px 1 1 continuous", `<div>` + strings.Repeat("<p>a</p>", 4) +
			`<p style="-ivy-line-numbers: 10px 1 7 continuous reset">b<br>c<br>d</p></div>`, "1 2 3 4 7 | 8 9"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			body := strings.ReplaceAll(tt.body, "<div>", `<div style="-ivy-line-numbers: `+tt.value+`">`)
			pages := renderPages(t, wordStyle+body)
			if got := numberTexts(pageLineNumbers(t, pages)); got != tt.want {
				t.Errorf("got the numbers %q, want %q", got, tt.want)
			}
		})
	}
}

func TestLineNumbersPlace(t *testing.T) {
	// A number ends at the distance before the content of the section, and
	// has its baseline at the baseline of its line.
	pages := renderPages(t, wordStyle+`<div style="-ivy-line-numbers: 12pt 1 1 page; margin-left: 20px">`+
		`<p>a</p><p style="margin-left: 30px; line-height: 40px">b</p></div>`)
	numbers := pageLineNumbers(t, pages)
	if len(numbers) != 1 || len(numbers[0]) != 2 {
		t.Fatalf("got the numbers %v, want 2 numbers on 1 page", numbers)
	}
	// The root box holds the body and the block of the numbers.
	body := pages[0].Children[0].Children[0]
	section := only(t, body)
	for i, n := range numbers[0] {
		line := only(t, section.Children[i])
		if want := fmt.Sprint(i + 1); n.text != want {
			t.Errorf("got the number %q, want %q", n.text, want)
		}
		assertApprox(t, "the end of the number", n.right, 60-16)
		assertApprox(t, "the baseline of the number", n.baseline, line.PositionY+line.Baseline)
	}
}

func TestLineNumbersSide(t *testing.T) {
	// The content of a page is 160px wide from x = 40px. A number on the
	// right starts at the distance after the content, and inner and outer
	// change sides from the first page, a right page, to the second.
	for _, tt := range []struct {
		side        string
		left, right bool
	}{
		{"right", false, false},
		{"inner", true, false},
		{"outer", false, true},
	} {
		t.Run(tt.side, func(t *testing.T) {
			pages := renderPages(t, wordStyle+`<div style="-ivy-line-numbers: 10px 1 1 page `+tt.side+`">`+
				strings.Repeat("<p>a</p>", 6)+`</div>`)
			numbers := pageLineNumbers(t, pages)
			if len(numbers) != 2 {
				t.Fatalf("got the numbers %v, want 2 pages", numbers)
			}
			for i, left := range [...]bool{tt.left, tt.right} {
				n := numbers[i][0]
				if left {
					assertApprox(t, "the end of the number", n.right, 30)
				} else if start := n.right - 20; start < 209 || start > 211 {
					t.Errorf("page %d: got a number from %v, want it from 210", i+1, start)
				}
			}
		})
	}
}

func TestLineNumbersDefault(t *testing.T) {
	pages := renderPages(t, wordStyle+`<p>a</p>`)
	_, body := pageBody(t, pages[0])
	if got := str(body.Style.Get("line_numbers")); got != "none" {
		t.Errorf("got %q, want none", got)
	}
	if got := numberTexts(pageLineNumbers(t, pages)); got != "" {
		t.Errorf("got the numbers %q, want none", got)
	}
}

func TestLineMultiple(t *testing.T) {
	for _, tt := range []struct {
		name       string
		lineHeight string
		multiple   string
		textTop    float64
	}{
		// CSS puts 10px of the 20px of leading above the text.
		{"none", "40px", "none", 10},
		// With a multiple of 2, the single line is 20px high, as the
		// text, so the text is at the top of the line.
		{"double", "40px", "2", 0},
		// The single line of 25px has a line gap of 5px, above the text.
		// CSS puts 15px above it.
		{"gap", "50px", "2", 5},
		// The single line of a multiple of 4 is 10px high, less than the
		// text, which stays at the top of the line.
		{"four", "40px", "4", 0},
		// The single line of 32px has a line gap of 12px, above the text.
		// CSS puts 10px above it.
		{"small", "40px", "1.25", 12},
		// A single line puts its whole leading above the text.
		{"single", "25px", "1", 5},
		{"one", "40px", "1", 20},
		// A multiple below 1 keeps the rule of CSS.
		{"half", "40px", "0.5", 10},
		{"invalid", "40px", "-2", 10},
	} {
		t.Run(tt.name, func(t *testing.T) {
			pages := renderPages(t, wordStyle+`<p style="line-height: `+tt.lineHeight+`; -ivy-line-multiple: `+tt.multiple+`">a<span>b</span></p><p>c</p>`)
			_, body := pageBody(t, pages[0])
			p1, p2 := body.Children[0], body.Children[1]
			height := p1.Height
			assertApprox(t, "the top of the next paragraph", p2.PositionY, height)
			if want := tt.lineHeight; fmt.Sprint(height)+"px" != want {
				t.Errorf("got the height %v of the paragraph, want %s", height, want)
			}
			line := only(t, p1)
			text, span := line.Children[0], line.Children[1]
			assertNear(t, "the top of the text", text.ContentBoxY(), tt.textTop)
			assertNear(t, "the top of the span", span.ContentBoxY(), tt.textTop)
			assertNear(t, "the baseline of the line", line.PositionY+line.Baseline, tt.textTop+16)
		})
	}
}

func TestLineMultipleTextBottom(t *testing.T) {
	// The test font of emoji has an ascent of 950 and a descent of 250 in
	// 1024 units, so its text of 20px is 23.4375px high, from 18.5547px
	// above the baseline. The single line of 30px puts its leading of
	// 6.5625px above the text, so the text ends at the bottom of the line.
	// A box of 50px at the bottom of the text then ends the line, which is
	// 50px high. Without -ivy-line-multiple, the content area of upstream
	// is 20px high, with half of the leading of 10px above it, so it ends
	// 5px above the line, which is 55px high.
	for _, tt := range []struct {
		multiple string
		height   float64
	}{
		{"1", 50},
		{"none", 55},
	} {
		t.Run(tt.multiple, func(t *testing.T) {
			pages := renderPages(t, `<style>@font-face { src: url(NotoColorEmojiCOLR.subset.ttf); font-family: emoji }
				@page { size: 200px } body { margin: 0 }</style>
				<p style="margin: 0; font: 20px/30px emoji; -ivy-line-multiple: `+tt.multiple+`"><span style="display: inline-block; width: 10px; height: 50px; vertical-align: text-bottom"></span></p>`)
			_, body := pageBody(t, pages[0])
			line := only(t, body.Children[0])
			assertNear(t, "the height of the line", line.Height, tt.height)
			assertNear(t, "the top of the box", line.Children[0].PositionY, 0)
		})
	}
}

func TestPageTopSpace(t *testing.T) {
	five := strings.Repeat("<p>a</p>", 5)
	for _, tt := range []struct {
		name  string
		space string
		body  string
		want  float64
	}{
		// The sixth paragraph does not fit on the first page. Its margin of
		// 10px at the top of the second page goes with 'discard'. The
		// margin-break of CSS discards it too, but not in a named page,
		// whose breaks upstream counts as forced.
		{"keep", "keep", five + `<p style="margin-top: 10px">b</p>`, 0},
		{"discard", "discard", five + `<p style="margin-top: 10px">b</p>`, 0},
		{"named keep", "keep", `<div style="page: s1">` + five + `<p style="margin-top: 10px">b</p></div>`, 10},
		{"named", "discard", `<div style="page: s1">` + five + `<p style="margin-top: 10px">b</p></div>`, 0},
		// The padding without a border goes too, and a padding with a top
		// border stays.
		{"padding keep", "keep", five + `<p style="padding-top: 10px">b</p>`, 10},
		{"padding", "discard", five + `<p style="padding-top: 10px">b</p>`, 0},
		{"border", "discard", five + `<p style="padding-top: 10px; border-top: 1px solid">b</p>`, 11},
		{"background", "discard", five + `<p style="padding-top: 10px; background: red">b</p>`, 10},
		// A forced break keeps the space.
		{"forced", "discard", `<p>a</p><p style="break-before: page; margin-top: 10px">b</p>`, 10},
	} {
		t.Run(tt.name, func(t *testing.T) {
			body := `<div style="-ivy-page-top-space: ` + tt.space + `"><div>` + tt.body + `</div></div>`
			pages := renderPages(t, wordStyle+body)
			if len(pages) != 2 {
				t.Fatalf("got %d pages, want 2", len(pages))
			}
			var text *Box
			pages[1].Descendants(func(b *Box) bool {
				if b.Kind == KindText && text == nil {
					text = b
				}
				return true
			})
			if text == nil {
				t.Fatal("no text on the second page")
			}
			assertApprox(t, "the top of the text", text.PositionY, tt.want)
		})
	}
}

func TestPageTopSpaceColumns(t *testing.T) {
	five := strings.Repeat("<p>a</p>", 5)
	for _, tt := range []struct {
		name  string
		space string
		body  string
		page  int
		x, y  float64
	}{
		// The sixth paragraph starts the second column, and its margin
		// of 10px goes with 'discard'.
		{"keep", "keep", five + `<p style="margin-top: 10px">b</p>`, 0, 120, 10},
		{"discard", "discard", five + `<p style="margin-top: 10px">b</p>`, 0, 120, 0},
		// The first block of a block that starts a column loses its
		// space too.
		{"nested", "discard", five + `<div><p style="margin-top: 10px">b</p></div>`, 0, 120, 0},
		// The eleventh paragraph starts the first column of the second
		// page.
		{"page keep", "keep", five + five + `<p style="margin-top: 10px">b</p>`, 1, 40, 10},
		{"page", "discard", five + five + `<p style="margin-top: 10px">b</p>`, 1, 40, 0},
		// A column break keeps the space, as a page break does.
		{"forced", "discard", `<p>a</p><p style="break-before: column; margin-top: 10px">b</p>`, 0, 120, 10},
		// The first column of the first page keeps the space.
		{"first", "discard", `<p style="margin-top: 10px">b</p>`, 0, 40, 10},
	} {
		t.Run(tt.name, func(t *testing.T) {
			body := `<div style="-ivy-page-top-space: ` + tt.space + `; columns: 2; column-gap: 0; column-fill: auto">` + tt.body + `</div>`
			pages := renderPages(t, wordStyle+body)
			for i, page := range pages {
				var text *Box
				page.Descendants(func(b *Box) bool {
					if b.Kind == KindText && b.Text == "b" && text == nil {
						text = b
					}
					return true
				})
				if text == nil {
					continue
				}
				if i != tt.page {
					t.Fatalf("the text is on page %d, want %d", i+1, tt.page+1)
				}
				assertApprox(t, "the left of the text", text.PositionX, tt.x)
				assertApprox(t, "the top of the text", text.PositionY, tt.y)
				return
			}
			t.Fatal("no text b")
		})
	}
}

func TestPageTopSpaceCell(t *testing.T) {
	four := strings.Repeat("<p>a</p>", 4)
	for _, tt := range []struct {
		name  string
		space string
		body  string
		want  float64
	}{
		// The cell goes on at the top of the second page, and the first
		// paragraph there loses its space before with 'discard'.
		{"keep", "keep", `<table><tr><td>` + four + `<p style="margin-top: 10px">b</p></td></tr></table>`, 10},
		{"discard", "discard", `<table><tr><td>` + four + `<p style="margin-top: 10px">b</p></td></tr></table>`, 0},
		// A row that starts on the second page keeps the space.
		{"row", "discard", `<table><tr><td>` + four + `</td></tr><tr><td><p style="margin-top: 10px">b</p></td></tr></table>`, 10},
	} {
		t.Run(tt.name, func(t *testing.T) {
			body := `<div style="-ivy-page-top-space: ` + tt.space + `">` + tt.body + `</div>`
			pages := renderPages(t, wordStyle+`<style>table { border-collapse: collapse } td { padding: 0 }</style>`+body)
			if len(pages) != 2 {
				t.Fatalf("got %d pages, want 2", len(pages))
			}
			var text *Box
			pages[1].Descendants(func(b *Box) bool {
				if b.Kind == KindText && b.Text == "b" && text == nil {
					text = b
				}
				return true
			})
			if text == nil {
				t.Fatal("no text b on the second page")
			}
			assertApprox(t, "the top of the text", text.PositionY, tt.want)
		})
	}
}

func TestPageTopSpaceFirstPage(t *testing.T) {
	// The first page keeps the space before its first paragraph.
	pages := renderPages(t, wordStyle+`<div style="-ivy-page-top-space: discard"><p style="margin-top: 10px">a</p></div>`)
	_, body := pageBody(t, pages[0])
	p := only(t, only(t, body))
	assertApprox(t, "the top of the paragraph", p.BorderBoxY(), 10)
}

// assertNear fails the test when got is not want within 0.01, the
// rounding of the metrics of the font.
func assertNear(t *testing.T, name string, got, want float64) {
	t.Helper()
	if got < want-0.01 || got > want+0.01 {
		t.Errorf("%s = %v, want %v", name, got, want)
	}
}

// pageLinesCells returns the line boxes of the text of a page, in their
// order, and the boxes of the table cells of the page.
func pageLinesCells(page *Box) (lines, cells []*Box) {
	page.Descendants(func(b *Box) bool {
		switch b.Kind {
		case KindLine:
			lines = append(lines, b)
		case KindTableCell:
			cells = append(cells, b)
		}
		return true
	})
	return lines, cells
}

func TestLineMultipleLast(t *testing.T) {
	// A line of 26px with a multiple of 1.3 has a single line of 20px and
	// 6px of extra space below its text. Three lines and the text of a
	// fourth one fit on the page of 100px, but four whole lines do not.
	six := strings.Repeat("a<br>", 5) + "a"
	for _, tt := range []struct {
		name     string
		multiple string
		cell     bool
		// lines is the number of lines on each page, bottom the bottom
		// of the last line of the first page, and height the height of
		// the cell on the last page.
		lines  []int
		bottom float64
		height float64
	}{
		{"page", "1.3", false, []int{3, 3}, 78, 0},
		// With 'last', the last line of the page loses its extra space,
		// as in LibreOffice.
		{"page last", "1.3 last", false, []int{4, 2}, 98, 0},
		{"cell", "1.3", true, []int{3, 3}, 78, 78},
		// In a cell, the last line of each part of the cell loses its
		// extra space: the last line of the page, and the last line of
		// the cell.
		{"cell last", "1.3 last", true, []int{4, 2}, 98, 46},
		{"invalid", "1.3 first", false, []int{3, 3}, 78, 0},
	} {
		t.Run(tt.name, func(t *testing.T) {
			p := `<p style="line-height: 26px; -ivy-line-multiple: ` + tt.multiple + `">` + six + `</p>`
			if tt.cell {
				p = `<table><tr><td>` + p + `</td></tr></table>`
			}
			pages := renderPages(t, wordStyle+`<style>table { border-collapse: collapse } td { padding: 0 }</style>`+p)
			if len(pages) != len(tt.lines) {
				t.Fatalf("got %d pages, want %d", len(pages), len(tt.lines))
			}
			for i, page := range pages {
				lines, cells := pageLinesCells(page)
				if len(lines) != tt.lines[i] {
					t.Fatalf("page %d: got %d lines, want %d", i+1, len(lines), tt.lines[i])
				}
				if i == 0 {
					last := lines[len(lines)-1]
					assertApprox(t, "the bottom of the last line", last.PositionY+last.Height, tt.bottom)
				}
				if tt.cell && i == len(pages)-1 {
					if len(cells) != 1 {
						t.Fatalf("page %d: got %d cells, want 1", i+1, len(cells))
					}
					assertApprox(t, "the height of the cell", cells[0].Height, tt.height)
				}
			}
		})
	}
}

func TestPageTopSpaceCellLength(t *testing.T) {
	// The cell goes on at the top of the second page. Its first paragraph
	// there keeps 4px of its space before with the length of
	// -ivy-page-top-space, as a paragraph of LibreOffice keeps its own
	// space above it and not the space below the paragraph before it.
	style := `-ivy-page-top-space: discard 4px`
	for _, tt := range []struct {
		name, body, text string
	}{
		{"start", strings.Repeat("<p>a</p>", 4) + `<p style="margin-top: 10px; ` + style + `">b</p>`, "b"},
		// A paragraph that goes on from the page before keeps it too.
		{"continued", strings.Repeat("<p>a</p>", 3) + `<p style="margin-top: 10px; ` + style + `">b<br>c</p>`, "c"},
		// The space is at most the margin.
		{"margin", strings.Repeat("<p>a</p>", 4) + `<p style="margin-top: 2px; ` + style + `">b</p>`, "b"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			body := `<div style="-ivy-page-top-space: discard"><table><tr><td>` + tt.body + `</td></tr></table></div>`
			pages := renderPages(t, wordStyle+`<style>table { border-collapse: collapse } td { padding: 0 }</style>`+body)
			if len(pages) != 2 {
				t.Fatalf("got %d pages, want 2", len(pages))
			}
			var text *Box
			pages[1].Descendants(func(b *Box) bool {
				if b.Kind == KindText && b.Text == tt.text && text == nil {
					text = b
				}
				return true
			})
			if text == nil {
				t.Fatalf("no text %s on the second page", tt.text)
			}
			want := 4.0
			if tt.name == "margin" {
				want = 2
			}
			assertApprox(t, "the top of the text", text.PositionY, want)
		})
	}
}
