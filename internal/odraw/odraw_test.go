package odraw

import (
	"bytes"
	"encoding/binary"
	"image/color"
	"image/png"
	"math"
	"slices"
	"testing"

	"github.com/xo/ivy"
	"github.com/xo/ivy/internal/drawingml"
	"github.com/xo/ivy/internal/fontdb"
)

// near reports whether a and b differ by less than 0.01.
func near(a, b float64) bool {
	return math.Abs(a-b) < 0.01
}

// record returns a record of Office Drawing of the type typ and the
// instance inst, with the body.
func record(typ, inst uint16, body []byte) []byte {
	b := binary.LittleEndian.AppendUint16(nil, inst<<4)
	b = binary.LittleEndian.AppendUint16(b, typ)
	b = binary.LittleEndian.AppendUint32(b, uint32(len(body)))
	return append(b, body...)
}

// container returns a container record of the type typ that holds the
// records.
func container(typ uint16, records ...[]byte) []byte {
	b := record(typ, 0, bytes.Join(records, nil))
	b[0] |= 0x0F
	return b
}

func TestArrayWithoutHeader(t *testing.T) {
	arr := le.AppendUint16(nil, 2)
	arr = le.AppendUint16(arr, 2)
	arr = le.AppendUint16(arr, 0xFFF0)
	arr = append(arr, 1, 0, 2, 0, 3, 0, 4, 0)
	for _, size := range []uint32{8, 14} {
		var fopt []byte
		fopt = le.AppendUint16(fopt, PropVertices|0x8000)
		fopt = le.AppendUint32(fopt, size)
		fopt = le.AppendUint16(fopt, PropFillColor)
		fopt = le.AppendUint32(fopt, 0x123456)
		fopt = append(fopt, arr...)
		var p Props
		p.Read(fopt, 2)
		if got := Array(p.Complex(PropVertices), 4); len(got) != 2 {
			t.Errorf("size %d: expected 2 vertices, got: %d", size, len(got))
		}
		if v, _ := p.Get(PropFillColor); v != 0x123456 {
			t.Errorf("size %d: expected the fill color 0x123456, got: %#x", size, v)
		}
	}
}

// dib1x1 returns a DIB of 1 by 1 pixel of 24 bits, red.
func dib1x1() []byte {
	dib := make([]byte, 40, 44)
	le.PutUint32(dib, 40)
	le.PutUint32(dib[4:], 1)
	le.PutUint32(dib[8:], 1)
	le.PutUint16(dib[12:], 1)
	le.PutUint16(dib[14:], 24)
	return append(dib, 0, 0, 255, 0)
}

func TestBlips(t *testing.T) {
	dib := dib1x1()
	rec := record(RecBlipDIB, 0x7A8, append(make([]byte, 17), dib...))
	data := Data(rec, 1<<20)
	if len(data) != 14+len(dib) || string(data[:2]) != "BM" || le.Uint32(data[10:]) != 54 {
		t.Fatalf("expected a BMP file, got: %q", data)
	}
	keyed, ok := ColorKey(t.Context(), data, 255, 0, 0, ivy.Limits{}.WithDefaults())
	if !ok {
		t.Fatal("expected the picture with a color key")
	}
	img, err := png.Decode(bytes.NewReader(keyed))
	if err != nil {
		t.Fatal(err)
	}
	if c := color.NRGBAModel.Convert(img.At(0, 0)).(color.NRGBA); c.A != 0 {
		t.Errorf("expected a transparent pixel, got: %v", c)
	}
	if Data(rec[:20], 1<<20) != nil {
		t.Error("expected no picture of a cut record")
	}
}

// TestBMPCore checks the offset of the pixels of a DIB with a
// BITMAPCOREHEADER, whose colors take 3 bytes.
func TestBMPCore(t *testing.T) {
	dib := make([]byte, 12, 22)
	le.PutUint32(dib, 12)
	le.PutUint16(dib[4:], 1)
	le.PutUint16(dib[6:], 1)
	le.PutUint16(dib[8:], 1)
	le.PutUint16(dib[10:], 1)
	dib = append(dib, 0, 0, 0, 255, 255, 255, 0x80, 0, 0, 0)
	if bmp := BMPFile(dib); le.Uint32(bmp[10:]) != 14+12+6 {
		t.Errorf("expected the pixels at %d, got: %d", 14+12+6, le.Uint32(bmp[10:]))
	}
}

