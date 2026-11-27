package rtf

import (
	"fmt"
	"math"
	"strconv"
	"strings"
)

// This file draws a shape, \shp: the WordArt of a shape whose property
// gtextUNICODE holds a text, such as the watermark that Word puts in a
// header, and the picture or the text box of a shape, which floats on the
// page (RTF 1.9.1, "Word 97 through Word 2007 RTF for Drawing Objects
// (Shapes)", and MS-ODRAW 2.3.22).

// The limits of the properties of a shape: their number and the size of a
// value. maxShapeSize is the largest side of a shape that floats, in
// twips.
const (
	maxShapeProps     = 256
	maxShapeValueSize = 4096
	maxShapeSize      = 31680
)

// The origins of the place of a shape, from \shpbx and \shpby, or from the
// properties posrelh and posrelv: the margin, the page, and the column or
// the paragraph. They are the values of bx and by of an FSPA of MS-DOC.
const (
	originMargin = iota
	originPage
	originColumn
)

// shapeData is the state of a shape, from its group \shpinst.
type shapeData struct {
	// prop is the name of the last property, and box the box of the shape
	// in twips: left, top, right and bottom.
	prop string
	box  [4]int
	// props holds the values of the properties, by their names.
	props map[string]string
	// bx and by are the origins of the box, and ignoreX and ignoreY are
	// set by \shpbxignore and \shpbyignore, which give the origins to the
	// properties posrelh and posrelv.
	bx, by           int
	ignoreX, ignoreY bool
	// wr is how the text wraps: 1 above and below, 3 not at all, and the
	// others around the shape. wrk is the side of the text: 0 both sides,
	// 1 the left side, 2 the right side and 3 the largest side. behind is
	// set by \shpfblwtxt1, for a shape behind the text.
	wr, wrk int
	behind  bool
	// pic is the name of the picture of the property pib, of picW by picH
	// twips, or "".
	pic        string
	picW, picH int
	// text is the XHTML of the text of a text box, \shptxt, when hasText
	// is set.
	text    string
	hasText bool
}

// shapeWord handles a control word of the group \shpinst.
func (c *converter) shapeWord(t token) {
	sh := &c.shape
	v := min(max(t.param, -288000), 288000)
	switch t.word {
	case "shpleft":
		sh.box[0] = v
	case "shptop":
		sh.box[1] = v
	case "shpright":
		sh.box[2] = v
	case "shpbottom":
		sh.box[3] = v
	case "shpbxmargin":
		sh.bx = originMargin
	case "shpbxpage":
		sh.bx = originPage
	case "shpbxcolumn":
		sh.bx = originColumn
	case "shpbxignore":
		sh.ignoreX = true
	case "shpbymargin":
		sh.by = originMargin
	case "shpbypage":
		sh.by = originPage
	case "shpbypara":
		sh.by = originColumn
	case "shpbyignore":
		sh.ignoreY = true
	case "shpwr":
		sh.wr = v
	case "shpwrk":
		sh.wrk = v
	case "shpfblwtxt":
		sh.behind = v != 0
	}
}

// origins returns the origins of the box of the shape. The properties
// posrelh and posrelv replace \shpbx and \shpby, as in the package msdoc.
// The character and the line are the column and the paragraph. Without
// them, \shpbxignore and \shpbyignore give the default of MS-ODRAW, the
// column and the paragraph.
func (c *converter) origins() (int, int) {
	bx, by := c.shape.bx, c.shape.by
	if c.shape.ignoreX {
		bx = originColumn
	}
	if c.shape.ignoreY {
		by = originColumn
	}
	if v := c.shapeInt("posrelh", -1); v >= 0 && v <= 3 {
		bx = [...]int{originMargin, originPage, originColumn, originColumn}[v]
	}
	if v := c.shapeInt("posrelv", -1); v >= 0 && v <= 3 {
		by = [...]int{originMargin, originPage, originColumn, originColumn}[v]
	}
	return bx, by
}

