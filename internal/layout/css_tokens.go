// Copyright (c) 2011-2021, Simon Sapin and contributors.
//
// Use of this source code is governed by the BSD-3-Clause license that can be
// found in the LICENSE file.

// This file ports weasyprint/css/tokens.py.

package layout

import (
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/xo/okage/tinycss2"
)

// The percentages of weasyprint/css/tokens.py.
var (
	zeroPercent    = Dimension{0, "%"}
	fiftyPercent   = Dimension{50, "%"}
	hundredPercent = Dimension{100, "%"}
)

// backgroundPositionPercentages is BACKGROUND_POSITION_PERCENTAGES in
// weasyprint/css/tokens.py.
var backgroundPositionPercentages = map[string]Dimension{
	"top":    zeroPercent,
	"left":   zeroPercent,
	"center": fiftyPercent,
	"bottom": hundredPercent,
	"right":  hundredPercent,
}

// directionKeywords is DIRECTION_KEYWORDS in weasyprint/css/tokens.py. The
// keys are the keywords joined with spaces. The values are ('angle',
// radians), 0 upwards and then clockwise, or ('corner', keyword).
var directionKeywords = map[string]Tuple{
	"to top":          {String("angle"), Float(0)},
	"to right":        {String("angle"), Float(math.Pi / 2)},
	"to bottom":       {String("angle"), Float(math.Pi)},
	"to left":         {String("angle"), Float(math.Pi * 3 / 2)},
	"to top left":     {String("corner"), String("top_left")},
	"to left top":     {String("corner"), String("top_left")},
	"to top right":    {String("corner"), String("top_right")},
	"to right top":    {String("corner"), String("top_right")},
	"to bottom left":  {String("corner"), String("bottom_left")},
	"to left bottom":  {String("corner"), String("bottom_left")},
	"to bottom right": {String("corner"), String("bottom_right")},
	"to right bottom": {String("corner"), String("bottom_right")},
}

// The constants of the math functions, E, PI, PLUS_INFINITY,
// MINUS_INFINITY and NAN in weasyprint/css/tokens.py. They are functions,
// because the port can change the value of a token.
func tokenE() tinycss2.Node { return tinycss2.NewNumberToken(0, 0, math.E, 0, false, "e") }
func tokenPi() tinycss2.Node {
	return tinycss2.NewNumberToken(0, 0, math.Pi, 0, false, "π")
}
func tokenPlusInfinity() tinycss2.Node {
	return tinycss2.NewNumberToken(0, 0, math.Inf(1), 0, false, "∞")
}
func tokenMinusInfinity() tinycss2.Node {
	return tinycss2.NewNumberToken(0, 0, math.Inf(-1), 0, false, "-∞")
}
func tokenNaN() tinycss2.Node { return tinycss2.NewNumberToken(0, 0, math.NaN(), 0, false, "NaN") }

// InvalidValuesError is InvalidValues in weasyprint/css/tokens.py: values
// that are not valid or not supported for a known property.
type InvalidValuesError struct {
	// Message is the reason, or "" for none.
	Message string
}

// Error satisfies the error interface.
func (e *InvalidValuesError) Error() string {
	if e.Message == "" {
		return "invalid value"
	}
	return e.Message
}

// invalidValues returns an InvalidValuesError.
func invalidValues(format string, args ...any) error {
	return &InvalidValuesError{Message: fmt.Sprintf(format, args...)}
}

// errInvalid is InvalidValues with no message.
var errInvalid = &InvalidValuesError{}

// The errors of the math functions of weasyprint/css/tokens.py.
var (
	// errPercentageInMath is PercentageInMath: a percentage in a math
	// function without a reference length.
	errPercentageInMath = errors.New("percentage in math function without reference length")
	// errRelativeLengthInMath is RelativeLengthInMath: a relative length
	// unit in a math function without a reference style.
	errRelativeLengthInMath = errors.New("relative length unit in math function without reference style")
)

// Pending is Pending in weasyprint/css/tokens.py: a property value with a
// validation that waits until the computed values are defined, because it
// holds var(). See
// https://drafts.csswg.org/css-variables-2/#variables-in-shorthands.
//
// PendingProperty of css/validation/properties.py and PendingExpander of
// css/validation/expanders.py are a Pending with their own validate
// function.
type Pending struct {
	// Tokens are the tokens of the declaration.
	Tokens []tinycss2.Node
	// Name is the name of the property or the shorthand, in kebab case.
	Name string

	// validate is the abstract method validate of upstream. It returns the
	// validated value for the wanted key.
	validate func(tokens []tinycss2.Node, wantedKey, baseURL string) (Value, error)

	reportedError bool
}

func (*Pending) isValue() {}

