package graphics

import (
	"math"

	"github.com/xo/ivy/ps/internal/interp"
)

// Matrix is a matrix of PostScript, [a b c d tx ty] (PLRM 4.3.3). A point
// (x, y) becomes (a*x + c*y + tx, b*x + d*y + ty).
type Matrix [6]float64

// identity is the identity matrix.
var identity = Matrix{1, 0, 0, 1, 0, 0}

// Mul returns the matrix that applies m, then n, as concat makes the CTM.
func (m Matrix) Mul(n Matrix) Matrix {
	return Matrix{
		m[0]*n[0] + m[1]*n[2],
		m[0]*n[1] + m[1]*n[3],
		m[2]*n[0] + m[3]*n[2],
		m[2]*n[1] + m[3]*n[3],
		m[4]*n[0] + m[5]*n[2] + n[4],
		m[4]*n[1] + m[5]*n[3] + n[5],
	}
}

// Apply transforms a point.
func (m Matrix) Apply(x, y float64) (float64, float64) {
	return m[0]*x + m[2]*y + m[4], m[1]*x + m[3]*y + m[5]
}

// ApplyDist transforms a distance, without the translation.
func (m Matrix) ApplyDist(x, y float64) (float64, float64) {
	return m[0]*x + m[2]*y, m[1]*x + m[3]*y
}

// Det returns the determinant.
func (m Matrix) Det() float64 {
	return m[0]*m[3] - m[1]*m[2]
}

// Invert returns the inverse, and false for a matrix without one.
func (m Matrix) Invert() (Matrix, bool) {
	det := m.Det()
	if det == 0 || math.IsNaN(det) || math.IsInf(det, 0) {
		return Matrix{}, false
	}
	a, b, c, d := m[3]/det, -m[1]/det, -m[2]/det, m[0]/det
	return Matrix{a, b, c, d, -(m[4]*a + m[5]*c), -(m[4]*b + m[5]*d)}, true
}

// Finite reports whether the entries are finite.
func (m Matrix) Finite() bool {
	for _, v := range m {
		if math.IsNaN(v) || math.IsInf(v, 0) {
			return false
		}
	}
	return true
}

// Scale returns the mean scale of the matrix, the square root of the
// determinant.
func (m Matrix) Scale() float64 {
	return math.Sqrt(math.Abs(m.Det()))
}

// matrixOf returns the matrix of an array of 6 numbers.
func matrixOf(o interp.Object) (Matrix, error) {
	if !o.IsArray() {
		return Matrix{}, interp.ErrTypecheck
	}
	a := o.Array()
	if len(a) != 6 {
		return Matrix{}, interp.ErrRangecheck
	}
	var m Matrix
	for i, e := range a {
		if !e.IsNumber() {
			return Matrix{}, interp.ErrTypecheck
		}
		m[i] = e.Float()
	}
	return m, nil
}

// popMatrix pops an array of 6 numbers.
func (m *Machine) popMatrix() (Matrix, interp.Object, error) {
	o, err := m.in.Peek(0)
	if err != nil {
		return Matrix{}, interp.Null, err
	}
	mat, err := matrixOf(o)
	if err != nil {
		return Matrix{}, interp.Null, err
	}
	_, _ = m.in.Pop()
	return mat, o, nil
}

// storeMatrix stores a matrix into an array of 6 elements.
func (m *Machine) storeMatrix(o interp.Object, mat Matrix) error {
	if o.Type() != interp.TArray || o.Len() != 6 {
		return interp.ErrRangecheck
	}
	if o.Access() != interp.Unlimited {
		return interp.ErrInvalidAccess
	}
	m.in.Touch(o)
	a := o.Array()
	for i := range a {
		a[i] = interp.Real(mat[i])
	}
	return nil
}

// newMatrix returns a new array that holds a matrix.
func (m *Machine) newMatrix(mat Matrix) (interp.Object, error) {
	a := make([]interp.Object, 6)
	for i := range a {
		a[i] = interp.Real(mat[i])
	}
	return m.in.NewArray(a)
}

