package interp

import (
	"bytes"
	"math"
)

// opsStack are the operators of the operand stack (PLRM 8.1).
func (in *Interp) opsStack() {
	in.Define("pop", func(in *Interp) error {
		_, err := in.Pop()
		return err
	})
	in.Define("exch", func(in *Interp) error {
		n := len(in.os)
		if n < 2 {
			return errUnderflow
		}
		in.os[n-1], in.os[n-2] = in.os[n-2], in.os[n-1]
		return nil
	})
	in.Define("dup", func(in *Interp) error {
		o, err := in.Peek(0)
		if err != nil {
			return err
		}
		in.Push(o)
		return nil
	})
	in.Define("copy", opCopy)
	in.Define("index", func(in *Interp) error {
		n, err := in.PopInt()
		if err != nil {
			return err
		}
		if n < 0 {
			return errRangecheck
		}
		o, err := in.Peek(n)
		if err != nil {
			return errRangecheck
		}
		in.Push(o)
		return nil
	})
	in.Define("roll", func(in *Interp) error {
		if len(in.os) < 2 {
			return errUnderflow
		}
		jo, no := in.os[len(in.os)-1], in.os[len(in.os)-2]
		if jo.typ != TInt || no.typ != TInt {
			return errTypecheck
		}
		n, j := int(no.i), int(jo.i)
		if n < 0 {
			return errRangecheck
		}
		if n > len(in.os)-2 {
			return errUnderflow
		}
		in.os = in.os[:len(in.os)-2]
		if n == 0 {
			return nil
		}
		j %= n
		if j < 0 {
			j += n
		}
		s := in.os[len(in.os)-n:]
		rotate(s, j)
		return nil
	})
	in.Define("clear", func(in *Interp) error {
		in.os = in.os[:0]
		return nil
	})
	in.Define("count", func(in *Interp) error {
		in.Push(Int(len(in.os)))
		return nil
	})
	in.Define("mark", func(in *Interp) error {
		in.Push(Mark)
		return nil
	})
	in.Define("cleartomark", func(in *Interp) error {
		i, err := in.markIndex()
		if err != nil {
			return err
		}
		in.os = in.os[:i]
		return nil
	})
	in.Define("counttomark", func(in *Interp) error {
		i, err := in.markIndex()
		if err != nil {
			return err
		}
		in.Push(Int(len(in.os) - i - 1))
		return nil
	})
}

// rotate rotates s up by j places: the top element moves down.
func rotate(s []Object, j int) {
	if j == 0 {
		return
	}
	tmp := append([]Object(nil), s[len(s)-j:]...)
	copy(s[j:], s[:len(s)-j])
	copy(s, tmp)
}

// markIndex returns the index of the topmost mark.
func (in *Interp) markIndex() (int, error) {
	for i := len(in.os) - 1; i >= 0; i-- {
		if in.os[i].typ == TMark {
			return i, nil
		}
	}
	return 0, errUnmatched
}

// opCopy is copy, of the stack or of a composite object.
func opCopy(in *Interp) error {
	top, err := in.Peek(0)
	if err != nil {
		return err
	}
	if top.typ == TInt {
		n := int(top.i)
		if n < 0 {
			return errRangecheck
		}
		if n > len(in.os)-1 {
			return errUnderflow
		}
		in.os = in.os[:len(in.os)-1]
		in.os = append(in.os, in.os[len(in.os)-n:]...)
		return nil
	}
	src, err := in.Peek(1)
	if err != nil {
		return err
	}
	switch {
	case src.IsArray() && top.typ == TArray:
		if src.n > top.n {
			return errRangecheck
		}
		in.Touch(top)
		copy(top.Array(), src.Array())
		top.n = src.n
	case src.typ == TString && top.typ == TString:
		if src.n > top.n {
			return errRangecheck
		}
		copy(top.Bytes(), src.Bytes())
		top.n = src.n
	case src.typ == TDict && top.typ == TDict:
		d := top.Dict()
		if !d.Writable() {
			return errInvalidAcc
		}
		src.Dict().Each(func(k, v Object) bool {
			d.Put(k, v)
			return true
		})
	case src.typ == TGState && top.typ == TGState:
		if in.Hooks.CopyGState != nil {
			in.Hooks.CopyGState(top.ref, src.ref)
		}
	default:
		return errTypecheck
	}
	in.os = in.os[:len(in.os)-2]
	in.Push(top)
	return nil
}

