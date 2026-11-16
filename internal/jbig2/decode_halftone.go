// Copyright (c) The Hayro Authors
//
// Use of this source code is governed by the Apache-2.0 or MIT license that
// can be found in the LICENSE-APACHE and LICENSE-MIT files.

package jbig2

// This file ports hayro-jbig2 src/decode/halftone.rs, the halftone region
// decoding procedure of T.88 6.6.
//
// Port: upstream writes the copy of a pattern as the macro blit_rows!, with
// a closure for each combination operator. Go writes it as the function
// blitRows, with the operator as an argument.

import "math/bits"

// decodeHalftoneRegion is decode in hayro-jbig2 src/decode/halftone.rs.
func decodeHalftoneRegion(header *halftoneRegionHeader, patternDict *patternDictionary, ctx *decodeState) (regionBitmap, error) {
	region := &header.regionInfo

	htreg, err := newBitmapWith(region.width, region.height, region.xLocation, region.yLocation,
		header.flags.initialPixelColor, &ctx.budget)
	if err != nil {
		return regionBitmap{}, err
	}

	if err := decodeHalftoneRegionInto(header, patternDict, htreg, ctx); err != nil {
		return regionBitmap{}, err
	}

	return regionBitmap{bitmap: htreg, combinationOperator: region.combinationOperator}, nil
}

// decodeHalftoneRegionInto is decode_into in hayro-jbig2
// src/decode/halftone.rs.
func decodeHalftoneRegionInto(header *halftoneRegionHeader, patternDict *patternDictionary, htreg *bitmap, ctx *decodeState) error {
	var skipBitmap []uint32
	if header.flags.enableSkip {
		var err error
		skipBitmap, err = computeSkipBitmap(header, patternDict, htreg, &ctx.budget, ctx.cancel)
		if err != nil {
			return err
		}
	}

	// The number of bits of the largest gray-scale value, and at least 1.
	bitsPerPixel := uint32(1)
	if n := uint32(len(patternDict.patterns)); n > 1 {
		bitsPerPixel = uint32(bits.Len32(n - 1))
	}

	params := grayScaleParams{
		useMMR:       header.flags.mmr,
		bitsPerPixel: bitsPerPixel,
		width:        header.gridPositionAndSize.width,
		height:       header.gridPositionAndSize.height,
		template:     header.flags.template,
		skipMask:     skipBitmap,
		strict:       header.strict,
	}
	gi, err := decodeGrayScaleImage(header.data, &params, ctx)
	if err != nil {
		return err
	}

	return renderPatterns(htreg, gi, header, patternDict, &ctx.budget, ctx.cancel)
}

// parseHalftoneRegion is parse in hayro-jbig2 src/decode/halftone.rs.
func parseHalftoneRegion(r *reader, strict bool) (*halftoneRegionHeader, error) {
	regionInfo, err := parseRegionSegmentInfo(r)
	if err != nil {
		return nil, err
	}
	flagsByte, ok := r.readByte()
	if !ok {
		return nil, ErrUnexpectedEOF
	}
	op, err := combinationOperatorFromValue(flagsByte >> 4)
	if err != nil {
		return nil, err
	}

	flags := halftoneRegionFlags{
		mmr:                 flagsByte&0x01 != 0,
		template:            templateFromByte(flagsByte >> 1),
		enableSkip:          flagsByte&0x08 != 0,
		combinationOperator: op,
		initialPixelColor:   flagsByte&0x80 != 0,
	}

	gridWidth, ok1 := r.readU32()
	gridHeight, ok2 := r.readU32()
	if !ok1 || !ok2 {
		return nil, ErrUnexpectedEOF
	}

	if gridWidth > maxDimension || gridHeight > maxDimension {
		return nil, ErrOverflowGridDimension
	}

	gridHorizontalOffset, ok1 := r.readI32()
	gridVerticalOffset, ok2 := r.readI32()
	if !ok1 || !ok2 {
		return nil, ErrUnexpectedEOF
	}

	gridXVector, ok1 := r.readU16()
	gridYVector, ok2 := r.readU16()
	if !ok1 || !ok2 {
		return nil, ErrUnexpectedEOF
	}

	data, ok := r.tail()
	if !ok {
		return nil, ErrUnexpectedEOF
	}

	return &halftoneRegionHeader{
		regionInfo: regionInfo,
		flags:      flags,
		gridPositionAndSize: halftoneGridPositionAndSize{
			width:            gridWidth,
			height:           gridHeight,
			horizontalOffset: gridHorizontalOffset,
			verticalOffset:   gridVerticalOffset,
		},
		gridVector: halftoneGridVector{xVector: gridXVector, yVector: gridYVector},
		data:       data,
		strict:     strict,
	}, nil
}

