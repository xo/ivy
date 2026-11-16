// Copyright (c) The Hayro Authors
//
// Use of this source code is governed by the Apache-2.0 or MIT license that
// can be found in the LICENSE-APACHE and LICENSE-MIT files.

package jpeg2000

// This file ports hayro-jpeg2000 src/j2c/bitplane.rs.

// bitplaneDecode is decode in hayro-jpeg2000 src/j2c/bitplane.rs. It decodes
// the coding passes of one code-block into the context of the tile.
func bitplaneDecode(cb *codeBlock, sbt subBandType, totalBitplanes uint8, style *codeBlockStyle, tileCtx *tileDecodeContext, storage *decompositionStorage, strict bool) error {
	ctx := &tileCtx.bitPlaneDecodeContext
	if err := ctx.reset(cb, sbt, style, totalBitplanes, strict); err != nil {
		return err
	}
	tileCtx.bitPlaneDecodeBuffers.reset()

	if !decodeInner(cb, storage, ctx, &tileCtx.bitPlaneDecodeBuffers) {
		return errCodeBlockDecodeFailure
	}
	return nil
}

// decodeInner is decode_inner in hayro-jpeg2000 src/j2c/bitplane.rs.
//
// Port: upstream asserts that the segments follow each other, and that the
// coding passes add up, and panics otherwise. The port returns false.
func decodeInner(cb *codeBlock, storage *decompositionStorage, ctx *bitPlaneDecodeContext, bp *bitPlaneDecodeBuffers) bool {
	bp.reset()

	var lastSegmentIdx uint8
	var codingPasses uint8

	for _, l := range storage.layers[cb.layers.start:cb.layers.end] {
		if !l.hasSegments {
			continue
		}
		for _, seg := range storage.segments[l.segments.start:l.segments.end] {
			if seg.idx != lastSegmentIdx {
				if seg.idx != lastSegmentIdx+1 {
					return false
				}
				bp.segmentRanges = append(bp.segmentRanges, len(bp.combinedLayers))
				bp.segmentCodingPasses = append(bp.segmentCodingPasses, codingPasses)
				lastSegmentIdx++
			}
			bp.combinedLayers = append(bp.combinedLayers, seg.data...)
			codingPasses += seg.codingPases
		}
	}

	if codingPasses != cb.numberOfCodingPasses {
		return false
	}

	bp.segmentRanges = append(bp.segmentRanges, len(bp.combinedLayers))
	bp.segmentCodingPasses = append(bp.segmentCodingPasses, codingPasses)

	isNormalMode := !ctx.style.selectiveArithmeticCodingBypass && !ctx.style.terminationOnEachPass

	if isNormalMode {
		decoder := newArithmeticDecoder(bp.combinedLayers)
		end := min(cb.numberOfCodingPasses, ctx.maxCodingPasses)
		if ctx.canUseFastPath() {
			return fastPathHandleCodingPasses(0, end, ctx, &decoder)
		}
		return handleCodingPasses(0, end, ctx, arithmeticBitDecoder{&decoder})
	}

	for seg := 0; seg < len(bp.segmentCodingPasses)-1; seg++ {
		startPass := bp.segmentCodingPasses[seg]
		endPass := min(bp.segmentCodingPasses[seg+1], ctx.maxCodingPasses)
		data := bp.combinedLayers[bp.segmentRanges[seg]:bp.segmentRanges[seg+1]]

		useArithmetic := true
		if ctx.style.selectiveArithmeticCodingBypass && startPass > 9 {
			useArithmetic = startPass%3 == 0
		}

		var ok bool
		if useArithmetic {
			decoder := newArithmeticDecoder(data)
			ok = handleCodingPasses(startPass, endPass, ctx, arithmeticBitDecoder{&decoder})
		} else {
			decoder := newBypassDecoder(data, ctx.strict)
			ok = handleCodingPasses(startPass, endPass, ctx, &decoder)
		}
		if !ok {
			return false
		}
	}
	return true
}

// handleCodingPasses is handle_coding_passes in hayro-jpeg2000
// src/j2c/bitplane.rs. It runs the coding passes from start to end, without
// end.
func handleCodingPasses(start, end uint8, ctx *bitPlaneDecodeContext, decoder bitDecoder) bool {
	reset := ctx.style.resetContextProbabilities

	for pass := start; pass < end; pass++ {
		currentBitplane := (pass + 2) / 3
		ctx.currentBitPosition = ctx.bitplanes - 1 - currentBitplane

		switch pass % 3 {
		case 0:
			if !cleanupPass(ctx, decoder) {
				return false
			}
			if ctx.style.segmentationSymbols {
				var b [4]uint32
				for i := range b {
					v, ok := decoder.readBit(ctx.arithmeticDecoderContext(18))
					if !ok {
						return false
					}
					b[i] = v
				}
				if (b[0] != 1 || b[1] != 0 || b[2] != 1 || b[3] != 0) && ctx.strict {
					return false
				}
			}
			ctx.resetForNextBitplane()
		case 1:
			if !significancePropagationPass(ctx, decoder) {
				return false
			}
		case 2:
			if !magnitudeRefinementPass(ctx, decoder) {
				return false
			}
		}

		if reset {
			ctx.resetContexts()
		}
	}
	return true
}

// bitplaneBitSize is BITPLANE_BIT_SIZE. We only allow 31 bit planes
// because we need one bit for the sign.
const bitplaneBitSize = 4*8 - 1

const (
	// significanceShift is SIGNIFICANCE_SHIFT.
	significanceShift = 7
	// hasMagnitudeRefinementShift is HAS_MAGNITUDE_REFINEMENT_SHIFT.
	hasMagnitudeRefinementShift = 6
	// hasZeroCodingShift is HAS_ZERO_CODING_SHIFT.
	hasZeroCodingShift = 5
	// decodedBitPositionMask is DECODED_BIT_POSITION_MASK.
	decodedBitPositionMask = 0x1f
	// cleanupSkipMask is CLEANUP_SKIP_MASK.
	cleanupSkipMask = 1<<significanceShift | 1<<hasZeroCodingShift
	// refinementPassMask is REFINEMENT_PASS_MASK.
	refinementPassMask = 1<<significanceShift | 1<<hasZeroCodingShift
)

// coefficientState is CoefficientState in hayro-jpeg2000
// src/j2c/bitplane.rs.
type coefficientState uint8

// setSignificant is CoefficientState::set_significant.
func (s *coefficientState) setSignificant() {
	*s |= 1 << significanceShift
}

// setZeroCoded is CoefficientState::set_zero_coded.
func (s *coefficientState) setZeroCoded(value uint8) {
	*s &^= 1 << hasZeroCodingShift
	*s |= coefficientState(value << hasZeroCodingShift)
}

// setMagnitudeRefined is CoefficientState::set_magnitude_refined.
func (s *coefficientState) setMagnitudeRefined() {
	*s |= 1 << hasMagnitudeRefinementShift
}

// setDecodedBitPosition is CoefficientState::set_decoded_bit_position.
func (s *coefficientState) setDecodedBitPosition(position uint8) {
	*s = *s&^decodedBitPositionMask | coefficientState(position)
}

// decodedBitPosition is CoefficientState::decoded_bit_position.
func (s coefficientState) decodedBitPosition() uint8 {
	return uint8(s & decodedBitPositionMask)
}

