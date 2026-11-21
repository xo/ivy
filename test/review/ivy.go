package review

import (
	"context"
	"errors"
	"fmt"
	"image"
	"image/color"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"runtime/debug"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/xo/ivy"
	"github.com/xo/ivy/camera"
	"github.com/xo/ivy/chm"
	"github.com/xo/ivy/comic"
	"github.com/xo/ivy/csv"
	"github.com/xo/ivy/djvu"
	"github.com/xo/ivy/docx"
	"github.com/xo/ivy/emf"
	"github.com/xo/ivy/epub"
	"github.com/xo/ivy/fb2"
	"github.com/xo/ivy/html"
	"github.com/xo/ivy/iwork"
	"github.com/xo/ivy/latex"
	"github.com/xo/ivy/markdown"
	"github.com/xo/ivy/mobi"
	"github.com/xo/ivy/msdoc"
	"github.com/xo/ivy/msppt"
	"github.com/xo/ivy/msxls"
	"github.com/xo/ivy/odf"
	"github.com/xo/ivy/ora"
	"github.com/xo/ivy/pdf"
	"github.com/xo/ivy/pptx"
	"github.com/xo/ivy/ps"
	"github.com/xo/ivy/psd"
	"github.com/xo/ivy/pub"
	"github.com/xo/ivy/rtf"
	"github.com/xo/ivy/text"
	"github.com/xo/ivy/vsdx"
	"github.com/xo/ivy/xlsx"
	"github.com/xo/ivy/xps"
)

// formats are the formats of ivy. A kind of the manifest names the format
// that lists it as an extension. To add a format, add its Format value
// here.
var formats = []ivy.Format{
	camera.Format, chm.Format, comic.Format, csv.Format, djvu.Format, docx.Format,
	emf.Format, epub.Format, fb2.Format, html.Format, iwork.Format,
	latex.Format, markdown.Format, mobi.Format, msdoc.Format, msppt.Format, msxls.Format,
	odf.Format, ora.Format, pdf.Format, pptx.Format, ps.Format, psd.Format,
	pub.Format, rtf.Format, text.Format, vsdx.Format, xlsx.Format, xps.Format,
}

// kindAliases maps a kind that is not an extension to an extension.
var kindAliases = map[string]string{
	"raw":      "dng",
	"markdown": "md",
}

// formatFor returns the format of ivy that opens the kind.
func formatFor(kind string) (ivy.Format, bool) {
	if alias, ok := kindAliases[kind]; ok {
		kind = alias
	}
	for _, f := range formats {
		if f.Name == kind {
			return f, true
		}
		for _, ext := range f.Exts {
			if ext == kind {
				return f, true
			}
		}
	}
	return ivy.Format{}, false
}

// The status of a document or of a page of ivy.
const (
	StatusOK          = "ok"
	StatusPartial     = "partial"
	StatusError       = "error"
	StatusPanic       = "panic"
	StatusTimeout     = "timeout"
	StatusMissing     = "missing"
	StatusUnsupported = "unsupported"
)

// statusRank orders the statuses from the worst.
var statusRank = map[string]int{
	StatusPanic: 0, StatusTimeout: 1, StatusError: 2, StatusMissing: 3,
	StatusUnsupported: 4, StatusPartial: 5, StatusOK: 6,
}

// worse returns the worse of the statuses a and b.
func worse(a, b string) string {
	if a == "" {
		return b
	}
	if b != "" && statusRank[b] < statusRank[a] {
		return b
	}
	return a
}

// errStuck is the error of a call of ivy that does not return after the
// end of its context.
var errStuck = errors.New("ivy did not return after the end of its context")

// panicError is a panic of ivy.
type panicError struct {
	value any
	stack string
}

func (err *panicError) Error() string {
	return fmt.Sprintf("panic: %v\n\n%s", err.value, err.stack)
}

// stuckGrace is how long a call of ivy can take to return after the end of
// its context.
const stuckGrace = 5 * time.Second

// call runs f in a goroutine, turns a panic into a *panicError, and stops
// waiting for f a short time after the end of ctx, with errStuck. A stuck f
// keeps running, and the caller must not touch what f uses.
func call[T any](ctx context.Context, f func() (T, error)) (T, error) {
	type result struct {
		v   T
		err error
	}
	ch := make(chan result, 1)
	go func() {
		var r result
		defer func() {
			if p := recover(); p != nil {
				r.err = &panicError{value: p, stack: string(debug.Stack())}
			}
			ch <- r
		}()
		r.v, r.err = f()
	}()
	select {
	case r := <-ch:
		return r.v, r.err
	case <-ctx.Done():
	}
	select {
	case r := <-ch:
		return r.v, r.err
	case <-time.After(stuckGrace):
		var zero T
		return zero, errStuck
	}
}

