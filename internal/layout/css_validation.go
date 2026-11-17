// Copyright (c) 2011-2021, Simon Sapin and contributors.
//
// Use of this source code is governed by the BSD-3-Clause license that can be
// found in the LICENSE file.

// This file ports weasyprint/css/validation/__init__.py: the validation of
// the properties, the expanders and the descriptors.

package layout

import (
	"errors"
	"strings"

	"github.com/xo/okage/cssselect2"
	"github.com/xo/okage/tinycss2"
)

// notPrintMedia is NOT_PRINT_MEDIA in
// weasyprint/css/validation/__init__.py: the properties that do not apply
// to the print media.
var notPrintMedia = map[string]bool{
	// Aural media
	"azimuth":           true,
	"cue":               true,
	"cue-after":         true,
	"cue-before":        true,
	"elevation":         true,
	"pause":             true,
	"pause-after":       true,
	"pause-before":      true,
	"pitch-range":       true,
	"pitch":             true,
	"play-during":       true,
	"richness":          true,
	"speak-header":      true,
	"speak-numeral":     true,
	"speak-punctuation": true,
	"speak":             true,
	"speech-rate":       true,
	"stress":            true,
	"voice-family":      true,
	"volume":            true,
	// Animations, transitions, timelines
	"animation":                  true,
	"animation-composition":      true,
	"animation-delay":            true,
	"animation-direction":        true,
	"animation-duration":         true,
	"animation-fill-mode":        true,
	"animation-iteration-count":  true,
	"animation-name":             true,
	"animation-play-state":       true,
	"animation-range":            true,
	"animation-range-end":        true,
	"animation-range-start":      true,
	"animation-timeline":         true,
	"animation-timing-function":  true,
	"timeline-scope":             true,
	"transition":                 true,
	"transition-delay":           true,
	"transition-duration":        true,
	"transition-property":        true,
	"transition-timing-function": true,
	"view-timeline":              true,
	"view-timeline-axis":         true,
	"view-timeline-inset":        true,
	"view-timeline-name":         true,
	"view-transition-name":       true,
	"will-change":                true,
	// Dynamic and interactive
	"caret":          true,
	"caret-color":    true,
	"caret-shape":    true,
	"cursor":         true,
	"field-sizing":   true,
	"pointer-events": true,
	"resize":         true,
	"touch-action":   true,
	// Browser viewport scrolling
	"overscroll-behavior":         true,
	"overscroll-behavior-block":   true,
	"overscroll-behavior-inline":  true,
	"overscroll-behavior-x":       true,
	"overscroll-behavior-y":       true,
	"scroll-behavior":             true,
	"scroll-margin":               true,
	"scroll-margin-block":         true,
	"scroll-margin-block-end":     true,
	"scroll-margin-block-start":   true,
	"scroll-margin-bottom":        true,
	"scroll-margin-inline":        true,
	"scroll-margin-inline-end":    true,
	"scroll-margin-inline-start":  true,
	"scroll-margin-left":          true,
	"scroll-margin-right":         true,
	"scroll-margin-top":           true,
	"scroll-padding":              true,
	"scroll-padding-block":        true,
	"scroll-padding-block-end":    true,
	"scroll-padding-block-start":  true,
	"scroll-padding-bottom":       true,
	"scroll-padding-inline":       true,
	"scroll-padding-inline-end":   true,
	"scroll-padding-inline-start": true,
	"scroll-padding-left":         true,
	"scroll-padding-right":        true,
	"scroll-padding-top":          true,
	"scroll-snap-align":           true,
	"scroll-snap-stop":            true,
	"scroll-snap-type":            true,
	"scroll-timeline":             true,
	"scroll-timeline-axis":        true,
	"scroll-timeline-name":        true,
	"scrollbar-color":             true,
	"scrollbar-gutter":            true,
	"scrollbar-width":             true,
}

// Declaration is a declaration of a property that the validation keeps: the
// tuple (name, value, important) of upstream. The name is in snake case,
// such as "margin_top", and the name of a custom property starts with "__".
type Declaration struct {
	Name      string
	Value     Value
	Important bool
}

// nameValue is a pair (name, value) that a validator or an expander
// returns. The name is in kebab case, such as "margin-top".
type nameValue struct {
	name  string
	value Value
}

