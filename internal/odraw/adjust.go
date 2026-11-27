package odraw

import (
	"math"

	"github.com/xo/okage"

	"github.com/xo/ivy/internal/drawingml"
)

// This file converts the adjust values of the preset shapes of Office
// Drawing (MS-ODRAW 2.3.6.13 to 2.3.6.22) to those of the presets of
// ECMA-376 that draw them. An adjust value of Office Drawing is in the
// units of the geometry, 21600 for the width and the height of the box,
// and most of them are a coordinate in the box. An adjust value of
// ECMA-376 is a share of the width, of the height or of the shorter side,
// in units of 100000.

// unknown marks a default of Office Drawing that the package does not know.
// A value of ECMA-376 that needs it keeps the default of ECMA-376.
var unknown = math.NaN()

// adjustRule converts the adjust values of a type of shape.
type adjustRule struct {
	// defaults holds the default of each adjust value of Office Drawing,
	// from adjustValue to adjust8Value, or unknown.
	defaults []float64
	// conv returns the adjust values of ECMA-376, in the order of the
	// names of the preset, from the values a of Office Drawing in the box
	// b. A value that is NaN keeps the default of ECMA-376.
	conv func(a []float64, b box) []float64
	// place returns the frame of the preset from the values a of Office
	// Drawing in the box b. Without it, the preset fills the box.
	place func(a []float64, b box) frame
}

// frame is the box in which a preset draws a shape of Office Drawing, in
// points from the top left corner of the box of the shape. A frame larger
// than the box draws a shape that Office Drawing draws out of its box,
// such as the head of a circular arrow. When flipV is set, the preset
// draws upside down in the frame.
type frame struct {
	x, y, w, h float64
	flipV      bool
}

// transform returns the transform from the box of the preset to the box
// of the shape.
func (f frame) transform() okage.Transform {
	if f.flipV {
		return okage.TransformFromRow(1, 0, 0, -1, float32(f.x), float32(f.y+f.h))
	}
	return okage.TransformFromRow(1, 0, 0, 1, float32(f.x), float32(f.y))
}

// point returns the point x, y of the box of the preset in the box of the
// shape.
func (f frame) point(x, y float32) (float32, float32) {
	if f.flipV {
		return x + float32(f.x), float32(f.y+f.h) - y
	}
	return x + float32(f.x), y + float32(f.y)
}

// flipped returns the frame of a preset that Office Drawing draws upside
// down, such as the trapezoid, which has its narrow side down.
func flipped(_ []float64, b box) frame {
	return frame{w: b.w, h: b.h, flipV: true}
}

// box is the box of a shape, in points, with its shorter side.
type box struct {
	w, h, ss float64
}

// frac returns a coordinate v of the geometry as a share of its side.
func frac(v float64) float64 {
	return v / geoSize * 100000
}

// center returns a coordinate v of the geometry as an offset from the
// center of its side, as a share of the side.
func center(v float64) float64 {
	return frac(v - geoSize/2)
}

// xss returns an x coordinate v of the geometry as a share of the shorter
// side of the box.
func (b box) xss(v float64) float64 {
	return frac(v) * b.w / b.ss
}

// yss returns a y coordinate v of the geometry as a share of the shorter
// side of the box.
func (b box) yss(v float64) float64 {
	return frac(v) * b.h / b.ss
}

// fracRule returns the rule of a shape whose only adjust value is a share of
// a side or of the shorter side, with the default def.
func fracRule(def float64) adjustRule {
	return adjustRule{defaults: []float64{def}, conv: func(a []float64, _ box) []float64 {
		return []float64{frac(a[0])}
	}}
}

// arrowRule returns the rule of an arrow with one head, such as the right
// arrow. Along the x axis, the first value is the x of the base of the
// head, and the second one is the y of the top of the shaft. right is
// whether the head is on the right. When vert is set, the arrow is on the
// y axis, and the two values are a y and an x.
func arrowRule(defaults []float64, right, vert bool) adjustRule {
	return adjustRule{defaults: defaults, conv: func(a []float64, b box) []float64 {
		head := a[0]
		if right {
			head = geoSize - head
		}
		if vert {
			return []float64{frac(geoSize - 2*a[1]), b.yss(head)}
		}
		return []float64{frac(geoSize - 2*a[1]), b.xss(head)}
	}}
}