// TestStore checks the store of pictures: an entry with its picture, an
// entry of another type that keeps the index of the next one, and an entry
// whose picture is in the stream of the pictures.
func TestStore(t *testing.T) {
	blip := record(RecBlipDIB, 0x7A8, append(make([]byte, 17), dib1x1()...))
	fbse := make([]byte, fbseSize)
	embedded := record(RecFBSE, 0, append(bytes.Clone(fbse), blip...))
	le.PutUint32(fbse[28:], 3)
	delayed := record(RecFBSE, 0, fbse)
	dgg := container(RecBStoreContainer, embedded, record(0xF00B, 0, nil), delayed)
	store := ReadStore(dgg, 100)
	if len(store) != 3 || !store[0].OK || store[1].OK || !store[2].OK {
		t.Fatalf("expected 3 entries, the second empty, got: %+v", store)
	}
	pics := NewPictures(store, append([]byte{1, 2, 3}, blip...), ivy.Config{})
	for _, pib := range []int{1, 3} {
		if pics.Get(t.Context(), pib, 0, false) == nil || pics.Data(pib) == nil {
			t.Errorf("expected the picture %d", pib)
		}
	}
	for _, pib := range []int{0, 2, 4} {
		if pics.Get(t.Context(), pib, 0, false) != nil {
			t.Errorf("expected no picture %d", pib)
		}
	}
}

// TestPathGradient checks that a gradient from the center starts at the
// focus with the back color, as page 4 of backgrounds.ppt shows it, and
// that the colors of the default shading mix in linear light along the
// curve of sigma.
func TestPathGradient(t *testing.T) {
	var s Style
	var p Props
	p.Set(PropFillType, FillShadeCenter)
	p.Set(PropFillColor, 0x404040)
	p.Set(PropFillBackColor, 0xFFFFFF)
	pt := s.Fill(Chain{&p})
	g := pt.Gradient
	if pt.Kind != drawingml.PaintGradient || g.Path != "rect" || len(g.Stops) < 9 || len(g.Stops)%2 == 0 {
		t.Fatalf("expected a path gradient, got %+v", pt)
	}
	if first, last := g.Stops[0].Color, g.Stops[len(g.Stops)-1].Color; first != drawingml.White || last.NRGBA().R != 0x40 {
		t.Errorf("expected white at the focus and gray at the edge, got %v and %v", first, last)
	}
	// the middle of white and of 40% gray in linear light is lighter than
	// the middle of their values
	mid := g.Stops[len(g.Stops)/2]
	if !near(mid.Pos, 0.5) || mid.Color.NRGBA().R < 0xb4 {
		t.Errorf("expected a light middle, got %+v", mid)
	}
	// sigma changes the color slowly near the stops: a quarter of the way
	// is less than a quarter of the mix in linear light
	quarter := g.Stops[len(g.Stops)/4]
	if lin := math.Pow(quarter.Color.R, 2.2); !near(quarter.Pos, 0.25) || lin < 0.85 || lin > 0.9 {
		t.Errorf("expected the curve of sigma, got %+v, %g in linear light", quarter, lin)
	}
}

// TestShadeType checks the flags of fillShadeType, the angle of a scaled
// shading, and the stops of fillShadeColors, which mix linearly. Every
// parameter of sigma draws the curve of the default.
func TestShadeType(t *testing.T) {
	var s Style
	tests := []struct {
		typ    uint32
		set    bool
		stops  int
		scaled bool
		colors bool
	}{
		{typ: 0, set: true, stops: 2},
		{typ: 0x40000001, set: true, stops: 9},
		{typ: 0x40000003, set: true, stops: 33},
		{set: false, stops: 33, scaled: true},
		{typ: 0x40000003, set: true, stops: 2, colors: true},
		// the parameter in the high 16 bits does not change the curve
		{typ: 0x10000003, set: true, stops: 33},
	}
	for i, test := range tests {
		var p Props
		p.Set(PropFillType, FillShade)
		if test.scaled {
			p.Set(PropFillType, FillShadeScale)
		}
		p.Set(PropFillColor, 0xFF0000)
		p.Set(PropFillBackColor, 0x0000FF)
		if test.set {
			p.Set(PropFillShadeType, test.typ)
		}
		if test.colors {
			arr := []byte{2, 0, 2, 0, 8, 0, 0xFF, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0xFF, 0, 0, 0, 0, 1, 0}
			p.Read(append([]byte{0x97, 0x81, byte(len(arr)), 0, 0, 0}, arr...), 1)
		}
		g := s.Fill(Chain{&p}).Gradient
		if len(g.Stops) != test.stops || g.Scaled != test.scaled {
			t.Errorf("%d: expected %d stops and scaled %t, got %d and %t", i, test.stops, test.scaled, len(g.Stops), g.Scaled)
		}
	}
	stops := func(typ uint32) []drawingml.Stop {
		var p Props
		p.Set(PropFillType, FillShade)
		p.Set(PropFillShadeType, typ)
		return s.Fill(Chain{&p}).Gradient.Stops
	}
	if !slices.Equal(stops(0x10000003), stops(0x40000003)) {
		t.Error("expected the curve of the default for every parameter of sigma")
	}
}

