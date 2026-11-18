package xlsx

import (
	"bytes"
	"crypto/sha256"
	"errors"
	"fmt"
	"image"
	"image/color"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/xo/ivy"
	"github.com/xo/ivy/internal/fixture"
	"github.com/xo/ivy/internal/sheet"
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
	f.Add(fixture.XLSX(longBook(120)))
	cfg := ivy.Config{Limits: ivy.Limits{MaxMemory: 1 << 20, MaxPixels: 1 << 16, MaxObjects: 1 << 14}}
	f.Fuzz(func(t *testing.T, buf []byte) {
		// parts of 1 page, so that a sheet of 2 pages is laid out in parts
		// (D55)
		ctx := sheet.WithPartPages(t.Context(), 1)
		d, err := Open(ctx, bytes.NewReader(buf), WithConfig(cfg))
		if err != nil {
			return
		}
		for p, err := range d.Previews(ctx) {
			if err != nil {
				break
			}
			_, _ = p.Decode(ctx)
		}
		// the first page of the sheets
		page, err := d.Page(ctx, 0)
		if err != nil {
			return
		}
		_, _ = ivy.RenderImage(ctx, page, 8, color.White)
		// the last page
		if n, err := d.PageCount(ctx); err == nil {
			if page, err := d.Page(ctx, n-1); err == nil {
				_, _ = ivy.RenderImage(ctx, page, 8, color.White)
			}
		}
	})
}

// longBook returns a workbook of a sheet of n rows over several pages, with
// merged cells, print titles, a manual break and a footer with the number
// of pages.
func longBook(n int) fixture.XLSXBook {
	var rows strings.Builder
	for i := 1; i <= n; i++ {
		fmt.Fprintf(&rows, `<row r="%d"><c r="A%d"><v>%d</v></c><c r="B%d" t="inlineStr"><is><t>row %d</t></is></c></row>`, i, i, i, i, i)
	}
	return fixture.XLSXBook{Sheets: []fixture.XLSXSheet{{Name: "Long", Body: `<sheetPr><pageSetUpPr fitToPage="0"/></sheetPr>` +
		`<sheetData>` + rows.String() + `</sheetData>` +
		`<mergeCells count="2"><mergeCell ref="A40:A50"/><mergeCell ref="B90:C93"/></mergeCells>` +
		`<pageMargins left="0.5" right="0.5" top="1" bottom="1" header="0.4" footer="0.4"/>` +
		`<headerFooter><oddFooter>&amp;CPage &amp;P of &amp;N</oddFooter></headerFooter>` +
		`<rowBreaks count="1" manualBreakCount="1"><brk id="70" max="16383" man="1"/></rowBreaks>`}},
		Names: `<definedName name="_xlnm.Print_Titles" localSheetId="0">Long!$1:$1</definedName>`,
	}
}

// TestLongBookParts checks that the pages of a sheet in parts of 1 page
// are the pages of the whole sheet (D55).
func TestLongBookParts(t *testing.T) {
	buf := fixture.XLSX(longBook(120))
	hashes := func(n int) []string {
		ctx := sheet.WithPartPages(t.Context(), n)
		d, err := Open(ctx, bytes.NewReader(buf))
		if err != nil {
			t.Fatal(err)
		}
		count, err := d.PageCount(ctx)
		if err != nil {
			t.Fatal(err)
		}
		out := make([]string, 0, count)
		for i := range count {
			page, err := d.Page(ctx, i)
			if err != nil {
				t.Fatal(err)
			}
			img, err := ivy.RenderImage(ctx, page, 36, color.White)
			if err != nil {
				t.Fatal(err)
			}
			h := sha256.New()
			b := img.Bounds()
			for y := b.Min.Y; y < b.Max.Y; y++ {
				for x := b.Min.X; x < b.Max.X; x++ {
					r, g, bl, _ := img.At(x, y).RGBA()
					_, _ = h.Write([]byte{byte(r >> 8), byte(g >> 8), byte(bl >> 8)})
				}
			}
			out = append(out, string(h.Sum(nil)))
		}
		return out
	}
	want := hashes(0)
	if len(want) < 3 {
		t.Fatalf("expected 3 pages or more, got %d", len(want))
	}
	if got := hashes(1); !slices.Equal(got, want) {
		t.Errorf("expected the pages of the whole sheet, got %d pages that differ from %d", len(got), len(want))
	}
}
