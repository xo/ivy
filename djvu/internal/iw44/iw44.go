// Copyright (c) 2026 Serge Vakulenko
// SPDX-License-Identifier: MIT

// Copied from github.com/sergev/dejapdf, file internal/iw44/iw44.go, at
// commit 2b6ca0c1123c (D7, D26, docs/UPSTREAM.md). Its MIT license is in
// djvu/internal/LICENSE.

package iw44

import (
	"errors"
	"fmt"

	"github.com/xo/ivy/djvu/internal/zp"
)

// Image is a decoded layer, rows top to bottom: one byte per pixel if
// Gray, else R, G, B.
type Image struct {
	W, H int
	Gray bool
	Pix  []byte
}

// Band layout (Table 2): first coefficient and bucket count of each band.
var (
	bandStart   = [10]int{0, 16, 32, 48, 64, 128, 192, 256, 512, 768}
	bandBuckets = [10]int{1, 1, 1, 1, 4, 4, 4, 16, 16, 16}
)

// stepIndex maps a coefficient index to its step size table entry
// (Table 3).

// maxPixels limits an image: an A3 page at 600 dpi is 7016×9921, 2^26.06.
const maxPixels = 1 << 27

func stepIndex(i int) int {
	switch {
	case i < 4:
		return i
	case i < 16:
		return 4 + (i-4)/4
	case i < 64:
		return 7 + (i-16)/16
	case i < 256:
		return 10 + (i-64)/64
	}
	return 13 + (i-256)/256
}

// stepOf caches stepIndex.
var stepOf = func() (t [1024]uint8) {
	for i := range t {
		t[i] = uint8(stepIndex(i))
	}
	return
}()

// Initial step sizes (Table 4).
var initialStep = [16]int32{
	0x04000, 0x08000, 0x08000, 0x10000, 0x10000, 0x10000, 0x20000, 0x20000,
	0x20000, 0x40000, 0x40000, 0x40000, 0x80000, 0x40000, 0x40000, 0x80000,
}

// component is the state of one colour component (Y, Cb or Cr).
type component struct {
	coef     [][1024]int16 // per block, 6 fraction bits
	step     [16]int32
	band     int
	ctxBlock byte
	ctxBuck  [10][8]byte
	ctxAct   [16]byte
	ctxRef   byte
	live     bool // some step of the current band is in range
}

// Decoder decodes the chunks of one layer (all BG44, or one FG44).
type Decoder struct {
	started    bool
	serial     int
	gray       bool
	chromaHalf bool // DjVuLibre's crcb_half: chroma at half resolution
	w, h       int
	bw, bh     int // blocks across and up
	delay      int
	comps      []*component
	zp         *zp.Decoder
	potent     [1024]bool
	active     [1024]bool
	decoding   [64]bool
}

// Size returns the image size from the first chunk's header.
func Size(b []byte) (w, h int, ok bool) {
	if len(b) < 9 || b[0] != 0 {
		return 0, 0, false
	}
	return int(b[4])<<8 | int(b[5]), int(b[6])<<8 | int(b[7]), true
}

// Decode adds one chunk.
func (d *Decoder) Decode(b []byte) error {
	if len(b) < 2 {
		return errors.New("iw44: chunk too short")
	}
	serial, slices := int(b[0]), int(b[1])
	if serial != d.serial {
		return fmt.Errorf("iw44: chunk serial %d, want %d", serial, d.serial)
	}
	b = b[2:]
	if serial == 0 {
		if len(b) < 7 {
			return errors.New("iw44: header too short")
		}
		if major := b[0] & 0x7f; major != 1 {
			return fmt.Errorf("iw44: version %d", major)
		}
		d.gray = b[0]&0x80 != 0
		d.w = int(b[2])<<8 | int(b[3])
		d.h = int(b[4])<<8 | int(b[5])
		if d.w == 0 || d.h == 0 || d.w*d.h > maxPixels {
			return fmt.Errorf("iw44: size %dx%d", d.w, d.h)
		}
		// From minor version 2: chroma delay; a clear bit 7 halves
		// the chroma resolution.
		if minor := b[1]; minor >= 2 {
			d.delay = int(b[6] & 0x7f)
			d.chromaHalf = !d.gray && b[6]&0x80 == 0
		}
		d.bw, d.bh = (d.w+31)/32, (d.h+31)/32
		n := 3
		if d.gray {
			n = 1
		}
		for range n {
			c := &component{coef: make([][1024]int16, d.bw*d.bh), step: initialStep}
			c.setLive()
			d.comps = append(d.comps, c)
		}
		d.started = true
		b = b[7:]
	}
	if !d.started {
		return errors.New("iw44: no first chunk")
	}
	d.serial++
	// Past the end of b the ZP decoder reads 0xff, as DjVuLibre does;
	// encoders rely on it.
	d.zp = zp.NewDecoder(b)
	for range slices {
		n := 1
		if d.delay == 0 {
			n = len(d.comps)
		}
		for _, c := range d.comps[:n] {
			for blk := range c.coef {
				d.blockBand(c, &c.coef[blk])
			}
			c.endSlice()
		}
		if d.delay > 0 {
			d.delay--
		}
	}
	return nil
}

