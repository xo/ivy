// Copyright (c) 2011-2021, Simon Sapin and contributors.
//
// Use of this source code is governed by the BSD-3-Clause license that can be
// found in the LICENSE file.

// This file ports weasyprint/css/computed_values.py: it converts the
// specified values of the properties into computed values.

package layout

import (
	"math"
	"strings"
)

// fontSizeKeywords is FONT_SIZE_KEYWORDS in
// weasyprint/css/computed_values.py: the font size in pixels of the
// <absolute-size> keywords. medium is 12pt (16px), and the other keywords
// have the scaling factors of CSS3:
// https://www.w3.org/TR/css-fonts-3/#font-size-prop
var fontSizeKeywords = []struct {
	name string
	size float64
}{
	{"xx-small", 16 * 3.0 / 5},
	{"x-small", 16 * 3.0 / 4},
	{"small", 16 * 8.0 / 9},
	{"medium", 16},
	{"large", 16 * 6.0 / 5},
	{"x-large", 16 * 3.0 / 2},
	{"xx-large", 16 * 2},
}

// fontSizeKeyword returns the size of a keyword of fontSizeKeywords.
func fontSizeKeyword(name string) (float64, bool) {
	for _, k := range fontSizeKeywords {
		if k.name == name {
			return k.size, true
		}
	}
	return 0, false
}

// borderWidthKeywords is BORDER_WIDTH_KEYWORDS in
// weasyprint/css/computed_values.py, in pixels. The values are not
// specified, other than 'thin' <= 'medium' <= 'thick'.
var borderWidthKeywords = map[string]int{
	"thin":   1,
	"medium": 3,
	"thick":  5,
}

// fontWeightRelative is FONT_WEIGHT_RELATIVE in
// weasyprint/css/computed_values.py. See
// https://www.w3.org/TR/CSS21/fonts.html#propdef-font-weight.
var fontWeightRelative = map[string]map[int]int{
	"bolder": {
		100: 400,
		200: 400,
		300: 400,
		400: 700,
		500: 700,
		600: 900,
		700: 900,
		800: 900,
		900: 900,
	},
	"lighter": {
		100: 100,
		200: 100,
		300: 100,
		400: 100,
		500: 100,
		600: 400,
		700: 400,
		800: 700,
		900: 700,
	},
}

// PageSizes is PAGE_SIZES in weasyprint/css/computed_values.py, in the
// "portrait" orientation. See https://www.w3.org/TR/css-page-3/#size.
var PageSizes = map[string][2]Dimension{
	"a10":     {Dimension{26, "mm"}, Dimension{37, "mm"}},
	"a9":      {Dimension{37, "mm"}, Dimension{52, "mm"}},
	"a8":      {Dimension{52, "mm"}, Dimension{74, "mm"}},
	"a7":      {Dimension{74, "mm"}, Dimension{105, "mm"}},
	"a6":      {Dimension{105, "mm"}, Dimension{148, "mm"}},
	"a5":      {Dimension{148, "mm"}, Dimension{210, "mm"}},
	"a4":      {Dimension{210, "mm"}, Dimension{297, "mm"}},
	"a3":      {Dimension{297, "mm"}, Dimension{420, "mm"}},
	"a2":      {Dimension{420, "mm"}, Dimension{594, "mm"}},
	"a1":      {Dimension{594, "mm"}, Dimension{841, "mm"}},
	"a0":      {Dimension{841, "mm"}, Dimension{1189, "mm"}},
	"b10":     {Dimension{31, "mm"}, Dimension{44, "mm"}},
	"b9":      {Dimension{44, "mm"}, Dimension{62, "mm"}},
	"b8":      {Dimension{62, "mm"}, Dimension{88, "mm"}},
	"b7":      {Dimension{88, "mm"}, Dimension{125, "mm"}},
	"b6":      {Dimension{125, "mm"}, Dimension{176, "mm"}},
	"b5":      {Dimension{176, "mm"}, Dimension{250, "mm"}},
	"b4":      {Dimension{250, "mm"}, Dimension{353, "mm"}},
	"b3":      {Dimension{353, "mm"}, Dimension{500, "mm"}},
	"b2":      {Dimension{500, "mm"}, Dimension{707, "mm"}},
	"b1":      {Dimension{707, "mm"}, Dimension{1000, "mm"}},
	"b0":      {Dimension{1000, "mm"}, Dimension{1414, "mm"}},
	"c10":     {Dimension{28, "mm"}, Dimension{40, "mm"}},
	"c9":      {Dimension{40, "mm"}, Dimension{57, "mm"}},
	"c8":      {Dimension{57, "mm"}, Dimension{81, "mm"}},
	"c7":      {Dimension{81, "mm"}, Dimension{114, "mm"}},
	"c6":      {Dimension{114, "mm"}, Dimension{162, "mm"}},
	"c5":      {Dimension{162, "mm"}, Dimension{229, "mm"}},
	"c4":      {Dimension{229, "mm"}, Dimension{324, "mm"}},
	"c3":      {Dimension{324, "mm"}, Dimension{458, "mm"}},
	"c2":      {Dimension{458, "mm"}, Dimension{648, "mm"}},
	"c1":      {Dimension{648, "mm"}, Dimension{917, "mm"}},
	"c0":      {Dimension{917, "mm"}, Dimension{1297, "mm"}},
	"jis-b10": {Dimension{32, "mm"}, Dimension{45, "mm"}},
	"jis-b9":  {Dimension{45, "mm"}, Dimension{64, "mm"}},
	"jis-b8":  {Dimension{64, "mm"}, Dimension{91, "mm"}},
	"jis-b7":  {Dimension{91, "mm"}, Dimension{128, "mm"}},
	"jis-b6":  {Dimension{128, "mm"}, Dimension{182, "mm"}},
	"jis-b5":  {Dimension{182, "mm"}, Dimension{257, "mm"}},
	"jis-b4":  {Dimension{257, "mm"}, Dimension{364, "mm"}},
	"jis-b3":  {Dimension{364, "mm"}, Dimension{515, "mm"}},
	"jis-b2":  {Dimension{515, "mm"}, Dimension{728, "mm"}},
	"jis-b1":  {Dimension{728, "mm"}, Dimension{1030, "mm"}},
	"jis-b0":  {Dimension{1030, "mm"}, Dimension{1456, "mm"}},
	"letter":  {Dimension{8.5, "in"}, Dimension{11, "in"}},
	"legal":   {Dimension{8.5, "in"}, Dimension{14, "in"}},
	"ledger":  {Dimension{11, "in"}, Dimension{17, "in"}},
}

