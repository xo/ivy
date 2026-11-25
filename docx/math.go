package docx

import (
	"strconv"
	"strings"

	"github.com/xo/okage"
	"github.com/xo/okage/roxmltree"
	"github.com/xo/okage/texmath"

	"github.com/xo/ivy/internal/drawingml"
	"github.com/xo/ivy/internal/formula"
	"github.com/xo/ivy/internal/layout"
	"github.com/xo/ivy/internal/ooxml"
)

// math writes an equation of Office Math, an m:oMath or an m:oMathPara
// element. An m:oMathPara holds display equations: each of its m:oMath
// elements is a line of its own, aligned by its m:jc, centered by default.
// An m:oMath in the text is an inline equation, whose baseline sits on the
// baseline of the line.
func (c *converter) math(n roxmltree.Node, base props) {
	if n.TagName().Name() != "oMathPara" {
		c.equation(n, base, texmath.Text)
		return
	}
	align := "center"
	if pr, ok := child(n, "oMathParaPr"); ok {
		if jc, ok := child(pr, "jc"); ok {
			switch ooxml.Attr(jc, "val") {
			case "left":
				align = "left"
			case "right":
				align = "right"
			}
		}
	}
	for e := range n.Children() {
		if !e.IsElement() || e.TagName().Name() != "oMath" {
			continue
		}
		c.w.WriteString(`<span style="display: block; text-align: ` + align + `; text-indent: 0">`)
		c.equation(e, base, texmath.Display)
		c.w.WriteString("</span>")
	}
}

// equation writes the m:oMath element n in the style, as an object of the
// layout that internal/formula lays out and draws. The size and the color
// of the equation are those of its first run. An equation that cannot be
// laid out is written as its text, in italics.
func (c *converter) equation(n roxmltree.Node, base props, style texmath.Style) {
	p := base
	if r, ok := firstRun(n); ok {
		for e := range r.Children() {
			if e.IsElement() && e.TagName().Name() == "rPr" && isWord(e) {
				p = p.over(parseRPr(e))
			}
		}
	}
	if c.fieldHidden() || p.on("vanish") {
		return
	}
	if c.typesetter == nil {
		ts, err := formula.New()
		if err != nil {
			c.log.Warn("docx: equations written as text", "err", err)
		}
		c.typesetter = ts
	}
	tree, ok := formula.OMML(n, c.lim.MaxDepth)
	if !ok || c.typesetter == nil || c.objects >= c.lim.MaxObjects {
		c.mathText(n, base)
		return
	}
	obj, err := c.typesetter.Layout(tree, fontSize(p), style, c.mathColor(p))
	if err != nil {
		c.log.Warn("docx: equation written as text", "err", err)
		c.mathText(n, base)
		return
	}
	c.objects++
	name := "math" + strconv.Itoa(len(c.charts)+1)
	c.charts[name] = obj
	w, h := obj.Size()
	c.w.WriteString(`<img src="` + attrText(layout.ObjectURL(name)) + `" alt="" style="width: ` + pt(w) +
		`; height: ` + pt(h) + `; vertical-align: ` + pt(-obj.Depth()) + `"/>`)
}

// mathText writes the text of the equation n, in italics.
func (c *converter) mathText(n roxmltree.Node, base props) {
	var text strings.Builder
	for d := range n.Descendants() {
		if d.IsElement() && d.TagName().Name() == "t" {
			s, _ := d.Text()
			text.WriteString(s)
		}
	}
	c.text(text.String(), base.over(props{"i": "1"}), base)
}

// mathColor returns the color of the text of an equation with the
// properties p, black by default.
func (c *converter) mathColor(p props) okage.Color {
	if v, ok := drawingml.ParseHex(p["color"]); ok {
		return drawingml.RGBAFrom(v).Okage()
	}
	if t := p["color.theme"]; t != "" && c.theme != nil {
		if v, ok := c.theme.Color(t); ok {
			return drawingml.RGBAFrom(v).Okage()
		}
	}
	return okage.ColorBlack
}

// isWord reports whether the element e is in the namespace of
// WordprocessingML, such as the w:rPr of a run of Office Math, which also
// holds an m:rPr.
func isWord(e roxmltree.Node) bool {
	ns, _ := e.TagName().Namespace()
	return strings.Contains(ns, "wordprocessingml")
}

// firstRun returns the first m:r element in the equation n.
func firstRun(n roxmltree.Node) (roxmltree.Node, bool) {
	for d := range n.Descendants() {
		if d.IsElement() && d.TagName().Name() == "r" {
			return d, true
		}
	}
	return roxmltree.Node{}, false
}
