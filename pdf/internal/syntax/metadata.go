// Copyright (c) The Hayro Authors
//
// Use of this source code is governed by the Apache-2.0 or MIT license that
// can be found in the LICENSE-APACHE and LICENSE-MIT files.

// This file ports hayro-syntax src/metadata.rs.

package syntax

// Metadata is Metadata in hayro-syntax src/metadata.rs. It holds the
// metadata of a PDF document. A nil field is absent.
type Metadata struct {
	// CreationDate is the date when the document was made.
	CreationDate *DateTime
	// ModificationDate is the date when the document last changed.
	ModificationDate *DateTime
	// Title is the title of the document.
	Title []byte
	// Author is the author of the document.
	Author []byte
	// Subject is the subject of the document.
	Subject []byte
	// Keywords holds the keywords of the document.
	Keywords []byte
	// Creator is the program that made the document.
	Creator []byte
	// Producer is the program that converted the document to PDF.
	Producer []byte
}
