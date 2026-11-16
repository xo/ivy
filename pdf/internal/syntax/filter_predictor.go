// Copyright (c) The Hayro Authors
//
// Use of this source code is governed by the Apache-2.0 or MIT license that
// can be found in the LICENSE-APACHE and LICENSE-MIT files.

// This file ports hayro-syntax src/filter/predictor.rs.

package syntax

import (
	"encoding/binary"
	"math/bits"
)

// applyPredictor is apply in hayro-syntax src/filter/predictor.rs. It undoes
// the TIFF or PNG predictor that params names.
func applyPredictor(data []byte, params Dict) ([]byte, bool) {
	predictor, ok := params.Get[uint32](KeyPredictor)
	if !ok {
		predictor = 1
	}
	if predictor == 1 {
		return data, true
	}
	colors, ok := params.Get[int](KeyColors)
	if !ok {
		colors = 1
	}
	bitsPerComponent, ok := params.Get[int](KeyBitsPerComponent)
	if !ok {
		bitsPerComponent = 8
	}
	columns, ok := params.Get[int](KeyColumns)
	if !ok {
		columns = 1
	}
	switch bitsPerComponent {
	case 1, 2, 4, 8, 16:
	default:
		return nil, false
	}
	bitsPerPixel, ok := checkedMul(colors, bitsPerComponent)
	if !ok {
		return nil, false
	}
	rowBits, ok := checkedMul(columns, bitsPerPixel)
	if !ok {
		return nil, false
	}
	rowLen := divCeil(rowBits, 8)
	if rowLen == 0 {
		return nil, false
	}
	switch {
	case predictor == 2:
		return tiffPredictor(data, colors, bitsPerComponent, columns, rowLen), true
	case predictor >= 10 && predictor <= 15:
		return pngPredictor(data, divCeil(bitsPerPixel, 8), rowLen), true
	}
	return nil, false
}

// tiffPredictor is tiff in hayro-syntax src/filter/predictor.rs.
func tiffPredictor(data []byte, colors, bitsPerComponent, columns, rowLen int) []byte {
	for start := 0; start < len(data); start += rowLen {
		row := data[start:min(start+rowLen, len(data))]
		switch bitsPerComponent {
		case 8:
			unfilterRow(RowFilterSub, colors, nil, row)
		case 16:
			distance := 2 * colors
			for i := distance; i < len(row)-1; i += 2 {
				left := binary.BigEndian.Uint16(row[i-distance:])
				value := binary.BigEndian.Uint16(row[i:]) + left
				binary.BigEndian.PutUint16(row[i:], value)
			}
		default:
			perByte := 8 / bitsPerComponent
			mask := byte(1)<<bitsPerComponent - 1
			shift := func(k int) int { return 8 - bitsPerComponent*(k%perByte+1) }
			numComponents := min(columns*colors, len(row)*perByte)
			for k := colors; k < numComponents; k++ {
				left := row[(k-colors)/perByte] >> shift(k-colors)
				b := &row[k/perByte]
				value := (*b>>shift(k) + left) & mask
				*b = *b&^(mask<<shift(k)) | value<<shift(k)
			}
		}
	}
	return data
}

// pngPredictor is png in hayro-syntax src/filter/predictor.rs.
func pngPredictor(data []byte, bytesPerPixel, rowLen int) []byte {
	src, dst := 0, 0
	for src < len(data) {
		filter := data[src]
		n := min(rowLen, len(data)-src-1)
		copy(data[dst:], data[src+1:src+1+n])
		done := data[:dst]
		previous := done[max(len(done)-rowLen, 0):]
		if f, ok := rowFilterFromU8(filter); ok {
			unfilterRow(f, bytesPerPixel, previous, data[dst:dst+n])
		}
		src += rowLen + 1
		dst += n
	}
	return data[:dst]
}

// checkedMul is usize::checked_mul of Rust, for values that are not
// negative.
func checkedMul(a, b int) (int, bool) {
	if a < 0 || b < 0 {
		return 0, false
	}
	hi, lo := bits.Mul64(uint64(a), uint64(b))
	if hi != 0 || lo > uint64(maxInt) {
		return 0, false
	}
	return int(lo), true
}

// maxInt is the largest int.
const maxInt = int(^uint(0) >> 1)

// divCeil is usize::div_ceil of Rust.
func divCeil(a, b int) int {
	q := a / b
	if a%b != 0 {
		q++
	}
	return q
}
