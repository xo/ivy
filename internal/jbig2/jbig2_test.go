// Copyright (c) The Hayro Authors
//
// Use of this source code is governed by the Apache-2.0 or MIT license that
// can be found in the LICENSE-APACHE and LICENSE-MIT files.

package jbig2

// This file is not a port. It tests the parts of the package that the PDF
// filter JBIG2Decode uses, and it holds the fuzz tests.

import (
	"bytes"
	"errors"
	"image"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/xo/ivy"
)

// splitEmbedded splits a JBIG2 file of sequential organization into the
// global segments and the segments of its first page, as a PDF file holds
// them in the streams JBIG2Globals and JBIG2Decode.
func splitEmbedded(t *testing.T, data []byte) (globals, page []byte) {
	t.Helper()
	r := newReader(data)
	h, err := parseFileHeader(&r)
	if err != nil {
		t.Fatal(err)
	}
	if h.organization != organizationSequential {
		t.Fatal("the file is not sequential")
	}

	var firstPage uint32
	for !r.atEnd() {
		start := r.bytePos()
		seg, err := parseSegment(&r, false)
		if err != nil {
			t.Fatal(err)
		}
		raw := data[start:r.bytePos()]
		switch n := seg.header.pageAssociation; {
		case n == 0:
			globals = append(globals, raw...)
		case firstPage == 0 || n == firstPage:
			firstPage = n
			page = append(page, raw...)
		}
		if seg.header.segmentType == segmentEndOfFile {
			break
		}
	}
	return globals, page
}

// TestEmbeddedPage decodes the first page of files as embedded pages, with
// their global segments in a Globals, as the PDF filter does.
func TestEmbeddedPage(t *testing.T) {
	tests := []struct {
		stream    string
		reference string
	}{
		{"generic_template0", "truchet"},
		{"generic_mmr", "truchet"},
		{"symbol_global", "honeycomb"},
		{"halftone_global", "maze"},
		{"huffman_text_refine_global_tables", "honeycomb"},
		{"text_refine", "honeycomb"},
		{"refine_template0", "truchet"},
		{"page_striped_unknown_height", "truchet"},
	}
	for _, tt := range tests {
		t.Run(tt.stream, func(t *testing.T) {
			globalsData, pageData := splitEmbedded(t, readStream(t, tt.stream))

			var globals *Globals
			if len(globalsData) > 0 {
				var err error
				if globals, err = NewGlobals(globalsData, DecodeSettings{Strict: true}); err != nil {
					t.Fatalf("NewGlobals: %v", err)
				}
			}
			page, err := NewEmbeddedPage(pageData, globals, DecodeSettings{Strict: true})
			if err != nil {
				t.Fatalf("NewEmbeddedPage: %v", err)
			}
			comparePage(t, tt.stream, page, loadReference(t, tt.reference))
		})
	}
}

// TestDataU8Values makes sure that DataU8 and Image give black as 0 and
// white as 255, as the rows give black as 1.
func TestDataU8Values(t *testing.T) {
	file, err := NewFile(readStream(t, "generic_template0"), DecodeSettings{})
	if err != nil {
		t.Fatal(err)
	}
	page := file.Page(0)
	pb, err := page.Decode()
	if err != nil {
		t.Fatal(err)
	}
	pix, err := pb.DataU8()
	if err != nil {
		t.Fatal(err)
	}
	img, err := page.Image()
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(img.Pix, pix) {
		t.Fatal("Image().Pix differs from DataU8()")
	}

	width := int(pb.Width())
	y := 0
	for row := range pb.Rows() {
		for x := range width {
			black := row[x/32]&(0x8000_0000>>(x%32)) != 0
			want := byte(0xFF)
			if black {
				want = 0
			}
			if got := pix[y*width+x]; got != want {
				t.Fatalf("pixel %d, %d = %#x, want %#x", x, y, got, want)
			}
		}
		y++
	}
}

// TestDataU8MemoryLimit makes sure that DataU8 respects the memory limit.
func TestDataU8MemoryLimit(t *testing.T) {
	file, err := NewFile(readStream(t, "generic_template0"), DecodeSettings{})
	if err != nil {
		t.Fatal(err)
	}
	pb, err := file.Page(0).Decode()
	if err != nil {
		t.Fatal(err)
	}
	// The limit is less than the number of pixels of the page.
	pb.settings.MemoryLimit = int(pb.Width())*int(pb.Height()) - 1
	if _, err := pb.DataU8(); !errors.Is(err, ErrMemoryLimitExceeded) || !errors.Is(err, ivy.ErrLimit) {
		t.Fatalf("DataU8() error = %v, want %v", err, ErrMemoryLimitExceeded)
	}
}

