// Copyright (c) The Hayro Authors
//
// Use of this source code is governed by the Apache-2.0 or MIT license that
// can be found in the LICENSE-APACHE and LICENSE-MIT files.

// This file ports the tests of hayro-syntax src/object/ref.rs.

package syntax

import (
	"math"
	"testing"
)

func TestObjRefDisplay(t *testing.T) {
	if got := NewObjRef(34, 1).String(); got != "34 1 R" {
		t.Errorf("got %q", got)
	}
	if got := (MaybeRef[Number]{Value: NumberFromI32(12)}).String(); got != "12" {
		t.Errorf("got %q", got)
	}
}

func TestObjRef(t *testing.T) {
	tests := []struct {
		data string
		want ObjRef
	}{
		{"34 1 R", NewObjRef(34, 1)},
		{"256 0 R (hi)", NewObjRef(256, 0)},
		{"2147483648 0 R", NewObjRef(2_147_483_648, 0)},
		{"4294967295 0 R", NewObjRef(math.MaxUint32, 0)},
	}
	for _, test := range tests {
		got, ok := NewReader([]byte(test.data)).ReadWithoutContext[ObjRef]()
		if !ok || got != test.want {
			t.Errorf("%q: got %v, %v", test.data, got, ok)
		}
	}
	for _, input := range []string{"256 R", "256 257", "4294967296 0 R", "-1 0 R"} {
		if _, ok := NewReader([]byte(input)).ReadWithoutContext[ObjRef](); ok {
			t.Errorf("%q: accepted", input)
		}
	}
}
