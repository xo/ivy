package ebook

import (
	"errors"
	"fmt"
	"html"
	"math"
	"regexp"
	"strings"

	"github.com/xo/ivy"
	"github.com/xo/okage"
	"github.com/xo/okage/roxmltree"
)

// ParseXML parses buf as XML, within lim.MaxObjects nodes. It allows a DTD,
// because an NCX file declares one. The parser of okage still guards against
// the expansion of entities without end.
//
// Some files use the entities of HTML, such as &nbsp;, which XML does not
// know. If the parse fails on one, ParseXML replaces each entity of HTML with
// its character and parses again.
func ParseXML(buf []byte, lim ivy.Limits) (*roxmltree.Document, error) {
	lim = lim.WithDefaults()
	opt := roxmltree.DefaultParsingOptions()
	opt.AllowDTD = true
	opt.NodesLimit = uint32(min(lim.MaxObjects, math.MaxUint32))
	text := Text(buf)
	doc, err := roxmltree.ParseWithOptions(text, opt)
	if errors.Is(err, okage.ErrXMLUnknownEntityReference) {
		doc, err = roxmltree.ParseWithOptions(htmlEntities(text), opt)
	}
	switch {
	case errors.Is(err, okage.ErrXMLNodesLimitReached):
		return nil, fmt.Errorf("xml of more than %d nodes: %w", lim.MaxObjects, ivy.ErrLimit)
	case err != nil:
		return nil, fmt.Errorf("%w: %w", ivy.ErrFormat, err)
	}
	return doc, nil
}

// entity matches a named reference to an entity.
var entity = regexp.MustCompile(`&[A-Za-z][A-Za-z0-9]{0,31};`)

// htmlEntities replaces each named entity of HTML in text with its
// character, escaped for XML. It keeps the five entities of XML.
func htmlEntities(text string) string {
	return entity.ReplaceAllStringFunc(text, func(ref string) string {
		switch ref {
		case "&amp;", "&lt;", "&gt;", "&quot;", "&apos;":
			return ref
		}
		s := html.UnescapeString(ref)
		if s == ref {
			// an unknown entity stays as text
			return "&amp;" + ref[1:]
		}
		return html.EscapeString(s)
	})
}

// Local returns the local name of the element n, without its namespace.
func Local(n roxmltree.Node) string {
	return n.TagName().Name()
}

// Attr returns the value of the attribute of n with the local name name, in
// any namespace, or the empty string.
func Attr(n roxmltree.Node, name string) string {
	for a := range n.Attributes() {
		if a.Name() == name {
			return a.Value()
		}
	}
	return ""
}

// NodeText returns the text of the element n and of its descendants, with
// its runs of white space collapsed to one space.
func NodeText(n roxmltree.Node) string {
	var b strings.Builder
	for d := range n.Descendants() {
		if d.IsText() {
			s, _ := d.Text()
			b.WriteString(s)
		}
	}
	return strings.Join(strings.Fields(b.String()), " ")
}
