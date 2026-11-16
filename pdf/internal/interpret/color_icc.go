// Copyright (c) The Hayro Authors.
//
// Use of this source code is governed by the Apache-2.0 or MIT license that
// can be found in the LICENSE-APACHE and LICENSE-MIT files.

// This file ports hayro-interpret src/color/icc.rs.

package interpret

import (
	"bytes"
	"sync"

	"github.com/xo/ivy/internal/moxcms"
)

// iccProfile is ICCProfile in hayro-interpret src/color/icc.rs, with
// ICCColorRepr of the same file. It converts the colors of an ICC profile
// into sRGB with moxcms.
//
// Port: upstream shares the state through an Arc. Go shares the pointer.
type iccProfile struct {
	nComponents int
	srgb        bool
	lab         bool
	srcProfile  *moxcms.ColorProfile
	srcLayout   moxcms.Layout

	transformU8Once sync.Once
	transformU8     moxcms.TransformExecutor

	transformInPlaceU8     moxcms.InPlaceTransformExecutor
	transformInPlaceLumaU8 moxcms.InPlaceTransformExecutor
}

// newICCProfile is ICCProfile::new in hayro-interpret src/color/icc.rs.
func newICCProfile(profile []byte, numberComponents int) (*iccProfile, bool) {
	src, err := moxcms.NewFromSlice(profile)
	if err != nil {
		return nil, false
	}
	isSRGB := len(profile) >= 56 && bytes.Equal(profile[52:56], []byte("sRGB"))
	return newICCProfileFromSrcProfile(src, isSRGB, numberComponents)
}

// newICCProfileFromSrcProfile is ICCProfile::new_from_src_profile in
// hayro-interpret src/color/icc.rs.
func newICCProfileFromSrcProfile(src *moxcms.ColorProfile, isSRGB bool, numberComponents int) (*iccProfile, bool) {
	isLab := src.ColorSpace == moxcms.DataColorSpaceLab
	var srcLayout moxcms.Layout
	switch numberComponents {
	case 1:
		srcLayout = moxcms.LayoutGray
	case 3:
		srcLayout = moxcms.LayoutRgb
	case 4:
		srcLayout = moxcms.LayoutRgba
	default:
		return nil, false
	}
	p := &iccProfile{
		nComponents: numberComponents,
		srgb:        isSRGB,
		lab:         isLab,
		srcLayout:   srcLayout,
	}
	dest := moxcms.NewSrgb()
	if srcLayout == moxcms.LayoutGray {
		destLuma := moxcms.NewGrayWithGamma(1.0)
		destLuma.GrayTrc = dest.RedTrc
		if t, err := src.CreateInPlaceTransform8Bit(srcLayout, destLuma, moxcms.DefaultTransformOptions()); err == nil {
			p.transformInPlaceLumaU8 = t
		}
	}
	if srcLayout == moxcms.LayoutRgb {
		if t, err := src.CreateInPlaceTransform8Bit(srcLayout, dest, moxcms.DefaultTransformOptions()); err == nil {
			p.transformInPlaceU8 = t
		}
	}
	if p.transformInPlaceU8 != nil {
		p.srcProfile = src
	} else {
		t, err := src.CreateTransform8Bit(srcLayout, dest, moxcms.LayoutRgb, moxcms.DefaultTransformOptions())
		if err != nil {
			return nil, false
		}
		p.transformU8 = t
		p.transformU8Once.Do(func() {})
	}
	return p, true
}

// numberComponents is ICCProfile::number_components in hayro-interpret
// src/color/icc.rs.
func (p *iccProfile) numberComponents() int {
	return p.nComponents
}

// isSRGB is ICCProfile::is_srgb in hayro-interpret src/color/icc.rs.
func (p *iccProfile) isSRGB() bool {
	return p.srgb
}

// isLab is ICCProfile::is_lab in hayro-interpret src/color/icc.rs.
func (p *iccProfile) isLab() bool {
	return p.lab
}

// getTransformU8 is ICCProfile::transform_u8 in hayro-interpret
// src/color/icc.rs. It makes the transform on its first use when the
// profile has an in-place transform.
func (p *iccProfile) getTransformU8() moxcms.TransformExecutor {
	p.transformU8Once.Do(func() {
		if p.srcProfile == nil {
			return
		}
		t, err := p.srcProfile.CreateTransform8Bit(p.srcLayout, moxcms.NewSrgb(), moxcms.LayoutRgb, moxcms.DefaultTransformOptions())
		if err == nil {
			p.transformU8 = t
		}
	})
	return p.transformU8
}

// convert is the ToRgb implementation of ICCProfile in hayro-interpret
// src/color/icc.rs.
func (p *iccProfile) convert(input, output []uint8) bool {
	t := p.getTransformU8()
	if t == nil {
		return false
	}
	return t.Transform(input, output) == nil
}

// convertInPlace is the ToRgb implementation of ICCProfile in
// hayro-interpret src/color/icc.rs.
func (p *iccProfile) convertInPlace(input []uint8) bool {
	if !p.isSRGB() {
		if p.transformInPlaceU8 == nil {
			return false
		}
		if p.transformInPlaceU8.Transform(input) != nil {
			return false
		}
	}
	return true
}

// isNone is the default of ToRgb::is_none.
func (p *iccProfile) isNone() bool { return false }

// toLuma is the ToLuma implementation of ICCProfile in hayro-interpret
// src/color/icc.rs.
func (p *iccProfile) toLuma(input []uint8) bool {
	if p.numberComponents() != 1 || p.transformInPlaceLumaU8 == nil {
		return false
	}
	return p.transformInPlaceLumaU8.Transform(input) == nil
}
