// Copyright (c) The Hayro Authors.
//
// Use of this source code is governed by the Apache-2.0 or MIT license that
// can be found in the LICENSE-APACHE and LICENSE-MIT files.

// This file ports hayro-ccitt tests/modes.rs.

package ccitt

import "testing"

// TestModes runs the tests of hayro-ccitt tests/modes.rs. Each subtest has the
// name of a test of upstream.
func TestModes(t *testing.T) {
	t.Run("pass_g4", func(t *testing.T) {
		check(t, "mode_pass_g4", "mode_pass", func(*DecodeSettings) {})
	})
	t.Run("pass_g3_2d", func(t *testing.T) {
		check(t, "mode_pass_g3_2d", "mode_pass", func(s *DecodeSettings) { s.Encoding = Group3_2D })
	})
	t.Run("vertical_g4", func(t *testing.T) {
		check(t, "mode_vertical_g4", "mode_vertical", func(*DecodeSettings) {})
	})
	t.Run("vertical_g3_2d", func(t *testing.T) {
		check(t, "mode_vertical_g3_2d", "mode_vertical", func(s *DecodeSettings) { s.Encoding = Group3_2D })
	})
	t.Run("horizontal_g4", func(t *testing.T) {
		check(t, "mode_horizontal_g4", "mode_horizontal", func(*DecodeSettings) {})
	})
	t.Run("horizontal_g3_2d", func(t *testing.T) {
		check(t, "mode_horizontal_g3_2d", "mode_horizontal", func(s *DecodeSettings) { s.Encoding = Group3_2D })
	})
}
