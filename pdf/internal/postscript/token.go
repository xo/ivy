// Copyright (c) The Hayro Authors.
//
// Use of this source code is governed by the Apache-2.0 or MIT license that
// can be found in the LICENSE-APACHE and LICENSE-MIT files.

// This file ports hayro-postscript src/token.rs.

package postscript

import "errors"

// Token is Token in hayro-postscript src/token.rs. It is a PostScript token:
// a Number, a Name, a String or a Procedure.
//
// Port: upstream is an enum with data. Go uses an interface that only the
// four types of this package implement. A caller tells them apart with a
// type switch.
type Token interface {
	isToken()
}

// TokenEqual is the derived PartialEq of Token in hayro-postscript
// src/token.rs. It reports whether a and b are the same token.
//
// Port: Go cannot compare a struct that holds a slice with ==.
func TokenEqual(a, b Token) bool {
	switch a := a.(type) {
	case Number:
		b, ok := b.(Number)
		return ok && a == b
	case Name:
		b, ok := b.(Name)
		return ok && a.Equal(b)
	case String:
		b, ok := b.(String)
		return ok && a.Equal(b)
	case Procedure:
		b, ok := b.(Procedure)
		return ok && a.Equal(b)
	}
	return a == nil && b == nil
}

// readToken is read in hayro-postscript src/token.rs. It skips whitespace
// and comments, and it reads the next token. On success, it always moves
// past at least one byte.
func readToken(r *reader) (Token, error) {
	skipWhitespaceAndComments(r)

	b, ok := r.peekByte()
	if !ok {
		return nil, ErrSyntax
	}

	switch {
	case b == '(':
		s, ok := parseLiteralString(r)
		if !ok {
			return nil, ErrSyntax
		}
		return stringFromLiteral(s), nil
	case b == '<':
		switch {
		case r.peekTag("<~"):
			s, ok := parseASCII85String(r)
			if !ok {
				return nil, ErrSyntax
			}
			return stringFromASCII85(s), nil
		case r.peekTag("<<"):
			return readDelimiterName(r, 2)
		default:
			s, ok := parseHexString(r)
			if !ok {
				return nil, ErrSyntax
			}
			return stringFromHex(s), nil
		}
	case b == '/':
		if r.peekTag("//") {
			s, ok := parseImmediateName(r)
			if !ok {
				return nil, ErrSyntax
			}
			return newImmediateName(s), nil
		}
		s, ok := parseLiteralName(r)
		if !ok {
			return nil, ErrSyntax
		}
		return newName(s, true), nil
	case b == '[' || b == ']':
		return readDelimiterName(r, 1)
	case b == '>':
		if r.peekTag(">>") {
			return readDelimiterName(r, 2)
		}
		return nil, ErrSyntax
	case b == '{':
		d, err := parseProcedure(r)
		if err != nil {
			return nil, err
		}
		return newProcedure(d), nil
	case b == '.' || b == '+' || b == '-' || isASCIIDigit(b):
		token, ok := parseExecutableName(r)
		if !ok {
			return nil, ErrSyntax
		}
		n, err := parseNumber(token)
		switch {
		case errors.Is(err, ErrSyntax):
			// A token that only looks like a number is a name, such as 23A.
			return newName(token, false), nil
		case err != nil:
			return nil, err
		}
		return n, nil
	default:
		s, ok := parseExecutableName(r)
		if !ok {
			return nil, ErrSyntax
		}
		return newName(s, false), nil
	}
}

// readDelimiterName is read_delimiter_name in hayro-postscript src/token.rs.
// It reads [, ], << or >> as an executable name.
func readDelimiterName(r *reader, n int) (Token, error) {
	s, ok := r.readBytes(n)
	if !ok {
		return nil, ErrSyntax
	}
	return newName(s, false), nil
}

// atEnd is at_end in hayro-postscript src/token.rs. It skips whitespace and
// comments, and it reports whether the data ends.
func atEnd(r *reader) bool {
	skipWhitespaceAndComments(r)
	_, ok := r.peekByte()
	return !ok
}

// skipWhitespaceAndComments is skip_whitespace_and_comments in
// hayro-postscript src/token.rs.
func skipWhitespaceAndComments(r *reader) {
	for {
		b, ok := r.peekByte()
		switch {
		case !ok:
			return
		case isWhitespace(b):
			r.forward()
		case b == '%':
			r.forward()
			r.forwardWhile(func(b byte) bool { return !isEOL(b) })
		default:
			return
		}
	}
}
