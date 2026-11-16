package fb2

import (
	"bytes"
	"strings"

	"github.com/xo/ivy/internal/charset"
	"github.com/xo/ivy/internal/ebook"
)

// decodeText returns the text of an XML file as UTF-8, without its XML
// declaration. It reads a byte order mark first, as ebook.Text does, then
// the encoding that the XML declaration names. It knows each encoding of the
// WHATWG Encoding Standard, such as windows-1251 and KOI8-R. It returns
// false as its second result for an encoding that it does not know, and
// then reads the text as ebook.Text does.
func decodeText(buf []byte) (string, bool) {
	if bytes.HasPrefix(buf, []byte{0xef, 0xbb, 0xbf}) || bytes.HasPrefix(buf, []byte{0xff, 0xfe}) ||
		bytes.HasPrefix(buf, []byte{0xfe, 0xff}) || (len(buf) >= 2 && (buf[0] == 0 || buf[1] == 0)) {
		return stripDeclaration(ebook.Text(buf)), true
	}
	label := strings.ToLower(declaredEncoding(buf))
	switch label {
	case "", "utf-8", "utf8":
		return stripDeclaration(ebook.Text(buf)), true
	}
	text, ok := charset.Decode(buf, label)
	if !ok {
		return stripDeclaration(ebook.Text(buf)), false
	}
	return stripDeclaration(text), true
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
