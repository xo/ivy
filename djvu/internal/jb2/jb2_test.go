// Copyright (c) 2026 Serge Vakulenko
// SPDX-License-Identifier: MIT

// Copied from github.com/sergev/dejapdf, file internal/jb2/jb2_test.go, at
// commit 2b6ca0c1123c (D7, D26, docs/UPSTREAM.md). Its MIT license is in
// djvu/internal/LICENSE.

package jb2

import (
	"os"
	"testing"

	"github.com/xo/ivy/djvu/internal/iff"
)

// chunk returns the first chunk with the given id in a corpus file.
func chunk(t testing.TB, file, id string) []byte {
	// ivy: the files of the tests are in djvu/testdata.
	b, err := os.ReadFile("../../testdata/" + file)
	if err != nil {
		t.Fatal(err)
	}
	root, err := iff.Parse(b)
	if err != nil {
		t.Fatal(err)
	}
	var find func(*iff.Chunk) *iff.Chunk
	find = func(c *iff.Chunk) *iff.Chunk {
		if c.ID == id {
			return c
		}
		for _, k := range c.Children {
			if f := find(k); f != nil {
				return f
			}
		}
		return nil
	}
	c := find(root)
	if c == nil {
		t.Fatalf("%s: no %s", file, id)
	}
	return c.Data
}

func TestDecode(t *testing.T) {
	im, err := Decode(chunk(t, "bilevel-lossy.djvu", "Sjbz"), nil)
	if err != nil {
		t.Fatal(err)
	}
	if im.W != 2550 || im.H != 3300 || im.NDict != 0 || len(im.Blits) != 2066 {
		t.Errorf("got %dx%d, %d dict, %d blits", im.W, im.H, im.NDict, len(im.Blits))
	}
	for _, b := range im.Blits {
		s := im.Shape(b.Shape)
		if s.W == 0 || s.Pix[0]|s.Pix[len(s.Pix)-1] > 1 {
			t.Fatalf("bad shape %d", b.Shape)
		}
	}
}

func TestDict(t *testing.T) {
	sjbz := chunk(t, "shared-dict.djvu", "Sjbz")
	if _, err := Decode(sjbz, nil); err == nil {
		t.Error("missing dictionary accepted")
	}
	d, err := DecodeDict(chunk(t, "shared-dict.djvu", "Djbz"), nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(d.Shapes) != 70 {
		t.Errorf("%d shapes", len(d.Shapes))
	}
	if _, err := Decode(sjbz, &Dict{Shapes: d.Shapes[:10]}); err == nil {
		t.Error("short dictionary accepted")
	}
	im, err := Decode(sjbz, d)
	if err != nil || im.NDict != 70 {
		t.Fatalf("%v, %d dict", err, im.NDict)
	}
}

func TestTruncated(t *testing.T) {
	b := chunk(t, "bilevel.djvu", "Sjbz")
	for _, n := range []int{0, 1, 10, len(b) / 2, len(b) - 100} {
		if _, err := Decode(b[:n], nil); err == nil {
			t.Errorf("truncated at %d accepted", n)
		}
	}
}

func FuzzDecode(f *testing.F) {
	f.Add(chunk(f, "bilevel-lossy.djvu", "Sjbz")[:400])
	f.Add(chunk(f, "shared-dict.djvu", "Djbz")[:400])
	f.Add([]byte{0xff, 0, 0x55})
	f.Fuzz(func(t *testing.T, b []byte) {
		if d, err := DecodeDict(b, nil); err == nil {
			Decode(b, d)
		}
	})
}
