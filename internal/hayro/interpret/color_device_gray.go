// Copyright (c) The Hayro Authors.
//
// Use of this source code is governed by the Apache-2.0 or MIT license that
// can be found in the LICENSE-APACHE and LICENSE-MIT files.

// This file ports hayro-interpret src/color/device_gray.rs.

package interpret

// deviceGray is DeviceGray in hayro-interpret src/color/device_gray.rs.
type deviceGray struct{}

// deviceGraySpace is the one value of deviceGray.
var deviceGraySpace = &deviceGray{}

// convert is the ToRgb implementation of DeviceGray in hayro-interpret
// src/color/device_gray.rs.
func (*deviceGray) convert(input, output []uint8) bool {
	n := min(len(input), len(output)/3)
	for i := range n {
		g := input[i]
		output[i*3], output[i*3+1], output[i*3+2] = g, g, g
	}
	return true
}

// convertInPlace is the default of ToRgb::convert_in_place.
func (*deviceGray) convertInPlace([]uint8) bool { return false }

// isNone is the default of ToRgb::is_none.
func (*deviceGray) isNone() bool { return false }

// toLuma is the ToLuma implementation of DeviceGray in hayro-interpret
// src/color/device_gray.rs.
func (*deviceGray) toLuma([]uint8) bool { return true }
