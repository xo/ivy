// Copyright (c) The Hayro Authors
//
// Use of this source code is governed by the Apache-2.0 or MIT license that
// can be found in the LICENSE-APACHE and LICENSE-MIT files.

package jbig2

// This file ports hayro-jbig2 src/decode/mod.rs.

// regionBitmap is RegionBitmap in hayro-jbig2 src/decode/mod.rs.
type regionBitmap struct {
	bitmap              *bitmap
	combinationOperator combinationOperator
}

// combinationOperator is CombinationOperator in hayro-jbig2
// src/decode/mod.rs. Its zero value is Or, the default of upstream.
type combinationOperator uint8

// The values of combinationOperator.
const (
	combinationOr combinationOperator = iota
	combinationAnd
	combinationXor
	combinationXnor
	combinationReplace
)

// combinationOperatorFromValue is CombinationOperator::from_value. It reads
// the low three bits of value.
func combinationOperatorFromValue(value uint8) (combinationOperator, error) {
	v := value & 0x07
	if v > 4 {
		return 0, ErrInvalidCombinationOperator
	}
	return combinationOperator(v), nil
}

// template is Template in hayro-jbig2 src/decode/mod.rs, the template of a
// generic region.
type template uint8

// The values of template.
const (
	template0 template = iota
	template1
	template2
	template3
)

// templateFromByte is Template::from_byte. It reads the low two bits of
// value.
func templateFromByte(value uint8) template {
	return template(value & 0x03)
}

// contextBits is Template::context_bits.
func (t template) contextBits() uint {
	switch t {
	case template0:
		return 16
	case template1:
		return 13
	}
	return 10
}

// adaptiveTemplatePixels is Template::adaptive_template_pixels. It is the
// number of adaptive template pixels of the template.
func (t template) adaptiveTemplatePixels() int {
	if t == template0 {
		return 4
	}
	return 1
}

// regionSegmentInfo is RegionSegmentInfo in hayro-jbig2 src/decode/mod.rs,
// the region segment information field of T.88 7.4.1.
type regionSegmentInfo struct {
	width               uint32
	height              uint32
	xLocation           uint32
	yLocation           uint32
	combinationOperator combinationOperator
}

// parseRegionSegmentInfo is parse_region_segment_info in hayro-jbig2
// src/decode/mod.rs. A region with colour extension returns ErrUnsupported.
func parseRegionSegmentInfo(r *reader) (regionSegmentInfo, error) {
	var info regionSegmentInfo
	var ok1, ok2, ok3, ok4 bool
	info.width, ok1 = r.readU32()
	info.height, ok2 = r.readU32()
	info.xLocation, ok3 = r.readU32()
	info.yLocation, ok4 = r.readU32()
	if !ok1 || !ok2 || !ok3 || !ok4 {
		return info, ErrUnexpectedEOF
	}

	flags, ok := r.readByte()
	if !ok {
		return info, ErrUnexpectedEOF
	}

	op, err := combinationOperatorFromValue(flags)
	if err != nil {
		return info, err
	}
	info.combinationOperator = op

	if flags&0x08 != 0 {
		return info, ErrUnsupported
	}

	return info, nil
}

// adaptiveTemplatePixel is AdaptiveTemplatePixel in hayro-jbig2
// src/decode/mod.rs. It is the offset of an adaptive template pixel from the
// current pixel.
type adaptiveTemplatePixel struct {
	x int8
	y int8
}

// parseRefinementATPixels is parse_refinement_at_pixels in hayro-jbig2
// src/decode/mod.rs.
func parseRefinementATPixels(r *reader) ([]adaptiveTemplatePixel, error) {
	var b [4]byte
	for i := range b {
		v, ok := r.readByte()
		if !ok {
			return nil, ErrUnexpectedEOF
		}
		b[i] = v
	}

	return []adaptiveTemplatePixel{
		{x: int8(b[0]), y: int8(b[1])},
		{x: int8(b[2]), y: int8(b[3])},
	}, nil
}

// refinementTemplate is RefinementTemplate in hayro-jbig2 src/decode/mod.rs.
type refinementTemplate uint8

// The values of refinementTemplate.
const (
	refinementTemplate0 refinementTemplate = iota
	refinementTemplate1
)

// refinementTemplateFromByte is RefinementTemplate::from_byte. It reads the
// low bit of value.
func refinementTemplateFromByte(value uint8) refinementTemplate {
	return refinementTemplate(value & 0x01)
}

// contextBits is RefinementTemplate::context_bits.
func (t refinementTemplate) contextBits() uint {
	if t == refinementTemplate0 {
		return 13
	}
	return 10
}
