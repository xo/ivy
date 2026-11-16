package ora

import (
	"bytes"
	"errors"
	"image"
	"image/color"
	"testing"

	"github.com/xo/ivy"
	"github.com/xo/ivy/internal/fixture"
	"github.com/xo/okage"
)

// file returns an OpenRaster or a Krita file, with its mimetype entry stored
// first, as the specification of OpenRaster says.
func file(mime string, extra ...fixture.Entry) []byte {
	entries := append([]fixture.Entry{
		{Name: "mimetype", Data: []byte(mime), Store: true},
		{Name: "stack.xml", Data: []byte("<image/>")},
		{Name: "mergedimage.png", Data: fixture.PNG(6, 4, color.NRGBA{255, 0, 0, 255})},
	}, extra...)
	return fixture.Zip(entries...)
}

func TestOpen(t *testing.T) {
	tests := []struct {
		mime    string
		preview string
	}{
		{mimeORA, "Thumbnails/thumbnail.png"},
		{mimeKrita, "preview.png"},
	}
	for _, test := range tests {
		t.Run(test.mime, func(t *testing.T) {
			buf := file(test.mime, fixture.Entry{Name: test.preview, Data: fixture.PNG(3, 2, color.White)})
			d, err := Open(t.Context(), bytes.NewReader(buf))
			if err != nil {
				t.Fatal(err)
			}
			if d.Mime() != test.mime {
				t.Errorf("expected %s, got: %s", test.mime, d.Mime())
			}
			if n, _ := d.PageCount(t.Context()); n != 1 {
				t.Errorf("expected one page, got: %d", n)
			}
			page, err := d.Page(t.Context(), 0)
			if err != nil {
				t.Fatal(err)
			}
			img, err := ivy.RenderImage(t.Context(), page, 72, nil)
			if err != nil {
				t.Fatal(err)
			}
			if img.Bounds().Size() != image.Pt(6, 4) {
				t.Errorf("expected 6x4, got: %v", img.Bounds().Size())
			}
			if got := color.RGBAModel.Convert(img.At(2, 2)); got != (color.RGBA{255, 0, 0, 255}) {
				t.Errorf("expected red, got: %v", got)
			}
			if _, err := d.Page(t.Context(), 1); !errors.Is(err, ivy.ErrPageRange) {
				t.Errorf("expected ErrPageRange, got: %v", err)
			}
			preview, err := ivy.BestPreview(t.Context(), d, image.Point{})
			if err != nil {
				t.Fatal(err)
			}
			if preview.Bounds().Size() != image.Pt(3, 2) {
				t.Errorf("expected 3x2, got: %v", preview.Bounds().Size())
			}
		})
	}
}

func TestImageDecode(t *testing.T) {
	for _, mime := range []string{mimeORA, mimeKrita} {
		img, name, err := image.Decode(bytes.NewReader(file(mime)))
		if err != nil {
			t.Fatalf("%s: %v", mime, err)
		}
		if name != map[string]string{mimeORA: "ora", mimeKrita: "kra"}[mime] {
			t.Errorf("%s: got format %q", mime, name)
		}
		if img.Bounds().Size() != image.Pt(6, 4) {
			t.Errorf("%s: expected 6x4, got: %v", mime, img.Bounds().Size())
		}
		cfg, _, err := image.DecodeConfig(bytes.NewReader(file(mime)))
		if err != nil || cfg.Width != 6 || cfg.Height != 4 {
			t.Errorf("%s: expected 6x4, got: %+v %v", mime, cfg, err)
		}
	}
}

func TestOpenErrors(t *testing.T) {
	tests := []struct {
		name string
		buf  []byte
		want error
	}{
		{"no mimetype", fixture.Zip(fixture.Entry{Name: "mergedimage.png"}), ivy.ErrFormat},
		{"wrong mime", file("image/png"), ivy.ErrFormat},
		{"no merged image", fixture.Zip(fixture.Entry{Name: "mimetype", Data: []byte(mimeORA)}), ivy.ErrFormat},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if _, err := Open(t.Context(), bytes.NewReader(test.buf)); !errors.Is(err, test.want) {
				t.Errorf("expected %v, got: %v", test.want, err)
			}
		})
	}
	cfg := ivy.Config{Limits: ivy.Limits{MaxPixels: 10}}
	if _, err := Open(t.Context(), bytes.NewReader(file(mimeORA)), WithConfig(cfg)); !errors.Is(err, ivy.ErrLimit) {
		t.Errorf("expected ErrLimit, got: %v", err)
	}
}

func FuzzOpen(f *testing.F) {
	f.Add(file(mimeORA))
	f.Add(file(mimeKrita))
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
		dst, _ := okage.NewPixmap(8, 8)
		_ = page.Render(t.Context(), dst, okage.IdentityTransform())
	})
}
