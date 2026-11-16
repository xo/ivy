// Copyright (c) 2026 Serge Vakulenko
// SPDX-License-Identifier: MIT

// Copied from github.com/sergev/dejapdf, file internal/djvu/dir.go, at
// commit 2b6ca0c1123c (D7, D26, docs/UPSTREAM.md). Its MIT license is in
// djvu/internal/LICENSE.

package doc // ivy: renamed from djvu, the name of the package of the format

import (
	"bytes"
	"encoding/binary"
	"fmt"

	"github.com/xo/ivy"
)

// ComponentType is the kind of a document component.
type ComponentType int

const (
	Include    ComponentType = iota // FORM:DJVI
	PageComp                        // FORM:DJVU
	Thumbnails                      // FORM:THUM
	SharedAnno                      // FORM:DJVI with shared annotations
)

// Component is one DIRM entry.
type Component struct {
	ID     string
	Name   string // file name; defaults to ID
	Title  string // defaults to ID
	Type   ComponentType
	Offset int // bundled only: absolute offset of the FORM
	Size   int

	HasName, HasTitle bool
}

// Dir is a decoded DIRM chunk.
type Dir struct {
	Bundled    bool
	Version    int
	Components []Component
}

// NumPages returns the number of page components.
func (d *Dir) NumPages() int {
	n := 0
	for _, c := range d.Components {
		if c.Type == PageComp {
			n++
		}
	}
	return n
}

// parseDirHeader decodes the uncompressed part of DIRM and returns
// the remaining BZZ data.
func parseDirHeader(b []byte) (*Dir, []byte, error) {
	if len(b) < 3 {
		return nil, nil, fmt.Errorf("djvu: DIRM too short")
	}
	d := &Dir{Bundled: b[0]&0x80 != 0, Version: int(b[0] & 0x7f)}
	n := int(binary.BigEndian.Uint16(b[1:]))
	d.Components = make([]Component, n)
	b = b[3:]
	if d.Bundled {
		if len(b) < 4*n {
			return nil, nil, fmt.Errorf("djvu: DIRM offsets truncated")
		}
		for i := range d.Components {
			d.Components[i].Offset = int(binary.BigEndian.Uint32(b[4*i:]))
		}
		b = b[4*n:]
	}
	return d, b, nil
}

// parseDirBody fills components from the decompressed part of DIRM:
// 24-bit sizes, flag bytes, then NUL-terminated id, name, title.
func (d *Dir) parseDirBody(b []byte) error {
	n := len(d.Components)
	if len(b) < 4*n {
		return fmt.Errorf("djvu: DIRM body truncated")
	}
	for i := range d.Components {
		p := b[3*i:]
		d.Components[i].Size = int(p[0])<<16 | int(p[1])<<8 | int(p[2])
	}
	flags := b[3*n : 4*n]
	b = b[4*n:]
	str := func() (string, error) {
		k := bytes.IndexByte(b, 0)
		if k < 0 {
			return "", fmt.Errorf("djvu: DIRM name not terminated")
		}
		s := string(b[:k])
		b = b[k+1:]
		return s, nil
	}
	for i := range d.Components {
		c := &d.Components[i]
		f := flags[i]
		c.Type = ComponentType(f & 0x3f)
		c.HasName = f&0x80 != 0
		c.HasTitle = f&0x40 != 0
		var err error
		if c.ID, err = str(); err != nil {
			return err
		}
		c.Name, c.Title = c.ID, c.ID
		if c.HasName {
			if c.Name, err = str(); err != nil {
				return err
			}
		}
		if c.HasTitle {
			if c.Title, err = str(); err != nil {
				return err
			}
		}
	}
	return nil
}

// ParseDir decodes a DIRM chunk. On a BZZ error it still returns the
// header part.
func ParseDir(b []byte) (*Dir, error) {
	return parseDir(b, ivy.Limits{})
}

// parseDir is ParseDir within lim.MaxMemory.
//
// ivy: added.
func parseDir(b []byte, lim ivy.Limits) (*Dir, error) {
	d, rest, err := parseDirHeader(b)
	if err != nil {
		return nil, err
	}
	body, err := unbzz(rest, lim)
	if err != nil {
		return d, fmt.Errorf("djvu: DIRM: %w", err)
	}
	if err := d.parseDirBody(body); err != nil {
		return d, err
	}
	return d, nil
}
