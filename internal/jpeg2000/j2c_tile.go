// Copyright (c) The Hayro Authors
//
// Use of this source code is governed by the Apache-2.0 or MIT license that
// can be found in the LICENSE-APACHE and LICENSE-MIT files.

package jpeg2000

// This file ports hayro-jpeg2000 src/j2c/tile.rs.

import (
	"iter"
	"slices"
)

// tile is Tile in hayro-jpeg2000 src/j2c/tile.rs.
type tile struct {
	tileParts          []tilePart
	componentInfos     []componentInfo
	rect               intRect
	progressionOrder   progressionOrder
	progressionChanges []progressionChange
	numLayers          uint8
	mct                bool
}

// tilePart is TilePart in hayro-jpeg2000 src/j2c/tile.rs, with the cases
// MergedTilePart and SeparatedTilePart.
//
// Port: the Rust enum becomes one struct. A merged tile-part holds its
// packet headers and its packet bodies in data. A separated tile-part holds
// its packet headers in headers, from the markers PPT or PPM, and its packet
// bodies in body.
type tilePart struct {
	separated bool

	// data is the data of MergedTilePart.
	data bitReader

	// headers, activeHeaderReader and bodyReader are the fields of
	// SeparatedTilePart.
	headers            []bitReader
	activeHeaderReader int
	bodyReader         bitReader
}

// clone returns a copy of the tile-part, with readers of its own, as Clone
// does upstream.
func (tp *tilePart) clone() tilePart {
	c := *tp
	c.headers = slices.Clone(tp.headers)
	return c
}

// header is TilePart::header. It returns the reader of the packet headers.
func (tp *tilePart) header() *bitReader {
	if !tp.separated {
		return &tp.data
	}
	if tp.headers[tp.activeHeaderReader].atEnd() && len(tp.headers)-1 > tp.activeHeaderReader {
		tp.activeHeaderReader++
	}
	return &tp.headers[tp.activeHeaderReader]
}

// body is TilePart::body. It returns the reader of the packet bodies.
func (tp *tilePart) body() *bitReader {
	if !tp.separated {
		return &tp.data
	}
	return &tp.bodyReader
}

// newTile is Tile::new.
func newTile(idx uint32, h *header) tile {
	sd := &h.sizeData
	xCoord := uint64(sd.tileXCoord(idx))
	yCoord := uint64(sd.tileYCoord(idx))

	x0 := uint32(max(uint64(sd.tileXOffset)+xCoord*uint64(sd.tileWidth), uint64(sd.imageAreaXOffset)))
	y0 := uint32(max(uint64(sd.tileYOffset)+yCoord*uint64(sd.tileHeight), uint64(sd.imageAreaYOffset)))
	x1 := uint32(min(uint64(sd.tileXOffset)+(xCoord+1)*uint64(sd.tileWidth), uint64(sd.referenceGridWidth)))
	y1 := uint32(min(uint64(sd.tileYOffset)+(yCoord+1)*uint64(sd.tileHeight), uint64(sd.referenceGridHeight)))

	return tile{
		rect:             intRectFromLTRB(x0, y0, x1, y1),
		componentInfos:   slices.Clone(h.componentInfos),
		progressionOrder: h.globalCodingStyle.progressionOrder,
		mct:              h.globalCodingStyle.mct,
		numLayers:        h.globalCodingStyle.numLayers,
	}
}

// componentTiles is Tile::component_tiles.
func (t *tile) componentTiles() iter.Seq2[int, componentTile] {
	return func(yield func(int, componentTile) bool) {
		for i := range t.componentInfos {
			if !yield(i, newComponentTile(t, &t.componentInfos[i])) {
				return
			}
		}
	}
}

