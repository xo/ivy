package pptx

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/xo/okage/roxmltree"

	"github.com/xo/ivy"
	"github.com/xo/ivy/internal/drawingml"
	"github.com/xo/ivy/internal/ooxml"
)

// The size of a slide without p:sldSz: 10 by 7.5 inches, in points.
const (
	defaultWidth  = 720
	defaultHeight = 540
)

// maxParts is the number of parsed parts of XML that a presentation keeps,
// such as the masters, the layouts and the themes.
const maxParts = 64

// presentation is the part ppt/presentation.xml, with the slides that are
// not hidden.
type presentation struct {
	d    *Document
	name string
	// width and height are the size of the slides in points.
	width, height float64
	// slides are the slides that are not hidden, in order.
	slides []slideRef
	// defaults is a:defaultTextStyle, or the zero Node.
	defaults    roxmltree.Node
	hasDefaults bool
	// tableStyles holds the table styles, by ID.
	tableStyles map[string]roxmltree.Node

	mu sync.Mutex
	// parts holds the parsed parts, by name.
	parts map[string]*roxmltree.Document
	// themes holds the themes, by the name of their part.
	themes map[string]*drawingml.Theme
	// pictures holds the pictures, by the name of their part.
	pictures map[string]*drawingml.Picture
}

// slideRef is a slide of the presentation.
type slideRef struct {
	part string
	// number is the number of the slide, from the first slide number,
	// with the hidden slides.
	number int
}

// The types of the relationships of a presentation.
const (
	relSlide       = "slide"
	relLayout      = "slideLayout"
	relMaster      = "slideMaster"
	relTheme       = "theme"
	relTableStyles = "tableStyles"
)

// readPresentation reads the main part of the package and the list of its
// slides.
func readPresentation(ctx context.Context, d *Document) (*presentation, error) {
	main, err := d.doc.MainPart(d.lim)
	if err != nil {
		return nil, err
	}
	p := &presentation{
		d:        d,
		name:     main,
		width:    defaultWidth,
		height:   defaultHeight,
		parts:    make(map[string]*roxmltree.Document),
		themes:   make(map[string]*drawingml.Theme),
		pictures: make(map[string]*drawingml.Picture),
	}
	doc, err := p.part(main)
	if err != nil {
		return nil, err
	}
	rels, err := d.doc.Relationships(main, d.lim)
	if err != nil {
		return nil, err
	}
	byID := make(map[string]ooxml.Relationship, len(rels))
	for _, r := range rels {
		byID[r.ID] = r
		if !r.External && r.TypeIs(relTableStyles) {
			p.readTableStyles(r.Target)
		}
	}
	root := doc.RootElement()
	first := 1
	if v, ok := atoi(ooxml.Attr(root, "firstSlideNum")); ok {
		first = v
	}
	for e := range root.Children() {
		if !e.IsElement() {
			continue
		}
		switch e.TagName().Name() {
		case "sldSz":
			w, ok1 := atoi(ooxml.Attr(e, "cx"))
			h, ok2 := atoi(ooxml.Attr(e, "cy"))
			if ok1 && ok2 && w > 0 && h > 0 {
				// ECMA-376 bounds a slide to 51206400 EMU
				p.width = float64(min(w, 51206400)) / 12700
				p.height = float64(min(h, 51206400)) / 12700
			}
		case "defaultTextStyle":
			p.defaults, p.hasDefaults = e, true
		case "sldIdLst":
			number := first
			for s := range e.Children() {
				if !s.IsElement() || s.TagName().Name() != "sldId" {
					continue
				}
				if err := ctx.Err(); err != nil {
					return nil, fmt.Errorf("reading the slides: %w", err)
				}
				if len(p.slides) >= d.lim.MaxObjects {
					return nil, fmt.Errorf("more than %d slides: %w", d.lim.MaxObjects, ivy.ErrLimit)
				}
				r, ok := byID[relID(s)]
				n := number
				number++
				if !ok || r.External || !r.TypeIs(relSlide) {
					continue
				}
				if p.hidden(r.Target) {
					continue
				}
				p.slides = append(p.slides, slideRef{part: r.Target, number: n})
			}
		}
	}
	return p, nil
}

// relID returns the attribute r:id of n.
func relID(n roxmltree.Node) string {
	for a := range n.Attributes() {
		if ns, _ := a.Namespace(); a.Name() == "id" && strings.Contains(ns, "relationships") {
			return a.Value()
		}
	}
	return ""
}

// atoi returns s as a whole number within the range of an int32.
func atoi(s string) (int, bool) {
	s = strings.TrimSpace(s)
	if s == "" || len(s) > 12 {
		return 0, false
	}
	n, neg := 0, false
	for i, c := range s {
		switch {
		case i == 0 && c == '-':
			neg = true
		case c >= '0' && c <= '9':
			n = n*10 + int(c-'0')
		default:
			return 0, false
		}
	}
	if neg {
		n = -n
	}
	return min(max(n, -1<<31), 1<<31-1), true
}

