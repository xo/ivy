// Copyright (c) The Hayro Authors.
//
// Use of this source code is governed by the Apache-2.0 or MIT license that
// can be found in the LICENSE-APACHE and LICENSE-MIT files.

// This file ports hayro-interpret src/function/mod.rs.

package interpret

import (
	"slices"
	"sync"

	"github.com/xo/ivy/pdf/internal/syntax"
)

// maxComponents is MAX_COMPONENTS in hayro-interpret src/function/mod.rs.
const maxComponents = 32

// maxFunctionDepth is MAX_FUNCTION_DEPTH in hayro-interpret
// src/function/mod.rs.
const maxFunctionDepth = 32

// maxFunctionNodes is the largest number of functions that one function of
// type 3 can nest.
//
// Port: upstream bounds the depth of nested functions, but not their number.
// An array of functions that names one object more than once makes the
// number of functions grow exponentially with the depth. The cap keeps the
// work bounded (D18).
const maxFunctionNodes = 1 << 16

// interval is Interval in hayro-interpret src/function/mod.rs.
type interval struct {
	min, max float32
}

// clamp is Interval::clamp in hayro-interpret src/function/mod.rs.
func (i interval) clamp(x float32) float32 {
	return rustMax(rustMin(x, i.max), i.min)
}

// mapTo is Interval::map in hayro-interpret src/function/mod.rs. It maps x
// from i to the interval to.
func (i interval) mapTo(x float32, to interval) float32 {
	if i.min == i.max {
		return to.min
	}
	return interpolate(x, i.min, i.max, to.min, to.max)
}

// readIntervals is read_intervals in hayro-interpret src/function/mod.rs.
func readIntervals(dict syntax.Dict, key syntax.Name) ([]interval, bool) {
	pairs, ok := dict.Get[[]syntax.Tuple2[float32, float32]](key)
	if !ok {
		return nil, false
	}
	out := make([]interval, len(pairs))
	for i, p := range pairs {
		out[i] = interval{min: p.V0, max: p.V1}
	}
	return out, true
}

// Function is Function in hayro-interpret src/function/mod.rs, with Repr
// of the same file. It is a function of PDF.
//
// Port: upstream shares the function through an Arc. Go shares the
// pointer. The enum FunctionType becomes one field for each type, and one
// of them is set.
type Function struct {
	domain     []interval
	rng        []interval
	hasRange   bool
	numOutputs int

	type0 *functionType0
	type2 *functionType2
	type3 *functionType3
	type4 *functionType4
}

// NewFunction is Function::new in hayro-interpret src/function/mod.rs. It
// reads a function from a dictionary or a stream.
func NewFunction(obj syntax.Object) (*Function, bool) {
	budget := maxFunctionNodes
	return newFunctionNested(obj, 0, &budget)
}

// newFunctionNested is Function::new_nested in hayro-interpret
// src/function/mod.rs. budget counts down the functions that the call can
// still read.
func newFunctionNested(obj syntax.Object, depth uint8, budget *int) (*Function, bool) {
	if depth >= maxFunctionDepth {
		return nil, false
	}
	// Port: see maxFunctionNodes.
	if *budget <= 0 {
		return nil, false
	}
	*budget--
	dict, stream, ok := syntax.DictOrStream(obj)
	if !ok {
		return nil, false
	}
	functionType, ok := dict.Get[uint8](syntax.KeyFunctionType)
	if !ok {
		return nil, false
	}
	domain, ok := readIntervals(dict, syntax.KeyDomain)
	if !ok {
		return nil, false
	}
	rng, hasRange := readIntervals(dict, syntax.KeyRange)
	if len(domain) == 0 || len(domain) > maxComponents {
		return nil, false
	}
	if (functionType == 2 || functionType == 3) && len(domain) != 1 {
		return nil, false
	}
	f := &Function{domain: domain, rng: rng, hasRange: hasRange}
	switch functionType {
	case 0:
		if !hasRange || stream == nil {
			return nil, false
		}
		t, ok := newFunctionType0(*stream, domain, rng)
		if !ok {
			return nil, false
		}
		f.type0, f.numOutputs = t, len(rng)
	case 2:
		t, ok := newFunctionType2(dict)
		if !ok {
			return nil, false
		}
		f.type2, f.numOutputs = t, t.numOutputs()
	case 3:
		t, ok := newFunctionType3(dict, domain[0], depth, budget)
		if !ok {
			return nil, false
		}
		f.type3, f.numOutputs = t, t.numOutputs()
	case 4:
		if stream == nil {
			return nil, false
		}
		t, ok := newFunctionType4(*stream)
		if !ok || !hasRange {
			return nil, false
		}
		f.type4, f.numOutputs = t, len(rng)
	default:
		return nil, false
	}
	if f.numOutputs == 0 || f.numOutputs > maxComponents {
		return nil, false
	}
	return f, true
}

// NumInputs is Function::num_inputs in hayro-interpret
// src/function/mod.rs. It returns the number of input values of each
// sample.
func (f *Function) NumInputs() int {
	return len(f.domain)
}

