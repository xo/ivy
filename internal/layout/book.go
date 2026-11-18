// This file is not a port. It puts the documents of a whole Source on pages,
// one after the other, for the formats of ebooks. Upstream lays out one
// document at a time.

package layout

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"math"
	"runtime"
	"slices"
	"sync"
	"sync/atomic"

	"github.com/xo/okage"
	"github.com/xo/okage/svg"

	"github.com/xo/ivy"
	"github.com/xo/ivy/internal/display"
	"github.com/xo/ivy/internal/fontdb"
)

// maxLoadedDocuments is the number of documents of a book whose layout a
// Book keeps. A document past it is laid out again when a page of it is
// asked for. The pages that a program holds stay valid.
const maxLoadedDocuments = 4

// pointsPerPixel converts the CSS pixels of a page to points.
const pointsPerPixel = 0.75

// BookOptions are the options of NewBook.
type BookOptions struct {
	// Limits bound the work on the book. A zero field takes its default.
	Limits ivy.Limits
	// Logger receives the log of the book, or nil.
	Logger *slog.Logger
	// Fonts are the fonts of the book, or nil for the default fonts: the
	// fonts of FontFiles, then the Go fonts, then the fonts of the system
	// when SystemFonts is set.
	Fonts *svg.FontDatabase
	// FontFiles holds font files that come before the Go fonts, such as the
	// fonts of the module github.com/xo/ivy/fonts, or nil. A program gives
	// them with ivy.Config.Fonts (D24). It is ignored when Fonts is set.
	FontFiles fs.FS
	// SystemFonts adds the fonts of the system to the default fonts. It is
	// ignored when Fonts is set.
	SystemFonts bool
}

// Book is a paged document over the documents of a Source. It lays out the
// documents in their order, with the style sheets of the Source as user
// style sheets and one FontConfiguration for the whole book. The pages are
// numbered across the book, from 0.
//
// The layout is lazy: Page(n) lays out only the documents and the pages up to
// page n. Each document is one document of upstream. counter(page) goes on
// across the documents, and counter(pages) is the number of pages of the
// whole book (D43). When a document uses counter(pages), the book lays out
// the documents after it before it gives a page of that document. The book
// takes the number of pages of a document to be the same whatever the
// number of its first page.
//
// A document that cannot be read or laid out is logged and left out, so a
// book with one bad chapter still opens. If no document gives a page, the
// book returns the first error.
//
// A Book and its pages are safe for concurrent use. One mutex serializes the
// layout and the drawing, because the styles and the fonts are not safe for
// concurrent use.
type Book struct {
	mu   sync.Mutex
	src  Source
	opts BookOptions
	lim  ivy.Limits
	log  *slog.Logger
	fc   *FontConfiguration
	docs []*bookDocument
	// loaded holds the documents whose layout is kept, the most recent
	// last.
	loaded []*bookDocument
	// loads counts the documents that the book read and laid out, for the
	// tests of the laziness.
	loads int
	// err is the first error of a document that the book left out.
	err error
	// recordings holds the pages that Render recorded, the most recent
	// last.
	recordings []*recording
	// db is the font database of the program, and first the number of its
	// first faces, from which countAll makes a font configuration for each
	// goroutine.
	db    *svg.FontDatabase
	first int
}

// maxRecordings is the number of recorded pages that a book keeps.
const maxRecordings = 4

// recording is a page that bookPage.Render drew into a list of commands, at
// a transform whose translation is less than a pixel. The bands of a page
// that RenderImage draws replay one recording, each moved by a whole number
// of pixels (D47).
type recording struct {
	box  *Box
	t    okage.Transform
	list *display.List
	// err is nil or the *ivy.PartialError of the drawing.
	err error
}

