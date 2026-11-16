// Copyright (c) The Hayro Authors.
//
// Use of this source code is governed by the Apache-2.0 or MIT license that
// can be found in the LICENSE-APACHE and LICENSE-MIT files.

// This file ports hayro-ccitt src/codes.rs.

package ccitt

import (
	"math"
	"math/bits"
)

// The kinds of a mode.
const (
	modePass uint8 = iota
	modeHorizontal
	modeVertical
)

// mode is Mode in hayro-ccitt src/codes.rs, a coding mode of T.6.
//
// Port: the enum of upstream is a struct. The offset is the data of the
// variant Vertical.
type mode struct {
	kind   uint8
	offset int8
}

// decodeRun is BitReader::decode_run in hayro-ccitt src/codes.rs. It decodes
// a run of the color c.
func (r *bitReader) decodeRun(c color) (uint32, error) {
	if c == white {
		return r.decodeRunInner(&whiteLookup)
	}
	return r.decodeRunInner(&blackLookup)
}

// decodeMode is BitReader::decode_mode in hayro-ccitt src/codes.rs. It
// decodes a mode code of T.6 Table 1.
func (r *bitReader) decodeMode() (mode, error) {
	b := r.peekU32()

	if b>>31 == 1 {
		if err := r.consume(1); err != nil {
			return mode{}, err
		}
		return mode{kind: modeVertical}, nil
	}

	zeros := bits.LeadingZeros32(b)
	var offset int8 = -1
	// A shift of 32 bits or more gives 0 in Go, as checked_shl gives None in
	// Rust.
	if (b<<uint(zeros+1))>>31 == 1 {
		offset = 1
	}

	var m mode
	var n int
	switch zeros {
	case 1:
		m, n = mode{kind: modeVertical, offset: offset}, 3
	case 2:
		m, n = mode{kind: modeHorizontal}, 3
	case 3:
		m, n = mode{kind: modePass}, 4
	case 4:
		m, n = mode{kind: modeVertical, offset: 2 * offset}, 6
	case 5:
		m, n = mode{kind: modeVertical, offset: 3 * offset}, 7
	default:
		if err := r.consume(6); err != nil {
			return mode{}, err
		}
		return mode{}, ErrInvalidCode
	}

	if err := r.consume(n); err != nil {
		return mode{}, err
	}

	return m, nil
}

// readEOLIfAvailable is BitReader::read_eol_if_available in hayro-ccitt
// src/codes.rs. It reads all the EOL codes at the position.
func (r *bitReader) readEOLIfAvailable() int {
	return r.readEOLs(math.MaxInt)
}

// readEOLs is BitReader::read_eols in hayro-ccitt src/codes.rs. It reads up
// to maxCount EOL codes, and it returns the number that it read.
func (r *bitReader) readEOLs(maxCount int) int {
	count := 0

	for count < maxCount {
		zeros, after, ok := r.zeroRun()
		if !ok || zeros < 11 {
			break
		}

		*r = after
		count++
	}

	return count
}

// skipToEOL is BitReader::skip_to_eol in hayro-ccitt src/codes.rs. It moves
// the reader to the start of the next EOL code. It returns false, and it
// does not move the reader, if the data holds no more EOL codes.
func (r *bitReader) skipToEOL() bool {
	reader := *r

	for {
		zeros, after, ok := reader.zeroRun()
		if !ok {
			return false
		}

		if zeros >= 11 {
			*r = reader
			return true
		}

		reader = after
	}
}

// decodeRunInner is BitReader::decode_run_inner in hayro-ccitt
// src/codes.rs. It adds make-up codes until it reads a terminating code.
func (r *bitReader) decodeRunInner(lookup *[1 << lookupBits]uint16) (uint32, error) {
	var total uint32

	for {
		entry := lookup[r.peekU32()>>(32-lookupBits)]
		if err := r.consume(int(entry >> 12)); err != nil {
			return 0, err
		}

		n := entry & 0xFFF

		if n == lookupInvalid {
			return 0, ErrInvalidCode
		}

		if total > math.MaxUint32-uint32(n) {
			return 0, ErrOverflow
		}
		total += uint32(n)

		if n < 64 {
			return total, nil
		}
	}
}

// zeroRun is BitReader::zero_run in hayro-ccitt src/codes.rs. It counts the
// zero bits before the next one bit. It returns the reader after the one
// bit, and false when the data ends first.
func (r *bitReader) zeroRun() (int, bitReader, bool) {
	after := *r
	zeros := 0

	for {
		bit, err := after.readBit()
		switch {
		case err != nil:
			return zeros, after, false
		case bit == 0:
			zeros++
		default:
			return zeros, after, true
		}
	}
}
