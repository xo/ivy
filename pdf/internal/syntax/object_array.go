// Copyright (c) The Hayro Authors
//
// Use of this source code is governed by the Apache-2.0 or MIT license that
// can be found in the LICENSE-APACHE and LICENSE-MIT files.

// This file ports hayro-syntax src/object/array.rs.

package syntax

import (
	"bytes"
	"iter"
	"strings"
)

// Array is Array in hayro-syntax src/object/array.rs. It is an array of PDF
// objects. It holds the raw bytes between the brackets, and it reads an item
// when a caller asks for it.
type Array struct {
	data []byte
	ctx  *ReaderContext
}

// Equal reports whether two arrays hold the same raw bytes, as the
// PartialEq of upstream does. White space matters.
func (a Array) Equal(other Array) bool {
	return bytes.Equal(a.data, other.data)
}

// context returns the context of the array, or a dummy context for the zero
// Array.
func (a Array) context() *ReaderContext {
	if a.ctx == nil {
		return DummyReaderContext()
	}
	return a.ctx
}

// RawIter is Array::raw_iter in hayro-syntax src/object/array.rs. It returns
// an iterator over the items, without resolving references.
func (a Array) RawIter() *ArrayIter {
	return &ArrayIter{reader: NewReader(a.data), ctx: a.context()}
}

// Iter is Array::iter in hayro-syntax src/object/array.rs. It returns an
// iterator over the items as a T, which resolves references. The iterator
// stops at the first item that is not a T.
func (a Array) Iter[T any]() *ResolvedArrayIter[T] {
	return &ResolvedArrayIter[T]{flex: a.FlexIter()}
}

// FlexIter is Array::flex_iter in hayro-syntax src/object/array.rs. It
// returns an iterator that can read each item as another type.
func (a Array) FlexIter() *FlexArrayIter {
	return &FlexArrayIter{reader: NewReader(a.data), ctx: a.context()}
}

// Data is Array::data in hayro-syntax src/object/array.rs. It returns the raw
// bytes of the array.
func (a Array) Data() []byte {
	return a.data
}

// GoString returns the Debug text of upstream.
func (a Array) GoString() string {
	var b strings.Builder
	b.WriteString("[")
	i := 0
	for item := range a.RawIter().All() {
		if i > 0 {
			b.WriteString(", ")
		}
		b.WriteString(item.GoString())
		i++
	}
	b.WriteString("]")
	return b.String()
}

// String returns the Display text of upstream.
func (a Array) String() string {
	var b strings.Builder
	b.WriteString("[")
	i := 0
	for item := range a.RawIter().All() {
		if i > 0 {
			b.WriteString(" ")
		}
		b.WriteString(item.String())
		i++
	}
	b.WriteString("]")
	return b.String()
}

// skipArray is the Skippable implementation of Array in hayro-syntax
// src/object/array.rs.
func skipArray(r *Reader, isContentStream bool, depth int) bool {
	if depth >= maxObjectNestingDepth {
		return false
	}
	if !r.ForwardTag("[") {
		return false
	}
	for {
		r.SkipWhiteSpacesAndComments()
		if r.ForwardTag("]") {
			return true
		}
		var ok bool
		if isContentStream {
			_, ok = r.SkipWithDepth[Object](true, depth+1)
		} else {
			_, ok = r.SkipWithDepth[MaybeRef[Object]](false, depth+1)
		}
		if !ok {
			return false
		}
	}
}

// DefaultArray is the Default implementation of Array in hayro-syntax
// src/object/array.rs: the empty array.
func DefaultArray() Array {
	a, _ := FromBytes[Array]([]byte("[]"))
	return a
}

// readArray is the Readable implementation of Array in hayro-syntax
// src/object/array.rs.
func readArray(r *Reader, ctx *ReaderContext) (Array, bool) {
	b, ok := r.Skip[Array](ctx.InContentStream())
	if !ok {
		return Array{}, false
	}
	return Array{data: b[1 : len(b)-1], ctx: ctx.clone()}, true
}

// ArrayIter is ArrayIter in hayro-syntax src/object/array.rs. It iterates
// over the items of an array, without resolving references.
type ArrayIter struct {
	reader *Reader
	ctx    *ReaderContext
}

