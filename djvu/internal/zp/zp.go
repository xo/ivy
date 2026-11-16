// Copyright (c) 2026 Serge Vakulenko
// SPDX-License-Identifier: MIT

// Copied from github.com/sergev/dejapdf, file internal/zp/zp.go, at
// commit 2b6ca0c1123c (D7, D26, docs/UPSTREAM.md). Its MIT license is in
// djvu/internal/LICENSE.

// Package zp implements the DjVu ZP-coder (adaptive binary arithmetic decoder),
// following the DjVu v3 spec, appendix 3.
package zp

type state struct {
	delta, theta uint16
	mu, lambda   uint8
}

// Decoder decodes one arithmetically coded stream. A context is a byte
// holding a probability state index; contexts start at 0.
type Decoder struct {
	a, c  int // spec registers A and C
	data  []byte
	pos   int
	cur   byte
	nbits uint
}

// NewDecoder starts decoding b. Past the end of b, input bits are 1.
func NewDecoder(b []byte) *Decoder {
	d := &Decoder{data: b}
	d.c = int(d.byte())<<8 | int(d.byte())
	return d
}

// Overrun returns how many bytes past the end of the input were read.
func (d *Decoder) Overrun() int {
	return max(d.pos-len(d.data), 0)
}

func (d *Decoder) byte() byte {
	if d.pos >= len(d.data) {
		d.pos++
		return 0xff
	}
	d.pos++
	return d.data[d.pos-1]
}

func (d *Decoder) bit() int {
	if d.nbits == 0 {
		d.cur = d.byte()
		d.nbits = 8
	}
	d.nbits--
	return int(d.cur>>d.nbits) & 1
}

func (d *Decoder) renorm() {
	for d.a >= 0x8000 {
		d.a = d.a + d.a - 0x10000
		d.c = d.c + d.c - 0x10000 + d.bit()
	}
}

// TryDecode decodes an MPS that needs no renormalisation, as Decode
// would, returning true; otherwise it does nothing, and the caller must
// use Decode. It inlines, for hot loops.
func (d *Decoder) TryDecode(ctx *byte) (int, bool) {
	v := *ctx
	if z := d.a + int(table[v].delta); z < 0x8000 && z <= d.c {
		d.a = z
		return int(v & 1), true
	}
	return 0, false
}

// Decode decodes one bit with context ctx and updates its state.
//
// This is figure 1 of the spec with two corrections, found by decoding
// streams from DjVuLibre's bzz: an MPS that needs no renormalisation
// leaves the state alone, and C == Z decodes as MPS.
func (d *Decoder) Decode(ctx *byte) int {
	s := &table[*ctx]
	z := d.a + int(s.delta)
	if z < 0x8000 && z <= d.c {
		d.a = z
		return int(*ctx & 1) // MPS, no adaptation
	}
	if lim := 0x6000 + (z+d.a)>>2; z > lim {
		z = lim
	}
	var b int
	if d.c >= z {
		b = int(*ctx & 1) // MPS
		if d.a >= int(s.theta) {
			*ctx = s.mu
		}
		d.a = z
	} else {
		b = 1 - int(*ctx&1) // LPS
		d.a += 0x10000 - z
		d.c += 0x10000 - z
		*ctx = s.lambda
	}
	d.renorm()
	return b
}

// DecodePassthrough decodes one bit with probability 1/2. Figure 2 of
// the spec gives Z = 0x8000 + 3A/8; DjVuLibre streams need 0x8000 + A/2.
func (d *Decoder) DecodePassthrough() int {
	return d.passthrough(0x8000 + d.a>>1)
}

// DecodePassthroughIW44 is the pass-through variant used by IW44,
// with figure 2's threshold Z = 0x8000 + 3A/8.
func (d *Decoder) DecodePassthroughIW44() int {
	return d.passthrough(0x8000 + (d.a+d.a+d.a)>>3)
}

func (d *Decoder) passthrough(z int) int {
	var b int
	if d.c >= z {
		d.a = z
	} else {
		b = 1
		d.a += 0x10000 - z
		d.c += 0x10000 - z
	}
	d.renorm()
	return b
}
