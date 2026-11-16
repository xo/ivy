// Package html opens HTML files and HTMLZ files.
//
// An HTMLZ file is a zip archive that calibre writes: index.html, with its
// images and style sheets, metadata.opf and a cover image. The cover image
// is the preview, and metadata.opf gives the metadata. An HTML file has no
// preview, and its head gives the metadata.
//
// The layout of phase 4 puts the document on pages (D10, D15): a page of US
// Letter for an HTML file, and the page of an ebook, 6 by 9 inches, for an
// HTMLZ file. Page lays out the document only as far as the page that a
// program asks for. An HTML file reads the files that it links to from the
// folder of WithDir (D22).
//
// The package html of ivy has the same name as the package html of okage
// and of the standard library. A program that imports two of them renames
// one.
package html

import (
	"context"
	"errors"
	"fmt"
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
	Name:  "html",
	Mimes: []string{"text/html", "application/xhtml+xml", "application/htmlz+zip"},
	Exts:  []string{"html", "htm", "xhtml", "xht", "htmlz"},
	Open: func(ctx context.Context, src ivy.Source, cfg ivy.Config) (ivy.Document, error) {
		return Open(ctx, src, WithConfig(cfg))
	},
}

// The names that an HTMLZ file gives its parts.
const (
	indexName    = "index.html"
	metadataName = "metadata.opf"
)

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

// WithDir gives the folder of an HTML file. The layout reads the style
// sheets, the images and the fonts that the file links to from it. The file
// is at the root of the folder, so a link out of the folder is not read, and
// a file of the folder with the name index.html or index.xhtml is hidden.
// An HTMLZ file holds its own files, and it ignores the folder. Without a
// folder, the layout reads no file that an HTML file links to (D22).
func WithDir(dir fs.FS) Option {
	return func(o *options) {
		o.dir = dir
	}
}

// Document is an open HTML or HTMLZ file.
type Document struct {
	lim ivy.Limits
	log *slog.Logger
	// fonts are the fonts of the configuration, for the layout.
	fonts ebook.Fonts
	// fsys holds the document and its resources. For an HTML file, it holds
	// the document over the folder of WithDir, or only the document, because
	// an ivy.Source has no folder.
	fsys fs.FS
	// index is the path of the document in fsys.
	index string
	// archive is the archive of an HTMLZ file, or nil.
	archive *zipdoc.Archive
	// pkg is the metadata.opf of an HTMLZ file, or nil.
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

// Open opens an HTML file or an HTMLZ file. A source that starts as a zip
// archive is an HTMLZ file, and any other source is an HTML file. It returns
// an error that wraps ivy.ErrFormat if an HTMLZ file holds no HTML document,
// and one that wraps ivy.ErrLimit if an HTML file is larger than
// Limits.MaxMemory.
func Open(ctx context.Context, src ivy.Source, opts ...Option) (d *Document, err error) {
	var o options
	for _, opt := range opts {
		opt(&o)
	}
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("opening html: %w", err)
	}
	defer ebook.Recover("opening html", &err)
	d = &Document{lim: o.cfg.Limits.WithDefaults(), log: o.cfg.Log(), fonts: ebook.FontsOf(o.cfg)}
	var magic [4]byte
	if n, _ := src.ReadAt(magic[:], 0); n == len(magic) && string(magic[:]) == "PK\x03\x04" {
		err = d.openHTMLZ(src)
	} else {
		err = d.openHTML(src, o.dir)
	}
	if err != nil {
		return nil, err
	}
	return d, nil
}

// openHTML reads an HTML file into memory.
func (d *Document) openHTML(src ivy.Source, dir fs.FS) error {
	if size := src.Size(); size > d.lim.MaxMemory {
		return fmt.Errorf("html of %d bytes: %w", size, ivy.ErrLimit)
	}
	data, err := io.ReadAll(io.NewSectionReader(src, 0, src.Size()))
	if err != nil {
		return fmt.Errorf("reading html: %w", err)
	}
	d.index = "index.html"
	// a document that starts with an XML declaration is XHTML, which the
	// layout reads by the extension of its name
	if strings.HasPrefix(strings.TrimLeft(ebook.Text(data[:min(len(data), 1024)]), " \t\r\n"), "<?xml") {
		d.index = "index.xhtml"
	}
	var fsys memfs.FS
	fsys.Add(d.index, data)
	d.fsys = &fsys
	if dir != nil {
		d.fsys = fsys.Over(dir)
	}
	return nil
}

