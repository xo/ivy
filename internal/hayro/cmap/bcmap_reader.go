// Copyright (c) The Hayro Authors.
//
// Use of this source code is governed by the Apache-2.0 or MIT license that
// can be found in the LICENSE-APACHE and LICENSE-MIT files.

// This file ports hayro-cmap src/bcmap/reader.rs.

package cmap

// reader is Reader in hayro-cmap src/bcmap/reader.rs. It reads bits and
// numbers of the big end first from a binary CMap.
//
// Port: upstream asserts in a debug build that each read of bytes starts at
// a whole byte. Go does not test it. The parser never mixes the two reads on
// one reader.
type reader struct {
	data    []byte
	bytePos int
	bitPos  uint8
}

// newBCMapReader is Reader::new in hayro-cmap src/bcmap/reader.rs.
func newBCMapReader(data []byte) *reader {
	return &reader{data: data}
}

// readBit is Reader::read_bit in hayro-cmap src/bcmap/reader.rs. It reads the
// highest bit of a byte first.
func (r *reader) readBit() (uint8, bool) {
	if r.bytePos >= len(r.data) {
		return 0, false
	}
	bit := (r.data[r.bytePos] >> (7 - r.bitPos)) & 1

	r.bitPos++
	if r.bitPos == 8 {
		r.bitPos = 0
		r.bytePos++
	}

	return bit, true
}

// readU16 is Reader::read_u16 in hayro-cmap src/bcmap/reader.rs.
func (r *reader) readU16() (uint16, bool) {
	b, ok := r.readBytes(2)
	if !ok {
		return 0, false
	}
	return uint16(b[0])<<8 | uint16(b[1]), true
}

// readU32 is Reader::read_u32 in hayro-cmap src/bcmap/reader.rs.
func (r *reader) readU32() (uint32, bool) {
	b, ok := r.readBytes(4)
	if !ok {
		return 0, false
	}
	return uint32(b[0])<<24 | uint32(b[1])<<16 | uint32(b[2])<<8 | uint32(b[3]), true
}

// readU8 is Reader::read_u8 in hayro-cmap src/bcmap/reader.rs.
func (r *reader) readU8() (uint8, bool) {
	b, ok := r.readBytes(1)
	if !ok {
		return 0, false
	}
	return b[0], true
}

// readNBytes is Reader::read_n_bytes in hayro-cmap src/bcmap/reader.rs. It
// reads a number of one to four bytes.
func (r *reader) readNBytes(n int) (uint32, bool) {
	switch n {
	case 1:
		v, ok := r.readU8()
		return uint32(v), ok
	case 2:
		v, ok := r.readU16()
		return uint32(v), ok
	case 3:
		b, ok := r.readBytes(3)
		if !ok {
			return 0, false
		}
		return uint32(b[0])<<16 | uint32(b[1])<<8 | uint32(b[2]), true
	case 4:
		return r.readU32()
	}
	return 0, false
}

// readBytes is Reader::read_bytes in hayro-cmap src/bcmap/reader.rs.
func (r *reader) readBytes(n int) ([]byte, bool) {
	if n < 0 || r.bytePos > len(r.data) || n > len(r.data)-r.bytePos {
		return nil, false
	}
	b := r.data[r.bytePos : r.bytePos+n]
	r.bytePos += n
	return b, true
}

// position is Reader::position in hayro-cmap src/bcmap/reader.rs.
func (r *reader) position() int {
	return r.bytePos
}

// atEnd is Reader::at_end in hayro-cmap src/bcmap/reader.rs.
func (r *reader) atEnd() bool {
	return r.bytePos >= len(r.data)
}

// eatUntil is Reader::eat_until in hayro-cmap src/bcmap/reader.rs. It reads
// bytes up to the first byte that f accepts, and it does not read that byte.
func (r *reader) eatUntil(f func(byte) bool) []byte {
	start := r.bytePos
	for r.bytePos < len(r.data) && !f(r.data[r.bytePos]) {
		r.bytePos++
	}
	return r.data[start:r.bytePos]
}
