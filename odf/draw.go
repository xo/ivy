package odf

import (
	"context"
	"strings"
	"sync"

	"github.com/xo/ivy"
	"github.com/xo/ivy/odf/internal/drawpage"
)

// drawPager is the pager of the presentations and the drawings. The package
// drawpage puts their pages on the display list.
type drawPager struct {
	d   *Document
	mu  sync.Mutex
	doc *drawpage.Document
	err error
}

// newDrawPager returns the pager of the presentations and the drawings. It
// reads nothing until the first call.
func newDrawPager(d *Document) (pager, error) {
	return &drawPager{d: d}, nil
}

// open reads the pages once. A read that the context stops is done again
// on the next call.
func (p *drawPager) open(ctx context.Context) (*drawpage.Document, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.doc != nil || p.err != nil {
		return p.doc, p.err
	}
	doc, err := drawpage.Open(ctx, drawpage.Options{
		Config:       p.d.cfg,
		Read:         p.d.readEntry,
		Presentation: strings.HasPrefix(strings.TrimPrefix(p.d.mime, mimePrefix), "presentation"),
	})
	if err != nil && ctx.Err() != nil {
		return nil, err
	}
	p.doc, p.err = doc, err
	return doc, err
}

// count satisfies the pager interface.
func (p *drawPager) count(ctx context.Context) (int, error) {
	doc, err := p.open(ctx)
	if err != nil {
		return 0, err
	}
	return doc.Count(), nil
}

// page satisfies the pager interface.
func (p *drawPager) page(ctx context.Context, n int) (ivy.Page, error) {
	doc, err := p.open(ctx)
	if err != nil {
		return nil, err
	}
	return doc.Page(ctx, n)
}
