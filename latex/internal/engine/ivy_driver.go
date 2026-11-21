// This file of ivy is the driver that draws a page of the tree of boxes onto
// the display list of ivy (D10, D62). It is the walk of the PDF driver of
// upstream, pdfdriver.go, with a device of ivy in place of a page of
// go-pdfkit: the glyphs are runs with their text (D58), and the rules, the
// colors, the frames, the decorations, the transforms, the formulas, the
// figures and the specials of SVG are paths, images and runs.
//
// The device is in TeX points, 1/72.27 inch, with y down from the top left
// corner of the page, as the coordinates of the specials. The driver
// concatenates the scale from TeX points to big points first.

package engine

import (
	"context"
	"fmt"
	"image"
	"strconv"
	"strings"
	"sync"

	"github.com/xo/ivy"
	"github.com/xo/ivy/internal/display"
	"github.com/xo/ivy/internal/font"
	"github.com/xo/okage"
	"github.com/xo/okage/texmath"
)

// FigureKind is the format of a figure that Figures draws.
type FigureKind int

// The kinds of a figure.
const (
	FigureRaster FigureKind = iota // PNG, JPEG or GIF
	FigurePDF                      // the first page of a PDF file
	FigureEPS                      // an EPS file
)

// Figures draws the figures of a document with the decoders of the
// program. The engine draws an SVG figure itself.
type Figures interface {
	// DrawFigure draws the figure into the rectangle from (0, 0) to (w, h)
	// of the user space of d, with y down. res is the resolution of the
	// device, in pixels for each unit of the user space.
	DrawFigure(ctx context.Context, d display.Device, data []byte, kind FigureKind, w, h, res float32) error
}

// Link is a link of a page: a rectangle of the page, in big points, and
// the URI of the link or the name of the destination in the document.
type Link struct {
	Rect okage.Rect
	URI  string
	Dest string
}

// Dest is a named destination of a page: a point in big points.
type Dest struct {
	Name string
	X, Y float32
}

// DrawPage draws page n onto d, in the user space of d, in big points with
// y down. res is the resolution of the device, in pixels for each point.
// It is safe for concurrent use. A figure that fails to draw is left out,
// and DrawPage returns the first such error after the page.
func (d *Doc) DrawPage(ctx context.Context, dev display.Device, n int, figs Figures, res float32) (err error) {
	// The tree of boxes comes from a stranger, and the code of upstream
	// that the walk calls can panic on it (D5).
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("latex: drawing page %d: panic: %v: %w", n, r, ivy.ErrFormat)
		}
	}()
	dr := d.newDrawer(ctx, dev, n, figs)
	dr.res = res
	dr.page(n)
	return dr.err
}

// Links returns the links and the named destinations of page n.
//
// The display list has no link yet. Phase 7c of PLAN.md adds the interface
// display.Linker, and then the driver gives the links to the device. Until
// then a program reads them here.
func (d *Doc) Links(ctx context.Context, n int) (links []Link, dests []Dest) {
	defer func() {
		if recover() != nil {
			links, dests = nil, nil
		}
	}()
	dr := d.newDrawer(ctx, nopDevice{}, n, nil)
	dr.page(n)
	return dr.links, dr.dests
}

// Outline is an entry of the outline of a document: a heading, its level
// and the page of its anchor, from 0.
type Outline struct {
	Title string
	Level int
	Page  int
}

// Outline returns the headings of the document, for a bookmark.
func (d *Doc) Outline() []Outline {
	var out []Outline
	for _, te := range d.e.tocEntries {
		if te.kind != "toc" {
			continue
		}
		if pg := d.e.pageOfIndex(te.marker); pg >= 1 && pg <= len(d.pages) {
			out = append(out, Outline{Title: te.plainTitle, Level: te.level, Page: pg - 1})
		}
	}
	return out
}

// drawer carries the state of the walk of one page.
type drawer struct {
	ctx  context.Context
	doc  *Doc
	dev  display.Device
	figs Figures
	n    int
	// base is the render font, which draws a glyph that names no face.
	base fontFace
	// run is the run of glyphs that is not drawn yet.
	run glyphRun
	// ctm is the transform that the open scopes of the specials impose,
	// and specials holds the specials of the page, in the order of the
	// walk.
	ctm      pictureCTM
	specials []string
	links    []Link
	dests    []Dest
	err      error
	// res is the resolution of the device, in pixels for each big point.
	res float32
}

