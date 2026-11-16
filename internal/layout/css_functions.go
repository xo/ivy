// Copyright (c) 2011-2021, Simon Sapin and contributors.
//
// Use of this source code is governed by the BSD-3-Clause license that can be
// found in the LICENSE file.

// This file ports weasyprint/css/functions.py.

package layout

import (
	"strings"

	"github.com/xo/okage/tinycss2"
)

// function is Function in weasyprint/css/functions.py, a CSS function. See
// https://drafts.csswg.org/css-values-4/#functional-notation.
//
// The name and the arguments of a token that is not a function are the None
// of upstream: ok is false.
type function struct {
	name      string
	arguments []tinycss2.Node
	ok        bool
}

// newFunction is Function.__init__ in weasyprint/css/functions.py.
func newFunction(token tinycss2.Node) function {
	if f, ok := token.(*tinycss2.FunctionBlock); ok {
		return function{name: f.LowerName, arguments: f.Arguments, ok: true}
	}
	return function{}
}

// splitSpace is Function.split_space in weasyprint/css/functions.py. It
// splits the arguments on spaces. It returns nil for None.
func (f function) splitSpace() []tinycss2.Node {
	if !f.ok {
		return nil
	}
	return removeWhitespace(f.arguments)
}

// splitCommaParts splits the arguments on commas. Spaces and comments are
// removed.
func (f function) splitCommaParts() [][]tinycss2.Node {
	parts := [][]tinycss2.Node{{}}
	for _, token := range f.arguments {
		if tinycss2.IsLiteral(token, ",") {
			parts = append(parts, []tinycss2.Node{})
			continue
		}
		if t := token.Type(); t != tinycss2.TypeComment && t != tinycss2.TypeWhitespace {
			parts[len(parts)-1] = append(parts[len(parts)-1], token)
		}
	}
	return parts
}

// splitComma is Function.split_comma in weasyprint/css/functions.py with
// single_tokens=True. It checks that only one token is between the commas,
// and returns the tokens. With trailing, a bare comma at the end is allowed.
// It returns nil for None and for the empty list of upstream: the two
// cases differ only for a token that is not a function.
func (f function) splitComma(trailing bool) []tinycss2.Node {
	if !f.ok {
		return nil
	}
	parts := f.splitCommaParts()
	if trailing {
		for _, part := range parts[:len(parts)-1] {
			if len(part) != 1 {
				return []tinycss2.Node{}
			}
		}
		if n := len(parts[len(parts)-1]); n != 0 && n != 1 {
			return []tinycss2.Node{}
		}
		parts = parts[:len(parts)-1]
	} else {
		for _, part := range parts {
			if len(part) != 1 {
				return []tinycss2.Node{}
			}
		}
	}
	result := make([]tinycss2.Node, len(parts))
	for i, part := range parts {
		result[i] = part[0]
	}
	return result
}

// splitCommaGroups is Function.split_comma in weasyprint/css/functions.py
// with single_tokens=False. It returns the groups of tokens between the
// commas, each one not empty. With trailing, a bare comma at the end is
// allowed. It returns nil for None and an empty list when a group is empty.
func (f function) splitCommaGroups(trailing bool) [][]tinycss2.Node {
	if !f.ok {
		return nil
	}
	parts := f.splitCommaParts()
	check := parts
	if trailing {
		check = parts[:len(parts)-1]
	}
	for _, part := range check {
		if len(part) == 0 {
			return [][]tinycss2.Node{}
		}
	}
	return parts
}

// checkAttr is check_attr in weasyprint/css/functions.py. It returns
// ('attr()', (name, type_or_unit, fallback)), or nil. An allowedType of ""
// is None.
func checkAttr(token tinycss2.Node, allowedType string) Value {
	f := newFunction(token)
	if f.name != "attr" {
		return nil
	}
	parts := f.splitCommaGroups(true)
	var nameAndType []tinycss2.Node
	fallback := ""
	switch len(parts) {
	case 1:
		nameAndType = parts[0]
	case 2:
		nameAndType = parts[0]
		// TODO: support fallbacks with multiple tokens and follow type.
		if len(parts[1]) >= 1 {
			if s, ok := parts[1][0].(*tinycss2.StringToken); ok {
				fallback = s.Value
			}
		}
	default:
		return nil
	}
	if len(nameAndType) == 0 {
		// Port: upstream raises IndexError for attr(,).
		return nil
	}
	for _, t := range nameAndType {
		if t.Type() != tinycss2.TypeIdent {
			return nil
		}
	}
	// TODO: follow new syntax, see
	// https://drafts.csswg.org/css-values-5/#attr-notation.
	name := nameAndType[0].(*tinycss2.IdentToken).Value
	typeOrUnit := "string"
	if len(nameAndType) == 2 {
		typeOrUnit = nameAndType[1].(*tinycss2.IdentToken).Value
	}
	if allowedType == "" || allowedType == typeOrUnit {
		return Tuple{String("attr()"), Tuple{String(name), String(typeOrUnit), String(fallback)}}
	}
	return nil
}

