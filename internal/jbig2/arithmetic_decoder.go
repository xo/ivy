// Copyright (c) The Hayro Authors
//
// Use of this source code is governed by the Apache-2.0 or MIT license that
// can be found in the LICENSE-APACHE and LICENSE-MIT files.

package jbig2

// This file ports hayro-jbig2 src/arithmetic_decoder.rs. Upstream keeps it as
// a copy of the arithmetic decoder of hayro-jpeg2000.

import "math/bits"

// arithmeticDecoder is ArithmeticDecoder in hayro-jbig2
// src/arithmetic_decoder.rs. It is the MQ decoder of T.88 Annex E.
type arithmeticDecoder struct {
	data        []byte
	c           uint32
	a           uint32
	basePointer uint32
	shiftCount  uint32
	markersSeen uint32
}

// maxMarkers is MAX_MARKERS. An encoder can trim the trailing pairs of 0xFF
// 0x7F, so valid data can reach the end marker many times.
const maxMarkers = 65_536

// newArithmeticDecoder is ArithmeticDecoder::new.
func newArithmeticDecoder(data []byte) *arithmeticDecoder {
	d := &arithmeticDecoder{data: data}
	d.initialize()
	return d
}

// isExhausted is ArithmeticDecoder::is_exhausted. It reports that the decoder
// read past the end of its data too often.
func (d *arithmeticDecoder) isExhausted() bool {
	return d.markersSeen > maxMarkers
}

// readBit is ArithmeticDecoder::read_bit.
func (d *arithmeticDecoder) readBit(cx *arithmeticDecoderContext) uint32 {
	return d.decode(cx)
}

// initialize is ArithmeticDecoder::initialize.
func (d *arithmeticDecoder) initialize() {
	d.c = (uint32(d.currentByte()) ^ 0xff) << 16
	d.readByte()
	d.c <<= 7
	d.shiftCount -= 7
	d.a = 0x8000
}

// readByte is ArithmeticDecoder::read_byte, the procedure BYTEIN.
func (d *arithmeticDecoder) readByte() {
	if d.currentByte() == 0xff {
		b1 := d.nextByte()
		if b1 > 0x8f {
			d.shiftCount = 8
			if d.markersSeen != ^uint32(0) {
				d.markersSeen++
			}
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

// renormalize is ArithmeticDecoder::renormalize, the procedure RENORMD.
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

// decode is ArithmeticDecoder::decode, the procedure DECODE without
// branches. Upstream also keeps exchange_lps and exchange_mps for reference.
// The port leaves them out, because nothing calls them.
func (d *arithmeticDecoder) decode(cx *arithmeticDecoderContext) uint32 {
	qe := &qeTable[cx.index()]

	d.a -= qe.qe

	if (d.c>>16) < d.a && d.a&0x8000 != 0 {
		return cx.mps()
	}

	var isLPS uint32
	if (d.c >> 16) >= d.a {
		isLPS = 1
	}
	lpsMask := -isLPS
	d.c -= (d.a << 16) & lpsMask

	var cond uint32
	if d.a < qe.qe {
		cond = 1
	}

	d.a = (d.a &^ lpsMask) | (qe.qe & lpsMask)

	r := cx.mps() ^ cond ^ isLPS

	var sw uint32
	if qe.switchMPS {
		sw = 1
	}
	cx.xorMPS((cond ^ isLPS) & sw)

	pickNLPS := -uint8(cond ^ isLPS)
	cx.setIndex(qe.nmps ^ ((qe.nmps ^ qe.nlps) & pickNLPS))

	d.renormalize()

	return r
}

// decodeMPSRun is ArithmeticDecoder::decode_mps_run. It decodes count MPS at
// once if none of them renormalizes. Otherwise it decodes nothing and returns
// false.
func (d *arithmeticDecoder) decodeMPSRun(cx arithmeticDecoderContext, count uint32) bool {
	totalQe := qeTable[cx.index()].qe * count
	floor := max((d.c>>16)+1, 0x8000)

	if d.a >= floor+totalQe {
		d.a -= totalQe
		return true
	}
	return false
}

// currentByte is ArithmeticDecoder::current_byte. It returns 0xFF past the
// end of the data.
func (d *arithmeticDecoder) currentByte() byte {
	if int(d.basePointer) < len(d.data) {
		return d.data[d.basePointer]
	}
	return 0xFF
}

// nextByte is ArithmeticDecoder::next_byte. It returns 0xFF past the end of
// the data.
func (d *arithmeticDecoder) nextByte() byte {
	if i := int(d.basePointer) + 1; i < len(d.data) {
		return d.data[i]
	}
	return 0xFF
}

// arithmeticDecoderContext is ArithmeticDecoderContext in hayro-jbig2
// src/arithmetic_decoder.rs. The low 7 bits hold the index into the Qe table,
// and the high bit holds the MPS.
type arithmeticDecoderContext uint8

// index is ArithmeticDecoderContext::index.
func (cx arithmeticDecoderContext) index() uint32 {
	return uint32(cx & 0x7F)
}

// mps is ArithmeticDecoderContext::mps.
func (cx arithmeticDecoderContext) mps() uint32 {
	return uint32(cx >> 7)
}

// setIndex is ArithmeticDecoderContext::set_index.
func (cx *arithmeticDecoderContext) setIndex(index uint8) {
	*cx = (*cx & 0x80) | arithmeticDecoderContext(index)
}

// xorMPS is ArithmeticDecoderContext::xor_mps.
func (cx *arithmeticDecoderContext) xorMPS(val uint32) {
	*cx ^= arithmeticDecoderContext((val & 1) << 7)
}

// qeData is QeData in hayro-jbig2 src/arithmetic_decoder.rs. It is a row of
// Table E.1 of T.88.
type qeData struct {
	qe        uint32
	nmps      uint8
	nlps      uint8
	switchMPS bool
}

// qeTable is QE_TABLE in hayro-jbig2 src/arithmetic_decoder.rs.
var qeTable = [47]qeData{
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
