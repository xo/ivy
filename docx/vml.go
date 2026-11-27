package docx

import (
	"strconv"
	"strings"

	"github.com/xo/okage/roxmltree"

	"github.com/xo/ivy/internal/ooxml"
)

// vmlShapes are the VML shapes that the conversion draws as a box: a
// rectangle, a rectangle with round corners, an ellipse, and a shape of
// another type, as its rectangle.
var vmlShapes = map[string]bool{"rect": true, "roundrect": true, "oval": true, "shape": true}

// horizontalRule writes a horizontal line of Word, a VML shape with o:hr,
// as a box on the baseline of its line, and reports whether shape is one.
// o:hrpct gives its width in tenths of a percent of the column, and 0 or
// no value with no width in the style is the whole column. The alignment
// of the paragraph places a line that is narrower than the column, and
// o:hralign is not read.
func (c *converter) horizontalRule(shape roxmltree.Node) bool {
	if !shape.IsElement() || ooxml.Attr(shape, "hr") != "t" {
		return false
	}
	st := vmlStyle(shape)
	h := vmlLength(st["height"])
	if h <= 0 {
		h = 1.5
	}
	width := "100%"
	pct := number(ooxml.Attr(shape, "hrpct"), 0)
	switch w := vmlLength(st["width"]); {
	case pct > 0:
		width = num(min(pct/10, 100)) + "%"
	case w > 0 && ooxml.Attr(shape, "hrpct") != "":
		width = pt(min(w, maxDrawingSize))
	}
	color := vmlColor(ooxml.Attr(shape, "fillcolor"))
	if color == "" {
		color = "#a0a0a0"
	}
	var s css
	s.set("display", "inline-block")
	s.set("width", width)
	s.set("height", pt(min(h, maxDrawingSize)))
	s.set("background-color", color)
	c.w.WriteString(`<span style="` + attrText(s.String()) + `"></span>`)
	return true
}