// halftoneRegionFlags is HalftoneRegionFlags in hayro-jbig2
// src/decode/halftone.rs.
type halftoneRegionFlags struct {
	mmr                 bool
	template            template
	enableSkip          bool
	combinationOperator combinationOperator
	initialPixelColor   bool
}

// halftoneGridPositionAndSize is HalftoneGridPositionAndSize in hayro-jbig2
// src/decode/halftone.rs.
type halftoneGridPositionAndSize struct {
	width            uint32
	height           uint32
	horizontalOffset int32
	verticalOffset   int32
}

// halftoneGridVector is HalftoneGridVector in hayro-jbig2
// src/decode/halftone.rs.
type halftoneGridVector struct {
	xVector uint16
	yVector uint16
}

// halftoneRegionHeader is HalftoneRegionHeader in hayro-jbig2
// src/decode/halftone.rs.
type halftoneRegionHeader struct {
	regionInfo          regionSegmentInfo
	flags               halftoneRegionFlags
	gridPositionAndSize halftoneGridPositionAndSize
	gridVector          halftoneGridVector
	data                []byte
	strict              bool
}

// gridCoords is GridCoords in hayro-jbig2 src/decode/halftone.rs. It walks
// the cells of the grid, in units of 1/256 pixel.
type gridCoords struct {
	x    int64
	y    int64
	rowX int64
	rowY int64
	hrx  int64
	hry  int64
}

// newGridCoords is GridCoords::new.
func newGridCoords(grid *halftoneGridPositionAndSize, vector *halftoneGridVector) gridCoords {
	return gridCoords{
		x:    int64(grid.horizontalOffset),
		y:    int64(grid.verticalOffset),
		rowX: int64(grid.horizontalOffset),
		rowY: int64(grid.verticalOffset),
		hrx:  int64(vector.xVector),
		hry:  int64(vector.yVector),
	}
}

// get is GridCoords::get. It returns the pixel of the current cell.
func (g *gridCoords) get() (int64, int64) {
	return g.x >> 8, g.y >> 8
}

// advanceCol is GridCoords::advance_col.
func (g *gridCoords) advanceCol() {
	g.x += g.hrx
	g.y -= g.hry
}

// advanceRow is GridCoords::advance_row.
func (g *gridCoords) advanceRow() {
	g.rowX += g.hry
	g.rowY += g.hrx
	g.x = g.rowX
	g.y = g.rowY
}

// computeSkipBitmap is compute_skip_bitmap in hayro-jbig2
// src/decode/halftone.rs. A bit of 1 marks a cell whose pattern lies outside
// the region.
func computeSkipBitmap(header *halftoneRegionHeader, patternDict *patternDictionary, htreg *bitmap,
	budget *memoryBudget, c cancel) ([]uint32, error) {
	grid := &header.gridPositionAndSize

	stride := divCeil(grid.width, 32)
	n := uint64(stride) * uint64(grid.height)
	if n > uint64(^uint32(0)) {
		return nil, ErrOverflowBitmapDimension
	}
	if err := budget.charge(int(n) * 4); err != nil {
		return nil, err
	}
	hskip := make([]uint32, n)
	coords := newGridCoords(grid, &header.gridVector)
	var work workCounter

	for mg := range grid.height {
		for ng := range grid.width {
			if err := work.add(1, c); err != nil {
				return nil, err
			}
			x, y := coords.get()

			if liesOutside(x, y, patternDict, htreg) {
				i := int(mg*stride + ng/32)
				bitPos := 31 - ng%32
				hskip[i] |= 1 << bitPos
			}

			coords.advanceCol()
		}
		coords.advanceRow()
	}

	return hskip, nil
}

