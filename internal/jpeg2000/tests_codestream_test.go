// Copyright (c) The Hayro Authors
//
// Use of this source code is governed by the Apache-2.0 or MIT license that
// can be found in the LICENSE-APACHE and LICENSE-MIT files.

package jpeg2000

// This file ports hayro-jpeg2000 tests/codestream.rs.

import "testing"

func TestCodestream(t *testing.T) {
	t.Run("gray", func(t *testing.T) {
		check(t, "codestream_gray", reference(t, "main_r"))
	})
	t.Run("odd_size", func(t *testing.T) {
		check(t, "codestream_odd_size", reference(t, "main_r_crop_5_3_61_47"))
	})
	t.Run("tiny", func(t *testing.T) {
		check(t, "codestream_tiny", reference(t, "main_r_crop_60_40_3_5"))
	})
	t.Run("single_row", func(t *testing.T) {
		check(t, "codestream_single_row", reference(t, "main_r_crop_0_45_128_1"))
	})
	t.Run("single_row_odd_offset", func(t *testing.T) {
		check(t, "codestream_single_row_odd_offset", reference(t, "main_r_crop_0_45_128_1"))
	})
	t.Run("tall_narrow_subsampled_offset", func(t *testing.T) {
		check(t, "codestream_tall_narrow_subsampled_offset", reference(t, "main_rg_crop_40_0_5_96_sub_1x1_2x1_at_123_0"))
	})
	t.Run("offset", func(t *testing.T) {
		check(t, "codestream_offset", reference(t, "main_r"))
	})
	t.Run("offset_large", func(t *testing.T) {
		check(t, "codestream_offset_large", reference(t, "main_r"))
	})
	t.Run("tiles_partial", func(t *testing.T) {
		check(t, "codestream_tiles_partial", reference(t, "main_rgb"))
	})
	t.Run("tiles_offset_many", func(t *testing.T) {
		check(t, "codestream_tiles_offset_many", reference(t, "main_r"))
	})
	t.Run("tile_larger_than_image", func(t *testing.T) {
		check(t, "codestream_tile_larger_than_image", reference(t, "main_rgb"))
	})
	t.Run("precision_4", func(t *testing.T) {
		check(t, "codestream_precision_4", reference(t, "main_r_4bit"))
	})
	t.Run("precision_10", func(t *testing.T) {
		check(t, "codestream_precision_10", reference(t, "main_r_10bit"))
	})
	t.Run("precision_12", func(t *testing.T) {
		check(t, "codestream_precision_12", reference(t, "main_r_12bit"))
	})
	t.Run("precision_16", func(t *testing.T) {
		check(t, "codestream_precision_16", reference(t, "main_r_16bit"))
	})
	t.Run("precision_mixed", func(t *testing.T) {
		check(t, "codestream_precision_mixed", reference(t, "main_rgba_5_5_5_1bit"))
	})
	t.Run("signed_4", func(t *testing.T) {
		check(t, "codestream_signed_4", reference(t, "main_r_4bit"))
	})
	t.Run("signed_12", func(t *testing.T) {
		check(t, "codestream_signed_12", reference(t, "main_r_12bit"))
	})
	t.Run("signed_16", func(t *testing.T) {
		check(t, "codestream_signed_16", reference(t, "main_r_16bit"))
	})
	t.Run("subsampling_2x1", func(t *testing.T) {
		check(t, "codestream_subsampling_2x1", reference(t, "main_rg_sub_1x1_2x1"))
	})
	t.Run("subsampling_1x2", func(t *testing.T) {
		check(t, "codestream_subsampling_1x2", reference(t, "main_rg_sub_1x1_1x2"))
	})
	t.Run("subsampling_2x2", func(t *testing.T) {
		check(t, "codestream_subsampling_2x2", reference(t, "main_rg_sub_1x1_2x2"))
	})
	t.Run("subsampling_4x1", func(t *testing.T) {
		check(t, "codestream_subsampling_4x1", reference(t, "main_rg_sub_1x1_4x1"))
	})
	t.Run("subsampling_4x4", func(t *testing.T) {
		check(t, "codestream_subsampling_4x4", reference(t, "main_rg_sub_1x1_4x4"))
	})
	t.Run("subsampling_odd_factors", func(t *testing.T) {
		check(t, "codestream_subsampling_odd_factors", reference(t, "main_rgb_sub_1x1_11x5_13x6"))
	})
	t.Run("subsampling_per_component", func(t *testing.T) {
		check(t, "codestream_subsampling_per_component", reference(t, "main_rgba_sub_1x1_1x2_2x1_2x2"))
	})
	t.Run("subsampling_all_components", func(t *testing.T) {
		check(t, "codestream_subsampling_all_components", reference(t, "main_rgb_sub_2x2_2x2_2x2"))
	})
	t.Run("subsampling_all_odd", func(t *testing.T) {
		check(t, "codestream_subsampling_all_odd", reference(t, "main_rgb_sub_17x7_17x7_17x7"))
	})
	t.Run("subsampling_with_offset", func(t *testing.T) {
		check(t, "codestream_subsampling_with_offset", reference(t, "main_r_sub_5x3_at_1_160"))
	})
	t.Run("levels_0", func(t *testing.T) {
		check(t, "codestream_levels_0", reference(t, "main_r"))
	})
	t.Run("levels_1", func(t *testing.T) {
		check(t, "codestream_levels_1", reference(t, "main_r"))
	})
	t.Run("levels_8", func(t *testing.T) {
		check(t, "codestream_levels_8", reference(t, "main_r"))
	})
	t.Run("levels_more_than_size", func(t *testing.T) {
		check(t, "codestream_levels_more_than_size", reference(t, "main_r_crop_10_10_17_9"))
	})
	t.Run("codeblock_4x16", func(t *testing.T) {
		check(t, "codestream_codeblock_4x16", reference(t, "main_r"))
	})
	t.Run("codeblock_8x64", func(t *testing.T) {
		check(t, "codestream_codeblock_8x64", reference(t, "main_r"))
	})
	t.Run("codeblock_64x32", func(t *testing.T) {
		check(t, "codestream_codeblock_64x32", reference(t, "main_r"))
	})
	t.Run("codeblock_32x32", func(t *testing.T) {
		check(t, "codestream_codeblock_32x32", reference(t, "main_r"))
	})
	t.Run("codeblock_128x32", func(t *testing.T) {
		check(t, "codestream_codeblock_128x32", reference(t, "main_r"))
	})
	t.Run("codeblock_precinct_limited", func(t *testing.T) {
		check(t, "codestream_codeblock_precinct_limited", reference(t, "main_r"))
	})
	t.Run("precincts", func(t *testing.T) {
		check(t, "codestream_precincts", reference(t, "main_rgb"))
	})
	t.Run("precincts_non_square", func(t *testing.T) {
		check(t, "codestream_precincts_non_square", reference(t, "main_r"))
	})
	t.Run("precincts_1x1", func(t *testing.T) {
		check(t, "codestream_precincts_1x1", reference(t, "main_r_crop_40_30_8_12"))
	})
	t.Run("guard_bits_zero", func(t *testing.T) {
		check(t, "codestream_guard_bits_zero", reference(t, "main_r"))
	})
	t.Run("guard_bits_high", func(t *testing.T) {
		check(t, "codestream_guard_bits_high", reference(t, "main_r"))
	})
	t.Run("tlm", func(t *testing.T) {
		check(t, "codestream_tlm", reference(t, "main_rgb"))
	})
	t.Run("plt", func(t *testing.T) {
		check(t, "codestream_plt", reference(t, "main_rgb"))
	})
	t.Run("components_5", func(t *testing.T) {
		checkComponents(t, "codestream_components_5", reference(t, "main_rgbar"))
	})
	t.Run("components_5_subsampled", func(t *testing.T) {
		checkComponents(t, "codestream_components_5_subsampled", reference(t, "main_rgbar_sub_1x1_1x1_1x1_2x2_2x2"))
	})
	t.Run("crg", func(t *testing.T) {
		check(t, "codestream_crg", reference(t, "main_rgb"))
	})
	t.Run("reserved_marker", func(t *testing.T) {
		check(t, "codestream_reserved_marker", reference(t, "main_r"))
	})
	t.Run("tile_size_huge", func(t *testing.T) {
		check(t, "codestream_tile_size_huge", reference(t, "main_r"))
	})
	t.Run("tile_cod_progression", func(t *testing.T) {
		check(t, "codestream_tile_cod_progression", reference(t, "main_rgb"))
	})
	t.Run("tile_cod_layers", func(t *testing.T) {
		check(t, "codestream_tile_cod_layers", reference(t, "main_r"))
	})
	t.Run("tile_cod_fewer_levels", func(t *testing.T) {
		check(t, "codestream_tile_cod_fewer_levels", reference(t, "main_r"))
	})
	t.Run("tile_qcd", func(t *testing.T) {
		check(t, "codestream_tile_qcd", reference(t, "main_r"))
	})
	t.Run("levels_per_component", func(t *testing.T) {
		check(t, "codestream_levels_per_component", reference(t, "main_rgb"))
	})
	t.Run("coc_style_per_component", func(t *testing.T) {
		check(t, "codestream_coc_style_per_component", reference(t, "main_rgb"))
	})
	t.Run("qcc_per_component", func(t *testing.T) {
		check(t, "codestream_qcc_per_component", reference(t, "main_rgb"))
	})
	t.Run("tile_coc_qcc", func(t *testing.T) {
		check(t, "codestream_tile_coc_qcc", reference(t, "main_rgb"))
	})
	t.Run("com_in_tile_part", func(t *testing.T) {
		check(t, "codestream_com_in_tile_part", reference(t, "main_r"))
	})
	t.Run("coc_wavelet_per_component", func(t *testing.T) {
		check(t, "codestream_coc_wavelet_per_component", reference(t, "main_rgb").withTolerance(1))
	})
}
