// Copyright (c) The Hayro Authors
//
// Use of this source code is governed by the Apache-2.0 or MIT license that
// can be found in the LICENSE-APACHE and LICENSE-MIT files.

package jbig2

// This file ports hayro-jbig2 src/decode/generic.rs, the generic region
// decoding procedure of T.88 6.2.
//
// Port: upstream writes its decoding loops as the macros decode_loop! and
// decode_default_template_fast_loop!, because generics made slower code in
// Rust. Go writes them as functions, and passes the gathering of the context
// as a function value.

import (
	"errors"

	"github.com/xo/ivy/internal/ccitt"
)

// decodeGenericRegion is decode in hayro-jbig2 src/decode/generic.rs.
func decodeGenericRegion(header *genericRegionHeader, ctx *decodeState) (regionBitmap, error) {
	info := &header.regionInfo
	bm, err := newBitmapWith(info.width, info.height, info.xLocation, info.yLocation, false, &ctx.budget)
	if err != nil {
		return regionBitmap{}, err
	}

	if err := decodeGenericRegionInto(header, bm, ctx); err != nil {
		return regionBitmap{}, err
	}

	return regionBitmap{bitmap: bm, combinationOperator: info.combinationOperator}, nil
}

// decodeGenericRegionInto is decode_into in hayro-jbig2
// src/decode/generic.rs.
func decodeGenericRegionInto(header *genericRegionHeader, bm *bitmap, ctx *decodeState) error {
	data := header.data

	if header.mmr {
		_, err := decodeBitmapMMR(bm, data, header.strict, ctx.cancel)
		return err
	}

	decoder := newArithmeticDecoder(data)
	ctx.resetContexts(1 << header.template.contextBits())

	if header.adaptiveTemplatePixels.extended {
		return decodeBitmapExtendedTemplate(bm, decoder, ctx.contexts, header.tpgdon,
			&header.adaptiveTemplatePixels.extendedPixels, ctx.cancel)
	}
	return decodeBitmapArithmeticCoding(bm, decoder, ctx.contexts, header.template, header.tpgdon,
		&header.adaptiveTemplatePixels.regular, ctx.cancel)
}

// genericRegionHeader is GenericRegionHeader in hayro-jbig2
// src/decode/generic.rs.
type genericRegionHeader struct {
	regionInfo             regionSegmentInfo
	mmr                    bool
	template               template
	tpgdon                 bool
	adaptiveTemplatePixels genericTemplatePixels
	data                   []byte
	strict                 bool
}

// genericTemplatePixels is GenericTemplatePixels in hayro-jbig2
// src/decode/generic.rs.
//
// Port: the enum of upstream is a struct. If extended is true, the variant
// is Extended and extendedPixels holds its pixels. Otherwise the variant is
// Regular and regular holds its pixels.
type genericTemplatePixels struct {
	extended       bool
	regular        [4]adaptiveTemplatePixel
	extendedPixels [12]adaptiveTemplatePixel
}

// parseGenericRegion is parse in hayro-jbig2 src/decode/generic.rs.
func parseGenericRegion(r *reader, hadUnknownLength, strict bool) (*genericRegionHeader, error) {
	regionInfo, err := parseRegionSegmentInfo(r)
	if err != nil {
		return nil, err
	}
	flags, ok := r.readByte()
	if !ok {
		return nil, ErrUnexpectedEOF
	}
	mmr := flags&0x01 != 0

	tmpl := templateFromByte(flags >> 1)
	tpgdon := flags&0x08 != 0
	extTemplate := flags&0x10 != 0 && !mmr && tmpl == template0

	var pixels genericTemplatePixels
	switch {
	case extTemplate:
		var ext [12]adaptiveTemplatePixel
		for i := range ext {
			ext[i], err = parseAdaptiveTemplatePixel(r)
			if err != nil {
				return nil, err
			}
		}

		// At their nominal locations, A1 to A12 form the same contexts as the
		// template 0 with the default AT pixels.
		nominal := true
		for i, p := range ext {
			if p != extendedTemplateNominal[i] {
				nominal = false
				break
			}
		}

		if nominal {
			pixels.regular = template0Default
		} else {
			pixels.extended = true
			pixels.extendedPixels = ext
		}
	case mmr:
	default:
		pixels.regular, err = parseAdaptiveTemplatePixels(r, tmpl)
		if err != nil {
			return nil, err
		}
	}

	data, ok := r.tail()
	if !ok {
		return nil, ErrUnexpectedEOF
	}

	if hadUnknownLength {
		// The segment parser already made sure that the data holds the row
		// count.
		if len(data) < 4 {
			return nil, ErrUnexpectedEOF
		}
		head, tail := data[:len(data)-4], data[len(data)-4:]
		rowCount := uint32(tail[0])<<24 | uint32(tail[1])<<16 | uint32(tail[2])<<8 | uint32(tail[3])

		if rowCount > regionInfo.height {
			return nil, ErrInvalidDimension
		}

		regionInfo.height = rowCount
		data = head
	}

	return &genericRegionHeader{
		regionInfo:             regionInfo,
		mmr:                    mmr,
		template:               tmpl,
		tpgdon:                 tpgdon,
		adaptiveTemplatePixels: pixels,
		data:                   data,
		strict:                 strict,
	}, nil
}

