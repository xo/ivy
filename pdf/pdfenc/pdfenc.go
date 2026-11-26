// Package pdfenc writes the pages of a document as a PDF file, through the
// PDF writer of okage (D58).
//
// A page that draws through the display list of ivy goes into the file as
// vectors: its paths, its images, its groups and its masks. A part that PDF
// cannot hold, such as a blend mode that PDF does not have, goes into the
// file as an image at the resolution of WithRasterDPI. A raster page, such
// as a page of PSD or of DjVu, goes into the file as one image at its own
// pixels. A page that a program implements itself goes into the file as one
// image that ivy.RenderImage draws.
//
// A run of glyphs whose format knows its text goes into the file as text
// of an embedded subset of its font, so a reader can search and copy it.
// The glyphs of a font whose license forbids a subset, of a font that the
// writer cannot read or subset, and of a run without text go into the file
// as paths. In this version, the glyphs of a PDF or PostScript file are
// paths (D58, phase 7b).
//
// The links of a page become link annotations, its anchors become named
// destinations, and its bookmarks become the outline of the file (D58,
// phase 7c).
package pdfenc

import (
	"context"
	"errors"
	"fmt"
	"image"
	"io"
	"iter"
	"log/slog"
	"maps"
	"math"
	"time"

	"github.com/xo/okage"
	"github.com/xo/okage/pdf"

	"github.com/xo/ivy"
	"github.com/xo/ivy/internal/display"
	"github.com/xo/ivy/internal/font"
)

// DefaultRasterDPI is the default of WithRasterDPI.
const DefaultRasterDPI = 300

// Option is an option of Encode and EncodePages.
type Option func(*options)

type options struct {
	cfg      ivy.Config
	pages    []int
	hasPages bool
	info     ivy.Info
	hasInfo  bool
	outline  bool
	links    bool
	dpi      float32
}

// WithConfig sets the configuration that every format shares. Encode
// reads its Logger and its Limits.
func WithConfig(cfg ivy.Config) Option {
	return func(o *options) {
		o.cfg = cfg
	}
}

// WithPages writes the pages whose numbers are in pages, from 0, in that
// order. Without it, Encode writes every page. A number past the last page
// gives ErrPageRange before Encode draws a page.
func WithPages(pages ...int) Option {
	return func(o *options) {
		o.pages = append([]int(nil), pages...)
		o.hasPages = true
	}
}

// WithInfo gives the title, the authors and the dates of the file. Without
// it, Encode takes them from Describe of the document, when the document
// is an ivy.Describer.
func WithInfo(info ivy.Info) Option {
	return func(o *options) {
		o.info = info
		o.hasInfo = true
	}
}

// WithOutline writes the outline of the document, where its format knows
// one. It is on by default. The outline holds the bookmarks of the pages, in
// the order of the pages of the file: the headings and the table of
// contents of a document of the layout, and the outline of a PDF, XPS or
// DjVu file.
func WithOutline(on bool) Option {
	return func(o *options) {
		o.outline = on
	}
}

// WithLinks writes the links of the pages, where their format knows them.
// It is on by default. A link to a URI becomes a link annotation with a URI
// action. A link inside the document becomes a link annotation to a named
// destination, and is dropped when the file does not hold the page of its
// target.
func WithLinks(on bool) Option {
	return func(o *options) {
		o.links = on
	}
}

// WithRasterDPI sets the resolution, in dots per inch, of a part that a
// PDF file cannot hold as vectors, and of a page that ivy can only render.
// The default is 300. A resolution that is not positive is the default.
func WithRasterDPI(dpi float32) Option {
	return func(o *options) {
		o.dpi = dpi
	}
}

// newOptions returns the options with their defaults.
func newOptions(opts []Option) options {
	o := options{outline: true, links: true}
	for _, opt := range opts {
		opt(&o)
	}
	if !(o.dpi > 0) || math.IsInf(float64(o.dpi), 0) {
		o.dpi = DefaultRasterDPI
	}
	o.cfg.Limits = o.cfg.Limits.WithDefaults()
	return o
}

