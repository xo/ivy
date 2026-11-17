// Copyright (c) The Hayro Authors
//
// Use of this source code is governed by the Apache-2.0 or MIT license that
// can be found in the LICENSE-APACHE and LICENSE-MIT files.

// This file ports hayro-syntax src/pdf.rs.

package syntax

import (
	"iter"

	"github.com/xo/ivy"
)

// Pdf is Pdf in hayro-syntax src/pdf.rs. It is a PDF file.
type Pdf struct {
	xref          *XRef
	headerVersion PdfVersion
	pages         *Pages
	data          []byte
}

// Options holds the options of NewPdfWithOptions.
//
// Port: upstream has no options. ivy bounds the work on each file, and it
// fills the decoders of JBIG2 and JPX from outside.
type Options struct {
	// Password opens a file that a password protects.
	Password string
	// Limits bound the work. MaxMemory bounds each decoded stream, and
	// MaxPixels bounds each decoded image. A zero field takes the default
	// of ivy.
	Limits ivy.Limits
	// Decoders holds the decoders of JBIG2 and JPX. A nil decoder makes its
	// filter fail with ivy.ErrUnsupported.
	Decoders ImageDecoders
}

// LoadPdfError is LoadPdfError in hayro-syntax src/pdf.rs. It is an error of
// the loading of a PDF file.
//
// Port: the Rust enum becomes a struct. Invalid selects the case Invalid,
// which wraps ivy.ErrFormat. Otherwise Decryption holds the case
// Decryption.
type LoadPdfError struct {
	// Decryption is the error of the case Decryption.
	Decryption DecryptionError
	// Invalid is true for the case Invalid: the file is not valid, or the
	// package cannot read it.
	Invalid bool
}

// Error returns the message of the error.
func (e *LoadPdfError) Error() string {
	if e.Invalid {
		return "reading pdf: " + ivy.ErrFormat.Error()
	}
	return "reading pdf: " + e.Decryption.Error()
}

// Unwrap returns the cause.
func (e *LoadPdfError) Unwrap() error {
	if e.Invalid {
		return ivy.ErrFormat
	}
	return e.Decryption
}

// NewPdf is Pdf::new in hayro-syntax src/pdf.rs. It reads a PDF file.
func NewPdf(data []byte) (*Pdf, error) {
	return NewPdfWithOptions(data, Options{})
}

// NewPdfWithPassword is Pdf::new_with_password in hayro-syntax src/pdf.rs.
// It reads a PDF file with a password.
func NewPdfWithPassword(data []byte, password string) (*Pdf, error) {
	return NewPdfWithOptions(data, Options{Password: password})
}

// NewPdfWithOptions reads a PDF file with the options. It is
// Pdf::new_with_password in hayro-syntax src/pdf.rs, with the limits and the
// decoders of ivy.
func NewPdfWithOptions(data []byte, opts Options) (*Pdf, error) {
	lim := opts.Limits.WithDefaults()
	env := &decodeEnv{maxMemory: lim.MaxMemory, maxPixels: lim.MaxPixels, decoders: opts.Decoders}
	password := []byte(opts.Password)
	version, ok := findVersion(data)
	if !ok {
		version = Pdf10
	}
	xref, xerr := rootXRef(data, password, env)
	if xerr != nil {
		if xerr.decryption != 0 {
			return nil, &LoadPdfError{Decryption: xerr.decryption}
		}
		xref, ok = fallback(data, password, env)
		if !ok {
			return nil, &LoadPdfError{Invalid: true}
		}
	}
	pages, ok := cachedPages(xref)
	if !ok {
		return nil, &LoadPdfError{Invalid: true}
	}
	return &Pdf{xref: xref, headerVersion: version, pages: pages, data: data}, nil
}

// Len is Pdf::len in hayro-syntax src/pdf.rs. It returns the number of
// objects in the xref table.
func (p *Pdf) Len() int {
	return p.xref.len()
}

// Objects is Pdf::objects in hayro-syntax src/pdf.rs. It returns each object
// of the file that can be read.
func (p *Pdf) Objects() iter.Seq[Object] {
	return p.xref.objects()
}

// Version is Pdf::version in hayro-syntax src/pdf.rs. It returns the version
// of the catalog, or else of the header.
func (p *Pdf) Version() PdfVersion {
	if v := p.xref.trailerData.version; v != nil {
		return *v
	}
	return p.headerVersion
}

// Data is Pdf::data in hayro-syntax src/pdf.rs. It returns the bytes of the
// file.
func (p *Pdf) Data() []byte {
	return p.data
}

// Pages is Pdf::pages in hayro-syntax src/pdf.rs.
func (p *Pdf) Pages() *Pages {
	return p.pages
}

// XRef is Pdf::xref in hayro-syntax src/pdf.rs.
func (p *Pdf) XRef() *XRef {
	return p.xref
}

// Metadata is Pdf::metadata in hayro-syntax src/pdf.rs. It returns the
// metadata of the document information dictionary.
func (p *Pdf) Metadata() *Metadata {
	return &p.xref.metadata
}

// findVersion is find_version in hayro-syntax src/pdf.rs. It reads the
// version of the header in the first 2000 bytes.
func findVersion(data []byte) (PdfVersion, bool) {
	r := NewReader(data[:min(len(data), 2000)])
	for !r.ForwardTag("%PDF-") {
		if _, ok := r.ReadU8(); !ok {
			return 0, false
		}
	}
	tail, ok := r.Tail()
	if !ok {
		return 0, false
	}
	return pdfVersionFromBytes(tail)
}

// PdfVersion is PdfVersion in hayro-syntax src/pdf.rs. It is the version of
// a PDF document.
type PdfVersion uint8

// The versions of hayro-syntax src/pdf.rs.
const (
	// Pdf10 is PDF 1.0.
	Pdf10 PdfVersion = iota
	// Pdf11 is PDF 1.1.
	Pdf11
	// Pdf12 is PDF 1.2.
	Pdf12
	// Pdf13 is PDF 1.3.
	Pdf13
	// Pdf14 is PDF 1.4.
	Pdf14
	// Pdf15 is PDF 1.5.
	Pdf15
	// Pdf16 is PDF 1.6.
	Pdf16
	// Pdf17 is PDF 1.7.
	Pdf17
	// Pdf20 is PDF 2.0.
	Pdf20
)

// pdfVersionFromBytes is PdfVersion::from_bytes in hayro-syntax src/pdf.rs.
func pdfVersionFromBytes(b []byte) (PdfVersion, bool) {
	if len(b) < 3 {
		return 0, false
	}
	switch string(b[:3]) {
	case "1.0":
		return Pdf10, true
	case "1.1":
		return Pdf11, true
	case "1.2":
		return Pdf12, true
	case "1.3":
		return Pdf13, true
	case "1.4":
		return Pdf14, true
	case "1.5":
		return Pdf15, true
	case "1.6":
		return Pdf16, true
	case "1.7":
		return Pdf17, true
	case "2.0":
		return Pdf20, true
	}
	return 0, false
}
