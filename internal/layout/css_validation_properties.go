// Copyright (c) 2011-2021, Simon Sapin and contributors.
//
// Use of this source code is governed by the BSD-3-Clause license that can be
// found in the LICENSE file.

// This file ports weasyprint/css/validation/properties.py: the validation of
// the properties. See https://www.w3.org/TR/CSS21/propidx.html and various
// CSS3 modules.

package layout

import (
	"math"
	"slices"
	"strings"

	"github.com/xo/okage/tinycss2"
)

// prefix is PREFIX in weasyprint/css/validation/properties.py.
const prefix = "-weasy-"

// proprietary is PROPRIETARY in weasyprint/css/validation/properties.py: the
// properties that are only known with the prefix.
var proprietary = map[string]bool{}

// unstable is UNSTABLE in weasyprint/css/validation/properties.py: the
// properties of specifications that did not reach the Candidate
// Recommendation stage. They can be used with or without the prefix.
var unstable = map[string]bool{}

// properties is PROPERTIES in weasyprint/css/validation/properties.py: the
// validators of the properties that are not shorthands, by name in kebab
// case.
var properties = map[string]validatorFunc{}

// propertyOption is an argument of the decorator property.
type propertyOption int

// The arguments of the decorator property. wants_base_url is not needed, as
// each validator takes the base URL.
const (
	// optProprietary is proprietary=True.
	optProprietary propertyOption = iota + 1
	// optUnstable is unstable=True.
	optUnstable
)

// property is the decorator property in
// weasyprint/css/validation/properties.py. It registers the validator of a
// property, by name in kebab case.
//
// Proprietary (vendor-specific, non-standard) properties are prefixed:
// anchors can for example be set using `-weasy-anchor: "id"`. See
// https://www.w3.org/TR/CSS/#proprietary. Unstable properties are defined in
// specifications that did not reach the Candidate Recommendation stage. They
// can be used both vendor-prefixed or unprefixed. See
// https://www.w3.org/TR/CSS/#unstable-syntax.
func property(name string, f validatorFunc, opts ...propertyOption) {
	if !knownProperties[name] {
		panic("unknown property " + name)
	}
	if _, ok := properties[name]; ok {
		panic("property registered twice " + name)
	}
	properties[name] = f
	for _, opt := range opts {
		switch opt {
		case optProprietary:
			proprietary[name] = true
		case optUnstable:
			unstable[name] = true
		}
	}
}

// newPendingProperty is PendingProperty in
// weasyprint/css/validation/properties.py: a property whose validation is
// done when the computed values are defined.
func newPendingProperty(tokens []tinycss2.Node, name string) *Pending {
	p := &Pending{Tokens: tokens, Name: name}
	p.validate = func(tokens []tinycss2.Node, _, baseURL string) (Value, error) {
		pair, err := validateNonShorthand(tokens, p.Name, baseURL, false)
		if err != nil {
			return nil, err
		}
		return pair.value, nil
	}
	return p
}

// validateNonShorthand is validate_non_shorthand in
// weasyprint/css/validation/properties.py, the validator of the properties
// that are not shorthands. Upstream returns ((name, value),). The port
// returns the pair.
func validateNonShorthand(tokens []tinycss2.Node, name, baseURL string, required bool) (nameValue, error) {
	isCustom := strings.HasPrefix(name, "--")
	if !required && !knownProperties[name] && !isCustom {
		return nameValue{}, invalidValues("unknown property")
	}

	f, ok := properties[name]
	if !required && !ok && !isCustom {
		return nameValue{}, invalidValues("property not supported yet")
	}

	for _, token := range tokens {
		if checkVar(token) {
			// Found CSS variable, return pending-substitution values.
			return nameValue{name, newPendingProperty(tokens, name)}, nil
		}
	}

	if isCustom {
		return nameValue{name, Tokens(tokens)}, nil
	}

	if f == nil {
		// Port: upstream raises KeyError for a required property that has
		// no validator.
		return nameValue{}, invalidValues("property not supported yet")
	}
	var value Value
	if keyword := getSingleKeyword(tokens); keyword == "initial" || keyword == "inherit" {
		value = String(keyword)
	} else {
		var err error
		value, err = f(tokens, baseURL)
		if err != nil {
			return nameValue{}, err
		}
		if value == nil {
			return nameValue{}, errInvalid
		}
	}
	return nameValue{name, value}, nil
}

// validateNonShorthandExpander is validate_non_shorthand with the signature
// of an expander.
func validateNonShorthandExpander(tokens []tinycss2.Node, name, baseURL string) ([]nameValue, error) {
	pair, err := validateNonShorthand(tokens, name, baseURL, false)
	if err != nil {
		return nil, err
	}
	return []nameValue{pair}, nil
}

func init() {
	property("background-attachment", validateBackgroundAttachment)
	for _, name := range []string{
		"background-color", "border-top-color", "border-right-color",
		"border-bottom-color", "border-left-color", "border-block-start-color",
		"border-block-end-color", "border-inline-start-color",
		"border-inline-end-color",
	} {
		property(name, validateOtherColors)
	}
	property("column-rule-color", validateOtherColors, optUnstable)
	property("text-decoration-color", validateOtherColors)
	property("outline-color", validateOutlineColor)
	property("border-collapse", validateBorderCollapse)
	property("empty-cells", validateEmptyCells)
	property("color", validateColor)
	property("background-image", validateBackgroundImage)
	property("list-style-image", validateListStyleImage)
	property("transform-origin", validateTransformOrigin)
	property("background-position", validateBackgroundPosition)
	property("object-position", validateObjectPosition)
	property("background-repeat", validateBackgroundRepeat)
	property("background-size", validateBackgroundSize)
	property("background-clip", validateBox)
	property("background-origin", validateBox)
	property("border-spacing", validateBorderSpacing)
	for _, name := range []string{
		"border-top-right-radius", "border-bottom-right-radius",
		"border-bottom-left-radius", "border-top-left-radius",
		"border-start-start-radius", "border-start-end-radius",
		"border-end-start-radius", "border-end-end-radius",
	} {
		property(name, validateBorderCornerRadius)
	}
	for _, name := range []string{
		"border-top-style", "border-right-style", "border-left-style",
		"border-bottom-style", "border-block-start-style",
		"border-block-end-style", "border-inline-start-style",
		"border-inline-end-style",
	} {
		property(name, validateBorderStyle)
	}
	property("column-rule-style", validateBorderStyle, optUnstable)
	property("box-shadow", validateBoxShadow)
	property("break-before", validateBreakBeforeAfter)
	property("break-after", validateBreakBeforeAfter)
	property("break-inside", validateBreakInside)
	property("box-decoration-break", validateBoxDecorationBreak)
	property("block-ellipsis", validateBlockEllipsis)
	property("continue", validateContinue, optUnstable)
	property("max-lines", validateMaxLines, optUnstable)
	property("margin-break", validateMarginBreak, optUnstable)
	property("page", validatePage, optUnstable)
	for _, name := range []string{"bleed-left", "bleed-right", "bleed-top", "bleed-bottom"} {
		property(name, validateBleed, optUnstable)
	}
	property("marks", validateMarks, optUnstable)
	property("outline-style", validateOutlineStyle)
	for _, name := range []string{
		"border-top-width", "border-right-width", "border-left-width",
		"border-bottom-width", "border-block-start-width",
		"border-block-end-width", "border-inline-start-width",
		"border-inline-end-width",
	} {
		property(name, validateBorderWidth)
	}
	property("column-rule-width", validateBorderWidth, optUnstable)
	property("outline-width", validateBorderWidth)
	property("border-image-source", validateBorderImageSource)
	property("mask-border-source", validateBorderImageSource)
	property("border-image-slice", validateBorderImageSlice)
	property("mask-border-slice", validateBorderImageSlice)
	property("border-image-width", validateBorderImageWidth)
	property("mask-border-width", validateBorderImageWidth)
	property("border-image-outset", validateBorderImageOutset)
	property("mask-border-outset", validateBorderImageOutset)
	property("border-image-repeat", validateBorderImageRepeat)
	property("mask-border-repeat", validateBorderImageRepeat)
	property("mask-border-mode", validateMaskBorderMode)
	property("column-width", validateColumnWidth, optUnstable)
	property("column-span", validateColumnSpan, optUnstable)
	property("box-sizing", validateBoxSizing)
	property("caption-side", validateCaptionSide)
	property("clear", validateClear)
	property("clip", validateClip)
	property("content", validateContent)
	property("counter-increment", validateCounterIncrement)
	property("counter-reset", validateCounterReset)
	property("counter-set", validateCounterSet)
	for _, name := range []string{
		"top", "right", "left", "bottom", "inset-block-start",
		"inset-block-end", "inset-inline-start", "inset-inline-end",
		"margin-top", "margin-right", "margin-bottom", "margin-left",
		"margin-block-start", "margin-block-end", "margin-inline-start",
		"margin-inline-end", "text-underline-offset",
	} {
		property(name, validateLenghtPrecentageOrAuto)
	}
	for _, name := range []string{"height", "width", "block-size", "inline-size"} {
		property(name, validateWidthHeight)
	}
	property("column-gap", validateGap, optUnstable)
	property("row-gap", validateGap, optUnstable)
	property("column-fill", validateColumnFill, optUnstable)
	property("direction", validateDirection)
	property("display", validateDisplay)
	property("float", validateFloat)
	property("font-family", validateFontFamily)
	property("font-kerning", validateFontKerning)
	property("font-language-override", validateFontLanguageOverride)
	property("font-variant-ligatures", validateFontVariantLigatures)
	property("font-variant-position", validateFontVariantPosition)
	property("font-variant-caps", validateFontVariantCaps)
	property("font-variant-numeric", validateFontVariantNumeric)
	property("font-feature-settings", validateFontFeatureSettings)
	property("font-variant-alternates", validateFontVariantAlternates)
	property("font-variant-east-asian", validateFontVariantEastAsian)
	property("font-variation-settings", validateFontVariationSettings)
	property("font-size", validateFontSize)
	property("font-style", validateFontStyle)
	property("font-stretch", validateFontStretch)
	property("font-weight", validateFontWeight)
	property("object-fit", validateObjectFit)
	property("image-resolution", validateImageResolution, optUnstable)
	property("letter-spacing", validateSpacing)
	property("word-spacing", validateSpacing)
	property("outline-offset", validateOutlineOffset)
	property("line-height", validateLineHeight)
	property("list-style-position", validateListStylePosition)
	property("list-style-type", validateListStyleType)
	for _, name := range []string{"min-width", "min-height", "min-block-size", "min-inline-size"} {
		property(name, validateMinWidthHeight)
	}
	for _, name := range []string{
		"padding-top", "padding-right", "padding-bottom", "padding-left",
		"padding-block-start", "padding-block-end", "padding-inline-start",
		"padding-inline-end",
	} {
		property(name, validateLengthOrPrecentage)
	}
	for _, name := range []string{"max-width", "max-height", "max-block-size", "max-inline-size"} {
		property(name, validateMaxWidthHeight)
	}
	property("opacity", validateOpacity)
	property("z-index", validateZIndex)
	property("orphans", validateOrphansWidows)
	property("widows", validateOrphansWidows)
	property("column-count", validateColumnCount, optUnstable)
	property("overflow", validateOverflow)
	property("text-overflow", validateTextOverflow)
	property("position", validatePosition)
	property("quotes", validateQuotes)
	property("table-layout", validateTableLayout)
	property("text-align-all", validateTextAlignAll)
	property("text-align-last", validateTextAlignLast)
	property("text-decoration-line", validateTextDecorationLine)
	property("text-decoration-style", validateTextDecorationStyle)
	property("text-decoration-thickness", validateTextDecorationThickness)
	property("text-indent", validateTextIndent)
	property("text-transform", validateTextTransform)
	property("vertical-align", validateVerticalAlign)
	property("visibility", validateVisibility)
	property("white-space", validateWhiteSpace)
	property("overflow-wrap", validateOverflowWrap)
	property("word-break", validateWordBreak)
	property("flex-basis", validateFlexBasis)
	property("flex-direction", validateFlexDirection)
	property("flex-grow", validateFlexGrowShrink)
	property("flex-shrink", validateFlexGrowShrink)
	property("grid-auto-columns", validateGridAuto)
	property("grid-auto-rows", validateGridAuto)
	property("grid-auto-flow", validateGridAutoFlow)
	property("grid-template-columns", validateGridTemplate)
	property("grid-template-rows", validateGridTemplate)
	property("grid-template-areas", validateGridTemplateAreas)
	for _, name := range []string{"grid-row-start", "grid-row-end", "grid-column-start", "grid-column-end"} {
		property(name, validateGridLine)
	}
	property("flex-wrap", validateFlexWrap)
	property("justify-content", validateJustifyContent)
	property("justify-items", validateJustifyItems)
	property("justify-self", validateJustifySelf)
	property("align-items", validateAlignItems)
	property("align-self", validateAlignSelf)
	property("align-content", validateAlignContent)
	property("order", validateOrder)
	property("image-rendering", validateImageRendering, optUnstable)
	property("image-orientation", validateImageOrientation, optUnstable)
	property("size", validateSize, optUnstable)
	property("anchor", validateAnchor, optProprietary)
	property("tab-size", validateTabSize)
	property("tab-stops", validateTabStops)
	property("margin-box-fit", validateMarginBoxFit)
	property("static-top", validateStaticTop)
	property("text-autospace", validateTextAutospace)
	property("page-overflow", validatePageOverflow)
	property("line-numbers", validateLineNumbers)
	property("line-multiple", validateLineMultiple)
	property("page-top-space", validatePageTopSpace)
	property("float-position", validateFloatPosition)
	property("float-overlap", validateFloatOverlap)
	property("wrap-flow", validateWrapFlow)
	property("wrap-shape", validateWrapShape)
	property("stroke-adjust", validateStrokeAdjust)
	property("line-baseline", validateLineBaseline)
	property("text-outline", validateTextOutline)
	property("hyphens", validateHyphens, optUnstable)
	property("hyphenate-character", validateHyphenateCharacter, optUnstable)
	property("hyphenate-limit-zone", validateHyphenateLimitZone, optUnstable)
	property("hyphenate-limit-chars", validateHyphenateLimitChars, optUnstable)
	property("lang", validateLang, optProprietary)
	property("bookmark-label", validateBookmarkLabel, optUnstable)
	property("bookmark-level", validateBookmarkLevel, optUnstable)
	property("bookmark-state", validateBookmarkState, optUnstable)
	property("footnote-display", validateFootnoteDisplay, optUnstable)
	property("footnote-policy", validateFootnotePolicy, optUnstable)
	property("string-set", validateStringSet, optUnstable)
	property("transform", validateTransform)
	property("appearance", validateAppearance)
	property("color-scheme", validateColorScheme)
}

