// Copyright (c) The Hayro Authors
//
// Use of this source code is governed by the Apache-2.0 or MIT license that
// can be found in the LICENSE-APACHE and LICENSE-MIT files.

package jpeg2000

// This file ports hayro-jpeg2000 tests/wavelet.rs.

import "testing"

func TestWavelet(t *testing.T) {
	t.Run("reduced_resolution_1", func(t *testing.T) {
		s := DefaultDecodeSettings()
		s.TargetResolution = &Resolution{Width: 64, Height: 48}
		checkWith(t, "codestream_gray", s, reference(t, "wavelet_reduced_resolution_1"))
	})
	t.Run("reduced_resolution_2", func(t *testing.T) {
		s := DefaultDecodeSettings()
		s.TargetResolution = &Resolution{Width: 32, Height: 24}
		checkWith(t, "codestream_gray", s, reference(t, "wavelet_reduced_resolution_2"))
	})
	t.Run("reduced_resolution_3", func(t *testing.T) {
		s := DefaultDecodeSettings()
		s.TargetResolution = &Resolution{Width: 16, Height: 12}
		checkWith(t, "codestream_gray", s, reference(t, "wavelet_reduced_resolution_3"))
	})
	t.Run("reduced_resolution_4", func(t *testing.T) {
		s := DefaultDecodeSettings()
		s.TargetResolution = &Resolution{Width: 8, Height: 6}
		checkWith(t, "codestream_gray", s, reference(t, "wavelet_reduced_resolution_4"))
	})
	t.Run("reduced_lowest", func(t *testing.T) {
		s := DefaultDecodeSettings()
		s.TargetResolution = &Resolution{Width: 4, Height: 3}
		checkWith(t, "codestream_gray", s, reference(t, "wavelet_reduced_lowest"))
	})
	t.Run("reduced_subsampled", func(t *testing.T) {
		s := DefaultDecodeSettings()
		s.TargetResolution = &Resolution{Width: 12, Height: 16}
		checkWith(t, "codestream_subsampling_with_offset", s, reference(t, "wavelet_reduced_subsampled"))
	})
	t.Run("reduced_offset", func(t *testing.T) {
		s := DefaultDecodeSettings()
		s.TargetResolution = &Resolution{Width: 32, Height: 24}
		checkWith(t, "codestream_offset_large", s, reference(t, "wavelet_reduced_offset"))
	})
	t.Run("reduced_precinct_limited", func(t *testing.T) {
		s := DefaultDecodeSettings()
		s.TargetResolution = &Resolution{Width: 4, Height: 6}
		checkWith(t, "wavelet_reduced_precinct_limited", s, reference(t, "wavelet_reduced_precinct_limited"))
	})
	t.Run("reduced_target_larger_than_image", func(t *testing.T) {
		s := DefaultDecodeSettings()
		s.TargetResolution = &Resolution{Width: 1000, Height: 1000}
		checkWith(t, "codestream_gray", s, reference(t, "main_r"))
	})
	t.Run("reduced_palette", func(t *testing.T) {
		s := DefaultDecodeSettings()
		s.TargetResolution = &Resolution{Width: 1, Height: 1}
		checkWith(t, "palette_small", s, reference(t, "palette_small"))
	})
	t.Run("reduced_per_component_levels", func(t *testing.T) {
		s := DefaultDecodeSettings()
		s.TargetResolution = &Resolution{Width: 64, Height: 48}
		checkWith(t, "codestream_levels_per_component", s, reference(t, "wavelet_reduced_per_component_levels"))
	})
}
