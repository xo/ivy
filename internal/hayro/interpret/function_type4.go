// Copyright (c) The Hayro Authors.
//
// Use of this source code is governed by the Apache-2.0 or MIT license that
// can be found in the LICENSE-APACHE and LICENSE-MIT files.

// This file ports hayro-interpret src/function/type4.rs.

package interpret

import (
	"math"

	"github.com/xo/ivy/internal/hayro/postscript"
	"github.com/xo/ivy/internal/hayro/syntax"
)

// maxProcedureDepth is MAX_PROCEDURE_DEPTH in hayro-interpret
// src/function/type4.rs.
const maxProcedureDepth = 256

// maxStackDepth is MAX_STACK_DEPTH in hayro-interpret
// src/function/type4.rs.
const maxStackDepth = 100

// functionType4 is Type4 in hayro-interpret src/function/type4.rs. It is a
// PostScript calculator function.
type functionType4 struct {
	code []postScriptOp
}

// newFunctionType4 is Type4::new in hayro-interpret src/function/type4.rs.
func newFunctionType4(stream syntax.Stream) (*functionType4, bool) {
	data, err := stream.DecodedData()
	if err != nil {
		return nil, false
	}
	code, ok := parseProcedure(data)
	if !ok {
		return nil, false
	}
	return &functionType4{code: code}, true
}

// evaluator is Type4::evaluator in hayro-interpret src/function/type4.rs. It
// returns a function that runs the program on one sample, with a stack that
// the calls share.
func (t *functionType4) evaluator() func(input, output []float32) bool {
	stack := &psStack{}
	return func(input, output []float32) bool {
		n := len(input)
		if n > maxStackDepth {
			return false
		}
		copy(stack.values[:n], input)
		clear(stack.bools[:n])
		stack.len = n
		if !execute(t.code, stack) {
			return false
		}
		values := stack.values[:stack.len]
		if len(values) < len(output) {
			return false
		}
		copy(output, values[len(values)-len(output):])
		return true
	}
}

// psStack is Stack in hayro-interpret src/function/type4.rs. bools marks
// the values that are booleans.
type psStack struct {
	values [maxStackDepth]float32
	bools  [maxStackDepth]bool
	len    int
}

// b2f is f32::from(bool) of Rust.
func b2f(b bool) float32 {
	if b {
		return 1
	}
	return 0
}

// toRadians is f32::to_radians of Rust.
func toRadians(x float32) float32 {
	return x * (float32(math.Pi) / 180)
}

// toDegrees is f32::to_degrees of Rust.
func toDegrees(x float32) float32 {
	return x * (180 / float32(math.Pi))
}

