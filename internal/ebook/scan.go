package ebook

import (
	"iter"
	"strings"
	"unicode"

	okhtml "github.com/xo/okage/html"

	"github.com/xo/ivy"
)

// ParseHTML parses the document doc, which is UTF-8 text, with the HTML
// parser of okage. The parse stops at lim.MaxObjects nodes or at
// lim.MaxDepth open elements, as at the end of doc. It also parses a
// document of XHTML or of SVG, as a browser parses one that a server sends
// as HTML.
func ParseHTML(doc string, lim ivy.Limits) *okhtml.Document {
	lim = lim.WithDefaults()
	tree, err := okhtml.ParseDocument(strings.NewReader(doc),
		okhtml.WithCharset("utf-8"),
		okhtml.WithMaxNodes(lim.MaxObjects),
		okhtml.WithMaxDepth(lim.MaxDepth))
	if err != nil {
		// ParseDocument fails only when its reader fails, and a
		// strings.Reader does not fail
		return nil
	}
	return tree
}

// Elements iterates over the elements under the node n, in the order of
// the document. It does not yield n.
func Elements(n *okhtml.Node) iter.Seq[*okhtml.Node] {
	return func(yield func(*okhtml.Node) bool) {
		if n == nil {
			return
		}
		c := n.FirstChild()
		for c != nil {
			if c.Type() == okhtml.ElementNode && !yield(c) {
				return
			}
			if first := c.FirstChild(); first != nil {
				c = first
				continue
			}
			for c != n && c.NextSibling() == nil {
				c = c.Parent()
			}
			if c == n {
				return
			}
			c = c.NextSibling()
		}
	}
}

// textOf returns the text of the children of the node n that are text.
func textOf(n *okhtml.Node) string {
	var b strings.Builder
	for c := range n.Children() {
		if c.Type() == okhtml.TextNode {
			b.WriteString(c.Data())
		}
	}
	return b.String()
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

// ScanHead reads the title, the language and the meta elements of the head
// of the document doc. Each value has its runs of white space collapsed to
// one space.
func ScanHead(doc string, lim ivy.Limits) Head {
	var h Head
	tree := ParseHTML(doc, lim)
	if tree == nil {
		return h
	}
	root := tree.DocumentElement()
	if root == nil {
		return h
	}
	if lang, ok := root.Attr("lang"); ok {
		h.Language = collapse(lang)
	} else if lang, ok := root.Attr("xml:lang"); ok {
		h.Language = collapse(lang)
	}
	var head *okhtml.Node
	for c := range root.Children() {
		if c.Type() == okhtml.ElementNode && c.Name() == "head" {
			head = c
			break
		}
	}
	for el := range Elements(head) {
		switch el.Name() {
		case "title":
			if h.Title == "" {
				h.Title = collapse(textOf(el))
			}
		case "meta":
			name, _ := el.Attr("name")
			content, _ := el.Attr("content")
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
		}
	}
	return h
}

// FirstImage returns the link of the first image of the document doc: the
// src of an img element, or the href of an image element of SVG.
func FirstImage(doc string, lim ivy.Limits) (string, bool) {
	img, ok := scanImage(ParseHTML(doc, lim))
	return img.link, ok
}

// pageImage is the first image of a document.
type pageImage struct {
	// link is the link of the image, as the document gives it.
	link string
	// path is the path of the image in the file system.
	path string
	// before counts the letters and digits of the text before the image,
	// and letters counts those of the whole text.
	before, letters int
}

// skipText are the elements whose text a page does not show.
var skipText = map[string]bool{
	"head": true, "script": true, "style": true, "title": true, "template": true, "noscript": true,
}

// scanImage returns the first image of the document tree, and counts the
// letters and digits of its text.
func scanImage(tree *okhtml.Document) (pageImage, bool) {
	var img pageImage
	if tree == nil {
		return img, false
	}
	found := false
	root := tree.Root()
	c := root.FirstChild()
	for c != nil {
		descend := false
		switch c.Type() {
		case okhtml.TextNode:
			for _, r := range c.Data() {
				if unicode.IsLetter(r) || unicode.IsNumber(r) {
					img.letters++
				}
			}
		case okhtml.ElementNode:
			descend = !skipText[c.Name()]
			if !found && descend {
				if link, ok := imageLink(c); ok {
					img.link, img.before, found = link, img.letters, true
				}
			}
		}
		if first := c.FirstChild(); descend && first != nil {
			c = first
			continue
		}
		for c != root && c.NextSibling() == nil {
			c = c.Parent()
		}
		if c == root {
			break
		}
		c = c.NextSibling()
	}
	return img, found
}

// imageLink returns the link of an img element of HTML, or of an image
// element of SVG. An image element of SVG with a prefix, such as svg:image,
// is an element of HTML when the document is read as HTML.
func imageLink(n *okhtml.Node) (string, bool) {
	var names []string
	switch {
	case n.Namespace() == okhtml.NamespaceSVG:
		if href, ok := n.AttrNS(okhtml.NamespaceXLink, "href"); ok && strings.TrimSpace(href) != "" {
			return href, true
		}
		names = []string{"href"}
	case n.Name() == "img":
		names = []string{"src"}
	case n.Name() == "svg:image":
		names = []string{"xlink:href", "href"}
	}
	for _, name := range names {
		if v, ok := n.Attr(name); ok && strings.TrimSpace(v) != "" {
			return v, true
		}
	}
	return "", false
}

// collapse trims s, and collapses each run of white space to one space.
func collapse(s string) string {
	return strings.Join(strings.Fields(s), " ")
}
