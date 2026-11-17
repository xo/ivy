// Copyright (c) The Hayro Authors.
//
// Use of this source code is governed by the Apache-2.0 or MIT license that
// can be found in the LICENSE-APACHE and LICENSE-MIT files.

// This file ports hayro-interpret src/function/type3.rs.

package interpret

import "github.com/xo/ivy/internal/hayro/syntax"

// functionType3 is Type3 in hayro-interpret src/function/type3.rs. It is a
// stitching function.
type functionType3 struct {
	functions []*Function
	bounds    []float32
	encode    []interval
}

// newFunctionType3 is Type3::new in hayro-interpret src/function/type3.rs.
func newFunctionType3(dict syntax.Dict, domain interval, depth uint8, budget *int) (*functionType3, bool) {
	arr, ok := dict.Get[syntax.Array](syntax.KeyFunctions)
	if !ok {
		return nil, false
	}
	var functions []*Function
	for o := range arr.Iter[syntax.Object]().All() {
		f, ok := newFunctionNested(o, depth+1, budget)
		if !ok {
			return nil, false
		}
		functions = append(functions, f)
	}
	if len(functions) == 0 {
		return nil, false
	}
	numOutputs := functions[0].NumOutputs()
	for _, f := range functions {
		if f.NumInputs() != 1 || f.NumOutputs() != numOutputs {
			return nil, false
		}
	}
	var innerBounds []float32
	if b, ok := dict.Get[syntax.Array](syntax.KeyBounds); ok {
		innerBounds = b.Iter[float32]().Collect()
	}
	encode, ok := readIntervals(dict, syntax.KeyEncode)
	if !ok {
		return nil, false
	}
	if len(innerBounds) != len(functions)-1 || len(encode) != len(functions) {
		return nil, false
	}
	bounds := make([]float32, 0, len(innerBounds)+2)
	bounds = append(bounds, domain.min)
	bounds = append(bounds, innerBounds...)
	bounds = append(bounds, domain.max)
	return &functionType3{functions: functions, bounds: bounds, encode: encode}, true
}

// numOutputs is Type3::num_outputs in hayro-interpret
// src/function/type3.rs.
func (t *functionType3) numOutputs() int {
	return t.functions[0].NumOutputs()
}

// eval is Type3::eval in hayro-interpret src/function/type3.rs.
func (t *functionType3) eval(input float32, output []float32) bool {
	index := t.findInterval(input)
	iv := interval{min: t.bounds[index], max: t.bounds[index+1]}
	return t.functions[index].Eval([]float32{iv.mapTo(input, t.encode[index])}, output)
}

// findInterval is Type3::find_interval in hayro-interpret
// src/function/type3.rs.
func (t *functionType3) findInterval(x float32) int {
	inner := t.bounds[1 : len(t.bounds)-1]
	if len(inner) > 0 && inner[0] == t.bounds[0] && x == t.bounds[0] {
		return 0
	}
	return partitionPoint(len(inner), func(i int) bool { return inner[i] <= x })
}

// stitchingBounds is Type3::stitching_bounds in hayro-interpret
// src/function/type3.rs. It returns the bounds of this function and of the
// functions that it stitches, in the domain of this function.
func (t *functionType3) stitchingBounds() []float32 {
	bounds := append([]float32(nil), t.bounds[1:len(t.bounds)-1]...)
	for index, function := range t.functions {
		lo, hi := t.bounds[index], t.bounds[index+1]
		enc := t.encode[index]
		d := enc.max - enc.min
		if d < 0 {
			d = -d
		}
		if d <= epsilonF32 {
			continue
		}
		for _, child := range function.stitchingBounds() {
			bound := interpolate(child, enc.min, enc.max, lo, hi)
			minBound, maxBound := rustMin(lo, hi), rustMax(lo, hi)
			if bound > minBound && bound < maxBound {
				bounds = append(bounds, bound)
			}
		}
	}
	return sortDedupBounds(bounds)
}
