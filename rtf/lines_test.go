package rtf

import (
	"strings"
	"testing"
)

// TestLineNumbers checks that a section gives the numbers of its lines to
// the layout, that a paragraph can leave its lines out, and that a
// paragraph with a multiple line spacing puts the extra space below its
// text (D56).
func TestLineNumbers(t *testing.T) {
	for _, tt := range []struct {
		name, words, want string
	}{
		{"all", `\linemod5\linex720\linestarts10\linerestart`, `-ivy-line-numbers: 36pt 5 10 section`},
		// The distance 0 is 0.25 inch, and the numbers restart on each
		// page by default.
		{"default", `\linemod1\linex0`, `-ivy-line-numbers: 18pt 1 1 page`},
		{"continuous", `\linemod2\linecont`, `-ivy-line-numbers: 18pt 2 1 continuous`},
		{"none", `\linex0`, ""},
	} {
		t.Run(tt.name, func(t *testing.T) {
			buf := []byte(`{\rtf1\ansi` + tt.words + `
\pard\sl360\slmult1 Spaced\par
\pard\noline Left out\par
}`)
			html, _ := convertHTML(t, buf)
			if tt.want == "" {
				if strings.Contains(html, "-ivy-line-numbers: 18pt") {
					t.Errorf("expected no numbers, got:\n%s", html)
				}
			} else if !strings.Contains(html, tt.want) {
				t.Errorf("expected %q, got:\n%s", tt.want, html)
			}
			if !strings.Contains(html, "-ivy-line-numbers: none") {
				t.Errorf("expected a paragraph without numbers, got:\n%s", html)
			}
			if !strings.Contains(html, "-ivy-line-multiple: 1.5") {
				t.Errorf("expected the multiple 1.5, got:\n%s", html)
			}
		})
	}
	if !strings.Contains(styleSheet, "-ivy-page-top-space: discard") {
		t.Error("expected the style sheet to leave out the space before a paragraph at the top of a page")
	}
}
