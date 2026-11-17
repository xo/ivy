// Package text opens text files and TXTZ files.
//
// A TXTZ file is a zip archive that calibre writes: index.txt, its images,
// metadata.opf and a cover image. The cover image is the preview, and
// metadata.opf gives the metadata. A text file has no preview and no
// metadata.
//
// The package converts the text into a document of HTML. A blank line ends a
// paragraph, and the style sheet keeps each break of a line and each run of
// spaces. The text is UTF-8 or UTF-16 with a byte order mark, or UTF-8
// without one. Text that is not valid UTF-8 is read as windows-1252. Markdown
// is the package markdown.
//
// The document takes the language of the text: the language of the header of
// a Project Gutenberg file, or else the East Asian language that the letters
// show. The layout takes the fonts of that language for the CJK characters.
//
// The layout of phase 4 puts the document on pages (D10, D15): a page of US
// Letter in a monospace font for a text file, and the page of an ebook for a
// TXTZ file. Page lays out the document only as far as the page that a
// program asks for.
package text

import (
	"archive/zip"
	"context"
	"fmt"
	"html"
	"io"
	"io/fs"
	"iter"
	"log/slog"
	"path"
	"strings"

	"github.com/xo/ivy"
	"github.com/xo/ivy/internal/ebook"
	"github.com/xo/ivy/internal/layout"
	"github.com/xo/ivy/internal/memfs"
	"github.com/xo/ivy/internal/zipdoc"
)

// Format describes the package for the registry of a program.
var Format = ivy.Format{
	Name:  "text",
	Mimes: []string{"text/plain", "application/txtz+zip"},
	Exts:  []string{"txt", "text", "txtz"},
	Open: func(ctx context.Context, src ivy.Source, cfg ivy.Config) (ivy.Document, error) {
		return Open(ctx, src, WithConfig(cfg))
	},
}

// The names of the parts of a TXTZ file, and of the document of a text file.
const (
	indexName    = "index.txt"
	metadataName = "metadata.opf"
	documentName = "text.html"
)

// textStyle returns the style sheet of a text file: a page of US Letter, or
// of the paper p if it is not zero (D32), and a monospace font, so that a text with fixed columns, such as a table, keeps
// them. ebook.BodyStyle follows it. The size of the font and the space
// between the paragraphs match the pages that calibre makes of a text file
// with a font of 13px. They come from measurements of the PDF files of
// ebook-convert, not from the code of calibre, which is under the GPL (D7):
//
//   - The font is 13px, 9.75pt. calibre takes a size in pixels, so a font of
//     10pt is not possible. A line is 15px high.
//   - A paragraph has a margin of 1em above and below it. The first line of
//     the first page is 1em below the margin of the page.
func textStyle(p ivy.Paper) string {
	return `@page {
  size: ` + ebook.PageSize(p, "letter") + `;
  margin: 0.75in;
}
html {
  font-family: monospace;
  font-size: 13px;
}
p {
  white-space: pre-wrap;
  margin: 1em 0;
}
`
}

// bookStyle is the style sheet of a TXTZ file, after the style sheet of an
// ebook. The page and the font are those of an ebook.
const bookStyle = `p {
  white-space: pre-wrap;
}
`

// Option is an option of Open.
type Option func(*options)

type options struct {
	cfg ivy.Config
}

// WithConfig sets the configuration that every format shares.
func WithConfig(cfg ivy.Config) Option {
	return func(o *options) {
		o.cfg = cfg
	}
}

// Document is an open text or TXTZ file.
type Document struct {
	lim ivy.Limits
	log *slog.Logger
	// fonts are the fonts of the configuration, for the layout.
	fonts ebook.Fonts
	// paper is the paper of the configuration, or zero (D32).
	paper ivy.Paper
	// fsys holds the document of HTML that the package makes from the text,
	// and the files of a TXTZ file.
	fsys fs.FS
	// doc is the path of the document of HTML in fsys.
	doc string
	// archive is the archive of a TXTZ file, or nil.
	archive *zipdoc.Archive
	// pkg is the metadata.opf of a TXTZ file, or nil.
	pkg *ebook.Package
	// pages holds the book of the layout.
	pages ebook.Pages
}

