package msppt

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/xo/okage"

	"github.com/xo/ivy"
	"github.com/xo/ivy/internal/display"
	"github.com/xo/ivy/internal/drawingml"
	"github.com/xo/ivy/internal/odraw"
)

// emuPerPoint is the number of EMUs in a point. A master unit is 1/8 of a
// point.
const (
	emuPerPoint    = 12700
	unitsPerPoint  = 8
	defaultInsetLR = 91440
	defaultInsetTB = 45720
	geoSize        = 21600
)

// master is a main master or a title master, read once.
type master struct {
	flags    uint16
	parent   *master
	drawing  *drawing
	scheme   [8]drawingml.RGBA
	hasSheme bool
	styles   [txTypes]masterStyle
	hasStyle [txTypes]bool
	// spids maps the id of each shape to the shape, for the shapes of a
	// slide that inherit from them.
	spids map[uint32]*shape
}

// slideAtom reads the master id and the flags of a SlideAtom.
func slideAtom(body []byte) (uint32, uint16) {
	a := child(body, rtSlideAtom)
	if len(a) < 22 {
		return 0, slideMasterObjects | slideMasterScheme | slideMasterBackground
	}
	return le.Uint32(a[12:]), le.Uint16(a[20:])
}

// colorScheme reads the current color scheme of a slide or of a master: the
// SlideSchemeColorSchemeAtom of the instance 1, or the first one.
func colorScheme(body []byte) ([8]drawingml.RGBA, bool) {
	var scheme [8]drawingml.RGBA
	var found []byte
	for h, b := range children(body) {
		if h.Type != rtColorScheme || len(b) < 32 {
			continue
		}
		if found == nil || h.Inst == 1 {
			found = b
		}
		if h.Inst == 1 {
			break
		}
	}
	if found == nil {
		return scheme, false
	}
	for i := range scheme {
		scheme[i] = rgb(found[4*i], found[4*i+1], found[4*i+2])
	}
	return scheme, true
}

// rgb returns the opaque color of the bytes r, g and b.
func rgb(r, g, b uint8) drawingml.RGBA {
	return odraw.RGB(r, g, b)
}

// defaultScheme is the color scheme of a presentation that gives none:
// black text on white.
var defaultScheme = [8]drawingml.RGBA{
	rgb(255, 255, 255), rgb(0, 0, 0), rgb(128, 128, 128), rgb(0, 0, 0),
	rgb(0, 204, 153), rgb(51, 51, 204), rgb(204, 204, 255), rgb(178, 178, 178),
}

// master returns the master with the id, read once. It follows a title
// master to its main master, depth times at most.
func (d *Document) master(id uint32, depth int) *master {
	if m, ok := d.masterCache[id]; ok {
		return m
	}
	// a master that refers to itself ends here
	d.masterCache[id] = nil
	persist, ok := d.doc.masters[id]
	if !ok || depth <= 0 {
		return nil
	}
	h, body, ok := d.stream.object(persist)
	if !ok || (h.Type != rtMainMaster && h.Type != rtSlide) {
		return nil
	}
	lim := d.cfg.Limits
	m := &master{spids: make(map[uint32]*shape)}
	var parentID uint32
	parentID, m.flags = slideAtom(body)
	if h.Type == rtSlide && parentID != 0 && parentID != id {
		// a title master follows a main master
		m.parent = d.master(parentID, depth-1)
	}
	m.scheme, m.hasSheme = colorScheme(body)
	for ch, b := range children(body) {
		if ch.Type == rtTextMasterStyle && int(ch.Inst) < txTypes {
			if st, ok := readMasterStyle(b, ch.Inst); ok {
				m.styles[ch.Inst], m.hasStyle[ch.Inst] = st, true
			}
		}
	}
	m.drawing = readDrawing(body, lim.MaxObjects, lim.MaxDepth)
	m.drawing.shapes(func(sh *shape, _ *odraw.Transform) {
		m.spids[sh.SPID] = sh
	})
	d.masterCache[id] = m
	return m
}

// mainMaster returns the main master of m: m, or the master of a title
// master.
func (m *master) mainMaster() *master {
	if m.parent != nil {
		return m.parent
	}
	return m
}

// colors returns the color scheme of m.
func (m *master) colors() [8]drawingml.RGBA {
	for x := m; x != nil; x = x.parent {
		if x.hasSheme && (x.parent == nil || x.flags&slideMasterScheme == 0) {
			return x.scheme
		}
	}
	return defaultScheme
}

