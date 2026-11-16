// Copyright (c) The Hayro Authors.
//
// Use of this source code is governed by the Apache-2.0 or MIT license that
// can be found in the LICENSE-APACHE and LICENSE-MIT files.

// This file ports hayro-ccitt src/bit_reader.rs.

package ccitt

import "encoding/binary"

// bitReader is BitReader in hayro-ccitt src/bit_reader.rs. The buffer
// holds the next bit in its most significant bit. It holds at least 32 valid
// bits, and zero bits past the end of the data.
type bitReader struct {
	data      []byte
	bitOffset int
	buffer    uint64
	limit     int
}

// newBitReader is BitReader::new in hayro-ccitt src/bit_reader.rs.
func newBitReader(data []byte) bitReader {
	r := bitReader{data: data}
	r.refill()
	return r
}

// readBit is BitReader::read_bit in hayro-ccitt src/bit_reader.rs.
func (r *bitReader) readBit() (uint32, error) {
	bit := uint32(r.buffer >> 63)
	if err := r.consume(1); err != nil {
		return 0, err
	}
	return bit, nil
}

// peekU32 is BitReader::peek_u32 in hayro-ccitt src/bit_reader.rs. It
// returns the next 32 bits.
func (r *bitReader) peekU32() uint32 {
	return uint32(r.buffer >> 32)
}

// consume is BitReader::consume in hayro-ccitt src/bit_reader.rs. It skips
// numBits bits.
func (r *bitReader) consume(numBits int) error {
	bitOffset := r.bitOffset + numBits

	if bitOffset > r.limit {
		return r.consumeSlow(bitOffset)
	}

	r.bitOffset = bitOffset
	r.buffer <<= uint(numBits)
	return nil
}

// align is BitReader::align in hayro-ccitt src/bit_reader.rs. It moves to
// the start of the next byte, unless the position is at the start of a byte.
func (r *bitReader) align() {
	if bitPos := r.bitPos(); bitPos%8 != 0 {
		// The end of the data is always at a byte boundary, so this cannot
		// fail.
		_ = r.consume(8 - bitPos)
	}
}

// atEnd is BitReader::at_end in hayro-ccitt src/bit_reader.rs.
func (r *bitReader) atEnd() bool {
	return r.bytePos() >= len(r.data)
}

// bytePos is BitReader::byte_pos in hayro-ccitt src/bit_reader.rs.
func (r *bitReader) bytePos() int {
	return r.bitOffset >> 3
}

// consumeSlow is BitReader::consume_slow in hayro-ccitt src/bit_reader.rs.
// It refills the buffer, and it returns ErrUnexpectedEOF past the end of the
// data.
func (r *bitReader) consumeSlow(bitOffset int) error {
	end := len(r.data) * 8
	r.bitOffset = min(bitOffset, end)
	r.refill()

	if bitOffset > end {
		return ErrUnexpectedEOF
	}
	return nil
}

// bitPos is BitReader::bit_pos in hayro-ccitt src/bit_reader.rs.
func (r *bitReader) bitPos() int {
	return r.bitOffset & 7
}

// refill is BitReader::refill in hayro-ccitt src/bit_reader.rs.
func (r *bitReader) refill() {
	bytePos := r.bytePos()

	var w uint64
	if bytePos+8 <= len(r.data) {
		w = binary.BigEndian.Uint64(r.data[bytePos:])
	} else {
		var b [8]byte
		if bytePos < len(r.data) {
			copy(b[:], r.data[bytePos:])
		}
		w = binary.BigEndian.Uint64(b[:])
	}

	r.buffer = w << uint(r.bitPos())
	// Refill once fewer than 32 buffered bits remain.
	r.limit = min(bytePos*8+32, len(r.data)*8)
}
