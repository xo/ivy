// Copyright (c) The Hayro Authors
//
// Use of this source code is governed by the Apache-2.0 or MIT license that
// can be found in the LICENSE-APACHE and LICENSE-MIT files.

// This file ports hayro-syntax src/byte_reader.rs.

package syntax

import "encoding/binary"

// Reader is Reader in hayro-syntax src/byte_reader.rs. It reads bytes and PDF
// objects.
//
// Port: the field offset of upstream is unexported, because Go does not
// allow a field and a method with the same name. Offset returns it, and Jump
// sets it.
type Reader struct {
	// Data is the underlying data of the reader.
	Data []byte
	// offset is the current byte offset.
	offset int
}

// NewReader is Reader::new in hayro-syntax src/byte_reader.rs. It returns a
// new reader at the offset 0.
func NewReader(data []byte) *Reader {
	return &Reader{Data: data}
}

// NewReaderWith is Reader::new_with in hayro-syntax src/byte_reader.rs. It
// returns a new reader at the given offset.
func NewReaderWith(data []byte, offset int) *Reader {
	return &Reader{Data: data, offset: offset}
}

// clone returns a copy of the reader. It is the derived Clone of upstream.
func (r *Reader) clone() *Reader {
	c := *r
	return &c
}

// AtEnd is Reader::at_end in hayro-syntax src/byte_reader.rs. It reports
// whether the reader reached the end of the data.
func (r *Reader) AtEnd() bool {
	return r.offset >= len(r.Data)
}

// JumpToEnd is Reader::jump_to_end in hayro-syntax src/byte_reader.rs. It
// moves the offset to the end of the data.
func (r *Reader) JumpToEnd() {
	r.offset = len(r.Data)
}

// Jump is Reader::jump in hayro-syntax src/byte_reader.rs. It moves the
// reader to the offset.
func (r *Reader) Jump(offset int) {
	r.offset = offset
}

// Tail is Reader::tail in hayro-syntax src/byte_reader.rs. It returns the
// data from the offset to the end.
func (r *Reader) Tail() ([]byte, bool) {
	if r.offset < 0 || r.offset > len(r.Data) {
		return nil, false
	}
	return r.Data[r.offset:], true
}

// Len is Reader::len in hayro-syntax src/byte_reader.rs. It returns the
// length of the data.
func (r *Reader) Len() int {
	return len(r.Data)
}

// IsEmpty is Reader::is_empty in hayro-syntax src/byte_reader.rs.
func (r *Reader) IsEmpty() bool {
	return len(r.Data) == 0
}

// Range is Reader::range in hayro-syntax src/byte_reader.rs. It returns the
// data from start to end.
func (r *Reader) Range(start, end int) ([]byte, bool) {
	if start < 0 || start > end || end > len(r.Data) {
		return nil, false
	}
	return r.Data[start:end], true
}

// Offset is Reader::offset in hayro-syntax src/byte_reader.rs.
func (r *Reader) Offset() int {
	return r.offset
}

// ReadBytes is Reader::read_bytes in hayro-syntax src/byte_reader.rs. It
// reads n bytes and moves the offset past them.
func (r *Reader) ReadBytes(n int) ([]byte, bool) {
	v, ok := r.PeekBytes(n)
	if !ok {
		return nil, false
	}
	r.offset += n
	return v, true
}

// ReadU8 is Reader::read_byte in hayro-syntax src/byte_reader.rs.
//
// Port: go vet requires the signature of io.ByteReader for a method named
// ReadByte, so the method takes the name ReadU8.
func (r *Reader) ReadU8() (byte, bool) {
	v, ok := r.PeekByte()
	if !ok {
		return 0, false
	}
	r.offset++
	return v, true
}

// SkipBytes is Reader::skip_bytes in hayro-syntax src/byte_reader.rs.
func (r *Reader) SkipBytes(n int) bool {
	_, ok := r.ReadBytes(n)
	return ok
}

// PeekBytes is Reader::peek_bytes in hayro-syntax src/byte_reader.rs.
func (r *Reader) PeekBytes(n int) ([]byte, bool) {
	if n < 0 || r.offset < 0 || r.offset > len(r.Data) || n > len(r.Data)-r.offset {
		return nil, false
	}
	return r.Data[r.offset : r.offset+n], true
}

// PeekByte is Reader::peek_byte in hayro-syntax src/byte_reader.rs.
func (r *Reader) PeekByte() (byte, bool) {
	if r.offset < 0 || r.offset >= len(r.Data) {
		return 0, false
	}
	return r.Data[r.offset], true
}

// Eat is Reader::eat in hayro-syntax src/byte_reader.rs. It reads the next
// byte if f accepts it.
func (r *Reader) Eat(f func(byte) bool) (byte, bool) {
	v, ok := r.PeekByte()
	if !ok || !f(v) {
		return 0, false
	}
	r.Forward()
	return v, true
}

// Forward is Reader::forward in hayro-syntax src/byte_reader.rs. It moves
// the offset by one byte.
func (r *Reader) Forward() {
	r.offset++
}

// ForwardIf is Reader::forward_if in hayro-syntax src/byte_reader.rs.
func (r *Reader) ForwardIf(f func(byte) bool) bool {
	v, ok := r.PeekByte()
	if !ok || !f(v) {
		return false
	}
	r.Forward()
	return true
}

// ForwardWhile1 is Reader::forward_while_1 in hayro-syntax
// src/byte_reader.rs. It moves the offset while f accepts the bytes, and it
// fails if f does not accept the first byte.
func (r *Reader) ForwardWhile1(f func(byte) bool) bool {
	if _, ok := r.Eat(f); !ok {
		return false
	}
	r.ForwardWhile(f)
	return true
}

// ForwardTag is Reader::forward_tag in hayro-syntax src/byte_reader.rs. It
// moves the offset past tag if the next bytes are tag.
func (r *Reader) ForwardTag(tag string) bool {
	if !r.PeekTag(tag) {
		return false
	}
	r.offset += len(tag)
	return true
}

// ForwardWhile is Reader::forward_while in hayro-syntax src/byte_reader.rs.
func (r *Reader) ForwardWhile(f func(byte) bool) {
	for {
		b, ok := r.PeekByte()
		if !ok || !f(b) {
			return
		}
		r.Forward()
	}
}

// PeekTag is Reader::peek_tag in hayro-syntax src/byte_reader.rs. It reports
// whether the next bytes are tag.
func (r *Reader) PeekTag(tag string) bool {
	b, ok := r.PeekBytes(len(tag))
	return ok && string(b) == tag
}

// ReadU16 is Reader::read_u16 in hayro-syntax src/byte_reader.rs. It reads an
// integer in big endian order.
func (r *Reader) ReadU16() (uint16, bool) {
	b, ok := r.ReadBytes(2)
	if !ok {
		return 0, false
	}
	return binary.BigEndian.Uint16(b), true
}

// ReadU32 is Reader::read_u32 in hayro-syntax src/byte_reader.rs.
func (r *Reader) ReadU32() (uint32, bool) {
	b, ok := r.ReadBytes(4)
	if !ok {
		return 0, false
	}
	return binary.BigEndian.Uint32(b), true
}

// ReadU64 is Reader::read_u64 in hayro-syntax src/byte_reader.rs.
func (r *Reader) ReadU64() (uint64, bool) {
	b, ok := r.ReadBytes(8)
	if !ok {
		return 0, false
	}
	return binary.BigEndian.Uint64(b), true
}