// isSignificant is CoefficientState::is_significant.
func (s coefficientState) isSignificant() bool {
	return s.significance() == 1
}

// significance is CoefficientState::significance.
func (s coefficientState) significance() uint8 {
	return uint8(s>>significanceShift) & 1
}

// magnitudeRefinement is CoefficientState::magnitude_refinement.
func (s coefficientState) magnitudeRefinement() uint8 {
	return uint8(s>>hasMagnitudeRefinementShift) & 1
}

// shouldDecodeCleanup is CoefficientState::should_decode_cleanup.
func (s coefficientState) shouldDecodeCleanup() bool {
	return s&cleanupSkipMask == 0
}

// shouldDecodeRefinement is CoefficientState::should_decode_refinement.
func (s coefficientState) shouldDecodeRefinement() bool {
	return s&refinementPassMask == 1<<significanceShift
}

// coefficient is Coefficient in hayro-jpeg2000 src/j2c/bitplane.rs. The
// high bit holds the sign, and the other bits hold the magnitude.
type coefficient uint32

// reconstructed is Coefficient::reconstructed. It returns the value of the
// coefficient, with the offset to the middle of the interval of the bits
// that the decoder did not decode.
func (c coefficient) reconstructed(state coefficientState, irreversible bool, roiShift uint8) float32 {
	magnitude := uint32(c) &^ 0x80000000
	bitPosition := state.decodedBitPosition()

	var roiMagnitude uint32
	if roiShift < 32 {
		roiMagnitude = magnitude >> roiShift
	}
	if roiMagnitude != 0 {
		magnitude = roiMagnitude
		if bitPosition > roiShift {
			bitPosition -= roiShift
		} else {
			bitPosition = 0
		}
	}

	m := int32(magnitude)
	m *= 1 - 2*int32(c.sign())

	if m != 0 && (irreversible || bitPosition != 0) {
		offset := 0.5 * float32(uint32(1)<<bitPosition)
		if m > 0 {
			return float32(m) + offset
		}
		return float32(m) - offset
	}
	return float32(m)
}

// setSign is Coefficient::set_sign.
func (c *coefficient) setSign(sign uint8) {
	*c |= coefficient(uint32(sign) << 31)
}

// sign is Coefficient::sign.
func (c coefficient) sign() uint32 {
	return uint32(c>>31) & 1
}

// pushBitAt is Coefficient::push_bit_at.
func (c *coefficient) pushBitAt(bit uint32, position uint8) {
	*c |= coefficient(bit << position)
}

// coefficientsPadding is COEFFICIENTS_PADDING.
const coefficientsPadding = 1

// neighborSignificances is NeighborSignificances in hayro-jpeg2000
// src/j2c/bitplane.rs. Direct neighbors occupy the even bits so they can be
// masked in one step.
type neighborSignificances uint8

func (n *neighborSignificances) setTopLeft()     { *n |= 1 << 7 }
func (n *neighborSignificances) setTop()         { *n |= 1 << 6 }
func (n *neighborSignificances) setTopRight()    { *n |= 1 << 5 }
func (n *neighborSignificances) setLeft()        { *n |= 1 << 4 }
func (n *neighborSignificances) setBottomLeft()  { *n |= 1 << 3 }
func (n *neighborSignificances) setRight()       { *n |= 1 << 2 }
func (n *neighborSignificances) setBottomRight() { *n |= 1 << 1 }
func (n *neighborSignificances) setBottom()      { *n |= 1 }

// all is NeighborSignificances::all.
func (n neighborSignificances) all() uint8 {
	return uint8(n)
}

// allWithoutBottom is NeighborSignificances::all_without_bottom.
func (n neighborSignificances) allWithoutBottom() uint8 {
	return uint8(n) & 0b11110100
}

// bitPlaneDecodeBuffers is BitPlaneDecodeBuffers in hayro-jpeg2000
// src/j2c/bitplane.rs.
type bitPlaneDecodeBuffers struct {
	combinedLayers      []byte
	segmentRanges       []int
	segmentCodingPasses []uint8
}

// reset is BitPlaneDecodeBuffers::reset.
func (b *bitPlaneDecodeBuffers) reset() {
	b.combinedLayers = b.combinedLayers[:0]
	b.segmentRanges = b.segmentRanges[:0]
	b.segmentCodingPasses = b.segmentCodingPasses[:0]

	// Fenceposts: segment i spans entries i and i+1.
	b.segmentRanges = append(b.segmentRanges, 0)
	b.segmentCodingPasses = append(b.segmentCodingPasses, 0)
}

// bitPlaneDecodeContext is BitPlaneDecodeContext in hayro-jpeg2000
// src/j2c/bitplane.rs.
//
// Port: the zero value has a padded width of 0, where the Default of
// upstream gives 2. reset sets the width before any use.
type bitPlaneDecodeContext struct {
	coefficientStates     []coefficientState
	stripeFlags           []uint32
	neighborSignificances []neighborSignificances
	coefficients          []coefficient
	width                 uint32
	paddedWidth           uint32
	height                uint32
	style                 codeBlockStyle
	bitplanes             uint8
	strict                bool
	maxCodingPasses       uint8
	subBandType           subBandType
	contexts              [19]arithmeticDecoderContext
	currentBitPosition    uint8
}

// canUseFastPath is BitPlaneDecodeContext::can_use_fast_path.
func (ctx *bitPlaneDecodeContext) canUseFastPath() bool {
	return ctx.height%4 == 0 && !ctx.style.verticallyCausalContext
}

// resized returns s with the length n and every element zero.
func resized[T any](s []T, n int) []T {
	if cap(s) < n {
		return make([]T, n)
	}
	s = s[:n]
	clear(s)
	return s
}

// reset is BitPlaneDecodeContext::reset.
func (ctx *bitPlaneDecodeContext) reset(cb *codeBlock, sbt subBandType, style *codeBlockStyle, totalBitplanes uint8, strict bool) error {
	width, height := cb.rect.width(), cb.rect.height()
	paddedWidth := width + coefficientsPadding*2
	paddedHeight := height + coefficientsPadding*2
	// Port: a code-block has at most 4096 samples, but a malformed
	// geometry can wrap the size of its rectangle.
	if uint64(paddedWidth)*uint64(paddedHeight) > 1<<24 || width > 1<<20 || height > 1<<20 {
		return errCodeBlockDecodeFailure
	}
	n := int(paddedWidth) * int(paddedHeight)

	ctx.coefficients = resized(ctx.coefficients, n)
	ctx.neighborSignificances = resized(ctx.neighborSignificances, n)
	ctx.coefficientStates = resized(ctx.coefficientStates, n)
	ctx.stripeFlags = resized(ctx.stripeFlags, int(divCeil32(height, 4)+2)*int(paddedWidth))

	ctx.width = width
	ctx.paddedWidth = paddedWidth
	ctx.height = height
	ctx.subBandType = sbt
	ctx.style = *style
	ctx.resetContexts()

	// Lenient on files whose missing bitplanes exceed the total (issue 399).
	switch {
	case totalBitplanes >= cb.missingBitPlanes:
		ctx.bitplanes = totalBitplanes - cb.missingBitPlanes
	case strict:
		return errInvalidBitplaneCount
	default:
		ctx.bitplanes = 0
	}

	ctx.maxCodingPasses = 0
	if ctx.bitplanes != 0 {
		ctx.maxCodingPasses = 1 + 3*(ctx.bitplanes-1)
	}

	if ctx.maxCodingPasses < cb.numberOfCodingPasses && strict {
		return errTooManyCodingPasses
	}
	// Upstream never sets the field strict of the context, so it stays
	// false. The bypass decoder and the check of the segmentation symbols
	// are always lenient. The port keeps that.
	return nil
}

