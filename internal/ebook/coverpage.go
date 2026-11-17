package ebook

import (
	"html"
	"io/fs"
	"net/url"
	"path"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	okhtml "github.com/xo/okage/html"

	"github.com/xo/ivy"
)

// maxCoverPageText is the number of letters and digits that a page of a
// cover can hold, such as a hidden heading "Cover". A page with more text is
// a page of the book that shows the cover image, such as a title page.
const maxCoverPageText = 64

// ImagePage returns a document of XHTML that shows the image at the link src
// alone on a page of BookStyleFor(p). The image is scaled to fit the whole
// page, over its margins, and centered, as on the cover page of calibre.
// src is relative to the document. BookStyleFor sets the margins of each
// page, and the book cannot change them, so the image takes the size of
// the page and moves up and left by the margins.
func ImagePage(src, title string, p ivy.Paper) []byte {
	p = bookPaperOf(p)
	var b strings.Builder
	b.WriteString(`<?xml version="1.0" encoding="UTF-8"?>
<html xmlns="http://www.w3.org/1999/xhtml">
<head>
<meta charset="UTF-8"/>
<title>`)
	b.WriteString(html.EscapeString(xmlText(title)))
	b.WriteString(`</title>
<style>
html, body { margin: 0; padding: 0; border: 0; }
img {
  display: block;
  position: absolute;
  left: -` + bookMarginX + `;
  top: -` + bookMarginY + `;
  width: ` + points(float64(p.Width)) + `;
  height: ` + points(float64(p.Height)) + `;
  max-width: none;
  max-height: none;
  margin: 0;
  object-fit: contain;
  object-position: center;
}
</style>
</head>
<body><img src="`)
	b.WriteString(html.EscapeString(xmlText(src)))
	b.WriteString(`" alt=""/></body>
</html>
`)
	return []byte(b.String())
}

// xmlText returns s without the characters that XML does not allow.
func xmlText(s string) string {
	return strings.Map(func(r rune) rune {
		switch {
		case r == '\t', r == '\n', r == '\r':
			return r
		case r < 0x20, r == 0xfffe, r == 0xffff, r == utf8.RuneError:
			return -1
		}
		return r
	}, s)
}

// ImagePageLink returns the link from the document page to the image name,
// for the src of ImagePage. Both are paths of the file system of the
// layout, such as "OEBPS/cover-page.xhtml" and "OEBPS/images/cover.jpg".
func ImagePageLink(page, name string) string {
	rel := name
	if dir := path.Dir(page); dir != "." {
		if after, ok := strings.CutPrefix(name, dir+"/"); ok {
			rel = after
		} else {
			rel = strings.Repeat("../", strings.Count(dir, "/")+1) + name
		}
	}
	segments := strings.Split(rel, "/")
	for i, s := range segments {
		segments[i] = url.PathEscape(s)
	}
	// "./" keeps a first segment with a colon from reading as a scheme
	return "./" + strings.Join(segments, "/")
}

// IsCoverPage reports whether the document name of fsys is a page of the
// cover image cover: its first image is cover, and it holds almost no text.
// A first image of SVG that shows cover also counts, as in the cover page of
// a book that calibre made. IsCoverPage also reports whether the document
// shows cover at its top, before almost any text, so that a format does not
// add a page of the cover before a page that already starts with the cover.
func IsCoverPage(fsys fs.FS, name, cover string, lim ivy.Limits) (page, shows bool) {
	lim = lim.WithDefaults()
	first, ok := firstImage(fsys, name, lim)
	if !ok || first.before > maxCoverPageText {
		return false, false
	}
	img := first.path
	if img != cover && strings.EqualFold(path.Ext(img), ".svg") {
		if inner, ok := firstImage(fsys, img, lim); ok {
			img = inner.path
		}
	}
	if img != cover {
		return false, false
	}
	return first.letters <= maxCoverPageText, true
}

// pageImage is the first image of a document.
type pageImage struct {
	// path is the path of the image in the file system.
	path string
	// before counts the letters and digits of the text before the image,
	// and letters counts those of the whole text.
	before, letters int
}

// firstImage returns the first image of the document name of fsys. It reads
// the document as HTML, so it also reads a document of XHTML or of SVG.
func firstImage(fsys fs.FS, name string, lim ivy.Limits) (pageImage, bool) {
	buf, err := fs.ReadFile(fsys, name)
	if err != nil {
		return pageImage{}, false
	}
	tree, err := okhtml.ParseDocument(strings.NewReader(Text(buf)),
		okhtml.WithCharset("utf-8"),
		okhtml.WithMaxNodes(lim.MaxObjects),
		okhtml.WithMaxDepth(lim.MaxDepth))
	if err != nil {
		return pageImage{}, false
	}
	var img pageImage
	var link string
	found := false
	var walk func(n *okhtml.Node)
	walk = func(n *okhtml.Node) {
		for c := range n.Children() {
			switch c.Type() {
			case okhtml.TextNode:
				for _, r := range c.Data() {
					if unicode.IsLetter(r) || unicode.IsNumber(r) {
						img.letters++
					}
				}
			case okhtml.ElementNode:
				switch c.Name() {
				case "head", "script", "style", "title", "template", "noscript":
					continue
				case "img", "image":
					if !found {
						link, found = imageLink(c)
						img.before = img.letters
					}
				}
				walk(c)
			}
		}
	}
	walk(tree.Root())
	if !found {
		return pageImage{}, false
	}
	img.path, found = Resolve(path.Dir(name), link)
	return img, found
}

// imageLink returns the link of an img element of HTML, or of an image
// element of SVG.
func imageLink(n *okhtml.Node) (string, bool) {
	if n.Namespace() == okhtml.NamespaceSVG {
		if href, ok := n.AttrNS(okhtml.NamespaceXLink, "href"); ok && strings.TrimSpace(href) != "" {
			return href, true
		}
		if href, ok := n.Attr("href"); ok && strings.TrimSpace(href) != "" {
			return href, true
		}
		return "", false
	}
	if n.Name() != "img" {
		return "", false
	}
	src, ok := n.Attr("src")
	return src, ok && strings.TrimSpace(src) != ""
}

// maxPageNames is the number of names that ImagePageName tries.
const maxPageNames = 1000

// ImagePageName returns the path of the page of ImagePage for the image
// name: a path in the folder of name, for which exists reports false.
func ImagePageName(name string, exists func(string) bool) string {
	base := strings.TrimSuffix(name, path.Ext(name)) + "-page"
	candidate := base + ".xhtml"
	for i := 1; exists(candidate) && i < maxPageNames; i++ {
		candidate = base + "-" + strconv.Itoa(i) + ".xhtml"
	}
	return candidate
}
