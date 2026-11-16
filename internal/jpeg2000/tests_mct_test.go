// Copyright (c) The Hayro Authors
//
// Use of this source code is governed by the Apache-2.0 or MIT license that
// can be found in the LICENSE-APACHE and LICENSE-MIT files.

package jpeg2000

// This file ports hayro-jpeg2000 tests/mct.rs.

import "testing"

func TestMct(t *testing.T) {
	t.Run("rct", func(t *testing.T) {
		check(t, "mct_rct", reference(t, "main_rgb"))
	})
	t.Run("rct_four_components", func(t *testing.T) {
		check(t, "mct_rct_four_components", reference(t, "main_rgba"))
	})
	t.Run("rct_five_components", func(t *testing.T) {
		checkComponents(t, "mct_rct_five_components", reference(t, "main_rgbar"))
	})
	t.Run("signed_components", func(t *testing.T) {
		check(t, "mct_signed_components", reference(t, "main_rgb"))
	})
	t.Run("one_component_ignored", func(t *testing.T) {
		check(t, "mct_one_component_ignored", reference(t, "main_r"))
	})
	t.Run("raw_ycbcr_guess", func(t *testing.T) {
		check(t, "mct_raw_ycbcr_guess", reference(t, "mct_raw_ycbcr_guess"))
	})
	t.Run("ict", func(t *testing.T) {
		check(t, "mct_ict", reference(t, "main_rgb").withTolerance(3))
	})
}
