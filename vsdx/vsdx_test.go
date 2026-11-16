package vsdx

import (
	"bytes"
	"encoding/binary"
	"errors"
	"image"
	"image/color"
	"strings"
	"testing"

	"github.com/xo/ivy"
	"github.com/xo/ivy/internal/fixture"
)

var blue = color.NRGBA{0, 0, 255, 255}

// drawing returns an EMF of 300 by 150 pixels at 100 to the inch, filled
// with blue.
func drawing() []byte {
	e := fixture.NewEMF(300, 150)
	e.Select(e.Brush(blue, -1))
	e.Select(fixture.Stock(8))
	e.Rectangle(0, 0, 300, 150)
	return e.Bytes()
}

// vsdxFile returns a package of Office Open XML with a thumbnail in EMF
// and core properties.
func vsdxFile() []byte {
	core := `<cp:coreProperties xmlns:cp="http://schemas.openxmlformats.org/package/2006/metadata/core-properties" xmlns:dc="http://purl.org/dc/elements/1.1/"><dc:title>Floor plan</dc:title><dc:creator>Ann</dc:creator></cp:coreProperties>`
	return fixture.Zip(
		fixture.Entry{Name: "[Content_Types].xml", Data: []byte(`<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types"/>`)},
		fixture.Entry{Name: "docProps/core.xml", Data: []byte(core)},
		fixture.Entry{Name: "docProps/thumbnail.emf", Data: drawing()},
		fixture.Entry{Name: "visio/document.xml", Data: []byte("<VisioDocument/>")},
	)
}

// vsdFile returns a binary Visio file with a thumbnail of CF_METAFILEPICT.
func vsdFile() []byte {
	w := fixture.NewWMF(200, 100, false)
	w.Select(w.Brush(blue))
	w.Select(w.NullPen())
	w.Rectangle(0, 0, 200, 100)
	pict := binary.LittleEndian.AppendUint16(nil, 8)
	pict = binary.LittleEndian.AppendUint16(pict, 5080)
	pict = binary.LittleEndian.AppendUint16(pict, 2540)
	pict = binary.LittleEndian.AppendUint16(pict, 0)
	sum := fixture.Summary{
		Title:           []byte("Old plan"),
		ThumbnailTag:    -1,
		ThumbnailFormat: fixture.ClipMetafilePict,
		Thumbnail:       append(pict, w.Bytes()...),
	}
	return fixture.CFB(
		fixture.CFBEntry{Name: "\x05SummaryInformation", Data: sum.Bytes()},
		fixture.CFBEntry{Name: "VisioDocument", Data: make([]byte, 100)},
	)
}

func TestOpen(t *testing.T) {
	tests := []struct {
		name  string
		buf   []byte
		title string
		size  image.Point
	}{
		{"vsdx", vsdxFile(), "Floor plan", image.Pt(288, 144)},
		{"vsd", vsdFile(), "Old plan", image.Pt(256, 128)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d, err := Format.Open(t.Context(), bytes.NewReader(tt.buf), ivy.Config{})
			if err != nil {
				t.Fatal(err)
			}
			if _, ok := d.(ivy.Pager); ok {
				t.Error("expected a document without pages")
			}
			info, err := d.(ivy.Describer).Describe(t.Context())
			if err != nil || info.Title != tt.title {
				t.Errorf("expected the title %q, got: %q, %v", tt.title, info.Title, err)
			}
			img, err := ivy.BestPreview(t.Context(), d, image.Point{})
			if err != nil {
				t.Fatal(err)
			}
			if img.Bounds().Size() != tt.size {
				t.Errorf("expected %v, got: %v", tt.size, img.Bounds().Size())
			}
			if got := color.NRGBAModel.Convert(img.At(tt.size.X/2, tt.size.Y/2)); got != blue {
				t.Errorf("expected blue, got: %v", got)
			}
		})
	}
}

func TestOpenErrors(t *testing.T) {
	tests := []struct {
		name string
		buf  []byte
		want error
	}{
		{"not a file of visio", []byte("plain text, not a drawing"), ivy.ErrFormat},
		{"other compound file", fixture.CFB(fixture.CFBEntry{Name: "WordDocument", Data: []byte("x")}), ivy.ErrFormat},
		{"zip without content types", fixture.Zip(fixture.Entry{Name: "a", Data: []byte("a")}), ivy.ErrFormat},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := Open(t.Context(), bytes.NewReader(tt.buf)); !errors.Is(err, tt.want) {
				t.Errorf("expected %v, got: %v", tt.want, err)
			}
		})
	}
}

func FuzzOpen(f *testing.F) {
	f.Add(vsdxFile())
	f.Add(vsdFile())
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