// parseAdaptiveTemplatePixels is parse_adaptive_template_pixels in
// hayro-jbig2 src/decode/generic.rs.
func parseAdaptiveTemplatePixels(r *reader, tmpl template) ([4]adaptiveTemplatePixel, error) {
	var pixels [4]adaptiveTemplatePixel
	for i := range tmpl.adaptiveTemplatePixels() {
		p, err := parseAdaptiveTemplatePixel(r)
		if err != nil {
			return pixels, err
		}
		pixels[i] = p
	}
	return pixels, nil
}

// parseAdaptiveTemplatePixel is parse_adaptive_template_pixel in
// hayro-jbig2 src/decode/generic.rs. An AT pixel must lie before the
// current pixel.
func parseAdaptiveTemplatePixel(r *reader) (adaptiveTemplatePixel, error) {
	x, ok1 := r.readByte()
	y, ok2 := r.readByte()
	if !ok1 || !ok2 {
		return adaptiveTemplatePixel{}, ErrUnexpectedEOF
	}
	p := adaptiveTemplatePixel{x: int8(x), y: int8(y)}

	if p.y > 0 || (p.y == 0 && p.x >= 0) {
		return adaptiveTemplatePixel{}, ErrInvalidATPixel
	}

	return p, nil
}

// template0Default is TEMPLATE0_DEFAULT, the default AT pixels of the
// template 0.
var template0Default = [4]adaptiveTemplatePixel{{3, -1}, {-3, -1}, {2, -2}, {-2, -2}}

// extendedTemplateNominal is EXTENDED_TEMPLATE_NOMINAL, the nominal
// locations of A1 to A12 of the extended template.
var extendedTemplateNominal = [12]adaptiveTemplatePixel{
	{-2, 0},
	{0, -2},
	{-2, -1},
	{-1, -2},
	{1, -2},
	{2, -1},
	{-3, 0},
	{-4, 0},
	{2, -2},
	{3, -1},
	{-2, -2},
	{-3, -1},
}

// hasDefaultATPixels is has_default_at_pixels in hayro-jbig2
// src/decode/generic.rs.
func hasDefaultATPixels(tmpl template, at *[4]adaptiveTemplatePixel) bool {
	switch tmpl {
	case template0:
		return *at == template0Default
	case template1:
		return at[0] == adaptiveTemplatePixel{3, -1}
	}
	return at[0] == adaptiveTemplatePixel{2, -1}
}

// decodeBitmapMMR is decode_bitmap_mmr in hayro-jbig2 src/decode/generic.rs.
// It returns the number of bytes that the MMR data takes.
//
// It calls the port of hayro-ccitt in internal/ccitt (D17).
func decodeBitmapMMR(bm *bitmap, data []byte, strict bool, c cancel) (int, error) {
	if bm.height == 0 {
		return 0, nil
	}

	dec := &mmrBitmapDecoder{bitmap: bm}
	settings := ccitt.DecodeSettings{
		Columns: bm.width,
		Rows:    bm.height,
		// EOFB is optional when the length of the data is known.
		EndOfBlock:         true,
		EndOfLine:          false,
		RowsAreByteAligned: false,
		Encoding:           ccitt.Group4,
		Strict:             strict,
		Cancel:             c.handle,
	}

	n, err := ccitt.Decode(data, dec, &settings)

	if errors.Is(err, ccitt.ErrCancelled) {
		return 0, ErrCancelled
	}

	if strict && (err != nil || dec.y < bm.height) {
		return 0, ErrInvalidMMRData
	}

	if err != nil {
		n = len(data)
	}
	return n, nil
}

