// Copyright (c) The Hayro Authors
//
// Use of this source code is governed by the Apache-2.0 or MIT license that
// can be found in the LICENSE-APACHE and LICENSE-MIT files.

package jbig2

// This file ports hayro-jbig2 tests/generic.rs.

import "testing"

// TestGeneric runs the tests of hayro-jbig2 tests/generic.rs. Each subtest has the
// name of a test of upstream.
func TestGeneric(t *testing.T) {
	t.Run("template0", func(t *testing.T) {
		check(t, "generic_template0", "truchet")
	})
	t.Run("template1", func(t *testing.T) {
		check(t, "generic_template1", "truchet")
	})
	t.Run("template1_custom_at", func(t *testing.T) {
		check(t, "generic_template1_custom_at", "truchet")
	})
	t.Run("template1_custom_at_tpgdon", func(t *testing.T) {
		check(t, "generic_template1_custom_at_tpgdon", "maze")
	})
	t.Run("template2", func(t *testing.T) {
		check(t, "generic_template2", "truchet")
	})
	t.Run("template2_custom_at", func(t *testing.T) {
		check(t, "generic_template2_custom_at", "truchet")
	})
	t.Run("template2_custom_at_tpgdon", func(t *testing.T) {
		check(t, "generic_template2_custom_at_tpgdon", "maze")
	})
	t.Run("template3", func(t *testing.T) {
		check(t, "generic_template3", "truchet")
	})
	t.Run("template3_custom_at", func(t *testing.T) {
		check(t, "generic_template3_custom_at", "truchet")
	})
	t.Run("template3_custom_at_tpgdon", func(t *testing.T) {
		check(t, "generic_template3_custom_at_tpgdon", "maze")
	})
	t.Run("template0_custom_at", func(t *testing.T) {
		check(t, "generic_template0_custom_at", "truchet")
	})
	t.Run("template0_custom_at_tpgdon", func(t *testing.T) {
		check(t, "generic_template0_custom_at_tpgdon", "maze")
	})
	t.Run("template0_ext", func(t *testing.T) {
		check(t, "generic_template0_ext", "truchet")
	})
	t.Run("template0_ext_custom_at", func(t *testing.T) {
		check(t, "generic_template0_ext_custom_at", "truchet")
	})
	t.Run("template0_ext_custom_at_tpgdon", func(t *testing.T) {
		check(t, "generic_template0_ext_custom_at_tpgdon", "maze")
	})
	t.Run("template0_tpgdon", func(t *testing.T) {
		check(t, "generic_template0_tpgdon", "maze")
	})
	t.Run("template1_tpgdon", func(t *testing.T) {
		check(t, "generic_template1_tpgdon", "maze")
	})
	t.Run("template2_tpgdon", func(t *testing.T) {
		check(t, "generic_template2_tpgdon", "maze")
	})
	t.Run("template3_tpgdon", func(t *testing.T) {
		check(t, "generic_template3_tpgdon", "maze")
	})
	t.Run("mmr", func(t *testing.T) {
		check(t, "generic_mmr", "truchet")
	})
	t.Run("unknown_length", func(t *testing.T) {
		check(t, "generic_unknown_length", "truchet")
	})
	t.Run("unknown_length_mmr", func(t *testing.T) {
		check(t, "generic_unknown_length_mmr", "truchet")
	})
	t.Run("unknown_length_custom_at", func(t *testing.T) {
		check(t, "generic_unknown_length_custom_at", "truchet")
	})
	t.Run("trailing_7fff_stripped", func(t *testing.T) {
		check(t, "generic_trailing_7fff_stripped", "honeycomb")
	})
	t.Run("trailing_7fff_stripped_many", func(t *testing.T) {
		check(t, "generic_trailing_7fff_stripped_many", "truchet")
	})
	t.Run("row_ending_in_full_chunks", func(t *testing.T) {
		checkCrop(t, "generic_row_ending_in_full_chunks", "truchet", 0, 30, 32, 23)
	})
}
