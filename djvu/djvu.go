// Package djvu opens DjVu documents, and renders their pages.
//
// DjVu is a format for scanned books (D26). A page holds layers: a mask of
// the text and the line art, in JB2 or in G4 (MMR), the colors of the mask,
// and a background picture, in IW44 wavelets or in JPEG. The package reads
// a single page (FORM:DJVU) and a bundled document (FORM:DJVM), with the
// shared dictionaries of shapes that its pages include. It decodes the
// layers with a copy of the decoders of dejapdf, in djvu/internal (D7).
//
// The package composites the layers of a page into one image at about the
// resolution of the render, and draws the image through the display list of
// ivy (D10). It reduces the mask by its area, so that thin strokes of a
// large bilevel page stay visible at a small size, and it reconstructs the
// background only down to the scale that the render needs.
//
// An indirect document names other files, and a source of ivy has no
// folder, so Open returns an error that wraps ivy.ErrUnsupported for it.
// The text layer and the outline are left out.
package djvu

import (
	"context"
	"errors"
	"fmt"
	"image"
	"iter"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/xo/ivy"
	"github.com/xo/ivy/djvu/internal/bzz"
	"github.com/xo/ivy/djvu/internal/doc"
	"github.com/xo/ivy/djvu/internal/iff"
	"github.com/xo/ivy/djvu/internal/iw44"
)

// Format describes the package for the registry of a program.
var Format = ivy.Format{
	Name:  "djvu",
	Mimes: []string{"image/vnd.djvu", "image/x-djvu"},
	Exts:  []string{"djvu", "djv"},
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

// maxCached is the number of pages whose decoded layers a document keeps.
const maxCached = 4

// Document is an open DjVu document.
type Document struct {
	doc   *doc.Document
	lim   ivy.Limits
	log   *slog.Logger
	pages []*Page

	// mu guards recent, the pages whose layers are decoded, from the
	// oldest, and the size of each.
	mu     sync.Mutex
	recent []*Page
}

// The capabilities of a document.
var (
	_ ivy.Pager     = (*Document)(nil)
	_ ivy.Previewer = (*Document)(nil)
	_ ivy.Describer = (*Document)(nil)
)

// Open opens a DjVu document. It reads the whole file into memory, and
// the directory of a bundled document. It decodes the layers of a page when
// the page renders. It returns an error that wraps ivy.ErrFormat if src is
// not a DjVu file, and one that wraps ivy.ErrUnsupported for an indirect
// document.
func Open(ctx context.Context, src ivy.Source, opts ...Option) (d *Document, err error) {
	var o options
	for _, opt := range opts {
		opt(&o)
	}
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("opening djvu: %w", err)
	}
	defer recoverTo("opening djvu", &err)
	lim := o.cfg.Limits.WithDefaults()
	data, err := read(src)
	if err != nil {
		return nil, err
	}
	dd, err := doc.ParseWithin(data, nil, lim)
	if err != nil {
		return nil, fmt.Errorf("reading djvu: %w", format(err))
	}
	if dd.Dir != nil && !dd.Dir.Bundled {
		return nil, fmt.Errorf("indirect djvu document: %w", ivy.ErrUnsupported)
	}
	if depth(dd.Root) > lim.MaxDepth {
		return nil, fmt.Errorf("djvu chunks nested more than %d deep: %w", lim.MaxDepth, ivy.ErrLimit)
	}
	if len(dd.Pages) > lim.MaxObjects {
		return nil, fmt.Errorf("djvu of %d pages: %w", len(dd.Pages), ivy.ErrLimit)
	}
	d = &Document{doc: dd, lim: lim, log: o.cfg.Log()}
	for _, w := range dd.Warnings {
		d.log.Debug("djvu: damaged document", "err", w)
	}
	// a page that cannot be read takes the size of the first page that can
	w, h := float32(612), float32(792)
	for _, p := range dd.Pages {
		if p.Err == nil {
			w, h = pageSize(p.Info)
			break
		}
	}
	d.pages = make([]*Page, len(dd.Pages))
	for i, p := range dd.Pages {
		d.pages[i] = &Page{d: d, p: p, n: i, w: w, h: h}
		if p.Err == nil {
			d.pages[i].w, d.pages[i].h = pageSize(p.Info)
		}
	}
	return d, nil
}

