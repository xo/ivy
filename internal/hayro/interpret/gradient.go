// Copyright (c) The Hayro Authors.
//
// Use of this source code is governed by the Apache-2.0 or MIT license that
// can be found in the LICENSE-APACHE and LICENSE-MIT files.

// This file ports hayro-interpret src/gradient.rs.

package interpret

import (
	"math"
	"math/bits"
	"slices"

	"github.com/xo/okage/kurbo"
)

// The constants of hayro-interpret src/gradient.rs.
const (
	maxGradientSubdivisionDepth = 10
	radialEpsilon               = 1.0e-6
	gradientPadding             = 0.01
	lutTolerance                = 0.01
	minLutSize                  = 256
	maxLutSize                  = 4096
)

// SVGGradient is SvgGradient in hayro-interpret src/gradient.rs. It is a
// gradient like those of SVG that approximates a shading.
type SVGGradient struct {
	// Kind is the kind of the gradient.
	Kind SVGGradientKind
	// Transform maps the coordinates of the encoded shading into the space
	// of the renderer.
	Transform kurbo.Affine
	// Stops are the stops of the gradient, in sRGB.
	Stops []SVGGradientStop
	// Decal is true when the gradient paints nothing before its first stop
	// and after its last stop.
	//
	// Port: upstream has no such field. See AsSVGGradient.
	Decal bool
}

// SVGGradientKind is SvgGradientKind in hayro-interpret src/gradient.rs. It
// is an SVGGradientLinear or an SVGGradientRadial.
//
// Port: upstream is an enum. Go uses an interface that only the two types
// implement.
type SVGGradientKind interface {
	isSVGGradientKind()
}

// SVGGradientLinear is SvgGradientKind::Linear in hayro-interpret
// src/gradient.rs.
type SVGGradientLinear struct {
	// Start is the normalized start point of the line of the gradient.
	Start kurbo.Point
	// End is the normalized end point of the line of the gradient.
	End kurbo.Point
}

// SVGGradientRadial is SvgGradientKind::Radial in hayro-interpret
// src/gradient.rs.
type SVGGradientRadial struct {
	// StartCenter is the center of the start circle.
	StartCenter kurbo.Point
	// StartRadius is the radius of the start circle.
	StartRadius float32
	// EndCenter is the center of the end circle.
	EndCenter kurbo.Point
	// EndRadius is the radius of the end circle.
	EndRadius float32
}

func (SVGGradientLinear) isSVGGradientKind() {}
func (SVGGradientRadial) isSVGGradientKind() {}

// SVGGradientStop is SvgGradientStop in hayro-interpret src/gradient.rs.
type SVGGradientStop struct {
	// Offset is the normalized offset.
	Offset float32
	// Color holds the components of RGBA in sRGB.
	Color [4]float32
}

// AsSVGGradient is EncodedRadialAxialShading::as_svg_gradient in
// hayro-interpret src/gradient.rs. It approximates the shading with a
// gradient like those of SVG, and returns false when no such gradient fits.
//
// Port: a shading with no extend at either end and no background gives a
// gradient with Decal set, and no padding. okage paints nothing past the
// ends of such a gradient (okage D55). Upstream pads the ends with a short
// stretch of transparent stops.
func (s *EncodedRadialAxialShading) AsSVGGradient(pattern *EncodedShadingPattern, tolerance float32) (*SVGGradient, bool) {
	bg := pattern.backgroundColor.Components()
	var background *[4]float32
	if bg[3] > 0 {
		f := pattern.finish(bg)
		background = &f
	}
	decal := background == nil && !s.extend[0] && !s.extend[1]
	rng, ok := s.paddedRange(background != nil, decal)
	if !ok {
		return nil, false
	}
	stops := approximateGradientStops(
		func(t float32) [4]float32 { return sampleFunction(&s.function, s.domain, pattern, t) },
		normalizedStitchingBounds(&s.function, s.domain),
		tolerance,
		pattern.run,
	)
	return &SVGGradient{
		Kind:      s.gradientKind(rng),
		Transform: pattern.BaseTransform.Inverse(),
		Stops:     padStops(stops, rng, background),
		Decal:     decal,
	}, true
}

