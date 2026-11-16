// Copyright (c) 2026 Serge Vakulenko
// SPDX-License-Identifier: MIT

// Copied from github.com/sergev/dejapdf, file internal/djvu/palette_test.go, at
// commit 2b6ca0c1123c (D7, D26, docs/UPSTREAM.md). Its MIT license is in
// djvu/internal/LICENSE.

package doc // ivy: renamed from djvu, the name of the package of the format

import (
	"path/filepath"
	"testing"
)

func TestPalette(t *testing.T) {
	d, err := Open(filepath.Join(corpus, "palette.djvu"))
	if err != nil {
		t.Fatal(err)
	}
	pg := d.Pages[0]
	pal, err := pg.Palette()
	if err != nil {
		t.Fatal(err)
	}
	m, err := d.Mask(pg)
	if err != nil {
		t.Fatal(err)
	}
	if len(pal.Colors) != 4 || len(pal.Index) != 2055 || len(m.Blits) != len(pal.Index) {
		t.Fatalf("%d colours, %d indices, %d blits", len(pal.Colors), len(pal.Index), len(m.Blits))
	}
	if p, err := d.Pages[0].Palette(); err != nil || p == nil {
		t.Fatal(p, err)
	}
	for _, b := range [][]byte{{}, {1, 0, 0}, {0, 0, 2, 1, 2, 3}, {0x80, 0, 1, 1, 2, 3, 0}} {
		if _, err := parsePalette(b); err == nil {
			t.Errorf("% x: no error", b)
		}
	}
}
