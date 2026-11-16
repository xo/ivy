// Copyright (c) The Hayro Authors
//
// Use of this source code is governed by the Apache-2.0 or MIT license that
// can be found in the LICENSE-APACHE and LICENSE-MIT files.

package jpeg2000

// This file ports hayro-jpeg2000 tests/packets.rs.

import "testing"

func TestPackets(t *testing.T) {
	t.Run("progression_lrcp", func(t *testing.T) {
		check(t, "packets_progression_lrcp", reference(t, "main_rgba"))
	})
	t.Run("progression_rlcp", func(t *testing.T) {
		check(t, "packets_progression_rlcp", reference(t, "main_rgba"))
	})
	t.Run("progression_rpcl", func(t *testing.T) {
		check(t, "packets_progression_rpcl", reference(t, "main_rgba"))
	})
	t.Run("progression_pcrl", func(t *testing.T) {
		check(t, "packets_progression_pcrl", reference(t, "main_rgba"))
	})
	t.Run("progression_cprl", func(t *testing.T) {
		check(t, "packets_progression_cprl", reference(t, "main_rgba"))
	})
	t.Run("sop", func(t *testing.T) {
		check(t, "packets_sop", reference(t, "main_r"))
	})
	t.Run("eph", func(t *testing.T) {
		check(t, "packets_eph", reference(t, "main_r"))
	})
	t.Run("sop_eph", func(t *testing.T) {
		check(t, "packets_sop_eph", reference(t, "main_r"))
	})
	t.Run("layers_many", func(t *testing.T) {
		check(t, "packets_layers_many", reference(t, "main_r"))
	})
	t.Run("late_inclusion", func(t *testing.T) {
		check(t, "packets_late_inclusion", reference(t, "main_r"))
	})
	t.Run("lblock_large", func(t *testing.T) {
		check(t, "packets_lblock_large", reference(t, "stress_16bit_crop_64_0_64_64"))
	})
	t.Run("passes_codeword_long", func(t *testing.T) {
		check(t, "packets_passes_codeword_long", reference(t, "stress_16bit_crop_64_0_64_64"))
	})
	t.Run("sop_missing", func(t *testing.T) {
		check(t, "packets_sop_missing", reference(t, "main_r"))
	})
	t.Run("ppm", func(t *testing.T) {
		check(t, "packets_ppm", reference(t, "main_rgb"))
	})
	t.Run("ppm_sop_eph", func(t *testing.T) {
		check(t, "packets_ppm_sop_eph", reference(t, "main_rgb"))
	})
	t.Run("ppm_many_segments", func(t *testing.T) {
		check(t, "packets_ppm_many_segments", reference(t, "main_rgb"))
	})
	t.Run("ppm_spanning_segments", func(t *testing.T) {
		check(t, "packets_ppm_spanning_segments", reference(t, "main_r"))
	})
	t.Run("ppm_many_tiles", func(t *testing.T) {
		check(t, "packets_ppm_many_tiles", reference(t, "main_r"))
	})
	t.Run("ppt", func(t *testing.T) {
		check(t, "packets_ppt", reference(t, "main_rgb"))
	})
	t.Run("ppt_many_segments", func(t *testing.T) {
		check(t, "packets_ppt_many_segments", reference(t, "main_rgb"))
	})
	t.Run("ppt_sop_eph_tiny_tiles", func(t *testing.T) {
		check(t, "packets_ppt_sop_eph_tiny_tiles", reference(t, "main_rgb_crop_50_40_12_12"))
	})
	t.Run("poc_lrcp", func(t *testing.T) {
		check(t, "packets_poc_lrcp", reference(t, "main_rgb"))
	})
	t.Run("poc_rlcp", func(t *testing.T) {
		check(t, "packets_poc_rlcp", reference(t, "main_rgb"))
	})
	t.Run("poc_rpcl", func(t *testing.T) {
		check(t, "packets_poc_rpcl", reference(t, "main_rgb"))
	})
	t.Run("poc_pcrl", func(t *testing.T) {
		check(t, "packets_poc_pcrl", reference(t, "main_rgb"))
	})
	t.Run("poc_cprl", func(t *testing.T) {
		check(t, "packets_poc_cprl", reference(t, "main_rgb"))
	})
	t.Run("poc_main_header", func(t *testing.T) {
		check(t, "packets_poc_main_header", reference(t, "main_rgb"))
	})
	t.Run("poc_main_header_tile_override", func(t *testing.T) {
		check(t, "packets_poc_main_header_tile_override", reference(t, "main_rgb"))
	})
	t.Run("poc_per_tile", func(t *testing.T) {
		check(t, "packets_poc_per_tile", reference(t, "main_rgb"))
	})
	t.Run("poc_main_header_ranges", func(t *testing.T) {
		check(t, "packets_poc_main_header_ranges", reference(t, "main_rgb"))
	})
	t.Run("poc_clamped_values", func(t *testing.T) {
		check(t, "packets_poc_clamped_values", reference(t, "main_rgb"))
	})
	t.Run("poc_subsampled", func(t *testing.T) {
		check(t, "packets_poc_subsampled", reference(t, "main_rgb_sub_1x1_2x2_1x2"))
	})
	t.Run("poc_offset", func(t *testing.T) {
		check(t, "packets_poc_offset", reference(t, "main_rgb"))
	})
	t.Run("poc_tile_parts", func(t *testing.T) {
		check(t, "packets_poc_tile_parts", reference(t, "main_rgb"))
	})
	t.Run("mixed_levels_lrcp", func(t *testing.T) {
		check(t, "packets_mixed_levels_lrcp", reference(t, "main_rgb"))
	})
	t.Run("mixed_levels_rlcp", func(t *testing.T) {
		check(t, "packets_mixed_levels_rlcp", reference(t, "main_rgb"))
	})
	t.Run("mixed_levels_rpcl", func(t *testing.T) {
		check(t, "packets_mixed_levels_rpcl", reference(t, "main_rgb"))
	})
	t.Run("mixed_levels_pcrl", func(t *testing.T) {
		check(t, "packets_mixed_levels_pcrl", reference(t, "main_rgb"))
	})
	t.Run("mixed_levels_cprl", func(t *testing.T) {
		check(t, "packets_mixed_levels_cprl", reference(t, "main_rgb"))
	})
	t.Run("empty_packets", func(t *testing.T) {
		check(t, "packets_empty_packets", reference(t, "main_rgba"))
	})
	t.Run("eph_empty_packets", func(t *testing.T) {
		check(t, "packets_eph_empty_packets", reference(t, "main_rgba"))
	})
	t.Run("invalid_stuff_bit", func(t *testing.T) {
		check(t, "packets_invalid_stuff_bit", reference(t, "packets_invalid_stuff_bit"))
	})
}
