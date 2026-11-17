// Copyright (c) The Hayro Authors.
//
// Use of this source code is governed by the Apache-2.0 or MIT license that
// can be found in the LICENSE-APACHE and LICENSE-MIT files.

// This file ports hayro-postscript src/string/literal.rs.

package postscript

// decodeLiteral is decode_into in hayro-postscript src/string/literal.rs. It
// appends the bytes of a string in parentheses to out, and it resolves the
// escapes and the ends of lines. It returns false if the data ends with a
// backslash.
func decodeLiteral(data, out []byte) ([]byte, bool) {
	r := newReader(data)

	for {
		b, ok := r.readByte()
		if !ok {
			return out, true
		}

		switch b {
		case '\\':
			next, ok := r.readByte()
			if !ok {
				return out, false
			}

			if isOctalDigit(next) {
				digits := readOctalEscape(r, next)
				value := int(digits[0]-'0')*64 + int(digits[1]-'0')*8 + int(digits[2]-'0')
				// A value over 255 does not fit in a byte. Upstream drops it.
				if value <= 0xff {
					out = append(out, byte(value))
				}
				continue
			}

			switch next {
			case 'n':
				out = append(out, 0x0a)
			case 'r':
				out = append(out, 0x0d)
			case 't':
				out = append(out, 0x09)
			case 'b':
				out = append(out, 0x08)
			case 'f':
				out = append(out, 0x0c)
			case '(', ')', '\\':
				out = append(out, next)
			case '\n', '\r':
				r.skipEOL()
			default:
				out = append(out, next)
			}
		case '\n', '\r':
			out = append(out, '\n')
			r.skipEOL()
		default:
			out = append(out, b)
		}
	}
}

// readOctalEscape is the octal branch of decode_into in hayro-postscript
// src/string/literal.rs. It reads up to two more octal digits after next,
// and it returns three digits with leading zeros.
//
// Port: upstream writes the branch inside decode_into.
func readOctalEscape(r *reader, next byte) [3]byte {
	n1, ok1 := r.readByte()
	n2, ok2 := r.readByte()

	switch {
	case ok1 && ok2:
		switch {
		case isOctalDigit(n1) && isOctalDigit(n2):
			return [3]byte{next, n1, n2}
		case isOctalDigit(n1):
			r.jump(r.getOffset() - 1)
			return [3]byte{'0', next, n1}
		default:
			r.jump(r.getOffset() - 2)
			return [3]byte{'0', '0', next}
		}
	case ok1:
		if isOctalDigit(n1) {
			return [3]byte{'0', next, n1}
		}
		r.jump(r.getOffset() - 1)
		return [3]byte{'0', '0', next}
	default:
		return [3]byte{'0', '0', next}
	}
}

// isOctalDigit is is_octal_digit in hayro-postscript src/string/literal.rs.
func isOctalDigit(b byte) bool {
	return '0' <= b && b <= '7'
}
