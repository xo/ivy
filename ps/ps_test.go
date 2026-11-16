package ps

import (
	"bytes"
	"errors"
	"image"
	"image/color"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/xo/ivy"
)

func open(t *testing.T, name string) *Document {
	t.Helper()
	buf, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatal(err)
	}
	d, err := Open(t.Context(), bytes.NewReader(buf))
	if err != nil {
		t.Fatal(err)
	}
	return d
}

func TestTIFFPreviewUnsupported(t *testing.T) {
	// ImageMagick writes a TIFF preview with a palette of 4 bits, which
	// golang.org/x/image/tiff does not decode
	d := open(t, "tiff4-preview.eps")
	if _, err := ivy.BestPreview(t.Context(), d, image.Point{}); !errors.Is(err, ivy.ErrUnsupported) {
		t.Errorf("expected ErrUnsupported, got: %v", err)
	}
}

func TestTIFFPreview(t *testing.T) {
	d := open(t, "tiff8-preview.eps")
	img, err := ivy.BestPreview(t.Context(), d, image.Point{})
	if err != nil {
		t.Fatal(err)
	}
	if img.Bounds().Size() != image.Pt(16, 8) {
		t.Errorf("expected 16x8, got: %v", img.Bounds().Size())
	}
	info, err := d.Describe(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if info.Producer != "ImageMagick" {
		t.Errorf("expected ImageMagick, got: %q", info.Producer)
	}
}

func TestEPSIPreview(t *testing.T) {
	d := open(t, "half.epsi")
	img, err := ivy.BestPreview(t.Context(), d, image.Point{})
	if err != nil {
		t.Fatal(err)
	}
	if img.Bounds().Size() != image.Pt(32, 16) {
		t.Fatalf("expected 32x16, got: %v", img.Bounds().Size())
	}
	// the left half of the image is black, and the right half is white
	if got := color.GrayModel.Convert(img.At(4, 8)).(color.Gray).Y; got > 32 {
		t.Errorf("expected black on the left, got: %d", got)
	}
	if got := color.GrayModel.Convert(img.At(28, 8)).(color.Gray).Y; got < 223 {
		t.Errorf("expected white on the right, got: %d", got)
	}
}

func TestNoPreview(t *testing.T) {
	d := open(t, "plain.eps")
	if _, err := ivy.BestPreview(t.Context(), d, image.Point{}); !errors.Is(err, ivy.ErrNoPreview) {
		t.Errorf("expected ErrNoPreview, got: %v", err)
	}
	info, err := d.Describe(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if info.Title != "plain.eps" || info.Created.IsZero() {
		t.Errorf("expected a title and a date, got: %+v", info)
	}
}

func TestEPSIDepths(t *testing.T) {
	tests := []struct {
		depth int
		hex   string
		want  []uint8
	}{
		{1, "a0", []uint8{0, 255, 0, 255}},
		{2, "1b", []uint8{255, 170, 85, 0}},
		{4, "f000", []uint8{0, 255, 255, 255}},
		{8, "00ff80ff", []uint8{255, 0, 127, 0}},
	}
	for _, test := range tests {
		eps := "%!PS-Adobe-3.0 EPSF-3.0\r%%BeginPreview: 4 1 " +
			string(rune('0'+test.depth)) + " 1\r% " + test.hex + "\r%%EndPreview\r%%EndComments\r"
		d, err := Open(t.Context(), bytes.NewReader([]byte(eps)))
		if err != nil {
			t.Fatalf("depth %d: %v", test.depth, err)
		}
		img, err := ivy.BestPreview(t.Context(), d, image.Point{})
		if err != nil {
			t.Fatalf("depth %d: %v", test.depth, err)
		}
		for x, want := range test.want {
			if got := img.(*image.Gray).GrayAt(x, 0).Y; got != want {
				t.Errorf("depth %d, x %d: expected %d, got: %d", test.depth, x, want, got)
			}
		}
	}
}

func TestErrors(t *testing.T) {
	tests := []struct {
		name string
		buf  string
		want error
	}{
		{"not postscript", "hello", ivy.ErrFormat},
		{"bad depth", "%!PS\n%%BeginPreview: 1 1 3 1\n% 00\n%%EndPreview\n", ivy.ErrFormat},
		{"no end", "%!PS\n%%BeginPreview: 1 1 8 1\n% 00\n", ivy.ErrFormat},
		{"short data", "%!PS\n%%BeginPreview: 2 2 8 1\n% 00\n%%EndPreview\n", ivy.ErrFormat},
		{"too large", "%!PS\n%%BeginPreview: 100000 100000 8 1\n%%EndPreview\n", ivy.ErrLimit},
		{"dos header", "\xc5\xd0\xd3\xc6\x1e\x00\x00\x00\xff\xff\x00\x00", ivy.ErrFormat},
		{"long line", "%!PS\n%" + strings.Repeat("a", maxLine+1) + "\n", ivy.ErrLimit},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if _, err := Open(t.Context(), bytes.NewReader([]byte(test.buf))); !errors.Is(err, test.want) {
				t.Errorf("expected %v, got: %v", test.want, err)
			}
		})
	}
}

func TestDSCTime(t *testing.T) {
	tests := map[string]time.Time{
		"2026-11-16T02:42:14+00:00": time.Date(2026, 11, 16, 2, 42, 14, 0, time.UTC),
		"D:20240102030405Z":         time.Date(2024, 1, 2, 3, 4, 5, 0, time.UTC),
		"Tue Jan  2 03:04:05 2024":  time.Date(2024, 1, 2, 3, 4, 5, 0, time.UTC),
		"yesterday":                 {},
	}
	for s, want := range tests {
		if got := dscTime(s); !got.Equal(want) {
			t.Errorf("%q: expected %v, got: %v", s, want, got)
		}
	}
}

func FuzzOpen(f *testing.F) {
	for _, name := range []string{"tiff4-preview.eps", "tiff8-preview.eps", "half.epsi", "plain.eps"} {
		buf, err := os.ReadFile(filepath.Join("testdata", name))
		if err != nil {
			f.Fatal(err)
		}
		f.Add(buf)
	}
	cfg := ivy.Config{Limits: ivy.Limits{MaxMemory: 1 << 20, MaxPixels: 1 << 16}}
	f.Fuzz(func(t *testing.T, buf []byte) {
		d, err := Open(t.Context(), bytes.NewReader(buf), WithConfig(cfg))
		if err != nil {
			return
		}
		for p, err := range d.Previews(t.Context()) {
			if err != nil {
				return
			}
			_, _ = p.Decode(t.Context())
		}
	})
}
