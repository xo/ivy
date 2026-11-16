// Copyright (c) Radzivon Bartoshyk. All rights reserved.
//
// Use of this source code is governed by the BSD-3-Clause or Apache-2.0
// license that can be found in the LICENSE.md and LICENSE-APACHE.md files.

// This file has no source in moxcms. It compares the transforms of the port
// with the output of moxcms.

package moxcms

import (
	"encoding/hex"
	"os"
	"strings"
	"testing"
)

// The file testdata/golden.txt holds the output of moxcms at the base commit
// of the port, built with the features "lut" and "in_place" only, so that it
// takes the scalar path. Each line holds the name of a case and the bytes of
// its output in hex. The profiles in testdata come from hayro, under CC0 1.0
// (testdata/LICENSE-CC0.txt).

// readGolden reads testdata/golden.txt.
func readGolden(t *testing.T) map[string][]byte {
	t.Helper()
	golden := make(map[string][]byte)
	for line := range strings.Lines(string(readTestdata(t, "golden.txt"))) {
		name, h, ok := strings.Cut(strings.TrimSpace(line), " ")
		if !ok {
			t.Fatalf("bad line %q", line)
		}
		b, err := hex.DecodeString(h)
		if err != nil {
			t.Fatal(err)
		}
		golden[name] = b
	}
	return golden
}

// readProfile reads a profile of testdata.
func readProfile(t *testing.T, name string) *ColorProfile {
	t.Helper()
	p, err := NewFromSlice(readTestdata(t, name))
	if err != nil {
		t.Fatalf("NewFromSlice(%s): %v", name, err)
	}
	return p
}

// goldenRgb returns 512 RGB pixels, eight values in each channel.
func goldenRgb() []uint8 {
	v := make([]uint8, 0, 512*3)
	for r := range uint32(8) {
		for g := range uint32(8) {
			for b := range uint32(8) {
				v = append(v, uint8(r*255/7), uint8(g*255/7), uint8(b*255/7))
			}
		}
	}
	return v
}

// goldenRgba returns the pixels of goldenRgb with an alpha.
func goldenRgba() []uint8 {
	s := goldenRgb()
	v := make([]uint8, 0, 512*4)
	for i := 0; i < len(s); i += 3 {
		v = append(v, s[i], s[i+1], s[i+2], uint8(i/3*7))
	}
	return v
}

// goldenCmyk returns 1296 CMYK pixels, six values in each channel.
func goldenCmyk() []uint8 {
	v := make([]uint8, 0, 1296*4)
	for c := range uint32(6) {
		for m := range uint32(6) {
			for y := range uint32(6) {
				for k := range uint32(6) {
					v = append(v, uint8(c*51), uint8(m*51), uint8(y*51), uint8(k*51))
				}
			}
		}
	}
	return v
}

// goldenGray returns each sample of 8 bits.
func goldenGray() []uint8 {
	v := make([]uint8, 256)
	for i := range v {
		v[i] = uint8(i)
	}
	return v
}