// coefficientRows is BitPlaneDecodeContext::coefficient_rows. It calls fn
// with the coefficients and the states of each row of the code-block,
// without the padding.
func (ctx *bitPlaneDecodeContext) coefficientRows(fn func(coefficients []coefficient, states []coefficientState)) {
	w := int(ctx.paddedWidth)
	for y := range int(ctx.height) {
		start := (y+coefficientsPadding)*w + coefficientsPadding
		fn(ctx.coefficients[start:start+int(ctx.width)], ctx.coefficientStates[start:start+int(ctx.width)])
	}
}

// setSign is BitPlaneDecodeContext::set_sign.
func (ctx *bitPlaneDecodeContext) setSign(pos position, sign uint8) {
	ctx.coefficients[pos.index(ctx.paddedWidth)].setSign(sign)
}

// arithmeticDecoderContext is
// BitPlaneDecodeContext::arithmetic_decoder_context.
func (ctx *bitPlaneDecodeContext) arithmeticDecoderContext(label uint8) *arithmeticDecoderContext {
	return &ctx.contexts[label]
}

// resetContexts is BitPlaneDecodeContext::reset_contexts.
func (ctx *bitPlaneDecodeContext) resetContexts() {
	for i := range ctx.contexts {
		ctx.contexts[i].reset()
	}
	ctx.contexts[0].resetWithIndex(4)
	ctx.contexts[17].resetWithIndex(3)
	ctx.contexts[18].resetWithIndex(46)
}

// resetForNextBitplane is BitPlaneDecodeContext::reset_for_next_bitplane.
func (ctx *bitPlaneDecodeContext) resetForNextBitplane() {
	for i := range ctx.coefficientStates {
		ctx.coefficientStates[i].setZeroCoded(0)
	}
}

// isSignificant is BitPlaneDecodeContext::is_significant.
func (ctx *bitPlaneDecodeContext) isSignificant(pos position) bool {
	return ctx.coefficientStates[pos.index(ctx.paddedWidth)].isSignificant()
}

// setSignificant is BitPlaneDecodeContext::set_significant.
func (ctx *bitPlaneDecodeContext) setSignificant(pos position) {
	w := ctx.paddedWidth
	idx := pos.index(w)
	ctx.coefficientStates[idx].setSignificant()
	ctx.coefficientStates[idx].setDecodedBitPosition(ctx.currentBitPosition)

	ns := ctx.neighborSignificances
	ns[pos.topLeft().index(w)].setBottomRight()
	ns[pos.top().index(w)].setBottom()
	ns[pos.topRight().index(w)].setBottomLeft()
	ns[pos.left().index(w)].setRight()
	ns[pos.right().index(w)].setLeft()
	ns[pos.bottomLeft().index(w)].setTopRight()
	ns[pos.bottom().index(w)].setTop()
	ns[pos.bottomRight().index(w)].setTopLeft()
}

// setZeroCoded is BitPlaneDecodeContext::set_zero_coded.
func (ctx *bitPlaneDecodeContext) setZeroCoded(pos position) {
	ctx.coefficientStates[pos.index(ctx.paddedWidth)].setZeroCoded(1)
}

// setMagnitudeRefined is BitPlaneDecodeContext::set_magnitude_refined.
func (ctx *bitPlaneDecodeContext) setMagnitudeRefined(pos position) {
	s := &ctx.coefficientStates[pos.index(ctx.paddedWidth)]
	s.setMagnitudeRefined()
	s.setDecodedBitPosition(ctx.currentBitPosition)
}

// magnitudeRefinement is BitPlaneDecodeContext::magnitude_refinement.
func (ctx *bitPlaneDecodeContext) magnitudeRefinement(pos position) uint8 {
	return ctx.coefficientStates[pos.index(ctx.paddedWidth)].magnitudeRefinement()
}

// shouldDecodeCleanup is BitPlaneDecodeContext::should_decode_cleanup.
func (ctx *bitPlaneDecodeContext) shouldDecodeCleanup(pos position) bool {
	return ctx.coefficientStates[pos.index(ctx.paddedWidth)].shouldDecodeCleanup()
}

// shouldDecodeRefinement is BitPlaneDecodeContext::should_decode_refinement.
func (ctx *bitPlaneDecodeContext) shouldDecodeRefinement(pos position) bool {
	return ctx.coefficientStates[pos.index(ctx.paddedWidth)].shouldDecodeRefinement()
}

// pushMagnitudeBit is BitPlaneDecodeContext::push_magnitude_bit.
func (ctx *bitPlaneDecodeContext) pushMagnitudeBit(pos position, bit uint32) {
	ctx.coefficients[pos.index(ctx.paddedWidth)].pushBitAt(bit, ctx.currentBitPosition)
}

// sign is BitPlaneDecodeContext::sign.
func (ctx *bitPlaneDecodeContext) sign(pos position) uint8 {
	return uint8(ctx.coefficients[pos.index(ctx.paddedWidth)].sign())
}

// neighborInNextStripe is BitPlaneDecodeContext::neighbor_in_next_stripe.
func (ctx *bitPlaneDecodeContext) neighborInNextStripe(pos position) bool {
	n := pos.bottom()
	return n.realY() < ctx.height && n.realY()>>2 > pos.realY()>>2
}

// neighborhoodSignificanceStates is
// BitPlaneDecodeContext::neighborhood_significance_states.
func (ctx *bitPlaneDecodeContext) neighborhoodSignificanceStates(pos position) uint8 {
	n := ctx.neighborSignificances[pos.index(ctx.paddedWidth)]
	if ctx.style.verticallyCausalContext && ctx.neighborInNextStripe(pos) {
		return n.allWithoutBottom()
	}
	return n.all()
}

// cleanupPass is cleanup_pass in hayro-jpeg2000 src/j2c/bitplane.rs.
func cleanupPass(ctx *bitPlaneDecodeContext, decoder bitDecoder) bool {
	return forEachPosition(ctx.width, ctx.height, func(cur *position) bool {
		if !ctx.shouldDecodeCleanup(*cur) {
			return true
		}
		useRL := cur.realY()%4 == 0 &&
			ctx.height-cur.realY() >= 4 &&
			ctx.neighborhoodSignificanceStates(*cur) == 0 &&
			ctx.neighborhoodSignificanceStates(position{cur.indexX, cur.indexY + 1}) == 0 &&
			ctx.neighborhoodSignificanceStates(position{cur.indexX, cur.indexY + 2}) == 0 &&
			ctx.neighborhoodSignificanceStates(position{cur.indexX, cur.indexY + 3}) == 0

		var bit uint32
		if useRL {
			b, ok := decoder.readBit(ctx.arithmeticDecoderContext(17))
			if !ok {
				return false
			}
			if b == 0 {
				cur.indexY += 3
				return true
			}
			z0, ok := decoder.readBit(ctx.arithmeticDecoderContext(18))
			if !ok {
				return false
			}
			z1, ok := decoder.readBit(ctx.arithmeticDecoderContext(18))
			if !ok {
				return false
			}
			cur.indexY += z0<<1 | z1
			bit = 1
		} else {
			label := contextLabelZeroCoding(*cur, ctx)
			b, ok := decoder.readBit(ctx.arithmeticDecoderContext(label))
			if !ok {
				return false
			}
			bit = b
		}

		if bit == 1 {
			ctx.pushMagnitudeBit(*cur, bit)
			// Upstream ignores the result of the sign here.
			decodeSignBit(*cur, ctx, decoder)
			ctx.setSignificant(*cur)
		}
		return true
	})
}