// solve is Pending.solve in weasyprint/css/tokens.py. It returns the
// validated value, or an error.
func (p *Pending) solve(log *logger, tokens []tinycss2.Node, wantedKey, baseURL string) (Value, error) {
	var value Value
	err := error(nil)
	if len(tokens) == 0 {
		// Having no tokens is allowed by grammar but refused by all
		// properties and expanders.
		err = invalidValues("no value")
	} else {
		value, err = p.validate(tokens, wantedKey, baseURL)
	}
	if err == nil {
		return value, nil
	}
	if p.reportedError {
		return nil, err
	}
	line, column := 0, 0
	if len(p.Tokens) > 0 {
		line, column = p.Tokens[0].Source()
	}
	parts := make([]string, len(tokens))
	for i, token := range tokens {
		parts[i] = token.Serialize()
	}
	message := "invalid value"
	var iv *InvalidValuesError
	if errors.As(err, &iv) && iv.Message != "" {
		message = iv.Message
	}
	log.warning("Ignored `%s: %s` at %d:%d, %s.", p.Name, strings.Join(parts, " "), line, column, message)
	p.reportedError = true
	return nil, err
}

// parseColor is parse_color of tinycss2 color5.py for a value: a token, a
// string to parse, or a Color, which it returns as it is. It returns a
// Color, the String "currentcolor", or nil for no color.
func parseColor(value Value, colorSchemes ...string) Value {
	var token tinycss2.Node
	switch v := value.(type) {
	case Color:
		return v
	case Token:
		token = v.Node
	case String:
		token = tinycss2.ParseOneComponentValue(string(v), tinycss2.WithSkipComments(true))
	default:
		return nil
	}
	return parseColorToken(token, colorSchemes...)
}

// parseColorToken is parse_color of tinycss2 color5.py for a token.
func parseColorToken(token tinycss2.Node, colorSchemes ...string) Value {
	if token == nil {
		return nil
	}
	color, current := tinycss2.ParseColor5(token, colorSchemes...)
	switch {
	case current:
		return String("currentcolor")
	case color == nil:
		return nil
	}
	return Color{color}
}

// parseColorHint is parse_color_hint in weasyprint/css/tokens.py.
func parseColorHint(tokens []tinycss2.Node) Value {
	if len(tokens) == 1 {
		return getLength(tokens[0], true, true)
	}
	return nil
}

// parseColorStop is parse_color_stop in weasyprint/css/tokens.py. It returns
// a color and a position, or nil for no position.
func parseColorStop(tokens []tinycss2.Node) ([2]Value, error) {
	switch len(tokens) {
	case 1:
		color := parseColorToken(tokens[0])
		if isStr(color, "currentcolor") {
			// TODO: return the current color instead
			return [2]Value{mustParseColor("black"), nil}, nil
		}
		if color != nil {
			return [2]Value{color, nil}, nil
		}
	case 2:
		color := parseColorToken(tokens[0])
		position := getLength(tokens[1], true, true)
		if color != nil && position != nil {
			return [2]Value{color, position}, nil
		}
	}
	return [2]Value{}, errInvalid
}

// parseColorStopsAndHints is parse_color_stops_and_hints in
// weasyprint/css/tokens.py.
func parseColorStopsAndHints(colorStopsHints [][]tinycss2.Node) ([][2]Value, []Value, error) {
	if len(colorStopsHints) == 0 {
		return nil, nil, errInvalid
	}
	first, err := parseColorStop(colorStopsHints[0])
	if err != nil {
		return nil, nil, err
	}
	colorStops := [][2]Value{first}
	var colorHints []Value
	previousWasColorStop := true

	for _, tokens := range colorStopsHints[1:] {
		if hint := parseColorHint(tokens); hint != nil {
			colorHints = append(colorHints, hint)
			previousWasColorStop = false
			continue
		}
		if previousWasColorStop {
			colorHints = append(colorHints, nil)
		}
		stop, err := parseColorStop(tokens)
		if err != nil {
			return nil, nil, err
		}
		colorStops = append(colorStops, stop)
		previousWasColorStop = true
	}

	if !previousWasColorStop {
		return nil, nil, errInvalid
	}
	return colorStops, colorHints, nil
}

// parseLinearGradientParameters is parse_linear_gradient_parameters in
// weasyprint/css/tokens.py.
func parseLinearGradientParameters(arguments [][]tinycss2.Node) (Tuple, [][]tinycss2.Node) {
	firstArg := arguments[0]
	if len(firstArg) == 1 {
		if angle, ok := getAngle(firstArg[0]); ok {
			return Tuple{String("angle"), Float(angle)}, arguments[1:]
		}
	} else {
		keywords := make([]string, len(firstArg))
		for i, token := range firstArg {
			keywords[i] = getKeyword(token)
			if keywords[i] == "" {
				// A token that is not a keyword is the None of upstream,
				// which no key holds.
				keywords[i] = "\x00"
			}
		}
		if result, ok := directionKeywords[strings.Join(keywords, " ")]; ok {
			return result, arguments[1:]
		}
	}
	// Default direction is 'to bottom'.
	return Tuple{String("angle"), Float(math.Pi)}, arguments
}

