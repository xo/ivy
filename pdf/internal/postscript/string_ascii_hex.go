// Copyright (c) The Hayro Authors.
//
// Use of this source code is governed by the Apache-2.0 or MIT license that
// can be found in the LICENSE-APACHE and LICENSE-MIT files.

// This file ports hayro-postscript src/string/ascii_hex.rs.

package postscript

import "slices"

// decodeASCIIHex is decode_into in hayro-postscript src/string/ascii_hex.rs.
// It appends the hexadecimal data to out. A last single digit gives the high
// half of a byte. It returns false for a byte that is not a hexadecimal digit
// or whitespace.
func decodeASCIIHex(data, out []byte) ([]byte, bool) {
	hasWhitespace := slices.ContainsFunc(data, isWhitespace)

	out = growBytes(out, (len(data)+1)/2)

	if !hasWhitespace {
		i := 0
		for ; i+1 < len(data); i += 2 {
			hi, ok1 := decodeHexDigit(data[i])
			lo, ok2 := decodeHexDigit(data[i+1])
			if !ok1 || !ok2 {
				return out, false
			}
			out = append(out, hi<<4|lo)
		}
		if i < len(data) {
			hi, ok := decodeHexDigit(data[i])
			if !ok {
				return out, false
			}
			out = append(out, hi<<4)
		}
		return out, true
	}

	i := 0
	readByte := func() (byte, bool) {
		for i < len(data) {
			b := data[i]
			i++
			if !isWhitespace(b) {
				return b, true
			}
		}
		return 0, false
	}

	for {
		hi, ok := readByte()
		if !ok {
			return out, true
		}
		hiv, ok := decodeHexDigit(hi)
		if !ok {
			return out, false
		}
		lo, ok := readByte()
		if !ok {
			return append(out, hiv<<4), true
		}
		lov, ok := decodeHexDigit(lo)
		if !ok {
			return out, false
		}
		out = append(out, hiv<<4|lov)
	}
}

// decodeHexDigit is decode_hex_digit in hayro-postscript
// src/string/ascii_hex.rs.
func decodeHexDigit(c byte) (byte, bool) {
	switch {
	case '0' <= c && c <= '9':
		return c - '0', true
	case 'A' <= c && c <= 'F':
		return c - 'A' + 10, true
	case 'a' <= c && c <= 'f':
		return c - 'a' + 10, true
	}
	return 0, false
}
