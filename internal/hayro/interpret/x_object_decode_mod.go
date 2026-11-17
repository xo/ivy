// Copyright (c) The Hayro Authors.
//
// Use of this source code is governed by the Apache-2.0 or MIT license that
// can be found in the LICENSE-APACHE and LICENSE-MIT files.

// This file ports hayro-interpret src/x_object/decode/mod.rs.

package interpret

import (
	"slices"

	"github.com/xo/ivy/internal/hayro/syntax"
)

// decodeCtx is DecodeContext in hayro-interpret src/x_object/decode/mod.rs.
type decodeCtx struct {
	decoded          syntax.FilterResult
	width            uint32
	height           uint32
	scaleFactors     [2]float32
	colorSpace       ColorSpace
	bitsPerComponent uint8
	decodeArr        [][2]float32
}

// decodeContext is decode_context in hayro-interpret
// src/x_object/decode/mod.rs. It decodes the stream of the image, and
// gathers what the decoding of its samples needs.
func decodeContext(obj *imageXObject, targetDimension [2]uint32) (*decodeCtx, bool) {
	dict := obj.stream.Dict()
	dictBpc, hasDictBpc := dict.Get[uint8](syntax.KeyBPC)
	if !hasDictBpc {
		dictBpc, hasDictBpc = dict.Get[uint8](syntax.KeyBitsPerComponent)
	}
	params := &syntax.ImageDecodeParams{
		IsIndexed:       obj.hasColorSpace && obj.colorSpace.isIndexed(),
		TargetDimension: targetDimension,
		Width:           obj.width,
		Height:          obj.height,
		Decoders:        obj.env.decoders,
	}
	if hasDictBpc {
		params.Bpc = dictBpc
	}
	if obj.hasColorSpace {
		params.NumComponents = obj.colorSpace.NumComponents()
	}
	decoded, err := obj.stream.DecodedImage(params)
	if err != nil {
		obj.env.warn(InterpreterWarningImageDecodeFailure)
		return nil, false
	}
	scaleX, scaleY := float32(1), float32(1)
	width, height := obj.width, obj.height
	if d := decoded.ImageData; d != nil {
		scaleX = float32(obj.width) / float32(d.Width)
		scaleY = float32(obj.height) / float32(d.Height)
		width, height = d.Width, d.Height
	}
	colorSpace, hasColorSpace := obj.colorSpace, obj.hasColorSpace
	if !hasColorSpace && decoded.ImageData != nil && decoded.ImageData.ColorSpace != nil {
		switch decoded.ImageData.ColorSpace.Kind {
		case syntax.ImageColorSpaceGray:
			colorSpace, hasColorSpace = DeviceGray(), true
		case syntax.ImageColorSpaceRGB:
			colorSpace, hasColorSpace = DeviceRGB(), true
		case syntax.ImageColorSpaceCMYK:
			colorSpace, hasColorSpace = DeviceCMYK(), true
		}
	}
	if !hasColorSpace {
		colorSpace = DeviceGray()
	}
	// Port: upstream allocates the samples for the size that the
	// dictionary states, which can be 4 billion pixels on each side. The
	// limits of the settings bound them (D18).
	pixels := uint64(width) * uint64(height)
	if pixels > uint64(obj.env.limits.MaxPixels) || pixels*uint64(max(colorSpace.NumComponents(), 3)) > uint64(obj.env.limits.MaxMemory) {
		obj.env.warn(InterpreterWarningImageDecodeFailure)
		return nil, false
	}
	fallbackBpc := uint8(8)
	if obj.kind == imageKindStencilMask {
		fallbackBpc = 1
	}
	bpc := fallbackBpc
	switch {
	case decoded.ImageData != nil:
		bpc = decoded.ImageData.BitsPerComponent
	case hasDictBpc:
		bpc = dictBpc
	}
	decodeArr, ok := decodeArrOf(dict)
	if !ok {
		decodeArr = colorSpace.defaultDecodeArr(float32(bpc))
	}
	return &decodeCtx{
		decoded:          decoded,
		width:            width,
		height:           height,
		scaleFactors:     [2]float32{scaleX, scaleY},
		colorSpace:       colorSpace,
		bitsPerComponent: bpc,
		decodeArr:        decodeArr,
	}, true
}

