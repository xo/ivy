package pdf

import (
	"context"
	"fmt"
	"math"
	"slices"

	"github.com/xo/okage"

	"github.com/xo/ivy/internal/display"
	"github.com/xo/ivy/internal/hayro/syntax"
)

// This file gives the links, the anchors and the outline of a PDF file to a
// device of the display list (D58). A link annotation of a page becomes a
// link. A named destination of the file becomes an anchor of its page, and
// an explicit destination that a link uses becomes an anchor that
// display.PageAnchor names. Each entry of the outline becomes a bookmark on
// the page of its destination.

// linkIndex holds the anchors and the bookmarks of each page of a file, and
// the destination of each name of the file.
type linkIndex struct {
	// anchors and bookmarks hold the anchors and the bookmarks of each
	// page, in their order.
	anchors   [][]anchor
	bookmarks [][]bookmark
	// named holds the destination of each named destination of the file
	// that resolves to a page.
	named map[string]destination
	// ids holds the number of each page, by the identifier of its object.
	ids map[syntax.ObjectIdentifier]int
}

// anchor is an anchor of a page, at x, y in the space of Draw.
type anchor struct {
	name string
	x, y float32
}

// bookmark is an entry of the outline on a page, at x, y in the space of
// Draw.
type bookmark struct {
	level int
	label string
	x, y  float32
	open  bool
}

// destination is a point of page n, from 0, in the space of Draw.
type destination struct {
	n    int
	x, y float32
}

// name returns the name of the anchor of an explicit destination.
func (dst destination) name() string {
	return display.PageAnchor(dst.n, dst.x, dst.y)
}

// indexer builds a linkIndex within the limits of the document.
type indexer struct {
	ctx  context.Context
	d    *Document
	idx  *linkIndex
	ids  map[syntax.ObjectIdentifier]int
	seen map[string]bool
	// raw holds the destination of each name of the file, as the file
	// gives it.
	raw map[string]syntax.Object
	// objects counts the objects that the index read or holds, which
	// MaxObjects bounds.
	objects int
	full    bool
}

// links returns the index of the links of the file. It builds the index on
// the first call, and keeps it when the context did not stop the build.
func (d *Document) links(ctx context.Context) *linkIndex {
	d.linksMu.Lock()
	defer d.linksMu.Unlock()
	if d.linkIdx != nil {
		return d.linkIdx
	}
	idx := d.buildLinks(ctx)
	if ctx.Err() == nil {
		d.linkIdx = idx
	}
	return idx
}

// buildLinks reads the named destinations, the destinations of the links
// and the outline of the file. A file that is not valid gives the part of
// the index that it read before the fault.
func (d *Document) buildLinks(ctx context.Context) (idx *linkIndex) {
	idx = &linkIndex{
		anchors:   make([][]anchor, len(d.pages)),
		bookmarks: make([][]bookmark, len(d.pages)),
		named:     map[string]destination{},
		ids:       make(map[syntax.ObjectIdentifier]int, len(d.pages)),
	}
	defer func() {
		if r := recover(); r != nil {
			d.log.Debug("pdf: links read in part", "err", fmt.Sprintf("panic: %v", r))
		}
	}()
	ix := &indexer{
		ctx:  ctx,
		d:    d,
		idx:  idx,
		ids:  idx.ids,
		seen: map[string]bool{},
		raw:  map[string]syntax.Object{},
	}
	for n, page := range d.pages {
		if id, ok := page.Raw().ObjID(); ok {
			if _, dup := ix.ids[id]; !dup {
				ix.ids[id] = n
			}
		}
	}
	xref := d.pdf.XRef()
	root, ok := xref.Get[syntax.Dict](xref.RootID())
	if !ok {
		return idx
	}
	ix.namedDestinations(root)
	ix.linkDestinations()
	if outlines, ok := root.Get[syntax.Dict](syntax.KeyOutlines); ok {
		ix.outline(outlines)
	}
	if ix.full {
		d.log.Debug("pdf: links past the limit", "max", d.lim.MaxObjects)
	}
	return idx
}

// count counts one object. It returns false past MaxObjects, or when the
// context stopped the build.
func (ix *indexer) count() bool {
	if ix.full || ix.ctx.Err() != nil {
		return false
	}
	ix.objects++
	if ix.objects > ix.d.lim.MaxObjects {
		ix.full = true
		return false
	}
	return true
}

