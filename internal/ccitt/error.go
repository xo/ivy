// Copyright (c) The Hayro Authors.
//
// Use of this source code is governed by the Apache-2.0 or MIT license that
// can be found in the LICENSE-APACHE and LICENSE-MIT files.

// This file ports hayro-ccitt src/error.rs.
//
// Port: the type alias Result of upstream is not ported. A Go function
// returns its value and an error.

package ccitt

import (
	"context"

	"github.com/xo/ivy"
)

// DecodeError is DecodeError in hayro-ccitt src/error.rs. It is an error of
// the decoder.
//
// Port: each value unwraps to a sentinel error of ivy, or to
// context.Canceled, so that a caller can test it with errors.Is.
type DecodeError uint8

// The values of DecodeError.
const (
	// ErrUnexpectedEOF is DecodeError::UnexpectedEof. The input ends while
	// the decoder reads bits.
	ErrUnexpectedEOF DecodeError = iota + 1
	// ErrInvalidCode is DecodeError::InvalidCode. The input holds a code
	// that is not valid.
	ErrInvalidCode
	// ErrOverflow is DecodeError::Overflow. The computation of a run length
	// or of a position overflowed.
	ErrOverflow
	// ErrCancelled is DecodeError::Cancelled. The check for cancellation
	// stopped the decoder.
	ErrCancelled
)

// Error satisfies the error interface. It is fmt::Display for DecodeError
// in hayro-ccitt src/error.rs.
func (e DecodeError) Error() string {
	switch e {
	case ErrUnexpectedEOF:
		return "ccitt: unexpected end of input"
	case ErrInvalidCode:
		return "ccitt: invalid CCITT code sequence"
	case ErrOverflow:
		return "ccitt: arithmetic overflow in position calculation"
	case ErrCancelled:
		return "ccitt: decoding was cancelled"
	}
	return "ccitt: unknown error"
}

// Unwrap returns ivy.ErrFormat, or context.Canceled for ErrCancelled.
func (e DecodeError) Unwrap() error {
	if e == ErrCancelled {
		return context.Canceled
	}
	return ivy.ErrFormat
}
