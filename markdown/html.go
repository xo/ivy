package markdown

import (
	"context"
	"fmt"
	"io/fs"
	"path"
	"strconv"
	"strings"

	"github.com/xo/ivy"
	"github.com/xo/ivy/internal/ebook"
	"github.com/xo/ivy/internal/goldmark/ast"
	"github.com/xo/ivy/internal/goldmark/text"
	"github.com/xo/ivy/internal/layout"
	"github.com/xo/ivy/internal/memfs"
	"github.com/xo/okage/html"
)

// documentName is the path of the HTML document in the FS of the source.
const documentName = "index.html"

// maxLangScan is the number of bytes at the start of the text that give
// its language.
const maxLangScan = 64 << 10

// styleSheet is the style sheet of the format. It comes after
// ebook.PaperStyle, which sets the page, and ebook.BodyStyle. It follows the
// light theme of github-markdown-css 5.8.1, the style sheet of GitHub for a
// README, as D70 decides. The rules come from that
// style sheet, under this license:
//
//	MIT License
//
//	Copyright (c) Sindre Sorhus <sindresorhus@gmail.com> (https://sindresorhus.com)
//
//	Permission is hereby granted, free of charge, to any person obtaining a
//	copy of this software and associated documentation files (the
//	"Software"), to deal in the Software without restriction, including
//	without limitation the rights to use, copy, modify, merge, publish,
//	distribute, sublicense, and/or sell copies of the Software, and to
//	permit persons to whom the Software is furnished to do so, subject to
//	the following conditions:
//
//	The above copyright notice and this permission notice shall be included
//	in all copies or substantial portions of the Software.
//
//	THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS
//	OR IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF
//	MERCHANTABILITY, FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT.
//	IN NO EVENT SHALL THE AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY
//	CLAIM, DAMAGES OR OTHER LIABILITY, WHETHER IN AN ACTION OF CONTRACT,
//	TORT OR OTHERWISE, ARISING FROM, OUT OF OR IN CONNECTION WITH THE
//	SOFTWARE OR THE USE OR OTHER DEALINGS IN THE SOFTWARE.
//
// The style sheet of GitHub styles the descendants of an element of the
// class markdown-body. Here the body of the document takes its place. The
// rules differ from it in these ways, for pages and for the layout of ivy:
//
//   - The fonts are the generic families sans-serif and monospace, which
//     the fonts of the configuration give. GitHub names the fonts of the
//     systems first.
//   - A block of code wraps its long lines, because a page cannot scroll.
//     GitHub scrolls the block.
//   - A table is a table. GitHub makes it a block that scrolls, as wide as
//     its content, so that the table is not wider than the page.
//   - A heading does not end a page, and a row of a table with an image
//     does not break across two pages, so that a grid of images keeps its
//     rows.
//   - The style sheet of WeasyPrint 70.0 makes each input a block, so the
//     box of a task list sets its display again, to stay on the line of
//     its text. goldmark gives the item and the box no class, so the rules
//     of a task list select the items that start with a box.
//   - goldmark puts a line in the footnotes, and GitHub puts a border above
//     them. The line is hidden, and a reference to a footnote takes the
//     brackets of GitHub.
//   - An alert of GitHub is a block quote with the classes of the alert, as
//     alerts makes it. GitHub makes a div, and shows an icon before the
//     title. The title here has no icon.
//   - A details element without the attribute open shows only its summary,
//     after a triangle that points right. The text of the element outside
//     the summary has a size of 0, so that it does not show. An open
//     details element shows a triangle that points down.
//
// The layout reads the attributes of HTML, such as align and width, as
// presentational hints.
const styleSheet = `html { font-family: sans-serif; font-size: 16px; }
body { color: #1f2328; background-color: #ffffff; line-height: 1.5; overflow-wrap: break-word; }
body > :first-child { margin-top: 0 !important; }
a { color: #0969da; text-decoration: none; }
b, strong { font-weight: 600; }
mark { background-color: #fff8c5; color: #1f2328; }
small { font-size: 90%; }
sub, sup { font-size: 75%; line-height: 0; position: relative; vertical-align: baseline; }
sub { bottom: -0.25em; }
sup { top: -0.5em; }
h1, h2, h3, h4, h5, h6 { margin-top: 1.5rem; margin-bottom: 1rem; font-weight: 600; line-height: 1.25;
  page-break-after: avoid; }
h1 { padding-bottom: .3em; font-size: 2em; border-bottom: 1px solid #d1d9e0b3; }
h2 { padding-bottom: .3em; font-size: 1.5em; border-bottom: 1px solid #d1d9e0b3; }
h3 { font-size: 1.25em; }
h4 { font-size: 1em; }
h5 { font-size: .875em; }
h6 { font-size: .85em; color: #59636e; }
h1 code, h2 code, h3 code, h4 code, h5 code, h6 code, h1 tt, h2 tt, h3 tt, h4 tt, h5 tt, h6 tt {
  padding: 0 .2em; font-size: inherit; }
p, blockquote, ul, ol, dl, table, pre, details { margin-top: 0; margin-bottom: 1rem; }
blockquote { margin-left: 0; margin-right: 0; padding: 0 1em; color: #59636e; border-left: .25em solid #d1d9e0; }
blockquote > :first-child { margin-top: 0; }
blockquote > :last-child { margin-bottom: 0; }
hr { box-sizing: content-box; height: .25em; padding: 0; margin: 1.5rem 0; background-color: #d1d9e0; border: 0; }
ul, ol { padding-left: 2em; }
ol ol, ul ol { list-style-type: lower-roman; }
ul ul ol, ul ol ol, ol ul ol, ol ol ol { list-style-type: lower-alpha; }
ul ul, ul ol, ol ol, ol ul { margin-top: 0; margin-bottom: 0; }
li > p { margin-top: 1rem; }
li + li { margin-top: .25em; }
li:has(> input[type="checkbox"]:first-child) { list-style-type: none; }
li > input[type="checkbox"]:first-child { display: inline-block; margin: 0 .2em .25em -1.4em; vertical-align: middle; }
dl { padding: 0; }
dl dt { padding: 0; margin-top: 1rem; font-size: 1em; font-style: italic; font-weight: 600; }
dl dd { margin-left: 0; padding: 0 1rem; margin-bottom: 1rem; }
code, kbd, pre, samp, tt { font-family: monospace; }
code, tt { padding: .2em .4em; margin: 0; font-size: 85%; background-color: #818b981f; border-radius: 6px; }
samp { font-size: 85%; }
pre { padding: 1rem; font-size: 85%; line-height: 1.45; color: #1f2328; background-color: #f6f8fa; border-radius: 6px;
  white-space: pre-wrap; overflow-wrap: anywhere; }
pre code, pre tt { display: inline; padding: 0; margin: 0; font-size: 100%; line-height: inherit;
  background-color: transparent; border: 0; }
kbd { display: inline-block; padding: 0.25rem; font-family: monospace; font-size: 11px; line-height: 10px;
  color: #1f2328; vertical-align: middle; background-color: #f6f8fa; border: solid 1px #d1d9e0b3; border-radius: 6px; }
table { border-spacing: 0; border-collapse: collapse; max-width: 100%; }
td, th { padding: 6px 13px; border: 1px solid #d1d9e0; }
th { font-weight: 600; }
td > :last-child { margin-bottom: 0; }
tr { background-color: #ffffff; border-top: 1px solid #d1d9e0b3; }
tr:nth-child(2n) { background-color: #f6f8fa; }
table img { background-color: transparent; }
img { border-style: none; max-width: 100%; box-sizing: content-box; }
img[align=right i] { padding-left: 20px; }
img[align=left i] { padding-right: 20px; }
tr:has(img) { break-inside: avoid; }
.footnotes { font-size: 12px; color: #59636e; border-top: 1px solid #d1d9e0; }
.footnotes > hr { display: none; }
.footnotes ol { padding-left: 1rem; }
a.footnote-ref::before { content: "["; }
a.footnote-ref::after { content: "]"; }
blockquote.markdown-alert { padding: 0.5rem 1rem; margin-bottom: 1rem; color: inherit; border-left: .25em solid #d1d9e0; }
.markdown-alert > :first-child { margin-top: 0; }
.markdown-alert > :last-child { margin-bottom: 0; }
.markdown-alert .markdown-alert-title { font-weight: 500; line-height: 1; }
.markdown-alert.markdown-alert-note { border-left-color: #0969da; }
.markdown-alert.markdown-alert-note .markdown-alert-title { color: #0969da; }
.markdown-alert.markdown-alert-important { border-left-color: #8250df; }
.markdown-alert.markdown-alert-important .markdown-alert-title { color: #8250df; }
.markdown-alert.markdown-alert-warning { border-left-color: #9a6700; }
.markdown-alert.markdown-alert-warning .markdown-alert-title { color: #9a6700; }
.markdown-alert.markdown-alert-tip { border-left-color: #1a7f37; }
.markdown-alert.markdown-alert-tip .markdown-alert-title { color: #1a7f37; }
.markdown-alert.markdown-alert-caution { border-left-color: #cf222e; }
.markdown-alert.markdown-alert-caution .markdown-alert-title { color: #d1242f; }
details { display: block; }
details > summary:first-of-type { display: list-item; list-style: "\25BA  " inside; }
details[open] > summary:first-of-type { list-style-type: "\25BC  "; }
details:not([open]) { font-size: 0; line-height: 0; }
details:not([open]) > :not(summary:first-of-type) { display: none; }
details:not([open]) > summary:first-of-type { font-size: 1rem; line-height: normal; }
summary h1, summary h2, summary h3, summary h4, summary h5, summary h6 { display: inline-block; }
summary h1, summary h2 { padding-bottom: 0; border-bottom: 0; }
`

