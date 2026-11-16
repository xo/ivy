// Copyright (c) The Hayro Authors
//
// Use of this source code is governed by the Apache-2.0 or MIT license that
// can be found in the LICENSE-APACHE and LICENSE-MIT files.

package jbig2

// This file ports hayro-jbig2 tests/huffman.rs.

import "testing"

// TestHuffman runs the tests of hayro-jbig2 tests/huffman.rs. Each subtest has the
// name of a test of upstream.
func TestHuffman(t *testing.T) {
	t.Run("text_refine_large_offsets", func(t *testing.T) {
		check(t, "huffman_text_refine_large_offsets", "honeycomb")
	})
	t.Run("custom_aggregate_count", func(t *testing.T) {
		check(t, "huffman_custom_aggregate_count", "honeycomb")
	})
	t.Run("many_symbols", func(t *testing.T) {
		check(t, "huffman_many_symbols", "honeycomb")
	})
	t.Run("standard_tables", func(t *testing.T) {
		check(t, "huffman_standard_tables", "honeycomb")
	})
	t.Run("tables_b10_b13", func(t *testing.T) {
		check(t, "huffman_tables_b10_b13", "honeycomb")
	})
	t.Run("tables_b5_b3_b7_b9_b12", func(t *testing.T) {
		check(t, "huffman_tables_b5_b3_b7_b9_b12", "honeycomb")
	})
	t.Run("table_b12_transposed", func(t *testing.T) {
		check(t, "huffman_table_b12_transposed", "honeycomb")
	})
	t.Run("uncompressed_symbols", func(t *testing.T) {
		check(t, "huffman_uncompressed_symbols", "honeycomb")
	})
	t.Run("custom_tables", func(t *testing.T) {
		check(t, "huffman_custom_tables", "honeycomb")
	})
	t.Run("custom_table_htlow_min", func(t *testing.T) {
		check(t, "huffman_custom_table_htlow_min", "honeycomb")
	})
	t.Run("custom_table_long_prefixes", func(t *testing.T) {
		check(t, "huffman_custom_table_long_prefixes", "honeycomb")
	})
	t.Run("symbol_id_run_codes", func(t *testing.T) {
		check(t, "huffman_symbol_id_run_codes", "honeycomb")
	})
	t.Run("trailing_unused_symbols", func(t *testing.T) {
		check(t, "huffman_trailing_unused_symbols", "honeycomb")
	})
	t.Run("text_refine", func(t *testing.T) {
		check(t, "huffman_text_refine", "honeycomb")
	})
	t.Run("text_refine_b15", func(t *testing.T) {
		check(t, "huffman_text_refine_b15", "honeycomb")
	})
	t.Run("text_refine_different_offset_tables", func(t *testing.T) {
		check(t, "huffman_text_refine_different_offset_tables", "honeycomb")
	})
	t.Run("text_refine_custom_x_offset", func(t *testing.T) {
		check(t, "huffman_text_refine_custom_x_offset", "honeycomb")
	})
	t.Run("text_refine_custom", func(t *testing.T) {
		check(t, "huffman_text_refine_custom", "honeycomb")
	})
	t.Run("text_refine_custom_offsets", func(t *testing.T) {
		check(t, "huffman_text_refine_custom_offsets", "honeycomb")
	})
	t.Run("text_refine_custom_sizes", func(t *testing.T) {
		check(t, "huffman_text_refine_custom_sizes", "honeycomb")
	})
	t.Run("text_refine_custom_sizes_offsets", func(t *testing.T) {
		check(t, "huffman_text_refine_custom_sizes_offsets", "honeycomb")
	})
	t.Run("text_refine_custom_data_size", func(t *testing.T) {
		check(t, "huffman_text_refine_custom_data_size", "honeycomb")
	})
	t.Run("text_refine_global_tables", func(t *testing.T) {
		check(t, "huffman_text_refine_global_tables", "honeycomb")
	})
}
