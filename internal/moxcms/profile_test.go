// Copyright (c) Radzivon Bartoshyk. All rights reserved.
//
// Use of this source code is governed by the BSD-3-Clause or Apache-2.0
// license that can be found in the LICENSE.md and LICENSE-APACHE.md files.

// This file ports the tests of moxcms src/profile.rs.

package moxcms

import (
	"encoding/binary"
	"errors"
	"testing"

	"github.com/xo/ivy"
)

// testTag is a tag for buildProfile.
type testTag struct {
	sig  string
	data []byte
}

// buildProfile returns an ICC profile with a header of the version, the
// class, the color space and the PCS, and with the tags. It aligns each tag
// to 4 bytes.
//
// Port: upstream builds its test profiles with its writer, which the subset
// does not have.
func buildProfile(version uint32, class, colorSpace, pcs string, tags ...testTag) []byte {
	b := make([]byte, profileHeaderSize+tagSize*len(tags))
	binary.BigEndian.PutUint32(b[8:], version)
	copy(b[12:], class)
	copy(b[16:], colorSpace)
	copy(b[20:], pcs)
	copy(b[36:], "acsp")
	binary.BigEndian.PutUint32(b[68:], 0x0000f6d6)
	binary.BigEndian.PutUint32(b[72:], 0x00010000)
	binary.BigEndian.PutUint32(b[76:], 0x0000d32d)
	binary.BigEndian.PutUint32(b[128:], uint32(len(tags)))
	for i, tag := range tags {
		for len(b)%4 != 0 {
			b = append(b, 0)
		}
		e := profileHeaderSize + tagSize*i
		copy(b[e:], tag.sig)
		binary.BigEndian.PutUint32(b[e+4:], uint32(len(b)))
		binary.BigEndian.PutUint32(b[e+8:], uint32(len(tag.data)))
		b = append(b, tag.data...)
	}
	binary.BigEndian.PutUint32(b, uint32(len(b)))
	return b
}

// wantKind fails the test when err is not a CmsError of the kind want.
func wantKind(t *testing.T, err error, want CmsErrorKind) {
	t.Helper()
	var e *CmsError
	if !errors.As(err, &e) || e.Kind != want {
		t.Fatalf("error %v, want the kind %d", err, want)
	}
}

func TestProfileVersionParsingStandard(t *testing.T) {
	for _, tc := range []struct {
		v    uint32
		want ProfileVersion
	}{
		{0x00000000, ProfileVersionV20},
		{0x02000000, ProfileVersionV20},
		{0x02400000, ProfileVersionV24},
		{0x04000000, ProfileVersionV40},
		{0x04400000, ProfileVersionV44},
	} {
		got, err := profileVersionFrom(tc.v)
		if err != nil || got != tc.want {
			t.Errorf("profileVersionFrom(%#x) = %#x, %v, want %#x", tc.v, got, err, tc.want)
		}
	}
}

func TestProfileVersionParsingPatchVersions(t *testing.T) {
	// v2.0.2 of SM245B.icc, v3.4 of ibm-t61.icc and v4.2.9 of
	// lcms_samsung_syncmaster.icc.
	for _, v := range []uint32{0x02020000, 0x03400000, 0x04290000} {
		if _, err := profileVersionFrom(v); err != nil {
			t.Errorf("profileVersionFrom(%#x): %v", v, err)
		}
	}
}

func TestProfileVersionParsingRejected(t *testing.T) {
	// v0.1 is not valid. v5.0 is iccMAX, with other white points. v6.0 is
	// unknown.
	for _, v := range []uint32{0x00100000, 0x05000000, 0x06000000} {
		if _, err := profileVersionFrom(v); err == nil {
			t.Errorf("profileVersionFrom(%#x): no error", v)
		}
	}
}

func TestRenderingIntentInvalidDefaultsToPerceptual(t *testing.T) {
	for _, v := range []uint32{0x01000000, 0x04000000, 0xffffffff} {
		if got := renderingIntentFrom(v); got != RenderingIntentPerceptual {
			t.Errorf("renderingIntentFrom(%#x) = %d, want Perceptual", v, got)
		}
	}
}