// starRule returns the rule of a star. The value is the distance of the
// inner points from the edge, and ECMA-376 takes their radius as a share
// of 50000.
func starRule(def float64) adjustRule {
	return adjustRule{defaults: []float64{def}, conv: func(a []float64, _ box) []float64 {
		return []float64{(geoSize/2 - a[0]) / (geoSize / 2) * 50000}
	}}
}

// calloutRule returns the rule of a callout with a line, whose points are
// pairs of values. Office Drawing gives the points from the end of the
// line to the box, each as x and y. ECMA-376 gives them from the box to the
// end, each as y and x.
func calloutRule(defaults []float64) adjustRule {
	return adjustRule{defaults: defaults, conv: func(a []float64, _ box) []float64 {
		n := len(a) / 2
		out := make([]float64, 0, len(a))
		for k := range n {
			p := 2 * (n - 1 - k)
			out = append(out, frac(a[p+1]), frac(a[p]))
		}
		return out
	}}
}

// wedgeRule returns the rule of a callout whose tip is the first two
// values, which ECMA-376 measures from the center. The defaults of the tip
// are unknown. extra are the defaults of the values after the tip, each a
// share of a side.
func wedgeRule(extra ...float64) adjustRule {
	return adjustRule{defaults: append([]float64{unknown, unknown}, extra...), conv: func(a []float64, _ box) []float64 {
		out := make([]float64, 0, len(a))
		out = append(out, center(a[0]), center(a[1]))
		for _, v := range a[2:] {
			out = append(out, frac(v))
		}
		return out
	}}
}

// The defaults of the line callouts of Office Drawing, with 2, 3 and 4
// points.
var (
	callout1Defaults = []float64{-8280, 24300, -1800, 4050}
	callout2Defaults = []float64{-10080, 24300, -3600, 4050, -1800, 4050}
	callout3Defaults = []float64{23400, 24400, 25200, 21600, 25200, 4050, 23400, 4050}
)

// callout90Defaults holds the defaults of the callouts with a vertical line
// of 2 points, the types 178 to 181.
var callout90Defaults = []float64{-1800, 24300, -1800, 4050}

