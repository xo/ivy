// Copyright (c) The Hayro Authors
//
// Use of this source code is governed by the Apache-2.0 or MIT license that
// can be found in the LICENSE-APACHE and LICENSE-MIT files.

package jbig2

// This file ports hayro-jbig2 src/page_info.rs.

// pageInformation is PageInformation in hayro-jbig2 src/page_info.rs, the
// data of a page information segment (T.88 7.4.8). A resolution of 0 is the
// None of upstream.
type pageInformation struct {
	width       uint32
	height      uint32
	xResolution uint32
	yResolution uint32
	flags       pageFlags
	striping    pageStriping
}

// pageFlags is PageFlags in hayro-jbig2 src/page_info.rs.
type pageFlags struct {
	isLossless                    bool
	mightContainRefinements       bool
	defaultPixel                  uint8
	defaultCombinationOperator    combinationOperator
	requiresAuxiliaryBuffers      bool
	combinationOperatorOverridden bool
	mightContainColoured          bool
}

// pageStriping is PageStriping in hayro-jbig2 src/page_info.rs.
type pageStriping struct {
	isStriped     bool
	maxStripeSize uint16
}

// parsePageInformation is parse_page_information in hayro-jbig2
// src/page_info.rs.
func parsePageInformation(r *reader) (pageInformation, error) {
	var info pageInformation
	var ok1, ok2, ok3, ok4 bool
	info.width, ok1 = r.readU32()
	info.height, ok2 = r.readU32()
	info.xResolution, ok3 = r.readU32()
	info.yResolution, ok4 = r.readU32()
	if !ok1 || !ok2 || !ok3 || !ok4 {
		return info, ErrUnexpectedEOF
	}

	flagsByte, ok := r.readByte()
	if !ok {
		return info, ErrUnexpectedEOF
	}
	info.flags = parsePageFlags(flagsByte)

	stripingRaw, ok := r.readU16()
	if !ok {
		return info, ErrUnexpectedEOF
	}
	info.striping = pageStriping{
		isStriped:     stripingRaw&0x8000 != 0,
		maxStripeSize: stripingRaw & 0x7FFF,
	}

	return info, nil
}

// parsePageFlags is parse_page_flags in hayro-jbig2 src/page_info.rs.
func parsePageFlags(flags uint8) pageFlags {
	// The two bits give Or, And, Xor or Xnor, the first four values of
	// combinationOperator.
	return pageFlags{
		isLossless:                    flags&0x01 != 0,
		mightContainRefinements:       flags&0x02 != 0,
		defaultPixel:                  (flags >> 2) & 0x01,
		defaultCombinationOperator:    combinationOperator((flags >> 3) & 0x03),
		requiresAuxiliaryBuffers:      flags&0x20 != 0,
		combinationOperatorOverridden: flags&0x40 != 0,
		mightContainColoured:          flags&0x80 != 0,
	}
}