// newDrawer returns a drawer for page n.
func (d *Doc) newDrawer(ctx context.Context, dev display.Device, n int, figs Figures) *drawer {
	return &drawer{ctx: ctx, doc: d, dev: dev, figs: figs, n: n, base: d.e.renderFont()}
}

// page draws page n.
func (dr *drawer) page(n int) {
	if n < 0 || n >= len(dr.doc.pages) {
		return
	}
	p := dr.doc.pages[n]
	dr.dev.Save()
	defer dr.dev.Restore()
	dr.dev.Concat(okage.TransformFromScale(bigPointsPerTeXPoint, bigPointsPerTeXPoint))
	pw, ph := dr.doc.pageSizePt(n)
	if e := dr.doc.e; e.hasPageColor {
		dr.rect(0, 0, pw, ph, e.pageColor)
	}
	dr.box(p, dr.doc.margin, dr.doc.vmargin+spToPt(p.height))
	dr.flush()
	if _, nop := dr.dev.(nopDevice); !nop {
		dr.drawSpecials(n, pw, ph)
	}
}

// setErr keeps the first error of a figure.
func (dr *drawer) setErr(err error) {
	if err != nil && dr.err == nil {
		dr.err = err
	}
}

// box draws a box with left edge x on the baseline.
func (dr *drawer) box(b *boxNode, x, baseline float64) {
	if b == nil {
		return
	}
	if b.kind == hbox {
		dr.hlist(b, x, baseline)
	} else {
		dr.vlist(b, x, baseline-spToPt(b.height))
	}
}

// hlist lays the material of an hbox along the baseline.
func (dr *drawer) hlist(b *boxNode, x, baseline float64) {
	cx := x
	var lastFace fontFace
	for _, n := range b.list {
		switch c := n.(type) {
		case kernNode:
			cx += spToPt(c.width)
		case glueNode:
			w := spToPt(b.setWidth(c.spec))
			if c.leader != leaderNone {
				dr.flush()
				dr.leader(c.leader, lastFace, x, cx, baseline, w, spToPt(c.cell))
			}
			cx += w
		case charNode:
			if c.face != nil {
				lastFace = c.face
			}
			if !c.covered {
				dr.char(c, cx, baseline)
			}
			cx += spToPt(c.width)
		case ruleNode:
			dr.flush()
			h := spToPt(ruleHeight(c, b))
			dp := spToPt(ruleDepth(c, b))
			w := spToPt(c.width)
			dr.inPicture(func() { dr.rect(cx, baseline-h, w, h+dp, 0) })
			cx += w
		case *boxNode:
			dr.flush()
			dr.box(c, cx, baseline+spToPt(c.shift))
			cx += spToPt(c.width)
		case frameNode:
			dr.flush()
			dr.frame(c, cx, baseline)
			cx += spToPt(c.width())
		case linkNode:
			dr.flush()
			dr.link(c, cx, baseline)
			cx += spToPt(c.width())
		case internalLinkNode:
			dr.flush()
			dr.internalLink(c, cx, baseline)
			cx += spToPt(c.width())
		case decoNode:
			dr.flush()
			dr.deco(c, cx, baseline)
			cx += spToPt(c.width())
		case transformNode:
			dr.flush()
			dr.transform(c, cx, baseline)
			cx += spToPt(c.width())
		case mathNode:
			dr.flush()
			dr.inPicture(func() { dr.math(c, cx, baseline) })
			cx += spToPt(c.width)
		case imageNode:
			dr.flush()
			w, h := spToPt(c.width), spToPt(c.height)
			dr.inPicture(func() { dr.image(c, cx, baseline-h, w, h) })
			cx += w
		case specialNode:
			dr.flush()
			dr.special(c, cx, baseline)
		}
	}
	dr.flush()
}