// shapeValue adds text to the value of the current property of a shape.
func (c *converter) shapeValue(text string) {
	sh := &c.shape
	name := strings.TrimSpace(sh.prop)
	if sh.props == nil {
		sh.props = map[string]string{}
	}
	v, ok := sh.props[name]
	if !ok && len(sh.props) >= maxShapeProps || len(v)+len(text) > maxShapeValueSize {
		return
	}
	sh.props[name] = v + text
}

// shapeInt returns the property name of the shape as a number, or def.
func (c *converter) shapeInt(name string, def int) int {
	v, err := strconv.Atoi(strings.TrimSpace(c.shape.props[name]))
	if err != nil {
		return def
	}
	return v
}

// endShape draws the shape that a group \shpinst ended in the group
// parent: its WordArt, or else its picture or its text box, see
// floatShape.
func (c *converter) endShape(parent *state) {
	if parent.dest != destText || parent.chr.hidden {
		return
	}
	if text := strings.TrimSpace(c.shape.props["gtextUNICODE"]); text != "" && c.shapeInt("fGtext", 1) != 0 {
		c.endWordArt(parent, text)
		return
	}
	c.floatShape(parent)
}

// endWordArt draws the WordArt text of the shape. A WordArt of a header or
// a footer, such as a watermark, shows on each page of the section, and
// waits for the element of the section. A WordArt of the body shows at its
// place.
func (c *converter) endWordArt(parent *state, text string) {
	svg, style := c.wordArt(text)
	if svg == "" {
		return
	}
	fixed := parent.target != c.body && !c.inNote()
	if !fixed {
		style = strings.Replace(style, "position: fixed", "position: absolute", 1)
	}
	// the headers of a section often repeat the same watermark
	key := svg + style
	if fixed && c.fixedSeen[key] {
		return
	}
	c.files++
	if c.files > c.lim.MaxObjects {
		return
	}
	name := "media/" + strconv.Itoa(c.files) + ".svg"
	c.fs.Add(name, []byte(svg))
	html := `<img src="` + name + `" alt="" style="` + attrText(style) + `"/>`
	if !fixed {
		c.inline(parent, html)
		return
	}
	if c.fixedSeen == nil {
		c.fixedSeen = map[string]bool{}
	}
	c.fixedSeen[key] = true
	c.fixed = append(c.fixed, html)
}

// wordArt returns the SVG image of the WordArt text, in the box of the
// shape and turned by its rotation, and the style of the image, which
// places it on the page. It returns "" for a shape without a size. The
// text of the WordArt Plain Text stretches to fill the box of the shape.
// The image holds the box after its rotation.
func (c *converter) wordArt(text string) (string, string) {
	b := c.shape.box
	w, h := float64(b[2]-b[0])/20, float64(b[3]-b[1])/20
	if w <= 0 || h <= 0 || w > 14400 || h > 14400 {
		return "", ""
	}
	rot := float64(c.shapeInt("rotation", 0)) / 65536
	rot = math.Mod(math.Mod(rot, 360)+360, 360)
	rad := rot * math.Pi / 180
	cos, sin := math.Abs(math.Cos(rad)), math.Abs(math.Sin(rad))
	bw, bh := w*cos+h*sin, w*sin+h*cos
	fill := c.shapeColor("fillColor", "#000000")
	opacity := ""
	if v := c.shapeInt("fillOpacity", 65536); v >= 0 && v < 65536 {
		opacity = ` fill-opacity="` + num(float64(v)/65536) + `"`
	}
	family := "sans-serif"
	if f := strings.TrimSpace(c.shape.props["gtextFont"]); f != "" {
		family = "'" + strings.NewReplacer("'", "", "<", "", "&", "", `"`, "").Replace(f) + "', sans-serif"
	}
	var t strings.Builder
	escape(&t, text, false)
	// the line of the text, from its ascent to its descent, fills the
	// height of the box
	size := h / 1.2
	svg := fmt.Sprintf(`<svg xmlns="http://www.w3.org/2000/svg" width="%spt" height="%spt" viewBox="0 0 %s %s">`+
		`<g transform="translate(%s %s) rotate(%s) translate(%s %s)">`+
		`<text x="0" y="%s" font-family="%s" font-size="%s" textLength="%s" lengthAdjust="spacingAndGlyphs" fill="%s"%s>%s</text>`+
		`</g></svg>`,
		num(bw), num(bh), num(bw), num(bh), num(bw/2), num(bh/2), num(rot), num(-w/2), num(-h/2),
		num(h*0.8), attrText(family), num(size), num(w), fill, opacity, t.String())
	var s css
	s.set("position", "fixed")
	left, top := c.shapePlace(w, h, bw, bh)
	s.set("left", num(left)+"pt")
	s.set("top", num(top)+"pt")
	s.set("width", num(bw)+"pt")
	s.set("height", num(bh)+"pt")
	if c.shapeInt("fBehindDocument", 0) != 0 {
		s.set("z-index", "-1")
	}
	return svg, s.String()
}

