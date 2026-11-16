// Copyright (c) The Hayro Authors
//
// Use of this source code is governed by the Apache-2.0 or MIT license that
// can be found in the LICENSE-APACHE and LICENSE-MIT files.

package jbig2

// This file ports hayro-jbig2 tests/text.rs.

import "testing"

// TestText runs the tests of hayro-jbig2 tests/text.rs. Each subtest has the
// name of a test of upstream.
func TestText(t *testing.T) {
	t.Run("bottomleft", func(t *testing.T) {
		check(t, "text_bottomleft", "sphere")
	})
	t.Run("bottomright", func(t *testing.T) {
		check(t, "text_bottomright", "sphere")
	})
	t.Run("topright", func(t *testing.T) {
		check(t, "text_topright", "sphere")
	})
	t.Run("bottomleft_transposed", func(t *testing.T) {
		check(t, "text_bottomleft_transposed", "sphere")
	})
	t.Run("bottomright_transposed", func(t *testing.T) {
		check(t, "text_bottomright_transposed", "sphere")
	})
	t.Run("topright_transposed", func(t *testing.T) {
		check(t, "text_topright_transposed", "sphere")
	})
	t.Run("topleft_transposed", func(t *testing.T) {
		check(t, "text_topleft_transposed", "sphere")
	})
	t.Run("negative_ds_offset", func(t *testing.T) {
		check(t, "text_negative_ds_offset", "honeycomb")
	})
	t.Run("refine", func(t *testing.T) {
		check(t, "text_refine", "honeycomb")
	})
	t.Run("refine_template1", func(t *testing.T) {
		check(t, "text_refine_template1", "honeycomb")
	})
	t.Run("refine_custom_at", func(t *testing.T) {
		check(t, "text_refine_custom_at", "honeycomb")
	})
	t.Run("refine_negative_delta_width", func(t *testing.T) {
		check(t, "text_refine_negative_delta_width", "honeycomb")
	})
	t.Run("refine_positive_offset", func(t *testing.T) {
		check(t, "text_refine_positive_offset", "honeycomb")
	})
	t.Run("refine_large_offsets", func(t *testing.T) {
		check(t, "text_refine_large_offsets", "honeycomb")
	})
	t.Run("many_instances", func(t *testing.T) {
		check(t, "text_many_instances", "honeycomb")
	})
	t.Run("instance_far_left", func(t *testing.T) {
		check(t, "text_instance_far_left", "honeycomb")
	})
	t.Run("refine_far_reference", func(t *testing.T) {
		check(t, "text_refine_far_reference", "honeycomb")
	})
	t.Run("then_refine", func(t *testing.T) {
		check(t, "text_then_refine", "honeycomb")
	})
	t.Run("then_refine_default_pixel", func(t *testing.T) {
		check(t, "text_then_refine_default_pixel", "honeycomb")
	})
	t.Run("large_integer", func(t *testing.T) {
		check(t, "text_large_integer", "honeycomb")
	})
	t.Run("composition", func(t *testing.T) {
		check(t, "text_composition", "honeycomb")
	})
}
