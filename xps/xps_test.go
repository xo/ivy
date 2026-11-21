package xps

import (
	"bytes"
	"errors"
	"image"
	"image/color"
	"math"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"golang.org/x/image/font/gofont/goregular"

	"github.com/xo/ivy"
	"github.com/xo/ivy/internal/display"
	"github.com/xo/ivy/internal/fixture"
	"github.com/xo/okage"
	"github.com/xo/okage/roxmltree"
)

func open(t *testing.T, name string) *Document {
	t.Helper()
	buf, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatal(err)
	}
	d, err := Open(t.Context(), bytes.NewReader(buf))
	if err != nil {
		t.Fatal(err)
	}
	return d
}

func TestTestdata(t *testing.T) {
	tests := []struct {
		name  string
		pages int
	}{
		{"example.xps", 1},
		{"xps-document-example.xps", 6},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			d := open(t, test.name)
			if n, _ := d.PageCount(t.Context()); n != test.pages {
				t.Fatalf("expected %d pages, got: %d", test.pages, n)
			}
			for p, err := range ivy.Pages(t.Context(), d) {
				if err != nil {
					t.Fatal(err)
				}
				img, err := ivy.RenderImage(t.Context(), p, 36, color.White)
				if err != nil {
					t.Fatal(err)
				}
				// an A4 page at 36 dots per inch
				if size := img.Bounds().Size(); size.X != 298 || size.Y < 420 || size.Y > 422 {
					t.Errorf("expected an A4 page, got: %v", size)
				}
				if ink(img) < 1000 {
					t.Error("expected a page with ink")
				}
			}
			preview, err := ivy.BestPreview(t.Context(), d, image.Point{})
			if err != nil || preview.Bounds().Dx() == 0 {
				t.Errorf("expected the thumbnail, got: %v", err)
			}
		})
	}
	info, err := open(t, "example.xps").Describe(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if len(info.Authors) != 1 || info.Authors[0] != "Vb1" || info.Created.Year() != 2014 {
		t.Errorf("expected the core properties, got: %+v", info)
	}
}

// ink counts the pixels of img that are not white.
func ink(img image.Image) int {
	n := 0
	b := img.Bounds()
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			r, g, bl, _ := img.At(x, y).RGBA()
			if r < 0xf000 || g < 0xf000 || bl < 0xf000 {
				n++
			}
		}
	}
	return n
}

// pkgFile builds an XPS package with one page, of 96 by 96 units, that holds
// body, and the parts of extra.
func pkgFile(body string, extra ...fixture.Entry) []byte {
	entries := append([]fixture.Entry{
		{Name: "_rels/.rels", Data: []byte(`<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">` +
			`<Relationship Type="http://schemas.microsoft.com/xps/2005/06/fixedrepresentation" Target="/seq.fdseq" Id="R1"/></Relationships>`)},
		{Name: "seq.fdseq", Data: []byte(`<FixedDocumentSequence xmlns="http://schemas.microsoft.com/xps/2005/06">` +
			`<DocumentReference Source="Documents/doc.fdoc"/></FixedDocumentSequence>`)},
		{Name: "Documents/doc.fdoc", Data: []byte(`<FixedDocument xmlns="http://schemas.microsoft.com/xps/2005/06">` +
			`<PageContent Source="Pages/1.fpage"/></FixedDocument>`)},
		{Name: "Documents/Pages/1.fpage", Data: []byte(`<FixedPage xmlns="http://schemas.microsoft.com/xps/2005/06" ` +
			`xmlns:x="http://schemas.microsoft.com/xps/2005/06/resourcedictionary-key" Width="96" Height="96">` + body + `</FixedPage>`)},
	}, extra...)
	return fixture.Zip(entries...)
}

