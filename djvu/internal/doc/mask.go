// Copyright (c) 2026 Serge Vakulenko
// SPDX-License-Identifier: MIT

// Copied from github.com/sergev/dejapdf, file internal/djvu/mask.go, at
// commit 2b6ca0c1123c (D7, D26, docs/UPSTREAM.md). Its MIT license is in
// djvu/internal/LICENSE.

package doc // ivy: renamed from djvu, the name of the package of the format

import (
	"fmt"
	"sync"

	"github.com/xo/ivy/djvu/internal/iff"
	"github.com/xo/ivy/djvu/internal/jb2"
)

// Mask decodes the page's JB2 mask (Sjbz), or its G4 mask (Smmr) as an
// image of one blit, or returns nil if it has neither. A shared
// dictionary (Djbz, in the page or an include) is decoded once per
// document.
func (d *Document) Mask(p *Page) (*jb2.Image, error) {
	s := p.Form.Find("Sjbz")
	if s == nil {
		if m := p.Form.Find("Smmr"); m != nil {
			im, err := mmrMask(m.Data, d.lim) // ivy: within the limits
			if err != nil {
				// ivy: im holds the rows that decoded, or is nil.
				return im, fmt.Errorf("djvu: page %q: %w", p.ID, err)
			}
			return im, nil
		}
		return nil, nil
	}
	var dict *jb2.Dict
	if c := p.Chunks("Djbz"); len(c) > 0 {
		var err error
		if dict, err = d.dict(c[0]); err != nil {
			return nil, err
		}
	}
	im, err := jb2.DecodeWithin(s.Data, dict, d.lim) // ivy: within the limits
	if err != nil {
		return nil, fmt.Errorf("djvu: page %q: %w", p.ID, err)
	}
	return im, nil
}

// dictEntry is a Djbz decoded once.
type dictEntry struct {
	once sync.Once
	dict *jb2.Dict
	err  error
}

func (d *Document) dict(c *iff.Chunk) (*jb2.Dict, error) {
	d.mu.Lock()
	e, ok := d.dicts[c]
	if !ok {
		e = &dictEntry{}
		if d.dicts == nil {
			d.dicts = map[*iff.Chunk]*dictEntry{}
		}
		d.dicts[c] = e
	}
	d.mu.Unlock()
	e.once.Do(func() {
		e.dict, e.err = jb2.DecodeDictWithin(c.Data, nil, d.lim) // ivy: within the limits
		if e.err != nil {
			e.err = fmt.Errorf("djvu: Djbz: %w", e.err)
			return
		}
		d.keep(c, e)
	})
	return e.dict, e.err
}

// keep counts the bytes of the dictionary of e. If the dictionaries that
// the document keeps take more than lim.MaxMemory, it forgets the others,
// and a page that needs one decodes it again.
//
// ivy: added, so that a document with many large dictionaries does not
// keep them all.
func (d *Document) keep(c *iff.Chunk, e *dictEntry) {
	var n int64
	for _, s := range e.dict.Shapes {
		n += int64(len(s.Pix))
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.dictSize+n > d.lim.MaxMemory {
		for k := range d.dicts {
			if k != c {
				delete(d.dicts, k)
			}
		}
		d.dictSize = 0
	}
	d.dictSize += n
}
