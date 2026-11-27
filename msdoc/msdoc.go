// Package msdoc opens the Word documents of Word 97 to Word 2003: .doc
// files and .dot templates (MS-DOC). It also reads the text of a document
// of Word 6 or Word 95, with the properties of its characters and its
// paragraphs.
//
// A Word document is a Compound File, which the package cfb of magic reads.
// The package reads the File Information Block, the piece table and the text
// of the main document, with the bold, the italic, the underline, the size,
// the color, the border and the font of each run, the alignment, the
// indents, the spacing and the tab stops of each paragraph, and the
// headings of the style sheet. It converts the text into HTML, with each
// table as an HTML table, for the layout of ivy (D10). Each section has
// pages of its own size and margins, with its headers, its footers and its
// columns. The layout makes the margins of a page larger for a header or a
// footer that needs it. A section break starts a new page, unless the next
// section is continuous on a page of the same size. The tables have the
// borders, the shading and the merged cells of their rows. The lists have
// their numbers and their bullets, with the indents and the tab stops of
// their levels. A table or a frame that floats is at its position, and the
// text wraps around it when it leaves room beside it. Its vertical
// position from the margin or from the page holds only when it starts a
// page, because the layout places a float at the line of its paragraph.
// The pictures of the stream Data and the shapes of Office Drawing that
// float on the page show. internal/odraw draws the shapes, as it draws
// those of msppt and msxls (D51): the preset shapes, the freeforms, the
// lines with their arrowheads, the groups, the pictures with their crops
// and turns, and the WordArt. Each shape goes into the layout as a box
// whose content is a display list. A text box shows its text over the box
// of its shape, and the text of a shape of a group shows over the box of
// the group. A footnote goes at the foot of its page, and the endnotes go
// at the end of the document. A run gives its East Asian font after its
// font, and the language of its East Asian text.
//
// The package leaves out the comments. A WordArt that bends its text,
// such as an arch, shows its text straight. The codes of the fields
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
	"strconv"
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
	p, err := newParser(s, cfg)
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
	sl := d.parser.sections()
	nameSections(secs, sl)
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
		d.parser.columns = secs[i].page.columns
		d.parser.columnGap = secs[i].page.columnGap
		style := "page: " + secs[i].name
		if v := secs[i].page.lines.css(); v != "" {
			style += "; -ivy-line-numbers: " + v + d.parser.lineNumberFont()
		}
		if n := secs[i].page.columns; n > 1 {
			style += "; columns: " + strconv.Itoa(n) + "; column-gap: " + pt(secs[i].page.columnGap)
			if i+1 >= len(secs) || !sl.continuous(secs[i+1].cp) {
				// Word makes the columns of a section the same height
				// only before a section that continues on the same page,
				// as in the package docx. Else the text fills each column
				// to the bottom of the page
				style += "; column-fill: auto"
			}
		}
		b.WriteString(`<div class="section" style="` + attr(style) + `">`)
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
		Objects:     d.parser.objects,
	}, nil
}
