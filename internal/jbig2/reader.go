// Copyright (c) The Hayro Authors
//
// Use of this source code is governed by the Apache-2.0 or MIT license that
// can be found in the LICENSE-APACHE and LICENSE-MIT files.

package jbig2

// This file ports hayro-jbig2 src/reader.rs.
//
// Port: each method that returns an Option returns a value and a bool. The
// bool is false for None.

import "encoding/binary"

// reader is Reader in hayro-jbig2 src/reader.rs. It reads bytes and bits,
// with the first bit in the most significant bit of a byte.
type reader struct {
	data   []byte
	curPos int
}

// newReader is Reader::new.
func newReader(data []byte) reader {
	return reader{data: data}
}

// align is Reader::align. It moves to the start of the next byte, unless the
// position is at the start of a byte.
func (r *reader) align() {
	if bitPos := r.bitPos(); bitPos%8 != 0 {
		r.curPos += 8 - bitPos
	}
}

// atEnd is Reader::at_end.
func (r *reader) atEnd() bool {
	return r.bytePos() >= len(r.data)
}

// tail is Reader::tail. It returns the bytes from the current byte to the
// end.
func (r *reader) tail() ([]byte, bool) {
	pos := r.bytePos()
	if pos > len(r.data) {
		return nil, false
	}
	return r.data[pos:], true
}

// readBytes is Reader::read_bytes.
func (r *reader) readBytes(n int) ([]byte, bool) {
	b, ok := r.peekBytes(n)
	if !ok {
		return nil, false
	}
	r.curPos += n * 8
	return b, true
}

// readByte is Reader::read_byte.
func (r *reader) readByte() (byte, bool) {
	b, ok := r.peekByte()
	if !ok {
		return 0, false
	}
	r.curPos += 8
	return b, true
}

// readNonzeroByte is Reader::read_nonzero_byte. It fails for a byte of zero.
func (r *reader) readNonzeroByte() (byte, bool) {
	b, ok := r.readByte()
	if !ok || b == 0 {
		return 0, false
	}
	return b, true
}

// skipBytes is Reader::skip_bytes.
func (r *reader) skipBytes(n int) bool {
	_, ok := r.readBytes(n)
	return ok
}

// peekBytes is Reader::peek_bytes.
func (r *reader) peekBytes(n int) ([]byte, bool) {
	start := r.bytePos()
	if n < 0 || start > len(r.data) || n > len(r.data)-start {
		return nil, false
	}
	return r.data[start : start+n], true
}

// peekByte is Reader::peek_byte.
func (r *reader) peekByte() (byte, bool) {
	return r.curByte()
}

// readU16 is Reader::read_u16. It reads a big-endian value.
func (r *reader) readU16() (uint16, bool) {
	b, ok := r.readBytes(2)
	if !ok {
		return 0, false
	}
	return binary.BigEndian.Uint16(b), true
}

// readU32 is Reader::read_u32. It reads a big-endian value.
func (r *reader) readU32() (uint32, bool) {
	b, ok := r.readBytes(4)
	if !ok {
		return 0, false
	}
	return binary.BigEndian.Uint32(b), true
}

// readI32 is Reader::read_i32. It reads a big-endian value.
func (r *reader) readI32() (int32, bool) {
	v, ok := r.readU32()
	return int32(v), ok
}

// readBit is Reader::read_bit.
func (r *reader) readBit() (uint8, bool) {
	b, ok := r.curByte()
	if !ok {
		return 0, false
	}
	shift := 7 - r.bitPos()
	r.curPos++
	return (b >> shift) & 1, true
}

// readBits is Reader::read_bits. It reads count bits, with the first bit in
// the most significant position.
func (r *reader) readBits(count uint8) (uint32, bool) {
	var value uint32
	remaining := count
	for remaining > 0 {
		bitOffset := r.bitPos()
		b, ok := r.curByte()
		if !ok {
			return 0, false
		}
		available := uint8(8 - bitOffset)
		take := min(remaining, available)
		shift := available - take
		mask := uint32(1)<<take - 1
		bits := (uint32(b) >> shift) & mask
		value = value<<take | bits
		r.curPos += int(take)
		remaining -= take
	}
	return value, true
}

// bytePos is Reader::byte_pos.
func (r *reader) bytePos() int {
	return r.curPos >> 3
}

// curByte is Reader::cur_byte.
func (r *reader) curByte() (byte, bool) {
	pos := r.bytePos()
	if pos >= len(r.data) {
		return 0, false
	}
	return r.data[pos], true
}

// bitPos is Reader::bit_pos.
func (r *reader) bitPos() int {
	return r.curPos & 7
}
