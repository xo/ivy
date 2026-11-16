// Copyright (c) The Hayro Authors
//
// Use of this source code is governed by the Apache-2.0 or MIT license that
// can be found in the LICENSE-APACHE and LICENSE-MIT files.

package jpeg2000

// This file ports hayro-jpeg2000 tests/integration.rs.

import "testing"

func TestIntegration(t *testing.T) {
	t.Run("gray_alpha", func(t *testing.T) {
		checkOutput(t, "jp2_gray_alpha", reference(t, "main_ra"))
	})
	t.Run("scaled_12bit", func(t *testing.T) {
		checkOutput(t, "codestream_precision_12", reference(t, "main_r_12bit"))
	})
	t.Run("scaled_mixed", func(t *testing.T) {
		checkOutput(t, "jp2_bpcc", reference(t, "main_rgba_5_5_5_1bit"))
	})
	t.Run("cmyk", func(t *testing.T) {
		checkOutput(t, "integration_cmyk", reference(t, "integration_cmyk").withTolerance(2))
	})
	t.Run("cmyk_alpha", func(t *testing.T) {
		checkOutput(t, "integration_cmyk_alpha", reference(t, "integration_cmyk_alpha").withTolerance(2))
	})
	t.Run("icc_rgb", func(t *testing.T) {
		checkOutput(t, "integration_icc_rgb", reference(t, "integration_icc_rgb"))
	})
	t.Run("icc_method_3_ignored", func(t *testing.T) {
		checkOutput(t, "integration_icc_method_3_ignored", reference(t, "integration_icc_method_3_ignored"))
	})
	t.Run("cmyk_method_3_ignored", func(t *testing.T) {
		checkOutput(t, "integration_cmyk_method_3_ignored", reference(t, "integration_cmyk").withTolerance(2))
	})
	t.Run("colr_romm_rgb", func(t *testing.T) {
		checkOutput(t, "integration_colr_romm_rgb", reference(t, "integration_icc_rgb"))
	})
	t.Run("icc_gray", func(t *testing.T) {
		checkOutput(t, "integration_icc_gray", reference(t, "integration_icc_gray"))
	})
	t.Run("colr_cielab", func(t *testing.T) {
		checkOutput(t, "integration_colr_cielab", reference(t, "integration_colr_cielab").withTolerance(2))
	})
	t.Run("colr_cielab_alpha", func(t *testing.T) {
		checkOutput(t, "integration_colr_cielab_alpha", reference(t, "integration_colr_cielab_alpha").withTolerance(2))
	})
	t.Run("colr_cielab_16bit", func(t *testing.T) {
		checkOutput(t, "integration_colr_cielab_16bit", reference(t, "integration_colr_cielab_16bit").withTolerance(2))
	})
	t.Run("icc_invalid", func(t *testing.T) {
		checkOutput(t, "integration_icc_invalid", reference(t, "main_rgb"))
	})
}
