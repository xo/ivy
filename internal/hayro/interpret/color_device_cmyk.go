// Copyright (c) The Hayro Authors.
//
// Use of this source code is governed by the Apache-2.0 or MIT license that
// can be found in the LICENSE-APACHE and LICENSE-MIT files.

// This file ports hayro-interpret src/color/device_cmyk.rs.

package interpret

import (
	_ "embed"
	"sync"
)

// cmykProfile is the file assets/CGATS001Compat-v2-micro.icc of
// hayro-interpret, under CC0 1.0 Universal.
//
//go:embed assets/CGATS001Compat-v2-micro.icc
var cmykProfile []byte

// deviceCMYK is DeviceCmyk in hayro-interpret src/color/device_cmyk.rs.
type deviceCMYK struct{}

// deviceCMYKSpace is the one value of deviceCMYK.
var deviceCMYKSpace = &deviceCMYK{}

// convert is the ToRgb implementation of DeviceCmyk in hayro-interpret
// src/color/device_cmyk.rs.
func (*deviceCMYK) convert(input, output []uint8) bool {
	t := cmykTransform()
	if t == nil {
		return false
	}
	return t.convert(input, output)
}

// convertInPlace is the default of ToRgb::convert_in_place.
func (*deviceCMYK) convertInPlace([]uint8) bool { return false }

// isNone is the default of ToRgb::is_none.
func (*deviceCMYK) isNone() bool { return false }

// cmykTransform is CMYK_TRANSFORM in hayro-interpret
// src/color/device_cmyk.rs.
//
// Port: upstream panics when the profile cannot be read. Go returns nil,
// and the conversion fails.
var cmykTransform = sync.OnceValue(func() *iccProfile {
	p, ok := newICCProfile(cmykProfile, 4)
	if !ok {
		return nil
	}
	return p
})
