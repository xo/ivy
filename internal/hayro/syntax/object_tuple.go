// Copyright (c) The Hayro Authors
//
// Use of this source code is governed by the Apache-2.0 or MIT license that
// can be found in the LICENSE-APACHE and LICENSE-MIT files.

// This file ports hayro-syntax src/object/tuple.rs.

package syntax

// readResolved reads a MaybeRef of T and resolves it, as each item of a tuple
// in hayro-syntax src/object/tuple.rs does.
func readResolved[T any](r *Reader, ctx *ReaderContext) (T, bool) {
	r.SkipWhiteSpacesAndComments()
	m, ok := r.Read[MaybeRef[T]](ctx)
	if !ok {
		var zero T
		return zero, false
	}
	return m.Resolve(ctx)
}

// Tuple2 is the tuple (T, U) of hayro-syntax src/object/tuple.rs. It reads
// two objects in a row, and it resolves references.
//
// Port: Go has no tuples. The field Vn holds the item n.
type Tuple2[A, B any] struct {
	V0 A
	V1 B
}

// readFrom is the Readable implementation of (T, U) in hayro-syntax
// src/object/tuple.rs.
func (t *Tuple2[A, B]) readFrom(r *Reader, ctx *ReaderContext) bool {
	var ok0, ok1 bool
	if t.V0, ok0 = readResolved[A](r, ctx); !ok0 {
		return false
	}
	t.V1, ok1 = readResolved[B](r, ctx)
	return ok1
}

// castFrom is the TryFrom<Object> implementation of (T, U) in hayro-syntax
// src/object/tuple.rs. It always fails.
func (t *Tuple2[A, B]) castFrom(Object) bool {
	return false
}

// Tuple3 is the tuple (T, U, V) of hayro-syntax src/object/tuple.rs.
type Tuple3[A, B, C any] struct {
	V0 A
	V1 B
	V2 C
}

// readFrom is the Readable implementation of (T, U, V) in hayro-syntax
// src/object/tuple.rs.
func (t *Tuple3[A, B, C]) readFrom(r *Reader, ctx *ReaderContext) bool {
	var ok bool
	if t.V0, ok = readResolved[A](r, ctx); !ok {
		return false
	}
	if t.V1, ok = readResolved[B](r, ctx); !ok {
		return false
	}
	t.V2, ok = readResolved[C](r, ctx)
	return ok
}

// castFrom is the TryFrom<Object> implementation of (T, U, V) in
// hayro-syntax src/object/tuple.rs. It always fails.
func (t *Tuple3[A, B, C]) castFrom(Object) bool {
	return false
}

// Tuple4 is the tuple (T, U, V, W) of hayro-syntax src/object/tuple.rs.
type Tuple4[A, B, C, D any] struct {
	V0 A
	V1 B
	V2 C
	V3 D
}

// readFrom is the Readable implementation of (T, U, V, W) in hayro-syntax
// src/object/tuple.rs.
func (t *Tuple4[A, B, C, D]) readFrom(r *Reader, ctx *ReaderContext) bool {
	var ok bool
	if t.V0, ok = readResolved[A](r, ctx); !ok {
		return false
	}
	if t.V1, ok = readResolved[B](r, ctx); !ok {
		return false
	}
	if t.V2, ok = readResolved[C](r, ctx); !ok {
		return false
	}
	t.V3, ok = readResolved[D](r, ctx)
	return ok
}

// castFrom is the TryFrom<Object> implementation of (T, U, V, W) in
// hayro-syntax src/object/tuple.rs. It always fails.
func (t *Tuple4[A, B, C, D]) castFrom(Object) bool {
	return false
}

// Tuple5 is the tuple (T, U, V, W, X) of hayro-syntax src/object/tuple.rs.
type Tuple5[A, B, C, D, E any] struct {
	V0 A
	V1 B
	V2 C
	V3 D
	V4 E
}

// readFrom is the Readable implementation of (T, U, V, W, X) in
// hayro-syntax src/object/tuple.rs.
func (t *Tuple5[A, B, C, D, E]) readFrom(r *Reader, ctx *ReaderContext) bool {
	var ok bool
	if t.V0, ok = readResolved[A](r, ctx); !ok {
		return false
	}
	if t.V1, ok = readResolved[B](r, ctx); !ok {
		return false
	}
	if t.V2, ok = readResolved[C](r, ctx); !ok {
		return false
	}
	if t.V3, ok = readResolved[D](r, ctx); !ok {
		return false
	}
	t.V4, ok = readResolved[E](r, ctx)
	return ok
}

// castFrom is the TryFrom<Object> implementation of (T, U, V, W, X) in
// hayro-syntax src/object/tuple.rs. It always fails.
func (t *Tuple5[A, B, C, D, E]) castFrom(Object) bool {
	return false
}