// TestColors checks the colors of a scheme, of RGB and of a system index
// that takes the fill color of the shape, darker.
func TestColors(t *testing.T) {
	s := Style{Scheme: func(i uint32) drawingml.RGBA {
		if i == 9 {
			return drawingml.White
		}
		return drawingml.Black
	}}
	var p Props
	p.Set(PropFillColor, 0x0000FF)
	c := Chain{&p}
	if got := s.Color(c, ColorSchemeIndex|9); got != drawingml.White {
		t.Errorf("expected the color 9 of the scheme, got: %v", got)
	}
	if got := s.Color(c, 0x00FF00).NRGBA(); got != (color.NRGBA{0, 255, 0, 255}) {
		t.Errorf("expected green, got: %v", got)
	}
	// the fill color, darkened by half
	if got := s.Color(c, ColorSysIndex|0x80<<16|0x01<<8|0xF0).NRGBA(); got.R < 0x7E || got.R > 0x81 || got.G != 0 {
		t.Errorf("expected a dark red, got: %v", got)
	}
}

// TestDrawing checks the groups of a drawing: the transform of a group
// maps its children onto the page, and a deleted shape does not draw.
func TestDrawing(t *testing.T) {
	fsp := func(spid, flags uint32) []byte {
		b := le.AppendUint32(nil, spid)
		return record(RecFSP, 1, le.AppendUint32(b, flags))
	}
	rect := func(typ uint16, l, t, r, b int32) []byte {
		var body []byte
		for _, v := range []int32{l, t, r, b} {
			body = le.AppendUint32(body, uint32(v))
		}
		return record(typ, 0, body)
	}
	patriarch := container(RecSpContainer, rect(RecFSPGR, 0, 0, 100, 100), fsp(1, FSPGroup|FSPPatriarch))
	group := container(RecSpgrContainer,
		container(RecSpContainer, rect(RecFSPGR, 0, 0, 10, 10), fsp(2, FSPGroup), record(RecClientAnchor, 0, []byte{1})),
		container(RecSpContainer, fsp(3, FSPChild), rect(RecChildAnchor, 5, 5, 10, 10)),
		container(RecSpContainer, fsp(4, FSPChild|FSPDeleted), rect(RecChildAnchor, 0, 0, 1, 1)),
	)
	dg := container(RecDgContainer, container(RecSpgrContainer, patriarch, group))
	d := ReadDrawing(dg[8:], 100, 8)
	var got []Rect
	d.Shapes(func(sh *Shape) (Rect, bool) {
		if len(sh.ClientAnchor) == 1 {
			return Rect{Left: 100, Top: 100, Right: 200, Bottom: 300}, true
		}
		return Rect{}, false
	}, func(sh *Shape, tr *Transform) {
		got = append(got, tr.Apply(sh.Anchor))
	})
	want := Rect{Left: 150, Top: 200, Right: 200, Bottom: 300}
	if len(got) != 1 || got[0] != want {
		t.Errorf("expected the shape at %+v, got: %+v", want, got)
	}
}

