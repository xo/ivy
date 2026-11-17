package interp

import (
	"bytes"
)

// checkWrite returns invalidaccess for an object that a program cannot
// change.
func checkWrite(o Object) error {
	if o.Access() != Unlimited || o.typ == TPacked {
		return errInvalidAcc
	}
	return nil
}

// opsArray are the operators of arrays, packed arrays and strings (PLRM
// 8.1).
func (in *Interp) opsArray() {
	in.Define("array", func(in *Interp) error {
		n, err := in.PopInt()
		if err != nil {
			return err
		}
		if n < 0 {
			return errRangecheck
		}
		if n > in.maxArray {
			return errLimitcheck
		}
		elems := make([]Object, n)
		o, err := in.NewArray(elems)
		if err != nil {
			return err
		}
		in.Push(o)
		return nil
	})
	in.Define("[", func(in *Interp) error {
		in.Push(Mark)
		return nil
	})
	in.Define("]", func(in *Interp) error {
		i, err := in.markIndex()
		if err != nil {
			return err
		}
		elems := append([]Object(nil), in.os[i+1:]...)
		o, err := in.NewArray(elems)
		if err != nil {
			return err
		}
		in.os = in.os[:i]
		in.Push(o)
		return nil
	})
	in.Define("packedarray", func(in *Interp) error {
		n, err := in.PopInt()
		if err != nil {
			return err
		}
		if n < 0 || n > len(in.os) {
			return errRangecheck
		}
		elems := append([]Object(nil), in.os[len(in.os)-n:]...)
		o, err := in.NewArray(elems)
		if err != nil {
			return err
		}
		in.os = in.os[:len(in.os)-n]
		o.typ = TPacked
		o.access = ReadOnly
		in.Push(o)
		return nil
	})
	in.Define("setpacking", func(in *Interp) error {
		b, err := in.PopBool()
		in.packing = b
		return err
	})
	in.Define("currentpacking", func(in *Interp) error {
		in.Push(Bool(in.packing))
		return nil
	})
	in.Define("length", func(in *Interp) error {
		o, err := in.Pop()
		if err != nil {
			return err
		}
		switch o.typ {
		case TString, TArray, TPacked, TDict:
			in.Push(Int(o.Len()))
		case TName:
			in.Push(Int(len(o.NameVal())))
		default:
			return errTypecheck
		}
		return nil
	})
	in.Define("get", opGet)
	in.Define("put", opPut)
	in.Define("getinterval", func(in *Interp) error {
		if len(in.os) < 3 {
			return errUnderflow
		}
		o, io, co := in.os[len(in.os)-3], in.os[len(in.os)-2], in.os[len(in.os)-1]
		if io.typ != TInt || co.typ != TInt {
			return errTypecheck
		}
		i, n := int(io.i), int(co.i)
		switch o.typ {
		case TString, TArray, TPacked:
		default:
			return errTypecheck
		}
		if i < 0 || n < 0 || i+n > int(o.n) {
			return errRangecheck
		}
		in.os = in.os[:len(in.os)-3]
		o.off += int32(i)
		o.n = int32(n)
		in.Push(o)
		return nil
	})
	in.Define("putinterval", func(in *Interp) error {
		if len(in.os) < 3 {
			return errUnderflow
		}
		dst, io, src := in.os[len(in.os)-3], in.os[len(in.os)-2], in.os[len(in.os)-1]
		if io.typ != TInt {
			return errTypecheck
		}
		i := int(io.i)
		switch {
		case dst.typ == TArray && src.IsArray():
			if i < 0 || i+int(src.n) > int(dst.n) {
				return errRangecheck
			}
			if err := checkWrite(dst); err != nil {
				return err
			}
			in.Touch(dst)
			copy(dst.Array()[i:], src.Array())
		case dst.typ == TString && src.typ == TString:
			if i < 0 || i+int(src.n) > int(dst.n) {
				return errRangecheck
			}
			if err := checkWrite(dst); err != nil {
				return err
			}
			copy(dst.Bytes()[i:], src.Bytes())
		default:
			return errTypecheck
		}
		in.os = in.os[:len(in.os)-3]
		return nil
	})
	in.Define("aload", func(in *Interp) error {
		o, err := in.Peek(0)
		if err != nil {
			return err
		}
		if !o.IsArray() {
			return errTypecheck
		}
		in.os = in.os[:len(in.os)-1]
		in.os = append(in.os, o.Array()...)
		in.Push(o)
		return nil
	})
	in.Define("astore", func(in *Interp) error {
		o, err := in.Peek(0)
		if err != nil {
			return err
		}
		if o.typ != TArray {
			return errTypecheck
		}
		n := int(o.n)
		if len(in.os)-1 < n {
			return errUnderflow
		}
		if err := checkWrite(o); err != nil {
			return err
		}
		in.Touch(o)
		copy(o.Array(), in.os[len(in.os)-1-n:len(in.os)-1])
		in.os = in.os[:len(in.os)-1-n]
		in.Push(o)
		return nil
	})
	in.Define("forall", func(in *Interp) error {
		if len(in.os) < 2 {
			return errUnderflow
		}
		o, proc := in.os[len(in.os)-2], in.os[len(in.os)-1]
		if !proc.IsArray() {
			return errTypecheck
		}
		f := frame{kind: fForall, obj: o, proc: proc}
		switch o.typ {
		case TArray, TPacked, TString:
		case TDict:
			f.keys = o.Dict().Keys()
		default:
			return errTypecheck
		}
		in.os = in.os[:len(in.os)-2]
		return in.pushFrame(f)
	})
	in.Define("string", func(in *Interp) error {
		n, err := in.PopInt()
		if err != nil {
			return err
		}
		if n < 0 {
			return errRangecheck
		}
		if n > in.maxString {
			return errLimitcheck
		}
		o, err := in.NewString(make([]byte, n))
		if err != nil {
			return err
		}
		in.Push(o)
		return nil
	})
	in.Define("anchorsearch", func(in *Interp) error {
		if len(in.os) < 2 {
			return errUnderflow
		}
		s, seek := in.os[len(in.os)-2], in.os[len(in.os)-1]
		if s.typ != TString || seek.typ != TString {
			return errTypecheck
		}
		in.os = in.os[:len(in.os)-2]
		if bytes.HasPrefix(s.Bytes(), seek.Bytes()) {
			post, match := s, s
			post.off += seek.n
			post.n -= seek.n
			match.n = seek.n
			in.Push(post)
			in.Push(match)
			in.Push(Bool(true))
			return nil
		}
		in.Push(s)
		in.Push(Bool(false))
		return nil
	})
	in.Define("search", func(in *Interp) error {
		if len(in.os) < 2 {
			return errUnderflow
		}
		s, seek := in.os[len(in.os)-2], in.os[len(in.os)-1]
		if s.typ != TString || seek.typ != TString {
			return errTypecheck
		}
		in.os = in.os[:len(in.os)-2]
		i := bytes.Index(s.Bytes(), seek.Bytes())
		if i < 0 {
			in.Push(s)
			in.Push(Bool(false))
			return nil
		}
		post, match, pre := s, s, s
		post.off += int32(i) + seek.n
		post.n -= int32(i) + seek.n
		match.off += int32(i)
		match.n = seek.n
		pre.n = int32(i)
		in.Push(post)
		in.Push(match)
		in.Push(pre)
		in.Push(Bool(true))
		return nil
	})
}

