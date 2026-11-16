// Copyright (c) Radzivon Bartoshyk. All rights reserved.
//
// Use of this source code is governed by the BSD-3-Clause or Apache-2.0
// license that can be found in the LICENSE.md and LICENSE-APACHE.md files.

// This file has no source in moxcms. It holds the fuzz test of the reading
// of a profile.

package moxcms

import (
	"errors"
	"testing"

	"github.com/xo/ivy"
)

// FuzzNewFromSlice reads a profile, and when the profile is valid, builds
// the transforms that hayro builds, and runs them on a few pixels. It must
// never panic, and each error must wrap a sentinel error of ivy.
func FuzzNewFromSlice(f *testing.F) {
	for _, name := range []string{"LAB.icc", "CGATS001Compat-v2-micro.icc", "ProPhoto-v2-micro.icc"} {
		f.Add(readTestdata(f, name))
	}
	f.Add(cmykMabProfile(curv(), para(0, 2.0), curv()))
	f.Add(cmykMabProfile(curv(0, 30000, 65535), para(3, 2.4, 0.9, 0.1, 0.07, 0.04), para(1, 1.8, 1, 0.1)))

	srgb := NewSrgb()
	luma := NewGrayWithGamma(1.0)
	luma.GrayTrc = srgb.RedTrc
	check := func(t *testing.T, err error) {
		t.Helper()
		if err != nil && !errors.Is(err, ivy.ErrFormat) && !errors.Is(err, ivy.ErrUnsupported) && !errors.Is(err, ivy.ErrLimit) {
			t.Fatalf("error %v wraps no sentinel of ivy", err)
		}
	}
	f.Fuzz(func(t *testing.T, b []byte) {
		p, err := NewFromSlice(b)
		check(t, err)
		if err != nil {
			return
		}
		src := []uint8{0, 1, 2, 3, 128, 64, 32, 16, 255, 254, 253, 252}
		for _, l := range []Layout{LayoutGray, LayoutRgb, LayoutRgba} {
			tr, err := p.CreateTransform8Bit(l, srgb, LayoutRgb, DefaultTransformOptions())
			check(t, err)
			if err == nil {
				n := len(src) / l.Channels()
				check(t, tr.Transform(src[:n*l.Channels()], make([]uint8, n*3)))
			}
		}
		if ip, err := p.CreateInPlaceTransform8Bit(LayoutRgb, srgb, DefaultTransformOptions()); err == nil {
			check(t, ip.Transform(append([]uint8(nil), src...)))
		} else {
			check(t, err)
		}
		if ip, err := p.CreateInPlaceTransform8Bit(LayoutGray, luma, DefaultTransformOptions()); err == nil {
			check(t, ip.Transform(append([]uint8(nil), src...)))
		} else {
			check(t, err)
		}
	})
}