// decodeArrOf reads the array D or Decode of an image.
func decodeArrOf(dict syntax.Dict) ([][2]float32, bool) {
	arr, ok := dict.Get[syntax.Array](syntax.KeyD)
	if !ok {
		if arr, ok = dict.Get[syntax.Array](syntax.KeyDecode); !ok {
			return nil, false
		}
	}
	var out [][2]float32
	for p := range arr.Iter[syntax.Tuple2[float32, float32]]().All() {
		out = append(out, [2]float32{p.V0, p.V1})
	}
	return out, true
}

// fixImageLength is fix_image_length in hayro-interpret
// src/x_object/decode/mod.rs. It cuts the image to its height, or it lowers
// the height to the rows that the data holds and fills the last row. It
// fails for an empty image.
func fixImageLength[T any](image *[]T, width uint32, height *uint32, filler T, numComponents int) bool {
	rowLen := int(width) * numComponents
	if rowLen == 0 {
		// Port: upstream divides by the length of a row, which is never 0
		// for an image that it decodes.
		return false
	}
	if rowLen*int(*height) <= len(*image) {
		*image = (*image)[:rowLen*int(*height)]
	} else {
		*height = uint32((len(*image) + rowLen - 1) / rowLen)
		if rem := len(*image) % rowLen; rem != 0 {
			for range rowLen - rem {
				*image = append(*image, filler)
			}
		}
	}
	return width != 0 && *height != 0
}

// decodeU8Samples is decode_u8_samples in hayro-interpret
// src/x_object/decode/mod.rs. It decodes the samples into components of 8
// bits, through the decode array and the ranges of the color space.
func decodeU8Samples(data []byte, width, height uint32, colorSpace ColorSpace, bitsPerComponent uint8, decode [][2]float32) ([]uint8, bool) {
	sourceMax := powf32(2, float32(bitsPerComponent)) - 1
	numComponents := int(colorSpace.NumComponents())
	capacity := int(width) * int(height) * numComponents
	ranges := colorSpace.componentRanges()
	hival, isIndexed := colorSpace.indexedHival()
	if numComponents == 0 {
		return nil, true
	}
	decodeComponent := func(value uint32, componentIndex int) (uint8, bool) {
		if componentIndex >= len(decode) {
			return 0, false
		}
		d := decode[componentIndex]
		decoded := interpolate(float32(value), 0, sourceMax, d[0], d[1])
		if isIndexed {
			return asU8(rustClamp(decoded+0.5, 0, float32(hival))), true
		}
		if componentIndex >= len(ranges) {
			return 0, false
		}
		r := ranges[componentIndex]
		var normalized float32
		if r[0] != r[1] {
			normalized = (decoded - r[0]) / (r[1] - r[0])
		}
		return asU8(normalized*255 + 0.5), true
	}
	switch {
	case bitsPerComponent == 1 || bitsPerComponent == 2 || bitsPerComponent == 4 || bitsPerComponent == 8:
		numValues := 1 << bitsPerComponent
		lut := make([]uint8, 0, numComponents*numValues)
		for component := range numComponents {
			for value := range numValues {
				v, ok := decodeComponent(uint32(value), component)
				if !ok {
					return nil, false
				}
				lut = append(lut, v)
			}
		}
		if bitsPerComponent == 8 {
			return decode8bitSamples(data, numComponents, lut), true
		}
		return decodePackedSamples(data, width, height, numComponents, bitsPerComponent, lut), true
	case bitsPerComponent >= 1 && bitsPerComponent < 16:
		buf := make([]uint8, 0, capacity)
		ok := forEachSample(data, width, height, numComponents, bitsPerComponent, func(value uint32, index int) bool {
			v, ok := decodeComponent(value, index%numComponents)
			if !ok {
				return false
			}
			buf = append(buf, v)
			return true
		})
		return buf, ok
	case bitsPerComponent == 16:
		buf := make([]uint8, 0, len(data)/2)
		for i := 0; i+2 <= len(data); i += 2 {
			v, ok := decodeComponent(uint32(data[i])<<8|uint32(data[i+1]), (i/2)%numComponents)
			if !ok {
				return nil, false
			}
			buf = append(buf, v)
		}
		return buf, true
	}
	return nil, false
}

// decode8bitSamples is decode_8bit_samples in hayro-interpret
// src/x_object/decode/mod.rs.
func decode8bitSamples(data []byte, numComponents int, lut []uint8) []uint8 {
	if numComponents == 1 {
		out := make([]uint8, len(data))
		for i, v := range data {
			out[i] = lut[v]
		}
		return out
	}
	buf := make([]uint8, len(data))
	for i, v := range data {
		buf[i] = lut[(i%numComponents)*256+int(v)]
	}
	return buf
}