// namedDestinations reads the names of the dictionary Dests of the catalog
// and of the name tree Dests of the dictionary Names, and gives each one
// that resolves an anchor on its page. A name of both keeps the first.
func (ix *indexer) namedDestinations(root syntax.Dict) {
	if dests, ok := root.Get[syntax.Dict](syntax.KeyDests); ok {
		for _, key := range dests.Keys() {
			if !ix.count() {
				return
			}
			if v, ok := dests.Get[syntax.Object](key); ok {
				ix.addRaw(string(key), v)
			}
		}
	}
	if names, ok := root.Get[syntax.Dict](syntax.KeyNames); ok {
		if tree, ok := names.Get[syntax.Dict](syntax.KeyDests); ok {
			visited := map[syntax.ObjectIdentifier]bool{}
			if ref, ok := names.GetRef(syntax.KeyDests); ok {
				visited[ref.ID()] = true
			}
			ix.nameTree(tree, 0, visited)
		}
	}
	for name, v := range ix.raw {
		if dst, ok := ix.resolve(v, 0); ok {
			ix.idx.named[name] = dst
		}
	}
	// The anchors of a page come in the order of the names, so that the
	// commands of a page do not depend on the order of a map.
	for _, name := range sortedKeys(ix.idx.named) {
		dst := ix.idx.named[name]
		ix.idx.anchors[dst.n] = append(ix.idx.anchors[dst.n], anchor{name: name, x: dst.x, y: dst.y})
	}
}

// addRaw keeps the destination v of a name, unless the name has one.
func (ix *indexer) addRaw(name string, v syntax.Object) {
	if _, ok := ix.raw[name]; !ok {
		ix.raw[name] = v
	}
}

// nameTree reads a node of a name tree, at depth from its root. visited
// holds the nodes that it read, by the identifiers of their objects, so
// that a cycle stops.
func (ix *indexer) nameTree(node syntax.Dict, depth int, visited map[syntax.ObjectIdentifier]bool) {
	if depth > ix.d.lim.MaxDepth || !ix.count() {
		return
	}
	if names, ok := node.Get[syntax.Array](syntax.KeyNames); ok {
		it := names.FlexIter()
		for {
			key, ok := it.Next[syntax.String]()
			if !ok {
				break
			}
			v, ok := it.Next[syntax.Object]()
			if !ok || !ix.count() {
				break
			}
			ix.addRaw(string(key), v)
		}
	}
	kids, ok := node.Get[syntax.Array](syntax.KeyKids)
	if !ok {
		return
	}
	// A kid is an indirect object (ISO 32000-1, 7.9.6), so its reference
	// identifies it.
	for m := range kids.RawIter().All() {
		ref, ok := m.AsObjRef()
		if !ok || visited[ref.ID()] {
			continue
		}
		if ix.full || ix.ctx.Err() != nil {
			return
		}
		visited[ref.ID()] = true
		if kid, ok := ix.d.pdf.XRef().Get[syntax.Dict](ref.ID()); ok {
			ix.nameTree(kid, depth+1, visited)
		}
	}
}

// linkDestinations gives the page of each explicit destination that a link
// of the file uses an anchor.
func (ix *indexer) linkDestinations() {
	for n := range ix.d.pages {
		for _, a := range linkAnnotations(ix.d.pages[n]) {
			if !ix.count() {
				return
			}
			v, ok := linkDestination(a)
			if !ok {
				continue
			}
			if _, named := destName(v); named {
				continue
			}
			if dst, ok := ix.resolve(v, 0); ok {
				ix.addAnchor(dst)
			}
		}
	}
}

// addAnchor gives the page of an explicit destination its anchor, once.
func (ix *indexer) addAnchor(dst destination) {
	name := dst.name()
	if ix.seen[name] {
		return
	}
	ix.seen[name] = true
	ix.idx.anchors[dst.n] = append(ix.idx.anchors[dst.n], anchor{name: name, x: dst.x, y: dst.y})
}

// outline reads the entries of the outline in preorder, and gives each one
// whose destination resolves a bookmark on its page.
func (ix *indexer) outline(outlines syntax.Dict) {
	visited := map[syntax.ObjectIdentifier]bool{}
	var walk func(parent syntax.Dict, level int)
	walk = func(parent syntax.Dict, level int) {
		if level > ix.d.lim.MaxDepth {
			return
		}
		ref, ok := parent.GetRef(syntax.KeyFirst)
		for ok {
			if visited[ref.ID()] || !ix.count() {
				return
			}
			visited[ref.ID()] = true
			item, found := ix.d.pdf.XRef().Get[syntax.Dict](ref.ID())
			if !found {
				return
			}
			ix.outlineItem(item, level)
			walk(item, level+1)
			ref, ok = item.GetRef(syntax.KeyNext)
		}
	}
	walk(outlines, 1)
}

