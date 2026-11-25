package review

import (
	"context"
	"log/slog"
	"strings"
	"testing"
)

// TestHighlightShiki makes sure that the review colors code as Shiki does
// with the theme github-light-default, for the tokens that the table
// shikiColors maps.
func TestHighlightShiki(t *testing.T) {
	h := highlighter(context.Background(), slog.Default())
	for _, tc := range []struct{ lang, code, want string }{
		{"go", "x := 1\n", `<span style="color: #cf222e">:=</span>`},
		{"go", "var n int\n", `<span style="color: #cf222e">int</span>`},
		{"go", "type T struct{}\n", `<span style="color: #953800">T</span>`},
		{"go", "var p *int = nil\n", `<span style="color: #0550ae">nil</span>`},
		{"json", `{"a": 1}`, `<span style="color: #116329">&#34;a&#34;</span>`},
		{"python", "# note\n", `<span style="color: #6e7781"># note</span>`},
	} {
		got, ok := h(tc.lang, tc.code)
		if !ok || !strings.Contains(got, tc.want) {
			t.Errorf("%s %q: expected %s, got: %v %s", tc.lang, tc.code, tc.want, ok, got)
		}
	}
}