func TestGolden(t *testing.T) {
	golden := readGolden(t)
	srgb := NewSrgb()
	bt2020 := newBt2020()
	def := DefaultTransformOptions()
	float := DefaultTransformOptions()
	float.PreferFixedPoint = false
	rel := DefaultTransformOptions()
	rel.RenderingIntent = RenderingIntentRelativeColorimetric

	check := func(name string, got []uint8) {
		t.Helper()
		want, ok := golden[name]
		if !ok {
			t.Fatalf("%s: no golden output", name)
		}
		if len(got) != len(want) {
			t.Fatalf("%s: %d bytes, want %d", name, len(got), len(want))
		}
		bad := 0
		for i := range got {
			if got[i] != want[i] {
				if bad < 5 {
					t.Errorf("%s: byte %d is %d, want %d", name, i, got[i], want[i])
				}
				bad++
			}
		}
		if bad > 0 {
			t.Errorf("%s: %d of %d bytes differ", name, bad, len(got))
		}
	}
	run := func(name string, src *ColorProfile, sl Layout, dst *ColorProfile, dl Layout, opts TransformOptions, in []uint8) {
		t.Helper()
		tr, err := src.CreateTransform8Bit(sl, dst, dl, opts)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		out := make([]uint8, len(in)/sl.Channels()*dl.Channels())
		if err := tr.Transform(in, out); err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		check(name, out)
	}
	runInPlace := func(name string, src *ColorProfile, l Layout, dst *ColorProfile, opts TransformOptions, in []uint8) {
		t.Helper()
		tr, err := src.CreateInPlaceTransform8Bit(l, dst, opts)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		buf := append([]uint8(nil), in...)
		if err := tr.Transform(buf); err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		check(name, buf)
	}

	run("bt2020_srgb_q", bt2020, LayoutRgb, srgb, LayoutRgb, def, goldenRgb())
	run("bt2020_srgb_f", bt2020, LayoutRgb, srgb, LayoutRgb, float, goldenRgb())
	run("srgb_bt2020_rgba", srgb, LayoutRgba, bt2020, LayoutRgba, def, goldenRgba())
	run("srgb_bt2020_rgba_rgb", srgb, LayoutRgba, bt2020, LayoutRgb, def, goldenRgba())
	runInPlace("bt2020_srgb_inplace", bt2020, LayoutRgb, srgb, def, goldenRgb())

	mixed := NewSrgb()
	mixed.GreenTrc = curveFromGamma(2.2)
	mixed.Cicp = nil
	run("mixed_srgb", mixed, LayoutRgb, srgb, LayoutRgb, def, goldenRgb())
	runInPlace("mixed_srgb_inplace", mixed, LayoutRgba, srgb, def, goldenRgba())

	gray := NewGrayWithGamma(2.2)
	run("gray_srgb_rgb", gray, LayoutGray, srgb, LayoutRgb, def, goldenGray())
	run("gray_srgb_rgba", gray, LayoutGray, srgb, LayoutRgba, def, goldenGray())
	ga := make([]uint8, 0, 512)
	for v := range 256 {
		ga = append(ga, uint8(v), uint8(255-v))
	}
	run("grayalpha_srgb_rgba", gray, LayoutGrayAlpha, srgb, LayoutRgba, def, ga)
	// The luma transform of hayro-interpret src/color/icc.rs.
	luma := NewGrayWithGamma(1.0)
	luma.GrayTrc = srgb.RedTrc
	runInPlace("gray_luma_inplace", gray, LayoutGray, luma, def, goldenGray())

	// The Lab profile of hayro-interpret src/color/lab.rs.
	lab := readProfile(t, "LAB.icc")
	lab.WhitePoint = NewXyzd(0.9505, 1.0, 1.089)
	run("lab_srgb", lab, LayoutRgb, srgb, LayoutRgb, def, goldenRgb())

	cmyk := readProfile(t, "CGATS001Compat-v2-micro.icc")
	run("cmyk_srgb_q", cmyk, LayoutRgba, srgb, LayoutRgb, def, goldenCmyk())
	run("cmyk_srgb_f", cmyk, LayoutRgba, srgb, LayoutRgb, float, goldenCmyk())
	run("cmyk_srgb_rel", cmyk, LayoutRgba, srgb, LayoutRgba, rel, goldenCmyk())

	prophoto := readProfile(t, "ProPhoto-v2-micro.icc")
	run("prophoto_srgb", prophoto, LayoutRgb, srgb, LayoutRgb, def, goldenRgb())
	runInPlace("prophoto_srgb_inplace", prophoto, LayoutRgb, srgb, def, goldenRgb())

	// Upstream cannot transform to the Lab profile, which has no B2A LUT.
	if _, err := srgb.CreateTransform8Bit(LayoutRgb, lab, LayoutRgb, def); err == nil {
		t.Error("sRGB to the Lab profile: no error")
	}
}

// readTestdata reads a file of testdata.
func readTestdata(tb testing.TB, name string) []byte {
	tb.Helper()
	b, err := os.ReadFile("testdata/" + name)
	if err != nil {
		tb.Fatal(err)
	}
	return b
}
