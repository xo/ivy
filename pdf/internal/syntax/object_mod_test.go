// Copyright (c) The Hayro Authors
//
// Use of this source code is governed by the Apache-2.0 or MIT license that
// can be found in the LICENSE-APACHE and LICENSE-MIT files.

// This file ports the tests of hayro-syntax src/object/mod.rs.

package syntax

import (
	"strings"
	"testing"
)

// objectImpl is object_impl of the tests of upstream.
func objectImpl(data string) (Object, bool) {
	return NewReader([]byte(data)).ReadIndirect[Object](DummyReaderContext())
}

func TestDeeplyNestedArrays(t *testing.T) {
	data := strings.Repeat("[", 5000) + "0" + strings.Repeat("]", 5000)
	if _, ok := objectImpl(data); ok {
		t.Fatal("expected a failure")
	}
}

func TestDeeplyNestedDictionaries(t *testing.T) {
	data := strings.Repeat("<< /X ", 5000) + "0" + strings.Repeat(" >>", 5000)
	if _, ok := objectImpl(data); ok {
		t.Fatal("expected a failure")
	}
}

func TestObjectKinds(t *testing.T) {
	tests := []struct {
		data string
		want string
	}{
		{"null", "Null"},
		{"true", "Bool"},
		{"34.5", "Number"},
		{"(Hi)", "String"},
		{"<34>", "String"},
		{"/Name", "Name"},
		{"<</Entry 45>>", "Dict"},
		{"[45]", "Array"},
		{"<< /Length 3 >> stream\nabc\nendstream", "Stream"},
	}
	for _, test := range tests {
		o, ok := objectImpl(test.data)
		if !ok {
			t.Fatalf("%q: no object", test.data)
		}
		var got string
		switch o.(type) {
		case Null:
			got = "Null"
		case Bool:
			got = "Bool"
		case Number:
			got = "Number"
		case String:
			got = "String"
		case Name:
			got = "Name"
		case Dict:
			got = "Dict"
		case Array:
			got = "Array"
		case Stream:
			got = "Stream"
		}
		if got != test.want {
			t.Errorf("%q: got %s, want %s", test.data, got, test.want)
		}
	}
}

func TestStreamInDict(t *testing.T) {
	dict, ok := FromBytes[Dict]([]byte("<< /A << /Length 3 >> stream\nabc\nendstream >>"))
	if !ok {
		t.Fatal("no dict")
	}
	if _, ok := dict.Get[Stream]("A"); ok {
		t.Fatal("a stream in a dict was read")
	}
}

func TestObjectDisplay(t *testing.T) {
	tests := []struct{ data, want string }{
		{"null", "null"},
		{"true", "true"},
		{"34.5", "34.5"},
		{"(Hi)", "\"Hi\""},
		{"/Name", "/Name"},
		{"[45]", "[45]"},
		{"<</Entry 45>>", "<< /Entry 45 >>"},
		{"<< /Length 3 >> stream\nabc\nendstream", "Stream (len: 3)"},
	}
	for _, test := range tests {
		o, ok := objectImpl(test.data)
		if !ok {
			t.Fatalf("%q: no object", test.data)
		}
		if got := o.String(); got != test.want {
			t.Errorf("%q: got %q, want %q", test.data, got, test.want)
		}
	}
}
