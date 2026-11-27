package rtf

import (
	"image"
	"image/color"
	"strings"
	"testing"

	"github.com/xo/ivy"
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

// TestWidowControl checks that a paragraph keeps 2 lines together at the
// break of a page only with \widctlpar, in the paragraph, in its style or in
// the style Normal, or with \widowctrl, as LibreOffice does.
func TestWidowControl(t *testing.T) {
	const normal = `{\stylesheet{\widctlpar Normal;}{\s1 Body;}{\s2\sbasedon0 Based;}}`
	for _, tt := range []struct {
		name, head, words string
		want              bool
	}{
		{"none", "", "", false},
		{"paragraph", "", `\widctlpar`, true},
		{"off", "", `\widctlpar\nowidctlpar`, false},
		{"document", `\widowctrl`, "", true},
		{"document off", `\widowctrl`, `\nowidctlpar`, false},
		{"normal", normal, "", true},
		{"normal s0", normal, `\s0`, true},
		{"other style", normal, `\s1`, false},
		{"based on normal", normal, `\s2`, true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			// 15 paragraphs of 3 lines of 13.8 points, then a paragraph
			// whose first line is the last line that fits on the page
			var b strings.Builder
			b.WriteString(`{\rtf1\ansi` + tt.head + `\paperw12240\paperh15840\margl1440\margr1440\margt1440\margb1440` + "\n")
			for range 16 {
				b.WriteString(`\pard` + tt.words + `\sl-276 A\line B\line C\par` + "\n")
			}
			b.WriteString("}")
			html, _ := convertHTML(t, []byte(b.String()))
			if got := strings.Contains(html, "orphans: 2; widows: 2"); got != tt.want {
				t.Errorf("expected the widow control %v, got:\n%s", tt.want, html)
			}
			d, err := Open(t.Context(), strings.NewReader(b.String()))
			if err != nil {
				t.Fatal(err)
			}
			p, err := d.Page(t.Context(), 0)
			if err != nil {
				t.Fatal(err)
			}
			img, err := ivy.RenderImage(t.Context(), p, 72, color.White)
			if err != nil {
				t.Fatal(err)
			}
			// the line 46 is from 693 to 706.8 points
			if last := inkBox(img, image.Rect(72, 696, 540, 718)); last.Empty() != tt.want {
				t.Errorf("expected the first line of the last paragraph on page 1 %v, got the ink %v", !tt.want, last)
			}
		})
	}
}