// Encode writes the pages of d to w as one PDF file. d must be an
// ivy.Pager. Encode writes to w only after it drew every page, so w gets
// no bytes when Encode fails before it writes. When a page draws only in
// part, Encode still writes the file, and returns a *ivy.PartialError
// that names each such page.
//
// Each anchor of a page becomes a named destination of the file. A link
// to an anchor goes to the named destination, on an earlier or a later
// page.
func Encode(ctx context.Context, w io.Writer, d ivy.Document, opts ...Option) error {
	o := newOptions(opts)
	pager, ok := d.(ivy.Pager)
	if !ok {
		return fmt.Errorf("encoding pdf: the document has no pages: %w", ivy.ErrUnsupported)
	}
	if !o.hasInfo {
		if desc, ok := d.(ivy.Describer); ok {
			info, err := desc.Describe(ctx)
			if err != nil {
				if ctx.Err() != nil {
					return fmt.Errorf("encoding pdf: %w", ctx.Err())
				}
				o.cfg.Log().Debug("pdfenc: no metadata", "err", err)
			} else {
				o.info, o.hasInfo = info, true
			}
		}
	}
	numbers := o.pages
	if !o.hasPages {
		n, err := pager.PageCount(ctx)
		if err != nil {
			return fmt.Errorf("encoding pdf: counting the pages: %w", err)
		}
		numbers = make([]int, n)
		for i := range numbers {
			numbers[i] = i
		}
	}
	// Each page comes first, so that a number past the last page fails
	// before a page draws.
	pages := make([]ivy.Page, len(numbers))
	for i, n := range numbers {
		if n < 0 {
			return fmt.Errorf("encoding pdf: page %d: %w", n+1, ivy.ErrPageRange)
		}
		p, err := pager.Page(ctx, n)
		if err != nil {
			return fmt.Errorf("encoding pdf: page %d: %w", n+1, err)
		}
		pages[i] = p
	}
	e := newEncoder(o, nil)
	for i, p := range pages {
		if err := e.page(ctx, p, numbers[i]); err != nil {
			return err
		}
	}
	return e.finish(ctx, w)
}

// EncodePages writes pages to w as one PDF file, as Encode does, such as
// pages of more than one document. It writes no outline, and the metadata
// only from WithInfo. With WithPages, it writes the pages of the sequence
// whose numbers are in the list, from 0, in the order of the list.
func EncodePages(ctx context.Context, w io.Writer, pages iter.Seq2[ivy.Page, error], opts ...Option) error {
	o := newOptions(opts)
	o.outline = false
	e := newEncoder(o, nil)
	if o.hasPages {
		var all []ivy.Page
		for p, err := range pages {
			if err != nil {
				return fmt.Errorf("encoding pdf: page %d: %w", len(all)+1, err)
			}
			if len(all) >= o.cfg.Limits.MaxObjects {
				return fmt.Errorf("encoding pdf: more than %d pages: %w", o.cfg.Limits.MaxObjects, ivy.ErrLimit)
			}
			all = append(all, p)
		}
		for _, n := range o.pages {
			if n < 0 || n >= len(all) {
				return fmt.Errorf("encoding pdf: page %d: %w", n+1, ivy.ErrPageRange)
			}
		}
		for _, n := range o.pages {
			if err := e.page(ctx, all[n], n); err != nil {
				return err
			}
		}
		return e.finish(ctx, w)
	}
	n := 0
	for p, err := range pages {
		if err != nil {
			return fmt.Errorf("encoding pdf: page %d: %w", n+1, err)
		}
		if err := e.page(ctx, p, n); err != nil {
			return err
		}
		n++
	}
	return e.finish(ctx, w)
}

// encoder holds the document that the pages go into, and what the pages
// share: the images, the masks and the count of the bytes of the file.
type encoder struct {
	o   options
	lim ivy.Limits
	log *slog.Logger
	doc *pdf.Document

	// pages is the number of pages of the file.
	pages int
	// partial holds the errors of the pages that drew in part.
	partial []error

	images map[imageKey]*pdf.Image
	masks  map[maskKey]*pdf.Mask
	// fonts holds the font of each source of a face, and nil for a source
	// whose glyphs are paths. blocked holds the sources whose subset an
	// earlier attempt did not make.
	fonts   map[font.SourceKey]*embedded
	blocked map[font.SourceKey]bool
	// drawn holds the pages of the file, which a second attempt draws
	// again.
	drawn []drawnPage
	// bytes counts the bytes that the writer holds: the pixels of the
	// images, and the commands of the pages.
	bytes int64

	// anchors holds the destination of each anchor that the pages named,
	// and linked holds the names of the anchors that a link goes to.
	anchors map[string]destination
	linked  map[string]bool
	// bookmarks holds the entries of the outline, in the order of the
	// pages. marks counts the links, the anchors and the bookmarks.
	bookmarks []bookmark
	marks     int
	// err is the first error of a limit. The devices draw nothing after it.
	err error
}

