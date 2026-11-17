// Copyright (c) The Hayro Authors.
//
// Use of this source code is governed by the Apache-2.0 or MIT license that
// can be found in the LICENSE-APACHE and LICENSE-MIT files.

// This file ports hayro-interpret src/color/device_rgb.rs.

package interpret

// deviceRGB is DeviceRgb in hayro-interpret src/color/device_rgb.rs.
type deviceRGB struct{}

// deviceRGBSpace is the one value of deviceRGB.
var deviceRGBSpace = &deviceRGB{}

// convert is the ToRgb implementation of DeviceRgb in hayro-interpret
// src/color/device_rgb.rs.
//
// Port: upstream panics when the two buffers differ in length. Go copies
// the shorter length.
func (*deviceRGB) convert(input, output []uint8) bool {
	copy(output, input)
	return true
}

// convertInPlace is the ToRgb implementation of DeviceRgb in
// hayro-interpret src/color/device_rgb.rs.
func (*deviceRGB) convertInPlace([]uint8) bool { return true }

// isNone is the default of ToRgb::is_none.
func (*deviceRGB) isNone() bool { return false }
