// Package pptx opens PowerPoint files in Office Open XML: pptx, pptm, potx,
// ppsx and ppsm.
//
// A document gives the thumbnail that the file can hold, in
// docProps/thumbnail.* (D11), and its core properties. It gives one page for
// each slide that is not hidden, at the size of the slides. The page draws
// the background, the shapes of the master and of the layout, and the
// shapes of the slide onto the display list of D10, with the geometry, the
// fills, the lines and the text of internal/drawingml. A placeholder takes
// what it does not set from the placeholder of the layout and of the
// master. The text goes through the layout of the ebooks. internal/chart
// draws the charts (D27), with the theme of the slide and the
// fonts of ivy.Config. The notes are never drawn.
//
// A file that a password protects opens with the password of ivy.Config, or
// with its PasswordFunc (D12).
package pptx

import (
	"context"
	"fmt"
	"iter"
	"sync"

	"github.com/xo/ivy"
	"github.com/xo/ivy/internal/ooxml"
)

// Format describes the package for the registry of a program.
var Format = ivy.Format{
	Name: "pptx",
	Mimes: []string{
		"application/vnd.openxmlformats-officedocument.presentationml.presentation",
		"application/vnd.openxmlformats-officedocument.presentationml.template",
		"application/vnd.openxmlformats-officedocument.presentationml.slideshow",
		"application/vnd.ms-powerpoint.presentation.macroEnabled.12",
		"application/vnd.ms-powerpoint.slideshow.macroEnabled.12",
	},
	Exts: []string{"pptx", "potx", "ppsx", "pptm", "ppsm"},
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

// Document is an open PowerPoint file. It is safe for concurrent use.
type Document struct {
	doc *ooxml.Document
	cfg ivy.Config
	lim ivy.Limits

	once sync.Once
	pres *presentation
	err  error
}

// The capabilities of a document.
var (
	_ ivy.Previewer = (*Document)(nil)
	_ ivy.Pager     = (*Document)(nil)
	_ ivy.Describer = (*Document)(nil)
)

// Open opens a PowerPoint file. It returns an error that wraps ivy.ErrFormat
// if src is not an Office Open XML package, and one that wraps
// ivy.ErrPassword if a password protects it and the configuration does not
// give the right one.
func Open(ctx context.Context, src ivy.Source, opts ...Option) (*Document, error) {
	var o options
	for _, opt := range opts {
		opt(&o)
	}
	doc, err := ooxml.Open(ctx, src, o.cfg)
	if err != nil {
		return nil, err
	}
	return &Document{doc: doc, cfg: o.cfg, lim: o.cfg.Limits.WithDefaults()}, nil
}

// Close satisfies the ivy.Document interface.
func (d *Document) Close() error {
	return nil
}

// Previews satisfies the ivy.Previewer interface. It lists the thumbnail.
func (d *Document) Previews(ctx context.Context) iter.Seq2[ivy.Preview, error] {
	return d.doc.Previews(ctx)
}

// Describe satisfies the ivy.Describer interface. It reads the core
// properties of the file, in docProps/core.xml, and the program that made
// it, in docProps/app.xml.
func (d *Document) Describe(ctx context.Context) (ivy.Info, error) {
	return d.doc.Describe(ctx)
}

// presentation returns the presentation of the file, which the first call
// reads.
func (d *Document) presentation(ctx context.Context) (*presentation, error) {
	d.once.Do(func() {
		defer func() {
			if r := recover(); r != nil {
				d.pres, d.err = nil, fmt.Errorf("reading the presentation: panic: %v: %w", r, ivy.ErrFormat)
			}
		}()
		d.pres, d.err = readPresentation(ctx, d)
	})
	return d.pres, d.err
}

// PageCount satisfies the ivy.Pager interface. It counts the slides that are
// not hidden.
func (d *Document) PageCount(ctx context.Context) (int, error) {
	if err := ctx.Err(); err != nil {
		return 0, fmt.Errorf("counting the slides: %w", err)
	}
	p, err := d.presentation(ctx)
	if err != nil {
		return 0, err
	}
	return len(p.slides), nil
}

// Page satisfies the ivy.Pager interface. n counts the slides that are not
// hidden, from 0.
func (d *Document) Page(ctx context.Context, n int) (page ivy.Page, err error) {
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("reading slide %d: %w", n+1, err)
	}
	p, err := d.presentation(ctx)
	if err != nil {
		return nil, err
	}
	if n < 0 || n >= len(p.slides) {
		return nil, fmt.Errorf("slide %d of %d: %w", n+1, len(p.slides), ivy.ErrPageRange)
	}
	defer func() {
		if r := recover(); r != nil {
			page, err = nil, fmt.Errorf("reading slide %d: panic: %v: %w", n+1, r, ivy.ErrFormat)
		}
	}()
	return p.page(ctx, n)
}
