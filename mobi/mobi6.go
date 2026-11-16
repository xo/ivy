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
)

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
	doc := rewriteTags(h.decode(text), func(tag string) string {
		if pageBreak.MatchString(tag) {
			if strings.HasPrefix(tag, "</") {
				return ""
			}
			return `<div class="mbp-pagebreak"></div>`
		}
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
