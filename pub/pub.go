// Package pub opens the publications of Microsoft Publisher, .pub files,
// for their previews.
//
// A publication is a Compound File, which the package cfb of magic reads.
// The package gives its thumbnail and its properties, from the stream
// "\x05SummaryInformation", and no pages (D5). It draws a thumbnail in EMF
// or WMF with internal/metafile.
package pub

import (
	"context"
	"fmt"
	"iter"

	"github.com/xo/ivy"
	"github.com/xo/ivy/internal/ebook"
	"github.com/xo/ivy/internal/msole"
)

// Format describes the package for the registry of a program.
var Format = ivy.Format{
	Name:  "pub",
	Mimes: []string{"application/vnd.ms-publisher", "application/x-mspublisher"},
	Exts:  []string{"pub"},
	Open: func(ctx context.Context, src ivy.Source, cfg ivy.Config) (ivy.Document, error) {
		return Open(ctx, src, WithConfig(cfg))
	},
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

// Document is an open Publisher file.
type Document struct {
	cfg     ivy.Config
	summary msole.Summary
}

// The capabilities of a document.
var (
	_ ivy.Previewer = (*Document)(nil)
	_ ivy.Describer = (*Document)(nil)
)

// Open opens a Publisher file. It returns an error that wraps ivy.ErrFormat
// if src is not a Compound File with the storage Quill or the stream
// Contents of Publisher.
func Open(ctx context.Context, src ivy.Source, opts ...Option) (d *Document, err error) {
	var o options
	for _, opt := range opts {
		opt(&o)
	}
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("opening publisher file: %w", err)
	}
	defer ebook.Recover("opening publisher file", &err)
	cfg := o.cfg
	cfg.Limits = cfg.Limits.WithDefaults()
	f, err := msole.Open(src, cfg.Limits)
	if err != nil {
		return nil, err
	}
	if _, err := f.Lookup("Quill"); err != nil && !msole.Has(f, "Contents") {
		return nil, fmt.Errorf("compound file without the storage Quill of publisher: %w", ivy.ErrFormat)
	}
	summary, err := msole.ReadSummary(f, cfg.Limits)
	if err != nil {
		return nil, err
	}
	return &Document{cfg: cfg, summary: summary}, nil
}

// Close satisfies the ivy.Document interface.
func (d *Document) Close() error {
	return nil
}

// Previews satisfies the ivy.Previewer interface. It lists the thumbnail of
// the summary information.
func (d *Document) Previews(ctx context.Context) iter.Seq2[ivy.Preview, error] {
	return d.summary.Previews(ctx, d.cfg)
}

// Describe satisfies the ivy.Describer interface. It returns the
// properties of the summary information.
func (d *Document) Describe(ctx context.Context) (ivy.Info, error) {
	if err := ctx.Err(); err != nil {
		return ivy.Info{}, fmt.Errorf("describing publisher file: %w", err)
	}
	return d.summary.Info, nil
}
