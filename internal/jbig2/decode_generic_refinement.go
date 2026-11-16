// Copyright (c) The Hayro Authors
//
// Use of this source code is governed by the Apache-2.0 or MIT license that
// can be found in the LICENSE-APACHE and LICENSE-MIT files.

package jbig2

// This file ports hayro-jbig2 src/decode/generic_refinement.rs, the generic
// refinement region decoding procedure of T.88 6.3.
//
// Port: upstream writes the decoding loop as the macro
// refinement_decode_loop!. Go writes it as a function, and passes the
// gathering of the context as a function value.

import "math"

// decodeGenericRefinementRegion is decode in hayro-jbig2
// src/decode/generic_refinement.rs.
func decodeGenericRefinementRegion(header *genericRefinementRegionHeader, reference *bitmap, ctx *decodeState) (regionBitmap, error) {
	info := &header.regionInfo
	region, err := newBitmapWith(info.width, info.height, info.xLocation, info.yLocation, false, &ctx.budget)
	if err != nil {
		return regionBitmap{}, err
	}

	if err := decodeGenericRefinementRegionInto(header, reference, region, ctx); err != nil {
		return regionBitmap{}, err
	}

	return regionBitmap{bitmap: region, combinationOperator: info.combinationOperator}, nil
}

// decodeGenericRefinementRegionInto is decode_into in hayro-jbig2
// src/decode/generic_refinement.rs.
func decodeGenericRefinementRegionInto(header *genericRefinementRegionHeader, reference, region *bitmap, ctx *decodeState) error {
	data := header.data

	if header.regionInfo.width > reference.width || header.regionInfo.height > reference.height {
		return ErrInvalidDimension
	}

	referenceDX, ok := referenceOffset(reference.xLocation, header.regionInfo.xLocation)
	if !ok {
		return ErrOverflowReferenceOffset
	}
	referenceDY, ok := referenceOffset(reference.yLocation, header.regionInfo.yLocation)
	if !ok {
		return ErrOverflowReferenceOffset
	}

	decoder := newArithmeticDecoder(data)
	ctx.resetContexts(1 << header.template.contextBits())

	return decodeRefinementBitmap(decoder, ctx.contexts, region, reference, referenceDX, referenceDY,
		header.template, header.adaptiveTemplatePixels, header.tpgron, ctx.cancel)
}

// referenceOffset is the computation of reference_dx and reference_dy in
// decode_into in hayro-jbig2 src/decode/generic_refinement.rs. It is r - h,
// when both values and the result fit in an int32.
func referenceOffset(r, h uint32) (int32, bool) {
	if r > math.MaxInt32 || h > math.MaxInt32 {
		return 0, false
	}
	// Two values in 0 to MaxInt32 always have a difference in an int32.
	return int32(r) - int32(h), true
}

// genericRefinementRegionHeader is GenericRefinementRegionHeader in
// hayro-jbig2 src/decode/generic_refinement.rs.
type genericRefinementRegionHeader struct {
	regionInfo             regionSegmentInfo
	template               refinementTemplate
	tpgron                 bool
	adaptiveTemplatePixels []adaptiveTemplatePixel
	data                   []byte
}

// parseGenericRefinementRegion is parse in hayro-jbig2
// src/decode/generic_refinement.rs.
func parseGenericRefinementRegion(r *reader) (*genericRefinementRegionHeader, error) {
	regionInfo, err := parseRegionSegmentInfo(r)
	if err != nil {
		return nil, err
	}
	flags, ok := r.readByte()
	if !ok {
		return nil, ErrUnexpectedEOF
	}
	tmpl := refinementTemplateFromByte(flags)
	tpgron := flags&0x02 != 0
	var at []adaptiveTemplatePixel
	if tmpl == refinementTemplate0 {
		at, err = parseRefinementATPixels(r)
		if err != nil {
			return nil, err
		}
	}
	data, ok := r.tail()
	if !ok {
		return nil, ErrUnexpectedEOF
	}

	return &genericRefinementRegionHeader{
		regionInfo:             regionInfo,
		template:               tmpl,
		tpgron:                 tpgron,
		adaptiveTemplatePixels: at,
		data:                   data,
	}, nil
}

