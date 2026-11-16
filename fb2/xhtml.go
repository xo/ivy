package fb2

import (
	"context"
	"fmt"
	"path"
	"strconv"
	"strings"

	"github.com/xo/ivy"
	"github.com/xo/ivy/internal/ebook"
	"github.com/xo/ivy/internal/layout"
	"github.com/xo/ivy/internal/memfs"
	"github.com/xo/okage/roxmltree"
)

// documentName is the path of the XHTML document in the FS of the source.
const documentName = "index.xhtml"

// styleSheet is the style sheet of the format. It comes after
// ebook.BookStyle, which sets the page and the font, and it sets the style
// of each class of the XHTML document. A class has the name of the element
// of FictionBook that it comes from.
const styleSheet = `body { text-align: justify; }
p { margin: 0; text-indent: 1.5em; }
h1, h2, h3, h4, h5, h6 { text-align: center; text-indent: 0; margin: 1em 0 0.5em; page-break-after: avoid; }
h1 { font-size: 1.6em; }
h2 { font-size: 1.4em; }
h3 { font-size: 1.2em; }
h4, h5, h6 { font-size: 1em; }
div.body + div.body { page-break-before: always; }
div.annotation { margin: 1em 2em; font-style: italic; }
blockquote { margin: 1em 0 1em 2em; }
blockquote.epigraph { margin-left: 30%; font-style: italic; }
p.text-author, p.date { text-align: right; text-indent: 0; font-style: italic; }
p.subtitle { text-align: center; text-indent: 0; font-weight: bold; margin: 1em 0 0.5em; }
p.empty-line { text-indent: 0; }
div.poem { margin: 1em 0 1em 15%; text-align: left; }
div.poem-title { font-weight: bold; margin-bottom: 0.5em; }
div.stanza { margin: 0.5em 0; }
p.v { text-indent: 0; text-align: left; }
div.image { text-align: center; text-indent: 0; margin: 1em 0; page-break-inside: avoid; }
img { max-width: 100%; }
a.note { vertical-align: super; font-size: 0.75em; line-height: 0; }
div.notes { font-size: 0.9em; }
table { border-collapse: collapse; margin: 1em auto; }
th, td { border: 1px solid; padding: 0.2em 0.4em; text-indent: 0; }
code { font-family: monospace; }
`

// converter converts the body of a book into XHTML.
type converter struct {
	d *Document
	// done returns the error of the context of the conversion.
	done func() error
	b    strings.Builder
	fs   memfs.FS
	// images maps the id of a binary to its path in fs, or to "" if the
	// binary does not decode.
	images map[string]string
	// ids holds the ids that the document has, so that each one is unique.
	ids map[string]bool
	// deep is true after the converter left out content past the limit of
	// depth.
	deep bool
	err  error
}

// convert converts the book into a layout.Source.
func (d *Document) convert(ctx context.Context) (layout.Source, error) {
	c := &converter{d: d, done: ctx.Err, images: make(map[string]string), ids: make(map[string]bool)}
	c.b.WriteString(`<?xml version="1.0" encoding="UTF-8"?>` + "\n")
	c.b.WriteString(`<html xmlns="http://www.w3.org/1999/xhtml"`)
	if lang := d.info.Language; lang != "" {
		c.b.WriteString(` lang="`)
		c.escape(lang, true)
		c.b.WriteString(`" xml:lang="`)
		c.escape(lang, true)
		c.b.WriteString(`"`)
	}
	c.b.WriteString(">\n<head>\n<meta charset=\"UTF-8\"/>\n<title>")
	c.escape(d.info.Title, false)
	c.b.WriteString("</title>\n</head>\n<body>\n")
	if d.annotation.ok {
		c.element(d.annotation.node, 0, 0)
	}
	for n := range d.root.Children() {
		if name(n) == "body" {
			c.body(n)
		}
		if c.err != nil {
			return layout.Source{}, c.err
		}
	}
	c.b.WriteString("</body>\n</html>\n")
	if err := c.check(); err != nil {
		return layout.Source{}, err
	}
	if c.deep {
		d.log.Warn("fictionbook nests deeper than the limit, content left out", "limit", d.lim.MaxDepth)
	}
	c.fs.Add(documentName, []byte(c.b.String()))
	return layout.Source{
		FS:          &c.fs,
		Documents:   []string{documentName},
		StyleSheets: []string{ebook.BookStyle, styleSheet},
		Title:       d.info.Title,
	}, nil
}

