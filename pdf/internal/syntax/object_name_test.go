// Copyright (c) The Hayro Authors
//
// Use of this source code is governed by the Apache-2.0 or MIT license that
// can be found in the LICENSE-APACHE and LICENSE-MIT files.

// This file ports the tests of hayro-syntax src/object/name.rs.

package syntax

import "testing"

func TestNameDisplay(t *testing.T) {
	name, ok := NewReader([]byte("/Test")).ReadWithoutContext[Name]()
	if !ok || name.String() != "/Test" {
		t.Fatalf("got %q", name.String())
	}
}

func TestName(t *testing.T) {
	tests := []struct{ data, want string }{
		{"/", ""},
		{"/AB#FG", "AB#FG"},
		{"/Name1", "Name1"},
		{"/ASomewhatLongerName", "ASomewhatLongerName"},
		{"/A;Name_With-Various***Characters?", "A;Name_With-Various***Characters?"},
		{"/1.2", "1.2"},
		{"/$$", "$$"},
		{"/@pattern", "@pattern"},
		{"/.notdef", ".notdef"},
		{"/lime#20Green", "lime Green"},
		{"/paired#28#29parentheses", "paired()parentheses"},
		{"/The_Key_of_F#23_Minor", "The_Key_of_F#_Minor"},
		{"/A#42", "AB"},
		{"/A#3b", "A;"},
		{"/A#3B", "A;"},
		{"/k1  ", "k1"},
		{"/#", "#"},
		{"/A#4", "A#4"},
		{"/#2#23", "#2#"},
	}
	for _, test := range tests {
		got, ok := NewReader([]byte(test.data)).ReadWithoutContext[Name]()
		if !ok || string(got) != test.want {
			t.Errorf("%q: got %q, %v; want %q", test.data, got, ok, test.want)
		}
	}
	if _, ok := NewReader([]byte("dfg")).ReadWithoutContext[Name](); ok {
		t.Error("a name without a solidus was read")
	}
}