// convert converts the text into a layout.Source.
func (d *Document) convert(ctx context.Context) (layout.Source, error) {
	if err := ctx.Err(); err != nil {
		return layout.Source{}, fmt.Errorf("converting markdown: %w", err)
	}
	// the tree is parsed again, because convert changes the images in it
	doc := parse(d.text)
	if err := checkTree(doc, d.lim); err != nil {
		return layout.Source{}, err
	}
	var files memfs.FS
	if err := d.images(ctx, doc, &files); err != nil {
		return layout.Source{}, err
	}
	body, err := render(doc, d.text, d.highlight(doc))
	if err != nil {
		return layout.Source{}, err
	}
	var b strings.Builder
	b.WriteString("<!DOCTYPE html>\n<html")
	if d.lang != "" {
		b.WriteString(` lang="` + escape(d.lang) + `"`)
	}
	b.WriteString(">\n<head>\n<meta charset=\"utf-8\">\n<title>")
	b.WriteString(escape(d.title))
	b.WriteString("</title>\n</head>\n<body>\n")
	b.WriteString(body)
	b.WriteString("</body>\n</html>\n")
	if int64(b.Len()) > d.lim.MaxMemory {
		return layout.Source{}, fmt.Errorf("html of more than %d bytes: %w", d.lim.MaxMemory, ivy.ErrLimit)
	}
	files.Add(documentName, []byte(b.String()))
	return layout.Source{
		FS:          &files,
		Documents:   []string{documentName},
		StyleSheets: []string{ebook.PaperStyleFor(d.paper), ebook.BodyStyle, styleSheet},
		Title:       d.title,
	}, nil
}