// mmrBitmapDecoder is BitmapDecoder in decode_bitmap_mmr in hayro-jbig2
// src/decode/generic.rs. It writes the rows of the CCITT decoder into a
// bitmap.
type mmrBitmapDecoder struct {
	bitmap *bitmap
	y      uint32
}

// PushRow is the implementation of hayro_ccitt::Decoder for BitmapDecoder.
// The changes alternate from white to black and from black to white, so each
// pair of changes is a span of black pixels.
func (d *mmrBitmapDecoder) PushRow(changes []uint32) {
	bm := d.bitmap
	// Port: upstream never pushes a row past the height. The check keeps a
	// fault of the CCITT decoder from a panic.
	if d.y >= bm.height {
		return
	}
	stride := int(bm.stride)
	rowStart := int(d.y) * stride
	row := bm.data[rowStart : rowStart+stride]

	for i := 0; i < len(changes); i += 2 {
		start := changes[i]
		end := bm.width
		if i+1 < len(changes) {
			end = changes[i+1]
		}
		if start >= end || end > bm.width {
			continue
		}
		firstWord := int(start / wordBits)
		lastWord := int((end - 1) / wordBits)
		firstMask := ^word(0) >> (start % wordBits)
		endBit := end % wordBits
		lastMask := ^word(0)
		if endBit != 0 {
			lastMask = ^word(0) << (wordBits - endBit)
		}

		if firstWord == lastWord {
			row[firstWord] |= firstMask & lastMask
		} else {
			row[firstWord] |= firstMask
			fillShort(row[firstWord+1:lastWord], ^word(0))
			row[lastWord] |= lastMask
		}
	}

	d.y++
}

// fillShort is fill_short in hayro-jbig2 src/decode/generic.rs.
//
// Port: upstream uses stores of a fixed size for short spans, which beat a
// call of memset in Rust. Go fills the slice with a loop.
func fillShort(s []word, value word) {
	for i := range s {
		s[i] = value
	}
}

// gatherFunc is the type of the $gather argument of decode_loop! in
// hayro-jbig2 src/decode/generic.rs.
type gatherFunc func(g *contextGatherer, bm *bitmap, x uint32) uint16

// decodeLoop is the macro decode_loop! in hayro-jbig2 src/decode/generic.rs.
func decodeLoop(bm *bitmap, decoder *arithmeticDecoder, contexts []arithmeticDecoderContext,
	g *contextGatherer, tpgdon bool, sltpContext uint16, gather gatherFunc, c cancel) error {
	width := bm.width
	height := bm.height

	ltp := false
	checkMask := rowMask(width)

	for y := range height {
		if decoder.isExhausted() {
			return ErrUnexpectedEOF
		}

		if y&checkMask == checkMask {
			if err := c.check(); err != nil {
				return err
			}
		}

		if tpgdon {
			sltp := decoder.readBit(&contexts[sltpContext])
			ltp = ltp != (sltp != 0)
		}

		if ltp {
			if y > 0 {
				stride := int(bm.stride)
				src := (int(y) - 1) * stride
				copy(bm.data[int(y)*stride:], bm.data[src:src+stride])
			}
			continue
		}

		g.startRow(bm, y)

		for x := range width {
			g.maybeReloadBuffers(bm, x)
			contextBits := gather(g, bm, x)
			pixel := decoder.readBit(&contexts[contextBits])
			value := uint8(pixel)

			if value != 0 {
				bm.setPixel(x, y, value)
				g.updateCurrentRow(x, value)
			}
		}
	}

	return nil
}

// getRowByte is Bitmap::get_row_byte in hayro-jbig2 src/decode/generic.rs.
func (b *bitmap) getRowByte(row uint32, byteIdx int) uint32 {
	wordIdx := byteIdx / int(wordBytes)
	shift := (int(wordBytes) - 1 - byteIdx%int(wordBytes)) * 8
	return (b.data[int(row)*int(b.stride)+wordIdx] >> shift) & 0xff
}

// setRowByte is Bitmap::set_row_byte in hayro-jbig2 src/decode/generic.rs.
func (b *bitmap) setRowByte(row uint32, byteIdx int, value uint8) {
	wordIdx := byteIdx / int(wordBytes)
	shift := (int(wordBytes) - 1 - byteIdx%int(wordBytes)) * 8
	idx := int(row)*int(b.stride) + wordIdx
	mask := word(0xff) << shift
	b.data[idx] = (b.data[idx] &^ mask) | word(value)<<shift
}

