package chm

import (
	"bytes"
	"strings"
	"unicode/utf8"

	"github.com/xo/ivy"
	"github.com/xo/ivy/internal/ebook"
)

// prescan is the number of bytes at the start of a document in which the
// parser of HTML looks for a meta element that gives the encoding, as the
// HTML standard says.
const prescan = 1024

// withCharset gives the encoding of an HTML document that does not declare
// one. A help file holds HTML in the code page of its language, often with
// no meta element that says which. The parser of HTML then reads it as
// windows-1252. So withCharset adds the element meta charset after the start
// tag of the head, in the first 1024 bytes, where the parser finds it. The
// encoding is UTF-8 if the document is valid UTF-8 with bytes past ASCII,
// and charset if not. A document with a byte order mark, with a declaration
// of its own, or in ASCII stays as it is, and so does a document in
// windows-1252, which the parser reads by default.
func withCharset(doc []byte, charset string) []byte {
	if bytes.HasPrefix(doc, []byte{0xef, 0xbb, 0xbf}) || bytes.HasPrefix(doc, []byte{0xff, 0xfe}) || bytes.HasPrefix(doc, []byte{0xfe, 0xff}) {
		return doc
	}
	ascii := true
	for _, c := range doc {
		if c >= 0x80 {
			ascii = false
			break
		}
	}
	if ascii {
		return doc
	}
	if utf8.Valid(doc) {
		charset = "utf-8"
	}
	if charset == "windows-1252" || declaresCharset(doc[:min(len(doc), prescan)]) {
		return doc
	}
	meta := `<meta charset="` + charset + `">`
	at := metaPlace(doc[:min(len(doc), prescan-len(meta))])
	out := make([]byte, 0, len(doc)+len(meta))
	out = append(out, doc[:at]...)
	out = append(out, meta...)
	return append(out, doc[at:]...)
}

// declaresCharset reports whether the start of a document has a meta
// element that gives its encoding: meta charset, or meta http-equiv with a
// content that holds charset=.
func declaresCharset(head []byte) bool {
	tree := ebook.ParseHTML(ebook.Windows1252(head), ivy.Limits{})
	if tree == nil {
		return false
	}
	for el := range ebook.Elements(tree.Root()) {
		if el.Name() != "meta" {
			continue
		}
		if _, ok := el.Attr("charset"); ok {
			return true
		}
		if content, ok := el.Attr("content"); ok && strings.Contains(strings.ToLower(content), "charset=") {
			return true
		}
	}
	return false
}

// metaPlace returns the place for the meta element in the start of a
// document: after the start tag of head, or else after the start tag of
// html, or else after the doctype, or else the start.
func metaPlace(head []byte) int {
	// only ASCII changes case, so that lower keeps the indexes of head
	lower := make([]byte, len(head))
	for i, c := range head {
		if c >= 'A' && c <= 'Z' {
			c += 'a' - 'A'
		}
		lower[i] = c
	}
	for _, name := range []string{"<head", "<html", "<!doctype"} {
		i := bytes.Index(lower, []byte(name))
		if i < 0 {
			continue
		}
		if next := i + len(name); next < len(lower) && !isNameEnd(lower[next]) {
			continue
		}
		if gt := bytes.IndexByte(lower[i:], '>'); gt >= 0 {
			return i + gt + 1
		}
	}
	return 0
}

// isNameEnd reports whether c ends the name of a tag.
func isNameEnd(c byte) bool {
	switch c {
	case '>', '/', ' ', '\t', '\r', '\n', '\f':
		return true
	}
	return false
}
