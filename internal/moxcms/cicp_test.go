// Copyright (c) Radzivon Bartoshyk. All rights reserved.
//
// Use of this source code is governed by the BSD-3-Clause or Apache-2.0
// license that can be found in the LICENSE.md and LICENSE-APACHE.md files.

// This file has no source in moxcms. It tests NewFromCicp and the items of
// src/cicp.rs that it uses, which the package icc calls (D61).

package moxcms

import (
	"math"
	"testing"
)

// TestNewFromCicpSrgb makes the profile of the code points of sRGB. Its
// colorants are those of NewSrgb, and it converts each gray to itself.
func TestNewFromCicpSrgb(t *testing.T) {
	t.Parallel()
	p := NewFromCicp(CicpProfile{
		ColorPrimaries:          CicpColorPrimariesBt709,
		TransferCharacteristics: TransferCharacteristicsSrgb,
		MatrixCoefficients:      MatrixCoefficientsIdentity,
		FullRange:               true,
	})
	if !p.IsMatrixShaper() || p.Cicp == nil {
		t.Fatal("the profile is not a matrix shaper with a cicp tag")
	}
	srgb := NewSrgb()
	for i, pair := range [][2]Xyzd{
		{p.RedColorant, srgb.RedColorant},
		{p.GreenColorant, srgb.GreenColorant},
		{p.BlueColorant, srgb.BlueColorant},
	} {
		got, want := pair[0], pair[1]
		if math.Abs(got.X-want.X) > 1e-3 || math.Abs(got.Y-want.Y) > 1e-3 || math.Abs(got.Z-want.Z) > 1e-3 {
			t.Errorf("colorant %d is %v, want %v", i, got, want)
		}
	}
	tr, err := p.CreateTransform8Bit(LayoutRgb, srgb, LayoutRgb, DefaultTransformOptions())
	if err != nil {
		t.Fatal(err)
	}
	src := make([]uint8, 3*256)
	for i := range src {
		src[i] = uint8(i / 3)
	}
	dst := make([]uint8, len(src))
	if err := tr.Transform(src, dst); err != nil {
		t.Fatal(err)
	}
	for i := range src {
		if d := int(dst[i]) - int(src[i]); d < -1 || d > 1 {
			t.Fatalf("sample %d gives %d, want %d", i, dst[i], src[i])
		}
	}
}

// TestNewFromCicpEachCodePoint makes the profile of each pair of primaries
// and transfer characteristics. A pair with a reserved or an unspecified
// code point gives a profile without colorants.
func TestNewFromCicpEachCodePoint(t *testing.T) {
	t.Parallel()
	srgb := NewSrgb()
	for cp := range 24 {
		for tc := range 20 {
			c := CicpProfile{
				ColorPrimaries:          CicpColorPrimariesFrom(uint8(cp)),
				TransferCharacteristics: TransferCharacteristicsFrom(uint8(tc)),
			}
			p := NewFromCicp(c)
			want := c.ColorPrimaries.hasChromaticity() && c.TransferCharacteristics.hasTransferCurve()
			if p.IsMatrixShaper() != want {
				t.Errorf("%d, %d: matrix shaper %v, want %v", cp, tc, p.IsMatrixShaper(), want)
				continue
			}
			if !want {
				continue
			}
			tr, err := p.CreateTransform8Bit(LayoutRgb, srgb, LayoutRgb, DefaultTransformOptions())
			if err != nil {
				t.Errorf("%d, %d: %v", cp, tc, err)
				continue
			}
			if err := tr.Transform([]uint8{0, 128, 255}, make([]uint8, 3)); err != nil {
				t.Errorf("%d, %d: %v", cp, tc, err)
			}
		}
	}
}

// TestBuildTrcTable samples a transfer function from 0 to 1.
func TestBuildTrcTable(t *testing.T) {
	t.Parallel()
	table := buildTrcTable(numTrcTableEntries, pqToLinear)
	if len(table) != numTrcTableEntries || table[0] != 0 || table[len(table)-1] != 65535 {
		t.Errorf("the table of PQ has %d entries, from %d to %d", len(table), table[0], table[len(table)-1])
	}
	nan := buildTrcTable(4, func(float64) float64 { return math.NaN() })
	for i, v := range nan {
		if v != 0 {
			t.Errorf("a NaN gives %d at %d, want 0", v, i)
		}
	}
}
