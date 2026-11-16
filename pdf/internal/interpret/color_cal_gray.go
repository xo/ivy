// Copyright (c) The Hayro Authors.
//
// Use of this source code is governed by the Apache-2.0 or MIT license that
// can be found in the LICENSE-APACHE and LICENSE-MIT files.

// This file ports hayro-interpret src/color/cal_gray.rs.

package interpret

import "github.com/xo/ivy/pdf/internal/syntax"

// calGray is CalGray in hayro-interpret src/color/cal_gray.rs. Upstream
// follows the CalGray of pdf.js, in src/core/colorspace.js at the commit
// 06f44916.
type calGray struct {
	whitePoint [3]float32
	blackPoint [3]float32
	gamma      float32
	lookup     u8Lookup[uint8]
}

// newCalGray is CalGray::new in hayro-interpret src/color/cal_gray.rs.
func newCalGray(dict syntax.Dict) *calGray {
	whitePoint, ok := dict.Get[[3]float32](syntax.KeyWhitePoint)
	if !ok {
		whitePoint = [3]float32{1, 1, 1}
	}
	blackPoint, _ := dict.Get[[3]float32](syntax.KeyBlackPoint)
	gamma, ok := dict.Get[float32](syntax.KeyGamma)
	if !ok {
		gamma = 1
	}
	return &calGray{whitePoint: whitePoint, blackPoint: blackPoint, gamma: gamma}
}

// convertValue is CalGray::convert_value in hayro-interpret
// src/color/cal_gray.rs.
func (c *calGray) convertValue(input uint8) uint8 {
	yw := c.whitePoint[1]
	a := float32(input) / 255
	ag := powf32(a, c.gamma)
	l := yw * ag
	return asU8(rustMax(0, 295.8*powf32(l, 0.333_333_34)-40.8) + 0.5)
}

// u8Lookup is CalGray::u8_lookup in hayro-interpret
// src/color/cal_gray.rs.
func (c *calGray) u8Lookup() *[256]uint8 {
	return c.lookup.getOrInitWith(func(input *[256]uint8) *[256]uint8 {
		var table [256]uint8
		for i := range table {
			table[i] = c.convertValue(input[i])
		}
		return &table
	})
}

// convert is the ToRgb implementation of CalGray in hayro-interpret
// src/color/cal_gray.rs.
func (c *calGray) convert(input, output []uint8) bool {
	lookup := c.u8Lookup()
	n := min(len(input), len(output)/3)
	for i := range n {
		v := lookup[input[i]]
		output[i*3], output[i*3+1], output[i*3+2] = v, v, v
	}
	return true
}

// convertInPlace is the default of ToRgb::convert_in_place.
func (c *calGray) convertInPlace([]uint8) bool { return false }

// isNone is the default of ToRgb::is_none.
func (c *calGray) isNone() bool { return false }

// toLuma is the ToLuma implementation of CalGray in hayro-interpret
// src/color/cal_gray.rs.
func (c *calGray) toLuma(input []uint8) bool {
	lookup := c.u8Lookup()
	for i, v := range input {
		input[i] = lookup[v]
	}
	return true
}
