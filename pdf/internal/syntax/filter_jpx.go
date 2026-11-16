// Copyright (c) The Hayro Authors
//
// Use of this source code is governed by the Apache-2.0 or MIT license that
// can be found in the LICENSE-APACHE and LICENSE-MIT files.

// This file ports hayro-syntax src/filter/jpx.rs.
//
// Port: upstream calls the crate hayro-jpeg2000. This package calls the hook
// ImageDecoders.JPX in its place, and keeps the code around the call.

package syntax

import (
	"fmt"

	"github.com/xo/ivy"
)

// JPXFunc decodes a JPEG 2000 stream. channels is the number of color
// channels that the PDF states for a raw codestream, or 0. A call with 0
// stands for hayro_jpeg2000::Image::new, and a call with a number for
// Image::from_codestream.
type JPXFunc func(data []byte, channels uint8, settings JPXSettings) (JPXImage, error)

// JPXSettings is the subset of hayro_jpeg2000::DecodeSettings that jpx.rs
// sets. upstream also sets resolve_palette_indices and strict to false.
type JPXSettings struct {
	// TargetResolution is a hint for the size of the decoded image, or
	// zero.
	TargetResolution [2]uint32
}

// JPXColorSpaceKind is the kind of the color space of a JPXImage. It
// follows hayro_jpeg2000::ColorSpace.
type JPXColorSpaceKind uint8

// The kinds of JPXColorSpaceKind.
const (
	// JPXGray is ColorSpace::Gray.
	JPXGray JPXColorSpaceKind = iota
	// JPXRGB is ColorSpace::RGB.
	JPXRGB
	// JPXCMYK is ColorSpace::CMYK.
	JPXCMYK
	// JPXUnknown is ColorSpace::Unknown.
	JPXUnknown
	// JPXICC is ColorSpace::Icc.
	JPXICC
)

// JPXImage is the image that a JPXFunc decodes.
type JPXImage struct {
	// Width is the width of the image.
	Width uint32
	// Height is the height of the image.
	Height uint32
	// ColorSpace is the kind of the color space.
	ColorSpace JPXColorSpaceKind
	// NumChannels is the number of channels of JPXUnknown and JPXICC.
	NumChannels uint8
	// OriginalBitDepth is the precision of the first component, as
	// Image::original_bit_depth gives it.
	OriginalBitDepth uint8
	// HasAlpha is true when the last channel is alpha.
	HasAlpha bool
	// Data holds the samples with 8 bits each, the channels of a pixel
	// together, as DecodedImage::data_u8 gives them.
	Data []byte
}

// numComponents is ImageColorSpace::num_components in hayro-syntax
// src/filter/jpx.rs.
func (c ImageColorSpace) numComponents() uint8 {
	switch c.Kind {
	case ImageColorSpaceGray:
		return 1
	case ImageColorSpaceRGB:
		return 3
	case ImageColorSpaceCMYK:
		return 4
	}
	return c.Unknown
}

