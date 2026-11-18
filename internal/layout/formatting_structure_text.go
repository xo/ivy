// Copyright (c) 2011-2021, Simon Sapin and contributors.
//
// Use of this source code is governed by the BSD-3-Clause license that can be
// found in the LICENSE file.

// This file ports weasyprint/formatting_structure/text.py: the functions of
// the transformations of text of CSS.
//
// The rules are defined in
// https://www.unicode.org/versions/latest/core-spec/chapter-3/#G33992,
// https://www.unicode.org/Public/UCD/latest/ucd/SpecialCasing.txt and
// https://w3c.github.io/i18n-tests/results/text-transform.
//
// Upstream handles the common transformations with the algorithms of Python,
// and the special casing and the tailoring here, when they depend on the
// language and not only on the glyphs.
//
// Port: the methods upper and lower of Python apply the special casing of
// Unicode, which the package unicode of Go does not. specialUpper and
// specialLower hold the mappings of Python 3.14 (Unicode 16) that give more
// than one character, and lowercase handles the final sigma as Python does.
// unicodedata.normalize has no port: greekWithoutDiacritics holds the Greek
// letters with the diacritics that uppercase removes, removed.

package layout

import (
	"strings"
	"unicode"
	"unicode/utf8"
)

// The functions below replace the regular expressions of
// weasyprint/formatting_structure/text.py.
//
// Port: upstream calls re.sub with LINE_FEED_RE, TAB_RE and SPACE_RE for
// each text box. The functions give the same texts without a regular
// expression, and return the text itself when nothing changes.

// isBlank reports whether c is a space or a tab, [\t ] of the regular
// expressions.
func isBlank(c byte) bool {
	return c == ' ' || c == '\t'
}

// normalizeLineFeeds is LINE_FEED_RE.sub('\n', text): it replaces "\r\n"
// and "\r" with "\n".
func normalizeLineFeeds(text string) string {
	if strings.IndexByte(text, '\r') < 0 {
		return text
	}
	var b strings.Builder
	b.Grow(len(text))
	for i := 0; i < len(text); i++ {
		if c := text[i]; c != '\r' {
			b.WriteByte(c)
			continue
		}
		b.WriteByte('\n')
		if i+1 < len(text) && text[i+1] == '\n' {
			i++
		}
	}
	return b.String()
}

// trimLineFeedBlanks is TAB_RE.sub('\n', text): it removes the spaces and
// the tabs before and after each line feed.
func trimLineFeedBlanks(text string) string {
	if strings.IndexByte(text, '\n') < 0 {
		return text
	}
	var b strings.Builder
	changed := false
	for i := 0; i < len(text); {
		if !isBlank(text[i]) {
			if changed {
				b.WriteByte(text[i])
			}
			i++
			continue
		}
		j := i + 1
		for j < len(text) && isBlank(text[j]) {
			j++
		}
		switch {
		case i > 0 && text[i-1] == '\n' || j < len(text) && text[j] == '\n':
			if !changed {
				changed = true
				b.Grow(len(text))
				b.WriteString(text[:i])
			}
		case changed:
			b.WriteString(text[i:j])
		}
		i = j
	}
	if !changed {
		return text
	}
	return b.String()
}

// collapseSpaces is SPACE_RE.sub(' ', text): it replaces each run of spaces
// and tabs with one space. With lineFeeds, a run also has the line feeds.
//
// collapseSpaces(text, true) is the text of TAB_RE.sub('\n', text), then of
// the line feeds replaced by spaces, then of SPACE_RE.sub(' ', text): each
// space or tab next to a line feed goes, each line feed becomes a space,
// and the spaces of a run become one space.
func collapseSpaces(text string, lineFeeds bool) string {
	var b strings.Builder
	changed := false
	isSpace := func(c byte) bool {
		return isBlank(c) || lineFeeds && c == '\n'
	}
	for i := 0; i < len(text); {
		if !isSpace(text[i]) {
			if changed {
				b.WriteByte(text[i])
			}
			i++
			continue
		}
		j := i + 1
		for j < len(text) && isSpace(text[j]) {
			j++
		}
		if !changed && (j-i > 1 || text[i] != ' ') {
			changed = true
			b.Grow(len(text))
			b.WriteString(text[:i])
		}
		if changed {
			b.WriteByte(' ')
		}
		i = j
	}
	if !changed {
		return text
	}
	return b.String()
}

