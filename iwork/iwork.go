// Package iwork opens the files of Apple Pages, Keynote and Numbers.
//
// A document gives the previews that iWork writes into the file: a JPEG in
// QuickLook/ for iWork 2009, and preview.jpg and its smaller forms in the root
// for iWork 2013 and later (D11). ivy renders no page of iWork. A package that
// is a folder, not a zip file, is the work of the program that opens it.
package iwork

import (
	"context"
	"fmt"
	"iter"

	"github.com/xo/ivy"
	"github.com/xo/ivy/internal/zipdoc"
)

// Format describes the package for the registry of a program.
var Format = ivy.Format{
	Name: "iwork",
	Mimes: []string{
		"application/vnd.apple.pages",
		"application/vnd.apple.keynote",
		"application/vnd.apple.numbers",
		"application/x-iwork-pages-sffpages",
		"application/x-iwork-keynote-sffkey",
		"application/x-iwork-numbers-sffnumbers",
	},
	Exts: []string{"pages", "key", "numbers"},
	Open: func(ctx context.Context, src ivy.Source, cfg ivy.Config) (ivy.Document, error) {
		return Open(ctx, src, WithConfig(cfg))
	},
}

// previews are the entries that can hold a preview, largest first.
var previews = []zipdoc.Candidate{
	{Kind: ivy.Thumbnail, Name: "preview.jpg"},
	{Kind: ivy.Thumbnail, Name: "QuickLook/Thumbnail.jpg"},
	{Kind: ivy.Thumbnail, Name: "preview-web.jpg"},
	{Kind: ivy.Thumbnail, Name: "preview-micro.jpg"},
}

// Option is an option of Open.
type Option func(*options)

type options struct {
	cfg ivy.Config
}

// WithConfig sets the configuration that every format shares.
func WithConfig(cfg ivy.Config) Option {
	return func(o *options) {
		o.cfg = cfg
	}
}

// Document is an open iWork file.
type Document struct {
	archive *zipdoc.Archive
}

// The capabilities of a document.
var _ ivy.Previewer = (*Document)(nil)

// Open opens an iWork file. It returns an error that wraps ivy.ErrFormat if
// src is not a zip archive.
func Open(ctx context.Context, src ivy.Source, opts ...Option) (*Document, error) {
	var o options
	for _, opt := range opts {
		opt(&o)
	}
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("opening iwork: %w", err)
	}
	archive, err := zipdoc.Open(src, o.cfg.Limits)
	if err != nil {
		return nil, err
	}
	return &Document{archive: archive}, nil
}

// Close satisfies the ivy.Document interface.
func (d *Document) Close() error {
	return nil
}

// Previews satisfies the ivy.Previewer interface.
func (d *Document) Previews(ctx context.Context) iter.Seq2[ivy.Preview, error] {
	return d.archive.Previews(ctx, previews...)
}
