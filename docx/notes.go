package docx

import (
	"strings"

	"github.com/xo/okage/roxmltree"
)

// nestedBlocks converts the content of a note or of a text box into w, from the
// part part. A paragraph there is not the paragraph around it.
func (c *converter) nestedBlocks(n roxmltree.Node, part string, w *strings.Builder) {
	if !c.enter() {
		return
	}
	defer c.leave()
	saved, savedPart, para, open, fields := c.w, c.part, c.para, c.open, c.fields
	c.w, c.part, c.para, c.open, c.fields = w, part, "", nil, nil
	c.nested++
	defer func() {
		c.w, c.part, c.para, c.open, c.fields = saved, savedPart, para, open, fields
		c.nested--
	}()
	for e := range n.Children() {
		if e.IsElement() {
			c.block(e, nil)
		}
	}
}

// footnote writes the footnote id, which the layout puts at the foot of
// the page.
func (c *converter) footnote(id string) {
	n, ok := c.footnotes[id]
	if !ok {
		return
	}
	c.w.WriteString(`<span class="fn">`)
	c.nestedBlocks(n, c.footnotePart, c.w)
	c.w.WriteString("</span>")
}

// endnote writes the mark of the endnote id, and converts the endnote to
// the end of the document. Word numbers the endnotes in lower case Roman
// numerals.
func (c *converter) endnote(id string, p, base props) {
	n, ok := c.endnotes[id]
	if !ok {
		return
	}
	c.endnoteNum++
	mark := roman(c.endnoteNum)
	c.w.WriteString(`<span class="endnote-ref"`)
	if style := c.runStyle(p, base); style != "" {
		c.w.WriteString(` style="` + attrText(style) + `"`)
	}
	c.w.WriteString(">" + mark + "</span>")
	if c.nested > 0 {
		return
	}
	c.endnoteText.WriteString(`<div class="endnote"><span style="float: left; margin-right: 6pt; font-size: 0.65em; vertical-align: super">` + mark + "</span>")
	c.nestedBlocks(n, c.endnotePart, &c.endnoteText)
	c.endnoteText.WriteString("</div>")
}