// initialPageSize is INITIAL_PAGE_SIZE in
// weasyprint/css/computed_values.py.
var initialPageSize = PageSizes["a4"]

func init() {
	InitialValues["size"] = Tuple{
		Float(toPixels(initialPageSize[0], nil, "size", -1)),
		Float(toPixels(initialPageSize[1], nil, "size", -1)),
	}
}

// physicalFunctions is PHYSICAL_FUNCTIONS in
// weasyprint/css/computed_values.py. It maps a physical property to a
// function that returns its logical property, from the block and the inline
// directions.
var physicalFunctions = map[string]func(block, inline string) string{}

// registerLogical is register_logical in
// weasyprint/css/computed_values.py. It registers the logical properties of
// the physical names, with each prefix and each suffix. An empty prefix or
// suffix is none.
func registerLogical(f func(name, prefix, suffix, block, inline string) string, names, prefixes, suffixes []string) {
	for _, name := range names {
		name = strings.ReplaceAll(name, "-", "_")
		for _, prefix := range prefixes {
			for _, suffix := range suffixes {
				propertyName := name
				if prefix != "" {
					propertyName = prefix + "_" + propertyName
				}
				if suffix != "" {
					propertyName = propertyName + "_" + suffix
				}
				physicalFunctions[propertyName] = func(block, inline string) string {
					return f(name, prefix, suffix, block, inline)
				}
			}
		}
	}
}

func init() {
	sides := []string{"top", "left", "bottom", "right"}
	registerLogical(physicalSize, []string{"width", "height"}, []string{"", "max", "min"}, []string{""})
	registerLogical(physicalInset, sides, []string{"", "padding", "margin"}, []string{""})
	registerLogical(physicalInset, sides, []string{"border"}, []string{"width", "style", "color"})
	registerLogical(physicalRadius,
		[]string{"top_left", "top_right", "bottom_left", "bottom_right"},
		[]string{"border"}, []string{"radius"})
}

// physicalSize is physical_size in weasyprint/css/computed_values.py.
func physicalSize(name, prefix, _, block, _ string) string {
	verticalMainDirection := block == "ttb" || block == "btt"
	verticalProperty := strings.Contains(name, "height")
	logical := "inline"
	if verticalProperty == verticalMainDirection {
		logical = "block"
	}
	if prefix != "" {
		return prefix + "_" + logical + "_size"
	}
	return logical + "_size"
}

