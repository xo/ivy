// Copyright (c) The Hayro Authors
//
// Use of this source code is governed by the Apache-2.0 or MIT license that
// can be found in the LICENSE-APACHE and LICENSE-MIT files.

// This file ports the tests of hayro-syntax src/bit_reader.rs.

package syntax

import (
	"bytes"
	"testing"
)

// readBits reads each size in sizes and compares the results with want.
func readBits(t *testing.T, r *BitReader, sizes []uint8, want []uint32) {
	t.Helper()
	for i, size := range sizes {
		got, ok := r.Read(size)
		if !ok || got != want[i] {
			t.Fatalf("read %d of %d bits: got %b, %v; want %b", i, size, got, ok, want[i])
		}
	}
}

// repeat returns n copies of size.
func repeat(size uint8, n int) []uint8 {
	out := make([]uint8, n)
	for i := range out {
		out[i] = size
	}
	return out
}

// writeBits writes values with a writer of size bits into a buffer of n
// bytes.
func writeBits(t *testing.T, n int, size uint8, values []uint32) []byte {
	t.Helper()
	buf := make([]byte, n)
	w, ok := NewBitWriter(buf, size)
	if !ok {
		t.Fatalf("NewBitWriter(%d) failed", size)
	}
	for _, v := range values {
		if !w.Write(v) {
			t.Fatalf("write %d failed", v)
		}
	}
	return buf
}

func TestBitReader16(t *testing.T) {
	r := NewBitReader([]byte{0x01, 0x02, 0x03, 0x04, 0x05, 0x06})
	readBits(t, r, repeat(16, 3), []uint32{0x0102, 0x0304, 0x0506})
}

func TestBitWriter16(t *testing.T) {
	got := writeBits(t, 6, 16, []uint32{0x0102, 0x0304, 0x0506})
	if !bytes.Equal(got, []byte{0x01, 0x02, 0x03, 0x04, 0x05, 0x06}) {
		t.Fatalf("got %v", got)
	}
}

func TestBitReader12(t *testing.T) {
	r := NewBitReader([]byte{0b10011000, 0b00011111, 0b10101001, 0b11101001, 0b00011010})
	readBits(t, r, repeat(12, 3), []uint32{0b100110000001, 0b111110101001, 0b111010010001})
}

func TestBitReader9(t *testing.T) {
	r := NewBitReader([]byte{0b10011000, 0b00011111, 0b10101001, 0b11101001, 0b00011010})
	readBits(t, r, repeat(9, 4), []uint32{0b100110000, 0b001111110, 0b101001111, 0b010010001})
}

func TestBitWriter8(t *testing.T) {
	got := writeBits(t, 3, 8, []uint32{0x01, 0x02, 0x03})
	if !bytes.Equal(got, []byte{0x01, 0x02, 0x03}) {
		t.Fatalf("got %v", got)
	}
}

func TestBitReader8(t *testing.T) {
	r := NewBitReader([]byte{0x01, 0x02, 0x03})
	readBits(t, r, repeat(8, 3), []uint32{0x01, 0x02, 0x03})
}

func TestBitWriter4(t *testing.T) {
	got := writeBits(t, 3, 4, []uint32{0b1001, 0b1000, 0b0001, 0b1111, 0b1010, 0b1001})
	if !bytes.Equal(got, []byte{0b10011000, 0b00011111, 0b10101001}) {
		t.Fatalf("got %v", got)
	}
}

func TestBitReader4(t *testing.T) {
	r := NewBitReader([]byte{0b10011000, 0b00011111, 0b10101001})
	readBits(t, r, repeat(4, 6), []uint32{0b1001, 0b1000, 0b0001, 0b1111, 0b1010, 0b1001})
}

func TestBitWriter2(t *testing.T) {
	got := writeBits(t, 2, 2, []uint32{0b10, 0b01, 0b10, 0b00, 0b00, 0b01, 0b00, 0b00})
	if !bytes.Equal(got, []byte{0b10011000, 0b00010000}) {
		t.Fatalf("got %v", got)
	}
}

