package psd

import (
	"bytes"
	"encoding/binary"
	"errors"
	"image"
	"image/color"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/xo/ivy"
)

func read(t testing.TB, name string) []byte {
	t.Helper()
	buf, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatal(err)
	}
	return buf
}

func TestOpen(t *testing.T) {
	tests := []struct {
		name string
		size image.Point
	}{
		{"rgba8.psd", image.Pt(64, 48)},
		{"cmyk8.psd", image.Pt(64, 48)},
		{"deep16.psd", image.Pt(96, 64)},
		{"rgb8bit.psd", image.Pt(64, 64)},
		{"psd_nocompat.psd", image.Pt(64, 64)},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			d, err := Open(t.Context(), bytes.NewReader(read(t, test.name)))
			if err != nil {
				t.Fatal(err)
			}
			page, err := d.Page(t.Context(), 0)
			if err != nil {
				t.Fatal(err)
			}
			img, err := ivy.RenderImage(t.Context(), page, 72, nil)
			if err != nil {
				t.Fatal(err)
			}
			if img.Bounds().Size() != test.size {
				t.Errorf("expected %v, got: %v", test.size, img.Bounds().Size())
			}
			if blank(img) {
				t.Error("expected an image, got a blank page")
			}
		})
	}
}

// blank reports whether every pixel of img has one color.
func blank(img image.Image) bool {
	b := img.Bounds()
	first := color.RGBAModel.Convert(img.At(b.Min.X, b.Min.Y))
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			if color.RGBAModel.Convert(img.At(x, y)) != first {
				return false
			}
		}
	}
	return true
}

// TestDeep16OnManyCores holds the fix of the copy. Upstream splits the lines
// of an image of 16 bits at the wrong offsets when it decodes on more than
// one goroutine, and fails on deep16.psd.
func TestDeep16OnManyCores(t *testing.T) {
	defer runtime.GOMAXPROCS(runtime.GOMAXPROCS(4))
	img, err := Decode(bytes.NewReader(read(t, "deep16.psd")))
	if err != nil {
		t.Fatal(err)
	}
	// the gradient runs from yellow at the top to navy at the bottom
	top := color.NRGBAModel.Convert(img.At(48, 0)).(color.NRGBA)
	bottom := color.NRGBAModel.Convert(img.At(48, 63)).(color.NRGBA)
	if top.R < 240 || top.G < 240 || top.B > 15 {
		t.Errorf("expected yellow at the top, got: %v", top)
	}
	if bottom.R > 15 || bottom.G > 15 || bottom.B < 112 || bottom.B > 144 {
		t.Errorf("expected navy at the bottom, got: %v", bottom)
	}
}

func TestComposite(t *testing.T) {
	// psd_nocompat.psd holds no merged image, so its page is a composite of
	// its layers. psd_compat.psd of upstream is the same image, saved with a
	// merged image. The two agree within the tolerance of a review (D5),
	// because the composite leaves out the blend modes and the masks
	d, err := Open(t.Context(), bytes.NewReader(read(t, "psd_nocompat.psd")))
	if err != nil {
		t.Fatal(err)
	}
	if hasMergedImage(d.cfg) {
		t.Fatal("expected a file without a merged image")
	}
	got, err := Decode(bytes.NewReader(read(t, "psd_nocompat.psd")))
	if err != nil {
		t.Fatal(err)
	}
	want, err := Decode(bytes.NewReader(read(t, "psd_compat.psd")))
	if err != nil {
		t.Fatal(err)
	}
	if diff := meanDiff(got, want); diff > 16 {
		t.Errorf("expected the composite near the merged image, got a mean difference of %.1f", diff)
	}
}

// meanDiff returns the mean absolute difference of the channels of a and b,
// over a white background, from 0 to 255.
func meanDiff(a, b image.Image) float64 {
	var sum, n float64
	bounds := a.Bounds()
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			ca, cb := overWhite(a.At(x, y)), overWhite(b.At(x, y))
			for i := range ca {
				sum += abs(ca[i] - cb[i])
				n++
			}
		}
	}
	return sum / n
}

// overWhite returns the color c over white, in 8 bits for each channel.
func overWhite(c color.Color) [3]float64 {
	r, g, b, a := c.RGBA()
	white := float64(0xffff - a)
	return [3]float64{(float64(r) + white) / 257, (float64(g) + white) / 257, (float64(b) + white) / 257}
}

