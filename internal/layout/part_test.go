package layout

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"image/color"
	"io/fs"
	"slices"
	"strings"
	"sync/atomic"
	"testing"
	"testing/fstest"

	"github.com/xo/ivy"
)

// partRows is the number of rows of the table of the tests of the parts.
// A page holds 6 rows, so the whole table has 7 pages.
const partRows = 40

// partStyle sets pages of 6 rows of 10 pixels, and a footer with the page
// and the number of pages.
const partStyle = `@page { size: 100px 81px; margin: 10px 10px 10px 10px;
	@bottom-center { content: counter(page) "/" counter(pages); font-size: 6px } }
body { margin: 0 }
table { border-collapse: collapse; width: 80px }
tr { break-inside: avoid; height: 10px }
tr.pb { break-before: page }
td { padding: 0; font-size: 6px; line-height: 6px; border: 1px solid black }`

// partSplitter is the Splitter of the tests of the parts. It cuts the
// table of partRows rows before each row of cuts. A part from the row a to
// the row b has the path "t.a.b.html". It is also the file system of the
// source, which gives the documents of the parts.
type partSplitter struct {
	cuts []int
	// tall makes the row partTall taller than a page.
	tall   bool
	recuts atomic.Int64
}

// partTall is the row that is taller than a page when partSplitter.tall
// is set.
const partTall = 20

// partTable returns the document of the rows from a to b, with the copy of
// the first row and the row before a when a is not 0, and the row b as the
// probe row when b is not the end. When tall is set, the row partTall is
// taller than a page.
func partTable(a, b int, tall bool) string {
	var s strings.Builder
	s.WriteString(`<html><body><table><tbody>`)
	row := func(i int, class string) {
		text := ""
		if tall && i == partTall {
			text = strings.Repeat("<br>line", 14)
		}
		fmt.Fprintf(&s, `<tr%s><td>row %d%s</td></tr>`, class, i, text)
	}
	if a > 0 {
		row(0, "")
		row(a-1, "")
		row(a, ` class="pb"`)
		a++
	}
	for i := a; i < b; i++ {
		row(i, "")
	}
	if b < partRows {
		row(b, "")
	}
	s.WriteString(`</tbody></table></body></html>`)
	return s.String()
}

// part returns the part from the row a to the row b.
func (s *partSplitter) part(a, b int) Part {
	p := Part{Path: fmt.Sprintf("t.%d.%d.html", a, b), Cut: -1, Probe: -1, Pages: 1}
	n := b - a
	if a > 0 {
		p.Cut = 2
		n += 2
	}
	if b < partRows {
		p.Probe = n
	}
	return p
}

// rows returns the rows of the part of the path name.
func partRange(name string) (a, b int) {
	_, _ = fmt.Sscanf(name, "t.%d.%d.html", &a, &b)
	return a, b
}

// Split satisfies the Splitter interface.
func (s *partSplitter) Split(path string) []Part {
	if path != "t.html" {
		return nil
	}
	var parts []Part
	a := 0
	for _, b := range append(s.cuts, partRows) {
		parts = append(parts, s.part(a, b))
		a = b
	}
	return parts
}

// Recut satisfies the Splitter interface.
func (s *partSplitter) Recut(part, next Part, row int) Part {
	s.recuts.Add(1)
	a, _ := partRange(part.Path)
	_, b := partRange(next.Path)
	switch {
	case row >= 0 && a > 0:
		a += row - 2
	case row >= 0:
		a = row
	}
	return s.part(a, b)
}

// Open satisfies the fs.FS interface.
func (s *partSplitter) Open(name string) (fs.File, error) {
	data := partTable(0, partRows, s.tall)
	if name != "t.html" {
		a, b := partRange(name)
		data = partTable(a, b, s.tall)
	}
	f, err := fstest.MapFS{name: {Data: []byte(data)}}.Open(name)
	if err != nil {
		return nil, fmt.Errorf("opening %s: %w", name, err)
	}
	return f, nil
}

// partHashes returns the hash of the pixels of each page of the book of
// src. It renders the pages of first before PageCount.
func partHashes(t *testing.T, src Source, first ...int) []string {
	t.Helper()
	ctx := context.Background()
	b := NewBook(src, BookOptions{})
	hash := func(n int) string {
		t.Helper()
		p, err := b.Page(ctx, n)
		if err != nil {
			t.Fatalf("page %d: %v", n+1, err)
		}
		img, err := ivy.RenderImage(ctx, p, 96, color.White)
		if err != nil {
			t.Fatal(err)
		}
		h := sha256.New()
		b := img.Bounds()
		for y := b.Min.Y; y < b.Max.Y; y++ {
			for x := b.Min.X; x < b.Max.X; x++ {
				r, g, bl, a := img.At(x, y).RGBA()
				_, _ = h.Write([]byte{byte(r >> 8), byte(g >> 8), byte(bl >> 8), byte(a >> 8)})
			}
		}
		return hex.EncodeToString(h.Sum(nil)[:8])
	}
	early := map[int]string{}
	for _, n := range first {
		early[n] = hash(n)
	}
	n, err := b.PageCount(ctx)
	if err != nil {
		t.Fatal(err)
	}
	hashes := make([]string, n)
	for i := range n {
		hashes[i] = hash(i)
		if h, ok := early[i]; ok && h != hashes[i] {
			t.Errorf("page %d: expected the same pixels before and after PageCount", i+1)
		}
	}
	return hashes
}

