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
//
// Some files use a namespace prefix that they do not declare, such as l: or
// xlink: in a FictionBook file. If the parse fails on one, ParseXML declares
// the prefix on the root element and parses again, as the readers of these
// files do. It declares xlink: and l: with the namespace of XLink.
func ParseXML(buf []byte, lim ivy.Limits) (*roxmltree.Document, error) {
	lim = lim.WithDefaults()
	opt := roxmltree.DefaultParsingOptions()
	opt.AllowDTD = true
	opt.NodesLimit = uint32(min(lim.MaxObjects, math.MaxUint32))
	text := Text(buf)
	doc, err := roxmltree.ParseWithOptions(text, opt)
	entities, declared := false, 0
	for err != nil {
		if errors.Is(err, okage.ErrXMLUnknownEntityReference) && !entities {
			text, entities = htmlEntities(text), true
		} else {
			prefix, ok := unknownPrefix(err)
			if !ok || declared >= maxUndeclared {
				break
			}
			if text, ok = declarePrefix(text, prefix); !ok {
				break
			}
			declared++
		}
		doc, err = roxmltree.ParseWithOptions(text, opt)
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

// maxUndeclared is the number of undeclared namespace prefixes that ParseXML
// declares in one file.
const maxUndeclared = 8

// prefixNamespaces are the namespaces of the prefixes that files use without
// a declaration. Another prefix gets a namespace of its own.
var prefixNamespaces = map[string]string{
	"l":     "http://www.w3.org/1999/xlink",
	"xlink": "http://www.w3.org/1999/xlink",
}

// unknownPrefixMessage matches the message of an error of roxmltree for an
// unknown namespace prefix, which names the prefix.
var unknownPrefixMessage = regexp.MustCompile(`prefix '([A-Za-z_][A-Za-z0-9._-]{0,63})'`)

// unknownPrefix returns the namespace prefix that err says is unknown.
func unknownPrefix(err error) (string, bool) {
	var xerr *roxmltree.Error
	if !errors.Is(err, okage.ErrXMLUnknownNamespace) || !errors.As(err, &xerr) {
		return "", false
	}
	m := unknownPrefixMessage.FindStringSubmatch(xerr.Message())
	if m == nil {
		return "", false
	}
	return m[1], true
}

// declarePrefix returns text with a declaration of the namespace prefix
// prefix on its root element. It returns false if text has no root element,
// or if the root element already declares prefix.
func declarePrefix(text, prefix string) (string, bool) {
	i, ok := rootName(text)
	if !ok {
		return text, false
	}
	attr := " xmlns:" + prefix + "="
	if end := strings.IndexByte(text[i:], '>'); end < 0 || strings.Contains(text[i:i+end], attr) {
		return text, false
	}
	ns, ok := prefixNamespaces[prefix]
	if !ok {
		ns = "urn:x-ivy:undeclared:" + prefix
	}
	return text[:i] + attr + `"` + ns + `"` + text[i:], true
}

// rootName returns the index after the name of the start tag of the root
// element of text. It skips the declaration of XML, the processing
// instructions, the comments and the document type, with its internal
// subset.
func rootName(text string) (int, bool) {
	i := 0
	for {
		lt := strings.IndexByte(text[i:], '<')
		if lt < 0 {
			return 0, false
		}
		i += lt
		rest := text[i:]
		switch {
		case strings.HasPrefix(rest, "<?"):
			i = skipPast(text, i+2, "?>")
		case strings.HasPrefix(rest, "<!--"):
			i = skipPast(text, i+4, "-->")
		case strings.HasPrefix(rest, "<!"):
			i = skipDoctype(text, i+2)
		default:
			j := i + 1
			for j < len(text) && !isSpace(text[j]) && text[j] != '>' && text[j] != '/' {
				j++
			}
			if j == i+1 || j == len(text) {
				return 0, false
			}
			return j, true
		}
	}
}

// skipDoctype returns the index after the end of the document type that
// starts before i, after its internal subset in brackets.
func skipDoctype(text string, i int) int {
	depth := 0
	for ; i < len(text); i++ {
		switch text[i] {
		case '[':
			depth++
		case ']':
			depth--
		case '>':
			if depth <= 0 {
				return i + 1
			}
		}
	}
	return len(text)
}
