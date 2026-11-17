package zipdoc

import (
	"archive/zip"
	"bytes"
	"errors"
	"fmt"
	"strings"

	"github.com/xo/ivy"
	"github.com/xo/okage"
	"github.com/xo/okage/roxmltree"
)

// ParseXML reads the entry f, and parses it with the XML parser of okage
// (D7). It reads UTF-8, with or without a byte order mark, and with a
// DOCTYPE. The number of nodes is bounded by Limits.MaxObjects. It returns
// an error that wraps ivy.ErrFormat for XML that is not well formed, and one
// that wraps ivy.ErrLimit for too many nodes.
func (a *Archive) ParseXML(f *zip.File) (*roxmltree.Document, error) {
	buf, err := a.Read(f)
	if err != nil {
		return nil, err
	}
	return ParseXML(buf, a.lim)
}

// ParseXML parses buf as ParseXML of an Archive does.
func ParseXML(buf []byte, lim ivy.Limits) (*roxmltree.Document, error) {
	lim = lim.WithDefaults()
	text := string(bytes.TrimPrefix(buf, []byte("\xef\xbb\xbf")))
	// the declaration can name another encoding, and the text is UTF-8
	if strings.HasPrefix(text, "<?xml") {
		if end := strings.Index(text, "?>"); end >= 0 {
			text = text[end+2:]
		}
	}
	opt := roxmltree.DefaultParsingOptions()
	// Krita writes a DOCTYPE. The parser loads no outside DTD, and it
	// bounds the expansion of the entities
	opt.AllowDTD = true
	opt.NodesLimit = uint32(min(lim.MaxObjects, 1<<31))
	doc, err := roxmltree.ParseWithOptions(text, opt)
	switch {
	case errors.Is(err, okage.ErrXMLNodesLimitReached):
		return nil, fmt.Errorf("xml of more than %d nodes: %w", lim.MaxObjects, ivy.ErrLimit)
	case err != nil:
		return nil, fmt.Errorf("parsing xml: %w: %w", ivy.ErrFormat, err)
	}
	return doc, nil
}

// ChildText returns the text of the first child element of n with the local
// name name, without the space at its ends, or "" if n has none.
func ChildText(n roxmltree.Node, name string) string {
	for c := range n.Children() {
		if c.IsElement() && c.TagName().Name() == name {
			var b strings.Builder
			for t := range c.Descendants() {
				if s, ok := t.Text(); ok && t.IsText() {
					b.WriteString(s)
				}
			}
			return strings.TrimSpace(b.String())
		}
	}
	return ""
}