// tileParse is parse in hayro-jpeg2000 src/j2c/tile.rs. It reads every
// tile-part of the codestream.
//
// Port: the number of tiles is bounded by the limits.
func tileParse(reader *bitReader, mainHeader *header, lim *limits) ([]tile, error) {
	tileCount, err := mainHeader.sizeData.numTiles()
	if err != nil {
		return nil, err
	}
	if err := lim.objects(uint64(tileCount) * uint64(len(mainHeader.componentInfos))); err != nil {
		return nil, err
	}
	tiles := make([]tile, tileCount)
	for idx := range tileCount {
		tiles[idx] = newTile(idx, mainHeader)
	}

	tilePartIdx := 0
	if err := parseTilePart(reader, mainHeader, tiles, tilePartIdx); err != nil {
		return nil, err
	}
	tilePartIdx++

	for {
		m, ok := reader.peekMarker()
		if !ok || m != markerSOT {
			break
		}
		if err := parseTilePart(reader, mainHeader, tiles, tilePartIdx); err != nil {
			return nil, err
		}
		tilePartIdx++
	}

	if mainHeader.strict {
		m, err := reader.readMarker()
		if err != nil {
			return nil, err
		}
		if m != markerEOC {
			return nil, errMarkerExpected("EOC")
		}
	}
	return tiles, nil
}

// parseTilePart is parse_tile_part in hayro-jpeg2000 src/j2c/tile.rs.
func parseTilePart(reader *bitReader, mainHeader *header, tiles []tile, tilePartIdx int) error {
	m, err := reader.readMarker()
	if err != nil {
		return err
	}
	if m != markerSOT {
		return errMarkerExpected("SOT")
	}

	tph, ok := sotMarker(reader)
	if !ok {
		return errMarkerParseFailure("SOT")
	}
	numTiles, err := mainHeader.sizeData.numTiles()
	if err != nil {
		return err
	}
	if uint32(tph.tileIndex) >= numTiles || int(tph.tileIndex) >= len(tiles) {
		return errTileInvalidIndex
	}

	var dataLen int
	if tph.tilePartLength == 0 {
		if t, ok := reader.tail(); ok {
			dataLen = len(t)
		}
	} else {
		// Psot includes the 12-byte SOT marker segment.
		if tph.tilePartLength < 12 {
			return errTileInvalid
		}
		dataLen = int(tph.tilePartLength - 12)
	}

	start := reader.offset()
	t := &tiles[tph.tileIndex]
	numComponents := uint16(len(t.componentInfos))
	var pptHeaders []pptMarkerData

	strictOr := func(err error) error {
		if mainHeader.strict {
			return err
		}
		return nil
	}

loop:
	for {
		// Some files have tile-parts without an SOD marker.
		if reader.offset()-start >= dataLen {
			return strictOr(errMarkerExpected("SOD"))
		}

		m, ok := reader.peekMarker()
		if !ok {
			return strictOr(errMarkerInvalid)
		}

		switch {
		case m == markerSOD:
			_, _ = reader.readMarker()
			break loop
		case m == markerCOD:
			_, _ = reader.readMarker()
			cod, ok := codMarker(reader)
			if !ok {
				return errMarkerParseFailure("COD")
			}
			t.mct = cod.mct
			t.numLayers = cod.numLayers
			t.progressionOrder = cod.progressionOrder
			for i := range t.componentInfos {
				c := &t.componentInfos[i]
				c.codingStyle.flags.raw |= cod.componentParameters.flags.raw
				c.codingStyle.parameters = cod.componentParameters.parameters
			}
		case m == markerCOC:
			_, _ = reader.readMarker()
			idx, coc, ok := cocMarker(reader, numComponents)
			if !ok {
				return errMarkerParseFailure("COC")
			}
			if int(idx) >= len(t.componentInfos) {
				return errInvalidComponentMetadata
			}
			old := &t.componentInfos[idx]
			old.codingStyle.parameters = coc.parameters
			old.codingStyle.flags.raw |= coc.flags.raw
		case m == markerQCD:
			_, _ = reader.readMarker()
			qcd, ok := qcdMarker(reader)
			if !ok {
				return errMarkerParseFailure("QCD")
			}
			for i := range t.componentInfos {
				t.componentInfos[i].quantizationInfo = qcd
			}
		case m == markerQCC:
			_, _ = reader.readMarker()
			idx, qcc, ok := qccMarker(reader, numComponents)
			if !ok {
				return errMarkerParseFailure("QCC")
			}
			if int(idx) >= len(t.componentInfos) {
				return errInvalidComponentMetadata
			}
			t.componentInfos[idx].quantizationInfo = qcc
		case m == markerEOC:
			break loop
		case m == markerPPT:
			if len(mainHeader.ppmPackets) != 0 {
				return errTilePpmPptConflict
			}
			_, _ = reader.readMarker()
			ppt, ok := pptMarker(reader)
			if !ok {
				return errMarkerParseFailure("PPT")
			}
			pptHeaders = append(pptHeaders, ppt)
		case m == markerRGN:
			_, _ = reader.readMarker()
			idx, shift, ok := rgnMarker(reader, numComponents)
			if !ok {
				return errMarkerParseFailure("RGN")
			}
			if int(idx) >= len(t.componentInfos) {
				return errInvalidComponentMetadata
			}
			t.componentInfos[idx].roiShift = shift
		case m == markerPOC:
			_, _ = reader.readMarker()
			changes, ok := pocMarker(reader, numComponents)
			if !ok {
				return errMarkerParseFailure("POC")
			}
			t.progressionChanges = append(t.progressionChanges, changes...)
		case m == markerPLT:
			_, _ = reader.readMarker()
			if !skipMarkerSegment(reader) {
				return errMarkerParseFailure("PLT")
			}
		case m == markerCOM:
			_, _ = reader.readMarker()
			if !skipMarkerSegment(reader) {
				return errMarkerParseFailure("COM")
			}
		case m >= 0x30 && m <= 0x3f:
			_, _ = reader.readMarker()
		default:
			return errMarkerUnsupported
		}
	}

	used := reader.offset() - start
	if used > dataLen {
		return strictOr(errTileInvalid)
	}
	remainingBytes := dataLen - used

	slices.SortStableFunc(pptHeaders, func(a, b pptMarkerData) int {
		return int(a.sequenceIdx) - int(b.sequenceIdx)
	})
	headers := make([]bitReader, 0, len(pptHeaders)+1)
	for _, p := range pptHeaders {
		headers = append(headers, newBitReader(p.data))
	}
	if tilePartIdx < len(mainHeader.ppmPackets) {
		headers = append(headers, newBitReader(mainHeader.ppmPackets[tilePartIdx]))
	}

	data, ok := reader.readBytes(remainingBytes)
	if !ok {
		if mainHeader.strict {
			return errTileInvalid
		}
		data, ok = reader.tail()
		if !ok {
			return errTileInvalid
		}
		reader.jumpToEnd()
	}

	if len(headers) != 0 {
		t.tileParts = append(t.tileParts, tilePart{
			separated:  true,
			headers:    headers,
			bodyReader: newBitReader(data),
		})
	} else {
		t.tileParts = append(t.tileParts, tilePart{data: newBitReader(data)})
	}
	return nil
}