// render renders the page of an XPS package at 96 dots per inch, so that a
// unit of XPS is a pixel, over white.
func render(t *testing.T, buf []byte) (image.Image, error) {
	t.Helper()
	d, err := Open(t.Context(), bytes.NewReader(buf))
	if err != nil {
		t.Fatal(err)
	}
	p, err := d.Page(t.Context(), 0)
	if err != nil {
		t.Fatal(err)
	}
	return ivy.RenderImage(t.Context(), p, 96, color.White)
}

// rgb returns the color of img at (x, y) in 8 bits for each channel.
func rgb(img image.Image, x, y int) [3]uint8 {
	r, g, b, _ := img.At(x, y).RGBA()
	return [3]uint8{uint8(r >> 8), uint8(g >> 8), uint8(b >> 8)}
}

func TestPathsAndCanvases(t *testing.T) {
	body := `<FixedPage.Resources><ResourceDictionary>
		<SolidColorBrush x:Key="blue" Color="#0000FF"/>
		<PathGeometry x:Key="square" Figures="M 0,0 L 10,0 10,10 0,10 Z"/>
	</ResourceDictionary></FixedPage.Resources>
	<Path Data="M 0,0 H 48 V 48 H 0 Z" Fill="#FFFF0000"/>
	<Canvas RenderTransform="1,0,0,1,48,0">
		<Path Data="{StaticResource square}" Fill="{StaticResource blue}" RenderTransform="4,0,0,4,0,0"/>
	</Canvas>
	<Canvas Clip="M 0,48 L 24,48 24,96 0,96 Z">
		<Path Fill="#00FF00"><Path.Data><PathGeometry><PathFigure StartPoint="0,48" IsClosed="true">
			<PolyLineSegment Points="96,48 96,96 0,96"/></PathFigure></PathGeometry></Path.Data></Path>
	</Canvas>
	<Canvas Opacity="0.5"><Path Data="M 60,60 L 90,60 90,90 60,90 Z" Fill="#000000"/></Canvas>`
	img, err := render(t, pkgFile(body))
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		x, y int
		want [3]uint8
	}{
		{10, 10, [3]uint8{255, 0, 0}},
		{60, 20, [3]uint8{0, 0, 255}},
		{60, 45, [3]uint8{255, 255, 255}},
		{10, 70, [3]uint8{0, 255, 0}},
		{40, 70, [3]uint8{255, 255, 255}},
	}
	for _, test := range tests {
		if got := rgb(img, test.x, test.y); got != test.want {
			t.Errorf("at (%d, %d): expected %v, got: %v", test.x, test.y, test.want, got)
		}
	}
	if got := rgb(img, 75, 75); got[0] < 120 || got[0] > 135 {
		t.Errorf("expected gray at half opacity, got: %v", got)
	}
}

func TestGradientAndStroke(t *testing.T) {
	body := `<Path Data="M 0,0 H 96 V 48 H 0 Z">
		<Path.Fill><LinearGradientBrush MappingMode="Absolute" StartPoint="0,0" EndPoint="96,0">
			<LinearGradientBrush.GradientStops>
				<GradientStop Offset="1" Color="#0000FF"/>
				<GradientStop Offset="0" Color="#FF0000"/>
			</LinearGradientBrush.GradientStops>
		</LinearGradientBrush></Path.Fill></Path>
	<Path Data="M 10,70 L 86,70" Stroke="#000000" StrokeThickness="8"/>`
	img, err := render(t, pkgFile(body))
	if err != nil {
		t.Fatal(err)
	}
	if left, right := rgb(img, 2, 20), rgb(img, 93, 20); left[0] < 240 || right[2] < 240 {
		t.Errorf("expected red to blue, got: %v %v", left, right)
	}
	if got := rgb(img, 48, 70); got != ([3]uint8{0, 0, 0}) {
		t.Errorf("expected the stroke, got: %v", got)
	}
	if got := rgb(img, 48, 80); got != ([3]uint8{255, 255, 255}) {
		t.Errorf("expected white under the stroke, got: %v", got)
	}
}

