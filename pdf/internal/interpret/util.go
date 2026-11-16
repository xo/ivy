// Copyright (c) The Hayro Authors.
//
// Use of this source code is governed by the Apache-2.0 or MIT license that
// can be found in the LICENSE-APACHE and LICENSE-MIT files.

// This file ports hayro-interpret src/util.rs.

package interpret

import (
	"cmp"
	"encoding/binary"
	"hash"
	"hash/fnv"
	"math"

	"github.com/xo/okage/kurbo"

	"github.com/xo/ivy/pdf/internal/kurboext"
	"github.com/xo/ivy/pdf/internal/pdffont"
	"github.com/xo/ivy/pdf/internal/syntax"
)

// scalarNearlyZero is SCALAR_NEARLY_ZERO in hayro-interpret src/util.rs.
const scalarNearlyZero = 1.0 / (1 << 8)

// Float is the set of the types of Float32Ext and Float64Ext in
// hayro-interpret src/util.rs.
//
// Port: upstream has a trait for f32 and one for f64 with the same methods.
// Go has one generic function for each method.
type Float interface {
	~float32 | ~float64
}

// IsNearlyZero is Float32Ext::is_nearly_zero and Float64Ext::is_nearly_zero
// in hayro-interpret src/util.rs. It reports whether x is approximately 0.
func IsNearlyZero[F Float](x F) bool {
	return IsNearlyZeroWithinTolerance(x, F(scalarNearlyZero))
}

// IsNearlyEqual is Float32Ext::is_nearly_equal in hayro-interpret
// src/util.rs. It reports whether x is nearly equal to other.
func IsNearlyEqual[F Float](x, other F) bool {
	return IsNearlyZero(x - other)
}

// IsNearlyLessOrEqual is Float32Ext::is_nearly_less_or_equal in
// hayro-interpret src/util.rs.
func IsNearlyLessOrEqual[F Float](x, other F) bool {
	return IsNearlyZero(x-other) || x < other
}

// IsNearlyGreaterOrEqual is Float32Ext::is_nearly_greater_or_equal in
// hayro-interpret src/util.rs.
func IsNearlyGreaterOrEqual[F Float](x, other F) bool {
	return IsNearlyZero(x-other) || x > other
}

// IsNearlyZeroWithinTolerance is Float32Ext::is_nearly_zero_within_tolerance
// in hayro-interpret src/util.rs. It reports whether x is within tolerance
// of 0. tolerance must not be negative.
func IsNearlyZeroWithinTolerance[F Float](x, tolerance F) bool {
	if x < 0 {
		x = -x
	}
	return x <= tolerance
}

// pointNearlySame is PointExt::nearly_same in hayro-interpret src/util.rs.
// It compares the coordinates as f32.
func pointNearlySame(p, other kurbo.Point) bool {
	return IsNearlyEqual(float32(p.X), float32(other.X)) && IsNearlyEqual(float32(p.Y), float32(other.Y))
}

// Key128 is the u128 of the cache keys in hayro-interpret src/util.rs and
// src/cache.rs.
//
// Port: upstream hashes with SipHash-1-3 into a u128. Go hashes with FNV-1a
// into 16 bytes, the type that pdffont uses for its keys. The keys differ
// from upstream, but they play the same role.
type Key128 = pdffont.Key128

// keyHasher is the state of hash128 in hayro-interpret src/util.rs. It
// hashes a sequence of values into a Key128.
type keyHasher struct {
	h   hash.Hash
	buf [8]byte
}

// newKeyHasher returns an empty hasher.
func newKeyHasher() *keyHasher {
	return &keyHasher{h: fnv.New128a()}
}

// u64 adds a number.
func (k *keyHasher) u64(v uint64) *keyHasher {
	binary.LittleEndian.PutUint64(k.buf[:], v)
	_, _ = k.h.Write(k.buf[:])
	return k
}

// f32 adds the bits of a number.
func (k *keyHasher) f32(v float32) *keyHasher {
	return k.u64(uint64(math.Float32bits(v)))
}

// f64 adds the bits of a number.
func (k *keyHasher) f64(v float64) *keyHasher {
	return k.u64(math.Float64bits(v))
}

// boolean adds a bool.
func (k *keyHasher) boolean(v bool) *keyHasher {
	if v {
		return k.u64(1)
	}
	return k.u64(0)
}

// bytes adds a slice of bytes with its length.
func (k *keyHasher) bytes(b []byte) *keyHasher {
	k.u64(uint64(len(b)))
	_, _ = k.h.Write(b)
	return k
}

// str adds a string with its length.
func (k *keyHasher) str(s string) *keyHasher {
	return k.bytes([]byte(s))
}

// key adds another key.
func (k *keyHasher) key(v Key128) *keyHasher {
	_, _ = k.h.Write(v[:])
	return k
}

// sum returns the key.
func (k *keyHasher) sum() Key128 {
	var out Key128
	k.h.Sum(out[:0])
	return out
}

