// Package vsdx opens the drawings of Visio for their previews: the files of
// Office Open XML of Visio 2013 and later, .vsdx, .vsdm, .vstx, .vstm,
// .vssx and .vssm, and the binary files of Visio 2003 to 2010, .vsd, .vst
// and .vss.
//
// The package gives the thumbnail and the properties of a drawing, and no
// pages (D5). A file of Office Open XML holds its thumbnail in
// docProps/thumbnail.emf or in a raster image, and its properties in
// docProps/core.xml. A binary file is a Compound File, which holds them in
// the stream "\x05SummaryInformation". The package draws a thumbnail in
// EMF or WMF with internal/metafile.
//
// A file of Office Open XML that a password protects asks for the password
// as D12 says.
package vsdx

import (
	"bytes"
	"context"
	"fmt"
	"iter"

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

// Document is an open Visio drawing.
type Document struct {
	cfg ivy.Config
	// pkg is the package of a file of Office Open XML, or nil for a binary
	// file.
	pkg     *ooxml.Document
	summary msole.Summary
}

// The capabilities of a document.
var (
	_ ivy.Previewer = (*Document)(nil)
	_ ivy.Describer = (*Document)(nil)
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
