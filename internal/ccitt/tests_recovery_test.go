// Copyright (c) The Hayro Authors.
//
// Use of this source code is governed by the Apache-2.0 or MIT license that
// can be found in the LICENSE-APACHE and LICENSE-MIT files.

// This file ports hayro-ccitt tests/recovery.rs.

package ccitt

import "testing"

// TestRecovery runs the tests of hayro-ccitt tests/recovery.rs. Each subtest has the
// name of a test of upstream.
func TestRecovery(t *testing.T) {
	t.Run("corrupt_line_g3_1d", func(t *testing.T) {
		check(t, "g3_1d_corrupt_line", "g3_1d_corrupt_line", func(s *DecodeSettings) {
			s.Encoding = Group3_1D
			s.EndOfLine = true
		})
	})
	t.Run("truncated_line_g3_1d", func(t *testing.T) {
		check(t, "g3_1d_truncated_line", "g3_1d_truncated_line", func(s *DecodeSettings) {
			s.Encoding = Group3_1D
			s.EndOfLine = true
		})
	})
	t.Run("truncated_line_g3_2d", func(t *testing.T) {
		check(t, "g3_2d_truncated_line", "g3_2d_truncated_line", func(s *DecodeSettings) {
			s.Encoding = Group3_2D
			s.EndOfLine = true
		})
	})
	t.Run("eol_flag_without_eols", func(t *testing.T) {
		check(t, "g3_1d", "base", func(s *DecodeSettings) {
			s.Encoding = Group3_1D
			s.EndOfLine = true
		})
	})
	t.Run("run_past_line_end_g3_1d", func(t *testing.T) {
		check(t, "g3_1d_run_past_line_end", "g3_1d_run_past_line_end", func(s *DecodeSettings) { s.Encoding = Group3_1D })
	})
	t.Run("truncated_line_g4", func(t *testing.T) {
		check(t, "g4_truncated_line", "g4_truncated_line", func(*DecodeSettings) {})
	})
}
