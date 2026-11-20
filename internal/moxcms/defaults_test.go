// Copyright (c) Radzivon Bartoshyk. All rights reserved.
//
// Use of this source code is governed by the BSD-3-Clause or Apache-2.0
// license that can be found in the LICENSE.md and LICENSE-APACHE.md files.

// This file ports the part of moxcms src/defaults.rs that the tests of
// moxcms use, and that the subset does not need.

package moxcms

// newBt2020 is ColorProfile::new_bt2020 in moxcms src/defaults.rs, without
// the description and the copyright.
func newBt2020() *ColorProfile {
	p := basicRgbProfile(whitePointD65())
	p.updateColorants(colorantsMatrix(whitePointD65(), ColorPrimariesBt2020))
	rec709 := createRec709Parametric()
	p.RedTrc = &ToneReprCurve{Kind: CurveParametric, Parametric: rec709[:]}
	p.GreenTrc = &ToneReprCurve{Kind: CurveParametric, Parametric: rec709[:]}
	p.BlueTrc = &ToneReprCurve{Kind: CurveParametric, Parametric: rec709[:]}
	return p
}
