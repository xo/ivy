// Copyright (c) The Hayro Authors.
//
// Use of this source code is governed by the Apache-2.0 or MIT license that
// can be found in the LICENSE-APACHE and LICENSE-MIT files.

// This file ports hayro-interpret src/function/type2.rs.

package interpret

import "github.com/xo/ivy/pdf/internal/syntax"

// functionType2 is Type2 in hayro-interpret src/function/type2.rs. It is an
// exponential interpolation function.
type functionType2 struct {
	c0    []float32
	delta []float32
	n     float32
}

// newFunctionType2 is Type2::new in hayro-interpret src/function/type2.rs.
func newFunctionType2(dict syntax.Dict) (*functionType2, bool) {
	c0, ok := dict.Get[[]float32](syntax.KeyC0)
	if !ok {
		c0 = []float32{0}
	}
	c1, ok := dict.Get[[]float32](syntax.KeyC1)
	if !ok {
		c1 = []float32{1}
	}
	if len(c0) != len(c1) {
		return nil, false
	}
	delta := make([]float32, len(c0))
	for i := range c0 {
		delta[i] = c1[i] - c0[i]
	}
	n, ok := dict.Get[syntax.Number](syntax.KeyN)
	if !ok {
		return nil, false
	}
	return &functionType2{c0: c0, delta: delta, n: float32(n.AsF64())}, true
}

// numOutputs is Type2::num_outputs in hayro-interpret
// src/function/type2.rs.
func (t *functionType2) numOutputs() int {
	return len(t.c0)
}

// eval is Type2::eval in hayro-interpret src/function/type2.rs.
func (t *functionType2) eval(input float32, output []float32) {
	factor := input
	if t.n != 1 {
		factor = powf32(input, t.n)
	}
	for i := 0; i < len(output) && i < len(t.c0); i++ {
		output[i] = t.c0[i] + factor*t.delta[i]
	}
}
