// Copyright (c) The Hayro Authors
//
// Use of this source code is governed by the Apache-2.0 or MIT license that
// can be found in the LICENSE-APACHE and LICENSE-MIT files.

// This file ports the tests of hayro-syntax src/object/array.rs.

package syntax

import (
	"slices"
	"testing"
)

// arrayImpl is array_impl of the tests of upstream.
func arrayImpl(data string) ([]Object, bool) {
	a, ok := NewReader([]byte(data)).ReadWithContext[Array](newReaderContext(nil, false))
	if !ok {
		return nil, false
	}
	return a.Iter[Object]().Collect(), true
}

// arrayRefImpl is array_ref_impl of the tests of upstream.
func arrayRefImpl(data string) ([]MaybeRef[Object], bool) {
	a, ok := NewReader([]byte(data)).ReadWithContext[Array](newReaderContext(nil, false))
	if !ok {
		return nil, false
	}
	return slices.Collect(a.RawIter().All()), true
}

// kinds returns the kind of each object, as the type name.
func kinds(objects []Object) []string {
	out := make([]string, len(objects))
	for i, o := range objects {
		switch o.(type) {
		case Null:
			out[i] = "Null"
		case Bool:
			out[i] = "Bool"
		case Number:
			out[i] = "Number"
		case String:
			out[i] = "String"
		case Name:
			out[i] = "Name"
		case Dict:
			out[i] = "Dict"
		case Array:
			out[i] = "Array"
		case Stream:
			out[i] = "Stream"
		}
	}
	return out
}

func TestArray(t *testing.T) {
	tests := []struct {
		data string
		want []string
	}{
		{"[]", []string{}},
		{"[   \n]", []string{}},
		{"[34]", []string{"Number"}},
		{"[true  ]", []string{"Bool"}},
		{"[true \n false 34.564]", []string{"Bool", "Bool", "Number"}},
		{"[(A string.) << /Hi 34.35 >>]", []string{"String", "Dict"}},
		{"[[32]  345.6]", []string{"Array", "Number"}},
		{"[true % A comment \n false]", []string{"Bool", "Bool"}},
		{"[(Hi) /Test]trialing data", []string{"String", "Name"}},
	}
	for _, test := range tests {
		got, ok := arrayImpl(test.data)
		if !ok || !slices.Equal(kinds(got), test.want) {
			t.Errorf("%q: got %v, %v; want %v", test.data, kinds(got), ok, test.want)
		}
	}
}

func TestArrayWithRef(t *testing.T) {
	res, ok := arrayRefImpl("[345 34 5 R 34.0]")
	if !ok || len(res) != 3 {
		t.Fatalf("got %v", res)
	}
	if _, isNum := res[0].Value.(Number); res[0].IsRef || !isNum {
		t.Error("item 0")
	}
	if !res[1].IsRef || res[1].Ref != NewObjRef(34, 5) {
		t.Error("item 1")
	}
	if _, isNum := res[2].Value.(Number); res[2].IsRef || !isNum {
		t.Error("item 2")
	}
}

func TestMalformedRefIsNotValidArray(t *testing.T) {
	if _, ok := arrayRefImpl("[8 0R]"); ok {
		t.Fatal("accepted")
	}
}

func TestArrayWithSingleNumberIsValid(t *testing.T) {
	if _, ok := arrayRefImpl("[345345345]"); !ok {
		t.Fatal("refused")
	}
}

func TestArrayRepr(t *testing.T) {
	a, _ := FromBytes[Array]([]byte("[34]"))
	if got := a.GoString(); got != "[Number(Number(Integer(34)))]" {
		t.Errorf("got %q", got)
	}
}

func TestArrayDisplay(t *testing.T) {
	a, _ := FromBytes[Array]([]byte("[  34 % comment\n /Test  [1   2] ]"))
	if got := a.String(); got != "[34 /Test [1 2]]" {
		t.Errorf("got %q", got)
	}
}

func TestArrayWithBareNumberSignName(t *testing.T) {
	res, ok := arrayImpl("[34 /quotedbl /# 37 /percent]")
	if !ok || len(res) != 5 {
		t.Fatalf("got %v", res)
	}
	if n, ok := res[2].(Name); !ok || n != "#" {
		t.Errorf("item 2: %v", res[2])
	}
}

func TestArrayWithLonePlus(t *testing.T) {
	a, _ := FromBytes[Array]([]byte("[+ 1]"))
	if got := a.Iter[int32]().Collect(); !slices.Equal(got, []int32{0, 1}) {
		t.Errorf("got %v", got)
	}
}

func TestArrayWithUnreadableItem(_ *testing.T) {
	_, _ = arrayRefImpl("[true <<>>stream false]")
}