// decodeDefaultTemplateFastLoop is the macro
// decode_default_template_fast_loop! in hayro-jbig2 src/decode/generic.rs.
// It decodes a template with its default AT pixels a byte at a time.
func decodeDefaultTemplateFastLoop(bm *bitmap, decoder *arithmeticDecoder, contexts []arithmeticDecoderContext,
	tpgdon bool, sltpContext uint16, params *defaultTemplateFastParams, c cancel) error {
	const byteBits = 8
	stride := int(bm.stride)
	ltp := false
	checkMask := rowMask(bm.width)

	fullBytes := int((bm.width - 1) / byteBits)
	lastByteBits := ((bm.width - 1) & (byteBits - 1)) + 1

	for y := range bm.height {
		if decoder.isExhausted() {
			return ErrUnexpectedEOF
		}

		if y&checkMask == checkMask {
			if err := c.check(); err != nil {
				return err
			}
		}

		if tpgdon {
			sltp := decoder.readBit(&contexts[sltpContext])
			ltp = ltp != (sltp != 0)
		}

		if ltp {
			if y > 0 {
				src := (int(y) - 1) * stride
				copy(bm.data[int(y)*stride:], bm.data[src:src+stride])
			}
			continue
		}

		hasPrev2 := y >= 2
		hasPrev1 := y >= 1
		var valPrev2, valPrev1 uint32
		if hasPrev2 {
			valPrev2 = bm.getRowByte(y-2, 0) << params.prev2InitialShift
		}
		if hasPrev1 {
			valPrev1 = bm.getRowByte(y-1, 0)
		}
		context := (valPrev2 & params.prev2InitialMask) |
			((valPrev1 >> params.prev1Shift) & params.prev1InitialMask)

		for byteIdx := range fullBytes {
			if hasPrev2 {
				valPrev2 = valPrev2<<byteBits | bm.getRowByte(y-2, byteIdx+1)<<params.prev2InitialShift
			} else {
				valPrev2 = 0
			}
			if hasPrev1 {
				valPrev1 = valPrev1<<byteBits | bm.getRowByte(y-1, byteIdx+1)
			} else {
				valPrev1 = 0
			}

			// A uniform context and 8 MPS decodes without renormalization give
			// 8 pixels of that color.
			prev2Mask := params.prev2NextMask * 0xff
			prev1Mask := params.prev1NextMask * 0xff
			incomingPrev2 := valPrev2 & prev2Mask
			incomingPrev1 := (valPrev1 >> params.prev1Shift) & prev1Mask
			color := uint32(2)
			switch {
			case context == 0 && incomingPrev2 == 0 && incomingPrev1 == 0:
				color = 0
			case context == params.fullContext && incomingPrev2 == prev2Mask && incomingPrev1 == prev1Mask:
				color = 1
			}

			if color != 2 {
				state := contexts[context]
				if state.mps() == color && decoder.decodeMPSRun(state, byteBits) {
					var b uint8
					if color == 1 {
						b = 0xff
					}
					bm.setRowByte(y, byteIdx, b)
					continue
				}
			}

			var b uint8
			for bit := uint32(byteBits - 1); ; bit-- {
				pixel := decoder.readBit(&contexts[context])
				b |= uint8(pixel) << bit
				context = ((context & params.shiftMask) << 1) |
					pixel |
					((valPrev2 >> bit) & params.prev2NextMask) |
					((valPrev1 >> (bit + params.prev1Shift)) & params.prev1NextMask)
				if bit == 0 {
					break
				}
			}

			bm.setRowByte(y, byteIdx, b)
		}

		valPrev2 <<= byteBits
		valPrev1 <<= byteBits
		var b uint8
		for k := range lastByteBits {
			pixel := decoder.readBit(&contexts[context])
			b |= uint8(pixel) << (8 - 1 - k)
			context = ((context & params.shiftMask) << 1) |
				pixel |
				((valPrev2 >> (8 - 1 - k)) & params.prev2NextMask) |
				((valPrev1 >> (8 - 1 + params.prev1Shift - k)) & params.prev1NextMask)
		}
		bm.setRowByte(y, fullBytes, b)
	}

	return nil
}

