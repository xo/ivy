package mobi

import (
	"bytes"
	"fmt"
	"html"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/xo/ivy"
)

// mobiStyle is the style sheet of a MOBI book, after the style sheet of an
// ebook. It gives the page breaks of the book.
const mobiStyle = `.mbp-pagebreak {
  break-after: page;
}
pre.palmdoc {
  white-space: pre-wrap;
  font-family: serif;
}
`

// mobi6Style is the style sheet of the HTML of a MOBI 6 book, after
// mobiStyle. It matches the pages that calibre makes of such a book. The
// rules come from measurements of the PDF files of ebook-convert, not from
// the code of calibre, which is under the GPL (D7):
//
//   - The text is justified. A paragraph has no margin, and its first line
//     has an indent of 1.5em.
//   - A blockquote has a left margin of 2em and no other margin. Eleven
//     blockquotes inside each other put the text 22em from the left.
//   - A line of hr spans the column whatever its attribute width, and it
//     is 4px high, with a border of 1px. mobi6Attrs removes the width.
//
// mobi6Attrs gives the attributes height and width of a paragraph, and the
// attribute size of a font.
const mobi6Style = `body {
  text-align: justify;
}
p {
  margin: 0;
  text-indent: 1.5em;
}
blockquote {
  margin: 0 0 0 2em;
}
hr {
  height: 2px;
  margin: 0.5em 0;
  border-style: solid;
  border-width: 1px;
  border-color: #9a9a9a #eee #eee #9a9a9a;
}
`

// The markup of MOBI that the conversion changes.
var (
	// fileposRef is a link to a position in the text, such as
	// <a filepos=0000012345>.
	fileposRef = regexp.MustCompile(`(?i)\bfilepos\s*=\s*["']?0*([0-9]{1,10})["']?`)
	// recindexRef is the image of an img element, such as
	// <img recindex="00001">. hirecindex and lorecindex name the same image
	// in another resolution.
	recindexRef = regexp.MustCompile(`(?i)\b(?:hi|lo)?recindex\s*=\s*["']?0*([0-9]{1,10})["']?`)
	// pageBreak is the page break of MOBI.
	pageBreak = regexp.MustCompile(`(?i)<\s*mbp:pagebreak\b[^>]*>|<\s*/\s*mbp:pagebreak\s*>`)
	// brEnd is the end tag of a line break. A MOBI book often writes a line
	// break as <br></br>, which the parser of HTML reads as two breaks, as
	// HTML says. calibre reads one.
	brEnd = regexp.MustCompile(`(?i)^<\s*/\s*br\s*>$`)
	// guide is the guide of a MOBI book, in its head, which links to parts
	// of the book and shows nothing.
	guide = regexp.MustCompile(`(?is)<guide\b[^>]*>.*?</guide\s*>`)
	// tagName is the name of a start tag.
	tagName = regexp.MustCompile(`^<\s*([a-zA-Z][a-zA-Z0-9]*)`)
	// layoutAttr is an attribute of MOBI that gives a space or a size, such
	// as height="1em", width="-14pt" or size="+1".
	layoutAttr = regexp.MustCompile(`(?i)\s(height|width|size)\s*=\s*["']?([^"'\s>]*)`)
	// widthAttr is the attribute width.
	widthAttr = regexp.MustCompile(`(?i)\swidth\s*=\s*("[^"]*"|'[^']*'|[^\s>]*)`)
	// styleAttr is the attribute style.
	styleAttr = regexp.MustCompile(`(?i)\sstyle\s*=`)
	// length is a length of an attribute of MOBI, such as 3pt or -1.5em.
	length = regexp.MustCompile(`^-?[0-9]{1,6}(\.[0-9]{1,6})?(pt|em|px|%)$`)
)

// fontSizes are the sizes of the font element of MOBI, from size 1 to 7,
// that calibre draws. They are absolute, so size 3 is not the size of the
// text. ebook-convert with a text of 11.25pt draws them at 6, 9, 12, 13.5,
// 17, 20 and 22 points.
var fontSizes = [...]string{"6pt", "9pt", "12pt", "13.5pt", "17pt", "20pt", "22pt"}

