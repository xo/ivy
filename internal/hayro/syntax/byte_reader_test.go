// Copyright (c) The Hayro Authors
//
// Use of this source code is governed by the Apache-2.0 or MIT license that
// can be found in the LICENSE-APACHE and LICENSE-MIT files.

// This file ports the tests of hayro-syntax src/byte_reader.rs.

package syntax

import (
	"math"
	"testing"
)

func TestPeekBytesRejectsOverflowingLen(t *testing.T) {
	r := NewReader([]byte("abc"))
	if _, ok := r.PeekBytes(math.MaxInt); ok {
		t.Fatal("expected a failure")
	}
}