// hasDefaultRefinementATPixels is has_default_at_pixels in hayro-jbig2
// src/decode/generic_refinement.rs.
func hasDefaultRefinementATPixels(at []adaptiveTemplatePixel) bool {
	return len(at) >= 2 &&
		at[0].x == -1 &&
		at[0].y == -1 &&
		at[1].x == -1 &&
		at[1].y == -1
}

// refinementContextGatherer is RefinementContextGatherer in hayro-jbig2
// src/decode/generic_refinement.rs. It keeps the pixels near the current
// pixel in words: two rows of the region, and three rows of the reference.
type refinementContextGatherer struct {
	template     refinementTemplate
	atPixels     []adaptiveTemplatePixel
	useDefaultAT bool
	referenceDX  int32
	referenceDY  int32
	regY         uint32
	regCurX      uint32
	regM1        word
	regCur       word
	refCurX      int32
	refM1        word
	refCur       word
	refP1        word
	ctx          uint16
}

// The masks of the context bits that move one pixel to the left.
const (
	// refinementShiftMaskT0Default is SHIFT_MASK_T0_DEFAULT.
	refinementShiftMaskT0Default uint16 = 0b1_1001_1011_0110
	// refinementShiftMaskT0Custom is SHIFT_MASK_T0_CUSTOM.
	refinementShiftMaskT0Custom uint16 = 0b0_1000_1011_0110
	// refinementShiftMaskT1 is SHIFT_MASK_T1.
	refinementShiftMaskT1 uint16 = 0b11_0001_1010
)

// newRefinementContextGatherer is RefinementContextGatherer::new.
func newRefinementContextGatherer(tmpl refinementTemplate, at []adaptiveTemplatePixel, referenceDX, referenceDY int32) *refinementContextGatherer {
	// With a larger offset, all reference pixels lie outside the bitmap. The
	// clamp keeps the coordinates in an int32.
	const maxOffset = 4 * int32(maxDimension)

	return &refinementContextGatherer{
		template:     tmpl,
		atPixels:     at,
		useDefaultAT: tmpl == refinementTemplate0 && hasDefaultRefinementATPixels(at),
		referenceDX:  min(max(referenceDX, -maxOffset), maxOffset),
		referenceDY:  min(max(referenceDY, -maxOffset), maxOffset),
	}
}

// startRow is RefinementContextGatherer::start_row.
func (g *refinementContextGatherer) startRow(region, reference *bitmap, y uint32) {
	g.regY = y

	g.regCurX = 0
	g.regM1 = 0
	if y >= 1 {
		g.regM1 = loadWord(region, y-1, 0)
	}
	g.regCur = 0

	g.loadReferenceWords(reference, -g.referenceDX-1)
	const rbx = 1

	switch {
	case g.template == refinementTemplate1:
		regM10 := getBufPixel(g.regM1, 0)
		refCurM1 := getBufPixel(g.refCur, rbx-1)
		refCur0 := getBufPixel(g.refCur, rbx)
		refP10 := getBufPixel(g.refP1, rbx)

		g.ctx = regM10<<7 | refCurM1<<3 | refCur0<<2 | refP10
	case g.useDefaultAT:
		regM10 := getBufPixel(g.regM1, 0)
		refM1M1 := getBufPixel(g.refM1, rbx-1)
		refM10 := getBufPixel(g.refM1, rbx)
		refCurM1 := getBufPixel(g.refCur, rbx-1)
		refCur0 := getBufPixel(g.refCur, rbx)
		refP1M1 := getBufPixel(g.refP1, rbx-1)
		refP10 := getBufPixel(g.refP1, rbx)

		g.ctx = regM10<<10 |
			refM1M1<<7 |
			refM10<<6 |
			refCurM1<<4 |
			refCur0<<3 |
			refP1M1<<1 |
			refP10
	default:
		regM10 := getBufPixel(g.regM1, 0)
		refM10 := getBufPixel(g.refM1, rbx)
		refCurM1 := getBufPixel(g.refCur, rbx-1)
		refCur0 := getBufPixel(g.refCur, rbx)
		refP1M1 := getBufPixel(g.refP1, rbx-1)
		refP10 := getBufPixel(g.refP1, rbx)

		g.ctx = regM10<<10 |
			refM10<<6 |
			refCurM1<<4 |
			refCur0<<3 |
			refP1M1<<1 |
			refP10
	}
}

