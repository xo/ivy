// Copyright 2018 the Kurbo Authors
//
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

// This file ports the items of kurbo kurbo/src/rect.rs that the port of
// hayro needs and that the subset of kurbo in okage does not hold.

package kurboext

import (
	"math"

	"github.com/xo/okage/kurbo"
)

// RectZero is Rect::ZERO in kurbo kurbo/src/rect.rs.
var RectZero = kurbo.Rect{}

// RectMinX is Rect::min_x in kurbo kurbo/src/rect.rs. It returns the smaller
// x of the rectangle.
func RectMinX(r kurbo.Rect) float64 {
	return rustMin(r.X0, r.X1)
}

// RectMaxX is Rect::max_x in kurbo kurbo/src/rect.rs. It returns the larger
// x of the rectangle.
func RectMaxX(r kurbo.Rect) float64 {
	return rustMax(r.X0, r.X1)
}

// RectMinY is Rect::min_y in kurbo kurbo/src/rect.rs. It returns the smaller
// y of the rectangle.
func RectMinY(r kurbo.Rect) float64 {
	return rustMin(r.Y0, r.Y1)
}

// RectMaxY is Rect::max_y in kurbo kurbo/src/rect.rs. It returns the larger
// y of the rectangle.
func RectMaxY(r kurbo.Rect) float64 {
	return rustMax(r.Y0, r.Y1)
}

// RectArea is Rect::area in kurbo kurbo/src/rect.rs.
func RectArea(r kurbo.Rect) float64 {
	return r.Width() * r.Height()
}

// RectIsZeroArea is Rect::is_zero_area in kurbo kurbo/src/rect.rs.
func RectIsZeroArea(r kurbo.Rect) bool {
	return RectArea(r) == 0.0
}

// RectIntersect is Rect::intersect in kurbo kurbo/src/rect.rs. It returns
// the intersection of two rectangles. Two rectangles that do not meet give
// a rectangle of zero area.
func RectIntersect(r, other kurbo.Rect) kurbo.Rect {
	x0 := rustMax(r.X0, other.X0)
	y0 := rustMax(r.Y0, other.Y0)
	x1 := rustMin(r.X1, other.X1)
	y1 := rustMin(r.Y1, other.Y1)
	return kurbo.NewRect(x0, y0, rustMax(x1, x0), rustMax(y1, y0))
}

// RectOverlaps is Rect::overlaps in kurbo kurbo/src/rect.rs. It reports
// whether the two rectangles share an area.
func RectOverlaps(r, other kurbo.Rect) bool {
	return r.X0 < other.X1 && r.X1 > other.X0 && r.Y0 < other.Y1 && r.Y1 > other.Y0
}

// RectContains is Rect::contains in kurbo kurbo/src/rect.rs. It reports
// whether the point is in the rectangle. The left and the top edges are in
// the rectangle, the right and the bottom edges are not.
func RectContains(r kurbo.Rect, p kurbo.Point) bool {
	return p.X >= r.X0 && p.X < r.X1 && p.Y >= r.Y0 && p.Y < r.Y1
}

// RectContainsRect is Rect::contains_rect in kurbo kurbo/src/rect.rs. It
// reports whether other is in r.
func RectContainsRect(r, other kurbo.Rect) bool {
	return r.X0 <= other.X0 && r.Y0 <= other.Y0 && r.X1 >= other.X1 && r.Y1 >= other.Y1
}

// RectInflate is Rect::inflate in kurbo kurbo/src/rect.rs. It moves each
// edge of the rectangle out by width or by height.
func RectInflate(r kurbo.Rect, width, height float64) kurbo.Rect {
	x0, x1 := r.X0+width, r.X1-width
	if r.X0 < r.X1 {
		x0, x1 = r.X0-width, r.X1+width
	}
	y0, y1 := r.Y0+height, r.Y1-height
	if r.Y0 < r.Y1 {
		y0, y1 = r.Y0-height, r.Y1+height
	}
	return kurbo.NewRect(x0, y0, x1, y1)
}

// RectExpand is Rect::expand in kurbo kurbo/src/rect.rs. It rounds each
// edge of the rectangle out to an integer.
func RectExpand(r kurbo.Rect) kurbo.Rect {
	x0, x1 := math.Ceil(r.X0), math.Floor(r.X1)
	if r.X0 < r.X1 {
		x0, x1 = math.Floor(r.X0), math.Ceil(r.X1)
	}
	y0, y1 := math.Ceil(r.Y0), math.Floor(r.Y1)
	if r.Y0 < r.Y1 {
		y0, y1 = math.Floor(r.Y0), math.Ceil(r.Y1)
	}
	return kurbo.NewRect(x0, y0, x1, y1)
}

// RectToPath is Shape::to_path for Rect in kurbo kurbo/src/shape.rs. It
// returns the outline of the rectangle as a path.
func RectToPath(r kurbo.Rect, tolerance float64) BezPath {
	return BezPath{elements: r.PathElements(tolerance)}
}

// rustMin is f64::min of Rust. If one value is NaN, it returns the other.
func rustMin(a, b float64) float64 {
	switch {
	case math.IsNaN(a):
		return b
	case math.IsNaN(b):
		return a
	}
	return math.Min(a, b)
}

// rustMax is f64::max of Rust. If one value is NaN, it returns the other.
func rustMax(a, b float64) float64 {
	switch {
	case math.IsNaN(a):
		return b
	case math.IsNaN(b):
		return a
	}
	return math.Max(a, b)
}
