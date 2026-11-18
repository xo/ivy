package docx

import (
	"math"
	"strconv"
	"strings"

	"github.com/xo/okage/roxmltree"

	"github.com/xo/ivy"
	"github.com/xo/ivy/internal/ooxml"
)

// section is a section of a document, from its w:sectPr: the page, the
// columns and the references to the headers and the footers.
type section struct {
	// width, height and the margins are in points.
	width, height                                    float64
	top, right, bottom, left, header, footer, gutter float64
	cols                                             int
	colSpace                                         float64
	// linePitch is the pitch of the lines of the grid of the document in
	// points, from a w:docGrid of the type lines, or 0 without a grid of
	// lines.
	linePitch float64
	// typ is the type of the break that starts the section: nextPage,
	// continuous, evenPage, oddPage or nextColumn.
	typ     string
	titlePg bool
	// refs maps "header.default", "footer.first" and the like to the id of
	// the relationship of the part.
	refs map[string]string
	// name is the name of the page of the section in the style sheet.
	name string
	// running maps "header.default" and the like to the name of the running
	// element that the section shows.
	running map[string]string
	// borders are the borders of the pages, from w:pgBorders: top, right,
	// bottom and left, as "val sz color space", or "".
	borders [4]string
	// bordersFrom is "page" when the space of a border of the page is from
	// the edge of the page, and "text" from the text. bordersOn is
	// "allPages", "firstPage" or "notFirstPage".
	bordersFrom, bordersOn string
	// bordersShadow is set when the right or the bottom border of the
	// pages has a shadow.
	bordersShadow bool
	// bordersName is the name of the running element of the borders, or "".
	bordersName string
}

// parseSection reads a w:sectPr element. A missing value takes the value
// of defaultSection. paper is the paper of the program.
func parseSection(n roxmltree.Node, paper ivy.Paper) *section {
	s := defaultSection(paper)
	twips := func(e roxmltree.Node, name string, def float64) float64 {
		v := number(ooxml.Attr(e, name), math.NaN())
		if v != v {
			return def
		}
		return math.Abs(v) / 20
	}
	for c := range n.Children() {
		if !c.IsElement() {
			continue
		}
		switch c.TagName().Name() {
		case "pgSz":
			s.width = twips(c, "w", s.width)
			s.height = twips(c, "h", s.height)
			if ooxml.Attr(c, "orient") == "landscape" && s.width < s.height {
				s.width, s.height = s.height, s.width
			}
		case "pgMar":
			s.top = twips(c, "top", s.top)
			s.right = twips(c, "right", s.right)
			s.bottom = twips(c, "bottom", s.bottom)
			s.left = twips(c, "left", s.left)
			s.header = twips(c, "header", s.header)
			s.footer = twips(c, "footer", s.footer)
			s.gutter = twips(c, "gutter", 0)
		case "cols":
			if v, err := strconv.Atoi(ooxml.Attr(c, "num")); err == nil && v > 1 {
				s.cols = min(v, 16)
			}
			s.colSpace = twips(c, "space", s.colSpace)
		case "type":
			s.typ = ooxml.Attr(c, "val")
		case "docGrid":
			switch ooxml.Attr(c, "type") {
			case "lines", "linesAndChars", "snapToChars":
				if v := number(ooxml.Attr(c, "linePitch"), 0) / 20; v >= 1 && v <= 1584 {
					s.linePitch = v
				}
			}
		case "titlePg":
			v := ooxml.Attr(c, "val")
			s.titlePg = v == "" || v == "1" || v == "true" || v == "on"
		case "pgBorders":
			s.bordersFrom = ooxml.Attr(c, "offsetFrom")
			s.bordersOn = ooxml.Attr(c, "display")
			p := props{}
			borders(p, "pg.", c)
			for i, side := range [...]string{"top", "right", "bottom", "left"} {
				s.borders[i] = p["pg."+side]
			}
			for e := range c.Children() {
				if e.IsElement() && (e.TagName().Name() == "right" || e.TagName().Name() == "bottom") {
					switch ooxml.Attr(e, "shadow") {
					case "1", "true", "on":
						s.bordersShadow = true
					}
				}
			}
		case "headerReference", "footerReference":
			kind := strings.TrimSuffix(c.TagName().Name(), "Reference")
			typ := ooxml.Attr(c, "type")
			if typ == "" {
				typ = "default"
			}
			s.refs[kind+"."+typ] = ooxml.Attr(c, "id")
		}
	}
	// A page smaller than 1 inch or larger than 200 inches is damaged.
	s.width = min(max(s.width, 72), 14400)
	s.height = min(max(s.height, 72), 14400)
	return s
}

