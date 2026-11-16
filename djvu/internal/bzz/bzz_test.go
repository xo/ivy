// Copyright (c) 2026 Serge Vakulenko
// SPDX-License-Identifier: MIT

// Copied from github.com/sergev/dejapdf, file internal/bzz/bzz_test.go, at
// commit 2b6ca0c1123c (D7, D26, docs/UPSTREAM.md). Its MIT license is in
// djvu/internal/LICENSE.

package bzz

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Fixtures made with DjVuLibre's `bzz -e` (text.bzz: 10K blocks).
func TestFixtures(t *testing.T) {
	ins, _ := filepath.Glob("testdata/*.in")
	if len(ins) == 0 {
		t.Fatal("no fixtures")
	}
	for _, in := range ins {
		want, err := os.ReadFile(in)
		if err != nil {
			t.Fatal(err)
		}
		b, err := os.ReadFile(strings.TrimSuffix(in, ".in") + ".bzz")
		if err != nil {
			t.Fatal(err)
		}
		got, err := Decode(b)
		if err != nil || !bytes.Equal(got, want) {
			t.Errorf("%s: err %v, %d bytes, want %d", in, err, len(got), len(want))
		}
	}
}

func TestEmpty(t *testing.T) {
	// `bzz -e` writes nothing for empty input; a zero block size ends it.
	if got, err := Decode(nil); err != nil || len(got) != 0 {
		t.Errorf("got %d bytes, %v", len(got), err)
	}
}

// ivy: TestOracle is left out. It runs the bzz program of DjVuLibre, and
// only the test module of ivy runs an outside program.

func FuzzDecode(f *testing.F) {
	for _, p := range []string{"testdata/one.bzz", "testdata/random.bzz"} {
		if b, err := os.ReadFile(p); err == nil {
			f.Add(b)
		}
	}
	f.Fuzz(func(t *testing.T, b []byte) {
		Decode(b)
	})
}
