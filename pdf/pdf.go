// Package pdf opens PDF files, and renders their pages.
//
// The package reads a file with a port of hayro-syntax, runs the content of
// each page with a port of hayro-interpret, and draws its marks with a port
// of hayro onto the display list of ivy (D10, D14). It draws paths, text
// with the fonts of the file or the standard fonts, images, shadings, tiling
// patterns, soft masks, blend modes and annotations that have an appearance.
// A file that a password protects opens with the password of ivy.Config
// (D12).
//
// A page that the package draws only in part returns an *ivy.PartialError
// (D5).
//
// Adobe Illustrator writes a PDF page next to its own data in a file that is
// compatible with PDF. The package opens such a file, with the extension
// ai, as a PDF file.
package pdf

import (
	"context"
	"errors"
	"fmt"
	"image"
	"iter"
	"log/slog"
	"math"
	"time"
	"unicode/utf16"
	"unicode/utf8"

	"github.com/xo/okage"
	"github.com/xo/okage/kurbo"

	"github.com/xo/ivy"
	"github.com/xo/ivy/internal/display"
	"github.com/xo/ivy/internal/hayro/interpret"
	"github.com/xo/ivy/internal/hayro/render"
	"github.com/xo/ivy/internal/hayro/syntax"
)

// Format describes the package for the registry of a program. It lists the
// extension ai and the mime type of Illustrator, because Illustrator writes
// files that are compatible with PDF.
var Format = ivy.Format{
	Name:  "pdf",
	Mimes: []string{"application/pdf", "application/illustrator"},
	Exts:  []string{"pdf", "ai"},
	Open: func(ctx context.Context, src ivy.Source, cfg ivy.Config) (ivy.Document, error) {
		return Open(ctx, src, WithConfig(cfg))
	},
}

// Option is an option of Open.
type Option func(*options)

type options struct {
	cfg         ivy.Config
	annotations bool
	box         Box
}

// WithConfig sets the configuration that every format shares.
func WithConfig(cfg ivy.Config) Option {
	return func(o *options) {
		o.cfg = cfg
	}
}

// WithAnnotations draws the annotations of each page. It is on by default.
// The package draws the normal appearance of an annotation. An annotation
// without an appearance stream does not draw.
func WithAnnotations(on bool) Option {
	return func(o *options) {
		o.annotations = on
	}
}

// Box is a page box of a PDF page (ISO 32000-1, 14.11.2). The zero Box is
// CropBox.
type Box int

// The page boxes.
const (
	// CropBox is the region that a viewer shows. A page without a CropBox
	// uses its MediaBox.
	CropBox Box = iota
	// MediaBox is the whole medium of the page, with the margin of a
	// printer, such as the crop marks.
	MediaBox
	// BleedBox is the region that a printer prints, with the bleed. A page
	// without a BleedBox uses its CropBox.
	BleedBox
	// TrimBox is the size of the finished page after the trim. A page
	// without a TrimBox uses its CropBox.
	TrimBox
	// ArtBox is the region of the meaningful content of the page. A page
	// without an ArtBox uses its CropBox.
	ArtBox
)

// WithPageBox sets the page box that each page renders, its size and its
// clip (D31). The default is CropBox. A page box that a page does not have
// takes its default from the PDF specification, and a page box is cut to
// the MediaBox. The rotation of the page applies to each page box.
func WithPageBox(box Box) Option {
	return func(o *options) {
		o.box = box
	}
}

// Document is an open PDF file.
type Document struct {
	pdf         *syntax.Pdf
	cache       *render.RenderCache
	fonts       *fontResolver
	lim         ivy.Limits
	log         *slog.Logger
	annotations bool
	pages       []*syntax.Page
}

// The capabilities of a document.
var (
	_ ivy.Pager     = (*Document)(nil)
	_ ivy.Previewer = (*Document)(nil)
	_ ivy.Describer = (*Document)(nil)
)

