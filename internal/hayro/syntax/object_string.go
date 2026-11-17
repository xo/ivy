// Copyright (c) The Hayro Authors
//
// Use of this source code is governed by the Apache-2.0 or MIT license that
// can be found in the LICENSE-APACHE and LICENSE-MIT files.

// This file ports hayro-syntax src/object/string.rs.

package syntax

import (
	"bytes"
	"unicode/utf8"
)

// String is String in hayro-syntax src/object/string.rs. It is a PDF string
// object, with its escapes decoded and its encryption removed.
type String []byte

// AsBytes is String::as_bytes in hayro-syntax src/object/string.rs.
func (s String) AsBytes() []byte {
	return s
}

// Equal reports whether two strings hold the same bytes, as the PartialEq of
// upstream does.
func (s String) Equal(other String) bool {
	return bytes.Equal(s, other)
}

// String returns the Display text of upstream: the quoted text when the
// string is UTF-8, and the list of bytes otherwise.
func (s String) String() string {
	if utf8.Valid(s) {
		return rustDebugStr(string(s))
	}
	return rustDebugBytes(s)
}

// GoString returns the Debug text of upstream.
func (s String) GoString() string {
	return rustDebugBytes(s)
}

// skipString is the Skippable implementation of String in hayro-syntax
// src/object/string.rs.
func skipString(r *Reader) bool {
	b, ok := r.PeekByte()
	if !ok {
		return false
	}
	switch b {
	case '<':
		return skipHex(r)
	case '(':
		return skipLiteral(r)
	}
	return false
}

// readString is the Readable implementation of String in hayro-syntax
// src/object/string.rs.
func readString(r *Reader, ctx *ReaderContext) (String, bool) {
	b, ok := r.PeekByte()
	if !ok {
		return nil, false
	}
	var decoded []byte
	switch b {
	case '<':
		decoded, ok = readHex(r)
	case '(':
		decoded, ok = readLiteral(r)
	default:
		return nil, false
	}
	if !ok {
		return nil, false
	}
	if xref := ctx.XRef(); xref.needsDecryption(ctx) {
		if id, ok := ctx.objNumberCtx(); ok {
			if d, ok := xref.decrypt(id, decoded, decryptionTarget{kind: targetString}); ok {
				decoded = d
			}
		}
	}
	return String(decoded), true
}

// skipHex is skip_hex in hayro-syntax src/object/string.rs.
func skipHex(r *Reader) bool {
	if !r.ForwardTag("<") {
		return false
	}
	for {
		b, ok := r.PeekByte()
		if !ok {
			break
		}
		if !isHexDigit(b) && !isWhiteSpaceCharacter(b) {
			break
		}
		r.Forward()
	}
	return r.ForwardTag(">")
}

// readHex is read_hex in hayro-syntax src/object/string.rs.
func readHex(r *Reader) ([]byte, bool) {
	start := r.offset
	if !skipHex(r) {
		return nil, false
	}
	raw, ok := r.Range(start+1, r.offset-1)
	if !ok {
		return nil, false
	}
	return decodeASCIIHexInto(raw)
}

// skipLiteral is skip_literal in hayro-syntax src/object/string.rs.
func skipLiteral(r *Reader) bool {
	if !r.ForwardTag("(") {
		return false
	}
	bracketCounter := 1
	for bracketCounter > 0 {
		b, ok := r.ReadU8()
		if !ok {
			return false
		}
		switch b {
		case '\\':
			if _, ok := r.ReadU8(); !ok {
				return false
			}
		case '(':
			bracketCounter++
		case ')':
			bracketCounter--
		}
	}
	return true
}

// readLiteral is read_literal in hayro-syntax src/object/string.rs.
func readLiteral(r *Reader) ([]byte, bool) {
	start := r.offset
	if !skipLiteral(r) {
		return nil, false
	}
	data, ok := r.Range(start+1, r.offset-1)
	if !ok {
		return nil, false
	}
	if !bytes.ContainsAny(data, "\\\n\r") {
		return data, true
	}
	lr := NewReader(data)
	result := make([]byte, 0, len(data))
	for {
		if lr.ReadEOL() {
			result = append(result, '\n')
			continue
		}
		b, ok := lr.ReadU8()
		if !ok {
			break
		}
		if b != '\\' {
			result = append(result, b)
			continue
		}
		if lr.ReadEOL() {
			continue
		}
		next, ok := lr.ReadU8()
		if !ok {
			return nil, false
		}
		if !isOctalDigit(next) {
			switch next {
			case 'n':
				result = append(result, 0xA)
			case 'r':
				result = append(result, 0xD)
			case 't':
				result = append(result, 0x9)
			case 'b':
				result = append(result, 0x8)
			case 'f':
				result = append(result, 0xC)
			default:
				result = append(result, next)
			}
			continue
		}
		second, okSecond := lr.ReadU8()
		third, okThird := lr.ReadU8()
		var digits [3]byte
		switch {
		case okSecond && okThird:
			switch {
			case isOctalDigit(second) && isOctalDigit(third):
				digits = [3]byte{next, second, third}
			case isOctalDigit(second):
				lr.Jump(lr.Offset() - 1)
				digits = [3]byte{'0', next, second}
			default:
				lr.Jump(lr.Offset() - 2)
				digits = [3]byte{'0', '0', next}
			}
		case okSecond:
			if isOctalDigit(second) {
				digits = [3]byte{'0', next, second}
			} else {
				lr.Jump(lr.Offset() - 1)
				digits = [3]byte{'0', '0', next}
			}
		default:
			digits = [3]byte{'0', '0', next}
		}
		num := int(digits[0]-'0')*64 + int(digits[1]-'0')*8 + int(digits[2]-'0')
		// A value above 255 does not fit into u8, so upstream drops it.
		if num <= 0xFF {
			result = append(result, byte(num))
		}
	}
	return result, true
}

// isOctalDigit is is_octal_digit in hayro-syntax src/object/string.rs.
func isOctalDigit(b byte) bool {
	return b >= '0' && b <= '7'
}

// isHexDigit is u8::is_ascii_hexdigit of Rust.
func isHexDigit(b byte) bool {
	_, ok := decodeHexDigit(b)
	return ok
}
