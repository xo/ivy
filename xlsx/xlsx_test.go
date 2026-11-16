package xlsx

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
	buf, err := os.ReadFile(filepath.Join("testdata", "sheet.xlsx"))
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

func TestDescribe(t *testing.T) {
	book := testBook()
	book.Extra = append(book.Extra, fixture.Entry{Name: "docProps/core.xml", Data: []byte(`<?xml version="1.0" encoding="UTF-8"?>` +
		`<cp:coreProperties xmlns:cp="http://schemas.openxmlformats.org/package/2006/metadata/core-properties" xmlns:dc="http://purl.org/dc/elements/1.1/">` +
		`<dc:title>Budget</dc:title><dc:creator>A writer</dc:creator></cp:coreProperties>`)})
	d := open(t, fixture.XLSX(book), ivy.Config{})
	info, err := d.Describe(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if info.Title != "Budget" || len(info.Authors) != 1 || info.Authors[0] != "A writer" {
		t.Errorf("expected the title and the creator, got: %+v", info)
	}
	// a file without core properties says nothing
	if info, err := open(t, fixture.XLSX(testBook()), ivy.Config{}).Describe(t.Context()); err != nil || info.Title != "" {
		t.Errorf("expected no information and no error, got: %+v, %v", info, err)
	}
}

func TestOpenErrors(t *testing.T) {
	if _, err := Open(t.Context(), bytes.NewReader(fixture.Zip(fixture.Entry{Name: "a"}))); !errors.Is(err, ivy.ErrFormat) {
		t.Errorf("expected ErrFormat, got: %v", err)
	}
	cfb := append([]byte{0xd0, 0xcf, 0x11, 0xe0, 0xa1, 0xb1, 0x1a, 0xe1}, make([]byte, 504)...)
	if _, err := Open(t.Context(), bytes.NewReader(cfb)); !errors.Is(err, ivy.ErrFormat) {
		t.Errorf("expected ErrFormat for a damaged compound file, got: %v", err)
	}
}

func FuzzOpen(f *testing.F) {
	buf, err := os.ReadFile(filepath.Join("testdata", "sheet.xlsx"))
	if err != nil {
		f.Fatal(err)
	}
	f.Add(buf)
	f.Add(fixture.Zip(contentTypes, fixture.Entry{Name: "docProps/thumbnail.png", Data: fixture.PNG(4, 4, color.Black)}))
	f.Add(fixture.XLSX(testBook()))
	f.Add(fixture.XLSX(chartSheetBook("FF0000")))
	cfg := ivy.Config{Limits: ivy.Limits{MaxMemory: 1 << 20, MaxPixels: 1 << 16, MaxObjects: 1 << 14}}
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
		// the first page of the sheets
		page, err := d.Page(t.Context(), 0)
		if err != nil {
			return
		}
		_, _ = ivy.RenderImage(t.Context(), page, 8, color.White)
	})
}
