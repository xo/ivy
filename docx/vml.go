package docx

import (
	"strings"

	"github.com/xo/okage/roxmltree"

	"github.com/xo/ivy/internal/ooxml"
)

// vmlShapes are the VML shapes that the conversion draws as a box: a
// rectangle, a rectangle with round corners, an ellipse, and a shape of
// another type, as its rectangle.
var vmlShapes = map[string]bool{"rect": true, "roundrect": true, "oval": true, "shape": true}

// vmlBox writes a VML shape of the body whose position is absolute, such
// as a text box or a filled rectangle, as a box with its fill, its line
// and its text. Text that wraps around the shape flows around a float, as
// for a drawing of DrawingML, except on a page of columns. It returns false for a shape that it does
// not draw, such as a picture.
func (c *converter) vmlBox(shape roxmltree.Node) bool {
	if !shape.IsElement() || !vmlShapes[shape.TagName().Name()] || c.margin != "" {
		return false
	}
	st := vmlStyle(shape)
	w, h := vmlLength(st["width"]), vmlLength(st["height"])
	if st["position"] != "absolute" || w <= 0 || h <= 0 {
		return false
	}
	var txbx roxmltree.Node
	hasText := false
	for e := range shape.Children() {
		if !e.IsElement() {
			continue
		}
		switch e.TagName().Name() {
		case "imagedata":
			return false
		case "textbox":
			if t, ok := child(e, "txbxContent"); ok {
				txbx, hasText = t, true
			}
		}
	}
	fill, stroke := vmlFill(shape), vmlStroke(shape)
	if !hasText && fill == "" && stroke == "" {
		return false
	}
	sec := c.sections[c.section]
	colWidth := sec.width - sec.left - sec.right - sec.gutter
	x, y := vmlSigned(st["margin-left"]), vmlSigned(st["margin-top"])
	switch st["mso-position-horizontal-relative"] {
	case "page":
		x -= sec.left + sec.gutter
	case "left-margin-area", "inner-margin-area":
		x -= sec.left
	}
	switch st["mso-position-horizontal"] {
	case "left", "inside":
		x = 0
	case "right", "outside":
		x = colWidth - w
	case "center":
		x = (colWidth - w) / 2
	}
	wrap := ""
	for e := range shape.Children() {
		if e.IsElement() && e.TagName().Name() == "wrap" {
			wrap = ooxml.Attr(e, "type")
		}
	}
	if sec.cols > 1 {
		// A float stays in its column, and a shape of a page of columns
		// often crosses them. It takes its place on the page, and the text
		// does not wrap around it.
		wrap = ""
	}
	var s css
	switch wrap {
	case "square", "tight", "through":
		if x+w/2 > colWidth/2 {
			s.set("float", "right")
			s.set("margin-right", pt(max(colWidth-x-w, 0)))
			s.set("margin-left", "9pt")
		} else {
			s.set("float", "left")
			s.set("margin-left", pt(max(x, 0)))
			s.set("margin-right", "9pt")
		}
		if y > 0 && st["mso-position-vertical-relative"] != "page" && st["mso-position-vertical-relative"] != "margin" {
			s.set("margin-top", pt(y))
		}
	case "topAndBottom":
		s.set("display", "block")
		s.set("margin-left", pt(max(x, 0)))
		if y > 0 && st["mso-position-vertical-relative"] != "page" && st["mso-position-vertical-relative"] != "margin" {
			s.set("margin-top", pt(y))
		}
	default:
		s.set("display", "block")
		s.set("position", "absolute")
		s.set("left", pt(x))
		switch st["mso-position-vertical-relative"] {
		case "page":
			s.set("top", pt(y-sec.top))
		case "margin", "top-margin-area":
			s.set("top", pt(y))
		default:
			s.set("margin-top", pt(y))
		}
		if strings.HasPrefix(strings.TrimSpace(st["z-index"]), "-") {
			s.set("z-index", "-1")
		}
	}
	s.set("width", pt(w))
	s.set("height", pt(h))
	s.set("box-sizing", "border-box")
	if fill != "" {
		s.set("background-color", fill)
	}
	if stroke != "" {
		s.set("border", stroke)
	}
	switch shape.TagName().Name() {
	case "oval":
		s.set("border-radius", "50%")
	case "roundrect":
		s.set("border-radius", pt(min(w, h)*0.1))
	}
	if hasText {
		inset := vmlInset(shape)
		if shape.TagName().Name() == "oval" {
			// The text of an ellipse is in the rectangle inside it.
			for i, v := range [...]float64{h, w, h, w} {
				inset[i] += v * 0.146
			}
		}
		s.set("padding", pt(inset[0])+" "+pt(inset[1])+" "+pt(inset[2])+" "+pt(inset[3]))
		switch st["v-text-anchor"] {
		case "middle", "middle-center", "middle-baseline":
			s.set("display", "flex")
			s.set("flex-direction", "column")
			s.set("justify-content", "center")
		case "bottom", "bottom-center", "bottom-baseline":
			s.set("display", "flex")
			s.set("flex-direction", "column")
			s.set("justify-content", "flex-end")
		}
	}
	c.w.WriteString(`<span style="` + attrText(s.String()) + `">`)
	if hasText {
		c.nestedBlocks(txbx, c.part, c.w)
	}
	c.w.WriteString("</span>")
	return true
}

