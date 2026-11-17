// Copyright (c) The Hayro Authors.
//
// Use of this source code is governed by the Apache-2.0 or MIT license that
// can be found in the LICENSE-APACHE and LICENSE-MIT files.

// This file ports the tests of hayro-interpret src/function/type2.rs.

package interpret

import (
	"slices"
	"testing"

	"github.com/xo/ivy/internal/hayro/syntax"
)

// functionFromBytes reads a function from the text of a dictionary.
func functionFromBytes(t *testing.T, data string) *Function {
	t.Helper()
	obj, ok := syntax.FromBytes[syntax.Object]([]byte(data))
	if !ok {
		t.Fatalf("cannot read %q", data)
	}
	f, ok := NewFunction(obj)
	if !ok {
		t.Fatalf("cannot make a function of %q", data)
	}
	return f
}

// mustEval evaluates f on one sample.
func mustEval(t *testing.T, f *Function, input ...float32) []float32 {
	t.Helper()
	out, ok := f.evalOne(input)
	if !ok {
		t.Fatalf("eval(%v) fails", input)
	}
	return out
}

func TestType2Simple(t *testing.T) {
	f := functionFromBytes(t, `<<
  /FunctionType 2
  /Domain [ 0  1 ]
  /C0 [ 0 20  ]
  /C1 [ 30 -50 ]
  /N 1
>>`)
	for _, c := range []struct{ in, want []float32 }{
		{[]float32{0}, []float32{0, 20}},
		{[]float32{0.5}, []float32{15, -15}},
		{[]float32{1}, []float32{30, -50}},
	} {
		if got := mustEval(t, f, c.in...); !slices.Equal(got, c.want) {
			t.Errorf("eval(%v) = %v, want %v", c.in, got, c.want)
		}
	}
}

func TestType2WithExponent(t *testing.T) {
	f := functionFromBytes(t, `<<
  /FunctionType 2
  /Domain [ 0  1 ]
  /C0 [ 0  ]
  /C1 [ 30 ]
  /N 2
>>`)
	if got := mustEval(t, f, 0.5); !slices.Equal(got, []float32{7.5}) {
		t.Errorf("eval(0.5) = %v, want [7.5]", got)
	}
}

func TestType2ClampDomain(t *testing.T) {
	f := functionFromBytes(t, `<<
  /FunctionType 2
  /Domain [ 0.2  0.8 ]
  /C0 [ 0  ]
  /C1 [ 30 ]
  /N 2
>>`)
	for _, c := range [][2]float32{{0, 0.2}, {-10, 0.2}, {0.8, 0.8}, {1.2, 1.0}} {
		if a, b := mustEval(t, f, c[0]), mustEval(t, f, c[1]); !slices.Equal(a, b) {
			t.Errorf("eval(%v) = %v, eval(%v) = %v", c[0], a, c[1], b)
		}
	}
}

func TestType2ClampRange(t *testing.T) {
	f := functionFromBytes(t, `<<
  /FunctionType 2
  /Domain [ 0.0  1.0 ]
  /Range [10.0 20.0]
  /C0 [ 0  ]
  /C1 [ 30 ]
  /N 1
>>`)
	for _, c := range [][2]float32{{0, 10}, {0.5, 15}, {1, 20}} {
		if got := mustEval(t, f, c[0]); !slices.Equal(got, []float32{c[1]}) {
			t.Errorf("eval(%v) = %v, want [%v]", c[0], got, c[1])
		}
	}
}
