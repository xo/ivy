// Copyright (c) The Hayro Authors
//
// Use of this source code is governed by the Apache-2.0 or MIT license that
// can be found in the LICENSE-APACHE and LICENSE-MIT files.

// This file ports hayro-syntax src/reader.rs.

package syntax

import (
	"fmt"
	"slices"
)

// maxParentChain bounds the parent chain of a ReaderContext.
//
// Port: upstream does not bound the chain. Each read of an object through a
// reference copies the chain, so a long chain of references costs time and
// memory that grow with the square of its length. A read past this depth
// fails.
const maxParentChain = 4096

// Read is ReaderExt::read in hayro-syntax src/reader.rs. It reads a T, and
// it moves the reader back to its offset when the read fails.
//
// T is one of the types that upstream reads: Object, Null, bool, Bool,
// Number, String, Name, Dict, Array, Stream, ObjRef, ObjectIdentifier, Rect,
// Comment, Operator, int32, int64, uint32, uint16, int, uint8, float32,
// float64, MaybeRef, IndirectObject, Tuple2 to Tuple5, the fixed arrays and
// the slices that readInto lists. The type int stands for usize of upstream,
// so it does not take a negative number. A call with another type panics,
// because it is a fault of the program.
func (r *Reader) Read[T any](ctx *ReaderContext) (T, bool) {
	old := r.offset
	v, ok := readValue[T](r, ctx)
	if !ok {
		r.offset = old
	}
	return v, ok
}

// ReadWithContext is ReaderExt::read_with_context in hayro-syntax
// src/reader.rs.
func (r *Reader) ReadWithContext[T any](ctx *ReaderContext) (T, bool) {
	return r.Read[T](ctx)
}

// ReadWithoutContext is ReaderExt::read_without_context in hayro-syntax
// src/reader.rs. It reads with the dummy context of a content stream.
func (r *Reader) ReadWithoutContext[T any]() (T, bool) {
	return r.Read[T](dummyRefContext())
}

// ReadIndirect is ReaderExt::read_indirect in hayro-syntax src/reader.rs. It
// reads a T as an indirect object, so that a dictionary followed by a stream
// becomes a stream.
func (r *Reader) ReadIndirect[T any](ctx *ReaderContext) (T, bool) {
	old := r.offset
	v, ok := readIndirectValue[T](r, ctx)
	if !ok {
		r.offset = old
	}
	return v, ok
}

// Skip is ReaderExt::skip in hayro-syntax src/reader.rs. It skips a T and
// returns the bytes that it skipped.
func (r *Reader) Skip[T any](isContentStream bool) ([]byte, bool) {
	return r.SkipWithDepth[T](isContentStream, 0)
}

// SkipWithDepth is ReaderExt::skip_with_depth in hayro-syntax src/reader.rs.
func (r *Reader) SkipWithDepth[T any](isContentStream bool, depth int) ([]byte, bool) {
	old := r.offset
	var zero T
	if !skipInto(any(&zero), r, isContentStream, depth) {
		r.offset = old
		return nil, false
	}
	return r.Range(old, r.offset)
}

// SkipWhiteSpaces is ReaderExt::skip_white_spaces in hayro-syntax
// src/reader.rs.
func (r *Reader) SkipWhiteSpaces() {
	for r.offset >= 0 && r.offset < len(r.Data) && isWhiteSpaceCharacter(r.Data[r.offset]) {
		r.offset++
	}
}

// ReadWhiteSpace is ReaderExt::read_white_space in hayro-syntax
// src/reader.rs. It reads one white space character, and a carriage return
// together with the line feed after it.
func (r *Reader) ReadWhiteSpace() bool {
	b, ok := r.PeekByte()
	if !ok || !isWhiteSpaceCharacter(b) {
		return false
	}
	w, _ := r.ReadU8()
	if next, ok := r.PeekByte(); w == '\r' && ok && next == '\n' {
		r.Forward()
	}
	return true
}

// ReadEOL is ReaderExt::read_eol in hayro-syntax src/reader.rs.
func (r *Reader) ReadEOL() bool {
	b, ok := r.PeekByte()
	if !ok {
		return false
	}
	switch b {
	case '\n':
		r.Forward()
	case '\r':
		r.Forward()
		r.ForwardIf(func(b byte) bool { return b == '\n' })
	default:
		return false
	}
	return true
}

// SkipWhiteSpacesAndComments is ReaderExt::skip_white_spaces_and_comments in
// hayro-syntax src/reader.rs.
func (r *Reader) SkipWhiteSpacesAndComments() {
	for {
		b, ok := r.PeekByte()
		switch {
		case !ok:
			return
		case isWhiteSpaceCharacter(b):
			r.Forward()
		case b == '%':
			skipComment(r)
		default:
			return
		}
	}
}

