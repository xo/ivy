// Copyright (c) The Hayro Authors.
//
// Use of this source code is governed by the Apache-2.0 or MIT license that
// can be found in the LICENSE-APACHE and LICENSE-MIT files.

// This file ports the tests of hayro-interpret src/color/mod.rs.

package interpret

import (
	"fmt"
	"testing"

	"github.com/xo/ivy/pdf/internal/syntax"
)

// minimalPdf returns a PDF file with an empty page tree and the objects
// numbered from 10.
func minimalPdf(t testing.TB, bodies []string) *syntax.Pdf {
	t.Helper()
	data := []byte("%PDF-1.7\n1 0 obj\n<< /Type /Catalog /Pages 2 0 R >>\nendobj\n" +
		"2 0 obj\n<< /Type /Pages /Kids [] /Count 0 >>\nendobj\n")
	for i, body := range bodies {
		data = fmt.Appendf(data, "%d 0 obj\n%s\nendobj\n", 10+i, body)
	}
	data = append(data, "trailer\n<< /Root 1 0 R >>\n%%EOF\n"...)
	pdf, err := syntax.NewPdf(data)
	if err != nil {
		t.Fatal(err)
	}
	return pdf
}

// indexedChain is indexed_chain in the tests of hayro-interpret
// src/color/mod.rs.
func indexedChain(t testing.TB, n int) *syntax.Pdf {
	bodies := make([]string, n)
	for i := range n {
		if i+1 == n {
			bodies[i] = "/DeviceGray"
		} else {
			bodies[i] = fmt.Sprintf("[/Indexed %d 0 R 0 <00>]", 10+i+1)
		}
	}
	return minimalPdf(t, bodies)
}

// colorSpaceOfChain is color_space_of_chain in the tests of
// hayro-interpret src/color/mod.rs.
func colorSpaceOfChain(t *testing.T, n int) bool {
	pdf := indexedChain(t, n)
	obj, ok := pdf.XRef().Get[syntax.Object](syntax.NewObjectIdentifier(10, 0))
	if !ok {
		t.Fatal("no object 10")
	}
	_, ok = newColorSpace(obj, newCache())
	return ok
}

func TestColorSpaceNestingBound(t *testing.T) {
	if !colorSpaceOfChain(t, maxColorSpaceDepth) {
		t.Error("chain at the bound fails")
	}
	if colorSpaceOfChain(t, maxColorSpaceDepth+1) {
		t.Error("chain past the bound succeeds")
	}
}

func TestDeepColorSpaceChainFails(t *testing.T) {
	if colorSpaceOfChain(t, 20_000) {
		t.Error("deep chain succeeds")
	}
}
