// Copyright (c) The Hayro Authors.
//
// Use of this source code is governed by the Apache-2.0 or MIT license that
// can be found in the LICENSE-APACHE and LICENSE-MIT files.

// This file ports the tests of hayro-interpret src/function/type3.rs.

package interpret

import (
	"slices"
	"testing"
)

func TestType3Simple(t *testing.T) {
	f := functionFromBytes(t, `<<
  /FunctionType 3
  /Domain [-7 7]
  /Functions [
    << /FunctionType 2
       /Domain [0 1]
       /C0 [0.5 0.5 0.5]
       /C1 [0.5 0.5 0.5]
       /N 1
    >>
    << /FunctionType 2
       /Domain [0 1]
       /C0 [0.7 0.7 0.7]
       /C1 [0.7 0.7 0.7]
       /N 1
    >>
  ]
  /Bounds [0]
  /Encode [0 1 0 1]
>>`)
	low := []float32{0.5, 0.5, 0.5}
	high := []float32{0.7, 0.7, 0.7}
	for _, c := range []struct {
		in   float32
		want []float32
	}{{-7, low}, {-3, low}, {-0.5, low}, {0, high}, {7, high}} {
		if got := mustEval(t, f, c.in); !slices.Equal(got, c.want) {
			t.Errorf("eval(%v) = %v, want %v", c.in, got, c.want)
		}
	}
}
