// Copyright (c) The Hayro Authors.
//
// Use of this source code is governed by the Apache-2.0 or MIT license that
// can be found in the LICENSE-APACHE and LICENSE-MIT files.

// This file ports hayro-interpret src/color/device_n.rs.

package interpret

import "github.com/xo/ivy/internal/hayro/syntax"

// maxCachedColors is MAX_CACHED_COLORS in hayro-interpret
// src/color/device_n.rs.
const maxCachedColors = 1 << 18

// deviceNSpace is DeviceN in hayro-interpret src/color/device_n.rs.
type deviceNSpace struct {
	alternateSpace ColorSpace
	numComponents  uint8
	tintTransform  *Function
	none           bool
	lookup         u8Lookup[[3]uint8]
}

// newDeviceN is DeviceN::new in hayro-interpret src/color/device_n.rs.
func newDeviceN(array syntax.Array, cache *Cache, depth uint8) (*deviceNSpace, bool) {
	iter := array.FlexIter()
	if _, ok := iter.Next[syntax.Name](); !ok {
		return nil, false
	}
	namesArr, ok := iter.Next[syntax.Array]()
	if !ok {
		return nil, false
	}
	names := namesArr.Iter[syntax.Name]().Collect()
	if len(names) > 255 {
		return nil, false
	}
	numComponents := uint8(len(names))
	allNone := true
	for _, n := range names {
		if n != syntax.KeyNone {
			allNone = false
		}
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
	if numComponents == 0 || int(numComponents) > maxComponents {
		return nil, false
	}
	return &deviceNSpace{alternateSpace: alternate, numComponents: numComponents, tintTransform: tint, none: allNone}, true
}

// evaluate is DeviceN::evaluate in hayro-interpret src/color/device_n.rs.
// It converts each pixel into the components of the alternate space, and
// it caches the result of each color of up to 8 components.
func (d *deviceNSpace) evaluate(input []uint8) []uint8 {
	numComponents := int(d.numComponents)
	numEncoded := int(d.alternateSpace.NumComponents())
	components := make([]float32, numComponents)
	values := make([]float32, d.tintTransform.NumOutputs())
	evaluatePixel := func(pixel []uint8) []uint8 {
		for i := range components {
			components[i] = float32(pixel[i]) * (1.0 / 255.0)
		}
		if d.tintTransform.Eval(components, values) {
			return d.alternateSpace.encodeValues(values)
		}
		return d.alternateSpace.encodeValues(d.alternateSpace.initialColor())
	}
	evaluated := make([]uint8, 0, len(input)/numComponents*numEncoded)
	if numComponents > 8 {
		for i := 0; i+numComponents <= len(input); i += numComponents {
			evaluated = append(evaluated, evaluatePixel(input[i:i+numComponents])...)
		}
		return evaluated
	}
	var cached []uint8
	offsets := make(map[uint64]int)
	var prevKey uint64
	prevOffset, hasPrev := 0, false
	for i := 0; i+numComponents <= len(input); i += numComponents {
		pixel := input[i : i+numComponents]
		var key uint64
		for _, v := range pixel {
			key = key<<8 | uint64(v)
		}
		offset, found := 0, false
		switch {
		case hasPrev && prevKey == key:
			offset, found = prevOffset, true
		default:
			if o, ok := offsets[key]; ok {
				offset, found = o, true
			} else if len(offsets) < maxCachedColors {
				offset = len(cached)
				cached = append(cached, evaluatePixel(pixel)...)
				offsets[key] = offset
				found = true
			}
		}
		if found {
			evaluated = append(evaluated, cached[offset:offset+numEncoded]...)
			prevKey, prevOffset, hasPrev = key, offset, true
		} else {
			evaluated = append(evaluated, evaluatePixel(pixel)...)
		}
	}
	return evaluated
}

// convertInner is DeviceN::convert_inner in hayro-interpret
// src/color/device_n.rs.
func (d *deviceNSpace) convertInner(input, output []uint8) bool {
	return d.alternateSpace.convert(d.evaluate(input), output)
}

// u8Lookup is DeviceN::u8_lookup in hayro-interpret src/color/device_n.rs.
func (d *deviceNSpace) u8Lookup() *[256][3]uint8 {
	return rgbLookupGetOrInit(&d.lookup, d.convertInner)
}

// convert is the ToRgb implementation of DeviceN in hayro-interpret
// src/color/device_n.rs.
func (d *deviceNSpace) convert(input, output []uint8) bool {
	if d.numComponents == 1 {
		lookup := d.u8Lookup()
		if lookup == nil {
			return false
		}
		n := min(len(input), len(output)/3)
		for i := range n {
			copy(output[i*3:i*3+3], lookup[input[i]][:])
		}
		return true
	}
	return d.convertInner(input, output)
}

// convertInPlace is the ToRgb implementation of DeviceN in hayro-interpret
// src/color/device_n.rs.
func (d *deviceNSpace) convertInPlace(input []uint8) bool {
	if d.numComponents != 3 {
		return false
	}
	return d.alternateSpace.convert(d.evaluate(input), input)
}

// isNone is the ToRgb implementation of DeviceN in hayro-interpret
// src/color/device_n.rs.
func (d *deviceNSpace) isNone() bool {
	return d.none
}
