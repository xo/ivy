// Copyright (c) The Hayro Authors
//
// Use of this source code is governed by the Apache-2.0 or MIT license that
// can be found in the LICENSE-APACHE and LICENSE-MIT files.

package jpeg2000

// This file ports hayro-jpeg2000 src/j2c/decode.rs.

// j2cDecode is decode in hayro-jpeg2000 src/j2c/decode.rs. It decodes the
// tiles of the codestream into the components of the context.
func j2cDecode(data []byte, h *header, ctx *DecoderContext, c cancel, lim *limits) error {
	reader := newBitReader(data)
	tiles, err := tileParse(&reader, h, lim)
	if err != nil {
		return err
	}
	if len(tiles) == 0 {
		return errTileInvalid
	}

	if err := ctx.reset(h, &tiles[0], c, lim); err != nil {
		return err
	}

	for i := range tiles {
		t := &tiles[i]
		if err := c.check(); err != nil {
			return err
		}
		it, err := newProgressionIterator(t, h, lim)
		if err != nil {
			return err
		}
		if err := decodeTile(t, h, it, &ctx.tileDecodeContext, ctx.channelData, &ctx.storage, c); err != nil {
			return err
		}
	}

	// Assumes either all tiles or none use MCT.
	if tiles[0].mct {
		if err := applyInverse(ctx.channelData, tiles[0].componentInfos, h, c); err != nil {
			return err
		}
	}

	return applySignShift(ctx.channelData, h.componentInfos, c)
}

// DecoderContext is DecoderContext in hayro-jpeg2000 src/j2c/decode.rs. It
// holds the buffers of the decoder, which a program can reuse from one
// image to the next. The zero value is ready to use.
type DecoderContext struct {
	tileDecodeContext tileDecodeContext
	channelData       []ComponentData
	storage           decompositionStorage
}

// reset is DecoderContext::reset.
func (ctx *DecoderContext) reset(h *header, initialTile *tile, c cancel, lim *limits) error {
	ctx.tileDecodeContext.reset()
	ctx.storage.reset()
	ctx.storage.lim = lim

	ctx.channelData = ctx.channelData[:0]
	sampleCount := uint64(h.sizeData.imageWidth()) * uint64(h.sizeData.imageHeight())
	// Port: the limits bound the size of the image and the memory of its
	// components.
	if err := lim.pixels(sampleCount); err != nil {
		return err
	}
	if err := lim.memory(sampleCount * 4 * uint64(len(initialTile.componentInfos))); err != nil {
		return err
	}
	// TODO: Reuse the SIMD buffers across runs.
	for i := range initialTile.componentInfos {
		if err := c.check(); err != nil {
			return err
		}
		ctx.channelData = append(ctx.channelData, ComponentData{
			container: simdBufferZeros(int(sampleCount)),
			bitDepth:  initialTile.componentInfos[i].sizeInfo.precision,
		})
	}
	return nil
}

// decodeTile is decode_tile in hayro-jpeg2000 src/j2c/decode.rs.
func decodeTile(t *tile, h *header, it progressionIter, tileCtx *tileDecodeContext, channelData []ComponentData, storage *decompositionStorage, c cancel) error {
	storage.reset()

	if err := build(t, storage, h.skippedResolutionLevels, c); err != nil {
		return err
	}
	if err := segmentParse(t, it, h, storage, c); err != nil {
		return err
	}

	for idx := range h.componentInfos {
		info := &h.componentInfos[idx]
		if err := c.check(); err != nil {
			return err
		}
		// Port: upstream indexes the components of the tile and of the
		// context with the index of the main header, and panics when they
		// differ.
		if idx >= len(storage.tileDecompositions) || idx >= len(channelData) || idx >= len(t.componentInfos) {
			return errInternal
		}

		if idx > 0 {
			count := storage.tileDecompositions[idx].coefficientCount
			clear(storage.coefficients[:count])
		}
		if err := decodeComponentTileBitPlanes(idx, t, tileCtx, storage, h, c); err != nil {
			return err
		}
		if err := idwtApply(storage, tileCtx, idx, h, info.waveletTransform(), c); err != nil {
			return err
		}
		// MCT is deferred until all tiles are stored so IDWT buffers can be
		// reused per component.
		if err := store(t, h, tileCtx, &channelData[idx], info, c); err != nil {
			return err
		}
	}
	return nil
}

