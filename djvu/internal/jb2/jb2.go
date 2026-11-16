// Copyright (c) 2026 Serge Vakulenko
// SPDX-License-Identifier: MIT

// Copied from github.com/sergev/dejapdf, file internal/jb2/jb2.go, at
// commit 2b6ca0c1123c (D7, D26, docs/UPSTREAM.md). Its MIT license is in
// djvu/internal/LICENSE.

package jb2

import (
	"bytes"
	"errors"
	"fmt"

	"github.com/xo/ivy"
	"github.com/xo/ivy/djvu/internal/zp"
)

// Bitmap is a bilevel image, rows top to bottom, one byte per pixel,
// 1 = black.
type Bitmap struct {
	W, H int
	Pix  []byte
}

func newBitmap(w, h int) *Bitmap {
	return &Bitmap{W: w, H: h, Pix: make([]byte, w*h)}
}

func (b *Bitmap) at(r, c int) int {
	if r < 0 || c < 0 || r >= b.H || c >= b.W {
		return 0
	}
	return int(b.Pix[r*b.W+c])
}

// Dict is a shape library, from a Djbz chunk.
type Dict struct {
	Shapes []*Bitmap
	Refs   []Ref // how each shape was coded
}

// Ref tells how a shape was coded: Shape < 0 for directly, else as a
// refinement of shape Shape, whose pixel (x−DX, y−DY) lay over this
// shape's (x, y), rows from the top.
type Ref struct {
	Shape, DX, DY int
}

// Blit draws a shape with its lower-left corner at (X, Y): pixels from
// the bottom left of the image, 0-based.
type Blit struct {
	Shape, X, Y int
}

// Image is a decoded Sjbz chunk. Blit.Shape below NDict is a shape of
// Dict, otherwise Shapes[Shape-NDict]. Shapes are trimmed of empty
// edge rows and columns, and may be empty (0x0). Blits are in record
// order.
type Image struct {
	W, H   int
	Dict   *Dict
	NDict  int       // shapes of Dict in use (record 9)
	Shapes []*Bitmap // the page's own shapes, in the library or not
	Refs   []Ref     // how each of Shapes was coded
	Blits  []Blit
}

// ShapeRef returns how shape i was coded.
func (im *Image) ShapeRef(i int) Ref {
	if i < im.NDict {
		return im.Dict.Refs[i]
	}
	return im.Refs[i-im.NDict]
}

// Shape returns the bitmap a blit index refers to.
func (im *Image) Shape(i int) *Bitmap {
	if i < im.NDict {
		return im.Dict.Shapes[i]
	}
	return im.Shapes[i-im.NDict]
}

// Render draws the image.
func (im *Image) Render() *Bitmap {
	return im.RenderSome(im.W, im.H, nil)
}

// RenderSome draws the blits i with keep(i) true (all if keep is nil)
// into a w×h bitmap whose bottom-left corner is the image's.
func (im *Image) RenderSome(w, h int, keep func(i int) bool) *Bitmap {
	p := newBitmap(w, h)
	for i, b := range im.Blits {
		if keep != nil && !keep(i) {
			continue
		}
		s := im.Shape(b.Shape)
		top := h - b.Y - s.H // bitmap row of the shape's top row
		for r := range s.H {
			pr := top + r
			if pr < 0 || pr >= p.H {
				continue
			}
			for c := range s.W {
				pc := b.X + c
				if pc >= 0 && pc < p.W && s.Pix[r*s.W+c] != 0 {
					p.Pix[pr*p.W+pc] = 1
				}
			}
		}
	}
	return p
}

// Integer contexts (Table 7).
const (
	numRecord = iota
	numImageSize
	numMatch
	numWidth
	numHeight
	numWidthDiff
	numHeightDiff
	numCol
	numRow
	numSameCol
	numSameRow
	numNewCol
	numNewRow
	numCommentLen
	numCommentByte
	numDictSize
	numCount
)

// Record types (Table 6).
const (
	recStart = iota
	recNewLibImage
	recNewLib
	recNewImage
	recRefLibImage
	recRefLib
	recRefImage
	recCopy
	recNonSymbol
	recDictOrReset
	recComment
	recEnd
)

