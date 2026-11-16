// Copyright (c) The Hayro Authors
//
// Use of this source code is governed by the Apache-2.0 or MIT license that
// can be found in the LICENSE-APACHE and LICENSE-MIT files.

// This file ports hayro-syntax src/object/number.rs.

package syntax

import (
	"errors"
	"math"
	"strconv"
	"strings"
)

// powersOf10 is POWERS_OF_10 in hayro-syntax src/object/number.rs.
var powersOf10 = [23]float64{
	1.0, 1e1, 1e2, 1e3, 1e4, 1e5, 1e6, 1e7, 1e8, 1e9,
	1e10, 1e11, 1e12, 1e13, 1e14, 1e15, 1e16, 1e17, 1e18, 1e19,
	1e20, 1e21, 1e22,
}

// maxExactMantissa is MAX_EXACT_MANTISSA in hayro-syntax
// src/object/number.rs.
const maxExactMantissa uint64 = 1 << 53

// Number is Number in hayro-syntax src/object/number.rs. It holds a real
// number or an integer.
//
// Port: the enum InternalNumber becomes the field isReal. Two numbers are
// equal with == when upstream finds them equal.
type Number struct {
	real    float64
	integer int64
	isReal  bool
}

// NumberZero is Number::ZERO in hayro-syntax src/object/number.rs.
var NumberZero = NumberFromI32(0)

// NumberOne is Number::ONE in hayro-syntax src/object/number.rs.
var NumberOne = NumberFromI32(1)

// AsF64 is Number::as_f64 in hayro-syntax src/object/number.rs.
func (n Number) AsF64() float64 {
	if n.isReal {
		return n.real
	}
	return float64(n.integer)
}

// AsF32 is Number::as_f32 in hayro-syntax src/object/number.rs.
func (n Number) AsF32() float32 {
	if n.isReal {
		return float32(n.real)
	}
	return float32(n.integer)
}

// AsI64 is Number::as_i64 in hayro-syntax src/object/number.rs. A real
// number saturates, as the cast of Rust does.
func (n Number) AsI64() int64 {
	if n.isReal {
		return f64ToI64(n.real)
	}
	return n.integer
}

// NumberFromF32 is Number::from_f32 in hayro-syntax src/object/number.rs.
func NumberFromF32(v float32) Number {
	return Number{real: float64(v), isReal: true}
}

// NumberFromI32 is Number::from_i32 in hayro-syntax src/object/number.rs.
func NumberFromI32(v int32) Number {
	return Number{integer: int64(v)}
}

// String returns the Display text of upstream.
func (n Number) String() string {
	if n.isReal {
		return formatF64(n.real)
	}
	return strconv.FormatInt(n.integer, 10)
}

// GoString returns the Debug text of upstream.
func (n Number) GoString() string {
	if n.isReal {
		return "Number(Real(" + formatF64(n.real) + "))"
	}
	return "Number(Integer(" + strconv.FormatInt(n.integer, 10) + "))"
}

// skipNumber is the Skippable implementation of Number in hayro-syntax
// src/object/number.rs.
func skipNumber(r *Reader) bool {
	hasSign := r.ForwardIf(func(b byte) bool { return b == '+' || b == '-' })
	b, ok := r.PeekByte()
	if !ok {
		return false
	}
	// Some PDFs have weird trailing minuses, so try to accept those as well.
	switch {
	case b == '.':
		r.Forward()
		// See PDFJS-9252: treat a single . as 0.
		r.ForwardWhile(isDigitOrMinus)
	case isDigit(b) || b == '-':
		if !r.ForwardWhile1(isDigitOrMinus) {
			return false
		}
		if r.ForwardTag(".") {
			r.ForwardWhile(isDigitOrMinus)
		}
	case hasSign:
		// See PDFJS-bug1753983 and PDFJS-bug1953099: accept just + or - as
		// a zero.
	default:
		return false
	}
	// See issue 994. Do not accept numbers that a regular character follows.
	if next, ok := r.PeekByte(); ok && isRegularCharacter(next) {
		return false
	}
	return true
}