// drawnPage is a page of the file, number n of its document from 0.
type drawnPage struct {
	p ivy.Page
	n int
}

// newEncoder returns an encoder with an empty document. The glyphs of the
// fonts of blocked are paths.
func newEncoder(o options, blocked map[font.SourceKey]bool) *encoder {
	return &encoder{
		o:       o,
		lim:     o.cfg.Limits,
		log:     o.cfg.Log(),
		doc:     pdf.NewDocument(),
		images:  map[imageKey]*pdf.Image{},
		masks:   map[maskKey]*pdf.Mask{},
		fonts:   map[font.SourceKey]*embedded{},
		blocked: blocked,
		anchors: map[string]destination{},
		linked:  map[string]bool{},
	}
}

// charge adds n bytes to the count of the bytes of the file. Past
// MaxMemory, it keeps an error that wraps ivy.ErrLimit and returns false.
func (e *encoder) charge(n int64) bool {
	if e.err != nil {
		return false
	}
	e.bytes += max(n, 0)
	if e.bytes > e.lim.MaxMemory {
		e.err = fmt.Errorf("encoding pdf: the file holds more than %d bytes: %w", e.lim.MaxMemory, ivy.ErrLimit)
		return false
	}
	return true
}

// page draws page p, number n of its document from 0, onto a new page of
// the file.
func (e *encoder) page(ctx context.Context, p ivy.Page, n int) error {
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("encoding pdf: %w", err)
	}
	if e.pages >= e.lim.MaxObjects {
		return fmt.Errorf("encoding pdf: more than %d pages: %w", e.lim.MaxObjects, ivy.ErrLimit)
	}
	w, h := p.Size()
	settings, ok := pdf.PageSettingsFromWH(w, h)
	if !ok || math.IsInf(float64(w), 0) || math.IsInf(float64(h), 0) {
		return fmt.Errorf("encoding pdf: page %d of %gx%g points: %w", n+1, w, h, ivy.ErrFormat)
	}
	e.drawn = append(e.drawn, drawnPage{p: p, n: n})
	page := e.doc.StartPage(settings)
	s := page.Surface()
	dev := newDevice(e, page, s, e.pages, w, h)
	err := drawPage(ctx, p, dev, e.o.dpi)
	dev.finish()
	s.Finish()
	page.Finish()
	e.pages++
	if e.err != nil {
		return fmt.Errorf("encoding pdf: page %d: %w", n+1, e.err)
	}
	if pe, ok := errors.AsType[*ivy.PartialError](err); ok {
		e.log.Debug("pdfenc: page drawn in part", "page", n+1, "err", pe.Err)
		e.partial = append(e.partial, fmt.Errorf("page %d: %w", n+1, pe.Err))
		return nil
	}
	if err != nil {
		return fmt.Errorf("encoding pdf: page %d: %w", n+1, err)
	}
	return nil
}

// imager is a raster page, such as a page of internal/raster, which gives
// its image at its own pixels, one for each point.
type imager interface {
	Image(ctx context.Context) (*image.RGBA, error)
}

// drawPage draws p onto dev: through the display list when p is a
// display.Drawer, as its own image when p is a raster page, and as one
// image of ivy.RenderImage at dpi otherwise.
func drawPage(ctx context.Context, p ivy.Page, dev *device, dpi float32) error {
	switch p := p.(type) {
	case display.Drawer:
		return p.Draw(ctx, dev, dpi/72)
	case imager:
		img, err := p.Image(ctx)
		if err != nil {
			return err
		}
		dev.DrawImage(img, okage.IdentityTransform(), display.ImageOptions{Opacity: 1, Smooth: true})
		return nil
	}
	img, err := ivy.RenderImage(ctx, p, dpi, nil)
	if img != nil {
		s := 72 / dpi
		dev.DrawImage(img, okage.TransformFromScale(s, s), display.ImageOptions{Opacity: 1, Smooth: true})
	}
	return err
}