// execute is execute in hayro-interpret src/function/type4.rs. It runs the
// program on the stack. It fails on an underflow or an overflow of the
// stack, and on a division of integers by zero. The program only jumps
// forward, so it ends.
func execute(code []postScriptOp, stack *psStack) bool {
	values, bools := &stack.values, &stack.bools
	n := stack.len
	push := func(v float32, b bool) bool {
		if n >= maxStackDepth {
			return false
		}
		values[n], bools[n] = v, b
		n++
		return true
	}
	pop := func() (float32, bool) {
		if n < 1 {
			return 0, false
		}
		n--
		return values[n], true
	}
	unary := func(f func(float32) float32) bool {
		if n < 1 {
			return false
		}
		i := n - 1
		values[i] = f(values[i])
		bools[i] = false
		return true
	}
	binary := func(f func(a, b float32) (float32, bool), isBool bool) bool {
		if n < 2 {
			return false
		}
		i := n - 2
		v, ok := f(values[i], values[i+1])
		if !ok {
			return false
		}
		values[i], bools[i] = v, isBool
		n--
		return true
	}
	number := func(f func(a, b float32) float32) bool {
		return binary(func(a, b float32) (float32, bool) { return f(a, b), true }, false)
	}
	compare := func(f func(a, b float32) bool) bool {
		return binary(func(a, b float32) (float32, bool) { return b2f(f(a, b)), true }, true)
	}
	logical := func(fi func(a, b int32) int32, fb func(a, b bool) bool) bool {
		if n < 2 {
			return false
		}
		i := n - 2
		anyBool := bools[i] || bools[i+1]
		a, b := values[i], values[i+1]
		if anyBool {
			values[i] = b2f(fb(a != 0, b != 0))
		} else {
			values[i] = float32(fi(asI32(a), asI32(b)))
		}
		bools[i] = anyBool
		n--
		return true
	}

	ok := true
	for pc := 0; pc < len(code) && ok; {
		op := code[pc]
		pc++
		switch op.kind {
		case psNumber:
			ok = push(op.num, false)
		case psTrue:
			ok = push(1, true)
		case psFalse:
			ok = push(0, true)
		case psAbs:
			ok = unary(func(x float32) float32 { return float32(math.Abs(float64(x))) })
		case psCeiling:
			ok = unary(func(x float32) float32 { return float32(math.Ceil(float64(x))) })
		case psCos:
			ok = unary(func(x float32) float32 { return float32(math.Cos(float64(toRadians(x)))) })
		case psCvi, psTruncate:
			ok = unary(func(x float32) float32 { return float32(math.Trunc(float64(x))) })
		case psCvr:
			ok = unary(func(x float32) float32 { return x })
		case psFloor:
			ok = unary(func(x float32) float32 { return float32(math.Floor(float64(x))) })
		case psLn:
			ok = unary(func(x float32) float32 { return float32(math.Log(float64(x))) })
		case psLog:
			ok = unary(func(x float32) float32 { return float32(math.Log10(float64(x))) })
		case psNeg:
			ok = unary(func(x float32) float32 { return -x })
		case psRound:
			ok = unary(func(x float32) float32 { return float32(math.Round(float64(x))) })
		case psSin:
			ok = unary(func(x float32) float32 { return float32(math.Sin(float64(toRadians(x)))) })
		case psSqrt:
			ok = unary(func(x float32) float32 { return float32(math.Sqrt(float64(x))) })
		case psAdd:
			ok = number(func(a, b float32) float32 { return a + b })
		case psAtan:
			ok = number(func(a, b float32) float32 {
				degrees := toDegrees(float32(math.Atan2(float64(a), float64(b))))
				if degrees < 0 {
					return degrees + 360
				}
				return degrees
			})
		case psDiv:
			ok = number(func(a, b float32) float32 { return a / b })
		case psExp:
			ok = number(powf32)
		case psIdiv:
			ok = binary(func(a, b float32) (float32, bool) {
				x, y := asI32(a), asI32(b)
				if y == 0 || (x == math.MinInt32 && y == -1) {
					return 0, false
				}
				return float32(x / y), true
			}, false)
		case psMod:
			ok = number(func(a, b float32) float32 { return float32(math.Mod(float64(a), float64(b))) })
		case psMul:
			ok = number(func(a, b float32) float32 { return a * b })
		case psSub:
			ok = number(func(a, b float32) float32 { return a - b })
		case psBitshift:
			ok = number(func(a, b float32) float32 {
				num, shift := asU32(a), asI32(b)
				if shift >= 0 {
					if shift >= 32 {
						return 0
					}
					return float32(num << uint(shift))
				}
				s := uint32(-int64(shift))
				if s >= 32 {
					return 0
				}
				return float32(num >> s)
			})
		case psEq:
			ok = compare(func(a, b float32) bool { return a == b })
		case psGe:
			ok = compare(func(a, b float32) bool { return a >= b })
		case psGt:
			ok = compare(func(a, b float32) bool { return a > b })
		case psLe:
			ok = compare(func(a, b float32) bool { return a <= b })
		case psLt:
			ok = compare(func(a, b float32) bool { return a < b })
		case psNe:
			ok = compare(func(a, b float32) bool { return a != b })
		case psAnd:
			ok = logical(func(a, b int32) int32 { return a & b }, func(a, b bool) bool { return a && b })
		case psOr:
			ok = logical(func(a, b int32) int32 { return a | b }, func(a, b bool) bool { return a || b })
		case psXor:
			ok = logical(func(a, b int32) int32 { return a ^ b }, func(a, b bool) bool { return a != b })
		case psNot:
			if n < 1 {
				return false
			}
			i := n - 1
			if bools[i] {
				values[i] = b2f(values[i] == 0)
			} else {
				values[i] = float32(^asI32(values[i]))
			}
		case psCopy:
			var v float32
			if v, ok = pop(); ok {
				count := int(asU32(v))
				if count > n {
					return false
				}
				start := n - count
				for i := start; i < start+count && ok; i++ {
					ok = push(values[i], bools[i])
				}
			}
		case psDup:
			if n < 1 {
				return false
			}
			ok = push(values[n-1], bools[n-1])
		case psExch:
			if n < 2 {
				return false
			}
			i := n - 2
			values[i], values[i+1] = values[i+1], values[i]
			bools[i], bools[i+1] = bools[i+1], bools[i]
		case psIndex:
			var v float32
			if v, ok = pop(); ok {
				idx := int(asU32(v))
				if idx+1 > n {
					return false
				}
				i := n - (idx + 1)
				ok = push(values[i], bools[i])
			}
		case psPop:
			_, ok = pop()
		case psRoll:
			var jv, nv float32
			if jv, ok = pop(); !ok {
				return false
			}
			if nv, ok = pop(); !ok {
				return false
			}
			j := asI32(jv)
			count := int(asU32(nv))
			if count > n {
				return false
			}
			start := n - count
			if count > 0 {
				aj := uint64(j)
				if j < 0 {
					aj = uint64(-int64(j))
				}
				shift := int(aj % uint64(count))
				vs, bs := values[start:n], bools[start:n]
				if j >= 0 {
					rotateRight(vs, shift)
					rotateRight(bs, shift)
				} else {
					rotateLeft(vs, shift)
					rotateLeft(bs, shift)
				}
			}
		case psSkipUnless:
			var v float32
			if v, ok = pop(); ok && v == 0 {
				pc += int(op.skip)
			}
		case psSkip:
			pc += int(op.skip)
		}
	}
	if !ok {
		return false
	}
	stack.len = n
	return true
}

