// Copyright (c) 2011-2021, Simon Sapin and contributors.
//
// Use of this source code is governed by the BSD-3-Clause license that can be
// found in the LICENSE file.

// This file ports weasyprint/css/counters.py, the counter styles of CSS
// Counter Styles Level 3:
// https://www.w3.org/TR/css-counter-styles-3/#counter-style-system

package layout

import (
	"maps"
	"math"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/xo/okage/tinycss2"
)

// maxCounterRepresentation is the largest size in bytes of the text of a
// counter.
//
// Port: upstream has no limit, and a counter of a symbolic or an additive
// style, or with a large pad, can make a text of any size. A text past the
// limit falls back to the decimal style, and a pad past the limit is cut.
const maxCounterRepresentation = 1 << 12

// maxCounterFallbacks bounds the fallbacks of a counter style.
//
// Port: upstream falls back to "decimal" with no limit, which recurses
// without end when "decimal" itself falls back.
const maxCounterFallbacks = 32

// symbol is symbol in weasyprint/css/counters.py. It returns the string of
// a symbol, ('string', value), or "" for any other symbol.
func symbol(stringOrURL Value) string {
	// TODO: this function should handle images too, and return something
	// else than strings.
	t, ok := stringOrURL.(Tuple)
	if ok && len(t) == 2 && isStr(t[0], "string") {
		return str(t[1])
	}
	return ""
}

// parseCounterStyleName is parse_counter_style_name in
// weasyprint/css/counters.py. It returns the name of a counter style, or
// "" for None.
func parseCounterStyleName(tokens []tinycss2.Node, counterStyle CounterStyle) string {
	tokens = removeWhitespace(tokens)
	if len(tokens) == 1 {
		if ident, ok := tokens[0].(*tinycss2.IdentToken); ok {
			switch ident.LowerValue {
			case "decimal", "disc":
				if _, ok := counterStyle[ident.LowerValue]; !ok {
					return ident.Value
				}
			case "none":
			default:
				return ident.Value
			}
		}
	}
	return ""
}

// CounterStyleRule holds the descriptors of an @counter-style rule, by name
// in snake case: "system", "negative", "prefix", "suffix", "range", "pad",
// "fallback", "symbols" and "additive_symbols". A descriptor that the rule
// does not set is nil.
type CounterStyleRule map[string]Value

// CounterStyle is CounterStyle in weasyprint/css/counters.py: the counter
// styles of the @counter-style rules, by name. See
// https://www.w3.org/TR/css-counter-styles-3/.
type CounterStyle map[string]CounterStyleRule

// counterSystem returns the extends, the system and the fixed number of the
// descriptor system of a counter.
func counterSystem(counter CounterStyleRule) (extends Value, system string, fixedNumber Value) {
	if t, ok := counter["system"].(Tuple); ok && len(t) == 3 {
		return t[0], str(t[1]), t[2]
	}
	return nil, "symbolic", nil
}

// truthy reports whether a value is true for Python.
func truthy(v Value) bool {
	switch v := v.(type) {
	case nil:
		return false
	case String:
		return v != ""
	case Tuple:
		return len(v) > 0
	case Tokens:
		return len(v) > 0
	case StringSet:
		return len(v) > 0
	case Dimension:
		return true
	}
	if f, ok := toFloat(v); ok {
		return f != 0
	}
	return true
}

// containsValue reports whether a list holds a value.
func containsValue(list []Value, v Value) bool {
	return slices.ContainsFunc(list, func(item Value) bool { return Equal(item, v) })
}