// parse2DPosition is parse_2d_position in weasyprint/css/tokens.py, the
// common syntax of background-position and transform-origin. It returns
// two values, or nil.
func parse2DPosition(tokens []tinycss2.Node) Tuple {
	switch len(tokens) {
	case 1:
		tokens = []tinycss2.Node{tokens[0], tinycss2.NewIdentToken(0, 0, "center")}
	case 2:
	default:
		return nil
	}

	token1, token2 := tokens[0], tokens[1]
	length1 := getLength(token1, true, true)
	length2 := getLength(token2, true, true)
	if length1 != nil && length2 != nil {
		return Tuple{length1, length2}
	}
	keyword1, keyword2 := getKeyword(tokens[0]), getKeyword(tokens[1])
	switch {
	case length1 != nil && in(keyword2, "top", "center", "bottom"):
		return Tuple{length1, backgroundPositionPercentages[keyword2]}
	case length2 != nil && in(keyword1, "left", "center", "right"):
		return Tuple{backgroundPositionPercentages[keyword1], length2}
	case in(keyword1, "left", "center", "right") && in(keyword2, "top", "center", "bottom"):
		return Tuple{backgroundPositionPercentages[keyword1], backgroundPositionPercentages[keyword2]}
	case in(keyword1, "top", "center", "bottom") && in(keyword2, "left", "center", "right"):
		// Swap tokens. They need to be in (horizontal, vertical) order.
		return Tuple{backgroundPositionPercentages[keyword2], backgroundPositionPercentages[keyword1]}
	}
	return nil
}

// in reports whether s is one of the strings, as the expression s in (a, b)
// of Python does. The empty string is the None of upstream, and is in no
// list.
func in(s string, strs ...string) bool {
	if s == "" {
		return false
	}
	for _, x := range strs {
		if s == x {
			return true
		}
	}
	return false
}

// parsePosition is parse_position in weasyprint/css/tokens.py, for
// background-position and object-position. It returns the tuple (origin_x,
// pos_x, origin_y, pos_y), or nil.
//
// See https://drafts.csswg.org/css-backgrounds-3/#the-background-position
// and https://drafts.csswg.org/css-images-3/#propdef-object-position.
func parsePosition(tokens []tinycss2.Node) Value {
	if result := parse2DPosition(tokens); result != nil {
		return Tuple{String("left"), result[0], String("top"), result[1]}
	}

	if len(tokens) == 4 {
		keyword1 := getKeyword(tokens[0])
		keyword2 := getKeyword(tokens[2])
		length1 := getLength(tokens[1], true, true)
		length2 := getLength(tokens[3], true, true)
		if length1 != nil && length2 != nil {
			if in(keyword1, "left", "right") && in(keyword2, "top", "bottom") {
				return Tuple{String(keyword1), length1, String(keyword2), length2}
			}
			if in(keyword2, "left", "right") && in(keyword1, "top", "bottom") {
				return Tuple{String(keyword2), length2, String(keyword1), length1}
			}
		}
	}

	if len(tokens) == 3 {
		var keyword, otherKeyword string
		length := getLength(tokens[2], true, true)
		if length != nil {
			keyword = getKeyword(tokens[1])
			otherKeyword = getKeyword(tokens[0])
		} else {
			length = getLength(tokens[1], true, true)
			otherKeyword = getKeyword(tokens[2])
			keyword = getKeyword(tokens[0])
		}

		if length != nil {
			switch {
			case otherKeyword == "center":
				if in(keyword, "top", "bottom") {
					return Tuple{String("left"), fiftyPercent, String(keyword), length}
				}
				if in(keyword, "left", "right") {
					return Tuple{String(keyword), length, String("top"), fiftyPercent}
				}
			case in(keyword, "left", "right") && in(otherKeyword, "top", "bottom"):
				return Tuple{String(keyword), length, String(otherKeyword), zeroPercent}
			case in(keyword, "top", "bottom") && in(otherKeyword, "left", "right"):
				return Tuple{String(otherKeyword), zeroPercent, String(keyword), length}
			}
		}
	}
	return nil
}

// radialGradientParameters is the result of
// parse_radial_gradient_parameters.
type radialGradientParameters struct {
	shape      string
	size       Tuple
	position   Value
	colorStops [][]tinycss2.Node
}