// paddedRange is EncodedRadialAxialShading::padded_range in hayro-interpret
// src/gradient.rs. A gradient of SVG pads with its end stops, so an end
// without an extend gets a short stretch of the background.
//
// Port: a decal gradient gets no padding. See AsSVGGradient.
func (s *EncodedRadialAxialShading) paddedRange(hasBackground, decal bool) ([2]float64, bool) {
	var padding [2]float64
	for i, e := range s.extend {
		if !e && !decal {
			padding[i] = gradientPadding
		}
	}
	if s.params.radial {
		p1, r := s.params.p1, s.params.r
		growth := r.Y - r.X
		nested := math.Abs(growth)-p1.ToVec2().Hypot() > radialEpsilon
		// Unless the circles nest, the circles past the end outrank the
		// earlier ones, and some points need the background.
		if r.X < 0 || r.Y < 0 || !nested && (hasBackground || !s.extend[1]) {
			return [2]float64{}, false
		}
		// The padded circles must not have a negative radius.
		padding[0] = rustMin(padding[0], r.X/rustMax(growth, 0))
		padding[1] = rustMin(padding[1], r.Y/rustMax(-growth, 0))
	}
	return [2]float64{-padding[0], 1 + padding[1]}, true
}

// gradientKind is EncodedRadialAxialShading::gradient_kind in
// hayro-interpret src/gradient.rs.
func (s *EncodedRadialAxialShading) gradientKind(rng [2]float64) SVGGradientKind {
	t0, t1 := rng[0], rng[1]
	if !s.params.radial {
		return SVGGradientLinear{Start: kurbo.NewPoint(t0, 0), End: kurbo.NewPoint(t1, 0)}
	}
	p1, r := s.params.p1, s.params.r
	return SVGGradientRadial{
		StartCenter: p1.ToVec2().Mul(t0).ToPoint(),
		StartRadius: float32(r.X + (r.Y-r.X)*t0),
		EndCenter:   p1.ToVec2().Mul(t1).ToPoint(),
		EndRadius:   float32(r.X + (r.Y-r.X)*t1),
	}
}

// functionLut is function_lut in hayro-interpret src/gradient.rs.
func functionLut(function *ShadingFunction, rng [2]float32, pattern *EncodedShadingPattern) *colorLut {
	return newColorLut(functionStops(function, rng, pattern))
}

// functionStops returns the stops that function_lut in hayro-interpret
// src/gradient.rs puts in its table.
//
// Port: upstream has no such function. A meshEmitter cuts the triangles of
// a mesh at the stops.
func functionStops(function *ShadingFunction, rng [2]float32, pattern *EncodedShadingPattern) []SVGGradientStop {
	return approximateGradientStops(
		func(t float32) [4]float32 { return sampleFunction(function, rng, pattern, t) },
		normalizedStitchingBounds(function, rng),
		lutTolerance,
		pattern.run,
	)
}

// sampleFunction is sample_function in hayro-interpret src/gradient.rs.
func sampleFunction(function *ShadingFunction, rng [2]float32, pattern *EncodedShadingPattern, t float32) [4]float32 {
	return pattern.eval(function, []float32{rng[0] + (rng[1]-rng[0])*t})
}

// normalizedStitchingBounds is normalized_stitching_bounds in
// hayro-interpret src/gradient.rs.
func normalizedStitchingBounds(function *ShadingFunction, rng [2]float32) []float32 {
	t0, t1 := rng[0], rng[1]
	dt := t1 - t0
	if abs32(dt) <= epsilonF32 {
		return nil
	}
	var bounds []float32
	for _, b := range function.stitchingBounds() {
		n := (b - t0) / dt
		if n > 0 && n < 1 {
			bounds = append(bounds, n)
		}
	}
	if dt < 0 {
		slices.Reverse(bounds)
	}
	return bounds
}

// colorLut is ColorLut in hayro-interpret src/gradient.rs. It is a table of
// colors along a gradient.
type colorLut struct {
	colors [][4]float32
}

// newColorLut is ColorLut::new in hayro-interpret src/gradient.rs.
func newColorLut(stops []SVGGradientStop) *colorLut {
	minGap := float32(1)
	for i := 0; i+1 < len(stops); i++ {
		if gap := stops[i+1].Offset - stops[i].Offset; gap > 0 {
			minGap = rustMin(minGap, gap)
		}
	}
	size := nextPowerOfTwo(asUsize(math.Ceil(float64(1 / minGap))))
	size = max(minLutSize, min(size, maxLutSize))
	colors := make([][4]float32, size)
	if len(stops) == 0 {
		// Port: upstream never makes a table without stops.
		return &colorLut{colors: colors}
	}
	segment := 0
	for i := range colors {
		t := float32(i) / float32(size-1)
		for segment+2 < len(stops) && stops[segment+1].Offset < t {
			segment++
		}
		start, end := stops[segment], stops[min(segment+1, len(stops)-1)]
		width := end.Offset - start.Offset
		local := float32(1)
		if width > 0 {
			local = rustClamp((t-start.Offset)/width, 0, 1)
		}
		colors[i] = lerpColor(start.Color, end.Color, local)
	}
	return &colorLut{colors: colors}
}

