// Copyright (c) The Hayro Authors.
//
// Use of this source code is governed by the Apache-2.0 or MIT license that
// can be found in the LICENSE-APACHE and LICENSE-MIT files.

// This file ports hayro-interpret src/color/separation.rs.

package interpret

import "github.com/xo/ivy/internal/hayro/syntax"

// separationSpace is Separation in hayro-interpret
// src/color/separation.rs.
type separationSpace struct {
	alternateSpace   ColorSpace
	tintTransform    *Function
	isNoneSeparation bool
	lookup           u8Lookup[[3]uint8]
}

// newSeparation is Separation::new in hayro-interpret
// src/color/separation.rs.
func newSeparation(array syntax.Array, cache *Cache, depth uint8) (*separationSpace, bool) {
	iter := array.FlexIter()
	if _, ok := iter.Next[syntax.Name](); !ok {
		return nil, false
	}
	name, ok := iter.Next[syntax.Name]()
	if !ok {
		return nil, false
	}
	altObj, ok := iter.Next[syntax.Object]()
	if !ok {
		return nil, false
	}
	alternate, ok := newColorSpaceNested(altObj, cache, depth)
	if !ok {
		return nil, false
	}
	fnObj, ok := iter.Next[syntax.Object]()
	if !ok {
		return nil, false
	}
	tint, ok := NewFunction(fnObj)
	if !ok {
		return nil, false
	}
	// Upstream does not treat All in a special way, as other viewers do
	// not.
	return &separationSpace{alternateSpace: alternate, tintTransform: tint, isNoneSeparation: name == syntax.KeyNone}, true
}

// convertInner is Separation::convert_inner in hayro-interpret
// src/color/separation.rs.
func (s *separationSpace) convertInner(input, output []uint8) bool {
	values := make([]float32, s.tintTransform.NumOutputs())
	evaluated := make([]uint8, 0, len(input)*len(values))
	for _, n := range input {
		if s.tintTransform.Eval([]float32{float32(n) * (1.0 / 255.0)}, values) {
			evaluated = append(evaluated, s.alternateSpace.encodeValues(values)...)
		} else {
			evaluated = append(evaluated, s.alternateSpace.encodeValues(s.alternateSpace.initialColor())...)
		}
	}
	return s.alternateSpace.convert(evaluated, output)
}

// u8Lookup is Separation::u8_lookup in hayro-interpret
// src/color/separation.rs.
func (s *separationSpace) u8Lookup() *[256][3]uint8 {
	return rgbLookupGetOrInit(&s.lookup, s.convertInner)
}

// convert is the ToRgb implementation of Separation in hayro-interpret
// src/color/separation.rs.
func (s *separationSpace) convert(input, output []uint8) bool {
	lookup := s.u8Lookup()
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
func (s *separationSpace) convertInPlace([]uint8) bool { return false }

// isNone is the ToRgb implementation of Separation in hayro-interpret
// src/color/separation.rs.
func (s *separationSpace) isNone() bool {
	return s.isNoneSeparation
}