// check records an error if the context ends or the document grows past
// the limit of memory, and returns the first error that the converter met.
func (c *converter) check() error {
	if c.err != nil {
		return c.err
	}
	if err := c.done(); err != nil {
		c.err = fmt.Errorf("converting fictionbook: %w", err)
	} else if int64(c.b.Len()) > c.d.lim.MaxMemory {
		c.err = fmt.Errorf("xhtml of more than %d bytes: %w", c.d.lim.MaxMemory, ivy.ErrLimit)
	}
	return c.err
}

// body converts a body element. The first body is the main text. A later
// body, such as the body of the notes, follows it.
func (c *converter) body(n roxmltree.Node) {
	class := "body"
	if bodyName := ebook.Attr(n, "name"); bodyName == "notes" || bodyName == "comments" {
		class = "body notes"
	}
	c.b.WriteString(`<div class="` + class + `"`)
	c.id(n)
	c.b.WriteString(">\n")
	c.blocks(n, 0, 1)
	c.b.WriteString("</div>\n")
}

// blocks converts the children of n as blocks. level is the level of the
// sections, for the headings of the titles, and depth is the depth of
// nesting.
func (c *converter) blocks(n roxmltree.Node, level, depth int) {
	for child := range n.Children() {
		if c.check() != nil {
			return
		}
		if child.IsElement() {
			c.element(child, level, depth)
		}
	}
}

// blockTags maps an element of FictionBook that holds blocks to its XHTML
// element and its class.
var blockTags = map[string][2]string{
	"section":    {"div", "section"},
	"annotation": {"div", "annotation"},
	"epigraph":   {"blockquote", "epigraph"},
	"cite":       {"blockquote", "cite"},
	"poem":       {"div", "poem"},
	"stanza":     {"div", "stanza"},
}

// paragraphClasses maps an element of FictionBook that holds inline content
// to the class of its XHTML paragraph.
var paragraphClasses = map[string]string{
	"p":           "",
	"subtitle":    "subtitle",
	"v":           "v",
	"text-author": "text-author",
	"date":        "date",
}

// element converts the block element n.
func (c *converter) element(n roxmltree.Node, level, depth int) {
	if depth > c.d.lim.MaxDepth {
		c.deep = true
		return
	}
	tag := name(n)
	if t, ok := blockTags[tag]; ok {
		c.b.WriteString("<" + t[0] + ` class="` + t[1] + `"`)
		c.id(n)
		c.b.WriteString(">\n")
		if tag == "section" {
			level++
		}
		c.blocks(n, level, depth+1)
		c.b.WriteString("</" + t[0] + ">\n")
		return
	}
	if class, ok := paragraphClasses[tag]; ok {
		c.b.WriteString("<p")
		if class != "" {
			c.b.WriteString(` class="` + class + `"`)
		}
		c.id(n)
		c.b.WriteString(">")
		c.inlines(n, depth+1)
		c.b.WriteString("</p>\n")
		return
	}
	switch tag {
	case "title":
		c.title(n, level, depth)
	case "empty-line":
		c.b.WriteString("<p class=\"empty-line\"> </p>\n")
	case "image":
		c.b.WriteString(`<div class="image"`)
		c.id(n)
		c.b.WriteString(">")
		c.image(n)
		c.b.WriteString("</div>\n")
	case "table":
		c.table(n, depth)
	default:
		// an element that FictionBook does not define here keeps its text
		c.blocks(n, level, depth+1)
	}
}