// defaultSection returns the section of a document without w:sectPr.
// ECMA-376 Part 1, 17.6.13, gives no size to a page without w:pgSz, or
// without its attributes w and h. Word takes US Letter in portrait, with
// margins of 1 inch. The paper of the program replaces US Letter, and the
// margins stay (D32).
func defaultSection(paper ivy.Paper) *section {
	s := &section{
		width: 612, height: 792, top: 72, right: 72, bottom: 72, left: 72,
		header: 36, footer: 36, cols: 1, colSpace: 36, typ: "nextPage",
		refs: map[string]string{}, running: map[string]string{},
	}
	if paper.Width > 0 && paper.Height > 0 {
		s.width = min(max(float64(paper.Width), 72), 14400)
		s.height = min(max(float64(paper.Height), 72), 14400)
	}
	return s
}

// convertBody converts w:body. Each w:sectPr of a paragraph ends a section, and the
// w:sectPr of the body is the last section.
func (c *converter) convertBody(body roxmltree.Node) {
	ends := map[roxmltree.NodeID]bool{}
	for n := range body.Children() {
		if !n.IsElement() {
			continue
		}
		switch n.TagName().Name() {
		case "p":
			if ppr, ok := child(n, "pPr"); ok {
				if sp, ok := child(ppr, "sectPr"); ok {
					c.sections = append(c.sections, parseSection(sp, c.d.cfg.Paper))
					ends[n.ID()] = true
				}
			}
		case "sectPr":
			c.sections = append(c.sections, parseSection(n, c.d.cfg.Paper))
		}
	}
	if len(c.sections) == 0 || len(ends) == len(c.sections) {
		c.sections = append(c.sections, defaultSection(c.d.cfg.Paper))
	}
	c.startSection(0)
	i := 0
	for n := range body.Children() {
		if c.check() != nil {
			return
		}
		if !n.IsElement() {
			continue
		}
		c.block(n, nil)
		if ends[n.ID()] && i+1 < len(c.sections) {
			c.w.WriteString("</div>")
			i++
			c.startSection(i)
		}
	}
	c.w.WriteString("</div>")
}

// startSection opens the element of section i, writes the rules of its
// page, and writes the running elements of its headers and its footers.
func (c *converter) startSection(i int) {
	s := c.sections[i]
	c.section = i
	var prev *section
	if i > 0 {
		prev = c.sections[i-1]
	}
	var style css
	switch {
	case prev != nil && s.typ == "continuous":
		// A continuous section stays on the page, with the page of the
		// section before. It names that page too, so that a page that it
		// starts has the size and the headers of the section before.
		s.name = prev.name
		s.running = prev.running
		style.set("page", s.name)
	default:
		s.name = "s" + strconv.Itoa(i+1)
		style.set("page", s.name)
		switch s.typ {
		case "evenPage":
			style.set("break-before", "left")
		case "oddPage":
			style.set("break-before", "right")
		default:
			if prev != nil {
				style.set("break-before", "page")
			}
		}
	}
	if s.cols > 1 {
		style.set("columns", strconv.Itoa(s.cols))
		style.set("column-gap", pt(s.colSpace))
		if i+1 >= len(c.sections) || c.sections[i+1].typ != "continuous" {
			// Word makes the columns of a section the same height only
			// before a section that continues on the same page. Else the
			// text fills each column to the bottom of the page.
			style.set("column-fill", "auto")
		}
	}
	c.w.WriteString(`<div class="section"`)
	if style.b.Len() > 0 {
		c.w.WriteString(` style="`)
		escape(c.w, style.String(), true)
		c.w.WriteString(`"`)
	}
	c.w.WriteString(">")
	if prev != nil && s.typ == "continuous" {
		return
	}
	// The headers and the footers of a section that does not give its own
	// come from the section before.
	running := map[string]string{}
	if prev != nil {
		for k, v := range prev.running {
			running[k] = v
		}
	}
	for _, key := range [...]string{
		"header.default", "header.first", "header.even",
		"footer.default", "footer.first", "footer.even",
	} {
		id, ok := s.refs[key]
		if !ok {
			continue
		}
		if name, ok := c.headerFooter(id, key[:strings.IndexByte(key, '.')]); ok {
			running[key] = name
		} else {
			delete(running, key)
		}
	}
	s.running = running
	c.pageBorders(s)
	c.pageRules(s)
}

