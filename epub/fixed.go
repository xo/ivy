package epub

import (
	"bytes"
	"fmt"
	"html"
	"path"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/xo/ivy/internal/ebook"
	"github.com/xo/ivy/internal/raster"
)

// The properties of an item of the spine that give its layout, and the
// value of rendition:layout of a book of fixed layout.
const (
	prePaginated       = "pre-paginated"
	layoutPrePaginated = "rendition:layout-pre-paginated"
	layoutReflowable   = "rendition:layout-reflowable"
)

// pageAttr is the attribute that marks the root element of a document of
// fixed layout with the name of its page, for the style sheet of
// fixedStyle.
const pageAttr = "data-ivy-page"

// pageSize is the size of a page of fixed layout, in CSS pixels.
type pageSize struct {
	width, height float64
}

// name returns the name of the page of size s in the style sheet, such as
// ivy-fxl-1200x1600.
func (s pageSize) name() string {
	px := func(v float64) string {
		return strings.ReplaceAll(strconv.FormatFloat(v, 'f', -1, 64), ".", "_")
	}
	return "ivy-fxl-" + px(s.width) + "x" + px(s.height)
}

// fixedPages makes the pages of fixed layout of a book. A book of fixed
// layout sets rendition:layout to pre-paginated, and each of its documents
// gives the size of its page in a meta element viewport. Each such
// document is a page of that size, with no margin, and the images in it
// keep their sizes. The paper of the configuration does not change it,
// because the file gives the size of its pages (D32).
type fixedPages struct {
	d *Document
	// props are the properties of the items of the spine, by path.
	props map[string][]string
	// sizes are the sizes of the pages, in the order of their first use.
	sizes []pageSize
}

// newFixedPages returns the pages of fixed layout of the book d.
func newFixedPages(d *Document) *fixedPages {
	f := &fixedPages{d: d, props: make(map[string][]string)}
	for _, ref := range d.pkg.Spine {
		if it, ok := d.pkg.Item(ref.IDRef); ok {
			if _, seen := f.props[it.Path]; !seen {
				f.props[it.Path] = ref.Properties
			}
		}
	}
	return f
}

// isFixed reports whether the item name of the spine has a fixed layout:
// the property of its item of the spine, or else the layout of the book.
func (f *fixedPages) isFixed(name string) bool {
	for _, p := range f.props[name] {
		switch p {
		case layoutPrePaginated:
			return true
		case layoutReflowable:
			return false
		}
	}
	return strings.TrimSpace(f.d.pkg.Metadata.Layout) == prePaginated
}

// add records the size s, and returns the name of its page.
func (f *fixedPages) add(s pageSize) string {
	for _, known := range f.sizes {
		if known == s {
			return s.name()
		}
	}
	f.sizes = append(f.sizes, s)
	return s.name()
}

// defaultSize returns the size of a page of the package, if it gives one.
func (f *fixedPages) defaultSize() (pageSize, bool) {
	w, h, ok := ebook.ParseViewport(f.d.pkg.Metadata.Viewport)
	return pageSize{w, h}, ok
}

// document returns the document name, which has a fixed layout, as a page
// of the size of its viewport: the document with the attribute pageAttr
// on its root element. It returns false if the document gives no size and
// the package gives none either, or if the document is not UTF-8. Such a
// document flows onto the pages of the book.
func (f *fixedPages) document(name string) ([]byte, bool) {
	buf, err := f.d.fsys.ReadFile(name)
	if err != nil || !utf8.Valid(buf) {
		return nil, false
	}
	size, ok := pageSize{}, false
	if viewport, found := ebook.DocumentViewport(ebook.Text(buf), f.d.lim); found {
		w, h, valid := ebook.ParseViewport(viewport)
		size, ok = pageSize{w, h}, valid
	}
	if !ok {
		if size, ok = f.defaultSize(); !ok {
			return nil, false
		}
	}
	doc, ok := ebook.SetRootAttr(string(buf), pageAttr, size.name())
	if !ok {
		return nil, false
	}
	f.add(size)
	return []byte(doc), true
}

