// Copyright (c) The Hayro Authors
//
// Use of this source code is governed by the Apache-2.0 or MIT license that
// can be found in the LICENSE-APACHE and LICENSE-MIT files.

package jbig2

// This file ports hayro-jbig2 tests/file.rs.

import "testing"

// TestFile runs the tests of hayro-jbig2 tests/file.rs. Each subtest has the
// name of a test of upstream.
func TestFile(t *testing.T) {
	t.Run("random_access", func(t *testing.T) {
		check(t, "file_random_access", "truchet")
	})
	t.Run("page_association_32bit", func(t *testing.T) {
		check(t, "file_page_association_32bit", "truchet")
	})
	t.Run("pages", func(t *testing.T) {
		checkPages(t, "file_pages", []string{"honeycomb", "truchet", "maze"})
	})
}
