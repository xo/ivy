// Copyright (c) The Hayro Authors
//
// Use of this source code is governed by the Apache-2.0 or MIT license that
// can be found in the LICENSE-APACHE and LICENSE-MIT files.

// This file ports the tests of hayro-syntax src/object/bool.rs and
// src/object/null.rs.

package syntax

import "testing"

func TestBool(t *testing.T) {
	if v, ok := NewReader([]byte("true")).ReadWithoutContext[bool](); !ok || !v {
		t.Error("true")
	}
	if v, ok := NewReader([]byte("false")).ReadWithoutContext[bool](); !ok || v {
		t.Error("false")
	}
	if _, ok := NewReader([]byte("trueabdf")).ReadWithoutContext[bool](); ok {
		t.Error("trailing")
	}
}

func TestNull(t *testing.T) {
	if got := (Null{}).String(); got != "null" {
		t.Errorf("display: %q", got)
	}
	if _, ok := NewReader([]byte("null")).ReadWithoutContext[Null](); !ok {
		t.Error("null")
	}
	if _, ok := NewReader([]byte("nullabs")).ReadWithoutContext[Null](); ok {
		t.Error("trailing")
	}
}
