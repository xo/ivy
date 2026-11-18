package odraw

import (
	"math"

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
	r, ok := adjustRules[typ]
	if !ok {
		return nil
	}
	b := box{w: w, h: h, ss: min(w, h)}
	if !(b.ss > 0) {
		return nil
	}
	a := make([]float64, len(r.defaults))
	for i, def := range r.defaults {
		a[i] = def
		if v, ok := c.Get(PropAdjust1 + uint16(i)); ok {
			a[i] = float64(int32(v))
		}
	}
	return r.conv(a, b)
}
