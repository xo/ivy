// Package epub opens EPUB 2 and EPUB 3 files, and the kepub files of Kobo,
// which are EPUB files.
//
// An EPUB file is a zip archive. Its container names the package document.
// The spine of the package lists the documents of XHTML in the order of
// reading, and its metadata gives the title, the authors and the cover. Open
// reads the package. The cover image is the preview.
//
// The layout of phase 4 puts the documents of the spine on pages, one after
// the other, with the style sheet of an ebook: a page of 6 by 9 inches
// (D10, D15). Page lays out the book only as far as the page that a program
// asks for. As in calibre, the first page shows the cover image, scaled to
// fit the page. An image of the spine, such as an image of SVG, is on a page
// of its own.
//
// A book of fixed layout, whose rendition:layout is pre-paginated, gives the
// size of each page in a meta element viewport of its document. Each such
// document is one page of that size, with no margin, and an image of its
// spine is a page of the size of the image. The book starts with its own
// first page, and the package adds no page of the cover.
//
// The package undoes the obfuscation of fonts of the IDPF and of Adobe,
// because it is not DRM (D12). A book whose documents DRM encrypts, such as
// a book of Adobe ADEPT or of Readium LCP, still opens for its cover and its
// metadata, where they are in the clear. Its documents give an error that
// wraps ivy.ErrDRM.
package epub

import (
	"context"
	"fmt"
	"image"
	"io/fs"
	"iter"
	"log/slog"
	"path"
	"strings"

	_ "golang.org/x/image/webp" // the WebP decoder, for a cover of EPUB 3.3

	"github.com/xo/ivy"
	"github.com/xo/ivy/internal/ebook"
	"github.com/xo/ivy/internal/layout"
	"github.com/xo/ivy/internal/memfs"
	"github.com/xo/ivy/internal/zipdoc"
)

// Format describes the package for the registry of a program.
var Format = ivy.Format{
	Name:  "epub",
	Mimes: []string{"application/epub+zip"},
	Exts:  []string{"epub", "kepub"},
	Open: func(ctx context.Context, src ivy.Source, cfg ivy.Config) (ivy.Document, error) {
		return Open(ctx, src, WithConfig(cfg))
	},
}

// maxCoverPages is the number of pages that the search for a cover reads to
// find the first image of a page. A crafted file can name many pages.
const maxCoverPages = 4

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

// Document is an open EPUB file.
type Document struct {
	archive *zipdoc.Archive
	lim     ivy.Limits
	log     *slog.Logger
	// fonts are the fonts of the configuration, for the layout.
	fonts ebook.Fonts
	// paper is the paper of the configuration, or zero (D32).
	paper ivy.Paper
	// opf is the path of the package document.
	opf string
	pkg *ebook.Package
	// fsys holds the files of the archive, with the fonts deobfuscated.
	fsys *zipdoc.FS
	// drm holds the paths of the files that DRM encrypts.
	drm map[string]string
	// pages holds the book of the layout.
	pages ebook.Pages
}

// The capabilities of a document.
var (
	_ ivy.Pager     = (*Document)(nil)
	_ ivy.Previewer = (*Document)(nil)
	_ ivy.Describer = (*Document)(nil)
)

// Open opens an EPUB file. It reads the container, the package document and
// the list of encrypted files. It returns an error that wraps ivy.ErrFormat
// if src is not an EPUB file.
func Open(ctx context.Context, src ivy.Source, opts ...Option) (d *Document, err error) {
	var o options
	for _, opt := range opts {
		opt(&o)
	}
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("opening epub: %w", err)
	}
	defer ebook.Recover("opening epub", &err)
	return open(src, o.cfg)
}

// open opens an EPUB file.
func open(src ivy.Source, cfg ivy.Config) (*Document, error) {
	lim := cfg.Limits.WithDefaults()
	archive, err := zipdoc.Open(src, lim)
	if err != nil {
		return nil, err
	}
	name, err := rootfile(archive, lim)
	if err != nil {
		return nil, err
	}
	f := archive.Find(name)
	if f == nil {
		return nil, fmt.Errorf("epub without its package document %s: %w", name, ivy.ErrFormat)
	}
	buf, err := archive.Read(f)
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", name, err)
	}
	pkg, err := ebook.ParseOPF(buf, path.Dir(name), lim)
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", name, err)
	}
	encryption, err := readEncryption(archive, lim)
	if err != nil {
		return nil, err
	}
	d := &Document{
		archive: archive,
		lim:     lim,
		log:     cfg.Log(),
		fonts:   ebook.FontsOf(cfg),
		paper:   cfg.Paper,
		opf:     name,
		pkg:     pkg,
		fsys:    archive.FS(),
		drm:     make(map[string]string),
	}
	for name, algorithm := range encryption {
		if isObfuscation(algorithm) {
			deobfuscate, err := deobfuscator(algorithm, pkg.Metadata)
			if err != nil {
				deobfuscate = func([]byte) ([]byte, error) { return nil, fmt.Errorf("font %s: %w", name, err) }
			}
			d.fsys.Transform(name, deobfuscate)
			continue
		}
		d.drm[name] = algorithm
		d.fsys.Transform(name, func([]byte) ([]byte, error) {
			return nil, fmt.Errorf("%s encrypted with %s: %w", name, algorithm, ivy.ErrDRM)
		})
	}
	return d, nil
}

