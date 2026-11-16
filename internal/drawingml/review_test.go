package drawingml

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"strings"
	"testing"

	"github.com/xo/okage"

	"github.com/xo/ivy"
	"github.com/xo/ivy/internal/fixture"
)

// The tests of this file come from the faults that the visual review found
// in the drawings of Office.

func TestPictureResolution(t *testing.T) {
	blue := color.NRGBA{0, 0, 255, 255}
	for _, c := range []struct {
		name string
		data []byte
		dpi  float64
	}{
		{"jpeg of 300 dpi", fixture.JPEGDPI(fixture.JPEG(4, 4, blue), 300), 300},
		{"png of 144 dpi", fixture.PNGDPI(fixture.PNG(4, 4, blue), 144), 144},
		{"png without a resolution", fixture.PNG(4, 4, blue), 96},
		{"not a picture", []byte("text"), 96},
	} {
		t.Run(c.name, func(t *testing.T) {
			x, y := NewPicture(c.data, ivy.Config{}).Resolution()
			if x < c.dpi-0.5 || x > c.dpi+0.5 || y < c.dpi-0.5 || y > c.dpi+0.5 {
				t.Errorf("expected %g dots per inch, got %g by %g", c.dpi, x, y)
			}
		})
	}
}

// TestTileResolution checks that a picture that repeats takes the size of
// its pixels at its resolution: 64 pixels at 144 dots per inch are 32
// points.
func TestTileResolution(t *testing.T) {
	img := fixture.Image(64, 64, color.NRGBA{255, 0, 0, 255})
	for x := 32; x < 64; x++ {
		for y := range 64 {
			img.Set(x, y, color.NRGBA{0, 0, 255, 255})
		}
	}
	data := fixture.PNGDPI(encodePNG(t, img), 144)
	pt := Paint{Kind: PaintPicture, Picture: NewPicture(data, ivy.Config{}), Opacity: 1,
		Tile: &Tile{SX: 1, SY: 1, Flip: "none", Align: "tl"}}
	s := NewScene(ivy.Config{})
	box, _ := okage.RectFromXYWH(0, 0, 64, 64)
	s.AddFigure(&Figure{
		Transform: okage.IdentityTransform(), W: 64, H: 64,
		Paths: []Path{{Path: okage.PathFromRect(box)}},
		Fill:  pt, Line: Line{Paint: Paint{Kind: PaintNone}},
	})
	pix, _ := okage.NewPixmap(64, 64)
	pix.Fill(okage.ColorWhite)
	if err := s.Render(t.Context(), pix, okage.IdentityTransform()); err != nil {
		t.Fatal(err)
	}
	// the tile is 32 points across: red from 0 to 16, blue from 16 to 32,
	// and red again from 32
	for _, c := range []struct {
		x    uint32
		blue bool
	}{{8, false}, {24, true}, {40, false}, {56, true}} {
		p, _ := pix.Pixel(c.x, 8)
		if (p.Blue() > 128) != c.blue {
			t.Errorf("pixel %d: expected blue %v, got %v", c.x, c.blue, p)
		}
	}
}

func TestGroupSpace(t *testing.T) {
	// the children of a box of 10 by 20 map onto a group of 20 by 60
	s := space{m: okage.IdentityTransform(), sx: 2, sy: 3, ox: 100, oy: 200}
	x := s.place(Xfrm{X: 105, Y: 210, W: 5, H: 10, OK: true})
	if x.X != 10 || x.Y != 30 || x.W != 10 || x.H != 30 {
		t.Errorf("expected the box (10, 30) of 10 by 30, got (%g, %g) of %g by %g", x.X, x.Y, x.W, x.H)
	}
	// the identity space leaves the box as it is
	if got := rootSpace(okage.IdentityTransform()).place(Xfrm{X: 1, Y: 2, W: 3, H: 4}); got.X != 1 || got.W != 3 {
		t.Errorf("expected the same box, got %+v", got)
	}
}

// TestBoxOverflow checks that the text of a box anchored at its middle
// overflows the box by the same amount at its top and at its bottom, and
// the text anchored at the bottom overflows at the top.
func TestBoxOverflow(t *testing.T) {
	c := &Colors{Theme: testTheme}
	var list ListStyle
	for _, a := range []struct {
		anchor string
		up     bool
	}{{"t", false}, {"ctr", true}, {"b", true}} {
		body := parse(t, `<a:txBody><a:bodyPr anchor="`+a.anchor+`"/><a:p><a:r><a:t>text</a:t></a:r></a:p></a:txBody>`)
		tb := c.ParseTextBody(body, &list, nil, ParseBodyStyle(parse(t, `<a:bodyPr anchor="`+a.anchor+`"/>`)), Fields{})
		box, ok := tb.Box(testTheme, 200, 40)
		if !ok {
			t.Fatal("expected a box")
		}
		// the page of the text starts above the box when the text can
		// overflow upward
		if up := box.Transform.TY < 0; up != a.up {
			t.Errorf("anchor %s: expected the page above the box %v, got the offset %g", a.anchor, a.up, box.Transform.TY)
		}
		if a.anchor == "ctr" && !strings.Contains(box.HTML, `vertical-align:middle`) {
			t.Errorf("anchor ctr: expected the middle alignment:\n%s", box.HTML)
		}
	}
}

// encodePNG returns img as a PNG file.
func encodePNG(t *testing.T, img image.Image) []byte {
	t.Helper()
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}