// opsMatrix are the operators of matrices and of the CTM (PLRM 8.1).
func (m *Machine) opsMatrix() {
	in := m.in
	in.Define("matrix", func(*interp.Interp) error {
		o, err := m.newMatrix(identity)
		if err != nil {
			return err
		}
		in.Push(o)
		return nil
	})
	in.Define("initmatrix", func(*interp.Interp) error {
		m.gs.ctm = m.defaultMatrix()
		return nil
	})
	in.Define("identmatrix", func(*interp.Interp) error {
		o, err := in.Peek(0)
		if err != nil {
			return err
		}
		return m.storeMatrix(o, identity)
	})
	in.Define("defaultmatrix", func(*interp.Interp) error {
		o, err := in.Peek(0)
		if err != nil {
			return err
		}
		return m.storeMatrix(o, m.defaultMatrix())
	})
	in.Define("currentmatrix", func(*interp.Interp) error {
		o, err := in.Peek(0)
		if err != nil {
			return err
		}
		return m.storeMatrix(o, m.gs.ctm)
	})
	in.Define("setmatrix", func(*interp.Interp) error {
		mat, _, err := m.popMatrix()
		if err != nil {
			return err
		}
		m.gs.ctm = mat
		return nil
	})
	in.Define("translate", func(*interp.Interp) error {
		return m.matrixOp(func(v []float64) Matrix { return Matrix{1, 0, 0, 1, v[0], v[1]} }, 2)
	})
	in.Define("scale", func(*interp.Interp) error {
		return m.matrixOp(func(v []float64) Matrix { return Matrix{v[0], 0, 0, v[1], 0, 0} }, 2)
	})
	in.Define("rotate", func(*interp.Interp) error {
		return m.matrixOp(func(v []float64) Matrix {
			s, c := sinCos(v[0])
			return Matrix{c, s, -s, c, 0, 0}
		}, 1)
	})
	in.Define("concat", func(*interp.Interp) error {
		mat, _, err := m.popMatrix()
		if err != nil {
			return err
		}
		m.gs.ctm = mat.Mul(m.gs.ctm)
		return nil
	})
	in.Define("concatmatrix", func(*interp.Interp) error {
		dst, err := in.Peek(0)
		if err != nil {
			return err
		}
		a1, err := in.Peek(2)
		if err != nil {
			return err
		}
		a2, err := in.Peek(1)
		if err != nil {
			return err
		}
		m1, err := matrixOf(a1)
		if err != nil {
			return err
		}
		m2, err := matrixOf(a2)
		if err != nil {
			return err
		}
		if err := m.storeMatrix(dst, m1.Mul(m2)); err != nil {
			return err
		}
		for range 3 {
			_, _ = in.Pop()
		}
		in.Push(dst)
		return nil
	})
	in.Define("invertmatrix", func(*interp.Interp) error {
		dst, err := in.Peek(0)
		if err != nil {
			return err
		}
		src, err := in.Peek(1)
		if err != nil {
			return err
		}
		mat, err := matrixOf(src)
		if err != nil {
			return err
		}
		inv, ok := mat.Invert()
		if !ok {
			return interp.ErrUndefinedResult
		}
		if err := m.storeMatrix(dst, inv); err != nil {
			return err
		}
		dst, _ = in.Pop()
		_, _ = in.Pop()
		in.Push(dst)
		return nil
	})
	point := func(name string, dist, inverse bool) {
		in.Define(name, func(*interp.Interp) error {
			mat := m.gs.ctm
			top, err := in.Peek(0)
			if err != nil {
				return err
			}
			hasMatrix := top.IsArray()
			if hasMatrix {
				if mat, err = matrixOf(top); err != nil {
					return err
				}
			}
			if inverse {
				var ok bool
				if mat, ok = mat.Invert(); !ok {
					return interp.ErrUndefinedResult
				}
			}
			skip := 0
			if hasMatrix {
				skip = 1
			}
			x, err := in.Peek(skip + 1)
			if err != nil {
				return err
			}
			y, err := in.Peek(skip)
			if err != nil {
				return err
			}
			if !x.IsNumber() || !y.IsNumber() {
				return interp.ErrTypecheck
			}
			var rx, ry float64
			if dist {
				rx, ry = mat.ApplyDist(x.Float(), y.Float())
			} else {
				rx, ry = mat.Apply(x.Float(), y.Float())
			}
			for range skip + 2 {
				_, _ = in.Pop()
			}
			in.PushNum(rx)
			in.PushNum(ry)
			return nil
		})
	}
	point("transform", false, false)
	point("dtransform", true, false)
	point("itransform", false, true)
	point("idtransform", true, true)
}

// matrixOp pops n numbers, or n numbers and a matrix, and either
// concatenates the CTM with the matrix that fn makes, or stores that matrix
// into the matrix operand.
func (m *Machine) matrixOp(fn func([]float64) Matrix, n int) error {
	top, err := m.in.Peek(0)
	if err != nil {
		return err
	}
	if top.IsArray() {
		if top.Len() != 6 {
			return interp.ErrRangecheck
		}
		_, _ = m.in.Pop()
		v, err := m.in.PopNums(n)
		if err != nil {
			return err
		}
		if err := m.storeMatrix(top, fn(v)); err != nil {
			return err
		}
		m.in.Push(top)
		return nil
	}
	v, err := m.in.PopNums(n)
	if err != nil {
		return err
	}
	m.gs.ctm = fn(v).Mul(m.gs.ctm)
	return nil
}

// sinCos returns the sine and the cosine of an angle in degrees, exact at
// the multiples of 90.
func sinCos(deg float64) (float64, float64) {
	a := math.Mod(deg, 360)
	if a < 0 {
		a += 360
	}
	switch a {
	case 0:
		return 0, 1
	case 90:
		return 1, 0
	case 180:
		return 0, -1
	case 270:
		return -1, 0
	}
	return math.Sincos(a * math.Pi / 180)
}