// adjustRules holds the rules of the types of shapes of MS-ODRAW 2.4.24
// whose adjust values the package converts. A type that it does not hold
// draws with the defaults of ECMA-376.
var adjustRules = map[int]adjustRule{
	// the radius of the corners, as a share of the shorter side
	SptRoundRect: fracRule(3600),
	// the x of the apex
	sptTriangle: fracRule(10800),
	// the slant, as a share of the width
	sptParallelogram: {defaults: []float64{5400}, conv: func(a []float64, b box) []float64 {
		return []float64{b.xss(a[0])}
	}},
	sptHexagon: {defaults: []float64{5400}, conv: func(a []float64, b box) []float64 {
		return []float64{b.xss(a[0])}
	}},
	// the corners and the arms, as a share of the shorter side
	10:  fracRule(6326),
	11:  fracRule(5400),
	16:  fracRule(5400),
	21:  fracRule(3600),
	23:  fracRule(5400),
	57:  fracRule(2700),
	84:  fracRule(2700),
	97:  fracRule(2700),
	98:  fracRule(2700),
	183: fracRule(5400),
	184: fracRule(10800),
	185: fracRule(3600),
	186: fracRule(1800),
	// the arrows
	13: arrowRule([]float64{16200, 5400}, true, false),
	14: arrowRule([]float64{16200, 5400}, true, false),
	93: arrowRule([]float64{16200, 5400}, true, false),
	94: arrowRule([]float64{16200, 5400}, true, false),
	66: arrowRule([]float64{5400, 5400}, false, false),
	67: arrowRule([]float64{16200, 5400}, true, true),
	68: arrowRule([]float64{5400, 5400}, false, true),
	69: arrowRule([]float64{4320, 5400}, false, false),
	70: {defaults: []float64{5400, 4320}, conv: func(a []float64, b box) []float64 {
		return []float64{frac(geoSize - 2*a[0]), b.yss(a[1])}
	}},
	// the point of the pentagon and of the chevron
	15: {defaults: []float64{16200}, conv: func(a []float64, b box) []float64 {
		return []float64{b.xss(geoSize - a[0])}
	}},
	55: {defaults: []float64{16200}, conv: func(a []float64, b box) []float64 {
		return []float64{b.xss(geoSize - a[0])}
	}},
	// the top of the can, as a share of the shorter side
	22: {defaults: []float64{5400}, conv: func(a []float64, b box) []float64 {
		return []float64{b.yss(a[0])}
	}},
	// the bends of the connectors, as a share of the width or the height
	34: fracRule(10800),
	38: fracRule(10800),
	35: {defaults: []float64{10800, 10800}, conv: fracs},
	39: {defaults: []float64{10800, 10800}, conv: fracs},
	36: {defaults: []float64{10800, 10800, 10800}, conv: fracs},
	40: {defaults: []float64{10800, 10800, 10800}, conv: fracs},
	// the line callouts
	41: calloutRule(callout1Defaults),
	44: calloutRule(callout1Defaults),
	47: calloutRule(callout1Defaults),
	50: calloutRule(callout1Defaults),
	42: calloutRule(callout2Defaults),
	45: calloutRule(callout2Defaults),
	48: calloutRule(callout2Defaults),
	51: calloutRule(callout2Defaults),
	43: calloutRule(callout3Defaults),
	46: calloutRule(callout3Defaults),
	49: calloutRule(callout3Defaults),
	52: calloutRule(callout3Defaults),

	// the callouts with a vertical line
	178: calloutRule(callout90Defaults),
	179: calloutRule(callout90Defaults),
	180: calloutRule(callout90Defaults),
	181: calloutRule(callout90Defaults),
	// the callouts with a tip
	61:  wedgeRule(),
	62:  wedgeRule(3600),
	63:  wedgeRule(),
	106: wedgeRule(),
	// the stars
	58:  starRule(2538),
	59:  starRule(2700),
	60:  starRule(2700),
	92:  starRule(2700),
	187: starRule(8100),
	// the height of the wave and its shift from the center
	64:  {defaults: []float64{2700, 10800}, conv: waves},
	188: {defaults: []float64{1350, 10800}, conv: waves},
	// the fold of the folded corner
	65: {defaults: []float64{18900}, conv: func(a []float64, _ box) []float64 {
		return []float64{frac(geoSize - a[0])}
	}},
	// the ends of the brackets and of the braces, and the middle of a
	// brace
	85: {defaults: []float64{1800}, conv: brackets},
	86: {defaults: []float64{1800}, conv: brackets},
	87: {defaults: []float64{1800, 10800}, conv: brackets},
	88: {defaults: []float64{1800, 10800}, conv: brackets},
	// the y of the mouth of the smiley face, from 15510 to 17520
	96: {defaults: []float64{17520}, conv: func(a []float64, _ box) []float64 {
		return []float64{frac(a[0] - 16515)}
	}},
	// the x of the ends of the narrow side of the trapezoid, which is down
	8: {defaults: []float64{5400}, conv: func(a []float64, b box) []float64 {
		return []float64{b.xss(a[0])}
	}, place: flipped},
	// the angles of the arc, which fills the box
	19: {defaults: []float64{-90 * fixedDegree, 0}, conv: arcAngles, place: arcFrame},
	// the start angle of the block arc, which ends at the mirror of the
	// start, and its inner radius
	95: {defaults: []float64{180 * fixedDegree, 5400}, conv: func(a []float64, b box) []float64 {
		st := a[0] / fixedDegree
		return []float64{visual(st, b.w, b.h), visual(180-st, b.w, b.h), frac(geoSize/2 - a[1])}
	}},
	// the start and the end angles of the circular arrow, and its inner
	// radius
	99: {defaults: []float64{-180 * fixedDegree, 0, 5400}, conv: circularArrow, place: func(_ []float64, b box) frame {
		return frame{x: -b.ss / 8, y: -b.ss / 8, w: b.w + b.ss/4, h: b.h + b.ss/4}
	}},
	// the U-turn arrow has no adjust values in Office Drawing
	101: {conv: func(_ []float64, b box) []float64 {
		return []float64{b.xss(6105), b.xss(5950), b.yss(5960), min(b.xss(9300), b.yss(8550)), frac(14260)}
	}},
	// the bent arrows: the x of the base of the head, and the y of the
	// top of the shaft
	91: {defaults: []float64{15100, 2900}, conv: func(a []float64, b box) []float64 {
		return []float64{frac(12160 - 2*a[1]), b.yss(6080), b.xss(geoSize - a[0]), b.yss(12160 - a[1])}
	}},
	// the x of the left of the head, the x of the right of the shaft, and
	// the y of the base of the head
	90: {defaults: []float64{9340, 18500, 7200}, conv: func(a []float64, b box) []float64 {
		return []float64{b.xss(2*a[1] - a[0] - geoSize), b.xss((geoSize - a[0]) / 2), b.yss(a[2])}
	}},
	89: {defaults: []float64{9340, 18500, 6200}, conv: func(a []float64, _ box) []float64 {
		return []float64{frac(2*a[1] - a[0] - geoSize), frac((geoSize - a[0]) / 2), frac(a[2])}
	}},
	// the curved arrows: the side of the head, the side of the shaft, and
	// the base of the head
	102: curvedRule(12960, 19440, 14400, false, true),
	103: curvedRule(12960, 19440, 7200, false, false),
	104: curvedRule(12960, 19440, 7200, true, false),
	105: curvedRule(12960, 19440, 14400, true, true),
	// the ribbons: the x of the left of the middle, and the y of the end
	// of the fold. The center of the type 53 is down, as the ribbon of
	// ECMA-376, and that of the type 54 is up.
	53: {defaults: []float64{5400, 2700}, conv: func(a []float64, _ box) []float64 {
		return []float64{frac(a[1]), frac(geoSize - 2*a[0])}
	}},
	54: {defaults: []float64{5400, 18900}, conv: func(a []float64, _ box) []float64 {
		return []float64{frac(geoSize - a[1]), frac(geoSize - 2*a[0])}
	}},
	107: {defaults: []float64{5400, 5400, 18900}, conv: func(a []float64, _ box) []float64 {
		return []float64{frac(a[1]), frac(geoSize - 2*a[0]), frac(geoSize - a[2])}
	}},
	108: {defaults: []float64{5400, 16200, 2700}, conv: func(a []float64, _ box) []float64 {
		return []float64{frac(geoSize - a[1]), frac(geoSize - 2*a[0]), frac(a[2])}
	}},
	// the arrow callouts: the side of the box, the side of the head, the
	// base of the head and the side of the shaft
	77: arrowCalloutRule([]float64{7200, 5400, 3600, 8100}, false, false, false),
	78: arrowCalloutRule([]float64{14400, 5400, 18000, 8100}, false, true, false),
	79: arrowCalloutRule([]float64{7200, 5400, 3600, 8100}, true, false, false),
	80: arrowCalloutRule([]float64{14400, 5400, 18000, 8100}, true, true, false),
	81: arrowCalloutRule([]float64{5400, 5500, 2700, 8100}, false, false, true),
	82: arrowCalloutRule([]float64{5400, 5500, 2700, 8100}, true, false, true),
	83: {defaults: []float64{5400, 8100, 2700, 9400}, conv: func(a []float64, _ box) []float64 {
		return []float64{frac(geoSize - 2*a[3]), frac(geoSize/2 - a[1]), frac(a[2]), frac(geoSize - 2*a[0])}
	}},
}

