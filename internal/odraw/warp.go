package odraw

import (
	"math"

	"github.com/xo/okage"
)

// This file bends the text of WordArt onto the shape of its type of text
// effect (MS-ODRAW 2.4.24, the types 137 to 175). The types have the shapes
// of the text warps of ECMA-376, such as textArchUp and textWave1, and the
// adjust values of the shape change them, as LibreOffice draws them. The
// package draws the types that it does not know as plain text: the rings,
// the buttons, the curves and the two mixes of inflate and deflate.

// The types of text effects of MS-ODRAW 2.4.24 that the package bends.
const (
	sptTextStop             = 137
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
	sptTextCascadeUp        = 154
	sptTextCascadeDown      = 155
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

// warpDefaults holds the defaults of the adjust values of the types of
// text effects that the package bends. Most of them are a y of the
// geometry, from 0 to 21600. The first value of an arch is an angle in
// 16.16 bits, and the second value of a pour is the inner radius of its
// band, from 0 to 10800. The second value of a wave, its shift, is left out.
var warpDefaults = map[int][]float64{
	sptTextStop:             {2700},
	sptTextTriangle:         {10800},
	sptTextTriangleInverted: {10800},
	sptTextChevron:          {5400},
	sptTextChevronInverted:  {16200},
	sptTextArchUpCurve:      {180 * fixedDegree},
	sptTextArchDownCurve:    {0},
	sptTextCircleCurve:      {-179 * fixedDegree},
	sptTextArchUpPour:       {180 * fixedDegree, 5400},
	sptTextArchDownPour:     {0, 5400},
	sptTextCirclePour:       {-179 * fixedDegree, 5400},
	sptTextCascadeUp:        {9600},
	sptTextCascadeDown:      {9600},
	sptTextWave1:            {1404},
	sptTextWave2:            {1404},
	sptTextWave3:            {1404},
	sptTextWave4:            {1404},
	sptTextInflate:          {2945},
	sptTextDeflate:          {8100},
	sptTextInflateBottom:    {14941},
	sptTextDeflateBottom:    {11475},
	sptTextInflateTop:       {6556},
	sptTextDeflateTop:       {10125},
	sptTextFadeRight:        {7200},
	sptTextFadeLeft:         {7200},
	sptTextFadeUp:           {7200},
	sptTextFadeDown:         {7200},
	sptTextSlantUp:          {12000},
	sptTextSlantDown:        {12000},
	sptTextCanUp:            {18514},
	sptTextCanDown:          {3086},
}

// warpValues returns the adjust values of the type of text effect typ with
// the properties c, or nil for a type that the package does not bend. A
// value that is not an angle is a share of the side of the geometry, and
// it is between 0 and 1.
func warpValues(typ int, c Chain) []float64 {
	defs, ok := warpDefaults[typ]
	if !ok {
		return nil
	}
	a := make([]float64, len(defs))
	for i, def := range defs {
		a[i] = def
		if v, ok := c.Get(PropAdjust1 + uint16(i)); ok {
			a[i] = float64(int32(v))
		}
		if i > 0 || !isArch(typ) {
			a[i] = min(max(a[i]/geoSize, 0), 1)
		}
	}
	return a
}

// isArch reports whether the type of text effect typ puts its text along
// an arc, whose first adjust value is an angle.
func isArch(typ int) bool {
	return typ >= sptTextArchUpCurve && typ <= sptTextCirclePour && typ != 147
}

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

// stop returns the curve from y+a at the ends to y from a third to two
// thirds of the width, in straight lines, as the side of an octagon.
func stop(y, a float64) func(float64) float64 {
	return func(u float64) float64 { return y + a*max(1-3*u, 3*u-2, 0) }
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
// the bottom when down is set, by a of the width on each side.
func fade(a float64, down bool) warp {
	return func(u, v float64) (float64, float64) {
		f := 1 - v
		if down {
			f = v
		}
		return 0.5 + (u-0.5)*(1-2*a*f), v
	}
}

// arc returns the warp that puts the text along the ellipse of the box,
// over span radians about the direction mid, which is counterclockwise
// from the right. The text goes clockwise over the top when up is set, and
// counterclockwise under the bottom when it is not. The top of the glyphs
// points out of the ellipse over the top, and into it under the bottom.
// The text takes the band of the share t of the radius at the edge of the
// ellipse.
func arc(mid, span, t float64, up bool) warp {
	return func(u, v float64) (float64, float64) {
		theta, rho := mid+span/2-span*u, 1-t*v
		if !up {
			theta, rho = mid-span/2+span*u, 1-t*(1-v)
		}
		return 0.5 + 0.5*rho*math.Cos(theta), 0.5 - 0.5*rho*math.Sin(theta)
	}
}

// arcSweep returns the span in radians of an arc whose text is symmetric
// about the direction mid, in degrees clockwise from the right with y
// down, and starts at the angle a in 16.16 bits.
func arcSweep(mid, a float64) float64 {
	d := math.Mod(a/fixedDegree-mid, 360)
	switch {
	case d > 180:
		d -= 360
	case d <= -180:
		d += 360
	}
	if math.IsNaN(d) {
		return math.Pi
	}
	return min(2*math.Abs(d), 360) * math.Pi / 180
}

// The bands of the text along an arc: at most maxBand of the radius, and
// at least minBand.
const (
	maxBand = 0.6
	minBand = 0.05
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

// pourBand returns the band of a type that pours its text into the ring
// between the edge of the ellipse and the inner radius a, as a share of the
// side of the geometry.
func pourBand(a float64) float64 {
	return min(max(1-2*a, minBand), 1)
}

// warpOf returns the warp of the type of text effect typ with the
// properties c, in a box of w by h points, for a text whose height is
// aspect times its width, or nil for a type that draws its text plain.
func warpOf(typ int, c Chain, aspect, w, h float64) warp {
	v := warpValues(typ, c)
	if v == nil {
		return nil
	}
	a := v[0]
	switch typ {
	case sptTextStop:
		return envelope(stop(0, a), stop(1, -a))
	case sptTextTriangle:
		return envelope(peak(0, a), flat(1))
	case sptTextTriangleInverted:
		return envelope(flat(0), peak(1, a-1))
	case sptTextChevron:
		return envelope(peak(0, a), peak(1-a, a))
	case sptTextChevronInverted:
		return envelope(peak(1-a, a-1), peak(1, a-1))
	case sptTextArchUpCurve:
		span := arcSweep(270, a)
		return arc(math.Pi/2, span, curveBand(span, aspect, w, h), true)
	case sptTextArchDownCurve:
		span := arcSweep(90, a)
		return arc(-math.Pi/2, span, curveBand(span, aspect, w, h), false)
	case sptTextCircleCurve:
		span := arcSweep(0, a)
		return arc(0, span, curveBand(span, aspect, w, h), true)
	case sptTextArchUpPour:
		return arc(math.Pi/2, arcSweep(270, a), pourBand(v[1]), true)
	case sptTextArchDownPour:
		return arc(-math.Pi/2, arcSweep(90, a), pourBand(v[1]), false)
	case sptTextCirclePour:
		return arc(0, arcSweep(0, a), pourBand(v[1]), true)
	case sptTextCascadeUp:
		return envelope(slope((1-a)/4, 0), slope(1, a))
	case sptTextCascadeDown:
		return envelope(slope(0, (1-a)/4), slope(a, 1))
	case sptTextWave1:
		return envelope(wave(a, a, 1), wave(1-a, a, 1))
	case sptTextWave2:
		return envelope(wave(a, -a, 1), wave(1-a, -a, 1))
	case sptTextWave3:
		return envelope(wave(a, a, 2), wave(1-a, a, 2))
	case sptTextWave4:
		return envelope(wave(a, -a, 2), wave(1-a, -a, 2))
	case sptTextInflate:
		return envelope(bulge(0, a), bulge(1, -a))
	case sptTextDeflate:
		return envelope(bulge(a, -a), bulge(1-a, a))
	case sptTextInflateBottom:
		return envelope(flat(0), bulge(1, a-1))
	case sptTextDeflateBottom:
		return envelope(flat(0), bulge(a, 1-a))
	case sptTextInflateTop:
		return envelope(bulge(0, a), flat(1))
	case sptTextDeflateTop:
		return envelope(bulge(a, -a), flat(1))
	case sptTextFadeRight:
		return envelope(slope(0, a), slope(1, 1-a))
	case sptTextFadeLeft:
		return envelope(slope(a, 0), slope(1-a, 1))
	case sptTextFadeUp:
		return fade(a, false)
	case sptTextFadeDown:
		return fade(a, true)
	case sptTextSlantUp:
		return envelope(slope(a, 0), slope(1, 1-a))
	case sptTextSlantDown:
		return envelope(slope(0, 1-a), slope(a, 1))
	case sptTextCanUp:
		return envelope(bulge(0, 1-a), bulge(a, 1-a))
	case sptTextCanDown:
		return envelope(bulge(a, -a), bulge(1, -a))
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
