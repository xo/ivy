// Package ooxml opens the Office Open XML packages of Word, Excel,
// PowerPoint and Visio, for the packages docx, xlsx, pptx and vsdx.
//
// It finds the thumbnail of a package, in docProps/thumbnail.*. A thumbnail
// in EMF or WMF is drawn with internal/metafile. It reads the core
// properties of a package, in docProps/core.xml, and the extended
// properties, in docProps/app.xml. It opens a package that a
// password protects with internal/offcrypto (D12, D24).
package ooxml

import (
	"bytes"
	"context"
	"fmt"
	"iter"

	"github.com/xo/ivy"
	"github.com/xo/ivy/internal/msole"
	"github.com/xo/ivy/internal/zipdoc"
)

// cfbSignature starts a Compound File Binary file. An Office Open XML package
// that a password protects is such a file (D12).
var cfbSignature = []byte{0xd0, 0xcf, 0x11, 0xe0, 0xa1, 0xb1, 0x1a, 0xe1}

// thumbnails are the names that a package uses for a thumbnail in a raster
// image.
var thumbnails = []string{
	"docProps/thumbnail.jpeg",
	"docProps/thumbnail.jpg",
	"docProps/thumbnail.png",
	"docProps/thumbnail.gif",
}

// metafileThumbnails are the names that a package uses for a thumbnail in a
// metafile. Visio writes one in EMF.
var metafileThumbnails = []string{
	"docProps/thumbnail.emf",
	"docProps/thumbnail.wmf",
}

// Document is an open Office Open XML package.
type Document struct {
	archive *zipdoc.Archive
	cfg     ivy.Config
}

// Open opens the package in src. It returns an error that wraps
// ivy.ErrFormat if src is not a package. If a password protects the
// package, Open asks for the password as D12 says, and returns an error
// that wraps ivy.ErrPassword if no password is right.
func Open(ctx context.Context, src ivy.Source, cfg ivy.Config) (*Document, error) {
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("opening office open xml: %w", err)
	}
	if isCompoundFile(src) {
		plain, err := decrypt(ctx, src, cfg)
		if err != nil {
			return nil, err
		}
		src = bytes.NewReader(plain)
	}
	archive, err := zipdoc.Open(src, cfg.Limits)
	if err != nil {
		return nil, err
	}
	if archive.Find("[Content_Types].xml") == nil {
		return nil, fmt.Errorf("zip archive without [Content_Types].xml: %w", ivy.ErrFormat)
	}
	return &Document{archive: archive, cfg: cfg}, nil
}

// Previews lists the thumbnails: first one in a raster image, then one in a
// metafile. A metafile that the package cannot read is left out, and
// logged.
func (d *Document) Previews(ctx context.Context) iter.Seq2[ivy.Preview, error] {
	candidates := make([]zipdoc.Candidate, len(thumbnails))
	for i, name := range thumbnails {
		candidates[i] = zipdoc.Candidate{Kind: ivy.Thumbnail, Name: name}
	}
	return func(yield func(ivy.Preview, error) bool) {
		for p, err := range d.archive.Previews(ctx, candidates...) {
			if !yield(p, err) || err != nil {
				return
			}
		}
		for _, name := range metafileThumbnails {
			if err := ctx.Err(); err != nil {
				yield(ivy.Preview{}, fmt.Errorf("listing the previews: %w", err))
				return
			}
			f := d.archive.Find(name)
			if f == nil {
				continue
			}
			data, err := d.archive.Read(f)
			if err == nil {
				var p ivy.Preview
				if p, err = msole.MetafilePreview(data, d.cfg); err == nil {
					if !yield(p, nil) {
						return
					}
					continue
				}
			}
			d.cfg.Log().Debug("ooxml: thumbnail left out", "name", name, "err", err)
		}
	}
}
