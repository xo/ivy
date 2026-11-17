// Copyright (c) The Hayro Authors
//
// Use of this source code is governed by the Apache-2.0 or MIT license that
// can be found in the LICENSE-APACHE and LICENSE-MIT files.

// This file ports hayro-syntax src/object/bool.rs.

package syntax

// Bool is the boolean object, the case Boolean of Object in hayro-syntax
// src/object/mod.rs.
//
// Port: upstream uses the type bool for the case. Go needs a named type,
// so that it can satisfy Object.
type Bool bool

// String returns the Display text of upstream.
func (b Bool) String() string {
	if b {
		return "true"
	}
	return "false"
}

// GoString returns the Debug text of upstream.
func (b Bool) GoString() string {
	return b.String()
}

// skipBool is the Skippable implementation of bool in hayro-syntax
// src/object/bool.rs.
func skipBool(r *Reader) bool {
	b, ok := r.PeekByte()
	if !ok {
		return false
	}
	switch b {
	case 't':
		if !r.ForwardTag("true") {
			return false
		}
	case 'f':
		if !r.ForwardTag("false") {
			return false
		}
	default:
		return false
	}
	next, ok := r.PeekByte()
	return !ok || !isRegularCharacter(next)
}

// readBool is the Readable implementation of bool in hayro-syntax
// src/object/bool.rs.
func readBool(r *Reader) (Bool, bool) {
	b, ok := r.Skip[Bool](true)
	if !ok {
		return false, false
	}
	switch string(b) {
	case "true":
		return true, true
	case "false":
		return false, true
	}
	return false, false
}
