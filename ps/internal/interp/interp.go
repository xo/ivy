package interp

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"math"

	"github.com/xo/ivy"
)

// Hooks are the functions that another package gives the interpreter.
type Hooks struct {
	// Save returns a copy of the graphics state, for save.
	Save func() any
	// Restore puts back the graphics state that Save returned, for restore.
	Restore func(state any)
	// CopyGState copies the graphics state src into dst, for copy.
	CopyGState func(dst, src any)
}

// Config is the configuration of an interpreter.
type Config struct {
	// Limits bound the work of the program.
	Limits ivy.Limits
	// Log receives the text that the program prints, and its errors.
	Log *slog.Logger
}

// frameKind is the kind of a frame of the execution stack.
type frameKind uint8

// The kinds of the frames.
const (
	// fProc runs the elements of a procedure.
	fProc frameKind = iota
	// fFile runs the tokens of a file.
	fFile
	// fObj executes one object.
	fObj
	// fFor, fRepeat, fLoop and fForall are the loops.
	fFor
	fRepeat
	fLoop
	fForall
	// fStopped is the mark of stopped.
	fStopped
	// fBarrier is the mark of a call from Go.
	fBarrier
	// fCont calls a function of Go.
	fCont
)

// frame is a frame of the execution stack.
type frame struct {
	kind frameKind
	// obj is the procedure, the file, the object, or the composite object
	// of forall.
	obj Object
	// proc is the body of a loop.
	proc Object
	// i is the index of a procedure or of forall, or the count of repeat.
	i int
	// cur, inc and lim are the control of for. isInt is true when all three
	// are integers.
	cur, inc, lim float64
	isInt         bool
	// keys are the keys of a dictionary for forall.
	keys []Object
	// fn is the function of fCont.
	fn func() error
}

// Interp is an interpreter of PostScript. It is not safe for concurrent
// use.
type Interp struct {
	ctx context.Context
	os  []Object
	ds  []*Dict
	es  []frame
	vm  vm

	// The standard dictionaries (PLRM 3.7.5).
	SystemDict  *Dict
	GlobalDict  *Dict
	UserDict    *Dict
	ErrorDict   *Dict
	DollarError *Dict
	StatusDict  *Dict
	// Resources holds a dictionary for each category of resources.
	Resources *Dict

	// Hooks are the functions of the graphics state.
	Hooks Hooks

	log *slog.Logger
	lim ivy.Limits

	ops, maxOps                     int64
	pageStart, maxPageOps           int64
	maxOStack, maxDStack, maxEStack int
	maxCall, callDepth              int
	maxString, maxArray             int
	errCommand                      Object
	packing                         bool
	seed                            uint32
	userParams, systemParams        *Dict
	objectFormat                    int
	lastJobErr                      *JobError
	printed                         int
	resourceHooks                   map[string]ResourceHooks
}

// New returns an interpreter with the standard dictionaries and the
// operators of the language.
func New(cfg Config) *Interp {
	lim := cfg.Limits.WithDefaults()
	log := cfg.Log
	if log == nil {
		log = slog.New(slog.DiscardHandler)
	}
	in := &Interp{
		ctx:        context.Background(),
		log:        log,
		lim:        lim,
		maxOps:     int64(lim.MaxObjects) * 64,
		maxPageOps: int64(lim.MaxObjects) * 48,
		maxOStack:  min(lim.MaxObjects, 1<<17),
		maxDStack:  lim.MaxDepth * 16,
		maxEStack:  lim.MaxDepth * 256,
		maxCall:    lim.MaxDepth,
		maxString:  int(min(lim.MaxMemory, math.MaxInt32)),
		maxArray:   lim.MaxObjects,
		seed:       1,
	}
	in.vm.max = lim.MaxMemory
	in.setup()
	return in
}

// NewPage starts the count of the operators of a page, which the limit of
// a page bounds.
func (in *Interp) NewPage() {
	in.pageStart = in.ops
}

// Limits returns the limits of the interpreter.
func (in *Interp) Limits() ivy.Limits {
	return in.lim
}

// Log returns the logger of the interpreter.
func (in *Interp) Log() *slog.Logger {
	return in.log
}

// Context returns the context of the current run.
func (in *Interp) Context() context.Context {
	return in.ctx
}

// newDict returns an empty dictionary.
func (in *Interp) newDict(n int) *Dict {
	return &Dict{
		max:    n,
		global: in.vm.global,
		id:     in.vm.id(),
		saved:  in.vm.level,
		vm:     &in.vm,
	}
}

