// Copyright (c) 2026 Serge Vakulenko
// SPDX-License-Identifier: MIT

// Copied from github.com/sergev/dejapdf, file internal/iw44/image.go, at
// commit 2b6ca0c1123c (D7, D26, docs/UPSTREAM.md). Its MIT license is in
// djvu/internal/LICENSE.

package iw44

import "errors"

// zigzag maps a coefficient index to its row and column in a block
// (10.6.3): row bits b1 b3 b5 b7 b9, column bits b0 b2 b4 b6 b8, the
// first bit most significant. Row 0 is the bottom.
var zigzagRow, zigzagCol [1024]int

func init() {
	for i := range 1024 {
		r, c := 0, 0
		for k := range 5 {
			r |= (i >> (2*k + 1) & 1) << (4 - k)
			c |= (i >> (2 * k) & 1) << (4 - k)
		}
		zigzagRow[i], zigzagCol[i] = r, c
	}
}

// Image reconstructs the layer from the chunks decoded so far.
func (d *Decoder) Image() (*Image, error) {
	if !d.started {
		return nil, errors.New("iw44: no data")
	}
	w, h := d.w, d.h
	im := &Image{W: w, H: h, Gray: d.gray}
	y := d.plane(d.comps[0], 1)
	if d.gray {
		im.Pix = make([]byte, w*h)
		for r := range h {
			src := y[(h-1-r)*w:] // planes are bottom-up
			for x := range w {
				im.Pix[r*w+x] = byte(127 - reduce(src[x]))
			}
		}
		return im, nil
	}
	// Half-resolution chroma stops at scale 2 and covers 2x2 pixels.
	cs := 1
	if d.chromaHalf {
		cs = 2
	}
	cb, cr := d.plane(d.comps[1], cs), d.plane(d.comps[2], cs)
	im.Pix = make([]byte, 3*w*h)
	for r := range h {
		yr := h - 1 - r
		src := y[yr*w:]
		crow := (yr / cs * cs) * w
		for x := range w {
			cx := crow + x/cs*cs
			Y := reduce(src[x])
			b, rr := reduce(cb[cx]), reduce(cr[cx])
			// Spec 10.6.7 with DjVuLibre's rounding.
			t2 := rr + rr>>1
			t3 := Y + 128 - b>>2
			p := im.Pix[3*(r*w+x):]
			p[0] = clamp(Y + 128 + t2)
			p[1] = clamp(t3 - t2>>1)
			p[2] = clamp(t3 + b<<1)
		}
	}
	return im, nil
}

// ImageScaled reconstructs the layer at 1/s of its size, for s a power
// of 2 up to 16, as a viewer does for a reduced view. It inverts the
// wavelet transform only down to scale s, and keeps the pixels whose
// column and whose row from the bottom are multiples of s. The result is
// (W+s-1)/s by (H+s-1)/s pixels. An s of 1 or less is Image.
//
// ivy: added, so that a page that ivy draws small decodes its background
// at the size that it needs. IW44 is progressive, so the coarse scales do
// not depend on the fine ones.
func (d *Decoder) ImageScaled(s int) (*Image, error) {
	if s <= 1 {
		return d.Image()
	}
	if !d.started {
		return nil, errors.New("iw44: no data")
	}
	if s > 16 || s&(s-1) != 0 {
		return nil, errors.New("iw44: scale is not a power of 2 up to 16")
	}
	w, h := d.w, d.h
	sw, sh := (w+s-1)/s, (h+s-1)/s
	im := &Image{W: sw, H: sh, Gray: d.gray}
	y := d.plane(d.comps[0], s)
	if d.gray {
		im.Pix = make([]byte, sw*sh)
		for r := range sh {
			src := y[(sh-1-r)*s*w:] // planes are bottom-up
			for x := range sw {
				im.Pix[r*sw+x] = byte(127 - reduce(src[x*s]))
			}
		}
		return im, nil
	}
	// Half-resolution chroma stops at scale 2, so a scale of 2 or more
	// reads it as the luminance.
	cb, cr := d.plane(d.comps[1], s), d.plane(d.comps[2], s)
	im.Pix = make([]byte, 3*sw*sh)
	for r := range sh {
		row := (sh - 1 - r) * s * w
		for x := range sw {
			i := row + x*s
			Y := reduce(y[i])
			b, rr := reduce(cb[i]), reduce(cr[i])
			t2 := rr + rr>>1
			t3 := Y + 128 - b>>2
			p := im.Pix[3*(r*sw+x):]
			p[0] = clamp(Y + 128 + t2)
			p[1] = clamp(t3 - t2>>1)
			p[2] = clamp(t3 + b<<1)
		}
	}
	return im, nil
}

// reduce rounds away the 6 fraction bits and clamps to -128..127.
func reduce(v int16) int32 {
	return min(max((int32(v)+32)>>6, -128), 127)
}

func clamp(v int32) byte {
	return byte(min(max(v, 0), 255))
}

// plane places a component's coefficients and inverts the wavelet
// transform down to scale smin. The result is w*h, bottom row first.
// Like DjVuLibre, it keeps samples as 16-bit values.
func (d *Decoder) plane(c *component, smin int) []int16 {
	w, h := d.w, d.h
	p := make([]int16, w*h)
	for by := range d.bh {
		for bx := range d.bw {
			co := &c.coef[by*d.bw+bx]
			for i, v := range co {
				if v == 0 {
					continue
				}
				y, x := 32*by+zigzagRow[i], 32*bx+zigzagCol[i]
				if x < w && y < h {
					p[y*w+x] = v
				}
			}
		}
	}
	buf := make([]int16, max(w, h))
	for s := 16; s >= smin; s /= 2 {
		for x := 0; x < w; x += s { // columns
			n := (h-1)/s + 1
			for k := range n {
				buf[k] = p[k*s*w+x]
			}
			lift(buf[:n], false)
			for k := range n {
				p[k*s*w+x] = buf[k]
			}
		}
		for y := 0; y < h; y += s { // rows
			n := (w-1)/s + 1
			row := p[y*w:]
			for k := range n {
				buf[k] = row[k*s]
			}
			lift(buf[:n], true)
			for k := range n {
				row[k*s] = buf[k]
			}
		}
	}
	return p
}

// lift inverts one level of the (4,4) transform on c (10.6.4): lifting
// on the even samples, then prediction on the odd ones. Missing
// neighbours are 0, except that for rows, near the end, the lifting step
// keeps rolling its neighbour registers as DjVuLibre does: at k = 2 and
// 4, a missing c[k+3] repeats c[k+1].
func lift(c []int16, row bool) {
	kmax := len(c) - 1
	var p3, p1, n1, n3 int32
	if kmax >= 1 {
		n3 = int32(c[1])
	}
	for k := 0; k <= kmax; k += 2 {
		p3, p1, n1 = p1, n1, n3
		switch {
		case k+3 <= kmax:
			n3 = int32(c[k+3])
		case row && (k == 2 || k == 4):
			n3 = n1
		default:
			n3 = 0
		}
		c[k] = int16(int32(c[k]) - (9*(p1+n1)-(p3+n3)+16)>>5)
	}
	for k := 1; k <= kmax; k += 2 {
		v := int32(c[k])
		switch {
		case k >= 3 && k+3 <= kmax:
			a, b := int32(c[k-1])+int32(c[k+1]), int32(c[k-3])+int32(c[k+3])
			v += (9*a - b + 8) >> 4
		case k+1 <= kmax:
			v += (int32(c[k-1]) + int32(c[k+1]) + 1) >> 1
		default:
			v += int32(c[k-1])
		}
		c[k] = int16(v)
	}
}
