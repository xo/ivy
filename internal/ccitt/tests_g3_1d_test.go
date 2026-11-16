// Copyright (c) The Hayro Authors.
//
// Use of this source code is governed by the Apache-2.0 or MIT license that
// can be found in the LICENSE-APACHE and LICENSE-MIT files.

// This file ports hayro-ccitt tests/g3_1d.rs.

package ccitt

import "testing"

// TestG31D runs the tests of hayro-ccitt tests/g3_1d.rs. Each subtest has the
// name of a test of upstream.
func TestG31D(t *testing.T) {
	t.Run("plain", func(t *testing.T) {
		check(t, "g3_1d", "base", func(s *DecodeSettings) { s.Encoding = Group3_1D })
	})
	t.Run("eol", func(t *testing.T) {
		check(t, "g3_1d_eol", "base", func(s *DecodeSettings) {
			s.Encoding = Group3_1D
			s.EndOfLine = true
		})
	})
	t.Run("eol_fill", func(t *testing.T) {
		check(t, "g3_1d_eol_fill", "base", func(s *DecodeSettings) {
			s.Encoding = Group3_1D
			s.EndOfLine = true
		})
	})
	t.Run("long_fill", func(t *testing.T) {
		check(t, "g3_1d_long_fill", "base", func(s *DecodeSettings) {
			s.Encoding = Group3_1D
			s.EndOfLine = true
		})
	})
	t.Run("rtc", func(t *testing.T) {
		check(t, "g3_1d_rtc", "base", func(s *DecodeSettings) {
			s.Encoding = Group3_1D
			s.EndOfLine = true
			s.EndOfBlock = true
		})
	})
	t.Run("byte_align", func(t *testing.T) {
		check(t, "g3_1d_byte_align", "base", func(s *DecodeSettings) {
			s.Encoding = Group3_1D
			s.RowsAreByteAligned = true
		})
	})
	t.Run("byte_align_eol", func(t *testing.T) {
		check(t, "g3_1d_byte_align_eol", "base", func(s *DecodeSettings) {
			s.Encoding = Group3_1D
			s.EndOfLine = true
			s.RowsAreByteAligned = true
		})
	})
	t.Run("byte_align_eol_fill", func(t *testing.T) {
		check(t, "g3_1d_byte_align_eol_fill", "base", func(s *DecodeSettings) {
			s.Encoding = Group3_1D
			s.EndOfLine = true
			s.RowsAreByteAligned = true
		})
	})
	t.Run("byte_align_rtc", func(t *testing.T) {
		check(t, "g3_1d_byte_align_rtc", "base", func(s *DecodeSettings) {
			s.Encoding = Group3_1D
			s.EndOfLine = true
			s.EndOfBlock = true
			s.RowsAreByteAligned = true
		})
	})
}
