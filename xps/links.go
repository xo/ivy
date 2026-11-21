package xps

import (
	"net/url"
	"path"
	"strconv"
	"strings"

	"github.com/xo/ivy/internal/display"
	"github.com/xo/okage"
	"github.com/xo/okage/roxmltree"
)

// relDocumentStructure is the suffix of the type of the relationship from a
// FixedDocument to its DocumentStructure.
const relDocumentStructure = "/documentstructure"

// linkIndex is the index of the link targets and of the outline of a
// document (D58). A FixedDocument names its link targets in the
// LinkTargets of each PageContent, and the names are unique in that
// FixedDocument (ECMA-388, 9.3.3).
type linkIndex struct {
	// names maps the name of each link target of each FixedDocument to the
	// number of its page.
	names []map[string]int
	// pageDocs holds the number of the FixedDocument of each page.
	pageDocs []int
	// partPage maps the part name of each page to its number, and docPart
	// maps the part name of each FixedDocument to its number.
	partPage map[string]int
	docPart  map[string]int
	// outline holds the entries of the outline of each page, in the order
	// of the outline.
	outline map[int][]outlineEntry
	// multi is set when the sequence has more than one FixedDocument, so
	// that the name of an anchor holds the part of its FixedDocument.
	multi bool
}

// outlineEntry is an entry of the DocumentOutline of a document, on the
// page of its target. name is the link target, or empty for the top of the
// page.
type outlineEntry struct {
	level int
	label string
	name  string
}

// linkIndex returns the index of the links of the document, which it reads
// once. The index holds at most Limits.MaxObjects link targets and entries
// of the outline. A part that it cannot read adds nothing.
func (d *Document) linkIndex() *linkIndex {
	d.linksOnce.Do(func() {
		ix := &linkIndex{
			names:    make([]map[string]int, len(d.docs)),
			pageDocs: make([]int, len(d.pages)),
			partPage: make(map[string]int, len(d.pages)),
			docPart:  make(map[string]int, len(d.docs)),
			outline:  map[int][]outlineEntry{},
			multi:    len(d.docs) > 1,
		}
		for i, p := range d.pages {
			ix.pageDocs[i] = p.docN
			if _, ok := ix.partPage[p.part]; !ok {
				ix.partPage[p.part] = i
			}
		}
		for i, part := range d.docs {
			if _, ok := ix.docPart[part]; !ok {
				ix.docPart[part] = i
			}
			ix.names[i] = map[string]int{}
		}
		count := 0
		for i, part := range d.docs {
			count = d.readTargets(ix, i, part, count)
		}
		for i, part := range d.docs {
			count = d.readOutline(ix, i, part, count)
		}
		d.links = ix
	})
	return d.links
}

// readTargets adds the link targets of the FixedDocument number i, the part
// part, to ix. count is the number of the targets and the entries of the
// index so far, and readTargets returns the new count.
func (d *Document) readTargets(ix *linkIndex, i int, part string, count int) int {
	doc, err := d.pkg.xml(part)
	if err != nil {
		d.log.Debug("xps: link targets left out", "part", part, "err", err)
		return count
	}
	for pc := range doc.Descendants() {
		if !pc.IsElement() || local(pc) != "PageContent" {
			continue
		}
		page, ok := ix.partPage[resolve(part, attr(pc, "Source"))]
		if !ok {
			continue
		}
		for t := range pc.Descendants() {
			if !t.IsElement() || local(t) != "LinkTarget" {
				continue
			}
			name := attr(t, "Name")
			if name == "" {
				continue
			}
			if _, ok := ix.names[i][name]; ok {
				continue
			}
			if count >= d.lim.MaxObjects {
				d.log.Debug("xps: link targets past the limit", "max", d.lim.MaxObjects)
				return count
			}
			ix.names[i][name] = page
			count++
		}
	}
	return count
}

// readOutline adds the DocumentOutline of the FixedDocument number i to ix.
// Each entry goes on the page of its target. An entry whose target does not
// resolve is left out.
func (d *Document) readOutline(ix *linkIndex, i int, part string, count int) int {
	name, ok := find(d.pkg.rels(part), relDocumentStructure)
	if !ok {
		return count
	}
	doc, err := d.pkg.xml(name)
	if err != nil {
		d.log.Debug("xps: outline left out", "part", name, "err", err)
		return count
	}
	for e := range doc.Descendants() {
		if !e.IsElement() || local(e) != "OutlineEntry" {
			continue
		}
		if count >= d.lim.MaxObjects {
			d.log.Debug("xps: outline past the limit", "max", d.lim.MaxObjects)
			return count
		}
		page, target, ok := ix.target(name, attr(e, "OutlineTarget"), i)
		if !ok {
			continue
		}
		level := 1
		if v, err := strconv.Atoi(strings.TrimSpace(attr(e, "OutlineLevel"))); err == nil && v > 1 {
			level = min(v, d.lim.MaxDepth)
		}
		ix.outline[page] = append(ix.outline[page], outlineEntry{
			level: level,
			label: strings.TrimSpace(attr(e, "Description")),
			name:  target,
		})
		count++
	}
	return count
}