func TestGlyphsFromText(t *testing.T) {
	body := `<Glyphs FontUri="/font.ttf" FontRenderingEmSize="40" OriginX="8" OriginY="60" UnicodeString="II" Fill="#000000"/>`
	img, err := render(t, pkgFile(body, fixture.Entry{Name: "font.ttf", Data: goregular.TTF}))
	if err != nil {
		t.Fatal(err)
	}
	if ink(img) < 100 {
		t.Error("expected the ink of two glyphs")
	}
}

func TestPartial(t *testing.T) {
	body := `<Path Data="M 0,0 H 96 V 96 H 0 Z" Fill="#FF0000"/>
		<Path Data="M 0,0 H 10 V 10 Z"><Path.Fill><VisualBrush/></Path.Fill></Path>
		<Glyphs FontUri="/missing.ttf" FontRenderingEmSize="10" UnicodeString="a" Fill="#000000"/>`
	img, err := render(t, pkgFile(body))
	if _, ok := errors.AsType[*ivy.PartialError](err); !ok {
		t.Fatalf("expected a *PartialError, got: %v", err)
	}
	if got := rgb(img, 50, 50); got != ([3]uint8{255, 0, 0}) {
		t.Errorf("expected the part that draws, got: %v", got)
	}
}

func TestOpenErrors(t *testing.T) {
	if _, err := Open(t.Context(), bytes.NewReader([]byte("not a zip"))); !errors.Is(err, ivy.ErrFormat) {
		t.Errorf("expected ErrFormat, got: %v", err)
	}
	empty := fixture.Zip(fixture.Entry{Name: "a.txt"})
	if _, err := Open(t.Context(), bytes.NewReader(empty)); !errors.Is(err, ivy.ErrFormat) {
		t.Errorf("expected ErrFormat, got: %v", err)
	}
	d, err := Open(t.Context(), bytes.NewReader(pkgFile("")))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := d.Page(t.Context(), 1); !errors.Is(err, ivy.ErrPageRange) {
		t.Errorf("expected ErrPageRange, got: %v", err)
	}
	big := pkgFile(strings.Repeat(`<Path Data="M 0,0 L 1,1"/>`, 50))
	d, err = Open(t.Context(), bytes.NewReader(big), WithConfig(ivy.Config{Limits: ivy.Limits{MaxObjects: 20}}))
	if err != nil {
		t.Fatal(err)
	}
	p, err := d.Page(t.Context(), 0)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ivy.RenderImage(t.Context(), p, 72, nil); !errors.Is(err, ivy.ErrLimit) {
		t.Errorf("expected ErrLimit, got: %v", err)
	}
}

func TestParseAbbreviated(t *testing.T) {
	tests := []struct {
		s    string
		rule okage.FillRule
		want [4]float32
	}{
		{"M 0,0 L 10,0 10,10 Z", okage.FillRuleEvenOdd, [4]float32{0, 0, 10, 10}},
		{"F1 M1 2 h 5 v 5 h -5 z", okage.FillRuleWinding, [4]float32{1, 2, 6, 7}},
		{"M0,0 C 0,10 10,10 10,0 S 20,-10 20,0", okage.FillRuleEvenOdd, [4]float32{0, -10, 20, 10}},
		{"M0,0 Q 5,10 10,0", okage.FillRuleEvenOdd, [4]float32{0, 0, 10, 10}},
		{"M0,0 m 1,1 l 1e1,0", okage.FillRuleEvenOdd, [4]float32{1, 1, 11, 1}},
	}
	for _, test := range tests {
		g, err := parseAbbreviated(test.s)
		if err != nil {
			t.Fatalf("%q: %v", test.s, err)
		}
		if g.rule != test.rule {
			t.Errorf("%q: expected rule %v, got: %v", test.s, test.rule, g.rule)
		}
		b := g.path.Bounds()
		got := [4]float32{b.Left(), b.Top(), b.Right(), b.Bottom()}
		if got != test.want {
			t.Errorf("%q: expected bounds %v, got: %v", test.s, test.want, got)
		}
	}
	for _, bad := range []string{"0,0 L 1,1", "M 0", "M 0,0 X 1,1", "M a,b"} {
		if _, err := parseAbbreviated(bad); !errors.Is(err, ivy.ErrFormat) {
			t.Errorf("%q: expected ErrFormat, got: %v", bad, err)
		}
	}
}