// componentTile is ComponentTile in hayro-jpeg2000 src/j2c/tile.rs.
type componentTile struct {
	tile          *tile
	componentInfo *componentInfo
	rect          intRect
}

// newComponentTile is ComponentTile::new.
func newComponentTile(t *tile, info *componentInfo) componentTile {
	r := t.rect
	if info.sizeInfo.horizontalResolution != 1 || info.sizeInfo.verticalResolution != 1 {
		hr := uint32(info.sizeInfo.horizontalResolution)
		vr := uint32(info.sizeInfo.verticalResolution)
		r = intRectFromLTRB(divCeil32(r.x0, hr), divCeil32(r.y0, vr), divCeil32(r.x1, hr), divCeil32(r.y1, vr))
	}
	return componentTile{tile: t, componentInfo: info, rect: r}
}

// resolutionTile is ResolutionTile in hayro-jpeg2000 src/j2c/tile.rs.
type resolutionTile struct {
	resolution         uint8
	decompositionLevel uint8
	componentTile      componentTile
	rect               intRect
}

// newResolutionTile is ResolutionTile::new.
//
// Port: upstream asserts that the resolution exists, and panics otherwise.
// Each caller makes sure of it, and the port returns an error otherwise.
func newResolutionTile(ct componentTile, resolution uint8) (resolutionTile, error) {
	params := &ct.componentInfo.codingStyle.parameters
	if params.numResolutionLevels <= resolution {
		return resolutionTile{}, errInternal
	}
	nL := params.numDecompositionLevels
	div := uint64(1) << ((uint32(nL) - uint32(resolution)) & 63)
	r := intRectFromLTRB(
		uint32(divCeil64(uint64(ct.rect.x0), div)),
		uint32(divCeil64(uint64(ct.rect.y0), div)),
		uint32(divCeil64(uint64(ct.rect.x1), div)),
		uint32(divCeil64(uint64(ct.rect.y1), div)),
	)
	level := nL
	if resolution != 0 {
		level = nL - (resolution - 1)
	}
	return resolutionTile{
		resolution:         resolution,
		decompositionLevel: level,
		componentTile:      ct,
		rect:               r,
	}, nil
}

