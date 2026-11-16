// Copyright (c) The Hayro Authors.
//
// Use of this source code is governed by the Apache-2.0 or MIT license that
// can be found in the LICENSE-APACHE and LICENSE-MIT files.

// This file ports the tests of hayro-interpret src/shading.rs.

package interpret

import "testing"

func TestLatticeRejectsRowsOfFewerThanTwoVertices(t *testing.T) {
	decode := []float32{0, 100, 0, 100, 0, 1}
	if _, _, ok := readLatticeTriangles(nil, 8, 8, false, 0, decode); ok {
		t.Error("0 vertices for each row succeeds")
	}
	if _, _, ok := readLatticeTriangles(make([]byte, 3), 8, 8, false, 1, decode); ok {
		t.Error("1 vertex for each row succeeds")
	}
}
