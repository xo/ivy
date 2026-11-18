package odraw

import (
	"math"

	"github.com/xo/okage"
)

// This file bends the text of WordArt onto the shape of its type of text
// effect (MS-ODRAW 2.4.24, the types 137 to 175). The types have the same
// shapes as the text warps of ECMA-376, such as textArchUp and textWave1.
// The package draws each one with its default adjust values, and it draws
// the types that it does not know as plain text: the rings, the buttons,
// the curves, the cascades, the stop and the two mixes of inflate and
// deflate.

// The types of text effects of MS-ODRAW 2.4.24 that the package bends.
const (
	sptTextTriangle         = 138
	sptTextTriangleInverted = 139
	sptTextChevron          = 140
	sptTextChevronInverted  = 141
	sptTextArchUpCurve      = 144
	sptTextArchDownCurve    = 145
	sptTextCircleCurve      = 146
	sptTextArchUpPour       = 148
	sptTextArchDownPour     = 149
	sptTextCirclePour       = 150
	sptTextWave1            = 156
	sptTextWave2            = 157
	sptTextWave3            = 158
	sptTextWave4            = 159
	sptTextInflate          = 160
	sptTextDeflate          = 161
	sptTextInflateBottom    = 162
	sptTextDeflateBottom    = 163
	sptTextInflateTop       = 164
	sptTextDeflateTop       = 165
	sptTextFadeRight        = 168
	sptTextFadeLeft         = 169
	sptTextFadeUp           = 170
	sptTextFadeDown         = 171
	sptTextSlantUp          = 172
	sptTextSlantDown        = 173
	sptTextCanUp            = 174
	sptTextCanDown          = 175
)

// warp maps a point of the text, u from the left and v from the top of its
// box, each from 0 to 1, onto the box of the shape, x from the left and y
// from the top, each from 0 to 1.
type warp func(u, v float64) (x, y float64)

// envelope returns the warp that puts the text between the curves top and
// bottom, which give a y for each x.
func envelope(top, bottom func(u float64) float64) warp {
	return func(u, v float64) (float64, float64) {
		t := top(u)
		return u, t + v*(bottom(u)-t)
	}
}

// flat returns the curve of a line at y.
func flat(y float64) func(float64) float64 {
	return func(float64) float64 { return y }
}

// peak returns the curve from y+a at the ends to y in the middle, in
// straight lines.
func peak(y, a float64) func(float64) float64 {
	return func(u float64) float64 { return y + a*math.Abs(2*u-1) }
}

// bulge returns the curve from y+a at the ends to y in the middle, along a
// parabola.
func bulge(y, a float64) func(float64) float64 {
	return func(u float64) float64 { return y + a*(2*u-1)*(2*u-1) }
}

// wave returns the curve of n periods of a sine wave about y, which goes
// up by a first.
func wave(y, a, n float64) func(float64) float64 {
	return func(u float64) float64 { return y - a*math.Sin(2*math.Pi*n*u) }
}

// slope returns the straight curve from y0 at the left to y1 at the right.
func slope(y0, y1 float64) func(float64) float64 {
	return func(u float64) float64 { return y0 + (y1-y0)*u }
}

// fade returns the warp that narrows the text toward the top, or toward
// the bottom when down is set, to 1-a of its width.
func fade(a float64, down bool) warp {
	return func(u, v float64) (float64, float64) {
		f := 1 - v
		if down {
			f = v
		}
		return 0.5 + (u-0.5)*(1-a*f), v
	}
}

// arc returns the warp that puts the text along the ellipse of the box,
// over span radians from the left, clockwise over the top when up is set,
// and counterclockwise under the bottom when it is not. The top of the
// glyphs points out of the ellipse over the top, and into it under the
// bottom. The text takes the band of the share t of the radius at the
// edge of the ellipse.
func arc(span, t float64, up bool) warp {
	return func(u, v float64) (float64, float64) {
		theta, rho := math.Pi-span*u, 1-t*v
		if !up {
			theta, rho = math.Pi+span*u, 1-t*(1-v)
		}
		return 0.5 + 0.5*rho*math.Cos(theta), 0.5 - 0.5*rho*math.Sin(theta)
	}
}

// The bands of the text along an arc: at most maxBand of the radius, and
// pourBand for the types that pour the text into the band.
const (
	maxBand  = 0.6
	minBand  = 0.05
	pourBand = 0.5
)

// curveBand returns the band of a line of text along an arc of span
// radians around the box of w by h points, for a text whose height is
// aspect times its width: the text keeps its shape.
func curveBand(span, aspect, w, h float64) float64 {
	r := min(w, h) / 2
	if !(r > 0) {
		return minBand
	}
	// the length of the arc on the ellipse, with the mean of its radii
	length := span * (w + h) / 4
	t := length * aspect / r
	if math.IsNaN(t) {
		return minBand
	}
	return min(max(t, minBand), maxBand)
}