// title converts a title. In a body or a section, it is a heading of the
// level of the section. In a poem or a stanza, it is a block in bold.
func (c *converter) title(n roxmltree.Node, level, depth int) {
	parent, _ := n.ParentElement()
	if p := name(parent); p == "poem" || p == "stanza" {
		c.b.WriteString(`<div class="poem-title">` + "\n")
		c.blocks(n, level, depth+1)
		c.b.WriteString("</div>\n")
		return
	}
	h := "h" + strconv.Itoa(min(level+1, 6))
	c.b.WriteString("<" + h + ` class="title"`)
	c.id(n)
	c.b.WriteString(">")
	first := true
	for child := range n.Children() {
		switch name(child) {
		case "p":
			if !first {
				c.b.WriteString("<br/>")
			}
			c.inlines(child, depth+1)
			first = false
		case "empty-line":
			c.b.WriteString("<br/>")
		}
	}
	c.b.WriteString("</" + h + ">\n")
}

// table converts a table. The cells keep their spans and their alignment.
func (c *converter) table(n roxmltree.Node, depth int) {
	c.b.WriteString("<table")
	c.id(n)
	c.b.WriteString(">\n")
	for tr := range n.Children() {
		if name(tr) != "tr" {
			continue
		}
		c.b.WriteString("<tr")
		c.align(tr)
		c.b.WriteString(">")
		for cell := range tr.Children() {
			tag := name(cell)
			if tag != "th" && tag != "td" {
				continue
			}
			c.b.WriteString("<" + tag)
			c.id(cell)
			for _, attr := range []string{"colspan", "rowspan"} {
				v := ebook.Attr(cell, attr)
				if span, err := strconv.Atoi(strings.TrimSpace(v)); err == nil && span > 1 && span <= 1000 {
					c.b.WriteString(" " + attr + `="` + strconv.Itoa(span) + `"`)
				}
			}
			c.align(cell)
			c.b.WriteString(">")
			c.inlines(cell, depth+1)
			c.b.WriteString("</" + tag + ">")
		}
		c.b.WriteString("</tr>\n")
	}
	c.b.WriteString("</table>\n")
}

// align writes the attributes align and valign of n as a style.
func (c *converter) align(n roxmltree.Node) {
	var style []string
	align := ebook.Attr(n, "align")
	switch align = strings.TrimSpace(align); align {
	case "left", "right", "center":
		style = append(style, "text-align: "+align)
	}
	valign := ebook.Attr(n, "valign")
	switch valign = strings.TrimSpace(valign); valign {
	case "top", "middle", "bottom":
		style = append(style, "vertical-align: "+valign)
	}
	if len(style) > 0 {
		c.b.WriteString(` style="` + strings.Join(style, "; ") + `"`)
	}
}

// inlineTags maps an inline element of FictionBook to its XHTML element.
var inlineTags = map[string]string{
	"strong":        "strong",
	"emphasis":      "em",
	"strikethrough": "s",
	"sub":           "sub",
	"sup":           "sup",
	"code":          "code",
	"style":         "span",
}

// inlines converts the children of n as inline content.
func (c *converter) inlines(n roxmltree.Node, depth int) {
	if depth > c.d.lim.MaxDepth {
		c.deep = true
		return
	}
	for child := range n.Children() {
		if c.check() != nil {
			return
		}
		if child.IsText() {
			s, _ := child.Text()
			c.escape(s, false)
			continue
		}
		tag := name(child)
		if t, ok := inlineTags[tag]; ok {
			c.b.WriteString("<" + t + ">")
			c.inlines(child, depth+1)
			c.b.WriteString("</" + t + ">")
			continue
		}
		switch tag {
		case "a":
			c.link(child, depth)
		case "image":
			c.image(child)
		default:
			c.inlines(child, depth+1)
		}
	}
}

