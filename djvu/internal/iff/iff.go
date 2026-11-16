// Copyright (c) 2026 Serge Vakulenko
// SPDX-License-Identifier: MIT

// Copied from github.com/sergev/dejapdf, file internal/iff/iff.go, at
// commit 2b6ca0c1123c (D7, D26, docs/UPSTREAM.md). Its MIT license is in
// djvu/internal/LICENSE.

// Package iff parses the IFF85 chunk structure of DjVu files.
package iff

import (
	"encoding/binary"
	"fmt"
)

// Chunk is one IFF chunk. Offsets are absolute within the file.
type Chunk struct {
	ID       string   // "FORM" or a data chunk id like "INFO"
	Type     string   // secondary id of a composite chunk, e.g. "DJVU"
	Offset   int      // offset of the chunk header
	Data     []byte   // payload; for a composite chunk, the bytes after Type
	Children []*Chunk // sub-chunks of a composite chunk
}

// Composite reports whether the chunk contains sub-chunks.
func (c *Chunk) Composite() bool { return isComposite(c.ID) }

// Name returns "FORM:DJVU" for composite chunks, else the id.
func (c *Chunk) Name() string {
	if c.Composite() {
		return c.ID + ":" + c.Type
	}
	return c.ID
}

// Size returns the length field of the chunk header.
func (c *Chunk) Size() int {
	if c.Composite() {
		return len(c.Data) + 4
	}
	return len(c.Data)
}

// Find returns the first child with the given id (or "FORM:TYPE" name).
// A nil chunk has no children.
func (c *Chunk) Find(id string) *Chunk {
	if c == nil {
		return nil
	}
	for _, k := range c.Children {
		if k.ID == id || k.Name() == id {
			return k
		}
	}
	return nil
}

// All returns all children with the given id (or "FORM:TYPE" name).
func (c *Chunk) All(id string) []*Chunk {
	var r []*Chunk
	if c == nil {
		return nil
	}
	for _, k := range c.Children {
		if k.ID == id || k.Name() == id {
			r = append(r, k)
		}
	}
	return r
}

func isComposite(id string) bool {
	switch id {
	case "FORM", "LIST", "PROP", "CAT ":
		return true
	}
	return false
}

// Parse parses a whole file: optional "AT&T" magic, then one chunk.
// A damaged file gives an error and, if its first chunk header is
// intact, the chunks that could be read: one running past its
// container is cut short, and a composite stops at a bad child.
func Parse(b []byte) (*Chunk, error) {
	off := 0
	if len(b) >= 4 && string(b[:4]) == "AT&T" {
		off = 4
	}
	return ParseAt(b, off)
}

// ParseAt parses the chunk whose header starts at off.
func ParseAt(b []byte, off int) (*Chunk, error) {
	c, _, err := parse(b, off, len(b), 0)
	return c, err
}

const maxDepth = 32

// parse reads one chunk at off that must end by limit; returns the offset
// past it. On damage, the chunk may come back with an error.
func parse(b []byte, off, limit, depth int) (*Chunk, int, error) {
	if depth > maxDepth {
		return nil, 0, fmt.Errorf("iff: chunks nested too deep at offset %d", off)
	}
	if limit-off < 8 {
		return nil, 0, fmt.Errorf("iff: truncated chunk header at offset %d", off)
	}
	id := string(b[off : off+4])
	n := int(binary.BigEndian.Uint32(b[off+4 : off+8]))
	start := off + 8
	var damage error
	if n > limit-start {
		damage = fmt.Errorf("iff: chunk %q at offset %d: size %d exceeds container", id, off, n)
		n = limit - start
	}
	end := start + n
	c := &Chunk{ID: id, Offset: off, Data: b[start:end]}
	if isComposite(id) {
		if n < 4 {
			return nil, 0, fmt.Errorf("iff: %s at offset %d: missing type", id, off)
		}
		c.Type = string(b[start : start+4])
		c.Data = b[start+4 : end]
		for p := start + 4; p < end; {
			k, next, err := parse(b, p, end, depth+1)
			if k != nil {
				c.Children = append(c.Children, k)
			}
			if err != nil {
				if damage == nil {
					damage = err
				}
				if k == nil {
					break
				}
			}
			p = next
			if p&1 == 1 && p < end {
				p++ // pad byte
			}
		}
	}
	return c, end, damage
}
