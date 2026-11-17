package ivy

import (
	"context"
	"errors"
	"fmt"
	"image"
	"image/color"
	"iter"
	"testing"

	"github.com/xo/okage"
)

// testPage is a page that fills itself with one color.
type testPage struct {
	w, h float32
	c    okage.Color
	err  error
}

func (p testPage) Size() (float32, float32) {
	return p.w, p.h
}

func (p testPage) Render(_ context.Context, dst *okage.Pixmap, t okage.Transform) error {
	rect, _ := okage.RectFromXYWH(0, 0, p.w, p.h)
	paint := okage.DefaultPaint()
	paint.SetColor(p.c)
	dst.FillRect(rect, &paint, t, nil)
	return p.err
}

// testDoc is a document with pages and previews.
type testDoc struct {
	pages    []Page
	previews []Preview
}

func (d *testDoc) Close() error {
	return nil
}

func (d *testDoc) PageCount(context.Context) (int, error) {
	return len(d.pages), nil
}

func (d *testDoc) Page(_ context.Context, n int) (Page, error) {
	if n < 0 || n >= len(d.pages) {
		return nil, ErrPageRange
	}
	return d.pages[n], nil
}

func (d *testDoc) Previews(context.Context) iter.Seq2[Preview, error] {
	return func(yield func(Preview, error) bool) {
		for _, p := range d.previews {
			if !yield(p, nil) {
				return
			}
		}
	}
}

func TestRenderImage(t *testing.T) {
	red := okage.ColorFromRGBA8(255, 0, 0, 255)
	img, err := RenderImage(t.Context(), testPage{w: 72, h: 36, c: red}, 144, color.White)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := img.Bounds().Size(), image.Pt(144, 72); got != want {
		t.Errorf("expected size %v, got: %v", want, got)
	}
	if got := color.RGBAModel.Convert(img.At(10, 10)); got != (color.RGBA{255, 0, 0, 255}) {
		t.Errorf("expected red, got: %v", got)
	}
}

func TestRenderImageBackground(t *testing.T) {
	half := okage.ColorFromRGBA8(0, 0, 0, 0)
	img, err := RenderImage(t.Context(), testPage{w: 4, h: 4, c: half}, 72, color.White)
	if err != nil {
		t.Fatal(err)
	}
	if got := color.RGBAModel.Convert(img.At(1, 1)); got != (color.RGBA{255, 255, 255, 255}) {
		t.Errorf("expected white, got: %v", got)
	}
}

func TestRenderImagePartial(t *testing.T) {
	cause := errors.New("bad object")
	p := testPage{w: 4, h: 4, c: okage.ColorBlack, err: &PartialError{Err: cause}}
	img, err := RenderImage(t.Context(), p, 72, nil)
	if img == nil {
		t.Fatal("expected an image")
	}
	if !errors.Is(err, cause) {
		t.Errorf("expected the cause, got: %v", err)
	}
	if _, ok := errors.AsType[*PartialError](err); !ok {
		t.Errorf("expected a *PartialError, got: %T", err)
	}
}

func TestRenderImageErrors(t *testing.T) {
	tests := []struct {
		p    testPage
		dpi  float32
		want error
	}{
		{testPage{w: 1, h: 1}, 0, ErrUnsupported},
		{testPage{w: 0, h: 1}, 72, ErrFormat},
		{testPage{w: 1e6, h: 1e6}, 72, ErrLimit},
		{testPage{w: 1, h: 1, err: ErrFormat}, 72, ErrFormat},
	}
	for i, test := range tests {
		t.Run(fmt.Sprint(i), func(t *testing.T) {
			if _, err := RenderImage(t.Context(), test.p, test.dpi, nil); !errors.Is(err, test.want) {
				t.Errorf("expected %v, got: %v", test.want, err)
			}
		})
	}
}

func TestPages(t *testing.T) {
	d := &testDoc{pages: []Page{testPage{w: 1}, testPage{w: 2}, testPage{w: 3}}}
	var widths []float32
	for p, err := range Pages(t.Context(), d) {
		if err != nil {
			t.Fatal(err)
		}
		w, _ := p.Size()
		widths = append(widths, w)
	}
	if fmt.Sprint(widths) != "[1 2 3]" {
		t.Errorf("expected [1 2 3], got: %v", widths)
	}
}

