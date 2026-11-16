package odf

import (
	"context"
	"encoding/base64"
	"fmt"
	"log/slog"
	"path"
	"strconv"
	"strings"

	"github.com/xo/okage/roxmltree"

	"github.com/xo/ivy"
	"github.com/xo/ivy/internal/layout"
	"github.com/xo/ivy/internal/memfs"
	"github.com/xo/ivy/internal/ooxml"
)

// odtDocumentName is the path of the XHTML document of a text in the FS of
// the source.
const odtDocumentName = "index.xhtml"

// odtStyleSheet is the style sheet of the texts. The conversion writes the
// formatting of each element in its style attribute, and the rules of each
// master page after this style sheet.
const odtStyleSheet = `html { font-family: serif; font-size: 12pt; line-height: 1.17 }
body { margin: 0; counter-reset: note }
p { margin: 0; white-space: pre-wrap; orphans: 2; widows: 2 }
table { border-collapse: collapse; border-spacing: 0 }
td { padding: 0; vertical-align: top; border: none }
a { color: inherit; text-decoration: none }
.pg::before { content: counter(page) }
.pgs::before { content: counter(pages) }
.fn { float: footnote; counter-increment: note }
::footnote-call { content: counter(note); vertical-align: super; font-size: 0.58em; line-height: inherit }
::footnote-marker { content: none }
.fnref::before { content: counter(note) }
.fnref { padding-right: 0.3em }
.endnotes { margin-top: 12pt }
@page {
  @footnote { margin-top: 6pt; padding-top: 3pt; border-top: 0.5pt solid black }
  @top-center { width: 100%; vertical-align: top; text-align: left }
  @bottom-center { width: 100%; vertical-align: bottom; text-align: left }
}
`

// odtList is a list that the conversion is in: its style, its level and
// whether the next paragraph is the first of an item.
type odtList struct {
	style string
	level int
	// marker is the text of the marker of the next paragraph, or "".
	marker string
	lvl    *odtListLevel
}

// odtConverter converts a text into XHTML.
type odtConverter struct {
	d   *Document
	ctx context.Context
	lim ivy.Limits
	log *slog.Logger

	// w receives the XHTML: the body, or the endnotes.
	w        *strings.Builder
	body     strings.Builder
	endnotes strings.Builder
	sheet    strings.Builder
	fs       memfs.FS

	st *odtStyles
	// files maps the name of an entry to its path in fs, or to "" for an
	// entry that cannot be read.
	files   map[string]string
	objects int
	// charts are the charts that the layout draws, by name.
	charts map[string]layout.Object

	// master is the name of the master page of the current section, and
	// sections counts the sections.
	master   string
	sections int
	// running maps a part of a master page to the name of its running
	// element.
	running map[string]string
	// runningHeights holds the estimated heights of the running elements.
	runningHeights map[string]float64
	// layout is the page layout of the current section.
	layout *odtPageLayout
	// started is set after the first block.
	started bool

	// counters holds the counters of each list style, by level.
	counters map[string]*[10]int
	list     *odtList

	// space is set when the last character of the paragraph is a space,
	// for the collapsing of white space.
	space bool
	// noteMark asks the next paragraph for the mark of its note.
	noteMark   bool
	nested     int
	endnoteNum int

	depth int
	deep  bool
	err   error
}