// read reads the root chunk of a DjVu file from src, with the magic AT&T
// before it, if any. It reads no more than the size of the root chunk.
func read(src ivy.Source) ([]byte, error) {
	var head [16]byte
	size := src.Size()
	if size < 12 {
		return nil, fmt.Errorf("djvu of %d bytes: %w", size, ivy.ErrFormat)
	}
	n, err := src.ReadAt(head[:min(size, 16)], 0)
	if n < 12 {
		return nil, fmt.Errorf("reading djvu: %w", errors.Join(err, ivy.ErrFormat))
	}
	off := int64(0)
	if string(head[:4]) == "AT&T" {
		off = 4
	}
	if string(head[off:off+4]) != "FORM" {
		return nil, fmt.Errorf("djvu without a FORM chunk: %w", ivy.ErrFormat)
	}
	if n < int(off)+12 {
		return nil, fmt.Errorf("djvu of %d bytes: %w", size, ivy.ErrFormat)
	}
	kind := string(head[off+8 : off+12])
	switch kind {
	case "DJVU", "DJVM":
	case "DJVI", "THUM", "BM44", "PM44":
		return nil, fmt.Errorf("djvu file of the type %s: %w", kind, ivy.ErrUnsupported)
	default:
		return nil, fmt.Errorf("djvu file of the type %q: %w", kind, ivy.ErrFormat)
	}
	chunk := int64(head[off+4])<<24 | int64(head[off+5])<<16 | int64(head[off+6])<<8 | int64(head[off+7])
	end := min(size, off+8+chunk)
	data := make([]byte, end)
	if n, err := src.ReadAt(data, 0); int64(n) < end {
		return nil, fmt.Errorf("reading djvu: %w", errors.Join(err, ivy.ErrFormat))
	}
	return data, nil
}

// depth returns the depth of the chunks under c, from 1.
func depth(c *iff.Chunk) int {
	n := 0
	for _, k := range c.Children {
		n = max(n, depth(k))
	}
	return n + 1
}

// format wraps err with ivy.ErrFormat, unless it wraps a sentinel error of
// ivy or the error of a context already.
func format(err error) error {
	switch {
	case err == nil, errors.Is(err, ivy.ErrLimit), errors.Is(err, ivy.ErrFormat),
		errors.Is(err, ivy.ErrUnsupported), errors.Is(err, context.Canceled),
		errors.Is(err, context.DeadlineExceeded):
		return err
	}
	return fmt.Errorf("%w: %w", err, ivy.ErrFormat)
}

// recoverTo turns a panic into an error that wraps ivy.ErrFormat. A fuzz
// test still fails on the panic, so that the cause is fixed.
func recoverTo(what string, err *error) {
	if p := recover(); p != nil {
		*err = fmt.Errorf("%s: panic: %v: %w", what, p, ivy.ErrFormat)
	}
}

// Close satisfies the ivy.Document interface. It forgets the decoded
// layers.
func (d *Document) Close() error {
	d.mu.Lock()
	defer d.mu.Unlock()
	for _, p := range d.recent {
		p.layers.Store(nil)
	}
	d.recent = nil
	return nil
}

// PageCount satisfies the ivy.Pager interface.
func (d *Document) PageCount(context.Context) (int, error) {
	return len(d.pages), nil
}

// Page satisfies the ivy.Pager interface. A page that the document cannot
// read has the size of the first page, and its Render returns an
// *ivy.PartialError.
func (d *Document) Page(_ context.Context, n int) (ivy.Page, error) {
	if n < 0 || n >= len(d.pages) {
		return nil, fmt.Errorf("page %d: %w", n, ivy.ErrPageRange)
	}
	return d.pages[n], nil
}

// touch records that page p decoded its layers, which take size bytes. If
// the decoded pages take more than maxCached pages or Limits.MaxMemory
// bytes, the document forgets the layers of the oldest, which decode again
// on their next render.
func (d *Document) touch(p *Page, size int64) {
	d.mu.Lock()
	defer d.mu.Unlock()
	for i, q := range d.recent {
		if q == p {
			d.recent = append(d.recent[:i], d.recent[i+1:]...)
			break
		}
	}
	p.cached = size
	d.recent = append(d.recent, p)
	var total int64
	for _, q := range d.recent {
		total += q.cached
	}
	for len(d.recent) > 1 && (len(d.recent) > maxCached || total > d.lim.MaxMemory) {
		old := d.recent[0]
		old.layers.Store(nil)
		total -= old.cached
		d.recent = d.recent[1:]
	}
}

// Previews satisfies the ivy.Previewer interface. It lists the thumbnails
// of a bundled document, one for each page, in order. A document without
// thumbnails lists none.
func (d *Document) Previews(context.Context) iter.Seq2[ivy.Preview, error] {
	return func(yield func(ivy.Preview, error) bool) {
		for _, k := range d.doc.Root.Children {
			if k.Name() != "FORM:THUM" {
				continue
			}
			for _, c := range k.All("TH44") {
				w, h, ok := iw44.Size(c.Data)
				if !ok {
					continue
				}
				data := c.Data
				p := ivy.Preview{
					Kind: ivy.Thumbnail,
					Size: image.Pt(w, h),
					Decode: func(ctx context.Context) (image.Image, error) {
						return d.thumbnail(ctx, data)
					},
				}
				if !yield(p, nil) {
					return
				}
			}
		}
	}
}

