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
)

// documentName is the path of the HTML document in the FS of the source.
const documentName = "index.html"

// maxLangScan is the number of bytes at the start of the text that give
// its language.
const maxLangScan = 64 << 10

// styleSheet is the style sheet of the format. It comes after
// ebook.PaperStyle, which sets the page, and ebook.BodyStyle. The rules
// match the pages that calibre makes of a Markdown file with a font of
// 16px without serifs. They come from measurements of the PDF files of
// ebook-convert, not from the code of calibre, which is under the GPL (D7).
// calibre draws the HTML with the default styles of Chromium:
//
//   - The text is 16px, 12pt, and a line is 18px high, the normal height of
//     a line of Liberation Sans.
//   - A paragraph has a margin of 1em above and below it. The headings are
//     bold, and an h2 is 24px with a margin of 0.83em: the default styles
//     of the layout give the same sizes and margins.
//   - Code is 16px, as large as the text, and a line of a block of code is
//     18px high.
//   - Code, quotes and tables have no border and no background.
//
// calibre does not wrap the lines of a block of code. When a line is wider
// than the page, it draws the whole document smaller. ivy wraps the line
// instead. The style sheet of WeasyPrint 70.0 makes each input a block, so
// a box of a task list sets its display again, to stay on the line of its
// text.
const styleSheet = `html { font-family: sans-serif; font-size: 16px; }
h1, h2, h3, h4, h5, h6 { page-break-after: avoid; }
code, kbd, pre, samp, tt { font-family: monospace; font-size: 1em; }
pre { white-space: pre-wrap; overflow-wrap: anywhere; }
li > input[type="checkbox"] { display: inline-block; margin-right: 0.4em; }
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
	body, err := render(doc, d.text)
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
func (d *Document) images(ctx context.Context, doc ast.Node, files *memfs.FS) error {
	var imgs []*ast.Image
	err := ast.Walk(doc, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if img, ok := n.(*ast.Image); ok && entering {
			imgs = append(imgs, img)
		}
		return ast.WalkContinue, nil
	})
	if err != nil {
		return fmt.Errorf("walking markdown: %w", err)
	}
	var total int64
	read := make(map[string]string)
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
			p = d.readImage(name, len(read), &total, files)
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

// resolve returns the path in the folder of the file of the image dest. It
// returns false if the document has no folder, or if dest is a URL with a
// scheme, an absolute path or a path out of the folder.
func (d *Document) resolve(dest string) (string, bool) {
	if d.dir == nil || strings.HasPrefix(dest, "/") || strings.HasPrefix(dest, `\`) {
		return "", false
	}
	return ebook.Resolve("", dest)
}

// readImage reads the image name from the folder into files, within the
// limits, and returns its path in files. It returns "" if the image is
// missing or too large. n is the number of the image, and total is the
// number of bytes of the images that it read before.
func (d *Document) readImage(name string, n int, total *int64, files *memfs.FS) string {
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
	p := "images/" + strconv.Itoa(n) + strings.ToLower(path.Ext(name))
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