// vmlSigned returns a length of the style of a VML shape that can be
// negative, in points.
func vmlSigned(s string) float64 {
	s = strings.TrimSpace(s)
	if rest, ok := strings.CutPrefix(s, "-"); ok {
		return -vmlLength(rest)
	}
	return vmlLength(s)
}

// vmlColor returns a color of VML, such as "#ff420e", "aqua" or
// "#4f81bd [3204]", as CSS, or "".
func vmlColor(s string) string {
	s, _, _ = strings.Cut(strings.TrimSpace(s), " ")
	if v, ok := hexColor(strings.TrimPrefix(s, "#")); ok && strings.HasPrefix(s, "#") {
		return v
	}
	if len(s) == 4 && s[0] == '#' {
		if v, ok := hexColor(string([]byte{s[1], s[1], s[2], s[2], s[3], s[3]})); ok {
			return v
		}
	}
	for _, r := range s {
		if (r < 'a' || r > 'z') && (r < 'A' || r > 'Z') {
			return ""
		}
	}
	return strings.ToLower(s)
}

// vmlOff reports whether a boolean of VML is off: "f", "false" or "0".
func vmlOff(s string) bool {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "f", "false", "0":
		return true
	}
	return false
}

// vmlFill returns the color of the fill of a VML shape as CSS, or "" for a
// shape without a fill. A shape fills with white when it gives no color.
func vmlFill(shape roxmltree.Node) string {
	if vmlOff(ooxml.Attr(shape, "filled")) {
		return ""
	}
	if f, ok := child(shape, "fill"); ok && vmlOff(ooxml.Attr(f, "on")) {
		return ""
	}
	if v := vmlColor(ooxml.Attr(shape, "fillcolor")); v != "" {
		return v
	}
	return "#ffffff"
}

// vmlStroke returns the border of CSS of the line of a VML shape, or ""
// for a shape without a line. A line is black and 0.75 points wide when
// the shape gives no color and no weight.
func vmlStroke(shape roxmltree.Node) string {
	if vmlOff(ooxml.Attr(shape, "stroked")) {
		return ""
	}
	if f, ok := child(shape, "stroke"); ok && vmlOff(ooxml.Attr(f, "on")) {
		return ""
	}
	color := vmlColor(ooxml.Attr(shape, "strokecolor"))
	if color == "" {
		color = "#000000"
	}
	weight := 0.75
	if v := vmlLength(ooxml.Attr(shape, "strokeweight")); v > 0 {
		weight = min(v, 12)
	}
	return pt(weight) + " solid " + color
}

// vmlInset returns the padding of the text box of a VML shape, from the
// attribute inset of v:textbox, as the top, the right, the bottom and the
// left. The attribute gives the left, the top, the right and the bottom,
// by default 7.2, 3.6, 7.2 and 3.6 points.
func vmlInset(shape roxmltree.Node) [4]float64 {
	in := [4]float64{7.2, 3.6, 7.2, 3.6}
	if tb, ok := child(shape, "textbox"); ok {
		for i, v := range strings.Split(ooxml.Attr(tb, "inset"), ",") {
			if i >= 4 {
				break
			}
			v = strings.TrimSpace(v)
			if v == "" {
				continue
			}
			if v == "0" {
				in[i] = 0
				continue
			}
			in[i] = vmlLength(v)
		}
	}
	return [4]float64{in[1], in[2], in[3], in[0]}
}
