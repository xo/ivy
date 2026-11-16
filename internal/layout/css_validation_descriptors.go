// Copyright (c) 2011-2021, Simon Sapin and contributors.
//
// Use of this source code is governed by the BSD-3-Clause license that can be
// found in the LICENSE file.

// This file ports weasyprint/css/validation/descriptors.py: the validation of
// the descriptors of some at-rules.

package layout

import (
	"errors"
	"math"
	"slices"
	"strings"

	"github.com/xo/okage/tinycss2"
)

// descriptors is DESCRIPTORS in weasyprint/css/validation/descriptors.py:
// the validators of the descriptors of each at-rule, by rule and by name in
// kebab case.
var descriptors = map[string]map[string]validatorFunc{
	"font-face":     {},
	"counter-style": {},
	"color-profile": {},
}

// descriptorsNotPrintMedia is NOT_PRINT_MEDIA in
// weasyprint/css/validation/descriptors.py: the descriptors that do not
// apply to the print media.
var descriptorsNotPrintMedia = []string{"font-display"}

// noneFakeToken and normalFakeToken are NoneFakeToken and NormalFakeToken
// in weasyprint/css/validation/descriptors.py: keywords that get_keyword
// reads.
func noneFakeToken() tinycss2.Node   { return tinycss2.NewIdentToken(0, 0, "none") }
func normalFakeToken() tinycss2.Node { return tinycss2.NewIdentToken(0, 0, "normal") }

// preprocessDescriptors is preprocess_descriptors in
// weasyprint/css/validation/descriptors.py. It filters the unsupported names
// and values of the descriptors of a rule, and logs a warning for each
// ignored descriptor. It returns the pairs (name, value), with the names in
// snake case.
func preprocessDescriptors(log *logger, rule, baseURL string, declarations []tinycss2.Node) []nameValue {
	var result []nameValue
	for _, node := range declarations {
		descriptor, ok := node.(*tinycss2.Declaration)
		if !ok || descriptor.Important {
			continue
		}
		tokens := removeWhitespace(descriptor.Value)
		if slices.Contains(descriptorsNotPrintMedia, descriptor.Name) {
			continue
		}
		var value Value
		var err error
		if function, ok := descriptors[rule][descriptor.Name]; !ok {
			err = invalidValues("descriptor not supported")
		} else {
			value, err = function(tokens, baseURL)
			if err == nil && value == nil {
				err = errInvalid
			}
		}
		if err != nil {
			reason := "invalid value"
			var iv *InvalidValuesError
			if errors.As(err, &iv) && iv.Message != "" {
				reason = iv.Message
			}
			log.warning(
				"Ignored `%s:%s` at %d:%d, %s.",
				descriptor.Name, tinycss2.Serialize(descriptor.Value),
				descriptor.SourceLine, descriptor.SourceColumn, reason)
			continue
		}
		result = append(result, nameValue{strings.ReplaceAll(descriptor.Name, "-", "_"), value})
	}
	return result
}

// descriptor is the decorator descriptor in
// weasyprint/css/validation/descriptors.py. It registers the validator of a
// descriptor of a rule, by name in kebab case.
func descriptor(rule, name string, f validatorFunc) {
	if _, ok := descriptors[rule][name]; ok {
		panic("descriptor registered twice " + name)
	}
	descriptors[rule][name] = f
}

func init() {
	descriptor("font-face", "font-family", func(tokens []tinycss2.Node, _ string) (Value, error) {
		return descriptorFontFamily(tokens, false), nil
	})
	descriptor("font-face", "src", commaSeparatedList(descriptorSrc))
	descriptor("font-face", "font-style", singleKeyword(descriptorFontStyle))
	descriptor("font-face", "font-weight", singleToken(descriptorFontWeight))
	descriptor("font-face", "font-stretch", singleKeyword(descriptorFontStretch))
	descriptor("font-face", "font-feature-settings", descriptorFontFeatureSettings)
	descriptor("font-face", "font-variant", descriptorFontVariant)
	descriptor("font-face", "unicode-range", commaSeparatedList(singleToken(descriptorUnicodeRange)))
	descriptor("counter-style", "system", descriptorSystem)
	descriptor("counter-style", "negative", descriptorNegative)
	descriptor("counter-style", "prefix", descriptorPrefixSuffix)
	descriptor("counter-style", "suffix", descriptorPrefixSuffix)
	descriptor("counter-style", "range", commaSeparatedList(descriptorRange))
	descriptor("counter-style", "pad", descriptorPad)
	descriptor("counter-style", "fallback", singleToken(descriptorFallback))
	descriptor("counter-style", "symbols", descriptorSymbols)
	descriptor("counter-style", "additive-symbols", descriptorAdditiveSymbols)
	descriptor("color-profile", "src", singleToken(descriptorColorProfileSrc))
	descriptor("color-profile", "rendering-intent", singleKeyword(descriptorRenderingIntent))
	descriptor("color-profile", "components", commaSeparatedList(descriptorComponents))
}