// checkCounter is check_counter in weasyprint/css/functions.py. It returns
// ('counter()', (name, style)) or ('counters()', (name, separator, style)),
// or nil.
func checkCounter(token tinycss2.Node) Value {
	f := newFunction(token)
	arguments := f.splitComma(false)
	switch f.name {
	case "counter":
		if len(arguments) != 1 && len(arguments) != 2 {
			return nil
		}
	case "counters":
		if len(arguments) != 2 && len(arguments) != 3 {
			return nil
		}
	default:
		return nil
	}

	var result Tuple
	ident, ok := arguments[0].(*tinycss2.IdentToken)
	if !ok {
		return nil
	}
	arguments = arguments[1:]
	result = append(result, String(ident.Value))

	if f.name == "counters" {
		s, ok := arguments[0].(*tinycss2.StringToken)
		if !ok {
			return nil
		}
		arguments = arguments[1:]
		result = append(result, String(s.Value))
	}

	if len(arguments) > 0 {
		counterStyle, err := validateListStyleType(arguments[:1], "")
		if counterStyle == nil || err != nil {
			return nil
		}
		result = append(result, counterStyle)
	} else {
		result = append(result, String("decimal"))
	}

	return Tuple{String(f.name + "()"), result}
}

// checkContent is check_content in weasyprint/css/functions.py. It returns
// ('content()', keyword), or nil.
func checkContent(token tinycss2.Node) Value {
	f := newFunction(token)
	if f.name != "content" {
		return nil
	}
	arguments := f.splitComma(false)
	switch len(arguments) {
	case 0:
		return Tuple{String("content()"), String("text")}
	case 1:
		if ident, ok := arguments[0].(*tinycss2.IdentToken); ok {
			switch ident.LowerValue {
			case "text", "before", "after", "first-letter", "marker":
				return Tuple{String("content()"), String(ident.LowerValue)}
			}
		}
	}
	return nil
}

// checkStringOrElement is check_string_or_element in
// weasyprint/css/functions.py. stringOrElement is "string" or "element". It
// returns ('string()', (name, keyword)) or ('element()', (name, keyword)),
// or nil.
func checkStringOrElement(stringOrElement string, token tinycss2.Node) Value {
	f := newFunction(token)
	arguments := f.splitComma(false)
	if f.name != stringOrElement || (len(arguments) != 1 && len(arguments) != 2) {
		return nil
	}
	customIdent, ok := arguments[0].(*tinycss2.IdentToken)
	if !ok {
		return nil
	}
	keyword := "first"
	if len(arguments) > 1 {
		ident, ok := arguments[1].(*tinycss2.IdentToken)
		if !ok {
			return nil
		}
		switch ident.LowerValue {
		case "first", "start", "last", "first-except", "all-once":
		default:
			return nil
		}
		keyword = ident.LowerValue
	}
	return Tuple{String(stringOrElement + "()"), Tuple{String(customIdent.Value), String(keyword)}}
}

// checkVar is check_var in weasyprint/css/functions.py. It reports whether
// the token holds a var() function.
func checkVar(token tinycss2.Node) bool {
	if block, ok := token.(*tinycss2.ParenthesesBlock); ok {
		for _, item := range block.Content {
			if checkVar(item) {
				return true
			}
		}
		return false
	}
	f := newFunction(token)
	if !f.ok {
		return false
	}
	arguments := f.splitSpace()
	if f.name == "var" {
		if len(arguments) == 0 {
			// Port: upstream raises IndexError for var().
			return false
		}
		// TODO: we should check authorized tokens
		// https://drafts.csswg.org/css-syntax-3/#typedef-declaration-value
		ident, ok := arguments[0].(*tinycss2.IdentToken)
		return ok && strings.HasPrefix(ident.Value, "--")
	}
	for _, argument := range arguments {
		if checkVar(argument) {
			return true
		}
	}
	return false
}

// checkMath is check_math in weasyprint/css/functions.py for a value. It
// reports whether the value is a math function, or a tuple that holds one.
func checkMath(value Value) bool {
	switch v := value.(type) {
	case Tuple:
		for _, item := range v {
			if checkMath(item) {
				return true
			}
		}
	case Token:
		return checkMathToken(v.Node)
	}
	return false
}

// checkMathToken is check_math in weasyprint/css/functions.py for a token.
func checkMathToken(token tinycss2.Node) bool {
	// TODO: validate for real.
	if token == nil || token.Type() != tinycss2.TypeFunction {
		return false
	}
	f := newFunction(token)
	n := len(f.splitCommaGroups(false))
	switch f.name {
	case "calc":
		return n == 1
	case "min", "max":
		return n >= 1
	case "clamp":
		return n == 3
	case "round":
		return 1 <= n && n <= 3
	case "mod", "rem":
		return n == 2
	case "sin", "cos", "tan":
		return n == 1
	case "asin", "acos", "atan":
		return n == 1
	case "atan2":
		return n == 2
	case "pow":
		return n == 2
	case "sqrt":
		return n == 1
	case "hypot":
		return n >= 1
	case "log":
		return 1 <= n && n <= 2
	case "exp":
		return n == 1
	case "abs", "sign":
		return n == 1
	}
	for _, argument := range f.splitSpace() {
		if checkMathToken(argument) {
			return true
		}
	}
	return false
}