// NewDict returns a dictionary object with room for n entries.
func (in *Interp) NewDict(n int) Object {
	return Object{typ: TDict, ref: in.newDict(n)}
}

// DictObj returns the object of a dictionary.
func DictObj(d *Dict) Object {
	return Object{typ: TDict, ref: d}
}

// NewString returns a string that holds b. The string takes b, which the
// caller must not change.
func (in *Interp) NewString(b []byte) (Object, error) {
	if len(b) > in.maxString {
		return Null, errLimitcheck
	}
	if err := in.vm.alloc(int64(len(b))); err != nil {
		return Null, err
	}
	if b == nil {
		b = []byte{}
	}
	return Object{typ: TString, ref: &strBuf{b: b, global: in.vm.global}, n: int32(len(b))}, nil
}

// NewArray returns an array that holds elems. The array takes elems, which
// the caller must not change.
func (in *Interp) NewArray(elems []Object) (Object, error) {
	if len(elems) > in.maxArray {
		return Null, errLimitcheck
	}
	if err := in.vm.alloc(int64(len(elems)) * 48); err != nil {
		return Null, err
	}
	b := &arrBuf{a: elems, id: in.vm.id(), global: in.vm.global, saved: in.vm.level}
	return Object{typ: TArray, ref: b, n: int32(len(elems))}, nil
}

// SetGlobal sets the allocation in global VM on or off, and returns the
// mode before the call.
func (in *Interp) SetGlobal(global bool) bool {
	old := in.vm.global
	in.vm.global = global
	return old
}

// Touch keeps the contents of an array in the journal of save before the
// caller changes them.
func (in *Interp) Touch(o Object) {
	if o.IsArray() {
		in.vm.touchArray(o.ref.(*arrBuf))
	}
}

// Define defines an operator in systemdict.
func (in *Interp) Define(name string, fn func(*Interp) error) {
	in.SystemDict.PutName(name, OpObj(&Operator{Name: name, Fn: fn}))
}

// Push pushes an object onto the operand stack.
func (in *Interp) Push(o Object) {
	in.os = append(in.os, o)
}

// PushNum pushes a real.
func (in *Interp) PushNum(v float64) {
	in.os = append(in.os, Real(v))
}

// Count returns the depth of the operand stack.
func (in *Interp) Count() int {
	return len(in.os)
}

// Peek returns the object at a depth of the operand stack: 0 is the top.
func (in *Interp) Peek(i int) (Object, error) {
	if i < 0 || i >= len(in.os) {
		return Null, errUnderflow
	}
	return in.os[len(in.os)-1-i], nil
}

// Pop pops the top object.
//
// Pop leaves the object in the memory of the stack, so that the
// interpreter puts back the operands of an operator that fails before it
// pushes.
func (in *Interp) Pop() (Object, error) {
	if len(in.os) == 0 {
		return Null, errUnderflow
	}
	o := in.os[len(in.os)-1]
	in.os = in.os[:len(in.os)-1]
	return o, nil
}

// PopNum pops a number.
func (in *Interp) PopNum() (float64, error) {
	o, err := in.Pop()
	if err != nil {
		return 0, err
	}
	if !o.IsNumber() {
		return 0, errTypecheck
	}
	return o.Float(), nil
}

// PopNums pops n numbers, and returns them in the order of the stack.
func (in *Interp) PopNums(n int) ([]float64, error) {
	if len(in.os) < n {
		return nil, errUnderflow
	}
	v := make([]float64, n)
	for i := range n {
		o := in.os[len(in.os)-n+i]
		if !o.IsNumber() {
			return nil, errTypecheck
		}
		v[i] = o.Float()
	}
	in.os = in.os[:len(in.os)-n]
	return v, nil
}

// PopInt pops an integer.
func (in *Interp) PopInt() (int, error) {
	o, err := in.Pop()
	if err != nil {
		return 0, err
	}
	if o.typ != TInt {
		if o.typ == TReal && o.r == math.Trunc(o.r) && math.Abs(o.r) < 1<<31 {
			return int(o.r), nil
		}
		return 0, errTypecheck
	}
	return int(o.i), nil
}

// PopBool pops a boolean.
func (in *Interp) PopBool() (bool, error) {
	o, err := in.Pop()
	if err != nil {
		return false, err
	}
	if o.typ != TBool {
		return false, errTypecheck
	}
	return o.i != 0, nil
}

// PopType pops an object of the type t.
func (in *Interp) PopType(t Type) (Object, error) {
	o, err := in.Pop()
	if err != nil {
		return Null, err
	}
	if o.typ != t && (t != TArray || o.typ != TPacked) {
		return Null, errTypecheck
	}
	return o, nil
}

