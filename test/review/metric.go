package review

import (
	"image"
	"image/color"
	"image/draw"

	xdraw "golang.org/x/image/draw"
)

// blurRadius is the radius of the box blur of Metric.Blurred: a box of 5 by
// 5 pixels, as in TestEPUB.
const blurRadius = 2

// Metric is the difference of a page of ivy from the page of a reference.
type Metric struct {
	// Mean is the mean absolute difference of the channels, over white,
	// from 0 to 255.
	Mean float64 `json:"mean"`
	// Blurred is the mean absolute difference of the grey levels after a
	// box blur of 5 by 5 pixels, which hides the differences of the
	// anti-aliasing and of the positions of the glyphs within a pixel.
	Blurred float64 `json:"blurred"`
	// Size is the size of the reference in pixels, and IvySize the one of
	// ivy. When they differ, the page of ivy is scaled to Size first.
	Size    image.Point `json:"size"`
	IvySize image.Point `json:"ivy_size"`
}

// Scaled reports whether the page of ivy was scaled to the size of the
// reference.
func (m Metric) Scaled() bool {
	return m.Size != m.IvySize
}

// SizeOff reports whether a side of the two pages differs by more than 2
// percent, which is more than the rounding of a size to whole pixels.
func (m Metric) SizeOff() bool {
	off := func(a, b int) bool {
		d := a - b
		if d < 0 {
			d = -d
		}
		return d > 1 && float64(d) > 0.02*float64(max(a, b))
	}
	return off(m.Size.X, m.IvySize.X) || off(m.Size.Y, m.IvySize.Y)
}

// Compare returns the difference of img from ref. When heat is true, it
// also returns a heat map of the difference: the reference in light grey,
// with the pixels that differ in red, darker where they differ more.
func Compare(img, ref image.Image, heat bool) (Metric, *image.RGBA) {
	b := flatten(ref)
	a := flatten(img)
	m := Metric{Size: b.Rect.Size(), IvySize: a.Rect.Size()}
	if m.Size.X == 0 || m.Size.Y == 0 {
		return m, nil
	}
	if m.Scaled() {
		a = scaleTo(a, m.Size)
	}
	var sum int64
	for i := 0; i < len(a.Pix); i += 4 {
		for c := range 3 {
			sum += absDiff(a.Pix[i+c], b.Pix[i+c])
		}
	}
	n := int64(m.Size.X) * int64(m.Size.Y)
	m.Mean = float64(sum) / float64(3*n)
	ga, gb := blurGrey(a, blurRadius), blurGrey(b, blurRadius)
	sum = 0
	for i := range ga {
		sum += absDiff(ga[i], gb[i])
	}
	m.Blurred = float64(sum) / float64(n)
	if !heat {
		return m, nil
	}
	return m, heatMap(a, b)
}

// absDiff returns |a-b|.
func absDiff(a, b uint8) int64 {
	if a > b {
		return int64(a - b)
	}
	return int64(b - a)
}

// flatten returns img over white, as an opaque *image.RGBA with its origin
// at 0, 0.
func flatten(img image.Image) *image.RGBA {
	r := img.Bounds()
	dst := image.NewRGBA(image.Rect(0, 0, r.Dx(), r.Dy()))
	draw.Draw(dst, dst.Rect, image.NewUniform(color.White), image.Point{}, draw.Src)
	draw.Draw(dst, dst.Rect, img, r.Min, draw.Over)
	return dst
}

// scaleTo returns img scaled to size, with a bilinear filter.
func scaleTo(img image.Image, size image.Point) *image.RGBA {
	dst := image.NewRGBA(image.Rectangle{Max: size})
	draw.Draw(dst, dst.Rect, image.NewUniform(color.White), image.Point{}, draw.Src)
	xdraw.BiLinear.Scale(dst, dst.Rect, img, img.Bounds(), draw.Over, nil)
	return dst
}

// blurGrey returns the grey levels of img, blurred with a box of 2r+1 by
// 2r+1 pixels that is cut at the edges, rounded to 8 bits.
func blurGrey(img *image.RGBA, r int) []uint8 {
	w, h := img.Rect.Dx(), img.Rect.Dy()
	grey := make([]int32, w*h)
	for y := range h {
		row := img.Pix[y*img.Stride:]
		for x := range w {
			p := row[4*x:]
			grey[y*w+x] = int32(p[0]) + int32(p[1]) + int32(p[2])
		}
	}
	// a pass along the rows, then a pass along the columns
	rows := make([]int32, w*h)
	for y := range h {
		line := grey[y*w : (y+1)*w]
		var sum int32
		for x := 0; x <= r && x < w; x++ {
			sum += line[x]
		}
		for x := range w {
			rows[y*w+x] = sum
			if x+r+1 < w {
				sum += line[x+r+1]
			}
			if x-r >= 0 {
				sum -= line[x-r]
			}
		}
	}
	span := func(i, n int) int32 {
		return int32(min(i+r, n-1) - max(i-r, 0) + 1)
	}
	out := make([]uint8, w*h)
	for x := range w {
		var sum int32
		for y := 0; y <= r && y < h; y++ {
			sum += rows[y*w+x]
		}
		cx := span(x, w)
		for y := range h {
			n := 3 * cx * span(y, h)
			out[y*w+x] = uint8((sum + n/2) / n)
			if y+r+1 < h {
				sum += rows[(y+r+1)*w+x]
			}
			if y-r >= 0 {
				sum -= rows[(y-r)*w+x]
			}
		}
	}
	return out
}

// heatMap returns the difference of a and b, which have the same size: b
// in light grey, and red where the pixels differ.
func heatMap(a, b *image.RGBA) *image.RGBA {
	out := image.NewRGBA(b.Rect)
	for i := 0; i < len(b.Pix); i += 4 {
		var d int64
		for c := range 3 {
			d = max(d, absDiff(a.Pix[i+c], b.Pix[i+c]))
		}
		g := (int(b.Pix[i]) + int(b.Pix[i+1]) + int(b.Pix[i+2])) / 3
		base := 255 - (255-g)/4
		// a difference under 8 is the noise of the anti-aliasing
		alpha := 0.0
		if d >= 8 {
			alpha = min(1, 0.25+float64(d)/128)
		}
		red := [3]float64{220 - 120*alpha, 0, 0}
		for c := range 3 {
			out.Pix[i+c] = uint8(float64(base)*(1-alpha) + red[c]*alpha + 0.5)
		}
		out.Pix[i+3] = 255
	}
	return out
}