// sizedPreview returns a preview of size that decodes to an image of that
// size.
func sizedPreview(size image.Point) Preview {
	return Preview{
		Size: size,
		Decode: func(context.Context) (image.Image, error) {
			if size == (image.Point{}) {
				return image.NewGray(image.Rect(0, 0, 7, 7)), nil
			}
			return image.NewGray(image.Rectangle{Max: size}), nil
		},
	}
}

func TestBestPreview(t *testing.T) {
	d := &testDoc{previews: []Preview{
		sizedPreview(image.Pt(16, 16)),
		sizedPreview(image.Pt(512, 512)),
		sizedPreview(image.Pt(128, 128)),
		sizedPreview(image.Point{}),
	}}
	tests := []struct {
		want, got image.Point
	}{
		{image.Pt(100, 100), image.Pt(128, 128)},
		{image.Pt(128, 128), image.Pt(128, 128)},
		{image.Pt(129, 64), image.Pt(512, 512)},
		{image.Pt(1000, 1000), image.Pt(512, 512)},
		{image.Pt(8, 8), image.Pt(16, 16)},
		{image.Point{}, image.Pt(512, 512)},
	}
	for _, test := range tests {
		img, err := BestPreview(t.Context(), d, test.want)
		if err != nil {
			t.Fatal(err)
		}
		if got := img.Bounds().Size(); got != test.got {
			t.Errorf("want %v: expected %v, got: %v", test.want, test.got, got)
		}
	}
}

func TestBestPreviewFallsBack(t *testing.T) {
	bad := Preview{
		Size: image.Pt(64, 64),
		Decode: func(context.Context) (image.Image, error) {
			return nil, ErrFormat
		},
	}
	d := &testDoc{previews: []Preview{bad, sizedPreview(image.Pt(32, 32))}}
	img, err := BestPreview(t.Context(), d, image.Pt(64, 64))
	if err != nil {
		t.Fatal(err)
	}
	if got := img.Bounds().Size(); got != image.Pt(32, 32) {
		t.Errorf("expected the second preview, got: %v", got)
	}
}

func TestBestPreviewNone(t *testing.T) {
	if _, err := BestPreview(t.Context(), &testDoc{}, image.Point{}); !errors.Is(err, ErrNoPreview) {
		t.Errorf("expected ErrNoPreview, got: %v", err)
	}
	type closer struct{ Document }
	if _, err := BestPreview(t.Context(), closer{}, image.Point{}); !errors.Is(err, ErrNoPreview) {
		t.Errorf("expected ErrNoPreview, got: %v", err)
	}
}

func TestLimitsWithDefaults(t *testing.T) {
	lim := Limits{MaxPixels: 10, MaxDepth: -1}.WithDefaults()
	want := Limits{
		MaxMemory:  DefaultMaxMemory,
		MaxPixels:  10,
		MaxObjects: DefaultMaxObjects,
		MaxDepth:   DefaultMaxDepth,
	}
	if lim != want {
		t.Errorf("expected %+v, got: %+v", want, lim)
	}
}

func TestPreviewKindString(t *testing.T) {
	for kind, want := range map[PreviewKind]string{
		Thumbnail:      "thumbnail",
		Icon:           "icon",
		Cover:          "cover",
		Background:     "background",
		PreviewKind(9): "unknown",
	} {
		if got := kind.String(); got != want {
			t.Errorf("expected %q, got: %q", want, got)
		}
	}
}

func TestConfigLog(t *testing.T) {
	if (Config{}).Log() == nil {
		t.Error("expected a logger")
	}
}

func TestPaper(t *testing.T) {
	if p, exp := A4.Landscape(), (Paper{Width: 841.89, Height: 595.28}); p != exp {
		t.Errorf("expected %v, got: %v", exp, p)
	}
	if p := A4.Landscape().Portrait(); p != A4 {
		t.Errorf("expected %v, got: %v", A4, p)
	}
	if p := Letter.Portrait(); p != Letter {
		t.Errorf("expected %v, got: %v", Letter, p)
	}
	if p := (Paper{}).Landscape(); p != (Paper{}) {
		t.Errorf("expected the zero Paper, got: %v", p)
	}
}
