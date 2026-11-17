// This file is not a port. It tests -ivy-text-autospace.

package layout

import "testing"

func TestTextAutospace(t *testing.T) {
	width := func(autospace, text string) float64 {
		t.Helper()
		body := inlineBody(t, `<p style="font: 16px weasyprint; white-space: pre; -ivy-text-autospace: `+autospace+`">`+text+`</p>`)
		return only(t, only(t, body)).Width
	}
	for _, tt := range []struct {
		autospace, text string
		spaces          int
	}{
		{"ideograph-alpha", "a漢b", 2},
		{"ideograph-alpha", "1漢2", 0},
		{"ideograph-numeric", "1漢2", 2},
		{"ideograph-numeric ideograph-alpha", "a漢1", 2},
		{"ideograph-alpha", "漢字", 0},
		{"ideograph-alpha", "a　b", 0},
	} {
		got := width(tt.autospace, tt.text) - width("none", tt.text)
		assertApprox(t, tt.autospace+" "+tt.text, got, float64(tt.spaces)*16*autospaceRatio)
	}
}
