package docx

import (
	"strconv"
	"strings"
	"unicode/utf8"
)

// escape writes s to b with the characters of XML escaped. It leaves out
// the characters that XML 1.0 does not allow. Inside an attribute, attr
// escapes the quotes too.
func escape(b *strings.Builder, s string, attr bool) {
	for i := 0; i < len(s); {
		r, size := utf8.DecodeRuneInString(s[i:])
		i += size
		switch {
		case r == '<':
			b.WriteString("&lt;")
		case r == '>':
			b.WriteString("&gt;")
		case r == '&':
			b.WriteString("&amp;")
		case r == '"' && attr:
			b.WriteString("&quot;")
		case r == '\t' || r == '\n' || r == '\r':
			if attr {
				b.WriteString("&#" + strconv.Itoa(int(r)) + ";")
			} else {
				b.WriteRune(r)
			}
		case r < 0x20, r == 0xfffe, r == 0xffff, r == utf8.RuneError && size == 1:
		case r >= 0xd800 && r <= 0xdfff:
		default:
			b.WriteRune(r)
		}
	}
}

// pt formats a length in points for CSS.
func pt(v float64) string {
	return strconv.FormatFloat(round(v), 'f', -1, 64) + "pt"
}

// round rounds v to 2 decimals.
func round(v float64) float64 {
	if v != v || v > 1e9 || v < -1e9 {
		return 0
	}
	return float64(int64(v*100+copySign(0.5, v))) / 100
}

// copySign returns the magnitude of a with the sign of b.
func copySign(a, b float64) float64 {
	if b < 0 {
		return -a
	}
	return a
}

// num formats a number for CSS.
func num(v float64) string {
	return strconv.FormatFloat(round(v), 'f', -1, 64)
}

// quoteFamily returns a font family as a CSS string.
func quoteFamily(name string) string {
	var b strings.Builder
	b.WriteByte('"')
	for _, r := range name {
		switch {
		case r == '"' || r == '\\':
			b.WriteByte('\\')
			b.WriteRune(r)
		case r < 0x20 || r == 0x7f:
		default:
			b.WriteRune(r)
		}
	}
	b.WriteByte('"')
	return b.String()
}

// css builds the declarations of a style attribute.
type css struct {
	b strings.Builder
}

// set adds the declaration name: value.
func (c *css) set(name, value string) {
	if value == "" {
		return
	}
	if c.b.Len() > 0 {
		c.b.WriteString("; ")
	}
	c.b.WriteString(name)
	c.b.WriteString(": ")
	c.b.WriteString(value)
}

// String returns the declarations.
func (c *css) String() string {
	return c.b.String()
}
