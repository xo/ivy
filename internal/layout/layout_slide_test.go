// This file is not a port. It tests the property of D71:
// -ivy-line-baseline.

package layout

import (
	"fmt"
	"testing"
)

func TestLineBaseline(t *testing.T) {
	// The Windows ascent of weasyprint.otf is 819 and its Windows descent
	// 205, so the baseline of a line is at 0.7998 of its height with
	// 'win'. The text is 20px high, with its baseline 16px below its top.
	for _, tt := range []struct {
		name       string
		lineHeight float64
		baseline   string
		textTop    float64
	}{
		// CSS puts 10px of the 20px of leading above the text.
		{"normal", 40, "normal", 10},
		// The baseline at 32px puts the top of the text at 16px.
		{"win", 40, "win", 16},
		// A line as high as its text keeps its text at the top.
		{"text", 20, "win", 0},
		// A line lower than its text puts the top of the text above it.
		{"low", 10, "win", -8},
	} {
		t.Run(tt.name, func(t *testing.T) {
			pages := renderPages(t, wordStyle+`<p style="line-height: `+fmt.Sprint(tt.lineHeight)+`px; -ivy-line-baseline: `+tt.baseline+`">a<span>b</span></p><p>c</p>`)
			_, body := pageBody(t, pages[0])
			p1, p2 := body.Children[0], body.Children[1]
			assertApprox(t, "the height of the paragraph", p1.Height, tt.lineHeight)
			assertApprox(t, "the top of the next paragraph", p2.PositionY, tt.lineHeight)
			line := only(t, p1)
			text, span := line.Children[0], line.Children[1]
			assertNear(t, "the top of the text", text.ContentBoxY(), tt.textTop)
			assertNear(t, "the top of the span", span.ContentBoxY(), tt.textTop)
			assertNear(t, "the baseline of the line", line.PositionY+line.Baseline, tt.textTop+16)
		})
	}
}
