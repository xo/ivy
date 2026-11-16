// Package mobi opens the ebooks of Mobipocket and of the Kindle: MOBI and
// PRC files, AZW files, which are MOBI files with the extensions of the
// Kindle, AZW3 files, also called KF8, and AZW4 files, also called Print
// Replica. It also opens a PalmDOC book, which is a PRC file of plain text.
//
// Each of them is a PalmDB file: a list of records. Record 0 holds the
// headers and the EXTH records, which give the metadata and the cover. The
// text records follow, compressed with PalmDoc or with HUFF/CDIC, and then
// the images and the other resources. The package follows the MOBI page of
// the MobileRead wiki, and the decoders of github.com/xo/magic/compress.
//
// The text of a MOBI book is one document of old HTML. The package converts
// it into one HTML document: each recindex attribute of an image becomes a
// link to the image, each filepos link becomes a link to an anchor, and each
// mbp:pagebreak element becomes a page break. The text of a KF8 book is a
// flow of skeletons and fragments, and the package puts them together into
// one document of XHTML for each part, with the style sheets and the images
// of SVG of the other flows. A joint file holds a MOBI book and a KF8 book,
// and the package reads the KF8 book.
//
// The cover image and the thumbnail are the previews. A book whose text DRM
// encrypts still opens for its metadata and its previews, which DRM does not
// encrypt. Its text gives an error that wraps ivy.ErrDRM (D12).
//
// The layout puts the documents on the pages of an ebook (D10, D21), and Page
// lays them out only as far as the page that a program asks for. As in
// calibre, the first page shows the cover image, scaled to fit the page. An AZW4
// book holds a PDF file in its text, and its pages are the pages of that
// PDF file, which the package pdf renders.
package mobi

import (
	"context"
	"fmt"
	"image"
	"iter"
	"log/slog"
	"strings"
	"sync"

	"github.com/xo/magic/compress/palmdoc"

	"github.com/xo/ivy"
	"github.com/xo/ivy/internal/ebook"
	"github.com/xo/ivy/internal/layout"
	"github.com/xo/ivy/internal/raster"
	"github.com/xo/ivy/internal/zipdoc"
)