// warpOf returns the warp of the type of text effect typ in a box of w by
// h points, for a text whose height is aspect times its width, or nil for
// a type that draws its text plain.
func warpOf(typ int, aspect, w, h float64) warp {
	switch typ {
	case sptTextTriangle:
		return envelope(peak(0, 0.5), flat(1))
	case sptTextTriangleInverted:
		return envelope(flat(0), peak(1, -0.5))
	case sptTextChevron:
		return envelope(peak(0, 0.25), peak(0.75, 0.25))
	case sptTextChevronInverted:
		return envelope(peak(0.25, -0.25), peak(1, -0.25))
	case sptTextArchUpCurve:
		return arc(math.Pi, curveBand(math.Pi, aspect, w, h), true)
	case sptTextArchDownCurve:
		return arc(math.Pi, curveBand(math.Pi, aspect, w, h), false)
	case sptTextCircleCurve:
		return arc(2*math.Pi, curveBand(2*math.Pi, aspect, w, h), true)
	case sptTextArchUpPour:
		return arc(math.Pi, pourBand, true)
	case sptTextArchDownPour:
		return arc(math.Pi, pourBand, false)
	case sptTextCirclePour:
		return arc(2*math.Pi, pourBand, true)
	case sptTextWave1:
		return envelope(wave(0.125, 0.125, 1), wave(0.875, 0.125, 1))
	case sptTextWave2:
		return envelope(wave(0.125, -0.125, 1), wave(0.875, -0.125, 1))
	case sptTextWave3:
		return envelope(wave(0.0625, 0.0625, 2), wave(0.9375, 0.0625, 2))
	case sptTextWave4:
		return envelope(wave(0.0625, -0.0625, 2), wave(0.9375, -0.0625, 2))
	case sptTextInflate:
		return envelope(bulge(0, 0.1875), bulge(1, -0.1875))
	case sptTextDeflate:
		return envelope(bulge(0.1875, -0.1875), bulge(0.8125, 0.1875))
	case sptTextInflateBottom:
		return envelope(flat(0), bulge(1, -0.4))
	case sptTextDeflateBottom:
		return envelope(flat(0), bulge(0.5, 0.5))
	case sptTextInflateTop:
		return envelope(bulge(0, 0.4), flat(1))
	case sptTextDeflateTop:
		return envelope(bulge(0.5, -0.5), flat(1))
	case sptTextFadeRight:
		return envelope(slope(0, 1.0/3), slope(1, 2.0/3))
	case sptTextFadeLeft:
		return envelope(slope(1.0/3, 0), slope(2.0/3, 1))
	case sptTextFadeUp:
		return fade(1.0/3, false)
	case sptTextFadeDown:
		return fade(1.0/3, true)
	case sptTextSlantUp:
		return envelope(slope(0.556, 0), slope(1, 0.444))
	case sptTextSlantDown:
		return envelope(slope(0, 0.556), slope(0.444, 1))
	case sptTextCanUp:
		return envelope(bulge(0.15, -0.15), bulge(1, -0.15))
	case sptTextCanDown:
		return envelope(bulge(0, 0.15), bulge(0.85, 0.15))
	}
	return nil
}

// warpSteps bounds the pieces of a segment of an outline, which the warp
// bends as lines.
const (
	warpSteps = 32
	// warpStep is the length of a piece, as a share of the longer side of
	// the box
	warpStep = 1.0 / 128
)

// warpPath returns the path p in the box of w by h points, bent by the
// warp wp. It cuts each segment into short lines, so that a straight
// segment follows the curve of the warp.
func warpPath(p *okage.Path, wp warp, w, h float64) (*okage.Path, bool) {
	step := max(w, h) * warpStep
	at := func(pt okage.Point) (float32, float32) {
		x, y := wp(float64(pt.X)/w, float64(pt.Y)/h)
		return float32(x * w), float32(y * h)
	}
	pieces := func(pts ...okage.Point) int {
		l := 0.0
		for i := 1; i < len(pts); i++ {
			l += math.Hypot(float64(pts[i].X-pts[i-1].X), float64(pts[i].Y-pts[i-1].Y))
		}
		n := math.Ceil(l / step)
		if math.IsNaN(n) {
			return 1
		}
		return int(min(max(n, 1), warpSteps))
	}
	pb := okage.NewPathBuilder()
	var last okage.Point
	it := p.Segments()
	for {
		seg, ok := it.Next()
		if !ok {
			break
		}
		pts := seg.Points
		switch seg.Kind {
		case okage.PathSegmentMoveTo:
			pb.MoveTo(at(pts[0]))
			last = pts[0]
		case okage.PathSegmentLineTo:
			n := pieces(last, pts[0])
			for k := 1; k <= n; k++ {
				pb.LineTo(at(lerp(last, pts[0], float32(k)/float32(n))))
			}
			last = pts[0]
		case okage.PathSegmentQuadTo:
			n := pieces(last, pts[0], pts[1])
			for k := 1; k <= n; k++ {
				pb.LineTo(at(quadAt(last, pts[0], pts[1], float32(k)/float32(n))))
			}
			last = pts[1]
		case okage.PathSegmentCubicTo:
			n := pieces(last, pts[0], pts[1], pts[2])
			for k := 1; k <= n; k++ {
				pb.LineTo(at(cubicAt(last, pts[0], pts[1], pts[2], float32(k)/float32(n))))
			}
			last = pts[2]
		case okage.PathSegmentClose:
			pb.Close()
		}
	}
	return pb.Finish()
}

// lerp returns the point the share t of the way from a to b.
func lerp(a, b okage.Point, t float32) okage.Point {
	return okage.PointFromXY(a.X+(b.X-a.X)*t, a.Y+(b.Y-a.Y)*t)
}

// quadAt returns the point at t of the quadratic curve from a over c to b.
func quadAt(a, c, b okage.Point, t float32) okage.Point {
	return lerp(lerp(a, c, t), lerp(c, b, t), t)
}

// cubicAt returns the point at t of the cubic curve from a over c1 and c2
// to b.
func cubicAt(a, c1, c2, b okage.Point, t float32) okage.Point {
	return lerp(quadAt(a, c1, c2, t), quadAt(c1, c2, b, t), t)
}
