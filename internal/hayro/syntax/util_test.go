// Copyright (c) The Hayro Authors
//
// Use of this source code is governed by the Apache-2.0 or MIT license that
// can be found in the LICENSE-APACHE and LICENSE-MIT files.

// This file ports the tests of hayro-syntax src/util.rs.

package syntax

import (
	"strconv"
	"testing"
)

func TestSegmentList(t *testing.T) {
	e := newSegmentList[string](10)
	for i := range 500 {
		e.getOrInit(i, func() string { return strconv.Itoa(i) })
	}
	for i := range 500 {
		v, ok := e.get(i)
		if !ok || *v != strconv.Itoa(i) {
			t.Fatalf("item %d: got %v, %v", i, v, ok)
		}
	}
	for s := range 8 {
		segment := e.segments[s].Load()
		if segment == nil {
			t.Fatalf("segment %d is not set", s)
		}
		for k := range *segment {
			if _, ok := (*segment)[k].get(); !ok {
				t.Fatalf("segment %d item %d is not set", s, k)
			}
		}
	}
}

func TestFindrNeedle(t *testing.T) {
	tests := []struct {
		haystack, needle string
		want             int
		ok               bool
	}{
		{"abc foo def foo xyz", "foo", 12, true},
		{"abc", "", 3, true},
		{"abc", "abcd", 0, false},
		{"abc", "z", 0, false},
	}
	for _, test := range tests {
		got, ok := findrNeedle([]byte(test.haystack), []byte(test.needle))
		if ok != test.ok || (ok && got != test.want) {
			t.Errorf("findrNeedle(%q, %q) = %d, %v; want %d, %v", test.haystack, test.needle, got, ok, test.want, test.ok)
		}
	}
}
