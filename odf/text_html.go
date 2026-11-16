package odf

import (
	"strconv"
	"strings"
	"unicode/utf8"
)

// odtEscape writes s to b with the characters of XML escaped. It leaves out
// the characters that XML 1.0 does not allow. Inside an attribute, attr
// escapes the quotes too.
func odtEscape(b *strings.Builder, s string, attr bool) {
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
		case r == '\t' || r == '\n':
			if attr {
				b.WriteString("&#" + strconv.Itoa(int(r)) + ";")
			} else {
				b.WriteRune(r)
			}
		case r < 0x20, r == 0xfffe, r == 0xffff, r == utf8.RuneError && size == 1:
		default:
			b.WriteRune(r)
		}
	}
}

// odtAttrText returns s escaped for an attribute.
func odtAttrText(s string) string {
	var b strings.Builder
	odtEscape(&b, s, true)
	return b.String()
}

// odtPt formats a length in points for CSS.
func odtPt(v float64) string {
	return odtNum(v) + "pt"
}

// odtNum formats a number for CSS, with 2 decimals at most.
func odtNum(v float64) string {
	if v != v || v > 1e9 || v < -1e9 {
		v = 0
	}
	return strconv.FormatFloat(float64(int64(v*100+map[bool]float64{true: -0.5, false: 0.5}[v < 0]))/100, 'f', -1, 64)
}

// odtQuote returns a font family as a CSS string.
func odtQuote(name string) string {
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

// odtCSS builds the declarations of a style attribute.
type odtCSS struct {
	b strings.Builder
}

// set adds the declaration name: value.
func (c *odtCSS) set(name, value string) {
	if value == "" {
		return
	}
	if c.b.Len() > 0 {
		c.b.WriteString("; ")
	}
	c.b.WriteString(name + ": " + value)
}

// String returns the declarations.
func (c *odtCSS) String() string {
	return c.b.String()
}

// odtSafeValue returns a value of OpenDocument that CSS reads the same way,
// such as a border "0.5pt solid #000000", or "" for another value.
func odtSafeValue(s string) string {
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
		case strings.ContainsRune("#.%(), -", r):
		default:
			return ""
		}
	}
	return strings.TrimSpace(s)
}

// odtLineRatios are the heights of a single line of the fonts that office
// documents use most, as a multiple of the font size.
var odtLineRatios = map[string]float64{
	"calibri": 1.2207, "carlito": 1.2207,
	"cambria": 1.1719, "caladea": 1.1719,
	"arial": 1.1499, "liberation sans": 1.1499, "helvetica": 1.1499,
	"times new roman": 1.1499, "liberation serif": 1.1499, "times": 1.1499,
	"courier new": 1.1328, "liberation mono": 1.1328,
	"dejavu sans": 1.1641, "dejavu serif": 1.1641,
}

// odtLineRatio returns the height of a single line of a font.
func odtLineRatio(family string) float64 {
	if r, ok := odtLineRatios[strings.ToLower(family)]; ok {
		return r
	}
	return 1.17
}