// tileDecompositions is TileDecompositions in hayro-jpeg2000
// src/j2c/decode.rs.
type tileDecompositions struct {
	firstLLSubBand   int
	decompositions   span
	coefficientCount int
}

// subBandIter is TileDecompositions::sub_band_iter. It returns the indices
// of the sub-bands of the resolution: the low-low sub-band for resolution
// 0, and the three other sub-bands otherwise.
//
// Port: the iterator of upstream becomes a slice. It returns false for a
// resolution that does not exist, where upstream panics.
func (td *tileDecompositions) subBandIter(resolution uint8, decompositions []decomposition) ([]int, bool) {
	if resolution == 0 {
		return []int{td.firstLLSubBand}, true
	}
	i := td.decompositions.start + int(resolution) - 1
	if i >= td.decompositions.end || i >= len(decompositions) {
		return nil, false
	}
	sb := decompositions[i].subBands
	return sb[:], true
}

// decompositionStorage is DecompositionStorage in hayro-jpeg2000
// src/j2c/decode.rs.
type decompositionStorage struct {
	segments           []segment
	layers             []layer
	codeBlocks         []codeBlock
	precincts          []precinct
	tagTreeNodes       []tagNode
	coefficients       []float32
	subBands           []subBand
	decompositions     []decomposition
	tileDecompositions []tileDecompositions

	// Port: lim bounds the work on the image, and used counts the bytes
	// that the build of one tile takes.
	lim  *limits
	used uint64
}

// reset is DecompositionStorage::reset.
func (s *decompositionStorage) reset() {
	s.segments = s.segments[:0]
	s.layers = s.layers[:0]
	s.codeBlocks = s.codeBlocks[:0]
	s.precincts = s.precincts[:0]
	s.subBands = s.subBands[:0]
	s.decompositions = s.decompositions[:0]
	s.tileDecompositions = s.tileDecompositions[:0]
	s.tagTreeNodes = s.tagTreeNodes[:0]
	s.used = 0
}

// The approximate sizes in bytes of the parts of a precinct, for the
// limits.
const (
	sizePrecinct  = 64
	sizeCodeBlock = 64
	sizeLayer     = 24
	sizeTagNode   = 48
)

// grow charges the storage of one precinct with codeBlocks code-blocks of
// numLayers layers each, and of its two tag trees, to the limits.
//
// Port: upstream has no limit. A malformed geometry can make it build
// billions of code-blocks.
func (s *decompositionStorage) grow(codeBlocks, numLayers uint64) error {
	n := sizePrecinct + codeBlocks*(sizeCodeBlock+numLayers*sizeLayer) + 2*(2*codeBlocks+64)*sizeTagNode
	s.used += n
	if s.used < n || codeBlocks > 1<<40 {
		return errImageTooLarge
	}
	return s.lim.memory(s.used)
}

// tileDecodeContext is TileDecodeContext in hayro-jpeg2000
// src/j2c/decode.rs.
type tileDecodeContext struct {
	idwtOutput            idwtOutput
	idwtScratchBuffer     []float32
	bitPlaneDecodeContext bitPlaneDecodeContext
	bitPlaneDecodeBuffers bitPlaneDecodeBuffers
}

// reset is TileDecodeContext::reset. It does nothing.
func (ctx *tileDecodeContext) reset() {}

// decodeComponentTileBitPlanes is decode_component_tile_bit_planes in
// hayro-jpeg2000 src/j2c/decode.rs.
func decodeComponentTileBitPlanes(componentIdx int, t *tile, tileCtx *tileDecodeContext, storage *decompositionStorage, h *header, c cancel) error {
	info := &t.componentInfos[componentIdx]

	levels := info.numResolutionLevels() - h.skippedResolutionLevels
	for resolution := range levels {
		td := &storage.tileDecompositions[componentIdx]
		sbs, ok := td.subBandIter(resolution, storage.decompositions)
		if !ok {
			return errInternal
		}
		for _, sb := range sbs {
			if err := decodeSubBandBitplanes(sb, resolution, info, tileCtx, storage, h, c); err != nil {
				return err
			}
		}
	}
	return nil
}

