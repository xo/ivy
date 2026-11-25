package layout

import (
	"strings"

	"github.com/xo/okage"
	"github.com/xo/okage/texmath"

	"github.com/xo/ivy/internal/formula"
)

// This file ports no file of WeasyPrint, which draws no MathML. It draws a
// math element of MathML as a replaced box, whose picture internal/formula
// lays out with texmath (D62).

// nsMathML is the namespace of MathML.
const nsMathML = "http://www.w3.org/1998/Math/MathML"

// maxMathDepth is the depth of the elements of a formula that the layout
// reads. A deeper element draws nothing.
const maxMathDepth = 64

// handleMath handles a math element of MathML. The formula is an inline
// replaced box, whose baseline sits on the baseline of the line. A formula
// with the attribute display="block" is a line of its own, centered. A
// formula that cannot be laid out keeps the boxes of its text.
func handleMath(b *builder, element *Element, box *Box, _ string) []*Box {
	if b.typesetter == nil {
		ts, err := formula.New()
		if err != nil {
			b.log.warning("Failed to load the math font: %v", err)
			return []*Box{box}
		}
		b.typesetter = ts
	}
	list, block := formula.MathML(mathElement(element, 0))
	style := texmath.Text
	if block {
		style = texmath.Display
	}
	// The font size is in px, and the object is in points.
	size := num(box.Style.Get("font_size")) * 0.75
	obj, err := b.typesetter.Layout(list, size, style, mathColor(box.Style))
	if err != nil {
		b.log.warning("Failed to lay out a formula: %v", err)
		return []*Box{box}
	}
	image := &ObjectImage{Name: "math", Object: obj}
	inline := box.Style.Copy()
	inline.Set("display", tupleOf("inline", "flow"))
	inline.Set("vertical_align", Float(-obj.Depth()/0.75))
	replaced := newReplacedBox(KindInlineReplaced, element.Tag, inline, element, image)
	if !block {
		return []*Box{replaced}
	}
	outer := box.Style.Copy()
	outer.Set("display", tupleOf("block", "flow"))
	outer.Set("text_align_all", String("center"))
	outer.Set("text_align_last", String("center"))
	outer.Set("text_indent", Dimension{0, "px"})
	return []*Box{makeBox(element.Tag, outer, []*Box{replaced}, element)}
}

// mathElement returns the element e of MathML, and its children, for
// internal/formula. The text of an element is its text and the tails of its
// children.
func mathElement(e *Element, depth int) *formula.Element {
	name, _ := e.localName()
	out := &formula.Element{Name: name, Attrs: make(map[string]string, len(e.Attrib))}
	for _, a := range e.Attrib {
		out.Attrs[a.Name] = a.Value
	}
	var text strings.Builder
	text.WriteString(e.Text)
	for _, c := range e.Children {
		text.WriteString(c.Tail)
		if depth < maxMathDepth {
			out.Children = append(out.Children, mathElement(c, depth+1))
		}
	}
	out.Text = text.String()
	return out
}

// mathColor returns the color of the text of the style.
func mathColor(style *Style) okage.Color {
	r, g, b, alpha := srgbOf(getColor(style, "color"))
	c, ok := okage.ColorFromRGBA(float32(r), float32(g), float32(b), float32(alpha))
	if !ok {
		return okage.ColorBlack
	}
	return c
}