// subBandRect is ResolutionTile::sub_band_rect.
func (rt *resolutionTile) subBandRect(t subBandType) intRect {
	var xoB, yoB uint64
	if t == highLow || t == highHigh {
		xoB = 1
	}
	if t == lowHigh || t == highHigh {
		yoB = 1
	}

	var numeratorX, numeratorY uint64
	if rt.decompositionLevel > 0 {
		numeratorX = (uint64(1) << ((uint32(rt.decompositionLevel) - 1) & 63)) * xoB
		numeratorY = (uint64(1) << ((uint32(rt.decompositionLevel) - 1) & 63)) * yoB
	}
	denominator := uint64(1) << (uint32(rt.decompositionLevel) & 63)

	sub := func(a, b uint64) uint64 {
		if a < b {
			return 0
		}
		return a - b
	}
	cr := rt.componentTile.rect
	return intRectFromLTRB(
		uint32(divCeil64(sub(uint64(cr.x0), numeratorX), denominator)),
		uint32(divCeil64(sub(uint64(cr.y0), numeratorY), denominator)),
		uint32(divCeil64(sub(uint64(cr.x1), numeratorX), denominator)),
		uint32(divCeil64(sub(uint64(cr.y1), numeratorY), denominator)),
	)
}

// precinctExponentX is ResolutionTile::precinct_exponent_x.
func (rt *resolutionTile) precinctExponentX() uint8 {
	return rt.componentTile.componentInfo.codingStyle.parameters.precinctExponents[rt.resolution][0]
}

// precinctExponentY is ResolutionTile::precinct_exponent_y.
func (rt *resolutionTile) precinctExponentY() uint8 {
	return rt.componentTile.componentInfo.codingStyle.parameters.precinctExponents[rt.resolution][1]
}

// numPrecinctsX is ResolutionTile::num_precincts_x.
func (rt *resolutionTile) numPrecinctsX() uint32 {
	if rt.rect.x0 == rt.rect.x1 {
		return 0
	}
	p := uint32(1) << rt.precinctExponentX()
	return divCeil32(rt.rect.x1, p) - rt.rect.x0/p
}

// numPrecinctsY is ResolutionTile::num_precincts_y.
func (rt *resolutionTile) numPrecinctsY() uint32 {
	if rt.rect.y0 == rt.rect.y1 {
		return 0
	}
	p := uint32(1) << rt.precinctExponentY()
	return divCeil32(rt.rect.y1, p) - rt.rect.y0/p
}

// numPrecincts is ResolutionTile::num_precincts.
func (rt *resolutionTile) numPrecincts() uint64 {
	return uint64(rt.numPrecinctsX()) * uint64(rt.numPrecinctsY())
}

