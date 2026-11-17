// Copyright (c) The Hayro Authors.
//
// Use of this source code is governed by the Apache-2.0 or MIT license that
// can be found in the LICENSE-APACHE and LICENSE-MIT files.

// This file ports hayro-postscript src/number.rs.

package postscript

import (
	"errors"
	"math"
	"strconv"
)

// Number is Number in hayro-postscript src/number.rs. It is a PostScript
// number object, an integer or a real number.
//
// Port: upstream is an enum with the variants Integer and Real. Go holds
// both values in one struct. IsReal tells which value is set.
type Number struct {
	// Integer is the value of Number::Integer.
	Integer int64
	// Real is the value of Number::Real.
	Real float64
	// IsReal is true for Number::Real and false for Number::Integer.
	IsReal bool
}

// NewInteger is Number::Integer in hayro-postscript src/number.rs.
func NewInteger(v int64) Number {
	return Number{Integer: v}
}

// NewReal is Number::Real in hayro-postscript src/number.rs.
func NewReal(v float64) Number {
	return Number{Real: v, IsReal: true}
}

// Float64 is Number::as_f64 in hayro-postscript src/number.rs. It returns the
// value as a float64.
func (n Number) Float64() float64 {
	if n.IsReal {
		return n.Real
	}
	return float64(n.Integer)
}

// isToken marks Number as a Token.
func (Number) isToken() {}

// isASCIIDigit is u8::is_ascii_digit of the Rust core library.
func isASCIIDigit(b byte) bool {
	return '0' <= b && b <= '9'
}

// isASCIIAlphanumeric is u8::is_ascii_alphanumeric of the Rust core library.
func isASCIIAlphanumeric(b byte) bool {
	return isASCIIDigit(b) || 'a' <= b && b <= 'z' || 'A' <= b && b <= 'Z'
}

// isDigitInBase is char::is_digit of the Rust core library, for an ASCII
// byte and a base from 2 to 36.
func isDigitInBase(b byte, base int) bool {
	var v int
	switch {
	case isASCIIDigit(b):
		v = int(b - '0')
	case 'a' <= b && b <= 'z':
		v = int(b-'a') + 10
	case 'A' <= b && b <= 'Z':
		v = int(b-'A') + 10
	default:
		return false
	}
	return v < base
}

// parseNumber is parse in hayro-postscript src/number.rs. It reads all of
// token as a number. The token can be an integer, a real number, or an
// integer with a base, such as 16#FFFE.
func parseNumber(token []byte) (Number, error) {
	r := newReader(token)

	first, ok := r.peekByte()
	if !ok {
		return Number{}, ErrSyntax
	}
	hasSign := first == '+' || first == '-'
	if hasSign {
		r.forward()
	}

	digitStart := r.getOffset()
	r.forwardWhile(isASCIIDigit)
	hasDigits := r.getOffset() > digitStart

	if b, ok := r.peekByte(); !hasSign && hasDigits && ok && b == '#' {
		return parseRadix(r, digitStart)
	}

	hasDot := false
	if b, ok := r.peekByte(); ok && b == '.' {
		hasDot = true
		r.forward()
		r.forwardWhile(isASCIIDigit)
	}

	if !hasDigits && !hasDot {
		return Number{}, ErrSyntax
	}

	hasExponent := false
	if b, ok := r.peekByte(); ok && (b == 'e' || b == 'E') {
		hasExponent = true
		r.forward()
		if b, ok := r.peekByte(); ok && (b == '+' || b == '-') {
			r.forward()
		}
		r.forwardWhile(isASCIIDigit)
	}

	if _, ok := r.peekByte(); ok {
		return Number{}, ErrSyntax
	}

	str := string(token)
	if hasDot || hasExponent {
		return parseReal(str)
	}
	value, err := strconv.ParseInt(str, 10, 64)
	if err == nil {
		return NewInteger(value), nil
	}
	if errors.Is(err, strconv.ErrRange) {
		return parseReal(str)
	}
	return Number{}, ErrSyntax
}

// parseRadix is the branch of parse in hayro-postscript src/number.rs that
// reads an integer with a base. The reader stands on the #.
//
// Port: upstream writes the branch inside parse.
func parseRadix(r *reader, digitStart int) (Number, error) {
	baseBytes, ok := r.rangeOf(digitStart, r.getOffset())
	if !ok {
		return Number{}, ErrSyntax
	}
	base, err := strconv.ParseUint(string(baseBytes), 10, 32)
	if err != nil {
		return Number{}, ErrSyntax
	}
	if base < 2 || base > 36 {
		return Number{}, ErrSyntax
	}

	r.forward()

	numStart := r.getOffset()
	r.forwardWhile(isASCIIAlphanumeric)
	if _, more := r.peekByte(); r.getOffset() == numStart || more {
		return Number{}, ErrSyntax
	}

	numBytes, ok := r.rangeOf(numStart, r.getOffset())
	if !ok {
		return Number{}, ErrSyntax
	}
	for _, b := range numBytes {
		if !isDigitInBase(b, int(base)) {
			return Number{}, ErrSyntax
		}
	}
	value, err := strconv.ParseUint(string(numBytes), int(base), 64)
	if err != nil {
		if errors.Is(err, strconv.ErrRange) {
			return Number{}, ErrLimitCheck
		}
		return Number{}, ErrSyntax
	}
	// The value keeps its bits, as the cast of upstream does.
	return NewInteger(int64(value)), nil
}

// parseReal is real in hayro-postscript src/number.rs.
//
// Port: strconv.ParseFloat reports ErrRange for a value that overflows.
// Rust returns an infinity, which this function reports as ErrLimitCheck,
// as upstream does.
func parseReal(str string) (Number, error) {
	value, err := strconv.ParseFloat(str, 64)
	if err != nil && !errors.Is(err, strconv.ErrRange) {
		return Number{}, ErrSyntax
	}
	if math.IsInf(value, 0) || math.IsNaN(value) {
		return Number{}, ErrLimitCheck
	}
	return NewReal(value), nil
}
