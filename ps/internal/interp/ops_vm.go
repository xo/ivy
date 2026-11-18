package interp

import (
	"errors"
	"path"
)

// opsVM are the operators of the virtual memory (PLRM 8.1).
func (in *Interp) opsVM() {
	in.Define("save", func(in *Interp) error {
		var state any
		if in.Hooks.Save != nil {
			state = in.Hooks.Save()
		}
		s := in.vm.save(state)
		in.Push(Object{typ: TSave, ref: s})
		return nil
	})
	in.Define("restore", func(in *Interp) error {
		o, err := in.PopType(TSave)
		if err != nil {
			return err
		}
		s := o.ref.(*saveRec)
		if !s.valid {
			return ErrInvalidRestore
		}
		in.vm.restore(s)
		if in.Hooks.Restore != nil {
			in.Hooks.Restore(s.state)
		}
		return nil
	})
	in.Define("setglobal", func(in *Interp) error {
		b, err := in.PopBool()
		in.vm.global = b
		return err
	})
	in.Define("currentglobal", func(in *Interp) error {
		in.Push(Bool(in.vm.global))
		return nil
	})
	in.Define("gcheck", func(in *Interp) error {
		o, err := in.Pop()
		if err != nil {
			return err
		}
		global := true
		switch o.typ {
		case TString:
			global = o.ref.(*strBuf).global
		case TArray, TPacked:
			global = o.ref.(*arrBuf).global
		case TDict:
			global = o.Dict().global
		}
		in.Push(Bool(global))
		return nil
	})
	in.Define("startjob", func(in *Interp) error {
		if len(in.os) < 2 {
			return errUnderflow
		}
		in.os = in.os[:len(in.os)-2]
		in.Push(Bool(false))
		return nil
	})
	in.Define("vmstatus", func(in *Interp) error {
		in.Push(Int(in.vm.level))
		in.Push(Int(int(min(in.vm.used, 1<<31-1))))
		in.Push(Int(int(min(in.vm.max, 1<<31-1))))
		return nil
	})
	in.Define("vmreclaim", func(in *Interp) error {
		_, err := in.PopInt()
		return err
	})
	in.Define("setvmthreshold", func(in *Interp) error {
		_, err := in.PopInt()
		return err
	})
}

