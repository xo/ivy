// Copyright (c) The Hayro Authors.
//
// Use of this source code is governed by the Apache-2.0 or MIT license that
// can be found in the LICENSE-APACHE and LICENSE-MIT files.

// This file ports hayro-ccitt tests/g3_2d.rs.

package ccitt

import "testing"

// TestG32D runs the tests of hayro-ccitt tests/g3_2d.rs. Each subtest has the
// name of a test of upstream.
func TestG32D(t *testing.T) {
	t.Run("plain", func(t *testing.T) {
		check(t, "g3_2d", "base", func(s *DecodeSettings) { s.Encoding = Group3_2D })
	})
	t.Run("eol", func(t *testing.T) {
		check(t, "g3_2d_eol", "base", func(s *DecodeSettings) {
			s.Encoding = Group3_2D
			s.EndOfLine = true
		})
	})
	t.Run("eol_fill", func(t *testing.T) {
		check(t, "g3_2d_eol_fill", "base", func(s *DecodeSettings) {
			s.Encoding = Group3_2D
			s.EndOfLine = true
		})
	})
	t.Run("rtc", func(t *testing.T) {
		check(t, "g3_2d_rtc", "base", func(s *DecodeSettings) {
			s.Encoding = Group3_2D
			s.EndOfLine = true
			s.EndOfBlock = true
		})
	})
	t.Run("byte_align", func(t *testing.T) {
		check(t, "g3_2d_byte_align", "base", func(s *DecodeSettings) {
			s.Encoding = Group3_2D
			s.RowsAreByteAligned = true
		})
	})
	t.Run("byte_align_eol", func(t *testing.T) {
		check(t, "g3_2d_byte_align_eol", "base", func(s *DecodeSettings) {
			s.Encoding = Group3_2D
			s.EndOfLine = true
			s.RowsAreByteAligned = true
		})
	})
	t.Run("byte_align_eol_fill", func(t *testing.T) {
		check(t, "g3_2d_byte_align_eol_fill", "base", func(s *DecodeSettings) {
			s.Encoding = Group3_2D
			s.EndOfLine = true
			s.RowsAreByteAligned = true
		})
	})
	t.Run("all_1d", func(t *testing.T) {
		check(t, "g3_2d_all_1d", "base", func(s *DecodeSettings) { s.Encoding = Group3_2D })
	})
	t.Run("irregular_tags", func(t *testing.T) {
		check(t, "g3_2d_irregular_tags", "base", func(s *DecodeSettings) { s.Encoding = Group3_2D })
	})
}
