// Copyright (c) The Hayro Authors.
//
// Use of this source code is governed by the Apache-2.0 or MIT license that
// can be found in the LICENSE-APACHE and LICENSE-MIT files.

// This file ports hayro-ccitt tests/dimensions.rs.

package ccitt

import "testing"

// TestDimensions runs the tests of hayro-ccitt tests/dimensions.rs. Each subtest has the
// name of a test of upstream.
func TestDimensions(t *testing.T) {
	t.Run("width_1_g4", func(t *testing.T) {
		check(t, "width_1_g4", "width_1", func(*DecodeSettings) {})
	})
	t.Run("width_1_g3_1d", func(t *testing.T) {
		check(t, "width_1_g3_1d", "width_1", func(s *DecodeSettings) { s.Encoding = Group3_1D })
	})
	t.Run("width_1_g3_2d", func(t *testing.T) {
		check(t, "width_1_g3_2d", "width_1", func(s *DecodeSettings) { s.Encoding = Group3_2D })
	})
	t.Run("height_1_g4", func(t *testing.T) {
		check(t, "height_1_g4", "height_1", func(*DecodeSettings) {})
	})
	t.Run("height_1_g3_1d", func(t *testing.T) {
		check(t, "height_1_g3_1d", "height_1", func(s *DecodeSettings) { s.Encoding = Group3_1D })
	})
	t.Run("height_1_g3_2d", func(t *testing.T) {
		check(t, "height_1_g3_2d", "height_1", func(s *DecodeSettings) { s.Encoding = Group3_2D })
	})
}