// parseRadialGradientParameters is parse_radial_gradient_parameters in
// weasyprint/css/tokens.py. It returns false for None.
func parseRadialGradientParameters(arguments [][]tinycss2.Node) (radialGradientParameters, bool) {
	var shape, sizeShape string
	var position Value
	var size Tuple
	// The stack holds the tokens of the first argument in reverse order.
	stack := make([]tinycss2.Node, len(arguments[0]))
	for i, token := range arguments[0] {
		stack[len(stack)-1-i] = token
	}
	for len(stack) > 0 {
		token := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		keyword := getKeyword(token)
		switch {
		case keyword == "at":
			rest := make([]tinycss2.Node, len(stack))
			for i, t := range stack {
				rest[len(rest)-1-i] = t
			}
			position = parsePosition(rest)
			if position == nil {
				return radialGradientParameters{}, false
			}
			stack = nil
		case in(keyword, "circle", "ellipse") && shape == "":
			shape = keyword
		case in(keyword, "closest-corner", "farthest-corner", "closest-side", "farthest-side") && size == nil:
			size = Tuple{String("keyword"), String(keyword)}
		default:
			if len(stack) > 0 && size == nil {
				length1 := getLength(token, true, true)
				length2 := getLength(stack[len(stack)-1], true, true)
				if length1 != nil && length2 != nil {
					size = Tuple{String("explicit"), Tuple{length1, length2}}
					sizeShape = "ellipse"
					stack = stack[:len(stack)-1]
				}
			}
			if size == nil {
				if length1 := getLength(token, true, false); length1 != nil {
					size = Tuple{String("explicit"), Tuple{length1, length1}}
					sizeShape = "circle"
				}
			}
			if size == nil {
				return radialGradientParameters{}, false
			}
		}
	}
	if shape == "circle" && sizeShape == "ellipse" {
		return radialGradientParameters{}, false
	}
	result := radialGradientParameters{
		shape:      shape,
		size:       size,
		position:   position,
		colorStops: arguments[1:],
	}
	if result.shape == "" {
		result.shape = sizeShape
	}
	if result.shape == "" {
		result.shape = "ellipse"
	}
	if result.size == nil {
		result.size = Tuple{String("keyword"), String("farthest-corner")}
	}
	if result.position == nil {
		result.position = Tuple{String("left"), fiftyPercent, String("top"), fiftyPercent}
	}
	return result, true
}

// splitOnComma is split_on_comma in weasyprint/css/tokens.py. It splits a
// list of tokens on the commas at the top level, not on the commas inside a
// function or a block.
func splitOnComma(tokens []tinycss2.Node) [][]tinycss2.Node {
	var parts [][]tinycss2.Node
	var thisPart []tinycss2.Node
	for _, token := range tokens {
		if tinycss2.IsLiteral(token, ",") {
			parts = append(parts, thisPart)
			thisPart = nil
		} else {
			thisPart = append(thisPart, token)
		}
	}
	return append(parts, thisPart)
}

// removeWhitespace is remove_whitespace in weasyprint/css/tokens.py. It
// removes the whitespace and the comments at the top level of a list of
// tokens.
func removeWhitespace(tokens []tinycss2.Node) []tinycss2.Node {
	result := make([]tinycss2.Node, 0, len(tokens))
	for _, token := range tokens {
		if t := token.Type(); t != tinycss2.TypeWhitespace && t != tinycss2.TypeComment {
			result = append(result, token)
		}
	}
	return result
}

// getKeyword is get_keyword in weasyprint/css/tokens.py. It returns the
// lower case name of a keyword, or "" for None.
func getKeyword(token tinycss2.Node) string {
	if ident, ok := token.(*tinycss2.IdentToken); ok {
		return ident.LowerValue
	}
	return ""
}

// getCustomIdent is get_custom_ident in weasyprint/css/tokens.py. It
// returns the name of a keyword, or "" for None.
func getCustomIdent(token tinycss2.Node) string {
	if ident, ok := token.(*tinycss2.IdentToken); ok {
		return ident.Value
	}
	return ""
}

// getSingleKeyword is get_single_keyword in weasyprint/css/tokens.py. It
// returns the lower case name of a list of one keyword, or "" for None.
func getSingleKeyword(tokens []tinycss2.Node) string {
	if len(tokens) == 1 {
		return getKeyword(tokens[0])
	}
	return ""
}

// getNumber is get_number in weasyprint/css/tokens.py. It parses a
// <number> token, and returns a Dimension without unit, or nil.
func getNumber(token tinycss2.Node, negative, integer bool) Value {
	if checkMathToken(token) {
		resolved, err := resolveMath(token, nil, "", nil)
		if err != nil || resolved == nil {
			return nil
		}
		number, ok := resolved.(*tinycss2.NumberToken)
		if !ok {
			return nil
		}
		value := number.Value
		if !negative && value < 0 {
			value = 0
		}
		if integer {
			// TODO: always round x.5 to +inf, see
			// https://drafts.csswg.org/css-values-4/#combine-integers.
			value = math.RoundToEven(value)
		}
		return Dimension{value, ""}
	}
	if number, ok := token.(*tinycss2.NumberToken); ok {
		if integer {
			if number.IsInteger && (negative || number.IntValue >= 0) {
				return Dimension{float64(number.IntValue), ""}
			}
		} else if negative || number.Value >= 0 {
			return Dimension{number.Value, ""}
		}
	}
	return nil
}