func abs(v float64) float64 {
	if v < 0 {
		return -v
	}
	return v
}

func TestPreview(t *testing.T) {
	d, err := Open(t.Context(), bytes.NewReader(read(t, "rgb8bit.psd")))
	if err != nil {
		t.Fatal(err)
	}
	img, err := ivy.BestPreview(t.Context(), d, image.Point{})
	if err != nil {
		t.Fatal(err)
	}
	if size := img.Bounds().Size(); size.X == 0 || size.X > 160 {
		t.Errorf("expected a small thumbnail, got: %v", size)
	}
	d, err = Open(t.Context(), bytes.NewReader(read(t, "rgba8.psd")))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ivy.BestPreview(t.Context(), d, image.Point{}); !errors.Is(err, ivy.ErrNoPreview) {
		t.Errorf("expected ErrNoPreview, got: %v", err)
	}
}

func TestImageDecode(t *testing.T) {
	img, name, err := image.Decode(bytes.NewReader(read(t, "rgba8.psd")))
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := img.(*image.NRGBA); !ok || name != "psd" {
		t.Errorf("expected an *image.NRGBA of psd, got: %T %s", img, name)
	}
	cfg, _, err := image.DecodeConfig(bytes.NewReader(read(t, "deep16.psd")))
	if err != nil || cfg.Width != 96 || cfg.Height != 64 {
		t.Errorf("expected 96x64, got: %+v %v", cfg, err)
	}
}

func TestErrors(t *testing.T) {
	if _, err := Open(t.Context(), bytes.NewReader([]byte("8BPS"))); !errors.Is(err, ivy.ErrFormat) {
		t.Errorf("expected ErrFormat, got: %v", err)
	}
	cfg := ivy.Config{Limits: ivy.Limits{MaxPixels: 100}}
	if _, err := Open(t.Context(), bytes.NewReader(read(t, "rgba8.psd")), WithConfig(cfg)); !errors.Is(err, ivy.ErrLimit) {
		t.Errorf("expected ErrLimit, got: %v", err)
	}
	cfg = ivy.Config{Limits: ivy.Limits{MaxMemory: 1000}}
	if _, err := Open(t.Context(), bytes.NewReader(read(t, "rgba8.psd")), WithConfig(cfg)); !errors.Is(err, ivy.ErrLimit) {
		t.Errorf("expected ErrLimit, got: %v", err)
	}
	broken := read(t, "rgba8.psd")
	broken = broken[:len(broken)-1000]
	d, err := Open(t.Context(), bytes.NewReader(broken))
	if err != nil {
		t.Fatal(err)
	}
	page, _ := d.Page(t.Context(), 0)
	if _, err := ivy.RenderImage(t.Context(), page, 72, nil); !errors.Is(err, ivy.ErrFormat) {
		t.Errorf("expected ErrFormat, got: %v", err)
	}
}

func FuzzOpen(f *testing.F) {
	for _, name := range []string{"rgba8.psd", "deep16.psd", "rgb8bit.psd"} {
		f.Add(read(f, name))
	}
	// a file with a spot channel, an alpha channel and an alternate color
	alt := binary.BigEndian.AppendUint32([]byte{0, 1, 0, 1}, 7)
	f.Add(psdFile(2, 1, [][]byte{{255, 0}, {255, 0}, {255, 0}, {0, 255}, {0, 0}}, map[uint16][]byte{
		resAlphaIDs: append(binary.BigEndian.AppendUint32(nil, 7), 0, 0, 0, 9),
		resDisplayInfo: displayInfoRes(
			displayEntry(colorStruct(3), 30, kindSpot),
			displayEntry(colorStruct(spaceRGB, 0xffff), 50, 1),
		),
		resAltSpotColors: append(alt, colorStruct(spaceLab, 5000, 0xf000, 0x1000)...),
	}))
	cfg := ivy.Config{Limits: ivy.Limits{MaxMemory: 1 << 20, MaxPixels: 1 << 16}}
	f.Fuzz(func(t *testing.T, buf []byte) {
		d, err := Open(t.Context(), bytes.NewReader(buf), WithConfig(cfg))
		if err != nil {
			return
		}
		page, err := d.Page(t.Context(), 0)
		if err != nil {
			t.Fatal(err)
		}
		_, _ = ivy.RenderImage(t.Context(), page, 72, nil)
		for p := range d.Previews(t.Context()) {
			_, _ = p.Decode(t.Context())
		}
	})
}
