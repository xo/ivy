// Copyright (c) Radzivon Bartoshyk. All rights reserved.
//
// Use of this source code is governed by the BSD-3-Clause or Apache-2.0
// license that can be found in the LICENSE.md and LICENSE-APACHE.md files.

// This file ports the tests of moxcms src/conversions/gray2rgb.rs.

package moxcms

import "testing"

func TestGrayRgbRoundtrip(t *testing.T) {
	gray := NewGrayWithGamma(2.2)
	srgb := NewSrgb()
	for _, src := range []Layout{LayoutGray, LayoutGrayAlpha} {
		for _, dst := range []Layout{LayoutRgb, LayoutGrayAlpha, LayoutRgba, LayoutGray} {
			tr := mustTransform(t, gray, src, srgb, dst, DefaultTransformOptions())
			in := make([]uint8, src.Channels())
			out := make([]uint8, dst.Channels())
			if err := tr.Transform(in, out); err != nil {
				t.Fatal(err)
			}
		}
	}
}
