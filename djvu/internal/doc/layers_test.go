// Copyright (c) 2026 Serge Vakulenko
// SPDX-License-Identifier: MIT

// Copied from github.com/sergev/dejapdf, file internal/djvu/layers_test.go, at
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

	"github.com/xo/ivy/djvu/internal/iw44"
)

// ppm encodes an image as ddjvu -format=ppm does (gray as RGB).
func ppm(im *iw44.Image) []byte {
	out := fmt.Appendf(nil, "P6\n%d %d\n255\n", im.W, im.H)
	if !im.Gray {
		return append(out, im.Pix...)
	}
	for _, v := range im.Pix {
		out = append(out, v, v, v)
	}
	return out
}

// IW44 layers match ddjvu (iw44.sha256, from mkcorpus.sh).
func TestLayersCorpus(t *testing.T) {
	f, err := os.Open(filepath.Join(corpus, "iw44.sha256"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	n := 0
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		w := strings.Fields(sc.Text())
		page, _ := strconv.Atoi(w[2])
		// ivy: djvu/testdata holds a part of the corpus of dejapdf.
		if _, err := os.Stat(filepath.Join(corpus, w[1])); err != nil {
			continue
		}
		d, err := Open(filepath.Join(corpus, w[1]))
		if err != nil {
			t.Fatal(err)
		}
		p := d.Pages[page-1]
		var im *iw44.Image
		if w[3] == "BG44" {
			im, err = p.Background()
		} else {
			im, err = p.Foreground()
		}
		if err != nil || im == nil {
			t.Fatalf("%s page %d %s: %v", w[1], page, w[3], err)
		}
		if got := fmt.Sprintf("%x", sha256.Sum256(ppm(im))); got != w[0] {
			t.Errorf("%s page %d %s differs from ddjvu", w[1], page, w[3])
		}
		n++
	}
	if n < 10 {
		t.Errorf("only %d layers", n)
	}
}