// Open opens a PDF file. It reads the whole file into memory. It returns an
// error that wraps ivy.ErrFormat if src is not a PDF file, one that wraps
// ivy.ErrLimit if src is larger than Limits.MaxMemory, and one that wraps
// ivy.ErrPassword if the file needs a password that the configuration does
// not give.
func Open(ctx context.Context, src ivy.Source, opts ...Option) (d *Document, err error) {
	o := options{annotations: true}
	for _, opt := range opts {
		opt(&o)
	}
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("opening pdf: %w", err)
	}
	defer func() {
		if p := recover(); p != nil {
			d, err = nil, fmt.Errorf("opening pdf: panic: %v: %w", p, ivy.ErrFormat)
		}
	}()
	lim := o.cfg.Limits.WithDefaults()
	size := src.Size()
	if size <= 0 || size > math.MaxInt32*int64(16) {
		return nil, fmt.Errorf("pdf of %d bytes: %w", size, ivy.ErrFormat)
	}
	// The whole file goes into memory, so a file larger than the limit
	// stops before the read (D39).
	if size > lim.MaxMemory {
		return nil, fmt.Errorf("pdf of %d bytes, more than %d: %w", size, lim.MaxMemory, ivy.ErrLimit)
	}
	data := make([]byte, size)
	if _, err := src.ReadAt(data, 0); err != nil {
		return nil, fmt.Errorf("reading pdf: %w", err)
	}
	pdf, err := load(ctx, data, o.cfg, lim)
	if err != nil {
		return nil, err
	}
	pages := pdf.Pages().Slice()
	if len(pages) > lim.MaxObjects {
		return nil, fmt.Errorf("pdf of more than %d pages: %w", lim.MaxObjects, ivy.ErrLimit)
	}
	if o.box != CropBox {
		for _, page := range pages {
			page.SetCropBox(pageBox(page, o.box))
		}
	}
	return &Document{
		pdf:         pdf,
		cache:       render.NewRenderCache(),
		fonts:       newFontResolver(o.cfg, lim),
		lim:         lim,
		log:         o.cfg.Log(),
		annotations: o.annotations,
		pages:       pages,
	}, nil
}

// load reads the file with the password of the configuration. While the
// password is wrong, it asks PasswordFunc for another one.
func load(ctx context.Context, data []byte, cfg ivy.Config, lim ivy.Limits) (*syntax.Pdf, error) {
	opts := syntax.Options{
		Password: cfg.Password,
		Limits:   lim,
		Decoders: interpret.ImageDecoders(nil, lim),
	}
	pdf, err := syntax.NewPdfWithOptions(data, opts)
	for attempt := 1; err != nil && errors.Is(err, ivy.ErrPassword) && cfg.PasswordFunc != nil; attempt++ {
		password, perr := cfg.PasswordFunc(ctx, attempt)
		if perr != nil {
			return nil, fmt.Errorf("opening pdf: %w: %w", ivy.ErrPassword, perr)
		}
		if err := ctx.Err(); err != nil {
			return nil, fmt.Errorf("opening pdf: %w", err)
		}
		opts.Password = password
		pdf, err = syntax.NewPdfWithOptions(data, opts)
	}
	if err != nil {
		return nil, fmt.Errorf("opening pdf: %w", err)
	}
	return pdf, nil
}

// pageBox returns the rectangle of box on page. A page without the box
// uses its default (ISO 32000-1, 14.11.2): the CropBox for the BleedBox,
// the TrimBox and the ArtBox. An unknown box is the CropBox.
func pageBox(page *syntax.Page, box Box) syntax.Rect {
	var key syntax.Name
	switch box {
	case MediaBox:
		return page.MediaBox()
	case BleedBox:
		key = syntax.KeyBleedBox
	case TrimBox:
		key = syntax.KeyTrimBox
	case ArtBox:
		key = syntax.KeyArtBox
	default:
		return page.CropBox()
	}
	if r, ok := page.Raw().Get[syntax.Rect](key); ok {
		return r
	}
	return page.CropBox()
}

// Close satisfies the ivy.Document interface.
func (d *Document) Close() error {
	return nil
}

// PageCount satisfies the ivy.Pager interface.
func (d *Document) PageCount(context.Context) (int, error) {
	return len(d.pages), nil
}

// Page satisfies the ivy.Pager interface.
func (d *Document) Page(ctx context.Context, n int) (ivy.Page, error) {
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("reading pdf page: %w", err)
	}
	if n < 0 || n >= len(d.pages) {
		return nil, ivy.ErrPageRange
	}
	return &Page{doc: d, page: d.pages[n], n: n}, nil
}

// Previews satisfies the ivy.Previewer interface. It lists the thumbnail
// of each page that has one, the entry Thumb of the page, in the order of
// the pages. A file without thumbnails has no previews, and a program
// renders a page instead.
func (d *Document) Previews(ctx context.Context) iter.Seq2[ivy.Preview, error] {
	return func(yield func(ivy.Preview, error) bool) {
		for _, page := range d.pages {
			if err := ctx.Err(); err != nil {
				yield(ivy.Preview{}, fmt.Errorf("reading pdf thumbnails: %w", err))
				return
			}
			preview, ok := d.thumbnail(page)
			if ok && !yield(preview, nil) {
				return
			}
		}
	}
}

