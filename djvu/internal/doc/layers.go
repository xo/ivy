// Copyright (c) 2026 Serge Vakulenko
// SPDX-License-Identifier: MIT

// Copied from github.com/sergev/dejapdf, file internal/djvu/layers.go, at
// commit 2b6ca0c1123c (D7, D26, docs/UPSTREAM.md). Its MIT license is in
// djvu/internal/LICENSE.

package doc // ivy: renamed from djvu, the name of the package of the format

import (
	"fmt"

	"github.com/xo/ivy"
	"github.com/xo/ivy/djvu/internal/iw44"
)

// Background decodes the page's BG44 chunks, or returns nil if it has
// none. The image has the layer's own (usually reduced) size.
func (p *Page) Background() (*iw44.Image, error) {
	return p.iw44("BG44")
}

// Foreground decodes the page's FG44 chunk, or returns nil.
func (p *Page) Foreground() (*iw44.Image, error) {
	return p.iw44("FG44")
}

// Layer decodes the page's chunks of an IW44 layer, BG44 or FG44, and
// returns the decoder, from which the image comes at any scale. It
// returns nil if the page has none. If a later chunk fails, it returns
// the decoder with the chunks before it, and the error. An image larger
// than the limits of the page returns an error that wraps ivy.ErrLimit.
//
// ivy: added, so that ivy can draw a layer at a reduced scale.
func (p *Page) Layer(id string) (*iw44.Decoder, error) {
	cs := p.Form.All(id)
	if len(cs) == 0 {
		return nil, nil
	}
	if w, h, ok := iw44.Size(cs[0].Data); ok {
		lim := p.lim.WithDefaults()
		// The coefficients take 2 bytes for each pixel of each of 3
		// components, and an image 3 bytes more.
		if px := int64(w) * int64(h); px > lim.MaxPixels || 9*px > lim.MaxMemory {
			return nil, fmt.Errorf("djvu: page %q: %s of %dx%d: %w", p.ID, id, w, h, ivy.ErrLimit)
		}
	}
	var d iw44.Decoder
	for i, c := range cs {
		if err := d.Decode(c.Data); err != nil {
			err = fmt.Errorf("djvu: page %q: %s: %w", p.ID, id, err)
			if i == 0 {
				return nil, err
			}
			return &d, err
		}
	}
	return &d, nil
}

func (p *Page) iw44(id string) (*iw44.Image, error) {
	cs := p.Form.All(id)
	if len(cs) == 0 {
		return nil, nil
	}
	var d iw44.Decoder
	for _, c := range cs {
		if err := d.Decode(c.Data); err != nil {
			return nil, fmt.Errorf("djvu: page %q: %s: %w", p.ID, id, err)
		}
	}
	return d.Image()
}
