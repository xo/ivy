package emf

import (
	"bytes"
	"compress/gzip"
	"errors"
	"image"
	"image/color"
	"strings"
	"testing"

	"github.com/xo/okage"

	"github.com/xo/ivy"
	"github.com/xo/ivy/internal/fixture"
)

var (
	red   = color.NRGBA{255, 0, 0, 255}
	white = color.NRGBA{255, 255, 255, 255}
)

// square returns an EMF of 100 by 100 pixels, 72 by 72 points, with a red
// square in its top right quarter.
func square() *fixture.EMF {
	e := fixture.NewEMF(100, 100)
	e.Select(e.Brush(red, -1))
	e.Select(fixture.Stock(8))
	e.Rectangle(50, 0, 100, 50)
	return e
}

// wmfSquare returns a placeable WMF of 2 by 2 inches with a red square in
// its top right quarter.
func wmfSquare() []byte {
	w := fixture.NewWMF(200, 200, true)
	w.Select(w.Brush(red))
	w.Select(w.NullPen())
	w.Rectangle(100, 0, 200, 100)
	return w.Bytes()
}

// compress returns data in gzip.
func compress(t *testing.T, data []byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := gzip.NewWriter(&buf)
	if _, err := zw.Write(data); err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// near reports whether the colors differ by at most 8 in each channel.
func near(a, b color.Color) bool {
	ar, ag, ab, aa := a.RGBA()
	br, bg, bb, ba := b.RGBA()
	d := func(x, y uint32) bool { return max(x, y)-min(x, y) <= 8<<8 }
	return d(ar, br) && d(ag, bg) && d(ab, bb) && d(aa, ba)
}

func TestOpen(t *testing.T) {
	for _, c := range []struct {
		name string
		data []byte
		size float32
	}{
		{"emf", square().Bytes(), 72},
		{"emz", compress(t, square().Bytes()), 72},
		{"wmf", wmfSquare(), 144},
		{"wmz", compress(t, wmfSquare()), 144},
	} {
		t.Run(c.name, func(t *testing.T) {
			d, err := Open(t.Context(), bytes.NewReader(c.data))
			if err != nil {
				t.Fatal(err)
			}
			defer func() { _ = d.Close() }()
			if n, err := d.PageCount(t.Context()); n != 1 || err != nil {
				t.Fatalf("page count %d, %v", n, err)
			}
			if _, err := d.Page(t.Context(), 1); !errors.Is(err, ivy.ErrPageRange) {
				t.Errorf("page 1: %v, want ErrPageRange", err)
			}
			p, err := d.Page(t.Context(), 0)
			if err != nil {
				t.Fatal(err)
			}
			if w, h := p.Size(); w != c.size || h != c.size {
				t.Errorf("size %gx%g, want %g", w, h, c.size)
			}
			img, err := ivy.RenderImage(t.Context(), p, 72, color.White)
			if err != nil {
				t.Fatal(err)
			}
			s := int(c.size)
			for _, pt := range []struct {
				x, y int
				want color.Color
			}{
				{s * 3 / 4, s / 4, red},
				{s / 4, s / 4, white},
				{s * 3 / 4, s * 3 / 4, white},
			} {
				if got := img.At(pt.x, pt.y); !near(got, pt.want) {
					t.Errorf("pixel (%d, %d) = %v, want %v", pt.x, pt.y, got, pt.want)
				}
			}
		})
	}
}

func TestFormat(t *testing.T) {
	d, err := Format.Open(t.Context(), bytes.NewReader(square().Bytes()), ivy.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := d.(ivy.Pager); !ok {
		t.Error("a document is not a Pager")
	}
}

func TestDescribe(t *testing.T) {
	d, err := Open(t.Context(), bytes.NewReader(square().Describe("Paint", "Square").Bytes()))
	if err != nil {
		t.Fatal(err)
	}
	info, err := d.Describe(t.Context())
	if err != nil || info.Producer != "Paint" || info.Title != "Square" {
		t.Errorf("describe %+v, %v", info, err)
	}
	d, err = Open(t.Context(), bytes.NewReader(wmfSquare()))
	if err != nil {
		t.Fatal(err)
	}
	if info, err := d.Describe(t.Context()); err != nil || info.Title != "" || info.Producer != "" {
		t.Errorf("describe wmf %+v, %v", info, err)
	}
}

func TestErrors(t *testing.T) {
	for name, data := range map[string][]byte{
		"text":      []byte("this is not a metafile"),
		"truncated": square().Bytes()[:100],
		"gzip":      compress(t, []byte("this is not a metafile")),
		"bad gzip":  {0x1f, 0x8b, 8, 0, 0},
	} {
		if _, err := Open(t.Context(), bytes.NewReader(data)); !errors.Is(err, ivy.ErrFormat) {
			t.Errorf("%s: %v, want ErrFormat", name, err)
		}
	}
	cfg := ivy.Config{Limits: ivy.Limits{MaxMemory: 1000}}
	big := compress(t, append(square().Bytes(), make([]byte, 4000)...))
	if _, err := Open(t.Context(), bytes.NewReader(big), WithConfig(cfg)); !errors.Is(err, ivy.ErrLimit) {
		t.Errorf("past MaxMemory: %v, want ErrLimit", err)
	}
	// a WMF file with neither a placeable header nor a window has no size
	w := fixture.NewWMF(10, 10, true).Bytes()[22:]
	if _, err := Open(t.Context(), bytes.NewReader(w)); !errors.Is(err, ivy.ErrUnsupported) {
		t.Errorf("wmf without a size: %v, want ErrUnsupported", err)
	}
}

func TestPartial(t *testing.T) {
	// EMR_EXTFLOODFILL needs the pixels of the device
	e := square().Record(53, 20, 20, color.NRGBA{}, uint32(0))
	d, err := Open(t.Context(), bytes.NewReader(e.Bytes()))
	if err != nil {
		t.Fatal(err)
	}
	p, err := d.Page(t.Context(), 0)
	if err != nil {
		t.Fatal(err)
	}
	img, err := ivy.RenderImage(t.Context(), p, 72, color.White)
	if _, ok := errors.AsType[*ivy.PartialError](err); !ok {
		t.Fatalf("render: %v, want a partial error", err)
	}
	if !near(img.At(60, 10), red) {
		t.Errorf("the part before the flood fill is missing")
	}
}

func FuzzOpen(f *testing.F) {
	img := image.NewNRGBA(image.Rect(0, 0, 2, 2))
	img.SetNRGBA(1, 1, red)
	text := fixture.NewEMF(100, 100)
	text.Select(text.Font(-20, 700, true, "Arial"))
	text.Text(5, 5, "Hi")
	clip := square().IntersectClip(10, 10, 90, 90).ExcludeClip(20, 20, 40, 40)
	pen := fixture.NewEMF(100, 100)
	pen.Select(pen.Pen(1, 1, red))
	pen.Polyline(0, 0, 50, 90, 100, 0)
	for _, data := range [][]byte{
		square().Bytes(),
		wmfSquare(),
		fixture.NewWMF(100, 50, false).Bytes(),
		text.Bytes(),
		clip.Bytes(),
		pen.Bytes(),
		fixture.NewEMF(100, 100).Image(0, 0, 50, 50, img).Gradient(50, 50, 100, 100, red, white, true).Bytes(),
	} {
		f.Add(data)
	}
	cfg := ivy.Config{Limits: ivy.Limits{MaxMemory: 4 << 20, MaxPixels: 1 << 20, MaxObjects: 10000, MaxDepth: 16}}
	f.Fuzz(func(t *testing.T, data []byte) {
		d, err := Open(t.Context(), bytes.NewReader(data), WithConfig(cfg))
		if err != nil {
			if strings.Contains(err.Error(), "panic") {
				t.Fatal(err)
			}
			return
		}
		_, _ = d.Describe(t.Context())
		p, err := d.Page(t.Context(), 0)
		if err != nil {
			t.Fatal(err)
		}
		w, h := p.Size()
		dst, _ := okage.NewPixmap(32, 32)
		scale := 32 / max(w, h, 1)
		if err := p.Render(t.Context(), dst, okage.TransformFromScale(scale, scale)); err != nil && strings.Contains(err.Error(), "panic") {
			t.Fatal(err)
		}
	})
}