// statusOf returns the status of an error of ivy.
func statusOf(ctx context.Context, err error) string {
	var pe *panicError
	var partial *ivy.PartialError
	switch {
	case err == nil:
		return StatusOK
	case errors.As(err, &pe):
		return StatusPanic
	case errors.Is(err, errStuck), ctx.Err() != nil && errors.Is(err, ctx.Err()), errors.Is(err, context.DeadlineExceeded):
		return StatusTimeout
	case errors.As(err, &partial):
		return StatusPartial
	}
	return StatusError
}

// logCollector is a slog.Handler that keeps the distinct messages of the
// log, with their counts.
type logCollector struct {
	mu     *sync.Mutex
	counts map[string]int
	order  *[]string
	attrs  string
}

// maxLogLines bounds the distinct messages that a logCollector keeps.
const maxLogLines = 300

func newLogCollector() *logCollector {
	return &logCollector{mu: &sync.Mutex{}, counts: map[string]int{}, order: new([]string)}
}

func (h *logCollector) Enabled(context.Context, slog.Level) bool { return true }

func (h *logCollector) Handle(_ context.Context, r slog.Record) error {
	var b strings.Builder
	b.WriteString(r.Level.String())
	b.WriteString(" ")
	b.WriteString(r.Message)
	b.WriteString(h.attrs)
	r.Attrs(func(a slog.Attr) bool {
		fmt.Fprintf(&b, " %s=%v", a.Key, a.Value)
		return true
	})
	line := b.String()
	h.mu.Lock()
	defer h.mu.Unlock()
	if _, ok := h.counts[line]; !ok {
		if len(*h.order) >= maxLogLines {
			return nil
		}
		*h.order = append(*h.order, line)
	}
	h.counts[line]++
	return nil
}

func (h *logCollector) WithAttrs(attrs []slog.Attr) slog.Handler {
	var b strings.Builder
	b.WriteString(h.attrs)
	for _, a := range attrs {
		fmt.Fprintf(&b, " %s=%v", a.Key, a.Value)
	}
	return &logCollector{mu: h.mu, counts: h.counts, order: h.order, attrs: b.String()}
}

func (h *logCollector) WithGroup(string) slog.Handler { return h }

// lines returns the messages, the warnings and errors first, each with its
// count when it is more than 1.
func (h *logCollector) lines() []string {
	h.mu.Lock()
	defer h.mu.Unlock()
	out := make([]string, 0, len(*h.order))
	for _, line := range *h.order {
		if n := h.counts[line]; n > 1 {
			line = fmt.Sprintf("%s (%d times)", line, n)
		}
		out = append(out, line)
	}
	rank := func(s string) int {
		switch {
		case strings.HasPrefix(s, "ERROR"):
			return 0
		case strings.HasPrefix(s, "WARN"):
			return 1
		case strings.HasPrefix(s, "INFO"):
			return 2
		}
		return 3
	}
	sort.SliceStable(out, func(i, j int) bool { return rank(out[i]) < rank(out[j]) })
	return out
}

// ivyDoc is a document that ivy opened.
type ivyDoc struct {
	doc   ivy.Document
	pager ivy.Pager
	file  *os.File
	log   *logCollector
	// pages is the number of pages, or 0 for a document without pages.
	pages int
}

