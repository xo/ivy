package pptx

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

// contentTypes is a minimal [Content_Types].xml.
var contentTypes = fixture.Entry{Name: "[Content_Types].xml", Data: []byte("<Types/>")}

func TestOpenLibreOffice(t *testing.T) {
	buf, err := os.ReadFile(filepath.Join("testdata", "sample.pptx"))
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
	// LibreOffice writes no thumbnail into an Office Open XML file
	if _, err := ivy.BestPreview(t.Context(), d, image.Point{}); !errors.Is(err, ivy.ErrNoPreview) {
		t.Errorf("expected ErrNoPreview, got: %v", err)
	}
}

func TestThumbnail(t *testing.T) {
	buf := fixture.Zip(
		contentTypes,
		fixture.Entry{Name: "docProps/thumbnail.jpeg", Data: fixture.JPEG(40, 30, color.White)},
	)
	d, err := Format.Open(t.Context(), bytes.NewReader(buf), ivy.Config{})
	if err != nil {
		t.Fatal(err)
	}
	img, err := ivy.BestPreview(t.Context(), d, image.Point{})
	if err != nil {
		t.Fatal(err)
	}
	if img.Bounds().Size() != image.Pt(40, 30) {
		t.Errorf("expected 40x30, got: %v", img.Bounds().Size())
	}
}

func TestThumbnailEMF(t *testing.T) {
	buf := fixture.Zip(contentTypes, fixture.Entry{Name: "docProps/thumbnail.emf", Data: []byte("emf")})
	d, err := Open(t.Context(), bytes.NewReader(buf))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ivy.BestPreview(t.Context(), d, image.Point{}); !errors.Is(err, ivy.ErrNoPreview) {
		t.Errorf("expected ErrNoPreview until the EMF reader, got: %v", err)
	}
}

func TestOpenErrors(t *testing.T) {
	if _, err := Open(t.Context(), bytes.NewReader(fixture.Zip(fixture.Entry{Name: "a"}))); !errors.Is(err, ivy.ErrFormat) {
		t.Errorf("expected ErrFormat, got: %v", err)
	}
	cfb := append([]byte{0xd0, 0xcf, 0x11, 0xe0, 0xa1, 0xb1, 0x1a, 0xe1}, make([]byte, 504)...)
	if _, err := Open(t.Context(), bytes.NewReader(cfb)); !errors.Is(err, ivy.ErrUnsupported) {
		t.Errorf("expected ErrUnsupported, got: %v", err)
	}
}

func FuzzOpen(f *testing.F) {
	buf, err := os.ReadFile(filepath.Join("testdata", "sample.pptx"))
	if err != nil {
		f.Fatal(err)
	}
	f.Add(buf)
	f.Add(fixture.Zip(contentTypes, fixture.Entry{Name: "docProps/thumbnail.png", Data: fixture.PNG(4, 4, color.Black)}))
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
