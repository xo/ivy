package rtf

import (
	"fmt"
	"math"
	"strconv"
	"strings"
)

// This file draws the WordArt of a shape, \shp: a shape whose property
// gtextUNICODE holds a text, such as the watermark that Word puts in a
// header (RTF 1.9.1, "Word 97 through Word 2007 RTF for Drawing Objects
// (Shapes)", and MS-ODRAW 2.3.22).

// The limits of the properties of a shape: their number and the size of a
// value.
const (
	maxShapeProps     = 256
	maxShapeValueSize = 4096
)

// shapeValue adds text to the value of the current property of a shape.
func (c *converter) shapeValue(text string) {
	name := strings.TrimSpace(c.shapeProp)
	if c.shapeProps == nil {
		c.shapeProps = map[string]string{}
	}
	v, ok := c.shapeProps[name]
	if !ok && len(c.shapeProps) >= maxShapeProps || len(v)+len(text) > maxShapeValueSize {
		return
	}
	c.shapeProps[name] = v + text
}

// shapeInt returns the property name of the shape as a number, or def.
func (c *converter) shapeInt(name string, def int) int {
	v, err := strconv.Atoi(strings.TrimSpace(c.shapeProps[name]))
	if err != nil {
		return def
	}
	return v
}

// endShape draws the WordArt of the shape that a group \shpinst ended in
// the group parent. A WordArt of a header or a footer, such as a
// watermark, shows on each page of the section, and waits for the element
// of the section. A WordArt of the body shows at its place.
func (c *converter) endShape(parent *state) {
	text := strings.TrimSpace(c.shapeProps["gtextUNICODE"])
	if text == "" || c.shapeInt("fGtext", 1) == 0 || parent.dest != destText || parent.chr.hidden {
		return
	}
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
	b := c.shapeBox
	w, h := float64(b[2]-b[0])/20, float64(b[3]-b[1])/20
	if w <= 0 || h <= 0 || w > 14400 || h > 14400 {
		return "", ""
	}
	rot := float64(c.shapeInt("rotation", 0)) / 65536
	rot = math.Mod(math.Mod(rot, 360)+360, 360)
	rad := rot * math.Pi / 180
	cos, sin := math.Abs(math.Cos(rad)), math.Abs(math.Sin(rad))
	bw, bh := w*cos+h*sin, w*sin+h*cos
	fill := "#000000"
	if v := c.shapeInt("fillColor", -1); v >= 0 && v>>24 == 0 {
		fill = fmt.Sprintf("#%02x%02x%02x", v&0xFF, v>>8&0xFF, v>>16&0xFF)
	}
	opacity := ""
	if v := c.shapeInt("fillOpacity", 65536); v >= 0 && v < 65536 {
		opacity = ` fill-opacity="` + num(float64(v)/65536) + `"`
	}
	family := "sans-serif"
	if f := strings.TrimSpace(c.shapeProps["gtextFont"]); f != "" {
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
	x := place(c.shapeInt("posh", 0), c.shapeInt("posrelh", 0), c.shapeBox[0], w, pageW, left, right) - (bw-w)/2
	y := place(c.shapeInt("posv", 0), c.shapeInt("posrelv", 0), c.shapeBox[1], h, pageH, top, bottom) - (bh-h)/2
	return x, y
}
