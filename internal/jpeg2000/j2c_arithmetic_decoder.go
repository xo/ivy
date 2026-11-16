// Copyright (c) The Hayro Authors
//
// Use of this source code is governed by the Apache-2.0 or MIT license that
// can be found in the LICENSE-APACHE and LICENSE-MIT files.

package jpeg2000

// This file ports hayro-jpeg2000 src/j2c/arithmetic_decoder.rs.

import "math/bits"

// arithmeticDecoder is ArithmeticDecoder in hayro-jpeg2000
// src/j2c/arithmetic_decoder.rs. It is the MQ decoder. INITDEC, BYTEIN and
// DECODE follow the T.88 Annex G variant.
type arithmeticDecoder struct {
	data        []byte
	c           uint32
	a           uint32
	basePointer uint32
	shiftCount  uint32
}

// newArithmeticDecoder is ArithmeticDecoder::new.
func newArithmeticDecoder(data []byte) arithmeticDecoder {
	d := arithmeticDecoder{data: data}
	d.initialize()
	return d
}

// readBit is ArithmeticDecoder::read_bit.
func (d *arithmeticDecoder) readBit(ctx *arithmeticDecoderContext) uint32 {
	return d.decode(ctx)
}

// initialize is ArithmeticDecoder::initialize.
func (d *arithmeticDecoder) initialize() {
	d.c = (uint32(d.currentByte()) ^ 0xff) << 16
	d.readByte()
	d.c <<= 7
	d.shiftCount -= 7
	d.a = 0x8000
}

// readByte is ArithmeticDecoder::read_byte.
func (d *arithmeticDecoder) readByte() {
	if d.currentByte() == 0xff {
		if d.nextByte() > 0x8f {
			d.shiftCount = 8
		} else {
			d.basePointer++
			d.c = d.c + 0xfe00 - uint32(d.currentByte())<<9
			d.shiftCount = 7
		}
	} else {
		d.basePointer++
		d.c = d.c + 0xff00 - uint32(d.currentByte())<<8
		d.shiftCount = 8
	}
}

// renormalize is ArithmeticDecoder::renormalize.
func (d *arithmeticDecoder) renormalize() {
	for d.a&0x8000 == 0 {
		if d.shiftCount == 0 {
			d.readByte()
		}
		shiftsNeeded := uint32(bits.LeadingZeros32(d.a)) - 16
		batch := min(shiftsNeeded, d.shiftCount)
		d.a <<= batch
		d.c <<= batch
		d.shiftCount -= batch
	}
}

// decode is ArithmeticDecoder::decode. It decodes one bit with the
// context. The functions exchange_lps and exchange_mps of upstream exist
// only for reference, and the port leaves them out.
func (d *arithmeticDecoder) decode(ctx *arithmeticDecoderContext) uint32 {
	qe := &qeTable[ctx.index()]

	d.a -= qe.qe

	if d.c>>16 < d.a && d.a&0x8000 != 0 {
		return ctx.mps()
	}

	var isLPS uint32
	if d.c>>16 >= d.a {
		isLPS = 1
	}
	lpsMask := -isLPS
	d.c -= (d.a << 16) & lpsMask

	var cond uint32
	if d.a < qe.qe {
		cond = 1
	}

	d.a = d.a&^lpsMask | qe.qe&lpsMask

	bit := ctx.mps() ^ cond ^ isLPS

	var sw uint32
	if qe.switchMPS {
		sw = 1
	}
	ctx.xorMPS((cond ^ isLPS) & sw)

	pickNLPS := -uint8(cond ^ isLPS)
	ctx.setIndex(qe.nmps ^ (qe.nmps^qe.nlps)&pickNLPS)

	d.renormalize()
	return bit
}

// currentByte is ArithmeticDecoder::current_byte. It returns 0xFF after the
// end of the data.
func (d *arithmeticDecoder) currentByte() uint8 {
	if int(d.basePointer) < len(d.data) {
		return d.data[d.basePointer]
	}
	return 0xff
}