// bookDocument is a document of a book.
type bookDocument struct {
	path string
	// doc is the layout of the document, or nil when it is not loaded.
	doc *Document
	// count is the number of pages of the document, or -1 when it is not
	// known yet.
	count int
	// failed is set when the layout of the document failed after count
	// pages. The document keeps these pages.
	failed bool
	// before is the number of pages of the documents before it, which
	// the counter page of the document starts from.
	before int
	// after is the number of pages of the documents after it, which the
	// counter pages adds, or -1 when it is not known yet.
	after int
}

// NewBook returns the book of the documents of src. It reads no document
// yet.
func NewBook(src Source, opts BookOptions) *Book {
	lim := opts.Limits.WithDefaults()
	log := opts.Logger
	if log == nil {
		log = slog.New(slog.DiscardHandler)
	}
	db, first := opts.Fonts, 0
	if db == nil {
		db, first = fontdb.Load(opts.FontFiles, opts.SystemFonts, lim, log)
	}
	b := &Book{
		src:   src,
		opts:  opts,
		lim:   lim,
		log:   log,
		db:    db,
		first: first,
		fc: NewFontConfiguration(db, FontConfigurationOptions{
			Limits:     lim,
			Logger:     opts.Logger,
			FirstFaces: first,
		}),
	}
	docs := src.Documents
	// The documents are bounded as the pages are: each one has a page at
	// least.
	if n := b.maxPages(); len(docs) > n {
		log.Warn("layout: book with too many documents", "documents", len(docs), "max", n)
		docs = docs[:n]
	}
	for _, path := range docs {
		b.docs = append(b.docs, &bookDocument{path: path, count: -1, after: -1})
	}
	return b
}

// maxPages is the number of pages of a book: the bound of the pages of one
// document of Document.
func (b *Book) maxPages() int {
	return max(1, b.lim.MaxObjects/16)
}

// PageCount lays out the whole book and returns the number of its pages.
func (b *Book) PageCount(ctx context.Context) (n int, err error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.countAll(ctx)
	for _, bd := range b.docs {
		b.place(bd, n)
		count, err := b.count(ctx, bd)
		if err != nil {
			return 0, err
		}
		n += count
		if n > b.maxPages() {
			return 0, fmt.Errorf("book of more than %d pages: %w", b.maxPages(), ivy.ErrLimit)
		}
	}
	if n == 0 && b.err != nil {
		return 0, b.err
	}
	return n, nil
}

// Page returns page n of the book, from 0. It lays out the documents and the
// pages up to n. It returns an error that wraps ivy.ErrPageRange past the
// last page.
func (b *Book) Page(ctx context.Context, n int) (ivy.Page, error) {
	if n < 0 {
		return nil, fmt.Errorf("page %d: %w", n, ivy.ErrPageRange)
	}
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("laying out page %d: %w", n+1, err)
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	offset := 0
	for _, bd := range b.docs {
		if offset >= b.maxPages() {
			return nil, fmt.Errorf("book of more than %d pages: %w", b.maxPages(), ivy.ErrLimit)
		}
		if bd.count >= 0 && n >= offset+bd.count {
			offset += bd.count
			continue
		}
		b.place(bd, offset)
		box, err := b.page(ctx, bd, n-offset)
		switch {
		case err == nil:
			return &bookPage{book: b, box: box, n: n}, nil
		case errors.Is(err, ErrNoPage):
			// The document has fewer pages: its count is now known.
			offset += bd.count
		default:
			return nil, err
		}
	}
	if offset == 0 && b.err != nil {
		return nil, b.err
	}
	return nil, fmt.Errorf("page %d of %d: %w", n+1, offset, ivy.ErrPageRange)
}

