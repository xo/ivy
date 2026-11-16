package ebook

import (
	"html"
	"iter"
	"strings"
)

// Tag is a start tag that Tags finds.
type Tag struct {
	// Name is the name of the element in lower case, with its prefix, such
	// as "img" or "svg:image".
	Name  string
	attrs []tagAttr
	// Text is the raw text of an element whose content is text, such as
	// title, script and style.
	Text string
}

// tagAttr is an attribute of a tag.
type tagAttr struct {
	name, value string
}

// Attr returns the value of the attribute name of t, with its entities
// decoded. The name is in lower case, with its prefix, such as
// "xlink:href".
func (t Tag) Attr(name string) (string, bool) {
	for _, a := range t.attrs {
		if a.name == name {
			return a.value, true
		}
	}
	return "", false
}

// rawText are the elements whose content is text and not tags.
var rawText = map[string]bool{
	"title": true, "script": true, "style": true, "textarea": true, "xmp": true,
}

// Tags iterates over the start tags of the document doc, with a light scan
// of its bytes. It skips comments, declarations, end tags and the content
// of the elements whose content is text, such as script. It is not a parser
// of HTML, and it does not build a tree. Its time is linear in the length
// of doc.
func Tags(doc string) iter.Seq[Tag] {
	return func(yield func(Tag) bool) {
		lower := asciiLower(doc)
		i := 0
		for {
			lt := strings.IndexByte(doc[i:], '<')
			if lt < 0 {
				return
			}
			i += lt
			rest := doc[i:]
			switch {
			case strings.HasPrefix(rest, "<!--"):
				i = skipPast(doc, i+4, "-->")
				continue
			case strings.HasPrefix(rest, "<!"), strings.HasPrefix(rest, "<?"), strings.HasPrefix(rest, "</"):
				i = skipPast(doc, i+2, ">")
				continue
			case len(rest) < 2 || !isLetter(rest[1]):
				i++
				continue
			}
			tag, end := scanTag(doc, lower, i+1)
			i = end
			if rawText[tag.Name] {
				closing := strings.Index(lower[i:], "</"+tag.Name)
				if closing < 0 {
					closing = len(doc) - i
				}
				tag.Text = doc[i : i+closing]
				i += closing
			}
			if !yield(tag) {
				return
			}
		}
	}
}

// skipPast returns the index after the first end at or after i, or the
// length of doc.
func skipPast(doc string, i int, end string) int {
	j := strings.Index(doc[i:], end)
	if j < 0 {
		return len(doc)
	}
	return i + j + len(end)
}

// scanTag reads the tag whose name starts at i, and returns the index after
// its end.
func scanTag(doc, lower string, i int) (Tag, int) {
	start := i
	for i < len(doc) && isNameByte(doc[i]) {
		i++
	}
	tag := Tag{Name: lower[start:i]}
	for i < len(doc) {
		switch c := doc[i]; {
		case c == '>':
			return tag, i + 1
		case c == '/' || isSpace(c):
			i++
			continue
		}
		nameStart := i
		for i < len(doc) && !isSpace(doc[i]) && doc[i] != '=' && doc[i] != '>' && doc[i] != '/' {
			i++
		}
		name := lower[nameStart:i]
		if name == "" {
			// a stray "=" is skipped
			i++
			continue
		}
		for i < len(doc) && isSpace(doc[i]) {
			i++
		}
		var value string
		if i < len(doc) && doc[i] == '=' {
			i++
			for i < len(doc) && isSpace(doc[i]) {
				i++
			}
			switch {
			case i < len(doc) && (doc[i] == '"' || doc[i] == '\''):
				q := doc[i]
				end := strings.IndexByte(doc[i+1:], q)
				if end < 0 {
					end = len(doc) - i - 1
				}
				value = doc[i+1 : i+1+end]
				i = min(i+2+end, len(doc))
			default:
				valueStart := i
				for i < len(doc) && !isSpace(doc[i]) && doc[i] != '>' {
					i++
				}
				value = doc[valueStart:i]
			}
		}
		tag.attrs = append(tag.attrs, tagAttr{name: name, value: html.UnescapeString(value)})
	}
	return tag, i
}

// asciiLower returns s with the ASCII letters in lower case. Each byte keeps
// its index.
func asciiLower(s string) string {
	b := []byte(s)
	for i, c := range b {
		if 'A' <= c && c <= 'Z' {
			b[i] = c + 'a' - 'A'
		}
	}
	return string(b)
}

func isLetter(c byte) bool {
	return 'a' <= c && c <= 'z' || 'A' <= c && c <= 'Z'
}

func isNameByte(c byte) bool {
	return isLetter(c) || '0' <= c && c <= '9' || c == ':' || c == '-' || c == '_' || c == '.'
}

func isSpace(c byte) bool {
	return c == ' ' || c == '\t' || c == '\n' || c == '\r' || c == '\f'
}

// Head is what the head of an HTML document says about the document.
type Head struct {
	Title    string
	Authors  []string
	Subject  string
	Language string
	// Generator is the program that made the document.
	Generator string
}

// ScanHead reads the title, the language and the meta elements of the
// document doc, until its body. Each value has its runs of white space
// collapsed to one space.
func ScanHead(doc string) Head {
	var h Head
	for tag := range Tags(doc) {
		switch tag.Name {
		case "html":
			if lang, ok := tag.Attr("lang"); ok {
				h.Language = collapse(lang)
			} else if lang, ok := tag.Attr("xml:lang"); ok {
				h.Language = collapse(lang)
			}
		case "title":
			if h.Title == "" {
				h.Title = collapse(html.UnescapeString(tag.Text))
			}
		case "meta":
			name, _ := tag.Attr("name")
			content, _ := tag.Attr("content")
			content = collapse(content)
			if content == "" {
				continue
			}
			switch strings.ToLower(strings.TrimSpace(name)) {
			case "author", "dc.creator", "dcterms.creator":
				h.Authors = append(h.Authors, content)
			case "subject", "dc.subject", "keywords":
				if h.Subject == "" {
					h.Subject = content
				}
			case "generator":
				h.Generator = content
			}
		case "body":
			return h
		}
	}
	return h
}

// FirstImage returns the link of the first image of the document doc: the
// src of an img element, or the href of an image element of SVG.
func FirstImage(doc string) (string, bool) {
	for tag := range Tags(doc) {
		switch tag.Name {
		case "img":
			if src, ok := tag.Attr("src"); ok && strings.TrimSpace(src) != "" {
				return src, true
			}
		case "image", "svg:image":
			for _, name := range []string{"xlink:href", "href"} {
				if href, ok := tag.Attr(name); ok && strings.TrimSpace(href) != "" {
					return href, true
				}
			}
		}
	}
	return "", false
}

// collapse trims s, and collapses each run of white space to one space.
func collapse(s string) string {
	return strings.Join(strings.Fields(s), " ")
}
