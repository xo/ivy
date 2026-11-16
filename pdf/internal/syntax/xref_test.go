// Copyright (c) The Hayro Authors
//
// Use of this source code is governed by the Apache-2.0 or MIT license that
// can be found in the LICENSE-APACHE and LICENSE-MIT files.

// This file ports the tests of hayro-syntax src/xref.rs.

package syntax

import (
	"math"
	"strconv"
	"testing"
)

func TestCircularPrevChain(t *testing.T) {
	pdf := []byte("%PDF-1.0\n1 0 obj\n<< /Type /Catalog /Pages 2 0 R >>\nendobj\n")
	pos := strconv.Itoa(len(pdf))
	pdf = append(pdf, "xref\n0 1\n0000000000 65535 f \r\ntrailer\n<< /Size 1 /Root 1 0 R /Prev "+pos+" >>\nstartxref\n"+pos+"\n%%EOF"...)
	m := xrefMap{}
	xrefPos, ok := findLastXRefPos(pdf)
	if !ok {
		t.Fatal("no startxref")
	}
	_, _ = populateXRefImpl(pdf, xrefPos, m, defaultDecodeEnv())
}

func TestFindLastXRefUsesLastStartxref(t *testing.T) {
	got, ok := findLastXRefPos([]byte("%PDF-1.0\nstartxref\n5\n%%EOF\nstartxref\n42\n%%EOF"))
	if !ok || got != 42 {
		t.Fatalf("got %d, %v", got, ok)
	}
}

func TestXRefTableTrailerRejectsOverflowingEntrySkip(t *testing.T) {
	r := NewReader([]byte("xref\n0 999999999999999999999\ntrailer\n<<>>"))
	if _, ok := readXRefTableTrailer(r, DummyReaderContext()); ok {
		t.Fatal("accepted")
	}
}

func TestXRefStreamSubsectionRejectsOverflowingObjectNumber(t *testing.T) {
	m := xrefMap{}
	if xrefStreamSubsection(NewReader([]byte{1, 0, 1, 0}), math.MaxUint32, 2, 1, 1, 0, m) {
		t.Fatal("accepted")
	}
	if _, ok := m[NewObjectIdentifier(math.MaxUint32, 0)]; !ok {
		t.Fatal("the first entry is missing")
	}
}