// The capabilities of a document.
var (
	_ ivy.Pager     = (*Document)(nil)
	_ ivy.Previewer = (*Document)(nil)
	_ ivy.Describer = (*Document)(nil)
)

// Open opens a text file or a TXTZ file, and converts the text into a
// document of HTML. A source that starts as a zip archive is a TXTZ file,
// and any other source is a text file. It returns an error that wraps
// ivy.ErrFormat if a TXTZ file holds no text, and one that wraps
// ivy.ErrLimit if the text or its document of HTML is larger than
// Limits.MaxMemory.
func Open(ctx context.Context, src ivy.Source, opts ...Option) (d *Document, err error) {
	var o options
	for _, opt := range opts {
		opt(&o)
	}
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("opening text: %w", err)
	}
	defer ebook.Recover("opening text", &err)
	d = &Document{lim: o.cfg.Limits.WithDefaults(), log: o.cfg.Log(), fonts: ebook.FontsOf(o.cfg), paper: o.cfg.Paper}
	var magic [4]byte
	if n, _ := src.ReadAt(magic[:], 0); n == len(magic) && string(magic[:]) == "PK\x03\x04" {
		err = d.openTXTZ(src)
	} else {
		err = d.openText(src)
	}
	if err != nil {
		return nil, err
	}
	return d, nil
}

// openText reads a text file into memory.
func (d *Document) openText(src ivy.Source) error {
	if size := src.Size(); size > d.lim.MaxMemory {
		return fmt.Errorf("text of %d bytes: %w", size, ivy.ErrLimit)
	}
	data, err := io.ReadAll(io.NewSectionReader(src, 0, src.Size()))
	if err != nil {
		return fmt.Errorf("reading text: %w", err)
	}
	doc, err := d.convert(data, "", "")
	if err != nil {
		return err
	}
	var fsys memfs.FS
	fsys.Add(documentName, doc)
	d.fsys, d.doc = &fsys, documentName
	return nil
}

// openTXTZ opens a TXTZ file. It reads metadata.opf, and converts the text
// into a document of HTML next to the text, so that a link to an image is
// relative to the same folder.
func (d *Document) openTXTZ(src ivy.Source) error {
	archive, err := zipdoc.Open(src, d.lim)
	if err != nil {
		return err
	}
	d.archive = archive
	if f := archive.Find(metadataName); f != nil {
		buf, err := archive.Read(f)
		if err == nil {
			d.pkg, err = ebook.ParseOPF(buf, ".", d.lim)
		}
		if err != nil {
			d.log.Debug("txtz: metadata left out", "err", err)
		}
	}
	name, f, ok := firstText(archive)
	if !ok {
		return fmt.Errorf("txtz without a text file: %w", ivy.ErrFormat)
	}
	data, err := archive.Read(f)
	if err != nil {
		return fmt.Errorf("reading %s: %w", name, err)
	}
	var title, lang string
	if d.pkg != nil {
		title, lang = d.pkg.Metadata.Title, d.pkg.Metadata.Language
	}
	doc, err := d.convert(data, title, lang)
	if err != nil {
		return err
	}
	fsys := archive.FS()
	d.doc = name + ".html"
	if !fsys.Add(d.doc, doc) {
		return fmt.Errorf("txtz with a folder named %s: %w", d.doc, ivy.ErrFormat)
	}
	d.fsys = fsys
	return nil
}

// firstText returns index.txt, or else the text file of the archive that is
// nearest to the root, and first in the order of the names among those.
func firstText(archive *zipdoc.Archive) (string, *zip.File, bool) {
	if f := archive.Find(indexName); f != nil {
		return indexName, f, true
	}
	var best string
	var bestFile *zip.File
	for _, f := range archive.Files() {
		name, ok := zipdoc.CleanName(f.Name)
		if !ok || strings.HasSuffix(f.Name, "/") {
			continue
		}
		switch strings.ToLower(path.Ext(name)) {
		case ".txt", ".text":
		default:
			continue
		}
		if best == "" || depth(name) < depth(best) || depth(name) == depth(best) && name < best {
			best, bestFile = name, f
		}
	}
	return best, bestFile, best != ""
}

// depth returns the number of folders above name.
func depth(name string) int {
	return strings.Count(name, "/")
}

