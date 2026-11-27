package docx

import "strings"

// symbolChars maps the characters of the font Symbol to Unicode.
var symbolChars = map[rune]rune{
	'A': 'Α', 'B': 'Β', 'C': 'Χ', 'D': 'Δ', 'E': 'Ε', 'F': 'Φ', 'G': 'Γ', 'H': 'Η',
	'I': 'Ι', 'J': 'ϑ', 'K': 'Κ', 'L': 'Λ', 'M': 'Μ', 'N': 'Ν', 'O': 'Ο', 'P': 'Π',
	'Q': 'Θ', 'R': 'Ρ', 'S': 'Σ', 'T': 'Τ', 'U': 'Υ', 'V': 'ς', 'W': 'Ω', 'X': 'Ξ',
	'Y': 'Ψ', 'Z': 'Ζ',
	'a': 'α', 'b': 'β', 'c': 'χ', 'd': 'δ', 'e': 'ε', 'f': 'φ', 'g': 'γ', 'h': 'η',
	'i': 'ι', 'j': 'ϕ', 'k': 'κ', 'l': 'λ', 'm': 'μ', 'n': 'ν', 'o': 'ο', 'p': 'π',
	'q': 'θ', 'r': 'ρ', 's': 'σ', 't': 'τ', 'u': 'υ', 'v': 'ϖ', 'w': 'ω', 'x': 'ξ',
	'y': 'ψ', 'z': 'ζ',
	0xa3: '≤', 0xa5: '∞', 0xa7: '♣', 0xa8: '♦', 0xa9: '♥', 0xaa: '♠', 0xab: '↔',
	0xac: '←', 0xad: '↑', 0xae: '→', 0xaf: '↓', 0xb0: '°', 0xb1: '±', 0xb3: '≥',
	0xb4: '×', 0xb5: '∝', 0xb6: '∂', 0xb7: '•', 0xb8: '÷', 0xb9: '≠', 0xba: '≡',
	0xbb: '≈', 0xbc: '…', 0xc5: '⊕', 0xc6: '∅', 0xc7: '∩', 0xc8: '∪', 0xd0: '∠',
	0xd1: '∇', 0xd2: '®', 0xd3: '©', 0xd4: '™', 0xd5: '∏', 0xd6: '√', 0xd7: '⋅',
	0xd8: '¬', 0xd9: '∧', 0xda: '∨', 0xdb: '⇔', 0xdc: '⇐', 0xdd: '⇑', 0xde: '⇒',
	0xdf: '⇓', 0xe0: '◊', 0xe5: '∑', 0xf2: '∫',
}

// wingdingsChars maps the characters of the font Wingdings that lists and
// forms use to Unicode.
var wingdingsChars = map[rune]rune{
	'l': '●', 'n': '■', 'o': '□', 'q': '❑', 'r': '❒', 's': '⬧', 't': '⧫',
	'u': '◆', 'v': '❖', 'w': '⬥', 'x': '⌧', 0x9f: '•', 0xa1: '○', 0xa7: '▪',
	0xa8: '◻', 0xd8: '➢', 0xe0: '➝', 0xe8: '➔', 0xef: '⇦', 0xf0: '⇨', 0xfb: '✗',
	0xfc: '✓', 0xfd: '☒', 0xfe: '☑',
}

// symbolRune returns the Unicode character of the character r of a font,
// for the fonts of symbols. A symbol font maps its characters to the
// private area from U+F020 to U+F0FF, or to the range of ASCII and Latin-1.
// A character of the font Symbol that the font does not have is
// notdefRune, which escapeSymbols writes as the empty box of the font.
func symbolRune(font string, r rune) rune {
	if r == notdefRune {
		// A noncharacter draws nothing.
		return 0
	}
	lower := strings.ToLower(font)
	isSymbol := lower == "symbol"
	isWingdings := strings.HasPrefix(lower, "wingdings")
	c := r
	switch {
	case r >= 0xf020 && r <= 0xf0ff:
		c = r - 0xf000
	case !isSymbol && !isWingdings:
		return r
	}
	switch {
	case isSymbol:
		if u, ok := symbolChars[c]; ok {
			return u
		}
		if c >= 0x7f && c <= 0xa0 || c == 0xf0 || c == 0xff {
			return notdefRune
		}
	case isWingdings:
		if u, ok := wingdingsChars[c]; ok {
			return u
		}
		return '•'
	case r >= 0xf020 && r <= 0xf0ff:
		// A character of the private area in a font that is not known: a
		// bullet of a list, most often.
		if u, ok := symbolChars[c]; ok && c >= 0x80 {
			return u
		}
	}
	return c
}

// notdefRune stands for the empty box that Word draws for a character that
// the font Symbol does not have, from 0x7F to 0xA0, 0xF0 and 0xFF. It is a
// noncharacter, which no text of a document holds.
const notdefRune = '\uffff'

// notdefBox is the HTML of the empty box of notdefRune: the glyph .notdef
// of Symbol, 0.5 em wide and 0.625 em high, in an advance of 0.6 em. Its
// lines are 1/64 em wide in the font. The box has lines of 0.05 em, which
// a page of 96 dots per inch still shows. Word gives the line of the box
// the ascent and the descent of Symbol, 1.005 em and 0.22 em, so the
// margins of the box take the line as high, as on page 1 of
// poi/docx/form-footnotes.docx.
const notdefBox = `<span style="display: inline-block; width: 0.4em; height: 0.525em; ` +
	`border: 0.05em solid; margin: 0.38em 0.05em 0.22em; vertical-align: -0.22em"></span>`

// escapeSymbols writes the text s as escape does, with notdefBox for each
// notdefRune.
func escapeSymbols(b *strings.Builder, s string) {
	for {
		before, after, found := strings.Cut(s, string(notdefRune))
		escape(b, before, false)
		if !found {
			return
		}
		b.WriteString(notdefBox)
		s = after
	}
}

// symbolCodes returns the text s of a symbol font with each character in the
// private area from U+F020 to U+F0FF, where the character map of a symbol
// font puts it. It returns false if a character is outside that area and
// outside the range from U+0020 to U+00FF.
func symbolCodes(s string) (string, bool) {
	var b strings.Builder
	for _, r := range s {
		switch {
		case r >= 0x20 && r <= 0xff:
			b.WriteRune(r + 0xf000)
		case r >= 0xf020 && r <= 0xf0ff:
			b.WriteRune(r)
		default:
			return "", false
		}
	}
	return b.String(), b.Len() > 0
}

// isSymbolFont reports whether the font maps its characters to symbols
// instead of Unicode, such as Symbol and Wingdings.
func isSymbolFont(font string) bool {
	lower := strings.ToLower(font)
	return lower == "symbol" || strings.HasPrefix(lower, "wingdings") || strings.HasPrefix(lower, "webdings")
}
