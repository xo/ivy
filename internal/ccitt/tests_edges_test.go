// Copyright (c) The Hayro Authors.
//
// Use of this source code is governed by the Apache-2.0 or MIT license that
// can be found in the LICENSE-APACHE and LICENSE-MIT files.

// This file ports hayro-ccitt tests/edges.rs.

package ccitt

import "testing"

// TestEdges runs the tests of hayro-ccitt tests/edges.rs. Each subtest has the
// name of a test of upstream.
func TestEdges(t *testing.T) {
	t.Run("g4", func(t *testing.T) {
		check(t, "edges_g4", "edges", func(*DecodeSettings) {})
	})
	t.Run("g3_1d", func(t *testing.T) {
		check(t, "edges_g3_1d", "edges", func(s *DecodeSettings) { s.Encoding = Group3_1D })
	})
	t.Run("g3_2d", func(t *testing.T) {
		check(t, "edges_g3_2d", "edges", func(s *DecodeSettings) { s.Encoding = Group3_2D })
	})
}
