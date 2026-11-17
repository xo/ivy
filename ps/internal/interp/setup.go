package interp

// setup makes the standard dictionaries and defines the operators of the
// language.
func (in *Interp) setup() {
	in.vm.global = true
	in.SystemDict = in.newDict(512)
	in.GlobalDict = in.newDict(64)
	in.Resources = in.newDict(64)
	in.ErrorDict = in.newDict(32)
	in.DollarError = in.newDict(16)
	in.StatusDict = in.newDict(32)
	in.ds = []*Dict{in.SystemDict, in.GlobalDict}
	in.vm.global = false
	in.UserDict = in.newDict(256)
	in.ds = append(in.ds, in.UserDict)

	in.opsStack()
	in.opsMath()
	in.opsRelational()
	in.opsArray()
	in.opsDict()
	in.opsControl()
	in.opsType()
	in.opsFile()
	in.opsVM()
	in.opsMisc()
	in.opsResource()
	in.opsEexec()
	in.opsCompat()
	in.opsUserObjects()

	sd := in.SystemDict
	sd.PutName("true", Bool(true))
	sd.PutName("false", Bool(false))
	sd.PutName("systemdict", DictObj(sd))
	sd.PutName("globaldict", DictObj(in.GlobalDict))
	sd.PutName("userdict", DictObj(in.UserDict))
	sd.PutName("errordict", DictObj(in.ErrorDict))
	sd.PutName("$error", DictObj(in.DollarError))
	sd.PutName("statusdict", DictObj(in.StatusDict))
	sd.PutName("serverdict", in.NewDict(4))
	if d, ok := sd.GetName("serverdict"); ok {
		d.Dict().PutName("exitserver", OpObj(&Operator{Name: "exitserver", Fn: func(in *Interp) error {
			_, err := in.Pop()
			return err
		}}))
	}

	for _, e := range allErrors {
		in.ErrorDict.PutName(e.Name, OpObj(&Operator{Name: e.Name, Fn: func(in *Interp) error {
			cmd, err := in.Pop()
			if err != nil {
				cmd = Null
			}
			in.recordError(e.Name, cmd)
			return errStop
		}}))
	}
	if h, ok := sd.GetName("handleerror"); ok {
		in.ErrorDict.PutName("handleerror", h)
	}
	in.DollarError.PutName("newerror", Bool(false))
	in.DollarError.PutName("errorname", Null)
	in.DollarError.PutName("command", Null)
	in.DollarError.PutName("recordstacks", Bool(true))
	in.DollarError.PutName("binary", Bool(false))

	st := in.StatusDict
	st.PutName("product", in.mustString("ivy"))
	st.PutName("revision", Int(1))
	st.PutName("jobname", in.mustString(""))
	for _, name := range []string{
		"a4tray", "a3tray", "b5tray", "lettertray", "legaltray", "11x17tray",
		"letter", "legal", "a4", "a3", "note", "setduplexmode", "setpapertray",
		"setpageparams", "setmargins", "setjobtimeout", "setdefaulttimeouts",
		"waittimeout", "manualfeed",
	} {
		pops := 0
		switch name {
		case "setduplexmode", "setpapertray", "setjobtimeout":
			pops = 1
		case "setmargins":
			pops = 2
		case "setdefaulttimeouts":
			pops = 3
		case "setpageparams":
			pops = 4
		}
		st.PutName(name, OpObj(&Operator{Name: name, Fn: func(in *Interp) error {
			if len(in.os) < pops {
				return errUnderflow
			}
			in.os = in.os[:len(in.os)-pops]
			return nil
		}}))
	}
	st.PutName("manualfeed", Bool(false))
	st.PutName("waittimeout", Int(0))
	st.PutName("duplexmode", Bool(false))
	st.PutName("pagetype", Int(0))
}

// mustString returns a string of the setup, which never passes the limit
// of memory.
func (in *Interp) mustString(s string) Object {
	o, err := in.NewString([]byte(s))
	if err != nil {
		return Null
	}
	return o
}

// Finish ends the setup: it makes systemdict read-only. A caller defines
// its operators before it calls Finish.
func (in *Interp) Finish() {
	in.SystemDict.access = ReadOnly
}

// NewFile returns a file that reads data, such as the text of a program.
func NewFile(name string, data []byte) *File {
	return newFile(name, &memSource{data: data})
}

// FileObj returns the object of a file.
func FileObj(f *File) Object {
	return Object{typ: TFile, ref: f, exec: true}
}