// decodeBitmapArithmeticCoding is decode_bitmap_arithmetic_coding in
// hayro-jbig2 src/decode/generic.rs.
func decodeBitmapArithmeticCoding(bm *bitmap, decoder *arithmeticDecoder, contexts []arithmeticDecoderContext,
	tmpl template, tpgdon bool, at *[4]adaptiveTemplatePixel, c cancel) error {
	if bm.width == 0 || bm.height == 0 {
		return nil
	}

	g := newContextGatherer(tmpl, at)
	sltp := sltpContext(tmpl)

	if g.useDefaultAT {
		var params *defaultTemplateFastParams
		switch tmpl {
		case template0:
			params = &defaultTemplate0FastParams
		case template1:
			params = &defaultTemplate1FastParams
		case template2:
			params = &defaultTemplate2FastParams
		default:
			params = &defaultTemplate3FastParams
		}
		return decodeDefaultTemplateFastLoop(bm, decoder, contexts, tpgdon, sltp, params, c)
	}

	var gather gatherFunc
	switch tmpl {
	case template0:
		gather = (*contextGatherer).gatherTemplate0Custom
	case template1:
		gather = (*contextGatherer).gatherTemplate1Custom
	case template2:
		gather = (*contextGatherer).gatherTemplate2Custom
	default:
		gather = (*contextGatherer).gatherTemplate3Custom
	}
	return decodeLoop(bm, decoder, contexts, g, tpgdon, sltp, gather, c)
}

// sltpContext is sltp_context in hayro-jbig2 src/decode/generic.rs, the
// context of the bit SLTP of typical prediction (T.88 6.2.5.7).
func sltpContext(tmpl template) uint16 {
	switch tmpl {
	case template0:
		return 0b1001101100100101
	case template1:
		return 0b0011110010101
	case template2:
		return 0b0011100101
	}
	return 0b0110010101
}

// extendedTemplateBits is EXTENDED_TEMPLATE_BITS, the bits of the context of
// the template 0 for the nominal A1 to A12, so that the SLTP context
// matches.
var extendedTemplateBits = [12]uint32{1, 13, 9, 14, 12, 5, 2, 3, 11, 4, 15, 10}

// decodeBitmapExtendedTemplate is decode_bitmap_extended_template in
// hayro-jbig2 src/decode/generic.rs.
func decodeBitmapExtendedTemplate(bm *bitmap, decoder *arithmeticDecoder, contexts []arithmeticDecoderContext,
	tpgdon bool, at *[12]adaptiveTemplatePixel, c cancel) error {
	defaultATPixels := template0Default
	g := newContextGatherer(template0, &defaultATPixels)

	gather := func(g *contextGatherer, bm *bitmap, x uint32) uint16 {
		y := g.curY
		pixel := func(dx, dy int32, bit uint32) uint16 {
			value := bm.getPixel(uint32(int32(x)+dx), uint32(int32(y)+dy))
			return uint16(value) << bit
		}

		context := pixel(-1, 0, 0) | pixel(1, -1, 6) | pixel(0, -1, 7) | pixel(-1, -1, 8)
		for i, p := range at {
			context |= pixel(int32(p.x), int32(p.y), extendedTemplateBits[i])
		}

		return context
	}

	return decodeLoop(bm, decoder, contexts, g, tpgdon, sltpContext(template0), gather, c)
}

// contextGatherer is ContextGatherer in hayro-jbig2 src/decode/generic.rs.
// It keeps the pixels near the current pixel in three words, one for each of
// the two rows above and one for the current row.
type contextGatherer struct {
	template     template
	atPixels     *[4]adaptiveTemplatePixel
	useDefaultAT bool
	maxRight     uint32
	curY         uint32
	curX         uint32
	bufM2        word
	bufM1        word
	bufCur       word
	ctx          uint16
}

// newContextGatherer is ContextGatherer::new.
func newContextGatherer(tmpl template, at *[4]adaptiveTemplatePixel) *contextGatherer {
	useDefaultAT := hasDefaultATPixels(tmpl, at)
	var maxRight uint32
	switch tmpl {
	case template0, template1:
		maxRight = 2
		if useDefaultAT {
			maxRight = 3
		}
	default:
		maxRight = 1
		if useDefaultAT {
			maxRight = 2
		}
	}
	return &contextGatherer{
		template:     tmpl,
		atPixels:     at,
		useDefaultAT: useDefaultAT,
		maxRight:     maxRight,
	}
}