// thumbnail returns the preview of the thumbnail of a page, and false if
// the page has none.
func (d *Document) thumbnail(page *syntax.Page) (preview ivy.Preview, ok bool) {
	defer func() {
		if recover() != nil {
			preview, ok = ivy.Preview{}, false
		}
	}()
	stream, ok := page.Raw().Get[syntax.Stream](syntax.KeyThumb)
	if !ok {
		return ivy.Preview{}, false
	}
	dict := stream.Dict()
	w, _ := dict.Get[int32](syntax.KeyWidth)
	h, _ := dict.Get[int32](syntax.KeyHeight)
	if w < 0 || h < 0 {
		w, h = 0, 0
	}
	return ivy.Preview{
		Kind: ivy.Thumbnail,
		Size: image.Pt(int(w), int(h)),
		Decode: func(ctx context.Context) (image.Image, error) {
			return d.decodeImage(ctx, stream)
		},
	}, true
}

// decodeImage decodes an image stream.
func (d *Document) decodeImage(ctx context.Context, stream syntax.Stream) (img image.Image, err error) {
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("decoding pdf thumbnail: %w", err)
	}
	defer func() {
		if p := recover(); p != nil {
			img, err = nil, fmt.Errorf("decoding pdf thumbnail: panic: %v: %w", p, ivy.ErrFormat)
		}
	}()
	raster, ok := interpret.NewRasterImage(stream, d.settings(ctx))
	if !ok {
		return nil, fmt.Errorf("pdf thumbnail is not an image: %w", ivy.ErrFormat)
	}
	raster.WithRGBA(func(data interpret.ImageData, alpha *interpret.LumaData) {
		img = toImage(data, alpha)
	}, [2]uint32{})
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("decoding pdf thumbnail: %w", err)
	}
	if img == nil {
		return nil, fmt.Errorf("decoding pdf thumbnail: %w", ivy.ErrFormat)
	}
	return img, nil
}

// toImage returns the decoded image as an image of NRGBA, or nil when the
// data is shorter than the size.
func toImage(data interpret.ImageData, alpha *interpret.LumaData) image.Image {
	var w, h int
	switch {
	case data.RGB != nil:
		w, h = int(data.RGB.Width), int(data.RGB.Height)
	case data.Luma != nil:
		w, h = int(data.Luma.Width), int(data.Luma.Height)
	default:
		return nil
	}
	n := w * h
	if n == 0 {
		return nil
	}
	img := image.NewNRGBA(image.Rect(0, 0, w, h))
	switch {
	case data.RGB != nil:
		if len(data.RGB.Data) < 3*n {
			return nil
		}
		for i := range n {
			copy(img.Pix[4*i:4*i+3], data.RGB.Data[3*i:3*i+3])
			img.Pix[4*i+3] = 255
		}
	default:
		if len(data.Luma.Data) < n {
			return nil
		}
		for i, g := range data.Luma.Data[:n] {
			img.Pix[4*i], img.Pix[4*i+1], img.Pix[4*i+2], img.Pix[4*i+3] = g, g, g, 255
		}
	}
	if alpha != nil && int(alpha.Width) == w && int(alpha.Height) == h && len(alpha.Data) >= n {
		for i, a := range alpha.Data[:n] {
			img.Pix[4*i+3] = a
		}
	}
	return img
}

// Describe satisfies the ivy.Describer interface. It reads the document
// information dictionary, and the language of the catalog.
func (d *Document) Describe(context.Context) (info ivy.Info, err error) {
	defer func() {
		if p := recover(); p != nil {
			err = fmt.Errorf("describing pdf: panic: %v: %w", p, ivy.ErrFormat)
		}
	}()
	m := d.pdf.Metadata()
	info.Title = textString(m.Title)
	if author := textString(m.Author); author != "" {
		info.Authors = []string{author}
	}
	info.Subject = textString(m.Subject)
	info.Producer = textString(m.Producer)
	if info.Producer == "" {
		info.Producer = textString(m.Creator)
	}
	info.Created = dateTime(m.CreationDate)
	info.Modified = dateTime(m.ModificationDate)
	xref := d.pdf.XRef()
	if root, ok := xref.Get[syntax.Dict](xref.RootID()); ok {
		if lang, ok := root.Get[syntax.String](syntax.KeyLang); ok {
			info.Language = textString(lang)
		}
	}
	return info, nil
}

