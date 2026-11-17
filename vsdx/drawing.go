package vsdx

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"sync"

	"github.com/xo/okage/roxmltree"

	"github.com/xo/ivy"
	"github.com/xo/ivy/internal/drawingml"
	"github.com/xo/ivy/internal/ooxml"
)

// drawing is the content of a file of Office Open XML that all its pages
// share: the styles, the color table, the theme, the list of the pages and
// the masters. The first call to PageCount or Page reads it.
type drawing struct {
	pkg *ooxml.Document
	cfg ivy.Config
	lim ivy.Limits

	// styles maps the ID of each style to its sheet.
	styles map[string]*sheet
	// defaults are the IDs of the default line, fill and text styles of
	// the document.
	defaults [3]string
	// colors is the color table of the document: the 24 colors of Visio,
	// then the entries of the element Colors.
	colors map[int]drawingml.RGBA
	// faces maps the ID of each font of the element FaceNames to its name.
	faces map[string]string
	// theme is the theme of the document, or nil.
	theme *theme
	// pages are the pages in the order of the file, and fore the indexes
	// of the foreground pages.
	pages []*pageRef
	fore  []int
	// masterParts maps the ID of each master to its part.
	masterParts map[string]string

	mu sync.Mutex
	// masters holds the masters that a page read, by their IDs.
	masters map[string]*master
}

// pageRef is a page of pages.xml.
type pageRef struct {
	id, name string
	// part is the part of the page, or "" for a page without one.
	part       string
	sheet      sheet
	background bool
	// backPage is the ID of the background page of the page, or "".
	backPage string
}

// master is a master of the drawing: the shapes of its part.
type master struct {
	shapes []*shape
	// byID maps the ID of each shape of the master, at any depth, to the
	// shape.
	byID map[string]*shape
	err  error
}

// shape is a shape of a page or of a master, as the file gives it, before
// it inherits from its master and its styles.
type shape struct {
	sheet
	id, kind string
	// masterID and masterShapeID are the attributes Master and
	// MasterShape.
	masterID, masterShapeID string
	// part is the part that holds the shape, for the relationships of its
	// pictures.
	part string
	// text is the element Text, and foreign the element ForeignData. ok
	// reports each one.
	text, foreign       roxmltree.Node
	hasText, hasForeign bool
	children            []*shape
	hasChildren         bool
	// del reports a shape that an instance of a group deletes from its
	// master, which the attribute Del gives.
	del bool
}

// The types of the relationships of the parts of Visio.
const (
	relPages   = "pages"
	relPage    = "page"
	relMasters = "masters"
	relMaster  = "master"
	relTheme   = "theme"
)

// readDrawing reads the parts that the pages share. A drawing without
// pages has no foreground pages.
func readDrawing(ctx context.Context, d *Document) (*drawing, error) {
	dr := &drawing{
		pkg:         d.pkg,
		cfg:         d.cfg,
		lim:         d.cfg.Limits.WithDefaults(),
		styles:      make(map[string]*sheet),
		colors:      make(map[int]drawingml.RGBA),
		faces:       make(map[string]string),
		masterParts: make(map[string]string),
		masters:     make(map[string]*master),
	}
	for i, c := range defaultColors {
		dr.colors[i] = hex(c)
	}
	main, err := dr.mainPart()
	if err != nil {
		return nil, err
	}
	doc, err := dr.part(main)
	if err != nil {
		return nil, fmt.Errorf("reading the document: %w", err)
	}
	cnt := &counter{max: dr.lim.MaxObjects}
	dr.readDocument(doc.RootElement(), cnt)
	rels, err := dr.pkg.Relationships(main, dr.lim)
	if err != nil {
		return nil, err
	}
	log := dr.cfg.Log()
	for _, r := range rels {
		if err := ctx.Err(); err != nil {
			return nil, fmt.Errorf("reading the document: %w", err)
		}
		if r.External {
			continue
		}
		switch {
		case r.TypeIs(relPages):
			if err := dr.readPages(r.Target, cnt); err != nil {
				return nil, err
			}
		case r.TypeIs(relMasters):
			if err := dr.readMasters(r.Target); err != nil {
				log.Debug("vsdx: masters left out", "err", err)
			}
		case r.TypeIs(relTheme):
			if t, err := dr.readTheme(r.Target); err == nil {
				dr.theme = t
			} else {
				log.Debug("vsdx: theme left out", "err", err)
			}
		}
	}
	return dr, nil
}

// mainPart returns the name of the document part, from the relationship
// of the type document of Visio, or of the type officeDocument.
func (dr *drawing) mainPart() (string, error) {
	rels, err := dr.pkg.Relationships("", dr.lim)
	if err != nil {
		return "", err
	}
	for _, r := range rels {
		if !r.External && (r.TypeIs("document") || r.TypeIs("officeDocument")) {
			return r.Target, nil
		}
	}
	if dr.pkg.HasPart(documentPart) {
		return documentPart, nil
	}
	return "", fmt.Errorf("package without a document part: %w", ivy.ErrFormat)
}