// Format describes the package for the registry of a program.
var Format = ivy.Format{
	Name: "mobi",
	Mimes: []string{
		"application/x-mobipocket-ebook",
		"application/vnd.amazon.ebook",
		"application/vnd.amazon.mobi8-ebook",
		"application/x-mobi8-ebook",
	},
	Exts: []string{"mobi", "prc", "azw", "azw3", "azw4"},
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

// kind is the kind of a book.
type kind int

// The kinds of books.
const (
	// palmDOC is a book of plain text.
	palmDOC kind = iota
	// mobi6 is a MOBI book, with a text of HTML.
	mobi6
	// kf8 is a KF8 book, alone or in a joint file.
	kf8
	// replica is a Print Replica book, which holds a PDF file.
	replica
)

// String returns the name of the kind.
func (k kind) String() string {
	switch k {
	case palmDOC:
		return "palmdoc"
	case mobi6:
		return "mobi"
	case kf8:
		return "kf8"
	}
	return "print replica"
}

// replicaMagic starts the text of a Print Replica book.
const replicaMagic = "%MOP"

// Document is an open MOBI, AZW, AZW3, AZW4 or PalmDOC file.
type Document struct {
	db  *database
	cfg ivy.Config
	lim ivy.Limits
	log *slog.Logger
	// fonts are the fonts of the configuration, for the layout.
	fonts ebook.Fonts
	// first is the header of record 0. book is the header of the book that
	// the package reads: the KF8 header of a joint file, or first.
	first, book *header
	kind        kind
	// firstResource is the record of resource 1. The resources of a joint
	// file are in its MOBI part, so they count from the first image of
	// record 0.
	firstResource int

	mu     sync.Mutex
	src    *layout.Source
	srcErr error
	// pages holds the book of the layout.
	pages ebook.Pages
	// replica holds the PDF file of a Print Replica book.
	replica replicaPDF
}

// The capabilities of a document.
var (
	_ ivy.Pager     = (*Document)(nil)
	_ ivy.Previewer = (*Document)(nil)
	_ ivy.Describer = (*Document)(nil)
)

// Open opens a MOBI, AZW, AZW3, AZW4 or PalmDOC file. It reads the headers
// and the EXTH records. It returns an error that wraps ivy.ErrFormat if src
// is not such a file.
func Open(ctx context.Context, src ivy.Source, opts ...Option) (d *Document, err error) {
	var o options
	for _, opt := range opts {
		opt(&o)
	}
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("opening mobi: %w", err)
	}
	defer ebook.Recover("opening mobi", &err)
	lim := o.cfg.Limits.WithDefaults()
	db, err := readDatabase(src, lim)
	if err != nil {
		return nil, err
	}
	first, err := readHeader(db, 0, lim)
	if err != nil {
		return nil, err
	}
	d = &Document{db: db, cfg: o.cfg, lim: lim, log: o.cfg.Log(), fonts: ebook.FontsOf(o.cfg), first: first, book: first, firstResource: first.firstImage}
	switch {
	case !first.mobi:
		d.kind = palmDOC
	case first.version >= 8 && first.version != none:
		d.kind = kf8
	default:
		d.kind = mobi6
		if boundary, ok := first.exthInt(exthBoundary); ok && boundary > 0 && boundary < db.count() {
			if h, err := readHeader(db, boundary, lim); err == nil && h.mobi && h.version >= 8 && h.version != none {
				d.book, d.kind = h, kf8
			} else {
				d.log.Debug("mobi: kf8 part left out", "record", boundary, "err", err)
			}
		}
	}
	if d.kind == mobi6 && d.book.encryption == 0 && d.book.textRecords > 0 {
		if rec, err := db.record(1); err == nil && d.isReplica(rec) {
			d.kind = replica
		}
	}
	return d, nil
}

// isReplica reports whether the first text record of a MOBI book starts a
// Print Replica book.
func (d *Document) isReplica(rec []byte) bool {
	rec = trimTrailing(rec, d.book.extraFlags)
	switch d.book.compression {
	case compressionNone:
		return len(rec) >= len(replicaMagic) && string(rec[:len(replicaMagic)]) == replicaMagic
	case compressionPalmDoc:
		text, _ := palmdoc.Decode(nil, rec, len(replicaMagic))
		return string(text) == replicaMagic
	}
	return false
}

// Close satisfies the ivy.Document interface.
func (d *Document) Close() error {
	return d.replica.close()
}

// PageCount satisfies the ivy.Pager interface. It lays out the whole book
// on its first call. It returns an error that wraps ivy.ErrDRM if DRM
// encrypts the text.
func (d *Document) PageCount(ctx context.Context) (int, error) {
	if d.kind == replica {
		doc, err := d.replica.open(ctx, d)
		if err != nil {
			return 0, err
		}
		return doc.PageCount(ctx)
	}
	return d.pages.PageCount(ctx, d.layoutSource, d.config())
}

