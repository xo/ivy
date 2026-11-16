// Copyright (c) 2026 Serge Vakulenko
// SPDX-License-Identifier: MIT

// Copied from github.com/sergev/dejapdf, file internal/djvu/anno.go, at
// commit 2b6ca0c1123c (D7, D26, docs/UPSTREAM.md). Its MIT license is in
// djvu/internal/LICENSE.

package doc // ivy: renamed from djvu, the name of the package of the format

import (
	"fmt"

	"github.com/xo/ivy"
	"github.com/xo/ivy/djvu/internal/iff"
)

// Annotations returns the S-expressions of all ANTa and ANTz chunks
// of the page and its includes, in order.
func (p *Page) Annotations() ([]*Sexp, error) {
	return annoOf(p.lim, append([]*iff.Chunk{p.Form}, p.Includes...)...)
}

// AnnotationsOf returns the S-expressions of the ANTa and ANTz chunks of
// the forms, in order, within lim.MaxMemory.
//
// ivy: added, for the shared annotations of a document, which a page does
// not have to include.
func AnnotationsOf(lim ivy.Limits, forms ...*iff.Chunk) ([]*Sexp, error) {
	return annoOf(lim, forms...)
}

func annoOf(lim ivy.Limits, forms ...*iff.Chunk) ([]*Sexp, error) { // ivy: within lim
	var r []*Sexp
	for _, f := range forms {
		if f == nil {
			continue
		}
		for _, c := range f.Children {
			b := c.Data
			switch c.ID {
			case "ANTa":
			case "ANTz":
				var err error
				if b, err = unbzz(b, lim); err != nil { // ivy: within lim
					return r, fmt.Errorf("djvu: ANTz: %w", err)
				}
			default:
				continue
			}
			r = append(r, ParseSexps(b)...)
		}
	}
	return r, nil
}