// Next is Iterator::next of ArrayIter in hayro-syntax src/object/array.rs.
func (it *ArrayIter) Next() (MaybeRef[Object], bool) {
	it.reader.SkipWhiteSpacesAndComments()
	if it.reader.AtEnd() {
		return MaybeRef[Object]{}, false
	}
	// A skip accepts some items that a read cannot read.
	return it.reader.ReadWithContext[MaybeRef[Object]](it.ctx)
}

// All returns the items that Next returns, until it fails.
func (it *ArrayIter) All() iter.Seq[MaybeRef[Object]] {
	return func(yield func(MaybeRef[Object]) bool) {
		for {
			v, ok := it.Next()
			if !ok || !yield(v) {
				return
			}
		}
	}
}

// ResolvedArrayIter is ResolvedArrayIter in hayro-syntax
// src/object/array.rs. It iterates over the items of an array as a T, and
// it resolves references.
type ResolvedArrayIter[T any] struct {
	flex *FlexArrayIter
}

// Next is Iterator::next of ResolvedArrayIter in hayro-syntax
// src/object/array.rs.
func (it *ResolvedArrayIter[T]) Next() (T, bool) {
	return it.flex.Next[T]()
}

// All returns the items that Next returns, until it fails.
func (it *ResolvedArrayIter[T]) All() iter.Seq[T] {
	return func(yield func(T) bool) {
		for {
			v, ok := it.Next()
			if !ok || !yield(v) {
				return
			}
		}
	}
}

// Collect returns the items that Next returns, until it fails.
func (it *ResolvedArrayIter[T]) Collect() []T {
	var out []T
	for v := range it.All() {
		out = append(out, v)
	}
	return out
}

// FlexArrayIter is FlexArrayIter in hayro-syntax src/object/array.rs. It
// reads each item of an array as a type that the caller chooses.
type FlexArrayIter struct {
	reader *Reader
	ctx    *ReaderContext
}

// Next is FlexArrayIter::next in hayro-syntax src/object/array.rs. It reads
// the next item as a T, and it resolves a reference.
func (it *FlexArrayIter) Next[T any]() (T, bool) {
	var zero T
	it.reader.SkipWhiteSpacesAndComments()
	if it.reader.AtEnd() {
		return zero, false
	}
	m, ok := it.reader.ReadWithContext[MaybeRef[T]](it.ctx)
	if !ok {
		return zero, false
	}
	if m.IsRef {
		return it.ctx.XRef().getWith[T](m.Ref.ID(), it.ctx)
	}
	return m.Value, true
}

// arrayIntoFixed is the TryFrom<Array> implementation of [T; C] in
// hayro-syntax src/object/array.rs. It fills dst with exactly len(dst)
// items.
func arrayIntoFixed[T any](a Array, dst []T) bool {
	it := a.Iter[T]()
	for i := range dst {
		v, ok := it.Next()
		if !ok {
			return false
		}
		dst[i] = v
	}
	if _, ok := it.Next(); ok {
		return false
	}
	return true
}

// castFixedArray is the TryFrom<Object> implementation of [T; C] in
// hayro-syntax src/object/array.rs.
func castFixedArray[T any](o Object, dst []T) bool {
	a, ok := o.(Array)
	return ok && arrayIntoFixed(a, dst)
}

// readFixedArray is the Readable implementation of [T; C] in hayro-syntax
// src/object/array.rs.
func readFixedArray[T any](r *Reader, ctx *ReaderContext, dst []T) bool {
	a, ok := readArray(r, ctx)
	return ok && arrayIntoFixed(a, dst)
}

// castVec is the TryFrom<Object> implementation of Vec<T> in hayro-syntax
// src/object/array.rs. It collects the items up to the first one that is
// not a T.
func castVec[T any](o Object) ([]T, bool) {
	a, ok := o.(Array)
	if !ok {
		return nil, false
	}
	return a.Iter[T]().Collect(), true
}

// readVec is the Readable implementation of Vec<T> in hayro-syntax
// src/object/array.rs.
func readVec[T any](r *Reader, ctx *ReaderContext) ([]T, bool) {
	a, ok := readArray(r, ctx)
	if !ok {
		return nil, false
	}
	return a.Iter[T]().Collect(), true
}
