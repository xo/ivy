package odraw

import (
	"bytes"
	"encoding/binary"
	"image/color"
	"image/png"
	"testing"

	"github.com/xo/ivy"
	"github.com/xo/ivy/internal/drawingml"
)

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
// that the colors of a shading mix in linear light.
func TestPathGradient(t *testing.T) {
	var s Style
	var p Props
	p.Set(PropFillType, FillShadeCenter)
	p.Set(PropFillColor, 0x404040)
	p.Set(PropFillBackColor, 0xFFFFFF)
	pt := s.Fill(Chain{&p})
	g := pt.Gradient
	if pt.Kind != drawingml.PaintGradient || g.Path != "rect" || len(g.Stops) != gammaSteps+1 {
		t.Fatalf("expected a path gradient, got %+v", pt)
	}
	if first, last := g.Stops[0].Color, g.Stops[len(g.Stops)-1].Color; first != drawingml.White || last.NRGBA().R != 0x40 {
		t.Errorf("expected white at the focus and gray at the edge, got %v and %v", first, last)
	}
	// the middle of white and of 40% gray in linear light is lighter than
	// the middle of their values
	if mid := g.Stops[gammaSteps/2].Color.NRGBA().R; mid < 0xb4 {
		t.Errorf("expected a light middle, got %#x", mid)
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