// fixedDegree is one degree in 16.16 bits, the unit of an angle of Office
// Drawing.
const fixedDegree = 1 << 16

// visual returns the angle a in degrees of a point of the ellipse with the
// radii rx and ry, as the angle of ECMA-376 of the direction of the point
// from the center, from 0 to 360 degrees, in 60000ths of a degree. Office
// Drawing measures the angle on the circle of its geometry, which the box
// stretches, and ECMA-376 measures the direction in the box.
func visual(a, rx, ry float64) float64 {
	s, c := math.Sincos(a * math.Pi / 180)
	d := math.Atan2(ry*s, rx*c) * 180 / math.Pi
	if d < 0 {
		d += 360
	}
	return d * 60000
}

// arcSpan returns the start and the sweep of the arc of Office Drawing
// with the adjust values a, in radians, clockwise. The same start and end
// make a whole ellipse, as in ECMA-376.
func arcSpan(a []float64) (st, sw float64) {
	st = a[0] / fixedDegree * math.Pi / 180
	sw = math.Mod(a[1]/fixedDegree*math.Pi/180-st, 2*math.Pi)
	if !(sw > 0) {
		sw += 2 * math.Pi
	}
	return st, sw
}

// minArcSpan bounds the width and the height of an arc, as a share of the
// diameter of its circle, so that a flat arc keeps a finite frame.
const minArcSpan = 1.0 / 64