// headerFooter converts the part of a header or a footer into a running
// element, and returns its name. margin is "header" or "footer".
func (c *converter) headerFooter(id, margin string) (string, bool) {
	r, ok := c.relationship(id)
	if !ok || r.External {
		return "", false
	}
	if name, ok := c.running[r.Target]; ok {
		return name, name != ""
	}
	c.running[r.Target] = ""
	doc, err := c.readXML(r.Target)
	if err != nil {
		c.log.Warn("docx: header or footer left out", "part", r.Target, "err", err)
		return "", false
	}
	name := "hf" + strconv.Itoa(len(c.running))
	c.running[r.Target] = name
	part := c.part
	c.part = r.Target
	defer func() { c.part = part }()
	// The drawings that float on the page are in a box of no height at the
	// edge of the margin box that is next to the page area: the top of the
	// box of a header, which aligns its content to the top, and the bottom
	// of the box of a footer, which aligns its content to the bottom.
	sec := c.sections[c.section]
	height, tops := c.partHeight(doc.RootElement(), sec.width-sec.left-sec.right-sec.gutter)
	c.hfHeights[name] = hfHeight{height: height, tops: tops}
	prev, prevHF := c.margin, c.hf
	c.margin, c.hf, c.hfTop = margin, c.hfHeights[name], 0
	c.floats.Reset()
	defer func() { c.margin, c.hf = prev, prevHF }()
	var body strings.Builder
	w := c.w
	c.w = &body
	for n := range doc.RootElement().Children() {
		if n.IsElement() {
			c.block(n, nil)
		}
	}
	c.w = w
	floats := ""
	if c.floats.Len() > 0 {
		floats = `<div style="position: relative; height: 0">` + c.floats.String() + "</div>"
	}
	c.w.WriteString(`<div style="position: running(` + name + `)">`)
	if margin == "header" {
		c.w.WriteString(floats)
	}
	c.w.WriteString(body.String())
	if margin != "header" {
		c.w.WriteString(floats)
	}
	c.w.WriteString("</div>")
	return name, true
}

