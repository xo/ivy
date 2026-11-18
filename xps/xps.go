// Package xps opens XPS and OpenXPS documents, and renders their pages.
//
// An XPS file is a zip package of parts: a FixedDocumentSequence names its
// FixedDocuments, and each FixedDocument names its FixedPages. A FixedPage
// holds canvases, paths and runs of glyphs, which the package draws through
// the display list of ivy (D10). The fonts are embedded, and the package
// undoes their obfuscation (D12).
//
// The package draws solid colors, linear and radial gradients, image brushes
// and visual brushes, with transforms, clips, opacities and opacity masks. A
// brush that tiles draws its tile into a pixmap at the resolution of the
// target, and repeats it. The modes FlipX, FlipY and FlipXY repeat a tile of
// two or four mirrored copies. A VisualBrush nests at most 4 deep, and the
// tiles and the visuals of a page draw at most Limits.MaxPixels pixels. A
// sideways glyph run turns each glyph about its vertical origin. An image in
// JPEG XR is left out, and the page that holds one returns an
// *ivy.PartialError (D5).
package xps

import (
	"context"
	"fmt"
	"image"
	"iter"
	"log/slog"
	"path"
	"strings"
	"sync"
	"time"

	_ "golang.org/x/image/tiff" // the TIFF decoder, for the images of a page

	"github.com/xo/ivy"
	"github.com/xo/ivy/internal/display"
	"github.com/xo/ivy/internal/font"
	"github.com/xo/ivy/internal/zipdoc"
	"github.com/xo/okage"
	"github.com/xo/okage/roxmltree"
)

// Format describes the package for the registry of a program.
var Format = ivy.Format{
	Name:  "xps",
	Mimes: []string{"application/oxps", "application/vnd.ms-xpsdocument"},
	Exts:  []string{"xps", "oxps"},
	Open: func(ctx context.Context, src ivy.Source, cfg ivy.Config) (ivy.Document, error) {
		return Open(ctx, src, WithConfig(cfg))
	},
}

// pointsPerUnit converts the units of XPS, 1/96 of an inch, to points.
const pointsPerUnit = 72.0 / 96.0

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

// Document is an open XPS document.
type Document struct {
	pkg   *pkg
	lim   ivy.Limits
	log   *slog.Logger
	pages []*Page
	rels  []rel

	mu     sync.Mutex
	fonts  map[string]fontResult
	images map[string]imageResult
}

// fontResult is a font that the document loaded, or the error of its load.
type fontResult struct {
	face *font.Face
	err  error
}

// imageResult is an image that the document decoded, or the error of its
// decode.
type imageResult struct {
	img image.Image
	// dpiX and dpiY are the resolution of the image in dots per inch, or 0
	// if the image does not give one.
	dpiX, dpiY float64
	err        error
}

// The capabilities of a document.
var (
	_ ivy.Pager     = (*Document)(nil)
	_ ivy.Previewer = (*Document)(nil)
	_ ivy.Describer = (*Document)(nil)
)

// Open opens an XPS or an OpenXPS document. It reads the list of the pages,
// and reads each page when it is asked for. It returns an error that wraps
// ivy.ErrFormat if src is not an XPS package.
func Open(ctx context.Context, src ivy.Source, opts ...Option) (*Document, error) {
	var o options
	for _, opt := range opts {
		opt(&o)
	}
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("opening xps: %w", err)
	}
	lim := o.cfg.Limits.WithDefaults()
	archive, err := zipdoc.Open(src, lim)
	if err != nil {
		return nil, err
	}
	d := &Document{
		pkg:    &pkg{archive: archive, lim: lim},
		lim:    lim,
		log:    o.cfg.Log(),
		fonts:  make(map[string]fontResult),
		images: make(map[string]imageResult),
	}
	d.rels = d.pkg.rels("/")
	seq, ok := find(d.rels, relFixedRepresentation)
	if !ok {
		// a package without relationships names its sequence by its type
		for _, f := range archive.Files() {
			if strings.EqualFold(path.Ext(f.Name), ".fdseq") {
				seq, ok = "/"+f.Name, true
				break
			}
		}
	}
	if !ok {
		return nil, fmt.Errorf("xps package without a fixed document sequence: %w", ivy.ErrFormat)
	}
	if err := d.readSequence(ctx, seq); err != nil {
		return nil, err
	}
	return d, nil
}

