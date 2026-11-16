// Copyright (c) 2026 Serge Vakulenko
// SPDX-License-Identifier: MIT

// Copied from github.com/sergev/dejapdf, file internal/djvu/mmr.go, at
// commit 2b6ca0c1123c (D7, D26, docs/UPSTREAM.md). Its MIT license is in
// djvu/internal/LICENSE.

package doc // ivy: renamed from djvu, the name of the package of the format

import (
	"errors"
	"fmt"

	"github.com/xo/ivy"
	"github.com/xo/ivy/djvu/internal/jb2"
	"github.com/xo/ivy/internal/ccitt"
)

// ivy: the copy decodes the G4 streams with internal/ccitt of ivy, the port
// of hayro-ccitt, not with the copy of golang.org/x/image/ccitt in dejapdf.
// A stream that fails keeps the rows that it decoded, and DecodeMMR returns
// the bitmap with the error, so that a page can show the part. A row that
// the stream does not reach has no ink.

// maxMMRPixels limits a decoded Smmr mask, like the JB2 page limit.
const maxMMRPixels = 1 << 27

// DecodeMMR decodes an Smmr chunk (spec 8.3.15): "MMR", flags (bit 0
// reverse video, bit 1 striped), width and height, then one G4 stream,
// or rows per stripe and, for each stripe from the top, a 4-byte length
// and an independent G4 stream. The result has 1 for black.
func DecodeMMR(b []byte) (*jb2.Bitmap, error) {
	return decodeMMR(b, ivy.Limits{})
}

// decodeMMR is DecodeMMR within lim.MaxPixels and lim.MaxMemory.
//
// ivy: added.
func decodeMMR(b []byte, lim ivy.Limits) (*jb2.Bitmap, error) {
	if len(b) < 8 || string(b[:3]) != "MMR" {
		return nil, errors.New("djvu: Smmr: bad header")
	}
	invert, striped := b[3]&1 != 0, b[3]&2 != 0
	w, h := int(b[4])<<8|int(b[5]), int(b[6])<<8|int(b[7])
	if w == 0 || h == 0 || w*h > maxMMRPixels {
		return nil, fmt.Errorf("djvu: Smmr: size %dx%d", w, h)
	}
	if lim = lim.WithDefaults(); int64(w*h) > min(lim.MaxPixels, lim.MaxMemory) { // ivy: the limits of the caller
		return nil, fmt.Errorf("djvu: Smmr: size %dx%d: %w", w, h, ivy.ErrLimit)
	}
	b = b[8:]
	bm := &jb2.Bitmap{W: w, H: h, Pix: make([]byte, w*h)}
	rps := h
	if striped {
		if len(b) < 2 {
			return nil, errors.New("djvu: Smmr: no rows per stripe")
		}
		rps = int(b[0])<<8 | int(b[1])
		b = b[2:]
		if rps == 0 {
			return nil, errors.New("djvu: Smmr: zero rows per stripe")
		}
	}
	for y0 := 0; y0 < h; y0 += rps {
		data := b
		if striped {
			if len(b) < 4 {
				return bm, errors.New("djvu: Smmr: stripe truncated")
			}
			n := int(b[0])<<24 | int(b[1])<<16 | int(b[2])<<8 | int(b[3])
			if n < 0 || n > len(b)-4 {
				return bm, errors.New("djvu: Smmr: stripe truncated")
			}
			data, b = b[4:4+n], b[4+n:]
		}
		rows := min(rps, h-y0)
		// ivy: internal/ccitt gives each row as the positions where its
		// color changes, from white.
		r := &mmrRows{bm: bm, y: y0, end: y0 + rows, invert: invert}
		_, err := ccitt.Decode(data, r, &ccitt.DecodeSettings{
			Columns:    uint32(w),
			Rows:       uint32(rows),
			EndOfBlock: true,
			Encoding:   ccitt.Group4,
		})
		if err != nil {
			return bm, fmt.Errorf("djvu: Smmr: row %d: %w", r.y, err)
		}
	}
	return bm, nil
}

// mmrRows writes the rows of a G4 stream into a bitmap.
//
// ivy: added, for internal/ccitt.
type mmrRows struct {
	bm     *jb2.Bitmap
	y, end int
	invert bool
}

// PushRow satisfies the ccitt.Decoder interface.
func (r *mmrRows) PushRow(changes []uint32) {
	if r.y >= r.end {
		return
	}
	w := r.bm.W
	px := r.bm.Pix[r.y*w : (r.y+1)*w]
	ink, paper := byte(1), byte(0)
	if r.invert {
		ink, paper = 0, 1
	}
	black := false
	x := 0
	for _, c := range changes {
		e := min(int(c), w)
		v := paper
		if black {
			v = ink
		}
		for ; x < e; x++ {
			px[x] = v
		}
		black = !black
	}
	v := paper
	if black {
		v = ink
	}
	for ; x < w; x++ {
		px[x] = v
	}
	r.y++
}

// mmrMask wraps an Smmr bitmap as a mask image of one blit. If the
// decode fails in part, it returns the image with the error.
func mmrMask(data []byte, lim ivy.Limits) (*jb2.Image, error) {
	bm, err := decodeMMR(data, lim)
	if bm == nil { // ivy: keep a bitmap that is decoded in part
		return nil, err
	}
	return &jb2.Image{W: bm.W, H: bm.H, Shapes: []*jb2.Bitmap{bm},
		Refs: []jb2.Ref{{Shape: -1}}, Blits: []jb2.Blit{{}}}, err
}
