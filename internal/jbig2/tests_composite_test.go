// Copyright (c) The Hayro Authors
//
// Use of this source code is governed by the Apache-2.0 or MIT license that
// can be found in the LICENSE-APACHE and LICENSE-MIT files.

package jbig2

// This file ports hayro-jbig2 tests/composite.rs.

import "testing"

// TestComposite runs the tests of hayro-jbig2 tests/composite.rs. Each subtest has the
// name of a test of upstream.
func TestComposite(t *testing.T) {
	t.Run("or_xor_replace", func(t *testing.T) {
		check(t, "composite_or_xor_replace", "truchet")
	})
	t.Run("and_xnor", func(t *testing.T) {
		check(t, "composite_and_xnor", "truchet")
	})
	t.Run("or_xor_replace_refine", func(t *testing.T) {
		check(t, "composite_or_xor_replace_refine", "truchet")
	})
	t.Run("and_xnor_refine", func(t *testing.T) {
		check(t, "composite_and_xnor_refine", "truchet")
	})
	t.Run("page_replace", func(t *testing.T) {
		check(t, "composite_page_replace", "truchet")
	})
	t.Run("page_default_pixel_and", func(t *testing.T) {
		check(t, "composite_page_default_pixel_and", "truchet")
	})
	t.Run("page_default_pixel_xnor", func(t *testing.T) {
		check(t, "composite_page_default_pixel_xnor", "truchet")
	})
	t.Run("page_default_pixel_replace", func(t *testing.T) {
		check(t, "composite_page_default_pixel_replace", "truchet")
	})
	t.Run("region_default_pixel", func(t *testing.T) {
		check(t, "composite_region_default_pixel", "truchet")
	})
	t.Run("or_xor_replace_text", func(t *testing.T) {
		check(t, "composite_or_xor_replace_text", "truchet")
	})
	t.Run("and_xnor_text", func(t *testing.T) {
		check(t, "composite_and_xnor_text", "truchet")
	})
	t.Run("or_xor_replace_halftone", func(t *testing.T) {
		check(t, "composite_or_xor_replace_halftone", "truchet")
	})
	t.Run("and_xnor_halftone", func(t *testing.T) {
		check(t, "composite_and_xnor_halftone", "truchet")
	})
}
