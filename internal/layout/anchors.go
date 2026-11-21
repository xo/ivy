// Copyright (c) 2011-2021, Simon Sapin and contributors.
//
// Use of this source code is governed by the BSD-3-Clause license that can be
// found in the LICENSE file.

// This file ports weasyprint/anchors.py: gather_anchors, which sets the
// transformation matrices of the boxes and finds the anchors, the links and
// the bookmarks of a page, rectangle_aabb, and the part of
// make_page_bookmark_tree that a page needs. ivy writes no form, so the
// inputs and the forms of gather_anchors are not ported. The tree of the
// bookmarks is built by the device that takes them (D58).

package layout

import (
	"math"
	"net/url"
	"path"
	"slices"
	"strings"

	"github.com/xo/okage"

	"github.com/xo/ivy/internal/display"
)

// PageLinks names the links, the anchors and the bookmarks of a page of a
// book (D58). DrawPage gives them to a device that is a display.Linker or a
// display.Bookmarker only when its options hold a PageLinks.
//
// Port: upstream names an anchor by its id, because it lays out one
// document. A book holds many documents, so the names hold the path of the
// document.
type PageLinks struct {
	// Path is the path of the document of the page in the FS of its Source,
	// or "". With a path, the anchor of the id X has the name "Path#X", and
	// the start of the document has the name Path. Without a path, the
	// anchor has the name X.
	Path string
	// Start is set on the first page of the document. The page names its
	// top Path.
	Start bool
	// IsDocument reports whether a path of the FS is a document of the
	// book, or is nil for none. A link to such a document is a link to its
	// anchor. A link to another file of the FS is dropped.
	IsDocument func(path string) bool
	// TOC holds the bookmarks of the table of contents of the book, by the
	// name of the anchor of their target, or is nil. When TOC is not nil,
	// the headings give no bookmark.
	TOC map[string][]TOCBookmark
}

// TOCBookmark is a bookmark of the table of contents of a book.
type TOCBookmark struct {
	// Index is the place of the entry in the table, which orders the
	// bookmarks of one page.
	Index int
	Level int
	Label string
}

// pageMarks are the anchors, the links and the bookmarks of a page, as
// gather_anchors finds them. The points are in the CSS pixels of the page.
type pageMarks struct {
	// anchors holds the name of each anchor and its top left corner.
	anchors []anchorMark
	links   []linkMark
	// bookmarks holds the bookmarks of the headings.
	bookmarks []bookmarkMark
	// names holds the names of the anchors, for the duplicates.
	names map[string]bool
	// count bounds the number of the marks.
	count, max int
}

// anchorMark is an anchor of a page.
type anchorMark struct {
	name string
	x, y float64
}

// linkMark is a link of a page: its type, "external" or "internal", its
// target, and its rectangle (x1, y1, x2, y2).
type linkMark struct {
	kind, target string
	rect         [4]float64
}

// bookmarkMark is a bookmark of a page.
type bookmarkMark struct {
	level int
	label string
	x, y  float64
	open  bool
}

// add counts one mark, and reports whether the page can hold it.
func (m *pageMarks) add() bool {
	if m == nil || m.count >= m.max {
		return false
	}
	m.count++
	return true
}

// rectangleAABB is rectangle_aabb in weasyprint/anchors.py: it applies m,
// when it is not nil, to a rectangle, and returns the bounding box of the
// result as (x1, y1, x2, y2).
func rectangleAABB(m *Matrix, x, y, width, height float64) [4]float64 {
	if m == nil {
		return [4]float64{x, y, x + width, y + height}
	}
	x1, y1 := m.TransformPoint(x, y)
	x2, y2 := m.TransformPoint(x+width, y)
	x3, y3 := m.TransformPoint(x, y+height)
	x4, y4 := m.TransformPoint(x+width, y+height)
	return [4]float64{
		min(x1, x2, x3, x4), min(y1, y2, y3, y4),
		max(x1, x2, x3, x4), max(y1, y2, y3, y4),
	}
}

