// Package xlsx opens Excel files in Office Open XML.
//
// On 2026-11-16 a document gives the thumbnail that the file can hold, in
// docProps/thumbnail.* (D11). The pages come in phase 5, with the files that
// a password protects (D12).
package xlsx

import (
	"context"
	"iter"

	"github.com/xo/ivy"
	"github.com/xo/ivy/internal/ooxml"
)

// Format describes the package for the registry of a program.
var Format = ivy.Format{
	Name: "xlsx",
	Mimes: []string{
		"application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",
		"application/vnd.openxmlformats-officedocument.spreadsheetml.template",
		"application/vnd.ms-excel.sheet.macroEnabled.12",
		"application/vnd.ms-excel.template.macroEnabled.12",
	},
	Exts: []string{"xlsx", "xltx", "xlsm", "xltm"},
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

// Document is an open Excel file.
type Document struct {
	doc *ooxml.Document
}

// The capabilities of a document.
var _ ivy.Previewer = (*Document)(nil)

// Open opens a Excel file. It returns an error that wraps ivy.ErrFormat if
// src is not an Office Open XML package, and ivy.ErrUnsupported if a password
// protects it.
func Open(ctx context.Context, src ivy.Source, opts ...Option) (*Document, error) {
	var o options
	for _, opt := range opts {
		opt(&o)
	}
	doc, err := ooxml.Open(ctx, src, o.cfg)
	if err != nil {
		return nil, err
	}
	return &Document{doc: doc}, nil
}

// Close satisfies the ivy.Document interface.
func (d *Document) Close() error {
	return nil
}

// Previews satisfies the ivy.Previewer interface. It lists the thumbnail.
func (d *Document) Previews(ctx context.Context) iter.Seq2[ivy.Preview, error] {
	return d.doc.Previews(ctx)
}