// opGet is get.
func opGet(in *Interp) error {
	if len(in.os) < 2 {
		return errUnderflow
	}
	o, k := in.os[len(in.os)-2], in.os[len(in.os)-1]
	var v Object
	switch o.typ {
	case TArray, TPacked, TString:
		if k.typ != TInt {
			return errTypecheck
		}
		i := int(k.i)
		if i < 0 || i >= int(o.n) {
			return errRangecheck
		}
		if o.typ == TString {
			v = Int(int(o.Bytes()[i]))
		} else {
			v = o.Array()[i]
		}
	case TDict:
		var ok bool
		if v, ok = o.Dict().Get(k); !ok {
			return errUndefined
		}
	default:
		return errTypecheck
	}
	in.os = in.os[:len(in.os)-2]
	in.Push(v)
	return nil
}

// opPut is put.
func opPut(in *Interp) error {
	if len(in.os) < 3 {
		return errUnderflow
	}
	o, k, v := in.os[len(in.os)-3], in.os[len(in.os)-2], in.os[len(in.os)-1]
	switch o.typ {
	case TArray, TString:
		if k.typ != TInt {
			return errTypecheck
		}
		i := int(k.i)
		if i < 0 || i >= int(o.n) {
			return errRangecheck
		}
		if err := checkWrite(o); err != nil {
			return err
		}
		if o.typ == TString {
			if v.typ != TInt {
				return errTypecheck
			}
			o.Bytes()[i] = byte(v.i)
		} else {
			in.Touch(o)
			o.Array()[i] = v
		}
	case TDict:
		d := o.Dict()
		if !d.Writable() {
			return errInvalidAcc
		}
		if !d.Put(k, v) {
			return errTypecheck
		}
	case TPacked:
		return errInvalidAcc
	default:
		return errTypecheck
	}
	in.os = in.os[:len(in.os)-3]
	return nil
}

