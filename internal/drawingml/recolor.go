package drawingml

import (
	"image"
	"image/draw"
	"math"

	"github.com/xo/okage/filter"
	"github.com/xo/okage/roxmltree"
)

// Recolor is a change of the colors of a picture: the brightness, the
// contrast, the grayscale and the black and white of a picture of Office
// Drawing, and a:lum, a:grayscl, a:biLevel and a:duotone of a:blip
// (ECMA-376 Part 1, 20.1.8). The zero Recolor changes nothing.
//
// The changes apply in this order to the color of each pixel, and the alpha
// stays: the contrast and the brightness, the gray, the black and white,
// then the duotone.
type Recolor struct {
	// Bright is the brightness, from -1 to 1. 1 makes the picture white.
	Bright float64
	// Contrast is the contrast, from -1 to 1, as the attribute contrast of
	// a:lum. -1 makes the picture gray, and 1 makes each color full or
	// empty.
	Contrast float64
	// Gray makes the picture gray, by the luminance of each pixel.
	Gray bool
	// BiLevel makes each pixel black or white: white when its luminance is
	// at least Threshold.
	BiLevel   bool
	Threshold float64
	// Duotone gives each pixel the color between Duotone[0] and Duotone[1]
	// that its luminance gives, when HasDuotone is set.
	Duotone    [2]RGBA
	HasDuotone bool
}

// Zero reports whether r changes nothing.
func (r Recolor) Zero() bool {
	return r == Recolor{}
}

// ContrastOf returns the contrast of a:lum for the factor k of the contrast
// of a picture of Office Drawing, which multiplies the distance of each
// color from the middle gray. A factor of 1 changes nothing.
func ContrastOf(k float64) float64 {
	switch {
	case math.IsNaN(k) || k <= 0:
		return -1
	case k <= 1:
		return k - 1
	}
	return 1 - 1/k
}

// factor returns the factor of the contrast of r, which multiplies the
// distance of each color from the middle gray. A contrast of 1 gives a
// large factor, which makes each color full or empty.
func (r Recolor) factor() float64 {
	c := r.Contrast
	switch {
	case math.IsNaN(c):
		return 1
	case c <= 0:
		return max(1+c, 0)
	case c >= 1:
		return 1e6
	}
	return 1 / (1 - c)
}

// ParseRecolor reads the changes of the colors of the children of the
// a:blip n, with the colors c for a:duotone.
func (c *Colors) ParseRecolor(n roxmltree.Node) Recolor {
	var r Recolor
	for e := range n.Children() {
		if !e.IsElement() {
			continue
		}
		switch e.TagName().Name() {
		case "lum":
			r.Bright = math.Max(-1, math.Min(pct(numberAttr(e, "bright", 0)), 1))
			r.Contrast = math.Max(-1, math.Min(pct(numberAttr(e, "contrast", 0)), 1))
		case "grayscl":
			r.Gray = true
		case "biLevel":
			r.BiLevel = true
			r.Threshold = unit(pct(numberAttr(e, "thresh", 50000)))
		case "duotone":
			var cols []RGBA
			for ce := range e.Children() {
				if col, ok := c.Of(ce); ok && ce.IsElement() {
					cols = append(cols, col)
				}
			}
			if len(cols) == 2 {
				r.Duotone, r.HasDuotone = [2]RGBA{cols[0], cols[1]}, true
			}
		}
	}
	return r
}

// half is half a level of 8 bits. The functions of filter truncate each
// channel to 8 bits, and half added to each constant makes them round it.
const half = 0.5 / 255

// transfer returns the change of each channel for the contrast and the
// brightness, a linear function from 0 to 1. It returns false when r changes
// neither. Office keeps the middle gray for the contrast alone. With a
// contrast below 1, Office adds less of the brightness: the washout of Word,
// a factor of 0.3 and a brightness of 0.7, gives 0.3 x + 0.805, which the
// export of Word to PDF shows.
func (r Recolor) transfer() (filter.TransferFunction, bool) {
	if r.Bright == 0 && r.Contrast == 0 {
		return filter.TransferFunction{}, false
	}
	k := r.factor()
	b := r.Bright * (1 + min(k, 1)) / 2
	return filter.TransferLinear(float32(k), float32(0.5-k/2+b+half)), true
}

// matrix returns the color matrix of the gray or of the duotone of r. Each
// channel is the color between the two colors that the luminance of the
// pixel gives: black and white for the gray. The alpha stays. It returns
// false when r changes no color by the luminance. The black and white of
// BiLevel makes each pixel gray before the matrix, so the duotone of a
// black and white picture also takes this matrix.
func (r Recolor) matrix() (filter.ColorMatrix, bool) {
	from, to := Black, White
	switch {
	case r.HasDuotone:
		from, to = r.Duotone[0], r.Duotone[1]
	case !r.Gray || r.BiLevel:
		return filter.ColorMatrix{}, false
	}
	var m [20]float32
	for i, c := range [3][2]float64{{from.R, to.R}, {from.G, to.G}, {from.B, to.B}} {
		d := c[1] - c[0]
		m[5*i], m[5*i+1], m[5*i+2], m[5*i+4] = float32(d*lumR), float32(d*lumG), float32(d*lumB), float32(c[0]+half)
	}
	m[18], m[19] = 1, half
	return filter.NewColorMatrix(m), true
}

// The weights of the luminance of a color.
const (
	lumR = 0.299
	lumG = 0.587
	lumB = 0.114
)

// luminance returns the luminance of the color of 8 bits, from 0 to 1.
func luminance(r, g, b uint8) float64 {
	return (lumR*float64(r) + lumG*float64(g) + lumB*float64(b)) / 255
}

// Apply returns img with its colors changed by r, as an image of its own.
func (r Recolor) Apply(img image.Image) *image.NRGBA {
	b := img.Bounds()
	out, ok := img.(*image.NRGBA)
	if !ok || out.Rect != b {
		out = image.NewNRGBA(b)
		draw.Draw(out, b, img, b.Min, draw.Src)
	} else {
		out = &image.NRGBA{Pix: append([]byte(nil), out.Pix...), Stride: out.Stride, Rect: out.Rect}
	}
	r.applyNRGBA(out)
	return out
}

// applyNRGBA changes the colors of img in place, with the transfer function
// and the color matrix of the package filter of okage. No filter of okage
// compares a value with a threshold, so the black and white is a loop of its
// own.
func (r Recolor) applyNRGBA(img *image.NRGBA) {
	if f, ok := r.transfer(); ok {
		filter.ApplyComponentTransferNRGBA(img, f, f, f, filter.TransferFunction{})
	}
	if r.BiLevel {
		r.threshold(img)
	}
	if m, ok := r.matrix(); ok {
		filter.ApplyColorMatrixNRGBA(img, m)
	}
}

// threshold makes each pixel of img black or white, in place: white when its
// luminance is at least the threshold of r.
func (r Recolor) threshold(img *image.NRGBA) {
	b := img.Rect
	for y := range b.Dy() {
		row := img.Pix[y*img.Stride : y*img.Stride+4*b.Dx()]
		for i := 0; i+4 <= len(row); i += 4 {
			var v uint8
			if luminance(row[i], row[i+1], row[i+2]) >= r.Threshold {
				v = 255
			}
			row[i], row[i+1], row[i+2] = v, v, v
		}
	}
}