// mobi6Attrs returns tag with a style that gives its attributes of MOBI, as
// calibre draws them. On a paragraph, a div or a blockquote, height is the
// space above, and width is the indent of the first line. A negative width
// is a hanging indent: the other lines start that far from the left. On a
// font element, size is a size of fontSizes, or a size relative to 3, such
// as +1. A tag that has a style already is left as it is. calibre draws a
// line of hr across the column, so mobi6Attrs removes its width.
func mobi6Attrs(tag string) string {
	m := tagName.FindStringSubmatch(tag)
	if m == nil {
		return tag
	}
	name := strings.ToLower(m[1])
	if name == "hr" {
		return widthAttr.ReplaceAllString(tag, "")
	}
	if name != "p" && name != "div" && name != "blockquote" && name != "font" {
		return tag
	}
	if styleAttr.MatchString(tag) {
		return tag
	}
	var style []string
	for _, a := range layoutAttr.FindAllStringSubmatch(tag, 4) {
		attr, v := strings.ToLower(a[1]), strings.ToLower(a[2])
		switch {
		case name == "font" && attr == "size":
			if size, ok := fontSize(v); ok {
				style = append(style, "font-size: "+size)
			}
		case name == "font", !length.MatchString(v):
		case attr == "height":
			style = append(style, "margin-top: "+v)
		case attr == "width":
			style = append(style, "text-indent: "+v)
			if indent, ok := strings.CutPrefix(v, "-"); ok {
				style = append(style, "margin-left: "+indent)
			}
		}
	}
	if len(style) == 0 {
		return tag
	}
	end := len(tag)
	switch {
	case strings.HasSuffix(tag, "/>"):
		end -= 2
	case strings.HasSuffix(tag, ">"):
		end--
	}
	return tag[:end] + ` style="` + strings.Join(style, "; ") + `"` + tag[end:]
}

// fontSize returns the size of the font of the attribute size v of a font
// element.
func fontSize(v string) (string, bool) {
	n, err := strconv.Atoi(strings.TrimPrefix(v, "+"))
	if err != nil {
		return "", false
	}
	if strings.HasPrefix(v, "+") || strings.HasPrefix(v, "-") {
		n += 3
	}
	return fontSizes[min(max(n, 1), len(fontSizes))-1], true
}

// maxTagScan is the number of bytes that the conversion reads back from a
// position of a link to find the start of a tag.
const maxTagScan = 4096

// mobiHTML converts the text of a MOBI book into one HTML document, in
// UTF-8 with a byte order mark. Each target of a filepos link becomes an
// anchor, each image takes the name that res gives it, and each page break
// becomes an empty div element that breaks the page.
func mobiHTML(text []byte, h *header, res *resources, lim ivy.Limits) ([]byte, error) {
	text, err := addAnchors(text, lim)
	if err != nil {
		return nil, err
	}
	doc := rewriteTags(guide.ReplaceAllString(h.decode(text), ""), func(tag string) string {
		if pageBreak.MatchString(tag) {
			if strings.HasPrefix(tag, "</") {
				return ""
			}
			return `<div class="mbp-pagebreak"></div>`
		}
		if brEnd.MatchString(tag) {
			return ""
		}
		tag = mobi6Attrs(tag)
		tag = fileposRef.ReplaceAllStringFunc(tag, func(m string) string {
			pos := fileposRef.FindStringSubmatch(m)[1]
			return `href="#filepos` + pos + `"`
		})
		return recindexRef.ReplaceAllStringFunc(tag, func(m string) string {
			n, err := strconv.Atoi(recindexRef.FindStringSubmatch(m)[1])
			if err != nil {
				return m
			}
			name, ok := res.name(n)
			if !ok {
				return m
			}
			return `src="` + name + `"`
		})
	})
	return append([]byte("\uFEFF"), doc...), nil
}