// rotateLeft is slice::rotate_left of Rust.
func rotateLeft[T any](s []T, k int) {
	if len(s) == 0 {
		return
	}
	k %= len(s)
	reverse(s[:k])
	reverse(s[k:])
	reverse(s)
}

// rotateRight is slice::rotate_right of Rust.
func rotateRight[T any](s []T, k int) {
	if len(s) == 0 {
		return
	}
	rotateLeft(s, len(s)-k%len(s))
}

// reverse reverses s in place.
func reverse[T any](s []T) {
	for i, j := 0, len(s)-1; i < j; i, j = i+1, j-1 {
		s[i], s[j] = s[j], s[i]
	}
}

// parseProcedure is parse_procedure in hayro-interpret
// src/function/type4.rs.
func parseProcedure(data []byte) ([]postScriptOp, bool) {
	procedure, err := postscript.NewScanner(data).ParseProcedure()
	if err != nil {
		return nil, false
	}
	return parseProcedureInner(procedure, 0)
}

// parseProcedureInner is parse_procedure_inner in hayro-interpret
// src/function/type4.rs. It compiles a procedure into a flat list of
// operators, with the procedures of if and ifelse inline.
func parseProcedureInner(procedure postscript.Procedure, depth int) ([]postScriptOp, bool) {
	if depth >= maxProcedureDepth {
		return nil, false
	}
	var procedures [][]postScriptOp
	var code []postScriptOp
	tokens := procedure.Tokens()
	pop := func() ([]postScriptOp, bool) {
		if len(procedures) == 0 {
			return nil, false
		}
		p := procedures[len(procedures)-1]
		procedures = procedures[:len(procedures)-1]
		return p, true
	}
	for !tokens.AtEnd() {
		token, err := tokens.ParseToken()
		if err != nil {
			return nil, false
		}
		switch t := token.(type) {
		case postscript.Number:
			code = append(code, postScriptOp{kind: psNumber, num: float32(t.Float64())})
		case postscript.Procedure:
			if len(procedures) >= 2 {
				return nil, false
			}
			inner, ok := parseProcedureInner(t, depth+1)
			if !ok {
				return nil, false
			}
			procedures = append(procedures, inner)
		case postscript.Name:
			if t.IsLiteral() || t.IsImmediate() {
				return nil, false
			}
			switch name := string(t.AsBytes()); name {
			case "if":
				then, ok := pop()
				if !ok {
					return nil, false
				}
				code = append(code, postScriptOp{kind: psSkipUnless, skip: uint32(len(then))})
				code = append(code, then...)
			case "ifelse":
				otherwise, ok := pop()
				if !ok {
					return nil, false
				}
				then, ok := pop()
				if !ok {
					return nil, false
				}
				code = append(code, postScriptOp{kind: psSkipUnless, skip: uint32(len(then)) + 1})
				code = append(code, then...)
				code = append(code, postScriptOp{kind: psSkip, skip: uint32(len(otherwise))})
				code = append(code, otherwise...)
			default:
				op, ok := postScriptOpFromOperator(name)
				if !ok {
					return nil, false
				}
				code = append(code, op)
			}
		default:
			return nil, false
		}
	}
	return code, true
}

