// Copyright (c) The Hayro Authors
//
// Use of this source code is governed by the Apache-2.0 or MIT license that
// can be found in the LICENSE-APACHE and LICENSE-MIT files.

// This file ports the tests of hayro-syntax src/object/dict.rs.

package syntax

import "testing"

// dictImpl is dict_impl of the tests of upstream.
func dictImpl(t *testing.T, data string) Dict {
	t.Helper()
	d, ok := NewReader([]byte(data)).ReadWithContext[Dict](DummyReaderContext())
	if !ok {
		t.Fatalf("%q: no dict", data)
	}
	return d
}

func TestEmptyDict(t *testing.T) {
	for _, data := range []string{"<<>>", "<<   \n >>"} {
		if n := dictImpl(t, data).Len(); n != 0 {
			t.Errorf("%q: len %d", data, n)
		}
	}
}

func TestDict(t *testing.T) {
	d := dictImpl(t, "<<  /Hi 34.0 >>")
	if _, ok := d.Get[Number]("Hi"); d.Len() != 1 || !ok {
		t.Error("dict 1")
	}
	d = dictImpl(t, "<<  /Hi \n 34.0 /Second true >>")
	_, ok1 := d.Get[Number]("Hi")
	_, ok2 := d.Get[bool]("Second")
	if d.Len() != 2 || !ok1 || !ok2 {
		t.Error("dict 2")
	}
}

func TestDictComplex(t *testing.T) {
	d := dictImpl(t, `<< /Type /Example
/Subtype /DictionaryExample
/Version 0.01
/IntegerItem 12
/StringItem ( a string )
/Subdictionary << /Item1 0.4
                /Item2 true
                /LastItem ( not ! )
                /VeryLastItem ( OK )
                >>
>>`)
	if d.Len() != 6 {
		t.Fatalf("len %d", d.Len())
	}
	_, ok1 := d.Get[Name]("Type")
	_, ok2 := d.Get[Name]("Subtype")
	_, ok3 := d.Get[Number]("Version")
	_, ok4 := d.Get[int32]("IntegerItem")
	_, ok5 := d.Get[String]("StringItem")
	_, ok6 := d.Get[Dict]("Subdictionary")
	if !ok1 || !ok2 || !ok3 || !ok4 || !ok5 || !ok6 {
		t.Error("an entry is missing")
	}
}

func TestDictWithTrailing(t *testing.T) {
	if n := dictImpl(t, "<<  /Hi 67.0  >>trailing data").Len(); n != 1 {
		t.Errorf("len %d", n)
	}
}

func TestDictWithComment(t *testing.T) {
	if n := dictImpl(t, "<<  /Hi % A comment \n 67.0 % Another comment \n >>").Len(); n != 1 {
		t.Errorf("len %d", n)
	}
}

func TestInlineDict(t *testing.T) {
	d, ok := NewReader([]byte("/W 17 /H 17 /CS /RGB /BPC 8 /F [ /A85 /LZW ] ID ")).ReadWithContext[InlineImageDict](DummyReaderContext())
	if !ok || d.GetDict().Len() != 5 {
		t.Fatal("inline dict")
	}
}

func TestDictWithEscapedName(t *testing.T) {
	if !dictImpl(t, "<< /PANTONE#20104#20C 234 >>").ContainsKey("PANTONE 104 C") {
		t.Fatal("key missing")
	}
}

func TestGarbageInBetween(t *testing.T) {
	d := dictImpl(t, `<< 
/ProcSet [ /PDF /Text ] 
/Font << /F4 31 0 R /F6 23 0 R >> 
/ExtGState << /GS2 14 0 R
2000
 /GS3 15 0 R >> 
/ColorSpace << /Cs5 13 0 R >> 
>> `)
	for _, key := range []Name{KeyProcSet, KeyFont, KeyExtGState, KeyColorSpace} {
		if !d.ContainsKey(key) {
			t.Errorf("%s missing", key)
		}
	}
	ext, ok := d.Get[Dict](KeyExtGState)
	if !ok {
		t.Fatal("failed to parse ext g state")
	}
	if r, ok := ext.GetRef("GS2"); !ok || r != NewObjRef(14, 0) {
		t.Errorf("GS2: %v", r)
	}
	if r, ok := ext.GetRef("GS3"); !ok || r != NewObjRef(15, 0) {
		t.Errorf("GS3: %v", r)
	}
}

func TestDictRepr(t *testing.T) {
	d, _ := FromBytes[Dict]([]byte("<< /Hi 34 >>"))
	if got := d.GoString(); got != "Dict { \"Hi\": Number(Number(Integer(34))) }" {
		t.Errorf("got %q", got)
	}
}

func TestDictDisplay(t *testing.T) {
	d, _ := FromBytes[Dict]([]byte("<< /B [1   2] % comment\n /A << /X   true >> >>"))
	if got := d.String(); got != "<< /A << /X true >> /B [1 2] >>" {
		t.Errorf("got %q", got)
	}
	if got := EmptyDict().String(); got != "<<>>" {
		t.Errorf("got %q", got)
	}
}

func TestDictWithUnreadableValue(t *testing.T) {
	d, ok := FromBytes[Dict]([]byte("<< /A <<>>stream /B true >>"))
	if !ok {
		t.Fatal("no dict")
	}
	_ = len(d.Entries())
	_ = d.GoString()
}