// convertText converts the text of d into a layout.Source.
func convertText(ctx context.Context, d *Document) (layout.Source, error) {
	c := &odtConverter{
		d: d, ctx: ctx, lim: d.lim, log: d.cfg.Log(),
		st:       newODTStyles(d.lim.MaxDepth),
		files:    map[string]string{},
		running:  map[string]string{},
		counters: map[string]*[10]int{},
		charts:   map[string]layout.Object{},
	}
	c.w = &c.body
	if d.archive.Find("styles.xml") != nil {
		doc, err := c.readXML("styles.xml")
		if err != nil {
			c.log.Warn("odf: styles left out", "err", err)
		} else {
			c.st.read(doc.RootElement())
		}
	}
	doc, err := c.readXML("content.xml")
	if err != nil {
		return layout.Source{}, err
	}
	root := doc.RootElement()
	c.st.read(root)
	text, ok := odtFind(root, "body", "text")
	if !ok {
		return layout.Source{}, fmt.Errorf("content.xml without office:text: %w", ivy.ErrFormat)
	}
	c.startSection(c.firstMaster(text))
	c.blocks(text)
	c.w.WriteString("</div>")
	if c.err != nil {
		return layout.Source{}, c.err
	}
	if c.deep {
		c.log.Warn("odf: content nests deeper than the limit and is left out", "limit", c.lim.MaxDepth)
	}
	var out strings.Builder
	out.WriteString(`<?xml version="1.0" encoding="UTF-8"?>` + "\n")
	out.WriteString(`<html xmlns="http://www.w3.org/1999/xhtml"><head><meta charset="UTF-8"/><title></title></head>` + "\n<body>")
	out.WriteString(c.body.String())
	if c.endnotes.Len() > 0 {
		out.WriteString(`<div class="endnotes">` + c.endnotes.String() + "</div>")
	}
	out.WriteString("</body></html>\n")
	if int64(out.Len()) > c.lim.MaxMemory {
		return layout.Source{}, fmt.Errorf("xhtml of more than %d bytes: %w", c.lim.MaxMemory, ivy.ErrLimit)
	}
	c.fs.Add(odtDocumentName, []byte(out.String()))
	return layout.Source{
		FS:          &c.fs,
		Documents:   []string{odtDocumentName},
		StyleSheets: []string{odtStyleSheet, c.sheet.String()},
		Objects:     c.charts,
	}, nil
}

// odtFind returns the descendant of n at the path of local names.
func odtFind(n roxmltree.Node, names ...string) (roxmltree.Node, bool) {
	for _, name := range names {
		next, ok := odtChild(n, name)
		if !ok {
			return roxmltree.Node{}, false
		}
		n = next
	}
	return n, true
}

// readXML reads and parses an entry of the package, decrypted.
func (c *odtConverter) readXML(name string) (*roxmltree.Document, error) {
	buf, err := c.d.readEntry(c.ctx, name)
	if err != nil {
		return nil, err
	}
	doc, err := ooxml.ParseXML(buf, c.lim)
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", name, err)
	}
	return doc, nil
}

// check records an error if the context ends or the document grows past
// the limit of memory, and returns the first error of the conversion.
func (c *odtConverter) check() error {
	if c.err != nil {
		return c.err
	}
	if err := c.ctx.Err(); err != nil {
		c.err = fmt.Errorf("converting odf: %w", err)
	} else if int64(c.body.Len()+c.endnotes.Len()) > c.lim.MaxMemory {
		c.err = fmt.Errorf("xhtml of more than %d bytes: %w", c.lim.MaxMemory, ivy.ErrLimit)
	}
	return c.err
}

// enter goes one level deeper. It returns false past the limit of depth.
func (c *odtConverter) enter() bool {
	if c.depth >= c.lim.MaxDepth {
		c.deep = true
		return false
	}
	c.depth++
	return true
}

// leave goes one level up.
func (c *odtConverter) leave() {
	c.depth--
}

// blockMaster returns the master page that the block n starts, or "".
func (c *odtConverter) blockMaster(n roxmltree.Node) string {
	switch n.TagName().Name() {
	case "p", "h":
		return c.st.inherited("paragraph", odtAttr(n, "style-name"), func(s *odtStyle) string { return s.master })
	case "table":
		return c.st.inherited("table", odtAttr(n, "style-name"), func(s *odtStyle) string { return s.master })
	}
	return ""
}

// firstMaster returns the master page of the first page: the master page
// of the first block, or the master page Standard, or the first one.
func (c *odtConverter) firstMaster(text roxmltree.Node) string {
	for n := range text.Descendants() {
		if !n.IsElement() {
			continue
		}
		name := n.TagName().Name()
		if name == "p" || name == "h" || name == "table" {
			if m := c.blockMaster(n); m != "" {
				return m
			}
			break
		}
	}
	if _, ok := c.st.masters["Standard"]; ok {
		return "Standard"
	}
	if len(c.st.masterOrder) > 0 {
		return c.st.masterOrder[0]
	}
	return ""
}