// finish writes the file to w, with the metadata. When the writer fails
// and a font cannot give its subset, finish draws the pages again onto a
// new encoder, with the glyphs of that font as paths (D58).
func (e *encoder) finish(ctx context.Context, w io.Writer) error {
	if e.o.hasInfo {
		e.doc.SetMetadata(metadata(e.o.info))
	}
	e.writeLinks()
	ew := &errWriter{w: w}
	if err := finishDocument(e.doc, ew); err != nil {
		if ew.err == nil {
			if failed := e.failedFonts(); len(failed) > 0 {
				return e.again(ctx, w, failed)
			}
		}
		switch {
		case ew.err != nil:
			return fmt.Errorf("encoding pdf: %w", ew.err)
		case errors.Is(err, okage.ErrPDFTooLargeFloat), errors.Is(err, okage.ErrPDFTooLongArray),
			errors.Is(err, okage.ErrPDFTooLongDictionary):
			return fmt.Errorf("encoding pdf: %w: %w", ivy.ErrLimit, err)
		}
		return fmt.Errorf("encoding pdf: %w: %w", ivy.ErrUnsupported, err)
	}
	if len(e.partial) > 0 {
		return &ivy.PartialError{Err: fmt.Errorf("encoding pdf: %w", errors.Join(e.partial...))}
	}
	return nil
}

// finishDocument writes doc to w. A panic of the writer, such as on a
// font from a stranger, is an error (D5).
func finishDocument(doc *pdf.Document, w io.Writer) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("writing the file: %v: %w", r, ivy.ErrFormat)
		}
	}()
	return doc.Finish(w) //nolint:wrapcheck // finish wraps the error of the writer
}

// again draws the pages of e onto a new encoder whose fonts of failed, and
// of the fonts that e blocked, are paths, and writes the file to w.
func (e *encoder) again(ctx context.Context, w io.Writer, failed map[font.SourceKey]bool) error {
	blocked := maps.Clone(e.blocked)
	if blocked == nil {
		blocked = map[font.SourceKey]bool{}
	}
	maps.Copy(blocked, failed)
	next := newEncoder(e.o, blocked)
	for _, d := range e.drawn {
		if err := next.page(ctx, d.p, d.n); err != nil {
			return err
		}
	}
	return next.finish(ctx, w)
}

// errWriter keeps the error of its writer, so that Encode tells an error of
// the writer from an error of the file.
type errWriter struct {
	w   io.Writer
	err error
}

// Write satisfies the io.Writer interface.
func (ew *errWriter) Write(p []byte) (int, error) {
	n, err := ew.w.Write(p)
	if err != nil {
		ew.err = err
	}
	return n, err //nolint:wrapcheck // the error of the writer goes on as it is, as io.Writer says
}

// metadata maps the information of a document onto the metadata of the
// file (D58). Publisher, Identifier and Modified have no field there.
func metadata(info ivy.Info) pdf.Metadata {
	m := pdf.NewMetadata().
		Title(info.Title).
		Description(info.Subject).
		Language(info.Language).
		Creator(info.Producer)
	if len(info.Authors) > 0 {
		m = m.Authors(info.Authors)
	}
	if dt, ok := dateTime(info.Created); ok {
		m = m.CreationDate(dt)
	}
	return m
}

// dateTime converts t into a date of the file. It returns false for the
// zero time and for a year that PDF cannot hold.
func dateTime(t time.Time) (pdf.DateTime, bool) {
	if t.IsZero() || t.Year() < 0 || t.Year() > 9999 {
		return pdf.DateTime{}, false
	}
	_, offset := t.Zone()
	hours, minutes := offset/3600, (offset%3600)/60
	if minutes < 0 {
		minutes = -minutes
	}
	return pdf.NewDateTime(uint16(t.Year())).
		Month(uint8(t.Month())).
		Day(uint8(t.Day())).
		Hour(uint8(t.Hour())).
		Minute(uint8(t.Minute())).
		Second(uint8(t.Second())).
		UTCOffsetHour(int8(hours)).
		UTCOffsetMinute(uint8(minutes)), true
}
