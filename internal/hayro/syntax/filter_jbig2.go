// Copyright (c) The Hayro Authors
//
// Use of this source code is governed by the Apache-2.0 or MIT license that
// can be found in the LICENSE-APACHE and LICENSE-MIT files.

// This file ports hayro-syntax src/filter/jbig2.rs.
//
// Port: upstream calls the crate hayro-jbig2. This package calls the hook
// ImageDecoders.JBIG2 in its place, and keeps the code around the call.

package syntax

import (
	"fmt"

	"github.com/xo/ivy"
)

// JBIG2Func decodes the page of an embedded JBIG2 stream. globals holds the
// decoded data of the stream JBIG2Globals, or nil. It stands for
// hayro_jbig2::Globals::new, Page::new_embedded and Page::decode.
type JBIG2Func func(data, globals []byte) (JBIG2Bitmap, error)

// JBIG2Bitmap is the page that a JBIG2Func decodes.
type JBIG2Bitmap struct {
	// Width is the width of the page in pixels.
	Width uint32
	// Height is the height of the page in pixels.
	Height uint32
	// Data holds the rows of the page, each packed into (Width+7)/8 bytes,
	// with the first pixel in the high bit. A set bit is black, as in
	// JBIG2.
	Data []byte
}

// decodeJbig2 is decode in hayro-syntax src/filter/jbig2.rs.
func decodeJbig2(data []byte, params Dict, imageParams *ImageDecodeParams, env *decodeEnv) (FilterResult, error) {
	if env.decoders.JBIG2 == nil {
		return FilterResult{}, fmt.Errorf("decoding jbig2: no decoder: %w", ivy.ErrUnsupported)
	}
	var globals []byte
	if g, ok := params.Get[Stream](KeyJBIG2Globals); ok {
		if d, err := g.DecodedData(); err == nil {
			globals = d
		}
	}
	bitmap, err := env.decoders.JBIG2(data, globals)
	if err != nil {
		return FilterResult{}, fmt.Errorf("decoding jbig2: %w", err)
	}
	width := int64(bitmap.Width)
	height := int64(bitmap.Height)
	rowBytes := (width + 7) / 8
	if width*height > env.maxPixels || width*height > env.maxMemory {
		return FilterResult{}, fmt.Errorf("decoding jbig2: image of %dx%d: %w", width, height, ivy.ErrLimit)
	}
	if int64(len(bitmap.Data)) < rowBytes*height {
		return FilterResult{}, fmt.Errorf("decoding jbig2: short bitmap: %w", ivy.ErrFormat)
	}
	// Expand to 8 bits per component unless the image is indexed, for an
	// easier conversion to RGBA8. JBIG2 uses 1 for black, and PDF uses 0.
	var decoded []byte
	var bpc uint8
	if imageParams.IsIndexed {
		packed := make([]byte, 0, rowBytes*height)
		paddingMask := byte(0xFF) << ((8 - width%8) % 8)
		for y := range height {
			row := bitmap.Data[y*rowBytes : (y+1)*rowBytes]
			for _, b := range row {
				packed = append(packed, ^b)
			}
			if len(packed) > 0 {
				packed[len(packed)-1] &= paddingMask
			}
		}
		decoded, bpc = packed, 1
	} else {
		// This is Bitmap::data_u8 of hayro-jbig2: black is 0 and white is
		// 255.
		decoded = make([]byte, width*height)
		for y := range height {
			for x := range width {
				if bitmap.Data[y*rowBytes+x/8]>>(7-x%8)&1 == 0 {
					decoded[y*width+x] = 0xFF
				}
			}
		}
		bpc = 8
	}
	return FilterResult{
		Data: decoded,
		ImageData: &ImageData{
			ColorSpace:       &ImageColorSpace{Kind: ImageColorSpaceGray},
			BitsPerComponent: bpc,
			Width:            bitmap.Width,
			Height:           bitmap.Height,
		},
	}, nil
}
