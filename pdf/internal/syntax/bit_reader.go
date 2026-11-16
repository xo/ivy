// Copyright (c) The Hayro Authors
//
// Use of this source code is governed by the Apache-2.0 or MIT license that
// can be found in the LICENSE-APACHE and LICENSE-MIT files.

// This file ports hayro-syntax src/bit_reader.rs.

package syntax

// BitReader is BitReader in hayro-syntax src/bit_reader.rs. It reads
// numbers of up to 32 bits, the high bit first.
type BitReader struct {
	// Data is the underlying bytes of the bit reader.
	Data   []byte
	curPos int
}

// NewBitReader is BitReader::new in hayro-syntax src/bit_reader.rs.
func NewBitReader(data []byte) *BitReader {
	return NewBitReaderWith(data, 0)
}

// NewBitReaderWith is BitReader::new_with in hayro-syntax
// src/bit_reader.rs. It starts at the bit offset curPos.
func NewBitReaderWith(data []byte, curPos int) *BitReader {
	return &BitReader{Data: data, curPos: curPos}
}

// Align is BitReader::align in hayro-syntax src/bit_reader.rs. It moves the
// reader to the next byte boundary.
func (r *BitReader) Align() {
	if bitPos := r.BitPos(); bitPos%8 != 0 {
		r.curPos += 8 - bitPos
	}
}

// Read is BitReader::read in hayro-syntax src/bit_reader.rs. It reads
// bitSize bits, from 1 to 32.
func (r *BitReader) Read(bitSize uint8) (uint32, bool) {
	if bitSize < 1 || bitSize > 32 {
		return 0, false
	}
	bytePos := r.BytePos()
	if bytePos >= len(r.Data) {
		return 0, false
	}
	if bitSize == 8 {
		item := uint32(r.Data[bytePos])
		r.curPos += 8
		return item, true
	}
	bitPos := r.BitPos()
	endBytePos := (bitPos + int(bitSize) - 1) / 8
	var read uint64
	for i := range 8 {
		var b byte
		if i <= endBytePos {
			if bytePos+i >= len(r.Data) {
				return 0, false
			}
			b = r.Data[bytePos+i]
		}
		read = read<<8 | uint64(b)
	}
	item := uint32(read>>(64-bitPos-int(bitSize))) & BitMask(bitSize)
	r.curPos += int(bitSize)
	return item, true
}

// Peak is BitReader::peak in hayro-syntax src/bit_reader.rs. It reads bits
// without moving the reader.
func (r *BitReader) Peak(bitSize uint8) (uint32, bool) {
	c := *r
	return c.Read(bitSize)
}

// AtEnd is BitReader::at_end in hayro-syntax src/bit_reader.rs.
func (r *BitReader) AtEnd() bool {
	return r.BytePos() >= len(r.Data)
}

// BytePos is BitReader::byte_pos in hayro-syntax src/bit_reader.rs.
func (r *BitReader) BytePos() int {
	return r.curPos / 8
}

// BitPos is BitReader::bit_pos in hayro-syntax src/bit_reader.rs.
func (r *BitReader) BitPos() int {
	return r.curPos % 8
}

// CurPos is BitReader::cur_pos in hayro-syntax src/bit_reader.rs. It returns
// the position in bits.
func (r *BitReader) CurPos() int {
	return r.curPos
}

// Tail is BitReader::tail in hayro-syntax src/bit_reader.rs. It returns the
// data from the byte of the current position.
//
// Port: upstream panics past the end. Go returns nil there.
func (r *BitReader) Tail() []byte {
	if r.BytePos() > len(r.Data) {
		return nil
	}
	return r.Data[r.BytePos():]
}

// CurByte is BitReader::cur_byte in hayro-syntax src/bit_reader.rs.
func (r *BitReader) CurByte() (byte, bool) {
	if r.BytePos() >= len(r.Data) {
		return 0, false
	}
	return r.Data[r.BytePos()], true
}

// BitMask is bit_mask in hayro-syntax src/bit_reader.rs. It returns the mask
// of the low bitSize bits.
func BitMask(bitSize uint8) uint32 {
	switch {
	case bitSize == 0:
		return 0
	case bitSize <= 31:
		return 1<<bitSize - 1
	}
	return 0xFFFFFFFF
}

// BitWriter is BitWriter in hayro-syntax src/bit_reader.rs. It writes
// numbers of a fixed size of up to 32 bits, the high bit first.
type BitWriter struct {
	data    []byte
	curPos  int
	bitSize uint8
}

// NewBitWriter is BitWriter::new in hayro-syntax src/bit_reader.rs. The size
// is from 1 to 32 bits.
func NewBitWriter(data []byte, bitSize uint8) (*BitWriter, bool) {
	if bitSize < 1 || bitSize > 32 {
		return nil, false
	}
	return &BitWriter{data: data, bitSize: bitSize}, true
}

// Align is BitWriter::align in hayro-syntax src/bit_reader.rs.
func (w *BitWriter) Align() {
	if bitPos := w.curPos % 8; bitPos != 0 {
		w.curPos += 8 - bitPos
	}
}

// CurPos is BitWriter::cur_pos in hayro-syntax src/bit_reader.rs. It returns
// the number of bits written.
func (w *BitWriter) CurPos() int {
	return w.curPos
}

// Write is BitWriter::write in hayro-syntax src/bit_reader.rs.
func (w *BitWriter) Write(val uint32) bool {
	bitSize := int(w.bitSize)
	bitsLeft := bitSize
	value := val & BitMask(w.bitSize)
	for bitsLeft > 0 {
		alreadyAdvanced := bitSize - bitsLeft
		absolutePos := w.curPos + alreadyAdvanced
		bytePos := absolutePos / 8
		bitPos := absolutePos % 8
		bitsInByte := min(8-bitPos, bitsLeft)
		shift := bitsLeft - bitsInByte
		chunkMask := BitMask(uint8(bitsInByte))
		chunk := byte((value >> shift) & chunkMask)
		shiftInByte := 8 - bitsInByte - bitPos
		if bytePos >= len(w.data) {
			return false
		}
		byteMask := byte(chunkMask) << shiftInByte
		w.data[bytePos] = w.data[bytePos]&^byteMask | (chunk<<shiftInByte)&byteMask
		bitsLeft -= bitsInByte
	}
	w.curPos += bitSize
	return true
}
