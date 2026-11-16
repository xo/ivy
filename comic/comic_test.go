package comic

import (
	"bytes"
	"errors"
	"image"
	"image/color"
	"slices"
	"testing"

	"github.com/xo/ivy"
	"github.com/xo/ivy/internal/fixture"
)

// pages returns the entries of a comic: three pages, out of order, with a
// file that is not an image and the files of macOS.
func pages() []fixture.Entry {
	return []fixture.Entry{
		{Name: "book/page10.png", Data: fixture.PNG(30, 10, color.White)},
		{Name: "book/page2.jpg", Data: fixture.JPEG(20, 10, color.White)},
		{Name: "book/ComicInfo.xml", Data: []byte("<ComicInfo/>")},
		{Name: "__MACOSX/book/._page1.png", Data: []byte("resource fork")},
		{Name: "book/page1.png", Data: fixture.PNG(10, 10, color.NRGBA{0, 255, 0, 255})},
	}
}

func TestOpen(t *testing.T) {
	for name, buf := range map[string][]byte{
		"cbz": fixture.Zip(pages()...),
		"cbt": fixture.Tar(pages()...),
	} {
		t.Run(name, func(t *testing.T) {
			d, err := Format.Open(t.Context(), bytes.NewReader(buf), ivy.Config{})
			if err != nil {
				t.Fatal(err)
			}
			pager := d.(ivy.Pager)
			if n, _ := pager.PageCount(t.Context()); n != 3 {
				t.Fatalf("expected three pages, got: %d", n)
			}
			var widths []float32
			for p, err := range ivy.Pages(t.Context(), pager) {
				if err != nil {
					t.Fatal(err)
				}
				w, _ := p.Size()
				widths = append(widths, w)
			}
			if !slices.Equal(widths, []float32{10, 20, 30}) {
				t.Errorf("expected the pages in natural order, got: %v", widths)
			}
			page, err := pager.Page(t.Context(), 0)
			if err != nil {
				t.Fatal(err)
			}
			img, err := ivy.RenderImage(t.Context(), page, 144, nil)
			if err != nil {
				t.Fatal(err)
			}
			if img.Bounds().Size() != image.Pt(20, 20) {
				t.Errorf("expected 20x20, got: %v", img.Bounds().Size())
			}
			if got := color.RGBAModel.Convert(img.At(10, 10)); got != (color.RGBA{0, 255, 0, 255}) {
				t.Errorf("expected green, got: %v", got)
			}
			cover, err := ivy.BestPreview(t.Context(), d, image.Point{})
			if err != nil {
				t.Fatal(err)
			}
			if cover.Bounds().Size() != image.Pt(10, 10) {
				t.Errorf("expected the first page as the cover, got: %v", cover.Bounds().Size())
			}
		})
	}
}

func TestEmptyAndErrors(t *testing.T) {
	d, err := Open(t.Context(), bytes.NewReader(fixture.Zip(fixture.Entry{Name: "readme.txt"})))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := d.Page(t.Context(), 0); !errors.Is(err, ivy.ErrPageRange) {
		t.Errorf("expected ErrPageRange, got: %v", err)
	}
	if _, err := ivy.BestPreview(t.Context(), d, image.Point{}); !errors.Is(err, ivy.ErrNoPreview) {
		t.Errorf("expected ErrNoPreview, got: %v", err)
	}
	if _, err := Open(t.Context(), bytes.NewReader([]byte("neither zip nor tar"))); !errors.Is(err, ivy.ErrFormat) {
		t.Errorf("expected ErrFormat, got: %v", err)
	}
	broken := fixture.Zip(fixture.Entry{Name: "1.png", Data: []byte("not a png")})
	d, err = Open(t.Context(), bytes.NewReader(broken))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := d.Page(t.Context(), 0); !errors.Is(err, ivy.ErrFormat) {
		t.Errorf("expected ErrFormat, got: %v", err)
	}
	big := fixture.Zip(fixture.Entry{Name: "1.png", Data: fixture.PNG(100, 100, color.White)})
	d, err = Open(t.Context(), bytes.NewReader(big), WithConfig(ivy.Config{Limits: ivy.Limits{MaxPixels: 99}}))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := d.Page(t.Context(), 0); !errors.Is(err, ivy.ErrLimit) {
		t.Errorf("expected ErrLimit, got: %v", err)
	}
}

func TestNaturalCompare(t *testing.T) {
	names := []string{"p10.png", "P2.png", "p1.png", "p01b.png", "a.png", "p1a.png"}
	slices.SortFunc(names, naturalCompare)
	want := []string{"a.png", "p1.png", "p1a.png", "p01b.png", "P2.png", "p10.png"}
	if !slices.Equal(names, want) {
		t.Errorf("expected %v, got: %v", want, names)
	}
}

func FuzzOpen(f *testing.F) {
	f.Add(fixture.Zip(pages()...))
	f.Add(fixture.Tar(pages()...))
	cfg := ivy.Config{Limits: ivy.Limits{MaxMemory: 1 << 20, MaxPixels: 1 << 16, MaxObjects: 64}}
	f.Fuzz(func(t *testing.T, buf []byte) {
		d, err := Open(t.Context(), bytes.NewReader(buf), WithConfig(cfg))
		if err != nil {
			return
		}
		for p, err := range ivy.Pages(t.Context(), d) {
			if err != nil {
				return
			}
			_, _ = ivy.RenderImage(t.Context(), p, 36, nil)
		}
	})
}