// vlist stacks the material of a vbox from its top edge.
func (dr *drawer) vlist(b *boxNode, x, top float64) {
	cy := top
	for _, n := range b.list {
		switch c := n.(type) {
		case kernNode:
			cy += spToPt(c.width)
		case glueNode:
			cy += spToPt(b.setWidth(c.spec))
		case ruleNode:
			w := spToPt(ruleWidth(c, b))
			hd := spToPt(c.height + c.depth)
			dr.inPicture(func() { dr.rect(x, cy, w, hd, 0) })
			cy += hd
		case *boxNode:
			dr.box(c, x+spToPt(c.shift), cy+spToPt(c.height))
			cy += spToPt(c.height + c.depth)
		case frameNode:
			dr.frame(c, x, cy+spToPt(c.height()))
			cy += spToPt(c.height() + c.depth())
		case decoNode:
			dr.deco(c, x, cy+spToPt(c.height()))
			cy += spToPt(c.height() + c.depth())
		case transformNode:
			dr.transform(c, x, cy+spToPt(c.height()))
			cy += spToPt(c.height() + c.depth())
		case linkNode:
			dr.link(c, x, cy+spToPt(c.height()))
			cy += spToPt(c.height() + c.depth())
		case internalLinkNode:
			dr.internalLink(c, x, cy+spToPt(c.height()))
			cy += spToPt(c.height() + c.depth())
		case mathNode:
			dr.inPicture(func() { dr.math(c, x, cy+spToPt(c.height)) })
			cy += spToPt(c.height + c.depth)
		case imageNode:
			w, h := spToPt(c.width), spToPt(c.height)
			dr.inPicture(func() { dr.image(c, x, cy, w, h) })
			cy += h + spToPt(c.depth)
		case specialNode:
			dr.special(c, x, cy)
		}
	}
}

// glyphRun is a run of glyphs of one face, one size and one color, that
// the driver has not drawn yet.
type glyphRun struct {
	face   *font.Face
	size   float64
	color  uint32
	glyphs []display.Glyph
	text   strings.Builder
}

// char adds a character to the run, and draws the run first when the
// character needs another face, size or color.
func (dr *drawer) char(c charNode, x, baseline float64) {
	f, scale := charFace(c, dr.base)
	o, ok := f.(*OpenTypeFont)
	if !ok {
		return
	}
	size := o.sizePt() * scale
	gid, ok := o.drawGlyph(c.ch)
	if !ok {
		return
	}
	face := o.f.DrawFace()
	r := &dr.run
	if r.face != face || r.size != size || r.color != c.color {
		dr.flush()
		r.face, r.size, r.color = face, size, c.color
	}
	start := r.text.Len()
	if comps := expandLigature(c.ch); len(comps) > 1 {
		r.text.WriteString(string(comps))
	} else {
		r.text.WriteRune(c.ch)
	}
	r.glyphs = append(r.glyphs, display.Glyph{
		ID: gid, X: float32(x), Y: float32(baseline),
		Start: start, End: r.text.Len(),
	})
}

// drawGlyph returns the glyph that draws r: the glyph of the cmap, or the
// ligature of the components of r, as glyphFor does. It is safe for
// concurrent use.
func (o *OpenTypeFont) drawGlyph(r rune) (font.GID, bool) {
	if gid, ok := o.f.GlyphIndex(r); ok {
		return gid, true
	}
	comps := expandLigature(r)
	if len(comps) < 2 {
		return 0, false
	}
	gids := o.fc.Shape(string(comps), "liga")
	if len(gids) != 1 || gids[0] == 0 {
		return 0, false
	}
	return gids[0], true
}

// flush draws the run of glyphs, in the transform of the open scopes of
// the specials.
func (dr *drawer) flush() {
	r := &dr.run
	if len(r.glyphs) == 0 {
		r.text.Reset()
		return
	}
	run := display.GlyphRun{
		Face:      r.face,
		Size:      float32(r.size),
		Glyphs:    r.glyphs,
		Transform: okage.IdentityTransform(),
		Text:      r.text.String(),
	}
	shader := solid(r.color)
	dr.inPicture(func() { dr.dev.FillGlyphs(run, shader) })
	r.glyphs = nil
	r.text.Reset()
}

