// Copyright (c) The Hayro Authors.
//
// Use of this source code is governed by the Apache-2.0 or MIT license that
// can be found in the LICENSE-APACHE and LICENSE-MIT files.

// This file ports hayro-postscript src/lib.rs.

// Package postscript is a port of the Rust crate hayro-postscript, a small
// scanner of PostScript.
//
// The scanner splits a PostScript program into typed tokens. It reads only a
// small part of the PostScript language. That part is enough to read a CMap
// file. A token is an integer or a real number, a name, a string or a
// procedure. The scanner returns [, ], << and >> as executable names, so that
// a caller can build arrays and dictionaries.
//
// Each error is an Error that wraps ivy.ErrFormat or ivy.ErrLimit.
package postscript

// Scanner is Scanner in hayro-postscript src/lib.rs. It reads Tokens from the
// bytes of a PostScript program. Each token points into these bytes.
type Scanner struct {
	r *reader
}

// NewScanner is Scanner::new in hayro-postscript src/lib.rs. It returns a
// scanner over data.
func NewScanner(data []byte) *Scanner {
	return &Scanner{r: newReader(data)}
}

// Offset is Scanner::offset in hayro-postscript src/lib.rs. It returns the
// current offset into the data of the scanner.
func (s *Scanner) Offset() int {
	return s.r.getOffset()
}

// SetOffset is Scanner::set_offset in hayro-postscript src/lib.rs. It moves
// the scanner to an offset into its data. The offset can be past the end.
// After a negative offset, each read fails.
//
// Port: upstream takes a usize, which cannot be negative.
func (s *Scanner) SetOffset(offset int) {
	s.r.jump(offset)
}

// AtEnd is Scanner::at_end in hayro-postscript src/lib.rs. It reports
// whether the data holds no more tokens.
func (s *Scanner) AtEnd() bool {
	return atEnd(s.r)
}

// ParseToken is Scanner::parse_token in hayro-postscript src/lib.rs. It reads
// the next token.
func (s *Scanner) ParseToken() (Token, error) {
	return readToken(s.r)
}

// ParseNumber is Scanner::parse_number in hayro-postscript src/lib.rs. It
// reads the next token, which must be a Number.
func (s *Scanner) ParseNumber() (Number, error) {
	t, err := s.ParseToken()
	if err != nil {
		return Number{}, err
	}
	n, ok := t.(Number)
	if !ok {
		return Number{}, ErrSyntax
	}
	return n, nil
}

// ParseName is Scanner::parse_name in hayro-postscript src/lib.rs. It reads
// the next token, which must be a Name.
func (s *Scanner) ParseName() (Name, error) {
	t, err := s.ParseToken()
	if err != nil {
		return Name{}, err
	}
	n, ok := t.(Name)
	if !ok {
		return Name{}, ErrSyntax
	}
	return n, nil
}

// ParseString is Scanner::parse_string in hayro-postscript src/lib.rs. It
// reads the next token, which must be a String.
func (s *Scanner) ParseString() (String, error) {
	t, err := s.ParseToken()
	if err != nil {
		return String{}, err
	}
	str, ok := t.(String)
	if !ok {
		return String{}, ErrSyntax
	}
	return str, nil
}

// ParseProcedure is Scanner::parse_procedure in hayro-postscript src/lib.rs.
// It reads the next token, which must be a Procedure.
func (s *Scanner) ParseProcedure() (Procedure, error) {
	t, err := s.ParseToken()
	if err != nil {
		return Procedure{}, err
	}
	p, ok := t.(Procedure)
	if !ok {
		return Procedure{}, ErrSyntax
	}
	return p, nil
}
