// Copyright (c) The Hayro Authors
//
// Use of this source code is governed by the Apache-2.0 or MIT license that
// can be found in the LICENSE-APACHE and LICENSE-MIT files.

package jbig2

// This file ports the tests of hayro-jbig2 src/segment.rs.

import (
	"slices"
	"testing"
)

// TestSegmentHeaderExample1 is the test test_segment_header_example_1.
func TestSegmentHeaderExample1(t *testing.T) {
	data := []byte{
		0x00, 0x00, 0x00, 0x20, 0x86, 0x6B, 0x02, 0x1E, 0x05, 0x04, 0x00, 0x00, 0x00, 0x10,
	}

	r := newReader(data)
	h, err := parseSegmentHeader(&r, true)
	if err != nil {
		t.Fatal(err)
	}

	if h.segmentNumber != 32 {
		t.Errorf("segmentNumber = %d, want 32", h.segmentNumber)
	}
	if h.segmentType != segmentImmediateTextRegion {
		t.Errorf("segmentType = %d, want %d", h.segmentType, segmentImmediateTextRegion)
	}
	if h.retainFlag {
		t.Error("retainFlag = true, want false")
	}
	if want := []uint32{2, 30, 5}; !slices.Equal(h.referredToSegments, want) {
		t.Errorf("referredToSegments = %v, want %v", h.referredToSegments, want)
	}
	if h.pageAssociation != 4 {
		t.Errorf("pageAssociation = %d, want 4", h.pageAssociation)
	}
	if !h.hasDataLength || h.dataLength != 16 {
		t.Errorf("dataLength = %d, %t, want 16, true", h.dataLength, h.hasDataLength)
	}
}

// TestSegmentHeaderExample2 is the test test_segment_header_example_2.
func TestSegmentHeaderExample2(t *testing.T) {
	data := []byte{
		0x00, 0x00, 0x02, 0x34,
		0x40,
		0xE0, 0x00, 0x00, 0x09,
		0x02, 0xFD,
		0x01, 0x00,
		0x00, 0x02,
		0x00, 0x1E,
		0x00, 0x05,
		0x02, 0x00,
		0x02, 0x01,
		0x02, 0x02,
		0x02, 0x03,
		0x02, 0x04,
		0x00, 0x00, 0x04, 0x01,
		0x00, 0x00, 0x00, 0x20,
	}

	r := newReader(data)
	h, err := parseSegmentHeader(&r, true)
	if err != nil {
		t.Fatal(err)
	}

	if h.segmentNumber != 564 {
		t.Errorf("segmentNumber = %d, want 564", h.segmentNumber)
	}
	if h.segmentType != segmentSymbolDictionary {
		t.Errorf("segmentType = %d, want %d", h.segmentType, segmentSymbolDictionary)
	}
	if !h.retainFlag {
		t.Error("retainFlag = false, want true")
	}
	if want := []uint32{256, 2, 30, 5, 512, 513, 514, 515, 516}; !slices.Equal(h.referredToSegments, want) {
		t.Errorf("referredToSegments = %v, want %v", h.referredToSegments, want)
	}
	if h.pageAssociation != 1025 {
		t.Errorf("pageAssociation = %d, want 1025", h.pageAssociation)
	}
	if !h.hasDataLength || h.dataLength != 32 {
		t.Errorf("dataLength = %d, %t, want 32, true", h.dataLength, h.hasDataLength)
	}
}