// gatherAnchors is gather_anchors in weasyprint/anchors.py. It sets
// TransformationMatrix on each box of the tree that has a transform, and nil
// on the other boxes. When marks is not nil, it adds the anchors, the links
// and the bookmarks of the boxes to marks. parent is the matrix of the
// parent, or nil, and parentLink is the link of the parent, or nil.
//
// Port: the forms and the inputs are not gathered.
func gatherAnchors(box *Box, depth int, marks *pageMarks, parent *Matrix, parentLink Value) {
	// Get the transformation matrix of the box.
	// "Transforms apply to block-level and atomic inline-level elements,
	//  but do not apply to elements which may be split into
	//  multiple inline-level boxes."
	// https://www.w3.org/TR/css-transforms-1/#introduction
	if box.ext != nil {
		box.ext.TransformationMatrix = nil
	}
	matrix := parent
	if transforms, ok := box.Style.Get("transform").(Tuple); ok && len(transforms) > 0 && box.Kind != KindInline {
		borderWidth := box.BorderWidth()
		borderHeight := box.BorderHeight()
		var offsetX, offsetY float64
		if origin, ok := box.Style.Get("transform_origin").(Tuple); ok && len(origin) == 2 {
			offsetX, _ = percentageFloat(origin[0], box.Style, borderWidth)
			offsetY, _ = percentageFloat(origin[1], box.Style, borderHeight)
		}
		originX := box.BorderBoxX() + offsetX
		originY := box.BorderBoxY() + offsetY

		m := translation(originX, originY)
		for _, item := range transforms {
			pair, ok := item.(Tuple)
			if !ok || len(pair) != 2 {
				continue
			}
			name, args := str(pair[0]), pair[1]
			t := identity
			a, _ := args.(Tuple)
			switch {
			case name == "scale" && len(a) == 2:
				t.A, t.D = num(a[0]), num(a[1])
			case name == "rotate":
				angle := num(args)
				t.A, t.D = math.Cos(angle), math.Cos(angle)
				t.B = math.Sin(angle)
				t.C = -t.B
			case name == "translate" && len(a) == 2:
				t.E, _ = percentageFloat(a[0], box.Style, borderWidth)
				t.F, _ = percentageFloat(a[1], box.Style, borderHeight)
			case name == "skew" && len(a) == 2:
				t.B, t.C = math.Tan(num(a[1])), math.Tan(num(a[0]))
			case name == "matrix" && len(a) == 6:
				t = Matrix{A: num(a[0]), B: num(a[1]), C: num(a[2]), D: num(a[3]), E: num(a[4]), F: num(a[5])}
			}
			// Port: upstream asserts that the name is matrix for the
			// other names.
			m = t.Mul(m)
		}
		own := translation(-originX, -originY).Mul(m)
		box.ownExtra().TransformationMatrix = own
		if parent != nil {
			own = own.Mul(*parent)
		}
		matrix = &own
	}

	link := parentLink
	if box.ext != nil && box.ext.Link != nil {
		link = box.ext.Link
	}
	if marks != nil {
		marks.gather(box, matrix, link)
	}

	// Port: the depth of the tree is bounded, as the depth of the layout.
	if depth > maxLayoutDepth {
		return
	}
	for _, child := range box.AllChildren() {
		gatherAnchors(child, depth+1, marks, matrix, link)
	}
}

// gather is the part of gather_anchors in weasyprint/anchors.py that adds
// the anchor, the link and the bookmark of one box to the marks.
func (m *pageMarks) gather(box *Box, matrix *Matrix, link Value) {
	var label string
	if box.ext != nil && box.ext.HasBookmarkLabel {
		label = box.ext.BookmarkLabel
	}
	level := 0
	if lv := box.Style.Get("bookmark_level"); !isStr(lv, "none") {
		level = int(num(lv))
	}
	open := !isStr(box.Style.Get("bookmark_state"), "closed")
	anchorName := str(box.Style.Get("anchor"))
	hasBookmark := label != "" && level > 0
	// 'link' is inherited but redundant on text boxes
	hasLink := link != nil && box.Kind != KindText && box.Kind != KindLine
	// In case of duplicate IDs, only the first is an anchor.
	hasAnchor := anchorName != "" && !m.names[anchorName]
	if !hasBookmark && !hasLink && !hasAnchor {
		return
	}

	hit := box.HitArea()
	x, y, width, height := hit[0], hit[1], hit[2], hit[3]
	if hasLink && m.add() {
		// Port: upstream asserts the types of the link. A link of
		// another form is dropped.
		if kind, target, ok := linkTarget(link); ok {
			m.links = append(m.links, linkMark{kind: kind, target: target, rect: rectangleAABB(matrix, x, y, width, height)})
		}
	}
	if hasBookmark && m.add() {
		bx, by := x, y
		if matrix != nil {
			bx, by = matrix.TransformPoint(bx, by)
		}
		m.bookmarks = append(m.bookmarks, bookmarkMark{level: level, label: label, x: bx, y: by, open: open})
	}
	if hasAnchor && m.add() {
		ax, ay := x, y
		if matrix != nil {
			ax, ay = matrix.TransformPoint(ax, ay)
		}
		if m.names == nil {
			m.names = map[string]bool{}
		}
		m.names[anchorName] = true
		m.anchors = append(m.anchors, anchorMark{name: anchorName, x: ax, y: ay})
	}
}

