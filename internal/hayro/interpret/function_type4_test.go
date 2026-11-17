// Copyright (c) The Hayro Authors.
//
// Use of this source code is governed by the Apache-2.0 or MIT license that
// can be found in the LICENSE-APACHE and LICENSE-MIT files.

// This file ports the tests of hayro-interpret src/function/type4.rs.

package interpret

import (
	"math"
	"slices"
	"strings"
	"testing"
)

// nestedProgram is nested_program in the tests of hayro-interpret
// src/function/type4.rs.
func nestedProgram(depth int) []byte {
	program := strings.Repeat("{ true ", depth-1) + "{ 1 }" + strings.Repeat(" if }", depth-1)
	return []byte(program)
}

func TestNestedProceduresUpToBound(t *testing.T) {
	if _, ok := parseProcedure(nestedProgram(maxProcedureDepth)); !ok {
		t.Error("program at the bound fails")
	}
	if _, ok := parseProcedure(nestedProgram(maxProcedureDepth + 1)); ok {
		t.Error("program past the bound succeeds")
	}
}

func TestDeepProcedureNestingFails(t *testing.T) {
	program := strings.Repeat("{", 300_000) + strings.Repeat("}", 300_000)
	if _, ok := parseProcedure([]byte(program)); ok {
		t.Error("deep program succeeds")
	}
}