// opsMisc are the miscellaneous operators (PLRM 8.1).
func (in *Interp) opsMisc() {
	in.Define("bind", func(in *Interp) error {
		o, err := in.Peek(0)
		if err != nil {
			return err
		}
		if !o.IsArray() {
			return errTypecheck
		}
		in.bind(o, map[*arrBuf]bool{}, 0)
		return nil
	})
	in.Define("null", func(in *Interp) error {
		in.Push(Null)
		return nil
	})
	in.Define("version", func(in *Interp) error {
		return in.pushText("3010")
	})
	in.Define("product", func(in *Interp) error {
		return in.pushText("ivy")
	})
	in.Define("revision", func(in *Interp) error {
		in.Push(Int(1))
		return nil
	})
	in.Define("serialnumber", func(in *Interp) error {
		in.Push(Int(0))
		return nil
	})
	// languagelevel is an integer in systemdict, as in Ghostscript, so that
	// "systemdict /languagelevel get" gives the level. Adobe Illustrator
	// compares that value with 2. A name of an integer pushes the integer,
	// as the operator of PLRM does.
	in.SystemDict.PutName("languagelevel", Int(3))
	// the clocks count the operators that the program ran, one millisecond
	// for each thousand, and not the time: a program that seeds rand with
	// them, such as vasarely.ps, draws the same page on each run
	clock := func(in *Interp) error {
		in.Push(Int(int(in.ops / 1000 & 0x7fffffff)))
		return nil
	}
	in.Define("realtime", clock)
	in.Define("usertime", clock)
	internal := in.newDict(8)
	in.Define("internaldict", func(in *Interp) error {
		if _, err := in.PopInt(); err != nil {
			return err
		}
		in.Push(DictObj(internal))
		return nil
	})
	params := func(set, current string, d **Dict) {
		*d = in.newDict(8)
		in.Define(set, func(in *Interp) error {
			src, err := in.PopDict()
			if err != nil {
				return err
			}
			src.Each(func(k, v Object) bool {
				(*d).Put(k, v)
				return true
			})
			return nil
		})
		in.Define(current, func(in *Interp) error {
			c := in.newDict((*d).Len())
			(*d).Each(func(k, v Object) bool {
				c.Put(k, v)
				return true
			})
			in.Push(DictObj(c))
			return nil
		})
	}
	params("setsystemparams", "currentsystemparams", &in.systemParams)
	params("setuserparams", "currentuserparams", &in.userParams)
	in.userParams.PutName("MaxOpStack", Int(in.maxOStack))
	in.userParams.PutName("MaxDictStack", Int(in.maxDStack))
	in.userParams.PutName("MaxExecStack", Int(in.maxEStack))
	// The other parameters of PLRM appendix C hold sizes of caches that the
	// interpreter does not keep. A program reads them, such as Adobe
	// Illustrator, which copies MaxPatternCache into MaxPatternItem.
	for _, p := range []struct {
		d     *Dict
		name  string
		value int
	}{
		{in.userParams, "MaxFontItem", 12500},
		{in.userParams, "MinFontCompress", 100},
		{in.userParams, "MaxUPathItem", 0},
		{in.userParams, "MaxFormItem", 100000},
		{in.userParams, "MaxPatternItem", 20000},
		{in.userParams, "MaxScreenItem", 48000},
		{in.userParams, "VMReclaim", 0},
		{in.userParams, "VMThreshold", 8000000},
		{in.systemParams, "MaxFontCache", 400000},
		{in.systemParams, "CurFontCache", 0},
		{in.systemParams, "MaxOutlineCache", 65000},
		{in.systemParams, "CurOutlineCache", 0},
		{in.systemParams, "MaxUPathCache", 0},
		{in.systemParams, "CurUPathCache", 0},
		{in.systemParams, "MaxFormCache", 100000},
		{in.systemParams, "CurFormCache", 0},
		{in.systemParams, "MaxPatternCache", 100000},
		{in.systemParams, "CurPatternCache", 0},
		{in.systemParams, "MaxScreenStorage", 84000},
		{in.systemParams, "MaxDisplayList", 0},
		{in.systemParams, "CurDisplayList", 0},
	} {
		p.d.PutName(p.name, Int(p.value))
	}
	in.userParams.PutName("AccurateScreens", Bool(false))
	in.Define("setdevparams", func(in *Interp) error {
		if len(in.os) < 2 {
			return errUnderflow
		}
		in.os = in.os[:len(in.os)-2]
		return nil
	})
	in.Define("currentdevparams", func(in *Interp) error {
		if _, err := in.Pop(); err != nil {
			return err
		}
		in.Push(in.NewDict(0))
		return nil
	})
	in.Define("handleerror", func(in *Interp) error {
		name, _ := in.DollarError.GetName("errorname")
		cmd, _ := in.DollarError.GetName("command")
		in.print("%%[ Error: " + in.cvs(name) + "; OffendingCommand: " + in.cvs(cmd) + " ]%%")
		return nil
	})
}

// pushText pushes a new string.
func (in *Interp) pushText(s string) error {
	o, err := in.NewString([]byte(s))
	if err != nil {
		return err
	}
	in.Push(o)
	return nil
}

// bind replaces the names of a procedure whose values are operators with
// the operators, in the procedure and in the procedures that it holds
// (PLRM bind).
func (in *Interp) bind(o Object, seen map[*arrBuf]bool, depth int) {
	b := o.ref.(*arrBuf)
	if seen[b] || depth > 64 {
		return
	}
	seen[b] = true
	elems := o.Array()
	writable := o.access == Unlimited && o.typ == TArray
	for i, e := range elems {
		switch {
		case e.typ == TName && e.exec:
			v, ok := in.Lookup(e.NameVal())
			if ok && v.typ == TOperator && writable {
				in.vm.touchArray(b)
				elems[i] = v
			}
		case e.IsProc():
			in.bind(e, seen, depth+1)
			if e.access == Unlimited && writable {
				in.vm.touchArray(b)
				elems[i].access = ReadOnly
			}
		}
	}
}

