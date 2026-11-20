package icc

import (
	"fmt"

	"github.com/xo/ivy"
	"github.com/xo/ivy/internal/moxcms"
)

// CICP holds the code points of Rec. ITU-T H.273 that describe the colors of
// an image without an ICC profile. An AVIF or a HEIC file holds them in a
// colr box of the type nclx, and a PNG file in a cICP chunk.
type CICP struct {
	// ColorPrimaries is the code point of the primaries and of the white,
	// such as 1 for BT.709 and sRGB, 9 for BT.2020, or 12 for Display P3.
	ColorPrimaries uint8
	// TransferCharacteristics is the code point of the transfer function,
	// such as 13 for sRGB, 16 for PQ (SMPTE ST 2084), or 18 for HLG.
	TransferCharacteristics uint8
	// MatrixCoefficients is the code point of the matrix from RGB to YCbCr.
	MatrixCoefficients uint8
	// FullRange is true when the samples use the full range of their bits.
	FullRange bool
}

// FromCICP returns the RGB profile of the primaries and of the transfer
// function of c, as moxcms makes it. The profile converts the red, the
// green and the blue of an image. The codec must convert YCbCr to RGB with
// MatrixCoefficients and FullRange first, so the transform does not use
// them. FromCICP only makes sure that MatrixCoefficients is a code point of
// H.273.
//
// The transform does no tone mapping. A linear value of 1 is the white of
// sRGB. In PQ a linear value of 1 is 10000 cd/m2, so an image in PQ looks
// dark. A color that sRGB cannot show clips.
//
// A code point that is reserved, or that is unspecified (2), gives an error
// that wraps ivy.ErrUnsupported. If a format reads an unspecified code
// point as sRGB, the program gives the primaries 1 and the transfer
// characteristics 13.
func FromCICP(c CICP) (*Profile, error) {
	cp := moxcms.CicpColorPrimariesFrom(c.ColorPrimaries)
	if cp == moxcms.CicpColorPrimariesReserved || cp == moxcms.CicpColorPrimariesUnspecified {
		return nil, fmt.Errorf("icc: making a profile of the color primaries %d: %w", c.ColorPrimaries, ivy.ErrUnsupported)
	}
	tc := moxcms.TransferCharacteristicsFrom(c.TransferCharacteristics)
	if tc == moxcms.TransferCharacteristicsReserved || tc == moxcms.TransferCharacteristicsUnspecified {
		return nil, fmt.Errorf("icc: making a profile of the transfer characteristics %d: %w", c.TransferCharacteristics, ivy.ErrUnsupported)
	}
	mc, err := moxcms.MatrixCoefficientsFrom(c.MatrixCoefficients)
	if err != nil {
		return nil, fmt.Errorf("icc: making a profile of the matrix coefficients %d: %w", c.MatrixCoefficients, err)
	}
	p := moxcms.NewFromCicp(moxcms.CicpProfile{
		ColorPrimaries:          cp,
		TransferCharacteristics: tc,
		MatrixCoefficients:      mc,
		FullRange:               c.FullRange,
	})
	if !p.IsMatrixShaper() {
		return nil, fmt.Errorf("icc: making a profile of the code points %d, %d and %d: %w",
			c.ColorPrimaries, c.TransferCharacteristics, c.MatrixCoefficients, ivy.ErrUnsupported)
	}
	return newProfile(p), nil
}