func TestArc(t *testing.T) {
	// a half circle of radius 10 from (0, 0) to (20, 0), above the line
	g, err := parseAbbreviated("M 0,0 A 10,10 0 0 1 20,0")
	if err != nil {
		t.Fatal(err)
	}
	b := g.path.Bounds()
	if math.Abs(float64(b.Top()+10)) > 0.1 || b.Bottom() > 0.01 || math.Abs(float64(b.Right()-20)) > 0.01 {
		t.Errorf("expected a half circle above the line, got: %v", b)
	}
	// a large arc of an ellipse that is too small grows to reach its end
	g, err = parseAbbreviated("M 0,0 A 1,1 0 1 0 20,0")
	if err != nil {
		t.Fatal(err)
	}
	if b := g.path.Bounds(); b.Bottom() < 9.9 {
		t.Errorf("expected a half circle below the line, got: %v", b)
	}
}

func TestParseColor(t *testing.T) {
	tests := map[string][4]uint8{
		"#FF0000":                       {255, 0, 0, 255},
		"#80FF0000":                     {255, 0, 0, 128},
		"sc#1,0,0":                      {255, 0, 0, 255},
		"sc#0.5,0,0,1":                  {0, 0, 255, 128},
		"ContextColor /p.icc 1,0,1,0":   {0, 255, 0, 255},
		"ContextColor /p.icc 1,0,0,0,1": {0, 0, 0, 255},
		"ContextColor /p.icc 1,0,0,0,0": {255, 255, 255, 255},
		"ContextColor /p.icc 0.5,0.5":   {128, 128, 128, 128},
	}
	for s, want := range tests {
		c, err := parseColor(s)
		if err != nil {
			t.Fatalf("%q: %v", s, err)
		}
		u := c.ToColorU8()
		got := [4]uint8{u.Red(), u.Green(), u.Blue(), u.Alpha()}
		for i := range got {
			if d := int(got[i]) - int(want[i]); d < -1 || d > 1 {
				t.Errorf("%q: expected %v, got: %v", s, want, got)
				break
			}
		}
	}
	for _, bad := range []string{"red", "#12", "sc#1", "ContextColor"} {
		if _, err := parseColor(bad); err == nil {
			t.Errorf("%q: expected an error", bad)
		}
	}
}

// TestGlyphRunText checks the text of a run and the clusters of its glyphs
// from the cluster map of the Indices (D58).
func TestGlyphRunText(t *testing.T) {
	d, err := Open(t.Context(), bytes.NewReader(pkgFile("", fixture.Entry{Name: "font.ttf", Data: goregular.TTF})))
	if err != nil {
		t.Fatal(err)
	}
	r := &renderer{ctx: t.Context(), doc: d, part: "/Documents/Pages/1.fpage", dev: &display.List{}}
	tests := []struct {
		indices, text string
		want          [][2]int
	}{
		{"", "aé", [][2]int{{0, 1}, {1, 3}}},
		// a cluster of two characters and one glyph, then a glyph from the
		// text
		{"(2:1)5,100;", "fié", [][2]int{{0, 2}, {2, 4}}},
		// one character and two glyphs
		{"(1:2)5;6;7", "éb", [][2]int{{0, 2}, {0, 2}, {2, 3}}},
		// a map past the end of the text
		{"(9:1)5;6", "ab", [][2]int{{0, 2}, {2, 2}}},
	}
	for _, test := range tests {
		n := mustParse(t, `<Glyphs FontUri="/font.ttf" FontRenderingEmSize="10" OriginX="0" OriginY="0" Indices="`+
			test.indices+`" UnicodeString="`+test.text+`"/>`)
		run, err := r.glyphRun(n)
		if err != nil {
			t.Fatal(err)
		}
		if run.Text != test.text || !run.TextRanges() {
			t.Errorf("%q: expected the text %q with valid ranges, got %q", test.indices, test.text, run.Text)
		}
		var got [][2]int
		for _, g := range run.Glyphs {
			got = append(got, [2]int{g.Start, g.End})
		}
		if !slices.Equal(got, test.want) {
			t.Errorf("%q: expected the ranges %v, got %v", test.indices, test.want, got)
		}
	}
}

