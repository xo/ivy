// Copyright (c) The Hayro Authors
//
// Use of this source code is governed by the Apache-2.0 or MIT license that
// can be found in the LICENSE-APACHE and LICENSE-MIT files.

package jbig2

// This file ports hayro-jbig2 tests/memory.rs.

import "testing"

// TestMemory runs the tests of hayro-jbig2 tests/memory.rs. Each subtest has the
// name of a test of upstream.
func TestMemory(t *testing.T) {
	t.Run("immediate_regions", func(t *testing.T) {
		checkWithMemoryLimit(t, "memory_immediate_regions", "truchet", 49152)
	})
	t.Run("intermediate_regions", func(t *testing.T) {
		checkMemoryLimitExceeded(t, "memory_intermediate_regions", 49152)
	})
	t.Run("page", func(t *testing.T) {
		checkMemoryLimitExceeded(t, "memory_page", 49152)
	})
	t.Run("halftone_grid", func(t *testing.T) {
		checkMemoryLimitExceeded(t, "memory_halftone_grid", 49152)
	})
	t.Run("symbol_dictionary_large_symbol", func(t *testing.T) {
		checkMemoryLimitExceeded(t, "memory_symbol_dictionary_large_symbol", 49152)
	})
	t.Run("symbol_dictionary_many_symbols", func(t *testing.T) {
		checkMemoryLimitExceeded(t, "memory_symbol_dictionary_many_symbols", 8192)
	})
	t.Run("symbol_dictionary_global", func(t *testing.T) {
		checkMemoryLimitExceeded(t, "memory_symbol_dictionary_global", 49152)
	})
}