const (
	maxSize    = 262142
	maxPixels  = 1 << 26
	maxPage    = 1 << 27 // pixels of a page: A3 at 600 dpi is 2^26.06
	maxOver    = 64      // bytes read past the end before giving up
	maxRecords = 1 << 22
)

type libEntry struct {
	bm    *Bitmap
	shape int // blit index
}

type decoder struct {
	zp         *zp.Decoder
	num        [numCount]numCoder
	refineFlag byte
	offsetType byte
	direct     [1024]byte
	refine     [2048]byte

	im     *Image
	lib    []libEntry
	budget int // pixels left to decode

	// Current text line (spec 11.5.8), in spec coordinates: column 1 at
	// the left, row 1 at the bottom.
	firstLeft, firstBottom int
	prevRight              int
	bottoms                [3]int // most recent first
	onLine                 int

	scratch []byte // padded bitmaps, reused

	lim ivy.Limits // ivy: the limits of the caller
}

// buffers returns two scratch slices of n and m bytes, the first zeroed.
func (d *decoder) buffers(n, m int) ([]byte, []byte) {
	if cap(d.scratch) < n+m {
		d.scratch = make([]byte, n+m)
	}
	b := d.scratch[:n+m]
	clear(b[:n])
	return b[:n:n], b[n:]
}

// Decode decodes an Sjbz chunk. dict is the shared dictionary the
// page includes, or nil.
func Decode(data []byte, dict *Dict) (*Image, error) {
	return DecodeWithin(data, dict, ivy.Limits{})
}

// DecodeWithin is Decode within the limits of ivy.
//
// ivy: added. lim.MaxPixels bounds the size of the page, lim.MaxMemory the
// pixels of the bitmaps that the stream decodes, and lim.MaxObjects its
// shapes and blits. An error past a limit wraps ivy.ErrLimit.
func DecodeWithin(data []byte, dict *Dict, lim ivy.Limits) (*Image, error) {
	r, err := decode(data, dict, lim)
	return r.Image, err
}

// DecodeDict decodes a Djbz chunk. parent is the dictionary it
// includes, if any.
func DecodeDict(data []byte, parent *Dict) (*Dict, error) {
	return DecodeDictWithin(data, parent, ivy.Limits{})
}

// DecodeDictWithin is DecodeDict within the limits of ivy, as
// DecodeWithin.
//
// ivy: added.
func DecodeDictWithin(data []byte, parent *Dict, lim ivy.Limits) (*Dict, error) {
	r, err := decode(data, parent, lim)
	if err != nil {
		return nil, err
	}
	d := &Dict{}
	pos := map[int]int{} // shape number → library position
	for i, e := range r.lib {
		pos[e.shape] = i
	}
	for _, e := range r.lib {
		d.Shapes = append(d.Shapes, e.bm)
		ref := r.ShapeRef(e.shape)
		if ref.Shape >= 0 {
			ref.Shape = pos[ref.Shape] // a reference is always in the library
		}
		d.Refs = append(d.Refs, ref)
	}
	return d, nil
}

type result struct {
	*Image
	lib []libEntry
}

