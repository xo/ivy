// Copyright (c) The Hayro Authors
//
// Use of this source code is governed by the Apache-2.0 or MIT license that
// can be found in the LICENSE-APACHE and LICENSE-MIT files.

package jpeg2000

// This file ports hayro-jpeg2000 tests/cancel.rs.

import "testing"

func TestCancel(t *testing.T) {
	t.Run("cancelled", func(t *testing.T) {
		s := DefaultDecodeSettings()
		s.Cancel = NewCancelHandle(CancelFunc(func() bool { return true }))
		checkError(t, "code_blocks_all_styles", s, "decoding was cancelled")
	})
}
