// Copyright (c) The Hayro Authors
//
// Use of this source code is governed by the Apache-2.0 or MIT license that
// can be found in the LICENSE-APACHE and LICENSE-MIT files.

package jbig2

// This file ports hayro-jbig2 tests/cancel.rs.

import (
	"errors"
	"sync/atomic"
	"testing"
)

// TestCancelled is the test cancelled in hayro-jbig2 tests/cancel.rs.
func TestCancelled(t *testing.T) {
	data := readStream(t, "generic_template0")
	settings := DecodeSettings{Cancel: CancelFunc(func() bool { return true })}
	file, err := NewFile(data, settings)
	if err != nil {
		t.Fatal(err)
	}

	if _, err := file.Page(0).Decode(); !errors.Is(err, ErrCancelled) {
		t.Fatalf("Decode() error = %v, want %v", err, ErrCancelled)
	}
}

// TestDataU8 is the test data_u8 in hayro-jbig2 tests/cancel.rs.
func TestDataU8(t *testing.T) {
	data := readStream(t, "generic_template0")
	var cancelled atomic.Bool
	settings := DecodeSettings{Cancel: CancelFunc(cancelled.Load)}
	file, err := NewFile(data, settings)
	if err != nil {
		t.Fatal(err)
	}
	pb, err := file.Page(0).Decode()
	if err != nil {
		t.Fatal(err)
	}

	cancelled.Store(true)
	buf := make([]byte, int(pb.Width())*int(pb.Height()))

	if _, err := pb.DataU8(); !errors.Is(err, ErrCancelled) {
		t.Errorf("DataU8() error = %v, want %v", err, ErrCancelled)
	}
	if err := pb.StoreU8Into(buf); !errors.Is(err, ErrCancelled) {
		t.Errorf("StoreU8Into() error = %v, want %v", err, ErrCancelled)
	}
}
