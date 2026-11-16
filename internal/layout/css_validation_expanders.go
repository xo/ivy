// Copyright (c) 2011-2021, Simon Sapin and contributors.
//
// Use of this source code is governed by the BSD-3-Clause license that can be
// found in the LICENSE file.

// This file ports weasyprint/css/validation/expanders.py: the validation of
// the shorthand properties, which expand into other properties.

package layout

import (
	"slices"
	"strings"

	"github.com/xo/okage/tinycss2"
)

// expanders is EXPANDERS in weasyprint/css/validation/expanders.py: the
// expanders of the shorthand properties, by name in kebab case.
var expanders = map[string]expanderFunc{}

// expander is the decorator expander in
// weasyprint/css/validation/expanders.py. It registers the expander of a
// shorthand property.
func expander(name string, f expanderFunc) {
	if _, ok := expanders[name]; ok {
		panic("expander registered twice " + name)
	}
	expanders[name] = f
}

// newPendingExpander is PendingExpander in
// weasyprint/css/validation/expanders.py: an expander whose validation is
// done when the computed values are defined. validator is the expander,
// with the name of the shorthand and the base URL bound, the partial of
// upstream.
func newPendingExpander(tokens []tinycss2.Node, name string, validator func([]tinycss2.Node) ([]nameValue, error)) *Pending {
	p := &Pending{Tokens: tokens, Name: name}
	p.validate = func(tokens []tinycss2.Node, wantedKey, _ string) (Value, error) {
		pairs, err := validator(tokens)
		if err != nil {
			return nil, err
		}
		for _, pair := range pairs {
			key := pair.name
			if strings.HasPrefix(key, "-") {
				key = name + key
			}
			if key == wantedKey {
				return pair.value, nil
			}
		}
		// Port: upstream raises KeyError, which no code catches.
		return nil, invalidValues("%s is not set by the %s shorthand", wantedKey, name)
	}
	return p
}

// findVar is _find_var in weasyprint/css/validation/expanders.py. It returns
// the pending expander that each expanded name takes when var() is in the
// tokens, or nil.
func findVar(tokens []tinycss2.Node, name string, validator func([]tinycss2.Node) ([]nameValue, error)) *Pending {
	for _, token := range tokens {
		if checkVar(token) {
			// Found CSS variable, keep pending-substitution values.
			return newPendingExpander(tokens, name, validator)
		}
	}
	return nil
}

// wrappedExpander is the type of the functions that generic_expander wraps.
// It returns the names or the suffixes, such as "-color", with their tokens.
type wrappedExpander func(tokens []tinycss2.Node, name, baseURL string) ([]nameTokens, error)

// genericExpander is the decorator generic_expander in
// weasyprint/css/validation/expanders.py. It wraps an expander so that it
// does not have to handle the 'inherit' and 'initial' cases, and can just
// return name suffixes. The missing suffixes get the initial value.
func genericExpander(expandedNames []string, wrapped wrappedExpander) expanderFunc {
	var self expanderFunc
	self = func(tokens []tinycss2.Node, name, baseURL string) ([]nameValue, error) {
		expander := func(tokens []tinycss2.Node) ([]nameValue, error) {
			return self(tokens, name, baseURL)
		}

		skipValidation := false
		var preset Value
		keyword := getSingleKeyword(tokens)
		if keyword == "inherit" || keyword == "initial" {
			preset = String(keyword)
			skipValidation = true
		} else if pending := findVar(tokens, name, expander); pending != nil {
			preset = pending
			skipValidation = true
		}

		results := map[string][]tinycss2.Node{}
		if !skipValidation {
			result, err := wrapped(tokens, name, baseURL)
			if err != nil {
				return nil, err
			}
			for _, pair := range result {
				if !slices.Contains(expandedNames, pair.name) {
					// Port: upstream asserts that the name is expanded.
					return nil, invalidValues("unexpected %s value in a %s shorthand", pair.name, name)
				}
				if _, ok := results[pair.name]; ok {
					return nil, invalidValues(
						"got multiple %s values in a %s shorthand", strings.Trim(pair.name, "-"), name)
				}
				results[pair.name] = pair.tokens
			}
		}

		pairs := make([]nameValue, 0, len(expandedNames))
		for _, newName := range expandedNames {
			actualNewName := newName
			if strings.HasPrefix(newName, "-") {
				// newName is a suffix.
				actualNewName = name + newName
			}
			var value Value
			switch tokens, ok := results[newName]; {
			case skipValidation:
				value = preset
			case ok:
				pair, err := validateNonShorthand(tokens, actualNewName, baseURL, true)
				if err != nil {
					return nil, err
				}
				actualNewName, value = pair.name, pair.value
			default:
				value = String("initial")
			}
			pairs = append(pairs, nameValue{actualNewName, value})
		}
		return pairs, nil
	}
	return self
}

// boundExpander returns the expander of a shorthand with its name and its
// base URL bound, as the partial of EXPANDERS[name] of upstream.
func boundExpander(name, baseURL string) func([]tinycss2.Node) ([]nameValue, error) {
	return func(tokens []tinycss2.Node) ([]nameValue, error) {
		return expanders[name](tokens, name, baseURL)
	}
}

// pendingPairs returns the pairs of a pending expander for each name.
func pendingPairs(pending *Pending, names []string) []nameValue {
	pairs := make([]nameValue, len(names))
	for i, name := range names {
		pairs[i] = nameValue{name, pending}
	}
	return pairs
}

// expandTwoLogicalSides is expand_two_logical_sides in
// weasyprint/css/validation/expanders.py. It expands the properties that set
// a token for two logical sides of a box.
func expandTwoLogicalSides(tokens []tinycss2.Node, name, baseURL string) ([]nameValue, error) {
	return expandSides(tokens, name, baseURL, []string{"start", "end"})
}

// expandFourSides is expand_four_sides in
// weasyprint/css/validation/expanders.py. It expands the properties that set
// a token for four sides of a box, possibly logical.
func expandFourSides(tokens []tinycss2.Node, name, baseURL string) ([]nameValue, error) {
	sides := []string{"top", "right", "bottom", "left"}
	if len(tokens) > 0 && getKeyword(tokens[0]) == "logical" {
		sides = []string{"block-start", "inline-start", "block-end", "inline-end"}
		tokens = tokens[1:]
	}
	return expandSides(tokens, name, baseURL, sides)
}