// target resolves ref, a link from the part base of the FixedDocument
// number doc, to the number of its page and the name of its link target,
// or an empty name for the top of the page. It returns false when ref names
// no page and no link target of the document.
func (ix *linkIndex) target(base, ref string, doc int) (int, string, bool) {
	ref, frag, _ := strings.Cut(strings.TrimSpace(ref), "#")
	if ref != "" {
		part := resolve(base, ref)
		switch page, isPage := ix.partPage[part]; {
		case isPage && frag == "":
			return page, "", true
		case isPage:
			doc = ix.docOf(page)
		default:
			n, isDoc := ix.docPart[part]
			switch {
			case isDoc:
				doc = n
			case strings.EqualFold(path.Ext(part), ".fdseq"):
				doc = -1
			default:
				return 0, "", false
			}
			if frag == "" {
				return ix.firstPage(doc)
			}
		}
	}
	if frag == "" {
		return 0, "", false
	}
	if doc < 0 {
		// a fragment of the sequence names a target of any document
		for i := range ix.names {
			if page, ok := ix.names[i][frag]; ok {
				return page, frag, true
			}
		}
		return 0, "", false
	}
	if doc >= len(ix.names) {
		return 0, "", false
	}
	page, ok := ix.names[doc][frag]
	if !ok {
		return 0, "", false
	}
	return page, frag, true
}

// docOf returns the number of the FixedDocument of page, or -1.
func (ix *linkIndex) docOf(page int) int {
	if page < 0 || page >= len(ix.pageDocs) {
		return -1
	}
	return ix.pageDocs[page]
}

// firstPage returns the first page of the FixedDocument number doc, or the
// first page of the sequence for -1.
func (ix *linkIndex) firstPage(doc int) (int, string, bool) {
	for page, n := range ix.pageDocs {
		if doc < 0 || n == doc {
			return page, "", true
		}
	}
	return 0, "", false
}

// anchorName returns the name of the anchor of the link target name of the
// FixedDocument number doc. With more than one FixedDocument, the name
// starts with the number of the FixedDocument, from 1, and "#".
func (ix *linkIndex) anchorName(doc int, name string) string {
	if !ix.multi {
		return name
	}
	return strconv.Itoa(doc+1) + "#" + name
}

// pageLinks is the state of the links of the walk of one page.
type pageLinks struct {
	ix   *linkIndex
	page int
	doc  int
	// seen holds the link targets that the walk named already, with the
	// point of each one on the device, for the outline.
	seen map[string]okage.Point
	// toPage maps the pixels of the device back to the units of the page,
	// the user space of the walk at its start.
	toPage okage.Transform
}

// pageLinks returns the state of the links of the walk of p onto dev, whose
// transform is ctm, or nil when dev takes no link and no bookmark. It names
// the top of the page with display.PageAnchor, for a link to the page.
func (p *Page) pageLinks(dev display.Device, ctm okage.Transform) *pageLinks {
	_, linker := dev.(display.Linker)
	_, bookmarker := dev.(display.Bookmarker)
	if !linker && !bookmarker {
		return nil
	}
	inv, ok := ctm.Invert()
	if !ok {
		return nil
	}
	ix := p.doc.linkIndex()
	display.Anchor(dev, display.PageAnchor(p.n, 0, 0), 0, 0)
	return &pageLinks{ix: ix, page: p.n, doc: p.docN, seen: map[string]okage.Point{}, toPage: inv}
}

// mark gives the links and the anchor of the element n, whose bounds in the
// current user space bounds computes. A link target of the page becomes an
// anchor at the top left corner of its bounds, and a NavigateUri a link.
// The elements of a VisualBrush give none.
func (r *renderer) mark(n roxmltree.Node, bounds func() (okage.Rect, bool)) {
	l := r.links
	if l == nil || r.visuals > 0 {
		return
	}
	uri := strings.TrimSpace(attr(n, "NavigateUri"))
	name := attr(n, "Name")
	named := false
	if name != "" && l.doc >= 0 && l.doc < len(l.ix.names) {
		if page, ok := l.ix.names[l.doc][name]; ok && page == l.page {
			_, done := l.seen[name]
			named = !done
		}
	}
	if uri == "" && !named {
		return
	}
	b, ok := bounds()
	if !ok {
		return
	}
	if named {
		display.Anchor(r.dev, l.ix.anchorName(l.doc, name), b.Left(), b.Top())
		pt := okage.Point{X: b.Left(), Y: b.Top()}
		r.ctm.MapPoint(&pt)
		l.seen[name] = pt
	}
	if uri != "" {
		r.link(b, uri)
	}
}

