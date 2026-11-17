// Package vsdx opens the drawings of Visio: the files of Office Open XML of
// Visio 2013 and later, .vsdx, .vsdm, .vstx, .vstm, .vssx and .vssm, and
// the binary files of Visio 2003 to 2010, .vsd, .vst and .vss.
//
// The package draws the pages of a file of Office Open XML onto the display
// list (D10, D30), as [MS-VSDX] gives them: one page for each foreground
// page, at the size of the page, with its background pages behind it. A
// shape inherits its cells, its sections and its text from its master
// shape and from its styles, and the cells that the theme gives take the
// colors and the styles of the theme. The package draws the geometry of a
// shape (lines, arcs, elliptical arcs, ellipses, polylines, B-splines and
// NURBS), its fill, its line, its arrows, its shadow, its text, its picture
// and its groups, with internal/drawingml. It computes no formula of the
// ShapeSheet except the trivial ones, and draws with the values that the
// file holds. A stencil (.vssx, .vssm) has no pages.
//
// The package gives the thumbnail and the properties of a drawing. A file
// of Office Open XML holds its thumbnail in docProps/thumbnail.emf or in a
// raster image, and its properties in docProps/core.xml. A binary file is
// a Compound File, which holds them in the stream
// "\x05SummaryInformation". The package draws a thumbnail in EMF or WMF
// with internal/metafile. A binary file has no pages (D30): the Document
// that Format.Open returns for it is not an ivy.Pager.
//
// A file of Office Open XML that a password protects asks for the password
// as D12 says.
package vsdx

import (
	"bytes"
	"context"
	"fmt"
	"iter"
	"sync"

	"github.com/xo/ivy"
	"github.com/xo/ivy/internal/ebook"
	"github.com/xo/ivy/internal/msole"
	"github.com/xo/ivy/internal/ooxml"
)

// Format describes the package for the registry of a program.
var Format = ivy.Format{
	Name: "vsdx",
	Mimes: []string{
		"application/vnd.ms-visio.drawing",
		"application/vnd.ms-visio.drawing.macroEnabled.12",
		"application/vnd.ms-visio.template",
		"application/vnd.ms-visio.template.macroEnabled.12",
		"application/vnd.ms-visio.stencil",
		"application/vnd.ms-visio.stencil.macroEnabled.12",
		"application/vnd.visio",
		"application/x-visio",
	},
	Exts: []string{"vsdx", "vsdm", "vstx", "vstm", "vssx", "vssm", "vsd", "vst", "vss"},
	Open: func(ctx context.Context, src ivy.Source, cfg ivy.Config) (ivy.Document, error) {
		d, err := Open(ctx, src, WithConfig(cfg))
		if err != nil {
			return nil, err
		}
		if d.pkg == nil || !d.pkg.HasPart(pagesPart) {
			// a binary file and a stencil have only their thumbnails
			return previewDocument{d}, nil
		}
		return d, nil
	},
}

// pagesPart is the part that lists the pages of a drawing. A stencil does
// not have it.
const pagesPart = "visio/pages/pages.xml"

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

// Document is an open Visio drawing.
type Document struct {
	cfg ivy.Config
	// pkg is the package of a file of Office Open XML, or nil for a binary
	// file.
	pkg     *ooxml.Document
	summary msole.Summary

	once sync.Once
	dr   *drawing
	err  error
}

// The capabilities of a document.
var (
	_ ivy.Previewer = (*Document)(nil)
	_ ivy.Describer = (*Document)(nil)
	_ ivy.Pager     = (*Document)(nil)
)

// Open opens a Visio drawing. It returns an error that wraps ivy.ErrFormat
// if src is not a Visio file, and one that wraps ivy.ErrPassword if a
// password protects the file and the password is missing or wrong.
func Open(ctx context.Context, src ivy.Source, opts ...Option) (d *Document, err error) {
	var o options
	for _, opt := range opts {
		opt(&o)
	}
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("opening visio drawing: %w", err)
	}
	defer ebook.Recover("opening visio drawing", &err)
	cfg := o.cfg
	cfg.Limits = cfg.Limits.WithDefaults()
	head := make([]byte, len(msole.Signature))
	if _, err := src.ReadAt(head, 0); err == nil && bytes.Equal(head, msole.Signature) {
		f, err := msole.Open(src, cfg.Limits)
		if err != nil {
			return nil, err
		}
		if !msole.Has(f, "VisioDocument") {
			// a package of Office Open XML that a password protects
			if !msole.Has(f, "EncryptionInfo") {
				return nil, fmt.Errorf("compound file without the stream VisioDocument: %w", ivy.ErrFormat)
			}
			return openPackage(ctx, src, cfg)
		}
		summary, err := msole.ReadSummary(f, cfg.Limits)
		if err != nil {
			return nil, err
		}
		return &Document{cfg: cfg, summary: summary}, nil
	}
	return openPackage(ctx, src, cfg)
}