// thumbnail decodes the IW44 image of a TH44 chunk.
func (d *Document) thumbnail(ctx context.Context, data []byte) (img image.Image, err error) {
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("decoding djvu thumbnail: %w", err)
	}
	defer recoverTo("decoding djvu thumbnail", &err)
	w, h, _ := iw44.Size(data)
	if px := int64(w) * int64(h); px > d.lim.MaxPixels || 9*px > d.lim.MaxMemory {
		return nil, fmt.Errorf("djvu thumbnail of %dx%d: %w", w, h, ivy.ErrLimit)
	}
	var dec iw44.Decoder
	if err := dec.Decode(data); err != nil {
		return nil, fmt.Errorf("decoding djvu thumbnail: %w", format(err))
	}
	im, err := dec.Image()
	if err != nil {
		return nil, fmt.Errorf("decoding djvu thumbnail: %w", format(err))
	}
	return toImage(im), nil
}

// toImage converts a decoded IW44 layer to an image.
func toImage(im *iw44.Image) image.Image {
	if im.Gray {
		return &image.Gray{Pix: im.Pix, Stride: im.W, Rect: image.Rect(0, 0, im.W, im.H)}
	}
	out := image.NewRGBA(image.Rect(0, 0, im.W, im.H))
	for i := range im.W * im.H {
		copy(out.Pix[4*i:4*i+3], im.Pix[3*i:3*i+3])
		out.Pix[4*i+3] = 0xff
	}
	return out
}

// Describe satisfies the ivy.Describer interface. It reads the metadata
// of the shared annotations of a bundled document, and of the first page
// and what it includes: the list (metadata (key "value") ...) of an ANTa or
// an ANTz chunk, or of a METa or a METz chunk.
func (d *Document) Describe(ctx context.Context) (info ivy.Info, err error) {
	if err := ctx.Err(); err != nil {
		return ivy.Info{}, fmt.Errorf("describing djvu: %w", err)
	}
	defer recoverTo("describing djvu", &err)
	var forms []*iff.Chunk
	if d.doc.Dir != nil {
		for _, c := range d.doc.Dir.Components {
			if c.Type != doc.SharedAnno {
				continue
			}
			for _, k := range d.doc.Root.Children {
				if k.Offset == c.Offset {
					forms = append(forms, k)
				}
			}
		}
	}
	if len(d.doc.Pages) > 0 && d.doc.Pages[0].Form != nil {
		forms = append(forms, d.doc.Pages[0].Form)
		forms = append(forms, d.doc.Pages[0].Includes...)
	}
	exprs, err := doc.AnnotationsOf(d.lim, forms...)
	if err != nil {
		d.log.Debug("djvu: damaged annotations", "err", err)
	}
	for _, f := range forms {
		for _, c := range f.Children {
			b := c.Data
			switch c.ID {
			case "METa":
			case "METz":
				if b, err = bzz.DecodeMax(b, int(min(d.lim.MaxMemory, 1<<30))); err != nil {
					d.log.Debug("djvu: damaged metadata", "err", err)
					continue
				}
			default:
				continue
			}
			exprs = append(exprs, doc.ParseSexps(b)...)
		}
	}
	for _, e := range exprs {
		if e.Head() != "metadata" {
			continue
		}
		for _, kv := range e.List[1:] {
			if !kv.IsList || len(kv.List) != 2 || kv.List[1].IsList {
				continue
			}
			setInfo(&info, strings.ToLower(kv.Head()), strings.TrimSpace(kv.List[1].Atom))
		}
	}
	return info, nil
}

// setInfo sets the field of info that the key of a metadata names, if it
// is empty.
func setInfo(info *ivy.Info, key, value string) {
	if value == "" {
		return
	}
	set := func(s *string) {
		if *s == "" {
			*s = value
		}
	}
	switch key {
	case "title":
		set(&info.Title)
	case "author":
		if len(info.Authors) == 0 {
			info.Authors = []string{value}
		}
	case "subject":
		set(&info.Subject)
	case "language", "lang":
		set(&info.Language)
	case "producer", "creator":
		set(&info.Producer)
	case "creationdate":
		if t, ok := parseDate(value); ok && info.Created.IsZero() {
			info.Created = t
		}
	case "moddate":
		if t, ok := parseDate(value); ok && info.Modified.IsZero() {
			info.Modified = t
		}
	}
}

// parseDate parses a date of the metadata, such as "2006-01-02
// 15:04:05-07:00", which djvused writes.
func parseDate(s string) (time.Time, bool) {
	for _, layout := range []string{"2006-01-02 15:04:05Z07:00", time.RFC3339, "2006-01-02 15:04:05", "2006-01-02"} {
		if t, err := time.Parse(layout, s); err == nil {
			return t, true
		}
	}
	return time.Time{}, false
}
