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

// blueEMF returns an EMF of 300 by 150 pixels at 100 to the inch, filled
// with blue.
func blueEMF() []byte {
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
		fixture.Entry{Name: "docProps/thumbnail.emf", Data: blueEMF()},
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
	f.Add(fuzzDrawing())
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
				break
			}
			if _, err := p.Decode(t.Context()); err != nil && strings.Contains(err.Error(), "panic") {
				t.Fatal(err)
			}
		}
		n, err := d.PageCount(t.Context())
		if err != nil || n == 0 {
			if err != nil && strings.Contains(err.Error(), "panic") {
				t.Fatal(err)
			}
			return
		}
		p, err := d.Page(t.Context(), 0)
		if err != nil {
			if strings.Contains(err.Error(), "panic") {
				t.Fatal(err)
			}
			return
		}
		// a small resolution keeps the fuzz fast
		if _, err := ivy.RenderImage(t.Context(), p, 6, color.White); err != nil && strings.Contains(err.Error(), "panic") {
			t.Fatal(err)
		}
	})
}

// fuzzDrawing returns a drawing of each kind of geometry, with a master, a
// style, a theme, a group, text and a background page, for the fuzz test.
func fuzzDrawing() []byte {
	geom := `<Section N="Geometry" IX="0">` +
		`<Row T="MoveTo" IX="1"><Cell N="X" V="0"/><Cell N="Y" V="0"/></Row>` +
		`<Row T="ArcTo" IX="2"><Cell N="X" V="1"/><Cell N="Y" V="0"/><Cell N="A" V="0.2"/></Row>` +
		`<Row T="EllipticalArcTo" IX="3"><Cell N="X" V="1"/><Cell N="Y" V="1"/><Cell N="A" V="1.2"/><Cell N="B" V="0.5"/><Cell N="C" V="0.3"/><Cell N="D" V="1.5"/></Row>` +
		`<Row T="RelCubBezTo" IX="4"><Cell N="X" V="0.5"/><Cell N="Y" V="1"/><Cell N="A" V="0.8"/><Cell N="B" V="1.2"/><Cell N="C" V="0.6"/><Cell N="D" V="1.2"/></Row>` +
		`<Row T="RelQuadBezTo" IX="5"><Cell N="X" V="0"/><Cell N="Y" V="0.5"/><Cell N="A" V="0.2"/><Cell N="B" V="0.9"/></Row>` +
		`<Row T="PolylineTo" IX="6"><Cell N="X" V="0"/><Cell N="Y" V="0"/><Cell N="A" V="0" F="POLYLINE(0, 0, 0.1,0.4, 0.2,0.2)"/></Row>` +
		`<Row T="NURBSTo" IX="7"><Cell N="X" V="1"/><Cell N="Y" V="0"/><Cell N="A" V="0"/><Cell N="B" V="1"/><Cell N="C" V="0"/><Cell N="D" V="1"/><Cell N="E" V="0" F="NURBS(1, 3, 0, 0, 0, 1, 0, 1, 1, 1, 0, 1)"/></Row>` +
		`<Row T="SplineStart" IX="8"><Cell N="X" V="1"/><Cell N="Y" V="1"/><Cell N="A" V="0"/><Cell N="B" V="0"/><Cell N="C" V="1"/><Cell N="D" V="2"/></Row>` +
		`<Row T="SplineKnot" IX="9"><Cell N="X" V="0"/><Cell N="Y" V="1"/><Cell N="A" V="0.5"/></Row>` +
		`</Section><Section N="Geometry" IX="1"><Cell N="NoFill" V="1"/>` +
		`<Row T="Ellipse" IX="1"><Cell N="X" V="0.5"/><Cell N="Y" V="0.5"/><Cell N="A" V="1"/><Cell N="B" V="0.5"/><Cell N="C" V="0.5"/><Cell N="D" V="1"/></Row>` +
		`<Row T="InfiniteLine" IX="2"><Cell N="X" V="0"/><Cell N="Y" V="0"/><Cell N="A" V="1"/><Cell N="B" V="1"/></Row></Section>`
	text := `<Section N="Character"><Row IX="0"><Cell N="Color" V="Themed"/><Cell N="Style" V="3"/></Row></Section>` +
		`<Section N="Paragraph"><Row IX="0"><Cell N="Bullet" V="1"/></Row></Section>` +
		`<Text><cp IX="0"/><pp IX="0"/>Text<fld IX="0">1</fld></Text>`
	shape := `<Shape ID="1" Type="Shape" FillStyle="1"><Cell N="PinX" V="2"/><Cell N="PinY" V="2"/><Cell N="Width" V="1"/><Cell N="Height" V="1"/>` +
		`<Cell N="FillPattern" V="30"/><Cell N="EndArrow" V="4"/><Cell N="ShdwPattern" V="1"/>` + geom + text + `</Shape>`
	group := `<Shape ID="2" Type="Group"><Cell N="PinX" V="1"/><Cell N="PinY" V="1"/><Cell N="Width" V="1"/><Cell N="Height" V="1"/><Cell N="Angle" V="0.5"/><Cell N="FlipY" V="1"/>` +
		`<Shapes><Shape ID="3" Type="Shape" Master="1"/></Shapes></Shape>`
	v := fixture.VSDX{
		Styles:  `<StyleSheet ID="1" FillStyle="0"><Cell N="FillGradientEnabled" V="1"/><Section N="FillGradient"><Row IX="0"><Cell N="GradientStopColor" V="2"/></Row></Section></StyleSheet>`,
		Masters: []fixture.VSDXMaster{{ID: 1, Shapes: `<Shape ID="5" Type="Shape"><Cell N="Width" V="1"/><Cell N="Height" V="1"/>` + fixture.VisioRect(0, 1, 1) + `</Shape>`}},
		Pages: []fixture.VSDXPage{
			{ID: 0, Width: 4, Height: 4, BackPage: 1, Shapes: shape + group},
			{ID: 1, Width: 4, Height: 4, Background: true, Shapes: `<Shape ID="1" Type="Shape" Master="1"><Cell N="PinX" V="1"/><Cell N="PinY" V="3"/></Shape>`},
		},
	}
	return v.Bytes()
}