// keywordIn returns a function that reports whether a keyword is one of the
// keywords, for the decorator single_keyword. The empty string is the None
// of upstream, and is in no list.
func keywordIn(keywords ...string) func(string) bool {
	return func(keyword string) bool {
		return in(keyword, keywords...)
	}
}

// numberValue is number.value of upstream for the result of getNumber: an
// Int when the number is an integer, and a Float else. It returns nil for a
// nil result.
func numberValue(number Value, integer bool) Value {
	d, ok := number.(Dimension)
	if !ok {
		return nil
	}
	if integer {
		return Int(clampInt(int64(max(min(d.Value, math.MaxInt64), math.MinInt64))))
	}
	return Float(d.Value)
}

// keywordValue returns the String of a keyword, or nil for the "" of None.
func keywordTuple(tokens []tinycss2.Node) Tuple {
	keywords := make(Tuple, len(tokens))
	for i, token := range tokens {
		keywords[i] = keywordOrNone(getKeyword(token))
	}
	return keywords
}

// validateBackgroundAttachment is background_attachment in
// weasyprint/css/validation/properties.py.
var (
	validateBackgroundAttachmentSingle = singleKeyword(keywordIn("scroll", "fixed", "local"))
	validateBackgroundAttachment       = commaSeparatedList(validateBackgroundAttachmentSingle)
)

// validateOtherColors is other_colors in
// weasyprint/css/validation/properties.py, the validator of the *-color
// properties.
var validateOtherColors = singleToken(func(token tinycss2.Node, _ string) (Value, error) {
	if parseColorToken(token) != nil {
		return Token{token}, nil
	}
	return nil, nil
})

// validateOutlineColor is outline_color in
// weasyprint/css/validation/properties.py.
var validateOutlineColor = singleToken(func(token tinycss2.Node, _ string) (Value, error) {
	if getKeyword(token) == "invert" {
		return String("currentcolor"), nil
	}
	if parseColorToken(token) != nil {
		return Token{token}, nil
	}
	return nil, nil
})

// validateBorderCollapse is border_collapse in
// weasyprint/css/validation/properties.py.
var validateBorderCollapse = singleKeyword(keywordIn("separate", "collapse"))

// validateEmptyCells is empty_cells in
// weasyprint/css/validation/properties.py.
var validateEmptyCells = singleKeyword(keywordIn("show", "hide"))

// validateColor is color in weasyprint/css/validation/properties.py.
var validateColor = singleToken(func(token tinycss2.Node, _ string) (Value, error) {
	result := parseColorToken(token)
	switch {
	case result == nil:
		return nil, nil
	case isStr(result, "currentcolor"):
		return String("inherit"), nil
	}
	return Token{token}, nil
})

// validateBackgroundImage is background_image in
// weasyprint/css/validation/properties.py.
var (
	validateBackgroundImageSingle = singleToken(func(token tinycss2.Node, baseURL string) (Value, error) {
		if getKeyword(token) == "none" {
			return Tuple{String("none"), nil}, nil
		}
		return getImage(token, baseURL)
	})
	validateBackgroundImage = commaSeparatedList(validateBackgroundImageSingle)
)

// validateListStyleImage is list_style_image in
// weasyprint/css/validation/properties.py.
var validateListStyleImage = singleToken(func(token tinycss2.Node, baseURL string) (Value, error) {
	if getKeyword(token) == "none" {
		return Tuple{String("none"), nil}, nil
	}
	parsedURL, err := getURL(token, baseURL)
	if err != nil {
		return nil, err
	}
	if t, ok := parsedURL.(Tuple); ok && isStr(t[0], "url") {
		if u, ok := t[1].(Tuple); ok && isStr(u[0], "external") {
			return Tuple{String("url"), u[1]}, nil
		}
	}
	return nil, nil
})

// validateTransformOrigin is transform_origin in
// weasyprint/css/validation/properties.py.
func validateTransformOrigin(tokens []tinycss2.Node, _ string) (Value, error) {
	if len(tokens) == 3 {
		// Ignore third parameter as 3D transforms are ignored.
		tokens = tokens[:2]
	}
	if result := parse2DPosition(tokens); result != nil {
		return result, nil
	}
	return nil, nil
}

// positionValidator is the validator of parse_position.
func positionValidator(tokens []tinycss2.Node, _ string) (Value, error) {
	return parsePosition(tokens), nil
}

// validateBackgroundPosition is background_position in
// weasyprint/css/validation/properties.py.
var (
	validateBackgroundPositionSingle = positionValidator
	validateBackgroundPosition       = commaSeparatedList(validateBackgroundPositionSingle)
)

// validateObjectPosition is object_position in
// weasyprint/css/validation/properties.py.
var (
	validateObjectPositionSingle = positionValidator
	validateObjectPosition       = commaSeparatedList(validateObjectPositionSingle)
)

// validateBackgroundRepeat is background_repeat in
// weasyprint/css/validation/properties.py.
var (
	validateBackgroundRepeatSingle validatorFunc = func(tokens []tinycss2.Node, _ string) (Value, error) {
		keywords := keywordTuple(tokens)
		if len(keywords) == 1 {
			switch {
			case isStr(keywords[0], "repeat-x"):
				return tupleOf("repeat", "no-repeat"), nil
			case isStr(keywords[0], "repeat-y"):
				return tupleOf("no-repeat", "repeat"), nil
			case isStrIn(keywords[0], "no-repeat", "repeat", "space", "round"):
				return Tuple{keywords[0], keywords[0]}, nil
			}
		}
		if len(keywords) == 2 && isStrIn(keywords[0], "no-repeat", "repeat", "space", "round") &&
			isStrIn(keywords[1], "no-repeat", "repeat", "space", "round") {
			return keywords, nil
		}
		return nil, nil
	}
	validateBackgroundRepeat = commaSeparatedList(validateBackgroundRepeatSingle)
)

// validateBackgroundSize is background_size in
// weasyprint/css/validation/properties.py.
var (
	validateBackgroundSizeSingle validatorFunc = func(tokens []tinycss2.Node, _ string) (Value, error) {
		switch len(tokens) {
		case 1:
			token := tokens[0]
			keyword := getKeyword(token)
			switch keyword {
			case "contain", "cover":
				return String(keyword), nil
			case "auto":
				return tupleOf("auto", "auto"), nil
			}
			if length := getLength(token, false, true); length != nil {
				return Tuple{length, String("auto")}, nil
			}
		case 2:
			var values Tuple
			for _, token := range tokens {
				if length := getLength(token, false, true); length != nil {
					values = append(values, length)
				} else if getKeyword(token) == "auto" {
					values = append(values, String("auto"))
				}
			}
			if len(values) == 2 {
				return values, nil
			}
		}
		return nil, nil
	}
	validateBackgroundSize = commaSeparatedList(validateBackgroundSizeSingle)
)

// validateBox is box in weasyprint/css/validation/properties.py, the
// validator of the <box> type of background-clip and background-origin.
var (
	validateBoxSingle = singleKeyword(keywordIn("border-box", "padding-box", "content-box"))
	validateBox       = commaSeparatedList(validateBoxSingle)
)

// twoLengths returns (lengths[0], lengths[0]) for one length and the tuple
// of two lengths, or nil, as border_spacing and border_corner_radius do.
func twoLengths(lengths Tuple) Value {
	for _, length := range lengths {
		if length == nil {
			return nil
		}
	}
	switch len(lengths) {
	case 1:
		return Tuple{lengths[0], lengths[0]}
	case 2:
		return lengths
	}
	return nil
}

// validateBorderSpacing is border_spacing in
// weasyprint/css/validation/properties.py.
func validateBorderSpacing(tokens []tinycss2.Node, _ string) (Value, error) {
	lengths := make(Tuple, len(tokens))
	for i, token := range tokens {
		lengths[i] = getLength(token, false, false)
	}
	return twoLengths(lengths), nil
}

// validateBorderCornerRadius is border_corner_radius in
// weasyprint/css/validation/properties.py.
func validateBorderCornerRadius(tokens []tinycss2.Node, _ string) (Value, error) {
	lengths := make(Tuple, len(tokens))
	for i, token := range tokens {
		lengths[i] = getLength(token, false, true)
	}
	return twoLengths(lengths), nil
}