// physicalInset is physical_inset in weasyprint/css/computed_values.py.
func physicalInset(name, prefix, suffix, block, inline string) string {
	var logical, side string
	pick := func(cond bool, a, b string) string {
		if cond {
			return a
		}
		return b
	}
	switch name {
	case "top":
		logical = pick(block == "ttb" || block == "btt", "block", "inline")
		side = pick(block == "ttb" || inline == "ttb", "start", "end")
	case "bottom":
		logical = pick(block == "ttb" || block == "btt", "block", "inline")
		side = pick(block == "btt" || inline == "btt", "start", "end")
	case "left":
		logical = pick(block == "ltr" || block == "rtl", "block", "inline")
		side = pick(block == "ltr" || inline == "ltr", "start", "end")
	case "right":
		logical = pick(block == "ltr" || block == "rtl", "block", "inline")
		side = pick(block == "rtl" || inline == "rtl", "start", "end")
	}
	if prefix == "" {
		prefix = "inset"
	}
	prefix += "_"
	if suffix != "" {
		suffix = "_" + suffix
	}
	return prefix + logical + "_" + side + suffix
}

// physicalRadius is physical_radius in weasyprint/css/computed_values.py.
func physicalRadius(name, prefix, suffix, block, inline string) string {
	vertical, horizontal, _ := strings.Cut(name, "_")
	resolve := func(direction string) string {
		switch direction {
		case "ttb":
			if vertical == "top" {
				return "start"
			}
			return "end"
		case "btt":
			if vertical == "bottom" {
				return "start"
			}
			return "end"
		case "ltr":
			if horizontal == "left" {
				return "start"
			}
			return "end"
		case "rtl":
			if horizontal == "right" {
				return "start"
			}
			return "end"
		}
		return direction
	}
	return prefix + "_" + resolve(block) + "_" + resolve(inline) + "_" + suffix
}

// computerFunc is the type of the functions of COMPUTER_FUNCTIONS: it
// returns the computed value of a property from its specified value.
type computerFunc func(style *Style, name string, value Value) Value

// computerFunctions is COMPUTER_FUNCTIONS in
// weasyprint/css/computed_values.py. It maps the name of a property, in
// snake case, to the function that computes its value.
var computerFunctions = map[string]computerFunc{}

// registerComputer is register_computer in
// weasyprint/css/computed_values.py.
func registerComputer(f computerFunc, names ...string) {
	for _, name := range names {
		computerFunctions[strings.ReplaceAll(name, "-", "_")] = f
	}
}

func init() {
	registerComputer(computeBackgroundImage, "background-image")
	registerComputer(computeImage, "border-image-source")
	registerComputer(computeColor,
		"color", "background-color", "border-top-color", "border-right-color",
		"border-bottom-color", "border-left-color", "column-rule-color",
		"outline-color", "text-decoration-color")
	registerComputer(computeBoxShadow, "box-shadow")
	registerComputer(computePosition, "background-position", "object-position")
	registerComputer(computeLengthOrPercentageTuple, "transform-origin")
	registerComputer(computeLengthTuple, "border-spacing", "size", "clip")
	registerComputer(computeBreakBeforeAfter, "break-after", "break-before")
	registerComputer(computeLengthProperty,
		"top", "right", "left", "bottom", "margin-top", "margin-right",
		"margin-bottom", "margin-left", "height", "width", "block-size",
		"inline-size", "min-width", "min-height", "min-block-size",
		"min-inline-size", "max-width", "max-height", "max-block-size",
		"max-inline-size", "padding-top", "padding-right", "padding-bottom",
		"padding-left", "text-indent", "hyphenate-limit-zone", "flex-basis",
		"text-underline-offset", "text-decoration-thickness")
	registerComputer(computeBleed, "bleed-left", "bleed-right", "bleed-top", "bleed-bottom")
	registerComputer(computePixelLength, "letter-spacing")
	registerComputer(computeBackgroundSize, "background-size")
	registerComputer(computeImageOrientation, "image-orientation")
	registerComputer(computeBorderWidth,
		"border-top-width", "border-right-width", "border-left-width",
		"border-bottom-width", "column-rule-width", "outline-width")
	registerComputer(computeBorderImageSlice, "border-image-slice", "mask-border-slice")
	registerComputer(computeBorderImageWidth, "border-image-width", "mask-border-width")
	registerComputer(computeBorderImageOutset, "border-image-outset", "mask-border-outset")
	registerComputer(computeBorderImageRepeat, "border-image-repeat", "mask-border-repeat")
	registerComputer(computeLengthPixelsOnly, "column-width", "outline-offset")
	registerComputer(computeBorderRadius,
		"border-top-left-radius", "border-top-right-radius",
		"border-bottom-left-radius", "border-bottom-right-radius")
	registerComputer(computeGap, "column-gap", "row-gap")
	registerComputer(computeBookmarkLabel, "bookmark-label")
	registerComputer(computeStringSet, "string-set")
	registerComputer(computeContent, "content")
	registerComputer(computeDisplay, "display")
	registerComputer(computeFloat, "float")
	registerComputer(computeFontSize, "font-size")
	registerComputer(computeFontWeight, "font-weight")
	registerComputer(computeGridTemplate, "grid-template-columns", "grid-template-rows")
	registerComputer(computeGridAuto, "grid-auto-columns", "grid-auto-rows")
	registerComputer(computeLineHeight, "line-height")
	registerComputer(computeLang, "lang")
	registerComputer(computeTabSize, "tab-size")
	registerComputer(computeTabStops, "tab-stops")
	registerComputer(computeTransform, "transform")
	registerComputer(computeVerticalAlign, "vertical-align")
	registerComputer(computeWordSpacing, "word-spacing")
}

