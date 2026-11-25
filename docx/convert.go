package docx

import (
	"context"
	"fmt"
	"log/slog"
	"path"
	"strconv"
	"strings"

	"github.com/xo/okage/roxmltree"

	"github.com/xo/ivy"
	"github.com/xo/ivy/internal/drawingml"
	"github.com/xo/ivy/internal/formula"
	"github.com/xo/ivy/internal/layout"
	"github.com/xo/ivy/internal/memfs"
	"github.com/xo/ivy/internal/ooxml"
)

// documentName is the path of the XHTML document in the FS of the source.
const documentName = "index.xhtml"

// styleSheet is the style sheet of the format. The conversion writes the
// formatting of each element in its style attribute, and the rules of each
// section after this style sheet.
const styleSheet = `html { font-family: serif; font-size: 10pt; line-height: 1.17; -ivy-page-overflow: descent; -ivy-page-top-space: discard }
body { margin: 0; counter-reset: note; -ivy-text-autospace: ideograph-alpha ideograph-numeric }
p { margin: 0; white-space: pre-wrap; orphans: 2; widows: 2 }
table { border-collapse: collapse; border-spacing: 0 }
td { padding: 0; vertical-align: top; border: none }
a { color: inherit; text-decoration: none }
img { vertical-align: baseline }
.pg::before { content: counter(page) }
.pgs::before { content: counter(pages) }
.fn { float: footnote; counter-increment: note }
::footnote-call { content: counter(note); vertical-align: super; font-size: 0.65em; line-height: inherit }
::footnote-marker { content: none }
.fnref::before { content: counter(note) }
.endnote-ref { vertical-align: super; font-size: 0.65em }
.endnotes { break-before: auto; margin-top: 12pt; border-top: 0.5pt solid black; padding-top: 6pt; -ivy-line-numbers: none }
@page {
  @footnote { margin-top: 6pt; padding-top: 3pt; border-top: 0.5pt solid black }
  @top-center { width: 100%; vertical-align: top; text-align: left }
  @bottom-center { width: 100%; vertical-align: bottom; text-align: left }
}
`

// converter converts the main part of a Word file into XHTML.
type converter struct {
	d   *Document
	ctx context.Context
	lim ivy.Limits
	log *slog.Logger

	// w receives the XHTML: the body, or the endnotes.
	w *strings.Builder
	// body is the XHTML of the body.
	body strings.Builder
	fs   memfs.FS
	// sheet holds the rules of the sections.
	sheet strings.Builder

	styles    *styles
	numbering *numbering
	theme     *drawingml.Theme
	// generic maps the name of a font to its generic family, from the font
	// table.
	generic map[string]string
	fontCache
	// defaultTab is the distance of the default tab stops, in points.
	defaultTab float64
	// evenOdd is set when the pages have even and odd headers.
	evenOdd bool
	// themeLang is the East Asian language that selects the East Asian
	// fonts of the theme: the w:eastAsia of w:themeFontLang, or else of the
	// w:lang of the default run properties.
	themeLang string

	// part is the part that the conversion reads, for its relationships.
	part string
	rels map[string]map[string]ooxml.Relationship

	footnotes, endnotes map[string]roxmltree.Node
	// footnotePart and endnotePart are the parts of the notes.
	footnotePart, endnotePart string
	// endnoteText holds the endnotes that the document references, which
	// come at its end.
	endnoteText strings.Builder
	endnoteNum  int

	// files maps the name of a part to its path in fs, or to "" for a part
	// that cannot be read.
	files map[string]string
	// objects counts the files and the objects of the conversion.
	objects int
	// charts are the charts that the layout draws, by name.
	charts map[string]layout.Object
	// typesetter lays out the equations, and is nil before the first
	// equation.
	typesetter *formula.Typesetter

	sections []*section
	// section is the index of the current section.
	section int
	// running maps the name of a part of a header or a footer to the name
	// of its running element.
	running map[string]string
	// margin is "header" or "footer" while the conversion reads a header or
	// a footer, and "" otherwise. The drawings of the part that float on
	// the page, such as a watermark, go to floats.
	margin string
	floats strings.Builder
	// hfHeights holds the estimate of the height of each header and each
	// footer, by the name of its running element.
	hfHeights map[string]hfHeight
	// hf is the estimate of the header or the footer that the conversion
	// reads, and hfTop the top of its current paragraph in it.
	hf    hfHeight
	hfTop float64
	// lineParent is the properties of the runs of the paragraph that the
	// conversion is in, with the font of its largest run, which its
	// element has. It is nil out of a paragraph.
	lineParent props

	// fields is the stack of the complex fields that are open.
	fields []*field
	// open holds the inline elements that are open in the current
	// paragraph, which a page break closes and opens again.
	open []tag
	// para is the start tag of the current paragraph, or "".
	para string
	// paraSeq counts the paragraphs.
	paraSeq int
	// nested counts the notes and the text boxes that the conversion is
	// in. A page break there is a line break.
	nested int

	depth int
	deep  bool
	err   error
}

