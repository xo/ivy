package interp

import (
	"math"
	"strconv"
	"strings"
)

// opsControl are the operators of control (PLRM 8.1).
func (in *Interp) opsControl() {
	in.Define("exec", func(in *Interp) error {
		o, err := in.Pop()
		if err != nil {
			return err
		}
		return in.execObj(o)
	})
	in.Define("if", func(in *Interp) error {
		if len(in.os) < 2 {
			return errUnderflow
		}
		c, proc := in.os[len(in.os)-2], in.os[len(in.os)-1]
		if c.typ != TBool || !proc.IsArray() {
			return errTypecheck
		}
		in.os = in.os[:len(in.os)-2]
		if c.i != 0 {
			return in.execObj(proc)
		}
		return nil
	})
	in.Define("ifelse", func(in *Interp) error {
		if len(in.os) < 3 {
			return errUnderflow
		}
		c, p1, p2 := in.os[len(in.os)-3], in.os[len(in.os)-2], in.os[len(in.os)-1]
		if c.typ != TBool || !p1.IsArray() || !p2.IsArray() {
			return errTypecheck
		}
		in.os = in.os[:len(in.os)-3]
		if c.i != 0 {
			return in.execObj(p1)
		}
		return in.execObj(p2)
	})
	in.Define("for", func(in *Interp) error {
		if len(in.os) < 4 {
			return errUnderflow
		}
		s := in.os[len(in.os)-4:]
		if !s[0].IsNumber() || !s[1].IsNumber() || !s[2].IsNumber() || !s[3].IsArray() {
			return errTypecheck
		}
		f := frame{
			kind:  fFor,
			proc:  s[3],
			cur:   s[0].Float(),
			inc:   s[1].Float(),
			lim:   s[2].Float(),
			isInt: s[0].typ == TInt && s[1].typ == TInt,
		}
		in.os = in.os[:len(in.os)-4]
		return in.pushFrame(f)
	})
	in.Define("repeat", func(in *Interp) error {
		if len(in.os) < 2 {
			return errUnderflow
		}
		n, proc := in.os[len(in.os)-2], in.os[len(in.os)-1]
		if n.typ != TInt || !proc.IsArray() {
			return errTypecheck
		}
		if n.i < 0 {
			return errRangecheck
		}
		in.os = in.os[:len(in.os)-2]
		return in.pushFrame(frame{kind: fRepeat, proc: proc, i: int(n.i)})
	})
	in.Define("loop", func(in *Interp) error {
		proc, err := in.PopProc()
		if err != nil {
			return err
		}
		return in.pushFrame(frame{kind: fLoop, proc: proc})
	})
	in.Define("exit", func(*Interp) error {
		return errExit
	})
	in.Define("stop", func(*Interp) error {
		return errStop
	})
	in.Define("stopped", func(in *Interp) error {
		o, err := in.Pop()
		if err != nil {
			return err
		}
		if err := in.pushFrame(frame{kind: fStopped}); err != nil {
			return err
		}
		return in.pushFrame(frame{kind: fObj, obj: o})
	})
	in.Define("countexecstack", func(in *Interp) error {
		in.Push(Int(len(in.es)))
		return nil
	})
	in.Define("execstack", func(in *Interp) error {
		o, err := in.Pop()
		if err != nil {
			return err
		}
		if o.typ != TArray {
			return errTypecheck
		}
		if int(o.n) < len(in.es) {
			return errRangecheck
		}
		in.Touch(o)
		for i, f := range in.es {
			o.Array()[i] = f.obj
		}
		o.n = int32(len(in.es))
		in.Push(o)
		return nil
	})
	in.Define("quit", func(*Interp) error {
		return errQuit
	})
	in.Define("start", func(*Interp) error {
		return nil
	})
}

