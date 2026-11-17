// Copyright (c) The Hayro Authors.
//
// Use of this source code is governed by the Apache-2.0 or MIT license that
// can be found in the LICENSE-APACHE and LICENSE-MIT files.

// This file ports no upstream file. It converts the geometry of kurbo, which
// interpret uses, to the geometry of okage, which the display device takes.
// Upstream needs no such code, because vello_cpu takes kurbo.

package render

import (
	"math"

	"github.com/xo/okage"
	"github.com/xo/okage/kurbo"

	"github.com/xo/ivy/internal/hayro/kurboext"
)

// toTransform converts an affine transform of kurbo to a transform of okage.
func toTransform(a kurbo.Affine) okage.Transform {
	return okage.TransformFromRow(float32(a[0]), float32(a[1]), float32(a[2]), float32(a[3]), float32(a[4]), float32(a[5]))
}

// invertible reports whether a has an inverse that is finite.
func invertible(a kurbo.Affine) bool {
	det := a.Determinant()
	return det != 0 && !math.IsNaN(det) && !math.IsInf(det, 0) && a.IsFinite() && a.Inverse().IsFinite()
}

// toPath converts a path of kurbo to a path of okage. It returns nil for a
// path without a segment.
func toPath(p *kurboext.BezPath) *okage.Path {
	if p == nil {
		return nil
	}
	return toPathElements(p.Elements())
}

// toPathElements converts the elements of a path of kurbo to a path of
// okage, or nil.
func toPathElements(els []kurbo.PathEl) *okage.Path {
	pb := okage.NewPathBuilderWithCapacity(len(els), len(els)*3)
	f := func(v float64) float32 { return float32(v) }
	for _, el := range els {
		switch el.Kind {
		case kurbo.PathElMoveTo:
			pb.MoveTo(f(el.P[0].X), f(el.P[0].Y))
		case kurbo.PathElLineTo:
			pb.LineTo(f(el.P[0].X), f(el.P[0].Y))
		case kurbo.PathElQuadTo:
			pb.QuadTo(f(el.P[0].X), f(el.P[0].Y), f(el.P[1].X), f(el.P[1].Y))
		case kurbo.PathElCurveTo:
			pb.CubicTo(f(el.P[0].X), f(el.P[0].Y), f(el.P[1].X), f(el.P[1].Y), f(el.P[2].X), f(el.P[2].Y))
		case kurbo.PathElClosePath:
			pb.Close()
		}
	}
	path, ok := pb.Finish()
	if !ok {
		return nil
	}
	return path
}

// rectPath returns the path of a rectangle of kurbo, or nil for an empty
// one.
func rectPath(r kurbo.Rect) *okage.Path {
	return toPathElements(r.PathElements(0.1))
}

// controlBox is BezPath::control_box of kurbo. It returns the smallest
// rectangle that holds each point of p, with the control points.
func controlBox(p *kurboext.BezPath) kurbo.Rect {
	first := true
	var r kurbo.Rect
	for _, el := range p.Elements() {
		var n int
		switch el.Kind {
		case kurbo.PathElMoveTo, kurbo.PathElLineTo:
			n = 1
		case kurbo.PathElQuadTo:
			n = 2
		case kurbo.PathElCurveTo:
			n = 3
		}
		for _, pt := range el.P[:n] {
			if first {
				r = kurbo.NewRect(pt.X, pt.Y, pt.X, pt.Y)
				first = false
				continue
			}
			r = r.UnionPt(pt)
		}
	}
	return r
}
