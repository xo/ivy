// Copyright (c) 2026 Serge Vakulenko
// SPDX-License-Identifier: MIT

// Copied from github.com/sergev/dejapdf, file internal/djvu/outline.go, at
// commit 2b6ca0c1123c (D7, D26, docs/UPSTREAM.md). Its MIT license is in
// djvu/internal/LICENSE.

package doc // ivy: renamed from djvu, the name of the package of the format

import (
	"errors"
	"fmt"

	"github.com/xo/ivy"
)

// Bookmark is an outline entry. URL is "#" plus a page number or a
// component id for a link inside the document; it may be empty.
type Bookmark struct {
	Title, URL string
	Children   []*Bookmark
}

// ParseOutline decodes uncompressed NAVM data (spec 8.3.3): a 2-byte
// total count, then bookmarks in preorder.
func ParseOutline(b []byte) ([]*Bookmark, error) {
	if len(b) < 2 {
		return nil, errors.New("djvu: outline too short")
	}
	o := &outlineParser{b: b[2:], left: int(b[0])<<8 | int(b[1])}
	var r []*Bookmark
	for o.left > 0 {
		k, err := o.bookmark(0)
		if err != nil {
			return nil, err
		}
		r = append(r, k)
	}
	return r, nil
}

type outlineParser struct {
	b    []byte
	left int // bookmarks not yet read
}

func (o *outlineParser) bookmark(depth int) (*Bookmark, error) {
	if depth > maxSexpDepth {
		return nil, errors.New("djvu: outline nested too deep")
	}
	if len(o.b) < 1 {
		return nil, errors.New("djvu: outline truncated")
	}
	n := int(o.b[0])
	o.b = o.b[1:]
	o.left--
	title, err := o.str()
	if err != nil {
		return nil, err
	}
	url, err := o.str()
	if err != nil {
		return nil, err
	}
	k := &Bookmark{Title: title, URL: url}
	if n > o.left {
		return nil, fmt.Errorf("djvu: outline: %d children, %d bookmarks left", n, o.left)
	}
	for range n {
		c, err := o.bookmark(depth + 1)
		if err != nil {
			return nil, err
		}
		k.Children = append(k.Children, c)
	}
	return k, nil
}

func (o *outlineParser) str() (string, error) {
	if len(o.b) < 3 || 3+u24(o.b) > len(o.b) {
		return "", errors.New("djvu: outline truncated")
	}
	n := u24(o.b)
	s := string(o.b[3 : 3+n])
	o.b = o.b[3+n:]
	return s, nil
}

// Outline returns the document outline, or nil if it has none.
func (d *Document) Outline() ([]*Bookmark, error) {
	c := d.Root.Find("NAVM")
	if c == nil {
		return nil, nil
	}
	return OutlineOf(c.Data, d.lim) // ivy: within the limits
}

// OutlineOf decodes the data of a NAVM chunk within lim.MaxMemory.
//
// ivy: added, for the NAVM chunk of a bundled document, which ivy reads
// without the rest of the document.
func OutlineOf(data []byte, lim ivy.Limits) ([]*Bookmark, error) {
	b, err := unbzz(data, lim)
	if err != nil {
		return nil, fmt.Errorf("djvu: NAVM: %w", err)
	}
	return ParseOutline(b)
}
