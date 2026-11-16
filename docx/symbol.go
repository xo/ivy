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
func symbolRune(font string, r rune) rune {
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

// isSymbolFont reports whether the font maps its characters to symbols
// instead of Unicode, such as Symbol and Wingdings.
func isSymbolFont(font string) bool {
	lower := strings.ToLower(font)
	return lower == "symbol" || strings.HasPrefix(lower, "wingdings") || strings.HasPrefix(lower, "webdings")
}
