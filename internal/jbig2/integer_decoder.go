// Copyright (c) The Hayro Authors
//
// Use of this source code is governed by the Apache-2.0 or MIT license that
// can be found in the LICENSE-APACHE and LICENSE-MIT files.

package jbig2

// This file ports hayro-jbig2 src/integer_decoder.rs.

// integerDecoder is IntegerDecoder in hayro-jbig2 src/integer_decoder.rs. It
// is the arithmetic integer decoding procedure of T.88 Annex A.2, with 512
// contexts of its own.
type integerDecoder struct {
	contexts [512]arithmeticDecoderContext
}

// newIntegerDecoder is IntegerDecoder::new.
func newIntegerDecoder() *integerDecoder {
	return &integerDecoder{}
}

// decode is IntegerDecoder::decode. It returns false for the out-of-band
// value.
func (id *integerDecoder) decode(decoder *arithmeticDecoder) (int32, bool) {
	prev := uint32(1)

	s := id.decodeBit(decoder, &prev)

	// Port: upstream writes a chain of if and else if, in which each
	// condition decodes a bit. Go walks the ranges of T.88 Table A.1 in a
	// loop, with the same order of decoding.
	v, found := uint32(0), false
	for _, rng := range integerRanges {
		if id.decodeBit(decoder, &prev) == 0 {
			v = id.decodeNBits(decoder, &prev, rng.bits) + rng.offset
			found = true
			break
		}
	}
	if !found {
		v = id.decodeNBits(decoder, &prev, 32) + 4436
	}

	switch {
	case s == 0:
		return int32(v), true
	case v > 0:
		return -int32(v), true
	}
	return 0, false
}

// integerRanges are the ranges of the values of T.88 Table A.1 before the
// last one: the number of bits of a value, and the offset of the range.
var integerRanges = [...]struct {
	bits   int
	offset uint32
}{{2, 0}, {4, 4}, {6, 20}, {8, 84}, {12, 340}}

// decodeBit is IntegerDecoder::decode_bit. It updates prev, the context of
// the next bit.
func (id *integerDecoder) decodeBit(decoder *arithmeticDecoder, prev *uint32) uint32 {
	d := decoder.readBit(&id.contexts[*prev&0x1FF])

	if *prev < 256 {
		*prev = *prev<<1 | d
	} else {
		*prev = ((*prev<<1)|d)&511 | 256
	}

	return d
}

// decodeNBits is IntegerDecoder::decode_n_bits.
func (id *integerDecoder) decodeNBits(decoder *arithmeticDecoder, prev *uint32, n int) uint32 {
	var value uint32
	for range n {
		value = value<<1 | id.decodeBit(decoder, prev)
	}
	return value
}