// openIvy opens the file at path with the format of kind, with the
// configuration cfg and a logger of its own. It returns the document, and
// the time to open it and to count its pages.
func openIvy(ctx context.Context, path, kind string, cfg ivy.Config) (*ivyDoc, time.Duration, error) {
	format, ok := formatFor(kind)
	if !ok {
		return nil, 0, fmt.Errorf("ivy has no format for the kind %q", kind)
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, 0, fmt.Errorf("opening the file: %w", err)
	}
	st, err := f.Stat()
	if err != nil {
		_ = f.Close()
		return nil, 0, fmt.Errorf("reading the size of the file: %w", err)
	}
	d := &ivyDoc{file: f, log: newLogCollector()}
	cfg.Logger = slog.New(d.log)
	start := time.Now()
	doc, err := call(ctx, func() (ivy.Document, error) {
		src := io.NewSectionReader(f, 0, st.Size())
		return OpenIvy(ctx, format, src, cfg, filepath.Dir(path))
	})
	if err != nil {
		if !errors.Is(err, errStuck) {
			_ = f.Close()
		}
		return d, time.Since(start), fmt.Errorf("opening with %s: %w", format.Name, err)
	}
	d.doc = doc
	if pager, ok := doc.(ivy.Pager); ok {
		d.pager = pager
		n, err := call(ctx, func() (int, error) { return pager.PageCount(ctx) })
		if err != nil {
			return d, time.Since(start), fmt.Errorf("counting the pages: %w", err)
		}
		d.pages = n
	}
	return d, time.Since(start), nil
}

// render renders page n of d at dpi over white. A document without pages
// renders its best preview, as page 0.
func (d *ivyDoc) render(ctx context.Context, n int, dpi float32) (image.Image, error) {
	return call(ctx, func() (image.Image, error) {
		if d.pager == nil {
			if n != 0 {
				return nil, ivy.ErrPageRange
			}
			return ivy.BestPreview(ctx, d.doc, image.Point{})
		}
		page, err := d.pager.Page(ctx, n)
		if err != nil {
			return nil, fmt.Errorf("getting the page: %w", err)
		}
		img, err := ivy.RenderImage(ctx, page, dpi, color.White)
		if err != nil {
			return img, fmt.Errorf("rendering: %w", err)
		}
		return img, nil
	})
}

// close closes the document and its file.
func (d *ivyDoc) close() {
	if d.doc != nil {
		_ = d.doc.Close()
	}
	if d.file != nil {
		_ = d.file.Close()
	}
}

// ivyMediaBox renders page n, from 0, of the PDF file at path at its
// MediaBox, at dpi over white, with the configuration cfg of the row of the
// CropBox. It opens the file with pdf.Open and the option
// pdf.WithPageBox(pdf.MediaBox) of D31.
func ivyMediaBox(ctx context.Context, path string, cfg ivy.Config, n int, dpi float32) (image.Image, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("opening the file: %w", err)
	}
	st, err := f.Stat()
	if err != nil {
		_ = f.Close()
		return nil, fmt.Errorf("reading the size of the file: %w", err)
	}
	img, err := call(ctx, func() (image.Image, error) {
		cfg.Logger = slog.New(slog.DiscardHandler)
		doc, err := pdf.Open(ctx, io.NewSectionReader(f, 0, st.Size()), pdf.WithConfig(cfg), pdf.WithPageBox(pdf.MediaBox))
		if err != nil {
			return nil, fmt.Errorf("opening with pdf: %w", err)
		}
		defer func() { _ = doc.Close() }()
		page, err := doc.Page(ctx, n)
		if err != nil {
			return nil, fmt.Errorf("getting the page: %w", err)
		}
		img, err := ivy.RenderImage(ctx, page, dpi, color.White)
		if err != nil {
			return img, fmt.Errorf("rendering: %w", err)
		}
		return img, nil
	})
	// a stuck call keeps the file, as openIvy does
	if !errors.Is(err, errStuck) {
		_ = f.Close()
	}
	return img, err
}

// FormatFor returns the format of ivy that opens the kind of a document of
// the manifest.
func FormatFor(kind string) (ivy.Format, bool) {
	return formatFor(kind)
}

// OpenIvy opens src with format and the configuration cfg, as render does.
// An HTML or Markdown file reads the files that it links to from the
// folder dir (D22), and a TeX document reads its figures, its inputs and
// its bibliography from it (D65). The manifest fetches them as assets.
func OpenIvy(ctx context.Context, format ivy.Format, src ivy.Source, cfg ivy.Config, dir string) (ivy.Document, error) {
	switch format.Name {
	case html.Format.Name:
		return html.Open(ctx, src, html.WithConfig(cfg), html.WithDir(os.DirFS(dir)))
	case markdown.Format.Name:
		return markdown.Open(ctx, src, markdown.WithConfig(cfg), markdown.WithDir(os.DirFS(dir)))
	case latex.Format.Name:
		return latex.Open(ctx, src, latex.WithConfig(cfg), latex.WithDir(os.DirFS(dir)))
	}
	return format.Open(ctx, src, cfg)
}