// readSequence reads the FixedDocumentSequence, and the list of the pages of
// each of its FixedDocuments.
func (d *Document) readSequence(ctx context.Context, name string) error {
	seq, err := d.pkg.xml(name)
	if err != nil {
		return err
	}
	for n := range seq.Descendants() {
		if !n.IsElement() || local(n) != "DocumentReference" {
			continue
		}
		if err := ctx.Err(); err != nil {
			return fmt.Errorf("reading xps: %w", err)
		}
		docName := resolve(name, attr(n, "Source"))
		doc, err := d.pkg.xml(docName)
		if err != nil {
			return err
		}
		for p := range doc.Descendants() {
			if !p.IsElement() || local(p) != "PageContent" {
				continue
			}
			if len(d.pages) >= d.lim.MaxObjects {
				return fmt.Errorf("xps of more than %d pages: %w", d.lim.MaxObjects, ivy.ErrLimit)
			}
			d.pages = append(d.pages, &Page{
				doc:    d,
				part:   resolve(docName, attr(p, "Source")),
				width:  number(attr(p, "Width"), 0),
				height: number(attr(p, "Height"), 0),
			})
		}
	}
	return nil
}

// Close satisfies the ivy.Document interface.
func (d *Document) Close() error {
	return nil
}

// PageCount satisfies the ivy.Pager interface.
func (d *Document) PageCount(context.Context) (int, error) {
	return len(d.pages), nil
}

// Page satisfies the ivy.Pager interface. It reads the FixedPage, for its
// size.
func (d *Document) Page(ctx context.Context, n int) (ivy.Page, error) {
	if n < 0 || n >= len(d.pages) {
		return nil, ivy.ErrPageRange
	}
	p := d.pages[n]
	if _, err := p.root(ctx); err != nil {
		return nil, err
	}
	return p, nil
}

// Previews satisfies the ivy.Previewer interface. It lists the thumbnail of
// the package, where it has one.
func (d *Document) Previews(ctx context.Context) iter.Seq2[ivy.Preview, error] {
	name, ok := find(d.rels, relThumbnail)
	if !ok {
		return func(func(ivy.Preview, error) bool) {}
	}
	return d.pkg.archive.Previews(ctx, zipdoc.Candidate{Kind: ivy.Thumbnail, Name: partName(name)})
}

// Describe satisfies the ivy.Describer interface. It reads the core
// properties of the package: its title, its creator, its subject, its
// language and its dates.
func (d *Document) Describe(context.Context) (ivy.Info, error) {
	var info ivy.Info
	name, ok := find(d.rels, relCoreProperties)
	if !ok {
		return info, nil
	}
	doc, err := d.pkg.xml(name)
	if err != nil {
		return info, err
	}
	for n := range doc.Descendants() {
		if !n.IsElement() {
			continue
		}
		text, _ := n.Text()
		text = strings.TrimSpace(text)
		switch local(n) {
		case "title":
			info.Title = text
		case "creator":
			if text != "" {
				info.Authors = []string{text}
			}
		case "subject":
			info.Subject = text
		case "language":
			info.Language = text
		case "created":
			info.Created, _ = time.Parse(time.RFC3339, text)
		case "modified":
			info.Modified, _ = time.Parse(time.RFC3339, text)
		}
	}
	return info, nil
}

// font returns the font part name, which it loads once. A font part with the
// extension .odttf is obfuscated (D12).
func (d *Document) font(name string) (*font.Face, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if r, ok := d.fonts[name]; ok {
		return r.face, r.err
	}
	var r fontResult
	data, err := d.pkg.read(name)
	if err == nil && strings.EqualFold(path.Ext(name), ".odttf") {
		err = deobfuscate(data, name)
	}
	if err == nil {
		r.face, err = font.Load(data, 0)
	}
	if err != nil {
		r.err = fmt.Errorf("font %s: %w", name, err)
	}
	d.fonts[name] = r
	return r.face, r.err
}

// image returns the image part name, which it decodes once, with its
// resolution.
func (d *Document) image(ctx context.Context, name string) imageResult {
	d.mu.Lock()
	if r, ok := d.images[name]; ok {
		d.mu.Unlock()
		return r
	}
	d.mu.Unlock()
	r := d.pkg.image(ctx, name)
	d.mu.Lock()
	defer d.mu.Unlock()
	if ctx.Err() == nil {
		d.images[name] = r
	}
	return r
}