// blocks converts the children of n as blocks.
func (c *odtConverter) blocks(n roxmltree.Node) {
	if !c.enter() {
		return
	}
	defer c.leave()
	for e := range n.Children() {
		if c.check() != nil {
			return
		}
		if e.IsElement() {
			c.block(e)
		}
	}
}

// block converts a block of the text.
func (c *odtConverter) block(n roxmltree.Node) {
	name := n.TagName().Name()
	if m := c.blockMaster(n); m != "" && c.started && c.nested == 0 {
		c.w.WriteString("</div>")
		c.startSection(m)
	}
	switch name {
	case "p", "h":
		c.started = true
		c.paragraph(n)
	case "list":
		c.started = true
		c.listBlock(n)
	case "table":
		c.started = true
		c.table(n)
	case "section":
		c.started = true
		c.section(n)
	case "table-of-content", "alphabetical-index", "illustration-index", "bibliography",
		"user-index", "object-index", "table-index", "index-body", "index-title", "deletion":
		if name != "deletion" {
			c.blocks(n)
		}
	case "frame":
		c.started = true
		c.frame(n)
	case "numbered-paragraph":
		for e := range n.Children() {
			if e.IsElement() && (e.TagName().Name() == "p" || e.TagName().Name() == "h") {
				c.paragraph(e)
			}
		}
	}
}

// section converts a text:section, with its columns.
func (c *odtConverter) section(n roxmltree.Node) {
	p := c.st.props("section", odtAttr(n, "style-name"))
	var s odtCSS
	if cols, err := strconv.Atoi(p["column-count"]); err == nil && cols > 1 {
		s.set("columns", strconv.Itoa(min(cols, 16)))
		if gap, ok := odtLength(p["column-gap"]); ok {
			s.set("column-gap", odtPt(gap))
		}
	}
	if v := odtColor(p["background-color"]); v != "" {
		s.set("background-color", v)
	}
	c.w.WriteString(`<div style="` + odtAttrText(s.String()) + `">`)
	c.blocks(n)
	c.w.WriteString("</div>")
}

// startSection opens the element of the pages of a master page, writes its
// running elements and the rules of its pages.
func (c *odtConverter) startSection(master string) {
	c.sections++
	c.master = master
	name := "m" + strconv.Itoa(c.sections)
	mp := c.st.masters[master]
	pl := &odtPageLayout{width: 612, height: 792, top: 72, right: 72, bottom: 72, left: 72, columns: 1}
	if mp != nil {
		if l, ok := c.st.layouts[mp.layout]; ok {
			pl = l
		}
	}
	c.layout = pl
	var s odtCSS
	s.set("page", name)
	if c.sections > 1 {
		s.set("break-before", "page")
	}
	if pl.columns > 1 {
		s.set("columns", strconv.Itoa(pl.columns))
		s.set("column-gap", odtPt(pl.columnGap))
	}
	c.w.WriteString(`<div class="section" style="` + odtAttrText(s.String()) + `">`)
	parts := map[string]string{}
	heights := map[string]float64{}
	if mp != nil {
		for _, part := range [...]string{"header", "footer", "header-left", "footer-left", "header-first", "footer-first"} {
			n, ok := mp.parts[part]
			if !ok || odtAttr(n, "display") == "false" {
				continue
			}
			if running, h, ok := c.headerFooter(n); ok {
				parts[part] = running
				heights[part] = h
			}
		}
	}
	top, bottom := pl.top, pl.bottom
	if _, ok := parts["header"]; ok {
		top += max(pl.headerHeight, heights["header"]) + pl.headerSpace
	}
	if _, ok := parts["footer"]; ok {
		bottom += max(pl.footerHeight, heights["footer"]) + pl.footerSpace
	}
	box := func(part string) string {
		if r, ok := parts[part]; ok {
			return "element(" + r + ")"
		}
		return "none"
	}
	margins := func(header, footer string) string {
		return "@top-center { content: " + box(header) + "; padding-top: " + odtPt(pl.top) + " } " +
			"@bottom-center { content: " + box(footer) + "; padding-bottom: " + odtPt(pl.bottom) + " }"
	}
	w := &c.sheet
	w.WriteString("@page " + name + " { size: " + odtPt(pl.width) + " " + odtPt(pl.height) +
		"; margin: " + odtPt(top) + " " + odtPt(pl.right) + " " + odtPt(bottom) + " " + odtPt(pl.left) + "; " +
		margins("header", "footer") + " }\n")
	if _, ok := parts["header-left"]; ok {
		w.WriteString("@page " + name + ":left { " + margins("header-left", odtPick(parts["footer-left"] != "", "footer-left", "footer")) + " }\n")
	}
	if _, ok := parts["header-first"]; ok {
		w.WriteString("@page " + name + ":nth(1 of " + name + ") { " + margins("header-first", odtPick(parts["footer-first"] != "", "footer-first", "footer")) + " }\n")
	}
}

