// Copyright (c) The Hayro Authors
//
// Use of this source code is governed by the Apache-2.0 or MIT license that
// can be found in the LICENSE-APACHE and LICENSE-MIT files.

package jpeg2000

// This file ports hayro-jpeg2000 tests/roi.rs.

import "testing"

func TestRoi(t *testing.T) {
	t.Run("reversible", func(t *testing.T) {
		check(t, "roi_reversible", reference(t, "main_rgb"))
	})
	t.Run("tile_part_override", func(t *testing.T) {
		check(t, "roi_tile_part_override", reference(t, "main_rgb"))
	})
	t.Run("irreversible", func(t *testing.T) {
		check(t, "roi_irreversible", reference(t, "main_rgb").withTolerance(3))
	})
}
