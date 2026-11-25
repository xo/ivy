package test

import (
	"bytes"
	"context"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/xo/ivy"
	"github.com/xo/ivy/xps"
)

// TestXPS compares each page of each test file of xps with the page that
// xpstopng of libgxps renders, which is the oracle of iv for XPS. The pages
// look the same. The difference comes from the anti-aliasing of dense text,
// where the glyphs of ivy are a little heavier, and it is the largest on a
// page full of text, at about 6.
func TestXPS(t *testing.T) {
	for _, name := range []string{"example.xps", "xps-document-example.xps"} {
		path := filepath.Join("..", "xps", "testdata", name)
		buf, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		d, err := xps.Open(t.Context(), bytes.NewReader(buf))
		if err != nil {
			t.Fatal(err)
		}
		n, _ := d.PageCount(t.Context())
		for i := range n {
			t.Run(fmt.Sprintf("%s/%d", name, i+1), func(t *testing.T) {
				page, err := d.Page(t.Context(), i)
				if err != nil {
					t.Fatal(err)
				}
				got, err := ivy.RenderImage(t.Context(), page, 72, color.White)
				if err != nil {
					t.Fatal(err)
				}
				want, err := xpstopng(t.Context(), t.TempDir(), path, i+1)
				if err != nil {
					t.Fatal(err)
				}
				diff := meanDiff(got, want)
				t.Logf("mean difference: %.2f", diff)
				if diff < 0 || diff > 8 {
					t.Errorf("expected the page of xpstopng within a mean difference of 8, got: %.2f", diff)
				}
			})
		}
	}
}

// xpstopng renders page n of the XPS file at 72 dots per inch.
func xpstopng(ctx context.Context, dir, path string, n int) (image.Image, error) {
	bin, err := exec.LookPath("xpstopng")
	if err != nil {
		return nil, fmt.Errorf("oracle xpstopng is not installed: %w", err)
	}
	prefix := filepath.Join(dir, "page")
	out, err := exec.CommandContext(ctx, bin, "-f", fmt.Sprint(n), "-l", fmt.Sprint(n), "-r", "72", path, prefix).CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("running xpstopng: %w: %s", err, out)
	}
	names, err := filepath.Glob(prefix + "-*.png")
	if err != nil || len(names) != 1 {
		return nil, fmt.Errorf("xpstopng wrote %d images", len(names))
	}
	f, err := os.Open(names[0])
	if err != nil {
		return nil, fmt.Errorf("opening the png of xpstopng: %w", err)
	}
	defer f.Close()
	img, err := png.Decode(f)
	if err != nil {
		return nil, fmt.Errorf("decoding the png of xpstopng: %w", err)
	}
	return img, nil
}