// odtPick returns a if cond is true, else b.
func odtPick(cond bool, a, b string) string {
	if cond {
		return a
	}
	return b
}

// headerFooter converts a part of a master page into a running element. It
// returns its name and an estimate of its height in points.
func (c *odtConverter) headerFooter(n roxmltree.Node) (string, float64, bool) {
	key := strconv.Itoa(int(n.ID()))
	if name, ok := c.running[key]; ok {
		return name, c.runningHeights[key], name != ""
	}
	name := "hf" + strconv.Itoa(len(c.running)+1)
	c.running[key] = name
	c.w.WriteString(`<div style="position: running(` + name + `)">`)
	c.nested++
	c.blocks(n)
	c.nested--
	c.w.WriteString("</div>")
	// The height of the part is the height of its lines, a line for each
	// paragraph.
	height := 0.0
	for d := range n.Descendants() {
		if d.IsElement() && (d.TagName().Name() == "p" || d.TagName().Name() == "h") {
			p := c.st.props("paragraph", odtAttr(d, "style-name"))
			family, _ := c.st.font(p)
			height += odtFontSize(p) * odtLineRatio(family)
		}
	}
	if c.runningHeights == nil {
		c.runningHeights = map[string]float64{}
	}
	c.runningHeights[key] = height
	return name, height, true
}

// file copies an entry of the package into the FS of the source, and
// returns its path there, or false if the entry cannot be read.
func (c *odtConverter) file(href string) (string, bool) {
	name := path.Clean(strings.TrimPrefix(href, "./"))
	if strings.Contains(href, "://") || strings.HasPrefix(name, "../") || name == "." {
		return "", false
	}
	if p, ok := c.files[name]; ok {
		return p, p != ""
	}
	c.files[name] = ""
	c.objects++
	if c.objects > c.lim.MaxObjects {
		c.log.Warn("odf: too many images, image left out", "entry", name)
		return "", false
	}
	buf, err := c.d.readEntry(c.ctx, name)
	if err != nil {
		c.log.Warn("odf: image left out", "entry", name, "err", err)
		return "", false
	}
	return c.addFile(name, buf), true
}

// addFile adds an image to the FS of the source, and returns its path.
func (c *odtConverter) addFile(name string, buf []byte) string {
	p := "media/" + strconv.Itoa(c.objects) + strings.ToLower(path.Ext(name))
	c.fs.Add(p, buf)
	c.files[name] = p
	return p
}

// binaryFile adds the base64 image of office:binary-data to the FS of the
// source.
func (c *odtConverter) binaryFile(n roxmltree.Node) (string, bool) {
	var b strings.Builder
	for t := range n.Children() {
		if t.IsText() {
			s, _ := t.Text()
			b.WriteString(s)
		}
	}
	clean := strings.Map(func(r rune) rune {
		if r == ' ' || r == '\n' || r == '\r' || r == '\t' {
			return -1
		}
		return r
	}, b.String())
	clean = strings.TrimRight(clean, "=")
	if int64(base64.RawStdEncoding.DecodedLen(len(clean))) > c.lim.MaxMemory {
		return "", false
	}
	buf, err := base64.RawStdEncoding.DecodeString(clean)
	if err != nil {
		c.log.Warn("odf: image of binary data left out", "err", err)
		return "", false
	}
	c.objects++
	if c.objects > c.lim.MaxObjects {
		return "", false
	}
	return c.addFile("binary"+strconv.Itoa(c.objects), buf), true
}
