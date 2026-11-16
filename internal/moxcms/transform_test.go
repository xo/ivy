// Copyright (c) Radzivon Bartoshyk. All rights reserved.
//
// Use of this source code is governed by the BSD-3-Clause or Apache-2.0
// license that can be found in the LICENSE.md and LICENSE-APACHE.md files.

// This file ports the tests of moxcms src/transform.rs for the transforms
// of 8 bits.

package moxcms

import (
	"bytes"
	"encoding/binary"
	"testing"
)

// Port: upstream fills the buffers of several tests with a random value.
// The port takes a few fixed values instead.
var testPoints = []uint8{0, 1, 77, 128, 254}

// mustTransform builds the transform, and fails the test on an error.
func mustTransform(t *testing.T, src *ColorProfile, sl Layout, dst *ColorProfile, dl Layout, options TransformOptions) TransformExecutor {
	t.Helper()
	tr, err := src.CreateTransform8Bit(sl, dst, dl, options)
	if err != nil {
		t.Fatal(err)
	}
	return tr
}

// runFilled transforms a buffer of 256*256 pixels of the value x.
func runFilled(t *testing.T, tr TransformExecutor, sl, dl Layout, x uint8) {
	t.Helper()
	src := bytes.Repeat([]uint8{x}, 256*256*sl.Channels())
	dst := bytes.Repeat([]uint8{x}, 256*256*dl.Channels())
	if err := tr.Transform(src, dst); err != nil {
		t.Fatal(err)
	}
}

func TestTransformRgb8(t *testing.T) {
	srgb := NewSrgb()
	bt2020 := newBt2020()
	for _, intent := range []RenderingIntent{RenderingIntentPerceptual, RenderingIntentRelativeColorimetric, RenderingIntentSaturation} {
		srgb.RenderingIntent = intent
		tr := mustTransform(t, bt2020, LayoutRgb, srgb, LayoutRgb, DefaultTransformOptions())
		for _, x := range testPoints {
			runFilled(t, tr, LayoutRgb, LayoutRgb, x)
		}
	}
}

func TestTransformRgba8(t *testing.T) {
	tr := mustTransform(t, newBt2020(), LayoutRgba, NewSrgb(), LayoutRgba, DefaultTransformOptions())
	for _, x := range testPoints {
		runFilled(t, tr, LayoutRgba, LayoutRgba, x)
	}
}

func TestTransformGrayToRgb8(t *testing.T) {
	gray := NewGrayWithGamma(2.2)
	bt2020 := newBt2020()
	for _, dl := range []Layout{LayoutRgb, LayoutRgba, LayoutGrayAlpha} {
		tr := mustTransform(t, gray, LayoutGray, bt2020, dl, DefaultTransformOptions())
		for _, x := range testPoints {
			runFilled(t, tr, LayoutGray, dl, x)
		}
	}
}

func TestTransformRoundTripRgb8(t *testing.T) {
	srgb := NewSrgb()
	bt2020 := newBt2020()
	tr := mustTransform(t, srgb, LayoutRgb, bt2020, LayoutRgb, DefaultTransformOptions())
	src := bytes.Repeat([]uint8{175, 75, 13}, 256*256)
	dst := make([]uint8, len(src))
	if err := tr.Transform(src, dst); err != nil {
		t.Fatal(err)
	}
	inverse := mustTransform(t, bt2020, LayoutRgb, srgb, LayoutRgb, DefaultTransformOptions())
	if err := inverse.Transform(dst, src); err != nil {
		t.Fatal(err)
	}
	want := [3]int{175, 75, 13}
	for i, v := range src {
		if d := int(v) - want[i%3]; d >= 3 || d <= -3 {
			t.Fatalf("channel %d differs by %d", i%3, d)
		}
	}
}

func TestTransformRgb8PixelIndependence(t *testing.T) {
	// A transform of several pixels must give each pixel the result of a
	// transform of that pixel alone.
	options := DefaultTransformOptions()
	options.PreferFixedPoint = true
	tr := mustTransform(t, newBt2020(), LayoutRgb, NewSrgb(), LayoutRgb, options)
	src := []uint8{
		255, 0, 0, // red
		0, 255, 0, // green
		0, 0, 255, // blue
		255, 255, 0, // yellow
	}
	dst := make([]uint8, 12)
	if err := tr.Transform(src, dst); err != nil {
		t.Fatal(err)
	}
	var single []uint8
	for i := 0; i < len(src); i += 3 {
		out := make([]uint8, 3)
		if err := tr.Transform(src[i:i+3], out); err != nil {
			t.Fatal(err)
		}
		single = append(single, out...)
	}
	if !bytes.Equal(dst, single) {
		t.Errorf("batch %v, single %v", dst, single)
	}
}

