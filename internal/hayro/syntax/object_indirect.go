// Copyright (c) The Hayro Authors
//
// Use of this source code is governed by the Apache-2.0 or MIT license that
// can be found in the LICENSE-APACHE and LICENSE-MIT files.

// This file ports hayro-syntax src/object/indirect.rs.

package syntax

// IndirectObject is IndirectObject in hayro-syntax src/object/indirect.rs.
// It is an object with its identifier, as "1 0 obj ... endobj" holds it.
type IndirectObject[T any] struct {
	id    ObjectIdentifier
	inner T
}

// Get is IndirectObject::get in hayro-syntax src/object/indirect.rs.
func (o IndirectObject[T]) Get() T {
	return o.inner
}

// ID is IndirectObject::id in hayro-syntax src/object/indirect.rs.
func (o IndirectObject[T]) ID() ObjectIdentifier {
	return o.id
}

// readFrom is the Readable implementation of IndirectObject in hayro-syntax
// src/object/indirect.rs.
func (o *IndirectObject[T]) readFrom(r *Reader, ctx *ReaderContext) bool {
	ctx = ctx.clone()
	id, ok := r.ReadWithoutContext[ObjectIdentifier]()
	if !ok {
		return false
	}
	if ctx.parentChainContains(id) {
		return false
	}
	ctx.setObjNumber(id)
	ctx.parentChainPush(id)
	r.SkipWhiteSpacesAndComments()
	inner, ok := r.ReadIndirect[T](ctx)
	if !ok {
		return false
	}
	r.SkipWhiteSpacesAndComments()
	// Lenient: endobj is not required.
	r.ForwardTag("endobj")
	*o = IndirectObject[T]{id: id, inner: inner}
	return true
}

// skipFrom is the Skippable implementation of IndirectObject in hayro-syntax
// src/object/indirect.rs.
func (o *IndirectObject[T]) skipFrom(r *Reader, _ bool, _ int) bool {
	if _, ok := r.Skip[ObjectIdentifier](false); !ok {
		return false
	}
	r.SkipWhiteSpacesAndComments()
	if _, ok := r.Skip[T](false); !ok {
		return false
	}
	r.SkipWhiteSpacesAndComments()
	// Lenient: endobj is not required.
	r.ForwardTag("endobj")
	return true
}