// transform is transform in weasyprint/formatting_structure/text.py. It
// applies a value of text-transform to a text.
func transform(function, text, langCode string) string {
	switch function {
	case "uppercase":
		return uppercase(text, langCode)
	case "lowercase":
		return lowercase(text, langCode)
	case "capitalize":
		return capitalize(text, langCode)
	case "full-width":
		return fullWidth(text, langCode)
	}
	// Port: upstream raises KeyError.
	return text
}

// uppercase is uppercase in weasyprint/formatting_structure/text.py.
func uppercase(text, langCode string) string {
	var mapper [][2]string

	switch langCode {
	case "el":
		// https://w3c.github.io/i18n-tests/css-text/text-transform/
		//   text-transform-tailoring-003.html
		// https://en.wikiversity.org/wiki/Greek_Language/Diphthongs
		mapper = [][2]string{
			{"άι", "ΑΪ"},
			{"άυ", "ΑΫ"},
			{"όι", "ΟΪ"},
			{"όυ", "ΟΫ"},
			{"έυ", "ΗΫ"},
		}
	case "tr", "az":
		// https://github.com/unicode-org/cldr/blob/main/common/transforms/tr-Upper.xml
		mapper = [][2]string{{"i", "İ"}}
	}

	for _, pair := range mapper {
		text = strings.ReplaceAll(text, pair[0], pair[1])
	}

	if langCode == "el" {
		// Remove diacritics in Greek.
		// https://github.com/unicode-org/cldr/blob/main/common/transforms/el-Upper.xml
		// TODO: we should keep tonos on disjunctive eta.
		// https://w3c.github.io/i18n-tests/css-text/text-transform/
		//   text-transform-tailoring-005.html
		var b strings.Builder
		for _, r := range text {
			if s, ok := greekWithoutDiacritics[r]; ok {
				b.WriteString(s)
			} else if !strings.ContainsRune("̓̔́̀̆͂̄ͅ", r) {
				b.WriteRune(r)
			}
		}
		text = b.String()
	}

	return pyUpper(text)
}

// lowercase is lowercase in weasyprint/formatting_structure/text.py.
func lowercase(text, langCode string) string {
	var mapper [][2]string

	switch langCode {
	case "tr", "az":
		// https://github.com/unicode-org/cldr/blob/main/common/transforms/tr-Lower.xml
		mapper = [][2]string{{"I", "ı"}, {"İ", "i"}}
	case "lt":
		// https://github.com/unicode-org/cldr/blob/main/common/transforms/lt-Lower.xml
		mapper = [][2]string{{"Ì", "i̇̀"}, {"Í", "i̇́"}, {"Ĩ", "i̇̃"}}
	}

	for _, pair := range mapper {
		text = strings.ReplaceAll(text, pair[0], pair[1])
	}

	return pyLower(text)
}

// capitalize is capitalize in weasyprint/formatting_structure/text.py.
func capitalize(text, langCode string) string {
	letterFound := false
	skipNextLetter := false
	var output strings.Builder
	for i, letter := range text {
		if skipNextLetter {
			skipNextLetter = false
			continue
		}
		s := string(letter)
		switch {
		case !letterFound && (unicode.IsLetter(letter) || unicode.IsNumber(letter)):
			letterFound = true
			switch {
			case langCode == "nl" && strings.HasPrefix(text[i:], "ij"):
				skipNextLetter = true
				s = "IJ"
			case langCode == "tr" || langCode == "az":
				s = uppercase(s, langCode)
			default:
				s = pyUpper(s)
			}
		case unicode.In(letter, unicode.Z):
			letterFound = false
		}
		output.WriteString(s)
	}
	return output.String()
}

// fullWidth is full_width in weasyprint/formatting_structure/text.py. It
// maps the ASCII characters to their full-width forms, as ASCII_TO_WIDE
// does. See https://stackoverflow.com/questions/16317534/.
func fullWidth(text, _ string) string {
	return strings.Map(func(r rune) rune {
		switch {
		case r == 0x20:
			return '　'
		case r == 0x2D:
			return '−'
		case r >= 0x21 && r < 0x7f:
			return r + 0xfee0
		}
		return r
	}, text)
}

