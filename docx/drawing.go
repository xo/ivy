package docx

import (
	"strconv"
	"strings"

	"github.com/xo/okage/roxmltree"

	"github.com/xo/ivy/internal/chart"
	"github.com/xo/ivy/internal/drawingml"
	"github.com/xo/ivy/internal/fontdb"
	"github.com/xo/ivy/internal/layout"
	"github.com/xo/ivy/internal/ooxml"
)

// emuPerPoint is the number of English Metric Units in a point.
const emuPerPoint = 12700

// maxDrawingSize bounds the size of a drawing, in points.
const maxDrawingSize = 14400

// emu returns the attribute name of n, in English Metric Units, in points.
func emu(n roxmltree.Node, name string) float64 {
	v := number(ooxml.Attr(n, name), 0) / emuPerPoint
	return min(max(v, -maxDrawingSize), maxDrawingSize)
}

// descendant returns the first descendant element of n with the local name
// name.
func descendant(n roxmltree.Node, name string) (roxmltree.Node, bool) {
	for d := range n.Descendants() {
		if d.IsElement() && d.TagName().Name() == name && d != n {
			return d, true
		}
	}
	return roxmltree.Node{}, false
}

// drawing converts a w:drawing element.
func (c *converter) drawing(d roxmltree.Node) {
	for e := range d.Children() {
		if !e.IsElement() {
			continue
		}
		switch e.TagName().Name() {
		case "inline":
			c.inlineDrawing(e)
		case "anchor":
			c.anchorDrawing(e)
		}
	}
}

// extent returns the size of a wp:inline or a wp:anchor element in points.
func extent(n roxmltree.Node) (float64, float64) {
	if e, ok := child(n, "extent"); ok {
		return max(emu(e, "cx"), 0), max(emu(e, "cy"), 0)
	}
	return 0, 0
}

// graphic writes the content of the a:graphic of a drawing, of width w and
// height h in points. It returns false when the drawing shows nothing that
// the conversion reads.
func (c *converter) graphic(n roxmltree.Node, w, h float64, alt string) bool {
	data, ok := descendant(n, "graphicData")
	if !ok {
		return false
	}
	for e := range data.Children() {
		if !e.IsElement() {
			continue
		}
		switch e.TagName().Name() {
		case "pic":
			return c.picture(e, w, h, alt)
		case "wsp":
			return c.shape(e, w, h)
		case "wgp", "grpSp":
			return c.group(e, w, h)
		case "chart":
			return c.chart(e, w, h)
		case "relIds", "wpc":
			c.log.Debug("docx: diagram or canvas left out")
			return false
		}
	}
	return false
}

// picture writes a pic:pic element as an image of w by h points.
func (c *converter) picture(n roxmltree.Node, w, h float64, alt string) bool {
	blip, ok := descendant(n, "blip")
	if !ok {
		return false
	}
	id := ooxml.Attr(blip, "embed")
	r, ok := c.relationship(id)
	if !ok || r.External {
		return false
	}
	src, ok := c.file(r.Target)
	if !ok {
		return false
	}
	c.w.WriteString(`<img src="` + attrText(src) + `" alt="` + attrText(alt) + `" style="width: ` + pt(w) + `; height: ` + pt(h) + `"/>`)
	return true
}

