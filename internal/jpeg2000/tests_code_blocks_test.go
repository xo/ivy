// Copyright (c) The Hayro Authors
//
// Use of this source code is governed by the Apache-2.0 or MIT license that
// can be found in the LICENSE-APACHE and LICENSE-MIT files.

package jpeg2000

// This file ports hayro-jpeg2000 tests/code_blocks.rs.

import "testing"

func TestCodeBlocks(t *testing.T) {
	t.Run("bypass", func(t *testing.T) {
		check(t, "code_blocks_bypass", reference(t, "main_r"))
	})
	t.Run("reset", func(t *testing.T) {
		check(t, "code_blocks_reset", reference(t, "main_r"))
	})
	t.Run("termall", func(t *testing.T) {
		check(t, "code_blocks_termall", reference(t, "main_r"))
	})
	t.Run("vcausal", func(t *testing.T) {
		check(t, "code_blocks_vcausal", reference(t, "main_r"))
	})
	t.Run("pterm", func(t *testing.T) {
		check(t, "code_blocks_pterm", reference(t, "main_r"))
	})
	t.Run("segsym", func(t *testing.T) {
		check(t, "code_blocks_segsym", reference(t, "main_r"))
	})
	t.Run("bypass_termall", func(t *testing.T) {
		check(t, "code_blocks_bypass_termall", reference(t, "main_r"))
	})
	t.Run("bypass_pterm", func(t *testing.T) {
		check(t, "code_blocks_bypass_pterm", reference(t, "main_r"))
	})
	t.Run("termall_pterm", func(t *testing.T) {
		check(t, "code_blocks_termall_pterm", reference(t, "main_r"))
	})
	t.Run("bypass_termall_pterm", func(t *testing.T) {
		check(t, "code_blocks_bypass_termall_pterm", reference(t, "main_r"))
	})
	t.Run("reset_termall", func(t *testing.T) {
		check(t, "code_blocks_reset_termall", reference(t, "main_r"))
	})
	t.Run("termall_segsym", func(t *testing.T) {
		check(t, "code_blocks_termall_segsym", reference(t, "main_r"))
	})
	t.Run("reset_termall_segsym", func(t *testing.T) {
		check(t, "code_blocks_reset_termall_segsym", reference(t, "main_r"))
	})
	t.Run("all_but_termall", func(t *testing.T) {
		check(t, "code_blocks_all_but_termall", reference(t, "main_r"))
	})
	t.Run("all_but_pterm", func(t *testing.T) {
		check(t, "code_blocks_all_but_pterm", reference(t, "main_r"))
	})
	t.Run("all_styles", func(t *testing.T) {
		check(t, "code_blocks_all_styles", reference(t, "main_r"))
	})
	t.Run("termall_layers", func(t *testing.T) {
		check(t, "code_blocks_termall_layers", reference(t, "main_r"))
	})
	t.Run("bypass_layers", func(t *testing.T) {
		check(t, "code_blocks_bypass_layers", reference(t, "main_r"))
	})
	t.Run("bypass_segment_spans_layers", func(t *testing.T) {
		check(t, "code_blocks_bypass_segment_spans_layers", reference(t, "main_r"))
	})
	t.Run("missing_bitplanes_exceed", func(t *testing.T) {
		check(t, "code_blocks_missing_bitplanes_exceed", reference(t, "code_blocks_missing_bitplanes_exceed"))
	})
	t.Run("bypass_data_exhausted", func(t *testing.T) {
		check(t, "code_blocks_bypass_data_exhausted", reference(t, "main_rgb"))
	})
	t.Run("no_coding_passes", func(t *testing.T) {
		check(t, "code_blocks_no_coding_passes", reference(t, "code_blocks_no_coding_passes"))
	})
}
