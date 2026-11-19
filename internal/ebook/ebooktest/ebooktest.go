// Package ebooktest holds the helpers that the tests of the formats of
// ebooks share: the ink of a rendered page, the size of the pages on the
// paper of a program, and the rendering of the first page of a document in
// a fuzz test.
package ebooktest

import (
	"errors"
	"image"
	"image/color"
	"math"
	"strings"
	"testing"

	"github.com/xo/okage"

	"github.com/xo/ivy"
)

// Ink counts the dark pixels of img inside r: the ink of text.
func Ink(img image.Image, r image.Rectangle) int {
	n := 0
	r = r.Intersect(img.Bounds())
	for y := r.Min.Y; y < r.Max.Y; y++ {
		for x := r.Min.X; x < r.Max.X; x++ {
			if c := color.GrayModel.Convert(img.At(x, y)).(color.Gray); c.Y < 128 {
				n++
			}
		}
	}
	return n
}

// RenderPage renders page n of d at 96 dots per inch over white. It fails
// the test on an error.
func RenderPage(t *testing.T, d ivy.Document, n int) image.Image {
	t.Helper()
	pager, ok := d.(ivy.Pager)
	if !ok {
		t.Fatal("expected an ivy.Pager")
	}
	p, err := pager.Page(t.Context(), n)
	if err != nil {
		t.Fatal(err)
	}
	img, err := ivy.RenderImage(t.Context(), p, 96, color.White)
	if err != nil {
		t.Fatal(err)
	}
	return img
}

// Landscape is the paper that CheckPaper expects: A4 in landscape.
var Landscape = ivy.A4.Landscape()

// CheckPaper fails the test unless each page of d is A4 in landscape,
// 841.89 by 595.28 points. A test opens d with Landscape as the paper of
// its ivy.Config (D32). CheckPaper also renders the pages, and fails the
// test unless one of them holds ink.
func CheckPaper(t *testing.T, d ivy.Document) {
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
		p, err := pager.Page(t.Context(), i)
		if err != nil {
			t.Fatal(err)
		}
		if w, h := p.Size(); w != Landscape.Width || h != Landscape.Height {
			t.Errorf("expected page %d on A4 in landscape, 841.89 by 595.28 points, got: %g by %g", i, w, h)
		}
	}
	for i := range n {
		if img := RenderPage(t, d, i); Ink(img, img.Bounds()) > 0 {
			return
		}
	}
	t.Error("expected ink on a page, got none")
}

// RenderFirstPage renders the first page of d, 32 pixels wide, for a fuzz
// test. It fails the test on a panic, and on an error that is not one of
// the errors of ivy for a hostile file.
func RenderFirstPage(t *testing.T, d ivy.Document) {
	t.Helper()
	pager, ok := d.(ivy.Pager)
	if !ok {
		return
	}
	p, err := pager.Page(t.Context(), 0)
	if err != nil {
		check(t, err)
		return
	}
	w, h := p.Size()
	if !(w > 0) || !(h > 0) || math.IsInf(float64(w), 0) || math.IsInf(float64(h), 0) {
		return
	}
	scale := 32 / w
	height := min(math.Ceil(float64(h*scale)), 256)
	dst, ok := okage.NewPixmap(32, uint32(max(height, 1)))
	if !ok {
		return
	}
	check(t, p.Render(t.Context(), dst, okage.TransformFromScale(scale, scale)))
}

// check fails the test on a panic, or on an error that is not an error of
// ivy for a hostile file.
func check(t *testing.T, err error) {
	t.Helper()
	if err == nil {
		return
	}
	if strings.Contains(err.Error(), "panic") {
		t.Fatal(err)
	}
	if _, ok := errors.AsType[*ivy.PartialError](err); ok {
		return
	}
	for _, want := range []error{ivy.ErrFormat, ivy.ErrLimit, ivy.ErrDRM, ivy.ErrPassword, ivy.ErrPageRange, ivy.ErrUnsupported} {
		if errors.Is(err, want) {
			return
		}
	}
	t.Fatalf("unexpected error: %v", err)
}