// validateBorderStyle is border_style in
// weasyprint/css/validation/properties.py.
var validateBorderStyle = singleKeyword(keywordIn(
	"none", "hidden", "dotted", "dashed", "double",
	"inset", "outset", "groove", "ridge", "solid"))

// validateBoxShadow is box_shadow in
// weasyprint/css/validation/properties.py.
func validateBoxShadow(tokens []tinycss2.Node, _ string) (Value, error) {
	if getSingleKeyword(tokens) == "none" {
		return Tuple{}, nil
	}
	var shadows Tuple
	tokens = slices.Clone(tokens)
	for len(tokens) > 0 {
		var lengths Tuple
		var color Value
		inset := false
		lastLength := false
		for len(tokens) > 0 {
			token := tokens[0]
			tokens = tokens[1:]
			if tinycss2.IsLiteral(token, ",") && len(tokens) > 0 {
				break
			}
			if ident, ok := token.(*tinycss2.IdentToken); ok && ident.Value == "inset" && !inset {
				inset = true
				lastLength = false
				continue
			}
			if length := getLength(token, true, false); length != nil && (len(lengths) == 0 || lastLength) {
				lengths = append(lengths, length)
				lastLength = true
				continue
			}
			if color == nil && parseColorToken(token) != nil {
				color = Token{token}
				lastLength = false
				continue
			}
			return nil, nil
		}
		if len(lengths) < 2 || len(lengths) > 4 {
			return nil, nil
		}
		x, y := lengths[0], lengths[1]
		lengths = lengths[2:]
		var blur Value = ZeroPixels
		if len(lengths) > 0 {
			blur, lengths = lengths[0], lengths[1:]
		}
		d, ok := blur.(Dimension)
		if !ok {
			// Port: upstream raises AttributeError for a blur that is a math
			// function.
			return nil, nil
		}
		if d.Value < 0 {
			return nil, nil
		}
		var spread Value = ZeroPixels
		if len(lengths) > 0 {
			spread = lengths[0]
		}
		if color == nil {
			color = String("currentcolor")
		}
		shadows = append(shadows, Tuple{x, y, blur, spread, color, Bool(inset)})
	}
	return shadows, nil
}

// validateBreakBeforeAfter is break_before_after in
// weasyprint/css/validation/properties.py.
var validateBreakBeforeAfter = singleKeyword(keywordIn(
	"auto", "avoid", "avoid-page", "page", "left", "right",
	"recto", "verso", "avoid-column", "column", "always"))

// validateBreakInside is break_inside in
// weasyprint/css/validation/properties.py.
var validateBreakInside = singleKeyword(keywordIn("auto", "avoid", "avoid-page", "avoid-column"))

// validateBoxDecorationBreak is box_decoration_break in
// weasyprint/css/validation/properties.py.
var validateBoxDecorationBreak = singleKeyword(keywordIn("slice", "clone"))

// validateBlockEllipsis is block_ellipsis in
// weasyprint/css/validation/properties.py.
var validateBlockEllipsis = singleToken(func(token tinycss2.Node, _ string) (Value, error) {
	if s, ok := token.(*tinycss2.StringToken); ok {
		return Tuple{String("string"), String(s.Value)}, nil
	}
	if keyword := getKeyword(token); in(keyword, "none", "auto") {
		return String(keyword), nil
	}
	return nil, nil
})

// validateContinue is continue_ in weasyprint/css/validation/properties.py.
var validateContinue = singleKeyword(keywordIn("auto", "discard"))

// validateMaxLines is max_lines in weasyprint/css/validation/properties.py.
var validateMaxLines = singleToken(func(token tinycss2.Node, _ string) (Value, error) {
	if number := getNumber(token, false, true); number != nil {
		if number.(Dimension).Value >= 1 {
			return numberValue(number, true), nil
		}
	} else if getKeyword(token) == "none" {
		return String("none"), nil
	}
	return nil, nil
})

// validateMarginBreak is margin_break in
// weasyprint/css/validation/properties.py.
var validateMarginBreak = singleKeyword(keywordIn("auto", "keep", "discard"))

// validatePage is page in weasyprint/css/validation/properties.py.
var validatePage = singleToken(func(token tinycss2.Node, _ string) (Value, error) {
	if ident, ok := token.(*tinycss2.IdentToken); ok {
		if ident.LowerValue == "auto" {
			return String("auto"), nil
		}
		return String(ident.Value), nil
	}
	return nil, nil
})

// validateBleed is bleed in weasyprint/css/validation/properties.py.
var validateBleed = singleToken(func(token tinycss2.Node, _ string) (Value, error) {
	if getKeyword(token) == "auto" {
		return String("auto"), nil
	}
	return getLength(token, true, false), nil
})

// validateMarks is marks in weasyprint/css/validation/properties.py.
func validateMarks(tokens []tinycss2.Node, _ string) (Value, error) {
	switch len(tokens) {
	case 2:
		keywords := keywordTuple(tokens)
		if tupleHas(keywords, "crop") && tupleHas(keywords, "cross") {
			return keywords, nil
		}
	case 1:
		switch keyword := getKeyword(tokens[0]); keyword {
		case "crop", "cross":
			return Tuple{String(keyword)}, nil
		case "none":
			return Tuple{}, nil
		}
	}
	return nil, nil
}

// validateOutlineStyle is outline_style in
// weasyprint/css/validation/properties.py.
var validateOutlineStyle = singleKeyword(keywordIn(
	"none", "dotted", "dashed", "double", "inset",
	"outset", "groove", "ridge", "solid"))

// validateBorderWidth is border_width in
// weasyprint/css/validation/properties.py, the validator of the widths of
// borders, column rules and outlines.
var validateBorderWidth = singleToken(func(token tinycss2.Node, _ string) (Value, error) {
	if length := getLength(token, false, false); length != nil {
		return length, nil
	}
	if keyword := getKeyword(token); in(keyword, "thin", "medium", "thick") {
		return String(keyword), nil
	}
	return nil, nil
})

// validateBorderImageSource is border_image_source in
// weasyprint/css/validation/properties.py.
var validateBorderImageSource = singleToken(func(token tinycss2.Node, baseURL string) (Value, error) {
	if getKeyword(token) == "none" {
		return Tuple{String("none"), nil}, nil
	}
	return getImage(token, baseURL)
})

// validateBorderImageSlice is border_image_slice in
// weasyprint/css/validation/properties.py.
func validateBorderImageSlice(tokens []tinycss2.Node, _ string) (Value, error) {
	var values Tuple
	fill := false
	for i, token := range tokens {
		// Do not use get_length() because a dimension with a unit is
		// disallowed.
		if percentage := getPercentage(token, false); percentage != nil {
			values = append(values, percentage)
		} else if getKeyword(token) == "fill" && !fill && (i == 0 || i == len(tokens)-1) {
			fill = true
			values = append(values, String("fill"))
		} else if number := getNumber(token, false, false); number != nil {
			values = append(values, number)
		} else {
			return nil, nil
		}
	}
	n := len(values)
	if fill {
		n--
	}
	if 1 <= n && n <= 4 {
		return values, nil
	}
	return nil, nil
}

// validateBorderImageWidth is border_image_width in
// weasyprint/css/validation/properties.py.
func validateBorderImageWidth(tokens []tinycss2.Node, _ string) (Value, error) {
	var values Tuple
	for _, token := range tokens {
		if getKeyword(token) == "auto" {
			values = append(values, String("auto"))
		} else if number := getNumber(token, false, false); number != nil {
			values = append(values, number)
		} else if length := getLength(token, false, true); length != nil {
			values = append(values, length)
		} else {
			return nil, nil
		}
	}
	if 1 <= len(values) && len(values) <= 4 {
		return values, nil
	}
	return nil, nil
}

// validateBorderImageOutset is border_image_outset in
// weasyprint/css/validation/properties.py.
func validateBorderImageOutset(tokens []tinycss2.Node, _ string) (Value, error) {
	var values Tuple
	for _, token := range tokens {
		if number := getNumber(token, false, false); number != nil {
			values = append(values, number)
		} else if length := getLength(token, false, false); length != nil {
			values = append(values, length)
		} else {
			return nil, nil
		}
	}
	if 1 <= len(values) && len(values) <= 4 {
		return values, nil
	}
	return nil, nil
}

// validateBorderImageRepeat is border_image_repeat in
// weasyprint/css/validation/properties.py.
func validateBorderImageRepeat(tokens []tinycss2.Node, _ string) (Value, error) {
	if 1 <= len(tokens) && len(tokens) <= 2 {
		keywords := keywordTuple(tokens)
		for _, keyword := range keywords {
			if !isStrIn(keyword, "stretch", "repeat", "round", "space") {
				return nil, nil
			}
		}
		return keywords, nil
	}
	return nil, nil
}

// validateMaskBorderMode is mask_border_mode in
// weasyprint/css/validation/properties.py.
var validateMaskBorderMode = singleKeyword(keywordIn("luminance", "alpha"))

// validateColumnWidth is column_width in
// weasyprint/css/validation/properties.py.
var validateColumnWidth = singleToken(func(token tinycss2.Node, _ string) (Value, error) {
	if length := getLength(token, false, false); length != nil {
		return length, nil
	}
	if getKeyword(token) == "auto" {
		return String("auto"), nil
	}
	return nil, nil
})

// validateColumnSpan is column_span in
// weasyprint/css/validation/properties.py.
var validateColumnSpan = singleKeyword(keywordIn("all", "none"))

// validateBoxSizing is box_sizing in
// weasyprint/css/validation/properties.py.
var validateBoxSizing = singleKeyword(keywordIn("padding-box", "border-box", "content-box"))

// validateCaptionSide is caption_side in
// weasyprint/css/validation/properties.py.
var validateCaptionSide = singleKeyword(keywordIn("top", "bottom"))

// validateClear is clear in weasyprint/css/validation/properties.py.
var validateClear = singleKeyword(keywordIn("left", "right", "inline-start", "inline-end", "both", "none"))

// validateClip is clip in weasyprint/css/validation/properties.py.
var validateClip = singleToken(func(token tinycss2.Node, _ string) (Value, error) {
	f := newFunction(token)
	arguments := f.splitComma(false)
	if f.name == "rect" && len(arguments) == 4 {
		var values Tuple
		for _, argument := range arguments {
			if getKeyword(argument) == "auto" {
				values = append(values, String("auto"))
			} else if length := getLength(argument, true, false); length != nil {
				values = append(values, length)
			} else {
				return nil, nil
			}
		}
		return values, nil
	}
	if getKeyword(token) == "auto" {
		return Tuple{}, nil
	}
	return nil, nil
})

// validateContent is content in weasyprint/css/validation/properties.py.
// See https://www.w3.org/TR/css-content-3/#content-property.
func validateContent(tokens []tinycss2.Node, baseURL string) (Value, error) {
	// Upstream collects the images and the URLs before the commas in a list
	// that it does not use.
	for len(tokens) >= 2 && tinycss2.IsLiteral(tokens[1], ",") {
		token := tokens[0]
		tokens = tokens[2:]
		parsedToken, err := getImage(token, baseURL)
		if err != nil {
			return nil, err
		}
		if parsedToken == nil {
			if parsedToken, err = getURL(token, baseURL); err != nil {
				return nil, err
			}
		}
		if parsedToken == nil {
			return nil, nil
		}
	}
	if len(tokens) == 0 {
		return nil, nil
	}
	if n := len(tokens); n >= 3 && tokens[n-1].Type() == tinycss2.TypeString && tinycss2.IsLiteral(tokens[n-2], "/") {
		// Ignore text for speech.
		tokens = tokens[:n-2]
	}
	if keyword := getSingleKeyword(tokens); keyword == "normal" || keyword == "none" {
		return Tuple{String(keyword)}, nil
	}
	return getContentList(tokens, baseURL)
}

