// Copyright (c) The Hayro Authors
//
// Use of this source code is governed by the Apache-2.0 or MIT license that
// can be found in the LICENSE-APACHE and LICENSE-MIT files.

// This file ports the tests of hayro-syntax src/filter/ascii_hex.rs,
// src/filter/run_length.rs, src/filter/lzw_flate.rs and src/filter/ccitt.rs.

package syntax

import (
	"bytes"
	"testing"
)

func TestDecodeASCIIHex(t *testing.T) {
	tests := []struct {
		data string
		want []byte
	}{
		{"AF3E2901>", []byte{0xaf, 0x3e, 0x29, 0x01}},
		{"AF3   E2   901>", []byte{0xaf, 0x3e, 0x29, 0x01}},
		{"AF3E2901", []byte{0xaf, 0x3e, 0x29, 0x01}},
		{"AF3E291>", []byte{0xaf, 0x3e, 0x29, 0x10}},
	}
	for _, test := range tests {
		got, ok := decodeASCIIHex([]byte(test.data))
		if !ok || !bytes.Equal(got, test.want) {
			t.Errorf("%q: got %x, %v", test.data, got, ok)
		}
	}
}

func TestRunLength(t *testing.T) {
	got, ok := decodeRunLength([]byte{4, 10, 11, 12, 13, 14, 253, 3, 128}, 1<<20)
	if !ok || !bytes.Equal(got, []byte{10, 11, 12, 13, 14, 3, 3, 3, 3}) {
		t.Fatalf("got %v, %v", got, ok)
	}
}

func TestDecodeLzw(t *testing.T) {
	input := []byte{0x80, 0x0B, 0x60, 0x50, 0x22, 0x0C, 0x0C, 0x85, 0x01}
	got, err := decodeLzw(input, EmptyDict(), 1<<20)
	if err != nil || !bytes.Equal(got, []byte{45, 45, 45, 45, 45, 65, 45, 45, 45, 66}) {
		t.Fatalf("got %v, %v", got, err)
	}
}

func TestDecodeFlate(t *testing.T) {
	inputs := [][]byte{
		{0x78, 0x9c, 0xf3, 0x48, 0xcd, 0xc9, 0xc9, 0x7, 0x0, 0x5, 0x8c, 0x1, 0xf5},
		{0xf3, 0x48, 0xcd, 0xc9, 0xc9, 0x7, 0x0},
	}
	for _, input := range inputs {
		got, err := decodeFlate(input, EmptyDict(), 1<<20)
		if err != nil || string(got) != "Hello" {
			t.Errorf("%x: got %q, %v", input, got, err)
		}
	}
}

// TestCcittIssue1258 is the test issue1258 of hayro-syntax
// src/filter/ccitt.rs.
func TestCcittIssue1258(t *testing.T) {
	params, _ := FromBytes[Dict]([]byte("<< /K 0 /Columns 8 /Rows 1 >>"))

	decoded, err := decodeCcitt([]byte{0x35, 0x14}, params, &ImageDecodeParams{}, defaultDecodeEnv())
	if err != nil {
		t.Fatalf("decoding: %v", err)
	}

	if !bytes.Equal(decoded.Data, make([]byte, 8)) || decoded.ImageData.Height != 1 {
		t.Fatalf("got %v, height %d", decoded.Data, decoded.ImageData.Height)
	}
}

// TestCcittUnknownRows is the test unknown_rows of hayro-syntax
// src/filter/ccitt.rs.
func TestCcittUnknownRows(t *testing.T) {
	params, _ := FromBytes[Dict]([]byte("<< /K 0 /Columns 8 >>"))

	decoded, err := decodeCcitt([]byte{0x35, 0x14}, params, &ImageDecodeParams{}, defaultDecodeEnv())
	if err != nil {
		t.Fatalf("decoding: %v", err)
	}
	if !bytes.Equal(decoded.Data, make([]byte, 8)) || decoded.ImageData.Height != 1 {
		t.Fatalf("got %v, height %d", decoded.Data, decoded.ImageData.Height)
	}

	imageParams := &ImageDecodeParams{IsIndexed: true}
	decoded, err = decodeCcitt([]byte{0x35, 0x14}, params, imageParams, defaultDecodeEnv())
	if err != nil {
		t.Fatalf("decoding indexed: %v", err)
	}
	if !bytes.Equal(decoded.Data, []byte{0}) || decoded.ImageData.Height != 1 {
		t.Fatalf("indexed: got %v, height %d", decoded.Data, decoded.ImageData.Height)
	}
}

func TestCcittGroup4(t *testing.T) {
	// Group 4 data of an image of 8 by 2 pixels: the first row white, the
	// second row black. The codes are V0, then H with white 0 and black 8,
	// then EOFB.
	data := []byte{0x93, 0x51, 0x40, 0x04, 0x00, 0x40}
	dict, _ := FromBytes[Dict]([]byte("<< /K -1 /Columns 8 /Rows 2 >>"))
	got, err := decodeCcitt(data, dict, &ImageDecodeParams{}, defaultDecodeEnv())
	if err != nil {
		t.Fatalf("decoding: %v", err)
	}
	want := []byte{0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0, 0, 0, 0, 0, 0, 0, 0}
	if !bytes.Equal(got.Data, want) || got.ImageData.Height != 2 {
		t.Fatalf("got %v, height %d", got.Data, got.ImageData.Height)
	}
	got, err = decodeCcitt(data, dict, &ImageDecodeParams{IsIndexed: true}, defaultDecodeEnv())
	if err != nil || !bytes.Equal(got.Data, []byte{0xFF, 0x00}) || got.ImageData.BitsPerComponent != 1 {
		t.Fatalf("indexed: got %v, %v", got.Data, err)
	}
}