// pageRules writes the rules of the page of a section.
func (c *converter) pageRules(s *section) {
	box := func(key string) string {
		name, ok := s.running[key]
		if !ok {
			return "none"
		}
		return "element(" + name + ")"
	}
	// header returns the rules of the headers and the footers of the kind.
	header := func(kind string) string {
		return "@top-center { content: " + box("header."+kind) + "; padding-top: " + pt(min(s.header, s.top)) + " } " +
			"@bottom-center { content: " + box("footer."+kind) + "; padding-bottom: " + pt(min(s.footer, s.bottom)) + " }"
	}
	// Word puts the text of a page below its header, and above its
	// footer: the top of the text is the larger of the top margin and the
	// distance of the header from the top of the paper plus the height of
	// the header, and the same holds for the footer at the bottom. The
	// layout makes the margins of a page larger when its header or its
	// footer needs it.
	w := &c.sheet
	w.WriteString("@page " + s.name + " { size: " + pt(s.width) + " " + pt(s.height) +
		"; margin: " + pt(s.top) + " " + pt(s.right) + " " + pt(s.bottom) + " " + pt(s.left+s.gutter) +
		"; -ivy-margin-box-fit: grow; ")
	w.WriteString(header("default"))
	if s.bordersName != "" && s.bordersOn != "firstPage" {
		w.WriteString(" " + c.bordersBox(s.bordersName))
	}
	w.WriteString(" }\n")
	if c.evenOdd {
		w.WriteString("@page " + s.name + ":left { " + header("even") + " }\n")
	}
	if s.titlePg || s.bordersName != "" && s.bordersOn != "" && s.bordersOn != "allPages" {
		first := header(pick(s.titlePg, "first", "default"))
		switch {
		case s.bordersName != "" && s.bordersOn == "firstPage":
			first += " " + c.bordersBox(s.bordersName)
		case s.bordersName != "" && s.bordersOn == "notFirstPage":
			first += " @top-left-corner { content: none }"
		}
		w.WriteString("@page " + s.name + ":nth(1 of " + s.name + ") { " + first + " }\n")
	}
}

// pageBorders writes the running element of the borders of the pages of
// the section s, from w:pgBorders (ECMA-376 Part 1, 17.6.10): a box at
// the space of each border from the edge of the page, or from the text. The
// box is in the margin box at the top left corner of the page, which is at
// the corner of the paper.
func (c *converter) pageBorders(s *section) {
	var lines [4]string
	var space [4]float64
	drawn := false
	for i, v := range s.borders {
		lines[i], space[i] = border(v)
		drawn = drawn || lines[i] != ""
	}
	if !drawn {
		return
	}
	left, top := space[3], space[0]
	right, bottom := s.width-space[1], s.height-space[2]
	if s.bordersFrom != "page" {
		// the space is from the text, outside the margins
		left, top = s.left+s.gutter-space[3], s.top-space[0]
		right, bottom = s.width-s.right+space[1], s.height-s.bottom+space[2]
	}
	if right-left < 1 || bottom-top < 1 {
		return
	}
	var st css
	st.set("position", "absolute")
	st.set("left", pt(left))
	st.set("top", pt(top))
	st.set("width", pt(right-left))
	st.set("height", pt(bottom-top))
	st.set("box-sizing", "border-box")
	for i, side := range [...]string{"top", "right", "bottom", "left"} {
		if lines[i] != "" {
			st.set("border-"+side, lines[i])
		}
	}
	s.bordersName = "pb" + strconv.Itoa(c.section+1)
	c.w.WriteString(`<div style="position: running(` + s.bordersName + `)"><div style="position: relative; height: 0">`)
	if s.bordersShadow {
		// Word draws the shadow of the borders as a black band as wide as
		// the border, outside the right and the bottom borders, moved
		// down and right by that width.
		var sh css
		w := 0.0
		for _, i := range [...]int{1, 2} {
			if f := strings.Fields(s.borders[i]); lines[i] != "" && len(f) == 4 {
				w = max(w, min(max(number(f[1], 4)/8, 0.25), 12))
			}
		}
		sh.set("position", "absolute")
		sh.set("left", pt(left+w))
		sh.set("top", pt(top+w))
		sh.set("width", pt(right-left))
		sh.set("height", pt(bottom-top))
		sh.set("box-sizing", "border-box")
		for _, i := range [...]int{1, 2} {
			if lines[i] != "" {
				sh.set("border-"+[...]string{"top", "right", "bottom", "left"}[i], pt(w)+" solid #000000")
			}
		}
		c.w.WriteString(`<div style="` + attrText(sh.String()) + `"></div>`)
	}
	c.w.WriteString(`<div style="` + attrText(st.String()) + `"></div></div></div>`)
}

// bordersBox returns the rule of the margin box of the borders of the page,
// whose running element is name.
func (c *converter) bordersBox(name string) string {
	return "@top-left-corner { content: element(" + name + "); vertical-align: top; text-align: left; padding: 0 }"
}