// validateCounterIncrement is counter_increment in
// weasyprint/css/validation/properties.py.
func validateCounterIncrement(tokens []tinycss2.Node, _ string) (Value, error) {
	return counter(tokens, 1)
}

// validateCounterReset is counter_reset in
// weasyprint/css/validation/properties.py.
func validateCounterReset(tokens []tinycss2.Node, _ string) (Value, error) {
	return counter(tokens, 0)
}

// validateCounterSet is counter_set in
// weasyprint/css/validation/properties.py.
func validateCounterSet(tokens []tinycss2.Node, _ string) (Value, error) {
	return counter(tokens, 0)
}

// counter is counter in weasyprint/css/validation/properties.py, the
// validation of counter-increment, counter-reset and counter-set.
func counter(tokens []tinycss2.Node, defaultInteger int) (Value, error) {
	if getSingleKeyword(tokens) == "none" {
		return Tuple{}, nil
	}
	if len(tokens) == 0 {
		// Port: upstream asserts that the list is not empty.
		return nil, nil
	}
	var results Tuple
	i := 0
	for i < len(tokens) {
		ident, ok := tokens[i].(*tinycss2.IdentToken)
		if !ok {
			return nil, nil // Expected a keyword here.
		}
		counterName := ident.Value
		if in(counterName, "none", "initial", "inherit") {
			return nil, invalidValues("Invalid counter name: %s", counterName)
		}
		i++
		var integer Value = Int(defaultInteger)
		if i < len(tokens) {
			if number := getNumber(tokens[i], true, true); number != nil {
				// Found an integer. Use it and get the next token.
				integer = numberValue(number, true)
				i++
			}
		}
		// Not an integer: it might be the next counter name, which the next
		// iteration reads.
		results = append(results, Tuple{String(counterName), integer})
	}
	return results, nil
}

// validateLenghtPrecentageOrAuto is lenght_precentage_or_auto in
// weasyprint/css/validation/properties.py, the validator of margin-* and
// other properties.
var validateLenghtPrecentageOrAuto = singleToken(func(token tinycss2.Node, _ string) (Value, error) {
	if length := getLength(token, true, true); length != nil {
		return length, nil
	}
	if getKeyword(token) == "auto" {
		return String("auto"), nil
	}
	return nil, nil
})

// validateWidthHeight is width_height in
// weasyprint/css/validation/properties.py.
var validateWidthHeight = singleToken(func(token tinycss2.Node, _ string) (Value, error) {
	if length := getLength(token, false, true); length != nil {
		return length, nil
	}
	if getKeyword(token) == "auto" {
		return String("auto"), nil
	}
	return nil, nil
})

// validateGap is gap in weasyprint/css/validation/properties.py, the
// validator of column-gap and row-gap.
var validateGap = singleToken(func(token tinycss2.Node, _ string) (Value, error) {
	if length := getLength(token, false, true); length != nil {
		return length, nil
	}
	if getKeyword(token) == "normal" {
		return String("normal"), nil
	}
	return nil, nil
})

// validateColumnFill is column_fill in
// weasyprint/css/validation/properties.py.
var validateColumnFill = singleKeyword(keywordIn("auto", "balance"))

// validateDirection is direction in weasyprint/css/validation/properties.py.
var validateDirection = singleKeyword(keywordIn("ltr", "rtl"))

// validateDisplay is display in weasyprint/css/validation/properties.py.
func validateDisplay(tokens []tinycss2.Node, _ string) (Value, error) {
	values := make([]string, len(tokens))
	for i, token := range tokens {
		ident, ok := token.(*tinycss2.IdentToken)
		if !ok {
			return nil, nil
		}
		values[i] = ident.Value
	}

	if len(values) == 1 {
		value := values[0]
		switch value {
		case "none", "table-caption", "table-row-group", "table-cell",
			"table-header-group", "table-footer-group", "table-row",
			"table-column-group", "table-column":
			return Tuple{String(value)}, nil
		case "inline-table", "inline-flex", "inline-grid":
			return tupleOf(strings.Split(value, "-")...), nil
		case "inline-block":
			return tupleOf("inline", "flow-root"), nil
		}
	}

	var outside, inside, listItem string
	for _, value := range values {
		switch value {
		case "block", "inline":
			if outside != "" {
				return nil, nil
			}
			outside = value
		case "flow", "flow-root", "table", "flex", "grid":
			if inside != "" {
				return nil, nil
			}
			inside = value
		case "list-item":
			if listItem != "" {
				return nil, nil
			}
			listItem = value
		default:
			return nil, nil
		}
	}

	if outside == "" {
		outside = "block"
	}
	if inside == "" {
		inside = "flow"
	}
	if listItem != "" {
		if inside == "flow" || inside == "flow-root" {
			return tupleOf(outside, inside, listItem), nil
		}
		return nil, nil
	}
	return tupleOf(outside, inside), nil
}

// validateFloat is float_ in weasyprint/css/validation/properties.py.
var validateFloat = singleKeyword(keywordIn(
	"left", "right", "inline-start", "inline-end", "footnote", "none"))

// validateFontFamily is font_family in
// weasyprint/css/validation/properties.py.
var (
	validateFontFamilySingle validatorFunc = func(tokens []tinycss2.Node, _ string) (Value, error) {
		if len(tokens) == 1 {
			if s, ok := tokens[0].(*tinycss2.StringToken); ok {
				return String(s.Value), nil
			}
		}
		if len(tokens) == 0 {
			return nil, nil
		}
		names := make([]string, len(tokens))
		for i, token := range tokens {
			ident, ok := token.(*tinycss2.IdentToken)
			if !ok {
				return nil, nil
			}
			names[i] = ident.Value
		}
		return String(strings.Join(names, " ")), nil
	}
	validateFontFamily = commaSeparatedList(validateFontFamilySingle)
)

// validateFontKerning is font_kerning in
// weasyprint/css/validation/properties.py.
var validateFontKerning = singleKeyword(keywordIn("auto", "normal", "none"))

// validateFontLanguageOverride is font_language_override in
// weasyprint/css/validation/properties.py.
var validateFontLanguageOverride = singleToken(func(token tinycss2.Node, _ string) (Value, error) {
	if getKeyword(token) == "normal" {
		return String("normal"), nil
	}
	if s, ok := token.(*tinycss2.StringToken); ok {
		return String(s.Value), nil
	}
	return nil, nil
})

// fontVariantValues is the common part of font_variant_ligatures,
// font_variant_numeric and font_variant_east_asian in
// weasyprint/css/validation/properties.py: a list of keywords, with at most
// one of each couple.
func fontVariantValues(tokens []tinycss2.Node, couples [][]string) Value {
	var values []string
	for _, token := range tokens {
		ident, ok := token.(*tinycss2.IdentToken)
		if !ok {
			return nil
		}
		index := slices.IndexFunc(couples, func(couple []string) bool {
			return slices.Contains(couple, ident.Value)
		})
		if index < 0 {
			return nil
		}
		for _, value := range couples[index] {
			if slices.Contains(values, value) {
				return nil
			}
		}
		values = append(values, ident.Value)
	}
	if len(values) > 0 {
		return tupleOf(values...)
	}
	return nil
}

// validateFontVariantLigatures is font_variant_ligatures in
// weasyprint/css/validation/properties.py.
func validateFontVariantLigatures(tokens []tinycss2.Node, _ string) (Value, error) {
	if len(tokens) == 1 {
		if keyword := getKeyword(tokens[0]); keyword == "normal" || keyword == "none" {
			return String(keyword), nil
		}
	}
	return fontVariantValues(tokens, [][]string{
		{"common-ligatures", "no-common-ligatures"},
		{"historical-ligatures", "no-historical-ligatures"},
		{"discretionary-ligatures", "no-discretionary-ligatures"},
		{"contextual", "no-contextual"},
	}), nil
}

// validateFontVariantPosition is font_variant_position in
// weasyprint/css/validation/properties.py.
var validateFontVariantPosition = singleKeyword(keywordIn("normal", "sub", "super"))

// validateFontVariantCaps is font_variant_caps in
// weasyprint/css/validation/properties.py.
var validateFontVariantCaps = singleKeyword(keywordIn(
	"normal", "small-caps", "all-small-caps", "petite-caps",
	"all-petite-caps", "unicase", "titling-caps"))

// validateFontVariantNumeric is font_variant_numeric in
// weasyprint/css/validation/properties.py.
func validateFontVariantNumeric(tokens []tinycss2.Node, _ string) (Value, error) {
	if len(tokens) == 1 && getKeyword(tokens[0]) == "normal" {
		return String("normal"), nil
	}
	return fontVariantValues(tokens, [][]string{
		{"lining-nums", "oldstyle-nums"},
		{"proportional-nums", "tabular-nums"},
		{"diagonal-fractions", "stacked-fractions"},
		{"ordinal"}, {"slashed-zero"},
	}), nil
}

// fontFeatureSettingsList is font_feature_settings_list in
// weasyprint/css/validation/properties.py.
var fontFeatureSettingsList = commaSeparatedList(func(tokens []tinycss2.Node, _ string) (Value, error) {
	var feature, value Value
	switch len(tokens) {
	case 2:
		token := tokens[1]
		tokens = tokens[:1]
		if ident, ok := token.(*tinycss2.IdentToken); ok {
			switch ident.Value {
			case "on":
				value = Int(1)
			case "off":
				value = Int(0)
			}
		} else if number := getNumber(token, false, true); number != nil {
			value = numberValue(number, true)
		}
	case 1:
		value = Int(1)
	}

	if len(tokens) == 1 {
		if s, ok := tokens[0].(*tinycss2.StringToken); ok && len([]rune(s.Value)) == 4 {
			ascii := true
			for _, letter := range s.Value {
				if letter < 0x20 || letter > 0x7f {
					ascii = false
				}
			}
			if ascii {
				feature = String(s.Value)
			}
		}
	}

	if feature != nil && value != nil {
		return Tuple{feature, value}, nil
	}
	return nil, nil
})

// validateFontFeatureSettings is font_feature_settings in
// weasyprint/css/validation/properties.py.
func validateFontFeatureSettings(tokens []tinycss2.Node, baseURL string) (Value, error) {
	if len(tokens) == 1 && getKeyword(tokens[0]) == "normal" {
		return String("normal"), nil
	}
	return fontFeatureSettingsList(tokens, baseURL)
}

// validateFontVariantAlternates is font_variant_alternates in
// weasyprint/css/validation/properties.py.
//
// TODO: support other values. See
// https://drafts.csswg.org/css-fonts/#font-variant-alternates-prop
var validateFontVariantAlternates = singleKeyword(keywordIn("normal", "historical-forms"))

// validateFontVariantEastAsian is font_variant_east_asian in
// weasyprint/css/validation/properties.py.
func validateFontVariantEastAsian(tokens []tinycss2.Node, _ string) (Value, error) {
	if len(tokens) == 1 && getKeyword(tokens[0]) == "normal" {
		return String("normal"), nil
	}
	return fontVariantValues(tokens, [][]string{
		{"jis78", "jis83", "jis90", "jis04", "simplified", "traditional"},
		{"full-width", "proportional-width"},
		{"ruby"},
	}), nil
}