// resolveCounter is CounterStyle.resolve_counter in
// weasyprint/css/counters.py. counterName is the name of a style, or a tuple
// ('symbols()', arguments) or ('string', value). It returns nil for None.
func (cs CounterStyle) resolveCounter(counterName Value, previousTypes *[]Value) CounterStyleRule {
	if t, ok := counterName.(Tuple); ok && len(t) == 2 && isStrIn(t[0], "symbols()", "string") {
		counterType, arguments := str(t[0]), t[1]
		var system, symbols, suffix Value
		if counterType == "string" {
			system = Tuple{nil, String("cyclic"), nil}
			symbols = Tuple{Tuple{String("string"), arguments}}
			suffix = Tuple{String("string"), String("")}
		} else {
			args, _ := arguments.(Tuple)
			if len(args) == 0 {
				return nil
			}
			var fixed Value
			if isStr(args[0], "fixed") {
				fixed = Int(1)
			}
			system = Tuple{nil, args[0], fixed}
			syms := make(Tuple, 0, len(args)-1)
			for _, argument := range args[1:] {
				syms = append(syms, Tuple{String("string"), argument})
			}
			symbols = syms
			suffix = Tuple{String("string"), String(" ")}
		}
		return CounterStyleRule{
			"system":           system,
			"negative":         Tuple{Tuple{String("string"), String("-")}, Tuple{String("string"), String("")}},
			"prefix":           Tuple{String("string"), String("")},
			"suffix":           suffix,
			"range":            String("auto"),
			"pad":              Tuple{Int(0), String("")},
			"fallback":         String("decimal"),
			"symbols":          symbols,
			"additive_symbols": Tuple{},
		}
	}
	name, ok := counterName.(String)
	if !ok {
		return nil
	}
	rule, ok := cs[string(name)]
	if !ok {
		return nil
	}
	// Avoid circular fallbacks.
	if previousTypes == nil {
		previousTypes = &[]Value{}
	} else if containsValue(*previousTypes, counterName) {
		return nil
	}
	*previousTypes = append(*previousTypes, counterName)

	counter := maps.Clone(rule)
	extends, system, _ := counterSystem(counter)

	// Handle extends.
	for truthy(extends) {
		extendedCounter, ok := cs[system]
		if !ok {
			return counter
		}
		counter["system"] = extendedCounter["system"]
		*previousTypes = append(*previousTypes, String(system))
		extends, system, _ = counterSystem(counter)
		if truthy(extends) && containsValue(*previousTypes, String(system)) {
			extends, system = String("extends"), "decimal"
			continue
		}
		for name, value := range extendedCounter {
			if counter[name] == nil && value != nil {
				counter[name] = value
			}
		}
	}
	return counter
}

// RenderValue is CounterStyle.render_value in weasyprint/css/counters.py. It
// returns the representation of a counter value in a counter style. See
// https://www.w3.org/TR/css-counter-styles-3/#generate-a-counter.
func (cs CounterStyle) RenderValue(counterValue int, counterName Value) string {
	return cs.renderValue(counterValue, counterName, nil, nil, 0)
}