// The masks of the context bits that move one pixel to the left.
const (
	// shiftMaskT0 is ContextGatherer::SHIFT_MASK_T0.
	shiftMaskT0 uint16 = 0b0110_0011_1100_1110
	// shiftMaskT1 is ContextGatherer::SHIFT_MASK_T1.
	shiftMaskT1 uint16 = 0b0001_1101_1110_0110
	// shiftMaskT2 is ContextGatherer::SHIFT_MASK_T2.
	shiftMaskT2 uint16 = 0b0000_0011_0111_0010
	// shiftMaskT3 is ContextGatherer::SHIFT_MASK_T3.
	shiftMaskT3 uint16 = 0b0000_0011_1100_1110

	// shiftMaskT0Default is ContextGatherer::SHIFT_MASK_T0_DEFAULT.
	shiftMaskT0Default uint16 = 0b1111_0111_1110_1110
	// shiftMaskT1Default is ContextGatherer::SHIFT_MASK_T1_DEFAULT.
	shiftMaskT1Default uint16 = 0b0001_1101_1111_0110
	// shiftMaskT2Default is ContextGatherer::SHIFT_MASK_T2_DEFAULT.
	shiftMaskT2Default uint16 = 0b0000_0011_0111_1010
	// shiftMaskT3Default is ContextGatherer::SHIFT_MASK_T3_DEFAULT.
	shiftMaskT3Default uint16 = 0b0000_0011_1110_1110
)

// startRow is ContextGatherer::start_row.
func (g *contextGatherer) startRow(bm *bitmap, y uint32) {
	g.curY = y
	g.curX = 0

	g.bufM2 = 0
	if y >= 2 {
		g.bufM2 = loadWord(bm, y-2, 0)
	}
	g.bufM1 = 0
	if y >= 1 {
		g.bufM1 = loadWord(bm, y-1, 0)
	}
	g.bufCur = 0

	if g.useDefaultAT {
		g.ctx = g.initContextDefault()
	} else {
		g.ctx = g.initContextCustom()
	}
}

// initContextCustom is ContextGatherer::init_context_custom.
func (g *contextGatherer) initContextCustom() uint16 {
	switch g.template {
	case template0:
		m2 := getBufPixel(g.bufM2, 0)
		m1 := getBufPixel(g.bufM1, 0)<<1 | getBufPixel(g.bufM1, 1)
		return m2<<12 | m1<<5
	case template1:
		m2 := getBufPixel(g.bufM2, 0)<<1 | getBufPixel(g.bufM2, 1)
		m1 := getBufPixel(g.bufM1, 0)<<1 | getBufPixel(g.bufM1, 1)
		return m2<<9 | m1<<4
	case template2:
		m2 := getBufPixel(g.bufM2, 0)
		m1 := getBufPixel(g.bufM1, 0)
		return m2<<7 | m1<<3
	}
	m1 := getBufPixel(g.bufM1, 0)
	return m1 << 5
}

// initContextDefault is ContextGatherer::init_context_default.
func (g *contextGatherer) initContextDefault() uint16 {
	switch g.template {
	case template0:
		return getBufPixel(g.bufM2, 0)<<12 |
			getBufPixel(g.bufM2, 1)<<11 |
			getBufPixel(g.bufM1, 0)<<6 |
			getBufPixel(g.bufM1, 1)<<5 |
			getBufPixel(g.bufM1, 2)<<4
	case template1:
		return getBufPixel(g.bufM2, 0)<<10 |
			getBufPixel(g.bufM2, 1)<<9 |
			getBufPixel(g.bufM1, 0)<<5 |
			getBufPixel(g.bufM1, 1)<<4 |
			getBufPixel(g.bufM1, 2)<<3
	case template2:
		return getBufPixel(g.bufM2, 0)<<7 |
			getBufPixel(g.bufM1, 0)<<3 |
			getBufPixel(g.bufM1, 1)<<2
	}
	return getBufPixel(g.bufM1, 0)<<5 |
		getBufPixel(g.bufM1, 1)<<4
}

