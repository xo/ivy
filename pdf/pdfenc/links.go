package pdfenc

import (
	"image"
	"sort"
	"strings"

	"github.com/xo/okage"
	"github.com/xo/okage/path"
	"github.com/xo/okage/pdf"

	"github.com/xo/ivy/internal/display"
)

// This file maps the links, the anchors and the bookmarks of the display
// list onto the link annotations, the named destinations and the outline of
// the file (D58).

// destination is a point of a page of the file.
type destination struct {
	// page is the index of the page in the file, from 0.
	page int
	x, y float32
}

// xyz returns the destination of the writer of okage.
func (dst destination) xyz() pdf.XYZDestination {
	return pdf.NewXYZDestination(dst.page, path.PointFromXY(dst.x, dst.y))
}

// bookmark is an entry of the outline, at a point of a page of the file.
type bookmark struct {
	level int
	label string
	dst   destination
	open  bool
}

// The interfaces that a device implements for the links.
var (
	_ display.Linker     = (*device)(nil)
	_ display.Bookmarker = (*device)(nil)
)

// marks reports whether the device gives the marks of a page to the file:
// the device of a page, not of a soft mask, and before an error of a limit.
func (d *device) marks() bool {
	return d.page != nil && d.enc.err == nil
}

// mark counts one link, anchor or bookmark of the file, and reports
// whether the file can hold it. Past MaxObjects, it logs the first mark that
// it drops.
func (e *encoder) mark() bool {
	if e.marks >= e.lim.MaxObjects {
		if e.marks == e.lim.MaxObjects {
			e.log.Debug("pdfenc: links and bookmarks past the limit", "max", e.lim.MaxObjects)
			e.marks++
		}
		return false
	}
	e.marks++
	return true
}

// LinkURI satisfies the display.Linker interface. It adds a link
// annotation with a URI action.
func (d *device) LinkURI(r okage.Rect, uri string) {
	if d.rec != nil {
		d.rec.LinkURI(r, uri)
		return
	}
	d.linkURI(r, uri, okage.IdentityTransform())
}

// linkURI adds a link to uri of the rectangle r, which t maps onto the
// user space of the device.
func (d *device) linkURI(r okage.Rect, uri string, t okage.Transform) {
	if !d.marks() || !d.enc.o.links || uri == "" || d.state.hidden {
		return
	}
	if a, ok := d.link(r, t, pdf.NewLinkAction(uri)); ok && d.enc.mark() {
		d.page.AddAnnotation(a)
	}
}

// LinkAnchor satisfies the display.Linker interface. It adds a link
// annotation to the named destination of the anchor, when the file knows
// the page of the anchor. A link to an anchor that a later page names waits
// for the second drawing of the pages, as Encode says.
func (d *device) LinkAnchor(r okage.Rect, name string) {
	if d.rec != nil {
		d.rec.LinkAnchor(r, name)
		return
	}
	d.linkAnchor(r, name, okage.IdentityTransform())
}

// linkAnchor adds a link to the anchor name of the rectangle r, which t
// maps onto the user space of the device.
func (d *device) linkAnchor(r okage.Rect, name string, t okage.Transform) {
	e := d.enc
	if !d.marks() || !e.o.links || name == "" || d.state.hidden {
		return
	}
	dst, ok := e.anchors[name]
	if !ok {
		dst, ok = e.known[name]
	}
	if !ok {
		// the anchor is on a later page, or on no page of the file
		if e.ahead == nil {
			e.ahead = map[string]bool{}
		}
		if len(e.ahead) < e.lim.MaxObjects {
			e.ahead[name] = true
		}
		return
	}
	if a, ok := d.link(r, t, pdf.NewNamedDestination(name, dst.xyz())); ok && e.mark() {
		d.page.AddAnnotation(a)
		e.linked[name] = true
	}
}