// opsType are the operators of types, attributes and conversions (PLRM
// 8.1).
func (in *Interp) opsType() {
	in.Define("type", func(in *Interp) error {
		o, err := in.Pop()
		if err != nil {
			return err
		}
		in.Push(ExecName(o.typ.String()))
		return nil
	})
	in.Define("cvlit", func(in *Interp) error {
		o, err := in.Pop()
		if err != nil {
			return err
		}
		in.Push(o.Literal())
		return nil
	})
	in.Define("cvx", func(in *Interp) error {
		o, err := in.Pop()
		if err != nil {
			return err
		}
		in.Push(o.Exec())
		return nil
	})
	in.Define("xcheck", func(in *Interp) error {
		o, err := in.Pop()
		if err != nil {
			return err
		}
		in.Push(Bool(o.exec))
		return nil
	})
	access := func(name string, a Access) {
		in.Define(name, func(in *Interp) error {
			o, err := in.Pop()
			if err != nil {
				return err
			}
			switch o.typ {
			case TDict:
				d := o.Dict()
				d.access = max(d.access, a)
			case TString, TArray, TPacked, TFile:
				o.access = max(o.access, a)
			default:
				return errTypecheck
			}
			in.Push(o)
			return nil
		})
	}
	access("readonly", ReadOnly)
	access("executeonly", ExecuteOnly)
	access("noaccess", NoAccess)
	in.Define("rcheck", func(in *Interp) error {
		o, err := in.Pop()
		if err != nil {
			return err
		}
		in.Push(Bool(o.Access() <= ReadOnly))
		return nil
	})
	in.Define("wcheck", func(in *Interp) error {
		o, err := in.Pop()
		if err != nil {
			return err
		}
		in.Push(Bool(o.Access() == Unlimited && o.typ != TPacked))
		return nil
	})
	in.Define("cvi", func(in *Interp) error {
		o, err := in.Peek(0)
		if err != nil {
			return err
		}
		var v float64
		switch o.typ {
		case TInt:
			return nil
		case TReal:
			v = o.r
		case TString:
			n, ok := parseNumber(strings.TrimSpace(o.Text()))
			if !ok {
				return errSyntax
			}
			v = n.Float()
		default:
			return errTypecheck
		}
		v = math.Trunc(v)
		if math.IsNaN(v) || v < math.MinInt32 || v > math.MaxInt32 {
			return errRangecheck
		}
		in.os[len(in.os)-1] = Int(int(v))
		return nil
	})
	in.Define("cvr", func(in *Interp) error {
		o, err := in.Peek(0)
		if err != nil {
			return err
		}
		switch o.typ {
		case TInt, TReal:
			in.os[len(in.os)-1] = Real(o.Float())
		case TString:
			n, ok := parseNumber(strings.TrimSpace(o.Text()))
			if !ok {
				return errSyntax
			}
			in.os[len(in.os)-1] = Real(n.Float())
		default:
			return errTypecheck
		}
		return nil
	})
	in.Define("cvn", func(in *Interp) error {
		o, err := in.Peek(0)
		if err != nil {
			return err
		}
		if o.typ != TString {
			return errTypecheck
		}
		n := NameObj(o.Text())
		n.exec = o.exec
		in.os[len(in.os)-1] = n
		return nil
	})
	in.Define("cvs", func(in *Interp) error {
		if len(in.os) < 2 {
			return errUnderflow
		}
		o, s := in.os[len(in.os)-2], in.os[len(in.os)-1]
		if s.typ != TString {
			return errTypecheck
		}
		text := in.cvs(o)
		if len(text) > int(s.n) {
			return errRangecheck
		}
		copy(s.Bytes(), text)
		s.n = int32(len(text))
		in.os = in.os[:len(in.os)-2]
		in.Push(s)
		return nil
	})
	in.Define("cvrs", func(in *Interp) error {
		if len(in.os) < 3 {
			return errUnderflow
		}
		o, r, s := in.os[len(in.os)-3], in.os[len(in.os)-2], in.os[len(in.os)-1]
		if !o.IsNumber() || r.typ != TInt || s.typ != TString {
			return errTypecheck
		}
		radix := int(r.i)
		if radix < 2 || radix > 36 {
			return errRangecheck
		}
		var text string
		switch {
		case radix == 10 && o.typ == TReal:
			text = formatReal(o.r)
		case radix == 10:
			text = strconv.Itoa(int(o.i))
		default:
			text = strings.ToUpper(strconv.FormatUint(uint64(uint32(int32(o.Float()))), radix))
		}
		if len(text) > int(s.n) {
			return errRangecheck
		}
		copy(s.Bytes(), text)
		s.n = int32(len(text))
		in.os = in.os[:len(in.os)-3]
		in.Push(s)
		return nil
	})
}

// cvs returns the text of an object, as cvs gives it.
func (in *Interp) cvs(o Object) string {
	switch o.typ {
	case TInt:
		return strconv.Itoa(int(o.i))
	case TReal:
		return formatReal(o.r)
	case TBool:
		if o.i != 0 {
			return "true"
		}
		return "false"
	case TString:
		return o.Text()
	case TName:
		return o.NameVal()
	case TOperator:
		return o.Op().Name
	case TNull:
		return "null"
	}
	return "--nostringval--"
}

// repr returns the text of an object, as == prints it.
func (in *Interp) repr(o Object, depth int) string {
	switch o.typ {
	case TString:
		var b strings.Builder
		b.WriteByte('(')
		for _, c := range o.Bytes() {
			switch {
			case c == '(' || c == ')' || c == '\\':
				b.WriteByte('\\')
				b.WriteByte(c)
			case c < 32 || c >= 127:
				b.WriteString("\\" + strconv.FormatInt(int64(c), 8))
			default:
				b.WriteByte(c)
			}
		}
		b.WriteByte(')')
		return b.String()
	case TName:
		if o.exec {
			return o.NameVal()
		}
		return "/" + o.NameVal()
	case TArray, TPacked:
		if depth > 8 {
			return "..."
		}
		open, end := "[", "]"
		if o.exec {
			open, end = "{", "}"
		}
		parts := make([]string, 0, o.n)
		for _, e := range o.Array() {
			parts = append(parts, in.repr(e, depth+1))
		}
		return open + strings.Join(parts, " ") + end
	case TDict:
		return "-dict-"
	case TOperator:
		return "--" + o.Op().Name + "--"
	case TMark:
		return "-mark-"
	case TFile:
		return "-file-"
	case TSave:
		return "-save-"
	case TFontID:
		return "-fontID-"
	case TGState:
		return "-gstate-"
	}
	return in.cvs(o)
}
