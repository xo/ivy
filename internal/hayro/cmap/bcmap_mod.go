// Copyright (c) The Hayro Authors.
//
// Use of this source code is governed by the Apache-2.0 or MIT license that
// can be found in the LICENSE-APACHE and LICENSE-MIT files.

// This file ports hayro-cmap src/bcmap/mod.rs.

package cmap

// This file reads the binary form of a CMap, the form of the predefined
// CMaps that the package embeds. The format is at
// https://github.com/LaurenzV/cmap-resources/tree/hayro.

// The constants of the binary form, from hayro-cmap src/bcmap/mod.rs.
const (
	bcmapMagic          = "bcmap"
	bcmapVersion        = 0x01
	bcmapFileHeaderSize = 10
	segHeaderSize       = 5
)

// The types of the segments, from hayro-cmap src/bcmap/mod.rs.
const (
	segmentRange1B          = 0x01
	segmentSingle1B         = 0x02
	segmentRange2B          = 0x03
	segmentSingle2B         = 0x04
	segmentRange3B          = 0x05
	segmentSingle3B         = 0x06
	segmentRange4B          = 0x07
	segmentSingle4B         = 0x08
	segmentUseCMap          = 0x09
	segmentNotdef           = 0x0a
	segmentWMode            = 0x0b
	segmentCodespace        = 0x0c
	segmentName             = 0x0d
	segmentCIDSystemInfo    = 0x0e
	segmentBfRangeVariable  = 0x0f
	segmentBfSingleVariable = 0x10
	segmentBfSingle1U       = 0x11
	segmentBfSingle2U       = 0x12
	segmentBfSingle3U       = 0x13
	segmentBfSingle4U       = 0x14
	segmentBfRange1U        = 0x15
	segmentBfRange2U        = 0x16
)

// msgTruncated is the message of the error for a binary CMap that ends too
// early.
const msgTruncated = "the binary CMap is truncated"

