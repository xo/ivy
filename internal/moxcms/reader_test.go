// Copyright (c) Radzivon Bartoshyk. All rights reserved.
//
// Use of this source code is governed by the BSD-3-Clause or Apache-2.0
// license that can be found in the LICENSE.md and LICENSE-APACHE.md files.

// This file ports the tests of moxcms src/reader.rs.

package moxcms

import (
	"encoding/binary"
	"testing"
)

// makeMabTagWithClut is make_mab_tag_with_clut in the tests of moxcms
// src/reader.rs. It returns an mAB tag with a CLUT of the grid gridValues
// and entries of one byte.
func makeMabTagWithClut(inChan, outChan uint8, gridValues []uint8) []byte {
	const clutOffset = 48
	var gridPoints [16]uint8
	copy(gridPoints[:], gridValues)
	clutSize := uint64(1)
	for _, g := range gridPoints[:inChan] {
		clutSize *= uint64(g)
	}
	clutSize *= uint64(outChan)
	tag := make([]byte, 48+20+int(clutSize))
	copy(tag, "mAB ")
	tag[8] = inChan
	tag[9] = outChan
	binary.BigEndian.PutUint32(tag[24:], clutOffset)
	copy(tag[clutOffset:], gridPoints[:])
	tag[clutOffset+16] = 1
	return tag
}

// makeMft1Tag is make_mft1_tag in the tests of moxcms src/reader.rs. It
// returns a minimal lut8Type tag.
func makeMft1Tag(inChan, outChan, gridPoints uint8) []byte {
	clutSize := 1
	for range inChan {
		clutSize *= int(gridPoints)
	}
	clutSize *= int(outChan)
	total := 48 + 256*int(inChan) + clutSize + 256*int(outChan)
	tag := make([]byte, total)
	copy(tag, "mft1")
	tag[8] = inChan
	tag[9] = outChan
	tag[10] = gridPoints
	for i := range 3 {
		binary.BigEndian.PutUint32(tag[12+i*3*4+i*4:], 0x00010000)
	}
	return tag
}

func TestMabRejectsDegenerateGridPoints(t *testing.T) {
	tag := makeMabTagWithClut(3, 3, []uint8{1, 1, 1})
	if _, err := readLutAbmType(tag, 0, uint64(len(tag)), true, DefaultParsingOptions()); err == nil {
		t.Error("no error")
	}
}

func TestMft1RejectsDegenerateGridPoints(t *testing.T) {
	tag := makeMft1Tag(3, 3, 1)
	_, err := readLutAToBType(tag, 0, uint64(len(tag)), DefaultParsingOptions())
	wantKind(t, err, InvalidProfile)
}

// curv returns a curveType tag with the entries.
func curv(entries ...uint16) []byte {
	b := make([]byte, 12+2*len(entries))
	copy(b, "curv")
	binary.BigEndian.PutUint32(b[8:], uint32(len(entries)))
	for i, e := range entries {
		binary.BigEndian.PutUint16(b[12+2*i:], e)
	}
	return b
}

// para returns a parametricCurveType tag of the function type with the
// parameters.
func para(function uint16, params ...float64) []byte {
	b := make([]byte, 12+4*len(params))
	copy(b, "para")
	binary.BigEndian.PutUint16(b[8:], function)
	for i, p := range params {
		binary.BigEndian.PutUint32(b[12+4*i:], uint32(int32(p*65536)))
	}
	return b
}

// align4 pads b with zeros to a multiple of 4 bytes.
func align4(b []byte) []byte {
	for len(b)%4 != 0 {
		b = append(b, 0)
	}
	return b
}

// cmykMabProfile returns a CMYK profile of version 4.4 with a Lab PCS. Its
// A2B0 is an mAB tag with four empty A curves, a CLUT of 2x2x2x2 nodes of
// zero, and the B curves bCurves.
func cmykMabProfile(bCurves ...[]byte) []byte {
	tag := make([]byte, 32)
	copy(tag, "mAB ")
	tag[8] = 4
	tag[9] = 3

	binary.BigEndian.PutUint32(tag[12:], uint32(len(tag)))
	for _, c := range bCurves {
		tag = align4(append(tag, c...))
	}

	binary.BigEndian.PutUint32(tag[24:], uint32(len(tag)))
	clut := make([]byte, 20+16*3*2)
	copy(clut, []byte{2, 2, 2, 2})
	clut[16] = 2
	tag = append(tag, clut...)

	binary.BigEndian.PutUint32(tag[28:], uint32(len(tag)))
	for range 4 {
		tag = align4(append(tag, curv()...))
	}
	return buildProfile(0x04400000, "prtr", "CMYK", "Lab ", testTag{"A2B0", tag})
}

func TestCurveAfterAnEmptyCurvInALutAToBIsRead(t *testing.T) {
	// An empty curv is the identity. The curve after one in the same set
	// must be read from its own offset, not as another identity.
	gamma := para(0, 2.0)
	b := cmykMabProfile(curv(), gamma, curv())
	p, err := NewFromSlice(b)
	if err != nil {
		t.Fatal(err)
	}
	read, ok := p.LutAToBPerceptual.(*LutMultidimensionalType)
	if !ok {
		t.Fatalf("A2B0 is %T", p.LutAToBPerceptual)
	}
	empty := &ToneReprCurve{}
	wantGamma := &ToneReprCurve{Kind: CurveParametric, Parametric: []float32{2}}
	if len(read.BCurves) != 3 || !read.BCurves[0].equal(empty) || !read.BCurves[1].equal(wantGamma) || !read.BCurves[2].equal(empty) {
		t.Errorf("B curves %+v %+v %+v", read.BCurves[0], read.BCurves[1], read.BCurves[2])
	}

	// The port also builds and runs the transform of this profile to sRGB,
	// through the whole grid of an mAB tag.
	tr, err := p.CreateTransform8Bit(LayoutRgba, NewSrgb(), LayoutRgb, DefaultTransformOptions())
	if err != nil {
		t.Fatal(err)
	}
	out := make([]uint8, 6)
	if err := tr.Transform([]uint8{0, 0, 0, 0, 255, 128, 64, 32}, out); err != nil {
		t.Fatal(err)
	}
}
