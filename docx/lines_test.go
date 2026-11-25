package docx

import (
	"strings"
	"testing"

	"github.com/xo/ivy/internal/fixture"
)

// TestLineNumbers checks that a section gives the numbers of its lines to
// the layout from w:lnNumType, and that a paragraph can leave its lines
// out (D56).
func TestLineNumbers(t *testing.T) {
	for _, tt := range []struct {
		name, lnNumType, want string
	}{
		// w:start is the first number minus 1, and the distance is 0.25
		// inch by default. The numbers have the font of the document.
		{"all", `<w:lnNumType w:countBy="5" w:distance="720" w:start="9" w:restart="newSection"/>`, `-ivy-line-numbers: 36pt 5 10 section; font-size: 10pt`},
		{"default", `<w:lnNumType w:countBy="1"/>`, `-ivy-line-numbers: 18pt 1 1 page`},
		{"continuous", `<w:lnNumType w:countBy="2" w:restart="continuous"/>`, `-ivy-line-numbers: 18pt 2 1 continuous`},
		// Without w:countBy, the lines have no numbers, as in LibreOffice.
		{"no step", `<w:lnNumType w:distance="720"/>`, ""},
	} {
		t.Run(tt.name, func(t *testing.T) {
			body := `<w:p><w:r><w:t>Numbered</w:t></w:r></w:p><w:p><w:pPr><w:suppressLineNumbers/></w:pPr><w:r><w:t>Left out</w:t></w:r></w:p>` +
				`<w:sectPr>` + tt.lnNumType + `</w:sectPr>`
			html, _ := convertHTML(t, fixture.DOCX(body))
			if tt.want == "" {
				if strings.Contains(html, "-ivy-line-numbers: 18pt") || strings.Contains(html, "-ivy-line-numbers: 36pt") {
					t.Errorf("expected no numbers, got:\n%s", html)
				}
				return
			}
			if !strings.Contains(html, tt.want) {
				t.Errorf("expected %q, got:\n%s", tt.want, html)
			}
			if !strings.Contains(html, "-ivy-line-numbers: none") {
				t.Errorf("expected a paragraph without numbers, got:\n%s", html)
			}
		})
	}
}

// TestLineMultiple checks that a paragraph with a multiple line spacing
// puts the extra space below its text, that a single line puts its line
// gap above its text, and that the pages leave out the space before a
// paragraph at their top (D56).
func TestLineMultiple(t *testing.T) {
	body := `<w:p><w:pPr><w:spacing w:line="360" w:lineRule="auto"/></w:pPr><w:r><w:t>Spaced</w:t></w:r></w:p>` +
		`<w:p><w:pPr><w:spacing w:line="360" w:lineRule="exact"/></w:pPr><w:r><w:t>Exact</w:t></w:r></w:p>` +
		`<w:p><w:r><w:t>Single</w:t></w:r></w:p>` +
		`<w:p><w:pPr><w:spacing w:line="120" w:lineRule="atLeast"/></w:pPr><w:r><w:t>Small</w:t></w:r></w:p>` +
		`<w:p><w:pPr><w:spacing w:line="720" w:lineRule="atLeast"/></w:pPr><w:r><w:t>Tall</w:t></w:r></w:p>`
	html, _ := convertHTML(t, fixture.DOCX(body))
	if n := strings.Count(html, "-ivy-line-multiple: 1.5"); n != 1 {
		t.Errorf("expected the multiple 1.5 once, got %d:\n%s", n, html)
	}
	if n := strings.Count(html, "-ivy-line-multiple: 1;"); n != 2 {
		t.Errorf("expected the multiple 1 twice, for the single line and the small line, got %d:\n%s", n, html)
	}
	if !strings.Contains(styleSheet, "-ivy-page-top-space: discard") {
		t.Error("expected the style sheet to leave out the space before a paragraph at the top of a page")
	}
}

// TestLineMultipleGrid checks that a single line on a grid of lines keeps
// its text in the middle of its lines of the grid, as in Word
// (libreoffice/docx/mixed-orientation.docx).
func TestLineMultipleGrid(t *testing.T) {
	body := `<w:p><w:r><w:t>Single</w:t></w:r></w:p>` +
		`<w:p><w:pPr><w:spacing w:line="480" w:lineRule="auto"/></w:pPr><w:r><w:t>Double</w:t></w:r></w:p>` +
		`<w:sectPr><w:docGrid w:type="lines" w:linePitch="326"/></w:sectPr>`
	html, _ := convertHTML(t, fixture.DOCX(body))
	if strings.Contains(html, "-ivy-line-multiple: 1;") {
		t.Errorf("expected no multiple for a single line on the grid:\n%s", html)
	}
	if !strings.Contains(html, "-ivy-line-multiple: 2") {
		t.Errorf("expected the multiple 2:\n%s", html)
	}
}
