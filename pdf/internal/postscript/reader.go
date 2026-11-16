// Copyright (c) The Hayro Authors.
//
// Use of this source code is governed by the Apache-2.0 or MIT license that
// can be found in the LICENSE-APACHE and LICENSE-MIT files.

// This file ports hayro-postscript src/reader.rs.

package postscript

// reader is Reader in hayro-postscript src/reader.rs. It reads bytes from a
// slice and keeps an offset into it. The offset can move past the end of the
// data. Then each read fails.
type reader struct {
	data   []byte
	offset int
}

// newReader is Reader::new in hayro-postscript src/reader.rs.
func newReader(data []byte) *reader {
	return &reader{data: data}
}

// jump is Reader::jump in hayro-postscript src/reader.rs.
func (r *reader) jump(offset int) {
	r.offset = offset
}

// rangeOf is Reader::range in hayro-postscript src/reader.rs. It returns the
// bytes from start to end, with end excluded. It returns false if the range
// is not in the data.
func (r *reader) rangeOf(start, end int) ([]byte, bool) {
	if start < 0 || start > end || end > len(r.data) {
		return nil, false
	}
	return r.data[start:end], true
}

// getOffset is Reader::offset in hayro-postscript src/reader.rs.
func (r *reader) getOffset() int {
	return r.offset
}

// readBytes is Reader::read_bytes in hayro-postscript src/reader.rs.
func (r *reader) readBytes(n int) ([]byte, bool) {
	v, ok := r.peekBytes(n)
	if !ok {
		return nil, false
	}
	r.offset += n
	return v, true
}

// readByte is Reader::read_byte in hayro-postscript src/reader.rs.
func (r *reader) readByte() (byte, bool) {
	v, ok := r.peekByte()
	if !ok {
		return 0, false
	}
	r.offset++
	return v, true
}

// peekBytes is Reader::peek_bytes in hayro-postscript src/reader.rs.
func (r *reader) peekBytes(n int) ([]byte, bool) {
	if n < 0 || r.offset < 0 || r.offset > len(r.data) || n > len(r.data)-r.offset {
		return nil, false
	}
	return r.data[r.offset : r.offset+n], true
}

// peekByte is Reader::peek_byte in hayro-postscript src/reader.rs.
func (r *reader) peekByte() (byte, bool) {
	if r.offset < 0 || r.offset >= len(r.data) {
		return 0, false
	}
	return r.data[r.offset], true
}

// eat is Reader::eat in hayro-postscript src/reader.rs. It reads the next
// byte if f accepts it.
func (r *reader) eat(f func(byte) bool) (byte, bool) {
	v, ok := r.peekByte()
	if !ok || !f(v) {
		return 0, false
	}
	r.forward()
	return v, true
}

// forward is Reader::forward in hayro-postscript src/reader.rs.
func (r *reader) forward() {
	r.offset++
}

// forwardWhile is Reader::forward_while in hayro-postscript src/reader.rs.
func (r *reader) forwardWhile(f func(byte) bool) {
	for {
		b, ok := r.peekByte()
		if !ok || !f(b) {
			return
		}
		r.forward()
	}
}

// forwardTag is Reader::forward_tag in hayro-postscript src/reader.rs. It
// moves past tag if the data continues with tag.
func (r *reader) forwardTag(tag string) bool {
	if !r.peekTag(tag) {
		return false
	}
	r.offset += len(tag)
	return true
}

// peekTag is Reader::peek_tag in hayro-postscript src/reader.rs.
func (r *reader) peekTag(tag string) bool {
	b, ok := r.peekBytes(len(tag))
	return ok && string(b) == tag
}

// skipEOL is Reader::skip_eol in hayro-postscript src/reader.rs. It skips
// each end of line byte that follows.
func (r *reader) skipEOL() {
	r.forwardWhile(isEOL)
}

// isWhitespace is is_whitespace in hayro-postscript src/reader.rs.
func isWhitespace(b byte) bool {
	switch b {
	case 0x00, 0x09, 0x0a, 0x0c, 0x0d, 0x20:
		return true
	}
	return false
}

// isDelimiter is is_delimiter in hayro-postscript src/reader.rs.
func isDelimiter(b byte) bool {
	switch b {
	case '(', ')', '<', '>', '[', ']', '{', '}', '/', '%':
		return true
	}
	return false
}

// isRegular is is_regular in hayro-postscript src/reader.rs.
func isRegular(b byte) bool {
	return !isWhitespace(b) && !isDelimiter(b)
}

// isEOL is is_eol in hayro-postscript src/reader.rs.
func isEOL(b byte) bool {
	return b == 0x0a || b == 0x0d
}