// TestBookParts checks that a book that lays out a document in parts gives
// the pages of the whole document, with the page numbers and the number of
// pages in the margin boxes, whether the cuts are at the page breaks or
// not (D55).
func TestBookParts(t *testing.T) {
	whole := &partSplitter{}
	want := partHashes(t, Source{FS: whole, Documents: []string{"t.html"}, StyleSheets: []string{partStyle}})
	if len(want) != 7 {
		t.Fatalf("expected 7 pages, got %d", len(want))
	}
	partsMatch(t, want, false)
}

// TestBookPartsTallRow checks the parts of a document with a row that is
// taller than a page, which the layout breaks across two pages (D55).
func TestBookPartsTallRow(t *testing.T) {
	whole := &partSplitter{tall: true}
	want := partHashes(t, Source{FS: whole, Documents: []string{"t.html"}, StyleSheets: []string{partStyle}})
	for _, cuts := range [][]int{{18, 19, 20, 21, 22}, {20}, {21}, {19, 30}, {12, 24}} {
		for _, first := range [][]int{{0}, {len(want) - 1, 0}, {3}} {
			s := &partSplitter{cuts: cuts, tall: true}
			got := partHashes(t, Source{FS: s, Documents: []string{"t.html"}, StyleSheets: []string{partStyle}, Splitter: s}, first...)
			if !slices.Equal(got, want) {
				t.Errorf("cuts %v, first pages %v: expected the pages of the whole document", cuts, first)
			}
		}
	}
}

// partsMatch checks the parts of the document of partSplitter, whose
// pages have the hashes want.
func partsMatch(t *testing.T, want []string, tall bool) {
	t.Helper()
	for _, c := range []struct {
		cuts   []int
		recuts bool
	}{
		{[]int{12, 24}, false},
		{[]int{6, 12, 18, 24, 30, 36}, false},
		// the rows 8, 20 and 27 do not start a page
		{[]int{8, 20, 27}, true},
		{[]int{1, 2, 3, 30}, true},
	} {
		for _, first := range [][]int{{0}, {6, 0}, {3}} {
			s := &partSplitter{cuts: c.cuts, tall: tall}
			got := partHashes(t, Source{FS: s, Documents: []string{"t.html"}, StyleSheets: []string{partStyle}, Splitter: s}, first...)
			if len(got) != len(want) {
				t.Errorf("cuts %v: expected %d pages, got %d", c.cuts, len(want), len(got))
				continue
			}
			for i := range want {
				if got[i] != want[i] {
					t.Errorf("cuts %v, first pages %v: page %d differs", c.cuts, first, i+1)
				}
			}
			if recut := s.recuts.Load() > 0; recut != c.recuts {
				t.Errorf("cuts %v: expected recuts %v, got %d", c.cuts, c.recuts, s.recuts.Load())
			}
		}
	}
}

// TestCountAllMarginPages checks that countAll keeps the count of a
// document that uses counter(pages) only in its margin boxes, and not the
// count of a document that uses it in its content (D55).
func TestCountAllMarginPages(t *testing.T) {
	src := testBook(3, nil)
	fsys := src.FS.(fstest.MapFS)
	fsys["c1.xhtml"] = &fstest.MapFile{Data: []byte(`<html xmlns="http://www.w3.org/1999/xhtml"><head><style>
		@page { @bottom-center { content: counter(page) " of " counter(pages) } }</style></head>
		<body><p>One</p></body></html>`)}
	fsys["c2.xhtml"] = &fstest.MapFile{Data: []byte(`<html xmlns="http://www.w3.org/1999/xhtml"><head><style>
		p::after { content: " of " counter(pages) }</style></head>
		<body><p>Two</p></body></html>`)}
	b := NewBook(src, BookOptions{})
	for _, bd := range b.docs {
		bd.count = -1
	}
	// Each document alone, so that the test does not depend on the CPUs.
	for _, bd := range b.docs {
		c := b.countAlone(context.Background(), bd, b.fc)
		if want := bd.path != "c2.xhtml"; (c.count >= 0) != want {
			t.Errorf("%s: got the count %d, want a count %v", bd.path, c.count, want)
		}
	}
}