// arcFrame returns the frame of the ellipse of an arc of Office Drawing.
// The arc and the center of its circle fill the box of the shape, so the
// circle is larger than the box: the default arc is a quarter of the
// ellipse whose center is the bottom left corner.
func arcFrame(a []float64, b box) frame {
	st, sw := arcSpan(a)
	x0, y0, x1, y1 := arcBounds(st, sw)
	rx := b.w / max(x1-x0, 2*minArcSpan)
	ry := b.h / max(y1-y0, 2*minArcSpan)
	return frame{x: -rx * (x0 + 1), y: -ry * (y0 + 1), w: 2 * rx, h: 2 * ry}
}

// arcBounds returns the bounds of the arc of the unit circle from the
// angle st over the sweep sw, clockwise with y down, with the center of the
// circle, as the box of an arc of Office Drawing holds its center.
func arcBounds(st, sw float64) (x0, y0, x1, y1 float64) {
	add := func(t float64) {
		s, c := math.Sincos(t)
		x0, y0, x1, y1 = min(x0, c), min(y0, s), max(x1, c), max(y1, s)
	}
	add(st)
	add(st + sw)
	for k := math.Ceil(st / (math.Pi / 2)); k*math.Pi/2 < st+sw; k++ {
		add(k * math.Pi / 2)
	}
	return x0, y0, x1, y1
}

// arcAngles converts the angles of an arc of Office Drawing to the
// directions in its frame.
func arcAngles(a []float64, b box) []float64 {
	f := arcFrame(a, b)
	st, sw := arcSpan(a)
	deg := 180 / math.Pi
	return []float64{visual(st*deg, f.w, f.h), visual((st+sw)*deg, f.w, f.h)}
}

// circularArrow converts the adjust values of a circular arrow of Office
// Drawing. Its ring has the outer radius 10800 and the inner radius a[2].
// The base of the head is at the end angle, 2700 wider than the ring on
// each side, and the tip is as far from the base as the base is from its
// middle. The head reaches 2700 out of the box, so the frame is larger
// than the box by an eighth of its shorter side on each side, 27000 in the
// units of the geometry. The values of ECMA-376 are the width of the ring,
// the angle of the head, the end and the start angles, and the half width
// of the head, the width and the half width as shares of the frame.
func circularArrow(a []float64, b box) []float64 {
	const size = 27000
	th := min(max(geoSize/2-a[2], 0), geoSize/2)
	hw := th/2 + geoSize/8
	head := math.Atan2(hw, geoSize/2-th/2) * 180 / math.Pi
	st, en := a[0]/fixedDegree, a[1]/fixedDegree
	if math.Mod(en-st, 360) == 0 {
		// the ring goes all the way round, and ECMA-376 needs room for
		// the head before the start
		en = st - head - 1
	}
	return []float64{
		th / size * 100000, head * 60000,
		visual(en, b.w, b.h), visual(st, b.w, b.h),
		hw / size * 100000,
	}
}

// curvedRule returns the rule of a curved arrow with the defaults a1, a2
// and a3. Along the x axis, the head is at the right or at the left, and
// vert puts it at the top or at the bottom. The head is from a1 to the
// side, a2 is the outer side of the shaft at the head, and a3 is the base
// of the head. far is whether the head is at the right or at the bottom.
func curvedRule(a1, a2, a3 float64, vert, far bool) adjustRule {
	return adjustRule{defaults: []float64{a1, a2, a3}, conv: func(a []float64, b box) []float64 {
		across, along := b.yss, b.xss
		if vert {
			across, along = b.xss, b.yss
		}
		head := a[2]
		if far {
			head = geoSize - head
		}
		return []float64{across(2*a[1] - a[0] - geoSize), across(geoSize - a[0]), along(head)}
	}}
}

