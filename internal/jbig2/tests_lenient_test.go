// Copyright (c) The Hayro Authors
//
// Use of this source code is governed by the Apache-2.0 or MIT license that
// can be found in the LICENSE-APACHE and LICENSE-MIT files.

package jbig2

// This file ports hayro-jbig2 tests/lenient.rs.

import "testing"

// TestLenient runs the tests of hayro-jbig2 tests/lenient.rs. Each subtest has the
// name of a test of upstream.
func TestLenient(t *testing.T) {
	t.Run("intermediate_page_refinement", func(t *testing.T) {
		checkLenient(t, "lenient_intermediate_page_refinement", "truchet")
	})
	t.Run("unknown_length_intermediate", func(t *testing.T) {
		checkLenient(t, "lenient_unknown_length_intermediate", "truchet")
	})
	t.Run("missing_reference", func(t *testing.T) {
		checkLenient(t, "lenient_missing_reference", "honeycomb")
	})
	t.Run("refinement_of_immediate_region", func(t *testing.T) {
		checkLenient(t, "lenient_refinement_of_immediate_region", "truchet")
	})
	t.Run("necessary_extension", func(t *testing.T) {
		checkLenient(t, "lenient_necessary_extension", "honeycomb")
	})
	t.Run("reserved_region_flags", func(t *testing.T) {
		checkLenient(t, "lenient_reserved_region_flags", "truchet")
	})
	t.Run("reserved_segment_type", func(t *testing.T) {
		checkLenient(t, "lenient_reserved_segment_type", "truchet")
	})
	t.Run("trailing_bytes", func(t *testing.T) {
		checkLenient(t, "lenient_trailing_bytes", "honeycomb")
	})
	t.Run("truncated_segment", func(t *testing.T) {
		checkLenient(t, "lenient_truncated_segment", "truchet")
	})
	t.Run("missing_final_oob", func(t *testing.T) {
		checkLenient(t, "lenient_missing_final_oob", "honeycomb")
	})
	t.Run("missing_final_oob_huffman", func(t *testing.T) {
		checkLenient(t, "lenient_missing_final_oob_huffman", "honeycomb")
	})
	t.Run("table_past_hthigh", func(t *testing.T) {
		checkLenient(t, "lenient_table_past_hthigh", "honeycomb")
	})
	t.Run("gray_value_out_of_range", func(t *testing.T) {
		checkLenient(t, "lenient_gray_value_out_of_range", "maze")
	})
	t.Run("mmr_rows_missing", func(t *testing.T) {
		checkLenient(t, "lenient_mmr_rows_missing", "truchet")
	})
	t.Run("failing_region", func(t *testing.T) {
		checkLenient(t, "lenient_failing_region", "honeycomb")
	})
	t.Run("symbol_out_of_range", func(t *testing.T) {
		checkLenient(t, "lenient_symbol_out_of_range", "honeycomb")
	})
	t.Run("colour_region", func(t *testing.T) {
		checkLenient(t, "lenient_colour_region", "honeycomb")
	})
	t.Run("page_without_end_of_page", func(t *testing.T) {
		checkPagesLenient(t, "lenient_page_without_end_of_page", []string{"truchet", "honeycomb", "maze"})
	})
	t.Run("end_of_page_of_previous_page", func(t *testing.T) {
		checkPagesLenient(t, "lenient_end_of_page_of_previous_page", []string{"truchet", "honeycomb", "maze"})
	})
	t.Run("page_count_mismatch", func(t *testing.T) {
		checkPagesLenient(t, "lenient_page_count_mismatch", []string{"truchet", "honeycomb", "maze"})
	})
}
