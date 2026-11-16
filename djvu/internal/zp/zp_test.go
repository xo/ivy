// Copyright (c) 2026 Serge Vakulenko
// SPDX-License-Identifier: MIT

// Copied from github.com/sergev/dejapdf, file internal/zp/zp_test.go, at
// commit 2b6ca0c1123c (D7, D26, docs/UPSTREAM.md). Its MIT license is in
// djvu/internal/LICENSE.

package zp

import (
	"fmt"
	"os"
	"testing"
)

func raw(d *Decoder, bits int) int {
	v := 0
	for i := 0; i < bits; i++ {
		v = v<<1 | d.DecodePassthrough()
	}
	return v
}

// Block size header of `bzz -e` output on n random bytes: n+1.
func TestPassthrough(t *testing.T) {
	cases := []struct {
		n int
		b []byte
	}{
		{1, []byte{0xff, 0xff, 0xfd, 0xfe, 0xb6, 0x58}},
		{100, []byte{0xff, 0xff, 0x9a, 0xfa, 0x51, 0x68}},
		{5000, []byte{0xff, 0xec, 0x76, 0xff, 0xa0, 0x7a}},
		{70000, []byte{0xfe, 0xee, 0x8e, 0xfe, 0x46, 0x2f}},
	}
	for _, c := range cases {
		if got := raw(NewDecoder(c.b), 24); got != c.n+1 {
			t.Errorf("n=%d: block size %d", c.n, got)
		}
	}
}

// mtfIndex decodes one BZZ move-to-front index (spec appendix 4);
// 256 is the end-of-block symbol.
func mtfIndex(d *Decoder, ctx []byte, prev int) int {
	c := 2
	if prev < 2 {
		c = prev
	}
	if d.Decode(&ctx[c]) == 1 {
		return 0
	}
	if d.Decode(&ctx[c+3]) == 1 {
		return 1
	}
	for b, base := 1, 6; b <= 7; b, base = b+1, base+1<<b {
		if d.Decode(&ctx[base]) == 1 {
			n := 1
			for n < 1<<b {
				n = n<<1 | d.Decode(&ctx[base+n])
			}
			return 1<<b + n - 1<<b
		}
	}
	return 256
}

// `bzz -e` on n zero bytes: n+1 symbols, all MTF index 0 except one
// end-of-block, then a zero block size. Long MPS runs walk the state
// chain; any wrong table entry on the path loses sync.
func TestZeros(t *testing.T) {
	for _, n := range []int{10, 1000, 100000} {
		b, err := os.ReadFile(fmt.Sprintf("testdata/zeros-%d.bzz", n))
		if err != nil {
			t.Fatal(err)
		}
		d := NewDecoder(b)
		if got := raw(d, 24); got != n+1 {
			t.Fatalf("n=%d: block size %d", n, got)
		}
		if d.DecodePassthrough() == 1 {
			d.DecodePassthrough() // fshift 1 or 2
		}
		ctx := make([]byte, 262)
		zeros, eob, prev := 0, 0, 3
		for i := 0; i <= n; i++ {
			switch prev = mtfIndex(d, ctx, prev); prev {
			case 0:
				zeros++
			case 256:
				eob++
			default:
				t.Fatalf("n=%d: symbol %d: MTF index %d", n, i, prev)
			}
		}
		if zeros != n || eob != 1 {
			t.Errorf("n=%d: %d zeros, %d EOB", n, zeros, eob)
		}
		if got := raw(d, 24); got != 0 {
			t.Errorf("n=%d: trailing block size %d", n, got)
		}
	}
}

// Vectors from the MIT-licensed djvu-zp encoder (tmp/zpgen): random
// decisions on 64 contexts mixed with both pass-through kinds.
func TestVectors(t *testing.T) {
	for _, name := range []string{"mixed", "skewed", "drift"} {
		stream, err := os.ReadFile("testdata/" + name + ".zp")
		if err != nil {
			t.Fatal(err)
		}
		ops, err := os.ReadFile("testdata/" + name + ".ops")
		if err != nil {
			t.Fatal(err)
		}
		d := NewDecoder(stream)
		ctx := make([]byte, 64)
		for i, op := range ops {
			var got int
			switch op & 0x7f {
			case 126:
				got = d.DecodePassthrough()
			case 127:
				got = d.DecodePassthroughIW44()
			default:
				got = d.Decode(&ctx[op&0x3f])
			}
			if got != int(op>>7) {
				t.Fatalf("%s: decision %d (op %d): got %d", name, i, op&0x7f, got)
			}
		}
	}
}

func TestTable(t *testing.T) {
	for k, s := range table {
		if int(s.mu) >= len(table) || int(s.lambda) >= len(table) {
			t.Errorf("%d: transition out of range", k)
		}
		if s.delta == 0 || s.delta > 0x8000 {
			t.Errorf("%d: delta %#x", k, s.delta)
		}
		if (k <= 2 || k >= 83) != (s.theta == 0) {
			t.Errorf("%d: theta %#x", k, s.theta)
		}
	}
	for k := 1; k < 83; k += 2 {
		a, b := table[k], table[k+1]
		if a.delta != b.delta || a.theta != b.theta {
			t.Errorf("steady pair %d,%d differs", k, k+1)
		}
	}
}

func FuzzDecode(f *testing.F) {
	f.Add([]byte{0xff, 0xec, 0x76, 0xff, 0xa0, 0x7a}, []byte{0, 1, 2})
	f.Fuzz(func(t *testing.T, data, ops []byte) {
		d := NewDecoder(data)
		ctx := make([]byte, 4)
		for _, op := range ops {
			switch {
			case op&8 != 0:
				d.DecodePassthroughIW44()
			case op&4 != 0:
				d.DecodePassthrough()
			default:
				d.Decode(&ctx[op&3])
			}
			if d.a < 0 || d.a >= 0x8000 || d.c < d.a {
				t.Fatalf("registers A=%#x C=%#x", d.a, d.c)
			}
		}
	})
}
