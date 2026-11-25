// Package xlsx opens Excel files in Office Open XML: .xlsx, .xltx, .xlsm and
// .xltm.
//
// A document gives the thumbnail that the file can hold, in
// docProps/thumbnail.* (D11), and the pages of its sheets, as printing does
// (D24): each sheet that is not hidden, in order, with the cells that hold
// data or a visible style, or with its print area. A sheet that is wider
// than its page is cut into bands of columns, down then over, or over then
// down when the page setup asks for it. The package converts the sheets
// into HTML tables for the layout of D10, with the package sheet.
//
// The cells show the values that the file holds, formatted with their
// number formats. No formula is computed: a formula shows the value that
// the program that wrote the file computed last. The package reads the
// fonts, the fills, the borders and the alignments of the cells, the
// colors of the theme, the merged cells, the widths of the columns and the
// heights of the rows, the hidden rows and columns, the page setup with the
// scale and the fit to pages, the print areas and the print titles, the
// manual page breaks, the headers and the footers, and the gridlines that
// printOptions asks for. As Excel prints, a page has no gridlines unless
// the sheet asks for them. A sheet without a header shows its name in
// small grey text at the top left of each page. The widths of the columns
// and the heights of the rows are those that Excel prints, and a row keeps
// its height as in Excel.
//
// The pictures, the charts and the shapes of the drawings show at their
// cells, and a chart sheet shows its chart on a page of its own.
// internal/chart draws the charts (D27), and internal/drawingml the shapes,
// with the colors of the theme and the fonts of ivy.Config. A chart that
// cannot be read shows as a frame with its title. A shape that does not
// print is left out.
//
// The conditional formats change the look of the cells, as Excel shows
// them: the highlights of the cells, the color scales, the data bars and
// the icon sets. A rule with a formula of its own needs the formulas, and
// is left out. The comments of the cells show on pages after the cells
// when the page setup asks for it. The sparklines are left out.
//
// A document describes the core properties of the file: its title, its
// creator and its dates.
//
// A workbook reads at most Limits.MaxObjects cells, and a sheet shows at
// most Limits.MaxObjects / 4 cells and rows. A sheet that reaches a limit
// stops there, and a line after its last row says so.
//
// A file that a password protects opens with the password of ivy.Config
// (D12).
package xlsx

import (
	"context"
	"fmt"
	"iter"
	"sync"

	"github.com/xo/ivy"
	"github.com/xo/ivy/internal/ebook"
	"github.com/xo/ivy/internal/layout"
	"github.com/xo/ivy/internal/ooxml"
	"github.com/xo/ivy/internal/sheet"
)