// TestRegisterDecodingHook decodes a file with image.Decode.
func TestRegisterDecodingHook(t *testing.T) {
	RegisterDecodingHook()
	if RegisterDecodingHook() {
		t.Error("RegisterDecodingHook() = true for the second call, want false")
	}

	data := readStream(t, "generic_template0")
	img, format, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	if format != "jbig2" {
		t.Errorf("format = %q, want jbig2", format)
	}
	ref := loadReference(t, "truchet")
	if b := img.Bounds(); b.Dx() != len(ref[0]) || b.Dy() != len(ref) {
		t.Errorf("bounds = %v, want %dx%d", b, len(ref[0]), len(ref))
	}
}

// TestErrorSentinels makes sure that the errors of the package wrap the
// sentinel errors of ivy.
func TestErrorSentinels(t *testing.T) {
	if _, err := NewFile([]byte("not a jbig2 file"), DecodeSettings{}); !errors.Is(err, ErrInvalidHeader) || !errors.Is(err, ivy.ErrFormat) {
		t.Errorf("NewFile error = %v, want %v and %v", err, ErrInvalidHeader, ivy.ErrFormat)
	}
	if !errors.Is(ErrUnsupported, ivy.ErrUnsupported) {
		t.Error("ErrUnsupported does not wrap ivy.ErrUnsupported")
	}
	if !errors.Is(ErrOverflowBitmapDimension, ivy.ErrLimit) {
		t.Error("ErrOverflowBitmapDimension does not wrap ivy.ErrLimit")
	}
}

// fuzzSettings bound the memory of a fuzz input, so that each input decodes
// fast.
var fuzzSettings = DecodeSettings{MemoryLimit: 1 << 22}

// addStreamSeeds adds the streams of the tests to the corpus.
func addStreamSeeds(f *testing.F, add func(data []byte)) {
	f.Helper()
	paths, err := filepath.Glob(filepath.Join("testdata", "streams", "*.jbig2"))
	if err != nil {
		f.Fatal(err)
	}
	for _, p := range paths {
		data, err := os.ReadFile(p)
		if err != nil {
			f.Fatal(err)
		}
		add(data)
	}
}

// FuzzFile parses and decodes a JBIG2 file. The decoder must not panic.
func FuzzFile(f *testing.F) {
	addStreamSeeds(f, func(data []byte) { f.Add(data) })

	f.Fuzz(func(_ *testing.T, data []byte) {
		for _, strict := range []bool{false, true} {
			settings := fuzzSettings
			settings.Strict = strict
			file, err := NewFile(data, settings)
			if err != nil {
				continue
			}
			for page := range file.Pages() {
				pb, err := page.Decode()
				if err != nil {
					continue
				}
				_, _ = pb.DataU8()
			}
		}
	})
}

// FuzzEmbeddedPage decodes an embedded page with its global segments, as
// the PDF filter JBIG2Decode does. The decoder must not panic.
func FuzzEmbeddedPage(f *testing.F) {
	addStreamSeeds(f, func(data []byte) {
		// The data after the file header of a sequential file is an embedded
		// page.
		if len(data) > 13 && data[8]&1 != 0 {
			f.Add([]byte{}, data[13:])
		}
	})

	f.Fuzz(func(_ *testing.T, globalsData, pageData []byte) {
		var globals *Globals
		if len(globalsData) > 0 {
			var err error
			if globals, err = NewGlobals(globalsData, fuzzSettings); err != nil {
				return
			}
		}
		page, err := NewEmbeddedPage(pageData, globals, fuzzSettings)
		if err != nil {
			return
		}
		pb, err := page.Decode()
		if err != nil {
			return
		}
		_, _ = pb.DataU8()
	})
}

// TestErrorMessages makes sure that each error has a message of its own.
func TestErrorMessages(t *testing.T) {
	errs := []error{
		ErrMemoryLimitExceeded, ErrUnsupported, ErrCancelled,
		ErrUnexpectedEOF,
		ErrInvalidHeader, ErrInvalidEndOfPage,
		ErrUnknownSegmentType, ErrMissingPageAssociation,
		ErrHuffmanInvalidCode, ErrHuffmanConflictingCodes,
		ErrInvalidCombinationOperator, ErrGrayScaleOutOfRange,
		ErrTemplateInvalid, ErrInvalidATPixel,
		ErrNoSymbols, ErrTooManyInstances,
		ErrOverflowBitmapDimension, ErrOverflowSymbolCount,
	}
	for _, err := range errs {
		if msg := err.Error(); !strings.HasPrefix(msg, "jbig2: ") || strings.Contains(msg, "unknown error") {
			t.Errorf("message %q of %#v", msg, err)
		}
	}
}
