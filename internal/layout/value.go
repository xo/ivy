// Copyright (c) 2011-2021, Simon Sapin and contributors.
//
// Use of this source code is governed by the BSD-3-Clause license that can be
// found in the LICENSE file.

// This file holds the Go types of the values that WeasyPrint keeps in its
// style dictionaries as Python objects. It ports no file of upstream.

package layout

import (
	"fmt"
	"slices"
	"strings"

	"github.com/xo/okage/tinycss2"
)

// Value is a value of a CSS property: a specified, a cascaded or a computed
// value. WeasyPrint holds these values as Python objects of a few types. Each
// type of Go below stands for one of them:
//
//   - String is a str: a keyword such as "auto", or a free string.
//   - Int is an int, and Float is a float. Equal compares them as numbers,
//     as Python does.
//   - Bool is a bool.
//   - Dimension is the namedtuple Dimension of css/properties.py. A unit of
//     None is the empty string.
//   - Tuple is a tuple or a list.
//   - StringSet is a set of str, such as the value of text-decoration-line.
//   - Token is a token of tinycss2 that a validator keeps as it is, such as a
//     color that the computed value parses, or a math function that waits
//     for the computed style.
//   - Tokens is a list of tokens, the value of a custom property.
//   - Color is a Color of tinycss2.
//   - *Pending is a Pending of css/tokens.py: a value with var() that waits
//     for the computed style.
//   - *LinearGradient and *RadialGradient are the gradients of images.py.
//
// The None of Python is a nil Value. Only the types of this package
// implement Value.
type Value interface {
	isValue()
}

// String is a str of Python: a keyword or a string.
type String string

// Int is an int of Python.
type Int int

// Float is a float of Python.
type Float float64

// Bool is a bool of Python.
type Bool bool

// Dimension is Dimension in weasyprint css/properties.py: a number and its
// unit. A Unit of "" is the None of upstream, as in Dimension(0, None).
type Dimension struct {
	Value float64
	Unit  string
}

// Tuple is a tuple or a list of Python.
type Tuple []Value

// StringSet is a set of str of Python. The strings are sorted and unique.
type StringSet []string

// Token is a token of tinycss2 that is kept as a value.
type Token struct {
	Node tinycss2.Node
}

// Tokens is a list of tokens of tinycss2 that is kept as a value, such as
// the value of a custom property.
type Tokens []tinycss2.Node

// Color is a color of tinycss2, as parse_color of tinycss2 color5.py returns
// it.
type Color struct {
	*tinycss2.Color
}

func (String) isValue()    {}
func (Int) isValue()       {}
func (Float) isValue()     {}
func (Bool) isValue()      {}
func (Dimension) isValue() {}
func (Tuple) isValue()     {}
func (StringSet) isValue() {}
func (Token) isValue()     {}
func (Tokens) isValue()    {}
func (Color) isValue()     {}

// ZeroPixels is ZERO_PIXELS in weasyprint css/properties.py.
var ZeroPixels = Dimension{0, "px"}

// newStringSet makes a StringSet of strings, sorted and unique.
func newStringSet(strs ...string) StringSet {
	set := slices.Clone(strs)
	slices.Sort(set)
	return StringSet(slices.Compact(set))
}

// union is the operator | of two sets of Python.
func (s StringSet) union(other StringSet) StringSet {
	return newStringSet(append(slices.Clone(s), other...)...)
}

// Has reports whether the set holds str.
func (s StringSet) Has(str string) bool {
	_, ok := slices.BinarySearch(s, str)
	return ok
}

// isStr reports whether v is the string s, as the expression v == s of
// Python does.
func isStr(v Value, s string) bool {
	str, ok := v.(String)
	return ok && string(str) == s
}

// isStrIn reports whether v is one of the strings, as the expression
// v in (a, b) of Python does.
func isStrIn(v Value, strs ...string) bool {
	str, ok := v.(String)
	return ok && slices.Contains(strs, string(str))
}

// str returns the string of v, or "" when v is not a String.
func str(v Value) string {
	s, _ := v.(String)
	return string(s)
}

// toFloat returns the number of an Int, a Float or a Bool, and false for
// any other value.
func toFloat(v Value) (float64, bool) {
	switch v := v.(type) {
	case Int:
		return float64(v), true
	case Float:
		return float64(v), true
	case Bool:
		if v {
			return 1, true
		}
		return 0, true
	}
	return 0, false
}