// images reads the image of each image of the text into files, and points
// the image to it. An image that it cannot read becomes its alternate
// text. An image with a URL of the scheme data stays as it is.
//
// An img element of the HTML of the text keeps its src, as GitHub shows a
// README. images reads its file into files at the path of the file in the
// folder, so that the src of the element finds it. The layout shows the
// alternate text of an img element whose file is not in files.
func (d *Document) images(ctx context.Context, doc ast.Node, files *memfs.FS) error {
	var imgs []*ast.Image
	var raw strings.Builder
	err := ast.Walk(doc, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}
		switch n := n.(type) {
		case *ast.Image:
			imgs = append(imgs, n)
		case *ast.HTMLBlock:
			_, _ = n.Value.WriteTo(&raw, d.text)
		case *ast.RawHTML:
			_, _ = n.Value.WriteTo(&raw, d.text)
		}
		return ast.WalkContinue, nil
	})
	if err != nil {
		return fmt.Errorf("walking markdown: %w", err)
	}
	var total int64
	// read holds the path in files of each file of the folder, or "" for a
	// file that the document cannot read. used holds the paths in files.
	read := make(map[string]string)
	used := map[string]bool{documentName: true}
	names := d.htmlImages(raw.String())
	for _, name := range names {
		used[name] = true
	}
	for _, name := range names {
		if err := ctx.Err(); err != nil {
			return fmt.Errorf("converting markdown: %w", err)
		}
		if _, ok := read[name]; !ok {
			read[name] = d.readImage(name, name, len(read), &total, files)
		}
	}
	next := 0
	for _, img := range imgs {
		if err := ctx.Err(); err != nil {
			return fmt.Errorf("converting markdown: %w", err)
		}
		dest := strings.TrimSpace(img.Destination.Value(d.text))
		if strings.HasPrefix(strings.ToLower(dest), "data:image/") {
			continue
		}
		name, ok := d.resolve(dest)
		if !ok {
			unwrap(img)
			continue
		}
		p, ok := read[name]
		if !ok {
			// the path of the image is new, so that it is not the path of a
			// file that an img element of the HTML reads
			for {
				p = "images/" + strconv.Itoa(next) + strings.ToLower(path.Ext(name))
				next++
				if !used[p] {
					break
				}
			}
			p = d.readImage(name, p, len(read), &total, files)
			read[name] = p
		}
		if p == "" {
			unwrap(img)
			continue
		}
		img.Destination = text.NewSingleLineValueFromString(p, nil)
	}
	return nil
}