// vmlBox writes a VML shape of the body whose position is absolute, such
// as a text box, a filled rectangle or a picture, as a box with its fill
// or its picture, its line and its text. Text that wraps around the shape
// flows around a float, as for a drawing of DrawingML. A shape above and
// below the text of a column floats at its position from the top of the
// part of its paragraph in the column, as in Word. A shape at a position
// from the margin or from the page with o:allowoverlap off moves down
// below the floats of the page that it covers (D60). A text box with
// mso-wrap-style none is as wide as its text, see fitsText. It returns
// false for a shape that it does not draw.
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
	src, opacity := "", 1.0
	for e := range shape.Children() {
		if !e.IsElement() {
			continue
		}
		switch e.TagName().Name() {
		case "imagedata":
			r, ok := c.relationship(ooxml.Attr(e, "id"))
			if !ok || r.External {
				return false
			}
			if src, ok = c.file(r.Target); !ok {
				return false
			}
			opacity = washout(e)
		case "textbox":
			if t, ok := child(e, "txbxContent"); ok {
				txbx, hasText = t, true
			}
		}
	}
	fill := vmlFill(shape)
	stroke, weight := vmlStroke(shape)
	if src != "" {
		// A picture covers its fill. Its shape type has no line, so it has
		// a line only when the shape asks for one.
		fill = ""
		if _, ok := child(shape, "stroke"); !ok && ooxml.Attr(shape, "stroked") == "" && ooxml.Attr(shape, "strokecolor") == "" {
			stroke, weight = "", 0
		}
	}
	if !hasText && src == "" && fill == "" && stroke == "" {
		return false
	}
	sec := c.sections[c.section]
	colWidth := sec.columnWidth()
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
	wrap, side := "", ""
	for e := range shape.Children() {
		if e.IsElement() && e.TagName().Name() == "wrap" {
			wrap, side = ooxml.Attr(e, "type"), ooxml.Attr(e, "side")
		}
	}
	// A shape with text around it or above and below it, at a position
	// from the margin or from the page, floats at that position (D60). In
	// a section with columns, a shape with text around it from its
	// paragraph floats at its paragraph, in its column. A shape above and
	// below the text of a column floats from the top of the part of its
	// paragraph in the column, as Word and LibreOffice place it.
	position := floatPosition(st["mso-position-vertical-relative"], st["mso-position-vertical"], y)
	if position == "" && sec.cols > 1 && wrap == "topAndBottom" && st["mso-position-vertical"] == "" {
		switch st["mso-position-vertical-relative"] {
		case "", "text":
			position = "block " + pt(y)
		case "line":
			position = "line " + pt(y)
		}
	}
	avoid := position != "" && vmlOff(ooxml.Attr(shape, "allowoverlap"))
	fit := fitsText(st, hasText, src)
	var s css
	switch wrap {
	case "square", "tight", "through":
		c.floatSeq = c.paraSeq
		if x+w/2 > colWidth/2 && !crossesColumn(sec, x, w) && !fit {
			s.set("float", "right")
			s.set("margin-right", pt(max(colWidth-x-w, 0)))
			s.set("margin-left", "9pt")
		} else {
			s.set("float", "left")
			s.set("margin-left", pt(max(x, 0)))
			s.set("margin-right", "9pt")
		}
		switch {
		case position != "":
			s.set("-ivy-float-position", position)
			if avoid {
				s.set("-ivy-float-overlap", "avoid")
			}
		case y > 0 && st["mso-position-vertical-relative"] != "page" && st["mso-position-vertical-relative"] != "margin":
			s.set("margin-top", pt(y))
		}
		s.set("-ivy-wrap-flow", wrapFlow(side)+" 9pt")
		if wrap != "square" {
			if v := vmlWrapCoords(ooxml.Attr(shape, "wrapcoords"), w, h, fit); v != "" {
				s.set("-ivy-wrap-shape", v)
			}
		}
	case "topAndBottom":
		if position != "" {
			s.set("float", "left")
			s.set("margin-left", pt(x))
			if fit {
				// The shape can be narrower than w.
				s.set("margin-right", pt(max(colWidth-x, 0)))
			} else {
				s.set("margin-right", pt(max(colWidth-x-w, 0)))
			}
			s.set("-ivy-float-position", position)
			if avoid {
				s.set("-ivy-float-overlap", "avoid")
			}
			break
		}
		s.set("display", "block")
		s.set("margin-left", pt(max(x, 0)))
		if y > 0 && st["mso-position-vertical-relative"] != "page" && st["mso-position-vertical-relative"] != "margin" {
			s.set("margin-top", pt(y))
		}
		// A block takes the width of its column, so it keeps the width of
		// the shape.
		fit = false
	default:
		s.set("display", "block")
		s.set("position", "absolute")
		switch st["mso-position-horizontal-relative"] {
		case "", "text", "char":
			if sec.cols > 1 && st["mso-position-horizontal"] == "" {
				// The shape is placed from the left of the column of its
				// anchor, as in Word. The box has no left, so that its
				// place starts in the column of the line of the anchor.
				s.set("margin-left", pt(x))
				break
			}
			s.set("left", pt(x))
		default:
			s.set("left", pt(x))
		}
		switch st["mso-position-vertical-relative"] {
		case "page":
			s.set("top", pt(y-sec.top))
		case "margin", "top-margin-area":
			s.set("top", pt(y))
		case "line":
			s.set("margin-top", pt(y))
		default:
			// The shape is placed from the top of the part of its
			// paragraph on the page of its anchor, as in Word.
			s.set("margin-top", pt(y))
			s.set("-ivy-static-top", "block")
		}
		if strings.HasPrefix(strings.TrimSpace(st["z-index"]), "-") {
			s.set("z-index", "-1")
		}
	}
	if !fit {
		s.set("width", pt(w))
	}
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
			if fit {
				// Word makes the ellipse as wide as its text, the insets,
				// the insets of the width of the style and its line, but it
				// puts the text at the inset of the width that it draws.
				// The box keeps its width, and the text moves by the part
				// of the inset that depends on the width of the text, see
				// fitShift.
				extra := inset[1] + inset[3] + weight
				left := inset[3] - w*0.146 + extra*0.146
				inset[1] = max(inset[1]+inset[3]-left, 0)
				inset[3] = left
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
	class := ""
	if fit {
		// The text of the box does not wrap, see the style sheet.
		class = ` class="fit"`
	}
	c.w.WriteString(`<span` + class + ` style="` + attrText(s.String()) + `">`)
	if src != "" {
		var is css
		is.set("display", "block")
		is.set("width", "100%")
		is.set("height", "100%")
		if opacity < 1 {
			is.set("opacity", strconv.FormatFloat(opacity, 'f', 3, 64))
		}
		c.w.WriteString(`<img src="` + attrText(src) + `" alt="" style="` + attrText(is.String()) + `"/>`)
	}
	if hasText {
		shift := fit && shape.TagName().Name() == "oval"
		if shift {
			c.w.WriteString(`<div style="position: relative; left: ` + fitShift + `">`)
		}
		c.nestedBlocks(txbx, c.part, c.w)
		if shift {
			c.w.WriteString("</div>")
		}
	}
	c.w.WriteString("</span>")
	return true
}

