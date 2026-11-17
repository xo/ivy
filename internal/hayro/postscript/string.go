// Copyright (c) The Hayro Authors.
//
// Use of this source code is governed by the Apache-2.0 or MIT license that
// can be found in the LICENSE-APACHE and LICENSE-MIT files.

// This file ports hayro-postscript src/string.rs.

package postscript

import "bytes"

// stringKind is StringInner in hayro-postscript src/string.rs, without its
// data. String holds the data.
type stringKind uint8

// The kinds of a string.
const (
	stringLiteral stringKind = iota
	stringHex
	stringASCII85
)

// String is String in hayro-postscript src/string.rs. It is a PostScript
// string object. It holds the encoded bytes between the delimiters, and it
// decodes them on request.
type String struct {
	data []byte
	kind stringKind
}

// stringFromLiteral is String::from_literal in hayro-postscript
// src/string.rs.
func stringFromLiteral(data []byte) String {
	return String{data: data, kind: stringLiteral}
}

// stringFromHex is String::from_hex in hayro-postscript src/string.rs.
func stringFromHex(data []byte) String {
	return String{data: data, kind: stringHex}
}

// stringFromASCII85 is String::from_ascii85 in hayro-postscript
// src/string.rs.
func stringFromASCII85(data []byte) String {
	return String{data: data, kind: stringASCII85}
}

// DecodeInto is String::decode_into in hayro-postscript src/string.rs. It
// decodes the string into out, in place of the old content of out, and it
// returns the result.
//
// Port: upstream fills a Vec that the caller owns. Go returns the slice, as
// append does.
func (s String) DecodeInto(out []byte) ([]byte, error) {
	out = out[:0]
	var ok bool
	switch s.kind {
	case stringHex:
		out, ok = decodeASCIIHex(s.data, out)
	case stringASCII85:
		out, ok = decodeASCII85(s.data, out)
	default:
		out, ok = decodeLiteral(s.data, out)
	}
	if !ok {
		return out[:0], ErrSyntax
	}
	return out, nil
}

// Decode is String::decode in hayro-postscript src/string.rs. It decodes the
// string into a new slice.
func (s String) Decode() ([]byte, error) {
	return s.DecodeInto(nil)
}

// Equal is the derived PartialEq of String in hayro-postscript
// src/string.rs.
//
// Port: Go cannot compare a struct that holds a slice with ==.
func (s String) Equal(o String) bool {
	return s.kind == o.kind && bytes.Equal(s.data, o.data)
}

// isToken marks String as a Token.
func (String) isToken() {}

// parseLiteralString is parse_literal in hayro-postscript src/string.rs. It
// reads a string in parentheses and returns the bytes between them.
func parseLiteralString(r *reader) ([]byte, bool) {
	start := r.getOffset()
	if !skipLiteralString(r) {
		return nil, false
	}
	end := r.getOffset()
	return r.rangeOf(start+1, end-1)
}

// parseHexString is parse_hex in hayro-postscript src/string.rs. It reads a
// string in angle brackets and returns the bytes between them.
func parseHexString(r *reader) ([]byte, bool) {
	if !r.forwardTag("<") {
		return nil, false
	}
	start := r.getOffset()
	for {
		b, ok := r.readByte()
		if !ok {
			return nil, false
		}
		if b == '>' {
			return r.rangeOf(start, r.getOffset()-1)
		}
	}
}

// parseASCII85String is parse_ascii85 in hayro-postscript src/string.rs. It
// reads a string between <~ and ~> and returns the bytes between them.
func parseASCII85String(r *reader) ([]byte, bool) {
	if !r.forwardTag("<~") {
		return nil, false
	}
	start := r.getOffset()
	for {
		b, ok := r.readByte()
		if !ok {
			return nil, false
		}
		if b == '~' {
			end := r.getOffset() - 1
			if !r.forwardTag(">") {
				return nil, false
			}
			return r.rangeOf(start, end)
		}
	}
}

// skipLiteralString is skip_literal in hayro-postscript src/string.rs. It
// moves past a string in parentheses. It counts the depth of the
// parentheses, so it needs no recursion.
func skipLiteralString(r *reader) bool {
	if !r.forwardTag("(") {
		return false
	}
	depth := uint32(1)

	for depth > 0 {
		b, ok := r.readByte()
		if !ok {
			return false
		}
		switch b {
		case '\\':
			if _, ok := r.readByte(); !ok {
				return false
			}
		case '(':
			depth++
		case ')':
			depth--
		}
	}

	return true
}
