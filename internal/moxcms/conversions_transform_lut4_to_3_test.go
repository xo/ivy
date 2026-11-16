// Copyright (c) Radzivon Bartoshyk. All rights reserved.
//
// Use of this source code is governed by the BSD-3-Clause or Apache-2.0
// license that can be found in the LICENSE.md and LICENSE-APACHE.md files.

// This file ports the tests of moxcms
// src/conversions/transform_lut4_to_3.rs.

package moxcms

import (
	"math"
	"testing"
)

// pmulhrswReference is pmulhrsw_reference in the tests of moxcms
// src/conversions/transform_lut4_to_3.rs.
func pmulhrswReference(a, b int16) int16 {
	product := int32(a) * int32(b)
	return int16(((product >> 14) + 1) >> 1)
}

func TestQ015MulMatchesPmulhrswRounding(t *testing.T) {
	values := []int16{
		math.MinInt16, -30000, -16384, -8192, -1, 0, 1, 8191, 8192, 16383, 16384, 30000, math.MaxInt16,
	}
	for _, a := range values {
		for _, b := range values {
			if got, want := q015Mul(a, b), pmulhrswReference(a, b); got != want {
				t.Errorf("q015Mul(%d, %d) = %d, want %d", a, b, got, want)
			}
		}
	}
}
