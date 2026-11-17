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
		case "wpc":
			// A drawing canvas holds its shapes and pictures as a group
			// does, at their places from its corner.
			return c.group(e, w, h)
		case "chart":
			return c.chart(e, w, h)
		case "relIds":
			return c.diagram(e, w, h)
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
		if c.margin != "" {
			c.marginDrawing(g, w, h, x, pv, docAlt(n))
			return
		}
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

// pageTop returns the top of a drawing of a header or a footer in the box
// of its floats, from its top y on the page area. The box of a header is
// at the distance of the header from the top of the paper, and the box of
// a footer at the distance of the footer from the bottom.
func (c *converter) pageTop(y float64) float64 {
	sec := c.sections[c.section]
	if c.margin == "footer" {
		return y + sec.top - (sec.height - min(sec.footer, sec.bottom))
	}
	return y + sec.top - min(sec.header, sec.top)
}

// marginDrawing writes a drawing of a header or a footer that does not take
// part in the flow of the text, such as a picture watermark, at its place on
// the page. x is its place in the column, from its wp:positionH.
func (c *converter) marginDrawing(g roxmltree.Node, w, h, x float64, pv position, alt string) {
	sec := c.sections[c.section]
	areaH := sec.height - sec.top - sec.bottom
	y := pv.offset
	switch pv.relative {
	case "page":
		y -= sec.top
	case "margin", "topMargin":
	default:
		y += c.paragraphTop()
	}
	switch pv.align {
	case "top":
		y = 0
	case "bottom":
		y = areaH - h
	case "center":
		y = (areaH - h) / 2
	}
	var s css
	s.set("position", "absolute")
	s.set("left", pt(x))
	s.set("top", pt(c.pageTop(y)))
	s.set("width", pt(w))
	// Word draws the drawings of a header behind the text of the page
	s.set("z-index", "-1")
	w0 := c.w
	c.w = &c.floats
	defer func() { c.w = w0 }()
	c.w.WriteString(`<div style="` + attrText(s.String()) + `">`)
	c.graphic(g, w, h, alt)
	c.w.WriteString("</div>")
}

// paragraphTop returns the top of the current paragraph of a header or a
// footer on the page area. A drawing that a paragraph places takes it as
// its origin. The top of a header is at the distance of the header from the
// top of the paper, and the bottom of a footer at the distance of the footer
// from the bottom, with the estimate of its height above it.
func (c *converter) paragraphTop() float64 {
	sec := c.sections[c.section]
	if c.margin == "footer" {
		return sec.height - min(sec.footer, sec.bottom) - c.hf.height + c.hfTop - sec.top
	}
	return min(sec.header, sec.top) + c.hfTop - sec.top
}

// vmlStyle returns the declarations of the style of a VML shape, with
// lower case names.
func vmlStyle(n roxmltree.Node) map[string]string {
	m := map[string]string{}
	for decl := range strings.SplitSeq(ooxml.Attr(n, "style"), ";") {
		k, v, ok := strings.Cut(decl, ":")
		if ok && len(m) < 64 {
			m[strings.TrimSpace(strings.ToLower(k))] = strings.TrimSpace(v)
		}
	}
	return m
}

// vmlFraction parses a fraction of VML, such as "0.5" or "19661f", which
// is 19661/65536.
func vmlFraction(s string) (float64, bool) {
	s = strings.TrimSpace(s)
	div := 1.0
	if rest, ok := strings.CutSuffix(s, "f"); ok {
		s, div = rest, 65536
	}
	v, err := strconv.ParseFloat(s, 64)
	if err != nil || v != v || v < -1e9 || v > 1e9 {
		return 0, false
	}
	return v / div, true
}

// marginPict writes a VML shape of a header or a footer whose position is
// absolute, such as a picture watermark, at its place on the page. It
// returns false for a shape that flows with the text.
func (c *converter) marginPict(shape roxmltree.Node, src string, w, h, opacity float64) bool {
	st := vmlStyle(shape)
	if st["position"] != "absolute" || w <= 0 || h <= 0 {
		return false
	}
	sec := c.sections[c.section]
	areaW := sec.width - sec.left - sec.right - sec.gutter
	areaH := sec.height - sec.top - sec.bottom
	x, y := vmlLength(st["margin-left"]), vmlLength(st["margin-top"])
	if v := strings.TrimSpace(st["margin-left"]); strings.HasPrefix(v, "-") {
		x = -vmlLength(v[1:])
	}
	if v := strings.TrimSpace(st["margin-top"]); strings.HasPrefix(v, "-") {
		y = -vmlLength(v[1:])
	}
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
		x = areaW - w
	case "center":
		x = (areaW - w) / 2
	}
	switch st["mso-position-vertical-relative"] {
	case "page":
		y -= sec.top
	case "margin", "top-margin-area":
	default:
		y += c.paragraphTop()
	}
	switch st["mso-position-vertical"] {
	case "top":
		y = 0
	case "bottom":
		y = areaH - h
	case "center":
		y = (areaH - h) / 2
	}
	var s css
	s.set("position", "absolute")
	s.set("left", pt(x))
	s.set("top", pt(c.pageTop(y)))
	s.set("width", pt(w))
	s.set("height", pt(h))
	s.set("z-index", "-1")
	if opacity < 1 {
		s.set("opacity", strconv.FormatFloat(opacity, 'f', 3, 64))
	}
	c.floats.WriteString(`<img src="` + attrText(src) + `" alt="" style="` + attrText(s.String()) + `"/>`)
	return true
}

// washout returns the opacity of a picture of VML with the gain and the
// black level of its v:imagedata. Word washes a watermark out with a gain
// g and a black level of (1-g)/2, which draws the picture as with an
// opacity of g over white.
func washout(d roxmltree.Node) float64 {
	g, ok := vmlFraction(ooxml.Attr(d, "gain"))
	if !ok || !(g > 0 && g < 1) {
		return 1
	}
	if b, ok := vmlFraction(ooxml.Attr(d, "blacklevel")); !ok || b <= 0 {
		return 1
	}
	return max(g, 0.05)
}

// pict converts a w:pict or a w:object element, which hold shapes of VML:
// an image of v:imagedata or the text of v:textbox.
func (c *converter) pict(n roxmltree.Node) {
	for e := range n.Children() {
		if e.IsElement() && c.vmlBox(e) {
			return
		}
	}
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
			opacity := washout(d)
			if c.margin != "" {
				if shape, ok := d.Parent(); ok && c.marginPict(shape, src, w, h, opacity) {
					return
				}
			}
			var s css
			if w > 0 && h > 0 {
				s.set("width", pt(w))
				s.set("height", pt(h))
			}
			if opacity < 1 {
				s.set("opacity", strconv.FormatFloat(opacity, 'f', 3, 64))
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
