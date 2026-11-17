// This file is not a port. It adds -ivy-text-autospace, a subset of
// text-autospace of CSS Text 4, which upstream does not have: a space
// between ideographs and Latin letters or digits, as the word processors
// add with w:autoSpaceDE and w:autoSpaceDN of Word.

package layout

import (
	"unicode"

	"github.com/xo/okage/tinycss2"
)

// autospaceRatio is the space of -ivy-text-autospace, in em: an eighth of
// an ideograph, as CSS Text 4 says.
const autospaceRatio = 0.125

// validateTextAutospace validates -ivy-text-autospace: 'none', or one or
// both of 'ideograph-alpha' and 'ideograph-numeric'.
func validateTextAutospace(tokens []tinycss2.Node, _ string) (Value, error) {
	if getSingleKeyword(tokens) == "none" {
		return String("none"), nil
	}
	if len(tokens) == 0 || len(tokens) > 2 {
		return nil, nil
	}
	var set []string
	for _, token := range tokens {
		keyword := getKeyword(token)
		if !in(keyword, "ideograph-alpha", "ideograph-numeric") || in(keyword, set...) {
			return nil, nil
		}
		set = append(set, keyword)
	}
	return newStringSet(set...), nil
}

// autospaceClass is the class of a character for -ivy-text-autospace.
type autospaceClass int

// The classes of the characters.
const (
	autospaceOther autospaceClass = iota
	autospaceIdeograph
	autospaceAlpha
	autospaceNumeric
)

// classOf returns the class of r: an ideograph of Han, Hiragana, Katakana
// or Bopomofo, a letter of another script that is not full width, a digit
// that is not full width, or another character.
func classOf(r rune) autospaceClass {
	switch {
	case unicode.In(r, unicode.Han, unicode.Hiragana, unicode.Katakana, unicode.Bopomofo):
		return autospaceIdeograph
	case r >= 0xff00 && r <= 0xffef:
		// the full width and the half width forms
		return autospaceOther
	case unicode.IsDigit(r):
		return autospaceNumeric
	case unicode.IsLetter(r) && !unicode.Is(unicode.Hangul, r):
		return autospaceAlpha
	}
	return autospaceOther
}

// autospaceAfter reports whether the space of -ivy-text-autospace comes
// after the cluster i of the layout: between an ideograph and a letter, or
// a digit, in either order.
func (l *TextLayout) autospaceAfter(i int) bool {
	if l.autospace == nil {
		l.autospace = []bool{}
		set, ok := l.Style.Get("text_autospace").(StringSet)
		alpha, numeric := ok && set.Has("ideograph-alpha"), ok && set.Has("ideograph-numeric")
		if alpha || numeric {
			clusters := l.shapedText().clusters
			l.autospace = make([]bool, len(clusters))
			for j := 0; j+1 < len(clusters); j++ {
				a, b := classOf(clusters[j].r), classOf(clusters[j+1].r)
				if a == autospaceIdeograph {
					a, b = b, a
				}
				l.autospace[j] = b == autospaceIdeograph &&
					(alpha && a == autospaceAlpha || numeric && a == autospaceNumeric)
			}
		}
	}
	return i < len(l.autospace) && l.autospace[i]
}