// NumOutputs is Function::num_outputs in hayro-interpret
// src/function/mod.rs. It returns the number of output values of each
// sample.
func (f *Function) NumOutputs() int {
	return f.numOutputs
}

// Eval is Function::eval in hayro-interpret src/function/mod.rs. It
// evaluates the function on the samples of input, one after the other, and
// writes the results to output. It fails when the lengths do not fit.
func (f *Function) Eval(input, output []float32) bool {
	numInputs, numOutputs := f.NumInputs(), f.NumOutputs()
	if len(input)%numInputs != 0 || len(input)/numInputs*numOutputs != len(output) {
		return false
	}
	switch {
	case f.type0 != nil:
		return f.evalSamples(input, output, func(in, out []float32) bool {
			f.type0.eval(in, out)
			return true
		})
	case f.type2 != nil:
		return f.evalSamples(input, output, func(in, out []float32) bool {
			f.type2.eval(in[0], out)
			return true
		})
	case f.type3 != nil:
		return f.evalSamples(input, output, func(in, out []float32) bool {
			return f.type3.eval(in[0], out)
		})
	}
	return f.evalSamples(input, output, f.type4.evaluator())
}

// evalSamples is Function::eval_samples in hayro-interpret
// src/function/mod.rs. It clamps each sample to the domain, and each
// result to the range.
func (f *Function) evalSamples(input, output []float32, eval func(in, out []float32) bool) bool {
	numInputs, numOutputs := f.NumInputs(), f.NumOutputs()
	var buf [maxComponents]float32
	clamped := buf[:numInputs]
	for i := 0; i*numInputs < len(input) && i*numOutputs < len(output); i++ {
		in := input[i*numInputs : (i+1)*numInputs]
		out := output[i*numOutputs : (i+1)*numOutputs]
		for j := range clamped {
			clamped[j] = f.domain[j].clamp(in[j])
		}
		if !eval(clamped, out) {
			return false
		}
		if f.hasRange {
			for j := 0; j < len(out) && j < len(f.rng); j++ {
				out[j] = f.rng[j].clamp(out[j])
			}
		}
	}
	return true
}

// stitchingBounds is Function::stitching_bounds in hayro-interpret
// src/function/mod.rs.
func (f *Function) stitchingBounds() []float32 {
	if f.type3 != nil {
		return f.type3.stitchingBounds()
	}
	return nil
}

// TransferFunction is TransferFunction in hayro-interpret
// src/function/mod.rs. It maps each value of a byte through a function.
//
// Port: upstream shares the table of samples through an Arc. Go shares the
// pointer.
type TransferFunction struct {
	function *Function
	once     sync.Once
	table    [256]uint8
}

// newTransferFunction is TransferFunction::new in hayro-interpret
// src/function/mod.rs.
func newTransferFunction(function *Function) *TransferFunction {
	return &TransferFunction{function: function}
}

// ApplyTo is TransferFunction::apply_to in hayro-interpret
// src/function/mod.rs. It maps each value of values.
func (t *TransferFunction) ApplyTo(values []uint8) {
	t.applyToStride(values, 1)
}

// applyToStride is TransferFunction::apply_to_stride in hayro-interpret
// src/function/mod.rs. It maps each value of values whose index is a
// multiple of stride.
func (t *TransferFunction) applyToStride(values []uint8, stride int) {
	samples := t.samples()
	for i := 0; i < len(values); i += stride {
		values[i] = samples[values[i]]
	}
}

// applyF32 is TransferFunction::apply_f32 in hayro-interpret
// src/function/mod.rs.
func (t *TransferFunction) applyF32(value float32) float32 {
	return float32(t.samples()[asU8(value*255+0.5)]) / 255
}

// samples is TransferFunction::samples in hayro-interpret
// src/function/mod.rs. It evaluates the function once for each byte.
func (t *TransferFunction) samples() *[256]uint8 {
	t.once.Do(func() {
		output := make([]float32, t.function.NumOutputs())
		for sample := range t.table {
			if t.function.Eval([]float32{float32(sample) * (1.0 / 255.0)}, output) {
				t.table[sample] = asU8(output[0]*255 + 0.5)
			} else {
				t.table[sample] = uint8(sample)
			}
		}
	})
	return &t.table
}

// interpolate is interpolate in hayro-interpret src/function/mod.rs.
func interpolate(x, xMin, xMax, yMin, yMax float32) float32 {
	return yMin + (x-xMin)*(yMax-yMin)/(xMax-xMin)
}

// sortDedupBounds sorts the bounds and removes each bound that is within
// f32::EPSILON of the last bound that it keeps, as the sort_by and dedup_by
// of upstream do.
func sortDedupBounds(bounds []float32) []float32 {
	slices.SortFunc(bounds, totalCmpF32)
	out := bounds[:0]
	for _, b := range bounds {
		if len(out) > 0 {
			d := b - out[len(out)-1]
			if d < 0 {
				d = -d
			}
			if d <= epsilonF32 {
				continue
			}
		}
		out = append(out, b)
	}
	return out
}

// epsilonF32 is f32::EPSILON of Rust.
const epsilonF32 = 1.1920929e-07
