// Copyright (c) The Hayro Authors.
//
// Use of this source code is governed by the Apache-2.0 or MIT license that
// can be found in the LICENSE-APACHE and LICENSE-MIT files.

// This file ports hayro-ccitt tests/cancel.rs.

package ccitt

import (
	"errors"
	"testing"
)

// TestCancelled is the test cancelled in hayro-ccitt tests/cancel.rs.
func TestCancelled(t *testing.T) {
	data := readStream(t, "g4")
	ref := loadReference(t, "base")
	settings := DecodeSettings{
		Columns:  uint32(len(ref[0])),
		Rows:     uint32(len(ref)),
		Encoding: Group4,
		Cancel:   CancelFunc(func() bool { return true }),
	}

	_, err := Decode(data, &collector{columns: settings.Columns}, &settings)
	if !errors.Is(err, ErrCancelled) {
		t.Fatalf("Decode() error = %v, want %v", err, ErrCancelled)
	}
}
