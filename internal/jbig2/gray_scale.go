// Copyright (c) The Hayro Authors
//
// Use of this source code is governed by the Apache-2.0 or MIT license that
// can be found in the LICENSE-APACHE and LICENSE-MIT files.

package jbig2

// This file ports hayro-jbig2 src/gray_scale.rs, the gray-scale image
// decoding procedure of T.88 Annex C.5.
//
// Port: upstream uses the SIMD vectors of fearless_simd in xor_planes and
// extract_bits. Go uses scalar loops. The macro gs_decode_loop! is a
// function that takes the gathering of the context as a function value.

// grayScaleParams is GrayScaleParams in hayro-jbig2 src/gray_scale.rs. A nil
// skipMask is the None of upstream.
type grayScaleParams struct {
	useMMR       bool
	bitsPerPixel uint32
	width        uint32
	height       uint32
	template     template
	skipMask     []uint32
	strict       bool
}

// decodeGrayScaleImage is decode_gray_scale_image in hayro-jbig2
// src/gray_scale.rs.
func decodeGrayScaleImage(data []byte, params *grayScaleParams, ctx *decodeState) (*grayScaleImage, error) {
	if params.useMMR {
		return decodeGrayScaleMMR(data, params, &ctx.budget, ctx.cancel)
	}
	return decodeGrayScaleArithmetic(data, params, ctx)
}

// decodeGrayScaleMMR is decode_mmr in hayro-jbig2 src/gray_scale.rs. The MMR
// data of each bitplane follows the data of the bitplane before it.
func decodeGrayScaleMMR(data []byte, params *grayScaleParams, budget *memoryBudget, c cancel) (*grayScaleImage, error) {
	offset := 0
	return decodeBitplanes(params.width, params.height, params.bitsPerPixel, budget, c,
		func(_ uint32, bitplane *bitmap) error {
			n, err := decodeBitmapMMR(bitplane, data[offset:], params.strict, c)
			if err != nil {
				return err
			}
			offset += n
			return nil
		})
}

// decodeGrayScaleArithmetic is decode_arithmetic in hayro-jbig2
// src/gray_scale.rs.
func decodeGrayScaleArithmetic(data []byte, params *grayScaleParams, ctx *decodeState) (*grayScaleImage, error) {
	width := params.width
	height := params.height

	if width > maxDimension || height > maxDimension {
		return nil, ErrOverflowBitmapDimension
	}

	skipMask := params.skipMask
	skipStride := divCeil(width, 32)
	tmpl := params.template

	var at [4]adaptiveTemplatePixel
	var gather gatherFunc
	switch tmpl {
	case template0:
		at = [4]adaptiveTemplatePixel{{3, -1}, {-3, -1}, {2, -2}, {-2, -2}}
		gather = (*contextGatherer).gatherTemplate0Default
	case template1:
		at[0] = adaptiveTemplatePixel{3, -1}
		gather = (*contextGatherer).gatherTemplate1Default
	case template2:
		at[0] = adaptiveTemplatePixel{2, -1}
		gather = (*contextGatherer).gatherTemplate2Default
	default:
		at[0] = adaptiveTemplatePixel{2, -1}
		gather = (*contextGatherer).gatherTemplate3Default
	}

	decoder := newArithmeticDecoder(data)
	c := ctx.cancel
	checkMask := rowMask(width)
	ctx.resetContexts(1 << tmpl.contextBits())
	contexts := ctx.contexts

	return decodeBitplanes(width, height, params.bitsPerPixel, &ctx.budget, c,
		func(_ uint32, bitplane *bitmap) error {
			g := newContextGatherer(tmpl, &at)

			for y := range height {
				if decoder.isExhausted() {
					return ErrUnexpectedEOF
				}

				if y&checkMask == checkMask {
					if err := c.check(); err != nil {
						return err
					}
				}

				g.startRow(bitplane, y)
				for x := range width {
					g.maybeReloadBuffers(bitplane, x)

					if skipMask != nil {
						i := int(y*skipStride + x/32)
						bitPos := 31 - x%32
						if (skipMask[i]>>bitPos)&1 != 0 {
							// The context must still move.
							gather(g, bitplane, x)
							continue
						}
					}

					context := gather(g, bitplane, x)
					pixel := decoder.readBit(&contexts[context])
					value := uint8(pixel)

					if value != 0 {
						bitplane.setPixel(x, y, value)
						g.updateCurrentRow(x, value)
					}
				}
			}

			return nil
		})
}

// grayScaleImage is GrayScaleImage in hayro-jbig2 src/gray_scale.rs. The
// plane j holds the bit j of each gray-scale value.
type grayScaleImage struct {
	width  uint32
	planes []*bitmap
}

// rowValues is GrayScaleImage::row_values. It writes the gray-scale values
// of the row y into row.
func (gi *grayScaleImage) rowValues(y uint32, row []uint32) {
	clear(row)

	for j, plane := range gi.planes {
		start := int(y) * int(plane.stride)
		data := plane.data[start : start+int(plane.stride)]
		extractBits(data, row, gi.width, 1, plane.stride, uint32(j))
	}
}

// decodeBitplanes is decode_bitplanes in hayro-jbig2 src/gray_scale.rs. It
// decodes the bitplanes from the most significant one, and it turns the
// Gray code of each bitplane into a binary value.
func decodeBitplanes(width, height, bitsPerPixel uint32, budget *memoryBudget, c cancel,
	decodeNext func(j uint32, bitplane *bitmap) error) (*grayScaleImage, error) {
	planes := make([]*bitmap, 0, bitsPerPixel)

	for j := int(bitsPerPixel) - 1; j >= 0; j-- {
		if err := c.check(); err != nil {
			return nil, err
		}
		bitplane, err := newBitmap(width, height, budget)
		if err != nil {
			return nil, err
		}
		if err := decodeNext(uint32(j), bitplane); err != nil {
			return nil, err
		}

		if len(planes) > 0 {
			xorPlanes(bitplane.data, planes[len(planes)-1].data)
		}

		planes = append(planes, bitplane)
	}

	for i, j := 0, len(planes)-1; i < j; i, j = i+1, j-1 {
		planes[i], planes[j] = planes[j], planes[i]
	}

	return &grayScaleImage{width: width, planes: planes}, nil
}

// xorPlanes is xor_planes in hayro-jbig2 src/gray_scale.rs.
func xorPlanes(plane, prev []uint32) {
	for i := range plane {
		plane[i] ^= prev[i]
	}
}

// extractBits is extract_bits in hayro-jbig2 src/gray_scale.rs. It sets the
// bit j of each value whose pixel is set in data.
func extractBits(data, values []uint32, width, height, stride, j uint32) {
	for y := range int(height) {
		rowData := y * int(stride)
		rowVals := y * int(width)

		for x := range int(width) {
			wordIdx := rowData + x/32
			bitPos := 31 - x%32
			if (data[wordIdx]>>bitPos)&1 != 0 {
				values[rowVals+x] |= 1 << j
			}
		}
	}
}