// documentPart is the usual name of the document part.
const documentPart = "visio/document.xml"

// part reads and parses the part of XML with the name.
func (dr *drawing) part(name string) (*roxmltree.Document, error) {
	buf, err := dr.pkg.ReadPart(name)
	if err != nil {
		return nil, err
	}
	doc, err := ooxml.ParseXML(buf, dr.lim)
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", name, err)
	}
	return doc, nil
}

// readTheme reads the theme part name.
func (dr *drawing) readTheme(name string) (*theme, error) {
	doc, err := dr.part(name)
	if err != nil {
		return nil, err
	}
	return parseTheme(doc)
}

// defaultColors are the 24 colors of the color table of Visio, by index.
var defaultColors = [...]uint32{
	0x000000, 0xffffff, 0xff0000, 0x00ff00, 0x0000ff, 0xffff00, 0xff00ff, 0x00ffff,
	0x800000, 0x008000, 0x000080, 0x808000, 0x800080, 0x008080, 0xc0c0c0, 0xe6e6e6,
	0xcdcdcd, 0xb3b3b3, 0x9a9a9a, 0x808080, 0x666666, 0x4d4d4d, 0x333333, 0x1a1a1a,
}

// readDocument reads the element VisioDocument: the default styles, the
// color table, the fonts and the styles.
func (dr *drawing) readDocument(root roxmltree.Node, cnt *counter) {
	for e := range elements(root) {
		switch e.TagName().Name() {
		case "DocumentSettings":
			dr.defaults = [3]string{
				ooxml.Attr(e, "DefaultLineStyle"), ooxml.Attr(e, "DefaultFillStyle"), ooxml.Attr(e, "DefaultTextStyle"),
			}
		case "Colors":
			for c := range elements(e) {
				ix, err := strconv.Atoi(ooxml.Attr(c, "IX"))
				if err != nil || ix < 0 || len(dr.colors) >= maxColors {
					continue
				}
				if v, ok := drawingml.ParseHex(ooxml.Attr(c, "RGB")); ok {
					dr.colors[ix] = drawingml.RGBAFrom(v)
				}
			}
		case "FaceNames":
			for c := range elements(e) {
				if len(dr.faces) >= maxColors {
					break
				}
				name := ooxml.Attr(c, "NameU")
				if name == "" {
					name = ooxml.Attr(c, "Name")
				}
				if id := ooxml.Attr(c, "ID"); id != "" {
					dr.faces[id] = name
				}
			}
		case "StyleSheets":
			for c := range elements(e) {
				if c.TagName().Name() != "StyleSheet" || cnt.over() {
					continue
				}
				s := parseSheet(c, cnt)
				dr.styles[ooxml.Attr(c, "ID")] = &s
			}
		}
	}
}

// maxColors bounds the entries of the color table and of the fonts.
const maxColors = 1 << 16

// readPages reads pages.xml: the pages, their sheets and their parts.
func (dr *drawing) readPages(name string, cnt *counter) error {
	doc, err := dr.part(name)
	if err != nil {
		return fmt.Errorf("reading the pages: %w", err)
	}
	rels, err := dr.pkg.Relationships(name, dr.lim)
	if err != nil {
		return err
	}
	for e := range elements(doc.RootElement()) {
		if e.TagName().Name() != "Page" {
			continue
		}
		if len(dr.pages) >= dr.lim.MaxObjects {
			return fmt.Errorf("more than %d pages: %w", dr.lim.MaxObjects, ivy.ErrLimit)
		}
		p := &pageRef{
			id:         ooxml.Attr(e, "ID"),
			name:       ooxml.Attr(e, "NameU"),
			background: ooxml.Attr(e, "Background") == "1",
			backPage:   ooxml.Attr(e, "BackPage"),
		}
		if p.name == "" {
			p.name = ooxml.Attr(e, "Name")
		}
		for c := range elements(e) {
			switch c.TagName().Name() {
			case "PageSheet":
				p.sheet = parseSheet(c, cnt)
			case "Rel":
				if t, ok := target(rels, ooxml.Attr(c, "id"), relPage); ok {
					p.part = t
				}
			}
		}
		if p.sheet.cells == nil {
			p.sheet = sheet{cells: map[string]cell{}, sections: map[string]*section{}}
		}
		if !p.background {
			dr.fore = append(dr.fore, len(dr.pages))
		}
		dr.pages = append(dr.pages, p)
	}
	return nil
}

// target returns the target of the relationship with the ID id and the
// type name.
func target(rels []ooxml.Relationship, id, name string) (string, bool) {
	for _, r := range rels {
		if r.ID == id && !r.External && r.TypeIs(name) {
			return r.Target, true
		}
	}
	return "", false
}

