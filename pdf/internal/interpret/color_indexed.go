// Copyright (c) The Hayro Authors.
//
// Use of this source code is governed by the Apache-2.0 or MIT license that
// can be found in the LICENSE-APACHE and LICENSE-MIT files.

// This file ports hayro-interpret src/color/indexed.rs.

package interpret

import "github.com/xo/ivy/pdf/internal/syntax"

// indexedSpace is Indexed in hayro-interpret src/color/indexed.rs.
type indexedSpace struct {
	values     [][]uint8
	hival      uint8
	base       ColorSpace
	lookup     u8Lookup[[3]uint8]
	lumaLookup u8Lookup[uint8]
}

// newIndexed is Indexed::new in hayro-interpret src/color/indexed.rs.
func newIndexed(array syntax.Array, cache *Cache, depth uint8) (*indexedSpace, bool) {
	iter := array.FlexIter()
	if _, ok := iter.Next[syntax.Name](); !ok {
		return nil, false
	}
	baseObj, ok := iter.Next[syntax.Object]()
	if !ok {
		return nil, false
	}
	base, ok := newColorSpaceNested(baseObj, cache, depth)
	if !ok {
		return nil, false
	}
	hival32, ok := iter.Next[uint32]()
	if !ok {
		return nil, false
	}
	hival := uint8(min(hival32, 255))
	var data []byte
	decoded := false
	if s, ok := iter.Next[syntax.Stream](); ok {
		if d, err := s.DecodedData(); err == nil {
			data, decoded = d, true
		}
	}
	if !decoded {
		if data, ok = nextString(iter); !ok {
			return nil, false
		}
	}
	numComponents := int(base.NumComponents())
	values := make([][]uint8, 0, int(hival)+1)
	pos := 0
	for range int(hival) + 1 {
		if pos+numComponents > len(data) {
			return nil, false
		}
		values = append(values, append([]uint8(nil), data[pos:pos+numComponents]...))
		pos += numComponents
	}
	return &indexedSpace{values: values, hival: hival, base: base}, true
}

// nextString reads the next item of the iterator as a string, and returns
// its bytes.
func nextString(iter *syntax.FlexArrayIter) ([]byte, bool) {
	s, ok := iter.Next[syntax.String]()
	if !ok {
		return nil, false
	}
	return append([]byte(nil), s.AsBytes()...), true
}

// convertInner is Indexed::convert_inner in hayro-interpret
// src/color/indexed.rs.
func (x *indexedSpace) convertInner(input, output []uint8) bool {
	nc := int(x.base.NumComponents())
	indexed := make([]uint8, len(input)*nc)
	for i, v := range input {
		idx := min(v, x.hival)
		copy(indexed[i*nc:(i+1)*nc], x.values[idx])
	}
	return x.base.convert(indexed, output)
}

// u8Lookup is Indexed::u8_lookup in hayro-interpret src/color/indexed.rs.
func (x *indexedSpace) u8Lookup() *[256][3]uint8 {
	return rgbLookupGetOrInit(&x.lookup, x.convertInner)
}

// u8LumaLookup is Indexed::u8_luma_lookup in hayro-interpret
// src/color/indexed.rs. It fails when a color of the palette is not gray.
func (x *indexedSpace) u8LumaLookup() *[256]uint8 {
	return x.lumaLookup.getOrInitWith(func(*[256]uint8) *[256]uint8 {
		rgb := x.u8Lookup()
		if rgb == nil {
			return nil
		}
		var table [256]uint8
		for i, p := range rgb {
			if p[0] != p[1] || p[0] != p[2] {
				return nil
			}
			table[i] = p[0]
		}
		return &table
	})
}

// convert is the ToRgb implementation of Indexed in hayro-interpret
// src/color/indexed.rs.
func (x *indexedSpace) convert(input, output []uint8) bool {
	lookup := x.u8Lookup()
	if lookup == nil {
		return false
	}
	n := min(len(input), len(output)/3)
	for i := range n {
		copy(output[i*3:i*3+3], lookup[input[i]][:])
	}
	return true
}

// convertInPlace is the default of ToRgb::convert_in_place.
func (x *indexedSpace) convertInPlace([]uint8) bool { return false }

// isNone is the default of ToRgb::is_none.
func (x *indexedSpace) isNone() bool { return false }

// toLuma is the ToLuma implementation of Indexed in hayro-interpret
// src/color/indexed.rs.
func (x *indexedSpace) toLuma(input []uint8) bool {
	lookup := x.u8LumaLookup()
	if lookup == nil {
		return false
	}
	for i, v := range input {
		input[i] = lookup[v]
	}
	return true
}