// loadWord is ContextGatherer::load_word. It loads the 32 pixels of a row
// from startX.
func loadWord(bm *bitmap, rowY, startX uint32) word {
	wordIdx := startX / wordBits

	if startX%wordBits == 0 {
		return bm.getWord(rowY, wordIdx)
	}
	bitOffset := startX % wordBits
	word1 := bm.getWord(rowY, wordIdx)
	word2 := bm.getWord(rowY, wordIdx+1)
	return word1<<bitOffset | word2>>(wordBits-bitOffset)
}

// getBufPixel is ContextGatherer::get_buf_pixel. It returns 0 for a position
// past the word.
func getBufPixel(buf word, pos uint32) uint16 {
	if pos < wordBits {
		return uint16((buf >> (wordShift - pos)) & 1)
	}
	return 0
}

// maybeReloadBuffers is ContextGatherer::maybe_reload_buffers. It moves the
// words when the pixels right of x leave them.
func (g *contextGatherer) maybeReloadBuffers(bm *bitmap, x uint32) {
	if x+g.maxRight >= g.curX+wordBits {
		newStart := x - min(x, 4)
		g.curX = newStart
		g.bufM2 = 0
		if g.curY >= 2 {
			g.bufM2 = loadWord(bm, g.curY-2, newStart)
		}
		g.bufM1 = 0
		if g.curY >= 1 {
			g.bufM1 = loadWord(bm, g.curY-1, newStart)
		}
		g.bufCur = loadWord(bm, g.curY, newStart)
	}
}

// atPixel returns the pixel at the AT pixel i of x. A negative coordinate
// wraps to a large uint32, which getPixel treats as outside the bitmap.
func (g *contextGatherer) atPixel(bm *bitmap, x uint32, i int) uint16 {
	p := g.atPixels[i]
	return uint16(bm.getPixel(uint32(int32(x)+int32(p.x)), uint32(int32(g.curY)+int32(p.y))))
}

// gatherTemplate0Custom is ContextGatherer::gather_template0_custom.
func (g *contextGatherer) gatherTemplate0Custom(bm *bitmap, x uint32) uint16 {
	bx := x - g.curX

	newPixels := getBufPixel(g.bufM2, bx+1)<<12 |
		getBufPixel(g.bufM1, bx+2)<<5 |
		getBufPixel(g.bufCur, bx-1) |
		g.atPixel(bm, x, 3)<<15 |
		g.atPixel(bm, x, 2)<<11 |
		g.atPixel(bm, x, 1)<<10 |
		g.atPixel(bm, x, 0)<<4

	g.ctx = (g.ctx<<1)&shiftMaskT0 | newPixels
	return g.ctx
}

// gatherTemplate1Custom is ContextGatherer::gather_template1_custom.
func (g *contextGatherer) gatherTemplate1Custom(bm *bitmap, x uint32) uint16 {
	bx := x - g.curX

	newPixels := getBufPixel(g.bufM2, bx+2)<<9 |
		getBufPixel(g.bufM1, bx+2)<<4 |
		getBufPixel(g.bufCur, bx-1) |
		g.atPixel(bm, x, 0)<<3

	g.ctx = (g.ctx<<1)&shiftMaskT1 | newPixels
	return g.ctx
}

// gatherTemplate2Custom is ContextGatherer::gather_template2_custom.
func (g *contextGatherer) gatherTemplate2Custom(bm *bitmap, x uint32) uint16 {
	bx := x - g.curX

	newPixels := getBufPixel(g.bufM2, bx+1)<<7 |
		getBufPixel(g.bufM1, bx+1)<<3 |
		getBufPixel(g.bufCur, bx-1) |
		g.atPixel(bm, x, 0)<<2

	g.ctx = (g.ctx<<1)&shiftMaskT2 | newPixels
	return g.ctx
}

// gatherTemplate3Custom is ContextGatherer::gather_template3_custom.
func (g *contextGatherer) gatherTemplate3Custom(bm *bitmap, x uint32) uint16 {
	bx := x - g.curX

	newPixels := getBufPixel(g.bufM1, bx+1)<<5 |
		getBufPixel(g.bufCur, bx-1) |
		g.atPixel(bm, x, 0)<<4

	g.ctx = (g.ctx<<1)&shiftMaskT3 | newPixels
	return g.ctx
}

// gatherTemplate0Default is ContextGatherer::gather_template0_default.
func (g *contextGatherer) gatherTemplate0Default(_ *bitmap, x uint32) uint16 {
	bx := x - g.curX
	newPixels := getBufPixel(g.bufM2, bx+2)<<11 |
		getBufPixel(g.bufM1, bx+3)<<4 |
		getBufPixel(g.bufCur, bx-1)

	g.ctx = (g.ctx<<1)&shiftMaskT0Default | newPixels
	return g.ctx
}

