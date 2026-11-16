package ebook

import (
	"context"
	"errors"
	"sync"

	"github.com/xo/ivy"
	"github.com/xo/ivy/internal/layout"
)

// Pages makes the book of the layout of a format once, and gives its pages.
// A format keeps one in its Document. The zero Pages is ready to use, and it
// is safe for concurrent use.
type Pages struct {
	mu   sync.Mutex
	book *layout.Book
	err  error
}

// Source returns the source of the layout of a format. ctx bounds the work.
type Source func(ctx context.Context) (layout.Source, error)

// Book returns the book of src, and makes it on the first call. It keeps the
// error of src, except the error of ctx, which a later call tries again.
func (p *Pages) Book(ctx context.Context, src Source, cfg ivy.Config) (*layout.Book, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.book != nil || p.err != nil {
		return p.book, p.err
	}
	s, err := src(ctx)
	if err != nil {
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return nil, err
		}
		p.err = err
		return nil, err
	}
	p.book = layout.NewBook(s, layout.BookOptions{Limits: cfg.Limits, Logger: cfg.Logger, SystemFonts: cfg.SystemFonts})
	return p.book, nil
}

// PageCount lays out the whole book of src and returns the number of its
// pages.
func (p *Pages) PageCount(ctx context.Context, src Source, cfg ivy.Config) (int, error) {
	book, err := p.Book(ctx, src, cfg)
	if err != nil {
		return 0, err
	}
	return book.PageCount(ctx)
}

// Page returns page n of the book of src, from 0. It lays out the book only
// as far as page n.
func (p *Pages) Page(ctx context.Context, src Source, cfg ivy.Config, n int) (ivy.Page, error) {
	book, err := p.Book(ctx, src, cfg)
	if err != nil {
		return nil, err
	}
	return book.Page(ctx, n)
}
