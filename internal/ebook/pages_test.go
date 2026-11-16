package ebook

import (
	"context"
	"image"
	"image/color"
	"os"
	"path/filepath"
	"testing"
	"testing/fstest"

	"github.com/xo/ivy"
	"github.com/xo/ivy/internal/layout"
)

// renderPage renders the first page of a book of one line of text in
// Calibri, with the configuration cfg.
func renderPage(t *testing.T, cfg ivy.Config) image.Image {
	t.Helper()
	src := func(context.Context) (layout.Source, error) {
		return layout.Source{
			FS:          fstest.MapFS{"a.html": &fstest.MapFile{Data: []byte(`<p>Office documents name Calibri</p>`)}},
			Documents:   []string{"a.html"},
			StyleSheets: []string{`@page { size: 400px 40px; margin: 5px } body { margin: 0; font: 20px Calibri }`},
		}, nil
	}
	var pages Pages
	p, err := pages.Page(context.Background(), src, cfg, 0)
	if err != nil {
		t.Fatal(err)
	}
	img, err := ivy.RenderImage(context.Background(), p, 96, color.White)
	if err != nil {
		t.Fatal(err)
	}
	return img
}

// TestPagesFonts checks that Pages gives Config.Fonts to the book, and that
// the zero Config draws the text with the Go fonts.
func TestPagesFonts(t *testing.T) {
	dir := filepath.Join("..", "..", "fonts")
	if _, err := os.Stat(filepath.Join(dir, "carlito")); err != nil {
		t.Skip("the fonts of github.com/xo/ivy/fonts are missing:", err)
	}
	zero := renderPage(t, ivy.Config{})
	fonts := renderPage(t, ivy.Config{Fonts: os.DirFS(dir)})
	if zero.Bounds() != fonts.Bounds() {
		t.Fatalf("bounds %v and %v", zero.Bounds(), fonts.Bounds())
	}
	inked, differ := false, false
	r := zero.Bounds()
	for y := r.Min.Y; y < r.Max.Y; y++ {
		for x := r.Min.X; x < r.Max.X; x++ {
			if zero.At(x, y) != fonts.At(x, y) {
				differ = true
			}
			if c, _, _, _ := zero.At(x, y).RGBA(); c < 0x8000 {
				inked = true
			}
		}
	}
	if !inked {
		t.Error("the zero Config draws no text")
	}
	if !differ {
		t.Error("Config.Fonts does not change the font of Calibri")
	}
}