// endSlice halves the steps of the current band (Table 5) and moves to
// the next band.
func (c *component) endSlice() {
	lo, hi := c.band+6, c.band+6
	if c.band == 0 {
		lo = 0
	}
	for i := lo; i <= hi; i++ {
		c.step[i] >>= 1
	}
	c.band = (c.band + 1) % 10
	c.setLive()
}

// setLive notes whether the current band has a step in range.
func (c *component) setLive() {
	lo, hi := c.band+6, c.band+6
	if c.band == 0 {
		lo = 0
	}
	c.live = false
	for i := lo; i <= hi; i++ {
		if s := c.step[i]; s > 0 && s < 0x8000 {
			c.live = true
		}
	}
}

// blockBand decodes the current band of one block (10.5).
func (d *Decoder) blockBand(c *component, co *[1024]int16) {
	if !c.live {
		return
	}
	band := c.band
	first, nb := bandStart[band], bandBuckets[band]
	// Flags (10.5.1).
	var bActive, bPotent bool
	for i := first; i < first+16*nb; i++ {
		s := c.step[stepOf[i]]
		ok := s > 0 && s < 0x8000
		d.active[i] = ok && co[i] != 0
		d.potent[i] = ok && co[i] == 0
		bActive = bActive || d.active[i]
		bPotent = bPotent || d.potent[i]
	}
	if !bActive && !bPotent {
		return
	}
	act := 0
	if bActive {
		act = 1
	}
	// Pass 1: any new coefficients in the block band?
	if nb < 16 || bActive || d.zp.Decode(&c.ctxBlock) == 1 {
		for k := range nb {
			d.bucket(c, co, band, first, k, act)
		}
		for k := range nb {
			d.activate(c, co, first, k)
		}
	}
	d.refine(c, co, first, nb)
}

// bucket decodes whether bucket k has new coefficients (pass 2). The
// context counts the non-zero coefficients among 4b..4b+3, the parents
// of bucket b (the spec says zero ones).
func (d *Decoder) bucket(c *component, co *[1024]int16, band, first, k, act int) {
	d.decoding[k] = false
	b0 := first + 16*k
	pot := false
	for i := b0; i < b0+16; i++ {
		pot = pot || d.potent[i]
	}
	if !pot {
		return
	}
	n := 0
	if band > 0 {
		t := 4 * (b0 / 16)
		for i := t; i < t+4; i++ {
			if co[i] != 0 {
				n++
			}
		}
		n = min(n, 3)
	}
	d.decoding[k] = d.zp.Decode(&c.ctxBuck[band][n+4*act]) == 1
}

// activate decodes the new coefficients of bucket k (pass 3). The
// context depends on the bucket's ACTIVE flag (the spec says the
// block's). A new coefficient gets 11S/8, not 3S/2 as the spec says.
func (d *Decoder) activate(c *component, co *[1024]int16, first, k int) {
	if !d.decoding[k] {
		return
	}
	b0 := first + 16*k
	np, act := 0, 0
	for i := b0; i < b0+16; i++ {
		if d.potent[i] {
			np++
		}
		if d.active[i] {
			act = 8
		}
	}
	for i := b0; i < b0+16; i++ {
		if !d.potent[i] {
			continue
		}
		if d.zp.Decode(&c.ctxAct[min(7, np)+act]) == 1 {
			s := c.step[stepOf[i]]
			a := s + s>>1 - s>>3
			if d.zp.DecodePassthroughIW44() == 1 {
				a = -a
			}
			co[i] = int16(a)
			np = 0
		}
		if np > 0 {
			np--
		}
	}
}

// refine decodes one more bit of each coefficient active before this
// band (pass 4). The first refinement, the one with a context, first
// moves the value up by S/4 to its interval's midpoint.
func (d *Decoder) refine(c *component, co *[1024]int16, first, nb int) {
	for i := first; i < first+16*nb; i++ {
		if !d.active[i] {
			continue
		}
		s := c.step[stepOf[i]]
		a := int32(co[i])
		neg := a < 0
		if neg {
			a = -a
		}
		var up int
		if a <= 3*s {
			up = d.zp.Decode(&c.ctxRef)
			a += s >> 2
		} else {
			up = d.zp.DecodePassthroughIW44()
		}
		if up == 1 {
			a += s >> 1
		} else {
			a += s>>1 - s
		}
		if neg {
			a = -a
		}
		co[i] = int16(a)
	}
}