// link returns the link annotation of the rectangle r, which t maps onto
// the user space of the device, to target. A rectangle that the transform
// turns gets the quad points of its corners. It returns false for a
// rectangle that the page cannot hold.
func (d *device) link(r okage.Rect, t okage.Transform, target pdf.Target) (pdf.Annotation, bool) {
	ctm := d.s.CTM().PreConcat(t)
	if !inRange(r) || !ctm.IsFinite() || r.Width() <= 0 || r.Height() <= 0 {
		return pdf.Annotation{}, false
	}
	b, ok := pageBounds(r, ctm)
	if !ok || b.Width() <= 0 || b.Height() <= 0 {
		return pdf.Annotation{}, false
	}
	if ctm.KX == 0 && ctm.KY == 0 {
		return pdf.AnnotationFromLink(pdf.NewLinkAnnotation(b, target)), true
	}
	q := pdf.QuadrilateralFromRect(r)
	for i := range q {
		ctm.MapPoint(&q[i])
	}
	return pdf.AnnotationFromLink(pdf.NewLinkAnnotationWithQuadPoints([]pdf.Quadrilateral{q}, target)), true
}

// Anchor satisfies the display.Linker interface. It keeps the point of the
// anchor on the page. When the file names an anchor twice, the first one
// counts.
func (d *device) Anchor(name string, x, y float32) {
	if d.rec != nil {
		d.rec.Anchor(name, x, y)
		return
	}
	d.anchor(name, x, y, okage.IdentityTransform())
}

// anchor keeps the anchor name at the point x, y, which t maps onto the
// user space of the device.
func (d *device) anchor(name string, x, y float32, t okage.Transform) {
	e := d.enc
	if !d.marks() || name == "" {
		return
	}
	if _, ok := e.anchors[name]; ok {
		return
	}
	dst, ok := e.known[name]
	if !ok {
		dst, ok = d.point(x, y, t)
		if !ok {
			return
		}
	}
	if e.mark() {
		e.anchors[name] = dst
	}
}

// point returns the destination of the point x, y, which t maps onto the
// user space of the device, cut to the page.
func (d *device) point(x, y float32, t okage.Transform) (destination, bool) {
	p := path.PointFromXY(x, y)
	ctm := d.s.CTM().PreConcat(t)
	ctm.MapPoint(&p)
	if !finitePoint(p) {
		return destination{}, false
	}
	return destination{
		page: d.index,
		x:    min(max(p.X, 0), d.w),
		y:    min(max(p.Y, 0), d.h),
	}, true
}

// Bookmark satisfies the display.Bookmarker interface. It keeps an entry
// of the outline at the point x, y of the page.
func (d *device) Bookmark(level int, label string, x, y float32, open bool) {
	if d.rec != nil {
		d.rec.Bookmark(level, label, x, y, open)
		return
	}
	d.bookmark(level, label, x, y, open, okage.IdentityTransform())
}

// bookmark keeps an entry of the outline at the point x, y, which t maps
// onto the user space of the device.
func (d *device) bookmark(level int, label string, x, y float32, open bool, t okage.Transform) {
	e := d.enc
	if !d.marks() || !e.o.outline || level < 1 {
		return
	}
	dst, ok := d.point(x, y, t)
	if !ok || !e.mark() {
		return
	}
	e.bookmarks = append(e.bookmarks, bookmark{level: level, label: label, dst: dst, open: open})
}

// writeLinks registers the named destination of each anchor, and sets the
// outline of the bookmarks. An anchor of display.PageAnchor counts only when
// a link of the file goes to it, because each page of some formats names
// its top.
func (e *encoder) writeLinks() {
	names := make([]string, 0, len(e.anchors))
	for name := range e.anchors {
		if strings.HasPrefix(name, display.PageAnchorPrefix) && !e.linked[name] {
			continue
		}
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		e.doc.RegisterNamedDestination(pdf.NewNamedDestination(name, e.anchors[name].xyz()))
	}
	if outline, ok := e.outline(); ok {
		e.doc.SetOutline(outline)
	}
}

// outlineNode is an entry of the tree of the outline that the encoder
// builds.
type outlineNode struct {
	bookmark
	children []*outlineNode
}

// outline returns the outline of the bookmarks, or false for none.
func (e *encoder) outline() (pdf.Outline, bool) {
	root := e.outlineTree()
	var o pdf.Outline
	for _, n := range root {
		o.PushChild(n.node())
	}
	return o, len(root) > 0
}