// maybeReloadBuffers is RefinementContextGatherer::maybe_reload_buffers.
func (g *refinementContextGatherer) maybeReloadBuffers(region, reference *bitmap, x uint32) {
	if x+1 >= g.regCurX+wordBits {
		newStart := x - min(x, 1)
		g.regCurX = newStart
		g.regM1 = 0
		if g.regY >= 1 {
			g.regM1 = loadWord(region, g.regY-1, newStart)
		}
		g.regCur = loadWord(region, g.regY, newStart)
	}

	refX := int32(x) - g.referenceDX
	if refX+1 >= g.refCurX+int32(wordBits) {
		g.loadReferenceWords(reference, refX-1)
	}
}

// loadReferenceWords is RefinementContextGatherer::load_reference_words. It
// loads the three rows of the reference from start.
func (g *refinementContextGatherer) loadReferenceWords(reference *bitmap, start int32) {
	refY := int32(g.regY) - g.referenceDY
	load := func(row int32) word {
		// A negative row wraps to a large value, which lies outside the
		// bitmap.
		r := uint32(row)
		switch {
		case start >= 0:
			return loadWord(reference, r, uint32(start))
		case start > -int32(wordBits):
			return loadWord(reference, r, 0) >> uint32(-start)
		}
		return 0
	}

	g.refCurX = start
	g.refM1 = load(refY - 1)
	g.refCur = load(refY)
	g.refP1 = load(refY + 1)
}

// referenceBufferX is RefinementContextGatherer::reference_buffer_x.
func (g *refinementContextGatherer) referenceBufferX(x uint32) uint32 {
	return uint32(int32(x) - g.referenceDX - g.refCurX)
}

// tpgrAllSame is RefinementContextGatherer::tpgr_all_same. It reports
// whether the 3 by 3 pixels of the reference around x have one color. The
// buffers are reloaded so that the pixels left and right of the position
// always lie within them.
func (g *refinementContextGatherer) tpgrAllSame(x uint32) bool {
	rbx := g.referenceBufferX(x)

	extract3Bits := func(buf word) uint32 {
		return (buf >> (wordShift - rbx - 1)) & 0b111
	}

	m1 := extract3Bits(g.refM1)
	cur := extract3Bits(g.refCur)
	p1 := extract3Bits(g.refP1)

	return m1 == cur && cur == p1 && (cur == 0 || cur == 0b111)
}

// refCenterPixel is RefinementContextGatherer::ref_center_pixel.
func (g *refinementContextGatherer) refCenterPixel(x uint32) uint8 {
	return uint8(getBufPixel(g.refCur, g.referenceBufferX(x)))
}

// updateCurrentRow is RefinementContextGatherer::update_current_row.
func (g *refinementContextGatherer) updateCurrentRow(x uint32, value uint8) {
	bitPos := wordShift - (x - g.regCurX)
	mask := word(1) << bitPos
	g.regCur = (g.regCur &^ mask) | word(value)<<bitPos
}

// gatherTemplate0Default is RefinementContextGatherer::gather_template0_default.
func (g *refinementContextGatherer) gatherTemplate0Default(_, _ *bitmap, x uint32) uint16 {
	bx := x - g.regCurX
	rbx := g.referenceBufferX(x)

	newPixels := getBufPixel(g.regM1, bx+1)<<10 |
		getBufPixel(g.regCur, bx-1)<<9 |
		getBufPixel(g.refM1, rbx+1)<<6 |
		getBufPixel(g.refCur, rbx+1)<<3 |
		getBufPixel(g.refP1, rbx+1)

	g.ctx = (g.ctx<<1)&refinementShiftMaskT0Default | newPixels
	return g.ctx
}

