// Package text opens text files and TXTZ files.
//
// A TXTZ file is a zip archive that calibre writes: index.txt, its images,
// metadata.opf and a cover image. The cover image is the preview, and
// metadata.opf gives the metadata. A text file has no preview and no
// metadata. A TXTZ file whose entries ZipCrypto or the AES of WinZip
// encrypts opens with the password of ivy.Config, or with its function
// PasswordFunc (D12).
//
// The package converts the text into a document of HTML. A blank line ends a
// paragraph. A text file joins the lines of a paragraph, which the layout
// wraps to the page, as calibre does with a book that a program wrapped,
// such as a book of Project Gutenberg (D34). A line that starts with white
// space, such as a space, a tab or an ideographic space, starts a new line,
// so that a table, code, a poem with an indent and a paragraph with an
// indent keep their lines. A text file in Chinese or Japanese, and a TXTZ
// file, keep each break of a line, as calibre does (D68). The style sheet
// keeps each run of spaces.
//
// The text is UTF-8 or UTF-16 with a byte order mark, or UTF-8 without one.
// Text that is not valid UTF-8 is read as windows-1252. Markdown is the
// package markdown.
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
	"unicode"
	"unicode/utf8"

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
// ivy.ErrFormat if a TXTZ file holds no text, one that wraps
// ivy.ErrPassword if no password opens an encrypted TXTZ file, and one that
// wraps ivy.ErrLimit if the text or its document of HTML is larger than
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
		err = d.openTXTZ(ctx, src, o.cfg)
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
	doc, err := d.convert(data, "", "", true)
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
// relative to the same folder. It asks for the password of an archive that
// encrypts its entries (D12).
func (d *Document) openTXTZ(ctx context.Context, src ivy.Source, cfg ivy.Config) error {
	archive, err := zipdoc.Open(src, d.lim)
	if err != nil {
		return err
	}
	if err := archive.UnlockSmallest(ctx, cfg); err != nil {
		return fmt.Errorf("opening txtz: %w", err)
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
	// a TXTZ file keeps the lines of its text (D34)
	doc, err := d.convert(data, title, lang, false)
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
// of memory. An empty lang takes the language that the text shows. If join
// is set, the lines of a paragraph join, as toHTML says, unless the text is
// in Chinese or Japanese, which keeps its lines, as calibre does (D68).
func (d *Document) convert(data []byte, title, lang string, join bool) ([]byte, error) {
	text := ebook.Text(data)
	if lang == "" {
		lang = ebook.TextLanguage(text)
	}
	if join && wideText(text) {
		join = false
	}
	doc := toHTML(text, title, lang, join)
	if int64(len(doc)) > d.lim.MaxMemory {
		return nil, fmt.Errorf("document of html of %d bytes: %w", len(doc), ivy.ErrLimit)
	}
	return doc, nil
}

// toHTML converts text into a document of HTML. Each run of lines between
// blank lines is a paragraph. A line that holds only white space is blank.
// A zero byte becomes U+FFFD. An empty title or lang is left out.
//
// If join is not set, each paragraph keeps its breaks of lines. If join is
// set, a line joins the line before it, with one space, and without the
// space next to a letter of Chinese or Japanese, which does not separate
// its words with spaces. A line that starts with white space keeps the
// break before it (D34). A rule line, such as a line of dashes, is a
// paragraph of its own, as it is in the file (D67).
func toHTML(text, title, lang string, join bool) []byte {
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
	// last is the line before, without the white space at its end, if join
	// is set
	var last string
	for line := range strings.Lines(text) {
		line = strings.TrimSuffix(line, "\n")
		if strings.TrimSpace(line) == "" {
			if open {
				b.WriteString("</p>\n")
				open = false
			}
			continue
		}
		if join && ruleLine(line) {
			if open {
				b.WriteString("</p>\n")
				open = false
			}
			b.WriteString("<p>" + html.EscapeString(strings.TrimRightFunc(line, unicode.IsSpace)) + "</p>\n")
			continue
		}
		switch {
		case !open:
			b.WriteString("<p>")
			open = true
		case !join || startsWithSpace(line):
			b.WriteByte('\n')
		default:
			if !noSpace(last, line) {
				b.WriteByte(' ')
			}
		}
		if join {
			line = strings.TrimRightFunc(line, unicode.IsSpace)
		}
		b.WriteString(html.EscapeString(line))
		last = line
	}
	if open {
		b.WriteString("</p>\n")
	}
	b.WriteString("</body>\n</html>\n")
	return []byte(b.String())
}

// ruleLine reports whether line is a rule line: one character of
// punctuation or of a symbol, repeated four times or more, such as ----,
// ==== or ────, with white space around it (D67).
func ruleLine(line string) bool {
	line = strings.TrimSpace(line)
	first, _ := utf8.DecodeRuneInString(line)
	if !unicode.IsPunct(first) && !unicode.IsSymbol(first) {
		return false
	}
	n := 0
	for _, r := range line {
		if r != first {
			return false
		}
		n++
	}
	return n >= 4
}

// wideText reports whether text is in Chinese or Japanese: whether more
// than half of its lines that are not blank are wide lines. A line is wide
// when more than half of its characters that are not white space are wide
// (D68).
func wideText(text string) bool {
	lines, wideLines := 0, 0
	for line := range strings.Lines(text) {
		chars, wideChars := 0, 0
		for _, r := range line {
			switch {
			case unicode.IsSpace(r):
			case wide(r):
				chars++
				wideChars++
			default:
				chars++
			}
		}
		if chars == 0 {
			continue
		}
		lines++
		if 2*wideChars > chars {
			wideLines++
		}
	}
	return 2*wideLines > lines
}

// startsWithSpace reports whether line starts with white space, such as a
// space, a tab or an ideographic space.
func startsWithSpace(line string) bool {
	r, _ := utf8.DecodeRuneInString(line)
	return unicode.IsSpace(r)
}

// noSpace reports whether the line next joins the line before it without
// a space: when the last letter before or the first letter after is a
// letter or a mark of Chinese or Japanese.
func noSpace(before, next string) bool {
	r1, _ := utf8.DecodeLastRuneInString(before)
	r2, _ := utf8.DecodeRuneInString(next)
	return wide(r1) || wide(r2)
}

// wide reports whether r is a letter or a punctuation mark of Chinese or
// Japanese: Han, Hiragana, Katakana, Bopomofo, the punctuation of CJK and
// the forms of full width. Korean separates its words with spaces, so
// Hangul is not wide here.
func wide(r rune) bool {
	switch {
	case unicode.In(r, unicode.Han, unicode.Hiragana, unicode.Katakana, unicode.Bopomofo):
		return true
	case r >= 0x3000 && r <= 0x303f, r >= 0xff00 && r <= 0xff60, r >= 0xffe0 && r <= 0xffe6:
		// the CJK Symbols and Punctuation, and the forms of full width
		return true
	}
	return false
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
