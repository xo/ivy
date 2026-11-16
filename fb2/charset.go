package fb2

import (
	"bytes"
	"strings"

	"github.com/xo/ivy/internal/ebook"
)

// decodeText returns the text of an XML file as UTF-8, without its XML
// declaration. It reads a byte order mark first, as ebook.Text does, then
// the encoding that the XML declaration names. It knows UTF-8, UTF-16,
// windows-1251 and windows-1252, which also reads ISO-8859-1 and ASCII. It
// returns false as its second result for an encoding that it does not know,
// and then reads the text as ebook.Text does.
func decodeText(buf []byte) (string, bool) {
	if bytes.HasPrefix(buf, []byte{0xef, 0xbb, 0xbf}) || bytes.HasPrefix(buf, []byte{0xff, 0xfe}) ||
		bytes.HasPrefix(buf, []byte{0xfe, 0xff}) || (len(buf) >= 2 && (buf[0] == 0 || buf[1] == 0)) {
		return stripDeclaration(ebook.Text(buf)), true
	}
	var text string
	known := true
	switch strings.ToLower(declaredEncoding(buf)) {
	case "", "utf-8", "utf8":
		text = ebook.Text(buf)
	case "windows-1251", "cp1251", "x-cp1251", "cp-1251":
		text = windows1251Text(buf)
	case "windows-1252", "cp1252", "x-cp1252", "iso-8859-1", "iso8859-1", "latin1", "us-ascii", "ascii":
		text = ebook.Windows1252(buf)
	default:
		text, known = ebook.Text(buf), false
	}
	return stripDeclaration(text), known
}

// declaredEncoding returns the value of the encoding of the XML declaration
// at the start of buf, or "" if there is none.
func declaredEncoding(buf []byte) string {
	if !bytes.HasPrefix(buf, []byte("<?xml")) {
		return ""
	}
	end := bytes.Index(buf, []byte("?>"))
	if end < 0 || end > 1024 {
		return ""
	}
	decl := string(buf[:end])
	i := strings.Index(decl, "encoding")
	if i < 0 {
		return ""
	}
	rest := strings.TrimLeft(decl[i+len("encoding"):], " \t\r\n")
	rest, ok := strings.CutPrefix(rest, "=")
	if !ok {
		return ""
	}
	rest = strings.TrimLeft(rest, " \t\r\n")
	if rest == "" || (rest[0] != '"' && rest[0] != '\'') {
		return ""
	}
	value, _, ok := strings.Cut(rest[1:], rest[:1])
	if !ok {
		return ""
	}
	return strings.TrimSpace(value)
}

// stripDeclaration removes the XML declaration from the start of text. The
// text is UTF-8 now, so a declaration of another encoding no longer holds.
func stripDeclaration(text string) string {
	if !strings.HasPrefix(text, "<?xml") {
		return text
	}
	if end := strings.Index(text, "?>"); end >= 0 {
		return text[end+2:]
	}
	return text
}

// windows1251Text decodes text of windows-1251. The bytes below 0x80 are
// ASCII.
func windows1251Text(buf []byte) string {
	var b strings.Builder
	b.Grow(len(buf) * 2)
	for _, c := range buf {
		if c < 0x80 {
			b.WriteByte(c)
		} else {
			b.WriteRune(windows1251[c-0x80])
		}
	}
	return b.String()
}

// windows1251 holds the characters of the bytes from 0x80 of windows-1251,
// the Cyrillic code page of Windows, as the Encoding Standard of WHATWG maps
// them. The byte 0x98 has no character, and it maps to U+0098.
var windows1251 = [128]rune{
	0x0402, 0x0403, 0x201A, 0x0453, 0x201E, 0x2026, 0x2020, 0x2021, // 0x80
	0x20AC, 0x2030, 0x0409, 0x2039, 0x040A, 0x040C, 0x040B, 0x040F, // 0x88
	0x0452, 0x2018, 0x2019, 0x201C, 0x201D, 0x2022, 0x2013, 0x2014, // 0x90
	0x0098, 0x2122, 0x0459, 0x203A, 0x045A, 0x045C, 0x045B, 0x045F, // 0x98
	0x00A0, 0x040E, 0x045E, 0x0408, 0x00A4, 0x0490, 0x00A6, 0x00A7, // 0xA0
	0x0401, 0x00A9, 0x0404, 0x00AB, 0x00AC, 0x00AD, 0x00AE, 0x0407, // 0xA8
	0x00B0, 0x00B1, 0x0406, 0x0456, 0x0491, 0x00B5, 0x00B6, 0x00B7, // 0xB0
	0x0451, 0x2116, 0x0454, 0x00BB, 0x0458, 0x0405, 0x0455, 0x0457, // 0xB8
	0x0410, 0x0411, 0x0412, 0x0413, 0x0414, 0x0415, 0x0416, 0x0417, // 0xC0
	0x0418, 0x0419, 0x041A, 0x041B, 0x041C, 0x041D, 0x041E, 0x041F, // 0xC8
	0x0420, 0x0421, 0x0422, 0x0423, 0x0424, 0x0425, 0x0426, 0x0427, // 0xD0
	0x0428, 0x0429, 0x042A, 0x042B, 0x042C, 0x042D, 0x042E, 0x042F, // 0xD8
	0x0430, 0x0431, 0x0432, 0x0433, 0x0434, 0x0435, 0x0436, 0x0437, // 0xE0
	0x0438, 0x0439, 0x043A, 0x043B, 0x043C, 0x043D, 0x043E, 0x043F, // 0xE8
	0x0440, 0x0441, 0x0442, 0x0443, 0x0444, 0x0445, 0x0446, 0x0447, // 0xF0
	0x0448, 0x0449, 0x044A, 0x044B, 0x044C, 0x044D, 0x044E, 0x044F, // 0xF8
}