func TestLex1(t *testing.T) {
	got, ok := parseProcedure([]byte("{ copy dup 2.0 exch roll }"))
	want := []postScriptOp{{kind: psCopy}, {kind: psDup}, {kind: psNumber, num: 2}, {kind: psExch}, {kind: psRoll}}
	if !ok || !slices.Equal(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestLex3(t *testing.T) {
	got, ok := parseProcedure([]byte(" {  {dup exch} if {0} {1} ifelse }"))
	want := []postScriptOp{
		{kind: psSkipUnless, skip: 2},
		{kind: psDup},
		{kind: psExch},
		{kind: psSkipUnless, skip: 2},
		{kind: psNumber, num: 0},
		{kind: psSkip, skip: 1},
		{kind: psNumber, num: 1},
	}
	if !ok || !slices.Equal(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

// run is run in the tests of hayro-interpret src/function/type4.rs.
func runCode(code []postScriptOp) ([]float32, bool) {
	stack := &psStack{}
	if !execute(code, stack) {
		return nil, false
	}
	return slices.Clone(stack.values[:stack.len]), true
}

// opImpl is op_impl in the tests of hayro-interpret src/function/type4.rs.
func opImpl(t *testing.T, prog string, out []float32) {
	t.Helper()
	code, ok := parseProcedure([]byte("{" + prog + "}"))
	if !ok {
		t.Fatalf("cannot parse %q", prog)
	}
	got, ok := runCode(code)
	if !ok {
		t.Fatalf("%q fails", prog)
	}
	if !slices.Equal(got, out) {
		t.Errorf("%q = %v, want %v", prog, got, out)
	}
}

func TestOpIdivByZero(t *testing.T) {
	code, ok := parseProcedure([]byte("{ 1 0 idiv }"))
	if !ok {
		t.Fatal("cannot parse")
	}
	if _, ok := runCode(code); ok {
		t.Error("division by zero succeeds")
	}
}

func TestDomain(t *testing.T) {
	code, ok := parseProcedure([]byte("{  }"))
	if !ok {
		t.Fatal("cannot parse")
	}
	f := &Function{
		domain:     []interval{{-5, 5}, {-5, 5}, {-5, 5}},
		numOutputs: 3,
		type4:      &functionType4{code: code},
	}
	got, ok := f.evalOne([]float32{-10, -2, 6})
	if !ok || !slices.Equal(got, []float32{-5, -2, 5}) {
		t.Errorf("got %v, want [-5 -2 5]", got)
	}
}

func TestRadixNumbers(t *testing.T) {
	opImpl(t, "16#FF 2#101", []float32{255.0, 5.0})
}

func TestOpAbs(t *testing.T) {
	opImpl(t, "4.5 abs", []float32{4.5})
	opImpl(t, "-3 abs", []float32{3.0})
	opImpl(t, "0 abs", []float32{0.0})
}

func TestOpAtan(t *testing.T) {
	opImpl(t, "0 1 atan", []float32{0.0})
	opImpl(t, "1 0 atan", []float32{90.0})
	opImpl(t, "-100 0 atan", []float32{270.0})
	opImpl(t, "4 4 atan", []float32{45.0})
}

func TestOpCeiling(t *testing.T) {
	opImpl(t, "3.2 ceiling", []float32{4.0})
	opImpl(t, "-4.8 ceiling", []float32{-4.0})
	opImpl(t, "99 ceiling", []float32{99.0})
}

func TestOpCos(t *testing.T) {
	opImpl(t, "0 cos", []float32{1.0})
	opImpl(t, "90 cos", []float32{-4.371139e-8})
}

func TestOpCvi(t *testing.T) {
	opImpl(t, "-47.8 cvi", []float32{-47.0})
	opImpl(t, "520.9 cvi", []float32{520.0})
}

func TestOpCvr(t *testing.T) {
	opImpl(t, "-47.8 cvr", []float32{-47.8})
	opImpl(t, "520 cvr", []float32{520.0})
}

func TestOpDiv(t *testing.T) {
	opImpl(t, "3 2 div", []float32{1.5})
	opImpl(t, "4 2 div", []float32{2.0})
}

func TestOpExp(t *testing.T) {
	opImpl(t, "9 0.5 exp", []float32{3.0})
	opImpl(t, "-9 -1 exp", []float32{-0.11111111})
}

func TestOpFloor(t *testing.T) {
	opImpl(t, "3.2 floor", []float32{3.0})
	opImpl(t, "-4.8 floor", []float32{-5.0})
	opImpl(t, "99 floor", []float32{99.0})
}

func TestOpIdiv(t *testing.T) {
	opImpl(t, "3 2 idiv", []float32{1.0})
	opImpl(t, "4 2 idiv", []float32{2.0})
	opImpl(t, "-5 2 idiv", []float32{-2.0})
}

func TestOpLn(t *testing.T) {
	opImpl(t, "10 ln", []float32{float32(math.Ln10)})
	opImpl(t, "100 ln", []float32{4.6051702})
}

func TestOpLog(t *testing.T) {
	opImpl(t, "10 log", []float32{1.0})
	opImpl(t, "100 log", []float32{2.0})
}

func TestOpMod(t *testing.T) {
	opImpl(t, "5 3 mod", []float32{2.0})
	opImpl(t, "5 2 mod", []float32{1.0})
	opImpl(t, "-5 3 mod", []float32{-2.0})
}

func TestOpMul(t *testing.T) {
	opImpl(t, "5 3 mul", []float32{15.0})
	opImpl(t, "-2 6 mul", []float32{-12.0})
}

func TestOpNeg(t *testing.T) {
	opImpl(t, "4.5 neg", []float32{-4.5})
	opImpl(t, "-3 neg", []float32{3.0})
}

func TestOpRound(t *testing.T) {
	opImpl(t, "3.2 round", []float32{3.0})
	opImpl(t, "6.5 round", []float32{7.0})
	opImpl(t, "-4.8 round", []float32{-5.0})
	opImpl(t, "99 round", []float32{99.0})
}

func TestOpSin(t *testing.T) {
	opImpl(t, "0.0 sin", []float32{0.0})
	opImpl(t, "90.0 sin", []float32{1.0})
}

func TestOpSqrt(t *testing.T) {
	opImpl(t, "100 sqrt", []float32{10.0})
}

func TestOpSub(t *testing.T) {
	opImpl(t, "3 4 sub", []float32{-1.0})
	opImpl(t, "6 0 sub", []float32{6.0})
}

func TestOpTruncate(t *testing.T) {
	opImpl(t, "3.2 truncate", []float32{3.0})
	opImpl(t, "-4.8 truncate", []float32{-4.0})
	opImpl(t, "99 truncate", []float32{99.0})
}

func TestOpAnd(t *testing.T) {
	opImpl(t, "true true and", []float32{1.0})
	opImpl(t, "true false and", []float32{0.0})
	opImpl(t, "false true and", []float32{0.0})
	opImpl(t, "false false and", []float32{0.0})
	opImpl(t, "99 1 and", []float32{1.0})
	opImpl(t, "52 7 and", []float32{4.0})
}

func TestOpBitshift(t *testing.T) {
	opImpl(t, "7 3 bitshift", []float32{56.0})
	opImpl(t, "142 -3 bitshift", []float32{17.0})
}

func TestOpEq(t *testing.T) {
	opImpl(t, "4.0 4 eq", []float32{1.0})
	opImpl(t, "-2.0 -3 eq", []float32{0.0})
}

func TestOpFalse(t *testing.T) {
	opImpl(t, "false", []float32{0.0})
}

func TestOpGe(t *testing.T) {
	opImpl(t, "4.2 4 ge", []float32{1.0})
	opImpl(t, "4.2 4.2 ge", []float32{1.0})
	opImpl(t, "4.2 6 ge", []float32{0.0})
}

func TestOpGt(t *testing.T) {
	opImpl(t, "4.2 4 gt", []float32{1.0})
	opImpl(t, "4.2 4.2 gt", []float32{0.0})
	opImpl(t, "4.2 6 gt", []float32{0.0})
}

func TestOpLe(t *testing.T) {
	opImpl(t, "4.2 4 le", []float32{0.0})
	opImpl(t, "4.2 4.2 le", []float32{1.0})
	opImpl(t, "4.2 6 le", []float32{1.0})
}

func TestOpLt(t *testing.T) {
	opImpl(t, "4.2 4 lt", []float32{0.0})
	opImpl(t, "4.2 4.2 lt", []float32{0.0})
	opImpl(t, "4.2 6 lt", []float32{1.0})
}

func TestOpNe(t *testing.T) {
	opImpl(t, "3.0 3 ne", []float32{0.0})
	opImpl(t, "3.0 3.0 ne", []float32{0.0})
	opImpl(t, "3.0 3.1 ne", []float32{1.0})
}

func TestOpNot(t *testing.T) {
	opImpl(t, "true not", []float32{0.0})
	opImpl(t, "false not", []float32{1.0})
	opImpl(t, "52 not", []float32{-53.0})
}

func TestComparisonsAreBooleans(t *testing.T) {
	opImpl(t, "1 2 lt not", []float32{0.0})
	opImpl(t, "2 1 lt not", []float32{1.0})
	opImpl(t, "1 2 lt 3 4 lt and not", []float32{0.0})
}

func TestOpBitshiftOutOfRange(t *testing.T) {
	opImpl(t, "1 40 bitshift", []float32{0.0})
	opImpl(t, "1 -40 bitshift", []float32{0.0})
}

func TestOpOr(t *testing.T) {
	opImpl(t, "true true or", []float32{1.0})
	opImpl(t, "true false or", []float32{1.0})
	opImpl(t, "false true or", []float32{1.0})
	opImpl(t, "false false or", []float32{0.0})
	opImpl(t, "17 5 or", []float32{21.0})
}

func TestOpXor(t *testing.T) {
	opImpl(t, "true true xor", []float32{0.0})
	opImpl(t, "true false xor", []float32{1.0})
	opImpl(t, "false true xor", []float32{1.0})
	opImpl(t, "false false xor", []float32{0.0})
	opImpl(t, "7 3 xor", []float32{4.0})
	opImpl(t, "12 3 xor", []float32{15.0})
}

func TestOpIf(t *testing.T) {
	opImpl(t, "true { 1.0 } if", []float32{1.0})
	opImpl(t, "false { 1.0 } if", []float32{})
}

func TestOpIfelse(t *testing.T) {
	opImpl(t, "true { 1.0 } { 2.0 } ifelse", []float32{1.0})
	opImpl(t, "false { 1.0 } { 2.0 } ifelse", []float32{2.0})
}

func TestOpCopy(t *testing.T) {
	opImpl(t, "1 2 3 2 copy", []float32{1.0, 2.0, 3.0, 2.0, 3.0})
	opImpl(t, "1 2 3 0 copy", []float32{1.0, 2.0, 3.0})
}

func TestOpDup(t *testing.T) {
	opImpl(t, "1 2 3 dup", []float32{1.0, 2.0, 3.0, 3.0})
}

func TestOpExch(t *testing.T) {
	opImpl(t, "1 2 3 exch", []float32{1.0, 3.0, 2.0})
}

func TestOpIndex(t *testing.T) {
	opImpl(t, "1 2 3 4 0 index", []float32{1.0, 2.0, 3.0, 4.0, 4.0})
	opImpl(t, "1 2 3 4 3 index", []float32{1.0, 2.0, 3.0, 4.0, 1.0})
}

func TestOpPop(t *testing.T) {
	opImpl(t, "1 2 3 pop", []float32{1.0, 2.0})
}

func TestOpRoll(t *testing.T) {
	opImpl(t, "1 2 3 3 -1 roll", []float32{2.0, 3.0, 1.0})
	opImpl(t, "1 2 3 3 1 roll", []float32{3.0, 1.0, 2.0})
	opImpl(t, "1 2 3 3 0 roll", []float32{1.0, 2.0, 3.0})
	opImpl(t, "1 2 3 3 5 roll", []float32{2.0, 3.0, 1.0})
	opImpl(t, "0 2 roll", []float32{})
	opImpl(t, "1 2 3 4 5 6 7 5 2 roll", []float32{1.0, 2.0, 6.0, 7.0, 3.0, 4.0, 5.0})
}
