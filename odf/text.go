package odf

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/xo/ivy"
	"github.com/xo/ivy/internal/ebook"
	"github.com/xo/ivy/internal/layout"
)

// textPager gives the pages of a text through the layout of ivy (D10,
// D15). It converts the text into one XHTML document with a style sheet,
// and the layout puts it on pages as far as the page that a program asks
// for.
type textPager struct {
	d *Document

	mu     sync.Mutex
	src    *layout.Source
	srcErr error
	pages  ebook.Pages
}

// newTextPager returns the pager of the texts.
func newTextPager(d *Document) (pager, error) {
	return &textPager{d: d}, nil
}

// config returns the configuration of the layout.
func (p *textPager) config() ivy.Config {
	cfg := p.d.cfg
	cfg.Limits = p.d.lim
	return cfg
}

// count satisfies the pager interface.
func (p *textPager) count(ctx context.Context) (n int, err error) {
	defer ebook.Recover("counting the pages of odf", &err)
	return p.pages.PageCount(ctx, p.source, p.config())
}

// page satisfies the pager interface.
func (p *textPager) page(ctx context.Context, n int) (pg ivy.Page, err error) {
	defer ebook.Recover("laying out odf", &err)
	return p.pages.Page(ctx, p.source, p.config(), n)
}

// source converts the text once, and returns the same result on each
// later call. An error of the context is not kept.
func (p *textPager) source(ctx context.Context) (src layout.Source, err error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	switch {
	case p.srcErr != nil:
		return layout.Source{}, p.srcErr
	case p.src != nil:
		return *p.src, nil
	}
	defer func() {
		switch {
		case err == nil:
			p.src = &src
		case !errors.Is(err, context.Canceled) && !errors.Is(err, context.DeadlineExceeded):
			p.srcErr = err
		}
	}()
	defer ebook.Recover("converting odf", &err)
	if err := ctx.Err(); err != nil {
		return layout.Source{}, fmt.Errorf("converting odf: %w", err)
	}
	return convertText(ctx, p.d)
}
