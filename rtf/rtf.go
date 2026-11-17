// Package rtf opens files in the Rich Text Format, as RTF 1.9.1 of
// Microsoft describes it.
//
// The package converts the text of the file into one XHTML document with a
// style sheet, and the layout of ivy puts it on pages (D10, D15). Page lays
// out the document only as far as the page that a program asks for. The
// package reads the fonts, the colors, the formatting of the characters and
// of the paragraphs, the tables, the pictures in PNG, JPEG, EMF and WMF,
// the sections with their pages and margins, the headers and the footers,
// the footnotes, the fields with their last result, and the text in each
// code page of the Encoding Standard and in Unicode. The tables have their
// merged cells and the tables that nest in their cells. A paragraph and a
// run take the formatting of their styles of the style sheet first. A run
// gives its East Asian font of \dbch after its font, and the language of
// its East Asian text. The WordArt of a header, such as a watermark,
// shows on each page. A rendering is good enough for a review (D5).
package rtf

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log/slog"
	"sync"

	"github.com/xo/ivy"
	"github.com/xo/ivy/internal/ebook"
	"github.com/xo/ivy/internal/layout"
)

// Format describes the package for the registry of a program.
var Format = ivy.Format{
	Name:  "rtf",
	Mimes: []string{"application/rtf", "text/rtf"},
	Exts:  []string{"rtf"},
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

// Document is an open RTF file.
type Document struct {
	data []byte
	cfg  ivy.Config
	lim  ivy.Limits
	log  *slog.Logger

	mu     sync.Mutex
	src    *layout.Source
	info   ivy.Info
	srcErr error
	pages  ebook.Pages
}

// The capabilities of a document.
var (
	_ ivy.Pager     = (*Document)(nil)
	_ ivy.Describer = (*Document)(nil)
)

// Open opens an RTF file. It reads the file, and converts it on the first
// call that needs its text. It returns an error that wraps ivy.ErrFormat if
// src is not an RTF file, and one that wraps ivy.ErrLimit if the file is
// larger than Limits.MaxMemory.
func Open(ctx context.Context, src ivy.Source, opts ...Option) (d *Document, err error) {
	var o options
	for _, opt := range opts {
		opt(&o)
	}
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("opening rtf: %w", err)
	}
	defer ebook.Recover("opening rtf", &err)
	lim := o.cfg.Limits.WithDefaults()
	if src.Size() > lim.MaxMemory {
		return nil, fmt.Errorf("rtf file of %d bytes: %w", src.Size(), ivy.ErrLimit)
	}
	data, err := io.ReadAll(io.NewSectionReader(src, 0, src.Size()))
	if err != nil {
		return nil, fmt.Errorf("reading rtf: %w", err)
	}
	// A byte order mark of UTF-8 can come first.
	body := bytes.TrimPrefix(data, []byte("\xef\xbb\xbf"))
	body = bytes.TrimLeft(body, " \t\r\n")
	if !bytes.HasPrefix(body, []byte(`{\rtf`)) {
		return nil, fmt.Errorf("file without {\\rtf: %w", ivy.ErrFormat)
	}
	o.cfg.Limits = lim
	return &Document{data: body, cfg: o.cfg, lim: lim, log: o.cfg.Log()}, nil
}

// Close satisfies the ivy.Document interface.
func (d *Document) Close() error {
	return nil
}

// PageCount satisfies the ivy.Pager interface. It lays out the whole
// document on its first call.
func (d *Document) PageCount(ctx context.Context) (n int, err error) {
	defer ebook.Recover("counting the pages of rtf", &err)
	return d.pages.PageCount(ctx, d.source, d.cfg)
}

// Page satisfies the ivy.Pager interface. It lays out the document only as
// far as page n, from 0.
func (d *Document) Page(ctx context.Context, n int) (p ivy.Page, err error) {
	defer ebook.Recover("laying out rtf", &err)
	return d.pages.Page(ctx, d.source, d.cfg, n)
}

// Describe satisfies the ivy.Describer interface. It reads the group \info
// of the file: the title, the author, the subject and the dates.
func (d *Document) Describe(ctx context.Context) (info ivy.Info, err error) {
	defer ebook.Recover("describing rtf", &err)
	if _, err := d.source(ctx); err != nil {
		return ivy.Info{}, err
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	info = d.info
	info.Authors = append([]string(nil), d.info.Authors...)
	return info, nil
}

// source converts the file into XHTML for the layout. It converts the file
// once, and returns the same result on each later call.
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
			// can convert the file.
			d.srcErr = err
		}
	}()
	defer ebook.Recover("converting rtf", &err)
	c := newConverter(ctx, d)
	src, err = c.convert()
	if err != nil {
		return layout.Source{}, err
	}
	d.info = c.info()
	return src, nil
}
