// Copyright (c) The Hayro Authors
//
// Use of this source code is governed by the Apache-2.0 or MIT license that
// can be found in the LICENSE-APACHE and LICENSE-MIT files.

// This file ports hayro-syntax src/object/null.rs.

package syntax

// Null is Null in hayro-syntax src/object/null.rs. It is the null object.
type Null struct{}

// String returns the Display text of upstream.
func (Null) String() string {
	return "null"
}

// GoString returns the Debug text of upstream.
func (Null) GoString() string {
	return "Null"
}

// skipNull is the Skippable implementation of Null in hayro-syntax
// src/object/null.rs.
func skipNull(r *Reader) bool {
	if !r.ForwardTag("null") {
		return false
	}
	b, ok := r.PeekByte()
	return !ok || !isRegularCharacter(b)
}

// readNull is the Readable implementation of Null in hayro-syntax
// src/object/null.rs.
func readNull(r *Reader) bool {
	return skipNull(r)
}
