// Copyright (c) The Hayro Authors.
//
// Use of this source code is governed by the Apache-2.0 or MIT license that
// can be found in the LICENSE-APACHE and LICENSE-MIT files.

// This file ports hayro-postscript src/error.rs.

package postscript

import "github.com/xo/ivy"

// Error is Error in hayro-postscript src/error.rs. It is an error that the
// scanner finds in a PostScript token stream.
//
// Port: upstream also declares the type Result. Go returns an error next to
// the value. Each Error wraps a sentinel error of ivy, so that a caller can
// test it with errors.Is.
type Error uint8

// The errors of the scanner.
const (
	// ErrSyntax is Error::SyntaxError in hayro-postscript src/error.rs. It
	// reports a syntax error in the input. It wraps ivy.ErrFormat.
	ErrSyntax Error = iota
	// ErrLimitCheck is Error::LimitCheck in hayro-postscript src/error.rs. It
	// reports a number that exceeds a limit of the implementation. It wraps
	// ivy.ErrLimit.
	ErrLimitCheck
)

// Error is fmt::Display for Error in hayro-postscript src/error.rs.
func (e Error) Error() string {
	if e == ErrLimitCheck {
		return "postscript: limitcheck"
	}
	return "postscript: syntaxerror"
}

// Unwrap returns the sentinel error of ivy that e wraps.
func (e Error) Unwrap() error {
	if e == ErrLimitCheck {
		return ivy.ErrLimit
	}
	return ivy.ErrFormat
}
