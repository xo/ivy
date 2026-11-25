// This file is not a port. It tests -ivy-stroke-adjust, which upstream does
// not have.

package layout

import (
	"context"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/xo/okage"

	"github.com/xo/ivy/internal/display"
)

func TestDrawStrokeAdjust(t *testing.T) {
	const table = `<table><tr><td>a</td><td class="b">b</td></tr></table>`
	tests := []struct {
		style           string
		adjusted, plain int
	}{
		// the 7 borders of the grid, without the adjustment
		{"td{border:0.5pt solid #ccc}", 0, 7},
		// the 7 borders on whole pixels
		{"td{border:0.5pt solid #ccc;-ivy-stroke-adjust:pixels}", 7, 0},
		// a wider border of a cell wins, and keeps its own value: the
		// 4 borders of the second cell are not adjusted
		{"td{border:0.5pt solid #ccc;-ivy-stroke-adjust:pixels}td.b{border:2pt solid black;-ivy-stroke-adjust:none}", 3, 4},
	}
	for _, test := range tests {
		body := `<html xmlns="http://www.w3.org/1999/xhtml"><head><style>table{border-collapse:collapse}` +
			test.style + `</style></head><body>` + table + `</body></html>`
		src := Source{
			FS:          fstest.MapFS{"t.xhtml": &fstest.MapFile{Data: []byte(body)}},
			Documents:   []string{"t.xhtml"},
			StyleSheets: []string{bookStyle},
		}
		var l display.List
		if err := NewBook(src, BookOptions{}).Draw(context.Background(), 0, &l, okage.IdentityTransform(), 1); err != nil {
			t.Fatal(err)
		}
		adjusted, plain := 0, 0
		for op := range strings.SplitSeq(l.String(), "\n") {
			switch {
			case strings.HasPrefix(op, "stroke adjusted "):
				adjusted++
			case strings.HasPrefix(op, "stroke "):
				plain++
			}
		}
		if adjusted != test.adjusted || plain != test.plain {
			t.Errorf("%s: expected %d adjusted and %d plain strokes, got: %d and %d\n%s",
				test.style, test.adjusted, test.plain, adjusted, plain, l.String())
		}
	}
}
