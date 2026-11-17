package ps

import (
	"bytes"
	"context"
	"errors"
	"image"
	"image/color"
	"math"
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

// renderPage renders page n of a document at 72 dots per inch.
func renderPage(t *testing.T, d *Document, n int) (image.Image, error) {
	t.Helper()
	p, err := d.Page(t.Context(), n)
	if err != nil {
		t.Fatal(err)
	}
	return ivy.RenderImage(t.Context(), p, 72, color.White)
}

func TestPages(t *testing.T) {
	// a file that starts with a bare %! is PostScript (D29)
	program := "%!\n0 0 1 setrgbcolor 0 0 100 100 rectfill showpage\n1 0 0 setrgbcolor 0 0 100 100 rectfill showpage\n"
	d, err := Open(t.Context(), bytes.NewReader([]byte(program)))
	if err != nil {
		t.Fatal(err)
	}
	n, err := d.PageCount(t.Context())
	if err != nil || n != 2 {
		t.Fatalf("expected 2 pages, got: %d, %v", n, err)
	}
	p, err := d.Page(t.Context(), 1)
	if err != nil {
		t.Fatal(err)
	}
	if w, h := p.Size(); w != 612 || h != 792 {
		t.Errorf("expected a page of 612 by 792, got: %g by %g", w, h)
	}
	img, err := renderPage(t, d, 1)
	if err != nil {
		t.Fatal(err)
	}
	// the page has y down: the square is at the bottom left
	if r, g, b, _ := img.At(50, 792-50).RGBA(); r>>8 != 255 || g>>8 != 0 || b>>8 != 0 {
		t.Errorf("expected red, got: %d %d %d", r>>8, g>>8, b>>8)
	}
	if _, err := d.Page(t.Context(), 2); !errors.Is(err, ivy.ErrPageRange) {
		t.Errorf("expected ErrPageRange, got: %v", err)
	}
}

func TestEPSPage(t *testing.T) {
	// plain.eps draws a gradient of 16 by 8 from red at the top to blue
	d := open(t, "plain.eps")
	n, err := d.PageCount(t.Context())
	if err != nil || n != 1 {
		t.Fatalf("expected 1 page, got: %d, %v", n, err)
	}
	img, err := renderPage(t, d, 0)
	if err != nil {
		t.Fatal(err)
	}
	if img.Bounds().Size() != image.Pt(16, 8) {
		t.Fatalf("expected 16x8, got: %v", img.Bounds().Size())
	}
	top := color.RGBAModel.Convert(img.At(8, 0)).(color.RGBA)
	bottom := color.RGBAModel.Convert(img.At(8, 7)).(color.RGBA)
	if top.R < 200 || top.B > 50 || bottom.B < 200 || bottom.R > 50 {
		t.Errorf("expected red over blue, got: %v over %v", top, bottom)
	}
}

func TestPartialPage(t *testing.T) {
	// an error of the program ends the page, which keeps what it drew
	d, err := Open(t.Context(), bytes.NewReader([]byte("%!PS\n0 0 100 100 rectfill undefinedname\n")))
	if err != nil {
		t.Fatal(err)
	}
	img, err := renderPage(t, d, 0)
	var pe *ivy.PartialError
	if !errors.As(err, &pe) {
		t.Fatalf("expected a PartialError, got: %v", err)
	}
	if r, _, _, _ := img.At(50, 792-50).RGBA(); r>>8 != 0 {
		t.Errorf("expected black, got: %d", r>>8)
	}
}

func TestCancelPages(t *testing.T) {
	d, err := Open(t.Context(), bytes.NewReader([]byte("%!PS\n{ 1 pop } loop\n")))
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if _, err := d.PageCount(ctx); !errors.Is(err, context.Canceled) {
		t.Errorf("expected context.Canceled, got: %v", err)
	}
}

// fuzzSeeds are small programs for the fuzz test.
var fuzzSeeds = []string{
	"%!PS\n/Helvetica findfont 12 scalefont setfont 10 10 moveto (Hello) show showpage\n",
	"%!PS\n<< /ShadingType 2 /ColorSpace /DeviceRGB /Coords [0 0 100 0] /Function << /FunctionType 2 /Domain [0 1] /C0 [1 0 0] /C1 [0 0 1] /N 1 >> >> shfill\n",
	"%!PS\n100 100 scale 2 2 8 [2 0 0 2 0 0] <00ff ff00> image\n",
	"%!PS\n<< /PatternType 1 /PaintType 1 /TilingType 1 /BBox [0 0 20 20] /XStep 20 /YStep 20 /PaintProc { pop 0 0 10 20 rectfill } >> matrix makepattern setpattern 0 0 100 100 rectfill\n",
	"%!PS-Adobe-3.0 EPSF-3.0\n%%BoundingBox: 0 0 10 10\n[/Indexed /DeviceRGB 1 <ff000000ff00>] setcolorspace 1 setcolor 0 0 10 10 rectfill\n",
	"%!PS\n8 dict begin /FontType 3 def /FontMatrix [0.01 0 0 0.01 0 0] def /FontBBox [0 0 100 100] def /Encoding 256 array def 0 1 255 { Encoding exch /a put } for /BuildChar { pop pop 100 0 0 0 100 100 setcachedevice 0 0 50 100 rectfill } def currentdict end /T exch definefont 10 scalefont setfont 0 0 moveto (ab) show\n",
}

func FuzzOpen(f *testing.F) {
	for _, name := range []string{"tiff4-preview.eps", "tiff8-preview.eps", "half.epsi", "plain.eps"} {
		buf, err := os.ReadFile(filepath.Join("testdata", name))
		if err != nil {
			f.Fatal(err)
		}
		f.Add(buf)
	}
	for _, s := range fuzzSeeds {
		f.Add([]byte(s))
	}
	cfg := ivy.Config{Limits: ivy.Limits{MaxMemory: 1 << 20, MaxPixels: 1 << 16, MaxObjects: 1 << 12, MaxDepth: 16}}
	f.Fuzz(func(t *testing.T, buf []byte) {
		d, err := Open(t.Context(), bytes.NewReader(buf), WithConfig(cfg))
		if err != nil {
			return
		}
		for p, err := range d.Previews(t.Context()) {
			if err != nil {
				break
			}
			_, _ = p.Decode(t.Context())
		}
		n, err := d.PageCount(t.Context())
		if err != nil || n == 0 {
			return
		}
		p, err := d.Page(t.Context(), 0)
		if err != nil {
			return
		}
		w, h := p.Size()
		// a page of the fuzz test renders at most 1 million pixels
		dpi := float32(math.Min(72, 72*1000/math.Max(float64(w), float64(h))))
		_, _ = ivy.RenderImage(t.Context(), p, dpi, color.White)
	})
}
