// This file is not a port. It tests -ivy-page-overflow (D52).

package layout

import (
	"fmt"
	"testing"
)

// pageOverflowStyle is the style of the tests of -ivy-page-overflow. A
// line is 20px high, with its baseline at 16px. The text of weasyprint.otf
// goes 4px under the baseline, so the descent of a line is 4px.
const pageOverflowStyle = `
      <style>
        @font-face { src: url(weasyprint.otf); font-family: weasyprint }
        @page { size: 200px %s; margin: 0 }
        html { -ivy-page-overflow: %s }
        body { margin: 0; font: 20px/20px weasyprint }
        p { margin: 0; orphans: 2; widows: 2 }
      </style>`

// pageLines returns the number of lines of each page. The lines in an
// inline block do not count.
func pageLines(t *testing.T, pages []*Box) []int {
	t.Helper()
	lines := make([]int, 0, len(pages))
	for _, page := range pages {
		_, body := pageBody(t, page)
		n := 0
		body.Descendants(func(b *Box) bool {
			if b.Kind != KindInlineBlock {
				for _, child := range b.Children {
					if child.Kind == KindLine {
						n++
					}
				}
			}
			return true
		})
		lines = append(lines, n)
	}
	return lines
}

func TestPageOverflow(t *testing.T) {
	four := `<p>a</p><p>b</p><p>c</p><p>d</p>`
	five := four + `<p><br></p>`
	for _, tt := range []struct {
		name     string
		height   string
		overflow string
		body     string
		want     []int
	}{
		// The fifth line is empty. It passes the bottom of the page by
		// 2px, less than its descent, the descent of the font of its
		// paragraph.
		{"descent", "98px", "descent", five, []int{5}},
		{"none", "98px", "none", five, []int{4, 1}},
		// A line of spaces has no text either.
		{"spaces", "98px", "descent", four + `<p style="white-space: pre">  </p>`, []int{5}},
		// A line with text goes to the next page, even when it passes the
		// bottom of the page by less than its descent.
		{"text", "98px", "descent", four + `<p>e</p>`, []int{4, 1}},
		// The fifth line passes the bottom of the page by 5px, more than
		// its descent.
		{"past the descent", "95px", "descent", five, []int{4, 1}},
		// The descent of the last line also lets its parents pass the
		// bottom of the page.
		{"nested", "98px", "descent", `<div><div>` + five + `</div></div>`, []int{5}},
		// The padding of the paragraph adds to the overflow of its line.
		{"padding", "98px", "descent", four + `<p style="padding-bottom: 3px"><br></p>`, []int{4, 1}},
		// The empty fifth line fits by its descent, but the sixth line
		// would be a widow. The paragraph breaks before the fifth line, as
		// it does without the property.
		{"widows", "98px", "descent", `<p>a<br>b<br>c<br>d<br><br>f</p>`, []int{4, 2}},
		// The text in an inline block does not count. The line has the
		// descent of its strut, and passes the bottom of the page by
		// 22px.
		{"inline block", "98px", "descent", four + `<p><span style="display: inline-block; vertical-align: top">e<br>f</span></p>`, []int{4, 1}},
		// A table row checks the bottom of the page without the descent,
		// so the lines of its cells keep the rule of upstream. The row
		// breaks before the empty second line of the cell, and does not
		// go to the next page as a whole.
		{"table", "98px", "descent", `<p>a</p><p>b</p><p>c</p><table style="border-spacing: 0"><tr><td style="padding: 0; orphans: 1; widows: 1">d<br><br></td></tr></table>`, []int{4, 1}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			pages := renderPages(t, fmt.Sprintf(pageOverflowStyle, tt.height, tt.overflow)+tt.body)
			got := pageLines(t, pages)
			if fmt.Sprint(got) != fmt.Sprint(tt.want) {
				t.Errorf("got the lines %v on the pages, want %v", got, tt.want)
			}
		})
	}
}

func TestPageOverflowDefault(t *testing.T) {
	// The property is off by default, and it is inherited.
	pages := renderPages(t, `<p>a</p><div style="-ivy-page-overflow: descent"><p>b</p></div>`)
	html := only(t, pages[0])
	body := only(t, html)
	if got := str(body.Style.Get("page_overflow")); got != "none" {
		t.Errorf("got %q on the body, want none", got)
	}
	p := only(t, body.Children[1])
	if got := str(p.Style.Get("page_overflow")); got != "descent" {
		t.Errorf("got %q on the paragraph, want descent", got)
	}
}