// load reads, styles and builds a document, and keeps it among the loaded
// documents. It returns false for a document that cannot be read, which the
// book leaves out with 0 pages. When the document uses counter(pages), it
// counts the pages of the documents after it first. It returns the error of
// ctx, and an error that wraps ivy.ErrLimit when the book has too many
// pages.
func (b *Book) load(ctx context.Context, bd *bookDocument) (bool, error) {
	if bd.doc != nil {
		b.touch(bd)
		return true, nil
	}
	if err := ctx.Err(); err != nil {
		return false, fmt.Errorf("laying out %s: %w", bd.path, err)
	}
	doc, err := b.build(bd.path, bd.before)
	if err != nil {
		b.skip(bd, err)
		return false, nil
	}
	b.loads++
	if doc.usesPages {
		if err := b.countAfter(ctx, bd); err != nil {
			return false, err
		}
		doc.pagesAfter = bd.after
	}
	bd.doc = doc
	b.loaded = append(b.loaded, bd)
	if len(b.loaded) > maxLoadedDocuments {
		b.loaded[0].doc = nil
		b.loaded = b.loaded[1:]
	}
	return true, nil
}

// countAfter sets the number of pages of the documents after bd. It counts
// them from the last one, so that a document after bd that uses
// counter(pages) finds the counts of the documents after it.
func (b *Book) countAfter(ctx context.Context, bd *bookDocument) error {
	if bd.after >= 0 {
		return nil
	}
	i := len(b.docs) - 1
	for i >= 0 && b.docs[i] != bd {
		i--
	}
	after := 0
	for _, next := range slices.Backward(b.docs[i+1:]) {
		n, err := b.count(ctx, next)
		if err != nil {
			return err
		}
		after += n
		if after > b.maxPages() {
			return fmt.Errorf("book of more than %d pages: %w", b.maxPages(), ivy.ErrLimit)
		}
	}
	bd.after = after
	return nil
}

// place sets the number of pages before a document. A loaded document
// whose number changes, which a document before it that failed later can
// make, is laid out again.
func (b *Book) place(bd *bookDocument, before int) {
	if bd.before != before {
		bd.before = before
		b.unload(bd)
	}
}

// touch moves a loaded document to the end of the loaded documents.
func (b *Book) touch(bd *bookDocument) {
	for i, l := range b.loaded {
		if l == bd {
			b.loaded = append(append(b.loaded[:i:i], b.loaded[i+1:]...), bd)
			return
		}
	}
}

// maxCountWorkers bounds the documents that countAll lays out at the same
// time, because each one holds its layout.
const maxCountWorkers = 8

// countAll counts the pages of the documents whose count is not known, on
// up to one goroutine for each CPU, before PageCount walks the documents
// (D48). Each goroutine lays out a document with a font configuration of its
// own, keeps its count and drops its layout. The count of a document does
// not depend on the pages before it, as place assumes. A document that uses
// counter(pages), or whose layout fails, keeps no count: the walk lays it
// out in order, as before.
func (b *Book) countAll(ctx context.Context) {
	var todo []*bookDocument
	for _, bd := range b.docs {
		if bd.count < 0 && !bd.failed && bd.doc == nil {
			todo = append(todo, bd)
		}
	}
	workers := min(runtime.GOMAXPROCS(0), maxCountWorkers, len(todo))
	if workers < 2 {
		return
	}
	counts := make([]int, len(todo))
	for i := range counts {
		counts[i] = -1
	}
	var next atomic.Int64
	var wg sync.WaitGroup
	for range workers {
		wg.Go(func() {
			fc := NewFontConfiguration(b.db, FontConfigurationOptions{
				Limits:     b.lim,
				Logger:     b.opts.Logger,
				FirstFaces: b.first,
			})
			for {
				i := int(next.Add(1)) - 1
				if i >= len(todo) || ctx.Err() != nil {
					return
				}
				counts[i] = b.countAlone(ctx, todo[i], fc)
			}
		})
	}
	wg.Wait()
	for i, bd := range todo {
		if counts[i] >= 0 {
			bd.count = counts[i]
		}
	}
}