// fitShift moves the text of an ellipse that is as wide as its text, see
// fitsText, by the part of the inset of its drawn width that depends on the
// width of the text. The text is as wide as the box that holds it. On page
// 1 of libreoffice/docx/mixed-orientation.docx, Word draws an ellipse of 140
// points 102.3 points wide, and puts its text at 15.4 points from its left
// side, not at 20.4 points.
const fitShift = "14.6%"

// fitsText reports whether a VML shape with the style st is as wide as
// the longest line of its text box, plus the insets of the text box and
// its line: a shape with text and without a picture, with mso-wrap-style
// none, at its place from the left of its area. Word draws such a shape
// at that width and not at the width of its style, and keeps its left
// side and its height. For an ellipse, the width counts the insets of the
// width of the style. On pages 1 and 2 of
// libreoffice/docx/mixed-orientation.docx, Word draws an ellipse of 140
// points 102.3 points wide, and a rectangle of 114 points 77.3 points wide.
// The text of the shape does not wrap.
func fitsText(st map[string]string, hasText bool, src string) bool {
	if !hasText || src != "" || strings.TrimSpace(st["mso-wrap-style"]) != "none" {
		return false
	}
	switch st["mso-position-horizontal"] {
	case "", "left", "inside":
		return true
	}
	return false
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

// vmlStroke returns the border of CSS of the line of a VML shape and the
// width of the line in points, or "" and 0 for a shape without a line. A
// line is black and 0.75 points wide when the shape gives no color and no
// weight.
func vmlStroke(shape roxmltree.Node) (string, float64) {
	if vmlOff(ooxml.Attr(shape, "stroked")) {
		return "", 0
	}
	if f, ok := child(shape, "stroke"); ok && vmlOff(ooxml.Attr(f, "on")) {
		return "", 0
	}
	color := vmlColor(ooxml.Attr(shape, "strokecolor"))
	if color == "" {
		color = "#000000"
	}
	weight := 0.75
	if v := vmlLength(ooxml.Attr(shape, "strokeweight")); v > 0 {
		weight = min(v, 12)
	}
	return pt(weight) + " solid " + color, weight
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

// vmlWrapCoords returns the value of -ivy-wrap-shape for the wrapcoords of
// a VML shape of w by h points, x and y pairs in a square of 21600 by 21600
// over the shape, or "" for none (D63). Word wraps the text of a tight or
// through wrap around them, as on page 1 of
// libreoffice/docx/mixed-orientation.docx. With fit, the points are
// percentages of the box, whose width the layout gives.
func vmlWrapCoords(v string, w, h float64, fit bool) string {
	fields := strings.Fields(strings.ReplaceAll(v, ",", " "))
	if len(fields)%2 != 0 || len(fields) > 2*maxWrapPoints {
		return ""
	}
	xy := make([]float64, 0, len(fields))
	for _, f := range fields {
		n, err := strconv.ParseFloat(f, 64)
		if err != nil || n != n {
			return ""
		}
		xy = append(xy, n)
	}
	if fit {
		return polygonPercent(xy)
	}
	return polygonShape(xy, w, h)
}
