// Copyright (c) The Hayro Authors.
//
// Use of this source code is governed by the Apache-2.0 or MIT license that
// can be found in the LICENSE-APACHE and LICENSE-MIT files.

// This file ports hayro-interpret src/color/pattern.rs.

package interpret

// patternSpace is Pattern in hayro-interpret src/color/pattern.rs. It is the
// color space Pattern, with the color space of its uncolored patterns.
//
// Port: the type Pattern of src/pattern.rs has the Go name Pattern.
type patternSpace struct {
	cs ColorSpace
}

// colorSpace is Pattern::color_space in hayro-interpret
// src/color/pattern.rs.
func (p *patternSpace) colorSpace() ColorSpace {
	return p.cs
}

// initialColor is Pattern::initial_color in hayro-interpret
// src/color/pattern.rs.
func (p *patternSpace) initialColor() ColorComponents {
	return p.cs.initialColor()
}

// numComponents is Pattern::num_components in hayro-interpret
// src/color/pattern.rs.
func (p *patternSpace) numComponents() uint8 {
	return p.cs.NumComponents()
}

// convert is the ToRgb implementation of Pattern in hayro-interpret
// src/color/pattern.rs.
func (p *patternSpace) convert(input, output []uint8) bool {
	return p.cs.convert(input, output)
}

// convertInPlace is the ToRgb implementation of Pattern in hayro-interpret
// src/color/pattern.rs.
func (p *patternSpace) convertInPlace(input []uint8) bool {
	return p.cs.convertInPlace(input)
}

// isNone is the default of ToRgb::is_none.
func (p *patternSpace) isNone() bool { return false }

// toLuma is the ToLuma implementation of Pattern in hayro-interpret
// src/color/pattern.rs.
func (p *patternSpace) toLuma(input []uint8) bool {
	return p.cs.toLuma(input)
}
