// Copyright (c) The Hayro Authors.
//
// Use of this source code is governed by the Apache-2.0 or MIT license that
// can be found in the LICENSE-APACHE and LICENSE-MIT files.

// This file ports hayro-interpret src/color/cal_rgb.rs.

package interpret

import "github.com/xo/ivy/pdf/internal/syntax"

// calRGB is CalRgb in hayro-interpret src/color/cal_rgb.rs. Upstream ports
// the CalRGB of pdf.js, in src/core/colorspace.js at the commit 06f44916.
type calRGB struct {
	whitePoint [3]float32
	blackPoint [3]float32
	matrix     [9]float32
	gamma      [3]float32
}

// newCalRGB is CalRgb::new in hayro-interpret src/color/cal_rgb.rs.
func newCalRGB(dict syntax.Dict) *calRGB {
	whitePoint, ok := dict.Get[[3]float32](syntax.KeyWhitePoint)
	if !ok {
		whitePoint = [3]float32{1, 1, 1}
	}
	blackPoint, _ := dict.Get[[3]float32](syntax.KeyBlackPoint)
	matrix, ok := dict.Get[[9]float32](syntax.KeyMatrix)
	if !ok {
		matrix = [9]float32{1, 0, 0, 0, 1, 0, 0, 0, 1}
	}
	gamma, ok := dict.Get[[3]float32](syntax.KeyGamma)
	if !ok {
		gamma = [3]float32{1, 1, 1}
	}
	return &calRGB{whitePoint: whitePoint, blackPoint: blackPoint, matrix: matrix, gamma: gamma}
}

// The constants of CalRgb in hayro-interpret src/color/cal_rgb.rs.
var (
	bradfordScaleMatrix = [9]float32{
		0.8951, 0.2664, -0.1614, -0.7502, 1.7135, 0.0367, 0.0389, -0.0685, 1.0296,
	}
	bradfordScaleInverseMatrix = [9]float32{
		0.9869929, -0.1470543, 0.1599627, 0.4323053, 0.5183603, 0.0492912, -0.0085287, 0.0400428,
		0.9684867,
	}
	srgbD65XYZToRGBMatrix = [9]float32{
		3.2404542, -1.5371385, -0.4985314, -0.969_266, 1.8760108, 0.0415560, 0.0556434, -0.2040259,
		1.0572252,
	}
	flatWhitepoint = [3]float32{1, 1, 1}
	d65Whitepoint  = [3]float32{0.95047, 1, 1.08883}
)

// decodeLConstant is CalRgb::decode_l_constant in hayro-interpret
// src/color/cal_rgb.rs.
func decodeLConstant() float32 {
	v := (8.0 + 16.0) / float32(116.0)
	return v * v * v / 8
}

// srgbTransferFunction is CalRgb::srgb_transfer_function in hayro-interpret
// src/color/cal_rgb.rs.
func srgbTransferFunction(color float32) float32 {
	switch {
	case color <= 0.0031308:
		return rustClamp(12.92*color, 0, 1)
	case color >= 0.99554525:
		return 1
	}
	return rustClamp((1.0+0.055)*powf32(color, 1.0/2.4)-0.055, 0, 1)
}

// matrixProduct is CalRgb::matrix_product in hayro-interpret
// src/color/cal_rgb.rs.
func matrixProduct(a *[9]float32, b [3]float32) [3]float32 {
	return [3]float32{
		a[0]*b[0] + a[1]*b[1] + a[2]*b[2],
		a[3]*b[0] + a[4]*b[1] + a[5]*b[2],
		a[6]*b[0] + a[7]*b[1] + a[8]*b[2],
	}
}

// toFlat is CalRgb::to_flat in hayro-interpret src/color/cal_rgb.rs.
func toFlat(sourceWhitePoint, lms [3]float32) [3]float32 {
	return [3]float32{lms[0] / sourceWhitePoint[0], lms[1] / sourceWhitePoint[1], lms[2] / sourceWhitePoint[2]}
}

// toD65 is CalRgb::to_d65 in hayro-interpret src/color/cal_rgb.rs.
func toD65(sourceWhitePoint, lms [3]float32) [3]float32 {
	return [3]float32{
		lms[0] * d65Whitepoint[0] / sourceWhitePoint[0],
		lms[1] * d65Whitepoint[1] / sourceWhitePoint[1],
		lms[2] * d65Whitepoint[2] / sourceWhitePoint[2],
	}
}

