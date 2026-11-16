package pub

import (
	"bytes"
	"errors"
	"image"
	"image/color"
	"strings"
	"testing"

	"github.com/xo/ivy"
	"github.com/xo/ivy/internal/fixture"
)

// publication returns a Publisher file with a thumbnail in EMF.
func publication() []byte {
	e := fixture.NewEMF(100, 200)
	e.Select(e.Brush(color.NRGBA{0, 128, 0, 255}, -1))
	e.Select(fixture.Stock(8))
	e.Rectangle(0, 0, 100, 200)
	sum := fixture.Summary{
		Title:           []byte("Flyer"),
		Author:          []byte("Ann"),
		ThumbnailTag:    -1,
		ThumbnailFormat: fixture.ClipEnhMetafile,
		Thumbnail:       e.Bytes(),
	}
	return fixture.CFB(
		fixture.CFBEntry{Name: "\x05SummaryInformation", Data: sum.Bytes()},
		fixture.CFBEntry{Name: "Contents", Data: make([]byte, 64)},
		fixture.CFBEntry{Name: "Quill", Storage: true, Children: []fixture.CFBEntry{{Name: "QuillSub", Storage: true}}},
	)
}

func TestOpen(t *testing.T) {
	d, err := Format.Open(t.Context(), bytes.NewReader(publication()), ivy.Config{})
	if err != nil {
		t.Fatal(err)
	}
	info, err := d.(ivy.Describer).Describe(t.Context())
	if err != nil || info.Title != "Flyer" || len(info.Authors) != 1 || info.Authors[0] != "Ann" {
		t.Errorf("expected the title and the author, got: %+v, %v", info, err)
	}
	img, err := ivy.BestPreview(t.Context(), d, image.Point{})
	if err != nil {
		t.Fatal(err)
	}
	if img.Bounds().Size() != image.Pt(128, 256) {
		t.Errorf("expected 128x256, got: %v", img.Bounds().Size())
	}
}

func TestOpenErrors(t *testing.T) {
	if _, err := Open(t.Context(), bytes.NewReader([]byte("plain text, not a publication"))); !errors.Is(err, ivy.ErrFormat) {
		t.Errorf("expected ivy.ErrFormat, got: %v", err)
	}
	other := fixture.CFB(fixture.CFBEntry{Name: "WordDocument", Data: []byte("x")})
	if _, err := Open(t.Context(), bytes.NewReader(other)); !errors.Is(err, ivy.ErrFormat) {
		t.Errorf("expected ivy.ErrFormat for another compound file, got: %v", err)
	}
	none := fixture.CFB(fixture.CFBEntry{Name: "Contents", Data: []byte("x")})
	d, err := Open(t.Context(), bytes.NewReader(none))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ivy.BestPreview(t.Context(), d, image.Point{}); !errors.Is(err, ivy.ErrNoPreview) {
		t.Errorf("expected ivy.ErrNoPreview, got: %v", err)
	}
}

func FuzzOpen(f *testing.F) {
	f.Add(publication())
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
		for p, err := range d.Previews(t.Context()) {
			if err != nil {
				return
			}
			if _, err := p.Decode(t.Context()); err != nil && strings.Contains(err.Error(), "panic") {
				t.Fatal(err)
			}
		}
	})
}