// builder builds the scene of a slide.
type builder struct {
	d      *Document
	scene  *drawingml.Scene
	scheme [8]drawingml.RGBA
	master *master
	texts  []text
	number int
	log    *slog.Logger
	// hf says which placeholders of the date, the slide number and the
	// footer of the master show.
	hf headersFooters
	// style turns the shapes into figures, with the color scheme of the
	// slide.
	style *odraw.Style
	// figures are the figures of the scene, in order, for the tests.
	figures []*drawingml.Figure
}

// add adds the figure f to the scene.
func (b *builder) add(f *drawingml.Figure) {
	b.figures = append(b.figures, f)
	b.scene.AddFigure(f)
}

// slidePage builds the page of the slide n.
func (d *Document) slidePage(ctx context.Context, n int) (*page, error) {
	w, h := d.doc.width/unitsPerPoint, d.doc.height/unitsPerPoint
	b := &builder{d: d, scene: drawingml.NewScene(d.cfg), scheme: defaultScheme, number: n + 1, log: d.cfg.Log()}
	pg := &page{scene: b.scene, w: w, h: h, n: n}
	defer func() {
		pg.figures = b.figures
	}()
	if n >= len(d.doc.slides) {
		// a presentation without a slide has one empty page
		b.fillPage(drawingml.Solid(drawingml.White))
		return pg, nil
	}
	ref := d.doc.slides[n]
	b.texts = ref.texts
	hd, body, ok := d.stream.object(ref.persist)
	if !ok || hd.Type != rtSlide {
		b.fillPage(drawingml.Solid(drawingml.White))
		return pg, nil
	}
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("reading slide %d: %w", n+1, err)
	}
	lim := d.cfg.Limits
	masterID, flags := slideAtom(body)
	b.master = d.master(masterID, 2)
	if b.master != nil {
		b.scheme = b.master.colors()
	}
	if flags&slideMasterScheme == 0 {
		if s, ok := colorScheme(body); ok {
			b.scheme = s
		}
	}
	b.style = &odraw.Style{
		Scheme:     func(i uint32) drawingml.RGBA { return b.scheme[i&7] },
		Picture:    d.picture,
		Background: b.scheme[0],
		Log:        b.log,
	}
	b.hf = d.doc.hf
	if hf := readHeadersFooters(body); hf.ok {
		b.hf = hf
	}
	dr := readDrawing(body, lim.MaxObjects, lim.MaxDepth)
	b.background(dr, flags)
	if flags&slideMasterObjects != 0 && b.master != nil {
		b.masterObjects(b.master, lim.MaxDepth)
	}
	dr.shapes(func(sh *shape, t *odraw.Transform) {
		if b.scene.Len() < lim.MaxObjects {
			b.addShape(sh, t)
		}
	})
	return pg, nil
}

// fillPage fills the whole page with pt.
func (b *builder) fillPage(pt drawingml.Paint) {
	w, h := b.d.doc.width/unitsPerPoint, b.d.doc.height/unitsPerPoint
	box, ok := okage.RectFromXYWH(0, 0, float32(w), float32(h))
	if !ok {
		return
	}
	b.add(&drawingml.Figure{
		Transform: okage.IdentityTransform(),
		W:         w,
		H:         h,
		Paths:     []drawingml.Path{{Path: okage.PathFromRect(box)}},
		Fill:      pt,
		Line:      drawingml.Line{Paint: drawingml.Paint{Kind: drawingml.PaintNone}},
	})
}

// background fills the page with the background of the slide, or of its
// master.
func (b *builder) background(dr *drawing, flags uint16) {
	bg := dr.background
	if flags&slideMasterBackground != 0 || bg == nil {
		bg = nil
		for m := b.master; m != nil; m = m.parent {
			if m.drawing.background != nil {
				bg = m.drawing.background
				if m.parent == nil || m.flags&slideMasterBackground == 0 {
					break
				}
			}
		}
	}
	pt := drawingml.Solid(b.scheme[0])
	if bg != nil {
		if p := b.style.Fill(b.chain(bg)); p.Kind != drawingml.PaintNone {
			pt = p
		}
	}
	b.fillPage(pt)
}

// masterObjects draws the shapes of the master m that are not
// placeholders, after the shapes of its main master if it shows them. The
// placeholders of the date, the slide number and the footer show as the
// HeadersFootersAtom of the slide says, with the text that it gives.
func (b *builder) masterObjects(m *master, depth int) {
	if depth <= 0 {
		return
	}
	if m.parent != nil && m.flags&slideMasterObjects != 0 {
		b.masterObjects(m.parent, depth-1)
	}
	lim := b.d.cfg.Limits
	m.drawing.shapes(func(sh *shape, t *odraw.Transform) {
		if b.scene.Len() >= lim.MaxObjects {
			return
		}
		if sh.placeholder != 0 {
			show, units := b.hf.shows(sh.placeholder)
			if !show {
				return
			}
			if units != nil && sh.text != nil {
				c := *sh
				c.text = sh.text.withUnits(units)
				sh = &c
			}
		}
		b.addShape(sh, t)
	})
}

