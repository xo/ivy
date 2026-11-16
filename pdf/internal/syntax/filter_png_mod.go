// Copyright (c) The Hayro Authors
//
// Use of this source code is governed by the Apache-2.0 or MIT license that
// can be found in the LICENSE-APACHE and LICENSE-MIT files.

// This file ports hayro-syntax src/filter/png/mod.rs, which upstream took
// from the png crate, src/filter/mod.rs.

package syntax

// RowFilter is RowFilter in hayro-syntax src/filter/png/mod.rs. It is the
// filter of one row of PNG data.
type RowFilter uint8

// The row filters of hayro-syntax src/filter/png/mod.rs.
const (
	// RowFilterNoFilter is RowFilter::NoFilter.
	RowFilterNoFilter RowFilter = 0
	// RowFilterSub is RowFilter::Sub.
	RowFilterSub RowFilter = 1
	// RowFilterUp is RowFilter::Up.
	RowFilterUp RowFilter = 2
	// RowFilterAvg is RowFilter::Avg.
	RowFilterAvg RowFilter = 3
	// RowFilterPaeth is RowFilter::Paeth.
	RowFilterPaeth RowFilter = 4
)

// rowFilterFromU8 is RowFilter::from_u8 in hayro-syntax
// src/filter/png/mod.rs.
func rowFilterFromU8(n byte) (RowFilter, bool) {
	if n > 4 {
		return 0, false
	}
	return RowFilter(n), true
}

// bytesPerPixelNew is BytesPerPixel::new in hayro-syntax
// src/filter/png/mod.rs. It reports whether bpp is a size of pixel that the
// png crate supports: 1, 2, 3, 4, 6 or 8.
//
// Port: the enum BytesPerPixel becomes an int, and one loop handles each
// size where upstream unrolls a loop for each.
func bytesPerPixelNew(bpp int) bool {
	switch bpp {
	case 1, 2, 3, 4, 6, 8:
		return true
	}
	return false
}

// unfilterRow is unfilter_row in hayro-syntax src/filter/png/mod.rs. It
// undoes the filter of the row current, with previous as the row above.
func unfilterRow(filter RowFilter, bpp int, previous, current []byte) {
	start := 0
	if bytesPerPixelNew(bpp) {
		start = len(current) - len(current)%bpp
		prev := previous
		if start <= len(previous) {
			prev = previous[:start]
		}
		unfilterPng(filter, bpp, prev, current[:start])
	}
	for i := start; i < len(current); i++ {
		var left, above, aboveLeft byte
		if i >= bpp {
			left = current[i-bpp]
			if i-bpp < len(previous) {
				aboveLeft = previous[i-bpp]
			}
		}
		if i < len(previous) {
			above = previous[i]
		}
		var prediction byte
		switch filter {
		case RowFilterNoFilter:
		case RowFilterSub:
			prediction = left
		case RowFilterUp:
			prediction = above
		case RowFilterAvg:
			prediction = byte((uint16(left) + uint16(above)) / 2)
		case RowFilterPaeth:
			prediction = filterPaeth(left, above, aboveLeft)
		}
		current[i] += prediction
	}
}

// unfilterPng is unfilter in hayro-syntax src/filter/png/mod.rs. The length
// of current is a multiple of tbpp.
func unfilterPng(filter RowFilter, tbpp int, previous, current []byte) {
	if len(previous) == 0 {
		switch filter {
		case RowFilterPaeth:
			filter = RowFilterSub
		case RowFilterUp:
			filter = RowFilterNoFilter
		}
	}
	switch filter {
	case RowFilterNoFilter:
	case RowFilterSub:
		for i := tbpp; i < len(current); i++ {
			current[i] += current[i-tbpp]
		}
	case RowFilterUp:
		for i := 0; i < len(current) && i < len(previous); i++ {
			current[i] += previous[i]
		}
	case RowFilterAvg:
		if len(previous) == 0 {
			for i := tbpp; i < len(current); i++ {
				current[i] += current[i-tbpp] / 2
			}
			return
		}
		// The chunks stop at the shorter of the two rows.
		n := min(len(current), len(previous)-len(previous)%tbpp)
		for i := range n {
			var left byte
			if i >= tbpp {
				left = current[i-tbpp]
			}
			current[i] += byte((uint16(previous[i]) + uint16(left)) / 2)
		}
	case RowFilterPaeth:
		unfilterPaeth(tbpp, previous, current)
	}
}