// ReaderContext is ReaderContext in hayro-syntax src/reader.rs. It holds the
// context for reading PDF objects.
//
// Port: upstream shares the data of a context behind an Arc and copies it
// on a write. A Go context is a value that the reader copies before each
// change. The parent chain is copied when it grows, so that two copies never
// share a write. The field dummy marks the Dummy case of upstream, whose
// setters do nothing except the one for in_content_stream.
type ReaderContext struct {
	xref            *XRef
	dummy           bool
	inContentStream bool
	inObjectStream  bool
	hasObjNumber    bool
	objNumber       ObjectIdentifier
	parentChain     []ObjectIdentifier
	// env holds the limits of a dummy context, or nil for the defaults.
	//
	// Port: upstream has no limits. The scans of the xref table read
	// streams with a dummy context, and the limits of the document must
	// bound them too.
	env *decodeEnv
}

// newReaderContext is ReaderContext::new in hayro-syntax src/reader.rs.
func newReaderContext(xref *XRef, inContentStream bool) *ReaderContext {
	return &ReaderContext{xref: xref, inContentStream: inContentStream}
}

// DummyReaderContext is ReaderContext::dummy in hayro-syntax src/reader.rs.
// It returns a context without an xref table.
func DummyReaderContext() *ReaderContext {
	return &ReaderContext{dummy: true}
}

// dummyEnvContext returns a dummy context with the limits of env.
func dummyEnvContext(env *decodeEnv) *ReaderContext {
	return &ReaderContext{dummy: true, env: env}
}

// dummyRefContext is ReaderContext::dummy_ref in hayro-syntax src/reader.rs.
// It returns a new dummy context of a content stream.
//
// Port: upstream returns a shared static value. Go returns a new value, so
// that no caller can change a shared one.
func dummyRefContext() *ReaderContext {
	return &ReaderContext{dummy: true, inContentStream: true}
}

// clone returns a copy of the context.
func (c *ReaderContext) clone() *ReaderContext {
	n := *c
	return &n
}

// XRef is ReaderContext::xref in hayro-syntax src/reader.rs. It returns nil,
// the dummy xref table, for a dummy context.
func (c *ReaderContext) XRef() *XRef {
	if c.dummy {
		return nil
	}
	return c.xref
}

// InContentStream is ReaderContext::in_content_stream in hayro-syntax
// src/reader.rs.
func (c *ReaderContext) InContentStream() bool {
	return c.inContentStream
}

// inObjectStreamCtx is ReaderContext::in_object_stream in hayro-syntax
// src/reader.rs.
func (c *ReaderContext) inObjectStreamCtx() bool {
	return !c.dummy && c.inObjectStream
}

// objNumberCtx is ReaderContext::obj_number in hayro-syntax src/reader.rs.
func (c *ReaderContext) objNumberCtx() (ObjectIdentifier, bool) {
	if c.dummy || !c.hasObjNumber {
		return ObjectIdentifier{}, false
	}
	return c.objNumber, true
}

// setObjNumber is ReaderContext::set_obj_number in hayro-syntax
// src/reader.rs.
func (c *ReaderContext) setObjNumber(id ObjectIdentifier) {
	if c.dummy {
		return
	}
	c.objNumber = id
	c.hasObjNumber = true
}

// setInContentStream is ReaderContext::set_in_content_stream in hayro-syntax
// src/reader.rs.
func (c *ReaderContext) setInContentStream(v bool) {
	c.inContentStream = v
}

// setInObjectStream is ReaderContext::set_in_object_stream in hayro-syntax
// src/reader.rs.
func (c *ReaderContext) setInObjectStream(v bool) {
	if c.dummy {
		return
	}
	c.inObjectStream = v
}

// parentChainContains is ReaderContext::parent_chain_contains in
// hayro-syntax src/reader.rs.
//
// Port: it also reports true when the chain is full, so that a read past
// maxParentChain fails like a cycle.
func (c *ReaderContext) parentChainContains(id ObjectIdentifier) bool {
	if c.dummy {
		return false
	}
	return len(c.parentChain) >= maxParentChain || slices.Contains(c.parentChain, id)
}

// parentChainPush is ReaderContext::parent_chain_push in hayro-syntax
// src/reader.rs.
func (c *ReaderContext) parentChainPush(id ObjectIdentifier) {
	if c.dummy {
		return
	}
	c.parentChain = append(slices.Clip(c.parentChain), id)
}

// readable is Readable in hayro-syntax src/reader.rs, for a type of this
// package that has methods. The pointer to the value receives the result.
type readable interface {
	readFrom(r *Reader, ctx *ReaderContext) bool
}

