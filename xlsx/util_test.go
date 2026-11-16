package xlsx

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"os"
	"testing"
)

// nrgba returns c as a color.NRGBA.
func nrgba(c color.Color) color.NRGBA {
	return color.NRGBAModel.Convert(c).(color.NRGBA)
}

// near reports whether two colors differ by at most 8 in each channel.
func near(a, b color.NRGBA) bool {
	d := func(x, y uint8) bool { return x-y <= 8 || y-x <= 8 }
	return d(a.R, b.R) && d(a.G, b.G) && d(a.B, b.B)
}

// hasInk reports whether img has a dark pixel in r.
func hasInk(img image.Image, r image.Rectangle) bool {
	r = r.Intersect(img.Bounds())
	for y := r.Min.Y; y < r.Max.Y; y++ {
		for x := r.Min.X; x < r.Max.X; x++ {
			c := nrgba(img.At(x, y))
			if int(c.R)+int(c.G)+int(c.B) < 3*128 {
				return true
			}
		}
	}
	return false
}

// writePNG writes img to path, for a person who looks at a page.
func writePNG(t *testing.T, path string, img image.Image) {
	t.Helper()
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, buf.Bytes(), 0o600); err != nil {
		t.Fatal(err)
	}
}