// readMasters reads masters.xml: the part of each master.
func (dr *drawing) readMasters(name string) error {
	doc, err := dr.part(name)
	if err != nil {
		return err
	}
	rels, err := dr.pkg.Relationships(name, dr.lim)
	if err != nil {
		return err
	}
	for e := range elements(doc.RootElement()) {
		if e.TagName().Name() != "Master" || len(dr.masterParts) >= dr.lim.MaxObjects {
			continue
		}
		for c := range elements(e) {
			if c.TagName().Name() != "Rel" {
				continue
			}
			if t, ok := target(rels, ooxml.Attr(c, "id"), relMaster); ok {
				dr.masterParts[ooxml.Attr(e, "ID")] = t
			}
		}
	}
	return nil
}

// master returns the master with the ID id, which it reads once. It
// returns nil for a master that the drawing does not have or cannot read.
func (dr *drawing) master(id string) *master {
	dr.mu.Lock()
	defer dr.mu.Unlock()
	if m, ok := dr.masters[id]; ok {
		return m
	}
	m := &master{byID: make(map[string]*shape)}
	dr.masters[id] = m
	name, ok := dr.masterParts[id]
	if !ok {
		m.err = fmt.Errorf("master %s: %w", id, ivy.ErrFormat)
		return m
	}
	doc, err := dr.part(name)
	if err != nil {
		m.err = err
		return m
	}
	cnt := &counter{max: dr.lim.MaxObjects}
	for e := range elements(doc.RootElement()) {
		if e.TagName().Name() == "Shapes" {
			m.shapes = parseShapes(e, name, cnt, 0, dr.lim.MaxDepth, m.byID)
		}
	}
	if cnt.over() {
		m.err = fmt.Errorf("master %s of more than %d objects: %w", id, dr.lim.MaxObjects, ivy.ErrLimit)
	}
	return m
}

// parseShapes reads the shapes of a Shapes element. byID, when it is not
// nil, receives each shape by its ID.
func parseShapes(n roxmltree.Node, part string, cnt *counter, depth, maxDepth int, byID map[string]*shape) []*shape {
	var list []*shape
	if depth > maxDepth {
		cnt.add(cnt.max + 1)
		return nil
	}
	for e := range elements(n) {
		if e.TagName().Name() != "Shape" || !cnt.add(1) {
			continue
		}
		s := &shape{
			sheet:         parseSheet(e, cnt),
			id:            ooxml.Attr(e, "ID"),
			kind:          ooxml.Attr(e, "Type"),
			masterID:      ooxml.Attr(e, "Master"),
			masterShapeID: ooxml.Attr(e, "MasterShape"),
			part:          part,
			del:           ooxml.Attr(e, "Del") == "1",
		}
		for c := range elements(e) {
			switch c.TagName().Name() {
			case "Text":
				s.text, s.hasText = c, true
			case "ForeignData":
				s.foreign, s.hasForeign = c, true
			case "Shapes":
				s.hasChildren = true
				s.children = parseShapes(c, part, cnt, depth+1, maxDepth, byID)
			}
		}
		if byID != nil && s.id != "" && !s.del {
			if _, ok := byID[s.id]; !ok {
				byID[s.id] = s
			}
		}
		list = append(list, s)
	}
	return list
}

// pageContents reads the shapes of the page part name.
func (dr *drawing) pageContents(name string, cnt *counter) ([]*shape, error) {
	doc, err := dr.part(name)
	if err != nil {
		return nil, err
	}
	var shapes []*shape
	for e := range elements(doc.RootElement()) {
		if e.TagName().Name() == "Shapes" {
			shapes = parseShapes(e, name, cnt, 0, dr.lim.MaxDepth, nil)
		}
	}
	return shapes, nil
}

// pageTheme returns the theme of the page part name: the theme that the
// part names, or the theme of the document.
func (dr *drawing) pageTheme(name string) *theme {
	rels, err := dr.pkg.Relationships(name, dr.lim)
	if err != nil {
		return dr.theme
	}
	for _, r := range rels {
		if !r.External && r.TypeIs(relTheme) {
			if t, err := dr.readTheme(r.Target); err == nil {
				return t
			}
		}
	}
	return dr.theme
}

// picture returns the data of the picture of the relationship id of the
// part name.
func (dr *drawing) picture(name, id string) ([]byte, error) {
	rels, err := dr.pkg.Relationships(name, dr.lim)
	if err != nil {
		return nil, err
	}
	for _, r := range rels {
		if r.ID == id && !r.External {
			return dr.pkg.ReadPart(r.Target)
		}
	}
	return nil, fmt.Errorf("picture %s of %s: %w", id, name, ivy.ErrFormat)
}

// pageIndex returns the index in dr.pages of the page with the ID id.
func (dr *drawing) pageIndex(id string) (int, bool) {
	for i, p := range dr.pages {
		if p.id == id {
			return i, true
		}
	}
	return 0, false
}

// isTrue reports whether the value of a cell of a boolean is true.
func isTrue(v string) bool {
	switch strings.ToUpper(v) {
	case "1", "TRUE":
		return true
	}
	return false
}
