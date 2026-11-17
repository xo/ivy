// Copyright (c) The Hayro Authors
//
// Use of this source code is governed by the Apache-2.0 or MIT license that
// can be found in the LICENSE-APACHE and LICENSE-MIT files.

// This file ports hayro-syntax src/object/ref.rs.

package syntax

import "strconv"

// ObjRef is ObjRef in hayro-syntax src/object/ref.rs. It is a reference to
// an object.
type ObjRef struct {
	// ObjNumber is the object number.
	ObjNumber uint32
	// GenNumber is the generation number.
	GenNumber int32
}

// NewObjRef is ObjRef::new in hayro-syntax src/object/ref.rs.
func NewObjRef(objNumber uint32, genNumber int32) ObjRef {
	return ObjRef{ObjNumber: objNumber, GenNumber: genNumber}
}

// ID converts the reference into an ObjectIdentifier, as the From
// implementation of upstream does.
func (r ObjRef) ID() ObjectIdentifier {
	return ObjectIdentifier(r)
}

// String returns the Display text of upstream.
func (r ObjRef) String() string {
	return strconv.FormatUint(uint64(r.ObjNumber), 10) + " " + strconv.FormatInt(int64(r.GenNumber), 10) + " R"
}

// GoString returns the Debug text of upstream.
func (r ObjRef) GoString() string {
	return "ObjRef { obj_number: " + strconv.FormatUint(uint64(r.ObjNumber), 10) + ", gen_number: " + strconv.FormatInt(int64(r.GenNumber), 10) + " }"
}

// skipObjRef is the Skippable implementation of ObjRef in hayro-syntax
// src/object/ref.rs.
func skipObjRef(r *Reader) bool {
	if _, ok := r.Skip[uint32](false); !ok {
		return false
	}
	r.SkipWhiteSpaces()
	if _, ok := r.Skip[int32](false); !ok {
		return false
	}
	r.SkipWhiteSpaces()
	return r.ForwardTag("R")
}

// readObjRef is the Readable implementation of ObjRef in hayro-syntax
// src/object/ref.rs.
func readObjRef(r *Reader) (ObjRef, bool) {
	objNumber, ok := r.ReadWithoutContext[uint32]()
	if !ok {
		return ObjRef{}, false
	}
	r.SkipWhiteSpaces()
	genNumber, ok := r.ReadWithoutContext[int32]()
	if !ok {
		return ObjRef{}, false
	}
	r.SkipWhiteSpaces()
	if !r.ForwardTag("R") {
		return ObjRef{}, false
	}
	return NewObjRef(objNumber, genNumber), true
}

// MaybeRef is MaybeRef in hayro-syntax src/object/ref.rs. It holds either an
// object or a reference to one.
//
// Port: the Rust enum becomes a struct. IsRef selects the case Ref, which
// Ref holds. Otherwise Value holds the case NotRef.
type MaybeRef[T any] struct {
	// Ref is the reference of the case Ref.
	Ref ObjRef
	// IsRef is true for the case Ref.
	IsRef bool
	// Value is the object of the case NotRef.
	Value T
}

// AsObjRef is MaybeRef::as_obj_ref in hayro-syntax src/object/ref.rs.
func (m MaybeRef[T]) AsObjRef() (ObjRef, bool) {
	return m.Ref, m.IsRef
}

// Resolve is MaybeRef::resolve in hayro-syntax src/object/ref.rs. It reads
// the object that a reference points to.
func (m MaybeRef[T]) Resolve(ctx *ReaderContext) (T, bool) {
	if m.IsRef {
		return ctx.XRef().getWith[T](m.Ref.ID(), ctx)
	}
	return m.Value, true
}

// String returns the Display text of upstream.
func (m MaybeRef[T]) String() string {
	if m.IsRef {
		return m.Ref.String()
	}
	return displayAny(m.Value)
}

// GoString returns the Debug text of upstream.
func (m MaybeRef[T]) GoString() string {
	if m.IsRef {
		return m.Ref.GoString()
	}
	if o, ok := any(m.Value).(Object); ok {
		return objectDebug(o)
	}
	return debugAny(m.Value)
}

// skipFrom is the Skippable implementation of MaybeRef in hayro-syntax
// src/object/ref.rs.
func (m *MaybeRef[T]) skipFrom(r *Reader, isContentStream bool, depth int) bool {
	if _, ok := r.Skip[ObjRef](isContentStream); ok {
		return true
	}
	_, ok := r.SkipWithDepth[T](isContentStream, depth)
	return ok
}

// readFrom is the Readable implementation of MaybeRef in hayro-syntax
// src/object/ref.rs.
func (m *MaybeRef[T]) readFrom(r *Reader, ctx *ReaderContext) bool {
	if ref, ok := r.Read[ObjRef](ctx); ok {
		*m = MaybeRef[T]{Ref: ref, IsRef: true}
		return true
	}
	v, ok := r.Read[T](ctx)
	if !ok {
		return false
	}
	*m = MaybeRef[T]{Value: v}
	return true
}
