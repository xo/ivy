package metafile

import (
	"context"
	"errors"
	"image"
	"image/color"
	"math"
	"strings"
	"testing"

	"github.com/xo/okage"

	"github.com/xo/ivy"
	"github.com/xo/ivy/internal/display"
	"github.com/xo/ivy/internal/fixture"
	"github.com/xo/ivy/internal/gowemf"
)

var (
	red   = color.NRGBA{255, 0, 0, 255}
	green = color.NRGBA{0, 255, 0, 255}
	blue  = color.NRGBA{0, 0, 255, 255}
	black = color.NRGBA{0, 0, 0, 255}
)

// render draws the metafile data onto a pixmap of 100 by 100 pixels, over
// white.
func render(t *testing.T, data []byte) (*okage.Pixmap, error) {
	t.Helper()
	p, err := Open(data, Options{})
	if err != nil {
		t.Fatal(err)
	}
	pm, ok := okage.NewPixmap(100, 100)
	if !ok {
		t.Fatal("no pixmap")
	}
	pm.Fill(okage.ColorWhite)
	dst, _ := okage.RectFromXYWH(0, 0, 100, 100)
	err = p.Draw(t.Context(), display.NewOkage(pm, okage.IdentityTransform()), dst)
	return pm, err
}

// pixel returns the color of the pixel (x, y) of pm.
func pixel(pm *okage.Pixmap, x, y int) color.NRGBA {
	c, _ := pm.Pixel(uint32(x), uint32(y))
	return color.NRGBA{c.Red(), c.Green(), c.Blue(), c.Alpha()}
}

// near reports whether the colors differ by at most tol in each channel.
func near(a, b color.NRGBA, tol int) bool {
	d := func(x, y uint8) bool { return abs(int(x)-int(y)) <= tol }
	return d(a.R, b.R) && d(a.G, b.G) && d(a.B, b.B) && d(a.A, b.A)
}

func abs(v int) int {
	if v < 0 {
		return -v
	}
	return v
}

// expect fails the test if the pixel (x, y) of pm is not near want.
func expect(t *testing.T, pm *okage.Pixmap, x, y int, want color.NRGBA) {
	t.Helper()
	if got := pixel(pm, x, y); !near(got, want, 8) {
		t.Errorf("pixel (%d, %d) = %v, want %v", x, y, got, want)
	}
}

// filled returns an EMF of 100 by 100 pixels with a null pen and a solid
// brush of the color c selected.
func filled(c color.NRGBA) *fixture.EMF {
	e := fixture.NewEMF(100, 100)
	e.Select(e.Brush(c, -1))
	e.Select(fixture.Stock(8))
	return e
}

func TestOpen(t *testing.T) {
	e := filled(red).Rectangle(10, 10, 50, 50).Describe("ivy test", "a square")
	p, err := Open(e.Bytes(), Options{})
	if err != nil {
		t.Fatal(err)
	}
	if p.Kind() != EMF || p.Kind().String() != "EMF" {
		t.Errorf("kind %v", p.Kind())
	}
	if w, h := p.Size(); w != 72 || h != 72 {
		t.Errorf("size %gx%g, want 72x72", w, h)
	}
	if program, title := p.Description(); program != "ivy test" || title != "a square" {
		t.Errorf("description %q %q", program, title)
	}
	for _, data := range [][]byte{nil, []byte("not a metafile at all"), e.Bytes()[:60]} {
		if _, err := Open(data, Options{}); !errors.Is(err, ivy.ErrFormat) {
			t.Errorf("open %q: %v, want ErrFormat", data, err)
		}
	}
	lim := ivy.Config{Limits: ivy.Limits{MaxMemory: 64}}
	if _, err := Open(e.Bytes(), Options{Config: lim}); !errors.Is(err, ivy.ErrLimit) {
		t.Errorf("open past MaxMemory: %v, want ErrLimit", err)
	}
}

func TestRectangle(t *testing.T) {
	pm, err := render(t, filled(red).Rectangle(10, 20, 50, 60).Bytes())
	if err != nil {
		t.Fatal(err)
	}
	expect(t, pm, 30, 40, red)
	expect(t, pm, 5, 5, color.NRGBA{255, 255, 255, 255})
	expect(t, pm, 70, 40, color.NRGBA{255, 255, 255, 255})
}