// nameTokens is a pair that an expander wrapped by generic_expander yields:
// a name or a suffix, such as "-caps", and the tokens of its value.
type nameTokens struct {
	name   string
	tokens []tinycss2.Node
}

// fontVariantFeatures are the features of expand_font_variant, in order,
// with the validators of their properties.
var fontVariantFeatures = []struct {
	name     string
	validate validatorFunc
}{
	{"alternates", validateFontVariantAlternates},
	{"caps", validateFontVariantCaps},
	{"east-asian", validateFontVariantEastAsian},
	{"ligatures", validateFontVariantLigatures},
	{"numeric", validateFontVariantNumeric},
	{"position", validateFontVariantPosition},
}

// expandFontVariant is expand_font_variant in
// weasyprint/css/validation/descriptors.py. It returns the suffixes of the
// font-variant-* properties and their tokens.
func expandFontVariant(tokens []tinycss2.Node) ([]nameTokens, error) {
	keyword := getSingleKeyword(tokens)
	if keyword == "normal" || keyword == "none" {
		var result []nameTokens
		for _, suffix := range [...]string{"-alternates", "-caps", "-east-asian", "-numeric", "-position"} {
			result = append(result, nameTokens{suffix, []tinycss2.Node{normalFakeToken()}})
		}
		token := normalFakeToken()
		if keyword == "none" {
			token = noneFakeToken()
		}
		return append(result, nameTokens{"-ligatures", []tinycss2.Node{token}}), nil
	}
	features := make([][]tinycss2.Node, len(fontVariantFeatures))
	for _, token := range tokens {
		if getKeyword(token) == "normal" {
			// We do not allow 'normal', only the specific values.
			return nil, errInvalid
		}
		found := false
		for i, feature := range fontVariantFeatures {
			value, err := feature.validate([]tinycss2.Node{token}, "")
			if err == nil && truthy(value) {
				features[i] = append(features[i], token)
				found = true
				break
			}
		}
		if !found {
			return nil, errInvalid
		}
	}
	var result []nameTokens
	for i, feature := range fontVariantFeatures {
		if len(features[i]) > 0 {
			result = append(result, nameTokens{"-" + feature.name, features[i]})
		}
	}
	return result, nil
}

// descriptorFontFamily is font_family in
// weasyprint/css/validation/descriptors.py, the validation of the
// font-family descriptor. It returns nil for None.
func descriptorFontFamily(tokens []tinycss2.Node, allowSpaces bool) Value {
	if len(tokens) == 1 {
		if s, ok := tokens[0].(*tinycss2.StringToken); ok {
			return String(s.Value)
		}
	}
	if len(tokens) == 0 {
		return nil
	}
	var names []string
	for _, token := range tokens {
		switch t := token.(type) {
		case *tinycss2.IdentToken:
			names = append(names, t.Value)
		case *tinycss2.WhitespaceToken:
			if !allowSpaces {
				return nil
			}
		default:
			return nil
		}
	}
	return String(strings.Join(names, " "))
}

// descriptorSrc is src in weasyprint/css/validation/descriptors.py, the
// validation of the src descriptor, for one item of its list.
func descriptorSrc(tokens []tinycss2.Node, baseURL string) (Value, error) {
	if len(tokens) != 1 && len(tokens) != 2 {
		return nil, nil
	}
	tokens, token := tokens[:len(tokens)-1], tokens[len(tokens)-1]
	if f, ok := token.(*tinycss2.FunctionBlock); ok && f.LowerName == "format" {
		if len(tokens) == 0 {
			// Port: upstream raises IndexError for format() alone.
			return nil, nil
		}
		token = tokens[len(tokens)-1]
	}
	if f, ok := token.(*tinycss2.FunctionBlock); ok && f.LowerName == "local" {
		return Tuple{String("local"), descriptorFontFamily(f.Arguments, true)}, nil
	}
	u, err := getURL(token, baseURL)
	if err != nil {
		return nil, err
	}
	if t, ok := u.(Tuple); ok && isStr(t[0], "url") {
		return t[1], nil
	}
	return nil, nil
}

// descriptorFontStyle is font_style in
// weasyprint/css/validation/descriptors.py.
func descriptorFontStyle(keyword string) bool {
	return in(keyword, "normal", "italic", "oblique")
}