func decode(data []byte, dict *Dict, lim ivy.Limits) (result, error) {
	lim = lim.WithDefaults()
	d := &decoder{zp: zp.NewDecoder(data), im: &Image{Dict: dict},
		budget: 1<<27 + 1024*len(data), lim: lim}
	// ivy: the bitmaps take a byte for each pixel, so the memory of the
	// caller also bounds the pixels.
	d.budget = int(min(int64(d.budget), lim.MaxMemory))
	started := false
	for range maxRecords {
		if d.zp.Overrun() > maxOver {
			return result{}, errors.New("jb2: truncated stream")
		}
		// ivy: a guard against a stream of many small records.
		if len(d.im.Shapes)+len(d.im.Blits) > lim.MaxObjects {
			return result{}, fmt.Errorf("jb2: more than %d shapes and blits: %w", lim.MaxObjects, ivy.ErrLimit)
		}
		t := d.num[numRecord].decode(d.zp, recStart, recEnd)
		if !started && t != recStart && t != recDictOrReset && t != recComment {
			return result{}, fmt.Errorf("jb2: record %d before start", t)
		}
		var err error
		switch t {
		case recStart:
			if started {
				return result{}, errors.New("jb2: second start record")
			}
			started = true
			d.im.W = d.num[numImageSize].decode(d.zp, 0, maxSize)
			d.im.H = d.num[numImageSize].decode(d.zp, 0, maxSize)
			if d.im.W*d.im.H > maxPage {
				return result{}, fmt.Errorf("jb2: page %dx%d", d.im.W, d.im.H)
			}
			if int64(d.im.W)*int64(d.im.H) > d.lim.MaxPixels { // ivy: the limit of the caller
				return result{}, fmt.Errorf("jb2: page %dx%d: %w", d.im.W, d.im.H, ivy.ErrLimit)
			}
			d.zp.Decode(&d.refineFlag)
			d.firstLeft, d.firstBottom = 0, d.im.H
			d.prevRight = 0
			d.onLine = 0
		case recNewLibImage, recNewLib, recNewImage:
			w := d.num[numWidth].decode(d.zp, 0, maxSize)
			h := d.num[numHeight].decode(d.zp, 0, maxSize)
			var bm *Bitmap
			if bm, err = d.directBitmap(w, h); err != nil {
				return result{}, err
			}
			d.add(bm, t != recNewLib, t != recNewImage, Ref{Shape: -1})
		case recRefLibImage, recRefLib, recRefImage:
			if len(d.lib) == 0 {
				return result{}, errors.New("jb2: refinement with an empty library")
			}
			e := d.lib[d.num[numMatch].decode(d.zp, 0, len(d.lib)-1)]
			ref := e.bm
			w := ref.W + d.num[numWidthDiff].decode(d.zp, -maxSize-1, maxSize)
			h := ref.H + d.num[numHeightDiff].decode(d.zp, -maxSize-1, maxSize)
			var bm *Bitmap
			if bm, err = d.refineBitmap(w, h, ref); err != nil {
				return result{}, err
			}
			// Pixel (r, c) lay over ref's (r+dr, c+dc), as in refineBitmap.
			dr, dc := ref.H/2-h/2, (ref.W-1)/2-(w-1)/2
			d.add(bm, t != recRefLib, t != recRefImage, Ref{e.shape, -dc, -dr})
		case recCopy:
			if len(d.lib) == 0 {
				return result{}, errors.New("jb2: copy from an empty library")
			}
			e := d.lib[d.num[numMatch].decode(d.zp, 0, len(d.lib)-1)]
			left, bottom := d.locate(e.bm.W, e.bm.H)
			d.blit(e.shape, e.bm, left, bottom)
		case recNonSymbol:
			w := d.num[numWidth].decode(d.zp, 0, maxSize)
			h := d.num[numHeight].decode(d.zp, 0, maxSize)
			var bm *Bitmap
			if bm, err = d.directBitmap(w, h); err != nil {
				return result{}, err
			}
			left := d.num[numCol].decode(d.zp, 1, max(d.im.W, 1))
			top := d.num[numRow].decode(d.zp, 1, max(d.im.H, 1))
			d.placeAt(bm, left, top-h+1)
		case recDictOrReset:
			if started {
				for i := range d.num {
					d.num[i].reset()
				}
				continue
			}
			n := d.num[numDictSize].decode(d.zp, 0, maxSize)
			if dict == nil || n > len(dict.Shapes) || len(d.lib) > 0 {
				return result{}, fmt.Errorf("jb2: needs a dictionary of %d shapes", n)
			}
			d.im.NDict = n
			for i, s := range dict.Shapes[:n] {
				d.lib = append(d.lib, libEntry{s, i})
			}
		case recComment:
			n := d.num[numCommentLen].decode(d.zp, 0, maxSize)
			for range n {
				d.num[numCommentByte].decode(d.zp, 0, 255)
				if d.zp.Overrun() > maxOver {
					return result{}, errors.New("jb2: truncated comment")
				}
			}
		case recEnd:
			return result{d.im, d.lib}, nil
		}
	}
	return result{}, errors.New("jb2: too many records")
}

