package icc

import (
	"context"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"math"

	"github.com/xo/ivy"
	"github.com/xo/ivy/internal/moxcms"
)

// checkRows is the number of rows between two checks of the context.
const checkRows = 64

// checkSize refuses an image of more than MaxPixels pixels, or an image
// whose result does not fit in the memory of the platform.
func checkSize(img image.Image) error {
	b := img.Bounds()
	n := int64(b.Dx()) * int64(b.Dy())
	// A pixel of an *image.NRGBA64 takes 8 bytes.
	if n > MaxPixels || n*8 > math.MaxInt {
		return fmt.Errorf("icc: converting an image of %d by %d pixels: %w", b.Dx(), b.Dy(), ivy.ErrLimit)
	}
	return nil
}

// isCMYK reports whether the colors of img are CMYK.
func isCMYK(img image.Image) bool {
	if _, ok := img.(*image.CMYK); ok {
		return true
	}
	return img.ColorModel() == color.CMYKModel
}

// isWide reports whether img has samples of more than 8 bits.
func isWide(img image.Image) bool {
	switch img.(type) {
	case *image.Gray16, *image.RGBA64, *image.NRGBA64, *image.Alpha16:
		return true
	}
	switch img.ColorModel() {
	case color.Gray16Model, color.RGBA64Model, color.NRGBA64Model, color.Alpha16Model:
		return true
	}
	return false
}

// newResult returns the image that holds the result for img: an
// *image.NRGBA64 when img has samples of more than 8 bits, and an
// *image.NRGBA otherwise.
func newResult(img image.Image) draw.Image {
	if isWide(img) {
		return image.NewNRGBA64(img.Bounds())
	}
	return image.NewNRGBA(img.Bounds())
}

// copySRGB returns img, which is in sRGB, in the image of newResult.
func copySRGB(ctx context.Context, img image.Image) (image.Image, error) {
	if err := checkSize(img); err != nil {
		return nil, err
	}
	b := img.Bounds()
	dst := newResult(img)
	for y := b.Min.Y; y < b.Max.Y; y += checkRows {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		r := image.Rect(b.Min.X, y, b.Max.X, min(y+checkRows, b.Max.Y))
		draw.Draw(dst, r, img, r.Min, draw.Src)
	}
	return dst, nil
}

// convert runs the transform t on each row of img, whose colors are of the
// kind s, and returns the result in the image of newResult.
func convert(ctx context.Context, img image.Image, s space, t moxcms.TransformExecutor) (image.Image, error) {
	b := img.Bounds()
	w := b.Dx()
	channels := 3
	switch s {
	case spaceGray:
		channels = 1
	case spaceFour:
		channels = 4
	}
	samples := make([]uint8, w*channels)
	rgb := make([]uint8, w*3)
	alpha := make([]uint16, w)
	var nrgba []uint16
	if s != spaceFour {
		nrgba = make([]uint16, w*4)
	}
	dst := newResult(img)
	for y := b.Min.Y; y < b.Max.Y; y++ {
		if (y-b.Min.Y)%checkRows == 0 {
			if err := ctx.Err(); err != nil {
				return nil, err
			}
		}
		if s == spaceFour {
			readCMYK(img, y, samples)
			for i := range alpha {
				alpha[i] = 0xffff
			}
		} else {
			readNRGBA64(img, y, nrgba)
			split(nrgba, s, samples, alpha)
		}
		if err := t.Transform(samples, rgb); err != nil {
			return nil, fmt.Errorf("icc: running the transform to sRGB: %w", err)
		}
		writeRow(dst, y, rgb, alpha)
	}
	return dst, nil
}

// to8 returns the sample of 8 bits nearest to the sample of 16 bits v.
func to8(v uint16) uint8 {
	return uint8((uint32(v) + 128) / 257)
}

// unpremultiply returns the color c of 16 bits, premultiplied by the alpha
// a, without the alpha.
func unpremultiply(c, a uint32) uint16 {
	if a == 0 {
		return 0
	}
	return uint16(min((c*0xffff+a/2)/a, 0xffff))
}