// fontVariationSettingsList is font_variation_settings_list in
// weasyprint/css/validation/properties.py.
var fontVariationSettingsList = commaSeparatedList(func(tokens []tinycss2.Node, _ string) (Value, error) {
	if len(tokens) == 2 {
		key, kok := tokens[0].(*tinycss2.StringToken)
		value, vok := tokens[1].(*tinycss2.NumberToken)
		if kok && vok {
			return Tuple{String(key.Value), Float(value.Value)}, nil
		}
	}
	return nil, nil
})

// validateFontVariationSettings is font_variation_settings in
// weasyprint/css/validation/properties.py.
func validateFontVariationSettings(tokens []tinycss2.Node, baseURL string) (Value, error) {
	if len(tokens) == 1 && getKeyword(tokens[0]) == "normal" {
		return String("normal"), nil
	}
	return fontVariationSettingsList(tokens, baseURL)
}

// validateFontSize is font_size in weasyprint/css/validation/properties.py.
var validateFontSize = singleToken(func(token tinycss2.Node, _ string) (Value, error) {
	if length := getLength(token, false, true); length != nil {
		return length, nil
	}
	keyword := getKeyword(token)
	if keyword == "smaller" || keyword == "larger" {
		return String(keyword), nil
	}
	if _, ok := fontSizeKeyword(keyword); ok {
		return String(keyword), nil
	}
	return nil, nil
})

// validateFontStyle is font_style in
// weasyprint/css/validation/properties.py.
var validateFontStyle = singleKeyword(keywordIn("normal", "italic", "oblique"))

// validateFontStretch is font_stretch in
// weasyprint/css/validation/properties.py.
var validateFontStretch = singleKeyword(keywordIn(
	"ultra-condensed", "extra-condensed", "condensed", "semi-condensed",
	"normal",
	"semi-expanded", "expanded", "extra-expanded", "ultra-expanded"))

// validateFontWeight is font_weight in
// weasyprint/css/validation/properties.py.
var validateFontWeight = singleToken(func(token tinycss2.Node, _ string) (Value, error) {
	if keyword := getKeyword(token); in(keyword, "normal", "bold", "bolder", "lighter") {
		return String(keyword), nil
	}
	if number, ok := token.(*tinycss2.NumberToken); ok && number.IsInteger {
		switch number.IntValue {
		case 100, 200, 300, 400, 500, 600, 700, 800, 900:
			return Int(number.IntValue), nil
		}
	}
	return nil, nil
})

// validateObjectFit is object_fit in
// weasyprint/css/validation/properties.py.
//
// TODO: Figure out what the spec means by "'scale-down' flag". As of this
// writing, neither Firefox nor chrome support anything other than a single
// keyword as is done here.
var validateObjectFit = singleKeyword(keywordIn("fill", "contain", "cover", "none", "scale-down"))

// validateImageResolution is image_resolution in
// weasyprint/css/validation/properties.py.
//
// TODO: support 'snap' and 'from-image'.
var validateImageResolution = singleToken(func(token tinycss2.Node, _ string) (Value, error) {
	if resolution, ok := getResolution(token); ok {
		return Float(resolution), nil
	}
	return nil, nil
})

// validateSpacing is spacing in weasyprint/css/validation/properties.py, the
// validator of letter-spacing and word-spacing.
var validateSpacing = singleToken(func(token tinycss2.Node, _ string) (Value, error) {
	if getKeyword(token) == "normal" {
		return String("normal"), nil
	}
	return getLength(token, true, false), nil
})

// validateOutlineOffset is outline_offset in
// weasyprint/css/validation/properties.py.
var validateOutlineOffset = singleToken(func(token tinycss2.Node, _ string) (Value, error) {
	return getLength(token, true, false), nil
})

// validateLineHeight is line_height in
// weasyprint/css/validation/properties.py.
var validateLineHeight = singleToken(func(token tinycss2.Node, _ string) (Value, error) {
	if getKeyword(token) == "normal" {
		return String("normal"), nil
	}
	if number := getNumber(token, false, false); number != nil {
		return number, nil
	}
	return getLength(token, false, true), nil
})

// validateListStylePosition is list_style_position in
// weasyprint/css/validation/properties.py.
var validateListStylePosition = singleKeyword(keywordIn("inside", "outside"))

// validateListStyleType is list_style_type in
// weasyprint/css/validation/properties.py.
var validateListStyleType = singleToken(func(token tinycss2.Node, _ string) (Value, error) {
	switch t := token.(type) {
	case *tinycss2.IdentToken:
		return String(t.Value), nil
	case *tinycss2.StringToken:
		return Tuple{String("string"), String(t.Value)}, nil
	case *tinycss2.FunctionBlock:
		if t.Name != "symbols" {
			return nil, nil
		}
		functionArguments := removeWhitespace(t.Arguments)
		if len(functionArguments) == 0 {
			return nil, nil
		}
		var arguments []string
		index := 0
		if ident, ok := functionArguments[0].(*tinycss2.IdentToken); ok {
			if !in(ident.Value, "cyclic", "numeric", "alphabetic", "symbolic", "fixed") {
				return nil, nil
			}
			index = 1
			arguments = append(arguments, ident.Value)
		} else {
			arguments = append(arguments, "symbolic")
		}
		if len(functionArguments) < index+1 {
			return nil, nil
		}
		for _, argument := range functionArguments[index:] {
			s, ok := argument.(*tinycss2.StringToken)
			if !ok {
				return nil, nil
			}
			arguments = append(arguments, s.Value)
		}
		if (arguments[0] == "alphabetic" || arguments[0] == "numeric") && len(arguments) < 3 {
			return nil, nil
		}
		return Tuple{String("symbols()"), tupleOf(arguments...)}, nil
	}
	return nil, nil
})

// validateMinWidthHeight is min_width_height in
// weasyprint/css/validation/properties.py. See
// https://www.w3.org/TR/css-flexbox-1/#min-size-auto.
var validateMinWidthHeight = singleToken(func(token tinycss2.Node, baseURL string) (Value, error) {
	if getKeyword(token) == "auto" {
		return String("auto"), nil
	}
	return validateLengthOrPrecentage([]tinycss2.Node{token}, baseURL)
})

// validateLengthOrPrecentage is length_or_precentage in
// weasyprint/css/validation/properties.py, the validator of padding-*.
var validateLengthOrPrecentage = singleToken(func(token tinycss2.Node, _ string) (Value, error) {
	return getLength(token, false, true), nil
})

// validateMaxWidthHeight is max_width_height in
// weasyprint/css/validation/properties.py.
var validateMaxWidthHeight = singleToken(func(token tinycss2.Node, _ string) (Value, error) {
	if length := getLength(token, false, true); length != nil {
		return length, nil
	}
	if getKeyword(token) == "none" {
		return Dimension{math.Inf(1), "px"}, nil
	}
	return nil, nil
})

// validateOpacity is opacity in weasyprint/css/validation/properties.py.
var validateOpacity = singleToken(func(token tinycss2.Node, _ string) (Value, error) {
	if number := getNumber(token, true, false); number != nil {
		return Float(min(1, max(0, number.(Dimension).Value))), nil
	}
	if percentage := getPercentage(token, true); percentage != nil {
		return Float(min(1, max(0, percentage.(Dimension).Value/100))), nil
	}
	return nil, nil
})

// validateZIndex is z_index in weasyprint/css/validation/properties.py.
var validateZIndex = singleToken(func(token tinycss2.Node, _ string) (Value, error) {
	if getKeyword(token) == "auto" {
		return String("auto"), nil
	}
	return numberValue(getNumber(token, true, true), true), nil
})

// positiveInteger returns the value of a positive integer token, or nil.
func positiveInteger(token tinycss2.Node) Value {
	if number := getNumber(token, false, true); number != nil && number.(Dimension).Value >= 1 {
		return numberValue(number, true)
	}
	return nil
}

// validateOrphansWidows is orphans_widows in
// weasyprint/css/validation/properties.py.
var validateOrphansWidows = singleToken(func(token tinycss2.Node, _ string) (Value, error) {
	return positiveInteger(token), nil
})

// validateColumnCount is column_count in
// weasyprint/css/validation/properties.py.
var validateColumnCount = singleToken(func(token tinycss2.Node, _ string) (Value, error) {
	if number := getNumber(token, false, true); number != nil {
		return positiveInteger(token), nil
	}
	if getKeyword(token) == "auto" {
		return String("auto"), nil
	}
	return nil, nil
})

// validateOverflow is overflow in weasyprint/css/validation/properties.py.
var validateOverflow = singleKeyword(keywordIn("auto", "visible", "hidden", "scroll"))

// validateTextOverflow is text_overflow in
// weasyprint/css/validation/properties.py.
var validateTextOverflow = singleKeyword(keywordIn("clip", "ellipsis"))

// validatePosition is position in weasyprint/css/validation/properties.py.
var validatePosition = singleToken(func(token tinycss2.Node, _ string) (Value, error) {
	if f, ok := token.(*tinycss2.FunctionBlock); ok && (f.Name == "running" || f.Name == "note") {
		if len(f.Arguments) == 1 {
			if ident, ok := f.Arguments[0].(*tinycss2.IdentToken); ok {
				return Tuple{String(f.Name + "()"), String(ident.Value)}, nil
			}
		}
	}
	if keyword := getSingleKeyword([]tinycss2.Node{token}); in(keyword, "static", "relative", "absolute", "fixed") {
		return String(keyword), nil
	}
	return nil, nil
})

// validateQuotes is quotes in weasyprint/css/validation/properties.py.
func validateQuotes(tokens []tinycss2.Node, _ string) (Value, error) {
	if len(tokens) == 1 {
		if keyword := getKeyword(tokens[0]); keyword == "auto" || keyword == "none" {
			return String(keyword), nil
		}
	}
	if len(tokens) == 0 || len(tokens)%2 != 0 {
		return nil, nil
	}
	var opens, closes Tuple
	for i, token := range tokens {
		s, ok := token.(*tinycss2.StringToken)
		if !ok {
			return nil, nil
		}
		// Separate open and close quotes.
		// eg.  ('«', '»', '“', '”')  -> (('«', '“'), ('»', '”'))
		if i%2 == 0 {
			opens = append(opens, String(s.Value))
		} else {
			closes = append(closes, String(s.Value))
		}
	}
	return Tuple{opens, closes}, nil
}

// validateTableLayout is table_layout in
// weasyprint/css/validation/properties.py.
var validateTableLayout = singleKeyword(keywordIn("fixed", "auto"))

// validateTextAlignAll is text_align_all in
// weasyprint/css/validation/properties.py.
var validateTextAlignAll = singleKeyword(keywordIn("left", "right", "center", "justify", "start", "end"))

// validateTextAlignLast is text_align_last in
// weasyprint/css/validation/properties.py.
var validateTextAlignLast = singleKeyword(keywordIn("auto", "left", "right", "center", "justify", "start", "end"))

// validateTextDecorationLine is text_decoration_line in
// weasyprint/css/validation/properties.py.
func validateTextDecorationLine(tokens []tinycss2.Node, _ string) (Value, error) {
	// The empty string stands for None in the set of keywords.
	keywords := map[string]bool{}
	for _, token := range tokens {
		keywords[getKeyword(token)] = true
	}
	if len(keywords) == 1 && keywords["none"] {
		return String("none"), nil
	}
	if len(tokens) != len(keywords) {
		return nil, nil
	}
	var set []string
	for keyword := range keywords {
		if !in(keyword, "underline", "overline", "line-through", "blink") {
			return nil, nil
		}
		set = append(set, keyword)
	}
	return newStringSet(set...), nil
}

