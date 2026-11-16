// Copyright (c) The Hayro Authors
//
// Use of this source code is governed by the Apache-2.0 or MIT license that
// can be found in the LICENSE-APACHE and LICENSE-MIT files.

package jpeg2000

// This file ports hayro-jpeg2000 src/jp2/icc.rs.

import "encoding/binary"

// iccColorSpace is ICCColorSpace in hayro-jpeg2000 src/jp2/icc.rs: the
// color space of the data of an ICC profile.
type iccColorSpace uint8

// The color spaces of an ICC profile.
const (
	iccXYZ iccColorSpace = iota
	iccLab
	iccLuv
	iccYcbr
	iccYxy
	iccLms
	iccRGB
	iccGray
	iccHsv
	iccHls
	iccCmyk
	iccCmy
	iccOneClr
	iccThreeClr
	iccFourClr
)

// numComponents is ICCColorSpace::num_components.
func (cs iccColorSpace) numComponents() uint8 {
	switch cs {
	case iccGray, iccOneClr:
		return 1
	case iccCmyk, iccFourClr:
		return 4
	}
	return 3
}

// iccColorSpaceFromU32 is TryFrom<u32> for ICCColorSpace.
func iccColorSpaceFromU32(value uint32) (iccColorSpace, bool) {
	switch value {
	case 0x58595A20:
		return iccXYZ, true
	case 0x4C616220:
		return iccLab, true
	case 0x4C757620:
		return iccLuv, true
	case 0x59436272:
		return iccYcbr, true
	case 0x59787920:
		return iccYxy, true
	case 0x4C4D5320:
		return iccLms, true
	case 0x52474220:
		return iccRGB, true
	case 0x47524159:
		return iccGray, true
	case 0x48535620:
		return iccHsv, true
	case 0x484C5320:
		return iccHls, true
	case 0x434D594B:
		return iccCmyk, true
	case 0x434D5920:
		return iccCmy, true
	case 0x31434C52:
		return iccOneClr, true
	case 0x33434C52:
		return iccThreeClr, true
	case 0x34434C52:
		return iccFourClr, true
	}
	return 0, false
}

// iccMetadata is ICCMetadata in hayro-jpeg2000 src/jp2/icc.rs.
type iccMetadata struct {
	colorSpace iccColorSpace
}

// iccMetadataFromData is ICCMetadata::from_data. It reads the color space
// of the data from the header of the profile.
func iccMetadataFromData(data []byte) (iccMetadata, bool) {
	if len(data) < 20 {
		return iccMetadata{}, false
	}
	cs, ok := iccColorSpaceFromU32(binary.BigEndian.Uint32(data[16:20]))
	if !ok {
		return iccMetadata{}, false
	}
	return iccMetadata{colorSpace: cs}, true
}
