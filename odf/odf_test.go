package odf

import (
	"bytes"
	"errors"
	"image"
	"image/color"
	"os"
	"path/filepath"
	"testing"

	"github.com/xo/ivy"
	"github.com/xo/ivy/internal/fixture"
)

func TestOpenLibreOffice(t *testing.T) {
	for _, name := range []string{"sample.odt", "sheet.ods", "sample.odp"} {
		t.Run(name, func(t *testing.T) {
			buf, err := os.ReadFile(filepath.Join("testdata", name))
			if err != nil {
				t.Fatal(err)
			}
			d, err := Open(t.Context(), bytes.NewReader(buf))
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() {
				if err := d.Close(); err != nil {
					t.Error(err)
				}
			})
			if d.Mime() == "" {
				t.Error("expected a mime type")
			}
			img, err := ivy.BestPreview(t.Context(), d, image.Point{})
			if err != nil {
				t.Fatal(err)
			}
			if size := img.Bounds().Size(); size.X == 0 || size.X > 512 || size.Y > 512 {
				t.Errorf("expected a thumbnail of at most 512 pixels, got: %v", size)
			}
		})
	}
}

func TestOpenErrors(t *testing.T) {
	tests := []struct {
		name string
		buf  []byte
	}{
		{"not a zip", []byte("plain text")},
		{"wrong mime", fixture.Zip(fixture.Entry{Name: "mimetype", Data: []byte("image/openraster")})},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if _, err := Open(t.Context(), bytes.NewReader(test.buf)); !errors.Is(err, ivy.ErrFormat) {
				t.Errorf("expected ErrFormat, got: %v", err)
			}
		})
	}
}

func TestNoThumbnail(t *testing.T) {
	buf := fixture.Zip(fixture.Entry{Name: "mimetype", Data: []byte(mimePrefix + "text")})
	d, err := Open(t.Context(), bytes.NewReader(buf))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ivy.BestPreview(t.Context(), d, image.Point{}); !errors.Is(err, ivy.ErrNoPreview) {
		t.Errorf("expected ErrNoPreview, got: %v", err)
	}
}

func TestFormat(t *testing.T) {
	buf := fixture.Zip(
		fixture.Entry{Name: "mimetype", Data: []byte(mimePrefix + "graphics")},
		fixture.Entry{Name: "Thumbnails/thumbnail.png", Data: fixture.PNG(10, 20, color.White)},
	)
	d, err := Format.Open(t.Context(), bytes.NewReader(buf), ivy.Config{})
	if err != nil {
		t.Fatal(err)
	}
	img, err := ivy.BestPreview(t.Context(), d, image.Pt(10, 10))
	if err != nil {
		t.Fatal(err)
	}
	if img.Bounds().Size() != image.Pt(10, 20) {
		t.Errorf("expected 10x20, got: %v", img.Bounds().Size())
	}
}

func FuzzOpen(f *testing.F) {
	for _, name := range []string{"sample.odt", "sheet.ods", "sample.odp"} {
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