// num returns the number of an Int, a Float or a Bool, or 0.
func num(v Value) float64 {
	f, _ := toFloat(v)
	return f
}

// isNumber reports whether v is an Int or a Float, as isinstance(v, (int,
// float)) of Python does.
func isNumber(v Value) bool {
	switch v.(type) {
	case Int, Float:
		return true
	}
	return false
}

// tupleOf makes a Tuple of strings.
func tupleOf(strs ...string) Tuple {
	t := make(Tuple, len(strs))
	for i, s := range strs {
		t[i] = String(s)
	}
	return t
}

// tupleHas reports whether the tuple v holds the string s, as the expression
// s in v of Python does.
func tupleHas(v Value, s string) bool {
	t, ok := v.(Tuple)
	if !ok {
		return false
	}
	for _, item := range t {
		if isStr(item, s) {
			return true
		}
	}
	return false
}

// Equal reports whether two values are equal, as the operator == of Python
// does for them. Numbers compare by value whatever their type. Colors compare
// with Color.Equal and their alpha channels.
func Equal(a, b Value) bool {
	if fa, ok := toFloat(a); ok {
		fb, ok := toFloat(b)
		return ok && fa == fb
	}
	switch a := a.(type) {
	case nil:
		return b == nil
	case String:
		return isStr(b, string(a))
	case Dimension:
		bd, ok := b.(Dimension)
		return ok && a == bd
	case Tuple:
		bt, ok := b.(Tuple)
		if !ok || len(a) != len(bt) {
			return false
		}
		for i := range a {
			if !Equal(a[i], bt[i]) {
				return false
			}
		}
		return true
	case StringSet:
		bs, ok := b.(StringSet)
		return ok && slices.Equal(a, bs)
	case Token:
		bt, ok := b.(Token)
		return ok && tokenEqual(a.Node, bt.Node)
	case Tokens:
		bt, ok := b.(Tokens)
		if !ok || len(a) != len(bt) {
			return false
		}
		for i := range a {
			if !tokenEqual(a[i], bt[i]) {
				return false
			}
		}
		return true
	case Color:
		bc, ok := b.(Color)
		if !ok {
			return false
		}
		if a.Color == nil || bc.Color == nil {
			return a.Color == bc.Color
		}
		return a.Equal(bc.Color) && a.Alpha == bc.Alpha
	case *Pending:
		return a == b
	case *LinearGradient:
		return a == b
	case *RadialGradient:
		return a == b
	}
	return false
}

// tokenEqual reports whether two tokens are equal. It compares their types
// and their serializations.
func tokenEqual(a, b tinycss2.Node) bool {
	if a == nil || b == nil {
		return a == b
	}
	return a.Type() == b.Type() && a.Serialize() == b.Serialize()
}

// ValueString returns the value as Python would print it, for the messages
// of the log and the tests.
func ValueString(v Value) string {
	switch v := v.(type) {
	case nil:
		return "None"
	case String:
		return fmt.Sprintf("%q", string(v))
	case Int:
		return fmt.Sprint(int(v))
	case Float:
		return pyStr(float64(v))
	case Bool:
		if v {
			return "True"
		}
		return "False"
	case Dimension:
		unit := "None"
		if v.Unit != "" {
			unit = fmt.Sprintf("%q", v.Unit)
		}
		return fmt.Sprintf("Dimension(%s, %s)", pyStr(v.Value), unit)
	case Tuple:
		parts := make([]string, len(v))
		for i, item := range v {
			parts[i] = ValueString(item)
		}
		if len(parts) == 1 {
			return "(" + parts[0] + ",)"
		}
		return "(" + strings.Join(parts, ", ") + ")"
	case StringSet:
		parts := make([]string, len(v))
		for i, item := range v {
			parts[i] = fmt.Sprintf("%q", item)
		}
		return "{" + strings.Join(parts, ", ") + "}"
	case Token:
		if v.Node == nil {
			return "None"
		}
		return v.Node.String()
	case Tokens:
		parts := make([]string, len(v))
		for i, item := range v {
			parts[i] = item.String()
		}
		return "[" + strings.Join(parts, ", ") + "]"
	case Color:
		if v.Color == nil {
			return "None"
		}
		return v.String()
	case *Pending:
		return fmt.Sprintf("<Pending %s>", v.Name)
	case *LinearGradient:
		return "<LinearGradient>"
	case *RadialGradient:
		return "<RadialGradient>"
	}
	return fmt.Sprintf("%v", v)
}