func TestTransform(t *testing.T) {
	tr := &Transform{From: Rect{0, 0, 100, 100}, To: Rect{10, 20, 210, 120}}
	got := tr.Apply(Rect{50, 50, 100, 100})
	if got != (Rect{110, 70, 210, 120}) {
		t.Errorf("expected the rectangle in the page, got: %+v", got)
	}
	var none *Transform
	if got := none.Apply(Rect{1, 2, 3, 4}); got != (Rect{1, 2, 3, 4}) {
		t.Errorf("expected the same rectangle, got: %+v", got)
	}
}

// TestWordArt checks that the text of a WordArt shape draws as the
// outlines of its glyphs, and that each line fills its share of the box
// when gtextAlign stretches the lines.
func TestWordArt(t *testing.T) {
	var text []byte
	for _, r := range "Hi\nthere" {
		text = le.AppendUint16(text, uint16(r))
	}
	text = le.AppendUint16(text, 0)
	var p Props
	opt := le.AppendUint16(nil, PropGtextUnicode|0x8000)
	opt = le.AppendUint32(opt, uint32(len(text)))
	p.Read(append(opt, text...), 1)
	p.Set(PropFillColor, 0x0000FF)
	p.Set(PropGtextAlign, alignStretch)
	sh := &Shape{Type: SptTextEffectMin, Props: p}
	c := Chain{&sh.Props}
	if !WordArt(sh, c) {
		t.Fatal("expected a WordArt shape")
	}
	s := &Style{Fonts: fontdb.Default()}
	f := s.Figure(sh, c, Rect{Left: 10, Top: 20, Right: 210, Bottom: 120})
	if f == nil || len(f.Paths) != 2 {
		t.Fatalf("expected a path for each line, got: %+v", f)
	}
	for i, p := range f.Paths {
		b, _ := p.Path.ComputeTightBounds()
		top := 50 * float64(i)
		if !near(float64(b.Left()), 0) || !near(float64(b.Right()), 200) || !near(float64(b.Top()), top) || !near(float64(b.Bottom()), top+50) {
			t.Errorf("expected line %d to fill its box, got: %v", i, b)
		}
	}
	if !f.Fill.Visible() || f.Fill.Color.R != 1 {
		t.Errorf("expected the red fill of the shape, got: %+v", f.Fill)
	}
	// without fonts, the shape keeps the geometry of its type
	if f := (&Style{}).Figure(sh, c, Rect{Right: 10, Bottom: 10}); f == nil || len(f.Paths) != 1 {
		t.Errorf("expected the geometry without fonts, got: %+v", f)
	}
}

// TestLineOutline checks that the glyphs of a line follow their advances,
// and that a line leaves the outlines that the face keeps as they are.
func TestLineOutline(t *testing.T) {
	f := (&Style{Fonts: fontdb.Default()}).face("", false, false)
	if f == nil {
		t.Fatal("expected a face")
	}
	g := gtext{face: f, spacing: 1}
	bounds := func(s string) float32 {
		p, ok := g.lineOutline(s)
		if !ok {
			t.Fatalf("expected the outline of %q", s)
		}
		b, _ := p.ComputeTightBounds()
		return b.Right()
	}
	one := bounds("l")
	two := bounds("ll")
	gid, _ := f.Glyph('l')
	if !near(float64(two), float64(one+f.Advance(gid))) {
		t.Errorf("expected the second glyph one advance after the first, got %g and %g", one, two)
	}
	if again := bounds("l"); again != one {
		t.Errorf("expected the same outline again, got %g and %g", one, again)
	}
}

// TestAdjust checks that the slant of a parallelogram is a share of its
// width, 5400 of 21600 by default, as in Office Drawing.
func TestAdjust(t *testing.T) {
	for _, c := range []struct {
		adjust uint32
		set    bool
		want   float64
	}{{0, false, 50}, {10800, true, 100}} {
		var p Props
		if c.set {
			p.Set(PropAdjust1, c.adjust)
		}
		sh := &Shape{Type: sptParallelogram, Props: p}
		paths, _ := (&Style{}).Geometry(sh, Chain{&sh.Props}, 200, 100)
		if len(paths) == 0 {
			t.Fatal("expected the parallelogram")
		}
		// the top edge starts at the slant
		x := 200.0
		for _, pt := range paths[0].Path.Points() {
			if pt.Y < 0.01 {
				x = min(x, float64(pt.X))
			}
		}
		if !near(x, c.want) {
			t.Errorf("expected the slant %g for the adjust value %d, got %g", c.want, c.adjust, x)
		}
	}
}
