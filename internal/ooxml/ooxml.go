// Package ooxml opens the Office Open XML packages of Word, Excel and
// PowerPoint, for the packages docx, xlsx and pptx.
//
// On 2026-11-16 it finds the thumbnail of a package, in
// docProps/thumbnail.*. A thumbnail in EMF or WMF waits for the reader of
// phase 5.
package ooxml

import (
	"bytes"
	"context"
	"fmt"
	"iter"

	"github.com/xo/ivy"
	"github.com/xo/ivy/internal/zipdoc"
)

// cfbSignature starts a Compound File Binary file. An Office Open XML package
// that a password protects is such a file (D12).
var cfbSignature = []byte{0xd0, 0xcf, 0x11, 0xe0, 0xa1, 0xb1, 0x1a, 0xe1}

// thumbnails are the names that a package uses for its thumbnail.
var thumbnails = []string{
	"docProps/thumbnail.jpeg",
	"docProps/thumbnail.jpg",
	"docProps/thumbnail.png",
	"docProps/thumbnail.gif",
}

// Document is an open Office Open XML package.
type Document struct {
	archive *zipdoc.Archive
}

// Open opens the package in src. It returns an error that wraps
// ivy.ErrFormat if src is not a package, and ivy.ErrUnsupported if a
// password protects it, until the encryption of phase 5.
func Open(ctx context.Context, src ivy.Source, cfg ivy.Config) (*Document, error) {
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("opening office open xml: %w", err)
	}
	head := make([]byte, len(cfbSignature))
	if _, err := src.ReadAt(head, 0); err == nil && bytes.Equal(head, cfbSignature) {
		return nil, fmt.Errorf("office file that a password protects: %w", ivy.ErrUnsupported)
	}
	archive, err := zipdoc.Open(src, cfg.Limits)
	if err != nil {
		return nil, err
	}
	if archive.Find("[Content_Types].xml") == nil {
		return nil, fmt.Errorf("zip archive without [Content_Types].xml: %w", ivy.ErrFormat)
	}
	return &Document{archive: archive}, nil
}

// Previews lists the thumbnail.
func (d *Document) Previews(ctx context.Context) iter.Seq2[ivy.Preview, error] {
	candidates := make([]zipdoc.Candidate, len(thumbnails))
	for i, name := range thumbnails {
		candidates[i] = zipdoc.Candidate{Kind: ivy.Thumbnail, Name: name}
	}
	return d.archive.Previews(ctx, candidates...)
}