// liesOutside is lies_outside in hayro-jbig2 src/decode/halftone.rs.
func liesOutside(x, y int64, patternDict *patternDictionary, region *bitmap) bool {
	return x+int64(patternDict.patternWidth) <= 0 ||
		x >= int64(region.width) ||
		y+int64(patternDict.patternHeight) <= 0 ||
		y >= int64(region.height)
}

// renderPatterns is render_patterns in hayro-jbig2 src/decode/halftone.rs. It
// draws the pattern of each cell of the grid into the region.
func renderPatterns(region *bitmap, gi *grayScaleImage, header *halftoneRegionHeader, patternDict *patternDictionary,
	budget *memoryBudget, c cancel) error {
	grid := &header.gridPositionAndSize
	op := header.flags.combinationOperator
	pw := patternDict.patternWidth
	ph := patternDict.patternHeight
	stride := region.stride
	numPatterns := len(patternDict.patterns)

	originalMask := ^uint32(0)
	if pw < wordBits {
		originalMask = ^uint32(0) << (wordBits - pw)
	}

	coords := newGridCoords(grid, &header.gridVector)
	if err := budget.charge(int(grid.width) * 4); err != nil {
		return err
	}
	grayValues := make([]uint32, grid.width)
	var work workCounter
	cellWork := 1 + uint64(wordWork)*uint64(ph)*uint64(divCeil(pw, wordBits))

	for mg := range grid.height {
		gi.rowValues(mg, grayValues)

		for _, grayValue := range grayValues {
			if err := work.add(cellWork, c); err != nil {
				return err
			}
			x, y := coords.get()
			gv := int(grayValue)

			if header.strict && gv >= numPatterns {
				return ErrGrayScaleOutOfRange
			}

			pattern := patternDict.patterns[min(gv, numPatterns-1)]

			if liesOutside(x, y, patternDict, region) {
				coords.advanceCol()
				continue
			}

			if pw <= wordBits &&
				x >= 0 &&
				y >= 0 &&
				x+int64(pw) <= int64(region.width) &&
				y+int64(ph) <= int64(region.height) {
				xu := uint32(x)
				yu := uint32(y)
				firstWord := xu / wordBits
				offset := xu % wordBits
				two := offset != 0 && firstWord+1 < stride

				var mask0, mask1 word
				switch op {
				case combinationAnd, combinationReplace:
					mask0 = ^(originalMask >> offset)
					mask1 = ^word(0)
					if offset != 0 {
						mask1 = ^(originalMask << (wordBits - offset))
					}
				case combinationXnor:
					mask0 = originalMask >> offset
					if offset != 0 {
						mask1 = originalMask << (wordBits - offset)
					}
				}
				blitRows(region.data, pattern.data, offset, yu, firstWord, two, stride, ph, op, mask0, mask1)
			} else if err := region.combine(pattern, int32(x), int32(y), op, c); err != nil {
				return err
			}

			coords.advanceCol()
		}
		coords.advanceRow()
	}

	return nil
}

// blitRows is the macro blit_rows! in render_patterns in hayro-jbig2
// src/decode/halftone.rs. It combines the ph rows of a pattern of one word
// into data, at the bit offset of the word firstWord. If two is true, the
// pattern also covers the next word.
//
// For And and Replace, mask0 and mask1 are the inverse masks of the pattern
// in the two words. For Xnor, they are the masks themselves.
func blitRows(data, pattern []word, offset, yu, firstWord uint32, two bool, stride, ph uint32,
	op combinationOperator, mask0, mask1 word) {
	combineWord := func(d, s, mask word) word {
		switch op {
		case combinationOr:
			return d | s
		case combinationXor:
			return d ^ s
		case combinationAnd:
			return d & (s | mask)
		case combinationReplace:
			return (d & mask) | s
		case combinationXnor:
			return (d &^ mask) | (^(d ^ s) & mask)
		}
		return d
	}

	for row := range ph {
		base := int((yu+row)*stride + firstWord)
		w := pattern[row]
		data[base] = combineWord(data[base], w>>offset, mask0)
		if two {
			data[base+1] = combineWord(data[base+1], w<<(wordBits-offset), mask1)
		}
	}
}