// skippable is Skippable in hayro-syntax src/reader.rs, for a generic type of
// this package.
type skippable interface {
	skipFrom(r *Reader, isContentStream bool, depth int) bool
}

// castable is TryFrom<Object> in hayro-syntax src/object/mod.rs, for a type
// of this package that has methods.
type castable interface {
	castFrom(o Object) bool
}

// readValue is Readable::read in hayro-syntax src/reader.rs. It does not move
// the reader back on a failure.
func readValue[T any](r *Reader, ctx *ReaderContext) (T, bool) {
	var v T
	ok := readInto(any(&v), r, ctx)
	return v, ok
}

// readIndirectValue is Readable::read_indirect in hayro-syntax
// src/reader.rs.
func readIndirectValue[T any](r *Reader, ctx *ReaderContext) (T, bool) {
	var v T
	var ok bool
	switch p := any(&v).(type) {
	case *Object:
		*p, ok = readIndirectObject(r, ctx)
	case *Stream:
		*p, ok = readIndirectStream(r, ctx)
	default:
		ok = readInto(p, r, ctx)
	}
	return v, ok
}

// castValue is TryFrom<Object> in hayro-syntax src/object/mod.rs. It
// converts an object into a T.
func castValue[T any](o Object) (T, bool) {
	var v T
	ok := castInto(any(&v), o)
	return v, ok
}

// readInto reads the value that p points to. It dispatches to the Readable
// implementation of each type of upstream.
func readInto(p any, r *Reader, ctx *ReaderContext) bool {
	var ok bool
	switch p := p.(type) {
	case *Object:
		*p, ok = readObject(r, ctx)
	case *Null:
		ok = readNull(r)
	case *bool:
		var b Bool
		b, ok = readBool(r)
		*p = bool(b)
	case *Bool:
		*p, ok = readBool(r)
	case *Number:
		*p, ok = readNumber(r)
	case *Name:
		*p, ok = readName(r)
	case *String:
		*p, ok = readString(r, ctx)
	case *Dict:
		*p, ok = readDict(r, ctx)
	case *Array:
		*p, ok = readArray(r, ctx)
	case *Stream:
		return false
	case *ObjRef:
		*p, ok = readObjRef(r)
	case *ObjectIdentifier:
		*p, ok = readObjectIdentifier(r)
	case *Rect:
		*p, ok = readRect(r, ctx)
	case *Comment:
		*p, ok = readComment(r)
	case *Operator:
		*p, ok = readOperator(r)
	case *int32:
		*p, ok = readInteger[int32](r, ctx)
	case *int64:
		*p, ok = readInteger[int64](r, ctx)
	case *uint32:
		*p, ok = readInteger[uint32](r, ctx)
	case *uint16:
		*p, ok = readInteger[uint16](r, ctx)
	case *uint8:
		*p, ok = readInteger[uint8](r, ctx)
	case *int:
		*p, ok = readInteger[int](r, ctx)
	case *float32:
		var n Number
		n, ok = r.ReadWithoutContext[Number]()
		*p = float32(n.AsF64())
	case *float64:
		var n Number
		n, ok = r.ReadWithoutContext[Number]()
		*p = n.AsF64()
	case *[2]float32:
		ok = readFixedArray(r, ctx, p[:])
	case *[3]float32:
		ok = readFixedArray(r, ctx, p[:])
	case *[4]float32:
		ok = readFixedArray(r, ctx, p[:])
	case *[6]float32:
		ok = readFixedArray(r, ctx, p[:])
	case *[9]float32:
		ok = readFixedArray(r, ctx, p[:])
	case *[4]float64:
		ok = readFixedArray(r, ctx, p[:])
	case *[6]float64:
		ok = readFixedArray(r, ctx, p[:])
	case *[3]uint8:
		ok = readFixedArray(r, ctx, p[:])
	case *[2]bool:
		ok = readFixedArray(r, ctx, p[:])
	case *[2]uint32:
		ok = readFixedArray(r, ctx, p[:])
	case *[]float32:
		*p, ok = readVec[float32](r, ctx)
	case *[]float64:
		*p, ok = readVec[float64](r, ctx)
	case *[]int32:
		*p, ok = readVec[int32](r, ctx)
	case *[]uint32:
		*p, ok = readVec[uint32](r, ctx)
	case *[]uint16:
		*p, ok = readVec[uint16](r, ctx)
	case *[]uint8:
		*p, ok = readVec[uint8](r, ctx)
	case *[]Number:
		*p, ok = readVec[Number](r, ctx)
	case *[]Object:
		*p, ok = readVec[Object](r, ctx)
	case *[]Name:
		*p, ok = readVec[Name](r, ctx)
	case *[]Dict:
		*p, ok = readVec[Dict](r, ctx)
	case *[]Array:
		*p, ok = readVec[Array](r, ctx)
	case *[]Tuple2[float32, float32]:
		*p, ok = readVec[Tuple2[float32, float32]](r, ctx)
	case readable:
		ok = p.readFrom(r, ctx)
	default:
		panic(fmt.Sprintf("syntax: type %T cannot be read", p))
	}
	return ok
}

