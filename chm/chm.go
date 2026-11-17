// Package chm opens the help files of Microsoft, Compiled HTML Help, whose
// extension is .chm.
//
// A CHM file holds the files of a help project: its pages of HTML, their
// images and their style sheets. The ITSF header leads to a directory of the
// files, and each file is in section 0, which is not compressed, or in
// section 1, which LZX compresses. The package reads the directory when it
// opens the file, and decodes a file only when the layout reads it, with
// the LZX decoder of github.com/xo/magic/compress/lzx. The format follows
// the description of the ITSF format by Matthew T. Russotto and the
// Unofficial HTML Help Specification.
//
// #SYSTEM gives the title, the language, the default topic and the table of
// contents, and #WINDOWS with #STRINGS gives those that #SYSTEM does not.
// The documents are the default topic, then the files of the table of
// contents in its order, then the other HTML files of the help file.
//
// A help file holds no preview. The layout puts the documents on pages of
// US Letter, one after the other (D10, D21), and Page lays them out only as
// far as the page that a program asks for.
package chm

import (
	"context"
	"fmt"
	"iter"
	"log/slog"
	"path"
	"strings"

	"github.com/xo/ivy"
	"github.com/xo/ivy/internal/ebook"
	"github.com/xo/ivy/internal/layout"
	"github.com/xo/ivy/internal/zipdoc"
)

// Format describes the package for the registry of a program.
var Format = ivy.Format{
	Name:  "chm",
	Mimes: []string{"application/vnd.ms-htmlhelp", "application/x-chm"},
	Exts:  []string{"chm"},
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

// Document is an open CHM file.
type Document struct {
	archive *archive
	lim     ivy.Limits
	log     *slog.Logger
	// fonts are the fonts of the configuration, for the layout.
	fonts ebook.Fonts
	// paper is the paper of the configuration, or zero (D32).
	paper  ivy.Paper
	system system
	// lcid is the LCID of the help file: the one of #SYSTEM, or else the one
	// of the ITSF header.
	lcid uint32
	// charset is the code page of lcid, as the WHATWG Encoding Standard
	// names it. The text of the help file that is not UTF-8 is in it.
	charset string
	// fsys holds the files of the help file, which decode when they open.
	// The size that Stat gives for an HTML file is its size in the help
	// file, before withCharset adds its meta element.
	fsys *zipdoc.FS
	// lower maps the lower case of each path of fsys to the path, because a
	// link of a help file can differ in case from the name of the file.
	lower map[string]string
	// pages holds the book of the layout.
	pages ebook.Pages
}

// The capabilities of a document.
var (
	_ ivy.Pager     = (*Document)(nil)
	_ ivy.Previewer = (*Document)(nil)
	_ ivy.Describer = (*Document)(nil)
)

// Open opens a CHM file. It reads the header, the directory and #SYSTEM.
// It returns an error that wraps ivy.ErrFormat if src is not a CHM file.
func Open(ctx context.Context, src ivy.Source, opts ...Option) (d *Document, err error) {
	var o options
	for _, opt := range opts {
		opt(&o)
	}
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("opening chm: %w", err)
	}
	defer ebook.Recover("opening chm", &err)
	lim := o.cfg.Limits.WithDefaults()
	a, err := readArchive(src, lim)
	if err != nil {
		return nil, err
	}
	d = &Document{archive: a, lim: lim, log: o.cfg.Log(), fonts: ebook.FontsOf(o.cfg), paper: o.cfg.Paper, fsys: zipdoc.NewFS(), lower: make(map[string]string)}
	d.system = a.readSystem()
	d.lcid = a.lcid
	if d.system.hasLCID {
		d.lcid = d.system.lcid
	}
	d.charset = ebook.LCIDCharset(d.lcid)
	charset := d.charset
	for _, name := range a.files() {
		clean, ok := zipdoc.CleanName(name)
		if !ok {
			continue
		}
		html := isHTML(clean)
		load := func() (buf []byte, err error) {
			defer ebook.Recover("reading chm file "+name, &err)
			buf, err = a.read(name)
			if err != nil || !html {
				return buf, err
			}
			return withCharset(buf, charset), nil
		}
		if d.fsys.AddFunc(clean, a.entries[name].length, load) {
			if lower := strings.ToLower(clean); d.lower[lower] == "" {
				d.lower[lower] = clean
			}
		}
	}
	return d, nil
}

// isHTML reports whether the extension of name is one of HTML.
func isHTML(name string) bool {
	switch strings.ToLower(path.Ext(name)) {
	case ".htm", ".html", ".xhtml", ".xht":
		return true
	}
	return false
}

// Close satisfies the ivy.Document interface.
func (d *Document) Close() error {
	return nil
}

