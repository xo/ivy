// Copyright (c) The Hayro Authors
//
// Use of this source code is governed by the Apache-2.0 or MIT license that
// can be found in the LICENSE-APACHE and LICENSE-MIT files.

package jpeg2000

// This file ports hayro-jpeg2000 src/j2c/segment.rs.

import "math/bits"

// maxBitplaneCount is MAX_BITPLANE_COUNT.
const maxBitplaneCount = 32

// segmentParse is parse in hayro-jpeg2000 src/j2c/segment.rs. It reads the
// packets of each tile-part of the tile.
func segmentParse(t *tile, it progressionIter, h *header, storage *decompositionStorage, c cancel) error {
	for i := range t.tileParts {
		tp := t.tileParts[i].clone()
		ok, err := parseInner(&tp, it, t.componentInfos, storage, c)
		if err != nil {
			return err
		}
		if !ok && h.strict {
			return errTileInvalid
		}
	}
	return nil
}

// parseInner is parse_inner in hayro-jpeg2000 src/j2c/segment.rs. It
// returns false when a packet is malformed.
func parseInner(tp *tilePart, it progressionIter, infos []componentInfo, storage *decompositionStorage, c cancel) (bool, error) {
	for !tp.header().atEnd() {
		if err := c.check(); err != nil {
			return false, err
		}
		if !parsePacket(tp, it, infos, storage) {
			return false, nil
		}
	}
	return true, nil
}

// parsePacket is parse_packet in hayro-jpeg2000 src/j2c/segment.rs. It
// reads the header and the body of one packet.
//
// Port: upstream indexes the storage with the numbers of the packet, which
// the build makes valid. The port also returns false for an index out of
// range, where upstream panics.
func parsePacket(tp *tilePart, it progressionIter, infos []componentInfo, storage *decompositionStorage) bool {
	pd, ok := it()
	if !ok {
		return false
	}
	resolution := pd.resolution
	if int(pd.component) >= len(infos) || int(pd.component) >= len(storage.tileDecompositions) {
		return false
	}
	info := &infos[pd.component]
	td := &storage.tileDecompositions[pd.component]
	sbIter, ok := td.subBandIter(resolution, storage.decompositions)
	if !ok {
		return false
	}

	body := tp.body()
	if info.codingStyle.flags.mayUseSopMarkers() {
		if m, ok := body.peekMarker(); ok && m == markerSOP {
			if _, err := body.readMarker(); err != nil {
				return false
			}
			if !body.skipBytes(4) {
				return false
			}
		}
	}

	hr := tp.header()
	zero, ok := hr.readBitsWithStuffing(1)
	if !ok {
		return false
	}
	zeroLength := zero == 0

	if !zeroLength {
		for _, sb := range sbIter {
			if !resolveSegments(sb, &pd, hr, storage, info) {
				return false
			}
		}
	}

	hr.align()

	if info.codingStyle.flags.usesEphMarker() {
		m, err := hr.readMarker()
		if err != nil || m != markerEPH {
			return false
		}
	}

	body = tp.body()

	if !zeroLength {
		for _, sbIdx := range sbIter {
			sb := &storage.subBands[sbIdx]
			pIdx := sb.precincts.start + int(pd.precinct)
			if pd.precinct >= uint64(sb.precincts.end-sb.precincts.start) {
				return false
			}
			p := &storage.precincts[pIdx]
			for cbIdx := p.codeBlocks.start; cbIdx < p.codeBlocks.end; cbIdx++ {
				cb := &storage.codeBlocks[cbIdx]
				if int(pd.layerNum) >= cb.layers.end-cb.layers.start {
					return false
				}
				l := &storage.layers[cb.layers.start+int(pd.layerNum)]
				if !l.hasSegments {
					continue
				}
				for s := l.segments.start; s < l.segments.end; s++ {
					seg := &storage.segments[s]
					data, ok := body.readBytes(int(seg.dataLength))
					if !ok {
						return false
					}
					seg.data = data
				}
			}
		}
	}
	return true
}

