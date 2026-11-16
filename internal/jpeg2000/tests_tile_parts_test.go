// Copyright (c) The Hayro Authors
//
// Use of this source code is governed by the Apache-2.0 or MIT license that
// can be found in the LICENSE-APACHE and LICENSE-MIT files.

package jpeg2000

// This file ports hayro-jpeg2000 tests/tile_parts.rs.

import "testing"

func TestTileParts(t *testing.T) {
	t.Run("split_resolution", func(t *testing.T) {
		check(t, "tile_parts_split_resolution", reference(t, "main_r"))
	})
	t.Run("split_layer", func(t *testing.T) {
		check(t, "tile_parts_split_layer", reference(t, "main_r"))
	})
	t.Run("split_component", func(t *testing.T) {
		check(t, "tile_parts_split_component", reference(t, "main_rgb"))
	})
	t.Run("interleaved", func(t *testing.T) {
		check(t, "tile_parts_interleaved", reference(t, "main_r"))
	})
	t.Run("tnsot_zero", func(t *testing.T) {
		check(t, "tile_parts_tnsot_zero", reference(t, "main_r"))
	})
	t.Run("tnsot_too_small", func(t *testing.T) {
		check(t, "tile_parts_tnsot_too_small", reference(t, "main_r"))
	})
	t.Run("index_overflow", func(t *testing.T) {
		check(t, "tile_parts_index_overflow", reference(t, "main_r"))
	})
	t.Run("psot_zero", func(t *testing.T) {
		check(t, "tile_parts_psot_zero", reference(t, "main_r"))
	})
	t.Run("psot_zero_padding", func(t *testing.T) {
		check(t, "tile_parts_psot_zero_padding", reference(t, "main_r"))
	})
	t.Run("missing_eoc", func(t *testing.T) {
		check(t, "tile_parts_missing_eoc", reference(t, "main_r"))
	})
	t.Run("bytes_after_eoc", func(t *testing.T) {
		check(t, "tile_parts_bytes_after_eoc", reference(t, "main_r"))
	})
	t.Run("garbage_before_eoc", func(t *testing.T) {
		check(t, "tile_parts_garbage_before_eoc", reference(t, "main_r"))
	})
	t.Run("truncated_end", func(t *testing.T) {
		check(t, "tile_parts_truncated_end", reference(t, "main_r"))
	})
	t.Run("trailing_packet_data", func(t *testing.T) {
		check(t, "tile_parts_trailing_packet_data", reference(t, "main_r"))
	})
	t.Run("truncated_layers", func(t *testing.T) {
		check(t, "tile_parts_truncated_layers", reference(t, "tile_parts_truncated_layers"))
	})
	t.Run("sot_only_tile_part", func(t *testing.T) {
		check(t, "tile_parts_sot_only_tile_part", reference(t, "main_r"))
	})
	t.Run("cod_qcd_in_later_tile_part", func(t *testing.T) {
		check(t, "tile_parts_cod_qcd_in_later_tile_part", reference(t, "main_r"))
	})
	t.Run("truncated_in_packet_header", func(t *testing.T) {
		check(t, "tile_parts_truncated_in_packet_header", reference(t, "tile_parts_truncated_in_packet_header"))
	})
	t.Run("truncated_in_packet_body", func(t *testing.T) {
		check(t, "tile_parts_truncated_in_packet_body", reference(t, "tile_parts_truncated_in_packet_body"))
	})
}