// shapePlace returns the place of the image of a shape of w by h points,
// whose image after its rotation has bw by bh points, from the top left
// corner of the area of the page inside its margins. The properties posh
// and posv align the shape on the margin, posrelh and posrelv 0, or on the
// page, posrelh and posrelv 1. Otherwise \shpleft and \shptop place it.
func (c *converter) shapePlace(w, h, bw, bh float64) (float64, float64) {
	sec := c.section
	pageW, pageH := float64(sec.width)/20, float64(sec.height)/20
	left, right := float64(sec.left)/20, float64(sec.right)/20
	top, bottom := float64(sec.top)/20, float64(sec.bottom)/20
	place := func(pos, rel, offset int, size, page, before, after float64) float64 {
		origin, area := 0.0, page-before-after
		if rel == 1 {
			origin, area = -before, page
		}
		switch pos {
		case 1:
			return origin
		case 2:
			return origin + (area-size)/2
		case 3:
			return origin + area - size
		}
		return origin + float64(offset)/20
	}
	// the image of the rotated shape has the same center as the shape
	x := place(c.shapeInt("posh", 0), c.shapeInt("posrelh", 0), c.shape.box[0], w, pageW, left, right) - (bw-w)/2
	y := place(c.shapeInt("posv", 0), c.shapeInt("posrelv", 0), c.shape.box[1], h, pageH, top, bottom) - (bh-h)/2
	return x, y
}

// shapeColor returns the color of the property name of the shape, a color
// of RGB in the order of its bytes, blue, green and red, or def for a
// color of a scheme or of the system.
func (c *converter) shapeColor(name, def string) string {
	v := c.shapeInt(name, -1)
	if v < 0 || v>>24 != 0 {
		return def
	}
	return fmt.Sprintf("#%02x%02x%02x", v&0xFF, v>>8&0xFF, v>>16&0xFF)
}

// shapeEMU returns the property name of the shape, a length in EMU, as
// points from 0 to most.
func (c *converter) shapeEMU(name string, def int, most float64) float64 {
	return min(max(float64(c.shapeInt(name, def))/12700, 0), most)
}

// floatsShape reports whether a shape in the group s floats on the page: a
// shape in the text of the body, not in a footnote, a header, a footer, a
// text box or a group of shapes. Another shape stays in its line.
func (c *converter) floatsShape(s *state) bool {
	return s.dest == destText && s.target == c.body && !s.shapeGroup && !c.inNote()
}

// shapeParent returns the group around the current shape: the group of
// the innermost \shpinst on the stack, or nil.
func (c *converter) shapeParent() *state {
	for i := len(c.stack) - 1; i > 0; i-- {
		if c.stack[i].dest == destShape && c.stack[i-1].dest != destShape {
			return c.stack[i-1]
		}
	}
	return nil
}

