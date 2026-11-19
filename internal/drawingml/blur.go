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

// silhouette is the area that the paths of a figure cover: the fills and
// the outlines of the strokes, in the user space of a device.
type silhouette struct {
	areas []*okage.Path
	rules []okage.FillRule
}

// newSilhouette returns the silhouette of the paths. stroke says which
// paths stroke with st, and the others fill with the even-odd rule. scale is
// the number of pixels for a unit of the user space. It returns false for
// no area.
func newSilhouette(paths []*okage.Path, stroke []bool, st okage.Stroke, scale float64) (silhouette, bool) {
	var s silhouette
	for i, p := range paths {
		if !stroke[i] {
			s.areas = append(s.areas, p)
			s.rules = append(s.rules, okage.FillRuleEvenOdd)
			continue
		}
		w := st
		if w.Width <= 0 {
			w.Width = float32(1 / scale)
		}
		if o, ok := p.Stroke(&w, float32(scale)); ok {
			s.areas = append(s.areas, o)
			s.rules = append(s.rules, okage.FillRuleWinding)
		}
	}
	return s, len(s.areas) > 0
}

// maskFrame is the place of the pixels of a mask in the user space: its
// corner, the number of its pixels for a unit, and its size.
type maskFrame struct {
	x, y, res float64
	w, h      int
}

// frame returns the frame of a mask over the bounds of s, grown by margin
// on each side, at scale pixels for a unit. A large mask takes fewer
// pixels for each unit. It returns false for a mask that it cannot make.
func (s silhouette) frame(margin, scale float64) (maskFrame, bool) {
	if !(scale > 0) || math.IsInf(scale, 0) || !(margin >= 0) || math.IsInf(margin, 0) {
		return maskFrame{}, false
	}
	b := s.areas[0].Bounds()
	minX, minY, maxX, maxY := float64(b.Left()), float64(b.Top()), float64(b.Right()), float64(b.Bottom())
	for _, a := range s.areas[1:] {
		r := a.Bounds()
		minX, minY = math.Min(minX, float64(r.Left())), math.Min(minY, float64(r.Top()))
		maxX, maxY = math.Max(maxX, float64(r.Right())), math.Max(maxY, float64(r.Bottom()))
	}
	minX, minY, maxX, maxY = minX-margin, minY-margin, maxX+margin, maxY+margin
	res := scale
	if px := (maxX - minX) * (maxY - minY) * res * res; px > maxBlurPixels {
		res *= math.Sqrt(maxBlurPixels / px)
	}
	w, h := math.Ceil((maxX-minX)*res), math.Ceil((maxY-minY)*res)
	if !(w >= 1 && h >= 1) || w*h > 2*maxBlurPixels {
		return maskFrame{}, false
	}
	return maskFrame{x: minX, y: minY, res: res, w: int(w), h: int(h)}, true
}

// toUser returns the transform from the pixels of the frame onto the user
// space.
func (f maskFrame) toUser() okage.Transform {
	return okage.TransformFromRow(float32(1/f.res), 0, 0, float32(1/f.res), float32(f.x), float32(f.y))
}

// mask returns the coverage of s moved by dx and dy, in the pixels of the
// frame f, blurred with the standard deviation sigma in units of the user
// space. It returns nil when it cannot make the mask.
func (s silhouette) mask(f maskFrame, dx, dy, grow, sigma float64) *image.Alpha {
	m, ok := okage.NewMask(uint32(f.w), uint32(f.h))
	if !ok {
		return nil
	}
	toMask := okage.TransformFromRow(float32(f.res), 0, 0, float32(f.res), float32((dx-f.x)*f.res), float32((dy-f.y)*f.res))
	for i, a := range s.areas {
		m.FillPath(a, s.rules[i], true, toMask)
	}
	if grow != 0 && !math.IsNaN(grow) && !math.IsInf(grow, 0) {
		// the outline of the areas, as wide as twice the growth, grows the
		// areas, or makes them smaller
		edge := m
		if grow < 0 {
			if edge, ok = okage.NewMask(uint32(f.w), uint32(f.h)); !ok {
				return nil
			}
		}
		st := okage.DefaultStroke()
		st.Width = float32(2 * math.Abs(grow))
		st.LineJoin = okage.LineJoinRound
		for _, a := range s.areas {
			if o, ok := a.Stroke(&st, float32(f.res)); ok {
				edge.FillPath(o, okage.FillRuleWinding, true, toMask)
			}
		}
		if grow < 0 {
			pix, cut := m.DataMut(), edge.Data()
			for i, v := range cut {
				pix[i] = uint8((int(pix[i])*(255-int(v)) + 127) / 255)
			}
		}
	}
	alpha := &image.Alpha{Pix: m.DataMut(), Stride: f.w, Rect: image.Rect(0, 0, f.w, f.h)}
	boxBlur(alpha, sigma*f.res)
	return alpha
}

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
	s, ok := newSilhouette(paths, stroke, st, scale)
	if !ok {
		return false
	}
	// the blur spreads the shadow by about its radius on each side
	f, ok := s.frame(blur*1.5, scale)
	if !ok {
		return false
	}
	// three box blurs make a blur close to a Gaussian blur, whose standard
	// deviation is half the radius
	alpha := s.mask(f, 0, 0, 0, blur/2)
	if alpha == nil {
		return false
	}
	dev.FillMask(alpha, f.toUser(), display.ImageOptions{Opacity: 1, Smooth: true}, okage.SolidColor(col.Okage()))
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