// outlineTree returns the tree of the bookmarks. A bookmark of level n is
// a child of the last bookmark of a lower level, as make_page_bookmark_tree
// of weasyprint/anchors.py builds it. An entry deeper than MaxDepth is left
// out, with its children.
func (e *encoder) outlineTree() []*outlineNode {
	var root []*outlineNode
	last := []*[]*outlineNode{&root}
	var skipped []int
	previous := 0
	for _, b := range e.bookmarks {
		if b.level > previous {
			// Example: if the previous bookmark is a <h2>, the next depth
			// "should" be for <h3>. If now we get a <h6> we are skipping two
			// levels: append 6 - 3 - 1 = 2
			skipped = append(skipped, b.level-previous-1)
		} else {
			temp := b.level
			for temp < previous && len(skipped) > 0 {
				temp += 1 + skipped[len(skipped)-1]
				skipped = skipped[:len(skipped)-1]
			}
			if temp > previous {
				// We remove too many "skips", add some back:
				skipped = append(skipped, temp-previous-1)
			}
		}
		previous = b.level
		depth := len(skipped)
		if depth < 1 || depth > len(last) {
			continue
		}
		n := &outlineNode{bookmark: b}
		if depth <= e.lim.MaxDepth {
			*last[depth-1] = append(*last[depth-1], n)
		}
		last = append(last[:depth], &n.children)
	}
	return root
}

// node returns the node of the writer of okage for n and its children.
func (n *outlineNode) node() pdf.OutlineNode {
	node := pdf.NewOutlineNode(n.label, n.dst.xyz()).WithOpen(n.open)
	for _, c := range n.children {
		node.PushChild(c.node())
	}
	return node
}

// markDevice takes the links, the anchors and the bookmarks of the content
// of a group that the device draws with the raster fallback, and gives them
// to the device with the transforms of the content. It draws nothing.
type markDevice struct {
	d     *device
	t     okage.Transform
	stack []okage.Transform
}

// The interfaces that a markDevice implements.
var (
	_ display.Device     = (*markDevice)(nil)
	_ display.Linker     = (*markDevice)(nil)
	_ display.Bookmarker = (*markDevice)(nil)
)

// Save satisfies the display.Device interface.
func (m *markDevice) Save() {
	m.stack = append(m.stack, m.t)
}

// Restore satisfies the display.Device interface.
func (m *markDevice) Restore() {
	if n := len(m.stack); n > 0 {
		m.t = m.stack[n-1]
		m.stack = m.stack[:n-1]
	}
}

// Concat satisfies the display.Device interface.
func (m *markDevice) Concat(t okage.Transform) {
	m.t = m.t.PreConcat(t)
}

// FillPath satisfies the display.Device interface. It draws nothing.
func (m *markDevice) FillPath(*okage.Path, okage.FillRule, okage.Shader) {}

// StrokePath satisfies the display.Device interface. It draws nothing.
func (m *markDevice) StrokePath(*okage.Path, okage.Stroke, okage.Shader) {}

// ClipPath satisfies the display.Device interface. It clips nothing.
func (m *markDevice) ClipPath(*okage.Path, okage.FillRule) {}

// DrawImage satisfies the display.Device interface. It draws nothing.
func (m *markDevice) DrawImage(image.Image, okage.Transform, display.ImageOptions) {}

// FillMask satisfies the display.Device interface. It draws nothing.
func (m *markDevice) FillMask(*image.Alpha, okage.Transform, display.ImageOptions, okage.Shader) {}

// FillGlyphs satisfies the display.Device interface. It draws nothing.
func (m *markDevice) FillGlyphs(display.GlyphRun, okage.Shader) {}

// BeginGroup satisfies the display.Device interface. The links of a group
// stay links, so the device keeps nothing of it.
func (m *markDevice) BeginGroup(display.Group) {}

// EndGroup satisfies the display.Device interface.
func (m *markDevice) EndGroup() {}

// LinkURI satisfies the display.Linker interface.
func (m *markDevice) LinkURI(r okage.Rect, uri string) {
	m.d.linkURI(r, uri, m.t)
}

// LinkAnchor satisfies the display.Linker interface.
func (m *markDevice) LinkAnchor(r okage.Rect, name string) {
	m.d.linkAnchor(r, name, m.t)
}

// Anchor satisfies the display.Linker interface.
func (m *markDevice) Anchor(name string, x, y float32) {
	m.d.anchor(name, x, y, m.t)
}

// Bookmark satisfies the display.Bookmarker interface.
func (m *markDevice) Bookmark(level int, label string, x, y float32, open bool) {
	m.d.bookmark(level, label, x, y, open, m.t)
}