// htmlImages returns the path in the folder of the file of each img element
// of raw, the HTML of the text, in their order and without repeats. It
// leaves out a src that resolve does not accept and a URL of the scheme
// data. The parser is the parser of the layout, so the img elements are the
// ones that the layout reads. htmlImages does not read the source elements
// of a picture element. The layout shows the img element of the picture, as
// GitHub does on a light page.
func (d *Document) htmlImages(raw string) []string {
	if d.dir == nil || !strings.Contains(raw, "<") {
		return nil
	}
	doc, err := html.ParseFragment(strings.NewReader(raw), "body",
		html.WithCharset("utf-8"), html.WithMaxNodes(d.lim.MaxObjects), html.WithMaxDepth(d.lim.MaxDepth))
	if err != nil {
		return nil
	}
	var names []string
	seen := make(map[string]bool)
	for n := doc.Root(); n != nil; n = nextNode(n, doc.Root()) {
		if n.Type() != html.ElementNode || n.Namespace() != html.NamespaceHTML || n.Name() != "img" {
			continue
		}
		src, _ := n.Attr("src")
		src = strings.TrimSpace(src)
		if strings.HasPrefix(strings.ToLower(src), "data:") {
			continue
		}
		name, ok := d.resolve(src)
		if !ok || name == documentName || seen[name] {
			continue
		}
		seen[name] = true
		names = append(names, name)
	}
	return names
}

// nextNode returns the node after n in the order of the document, in the
// tree of root, or nil. It does not recurse.
func nextNode(n, root *html.Node) *html.Node {
	if c := n.FirstChild(); c != nil {
		return c
	}
	for ; n != nil && n != root; n = n.Parent() {
		if s := n.NextSibling(); s != nil {
			return s
		}
	}
	return nil
}

// resolve returns the path in the folder of the file of the image dest. It
// returns false if the document has no folder, or if dest is a URL with a
// scheme, an absolute path or a path out of the folder.
func (d *Document) resolve(dest string) (string, bool) {
	if d.dir == nil || strings.HasPrefix(dest, "/") || strings.HasPrefix(dest, `\`) {
		return "", false
	}
	return ebook.Resolve("", dest)
}

// readImage reads the image name from the folder into files at the path p,
// within the limits, and returns p. It returns "" if the image is missing
// or too large. n is the number of the images that it read before, and
// total is their number of bytes.
func (d *Document) readImage(name, p string, n int, total *int64, files *memfs.FS) string {
	if n >= d.lim.MaxObjects {
		d.log.Warn("markdown image left out, too many images", "name", name)
		return ""
	}
	info, err := fs.Stat(d.dir, name)
	if err != nil || !info.Mode().IsRegular() {
		d.log.Warn("markdown image left out", "name", name, "err", err)
		return ""
	}
	if info.Size() > d.lim.MaxMemory-*total {
		d.log.Warn("markdown image left out, past the limit of memory", "name", name, "size", info.Size())
		return ""
	}
	buf, err := fs.ReadFile(d.dir, name)
	if err != nil || int64(len(buf)) > d.lim.MaxMemory-*total {
		d.log.Warn("markdown image left out", "name", name, "err", err)
		return ""
	}
	*total += int64(len(buf))
	files.Add(p, buf)
	return p
}

// unwrap puts the children of img, its alternate text, in its place.
func unwrap(img *ast.Image) {
	parent := img.Parent()
	if parent == nil {
		return
	}
	for c := img.FirstChild(); c != nil; c = img.FirstChild() {
		parent.InsertBefore(img, c)
	}
	parent.RemoveChild(img)
}

// escape escapes the characters that HTML reserves in text.
func escape(s string) string {
	return strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;").Replace(s)
}
