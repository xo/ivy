// Copyright (c) The Hayro Authors.
//
// Use of this source code is governed by the Apache-2.0 or MIT license that
// can be found in the LICENSE-APACHE and LICENSE-MIT files.

// This file ports the tests of hayro-postscript src/number.rs.

package postscript

import (
	"errors"
	"math"
	"strings"
	"testing"
)

// readNum is read_num in the tests of hayro-postscript src/number.rs.
func readNum(input string) (Number, error) {
	return parseNumber([]byte(strings.TrimRight(input, " \t\n\f\r")))
}

// wantNum compares the number of input with want.
func wantNum(t *testing.T, input string, want Number) {
	t.Helper()
	got, err := readNum(input)
	if err != nil || got != want {
		t.Fatalf("readNum(%q) = %+v, %v, want %+v", input, got, err, want)
	}
}

// wantNumErr compares the error of input with want.
func wantNumErr(t *testing.T, input string, want error) {
	t.Helper()
	got, err := readNum(input)
	if !errors.Is(err, want) {
		t.Fatalf("readNum(%q) = %+v, %v, want error %v", input, got, err, want)
	}
}

func TestSignedIntegers(t *testing.T) {
	wantNum(t, "123 ", NewInteger(123))
	wantNum(t, "-98 ", NewInteger(-98))
	wantNum(t, "43445 ", NewInteger(43445))
	wantNum(t, "0 ", NewInteger(0))
	wantNum(t, "+17 ", NewInteger(17))
}

func TestRealNumbers(t *testing.T) {
	wantNum(t, "-.002 ", NewReal(-0.002))
	wantNum(t, "34.5 ", NewReal(34.5))
	wantNum(t, "-3.62 ", NewReal(-3.62))
	wantNum(t, "123.6e10 ", NewReal(123.6e10))
	wantNum(t, "1.0E-5 ", NewReal(1.0e-5))
	wantNum(t, "1E6 ", NewReal(1e6))
	wantNum(t, "-1. ", NewReal(-1.0))
	wantNum(t, "0.0 ", NewReal(0.0))
	wantNum(t, "1.e5 ", NewReal(1e5))
}

func TestRadixNumbers(t *testing.T) {
	wantNum(t, "8#1777 ", NewInteger(0o1777))
	wantNum(t, "16#FFFE ", NewInteger(0xFFFE))
	wantNum(t, "2#1000 ", NewInteger(0b1000))
	wantNum(t, "36#ZZ ", NewInteger(1295))
}

func TestLargeIntegers(t *testing.T) {
	wantNum(t, "4000000001 ", NewInteger(4000000001))
	wantNum(t, "9223372036854775807 ", NewInteger(math.MaxInt64))
	wantNum(t, "-9223372036854775808 ", NewInteger(math.MinInt64))
}

func TestIntegerOverflowBecomesReal(t *testing.T) {
	wantNum(t, "9223372036854775808 ", NewReal(9223372036854775808.0))
	wantNum(t, "-9223372036854775809 ", NewReal(-9223372036854775809.0))
	wantNum(t, "99999999999999999999999 ", NewReal(1e23))
}

func TestRealOverflow(t *testing.T) {
	wantNumErr(t, "1e400 ", ErrLimitCheck)
	wantNumErr(t, "-1e400 ", ErrLimitCheck)
	wantNumErr(t, strings.Repeat("9", 400), ErrLimitCheck)
}

func TestRadixTwosComplement(t *testing.T) {
	wantNum(t, "16#FFFFFFFF ", NewInteger(0xFFFFFFFF))
	wantNum(t, "16#FFFFFFFFFFFFFFFF ", NewInteger(-1))
	wantNum(t, "16#8000000000000000 ", NewInteger(math.MinInt64))
}

func TestRadixOverflow(t *testing.T) {
	wantNumErr(t, "16#10000000000000000 ", ErrLimitCheck)
	wantNumErr(t, "8#9 ", ErrSyntax)
	wantNumErr(t, "16#FFFFFFFFFFFFFFFFFFZ ", ErrSyntax)
}

func TestInvalidNumbers(t *testing.T) {
	for _, input := range []string{"abc", "+abc", "1a", "- ", ". ", "1e ", "-16#FF "} {
		if _, err := readNum(input); err == nil {
			t.Errorf("readNum(%q) returned no error", input)
		}
	}
}

// TestRealUnderflow tests a real number that is too small for a float64.
// Rust and Go both give zero.
func TestRealUnderflow(t *testing.T) {
	wantNum(t, "1e-400", NewReal(0))
}
