// Copyright (c) 2026 Serge Vakulenko
// SPDX-License-Identifier: MIT

// Copied from github.com/sergev/dejapdf, file internal/djvu/mask_test.go, at
// commit 2b6ca0c1123c (D7, D26, docs/UPSTREAM.md). Its MIT license is in
// djvu/internal/LICENSE.

package doc // ivy: renamed from djvu, the name of the package of the format

import (
	"bufio"
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/xo/ivy/djvu/internal/jb2"
)

// pbm encodes a bitmap as ddjvu -format=pbm does.
func pbm(b *jb2.Bitmap) []byte {
	out := fmt.Appendf(nil, "P4\n%d %d\n", b.W, b.H)
	for r := range b.H {
		for c := 0; c < b.W; c += 8 {
			var v byte
			for k := range min(8, b.W-c) {
				v |= b.Pix[r*b.W+c+k] << (7 - k)
			}
			out = append(out, v)
		}
	}
	return out
}

// Masks match ddjvu -mode=mask (masks.sha256, from mkcorpus.sh).
func TestMaskCorpus(t *testing.T) {
	f, err := os.Open(filepath.Join(corpus, "masks.sha256"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	docs := map[string]*Document{}
	n := 0
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		w := strings.Fields(sc.Text())
		page, _ := strconv.Atoi(w[2])
		// ivy: djvu/testdata holds a part of the corpus of dejapdf.
		if _, err := os.Stat(filepath.Join(corpus, w[1])); err != nil {
			continue
		}
		d := docs[w[1]]
		if d == nil {
			if d, err = Open(filepath.Join(corpus, w[1])); err != nil {
				t.Fatal(err)
			}
			docs[w[1]] = d
		}
		p := d.Pages[page-1]
		im, err := d.Mask(p)
		if err != nil {
			t.Fatalf("%s page %d: %v", w[1], page, err)
		}
		var bm *jb2.Bitmap
		if im != nil {
			bm = im.Render()
			n++
		} else {
			bm = &jb2.Bitmap{W: p.Info.Width, H: p.Info.Height, Pix: make([]byte, p.Info.Width*p.Info.Height)}
		}
		if got := fmt.Sprintf("%x", sha256.Sum256(pbm(bm))); got != w[0] {
			t.Errorf("%s page %d: mask differs from ddjvu", w[1], page)
		}
	}
	if n < 10 {
		t.Errorf("only %d masks", n)
	}
	// The shared dictionary is decoded once.
	d := docs["shared-dict.djvu"]
	if len(d.dicts) != 1 {
		t.Errorf("%d dictionaries decoded", len(d.dicts))
	}
}