// chain returns the properties of the shape sh, then of the shape of the
// master that it inherits from. The default properties of the drawing
// group of the document are for the new shapes that a person draws in
// PowerPoint, and a shape does not take them: PowerPoint draws a shape
// without a fill color white, and a line without a width 0.75 points
// wide, as MS-ODRAW gives the defaults of the properties.
func (b *builder) chain(sh *shape) odraw.Chain {
	c := odraw.Chain{&sh.Props}
	if ms := b.masterShape(sh); ms != nil {
		c = append(c, &ms.Props)
	}
	return c
}

// addShape adds the shape sh, with the transform t of its group, to the
// scene.
func (b *builder) addShape(sh *shape, t *odraw.Transform) {
	c := b.chain(sh)
	if odraw.Hidden(c) {
		return
	}
	a, ok := sh.anchor, sh.hasAnchor
	if !ok {
		if ms := b.masterShape(sh); ms != nil && ms.hasAnchor {
			a, ok = ms.anchor, true
		}
	}
	if !ok {
		return
	}
	r := t.Apply(a)
	r = odraw.Rect{Left: r.Left / unitsPerPoint, Top: r.Top / unitsPerPoint, Right: r.Right / unitsPerPoint, Bottom: r.Bottom / unitsPerPoint}
	f := b.style.Figure(sh.Shape, c, r)
	if f == nil {
		return
	}
	if tx := b.textOf(sh); tx != nil {
		f.Text = b.textBody(tx, c)
	}
	b.add(f)
}

// masterShape returns the shape of the master that the shape sh inherits
// from, or nil.
func (b *builder) masterShape(sh *shape) *shape {
	spid, ok := sh.Props.Get(odraw.PropMaster)
	if !ok {
		return nil
	}
	for m := b.master; m != nil; m = m.parent {
		if ms, ok := m.spids[spid]; ok && ms != sh {
			return ms
		}
	}
	return nil
}

// textOf returns the text of the shape: its own, or a text of the slide
// list.
func (b *builder) textOf(sh *shape) *text {
	if sh.text != nil {
		return sh.text
	}
	if sh.textRef >= 0 && sh.textRef < len(b.texts) {
		return &b.texts[sh.textRef]
	}
	return nil
}

// picture returns the picture of the entry pib of the store of pictures,
// from 1, with the color key transparent when keyed is set, or nil. The
// pictures of a presentation that a password protects are left out.
func (d *Document) picture(pib int, key uint32, keyed bool) *drawingml.Picture {
	if d.stream.key != nil {
		return nil
	}
	return d.pics.Get(d.ctx, pib, key, keyed)
}

// page is a slide on a page.
type page struct {
	scene *drawingml.Scene
	w, h  float64
	n     int
	// figures are the figures of the scene, for the tests.
	figures []*drawingml.Figure
}

// The interfaces that a page implements.
var _ ivy.Page = (*page)(nil)

// Size satisfies the ivy.Page interface: the size of the slide in points.
func (p *page) Size() (float32, float32) {
	return float32(p.w), float32(p.h)
}

// Render satisfies the ivy.Page interface.
func (p *page) Render(ctx context.Context, dst *okage.Pixmap, t okage.Transform) (err error) {
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("rendering slide %d: %w", p.n+1, err)
	}
	defer func() {
		if r := recover(); r != nil {
			err = &ivy.PartialError{Err: fmt.Errorf("rendering slide %d: panic: %v: %w", p.n+1, r, ivy.ErrFormat)}
		}
	}()
	// the slide is clipped to its box, as a slide show shows it
	dev := display.NewOkage(dst, t)
	if box, ok := okage.RectFromXYWH(0, 0, float32(p.w), float32(p.h)); ok {
		dev.ClipPath(okage.PathFromRect(box), okage.FillRuleWinding)
	}
	err = p.scene.Draw(ctx, dev, t)
	if pe, ok := errors.AsType[*ivy.PartialError](err); ok {
		return &ivy.PartialError{Err: fmt.Errorf("rendering slide %d: %w", p.n+1, pe.Err)}
	}
	if err != nil {
		return fmt.Errorf("rendering slide %d: %w", p.n+1, err)
	}
	return nil
}