// psOpKind is the case of PostScriptOp in hayro-interpret
// src/function/type4.rs.
type psOpKind uint8

// The cases of PostScriptOp.
const (
	psNumber psOpKind = iota
	psAbs
	psAdd
	psAtan
	psCeiling
	psCos
	psCvi
	psCvr
	psDiv
	psExp
	psFloor
	psIdiv
	psLn
	psLog
	psMod
	psMul
	psNeg
	psRound
	psSin
	psSqrt
	psSub
	psTruncate
	psAnd
	psBitshift
	psEq
	psFalse
	psGe
	psGt
	psLe
	psLt
	psNe
	psNot
	psOr
	psTrue
	psXor
	psCopy
	psDup
	psExch
	psIndex
	psPop
	psRoll
	psSkipUnless
	psSkip
)

// postScriptOp is PostScriptOp in hayro-interpret src/function/type4.rs.
//
// Port: upstream is an enum. num holds the value of Number, and skip the
// value of SkipUnless and Skip.
type postScriptOp struct {
	kind psOpKind
	num  float32
	skip uint32
}

// psOperators maps the names of the operators to their cases.
var psOperators = map[string]psOpKind{
	"abs": psAbs, "add": psAdd, "atan": psAtan, "ceiling": psCeiling, "cos": psCos,
	"cvi": psCvi, "cvr": psCvr, "div": psDiv, "exp": psExp, "floor": psFloor,
	"idiv": psIdiv, "ln": psLn, "log": psLog, "mod": psMod, "mul": psMul,
	"neg": psNeg, "round": psRound, "sin": psSin, "sqrt": psSqrt, "sub": psSub,
	"truncate": psTruncate, "and": psAnd, "bitshift": psBitshift, "eq": psEq,
	"false": psFalse, "ge": psGe, "gt": psGt, "le": psLe, "lt": psLt, "ne": psNe,
	"not": psNot, "or": psOr, "true": psTrue, "xor": psXor, "copy": psCopy,
	"dup": psDup, "exch": psExch, "index": psIndex, "pop": psPop, "roll": psRoll,
}

// postScriptOpFromOperator is PostScriptOp::from_operator in
// hayro-interpret src/function/type4.rs.
func postScriptOpFromOperator(name string) (postScriptOp, bool) {
	kind, ok := psOperators[name]
	return postScriptOp{kind: kind}, ok
}
