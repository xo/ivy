// Copyright (c) The Hayro Authors
//
// Use of this source code is governed by the Apache-2.0 or MIT license that
// can be found in the LICENSE-APACHE and LICENSE-MIT files.

// This file ports hayro-syntax src/object/rect.rs.

package syntax

import "math"

// Rect is Rect in hayro-syntax src/object/rect.rs. It is a rectangle.
type Rect struct {
	// X0 is the minimum x coordinate.
	X0 float64
	// Y0 is the minimum y coordinate.
	Y0 float64
	// X1 is the maximum x coordinate.
	X1 float64
	// Y1 is the maximum y coordinate.
	Y1 float64
}

// RectZero is Rect::ZERO in hayro-syntax src/object/rect.rs.
var RectZero = NewRect(0, 0, 0, 0)

// NewRect is Rect::new in hayro-syntax src/object/rect.rs.
func NewRect(x0, y0, x1, y1 float64) Rect {
	return Rect{X0: x0, Y0: y0, X1: x1, Y1: y1}
}

// Intersect is Rect::intersect in hayro-syntax src/object/rect.rs.
func (r Rect) Intersect(other Rect) Rect {
	x0 := rustMax(r.X0, other.X0)
	y0 := rustMax(r.Y0, other.Y0)
	x1 := rustMin(r.X1, other.X1)
	y1 := rustMin(r.Y1, other.Y1)
	return NewRect(x0, y0, rustMax(x1, x0), rustMax(y1, y0))
}

// Width is Rect::width in hayro-syntax src/object/rect.rs.
func (r Rect) Width() float64 {
	return r.X1 - r.X0
}

// Height is Rect::height in hayro-syntax src/object/rect.rs.
func (r Rect) Height() float64 {
	return r.Y1 - r.Y0
}

// readRect is the Readable implementation of Rect in hayro-syntax
// src/object/rect.rs.
func readRect(r *Reader, ctx *ReaderContext) (Rect, bool) {
	a, ok := r.Read[Array](ctx)
	if !ok {
		return Rect{}, false
	}
	return rectFromArr(a)
}

// rectFromArr is from_arr in hayro-syntax src/object/rect.rs.
func rectFromArr(a Array) (Rect, bool) {
	it := a.Iter[float32]()
	var v [4]float64
	for i := range v {
		f, ok := it.Next()
		if !ok {
			return Rect{}, false
		}
		v[i] = float64(f)
	}
	x0, y0, x1, y1 := v[0], v[1], v[2], v[3]
	return NewRect(rustMin(x0, x1), rustMin(y0, y1), rustMax(x1, x0), rustMax(y1, y0)), true
}

// rustMax is f64::max of Rust: NaN loses to a number.
func rustMax(a, b float64) float64 {
	switch {
	case math.IsNaN(a):
		return b
	case math.IsNaN(b):
		return a
	}
	return math.Max(a, b)
}

// rustMin is f64::min of Rust: NaN loses to a number.
func rustMin(a, b float64) float64 {
	switch {
	case math.IsNaN(a):
		return b
	case math.IsNaN(b):
		return a
	}
	return math.Min(a, b)
}