// significancePropagationPass is significance_propagation_pass in
// hayro-jpeg2000 src/j2c/bitplane.rs.
func significancePropagationPass(ctx *bitPlaneDecodeContext, decoder bitDecoder) bool {
	return forEachPosition(ctx.width, ctx.height, func(cur *position) bool {
		if ctx.isSignificant(*cur) || ctx.neighborhoodSignificanceStates(*cur) == 0 {
			return true
		}
		label := contextLabelZeroCoding(*cur, ctx)
		bit, ok := decoder.readBit(ctx.arithmeticDecoderContext(label))
		if !ok {
			return false
		}
		ctx.setZeroCoded(*cur)
		if bit == 1 {
			ctx.pushMagnitudeBit(*cur, bit)
			if !decodeSignBit(*cur, ctx, decoder) {
				return false
			}
			ctx.setSignificant(*cur)
		}
		return true
	})
}

// magnitudeRefinementPass is magnitude_refinement_pass in hayro-jpeg2000
// src/j2c/bitplane.rs.
func magnitudeRefinementPass(ctx *bitPlaneDecodeContext, decoder bitDecoder) bool {
	return forEachPosition(ctx.width, ctx.height, func(cur *position) bool {
		if !ctx.shouldDecodeRefinement(*cur) {
			return true
		}
		label := contextLabelMagnitudeRefinementCoding(*cur, ctx)
		bit, ok := decoder.readBit(ctx.arithmeticDecoderContext(label))
		if !ok {
			return false
		}
		if bit == 1 {
			ctx.pushMagnitudeBit(*cur, bit)
		}
		ctx.setMagnitudeRefined(*cur)
		return true
	})
}

// forEachPosition is for_each_position in hayro-jpeg2000
// src/j2c/bitplane.rs. It visits the code-block in stripes of four rows,
// column by column. The action can move the position down.
func forEachPosition(width, height uint32, action func(*position) bool) bool {
	for baseRow := uint32(0); baseRow < height; baseRow += 4 {
		end := min(baseRow+4, height)
		for x := range width {
			cur := newPosition(x, baseRow)
			for cur.realY() < end {
				if !action(&cur) {
					return false
				}
				cur.indexY++
			}
		}
	}
	return true
}

// decodeSignBit is decode_sign_bit in hayro-jpeg2000 src/j2c/bitplane.rs.
func decodeSignBit(pos position, ctx *bitPlaneDecodeContext, decoder bitDecoder) bool {
	label, xorBit := contextLabelSignCoding(pos, ctx)
	bit, ok := decoder.readBit(ctx.arithmeticDecoderContext(label))
	if !ok {
		return false
	}
	if !decoder.isBypass() {
		bit ^= uint32(xorBit)
	}
	ctx.setSign(pos, uint8(bit))
	return true
}

// contextLabelSignCoding is the function context_label_sign_coding of
// decode_sign_bit in hayro-jpeg2000 src/j2c/bitplane.rs.
func contextLabelSignCoding(pos position, ctx *bitPlaneDecodeContext) (label, xorBit uint8) {
	significances := ctx.neighborhoodSignificanceStates(pos) & 0b0101_0101

	leftSign := ctx.sign(pos.left())
	rightSign := ctx.sign(pos.right())
	topSign := ctx.sign(pos.top())
	var bottomSign uint8
	if !ctx.style.verticallyCausalContext || !ctx.neighborInNextStripe(pos) {
		bottomSign = ctx.sign(pos.bottom())
	}

	signs := topSign<<6 | leftSign<<4 | rightSign<<2 | bottomSign
	negative := significances & signs
	positive := significances &^ signs
	merged := negative<<1 | positive

	e := signContextLookup[merged]
	return e[0], e[1]
}

// contextLabelZeroCoding is context_label_zero_coding in hayro-jpeg2000
// src/j2c/bitplane.rs.
func contextLabelZeroCoding(pos position, ctx *bitPlaneDecodeContext) uint8 {
	n := ctx.neighborhoodSignificanceStates(pos)
	switch ctx.subBandType {
	case highLow:
		return zeroCtxHLLookup[n]
	case highHigh:
		return zeroCtxHHLookup[n]
	}
	return zeroCtxLLLHLookup[n]
}

// contextLabelMagnitudeRefinementCoding is
// context_label_magnitude_refinement_coding in hayro-jpeg2000
// src/j2c/bitplane.rs.
func contextLabelMagnitudeRefinementCoding(pos position, ctx *bitPlaneDecodeContext) uint8 {
	m1 := ctx.magnitudeRefinement(pos) * 16
	m2 := 14 + min(ctx.neighborhoodSignificanceStates(pos), 1)
	return max(m1, m2)
}

// position is Position in hayro-jpeg2000 src/j2c/bitplane.rs. Its indices
// are offset by coefficientsPadding.
type position struct {
	indexX, indexY uint32
}

// newPosition is Position::new.
func newPosition(x, y uint32) position {
	return position{x + coefficientsPadding, y + coefficientsPadding}
}

func (p position) left() position        { return position{p.indexX - 1, p.indexY} }
func (p position) right() position       { return position{p.indexX + 1, p.indexY} }
func (p position) top() position         { return position{p.indexX, p.indexY - 1} }
func (p position) bottom() position      { return position{p.indexX, p.indexY + 1} }
func (p position) topLeft() position     { return position{p.indexX - 1, p.indexY - 1} }
func (p position) topRight() position    { return position{p.indexX + 1, p.indexY - 1} }
func (p position) bottomLeft() position  { return position{p.indexX - 1, p.indexY + 1} }
func (p position) bottomRight() position { return position{p.indexX + 1, p.indexY + 1} }

// realY is Position::real_y.
func (p position) realY() uint32 {
	return p.indexY - 1
}

// index is Position::index.
func (p position) index(paddedWidth uint32) int {
	return int(p.indexX) + int(p.indexY)*int(paddedWidth)
}

// bitDecoder is BitDecoder in hayro-jpeg2000 src/j2c/bitplane.rs.
//
// Port: the constant IS_BYPASS of the trait is the method isBypass.
type bitDecoder interface {
	readBit(ctx *arithmeticDecoderContext) (uint32, bool)
	isBypass() bool
}

// arithmeticBitDecoder is the implementation of BitDecoder for
// ArithmeticDecoder in hayro-jpeg2000 src/j2c/bitplane.rs.
//
// Port: the arithmetic decoder has a method readBit of its own, with
// another signature, so a wrapper implements bitDecoder.
type arithmeticBitDecoder struct {
	d *arithmeticDecoder
}