func TestPen(t *testing.T) {
	e := fixture.NewEMF(100, 100)
	e.Select(fixture.Stock(5))
	e.Select(e.Pen(0, 6, blue))
	e.Ellipse(10, 10, 90, 90)
	e.Polyline(0, 50, 100, 50)
	pm, err := render(t, e.Bytes())
	if err != nil {
		t.Fatal(err)
	}
	// the ellipse is not filled, and its outline crosses x = 50 at the top
	expect(t, pm, 50, 30, color.NRGBA{255, 255, 255, 255})
	expect(t, pm, 50, 11, blue)
	expect(t, pm, 25, 50, blue)
}

func TestClip(t *testing.T) {
	e := filled(green)
	e.IntersectClip(20, 20, 80, 80)
	e.ExcludeClip(40, 40, 60, 60)
	e.Rectangle(0, 0, 100, 100)
	pm, err := render(t, e.Bytes())
	if err != nil {
		t.Fatal(err)
	}
	expect(t, pm, 10, 10, color.NRGBA{255, 255, 255, 255})
	expect(t, pm, 30, 30, green)
	expect(t, pm, 50, 50, color.NRGBA{255, 255, 255, 255})
	expect(t, pm, 70, 50, green)
}

// TestFarPoints checks that a world transform with a huge scale draws its
// shapes, which then cover the picture, and that a transform that is not
// finite draws nothing and does not panic.
func TestFarPoints(t *testing.T) {
	img := image.NewNRGBA(image.Rect(0, 0, 2, 2))
	for i := range img.Pix {
		img.Pix[i] = 0xff
	}
	for _, s := range []float32{1e10, 1e12, 1e30, float32(math.Inf(1)), float32(math.NaN())} {
		draw := func(f func(e *fixture.EMF)) *okage.Pixmap {
			t.Helper()
			e := fixture.NewEMF(100, 100)
			// EMR_SETWORLDTRANSFORM, with the center of the picture at the
			// origin of the world
			e.Record(35, s, float32(0), float32(0), s, float32(50), float32(50))
			f(e)
			pm, err := render(t, e.Bytes())
			if err != nil && strings.Contains(err.Error(), "panic") {
				t.Fatalf("scale %g: %v", s, err)
			}
			return pm
		}
		fill := draw(func(e *fixture.EMF) {
			e.Select(e.Brush(red, -1))
			e.Select(fixture.Stock(8))
			e.Rectangle(-1, -1, 1, 1)
		})
		pen := draw(func(e *fixture.EMF) {
			e.Select(e.Pen(0, 1, blue))
			e.Polyline(-1, 0, 1, 0)
		})
		draw(func(e *fixture.EMF) {
			e.Gradient(-1, -1, 1, 1, red, green, true)
			e.Image(-1, -1, 2, 2, img)
			e.Select(e.Font(-1, 400, false, "Arial"))
			e.Text(0, 0, "Hi")
		})
		if s == 1e10 || s == 1e12 {
			for _, p := range [][2]int{{1, 1}, {50, 50}, {98, 98}} {
				expect(t, fill, p[0], p[1], red)
				expect(t, pen, p[0], p[1], blue)
			}
		}
	}
}

func TestHatch(t *testing.T) {
	e := fixture.NewEMF(100, 100)
	e.Select(e.Brush(black, 1)) // HS_VERTICAL
	e.Select(fixture.Stock(8))
	e.Transparent()
	e.Rectangle(0, 0, 100, 100)
	pm, err := render(t, e.Bytes())
	if err != nil {
		t.Fatal(err)
	}
	var dark, light int
	for x := range 16 {
		if pixel(pm, x, 50).R < 128 {
			dark++
		} else {
			light++
		}
	}
	if dark == 0 || light == 0 {
		t.Errorf("hatch: %d dark and %d light pixels, want both", dark, light)
	}
}

func TestImage(t *testing.T) {
	img := image.NewNRGBA(image.Rect(0, 0, 2, 2))
	img.SetNRGBA(0, 0, red)
	img.SetNRGBA(1, 0, green)
	img.SetNRGBA(0, 1, blue)
	img.SetNRGBA(1, 1, black)
	e := fixture.NewEMF(100, 100).Image(0, 0, 100, 100, img)
	pm, err := render(t, e.Bytes())
	if err != nil {
		t.Fatal(err)
	}
	expect(t, pm, 25, 25, red)
	expect(t, pm, 75, 25, green)
	expect(t, pm, 25, 75, blue)
	expect(t, pm, 75, 75, black)
}