// readNRGBA64 fills row with the red, the green, the blue and the alpha of
// 16 bits of each pixel of the row y of img, without premultiplication.
func readNRGBA64(img image.Image, y int, row []uint16) {
	b := img.Bounds()
	switch m := img.(type) {
	case *image.Gray:
		pix := m.Pix[m.PixOffset(b.Min.X, y):]
		for x := range b.Dx() {
			v := uint16(pix[x]) * 0x101
			row[4*x], row[4*x+1], row[4*x+2], row[4*x+3] = v, v, v, 0xffff
		}
	case *image.Gray16:
		pix := m.Pix[m.PixOffset(b.Min.X, y):]
		for x := range b.Dx() {
			v := uint16(pix[2*x])<<8 | uint16(pix[2*x+1])
			row[4*x], row[4*x+1], row[4*x+2], row[4*x+3] = v, v, v, 0xffff
		}
	case *image.NRGBA:
		pix := m.Pix[m.PixOffset(b.Min.X, y):]
		for i := range 4 * b.Dx() {
			row[i] = uint16(pix[i]) * 0x101
		}
	case *image.RGBA:
		pix := m.Pix[m.PixOffset(b.Min.X, y):]
		for x := range b.Dx() {
			a := uint32(pix[4*x+3]) * 0x101
			for c := range 3 {
				row[4*x+c] = unpremultiply(uint32(pix[4*x+c])*0x101, a)
			}
			row[4*x+3] = uint16(a)
		}
	case *image.NRGBA64:
		pix := m.Pix[m.PixOffset(b.Min.X, y):]
		for i := range 4 * b.Dx() {
			row[i] = uint16(pix[2*i])<<8 | uint16(pix[2*i+1])
		}
	case *image.RGBA64:
		pix := m.Pix[m.PixOffset(b.Min.X, y):]
		for x := range b.Dx() {
			a := uint32(pix[8*x+6])<<8 | uint32(pix[8*x+7])
			for c := range 3 {
				row[4*x+c] = unpremultiply(uint32(pix[8*x+2*c])<<8|uint32(pix[8*x+2*c+1]), a)
			}
			row[4*x+3] = uint16(a)
		}
	case *image.YCbCr:
		for x := range b.Dx() {
			yi := m.YOffset(b.Min.X+x, y)
			ci := m.COffset(b.Min.X+x, y)
			r, g, bl := color.YCbCrToRGB(m.Y[yi], m.Cb[ci], m.Cr[ci])
			row[4*x], row[4*x+1], row[4*x+2], row[4*x+3] = uint16(r)*0x101, uint16(g)*0x101, uint16(bl)*0x101, 0xffff
		}
	default:
		for x := range b.Dx() {
			c, _ := color.NRGBA64Model.Convert(img.At(b.Min.X+x, y)).(color.NRGBA64)
			row[4*x], row[4*x+1], row[4*x+2], row[4*x+3] = c.R, c.G, c.B, c.A
		}
	}
}

// split fills samples with the colors of row for the kind s, as samples of
// 8 bits, and alpha with the alpha of each pixel.
func split(row []uint16, s space, samples []uint8, alpha []uint16) {
	for x := range alpha {
		r, g, b := uint32(row[4*x]), uint32(row[4*x+1]), uint32(row[4*x+2])
		if s == spaceGray {
			// The luma of color.Gray16Model.
			samples[x] = to8(uint16((19595*r + 38470*g + 7471*b + 1<<15) >> 16))
		} else {
			samples[3*x], samples[3*x+1], samples[3*x+2] = to8(uint16(r)), to8(uint16(g)), to8(uint16(b))
		}
		alpha[x] = row[4*x+3]
	}
}

// readCMYK fills samples with the cyan, the magenta, the yellow and the
// black of each pixel of the row y of img, whose model is color.CMYKModel.
func readCMYK(img image.Image, y int, samples []uint8) {
	b := img.Bounds()
	if m, ok := img.(*image.CMYK); ok {
		i := m.PixOffset(b.Min.X, y)
		copy(samples, m.Pix[i:i+4*b.Dx()])
		return
	}
	for x := range b.Dx() {
		c, _ := color.CMYKModel.Convert(img.At(b.Min.X+x, y)).(color.CMYK)
		samples[4*x], samples[4*x+1], samples[4*x+2], samples[4*x+3] = c.C, c.M, c.Y, c.K
	}
}

// writeRow writes the colors rgb of 8 bits and the alpha of 16 bits into the
// row y of dst, which newResult made.
func writeRow(dst draw.Image, y int, rgb []uint8, alpha []uint16) {
	switch m := dst.(type) {
	case *image.NRGBA:
		pix := m.Pix[m.PixOffset(m.Rect.Min.X, y):]
		for x, a := range alpha {
			pix[4*x], pix[4*x+1], pix[4*x+2], pix[4*x+3] = rgb[3*x], rgb[3*x+1], rgb[3*x+2], to8(a)
		}
	case *image.NRGBA64:
		pix := m.Pix[m.PixOffset(m.Rect.Min.X, y):]
		for x, a := range alpha {
			for c := range 3 {
				// A sample of 8 bits times 0x101 is the same level in 16 bits.
				pix[8*x+2*c], pix[8*x+2*c+1] = rgb[3*x+c], rgb[3*x+c]
			}
			pix[8*x+6], pix[8*x+7] = uint8(a>>8), uint8(a)
		}
	}
}