// PopDict pops a dictionary.
func (in *Interp) PopDict() (*Dict, error) {
	o, err := in.PopType(TDict)
	if err != nil {
		return nil, err
	}
	return o.Dict(), nil
}

// PopProc pops a procedure.
func (in *Interp) PopProc() (Object, error) {
	o, err := in.Pop()
	if err != nil {
		return Null, err
	}
	if !o.IsArray() {
		return Null, errTypecheck
	}
	return o, nil
}

// Lookup returns the value of a name in the dictionary stack.
func (in *Interp) Lookup(name string) (Object, bool) {
	for i := len(in.ds) - 1; i >= 0; i-- {
		d := in.ds[i]
		if j, ok := d.names[name]; ok {
			return d.entries[j].v, true
		}
	}
	return Null, false
}

// lookupKey returns the value of a key in the dictionary stack, and the
// dictionary that holds it.
func (in *Interp) lookupKey(k Object) (Object, *Dict, bool) {
	kk, ok := keyOf(k)
	if !ok {
		return Null, nil, false
	}
	for i := len(in.ds) - 1; i >= 0; i-- {
		d := in.ds[i]
		if j, ok := d.index(kk); ok {
			return d.entries[j].v, d, true
		}
	}
	return Null, nil, false
}

// CurrentDict returns the dictionary on the top of the dictionary stack.
func (in *Interp) CurrentDict() *Dict {
	return in.ds[len(in.ds)-1]
}

// Begin pushes a dictionary onto the dictionary stack.
func (in *Interp) Begin(d *Dict) error {
	if len(in.ds) >= in.maxDStack {
		return ErrDictStackOverflow
	}
	in.ds = append(in.ds, d)
	return nil
}

// End pops the dictionary stack.
func (in *Interp) End() error {
	if len(in.ds) <= 3 {
		return ErrDictStackUnder
	}
	in.ds = in.ds[:len(in.ds)-1]
	return nil
}

// pushFrame pushes a frame onto the execution stack.
func (in *Interp) pushFrame(f frame) error {
	if len(in.es) >= in.maxEStack {
		return errExecOverflow
	}
	in.es = append(in.es, f)
	return nil
}

// popFrame pops the execution stack.
func (in *Interp) popFrame() {
	in.es[len(in.es)-1] = frame{}
	in.es = in.es[:len(in.es)-1]
}

// pushProc pushes a procedure to run.
func (in *Interp) pushProc(o Object) error {
	if o.n == 0 {
		return nil
	}
	return in.pushFrame(frame{kind: fProc, obj: o})
}

// Run runs a program from its text. It returns nil when the program ends,
// a *JobError when an error of PostScript ends it, and an error that wraps
// ivy.ErrLimit or the error of the context when it ends early.
func (in *Interp) Run(ctx context.Context, f *File) (err error) {
	in.ctx = ctx
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("postscript: panic: %v: %w", r, ivy.ErrFormat)
		}
	}()
	in.es = in.es[:0]
	if err := in.pushFrame(frame{kind: fFile, obj: Object{typ: TFile, ref: f, exec: true}}); err != nil {
		return err
	}
	err = in.run(0)
	switch {
	case err == nil, errors.Is(err, errQuit):
		return nil
	case errors.Is(err, errStop), errors.Is(err, errExit):
		return in.jobError()
	}
	return err
}

// jobError returns the error that $error holds.
func (in *Interp) jobError() error {
	name, _ := in.DollarError.GetName("errorname")
	cmd, _ := in.DollarError.GetName("command")
	e := &JobError{Name: name.Text(), Command: in.cvs(cmd)}
	if e.Name == "" {
		e.Name = "stop"
	}
	in.log.Debug("postscript: error", "error", e.Name, "command", e.Command)
	in.lastJobErr = e
	return e
}

// Call runs an object, such as a procedure, from Go, and returns when it
// ends. The calls nest up to the limit of depth.
func (in *Interp) Call(o Object) error {
	if in.callDepth >= in.maxCall {
		return limitError("depth of calls", int64(in.maxCall))
	}
	in.callDepth++
	defer func() { in.callDepth-- }()
	base := len(in.es)
	if err := in.pushFrame(frame{kind: fBarrier}); err != nil {
		return err
	}
	if err := in.pushFrame(frame{kind: fObj, obj: o}); err != nil {
		in.es = in.es[:base]
		return err
	}
	err := in.run(base + 1)
	for len(in.es) > base {
		in.popFrame()
	}
	return err
}