// Close satisfies the ivy.Document interface.
func (d *Document) Close() error {
	return nil
}

// PageCount satisfies the ivy.Pager interface. It lays out the whole book on
// its first call. It returns an error that wraps ivy.ErrDRM if DRM encrypts
// a document.
func (d *Document) PageCount(ctx context.Context) (int, error) {
	return d.pages.PageCount(ctx, d.layoutSource, d.config())
}

// Page satisfies the ivy.Pager interface. It lays out the book only as far
// as page n.
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

// Previews satisfies the ivy.Previewer interface. It lists the cover image,
// where the book has one in the clear.
func (d *Document) Previews(ctx context.Context) iter.Seq2[ivy.Preview, error] {
	return func(yield func(ivy.Preview, error) bool) {
		preview, ok, err := d.coverPreview(ctx)
		switch {
		case err != nil:
			yield(ivy.Preview{}, err)
		case ok:
			yield(preview, nil)
		}
	}
}

// coverPreview returns the preview of the cover image.
func (d *Document) coverPreview(ctx context.Context) (preview ivy.Preview, ok bool, err error) {
	if err := ctx.Err(); err != nil {
		return ivy.Preview{}, false, fmt.Errorf("reading the cover of epub: %w", err)
	}
	defer ebook.Recover("reading the cover of epub", &err)
	name, ok := d.cover()
	if !ok {
		return ivy.Preview{}, false, nil
	}
	f := d.archive.Find(name)
	if f == nil {
		return ivy.Preview{}, false, nil
	}
	preview = d.archive.Preview(ivy.Cover, f)
	decode := preview.Decode
	preview.Decode = func(ctx context.Context) (img image.Image, err error) {
		defer ebook.Recover("decoding the cover of epub", &err)
		return decode(ctx)
	}
	return preview, true, nil
}

// cover returns the path of the cover image. It tries, in order: the item
// of EPUB 3 with the property cover-image, the meta element of EPUB 2 whose
// name is cover, the cover of the guide, an image item whose id is cover,
// and the first image of a page whose id names a cover. An image that DRM
// encrypts is left out.
func (d *Document) cover() (string, bool) {
	p := d.pkg
	usable := func(name string) bool {
		if _, encrypted := d.drm[name]; encrypted {
			return false
		}
		if it, ok := p.ItemByPath(name); ok && !it.IsImage() {
			return false
		}
		return ebook.IsImagePath(name) || d.isItemImage(name)
	}
	for _, it := range p.Manifest {
		if it.HasProperty("cover-image") && usable(it.Path) {
			return it.Path, true
		}
	}
	if c := p.Metadata.Cover; c != "" {
		if it, ok := p.Item(c); ok && usable(it.Path) {
			return it.Path, true
		}
		// some files give the path of the image, not its id
		if name, ok := ebook.Resolve(path.Dir(d.opf), c); ok && usable(name) && d.exists(name) {
			return name, true
		}
	}
	pages := 0
	pageImage := func(page string) (string, bool) {
		if pages >= maxCoverPages {
			return "", false
		}
		pages++
		name, ok := d.pageImage(page)
		return name, ok && usable(name)
	}
	for _, ref := range p.Guide {
		switch ref.Type {
		case "cover", "other.ms-coverimage-standard", "other.ms-coverimage":
			if usable(ref.Path) && d.exists(ref.Path) {
				return ref.Path, true
			}
			if name, ok := pageImage(ref.Path); ok {
				return name, true
			}
		}
	}
	for _, it := range p.Manifest {
		switch strings.ToLower(it.ID) {
		case "cover", "cover-image", "coverimage", "cover_image":
			if usable(it.Path) {
				return it.Path, true
			}
		}
	}
	for _, it := range p.Manifest {
		if it.IsDocument() && strings.Contains(strings.ToLower(it.ID), "cover") {
			if name, ok := pageImage(it.Path); ok {
				return name, true
			}
		}
	}
	return "", false
}

// isItemImage reports whether the manifest lists name as a raster image.
func (d *Document) isItemImage(name string) bool {
	it, ok := d.pkg.ItemByPath(name)
	return ok && it.IsImage()
}

// exists reports whether the archive holds the file name.
func (d *Document) exists(name string) bool {
	info, err := d.fsys.Stat(name)
	return err == nil && !info.IsDir()
}

