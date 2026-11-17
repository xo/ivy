// Package msppt opens the presentations of PowerPoint 97 to PowerPoint
// 2003: .ppt files, .pps shows and .pot templates (MS-PPT).
//
// A presentation is a Compound File, which the package cfb of magic reads.
// The package reads the stream "PowerPoint Document" through the stream
// "Current User", the chain of the edits and the persist directory, to the
// slides in their order. Each slide is a page of the size of the slides,
// which the package draws on the display list of D10 with
// internal/drawingml.
//
// A slide draws its background, the shapes of its master, and its own
// shapes: the preset shapes, the shapes of a custom geometry, the lines,
// the groups and the pictures, with their fills, their outlines and their
// shadows. The text of a shape has the fonts, the sizes, the colors and
// the bullets of its runs and paragraphs, over the styles of the master.
// The colors come from the color scheme of the slide or of its master. The
// placeholders of the date, the slide number and the footer of a master
// show as the HeadersFootersAtom of the slide or of the presentation says,
// except a date of today. internal/odraw reads the shapes and the pictures
// of Office Drawing, which msxls reads too.
//
// The package leaves out the notes, the charts and the objects of OLE
// other than their pictures, the animations, the comments, the text of
// WordArt and the pictures in PICT. The pictures of a presentation that a
// password protects are left out.
//
// A presentation gives its thumbnail and its properties from the stream
// "\x05SummaryInformation". A presentation that a password protects with
// RC4 CryptoAPI asks for the password as D12 says. The package returns an
// error that wraps ivy.ErrUnsupported for a presentation of PowerPoint 95
// or of PowerPoint 4, whose format MS-PPT does not describe.
package msppt

import (
	"context"
	"fmt"
	"iter"
	"sync"

	"github.com/xo/ivy"
	"github.com/xo/ivy/internal/ebook"
	"github.com/xo/ivy/internal/msole"
	"github.com/xo/ivy/internal/odraw"
)

// Format describes the package for the registry of a program.
var Format = ivy.Format{
	Name:  "msppt",
	Mimes: []string{"application/vnd.ms-powerpoint", "application/mspowerpoint", "application/x-mspowerpoint"},
	Exts:  []string{"ppt", "pps", "pot"},
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

// Document is an open PowerPoint presentation.
type Document struct {
	cfg     ivy.Config
	summary msole.Summary
	stream  *stream
	doc     *document
	// picStream is the stream "Pictures", which holds the pictures of the
	// store.
	picStream []byte

	// mu guards the caches, which the pages fill as they are built.
	mu          sync.Mutex
	masterCache map[uint32]*master
	pics        *odraw.Pictures
	// ctx is the context of the page that is being built.
	ctx context.Context
}

// The capabilities of a document.
var (
	_ ivy.Pager     = (*Document)(nil)
	_ ivy.Previewer = (*Document)(nil)
	_ ivy.Describer = (*Document)(nil)
)

// Open opens a PowerPoint presentation. It reads the summary information,
// the persist directory and the list of the slides, and asks for the
// password if one protects the presentation. It returns an error that wraps
// ivy.ErrFormat if src is not a presentation, and one that wraps
// ivy.ErrPassword if the password is missing or wrong.
func Open(ctx context.Context, src ivy.Source, opts ...Option) (d *Document, err error) {
	var o options
	for _, opt := range opts {
		opt(&o)
	}
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("opening powerpoint file: %w", err)
	}
	defer ebook.Recover("opening powerpoint file", &err)
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
	s, err := readStream(ctx, f, cfg)
	if err != nil {
		return nil, err
	}
	doc, ok := readDocument(s)
	if !ok {
		return nil, fmt.Errorf("powerpoint file without a document container: %w", ivy.ErrFormat)
	}
	d = &Document{
		cfg:         cfg,
		summary:     summary,
		stream:      s,
		doc:         doc,
		masterCache: make(map[uint32]*master),
	}
	if msole.Has(f, "Pictures") {
		// a presentation whose pictures are too large shows its slides
		// without them
		d.picStream, _ = msole.ReadStream(f, "Pictures", cfg.Limits)
	}
	d.pics = odraw.NewPictures(doc.store, d.picStream, cfg)
	return d, nil
}

// Close satisfies the ivy.Document interface.
func (d *Document) Close() error {
	return nil
}

// Previews satisfies the ivy.Previewer interface. It lists the thumbnail of
// the summary information: PowerPoint writes one of the first slide.
func (d *Document) Previews(ctx context.Context) iter.Seq2[ivy.Preview, error] {
	return d.summary.Previews(ctx, d.cfg)
}

// Describe satisfies the ivy.Describer interface. It returns the
// properties of the summary information.
func (d *Document) Describe(ctx context.Context) (ivy.Info, error) {
	if err := ctx.Err(); err != nil {
		return ivy.Info{}, fmt.Errorf("describing powerpoint file: %w", err)
	}
	return d.summary.Info, nil
}

// PageCount satisfies the ivy.Pager interface. Each slide is a page, and a
// presentation without a slide has one empty page.
func (d *Document) PageCount(ctx context.Context) (int, error) {
	if err := ctx.Err(); err != nil {
		return 0, fmt.Errorf("counting slides: %w", err)
	}
	return max(1, len(d.doc.slides)), nil
}

// Page satisfies the ivy.Pager interface. It reads the slide n, from 0,
// and its master.
func (d *Document) Page(ctx context.Context, n int) (p ivy.Page, err error) {
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("reading slide %d: %w", n+1, err)
	}
	if n < 0 || n >= max(1, len(d.doc.slides)) {
		return nil, fmt.Errorf("slide %d of %d: %w", n+1, len(d.doc.slides), ivy.ErrPageRange)
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	defer ebook.Recover("reading powerpoint slide", &err)
	d.ctx = ctx
	defer func() {
		d.ctx = nil
	}()
	pg, err := d.slidePage(ctx, n)
	if err != nil {
		return nil, err
	}
	return pg, nil
}