// openHTMLZ opens an HTMLZ file. It finds the document and reads
// metadata.opf.
func (d *Document) openHTMLZ(src ivy.Source) error {
	archive, err := zipdoc.Open(src, d.lim)
	if err != nil {
		return err
	}
	fsys := archive.FS()
	d.archive, d.fsys = archive, fsys
	d.index = indexName
	if !fsys.Has(indexName) {
		var ok bool
		if d.index, ok = firstDocument(archive); !ok {
			return fmt.Errorf("htmlz without an html document: %w", ivy.ErrFormat)
		}
	}
	if f := archive.Find(metadataName); f != nil {
		buf, err := archive.Read(f)
		if err == nil {
			d.pkg, err = ebook.ParseOPF(buf, ".", d.lim)
		}
		if err != nil {
			d.log.Debug("htmlz: metadata left out", "err", err)
		}
	}
	return nil
}

// firstDocument returns the HTML document of the archive that is nearest
// to the root, and first in the order of the names among those.
func firstDocument(archive *zipdoc.Archive) (string, bool) {
	var best string
	for _, f := range archive.Files() {
		name, ok := zipdoc.CleanName(f.Name)
		if !ok || !isDocument(name) {
			continue
		}
		if best == "" || depth(name) < depth(best) || depth(name) == depth(best) && name < best {
			best = name
		}
	}
	return best, best != ""
}

// isDocument reports whether the extension of name is HTML or XHTML.
func isDocument(name string) bool {
	switch strings.ToLower(path.Ext(name)) {
	case ".html", ".htm", ".xhtml", ".xht":
		return true
	}
	return false
}

// depth returns the number of folders above name.
func depth(name string) int {
	return strings.Count(name, "/")
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
	return d.source()
}

// config returns the configuration of the layout.
func (d *Document) config() ivy.Config {
	return d.fonts.Config(d.lim, d.log)
}

// Previews satisfies the ivy.Previewer interface. It lists the cover image
// of an HTMLZ file: the cover that metadata.opf names, or a file named
// cover.jpg or cover.png in the root. An HTML file has no preview.
func (d *Document) Previews(ctx context.Context) iter.Seq2[ivy.Preview, error] {
	if d.archive == nil {
		return func(func(ivy.Preview, error) bool) {}
	}
	return ebook.CoverPreviews(ctx, d.archive, d.pkg)
}

// Describe satisfies the ivy.Describer interface. For an HTMLZ file, it
// reads metadata.opf. If that gives no title, or for an HTML file, it reads
// the head of the document: its title, its language, and the meta elements
// of the author, the subject and the program that made it.
func (d *Document) Describe(ctx context.Context) (info ivy.Info, err error) {
	if err := ctx.Err(); err != nil {
		return ivy.Info{}, fmt.Errorf("describing html: %w", err)
	}
	defer ebook.Recover("describing html", &err)
	if d.pkg != nil {
		info = d.pkg.Info()
		if info.Title != "" {
			return info, nil
		}
	}
	head, err := d.head()
	if err != nil {
		return info, err
	}
	if d.pkg != nil {
		info.Title = head.Title
		return info, nil
	}
	return ivy.Info{
		Title:    head.Title,
		Authors:  head.Authors,
		Subject:  head.Subject,
		Language: head.Language,
		Producer: head.Generator,
	}, nil
}

// head scans the head of the document.
func (d *Document) head() (ebook.Head, error) {
	buf, err := fs.ReadFile(d.fsys, d.index)
	switch {
	case err == nil:
	case errors.Is(err, ivy.ErrLimit), errors.Is(err, ivy.ErrFormat):
		return ebook.Head{}, fmt.Errorf("reading %s: %w", d.index, err)
	default:
		// An entry of the archive that cannot be read, such as one with a
		// bad checksum, is a malformed file.
		return ebook.Head{}, fmt.Errorf("reading %s: %w: %w", d.index, ivy.ErrFormat, err)
	}
	return ebook.ScanHead(ebook.Text(buf)), nil
}

// source returns the file as a source for the layout: the document and its
// resources, and a style sheet. An HTMLZ file is an ebook, and takes the
// page of an ebook. An HTML file takes a page of US Letter.
func (d *Document) source() (src layout.Source, err error) {
	defer ebook.Recover("reading html", &err)
	info, err := d.Describe(context.Background())
	if err != nil {
		return layout.Source{}, err
	}
	style := ebook.PaperStyle
	if d.archive != nil {
		style = ebook.BookStyle
	}
	return layout.Source{
		FS:          d.fsys,
		Documents:   []string{d.index},
		StyleSheets: []string{style},
		Title:       info.Title,
	}, nil
}
