// Copyright (c) Radzivon Bartoshyk. All rights reserved.
//
// Use of this source code is governed by the BSD-3-Clause or Apache-2.0
// license that can be found in the LICENSE.md and LICENSE-APACHE.md files.

// This file ports the tests of moxcms src/gamma.rs.

package moxcms

import (
	"math"
	"testing"
)

func TestIntegerGammaTableRoundsToNearest(t *testing.T) {
	const n = 4096
	for _, tc := range []TransferCharacteristics{TransferCharacteristicsSrgb, TransferCharacteristicsBt709} {
		table := tc.makeGammaTable(n, 8)
		for v := range n {
			want := f32ToU8(roundF32(float32(tc.Gamma(float64(v)/(n-1)) * 255.0)))
			if table[v] != want {
				t.Fatalf("%d: entry %d is %d, want %d", tc, v, table[v], want)
			}
		}
	}
}

func TestTransferRoundTrips(t *testing.T) {
	for _, tc := range []struct {
		name    string
		to      func(float64) float64
		from    func(float64) float64
		epsilon float64
	}{
		{"srgb", srgbToLinear, srgbFromLinear, 1e-9},
		{"log100_sqrt10", log100Sqrt10ToLinear, log100Sqrt10FromLinear, 0},
		{"log100", log100ToLinear, log100FromLinear, 0},
		{"iec61966", iec61966ToLinear, iec61966FromLinear, 1e-9},
		{"smpte240", smpte240ToLinear, smpte240FromLinear, 1e-9},
		{"smpte428", smpte428ToLinear, smpte428FromLinear, 1e-9},
		{"rec709", rec709ToLinear, rec709FromLinear, 1e-9},
		{"hlg", hlgToLinear, hlgFromLinear, 1e-5},
		{"pq", pqToLinear, pqFromLinear, 1e-5},
		{"bt1361", bt1361ToLinear, bt1361FromLinear, 1e-5},
	} {
		got := tc.from(tc.to(0.5))
		if tc.epsilon == 0 {
			if got != 0.5 {
				t.Errorf("%s: %v, want 0.5", tc.name, got)
			}
		} else if math.Abs(0.5-got) >= tc.epsilon {
			t.Errorf("%s: %v, want 0.5", tc.name, got)
		}
	}
}
