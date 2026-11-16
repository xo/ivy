// Copyright (c) The Hayro Authors
//
// Use of this source code is governed by the Apache-2.0 or MIT license that
// can be found in the LICENSE-APACHE and LICENSE-MIT files.

package jpeg2000

// This file ports hayro-jpeg2000 src/math.rs.
//
// Port: upstream holds two versions of the type f32x8, one on fearless_simd
// and one in scalar code. The port has neither. Each function that uses
// f32x8 upstream runs the scalar arithmetic of the scalar version on each
// sample, in a loop of its own (D9). The functions below that upstream
// has outside the two modules keep their names.

import "math"

// simdWidth is SIMD_WIDTH: the number of samples of one f32x8.
const simdWidth = 8

// mulAdd is mul_add in hayro-jpeg2000 src/math.rs. It returns a*b + c.
//
// Port: upstream uses a fused multiply-add on a target with FMA. The port
// takes the scalar path of upstream. The conversion of the product to
// float32 keeps the Go compiler from fusing the two operations, so the
// result is the same on each architecture.
func mulAdd(a, b, c float32) float32 {
	return float32(a*b) + c
}

// floorF32 is floor_f32 in hayro-jpeg2000 src/math.rs.
func floorF32(x float32) float32 {
	return float32(math.Floor(float64(x)))
}

// roundToU8 is round_to_u8 in hayro-jpeg2000 src/math.rs.
func roundToU8(x float32) uint8 {
	return f32ToU8(x + 0.5)
}

// pow2i is pow2i in hayro-jpeg2000 src/math.rs.
func pow2i(exp int32) float32 {
	if exp >= 0 {
		return float32(shl32(1, uint32(exp)))
	}
	return 1.0 / float32(shl32(1, uint32(-exp)))
}

// minF32 is min_f32 in hayro-jpeg2000 src/math.rs. Like f32::min of Rust,
// it returns the other value when one value is NaN.
func minF32(a, b float32) float32 {
	switch {
	case a != a:
		return b
	case b != b:
		return a
	case a < b:
		return a
	}
	return b
}

// maxF32 is max_f32 in hayro-jpeg2000 src/math.rs. Like f32::max of Rust,
// it returns the other value when one value is NaN.
func maxF32(a, b float32) float32 {
	switch {
	case a != a:
		return b
	case b != b:
		return a
	case a > b:
		return a
	}
	return b
}

// roundAndInterleave is round_and_interleave in hayro-jpeg2000 src/math.rs,
// in its scalar version. It writes the samples of the channels to out, one
// pixel after the other.
func roundAndInterleave(channels [][]float32, scale float32, out []byte) {
	n := len(channels)
	for i := 0; (i+1)*n <= len(out); i++ {
		for c, channel := range channels {
			out[i*n+c] = roundToU8(channel[i] * scale)
		}
	}
}

// simdBuffer is SimdBuffer in hayro-jpeg2000 src/math.rs. It holds samples
// with padding to a multiple of simdWidth.
type simdBuffer struct {
	data        []float32
	originalLen int
}

// newSimdBuffer is SimdBuffer::new.
func newSimdBuffer(data []float32) simdBuffer {
	originalLen := len(data)
	padded := simdPaddedLen(originalLen)
	if padded > originalLen {
		if cap(data) >= padded {
			data = data[:padded]
			clear(data[originalLen:])
		} else {
			grown := make([]float32, padded)
			copy(grown, data)
			data = grown
		}
	}
	return simdBuffer{data: data, originalLen: originalLen}
}

// simdBufferZeros is SimdBuffer::zeros.
func simdBufferZeros(originalLen int) simdBuffer {
	return simdBuffer{data: make([]float32, simdPaddedLen(originalLen)), originalLen: originalLen}
}

// truncated is SimdBuffer::truncated. It returns the samples without the
// padding.
func (b simdBuffer) truncated() []float32 {
	return b.data[:b.originalLen]
}

// simdPaddedLen is SimdBuffer::padded_len. Like upstream, it adds a full
// block of padding when the length is a multiple of simdWidth.
func simdPaddedLen(originalLen int) int {
	return originalLen + simdWidth - originalLen%simdWidth
}

// The functions below give the conversions and the shifts of Rust in a
// build for release, which ivy keeps (docs/UPSTREAM.md).

// f32ToU8 is the conversion `x as u8` of Rust: it saturates, and NaN gives
// zero.
func f32ToU8(x float32) uint8 {
	switch {
	case x != x || x <= 0:
		return 0
	case x >= 255:
		return 255
	}
	return uint8(x)
}

// f32ToI64 is the conversion `x as i64` of Rust: it saturates, and NaN
// gives zero.
func f32ToI64(x float32) int64 {
	switch {
	case x != x:
		return 0
	case x >= 9.223372e18:
		return math.MaxInt64
	case x <= -9.223372e18:
		return math.MinInt64
	}
	return int64(x)
}

// shl32 is `x << n` of Rust on u32 in a build for release: the shift takes
// n modulo 32.
func shl32(x, n uint32) uint32 {
	return x << (n & 31)
}

// shlI32 is `x << n` of Rust on i32 in a build for release: the shift takes
// n modulo 32.
func shlI32(x int32, n uint32) int32 {
	return x << (n & 31)
}

// divCeil32 is u32::div_ceil of Rust. It does not overflow.
func divCeil32(a, b uint32) uint32 {
	q := a / b
	if a%b != 0 {
		q++
	}
	return q
}

// divCeil64 is u64::div_ceil of Rust. It does not overflow.
func divCeil64(a, b uint64) uint64 {
	q := a / b
	if a%b != 0 {
		q++
	}
	return q
}

// nextMultipleOf32 is u32::next_multiple_of of Rust in a build for
// release: the addition wraps.
func nextMultipleOf32(x, rhs uint32) uint32 {
	if r := x % rhs; r != 0 {
		return x + (rhs - r)
	}
	return x
}

// checkedNextMultipleOf32 is u32::checked_next_multiple_of of Rust. It
// returns false on an overflow.
func checkedNextMultipleOf32(x, rhs uint32) (uint32, bool) {
	r := x % rhs
	if r == 0 {
		return x, true
	}
	sum := uint64(x) + uint64(rhs-r)
	if sum > math.MaxUint32 {
		return 0, false
	}
	return uint32(sum), true
}
