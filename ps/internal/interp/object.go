// Package interp is the language of PostScript: the scanner, the objects,
// the stacks, the virtual memory, and the operators that do not draw
// (D29). It follows the PostScript Language Reference Manual, third
// edition, which this package calls the PLRM.
//
// The package ps/internal/graphics adds the operators of the imaging model
// with Define. The interpreter bounds its stacks, its memory, the depth of
// its calls and the number of the operators that it runs, and it checks the
// context, because a file is input from a stranger (D5).
package interp

import (
	"math"
	"strconv"
)

// Type is the type of an object.
type Type uint8

// The types of the objects (PLRM 3.3).
const (
	TNull Type = iota
	TBool
	TInt
	TReal
	TName
	TString
	TArray
	TPacked
	TDict
	TOperator
	TMark
	TSave
	TFile
	TFontID
	TGState
)

// typeNames holds the names that the operator type returns.
var typeNames = [...]string{
	TNull:     "nulltype",
	TBool:     "booleantype",
	TInt:      "integertype",
	TReal:     "realtype",
	TName:     "nametype",
	TString:   "stringtype",
	TArray:    "arraytype",
	TPacked:   "packedarraytype",
	TDict:     "dicttype",
	TOperator: "operatortype",
	TMark:     "marktype",
	TSave:     "savetype",
	TFile:     "filetype",
	TFontID:   "fonttype",
	TGState:   "gstatetype",
}

// String returns the name of the type, such as integertype.
func (t Type) String() string {
	if int(t) < len(typeNames) {
		return typeNames[t]
	}
	return "unknowntype"
}

// Access is the access of a composite object (PLRM 3.3.2).
type Access uint8

// The accesses, from the most open.
const (
	Unlimited Access = iota
	ReadOnly
	ExecuteOnly
	NoAccess
)

// Object is a PostScript object. It is a value: a composite object, such as
// a string, an array or a dictionary, shares its value with its copies.
type Object struct {
	// ref holds the value of a composite object, a name, an operator, a
	// file, a save or a graphics state.
	ref any
	// i holds an integer, a boolean, or the identity of a font.
	i int64
	// r holds a real.
	r float64
	// off and n are the start and the length of a string or an array in its
	// buffer.
	off, n int32
	typ    Type
	exec   bool
	access Access
}

// Name is a name. Its value is its text.
type Name string

// strBuf is the buffer of strings.
type strBuf struct {
	b []byte
	// global is true for a string of global VM.
	global bool
}

// arrBuf is the buffer of arrays and packed arrays.
type arrBuf struct {
	a      []Object
	id     uint64
	global bool
	// saved is the level of save at which the journal holds the contents.
	saved int
}

// Operator is a built-in operator.
type Operator struct {
	Name string
	Fn   func(*Interp) error
}

// Null is the null object.
var Null = Object{typ: TNull}

// Mark is the mark object.
var Mark = Object{typ: TMark}

// Int returns an integer.
func Int(i int) Object {
	return Object{typ: TInt, i: int64(i)}
}

// Real returns a real.
func Real(r float64) Object {
	return Object{typ: TReal, r: r}
}

// Number returns an integer when r is a whole number that fits, and a real
// otherwise.
func Number(r float64) Object {
	if r == math.Trunc(r) && r >= math.MinInt32 && r <= math.MaxInt32 {
		return Int(int(r))
	}
	return Real(r)
}

// Bool returns a boolean.
func Bool(b bool) Object {
	o := Object{typ: TBool}
	if b {
		o.i = 1
	}
	return o
}

// NameObj returns a literal name.
func NameObj(s string) Object {
	return Object{typ: TName, ref: Name(s)}
}

// ExecName returns an executable name.
func ExecName(s string) Object {
	return Object{typ: TName, ref: Name(s), exec: true}
}

// OpObj returns the object of an operator.
func OpObj(op *Operator) Object {
	return Object{typ: TOperator, ref: op, exec: true}
}

// Type returns the type of the object.
func (o Object) Type() Type {
	return o.typ
}

// IsExec reports whether the object is executable.
func (o Object) IsExec() bool {
	return o.exec
}

// Exec returns the object with the executable attribute.
func (o Object) Exec() Object {
	o.exec = true
	return o
}

// Literal returns the object without the executable attribute.
func (o Object) Literal() Object {
	o.exec = false
	return o
}

// Access returns the access of the object. A dictionary keeps its access in
// itself.
func (o Object) Access() Access {
	if o.typ == TDict {
		return o.Dict().access
	}
	return o.access
}

