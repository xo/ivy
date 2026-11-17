// Copyright (c) The Hayro Authors.
//
// Use of this source code is governed by the Apache-2.0 or MIT license that
// can be found in the LICENSE-APACHE and LICENSE-MIT files.

// This file ports the tests of hayro-postscript src/token.rs.

package postscript

import (
	"errors"
	"strings"
	"testing"
)

// readOne is read_one in the tests of hayro-postscript src/token.rs.
func readOne(input string) (Token, error) {
	return readToken(newReader([]byte(input)))
}

// wantToken compares the token of input with want.
func wantToken(t *testing.T, input string, want Token) {
	t.Helper()
	got, err := readOne(input)
	if err != nil || !TokenEqual(got, want) {
		t.Fatalf("readOne(%q) = %#v, %v, want %#v", input, got, err, want)
	}
}

// wantTokenErr compares the error of input with want.
func wantTokenErr(t *testing.T, input string, want error) {
	t.Helper()
	got, err := readOne(input)
	if !errors.Is(err, want) {
		t.Fatalf("readOne(%q) = %#v, %v, want error %v", input, got, err, want)
	}
}

// wantNext reads the next token of r and compares it with want.
func wantNext(t *testing.T, r *reader, want Token) {
	t.Helper()
	got, err := readToken(r)
	if err != nil || !TokenEqual(got, want) {
		t.Fatalf("readToken = %#v, %v, want %#v", got, err, want)
	}
}

func TestTokenInteger(t *testing.T) {
	wantToken(t, "42 ", NewInteger(42))
}

func TestTokenNegativeInteger(t *testing.T) {
	wantToken(t, "-7 ", NewInteger(-7))
}

func TestTokenNumberLikeNames(t *testing.T) {
	for _, name := range []string{"-|", "|-", "23A", "13-456", "-", "+", ".", "1e", "-16#FF", "16#", "37#1", "8#9"} {
		wantToken(t, name, newName([]byte(name), false))
	}
}

func TestTokenNumberLimitCheck(t *testing.T) {
	wantTokenErr(t, "16#10000000000000000 ", ErrLimitCheck)
	wantTokenErr(t, "1e400 ", ErrLimitCheck)
}

func TestTokenLiteralName(t *testing.T) {
	wantToken(t, "/CMapName ", newName([]byte("CMapName"), true))
}

func TestTokenImmediateName(t *testing.T) {
	r := newReader([]byte("//add/x"))
	wantNext(t, r, newImmediateName([]byte("add")))
	wantNext(t, r, newName([]byte("x"), true))
	if !atEnd(r) {
		t.Fatal("atEnd = false")
	}
}

func TestTokenExecutableName(t *testing.T) {
	wantToken(t, "beginbfchar ", newName([]byte("beginbfchar"), false))
}

func TestTokenLiteralString(t *testing.T) {
	wantToken(t, "(Hello)", stringFromLiteral([]byte("Hello")))
}

func TestTokenHexString(t *testing.T) {
	wantToken(t, "<48656C6C6F>", stringFromHex([]byte("48656C6C6F")))
}

func TestTokenASCII85String(t *testing.T) {
	wantToken(t, "<~87cURDZ~>", stringFromASCII85([]byte("87cURDZ")))
}

func TestTokenArrayDelimiters(t *testing.T) {
	r := newReader([]byte("[1]]"))
	wantNext(t, r, newName([]byte("["), false))
	wantNext(t, r, NewInteger(1))
	wantNext(t, r, newName([]byte("]"), false))
	wantNext(t, r, newName([]byte("]"), false))
	if !atEnd(r) {
		t.Fatal("atEnd = false")
	}
}

func TestTokenProcedure(t *testing.T) {
	wantToken(t, "{1 {2} add}", newProcedure([]byte("1 {2} add")))
}

func TestTokenStrayCloseBrace(t *testing.T) {
	wantTokenErr(t, "}", ErrSyntax)
}

func TestTokenDictDelimiters(t *testing.T) {
	wantToken(t, "<</A", newName([]byte("<<"), false))
	wantToken(t, ">>/A", newName([]byte(">>"), false))
}

func TestTokenDictDelimitersAroundHexString(t *testing.T) {
	r := newReader([]byte("<<<41>>>"))
	wantNext(t, r, newName([]byte("<<"), false))
	wantNext(t, r, stringFromHex([]byte("41")))
	wantNext(t, r, newName([]byte(">>"), false))
	if !atEnd(r) {
		t.Fatal("atEnd = false")
	}
}

func TestTokenManyDictDelimiters(t *testing.T) {
	r := newReader([]byte(strings.Repeat("<<", 1_000_000)))
	want := newName([]byte("<<"), false)
	for !atEnd(r) {
		got, err := readToken(r)
		if err != nil || !TokenEqual(got, want) {
			t.Fatalf("readToken = %#v, %v, want <<", got, err)
		}
	}
}

func TestTokenStrayGt(t *testing.T) {
	wantTokenErr(t, ">x", ErrSyntax)
}

func TestTokenSkipsWhitespace(t *testing.T) {
	wantToken(t, "  \t\n 42 ", NewInteger(42))
}

func TestTokenSkipsComments(t *testing.T) {
	wantToken(t, "% this is a comment\n42 ", NewInteger(42))
}

func TestTokenEOF(t *testing.T) {
	for _, input := range []string{"", "   ", "% comment only\n"} {
		if _, err := readOne(input); err == nil {
			t.Errorf("readOne(%q) returned no error", input)
		}
	}
}