// readBit is BitDecoder::read_bit for ArithmeticDecoder. It never fails.
func (a arithmeticBitDecoder) readBit(ctx *arithmeticDecoderContext) (uint32, bool) {
	return a.d.readBit(ctx), true
}

func (a arithmeticBitDecoder) isBypass() bool { return false }

// bypassDecoder is BypassDecoder in hayro-jpeg2000 src/j2c/bitplane.rs. It
// reads the raw bits of a pass in the bypass mode.
type bypassDecoder struct {
	reader bitReader
	strict bool
}

// newBypassDecoder is BypassDecoder::new.
func newBypassDecoder(data []byte, strict bool) bypassDecoder {
	return bypassDecoder{reader: newBitReader(data), strict: strict}
}

// readBit is BitDecoder::read_bit for BypassDecoder.
func (d *bypassDecoder) readBit(*arithmeticDecoderContext) (uint32, bool) {
	if b, ok := d.reader.readBitsWithStuffing(1); ok {
		return b, true
	}
	if !d.strict {
		// Pad with ones, as the arithmetic decoder pads with 0xFF.
		return 1, true
	}
	return 0, false
}

func (d *bypassDecoder) isBypass() bool { return true }

// signContextLookup is SIGN_CONTEXT_LOOKUP in hayro-jpeg2000
// src/j2c/bitplane.rs. Each entry holds the label of the context and the
// bit to flip the sign.
var signContextLookup = [256][2]uint8{
	{9, 0}, {10, 0}, {10, 1}, {0, 0}, {12, 0}, {13, 0}, {11, 0}, {0, 0},
	{12, 1}, {11, 1}, {13, 1}, {0, 0}, {0, 0}, {0, 0}, {0, 0}, {0, 0},
	{12, 0}, {13, 0}, {11, 0}, {0, 0}, {12, 0}, {13, 0}, {11, 0}, {0, 0},
	{9, 0}, {10, 0}, {10, 1}, {0, 0}, {0, 0}, {0, 0}, {0, 0}, {0, 0},
	{12, 1}, {11, 1}, {13, 1}, {0, 0}, {9, 0}, {10, 0}, {10, 1}, {0, 0},
	{12, 1}, {11, 1}, {13, 1}, {0, 0}, {0, 0}, {0, 0}, {0, 0}, {0, 0},
	{0, 0}, {0, 0}, {0, 0}, {0, 0}, {0, 0}, {0, 0}, {0, 0}, {0, 0},
	{0, 0}, {0, 0}, {0, 0}, {0, 0}, {0, 0}, {0, 0}, {0, 0}, {0, 0},
	{10, 0}, {10, 0}, {9, 0}, {0, 0}, {13, 0}, {13, 0}, {12, 0}, {0, 0},
	{11, 1}, {11, 1}, {12, 1}, {0, 0}, {0, 0}, {0, 0}, {0, 0}, {0, 0},
	{13, 0}, {13, 0}, {12, 0}, {0, 0}, {13, 0}, {13, 0}, {12, 0}, {0, 0},
	{10, 0}, {10, 0}, {9, 0}, {0, 0}, {0, 0}, {0, 0}, {0, 0}, {0, 0},
	{11, 1}, {11, 1}, {12, 1}, {0, 0}, {10, 0}, {10, 0}, {9, 0}, {0, 0},
	{11, 1}, {11, 1}, {12, 1}, {0, 0}, {0, 0}, {0, 0}, {0, 0}, {0, 0},
	{0, 0}, {0, 0}, {0, 0}, {0, 0}, {0, 0}, {0, 0}, {0, 0}, {0, 0},
	{0, 0}, {0, 0}, {0, 0}, {0, 0}, {0, 0}, {0, 0}, {0, 0}, {0, 0},
	{10, 1}, {9, 0}, {10, 1}, {0, 0}, {11, 0}, {12, 0}, {11, 0}, {0, 0},
	{13, 1}, {12, 1}, {13, 1}, {0, 0}, {0, 0}, {0, 0}, {0, 0}, {0, 0},
	{11, 0}, {12, 0}, {11, 0}, {0, 0}, {11, 0}, {12, 0}, {11, 0}, {0, 0},
	{10, 1}, {9, 0}, {10, 1}, {0, 0}, {0, 0}, {0, 0}, {0, 0}, {0, 0},
	{13, 1}, {12, 1}, {13, 1}, {0, 0}, {10, 1}, {9, 0}, {10, 1}, {0, 0},
	{13, 1}, {12, 1}, {13, 1}, {0, 0}, {0, 0}, {0, 0}, {0, 0}, {0, 0},
	{0, 0}, {0, 0}, {0, 0}, {0, 0}, {0, 0}, {0, 0}, {0, 0}, {0, 0},
	{0, 0}, {0, 0}, {0, 0}, {0, 0}, {0, 0}, {0, 0}, {0, 0}, {0, 0},
	{0, 0}, {0, 0}, {0, 0}, {0, 0}, {0, 0}, {0, 0}, {0, 0}, {0, 0},
	{0, 0}, {0, 0}, {0, 0}, {0, 0}, {0, 0}, {0, 0}, {0, 0}, {0, 0},
	{0, 0}, {0, 0}, {0, 0}, {0, 0}, {0, 0}, {0, 0}, {0, 0}, {0, 0},
	{0, 0}, {0, 0}, {0, 0}, {0, 0}, {0, 0}, {0, 0}, {0, 0}, {0, 0},
	{0, 0}, {0, 0}, {0, 0}, {0, 0}, {0, 0}, {0, 0}, {0, 0}, {0, 0},
	{0, 0}, {0, 0}, {0, 0}, {0, 0}, {0, 0}, {0, 0}, {0, 0}, {0, 0},
	{0, 0}, {0, 0}, {0, 0}, {0, 0}, {0, 0}, {0, 0}, {0, 0}, {0, 0},
	{0, 0}, {0, 0}, {0, 0}, {0, 0}, {0, 0}, {0, 0}, {0, 0}, {0, 0},
}

// zeroCtxLLLHLookup is ZERO_CTX_LL_LH_LOOKUP in hayro-jpeg2000
// src/j2c/bitplane.rs.
var zeroCtxLLLHLookup = [256]uint8{
	0, 3, 1, 3, 5, 7, 6, 7, 1, 3, 2, 3, 6, 7, 6, 7,
	5, 7, 6, 7, 8, 8, 8, 8, 6, 7, 6, 7, 8, 8, 8, 8,
	1, 3, 2, 3, 6, 7, 6, 7, 2, 3, 2, 3, 6, 7, 6, 7,
	6, 7, 6, 7, 8, 8, 8, 8, 6, 7, 6, 7, 8, 8, 8, 8,
	3, 4, 3, 4, 7, 7, 7, 7, 3, 4, 3, 4, 7, 7, 7, 7,
	7, 7, 7, 7, 8, 8, 8, 8, 7, 7, 7, 7, 8, 8, 8, 8,
	3, 4, 3, 4, 7, 7, 7, 7, 3, 4, 3, 4, 7, 7, 7, 7,
	7, 7, 7, 7, 8, 8, 8, 8, 7, 7, 7, 7, 8, 8, 8, 8,
	1, 3, 2, 3, 6, 7, 6, 7, 2, 3, 2, 3, 6, 7, 6, 7,
	6, 7, 6, 7, 8, 8, 8, 8, 6, 7, 6, 7, 8, 8, 8, 8,
	2, 3, 2, 3, 6, 7, 6, 7, 2, 3, 2, 3, 6, 7, 6, 7,
	6, 7, 6, 7, 8, 8, 8, 8, 6, 7, 6, 7, 8, 8, 8, 8,
	3, 4, 3, 4, 7, 7, 7, 7, 3, 4, 3, 4, 7, 7, 7, 7,
	7, 7, 7, 7, 8, 8, 8, 8, 7, 7, 7, 7, 8, 8, 8, 8,
	3, 4, 3, 4, 7, 7, 7, 7, 3, 4, 3, 4, 7, 7, 7, 7,
	7, 7, 7, 7, 8, 8, 8, 8, 7, 7, 7, 7, 8, 8, 8, 8,
}

