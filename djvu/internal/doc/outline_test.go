// Copyright (c) 2026 Serge Vakulenko
// SPDX-License-Identifier: MIT

// Copied from github.com/sergev/dejapdf, files internal/djvu/text_test.go
// and internal/djvu/fuzz_test.go, at commit 2b6ca0c1123c (D7, D26,
// docs/UPSTREAM.md). Its MIT license is in djvu/internal/LICENSE.

package doc // ivy: renamed from djvu, the name of the package of the format

import (
	"bytes"
	"testing"
)

// ivy: TestParseOutlineErrors of text_test.go and FuzzParseOutline of
// fuzz_test.go, which go with outline.go.

func TestParseOutlineErrors(t *testing.T) {
	ok := []byte{0, 2, 1, 0, 0, 1, 'A', 0, 0, 2, '#', '1', 0, 0, 0, 1, 'B', 0, 0, 0}
	o, err := ParseOutline(ok)
	if err != nil || len(o) != 1 || o[0].URL != "#1" || o[0].Children[0].Title != "B" {
		t.Fatalf("got %v, %v", o, err)
	}
	for n := range len(ok) - 1 {
		if _, err := ParseOutline(ok[:n]); err == nil {
			t.Errorf("truncated at %d accepted", n)
		}
	}
	bad := bytes.Clone(ok)
	bad[1] = 1 // child beyond the total
	if _, err := ParseOutline(bad); err == nil {
		t.Error("bad count accepted")
	}
}

func FuzzParseOutline(f *testing.F) {
	seeds(f, true, "NAVM")
	f.Fuzz(func(t *testing.T, b []byte) { _, _ = ParseOutline(b) }) // ivy: the results are ignored, for the linter
}
