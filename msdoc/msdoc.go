// Package msdoc opens the Word documents of Word 97 to Word 2003: .doc
// files and .dot templates (MS-DOC). It also reads the text of a document
// of Word 6 or Word 95, with the properties of its characters and its
// paragraphs.
//
// A Word document is a Compound File, which the package cfb of magic reads.
// The package reads the File Information Block, the piece table and the text
// of the main document, with the bold, the italic, the underline, the size,
// the color and the font of each run, the alignment, the indents and the
// spacing of each paragraph, and the headings of the style sheet. It
// converts the text into HTML, with each table as an HTML table, for the
// layout of ivy (D10). Each section has pages of its own size and margins,
// with its headers and its footers. A section break starts a new page,
// unless the next section is continuous on a page of the same size. The
// tables have the borders, the shading and the merged cells of their rows,
// the lists have their numbers and their bullets, and the pictures of the
// stream Data and the pictures of Office Drawing that float on the page
// show. The text boxes show with their text, and the rectangles, the
// ellipses, the lines, the freeforms and the groups of Office Drawing show
// as SVG images. A footnote goes at the foot of its page, and the
// endnotes go at the end of the document. A run gives its East Asian font
// after its font, and the language of its East Asian text.
//
// The package leaves out the comments, the WordArt, and the shapes of
// Office Drawing whose geometry it does not know. The codes of the fields
// are left out, and their results stay, but the page numbers of the fields
// PAGE and NUMPAGES come from the layout. Of a document of Word 6 or Word
// 95, the package leaves out the headers, the footers, the footnotes, the
// text boxes, the lists, the pictures and the properties of the tables.
//
// A document gives its thumbnail and its properties from the stream
// "\x05SummaryInformation". A document that a password protects with RC4,
// RC4 CryptoAPI or the XOR obfuscation asks for the password as D12 says.
// The package returns an error that wraps ivy.ErrUnsupported for a
// document of Word 6 or Word 95 that a password protects.
package msdoc

import (
	"context"
	"fmt"
	"iter"
	"strings"
	"sync"

	"github.com/xo/ivy"
	"github.com/xo/ivy/internal/ebook"
	"github.com/xo/ivy/internal/layout"
	"github.com/xo/ivy/internal/memfs"
	"github.com/xo/ivy/internal/msole"
)

// Format describes the package for the registry of a program.
var Format = ivy.Format{
	Name:  "msdoc",
	Mimes: []string{"application/msword", "application/vnd.ms-word", "application/x-msword"},
	Exts:  []string{"doc", "dot"},
	Open: func(ctx context.Context, src ivy.Source, cfg ivy.Config) (ivy.Document, error) {
		return Open(ctx, src, WithConfig(cfg))
	},
}

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

// documentName is the name of the HTML document in the source of the
// layout.
const documentName = "document.html"

// Document is an open Word document.
type Document struct {
	cfg     ivy.Config
	summary msole.Summary
	lang    string
	streams *streams
	parser  *parser

	mu     sync.Mutex
	src    *layout.Source
	srcErr error
	// pages holds the book of the layout.
	pages ebook.Pages
}

// The capabilities of a document.
var (
	_ ivy.Pager     = (*Document)(nil)
	_ ivy.Previewer = (*Document)(nil)
	_ ivy.Describer = (*Document)(nil)
)

// Open opens a Word document. It reads the summary information, the FIB,
// the piece table, the style sheet and the formatted disk pages, and
// decrypts the document if a password protects it. It returns an error that
// wraps ivy.ErrFormat if src is not a Word document, one that wraps
// ivy.ErrPassword if the password is missing or wrong, and one that wraps
// ivy.ErrUnsupported for a document of Word 6 or Word 95 with a password,
// or for a FIB of Word 97 with the nFib of an older version.
func Open(ctx context.Context, src ivy.Source, opts ...Option) (d *Document, err error) {
	var o options
	for _, opt := range opts {
		opt(&o)
	}
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("opening word document: %w", err)
	}
	defer ebook.Recover("opening word document", &err)
	cfg := o.cfg
	cfg.Limits = cfg.Limits.WithDefaults()
	f, err := msole.Open(src, cfg.Limits)
	if err != nil {
		return nil, err
	}
	summary, err := msole.ReadSummary(f, cfg.Limits)
	if err != nil {
		return nil, err
	}
	s, err := readStreams(ctx, f, cfg)
	if err != nil {
		return nil, err
	}
	p, err := newParser(s, cfg.Limits)
	if err != nil {
		return nil, err
	}
	return &Document{
		cfg:     cfg,
		summary: summary,
		lang:    ebook.LCIDLanguage(uint32(s.fib.lid)),
		streams: s,
		parser:  p,
	}, nil
}