// resolveSegments is resolve_segments in hayro-jpeg2000
// src/j2c/segment.rs. It reads the part of a packet header for the
// code-blocks of one sub-band.
func resolveSegments(sbIdx int, pd *progressionData, reader *bitReader, storage *decompositionStorage, info *componentInfo) bool {
	const maxCodingPasses = 1 + 3*(maxBitplaneCount-1)

	sb := &storage.subBands[sbIdx]
	if pd.precinct >= uint64(sb.precincts.end-sb.precincts.start) {
		return false
	}
	p := &storage.precincts[sb.precincts.start+int(pd.precinct)]

	for cbIdx := p.codeBlocks.start; cbIdx < p.codeBlocks.end; cbIdx++ {
		cb := &storage.codeBlocks[cbIdx]

		var isIncluded bool
		if cb.hasBeenIncluded {
			b, ok := reader.readBitsWithStuffing(1)
			if !ok {
				return false
			}
			isIncluded = b == 1
		} else {
			v, ok := p.codeInclusionTree.read(cb.xIdx, cb.yIdx, reader, uint32(pd.layerNum)+1, storage.tagTreeNodes)
			if !ok {
				return false
			}
			isIncluded = v <= uint32(pd.layerNum)
		}

		if !isIncluded {
			continue
		}

		if int(pd.layerNum) >= cb.layers.end-cb.layers.start {
			return false
		}
		l := &storage.layers[cb.layers.start+int(pd.layerNum)]

		if !cb.hasBeenIncluded {
			v, ok := p.zeroBitplaneTree.read(cb.xIdx, cb.yIdx, reader, 0xffffffff, storage.tagTreeNodes)
			if !ok {
				return false
			}
			cb.missingBitPlanes = uint8(v)
		}
		cb.hasBeenIncluded = true

		added, ok := readCodingPasses(reader)
		if !ok {
			return false
		}

		var k uint32
		for {
			b, ok := reader.readBitsWithStuffing(1)
			if !ok {
				return false
			}
			if b != 1 {
				break
			}
			k++
		}
		cb.lBlock += k

		previous := cb.numberOfCodingPasses
		cumulative := uint32(previous) + uint32(added)
		if cumulative > maxCodingPasses {
			return false
		}

		segmentIdx := func(passIdx uint8) uint8 {
			style := info.codeBlockStyle()
			switch {
			case style.terminationOnEachPass:
				return passIdx
			case style.selectiveArithmeticCodingBypass:
				return segmentIdxForBypass(passIdx)
			}
			return cb.nonEmptyLayerCount
		}

		start := len(storage.segments)

		pushSegment := func(seg, passes uint8) bool {
			// Port: upstream asserts that the segment has a pass.
			if passes == 0 {
				return false
			}
			lengthBits := cb.lBlock + uint32(bits.Len8(passes)-1)
			length, ok := reader.readBitsWithStuffing(uint8(lengthBits))
			if !ok {
				return false
			}
			storage.segments = append(storage.segments, segment{
				idx:         seg,
				dataLength:  length,
				codingPases: passes,
			})
			return true
		}

		lastSegment := segmentIdx(previous)
		var passesForSegment uint8

		for pass := previous; uint32(pass) < cumulative; pass++ {
			s := segmentIdx(pass)
			if s != lastSegment {
				if !pushSegment(lastSegment, passesForSegment) {
					return false
				}
				lastSegment = s
				passesForSegment = 1
			} else {
				passesForSegment++
			}
		}

		if passesForSegment > 0 && !pushSegment(lastSegment, passesForSegment) {
			return false
		}

		l.segments = span{start, len(storage.segments)}
		l.hasSegments = true
		cb.numberOfCodingPasses += added
		cb.nonEmptyLayerCount++
	}
	return true
}

// readCodingPasses reads the number of coding passes that a packet adds to
// a code-block, as resolve_segments in hayro-jpeg2000 src/j2c/segment.rs
// does.
func readCodingPasses(reader *bitReader) (uint8, bool) {
	peek := func(n uint8, want uint32) bool {
		v, ok := reader.peakBitsWithStuffing(n)
		return ok && v == want
	}
	skip := func(n uint8) bool {
		_, ok := reader.readBitsWithStuffing(n)
		return ok
	}

	switch {
	case peek(9, 0x1ff):
		if !skip(9) {
			return 0, false
		}
		v, ok := reader.readBitsWithStuffing(7)
		return uint8(v + 37), ok
	case peek(4, 0x0f):
		if !skip(4) {
			return 0, false
		}
		v, ok := reader.readBitsWithStuffing(5)
		return uint8(v + 6), ok
	case peek(4, 0b1110):
		return 5, skip(4)
	case peek(4, 0b1101):
		return 4, skip(4)
	case peek(4, 0b1100):
		return 3, skip(4)
	case peek(2, 0b10):
		return 2, skip(2)
	case peek(1, 0):
		return 1, skip(1)
	}
	return 0, false
}

// segmentIdxForBypass is segment_idx_for_bypass in hayro-jpeg2000
// src/j2c/segment.rs.
func segmentIdxForBypass(passIdx uint8) uint8 {
	if passIdx < 10 {
		return 0
	}
	var extra uint8
	if (passIdx-10)%3 == 2 {
		extra = 1
	}
	return 1 + 2*((passIdx-10)/3) + extra
}
