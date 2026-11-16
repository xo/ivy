// Copyright 2018 the Kurbo Authors
//
// Use of this source code is governed by the Apache-2.0 or MIT license that
// can be found in the LICENSE-APACHE and LICENSE-MIT files.

// This file ports hayro-syntax src/transform.rs, which upstream took from
// kurbo.

package syntax

// Transform is Transform in hayro-syntax src/transform.rs. It is a 2D affine
// transform with the coefficients a, b, c, d, e and f.
type Transform [6]float64

// TransformIdentity is Transform::IDENTITY in hayro-syntax
// src/transform.rs.
var TransformIdentity = transformScale(1)

// transformRotateCW90 is Transform::ROTATE_CW_90 in hayro-syntax
// src/transform.rs.
var transformRotateCW90 = Transform{0, 1, -1, 0, 0, 0}

// transformRotateCCW90 is Transform::ROTATE_CCW_90 in hayro-syntax
// src/transform.rs.
var transformRotateCCW90 = Transform{0, -1, 1, 0, 0, 0}

// transformScale is Transform::scale in hayro-syntax src/transform.rs.
func transformScale(s float64) Transform {
	return Transform{s, 0, 0, s, 0, 0}
}

// transformTranslate is Transform::translate in hayro-syntax
// src/transform.rs.
func transformTranslate(x, y float64) Transform {
	return Transform{1, 0, 0, 1, x, y}
}

// AsCoeffs is Transform::as_coeffs in hayro-syntax src/transform.rs.
func (t Transform) AsCoeffs() [6]float64 {
	return t
}

// Mul is the Mul implementation of Transform in hayro-syntax
// src/transform.rs. It returns t times other.
func (t Transform) Mul(other Transform) Transform {
	return Transform{
		t[0]*other[0] + t[2]*other[1],
		t[1]*other[0] + t[3]*other[1],
		t[0]*other[2] + t[2]*other[3],
		t[1]*other[2] + t[3]*other[3],
		t[0]*other[4] + t[2]*other[5] + t[4],
		t[1]*other[4] + t[3]*other[5] + t[5],
	}
}
