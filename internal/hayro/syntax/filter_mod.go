// Copyright (c) The Hayro Authors
//
// Use of this source code is governed by the Apache-2.0 or MIT license that
// can be found in the LICENSE-APACHE and LICENSE-MIT files.

// This file ports hayro-syntax src/filter/mod.rs.

package syntax

import (
	"fmt"

	"github.com/xo/ivy"
)

// Filter is Filter in hayro-syntax src/filter/mod.rs. It is a filter of
// data.
type Filter uint8

// The filters of hayro-syntax src/filter/mod.rs.
const (
	// FilterASCIIHexDecode is Filter::AsciiHexDecode: ASCII hexadecimal
	// encoding.
	FilterASCIIHexDecode Filter = iota
	// FilterASCII85Decode is Filter::Ascii85Decode: ASCII base-85 encoding.
	FilterASCII85Decode
	// FilterLZWDecode is Filter::LzwDecode: Lempel-Ziv-Welch compression.
	FilterLZWDecode
	// FilterFlateDecode is Filter::FlateDecode: DEFLATE compression.
	FilterFlateDecode
	// FilterRunLengthDecode is Filter::RunLengthDecode: run-length
	// compression.
	FilterRunLengthDecode
	// FilterCCITTFaxDecode is Filter::CcittFaxDecode: CCITT Group 3 or Group
	// 4 fax compression.
	FilterCCITTFaxDecode
	// FilterJBIG2Decode is Filter::Jbig2Decode: JBIG2 compression of
	// bi-level images.
	FilterJBIG2Decode
	// FilterDCTDecode is Filter::DctDecode: JPEG compression.
	FilterDCTDecode
	// FilterJPXDecode is Filter::JpxDecode: JPEG 2000 compression.
	FilterJPXDecode
	// FilterCrypt is Filter::Crypt: the filter of encryption.
	FilterCrypt
)

// String returns the Debug text of upstream.
func (f Filter) String() string {
	switch f {
	case FilterASCIIHexDecode:
		return "AsciiHexDecode"
	case FilterASCII85Decode:
		return "Ascii85Decode"
	case FilterLZWDecode:
		return "LzwDecode"
	case FilterFlateDecode:
		return "FlateDecode"
	case FilterRunLengthDecode:
		return "RunLengthDecode"
	case FilterCCITTFaxDecode:
		return "CcittFaxDecode"
	case FilterJBIG2Decode:
		return "Jbig2Decode"
	case FilterDCTDecode:
		return "DctDecode"
	case FilterJPXDecode:
		return "JpxDecode"
	case FilterCrypt:
		return "Crypt"
	}
	return fmt.Sprintf("Filter(%d)", uint8(f))
}

// filterFromName is Filter::from_name in hayro-syntax src/filter/mod.rs.
func filterFromName(name Name) (Filter, bool) {
	switch name {
	case KeyASCIIHexDecode, KeyAHx:
		return FilterASCIIHexDecode, true
	case KeyASCII85Decode, KeyA85:
		return FilterASCII85Decode, true
	case KeyLZWDecode, KeyLZW:
		return FilterLZWDecode, true
	case KeyFlateDecode, KeyFl:
		return FilterFlateDecode, true
	case KeyRunLengthDecode, KeyRL:
		return FilterRunLengthDecode, true
	case KeyCCITTFaxDecode, KeyCCF:
		return FilterCCITTFaxDecode, true
	case KeyJBIG2Decode:
		return FilterJBIG2Decode, true
	case KeyDCTDecode, KeyDCT:
		return FilterDCTDecode, true
	case KeyJPXDecode:
		return FilterJPXDecode, true
	case KeyCrypt:
		return FilterCrypt, true
	}
	return 0, false
}