// countAlone lays out a document with fc, and returns its number of pages,
// or -1 when the walk of PageCount must lay it out: for counter(pages), an
// error or a panic.
func (b *Book) countAlone(ctx context.Context, bd *bookDocument, fc *FontConfiguration) (n int) {
	defer func() {
		if recover() != nil {
			n = -1
		}
	}()
	doc, err := b.buildWith(bd.path, 0, fc)
	if err != nil || doc.usesPages {
		return -1
	}
	n, err = doc.Count(ctx)
	if err != nil {
		return -1
	}
	return n
}

// build reads a document of the source, computes its styles and builds its
// boxes.
func (b *Book) build(path string, before int) (doc *Document, err error) {
	return b.buildWith(path, before, b.fc)
}

// buildWith is build, with the font configuration fc.
func (b *Book) buildWith(path string, before int, fc *FontConfiguration) (doc *Document, err error) {
	defer func() {
		if r := recover(); r != nil {
			doc, err = nil, fmt.Errorf("%w: panic in %s: %v", ivy.ErrFormat, path, r)
		}
	}()
	h, err := LoadHTML(b.src, path, HTMLOptions{Limits: b.lim, Logger: b.opts.Logger})
	if err != nil {
		return nil, err
	}
	user := make([]CSSSource, len(b.src.StyleSheets))
	for i, text := range b.src.StyleSheets {
		user[i] = CSSSource{Text: text}
	}
	targetCollector := NewTargetCollector(h.log)
	counterStyle := CounterStyle{}
	styleFor, err := GetAllComputedStyles(h, ComputedStylesOptions{
		UserStylesheets: user,
		// The presentational hints, such as the width of an image, are on,
		// because many ebooks rely on them.
		PresentationalHints: true,
		FontConfig:          fc,
		CounterStyle:        counterStyle,
		TargetCollector:     targetCollector,
	})
	if err != nil {
		return nil, fmt.Errorf("styling %s: %w", path, err)
	}
	root, footnotes, err := BuildFormattingStructure(h, styleFor, BuildOptions{
		TargetCollector: targetCollector,
		CounterStyle:    counterStyle,
	})
	if err != nil {
		return nil, fmt.Errorf("building the boxes of %s: %w", path, err)
	}
	if root == nil {
		return nil, fmt.Errorf("%s has no box: %w", path, ivy.ErrFormat)
	}
	return NewDocument(h, root, styleFor, DocumentOptions{
		FontConfig:      fc,
		TargetCollector: targetCollector,
		CounterStyle:    counterStyle,
		Footnotes:       footnotes,
		PagesBefore:     before,
	}), nil
}

// skip leaves out a document that failed with err, with 0 pages.
func (b *Book) skip(bd *bookDocument, err error) {
	b.log.Warn("layout: document left out", "path", bd.path, "err", err)
	b.keep(bd, err)
	bd.count = 0
	bd.failed = true
}

// keep keeps the first error of a document that the book left out or cut.
// An error that is not an error of ivy, such as a file of the source that
// cannot be read, wraps ivy.ErrFormat.
func (b *Book) keep(bd *bookDocument, err error) {
	if b.err != nil {
		return
	}
	for _, known := range []error{ivy.ErrFormat, ivy.ErrLimit, ivy.ErrDRM, ivy.ErrUnsupported, ivy.ErrPassword} {
		if errors.Is(err, known) {
			b.err = fmt.Errorf("laying out %s: %w", bd.path, err)
			return
		}
	}
	b.err = fmt.Errorf("laying out %s: %w: %w", bd.path, ivy.ErrFormat, err)
}

// isStop reports whether err stops the book: the error of a context.
func isStop(err error) bool {
	return errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded)
}