// Format describes the package for the registry of a program.
var Format = ivy.Format{
	Name: "xlsx",
	Mimes: []string{
		"application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",
		"application/vnd.openxmlformats-officedocument.spreadsheetml.template",
		"application/vnd.ms-excel.sheet.macroEnabled.12",
		"application/vnd.ms-excel.template.macroEnabled.12",
	},
	Exts: []string{"xlsx", "xltx", "xlsm", "xltm"},
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

// Document is an open Excel file.
type Document struct {
	doc *ooxml.Document
	cfg ivy.Config
	lim ivy.Limits

	mu     sync.Mutex
	src    *layout.Source
	srcErr error
	// pages holds the book of the layout.
	pages ebook.Pages
}

// The capabilities of a document.
var (
	_ ivy.Previewer = (*Document)(nil)
	_ ivy.Pager     = (*Document)(nil)
	_ ivy.Describer = (*Document)(nil)
)

// Open opens an Excel file. It returns an error that wraps ivy.ErrFormat
// if src is not an Office Open XML package, and ivy.ErrPassword if a
// password protects it and the configuration does not give the right one.
func Open(ctx context.Context, src ivy.Source, opts ...Option) (d *Document, err error) {
	var o options
	for _, opt := range opts {
		opt(&o)
	}
	defer ebook.Recover("opening xlsx", &err)
	doc, err := ooxml.Open(ctx, src, o.cfg)
	if err != nil {
		return nil, err
	}
	return &Document{doc: doc, cfg: o.cfg, lim: o.cfg.Limits.WithDefaults()}, nil
}

// Close satisfies the ivy.Document interface.
func (d *Document) Close() error {
	return nil
}

// Previews satisfies the ivy.Previewer interface. It lists the thumbnail.
func (d *Document) Previews(ctx context.Context) iter.Seq2[ivy.Preview, error] {
	return d.doc.Previews(ctx)
}

// Describe satisfies the ivy.Describer interface. It reads the core
// properties of the file, in docProps/core.xml, and the program that made
// it, in docProps/app.xml.
func (d *Document) Describe(ctx context.Context) (info ivy.Info, err error) {
	defer ebook.Recover("describing xlsx", &err)
	return d.doc.Describe(ctx)
}

// PageCount satisfies the ivy.Pager interface. It lays out every sheet on
// its first call.
func (d *Document) PageCount(ctx context.Context) (int, error) {
	return d.pages.PageCount(ctx, d.source, d.cfg)
}

// Page satisfies the ivy.Pager interface. It lays out the sheets only as
// far as page n.
func (d *Document) Page(ctx context.Context, n int) (ivy.Page, error) {
	return d.pages.Page(ctx, d.source, d.cfg, n)
}

// source converts the workbook into HTML for the layout, once.
func (d *Document) source(ctx context.Context) (src layout.Source, err error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	switch {
	case d.srcErr != nil:
		return layout.Source{}, d.srcErr
	case d.src != nil:
		return *d.src, nil
	}
	defer func() {
		switch {
		case err == nil:
			d.src = &src
		case ctx.Err() == nil:
			// an error of the context is not kept, so that a later call can
			// convert the workbook
			d.srcErr = err
		}
	}()
	defer ebook.Recover("converting xlsx", &err)
	wb, err := d.workbook(ctx)
	if err != nil {
		return layout.Source{}, err
	}
	return wb.Source(ctx, sheet.Options{Limits: d.lim, Logger: d.cfg.Logger, Paper: d.cfg.Paper})
}

// workbook reads the workbook: the sheets that are not hidden, in order.
func (d *Document) workbook(ctx context.Context) (*sheet.Workbook, error) {
	b := &book{ctx: ctx, doc: d.doc, cfg: d.cfg, lim: d.lim, files: make(map[string][]byte)}
	refs, err := b.readWorkbook()
	if err != nil {
		return nil, err
	}
	b.readTheme()
	b.styles = newStyles(b.theme)
	if part := b.relOfType("styles"); part != "" {
		if doc, err := b.parsePart(part); err == nil {
			b.styles.read(doc)
		} else if ctx.Err() != nil {
			return nil, err
		}
	}
	if len(b.styles.cells) == 0 {
		b.styles.cells = []sheet.Style{{Font: b.styles.defaultFont()}}
		b.styles.formats = append(b.styles.formats, b.styles.format(-1))
	}
	if err := b.readStrings(); err != nil {
		return nil, err
	}
	wb := &sheet.Workbook{Styles: b.styles.cells, Files: b.files, Excel: true}
	log := d.cfg.Log()
	for i, ref := range refs {
		if ref.state == "hidden" || ref.state == "veryHidden" || ref.part == "" {
			continue
		}
		sh := &sheet.Sheet{Name: ref.name}
		if b.sheets == nil {
			b.sheets = map[string]*sheet.Sheet{}
		}
		b.sheets[ref.name] = sh
		if ref.chart {
			b.readChartSheet(sh, ref.part)
		} else if err := b.readWorksheet(sh, ref.part, i); err != nil {
			if ctx.Err() != nil {
				return nil, fmt.Errorf("reading sheet %q: %w", ref.name, ctx.Err())
			}
			log.Warn("xlsx: sheet left out", "sheet", ref.name, "err", err)
			sh = &sheet.Sheet{Name: ref.name, Page: sheet.DefaultPage}
			sh.Page.SetPaper(d.cfg.Paper)
			sh.Rows = []sheet.Row{{Cells: []sheet.Cell{{Text: fmt.Sprintf("ivy cannot show this sheet: %v", err)}}}}
		}
		wb.Sheets = append(wb.Sheets, sh)
	}
	// the pivot tables add styles
	wb.Styles = b.styles.cells
	wb.Objects = b.chartObjects
	return wb, nil
}

// readChartSheet reads a chart sheet: one chart on a page of its own.
func (b *book) readChartSheet(sh *sheet.Sheet, part string) {
	sh.Page = sheet.Page{
		Width: sheet.LetterWidth, Height: sheet.LetterHeight, Landscape: true,
		Left: 0.7 * 72, Right: 0.7 * 72, Top: 0.75 * 72, Bottom: 0.75 * 72,
		Header: 0.3 * 72, Footer: 0.3 * 72,
	}
	// The paper of the program replaces the default paperSize 1, US Letter,
	// in its orientation (D32).
	sh.Page.SetPaper(b.cfg.Paper)
	doc, err := b.parsePart(part)
	if err == nil {
		for n := range doc.RootElement().Children() {
			switch local(n) {
			case "pageMargins":
				p := &sh.Page
				p.Left, p.Right = margin(n, "left", p.Left), margin(n, "right", p.Right)
				p.Top, p.Bottom = margin(n, "top", p.Top), margin(n, "bottom", p.Bottom)
			case "pageSetup":
				own := false
				if w, h, ok := sheet.PaperSize(integer(n, "paperSize", 0)); ok {
					sh.Page.Width, sh.Page.Height, own = w, h, true
				}
				// a chart sheet of Excel is in landscape by default
				sh.Page.Landscape = orientation(attr(n, "orientation"), sh.Page.Landscape || own)
			case "drawing":
				if r, ok := b.sheetRel(part, attr(n, "id")); ok {
					b.readDrawing(sh, r)
				}
			}
		}
	}
	// The chart fills the page: one cell of the size of the page holds it.
	w, h := sh.Page.Width, sh.Page.Height
	if sh.Page.Landscape {
		w, h = h, w
	}
	w -= sh.Page.Left + sh.Page.Right + 1
	h -= sh.Page.Top + sh.Page.Bottom + 1
	sh.DefaultColWidth, sh.DefaultRowHeight = max(72, w), max(72, h)
	img := sheet.Image{Width: max(72, w-2), Height: max(72, h-2), Label: "Chart"}
	if len(sh.Images) > 0 {
		img.Object, img.Label = sh.Images[0].Object, sh.Images[0].Label
	}
	if img.Object != "" {
		// the chart lays out at the size of the page
		if obj := b.chartObject(b.chartParts[img.Object], img.Width, img.Height); obj != nil {
			b.chartObjects[img.Object] = obj
		}
	}
	sh.Images = []sheet.Image{img}
}
