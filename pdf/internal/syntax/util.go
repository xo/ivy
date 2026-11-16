// Copyright (c) The Hayro Authors
//
// Use of this source code is governed by the Apache-2.0 or MIT license that
// can be found in the LICENSE-APACHE and LICENSE-MIT files.

// This file ports hayro-syntax src/util.rs.
//
// Port: the trait ByteBuf of upstream abstracts over Vec and SmallVec. Go
// uses a slice of bytes, so the trait falls away. The paths of the feature
// unsafe, which use memchr, fall away too (D16).

package syntax

import (
	"bytes"
	"fmt"
	"math"
	"strconv"
	"strings"
	"sync/atomic"
	"unicode"
	"unicode/utf8"
)

// scalarNearlyZero is SCALAR_NEARLY_ZERO in hayro-syntax src/util.rs.
const scalarNearlyZero = float32(1.0) / float32(1<<12)

// isNearlyZero is FloatExt::is_nearly_zero in hayro-syntax src/util.rs.
func isNearlyZero(v float32) bool {
	return isNearlyZeroWithinTolerance(v, scalarNearlyZero)
}

// isNearlyZeroWithinTolerance is FloatExt::is_nearly_zero_within_tolerance
// in hayro-syntax src/util.rs.
func isNearlyZeroWithinTolerance(v, tolerance float32) bool {
	return float32(math.Abs(float64(v))) <= tolerance
}

// findNeedle is find_needle in hayro-syntax src/util.rs. It returns the
// offset of the first needle in haystack.
func findNeedle(haystack, needle []byte) (int, bool) {
	i := bytes.Index(haystack, needle)
	return i, i >= 0
}

// findrNeedle is findr_needle in hayro-syntax src/util.rs. It returns the
// offset of the last needle in haystack.
func findrNeedle(haystack, needle []byte) (int, bool) {
	i := bytes.LastIndex(haystack, needle)
	return i, i >= 0
}

// segmentList is SegmentList in hayro-syntax src/util.rs, from
// https://matklad.github.io/2023/04/23/data-oriented-parallel-value-interner.html.
// It is a list whose items keep their address, with a read that takes no
// lock. Segment s holds 2^s items.
//
// Port: the number of segments is a field, since Go has no constant
// generics. Upstream panics past the index 2^C - 1. Go returns nil there.
type segmentList[T any] struct {
	segments []atomic.Pointer[[]onceCell[T]]
}

// newSegmentList is SegmentList::new in hayro-syntax src/util.rs.
func newSegmentList[T any](c int) *segmentList[T] {
	return &segmentList[T]{segments: make([]atomic.Pointer[[]onceCell[T]], c)}
}

// get is SegmentList::get in hayro-syntax src/util.rs.
func (l *segmentList[T]) get(i int) (*T, bool) {
	s, k := l.locate(i)
	if s >= len(l.segments) {
		return nil, false
	}
	segment := l.segments[s].Load()
	if segment == nil {
		return nil, false
	}
	return (*segment)[k].get()
}

// getOrInit is SegmentList::get_or_init in hayro-syntax src/util.rs.
func (l *segmentList[T]) getOrInit(i int, f func() T) *T {
	s, k := l.locate(i)
	if s >= len(l.segments) {
		return nil
	}
	segment := l.segments[s].Load()
	if segment == nil {
		fresh := make([]onceCell[T], 1<<s)
		l.segments[s].CompareAndSwap(nil, &fresh)
		segment = l.segments[s].Load()
	}
	return (*segment)[k].getOrInit(f)
}

// locate is SegmentList::locate in hayro-syntax src/util.rs.
func (l *segmentList[T]) locate(i int) (int, int) {
	power := nextPowerOfTwo(uint(i)+2) / 2
	s := 0
	for p := power; p > 1; p >>= 1 {
		s++
	}
	k := i - (1<<s - 1)
	return s, k
}

// nextPowerOfTwo is usize::next_power_of_two of Rust.
func nextPowerOfTwo(v uint) uint {
	p := uint(1)
	for p < v {
		p <<= 1
	}
	return p
}

// rustDebugStr formats s as the Debug text of str in Rust: in double quotes,
// with escapes.
//
// Port: Go has no Debug trait. The text of the derived Debug of upstream
// shows in the output of GoString.
func rustDebugStr(s string) string {
	var b strings.Builder
	b.WriteByte('"')
	for _, r := range s {
		switch r {
		case '"':
			b.WriteString(`\"`)
		case '\\':
			b.WriteString(`\\`)
		case '\n':
			b.WriteString(`\n`)
		case '\r':
			b.WriteString(`\r`)
		case '\t':
			b.WriteString(`\t`)
		case 0:
			b.WriteString(`\0`)
		default:
			if r == utf8.RuneError || !unicode.IsPrint(r) && r != ' ' {
				fmt.Fprintf(&b, `\u{%x}`, r)
			} else {
				b.WriteRune(r)
			}
		}
	}
	b.WriteByte('"')
	return b.String()
}

// rustDebugBytes formats b as the Debug text of [u8] in Rust, such as
// [0, 255].
func rustDebugBytes(b []byte) string {
	var sb strings.Builder
	sb.WriteByte('[')
	for i, c := range b {
		if i > 0 {
			sb.WriteString(", ")
		}
		sb.WriteString(strconv.Itoa(int(c)))
	}
	sb.WriteByte(']')
	return sb.String()
}

// displayAny formats v as the Display text of upstream.
func displayAny(v any) string {
	switch v := v.(type) {
	case fmt.Stringer:
		return v.String()
	case float32:
		return strconv.FormatFloat(float64(v), 'g', -1, 32)
	}
	return fmt.Sprint(v)
}

// debugAny formats v as the Debug text of upstream.
func debugAny(v any) string {
	if g, ok := v.(fmt.GoStringer); ok {
		return g.GoString()
	}
	return fmt.Sprint(v)
}