// ResourceHooks let another package take part in the resources of a
// category, such as the fonts.
type ResourceHooks struct {
	// Define returns the instance that defineresource keeps.
	Define func(key, instance Object) (Object, error)
	// Find returns an instance that the category does not hold yet, such as
	// a standard font.
	Find func(key Object) (Object, bool, error)
}

// categories are the categories of resources (PLRM 3.9).
var categories = []string{
	"Category", "Generic", "Font", "CIDFont", "CMap", "Encoding", "Form",
	"Pattern", "ProcSet", "ColorSpace", "Halftone", "ColorRendering",
	"IdiomSet", "InkParams", "TrapParams", "OutputDevice", "ControlLanguage",
	"Localization", "PDL", "HWOptions", "FontSet", "Filter",
	"ColorSpaceFamily", "Emulator", "IODevice", "ColorRenderingType",
	"FMapType", "FontType", "FormType", "HalftoneType", "ImageType",
	"PatternType", "FunctionType", "ShadingType", "TrappingType",
	"CIDSystemInfo",
}

// Category returns the dictionary of the instances of a category, and
// makes it when it does not exist.
func (in *Interp) Category(name string) *Dict {
	if o, ok := in.Resources.GetName(name); ok && o.typ == TDict {
		return o.Dict()
	}
	d := in.newDict(16)
	d.global = true
	in.Resources.PutName(name, DictObj(d))
	return d
}

// SetResourceHooks sets the hooks of a category.
func (in *Interp) SetResourceHooks(category string, h ResourceHooks) {
	if in.resourceHooks == nil {
		in.resourceHooks = map[string]ResourceHooks{}
	}
	in.resourceHooks[category] = h
}

// FindResource returns an instance of a category, as findresource does.
func (in *Interp) FindResource(category string, k Object) (Object, error) {
	d := in.Category(category)
	if v, ok := d.Get(k); ok {
		return v, nil
	}
	if h, ok := in.resourceHooks[category]; ok && h.Find != nil {
		v, ok, err := h.Find(k)
		if err != nil {
			return Null, err
		}
		if ok {
			return v, nil
		}
	}
	in.errCommand = k
	return Null, ErrUndefinedResource
}