// expanderFunc is the type of the expanders of
// weasyprint/css/validation/expanders.py, and of validate_non_shorthand. It
// returns the expanded pairs, or an error for an invalid value.
type expanderFunc func(tokens []tinycss2.Node, name, baseURL string) ([]nameValue, error)

// selectorDeclaration is a pair (selectors, declaration) that
// preprocess_declarations yields for the declarations of a rule. group
// tells the rules apart: two pairs of the same rule have the same group, as
// they share the list of selectors in upstream.
type selectorDeclaration struct {
	selectors   []*cssselect2.CompiledSelector
	group       int
	declaration Declaration
}

// rootTokens returns ROOT_TOKEN of weasyprint/css/validation/__init__.py,
// the selector :root. The nesting selector NESTING_SELECTOR is the literal
// "&".
func rootTokens() []tinycss2.Node {
	return []tinycss2.Node{tinycss2.NewLiteralToken(1, 1, ":"), tinycss2.NewIdentToken(1, 1, "root")}
}

// preprocessDeclarations is preprocess_declarations in
// weasyprint/css/validation/__init__.py without a prelude. It expands the
// shorthand properties, and filters the unsupported properties and values.
// It logs a warning for each ignored declaration.
func preprocessDeclarations(log *logger, baseURL string, declarations []tinycss2.Node) []Declaration {
	var result []Declaration
	// Without a prelude, no error can come.
	_ = preprocessDeclarationsTo(log, baseURL, declarations, nil, nil, func(_ []*cssselect2.CompiledSelector, _ int, d Declaration) {
		result = append(result, d)
	})
	return result
}

// preprocessRuleDeclarations is preprocess_declarations in
// weasyprint/css/validation/__init__.py with the prelude of a rule. It
// returns the declarations with their selectors, also those of the nested
// rules, or a *cssselect2.SelectorError for a selector that is not valid.
func preprocessRuleDeclarations(log *logger, baseURL string, declarations, prelude []tinycss2.Node, budget *cascadeBudget) ([]selectorDeclaration, error) {
	var result []selectorDeclaration
	err := preprocessDeclarationsTo(log, baseURL, declarations, prelude, budget, func(selectors []*cssselect2.CompiledSelector, group int, d Declaration) {
		result = append(result, selectorDeclaration{selectors, group, d})
	})
	return result, err
}