// decodeJpx is decode in hayro-syntax src/filter/jpx.rs.
func decodeJpx(data []byte, params *ImageDecodeParams, env *decodeEnv) (FilterResult, error) {
	if env.decoders.JPX == nil {
		return FilterResult{}, fmt.Errorf("decoding jpx: no decoder: %w", ivy.ErrUnsupported)
	}
	settings := JPXSettings{TargetResolution: params.TargetDimension}
	var image JPXImage
	var err error
	if params.NumComponents != 0 && !params.IsIndexed {
		image, err = env.decoders.JPX(data, params.NumComponents, settings)
	}
	if params.NumComponents == 0 || params.IsIndexed || err != nil {
		image, err = env.decoders.JPX(data, 0, settings)
	}
	if err != nil {
		return FilterResult{}, fmt.Errorf("decoding jpx: %w", err)
	}
	width, height := image.Width, image.Height
	bpc := uint8(8)
	if params.IsIndexed {
		bpc = image.OriginalBitDepth
		if params.Bpc != 0 {
			bpc = params.Bpc
		}
	}
	var cs ImageColorSpace
	switch image.ColorSpace {
	case JPXGray:
		cs = ImageColorSpace{Kind: ImageColorSpaceGray}
	case JPXRGB:
		cs = ImageColorSpace{Kind: ImageColorSpaceRGB}
	case JPXCMYK:
		cs = ImageColorSpace{Kind: ImageColorSpaceCMYK}
	case JPXUnknown:
		cs = ImageColorSpace{Kind: ImageColorSpaceUnknown, Unknown: image.NumChannels}
	default:
		switch image.NumChannels {
		case 1:
			cs = ImageColorSpace{Kind: ImageColorSpaceGray}
		case 3:
			cs = ImageColorSpace{Kind: ImageColorSpaceRGB}
		case 4:
			cs = ImageColorSpace{Kind: ImageColorSpaceCMYK}
		default:
			return FilterResult{}, fmt.Errorf("decoding jpx: icc profile with %d channels: %w", image.NumChannels, ivy.ErrUnsupported)
		}
	}
	bitmap := image.Data
	decoded, alpha := bitmap, []byte(nil)
	if image.HasAlpha {
		total := int(cs.numComponents()) + 1
		color := make([]byte, 0, len(bitmap)/total*(total-1))
		alpha = make([]byte, 0, len(bitmap)/total)
		for i := 0; i+total <= len(bitmap); i += total {
			alpha = append(alpha, bitmap[i+total-1])
			color = append(color, bitmap[i:i+total-1]...)
		}
		decoded = color
	}
	// The decoder always gives 8 bits.
	if bpc != 8 {
		scaled, ok := scaleJpx(decoded, bpc, cs.numComponents(), width, height, env.maxMemory)
		if !ok {
			return FilterResult{}, fmt.Errorf("decoding jpx: scaling to %d bits: %w", bpc, ivy.ErrFormat)
		}
		decoded = scaled
		if alpha != nil {
			alpha, _ = scaleJpx(alpha, bpc, cs.numComponents(), width, height, env.maxMemory)
		}
	}
	return FilterResult{
		Data: decoded,
		ImageData: &ImageData{
			Alpha:            alpha,
			ColorSpace:       &cs,
			BitsPerComponent: bpc,
			Width:            width,
			Height:           height,
		},
	}, nil
}

// scaleJpx is scale in hayro-syntax src/filter/jpx.rs. It packs samples of
// 8 bits into samples of bitsPerComponent bits.
//
// Port: it also fails past maxMemory bytes.
func scaleJpx(data []byte, bitsPerComponent, numComponents uint8, width, height uint32, maxMemory int64) ([]byte, bool) {
	if bitsPerComponent == 0 || bitsPerComponent > 32 {
		return nil, false
	}
	divFactor := float32(255)
	mulFactor := float32(uint64(1)<<bitsPerComponent - 1)
	bitsPerRow := uint64(width) * uint64(numComponents) * uint64(bitsPerComponent)
	inputLen := (bitsPerRow + 7) / 8 * uint64(height)
	if inputLen > uint64(maxMemory) {
		return nil, false
	}
	input := make([]byte, inputLen)
	writer, ok := NewBitWriter(input, bitsPerComponent)
	if !ok {
		return nil, false
	}
	componentsPerRow := int(numComponents) * int(width)
	if componentsPerRow == 0 {
		return input[:0], true
	}
	for start := 0; start+componentsPerRow <= len(data); start += componentsPerRow {
		for _, b := range data[start : start+componentsPerRow] {
			scaled := f32ToU32((float32(b)/divFactor)*mulFactor + 0.5)
			if !writer.Write(scaled) {
				return nil, false
			}
		}
		writer.Align()
	}
	// Upstream truncates to the number of bits that it wrote, which keeps
	// every byte.
	return input[:min(writer.CurPos(), len(input))], true
}