// shape writes a wps:wsp element of w by h points: its text box, or a box
// with its fill and its line.
func (c *converter) shape(n roxmltree.Node, w, h float64) bool {
	var s css
	s.set("display", "inline-block")
	s.set("box-sizing", "border-box")
	s.set("width", pt(w))
	s.set("min-height", pt(h))
	s.set("vertical-align", "top")
	colors := &drawingml.Colors{Theme: c.theme}
	fill, line := "", ""
	lineWidth := 0.75
	if sp, ok := child(n, "spPr"); ok {
		for e := range sp.Children() {
			if !e.IsElement() {
				continue
			}
			switch e.TagName().Name() {
			case "solidFill":
				if v, ok := colors.In(e); ok {
					fill = v.CSS()
				}
			case "noFill":
				fill = "none"
			case "ln":
				if v := number(ooxml.Attr(e, "w"), 0) / emuPerPoint; v > 0 {
					lineWidth = min(v, 12)
				}
				if _, ok := child(e, "noFill"); ok {
					line = "none"
				} else if f, ok := child(e, "solidFill"); ok {
					if v, ok := colors.In(f); ok {
						line = v.CSS()
					}
				}
			}
		}
	}
	// The style of the shape gives the fill and the line that its
	// properties do not give.
	if st, ok := child(n, "style"); ok {
		if ref, ok := child(st, "fillRef"); ok && fill == "" && ooxml.Attr(ref, "idx") != "0" {
			if v, ok := colors.In(ref); ok {
				fill = v.CSS()
			}
		}
		if ref, ok := child(st, "lnRef"); ok && line == "" && ooxml.Attr(ref, "idx") != "0" {
			if v, ok := colors.In(ref); ok {
				line = v.CSS()
			}
		}
	}
	if fill != "" && fill != "none" {
		s.set("background-color", fill)
	}
	if line != "" && line != "none" {
		s.set("border", pt(lineWidth)+" solid "+line)
	}
	txbx, hasText := descendant(n, "txbxContent")
	if !hasText && (fill == "" || fill == "none") && (line == "" || line == "none") {
		return false
	}
	if body, ok := child(n, "bodyPr"); ok {
		pad := func(name string, def float64) string {
			v := ooxml.Attr(body, name)
			if v == "" {
				return pt(def)
			}
			return pt(max(number(v, 0)/emuPerPoint, 0))
		}
		s.set("padding", pad("tIns", 3.6)+" "+pad("rIns", 7.2)+" "+pad("bIns", 3.6)+" "+pad("lIns", 7.2))
	}
	c.w.WriteString(`<span style="` + attrText(s.String()) + `">`)
	if hasText {
		c.nestedBlocks(txbx, c.part, c.w)
	}
	c.w.WriteString("</span>")
	return true
}

// group writes a group of shapes and pictures of w by h points. Each child
// takes its place in the group from its transform.
func (c *converter) group(n roxmltree.Node, w, h float64) bool {
	if !c.enter() {
		return false
	}
	defer c.leave()
	scaleX, scaleY, offX, offY := 1.0, 1.0, 0.0, 0.0
	if pr, ok := child(n, "grpSpPr"); ok {
		if xfrm, ok := child(pr, "xfrm"); ok {
			if off, ok := child(xfrm, "chOff"); ok {
				offX, offY = emu(off, "x"), emu(off, "y")
			}
			if ext, ok := child(xfrm, "chExt"); ok {
				if cw := emu(ext, "cx"); cw > 0 {
					scaleX = w / cw
				}
				if ch := emu(ext, "cy"); ch > 0 {
					scaleY = h / ch
				}
			}
		}
	}
	c.w.WriteString(`<span style="display: inline-block; position: relative; width: ` + pt(w) + `; height: ` + pt(h) + `">`)
	drew := false
	for e := range n.Children() {
		if !e.IsElement() {
			continue
		}
		name := e.TagName().Name()
		if name != "wsp" && name != "pic" && name != "grpSp" && name != "wgp" {
			continue
		}
		x, y, cw, ch := 0.0, 0.0, 0.0, 0.0
		if xfrm, ok := descendant(e, "xfrm"); ok {
			if off, ok := child(xfrm, "off"); ok {
				x, y = emu(off, "x"), emu(off, "y")
			}
			if ext, ok := child(xfrm, "ext"); ok {
				cw, ch = emu(ext, "cx"), emu(ext, "cy")
			}
		}
		c.w.WriteString(`<span style="position: absolute; left: ` + pt((x-offX)*scaleX) + `; top: ` + pt((y-offY)*scaleY) + `">`)
		cw, ch = max(cw*scaleX, 0), max(ch*scaleY, 0)
		switch name {
		case "wsp":
			drew = c.shape(e, cw, ch) || drew
		case "pic":
			drew = c.picture(e, cw, ch, "") || drew
		default:
			drew = c.group(e, cw, ch) || drew
		}
		c.w.WriteString("</span>")
	}
	c.w.WriteString("</span>")
	return drew
}