// add places a new bitmap on the image at a relative location and/or
// into the library.
// ref is how bm was coded, its offsets for bm before trimming.
func (d *decoder) add(bm *Bitmap, image, library bool, ref Ref) {
	var left, bottom int
	if image {
		left, bottom = d.locate(bm.W, bm.H)
	}
	t, dl, db := trim(bm)
	shape := d.im.NDict + len(d.im.Shapes)
	d.im.Shapes = append(d.im.Shapes, t)
	if ref.Shape >= 0 {
		top := bm.H - t.H - db // rows trimmed from the top
		ref.DX -= dl
		ref.DY -= top
	}
	d.im.Refs = append(d.im.Refs, ref)
	if library {
		d.lib = append(d.lib, libEntry{t, shape})
	}
	if image {
		d.blit(shape, t, left+dl, bottom+db)
	}
}

// placeAt places a bitmap that is not in the library.
func (d *decoder) placeAt(bm *Bitmap, left, bottom int) {
	t, dl, db := trim(bm)
	shape := d.im.NDict + len(d.im.Shapes)
	d.im.Shapes = append(d.im.Shapes, t)
	d.im.Refs = append(d.im.Refs, Ref{Shape: -1})
	d.blit(shape, t, left+dl, bottom+db)
}

// blit records a placement, even of an empty shape: FGbz colours
// blits by their index.
func (d *decoder) blit(shape int, bm *Bitmap, left, bottom int) {
	d.im.Blits = append(d.im.Blits, Blit{shape, left - 1, bottom - 1})
}

// locate decodes a location relative to the current text line and
// returns the column of the left edge and the row of the bottom edge.
func (d *decoder) locate(w, h int) (left, bottom int) {
	if d.zp.Decode(&d.offsetType) == 1 {
		left = d.firstLeft + d.num[numNewCol].decode(d.zp, -maxSize-1, maxSize)
		top := d.firstBottom + d.num[numNewRow].decode(d.zp, -maxSize-1, maxSize)
		bottom = top - h + 1
		d.firstLeft, d.firstBottom = left, bottom
		d.onLine = 0
	} else {
		left = d.prevRight + d.num[numSameCol].decode(d.zp, -maxSize-1, maxSize)
		base := d.firstBottom
		if d.onLine >= 3 {
			base = median(d.bottoms[0], d.bottoms[1], d.bottoms[2])
		}
		bottom = base + d.num[numSameRow].decode(d.zp, -maxSize-1, maxSize)
	}
	d.prevRight = left + w - 1
	d.bottoms[2], d.bottoms[1], d.bottoms[0] = d.bottoms[1], d.bottoms[0], bottom
	d.onLine++
	return left, bottom
}

func median(a, b, c int) int {
	return max(min(a, b), min(max(a, b), c))
}

// trim removes empty edge rows and columns. It returns the offsets of
// the result's left column and bottom row in bm.
func trim(bm *Bitmap) (*Bitmap, int, int) {
	r0, r1, c0, c1 := bm.H, -1, bm.W, -1
	for r := range bm.H {
		row := bm.Pix[r*bm.W : (r+1)*bm.W]
		if i := bytes.IndexByte(row, 1); i >= 0 {
			r0, r1 = min(r0, r), r
			c0, c1 = min(c0, i), max(c1, bytes.LastIndexByte(row, 1))
		}
	}
	if r1 < 0 {
		return &Bitmap{}, 0, 0
	}
	if r0 == 0 && c0 == 0 && r1 == bm.H-1 && c1 == bm.W-1 {
		return bm, 0, 0
	}
	t := newBitmap(c1-c0+1, r1-r0+1)
	for r := range t.H {
		copy(t.Pix[r*t.W:(r+1)*t.W], bm.Pix[(r0+r)*bm.W+c0:])
	}
	return t, c0, bm.H - 1 - r1
}

// checkSize limits a bitmap's size and the pixels decoded per stream.
func (d *decoder) checkSize(w, h int) error {
	if w < 0 || h < 0 || w*h > maxPixels {
		return fmt.Errorf("jb2: bitmap %dx%d", w, h)
	}
	d.budget -= w * h
	if d.budget < 0 {
		// ivy: the budget follows the limits of ivy, so it wraps ivy.ErrLimit.
		return fmt.Errorf("jb2: too many pixels for the stream size: %w", ivy.ErrLimit)
	}
	return nil
}

