// Copyright (c) The Hayro Authors
//
// Use of this source code is governed by the Apache-2.0 or MIT license that
// can be found in the LICENSE-APACHE and LICENSE-MIT files.

package jbig2

// This file ports hayro-jbig2 tests/halftone.rs.

import "testing"

// TestHalftone runs the tests of hayro-jbig2 tests/halftone.rs. Each subtest has the
// name of a test of upstream.
func TestHalftone(t *testing.T) {
	t.Run("basic", func(t *testing.T) {
		check(t, "halftone", "maze")
	})
	t.Run("replace_word_aligned", func(t *testing.T) {
		check(t, "halftone_replace_word_aligned", "maze")
	})
	t.Run("global", func(t *testing.T) {
		check(t, "halftone_global", "maze")
	})
	t.Run("template1", func(t *testing.T) {
		check(t, "halftone_template1", "maze")
	})
	t.Run("template2", func(t *testing.T) {
		check(t, "halftone_template2", "maze")
	})
	t.Run("template3", func(t *testing.T) {
		check(t, "halftone_template3", "maze")
	})
	t.Run("halftone_10_bitplanes", func(t *testing.T) {
		check(t, "halftone_10_bitplanes", "maze")
	})
	t.Run("halftone_10_bitplanes_mmr", func(t *testing.T) {
		check(t, "halftone_10_bitplanes_mmr", "maze")
	})
	t.Run("mmr_bitplane_boundary", func(t *testing.T) {
		checkCrop(t, "halftone_mmr_bitplane_boundary", "maze", 63, 0, 27, 9)
	})
	t.Run("skip_unused", func(t *testing.T) {
		check(t, "halftone_skip_unused", "maze")
	})
	t.Run("then_refine", func(t *testing.T) {
		check(t, "halftone_then_refine", "maze")
	})
	t.Run("then_refine_default_pixel", func(t *testing.T) {
		check(t, "halftone_then_refine_default_pixel", "maze")
	})
	t.Run("rotated_grid", func(t *testing.T) {
		check(t, "halftone_rotated_grid", "sphere")
	})
	t.Run("rotated_grid_skip", func(t *testing.T) {
		check(t, "halftone_rotated_grid_skip", "sphere")
	})
	t.Run("rotated_grid_skip_template1", func(t *testing.T) {
		check(t, "halftone_rotated_grid_skip_template1", "sphere")
	})
	t.Run("rotated_grid_skip_template2", func(t *testing.T) {
		check(t, "halftone_rotated_grid_skip_template2", "sphere")
	})
	t.Run("rotated_grid_skip_template3", func(t *testing.T) {
		check(t, "halftone_rotated_grid_skip_template3", "sphere")
	})
	t.Run("combination_operators", func(t *testing.T) {
		check(t, "halftone_combination_operators", "truchet")
	})
	t.Run("tiles_xor", func(t *testing.T) {
		check(t, "halftone_tiles_xor", "truchet")
	})
	t.Run("tiles_and", func(t *testing.T) {
		check(t, "halftone_tiles_and", "truchet")
	})
	t.Run("tiles_xnor", func(t *testing.T) {
		check(t, "halftone_tiles_xnor", "truchet")
	})
	t.Run("tiles_replace", func(t *testing.T) {
		check(t, "halftone_tiles_replace", "truchet")
	})
	t.Run("large_patterns", func(t *testing.T) {
		check(t, "halftone_large_patterns", "truchet")
	})
	t.Run("tall_patterns", func(t *testing.T) {
		check(t, "halftone_tall_patterns", "truchet")
	})
}