// readNumber is the Readable implementation of Number in hayro-syntax
// src/object/number.rs.
func readNumber(r *Reader) (Number, bool) {
	old := r.offset
	if n, ok := readSimpleNumber(r); ok {
		return n, true
	}
	r.Jump(old)
	if n, ok := readInnerNumber(r); ok {
		return n, true
	}
	r.Jump(old)
	return Number{}, false
}

// readSimpleNumber is Number::read_simple in hayro-syntax
// src/object/number.rs. It reads a number of at most 18 digits. It does not
// move the reader back on a failure.
func readSimpleNumber(r *Reader) (Number, bool) {
	negative := false
	if b, ok := r.PeekByte(); ok {
		switch b {
		case '-':
			r.Forward()
			negative = true
		case '+':
			r.Forward()
		}
	}
	var mantissa uint64
	readDigits := func() int {
		start := r.offset
		for r.offset >= 0 && r.offset < len(r.Data) {
			d := r.Data[r.offset] - '0'
			if d >= 10 {
				break
			}
			mantissa = mantissa*10 + uint64(d)
			r.offset++
		}
		return r.offset - start
	}
	digits := readDigits()
	shift, hasShift := 0, false
	if b, ok := r.PeekByte(); ok && b == '.' {
		r.Forward()
		fractionDigits := readDigits()
		shift, hasShift = fractionDigits, true
		digits += fractionDigits
	}
	if digits == 0 || digits > 18 {
		return Number{}, false
	}
	if b, ok := r.PeekByte(); ok && isRegularCharacter(b) {
		return Number{}, false
	}
	return numberFromParts(negative, mantissa, shift, hasShift)
}

// numberFromParts is Number::from_parts in hayro-syntax
// src/object/number.rs.
func numberFromParts(negative bool, mantissa uint64, shift int, hasShift bool) (Number, bool) {
	if !hasShift {
		value := int64(mantissa)
		if negative {
			value = -value
		}
		return Number{integer: value}, true
	}
	if mantissa > maxExactMantissa || shift >= len(powersOf10) {
		return Number{}, false
	}
	value := float64(mantissa) / powersOf10[shift]
	if negative {
		value = -value
	}
	return Number{real: value, isReal: true}, true
}

// readInnerNumber is read_inner in hayro-syntax src/object/number.rs. It
// reads a number of any length.
func readInnerNumber(r *Reader) (Number, bool) {
	start := r.offset
	b, ok := r.PeekByte()
	if !ok {
		return Number{}, false
	}
	hasSign, negative := false, false
	switch b {
	case '-':
		r.Forward()
		hasSign, negative = true, true
	case '+':
		r.Forward()
		hasSign = true
	}
	var mantissa uint64
	hasDot := false
	decimalShift := 0
	hasDigits := false
	overflowed := false
	numberEnd := -1
loop:
	for {
		b, ok := r.PeekByte()
		switch {
		case ok && isDigit(b):
			r.Forward()
			if !overflowed {
				d := uint64(b - '0')
				if mantissa > (math.MaxUint64-d)/10 {
					overflowed = true
				} else {
					mantissa = mantissa*10 + d
				}
			}
			hasDigits = true
			if hasDot {
				decimalShift++
			}
		case ok && b == '.' && !hasDot:
			r.Forward()
			hasDot = true
		case ok && b == '-' && hasDigits:
			// Some weird PDFs have a trailing minus in the fraction of a
			// number.
			numberEnd = r.offset
			r.Forward()
			r.ForwardWhile(isDigitOrMinus)
			break loop
		default:
			break loop
		}
	}
	if !hasDigits {
		next, ok := r.PeekByte()
		if hasDot || (hasSign && ok && !isRegularCharacter(next)) {
			// Treat numbers like just -, + or -. as zero.
			return Number{}, true
		}
		return Number{}, false
	}
	// See issue 994. Do not accept numbers that a regular character follows.
	if next, ok := r.PeekByte(); ok && isRegularCharacter(next) {
		return Number{}, false
	}
	integerOverflow := !hasDot && mantissa > math.MaxInt64
	if !overflowed && !integerOverflow {
		if n, ok := numberFromParts(negative, mantissa, decimalShift, hasDot); ok {
			return n, true
		}
	}
	end := numberEnd
	if end < 0 {
		end = r.offset
	}
	raw, ok := r.Range(start, end)
	if !ok {
		return Number{}, false
	}
	s := string(raw)
	if !hasDot {
		if v, err := strconv.ParseInt(s, 10, 64); err == nil {
			return Number{integer: v}, true
		}
	}
	v, err := strconv.ParseFloat(s, 64)
	if err != nil && !errors.Is(err, strconv.ErrRange) {
		return Number{}, false
	}
	return Number{real: v, isReal: true}, true
}

