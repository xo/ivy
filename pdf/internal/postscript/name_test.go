// Copyright (c) The Hayro Authors.
//
// Use of this source code is governed by the Apache-2.0 or MIT license that
// can be found in the LICENSE-APACHE and LICENSE-MIT files.

// This file ports the tests of hayro-postscript src/name.rs.

package postscript

import "testing"

// readLiteral is read_literal in the tests of hayro-postscript src/name.rs.
func readLiteral(input string) (Name, bool) {
	d, ok := parseLiteralName(newReader([]byte(input)))
	return newName(d, true), ok
}

// readExecutable is read_executable in the tests of hayro-postscript
// src/name.rs.
func readExecutable(input string) (Name, bool) {
	d, ok := parseExecutableName(newReader([]byte(input)))
	return newName(d, false), ok
}

// wantStr compares the string of n with want.
func wantStr(t *testing.T, n Name, want string) {
	t.Helper()
	got, ok := n.AsStr()
	if !ok || got != want {
		t.Fatalf("AsStr = %q, %v, want %q", got, ok, want)
	}
}

// wantPeek compares the next byte of r with want.
func wantPeek(t *testing.T, r *reader, want byte) {
	t.Helper()
	got, ok := r.peekByte()
	if !ok || got != want {
		t.Fatalf("peekByte = %q, %v, want %q", got, ok, want)
	}
}

func TestLiteralSimple(t *testing.T) {
	n, ok := readLiteral("/Name1")
	if !ok || !n.IsLiteral() {
		t.Fatal("no literal name")
	}
	wantStr(t, n, "Name1")
}

func TestLiteralEmptyName(t *testing.T) {
	n, ok := readLiteral("/")
	if !ok || !n.IsLiteral() {
		t.Fatal("no literal name")
	}
	wantStr(t, n, "")
}

func TestLiteralWithNumberSign(t *testing.T) {
	n, ok := readLiteral("/lime#20Green")
	if !ok || string(n.AsBytes()) != "lime#20Green" {
		t.Fatalf("name = %q, %v", n.AsBytes(), ok)
	}
}

func TestLiteralSpecialChars(t *testing.T) {
	n, ok := readLiteral("/A;Name_With-Various***Characters?")
	if !ok {
		t.Fatal("no literal name")
	}
	wantStr(t, n, "A;Name_With-Various***Characters?")
}

func TestLiteralStopsAtDelimiter(t *testing.T) {
	r := newReader([]byte("/Name(rest"))
	data, ok := parseLiteralName(r)
	if !ok || string(data) != "Name" {
		t.Fatalf("name = %q, %v", data, ok)
	}
	wantPeek(t, r, '(')
}

func TestLiteralStopsAtWhitespace(t *testing.T) {
	r := newReader([]byte("/Name rest"))
	data, ok := parseLiteralName(r)
	if !ok || string(data) != "Name" {
		t.Fatalf("name = %q, %v", data, ok)
	}
	wantPeek(t, r, ' ')
}

func TestLiteralNotAName(t *testing.T) {
	if _, ok := readLiteral("Name"); ok {
		t.Fatal("read a literal name without a slash")
	}
}

func TestImmediateSimple(t *testing.T) {
	r := newReader([]byte("//Helvetica "))
	data, ok := parseImmediateName(r)
	if !ok || string(data) != "Helvetica" {
		t.Fatalf("name = %q, %v", data, ok)
	}
	wantPeek(t, r, ' ')
}

func TestImmediateEmpty(t *testing.T) {
	r := newReader([]byte("//(x)"))
	data, ok := parseImmediateName(r)
	if !ok || len(data) != 0 {
		t.Fatalf("name = %q, %v", data, ok)
	}
	wantPeek(t, r, '(')
}

func TestExecutableSimple(t *testing.T) {
	n, ok := readExecutable("beginbfchar ")
	if !ok || n.IsLiteral() {
		t.Fatal("no executable name")
	}
	wantStr(t, n, "beginbfchar")
}

func TestExecutableStopsAtDelimiter(t *testing.T) {
	r := newReader([]byte("def/name"))
	data, ok := parseExecutableName(r)
	if !ok || string(data) != "def" {
		t.Fatalf("name = %q, %v", data, ok)
	}
	wantPeek(t, r, '/')
}

func TestExecutableAtEOF(t *testing.T) {
	n, ok := readExecutable("endcmap")
	if !ok {
		t.Fatal("no executable name")
	}
	wantStr(t, n, "endcmap")
}

func TestExecutableEmpty(t *testing.T) {
	if _, ok := readExecutable(""); ok {
		t.Fatal("read an empty executable name")
	}
}

func TestExecutableStartsAtDelimiter(t *testing.T) {
	if _, ok := readExecutable("(foo)"); ok {
		t.Fatal("read an executable name that starts at a delimiter")
	}
}
