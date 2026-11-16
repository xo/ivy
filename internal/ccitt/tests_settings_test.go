// Copyright (c) The Hayro Authors.
//
// Use of this source code is governed by the Apache-2.0 or MIT license that
// can be found in the LICENSE-APACHE and LICENSE-MIT files.

// This file ports hayro-ccitt tests/settings.rs.

package ccitt

import "testing"

// TestSettings runs the tests of hayro-ccitt tests/settings.rs. Each subtest has the
// name of a test of upstream.
func TestSettings(t *testing.T) {
	t.Run("rows_fewer_than_data", func(t *testing.T) {
		checkConsumed(t, "g4", "base", 127, func(s *DecodeSettings) { s.Rows = 48 })
	})
	t.Run("eofb_before_rows", func(t *testing.T) {
		check(t, "g4_eofb", "base", func(s *DecodeSettings) {
			s.EndOfBlock = true
			s.Rows = 200
		})
	})
	t.Run("rows_unknown_rtc", func(t *testing.T) {
		check(t, "g3_1d_rtc", "base", func(s *DecodeSettings) {
			s.Encoding = Group3_1D
			s.EndOfLine = true
			s.EndOfBlock = true
			s.Rows = 0
		})
	})
	t.Run("end_of_block_without_eofb", func(t *testing.T) {
		check(t, "g4", "base", func(s *DecodeSettings) { s.EndOfBlock = true })
	})
	t.Run("end_of_block_without_rtc", func(t *testing.T) {
		check(t, "g3_1d_eol", "base", func(s *DecodeSettings) {
			s.Encoding = Group3_1D
			s.EndOfLine = true
			s.EndOfBlock = true
		})
	})
	t.Run("rows_unknown_rtc_2d", func(t *testing.T) {
		check(t, "g3_2d_rtc", "base", func(s *DecodeSettings) {
			s.Encoding = Group3_2D
			s.EndOfLine = true
			s.EndOfBlock = true
			s.Rows = 0
		})
	})
}