// Template reports whether the document is a template, a .dot file.
func (d *Document) Template() bool {
	return d.streams.fib.flags&flagDot != 0
}

// Close satisfies the ivy.Document interface.
func (d *Document) Close() error {
	return nil
}

// Previews satisfies the ivy.Previewer interface. It lists the thumbnail of
// the summary information, which Word writes when a person asks for a
// preview picture.
func (d *Document) Previews(ctx context.Context) iter.Seq2[ivy.Preview, error] {
	return d.summary.Previews(ctx, d.cfg)
}

// Describe satisfies the ivy.Describer interface. It returns the
// properties of the summary information, and the language of the FIB.
func (d *Document) Describe(ctx context.Context) (ivy.Info, error) {
	if err := ctx.Err(); err != nil {
		return ivy.Info{}, fmt.Errorf("describing word document: %w", err)
	}
	info := d.summary.Info
	if info.Language == "" {
		info.Language = d.lang
	}
	return info, nil
}

// PageCount satisfies the ivy.Pager interface. It lays out the whole
// document on its first call.
func (d *Document) PageCount(ctx context.Context) (int, error) {
	return d.pages.PageCount(ctx, d.source, d.cfg)
}

// Page satisfies the ivy.Pager interface. It lays out the document only as
// far as page n.
func (d *Document) Page(ctx context.Context, n int) (ivy.Page, error) {
	return d.pages.Page(ctx, d.source, d.cfg, n)
}

// source converts the text into HTML for the layout, once.
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
	defer ebook.Recover("converting word document", &err)
	return d.convert(ctx)
}

// convert reads the paragraphs and writes them as an HTML document.
func (d *Document) convert(ctx context.Context) (layout.Source, error) {
	paras, err := d.parser.paragraphs(ctx)
	if err != nil {
		return layout.Source{}, err
	}
	secs, running, err := d.parser.docSections(ctx, d.cfg.Paper)
	if err != nil {
		return layout.Source{}, err
	}
	nameSections(secs, d.parser.sections())
	cur := -1
	// section starts the element of the section of each paragraph, with
	// the running elements of its headers and its footers
	section := func(b *strings.Builder, para paragraph, closeTable func()) {
		i := sectionOf(secs, para.cp)
		if i == cur {
			return
		}
		closeTable()
		if cur >= 0 {
			b.WriteString("</div>\n")
		}
		cur = i
		d.parser.page = secs[i].page.page
		b.WriteString(`<div class="section" style="page: ` + secs[i].name + `">`)
		b.WriteString(running[i])
	}
	body := d.parser.body(paras, section)
	if d.parser.endnotes.Len() > 0 {
		body += `<div class="endnotes">` + d.parser.endnotes.String() + "</div>\n"
	}
	if cur >= 0 {
		body += "</div>\n"
	}
	html := msole.HTML(d.summary.Info.Title, d.lang, "", body)
	if int64(len(html)) > d.cfg.Limits.MaxMemory {
		return layout.Source{}, fmt.Errorf("html of more than %d bytes: %w", d.cfg.Limits.MaxMemory, ivy.ErrLimit)
	}
	var files memfs.FS
	files.Add(documentName, []byte(html))
	for _, m := range d.parser.media {
		files.Add(m.name, m.data)
	}
	return layout.Source{
		FS:          &files,
		Documents:   []string{documentName},
		StyleSheets: []string{d.parser.css(d.parser.firstPage(d.cfg.Paper)), d.parser.sectionCSS(secs)},
		Title:       d.summary.Info.Title,
	}, nil
}
