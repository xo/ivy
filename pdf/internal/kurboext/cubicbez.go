// Copyright 2018 the Kurbo Authors
//
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

// This file ports the items of kurbo kurbo/src/cubicbez.rs and
// kurbo/src/param_curve.rs that the port of hayro needs and that the subset
// of kurbo in okage does not hold.

package kurboext

import (
	"slices"

	"github.com/xo/okage/kurbo"
)

// CubicBezExtrema is the implementation of ParamCurveExtrema::extrema for
// CubicBez in kurbo kurbo/src/cubicbez.rs. It returns the parameters in
// (0, 1) where the curve has an extremum in x or in y, in order.
func CubicBezExtrema(c kurbo.CubicBez) []float64 {
	result := make([]float64, 0, 4)
	oneCoord := func(d0, d1, d2 float64) {
		a := d0 - 2.0*d1 + d2
		b := 2.0 * (d1 - d0)
		for _, t := range kurbo.SolveQuadratic(d0, b, a) {
			if t > 0.0 && t < 1.0 {
				result = append(result, t)
			}
		}
	}
	d0 := c.P1.Sub(c.P0)
	d1 := c.P2.Sub(c.P1)
	d2 := c.P3.Sub(c.P2)
	oneCoord(d0.X, d1.X, d2.X)
	oneCoord(d0.Y, d1.Y, d2.Y)
	slices.Sort(result)
	return result
}

// CubicBezBoundingBox is ParamCurveExtrema::bounding_box in kurbo
// kurbo/src/param_curve.rs, for CubicBez. It returns the smallest rectangle
// that holds the curve.
func CubicBezBoundingBox(c kurbo.CubicBez) kurbo.Rect {
	bbox := kurbo.RectFromPoints(c.P0, c.P3)
	for _, t := range CubicBezExtrema(c) {
		bbox = bbox.UnionPt(c.Eval(t))
	}
	return bbox
}
