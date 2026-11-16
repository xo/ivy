// Copyright (c) The Hayro Authors
//
// Use of this source code is governed by the Apache-2.0 or MIT license that
// can be found in the LICENSE-APACHE and LICENSE-MIT files.

package jbig2

// This file ports hayro-jbig2 tests/main.rs. The tests decode the JBIG2
// files in testdata/streams and compare them with the images in
// testdata/references.

import (
	"bytes"
	"errors"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"testing"
)

// testBitmap is Bitmap in hayro-jbig2 tests/main.rs. A value of true is
// black.
type testBitmap [][]bool

// loadReference is load_reference in hayro-jbig2 tests/main.rs.
func loadReference(t *testing.T, name string) testBitmap {
	t.Helper()
	path := filepath.Join("testdata", "references", name+".png")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to load %s: %v", path, err)
	}
	img, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("failed to load %s: %v", path, err)
	}

	b := img.Bounds()
	bm := make(testBitmap, b.Dy())
	for y := range bm {
		bm[y] = make([]bool, b.Dx())
		for x := range bm[y] {
			g := color.GrayModel.Convert(img.At(b.Min.X+x, b.Min.Y+y)).(color.Gray)
			bm[y][x] = g.Y < 128
		}
	}
	return bm
}

// decodeTestPage is decode in hayro-jbig2 tests/main.rs.
func decodeTestPage(page *Page) (testBitmap, error) {
	pb, err := page.Decode()
	if err != nil {
		return nil, err
	}
	width := int(pb.Width())

	var bm testBitmap
	for row := range pb.Rows() {
		r := make([]bool, width)
		for x := range r {
			r[x] = row[x/32]&(0x8000_0000>>(x%32)) != 0
		}
		bm = append(bm, r)
	}
	return bm, nil
}

// check is check in hayro-jbig2 tests/main.rs.
func check(t *testing.T, stream, reference string) {
	t.Helper()
	compare(t, stream, loadReference(t, reference), true)
}

// checkWithMemoryLimit is check_with_memory_limit in hayro-jbig2
// tests/main.rs.
func checkWithMemoryLimit(t *testing.T, stream, reference string, memoryLimit int) {
	t.Helper()
	compareWith(t, stream, loadReference(t, reference), DecodeSettings{MemoryLimit: memoryLimit})
}

// checkMemoryLimitExceeded is check_memory_limit_exceeded in hayro-jbig2
// tests/main.rs.
func checkMemoryLimitExceeded(t *testing.T, stream string, memoryLimit int) {
	t.Helper()
	data := readStream(t, stream)
	file, err := NewFile(data, DecodeSettings{MemoryLimit: memoryLimit})
	if err != nil {
		t.Fatalf("%s: parsing failed: %v", stream, err)
	}
	page := file.Page(0)
	if page == nil {
		t.Fatalf("%s: no pages", stream)
	}

	if _, err := page.Decode(); !errors.Is(err, ErrMemoryLimitExceeded) {
		t.Fatalf("%s: memory limit not exceeded: %v", stream, err)
	}
}

// checkLenient is check_lenient in hayro-jbig2 tests/main.rs.
func checkLenient(t *testing.T, stream, reference string) {
	t.Helper()
	compare(t, stream, loadReference(t, reference), false)
}

// checkPages is check_pages in hayro-jbig2 tests/main.rs.
func checkPages(t *testing.T, stream string, references []string) {
	t.Helper()
	comparePages(t, stream, references, true)
}

// checkPagesLenient is check_pages_lenient in hayro-jbig2 tests/main.rs.
func checkPagesLenient(t *testing.T, stream string, references []string) {
	t.Helper()
	comparePages(t, stream, references, false)
}

// comparePages is compare_pages in hayro-jbig2 tests/main.rs.
func comparePages(t *testing.T, stream string, references []string, strict bool) {
	t.Helper()
	data := readStream(t, stream)
	file, err := NewFile(data, DecodeSettings{Strict: strict})
	if err != nil {
		t.Fatalf("%s: parsing failed: %v", stream, err)
	}
	if file.PageCount() != len(references) {
		t.Fatalf("%s: wrong number of pages: %d, want %d", stream, file.PageCount(), len(references))
	}

	var previousNumber uint32
	i := 0
	for page := range file.Pages() {
		if page.Number() <= previousNumber {
			t.Fatalf("%s: page %d after page %d", stream, page.Number(), previousNumber)
		}
		previousNumber = page.Number()
		comparePage(t, stream, page, loadReference(t, references[i]))
		i++
	}
}

// checkCrop is check_crop in hayro-jbig2 tests/main.rs.
func checkCrop(t *testing.T, stream, reference string, x, y, width, height int) {
	t.Helper()
	full := loadReference(t, reference)
	crop := make(testBitmap, height)
	for i := range crop {
		crop[i] = full[y+i][x : x+width]
	}
	compare(t, stream, crop, true)
}

// compare is compare in hayro-jbig2 tests/main.rs.
func compare(t *testing.T, stream string, reference testBitmap, strict bool) {
	t.Helper()
	compareWith(t, stream, reference, DecodeSettings{Strict: strict})
}

// compareWith is compare_with in hayro-jbig2 tests/main.rs.
func compareWith(t *testing.T, stream string, reference testBitmap, settings DecodeSettings) {
	t.Helper()
	data := readStream(t, stream)
	file, err := NewFile(data, settings)
	if err != nil {
		t.Fatalf("%s: parsing failed: %v", stream, err)
	}
	page := file.Page(0)
	if page == nil {
		t.Fatalf("%s: no pages", stream)
	}
	comparePage(t, stream, page, reference)
}

// readStream is read_stream in hayro-jbig2 tests/main.rs.
func readStream(t testing.TB, stream string) []byte {
	t.Helper()
	path := filepath.Join("testdata", "streams", stream+".jbig2")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read %s: %v", path, err)
	}
	return data
}

// comparePage is compare_page in hayro-jbig2 tests/main.rs.
func comparePage(t *testing.T, stream string, page *Page, reference testBitmap) {
	t.Helper()
	if int(page.Width()) != len(reference[0]) || int(page.Height()) != len(reference) {
		t.Fatalf("%s: wrong page size: %dx%d, want %dx%d", stream,
			page.Width(), page.Height(), len(reference[0]), len(reference))
	}

	rows, err := decodeTestPage(page)
	if err != nil {
		t.Fatalf("%s: decoding failed: %v", stream, err)
	}

	if len(rows) != len(reference) {
		t.Fatalf("%s: wrong number of rows: %d, want %d", stream, len(rows), len(reference))
	}

	for y := range rows {
		for x := range rows[y] {
			if rows[y][x] != reference[y][x] {
				t.Fatalf("%s: first mismatch at row %d, column %d", stream, y, x)
			}
		}
	}
}

// checkMalformed is check_malformed in hayro-jbig2 tests/main.rs. A
// malformed file must decode or fail without a panic.
func checkMalformed(t *testing.T, stream string) {
	t.Helper()
	data := readStream(t, stream)

	file, err := NewFile(data, DecodeSettings{})
	if err != nil {
		return
	}
	for page := range file.Pages() {
		_, _ = decodeTestPage(page)
	}
}