// fastBoundingBox is BezPathExt::fast_bounding_box in hayro-interpret
// src/util.rs. It returns the bounding box of the points of the path,
// control points included.
func fastBoundingBox(path *kurboext.BezPath) kurbo.Rect {
	minX, minY := math.Inf(1), math.Inf(1)
	maxX, maxY := math.Inf(-1), math.Inf(-1)
	include := func(p kurbo.Point) {
		minX = rustMin(minX, p.X)
		minY = rustMin(minY, p.Y)
		maxX = rustMax(maxX, p.X)
		maxY = rustMax(maxY, p.Y)
	}
	for _, el := range path.Elements() {
		switch el.Kind {
		case kurbo.PathElMoveTo, kurbo.PathElLineTo:
			include(el.P[0])
		case kurbo.PathElQuadTo:
			include(el.P[0])
			include(el.P[1])
		case kurbo.PathElCurveTo:
			include(el.P[0])
			include(el.P[1])
			include(el.P[2])
		}
	}
	if minX > maxX {
		return kurboext.RectZero
	}
	return kurbo.NewRect(minX, minY, maxX, maxY)
}

// TransformToKurbo is TransformExt::to_kurbo in hayro-interpret
// src/util.rs. It converts a transform of hayro-syntax into an Affine.
func TransformToKurbo(t syntax.Transform) kurbo.Affine {
	return kurbo.NewAffine(t.AsCoeffs())
}

// RectToKurbo is RectExt::to_kurbo in hayro-interpret src/util.rs. It
// converts a rectangle of hayro-syntax into a Rect of kurbo.
func RectToKurbo(r syntax.Rect) kurbo.Rect {
	return kurbo.NewRect(r.X0, r.Y0, r.X1, r.Y1)
}

// XYAdvances is x_y_advances in hayro-interpret src/util.rs. It returns the
// unit x and y vectors transformed by the transform, without its
// translation.
func XYAdvances(transform kurbo.Affine) (kurbo.Vec2, kurbo.Vec2) {
	scaleSkew := kurbo.NewAffine([6]float64{transform[0], transform[1], transform[2], transform[3], 0, 0})
	x := scaleSkew.MulPoint(kurbo.NewPoint(1, 0))
	y := scaleSkew.MulPoint(kurbo.NewPoint(0, 1))
	return kurbo.NewVec2(x.X, x.Y), kurbo.NewVec2(y.X, y.Y)
}

// The helpers below give Go the semantics of the arithmetic of Rust. They
// are not ports of an item of upstream.

// rustMin is f64::min of Rust. If one value is NaN, it returns the other.
func rustMin[F Float](a, b F) F {
	switch {
	case isNaN(a):
		return b
	case isNaN(b):
		return a
	case b < a:
		return b
	}
	return a
}

// rustMax is f64::max of Rust. If one value is NaN, it returns the other.
func rustMax[F Float](a, b F) F {
	switch {
	case isNaN(a):
		return b
	case isNaN(b):
		return a
	case b > a:
		return b
	}
	return a
}

// rustClamp is f32::clamp of Rust. A NaN stays NaN.
func rustClamp[F Float](x, lo, hi F) F {
	if x < lo {
		x = lo
	}
	if x > hi {
		x = hi
	}
	return x
}

// asU8 is the cast `as u8` of Rust from a float: it saturates, and NaN
// gives 0.
func asU8[F Float](x F) uint8 {
	switch {
	case isNaN(x) || x <= 0:
		return 0
	case x >= 255:
		return 255
	}
	return uint8(x)
}

// asU32 is the cast `as u32` of Rust from a float.
func asU32[F Float](x F) uint32 {
	switch {
	case isNaN(x) || x <= 0:
		return 0
	case float64(x) >= math.MaxUint32:
		return math.MaxUint32
	}
	return uint32(x)
}

// asI32 is the cast `as i32` of Rust from a float.
func asI32[F Float](x F) int32 {
	switch {
	case isNaN(x):
		return 0
	case float64(x) <= math.MinInt32:
		return math.MinInt32
	case float64(x) >= math.MaxInt32:
		return math.MaxInt32
	}
	return int32(x)
}

// asUsize is the cast `as usize` of Rust from a float, for a target of 64
// bits. A value past the range of int gives the largest int.
func asUsize[F Float](x F) int {
	switch {
	case isNaN(x) || x <= 0:
		return 0
	case float64(x) >= math.MaxInt:
		return math.MaxInt
	}
	return int(x)
}

// isNaN is f32::is_nan of Rust.
func isNaN[F Float](x F) bool {
	return math.IsNaN(float64(x))
}

// powf32 is f32::powf of Rust.
func powf32(x, y float32) float32 {
	return float32(math.Pow(float64(x), float64(y)))
}

// totalCmpF32 is f32::total_cmp of Rust: a total order of the floats, with
// the negative NaNs first and the positive NaNs last.
func totalCmpF32(a, b float32) int {
	ai := int32(math.Float32bits(a))
	bi := int32(math.Float32bits(b))
	ai ^= int32(uint32(ai>>31) >> 1)
	bi ^= int32(uint32(bi>>31) >> 1)
	return cmp.Compare(ai, bi)
}

// totalCmpF64 is f64::total_cmp of Rust.
func totalCmpF64(a, b float64) int {
	ai := int64(math.Float64bits(a))
	bi := int64(math.Float64bits(b))
	ai ^= int64(uint64(ai>>63) >> 1)
	bi ^= int64(uint64(bi>>63) >> 1)
	return cmp.Compare(ai, bi)
}

// partitionPoint is slice::partition_point of Rust, on the indices below n.
// It follows the binary search of Rust step by step, so that a slice that
// is not partitioned gives the same index as upstream.
func partitionPoint(n int, pred func(i int) bool) int {
	if n == 0 {
		return 0
	}
	base, size := 0, n
	for size > 1 {
		half := size / 2
		mid := base + half
		if pred(mid) {
			base = mid
		}
		size -= half
	}
	if pred(base) {
		return base + 1
	}
	return base
}