// computeBackgroundImage is background_image in
// weasyprint/css/computed_values.py. It computes the lengths of the
// gradients of background-image.
func computeBackgroundImage(style *Style, name string, values Value) Value {
	t, ok := values.(Tuple)
	if !ok {
		return values
	}
	result := make(Tuple, len(t))
	for i, value := range t {
		result[i] = computeImage(style, name, value)
	}
	return result
}

// computeImage is image in weasyprint/css/computed_values.py. It computes
// the lengths of a gradient of border-image-source.
//
// As upstream does, it changes the gradient, which the styles that share
// the declaration share.
func computeImage(style *Style, name string, image Value) Value {
	t, ok := image.(Tuple)
	if !ok || len(t) != 2 {
		return image
	}
	computeGradient := func(g *Gradient) {
		for i, pos := range g.StopPositions {
			if pos != nil {
				g.StopPositions[i] = computeLength(style, name, pos, -1, false)
			}
		}
		for i, hint := range g.ColorHints {
			if hint != nil {
				g.ColorHints[i] = computeLength(style, name, hint, -1, false)
			}
		}
	}
	switch value := t[1].(type) {
	case *LinearGradient:
		computeGradient(&value.Gradient)
	case *RadialGradient:
		computeGradient(&value.Gradient)
		if centers, ok := computePosition(style, name, Tuple{value.Center}).(Tuple); ok && len(centers) == 1 {
			value.Center = centers[0]
		}
		if value.SizeType == "explicit" {
			value.Size = computeLengthOrPercentageTuple(style, name, value.Size)
		}
	}
	return image
}

// colorSchemes returns the color schemes of the value of color-scheme, for
// parseColor.
func colorSchemes(v Value) []string {
	switch v := v.(type) {
	case String:
		return []string{string(v)}
	case Tuple:
		schemes := make([]string, 0, len(v))
		for _, item := range v {
			schemes = append(schemes, str(item))
		}
		return schemes
	}
	return nil
}

// computeColor is color in weasyprint/css/computed_values.py.
func computeColor(style *Style, _ string, values Value) Value {
	return parseColor(values, colorSchemes(style.Get("color_scheme"))...)
}

// computeBoxShadow is box_shadow in weasyprint/css/computed_values.py.
func computeBoxShadow(style *Style, name string, values Value) Value {
	t, ok := values.(Tuple)
	if !ok {
		return values
	}
	result := make(Tuple, 0, len(t))
	for _, item := range t {
		shadow, ok := item.(Tuple)
		if !ok || len(shadow) != 6 {
			continue
		}
		result = append(result, Tuple{
			computeLength(style, name, shadow[0], -1, false),
			computeLength(style, name, shadow[1], -1, false),
			computeLength(style, name, shadow[2], -1, false),
			computeLength(style, name, shadow[3], -1, false),
			parseColor(shadow[4], colorSchemes(style.Get("color_scheme"))...),
			shadow[5],
		})
	}
	return result
}

// computePosition is compute_position in weasyprint/css/computed_values.py.
// It computes the lengths of background-position.
func computePosition(style *Style, name string, values Value) Value {
	t, ok := values.(Tuple)
	if !ok {
		return values
	}
	result := make(Tuple, 0, len(t))
	for _, item := range t {
		position, ok := item.(Tuple)
		if !ok || len(position) != 4 {
			result = append(result, item)
			continue
		}
		result = append(result, Tuple{
			position[0], computeLength(style, name, position[1], -1, false),
			position[2], computeLength(style, name, position[3], -1, false),
		})
	}
	return result
}

// computeLengthOrPercentageTuple is length_or_percentage_tuple in
// weasyprint/css/computed_values.py. It computes the lists of lengths that
// can be percentages.
func computeLengthOrPercentageTuple(style *Style, name string, values Value) Value {
	t, ok := values.(Tuple)
	if !ok {
		return values
	}
	result := make(Tuple, len(t))
	for i, value := range t {
		result[i] = computeLength(style, name, value, -1, false)
	}
	return result
}

