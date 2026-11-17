// Copyright (c) The Hayro Authors
//
// Use of this source code is governed by the Apache-2.0 or MIT license that
// can be found in the LICENSE-APACHE and LICENSE-MIT files.

// This file ports hayro-syntax src/object/date.rs.

package syntax

import "strconv"

// DateTime is DateTime in hayro-syntax src/object/date.rs. It is a date and
// a time, as a PDF date string holds them.
type DateTime struct {
	// Year is the year.
	Year uint16
	// Month is the month.
	Month uint8
	// Day is the day.
	Day uint8
	// Hour is the hour.
	Hour uint8
	// Minute is the minute.
	Minute uint8
	// Second is the second.
	Second uint8
	// UTCOffsetHour is the offset from UTC in hours.
	UTCOffsetHour int8
	// UTCOffsetMinute is the offset from UTC in minutes.
	UTCOffsetMinute uint8
}

// DateTimeFromBytes is DateTime::from_bytes in hayro-syntax
// src/object/date.rs. It reads a date of the form D:YYYYMMDDHHmmSSOHH'mm.
// Each part after the year is optional.
func DateTimeFromBytes(b []byte) (DateTime, bool) {
	r := NewReader(b)
	if !r.ForwardTag("D:") {
		return DateTime{}, false
	}
	// readNum reads n digits between lo and hi, or returns def. A failed
	// read still moves the reader past the digits, as upstream does.
	readNum := func(n int, lo, hi, def uint64) (uint64, bool) {
		c, ok := r.PeekByte()
		if !ok || c == '-' || c == '+' || c == 'Z' {
			return def, false
		}
		digits, ok := r.ReadBytes(n)
		if !ok {
			return def, false
		}
		v, err := strconv.ParseUint(string(digits), 10, 16)
		if err != nil || v < lo || v > hi {
			return def, false
		}
		return v, true
	}
	year, ok := readNum(4, 0, 9999, 0)
	if !ok {
		return DateTime{}, false
	}
	month, _ := readNum(2, 1, 12, 1)
	day, _ := readNum(2, 1, 31, 1)
	hour, _ := readNum(2, 0, 23, 0)
	minute, _ := readNum(2, 0, 59, 0)
	second, _ := readNum(2, 0, 59, 0)
	var offsetHour int8
	var offsetMinute uint8
	if !r.AtEnd() {
		sign, _ := r.ReadU8()
		multiplier := int8(1)
		if sign == '-' {
			multiplier = -1
		}
		h, _ := readNum(2, 0, 23, 0)
		offsetHour = multiplier * int8(h)
		r.ForwardTag("'")
		m, _ := readNum(2, 0, 59, 0)
		offsetMinute = uint8(m)
	}
	return DateTime{
		Year:            uint16(year),
		Month:           uint8(month),
		Day:             uint8(day),
		Hour:            uint8(hour),
		Minute:          uint8(minute),
		Second:          uint8(second),
		UTCOffsetHour:   offsetHour,
		UTCOffsetMinute: offsetMinute,
	}, true
}