// integer lists the integer types that a Number converts into. The type int
// stands for usize of upstream.
type integer interface {
	~int | ~int32 | ~int64 | ~uint8 | ~uint16 | ~uint32
}

// i64ToInteger converts v as TryFrom<i64> of Rust does. It fails when the
// value does not fit. The type int fails for a negative value, as usize
// does.
func i64ToInteger[I integer](v int64) (I, bool) {
	x := I(v)
	if int64(x) != v {
		return 0, false
	}
	if _, isUsize := any(x).(int); isUsize && v < 0 {
		return 0, false
	}
	return x, true
}

// skipInteger is the Skippable implementation of the integer types in
// hayro-syntax src/object/number.rs.
func skipInteger(r *Reader) bool {
	r.ForwardIf(func(b byte) bool { return b == '+' || b == '-' })
	if !r.ForwardWhile1(isDigit) {
		return false
	}
	if b, ok := r.PeekByte(); ok && (b == '.' || isRegularCharacter(b)) {
		return false
	}
	return true
}

// readInteger is the Readable implementation of the integer types in
// hayro-syntax src/object/number.rs.
func readInteger[I integer](r *Reader, ctx *ReaderContext) (I, bool) {
	n, ok := r.Read[Number](ctx)
	if !ok {
		return 0, false
	}
	return i64ToInteger[I](n.AsI64())
}

// castInteger is the TryFrom<Object> implementation of the integer types in
// hayro-syntax src/object/number.rs.
func castInteger[I integer](o Object) (I, bool) {
	n, ok := o.(Number)
	if !ok {
		return 0, false
	}
	return i64ToInteger[I](n.AsI64())
}

// isDigit is is_digit in hayro-syntax src/object/number.rs.
func isDigit(b byte) bool {
	return b >= '0' && b <= '9'
}

// isDigitOrMinus is is_digit_or_minus in hayro-syntax src/object/number.rs.
func isDigitOrMinus(b byte) bool {
	return isDigit(b) || b == '-'
}

// f64ToI64 converts v as the cast "as i64" of Rust does: it saturates, and
// NaN becomes 0.
//
// Port: a conversion of Go is not defined for a value out of range.
func f64ToI64(v float64) int64 {
	switch {
	case math.IsNaN(v):
		return 0
	case v >= math.MaxInt64:
		return math.MaxInt64
	case v <= math.MinInt64:
		return math.MinInt64
	}
	return int64(v)
}

// f32ToU32 converts v as the cast "as u32" of Rust does.
func f32ToU32(v float32) uint32 {
	switch {
	case math.IsNaN(float64(v)) || v <= 0:
		return 0
	case v >= math.MaxUint32:
		return math.MaxUint32
	}
	return uint32(v)
}

// formatF64 formats v as the Debug text of f64 in Rust: the shortest text
// that reads back as v, with ".0" for a whole number, and an exponent below
// 1e-4 and from 1e16.
func formatF64(v float64) string {
	switch {
	case math.IsNaN(v):
		return "NaN"
	case math.IsInf(v, 1):
		return "inf"
	case math.IsInf(v, -1):
		return "-inf"
	}
	a := math.Abs(v)
	if v != 0 && (a < 1e-4 || a >= 1e16) {
		s := strconv.FormatFloat(v, 'e', -1, 64)
		mant, exp, _ := strings.Cut(s, "e")
		e, _ := strconv.Atoi(exp)
		return mant + "e" + strconv.Itoa(e)
	}
	s := strconv.FormatFloat(v, 'f', -1, 64)
	if !strings.Contains(s, ".") {
		s += ".0"
	}
	return s
}
