package ebook

import (
	"bytes"
	"strings"
	"unicode/utf16"
	"unicode/utf8"
)

// Text returns buf as UTF-8 text, without a byte order mark. It reads UTF-8
// and UTF-16 with a byte order mark, and UTF-16 without one when buf starts
// with "<" and a zero byte, as an XML file in UTF-16 does. Other text is
// UTF-8, or windows-1252 if it is not valid UTF-8.
func Text(buf []byte) string {
	switch {
	case bytes.HasPrefix(buf, []byte{0xef, 0xbb, 0xbf}):
		return validUTF8(buf[3:])
	case bytes.HasPrefix(buf, []byte{0xff, 0xfe}):
		return utf16Text(buf[2:], false)
	case bytes.HasPrefix(buf, []byte{0xfe, 0xff}):
		return utf16Text(buf[2:], true)
	case len(buf) >= 2 && buf[0] == '<' && buf[1] == 0:
		return utf16Text(buf, false)
	case len(buf) >= 2 && buf[0] == 0 && buf[1] == '<':
		return utf16Text(buf, true)
	case utf8.Valid(buf):
		return string(buf)
	}
	return Windows1252(buf)
}

// validUTF8 returns buf as a string, with U+FFFD in the place of each byte
// that is not valid UTF-8.
func validUTF8(buf []byte) string {
	if utf8.Valid(buf) {
		return string(buf)
	}
	return strings.ToValidUTF8(string(buf), "�")
}

// utf16Text decodes UTF-16 text. A last odd byte is dropped.
func utf16Text(buf []byte, bigEndian bool) string {
	units := make([]uint16, len(buf)/2)
	for i := range units {
		if bigEndian {
			units[i] = uint16(buf[2*i])<<8 | uint16(buf[2*i+1])
		} else {
			units[i] = uint16(buf[2*i+1])<<8 | uint16(buf[2*i])
		}
	}
	return string(utf16.Decode(units))
}

// cp1252 holds the characters of the bytes 0x80 to 0x9f of windows-1252. A
// byte that windows-1252 leaves out keeps its code point, as the WHATWG
// Encoding Standard says.
var cp1252 = [32]rune{
	'€', '\u0081', '‚', 'ƒ', '„', '…', '†', '‡',
	'ˆ', '‰', 'Š', '‹', 'Œ', '\u008d', 'Ž', '\u008f',
	'\u0090', '‘', '’', '“', '”', '•', '–', '—',
	'˜', '™', 'š', '›', 'œ', '\u009d', 'ž', 'Ÿ',
}

// Windows1252 decodes buf as windows-1252.
func Windows1252(buf []byte) string {
	var b strings.Builder
	b.Grow(len(buf) + len(buf)/4)
	for _, c := range buf {
		switch {
		case c < 0x80:
			b.WriteByte(c)
		case c < 0xa0:
			b.WriteRune(cp1252[c-0x80])
		default:
			b.WriteRune(rune(c))
		}
	}
	return b.String()
}
