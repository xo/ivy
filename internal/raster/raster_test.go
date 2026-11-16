package raster

import (
	"bytes"
	"context"
	"errors"
	"image"
	"image/color"
	"strings"
	"testing"

	"github.com/xo/ivy"
	"github.com/xo/ivy/internal/fixture"
	"github.com/xo/okage"
)

func TestReadAll(t *testing.T) {
	if _, err := ReadAll(strings.NewReader("12345"), 4); !errors.Is(err, ivy.ErrLimit) {
		t.Errorf("expected ErrLimit, got: %v", err)
	}
	buf, err := ReadAll(strings.NewReader("1234"), 4)
	if err != nil || string(buf) != "1234" {
		t.Errorf("expected 1234, got: %q %v", buf, err)
	}
}

func TestDecode(t *testing.T) {
	buf := fixture.PNG(8, 4, color.White)
	if size, err := Config(buf); err != nil || size != image.Pt(8, 4) {
		t.Fatalf("expected 8x4, got: %v %v", size, err)
	}
	img, err := Decode(t.Context(), buf, ivy.Limits{})
	if err != nil {
		t.Fatal(err)
	}
	if img.Bounds().Size() != image.Pt(8, 4) {
		t.Errorf("expected 8x4, got: %v", img.Bounds().Size())
	}
	if _, err := Decode(t.Context(), buf, ivy.Limits{MaxPixels: 31}); !errors.Is(err, ivy.ErrLimit) {
		t.Errorf("expected ErrLimit, got: %v", err)
	}
	if _, err := Decode(t.Context(), []byte("not an image"), ivy.Limits{}); !errors.Is(err, ivy.ErrFormat) {
		t.Errorf("expected ErrFormat, got: %v", err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if _, err := Decode(ctx, buf, ivy.Limits{}); !errors.Is(err, context.Canceled) {
		t.Errorf("expected context.Canceled, got: %v", err)
	}
}

func TestPageRender(t *testing.T) {
	src := fixture.Image(4, 2, color.NRGBA{0, 0, 255, 255})
	calls := 0
	p := NewPage(image.Pt(4, 2), func(context.Context) (image.Image, error) {
		calls++
		return src, nil
	})
	if w, h := p.Size(); w != 4 || h != 2 {
		t.Errorf("expected 4x2, got: %vx%v", w, h)
	}
	for range 2 {
		dst, _ := okage.NewPixmap(8, 4)
		if err := p.Render(t.Context(), dst, okage.TransformFromScale(2, 2)); err != nil {
			t.Fatal(err)
		}
		if got := color.RGBAModel.Convert(dst.At(5, 3)); got != (color.RGBA{0, 0, 255, 255}) {
			t.Errorf("expected blue, got: %v", got)
		}
	}
	if calls != 1 {
		t.Errorf("expected one decode, got: %d", calls)
	}
}

func TestPageRenderError(t *testing.T) {
	p := NewPage(image.Pt(1, 1), func(context.Context) (image.Image, error) {
		return nil, ivy.ErrFormat
	})
	dst, _ := okage.NewPixmap(1, 1)
	if err := p.Render(t.Context(), dst, okage.IdentityTransform()); !errors.Is(err, ivy.ErrFormat) {
		t.Errorf("expected ErrFormat, got: %v", err)
	}
}

func TestPixmapSubImage(t *testing.T) {
	src := fixture.Image(4, 4, color.NRGBA{255, 0, 0, 255})
	sub := src.SubImage(image.Rect(1, 1, 3, 3))
	pix, err := Pixmap(sub)
	if err != nil {
		t.Fatal(err)
	}
	if pix.Width() != 2 || pix.Height() != 2 {
		t.Errorf("expected 2x2, got: %dx%d", pix.Width(), pix.Height())
	}
	if _, err := Pixmap(image.NewRGBA(image.Rectangle{})); !errors.Is(err, ivy.ErrFormat) {
		t.Errorf("expected ErrFormat, got: %v", err)
	}
	if !bytes.Equal(pix.Data()[:4], []byte{255, 0, 0, 255}) {
		t.Errorf("expected red, got: %v", pix.Data()[:4])
	}
}