// linkTarget returns the type and the target of a link ('url', (type,
// target)) that GetLink gives.
func linkTarget(link Value) (kind, target string, ok bool) {
	t, ok := link.(Tuple)
	if !ok || len(t) != 2 || !isStr(t[0], "url") {
		return "", "", false
	}
	u, ok := t[1].(Tuple)
	if !ok || len(u) != 2 {
		return "", "", false
	}
	kind, target = str(u[0]), str(u[1])
	return kind, target, (kind == "external" || kind == "internal") && target != ""
}

// draw gives the marks to dev, whose user space is in the CSS pixels of the
// page. links names them.
//
// Port: upstream writes the marks into the PDF file in
// weasyprint/pdf/anchors.py. ivy gives them to the device of the display
// list (D58).
func (m *pageMarks) draw(dev display.Device, links *PageLinks) {
	for _, a := range m.anchors {
		display.Anchor(dev, links.name(a.name), float32(a.x), float32(a.y))
	}
	if links.Start && links.Path != "" {
		display.Anchor(dev, links.Path, 0, 0)
	}
	for _, l := range m.links {
		r, ok := okage.RectFromLTRB(float32(l.rect[0]), float32(l.rect[1]), float32(l.rect[2]), float32(l.rect[3]))
		if !ok {
			continue
		}
		if l.kind == "internal" {
			display.LinkAnchor(dev, r, links.name(l.target))
			continue
		}
		if name, uri, ok := links.external(l.target); ok && name != "" {
			display.LinkAnchor(dev, r, name)
		} else if ok {
			display.LinkURI(dev, r, uri)
		}
	}
	if links.TOC == nil {
		for _, b := range m.bookmarks {
			display.Bookmark(dev, b.level, b.label, float32(b.x), float32(b.y), b.open)
		}
		return
	}
	// Port: the table of contents of a book gives the bookmarks in place
	// of the headings (D58). Each one is at the anchor of its target, in
	// the order of the table.
	type tocMark struct {
		TOCBookmark
		x, y float64
	}
	var toc []tocMark
	at := func(name string, x, y float64) {
		for _, b := range links.TOC[name] {
			if m.add() {
				toc = append(toc, tocMark{b, x, y})
			}
		}
	}
	if links.Start && links.Path != "" {
		at(links.Path, 0, 0)
	}
	for _, a := range m.anchors {
		at(links.name(a.name), a.x, a.y)
	}
	slices.SortStableFunc(toc, func(a, b tocMark) int { return a.Index - b.Index })
	for _, b := range toc {
		display.Bookmark(dev, b.Level, b.Label, float32(b.x), float32(b.y), true)
	}
}

// name returns the name of the anchor of the id of the document of the
// page.
func (l *PageLinks) name(id string) string {
	if l.Path == "" {
		return id
	}
	return l.Path + "#" + id
}

// external returns the name of the anchor of the external link uri, when
// uri is a document of the book, or uri itself when a reader can follow
// it. It returns false for a link to a file of the FS that is not a
// document, and for the schemes of the data that the layout reads, such as
// data.
func (l *PageLinks) external(uri string) (name, target string, ok bool) {
	u, err := url.Parse(uri)
	if err != nil {
		return "", "", false
	}
	switch strings.ToLower(u.Scheme) {
	case "", "data", "javascript", ObjectScheme:
		return "", "", false
	case "file":
		p := DocumentPath(u.Path)
		if l.IsDocument == nil || !l.IsDocument(p) {
			return "", "", false
		}
		if u.Fragment == "" {
			return p, "", true
		}
		return p + "#" + u.Fragment, "", true
	}
	return "", uri, true
}

// DocumentPath returns p as a path of a document of a Source: clean, and
// without a leading "/".
func DocumentPath(p string) string {
	return strings.TrimPrefix(path.Clean("/"+p), "/")
}
