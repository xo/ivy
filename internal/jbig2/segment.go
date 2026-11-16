// Copyright (c) The Hayro Authors
//
// Use of this source code is governed by the Apache-2.0 or MIT license that
// can be found in the LICENSE-APACHE and LICENSE-MIT files.

package jbig2

// This file ports hayro-jbig2 src/segment.rs.

// segmentType is SegmentType in hayro-jbig2 src/segment.rs.
type segmentType uint8

// The values of segmentType.
const (
	segmentSymbolDictionary segmentType = iota
	segmentIntermediateTextRegion
	segmentImmediateTextRegion
	segmentImmediateLosslessTextRegion
	segmentPatternDictionary
	segmentIntermediateHalftoneRegion
	segmentImmediateHalftoneRegion
	segmentImmediateLosslessHalftoneRegion
	segmentIntermediateGenericRegion
	segmentImmediateGenericRegion
	segmentImmediateLosslessGenericRegion
	segmentIntermediateGenericRefinementRegion
	segmentImmediateGenericRefinementRegion
	segmentImmediateLosslessGenericRefinementRegion
	segmentPageInformation
	segmentEndOfPage
	segmentEndOfStripe
	segmentEndOfFile
	segmentProfiles
	segmentTables
	segmentColourPalette
	segmentExtension
	segmentReserved
)

// requiresPage is SegmentType::requires_page. It reports whether a segment
// of the type must belong to a page.
func (t segmentType) requiresPage() bool {
	if t.isImmediateRegion() {
		return true
	}
	switch t {
	case segmentIntermediateTextRegion, segmentIntermediateHalftoneRegion,
		segmentIntermediateGenericRegion, segmentIntermediateGenericRefinementRegion,
		segmentPageInformation, segmentEndOfPage, segmentEndOfStripe:
		return true
	}
	return false
}

// isImmediateRegion is SegmentType::is_immediate_region.
func (t segmentType) isImmediateRegion() bool {
	switch t {
	case segmentImmediateTextRegion, segmentImmediateLosslessTextRegion,
		segmentImmediateHalftoneRegion, segmentImmediateLosslessHalftoneRegion,
		segmentImmediateGenericRegion, segmentImmediateLosslessGenericRegion,
		segmentImmediateGenericRefinementRegion, segmentImmediateLosslessGenericRefinementRegion:
		return true
	}
	return false
}

// segmentTypeFromValue is SegmentType::from_type_value. It maps the type
// numbers of T.88 7.3.
func segmentTypeFromValue(value uint8) segmentType {
	switch value {
	case 0:
		return segmentSymbolDictionary
	case 4:
		return segmentIntermediateTextRegion
	case 6:
		return segmentImmediateTextRegion
	case 7:
		return segmentImmediateLosslessTextRegion
	case 16:
		return segmentPatternDictionary
	case 20:
		return segmentIntermediateHalftoneRegion
	case 22:
		return segmentImmediateHalftoneRegion
	case 23:
		return segmentImmediateLosslessHalftoneRegion
	case 36:
		return segmentIntermediateGenericRegion
	case 38:
		return segmentImmediateGenericRegion
	case 39:
		return segmentImmediateLosslessGenericRegion
	case 40:
		return segmentIntermediateGenericRefinementRegion
	case 42:
		return segmentImmediateGenericRefinementRegion
	case 43:
		return segmentImmediateLosslessGenericRefinementRegion
	case 48:
		return segmentPageInformation
	case 49:
		return segmentEndOfPage
	case 50:
		return segmentEndOfStripe
	case 51:
		return segmentEndOfFile
	case 52:
		return segmentProfiles
	case 53:
		return segmentTables
	case 54:
		return segmentColourPalette
	case 62:
		return segmentExtension
	}
	return segmentReserved
}

// segmentHeader is SegmentHeader in hayro-jbig2 src/segment.rs.
//
// Port: dataLength holds the Option<u32> of upstream as a value and the bool
// hasDataLength. The bool is false for an unknown length.
type segmentHeader struct {
	segmentNumber      uint32
	segmentType        segmentType
	retainFlag         bool
	pageAssociation    uint32
	referredToSegments []uint32
	dataLength         uint32
	hasDataLength      bool
}

// segment is Segment in hayro-jbig2 src/segment.rs. The data is a part of
// the input.
type segment struct {
	header segmentHeader
	data   []byte
}