// apply is Filter::apply in hayro-syntax src/filter/mod.rs. It decodes data
// with the filter.
func (f Filter) apply(data []byte, params Dict, imageParams *ImageDecodeParams, env *decodeEnv) (FilterResult, error) {
	streamFailure := func(ok bool, out []byte) (FilterResult, error) {
		if !ok {
			return FilterResult{}, &DecodeFailure{Kind: DecodeFailureStreamDecode}
		}
		if int64(len(out)) > env.maxMemory {
			return FilterResult{}, &DecodeFailure{Kind: DecodeFailureStreamDecode, Err: errLimit(env.maxMemory)}
		}
		return filterResultFromData(out), nil
	}
	imageFailure := func(res FilterResult, err error) (FilterResult, error) {
		if err != nil {
			return FilterResult{}, &DecodeFailure{Kind: DecodeFailureImageDecode, Err: err}
		}
		return res, nil
	}
	switch f {
	case FilterASCIIHexDecode:
		out, ok := decodeASCIIHex(data)
		return streamFailure(ok, out)
	case FilterASCII85Decode:
		out, ok := decodeASCII85(data)
		return streamFailure(ok, out)
	case FilterRunLengthDecode:
		out, ok := decodeRunLength(data, env.maxMemory)
		return streamFailure(ok, out)
	case FilterLZWDecode:
		out, err := decodeLzw(data, params, env.maxMemory)
		if err != nil {
			return FilterResult{}, &DecodeFailure{Kind: DecodeFailureStreamDecode, Err: err}
		}
		return filterResultFromData(out), nil
	case FilterFlateDecode:
		out, err := decodeFlate(data, params, env.maxMemory)
		if err != nil {
			return FilterResult{}, &DecodeFailure{Kind: DecodeFailureStreamDecode, Err: err}
		}
		return filterResultFromData(out), nil
	case FilterDCTDecode:
		return imageFailure(decodeDct(data, params, imageParams, env))
	case FilterCCITTFaxDecode:
		return imageFailure(decodeCcitt(data, params, imageParams, env))
	case FilterJBIG2Decode:
		return imageFailure(decodeJbig2(data, params, imageParams, env))
	case FilterJPXDecode:
		return imageFailure(decodeJpx(data, imageParams, env))
	}
	return FilterResult{}, &DecodeFailure{Kind: DecodeFailureStreamDecode}
}

// ImageDecoders holds the decoders of the image filters that this package
// does not hold. A caller fills them. A nil decoder makes its filter fail
// with ivy.ErrUnsupported.
//
// Port: upstream calls the crates hayro-jbig2 and hayro-jpeg2000 in
// src/filter/jbig2.rs and src/filter/jpx.rs. ivy ports those crates in
// other packages, so this package takes them as hooks. The code around each
// call, such as the packing of bits and the split of the alpha channel,
// stays here.
type ImageDecoders struct {
	// JBIG2 decodes the page of a JBIG2Decode stream. It stands for
	// hayro_jbig2::Page::new_embedded and Page::decode.
	JBIG2 JBIG2Func
	// JPX decodes a JPXDecode stream. It stands for hayro_jpeg2000::Image
	// and Image::decode.
	JPX JPXFunc
}

// decodeEnv holds the limits and the decoders that the filters use.
type decodeEnv struct {
	// maxMemory is the largest number of bytes of one decoded stream.
	maxMemory int64
	// maxPixels is the largest number of pixels of one decoded image.
	maxPixels int64
	// decoders holds the hooks of JBIG2 and JPX.
	decoders ImageDecoders
}

// defaultDecodeEnv returns the environment of a stream that has no xref
// table: the default limits of ivy, and no hooks.
func defaultDecodeEnv() *decodeEnv {
	lim := ivy.Limits{}.WithDefaults()
	return &decodeEnv{maxMemory: lim.MaxMemory, maxPixels: lim.MaxPixels}
}

// errLimit returns an error that wraps ivy.ErrLimit for a decoded stream
// larger than max bytes.
func errLimit(maxMemory int64) error {
	return fmt.Errorf("decoded stream exceeds %d bytes: %w", maxMemory, ivy.ErrLimit)
}
