// Copyright (c) The Hayro Authors
//
// Use of this source code is governed by the Apache-2.0 or MIT license that
// can be found in the LICENSE-APACHE and LICENSE-MIT files.

package jpeg2000

// This file ports hayro-jpeg2000 tests/quantization.rs.

import "testing"

func TestQuantization(t *testing.T) {
	t.Run("irreversible", func(t *testing.T) {
		check(t, "quantization_irreversible", reference(t, "main_r").withTolerance(2))
	})
	t.Run("irreversible_12bit", func(t *testing.T) {
		check(t, "quantization_irreversible_12bit", reference(t, "main_r_12bit").withTolerance(1))
	})
	t.Run("irreversible_16bit", func(t *testing.T) {
		check(t, "quantization_irreversible_16bit", reference(t, "stress_16bit").withTolerance(1))
	})
	t.Run("scalar_derived", func(t *testing.T) {
		check(t, "quantization_scalar_derived", reference(t, "main_r").withTolerance(1))
	})
	t.Run("irreversible_tiny_tiles", func(t *testing.T) {
		check(t, "quantization_irreversible_tiny_tiles", reference(t, "main_rgb_crop_50_40_12_12").withTolerance(2))
	})
	t.Run("lossy_layers", func(t *testing.T) {
		check(t, "quantization_lossy_layers", reference(t, "quantization_lossy_layers").withTolerance(1))
	})
	t.Run("irreversible_rate", func(t *testing.T) {
		check(t, "quantization_irreversible_rate", reference(t, "quantization_irreversible_rate").withTolerance(1))
	})
	t.Run("irreversible_tiles_offset", func(t *testing.T) {
		check(t, "quantization_irreversible_tiles_offset", reference(t, "quantization_irreversible_tiles_offset"))
	})
	t.Run("irreversible_offset_large", func(t *testing.T) {
		check(t, "quantization_irreversible_offset_large", reference(t, "quantization_irreversible_offset_large"))
	})
	t.Run("reversible_lossy_layers", func(t *testing.T) {
		check(t, "quantization_reversible_lossy_layers", reference(t, "quantization_reversible_lossy_layers"))
	})
}