// PageCount satisfies the ivy.Pager interface. It lays out all the
// documents on its first call.
func (d *Document) PageCount(ctx context.Context) (int, error) {
	return d.pages.PageCount(ctx, d.layoutSource, d.config())
}

// Page satisfies the ivy.Pager interface. It lays out the documents only as
// far as page n.
func (d *Document) Page(ctx context.Context, n int) (ivy.Page, error) {
	return d.pages.Page(ctx, d.layoutSource, d.config(), n)
}

// layoutSource returns the source of the layout.
func (d *Document) layoutSource(context.Context) (layout.Source, error) {
	return d.source()
}

// config returns the configuration of the layout.
func (d *Document) config() ivy.Config {
	return d.fonts.Config(d.lim, d.log)
}

// Previews satisfies the ivy.Previewer interface. A help file has no
// preview.
func (d *Document) Previews(context.Context) iter.Seq2[ivy.Preview, error] {
	return func(func(ivy.Preview, error) bool) {}
}

// Describe satisfies the ivy.Describer interface. The title is the title
// of #SYSTEM or of #WINDOWS, or else the title of the default topic. The
// language is the one of the LCID of #SYSTEM, or of the ITSF header.
func (d *Document) Describe(ctx context.Context) (info ivy.Info, err error) {
	if err := ctx.Err(); err != nil {
		return ivy.Info{}, fmt.Errorf("describing chm: %w", err)
	}
	defer ebook.Recover("describing chm", &err)
	return ivy.Info{Title: d.title(), Language: ebook.LCIDLanguage(d.lcid)}, nil
}

// title returns the title of the help file.
func (d *Document) title() string {
	if title := strings.TrimSpace(ebook.TextIn(d.system.title, d.charset)); title != "" {
		return title
	}
	if topic, ok := d.topic(); ok {
		if buf, err := d.fsys.ReadFile(topic); err == nil {
			return ebook.ScanHead(ebook.TextIn(buf, d.charset), d.lim).Title
		}
	}
	return ""
}

// lookup returns the path in fsys of the file name, in any case.
func (d *Document) lookup(name string) (string, bool) {
	clean, ok := d.lower[strings.ToLower(name)]
	return clean, ok
}

// topic returns the path of the default topic.
func (d *Document) topic() (string, bool) {
	link := strings.TrimSpace(ebook.TextIn(d.system.topic, d.charset))
	if link == "" {
		return "", false
	}
	name, ok := resolveLink(".", link)
	if !ok {
		return "", false
	}
	return d.lookup(name)
}

// contents returns the files of the table of contents, in its order.
func (d *Document) contents() []string {
	name := strings.TrimSpace(ebook.TextIn(d.system.contents, d.charset))
	clean, ok := "", false
	if name != "" {
		if resolved, okResolve := resolveLink(".", name); okResolve {
			clean, ok = d.lookup(resolved)
		}
	}
	if !ok {
		// a help file without the name of its table of contents often
		// still holds one
		for _, file := range d.archive.files() {
			if strings.EqualFold(path.Ext(file), ".hhc") {
				clean, ok = zipdoc.CleanName(file)
				break
			}
		}
	}
	if !ok {
		return nil
	}
	hhc, err := d.fsys.ReadFile(clean)
	if err != nil {
		d.log.Debug("chm: table of contents left out", "name", clean, "err", err)
		return nil
	}
	return contentsOrder(hhc, path.Dir(clean), d.charset, d.lim)
}

// source returns the help file as a source for the layout: its files, the
// documents in the order of reading, the style sheets and the title. The
// style sheets are a page of US Letter and the body of ebook.BodyStyle, as
// calibre draws it. It returns an error that wraps ivy.ErrFormat if the
// help file holds no HTML document.
func (d *Document) source() (src layout.Source, err error) {
	defer ebook.Recover("reading chm", &err)
	var docs []string
	seen := make(map[string]bool)
	add := func(name string) {
		clean, ok := d.lookup(name)
		if !ok || seen[clean] || !isHTML(clean) || len(docs) >= d.lim.MaxObjects {
			return
		}
		seen[clean] = true
		docs = append(docs, clean)
	}
	if topic, ok := d.topic(); ok {
		add(topic)
	}
	for _, name := range d.contents() {
		add(name)
	}
	for _, name := range d.archive.files() {
		if clean, ok := zipdoc.CleanName(name); ok {
			add(clean)
		}
	}
	if len(docs) == 0 {
		return layout.Source{}, fmt.Errorf("chm without an html document: %w", ivy.ErrFormat)
	}
	return layout.Source{
		FS:          d.fsys,
		Documents:   docs,
		StyleSheets: []string{ebook.PaperStyleFor(d.paper), ebook.BodyStyle},
		Title:       d.title(),
	}, nil
}
