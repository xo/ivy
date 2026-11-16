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
	"github.com/xo/ivy/emf"
	"github.com/xo/ivy/internal/fixture"
)

// TestEMF compares the pictures of small metafiles that the test builds with
// the PNG images that Inkscape exports from them. Inkscape reads EMF and WMF
// with libUEMF. The pictures hold fills, an outline, a clip, a bitmap and a
// gradient, and no text, because Inkscape draws the text with other fonts.
// On 2026-11-17 the mean difference is 1.3 at most, from the anti-aliasing
// of the edges and the size, which Inkscape rounds up by a pixel.
func TestEMF(t *testing.T) {
	red := color.NRGBA{255, 0, 0, 255}
	green := color.NRGBA{0, 160, 0, 255}
	blue := color.NRGBA{0, 0, 255, 255}
	black := color.NRGBA{0, 0, 0, 255}
	// Inkscape smooths a bitmap that it scales, and GDI does not by default,
	// so the bitmap draws at its size at 100 dots per inch
	img := image.NewNRGBA(image.Rect(0, 0, 40, 40))
	for y := range 40 {
		for x := range 40 {
			img.SetNRGBA(x, y, []color.NRGBA{red, green, blue, black}[x/20+2*(y/20)])
		}
	}

	shapes := fixture.NewEMF(200, 200)
	shapes.Select(shapes.Brush(red, -1))
	shapes.Select(fixture.Stock(8))
	shapes.Rectangle(100, 0, 200, 100)
	shapes.Select(shapes.Brush(blue, -1))
	shapes.Select(shapes.Pen(0, 6, black))
	shapes.Ellipse(20, 100, 120, 190)
	shapes.Image(20, 20, 40, 40, img)
	shapes.Gradient(130, 110, 190, 190, red, green, true)

	clip := fixture.NewEMF(200, 200)
	clip.Select(clip.Brush(green, -1))
	clip.Select(fixture.Stock(8))
	clip.IntersectClip(20, 20, 180, 180)
	clip.ExcludeClip(60, 60, 140, 140)
	clip.Rectangle(0, 0, 200, 200)

	wmf := fixture.NewWMF(200, 200, true)
	wmf.Select(wmf.Brush(red))
	wmf.Select(wmf.NullPen())
	wmf.Rectangle(100, 0, 200, 100)

	// IVY_EMF_DIR keeps the images, for a person who looks at them
	dir := os.Getenv("IVY_EMF_DIR")
	if dir == "" {
		dir = t.TempDir()
	}
	for _, c := range []struct {
		name string
		data []byte
	}{
		{"shapes.emf", shapes.Bytes()},
		{"clip.emf", clip.Bytes()},
		{"square.wmf", wmf.Bytes()},
	} {
		t.Run(c.name, func(t *testing.T) {
			path := filepath.Join(dir, c.name)
			if err := os.WriteFile(path, c.data, 0o600); err != nil {
				t.Fatal(err)
			}
			d, err := emf.Open(t.Context(), bytes.NewReader(c.data))
			if err != nil {
				t.Fatal(err)
			}
			page, err := d.Page(t.Context(), 0)
			if err != nil {
				t.Fatal(err)
			}
			got, err := ivy.RenderImage(t.Context(), page, 100, color.White)
			if err != nil {
				t.Fatal(err)
			}
			want, err := inkscape(t.Context(), dir, path)
			if err != nil {
				t.Fatal(err)
			}
			// Inkscape rounds the size of the picture up by a pixel
			got, want = common(got, want)
			diff := meanDiff(got, want)
			t.Logf("mean difference: %.2f", diff)
			if diff < 0 || diff > 4 {
				writePNG(t, filepath.Join(dir, c.name+".ivy.png"), got)
				writePNG(t, filepath.Join(dir, c.name+".inkscape.png"), want)
				t.Errorf("expected the picture of Inkscape within a mean difference of 4, got: %.2f", diff)
			}
		})
	}
}

// inkscape exports the metafile path as a PNG image at 100 dots per inch.
func inkscape(ctx context.Context, dir, path string) (image.Image, error) {
	bin, err := exec.LookPath("inkscape")
	if err != nil {
		return nil, fmt.Errorf("oracle inkscape is not installed: %w", err)
	}
	out := filepath.Join(dir, filepath.Base(path)+".png")
	cmd := exec.CommandContext(ctx, bin, "--export-type=png", "--export-dpi=100", "--export-background=white",
		"--export-background-opacity=1", "-o", out, path)
	if msg, err := cmd.CombinedOutput(); err != nil {
		return nil, fmt.Errorf("running inkscape: %w: %s", err, msg)
	}
	f, err := os.Open(out)
	if err != nil {
		return nil, fmt.Errorf("opening the png of inkscape: %w", err)
	}
	defer func() { _ = f.Close() }()
	img, err := png.Decode(f)
	if err != nil {
		return nil, fmt.Errorf("decoding the png of inkscape: %w", err)
	}
	return img, nil
}

// common returns a and b cut to the size that they share.
func common(a, b image.Image) (image.Image, image.Image) {
	size := image.Pt(min(a.Bounds().Dx(), b.Bounds().Dx()), min(a.Bounds().Dy(), b.Bounds().Dy()))
	cut := func(img image.Image) image.Image {
		sub, ok := img.(interface {
			SubImage(image.Rectangle) image.Image
		})
		if !ok {
			return img
		}
		return sub.SubImage(image.Rectangle{Min: img.Bounds().Min, Max: img.Bounds().Min.Add(size)})
	}
	return cut(a), cut(b)
}
