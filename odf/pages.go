package odf

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/xo/ivy"
)

// The capabilities of a document.
var _ ivy.Pager = (*Document)(nil)

// pager gives the pages of one kind of OpenDocument file. text.go holds the
// pager of the texts, sheet.go the pager of the spreadsheets, and draw.go
// the pager of the presentations and the drawings.
type pager interface {
	count(ctx context.Context) (int, error)
	page(ctx context.Context, n int) (ivy.Page, error)
}

// pages holds the pager of a document, which the first call makes.
type pages struct {
	once  sync.Once
	pager pager
	err   error
}

// pagerOf returns the pager of the kind of d, from its mime type.
func (d *Document) pagerOf() (pager, error) {
	d.pages.once.Do(func() {
		kind := strings.TrimPrefix(d.mime, mimePrefix)
		kind = strings.TrimSuffix(kind, "-template")
		switch kind {
		case "text", "text-master", "text-web":
			d.pages.pager, d.pages.err = newTextPager(d)
		case "spreadsheet":
			d.pages.pager, d.pages.err = newSheetPager(d)
		case "presentation", "graphics":
			d.pages.pager, d.pages.err = newDrawPager(d)
		default:
			d.pages.err = fmt.Errorf("pages of %q: %w", d.mime, ivy.ErrUnsupported)
		}
	})
	return d.pages.pager, d.pages.err
}

// PageCount satisfies the ivy.Pager interface.
func (d *Document) PageCount(ctx context.Context) (int, error) {
	p, err := d.pagerOf()
	if err != nil {
		return 0, err
	}
	return p.count(ctx)
}

// Page satisfies the ivy.Pager interface. n counts from 0.
func (d *Document) Page(ctx context.Context, n int) (ivy.Page, error) {
	p, err := d.pagerOf()
	if err != nil {
		return nil, err
	}
	return p.page(ctx, n)
}

// readEntry reads the entry name of the package, and decrypts it if the
// file is encrypted. It returns an error that wraps ivy.ErrFormat if the
// package has no such entry.
func (d *Document) readEntry(ctx context.Context, name string) ([]byte, error) {
	f := d.archive.Find(name)
	if f == nil {
		return nil, fmt.Errorf("entry %s: %w", name, ivy.ErrFormat)
	}
	if enc := d.encryption(f); enc != nil {
		return d.decrypt(ctx, f, enc)
	}
	return d.archive.Read(f)
}
