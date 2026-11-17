// Copyright (c) The Hayro Authors
//
// Use of this source code is governed by the Apache-2.0 or MIT license that
// can be found in the LICENSE-APACHE and LICENSE-MIT files.

// This file ports the tests of hayro-syntax src/pdf.rs, and the example of
// src/lib.rs.

package syntax

import (
	"errors"
	"os"
	"testing"

	"github.com/xo/ivy"
)

// readTestPDF reads a file of testdata.
func readTestPDF(t *testing.T, name string) []byte {
	t.Helper()
	data, err := os.ReadFile("testdata/" + name)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func TestIssue49(t *testing.T) {
	_, err := NewPdf(nil)
	if !errors.Is(err, ivy.ErrFormat) {
		t.Fatalf("got %v", err)
	}
}

func TestPdfVersionHeader(t *testing.T) {
	pdf, err := NewPdf(readTestPDF(t, "version_header.pdf"))
	if err != nil {
		t.Fatal(err)
	}
	if pdf.Version() != Pdf17 {
		t.Fatalf("got %d", pdf.Version())
	}
}

func TestPdfVersionCatalog(t *testing.T) {
	pdf, err := NewPdf(readTestPDF(t, "version_catalog.pdf"))
	if err != nil {
		t.Fatal(err)
	}
	if pdf.Version() != Pdf17 {
		t.Fatalf("got %d", pdf.Version())
	}
}

func TestXRefStreamInEncryptedPdf(t *testing.T) {
	pdf, err := NewPdf(readTestPDF(t, "catalog-in-objstm-aes.pdf"))
	if err != nil {
		t.Fatal(err)
	}
	stream, ok := pdf.XRef().Get[Stream](NewObjectIdentifier(13, 0))
	if !ok {
		t.Fatal("no stream 13")
	}
	data, err := stream.Decoded()
	if err != nil || len(data) != 98 {
		t.Fatalf("got %d bytes, %v", len(data), err)
	}
}

// TestIterate follows the example of src/lib.rs.
func TestIterate(t *testing.T) {
	pdf, err := NewPdf(readTestPDF(t, "text_with_rise.pdf"))
	if err != nil {
		t.Fatal(err)
	}
	if pdf.Pages().Len() == 0 {
		t.Fatal("no pages")
	}
	n := 0
	for _, page := range pdf.Pages().Slice() {
		ops := page.TypedOperations()
		for {
			op, ok := ops.Next()
			if !ok {
				break
			}
			if _, isFallback := op.(Fallback); isFallback {
				t.Errorf("unknown operator %s", op)
			}
			n++
		}
	}
	if n == 0 {
		t.Fatal("no operators")
	}
}
