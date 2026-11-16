package odf

import (
	"fmt"
	"image"
	"image/color"
	"image/png"
	"io"
)

// odsPNG encodes img as PNG.
func odsPNG(w io.Writer, img image.Image) error {
	if err := png.Encode(w, img); err != nil {
		return fmt.Errorf("encoding png: %w", err)
	}
	return nil
}

// odsInked reports whether img has a dark pixel in r.
func odsInked(img image.Image, r image.Rectangle) bool {
	r = r.Intersect(img.Bounds())
	for y := r.Min.Y; y < r.Max.Y; y++ {
		for x := r.Min.X; x < r.Max.X; x++ {
			c := color.NRGBAModel.Convert(img.At(x, y)).(color.NRGBA)
			if int(c.R)+int(c.G)+int(c.B) < 3*128 {
				return true
			}
		}
	}
	return false
}