// field is a complex field, from its w:fldChar begin to its w:fldChar end.
type field struct {
	instr strings.Builder
	// result is set after w:fldChar separate.
	result bool
	// hide hides the result, for a field that the conversion writes itself,
	// such as PAGE.
	hide bool
	// link is the number of the paragraph that opened the link of a field
	// HYPERLINK, or 0.
	link int
	// form is the form field of the field, from its w:ffData, or nil.
	form *formField
}

// convert converts the document into a layout.Source.
func (d *Document) convert(ctx context.Context) (layout.Source, error) {
	c := &converter{
		d: d, ctx: ctx, lim: d.lim, log: d.log,
		generic:    map[string]string{},
		fontCache:  fontCache{fonts: map[string]bool{}},
		defaultTab: 36,
		rels:       map[string]map[string]ooxml.Relationship{},
		files:      map[string]string{},
		running:    map[string]string{},
		charts:     map[string]layout.Object{},
		hfHeights:  map[string]hfHeight{},
	}
	c.w = &c.body
	main, err := d.doc.MainPart(d.lim)
	if err != nil {
		return layout.Source{}, err
	}
	doc, err := c.readXML(main)
	if err != nil {
		return layout.Source{}, fmt.Errorf("reading %s: %w", main, err)
	}
	c.readParts(main)
	body, ok := child(doc.RootElement(), "body")
	if !ok {
		return layout.Source{}, fmt.Errorf("%s without a body: %w", main, ivy.ErrFormat)
	}
	c.part = main
	c.convertBody(body)
	if c.err != nil {
		return layout.Source{}, c.err
	}
	if c.deep {
		c.log.Warn("docx: content nests deeper than the limit and is left out", "limit", c.lim.MaxDepth)
	}
	var out strings.Builder
	out.WriteString(`<?xml version="1.0" encoding="UTF-8"?>` + "\n")
	out.WriteString(`<html xmlns="http://www.w3.org/1999/xhtml"><head><meta charset="UTF-8"/><title>`)
	info, _ := d.doc.Describe(ctx)
	escape(&out, info.Title, false)
	out.WriteString("</title></head>\n<body>")
	out.WriteString(c.body.String())
	if c.endnoteText.Len() > 0 {
		out.WriteString(`<div class="endnotes">`)
		out.WriteString(c.endnoteText.String())
		out.WriteString("</div>")
	}
	out.WriteString("</body></html>\n")
	if int64(out.Len()) > c.lim.MaxMemory {
		return layout.Source{}, fmt.Errorf("xhtml of more than %d bytes: %w", c.lim.MaxMemory, ivy.ErrLimit)
	}
	c.fs.Add(documentName, []byte(out.String()))
	return layout.Source{
		FS:          &c.fs,
		Documents:   []string{documentName},
		StyleSheets: []string{styleSheet, c.sheet.String()},
		Title:       info.Title,
		Objects:     c.charts,
	}, nil
}

// readXML reads and parses the part name.
func (c *converter) readXML(name string) (*roxmltree.Document, error) {
	buf, err := c.d.doc.ReadPart(name)
	if err != nil {
		return nil, err
	}
	return ooxml.ParseXML(buf, c.lim)
}

// readParts reads the parts that the main part relates to: the styles, the
// lists, the theme, the settings, the font table, the footnotes and the
// endnotes. A part that is missing or damaged is logged and left out.
func (c *converter) readParts(main string) {
	rels, err := c.d.doc.Relationships(main, c.lim)
	if err != nil {
		c.log.Warn("docx: relationships of the main part left out", "err", err)
	}
	var stylesDoc, numberingDoc *roxmltree.Document
	for _, r := range rels {
		if r.External {
			continue
		}
		var kind string
		for _, k := range [...]string{"styles", "numbering", "theme", "settings", "fontTable", "footnotes", "endnotes"} {
			if r.TypeIs(k) {
				kind = k
			}
		}
		if kind == "" {
			continue
		}
		doc, err := c.readXML(r.Target)
		if err != nil {
			c.log.Warn("docx: part left out", "part", r.Target, "err", err)
			continue
		}
		switch kind {
		case "styles":
			stylesDoc = doc
		case "numbering":
			numberingDoc = doc
		case "theme":
			if th, err := drawingml.ParseTheme(doc); err == nil {
				c.theme = th
			}
		case "settings":
			c.settings(doc.RootElement())
		case "fontTable":
			c.fontTable(doc.RootElement())
		case "footnotes", "endnotes":
			notes := map[string]roxmltree.Node{}
			for n := range doc.RootElement().Children() {
				if n.IsElement() && (n.TagName().Name() == "footnote" || n.TagName().Name() == "endnote") {
					notes[ooxml.Attr(n, "id")] = n
				}
			}
			if kind == "footnotes" {
				c.footnotes, c.footnotePart = notes, r.Target
			} else {
				c.endnotes, c.endnotePart = notes, r.Target
			}
		}
	}
	c.styles = parseStyles(stylesDoc, c.lim.MaxDepth)
	c.numbering = parseNumbering(numberingDoc)
	if c.themeLang == "" {
		c.themeLang = c.styles.rDefault["lang.ea"]
	}
}

