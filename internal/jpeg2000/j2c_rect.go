// Copyright (c) The Hayro Authors
//
// Use of this source code is governed by the Apache-2.0 or MIT license that
// can be found in the LICENSE-APACHE and LICENSE-MIT files.

package jpeg2000

// This file ports hayro-jpeg2000 src/j2c/rect.rs.

// intRect is IntRect in hayro-jpeg2000 src/j2c/rect.rs. The arithmetic on
// its sides wraps, as it does upstream in a build for release.
type intRect struct {
	x0, y0, x1, y1 uint32
}

// intRectFromLTRB is IntRect::from_ltrb.
func intRectFromLTRB(x0, y0, x1, y1 uint32) intRect {
	return intRect{x0: x0, y0: y0, x1: x1, y1: y1}
}

// intRectFromXYWH is IntRect::from_xywh.
func intRectFromXYWH(x, y, w, h uint32) intRect {
	return intRect{x0: x, y0: y, x1: x + w, y1: y + h}
}

// width is IntRect::width.
func (r intRect) width() uint32 {
	return r.x1 - r.x0
}

// height is IntRect::height.
func (r intRect) height() uint32 {
	return r.y1 - r.y0
}

// area is IntRect::area.
func (r intRect) area() uint64 {
	return uint64(r.width()) * uint64(r.height())
}

// intersect is IntRect::intersect.
func (r intRect) intersect(other intRect) intRect {
	if r.x1 < other.x0 || other.x1 < r.x0 || r.y1 < other.y0 || other.y1 < r.y0 {
		return intRectFromXYWH(0, 0, 0, 0)
	}
	return intRectFromLTRB(max(r.x0, other.x0), max(r.y0, other.y0), min(r.x1, other.x1), min(r.y1, other.y1))
}