// getString is get_string in weasyprint/css/tokens.py. It parses a <string>
// token, and returns ('string', value) or the result of a function, or nil.
func getString(token tinycss2.Node) Value {
	switch t := token.(type) {
	case *tinycss2.StringToken:
		return Tuple{String("string"), String(t.Value)}
	case *tinycss2.FunctionBlock:
		switch t.Name {
		case "attr":
			return checkAttr(token, "string")
		case "counter", "counters":
			return checkCounter(token)
		case "content":
			return checkContent(token)
		case "string":
			return checkStringOrElement("string", token)
		}
	}
	return nil
}

// getPercentage is get_percentage in weasyprint/css/tokens.py. It parses a
// <percentage> token, and returns a Dimension, or nil.
func getPercentage(token tinycss2.Node, negative bool) Value {
	if checkMathToken(token) {
		resolved, err := resolveMath(token, nil, "", nil)
		if err != nil {
			return nil
		}
		if resolved != nil {
			token = resolved
		}
		// Range clamp percentages.
		if p, ok := token.(*tinycss2.PercentageToken); ok && !negative {
			p.Value = pyMax0(p.Value)
		}
	}
	if p, ok := token.(*tinycss2.PercentageToken); ok && (negative || p.Value >= 0) {
		return Dimension{p.Value, "%"}
	}
	return nil
}

// getLength is get_length in weasyprint/css/tokens.py. It parses a <length>
// token, and returns a Dimension, a Token of a math function that waits for
// the computed style, or nil.
func getLength(token tinycss2.Node, negative, percentage bool) Value {
	if checkMathToken(token) {
		resolved, err := resolveMath(token, nil, "", nil)
		switch {
		case errors.Is(err, errPercentageInMath):
			// PercentageInMath is raised in priority to help discarding
			// percentages for properties that do not allow them.
			if percentage {
				return Token{token}
			}
			return nil
		case errors.Is(err, errRelativeLengthInMath):
			return Token{token}
		case err != nil:
			return nil
		}
		if resolved != nil {
			token = resolved
		}
		// Range clamp.
		if t := token.Type(); !negative && t != tinycss2.TypeFunction && t != tinycss2.TypeNumber {
			if v, ok := tokenNumber(token); ok {
				setTokenNumber(token, pyMax0(v))
			}
		}
	}
	switch t := token.(type) {
	case *tinycss2.PercentageToken:
		if percentage && (negative || t.Value >= 0) {
			return Dimension{t.Value, "%"}
		}
	case *tinycss2.DimensionToken:
		if lengthUnits[t.LowerUnit] && (negative || t.Value >= 0) {
			return Dimension{t.Value, t.LowerUnit}
		}
	case *tinycss2.NumberToken:
		if t.Value == 0 {
			return Dimension{0, ""}
		}
	}
	return nil
}

// tokenNumber returns the value of a number, a percentage or a dimension
// token.
func tokenNumber(token tinycss2.Node) (float64, bool) {
	switch t := token.(type) {
	case *tinycss2.NumberToken:
		return t.Value, true
	case *tinycss2.PercentageToken:
		return t.Value, true
	case *tinycss2.DimensionToken:
		return t.Value, true
	}
	return 0, false
}

// setTokenNumber sets the value of a number, a percentage or a dimension
// token, as `token.value = value` of upstream does.
func setTokenNumber(token tinycss2.Node, value float64) {
	switch t := token.(type) {
	case *tinycss2.NumberToken:
		t.Value = value
	case *tinycss2.PercentageToken:
		t.Value = value
	case *tinycss2.DimensionToken:
		t.Value = value
	}
}

// tokenUnit returns the lower case unit of a dimension token, or "".
func tokenUnit(token tinycss2.Node) string {
	if d, ok := token.(*tinycss2.DimensionToken); ok {
		return d.LowerUnit
	}
	return ""
}

// getAngle is get_angle in weasyprint/css/tokens.py. It parses an <angle>
// token, and returns radians, and false for None.
func getAngle(token tinycss2.Node) (float64, bool) {
	resolved, err := resolveMath(token, nil, "", nil)
	if err != nil {
		return 0, false
	}
	if resolved != nil {
		token = resolved
	}
	switch t := token.(type) {
	case *tinycss2.NumberToken:
		if t.Value == 0 {
			// Legacy syntax: https://drafts.csswg.org/css-values-4/#angles.
			return 0, true
		}
	case *tinycss2.DimensionToken:
		if factor, ok := angleToRadians[t.LowerUnit]; ok {
			return t.Value * factor, true
		}
	}
	return 0, false
}