// hidden reports whether the slide of the part name is hidden: its root
// element has show="0". It reads the start of the part only, and a slide
// that cannot be read is not hidden, so that its page reports the error.
func (p *presentation) hidden(name string) bool {
	buf, err := p.d.doc.ReadPart(name)
	if err != nil {
		return false
	}
	// the start tag of the root element, after the declaration and the
	// comments
	i := 0
	for {
		j := bytes.IndexByte(buf[i:], '<')
		if j < 0 {
			return false
		}
		i += j
		if i+1 < len(buf) && (buf[i+1] == '?' || buf[i+1] == '!') {
			i++
			continue
		}
		break
	}
	end := bytes.IndexByte(buf[i:], '>')
	if end < 0 {
		return false
	}
	tag := buf[i : i+end]
	return bytes.Contains(tag, []byte(` show="0"`)) || bytes.Contains(tag, []byte(` show="false"`))
}

// part returns the parsed part name, from the cache.
func (p *presentation) part(name string) (*roxmltree.Document, error) {
	p.mu.Lock()
	doc, ok := p.parts[name]
	p.mu.Unlock()
	if ok {
		return doc, nil
	}
	buf, err := p.d.doc.ReadPart(name)
	if err != nil {
		return nil, err
	}
	doc, err = ooxml.ParseXML(buf, p.d.lim)
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", name, err)
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if len(p.parts) >= maxParts {
		clear(p.parts)
	}
	p.parts[name] = doc
	return doc, nil
}

// theme returns the theme of the part name, from the cache.
func (p *presentation) theme(name string) (*drawingml.Theme, error) {
	p.mu.Lock()
	t, ok := p.themes[name]
	p.mu.Unlock()
	if ok {
		return t, nil
	}
	doc, err := p.part(name)
	if err != nil {
		return nil, err
	}
	t, err = drawingml.ParseTheme(doc)
	if err != nil {
		return nil, err
	}
	t.Pictures = p.picturesOf(name)
	p.mu.Lock()
	defer p.mu.Unlock()
	if len(p.themes) >= maxParts {
		clear(p.themes)
	}
	p.themes[name] = t
	return t, nil
}

// readTableStyles reads the table styles of the part name. A part that
// cannot be read gives no style, and is logged.
func (p *presentation) readTableStyles(name string) {
	doc, err := p.part(name)
	if err != nil {
		p.d.cfg.Log().Debug("pptx: table styles left out", "err", err)
		return
	}
	p.tableStyles = make(map[string]roxmltree.Node)
	for e := range doc.RootElement().Children() {
		if e.IsElement() && e.TagName().Name() == "tblStyle" {
			p.tableStyles[strings.TrimSpace(ooxml.Attr(e, "styleId"))] = e
		}
	}
}

// tableStyle returns the table style with the ID: the style of the
// package, or the default table style of PowerPoint.
func (p *presentation) tableStyle(id string) (roxmltree.Node, bool) {
	if n, ok := p.tableStyles[id]; ok {
		return n, true
	}
	return builtinTableStyle(id)
}

// partPictures gives the pictures of the relationships of a part.
type partPictures struct {
	p    *presentation
	part string
	rels map[string]ooxml.Relationship
}

// The interfaces that a partPictures implements.
var _ drawingml.Pictures = (*partPictures)(nil)

// pictures returns the pictures of the part name.
func (p *presentation) picturesOf(name string) *partPictures {
	pp := &partPictures{p: p, part: name, rels: map[string]ooxml.Relationship{}}
	rels, err := p.d.doc.Relationships(name, p.d.lim)
	if err != nil {
		p.d.cfg.Log().Debug("pptx: relationships left out", "part", name, "err", err)
	}
	for _, r := range rels {
		pp.rels[r.ID] = r
	}
	return pp
}

// Picture satisfies the drawingml.Pictures interface. It returns the
// picture of the part that the relationship id targets, from the cache of
// the presentation.
func (pp *partPictures) Picture(id string) *drawingml.Picture {
	r, ok := pp.rels[id]
	if !ok || r.External || !pp.p.d.doc.HasPart(r.Target) {
		return nil
	}
	p := pp.p
	p.mu.Lock()
	defer p.mu.Unlock()
	if pic, ok := p.pictures[r.Target]; ok {
		return pic
	}
	buf, err := p.d.doc.ReadPart(r.Target)
	if err != nil {
		p.d.cfg.Log().Debug("pptx: picture left out", "part", r.Target, "err", err)
		return nil
	}
	pic := drawingml.NewPicture(buf, p.d.cfg)
	if len(p.pictures) >= maxParts {
		clear(p.pictures)
	}
	p.pictures[r.Target] = pic
	return pic
}

// target returns the target of the first relationship of the type of the
// part name.
func (p *presentation) target(name, typ string) (string, bool) {
	rels, err := p.d.doc.Relationships(name, p.d.lim)
	if err != nil {
		return "", false
	}
	for _, r := range rels {
		if !r.External && r.TypeIs(typ) {
			return r.Target, true
		}
	}
	return "", false
}