// getFirstLetter is get_first_letter in weasyprint/formatting_structure/
// text.py: the punctuation at the start of a text, with the first character
// that is not punctuation and the punctuation after it.
func getFirstLetter(text string) string {
	characterFound := false
	var firstLetter strings.Builder
	for _, letter := range text {
		if !unicode.In(letter, unicode.Ps, unicode.Pe, unicode.Pi, unicode.Pf, unicode.Po) {
			if characterFound {
				break
			}
			characterFound = true
		}
		firstLetter.WriteRune(letter)
	}
	return firstLetter.String()
}

// pyUpper is the method upper of the str of Python.
func pyUpper(text string) string {
	var b strings.Builder
	b.Grow(len(text))
	for _, r := range text {
		if s, ok := specialUpper[r]; ok {
			b.WriteString(s)
		} else {
			b.WriteRune(unicode.ToUpper(r))
		}
	}
	return b.String()
}

// pyLower is the method lower of the str of Python, with the final sigma.
func pyLower(text string) string {
	var b strings.Builder
	b.Grow(len(text))
	for i, r := range text {
		switch {
		case r == 'Σ' && isFinalSigma(text, i):
			b.WriteRune('ς')
		case specialLower[r] != "":
			b.WriteString(specialLower[r])
		default:
			b.WriteRune(unicode.ToLower(r))
		}
	}
	return b.String()
}

// isFinalSigma reports whether the capital sigma at the index i of text is
// at the end of a word: a cased letter comes before it, and no cased letter
// comes after it, past the characters that the case ignores.
func isFinalSigma(text string, i int) bool {
	before := false
	for j := i; j > 0; {
		r, size := utf8.DecodeLastRuneInString(text[:j])
		j -= size
		if caseIgnorable(r) {
			continue
		}
		before = cased(r)
		break
	}
	if !before {
		return false
	}
	for _, r := range text[i+len("Σ"):] {
		if caseIgnorable(r) {
			continue
		}
		return !cased(r)
	}
	return true
}

// cased reports whether r has the property Cased of Unicode.
func cased(r rune) bool {
	return unicode.IsUpper(r) || unicode.IsLower(r) || unicode.IsTitle(r)
}

// caseIgnorable reports whether r has the property Case_Ignorable of
// Unicode, for the marks, the format characters and the modifiers.
func caseIgnorable(r rune) bool {
	return unicode.In(r, unicode.Mn, unicode.Me, unicode.Cf, unicode.Lm, unicode.Sk) ||
		strings.ContainsRune("'.:^`\u00ad·‘’․‧", r)
}

