package drawingml

import (
	"image"
	"math"

	"github.com/xo/okage"

	"github.com/xo/ivy/internal/display"
)

// maxBlurPixels is the number of pixels of the mask of a blurred shadow.
// A larger shadow takes fewer pixels for each unit, and draws softer.
const maxBlurPixels = 4 << 20

// blurredShadow fills the shadow of the paths with the color col, blurred
// with the radius blur in the units of the user space of dev. The paths
// that fill and stroke say which paths fill, with the even-odd rule, and
// which ones stroke. scale is the number of pixels for a unit of the user
// space. It returns false when it cannot make the mask, and the caller
// draws the shadow without its blur.
func blurredShadow(dev display.Device, paths []*okage.Path, stroke []bool, st okage.Stroke, col RGBA, blur, scale float64) bool {
	if !(blur > 0) || !(scale > 0) || math.IsInf(blur, 0) || math.IsInf(scale, 0) {
		return false
	}
	// the outlines of the strokes fill as the other paths
	areas := make([]*okage.Path, 0, len(paths))
	rules := make([]okage.FillRule, 0, len(paths))
	for i, p := range paths {
		if !stroke[i] {
			areas = append(areas, p)
			rules = append(rules, okage.FillRuleEvenOdd)
			continue
		}
		s := st
		if s.Width <= 0 {
			s.Width = float32(1 / scale)
		}
		if o, ok := p.Stroke(&s, float32(scale)); ok {
			areas = append(areas, o)
			rules = append(rules, okage.FillRuleWinding)
		}
	}
	if len(areas) == 0 {
		return false
	}
	b := areas[0].Bounds()
	minX, minY, maxX, maxY := float64(b.Left()), float64(b.Top()), float64(b.Right()), float64(b.Bottom())
	for _, a := range areas[1:] {
		r := a.Bounds()
		minX, minY = math.Min(minX, float64(r.Left())), math.Min(minY, float64(r.Top()))
		maxX, maxY = math.Max(maxX, float64(r.Right())), math.Max(maxY, float64(r.Bottom()))
	}
	// the blur spreads the shadow by about its radius on each side
	margin := blur * 1.5
	minX, minY, maxX, maxY = minX-margin, minY-margin, maxX+margin, maxY+margin
	res := scale
	if px := (maxX - minX) * (maxY - minY) * res * res; px > maxBlurPixels {
		res *= math.Sqrt(maxBlurPixels / px)
	}
	w, h := math.Ceil((maxX-minX)*res), math.Ceil((maxY-minY)*res)
	if !(w >= 1 && h >= 1) || w*h > 2*maxBlurPixels {
		return false
	}
	mask, ok := okage.NewMask(uint32(w), uint32(h))
	if !ok {
		return false
	}
	toMask := okage.TransformFromRow(float32(res), 0, 0, float32(res), float32(-minX*res), float32(-minY*res))
	for i, a := range areas {
		mask.FillPath(a, rules[i], true, toMask)
	}
	alpha := &image.Alpha{Pix: mask.DataMut(), Stride: int(w), Rect: image.Rect(0, 0, int(w), int(h))}
	// three box blurs make a blur close to a Gaussian blur, whose standard
	// deviation is half the radius
	boxBlur(alpha, blur/2*res)
	toUser := okage.TransformFromRow(float32(1/res), 0, 0, float32(1/res), float32(minX), float32(minY))
	dev.FillMask(alpha, toUser, display.ImageOptions{Opacity: 1, Smooth: true}, okage.SolidColor(col.Okage()))
	return true
}

// boxBlur blurs img in place with three box blurs in each direction, which
// give a blur close to a Gaussian blur of the standard deviation sigma, in
// pixels.
func boxBlur(img *image.Alpha, sigma float64) {
	if !(sigma > 0.25) {
		return
	}
	// the width of each box of three, from the standard deviation
	r := max(int(math.Round(math.Sqrt(4*sigma*sigma+1)-1)/2), 1)
	w, h := img.Rect.Dx(), img.Rect.Dy()
	line := make([]int, max(w, h))
	out := make([]int, max(w, h))
	for range 3 {
		for y := range h {
			for x := range w {
				line[x] = int(img.Pix[y*img.Stride+x])
			}
			boxLine(line[:w], out[:w], r)
			for x := range w {
				img.Pix[y*img.Stride+x] = uint8(out[x])
			}
		}
		for x := range w {
			for y := range h {
				line[y] = int(img.Pix[y*img.Stride+x])
			}
			boxLine(line[:h], out[:h], r)
			for y := range h {
				img.Pix[y*img.Stride+x] = uint8(out[y])
			}
		}
	}
}

// boxLine sets each value of out to the mean of the values of in within r
// of it, with 0 outside in.
func boxLine(in, out []int, r int) {
	n := len(in)
	d := 2*r + 1
	sum := 0
	for i := 0; i <= r && i < n; i++ {
		sum += in[i]
	}
	for i := range n {
		out[i] = (sum + d/2) / d
		if j := i + r + 1; j < n {
			sum += in[j]
		}
		if j := i - r; j >= 0 {
			sum -= in[j]
		}
	}
}
