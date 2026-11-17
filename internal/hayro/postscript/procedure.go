// Copyright (c) The Hayro Authors.
//
// Use of this source code is governed by the Apache-2.0 or MIT license that
// can be found in the LICENSE-APACHE and LICENSE-MIT files.

// This file ports hayro-postscript src/procedure.rs.

package postscript

import "bytes"

// Procedure is Procedure in hayro-postscript src/procedure.rs. It is a
// PostScript procedure, an executable array. It holds the bytes between the
// braces.
type Procedure struct {
	data []byte
}

// newProcedure is Procedure::new in hayro-postscript src/procedure.rs.
func newProcedure(data []byte) Procedure {
	return Procedure{data: data}
}

// Tokens is Procedure::tokens in hayro-postscript src/procedure.rs. It
// returns a Scanner over the tokens of the procedure.
func (p Procedure) Tokens() *Scanner {
	return NewScanner(p.data)
}

// Equal is the derived PartialEq of Procedure in hayro-postscript
// src/procedure.rs.
//
// Port: Go cannot compare a struct that holds a slice with ==.
func (p Procedure) Equal(o Procedure) bool {
	return bytes.Equal(p.data, o.data)
}

// isToken marks Procedure as a Token.
func (Procedure) isToken() {}

// parseProcedure is parse in hayro-postscript src/procedure.rs. It reads a
// procedure and returns the bytes between its braces.
func parseProcedure(r *reader) ([]byte, error) {
	if !r.forwardTag("{") {
		return nil, ErrSyntax
	}

	start := r.getOffset()
	if err := skipProcedure(r); err != nil {
		return nil, err
	}
	end := r.getOffset() - 1

	data, ok := r.rangeOf(start, end)
	if !ok {
		return nil, ErrSyntax
	}
	return data, nil
}

// skipProcedure is skip in hayro-postscript src/procedure.rs. It moves past
// the end of the procedure. It counts the depth of the braces, so it needs
// no recursion.
func skipProcedure(r *reader) error {
	depth := uint32(1)

	for depth > 0 {
		b, ok := r.peekByte()
		if !ok {
			return ErrSyntax
		}
		switch b {
		case '{':
			r.forward()
			depth++
		case '}':
			r.forward()
			depth--
		case '(':
			if _, ok := parseLiteralString(r); !ok {
				return ErrSyntax
			}
		case '<':
			switch {
			case r.peekTag("<~"):
				if _, ok := parseASCII85String(r); !ok {
					return ErrSyntax
				}
			case r.peekTag("<<"):
				r.forward()
				r.forward()
			default:
				if _, ok := parseHexString(r); !ok {
					return ErrSyntax
				}
			}
		case '%':
			skipWhitespaceAndComments(r)
		default:
			r.forward()
		}
	}

	return nil
}
