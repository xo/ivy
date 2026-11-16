// Copyright (c) The Hayro Authors.
//
// Use of this source code is governed by the Apache-2.0 or MIT license that
// can be found in the LICENSE-APACHE and LICENSE-MIT files.

// This file ports hayro-postscript src/string/ascii_85.rs.

package postscript

import "math"

// pow85 is POW_85 in decode_into of hayro-postscript src/string/ascii_85.rs.
var pow85 = [5]uint64{52200625, 614125, 7225, 85, 1}

// decodeASCII85 is decode_into in hayro-postscript src/string/ascii_85.rs. It
// appends the ASCII base-85 data to out. It returns false for a malformed
// group.
func decodeASCII85(data, out []byte) ([]byte, bool) {
	r := newReader(data)

	readByte := func() (byte, bool) {
		for {
			b, ok := r.readByte()
			if !ok {
				return 0, false
			}
			if !isWhitespace(b) {
				return b, true
			}
		}
	}

	var group [5]byte
	n := 0

	flushGroup := func() bool {
		if n == 0 {
			return true
		}
		if n == 1 {
			return false
		}
		// The decoder pads a short group with u. The group gives n-1 bytes.
		digits := [5]byte{'u', 'u', 'u', 'u', 'u'}
		copy(digits[:], group[:n])
		var value uint64
		for i, d := range digits {
			value += uint64(d-'!') * pow85[i]
		}
		// Upstream adds with arithmetic of 32 bits that fails on an
		// overflow. No term is negative, so the sum of 64 bits overflows 32
		// bits only when one step of upstream overflows.
		if value > math.MaxUint32 {
			return false
		}
		be := [4]byte{byte(value >> 24), byte(value >> 16), byte(value >> 8), byte(value)}
		out = append(out, be[:n-1]...)
		n = 0
		return true
	}

	out = growBytes(out, len(data)*4/5)

	for {
		b, ok := readByte()
		if !ok {
			return out, flushGroup()
		}

		switch {
		case '!' <= b && b <= 'u':
			group[n] = b
			n++
			if n == 5 && !flushGroup() {
				return out, false
			}
		case b == 'z':
			if !flushGroup() {
				return out, false
			}
			out = append(out, 0, 0, 0, 0)
		case b == '~':
			return out, flushGroup()
		default:
			return out, false
		}
	}
}

// growBytes is Vec::reserve of the Rust core library. It makes room for n
// more bytes in b.
func growBytes(b []byte, n int) []byte {
	if n <= cap(b)-len(b) {
		return b
	}
	grown := make([]byte, len(b), len(b)+n)
	copy(grown, b)
	return grown
}