// opsDict are the operators of dictionaries (PLRM 8.1).
func (in *Interp) opsDict() {
	in.Define("dict", func(in *Interp) error {
		n, err := in.PopInt()
		if err != nil {
			return err
		}
		if n < 0 {
			return errRangecheck
		}
		in.Push(in.NewDict(n))
		return nil
	})
	in.Define("<<", func(in *Interp) error {
		in.Push(Mark)
		return nil
	})
	in.Define(">>", func(in *Interp) error {
		i, err := in.markIndex()
		if err != nil {
			return err
		}
		elems := in.os[i+1:]
		if len(elems)%2 != 0 {
			return errRangecheck
		}
		d := in.newDict(len(elems) / 2)
		for j := 0; j < len(elems); j += 2 {
			if !d.Put(elems[j], elems[j+1]) {
				return errTypecheck
			}
		}
		in.os = in.os[:i]
		in.Push(DictObj(d))
		return nil
	})
	in.Define("maxlength", func(in *Interp) error {
		d, err := in.PopDict()
		if err != nil {
			return err
		}
		in.Push(Int(d.MaxLen()))
		return nil
	})
	in.Define("begin", func(in *Interp) error {
		d, err := in.PopDict()
		if err != nil {
			return err
		}
		return in.Begin(d)
	})
	in.Define("end", func(in *Interp) error {
		return in.End()
	})
	in.Define("def", func(in *Interp) error {
		if len(in.os) < 2 {
			return errUnderflow
		}
		k, v := in.os[len(in.os)-2], in.os[len(in.os)-1]
		d := in.CurrentDict()
		if !d.Writable() {
			return errInvalidAcc
		}
		if !d.Put(k, v) {
			return errTypecheck
		}
		in.os = in.os[:len(in.os)-2]
		return nil
	})
	in.Define("load", func(in *Interp) error {
		k, err := in.Peek(0)
		if err != nil {
			return err
		}
		v, _, ok := in.lookupKey(k)
		if !ok {
			in.errCommand = k
			return errUndefined
		}
		in.os[len(in.os)-1] = v
		return nil
	})
	in.Define("store", func(in *Interp) error {
		if len(in.os) < 2 {
			return errUnderflow
		}
		k, v := in.os[len(in.os)-2], in.os[len(in.os)-1]
		_, d, ok := in.lookupKey(k)
		if !ok {
			d = in.CurrentDict()
		}
		if !d.Writable() {
			return errInvalidAcc
		}
		if !d.Put(k, v) {
			return errTypecheck
		}
		in.os = in.os[:len(in.os)-2]
		return nil
	})
	in.Define("known", func(in *Interp) error {
		if len(in.os) < 2 {
			return errUnderflow
		}
		o, k := in.os[len(in.os)-2], in.os[len(in.os)-1]
		if o.typ != TDict {
			return errTypecheck
		}
		in.os = in.os[:len(in.os)-2]
		in.Push(Bool(o.Dict().Known(k)))
		return nil
	})
	in.Define("undef", func(in *Interp) error {
		if len(in.os) < 2 {
			return errUnderflow
		}
		o, k := in.os[len(in.os)-2], in.os[len(in.os)-1]
		if o.typ != TDict {
			return errTypecheck
		}
		in.os = in.os[:len(in.os)-2]
		o.Dict().Delete(k)
		return nil
	})
	in.Define("where", func(in *Interp) error {
		k, err := in.Pop()
		if err != nil {
			return err
		}
		if _, d, ok := in.lookupKey(k); ok {
			in.Push(DictObj(d))
			in.Push(Bool(true))
			return nil
		}
		in.Push(Bool(false))
		return nil
	})
	in.Define("currentdict", func(in *Interp) error {
		in.Push(DictObj(in.CurrentDict()))
		return nil
	})
	in.Define("countdictstack", func(in *Interp) error {
		in.Push(Int(len(in.ds)))
		return nil
	})
	in.Define("dictstack", func(in *Interp) error {
		o, err := in.Pop()
		if err != nil {
			return err
		}
		if o.typ != TArray {
			return errTypecheck
		}
		if int(o.n) < len(in.ds) {
			return errRangecheck
		}
		in.Touch(o)
		for i, d := range in.ds {
			o.Array()[i] = DictObj(d)
		}
		o.n = int32(len(in.ds))
		in.Push(o)
		return nil
	})
	in.Define("cleardictstack", func(in *Interp) error {
		in.ds = in.ds[:3]
		return nil
	})
}