// renderValue is CounterStyle.render_value in weasyprint/css/counters.py,
// with the depth of the fallbacks.
func (cs CounterStyle) renderValue(counterValue int, counterName Value, counter CounterStyleRule, previousTypes *[]Value, depth int) string {
	if depth > maxCounterFallbacks {
		return ""
	}
	decimal := func() string {
		return cs.renderValue(counterValue, String("decimal"), nil, nil, depth+1)
	}
	if counter == nil {
		counter = cs.resolveCounter(counterName, previousTypes)
	}
	if counter == nil {
		if _, ok := cs["decimal"]; ok {
			return decimal()
		}
		// Could happen if the UA stylesheet is not used.
		return ""
	}

	extends, system, fixedNumber := counterSystem(counter)

	// Avoid circular fallbacks.
	if previousTypes == nil {
		previousTypes = &[]Value{}
	} else if containsValue(*previousTypes, String(system)) {
		return decimal()
	}
	*previousTypes = append(*previousTypes, counterName)

	// Handle extends.
	for truthy(extends) {
		extendedCounter, ok := cs[system]
		if !ok {
			return decimal()
		}
		counter["system"] = extendedCounter["system"]
		extends, system, fixedNumber = counterSystem(counter)
		if containsValue(*previousTypes, String(system)) {
			return decimal()
		}
		*previousTypes = append(*previousTypes, String(system))
		for name, value := range extendedCounter {
			if counter[name] == nil && value != nil {
				counter[name] = value
			}
		}
	}

	fallback := func() string {
		name := counter["fallback"]
		if !truthy(name) {
			name = String("decimal")
		}
		return cs.renderValue(counterValue, name, nil, previousTypes, depth+1)
	}

	// Step 2.
	type counterRange struct{ min, max float64 }
	var counterRanges []counterRange
	if r := counter["range"]; r == nil || isStr(r, "auto") {
		minRange, maxRange := math.Inf(-1), math.Inf(1)
		switch system {
		case "alphabetic", "symbolic":
			minRange = 1
		case "additive":
			minRange = 0
		}
		counterRanges = []counterRange{{minRange, maxRange}}
	} else if ranges, ok := r.(Tuple); ok {
		for _, item := range ranges {
			if pair, ok := item.(Tuple); ok && len(pair) == 2 {
				counterRanges = append(counterRanges, counterRange{num(pair[0]), num(pair[1])})
			}
		}
	}
	inRange := false
	for _, r := range counterRanges {
		if r.min <= float64(counterValue) && float64(counterValue) <= r.max {
			inRange = true
			break
		}
	}
	if !inRange {
		return fallback()
	}

	// Step 3.
	var initial string
	hasInitial := false
	isNegative := counterValue < 0
	var negativePrefix, negativeSuffix string
	useNegative := false
	if isNegative {
		negative, ok := counter["negative"].(Tuple)
		if !ok || len(negative) != 2 {
			negative = Tuple{Tuple{String("string"), String("-")}, Tuple{String("string"), String("")}}
		}
		negativePrefix, negativeSuffix = symbol(negative[0]), symbol(negative[1])
		useNegative = system == "symbolic" || system == "alphabetic" || system == "numeric" || system == "additive"
		if useNegative {
			counterValue = -counterValue
		}
	}

	// TODO: instead of using the decimal fallback when we have the wrong
	// number of symbols, we should discard the whole counter. The problem
	// only happens when extending from another style, it is easily refused
	// during validation otherwise.

	symbols, _ := counter["symbols"].(Tuple)
	length := len(symbols)
	switch system {
	case "cyclic":
		if length < 1 {
			return decimal()
		}
		index := pyMod(counterValue-1, length)
		initial, hasInitial = symbol(symbols[index]), true

	case "fixed":
		if length < 1 {
			return decimal()
		}
		index := counterValue - int(num(fixedNumber))
		if 0 <= index && index < length {
			initial, hasInitial = symbol(symbols[index]), true
		} else {
			return fallback()
		}

	case "symbolic":
		if length < 1 {
			return decimal()
		}
		index := pyMod(counterValue-1, length)
		repeat := pyDiv(counterValue-1, length) + 1
		s := symbol(symbols[index])
		if repeat > 0 && len(s)*repeat > maxCounterRepresentation {
			return decimal()
		}
		initial, hasInitial = strings.Repeat(s, max(0, repeat)), true

	case "alphabetic":
		if length < 2 {
			return decimal()
		}
		var reversedParts []string
		for counterValue != 0 {
			counterValue--
			reversedParts = append(reversedParts, symbol(symbols[pyMod(counterValue, length)]))
			counterValue = pyDiv(counterValue, length)
		}
		slices.Reverse(reversedParts)
		initial, hasInitial = strings.Join(reversedParts, ""), true

	case "numeric":
		if counterValue == 0 {
			if length < 1 {
				// Port: upstream raises IndexError.
				return ""
			}
			initial, hasInitial = symbol(symbols[0]), true
		} else {
			if length < 2 {
				return decimal()
			}
			var reversedParts []string
			counterValue = abs(counterValue)
			for counterValue != 0 {
				reversedParts = append(reversedParts, symbol(symbols[pyMod(counterValue, length)]))
				counterValue = pyDiv(counterValue, length)
			}
			slices.Reverse(reversedParts)
			initial, hasInitial = strings.Join(reversedParts, ""), true
		}

	case "additive":
		additiveSymbols, _ := counter["additive_symbols"].(Tuple)
		if counterValue == 0 {
			for _, item := range additiveSymbols {
				if pair, ok := item.(Tuple); ok && len(pair) == 2 && num(pair[0]) == 0 {
					initial, hasInitial = symbol(pair[1]), true
				}
			}
		} else {
			if len(additiveSymbols) < 1 {
				return decimal()
			}
			var parts strings.Builder
			for _, item := range additiveSymbols {
				pair, ok := item.(Tuple)
				if !ok || len(pair) != 2 {
					continue
				}
				weight := int(num(pair[0]))
				if weight <= 0 {
					// Port: upstream divides by zero for a weight of 0.
					continue
				}
				repetitions := pyDiv(counterValue, weight)
				s := symbol(pair[1])
				if parts.Len()+len(s)*repetitions > maxCounterRepresentation {
					return decimal()
				}
				for range repetitions {
					parts.WriteString(s)
				}
				counterValue -= weight * repetitions
				if counterValue == 0 {
					initial, hasInitial = parts.String(), true
					break
				}
			}
		}
		if !hasInitial {
			return fallback()
		}
	}

	if !hasInitial {
		// Port: upstream asserts that the system is known.
		return decimal()
	}

	// Step 4.
	padCount, padSymbol := 0, ""
	if pad, ok := counter["pad"].(Tuple); ok && len(pad) == 2 {
		padCount, padSymbol = int(num(pad[0])), symbol(pad[1])
	}
	padDifference := padCount - utf8.RuneCountInString(initial)
	if isNegative && useNegative {
		padDifference -= utf8.RuneCountInString(negativePrefix) + utf8.RuneCountInString(negativeSuffix)
	}
	if padDifference > 0 {
		if len(padSymbol) > 0 {
			padDifference = min(padDifference, maxCounterRepresentation/len(padSymbol))
		}
		initial = strings.Repeat(padSymbol, padDifference) + initial
	}

	// Step 5.
	if isNegative && useNegative {
		initial = negativePrefix + initial + negativeSuffix
	}

	// Step 6.
	return initial
}

