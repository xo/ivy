// Copyright (c) 2026 Serge Vakulenko
// SPDX-License-Identifier: MIT

// Copied from github.com/sergev/dejapdf, file internal/bzz/bzz.go, at
// commit 2b6ca0c1123c (D7, D26, docs/UPSTREAM.md). Its MIT license is in
// djvu/internal/LICENSE.

package bzz

import (
	"errors"
	"fmt"

	"github.com/xo/ivy"
	"github.com/xo/ivy/djvu/internal/zp"
)

const (
	maxBlock  = 1 << 24 // the 24-bit size field
	maxOutput = 1 << 28 // refuse to expand beyond this
	maxOver   = 64      // bytes read past the end before giving up
)

// Decode decompresses a BZZ stream (DjVu v3 spec, appendix 4).
func Decode(b []byte) ([]byte, error) {
	return DecodeMax(b, maxOutput)
}

// DecodeMax is Decode with a limit on the size of the output, in bytes.
//
// ivy: added, so that the output follows the limits of ivy. An output over
// the limit returns an error that wraps ivy.ErrLimit.
func DecodeMax(b []byte, limit int) ([]byte, error) {
	d := zp.NewDecoder(b)
	ctx := make([]byte, 262) // shared by all blocks
	var out []byte
	for {
		size := raw(d, 24)
		if d.Overrun() > maxOver {
			return nil, errors.New("bzz: truncated stream")
		}
		if size == 0 {
			return out, nil
		}
		if len(out)+size > limit {
			return nil, fmt.Errorf("bzz: output exceeds %d bytes: %w", limit, ivy.ErrLimit) // ivy: the limit of the caller
		}
		blk, err := decodeBlock(d, ctx, size)
		if err != nil {
			return nil, err
		}
		out = append(out, blk...)
	}
}

// raw decodes a bits-wide integer in pass-through mode.
func raw(d *zp.Decoder, bits int) int {
	n := 1
	for n < 1<<bits {
		n = n<<1 | d.DecodePassthrough()
	}
	return n - 1<<bits
}

// bin decodes a bits-wide integer with contexts ctx[1..2^bits-1].
func bin(d *zp.Decoder, ctx []byte, bits int) int {
	n := 1
	for n < 1<<bits {
		n = n<<1 | d.Decode(&ctx[n])
	}
	return n - 1<<bits
}

func decodeBlock(d *zp.Decoder, ctx []byte, size int) ([]byte, error) {
	if size > maxBlock {
		return nil, fmt.Errorf("bzz: block size %d", size)
	}
	fshift := uint(0)
	if d.DecodePassthrough() == 1 {
		fshift = 1 + uint(d.DecodePassthrough())
	}

	// Move-to-front indices to the Burrows-Wheeler buffer.
	var mtf [256]byte
	for i := range mtf {
		mtf[i] = byte(i)
	}
	var freq [4]uint32
	fadd := uint32(4)
	data := make([]byte, size)
	marker := -1
	mtfno := 3
	for i := range data {
		if i&0xfff == 0 && d.Overrun() > maxOver {
			return nil, errors.New("bzz: truncated stream")
		}
		c := 2
		if mtfno < 2 {
			c = mtfno
		}
		switch {
		case d.Decode(&ctx[c]) == 1:
			mtfno = 0
		case d.Decode(&ctx[c+3]) == 1:
			mtfno = 1
		default:
			mtfno = 256 // end of block
			for bits, base := 1, 6; bits <= 7; bits, base = bits+1, base+1<<bits {
				if d.Decode(&ctx[base]) == 1 {
					mtfno = 1<<bits + bin(d, ctx[base:], bits)
					break
				}
			}
		}
		if mtfno == 256 {
			if marker >= 0 {
				return nil, errors.New("bzz: two end-of-block markers")
			}
			marker = i
			continue
		}
		data[i] = mtf[mtfno]

		fadd += fadd >> fshift
		if fadd > 0x10000000 {
			fadd >>= 24
			for j := range freq {
				freq[j] >>= 24
			}
		}
		fc := fadd
		if mtfno < 4 {
			fc += freq[mtfno]
		}
		k := mtfno
		for ; k > 3; k-- {
			mtf[k] = mtf[k-1]
		}
		for ; k > 0 && fc >= freq[k-1]; k-- {
			mtf[k] = mtf[k-1]
			freq[k] = freq[k-1]
		}
		mtf[k] = data[i]
		freq[k] = fc
	}
	if marker <= 0 {
		return nil, fmt.Errorf("bzz: bad end-of-block position %d", marker)
	}

	// Inverse Burrows-Wheeler transform.
	var count [256]int
	posn := make([]int32, size)
	for i, k := range data {
		if i == marker {
			continue
		}
		posn[i] = int32(count[k])
		count[k]++
	}
	last := 1
	for i, k := range count {
		count[i] = last
		last += k
	}
	out := make([]byte, size-1)
	k := 0
	for i := size - 2; i >= 0; i-- {
		out[i] = data[k]
		k = count[data[k]] + int(posn[k])
	}
	if k != marker {
		return nil, errors.New("bzz: corrupt block")
	}
	return out, nil
}