func TestLut16LabTablesUseTheV2EncodingInV4ProfilesToo(t *testing.T) {
	// Port: upstream uses us_swop_coated.icc. The port uses the CMYK profile
	// of hayro, which also has a lut16Type A2B0 to Lab.
	v2 := readProfile(t, "CGATS001Compat-v2-micro.icc")
	v4 := readProfile(t, "CGATS001Compat-v2-micro.icc")
	v4.versionInternal = ProfileVersionV43
	srgb := NewSrgb()
	src := make([]uint8, 4096*4)
	for i := range uint32(4096) {
		binary.LittleEndian.PutUint32(src[4*i:], i*2654435761)
	}
	for _, intent := range []RenderingIntent{RenderingIntentPerceptual, RenderingIntentRelativeColorimetric} {
		options := DefaultTransformOptions()
		options.RenderingIntent = intent
		want := make([]uint8, len(src)/4*3)
		got := make([]uint8, len(src)/4*3)
		if err := mustTransform(t, v2, LayoutRgba, srgb, LayoutRgb, options).Transform(src, want); err != nil {
			t.Fatal(err)
		}
		if err := mustTransform(t, v4, LayoutRgba, srgb, LayoutRgb, options).Transform(src, got); err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(want, got) {
			t.Errorf("intent %d: the outputs differ", intent)
		}
	}
}

func TestCmykToALabPcsDestinationOfFourOutputsIsRefused(t *testing.T) {
	cmyk := readProfile(t, "CGATS001Compat-v2-micro.icc")
	// The B2A tables of the profile are Lab to four inks, which no
	// destination of three channels can use.
	dest := readProfile(t, "CGATS001Compat-v2-micro.icc")
	dest.ColorSpace = DataColorSpaceRgb
	_, err := cmyk.CreateTransform8Bit(LayoutRgba, dest, LayoutRgb, DefaultTransformOptions())
	wantKind(t, err, UnsupportedProfileConnection)
}

func TestTransformsBuild(t *testing.T) {
	// Port: upstream builds these transforms on a small thread stack. Go
	// grows a stack as it needs, so the port only builds them.
	srgb := NewSrgb()
	bt2020 := newBt2020()
	gray := NewGrayWithGamma(2.2)
	cmyk := readProfile(t, "CGATS001Compat-v2-micro.icc")
	options := DefaultTransformOptions()
	if _, err := bt2020.CreateTransform8Bit(LayoutRgba, srgb, LayoutRgba, options); err != nil {
		t.Error(err)
	}
	if _, err := gray.CreateTransform8Bit(LayoutGray, srgb, LayoutRgba, options); err != nil {
		t.Error(err)
	}
	if _, err := cmyk.CreateTransform8Bit(LayoutRgba, srgb, LayoutRgba, options); err != nil {
		t.Error(err)
	}
}

func TestTransformErrors(t *testing.T) {
	srgb := NewSrgb()
	tr := mustTransform(t, newBt2020(), LayoutRgb, srgb, LayoutRgba, DefaultTransformOptions())
	wantKind(t, tr.Transform(make([]uint8, 6), make([]uint8, 4)), LaneSizeMismatch)
	wantKind(t, tr.Transform(make([]uint8, 7), make([]uint8, 8)), LaneMultipleOfChannels)

	_, err := newBt2020().CreateTransform8Bit(LayoutGray, srgb, LayoutRgb, DefaultTransformOptions())
	wantKind(t, err, InvalidLayout)

	inPlace, err := newBt2020().CreateInPlaceTransform8Bit(LayoutRgb, srgb, DefaultTransformOptions())
	if err != nil {
		t.Fatal(err)
	}
	wantKind(t, inPlace.Transform(make([]uint8, 4)), LaneMultipleOfChannels)

	lab := readProfile(t, "LAB.icc")
	_, err = lab.CreateInPlaceTransform8Bit(LayoutRgb, srgb, DefaultTransformOptions())
	wantKind(t, err, UnsupportedProfileConnection)
}
