// Package msppt opens the presentations of PowerPoint 97 to PowerPoint
// 2003: .ppt files, .pps shows and .pot templates (MS-PPT).
//
// A presentation is a Compound File, which the package cfb of magic reads.
// The package reads the stream "PowerPoint Document" through the stream
// "Current User", the chain of the edits and the persist directory, to the
// slides in their order. It reads the text of each shape of a slide, with
// its place on the slide and its kind: a title, a body or another text. It
// puts each slide on a page of the size of the slides, with each text in
// its place, for the layout of ivy (D10). The texts of a body are lists
// with bullets.
//
// The package leaves out the pictures, the shapes without text, the
// masters, the backgrounds, the notes, and the fonts, the sizes and the
// colors of the runs of text. A text has the size of its kind.
//
// A presentation gives its thumbnail and its properties from the stream
// "\x05SummaryInformation". A presentation that a password protects with
// RC4 CryptoAPI asks for the password as D12 says.
package msppt

import (
	"context"
	"fmt"
	"iter"
	"strconv"
	"sync"

	"github.com/xo/ivy"
	"github.com/xo/ivy/internal/ebook"
	"github.com/xo/ivy/internal/layout"
	"github.com/xo/ivy/internal/memfs"
	"github.com/xo/ivy/internal/msole"
)

// Format describes the package for the registry of a program.
var Format = ivy.Format{
	Name:  "msppt",
	Mimes: []string{"application/vnd.ms-powerpoint", "application/mspowerpoint", "application/x-mspowerpoint"},
	Exts:  []string{"ppt", "pps", "pot"},
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

// Document is an open PowerPoint presentation.
type Document struct {
	cfg     ivy.Config
	summary msole.Summary
	stream  *stream
	doc     *document

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

// Open opens a PowerPoint presentation. It reads the summary information,
// the persist directory and the list of the slides, and asks for the
// password if one protects the presentation. It returns an error that wraps
// ivy.ErrFormat if src is not a presentation, and one that wraps
// ivy.ErrPassword if the password is missing or wrong.
func Open(ctx context.Context, src ivy.Source, opts ...Option) (d *Document, err error) {
	var o options
	for _, opt := range opts {
		opt(&o)
	}
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("opening powerpoint file: %w", err)
	}
	defer ebook.Recover("opening powerpoint file", &err)
	cfg := o.cfg
	cfg.Limits = cfg.Limits.WithDefaults()
	f, err := msole.Open(src, cfg.Limits)
	if err != nil {
		return nil, err
	}
	summary, err := msole.ReadSummary(f, cfg.Limits)
	if err != nil {
		return nil, err
	}
	s, err := readStream(ctx, f, cfg)
	if err != nil {
		return nil, err
	}
	doc, ok := readDocument(s)
	if !ok {
		return nil, fmt.Errorf("powerpoint file without a document container: %w", ivy.ErrFormat)
	}
	return &Document{cfg: cfg, summary: summary, stream: s, doc: doc}, nil
}

// Close satisfies the ivy.Document interface.
func (d *Document) Close() error {
	return nil
}

// Previews satisfies the ivy.Previewer interface. It lists the thumbnail of
// the summary information: PowerPoint writes one of the first slide.
func (d *Document) Previews(ctx context.Context) iter.Seq2[ivy.Preview, error] {
	return d.summary.Previews(ctx, d.cfg)
}

// Describe satisfies the ivy.Describer interface. It returns the
// properties of the summary information.
func (d *Document) Describe(ctx context.Context) (ivy.Info, error) {
	if err := ctx.Err(); err != nil {
		return ivy.Info{}, fmt.Errorf("describing powerpoint file: %w", err)
	}
	return d.summary.Info, nil
}

// PageCount satisfies the ivy.Pager interface. Each slide is a page.
func (d *Document) PageCount(ctx context.Context) (int, error) {
	return d.pages.PageCount(ctx, d.source, d.cfg)
}

// Page satisfies the ivy.Pager interface.
func (d *Document) Page(ctx context.Context, n int) (ivy.Page, error) {
	return d.pages.Page(ctx, d.source, d.cfg, n)
}

// source converts the slides into HTML for the layout, once.
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
			// an error of the context is not kept, so that a later call can
			// convert the slides
			d.srcErr = err
		}
	}()
	defer ebook.Recover("converting powerpoint file", &err)
	return d.convert(ctx)
}

// convert writes each slide as an HTML document of its own, of the size of
// the slides, so that each slide is one page.
func (d *Document) convert(ctx context.Context) (layout.Source, error) {
	lim := d.cfg.Limits
	var files memfs.FS
	var docs []string
	var total, read int64
	for i, ref := range d.doc.slides {
		if err := ctx.Err(); err != nil {
			return layout.Source{}, fmt.Errorf("converting powerpoint file: %w", err)
		}
		sl, size := readSlide(d.stream, ref, lim.MaxDepth)
		// a slide list can name one large slide many times
		if read += int64(size); read > lim.MaxMemory {
			return layout.Source{}, fmt.Errorf("slides of more than %d bytes: %w", lim.MaxMemory, ivy.ErrLimit)
		}
		head, body := slideHTML(sl, d.doc.width, d.doc.height)
		doc := msole.HTML(d.summary.Info.Title, "", head, body)
		total += int64(len(doc))
		if total > lim.MaxMemory {
			return layout.Source{}, fmt.Errorf("html of more than %d bytes: %w", lim.MaxMemory, ivy.ErrLimit)
		}
		name := "slide" + strconv.Itoa(i+1) + ".html"
		files.Add(name, []byte(doc))
		docs = append(docs, name)
	}
	if len(docs) == 0 {
		// a presentation without a slide has one empty page
		head, body := slideHTML(slide{}, d.doc.width, d.doc.height)
		files.Add("empty.html", []byte(msole.HTML(d.summary.Info.Title, "", head, body)))
		docs = append(docs, "empty.html")
	}
	return layout.Source{
		FS:          &files,
		Documents:   docs,
		StyleSheets: []string{styleSheet},
		Title:       d.summary.Info.Title,
	}, nil
}
