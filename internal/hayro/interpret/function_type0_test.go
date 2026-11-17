// Copyright (c) The Hayro Authors.
//
// Use of this source code is governed by the Apache-2.0 or MIT license that
// can be found in the LICENSE-APACHE and LICENSE-MIT files.

// This file ports the tests of hayro-interpret src/function/type0.rs.

package interpret

import "testing"

func TestSampleIndexStaysBelowSizeWhenSizeRounds(t *testing.T) {
	const size = 16_777_220
	input := sampledInput{
		domain: interval{min: 0, max: 1},
		encode: interval{min: 0, max: float32(size - 1)},
		size:   size,
		stride: 1,
	}
	for _, x := range []float32{1, 2} {
		index, _, hasFraction := input.samplePosition(x)
		if index != 16_777_219 || hasFraction {
			t.Errorf("samplePosition(%v) = %d, %v, want 16777219, false", x, index, hasFraction)
		}
	}
}