func TestGlyphRun(t *testing.T) {
	d, err := Open(t.Context(), bytes.NewReader(pkgFile("", fixture.Entry{Name: "font.ttf", Data: goregular.TTF})))
	if err != nil {
		t.Fatal(err)
	}
	r := &renderer{ctx: t.Context(), doc: d, part: "/Documents/Pages/1.fpage", dev: &display.List{}}
	tests := []struct {
		indices, text string
		bidi          string
		want          []float32
	}{
		// explicit advances in hundredths of an em of 10 units
		{"1,50;2,50;3", "", "0", []float32{0, 5, 10}},
		// glyphs from the text, with the advances of the font
		{"", "ab", "0", nil},
		// a cluster of two characters and one glyph, then a glyph from the
		// text
		{"(2:1)5,100;", "fia", "0", []float32{0, 10}},
		// right to left: each glyph is left of the pen
		{"1,100;2,100", "", "1", []float32{-10, -20}},
	}
	for _, test := range tests {
		n := mustParse(t, `<Glyphs FontUri="/font.ttf" FontRenderingEmSize="10" OriginX="0" OriginY="0" Indices="`+
			test.indices+`" UnicodeString="`+test.text+`" BidiLevel="`+test.bidi+`"/>`)
		run, err := r.glyphRun(n)
		if err != nil {
			t.Fatal(err)
		}
		if test.want == nil {
			if len(run.Glyphs) != len(test.text) {
				t.Errorf("%q: expected %d glyphs, got: %d", test.text, len(test.text), len(run.Glyphs))
			}
			continue
		}
		var xs []float32
		for _, g := range run.Glyphs {
			xs = append(xs, float32(math.Round(float64(g.X)*100)/100))
		}
		if len(xs) != len(test.want) {
			t.Errorf("%q: expected %v, got: %v", test.indices, test.want, xs)
			continue
		}
		for i := range xs {
			if xs[i] != test.want[i] {
				t.Errorf("%q: expected %v, got: %v", test.indices, test.want, xs)
				break
			}
		}
	}
}

func TestDeobfuscate(t *testing.T) {
	name := "/Resources/0A1B2C3D-4E5F-6071-8293-A4B5C6D7E8F9.odttf"
	data := bytes.Clone(goregular.TTF)
	// obfuscation is its own inverse
	if err := deobfuscate(data, name); err != nil {
		t.Fatal(err)
	}
	if bytes.Equal(data[:32], goregular.TTF[:32]) {
		t.Fatal("expected the first 32 bytes to change")
	}
	// the first byte is XORed with the last byte of the guid string
	if data[0] != goregular.TTF[0]^0xf9 {
		t.Errorf("expected the key in the order of a guid, got: %#x", data[0]^goregular.TTF[0])
	}
	if err := deobfuscate(data, name); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(data, goregular.TTF) {
		t.Error("expected the font back")
	}
	if err := deobfuscate(data, "/Resources/font.odttf"); !errors.Is(err, ivy.ErrFormat) {
		t.Errorf("expected ErrFormat, got: %v", err)
	}
}