// parseSegmentHeader is parse_segment_header in hayro-jbig2 src/segment.rs.
func parseSegmentHeader(r *reader, strict bool) (segmentHeader, error) {
	var h segmentHeader

	segmentNumber, ok := r.readU32()
	if !ok {
		return h, ErrUnexpectedEOF
	}

	flags, ok := r.readByte()
	if !ok {
		return h, ErrUnexpectedEOF
	}

	typ := segmentTypeFromValue(flags & 0x3F)
	if strict && typ == segmentReserved {
		return h, ErrUnknownSegmentType
	}

	pageAssociationLong := flags&0x40 != 0
	retainFlag := flags&0x80 == 0

	countAndRetention, ok := r.readByte()
	if !ok {
		return h, ErrUnexpectedEOF
	}
	shortCount := (countAndRetention >> 5) & 0x07

	if shortCount == 5 || shortCount == 6 {
		return h, ErrInvalidReferredCount
	}

	referredToCount := uint32(shortCount)
	if shortCount >= 7 {
		rest, ok := r.readBytes(3)
		if !ok {
			return h, ErrUnexpectedEOF
		}
		referredToCount = uint32(countAndRetention&0x1F)<<24 | uint32(rest[0])<<16 | uint32(rest[1])<<8 | uint32(rest[2])
	}

	if shortCount == 7 {
		retentionBytes := (int(referredToCount) + 1 + 7) / 8
		if !r.skipBytes(retentionBytes) {
			return h, ErrUnexpectedEOF
		}
	}

	numberSize := 4
	switch {
	case segmentNumber <= 256:
		numberSize = 1
	case segmentNumber <= 65536:
		numberSize = 2
	}
	numbers, ok := r.readBytes(int(referredToCount) * numberSize)
	if !ok {
		return h, ErrUnexpectedEOF
	}

	referredToSegments := make([]uint32, 0, referredToCount)
	for i := 0; i+numberSize <= len(numbers); i += numberSize {
		var referred uint32
		for _, b := range numbers[i : i+numberSize] {
			referred = referred<<8 | uint32(b)
		}

		if referred >= segmentNumber {
			return h, ErrInvalidReference
		}

		referredToSegments = append(referredToSegments, referred)
	}

	var pageAssociation uint32
	if pageAssociationLong {
		pageAssociation, ok = r.readU32()
	} else {
		var b byte
		b, ok = r.readByte()
		pageAssociation = uint32(b)
	}
	if !ok {
		return h, ErrUnexpectedEOF
	}

	dataLengthRaw, ok := r.readU32()
	if !ok {
		return h, ErrUnexpectedEOF
	}
	hasDataLength := true
	if dataLengthRaw == 0xFFFFFFFF {
		if strict && typ != segmentImmediateGenericRegion {
			return h, ErrInvalidUnknownLength
		}
		hasDataLength = false
	}

	return segmentHeader{
		segmentNumber:      segmentNumber,
		segmentType:        typ,
		retainFlag:         retainFlag,
		pageAssociation:    pageAssociation,
		referredToSegments: referredToSegments,
		dataLength:         dataLengthRaw,
		hasDataLength:      hasDataLength,
	}, nil
}

// parseSegment is parse_segment in hayro-jbig2 src/segment.rs.
func parseSegment(r *reader, strict bool) (segment, error) {
	h, err := parseSegmentHeader(r, strict)
	if err != nil {
		return segment{}, err
	}
	return parseSegmentData(r, h)
}

// parseSegmentData is parse_segment_data in hayro-jbig2 src/segment.rs.
func parseSegmentData(r *reader, h segmentHeader) (segment, error) {
	var n int
	if h.hasDataLength {
		n = int(h.dataLength)
	} else {
		var err error
		n, err = scanForImmediateGenericRegionSize(r)
		if err != nil {
			return segment{}, err
		}
	}

	data, ok := r.readBytes(n)
	if !ok {
		return segment{}, ErrUnexpectedEOF
	}
	return segment{header: h, data: data}, nil
}

// scanForImmediateGenericRegionSize is scan_for_immediate_generic_region_size
// in hayro-jbig2 src/segment.rs. It finds the length of an immediate generic
// region from its end marker and its row count (T.88 7.2.7).
func scanForImmediateGenericRegionSize(r *reader) (int, error) {
	scan := *r
	startOffset := scan.bytePos()

	if !scan.skipBytes(17) {
		return 0, ErrUnexpectedEOF
	}
	flags, ok := scan.readByte()
	if !ok {
		return 0, ErrUnexpectedEOF
	}
	usesMMR := flags&1 != 0

	// The AT flags can hold the end marker, so the search starts after them.
	var atBytes int
	switch {
	case usesMMR:
		atBytes = 0
	case (flags>>1)&0x03 == 0 && flags&0x10 == 0:
		atBytes = 8
	case (flags>>1)&0x03 == 0:
		atBytes = 24
	default:
		atBytes = 2
	}
	if !scan.skipBytes(atBytes) {
		return 0, ErrUnexpectedEOF
	}

	endMarker := [2]byte{0xFF, 0xAC}
	if usesMMR {
		endMarker = [2]byte{0x00, 0x00}
	}

	for {
		b, ok := scan.peekBytes(6)
		if !ok {
			break
		}
		if b[0] == endMarker[0] && b[1] == endMarker[1] {
			return scan.bytePos() - startOffset + 2 + 4, nil
		}
		if !scan.skipBytes(1) {
			return 0, ErrUnexpectedEOF
		}
	}

	return 0, ErrMissingEndMarker
}