// decodeL is CalRgb::decode_l in hayro-interpret src/color/cal_rgb.rs.
func decodeL(l float32) float32 {
	switch {
	case l < 0:
		return -decodeL(-l)
	case l > 8:
		v := (l + 16) / 116
		return v * v * v
	}
	return l * decodeLConstant()
}

// compensateBlackPoint is CalRgb::compensate_black_point in
// hayro-interpret src/color/cal_rgb.rs.
func compensateBlackPoint(sourceBP, xyzFlat [3]float32) [3]float32 {
	if sourceBP == [3]float32{} {
		return xyzFlat
	}
	zeroDecodeL := decodeL(0)
	var out [3]float32
	for i := range out {
		src := decodeL(sourceBP[i])
		scale := (1 - zeroDecodeL) / (1 - src)
		offset := 1 - scale
		out[i] = xyzFlat[i]*scale + offset
	}
	return out
}

// normalizeWhitePointToFlat is CalRgb::normalize_white_point_to_flat in
// hayro-interpret src/color/cal_rgb.rs.
func normalizeWhitePointToFlat(sourceWhitePoint, xyz [3]float32) [3]float32 {
	if sourceWhitePoint[0] == 1 && sourceWhitePoint[2] == 1 {
		return xyz
	}
	lms := matrixProduct(&bradfordScaleMatrix, xyz)
	lmsFlat := toFlat(sourceWhitePoint, lms)
	return matrixProduct(&bradfordScaleInverseMatrix, lmsFlat)
}

// normalizeWhitePointToD65 is CalRgb::normalize_white_point_to_d65 in
// hayro-interpret src/color/cal_rgb.rs.
func normalizeWhitePointToD65(sourceWhitePoint, xyz [3]float32) [3]float32 {
	lms := matrixProduct(&bradfordScaleMatrix, xyz)
	lmsD65 := toD65(sourceWhitePoint, lms)
	return matrixProduct(&bradfordScaleInverseMatrix, lmsD65)
}

// convertPixel is CalRgb::convert_pixel in hayro-interpret
// src/color/cal_rgb.rs.
func (c *calRGB) convertPixel(r8, g8, b8 uint8) (uint8, uint8, uint8) {
	r, g, b := float32(r8)/255, float32(g8)/255, float32(b8)/255
	gammaOf := func(v, gamma float32) float32 {
		if v == 1 {
			return 1
		}
		return powf32(v, gamma)
	}
	agr, bgg, cgb := gammaOf(r, c.gamma[0]), gammaOf(g, c.gamma[1]), gammaOf(b, c.gamma[2])
	m := &c.matrix
	x := m[0]*agr + m[3]*bgg + m[6]*cgb
	y := m[1]*agr + m[4]*bgg + m[7]*cgb
	z := m[2]*agr + m[5]*bgg + m[8]*cgb
	xyzFlat := normalizeWhitePointToFlat(c.whitePoint, [3]float32{x, y, z})
	xyzBlack := compensateBlackPoint(c.blackPoint, xyzFlat)
	xyzD65 := normalizeWhitePointToD65(flatWhitepoint, xyzBlack)
	srgbXYZ := matrixProduct(&srgbD65XYZToRGBMatrix, xyzD65)
	return asU8(srgbTransferFunction(srgbXYZ[0])*255 + 0.5),
		asU8(srgbTransferFunction(srgbXYZ[1])*255 + 0.5),
		asU8(srgbTransferFunction(srgbXYZ[2])*255 + 0.5)
}

// convert is the ToRgb implementation of CalRgb in hayro-interpret
// src/color/cal_rgb.rs.
func (c *calRGB) convert(input, output []uint8) bool {
	n := min(len(input)/3, len(output)/3)
	for i := range n {
		j := i * 3
		output[j], output[j+1], output[j+2] = c.convertPixel(input[j], input[j+1], input[j+2])
	}
	return true
}

// convertInPlace is the ToRgb implementation of CalRgb in hayro-interpret
// src/color/cal_rgb.rs.
func (c *calRGB) convertInPlace(input []uint8) bool {
	for j := 0; j+3 <= len(input); j += 3 {
		input[j], input[j+1], input[j+2] = c.convertPixel(input[j], input[j+1], input[j+2])
	}
	return true
}

// isNone is the default of ToRgb::is_none.
func (c *calRGB) isNone() bool { return false }
