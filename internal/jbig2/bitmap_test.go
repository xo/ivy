// Copyright (c) The Hayro Authors
//
// Use of this source code is governed by the Apache-2.0 or MIT license that
// can be found in the LICENSE-APACHE and LICENSE-MIT files.

package jbig2

// This file ports the tests of hayro-jbig2 src/bitmap.rs.

import "testing"

// TestCombineFarAway is the test combine_far_away.
func TestCombineFarAway(t *testing.T) {
	budget := newMemoryBudget(-1)
	page, err := newBitmap(8, 8, &budget)
	if err != nil {
		t.Fatal(err)
	}
	region, err := newBitmapWith(8, 8, 0, 0, true, &budget)
	if err != nil {
		t.Fatal(err)
	}
	if err := page.combine(region, maxInt32-1, 0, combinationOr, cancel{}); err != nil {
		t.Fatal(err)
	}
	if err := page.combine(region, 0, maxInt32-1, combinationOr, cancel{}); err != nil {
		t.Fatal(err)
	}
	for i, w := range page.data {
		if w != 0 {
			t.Fatalf("page.data[%d] = %#x, want 0", i, w)
		}
	}
}
