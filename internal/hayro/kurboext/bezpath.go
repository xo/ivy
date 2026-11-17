// Copyright 2018 the Kurbo Authors
//
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

// This file ports the items of kurbo kurbo/src/bezpath.rs that the port of
// hayro needs and that the subset of kurbo in okage does not hold.

// Package kurboext holds the items of the Rust crate kurbo that the port of
// hayro needs, and that the subset of kurbo in okage does not hold. The
// package github.com/xo/okage/kurbo holds the rest.
//
// The items follow kurbo 0.13. kurbo is under the MIT license or the
// Apache-2.0 license. LICENSE holds the MIT license of kurbo.
package kurboext

import "github.com/xo/okage/kurbo"

// BezPath is BezPath in kurbo kurbo/src/bezpath.rs. It is a path of Bezier
// curves, as a list of path elements.
type BezPath struct {
	elements []kurbo.PathEl
}

// NewBezPath is BezPath::new in kurbo kurbo/src/bezpath.rs. It returns an
// empty path.
func NewBezPath() BezPath {
	return BezPath{}
}

// BezPathFromElements is BezPath::from_vec in kurbo kurbo/src/bezpath.rs. It
// returns a path that holds the elements. The path takes the slice.
func BezPathFromElements(elements []kurbo.PathEl) BezPath {
	return BezPath{elements: elements}
}

// MoveTo is BezPath::move_to in kurbo kurbo/src/bezpath.rs. It starts a new
// subpath at p.
func (b *BezPath) MoveTo(p kurbo.Point) {
	b.elements = append(b.elements, kurbo.MoveTo(p))
}

// LineTo is BezPath::line_to in kurbo kurbo/src/bezpath.rs. It adds a line
// to p.
func (b *BezPath) LineTo(p kurbo.Point) {
	b.elements = append(b.elements, kurbo.LineTo(p))
}

// QuadTo is BezPath::quad_to in kurbo kurbo/src/bezpath.rs. It adds a
// quadratic Bezier curve.
func (b *BezPath) QuadTo(p1, p2 kurbo.Point) {
	b.elements = append(b.elements, kurbo.QuadTo(p1, p2))
}

// CurveTo is BezPath::curve_to in kurbo kurbo/src/bezpath.rs. It adds a
// cubic Bezier curve.
func (b *BezPath) CurveTo(p1, p2, p3 kurbo.Point) {
	b.elements = append(b.elements, kurbo.CurveTo(p1, p2, p3))
}

// ClosePath is BezPath::close_path in kurbo kurbo/src/bezpath.rs. It closes
// the current subpath.
func (b *BezPath) ClosePath() {
	b.elements = append(b.elements, kurbo.ClosePath())
}

// Push is BezPath::push in kurbo kurbo/src/bezpath.rs. It adds one element.
func (b *BezPath) Push(el kurbo.PathEl) {
	b.elements = append(b.elements, el)
}

// Extend is the implementation of Extend<PathEl> for BezPath in kurbo
// kurbo/src/bezpath.rs. It adds the elements to the end of the path.
func (b *BezPath) Extend(elements []kurbo.PathEl) {
	b.elements = append(b.elements, elements...)
}

// Truncate is BezPath::truncate in kurbo kurbo/src/bezpath.rs. It keeps the
// first n elements of the path.
func (b *BezPath) Truncate(n int) {
	if n < len(b.elements) {
		b.elements = b.elements[:n]
	}
}

// Elements is BezPath::elements in kurbo kurbo/src/bezpath.rs. It returns
// the elements of the path. The caller must not change them.
func (b BezPath) Elements() []kurbo.PathEl {
	return b.elements
}

// Len returns the number of elements of the path.
func (b BezPath) Len() int {
	return len(b.elements)
}

// Clone is the derived Clone of BezPath in kurbo kurbo/src/bezpath.rs. It
// returns a path that shares no memory with b.
func (b BezPath) Clone() BezPath {
	if b.elements == nil {
		return BezPath{}
	}
	return BezPath{elements: append([]kurbo.PathEl(nil), b.elements...)}
}

// IsEmpty is BezPath::is_empty in kurbo kurbo/src/bezpath.rs. It reports
// whether the path has no element that draws.
func (b BezPath) IsEmpty() bool {
	for _, el := range b.elements {
		if el.Kind != kurbo.PathElMoveTo && el.Kind != kurbo.PathElClosePath {
			return false
		}
	}
	return true
}

// Segments is BezPath::segments in kurbo kurbo/src/bezpath.rs. It returns
// the segments of the path.
func (b BezPath) Segments() []kurbo.PathSeg {
	return kurbo.Segments(b.elements)
}

// HasSegments reports whether BezPath::segments in kurbo
// kurbo/src/bezpath.rs gives at least one segment. A path has a segment
// when an element after the first one is not a MoveTo, unless it is a
// ClosePath that ends at the start of its subpath.
func (b BezPath) HasSegments() bool {
	return len(b.Segments()) > 0
}

// BoundingBox is the implementation of Shape::bounding_box for BezPath in
// kurbo kurbo/src/bezpath.rs. It returns the smallest rectangle that holds
// each segment of the path, or the zero rectangle for a path without a
// segment.
func (b BezPath) BoundingBox() kurbo.Rect {
	var bbox kurbo.Rect
	have := false
	for _, seg := range b.Segments() {
		segBbox := PathSegBoundingBox(seg)
		if have {
			bbox = bbox.Union(segBbox)
		} else {
			bbox, have = segBbox, true
		}
	}
	return bbox
}

// MulBezPath is the implementation of Mul<BezPath> for Affine in kurbo
// kurbo/src/bezpath.rs. It returns the path transformed by a.
func MulBezPath(a kurbo.Affine, b BezPath) BezPath {
	if b.elements == nil {
		return BezPath{}
	}
	out := make([]kurbo.PathEl, len(b.elements))
	for i, el := range b.elements {
		out[i] = a.MulPathEl(el)
	}
	return BezPath{elements: out}
}

// PathSegBoundingBox is the implementation of
// ParamCurveExtrema::bounding_box for PathSeg in kurbo kurbo/src/bezpath.rs.
func PathSegBoundingBox(s kurbo.PathSeg) kurbo.Rect {
	switch s.Kind {
	case kurbo.PathSegLine:
		return kurbo.RectFromPoints(s.Line.P0, s.Line.P1)
	case kurbo.PathSegQuad:
		return s.Quad.BoundingBox()
	}
	return CubicBezBoundingBox(s.Cubic)
}