// computeLengthTuple is length_tuple in weasyprint/css/computed_values.py. It
// computes the properties with a list of lengths.
func computeLengthTuple(style *Style, name string, values Value) Value {
	t, ok := values.(Tuple)
	if !ok {
		return values
	}
	result := make(Tuple, len(t))
	for i, value := range t {
		result[i] = computeLength(style, name, value, -1, true)
	}
	return result
}

// computeBreakBeforeAfter is break_before_after in
// weasyprint/css/computed_values.py.
func computeBreakBeforeAfter(_ *Style, _ string, value Value) Value {
	if isStr(value, "always") {
		return String("page")
	}
	return value
}

// computeLengthProperty is length in weasyprint/css/computed_values.py, as a
// function of COMPUTER_FUNCTIONS.
func computeLengthProperty(style *Style, name string, value Value) Value {
	return computeLength(style, name, value, -1, false)
}

// computeLength is length in weasyprint/css/computed_values.py. It computes
// a length. fontSize is the font size for em, or a negative number for the
// None of upstream.
func computeLength(style *Style, name string, value Value, fontSize float64, pixelsOnly bool) Value {
	if isStrIn(value, "auto", "content", "from-font") || checkMath(value) {
		return value
	}
	d, ok := value.(Dimension)
	if !ok {
		// Port: upstream reads the attribute value, and fails for a value
		// that is not a Dimension.
		return value
	}
	if d.Value == 0 {
		if pixelsOnly {
			return Int(0)
		}
		return ZeroPixels
	}
	if !lengthUnits[d.Unit] {
		// A percentage or 'auto': no conversion needed.
		return value
	}
	pixels := toPixels(d, style, name, fontSize)
	if pixelsOnly {
		return Float(pixels)
	}
	return Dimension{pixels, "px"}
}

// computeBleed is bleed in weasyprint/css/computed_values.py.
func computeBleed(style *Style, name string, value Value) Value {
	if isStr(value, "auto") {
		if tupleHas(style.Get("marks"), "crop") {
			return Dimension{8, "px"}
		}
		return Dimension{0, "px"}
	}
	return computeLength(style, name, value, -1, false)
}

// computePixelLength is pixel_length in weasyprint/css/computed_values.py.
func computePixelLength(style *Style, name string, value Value) Value {
	if isStr(value, "normal") {
		return value
	}
	return computeLength(style, name, value, -1, true)
}

// computeBackgroundSize is background_size in
// weasyprint/css/computed_values.py.
func computeBackgroundSize(style *Style, name string, values Value) Value {
	t, ok := values.(Tuple)
	if !ok {
		return values
	}
	result := make(Tuple, len(t))
	for i, value := range t {
		if isStrIn(value, "contain", "cover") {
			result[i] = value
		} else {
			result[i] = computeLengthOrPercentageTuple(style, name, value)
		}
	}
	return result
}

// computeImageOrientation is image_orientation in
// weasyprint/css/computed_values.py.
func computeImageOrientation(_ *Style, _ string, values Value) Value {
	if isStrIn(values, "none", "from-image") {
		return values
	}
	t, ok := values.(Tuple)
	if !ok || len(t) != 2 {
		return values
	}
	angle := num(t[0])
	return Tuple{Int(pyMod(int(math.RoundToEven(angle/math.Pi*2)), 4) * 90), t[1]}
}

// computeBorderWidth is border_width in weasyprint/css/computed_values.py.
func computeBorderWidth(style *Style, name string, value Value) Value {
	borderStyle := style.Get(strings.Replace(name, "width", "style", 1))
	if isStrIn(borderStyle, "none", "hidden") {
		return Int(0)
	}

	if s, ok := value.(String); ok {
		if width, ok := borderWidthKeywords[string(s)]; ok {
			return Int(width)
		}
	}

	if isNumber(value) {
		// The initial value can get here, but length() would fail as it
		// does not have a 'unit' attribute.
		return value
	}

	return computeLength(style, name, value, -1, true)
}

// fillFour repeats the values of a list of one to three values to make
// four, as the border-image properties do.
func fillFour(values Tuple) Tuple {
	switch len(values) {
	case 1:
		return Tuple{values[0], values[0], values[0], values[0]}
	case 2:
		return Tuple{values[0], values[1], values[0], values[1]}
	case 3:
		return append(values, values[1])
	}
	return values
}

// computeBorderImageSlice is border_image_slice in
// weasyprint/css/computed_values.py.
func computeBorderImageSlice(_ *Style, _ string, values Value) Value {
	t, ok := values.(Tuple)
	if !ok {
		return values
	}
	var computedValues Tuple
	var fill Value
	for _, value := range t {
		if isStr(value, "fill") {
			fill = value
			continue
		}
		d, ok := value.(Dimension)
		if !ok {
			continue
		}
		if d.Unit == "" {
			computedValues = append(computedValues, Float(d.Value))
		} else {
			computedValues = append(computedValues, Dimension{d.Value, "%"})
		}
	}
	return append(fillFour(computedValues), fill)
}