// page returns page i of a document. It returns an error that wraps
// ErrNoPage when the document has fewer pages, and sets its count.
func (b *Book) page(ctx context.Context, bd *bookDocument, i int) (*Box, error) {
	if bd.count >= 0 && i >= bd.count {
		return nil, ErrNoPage
	}
	ok, err := b.load(ctx, bd)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, ErrNoPage
	}
	box, err := bd.doc.Page(ctx, i)
	switch {
	case err == nil:
		return box, nil
	case isStop(err):
		// The layout stopped in the middle of a page: lay out the document
		// again next time.
		b.unload(bd)
		return nil, fmt.Errorf("laying out %s: %w", bd.path, err)
	case errors.Is(err, ErrNoPage):
		if bd.count < 0 {
			if _, err := b.count(ctx, bd); err != nil {
				return nil, err
			}
		}
		return nil, ErrNoPage
	case bd.failed:
		// The document failed before at a later page. A new layout gives
		// the pages before.
		b.unload(bd)
		if ok, err := b.load(ctx, bd); err != nil || !ok {
			return nil, cmpErr(err, ErrNoPage)
		}
		if box, err := bd.doc.Page(ctx, i); err == nil {
			return box, nil
		}
		return nil, ErrNoPage
	}
	// The document fails at page i: it keeps the pages before.
	b.log.Warn("layout: document cut", "path", bd.path, "page", i+1, "err", err)
	b.keep(bd, err)
	bd.count = i
	bd.failed = true
	return nil, ErrNoPage
}

// cmpErr returns err, or def when err is nil.
func cmpErr(err, def error) error {
	if err != nil {
		return err
	}
	return def
}

// unload drops the layout of a document.
func (b *Book) unload(bd *bookDocument) {
	bd.doc = nil
	for i, l := range b.loaded {
		if l == bd {
			b.loaded = append(b.loaded[:i], b.loaded[i+1:]...)
			return
		}
	}
}

// count returns the number of pages of a document. It lays out the whole
// document when the count is not known.
func (b *Book) count(ctx context.Context, bd *bookDocument) (int, error) {
	if bd.count >= 0 {
		return bd.count, nil
	}
	ok, err := b.load(ctx, bd)
	if err != nil {
		return 0, err
	}
	if !ok {
		return 0, nil
	}
	n, err := bd.doc.Count(ctx)
	switch {
	case err == nil:
		bd.count = n
		return n, nil
	case isStop(err):
		b.unload(bd)
		return 0, fmt.Errorf("laying out %s: %w", bd.path, err)
	}
	// The whole layout failed. The pages of the first pass that the
	// document gives stay, until the first one that fails.
	b.unload(bd)
	if ok, err := b.load(ctx, bd); err != nil || !ok {
		return 0, err
	}
	n = 0
	for n < b.maxPages() {
		if _, err := bd.doc.Page(ctx, n); err != nil {
			if isStop(err) {
				b.unload(bd)
				return 0, fmt.Errorf("laying out %s: %w", bd.path, err)
			}
			break
		}
		n++
	}
	b.log.Warn("layout: document cut", "path", bd.path, "pages", n, "err", err)
	b.keep(bd, err)
	bd.count = n
	bd.failed = true
	return n, nil
}

// bookPage is a page of a book.
type bookPage struct {
	book *Book
	box  *Box
	n    int
}

// The interfaces that a page implements.
var _ ivy.Page = (*bookPage)(nil)

// Size satisfies the ivy.Page interface: the size of the margin box of the
// page, in points.
func (p *bookPage) Size() (float32, float32) {
	return float32(p.box.MarginWidth() * pointsPerPixel), float32(p.box.MarginHeight() * pointsPerPixel)
}

// Render satisfies the ivy.Page interface. It returns an *ivy.PartialError
// when it drew the page in part.
//
// Render records the page once into a list of commands, at t without the
// whole pixels of its translation, and replays the list onto dst moved by
// those pixels. The calls of Render for the bands of one page share the
// recording, and replay it at the same time without the lock of the book
// (D47). The commands are the same as those of a drawing at t.
func (p *bookPage) Render(ctx context.Context, dst *okage.Pixmap, t okage.Transform) error {
	x, y := float32(math.Floor(float64(t.TX))), float32(math.Floor(float64(t.TY)))
	if math.IsInf(float64(x), 0) || math.IsNaN(float64(x)) || math.IsInf(float64(y), 0) || math.IsNaN(float64(y)) {
		x, y = 0, 0
	}
	at := t
	at.TX, at.TY = t.TX-x, t.TY-y
	rec, err := p.record(ctx, at)
	if err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("rendering page %d: %w", p.n+1, err)
	}
	rec.list.Replay(display.NewOkage(dst, okage.TransformFromTranslate(x, y)))
	return rec.err
}

