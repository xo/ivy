// Copyright (c) 2026 Serge Vakulenko
// SPDX-License-Identifier: MIT

// Copied from github.com/sergev/dejapdf, file internal/djvu/text.go, at
// commit 2b6ca0c1123c (D7, D26, docs/UPSTREAM.md). Its MIT license is in
// djvu/internal/LICENSE.

package doc // ivy: renamed from djvu, the name of the package of the format

import (
	"errors"
	"fmt"

	"github.com/xo/ivy"
	"github.com/xo/ivy/djvu/internal/iff"
)

// ZoneType is the kind of a hidden-text zone.
type ZoneType int

const (
	ZPage ZoneType = 1 + iota
	ZColumn
	ZRegion
	ZParagraph
	ZLine
	ZWord
	ZChar
)

var zoneNames = [...]string{"", "page", "column", "region", "para", "line", "word", "char"}

// String returns the name djvused uses.
func (t ZoneType) String() string {
	if t >= ZPage && t <= ZChar {
		return zoneNames[t]
	}
	return fmt.Sprintf("zone%d", int(t))
}

// Zone is a hidden-text zone. Coordinates are pixels from the bottom
// left of the unrotated page; X1, Y1 are exclusive.
type Zone struct {
	Type           ZoneType
	X0, Y0, X1, Y1 int
	Text           string // the zone's slice of the page text, separators included
	Children       []*Zone
}

const zoneSize = 17

// ParseText decodes uncompressed TXTa/TXTz data (spec 8.3.5).
//
// ivy: it takes the default limits. ParseTextWithin takes the limits of
// the caller.
func ParseText(b []byte) (*Zone, error) {
	return ParseTextWithin(b, ivy.Limits{})
}

// ParseTextWithin is ParseText within lim. It reads at most
// lim.MaxObjects zones, nested at most lim.MaxDepth deep, and an error past
// a limit wraps ivy.ErrLimit.
//
// ivy: added, for the limits of D5.
func ParseTextWithin(b []byte, lim ivy.Limits) (*Zone, error) {
	if len(b) < 3 {
		return nil, errors.New("djvu: text too short")
	}
	n := u24(b)
	if 3+n+1 > len(b) {
		return nil, errors.New("djvu: text truncated")
	}
	lim = lim.WithDefaults()
	// ivy: each zone takes its text from one string of the whole text. A
	// copy for each zone takes memory that grows with the number of zones
	// times the length of the text, because zones can overlap.
	t := &textParser{text: string(b[3 : 3+n]), b: b[3+n+1:],
		objects: lim.MaxObjects, depth: min(lim.MaxDepth, maxZoneDepth)}
	if len(t.b) == 0 {
		return nil, nil // text without zones
	}
	z, err := t.zone(nil, nil, 0)
	if err != nil {
		return nil, err
	}
	return z.Zone, nil
}

type textParser struct {
	text string // ivy: a string, which the zones share
	b    []byte

	// ivy: the zones that the parser can still read, and the depth that it
	// can follow.
	objects, depth int
}

// placed is a zone with its text offset.
type placed struct {
	*Zone
	start int
}

const maxZoneDepth = 32

// zone reads a zone and its children. Zones are placed relative to
// the parent's upper-left corner (first child) or to the previous
// sibling: page, paragraph and line from its lower-left corner, y
// down; column, region, word and character from its lower-right
// corner, y up.
func (t *textParser) zone(parent, prev *placed, depth int) (*placed, error) {
	if depth > t.depth { // ivy: within lim.MaxDepth
		return nil, fmt.Errorf("djvu: text zones nested more than %d deep: %w", t.depth, ivy.ErrLimit)
	}
	if len(t.b) < zoneSize {
		return nil, errors.New("djvu: text zone truncated")
	}
	if t.objects--; t.objects < 0 { // ivy: within lim.MaxObjects
		return nil, fmt.Errorf("djvu: text zones: %w", ivy.ErrLimit)
	}
	r := t.b[:zoneSize]
	t.b = t.b[zoneSize:]
	typ := ZoneType(r[0])
	x, y, w, h, start := s16(r[1:]), s16(r[3:]), s16(r[5:]), s16(r[7:]), s16(r[9:])
	length, nkids := u24(r[11:]), u24(r[14:])
	switch {
	case prev != nil:
		start += prev.start + len(prev.Text)
		if typ == ZColumn || typ == ZRegion || typ == ZWord || typ == ZChar {
			x += prev.X1
			y += prev.Y0
		} else {
			x += prev.X0
			y = prev.Y0 - y - h
		}
	case parent != nil:
		start += parent.start
		x += parent.X0
		y = parent.Y1 - y - h
	}
	if start < 0 || length < 0 || start+length > len(t.text) {
		return nil, fmt.Errorf("djvu: text zone [%d,+%d) outside %d bytes", start, length, len(t.text))
	}
	if nkids*zoneSize > len(t.b) {
		return nil, errors.New("djvu: text zone children truncated")
	}
	z := &placed{&Zone{Type: typ, X0: x, Y0: y, X1: x + w, Y1: y + h,
		Text: t.text[start : start+length]}, start} // ivy: no copy of the text
	var k *placed
	for range nkids {
		var err error
		if k, err = t.zone(z, k, depth+1); err != nil {
			return nil, err
		}
		z.Children = append(z.Children, k.Zone)
	}
	return z, nil
}

// Text returns the page's hidden text, or nil if it has none.
func (p *Page) Text() (*Zone, error) {
	return textOf(p.lim, append([]*iff.Chunk{p.Form}, p.Includes...)...)
}

func textOf(lim ivy.Limits, forms ...*iff.Chunk) (*Zone, error) { // ivy: within lim
	for _, f := range forms {
		if f == nil {
			continue
		}
		for _, c := range f.Children {
			switch c.ID {
			case "TXTa":
				return ParseTextWithin(c.Data, lim)
			case "TXTz":
				b, err := unbzz(c.Data, lim) // ivy: within lim
				if err != nil {
					return nil, fmt.Errorf("djvu: TXTz: %w", err)
				}
				return ParseTextWithin(b, lim)
			}
		}
	}
	return nil, nil
}

// ivy: u24 is in limits.go.

func s16(b []byte) int { return int(b[0])<<8 | int(b[1]) - 0x8000 }