// inPicture runs one drawing operation through the transform that the open
// scopes of the specials impose, as the PDF driver does.
func (dr *drawer) inPicture(draw func()) {
	if !dr.ctm.active() {
		draw()
		return
	}
	dr.dev.Save()
	dr.dev.Concat(affineTransform(dr.ctm.cur()))
	draw()
	dr.dev.Restore()
}

// affineTransform returns the transform of okage of m.
func affineTransform(m affine) okage.Transform {
	return okage.TransformFromRow(float32(m.a), float32(m.b), float32(m.c), float32(m.d), float32(m.e), float32(m.f))
}

// solid returns the shader of a color 0xRRGGBB.
func solid(c uint32) okage.Shader {
	return okage.SolidColor(okage.ColorFromRGBA8(uint8(c>>16), uint8(c>>8), uint8(c), 255))
}

// rect fills a rectangle whose top left corner is (x, top).
func (dr *drawer) rect(x, top, w, h float64, color uint32) {
	if !(w > 0 && h > 0) {
		return
	}
	r, ok := okage.RectFromXYWH(float32(x), float32(top), float32(w), float32(h))
	if !ok {
		return
	}
	dr.dev.FillPath(okage.PathFromRect(r), okage.FillRuleWinding, solid(color))
}

// leader draws the set width of a glue as a rule or as dots.
func (dr *drawer) leader(kind glueLeader, face fontFace, edge, x, baseline, w, cell float64) {
	switch kind {
	case leaderRule:
		th := spToPt(defaultRule)
		dr.rect(x, baseline-th, w, th, 0)
	case leaderDots:
		if face == nil {
			face = dr.base
		}
		o, ok := face.(*OpenTypeFont)
		if !ok {
			return
		}
		gid, ok := o.drawGlyph('.')
		if !ok {
			return
		}
		draw := o.f.DrawFace()
		dotW := float64(draw.Advance(gid)) * o.sizePt()
		_, cell = dotLeaderGeom(w, dr.base.sizePt(), cell)
		var glyphs []display.Glyph
		for _, at := range dotLeaderOrigins(edge, x, w, cell) {
			glyphs = append(glyphs, display.Glyph{ID: gid, X: float32(at + (cell-dotW)/2), Y: float32(baseline)})
		}
		if len(glyphs) == 0 {
			return
		}
		dr.dev.FillGlyphs(display.GlyphRun{
			Face: draw, Size: float32(o.sizePt()), Glyphs: glyphs,
			Transform: okage.IdentityTransform(),
		}, solid(0))
	}
}

// frame draws a frame and its background, then its box.
func (dr *drawer) frame(fr frameNode, x, baseline float64) {
	w := spToPt(fr.width())
	top := baseline - spToPt(fr.height())
	total := spToPt(fr.height() + fr.depth())
	if fr.bg != 0 {
		dr.rect(x, top, w, total, fr.bg)
	}
	if fr.rule > 0 {
		ru := spToPt(fr.rule)
		dr.rect(x, top, w, ru, fr.ruleColor)
		dr.rect(x, top+total-ru, w, ru, fr.ruleColor)
		dr.rect(x, top, ru, total, fr.ruleColor)
		dr.rect(x+w-ru, top, ru, total, fr.ruleColor)
	}
	dr.box(fr.inner, x+spToPt(fr.rule+fr.sep), baseline)
}

// pageRect returns the rectangle of the page, in big points, of a box of
// the walk.
func pageRect(x, top, w, h float64) (okage.Rect, bool) {
	k := bigPointsPerTeXPoint
	return okage.RectFromXYWH(float32(x*k), float32(top*k), float32(w*k), float32(h*k))
}

// link draws the box of a hyperlink and records its rectangle.
func (dr *drawer) link(ln linkNode, x, baseline float64) {
	dr.box(ln.inner, x, baseline)
	if ln.url == "" || dr.ctm.active() {
		return
	}
	in := ln.inner
	if r, ok := pageRect(x, baseline-spToPt(in.height), spToPt(in.width), spToPt(in.height+in.depth)); ok {
		dr.links = append(dr.links, Link{Rect: r, URI: ln.url})
	}
}

