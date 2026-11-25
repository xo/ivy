// Package latex opens a LaTeX or a plain TeX document and puts it on pages
// (D62). It runs a copy of the TeX engine of github.com/go-tex/engine, in
// latex/internal/engine, and draws its pages onto the display list of ivy
// (D10).
//
// A document reads other files by their names: its class, its packages,
// the files of \input and its figures. The package reads them from two
// places. WithDir gives the folder of the document, and the field TeX of
// ivy.Config gives a tree of TeX files, such as the classes and the
// packages of a TeX distribution. The module of ivy holds no file of LaTeX,
// so a document whose class neither place holds is set without it. The
// package reads no file of the disk and no variable of the environment,
// because a document is input from a stranger (D5).
//
// The engine sets the text in Latin Modern and the math in Latin Modern
// Math, as TeX does, when the field Fonts or the field TeX of ivy.Config
// holds them, such as fonts.TeX of the module github.com/xo/ivy/fonts or
// the tree of a TeX distribution. Without them, it sets the text in the Go
// fonts and the math in STIX Two Math.
//
// The engine runs the whole document when a program first asks for its
// pages, because TeX numbers the pages and resolves the references of the
// whole document. The context stops the run. A command that the engine does
// not know is skipped, and the page is drawn without it.
package latex

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"strings"
	"sync"
	"unicode/utf8"

	"github.com/xo/okage"

	"github.com/xo/ivy"
	"github.com/xo/ivy/internal/display"
	"github.com/xo/ivy/latex/internal/engine"
)

// Format describes the package for the registry of a program.
var Format = ivy.Format{
	Name:  "latex",
	Mimes: []string{"text/x-tex", "application/x-tex", "text/x-latex", "application/x-latex"},
	Exts:  []string{"tex", "latex", "ltx"},
	Open: func(ctx context.Context, src ivy.Source, cfg ivy.Config) (ivy.Document, error) {
		return Open(ctx, src, WithConfig(cfg))
	},
}

// Option is an option of Open.
type Option func(*options)

type options struct {
	cfg  ivy.Config
	dir  fs.FS
	name string
}

// WithConfig sets the configuration that every format shares.
func WithConfig(cfg ivy.Config) Option {
	return func(o *options) {
		o.cfg = cfg
	}
}

// WithDir gives the folder of the document. The document reads the files
// that it names, such as \input{chapter1} and \includegraphics{fig}, from
// dir, by their paths relative to the folder. dir must be safe for
// concurrent use. Without it, the document reads only the files of the
// field TeX of ivy.Config.
func WithDir(dir fs.FS) Option {
	return func(o *options) {
		o.dir = dir
	}
}

// WithName gives the job name of the document: the name of its input file
// without the extension, such as "report" for report.tex. The engine reads
// the files of the run that carry the job name, such as report.bbl, from the
// folder of WithDir. Without it, the job name is "texput", as in TeX.
func WithName(name string) Option {
	return func(o *options) {
		o.name = name
	}
}

// Document is an open LaTeX document.
type Document struct {
	src  []byte
	opts options
	log  *slog.Logger

	// mu guards the run. A run that its context stops can run again, and
	// the error of another run is kept.
	mu   sync.Mutex
	doc  *engine.Doc
	err  error
	figs *figures
}

// The capabilities of a document.
var (
	_ ivy.Pager     = (*Document)(nil)
	_ ivy.Describer = (*Document)(nil)
)

// Open opens a TeX document. It reads the whole source into memory. It
// returns an error that wraps ivy.ErrFormat if src is not text, and one
// that wraps ivy.ErrLimit if src is larger than the memory of the limits.
// The engine runs later, when a program asks for the pages.
func Open(ctx context.Context, src ivy.Source, opts ...Option) (*Document, error) {
	var o options
	for _, opt := range opts {
		opt(&o)
	}
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("opening latex: %w", err)
	}
	lim := o.cfg.Limits.WithDefaults()
	size := src.Size()
	if size <= 0 {
		return nil, fmt.Errorf("latex of %d bytes: %w", size, ivy.ErrFormat)
	}
	if size > lim.MaxMemory {
		return nil, fmt.Errorf("latex of %d bytes: %w", size, ivy.ErrLimit)
	}
	data := make([]byte, size)
	if _, err := src.ReadAt(data, 0); err != nil && !errors.Is(err, io.EOF) {
		return nil, fmt.Errorf("reading latex: %w", err)
	}
	if !isText(data) {
		return nil, fmt.Errorf("latex: not text: %w", ivy.ErrFormat)
	}
	return &Document{src: data, opts: o, log: o.cfg.Log()}, nil
}

