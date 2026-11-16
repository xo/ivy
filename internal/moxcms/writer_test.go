// Copyright (c) Radzivon Bartoshyk. All rights reserved.
//
// Use of this source code is governed by the BSD-3-Clause or Apache-2.0
// license that can be found in the LICENSE.md and LICENSE-APACHE.md files.

// This file ports the tests of moxcms src/writer.rs.

package moxcms

import (
	"math"
	"testing"
)

func TestToU8Fixed8(t *testing.T) {
	for _, tc := range []struct {
		v    float32
		want uint16
	}{
		{0, 0},
		{1, 0x0100},
		{255 + 255.0/256, math.MaxUint16},
	} {
		if got := toU8Fixed8(tc.v); got != tc.want {
			t.Errorf("toU8Fixed8(%v) = %#x, want %#x", tc.v, got, tc.want)
		}
	}
}
