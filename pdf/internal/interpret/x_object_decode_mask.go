// Copyright (c) The Hayro Authors.
//
// Use of this source code is governed by the Apache-2.0 or MIT license that
// can be found in the LICENSE-APACHE and LICENSE-MIT files.

// This file ports hayro-interpret src/x_object/decode/mask.rs.

package interpret

import "slices"

// bilevelMaskLUT is BILEVEL_MASK_LUT in hayro-interpret
// src/x_object/decode/mask.rs. It expands each byte into 8 pixels.
//
// Port: upstream packs the 8 pixels into a u64. Go keeps the bytes.
var bilevelMaskLUT = func() *[256][8]uint8 {
	var lut [256][8]uint8
	for b := range lut {
		for bit := range 8 {
			if b&(0x80>>bit) != 0 {
				lut[b][bit] = 255
			}
		}
	}
	return &lut
}()

// decodedMask is DecodedMask in hayro-interpret
// src/x_object/decode/mask.rs.
type decodedMask struct {
	luma LumaData
}

// decodeMask is decode_mask in hayro-interpret src/x_object/decode/mask.rs.
func decodeMask(obj *imageXObject, targetDimension [2]uint32) (decodedMask, bool) {
	ctx, ok := decodeContext(obj, targetDimension)
	if !ok {
		return decodedMask{}, false
	}
	width := ctx.width
	scaleFactors := ctx.scaleFactors
	// A stencil mask uses 1 for painted, the inverse of a soft mask.
	invert := obj.kind == imageKindStencilMask
	data, height, ok := decodeMaskData(ctx, invert)
	if !ok {
		return decodedMask{}, false
	}
	return decodedMask{luma: LumaData{
		Data:         data,
		Width:        width,
		Height:       height,
		Interpolate:  obj.interpolate,
		ScaleFactors: scaleFactors,
	}}, true
}

// decodeMaskData is decode_mask_data in hayro-interpret
// src/x_object/decode/mask.rs.
func decodeMaskData(ctx *decodeCtx, invert bool) ([]uint8, uint32, bool) {
	defaultDecode := ctx.colorSpace.defaultDecodeArr(float32(ctx.bitsPerComponent))
	invertedDefault := ctx.colorSpace.invertedDefaultDecodeArr(float32(ctx.bitsPerComponent))
	isDefault := equalRanges(ctx.decodeArr, defaultDecode)
	isInverted := equalRanges(ctx.decodeArr, invertedDefault)
	numComponents := int(ctx.colorSpace.NumComponents())
	bilevelFastPath := ctx.bitsPerComponent == 1 && numComponents == 1 && (isDefault || isInverted)
	fastPath := ctx.bitsPerComponent == 8 && (isDefault || isInverted)

	var data []uint8
	switch {
	case bilevelFastPath:
		data = decodeBilevelMaskData(ctx, invert != isInverted)
	case fastPath:
		// Port: the decoded data can be the data of the file itself, so the
		// port copies it, as Cow::to_mut and into_owned of upstream do.
		data = slices.Clone(ctx.decoded.Data)
		if invert != isInverted {
			for i, v := range data {
				data[i] = 255 - v
			}
		}
	default:
		components, ok := unpackSamples(ctx.decoded.Data, ctx.width, ctx.height, numComponents, ctx.bitsPerComponent)
		if !ok {
			return nil, 0, false
		}
		sourceMax := powf32(2, float32(ctx.bitsPerComponent)) - 1
		data = make([]uint8, 0, len(components))
		if numComponents > 0 {
			for i := 0; i < len(components); i += numComponents {
				pixel := components[i:min(i+numComponents, len(components))]
				for j, c := range pixel {
					if j >= len(ctx.decodeArr) {
						break
					}
					d := ctx.decodeArr[j]
					value := interpolate(float32(c), 0, sourceMax, d[0], d[1])
					if invert {
						value = 1 - value
					}
					data = append(data, asU8(value*255+0.5))
				}
			}
		}
	}
	height := ctx.height
	if !fixImageLength(&data, ctx.width, &height, 0, numComponents) {
		return nil, 0, false
	}
	return data, height, true
}

// decodeBilevelMaskData is decode_bilevel_mask_data in hayro-interpret
// src/x_object/decode/mask.rs. It expands a mask of 1 bit into bytes.
func decodeBilevelMaskData(ctx *decodeCtx, shouldInvert bool) []uint8 {
	width := int(ctx.width)
	height := int(ctx.height)
	rowBytes := (width + 7) / 8
	data := ctx.decoded.Data
	var xor uint8
	if shouldInvert {
		xor = 0xff
	}
	fullBytes := width / 8
	tailBits := width % 8
	lut := bilevelMaskLUT
	out := make([]uint8, 0, width*height)
	if rowBytes == 0 {
		return out
	}
	for row := 0; row < height && row*rowBytes < len(data); row++ {
		rd := data[row*rowBytes : min((row+1)*rowBytes, len(data))]
		for _, b := range rd[:min(len(rd), fullBytes)] {
			for _, v := range lut[b] {
				out = append(out, v^xor)
			}
		}
		if tailBits != 0 && len(rd) > fullBytes {
			for _, v := range lut[rd[fullBytes]][:tailBits] {
				out = append(out, v^xor)
			}
		}
	}
	return out
}