func TestGradient(t *testing.T) {
	e := fixture.NewEMF(100, 100).Gradient(0, 0, 100, 100, red, blue, false)
	pm, err := render(t, e.Bytes())
	if err != nil {
		t.Fatal(err)
	}
	expect(t, pm, 1, 50, red)
	expect(t, pm, 98, 50, blue)
	mid := pixel(pm, 50, 50)
	if !near(mid, color.NRGBA{128, 0, 128, 255}, 12) {
		t.Errorf("middle of the gradient %v", mid)
	}
	// the two triangles of the rectangle join without a seam
	for y := 1; y < 99; y++ {
		if c := pixel(pm, y, y); c.A != 255 || c.G > 8 {
			t.Fatalf("seam at (%d, %d): %v", y, y, c)
		}
	}
	// a triangle with three colors is an image
	tri := gowemfTriangle()
	if _, ok := linearOf(tri); ok {
		t.Error("a triangle of three colors is not a linear gradient")
	}
}

func TestText(t *testing.T) {
	e := fixture.NewEMF(100, 100)
	e.Select(e.Font(-60, 400, false, "Arial"))
	e.TextColor(black)
	e.Transparent()
	e.Text(10, 10, "HI")
	pm, err := render(t, e.Bytes())
	if err != nil {
		t.Fatal(err)
	}
	// the stem of the H is dark, and the space above the text is white
	var dark int
	for y := 15; y < 60; y++ {
		for x := 10; x < 30; x++ {
			if pixel(pm, x, y).R < 64 {
				dark++
			}
		}
	}
	if dark < 50 {
		t.Errorf("text: %d dark pixels, want 50 or more", dark)
	}
	expect(t, pm, 50, 5, color.NRGBA{255, 255, 255, 255})
}

// TestGlyphText checks that glyph indexes draw the text of another record in
// the font of ivy (D53). The indexes belong to Arial, which the program does
// not have.
func TestGlyphText(t *testing.T) {
	glyphs := func(e *fixture.EMF) *fixture.EMF {
		// EMR_EXTTEXTOUTW with ETO_GLYPH_INDEX, at the reference point of
		// fixture.EMF.Text
		return e.Record(84, 0, 0, -1, -1, uint32(1), float32(1), float32(1),
			10, 10, uint32(2), uint32(76), uint32(0x10), 0, 0, -1, -1, uint32(0), uint16(43), uint16(44))
	}
	dark := func(pm *okage.Pixmap) int {
		var n int
		for y := 15; y < 60; y++ {
			for x := 10; x < 30; x++ {
				if pixel(pm, x, y).R < 64 {
					n++
				}
			}
		}
		return n
	}
	e := fixture.NewEMF(100, 100)
	e.Select(e.Font(-60, 400, false, "Arial"))
	e.Transparent()
	// the text in white, which does not show, and then its glyphs in black
	e.TextColor(color.NRGBA{255, 255, 255, 255})
	e.Text(10, 10, "HI")
	e.TextColor(black)
	pm, err := render(t, glyphs(e).Bytes())
	if err != nil {
		t.Fatal(err)
	}
	if n := dark(pm); n < 50 {
		t.Errorf("glyphs with their text: %d dark pixels, want 50 or more", n)
	}
	// without the text, the indexes of Arial are not drawn
	e = fixture.NewEMF(100, 100)
	e.Select(e.Font(-60, 400, false, "Arial"))
	e.Transparent()
	e.TextColor(black)
	pm, err = render(t, glyphs(e).Bytes())
	if _, ok := errors.AsType[*ivy.PartialError](err); !ok {
		t.Errorf("glyphs alone: %v, want a partial error", err)
	}
	if n := dark(pm); n != 0 {
		t.Errorf("glyphs alone: %d dark pixels, want 0", n)
	}
}

