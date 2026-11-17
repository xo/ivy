package fb2

import (
	"context"
	"fmt"
	"html"
	"path"
	"strconv"
	"strings"

	"github.com/xo/ivy"
	"github.com/xo/ivy/internal/ebook"
	"github.com/xo/ivy/internal/layout"
	"github.com/xo/ivy/internal/memfs"
	"github.com/xo/ivy/internal/raster"
	"github.com/xo/okage/roxmltree"
)

// The paths of the documents of XHTML in the FS of the source: the book,
// and the page of the cover.
const (
	documentName = "index.xhtml"
	coverName    = "cover.xhtml"
)

// styleSheet is the style sheet of the format. It comes after
// ebook.BookStyle, which sets the page and the font, and it sets the style
// of each class of the XHTML document. A class has the name of the element
// of FictionBook that it comes from.
//
// The rules match the pages that calibre makes of a FictionBook file. They
// come from measurements of the PDF files of ebook-convert, not from the
// code of calibre, which is under the GPL (D7). An em is the size of the
// font of the text. These are the measurements:
//
//   - The text is justified. A paragraph has no margin, and its first line
//     has an indent of 2em.
//   - A title is a heading in a box that spans the column, with a border of
//     1px and a grey background. Its text is bold and on the left. The
//     title of the body and of a section at the first level is an h1 of
//     1.6em, at the second level an h2 of 1.3em, and at the third an h3 of
//     1.1em. The heading of a deeper level has the size of the text: bold
//     at the fourth level, bold and italic at the fifth, italic below. The
//     margins above and below the box are 11.55pt for an h1 and an h2 at a
//     font of 11.25pt, 1em of the heading for an h3, and 1.33em, 1.67em and
//     2.33em for the deeper ones.
//   - Each h1 and h2 starts a page.
//   - An epigraph is italic, with a left margin of 25% of the column. A
//     cite and a poem have a left margin of 4em and a right margin of 3px.
//     These blocks have a margin of 1em above and below.
//   - The author of a text is italic, 4em from the left of its block, with
//     a margin of 1em above it. The date of a poem is not italic, 0.75em
//     from the left.
//   - A subtitle is bold, italic and centered, in a box like the one of a
//     heading of the fourth level, with margins of 1.67em.
//   - The title of a poem is in a box like an h2. The title of a stanza is
//     a line of the text. An empty line follows each stanza.
//   - An empty-line is one line of the text.
//   - An image is centered at its size, with no margin.
//   - A note is a link in a superscript of 0.83em.
//   - A table has no border, and no margin.
//   - The table of contents is a list of links before the body, with a
//     line of 4px above and below.
const styleSheet = `body { text-align: justify; }
p { margin: 0; text-indent: 2em; }
h1, h2, h3, h4, h5, h6 {
  text-align: left;
  text-indent: 0;
  border: 1px solid #808080;
  background: #f4f4f4;
  padding: 0;
  page-break-after: avoid;
}
h1 { font-size: 1.6em; margin: 0.642em 0; border-color: #000; background: #e7e7e7; }
h2 { font-size: 1.3em; margin: 0.79em 0; background: #eee; }
h3 { font-size: 1.1em; line-height: 1.212; margin: 1em 0; border-color: #c0c0c0; background: #f1f1f1; }
h4 { font-size: 1em; margin: 1.33em 0; }
h5 { font-size: 1em; margin: 1.67em 0; font-style: italic; }
h6 { font-size: 1em; margin: 2.33em 0; font-weight: normal; font-style: italic; }
h1.title, h2.title { page-break-before: always; }
blockquote { margin: 1em 0; }
blockquote.epigraph { margin: 1em 0 1em 25%; font-style: italic; }
blockquote.cite { margin: 1em 3px 1em 4em; }
p.text-author { text-indent: 0; margin: 1em 0 0 4em; font-style: italic; text-align: left; }
p.date { text-indent: 0; padding-left: 0.75em; margin: 1em 0; }
p.subtitle {
  text-align: center;
  text-indent: 0;
  font-weight: bold;
  font-style: italic;
  margin: 1.67em 0;
  border: 1px solid #808080;
  background: #f4f4f4;
}
p.bodyname {
  text-align: center;
  text-indent: 0;
  font-weight: bold;
  margin: 1.33em 0;
  border: 1px solid #808080;
  background: #f4f4f4;
}
p.empty-line { text-indent: 0; }
div.poem { margin: 1em 3px 1em 4em; page-break-inside: avoid; }
div.poem-title {
  font-size: 1.3em;
  font-weight: bold;
  margin: 0.79em 0;
  border: 1px solid #808080;
  background: #eee;
  page-break-after: avoid;
}
div.poem-title p, div.stanza-title p, p.v { text-indent: 0; }
div.image { text-align: center; text-indent: 0; page-break-inside: avoid; }
a.note { vertical-align: super; font-size: 0.83em; }
table { border-collapse: separate; border-spacing: 2px; margin: 0; }
th, td { padding: 1px; text-indent: 0; }
ul.toc { margin: 1em 0; }
ul.toc li { text-indent: 0; }
hr {
  height: 2px;
  margin: 0.5em 0;
  border-style: solid;
  border-width: 1px;
  border-color: #9a9a9a #eee #eee #9a9a9a;
}
`

