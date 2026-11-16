// Copyright (c) The Hayro Authors
//
// Use of this source code is governed by the Apache-2.0 or MIT license that
// can be found in the LICENSE-APACHE and LICENSE-MIT files.

// This file ports hayro-syntax src/filter/png/paeth.rs, which upstream took
// from the png crate, src/filter/paeth.rs.

package syntax

// filterPaeth is filter_paeth in hayro-syntax src/filter/png/paeth.rs.
func filterPaeth(a, b, c byte) byte {
	pa := absInt16(int16(b) - int16(c))
	pb := absInt16(int16(a) - int16(c))
	pc := absInt16((int16(a) - int16(c)) + (int16(b) - int16(c)))
	out := a
	least := pa
	if pb < least {
		least = pb
		out = b
	}
	if pc < least {
		out = c
	}
	return out
}

// unfilterPaeth is unfilter in hayro-syntax src/filter/png/paeth.rs.
//
// Port: Go ports the scalar code for platforms other than x86_64, with one
// loop for each size of pixel. The code for x86_64 gives the same bytes.
func unfilterPaeth(tbpp int, previous, current []byte) {
	// The pixels of the Paeth filter:
	//
	//	C B D
	//	A X
	n := min(len(current)-len(current)%tbpp, len(previous)-len(previous)%tbpp)
	for i := range n {
		var a, c byte
		if i >= tbpp {
			a = current[i-tbpp]
			c = previous[i-tbpp]
		}
		current[i] += filterPaeth(a, previous[i], c)
	}
}

// absInt16 is i16::abs of Rust.
func absInt16(v int16) int16 {
	if v < 0 {
		return -v
	}
	return v
}