func TestRgbToXyzMatrixEqualsColorantMatrix(t *testing.T) {
	srgb := NewSrgb()
	rgbToXyz := srgb.RgbToXyzMatrix()
	colorants := srgb.colorantMatrix()
	for i := range 3 {
		for j := range 3 {
			if d := rgbToXyz.V[i][j] - colorants.V[i][j]; d > 1e-10 || d < -1e-10 {
				t.Errorf("sRGB: element [%d][%d] differs", i, j)
			}
		}
	}
}

func TestReadHayroProfiles(t *testing.T) {
	lab := readProfile(t, "LAB.icc")
	if lab.ColorSpace != DataColorSpaceLab || lab.Pcs != DataColorSpaceLab || lab.ProfileClass != ProfileClassAbstract {
		t.Errorf("LAB.icc: color space %d, PCS %d, class %d", lab.ColorSpace, lab.Pcs, lab.ProfileClass)
	}
	if lab.Version() != ProfileVersionV44 {
		t.Errorf("LAB.icc: version %#x", lab.Version())
	}
	if _, ok := lab.LutAToBPerceptual.(*LutMultidimensionalType); !ok {
		t.Errorf("LAB.icc: A2B0 is %T", lab.LutAToBPerceptual)
	}

	cmyk := readProfile(t, "CGATS001Compat-v2-micro.icc")
	if cmyk.ColorSpace != DataColorSpaceCmyk || cmyk.Pcs != DataColorSpaceLab {
		t.Errorf("CGATS: color space %d, PCS %d", cmyk.ColorSpace, cmyk.Pcs)
	}
	if lut, ok := cmyk.LutAToBPerceptual.(*LutDataType); !ok || lut.LutType != LutTypeLut16 {
		t.Errorf("CGATS: A2B0 is %T", cmyk.LutAToBPerceptual)
	}

	prophoto := readProfile(t, "ProPhoto-v2-micro.icc")
	if !prophoto.IsMatrixShaper() {
		t.Error("ProPhoto: not a matrix shaper")
	}
}

func TestInvalidRenderingIntentDefaultsToPerceptual(t *testing.T) {
	// A profile with the rendering intent 0x01000000, as in old Linotype
	// "Lino" profiles.
	b := readTestdata(t, "ProPhoto-v2-micro.icc")
	binary.BigEndian.PutUint32(b[64:], 0x01000000)
	p, err := NewFromSlice(b)
	if err != nil {
		t.Fatal(err)
	}
	if p.RenderingIntent != RenderingIntentPerceptual {
		t.Errorf("rendering intent %d", p.RenderingIntent)
	}
	tr, err := p.CreateTransform8Bit(LayoutRgba, NewSrgb(), LayoutRgba, DefaultTransformOptions())
	if err != nil {
		t.Fatal(err)
	}
	out := make([]uint8, 4)
	if err := tr.Transform([]uint8{128, 128, 128, 255}, out); err != nil {
		t.Fatal(err)
	}
	if out[3] != 255 {
		t.Errorf("alpha %d, want 255", out[3])
	}
}

func TestMalformedProfiles(t *testing.T) {
	good := readTestdata(t, "ProPhoto-v2-micro.icc")
	for name, mutate := range map[string]func([]byte) []byte{
		"short":         func(b []byte) []byte { return b[:100] },
		"signature":     func(b []byte) []byte { copy(b[36:], "xxxx"); return b },
		"color space":   func(b []byte) []byte { copy(b[16:], "????"); return b },
		"version":       func(b []byte) []byte { b[8] = 5; return b },
		"tag count":     func(b []byte) []byte { binary.BigEndian.PutUint32(b[128:], 0xffffffff); return b },
		"truncated tag": func(b []byte) []byte { return b[:len(b)-8] },
	} {
		b := append([]byte(nil), good...)
		_, err := NewFromSlice(mutate(b))
		if !errors.Is(err, ivy.ErrFormat) {
			t.Errorf("%s: error %v, want ivy.ErrFormat", name, err)
		}
	}
}