// parseBCMap is parse in hayro-cmap src/bcmap/mod.rs. It parses a CMap in
// the binary form. It decodes the codes with the Huffman tables of the
// embedded data.
//
// Port: upstream reads the CMap of a usecmap segment at the same depth. If
// the resolver returns a binary CMap that names itself, upstream recurses
// without end. Go adds one to the depth, so that maxNestingDepth bounds the
// recursion, and it counts the CMap against maxNestedParses.
func parseBCMap(data []byte, getCMap Resolver, depth uint32, budget *int) (*CMap, error) {
	if len(data) < 6 || string(data[:5]) != bcmapMagic || data[5] != bcmapVersion {
		return nil, errFormat("the binary CMap has no valid header")
	}

	r := newBCMapReader(data)
	r.readBytes(6)
	fileLen, ok := r.readU32()
	if !ok {
		return nil, errFormat(msgTruncated)
	}

	b, err := loadBundle()
	if err != nil {
		return nil, err
	}
	deltaTable := &b.deltaTable
	countTable := &b.countTable

	var cmapName []byte
	var characterCollection *CharacterCollection
	var writingMode *WritingMode
	var base *CMap
	var codespaceRanges []CodespaceRange
	var cidRanges, notdefRanges partitionedRanges
	var bfEntries []bfRange

	if uint64(fileLen) < bcmapFileHeaderSize || uint64(fileLen) > uint64(len(data)) {
		return nil, errFormat("the binary CMap has a wrong length")
	}
	r = newBCMapReader(data[bcmapFileHeaderSize:fileLen])

	for !r.atEnd() {
		segType, ok1 := r.readU8()
		segLen, ok2 := r.readU32()
		if !ok1 || !ok2 || segLen < segHeaderSize {
			return nil, errFormat(msgTruncated)
		}
		payload, ok := r.readBytes(int(segLen - segHeaderSize))
		if !ok {
			return nil, errFormat(msgTruncated)
		}

		ok = true
		switch segType {
		case segmentName:
			cmapName = append([]byte{}, payload...)
		case segmentCIDSystemInfo:
			pr := newBCMapReader(payload)
			isNUL := func(b byte) bool { return b == 0 }
			registry := pr.eatUntil(isNUL)
			_, ok1 := pr.readU8()
			ordering := pr.eatUntil(isNUL)
			_, ok2 := pr.readU8()
			supplement, ok3 := pr.readU16()
			if !ok1 || !ok2 || !ok3 {
				return nil, errFormat(msgTruncated)
			}
			characterCollection = &CharacterCollection{
				Family:     CIDFamilyFromRegistryOrdering(registry, ordering),
				Supplement: int32(supplement),
			}
		case segmentUseCMap:
			baseData, err := resolve(getCMap, NameFromBytes(payload), budget)
			if err != nil {
				return nil, err
			}
			if base, err = parseInner(baseData, getCMap, depth+1, budget); err != nil {
				return nil, err
			}
		case segmentWMode:
			if len(payload) == 0 {
				return nil, errFormat(msgTruncated)
			}
			var m WritingMode
			switch payload[0] {
			case 0:
				m = Horizontal
				writingMode = &m
			case 1:
				m = Vertical
				writingMode = &m
			default:
				writingMode = nil
			}
		case segmentCodespace:
			ok = parseCodespace(payload, &codespaceRanges)
		case segmentNotdef:
			ok = parseNotdef(payload, &notdefRanges)
		case segmentRange1B, segmentRange2B, segmentRange3B, segmentRange4B:
			bw := (int(segType) + 1) / 2
			ok = parseCIDSegment(payload, &cidRanges, bw, deltaTable, countTable)
		case segmentSingle1B, segmentSingle2B, segmentSingle3B, segmentSingle4B:
			bw := (int(segType) + 1) / 2
			ok = parseCIDSegment(payload, &cidRanges, bw, deltaTable, nil)
		case segmentBfRangeVariable:
			ok = parseBfSegment(payload, &bfEntries, deltaTable, countTable, 0)
		case segmentBfSingleVariable:
			ok = parseBfSegment(payload, &bfEntries, deltaTable, nil, 0)
		case segmentBfRange1U:
			ok = parseBfSegment(payload, &bfEntries, deltaTable, countTable, 1)
		case segmentBfRange2U:
			ok = parseBfSegment(payload, &bfEntries, deltaTable, countTable, 2)
		case segmentBfSingle1U:
			ok = parseBfSegment(payload, &bfEntries, deltaTable, nil, 1)
		case segmentBfSingle2U:
			ok = parseBfSegment(payload, &bfEntries, deltaTable, nil, 2)
		case segmentBfSingle3U:
			ok = parseBfSegment(payload, &bfEntries, deltaTable, nil, 3)
		case segmentBfSingle4U:
			ok = parseBfSegment(payload, &bfEntries, deltaTable, nil, 4)
		default:
			return nil, errFormat("the binary CMap has an unknown segment")
		}
		if !ok {
			return nil, errFormat("the binary CMap has a malformed segment")
		}
	}

	cidRanges.sort()
	notdefRanges.sort()
	sortBfEntries(bfEntries)

	return &CMap{
		metadata: Metadata{
			CharacterCollection: characterCollection,
			Name:                cmapName,
			WritingMode:         writingMode,
		},
		codespaceRanges: codespaceRanges,
		cidRanges:       cidRanges,
		notdefRanges:    notdefRanges,
		bfEntries:       bfEntries,
		base:            base,
	}, nil
}

// parseCodespace is parse_codespace in hayro-cmap src/bcmap/mod.rs.
func parseCodespace(payload []byte, ranges *[]CodespaceRange) bool {
	r := newBCMapReader(payload)
	nRanges, ok := r.readU8()
	if !ok {
		return false
	}

	for range nRanges {
		bw, ok := r.readU8()
		if !ok {
			return false
		}
		low, ok1 := r.readNBytes(int(bw))
		high, ok2 := r.readNBytes(int(bw))
		if !ok1 || !ok2 {
			return false
		}
		*ranges = append(*ranges, CodespaceRange{numberBytes: bw, low: low, high: high})
	}

	return true
}

// parseNotdef is parse_notdef in hayro-cmap src/bcmap/mod.rs.
func parseNotdef(payload []byte, ranges *partitionedRanges) bool {
	r := newBCMapReader(payload)
	bw, ok1 := r.readU8()
	nEntries, ok2 := r.readU16()
	if !ok1 || !ok2 {
		return false
	}

	for range nEntries {
		start, ok1 := r.readNBytes(int(bw))
		end, ok2 := r.readNBytes(int(bw))
		cid, ok3 := r.readU16()
		if !ok1 || !ok2 || !ok3 {
			return false
		}
		ranges.push(int(bw), CIDRange{rng: codeRange{start: start, end: end}, cidStart: CID(cid)})
	}

	return true
}