// chart writes a chart of DrawingML of w by h points, as an object of the
// layout that internal/chart draws. A chart that cannot be read is logged
// and left out.
func (c *converter) chart(n roxmltree.Node, w, h float64) bool {
	r, ok := c.relationship(ooxml.Attr(n, "id"))
	if !ok || r.External {
		return false
	}
	c.objects++
	if c.objects > c.lim.MaxObjects {
		return false
	}
	buf, err := c.d.doc.ReadPart(r.Target)
	var ch *chart.Chart
	if err == nil {
		ch, err = chart.Parse(buf, chart.Options{Theme: c.theme, Config: c.d.cfg, Fonts: fontdb.Of(c.d.cfg), Width: w, Height: h})
	}
	if err != nil {
		c.log.Warn("docx: chart left out", "part", r.Target, "err", err)
		return false
	}
	name := "chart" + strconv.Itoa(len(c.charts)+1)
	c.charts[name] = ch
	c.w.WriteString(`<img src="` + attrText(layout.ObjectURL(name)) + `" alt="" style="width: ` + pt(w) + `; height: ` + pt(h) + `"/>`)
	return true
}

// inlineDrawing converts a wp:inline element.
func (c *converter) inlineDrawing(n roxmltree.Node) {
	w, h := extent(n)
	g, ok := child(n, "graphic")
	if !ok {
		return
	}
	c.graphic(g, w, h, docAlt(n))
}

// docAlt returns the description of a drawing, for the alternate text of
// its image.
func docAlt(n roxmltree.Node) string {
	if pr, ok := child(n, "docPr"); ok {
		if d := ooxml.Attr(pr, "descr"); d != "" {
			return d
		}
		return ooxml.Attr(pr, "name")
	}
	return ""
}

// position is the position of an anchored drawing on one axis.
type position struct {
	relative, align string
	offset          float64
	set             bool
}

// readPosition reads wp:positionH or wp:positionV.
func readPosition(n roxmltree.Node) position {
	p := position{relative: ooxml.Attr(n, "relativeFrom")}
	for e := range n.Children() {
		if !e.IsElement() {
			continue
		}
		switch e.TagName().Name() {
		case "align":
			s, _ := e.Text()
			p.align = strings.TrimSpace(s)
		case "posOffset":
			s, _ := e.Text()
			p.offset = min(max(number(s, 0)/emuPerPoint, -maxDrawingSize), maxDrawingSize)
			p.set = true
		}
	}
	return p
}

// anchorDrawing converts a wp:anchor element. A drawing that text wraps
// around floats to the side of the column where it is. A drawing that text
// does not wrap is placed on the page. A drawing above and below text is a
// block.
func (c *converter) anchorDrawing(n roxmltree.Node) {
	w, h := extent(n)
	g, ok := child(n, "graphic")
	if !ok {
		return
	}
	var ph, pv position
	wrap := "wrapNone"
	for e := range n.Children() {
		if !e.IsElement() {
			continue
		}
		switch name := e.TagName().Name(); name {
		case "positionH":
			ph = readPosition(e)
		case "positionV":
			pv = readPosition(e)
		case "wrapNone", "wrapSquare", "wrapTight", "wrapThrough", "wrapTopAndBottom":
			wrap = name
		}
	}
	sec := c.sections[c.section]
	colWidth := sec.width - sec.left - sec.right - sec.gutter
	x := ph.offset
	switch ph.relative {
	case "page":
		x -= sec.left + sec.gutter
	case "leftMargin", "insideMargin":
		x -= sec.left
	}
	switch ph.align {
	case "left", "inside":
		x = 0
	case "right", "outside":
		x = colWidth - w
	case "center":
		x = (colWidth - w) / 2
	}
	distL, distR := emu(n, "distL"), emu(n, "distR")
	distT, distB := emu(n, "distT"), emu(n, "distB")
	var s css
	switch wrap {
	case "wrapSquare", "wrapTight", "wrapThrough":
		if x+w/2 > colWidth/2 {
			s.set("float", "right")
			s.set("margin-right", pt(max(colWidth-x-w, 0)))
			s.set("margin-left", pt(distL))
		} else {
			s.set("float", "left")
			s.set("margin-left", pt(max(x, 0)))
			s.set("margin-right", pt(distR))
		}
		s.set("margin-bottom", pt(distB))
		if (pv.relative == "paragraph" || pv.relative == "line") && pv.offset > 0 {
			s.set("margin-top", pt(pv.offset))
		}
	case "wrapTopAndBottom":
		s.set("display", "block")
		s.set("margin-left", pt(max(x, 0)))
		s.set("margin-top", pt(distT))
		s.set("margin-bottom", pt(distB))
	default:
		s.set("position", "absolute")
		s.set("left", pt(x))
		switch pv.relative {
		case "page":
			s.set("top", pt(pv.offset-sec.top))
		case "margin", "topMargin":
			s.set("top", pt(pv.offset))
		default:
			if pv.offset != 0 {
				s.set("margin-top", pt(pv.offset))
			}
		}
		if v := ooxml.Attr(n, "behindDoc"); v == "1" || v == "true" {
			s.set("z-index", "-1")
		}
	}
	s.set("width", pt(w))
	c.w.WriteString(`<span style="` + attrText(s.String()) + `">`)
	c.graphic(g, w, h, docAlt(n))
	c.w.WriteString("</span>")
}

