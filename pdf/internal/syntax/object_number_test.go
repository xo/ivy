// Copyright (c) The Hayro Authors
//
// Use of this source code is governed by the Apache-2.0 or MIT license that
// can be found in the LICENSE-APACHE and LICENSE-MIT files.

// This file ports the tests of hayro-syntax src/object/number.rs.

package syntax

import (
	"math"
	"strconv"
	"strings"
	"testing"
)

func TestNumberDisplay(t *testing.T) {
	tests := []struct {
		n    Number
		want string
	}{
		{NumberFromI32(10), "10"},
		{NumberFromF32(10.0), "10.0"},
		{NumberFromF32(10.5), "10.5"},
	}
	for _, test := range tests {
		if got := test.n.String(); got != test.want {
			t.Errorf("got %q, want %q", got, test.want)
		}
	}
}

func TestInt(t *testing.T) {
	tests := []struct {
		data string
		want int32
	}{
		{"0", 0},
		{"+32", 32},
		{"-32", -32},
		{"98349", 98349},
		{"003245", 3245},
	}
	for _, test := range tests {
		got, ok := NewReader([]byte(test.data)).ReadWithoutContext[int32]()
		if !ok || got != test.want {
			t.Errorf("%q: got %d, %v; want %d", test.data, got, ok, test.want)
		}
	}
}

func TestIntMinStaysInteger(t *testing.T) {
	n, ok := NewReader([]byte("-9223372036854775808")).ReadWithoutContext[Number]()
	if !ok || n.isReal || n.integer != math.MinInt64 || n.AsI64() != math.MinInt64 {
		t.Fatalf("got %#v", n)
	}
}

func TestReal(t *testing.T) {
	tests := []struct {
		data string
		want float32
	}{
		{"3", 3.0},
		{"+32", 32.0},
		{"-32", -32.0},
		{"-32.01", -32.01},
		{"-.345", -0.345},
		{"-.00143", -0.00143},
		{"-12.0013", -12.0013},
		{"98349.432534", 98_349.43},
		{"-34534656.34", -34534656.34},
	}
	for _, test := range tests {
		got, ok := NewReader([]byte(test.data)).ReadWithoutContext[float32]()
		if !ok || got != test.want {
			t.Errorf("%q: got %v, %v; want %v", test.data, got, ok, test.want)
		}
	}
}

func TestLoneSign(t *testing.T) {
	for _, input := range []string{"+ ", "+(", "+<", "+]", "+/", "- ", "-(", "-<", "-]", "-/"} {
		got, ok := NewReader([]byte(input)).ReadWithoutContext[int32]()
		if !ok || got != 0 {
			t.Errorf("%q: got %d, %v", input, got, ok)
		}
	}
	for _, input := range []string{"+", "-", "+abc", "-abc"} {
		if _, ok := NewReader([]byte(input)).ReadWithoutContext[int32](); ok {
			t.Errorf("%q: accepted", input)
		}
	}
}

func TestRealFailing(t *testing.T) {
	if _, ok := NewReader([]byte("+abc")).ReadWithoutContext[float32](); ok {
		t.Fatal("accepted")
	}
}

func TestNumberAsF32(t *testing.T) {
	tests := []struct {
		data string
		want float32
	}{
		{"+32", 32.0},
		{"-32.01", -32.01},
		{"-.345", -0.345},
	}
	for _, test := range tests {
		n, ok := NewReader([]byte(test.data)).ReadWithoutContext[Number]()
		if !ok || float32(n.AsF64()) != test.want {
			t.Errorf("%q: got %v, %v", test.data, n, ok)
		}
	}
}

func TestLargeNumber(t *testing.T) {
	n, ok := NewReader([]byte("38359922")).ReadWithoutContext[Number]()
	if !ok || n.AsI64() != 38359922 {
		t.Errorf("got %v", n)
	}
	u, ok := NewReader([]byte("4294966260")).ReadWithoutContext[uint32]()
	if !ok || u != 4294966260 {
		t.Errorf("got %d", u)
	}
}

func TestLongReals(t *testing.T) {
	for _, input := range []string{"190.50000762939453225", "190.5000076293945313"} {
		n, ok := NewReader([]byte(input)).ReadWithoutContext[Number]()
		if !ok || n.AsF64() != 190.500_007_629_394_53 {
			t.Errorf("%q: got %v", input, n)
		}
	}
}

func TestOversizedIntegerUsesFloatFallback(t *testing.T) {
	n, ok := NewReader([]byte("9999999999999999999")).ReadWithoutContext[Number]()
	if !ok || n.AsF64() != float64(uint64(9_999_999_999_999_999_999)) || n.AsI64() != math.MaxInt64 {
		t.Errorf("got %v", n)
	}
	n, ok = NewReader([]byte("-1234567890123456789012345")).ReadWithoutContext[Number]()
	if !ok || n.AsF64() != -1.234_567_890_123_456_8e24 || n.AsI64() != math.MinInt64 {
		t.Errorf("got %v", n)
	}
}

func TestIntegerOverflowThenFractionUsesFloatFallback(t *testing.T) {
	n, ok := NewReader([]byte("12345678901234567890123.456")).ReadWithoutContext[Number]()
	if !ok || n.AsF64() != 1.234_567_890_123_456_8e22 {
		t.Errorf("got %v", n)
	}
}

func TestF32MaxUsesFloatFallback(t *testing.T) {
	got, ok := NewReader([]byte("340282346638528859811704183484516925440")).ReadWithoutContext[float32]()
	if !ok || got != math.MaxFloat32 {
		t.Errorf("got %v", got)
	}
}

func TestFloatFallbackIgnoresTrailingMinus(t *testing.T) {
	input := []byte("1234567890123456789012345-123")
	r := NewReader(input)
	n, ok := r.ReadWithoutContext[Number]()
	if !ok || n.AsF64() != 1.234_567_890_123_456_8e24 || r.Offset() != len(input) {
		t.Errorf("got %v at %d", n, r.Offset())
	}
}

func TestRealsAreCorrectlyRounded(t *testing.T) {
	tiny := "0." + strings.Repeat("0", 309) + "1"
	for _, input := range []string{"519188.22107137204", "-0.000123", tiny} {
		n, ok := NewReader([]byte(input)).ReadWithoutContext[Number]()
		want, _ := strconv.ParseFloat(input, 64)
		if !ok || n.AsF64() != want {
			t.Errorf("%q: got %v, want %v", input, n.AsF64(), want)
		}
	}
}