func TestWMF(t *testing.T) {
	for _, placeable := range []bool{true, false} {
		w := fixture.NewWMF(200, 200, placeable)
		w.Select(w.Brush(red))
		w.Select(w.NullPen())
		w.Rectangle(100, 0, 200, 100)
		p, err := Open(w.Bytes(), Options{})
		if err != nil {
			t.Fatal(placeable, err)
		}
		if p.Kind() != WMF {
			t.Errorf("kind %v", p.Kind())
		}
		width, _ := p.Size()
		want := float32(144)
		if !placeable {
			want = 150
		}
		if width != want {
			t.Errorf("placeable %v: width %g, want %g", placeable, width, want)
		}
		pm, err := render(t, w.Bytes())
		if err != nil {
			t.Fatal(placeable, err)
		}
		expect(t, pm, 75, 25, red)
		expect(t, pm, 25, 25, color.NRGBA{255, 255, 255, 255})
		expect(t, pm, 75, 75, color.NRGBA{255, 255, 255, 255})
	}
}

func TestPartial(t *testing.T) {
	// EMR_EXTFLOODFILL reads the pixels of the device, which a display list
	// cannot
	e := filled(red).Rectangle(10, 10, 50, 50).Record(53, 20, 20, color.NRGBA{}, uint32(0))
	pm, err := render(t, e.Bytes())
	if _, ok := errors.AsType[*ivy.PartialError](err); !ok || !errors.Is(err, ivy.ErrUnsupported) {
		t.Fatalf("flood fill: %v, want a partial error", err)
	}
	expect(t, pm, 30, 30, red)
}

func TestCancel(t *testing.T) {
	p, err := Open(filled(red).Rectangle(10, 10, 50, 50).Bytes(), Options{})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	var l display.List
	dst, _ := okage.RectFromXYWH(0, 0, 10, 10)
	if err := p.Draw(ctx, &l, dst); !errors.Is(err, context.Canceled) {
		t.Errorf("draw after cancel: %v", err)
	}
}

func TestList(t *testing.T) {
	p, err := Open(filled(red).Rectangle(10, 10, 50, 50).Bytes(), Options{})
	if err != nil {
		t.Fatal(err)
	}
	var l display.List
	dst, _ := okage.RectFromXYWH(0, 0, 200, 200)
	if err := p.Draw(t.Context(), &l, dst); err != nil {
		t.Fatal(err)
	}
	want := "save\nfill [20 20 100 100] EvenOdd\nrestore"
	if got := l.String(); got != want {
		t.Errorf("list:\n%s\nwant:\n%s", got, want)
	}
}

// gowemfTriangle returns a triangle with a red, a green and a blue corner.
func gowemfTriangle() gowemf.GradientTriangle {
	return gowemf.GradientTriangle{
		Points: [3]gowemf.Point{{X: 0, Y: 0}, {X: 10, Y: 0}, {X: 0, Y: 10}},
		Colors: [3]color.NRGBA64{{R: 0xffff, A: 0xffff}, {G: 0xffff, A: 0xffff}, {B: 0xffff, A: 0xffff}},
	}
}

// TestClipKept checks that the records under one clip share it on the
// device, and that a new clip replaces it.
func TestClipKept(t *testing.T) {
	e := filled(green)
	e.IntersectClip(20, 20, 80, 80)
	e.Rectangle(0, 0, 50, 50)
	e.Rectangle(50, 0, 100, 50)
	e.Rectangle(0, 50, 50, 100)
	e.ExcludeClip(40, 40, 60, 60)
	e.Rectangle(50, 50, 100, 100)
	p, err := Open(e.Bytes(), Options{})
	if err != nil {
		t.Fatal(err)
	}
	var l display.List
	dst, _ := okage.RectFromXYWH(0, 0, 100, 100)
	if err := p.Draw(t.Context(), &l, dst); err != nil {
		t.Fatal(err)
	}
	got := l.String()
	if n := strings.Count(got, "\nsave\n"); n != 2 {
		t.Errorf("expected two clips, got %d:\n%s", n, got)
	}
	if strings.Count(got, "save") != strings.Count(got, "restore") {
		t.Errorf("expected a restore for each save:\n%s", got)
	}
	pm, err := render(t, e.Bytes())
	if err != nil {
		t.Fatal(err)
	}
	expect(t, pm, 10, 10, color.NRGBA{255, 255, 255, 255})
	expect(t, pm, 30, 30, green)
	expect(t, pm, 70, 30, green)
	expect(t, pm, 30, 70, green)
	expect(t, pm, 55, 55, color.NRGBA{255, 255, 255, 255})
	expect(t, pm, 70, 70, green)
	expect(t, pm, 90, 90, color.NRGBA{255, 255, 255, 255})
}
