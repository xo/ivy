// Package markdown opens Markdown files.
//
// The package converts the text into an HTML document with a style sheet,
// for the layout of ivy (D10, D21). It reads CommonMark with the copy of
// goldmark in internal/goldmark (D7), and the extensions of GitHub for
// tables, strikethrough, links without brackets and task lists. It also
// reads footnotes and definition lists. HTML in the text stays in the
// document. The layout puts the document on pages of US Letter, and Page
// lays it out only as far as the page that a program asks for.
//
// An image of the text is a file next to the Markdown file. An ivy.Source
// gives no folder, so a program gives it with WithDir (D22). Without it, or
// for an image that the package cannot read, the package puts the alternate
// text of the image in its place. An image with a URL of the scheme data
// stays in the document.
package markdown

import (
	"bytes"
	"context"
	"fmt"
	"io/fs"
	"log/slog"
	"strings"
	"sync"

	"github.com/xo/ivy"
	"github.com/xo/ivy/internal/ebook"
	"github.com/xo/ivy/internal/goldmark/ast"
	"github.com/xo/ivy/internal/goldmark/extension"
	"github.com/xo/ivy/internal/goldmark/parser"
	"github.com/xo/ivy/internal/goldmark/renderer/html"
	"github.com/xo/ivy/internal/layout"
)

// Format describes the package for the registry of a program.
var Format = ivy.Format{
	Name:  "markdown",
	Mimes: []string{"text/markdown", "text/x-markdown"},
	Exts:  []string{"md", "markdown"},
	Open: func(ctx context.Context, src ivy.Source, cfg ivy.Config) (ivy.Document, error) {
		return Open(ctx, src, WithConfig(cfg))
	},
}

// Option is an option of Open.
type Option func(*options)

type options struct {
	cfg ivy.Config
	dir fs.FS
}

// WithConfig sets the configuration that every format shares.
func WithConfig(cfg ivy.Config) Option {
	return func(o *options) {
		o.cfg = cfg
	}
}

// WithDir gives the folder of the Markdown file. The document reads its
// images from it, within the limits of the configuration. A link out of the
// folder, an absolute path and a URL with a scheme are not read. Without a
// folder, the document shows the alternate text of each image (D22).
func WithDir(dir fs.FS) Option {
	return func(o *options) {
		o.dir = dir
	}
}

// Document is an open Markdown file.
type Document struct {
	lim ivy.Limits
	log *slog.Logger
	// fonts are the fonts of the configuration, for the layout.
	fonts ebook.Fonts
	// paper is the paper of the configuration, or zero (D32).
	paper ivy.Paper
	dir   fs.FS
	text  []byte
	title string

	mu     sync.Mutex
	src    *layout.Source
	srcErr error
	// pages holds the book of the layout.
	pages ebook.Pages
}

// The capabilities of a document.
var (
	_ ivy.Pager     = (*Document)(nil)
	_ ivy.Describer = (*Document)(nil)
)

// Open opens a Markdown file. It parses the text, to find the title. It
// returns an error that wraps ivy.ErrFormat if src holds a NUL byte, which
// text does not hold, and one that wraps ivy.ErrLimit if the text nests
// deeper than the limit of depth.
func Open(ctx context.Context, src ivy.Source, opts ...Option) (d *Document, err error) {
	var o options
	for _, opt := range opts {
		opt(&o)
	}
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("opening markdown: %w", err)
	}
	defer ebook.Recover("opening markdown", &err)
	lim := o.cfg.Limits.WithDefaults()
	size := src.Size()
	switch {
	case size < 0:
		return nil, fmt.Errorf("markdown of %d bytes: %w", size, ivy.ErrFormat)
	case size > lim.MaxMemory:
		return nil, fmt.Errorf("markdown of %d bytes: %w", size, ivy.ErrLimit)
	}
	buf := make([]byte, size)
	if size > 0 {
		if _, err := src.ReadAt(buf, 0); err != nil {
			return nil, fmt.Errorf("reading markdown: %w", err)
		}
	}
	text := []byte(ebook.Text(buf))
	if bytes.IndexByte(text, 0) >= 0 {
		return nil, fmt.Errorf("markdown with a nul byte: %w", ivy.ErrFormat)
	}
	text = text[frontMatter(text):]
	d = &Document{lim: lim, log: o.cfg.Log(), fonts: ebook.FontsOf(o.cfg), paper: o.cfg.Paper, dir: o.dir, text: text}
	doc := parse(text)
	if err := checkTree(doc, lim); err != nil {
		return nil, err
	}
	d.title = title(doc, text)
	return d, nil
}

// maxFrontMatter is the number of lines that frontMatter reads to find the
// end of the front matter.
const maxFrontMatter = 256

// frontMatter returns the length of the front matter at the start of text,
// or 0. Front matter is metadata in YAML, such as the title of a page,
// between two lines of three dashes. The first line after the dashes must
// be a key and a colon. calibre leaves the front matter out of the pages,
// and so does the package. CommonMark would read the dashes as a line and
// the metadata as a heading.
func frontMatter(text []byte) int {
	rest, ok := bytes.CutPrefix(text, []byte("---"))
	if !ok {
		return 0
	}
	line, rest, ok := bytes.Cut(rest, []byte("\n"))
	if !ok || len(bytes.TrimSpace(line)) != 0 {
		return 0
	}
	for i := 0; i < maxFrontMatter && len(rest) > 0; i++ {
		line, after, _ := bytes.Cut(rest, []byte("\n"))
		trimmed := bytes.TrimSpace(line)
		switch {
		case i == 0 && !isKey(trimmed):
			return 0
		case i > 0 && (string(trimmed) == "---" || string(trimmed) == "..."):
			return len(text) - len(after)
		}
		rest = after
	}
	return 0
}