// arith runs an operator of two numbers: fi on two integers, which
// returns false when the result overflows, and fr on reals.
func (in *Interp) arith(fi func(a, b int64) (int64, bool), fr func(a, b float64) float64) error {
	if len(in.os) < 2 {
		return errUnderflow
	}
	a, b := in.os[len(in.os)-2], in.os[len(in.os)-1]
	if !a.IsNumber() || !b.IsNumber() {
		return errTypecheck
	}
	in.os = in.os[:len(in.os)-2]
	if a.typ == TInt && b.typ == TInt && fi != nil {
		if v, ok := fi(a.i, b.i); ok && v >= math.MinInt32 && v <= math.MaxInt32 {
			in.Push(Int(int(v)))
			return nil
		}
	}
	in.Push(Real(fr(a.Float(), b.Float())))
	return nil
}

// unary runs an operator of one number that keeps the type of an integer.
func (in *Interp) unary(fi func(int64) int64, fr func(float64) float64) error {
	o, err := in.Peek(0)
	if err != nil {
		return err
	}
	if !o.IsNumber() {
		return errTypecheck
	}
	in.os = in.os[:len(in.os)-1]
	if o.typ == TInt {
		v := fi(o.i)
		if v >= math.MinInt32 && v <= math.MaxInt32 {
			in.Push(Int(int(v)))
			return nil
		}
		in.Push(Real(float64(v)))
		return nil
	}
	in.Push(Real(fr(o.r)))
	return nil
}

// realOp runs an operator of one number with a real result.
func (in *Interp) realOp(fn func(float64) (float64, error)) error {
	o, err := in.Peek(0)
	if err != nil {
		return err
	}
	if !o.IsNumber() {
		return errTypecheck
	}
	v, err := fn(o.Float())
	if err != nil {
		return err
	}
	in.os = in.os[:len(in.os)-1]
	in.Push(Real(v))
	return nil
}