// nextByte is ArithmeticDecoder::next_byte. It returns 0xFF after the end
// of the data.
func (d *arithmeticDecoder) nextByte() uint8 {
	if i := int(d.basePointer + 1); i < len(d.data) {
		return d.data[i]
	}
	return 0xff
}

// arithmeticDecoderContext is ArithmeticDecoderContext in hayro-jpeg2000
// src/j2c/arithmetic_decoder.rs. The low seven bits hold the index into the
// table of probabilities, and the high bit holds the more probable symbol.
type arithmeticDecoderContext uint8

// index is ArithmeticDecoderContext::index.
func (c arithmeticDecoderContext) index() uint32 {
	return uint32(c & 0x7f)
}

// mps is ArithmeticDecoderContext::mps.
func (c arithmeticDecoderContext) mps() uint32 {
	return uint32(c >> 7)
}

// setIndex is ArithmeticDecoderContext::set_index.
func (c *arithmeticDecoderContext) setIndex(index uint8) {
	*c = *c&0x80 | arithmeticDecoderContext(index)
}

// xorMPS is ArithmeticDecoderContext::xor_mps.
func (c *arithmeticDecoderContext) xorMPS(val uint32) {
	*c ^= arithmeticDecoderContext((val & 1) << 7)
}

// reset is ArithmeticDecoderContext::reset.
func (c *arithmeticDecoderContext) reset() {
	*c = 0
}

// resetWithIndex is ArithmeticDecoderContext::reset_with_index.
func (c *arithmeticDecoderContext) resetWithIndex(index uint8) {
	*c = arithmeticDecoderContext(index)
}

// qeData is QeData in hayro-jpeg2000 src/j2c/arithmetic_decoder.rs.
type qeData struct {
	qe        uint32
	nmps      uint8
	nlps      uint8
	switchMPS bool
}

// qeTable is QE_TABLE in hayro-jpeg2000 src/j2c/arithmetic_decoder.rs.
//
// Port: the index of a context has seven bits, but the table holds 47
// entries. The port pads the table to 128 entries with the last entry, so
// that no index can leave it. No decoding reaches the padding.
var qeTable = func() [128]qeData {
	t := [...]qeData{
		{0x5601, 1, 1, true},
		{0x3401, 2, 6, false},
		{0x1801, 3, 9, false},
		{0x0AC1, 4, 12, false},
		{0x0521, 5, 29, false},
		{0x0221, 38, 33, false},
		{0x5601, 7, 6, true},
		{0x5401, 8, 14, false},
		{0x4801, 9, 14, false},
		{0x3801, 10, 14, false},
		{0x3001, 11, 17, false},
		{0x2401, 12, 18, false},
		{0x1C01, 13, 20, false},
		{0x1601, 29, 21, false},
		{0x5601, 15, 14, true},
		{0x5401, 16, 14, false},
		{0x5101, 17, 15, false},
		{0x4801, 18, 16, false},
		{0x3801, 19, 17, false},
		{0x3401, 20, 18, false},
		{0x3001, 21, 19, false},
		{0x2801, 22, 19, false},
		{0x2401, 23, 20, false},
		{0x2201, 24, 21, false},
		{0x1C01, 25, 22, false},
		{0x1801, 26, 23, false},
		{0x1601, 27, 24, false},
		{0x1401, 28, 25, false},
		{0x1201, 29, 26, false},
		{0x1101, 30, 27, false},
		{0x0AC1, 31, 28, false},
		{0x09C1, 32, 29, false},
		{0x08A1, 33, 30, false},
		{0x0521, 34, 31, false},
		{0x0441, 35, 32, false},
		{0x02A1, 36, 33, false},
		{0x0221, 37, 34, false},
		{0x0141, 38, 35, false},
		{0x0111, 39, 36, false},
		{0x0085, 40, 37, false},
		{0x0049, 41, 38, false},
		{0x0025, 42, 39, false},
		{0x0015, 43, 40, false},
		{0x0009, 44, 41, false},
		{0x0005, 45, 42, false},
		{0x0001, 45, 43, false},
		{0x5601, 46, 46, false},
	}
	var out [128]qeData
	for i := range out {
		out[i] = t[min(i, len(t)-1)]
	}
	return out
}()