// preprocessDeclarationsTo is preprocess_declarations in
// weasyprint/css/validation/__init__.py. It calls yield for each
// declaration. A nil prelude is the None of upstream.
func preprocessDeclarationsTo(log *logger, baseURL string, declarations, prelude []tinycss2.Node, budget *cascadeBudget, yield func([]*cssselect2.CompiledSelector, int, Declaration)) error {
	// Compile list of selectors.
	var selectors []*cssselect2.CompiledSelector
	group := 0
	if prelude != nil {
		if containsToken(prelude, "&") {
			// Handle & selector in non-nested rule. MDN explains that & is
			// then equivalent to :scope, and :scope is equivalent to :root as
			// we do not support :scope yet.
			original := prelude
			prelude = nil
			for _, token := range original {
				if tinycss2.IsLiteral(token, "&") {
					prelude = append(prelude, rootTokens()...)
				} else {
					prelude = append(prelude, token)
				}
			}
		}
		var err error
		selectors, err = cssselect2.CompileSelectorList(prelude, nil)
		if err != nil {
			return &cssselect2.SelectorError{Message: "'" + tinycss2.Serialize(prelude) + "'"}
		}
		group = budget.nextGroup()
	}

	// Yield declarations.
	for _, declaration := range declarations {
		if e, ok := declaration.(*tinycss2.ParseError); ok {
			log.warning("Error: %s at %d:%d.", e.Message, e.SourceLine, e.SourceColumn)
		}

		if rule, ok := declaration.(*tinycss2.QualifiedRule); ok {
			// Nested rule.
			if prelude == nil {
				continue
			}
			if !budget.nest() {
				// Port: the nesting of rules is bounded.
				log.warning("Ignored nested rule at %d:%d, too deep.", rule.SourceLine, rule.SourceColumn)
				continue
			}
			var declarationPrelude []tinycss2.Node
			isToken := func() []tinycss2.Node {
				return []tinycss2.Node{
					tinycss2.NewLiteralToken(1, 1, ":"),
					tinycss2.NewFunctionBlock(1, 1, "is", prelude),
				}
			}
			for _, tokenGroup := range splitOnComma(rule.Prelude) {
				if containsToken(tokenGroup, "&") {
					// Replace & selector by parent.
					for _, token := range rule.Prelude {
						if tinycss2.IsLiteral(token, "&") {
							declarationPrelude = append(declarationPrelude, isToken()...)
						} else {
							declarationPrelude = append(declarationPrelude, token)
						}
					}
				} else {
					// No & selector, prepend parent.
					declarationPrelude = append(declarationPrelude, isToken()...)
					declarationPrelude = append(declarationPrelude, tinycss2.NewWhitespaceToken(1, 1, " "))
					declarationPrelude = append(declarationPrelude, tokenGroup...)
				}
				declarationPrelude = append(declarationPrelude, tinycss2.NewLiteralToken(1, 1, ","))
			}
			err := preprocessDeclarationsTo(
				log, baseURL, tinycss2.ParseBlocksContents(rule.Content),
				declarationPrelude[:len(declarationPrelude)-1], budget, yield)
			budget.unnest()
			if err != nil {
				return err
			}
		}

		decl, ok := declaration.(*tinycss2.Declaration)
		if !ok {
			continue
		}

		name := decl.Name
		if !strings.HasPrefix(name, "--") {
			name = decl.LowerName
		}

		validationError := func(debug bool, reason string) {
			format := "Ignored `%s:%s` at %d:%d, %s."
			args := []any{decl.Name, tinycss2.Serialize(decl.Value), decl.SourceLine, decl.SourceColumn, reason}
			if debug {
				log.debug(format, args...)
			} else {
				log.warning(format, args...)
			}
		}

		if notPrintMedia[name] {
			validationError(true, "the property does not apply for the print media")
			continue
		}

		// Port: the properties that ivy adds are only known with the
		// prefix -ivy-.
		if unprefixedName, ok := strings.CutPrefix(name, ivyPrefix); ok && ivyProperties[unprefixedName] {
			name = unprefixedName
		} else if ivyProperties[name] {
			validationError(false, "the property is only known with the prefix "+ivyPrefix)
			continue
		}

		if unprefixedName, ok := strings.CutPrefix(name, prefix); ok {
			switch {
			case proprietary[unprefixedName]:
				name = unprefixedName
			case unstable[unprefixedName]:
				log.warning(
					"Deprecated `%s:%s` at %d:%d, prefixes on unstable attributes are deprecated, use %q instead.",
					decl.Name, tinycss2.Serialize(decl.Value), decl.SourceLine, decl.SourceColumn, unprefixedName)
				name = unprefixedName
			default:
				log.warning(
					"Ignored `%s:%s` at %d:%d, prefix on this attribute is not supported, use %q instead.",
					decl.Name, tinycss2.Serialize(decl.Value), decl.SourceLine, decl.SourceColumn, unprefixedName)
				continue
			}
		}

		if strings.HasPrefix(name, "-") && !strings.HasPrefix(name, "--") {
			validationError(true, "prefixed selectors are ignored")
			continue
		}

		validator, ok := expanders[name]
		if !ok {
			validator = validateNonShorthandExpander
		}
		tokens := removeWhitespace(decl.Value)
		var result []nameValue
		var err error
		if len(tokens) == 0 {
			// Having no tokens is allowed by grammar but refused by all
			// properties and expanders.
			err = invalidValues("no value")
		} else {
			result, err = validator(tokens, name, baseURL)
		}
		if err != nil {
			reason := "invalid value"
			var iv *InvalidValuesError
			if errors.As(err, &iv) && iv.Message != "" {
				reason = iv.Message
			}
			validationError(false, reason)
			continue
		}

		for _, pair := range result {
			d := Declaration{Name: strings.ReplaceAll(pair.name, "-", "_"), Value: pair.value, Important: decl.Important}
			yield(selectors, group, d)
		}
	}
	return nil
}

// containsToken reports whether a list of tokens holds a literal token,
// as the expression `token in tokens` of upstream does.
func containsToken(tokens []tinycss2.Node, literal string) bool {
	for _, token := range tokens {
		if tinycss2.IsLiteral(token, literal) {
			return true
		}
	}
	return false
}