// descriptorFontWeight is font_weight in
// weasyprint/css/validation/descriptors.py.
func descriptorFontWeight(token tinycss2.Node, _ string) (Value, error) {
	keyword := getKeyword(token)
	if keyword == "normal" || keyword == "bold" {
		return String(keyword), nil
	}
	if number, ok := getNumber(token, true, true).(Dimension); ok {
		switch number.Value {
		case 100, 200, 300, 400, 500, 600, 700, 800, 900:
			return Int(number.Value), nil
		}
	}
	return nil, nil
}

// descriptorFontStretch is font_stretch in
// weasyprint/css/validation/descriptors.py.
func descriptorFontStretch(keyword string) bool {
	return in(keyword,
		"ultra-condensed", "extra-condensed", "condensed", "semi-condensed",
		"normal",
		"semi-expanded", "expanded", "extra-expanded", "ultra-expanded")
}

// descriptorFontFeatureSettings is font_feature_settings in
// weasyprint/css/validation/descriptors.py.
func descriptorFontFeatureSettings(tokens []tinycss2.Node, baseURL string) (Value, error) {
	return validateFontFeatureSettings(tokens, baseURL)
}

// descriptorFontVariant is font_variant in
// weasyprint/css/validation/descriptors.py. It returns a list of tuples,
// each with one pair (name, value) of a font-variant-* property.
func descriptorFontVariant(tokens []tinycss2.Node, baseURL string) (Value, error) {
	if len(tokens) == 1 {
		if keyword := getKeyword(tokens[0]); in(keyword, "normal", "none", "inherit") {
			return Tuple{}, nil
		}
	}
	expanded, err := expandFontVariant(tokens)
	if err != nil {
		return nil, err
	}
	values := Tuple{}
	for _, pair := range expanded {
		validated, err := validateNonShorthand(pair.tokens, "font-variant"+pair.name, baseURL, true)
		if err != nil {
			// Upstream returns None, which is an invalid value.
			return nil, errInvalid
		}
		// Upstream appends the result of validate_non_shorthand, a tuple of
		// one pair.
		values = append(values, Tuple{Tuple{String(validated.name), validated.value}})
	}
	return values, nil
}

// descriptorUnicodeRange is unicode_range in
// weasyprint/css/validation/descriptors.py.
func descriptorUnicodeRange(token tinycss2.Node, _ string) (Value, error) {
	if token.Type() == tinycss2.TypeUnicodeRange {
		return Token{token}, nil
	}
	return nil, nil
}

// descriptorSystem is system in weasyprint/css/validation/descriptors.py.
// It returns the tuple (extends, system, fixed_number).
func descriptorSystem(tokens []tinycss2.Node, _ string) (Value, error) {
	if len(tokens) > 2 || len(tokens) == 0 {
		// Port: upstream raises IndexError for no token.
		return nil, nil
	}

	keyword := getKeyword(tokens[0])
	switch {
	case keyword == "extends":
		if len(tokens) == 2 {
			if secondKeyword := getKeyword(tokens[1]); secondKeyword != "" {
				return Tuple{String(keyword), String(secondKeyword), nil}, nil
			}
		}
	case keyword == "fixed":
		if len(tokens) == 1 {
			return Tuple{nil, String("fixed"), Int(1)}, nil
		}
		if number, ok := getNumber(tokens[1], true, true).(Dimension); ok {
			return Tuple{nil, String("fixed"), Int(number.Value)}, nil
		}
	case len(tokens) == 1 && in(keyword, "cyclic", "numeric", "alphabetic", "symbolic", "additive"):
		return Tuple{nil, String(keyword), nil}, nil
	}
	return nil, nil
}

// symbolOf returns ('string', value) for a string or an ident token, ('url',
// url) for a url token, or nil.
func symbolOf(token tinycss2.Node, baseURL string) (Value, error) {
	switch t := token.(type) {
	case *tinycss2.StringToken:
		return Tuple{String("string"), String(t.Value)}, nil
	case *tinycss2.IdentToken:
		return Tuple{String("string"), String(t.Value)}, nil
	}
	u, err := getURL(token, baseURL)
	if err != nil {
		return nil, err
	}
	if t, ok := u.(Tuple); ok && isStr(t[0], "url") {
		return Tuple{String("url"), t[1]}, nil
	}
	return nil, nil
}

// descriptorNegative is negative in
// weasyprint/css/validation/descriptors.py.
func descriptorNegative(tokens []tinycss2.Node, baseURL string) (Value, error) {
	if len(tokens) > 2 {
		return nil, nil
	}
	values := Tuple{}
	for _, token := range tokens {
		value, err := symbolOf(token, baseURL)
		if err != nil {
			return nil, err
		}
		if value != nil {
			values = append(values, value)
		}
	}
	if len(values) == 1 {
		values = append(values, Tuple{String("string"), String("")})
	}
	if len(values) == 2 {
		return values, nil
	}
	return nil, nil
}

