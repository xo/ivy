package vsdx

import (
	"math"

	"github.com/xo/okage"
)

// affine is an affine transform: x' = a*x + c*y + e, y' = b*x + d*y + f.
type affine struct {
	a, b, c, d, e, f float64
}

// translate returns the transform that moves by (x, y).
func translate(x, y float64) affine {
	return affine{a: 1, d: 1, e: x, f: y}
}

// rotate returns the transform that turns by the angle, in radians,
// counterclockwise when the y axis points up.
func rotate(angle float64) affine {
	s, c := math.Sincos(angle)
	return affine{a: c, b: s, c: -s, d: c}
}

// scale returns the transform that scales by sx and sy.
func scale(sx, sy float64) affine {
	return affine{a: sx, d: sy}
}

// mul returns the transform that applies n, then m.
func (m affine) mul(n affine) affine {
	return affine{
		a: m.a*n.a + m.c*n.b,
		b: m.b*n.a + m.d*n.b,
		c: m.a*n.c + m.c*n.d,
		d: m.b*n.c + m.d*n.d,
		e: m.a*n.e + m.c*n.f + m.e,
		f: m.b*n.e + m.d*n.f + m.f,
	}
}

// apply returns the point p transformed.
func (m affine) apply(p pt) pt {
	return pt{m.a*p.x + m.c*p.y + m.e, m.b*p.x + m.d*p.y + m.f}
}

// apply32 returns the point p transformed, for okage.
func (m affine) apply32(p pt) (float32, float32) {
	q := m.apply(p)
	return float32(q.x), float32(q.y)
}

// det returns the determinant of the transform.
func (m affine) det() float64 {
	return m.a*m.d - m.b*m.c
}

// finite reports whether each number of the transform is finite.
func (m affine) finite() bool {
	for _, v := range [...]float64{m.a, m.b, m.c, m.d, m.e, m.f} {
		if math.IsNaN(v) || math.IsInf(v, 0) || math.Abs(v) > 1e9 {
			return false
		}
	}
	return true
}

// okage returns the transform for okage.
func (m affine) okage() okage.Transform {
	return okage.TransformFromRow(float32(m.a), float32(m.b), float32(m.c), float32(m.d), float32(m.e), float32(m.f))
}
