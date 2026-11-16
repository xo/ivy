// Copyright (c) The Hayro Authors
//
// Use of this source code is governed by the Apache-2.0 or MIT license that
// can be found in the LICENSE-APACHE and LICENSE-MIT files.

package jbig2

// This file ports the tests of hayro-jbig2 src/memory.rs.

import "testing"

// TestSegmentMemoryIsReleased is the test segment_memory_is_released.
func TestSegmentMemoryIsReleased(t *testing.T) {
	budget := newMemoryBudget(100)
	if err := budget.charge(60); err != nil {
		t.Fatal(err)
	}
	if err := budget.charge(60); err == nil {
		t.Fatal("charge(60) = nil, want an error")
	}
	budget.endSegment()
	if err := budget.charge(60); err != nil {
		t.Fatal(err)
	}
}

// TestRetainedMemoryIsKept is the test retained_memory_is_kept.
func TestRetainedMemoryIsKept(t *testing.T) {
	budget := newMemoryBudget(100)
	if err := budget.charge(60); err != nil {
		t.Fatal(err)
	}
	budget.retain(60)
	budget.endSegment()
	if err := budget.charge(60); err == nil {
		t.Fatal("charge(60) = nil, want an error")
	}
	if err := budget.charge(40); err != nil {
		t.Fatal(err)
	}
}

// TestReleasedMemoryCanBeChargedAgain is the test
// released_memory_can_be_charged_again.
func TestReleasedMemoryCanBeChargedAgain(t *testing.T) {
	budget := newMemoryBudget(100)
	if err := budget.charge(60); err != nil {
		t.Fatal(err)
	}
	budget.release(60)
	if err := budget.charge(60); err != nil {
		t.Fatal(err)
	}
}