// expandSides is _expand_sides in weasyprint/css/validation/expanders.py. It
// expands the properties that set a token for two or four sides of a box.
func expandSides(tokens []tinycss2.Node, name, baseURL string, sides []string) ([]nameValue, error) {
	// Define expanded names.
	expandedNames := make([]string, 0, len(sides))
	for _, side := range sides {
		switch {
		case strings.HasSuffix(name, "-color") || strings.HasSuffix(name, "-style") || strings.HasSuffix(name, "-width"):
			// For example, border-color becomes border-*-color, not
			// border-color-*.
			expandedNames = append(expandedNames, name[:len(name)-6]+"-"+side+"-"+name[len(name)-5:])
		case name == "inset" && !strings.Contains(side, "-"):
			// Physical "inset" does not yield "inset-top", just "top".
			expandedNames = append(expandedNames, side)
		default:
			expandedNames = append(expandedNames, name+"-"+side)
		}
	}

	// Return pending expanders if var is found.
	if pending := findVar(tokens, name, boundExpander(name, baseURL)); pending != nil {
		return pendingPairs(pending, expandedNames), nil
	}

	// Make sure we have the right number of tokens.
	switch {
	case len(tokens) == 1:
		tokens = slices.Repeat(tokens, len(sides))
	case len(sides) == 4 && len(tokens) == 2:
		tokens = slices.Repeat(tokens, 2) // (bottom, left) defaults to (top, right)
	case len(sides) == 4 && len(tokens) == 3:
		tokens = append(slices.Clip(tokens), tokens[1]) // left defaults to right
	case len(tokens) != len(sides):
		return nil, invalidValues("Expected 1 to %d tokens, got %d", len(sides), len(tokens))
	}
	pairs := make([]nameValue, 0, len(sides))
	for i, expandedName := range expandedNames {
		pair, err := validateNonShorthand(tokens[i:i+1], expandedName, baseURL, true)
		if err != nil {
			return nil, err
		}
		pairs = append(pairs, pair)
	}
	return pairs, nil
}

// expandBorderRadius is border_radius in
// weasyprint/css/validation/expanders.py, the validator of the
// border-radius property.
func expandBorderRadius(tokens []tinycss2.Node, _, baseURL string) ([]nameTokens, error) {
	var horizontal, vertical []tinycss2.Node
	current := &horizontal
	for _, token := range tokens {
		if tinycss2.IsLiteral(token, "/") {
			if current != &horizontal {
				return nil, invalidValues(`Expected only one "/" separator`)
			}
			// Upstream compares the token with the last token, and a
			// literal token "/" equals any other one.
			if tinycss2.IsLiteral(tokens[len(tokens)-1], "/") {
				return nil, invalidValues(`Expected value after "/" separator`)
			}
			current = &vertical
		} else {
			*current = append(*current, token)
		}
	}

	if len(vertical) == 0 {
		vertical = slices.Clone(horizontal)
	}

	fill := func(values []tinycss2.Node) ([]tinycss2.Node, error) {
		// Make sure we have 4 tokens.
		switch len(values) {
		case 1:
			return slices.Repeat(values, 4), nil
		case 2:
			return slices.Repeat(values, 2), nil // (br, bl) defaults to (tl, tr)
		case 3:
			return append(values, values[1]), nil // bl defaults to tr
		case 4:
			return values, nil
		}
		return nil, invalidValues("Expected 1 to 4 token components got %d", len(values))
	}
	horizontal, err := fill(horizontal)
	if err != nil {
		return nil, err
	}
	vertical, err = fill(vertical)
	if err != nil {
		return nil, err
	}
	corners := [...]string{"top-left", "top-right", "bottom-right", "bottom-left"}
	result := make([]nameTokens, 0, len(corners))
	for i, corner := range corners {
		name := "border-" + corner + "-radius"
		pair := []tinycss2.Node{horizontal[i], vertical[i]}
		if _, err := validateNonShorthand(pair, name, baseURL, true); err != nil {
			return nil, err
		}
		result = append(result, nameTokens{name, pair})
	}
	return result, nil
}

// expandListStyle is expand_list_style in
// weasyprint/css/validation/expanders.py. It expands the list-style
// shorthand property. See
// https://www.w3.org/TR/CSS21/generate.html#propdef-list-style.
func expandListStyle(tokens []tinycss2.Node, _, baseURL string) ([]nameTokens, error) {
	var result []nameTokens
	typeSpecified, imageSpecified := false, false
	noneCount := 0
	var noneToken tinycss2.Node
	for _, token := range tokens {
		if getKeyword(token) == "none" {
			// Can be either -style or -image, see at the end which is not
			// otherwise specified.
			noneCount++
			noneToken = token
			continue
		}

		single := []tinycss2.Node{token}
		var suffix string
		image, err := validateListStyleImage(single, baseURL)
		if err != nil {
			return nil, err
		}
		if image != nil {
			suffix = "-image"
			imageSpecified = true
		} else if position, err := validateListStylePosition(single, baseURL); err != nil {
			return nil, err
		} else if position != nil {
			suffix = "-position"
		} else if listType, err := validateListStyleType(single, baseURL); err != nil {
			return nil, err
		} else if listType != nil {
			suffix = "-type"
			typeSpecified = true
		} else {
			return nil, errInvalid
		}
		result = append(result, nameTokens{suffix, single})
	}

	if !typeSpecified && noneCount > 0 {
		result = append(result, nameTokens{"-type", []tinycss2.Node{noneToken}})
		noneCount--
	}

	if !imageSpecified && noneCount > 0 {
		result = append(result, nameTokens{"-image", []tinycss2.Node{noneToken}})
		noneCount--
	}

	if noneCount > 0 {
		// Too many none tokens.
		return nil, errInvalid
	}
	return result, nil
}

// expandBorder is expand_border in weasyprint/css/validation/expanders.py. It
// expands the border shorthand property. See
// https://www.w3.org/TR/CSS21/box.html#propdef-border.
func expandBorder(tokens []tinycss2.Node, name, baseURL string) ([]nameValue, error) {
	var pairs []nameValue
	for _, suffix := range [...]string{"top", "right", "bottom", "left"} {
		side, err := expandBorderSide(tokens, name+"-"+suffix, baseURL)
		if err != nil {
			return nil, err
		}
		pairs = append(pairs, side...)
	}
	return pairs, nil
}

// expandLogicalBorder is expand_logical_border in
// weasyprint/css/validation/expanders.py. It expands the logical border-*
// shorthand properties.
func expandLogicalBorder(tokens []tinycss2.Node, name, baseURL string) ([]nameValue, error) {
	var pairs []nameValue
	for _, suffix := range [...]string{"start", "end"} {
		side, err := expandBorderSide(tokens, name+"-"+suffix, baseURL)
		if err != nil {
			return nil, err
		}
		pairs = append(pairs, side...)
	}
	return pairs, nil
}

// expandBorderSide is expand_border_side in
// weasyprint/css/validation/expanders.py, with its decorator
// generic_expander. It expands the border-* shorthand properties. See
// https://www.w3.org/TR/CSS21/box.html#propdef-border-top.
var expandBorderSide = genericExpander([]string{"-width", "-color", "-style"}, expandBorderSideTokens)

// expandBorderSideTokens is the function that expand_border_side wraps.
func expandBorderSideTokens(tokens []tinycss2.Node, _, baseURL string) ([]nameTokens, error) {
	result := make([]nameTokens, 0, len(tokens))
	for _, token := range tokens {
		single := []tinycss2.Node{token}
		var suffix string
		if parseColorToken(token) != nil {
			suffix = "-color"
		} else if width, err := validateBorderWidth(single, baseURL); err != nil {
			return nil, err
		} else if width != nil {
			suffix = "-width"
		} else if style, err := validateBorderStyle(single, baseURL); err != nil {
			return nil, err
		} else if style != nil {
			suffix = "-style"
		} else {
			return nil, errInvalid
		}
		result = append(result, nameTokens{suffix, single})
	}
	return result, nil
}

