package ooxml

import (
	"bytes"
	"image"
	"image/color"
	"testing"
	"time"

	"github.com/xo/ivy"
	"github.com/xo/ivy/internal/fixture"
)

var contentTypes = fixture.Entry{Name: "[Content_Types].xml", Data: []byte(`<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types"/>`)}

func TestMetafileThumbnails(t *testing.T) {
	red := color.NRGBA{255, 0, 0, 255}
	e := fixture.NewEMF(200, 100)
	e.Select(e.Brush(red, -1))
	e.Select(fixture.Stock(8))
	e.Rectangle(0, 0, 200, 100)
	w := fixture.NewWMF(200, 200, true)
	w.Select(w.Brush(red))
	w.Select(w.NullPen())
	w.Rectangle(0, 0, 200, 200)
	tests := []struct {
		name string
		data []byte
		size image.Point
	}{
		{"docProps/thumbnail.emf", e.Bytes(), image.Pt(256, 128)},
		{"docProps/thumbnail.wmf", w.Bytes(), image.Pt(256, 256)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			buf := fixture.Zip(contentTypes, fixture.Entry{Name: tt.name, Data: tt.data})
			d, err := Open(t.Context(), bytes.NewReader(buf), ivy.Config{})
			if err != nil {
				t.Fatal(err)
			}
			var previews []ivy.Preview
			for p, err := range d.Previews(t.Context()) {
				if err != nil {
					t.Fatal(err)
				}
				previews = append(previews, p)
			}
			if len(previews) != 1 || previews[0].Size != tt.size {
				t.Fatalf("expected 1 preview of %v, got: %+v", tt.size, previews)
			}
			img, err := previews[0].Decode(t.Context())
			if err != nil {
				t.Fatal(err)
			}
			if got := color.NRGBAModel.Convert(img.At(tt.size.X/2, tt.size.Y/2)); got != red {
				t.Errorf("expected red, got: %v", got)
			}
		})
	}
}

func TestThumbnailOrder(t *testing.T) {
	buf := fixture.Zip(contentTypes,
		fixture.Entry{Name: "docProps/thumbnail.emf", Data: []byte("not a metafile")},
		fixture.Entry{Name: "docProps/thumbnail.png", Data: fixture.PNG(10, 10, color.White)},
	)
	d, err := Open(t.Context(), bytes.NewReader(buf), ivy.Config{})
	if err != nil {
		t.Fatal(err)
	}
	n := 0
	for p, err := range d.Previews(t.Context()) {
		if err != nil {
			t.Fatal(err)
		}
		if p.Size != image.Pt(10, 10) {
			t.Errorf("expected the png of 10x10, got: %v", p.Size)
		}
		n++
	}
	if n != 1 {
		t.Errorf("expected 1 preview without the damaged metafile, got: %d", n)
	}
}

func TestDescribe(t *testing.T) {
	core := `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<cp:coreProperties xmlns:cp="http://schemas.openxmlformats.org/package/2006/metadata/core-properties" xmlns:dc="http://purl.org/dc/elements/1.1/" xmlns:dcterms="http://purl.org/dc/terms/" xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance">
<dc:title>A drawing</dc:title><dc:creator>Ann</dc:creator><dc:subject>Plans</dc:subject><dc:language>en-GB</dc:language>
<dcterms:created xsi:type="dcterms:W3CDTF">2020-01-02T03:04:05Z</dcterms:created>
<dcterms:modified xsi:type="dcterms:W3CDTF">2021-01-02T03:04:05Z</dcterms:modified>
</cp:coreProperties>`
	rels := `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
<Relationship Id="rId1" Type="http://schemas.openxmlformats.org/package/2006/relationships/metadata/core-properties" Target="meta/core.xml"/>
</Relationships>`
	buf := fixture.Zip(contentTypes,
		fixture.Entry{Name: "_rels/.rels", Data: []byte(rels)},
		fixture.Entry{Name: "meta/core.xml", Data: []byte(core)},
	)
	d, err := Open(t.Context(), bytes.NewReader(buf), ivy.Config{})
	if err != nil {
		t.Fatal(err)
	}
	info, err := d.Describe(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	want := ivy.Info{
		Title:    "A drawing",
		Authors:  []string{"Ann"},
		Subject:  "Plans",
		Language: "en-GB",
		Created:  time.Date(2020, 1, 2, 3, 4, 5, 0, time.UTC),
		Modified: time.Date(2021, 1, 2, 3, 4, 5, 0, time.UTC),
	}
	if info.Title != want.Title || len(info.Authors) != 1 || info.Authors[0] != "Ann" || info.Subject != want.Subject ||
		info.Language != want.Language || !info.Created.Equal(want.Created) || !info.Modified.Equal(want.Modified) {
		t.Errorf("expected %+v, got: %+v", want, info)
	}
	// a package without core properties says nothing
	d, err = Open(t.Context(), bytes.NewReader(fixture.Zip(contentTypes)), ivy.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if info, err := d.Describe(t.Context()); err != nil || info.Title != "" {
		t.Errorf("expected nothing, got: %+v, %v", info, err)
	}
}
