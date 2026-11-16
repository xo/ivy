// Copyright (c) The Hayro Authors
//
// Use of this source code is governed by the Apache-2.0 or MIT license that
// can be found in the LICENSE-APACHE and LICENSE-MIT files.

package jbig2

// This file ports hayro-jbig2 tests/page.rs.

import "testing"

// TestPage runs the tests of hayro-jbig2 tests/page.rs. Each subtest has the
// name of a test of upstream.
func TestPage(t *testing.T) {
	t.Run("striped", func(t *testing.T) {
		check(t, "page_striped", "truchet")
	})
	t.Run("striped_unknown_height", func(t *testing.T) {
		check(t, "page_striped_unknown_height", "truchet")
	})
	t.Run("striped_last_stripe_implicit", func(t *testing.T) {
		check(t, "page_striped_last_stripe_implicit", "truchet")
	})
	t.Run("striped_single_stripe", func(t *testing.T) {
		check(t, "page_striped_single_stripe", "truchet")
	})
	t.Run("striped_no_end_of_stripe", func(t *testing.T) {
		check(t, "page_striped_no_end_of_stripe", "truchet")
	})
}
