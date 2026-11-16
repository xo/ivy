// Copyright (c) The Hayro Authors
//
// Use of this source code is governed by the Apache-2.0 or MIT license that
// can be found in the LICENSE-APACHE and LICENSE-MIT files.

package jbig2

// This file ports hayro-jbig2 src/file.rs.

import (
	"bytes"
	"errors"
	"slices"
)

// fileOrganization is FileOrganization in hayro-jbig2 src/file.rs.
type fileOrganization uint8

// The values of fileOrganization.
const (
	organizationSequential fileOrganization = iota
	organizationRandomAccess
)

// fileHeader is FileHeader in hayro-jbig2 src/file.rs.
//
// Port: numberOfPages holds the Option<u32> of upstream as a value and the
// bool hasNumberOfPages.
type fileHeader struct {
	organization            fileOrganization
	numberOfPages           uint32
	hasNumberOfPages        bool
	usesExtendedTemplates   bool
	containsColouredRegions bool
}

// parsedFile is File in hayro-jbig2 src/file.rs.
//
// Port: the name File is the public type of src/lib.rs.
type parsedFile struct {
	header   fileHeader
	segments []segment
}

// fileHeaderID is FILE_HEADER_ID, the ID string of T.88 D.4.1.
var fileHeaderID = []byte{0x97, 0x4A, 0x42, 0x32, 0x0D, 0x0A, 0x1A, 0x0A}

// parseFile is parse_file in hayro-jbig2 src/file.rs.
func parseFile(data []byte, strict bool) (*parsedFile, error) {
	r := newReader(data)

	header, err := parseFileHeader(&r)
	if err != nil {
		return nil, err
	}
	segments, err := parseSegments(&r, header.organization, strict)
	if err != nil {
		return nil, err
	}

	// The specification requires sorted segments. The port sorts them anyway,
	// because the lookups search them with a binary search.
	sortSegments(segments)

	return &parsedFile{header: header, segments: segments}, nil
}

// sortSegments is segments.sort_by_key(|seg| seg.header.segment_number) of
// upstream, a stable sort.
func sortSegments(segments []segment) {
	slices.SortStableFunc(segments, func(a, b segment) int {
		switch {
		case a.header.segmentNumber < b.header.segmentNumber:
			return -1
		case a.header.segmentNumber > b.header.segmentNumber:
			return 1
		}
		return 0
	})
}

// parseFileHeader is parse_file_header in hayro-jbig2 src/file.rs.
func parseFileHeader(r *reader) (fileHeader, error) {
	var h fileHeader

	id, ok := r.readBytes(8)
	if !ok {
		return h, ErrUnexpectedEOF
	}
	if !bytes.Equal(id, fileHeaderID) {
		return h, ErrInvalidHeader
	}

	flags, ok := r.readByte()
	if !ok {
		return h, ErrUnexpectedEOF
	}

	h.organization = organizationRandomAccess
	if flags&0x01 != 0 {
		h.organization = organizationSequential
	}
	unknownPageCount := flags&0x02 != 0
	h.usesExtendedTemplates = flags&0x04 != 0
	h.containsColouredRegions = flags&0x08 != 0

	if flags&0xF0 != 0 {
		return h, ErrReservedBits
	}

	if !unknownPageCount {
		h.numberOfPages, ok = r.readU32()
		if !ok {
			return h, ErrUnexpectedEOF
		}
		h.hasNumberOfPages = true
	}

	return h, nil
}

// parseSegments is parse_segments in hayro-jbig2 src/file.rs.
func parseSegments(r *reader, organization fileOrganization, strict bool) ([]segment, error) {
	var segments []segment
	var err error
	switch organization {
	case organizationSequential:
		segments, err = parseSegmentsSequential(r, segments, strict)
	case organizationRandomAccess:
		segments, err = parseSegmentsRandomAccess(r, segments, strict)
	}
	return segments, err
}

// parseSegmentsSequential is parse_segments_sequential in hayro-jbig2
// src/file.rs. Each segment header comes just before the data of the
// segment. If strict is false, the segments stop at a truncated segment.
func parseSegmentsSequential(r *reader, segments []segment, strict bool) ([]segment, error) {
	for !r.atEnd() {
		seg, err := parseSegment(r, strict)
		if err != nil {
			if !strict && errors.Is(err, ErrUnexpectedEOF) {
				break
			}
			return segments, err
		}

		isEOF := seg.header.segmentType == segmentEndOfFile
		segments = append(segments, seg)

		if isEOF {
			break
		}
	}

	return segments, nil
}

// parseSegmentsRandomAccess is parse_segments_random_access in hayro-jbig2
// src/file.rs. All segment headers come first, then the data of the
// segments.
func parseSegmentsRandomAccess(r *reader, segments []segment, strict bool) ([]segment, error) {
	var headers []segmentHeader

	for !r.atEnd() {
		h, err := parseSegmentHeader(r, strict)
		if err != nil {
			return segments, err
		}

		isEOF := h.segmentType == segmentEndOfFile
		headers = append(headers, h)

		if isEOF {
			break
		}
	}

	for _, h := range headers {
		seg, err := parseSegmentData(r, h)
		if err != nil {
			return segments, err
		}
		segments = append(segments, seg)
	}

	return segments, nil
}