// image returns a page that shows the image name of the spine, which has a
// fixed layout, at the size of the image: the viewBox, or else the width
// and the height, of an image of SVG, or the pixels of a raster image. It
// returns false if it finds no size.
func (f *fixedPages) image(page, name, title string) ([]byte, bool) {
	size, ok := f.imageSize(name)
	if !ok {
		if size, ok = f.defaultSize(); !ok {
			return nil, false
		}
	}
	pageName := f.add(size)
	var b strings.Builder
	fmt.Fprintf(&b, `<?xml version="1.0" encoding="UTF-8"?>
<html xmlns="http://www.w3.org/1999/xhtml" %s="%s">
<head>
<meta charset="UTF-8"/>
<title>%s</title>
<style>
img { display: block; width: %gpx; height: %gpx; object-fit: contain; }
</style>
</head>
<body><img src="%s" alt=""/></body>
</html>
`, pageAttr, pageName, html.EscapeString(title), size.width, size.height,
		html.EscapeString(ebook.ImagePageLink(page, name)))
	return []byte(b.String()), true
}

// imageSize returns the size of the image name in CSS pixels.
func (f *fixedPages) imageSize(name string) (pageSize, bool) {
	buf, err := f.d.fsys.ReadFile(name)
	if err != nil {
		return pageSize{}, false
	}
	if !isSVG(name, buf) {
		cfg, err := raster.Config(buf)
		if err != nil || cfg.X <= 0 || cfg.Y <= 0 {
			return pageSize{}, false
		}
		return pageSize{float64(cfg.X), float64(cfg.Y)}, true
	}
	doc, err := ebook.ParseXML(buf, f.d.lim)
	if err != nil {
		return pageSize{}, false
	}
	root := doc.RootElement()
	if fields := strings.FieldsFunc(ebook.Attr(root, "viewBox"), func(r rune) bool { return r == ',' || r == ' ' || r == '\t' || r == '\n' || r == '\r' }); len(fields) == 4 {
		w, errW := strconv.ParseFloat(fields[2], 64)
		h, errH := strconv.ParseFloat(fields[3], 64)
		if errW == nil && errH == nil && validSide(w) && validSide(h) {
			return pageSize{w, h}, true
		}
	}
	w, errW := strconv.ParseFloat(strings.TrimSuffix(strings.TrimSpace(ebook.Attr(root, "width")), "px"), 64)
	h, errH := strconv.ParseFloat(strings.TrimSuffix(strings.TrimSpace(ebook.Attr(root, "height")), "px"), 64)
	if errW == nil && errH == nil && validSide(w) && validSide(h) {
		return pageSize{w, h}, true
	}
	return pageSize{}, false
}

// validSide reports whether v is a side of a page in CSS pixels.
func validSide(v float64) bool {
	return v >= 1 && v <= 1<<15
}

// isSVG reports whether the image name with the content buf is an image of
// SVG.
func isSVG(name string, buf []byte) bool {
	if strings.EqualFold(path.Ext(name), ".svg") {
		return true
	}
	head := buf[:min(len(buf), 1024)]
	return bytes.Contains(head, []byte("<svg"))
}

// style returns the style sheet of the pages of fixed layout: a page of the
// size of each viewport with no margin, and the rules of the book that the
// pages leave out. A rule of the page has the name of the page, so that it
// wins over the page of the book.
func (f *fixedPages) style() string {
	if len(f.sizes) == 0 {
		return ""
	}
	var b strings.Builder
	for _, s := range f.sizes {
		fmt.Fprintf(&b, "@page %s {\n  size: %gpx %gpx !important;\n  margin: 0 !important;\n}\n", s.name(), s.width, s.height)
		fmt.Fprintf(&b, "html[%s=%q] {\n  page: %s;\n}\n", pageAttr, s.name(), s.name())
	}
	fmt.Fprintf(&b, "html[%s] img {\n  max-height: none;\n}\n", pageAttr)
	return b.String()
}
