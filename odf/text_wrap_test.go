package odf

import "testing"

// TestWrapFlow checks that a frame with wrap gives -ivy-wrap-flow from the
// side of its style:wrap, with the larger of its left and right margins and
// the keyword margin (D63).
func TestWrapFlow(t *testing.T) {
	for _, tt := range []struct {
		g    odtProps
		want string
	}{
		{odtProps{"wrap": "parallel", "margin-left": "0.1in", "margin-right": "0.2in"}, "both 14.4pt margin"},
		{odtProps{"wrap": "dynamic"}, "maximum 0pt margin"},
		{odtProps{"wrap": "left", "margin-left": "9pt"}, "start 9pt margin"},
		{odtProps{"wrap": "right", "margin-right": "-9pt"}, "end 0pt margin"},
		{odtProps{"wrap": "biggest", "margin-right": "1cm"}, "maximum 28.35pt margin"},
	} {
		if got := odtWrapFlow(tt.g); got != tt.want {
			t.Errorf("odtWrapFlow(%v) = %q, want %q", tt.g, got, tt.want)
		}
	}
}
