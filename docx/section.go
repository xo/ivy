package docx

import (
	"math"
	"strconv"
	"strings"

	"github.com/xo/okage/roxmltree"

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
}

// parseSection reads a w:sectPr element. A missing value takes the value
// of Word: a page of US Letter with margins of 1 inch.
func parseSection(n roxmltree.Node) *section {
	s := defaultSection()
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
		case "titlePg":
			v := ooxml.Attr(c, "val")
			s.titlePg = v == "" || v == "1" || v == "true" || v == "on"
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
func defaultSection() *section {
	return &section{
		width: 612, height: 792, top: 72, right: 72, bottom: 72, left: 72,
		header: 36, footer: 36, cols: 1, colSpace: 36, typ: "nextPage",
		refs: map[string]string{}, running: map[string]string{},
	}
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
					c.sections = append(c.sections, parseSection(sp))
					ends[n.ID()] = true
				}
			}
		case "sectPr":
			c.sections = append(c.sections, parseSection(n))
		}
	}
	if len(c.sections) == 0 || len(ends) == len(c.sections) {
		c.sections = append(c.sections, defaultSection())
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
		// section before.
		s.name = prev.name
		s.running = prev.running
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
		if name, ok := c.headerFooter(id); ok {
			running[key] = name
		} else {
			delete(running, key)
		}
	}
	s.running = running
	c.pageRules(s)
}

// headerFooter converts the part of a header or a footer into a running
// element, and returns its name.
func (c *converter) headerFooter(id string) (string, bool) {
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
	c.w.WriteString(`<div style="position: running(` + name + `)">`)
	for n := range doc.RootElement().Children() {
		if n.IsElement() {
			c.block(n, nil)
		}
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
	header := func(kind string) string {
		return "@top-center { content: " + box("header."+kind) + "; padding-top: " + pt(min(s.header, s.top)) + " } " +
			"@bottom-center { content: " + box("footer."+kind) + "; padding-bottom: " + pt(min(s.footer, s.bottom)) + " }"
	}
	w := &c.sheet
	w.WriteString("@page " + s.name + " { size: " + pt(s.width) + " " + pt(s.height) +
		"; margin: " + pt(s.top) + " " + pt(s.right) + " " + pt(s.bottom) + " " + pt(s.left+s.gutter) + "; ")
	w.WriteString(header("default"))
	w.WriteString(" }\n")
	if c.evenOdd {
		w.WriteString("@page " + s.name + ":left { " + header("even") + " }\n")
	}
	if s.titlePg {
		w.WriteString("@page " + s.name + ":nth(1 of " + s.name + ") { " + header("first") + " }\n")
	}
}