// greekWithoutDiacritics maps the Greek letters to the same letters without
// the diacritics that uppercase removes: unicodedata.normalize('NFD'), the
// removal of the marks, and unicodedata.normalize('NFC') of upstream.
var greekWithoutDiacritics = map[rune]string{
	0x0374: "\u02b9",
	0x037E: ";",
	0x0385: "\u00a8",
	0x0386: "\u0391",
	0x0387: "\u00b7",
	0x0388: "\u0395",
	0x0389: "\u0397",
	0x038A: "\u0399",
	0x038C: "\u039f",
	0x038E: "\u03a5",
	0x038F: "\u03a9",
	0x0390: "\u03ca",
	0x03AC: "\u03b1",
	0x03AD: "\u03b5",
	0x03AE: "\u03b7",
	0x03AF: "\u03b9",
	0x03B0: "\u03cb",
	0x03CC: "\u03bf",
	0x03CD: "\u03c5",
	0x03CE: "\u03c9",
	0x03D3: "\u03d2",
	0x1F00: "\u03b1",
	0x1F01: "\u03b1",
	0x1F02: "\u03b1",
	0x1F03: "\u03b1",
	0x1F04: "\u03b1",
	0x1F05: "\u03b1",
	0x1F06: "\u03b1",
	0x1F07: "\u03b1",
	0x1F08: "\u0391",
	0x1F09: "\u0391",
	0x1F0A: "\u0391",
	0x1F0B: "\u0391",
	0x1F0C: "\u0391",
	0x1F0D: "\u0391",
	0x1F0E: "\u0391",
	0x1F0F: "\u0391",
	0x1F10: "\u03b5",
	0x1F11: "\u03b5",
	0x1F12: "\u03b5",
	0x1F13: "\u03b5",
	0x1F14: "\u03b5",
	0x1F15: "\u03b5",
	0x1F18: "\u0395",
	0x1F19: "\u0395",
	0x1F1A: "\u0395",
	0x1F1B: "\u0395",
	0x1F1C: "\u0395",
	0x1F1D: "\u0395",
	0x1F20: "\u03b7",
	0x1F21: "\u03b7",
	0x1F22: "\u03b7",
	0x1F23: "\u03b7",
	0x1F24: "\u03b7",
	0x1F25: "\u03b7",
	0x1F26: "\u03b7",
	0x1F27: "\u03b7",
	0x1F28: "\u0397",
	0x1F29: "\u0397",
	0x1F2A: "\u0397",
	0x1F2B: "\u0397",
	0x1F2C: "\u0397",
	0x1F2D: "\u0397",
	0x1F2E: "\u0397",
	0x1F2F: "\u0397",
	0x1F30: "\u03b9",
	0x1F31: "\u03b9",
	0x1F32: "\u03b9",
	0x1F33: "\u03b9",
	0x1F34: "\u03b9",
	0x1F35: "\u03b9",
	0x1F36: "\u03b9",
	0x1F37: "\u03b9",
	0x1F38: "\u0399",
	0x1F39: "\u0399",
	0x1F3A: "\u0399",
	0x1F3B: "\u0399",
	0x1F3C: "\u0399",
	0x1F3D: "\u0399",
	0x1F3E: "\u0399",
	0x1F3F: "\u0399",
	0x1F40: "\u03bf",
	0x1F41: "\u03bf",
	0x1F42: "\u03bf",
	0x1F43: "\u03bf",
	0x1F44: "\u03bf",
	0x1F45: "\u03bf",
	0x1F48: "\u039f",
	0x1F49: "\u039f",
	0x1F4A: "\u039f",
	0x1F4B: "\u039f",
	0x1F4C: "\u039f",
	0x1F4D: "\u039f",
	0x1F50: "\u03c5",
	0x1F51: "\u03c5",
	0x1F52: "\u03c5",
	0x1F53: "\u03c5",
	0x1F54: "\u03c5",
	0x1F55: "\u03c5",
	0x1F56: "\u03c5",
	0x1F57: "\u03c5",
	0x1F59: "\u03a5",
	0x1F5B: "\u03a5",
	0x1F5D: "\u03a5",
	0x1F5F: "\u03a5",
	0x1F60: "\u03c9",
	0x1F61: "\u03c9",
	0x1F62: "\u03c9",
	0x1F63: "\u03c9",
	0x1F64: "\u03c9",
	0x1F65: "\u03c9",
	0x1F66: "\u03c9",
	0x1F67: "\u03c9",
	0x1F68: "\u03a9",
	0x1F69: "\u03a9",
	0x1F6A: "\u03a9",
	0x1F6B: "\u03a9",
	0x1F6C: "\u03a9",
	0x1F6D: "\u03a9",
	0x1F6E: "\u03a9",
	0x1F6F: "\u03a9",
	0x1F70: "\u03b1",
	0x1F71: "\u03b1",
	0x1F72: "\u03b5",
	0x1F73: "\u03b5",
	0x1F74: "\u03b7",
	0x1F75: "\u03b7",
	0x1F76: "\u03b9",
	0x1F77: "\u03b9",
	0x1F78: "\u03bf",
	0x1F79: "\u03bf",
	0x1F7A: "\u03c5",
	0x1F7B: "\u03c5",
	0x1F7C: "\u03c9",
	0x1F7D: "\u03c9",
	0x1F80: "\u03b1",
	0x1F81: "\u03b1",
	0x1F82: "\u03b1",
	0x1F83: "\u03b1",
	0x1F84: "\u03b1",
	0x1F85: "\u03b1",
	0x1F86: "\u03b1",
	0x1F87: "\u03b1",
	0x1F88: "\u0391",
	0x1F89: "\u0391",
	0x1F8A: "\u0391",
	0x1F8B: "\u0391",
	0x1F8C: "\u0391",
	0x1F8D: "\u0391",
	0x1F8E: "\u0391",
	0x1F8F: "\u0391",
	0x1F90: "\u03b7",
	0x1F91: "\u03b7",
	0x1F92: "\u03b7",
	0x1F93: "\u03b7",
	0x1F94: "\u03b7",
	0x1F95: "\u03b7",
	0x1F96: "\u03b7",
	0x1F97: "\u03b7",
	0x1F98: "\u0397",
	0x1F99: "\u0397",
	0x1F9A: "\u0397",
	0x1F9B: "\u0397",
	0x1F9C: "\u0397",
	0x1F9D: "\u0397",
	0x1F9E: "\u0397",
	0x1F9F: "\u0397",
	0x1FA0: "\u03c9",
	0x1FA1: "\u03c9",
	0x1FA2: "\u03c9",
	0x1FA3: "\u03c9",
	0x1FA4: "\u03c9",
	0x1FA5: "\u03c9",
	0x1FA6: "\u03c9",
	0x1FA7: "\u03c9",
	0x1FA8: "\u03a9",
	0x1FA9: "\u03a9",
	0x1FAA: "\u03a9",
	0x1FAB: "\u03a9",
	0x1FAC: "\u03a9",
	0x1FAD: "\u03a9",
	0x1FAE: "\u03a9",
	0x1FAF: "\u03a9",
	0x1FB0: "\u03b1",
	0x1FB1: "\u03b1",
	0x1FB2: "\u03b1",
	0x1FB3: "\u03b1",
	0x1FB4: "\u03b1",
	0x1FB6: "\u03b1",
	0x1FB7: "\u03b1",
	0x1FB8: "\u0391",
	0x1FB9: "\u0391",
	0x1FBA: "\u0391",
	0x1FBB: "\u0391",
	0x1FBC: "\u0391",
	0x1FBE: "\u03b9",
	0x1FC1: "\u00a8",
	0x1FC2: "\u03b7",
	0x1FC3: "\u03b7",
	0x1FC4: "\u03b7",
	0x1FC6: "\u03b7",
	0x1FC7: "\u03b7",
	0x1FC8: "\u0395",
	0x1FC9: "\u0395",
	0x1FCA: "\u0397",
	0x1FCB: "\u0397",
	0x1FCC: "\u0397",
	0x1FCD: "\u1fbf",
	0x1FCE: "\u1fbf",
	0x1FCF: "\u1fbf",
	0x1FD0: "\u03b9",
	0x1FD1: "\u03b9",
	0x1FD2: "\u03ca",
	0x1FD3: "\u03ca",
	0x1FD6: "\u03b9",
	0x1FD7: "\u03ca",
	0x1FD8: "\u0399",
	0x1FD9: "\u0399",
	0x1FDA: "\u0399",
	0x1FDB: "\u0399",
	0x1FDD: "\u1ffe",
	0x1FDE: "\u1ffe",
	0x1FDF: "\u1ffe",
	0x1FE0: "\u03c5",
	0x1FE1: "\u03c5",
	0x1FE2: "\u03cb",
	0x1FE3: "\u03cb",
	0x1FE4: "\u03c1",
	0x1FE5: "\u03c1",
	0x1FE6: "\u03c5",
	0x1FE7: "\u03cb",
	0x1FE8: "\u03a5",
	0x1FE9: "\u03a5",
	0x1FEA: "\u03a5",
	0x1FEB: "\u03a5",
	0x1FEC: "\u03a1",
	0x1FED: "\u00a8",
	0x1FEE: "\u00a8",
	0x1FEF: "`",
	0x1FF2: "\u03c9",
	0x1FF3: "\u03c9",
	0x1FF4: "\u03c9",
	0x1FF6: "\u03c9",
	0x1FF7: "\u03c9",
	0x1FF8: "\u039f",
	0x1FF9: "\u039f",
	0x1FFA: "\u03a9",
	0x1FFB: "\u03a9",
	0x1FFC: "\u03a9",
	0x1FFD: "\u00b4",
}

