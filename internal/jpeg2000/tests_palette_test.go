// Copyright (c) The Hayro Authors
//
// Use of this source code is governed by the Apache-2.0 or MIT license that
// can be found in the LICENSE-APACHE and LICENSE-MIT files.

package jpeg2000

// This file ports hayro-jpeg2000 tests/palette.rs.

import "testing"

func TestPalette(t *testing.T) {
	t.Run("basic", func(t *testing.T) {
		check(t, "palette_basic", reference(t, "palette_basic"))
	})
	t.Run("tiled", func(t *testing.T) {
		check(t, "palette_tiled", reference(t, "palette_basic"))
	})
	t.Run("small", func(t *testing.T) {
		check(t, "palette_small", reference(t, "palette_small"))
	})
	t.Run("four_bit_entries", func(t *testing.T) {
		check(t, "palette_four_bit_entries", reference(t, "palette_four_bit_entries"))
	})
	t.Run("out_of_range_indices", func(t *testing.T) {
		check(t, "palette_out_of_range_indices", reference(t, "palette_out_of_range_indices"))
	})
	t.Run("cdef", func(t *testing.T) {
		check(t, "palette_cdef", reference(t, "palette_basic"))
	})
	t.Run("cmap_outside_jp2h", func(t *testing.T) {
		check(t, "palette_cmap_outside_jp2h", reference(t, "palette_basic"))
	})
	t.Run("unresolved", func(t *testing.T) {
		s := DefaultDecodeSettings()
		s.ResolvePaletteIndices = false
		checkWith(t, "palette_basic", s, reference(t, "palette_unresolved"))
	})
}