// computeBorderImageWidth is border_image_width in
// weasyprint/css/computed_values.py.
func computeBorderImageWidth(_ *Style, _ string, values Value) Value {
	t, ok := values.(Tuple)
	if !ok {
		return values
	}
	var computedValues Tuple
	for _, value := range t {
		if isStr(value, "auto") {
			computedValues = append(computedValues, value)
			continue
		}
		d, ok := value.(Dimension)
		if !ok {
			computedValues = append(computedValues, value)
			continue
		}
		if d.Unit == "" {
			computedValues = append(computedValues, Float(d.Value))
		} else {
			computedValues = append(computedValues, value)
		}
	}
	return fillFour(computedValues)
}

// computeBorderImageOutset is border_image_outset in
// weasyprint/css/computed_values.py.
func computeBorderImageOutset(style *Style, name string, values Value) Value {
	t, ok := values.(Tuple)
	if !ok {
		return values
	}
	computedValues := make(Tuple, len(t))
	for i, value := range t {
		if isNumber(value) {
			computedValues[i] = value
		} else {
			computedValues[i] = computeLength(style, name, value, -1, false)
		}
	}
	return fillFour(computedValues)
}

// computeBorderImageRepeat is border_image_repeat in
// weasyprint/css/computed_values.py.
func computeBorderImageRepeat(_ *Style, _ string, values Value) Value {
	if t, ok := values.(Tuple); ok && len(t) == 1 {
		return Tuple{t[0], t[0]}
	}
	return values
}

// computeLengthPixelsOnly is length_pixels_only in
// weasyprint/css/computed_values.py. It computes a length in pixels.
func computeLengthPixelsOnly(style *Style, name string, value Value) Value {
	return computeLength(style, name, value, -1, true)
}

// computeBorderRadius is border_radius in weasyprint/css/computed_values.py.
func computeBorderRadius(style *Style, name string, values Value) Value {
	return computeLengthOrPercentageTuple(style, name, values)
}

// computeGap is gap in weasyprint/css/computed_values.py.
func computeGap(style *Style, name string, value Value) Value {
	if isStr(value, "normal") {
		return value
	}
	return computeLength(style, name, value, -1, false)
}

// contentListTypes are the types of the items of a content list that
// _content_list keeps.
var contentListTypes = setOf(
	"string", "content", "url", "quote", "leader()",
	"counter()", "counters()", "content()", "element()", "string()",
	"attr()", "target-counter()", "target-counters()", "target-text()")

// contentList is _content_list in weasyprint/css/computed_values.py.
func contentList(style *Style, values Value) Value {
	t, ok := values.(Tuple)
	if !ok {
		return values
	}
	computedValues := make(Tuple, 0, len(t))
	for _, value := range t {
		if item, ok := value.(Tuple); ok && len(item) > 0 && contentListTypes[str(item[0])] {
			computedValues = append(computedValues, value)
		} else {
			style.ctx.log.warning("Unable to compute content: %s", ValueString(value))
		}
	}
	return computedValues
}

// computeBookmarkLabel is bookmark_label in
// weasyprint/css/computed_values.py.
func computeBookmarkLabel(style *Style, _ string, values Value) Value {
	return contentList(style, values)
}

// computeStringSet is string_set in weasyprint/css/computed_values.py.
func computeStringSet(style *Style, _ string, values Value) Value {
	// Spec asks for strings after custom keywords, but we allow content-lists.
	t, ok := values.(Tuple)
	if !ok {
		return values
	}
	result := make(Tuple, 0, len(t))
	for _, item := range t {
		stringSet, ok := item.(Tuple)
		if !ok || len(stringSet) != 2 {
			continue
		}
		result = append(result, Tuple{stringSet[0], contentList(style, stringSet[1])})
	}
	return result
}

// computeContent is content in weasyprint/css/computed_values.py.
func computeContent(style *Style, _ string, values Value) Value {
	if t, ok := values.(Tuple); ok && len(t) == 1 {
		switch {
		case isStr(t[0], "normal"):
			if style.PseudoType != "" {
				return String("inhibit")
			}
			return String("contents")
		case isStr(t[0], "none"):
			return String("inhibit")
		}
	}
	return contentList(style, values)
}

