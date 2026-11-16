// Copyright (c) The Hayro Authors
//
// Use of this source code is governed by the Apache-2.0 or MIT license that
// can be found in the LICENSE-APACHE and LICENSE-MIT files.

package jbig2

// This file ports hayro-jbig2 tests/malformed.rs.

import "testing"

// TestMalformed runs the tests of hayro-jbig2 tests/malformed.rs. Each subtest has the
// name of a test of upstream.
func TestMalformed(t *testing.T) {
	t.Run("reused_contexts_of_another_template", func(t *testing.T) {
		checkMalformed(t, "malformed_reused_contexts_of_another_template")
	})
	t.Run("mmr_region_with_height_0", func(t *testing.T) {
		checkMalformed(t, "malformed_mmr_region_with_height_0")
	})
	t.Run("huffman_value_out_of_range", func(t *testing.T) {
		checkMalformed(t, "malformed_huffman_value_out_of_range")
	})
	t.Run("text_placement_out_of_range", func(t *testing.T) {
		checkMalformed(t, "malformed_text_placement_out_of_range")
	})
	t.Run("symbol_dictionary_empty_export_runs", func(t *testing.T) {
		checkMalformed(t, "malformed_symbol_dictionary_empty_export_runs")
	})
	t.Run("symbol_dictionary_empty_height_classes", func(t *testing.T) {
		checkMalformed(t, "malformed_symbol_dictionary_empty_height_classes")
	})
	t.Run("symbol_dictionary_garbage_data", func(t *testing.T) {
		checkMalformed(t, "malformed_symbol_dictionary_garbage_data")
	})
}