// zeroCtxHLLookup is ZERO_CTX_HL_LOOKUP in hayro-jpeg2000
// src/j2c/bitplane.rs.
var zeroCtxHLLookup = [256]uint8{
	0, 5, 1, 6, 3, 7, 3, 7, 1, 6, 2, 6, 3, 7, 3, 7,
	3, 7, 3, 7, 4, 7, 4, 7, 3, 7, 3, 7, 4, 7, 4, 7,
	1, 6, 2, 6, 3, 7, 3, 7, 2, 6, 2, 6, 3, 7, 3, 7,
	3, 7, 3, 7, 4, 7, 4, 7, 3, 7, 3, 7, 4, 7, 4, 7,
	5, 8, 6, 8, 7, 8, 7, 8, 6, 8, 6, 8, 7, 8, 7, 8,
	7, 8, 7, 8, 7, 8, 7, 8, 7, 8, 7, 8, 7, 8, 7, 8,
	6, 8, 6, 8, 7, 8, 7, 8, 6, 8, 6, 8, 7, 8, 7, 8,
	7, 8, 7, 8, 7, 8, 7, 8, 7, 8, 7, 8, 7, 8, 7, 8,
	1, 6, 2, 6, 3, 7, 3, 7, 2, 6, 2, 6, 3, 7, 3, 7,
	3, 7, 3, 7, 4, 7, 4, 7, 3, 7, 3, 7, 4, 7, 4, 7,
	2, 6, 2, 6, 3, 7, 3, 7, 2, 6, 2, 6, 3, 7, 3, 7,
	3, 7, 3, 7, 4, 7, 4, 7, 3, 7, 3, 7, 4, 7, 4, 7,
	6, 8, 6, 8, 7, 8, 7, 8, 6, 8, 6, 8, 7, 8, 7, 8,
	7, 8, 7, 8, 7, 8, 7, 8, 7, 8, 7, 8, 7, 8, 7, 8,
	6, 8, 6, 8, 7, 8, 7, 8, 6, 8, 6, 8, 7, 8, 7, 8,
	7, 8, 7, 8, 7, 8, 7, 8, 7, 8, 7, 8, 7, 8, 7, 8,
}

// zeroCtxHHLookup is ZERO_CTX_HH_LOOKUP in hayro-jpeg2000
// src/j2c/bitplane.rs.
var zeroCtxHHLookup = [256]uint8{
	0, 1, 3, 4, 1, 2, 4, 5, 3, 4, 6, 7, 4, 5, 7, 7,
	1, 2, 4, 5, 2, 2, 5, 5, 4, 5, 7, 7, 5, 5, 7, 7,
	3, 4, 6, 7, 4, 5, 7, 7, 6, 7, 8, 8, 7, 7, 8, 8,
	4, 5, 7, 7, 5, 5, 7, 7, 7, 7, 8, 8, 7, 7, 8, 8,
	1, 2, 4, 5, 2, 2, 5, 5, 4, 5, 7, 7, 5, 5, 7, 7,
	2, 2, 5, 5, 2, 2, 5, 5, 5, 5, 7, 7, 5, 5, 7, 7,
	4, 5, 7, 7, 5, 5, 7, 7, 7, 7, 8, 8, 7, 7, 8, 8,
	5, 5, 7, 7, 5, 5, 7, 7, 7, 7, 8, 8, 7, 7, 8, 8,
	3, 4, 6, 7, 4, 5, 7, 7, 6, 7, 8, 8, 7, 7, 8, 8,
	4, 5, 7, 7, 5, 5, 7, 7, 7, 7, 8, 8, 7, 7, 8, 8,
	6, 7, 8, 8, 7, 7, 8, 8, 8, 8, 8, 8, 8, 8, 8, 8,
	7, 7, 8, 8, 7, 7, 8, 8, 8, 8, 8, 8, 8, 8, 8, 8,
	4, 5, 7, 7, 5, 5, 7, 7, 7, 7, 8, 8, 7, 7, 8, 8,
	5, 5, 7, 7, 5, 5, 7, 7, 7, 7, 8, 8, 7, 7, 8, 8,
	7, 7, 8, 8, 7, 7, 8, 8, 8, 8, 8, 8, 8, 8, 8, 8,
	7, 7, 8, 8, 7, 7, 8, 8, 8, 8, 8, 8, 8, 8, 8, 8,
}

// The functions below port the module fast_path in hayro-jpeg2000
// src/j2c/bitplane.rs. Each name takes the prefix fastPath.
//
// Note: the fast path takes some core ideas from OpenJPEG, such as the
// idea to pack the state of a column into a u32, and adapts them to this
// decoder.

// fastPathHandleCodingPasses is fast_path::handle_coding_passes.
func fastPathHandleCodingPasses(start, end uint8, ctx *bitPlaneDecodeContext, decoder *arithmeticDecoder) bool {
	reset := ctx.style.resetContextProbabilities

	for pass := start; pass < end; pass++ {
		currentBitplane := (pass + 2) / 3
		ctx.currentBitPosition = ctx.bitplanes - 1 - currentBitplane

		switch pass % 3 {
		case 0:
			fastPathCleanupPass(ctx, decoder)
			if ctx.style.segmentationSymbols {
				b0 := decoder.readBit(&ctx.contexts[18])
				b1 := decoder.readBit(&ctx.contexts[18])
				b2 := decoder.readBit(&ctx.contexts[18])
				b3 := decoder.readBit(&ctx.contexts[18])
				if (b0 != 1 || b1 != 0 || b2 != 1 || b3 != 0) && ctx.strict {
					return false
				}
			}
			fastPathResetForNextBitplane(ctx)
		case 1:
			fastPathSignificancePropagationPass(ctx, decoder)
		case 2:
			fastPathMagnitudeRefinementPass(ctx, decoder)
		}

		if reset {
			ctx.resetContexts()
		}
	}
	return true
}

// fastPathCleanupPass is fast_path::cleanup_pass.
func fastPathCleanupPass(ctx *bitPlaneDecodeContext, decoder *arithmeticDecoder) {
	zeroContexts := fastPathZeroContextLookup(ctx.subBandType)

	fastPathForEachStripeColumn(ctx, func(ctx *bitPlaneDecodeContext, column stripeColumn) uint32 {
		flags := column.flags
		if flags == 0 {
			if decoder.readBit(&ctx.contexts[17]) == 1 {
				numZeroes := decoder.readBit(&ctx.contexts[18])
				numZeroes = numZeroes<<1 | decoder.readBit(&ctx.contexts[18])
				fastPathCleanupFromRunLength(ctx, decoder, zeroContexts, column, &flags, numZeroes)
			}
		} else {
			for i := range uint32(4) {
				fastPathCleanupCoefficient(ctx, decoder, zeroContexts, column, &flags, i, true, false)
			}
		}
		return flags &^ flagZeroCodedAll
	})
}

