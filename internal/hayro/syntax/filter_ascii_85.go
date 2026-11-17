// Copyright (c) The Hayro Authors
//
// Use of this source code is governed by the Apache-2.0 or MIT license that
// can be found in the LICENSE-APACHE and LICENSE-MIT files.

// This file ports hayro-syntax src/filter/ascii_85.rs.

package syntax

import "encoding/binary"

// decodeASCII85 is decode in hayro-syntax src/filter/ascii_85.rs.
func decodeASCII85(data []byte) ([]byte, bool) {
	pow85 := [5]uint64{52200625, 614125, 7225, 85, 1}
	r := NewReader(data)
	readByte := func() (byte, bool) {
		for {
			b, ok := r.ReadU8()
			if !ok {
				return 0, false
			}
			if !isWhiteSpaceCharacter(b) {
				return b, true
			}
		}
	}
	decoded := make([]byte, 0, len(data)*4/5)
	group := make([]byte, 0, 5)
	flushGroup := func() bool {
		if len(group) == 0 {
			return true
		}
		if len(group) == 1 {
			return false
		}
		outputLen := len(group) - 1
		var digits [5]uint64
		for i := range digits {
			c := byte('u')
			if i < len(group) {
				c = group[i]
			}
			digits[i] = uint64(c - '!')
		}
		var value uint64
		for i, d := range digits {
			value += d * pow85[i]
		}
		// The checked arithmetic of upstream fails past u32::MAX.
		if value > 0xFFFFFFFF {
			return false
		}
		var buf [4]byte
		binary.BigEndian.PutUint32(buf[:], uint32(value))
		decoded = append(decoded, buf[:outputLen]...)
		group = group[:0]
		return true
	}
	for {
		b, ok := readByte()
		if !ok {
			// Be lenient and accept what is there (see PDFBOX-5910).
			if !flushGroup() {
				return nil, false
			}
			return decoded, true
		}
		switch {
		case b >= '!' && b <= 'u':
			group = append(group, b)
			if len(group) == 5 && !flushGroup() {
				return nil, false
			}
		case b == 'z':
			if !flushGroup() {
				return nil, false
			}
			decoded = append(decoded, 0, 0, 0, 0)
		case b == '~':
			// Lenient: accept a missing '>'.
			if !flushGroup() {
				return nil, false
			}
			return decoded, true
		default:
			return nil, false
		}
	}
}
