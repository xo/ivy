// Copyright (c) The Hayro Authors.
//
// Use of this source code is governed by the Apache-2.0 or MIT license that
// can be found in the LICENSE-APACHE and LICENSE-MIT files.

// This file ports the tests of hayro-postscript src/lib.rs.

package postscript

import (
	"bytes"
	"testing"
)

// wantName reads a Name from s and compares it with want.
func wantName(t *testing.T, s *Scanner, want Name) {
	t.Helper()
	got, err := s.ParseName()
	if err != nil {
		t.Fatalf("ParseName: %v, want %q", err, want.data)
	}
	if !got.Equal(want) {
		t.Fatalf("ParseName = %q (kind %d), want %q (kind %d)", got.data, got.kind, want.data, want.kind)
	}
}

// wantNumber reads a Number from s and compares it with want.
func wantNumber(t *testing.T, s *Scanner, want Number) {
	t.Helper()
	got, err := s.ParseNumber()
	if err != nil {
		t.Fatalf("ParseNumber: %v, want %+v", err, want)
	}
	if got != want {
		t.Fatalf("ParseNumber = %+v, want %+v", got, want)
	}
}

// wantString reads a String from s and compares it with want.
func wantString(t *testing.T, s *Scanner, want String) {
	t.Helper()
	got, err := s.ParseString()
	if err != nil {
		t.Fatalf("ParseString: %v, want %q", err, want.data)
	}
	if !got.Equal(want) {
		t.Fatalf("ParseString = %q, want %q", got.data, want.data)
	}
}

// wantAtEnd fails the test if s holds more tokens.
func wantAtEnd(t *testing.T, s *Scanner) {
	t.Helper()
	if !s.AtEnd() {
		t.Fatalf("AtEnd = false at offset %d", s.Offset())
	}
}

func TestCMapSnippet(t *testing.T) {
	input := []byte(`/CIDInit /ProcSet findresource begin
12 dict begin
begincmap
/CMapName /Test-H def
1 begincodespacerange
<00> <FF>
endcodespacerange
2 beginbfchar
<03> <0041>
<04> <0042>
endbfchar
endcmap`)

	s := NewScanner(input)
	wantName(t, s, newName([]byte("CIDInit"), true))
	wantName(t, s, newName([]byte("ProcSet"), true))
	wantName(t, s, newName([]byte("findresource"), false))
	wantName(t, s, newName([]byte("begin"), false))
	wantNumber(t, s, NewInteger(12))
	wantName(t, s, newName([]byte("dict"), false))
	wantName(t, s, newName([]byte("begin"), false))
	wantName(t, s, newName([]byte("begincmap"), false))
	wantName(t, s, newName([]byte("CMapName"), true))
	wantName(t, s, newName([]byte("Test-H"), true))
	wantName(t, s, newName([]byte("def"), false))
	wantNumber(t, s, NewInteger(1))
	wantName(t, s, newName([]byte("begincodespacerange"), false))
	wantString(t, s, stringFromHex([]byte("00")))
	wantString(t, s, stringFromHex([]byte("FF")))
	wantName(t, s, newName([]byte("endcodespacerange"), false))
	wantNumber(t, s, NewInteger(2))
	wantName(t, s, newName([]byte("beginbfchar"), false))
	wantString(t, s, stringFromHex([]byte("03")))
	wantString(t, s, stringFromHex([]byte("0041")))
	wantString(t, s, stringFromHex([]byte("04")))
	wantString(t, s, stringFromHex([]byte("0042")))
	wantName(t, s, newName([]byte("endbfchar"), false))
	wantName(t, s, newName([]byte("endcmap"), false))
	wantAtEnd(t, s)
}

func TestArrayTokens(t *testing.T) {
	s := NewScanner([]byte("[123 /abc (xyz)]"))
	wantName(t, s, newName([]byte("["), false))
	wantNumber(t, s, NewInteger(123))
	wantName(t, s, newName([]byte("abc"), true))
	wantString(t, s, stringFromLiteral([]byte("xyz")))
	wantName(t, s, newName([]byte("]"), false))
	wantAtEnd(t, s)
}

