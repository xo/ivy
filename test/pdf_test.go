package test

import (
	"bytes"
	"image/color"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/xo/ivy"
	"github.com/xo/ivy/pdf"
)

// popplerDiffers lists the test files where pdftoppm draws the page in a
// different way from hayro. On 2026-11-16, ivy matched the pages that hayro
// renders within a mean difference of 1.5 for each of these files. The test
// logs their difference, and does not compare it.
var popplerDiffers = map[string]string{
	"image_named_colorspace":              "poppler draws nothing",
	"stream_jpx_raw_subsampled_rgb":       "poppler draws nothing",
	"stream_jpx_raw_device_n":             "poppler draws other colors",
	"pattern_alpha_type3":                 "poppler draws it in another way",
	"pattern_tiling_nested":               "poppler draws another inner pattern",
	"image_smask_downscale":               "poppler draws it in another way",
	"mask_luminosity_no_cs":               "poppler draws it in another way",
	"image_1_bit_per_component":           "poppler draws it in another way",
	"pattern_tiling_uncolored_named_base": "poppler draws it in another way",
	"color_space_separation_1":            "poppler draws it in another way",
	"function_type3_intervals":            "poppler draws it in another way",
}

// TestPDF compares the first page of each test file of the port of
// hayro-interpret with the page that pdftoppm of poppler renders, which is
// the oracle of iv for PDF. The difference comes from the anti-aliasing and
// from the standard fonts, which are not the same files. It is at most about
// 7 on a page that both programs draw the same.
func TestPDF(t *testing.T) {
	names, err := filepath.Glob(filepath.Join("..", "pdf", "internal", "interpret", "testdata", "*.pdf"))
	if err != nil {
		t.Fatal(err)
	}
	if len(names) == 0 {
		t.Fatal("expected test files")
	}
	for _, path := range names {
		name := strings.TrimSuffix(filepath.Base(path), ".pdf")
		t.Run(name, func(t *testing.T) {
			buf, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			d, err := pdf.Open(t.Context(), bytes.NewReader(buf))
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
			want, err := run(t.Context(), "pdftoppm", "-png", "-r", "72", "-f", "1", "-l", "1", "-singlefile", path)
			if err != nil {
				t.Fatal(err)
			}
			diff := meanDiff(got, want)
			t.Logf("mean difference: %.2f", diff)
			if why, ok := popplerDiffers[name]; ok {
				t.Logf("not compared: %s", why)
				return
			}
			if diff < 0 || diff > 8 {
				t.Errorf("expected the page of pdftoppm within a mean difference of 8, got: %.2f", diff)
			}
		})
	}
}
