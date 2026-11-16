// Copyright (c) 2011-2021, Simon Sapin and contributors.
//
// Use of this source code is governed by the BSD-3-Clause license that can be
// found in the LICENSE file.

// This file ports weasyprint/matrix.py: the transformation matrices.

package layout

import "github.com/xo/okage"

// Matrix is Matrix in weasyprint/matrix.py: an affine transformation matrix
// of 3 rows and 3 columns, with the last column (0, 0, 1). A point is a row
// vector (x, y, 1) on the left of the matrix: x' = a*x + c*y + e and y' =
// b*x + d*y + f.
//
// Port: upstream keeps the nine values in lists. The port keeps the six
// values that an affine matrix uses.
type Matrix struct {
	A, B, C, D, E, F float64
}

// identity is Matrix(), the identity matrix.
var identity = Matrix{A: 1, D: 1}

// translation returns Matrix(e=e, f=f).
func translation(e, f float64) Matrix {
	return Matrix{A: 1, D: 1, E: e, F: f}
}

// Mul is the operator @ of Matrix: m applies first, then other.
func (m Matrix) Mul(other Matrix) Matrix {
	return Matrix{
		A: m.A*other.A + m.B*other.C,
		B: m.A*other.B + m.B*other.D,
		C: m.C*other.A + m.D*other.C,
		D: m.C*other.B + m.D*other.D,
		E: m.E*other.A + m.F*other.C + other.E,
		F: m.E*other.B + m.F*other.D + other.F,
	}
}

// Determinant is the property determinant of Matrix.
func (m Matrix) Determinant() float64 {
	return m.A*m.D - m.B*m.C
}

// Invert is the property invert of Matrix. It returns false when the matrix
// has no inverse, where upstream divides by zero.
func (m Matrix) Invert() (Matrix, bool) {
	d := m.Determinant()
	if d == 0 {
		return Matrix{}, false
	}
	return Matrix{
		A: m.D / d,
		B: -m.B / d,
		C: -m.C / d,
		D: m.A / d,
		E: (m.C*m.F - m.D*m.E) / d,
		F: (m.B*m.E - m.A*m.F) / d,
	}, true
}

// TransformPoint is the method transform_point of Matrix.
func (m Matrix) TransformPoint(x, y float64) (float64, float64) {
	return m.A*x + m.C*y + m.E, m.B*x + m.D*y + m.F
}

// Values is the property values of Matrix: (a, b, c, d, e, f).
func (m Matrix) Values() [6]float64 {
	return [6]float64{m.A, m.B, m.C, m.D, m.E, m.F}
}

// transform returns the matrix as a transform of okage.
func (m Matrix) transform() okage.Transform {
	return okage.TransformFromRow(float32(m.A), float32(m.B), float32(m.C), float32(m.D), float32(m.E), float32(m.F))
}
