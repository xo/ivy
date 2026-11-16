// Copyright (c) The Hayro Authors.
//
// Use of this source code is governed by the Apache-2.0 or MIT license that
// can be found in the LICENSE-APACHE and LICENSE-MIT files.

// This file ports hayro-postscript src/name.rs.

package postscript

import (
	"bytes"
	"unicode/utf8"
)

// Name is Name in hayro-postscript src/name.rs. It is a PostScript name
// object. Its bytes point into the data of the scanner.
type Name struct {
	data []byte
	kind nameKind
}

// nameKind is Kind in hayro-postscript src/name.rs.
type nameKind uint8

// The kinds of a name.
const (
	kindExecutable nameKind = iota
	kindLiteral
	kindImmediate
)

// newName is Name::new in hayro-postscript src/name.rs.
func newName(data []byte, literal bool) Name {
	kind := kindExecutable
	if literal {
		kind = kindLiteral
	}
	return Name{data: data, kind: kind}
}

// newImmediateName is Name::new_immediate in hayro-postscript src/name.rs.
func newImmediateName(data []byte) Name {
	return Name{data: data, kind: kindImmediate}
}

// IsLiteral is Name::is_literal in hayro-postscript src/name.rs. It reports
// whether n is a literal name, such as /Name.
func (n Name) IsLiteral() bool {
	return n.kind == kindLiteral
}

// IsImmediate is Name::is_immediate in hayro-postscript src/name.rs. It
// reports whether n is an immediately evaluated name, such as //Name.
func (n Name) IsImmediate() bool {
	return n.kind == kindImmediate
}

// AsStr is Name::as_str in hayro-postscript src/name.rs. It returns the name
// as a string. It returns false if the name is not valid UTF-8.
func (n Name) AsStr() (string, bool) {
	if !utf8.Valid(n.data) {
		return "", false
	}
	return string(n.data), true
}

// AsBytes is Name::as_bytes in hayro-postscript src/name.rs. It returns the
// raw bytes of the name. The caller must not change them.
func (n Name) AsBytes() []byte {
	return n.data
}

// Equal is the derived PartialEq of Name in hayro-postscript src/name.rs.
//
// Port: Go cannot compare a struct that holds a slice with ==.
func (n Name) Equal(o Name) bool {
	return n.kind == o.kind && bytes.Equal(n.data, o.data)
}

// isToken marks Name as a Token.
func (Name) isToken() {}

// parseLiteralName is parse_literal in hayro-postscript src/name.rs. It reads
// a name that starts with a slash.
func parseLiteralName(r *reader) ([]byte, bool) {
	if !r.forwardTag("/") {
		return nil, false
	}
	start := r.getOffset()
	for {
		if _, ok := r.eat(isRegular); !ok {
			break
		}
	}
	return r.rangeOf(start, r.getOffset())
}

// parseImmediateName is parse_immediate in hayro-postscript src/name.rs. It
// reads a name that starts with two slashes.
func parseImmediateName(r *reader) ([]byte, bool) {
	if !r.forwardTag("/") {
		return nil, false
	}
	return parseLiteralName(r)
}

// parseExecutableName is parse_executable in hayro-postscript src/name.rs.
// It reads a name without a slash. The name must not be empty.
func parseExecutableName(r *reader) ([]byte, bool) {
	start := r.getOffset()
	r.forwardWhile(isRegular)
	if r.getOffset() == start {
		return nil, false
	}
	return r.rangeOf(start, r.getOffset())
}