// Page satisfies the ivy.Pager interface. It lays out the book only as far
// as page n.
func (d *Document) Page(ctx context.Context, n int) (ivy.Page, error) {
	if d.kind == replica {
		doc, err := d.replica.open(ctx, d)
		if err != nil {
			return nil, err
		}
		return doc.Page(ctx, n)
	}
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

// Describe satisfies the ivy.Describer interface. It reads the EXTH
// records: the title, the authors, the subjects, the language, the date of
// the publication, the date of the last change and the program that made
// the book. A book without a title in its EXTH records takes its full name,
// or the name of its database. A book without a language takes the language
// of its locale. ivy.Info holds no field for the publisher.
func (d *Document) Describe(ctx context.Context) (info ivy.Info, err error) {
	if err := ctx.Err(); err != nil {
		return ivy.Info{}, fmt.Errorf("describing mobi: %w", err)
	}
	defer ebook.Recover("describing mobi", &err)
	return d.book.info(d.db), nil
}

// Previews satisfies the ivy.Previewer interface. It lists the cover image
// of EXTH record 201 and the thumbnail of EXTH record 202, which count from
// the first image of the book. A KF8 book without record 201 can give its
// cover as a kindle:embed link in EXTH record 129.
func (d *Document) Previews(ctx context.Context) iter.Seq2[ivy.Preview, error] {
	return func(yield func(ivy.Preview, error) bool) {
		if err := ctx.Err(); err != nil {
			yield(ivy.Preview{}, fmt.Errorf("listing previews: %w", err))
			return
		}
		seen := make(map[int]bool)
		for _, p := range d.previewRecords() {
			if seen[p.record] {
				continue
			}
			seen[p.record] = true
			preview, err := d.preview(p.kind, p.record)
			if err != nil {
				d.log.Debug("mobi: preview left out", "record", p.record, "err", err)
				continue
			}
			if !yield(preview, nil) {
				return
			}
		}
	}
}

// previewRecord is the record of a preview.
type previewRecord struct {
	kind   ivy.PreviewKind
	record int
}

// previewRecords returns the records of the cover and of the thumbnail.
func (d *Document) previewRecords() []previewRecord {
	if d.firstResource < 0 {
		return nil
	}
	var out []previewRecord
	for _, h := range []*header{d.book, d.first} {
		if n, ok := h.exthInt(exthCover); ok {
			out = append(out, previewRecord{ivy.Cover, d.firstResource + n})
		} else if uri, ok := h.exthData(exthCoverURI); ok {
			if m := kindleLink.FindStringSubmatch(string(uri)); m != nil && m[1] == "embed" {
				if n := base32(m[2]); n > 0 {
					out = append(out, previewRecord{ivy.Cover, d.firstResource + n - 1})
				}
			}
		}
		if n, ok := h.exthInt(exthThumbnail); ok {
			out = append(out, previewRecord{ivy.Thumbnail, d.firstResource + n})
		}
		if len(out) > 0 {
			break
		}
	}
	return out
}

// preview returns the preview of the image in record rec.
func (d *Document) preview(kind ivy.PreviewKind, rec int) (ivy.Preview, error) {
	if rec < 0 || rec >= d.db.count() {
		return ivy.Preview{}, fmt.Errorf("preview in record %d of %d: %w", rec, d.db.count(), ivy.ErrFormat)
	}
	buf, err := d.resource(rec)
	if err != nil {
		return ivy.Preview{}, err
	}
	size, err := raster.Config(buf)
	if err != nil {
		return ivy.Preview{}, fmt.Errorf("preview in record %d: %w", rec, err)
	}
	if err := raster.CheckPixels(size, d.lim); err != nil {
		return ivy.Preview{}, err
	}
	return ivy.Preview{
		Kind: kind,
		Size: size,
		Decode: func(ctx context.Context) (img image.Image, err error) {
			defer ebook.Recover("decoding the preview of mobi", &err)
			return raster.Decode(ctx, buf, d.lim)
		},
	}, nil
}

// source converts the book into HTML for the layout. It converts the book
// once, and returns the same result on each later call. It returns an error
// that wraps ivy.ErrDRM if DRM encrypts the text, and one that wraps
// ivy.ErrUnsupported for a Print Replica book, whose pages come from its PDF
// file.
func (d *Document) source() (src layout.Source, err error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	switch {
	case d.srcErr != nil:
		return layout.Source{}, d.srcErr
	case d.src != nil:
		return *d.src, nil
	}
	defer func() {
		if err == nil {
			d.src = &src
		} else {
			d.srcErr = err
		}
	}()
	// Recover runs before the function above, which then keeps its error
	defer ebook.Recover("converting mobi", &err)
	return d.convert()
}

// convert converts the book into HTML for the layout.
func (d *Document) convert() (layout.Source, error) {
	h := d.book
	if h.encryption != 0 {
		return layout.Source{}, fmt.Errorf("mobi text encrypted with type %d: %w", h.encryption, ivy.ErrDRM)
	}
	if d.kind == replica {
		// The pages of a Print Replica book are the pages of the PDF file
		// in its text, which the package pdf reads.
		return layout.Source{}, fmt.Errorf("print replica book: %w", ivy.ErrUnsupported)
	}
	text, err := d.text(h)
	if err != nil {
		return layout.Source{}, err
	}
	fsys := zipdoc.NewFS()
	src := layout.Source{
		FS:          fsys,
		StyleSheets: []string{ebook.BookStyle, mobiStyle},
		Title:       h.info(d.db).Title,
	}
	res := newResources(d, fsys, d.firstResource)
	switch d.kind {
	case palmDOC:
		fsys.Add("index.html", palmDOCHTML(ebook.Windows1252(text)))
		src.Documents = []string{"index.html"}
	case mobi6:
		doc, err := mobiHTML(text, h, res, d.lim)
		if err != nil {
			return layout.Source{}, err
		}
		fsys.Add("index.html", doc)
		src.Documents = []string{"index.html"}
	default:
		book, err := d.readKF8(h, text)
		if err != nil {
			return layout.Source{}, err
		}
		src.Documents = d.addKF8(fsys, book, res, h)
	}
	src.Documents = d.withCover(fsys, res, src.Documents, src.Title)
	return src, nil
}

// coverPage is the name of the page of the cover in the FS of the layout.
const coverPage = "cover-page.xhtml"

// withCover returns the documents of the book with a page of the cover
// image first, as calibre shows a book. The cover is the image of the first
// preview of the cover. A first document that is already a page of the
// cover, with almost no text, stays after the page of the cover, as in
// calibre. A first document that shows the cover with its text stays the
// first page. A book without a cover image is unchanged.
func (d *Document) withCover(fsys *zipdoc.FS, res *resources, docs []string, title string) []string {
	if len(docs) == 0 {
		return docs
	}
	var cover string
	for _, p := range d.previewRecords() {
		if p.kind != ivy.Cover {
			continue
		}
		name, ok := res.name(p.record - d.firstResource + 1)
		if ok && !strings.HasSuffix(name, ".ttf") {
			cover = name
		}
		break
	}
	if cover == "" || fsys.Has(coverPage) {
		return docs
	}
	if page, shows := ebook.IsCoverPage(fsys, docs[0], cover, d.lim); shows && !page {
		return docs
	}
	fsys.Add(coverPage, ebook.ImagePage(ebook.ImagePageLink(coverPage, cover), title))
	return append([]string{coverPage}, docs...)
}

// addKF8 adds the parts and the flows of a KF8 book to fsys, with their
// links to one another, and returns the names of the parts.
func (d *Document) addKF8(fsys *zipdoc.FS, book *kf8Book, res *resources, h *header) []string {
	l := &links{book: book, res: res}
	for i, part := range book.parts {
		ext := "html"
		if isXHTML(part, d.lim) {
			ext = "xhtml"
		}
		l.parts = append(l.parts, fmt.Sprintf("part%04d.%s", i, ext))
	}
	l.flows = make([]string, len(book.flows))
	for i, flow := range book.flows {
		if i > 0 && len(flow) > 0 {
			l.flows[i] = fmt.Sprintf("flow%04d.%s", i, flowType(flow))
		}
	}
	for i, flow := range book.flows {
		if l.flows[i] != "" {
			fsys.Add(l.flows[i], []byte(l.rewrite(h.decode(flow))))
		}
	}
	for i, part := range book.parts {
		doc := l.rewrite(h.decode(part))
		if h.encoding != encodingUTF8 {
			// the text is UTF-8 now, which the byte order mark tells the
			// parser of HTML
			doc = "\uFEFF" + doc
		}
		fsys.Add(l.parts[i], []byte(doc))
	}
	return l.parts
}