// check counts the operators that the program ran, and checks the
// context.
func (in *Interp) check() error {
	if in.ops > in.maxOps {
		return limitError("operators", in.maxOps)
	}
	if in.ops-in.pageStart > in.maxPageOps {
		return limitError("operators of a page", in.maxPageOps)
	}
	if err := in.ctx.Err(); err != nil {
		return fmt.Errorf("running postscript: %w", err)
	}
	return nil
}

// run runs the execution stack down to base.
func (in *Interp) run(base int) error {
	for len(in.es) > base {
		in.ops++
		if in.ops&1023 == 0 {
			if err := in.check(); err != nil {
				return err
			}
		}
		err := in.step()
		if err == nil && len(in.os) > in.maxOStack {
			in.os = in.os[:in.maxOStack-64]
			err = ErrStackOverflow
		}
		if err != nil {
			if err = in.handle(err, base); err != nil {
				return err
			}
		}
	}
	return nil
}

// step runs one step of the frame on the top of the execution stack.
func (in *Interp) step() error {
	f := &in.es[len(in.es)-1]
	switch f.kind {
	case fProc:
		if f.i >= int(f.obj.n) {
			in.popFrame()
			return nil
		}
		o := f.obj.ref.(*arrBuf).a[int(f.obj.off)+f.i]
		f.i++
		if f.i >= int(f.obj.n) {
			// a tail call does not grow the stack
			in.popFrame()
		}
		return in.execDirect(o)
	case fFile:
		file := f.obj.File()
		o, ok, err := in.token(file, 0)
		if err != nil {
			if fatal(err) {
				return err
			}
			if in.errCommand.typ == TNull {
				in.errCommand = f.obj
			}
			return err
		}
		if !ok {
			in.popFrame()
			return nil
		}
		return in.execDirect(o)
	case fObj:
		o := f.obj
		in.popFrame()
		return in.execObj(o)
	case fFor:
		if f.isInt {
			if f.inc > 0 && f.cur > f.lim || f.inc < 0 && f.cur < f.lim {
				in.popFrame()
				return nil
			}
			in.Push(Int(int(f.cur)))
		} else {
			if f.inc > 0 && f.cur > f.lim || f.inc < 0 && f.cur < f.lim {
				in.popFrame()
				return nil
			}
			in.Push(Real(f.cur))
		}
		f.cur += f.inc
		return in.pushProc(f.proc)
	case fRepeat:
		if f.i <= 0 {
			in.popFrame()
			return nil
		}
		f.i--
		return in.pushProc(f.proc)
	case fLoop:
		return in.pushProc(f.proc)
	case fForall:
		return in.stepForall(f)
	case fStopped:
		in.popFrame()
		in.Push(Bool(false))
		return nil
	case fBarrier:
		return nil
	case fCont:
		fn := f.fn
		in.popFrame()
		return fn()
	}
	in.popFrame()
	return nil
}

// stepForall runs one step of forall.
func (in *Interp) stepForall(f *frame) error {
	o := f.obj
	switch o.typ {
	case TArray, TPacked:
		if f.i >= int(o.n) {
			in.popFrame()
			return nil
		}
		in.Push(o.ref.(*arrBuf).a[int(o.off)+f.i])
	case TString:
		if f.i >= int(o.n) {
			in.popFrame()
			return nil
		}
		in.Push(Int(int(o.Bytes()[f.i])))
	case TDict:
		d := o.Dict()
		for {
			if f.i >= len(f.keys) {
				in.popFrame()
				return nil
			}
			k := f.keys[f.i]
			if v, ok := d.Get(k); ok {
				in.Push(k)
				in.Push(v)
				break
			}
			f.i++
		}
	default:
		in.popFrame()
		return nil
	}
	f.i++
	return in.pushProc(f.proc)
}

// execDirect executes an object that a procedure or a file holds: a
// procedure is pushed, not run (PLRM 3.5.3).
func (in *Interp) execDirect(o Object) error {
	if !o.exec || o.typ == TArray || o.typ == TPacked {
		in.os = append(in.os, o)
		return nil
	}
	return in.execObj(o)
}

// execObj executes an object, as exec does.
func (in *Interp) execObj(o Object) error {
	if !o.exec {
		in.os = append(in.os, o)
		return nil
	}
	switch o.typ {
	case TName:
		v, ok := in.Lookup(o.NameVal())
		if !ok {
			in.errCommand = o
			return errUndefined
		}
		return in.execValue(v, 0)
	case TOperator:
		return in.callOp(o)
	case TArray, TPacked:
		return in.pushProc(o)
	case TString:
		f := newFile("string", &memSource{data: o.Bytes()})
		return in.pushFrame(frame{kind: fFile, obj: Object{typ: TFile, ref: f, exec: true}})
	case TFile:
		return in.pushFrame(frame{kind: fFile, obj: o})
	case TNull:
		return nil
	}
	in.os = append(in.os, o)
	return nil
}

