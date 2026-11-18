package xps

import (
	"bytes"
	"encoding/binary"
	"image"
	"math"
	"testing"

	"golang.org/x/image/font/gofont/goregular"

	"github.com/xo/ivy/internal/display"
	"github.com/xo/ivy/internal/fixture"
	"github.com/xo/okage"
)

// inkBounds returns the bounds of the pixels of img that are not white.
func inkBounds(img image.Image) image.Rectangle {
	var out image.Rectangle
	b := img.Bounds()
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			r, g, bl, _ := img.At(x, y).RGBA()
			if r < 0xf000 || g < 0xf000 || bl < 0xf000 {
				out = out.Union(image.Rect(x, y, x+1, y+1))
			}
		}
	}
	return out
}

func TestSidewaysRun(t *testing.T) {
	d, err := Open(t.Context(), bytes.NewReader(pkgFile("", fixture.Entry{Name: "font.ttf", Data: goregular.TTF})))
	if err != nil {
		t.Fatal(err)
	}
	r := &renderer{ctx: t.Context(), doc: d, part: "/Documents/Pages/1.fpage", dev: &display.List{}}
	n := mustParse(t, `<Glyphs FontUri="/font.ttf" FontRenderingEmSize="10" OriginX="0" OriginY="0" UnicodeString="II" IsSideways="true"/>`)
	run, err := r.glyphRun(n)
	if err != nil {
		t.Fatal(err)
	}
	if len(run.Glyphs) != 2 {
		t.Fatalf("expected 2 glyphs, got: %d", len(run.Glyphs))
	}
	face, err := d.font("/font.ttf")
	if err != nil {
		t.Fatal(err)
	}
	g0, g1 := run.Glyphs[0], run.Glyphs[1]
	// Go Regular has no vmtx table, so the vertical advance is 1 em
	if dx := g1.X - g0.X; math.Abs(float64(dx-10)) > 0.01 {
		t.Errorf("expected an advance of 10, got: %v", dx)
	}
	// the top center of the glyph is at the pen: half of the horizontal
	// advance below the baseline after the turn
	if want := face.Advance(g0.ID) * 10 / 2; math.Abs(float64(g0.Y-want)) > 0.01 {
		t.Errorf("expected y %v, got: %v", want, g0.Y)
	}
	if g0.X <= 0 || g0.X > 10 {
		t.Errorf("expected the origin of the glyph within the first em of the run, got: %v", g0.X)
	}
	// the turn maps the x axis of the glyph to -y, and its y axis to x
	p := okage.PointFromXY(1, 2)
	run.Transform.MapPoint(&p)
	if p != okage.PointFromXY(2, -1) {
		t.Errorf("expected a turn of 90 degrees counterclockwise, got: %v", p)
	}
}

func TestSidewaysRender(t *testing.T) {
	// an I is taller than it is wide, and a sideways I is wider than it is
	// tall
	for _, sideways := range []string{"false", "true"} {
		body := `<Glyphs FontUri="/font.ttf" FontRenderingEmSize="40" OriginX="20" OriginY="60" UnicodeString="I" Fill="#000000" IsSideways="` + sideways + `"/>`
		img, err := render(t, pkgFile(body, fixture.Entry{Name: "font.ttf", Data: goregular.TTF}))
		if err != nil {
			t.Fatal(err)
		}
		b := inkBounds(img)
		if b.Empty() {
			t.Fatalf("sideways %s: expected ink", sideways)
		}
		wide := b.Dx() > b.Dy()
		if wide != (sideways == "true") {
			t.Errorf("sideways %s: expected the other shape, got: %v", sideways, b)
		}
		if sideways == "true" && (b.Min.X < 20 || b.Max.X > 20+40) {
			t.Errorf("expected the glyph within the first em after the pen, got: %v", b)
		}
	}
}

func TestSidewaysLongTable(t *testing.T) {
	// a font whose first table claims 4 GB still gives the vertical metrics
	// of a sideways run, because font.Load cuts the table
	long := bytes.Clone(goregular.TTF)
	binary.BigEndian.PutUint32(long[12+12:], 0xfffffff0)
	d, err := Open(t.Context(), bytes.NewReader(pkgFile("", fixture.Entry{Name: "font.ttf", Data: long})))
	if err != nil {
		t.Fatal(err)
	}
	r := &renderer{ctx: t.Context(), doc: d, part: "/Documents/Pages/1.fpage", dev: &display.List{}}
	n := mustParse(t, `<Glyphs FontUri="/font.ttf" FontRenderingEmSize="10" OriginX="0" OriginY="0" UnicodeString="II" IsSideways="true"/>`)
	run, err := r.glyphRun(n)
	if err != nil {
		t.Fatal(err)
	}
	if len(run.Glyphs) != 2 || run.Glyphs[1].X <= run.Glyphs[0].X {
		t.Errorf("expected 2 glyphs down the run, got: %+v", run.Glyphs)
	}
}
