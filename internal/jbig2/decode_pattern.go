// Copyright (c) The Hayro Authors
//
// Use of this source code is governed by the Apache-2.0 or MIT license that
// can be found in the LICENSE-APACHE and LICENSE-MIT files.

package jbig2

// This file ports hayro-jbig2 src/decode/pattern.rs, the pattern dictionary
// decoding procedure of T.88 6.7.

// decodePatternDictionary is decode in hayro-jbig2 src/decode/pattern.rs. It
// decodes the collective bitmap, and it cuts the patterns from it.
func decodePatternDictionary(header *patternDictionaryHeader, scratch *decodeState) (*patternDictionary, error) {
	patternWidth := uint32(header.patternWidth)
	patternHeight := uint32(header.patternHeight)
	if header.numPatterns == ^uint32(0) {
		return nil, ErrOverflowBitmapDimension
	}
	numPatterns := header.numPatterns + 1

	collectiveWidth := uint64(numPatterns) * uint64(patternWidth)
	if collectiveWidth > uint64(^uint32(0)) {
		return nil, ErrOverflowBitmapDimension
	}

	collective, err := newBitmap(uint32(collectiveWidth), patternHeight, &scratch.budget)
	if err != nil {
		return nil, err
	}

	if header.mmr {
		if _, err := decodeBitmapMMR(collective, header.data, header.strict, scratch.cancel); err != nil {
			return nil, err
		}
	} else {
		var at [4]adaptiveTemplatePixel
		// The first AT pixel lies one pattern to the left. The width wraps in
		// an int8, as in upstream.
		at[0] = adaptiveTemplatePixel{x: -int8(patternWidth), y: 0}
		if header.template == template0 {
			at[1] = adaptiveTemplatePixel{-3, -1}
			at[2] = adaptiveTemplatePixel{2, -2}
			at[3] = adaptiveTemplatePixel{-2, -2}
		}

		decoder := newArithmeticDecoder(header.data)
		scratch.resetContexts(1 << header.template.contextBits())

		if err := decodeBitmapArithmeticCoding(collective, decoder, scratch.contexts, header.template,
			false, &at, scratch.cancel); err != nil {
			return nil, err
		}
	}

	patterns := make([]*bitmap, 0, numPatterns)

	srcStride := collective.stride

	var work workCounter
	patternWork := 1 + uint64(wordWork)*uint64(patternHeight)*uint64(divCeil(patternWidth, wordBits))

	for gray := range numPatterns {
		if err := work.add(patternWork, scratch.cancel); err != nil {
			return nil, err
		}

		startX := gray * patternWidth
		pattern, err := newBitmap(patternWidth, patternHeight, &scratch.budget)
		if err != nil {
			return nil, err
		}

		srcWordIdx := startX / wordBits
		srcBitOffset := startX % wordBits
		dstStride := pattern.stride

		for y := range patternHeight {
			srcRow := int(y*srcStride + srcWordIdx)
			dstRow := int(y * dstStride)

			for w := range int(dstStride) {
				s1 := collective.data[srcRow+w]
				var s2 word
				if i := srcRow + w + 1; i < len(collective.data) {
					s2 = collective.data[i]
				}

				wd := s1
				if srcBitOffset != 0 {
					wd = s1<<srcBitOffset | s2>>(wordBits-srcBitOffset)
				}

				pattern.data[dstRow+w] = wd
			}

			if trailing := patternWidth % wordBits; trailing != 0 {
				last := dstRow + int(dstStride) - 1
				pattern.data[last] &= ^word(0) << (wordBits - trailing)
			}
		}

		patterns = append(patterns, pattern)
	}

	return &patternDictionary{
		patterns:      patterns,
		patternWidth:  patternWidth,
		patternHeight: patternHeight,
	}, nil
}

// patternDictionary is PatternDictionary in hayro-jbig2
// src/decode/pattern.rs.
type patternDictionary struct {
	patterns      []*bitmap
	patternWidth  uint32
	patternHeight uint32
}

// patternDictionaryHeader is PatternDictionaryHeader in hayro-jbig2
// src/decode/pattern.rs.
type patternDictionaryHeader struct {
	mmr           bool
	template      template
	patternWidth  uint8
	patternHeight uint8
	numPatterns   uint32
	data          []byte
	strict        bool
}

// parsePatternDictionary is parse in hayro-jbig2 src/decode/pattern.rs.
func parsePatternDictionary(r *reader, strict bool) (*patternDictionaryHeader, error) {
	flags, ok := r.readByte()
	if !ok {
		return nil, ErrUnexpectedEOF
	}
	mmr := flags&0x01 != 0

	tmpl := templateFromByte(flags >> 1)
	patternWidth, ok := r.readNonzeroByte()
	if !ok {
		return nil, ErrUnexpectedEOF
	}
	patternHeight, ok := r.readNonzeroByte()
	if !ok {
		return nil, ErrUnexpectedEOF
	}
	numPatterns, ok := r.readU32()
	if !ok {
		return nil, ErrUnexpectedEOF
	}
	data, ok := r.tail()
	if !ok {
		return nil, ErrUnexpectedEOF
	}

	return &patternDictionaryHeader{
		mmr:           mmr,
		template:      tmpl,
		patternWidth:  patternWidth,
		patternHeight: patternHeight,
		numPatterns:   numPatterns,
		data:          data,
		strict:        strict,
	}, nil
}