// precincts is ResolutionTile::precincts. It returns false when the
// geometry of the precincts overflows.
func (rt *resolutionTile) precincts() (iter.Seq[precinctData], bool) {
	numPrecinctsY := rt.numPrecinctsY()
	numPrecinctsX := rt.numPrecinctsX()

	ppx := rt.precinctExponentX()
	ppy := rt.precinctExponentY()

	yStart := rt.rect.y0 / (1 << ppy) * (1 << ppy)
	xStart := rt.rect.x0 / (1 << ppx) * (1 << ppx)

	// The spec only reduces PPx/PPy for code-blocks, but precincts need it
	// too.
	if rt.resolution > 0 {
		if ppx == 0 || ppy == 0 {
			return nil, false
		}
		ppx--
		ppy--
		xStart /= 2
		yStart /= 2
	}

	ppxPow2 := uint32(1) << ppx
	ppyPow2 := uint32(1) << ppy

	info := rt.componentTile.componentInfo
	nlMinusR := info.numDecompositionLevels() - rt.resolution

	xShift := uint32(rt.precinctExponentX()) + uint32(nlMinusR)
	yShift := uint32(rt.precinctExponentY()) + uint32(nlMinusR)
	if xShift > 0xff || yShift > 0xff || xShift >= 32 || yShift >= 32 {
		return nil, false
	}
	xStride := uint32(1) << xShift
	yStride := uint32(1) << yShift

	xStep := uint64(info.sizeInfo.horizontalResolution) * uint64(xStride)
	yStep := uint64(info.sizeInfo.verticalResolution) * uint64(yStride)
	if xStep > 0xffffffff || yStep > 0xffffffff {
		return nil, false
	}
	precinctXStep := uint32(xStep)
	precinctYStep := uint32(yStep)

	rX := rt.componentTile.tile.rect.x0
	rY := rt.componentTile.tile.rect.y0

	if rX%precinctXStep != 0 && (rt.rect.x0*shl32(1, uint32(nlMinusR)))%xStride == 0 {
		var ok bool
		if rX, ok = checkedNextMultipleOf32(rX, precinctXStep); !ok {
			return nil, false
		}
	}
	if rY%precinctYStep != 0 && (rt.rect.y0*shl32(1, uint32(nlMinusR)))%yStride == 0 {
		var ok bool
		if rY, ok = checkedNextMultipleOf32(rY, precinctYStep); !ok {
			return nil, false
		}
	}

	return func(yield func(precinctData) bool) {
		rowY := rY
		for y := range numPrecinctsY {
			y0 := y*ppyPow2 + yStart
			colX := rX
			for x := range numPrecinctsX {
				x0 := x*ppxPow2 + xStart
				d := precinctData{
					rX:   colX,
					rY:   rowY,
					rect: intRectFromXYWH(x0, y0, ppxPow2, ppyPow2),
					idx:  uint64(numPrecinctsX)*uint64(y) + uint64(x),
				}
				colX = nextMultipleOf32(colX+1, precinctXStep)
				if !yield(d) {
					return
				}
			}
			rowY = nextMultipleOf32(rowY+1, precinctYStep)
		}
	}, true
}

// codeBlockWidth is ResolutionTile::code_block_width.
func (rt *resolutionTile) codeBlockWidth() uint32 {
	xcb := rt.componentTile.componentInfo.codingStyle.parameters.codeBlockWidth
	if rt.resolution > 0 {
		xcb = min(xcb, rt.precinctExponentX()-1)
	} else {
		xcb = min(xcb, rt.precinctExponentX())
	}
	return uint32(1) << xcb
}

// codeBlockHeight is ResolutionTile::code_block_height.
func (rt *resolutionTile) codeBlockHeight() uint32 {
	ycb := rt.componentTile.componentInfo.codingStyle.parameters.codeBlockHeight
	if rt.resolution > 0 {
		ycb = min(ycb, rt.precinctExponentY()-1)
	} else {
		ycb = min(ycb, rt.precinctExponentY())
	}
	return uint32(1) << ycb
}

// tilePartHeader is TilePartHeader in hayro-jpeg2000 src/j2c/tile.rs.
type tilePartHeader struct {
	tileIndex      uint16
	tilePartLength uint32
}

// pptMarkerData is PptMarkerData in hayro-jpeg2000 src/j2c/tile.rs.
type pptMarkerData struct {
	data        []byte
	sequenceIdx uint8
}

// pptMarker is ppt_marker in hayro-jpeg2000 src/j2c/tile.rs.
func pptMarker(reader *bitReader) (pptMarkerData, bool) {
	length, ok := reader.readU16()
	if !ok || length < 3 {
		return pptMarkerData{}, false
	}
	seq, ok := reader.readByte()
	if !ok {
		return pptMarkerData{}, false
	}
	data, ok := reader.readBytes(int(length - 3))
	if !ok {
		return pptMarkerData{}, false
	}
	return pptMarkerData{data: data, sequenceIdx: seq}, true
}

// sotMarker is sot_marker in hayro-jpeg2000 src/j2c/tile.rs.
func sotMarker(reader *bitReader) (tilePartHeader, bool) {
	b, ok := reader.readBytes(10)
	if !ok {
		return tilePartHeader{}, false
	}
	return tilePartHeader{
		tileIndex:      uint16(b[2])<<8 | uint16(b[3]),
		tilePartLength: uint32(b[4])<<24 | uint32(b[5])<<16 | uint32(b[6])<<8 | uint32(b[7]),
	}, true
}