// truthyValidation calls a validator and reports whether its value is true
// for Python.
func truthyValidation(f validatorFunc, tokens []tinycss2.Node, baseURL string) (bool, error) {
	value, err := f(tokens, baseURL)
	return truthy(value), err
}

// firstSlash reports whether the first token is a literal "/".
func firstSlash(tokens []tinycss2.Node) bool {
	return len(tokens) > 0 && tinycss2.IsLiteral(tokens[0], "/")
}

// concat returns a new list with the tokens of a and b.
func concat(a, b []tinycss2.Node) []tinycss2.Node {
	return append(slices.Clip(a), b...)
}

// expandBorderImageTokens is the loop of expand_border_image and
// expand_mask_border in weasyprint/css/validation/expanders.py. withMode
// reads the -mode of mask-border.
func expandBorderImageTokens(tokens []tinycss2.Node, baseURL string, withMode bool) ([]nameTokens, error) {
	var result []nameTokens
	tokens = slices.Clone(tokens)
	pop := func() tinycss2.Node {
		token := tokens[0]
		tokens = tokens[1:]
		return token
	}
	// takeWhile pops the tokens that the validator accepts after the first
	// one. With withPrevious, the validator reads the tokens popped before.
	takeWhile := func(f validatorFunc, withPrevious bool) ([]tinycss2.Node, error) {
		values := []tinycss2.Node{pop()}
		for len(tokens) > 0 {
			check := tokens[:1]
			if withPrevious {
				check = concat(values, tokens[:1])
			}
			ok, err := truthyValidation(f, check, baseURL)
			if err != nil {
				return nil, err
			}
			if !ok {
				break
			}
			values = append(values, pop())
		}
		return values, nil
	}
	for len(tokens) > 0 {
		if ok, err := truthyValidation(validateBorderImageSource, tokens[:1], baseURL); err != nil {
			return nil, err
		} else if ok {
			result = append(result, nameTokens{"-source", []tinycss2.Node{pop()}})
			continue
		}
		if withMode {
			if ok, err := truthyValidation(validateMaskBorderMode, tokens[:1], baseURL); err != nil {
				return nil, err
			} else if ok {
				result = append(result, nameTokens{"-mode", []tinycss2.Node{pop()}})
				continue
			}
		}
		if ok, err := truthyValidation(validateBorderImageRepeat, tokens[:1], baseURL); err != nil {
			return nil, err
		} else if ok {
			repeats, err := takeWhile(validateBorderImageRepeat, false)
			if err != nil {
				return nil, err
			}
			result = append(result, nameTokens{"-repeat", repeats})
			continue
		}
		isSlice, err := truthyValidation(validateBorderImageSlice, tokens[:1], baseURL)
		if err != nil {
			return nil, err
		}
		if !isSlice && getKeyword(tokens[0]) != "fill" {
			return nil, errInvalid
		}
		sliceTokens, err := takeWhile(validateBorderImageSlice, true)
		if err != nil {
			return nil, err
		}
		result = append(result, nameTokens{"-slice", sliceTokens})
		if !firstSlash(tokens) {
			// slices other
			continue
		}
		// slices / *
		pop()
		if len(tokens) == 0 {
			// slices /
			return nil, errInvalid
		}
		isWidth, err := truthyValidation(validateBorderImageWidth, tokens[:1], baseURL)
		if err != nil {
			return nil, err
		}
		switch {
		case isWidth:
			widths, err := takeWhile(validateBorderImageWidth, true)
			if err != nil {
				return nil, err
			}
			result = append(result, nameTokens{"-width", widths})
			if !firstSlash(tokens) {
				// slices / widths other
				continue
			}
			// slices / widths / slash *
			pop()
		case firstSlash(tokens):
			// slices / / *
			pop()
		default:
			// slices / other
			return nil, errInvalid
		}
		if len(tokens) == 0 {
			// slices / * /
			return nil, errInvalid
		}
		isOutset, err := truthyValidation(validateBorderImageOutset, tokens[:1], baseURL)
		if err != nil {
			return nil, err
		}
		if !isOutset {
			// slash / * / other
			return nil, errInvalid
		}
		outsets, err := takeWhile(validateBorderImageOutset, true)
		if err != nil {
			return nil, err
		}
		result = append(result, nameTokens{"-outset", outsets})
	}
	return result, nil
}

// expandBorderImage is expand_border_image in
// weasyprint/css/validation/expanders.py. It expands the border-image-*
// shorthand properties. See
// https://drafts.csswg.org/css-backgrounds/#the-border-image.
func expandBorderImage(tokens []tinycss2.Node, _, baseURL string) ([]nameTokens, error) {
	return expandBorderImageTokens(tokens, baseURL, false)
}

// expandMaskBorder is expand_mask_border in
// weasyprint/css/validation/expanders.py. It expands the mask-border-*
// shorthand properties. See
// https://drafts.fxtf.org/css-masking/#the-mask-border.
func expandMaskBorder(tokens []tinycss2.Node, _, baseURL string) ([]nameTokens, error) {
	return expandBorderImageTokens(tokens, baseURL, true)
}

// backgroundExpandedNames are the expanded names of expand_background.
var backgroundExpandedNames = []string{
	"background-color", "background-image", "background-repeat",
	"background-attachment", "background-position", "background-size",
	"background-clip", "background-origin",
}

// lastReversed returns the last n tokens of a stack in the order of the
// source, as tokens[-n:][::-1] of upstream does on a reversed list.
func lastReversed(stack []tinycss2.Node, n int) []tinycss2.Node {
	n = min(n, len(stack))
	result := make([]tinycss2.Node, n)
	for i := range n {
		result[i] = stack[len(stack)-1-i]
	}
	return result
}

// expandBackground is expand_background in
// weasyprint/css/validation/expanders.py. It expands the background
// shorthand property. See
// https://drafts.csswg.org/css-backgrounds-3/#the-background.
func expandBackground(tokens []tinycss2.Node, name, baseURL string) ([]nameValue, error) {
	keyword := getSingleKeyword(tokens)
	if keyword == "initial" || keyword == "inherit" {
		pairs := make([]nameValue, len(backgroundExpandedNames))
		for i, name := range backgroundExpandedNames {
			pairs[i] = nameValue{name, String(keyword)}
		}
		return pairs, nil
	}

	expander := func(tokens []tinycss2.Node) ([]nameValue, error) {
		return expandBackground(tokens, name, baseURL)
	}
	if pending := findVar(tokens, name, expander); pending != nil {
		return pendingPairs(pending, backgroundExpandedNames), nil
	}

	var layers [][]nameValue
	var color Value
	parts := splitOnComma(tokens)
	for i := len(parts) - 1; i >= 0; i-- {
		layerColor, layer, err := parseBackgroundLayer(parts[i], baseURL, i == len(parts)-1)
		if err != nil {
			return nil, err
		}
		if i == len(parts)-1 {
			color = layerColor
		}
		layers = append(layers, layer)
	}
	// results maps each name to the values of the layers, from the last
	// layer, as the dict of upstream does.
	lastLayer := layers[0]
	pairs := make([]nameValue, 0, len(lastLayer)+1)
	for _, item := range lastLayer {
		values := Tuple{item.value}
		for _, layer := range layers[1:] {
			for _, other := range layer {
				if other.name == item.name {
					values = append(values, other.value)
				}
			}
		}
		slices.Reverse(values) // "Un-reverse"
		pairs = append(pairs, nameValue{item.name, values})
	}
	return append(pairs, nameValue{"background-color", color}), nil
}