// RenderMarker is CounterStyle.render_marker in weasyprint/css/counters.py.
// It returns the content of a ::marker pseudo-element.
func (cs CounterStyle) RenderMarker(counterName Value, counterValue int) string {
	return cs.renderMarker(counterName, counterValue, 0)
}

// renderMarker is RenderMarker with the depth of the fallbacks.
func (cs CounterStyle) renderMarker(counterName Value, counterValue, depth int) string {
	counter := cs.resolveCounter(counterName, nil)
	if counter == nil {
		if _, ok := cs["decimal"]; ok && depth == 0 {
			return cs.renderMarker(String("decimal"), counterValue, depth+1)
		}
		// Could happen if the UA stylesheet is not used.
		return ""
	}

	prefix := counter["prefix"]
	if !truthy(prefix) {
		prefix = Tuple{String("string"), String("")}
	}
	suffix := counter["suffix"]
	if !truthy(suffix) {
		suffix = Tuple{String("string"), String(". ")}
	}

	value := cs.RenderValue(counterValue, counterName)
	return symbol(prefix) + value + symbol(suffix)
}

// Copy is CounterStyle.copy in weasyprint/css/counters.py. The rules are
// never changed, so they are not copied.
func (cs CounterStyle) Copy() CounterStyle {
	return maps.Clone(cs)
}

// pyMod is the operator % of Python for integers: the sign of the result is
// the sign of the divisor.
func pyMod(a, b int) int {
	m := a % b
	if m != 0 && (m < 0) != (b < 0) {
		m += b
	}
	return m
}

// pyDiv is the operator // of Python for integers: it rounds toward
// negative infinity.
func pyDiv(a, b int) int {
	q := a / b
	if (a%b != 0) && ((a < 0) != (b < 0)) {
		q--
	}
	return q
}

// abs returns the absolute value of an integer.
func abs(v int) int {
	if v < 0 {
		return -v
	}
	return v
}
