package test

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"os"
	"path/filepath"
	"testing"

	"github.com/xo/ivy"
	"github.com/xo/ivy/ps"
)

// TestPS compares the first page of test files of ps with the page that
// Ghostscript renders, which is the reference of PostScript (D13, D29).
// Ghostscript runs as an outside program, and no code of it is read.
//
// On 2026-11-17 the mean differences are 0.59 for shapes.ps, 0.87 for
// text.ps and 0.00 for plain.eps. Ghostscript converts the colors of
// DeviceCMYK with a profile of its own, and it draws the standard fonts
// with the fonts of URW, where ivy draws them with the substitutes of the
// PDF reader.
func TestPS(t *testing.T) {
	tests := []struct {
		name string
		max  float64
	}{
		{"shapes.ps", 2},
		{"text.ps", 2},
		{"plain.eps", 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := filepath.Join("..", "ps", "testdata", tt.name)
			buf, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			d, err := ps.Open(t.Context(), bytes.NewReader(buf))
			if err != nil {
				t.Fatal(err)
			}
			page, err := d.Page(t.Context(), 0)
			if err != nil {
				t.Fatal(err)
			}
			got, err := ivy.RenderImage(t.Context(), page, 72, color.White)
			if err != nil {
				t.Fatal(err)
			}
			args := []string{"-q", "-dSAFER", "-dBATCH", "-dNOPAUSE", "-sDEVICE=png16m", "-r72",
				"-dTextAlphaBits=4", "-dGraphicsAlphaBits=4", "-sOutputFile=-"}
			if filepath.Ext(tt.name) == ".eps" {
				args = append(args, "-dEPSCrop")
			}
			want, err := run(t.Context(), "gs", append(args, path)...)
			if err != nil {
				t.Fatal(err)
			}
			diff := meanDiff(got, want)
			t.Logf("mean difference: %.2f", diff)
			if diff < 0 || diff > tt.max {
				stem := filepath.Join(t.TempDir(), tt.name)
				writePNG(t, stem+"-ivy.png", got)
				writePNG(t, stem+"-gs.png", want)
				t.Errorf("expected the page of Ghostscript within a mean difference of %g, got: %.2f (%s)", tt.max, diff,
					fmt.Sprint(got.Bounds().Size(), want.Bounds().Size()))
			}
		})
	}
}

// TestEPSPreviewPacked compares the TIFF preview of 4 bits of
// tiff4-preview.eps, which ImageMagick wrote with a palette of 16 colors,
// with the page that Ghostscript renders (D13). The gradient of the page is
// 16 pixels wide, so the palette holds each of its colors. On 2026-11-17 the
// mean difference is 0.00.
func TestEPSPreviewPacked(t *testing.T) {
	path := filepath.Join("..", "ps", "testdata", "tiff4-preview.eps")
	buf, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	d, err := ps.Open(t.Context(), bytes.NewReader(buf))
	if err != nil {
		t.Fatal(err)
	}
	got, err := ivy.BestPreview(t.Context(), d, image.Point{})
	if err != nil {
		t.Fatal(err)
	}
	want, err := run(t.Context(), "gs", "-q", "-dSAFER", "-dBATCH", "-dNOPAUSE", "-sDEVICE=png16m", "-r72",
		"-dEPSCrop", "-sOutputFile=-", path)
	if err != nil {
		t.Fatal(err)
	}
	diff := meanDiff(got, want)
	t.Logf("mean difference: %.2f", diff)
	if diff < 0 || diff > 3 {
		t.Errorf("expected the page of Ghostscript within a mean difference of 3, got: %.2f (ivy %v, gs %v)",
			diff, got.Bounds().Size(), want.Bounds().Size())
	}
}