// parseBackgroundLayer is parse_layer in expand_background in
// weasyprint/css/validation/expanders.py. It returns the color and the
// values of a layer.
func parseBackgroundLayer(tokens []tinycss2.Node, baseURL string, finalLayer bool) (Value, []nameValue, error) {
	var results []nameValue
	has := func(name string) bool {
		return slices.ContainsFunc(results, func(r nameValue) bool { return r.name == name })
	}
	add := func(name string, value Value, err error) (bool, error) {
		if err != nil {
			return false, err
		}
		if value == nil {
			return false, nil
		}
		name = "background-" + name
		if has(name) {
			return false, errInvalid
		}
		results = append(results, nameValue{name, value})
		return true, nil
	}

	// Make tokens a stack.
	stack := slices.Clone(tokens)
	slices.Reverse(stack)
	for len(stack) > 0 {
		v, err := validateBackgroundRepeatSingle(lastReversed(stack, 2), baseURL)
		if ok, err := add("repeat", v, err); err != nil {
			return nil, nil, err
		} else if ok {
			stack = stack[:len(stack)-min(2, len(stack))]
			continue
		}
		token := stack[len(stack)-1:]
		if finalLayer {
			v, err := validateOtherColors(token, baseURL)
			if ok, err := add("color", v, err); err != nil {
				return nil, nil, err
			} else if ok {
				stack = stack[:len(stack)-1]
				continue
			}
		}
		v, err = validateBackgroundImageSingle(token, baseURL)
		if ok, err := add("image", v, err); err != nil {
			return nil, nil, err
		} else if ok {
			stack = stack[:len(stack)-1]
			continue
		}
		v, err = validateBackgroundRepeatSingle(token, baseURL)
		if ok, err := add("repeat", v, err); err != nil {
			return nil, nil, err
		} else if ok {
			stack = stack[:len(stack)-1]
			continue
		}
		v, err = validateBackgroundAttachmentSingle(token, baseURL)
		if ok, err := add("attachment", v, err); err != nil {
			return nil, nil, err
		} else if ok {
			stack = stack[:len(stack)-1]
			continue
		}
		var position Value
		for _, n := range []int{4, 3, 2, 1}[4-min(4, len(stack)):] {
			position, err = validateBackgroundPositionSingle(lastReversed(stack, n), baseURL)
			if err != nil {
				return nil, nil, err
			}
			if position == nil {
				continue
			}
			if _, err := add("position", position, nil); err != nil {
				return nil, nil, err
			}
			stack = stack[:len(stack)-n]
			if len(stack) > 0 && tinycss2.IsLiteral(stack[len(stack)-1], "/") {
				for _, n := range []int{3, 2}[2-min(2, len(stack)):] {
					// n includes the '/' delimiter. The tokens are
					// tokens[-n:-1][::-1] of upstream.
					var nTokens []tinycss2.Node
					if len(stack) > 0 {
						end := len(stack) - 1
						start := max(0, len(stack)-n)
						nTokens = lastReversed(stack[start:end], end-start)
					}
					size, err := validateBackgroundSizeSingle(nTokens, baseURL)
					if err != nil {
						return nil, nil, err
					}
					if size != nil {
						if _, err := add("size", size, nil); err != nil {
							return nil, nil, err
						}
						stack = stack[:max(0, len(stack)-n)]
					}
				}
			}
			break
		}
		if position != nil {
			continue
		}
		v, err = validateBoxSingle(token, baseURL)
		if ok, err := add("origin", v, err); err != nil {
			return nil, nil, err
		} else if ok {
			stack = stack[:len(stack)-1]
			nextToken := stack[len(stack)-min(1, len(stack)):]
			v, err := validateBoxSingle(nextToken, baseURL)
			if ok, err := add("clip", v, err); err != nil {
				return nil, nil, err
			} else if ok {
				stack = stack[:len(stack)-1]
			} else {
				// The same keyword sets both.
				v, err := validateBoxSingle(token, baseURL)
				if _, err := add("clip", v, err); err != nil {
					return nil, nil, err
				}
			}
			continue
		}
		return nil, nil, errInvalid
	}

	var color Value = String("transparent")
	if i := slices.IndexFunc(results, func(r nameValue) bool { return r.name == "background-color" }); i >= 0 {
		color = results[i].value
		results = slices.Delete(results, i, i+1)
	}
	for _, name := range backgroundExpandedNames {
		if name != "background-color" && !has(name) {
			initial, _ := InitialValues[strings.ReplaceAll(name, "-", "_")].(Tuple)
			var value Value
			if len(initial) > 0 {
				value = initial[0]
			}
			results = append(results, nameValue{name, value})
		}
	}
	return color, results, nil
}

// expandTextDecoration is expand_text_decoration in
// weasyprint/css/validation/expanders.py. It expands the text-decoration
// shorthand property.
func expandTextDecoration(tokens []tinycss2.Node, _, baseURL string) ([]nameTokens, error) {
	var line, color, style, thickness []tinycss2.Node
	noneInLine := false

	for _, token := range tokens {
		keyword := getKeyword(token)
		switch {
		case in(keyword, "none", "underline", "overline", "line-through", "blink"):
			line = append(line, token)
			if noneInLine {
				return nil, errInvalid
			} else if keyword == "none" {
				noneInLine = true
			}
		case in(keyword, "solid", "double", "dotted", "dashed", "wavy"):
			if len(style) > 0 {
				return nil, errInvalid
			}
			style = append(style, token)
		case truthy(parseColorToken(token)):
			if len(color) > 0 {
				return nil, errInvalid
			}
			color = append(color, token)
		default:
			ok, err := truthyValidation(validateTextDecorationThickness, []tinycss2.Node{token}, baseURL)
			if err != nil {
				return nil, err
			}
			if !ok || len(thickness) > 0 {
				return nil, errInvalid
			}
			thickness = append(thickness, token)
		}
	}

	var result []nameTokens
	if len(line) > 0 {
		result = append(result, nameTokens{"-line", line})
	}
	if len(color) > 0 {
		result = append(result, nameTokens{"-color", color})
	}
	if len(style) > 0 {
		result = append(result, nameTokens{"-style", style})
	}
	if len(thickness) > 0 {
		result = append(result, nameTokens{"-thickness", thickness})
	}
	return result, nil
}