func TestDecodeText(t *testing.T) {
	want := `<a b="é"/>`
	utf16le := make([]byte, 0, 2+2*len(want))
	utf16le = append(utf16le, 0xff, 0xfe)
	for _, r := range want {
		utf16le = append(utf16le, byte(r), byte(r>>8))
	}
	for name, buf := range map[string][]byte{
		"utf-8":            []byte(want),
		"utf-8 with a bom": append([]byte{0xef, 0xbb, 0xbf}, want...),
		"utf-16le":         utf16le,
	} {
		if got := decodeText(buf); got != want {
			t.Errorf("%s: expected %q, got: %q", name, want, got)
		}
	}
}

func TestResolve(t *testing.T) {
	tests := []struct{ base, ref, want string }{
		{"/Documents/1/FixedDoc.fdoc", "Pages/1.fpage", "/Documents/1/Pages/1.fpage"},
		{"/Documents/1/Pages/1.fpage", "../../../Resources/f.odttf", "/Resources/f.odttf"},
		{"/a/b.fpage", "/Resources/x.png#0", "/Resources/x.png"},
	}
	for _, test := range tests {
		if got := resolve(test.base, test.ref); got != test.want {
			t.Errorf("%s + %s: expected %s, got: %s", test.base, test.ref, test.want, got)
		}
	}
}

func FuzzOpen(f *testing.F) {
	for _, name := range []string{"example.xps"} {
		buf, err := os.ReadFile(filepath.Join("testdata", name))
		if err != nil {
			f.Fatal(err)
		}
		f.Add(buf)
	}
	f.Add(pkgFile(`<Path Data="M 0,0 A 5,5 0 1 1 9,9 Z" Fill="#FF0000"/>`))
	f.Add(pkgFile(`<Canvas OpacityMask="#80000000"><Path Data="M 0,0 H 9 V 9 Z" Fill="#FF0000" Stroke="#00FF00">` +
		`<Path.OpacityMask><LinearGradientBrush MappingMode="Absolute" StartPoint="0,0" EndPoint="9,0">` +
		`<LinearGradientBrush.GradientStops><GradientStop Offset="0" Color="#FF000000"/></LinearGradientBrush.GradientStops>` +
		`</LinearGradientBrush></Path.OpacityMask></Path></Canvas>`))
	f.Add(pkgFile(`<FixedPage.Resources><ResourceDictionary><VisualBrush x:Key="v" Viewbox="0,0,4,4" Viewport="0,0,4,4" ` +
		`TileMode="FlipXY"><VisualBrush.Visual><Path Data="M 0,0 H 2 V 2 Z" Fill="{StaticResource v}"/></VisualBrush.Visual>` +
		`</VisualBrush></ResourceDictionary></FixedPage.Resources>` +
		`<Path Data="M 0,0 H 9 V 9 Z" Fill="{StaticResource v}" Stroke="{StaticResource v}"/>`))
	f.Add(linkFile(2))
	cfg := ivy.Config{Limits: ivy.Limits{MaxMemory: 4 << 20, MaxPixels: 1 << 20, MaxObjects: 1000, MaxDepth: 16}}
	f.Fuzz(func(t *testing.T, buf []byte) {
		d, err := Open(t.Context(), bytes.NewReader(buf), WithConfig(cfg))
		if err != nil {
			return
		}
		p, err := d.Page(t.Context(), 0)
		if err != nil {
			return
		}
		dst, _ := okage.NewPixmap(32, 32)
		_ = p.Render(t.Context(), dst, okage.TransformFromScale(0.05, 0.05))
		// a list takes the links, the anchors and the bookmarks
		_ = p.(display.Drawer).Draw(t.Context(), display.NewRecording(), 0.05)
	})
}

// mustParse parses an element for a test.
func mustParse(t *testing.T, s string) roxmltree.Node {
	t.Helper()
	doc, err := roxmltree.Parse(s)
	if err != nil {
		t.Fatal(err)
	}
	return doc.RootElement()
}
