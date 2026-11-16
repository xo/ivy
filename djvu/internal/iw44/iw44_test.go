// Copyright (c) 2026 Serge Vakulenko
// SPDX-License-Identifier: MIT

// Copied from github.com/sergev/dejapdf, file internal/iw44/iw44_test.go, at
// commit 2b6ca0c1123c (D7, D26, docs/UPSTREAM.md). Its MIT license is in
// djvu/internal/LICENSE.

package iw44

import (
	"os"
	"testing"

	"github.com/xo/ivy/djvu/internal/iff"
)

func bg44(t testing.TB, file string) [][]byte {
	// ivy: the files of the tests are in djvu/testdata.
	b, err := os.ReadFile("../../testdata/" + file)
	if err != nil {
		t.Fatal(err)
	}
	root, err := iff.Parse(b)
	if err != nil {
		t.Fatal(err)
	}
	var r [][]byte
	for _, c := range root.All("BG44") {
		r = append(r, c.Data)
	}
	return r
}

func TestHeader(t *testing.T) {
	cs := bg44(t, "photo.djvu")
	var d Decoder
	if _, err := d.Image(); err == nil {
		t.Error("image without data")
	}
	if err := d.Decode(cs[1]); err == nil {
		t.Error("chunk 2 accepted first")
	}
	for _, c := range cs {
		if err := d.Decode(c); err != nil {
			t.Fatal(err)
		}
	}
	im, _ := d.Image()
	if im.W != 850 || im.H != 1100 || im.Gray || len(im.Pix) != 3*850*1100 || d.delay != 0 || d.chromaHalf {
		t.Errorf("got %dx%d gray=%v delay=%d half=%v", im.W, im.H, im.Gray, d.delay, d.chromaHalf)
	}
	for _, b := range [][]byte{nil, {0}, {0, 1, 1, 2, 0, 0, 0, 1, 0}, {0, 1, 2, 2, 0, 1, 0, 1, 0}} {
		var d Decoder
		if d.Decode(b) == nil {
			t.Errorf("% x accepted", b)
		}
	}
}

// Truncated chunks decode (past the end the coder reads 0xff).
func TestTruncated(t *testing.T) {
	c := bg44(t, "gray.djvu")[0]
	for _, n := range []int{9, 10, 100, len(c) / 2} {
		var d Decoder
		if err := d.Decode(c[:n]); err != nil {
			t.Fatalf("%d bytes: %v", n, err)
		}
		if _, err := d.Image(); err != nil {
			t.Fatal(err)
		}
	}
}

func FuzzDecode(f *testing.F) {
	f.Add(bg44(f, "gray.djvu")[0][:200])
	f.Add([]byte{0, 3, 0, 2, 0, 40, 0, 30, 0x81, 1, 2, 3})
	f.Fuzz(func(t *testing.T, b []byte) {
		if len(b) > 9 && int(b[4])<<8|int(b[5]) > 512 || len(b) > 9 && int(b[6])<<8|int(b[7]) > 512 {
			return // keep runs fast
		}
		var d Decoder
		if d.Decode(b) == nil {
			d.Image()
		}
	})
}

func TestSize(t *testing.T) {
	if _, _, ok := Size([]byte{1, 0}); ok {
		t.Error("Size of a later chunk")
	}
	w, h, ok := Size([]byte{0, 74, 1, 2, 0x0d, 0xee, 0x15, 0xae, 0})
	if !ok || w != 3566 || h != 5550 {
		t.Errorf("Size = %d, %d, %v", w, h, ok)
	}
}

// ivy: added. A reduced image holds the pixels of the full image at the
// multiples of the scale, within the error of the coarser transform.
func TestImageScaled(t *testing.T) {
	for _, file := range []string{"photo.djvu", "photo-half.djvu", "gray.djvu"} {
		var d Decoder
		for _, c := range bg44(t, file) {
			if err := d.Decode(c); err != nil {
				t.Fatal(err)
			}
		}
		full, err := d.Image()
		if err != nil {
			t.Fatal(err)
		}
		if same, _ := d.ImageScaled(1); same.W != full.W {
			t.Errorf("%s: a scale of 1 is %dx%d", file, same.W, same.H)
		}
		for _, s := range []int{2, 4, 8, 16} {
			im, err := d.ImageScaled(s)
			if err != nil {
				t.Fatal(err)
			}
			if im.W != (full.W+s-1)/s || im.H != (full.H+s-1)/s {
				t.Fatalf("%s: scale %d is %dx%d", file, s, im.W, im.H)
			}
			n := 3
			if im.Gray {
				n = 1
			}
			var sum, count float64
			for r := range im.H {
				fr := full.H - 1 - (im.H-1-r)*s
				for x := range im.W {
					for k := range n {
						a := float64(im.Pix[n*(r*im.W+x)+k])
						b := float64(full.Pix[n*(fr*full.W+x*s)+k])
						sum += max(a-b, b-a)
						count++
					}
				}
			}
			// On 2026-11-17 the largest difference is 1.9, at a scale of 16.
			if mean := sum / count; mean > 4 {
				t.Errorf("%s: scale %d differs from the full image by %.1f", file, s, mean)
			}
		}
	}
	var d Decoder
	if _, err := d.ImageScaled(2); err == nil {
		t.Error("image without data")
	}
	for _, c := range bg44(t, "gray.djvu") {
		_ = d.Decode(c)
	}
	for _, s := range []int{3, 32} {
		if _, err := d.ImageScaled(s); err == nil {
			t.Errorf("scale %d accepted", s)
		}
	}
}
