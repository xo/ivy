package odf

import (
	"path"
	"strconv"
	"strings"

	"github.com/xo/okage"
	"github.com/xo/okage/roxmltree"
	"github.com/xo/okage/texmath"

	"github.com/xo/ivy/internal/formula"
	"github.com/xo/ivy/internal/layout"
)

// maxFormulaDepth is the depth of the elements of a formula that the
// conversion reads. A deeper element draws nothing.
const maxFormulaDepth = 64

// formula writes the embedded object href as an equation, if it is a
// formula of OpenDocument: MathML in the content.xml of the object, whose
// settings.xml gives the size of its base font, 12 points by default. The
// equation sits on the baseline of its line, as LibreOffice sets a formula
// anchored as a character. It returns false for another object.
func (c *odtConverter) formula(href string) bool {
	name := path.Clean(strings.TrimPrefix(href, "./"))
	if name == "." || strings.HasPrefix(name, "..") || strings.Contains(href, "://") {
		return false
	}
	doc, err := c.readXML(name + "/content.xml")
	if err != nil {
		return false
	}
	root := doc.RootElement()
	if root.TagName().Name() != "math" {
		return false
	}
	if c.objects >= c.lim.MaxObjects {
		return false
	}
	if c.typesetter == nil {
		ts, err := formula.New()
		if err != nil {
			c.log.Warn("odf: formula left out", "object", name, "err", err)
			return false
		}
		c.typesetter = ts
	}
	tree, block := formula.MathML(mathElement(root, 0))
	style := texmath.Text
	if block {
		style = texmath.Display
	}
	obj, err := c.typesetter.Layout(tree, c.formulaSize(name), style, okage.ColorBlack)
	if err != nil {
		c.log.Warn("odf: formula left out", "object", name, "err", err)
		return false
	}
	c.objects++
	id := "math" + strconv.Itoa(len(c.charts)+1)
	c.charts[id] = obj
	w, h := obj.Size()
	c.w.WriteString(`<img src="` + odtAttrText(layout.ObjectURL(id)) + `" alt="" style="width: ` + odtPt(w) +
		`; height: ` + odtPt(h) + `; vertical-align: ` + odtPt(-obj.Depth()) + `"/>`)
	return true
}

// formulaSize returns the size of the base font of the formula object
// name, in points, from the item BaseFontHeight of its settings.xml.
func (c *odtConverter) formulaSize(name string) float64 {
	doc, err := c.readXML(name + "/settings.xml")
	if err != nil {
		return 12
	}
	for d := range doc.Descendants() {
		if !d.IsElement() || d.TagName().Name() != "config-item" || odtAttr(d, "name") != "BaseFontHeight" {
			continue
		}
		s, _ := d.Text()
		if v, err := strconv.ParseFloat(strings.TrimSpace(s), 64); err == nil && v > 0 && v < 1000 {
			return v
		}
		break
	}
	return 12
}

// mathElement returns the element n of MathML, and its children, for
// internal/formula.
func mathElement(n roxmltree.Node, depth int) *formula.Element {
	e := &formula.Element{Name: n.TagName().Name(), Attrs: map[string]string{}}
	for a := range n.Attributes() {
		e.Attrs[a.Name()] = a.Value()
	}
	var text strings.Builder
	for c := range n.Children() {
		switch {
		case c.IsElement():
			if depth < maxFormulaDepth {
				e.Children = append(e.Children, mathElement(c, depth+1))
			}
		case c.IsText():
			s, _ := c.Text()
			text.WriteString(s)
		}
	}
	e.Text = text.String()
	return e
}