// fastPathCleanupFromRunLength is fast_path::cleanup_from_run_length.
func fastPathCleanupFromRunLength(ctx *bitPlaneDecodeContext, decoder *arithmeticDecoder, zeroContexts *[512]uint8, column stripeColumn, flags *uint32, numZeroes uint32) {
	first := min(numZeroes, 3)
	fastPathCleanupCoefficient(ctx, decoder, zeroContexts, column, flags, first, false, true)
	for i := first + 1; i < 4; i++ {
		fastPathCleanupCoefficient(ctx, decoder, zeroContexts, column, flags, i, false, false)
	}
}

// fastPathCleanupCoefficient is fast_path::cleanup_coefficient.
func fastPathCleanupCoefficient(ctx *bitPlaneDecodeContext, decoder *arithmeticDecoder, zeroContexts *[512]uint8, column stripeColumn, flags *uint32, inStripe uint32, checkFlags, alreadyKnownSignificant bool) {
	shift := inStripe * flagsPerCoefficient

	if checkFlags && *flags&((flagSignificantThis|flagZeroCodedThis)<<shift) != 0 {
		return
	}
	if !alreadyKnownSignificant {
		label := zeroContexts[(*flags>>shift)&flagSignificantNeighbors]
		if decoder.readBit(&ctx.contexts[label]) == 0 {
			return
		}
	}

	idx := column.coefficientIdx + int(inStripe)*column.stride
	fastPathPushMagnitudeBit(ctx, idx, 1)
	sign := fastPathDecodeSignBit(ctx, decoder, column, *flags, inStripe)
	fastPathSetSign(ctx, idx, uint8(sign))
	fastPathSetSignificant(ctx, column, flags, inStripe, sign)
}

// fastPathSignificancePropagationPass is
// fast_path::significance_propagation_pass.
func fastPathSignificancePropagationPass(ctx *bitPlaneDecodeContext, decoder *arithmeticDecoder) {
	zeroContexts := fastPathZeroContextLookup(ctx.subBandType)

	fastPathForEachStripeColumn(ctx, func(ctx *bitPlaneDecodeContext, column stripeColumn) uint32 {
		flags := column.flags
		if flags != 0 {
			for i := range uint32(4) {
				fastPathSignificancePropagationCoefficient(ctx, decoder, zeroContexts, column, &flags, i)
			}
		}
		return flags
	})
}

// fastPathSignificancePropagationCoefficient is
// fast_path::significance_propagation_coefficient.
func fastPathSignificancePropagationCoefficient(ctx *bitPlaneDecodeContext, decoder *arithmeticDecoder, zeroContexts *[512]uint8, column stripeColumn, flags *uint32, inStripe uint32) {
	shift := inStripe * flagsPerCoefficient
	shifted := *flags >> shift
	if shifted&(flagSignificantThis|flagZeroCodedThis) != 0 || shifted&flagSignificantNeighbors == 0 {
		return
	}

	label := zeroContexts[shifted&flagSignificantNeighbors]
	bit := decoder.readBit(&ctx.contexts[label])
	*flags |= flagZeroCodedThis << shift

	if bit == 1 {
		idx := column.coefficientIdx + int(inStripe)*column.stride
		fastPathPushMagnitudeBit(ctx, idx, bit)
		sign := fastPathDecodeSignBit(ctx, decoder, column, *flags, inStripe)
		fastPathSetSign(ctx, idx, uint8(sign))
		fastPathSetSignificant(ctx, column, flags, inStripe, sign)
	}
}

// fastPathMagnitudeRefinementPass is fast_path::magnitude_refinement_pass.
func fastPathMagnitudeRefinementPass(ctx *bitPlaneDecodeContext, decoder *arithmeticDecoder) {
	fastPathForEachStripeColumn(ctx, func(ctx *bitPlaneDecodeContext, column stripeColumn) uint32 {
		flags := column.flags
		if flags != 0 {
			for i := range uint32(4) {
				fastPathMagnitudeRefinementCoefficient(ctx, decoder, column, &flags, i)
			}
		}
		return flags
	})
}

// fastPathMagnitudeRefinementCoefficient is
// fast_path::magnitude_refinement_coefficient.
func fastPathMagnitudeRefinementCoefficient(ctx *bitPlaneDecodeContext, decoder *arithmeticDecoder, column stripeColumn, flags *uint32, inStripe uint32) {
	shift := inStripe * flagsPerCoefficient
	shifted := *flags >> shift
	if shifted&(flagSignificantThis|flagZeroCodedThis) != flagSignificantThis {
		return
	}

	label := fastPathMagnitudeRefinementContext(shifted)
	bit := decoder.readBit(&ctx.contexts[label])
	idx := column.coefficientIdx + int(inStripe)*column.stride
	if bit == 1 {
		fastPathPushMagnitudeBit(ctx, idx, bit)
	}
	ctx.coefficientStates[idx].setDecodedBitPosition(ctx.currentBitPosition)
	*flags |= flagMagnitudeRefinedThis << shift
}

// fastPathDecodeSignBit is fast_path::decode_sign_bit.
func fastPathDecodeSignBit(ctx *bitPlaneDecodeContext, decoder *arithmeticDecoder, column stripeColumn, flags, inStripe uint32) uint32 {
	shift := inStripe * flagsPerCoefficient
	shifted := flags >> shift
	leftFlags := ctx.stripeFlags[column.flagIdx-1]
	rightFlags := ctx.stripeFlags[column.flagIdx+1]

	var topSign uint32
	if inStripe == 0 {
		topSign = flags >> flagSignTopStripeShift & 1
	} else {
		topSign = flags >> (flagSignThisShift + (inStripe-1)*3) & 1
	}
	bottomSign := flags >> (flagSignBottomThisShift + inStripe*flagsPerCoefficient) & 1
	leftSign := leftFlags >> (flagSignThisShift + shift) & 1
	rightSign := rightFlags >> (flagSignThisShift + shift) & 1

	lookup := leftSign |
		b2u32(shifted&flagSignificantTop != 0)<<1 |
		rightSign<<2 |
		b2u32(shifted&flagSignificantLeft != 0)<<3 |
		topSign<<4 |
		b2u32(shifted&flagSignificantRight != 0)<<5 |
		bottomSign<<6 |
		b2u32(shifted&flagSignificantBottom != 0)<<7

	e := signContextStripeLookup[lookup]
	return decoder.readBit(&ctx.contexts[e[0]]) ^ uint32(e[1])
}

// b2u32 is the conversion `b as u32` of Rust for a boolean.
func b2u32(b bool) uint32 {
	if b {
		return 1
	}
	return 0
}

// fastPathZeroContextLookup is fast_path::zero_context_lookup.
func fastPathZeroContextLookup(t subBandType) *[512]uint8 {
	switch t {
	case highLow:
		return &zeroCtxHLStripeLookup
	case highHigh:
		return &zeroCtxHHStripeLookup
	}
	return &zeroCtxLLLHStripeLookup
}

