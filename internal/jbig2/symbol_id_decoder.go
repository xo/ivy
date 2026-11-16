// Copyright (c) The Hayro Authors
//
// Use of this source code is governed by the Apache-2.0 or MIT license that
// can be found in the LICENSE-APACHE and LICENSE-MIT files.

package jbig2

// This file ports hayro-jbig2 src/symbol_id_decoder.rs.

// symbolIDDecoder is SymbolIdDecoder in hayro-jbig2 src/symbol_id_decoder.rs.
// It is the procedure IAID of T.88 Annex A.3.
type symbolIDDecoder struct {
	contexts []arithmeticDecoderContext
	codeLen  uint32
}

// newSymbolIDDecoder is SymbolIdDecoder::new. A caller keeps codeLen at 21
// or less, so that the contexts take 2 MiB or less.
func newSymbolIDDecoder(codeLen uint32) *symbolIDDecoder {
	return &symbolIDDecoder{
		contexts: make([]arithmeticDecoderContext, 1<<codeLen),
		codeLen:  codeLen,
	}
}

// decode is SymbolIdDecoder::decode.
func (sd *symbolIDDecoder) decode(decoder *arithmeticDecoder) uint32 {
	prev := uint32(1)

	for range sd.codeLen {
		d := decoder.readBit(&sd.contexts[prev])
		prev = prev<<1 | d
	}

	prev -= 1 << sd.codeLen
	return prev
}