// pageImage returns the path of the first image of the page name.
func (d *Document) pageImage(name string) (string, bool) {
	buf, err := d.fsys.ReadFile(name)
	if err != nil {
		return "", false
	}
	src, ok := ebook.FirstImage(ebook.Text(buf), d.lim)
	if !ok {
		return "", false
	}
	img, ok := ebook.Resolve(path.Dir(name), src)
	if !ok || !d.exists(img) {
		return "", false
	}
	return img, true
}

// Describe satisfies the ivy.Describer interface. It reads the metadata of
// the package: the title, the authors, the subjects, the language, the date
// of the publication, the date of the last change, the program that made the
// file, the publisher and the identifier. The identifier is the one that the
// package names as its unique identifier, or else the first one. If the
// package has no title, it reads the title of the NCX file.
func (d *Document) Describe(ctx context.Context) (info ivy.Info, err error) {
	if err := ctx.Err(); err != nil {
		return ivy.Info{}, fmt.Errorf("describing epub: %w", err)
	}
	defer ebook.Recover("describing epub", &err)
	info = d.pkg.Info()
	if info.Title == "" {
		info.Title = d.ncxTitle()
	}
	return info, nil
}

// ncxTitle returns the title of the NCX file of EPUB 2, or the empty
// string.
func (d *Document) ncxTitle() string {
	it, ok := d.pkg.Item(d.pkg.Toc)
	if !ok {
		for _, candidate := range d.pkg.Manifest {
			if candidate.MediaType == "application/x-dtbncx+xml" {
				it, ok = candidate, true
				break
			}
		}
	}
	if !ok {
		return ""
	}
	buf, err := d.fsys.ReadFile(it.Path)
	if err != nil {
		return ""
	}
	doc, err := ebook.ParseXML(buf, d.lim)
	if err != nil {
		return ""
	}
	for n := range doc.RootElement().Descendants() {
		if n.IsElement() && ebook.Local(n) == "docTitle" {
			return ebook.NodeText(n)
		}
	}
	return ""
}

// source returns the book as a source for the layout: the files of the
// archive, the documents of the spine, the style sheet of an ebook and the
// title. A book with a cover image starts with a page of the cover. An image
// of the spine, such as an image of SVG, is on a page of its own. source
// returns an error that wraps ivy.ErrDRM if DRM encrypts a document, and
// ivy.ErrFormat if the book has no document.
func (d *Document) source() (src layout.Source, err error) {
	defer ebook.Recover("reading epub", &err)
	docs := d.documents()
	if len(docs) == 0 {
		return layout.Source{}, fmt.Errorf("epub without a document: %w", ivy.ErrFormat)
	}
	for _, name := range docs {
		if algorithm, ok := d.drm[name]; ok {
			return layout.Source{}, fmt.Errorf("document %s encrypted with %s: %w", name, algorithm, ivy.ErrDRM)
		}
	}
	title := d.pkg.Metadata.Title
	if title == "" {
		title = d.ncxTitle()
	}
	fixed := newFixedPages(d)
	p := &imagePages{d: d, title: title}
	if !fixed.isFixed(docs[0]) {
		// a book of fixed layout starts with its own page of the cover
		docs = p.withCover(docs)
	}
	// replaced maps an image of the spine to the page that shows it, for
	// the table of contents
	replaced := make(map[string]string)
	for i, name := range docs {
		switch {
		case !fixed.isFixed(name):
		case d.isSpineImage(name):
			page := ebook.ImagePageName(name, p.exists)
			if doc, ok := fixed.image(page, name, title); ok {
				p.fs.Add(page, doc)
				p.n++
				docs[i] = page
				replaced[name] = page
				continue
			}
		default:
			if doc, ok := fixed.document(name); ok {
				p.fs.Add(name, doc)
				p.n++
			}
			continue
		}
		if d.isSpineImage(name) {
			docs[i] = p.add(name)
			replaced[name] = docs[i]
		}
	}
	var fsys fs.FS = d.fsys
	if p.n > 0 {
		fsys = p.fs.Over(d.fsys)
	}
	styles := []string{ebook.BookStyleFor(d.paper)}
	if style := fixed.style(); style != "" {
		styles = append(styles, style)
	}
	return layout.Source{
		FS:          fsys,
		Documents:   docs,
		StyleSheets: styles,
		Title:       title,
		TOC:         d.toc(replaced),
	}, nil
}

// imagePages makes the pages of XHTML that show an image of the book: the
// page of the cover, and the page of each image of the spine.
type imagePages struct {
	d     *Document
	title string
	fs    memfs.FS
	// n counts the pages.
	n int
}

// exists reports whether the pages or the archive hold the file name.
func (p *imagePages) exists(name string) bool {
	_, err := p.fs.Stat(name)
	return err == nil || p.d.exists(name)
}