// opsMath are the arithmetic operators (PLRM 8.1).
func (in *Interp) opsMath() {
	in.Define("add", func(in *Interp) error {
		return in.arith(func(a, b int64) (int64, bool) { return a + b, true }, func(a, b float64) float64 { return a + b })
	})
	in.Define("sub", func(in *Interp) error {
		return in.arith(func(a, b int64) (int64, bool) { return a - b, true }, func(a, b float64) float64 { return a - b })
	})
	in.Define("mul", func(in *Interp) error {
		return in.arith(func(a, b int64) (int64, bool) { return a * b, true }, func(a, b float64) float64 { return a * b })
	})
	in.Define("div", func(in *Interp) error {
		if b, err := in.Peek(0); err == nil && b.IsNumber() && b.Float() == 0 {
			return errUndefResult
		}
		return in.arith(nil, func(a, b float64) float64 { return a / b })
	})
	in.Define("idiv", func(in *Interp) error {
		b, a, err := in.twoInts()
		if err != nil {
			return err
		}
		if b == 0 {
			return errUndefResult
		}
		in.os = in.os[:len(in.os)-2]
		if a == math.MinInt32 && b == -1 {
			in.Push(Real(-float64(a)))
			return nil
		}
		in.Push(Int(int(a / b)))
		return nil
	})
	in.Define("mod", func(in *Interp) error {
		b, a, err := in.twoInts()
		if err != nil {
			return err
		}
		if b == 0 {
			return errUndefResult
		}
		in.os = in.os[:len(in.os)-2]
		in.Push(Int(int(a % b)))
		return nil
	})
	in.Define("abs", func(in *Interp) error {
		return in.unary(func(a int64) int64 {
			if a < 0 {
				return -a
			}
			return a
		}, math.Abs)
	})
	in.Define("neg", func(in *Interp) error {
		return in.unary(func(a int64) int64 { return -a }, func(a float64) float64 { return -a })
	})
	in.Define("ceiling", func(in *Interp) error {
		return in.unary(func(a int64) int64 { return a }, math.Ceil)
	})
	in.Define("floor", func(in *Interp) error {
		return in.unary(func(a int64) int64 { return a }, math.Floor)
	})
	in.Define("round", func(in *Interp) error {
		return in.unary(func(a int64) int64 { return a }, func(a float64) float64 { return math.Floor(a + 0.5) })
	})
	in.Define("truncate", func(in *Interp) error {
		return in.unary(func(a int64) int64 { return a }, math.Trunc)
	})
	in.Define("sqrt", func(in *Interp) error {
		return in.realOp(func(a float64) (float64, error) {
			if a < 0 {
				return 0, errRangecheck
			}
			return math.Sqrt(a), nil
		})
	})
	in.Define("atan", func(in *Interp) error {
		v, err := in.PopNums(2)
		if err != nil {
			return err
		}
		if v[0] == 0 && v[1] == 0 {
			return errUndefResult
		}
		d := math.Atan2(v[0], v[1]) * 180 / math.Pi
		if d < 0 {
			d += 360
		}
		in.PushNum(d)
		return nil
	})
	in.Define("cos", func(in *Interp) error {
		return in.realOp(func(a float64) (float64, error) { return cosDeg(a), nil })
	})
	in.Define("sin", func(in *Interp) error {
		return in.realOp(func(a float64) (float64, error) { return sinDeg(a), nil })
	})
	in.Define("exp", func(in *Interp) error {
		v, err := in.PopNums(2)
		if err != nil {
			return err
		}
		if v[0] < 0 && v[1] != math.Trunc(v[1]) || v[0] == 0 && v[1] < 0 {
			return errUndefResult
		}
		in.PushNum(math.Pow(v[0], v[1]))
		return nil
	})
	in.Define("ln", func(in *Interp) error {
		return in.realOp(func(a float64) (float64, error) {
			if a <= 0 {
				return 0, errRangecheck
			}
			return math.Log(a), nil
		})
	})
	in.Define("log", func(in *Interp) error {
		return in.realOp(func(a float64) (float64, error) {
			if a <= 0 {
				return 0, errRangecheck
			}
			return math.Log10(a), nil
		})
	})
	in.Define("rand", func(in *Interp) error {
		// the generator of Park and Miller, which many interpreters use
		in.seed = uint32(uint64(in.seed) * 16807 % 2147483647)
		in.Push(Int(int(in.seed)))
		return nil
	})
	in.Define("srand", func(in *Interp) error {
		n, err := in.PopInt()
		if err != nil {
			return err
		}
		in.seed = uint32(n) & 0x7fffffff
		if in.seed == 0 {
			in.seed = 1
		}
		return nil
	})
	in.Define("rrand", func(in *Interp) error {
		in.Push(Int(int(in.seed)))
		return nil
	})
}

// cosDeg returns the cosine of an angle in degrees, exact at the multiples
// of 90.
func cosDeg(a float64) float64 {
	return sinDeg(a + 90)
}

// sinDeg returns the sine of an angle in degrees, exact at the multiples of
// 90.
func sinDeg(a float64) float64 {
	a = math.Mod(a, 360)
	if a < 0 {
		a += 360
	}
	switch a {
	case 0, 180:
		return 0
	case 90:
		return 1
	case 270:
		return -1
	}
	return math.Sin(a * math.Pi / 180)
}

// twoInts returns the two integers on the top of the stack, the top first,
// without popping them.
func (in *Interp) twoInts() (int64, int64, error) {
	if len(in.os) < 2 {
		return 0, 0, errUnderflow
	}
	a, b := in.os[len(in.os)-2], in.os[len(in.os)-1]
	if a.typ != TInt || b.typ != TInt {
		return 0, 0, errTypecheck
	}
	return b.i, a.i, nil
}