// IsNumber reports whether the object is an integer or a real.
func (o Object) IsNumber() bool {
	return o.typ == TInt || o.typ == TReal
}

// Float returns the value of a number, and 0 for any other object.
func (o Object) Float() float64 {
	switch o.typ {
	case TInt:
		return float64(o.i)
	case TReal:
		return o.r
	}
	return 0
}

// IntVal returns the value of an integer.
func (o Object) IntVal() int {
	return int(o.i)
}

// BoolVal returns the value of a boolean.
func (o Object) BoolVal() bool {
	return o.i != 0
}

// NameVal returns the text of a name, and the empty text for any other
// object.
func (o Object) NameVal() string {
	n, _ := o.ref.(Name)
	return string(n)
}

// Bytes returns the bytes of a string. They share the memory of the string.
func (o Object) Bytes() []byte {
	if o.typ != TString {
		return nil
	}
	return o.ref.(*strBuf).b[o.off : o.off+o.n]
}

// Text returns the text of a string or a name.
func (o Object) Text() string {
	switch o.typ {
	case TString:
		return string(o.Bytes())
	case TName:
		return o.NameVal()
	}
	return ""
}

// Array returns the elements of an array or a packed array. They share the
// memory of the array: a caller that changes them must call Interp.Touch
// first.
func (o Object) Array() []Object {
	if o.typ != TArray && o.typ != TPacked {
		return nil
	}
	return o.ref.(*arrBuf).a[o.off : o.off+o.n]
}

// IsArray reports whether the object is an array or a packed array.
func (o Object) IsArray() bool {
	return o.typ == TArray || o.typ == TPacked
}

// IsProc reports whether the object is a procedure: an executable array or
// packed array.
func (o Object) IsProc() bool {
	return o.exec && o.IsArray()
}

// Len returns the length of a string, an array or a dictionary.
func (o Object) Len() int {
	switch o.typ {
	case TString, TArray, TPacked:
		return int(o.n)
	case TDict:
		return o.Dict().Len()
	}
	return 0
}

// Dict returns the dictionary of a dictionary object, or nil.
func (o Object) Dict() *Dict {
	d, _ := o.ref.(*Dict)
	return d
}

// Op returns the operator of an operator object, or nil.
func (o Object) Op() *Operator {
	op, _ := o.ref.(*Operator)
	return op
}

// File returns the file of a file object, or nil.
func (o Object) File() *File {
	f, _ := o.ref.(*File)
	return f
}

// Ref returns the value of a graphics state or of a font identity.
func (o Object) Ref() any {
	return o.ref
}

// FontID returns a font identity that holds v.
func FontID(v any) Object {
	return Object{typ: TFontID, ref: v}
}

// GState returns a graphics state object that holds v.
func GState(v any) Object {
	return Object{typ: TGState, ref: v}
}

// Same reports whether two composite objects share their value, as eq does
// for arrays and dictionaries.
func (o Object) Same(p Object) bool {
	switch o.typ {
	case TArray, TPacked:
		return p.IsArray() && o.ref == p.ref && o.off == p.off && o.n == p.n
	case TDict, TFile, TSave, TGState, TFontID, TOperator:
		return o.typ == p.typ && o.ref == p.ref
	}
	return false
}

// Equal reports whether two objects are equal, as the operator eq says:
// numbers by value, strings and names by text, and composite objects by
// identity.
func Equal(a, b Object) bool {
	switch {
	case a.IsNumber() && b.IsNumber():
		if a.typ == TInt && b.typ == TInt {
			return a.i == b.i
		}
		return a.Float() == b.Float()
	case (a.typ == TString || a.typ == TName) && (b.typ == TString || b.typ == TName):
		if a.typ == TName && b.typ == TName {
			return a.ref == b.ref
		}
		return a.Text() == b.Text()
	case a.typ != b.typ:
		return false
	}
	switch a.typ {
	case TNull, TMark:
		return true
	case TBool:
		return a.i == b.i
	}
	return a.Same(b)
}

// formatReal returns the text of a real, as cvs gives it.
func formatReal(r float64) string {
	switch {
	case math.IsInf(r, 1):
		return "inf"
	case math.IsInf(r, -1):
		return "-inf"
	case math.IsNaN(r):
		return "nan"
	}
	s := strconv.FormatFloat(r, 'g', 6, 64)
	for i := range len(s) {
		if s[i] == '.' || s[i] == 'e' {
			return s
		}
	}
	return s + ".0"
}