// deobfuscate undoes the obfuscation of an embedded font in place. The key is
// the GUID in the name of the part, and the first 32 bytes of the font are
// XORed with its 16 bytes, in the order of the bytes of a GUID in memory
// (ECMA-388, the obfuscation of fonts).
func deobfuscate(data []byte, name string) error {
	base := strings.TrimSuffix(path.Base(name), path.Ext(name))
	hexDigits := strings.Map(func(r rune) rune {
		if r == '-' || r == '{' || r == '}' {
			return -1
		}
		return r
	}, base)
	if len(hexDigits) != 32 || len(data) < 32 {
		return fmt.Errorf("obfuscated font %s without a guid: %w", name, ivy.ErrFormat)
	}
	var guid [16]byte
	for i := range guid {
		var v byte
		for _, c := range hexDigits[2*i : 2*i+2] {
			switch {
			case c >= '0' && c <= '9':
				v = v<<4 | byte(c-'0')
			case c >= 'a' && c <= 'f':
				v = v<<4 | byte(c-'a'+10)
			case c >= 'A' && c <= 'F':
				v = v<<4 | byte(c-'A'+10)
			default:
				return fmt.Errorf("obfuscated font %s without a guid: %w", name, ivy.ErrFormat)
			}
		}
		guid[i] = v
	}
	// the bytes of the guid, read in its string, in the order of the key
	order := [16]int{15, 14, 13, 12, 11, 10, 9, 8, 6, 7, 4, 5, 0, 1, 2, 3}
	for i := range 32 {
		data[i] ^= guid[order[i%16]]
	}
	return nil
}

// Page is a page of an XPS document.
type Page struct {
	doc           *Document
	part          string
	width, height float32

	once sync.Once
	tree *roxmltree.Document
	err  error
}

// root reads the FixedPage of the page once, and its size.
func (p *Page) root(ctx context.Context) (roxmltree.Node, error) {
	if err := ctx.Err(); err != nil {
		return roxmltree.Node{}, fmt.Errorf("reading xps page: %w", err)
	}
	p.once.Do(func() {
		p.tree, p.err = p.doc.pkg.xml(p.part)
		if p.err != nil {
			return
		}
		root := p.tree.RootElement()
		if local(root) != "FixedPage" {
			p.err = fmt.Errorf("page %s is a %s: %w", p.part, local(root), ivy.ErrFormat)
			return
		}
		if w := number(attr(root, "Width"), 0); w > 0 {
			p.width = w
		}
		if h := number(attr(root, "Height"), 0); h > 0 {
			p.height = h
		}
		if p.width <= 0 || p.height <= 0 {
			p.err = fmt.Errorf("page %s without a size: %w", p.part, ivy.ErrFormat)
		}
	})
	if p.err != nil {
		return roxmltree.Node{}, p.err
	}
	return p.tree.RootElement(), nil
}

// Size satisfies the ivy.Page interface.
func (p *Page) Size() (float32, float32) {
	return p.width * pointsPerUnit, p.height * pointsPerUnit
}

// Render satisfies the ivy.Page interface. It returns an *ivy.PartialError
// if it left out an element of the page.
func (p *Page) Render(ctx context.Context, dst *okage.Pixmap, t okage.Transform) error {
	root, err := p.root(ctx)
	if err != nil {
		return err
	}
	ctm := t.PreScale(pointsPerUnit, pointsPerUnit)
	area, _ := okage.RectFromXYWH(0, 0, float32(dst.Width()), float32(dst.Height()))
	return p.draw(ctx, root, display.NewOkage(dst, ctm), ctm, area)
}

// draw walks the page onto a device. ctm maps the units of the page to the
// pixels of the device, and area is the target of the device in its pixels.
func (p *Page) draw(ctx context.Context, root roxmltree.Node, dev display.Device, ctm okage.Transform, area okage.Rect) error {
	r := &renderer{ctx: ctx, doc: p.doc, part: p.part, dev: dev, ctm: ctm, area: area, w: &walk{}}
	r.page(root)
	if r.w.err != nil {
		if err := ctx.Err(); err != nil {
			return fmt.Errorf("rendering xps page: %w", err)
		}
		return &ivy.PartialError{Err: r.w.err}
	}
	return nil
}
