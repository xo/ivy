package fixture

import (
	"math"
	"testing"

	"github.com/xo/ivy"
)

// CheckPaper fails the test unless d has a page, and each page of d has
// the size of p, to 0.01 points (D32).
func CheckPaper(t *testing.T, d ivy.Document, p ivy.Paper) {
	t.Helper()
	pager, ok := d.(ivy.Pager)
	if !ok {
		t.Fatal("expected an ivy.Pager")
	}
	n, err := pager.PageCount(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if n < 1 {
		t.Fatal("expected a page, got none")
	}
	for i := range n {
		page, err := pager.Page(t.Context(), i)
		if err != nil {
			t.Fatal(err)
		}
		if w, h := page.Size(); math.Abs(float64(w-p.Width)) > 0.01 || math.Abs(float64(h-p.Height)) > 0.01 {
			t.Errorf("expected page %d of %g by %g points, got: %g by %g", i, p.Width, p.Height, w, h)
		}
	}
}