// outlineItem gives one entry of the outline, at level, its bookmark.
func (ix *indexer) outlineItem(item syntax.Dict, level int) {
	var v syntax.Object
	if dest, ok := item.Get[syntax.Object](syntax.KeyDest); ok {
		v = dest
	} else if action, ok := item.Get[syntax.Dict](syntax.KeyA); ok {
		if s, _ := action.Get[syntax.Name](syntax.KeyS); s == "GoTo" {
			v, _ = action.Get[syntax.Object](syntax.KeyD)
		}
	}
	if v == nil {
		return
	}
	dst, ok := ix.resolve(v, 0)
	if !ok {
		return
	}
	title, _ := item.Get[syntax.String](syntax.KeyTitle)
	count, _ := item.Get[syntax.Number](syntax.KeyCount)
	ix.idx.bookmarks[dst.n] = append(ix.idx.bookmarks[dst.n], bookmark{
		level: level,
		label: textString(title),
		x:     dst.x,
		y:     dst.y,
		open:  count.AsI64() > 0,
	})
}

// resolve returns the destination of v: an explicit destination, a name of
// the file, or a dictionary whose entry D holds the destination. depth
// counts the names that it followed.
func (ix *indexer) resolve(v syntax.Object, depth int) (destination, bool) {
	if name, ok := destName(v); ok {
		if dst, ok := ix.idx.named[name]; ok {
			return dst, true
		}
		raw, ok := ix.raw[name]
		if !ok || depth > ix.d.lim.MaxDepth {
			return destination{}, false
		}
		return ix.resolve(raw, depth+1)
	}
	if dict, ok := v.(syntax.Dict); ok {
		d, ok := dict.Get[syntax.Object](syntax.KeyD)
		if !ok || depth > ix.d.lim.MaxDepth {
			return destination{}, false
		}
		return ix.resolve(d, depth+1)
	}
	arr, ok := v.(syntax.Array)
	if !ok {
		return destination{}, false
	}
	return ix.d.explicit(arr, ix.ids)
}

// destName returns the name of a named destination: a name or a string.
func destName(v syntax.Object) (string, bool) {
	switch v := v.(type) {
	case syntax.Name:
		return string(v), true
	case syntax.String:
		return string(v), true
	}
	return "", false
}

// explicit returns the destination of an explicit destination, [page /XYZ
// left top zoom] or another kind of ISO 32000-1, 12.3.2.2. The page is a
// reference to a page of the file, or the number of a page from 0. A
// coordinate that the destination does not give is the top or the left of
// the page.
func (d *Document) explicit(arr syntax.Array, ids map[syntax.ObjectIdentifier]int) (destination, bool) {
	var items []syntax.MaybeRef[syntax.Object]
	for m := range arr.RawIter().All() {
		items = append(items, m)
		if len(items) == 6 {
			break
		}
	}
	if len(items) < 2 {
		return destination{}, false
	}
	n := -1
	if ref, ok := items[0].AsObjRef(); ok {
		if i, ok := ids[ref.ID()]; ok {
			n = i
		}
	} else if num, ok := items[0].Value.(syntax.Number); ok {
		if i := num.AsI64(); i >= 0 && i < int64(len(d.pages)) {
			n = int(i)
		}
	}
	if n < 0 {
		return destination{}, false
	}
	kind, _ := items[1].Value.(syntax.Name)
	num := func(i int) (float64, bool) {
		if i >= len(items) {
			return 0, false
		}
		v, ok := items[i].Value.(syntax.Number)
		if !ok {
			return 0, false
		}
		f := v.AsF64()
		return f, !math.IsNaN(f) && !math.IsInf(f, 0)
	}
	box := d.pages[n].IntersectedCropBox()
	x, y := box.X0, box.Y1
	switch kind {
	case "XYZ":
		if v, ok := num(2); ok {
			x = v
		}
		if v, ok := num(3); ok {
			y = v
		}
	case "FitH", "FitBH":
		if v, ok := num(2); ok {
			y = v
		}
	case "FitV", "FitBV":
		if v, ok := num(2); ok {
			x = v
		}
	case "FitR":
		if v, ok := num(2); ok {
			x = v
		}
		if v, ok := num(5); ok {
			y = v
		}
	}
	px, py := d.toDraw(n, x, y)
	return destination{n: n, x: px, y: py}, true
}

// toDraw maps the point x, y of the user space of page n onto the space of
// Draw, inside the page.
func (d *Document) toDraw(n int, x, y float64) (float32, float32) {
	t := d.pages[n].InitialTransform(true)
	px := t[0]*x + t[2]*y + t[4]
	py := t[1]*x + t[3]*y + t[5]
	w, h := d.pages[n].RenderDimensions()
	px = min(max(px, 0), float64(w))
	py = min(max(py, 0), float64(h))
	// The values round to a hundredth of a point, so that the name of an
	// anchor is short and the same for each use.
	return float32(math.Round(px*100) / 100), float32(math.Round(py*100) / 100)
}