// validateTextDecorationStyle is text_decoration_style in
// weasyprint/css/validation/properties.py.
var validateTextDecorationStyle = singleKeyword(keywordIn("solid", "double", "dotted", "dashed", "wavy"))

// validateTextDecorationThickness is text_decoration_thickness in
// weasyprint/css/validation/properties.py.
var validateTextDecorationThickness = singleToken(func(token tinycss2.Node, _ string) (Value, error) {
	if length := getLength(token, true, true); length != nil {
		return length, nil
	}
	if keyword := getKeyword(token); keyword == "auto" || keyword == "from-font" {
		return String(keyword), nil
	}
	return nil, nil
})

// validateTextIndent is text_indent in
// weasyprint/css/validation/properties.py.
var validateTextIndent = singleToken(func(token tinycss2.Node, _ string) (Value, error) {
	return getLength(token, true, true), nil
})

// validateTextTransform is text_transform in
// weasyprint/css/validation/properties.py.
var validateTextTransform = singleKeyword(keywordIn("none", "uppercase", "lowercase", "capitalize", "full-width"))

// validateVerticalAlign is vertical_align in
// weasyprint/css/validation/properties.py.
var validateVerticalAlign = singleToken(func(token tinycss2.Node, _ string) (Value, error) {
	if length := getLength(token, true, true); length != nil {
		return length, nil
	}
	if keyword := getKeyword(token); in(keyword, "baseline", "middle", "sub", "super",
		"text-top", "text-bottom", "top", "bottom") {
		return String(keyword), nil
	}
	return nil, nil
})

// validateVisibility is visibility in
// weasyprint/css/validation/properties.py.
var validateVisibility = singleKeyword(keywordIn("visible", "hidden", "collapse"))

// validateWhiteSpace is white_space in
// weasyprint/css/validation/properties.py.
var validateWhiteSpace = singleKeyword(keywordIn("normal", "pre", "nowrap", "pre-wrap", "pre-line"))

// validateOverflowWrap is overflow_wrap in
// weasyprint/css/validation/properties.py.
var validateOverflowWrap = singleKeyword(keywordIn("anywhere", "normal", "break-word"))

// validateWordBreak is word_break in
// weasyprint/css/validation/properties.py.
var validateWordBreak = singleKeyword(keywordIn("normal", "break-all"))

// validateFlexBasis is flex_basis in
// weasyprint/css/validation/properties.py.
var validateFlexBasis = singleToken(func(token tinycss2.Node, baseURL string) (Value, error) {
	if basis, err := validateWidthHeight([]tinycss2.Node{token}, baseURL); err != nil || basis != nil {
		return basis, err
	}
	if getKeyword(token) == "content" {
		return String("content"), nil
	}
	return nil, nil
})

// validateFlexDirection is flex_direction in
// weasyprint/css/validation/properties.py.
var validateFlexDirection = singleKeyword(keywordIn("row", "row-reverse", "column", "column-reverse"))

// validateFlexGrowShrink is flex_grow_shrink in
// weasyprint/css/validation/properties.py.
var validateFlexGrowShrink = singleToken(func(token tinycss2.Node, _ string) (Value, error) {
	return numberValue(getNumber(token, true, false), false), nil
})

// inflexibleBreadth is _inflexible_breadth in
// weasyprint/css/validation/properties.py. It parses inflexible-breadth.
func inflexibleBreadth(token tinycss2.Node) Value {
	keyword := getKeyword(token)
	switch {
	case in(keyword, "auto", "min-content", "max-content"):
		return String(keyword)
	case keyword != "":
		return nil
	}
	return getLength(token, false, true)
}

// trackBreadth is _track_breadth in
// weasyprint/css/validation/properties.py. It parses track-breadth.
func trackBreadth(token tinycss2.Node) Value {
	if d, ok := token.(*tinycss2.DimensionToken); ok && d.Value >= 0 && d.LowerUnit == "fr" {
		return Dimension{d.Value, d.LowerUnit}
	}
	return inflexibleBreadth(token)
}

// trackSize is _track_size in weasyprint/css/validation/properties.py. It
// parses track-size.
func trackSize(token tinycss2.Node) Value {
	if breadth := trackBreadth(token); breadth != nil {
		return breadth
	}
	f := newFunction(token)
	arguments := f.splitComma(false)
	switch f.name {
	case "minmax":
		if len(arguments) == 2 {
			inflexible := inflexibleBreadth(arguments[0])
			track := trackBreadth(arguments[1])
			if inflexible != nil && track != nil {
				return Tuple{String("minmax()"), inflexible, track}
			}
		}
	case "fit-content":
		if len(arguments) == 1 {
			if length := getLength(arguments[0], false, true); length != nil {
				return Tuple{String("fit-content()"), length}
			}
		}
	}
	return nil
}

// fixedSize is _fixed_size in weasyprint/css/validation/properties.py. It
// parses fixed-size.
func fixedSize(token tinycss2.Node) Value {
	if length := getLength(token, false, true); length != nil {
		return length
	}
	f := newFunction(token)
	arguments := f.splitComma(false)
	if f.name == "minmax" && len(arguments) == 2 {
		length := getLength(arguments[0], false, true)
		if length != nil {
			if track := trackBreadth(arguments[1]); track != nil {
				return Tuple{String("minmax()"), length, track}
			}
		}
		keyword := getKeyword(arguments[0])
		if in(keyword, "min-content", "max-content", "auto") || length != nil {
			if fixedBreadth := getLength(arguments[1], false, true); fixedBreadth != nil {
				if length == nil {
					length = String(keyword)
				}
				return Tuple{String("minmax()"), length, fixedBreadth}
			}
		}
	}
	return nil
}

// lineNames is _line_names in weasyprint/css/validation/properties.py. It
// parses line-names, and returns nil for None.
func lineNames(token tinycss2.Node) Value {
	block, ok := token.(*tinycss2.SquareBracketsBlock)
	if !ok {
		return nil
	}
	names := Tuple{}
	for _, item := range block.Content {
		if ident, ok := item.(*tinycss2.IdentToken); ok {
			names = append(names, String(ident.Value))
		} else if item.Type() != tinycss2.TypeWhitespace {
			return nil
		}
	}
	return names
}

// validateGridAuto is grid_auto in weasyprint/css/validation/properties.py.
func validateGridAuto(tokens []tinycss2.Node, _ string) (Value, error) {
	returnTokens := Tuple{}
	for _, token := range tokens {
		size := trackSize(token)
		if size == nil {
			return nil, nil
		}
		returnTokens = append(returnTokens, size)
	}
	return returnTokens, nil
}

// validateGridAutoFlow is grid_auto_flow in
// weasyprint/css/validation/properties.py.
func validateGridAutoFlow(tokens []tinycss2.Node, _ string) (Value, error) {
	switch len(tokens) {
	case 1:
		switch keyword := getKeyword(tokens[0]); keyword {
		case "row", "column":
			return Tuple{String(keyword)}, nil
		case "dense":
			return tupleOf(keyword, "row"), nil
		}
	case 2:
		keywords := keywordTuple(tokens)
		if tupleHas(keywords, "dense") && (tupleHas(keywords, "row") || tupleHas(keywords, "column")) {
			return keywords, nil
		}
	}
	return nil, nil
}

// repeatArguments returns the arguments of repeat() of a grid template: the
// repeat token and the tracks. It returns false when the token is not a
// function with two such arguments.
func repeatArguments(token tinycss2.Node) (f function, repeat tinycss2.Node, tracks []tinycss2.Node, ok bool) {
	f = newFunction(token)
	arguments := f.splitCommaGroups(false)
	if arguments == nil || len(arguments) != 2 {
		return f, nil, nil, false
	}
	if len(arguments[0]) != 1 || len(arguments[1]) == 0 {
		return f, nil, nil, false
	}
	return f, arguments[0][0], arguments[1], true
}

// validateGridTemplate is grid_template in
// weasyprint/css/validation/properties.py, the validator of
// grid-template-columns and grid-template-rows.
func validateGridTemplate(tokens []tinycss2.Node, _ string) (Value, error) {
	returnTokens := Tuple{}
	if len(tokens) == 0 {
		// Port: upstream raises IndexError for the empty list that the
		// expander of grid-template gives for "/".
		return nil, nil
	}
	if len(tokens) == 1 && getKeyword(tokens[0]) == "none" {
		return String("none"), nil
	}
	if getKeyword(tokens[0]) == "subgrid" {
		returnTokens = append(returnTokens, String("subgrid"))
		subgridTokens := Tuple{}
		for _, token := range tokens[1:] {
			if names := lineNames(token); names != nil {
				subgridTokens = append(subgridTokens, names)
				continue
			}
			f, repeat, tracks, ok := repeatArguments(token)
			if !ok || f.name != "repeat" {
				return nil, nil
			}
			var number Value
			if n, ok := repeat.(*tinycss2.NumberToken); ok && n.Value == math.Trunc(n.Value) && n.Value >= 1 {
				// A float such as 2.0 has the int_value None in upstream.
				if n.IsInteger {
					number = Int(clampInt(n.IntValue))
				}
			} else if getKeyword(repeat) == "auto-fill" {
				number = String("auto-fill")
			} else {
				return nil, nil
			}
			lineNamesList := Tuple{}
			for _, argument := range tracks {
				if names := lineNames(argument); names != nil {
					lineNamesList = append(lineNamesList, names)
				}
			}
			subgridTokens = append(subgridTokens, Tuple{String("repeat()"), number, lineNamesList})
		}
		return append(returnTokens, subgridTokens), nil
	}

	includesAutoRepeat := false
	includesTrack := false
	lastIsLineName := false
	for _, token := range tokens {
		if names := lineNames(token); names != nil {
			if lastIsLineName {
				return nil, nil
			}
			lastIsLineName = true
			returnTokens = append(returnTokens, names)
			continue
		}
		if size := fixedSize(token); size != nil {
			if !lastIsLineName {
				returnTokens = append(returnTokens, Tuple{})
			}
			lastIsLineName = false
			returnTokens = append(returnTokens, size)
			continue
		}
		if size := trackSize(token); size != nil {
			if !lastIsLineName {
				returnTokens = append(returnTokens, Tuple{})
			}
			lastIsLineName = false
			returnTokens = append(returnTokens, size)
			includesTrack = true
			continue
		}
		f, repeat, tracks, ok := repeatArguments(token)
		if !ok || f.name != "repeat" {
			return nil, nil
		}
		var number Value
		if n := getNumber(repeat, false, true); n != nil {
			number = n
			if n.(Dimension).Value >= 1 {
				number = numberValue(n, true)
			}
		} else if keyword := getKeyword(repeat); keyword == "auto-fill" || keyword == "auto-fit" {
			// Auto-repeat.
			if includesAutoRepeat {
				return nil, nil
			}
			value, _ := tokenValue(repeat)
			number = String(value)
			includesAutoRepeat = true
		} else {
			return nil, nil
		}
		namesAndSizes := Tuple{}
		repeatLastIsLineName := false
		for _, arg := range tracks {
			if names := lineNames(arg); names != nil {
				if repeatLastIsLineName {
					return nil, nil
				}
				namesAndSizes = append(namesAndSizes, names)
				repeatLastIsLineName = true
				continue
			}
			// Fixed-repeat.
			if size := fixedSize(arg); size != nil {
				if !repeatLastIsLineName {
					namesAndSizes = append(namesAndSizes, Tuple{})
				}
				repeatLastIsLineName = false
				namesAndSizes = append(namesAndSizes, size)
				continue
			}
			// Track-repeat.
			if size := trackSize(arg); size != nil {
				includesTrack = true
				if !repeatLastIsLineName {
					namesAndSizes = append(namesAndSizes, Tuple{})
				}
				repeatLastIsLineName = false
				namesAndSizes = append(namesAndSizes, size)
				continue
			}
			return nil, nil
		}
		if !lastIsLineName {
			returnTokens = append(returnTokens, Tuple{})
		}
		lastIsLineName = false
		if !repeatLastIsLineName {
			namesAndSizes = append(namesAndSizes, Tuple{})
		}
		returnTokens = append(returnTokens, Tuple{String("repeat()"), number, namesAndSizes})
	}
	if includesAutoRepeat && includesTrack {
		return nil, nil
	}
	if !lastIsLineName {
		returnTokens = append(returnTokens, Tuple{})
	}
	return returnTokens, nil
}

