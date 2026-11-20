package icc_test

import (
	"context"
	"fmt"
	"image"
	"image/color"

	"github.com/xo/ivy/icc"
)

// A program converts a CMYK image without a profile with the default CMYK
// profile of ivy. With a profile, it gives the bytes of the profile instead
// of nil.
func ExampleToSRGB() {
	img := image.NewCMYK(image.Rect(0, 0, 2, 1))
	img.SetCMYK(0, 0, color.CMYK{})
	img.SetCMYK(1, 0, color.CMYK{K: 255})
	out, err := icc.ToSRGB(context.Background(), img, nil, icc.Perceptual)
	if err != nil {
		fmt.Println(err)
		return
	}
	fmt.Printf("%T %v %v\n", out, out.At(0, 0), out.At(1, 0))
	// Output: *image.NRGBA {255 255 255 255} {35 31 32 255}
}

// A program parses a profile once, and converts several images with it.
func ExampleParse() {
	p, err := icc.Parse(icc.DefaultCMYK())
	if err != nil {
		fmt.Println(err)
		return
	}
	for _, k := range []uint8{0, 128} {
		img := image.NewCMYK(image.Rect(0, 0, 1, 1))
		img.SetCMYK(0, 0, color.CMYK{K: k})
		out, err := p.ToSRGB(context.Background(), img, icc.RelativeColorimetric)
		if err != nil {
			fmt.Println(err)
			return
		}
		fmt.Println(out.At(0, 0))
	}
	// Output:
	// {255 255 255 255}
	// {146 148 151 255}
}

// An AVIF file in Display P3 holds the CICP code points 12, 13, 6 and true
// in its colr box. The red of Display P3 is past the red of sRGB, so it
// clips.
func ExampleFromCICP() {
	p, err := icc.FromCICP(icc.CICP{
		ColorPrimaries:          12,
		TransferCharacteristics: 13,
		MatrixCoefficients:      6,
		FullRange:               true,
	})
	if err != nil {
		fmt.Println(err)
		return
	}
	img := image.NewNRGBA(image.Rect(0, 0, 1, 1))
	img.SetNRGBA(0, 0, color.NRGBA{R: 255, A: 255})
	out, err := p.ToSRGB(context.Background(), img, icc.Perceptual)
	if err != nil {
		fmt.Println(err)
		return
	}
	fmt.Println(out.At(0, 0))
	// Output: {255 0 0 255}
}