// compare compares two numbers or two strings for the relational
// operators.
func compare(a, b Object) (int, error) {
	switch {
	case a.IsNumber() && b.IsNumber():
		x, y := a.Float(), b.Float()
		switch {
		case x < y:
			return -1, nil
		case x > y:
			return 1, nil
		}
		return 0, nil
	case a.typ == TString && b.typ == TString:
		return bytes.Compare(a.Bytes(), b.Bytes()), nil
	}
	return 0, errTypecheck
}

// opsRelational are the relational, boolean and bitwise operators (PLRM
// 8.1).
func (in *Interp) opsRelational() {
	in.Define("eq", func(in *Interp) error {
		if len(in.os) < 2 {
			return errUnderflow
		}
		a, b := in.os[len(in.os)-2], in.os[len(in.os)-1]
		in.os = in.os[:len(in.os)-2]
		in.Push(Bool(Equal(a, b)))
		return nil
	})
	in.Define("ne", func(in *Interp) error {
		if len(in.os) < 2 {
			return errUnderflow
		}
		a, b := in.os[len(in.os)-2], in.os[len(in.os)-1]
		in.os = in.os[:len(in.os)-2]
		in.Push(Bool(!Equal(a, b)))
		return nil
	})
	for _, rel := range []struct {
		name string
		ok   func(int) bool
	}{
		{"ge", func(c int) bool { return c >= 0 }},
		{"gt", func(c int) bool { return c > 0 }},
		{"le", func(c int) bool { return c <= 0 }},
		{"lt", func(c int) bool { return c < 0 }},
	} {
		in.Define(rel.name, func(in *Interp) error {
			if len(in.os) < 2 {
				return errUnderflow
			}
			a, b := in.os[len(in.os)-2], in.os[len(in.os)-1]
			c, err := compare(a, b)
			if err != nil {
				return err
			}
			in.os = in.os[:len(in.os)-2]
			in.Push(Bool(rel.ok(c)))
			return nil
		})
	}
	logic := func(name string, fb func(a, b bool) bool, fi func(a, b int64) int64) {
		in.Define(name, func(in *Interp) error {
			if len(in.os) < 2 {
				return errUnderflow
			}
			a, b := in.os[len(in.os)-2], in.os[len(in.os)-1]
			switch {
			case a.typ == TBool && b.typ == TBool:
				in.os = in.os[:len(in.os)-2]
				in.Push(Bool(fb(a.i != 0, b.i != 0)))
			case a.typ == TInt && b.typ == TInt:
				in.os = in.os[:len(in.os)-2]
				in.Push(Int(int(int32(fi(a.i, b.i)))))
			default:
				return errTypecheck
			}
			return nil
		})
	}
	logic("and", func(a, b bool) bool { return a && b }, func(a, b int64) int64 { return a & b })
	logic("or", func(a, b bool) bool { return a || b }, func(a, b int64) int64 { return a | b })
	logic("xor", func(a, b bool) bool { return a != b }, func(a, b int64) int64 { return a ^ b })
	in.Define("not", func(in *Interp) error {
		o, err := in.Peek(0)
		if err != nil {
			return err
		}
		switch o.typ {
		case TBool:
			in.os[len(in.os)-1] = Bool(o.i == 0)
		case TInt:
			in.os[len(in.os)-1] = Int(int(^int32(o.i)))
		default:
			return errTypecheck
		}
		return nil
	})
	in.Define("bitshift", func(in *Interp) error {
		shift, v, err := in.twoInts()
		if err != nil {
			return err
		}
		in.os = in.os[:len(in.os)-2]
		u := uint32(v)
		switch {
		case shift >= 32 || shift <= -32:
			u = 0
		case shift >= 0:
			u <<= uint(shift)
		default:
			u >>= uint(-shift)
		}
		in.Push(Int(int(int32(u))))
		return nil
	})
}
