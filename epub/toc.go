package epub

import (
	"path"
	"slices"
	"strings"

	"github.com/xo/okage/roxmltree"

	"github.com/xo/ivy/internal/ebook"
	"github.com/xo/ivy/internal/layout"
)

// toc returns the table of contents of the book: the nav element of the
// type toc of the navigation document of EPUB 3, or else the navMap of the
// NCX file of EPUB 2. A file that cannot be read gives no entry. replaced
// maps an image of the spine to the page that shows it, so that an entry
// that targets the image targets its page.
func (d *Document) toc(replaced map[string]string) []layout.TOCEntry {
	t := ebook.NewTOC(d.lim)
	if !d.navTOC(t) {
		d.ncxTOC(t)
	}
	for i, e := range t.Entries {
		name, fragment, found := strings.Cut(e.Target, "#")
		if page, ok := replaced[name]; ok {
			t.Entries[i].Target = page
			if found {
				t.Entries[i].Target += "#" + fragment
			}
		}
	}
	return t.Entries
}

// parse reads the item it of the manifest as XML.
func (d *Document) parse(it ebook.Item) (*roxmltree.Document, bool) {
	buf, err := d.fsys.ReadFile(it.Path)
	if err != nil {
		d.log.Debug("epub: table of contents not read", "path", it.Path, "err", err)
		return nil, false
	}
	doc, err := ebook.ParseXML(buf, d.lim)
	if err != nil {
		d.log.Debug("epub: table of contents not read", "path", it.Path, "err", err)
		return nil, false
	}
	return doc, true
}

// navTOC adds the entries of the navigation document of EPUB 3 to t. It
// reads the nav element whose epub:type holds toc, or the first nav
// element. It returns false when it adds no entry.
func (d *Document) navTOC(t *ebook.TOC) bool {
	var item ebook.Item
	found := false
	for _, it := range d.pkg.Manifest {
		if it.HasProperty("nav") {
			item, found = it, true
			break
		}
	}
	if !found {
		return false
	}
	doc, ok := d.parse(item)
	if !ok {
		return false
	}
	var nav roxmltree.Node
	navFound := false
	for n := range doc.RootElement().Descendants() {
		if !n.IsElement() || ebook.Local(n) != "nav" {
			continue
		}
		if slices.Contains(strings.Fields(ebook.Attr(n, "type")), "toc") {
			nav, navFound = n, true
			break
		}
		if !navFound {
			nav, navFound = n, true
		}
	}
	if !navFound {
		return false
	}
	dir := path.Dir(item.Path)
	for child := range nav.Children() {
		if child.IsElement() && ebook.Local(child) == "ol" {
			navList(t, child, dir, 1)
			break
		}
	}
	return len(t.Entries) > 0
}

// navList adds the entries of the list ol, at level, and of the lists
// inside its items, to t.
func navList(t *ebook.TOC, ol roxmltree.Node, dir string, level int) {
	if level > t.MaxDepth() {
		return
	}
	for li := range ol.Children() {
		if t.Full() {
			return
		}
		if !li.IsElement() || ebook.Local(li) != "li" {
			continue
		}
		for child := range li.Children() {
			if !child.IsElement() {
				continue
			}
			switch ebook.Local(child) {
			case "a":
				if target, ok := ebook.ResolveTarget(dir, ebook.Attr(child, "href")); ok {
					t.Add(ebook.NodeText(child), level, target)
				}
			case "ol":
				navList(t, child, dir, level+1)
			}
		}
	}
}

// ncxTOC adds the entries of the navMap of the NCX file of EPUB 2 to t.
func (d *Document) ncxTOC(t *ebook.TOC) {
	it, ok := d.ncxItem()
	if !ok {
		return
	}
	doc, ok := d.parse(it)
	if !ok {
		return
	}
	dir := path.Dir(it.Path)
	for n := range doc.RootElement().Children() {
		if n.IsElement() && ebook.Local(n) == "navMap" {
			navPoints(t, n, dir, 1)
			return
		}
	}
}

// ncxItem returns the item of the NCX file: the item that the spine names,
// or else the first item of the media type of NCX.
func (d *Document) ncxItem() (ebook.Item, bool) {
	if it, ok := d.pkg.Item(d.pkg.Toc); ok {
		return it, true
	}
	for _, it := range d.pkg.Manifest {
		if it.MediaType == "application/x-dtbncx+xml" {
			return it, true
		}
	}
	return ebook.Item{}, false
}

// navPoints adds the navPoint elements of n, at level, and the navPoint
// elements inside them, to t.
func navPoints(t *ebook.TOC, n roxmltree.Node, dir string, level int) {
	if level > t.MaxDepth() {
		return
	}
	for p := range n.Children() {
		if t.Full() {
			return
		}
		if !p.IsElement() || ebook.Local(p) != "navPoint" {
			continue
		}
		var label, src string
		for child := range p.Children() {
			if !child.IsElement() {
				continue
			}
			switch ebook.Local(child) {
			case "navLabel":
				for text := range child.Children() {
					if text.IsElement() && ebook.Local(text) == "text" {
						label = ebook.NodeText(text)
						break
					}
				}
			case "content":
				src = ebook.Attr(child, "src")
			}
		}
		if target, ok := ebook.ResolveTarget(dir, src); ok {
			t.Add(label, level, target)
		}
		navPoints(t, p, dir, level+1)
	}
}