// nextPowerOfTwo is usize::next_power_of_two of Rust.
func nextPowerOfTwo(n int) int {
	if n <= 1 {
		return 1
	}
	if n > 1<<62 {
		return 1 << 62
	}
	return 1 << bits.Len(uint(n-1))
}

// get is ColorLut::get in hayro-interpret src/gradient.rs.
func (l *colorLut) get(t float32) [4]float32 {
	last := len(l.colors) - 1
	return l.colors[min(asUsize(t*float32(last)+0.5), last)]
}

// padStops is pad_stops in hayro-interpret src/gradient.rs. It maps the
// stops into the padded range, and adds the background at the padded
// ends.
func padStops(stops []SVGGradientStop, rng [2]float64, background *[4]float32) []SVGGradientStop {
	t0, t1 := float32(rng[0]), float32(rng[1])
	padding := func(offset float32) SVGGradientStop {
		s := SVGGradientStop{Offset: offset}
		if background != nil {
			s.Color = *background
		}
		return s
	}
	for i := range stops {
		stops[i].Offset = (stops[i].Offset - t0) / (t1 - t0)
	}
	if len(stops) == 0 {
		return stops
	}
	if t0 < 0 {
		edge := stops[0].Offset
		stops = append([]SVGGradientStop{padding(0), padding(edge)}, stops...)
	}
	if t1 > 1 {
		edge := stops[len(stops)-1].Offset
		stops = append(stops, padding(edge), padding(1))
	}
	return stops
}

// approximateGradientStops is approximate_gradient_stops in hayro-interpret
// src/gradient.rs. It places stops at the breakpoints, and between them
// where the colors do not vary linearly.
//
// Port: run lets the approximation stop with the interpretation.
func approximateGradientStops(sample func(float32) [4]float32, breakpoints []float32, tolerance float32, run *runState) []SVGGradientStop {
	intervalBounds := make([]float32, 0, len(breakpoints)+2)
	intervalBounds = append(intervalBounds, 0)
	intervalBounds = append(intervalBounds, breakpoints...)
	intervalBounds = append(intervalBounds, 1)
	var stops []SVGGradientStop
	for i := 0; i+1 < len(intervalBounds); i++ {
		startOffset, endOffset := intervalBounds[i], intervalBounds[i+1]
		startColor := sample(startOffset)
		stops = append(stops, SVGGradientStop{Offset: startOffset, Color: startColor})
		if run.canceled() {
			break
		}
		endColor := sample(endOffset)
		approximateGradientInterval(sample, &stops,
			gradientSample{offset: startOffset, color: startColor},
			gradientSample{offset: endOffset, color: endColor},
			rustMax(tolerance, 0), 0)
	}
	return stops
}

// gradientSample is GradientSample in hayro-interpret src/gradient.rs.
type gradientSample struct {
	offset float32
	color  [4]float32
}

// approximateGradientInterval is approximate_gradient_interval in
// hayro-interpret src/gradient.rs.
func approximateGradientInterval(sample func(float32) [4]float32, stops *[]SVGGradientStop, start, end gradientSample, tolerance float32, depth uint8) {
	delta := end.offset - start.offset
	quarterOffset := start.offset + delta*0.25
	midOffset := start.offset + delta*0.5
	threeQuarterOffset := start.offset + delta*0.75
	quarterColor := sample(quarterOffset)
	midColor := sample(midOffset)
	threeQuarterColor := sample(threeQuarterOffset)
	if colorError(quarterColor, lerpColor(start.color, end.color, 0.25)) <= tolerance &&
		colorError(midColor, lerpColor(start.color, end.color, 0.5)) <= tolerance &&
		colorError(threeQuarterColor, lerpColor(start.color, end.color, 0.75)) <= tolerance ||
		depth == maxGradientSubdivisionDepth {
		*stops = append(*stops, SVGGradientStop{Offset: end.offset, Color: end.color})
		return
	}
	mid := gradientSample{offset: midOffset, color: midColor}
	approximateGradientInterval(sample, stops, start, mid, tolerance, depth+1)
	approximateGradientInterval(sample, stops, mid, end, tolerance, depth+1)
}
