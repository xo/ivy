// Package msxls opens the workbooks of Excel 97 to Excel 2003: .xls files
// and .xlt templates, in BIFF8 (MS-XLS), and the workbooks of Excel 5 and
// Excel 95 in BIFF5.
//
// A workbook is a Compound File, which the package cfb of magic reads. The
// package reads the workbook stream: the sheets, the shared strings, the
// fonts, the formats of the cells and the number formats. It reads the
// cells of each worksheet with the values that the file holds. It computes
// no formula, and shows the result that Excel saved with it.
//
// Every worksheet goes on pages, in order, as printing does (D24). The
// package converts the sheets into the model of internal/sheet, which xlsx
// and odf share, and which puts them on pages with the layout of ivy (D10):
// the widths of the columns, the heights of the rows, the merged cells, the
// fonts, the alignment, the borders and the fills of the cells, the setup
// of the page, the header and the footer. internal/numfmt formats the
// numbers with their number formats. The package leaves out the charts,
// the pictures, the comments, the print areas, the print titles and the
// hidden sheets.
//
// A workbook gives its thumbnail and its properties from the stream
// "\x05SummaryInformation". A workbook that a password protects with RC4 or
// RC4 CryptoAPI asks for the password as D12 says. A workbook that Excel
// encrypts with its default password, VelvetSweatshop, opens without a
// password. The XOR obfuscation is not read, and the package returns an
// error that wraps ivy.ErrUnsupported for it.
package msxls

import (
	"context"
	"fmt"
	"iter"
	"sync"

	"github.com/xo/ivy"
	"github.com/xo/ivy/internal/ebook"
	"github.com/xo/ivy/internal/layout"
	"github.com/xo/ivy/internal/msole"
	"github.com/xo/ivy/internal/sheet"
)

// Format describes the package for the registry of a program.
var Format = ivy.Format{
	Name:  "msxls",
	Mimes: []string{"application/vnd.ms-excel", "application/msexcel", "application/x-msexcel", "application/x-excel"},
	Exts:  []string{"xls", "xlt"},
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

// Document is an open Excel workbook.
type Document struct {
	cfg     ivy.Config
	summary msole.Summary
	wb      *workbook

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

// Open opens an Excel workbook. It reads the summary information and the
// globals of the workbook stream, and decrypts the stream if a password
// protects it. It returns an error that wraps ivy.ErrFormat if src is not a
// workbook, one that wraps ivy.ErrPassword if the password is missing or
// wrong, and one that wraps ivy.ErrUnsupported for a workbook before BIFF5
// or with the XOR obfuscation.
func Open(ctx context.Context, src ivy.Source, opts ...Option) (d *Document, err error) {
	var o options
	for _, opt := range opts {
		opt(&o)
	}
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("opening excel workbook: %w", err)
	}
	defer ebook.Recover("opening excel workbook", &err)
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
	name := "Workbook"
	if !msole.Has(f, name) {
		// Excel 5 and Excel 95
		name = "Book"
	}
	buf, err := msole.ReadStream(f, name, cfg.Limits)
	if err != nil {
		return nil, err
	}
	wb, err := readWorkbook(ctx, buf, cfg)
	if err != nil {
		return nil, err
	}
	return &Document{cfg: cfg, summary: summary, wb: wb}, nil
}

// Sheets returns the names of the worksheets that the document shows, in
// order. It leaves out the hidden sheets, the charts and the macro sheets.
func (d *Document) Sheets() []string {
	var names []string
	for _, bs := range d.wb.sheets {
		if bs.kind == 0 && !bs.hidden {
			names = append(names, bs.name)
		}
	}
	return names
}

// Close satisfies the ivy.Document interface.
func (d *Document) Close() error {
	return nil
}

// Previews satisfies the ivy.Previewer interface. It lists the thumbnail of
// the summary information, which Excel writes when a person asks for a
// preview picture.
func (d *Document) Previews(ctx context.Context) iter.Seq2[ivy.Preview, error] {
	return d.summary.Previews(ctx, d.cfg)
}

// Describe satisfies the ivy.Describer interface. It returns the
// properties of the summary information.
func (d *Document) Describe(ctx context.Context) (ivy.Info, error) {
	if err := ctx.Err(); err != nil {
		return ivy.Info{}, fmt.Errorf("describing excel workbook: %w", err)
	}
	return d.summary.Info, nil
}

// PageCount satisfies the ivy.Pager interface. It lays out every sheet on
// its first call.
func (d *Document) PageCount(ctx context.Context) (int, error) {
	return d.pages.PageCount(ctx, d.source, d.cfg)
}

// Page satisfies the ivy.Pager interface. It lays out the sheets only as
// far as page n.
func (d *Document) Page(ctx context.Context, n int) (ivy.Page, error) {
	return d.pages.Page(ctx, d.source, d.cfg, n)
}

// source converts the sheets into HTML for the layout, once.
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
			// convert the sheets
			d.srcErr = err
		}
	}()
	defer ebook.Recover("converting excel workbook", &err)
	return d.convert(ctx)
}

// convert puts the worksheets on pages with internal/sheet, as xlsx does.
func (d *Document) convert(ctx context.Context) (layout.Source, error) {
	book, err := d.wb.book(ctx, d.summary.Info.Title, d.cfg.Limits)
	if err != nil {
		return layout.Source{}, err
	}
	return book.Source(ctx, sheet.Options{Limits: d.cfg.Limits, Logger: d.cfg.Logger})
}