// arrowCalloutRule returns the rule of an arrow callout with the
// defaults. Along the x axis, the values are the x of the side of the box
// at the head, the y of the top of the head, the x of the base of the head
// and the y of the top of the shaft. far is whether the head is at the
// right, or at the bottom when vert is set. both is whether the callout
// has a head at each end.
func arrowCalloutRule(defaults []float64, vert, far, both bool) adjustRule {
	return adjustRule{defaults: defaults, conv: func(a []float64, b box) []float64 {
		across, along := b.yss, b.xss
		if vert {
			across, along = b.xss, b.yss
		}
		head, side := a[2], geoSize-a[0]
		switch {
		case both:
			side = geoSize - 2*a[0]
		case far:
			head, side = geoSize-a[2], a[0]
		}
		return []float64{across(geoSize - 2*a[3]), across(geoSize/2 - a[1]), along(head), frac(side)}
	}}
}

// fracs converts each value as a share of a side.
func fracs(a []float64, _ box) []float64 {
	out := make([]float64, len(a))
	for i, v := range a {
		out[i] = frac(v)
	}
	return out
}

// waves converts the height of a wave and its shift from the center.
func waves(a []float64, _ box) []float64 {
	return []float64{frac(a[0]), center(a[1])}
}

// brackets converts the end of a bracket, as a share of the shorter side,
// and the middle of a brace, as a share of the height.
func brackets(a []float64, b box) []float64 {
	out := []float64{b.yss(a[0])}
	if len(a) > 1 {
		out = append(out, frac(a[1]))
	}
	return out
}

// adjustOf returns the adjust values of the preset of the shape type typ
// with the properties c, in a box of w by h points, as guides of ECMA-376
// with the names of the preset. It returns nil for the defaults of
// ECMA-376.
func adjustOf(typ int, c Chain, names []string, w, h float64) []drawingml.Guide {
	var guides []drawingml.Guide
	for i, v := range adjustValues(typ, c, w, h) {
		if i >= len(names) || math.IsNaN(v) || math.IsInf(v, 0) {
			continue
		}
		guides = append(guides, drawingml.AdjustValue(names[i], v))
	}
	return guides
}

// adjustValues returns the adjust values of ECMA-376 of the shape type typ
// with the properties c, in a box of w by h points, in the order of the
// preset. A value that is NaN keeps the default of ECMA-376. It returns nil
// for a type without a rule.
func adjustValues(typ int, c Chain, w, h float64) []float64 {
	r, a, b, ok := officeValues(typ, c, w, h)
	if !ok {
		return nil
	}
	return r.conv(a, b)
}

// frameOf returns the frame of the preset of the shape type typ with the
// properties c, in a box of w by h points.
func frameOf(typ int, c Chain, w, h float64) frame {
	r, a, b, ok := officeValues(typ, c, w, h)
	if !ok || r.place == nil {
		return frame{w: w, h: h}
	}
	f := r.place(a, b)
	if !(f.w > 0) || !(f.h > 0) || math.IsInf(f.w+f.h+f.x+f.y, 0) || math.IsNaN(f.x+f.y) {
		return frame{w: w, h: h}
	}
	return f
}

// officeValues returns the rule of the shape type typ, the adjust values of
// Office Drawing of the properties c, with the defaults of the type, and
// the box of w by h points. It returns false for a type without a rule or
// an empty box.
func officeValues(typ int, c Chain, w, h float64) (adjustRule, []float64, box, bool) {
	r, ok := adjustRules[typ]
	b := box{w: w, h: h, ss: min(w, h)}
	if !ok || !(b.ss > 0) {
		return r, nil, b, false
	}
	a := make([]float64, len(r.defaults))
	for i, def := range r.defaults {
		a[i] = def
		if v, ok := c.Get(PropAdjust1 + uint16(i)); ok {
			a[i] = float64(int32(v))
		}
	}
	return r, a, b, true
}