func TestProcedureRoundTrip(t *testing.T) {
	s := NewScanner([]byte("{dup 0 lt {neg} if}"))
	proc, err := s.ParseProcedure()
	if err != nil {
		t.Fatal(err)
	}
	wantAtEnd(t, s)

	inner := proc.Tokens()
	wantName(t, inner, newName([]byte("dup"), false))
	wantNumber(t, inner, NewInteger(0))
	wantName(t, inner, newName([]byte("lt"), false))
	nested, err := inner.ParseProcedure()
	if err != nil {
		t.Fatal(err)
	}
	wantName(t, inner, newName([]byte("if"), false))
	wantAtEnd(t, inner)

	ns := nested.Tokens()
	wantName(t, ns, newName([]byte("neg"), false))
	wantAtEnd(t, ns)
}

func TestImmediateNameInProcedure(t *testing.T) {
	s := NewScanner([]byte("{//mydict begin}"))
	proc, err := s.ParseProcedure()
	if err != nil {
		t.Fatal(err)
	}
	inner := proc.Tokens()

	name, err := inner.ParseName()
	if err != nil {
		t.Fatal(err)
	}
	if !name.IsImmediate() || name.IsLiteral() || !bytes.Equal(name.AsBytes(), []byte("mydict")) {
		t.Fatalf("name = %q (kind %d), want the immediate name mydict", name.data, name.kind)
	}
	wantName(t, inner, newName([]byte("begin"), false))
	wantAtEnd(t, inner)
}

func TestDictTokens(t *testing.T) {
	s := NewScanner([]byte("<</Registry (Adobe)>>"))
	wantName(t, s, newName([]byte("<<"), false))
	wantName(t, s, newName([]byte("Registry"), true))
	wantString(t, s, stringFromLiteral([]byte("Adobe")))
	wantName(t, s, newName([]byte(">>"), false))
	wantAtEnd(t, s)
}

func TestOffsetAfterToken(t *testing.T) {
	input := []byte("/Private 4 dict eexec \x01\x02")
	s := NewScanner(input)
	if s.Offset() != 0 {
		t.Fatalf("Offset = %d, want 0", s.Offset())
	}

	wantName(t, s, newName([]byte("Private"), true))
	if s.Offset() != 8 {
		t.Fatalf("Offset = %d, want 8", s.Offset())
	}
	wantNumber(t, s, NewInteger(4))
	wantName(t, s, newName([]byte("dict"), false))
	wantName(t, s, newName([]byte("eexec"), false))
	if s.Offset() != len(input)-3 {
		t.Fatalf("Offset = %d, want %d", s.Offset(), len(input)-3)
	}
}

func TestSetOffsetSkipsBinaryData(t *testing.T) {
	input := []byte("dup 0 4 RD {(<% NP")
	s := NewScanner(input)

	wantName(t, s, newName([]byte("dup"), false))
	wantNumber(t, s, NewInteger(0))
	wantNumber(t, s, NewInteger(4))
	wantName(t, s, newName([]byte("RD"), false))

	start := s.Offset() + 1
	if got := string(input[start : start+4]); got != "{(<%" {
		t.Fatalf("binary data = %q, want %q", got, "{(<%")
	}

	s.SetOffset(start + 4)
	wantName(t, s, newName([]byte("NP"), false))
	wantAtEnd(t, s)
}

func TestSetOffsetPastEnd(t *testing.T) {
	s := NewScanner([]byte("1 2"))
	s.SetOffset(10)
	wantAtEnd(t, s)
	if _, err := s.ParseToken(); err == nil {
		t.Fatal("ParseToken past the end returned no error")
	}
}

func TestCommentsSkipped(t *testing.T) {
	s := NewScanner([]byte("% comment\n42 % another\n/Name"))
	wantNumber(t, s, NewInteger(42))
	wantName(t, s, newName([]byte("Name"), true))
	wantAtEnd(t, s)
}

func TestWrongTypeIsError(t *testing.T) {
	s := NewScanner([]byte("42 "))
	if _, err := s.ParseName(); err == nil {
		t.Fatal("ParseName of a number returned no error")
	}
}

// TestSetOffsetNegative tests the negative offset that only Go allows.
func TestSetOffsetNegative(t *testing.T) {
	s := NewScanner([]byte("1 2"))
	s.SetOffset(-5)
	wantAtEnd(t, s)
	if _, err := s.ParseToken(); err == nil {
		t.Fatal("ParseToken at a negative offset returned no error")
	}
}
