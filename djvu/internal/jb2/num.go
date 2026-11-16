// Copyright (c) 2026 Serge Vakulenko
// SPDX-License-Identifier: MIT

// Copied from github.com/sergev/dejapdf, file internal/jb2/num.go, at
// commit 2b6ca0c1123c (D7, D26, docs/UPSTREAM.md). Its MIT license is in
// djvu/internal/LICENSE.

package jb2

import "github.com/xo/ivy/djvu/internal/zp"

// numCoder is one integer context (spec 11.2.2): a binary decision
// tree whose nodes are created when first reached.
type numCoder struct {
	ctx []byte
	kid [][2]int32 // child node indices; 0 = not yet created
}

func (n *numCoder) reset() {
	n.ctx = append(n.ctx[:0], 0)
	n.kid = append(n.kid[:0], [2]int32{})
}

func (n *numCoder) child(i int32, bit int) int32 {
	k := n.kid[i][bit]
	if k == 0 {
		k = int32(len(n.ctx))
		n.ctx = append(n.ctx, 0)
		n.kid = append(n.kid, [2]int32{})
		n.kid[i][bit] = k
	}
	return k
}

// decide decodes the decision at node i unless the range fixes it:
// fixed < 0 means both outcomes are possible.
func (n *numCoder) decide(d *zp.Decoder, i int32, fixed int) int {
	if fixed >= 0 {
		return fixed
	}
	return d.Decode(&n.ctx[i])
}

// decode decodes an integer in [low, high] (spec 11.2.5).
func (n *numCoder) decode(d *zp.Decoder, low, high int) int {
	if len(n.ctx) == 0 {
		n.reset()
	}
	high = max(high, low)
	var cur int32
	// Phase 1: sign, 1 for n >= 0.
	fixed := -1
	if low >= 0 {
		fixed = 1
	} else if high < 0 {
		fixed = 0
	}
	pos := n.decide(d, cur, fixed) == 1
	cur = n.child(cur, b2i(pos))
	// v ranges over [vmin, vmax]: n itself, or -n-1 for negative n.
	vmin, vmax := max(low, 0), high
	if !pos {
		vmin, vmax = max(-high-1, 0), -low-1
	}
	// Phase 2: ranges [2^k-1, 2^(k+1)-2].
	a, b := 0, 0
	for {
		vmin = max(vmin, a)
		// 1 means v is beyond this range.
		fixed = -1
		if vmax <= b {
			fixed = 0
		} else if vmin > b {
			fixed = 1
		}
		beyond := n.decide(d, cur, fixed)
		cur = n.child(cur, beyond)
		if beyond == 0 {
			break
		}
		a, b = b+1, 2*b+2
	}
	// Phase 3: bits of v within [a, b], MSB first.
	for a < b {
		mid := (a + b) / 2
		fixed = -1
		if vmax <= mid {
			fixed = 0
		} else if vmin > mid {
			fixed = 1
		}
		bit := n.decide(d, cur, fixed)
		cur = n.child(cur, bit)
		if bit == 0 {
			b = mid
		} else {
			a = mid + 1
		}
	}
	if pos {
		return a
	}
	return -a - 1
}

func b2i(b bool) int {
	if b {
		return 1
	}
	return 0
}