// pdfDocEncoding holds the characters of PDFDocEncoding that differ from
// Latin-1 (ISO 32000-1, Annex D).
var pdfDocEncoding = map[byte]rune{
	0x18: 0x02D8, 0x19: 0x02C7, 0x1A: 0x02C6, 0x1B: 0x02D9, 0x1C: 0x02DD, 0x1D: 0x02DB, 0x1E: 0x02DA, 0x1F: 0x02DC,
	0x80: 0x2022, 0x81: 0x2020, 0x82: 0x2021, 0x83: 0x2026, 0x84: 0x2014, 0x85: 0x2013, 0x86: 0x0192, 0x87: 0x2044,
	0x88: 0x2039, 0x89: 0x203A, 0x8A: 0x2212, 0x8B: 0x2030, 0x8C: 0x201E, 0x8D: 0x201C, 0x8E: 0x201D, 0x8F: 0x2018,
	0x90: 0x2019, 0x91: 0x201A, 0x92: 0x2122, 0x93: 0xFB01, 0x94: 0xFB02, 0x95: 0x0141, 0x96: 0x0152, 0x97: 0x0160,
	0x98: 0x0178, 0x99: 0x017D, 0x9A: 0x0131, 0x9B: 0x0142, 0x9C: 0x0153, 0x9D: 0x0161, 0x9E: 0x017E, 0xA0: 0x20AC,
}

// textString decodes a text string of PDF: UTF-16BE or UTF-8 with a byte
// order mark, or else PDFDocEncoding.
func textString(b []byte) string {
	switch {
	case len(b) >= 2 && b[0] == 0xFE && b[1] == 0xFF:
		b = b[2:]
		u := make([]uint16, len(b)/2)
		for i := range u {
			u[i] = uint16(b[2*i])<<8 | uint16(b[2*i+1])
		}
		return string(utf16.Decode(u))
	case len(b) >= 3 && b[0] == 0xEF && b[1] == 0xBB && b[2] == 0xBF:
		if utf8.Valid(b[3:]) {
			return string(b[3:])
		}
	}
	runes := make([]rune, 0, len(b))
	for _, c := range b {
		if r, ok := pdfDocEncoding[c]; ok {
			runes = append(runes, r)
			continue
		}
		runes = append(runes, rune(c))
	}
	return string(runes)
}

// dateTime returns the time of a date of PDF, or the zero time for nil.
func dateTime(dt *syntax.DateTime) time.Time {
	if dt == nil {
		return time.Time{}
	}
	offset := int(dt.UTCOffsetHour)*3600 + int(dt.UTCOffsetMinute)*60
	if dt.UTCOffsetHour < 0 {
		offset = int(dt.UTCOffsetHour)*3600 - int(dt.UTCOffsetMinute)*60
	}
	return time.Date(int(dt.Year), time.Month(max(dt.Month, 1)), int(max(dt.Day, 1)),
		int(dt.Hour), int(dt.Minute), int(dt.Second), 0, time.FixedZone("", offset))
}

// settings returns the settings of the interpreter for a render that ctx
// bounds.
func (d *Document) settings(ctx context.Context) interpret.InterpreterSettings {
	settings := interpret.DefaultInterpreterSettings()
	settings.Cancel = func() bool { return ctx.Err() != nil }
	settings.Limits = d.lim
	settings.FontResolver = d.fonts.resolve
	decoders := interpret.ImageDecoders(settings.Cancel, d.lim)
	settings.Decoders = &decoders
	if !d.annotations {
		settings.RenderContent = interpret.RenderContentPage
	}
	return settings
}

// Page is a page of a PDF file.
type Page struct {
	doc  *Document
	page *syntax.Page
	n    int
}

// Size satisfies the ivy.Page interface. It returns the size of the page
// box of WithPageBox, by default the CropBox, in points, after the rotation
// of the page.
func (p *Page) Size() (float32, float32) {
	return p.page.RenderDimensions()
}

// Render satisfies the ivy.Page interface. It draws the content of the page
// inside the page box of WithPageBox, by default the CropBox, rotated by the
// entry Rotate of the page.
func (p *Page) Render(ctx context.Context, dst *okage.Pixmap, t okage.Transform) (err error) {
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("rendering pdf page %d: %w", p.n+1, err)
	}
	defer func() {
		if r := recover(); r != nil {
			err = &ivy.PartialError{Err: fmt.Errorf("rendering pdf page %d: panic: %v: %w", p.n+1, r, ivy.ErrFormat)}
		}
	}()
	transform := kurbo.NewAffine([6]float64{
		float64(t.SX), float64(t.KY), float64(t.KX), float64(t.SY), float64(t.TX), float64(t.TY),
	}).Mul(interpret.TransformToKurbo(p.page.InitialTransform(true)))
	dev := display.NewOkage(dst, okage.IdentityTransform())
	err = render.RenderInto(p.page, p.doc.cache, p.doc.settings(ctx), &render.RenderSettings{}, dev,
		int(dst.Width()), int(dst.Height()), transform)
	switch {
	case err == nil:
		return nil
	case ctx.Err() != nil:
		return fmt.Errorf("rendering pdf page %d: %w", p.n+1, ctx.Err())
	}
	p.doc.log.Debug("pdf: page drawn in part", "page", p.n+1, "err", err)
	return &ivy.PartialError{Err: fmt.Errorf("rendering pdf page %d: %w", p.n+1, err)}
}
