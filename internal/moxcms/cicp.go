// Copyright (c) Radzivon Bartoshyk. All rights reserved.
//
// Use of this source code is governed by the BSD-3-Clause or Apache-2.0
// license that can be found in the LICENSE.md and LICENSE-APACHE.md files.

// This file ports moxcms src/cicp.rs.

package moxcms

// CicpColorPrimaries is CicpColorPrimaries in moxcms src/cicp.rs. It is the
// code point of the color primaries in Rec. ITU-T H.273 table 2.
type CicpColorPrimaries uint8

// The color primaries of CicpColorPrimaries in moxcms src/cicp.rs. The
// values 0, 3, 13 to 21, and 23 to 255 are reserved, and they all map to
// CicpColorPrimariesReserved.
const (
	CicpColorPrimariesReserved    CicpColorPrimaries = 0
	CicpColorPrimariesBt709       CicpColorPrimaries = 1
	CicpColorPrimariesUnspecified CicpColorPrimaries = 2
	CicpColorPrimariesBt470M      CicpColorPrimaries = 4
	CicpColorPrimariesBt470Bg     CicpColorPrimaries = 5
	CicpColorPrimariesBt601       CicpColorPrimaries = 6
	CicpColorPrimariesSmpte240    CicpColorPrimaries = 7
	CicpColorPrimariesGenericFilm CicpColorPrimaries = 8
	CicpColorPrimariesBt2020      CicpColorPrimaries = 9
	CicpColorPrimariesXyz         CicpColorPrimaries = 10
	CicpColorPrimariesSmpte431    CicpColorPrimaries = 11
	CicpColorPrimariesSmpte432    CicpColorPrimaries = 12
	CicpColorPrimariesEbu3213     CicpColorPrimaries = 22
)

// cicpColorPrimariesFrom is impl TryFrom<u8> for CicpColorPrimaries in
// moxcms src/cicp.rs. Each value maps to a variant, so it never fails.
func cicpColorPrimariesFrom(v uint8) CicpColorPrimaries {
	switch {
	case v == 0 || v == 3 || (v >= 13 && v <= 21) || v >= 23:
		return CicpColorPrimariesReserved
	default:
		return CicpColorPrimaries(v)
	}
}

// ColorPrimaries is ColorPrimaries in moxcms src/cicp.rs. It holds the
// chromaticities of the red, green and blue primaries.
type ColorPrimaries struct {
	Red, Green, Blue Chromaticity
}

// ColorPrimariesBt709 is ColorPrimaries::BT_709 in moxcms src/cicp.rs.
var ColorPrimariesBt709 = ColorPrimaries{
	Red:   Chromaticity{X: 0.64, Y: 0.33},
	Green: Chromaticity{X: 0.30, Y: 0.60},
	Blue:  Chromaticity{X: 0.15, Y: 0.06},
}

// TransferCharacteristics is TransferCharacteristics in moxcms src/cicp.rs.
// It is the code point of the transfer characteristics in Rec. ITU-T H.273
// table 3.
type TransferCharacteristics uint8

// The transfer characteristics of TransferCharacteristics in moxcms
// src/cicp.rs. The values 0, 3, and 19 to 255 are reserved, and they all
// map to TransferCharacteristicsReserved.
const (
	TransferCharacteristicsReserved     TransferCharacteristics = 0
	TransferCharacteristicsBt709        TransferCharacteristics = 1
	TransferCharacteristicsUnspecified  TransferCharacteristics = 2
	TransferCharacteristicsBt470M       TransferCharacteristics = 4
	TransferCharacteristicsBt470Bg      TransferCharacteristics = 5
	TransferCharacteristicsBt601        TransferCharacteristics = 6
	TransferCharacteristicsSmpte240     TransferCharacteristics = 7
	TransferCharacteristicsLinear       TransferCharacteristics = 8
	TransferCharacteristicsLog100       TransferCharacteristics = 9
	TransferCharacteristicsLog100sqrt10 TransferCharacteristics = 10
	TransferCharacteristicsIec61966     TransferCharacteristics = 11
	TransferCharacteristicsBt1361       TransferCharacteristics = 12
	TransferCharacteristicsSrgb         TransferCharacteristics = 13
	TransferCharacteristicsBt202010bit  TransferCharacteristics = 14
	TransferCharacteristicsBt202012bit  TransferCharacteristics = 15
	TransferCharacteristicsSmpte2084    TransferCharacteristics = 16
	TransferCharacteristicsSmpte428     TransferCharacteristics = 17
	TransferCharacteristicsHlg          TransferCharacteristics = 18
)

// transferCharacteristicsFrom is impl TryFrom<u8> for
// TransferCharacteristics in moxcms src/cicp.rs. Each value maps to a
// variant, so it never fails.
func transferCharacteristicsFrom(v uint8) TransferCharacteristics {
	if v == 0 || v == 3 || v >= 19 {
		return TransferCharacteristicsReserved
	}
	return TransferCharacteristics(v)
}

// hasTransferCurve is TransferCharacteristics::has_transfer_curve in moxcms
// src/cicp.rs.
func (t TransferCharacteristics) hasTransferCurve() bool {
	return t != TransferCharacteristicsReserved && t != TransferCharacteristicsUnspecified
}

// createRec709Parametric is create_rec709_parametric in moxcms src/cicp.rs.
// It returns the parameters g, a, b, c and d of the parametric curve of
// Rec. 709.
func createRec709Parametric() [5]float32 {
	// Port: upstream computes b, d and a in f64 and rounds them to f32. The
	// variables keep Go from an exact computation with constants.
	alpha, alpha1, beta := 1.09929682680944, 0.09929682680944, 0.018053968510807
	powExp := float32(0.45)
	g := 1 / powExp
	b := float32(alpha1 / alpha)
	c := float32(1) / float32(4.5)
	d := float32(4.5 * beta)
	a := float32(1 / alpha)
	return [5]float32{g, a, b, c, d}
}

// MatrixCoefficients is MatrixCoefficients in moxcms src/cicp.rs. It is the
// code point of the matrix coefficients in ISO/IEC 23091-4.
type MatrixCoefficients uint8

// The matrix coefficients of MatrixCoefficients in moxcms src/cicp.rs.
const (
	MatrixCoefficientsIdentity MatrixCoefficients = iota
	MatrixCoefficientsBt709
	MatrixCoefficientsUnspecified
	MatrixCoefficientsReserved
	MatrixCoefficientsFcc
	MatrixCoefficientsBt470Bg
	MatrixCoefficientsSmpte170m
	MatrixCoefficientsSmpte240m
	MatrixCoefficientsYCgCo
	MatrixCoefficientsBt2020Ncl
	MatrixCoefficientsBt2020Cl
	MatrixCoefficientsSmpte2085
	MatrixCoefficientsChromaticityDerivedNCL
	MatrixCoefficientsChromaticityDerivedCL
	MatrixCoefficientsICtCp
	MatrixCoefficientsIPtC2
	MatrixCoefficientsYCgCoRe
	MatrixCoefficientsYCgCoRo
)

// matrixCoefficientsFrom is impl TryFrom<u8> for MatrixCoefficients in
// moxcms src/cicp.rs. It fails for a value larger than 17.
func matrixCoefficientsFrom(v uint8) (MatrixCoefficients, error) {
	if v > uint8(MatrixCoefficientsYCgCoRo) {
		return 0, cmsError(InvalidCicp)
	}
	return MatrixCoefficients(v), nil
}