// expandPageBreakBeforeAfter is expand_page_break_before_after in
// weasyprint/css/validation/expanders.py. It expands the legacy
// page-break-before and page-break-after properties. See
// https://www.w3.org/TR/css-break-3/#page-break-properties.
func expandPageBreakBeforeAfter(tokens []tinycss2.Node, name, _ string) ([]nameTokens, error) {
	keyword := getSingleKeyword(tokens)
	_, newName, _ := strings.Cut(name, "-")
	switch keyword {
	case "auto", "left", "right", "avoid":
		return []nameTokens{{newName, tokens}}, nil
	case "always":
		line, column := tokens[0].Source()
		return []nameTokens{{newName, []tinycss2.Node{tinycss2.NewIdentToken(line, column, "page")}}}, nil
	}
	return nil, errInvalid
}

// expandPageBreakInside is expand_page_break_inside in
// weasyprint/css/validation/expanders.py. It expands the legacy
// page-break-inside property. See
// https://www.w3.org/TR/css-break-3/#page-break-properties.
func expandPageBreakInside(tokens []tinycss2.Node, _, _ string) ([]nameTokens, error) {
	if keyword := getSingleKeyword(tokens); keyword == "auto" || keyword == "avoid" {
		return []nameTokens{{"break-inside", tokens}}, nil
	}
	return nil, errInvalid
}

// expandColumns is expand_columns in weasyprint/css/validation/expanders.py.
// It expands the columns shorthand property.
func expandColumns(tokens []tinycss2.Node, _, baseURL string) ([]nameTokens, error) {
	name := ""
	if len(tokens) == 2 && getKeyword(tokens[0]) == "auto" {
		tokens = []tinycss2.Node{tokens[1], tokens[0]}
	}
	var result []nameTokens
	for _, token := range tokens {
		single := []tinycss2.Node{token}
		width, err := validateColumnWidth(single, baseURL)
		if err != nil {
			return nil, err
		}
		if width != nil && name != "column-width" {
			name = "column-width"
		} else if count, err := validateColumnCount(single, baseURL); err != nil {
			return nil, err
		} else if count != nil {
			name = "column-count"
		} else {
			return nil, errInvalid
		}
		result = append(result, nameTokens{name, single})
	}
	if len(tokens) == 1 {
		if name == "column-count" {
			name = "column-width"
		} else {
			name = "column-count"
		}
		line, column := tokens[0].Source()
		result = append(result, nameTokens{name, []tinycss2.Node{tinycss2.NewIdentToken(line, column, "auto")}})
	}
	return result, nil
}

// expandFontVariantShorthand is font_variant in
// weasyprint/css/validation/expanders.py. It expands the font-variant
// shorthand property. See https://www.w3.org/TR/css-fonts-3/#font-variant-prop.
func expandFontVariantShorthand(tokens []tinycss2.Node, _, _ string) ([]nameTokens, error) {
	return expandFontVariant(tokens)
}

// fontSystemKeywords are the system fonts of expand_font.
var fontSystemKeywords = []string{"caption", "icon", "menu", "message-box", "small-caption", "status-bar"}

// expandFont is expand_font in weasyprint/css/validation/expanders.py. It
// expands the font shorthand property. See
// https://www.w3.org/TR/css-fonts-3/#font-prop.
func expandFont(tokens []tinycss2.Node, _, baseURL string) ([]nameTokens, error) {
	if keyword := getSingleKeyword(tokens); slices.Contains(fontSystemKeywords, keyword) {
		return nil, invalidValues("System fonts are not supported")
	}

	var result []nameTokens
	// Make tokens a stack.
	stack := slices.Clone(tokens)
	slices.Reverse(stack)
	pop := func() tinycss2.Node {
		token := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		return token
	}
	// Values for font-style, font-variant-caps, font-weight and
	// font-stretch can come in any order and are all optional.
	var token tinycss2.Node
	broken := false
	for range 4 {
		token = pop()
		if getKeyword(token) != "normal" {
			// Just ignore 'normal' keywords. Unspecified properties will get
			// their initial token, which is 'normal' for all four here.
			single := []tinycss2.Node{token}
			suffix := ""
			for _, candidate := range []struct {
				suffix string
				f      validatorFunc
			}{
				{"-style", validateFontStyle},
				{"-variant-caps", validateFontVariantCaps},
				{"-weight", validateFontWeight},
				{"-stretch", validateFontStretch},
			} {
				value, err := candidate.f(single, baseURL)
				if err != nil {
					return nil, err
				}
				if value != nil {
					suffix = candidate.suffix
					break
				}
			}
			if suffix == "" {
				// We are done with these four, continue with font-size.
				broken = true
				break
			}
			result = append(result, nameTokens{suffix, single})
		}
		if len(stack) == 0 {
			return nil, errInvalid
		}
	}
	if !broken {
		if len(stack) == 0 {
			return nil, errInvalid
		}
		token = pop()
	}

	// Then font-size is mandatory. The latest token from the loop.
	if size, err := validateFontSize([]tinycss2.Node{token}, baseURL); err != nil {
		return nil, err
	} else if size == nil {
		return nil, errInvalid
	}
	result = append(result, nameTokens{"-size", []tinycss2.Node{token}})

	// Then line-height is optional, but font-family is not so the list must
	// not be empty yet.
	if len(stack) == 0 {
		return nil, errInvalid
	}

	token = pop()
	if tinycss2.IsLiteral(token, "/") {
		if len(stack) == 0 {
			// Port: upstream raises IndexError for "12px /".
			return nil, errInvalid
		}
		token = pop()
		if lineHeight, err := validateLineHeight([]tinycss2.Node{token}, baseURL); err != nil {
			return nil, err
		} else if lineHeight == nil {
			return nil, errInvalid
		}
		result = append(result, nameTokens{"line-height", []tinycss2.Node{token}})
	} else {
		// We popped a font-family, add it back.
		stack = append(stack, token)
	}

	// Reverse the stack to get normal list.
	slices.Reverse(stack)
	if family, err := validateFontFamily(stack, baseURL); err != nil {
		return nil, err
	} else if family == nil {
		return nil, errInvalid
	}
	return append(result, nameTokens{"-family", stack}), nil
}

// expandWordWrap is expand_word_wrap in
// weasyprint/css/validation/expanders.py. It expands the word-wrap legacy
// property. See https://www.w3.org/TR/css-text-3/#overflow-wrap.
func expandWordWrap(tokens []tinycss2.Node, _, baseURL string) ([]nameTokens, error) {
	keyword, err := validateOverflowWrap(tokens, baseURL)
	if err != nil {
		return nil, err
	}
	if keyword == nil {
		return nil, errInvalid
	}
	return []nameTokens{{"overflow-wrap", tokens}}, nil
}

// numberToken makes a number token as NumberToken(line, column, value,
// int_value, str(value)) of upstream does.
func numberToken(line, column int, value float64) tinycss2.Node {
	isInteger := value == float64(int64(value))
	var intValue int64
	if isInteger {
		intValue = int64(value)
	}
	return tinycss2.NewNumberToken(line, column, value, intValue, isInteger, pyStr(value))
}

