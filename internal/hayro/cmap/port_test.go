// Copyright (c) The Hayro Authors.
//
// Use of this source code is governed by the Apache-2.0 or MIT license that
// can be found in the LICENSE-APACHE and LICENSE-MIT files.

// This file tests the parts of the port that upstream does not have: the
// bundle, the limits and the errors. Upstream has no such tests.

package cmap

import (
	"encoding/binary"
	"errors"
	"strings"
	"testing"

	"github.com/xo/ivy"
)

// TestEveryEmbeddedCMap parses each predefined CMap with its base CMaps.
func TestEveryEmbeddedCMap(t *testing.T) {
	b, err := loadBundle()
	if err != nil {
		t.Fatal(err)
	}
	if len(b.entries) != len(embeddedNames) {
		t.Fatalf("the bundle holds %d CMaps, want %d", len(b.entries), len(embeddedNames))
	}
	for _, name := range embeddedNames {
		c := mustParseEmbedded(t, name)
		if string(c.Metadata().Name) != string(name) {
			t.Errorf("CMap %s has the name %q", name, c.Metadata().Name)
		}
	}
	if _, ok := LoadEmbedded("Custom-Name"); ok {
		t.Error("LoadEmbedded found a custom name")
	}
}

// TestUseCMapLimits tests the depth of usecmap and the limit of the number
// of usecmap operators.
func TestUseCMapLimits(t *testing.T) {
	self := func(Name) ([]byte, bool) { return []byte("/Self usecmap"), true }
	if _, err := Parse([]byte("/Self usecmap"), self); !errors.Is(err, ivy.ErrLimit) {
		t.Errorf("a CMap that names itself: err = %v, want ivy.ErrLimit", err)
	}

	many := strings.Repeat("/H usecmap\n", maxNestedParses+1)
	if _, err := Parse([]byte(many), LoadEmbedded); !errors.Is(err, ivy.ErrLimit) {
		t.Errorf("%d usecmap operators: err = %v, want ivy.ErrLimit", maxNestedParses+1, err)
	}

	few := strings.Repeat("/H usecmap\n", 3)
	c, err := Parse([]byte(few), LoadEmbedded)
	if err != nil {
		t.Fatalf("three usecmap operators: %v", err)
	}
	wantCID(t, c, 0x2121, 2, 633)

	if _, err := Parse([]byte("/H usecmap"), nil); !errors.Is(err, ivy.ErrFormat) {
		t.Errorf("a nil resolver: err = %v, want ivy.ErrFormat", err)
	}
}

// TestBCMapSelfReference tests a binary CMap whose usecmap segment names
// itself. Upstream recurses without end.
func TestBCMapSelfReference(t *testing.T) {
	payload := []byte("Self")
	seg := make([]byte, 0, 5+len(payload))
	seg = append(seg, segmentUseCMap)
	seg = binary.BigEndian.AppendUint32(seg, uint32(segHeaderSize+len(payload)))
	seg = append(seg, payload...)

	data := append([]byte(bcmapMagic), bcmapVersion)
	data = binary.BigEndian.AppendUint32(data, uint32(bcmapFileHeaderSize+len(seg)))
	data = append(data, seg...)

	self := func(Name) ([]byte, bool) { return data, true }
	if _, err := Parse(data, self); !errors.Is(err, ivy.ErrLimit) {
		t.Errorf("err = %v, want ivy.ErrLimit", err)
	}
}

// TestMalformed tests that malformed CMaps give errors that wrap a sentinel
// error of ivy.
func TestMalformed(t *testing.T) {
	for _, input := range []string{
		"1 begincodespacerange <00> <0000> endcodespacerange",
		"1 begincidrange <00> <FF> -1 endcidrange",
		"1 begincidrange <0000000000> <FF> 1 endcidrange",
		"1 beginbfrange <00> <02> [<0041> <0042>] endbfrange",
		"1 beginbfrange <00> <02> 5 endbfrange",
		"1 beginbfchar <00> <0041>",
		"usecmap",
		"(abc",
		"bcmap\x01\x00\x00\x00\x05",
		"bcmap\x01\x00\x00\x00\x0f\xff\x00\x00\x00\x05",
		"bcmap\x02",
	} {
		_, err := Parse([]byte(input), LoadEmbedded)
		if !errors.Is(err, ivy.ErrFormat) && !errors.Is(err, ivy.ErrLimit) {
			t.Errorf("Parse(%q): err = %v, want a sentinel error of ivy", input, err)
		}
	}
}

// TestBfRangeToMaxCode tests a bfrange array that ends at the largest code,
// where a counter of 32 bits wraps.
func TestBfRangeToMaxCode(t *testing.T) {
	c := mustParse(t, "1 beginbfrange <FFFFFFFF> <FFFFFFFF> [<0041>] endbfrange", noCMap)
	wantBf(t, c, 0xffffffff, NewBfChar('A'))
}

// TestFindInRangesOverlap tests that the search finds the same entry as the
// binary search of Rust when two ranges overlap.
func TestFindInRangesOverlap(t *testing.T) {
	entries := []CIDRange{
		{rng: codeRange{start: 0, end: 100}, cidStart: 1},
		{rng: codeRange{start: 10, end: 10}, cidStart: 2},
		{rng: codeRange{start: 50, end: 60}, cidStart: 3},
	}
	// For the code 5, slice::binary_search_by tests the index 1 two times.
	// Both tests fail, and the index 0 holds the code.
	if i, ok := findInRanges(entries, 5); !ok || i != 0 {
		t.Errorf("findInRanges(5) = %d, %v, want 0", i, ok)
	}
	// For the code 10, the first test finds the index 1, and the search
	// stays there.
	if i, ok := findInRanges(entries, 10); !ok || i != 1 {
		t.Errorf("findInRanges(10) = %d, %v, want 1", i, ok)
	}
	if i, ok := findInRanges(entries, 55); !ok || i != 2 {
		t.Errorf("findInRanges(55) = %d, %v, want 2", i, ok)
	}
}