// specialUpper holds the characters whose upper case in Python is more than
// one character.
var specialUpper = map[rune]string{
	0x00DF: "SS",
	0x0149: "\u02bcN",
	0x01F0: "J\u030c",
	0x0390: "\u0399\u0308\u0301",
	0x03B0: "\u03a5\u0308\u0301",
	0x0587: "\u0535\u0552",
	0x1E96: "H\u0331",
	0x1E97: "T\u0308",
	0x1E98: "W\u030a",
	0x1E99: "Y\u030a",
	0x1E9A: "A\u02be",
	0x1F50: "\u03a5\u0313",
	0x1F52: "\u03a5\u0313\u0300",
	0x1F54: "\u03a5\u0313\u0301",
	0x1F56: "\u03a5\u0313\u0342",
	0x1F80: "\u1f08\u0399",
	0x1F81: "\u1f09\u0399",
	0x1F82: "\u1f0a\u0399",
	0x1F83: "\u1f0b\u0399",
	0x1F84: "\u1f0c\u0399",
	0x1F85: "\u1f0d\u0399",
	0x1F86: "\u1f0e\u0399",
	0x1F87: "\u1f0f\u0399",
	0x1F88: "\u1f08\u0399",
	0x1F89: "\u1f09\u0399",
	0x1F8A: "\u1f0a\u0399",
	0x1F8B: "\u1f0b\u0399",
	0x1F8C: "\u1f0c\u0399",
	0x1F8D: "\u1f0d\u0399",
	0x1F8E: "\u1f0e\u0399",
	0x1F8F: "\u1f0f\u0399",
	0x1F90: "\u1f28\u0399",
	0x1F91: "\u1f29\u0399",
	0x1F92: "\u1f2a\u0399",
	0x1F93: "\u1f2b\u0399",
	0x1F94: "\u1f2c\u0399",
	0x1F95: "\u1f2d\u0399",
	0x1F96: "\u1f2e\u0399",
	0x1F97: "\u1f2f\u0399",
	0x1F98: "\u1f28\u0399",
	0x1F99: "\u1f29\u0399",
	0x1F9A: "\u1f2a\u0399",
	0x1F9B: "\u1f2b\u0399",
	0x1F9C: "\u1f2c\u0399",
	0x1F9D: "\u1f2d\u0399",
	0x1F9E: "\u1f2e\u0399",
	0x1F9F: "\u1f2f\u0399",
	0x1FA0: "\u1f68\u0399",
	0x1FA1: "\u1f69\u0399",
	0x1FA2: "\u1f6a\u0399",
	0x1FA3: "\u1f6b\u0399",
	0x1FA4: "\u1f6c\u0399",
	0x1FA5: "\u1f6d\u0399",
	0x1FA6: "\u1f6e\u0399",
	0x1FA7: "\u1f6f\u0399",
	0x1FA8: "\u1f68\u0399",
	0x1FA9: "\u1f69\u0399",
	0x1FAA: "\u1f6a\u0399",
	0x1FAB: "\u1f6b\u0399",
	0x1FAC: "\u1f6c\u0399",
	0x1FAD: "\u1f6d\u0399",
	0x1FAE: "\u1f6e\u0399",
	0x1FAF: "\u1f6f\u0399",
	0x1FB2: "\u1fba\u0399",
	0x1FB3: "\u0391\u0399",
	0x1FB4: "\u0386\u0399",
	0x1FB6: "\u0391\u0342",
	0x1FB7: "\u0391\u0342\u0399",
	0x1FBC: "\u0391\u0399",
	0x1FC2: "\u1fca\u0399",
	0x1FC3: "\u0397\u0399",
	0x1FC4: "\u0389\u0399",
	0x1FC6: "\u0397\u0342",
	0x1FC7: "\u0397\u0342\u0399",
	0x1FCC: "\u0397\u0399",
	0x1FD2: "\u0399\u0308\u0300",
	0x1FD3: "\u0399\u0308\u0301",
	0x1FD6: "\u0399\u0342",
	0x1FD7: "\u0399\u0308\u0342",
	0x1FE2: "\u03a5\u0308\u0300",
	0x1FE3: "\u03a5\u0308\u0301",
	0x1FE4: "\u03a1\u0313",
	0x1FE6: "\u03a5\u0342",
	0x1FE7: "\u03a5\u0308\u0342",
	0x1FF2: "\u1ffa\u0399",
	0x1FF3: "\u03a9\u0399",
	0x1FF4: "\u038f\u0399",
	0x1FF6: "\u03a9\u0342",
	0x1FF7: "\u03a9\u0342\u0399",
	0x1FFC: "\u03a9\u0399",
	0xFB00: "FF",
	0xFB01: "FI",
	0xFB02: "FL",
	0xFB03: "FFI",
	0xFB04: "FFL",
	0xFB05: "ST",
	0xFB06: "ST",
	0xFB13: "\u0544\u0546",
	0xFB14: "\u0544\u0535",
	0xFB15: "\u0544\u053b",
	0xFB16: "\u054e\u0546",
	0xFB17: "\u0544\u053d",
}

// specialLower holds the characters whose lower case in Python is more than
// one character.
var specialLower = map[rune]string{
	0x0130: "i\u0307",
}
