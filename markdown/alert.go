package markdown

import (
	"strings"

	"github.com/xo/ivy/internal/goldmark/ast"
	"github.com/xo/ivy/internal/goldmark/text"
)

// alertTitles are the kinds of the alerts of GitHub, in lower case, and
// their titles.
var alertTitles = map[string]string{
	"note":      "Note",
	"tip":       "Tip",
	"important": "Important",
	"warning":   "Warning",
	"caution":   "Caution",
}

// alerts makes an alert of GitHub of each block quote of doc whose first
// line is a marker such as "[!NOTE]". goldmark does not read alerts. As on
// GitHub, an alert is a block quote at the top level of the document, its
// marker is alone on its line, and text follows the marker. The block quote
// takes the classes "markdown-alert" and "markdown-alert-<kind>", and a
// first paragraph of the class "markdown-alert-title" with the title of the
// kind, such as "Note", in place of the marker. GitHub also shows an icon
// before the title. The style sheet gives the colors of each kind.
func alerts(doc ast.Node, src []byte) {
	for n := doc.FirstChild(); n != nil; n = n.NextSibling() {
		q, ok := n.(*ast.Blockquote)
		if !ok {
			continue
		}
		p, ok := q.FirstChild().(*ast.Paragraph)
		if !ok {
			continue
		}
		lines := p.Source()
		if len(lines) == 0 {
			continue
		}
		first := lines[0]
		marker := strings.TrimSpace(first.Str(src))
		kind, ok := strings.CutPrefix(marker, "[!")
		if !ok {
			continue
		}
		kind, ok = strings.CutSuffix(kind, "]")
		title := alertTitles[strings.ToLower(kind)]
		if !ok || title == "" {
			continue
		}
		// the text of the alert follows the marker, in the same paragraph or
		// in a later block
		if len(lines) == 1 && p.NextSibling() == nil {
			continue
		}
		// the marker is the inline nodes of the first line
		for c := p.FirstChild(); c != nil; {
			next := c.NextSibling()
			if pos := c.Pos(); pos < 0 || pos >= first.Stop {
				break
			}
			p.RemoveChild(c)
			c = next
		}
		if p.FirstChild() == nil {
			q.RemoveChild(p)
		}
		h := ast.NewParagraph()
		h.SetAttribute("class", text.NewMultiLineValueFromString("markdown-alert-title", nil))
		h.AppendChild(ast.NewText(text.NewSingleLineValueFromString(title, nil)))
		if c := q.FirstChild(); c != nil {
			q.InsertBefore(c, h)
		} else {
			q.AppendChild(h)
		}
		q.SetAttribute("class", text.NewMultiLineValueFromString("markdown-alert markdown-alert-"+strings.ToLower(kind), nil))
	}
}
