// Copyright (c) The Hayro Authors.
//
// Use of this source code is governed by the Apache-2.0 or MIT license that
// can be found in the LICENSE-APACHE and LICENSE-MIT files.

// This file ports the tests of hayro-postscript src/string.rs.

package postscript

import (
	"bytes"
	"testing"
)

// decodeWith reads a string from input with parse and decodes it with mk.
func decodeWith(input string, parse func(*reader) ([]byte, bool), mk func([]byte) String) ([]byte, error) {
	data, ok := parse(newReader([]byte(input)))
	if !ok {
		return nil, ErrSyntax
	}
	return mk(data).Decode()
}

// wantDecode compares the decoded string with want.
func wantDecode(t *testing.T, input string, got []byte, err error, want string) {
	t.Helper()
	if err != nil || !bytes.Equal(got, []byte(want)) {
		t.Fatalf("decode(%q) = %q, %v, want %q", input, got, err, want)
	}
}

// testLiteral is decode_literal in the tests of hayro-postscript
// src/string.rs, with the comparison.
func testLiteral(t *testing.T, input, want string) {
	t.Helper()
	got, err := decodeWith(input, parseLiteralString, stringFromLiteral)
	wantDecode(t, input, got, err, want)
}

// testHex is decode_hex in the tests of hayro-postscript src/string.rs, with
// the comparison.
func testHex(t *testing.T, input, want string) {
	t.Helper()
	got, err := decodeWith(input, parseHexString, stringFromHex)
	wantDecode(t, input, got, err, want)
}

// decodeA85 is decode_a85 in the tests of hayro-postscript src/string.rs.
func decodeA85(input string) ([]byte, error) {
	return decodeWith(input, parseASCII85String, stringFromASCII85)
}

func TestLiteralEmpty(t *testing.T)        { testLiteral(t, "()", "") }
func TestLiteralSimpleString(t *testing.T) { testLiteral(t, "(Hello)", "Hello") }
func TestLiteralNestedParens(t *testing.T) { testLiteral(t, "(Hi (()) there)", "Hi (()) there") }
func TestLiteralEscapeN(t *testing.T)      { testLiteral(t, `(a\nb)`, "a\nb") }
func TestLiteralEscapeR(t *testing.T)      { testLiteral(t, `(a\rb)`, "a\rb") }
func TestLiteralEscapeT(t *testing.T)      { testLiteral(t, `(a\tb)`, "a\tb") }
func TestLiteralEscapeB(t *testing.T)      { testLiteral(t, `(a\bb)`, "a\x08b") }
func TestLiteralEscapeF(t *testing.T)      { testLiteral(t, `(a\fb)`, "a\x0cb") }
func TestLiteralEscapeBackslash(t *testing.T) {
	testLiteral(t, `(a\\b)`, `a\b`)
}
func TestLiteralEscapeParens(t *testing.T)     { testLiteral(t, `(Hi \()`, "Hi (") }
func TestLiteralOctalThreeDigits(t *testing.T) { testLiteral(t, `(\053)`, "+") }
func TestLiteralOctalTwoDigits(t *testing.T)   { testLiteral(t, `(\36)`, "\x1e") }
func TestLiteralOctalOneDigit(t *testing.T)    { testLiteral(t, `(\3)`, "\x03") }
func TestLiteralLineContinuationLF(t *testing.T) {
	testLiteral(t, "(Hi \\\nthere)", "Hi there")
}
func TestLiteralLineContinuationCR(t *testing.T) {
	testLiteral(t, "(Hi \\\rthere)", "Hi there")
}
func TestLiteralLineContinuationCRLF(t *testing.T) {
	testLiteral(t, "(Hi \\\r\nthere)", "Hi there")
}
func TestLiteralBareEOLLF(t *testing.T)   { testLiteral(t, "(a\nb)", "a\nb") }
func TestLiteralBareEOLCR(t *testing.T)   { testLiteral(t, "(a\rb)", "a\nb") }
func TestLiteralBareEOLCRLF(t *testing.T) { testLiteral(t, "(a\r\nb)", "a\nb") }

func TestHexSimple(t *testing.T)         { testHex(t, "<48656C6C6F>", "Hello") }
func TestHexWithWhitespace(t *testing.T) { testHex(t, "<48 65 6C 6C 6F>", "Hello") }
func TestHexOddNibble(t *testing.T)      { testHex(t, "<ABC>", "\xab\xc0") }
func TestHexEmpty(t *testing.T)          { testHex(t, "<>", "") }
func TestHexLowercase(t *testing.T)      { testHex(t, "<abcd>", "\xab\xcd") }
func TestHexMixedCase(t *testing.T)      { testHex(t, "<aB3E>", "\xab\x3e") }

func TestASCII85Simple(t *testing.T) {
	got, err := decodeA85("<~87cURDZ~>")
	wantDecode(t, "<~87cURDZ~>", got, err, "Hello")
}

func TestASCII85Empty(t *testing.T) {
	got, err := decodeA85("<~~>")
	wantDecode(t, "<~~>", got, err, "")
}

func TestASCII85ZShorthand(t *testing.T) {
	got, err := decodeA85("<~z~>")
	wantDecode(t, "<~z~>", got, err, "\x00\x00\x00\x00")
}

func TestASCII85PartialGroup(t *testing.T) {
	got, err := decodeA85("<~87~>")
	if err != nil || len(got) != 1 {
		t.Fatalf("decode = %q, %v, want one byte", got, err)
	}
}

func TestASCII85WithWhitespace(t *testing.T) {
	got, err := decodeA85("<~87cU RDZ~>")
	wantDecode(t, "<~87cU RDZ~>", got, err, "Hello")
}

// TestASCII85Malformed tests the errors of the decoder. Upstream has no such
// test.
func TestASCII85Malformed(t *testing.T) {
	for _, input := range []string{"<~8~>", "<~uuuuu~>", "<~87v~>"} {
		if got, err := decodeA85(input); err == nil {
			t.Errorf("decode(%q) = %q, want an error", input, got)
		}
	}
}