// directBitmap decodes a bitmap with the 10-pixel template (11.5.9):
// 3 pixels two rows up, 5 one row up, 2 to the left; the first is the
// most significant bit of the context.
func (d *decoder) directBitmap(w, h int) (*Bitmap, error) {
	if err := d.checkSize(w, h); err != nil {
		return nil, err
	}
	// Padded: 2 white rows on top, 2 columns left, 3 right.
	pw := w + 5
	buf, _ := d.buffers(pw*(h+2), 0)
	for r := range h {
		if d.zp.Overrun() > maxOver {
			return nil, errors.New("jb2: truncated bitmap")
		}
		row := buf[(r+2)*pw:]
		up1 := buf[(r+1)*pw:]
		up2 := buf[r*pw:]
		// Padded column of pixel c is c+2.
		t2 := int(up2[1])<<2 | int(up2[2])<<1 | int(up2[3])
		t1 := int(up1[0])<<4 | int(up1[1])<<3 | int(up1[2])<<2 | int(up1[3])<<1 | int(up1[4])
		t0 := 0
		for c := range w {
			cx := &d.direct[t2<<7|t1<<2|t0]
			b, ok := d.zp.TryDecode(cx)
			if !ok {
				b = d.zp.Decode(cx)
			}
			row[c+2] = byte(b)
			t2 = (t2<<1)&7 | int(up2[c+4])
			t1 = (t1<<1)&31 | int(up1[c+5])
			t0 = (t0<<1)&3 | b
		}
	}
	bm := newBitmap(w, h)
	for r := range h {
		copy(bm.Pix[r*w:(r+1)*w], buf[(r+2)*pw+2:])
	}
	return bm, nil
}

// refineBitmap decodes a bitmap with the 11-pixel template (11.5.10):
// 4 pixels from itself (3 one row up, 1 to the left) and 7 from ref
// (1 above, 3 level with and 3 below the pixel), the two aligned by
// their centres; the first is the most significant bit.
func (d *decoder) refineBitmap(w, h int, ref *Bitmap) (*Bitmap, error) {
	if err := d.checkSize(w, h); err != nil {
		return nil, err
	}
	dr := ref.H/2 - h/2
	dc := (ref.W-1)/2 - (w-1)/2
	// Both padded by 1 row on top and 1 column left, 2 right; m also by
	// 1 row below. m holds ref aligned with the new bitmap.
	pw := w + 3
	buf, m := d.buffers(pw*(h+1), pw*(h+2)) // m is filled below
	for r := -1; r <= h; r++ {
		for c := -1; c <= w+1; c++ {
			m[(r+1)*pw+c+1] = byte(ref.at(r+dr, c+dc))
		}
	}
	for r := range h {
		if d.zp.Overrun() > maxOver {
			return nil, errors.New("jb2: truncated bitmap")
		}
		row := buf[(r+1)*pw:]
		up := buf[r*pw:]
		m0, m1, m2 := m[r*pw:], m[(r+1)*pw:], m[(r+2)*pw:]
		// Padded column of pixel c is c+1.
		t := int(up[0])<<1 | int(up[1])
		a := int(m1[0])<<1 | int(m1[1])
		b := int(m2[0])<<1 | int(m2[1])
		left := 0
		for c := range w {
			t = (t<<1)&7 | int(up[c+2])
			a = (a<<1)&7 | int(m1[c+2])
			b = (b<<1)&7 | int(m2[c+2])
			ctx := t<<8 | left<<7 | int(m0[c+1])<<6 | a<<3 | b
			var ok bool
			if left, ok = d.zp.TryDecode(&d.refine[ctx]); !ok {
				left = d.zp.Decode(&d.refine[ctx])
			}
			row[c+1] = byte(left)
		}
	}
	bm := newBitmap(w, h)
	for r := range h {
		copy(bm.Pix[r*w:(r+1)*w], buf[(r+1)*pw+1:])
	}
	return bm, nil
}
