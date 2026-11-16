// Copyright (c) The Hayro Authors.
//
// Use of this source code is governed by the Apache-2.0 or MIT license that
// can be found in the LICENSE-APACHE and LICENSE-MIT files.

// This file ports the tests of hayro-interpret src/function/mod.rs.

package interpret

import (
	"fmt"
	"slices"
	"testing"

	"github.com/xo/ivy/pdf/internal/syntax"
)

// evalOne is Function::eval_one in hayro-interpret src/function/mod.rs, a
// helper of the tests.
func (f *Function) evalOne(input []float32) ([]float32, bool) {
	output := make([]float32, f.NumOutputs())
	if !f.Eval(input, output) {
		return nil, false
	}
	return output, true
}

// stitchingChain is stitching_chain in the tests of hayro-interpret
// src/function/mod.rs.
func stitchingChain(t testing.TB, n int) *syntax.Pdf {
	bodies := make([]string, n)
	for i := range n {
		if i+1 == n {
			bodies[i] = "<< /FunctionType 2 /Domain [0 1] /C0 [0] /C1 [1] /N 1 >>"
		} else {
			bodies[i] = fmt.Sprintf("<< /FunctionType 3 /Domain [0 1] /Functions [%d 0 R] /Bounds [] /Encode [0 1] >>", 10+i+1)
		}
	}
	return minimalPdf(t, bodies)
}

// functionOfChain is function_of_chain in the tests of hayro-interpret
// src/function/mod.rs.
func functionOfChain(t *testing.T, n int) (*Function, bool) {
	pdf := stitchingChain(t, n)
	obj, ok := pdf.XRef().Get[syntax.Object](syntax.NewObjectIdentifier(10, 0))
	if !ok {
		t.Fatal("no object 10")
	}
	return NewFunction(obj)
}

func TestNestedFunctionsUpToBound(t *testing.T) {
	f, ok := functionOfChain(t, maxFunctionDepth)
	if !ok {
		t.Fatal("chain at the bound fails")
	}
	if got, _ := f.evalOne([]float32{0.25}); !slices.Equal(got, []float32{0.25}) {
		t.Errorf("got %v, want [0.25]", got)
	}
	if _, ok := functionOfChain(t, maxFunctionDepth+1); ok {
		t.Error("chain past the bound succeeds")
	}
}

func TestDeepFunctionChainFails(t *testing.T) {
	if _, ok := functionOfChain(t, 20_000); ok {
		t.Error("deep chain succeeds")
	}
}
