// Copyright (c) The Hayro Authors
//
// Use of this source code is governed by the Apache-2.0 or MIT license that
// can be found in the LICENSE-APACHE and LICENSE-MIT files.

package jbig2

// This file ports hayro-jbig2 tests/symbol.rs.

import "testing"

// TestSymbol runs the tests of hayro-jbig2 tests/symbol.rs. Each subtest has the
// name of a test of upstream.
func TestSymbol(t *testing.T) {
	t.Run("basic", func(t *testing.T) {
		check(t, "symbol", "honeycomb")
	})
	t.Run("empty", func(t *testing.T) {
		check(t, "symbol_empty", "honeycomb")
	})
	t.Run("many_dictionaries", func(t *testing.T) {
		check(t, "symbol_many_dictionaries", "honeycomb")
	})
	t.Run("large_segment_numbers", func(t *testing.T) {
		check(t, "symbol_large_segment_numbers", "honeycomb")
	})
	t.Run("template2", func(t *testing.T) {
		check(t, "symbol_template2", "honeycomb")
	})
	t.Run("custom_at", func(t *testing.T) {
		check(t, "symbol_custom_at", "honeycomb")
	})
	t.Run("refine_one", func(t *testing.T) {
		check(t, "symbol_refine_one", "honeycomb")
	})
	t.Run("refine_offsets", func(t *testing.T) {
		check(t, "symbol_refine_offsets", "honeycomb")
	})
	t.Run("huffman_refine_offsets", func(t *testing.T) {
		check(t, "symbol_huffman_refine_offsets", "honeycomb")
	})
	t.Run("refine_one_custom_at", func(t *testing.T) {
		check(t, "symbol_refine_one_custom_at", "honeycomb")
	})
	t.Run("refine_one_template1", func(t *testing.T) {
		check(t, "symbol_refine_one_template1", "honeycomb")
	})
	t.Run("aggregate", func(t *testing.T) {
		check(t, "symbol_aggregate", "honeycomb")
	})
	t.Run("aggregate_then_refine", func(t *testing.T) {
		check(t, "symbol_aggregate_then_refine", "honeycomb")
	})
	t.Run("huffman_aggregate_then_refine", func(t *testing.T) {
		check(t, "symbol_huffman_aggregate_then_refine", "honeycomb")
	})
	t.Run("refine_text_refine", func(t *testing.T) {
		check(t, "symbol_refine_text_refine", "honeycomb")
	})
	t.Run("huffman_refine_one", func(t *testing.T) {
		check(t, "symbol_huffman_refine_one", "honeycomb")
	})
	t.Run("huffman_aggregate", func(t *testing.T) {
		check(t, "symbol_huffman_aggregate", "honeycomb")
	})
	t.Run("huffman_refine_text_refine", func(t *testing.T) {
		check(t, "symbol_huffman_refine_text_refine", "honeycomb")
	})
	t.Run("refine_text_refine_export", func(t *testing.T) {
		check(t, "symbol_refine_text_refine_export", "honeycomb")
	})
	t.Run("huffman_refine_text_refine_export", func(t *testing.T) {
		check(t, "symbol_huffman_refine_text_refine_export", "honeycomb")
	})
	t.Run("global", func(t *testing.T) {
		check(t, "symbol_global", "honeycomb")
	})
	t.Run("many_symbols", func(t *testing.T) {
		check(t, "symbol_many_symbols", "honeycomb")
	})
	t.Run("context_reuse", func(t *testing.T) {
		check(t, "symbol_context_reuse", "honeycomb")
	})
	t.Run("context_reuse_refine", func(t *testing.T) {
		check(t, "symbol_context_reuse_refine", "honeycomb")
	})
	t.Run("huffman_refine_after_arithmetic", func(t *testing.T) {
		check(t, "symbol_huffman_refine_after_arithmetic", "honeycomb")
	})
	t.Run("huffman_context_reuse_refine", func(t *testing.T) {
		check(t, "symbol_huffman_context_reuse_refine", "honeycomb")
	})
	t.Run("huffman_context_reuse_refine_table", func(t *testing.T) {
		check(t, "symbol_huffman_context_reuse_refine_table", "honeycomb")
	})
}