// computeDisplay is display in weasyprint/css/computed_values.py. See
// https://www.w3.org/TR/CSS21/visuren.html#dis-pos-flo.
func computeDisplay(style *Style, _ string, value Value) Value {
	float := style.Specified("float")
	position := style.Specified("position")
	if isStrIn(position, "absolute", "fixed") || !isStr(float, "none") || style.IsRootElement {
		t, _ := value.(Tuple)
		switch {
		case Equal(value, Tuple{String("inline-table")}):
			return Tuple{String("block"), String("table")}
		case len(t) == 1 && strings.HasPrefix(str(t[0]), "table-"):
			return Tuple{String("block"), String("flow")}
		case len(t) > 0 && isStr(t[0], "inline"):
			if tupleHas(value, "list-item") {
				return Tuple{String("block"), String("flow"), String("list-item")}
			}
			return Tuple{String("block"), String("flow")}
		}
	}
	return value
}

// computeFloat is compute_float in weasyprint/css/computed_values.py. See
// https://www.w3.org/TR/CSS21/visuren.html#dis-pos-flo.
func computeFloat(style *Style, _ string, value Value) Value {
	position := style.Specified("position")
	if isStrIn(position, "absolute", "fixed") {
		return String("none")
	}
	if t, ok := position.(Tuple); ok && len(t) > 0 && isStrIn(t[0], "running()", "note()") {
		return String("none")
	}
	return value
}

// computeFontSize is font_size in weasyprint/css/computed_values.py.
func computeFontSize(style *Style, name string, value Value) Value {
	if s, ok := value.(String); ok {
		if size, ok := fontSizeKeyword(string(s)); ok {
			return Float(size)
		}
	}

	var parentFontSize float64
	if style.ParentStyle == nil {
		parentFontSize = num(InitialValues["font_size"])
	} else {
		parentFontSize = num(style.ParentStyle.Get("font_size"))
	}

	switch {
	case isStr(value, "larger"):
		for _, k := range fontSizeKeywords {
			if k.size > parentFontSize {
				return Float(k.size)
			}
		}
		return Float(parentFontSize * 1.2)
	case isStr(value, "smaller"):
		for i := len(fontSizeKeywords) - 1; i >= 0; i-- {
			if fontSizeKeywords[i].size < parentFontSize {
				return Float(fontSizeKeywords[i].size)
			}
		}
		return Float(parentFontSize * 0.8)
	}
	if d, ok := value.(Dimension); ok && d.Unit == "%" {
		return Float(d.Value * parentFontSize / 100)
	}
	return computeLength(style, name, value, parentFontSize, true)
}

// computeFontWeight is font_weight in weasyprint/css/computed_values.py.
func computeFontWeight(style *Style, _ string, value Value) Value {
	switch {
	case isStr(value, "normal"):
		return Int(400)
	case isStr(value, "bold"):
		return Int(700)
	case isStrIn(value, "bolder", "lighter"):
		var parentValue Value
		if style.ParentStyle == nil {
			parentValue = InitialValues["font_weight"]
		} else {
			parentValue = style.ParentStyle.Get("font_weight")
		}
		weight, ok := fontWeightRelative[str(value)][int(num(parentValue))]
		if !ok {
			// Port: upstream raises KeyError for a weight that is not a
			// multiple of 100. The nearest lower multiple is used.
			p := min(900, max(100, int(num(parentValue))/100*100))
			weight = fontWeightRelative[str(value)][p]
		}
		return Int(weight)
	}
	return value
}

// computeTrackBreadth is _compute_track_breadth in
// weasyprint/css/computed_values.py. It returns nil for None.
func computeTrackBreadth(style *Style, name string, value Value) Value {
	if isStrIn(value, "auto", "min-content", "max-content") {
		return value
	}
	if d, ok := value.(Dimension); ok {
		if strings.ToLower(d.Unit) == "fr" {
			return value
		}
		return computeLength(style, name, value, -1, false)
	}
	return nil
}

// computeTrackSize is _track_size in weasyprint/css/computed_values.py.
func computeTrackSize(style *Style, name string, values Value) Value {
	t, ok := values.(Tuple)
	if !ok {
		return values
	}
	returnValues := make(Tuple, 0, len(t))
	for i, value := range t {
		if i%2 == 0 {
			// Line name.
			returnValues = append(returnValues, value)
			continue
		}
		// Track section.
		if trackBreadth := computeTrackBreadth(style, name, value); truthy(trackBreadth) {
			returnValues = append(returnValues, trackBreadth)
			continue
		}
		v, ok := value.(Tuple)
		if !ok || len(v) == 0 {
			continue
		}
		switch {
		case isStr(v[0], "minmax()") && len(v) == 3:
			returnValues = append(returnValues, Tuple{
				String("minmax()"),
				computeTrackBreadth(style, name, v[1]),
				computeTrackBreadth(style, name, v[2]),
			})
		case isStr(v[0], "fit-content()") && len(v) == 2:
			returnValues = append(returnValues, Tuple{
				String("fit-content()"), computeLength(style, name, v[1], -1, false),
			})
		case isStr(v[0], "repeat()") && len(v) == 3:
			returnValues = append(returnValues, Tuple{
				String("repeat()"), v[1], computeTrackSize(style, name, v[2]),
			})
		}
	}
	return returnValues
}