// getResolution is get_resolution in weasyprint/css/tokens.py. It parses a
// <resolution> token, and returns dppx, and false for None.
func getResolution(token tinycss2.Node) (float64, bool) {
	resolved, err := resolveMath(token, nil, "", nil)
	if err != nil {
		return 0, false
	}
	if resolved != nil {
		token = resolved
	}
	if t, ok := token.(*tinycss2.DimensionToken); ok {
		if factor, ok := resolutionToDPPX[t.LowerUnit]; ok {
			return t.Value * factor, true
		}
	}
	return 0, false
}

// getImage is get_image in weasyprint/css/tokens.py. It parses an <image>
// token, and returns ('url', url), ('linear-gradient', gradient) or
// ('radial-gradient', gradient), or nil.
func getImage(token tinycss2.Node, baseURL string) (Value, error) {
	parsedURL, err := getURL(token, baseURL)
	if err != nil {
		return nil, err
	}
	if parsedURL, ok := parsedURL.(Tuple); ok && isStr(parsedURL[0], "url") {
		if u, ok := parsedURL[1].(Tuple); ok && isStr(u[0], "external") {
			return Tuple{String("url"), u[1]}, nil
		}
	}
	f := newFunction(token)
	arguments := f.splitCommaGroups(false)
	if len(arguments) == 0 {
		return nil, nil
	}
	repeating := strings.HasPrefix(f.name, "repeating-")
	switch f.name {
	case "linear-gradient", "repeating-linear-gradient":
		direction, colorStopTokens := parseLinearGradientParameters(arguments)
		colorStops, colorHints, err := parseColorStopsAndHints(colorStopTokens)
		if err != nil {
			return nil, err
		}
		return Tuple{String("linear-gradient"), newLinearGradient(colorStops, direction, repeating, colorHints)}, nil
	case "radial-gradient", "repeating-radial-gradient":
		result, ok := parseRadialGradientParameters(arguments)
		if !ok {
			result = radialGradientParameters{
				shape:      "ellipse",
				size:       Tuple{String("keyword"), String("farthest-corner")},
				position:   Tuple{String("left"), fiftyPercent, String("top"), fiftyPercent},
				colorStops: arguments,
			}
		}
		colorStops, colorHints, err := parseColorStopsAndHints(result.colorStops)
		if err != nil {
			return nil, err
		}
		return Tuple{String("radial-gradient"), newRadialGradient(
			colorStops, result.shape, result.size, result.position, repeating, colorHints)}, nil
	}
	return nil, nil
}

// tokenValue returns the attribute value of a token that has one, such as a
// string or an ident token.
func tokenValue(token tinycss2.Node) (string, bool) {
	switch t := token.(type) {
	case *tinycss2.StringToken:
		return t.Value, true
	case *tinycss2.IdentToken:
		return t.Value, true
	case *tinycss2.URLToken:
		return t.Value, true
	case *tinycss2.WhitespaceToken:
		return t.Value, true
	case *tinycss2.LiteralToken:
		return t.Value, true
	case *tinycss2.HashToken:
		return t.Value, true
	case *tinycss2.AtKeywordToken:
		return t.Value, true
	case *tinycss2.Comment:
		return t.Value, true
	}
	return "", false
}

// getURL is get_url in weasyprint/css/tokens.py. It parses a <url> token,
// and returns ('url', ('internal', name)), ('url', ('external', url)), the
// result of attr(), or nil.
func getURL(token tinycss2.Node, baseURL string) (Value, error) {
	var u Value
	switch t := token.(type) {
	case *tinycss2.URLToken:
		u = getURLTuple(t.Value, baseURL)
	case *tinycss2.FunctionBlock:
		switch {
		case t.Name == "attr":
			return checkAttr(token, "url"), nil
		case t.Name == "url" && (len(t.Arguments) == 1 || len(t.Arguments) == 2):
			// Ignore url modifiers.
			// See https://drafts.csswg.org/css-values-3/#urls
			value, ok := tokenValue(t.Arguments[0])
			if !ok {
				// Port: upstream raises AttributeError for a block.
				return nil, nil
			}
			u = getURLTuple(value, baseURL)
		default:
			return nil, nil
		}
	default:
		return nil, nil
	}
	if u == nil {
		return nil, invalidValues("Relative URI reference without a base URI: None")
	}
	return Tuple{String("url"), u}, nil
}