// vmlLength returns a length of the style of a VML shape, such as "72pt"
// or "1in", in points.
func vmlLength(s string) float64 {
	s = strings.TrimSpace(s)
	for _, u := range [...]struct {
		suffix string
		scale  float64
	}{{"pt", 1}, {"in", 72}, {"cm", 72 / 2.54}, {"mm", 7.2 / 2.54}, {"px", 0.75}, {"pc", 12}} {
		if rest, ok := strings.CutSuffix(s, u.suffix); ok {
			v, err := strconv.ParseFloat(strings.TrimSpace(rest), 64)
			if err != nil || v != v {
				return 0
			}
			return min(max(v*u.scale, 0), maxDrawingSize)
		}
	}
	v, err := strconv.ParseFloat(s, 64)
	if err != nil || v != v {
		return 0
	}
	// A length without a unit is in EMU in VML.
	return min(max(v/emuPerPoint, 0), maxDrawingSize)
}

// pict converts a w:pict or a w:object element, which hold shapes of VML:
// an image of v:imagedata or the text of v:textbox.
func (c *converter) pict(n roxmltree.Node) {
	for d := range n.Descendants() {
		if !d.IsElement() {
			continue
		}
		switch d.TagName().Name() {
		case "imagedata":
			r, ok := c.relationship(ooxml.Attr(d, "id"))
			if !ok || r.External {
				continue
			}
			src, ok := c.file(r.Target)
			if !ok {
				continue
			}
			w, h := vmlSize(d)
			var s css
			if w > 0 && h > 0 {
				s.set("width", pt(w))
				s.set("height", pt(h))
			}
			c.w.WriteString(`<img src="` + attrText(src) + `" alt="" style="` + attrText(s.String()) + `"/>`)
			return
		case "txbxContent":
			w, _ := vmlSize(d)
			var s css
			s.set("display", "inline-block")
			s.set("vertical-align", "top")
			if w > 0 {
				s.set("width", pt(w))
			}
			c.w.WriteString(`<span style="` + attrText(s.String()) + `">`)
			c.nestedBlocks(d, c.part, c.w)
			c.w.WriteString("</span>")
			return
		}
	}
}

// vmlSize returns the size of the VML shape that holds n, from its style.
func vmlSize(n roxmltree.Node) (float64, float64) {
	for a := range n.Ancestors() {
		if !a.IsElement() {
			continue
		}
		style := ooxml.Attr(a, "style")
		if style == "" {
			continue
		}
		var w, h float64
		for decl := range strings.SplitSeq(style, ";") {
			k, v, ok := strings.Cut(decl, ":")
			if !ok {
				continue
			}
			switch strings.TrimSpace(strings.ToLower(k)) {
			case "width":
				w = vmlLength(v)
			case "height":
				h = vmlLength(v)
			}
		}
		if w > 0 || h > 0 {
			return w, h
		}
	}
	return 0, 0
}
