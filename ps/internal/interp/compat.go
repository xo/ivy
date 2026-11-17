package interp

// opsCompat defines the names of Ghostscript that the test files of
// Ghostscript and some programs use without a check: min, max, .min and
// .max, QUIET and =string. They are not operators of the PLRM.
func (in *Interp) opsCompat() {
	pick := func(name string, larger bool) {
		in.Define(name, func(in *Interp) error {
			if len(in.os) < 2 {
				return errUnderflow
			}
			a, b := in.os[len(in.os)-2], in.os[len(in.os)-1]
			if !a.IsNumber() || !b.IsNumber() {
				return errTypecheck
			}
			in.os = in.os[:len(in.os)-2]
			if (a.Float() > b.Float()) == larger {
				in.Push(a)
			} else {
				in.Push(b)
			}
			return nil
		})
	}
	pick("max", true)
	pick("min", false)
	pick(".max", true)
	pick(".min", false)
	in.SystemDict.PutName("QUIET", Bool(true))
	if s, err := in.NewString(make([]byte, 256)); err == nil {
		in.SystemDict.PutName("=string", s)
	}
}

// maxUserObjects bounds the index of a user object.
const maxUserObjects = 1 << 16

// opsUserObjects defines the user objects of Level 2 (PLRM 3.7.6), and the
// names of Display PostScript for the global VM: setshared, currentshared
// and shareddict.
func (in *Interp) opsUserObjects() {
	in.Define("defineuserobject", func(in *Interp) error {
		o, err := in.Pop()
		if err != nil {
			return err
		}
		i, err := in.PopInt()
		if err != nil {
			return err
		}
		if i < 0 || i >= maxUserObjects {
			return errRangecheck
		}
		cur, _ := in.UserDict.GetName("UserObjects")
		elems := cur.Array()
		if i >= len(elems) {
			grown := make([]Object, max(i+1, 2*len(elems)))
			copy(grown, elems)
			for j := len(elems); j < len(grown); j++ {
				grown[j] = Null
			}
			arr, err := in.NewArray(grown)
			if err != nil {
				return err
			}
			in.UserDict.PutName("UserObjects", arr)
			cur = arr
		}
		in.Touch(cur)
		cur.Array()[i] = o
		return nil
	})
	in.Define("execuserobject", func(in *Interp) error {
		i, err := in.PopInt()
		if err != nil {
			return err
		}
		cur, ok := in.UserDict.GetName("UserObjects")
		if !ok || i < 0 || i >= cur.Len() {
			return errUndefined
		}
		return in.execObj(cur.Array()[i])
	})
	in.Define("undefineuserobject", func(in *Interp) error {
		i, err := in.PopInt()
		if err != nil {
			return err
		}
		cur, ok := in.UserDict.GetName("UserObjects")
		if ok && i >= 0 && i < cur.Len() {
			in.Touch(cur)
			cur.Array()[i] = Null
		}
		return nil
	})
	if g, ok := in.SystemDict.GetName("setglobal"); ok {
		in.SystemDict.PutName("setshared", g)
	}
	if g, ok := in.SystemDict.GetName("currentglobal"); ok {
		in.SystemDict.PutName("currentshared", g)
	}
	in.SystemDict.PutName("shareddict", DictObj(in.GlobalDict))
}