// link makes b a link to uri. An absolute URI is a link to it. A URI of a
// page, a FixedDocument or the sequence of the package, with or without
// the name of a link target, is a link to its anchor.
func (r *renderer) link(b okage.Rect, uri string) {
	if u, err := url.Parse(uri); err == nil && len(u.Scheme) > 1 && !strings.EqualFold(u.Scheme, "pack") {
		display.LinkURI(r.dev, b, uri)
		return
	}
	l := r.links
	page, name, ok := l.ix.target(r.part, uri, l.doc)
	if !ok {
		return
	}
	if name == "" {
		display.LinkAnchor(r.dev, b, display.PageAnchor(page, 0, 0))
		return
	}
	display.LinkAnchor(r.dev, b, l.ix.anchorName(l.ix.docOf(page), name))
}

// bookmarks gives the entries of the outline of the page, in the order of
// the outline, in the units of the page. An entry goes at the anchor of its
// link target, or at the top of the page when the walk did not find it.
func (r *renderer) bookmarks() {
	l := r.links
	if l == nil {
		return
	}
	for _, e := range l.ix.outline[l.page] {
		var x, y float32
		if pt, ok := l.seen[e.name]; ok && e.name != "" {
			l.toPage.MapPoint(&pt)
			x, y = pt.X, pt.Y
		}
		display.Bookmark(r.dev, e.level, e.label, x, y, true)
	}
}

// boundsOf returns the bounds of the element n in the user space of its
// parent, with its RenderTransform and without its clip. It counts the
// elements that it reads against the limits of the page.
func (r *renderer) boundsOf(n roxmltree.Node, res *resources, depth int) (okage.Rect, bool) {
	r.w.count++
	if r.w.count > r.doc.lim.MaxObjects || depth > r.doc.lim.MaxDepth || r.ctx.Err() != nil {
		return okage.Rect{}, false
	}
	var b okage.Rect
	var ok bool
	switch local(n) {
	case "Canvas":
		b, ok = r.childrenBounds(n, r.resourcesOf(n, res), depth)
	case "Path":
		if g, has, err := r.geometryOf(n, "Data", res); err == nil && has && g.path != nil {
			b, ok = g.path.Bounds(), true
		}
	case "Glyphs":
		if run, err := r.glyphRun(n); err == nil {
			b, ok = runBounds(run)
		}
	}
	if !ok {
		return okage.Rect{}, false
	}
	t, err := r.transformOf(n, "RenderTransform", res)
	if err != nil || t.IsIdentity() {
		return b, true
	}
	return b.Transform(t)
}

// childrenBounds returns the union of the bounds of the child elements of n,
// in the user space of n.
func (r *renderer) childrenBounds(n roxmltree.Node, res *resources, depth int) (okage.Rect, bool) {
	var all okage.Rect
	ok := false
	for _, c := range elements(n) {
		if strings.Contains(local(c), ".") {
			continue
		}
		b, has := r.boundsOf(c, res, depth+1)
		if !has {
			continue
		}
		if !ok {
			all, ok = b, true
			continue
		}
		if u, has := okage.RectFromLTRB(min(all.Left(), b.Left()), min(all.Top(), b.Top()),
			max(all.Right(), b.Right()), max(all.Bottom(), b.Bottom())); has {
			all = u
		}
	}
	return all, ok
}

// runBounds returns the bounds of the outlines of a run of glyphs. A run
// without outlines, such as a run of spaces, takes the box from its origins
// to one em above them.
func runBounds(run display.GlyphRun) (okage.Rect, bool) {
	if len(run.Glyphs) == 0 {
		return okage.Rect{}, false
	}
	if p := run.Path(); p != nil {
		return p.Bounds(), true
	}
	g0 := run.Glyphs[0]
	left, right, top, bottom := g0.X, g0.X, g0.Y-run.Size, g0.Y
	for _, g := range run.Glyphs[1:] {
		left, right = min(left, g.X), max(right, g.X)
		top, bottom = min(top, g.Y-run.Size), max(bottom, g.Y)
	}
	return okage.RectFromLTRB(left, top, max(right, left+run.Size/2), bottom)
}