// castInto converts o into the value that p points to.
func castInto(p any, o Object) bool {
	var ok bool
	switch p := p.(type) {
	case *Object:
		*p, ok = o, o != nil
	case *Null:
		_, ok = o.(Null)
	case *bool:
		var b Bool
		b, ok = o.(Bool)
		*p = bool(b)
	case *Bool:
		*p, ok = o.(Bool)
	case *Number:
		*p, ok = o.(Number)
	case *Name:
		*p, ok = o.(Name)
	case *String:
		*p, ok = o.(String)
	case *Dict:
		*p, ok = o.(Dict)
	case *Array:
		*p, ok = o.(Array)
	case *Stream:
		*p, ok = o.(Stream)
	case *int32:
		*p, ok = castInteger[int32](o)
	case *int64:
		*p, ok = castInteger[int64](o)
	case *uint32:
		*p, ok = castInteger[uint32](o)
	case *uint16:
		*p, ok = castInteger[uint16](o)
	case *uint8:
		*p, ok = castInteger[uint8](o)
	case *int:
		*p, ok = castInteger[int](o)
	case *float32:
		var n Number
		n, ok = o.(Number)
		*p = float32(n.AsF64())
	case *float64:
		var n Number
		n, ok = o.(Number)
		*p = n.AsF64()
	case *Rect:
		var a Array
		if a, ok = o.(Array); ok {
			*p, ok = rectFromArr(a)
		}
	case *[2]float32:
		ok = castFixedArray(o, p[:])
	case *[3]float32:
		ok = castFixedArray(o, p[:])
	case *[4]float32:
		ok = castFixedArray(o, p[:])
	case *[6]float32:
		ok = castFixedArray(o, p[:])
	case *[9]float32:
		ok = castFixedArray(o, p[:])
	case *[4]float64:
		ok = castFixedArray(o, p[:])
	case *[6]float64:
		ok = castFixedArray(o, p[:])
	case *[3]uint8:
		ok = castFixedArray(o, p[:])
	case *[2]bool:
		ok = castFixedArray(o, p[:])
	case *[2]uint32:
		ok = castFixedArray(o, p[:])
	case *[]float32:
		*p, ok = castVec[float32](o)
	case *[]float64:
		*p, ok = castVec[float64](o)
	case *[]int32:
		*p, ok = castVec[int32](o)
	case *[]uint32:
		*p, ok = castVec[uint32](o)
	case *[]uint16:
		*p, ok = castVec[uint16](o)
	case *[]uint8:
		*p, ok = castVec[uint8](o)
	case *[]Number:
		*p, ok = castVec[Number](o)
	case *[]Object:
		*p, ok = castVec[Object](o)
	case *[]Name:
		*p, ok = castVec[Name](o)
	case *[]Dict:
		*p, ok = castVec[Dict](o)
	case *[]Array:
		*p, ok = castVec[Array](o)
	case *[]Tuple2[float32, float32]:
		*p, ok = castVec[Tuple2[float32, float32]](o)
	case castable:
		ok = p.castFrom(o)
	default:
		panic(fmt.Sprintf("syntax: type %T cannot be cast", p))
	}
	return ok
}

// skipInto skips the type that p points to. It dispatches to the Skippable
// implementation of each type of upstream.
func skipInto(p any, r *Reader, isContentStream bool, depth int) bool {
	switch p := p.(type) {
	case *Object:
		return skipObject(r, isContentStream, depth)
	case *Null:
		return skipNull(r)
	case *bool, *Bool:
		return skipBool(r)
	case *Number, *float32, *float64:
		return skipNumber(r)
	case *Name:
		return skipName(r)
	case *String:
		return skipString(r)
	case *Dict:
		return skipDict(r, isContentStream, depth)
	case *Array:
		return skipArray(r, isContentStream, depth)
	case *Stream:
		return false
	case *ObjRef:
		return skipObjRef(r)
	case *ObjectIdentifier:
		return skipObjectIdentifier(r)
	case *Comment:
		return skipComment(r)
	case *Operator:
		return skipOperator(r)
	case *int32, *int64, *uint32, *uint16, *uint8, *int:
		return skipInteger(r)
	case skippable:
		return p.skipFrom(r, isContentStream, depth)
	default:
		panic(fmt.Sprintf("syntax: type %T cannot be skipped", p))
	}
}