// link converts a link. A link inside the book points to an id. A link of
// the type note points to a note, and it is set in superscript. A link out
// of the book keeps its URL if the scheme is http, https, ftp or mailto.
func (c *converter) link(n roxmltree.Node, depth int) {
	target := href(n)
	ok := strings.HasPrefix(target, "#") && len(target) > 1
	if !ok {
		if i := strings.IndexByte(target, ':'); i > 0 {
			switch strings.ToLower(target[:i]) {
			case "http", "https", "ftp", "mailto":
				ok = true
			}
		}
	}
	if !ok {
		c.b.WriteString("<span>")
		c.inlines(n, depth+1)
		c.b.WriteString("</span>")
		return
	}
	c.b.WriteString("<a")
	if typ := ebook.Attr(n, "type"); typ == "note" {
		c.b.WriteString(` class="note"`)
	}
	c.b.WriteString(` href="`)
	c.escape(target, true)
	c.b.WriteString(`">`)
	c.inlines(n, depth+1)
	c.b.WriteString("</a>")
}

// image writes an img element for the image n. An image whose binary is
// missing or does not decode is left out, with its alternate text.
func (c *converter) image(n roxmltree.Node) {
	alt := ebook.Attr(n, "alt")
	id, ok := strings.CutPrefix(href(n), "#")
	src := ""
	if ok {
		src = c.binaryPath(id)
	}
	if src == "" {
		if alt != "" {
			c.b.WriteString("<span>")
			c.escape(alt, false)
			c.b.WriteString("</span>")
		}
		return
	}
	c.b.WriteString(`<img src="`)
	c.escape(src, true)
	c.b.WriteString(`" alt="`)
	c.escape(alt, true)
	c.b.WriteString(`"/>`)
}

// binaryPath decodes the binary id into the FS of the source, once, and
// returns its path. It returns "" if the binary is missing or does not
// decode.
func (c *converter) binaryPath(id string) string {
	if p, ok := c.images[id]; ok {
		return p
	}
	p := ""
	buf, err := c.d.binary(id)
	if err == nil {
		p = "images/" + strconv.Itoa(len(c.images)) + imageExt(c.d.binaries[id].contentType, id)
		c.fs.Add(p, buf)
	} else {
		c.d.log.Warn("fictionbook image left out", "id", id, "err", err)
	}
	c.images[id] = p
	return p
}

// imageExt returns the extension of the file of an image, from its content
// type, or else from its id.
func imageExt(contentType, id string) string {
	switch strings.ToLower(strings.TrimSpace(contentType)) {
	case "image/jpeg", "image/jpg":
		return ".jpg"
	case "image/png":
		return ".png"
	case "image/gif":
		return ".gif"
	case "image/svg+xml":
		return ".svg"
	case "image/webp":
		return ".webp"
	}
	ext := strings.ToLower(path.Ext(id))
	for _, r := range ext[min(1, len(ext)):] {
		if r < 'a' || r > 'z' {
			return ".bin"
		}
	}
	if len(ext) < 2 || len(ext) > 5 {
		return ".bin"
	}
	return ext
}

// id writes the attribute id of n, if n has one that no earlier element
// has.
func (c *converter) id(n roxmltree.Node) {
	id := ebook.Attr(n, "id")
	if id = strings.TrimSpace(id); id == "" || c.ids[id] {
		return
	}
	c.ids[id] = true
	c.b.WriteString(` id="`)
	c.escape(id, true)
	c.b.WriteString(`"`)
}

// escape writes s with the characters that XML reserves escaped. In an
// attribute, it also escapes the quote and the white space that the value of
// an attribute would lose.
func (c *converter) escape(s string, attr bool) {
	for _, r := range s {
		switch {
		case r == '&':
			c.b.WriteString("&amp;")
		case r == '<':
			c.b.WriteString("&lt;")
		case r == '>':
			c.b.WriteString("&gt;")
		case attr && r == '"':
			c.b.WriteString("&quot;")
		case attr && (r == '\n' || r == '\r' || r == '\t'):
			c.b.WriteString("&#" + strconv.Itoa(int(r)) + ";")
		case r < 0x20 && r != '\n' && r != '\r' && r != '\t', r == 0xFFFE, r == 0xFFFF:
			// a character that XML does not allow is left out
		default:
			c.b.WriteRune(r)
		}
	}
}
