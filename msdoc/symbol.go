package msdoc

import "strings"

// symbolChars maps the codes of the font Symbol to Unicode, as in the
// package docx.
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

// notdefRune stands for the empty box that Word draws for a code that the
// font Symbol does not have, from 0x7F to 0xA0, 0xF0 and 0xFF. It is a
// noncharacter, which visible drops from the text of a document.
const notdefRune = '￿'

// notdefBox is the HTML of the empty box of notdefRune, as in the package
// docx: the glyph .notdef of Symbol, 0.5 em wide and 0.625 em high, in an
// advance of 0.6 em. The box has lines of 0.05 em, which a page of 96 dots
// per inch still shows. Its margins give the line the ascent and the
// descent of Symbol, 1.005 em and 0.22 em, as Word does.
const notdefBox = `<span style="display: inline-block; width: 0.4em; height: 0.525em; ` +
	`border: 0.05em solid; margin: 0.38em 0.05em 0.22em; vertical-align: -0.22em"></span>`

// symbolFontRune returns the Unicode character of the code c, from 0x20 to
// 0xFF, of the font Symbol, or notdefRune for a code that the font does not
// have. Another code stays.
func symbolFontRune(c rune) rune {
	if u, ok := symbolChars[c]; ok {
		return u
	}
	if c >= 0x7f && c <= 0xa0 || c == 0xf0 || c == 0xff {
		return notdefRune
	}
	return c
}

// isSymbolFont reports whether the font ftc is the font Symbol.
func (p *parser) isSymbolFont(ftc int) bool {
	return ftc >= 0 && ftc < len(p.fonts) && strings.EqualFold(p.fonts[ftc].name, "Symbol")
}

// symbolText returns the text s of a run of the font ftc. When the font is
// Symbol, and the program does not draw the characters of s in the private
// area from U+F020 to U+F0FF with it, each of them becomes its Unicode
// character, or notdefRune. Word puts the codes of a symbol font in that
// area.
func (p *parser) symbolText(ftc int, s string) string {
	if !p.isSymbolFont(ftc) {
		return s
	}
	var codes []uint16
	for _, r := range s {
		if isSymbolCode(r) {
			codes = append(codes, uint16(r))
		}
	}
	if len(codes) == 0 || p.drawsSymbols(ftc, codes) {
		return s
	}
	return strings.Map(func(r rune) rune {
		if isSymbolCode(r) {
			return symbolFontRune(r - 0xF000)
		}
		return r
	}, s)
}

// isSymbolCode reports whether r is a code of a symbol font in the private
// area, from U+F020 to U+F0FF.
func isSymbolCode(r rune) bool {
	return r >= 0xF020 && r <= 0xF0FF
}

// symbolChar returns the character of the code c of the font ftc, from
// sprmCSymbol. A code from 0x20 to 0xFF moves to the private area, where a
// symbol font puts it. The code of a font other than Symbol that the
// program does not draw becomes the character of symbolRune, as in a
// bullet. symbolText gives the codes of Symbol their characters.
func (p *parser) symbolChar(ftc int, c rune) rune {
	if c >= 0x20 && c <= 0xFF {
		c += 0xF000
	}
	if !isSymbolCode(c) || p.isSymbolFont(ftc) || p.drawsSymbols(ftc, []uint16{uint16(c)}) {
		return c
	}
	return symbolRune(c - 0xF000)
}
