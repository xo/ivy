// Package docx opens Word files in Office Open XML: docx, dotx, docm and
// dotm.
//
// A document gives the thumbnail that the file can hold, in
// docProps/thumbnail.* (D11). It gives its pages through the layout of ivy
// (D10, D15): the package converts the WordprocessingML of the file into
// one XHTML document with a style sheet, and the layout puts it on pages.
// Page lays out the document only as far as the page that a program asks
// for. The package reads the styles, the direct formatting, the lists, the
// tables, the sections, the headers and the footers, the footnotes and the
// endnotes, the images, the text boxes, and the fields with their last
// result. It shows the text after the tracked changes, and it does not draw
// the comments. A rendering is good enough for a review (D5), so a page can
// break at another place than in Word.
//
// A file that a password protects opens with the password of ivy.Config,
// or with the function PasswordFunc of ivy.Config (D12).
package docx

import (
	"context"
	"fmt"
	"iter"
	"log/slog"
	"sync"

	"github.com/xo/ivy"
	"github.com/xo/ivy/internal/ebook"
	"github.com/xo/ivy/internal/layout"
	"github.com/xo/ivy/internal/ooxml"
)

// Format describes the package for the registry of a program.
var Format = ivy.Format{
	Name: "docx",
	Mimes: []string{
		"application/vnd.openxmlformats-officedocument.wordprocessingml.document",
		"application/vnd.openxmlformats-officedocument.wordprocessingml.template",
		"application/vnd.ms-word.document.macroEnabled.12",
		"application/vnd.ms-word.template.macroEnabled.12",
	},
	Exts: []string{"docx", "dotx", "docm", "dotm"},
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

// Document is an open Word file.
type Document struct {
	doc *ooxml.Document
	cfg ivy.Config
	lim ivy.Limits
	log *slog.Logger

	mu     sync.Mutex
	src    *layout.Source
	srcErr error
	// pages holds the book of the layout.
	pages ebook.Pages
}

// The capabilities of a document.
var (
	_ ivy.Pager     = (*Document)(nil)
	_ ivy.Previewer = (*Document)(nil)
	_ ivy.Describer = (*Document)(nil)
)

// Open opens a Word file. It returns an error that wraps ivy.ErrFormat if
// src is not an Office Open XML package. If a password protects the file,
// it returns an error that wraps ivy.ErrPassword when no password is right.
// Open reads no part of the document yet.
func Open(ctx context.Context, src ivy.Source, opts ...Option) (d *Document, err error) {
	var o options
	for _, opt := range opts {
		opt(&o)
	}
	defer ebook.Recover("opening docx", &err)
	doc, err := ooxml.Open(ctx, src, o.cfg)
	if err != nil {
		return nil, err
	}
	o.cfg.Limits = o.cfg.Limits.WithDefaults()
	return &Document{doc: doc, cfg: o.cfg, lim: o.cfg.Limits, log: o.cfg.Log()}, nil
}

// Close satisfies the ivy.Document interface.
func (d *Document) Close() error {
	return nil
}

// Previews satisfies the ivy.Previewer interface. It lists the thumbnail.
func (d *Document) Previews(ctx context.Context) iter.Seq2[ivy.Preview, error] {
	return d.doc.Previews(ctx)
}

// PageCount satisfies the ivy.Pager interface. It lays out the whole
// document on its first call.
func (d *Document) PageCount(ctx context.Context) (n int, err error) {
	defer ebook.Recover("counting the pages of docx", &err)
	return d.pages.PageCount(ctx, d.source, d.cfg)
}

// Page satisfies the ivy.Pager interface. It lays out the document only as
// far as page n, from 0.
func (d *Document) Page(ctx context.Context, n int) (p ivy.Page, err error) {
	defer ebook.Recover("laying out docx", &err)
	return d.pages.Page(ctx, d.source, d.cfg, n)
}

// source converts the document into XHTML for the layout. It converts the
// document once, and returns the same result on each later call.
func (d *Document) source(ctx context.Context) (src layout.Source, err error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	switch {
	case d.srcErr != nil:
		return layout.Source{}, d.srcErr
	case d.src != nil:
		return *d.src, nil
	}
	defer func() {
		switch {
		case err == nil:
			d.src = &src
		case ctx.Err() == nil:
			// An error of the context is not kept, so that a later call
			// can convert the document.
			d.srcErr = err
		}
	}()
	// Recover runs before the function above, which then keeps its error.
	defer ebook.Recover("converting docx", &err)
	if err := ctx.Err(); err != nil {
		return layout.Source{}, fmt.Errorf("converting docx: %w", err)
	}
	return d.convert(ctx)
}

// Describe satisfies the ivy.Describer interface. It reads the core
// properties of the file, in docProps/core.xml.
func (d *Document) Describe(ctx context.Context) (info ivy.Info, err error) {
	defer ebook.Recover("describing docx", &err)
	return d.doc.Describe(ctx)
}
