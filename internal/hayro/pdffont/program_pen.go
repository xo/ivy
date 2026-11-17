// Copyright (c) The Hayro Authors.
//
// Use of this source code is governed by the Apache-2.0 or MIT license that
// can be found in the LICENSE-APACHE and LICENSE-MIT files.

// This file ports hayro-font src/program/pen.rs.

package pdffont

import (
	"github.com/xo/okage/kurbo"

	"github.com/xo/ivy/internal/hayro/kurboext"
)

// outlinePath is OutlinePath in hayro-font src/program/pen.rs. It collects
// the outline of a glyph into a BezPath of kurboext. A line, a curve or a
// close before the first point is dropped.
//
// Port: upstream implements the trait PathSink of skrifa. The readers of
// this package call the methods directly.
type outlinePath struct {
	path kurboext.BezPath
}

// newOutlinePath is OutlinePath::new in hayro-font src/program/pen.rs.
func newOutlinePath() *outlinePath {
	return &outlinePath{path: kurboext.NewBezPath()}
}

// take is OutlinePath::take in hayro-font src/program/pen.rs.
func (o *outlinePath) take() kurboext.BezPath {
	return o.path
}

// moveTo is PathSink::move_to of OutlinePath in hayro-font
// src/program/pen.rs.
func (o *outlinePath) moveTo(x, y float32) {
	o.path.MoveTo(pt32(x, y))
}

// lineTo is PathSink::line_to of OutlinePath in hayro-font
// src/program/pen.rs.
func (o *outlinePath) lineTo(x, y float32) {
	if o.path.Len() != 0 {
		o.path.LineTo(pt32(x, y))
	}
}

// quadTo is PathSink::quad_to of OutlinePath in hayro-font
// src/program/pen.rs.
func (o *outlinePath) quadTo(cx, cy, x, y float32) {
	if o.path.Len() != 0 {
		o.path.QuadTo(pt32(cx, cy), pt32(x, y))
	}
}

// curveTo is PathSink::curve_to of OutlinePath in hayro-font
// src/program/pen.rs.
func (o *outlinePath) curveTo(cx0, cy0, cx1, cy1, x, y float32) {
	if o.path.Len() != 0 {
		o.path.CurveTo(pt32(cx0, cy0), pt32(cx1, cy1), pt32(x, y))
	}
}

// closePath is PathSink::close of OutlinePath in hayro-font
// src/program/pen.rs.
func (o *outlinePath) closePath() {
	if o.path.Len() != 0 {
		o.path.ClosePath()
	}
}

// pt32 is the conversion of a pair of f32 into a Point of kurbo.
func pt32(x, y float32) kurbo.Point {
	return kurbo.NewPoint(float64(x), float64(y))
}