// expandFlex is expand_flex in weasyprint/css/validation/expanders.py. It
// expands the flex property.
func expandFlex(tokens []tinycss2.Node, _, baseURL string) ([]nameTokens, error) {
	line, column := tokens[0].Source()
	if getSingleKeyword(tokens) == "none" {
		zeroToken := tinycss2.NewNumberToken(line, column, 0, 0, true, "0")
		autoToken := tinycss2.NewIdentToken(line, column, "auto")
		return []nameTokens{
			{"-grow", []tinycss2.Node{zeroToken}},
			{"-shrink", []tinycss2.Node{zeroToken}},
			{"-basis", []tinycss2.Node{autoToken}},
		}, nil
	}
	grow, shrink := 1.0, 1.0
	var basis tinycss2.Node
	growFound, shrinkFound, basisFound := false, false, false
	factor := func(token tinycss2.Node) (float64, error) {
		value, err := validateFlexGrowShrink([]tinycss2.Node{token}, baseURL)
		if err != nil {
			return 0, err
		}
		if value == nil {
			return 0, errInvalid
		}
		if d, ok := value.(Dimension); ok {
			return d.Value, nil
		}
		return num(value), nil
	}
	for _, token := range tokens {
		// "A unitless zero that is not already preceded by two flex
		// factors must be interpreted as a flex factor."
		n, isNumber := token.(*tinycss2.NumberToken)
		forcedFlexFactor := isNumber && n.IsInteger && n.IntValue == 0 && (!growFound || !shrinkFound)
		if !basisFound && !forcedFlexFactor {
			newBasis, err := validateFlexBasis([]tinycss2.Node{token}, baseURL)
			if err != nil {
				return nil, err
			}
			if newBasis != nil {
				basis = token
				basisFound = true
				continue
			}
		}
		switch {
		case !growFound:
			value, err := factor(token)
			if err != nil {
				return nil, err
			}
			grow, growFound = value, true
		case !shrinkFound:
			value, err := factor(token)
			if err != nil {
				return nil, err
			}
			shrink, shrinkFound = value, true
		default:
			return nil, errInvalid
		}
	}
	growToken := numberToken(line, column, grow)
	shrinkToken := numberToken(line, column, shrink)
	if !basisFound {
		basis = tinycss2.NewDimensionToken(line, column, 0, 0, true, "0", "px")
	}
	return []nameTokens{
		{"-grow", []tinycss2.Node{growToken}},
		{"-shrink", []tinycss2.Node{shrinkToken}},
		{"-basis", []tinycss2.Node{basis}},
	}, nil
}

// expandFlexFlow is expand_flex_flow in
// weasyprint/css/validation/expanders.py. It expands the flex-flow property.
func expandFlexFlow(tokens []tinycss2.Node, _, baseURL string) ([]nameTokens, error) {
	switch len(tokens) {
	case 2:
		for _, sorted := range [][]tinycss2.Node{tokens, {tokens[1], tokens[0]}} {
			direction, err := truthyValidation(validateFlexDirection, sorted[:1], baseURL)
			if err != nil {
				return nil, err
			}
			wrap, err := truthyValidation(validateFlexWrap, sorted[1:2], baseURL)
			if err != nil {
				return nil, err
			}
			if direction && wrap {
				return []nameTokens{
					{"flex-direction", sorted[:1]},
					{"flex-wrap", sorted[1:2]},
				}, nil
			}
		}
	case 1:
		direction, err := truthyValidation(validateFlexDirection, tokens, baseURL)
		if err != nil {
			return nil, err
		}
		if direction {
			return []nameTokens{{"flex-direction", tokens}}, nil
		}
		wrap, err := truthyValidation(validateFlexWrap, tokens, baseURL)
		if err != nil {
			return nil, err
		}
		if wrap {
			return []nameTokens{{"flex-wrap", tokens}}, nil
		}
	}
	return nil, errInvalid
}

// splitOnSlash splits tokens on the literal tokens "/".
func splitOnSlash(tokens []tinycss2.Node) [][]tinycss2.Node {
	parts := [][]tinycss2.Node{{}}
	for _, token := range tokens {
		if tinycss2.IsLiteral(token, "/") {
			parts = append(parts, []tinycss2.Node{})
		} else {
			parts[len(parts)-1] = append(parts[len(parts)-1], token)
		}
	}
	return parts
}

// expandGridTemplateTokens is _expand_grid_template in
// weasyprint/css/validation/expanders.py.
func expandGridTemplateTokens(tokens []tinycss2.Node, baseURL string) ([]nameTokens, error) {
	line, column := tokens[0].Source()
	none := tinycss2.NewIdentToken(line, column, "none")
	if len(tokens) == 1 && getKeyword(tokens[0]) == "none" {
		return []nameTokens{
			{"-columns", []tinycss2.Node{none}},
			{"-rows", []tinycss2.Node{none}},
			{"-areas", []tinycss2.Node{none}},
		}, nil
	}
	slashSeparated := splitOnSlash(tokens)
	if len(slashSeparated) == 2 {
		rows, err := truthyValidation(validateGridTemplate, slashSeparated[0], baseURL)
		if err != nil {
			return nil, err
		}
		columns, err := truthyValidation(validateGridTemplate, slashSeparated[1], baseURL)
		if err != nil {
			return nil, err
		}
		if columns && rows {
			return []nameTokens{
				{"-columns", slashSeparated[1]},
				{"-rows", slashSeparated[0]},
				{"-areas", []tinycss2.Node{none}},
			}, nil
		}
	}
	// TODO: Handle last syntax.
	return nil, errInvalid
}

// expandGridTemplate is expand_grid_template in
// weasyprint/css/validation/expanders.py. It expands the grid-template
// property.
func expandGridTemplate(tokens []tinycss2.Node, _, baseURL string) ([]nameTokens, error) {
	return expandGridTemplateTokens(tokens, baseURL)
}

// pyTokenEqual compares two tokens as == of Python does: a literal token
// equals a literal token of the same value, and any other token is only
// equal to itself.
func pyTokenEqual(a, b tinycss2.Node) bool {
	if l, ok := a.(*tinycss2.LiteralToken); ok {
		return tinycss2.IsLiteral(b, l.Value) || a == b
	}
	return a == b
}

