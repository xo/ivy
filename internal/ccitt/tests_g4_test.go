// Copyright (c) The Hayro Authors.
//
// Use of this source code is governed by the Apache-2.0 or MIT license that
// can be found in the LICENSE-APACHE and LICENSE-MIT files.

// This file ports hayro-ccitt tests/g4.rs.

package ccitt

import "testing"

// TestG4 runs the tests of hayro-ccitt tests/g4.rs. Each subtest has the
// name of a test of upstream.
func TestG4(t *testing.T) {
	t.Run("plain", func(t *testing.T) {
		check(t, "g4", "base", func(*DecodeSettings) {})
	})
	t.Run("eofb", func(t *testing.T) {
		check(t, "g4_eofb", "base", func(s *DecodeSettings) { s.EndOfBlock = true })
	})
	t.Run("eofb_pad_bits", func(t *testing.T) {
		checkConsumed(t, "g4_eofb_pad_bits", "base", 1086, func(s *DecodeSettings) { s.EndOfBlock = true })
	})
	t.Run("eofb_trailing_data", func(t *testing.T) {
		checkConsumed(t, "g4_eofb_trailing_data", "base", 1086, func(s *DecodeSettings) { s.EndOfBlock = true })
	})
	t.Run("byte_align", func(t *testing.T) {
		check(t, "g4_byte_align", "base", func(s *DecodeSettings) { s.RowsAreByteAligned = true })
	})
	t.Run("byte_align_eofb", func(t *testing.T) {
		check(t, "g4_byte_align_eofb", "base", func(s *DecodeSettings) {
			s.RowsAreByteAligned = true
			s.EndOfBlock = true
		})
	})
	t.Run("eol", func(t *testing.T) {
		check(t, "g4_eol", "base", func(s *DecodeSettings) {
			s.EndOfLine = true
			s.EndOfBlock = true
		})
	})
	t.Run("truncated", func(t *testing.T) {
		checkMalformed(t, "malformed_g4_truncated", 8, 2, func(s *DecodeSettings) { s.Encoding = Group4 })
	})
}
