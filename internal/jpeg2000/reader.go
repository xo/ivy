// Copyright (c) The Hayro Authors
//
// Use of this source code is governed by the Apache-2.0 or MIT license that
// can be found in the LICENSE-APACHE and LICENSE-MIT files.

package jpeg2000

// This file ports hayro-jpeg2000 src/reader.rs.

import "encoding/binary"

// bitReader is BitReader in hayro-jpeg2000 src/reader.rs. A copy of the
// struct is a copy of the reader, as Clone is upstream.
//
// Port: each method that returns an Option upstream returns a value and a
// boolean.
type bitReader struct {
	data   []byte
	curPos int
}

// newBitReader is BitReader::new.
func newBitReader(data []byte) bitReader {
	return bitReader{data: data}
}

// align is BitReader::align. It moves to the start of the next byte.
func (r *bitReader) align() {
	if bit := r.bitPos(); bit%8 != 0 {
		r.curPos += 8 - bit
	}
}

// atEnd is BitReader::at_end.
func (r *bitReader) atEnd() bool {
	return r.bytePos() >= len(r.data)
}

// jumpToEnd is BitReader::jump_to_end.
func (r *bitReader) jumpToEnd() {
	r.curPos = len(r.data) * 8
}

// tail is BitReader::tail. It returns the bytes after the current byte.
func (r *bitReader) tail() ([]byte, bool) {
	pos := r.bytePos()
	if pos > len(r.data) {
		return nil, false
	}
	return r.data[pos:], true
}

// offset is BitReader::offset.
func (r *bitReader) offset() int {
	return r.bytePos()
}

// readBytes is BitReader::read_bytes.
func (r *bitReader) readBytes(n int) ([]byte, bool) {
	b, ok := r.peekBytes(n)
	if !ok {
		return nil, false
	}
	r.curPos += n * 8
	return b, true
}

// readByte is BitReader::read_byte.
func (r *bitReader) readByte() (uint8, bool) {
	b, ok := r.peekByte()
	if !ok {
		return 0, false
	}
	r.curPos += 8
	return b, true
}

// skipBytes is BitReader::skip_bytes.
func (r *bitReader) skipBytes(n int) bool {
	_, ok := r.readBytes(n)
	return ok
}

// peekBytes is BitReader::peek_bytes.
func (r *bitReader) peekBytes(n int) ([]byte, bool) {
	start := r.bytePos()
	if n < 0 || start > len(r.data) || n > len(r.data)-start {
		return nil, false
	}
	return r.data[start : start+n], true
}

// peekByte is BitReader::peek_byte.
func (r *bitReader) peekByte() (uint8, bool) {
	pos := r.bytePos()
	if pos >= len(r.data) {
		return 0, false
	}
	return r.data[pos], true
}

// readU16 is BitReader::read_u16. It reads big endian.
func (r *bitReader) readU16() (uint16, bool) {
	b, ok := r.readBytes(2)
	if !ok {
		return 0, false
	}
	return binary.BigEndian.Uint16(b), true
}

// readU32 is BitReader::read_u32. It reads big endian.
func (r *bitReader) readU32() (uint32, bool) {
	b, ok := r.readBytes(4)
	if !ok {
		return 0, false
	}
	return binary.BigEndian.Uint32(b), true
}

// readU64 is BitReader::read_u64. It reads big endian.
func (r *bitReader) readU64() (uint64, bool) {
	b, ok := r.readBytes(8)
	if !ok {
		return 0, false
	}
	return binary.BigEndian.Uint64(b), true
}

// readBit is BitReader::read_bit.
func (r *bitReader) readBit() (uint32, bool) {
	pos := r.bytePos()
	if pos >= len(r.data) {
		return 0, false
	}
	shift := 7 - r.bitPos()
	r.curPos++
	return uint32(r.data[pos]>>shift) & 1, true
}

// bytePos is BitReader::byte_pos.
func (r *bitReader) bytePos() int {
	return r.curPos / 8
}

// bitPos is BitReader::bit_pos.
func (r *bitReader) bitPos() int {
	return r.curPos % 8
}

// readBitsWithStuffing is BitReader::read_bits_with_stuffing. It reads the
// bits of a packet header, and skips the bit that the encoder stuffs after
// a byte 0xFF.
func (r *bitReader) readBitsWithStuffing(bitSize uint8) (uint32, bool) {
	var bit uint32
	for range bitSize {
		needsStuffBit := r.needsToReadStuffBit()
		b, ok := r.readBit()
		if !ok {
			return 0, false
		}
		bit = bit<<1 | b
		if needsStuffBit && !r.readStuffBit() {
			return 0, false
		}
	}
	return bit, true
}

// needsToReadStuffBit is BitReader::needs_to_read_stuff_bit.
//
// Port: upstream indexes past the data at its end, and panics. The port
// returns false there, and the next read of a bit fails.
func (r *bitReader) needsToReadStuffBit() bool {
	pos := r.bytePos()
	return r.bitPos() == 7 && pos < len(r.data) && r.data[pos] == 0xff
}

// readStuffBit is BitReader::read_stuff_bit. It fails when the stuffed bit
// is not zero.
func (r *bitReader) readStuffBit() bool {
	b, ok := r.readBit()
	return ok && b == 0
}

// peakBitsWithStuffing is BitReader::peak_bits_with_stuffing. It reads the
// bits from a copy of the reader.
func (r *bitReader) peakBitsWithStuffing(bitSize uint8) (uint32, bool) {
	c := *r
	return c.readBitsWithStuffing(bitSize)
}

// readMarker is BitReader::read_marker. It reads 0xFF and the code of the
// marker.
func (r *bitReader) readMarker() (uint8, error) {
	b, ok := r.peekByte()
	if !ok || b != 0xff {
		return 0, errMarkerInvalid
	}
	r.curPos += 8
	m, ok := r.readByte()
	if !ok {
		return 0, errMarkerInvalid
	}
	return m, nil
}

// peekMarker is BitReader::peek_marker.
func (r *bitReader) peekMarker() (uint8, bool) {
	c := *r
	m, err := c.readMarker()
	return m, err == nil
}