// settings reads the settings that the conversion uses.
func (c *converter) settings(root roxmltree.Node) {
	for n := range root.Children() {
		if !n.IsElement() {
			continue
		}
		switch n.TagName().Name() {
		case "defaultTabStop":
			if v := number(ooxml.Attr(n, "val"), 0); v > 0 {
				c.defaultTab = v / 20
			}
		case "evenAndOddHeaders":
			v := ooxml.Attr(n, "val")
			c.evenOdd = v == "" || v == "1" || v == "true" || v == "on"
		case "themeFontLang":
			c.themeLang = ooxml.Attr(n, "eastAsia")
		}
	}
}

// fontTable reads the generic family of each font of the font table.
func (c *converter) fontTable(root roxmltree.Node) {
	for n := range root.Children() {
		if !n.IsElement() || n.TagName().Name() != "font" {
			continue
		}
		name := ooxml.Attr(n, "name")
		for e := range n.Children() {
			if !e.IsElement() {
				continue
			}
			switch e.TagName().Name() {
			case "family":
				c.generic[strings.ToLower(name)] = map[string]string{
					"roman": "serif", "swiss": "sans-serif", "modern": "monospace",
					"script": "cursive", "decorative": "fantasy",
				}[ooxml.Attr(e, "val")]
			case "pitch":
				if ooxml.Attr(e, "val") == "fixed" {
					c.generic[strings.ToLower(name)] = "monospace"
				}
			}
		}
	}
}

// check records an error if the context ends or the document grows past
// the limit of memory, and returns the first error of the conversion.
func (c *converter) check() error {
	if c.err != nil {
		return c.err
	}
	if err := c.ctx.Err(); err != nil {
		c.err = fmt.Errorf("converting docx: %w", err)
	} else if int64(c.body.Len()+c.endnoteText.Len()) > c.lim.MaxMemory {
		c.err = fmt.Errorf("xhtml of more than %d bytes: %w", c.lim.MaxMemory, ivy.ErrLimit)
	}
	return c.err
}

// enter goes one level deeper into the document. It returns false past the
// limit of depth, and the content there is left out.
func (c *converter) enter() bool {
	if c.depth >= c.lim.MaxDepth {
		c.deep = true
		return false
	}
	c.depth++
	return true
}

// leave goes one level up.
func (c *converter) leave() {
	c.depth--
}

// child returns the first child element of n with the local name name.
func child(n roxmltree.Node, name string) (roxmltree.Node, bool) {
	for c := range n.Children() {
		if c.IsElement() && c.TagName().Name() == name {
			return c, true
		}
	}
	return roxmltree.Node{}, false
}

// relationship returns the relationship id of the current part.
func (c *converter) relationship(id string) (ooxml.Relationship, bool) {
	rels, ok := c.rels[c.part]
	if !ok || rels == nil {
		rels = map[string]ooxml.Relationship{}
		list, err := c.d.doc.Relationships(c.part, c.lim)
		if err != nil {
			c.log.Debug("docx: relationships left out", "part", c.part, "err", err)
		}
		for _, r := range list {
			rels[r.ID] = r
		}
		c.rels[c.part] = rels
	}
	r, ok := rels[id]
	return r, ok
}

// file copies the part name into the FS of the source, and returns its
// path there, or false if the part cannot be read.
func (c *converter) file(name string) (string, bool) {
	if p, ok := c.files[name]; ok {
		return p, p != ""
	}
	c.files[name] = ""
	c.objects++
	if c.objects > c.lim.MaxObjects {
		c.log.Warn("docx: too many images, image left out", "part", name)
		return "", false
	}
	buf, err := c.d.doc.ReadPart(name)
	if err != nil {
		c.log.Warn("docx: image left out", "part", name, "err", err)
		return "", false
	}
	p := "media/" + strconv.Itoa(c.objects) + strings.ToLower(path.Ext(name))
	c.fs.Add(p, buf)
	c.files[name] = p
	return p, true
}