// openPackage opens a file of Office Open XML.
func openPackage(ctx context.Context, src ivy.Source, cfg ivy.Config) (*Document, error) {
	pkg, err := ooxml.Open(ctx, src, cfg)
	if err != nil {
		return nil, err
	}
	return &Document{cfg: cfg, pkg: pkg}, nil
}

// Close satisfies the ivy.Document interface.
func (d *Document) Close() error {
	return nil
}

// Previews satisfies the ivy.Previewer interface. It lists the thumbnail.
func (d *Document) Previews(ctx context.Context) iter.Seq2[ivy.Preview, error] {
	if d.pkg != nil {
		return d.pkg.Previews(ctx)
	}
	return d.summary.Previews(ctx, d.cfg)
}

// Describe satisfies the ivy.Describer interface. It returns the core
// properties of a file of Office Open XML, or the summary information of a
// binary file.
func (d *Document) Describe(ctx context.Context) (ivy.Info, error) {
	if err := ctx.Err(); err != nil {
		return ivy.Info{}, fmt.Errorf("describing visio drawing: %w", err)
	}
	if d.pkg != nil {
		return d.pkg.Describe(ctx)
	}
	return d.summary.Info, nil
}

// drawing returns the content that the pages share, which the first call
// reads. A binary file has none.
func (d *Document) drawing(ctx context.Context) (*drawing, error) {
	if d.pkg == nil {
		return nil, nil
	}
	d.once.Do(func() {
		defer ebook.Recover("reading the drawing", &d.err)
		d.dr, d.err = readDrawing(ctx, d)
	})
	return d.dr, d.err
}

// PageCount satisfies the ivy.Pager interface. It counts the foreground
// pages. A binary file and a stencil have no pages.
func (d *Document) PageCount(ctx context.Context) (n int, err error) {
	if err := ctx.Err(); err != nil {
		return 0, fmt.Errorf("counting the pages: %w", err)
	}
	defer ebook.Recover("counting the pages", &err)
	dr, err := d.drawing(ctx)
	if err != nil || dr == nil {
		return 0, err
	}
	return len(dr.fore), nil
}

// Page satisfies the ivy.Pager interface. n counts the foreground pages,
// from 0. A page drawn in part renders with an *ivy.PartialError.
func (d *Document) Page(ctx context.Context, n int) (p ivy.Page, err error) {
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("reading page %d: %w", n+1, err)
	}
	defer ebook.Recover(fmt.Sprintf("reading page %d", n+1), &err)
	dr, err := d.drawing(ctx)
	if err != nil {
		return nil, err
	}
	count := 0
	if dr != nil {
		count = len(dr.fore)
	}
	if n < 0 || n >= count {
		return nil, fmt.Errorf("page %d of %d: %w", n+1, count, ivy.ErrPageRange)
	}
	pg, err := dr.buildPage(ctx, n)
	if err != nil {
		return nil, err
	}
	return pg, nil
}

// previewDocument is a binary Visio file or a stencil, which has a
// thumbnail and properties, and no pages.
type previewDocument struct {
	d *Document
}

// The capabilities of a binary file and of a stencil.
var (
	_ ivy.Previewer = previewDocument{}
	_ ivy.Describer = previewDocument{}
)

// Close satisfies the ivy.Document interface.
func (p previewDocument) Close() error {
	return p.d.Close()
}

// Previews satisfies the ivy.Previewer interface. It lists the thumbnail.
func (p previewDocument) Previews(ctx context.Context) iter.Seq2[ivy.Preview, error] {
	return p.d.Previews(ctx)
}

// Describe satisfies the ivy.Describer interface.
func (p previewDocument) Describe(ctx context.Context) (ivy.Info, error) {
	return p.d.Describe(ctx)
}