// expandGrid is expand_grid in weasyprint/css/validation/expanders.py. It
// expands the grid property.
func expandGrid(tokens []tinycss2.Node, _, baseURL string) ([]nameTokens, error) {
	line, column := tokens[0].Source()
	auto := tinycss2.NewIdentToken(line, column, "auto")
	none := tinycss2.NewIdentToken(line, column, "none")
	row := tinycss2.NewIdentToken(line, column, "row")
	columnToken := tinycss2.NewIdentToken(line, column, "column")
	if template, err := expandGridTemplateTokens(tokens, baseURL); err == nil {
		result := make([]nameTokens, 0, len(template)+3)
		for _, pair := range template {
			parts := strings.Split(pair.name, "-")
			result = append(result, nameTokens{"-template-" + parts[len(parts)-1], pair.tokens})
		}
		return append(result,
			nameTokens{"-auto-columns", []tinycss2.Node{auto}},
			nameTokens{"-auto-rows", []tinycss2.Node{auto}},
			nameTokens{"-auto-flow", []tinycss2.Node{row}},
		), nil
	}
	splitTokens := splitOnSlash(tokens)
	if len(splitTokens) != 2 {
		return nil, errInvalid
	}
	autoTrack := ""
	var dense tinycss2.Node
	templates := map[string][]tinycss2.Node{"row": nil, "column": nil}
	for i, track := range [...]string{"row", "column"} {
		group := splitTokens[i]
		autoFlowToken := false
		for _, token := range group {
			switch getKeyword(token) {
			case "dense":
				if dense != nil || (autoTrack != "" && autoTrack != track) {
					return nil, errInvalid
				}
				dense = token
				autoTrack = track
			case "auto-flow":
				if autoFlowToken || (autoTrack != "" && autoTrack != track) {
					return nil, errInvalid
				}
				autoFlowToken = true
				autoTrack = track
			default:
				if !pyTokenEqual(token, group[len(group)-1]) {
					return nil, errInvalid
				}
				templates[track] = append(templates[track], token)
			}
		}
	}
	if autoTrack == "" {
		return nil, errInvalid
	}
	nonAutoTrack := "column"
	autoTrackToken := row
	if autoTrack == "column" {
		nonAutoTrack = "row"
		autoTrackToken = columnToken
	}
	autoFlow := []tinycss2.Node{autoTrackToken}
	if dense != nil {
		autoFlow = append(autoFlow, dense)
	}
	return []nameTokens{
		{"-auto-flow", autoFlow},
		{"-auto-" + autoTrack + "s", templates[autoTrack]},
		{"-auto-" + nonAutoTrack + "s", []tinycss2.Node{auto}},
		{"-template-" + autoTrack + "s", []tinycss2.Node{none}},
		{"-template-" + nonAutoTrack + "s", templates[nonAutoTrack]},
		{"-template-areas", []tinycss2.Node{none}},
	}, nil
}

// isCustomIdent reports whether the first two items of a validated grid
// line are None, as set(validation[:2]) == {None} of upstream does.
func isCustomIdent(validation Value) bool {
	t, ok := validation.(Tuple)
	if !ok || len(t) == 0 {
		return false
	}
	for _, item := range t[:min(2, len(t))] {
		if item != nil {
			return false
		}
	}
	return true
}

// expandGridColumnRowArea is _expand_grid_column_row_area in
// weasyprint/css/validation/expanders.py. It returns the tokens of each
// grid line.
func expandGridColumnRowArea(tokens []tinycss2.Node, maxNumber int, baseURL string) ([][]tinycss2.Node, error) {
	gridLines := splitOnSlash(tokens)
	if len(gridLines) < 1 || len(gridLines) > maxNumber {
		return nil, errInvalid
	}
	var result [][]tinycss2.Node
	var validations []Value
	for _, group := range gridLines {
		validation, err := validateGridLine(group, baseURL)
		if err != nil {
			return nil, err
		}
		if !truthy(validation) {
			return nil, errInvalid
		}
		validations = append(validations, validation)
		result = append(result, group)
	}
	last := tokens[len(tokens)-1]
	line, column := last.Source()
	auto := []tinycss2.Node{tinycss2.NewIdentToken(line, column, "auto")}
	lines := len(gridLines)
	if lines <= 1 {
		value := auto
		if isCustomIdent(validations[0]) {
			value = gridLines[0]
		}
		gridLines = append(gridLines, gridLines[len(gridLines)-1])
		validations = append(validations, validations[0])
		result = append(result, value)
	}
	if lines <= 2 && 2 < maxNumber {
		value := auto
		if isCustomIdent(validations[0]) {
			value = gridLines[0]
		}
		result = append(result, value)
	}
	if lines <= 3 && 3 < maxNumber {
		value := auto
		if isCustomIdent(validations[1]) {
			value = gridLines[1]
		}
		result = append(result, value)
	}
	return result, nil
}

// expandGridColumnRow is expand_grid_column_row in
// weasyprint/css/validation/expanders.py. It expands the grid-column and
// grid-row properties.
func expandGridColumnRow(tokens []tinycss2.Node, _, baseURL string) ([]nameTokens, error) {
	tokensList, err := expandGridColumnRowArea(tokens, 2, baseURL)
	if err != nil {
		return nil, err
	}
	sides := [...]string{"start", "end"}
	var result []nameTokens
	for i, tokens := range tokensList[:min(len(tokensList), len(sides))] {
		result = append(result, nameTokens{"-" + sides[i], tokens})
	}
	return result, nil
}

// expandGridArea is expand_grid_area in
// weasyprint/css/validation/expanders.py. It expands the grid-area property.
func expandGridArea(tokens []tinycss2.Node, _, baseURL string) ([]nameTokens, error) {
	tokensList, err := expandGridColumnRowArea(tokens, 4, baseURL)
	if err != nil {
		return nil, err
	}
	sides := [...]string{"row-start", "column-start", "row-end", "column-end"}
	var result []nameTokens
	for i, tokens := range tokensList[:min(len(tokensList), len(sides))] {
		result = append(result, nameTokens{"grid-" + sides[i], tokens})
	}
	return result, nil
}

// expandGap is expand_gap in weasyprint/css/validation/expanders.py. It
// expands the gap property.
func expandGap(tokens []tinycss2.Node, _, baseURL string) ([]nameTokens, error) {
	switch len(tokens) {
	case 1:
		if gap, err := validateGap(tokens, baseURL); err != nil {
			return nil, err
		} else if gap == nil {
			return nil, errInvalid
		}
		return []nameTokens{{"row-gap", tokens}, {"column-gap", tokens}}, nil
	case 2:
		columnGap, err := validateGap(tokens[0:1], baseURL)
		if err != nil {
			return nil, err
		}
		rowGap, err := validateGap(tokens[1:2], baseURL)
		if err != nil {
			return nil, err
		}
		if columnGap == nil || rowGap == nil {
			return nil, errInvalid
		}
		return []nameTokens{{"row-gap", tokens[0:1]}, {"column-gap", tokens[1:2]}}, nil
	}
	return nil, errInvalid
}

// expandLegacyGap returns the expander of the legacy grid-column-gap or
// grid-row-gap property: expand_legacy_column_gap and expand_legacy_row_gap
// in weasyprint/css/validation/expanders.py.
func expandLegacyGap(newName string) wrappedExpander {
	return func(tokens []tinycss2.Node, _, baseURL string) ([]nameTokens, error) {
		keyword, err := validateGap(tokens, baseURL)
		if err != nil {
			return nil, err
		}
		if keyword == nil {
			return nil, errInvalid
		}
		return []nameTokens{{newName, tokens}}, nil
	}
}

// expandPlace is expand_place_content, expand_place_items and
// expand_place_self in weasyprint/css/validation/expanders.py.
func expandPlace([]tinycss2.Node, string, string) ([]nameTokens, error) {
	// TODO
	return nil, errInvalid
}