// getQuote is get_quote in weasyprint/css/tokens.py. It parses a <quote>
// token, and returns the keyword, or "" for None.
func getQuote(token tinycss2.Node) string {
	keyword := getKeyword(token)
	if in(keyword, "open-quote", "close-quote", "no-open-quote", "no-close-quote") {
		return keyword
	}
	return ""
}

// getTarget is get_target in weasyprint/css/tokens.py. It parses a <target>
// token, and returns ('target-counter()', values), ('target-counters()',
// values) or ('target-text()', values), or nil.
func getTarget(token tinycss2.Node, baseURL string) (Value, error) {
	f := newFunction(token)
	arguments := f.splitComma(false)
	switch f.name {
	case "target-counter":
		if len(arguments) != 2 && len(arguments) != 3 {
			return nil, nil
		}
	case "target-counters":
		if len(arguments) != 3 && len(arguments) != 4 {
			return nil, nil
		}
	case "target-text":
		if len(arguments) != 1 && len(arguments) != 2 {
			return nil, nil
		}
	default:
		return nil, nil
	}

	var values Tuple

	link := arguments[0]
	arguments = arguments[1:]
	if stringLink := getString(link); stringLink != nil {
		values = append(values, stringLink)
	} else {
		u, err := getURL(link, baseURL)
		if err != nil {
			return nil, err
		}
		if u == nil {
			return nil, nil
		}
		values = append(values, u)
	}

	if strings.HasPrefix(f.name, "target-counter") {
		ident, ok := arguments[0].(*tinycss2.IdentToken)
		if !ok {
			return nil, nil
		}
		arguments = arguments[1:]
		values = append(values, String(ident.Value))

		if f.name == "target-counters" {
			s := getString(arguments[0])
			if s == nil {
				return nil, nil
			}
			arguments = arguments[1:]
			values = append(values, s)
		}

		counterStyle := "decimal"
		if len(arguments) > 0 {
			counterStyle = getKeyword(arguments[0])
		}
		values = append(values, keywordOrNone(counterStyle))
	} else {
		content := "content"
		if len(arguments) > 0 {
			content = getKeyword(arguments[0])
			if !in(content, "content", "before", "after", "first-letter") {
				return nil, nil
			}
		}
		values = append(values, String(content))
	}

	return Tuple{String(f.name + "()"), values}, nil
}

// keywordOrNone returns the String of a keyword, or nil for the "" of None.
func keywordOrNone(keyword string) Value {
	if keyword == "" {
		return nil
	}
	return String(keyword)
}

// getContentList is get_content_list in weasyprint/css/tokens.py. It parses
// <content-list> tokens, and returns a Tuple, or nil. See
// https://www.w3.org/TR/css-content-3/#typedef-content-list.
func getContentList(tokens []tinycss2.Node, baseURL string) (Value, error) {
	parsedTokens := make(Tuple, len(tokens))
	for i, token := range tokens {
		parsed, err := getContentListToken(token, baseURL)
		if err != nil {
			return nil, err
		}
		if parsed == nil {
			return nil, nil
		}
		parsedTokens[i] = parsed
	}
	return parsedTokens, nil
}

// getContentListToken is get_content_list_token in
// weasyprint/css/tokens.py. It parses one of the <content-list> tokens. See
// https://drafts.csswg.org/css-content-3/#content-values.
func getContentListToken(token tinycss2.Node, baseURL string) (Value, error) {
	// <string>
	if s := getString(token); s != nil {
		return s, nil
	}

	// contents
	if getKeyword(token) == "contents" {
		return Tuple{String("content()"), String("text")}, nil
	}

	// <uri>
	u, err := getURL(token, baseURL)
	if err != nil {
		return nil, err
	}
	if u != nil {
		return u, nil
	}

	// <quote>
	if quote := getQuote(token); quote != "" {
		return Tuple{String("quote"), String(quote)}, nil
	}

	// <target>
	target, err := getTarget(token, baseURL)
	if err != nil {
		return nil, err
	}
	if target != nil {
		return target, nil
	}

	f := newFunction(token)
	arguments := f.splitComma(false)

	switch f.name {
	case "leader":
		// <leader()>
		if len(arguments) != 1 {
			return nil, nil
		}
		var s string
		switch arg := arguments[0].(type) {
		case *tinycss2.IdentToken:
			switch arg.Value {
			case "dotted":
				s = "."
			case "solid":
				s = "_"
			case "space":
				s = " "
			default:
				return nil, nil
			}
		case *tinycss2.StringToken:
			s = arg.Value
		default:
			// Port: upstream raises UnboundLocalError.
			return nil, nil
		}
		return Tuple{String("leader()"), Tuple{String("string"), String(s)}}, nil
	case "element":
		// <element()>
		return checkStringOrElement("element", token), nil
	}
	return nil, nil
}