// floatShape writes the picture or the text box of the shape into the
// paragraph of parent. A picture covers the text of its shape, as in the
// package msdoc. A shape that floats takes its place and its wrap from
// floatCSS, and goes to the start of its paragraph, since Word places it
// from the top of the paragraph, wherever its anchor is in the text.
// Another shape stays in its line, with the size of its box.
func (c *converter) floatShape(parent *state) {
	sh := &c.shape
	if sh.pic == "" && !sh.hasText {
		return
	}
	w, h := sh.box[2]-sh.box[0], sh.box[3]-sh.box[1]
	box := w > 0 && h > 0 && w <= maxShapeSize && h <= maxShapeSize
	var s css
	place := c.inline
	switch {
	case box && c.floatsShape(parent) && c.shapeInt("fPseudoInline", 0) == 0:
		c.floatCSS(&s, float64(w)/20, float64(h)/20, sh.pic != "")
		place = c.paraStart
	case sh.pic != "":
		if !box {
			w, h = sh.picW, sh.picH
		}
	default:
		s.set("display", "inline-block")
	}
	if sh.pic != "" {
		sizeCSS(&s, w, h)
		place(parent, imgHTML(sh.pic, s.String()))
		return
	}
	if box {
		sizeCSS(&s, w, h)
	}
	c.textBoxCSS(&s)
	place(parent, `<span style="`+attrText(s.String())+`">`+sh.text+`</span>`)
}

// paraStart writes an element at the start of the current paragraph of s,
// after the floats before it. The marker of a list stays before the text.
func (c *converter) paraStart(s *state, html string) {
	c.flushBytes(s)
	t := s.target
	c.flushRun(t)
	para := t.para.String()
	at := min(t.floats, len(para))
	t.para.Reset()
	t.para.WriteString(para[:at] + html + para[at:])
	t.floats = at + len(html)
	if t.marker != "" && t.markerAt >= at {
		t.markerAt += len(html)
	}
}

// textBoxCSS sets the declarations of the box of a text box: the insets of
// its text, its fill and its line, with the defaults of MS-ODRAW, a white
// fill and a black line of 0.75 point.
func (c *converter) textBoxCSS(s *css) {
	s.set("box-sizing", "border-box")
	s.set("padding", num(c.shapeEMU("dyTextTop", 45720, 72))+"pt "+num(c.shapeEMU("dxTextRight", 91440, 72))+"pt "+
		num(c.shapeEMU("dyTextBottom", 45720, 72))+"pt "+num(c.shapeEMU("dxTextLeft", 91440, 72))+"pt")
	if c.shapeInt("fFilled", 1) != 0 {
		s.set("background-color", c.shapeColor("fillColor", "#ffffff"))
	}
	if c.shapeInt("fLine", 1) != 0 {
		s.set("border", num(c.shapeEMU("lineWidth", 9525, 72))+"pt solid "+c.shapeColor("lineColor", "#000000"))
	}
}

