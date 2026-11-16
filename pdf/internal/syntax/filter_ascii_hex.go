// Copyright (c) The Hayro Authors
//
// Use of this source code is governed by the Apache-2.0 or MIT license that
// can be found in the LICENSE-APACHE and LICENSE-MIT files.

// This file ports hayro-syntax src/filter/ascii_hex.rs.

package syntax

// decodeASCIIHex is decode in hayro-syntax src/filter/ascii_hex.rs.
func decodeASCIIHex(data []byte) ([]byte, bool) {
	return decodeASCIIHexInto(data)
}

// decodeASCIIHexInto is decode_into in hayro-syntax
// src/filter/ascii_hex.rs. It decodes pairs of hex digits up to the first
// '>', and it ignores white space. A last digit alone stands for its high
// half.
func decodeASCIIHexInto(data []byte) ([]byte, bool) {
	hasWhitespace := false
	end := len(data)
	for idx, b := range data {
		hasWhitespace = hasWhitespace || isWhiteSpaceCharacter(b)
		if b == '>' {
			end = idx
			break
		}
	}
	data = data[:end]
	decoded := make([]byte, 0, (len(data)+1)/2)
	if !hasWhitespace {
		for i := 0; i+1 < len(data); i += 2 {
			hi, okHi := decodeHexDigit(data[i])
			lo, okLo := decodeHexDigit(data[i+1])
			if !okHi || !okLo {
				return nil, false
			}
			decoded = append(decoded, hi<<4|lo)
		}
		if len(data)%2 == 1 {
			hi, ok := decodeHexDigit(data[len(data)-1])
			if !ok {
				return nil, false
			}
			decoded = append(decoded, hi<<4)
		}
		return decoded, true
	}
	i := 0
	readByte := func() (byte, bool) {
		for i < len(data) {
			b := data[i]
			i++
			if !isWhiteSpaceCharacter(b) {
				return b, true
			}
		}
		return 0, false
	}
	for {
		hi, okHi := readByte()
		if !okHi {
			break
		}
		lo, okLo := readByte()
		h, ok := decodeHexDigit(hi)
		if !ok {
			return nil, false
		}
		if !okLo {
			decoded = append(decoded, h<<4)
			break
		}
		l, ok := decodeHexDigit(lo)
		if !ok {
			return nil, false
		}
		decoded = append(decoded, h<<4|l)
	}
	return decoded, true
}

// decodeHexDigit is decode_hex_digit in hayro-syntax
// src/filter/ascii_hex.rs.
func decodeHexDigit(c byte) (byte, bool) {
	switch {
	case c >= '0' && c <= '9':
		return c - '0', true
	case c >= 'A' && c <= 'F':
		return c - 'A' + 10, true
	case c >= 'a' && c <= 'f':
		return c - 'a' + 10, true
	}
	return 0, false
}