// annotationHeading is the heading that calibre gives the annotation of the
// book, in English whatever the language of the book.
const annotationHeading = "Annotation"

// tocEntry is an entry of the table of contents: the text of a title and
// the id of its heading, and the entries of the sections inside it. The
// text of an entry for a section without a title is "".
type tocEntry struct {
	text, id string
	children []*tocEntry
}

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
	// toc is the root of the table of contents. parent is the entry that
	// takes the entry of the next section, and pending is the entry that
	// takes the next title.
	toc     tocEntry
	parent  *tocEntry
	pending *tocEntry
	// nextID numbers the ids that the converter makes for the headings.
	nextID int
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
		c.annotation(d.annotation.node)
	}
	// the table of contents goes here, once the bodies give its entries
	tocAt := c.b.Len()
	c.parent = &c.toc
	first := true
	for n := range d.root.Children() {
		if name(n) == "body" {
			c.body(n, first)
			first = false
		}
		if c.err != nil {
			return layout.Source{}, c.err
		}
	}
	c.b.WriteString("</body>\n</html>\n")
	if err := c.check(); err != nil {
		return layout.Source{}, err
	}
	doc := c.b.String()
	if toc := c.tocHTML(); toc != "" {
		if int64(len(doc)+len(toc)) > d.lim.MaxMemory {
			return layout.Source{}, fmt.Errorf("xhtml of more than %d bytes: %w", d.lim.MaxMemory, ivy.ErrLimit)
		}
		doc = doc[:tocAt] + toc + doc[tocAt:]
	}
	if c.deep {
		d.log.Warn("fictionbook nests deeper than the limit, content left out", "limit", d.lim.MaxDepth)
	}
	c.fs.Add(documentName, []byte(doc))
	docs := []string{documentName}
	if cover := c.coverPath(); cover != "" {
		// the book starts with a page of the cover, as calibre shows it
		c.fs.Add(coverName, ebook.ImagePage(ebook.ImagePageLink(coverName, cover), d.info.Title, d.paper))
		docs = []string{coverName, documentName}
	}
	return layout.Source{
		FS:          &c.fs,
		Documents:   docs,
		StyleSheets: []string{ebook.BookStyleFor(d.paper), styleSheet},
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

// annotation converts the annotation of the title-info. As calibre does,
// it has a heading, and a line follows it.
func (c *converter) annotation(n roxmltree.Node) {
	c.b.WriteString(`<div class="annotation info"`)
	c.id(n)
	c.b.WriteString(">\n<h1>" + annotationHeading + "</h1>\n")
	c.blocks(n, 0, 1)
	c.b.WriteString("</div>\n<hr/>\n")
}

// body converts a body element. The first body is the main text. A later
// body, such as the body of the notes, follows it. As in calibre, a later
// body with a name starts with a line and its name, and the table of
// contents lists it by that name.
func (c *converter) body(n roxmltree.Node, first bool) {
	bodyName := strings.TrimSpace(ebook.Attr(n, "name"))
	class := "body"
	if bodyName == "notes" || bodyName == "comments" {
		class = "body notes"
	}
	c.parent = &c.toc
	c.pending = nil
	switch {
	case !first && bodyName != "":
		id := c.newID()
		c.b.WriteString(`<hr/>` + "\n" + `<p class="bodyname" id="` + id + `">`)
		c.escape(bodyName, false)
		c.b.WriteString("</p>\n")
		c.parent = c.addEntry(bodyName, id)
	case hasTitle(n):
		c.parent = c.addEntry("", "")
		c.pending = c.parent
	case !first && len(c.toc.children) > 0:
		// calibre lists the sections of a later body without a title or a
		// name under the entry before them
		c.parent = c.toc.children[len(c.toc.children)-1]
	}
	c.b.WriteString(`<div class="` + class + `"`)
	c.id(n)
	c.b.WriteString(">\n")
	c.blocks(n, 0, 1)
	c.b.WriteString("</div>\n")
}

// hasTitle reports whether n has a title element.
func hasTitle(n roxmltree.Node) bool {
	for child := range n.Children() {
		if name(child) == "title" {
			return true
		}
	}
	return false
}

// addEntry adds an entry of the table of contents to the current parent,
// and returns it.
func (c *converter) addEntry(text, id string) *tocEntry {
	e := &tocEntry{text: text, id: id}
	c.parent.children = append(c.parent.children, e)
	return e
}

// newID returns an id that no element of the document has, for a heading
// that the table of contents links to.
func (c *converter) newID() string {
	for {
		c.nextID++
		id := "toc-" + strconv.Itoa(c.nextID)
		if !c.ids[id] {
			c.ids[id] = true
			return id
		}
	}
}

// tocHTML returns the table of contents as a list of links, with a line
// after it, or "" if no entry has a text. An entry without a text shows
// only its bullet, as in calibre.
func (c *converter) tocHTML() string {
	if !hasText(&c.toc) {
		return ""
	}
	var b strings.Builder
	var list func(entries []*tocEntry, top bool)
	list = func(entries []*tocEntry, top bool) {
		if top {
			b.WriteString(`<ul class="toc">` + "\n")
		} else {
			b.WriteString("<ul>\n")
		}
		for _, e := range entries {
			b.WriteString("<li>")
			switch {
			case e.text != "" && e.id != "":
				b.WriteString(`<a href="#` + html.EscapeString(e.id) + `">` + e.text + "</a>")
			case e.text != "":
				b.WriteString(e.text)
			case len(e.children) == 0:
				b.WriteString("&#160;")
			}
			if len(e.children) > 0 {
				b.WriteString("\n")
				list(e.children, false)
			}
			b.WriteString("</li>\n")
		}
		b.WriteString("</ul>\n")
	}
	list(c.toc.children, true)
	b.WriteString("<hr/>\n")
	return b.String()
}

// hasText reports whether e or an entry inside it has a text.
func hasText(e *tocEntry) bool {
	for _, child := range e.children {
		if child.text != "" || hasText(child) {
			return true
		}
	}
	return false
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

// emptyLine is the paragraph of an empty-line: a line of the text. Its
// space does not break, so that the line keeps its height.
const emptyLine = "<p class=\"empty-line\">&#160;</p>\n"

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
		switch tag {
		case "section":
			level++
			parent := c.parent
			c.parent = c.addEntry("", "")
			c.pending = c.parent
			c.blocks(n, level, depth+1)
			if c.pending == c.parent {
				c.pending = nil
			}
			c.parent = parent
		case "stanza":
			c.blocks(n, level, depth+1)
			// calibre leaves an empty line after each stanza
			c.b.WriteString(emptyLine)
		default:
			c.blocks(n, level, depth+1)
		}
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
// level of the section: an h1 for the title of the body and of a section
// at the first level, an h2 at the second level, and so on. In a poem, it
// is a block in a box, and in a stanza, a line of the text. The first
// title of a section gives the text of its entry in the table of contents:
// the text of its first paragraph.
func (c *converter) title(n roxmltree.Node, level, depth int) {
	parent, _ := n.ParentElement()
	if p := name(parent); p == "poem" || p == "stanza" {
		c.b.WriteString(`<div class="` + p + `-title">` + "\n")
		c.blocks(n, level, depth+1)
		c.b.WriteString("</div>\n")
		return
	}
	h := "h" + strconv.Itoa(min(max(level, 1), 6))
	c.b.WriteString("<" + h + ` class="title"`)
	id := strings.TrimSpace(ebook.Attr(n, "id"))
	if id == "" || c.ids[id] {
		id = ""
	}
	if e := c.pending; e != nil {
		c.pending = nil
		if id == "" {
			id = c.newID()
			c.b.WriteString(` id="` + id + `"`)
		} else {
			c.id(n)
		}
		e.id = id
		for child := range n.Children() {
			if name(child) == "p" {
				e.text = c.inlineText(child)
				break
			}
		}
	} else {
		c.id(n)
	}
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

// inlineText returns the text of n, with each run of white space as one
// space, escaped for XHTML.
func (c *converter) inlineText(n roxmltree.Node) string {
	var b strings.Builder
	for _, r := range text(n) {
		switch r {
		case '&':
			b.WriteString("&amp;")
		case '<':
			b.WriteString("&lt;")
		case '>':
			b.WriteString("&gt;")
		default:
			if r >= 0x20 && r != 0xFFFE && r != 0xFFFF {
				b.WriteRune(r)
			}
		}
	}
	return b.String()
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

// coverPath returns the path of the image of the coverpage in the FS of
// the source, or "" if the book has no cover image that decodes.
func (c *converter) coverPath() string {
	if c.d.cover == "" {
		return ""
	}
	p := c.binaryPath(c.d.cover)
	if p == "" || path.Ext(p) == ".svg" {
		return p
	}
	buf, err := c.fs.ReadFile(p)
	if err == nil {
		_, err = raster.Config(buf)
	}
	if err != nil {
		c.d.log.Warn("fictionbook cover left out", "id", c.d.cover, "err", err)
		return ""
	}
	return p
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
