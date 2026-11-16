// Copyright (c) The Hayro Authors
//
// Use of this source code is governed by the Apache-2.0 or MIT license that
// can be found in the LICENSE-APACHE and LICENSE-MIT files.

// This file ports hayro-syntax src/lib.rs.

// Package syntax is a port of the crate hayro-syntax, a library that reads
// PDF files at a low level. It implements the chapter Syntax of the PDF
// reference: the xref tables in each form, the repair of a broken xref
// table, the objects, the object streams, the streams and their filters, the
// encryption, the pages, and the operators of content streams.
//
// The package does not read fonts or color spaces. Other packages build on
// it, such as the port of hayro-interpret.
//
// The input is untrusted. A malformed file never makes the package panic.
// The options of NewPdfWithOptions bound the memory of each decoded stream
// and image.
//
// This example reads a PDF file and prints the operators of each page:
//
//	pdf, err := syntax.NewPdf(data)
//	if err != nil {
//		return err
//	}
//	for _, page := range pdf.Pages().Slice() {
//		ops := page.TypedOperations()
//		for {
//			op, ok := ops.Next()
//			if !ok {
//				break
//			}
//			fmt.Println(op)
//		}
//	}
package syntax
