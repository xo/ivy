// Copyright (c) The Hayro Authors
//
// Use of this source code is governed by the Apache-2.0 or MIT license that
// can be found in the LICENSE-APACHE and LICENSE-MIT files.

// This file ports the tests of hayro-syntax src/content/ops.rs.

package syntax

import (
	"bytes"
	"slices"
	"testing"
)

// n is n of the tests of upstream.
func n(v int32) Number {
	return NumberFromI32(v)
}

// next returns the next typed instruction, or fails the test.
func next(t *testing.T, it *TypedIter) TypedInstruction {
	t.Helper()
	op, ok := it.Next()
	if !ok {
		t.Fatal("no instruction")
	}
	return op
}

// expectEnd fails the test when the iterator has more instructions.
func expectEnd(t *testing.T, it *TypedIter) {
	t.Helper()
	if op, ok := it.Next(); ok {
		t.Fatalf("unexpected instruction %v", op)
	}
}

func TestBasicOps1(t *testing.T) {
	it := NewTypedIter([]byte("\n1 0 0 -1 0 200 cm\n/g0 gs\n1 0 0 rg\n"))
	if op, ok := next(t, it).(TransformOps); !ok || op != (TransformOps{n(1), n(0), n(0), n(-1), n(0), n(200)}) {
		t.Error("cm")
	}
	if op, ok := next(t, it).(SetGraphicsState); !ok || op.V0 != "g0" {
		t.Error("gs")
	}
	if op, ok := next(t, it).(NonStrokeColorDeviceRgb); !ok || op != (NonStrokeColorDeviceRgb{n(1), n(0), n(0)}) {
		t.Error("rg")
	}
	expectEnd(t, it)
}

func TestBasicOps2(t *testing.T) {
	it := NewTypedIter([]byte("\n20 20 m\n180 20 l\n180 180 l\n20 180 l\nh\nf\n"))
	if op, ok := next(t, it).(MoveTo); !ok || op != (MoveTo{n(20), n(20)}) {
		t.Error("m")
	}
	for _, want := range []LineTo{{n(180), n(20)}, {n(180), n(180)}, {n(20), n(180)}} {
		if op, ok := next(t, it).(LineTo); !ok || op != want {
			t.Errorf("l: want %v", want)
		}
	}
	if _, ok := next(t, it).(ClosePath); !ok {
		t.Error("h")
	}
	if _, ok := next(t, it).(FillPathNonZero); !ok {
		t.Error("f")
	}
	expectEnd(t, it)
}

func TestDisplayOps(t *testing.T) {
	it := NewTypedIter([]byte("h 2 w 1 0 0 RG 0 0 10 20 re 10 20 m 30 20 60 40 90 80 c " +
		"0.1 0.2 SC 0.0 scn 1.0 /Spot SCN (Hello) Tj <00FF> Tj foo"))
	for _, want := range []string{
		"ClosePath",
		"LineWidth(2)",
		"StrokeColorDeviceRgb(1 0 0)",
		"RectPath(0 0 10 20)",
		"MoveTo(10 20)",
		"CubicTo(30 20 60 40 90 80)",
		"StrokeColor(0.1 0.2)",
		"NonStrokeColorNamed(0.0)",
		"StrokeColorNamed(1.0 /Spot)",
		"ShowText(\"Hello\")",
		"ShowText([0, 255])",
		"Fallback(foo)",
	} {
		if got := next(t, it).String(); got != want {
			t.Errorf("got %q, want %q", got, want)
		}
	}
	expectEnd(t, it)
}

func TestScn(t *testing.T) {
	it := NewTypedIter([]byte("\n0.0 scn\n1.0 1.0 1.0 /DeviceRgb SCN\n"))
	op1, ok := next(t, it).(NonStrokeColorNamed)
	if !ok || op1.V1 != nil || !slices.Equal(op1.V0, []Number{NumberFromF32(0)}) {
		t.Errorf("scn: %v", op1)
	}
	op2, ok := next(t, it).(StrokeColorNamed)
	one := NumberFromF32(1)
	if !ok || op2.V1 == nil || *op2.V1 != "DeviceRgb" || !slices.Equal(op2.V0, []Number{one, one, one}) {
		t.Errorf("SCN: %v", op2)
	}
	expectEnd(t, it)
}

func TestScnDeviceNPattern(t *testing.T) {
	input := append(bytes.Repeat([]byte("0.5 "), 32), "/P0 scn 0 0 m"...)
	it := NewTypedIter(input)
	op, ok := next(t, it).(NonStrokeColorNamed)
	want := slices.Repeat([]Number{NumberFromF32(0.5)}, 32)
	if !ok || op.V1 == nil || *op.V1 != "P0" || !slices.Equal(op.V0, want) {
		t.Errorf("scn: %v", op)
	}
	if _, ok := next(t, it).(MoveTo); !ok {
		t.Error("m")
	}
	expectEnd(t, it)
}

func TestDP(t *testing.T) {
	it := NewTypedIter([]byte("/Attribute<</ShowCenterPoint false >> DP"))
	op, ok := next(t, it).(MarkedContentPointWithProperties)
	want, _ := FromBytes[Dict]([]byte("<</ShowCenterPoint false >>"))
	if d, isDict := op.V1.(Dict); !ok || op.V0 != "Attribute" || !isDict || !d.Equal(want) {
		t.Errorf("DP: %v", op)
	}
	expectEnd(t, it)
}

func TestBDCWithDict(t *testing.T) {
	it := NewTypedIter([]byte("/Span << /MCID 0 /Alt (Alt)>> BDC EMC"))
	op, ok := next(t, it).(BeginMarkedContentWithProperties)
	want, _ := FromBytes[Dict]([]byte("<< /MCID 0 /Alt (Alt)>>"))
	if d, isDict := op.V1.(Dict); !ok || op.V0 != "Span" || !isDict || !d.Equal(want) {
		t.Errorf("BDC: %v", op)
	}
	if _, ok := next(t, it).(EndMarkedContent); !ok {
		t.Error("EMC")
	}
	expectEnd(t, it)
}

func TestBDCWithName(t *testing.T) {
	it := NewTypedIter([]byte("/Span /Name BDC EMC"))
	op, ok := next(t, it).(BeginMarkedContentWithProperties)
	if name, isName := op.V1.(Name); !ok || op.V0 != "Span" || !isName || name != "Name" {
		t.Errorf("BDC: %v", op)
	}
	if _, ok := next(t, it).(EndMarkedContent); !ok {
		t.Error("EMC")
	}
	expectEnd(t, it)
}
