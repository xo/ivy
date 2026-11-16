// Copyright (c) The Hayro Authors
//
// Use of this source code is governed by the Apache-2.0 or MIT license that
// can be found in the LICENSE-APACHE and LICENSE-MIT files.

package jpeg2000

// This file ports hayro-jpeg2000 src/j2c/build.rs.

// build is build in hayro-jpeg2000 src/j2c/build.rs. It builds the
// decompositions, the sub-bands, the precincts and the code-blocks of a
// tile in the storage.
func build(t *tile, storage *decompositionStorage, skippedResolutionLevels uint8, c cancel) error {
	return buildDecompositions(t, storage, skippedResolutionLevels, c)
}

// buildDecompositions is build_decompositions in hayro-jpeg2000
// src/j2c/build.rs.
func buildDecompositions(t *tile, storage *decompositionStorage, skippedResolutionLevels uint8, c cancel) error {
	componentSamples := make([]int, 0, len(t.componentInfos))

	for _, ct := range t.componentTiles() {
		// Tile-part headers can lower the resolution levels below the
		// skipped count.
		levels := ct.componentInfo.numResolutionLevels()
		if levels <= skippedResolutionLevels {
			return errInvalidComponentMetadata
		}
		decodedResolutions := levels - skippedResolutionLevels
		top, err := newResolutionTile(ct, decodedResolutions-1)
		if err != nil {
			return err
		}
		area := top.rect.area()
		// Port: the limits bound the buffer of the coefficients.
		if err := storage.lim.memory(area * 4); err != nil {
			return err
		}
		componentSamples = append(componentSamples, int(area))
	}
	maxCoefficients := 0
	for _, n := range componentSamples {
		maxCoefficients = max(maxCoefficients, n)
	}

	if len(storage.coefficients) == 0 {
		storage.coefficients = make([]float32, maxCoefficients)
	} else {
		var err error
		if storage.coefficients, err = c.resize(storage.coefficients, maxCoefficients); err != nil {
			return err
		}
	}

	for componentIdx, ct := range t.componentTiles() {
		b := subBandBuilder{
			storage:            storage,
			tile:               t,
			cancel:             c,
			decodedResolutions: ct.componentInfo.numResolutionLevels() - skippedResolutionLevels,
		}
		dStart := len(storage.decompositions)

		levels := ct.componentInfo.numResolutionLevels()
		llTile, err := newResolutionTile(ct, 0)
		if err != nil {
			return err
		}
		firstLLSubBand, err := b.buildSubBand(lowLow, &llTile)
		if err != nil {
			return err
		}

		for r := uint8(1); r < levels; r++ {
			rt, err := newResolutionTile(ct, r)
			if err != nil {
				return err
			}
			var d decomposition
			for i, sbt := range [3]subBandType{highLow, lowHigh, highHigh} {
				if d.subBands[i], err = b.buildSubBand(sbt, &rt); err != nil {
					return err
				}
			}
			d.rect = rt.rect
			storage.decompositions = append(storage.decompositions, d)
		}

		dEnd := len(storage.decompositions)
		storage.tileDecompositions = append(storage.tileDecompositions, tileDecompositions{
			decompositions:   span{dStart, dEnd},
			firstLLSubBand:   firstLLSubBand,
			coefficientCount: b.coefficientCounter,
		})

		// Port: upstream asserts this equality, and panics otherwise.
		if b.coefficientCounter != componentSamples[componentIdx] {
			return errInternal
		}
	}
	return nil
}

// subBandBuilder holds the state of the closure build_sub_band of
// build_decompositions in hayro-jpeg2000 src/j2c/build.rs.
type subBandBuilder struct {
	storage            *decompositionStorage
	tile               *tile
	cancel             cancel
	decodedResolutions uint8
	coefficientCounter int
}

// buildSubBand is the closure build_sub_band of build_decompositions. It
// returns the index of the new sub-band.
func (b *subBandBuilder) buildSubBand(t subBandType, rt *resolutionTile) (int, error) {
	rect := rt.subBandRect(t)

	precincts, err := buildPrecincts(rt, rect, b.tile, b.storage, b.cancel)
	if err != nil {
		return 0, err
	}

	var added int
	if rt.resolution < b.decodedResolutions {
		added = int(rect.area())
	}
	next := b.coefficientCounter + added
	if next < b.coefficientCounter {
		return 0, errImageTooLarge
	}
	coefficients := span{b.coefficientCounter, next}
	b.coefficientCounter = next

	idx := len(b.storage.subBands)
	b.storage.subBands = append(b.storage.subBands, subBand{
		subBandType:  t,
		rect:         rect,
		precincts:    precincts,
		coefficients: coefficients,
	})
	return idx, nil
}

