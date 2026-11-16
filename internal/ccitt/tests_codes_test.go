// Copyright (c) The Hayro Authors.
//
// Use of this source code is governed by the Apache-2.0 or MIT license that
// can be found in the LICENSE-APACHE and LICENSE-MIT files.

// This file ports hayro-ccitt tests/codes.rs.

package ccitt

import "testing"

// TestCodes runs the tests of hayro-ccitt tests/codes.rs. Each subtest has the
// name of a test of upstream.
func TestCodes(t *testing.T) {
	t.Run("g3_1d", func(t *testing.T) {
		check(t, "codes_g3_1d", "wedge", func(s *DecodeSettings) { s.Encoding = Group3_1D })
	})
	t.Run("g3_2d", func(t *testing.T) {
		check(t, "codes_g3_2d", "wedge", func(s *DecodeSettings) { s.Encoding = Group3_2D })
	})
	t.Run("g4", func(t *testing.T) {
		check(t, "codes_g4", "wedge", func(*DecodeSettings) {})
	})
}
