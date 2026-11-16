// Copyright (c) The Hayro Authors.
//
// Use of this source code is governed by the Apache-2.0 or MIT license that
// can be found in the LICENSE-APACHE and LICENSE-MIT files.

// This file ports hayro-interpret src/color/lab.rs.

package interpret

import (
	_ "embed"

	"github.com/xo/ivy/internal/moxcms"
	"github.com/xo/ivy/pdf/internal/syntax"
)

// labProfileData is the file assets/LAB.icc of hayro-interpret, under CC0
// 1.0 Universal.
//
//go:embed assets/LAB.icc
var labProfileData []byte

// labSpace is Lab in hayro-interpret src/color/lab.rs.
type labSpace struct {
	rng     [4]float32
	profile *iccProfile
}

// newLab is Lab::new in hayro-interpret src/color/lab.rs.
func newLab(dict syntax.Dict) (*labSpace, bool) {
	whitePoint, ok := dict.Get[[3]float32](syntax.KeyWhitePoint)
	if !ok {
		whitePoint = [3]float32{1, 1, 1}
	}
	rng, ok := dict.Get[[4]float32](syntax.KeyRange)
	if !ok {
		rng = [4]float32{-100, 100, -100, 100}
	}
	profile, err := moxcms.NewFromSlice(labProfileData)
	if err != nil {
		return nil, false
	}
	profile.WhitePoint = moxcms.NewXyzd(float64(whitePoint[0]), float64(whitePoint[1]), float64(whitePoint[2]))
	icc, ok := newICCProfileFromSrcProfile(profile, false, 3)
	if !ok {
		return nil, false
	}
	return &labSpace{rng: rng, profile: icc}, true
}

// convert is the ToRgb implementation of Lab in hayro-interpret
// src/color/lab.rs.
func (l *labSpace) convert(input, output []uint8) bool {
	return l.profile.convert(input, output)
}

// convertInPlace is the ToRgb implementation of Lab in hayro-interpret
// src/color/lab.rs.
func (l *labSpace) convertInPlace(input []uint8) bool {
	return l.profile.convertInPlace(input)
}

// isNone is the default of ToRgb::is_none.
func (l *labSpace) isNone() bool { return false }
