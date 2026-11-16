package review

import (
	"bufio"
	"image"
	"image/color"
	"math"
	"strings"
	"testing"
)

// filled returns an image of w by h pixels of the color c.
func filled(w, h int, c color.Color) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := range h {
		for x := range w {
			img.Set(x, y, c)
		}
	}
	return img
}

func TestCompare(t *testing.T) {
	white := filled(40, 30, color.White)
	if m, _ := Compare(white, white, false); m.Mean != 0 || m.Blurred != 0 || m.Scaled() {
		t.Errorf("expected no difference, got: %+v", m)
	}
	black := filled(40, 30, color.Black)
	if m, _ := Compare(black, white, false); m.Mean != 255 || m.Blurred != 255 {
		t.Errorf("expected a difference of 255, got: %+v", m)
	}
	// a transparent image is white
	transparent := image.NewNRGBA(image.Rect(0, 0, 40, 30))
	if m, _ := Compare(transparent, white, false); m.Mean != 0 {
		t.Errorf("expected a transparent image to match white, got: %+v", m)
	}
	// a red square of 10 by 10 pixels in a page of 40 by 30: the mean of
	// the channels is 2*255*100/(3*1200)
	square := filled(40, 30, color.White)
	for y := 10; y < 20; y++ {
		for x := 10; x < 20; x++ {
			square.Set(x, y, color.RGBA{255, 0, 0, 255})
		}
	}
	m, heat := Compare(square, white, true)
	if want := 2 * 255 * 100.0 / 3600; math.Abs(m.Mean-want) > 1e-9 {
		t.Errorf("expected a mean of %.3f, got: %.3f", want, m.Mean)
	}
	// the blur keeps the sum of the grey levels inside the page
	if want := 2 * 255 * 100.0 / 3 / 1200; math.Abs(m.Blurred-want) > 1 {
		t.Errorf("expected a blurred difference near %.3f, got: %.3f", want, m.Blurred)
	}
	if heat == nil || heat.Bounds().Size() != image.Pt(40, 30) {
		t.Fatalf("expected a heat map of 40 by 30, got: %v", heat)
	}
	if r, g, _, _ := heat.At(15, 15).RGBA(); r>>8 < 90 || g>>8 > 10 {
		t.Errorf("expected red in the square of the heat map, got: %v", heat.At(15, 15))
	}
	if c := heat.RGBAAt(0, 0); c.R != 255 || c.G != 255 || c.B != 255 {
		t.Errorf("expected white outside the square of the heat map, got: %v", c)
	}
	// a page of ivy of another size is scaled to the size of the reference
	big := filled(80, 60, color.White)
	m, _ = Compare(big, white, false)
	if !m.Scaled() || m.Mean != 0 || m.IvySize != image.Pt(80, 60) || m.Size != image.Pt(40, 30) {
		t.Errorf("expected a scaled page without difference, got: %+v", m)
	}
	if !m.SizeOff() {
		t.Error("expected a page of twice the size to be off")
	}
	if m, _ := Compare(filled(41, 30, color.White), white, false); m.SizeOff() {
		t.Errorf("expected a page one pixel wider not to be off, got: %+v", m)
	}
}

func TestBlurGrey(t *testing.T) {
	img := filled(7, 5, color.White)
	img.Set(3, 2, color.Black)
	g := blurGrey(img, 2)
	// the box of 5 by 5 pixels around the black pixel holds it once
	if want := uint8(255 - (255+12)/25); g[2*7+3] != want {
		t.Errorf("expected %d at the center, got: %d", want, g[2*7+3])
	}
	// a corner sees a box of 3 by 3 pixels, without the black pixel
	if g[0] != 255 {
		t.Errorf("expected 255 at the corner, got: %d", g[0])
	}
}

func TestDecodePNM(t *testing.T) {
	in := "P6\n# a comment\n2 1\n255\n" + string([]byte{255, 0, 0, 0, 0, 255})
	img, err := decodePNM(bufio.NewReader(strings.NewReader(in)))
	if err != nil {
		t.Fatal(err)
	}
	if got := color.RGBAModel.Convert(img.At(1, 0)).(color.RGBA); got != (color.RGBA{0, 0, 255, 255}) {
		t.Errorf("expected blue, got: %v", got)
	}
	if _, err := decodePNM(bufio.NewReader(strings.NewReader("P3\n1 1\n255\n0 0 0\n"))); err == nil {
		t.Error("expected an error for a P3 image")
	}
}