// validateGridTemplateAreas is grid_template_areas in
// weasyprint/css/validation/properties.py.
func validateGridTemplateAreas(tokens []tinycss2.Node, _ string) (Value, error) {
	if len(tokens) == 1 && getKeyword(tokens[0]) == "none" {
		return String("none"), nil
	}
	// A cell is the name of an area, or "" for the None of a dot.
	var gridAreas [][]string
	for _, token := range tokens {
		s, ok := token.(*tinycss2.StringToken)
		if !ok {
			return nil, nil
		}
		var row []string
		lastIsDot := false
		for _, value := range tinycss2.ParseComponentValueList(s.Value) {
			switch v := value.(type) {
			case *tinycss2.IdentToken:
				row = append(row, v.Value)
				lastIsDot = false
			case *tinycss2.LiteralToken:
				if v.Value != "." {
					return nil, nil
				}
				if lastIsDot {
					continue
				}
				row = append(row, "")
				lastIsDot = true
			case *tinycss2.WhitespaceToken:
				lastIsDot = false
			default:
				return nil, nil
			}
		}
		if len(row) == 0 {
			return nil, nil
		}
		gridAreas = append(gridAreas, row)
	}
	// Check that the rows and the columns have the same sizes.
	for _, row := range gridAreas {
		if len(row) != len(gridAreas[0]) {
			return nil, nil
		}
	}
	// Check that the areas are continuous rectangles.
	type point struct{ x, y int }
	coordinates := map[point]bool{}
	areas := map[string]bool{}
	for y, row := range gridAreas {
		for x, area := range row {
			if coordinates[point{x, y}] || area == "" {
				continue
			}
			if areas[area] {
				return nil, nil
			}
			areas[area] = true
			coordinates[point{x, y}] = true
			var nx int
			broke := false
			for nx = x + 1; nx < len(row); nx++ {
				if row[nx] != area {
					broke = true
					break
				}
				coordinates[point{nx, y}] = true
			}
			if !broke {
				// The for else of upstream adds one to the last index.
				nx = len(row)
			}
			for ny := y + 1; ny < len(gridAreas); ny++ {
				nrow := gridAreas[ny]
				same := x < nx
				for _, narea := range nrow[x:min(nx, len(nrow))] {
					if narea != area {
						same = false
					}
				}
				if !same {
					break
				}
				for nnx := x; nnx < nx; nnx++ {
					coordinates[point{nnx, ny}] = true
				}
			}
		}
	}
	result := make(Tuple, len(gridAreas))
	for i, row := range gridAreas {
		t := make(Tuple, len(row))
		for j, area := range row {
			if area != "" {
				t[j] = String(area)
			}
		}
		result[i] = t
	}
	return result, nil
}

// validateGridLine is grid_line in weasyprint/css/validation/properties.py,
// the validator of grid-[row|column]-[start|end].
func validateGridLine(tokens []tinycss2.Node, _ string) (Value, error) {
	if len(tokens) == 1 {
		token := tokens[0]
		if keyword := getKeyword(token); keyword != "" {
			if keyword == "auto" {
				return String(keyword), nil
			} else if keyword != "span" {
				return Tuple{nil, nil, String(token.(*tinycss2.IdentToken).Value)}, nil
			}
		} else if number := getNumber(token, true, true); number != nil {
			if number.(Dimension).Value != 0 {
				return Tuple{nil, numberValue(number, true), nil}, nil
			}
		}
		return nil, nil
	}
	var number, ident, span Value
	for _, token := range tokens {
		if keyword := getKeyword(token); keyword != "" {
			if keyword == "auto" {
				return nil, nil
			}
			if keyword == "span" {
				if span == nil {
					span = String("span")
					continue
				}
			} else if ident == nil {
				ident = String(token.(*tinycss2.IdentToken).Value)
				continue
			}
		} else if item := getNumber(token, true, true); item != nil {
			if item.(Dimension).Value != 0 && number == nil {
				number = numberValue(item, true)
				continue
			}
		}
		return nil, nil
	}
	if span != nil {
		if n, ok := number.(Int); ok && n < 0 {
			return nil, nil
		} else if ident != nil || number != nil {
			return Tuple{span, number, ident}, nil
		}
	} else if number != nil {
		return Tuple{span, number, ident}, nil
	}
	return nil, nil
}

// validateFlexWrap is flex_wrap in weasyprint/css/validation/properties.py.
var validateFlexWrap = singleKeyword(keywordIn("nowrap", "wrap", "wrap-reverse"))

// alignment is the common part of the validators of justify-* and align-*
// in weasyprint/css/validation/properties.py. single holds the keywords of
// one token, baseline allows baseline alone, safe holds the keywords after
// safe or unsafe, and legacy allows legacy with left, right or center.
func alignment(tokens []tinycss2.Node, single []string, baseline bool, safe []string, legacy bool) Value {
	switch len(tokens) {
	case 1:
		keyword := getKeyword(tokens[0])
		if in(keyword, single...) {
			return Tuple{String(keyword)}
		} else if baseline && keyword == "baseline" {
			return tupleOf("first", keyword)
		}
	case 2:
		keywords := keywordTuple(tokens)
		switch {
		case isStrIn(keywords[0], "safe", "unsafe"):
			if isStrIn(keywords[1], safe...) {
				return keywords
			}
		case tupleHas(keywords, "baseline"):
			if tupleHas(keywords, "first") || tupleHas(keywords, "last") {
				return keywords
			}
		case legacy && tupleHas(keywords, "legacy"):
			if tupleHas(keywords, "left") || tupleHas(keywords, "right") || tupleHas(keywords, "center") {
				return keywords
			}
		}
	}
	return nil
}

// validateJustifyContent is justify_content in
// weasyprint/css/validation/properties.py.
func validateJustifyContent(tokens []tinycss2.Node, _ string) (Value, error) {
	switch len(tokens) {
	case 1:
		if keyword := getKeyword(tokens[0]); in(keyword,
			"center", "space-between", "space-around", "space-evenly",
			"stretch", "normal", "flex-start", "flex-end",
			"start", "end", "left", "right") {
			return Tuple{String(keyword)}, nil
		}
	case 2:
		keywords := keywordTuple(tokens)
		if isStrIn(keywords[0], "safe", "unsafe") && isStrIn(keywords[1],
			"center", "start", "end", "flex-start", "flex-end", "left", "right") {
			return keywords, nil
		}
	}
	return nil, nil
}

// validateJustifyItems is justify_items in
// weasyprint/css/validation/properties.py.
func validateJustifyItems(tokens []tinycss2.Node, _ string) (Value, error) {
	return alignment(tokens,
		[]string{
			"normal", "stretch", "center", "start", "end", "self-start",
			"self-end", "flex-start", "flex-end", "left", "right", "legacy",
		}, true,
		[]string{
			"center", "start", "end", "self-start", "self-end",
			"flex-start", "flex-end", "left", "right",
		}, true), nil
}

// validateJustifySelf is justify_self in
// weasyprint/css/validation/properties.py.
func validateJustifySelf(tokens []tinycss2.Node, _ string) (Value, error) {
	return alignment(tokens,
		[]string{
			"auto", "normal", "stretch", "center", "start", "end",
			"self-start", "self-end", "flex-start", "flex-end", "left", "right",
		}, true,
		[]string{
			"center", "start", "end", "self-start", "self-end",
			"flex-start", "flex-end", "left", "right",
		}, false), nil
}

// validateAlignItems is align_items in
// weasyprint/css/validation/properties.py.
func validateAlignItems(tokens []tinycss2.Node, _ string) (Value, error) {
	return alignment(tokens,
		[]string{
			"normal", "stretch", "center", "start", "end", "self-start",
			"self-end", "flex-start", "flex-end",
		}, true,
		[]string{
			"center", "start", "end", "self-start", "self-end",
			"flex-start", "flex-end",
		}, false), nil
}

// validateAlignSelf is align_self in
// weasyprint/css/validation/properties.py.
func validateAlignSelf(tokens []tinycss2.Node, _ string) (Value, error) {
	return alignment(tokens,
		[]string{
			"auto", "normal", "stretch", "center", "start", "end",
			"self-start", "self-end", "flex-start", "flex-end",
		}, true,
		[]string{
			"center", "start", "end", "self-start", "self-end",
			"flex-start", "flex-end",
		}, false), nil
}

// validateAlignContent is align_content in
// weasyprint/css/validation/properties.py.
func validateAlignContent(tokens []tinycss2.Node, _ string) (Value, error) {
	return alignment(tokens,
		[]string{
			"center", "space-between", "space-around", "space-evenly",
			"stretch", "normal", "flex-start", "flex-end", "start", "end",
		}, true,
		[]string{"center", "start", "end", "flex-start", "flex-end"}, false), nil
}

// validateOrder is order in weasyprint/css/validation/properties.py.
var validateOrder = singleToken(func(token tinycss2.Node, _ string) (Value, error) {
	return numberValue(getNumber(token, true, true), true), nil
})

// validateImageRendering is image_rendering in
// weasyprint/css/validation/properties.py.
var validateImageRendering = singleKeyword(keywordIn("auto", "crisp-edges", "pixelated"))

// validateImageOrientation is image_orientation in
// weasyprint/css/validation/properties.py.
func validateImageOrientation(tokens []tinycss2.Node, _ string) (Value, error) {
	if keyword := getSingleKeyword(tokens); keyword == "none" || keyword == "from-image" {
		return String(keyword), nil
	}
	var angle Value
	var flip Value
	for _, token := range tokens {
		if getKeyword(token) == "flip" {
			if flip != nil {
				return nil, nil
			}
			flip = Bool(true)
			continue
		}
		if angle == nil {
			if a, ok := getAngle(token); ok {
				angle = Float(a)
				continue
			}
		}
		return nil, nil
	}
	if angle == nil {
		angle = Int(0)
	}
	if flip == nil {
		flip = Bool(false)
	}
	return Tuple{angle, flip}, nil
}

// pageSizeTuple returns a page size as a Tuple of two Dimension values.
func pageSizeTuple(size [2]Dimension) Tuple {
	return Tuple{size[0], size[1]}
}