// descriptorPrefixSuffix is prefix_suffix in
// weasyprint/css/validation/descriptors.py, the validation of the prefix and
// the suffix descriptors.
func descriptorPrefixSuffix(tokens []tinycss2.Node, baseURL string) (Value, error) {
	if len(tokens) != 1 {
		return nil, nil
	}
	return symbolOf(tokens[0], baseURL)
}

// descriptorRange is range in weasyprint/css/validation/descriptors.py, for
// one item of its list.
func descriptorRange(tokens []tinycss2.Node, _ string) (Value, error) {
	switch len(tokens) {
	case 1:
		if getSingleKeyword(tokens) == "auto" {
			return String("auto"), nil
		}
	case 2:
		var values Tuple
		for i, token := range tokens {
			if ident, ok := token.(*tinycss2.IdentToken); ok && ident.Value == "infinite" {
				if i > 0 {
					values = append(values, Float(math.Inf(1)))
				} else {
					values = append(values, Float(math.Inf(-1)))
				}
			} else if number, ok := getNumber(token, true, true).(Dimension); ok {
				values = append(values, Int(number.Value))
			}
		}
		if len(values) == 2 && num(values[0]) <= num(values[1]) {
			return values, nil
		}
	}
	return nil, nil
}

// descriptorPad is pad in weasyprint/css/validation/descriptors.py. It
// returns the tuple (count, symbol).
func descriptorPad(tokens []tinycss2.Node, baseURL string) (Value, error) {
	if len(tokens) != 2 {
		return nil, nil
	}
	values := Tuple{nil, nil}
	for _, token := range tokens {
		if number, ok := getNumber(token, false, true).(Dimension); ok {
			if values[0] == nil {
				values[0] = Int(number.Value)
			}
		} else if value, ok := tokenValue(token); ok && (token.Type() == tinycss2.TypeString || token.Type() == tinycss2.TypeIdent) {
			values[1] = Tuple{String("string"), String(value)}
		}
		u, err := getURL(token, baseURL)
		if err != nil {
			return nil, err
		}
		if t, ok := u.(Tuple); ok && isStr(t[0], "url") {
			values[1] = Tuple{String("url"), t[1]}
		}
	}
	if values[0] != nil && values[1] != nil {
		return values, nil
	}
	return nil, nil
}

// descriptorFallback is fallback in
// weasyprint/css/validation/descriptors.py.
func descriptorFallback(token tinycss2.Node, _ string) (Value, error) {
	ident := getCustomIdent(token)
	if ident != "none" && ident != "" {
		return String(ident), nil
	}
	return nil, nil
}

// descriptorSymbols is symbols in weasyprint/css/validation/descriptors.py.
func descriptorSymbols(tokens []tinycss2.Node, baseURL string) (Value, error) {
	values := Tuple{}
	for _, token := range tokens {
		value, err := symbolOf(token, baseURL)
		if err != nil {
			return nil, err
		}
		if value == nil {
			return nil, nil
		}
		values = append(values, value)
	}
	return values, nil
}

// descriptorAdditiveSymbols is additive_symbols in
// weasyprint/css/validation/descriptors.py.
func descriptorAdditiveSymbols(tokens []tinycss2.Node, baseURL string) (Value, error) {
	results := Tuple{}
	for _, part := range splitOnComma(tokens) {
		result, err := descriptorPad(removeWhitespace(part), baseURL)
		if err != nil {
			return nil, err
		}
		pair, ok := result.(Tuple)
		if !ok {
			return nil, nil
		}
		if len(results) > 0 && num(results[len(results)-1].(Tuple)[0]) <= num(pair[0]) {
			return nil, nil
		}
		results = append(results, pair)
	}
	return results, nil
}

// descriptorColorProfileSrc is color_profile_src in
// weasyprint/css/validation/descriptors.py.
func descriptorColorProfileSrc(token tinycss2.Node, baseURL string) (Value, error) {
	u, err := getURL(token, baseURL)
	if err != nil {
		return nil, err
	}
	if t, ok := u.(Tuple); ok && isStr(t[0], "url") {
		return t[1], nil
	}
	return nil, nil
}

// descriptorRenderingIntent is rendering_intent in
// weasyprint/css/validation/descriptors.py.
func descriptorRenderingIntent(keyword string) bool {
	return in(keyword, "relative-colorimetric", "absolute-colorimetric", "perceptual", "saturation")
}

// descriptorComponents is components in
// weasyprint/css/validation/descriptors.py, for one item of its list.
func descriptorComponents(tokens []tinycss2.Node, _ string) (Value, error) {
	components := Tuple{}
	for _, token := range tokens {
		ident, ok := token.(*tinycss2.IdentToken)
		if !ok {
			return nil, nil
		}
		components = append(components, String(ident.Value))
	}
	return components, nil
}