// isText reports whether data is text: valid UTF-8 without NUL. A TeX
// source in Latin-1 is also text, so the test of UTF-8 reads only the
// first block.
func isText(data []byte) bool {
	head := data[:min(len(data), 4096)]
	if bytes.IndexByte(head, 0) >= 0 {
		return false
	}
	for len(head) > 0 && !utf8.FullRune(head) {
		head = head[:len(head)-1]
	}
	if utf8.Valid(head) {
		return true
	}
	// A legacy 8-bit source has no control characters but the spaces.
	for _, c := range head {
		if c < 0x20 && c != '\t' && c != '\n' && c != '\r' && c != '\f' {
			return false
		}
	}
	return true
}

// jobName returns the job name that the engine takes for the name of
// WithName. A name that is empty, or that holds a slash, is no name, and
// the engine then takes "texput" (D66).
func jobName(name string) string {
	if strings.Contains(name, "/") {
		return ""
	}
	return name
}

// run runs the engine once, and keeps its pages.
func (d *Document) run(ctx context.Context) (*engine.Doc, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.doc != nil || d.err != nil {
		return d.doc, d.err
	}
	cfg := d.opts.cfg
	lim := cfg.Limits.WithDefaults()
	faces := loadFaces(cfg.Fonts, cfg.TeX, lim)
	res := newResolver(d.opts.dir, cfg.TeX, lim)
	doc, err := engine.Compile(ctx, d.src, engine.RunOptions{
		Faces:    faces,
		Resolve:  res.resolve,
		JobName:  jobName(d.opts.name),
		MaxPages: lim.MaxObjects,
	})
	if err != nil {
		// a run that its context stopped can run again
		if ctx.Err() == nil {
			d.err = err
		}
		return nil, err
	}
	if doc.PageCount() == 0 {
		d.err = fmt.Errorf("latex: no pages: %w", ivy.ErrFormat)
		return nil, d.err
	}
	if n := doc.Skipped(); n > 0 {
		d.log.Debug("latex: commands skipped", "count", n)
	}
	d.doc = doc
	d.figs = newFigures(cfg)
	return doc, nil
}

// Close satisfies the ivy.Document interface.
func (d *Document) Close() error {
	return nil
}

// PageCount satisfies the ivy.Pager interface. It runs the engine.
func (d *Document) PageCount(ctx context.Context) (int, error) {
	doc, err := d.run(ctx)
	if err != nil {
		return 0, err
	}
	return doc.PageCount(), nil
}

// Page satisfies the ivy.Pager interface. It runs the engine.
func (d *Document) Page(ctx context.Context, n int) (ivy.Page, error) {
	doc, err := d.run(ctx)
	if err != nil {
		return nil, err
	}
	if n < 0 || n >= doc.PageCount() {
		return nil, fmt.Errorf("page %d of %d: %w", n, doc.PageCount(), ivy.ErrPageRange)
	}
	w, h := doc.PageSize(n)
	return &Page{doc: doc, n: n, w: w, h: h, figs: d.figs, log: d.log}, nil
}

// Describe satisfies the ivy.Describer interface. It gives the title and
// the authors of \hypersetup, or of \title and \author. It runs the engine.
func (d *Document) Describe(ctx context.Context) (ivy.Info, error) {
	doc, err := d.run(ctx)
	if err != nil {
		return ivy.Info{}, err
	}
	info := doc.Info()
	out := ivy.Info{Title: info.Title, Subject: info.Subject}
	if info.Author != "" {
		out.Authors = []string{info.Author}
	}
	return out, nil
}

// Page is a page of a LaTeX document.
type Page struct {
	doc  *engine.Doc
	n    int
	w, h float32
	figs *figures
	log  *slog.Logger
}

// The interfaces that a Page implements.
var (
	_ ivy.Page       = (*Page)(nil)
	_ display.Drawer = (*Page)(nil)
)

// Size satisfies the ivy.Page interface. It is the paper of the class, or
// US Letter for a document of plain TeX, as in pdfTeX. A LaTeX document
// that names no paper and has no class takes the text of the page with a
// margin of one inch.
func (p *Page) Size() (width, height float32) {
	return p.w, p.h
}

// Render satisfies the ivy.Page interface.
func (p *Page) Render(ctx context.Context, dst *okage.Pixmap, t okage.Transform) error {
	return p.Draw(ctx, display.NewOkage(dst, t), 1)
}

// Draw satisfies the display.Drawer interface (D58). A figure that fails
// to draw is left out, and Draw returns an *ivy.PartialError.
func (p *Page) Draw(ctx context.Context, d display.Device, res float32) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := p.doc.DrawPage(ctx, d, p.n, p.figs, res); err != nil {
		if cerr := ctx.Err(); cerr != nil {
			return cerr
		}
		p.log.Debug("latex: page drawn in part", "page", p.n, "err", err)
		return &ivy.PartialError{Err: err}
	}
	return nil
}