// decodeSubBandBitplanes is decode_sub_band_bitplanes in hayro-jpeg2000
// src/j2c/decode.rs.
func decodeSubBandBitplanes(sbIdx int, resolution uint8, info *componentInfo, tileCtx *tileDecodeContext, storage *decompositionStorage, h *header, c cancel) error {
	sb := &storage.subBands[sbIdx]

	quantised := info.quantizationInfo.quantizationStyle != noQuantization
	irreversible := info.waveletTransform() == irreversible97

	dequantizationStep := float32(1)
	if quantised {
		exponent, mantissa, err := info.exponentMantissa(sb.subBandType, resolution)
		if err != nil {
			return err
		}
		var logGain uint16
		switch sb.subBandType {
		case lowHigh, highLow:
			logGain = 1
		case highHigh:
			logGain = 2
		}
		rB := uint16(info.sizeInfo.precision) + logGain
		dequantizationStep = pow2i(int32(rB)-int32(exponent)) * (1.0 + float32(mantissa)/2048.0)
	}

	exponent, _, err := info.exponentMantissa(sb.subBandType, resolution)
	if err != nil {
		return err
	}
	nb := uint32(info.quantizationInfo.guardBits) + uint32(exponent)
	if nb == 0 || nb > 0xffff {
		return errInvalidBitplaneCount
	}
	nb = nb - 1 + uint32(info.roiShift)
	if nb > 0xffff {
		return errInvalidBitplaneCount
	}
	if nb > maxBitplaneCount {
		return errTooManyBitplanes
	}
	numBitplanes := uint8(nb)

	if sb.coefficients.end > len(storage.coefficients) {
		return errInternal
	}
	baseStore := storage.coefficients[sb.coefficients.start:sb.coefficients.end]
	sbWidth := int(sb.rect.width())

	for pi := sb.precincts.start; pi < sb.precincts.end; pi++ {
		p := &storage.precincts[pi]
		for ci := p.codeBlocks.start; ci < p.codeBlocks.end; ci++ {
			cb := &storage.codeBlocks[ci]
			if err := c.check(); err != nil {
				return err
			}

			style := info.codingStyle.parameters.codeBlockStyle
			if err := bitplaneDecode(cb, sb.subBandType, numBitplanes, &style, tileCtx, storage, h.strict); err != nil {
				return err
			}

			xOffset := cb.rect.x0 - sb.rect.x0
			yOffset := cb.rect.y0 - sb.rect.y0
			baseIdx := int(yOffset*sb.rect.width()) + int(xOffset)

			var failed bool
			tileCtx.bitPlaneDecodeContext.coefficientRows(func(coefficients []coefficient, states []coefficientState) {
				// Port: upstream slices the row from baseIdx, and panics
				// past the end. The zip of upstream stops at the shorter
				// side, so the port also stops there.
				if failed || baseIdx < 0 || baseIdx > len(baseStore) {
					failed = true
					return
				}
				out := baseStore[baseIdx:]
				n := min(len(out), len(coefficients))
				for i := range n {
					v := coefficients[i].reconstructed(states[i], irreversible, info.roiShift)
					out[i] = v * dequantizationStep
				}
				baseIdx += sbWidth
			})
			if failed {
				return errInternal
			}
		}
	}
	return nil
}

// applySignShift is apply_sign_shift in hayro-jpeg2000 src/j2c/decode.rs.
// It adds the offset of the unsigned samples to each component.
func applySignShift(channelData []ComponentData, infos []componentInfo, c cancel) error {
	n := min(len(channelData), len(infos))
	for i := range n {
		if err := c.check(); err != nil {
			return err
		}
		offset := float32(shl32(1, uint32(infos[i].sizeInfo.precision)-1))
		data := channelData[i].container.data
		blocks := len(data) / simdWidth * simdWidth
		for j := range data[:blocks] {
			data[j] += offset
		}
	}
	return nil
}