func TestBitReader2(t *testing.T) {
	r := NewBitReader([]byte{0b10011000, 0b00010000})
	readBits(t, r, repeat(2, 8), []uint32{0b10, 0b01, 0b10, 0b00, 0b00, 0b01, 0b00, 0b00})
}

func TestBitWriter1(t *testing.T) {
	got := writeBits(t, 2, 1, []uint32{1, 0, 0, 1, 1, 0, 0, 0, 0, 0, 0, 1, 0, 0, 0, 0})
	if !bytes.Equal(got, []byte{0b10011000, 0b00010000}) {
		t.Fatalf("got %v", got)
	}
}

func TestBitReader1(t *testing.T) {
	r := NewBitReader([]byte{0b10011000, 0b00010000})
	readBits(t, r, repeat(1, 16), []uint32{1, 0, 0, 1, 1, 0, 0, 0, 0, 0, 0, 1, 0, 0, 0, 0})
}

func TestBitReaderAlign(t *testing.T) {
	r := NewBitReader([]byte{0b10011000, 0b00010000})
	readBits(t, r, repeat(1, 4), []uint32{1, 0, 0, 1})
	r.Align()
	readBits(t, r, repeat(1, 8), []uint32{0, 0, 0, 1, 0, 0, 0, 0})
}

func TestBitReaderVaryingBitSizes(t *testing.T) {
	r := NewBitReader([]byte{0b10011000, 0b00011111, 0b10101001})
	readBits(t, r, []uint8{4, 1, 4, 5, 1, 2, 7}, []uint32{0b1001, 0b1, 0b0000, 0b00111, 0b1, 0b11, 0b0101001})
}

func TestBitWriterRejectsInvalidSizes(t *testing.T) {
	buf := make([]byte, 4)
	if _, ok := NewBitWriter(buf, 0); ok {
		t.Error("size 0 accepted")
	}
	if _, ok := NewBitWriter(buf, 33); ok {
		t.Error("size 33 accepted")
	}
}

func TestBitReaderRejectsInvalidSizes(t *testing.T) {
	r := NewBitReader([]byte{0xff})
	if _, ok := r.Read(0); ok {
		t.Error("size 0 accepted")
	}
	if _, ok := r.Read(33); ok {
		t.Error("size 33 accepted")
	}
}

// roundTrip writes values with bitSize bits and reads them back.
func roundTrip(t *testing.T, bitSize uint8, values []uint32) {
	t.Helper()
	totalBits := int(bitSize) * len(values)
	buf := writeBits(t, (totalBits+7)/8, bitSize, values)
	r := NewBitReader(buf)
	for _, want := range values {
		got, ok := r.Read(bitSize)
		if !ok || got != want {
			t.Fatalf("round trip of %d bits: got %d, %v; want %d", bitSize, got, ok, want)
		}
	}
}

func TestBitWriterRoundTripAllBitSizes(t *testing.T) {
	for bitSize := uint8(1); bitSize <= 32; bitSize++ {
		mask := BitMask(bitSize)
		count := 6
		if bitSize > 16 {
			count = 4
		}
		values := make([]uint32, count)
		for i := range values {
			seed := uint32(0x9E37)*uint32(i+1) ^ uint32(bitSize)*0x45
			values[i] = seed & mask
		}
		roundTrip(t, bitSize, values)
	}
}

func TestBitWriterRoundTrip12And25Bits(t *testing.T) {
	for _, bitSize := range []uint8{12, 25} {
		mask := BitMask(bitSize)
		values := make([]uint32, 5)
		for i := range values {
			values[i] = (uint32(0xABCDE)*uint32(i+1) ^ uint32(bitSize)) & mask
		}
		roundTrip(t, bitSize, values)
	}
}
