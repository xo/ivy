package review

import (
	"context"
	"image"
	"image/color"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

// TestChrome draws page 2 of a PDF file of four pages with Chrome and with
// MuPDF, at two resolutions. The picture of Chrome must have the size of the
// page of MuPDF, and look as it does: the review cut the viewer away.
func TestChrome(t *testing.T) {
	for _, name := range []string{chromeProgram, "mutool", "pdfinfo", "fc-list"} {
		if _, err := exec.LookPath(name); err != nil {
			t.Fatalf("oracle %s is not installed: %v", name, err)
		}
	}
	repo, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	in := filepath.Join(repo, "internal", "hayro", "interpret", "testdata", "color_space_separation_1.pdf")
	for _, dpi := range []int{96, 150} {
		env, _, err := toolEnv(t.Context(), RenderOptions{Repo: repo, Cache: t.TempDir(), DPI: dpi}, t.TempDir())
		if err != nil {
			t.Fatal(err)
		}
		j := &refJob{refEnv: env, kind: "pdf", tmp: t.TempDir()}
		ctx, cancel := context.WithTimeout(t.Context(), time.Minute)
		dir := t.TempDir()
		chrome, mupdf := filepath.Join(dir, "chrome.png"), filepath.Join(dir, "mupdf.png")
		if err := chromeDraw(ctx, j, in, 2, chrome); err != nil {
			t.Fatalf("%d dpi: %v", dpi, err)
		}
		if err := mutoolDraw(ctx, j, nil, in, 2, mupdf); err != nil {
			t.Fatalf("%d dpi: %v", dpi, err)
		}
		cancel()
		env.closeChrome()
		a, err := readImage(chrome)
		if err != nil {
			t.Fatal(err)
		}
		b, err := readImage(mupdf)
		if err != nil {
			t.Fatal(err)
		}
		if a.Bounds().Size() != b.Bounds().Size() {
			t.Errorf("%d dpi: expected the size of MuPDF %v, got: %v", dpi, b.Bounds().Size(), a.Bounds().Size())
		}
		m, _ := Compare(a, b, false)
		if m.Blurred > 3 {
			t.Errorf("%d dpi: expected Chrome to look as MuPDF, got a blurred difference of %.2f", dpi, m.Blurred)
		}
		// the corners and the edges are the page, not the shadow or the
		// background of the viewer
		size := a.Bounds().Size()
		for _, p := range []image.Point{{0, 0}, {size.X - 1, 0}, {0, size.Y - 1}, {size.X - 1, size.Y - 1}, {size.X / 2, size.Y - 1}} {
			if d := colorDistance(a.At(p.X, p.Y), b.At(p.X, p.Y)); d > 24 {
				t.Errorf("%d dpi: expected the color of MuPDF at %v, got a difference of %d", dpi, p, d)
			}
		}
	}
}

// colorDistance returns the largest difference of the channels of a and b,
// from 0 to 255.
func colorDistance(a, b color.Color) int {
	ar, ag, ab, _ := a.RGBA()
	br, bg, bb, _ := b.RGBA()
	d := 0
	for _, v := range [][2]uint32{{ar, br}, {ag, bg}, {ab, bb}} {
		d = max(d, abs(int(v[0]>>8)-int(v[1]>>8)))
	}
	return d
}