// store is store in hayro-jpeg2000 src/j2c/decode.rs. It copies the output
// of the inverse wavelet transform of one component of a tile into the
// component of the image.
//
// Port: upstream slices the buffers with the geometry of the tile, and
// panics when the geometry does not fit. The port returns an error.
func store(t *tile, h *header, tileCtx *tileDecodeContext, channel *ComponentData, info *componentInfo, c cancel) error {
	out := &tileCtx.idwtOutput

	ct := newComponentTile(t, info)
	rt, err := newResolutionTile(ct, info.numResolutionLevels()-1-h.skippedResolutionLevels)
	if err != nil {
		return err
	}

	if out.rect.width() == 0 || out.rect.height() == 0 {
		return nil
	}

	scaleX := info.sizeInfo.horizontalResolution
	scaleY := info.sizeInfo.verticalResolution
	imageXOffset := h.sizeData.imageXOffset()
	imageYOffset := h.sizeData.imageYOffset()
	satSub := func(a, b uint32) uint32 {
		if a < b {
			return 0
		}
		return a - b
	}

	if scaleX == 1 && scaleY == 1 {
		skipX := int(satSub(imageXOffset, out.rect.x0))
		skipY := int(satSub(imageYOffset, out.rect.y0))
		inWidth := int(out.rect.width())
		inHeight := int(out.rect.height())
		outWidth := int(h.sizeData.imageWidth())
		if inWidth == 0 || outWidth == 0 {
			return nil
		}
		inRows := len(out.coefficients) / inWidth
		outRows := len(channel.container.data) / outWidth
		outRow := int(satSub(rt.rect.y0, imageYOffset))
		outCol := int(satSub(rt.rect.x0, imageXOffset))

		for y := skipY; y < inRows && y < skipY+inHeight && outRow < outRows; y++ {
			if err := c.check(); err != nil {
				return err
			}
			if skipX > inWidth {
				return errInternal
			}
			inRow := out.coefficients[y*inWidth+skipX : (y+1)*inWidth]
			if outCol+len(inRow) > outWidth {
				return errInternal
			}
			copy(channel.container.data[outRow*outWidth+outCol:], inRow)
			outRow++
		}
		return nil
	}

	imageWidth := h.sizeData.imageWidth()
	imageHeight := h.sizeData.imageHeight()
	xShrink := h.sizeData.xShrinkFactor
	yShrink := h.sizeData.yShrinkFactor

	firstX, hasFirstX := rt.rect.x0, t.rect.x0 == h.sizeData.imageAreaXOffset
	firstY, hasFirstY := rt.rect.y0, t.rect.y0 == h.sizeData.imageAreaYOffset
	outW := int(out.rect.width())

	for y := rt.rect.y0; y < rt.rect.y1; y++ {
		if err := c.check(); err != nil {
			return err
		}

		relY := int(y - out.rect.y0)
		gridY := uint32(scaleY) * y / yShrink
		if hasFirstY && firstY == y {
			gridY = imageYOffset
		}
		yEnd := uint32(scaleY) * (y + 1) / yShrink

		for x := rt.rect.x0; x < rt.rect.x1; x++ {
			relX := int(x - out.rect.x0)
			gridX := uint32(scaleX) * x / xShrink
			if hasFirstX && firstX == x {
				gridX = imageXOffset
			}
			xEnd := uint32(scaleX) * (x + 1) / xShrink

			si := relY*outW + relX
			if relX < 0 || relY < 0 || si < 0 || si >= len(out.coefficients) {
				return errInternal
			}
			sample := out.coefficients[si]

			for xp := max(gridX, imageXOffset); xp < min(xEnd, imageWidth+imageXOffset); xp++ {
				for yp := max(gridY, imageYOffset); yp < min(yEnd, imageHeight+imageYOffset); yp++ {
					pos := int(yp-imageYOffset)*int(imageWidth) + int(xp-imageXOffset)
					if pos >= len(channel.container.data) {
						return errInternal
					}
					channel.container.data[pos] = sample
				}
			}
		}
	}
	return nil
}
