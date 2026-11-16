// Copyright (c) 2026 Serge Vakulenko
// SPDX-License-Identifier: MIT

// Copied from github.com/sergev/dejapdf, file internal/djvu/djvu_test.go, at
// commit 2b6ca0c1123c (D7, D26, docs/UPSTREAM.md). Its MIT license is in
// djvu/internal/LICENSE.

package doc // ivy: renamed from djvu, the name of the package of the format

import (
	"encoding/binary"
	"os"
	"path/filepath"
	"testing"
)

const corpus = "../../testdata" // ivy: the files of the tests are in djvu/testdata

// Open reads a document from a file, for the tests.
//
// ivy: the copy leaves out Open, which also reads the files that a
// document names. The tests open only single and bundled documents.
func Open(path string) (*Document, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return Parse(data, nil)
}

// ivy: the copy leaves out TestDumpCorpus, which tests the dump of the
// chunks, and TestIncludeFile, TestIndirect and TestBundledMatchesIndirect,
// which read indirect documents. ivy does not open an indirect document.

func TestParseInfo(t *testing.T) {
	mk := func(minor, major byte, dpi uint16, gamma, flags byte) []byte {
		b := binary.BigEndian.AppendUint16(nil, 2550)
		b = binary.BigEndian.AppendUint16(b, 3300)
		b = append(b, minor, major)
		b = binary.LittleEndian.AppendUint16(b, dpi)
		return append(b, gamma, flags)
	}
	cases := []struct {
		b    []byte
		want Info
	}{
		{mk(24, 0, 300, 22, 1), Info{2550, 3300, 24, 300, 22, 0}},
		{mk(24, 1, 600, 22, 6), Info{2550, 3300, 280, 600, 22, 270}},
		{mk(24, 0, 24, 2, 2), Info{2550, 3300, 24, 300, 3, 180}},
		{mk(24, 0, 6001, 51, 5), Info{2550, 3300, 24, 300, 50, 90}},
		{mk(24, 0, 25, 3, 0x0d), Info{2550, 3300, 24, 25, 3, 90}},
		{mk(24, 0, 6000, 50, 0)[:4], Info{2550, 3300, 0, 300, 22, 0}},
	}
	for i, c := range cases {
		got, err := ParseInfo(c.b)
		if err != nil || got != c.want {
			t.Errorf("case %d: got %+v, %v; want %+v", i, got, err, c.want)
		}
	}
	if _, err := ParseInfo([]byte{1, 2, 3}); err == nil {
		t.Error("short INFO accepted")
	}
}

func TestDirBody(t *testing.T) {
	d := &Dir{Components: make([]Component, 3)}
	body := []byte{0, 1, 2, 0, 0, 3, 1, 0, 0}
	body = append(body, 0x01, 0xc1, 0x03)
	body = append(body, "a.djvu\x00b.djvu\x00b-file.djvu\x00Two\x00s.iff\x00"...)
	if err := d.parseDirBody(body); err != nil {
		t.Fatal(err)
	}
	want := []Component{
		{ID: "a.djvu", Name: "a.djvu", Title: "a.djvu", Type: PageComp, Size: 0x102},
		{ID: "b.djvu", Name: "b-file.djvu", Title: "Two", Type: PageComp, Size: 3, HasName: true, HasTitle: true},
		{ID: "s.iff", Name: "s.iff", Title: "s.iff", Type: SharedAnno, Size: 0x10000},
	}
	for i := range want {
		if d.Components[i] != want[i] {
			t.Errorf("%d: got %+v want %+v", i, d.Components[i], want[i])
		}
	}
	if d.NumPages() != 2 {
		t.Errorf("pages %d", d.NumPages())
	}
	if err := d.parseDirBody(body[:len(body)-1]); err == nil {
		t.Error("unterminated name accepted")
	}
	if err := d.parseDirBody(body[:5]); err == nil {
		t.Error("truncated body accepted")
	}
}

func TestSinglePage(t *testing.T) {
	d, err := Open(filepath.Join(corpus, "photo.djvu"))
	if err != nil {
		t.Fatal(err)
	}
	if len(d.Pages) != 1 || d.Dir != nil {
		t.Fatalf("pages %d", len(d.Pages))
	}
	p := d.Pages[0]
	if p.Info != (Info{850, 1100, 24, 100, 22, 0}) {
		t.Errorf("info %+v", p.Info)
	}
	if n := len(p.Chunks("BG44")); n != 3 {
		t.Errorf("BG44 chunks %d", n)
	}
}

func TestBundled(t *testing.T) {
	d, err := Open(filepath.Join(corpus, "shared-dict.djvu"))
	if err != nil {
		t.Fatal(err)
	}
	if len(d.Pages) != 2 || !d.Dir.Bundled || len(d.Dir.Components) != 3 {
		t.Fatalf("pages %d", len(d.Pages))
	}
	for _, p := range d.Pages {
		if len(p.Chunks("Djbz")) != 1 || len(p.Chunks("Sjbz")) != 1 {
			t.Errorf("%s: Djbz %d Sjbz %d", p.ID, len(p.Chunks("Djbz")), len(p.Chunks("Sjbz")))
		}
	}
	if d.Pages[0].Includes[0] != d.Pages[1].Includes[0] {
		t.Error("shared dictionary not shared")
	}
	if d.Pages[0].ID != "p1.djvu" || d.Pages[1].ID != "p4.djvu" {
		t.Errorf("ids %q %q", d.Pages[0].ID, d.Pages[1].ID)
	}
}