// execValue executes the value of a name.
func (in *Interp) execValue(v Object, depth int) error {
	if !v.exec {
		in.os = append(in.os, v)
		return nil
	}
	switch v.typ {
	case TOperator:
		return in.callOp(v)
	case TArray, TPacked:
		return in.pushProc(v)
	case TName:
		if depth > 32 {
			return errLimitcheck
		}
		w, ok := in.Lookup(v.NameVal())
		if !ok {
			in.errCommand = v
			return errUndefined
		}
		return in.execValue(w, depth+1)
	}
	return in.execObj(v)
}

// callOp calls an operator. When it fails, the operand stack gets back the
// operands that the operator popped.
func (in *Interp) callOp(o Object) error {
	n := len(in.os)
	op := o.ref.(*Operator)
	err := op.Fn(in)
	if err != nil {
		var pe *Error
		if errors.As(err, &pe) {
			if len(in.os) < n && n <= cap(in.os) {
				in.os = in.os[:n]
			}
			if in.errCommand.typ == TNull {
				in.errCommand = o
			}
		}
	}
	return err
}

// handle handles an error or a signal that a step returned. It returns an
// error that ends the run down to base: a signal that goes past base, or a
// fatal error.
func (in *Interp) handle(err error, base int) error {
	for range 64 {
		switch {
		case err == nil:
			return nil
		case errors.Is(err, errStop):
			return in.unwindStop(base)
		case errors.Is(err, errExit):
			err = in.unwindExit(base)
			if errors.Is(err, errExit) {
				return err
			}
			continue
		case fatal(err):
			return err
		}
		var pe *Error
		if !errors.As(err, &pe) {
			return err
		}
		err = in.signal(pe)
	}
	return limitError("nested errors", 64)
}

// signal runs the handler of an error in errordict (PLRM 3.11). It returns
// the signal of stop of a handler of Go.
func (in *Interp) signal(pe *Error) error {
	cmd := in.errCommand
	in.errCommand = Null
	in.Push(cmd)
	h, ok := in.ErrorDict.GetName(pe.Name)
	if !ok {
		_, _ = in.Pop()
		in.recordError(pe.Name, cmd)
		return errStop
	}
	if h.typ == TOperator {
		return h.Op().Fn(in)
	}
	return in.execValue(h, 0)
}

// unwindStop pops the execution stack down to the innermost stopped, which
// pushes true. Without one above base, it returns the signal of stop.
func (in *Interp) unwindStop(base int) error {
	for len(in.es) > base {
		k := in.es[len(in.es)-1].kind
		in.popFrame()
		if k == fStopped {
			in.Push(Bool(true))
			return nil
		}
	}
	return errStop
}

// unwindExit pops the execution stack down to the innermost loop. Without
// one above base, it returns the signal of exit, or invalidexit at the top.
func (in *Interp) unwindExit(base int) error {
	for i := len(in.es) - 1; i >= base; i-- {
		switch in.es[i].kind {
		case fFor, fRepeat, fLoop, fForall:
			for len(in.es) > i {
				in.popFrame()
			}
			return nil
		case fStopped:
			return errInvalidExit
		}
	}
	if base > 0 {
		for len(in.es) > base {
			in.popFrame()
		}
		return errExit
	}
	return errInvalidExit
}

// recordError fills $error for an error (PLRM 3.11.1).
func (in *Interp) recordError(name string, cmd Object) {
	e := in.DollarError
	e.PutName("newerror", Bool(true))
	e.PutName("errorname", NameObj(name))
	e.PutName("command", cmd)
	e.PutName("recordstacks", Bool(false))
	if in.lastJobErr == nil {
		in.log.Debug("postscript: error", "error", name, "command", in.cvs(cmd))
	}
}

// Stop returns the signal of stop, which a Go function returns to unwind to
// the innermost stopped.
func Stop() error {
	return errStop
}

// Quit returns the signal of quit, which ends the program without an
// error, as the operator quit does.
func Quit() error {
	return errQuit
}

// IsExit reports whether err is the signal of exit, which ends a loop that a
// Go function runs.
func IsExit(err error) bool {
	return errors.Is(err, errExit)
}

// IsSignal reports whether err is a signal of stop, exit or quit, which a
// Go function must pass on.
func IsSignal(err error) bool {
	return errors.Is(err, errStop) || errors.Is(err, errExit) || errors.Is(err, errQuit)
}