// isKey reports whether line starts with a key of YAML and a colon, such as
// "title:".
func isKey(line []byte) bool {
	key, _, ok := bytes.Cut(line, []byte(":"))
	if !ok || len(key) == 0 {
		return false
	}
	for _, c := range key {
		if c != '-' && c != '_' && (c < '0' || c > '9') && (c < 'a' || c > 'z') && (c < 'A' || c > 'Z') {
			return false
		}
	}
	return true
}

// parse parses the text with the extensions of the package.
func parse(text []byte) ast.Node {
	p := parser.New(
		parser.WithAutoHeadingID(),
		parser.WithExtensions(
			extension.NewGFMParser(),
			extension.NewFootnoteParser(),
			extension.NewDefinitionListParser(),
		),
	)
	return p.Parse(text)
}

// render renders the tree of the text as HTML.
func render(doc ast.Node, text []byte) (string, error) {
	r := html.New(
		html.WithUnsafe(),
		html.WithExtensions(
			extension.NewGFMHTMLRenderer(),
			// a link back from a note does nothing on a page, and the
			// default fonts have no glyph for its arrow
			extension.NewFootnoteHTMLRenderer(extension.WithBacklinkHTML("")),
			extension.NewDefinitionListHTMLRenderer(),
		),
	)
	var b strings.Builder
	if err := r.Render(&b, text, doc); err != nil {
		return "", fmt.Errorf("rendering markdown: %w", err)
	}
	return b.String(), nil
}

// checkTree returns an error that wraps ivy.ErrLimit if the tree nests
// deeper than lim.MaxDepth or holds more than lim.MaxObjects nodes. The
// renderer walks the tree by recursion, so a deep tree must not reach it.
// checkTree itself does not recurse.
func checkTree(doc ast.Node, lim ivy.Limits) error {
	count, depth := 0, 0
	for n := doc; n != nil; {
		count++
		if count > lim.MaxObjects {
			return fmt.Errorf("markdown of more than %d nodes: %w", lim.MaxObjects, ivy.ErrLimit)
		}
		if c := n.FirstChild(); c != nil {
			depth++
			if depth > lim.MaxDepth {
				return fmt.Errorf("markdown that nests deeper than %d: %w", lim.MaxDepth, ivy.ErrLimit)
			}
			n = c
			continue
		}
		for n != nil && n != doc && n.NextSibling() == nil {
			n = n.Parent()
			depth--
		}
		if n == nil || n == doc {
			break
		}
		n = n.NextSibling()
	}
	return nil
}

// title returns the text of the first heading of level 1, or "".
func title(doc ast.Node, text []byte) string {
	for n := doc.FirstChild(); n != nil; n = n.NextSibling() {
		if h, ok := n.(*ast.Heading); ok && h.Level == 1 {
			return strings.Join(strings.Fields(plainText(h, text)), " ")
		}
	}
	return ""
}

// plainText returns the text of the inline descendants of n.
func plainText(n ast.Node, text []byte) string {
	var b strings.Builder
	for c := n.FirstChild(); c != nil; c = c.NextSibling() {
		switch c := c.(type) {
		case *ast.Text:
			b.WriteString(c.Value.Value(text))
			if c.SoftLineBreak() || c.HardLineBreak() {
				b.WriteByte(' ')
			}
		case *ast.CodeSpan:
			b.WriteString(c.Value.Value(text))
		default:
			b.WriteString(plainText(c, text))
		}
	}
	return b.String()
}

// Close satisfies the ivy.Document interface.
func (d *Document) Close() error {
	return nil
}

// PageCount satisfies the ivy.Pager interface. It lays out the whole
// document on its first call.
func (d *Document) PageCount(ctx context.Context) (int, error) {
	return d.pages.PageCount(ctx, d.source, d.config())
}

// Page satisfies the ivy.Pager interface. It lays out the document only as
// far as page n.
func (d *Document) Page(ctx context.Context, n int) (ivy.Page, error) {
	return d.pages.Page(ctx, d.source, d.config(), n)
}

// config returns the configuration of the layout.
func (d *Document) config() ivy.Config {
	return d.fonts.Config(d.lim, d.log)
}

// Describe satisfies the ivy.Describer interface. The title is the text of
// the first heading of level 1. Markdown says nothing else about the
// document.
func (d *Document) Describe(ctx context.Context) (ivy.Info, error) {
	if err := ctx.Err(); err != nil {
		return ivy.Info{}, fmt.Errorf("describing markdown: %w", err)
	}
	return ivy.Info{Title: d.title}, nil
}

// source converts the text into HTML for the layout. It converts the text
// once, and returns the same result on each later call.
func (d *Document) source(ctx context.Context) (src layout.Source, err error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	switch {
	case d.srcErr != nil:
		return layout.Source{}, d.srcErr
	case d.src != nil:
		return *d.src, nil
	}
	defer func() {
		switch {
		case err == nil:
			d.src = &src
		case ctx.Err() == nil:
			// an error of the context is not kept, so that a later call can
			// convert the text
			d.srcErr = err
		}
	}()
	// Recover runs before the function above, which then keeps its error
	defer ebook.Recover("converting markdown", &err)
	return d.convert(ctx)
}