// gatherTemplate1Default is ContextGatherer::gather_template1_default.
func (g *contextGatherer) gatherTemplate1Default(_ *bitmap, x uint32) uint16 {
	bx := x - g.curX
	newPixels := getBufPixel(g.bufM2, bx+2)<<9 |
		getBufPixel(g.bufM1, bx+3)<<3 |
		getBufPixel(g.bufCur, bx-1)

	g.ctx = (g.ctx<<1)&shiftMaskT1Default | newPixels
	return g.ctx
}

// gatherTemplate2Default is ContextGatherer::gather_template2_default.
func (g *contextGatherer) gatherTemplate2Default(_ *bitmap, x uint32) uint16 {
	bx := x - g.curX
	newPixels := getBufPixel(g.bufM2, bx+1)<<7 |
		getBufPixel(g.bufM1, bx+2)<<2 |
		getBufPixel(g.bufCur, bx-1)

	g.ctx = (g.ctx<<1)&shiftMaskT2Default | newPixels
	return g.ctx
}

// gatherTemplate3Default is ContextGatherer::gather_template3_default.
func (g *contextGatherer) gatherTemplate3Default(_ *bitmap, x uint32) uint16 {
	bx := x - g.curX
	newPixels := getBufPixel(g.bufM1, bx+2)<<4 |
		getBufPixel(g.bufCur, bx-1)

	g.ctx = (g.ctx<<1)&shiftMaskT3Default | newPixels
	return g.ctx
}

// updateCurrentRow is ContextGatherer::update_current_row. It writes a
// decoded pixel into the word of the current row.
func (g *contextGatherer) updateCurrentRow(x uint32, value uint8) {
	bitPos := wordShift - (x - g.curX)
	mask := word(1) << bitPos
	g.bufCur = (g.bufCur &^ mask) | word(value)<<bitPos
}

// defaultTemplateFastParams is DefaultTemplateFastParams in hayro-jbig2
// src/decode/generic.rs.
type defaultTemplateFastParams struct {
	prev2InitialShift uint32
	prev2InitialMask  uint32
	prev1Shift        uint32
	prev1InitialMask  uint32
	shiftMask         uint32
	prev2NextMask     uint32
	prev1NextMask     uint32
	fullContext       uint32
}

// defaultTemplate0FastParams is DEFAULT_TEMPLATE0_FAST_PARAMS.
var defaultTemplate0FastParams = defaultTemplateFastParams{
	prev2InitialShift: 6,
	prev2InitialMask:  0xf800,
	prev1Shift:        0,
	prev1InitialMask:  0x07f0,
	shiftMask:         0x7bf7,
	prev2NextMask:     0x0800,
	prev1NextMask:     0x0010,
	fullContext:       0xffff,
}

// defaultTemplate1FastParams is DEFAULT_TEMPLATE1_FAST_PARAMS.
var defaultTemplate1FastParams = defaultTemplateFastParams{
	prev2InitialShift: 4,
	prev2InitialMask:  0x1e00,
	prev1Shift:        1,
	prev1InitialMask:  0x01f8,
	shiftMask:         0x0efb,
	prev2NextMask:     0x0200,
	prev1NextMask:     0x0008,
	fullContext:       0x1fff,
}

// defaultTemplate2FastParams is DEFAULT_TEMPLATE2_FAST_PARAMS.
var defaultTemplate2FastParams = defaultTemplateFastParams{
	prev2InitialShift: 1,
	prev2InitialMask:  0x0380,
	prev1Shift:        3,
	prev1InitialMask:  0x007c,
	shiftMask:         0x01bd,
	prev2NextMask:     0x0080,
	prev1NextMask:     0x0004,
	fullContext:       0x03ff,
}

// defaultTemplate3FastParams is DEFAULT_TEMPLATE3_FAST_PARAMS.
var defaultTemplate3FastParams = defaultTemplateFastParams{
	prev2InitialShift: 0,
	prev2InitialMask:  0x0000,
	prev1Shift:        1,
	prev1InitialMask:  0x03f0,
	shiftMask:         0x01f7,
	prev2NextMask:     0x0000,
	prev1NextMask:     0x0010,
	fullContext:       0x03ff,
}
