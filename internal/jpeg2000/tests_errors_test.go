// Copyright (c) The Hayro Authors
//
// Use of this source code is governed by the Apache-2.0 or MIT license that
// can be found in the LICENSE-APACHE and LICENSE-MIT files.

package jpeg2000

// This file ports hayro-jpeg2000 tests/errors.rs.

import "testing"

func TestErrors(t *testing.T) {
	t.Run("too_few_exponents", func(t *testing.T) {
		checkError(t, "errors_too_few_exponents", DefaultDecodeSettings(), "not enough exponents provided in header")
	})
	t.Run("reduced_below_tile_levels", func(t *testing.T) {
		s := DefaultDecodeSettings()
		s.TargetResolution = &Resolution{Width: 32, Height: 24}
		checkError(t, "codestream_tile_cod_fewer_levels", s, "invalid component metadata")
	})
	t.Run("jp2_invalid_codestream", func(t *testing.T) {
		checkError(t, "errors_jp2_invalid_codestream", DefaultDecodeSettings(), "failed to parse COD marker")
	})
}