// record returns the recording of the page at t, and draws it when the
// book does not hold it. It returns an error that is not an
// *ivy.PartialError when the drawing failed.
func (p *bookPage) record(ctx context.Context, t okage.Transform) (*recording, error) {
	b := p.book
	b.mu.Lock()
	defer b.mu.Unlock()
	for _, rec := range b.recordings {
		if rec.box == p.box && rec.t == t {
			return rec, nil
		}
	}
	list := display.NewRecording()
	err := p.drawLocked(ctx, list, t, 1)
	if _, ok := errors.AsType[*ivy.PartialError](err); err != nil && !ok {
		return nil, err
	}
	rec := &recording{box: p.box, t: t, list: list, err: err}
	if len(b.recordings) >= maxRecordings {
		b.recordings = slices.Delete(b.recordings, 0, len(b.recordings)-maxRecordings+1)
	}
	b.recordings = append(b.recordings, rec)
	return rec, nil
}

// Draw draws page n of the book, from 0, onto dev. t maps the points of the
// page onto the user space of dev. scale is the number of pixels of the
// device for a unit of its user space: the drawing renders the SVG images
// and the patterns of the page at that resolution. A scale that is not
// positive is 1.
//
// Draw sends the text, the paths and the images to dev as commands of the
// display list, so a device that records the commands holds the glyphs of
// the text. The clip and the groups of dev apply to the page. Draw saves
// and restores the state of dev. It returns an *ivy.PartialError when it
// drew the page in part, and an error that wraps ivy.ErrPageRange past the
// last page.
func (b *Book) Draw(ctx context.Context, n int, dev display.Device, t okage.Transform, scale float64) error {
	page, err := b.Page(ctx, n)
	if err != nil {
		return err
	}
	return page.(*bookPage).draw(ctx, dev, t, scale)
}

// draw draws the page onto dev, as Book.Draw does.
func (p *bookPage) draw(ctx context.Context, dev display.Device, t okage.Transform, scale float64) (err error) {
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("rendering page %d: %w", p.n+1, err)
	}
	p.book.mu.Lock()
	defer p.book.mu.Unlock()
	return p.drawLocked(ctx, dev, t, scale)
}

// drawLocked is draw, with the lock of the book held.
func (p *bookPage) drawLocked(ctx context.Context, dev display.Device, t okage.Transform, scale float64) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = &ivy.PartialError{Err: fmt.Errorf("rendering page %d: panic: %v: %w", p.n+1, r, ivy.ErrFormat)}
		}
	}()
	if !(scale > 0) || math.IsInf(scale, 0) {
		scale = 1
	}
	// DrawPage takes the transform onto the pixels, so the device first
	// undoes the scale.
	dev.Save()
	defer dev.Restore()
	if scale != 1 {
		dev.Concat(okage.TransformFromScale(float32(1/scale), float32(1/scale)))
	}
	toDevice := t.PostScale(float32(scale), float32(scale)).PreScale(pointsPerPixel, pointsPerPixel)
	err = DrawPage(ctx, dev, p.box, toDevice, DrawOptions{
		FontConfig: p.book.fc,
		Limits:     p.book.lim,
		Logger:     p.book.opts.Logger,
	})
	if pe, ok := errors.AsType[*ivy.PartialError](err); ok {
		return &ivy.PartialError{Err: fmt.Errorf("rendering page %d: %w", p.n+1, pe.Err)}
	}
	if err != nil {
		return fmt.Errorf("rendering page %d: %w", p.n+1, err)
	}
	return nil
}