// linkAnnotations returns the link annotations of a page.
func linkAnnotations(page *syntax.Page) []syntax.Dict {
	annots, ok := page.Raw().Get[syntax.Array](syntax.KeyAnnots)
	if !ok {
		return nil
	}
	var links []syntax.Dict
	for a := range annots.Iter[syntax.Dict]().All() {
		if s, _ := a.Get[syntax.Name](syntax.KeySubtype); s == "Link" {
			links = append(links, a)
		}
	}
	return links
}

// linkDestination returns the destination of a link annotation: its entry
// Dest, or the entry D of its action GoTo.
func linkDestination(a syntax.Dict) (syntax.Object, bool) {
	if dest, ok := a.Get[syntax.Object](syntax.KeyDest); ok {
		return dest, true
	}
	action, ok := a.Get[syntax.Dict](syntax.KeyA)
	if !ok {
		return nil, false
	}
	if s, _ := action.Get[syntax.Name](syntax.KeyS); s != "GoTo" {
		return nil, false
	}
	return action.Get[syntax.Object](syntax.KeyD)
}

// linkURI returns the URI of the action URI of a link annotation.
func linkURI(a syntax.Dict) (string, bool) {
	action, ok := a.Get[syntax.Dict](syntax.KeyA)
	if !ok {
		return "", false
	}
	if s, _ := action.Get[syntax.Name](syntax.KeyS); s != "URI" {
		return "", false
	}
	uri, ok := action.Get[syntax.String](syntax.KeyURI)
	if !ok || len(uri) == 0 {
		return "", false
	}
	return string(uri), true
}

// drawLinks gives the links, the anchors and the bookmarks of the page to
// d, in the space of Draw, when d takes them. A file that is not valid
// gives the links that it read before the fault.
func (p *Page) drawLinks(ctx context.Context, d display.Device) {
	linker, isLinker := d.(display.Linker)
	_, isBookmarker := d.(display.Bookmarker)
	if !isLinker && !isBookmarker {
		return
	}
	defer func() {
		if r := recover(); r != nil {
			p.doc.log.Debug("pdf: links of a page read in part", "page", p.n+1, "err", fmt.Sprintf("panic: %v", r))
		}
	}()
	idx := p.doc.links(ctx)
	for _, b := range idx.bookmarks[p.n] {
		display.Bookmark(d, b.level, b.label, b.x, b.y, b.open)
	}
	if !isLinker {
		return
	}
	for _, a := range idx.anchors[p.n] {
		linker.Anchor(a.name, a.x, a.y)
	}
	t := p.page.InitialTransform(true)
	w, h := p.page.RenderDimensions()
	for i, a := range linkAnnotations(p.page) {
		if i >= p.doc.lim.MaxObjects || ctx.Err() != nil {
			return
		}
		r, ok := a.Get[syntax.Rect](syntax.KeyRect)
		if !ok {
			continue
		}
		rect, ok := drawRect(t, r, w, h)
		if !ok {
			continue
		}
		if uri, ok := linkURI(a); ok {
			linker.LinkURI(rect, uri)
			continue
		}
		v, ok := linkDestination(a)
		if !ok {
			continue
		}
		if name, ok := destName(v); ok {
			if _, ok := idx.named[name]; ok {
				linker.LinkAnchor(rect, name)
			}
			continue
		}
		if arr, ok := v.(syntax.Array); ok {
			if dst, ok := p.doc.explicit(arr, idx.ids); ok {
				linker.LinkAnchor(rect, dst.name())
			}
		}
	}
}

// drawRect maps the rectangle r of the user space of a page onto the space
// of Draw with t, and cuts it to the page of w by h points. The rotation of
// a page is a multiple of 90 degrees, so the bounds of the corners are the
// rectangle.
func drawRect(t syntax.Transform, r syntax.Rect, w, h float32) (okage.Rect, bool) {
	left, top := math.Inf(1), math.Inf(1)
	right, bottom := math.Inf(-1), math.Inf(-1)
	for _, c := range [4][2]float64{{r.X0, r.Y0}, {r.X1, r.Y0}, {r.X0, r.Y1}, {r.X1, r.Y1}} {
		x := t[0]*c[0] + t[2]*c[1] + t[4]
		y := t[1]*c[0] + t[3]*c[1] + t[5]
		left, top = min(left, x), min(top, y)
		right, bottom = max(right, x), max(bottom, y)
	}
	left, top = max(left, 0), max(top, 0)
	right, bottom = min(right, float64(w)), min(bottom, float64(h))
	if !(left < right && top < bottom) {
		return okage.Rect{}, false
	}
	return okage.RectFromLTRB(float32(left), float32(top), float32(right), float32(bottom))
}

// sortedKeys returns the keys of m in order.
func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	return keys
}