// convert converts the text data into a document of HTML, within the limit
// of memory. An empty lang takes the language that the text shows.
func (d *Document) convert(data []byte, title, lang string) ([]byte, error) {
	text := ebook.Text(data)
	if lang == "" {
		lang = ebook.TextLanguage(text)
	}
	doc := toHTML(text, title, lang)
	if int64(len(doc)) > d.lim.MaxMemory {
		return nil, fmt.Errorf("document of html of %d bytes: %w", len(doc), ivy.ErrLimit)
	}
	return doc, nil
}

// toHTML converts text into a document of HTML. Each run of lines between
// blank lines is a paragraph, and each paragraph keeps its breaks of lines.
// A line that holds only white space is blank. A zero byte becomes U+FFFD.
// An empty title or lang is left out.
func toHTML(text, title, lang string) []byte {
	text = strings.NewReplacer("\r\n", "\n", "\r", "\n", "\x00", "�").Replace(text)
	var b strings.Builder
	b.Grow(len(text) + len(text)/8 + 256)
	b.WriteString("<!DOCTYPE html>\n<html")
	if lang != "" {
		b.WriteString(` lang="` + html.EscapeString(lang) + `"`)
	}
	b.WriteString(">\n<head>\n<meta charset=\"utf-8\">\n")
	if title != "" {
		b.WriteString("<title>" + html.EscapeString(title) + "</title>\n")
	}
	b.WriteString("</head>\n<body>\n")
	open := false
	for line := range strings.Lines(text) {
		line = strings.TrimSuffix(line, "\n")
		if strings.TrimSpace(line) == "" {
			if open {
				b.WriteString("</p>\n")
				open = false
			}
			continue
		}
		if open {
			b.WriteByte('\n')
		} else {
			b.WriteString("<p>")
			open = true
		}
		b.WriteString(html.EscapeString(line))
	}
	if open {
		b.WriteString("</p>\n")
	}
	b.WriteString("</body>\n</html>\n")
	return []byte(b.String())
}

// Close satisfies the ivy.Document interface.
func (d *Document) Close() error {
	return nil
}

// PageCount satisfies the ivy.Pager interface. It lays out the whole
// document on its first call.
func (d *Document) PageCount(ctx context.Context) (int, error) {
	return d.pages.PageCount(ctx, d.layoutSource, d.config())
}

// Page satisfies the ivy.Pager interface. It lays out the document only as
// far as page n.
func (d *Document) Page(ctx context.Context, n int) (ivy.Page, error) {
	return d.pages.Page(ctx, d.layoutSource, d.config(), n)
}

// layoutSource returns the source of the layout.
func (d *Document) layoutSource(context.Context) (layout.Source, error) {
	return d.source(), nil
}

// config returns the configuration of the layout.
func (d *Document) config() ivy.Config {
	return d.fonts.Config(d.lim, d.log)
}

// Previews satisfies the ivy.Previewer interface. It lists the cover image
// of a TXTZ file: the cover that metadata.opf names, or a file named
// cover.jpg or cover.png in the root. A text file has no preview.
func (d *Document) Previews(ctx context.Context) iter.Seq2[ivy.Preview, error] {
	if d.archive == nil {
		return func(func(ivy.Preview, error) bool) {}
	}
	return ebook.CoverPreviews(ctx, d.archive, d.pkg)
}

// Describe satisfies the ivy.Describer interface. For a TXTZ file, it reads
// metadata.opf. A text file says nothing about itself.
func (d *Document) Describe(ctx context.Context) (ivy.Info, error) {
	if err := ctx.Err(); err != nil {
		return ivy.Info{}, fmt.Errorf("describing text: %w", err)
	}
	if d.pkg == nil {
		return ivy.Info{}, nil
	}
	return d.pkg.Info(), nil
}

// source returns the file as a source for the layout: the document of HTML,
// the files of a TXTZ file, and the style sheets. A TXTZ file is an ebook,
// and takes the page of an ebook. A text file takes textStyle.
func (d *Document) source() layout.Source {
	src := layout.Source{
		FS:          d.fsys,
		Documents:   []string{d.doc},
		StyleSheets: []string{textStyle(d.paper), ebook.BodyStyle},
	}
	if d.archive != nil {
		src.StyleSheets = []string{ebook.BookStyleFor(d.paper), bookStyle}
	}
	if d.pkg != nil {
		src.Title = d.pkg.Metadata.Title
	}
	return src
}