// validateSize is size in weasyprint/css/validation/properties.py. See
// https://www.w3.org/TR/css-page-3/#page-size-prop.
func validateSize(tokens []tinycss2.Node, _ string) (Value, error) {
	lengths := make(Tuple, len(tokens))
	for i, token := range tokens {
		lengths[i] = getLength(token, false, false)
	}
	if v := twoLengths(lengths); v != nil {
		return v, nil
	}

	keywords := make([]string, len(tokens))
	for i, token := range tokens {
		keywords[i] = getKeyword(token)
	}
	switch len(keywords) {
	case 1:
		keyword := keywords[0]
		if size, ok := PageSizes[keyword]; ok {
			return pageSizeTuple(size), nil
		} else if keyword == "auto" || keyword == "portrait" {
			return pageSizeTuple(initialPageSize), nil
		} else if keyword == "landscape" {
			return Tuple{initialPageSize[1], initialPageSize[0]}, nil
		}
	case 2:
		var orientation, pageSize string
		switch {
		case in(keywords[0], "portrait", "landscape"):
			orientation, pageSize = keywords[0], keywords[1]
		case in(keywords[1], "portrait", "landscape"):
			pageSize, orientation = keywords[0], keywords[1]
		}
		if size, ok := PageSizes[pageSize]; ok && pageSize != "" {
			if orientation == "portrait" {
				return pageSizeTuple(size), nil
			}
			return Tuple{size[1], size[0]}, nil
		}
	}
	return nil, nil
}

// validateAnchor is anchor in weasyprint/css/validation/properties.py.
var validateAnchor = singleToken(func(token tinycss2.Node, _ string) (Value, error) {
	if s, ok := token.(*tinycss2.StringToken); ok {
		return String(s.Value), nil
	}
	return nil, nil
})

// validateTabSize is tab_size in weasyprint/css/validation/properties.py.
// See https://www.w3.org/TR/css-text-3/#tab-size.
var validateTabSize = singleToken(func(token tinycss2.Node, _ string) (Value, error) {
	if number, ok := token.(*tinycss2.NumberToken); ok && number.IsInteger && number.IntValue != 0 {
		return Int(clampInt(number.IntValue)), nil
	}
	return getLength(token, false, false), nil
})

// validateHyphens is hyphens in weasyprint/css/validation/properties.py.
var validateHyphens = singleToken(func(token tinycss2.Node, _ string) (Value, error) {
	if keyword := getKeyword(token); in(keyword, "none", "manual", "auto") {
		return String(keyword), nil
	}
	return nil, nil
})

// validateHyphenateCharacter is hyphenate_character in
// weasyprint/css/validation/properties.py.
var validateHyphenateCharacter = singleToken(func(token tinycss2.Node, _ string) (Value, error) {
	if getKeyword(token) == "auto" {
		return String("‐"), nil
	}
	if s, ok := token.(*tinycss2.StringToken); ok {
		return String(s.Value), nil
	}
	return nil, nil
})

// validateHyphenateLimitZone is hyphenate_limit_zone in
// weasyprint/css/validation/properties.py.
var validateHyphenateLimitZone = singleToken(func(token tinycss2.Node, _ string) (Value, error) {
	return getLength(token, false, true), nil
})

// validateHyphenateLimitChars is hyphenate_limit_chars in
// weasyprint/css/validation/properties.py.
func validateHyphenateLimitChars(tokens []tinycss2.Node, _ string) (Value, error) {
	integer := func(token tinycss2.Node) Value {
		return numberValue(getNumber(token, true, true), true)
	}
	switch len(tokens) {
	case 1:
		token := tokens[0]
		if getKeyword(token) == "auto" {
			return Tuple{Int(5), Int(2), Int(2)}, nil
		} else if number := integer(token); number != nil {
			return Tuple{number, Int(2), Int(2)}, nil
		}
	case 2:
		total, left := tokens[0], tokens[1]
		totalKeyword := getKeyword(total)
		leftKeyword := getKeyword(left)
		if totalNumber := integer(total); totalNumber != nil {
			if leftNumber := integer(left); leftNumber != nil {
				return Tuple{totalNumber, leftNumber, leftNumber}, nil
			} else if leftKeyword == "auto" {
				return Tuple{totalNumber, Int(2), Int(2)}, nil
			}
		} else if totalKeyword == "auto" {
			if leftNumber := integer(left); leftNumber != nil {
				return Tuple{Int(5), leftNumber, leftNumber}, nil
			} else if leftKeyword == "auto" {
				return Tuple{Int(5), Int(2), Int(2)}, nil
			}
		}
	case 3:
		var result Tuple
		for i, token := range tokens {
			if getKeyword(token) == "auto" {
				if i == 0 {
					result = append(result, Int(5))
				} else {
					result = append(result, Int(2))
				}
			} else if number := integer(token); number != nil {
				result = append(result, number)
			} else {
				return nil, nil
			}
		}
		return result, nil
	}
	return nil, nil
}

// validateLang is lang in weasyprint/css/validation/properties.py.
var validateLang = singleToken(func(token tinycss2.Node, _ string) (Value, error) {
	if getKeyword(token) == "none" {
		return String("none"), nil
	}
	if s, ok := token.(*tinycss2.StringToken); ok {
		return Tuple{String("string"), String(s.Value)}, nil
	}
	return nil, nil
})

// contentListTokens parses each token with get_content_list_token, and
// returns nil when one of them is None.
func contentListTokens(tokens []tinycss2.Node, baseURL string) (Value, error) {
	parsedTokens := make(Tuple, len(tokens))
	for i, token := range tokens {
		parsed, err := getContentListToken(token, baseURL)
		if err != nil {
			return nil, err
		}
		parsedTokens[i] = parsed
	}
	for _, parsed := range parsedTokens {
		if parsed == nil {
			return nil, nil
		}
	}
	return parsedTokens, nil
}

// validateBookmarkLabel is bookmark_label in
// weasyprint/css/validation/properties.py.
func validateBookmarkLabel(tokens []tinycss2.Node, baseURL string) (Value, error) {
	return contentListTokens(tokens, baseURL)
}

// validateBookmarkLevel is bookmark_level in
// weasyprint/css/validation/properties.py.
var validateBookmarkLevel = singleToken(func(token tinycss2.Node, _ string) (Value, error) {
	if number := getNumber(token, false, true); number != nil {
		return positiveInteger(token), nil
	}
	if getKeyword(token) == "none" {
		return String("none"), nil
	}
	return nil, nil
})

// validateBookmarkState is bookmark_state in
// weasyprint/css/validation/properties.py.
var validateBookmarkState = singleKeyword(keywordIn("open", "closed"))

// validateFootnoteDisplay is footnote_display in
// weasyprint/css/validation/properties.py.
var validateFootnoteDisplay = singleKeyword(keywordIn("block", "inline", "compact"))

// validateFootnotePolicy is footnote_policy in
// weasyprint/css/validation/properties.py.
var validateFootnotePolicy = singleKeyword(keywordIn("auto", "line", "block"))

// validateStringSet is string_set in
// weasyprint/css/validation/properties.py.
var (
	validateStringSetSingle validatorFunc = func(tokens []tinycss2.Node, baseURL string) (Value, error) {
		// Spec asks for strings after custom keywords, but we allow
		// content-lists.
		if len(tokens) >= 2 {
			varName := getCustomIdent(tokens[0])
			if varName == "" {
				return nil, nil
			}
			parsedTokens, err := contentListTokens(tokens[1:], baseURL)
			if err != nil || parsedTokens == nil {
				return nil, err
			}
			return Tuple{String(varName), parsedTokens}, nil
		} else if len(tokens) > 0 && getKeyword(tokens[0]) == "none" {
			return Tuple{String("none"), Tuple{}}, nil
		}
		return nil, nil
	}
	validateStringSet = commaSeparatedList(validateStringSetSingle)
)

// validateTransform is transform in weasyprint/css/validation/properties.py.
func validateTransform(tokens []tinycss2.Node, _ string) (Value, error) {
	if getSingleKeyword(tokens) == "none" {
		return Tuple{}, nil
	}
	var transforms Tuple
	for _, token := range tokens {
		f := newFunction(token)
		if !f.ok {
			return nil, nil
		}
		arguments := f.splitComma(false)

		allNumbers := len(arguments) > 0
		numbers := make(Tuple, len(arguments))
		for i, argument := range arguments {
			n, ok := argument.(*tinycss2.NumberToken)
			if !ok {
				allNumbers = false
				continue
			}
			numbers[i] = Float(n.Value)
		}
		switch {
		case len(arguments) == 1:
			angle, hasAngle := getAngle(arguments[0])
			length := getLength(arguments[0], true, true)
			switch {
			case f.name == "rotate" && hasAngle:
				transforms = append(transforms, Tuple{String(f.name), Float(angle)})
			case (f.name == "skewx" || f.name == "skew") && hasAngle:
				transforms = append(transforms, Tuple{String("skew"), Tuple{Float(angle), Int(0)}})
			case f.name == "skewy" && hasAngle:
				transforms = append(transforms, Tuple{String("skew"), Tuple{Int(0), Float(angle)}})
			case (f.name == "translatex" || f.name == "translate") && length != nil:
				transforms = append(transforms, Tuple{String("translate"), Tuple{length, ZeroPixels}})
			case f.name == "translatey" && length != nil:
				transforms = append(transforms, Tuple{String("translate"), Tuple{ZeroPixels, length}})
			case f.name == "scalex" && allNumbers:
				transforms = append(transforms, Tuple{String("scale"), Tuple{numbers[0], Int(1)}})
			case f.name == "scaley" && allNumbers:
				transforms = append(transforms, Tuple{String("scale"), Tuple{Int(1), numbers[0]}})
			case f.name == "scale" && allNumbers:
				transforms = append(transforms, Tuple{String("scale"), Tuple{numbers[0], numbers[0]}})
			default:
				return nil, nil
			}
		case len(arguments) == 2:
			switch {
			case f.name == "scale" && allNumbers:
				transforms = append(transforms, Tuple{String(f.name), numbers})
			case f.name == "translate":
				lengths := make(Tuple, 2)
				for i, argument := range arguments {
					lengths[i] = getLength(argument, true, true)
					if lengths[i] == nil {
						return nil, nil
					}
				}
				transforms = append(transforms, Tuple{String(f.name), lengths})
			case f.name == "skew":
				angles := make(Tuple, 2)
				for i, argument := range arguments {
					angle, ok := getAngle(argument)
					if !ok {
						return nil, nil
					}
					angles[i] = Float(angle)
				}
				transforms = append(transforms, Tuple{String(f.name), angles})
			default:
				return nil, nil
			}
		case len(arguments) == 6 && f.name == "matrix" && allNumbers:
			transforms = append(transforms, Tuple{String(f.name), numbers})
		default:
			return nil, nil
		}
	}
	return transforms, nil
}

// validateAppearance is appearance in
// weasyprint/css/validation/properties.py.
var validateAppearance = singleToken(func(token tinycss2.Node, _ string) (Value, error) {
	if keyword := getKeyword(token); keyword == "none" || keyword == "auto" {
		return String(keyword), nil
	}
	return nil, nil
})

// validateColorScheme is color_scheme in
// weasyprint/css/validation/properties.py.
func validateColorScheme(tokens []tinycss2.Node, _ string) (Value, error) {
	if len(tokens) == 1 {
		keyword := getSingleKeyword(tokens)
		switch keyword {
		case "normal":
			return String(keyword), nil
		case "only":
			return nil, nil
		}
		return Tuple{keywordOrNone(keyword)}, nil
	}
	var keywords Tuple
	only := false
	for i, token := range tokens {
		keyword := getKeyword(token)
		switch {
		case keyword == "only":
			if only || (i != 0 && i != len(tokens)-1) {
				return nil, nil
			}
			only = true
		case keyword == "normal":
			return nil, nil
		case keyword != "":
			keywords = append(keywords, String(keyword))
		default:
			return nil, nil
		}
	}
	if only {
		keywords = append(keywords, String("only"))
	}
	if keywords == nil {
		keywords = Tuple{}
	}
	return keywords, nil
}