// floatCSS sets the declarations of a shape of w by h points that floats
// on the page, as the package msdoc places the shape of an FSPA. A shape
// that the text wraps around floats to the side of the column where it is,
// a shape with text above and below is a block, and a shape that the text
// does not wrap is placed on the page. A shape at a position from the
// margin or from the page, with text around it or above and below it,
// floats at that position, in a section of one column (D60). A shape with
// text above and below it then has a margin to the right side of the
// column, so that no line fits beside it. The text wraps around a shape on
// the sides of its \shpwrk, so that the text is on both sides of a shape
// in the middle of the column (D63). pic is set for a picture and not for
// a text box.
func (c *converter) floatCSS(s *css, w, h float64, pic bool) {
	sh := &c.shape
	sec := c.section
	cols := max(sec.cols, 1)
	col := float64(sec.width-sec.left-sec.right-(cols-1)*sec.colSpace) / 20 / float64(cols)
	pageLeft, pageTop := float64(sec.left)/20, float64(sec.top)/20
	bx, by := c.origins()
	x, y := float64(sh.box[0])/20, float64(sh.box[1])/20
	refW := col
	if bx == originPage {
		refW = float64(sec.width) / 20
	}
	// posh aligns the shape on its origin: left, center, right, inside
	// and outside, which the package puts at the left and the right
	switch c.shapeInt("posh", 0) {
	case 1, 4:
		x = 0
	case 2:
		x = (refW - w) / 2
	case 3, 5:
		x = refW - w
	}
	if bx == originPage {
		x -= pageLeft
	}
	// posv aligns the shape on its origin, but not on its paragraph
	posv := c.shapeInt("posv", 0)
	if by == originColumn {
		posv = 0
	}
	position := ""
	if cols == 1 && by != originColumn {
		area := "margin "
		if by == originPage {
			area = "page "
		}
		switch posv {
		case 1, 4:
			position = area + "top"
		case 2:
			position = area + "center"
		case 3, 5:
			position = area + "bottom"
		default:
			position = area + num(y) + "pt"
		}
	}
	wr := sh.wr
	if wr != 1 && wr != 3 && max(x, col-x-w) < 36 {
		// Word puts the text below a shape that leaves less than half an
		// inch beside it
		wr = 1
	}
	distL, distR := c.shapeEMU("dxWrapDistLeft", 114300, 72), c.shapeEMU("dxWrapDistRight", 114300, 72)
	distT, distB := c.shapeEMU("dyWrapDistTop", 0, 72), c.shapeEMU("dyWrapDistBottom", 0, 72)
	switch {
	case wr == 1 && position != "":
		s.set("float", "left")
		s.set("margin", num(distT)+"pt "+num(max(col-x-w, 0))+"pt "+num(distB)+"pt "+num(x)+"pt")
		s.set("-ivy-float-position", position)
	case wr == 1:
		s.set("display", "block")
		s.set("margin-left", num(max(x, 0))+"pt")
		if by == originColumn && y > 0 {
			// Word measures the distance from the top of the paragraph,
			// below the space after the paragraph before it. A picture
			// takes a padding, which does not collapse with that space,
			// as a margin does. A text box holds its insets in its
			// padding, so it takes a margin
			s.set(map[bool]string{true: "padding-top", false: "margin-top"}[pic], num(y)+"pt")
		}
		s.set("margin-bottom", num(distB)+"pt")
	case wr == 3:
		s.set("position", "absolute")
		s.set("left", num(x)+"pt")
		if by == originColumn {
			s.set("margin-top", num(y)+"pt")
		} else {
			// the top from the area of the page inside its margins
			origin, area := 0.0, float64(sec.height-sec.top-sec.bottom)/20
			if by == originPage {
				origin, area = -pageTop, float64(sec.height)/20
			}
			switch posv {
			case 1, 4:
				y = 0
			case 2:
				y = (area - h) / 2
			case 3, 5:
				y = area - h
			}
			s.set("top", num(origin+y)+"pt")
		}
		if sh.behind || c.shapeInt("fBehindDocument", 0) != 0 {
			s.set("z-index", "-1")
		}
	default:
		if x+w/2 > col/2 {
			s.set("float", "right")
			s.set("margin-right", num(max(col-x-w, 0))+"pt")
			s.set("margin-left", num(distL)+"pt")
		} else {
			s.set("float", "left")
			s.set("margin-left", num(max(x, 0))+"pt")
			s.set("margin-right", num(distR)+"pt")
		}
		switch {
		case position != "":
			s.set("margin-top", num(distT)+"pt")
			s.set("-ivy-float-position", position)
		case by == originColumn && y > 0:
			s.set("margin-top", num(y)+"pt")
		}
		s.set("margin-bottom", num(distB)+"pt")
		s.set("-ivy-wrap-flow", wrapSide(sh.wrk)+" "+num(max(distL, distR))+"pt")
	}
}

// wrapSide returns the keyword of -ivy-wrap-flow for the \shpwrk of a
// shape: the side of the text that wraps around it (D63).
func wrapSide(wrk int) string {
	switch wrk {
	case 1:
		return "start"
	case 2:
		return "end"
	case 3:
		return "maximum"
	}
	return "both"
}