// validatorFunc is the type of the validators of the properties in
// weasyprint/css/validation/properties.py, and of the functions that their
// decorators wrap. It returns the validated value, or nil for the None of
// upstream, which is an invalid value. An error is an InvalidValues that
// upstream raises.
//
// Upstream passes base_url only to the validators that want it. The port
// passes it to each one.
type validatorFunc func(tokens []tinycss2.Node, baseURL string) (Value, error)

// singleKeyword is single_keyword in weasyprint/css/tokens.py, the decorator
// of the validators that only accept a single keyword.
func singleKeyword(f func(keyword string) bool) validatorFunc {
	return func(tokens []tinycss2.Node, _ string) (Value, error) {
		keyword := getSingleKeyword(tokens)
		if f(keyword) {
			return String(keyword), nil
		}
		return nil, nil
	}
}

// singleToken is single_token in weasyprint/css/tokens.py, the decorator of
// the validators that only accept a single token.
func singleToken(f func(token tinycss2.Node, baseURL string) (Value, error)) validatorFunc {
	return func(tokens []tinycss2.Node, baseURL string) (Value, error) {
		if len(tokens) == 1 {
			return f(tokens[0], baseURL)
		}
		return nil, nil
	}
}

// commaSeparatedList is comma_separated_list in weasyprint/css/tokens.py,
// the decorator of the validators that accept a comma separated list. The
// single_value attribute of upstream is the function f itself.
func commaSeparatedList(f validatorFunc) validatorFunc {
	return func(tokens []tinycss2.Node, baseURL string) (Value, error) {
		parts := splitOnComma(tokens)
		results := make(Tuple, 0, len(parts))
		for _, part := range parts {
			result, err := f(removeWhitespace(part), baseURL)
			if err != nil {
				return nil, err
			}
			if result == nil {
				return nil, nil
			}
			results = append(results, result)
		}
		return results, nil
	}
}

// tokenizeItem is the item that tokenize takes: a dimension, a percentage
// or a number token, a Dimension, or a number.
type tokenizeItem any

// tokenize is tokenize in weasyprint/css/tokens.py. It turns a computed
// value into a token. fn, when not nil, transforms the value, and unit is
// the unit of a number, "" for None.
func tokenize(item tokenizeItem, fn func(float64) float64, unit string) tinycss2.Node {
	apply := func(v float64) float64 {
		if fn != nil {
			return fn(v)
		}
		return v
	}
	switch t := item.(type) {
	case *tinycss2.DimensionToken:
		value := apply(t.Value)
		return tinycss2.NewDimensionToken(0, 0, value, 0, false, pyStr(value), t.LowerUnit)
	case Dimension:
		value := apply(t.Value)
		return tinycss2.NewDimensionToken(0, 0, value, 0, false, pyStr(value), strings.ToLower(t.Unit))
	case *tinycss2.PercentageToken:
		value := apply(t.Value)
		return tinycss2.NewPercentageToken(0, 0, value, 0, false, pyStr(value))
	}
	var value float64
	switch t := item.(type) {
	case *tinycss2.NumberToken:
		value = t.Value
	case float64:
		value = t
	case int:
		value = float64(t)
	default:
		return nil
	}
	value = apply(value)
	isInteger := value == math.Trunc(value) && !math.IsInf(value, 0)
	var intValue int64
	representation := pyStr(value)
	if isInteger {
		intValue = saturateInt64(value)
		representation = strconv.FormatFloat(value, 'f', -1, 64)
	}
	switch unit {
	case "":
		return tinycss2.NewNumberToken(0, 0, value, intValue, isInteger, representation)
	case "%":
		return tinycss2.NewPercentageToken(0, 0, value, intValue, isInteger, representation)
	}
	return tinycss2.NewDimensionToken(0, 0, value, intValue, isInteger, representation, unit)
}

// saturateInt64 converts a float to an int64, and saturates out of range.
func saturateInt64(v float64) int64 {
	switch {
	case v >= math.MaxInt64:
		return math.MaxInt64
	case v <= math.MinInt64:
		return math.MinInt64
	}
	return int64(v)
}

// pyStr formats a float as str of Python does.
func pyStr(v float64) string {
	switch {
	case math.IsInf(v, 1):
		return "inf"
	case math.IsInf(v, -1):
		return "-inf"
	case math.IsNaN(v):
		return "nan"
	case v == math.Trunc(v) && math.Abs(v) < 1e16:
		return strconv.FormatFloat(v, 'f', 1, 64)
	}
	return strconv.FormatFloat(v, 'g', -1, 64)
}

// pyMax0 is max(0, v) of Python: it returns 0 unless v is greater, so that
// NaN gives 0. The max of Go gives NaN.
func pyMax0(v float64) float64 {
	if v > 0 {
		return v
	}
	return 0
}
