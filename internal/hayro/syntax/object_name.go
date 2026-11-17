// Copyright (c) The Hayro Authors
//
// Use of this source code is governed by the Apache-2.0 or MIT license that
// can be found in the LICENSE-APACHE and LICENSE-MIT files.

// This file ports hayro-syntax src/object/name.rs.

package syntax

import (
	"bytes"
	"unicode/utf8"
)

// Name is Name in hayro-syntax src/object/name.rs. It is a PDF name object,
// with its escapes decoded.
//
// Port: upstream borrows or owns the bytes. A Go string holds them, so that
// a name compares with == and works as the key of a map.
type Name string

// NewName is Name::new in hayro-syntax src/object/name.rs. It decodes the
// escapes of data.
func NewName(data []byte) (Name, bool) {
	if !bytes.Contains(data, []byte{'#'}) {
		return NewNameUnescaped(data), true
	}
	return NewNameEscaped(data)
}

// NewNameUnescaped is Name::new_unescaped in hayro-syntax
// src/object/name.rs.
func NewNameUnescaped(data []byte) Name {
	return Name(data)
}

// NewNameEscaped is Name::new_escaped in hayro-syntax src/object/name.rs. It
// decodes each escape of the form #xx.
func NewNameEscaped(data []byte) (Name, bool) {
	result := make([]byte, 0, len(data))
	r := NewReader(data)
	for {
		b, ok := r.ReadU8()
		if !ok {
			break
		}
		// Before PDF 1.2, # was a regular character in names.
		if b == '#' {
			if hex, ok := r.PeekBytes(2); ok {
				high, okHigh := decodeHexDigit(hex[0])
				low, okLow := decodeHexDigit(hex[1])
				if okHigh && okLow {
					r.SkipBytes(2)
					result = append(result, high<<4|low)
					continue
				}
			}
		}
		result = append(result, b)
	}
	return Name(result), true
}

// AsStr is Name::as_str in hayro-syntax src/object/name.rs. It returns the
// name, or a placeholder when the name is not UTF-8.
func (n Name) AsStr() string {
	if !utf8.ValidString(string(n)) {
		return "{non-ascii key}"
	}
	return string(n)
}

// String returns the Display text of upstream.
func (n Name) String() string {
	return "/" + n.AsStr()
}

// GoString returns the Debug text of upstream.
func (n Name) GoString() string {
	if utf8.ValidString(string(n)) {
		return rustDebugStr(string(n))
	}
	return rustDebugBytes([]byte(n))
}

// skipName is the Skippable implementation of Name in hayro-syntax
// src/object/name.rs.
func skipName(r *Reader) bool {
	return skipNameLike(r, true)
}

// readName is the Readable implementation of Name in hayro-syntax
// src/object/name.rs.
func readName(r *Reader) (Name, bool) {
	start := r.offset
	if !skipNameLike(r, true) {
		return "", false
	}
	data, ok := r.Range(start+1, r.offset)
	if !ok {
		return "", false
	}
	return NewName(data)
}

// skipNameLike is skip_name_like in hayro-syntax src/object/name.rs.
// Escapes are not checked here. The read of a name checks them.
func skipNameLike(r *Reader, solidus bool) bool {
	if solidus {
		if !r.ForwardTag("/") {
			return false
		}
		r.ForwardWhile(isRegularCharacter)
		return true
	}
	return r.ForwardWhile1(isRegularCharacter)
}
