// Copyright (c) Radzivon Bartoshyk. All rights reserved.
//
// Use of this source code is governed by the BSD-3-Clause or Apache-2.0
// license that can be found in the LICENSE.md and LICENSE-APACHE.md files.

// This file ports the tests of moxcms src/lab.rs.

package moxcms

import "testing"

func TestRoundPcsTrip(t *testing.T) {
	xyz := NewXyz(0.1, 0.2, 0.3)
	back := FromPcsXyz(xyz).ToPcsXyz()
	for _, d := range []float32{xyz.X - back.X, xyz.Y - back.Y, xyz.Z - back.Z} {
		if absF32(d) >= 1e-5 {
			t.Errorf("%+v rolls back to %+v", xyz, back)
		}
	}
}