// decodePackedSamples is decode_packed_samples in hayro-interpret
// src/x_object/decode/mod.rs. It decodes samples of 1, 2 or 4 bits, row by
// row.
func decodePackedSamples(data []byte, width, height uint32, numComponents int, bitsPerComponent uint8, lut []uint8) []uint8 {
	bpc := int(bitsPerComponent)
	samplesPerByte := 8 / bpc
	numValues := 1 << bpc
	mask := uint8(numValues - 1)
	rowSamples := int(width) * numComponents
	rowBytes := (rowSamples*bpc + 7) / 8
	rowData := func(row int) []byte {
		start := row * rowBytes
		if start > len(data) {
			return nil
		}
		return data[start:]
	}
	if rowSamples == 0 {
		return nil
	}
	if numComponents == 1 {
		var expanded [256][8]uint8
		for b := range expanded {
			for i := range samplesPerByte {
				value := uint8(b>>(8-bpc*(i+1))) & mask
				expanded[b][i] = lut[value]
			}
		}
		buf := make([]uint8, rowSamples*int(height))
		for index := range int(height) {
			expandRow(rowData(index), buf[index*rowSamples:(index+1)*rowSamples], &expanded, samplesPerByte)
		}
		return buf
	}
	buf := make([]uint8, 0, rowSamples*int(height))
	for row := range int(height) {
		rd := rowData(row)
		component := 0
		for sample := range rowSamples {
			bit := sample * bpc
			var b uint8
			if bit/8 < len(rd) {
				b = rd[bit/8]
			}
			value := (b >> (8 - bpc - bit%8)) & mask
			buf = append(buf, lut[component*numValues+int(value)])
			component++
			if component == numComponents {
				component = 0
			}
		}
	}
	return buf
}

// expandRow is expand_row in hayro-interpret src/x_object/decode/mod.rs. n
// is the number of samples in a byte.
func expandRow(row, out []uint8, expanded *[256][8]uint8, n int) {
	chunks := len(out) / n
	available := min(chunks, len(row))
	for i := range available {
		copy(out[i*n:(i+1)*n], expanded[row[i]][:n])
	}
	for i := available; i < chunks; i++ {
		copy(out[i*n:(i+1)*n], expanded[0][:n])
	}
	if rem := out[chunks*n:]; len(rem) > 0 {
		var b uint8
		if chunks < len(row) {
			b = row[chunks]
		}
		copy(rem, expanded[b][:len(rem)])
	}
}

// unpackSamples is unpack_samples in hayro-interpret
// src/x_object/decode/mod.rs. It returns the raw value of each sample.
func unpackSamples(data []byte, width, height uint32, numComponents int, bitsPerComponent uint8) ([]uint16, bool) {
	switch {
	case bitsPerComponent == 8:
		out := make([]uint16, len(data))
		for i, v := range data {
			out[i] = uint16(v)
		}
		return out, true
	case bitsPerComponent == 16:
		out := make([]uint16, 0, len(data)/2)
		for i := 0; i+2 <= len(data); i += 2 {
			out = append(out, uint16(data[i])<<8|uint16(data[i+1]))
		}
		return out, true
	case bitsPerComponent >= 1 && bitsPerComponent < 16:
		buf := make([]uint16, 0, int(width)*int(height)*numComponents)
		ok := forEachSample(data, width, height, numComponents, bitsPerComponent, func(value uint32, _ int) bool {
			buf = append(buf, uint16(value))
			return true
		})
		return buf, ok
	}
	return nil, false
}

// forEachSample is for_each_sample in hayro-interpret
// src/x_object/decode/mod.rs. It visits each sample, row by row. A sample
// past the end of the data is 0.
func forEachSample(data []byte, width, height uint32, numComponents int, bitsPerComponent uint8, visit func(value uint32, index int) bool) bool {
	reader := syntax.NewBitReader(data)
	index := 0
	for range height {
		for range width {
			for range numComponents {
				// Some images lack data. They are padded with zeros.
				v, _ := reader.Read(bitsPerComponent)
				if !visit(v, index) {
					return false
				}
				index++
			}
		}
		reader.Align()
	}
	return true
}

// equalRanges compares two decode arrays.
func equalRanges(a, b [][2]float32) bool {
	return slices.Equal(a, b)
}
