package metafile

import (
	"context"
	"errors"
	"image"
	"image/color"
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