// add adds the page of the image name, and returns its path.
func (p *imagePages) add(name string) string {
	page := ebook.ImagePageName(name, p.exists)
	p.fs.Add(page, ebook.ImagePage(ebook.ImagePageLink(page, name), p.title, p.d.paper))
	p.n++
	return page
}

// withCover returns docs with a page of the cover image first, as calibre
// shows a book. A first document that is already a page of the cover, with
// almost no text, stays after the page of the cover, as in calibre. A first
// document that shows the cover with its text, or that the guide names as
// the cover, stays the first page. A book without a cover image is
// unchanged.
func (p *imagePages) withCover(docs []string) []string {
	d := p.d
	cover, ok := d.cover()
	if !ok || !d.exists(cover) {
		return docs
	}
	first := docs[0]
	if d.isSpineImage(first) {
		if first == cover || d.isCoverItem(first) {
			return docs
		}
		return append([]string{cover}, docs...)
	}
	page, shows := ebook.IsCoverPage(d.fsys, first, cover, d.lim)
	if page {
		return append([]string{cover}, docs...)
	}
	if shows {
		return docs
	}
	for _, ref := range d.pkg.Guide {
		if ref.Type == "cover" && ref.Path == first {
			return docs
		}
	}
	return append([]string{cover}, docs...)
}

// isCoverItem reports whether the manifest gives name the property
// cover-image of EPUB 3.
func (d *Document) isCoverItem(name string) bool {
	it, ok := d.pkg.ItemByPath(name)
	return ok && it.HasProperty("cover-image")
}

// isSpineImage reports whether name is an image, of SVG or raster, and not
// a document.
func (d *Document) isSpineImage(name string) bool {
	if it, ok := d.pkg.ItemByPath(name); ok {
		if it.IsDocument() {
			return false
		}
		if isImageItem(it) {
			return true
		}
	}
	return ebook.IsImagePath(name) || strings.EqualFold(path.Ext(name), ".svg")
}

// isImageItem reports whether the item is an image of SVG or a raster
// image.
func isImageItem(it ebook.Item) bool {
	return it.MediaType == "image/svg+xml" || it.IsImage() || strings.EqualFold(path.Ext(it.Path), ".svg")
}

// documents returns the paths of the documents of the spine: the linear
// items first, then the items that are not linear, such as notes. An item
// that is not XHTML is replaced with its fallback. An image without a
// fallback stays, so that source puts it on a page of its own. Another
// item is left out. If the spine gives no document, documents falls back on
// the documents of the manifest, without the navigation document of EPUB 3.
func (d *Document) documents() []string {
	var linear, other []string
	seen := make(map[string]bool)
	memo := make(map[string]resolved)
	for _, ref := range d.pkg.Spine {
		it, ok := d.pkg.Item(ref.IDRef)
		if !ok {
			d.log.Debug("epub: spine item without a manifest item", "idref", ref.IDRef)
			continue
		}
		spine := it
		it, ok = d.document(it, memo)
		if !ok && isImageItem(spine) {
			it, ok = spine, true
		}
		if !ok || seen[it.Path] || !d.exists(it.Path) {
			d.log.Debug("epub: spine item left out", "idref", ref.IDRef, "path", it.Path)
			continue
		}
		seen[it.Path] = true
		if ref.Linear {
			linear = append(linear, it.Path)
		} else {
			other = append(other, it.Path)
		}
	}
	linear = append(linear, other...)
	if len(linear) > 0 {
		return linear
	}
	var docs []string
	for _, it := range d.pkg.Manifest {
		if it.IsDocument() && !it.HasProperty("nav") && !seen[it.Path] && d.exists(it.Path) {
			seen[it.Path] = true
			docs = append(docs, it.Path)
		}
	}
	return docs
}

// resolved is the end of a chain of fallbacks.
type resolved struct {
	it ebook.Item
	ok bool
}

// document follows the chain of fallbacks from it to an item of XHTML or of
// HTML. A chain that loops ends. memo keeps the end of each chain, so that
// the chains of a crafted file take linear time.
func (d *Document) document(it ebook.Item, memo map[string]resolved) (ebook.Item, bool) {
	var chain []string
	end := func(r resolved) (ebook.Item, bool) {
		for _, id := range chain {
			memo[id] = r
		}
		return r.it, r.ok
	}
	visited := make(map[string]bool)
	for !it.IsDocument() {
		if r, ok := memo[it.ID]; ok {
			return end(r)
		}
		if visited[it.ID] || it.Fallback == "" {
			return end(resolved{it, false})
		}
		visited[it.ID] = true
		chain = append(chain, it.ID)
		next, ok := d.pkg.Item(it.Fallback)
		if !ok {
			return end(resolved{it, false})
		}
		it = next
	}
	return end(resolved{it, true})
}