// opsResource are the operators of resources (PLRM 3.9).
func (in *Interp) opsResource() {
	for _, c := range categories {
		in.Category(c)
	}
	in.Define("defineresource", func(in *Interp) error {
		if len(in.os) < 3 {
			return errUnderflow
		}
		k, inst, cat := in.os[len(in.os)-3], in.os[len(in.os)-2], in.os[len(in.os)-1]
		if cat.typ != TName {
			return errTypecheck
		}
		if h, ok := in.resourceHooks[cat.NameVal()]; ok && h.Define != nil {
			v, err := h.Define(k, inst)
			if err != nil {
				return err
			}
			inst = v
		}
		// A new category names itself in its implementation, so that the
		// procedures of the implementation act on it (PLRM 3.9.2).
		if cat.NameVal() == "Category" && k.typ == TName && inst.typ == TDict {
			inst.Dict().PutName("Category", k)
		}
		if !in.Category(cat.NameVal()).Put(k, inst) {
			return errTypecheck
		}
		in.os = in.os[:len(in.os)-3]
		in.Push(inst)
		return nil
	})
	in.Define("undefineresource", func(in *Interp) error {
		if len(in.os) < 2 {
			return errUnderflow
		}
		k, cat := in.os[len(in.os)-2], in.os[len(in.os)-1]
		if cat.typ != TName {
			return errTypecheck
		}
		in.Category(cat.NameVal()).Delete(k)
		in.os = in.os[:len(in.os)-2]
		return nil
	})
	in.Define("findresource", func(in *Interp) error {
		if len(in.os) < 2 {
			return errUnderflow
		}
		k, cat := in.os[len(in.os)-2], in.os[len(in.os)-1]
		if cat.typ != TName {
			return errTypecheck
		}
		v, err := in.FindResource(cat.NameVal(), k)
		if err != nil {
			return err
		}
		in.os = in.os[:len(in.os)-2]
		in.Push(v)
		return nil
	})
	in.Define("findencoding", func(in *Interp) error {
		k, err := in.Peek(0)
		if err != nil {
			return err
		}
		v, err := in.FindResource("Encoding", k)
		if err != nil {
			return err
		}
		in.os[len(in.os)-1] = v
		return nil
	})
	in.Define("resourcestatus", func(in *Interp) error {
		if len(in.os) < 2 {
			return errUnderflow
		}
		k, cat := in.os[len(in.os)-2], in.os[len(in.os)-1]
		if cat.typ != TName {
			return errTypecheck
		}
		in.os = in.os[:len(in.os)-2]
		if in.Category(cat.NameVal()).Known(k) {
			in.Push(Int(1))
			in.Push(Int(0))
			in.Push(Bool(true))
			return nil
		}
		if h, ok := in.resourceHooks[cat.NameVal()]; ok && h.Find != nil {
			if _, ok, err := h.Find(k); err == nil && ok {
				in.Push(Int(2))
				in.Push(Int(0))
				in.Push(Bool(true))
				return nil
			}
		}
		in.Push(Bool(false))
		return nil
	})
	in.Define("resourceforall", func(in *Interp) error {
		if len(in.os) < 4 {
			return errUnderflow
		}
		tmpl, proc, scratch, cat := in.os[len(in.os)-4], in.os[len(in.os)-3], in.os[len(in.os)-2], in.os[len(in.os)-1]
		if tmpl.typ != TString || !proc.IsArray() || scratch.typ != TString || cat.typ != TName {
			return errTypecheck
		}
		in.os = in.os[:len(in.os)-4]
		pattern := tmpl.Text()
		for _, k := range in.Category(cat.NameVal()).Keys() {
			text := in.cvs(k)
			if ok, _ := path.Match(pattern, text); !ok || len(text) > int(scratch.n) {
				continue
			}
			s := scratch
			copy(s.Bytes(), text)
			s.n = int32(len(text))
			in.Push(s)
			if err := in.Call(proc); err != nil {
				if errors.Is(err, errExit) {
					return nil
				}
				return err
			}
		}
		return nil
	})
	in.implementations()
}

// implementations makes the implementation of each category, an instance of
// the category Category (PLRM 3.9.2). Adobe Illustrator copies the
// implementation of Generic to make categories of its own, and calls the
// procedure ResourceStatus of an implementation. Each procedure of an
// implementation is the operator of the same name: it reads the name of its
// category from the key Category of the dictionary stack, where the caller
// put the implementation with begin.
func (in *Interp) implementations() {
	procs := [...][2]string{
		{"DefineResource", "defineresource"},
		{"UndefineResource", "undefineresource"},
		{"FindResource", "findresource"},
		{"ResourceStatus", "resourcestatus"},
		{"ResourceForAll", "resourceforall"},
	}
	var ops [len(procs)]Object
	for i, p := range procs {
		o, ok := in.SystemDict.GetName(p[1])
		if !ok || o.typ != TOperator {
			continue
		}
		fn := o.ref.(*Operator).Fn
		ops[i] = OpObj(&Operator{Name: p[0], Fn: func(in *Interp) error {
			cat, ok := in.Lookup("Category")
			if !ok || cat.typ != TName {
				return errUndefined
			}
			n := len(in.os)
			in.Push(cat)
			err := fn(in)
			if err != nil && len(in.os) == n+1 {
				// The operator failed before it took its operands.
				in.os = in.os[:n]
			}
			return err
		}})
	}
	old := in.SetGlobal(true)
	defer in.SetGlobal(old)
	impls := in.Category("Category")
	for _, c := range categories {
		d := in.newDict(len(procs) + 2)
		d.PutName("Category", NameObj(c))
		d.PutName("InstanceType", NameObj("dicttype"))
		for i, p := range procs {
			if ops[i].typ == TOperator {
				d.PutName(p[0], ops[i])
			}
		}
		impls.PutName(c, DictObj(d))
	}
}
