// Copyright (c) The Hayro Authors
//
// Use of this source code is governed by the Apache-2.0 or MIT license that
// can be found in the LICENSE-APACHE and LICENSE-MIT files.

package jbig2

// This file ports hayro-jbig2 tests/refinement.rs.

import "testing"

// TestRefinement runs the tests of hayro-jbig2 tests/refinement.rs. Each subtest has the
// name of a test of upstream.
func TestRefinement(t *testing.T) {
	t.Run("trailing_7fff_stripped_many", func(t *testing.T) {
		check(t, "refine_trailing_7fff_stripped_many", "truchet")
	})
	t.Run("template0", func(t *testing.T) {
		check(t, "refine_template0", "truchet")
	})
	t.Run("template1", func(t *testing.T) {
		check(t, "refine_template1", "truchet")
	})
	t.Run("template0_tpgron", func(t *testing.T) {
		check(t, "refine_template0_tpgron", "truchet")
	})
	t.Run("template1_tpgron", func(t *testing.T) {
		check(t, "refine_template1_tpgron", "truchet")
	})
	t.Run("template0_custom_at", func(t *testing.T) {
		check(t, "refine_template0_custom_at", "truchet")
	})
	t.Run("template0_custom_at_tpgron", func(t *testing.T) {
		check(t, "refine_template0_custom_at_tpgron", "truchet")
	})
	t.Run("lossless", func(t *testing.T) {
		check(t, "refine_lossless", "truchet")
	})
	t.Run("refined_region", func(t *testing.T) {
		check(t, "refine_refined_region", "truchet")
	})
	t.Run("page", func(t *testing.T) {
		check(t, "refine_page", "truchet")
	})
	t.Run("page_part", func(t *testing.T) {
		check(t, "refine_page_part", "truchet")
	})
}
