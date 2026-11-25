package formula

import (
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/xo/okage/texmath"
)

// Atoms appends the atoms of the characters of s to l, as Office Math and
// MathML set the characters of a run. Each letter is an atom of its own, in
// the math alphabet v. A run of digits, with a decimal point between them, is
// one atom. Each other character is an atom of its class in TeX, such as a
// binary operator for +, so that texmath gives it the space of TeX. A
// hyphen-minus is the minus sign, and white space is left out, as the
// equations of Word and MathML do.
func Atoms(l texmath.List, s string, v texmath.Variant) texmath.List {
	for i := 0; i < len(s); {
		r, n := utf8.DecodeRuneInString(s[i:])
		switch {
		case unicode.IsSpace(r):
			i += n
		case r >= '0' && r <= '9':
			j := i + digits(s[i:])
			l = append(l, texmath.Atom{Class: texmath.Ord, Text: s[i:j], Variant: digitVariant(v)})
			i = j
		case unicode.IsLetter(r):
			l = append(l, letter(r, v))
			i += n
		default:
			l = append(l, symbol(r))
			i += n
		}
	}
	return l
}

// digits returns the length of the number at the start of s: its digits,
// with a point or a comma between two digits.
func digits(s string) int {
	n := 0
	for n < len(s) {
		c := s[n]
		switch {
		case c >= '0' && c <= '9':
			n++
		case (c == '.' || c == ',') && n+1 < len(s) && s[n+1] >= '0' && s[n+1] <= '9':
			n++
		default:
			return n
		}
	}
	return n
}

// digitVariant returns the alphabet of the digits of a run whose letters
// are in the alphabet v. A digit has no italic form, so it stays upright in
// an italic run.
func digitVariant(v texmath.Variant) texmath.Variant {
	switch v {
	case texmath.Italic:
		return texmath.Upright
	case texmath.BoldItalic:
		return texmath.Bold
	case texmath.SansSerifItalic:
		return texmath.SansSerif
	case texmath.SansSerifBoldItalic:
		return texmath.SansSerifBold
	}
	return v
}

// letter returns the atom of a letter in the math alphabet v.
func letter(r rune, v texmath.Variant) texmath.Atom {
	// texmath maps the Latin letters only. A small Greek letter in italic
	// takes the italic Greek of the Mathematical Alphanumeric Symbols, as
	// TeX sets it. A capital Greek letter stays upright, as in TeX.
	if v == texmath.Italic && r >= 'α' && r <= 'ω' {
		r = 0x1D6FC + (r - 'α')
	}
	return texmath.Atom{Class: texmath.Ord, Text: string(r), Variant: v}
}

// symbol returns the atom of a character that is not a letter, a digit or a
// space.
func symbol(r rune) texmath.Node {
	switch r {
	case '-':
		r = '−'
	case '*':
		r = '∗'
	case '\'':
		r = '′'
	}
	if strings.ContainsRune(largeOps, r) {
		return texmath.Atom{Class: texmath.Op, Text: string(r), Variant: texmath.Upright, Large: true, Limits: limitsOf(r)}
	}
	return texmath.Atom{Class: classOf(r), Text: string(r), Variant: texmath.Upright}
}

// The characters of each class of TeX that an equation holds. A character
// in no list is an ordinary symbol.
const (
	binOps   = "+−±∓×÷·∗∘∙∩∪⊕⊖⊗⊘⊙∧∨∖⋅⋆△▽⊎⊓⊔≀†‡⨿"
	relOps   = "=<>≤≥≠≈≡∼≃≅∝→←↔⇒⇐⇔⟶⟵⟷⟹⟸⟺↦⟼∈∉∋∌⊂⊃⊆⊇⊄⊅⊊⊋≪≫⊥∥∣≺≻⪯⪰≐≜≔:⊢⊣⊨≲≳≮≯≰≱↑↓⇑⇓↗↘↙↖⇀↼⇌⊏⊐⊑⊒∽≍≎≏"
	openOps  = "([{⟨⌈⌊⟦⟮"
	closeOps = ")]}⟩⌉⌋⟧⟯!?"
	puncts   = ",;"
	largeOps = "∑∏∐∫∬∭∮∯∰⋀⋁⋂⋃⨀⨁⨂⨄⨆"
)

// classOf returns the class of TeX of the character r.
func classOf(r rune) texmath.Class {
	switch {
	case strings.ContainsRune(binOps, r):
		return texmath.Bin
	case strings.ContainsRune(relOps, r):
		return texmath.Rel
	case strings.ContainsRune(openOps, r):
		return texmath.Open
	case strings.ContainsRune(closeOps, r):
		return texmath.Close
	case strings.ContainsRune(puncts, r):
		return texmath.Punct
	}
	return texmath.Ord
}

// limitsOf returns where the limits of the large operator r go: at the
// side for an integral, and over and under it in display style for the
// others, as in TeX.
func limitsOf(r rune) texmath.Limits {
	if strings.ContainsRune("∫∬∭∮∯∰", r) {
		return texmath.LimitsSide
	}
	return texmath.LimitsAuto
}
