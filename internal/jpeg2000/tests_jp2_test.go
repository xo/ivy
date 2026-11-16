// Copyright (c) The Hayro Authors
//
// Use of this source code is governed by the Apache-2.0 or MIT license that
// can be found in the LICENSE-APACHE and LICENSE-MIT files.

package jpeg2000

// This file ports hayro-jpeg2000 tests/jp2.rs.

import "testing"

func TestJp2(t *testing.T) {
	t.Run("colr_srgb", func(t *testing.T) {
		check(t, "jp2_colr_srgb", reference(t, "main_rgb"))
	})
	t.Run("colr_greyscale", func(t *testing.T) {
		check(t, "jp2_colr_greyscale", reference(t, "main_r"))
	})
	t.Run("colr_esrgb", func(t *testing.T) {
		check(t, "jp2_colr_esrgb", reference(t, "main_rgb"))
	})
	t.Run("gray_alpha", func(t *testing.T) {
		check(t, "jp2_gray_alpha", reference(t, "main_ra"))
	})
	t.Run("alpha_whole_image", func(t *testing.T) {
		check(t, "jp2_alpha_whole_image", reference(t, "main_ra"))
	})
	t.Run("rgba", func(t *testing.T) {
		check(t, "jp2_rgba", reference(t, "main_rgba"))
	})
	t.Run("cdef_alpha_not_last", func(t *testing.T) {
		check(t, "jp2_cdef_alpha_not_last", reference(t, "main_rgba"))
	})
	t.Run("cdef_reordered", func(t *testing.T) {
		check(t, "jp2_cdef_reordered", reference(t, "main_rgba"))
	})
	t.Run("bpcc", func(t *testing.T) {
		check(t, "jp2_bpcc", reference(t, "main_rgba_5_5_5_1bit"))
	})
	t.Run("jpx_brand_rreq", func(t *testing.T) {
		check(t, "jp2_jpx_brand_rreq", reference(t, "main_rgb"))
	})
	t.Run("extra_boxes", func(t *testing.T) {
		check(t, "jp2_extra_boxes", reference(t, "main_rgb"))
	})
	t.Run("xl_box", func(t *testing.T) {
		check(t, "jp2_xl_box", reference(t, "main_rgb"))
	})
	t.Run("jp2c_length_zero", func(t *testing.T) {
		check(t, "jp2_jp2c_length_zero", reference(t, "main_rgb"))
	})
	t.Run("box_longer_than_file", func(t *testing.T) {
		check(t, "jp2_box_longer_than_file", reference(t, "main_rgb"))
	})
	t.Run("trailing_box_longer_than_file", func(t *testing.T) {
		check(t, "jp2_trailing_box_longer_than_file", reference(t, "main_rgb"))
	})
	t.Run("trailing_zero_box", func(t *testing.T) {
		check(t, "jp2_trailing_zero_box", reference(t, "main_rgb"))
	})
	t.Run("duplicate_ihdr", func(t *testing.T) {
		check(t, "jp2_duplicate_ihdr", reference(t, "main_rgb"))
	})
	t.Run("colr_sycc_444", func(t *testing.T) {
		check(t, "jp2_colr_sycc_444", reference(t, "jp2_colr_sycc_444"))
	})
	t.Run("colr_sycc_422", func(t *testing.T) {
		check(t, "jp2_colr_sycc_422", reference(t, "jp2_colr_sycc_422"))
	})
	t.Run("colr_sycc_420", func(t *testing.T) {
		check(t, "jp2_colr_sycc_420", reference(t, "jp2_colr_sycc_420"))
	})
	t.Run("cdef_reversed_sycc", func(t *testing.T) {
		check(t, "jp2_cdef_reversed_sycc", reference(t, "jp2_cdef_reversed_sycc"))
	})
	t.Run("colr_first_wins", func(t *testing.T) {
		check(t, "jp2_colr_first_wins", reference(t, "jp2_cdef_reversed_sycc"))
	})
	t.Run("colr_ycck", func(t *testing.T) {
		check(t, "jp2_colr_ycck", reference(t, "jp2_colr_ycck"))
	})
}