// decodeSymbols decodes n symbols of table from data. Each symbol takes at
// least one bit, so the capacity is at most eight for each byte of data.
//
// Port: upstream writes the loop twice in each of the two parsers of
// segments, and it reserves n entries.
func decodeSymbols(table *huffmanTable, data []byte, n int) ([]uint32, bool) {
	dr := newBCMapReader(data)
	out := make([]uint32, 0, min(n, 8*len(data)))
	for range n {
		v, ok := table.decode(dr)
		if !ok {
			return nil, false
		}
		out = append(out, v)
	}
	return out, true
}

// readSymbols is the read of a length and of the encoded symbols in
// parse_cid_segment and parse_bf_segment in hayro-cmap src/bcmap/mod.rs.
func readSymbols(r *reader, table *huffmanTable, n int) ([]uint32, bool) {
	l, ok := r.readU32()
	if !ok {
		return nil, false
	}
	data, ok := r.readBytes(int(l))
	if !ok {
		return nil, false
	}
	return decodeSymbols(table, data, n)
}

// parseCIDSegment is parse_cid_segment in hayro-cmap src/bcmap/mod.rs. A
// segment of ranges has a count table, and a segment of single codes has
// none.
func parseCIDSegment(payload []byte, ranges *partitionedRanges, byteWidth int, deltaTable, countTable *huffmanTable) bool {
	r := newBCMapReader(payload)
	n, ok := r.readU16()
	if !ok {
		return false
	}
	nEntries := int(n)
	if nEntries == 0 {
		return true
	}

	deltas, ok := readSymbols(r, deltaTable, nEntries)
	if !ok {
		return false
	}
	var counts []uint32
	if countTable != nil {
		if counts, ok = readSymbols(r, countTable, nEntries); !ok {
			return false
		}
	}

	isRange := countTable != nil
	var prevEnd, prevCID uint32
	hasPrev := false
	var prevRangeLen uint32

	// The arithmetic wraps, as it does in a release build of upstream.
	for i := range nEntries {
		rawCID, ok := r.readU16()
		if !ok {
			return false
		}

		// The data holds each delta of a start and each count minus one.
		start := deltas[i]
		if hasPrev {
			start = prevEnd + 1 + deltas[i]
		}

		end := start
		if isRange {
			end = start + counts[i] + 1
		}

		cid := CID(rawCID)
		if rawCID == 0 {
			cid = 0
			if hasPrev {
				cid = prevCID + prevRangeLen + 1
			}
		}

		ranges.push(byteWidth, CIDRange{rng: codeRange{start: start, end: end}, cidStart: cid})

		prevEnd = end
		prevCID = cid
		hasPrev = true
		prevRangeLen = end - start
	}

	return true
}

// parseBfSegment is parse_bf_segment in hayro-cmap src/bcmap/mod.rs. A
// segment of ranges has a count table, and a segment of single codes has
// none. If fixedUnits is 0, each entry starts with its number of units.
//
// Port: upstream passes the number of units as an Option. Go uses 0 for
// None, which upstream never passes as a number.
func parseBfSegment(payload []byte, entries *[]bfRange, deltaTable, countTable *huffmanTable, fixedUnits int) bool {
	r := newBCMapReader(payload)
	n, ok := r.readU16()
	if !ok {
		return false
	}
	nEntries := int(n)

	deltas, ok := readSymbols(r, deltaTable, nEntries)
	if !ok {
		return false
	}
	var counts []uint32
	if countTable != nil {
		if counts, ok = readSymbols(r, countTable, nEntries); !ok {
			return false
		}
	}

	isRange := countTable != nil
	var prevEnd uint32
	hasPrev := false

	for i := range nEntries {
		nUnits := fixedUnits
		if nUnits == 0 {
			v, ok := r.readU8()
			if !ok {
				return false
			}
			nUnits = int(v)
		}
		units, ok := r.readBytes(2 * nUnits)
		if !ok {
			return false
		}
		dstBase := make([]uint16, nUnits)
		for j := range dstBase {
			dstBase[j] = uint16(units[2*j])<<8 | uint16(units[2*j+1])
		}

		start := deltas[i]
		if hasPrev {
			start = prevEnd + 1 + deltas[i]
		}

		end := start
		if isRange {
			end = start + counts[i] + 1
		}

		*entries = append(*entries, bfRange{rng: codeRange{start: start, end: end}, dstBase: dstBase})

		prevEnd = end
		hasPrev = true
	}

	return true
}