// gatherTemplate0Custom is RefinementContextGatherer::gather_template0_custom.
func (g *refinementContextGatherer) gatherTemplate0Custom(region, reference *bitmap, x uint32) uint16 {
	bx := x - g.regCurX
	refXI := int32(x) - g.referenceDX
	rbx := g.referenceBufferX(x)

	xi := int32(x)
	yi := int32(g.regY)
	refY := yi - g.referenceDY
	at1 := g.atPixels[0]
	at2 := g.atPixels[1]

	newPixels := uint16(region.getPixel(uint32(xi+int32(at1.x)), uint32(yi+int32(at1.y))))<<12 |
		getBufPixel(g.regM1, bx+1)<<10 |
		getBufPixel(g.regCur, bx-1)<<9 |
		uint16(reference.getPixel(uint32(refXI+int32(at2.x)), uint32(refY+int32(at2.y))))<<8 |
		getBufPixel(g.refM1, rbx+1)<<6 |
		getBufPixel(g.refCur, rbx+1)<<3 |
		getBufPixel(g.refP1, rbx+1)

	g.ctx = (g.ctx<<1)&refinementShiftMaskT0Custom | newPixels
	return g.ctx
}

// gatherTemplate1 is RefinementContextGatherer::gather_template1.
func (g *refinementContextGatherer) gatherTemplate1(_, _ *bitmap, x uint32) uint16 {
	bx := x - g.regCurX
	rbx := g.referenceBufferX(x)

	newPixels := getBufPixel(g.regM1, bx+1)<<7 |
		getBufPixel(g.regCur, bx-1)<<6 |
		getBufPixel(g.refM1, rbx)<<5 |
		getBufPixel(g.refCur, rbx+1)<<2 |
		getBufPixel(g.refP1, rbx+1)

	g.ctx = (g.ctx<<1)&refinementShiftMaskT1 | newPixels
	return g.ctx
}

// decodeRefinementBitmap is decode_bitmap in hayro-jbig2
// src/decode/generic_refinement.rs.
func decodeRefinementBitmap(decoder *arithmeticDecoder, contexts []arithmeticDecoderContext,
	region, reference *bitmap, referenceDX, referenceDY int32, grTemplate refinementTemplate,
	at []adaptiveTemplatePixel, tpgron bool, c cancel) error {
	g := newRefinementContextGatherer(grTemplate, at, referenceDX, referenceDY)

	var sltpContext uint16 = 0b0000001000
	if grTemplate == refinementTemplate0 {
		sltpContext = 0b0000000010000
	}

	var gather func(g *refinementContextGatherer, region, reference *bitmap, x uint32) uint16
	switch {
	case grTemplate == refinementTemplate0 && g.useDefaultAT:
		gather = (*refinementContextGatherer).gatherTemplate0Default
	case grTemplate == refinementTemplate0:
		// Port: upstream indexes the two AT pixels, and a header without
		// them panics in Rust. The parser always reads two AT pixels for the
		// template 0, so this check only keeps a fault from a panic.
		if len(at) < 2 {
			return ErrInvalidATPixel
		}
		gather = (*refinementContextGatherer).gatherTemplate0Custom
	default:
		gather = (*refinementContextGatherer).gatherTemplate1
	}

	// The macro refinement_decode_loop! of upstream.
	width := region.width
	height := region.height

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

		if tpgron {
			sltp := decoder.readBit(&contexts[sltpContext])
			ltp = ltp != (sltp != 0)
		}

		g.startRow(region, reference, y)

		for x := range width {
			g.maybeReloadBuffers(region, reference, x)
			context := gather(g, region, reference, x)

			var value uint8
			if ltp && g.tpgrAllSame(x) {
				value = g.refCenterPixel(x)
			} else {
				value = uint8(decoder.readBit(&contexts[context]))
			}

			if value != 0 {
				region.setPixel(x, y, value)
				g.updateCurrentRow(x, value)
			}
		}
	}

	return nil
}