// internalLink draws the box of a link in the document, and records the
// link or the destination.
func (dr *drawer) internalLink(n internalLinkNode, x, baseline float64) {
	dr.box(n.inner, x, baseline)
	if n.name == "" || dr.ctm.active() {
		return
	}
	in := n.inner
	k := bigPointsPerTeXPoint
	if n.target {
		dr.dests = append(dr.dests, Dest{Name: n.name, X: float32(x * k), Y: float32((baseline - spToPt(in.height)) * k)})
		return
	}
	if r, ok := pageRect(x, baseline-spToPt(in.height), spToPt(in.width), spToPt(in.height+in.depth)); ok {
		dr.links = append(dr.links, Link{Rect: r, Dest: n.name})
	}
}

// deco draws an underline, an overline or a line through a box.
func (dr *drawer) deco(dn decoNode, x, baseline float64) {
	dr.box(dn.inner, x, baseline)
	dr.rect(x, baseline+spToPt(dn.decoRuleTop()), spToPt(dn.width()), spToPt(decoRule), dn.color)
}

// transform draws the box of \scalebox, \reflectbox, \resizebox or
// \rotatebox. The map of the node is in the space of the math, with y up,
// so in the space of the page, with y down, its coefficients b and c
// change their sign.
func (dr *drawer) transform(tn transformNode, x, baseline float64) {
	px := x + spToPt(tn.refDX)
	py := baseline
	m := affine{a: tn.a, b: -tn.b, c: -tn.c, d: tn.d}
	m.e = px - (m.a*px + m.c*py)
	m.f = py - (m.b*px + m.d*py)
	dr.dev.Save()
	dr.dev.Concat(affineTransform(m))
	dr.box(tn.inner, px, baseline)
	dr.dev.Restore()
}

// special records the literal of a special with the point it reached, as
// the PDF driver does. The literals of a page draw together after the walk.
func (dr *drawer) special(s specialNode, x, baseline float64) {
	lit, ok := specialLiteral(s.text, x, baseline)
	if !ok {
		return
	}
	lit = dr.ctm.org.next(lit)
	dr.specials = append(dr.specials, dr.ctm.feed(lit))
}

// math draws a formula whose baseline starts at (x, baseline).
func (dr *drawer) math(c mathNode, x, baseline float64) {
	box, color, ok := dr.doc.mathBox(c.svg)
	if !ok {
		return
	}
	draw := dr.doc.math.DrawFace()
	mathFace := dr.doc.math.MathFace()
	dr.dev.Save()
	defer dr.dev.Restore()
	dr.dev.Concat(okage.TransformFromTranslate(float32(x), float32(baseline)))
	for i := range box.Items {
		it := &box.Items[i]
		col := solid(color)
		if it.Color != okage.ColorBlack {
			col = okage.SolidColor(it.Color)
		}
		transformed := !it.Transform.IsIdentity()
		if transformed {
			dr.dev.Save()
			dr.dev.Concat(it.Transform)
		}
		switch it.Kind {
		case texmath.ItemGlyph:
			if draw == nil || it.Face != mathFace {
				break
			}
			run := display.GlyphRun{
				Face: draw, Size: float32(it.Size),
				Glyphs:    []display.Glyph{{ID: font.GID(it.Glyph), X: float32(it.X), Y: float32(it.Y), End: len(it.Text)}},
				Transform: okage.IdentityTransform(),
				Text:      it.Text,
			}
			dr.dev.FillGlyphs(run, col)
		case texmath.ItemRule:
			if r, ok := okage.RectFromXYWH(float32(it.X), float32(it.Y), float32(it.W), float32(it.H)); ok {
				dr.dev.FillPath(okage.PathFromRect(r), okage.FillRuleWinding, col)
			}
		case texmath.ItemPath:
			if it.Path != nil {
				dr.dev.Save()
				dr.dev.Concat(okage.TransformFromTranslate(float32(it.X), float32(it.Y)))
				dr.dev.FillPath(it.Path, okage.FillRuleWinding, col)
				dr.dev.Restore()
			}
		}
		if transformed {
			dr.dev.Restore()
		}
	}
}