// expandLineClamp is expand_line_clamp in
// weasyprint/css/validation/expanders.py. It expands the line-clamp
// property.
func expandLineClamp(tokens []tinycss2.Node, _, baseURL string) ([]nameTokens, error) {
	line, column := tokens[0].Source()
	switch len(tokens) {
	case 1:
		number, isNumber := tokens[0].(*tinycss2.NumberToken)
		switch {
		case getSingleKeyword(tokens) == "none":
			noneToken := tinycss2.NewIdentToken(line, column, "none")
			autoToken := tinycss2.NewIdentToken(line, column, "auto")
			return []nameTokens{
				{"max-lines", []tinycss2.Node{noneToken}},
				{"continue", []tinycss2.Node{autoToken}},
				{"block-ellipsis", []tinycss2.Node{noneToken}},
			}, nil
		case isNumber && number.IsInteger:
			autoToken := tinycss2.NewIdentToken(line, column, "auto")
			discardToken := tinycss2.NewIdentToken(line, column, "discard")
			return []nameTokens{
				{"max-lines", tokens[:1]},
				{"continue", []tinycss2.Node{discardToken}},
				{"block-ellipsis", []tinycss2.Node{autoToken}},
			}, nil
		}
	case 2:
		if number, ok := tokens[0].(*tinycss2.NumberToken); ok {
			ellipsis, err := validateBlockEllipsis(tokens[1:2], baseURL)
			if err != nil {
				return nil, err
			}
			if number.IsInteger && number.IntValue != 0 && ellipsis != nil {
				discardToken := tinycss2.NewIdentToken(line, column, "discard")
				return []nameTokens{
					{"max-lines", tokens[:1]},
					{"continue", []tinycss2.Node{discardToken}},
					{"block-ellipsis", tokens[1:2]},
				}, nil
			}
		}
	}
	return nil, errInvalid
}

// expandTextAlign is expand_text_align in
// weasyprint/css/validation/expanders.py. It expands the text-align
// property.
func expandTextAlign(tokens []tinycss2.Node, _, _ string) ([]nameTokens, error) {
	if len(tokens) != 1 {
		return nil, errInvalid
	}
	keyword := getSingleKeyword(tokens)
	if keyword == "" {
		return nil, errInvalid
	}
	line, column := tokens[0].Source()
	alignAll := tokens[0]
	if keyword == "justify-all" {
		alignAll = tinycss2.NewIdentToken(line, column, "justify")
	}
	alignLast := alignAll
	if keyword == "justify" {
		alignLast = tinycss2.NewIdentToken(line, column, "start")
	}
	return []nameTokens{
		{"-all", []tinycss2.Node{alignAll}},
		{"-last", []tinycss2.Node{alignLast}},
	}, nil
}

func init() {
	for _, name := range []string{
		"margin-block", "margin-inline", "padding-block", "padding-inline",
		"border-block-color", "border-block-style", "border-block-width",
		"border-inline-color", "border-inline-style", "border-inline-width",
		"inset-block", "inset-inline",
	} {
		expander(name, expandTwoLogicalSides)
	}
	for _, name := range []string{
		"border-color", "border-style", "border-width", "margin", "padding", "bleed", "inset",
	} {
		expander(name, expandFourSides)
	}
	expander("border-radius", genericExpander([]string{
		"border-top-left-radius", "border-top-right-radius",
		"border-bottom-right-radius", "border-bottom-left-radius",
	}, expandBorderRadius))
	expander("list-style", genericExpander([]string{"-type", "-position", "-image"}, expandListStyle))
	expander("border", expandBorder)
	expander("border-block", expandLogicalBorder)
	expander("border-inline", expandLogicalBorder)
	for _, name := range []string{
		"border-top", "border-right", "border-bottom", "border-left",
		"border-block-start", "border-block-end", "border-inline-start",
		"border-inline-end", "column-rule", "outline",
	} {
		expander(name, expandBorderSide)
	}
	expander("border-image", genericExpander(
		[]string{"-outset", "-repeat", "-slice", "-source", "-width"}, expandBorderImage))
	expander("mask-border", genericExpander(
		[]string{"-outset", "-repeat", "-slice", "-source", "-width", "-mode"}, expandMaskBorder))
	expander("background", expandBackground)
	expander("text-decoration", genericExpander(
		[]string{"-line", "-color", "-style", "-thickness"}, expandTextDecoration))
	expander("page-break-after", genericExpander([]string{"break-after"}, expandPageBreakBeforeAfter))
	expander("page-break-before", genericExpander([]string{"break-before"}, expandPageBreakBeforeAfter))
	expander("page-break-inside", genericExpander([]string{"break-inside"}, expandPageBreakInside))
	expander("columns", genericExpander([]string{"column-width", "column-count"}, expandColumns))
	expander("font-variant", genericExpander([]string{
		"-alternates", "-caps", "-east-asian", "-ligatures", "-numeric", "-position",
	}, expandFontVariantShorthand))
	// line-height is not a suffix.
	expander("font", genericExpander([]string{
		"-style", "-variant-caps", "-weight", "-stretch", "-size", "line-height", "-family",
	}, expandFont))
	expander("word-wrap", genericExpander([]string{"overflow-wrap"}, expandWordWrap))
	expander("flex", genericExpander([]string{"-grow", "-shrink", "-basis"}, expandFlex))
	expander("flex-flow", genericExpander([]string{"flex-direction", "flex-wrap"}, expandFlexFlow))
	expander("grid-template", genericExpander([]string{"-columns", "-rows", "-areas"}, expandGridTemplate))
	expander("grid", genericExpander([]string{
		"-template-columns", "-template-rows", "-template-areas",
		"-auto-columns", "-auto-rows", "-auto-flow",
	}, expandGrid))
	expander("grid-column", genericExpander([]string{"-start", "-end"}, expandGridColumnRow))
	expander("grid-row", genericExpander([]string{"-start", "-end"}, expandGridColumnRow))
	expander("grid-area", genericExpander([]string{
		"grid-row-start", "grid-row-end", "grid-column-start", "grid-column-end",
	}, expandGridArea))
	gap := genericExpander([]string{"column-gap", "row-gap"}, expandGap)
	expander("grid-gap", gap)
	expander("gap", gap)
	expander("grid-column-gap", genericExpander([]string{"column-gap"}, expandLegacyGap("column-gap")))
	expander("grid-row-gap", genericExpander([]string{"row-gap"}, expandLegacyGap("row-gap")))
	expander("place-content", genericExpander([]string{"align-content", "justify-content"}, expandPlace))
	expander("place-items", genericExpander([]string{"align-items", "justify-items"}, expandPlace))
	expander("place-self", genericExpander([]string{"align-self", "justify-self"}, expandPlace))
	expander("line-clamp", genericExpander([]string{"max-lines", "continue", "block-ellipsis"}, expandLineClamp))
	expander("text-align", genericExpander([]string{"-all", "-last"}, expandTextAlign))
}