// buildPrecincts is build_precincts in hayro-jpeg2000 src/j2c/build.rs.
func buildPrecincts(rt *resolutionTile, sbRect intRect, t *tile, storage *decompositionStorage, c cancel) (span, error) {
	start := len(storage.precincts)

	seq, ok := rt.precincts()
	if !ok {
		return span{}, errInvalidPrecinct
	}
	for pd := range seq {
		if err := c.check(); err != nil {
			return span{}, err
		}

		pr := pd.rect
		cbWidth := rt.codeBlockWidth()
		cbHeight := rt.codeBlockHeight()

		cbX0 := max(pr.x0, sbRect.x0) / cbWidth * cbWidth
		cbY0 := max(pr.y0, sbRect.y0) / cbHeight * cbHeight
		cbX1 := divCeil32(min(pr.x1, sbRect.x1), cbWidth) * cbWidth
		cbY1 := divCeil32(min(pr.y1, sbRect.y1), cbHeight) * cbHeight

		area := intRectFromLTRB(cbX0, cbY0, cbX1, cbY1)

		// An empty sub-band has no code-blocks. Without this check, the
		// rounding gives it one code-block on each axis.
		var codeBlocksX, codeBlocksY uint32
		if sbRect.width() != 0 {
			codeBlocksX = area.width() / cbWidth
		}
		if sbRect.height() != 0 {
			codeBlocksY = area.height() / cbHeight
		}

		// Port: the limits bound the storage of the precinct, of its
		// code-blocks, of their layers and of the two tag trees.
		if err := storage.grow(uint64(codeBlocksX)*uint64(codeBlocksY), uint64(t.numLayers)); err != nil {
			return span{}, err
		}

		blocks, err := buildCodeBlocks(area, sbRect, rt, codeBlocksX, codeBlocksY, t, storage, c)
		if err != nil {
			return span{}, err
		}

		inclusion := newTagTree(codeBlocksX, codeBlocksY, &storage.tagTreeNodes)
		zeroBitplane := newTagTree(codeBlocksX, codeBlocksY, &storage.tagTreeNodes)

		storage.precincts = append(storage.precincts, precinct{
			codeBlocks:        blocks,
			codeInclusionTree: inclusion,
			zeroBitplaneTree:  zeroBitplane,
		})
	}

	return span{start, len(storage.precincts)}, nil
}

// buildCodeBlocks is build_code_blocks in hayro-jpeg2000 src/j2c/build.rs.
func buildCodeBlocks(area, sbRect intRect, rt *resolutionTile, codeBlocksX, codeBlocksY uint32, t *tile, storage *decompositionStorage, c cancel) (span, error) {
	y := area.y0
	cbWidth := rt.codeBlockWidth()
	cbHeight := rt.codeBlockHeight()

	start := len(storage.codeBlocks)

	for yIdx := range codeBlocksY {
		if err := c.check(); err != nil {
			return span{}, err
		}

		x := area.x0
		for xIdx := range codeBlocksX {
			r := intRectFromXYWH(x, y, cbWidth, cbHeight).intersect(sbRect)

			lStart := len(storage.layers)
			for range t.numLayers {
				storage.layers = append(storage.layers, layer{})
			}
			lEnd := len(storage.layers)

			storage.codeBlocks = append(storage.codeBlocks, codeBlock{
				xIdx:   xIdx,
				yIdx:   yIdx,
				rect:   r,
				lBlock: 3,
				layers: span{lStart, lEnd},
			})

			x += cbWidth
		}
		y += cbHeight
	}

	return span{start, len(storage.codeBlocks)}, nil
}

// span is a Range<usize> of Rust: the indices from start to end, without
// end.
type span struct {
	start, end int
}

// decomposition is Decomposition in hayro-jpeg2000 src/j2c/build.rs.
type decomposition struct {
	// subBands holds the indices of the sub-bands in the order high-low,
	// low-high and high-high.
	subBands [3]int
	rect     intRect
}

// subBand is SubBand in hayro-jpeg2000 src/j2c/build.rs.
type subBand struct {
	subBandType  subBandType
	rect         intRect
	precincts    span
	coefficients span
}

// subBandType is SubBandType in hayro-jpeg2000 src/j2c/build.rs.
type subBandType uint8

// The types of sub-bands.
const (
	lowLow subBandType = iota
	highLow
	lowHigh
	highHigh
)

// precinct is Precinct in hayro-jpeg2000 src/j2c/build.rs.
type precinct struct {
	codeBlocks        span
	codeInclusionTree tagTree
	zeroBitplaneTree  tagTree
}

// precinctData is PrecinctData in hayro-jpeg2000 src/j2c/build.rs.
type precinctData struct {
	rX, rY uint32
	rect   intRect
	idx    uint64
}

// codeBlock is CodeBlock in hayro-jpeg2000 src/j2c/build.rs.
type codeBlock struct {
	rect                 intRect
	xIdx, yIdx           uint32
	layers               span
	hasBeenIncluded      bool
	missingBitPlanes     uint8
	numberOfCodingPasses uint8
	lBlock               uint32
	nonEmptyLayerCount   uint8
}

// segment is Segment in hayro-jpeg2000 src/j2c/build.rs.
type segment struct {
	idx         uint8
	codingPases uint8
	dataLength  uint32
	data        []byte
}

// layer is Layer in hayro-jpeg2000 src/j2c/build.rs.
//
// Port: the Option of upstream is the field hasSegments.
type layer struct {
	segments    span
	hasSegments bool
}
