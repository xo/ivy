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
		// A multiple up to 1 keeps the rule of CSS.
		{"one", "40px", "1", 10},
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