// computeGridTemplate is grid_template in
// weasyprint/css/computed_values.py.
func computeGridTemplate(style *Style, name string, values Value) Value {
	if isStr(values, "none") {
		return values
	}
	if t, ok := values.(Tuple); ok && len(t) > 0 && isStr(t[0], "subgrid") {
		return values
	}
	return computeTrackSize(style, name, values)
}

// computeGridAuto is grid_auto in weasyprint/css/computed_values.py.
func computeGridAuto(style *Style, name string, values Value) Value {
	t, ok := values.(Tuple)
	if !ok {
		return values
	}
	returnValues := make(Tuple, 0, len(t))
	for _, value := range t {
		if trackBreadth := computeTrackBreadth(style, name, value); truthy(trackBreadth) {
			returnValues = append(returnValues, trackBreadth)
			continue
		}
		v, ok := value.(Tuple)
		if !ok || len(v) == 0 {
			continue
		}
		first := func(v Value) Value {
			if r, ok := computeGridAuto(style, name, Tuple{v}).(Tuple); ok && len(r) > 0 {
				return r[0]
			}
			// Port: upstream raises IndexError.
			return nil
		}
		switch {
		case isStr(v[0], "minmax()") && len(v) == 3:
			returnValues = append(returnValues, Tuple{String("minmax()"), first(v[1]), first(v[2])})
		case isStr(v[0], "fit-content()") && len(v) == 2:
			returnValues = append(returnValues, Tuple{String("fit-content()"), first(v[1])})
		}
	}
	return returnValues
}

// computeLineHeight is line_height in weasyprint/css/computed_values.py.
func computeLineHeight(style *Style, name string, value Value) Value {
	if isStr(value, "normal") || checkMath(value) {
		return value
	}
	d, ok := value.(Dimension)
	if !ok {
		return value
	}
	switch d.Unit {
	case "":
		return Float(d.Value)
	case "%":
		factor := d.Value / 100
		fontSizeValue := num(style.Get("font_size"))
		pixels := factor * fontSizeValue
		return Dimension{pixels, "px"}
	}
	return computeLength(style, name, value, -1, false)
}

// computeLang is lang in weasyprint/css/computed_values.py.
func computeLang(_ *Style, _ string, values Value) Value {
	if t, ok := values.(Tuple); ok && len(t) == 2 && isStr(t[0], "string") {
		return t[1]
	}
	return nil
}

// computeTabSize is tab_size in weasyprint/css/computed_values.py.
func computeTabSize(style *Style, name string, value Value) Value {
	if _, ok := value.(Int); ok {
		return value
	}
	return computeLength(style, name, value, -1, false)
}

// computeTransform is transform in weasyprint/css/computed_values.py.
func computeTransform(style *Style, name string, value Value) Value {
	t, ok := value.(Tuple)
	if !ok {
		return value
	}
	result := make(Tuple, 0, len(t))
	for _, item := range t {
		pair, ok := item.(Tuple)
		if !ok || len(pair) != 2 {
			continue
		}
		function, args := pair[0], pair[1]
		if isStr(function, "translate") {
			args = computeLengthOrPercentageTuple(style, name, args)
		}
		result = append(result, Tuple{function, args})
	}
	return result
}

// computeVerticalAlign is vertical_align in
// weasyprint/css/computed_values.py.
func computeVerticalAlign(style *Style, name string, value Value) Value {
	// Use +/- half an em for super and sub, same as Pango.
	// (See the SUPERSUB_RISE constant in pango-markup.c)
	switch {
	case isStrIn(value, "baseline", "middle", "text-top", "text-bottom", "top", "bottom"):
		return value
	case isStr(value, "super"):
		return Float(num(style.Get("font_size")) * 0.5)
	case isStr(value, "sub"):
		return Float(num(style.Get("font_size")) * -0.5)
	case checkMath(value):
		return value
	}
	if d, ok := value.(Dimension); ok && d.Unit == "%" {
		height, _ := strut(style)
		return Float(height * d.Value / 100)
	}
	return computeLength(style, name, value, -1, true)
}

// computeWordSpacing is word_spacing in weasyprint/css/computed_values.py.
func computeWordSpacing(style *Style, name string, value Value) Value {
	if isStr(value, "normal") {
		return Int(0)
	}
	return computeLength(style, name, value, -1, true)
}
