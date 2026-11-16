// Copyright (c) 2026 Serge Vakulenko
// SPDX-License-Identifier: MIT

// Copied from github.com/sergev/dejapdf, file internal/djvu/sexp.go, at
// commit 2b6ca0c1123c (D7, D26, docs/UPSTREAM.md). Its MIT license is in
// djvu/internal/LICENSE.

package doc // ivy: renamed from djvu, the name of the package of the format

import (
	"strconv"
	"strings"
)

// Sexp is an annotation S-expression (spec 8.3.4): a list or an atom.
type Sexp struct {
	IsList bool
	List   []*Sexp
	Atom   string // symbol or number text, or string contents
	Quoted bool   // Atom was a quoted string
}

// Head returns the symbol a list starts with, or "".
func (s *Sexp) Head() string {
	if !s.IsList || len(s.List) == 0 || s.List[0].IsList || s.List[0].Quoted {
		return ""
	}
	return s.List[0].Atom
}

// String formats s on one line, strings with C escapes.
func (s *Sexp) String() string {
	var b strings.Builder
	s.write(&b)
	return b.String()
}

func (s *Sexp) write(b *strings.Builder) {
	switch {
	case s.IsList:
		b.WriteByte('(')
		for i, k := range s.List {
			if i > 0 {
				b.WriteByte(' ')
			}
			k.write(b)
		}
		b.WriteByte(')')
	case s.Quoted:
		b.WriteByte('"')
		for i := 0; i < len(s.Atom); i++ {
			switch c := s.Atom[i]; c {
			case '"', '\\':
				b.WriteByte('\\')
				b.WriteByte(c)
			case '\n':
				b.WriteString(`\n`)
			case '\t':
				b.WriteString(`\t`)
			default:
				if c < 0x20 || c == 0x7f {
					b.WriteString(`\` + strconv.FormatInt(int64(c)+0o1000, 8)[1:])
				} else {
					b.WriteByte(c)
				}
			}
		}
		b.WriteByte('"')
	default:
		b.WriteString(s.Atom)
	}
}

const maxSexpDepth = 64

// ParseSexps reads a sequence of S-expressions. Like viewers, it
// drops what it cannot read: a broken top-level expression ends the
// input, and stray ')' are skipped.
func ParseSexps(b []byte) []*Sexp {
	p := &sexpParser{b: b}
	var r []*Sexp
	for {
		p.space()
		if p.i >= len(p.b) {
			return r
		}
		if p.b[p.i] == ')' {
			p.i++
			continue
		}
		s, ok := p.expr(0)
		if !ok {
			return r
		}
		r = append(r, s)
	}
}

type sexpParser struct {
	b []byte
	i int
}

func isSpace(c byte) bool {
	return c == ' ' || c == '\t' || c == '\n' || c == '\r' || c == '\f' || c == '\v' || c == 0
}

func (p *sexpParser) space() {
	for p.i < len(p.b) && isSpace(p.b[p.i]) {
		p.i++
	}
}

func (p *sexpParser) expr(depth int) (*Sexp, bool) {
	if depth > maxSexpDepth {
		return nil, false
	}
	switch p.b[p.i] {
	case '(':
		p.i++
		s := &Sexp{IsList: true}
		for {
			p.space()
			if p.i >= len(p.b) {
				return nil, false
			}
			if p.b[p.i] == ')' {
				p.i++
				return s, true
			}
			k, ok := p.expr(depth + 1)
			if !ok {
				return nil, false
			}
			s.List = append(s.List, k)
		}
	case '"':
		return p.str()
	}
	j := p.i
	for p.i < len(p.b) {
		c := p.b[p.i]
		if isSpace(c) || c == '(' || c == ')' || c == '"' {
			break
		}
		p.i++
	}
	return &Sexp{Atom: string(p.b[j:p.i])}, true
}

// str reads a quoted string with C escapes.
func (p *sexpParser) str() (*Sexp, bool) {
	p.i++
	var out []byte
	for p.i < len(p.b) {
		c := p.b[p.i]
		p.i++
		switch c {
		case '"':
			return &Sexp{Atom: string(out), Quoted: true}, true
		case '\\':
			if p.i >= len(p.b) {
				return nil, false
			}
			c = p.b[p.i]
			p.i++
			switch c {
			case 'n':
				out = append(out, '\n')
			case 't':
				out = append(out, '\t')
			case 'r':
				out = append(out, '\r')
			case 'b':
				out = append(out, '\b')
			case 'f':
				out = append(out, '\f')
			case 'v':
				out = append(out, '\v')
			case 'a':
				out = append(out, '\a')
			case '0', '1', '2', '3', '4', '5', '6', '7':
				v := int(c - '0')
				for k := 0; k < 2 && p.i < len(p.b) && p.b[p.i] >= '0' && p.b[p.i] <= '7'; k++ {
					v = v*8 + int(p.b[p.i]-'0')
					p.i++
				}
				out = append(out, byte(v))
			default: // \" \\ and unknown escapes: the character itself
				out = append(out, c)
			}
		default:
			out = append(out, c)
		}
	}
	return nil, false
}