// addAnchors adds an anchor at each position of the text that a filepos
// link names. The position counts bytes from the start of the text. A
// position inside a tag moves to the start of the tag.
func addAnchors(text []byte, lim ivy.Limits) ([]byte, error) {
	var targets []int
	seen := make(map[int]bool)
	for _, m := range fileposRef.FindAllSubmatch(text, -1) {
		pos, err := strconv.Atoi(string(m[1]))
		if err != nil || pos < 0 || pos > len(text) || seen[pos] {
			continue
		}
		if len(targets) >= lim.MaxObjects {
			return nil, fmt.Errorf("mobi text of more than %d link targets: %w", lim.MaxObjects, ivy.ErrLimit)
		}
		seen[pos] = true
		targets = append(targets, pos)
	}
	if len(targets) == 0 {
		return text, nil
	}
	slices.Sort(targets)
	var b bytes.Buffer
	b.Grow(len(text) + len(targets)*32)
	last := 0
	for _, pos := range targets {
		at := tagStart(text, pos)
		if at < last {
			at = last
		}
		b.Write(text[last:at])
		fmt.Fprintf(&b, `<a id="filepos%d"></a>`, pos)
		last = at
	}
	b.Write(text[last:])
	if int64(b.Len()) > lim.MaxMemory {
		return nil, fmt.Errorf("mobi text of more than %d bytes: %w", lim.MaxMemory, ivy.ErrLimit)
	}
	return b.Bytes(), nil
}

// tagStart returns pos, or the start of the tag that holds pos.
func tagStart(text []byte, pos int) int {
	lo := max(pos-maxTagScan, 0)
	open := bytes.LastIndexByte(text[lo:pos], '<')
	if open < 0 {
		return pos
	}
	open += lo
	if bytes.IndexByte(text[open:pos], '>') >= 0 {
		return pos
	}
	return open
}

// rewriteTags passes each start tag and end tag of doc through fn, and
// leaves the text, the comments and the declarations as they are. A tag
// ends at the first ">" outside of quotes.
func rewriteTags(doc string, fn func(tag string) string) string {
	var b strings.Builder
	b.Grow(len(doc) + len(doc)/16)
	i := 0
	for {
		lt := strings.IndexByte(doc[i:], '<')
		if lt < 0 {
			b.WriteString(doc[i:])
			return b.String()
		}
		lt += i
		b.WriteString(doc[i:lt])
		rest := doc[lt:]
		if strings.HasPrefix(rest, "<!--") {
			end := strings.Index(rest[4:], "-->")
			if end < 0 {
				b.WriteString(rest)
				return b.String()
			}
			b.WriteString(rest[:4+end+3])
			i = lt + 4 + end + 3
			continue
		}
		if len(rest) < 2 || !isTagStart(rest[1]) {
			b.WriteByte('<')
			i = lt + 1
			continue
		}
		end := tagEnd(rest)
		b.WriteString(fn(rest[:end]))
		i = lt + end
	}
}

// isTagStart reports whether c can follow "<" in a start tag or an end tag.
func isTagStart(c byte) bool {
	return c == '/' || c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z'
}

// tagEnd returns the index after the ">" that ends the tag at the start of
// s, or the length of s.
func tagEnd(s string) int {
	// prev is the last byte outside of quotes that is not a space
	var quote, prev byte
	for i := 1; i < len(s); i++ {
		c := s[i]
		if quote != 0 {
			if c == quote {
				quote, prev = 0, c
			}
			continue
		}
		switch c {
		case '"', '\'':
			// a quote starts only a value, after "="
			if prev == '=' {
				quote = c
				continue
			}
		case '>':
			return i + 1
		case '<':
			// a tag that does not end before the next one ends there
			return i
		case ' ', '\t', '\n', '\r', '\f':
			continue
		}
		prev = c
	}
	return len(s)
}

// palmDOCHTML converts the text of a PalmDOC book, which is plain text in
// windows-1252, into an HTML document.
func palmDOCHTML(text string) []byte {
	var b bytes.Buffer
	b.WriteString("\uFEFF<html><body><pre class=\"palmdoc\">")
	b.WriteString(html.EscapeString(text))
	b.WriteString("</pre></body></html>")
	return b.Bytes()
}