// fastPathMagnitudeRefinementContext is
// fast_path::magnitude_refinement_context.
func fastPathMagnitudeRefinementContext(flags uint32) uint8 {
	if flags&flagMagnitudeRefinedThis != 0 {
		return 16
	}
	return 14 + uint8(b2u32(flags&flagSignificantNeighbors != 0))
}

// fastPathResetForNextBitplane is fast_path::reset_for_next_bitplane.
func fastPathResetForNextBitplane(ctx *bitPlaneDecodeContext) {
	for i := range ctx.stripeFlags {
		ctx.stripeFlags[i] &^= flagZeroCodedAll
	}
}

// stripeColumn is StripeColumn in the module fast_path.
type stripeColumn struct {
	flagIdx        int
	coefficientIdx int
	flags          uint32
	stride         int
}

// fastPathForEachStripeColumn is fast_path::for_each_stripe_column. It
// calls action for each column of four coefficients, and stores the flags
// that action returns.
func fastPathForEachStripeColumn(ctx *bitPlaneDecodeContext, action func(*bitPlaneDecodeContext, stripeColumn) uint32) {
	width := int(ctx.width)
	stride := int(ctx.paddedWidth)
	stripeCount := int(ctx.height / 4)
	flagBase := coefficientsPadding*stride + coefficientsPadding
	coefficientBase := coefficientsPadding*stride + coefficientsPadding

	for range stripeCount {
		for x := range width {
			flagIdx := flagBase + x
			column := stripeColumn{
				flagIdx:        flagIdx,
				coefficientIdx: coefficientBase + x,
				flags:          ctx.stripeFlags[flagIdx],
				stride:         stride,
			}
			ctx.stripeFlags[flagIdx] = action(ctx, column)
		}
		flagBase += stride
		coefficientBase += 4 * stride
	}
}

// fastPathPushMagnitudeBit is fast_path::push_magnitude_bit.
func fastPathPushMagnitudeBit(ctx *bitPlaneDecodeContext, idx int, bit uint32) {
	ctx.coefficients[idx].pushBitAt(bit, ctx.currentBitPosition)
}

// fastPathSetSign is fast_path::set_sign.
func fastPathSetSign(ctx *bitPlaneDecodeContext, idx int, sign uint8) {
	ctx.coefficients[idx].setSign(sign)
}

// fastPathSetSignificant is fast_path::set_significant.
func fastPathSetSignificant(ctx *bitPlaneDecodeContext, column stripeColumn, flags *uint32, inStripe, sign uint32) {
	shift := inStripe * flagsPerCoefficient
	idx := column.coefficientIdx + int(inStripe)*column.stride
	ctx.coefficientStates[idx].setDecodedBitPosition(ctx.currentBitPosition)

	ctx.stripeFlags[column.flagIdx-1] |= flagSignificantRight << shift
	*flags |= (sign<<flagSignThisShift | flagSignificantThis) << shift
	ctx.stripeFlags[column.flagIdx+1] |= flagSignificantLeft << shift

	if inStripe == 0 {
		top := column.flagIdx - column.stride
		ctx.stripeFlags[top] |= sign<<flagSignBottomStripeShift | flagSignificantBottom<<lastCoefficientShift
		ctx.stripeFlags[top-1] |= flagSignificantBottomRight << lastCoefficientShift
		ctx.stripeFlags[top+1] |= flagSignificantBottomLeft << lastCoefficientShift
	}

	if inStripe == 3 {
		bottom := column.flagIdx + column.stride
		ctx.stripeFlags[bottom] |= sign<<flagSignTopStripeShift | flagSignificantTop
		ctx.stripeFlags[bottom-1] |= flagSignificantTopRight
		ctx.stripeFlags[bottom+1] |= flagSignificantTopLeft
	}
}

// The flags of a column of the fast path. The flags of lane n are the
// flags below, shifted by 3n. Bits 18 and 31 hold the signs above and below the
// stripe.
const (
	flagsPerCoefficient  = 3
	lastCoefficientShift = 3 * flagsPerCoefficient

	flagSignificantTopLeft     = 1 << 0
	flagSignificantTop         = 1 << 1
	flagSignificantTopRight    = 1 << 2
	flagSignificantLeft        = 1 << 3
	flagSignificantThis        = 1 << 4
	flagSignificantRight       = 1 << 5
	flagSignificantBottomLeft  = 1 << 6
	flagSignificantBottom      = 1 << 7
	flagSignificantBottomRight = 1 << 8
	flagSignificantNeighbors   = flagSignificantTopLeft | flagSignificantTop | flagSignificantTopRight |
		flagSignificantLeft | flagSignificantRight | flagSignificantBottomLeft |
		flagSignificantBottom | flagSignificantBottomRight

	flagSignTopStripeShift    = 18
	flagSignThisShift         = 19
	flagMagnitudeRefinedThis  = 1 << 20
	flagZeroCodedThis         = 1 << 21
	flagSignBottomThisShift   = 22
	flagSignBottomStripeShift = 31
	flagZeroCodedAll          = 1<<21 | 1<<24 | 1<<27 | 1<<30
)

// signContextStripeLookup is SIGN_CONTEXT_STRIPE_LOOKUP in the module
// fast_path, from build_sign_context_lookup.
var signContextStripeLookup = func() [256][2]uint8 {
	var out [256][2]uint8
	for lookup := range 256 {
		leftSign := lookup & 1
		topSignificance := lookup >> 1 & 1
		rightSign := lookup >> 2 & 1
		leftSignificance := lookup >> 3 & 1
		topSign := lookup >> 4 & 1
		rightSignificance := lookup >> 5 & 1
		bottomSign := lookup >> 6 & 1
		bottomSignificance := lookup >> 7 & 1

		significances := topSignificance<<6 | leftSignificance<<4 | rightSignificance<<2 | bottomSignificance
		signs := topSign<<6 | leftSign<<4 | rightSign<<2 | bottomSign
		merged := (significances&signs)<<1 | significances&^signs
		out[lookup] = signContextLookup[merged]
	}
	return out
}()

// The zero coding tables of the fast path, from build_zero_context_lookup.
var (
	zeroCtxLLLHStripeLookup = fastPathBuildZeroContextLookup(&zeroCtxLLLHLookup)
	zeroCtxHLStripeLookup   = fastPathBuildZeroContextLookup(&zeroCtxHLLookup)
	zeroCtxHHStripeLookup   = fastPathBuildZeroContextLookup(&zeroCtxHHLookup)
)

// fastPathBuildZeroContextLookup is fast_path::build_zero_context_lookup.
func fastPathBuildZeroContextLookup(source *[256]uint8) [512]uint8 {
	var out [512]uint8
	for flags := range 512 {
		out[flags] = source[fastPathNeighborByte(flags)]
	}
	return out
}

// fastPathNeighborByte is fast_path::neighbor_byte.
func fastPathNeighborByte(flags int) int {
	bit := func(mask int) int {
		if flags&mask != 0 {
			return 1
		}
		return 0
	}
	return bit(flagSignificantTopLeft)<<7 |
		bit(flagSignificantTop)<<6 |
		bit(flagSignificantTopRight)<<5 |
		bit(flagSignificantLeft)<<4 |
		bit(flagSignificantBottomLeft)<<3 |
		bit(flagSignificantRight)<<2 |
		bit(flagSignificantBottomRight)<<1 |
		bit(flagSignificantBottom)
}
