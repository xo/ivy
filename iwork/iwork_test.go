package iwork

import (
	"bytes"
	"errors"
	"image"
	"image/color"
	"testing"

	"github.com/xo/ivy"
	"github.com/xo/ivy/internal/fixture"
)

func TestPreviews(t *testing.T) {
	tests := []struct {
		name    string
		entries []fixture.Entry
		want    image.Point
	}{
		{"iwork 2013", []fixture.Entry{
			{Name: "Index/Document.iwa", Data: []byte("iwa")},
			{Name: "preview.jpg", Data: fixture.JPEG(300, 200, color.White)},
			{Name: "preview-web.jpg", Data: fixture.JPEG(150, 100, color.White)},
			{Name: "preview-micro.jpg", Data: fixture.JPEG(30, 20, color.White)},
		}, image.Pt(150, 100)},
		{"iwork 2009", []fixture.Entry{
			{Name: "index.xml.gz", Data: []byte("xml")},
			{Name: "QuickLook/Thumbnail.jpg", Data: fixture.JPEG(64, 48, color.Black)},
		}, image.Pt(64, 48)},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			d, err := Format.Open(t.Context(), bytes.NewReader(fixture.Zip(test.entries...)), ivy.Config{})
			if err != nil {
				t.Fatal(err)
			}
			img, err := ivy.BestPreview(t.Context(), d, image.Pt(100, 100))
			if err != nil {
				t.Fatal(err)
			}
			if got := img.Bounds().Size(); got != test.want {
				t.Errorf("expected %v, got: %v", test.want, got)
			}
		})
	}
}

func TestNoPreview(t *testing.T) {
	d, err := Open(t.Context(), bytes.NewReader(fixture.Zip(fixture.Entry{Name: "Index/Document.iwa"})))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ivy.BestPreview(t.Context(), d, image.Point{}); !errors.Is(err, ivy.ErrNoPreview) {
		t.Errorf("expected ErrNoPreview, got: %v", err)
	}
	if _, err := Open(t.Context(), bytes.NewReader([]byte("x"))); !errors.Is(err, ivy.ErrFormat) {
		t.Errorf("expected ErrFormat, got: %v", err)
	}
}

func FuzzOpen(f *testing.F) {
	f.Add(fixture.Zip(fixture.Entry{Name: "preview.jpg", Data: fixture.JPEG(8, 8, color.White)}))
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