// mathBox returns the box of a formula whose SVG the math renderer of ivy
// wrote, and the color that the engine gave the formula.
func (d *Doc) mathBox(svg string) (*texmath.Box, uint32, bool) {
	if d.math == nil {
		return nil, 0, false
	}
	const key = `data-ivy-math="`
	i := strings.Index(svg, key)
	if i < 0 {
		return nil, 0, false
	}
	rest := svg[i+len(key):]
	j := strings.IndexByte(rest, '"')
	if j < 0 {
		return nil, 0, false
	}
	id, err := strconv.Atoi(rest[:j])
	if err != nil {
		return nil, 0, false
	}
	box, ok := d.math.Box(id)
	if !ok {
		return nil, 0, false
	}
	var color uint32
	if g := strings.Index(svg, "<g "); g >= 0 {
		if c, _, ok := parseSVGFill(tagAttr(svg[g:], "fill")); ok {
			color = c
		}
	}
	return box, color, true
}

// image draws a figure into the rectangle whose top left corner is (x,
// top).
func (dr *drawer) image(n imageNode, x, top, w, h float64) {
	if !(w > 0 && h > 0) {
		return
	}
	dr.dev.Save()
	defer dr.dev.Restore()
	dr.dev.Concat(okage.TransformFromTranslate(float32(x), float32(top)))
	if n.format == imgSVG {
		dr.setErr(drawSVGImage(dr.dev, n.data, w, h))
		return
	}
	if dr.figs == nil {
		return
	}
	kind := FigureRaster
	switch n.format {
	case imgPDF:
		kind = FigurePDF
	case imgEPS:
		kind = FigureEPS
	}
	dr.setErr(dr.figs.DrawFigure(dr.ctx, dr.dev, n.data, kind, float32(w), float32(h), dr.res*bigPointsPerTeXPoint))
}

// specialCache holds the parsed specials of each page of a document.
type specialCache struct {
	mu    sync.Mutex
	pages map[int]*specialPage
}

// specialPage is the parsed specials of one page.
type specialPage struct {
	once sync.Once
	tree *svgTree
	err  error
}

// drawSpecials draws the specials of page n as one SVG stream in the
// coordinates of the page, after the boxes, as the PDF driver does.
func (dr *drawer) drawSpecials(n int, pw, ph float64) {
	if len(dr.specials) == 0 {
		return
	}
	sp := dr.doc.specialPage(n)
	sp.once.Do(func() {
		sp.tree, sp.err = parseSpecials(strings.Join(dr.specials, ""), pw, ph)
	})
	if sp.err != nil {
		dr.setErr(sp.err)
		return
	}
	sp.tree.draw(dr.dev)
}

// specialPage returns the cache of the specials of page n.
func (d *Doc) specialPage(n int) *specialPage {
	d.specials.mu.Lock()
	defer d.specials.mu.Unlock()
	if d.specials.pages == nil {
		d.specials.pages = make(map[int]*specialPage)
	}
	sp, ok := d.specials.pages[n]
	if !ok {
		sp = new(specialPage)
		d.specials.pages[n] = sp
	}
	return sp
}

// nopDevice is a device that draws nothing, for a walk that only reads the
// links of a page.
type nopDevice struct{}

func (nopDevice) Save()                                                        {}
func (nopDevice) Restore()                                                     {}
func (nopDevice) Concat(okage.Transform)                                       {}
func (nopDevice) FillPath(*okage.Path, okage.FillRule, okage.Shader)           {}
func (nopDevice) StrokePath(*okage.Path, okage.Stroke, okage.Shader)           {}
func (nopDevice) ClipPath(*okage.Path, okage.FillRule)                         {}
func (nopDevice) DrawImage(image.Image, okage.Transform, display.ImageOptions) {}
func (nopDevice) FillMask(*image.Alpha, okage.Transform, display.ImageOptions, okage.Shader) {
}
func (nopDevice) FillGlyphs(display.GlyphRun, okage.Shader) {}
func (nopDevice) BeginGroup(display.Group)                  {}
func (nopDevice) EndGroup()                                 {}
