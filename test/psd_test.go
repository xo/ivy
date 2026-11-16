package test

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/xo/ivy"
	"github.com/xo/ivy/psd"
)

// TestPSD compares the page of each test file of psd with the image that
// ImageMagick decodes, which was the oracle of iv for PSD.
func TestPSD(t *testing.T) {
	// psd_nocompat.psd holds no merged image, and ImageMagick shows its
	// blank merged image, so it has no oracle
	// ImageMagick reads the merged image of rgba8.psd, which has an alpha
	// channel, with other colors than the gradient that made the file: at
	// (10, 20) it reads (109, 0, 60) for (146, 0, 109). Its one layer holds the
	// colors of the gradient, so the oracle reads the layer
	frames := map[string]string{"rgba8.psd": "[1]"}
	for _, name := range []string{"rgba8.psd", "cmyk8.psd", "deep16.psd", "rgb8bit.psd", "psd_compat.psd"} {
		t.Run(name, func(t *testing.T) {
			path := filepath.Join("..", "psd", "testdata", name)
			frame, ok := frames[name]
			if !ok {
				frame = "[0]"
			}
			buf, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			d, err := psd.Open(t.Context(), bytes.NewReader(buf))
			if err != nil {
				t.Fatal(err)
			}
			page, err := d.Page(t.Context(), 0)
			if err != nil {
				t.Fatal(err)
			}
			got, err := ivy.RenderImage(t.Context(), page, 72, nil)
			if err != nil {
				t.Fatal(err)
			}
			want, err := run(t.Context(), "magick", path+frame, "-colorspace", "sRGB", "-depth", "8", "png:-")
			if err != nil {
				t.Fatal(err)
			}
			diff := meanDiff(got, want)
			t.Logf("mean difference: %.2f", diff)
			if diff < 0 || diff > 8 {
				t.Errorf("expected the image of ImageMagick within a mean difference of 8, got: %.2f", diff)
			}
		})
	}
}
