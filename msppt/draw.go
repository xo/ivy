package msppt

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"math"

	"github.com/xo/okage"

	"github.com/xo/ivy"
	"github.com/xo/ivy/internal/display"
	"github.com/xo/ivy/internal/drawingml"
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
		if h.typ != rtColorScheme || len(b) < 32 {
			continue
		}
		if found == nil || h.inst == 1 {
			found = b
		}
		if h.inst == 1 {
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
	return drawingml.RGBA{R: float64(r) / 255, G: float64(g) / 255, B: float64(b) / 255, A: 1}
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
	if !ok || (h.typ != rtMainMaster && h.typ != rtSlide) {
		return nil
	}
	lim := d.cfg.Limits
	m := &master{spids: make(map[uint32]*shape)}
	var parentID uint32
	parentID, m.flags = slideAtom(body)
	if h.typ == rtSlide && parentID != 0 && parentID != id {
		// a title master follows a main master
		m.parent = d.master(parentID, depth-1)
	}
	m.scheme, m.hasSheme = colorScheme(body)
	for ch, b := range children(body) {
		if ch.typ == rtTextMasterStyle && int(ch.inst) < txTypes {
			if st, ok := readMasterStyle(b, ch.inst); ok {
				m.styles[ch.inst], m.hasStyle[ch.inst] = st, true
			}
		}
	}
	m.drawing = readDrawing(body, lim.MaxObjects, lim.MaxDepth)
	m.drawing.shapes(func(sh *shape, _ *transform) {
		m.spids[sh.spid] = sh
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
	if !ok || hd.typ != rtSlide {
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
	dr := readDrawing(body, lim.MaxObjects, lim.MaxDepth)
	b.background(dr, flags)
	if flags&slideMasterObjects != 0 && b.master != nil {
		b.masterObjects(b.master, lim.MaxDepth)
	}
	dr.shapes(func(sh *shape, t *transform) {
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
		if p := b.fill(b.chain(bg)); p.Kind != drawingml.PaintNone {
			pt = p
		}
	}
	b.fillPage(pt)
}

// masterObjects draws the shapes of the master m that are not
// placeholders, after the shapes of its main master if it shows them.
func (b *builder) masterObjects(m *master, depth int) {
	if depth <= 0 {
		return
	}
	if m.parent != nil && m.flags&slideMasterObjects != 0 {
		b.masterObjects(m.parent, depth-1)
	}
	lim := b.d.cfg.Limits
	m.drawing.shapes(func(sh *shape, t *transform) {
		if sh.placeholder != 0 || b.scene.Len() >= lim.MaxObjects {
			return
		}
		b.addShape(sh, t)
	})
}

// chain is the properties of a shape, then of the shape of the master that
// it inherits from, then the defaults of the document.
type chain []*props

// chain returns the properties of sh.
func (b *builder) chain(sh *shape) chain {
	c := chain{&sh.props}
	if spid, ok := sh.props.get(0x0301); ok && b.master != nil {
		for m := b.master; m != nil; m = m.parent {
			if ms, ok := m.spids[spid]; ok && ms != sh {
				c = append(c, &ms.props)
				break
			}
		}
	}
	return append(c, &b.d.doc.shapeDefaults)
}

func (c chain) get(id uint16) (uint32, bool) {
	for _, p := range c {
		if v, ok := p.get(id); ok {
			return v, true
		}
	}
	return 0, false
}

func (c chain) value(id uint16, def uint32) uint32 {
	if v, ok := c.get(id); ok {
		return v
	}
	return def
}

func (c chain) complex(id uint16) []byte {
	for _, p := range c {
		if _, ok := p.get(id); ok {
			return p.complex[id]
		}
	}
	return nil
}

// flag returns a bit of a boolean property: from the first set of
// properties that sets its bit of use.
func (c chain) flag(id uint16, bit, use uint32, def bool) bool {
	for _, p := range c {
		if v, ok := p.get(id); ok && v&use != 0 {
			return v&bit != 0
		}
	}
	return def
}

// color resolves an OfficeArtCOLORREF (MS-ODRAW 2.2.2): an index of the
// color scheme, a color of the shape for a system index, or an RGB value.
func (b *builder) color(c chain, v uint32, depth int) drawingml.RGBA {
	switch {
	case v&colorSchemeIndex != 0:
		return b.scheme[v&7]
	case v&colorSysIndex != 0 && depth > 0:
		var base drawingml.RGBA
		switch v & 0xFF {
		case 0xF0, 0xF7, 0xF4:
			base = b.color(c, c.value(propFillColor, 0xFFFFFF), depth-1)
		case 0xF1, 0xF2:
			base = b.color(c, c.value(propLineColor, 0), depth-1)
		case 0xF3:
			base = b.color(c, c.value(propShadowColor, 0x808080), depth-1)
		case 0xF5:
			base = b.color(c, c.value(propFillBackColor, 0xFFFFFF), depth-1)
		case 0xF6:
			base = b.color(c, c.value(0x01C2, 0xFFFFFF), depth-1)
		default:
			return drawingml.Black
		}
		f := float64((v>>16)&0xFF) / 255
		switch (v >> 8) & 0xFF {
		case 0x01:
			base = base.Mix(drawingml.Black, 1-f).WithAlpha(1)
		case 0x02:
			base = base.Mix(drawingml.White, 1-f).WithAlpha(1)
		}
		return base
	}
	return rgb(uint8(v), uint8(v>>8), uint8(v>>16))
}

// fill returns the fill of the properties c.
func (b *builder) fill(c chain) drawingml.Paint {
	if !c.flag(propFillBools, fillFilled, fillUseFilled, true) {
		return drawingml.Paint{Kind: drawingml.PaintNone}
	}
	col := b.color(c, c.value(propFillColor, 0xFFFFFF), 2)
	back := b.color(c, c.value(propFillBackColor, 0xFFFFFF), 2)
	alpha := unit(fixed(c.value(propFillOpacity, 0x10000)))
	switch c.value(propFillType, fillSolid) {
	case fillShade, fillShadeCenter, fillShadeShape, fillShadeScale, fillShadeTitle:
		return b.gradient(c, col, back.WithAlpha(unit(fixed(c.value(propFillBackOpac, 0x10000)))), alpha)
	case fillPicture, fillTexture:
		if pic := b.picture(int(c.value(propFillBlip, 0))); pic != nil {
			pt := drawingml.Paint{Kind: drawingml.PaintPicture, Picture: pic, Opacity: alpha}
			if c.value(propFillType, 0) == fillTexture {
				pt.Tile = &drawingml.Tile{SX: 1, SY: 1}
			}
			return pt
		}
		return drawingml.Solid(col.WithAlpha(alpha))
	case fillPattern:
		return drawingml.Paint{Kind: drawingml.PaintPattern, Pattern: "pct50", Color: col.WithAlpha(alpha), Background: back}
	case fillBackground:
		return drawingml.Solid(b.scheme[0])
	}
	return drawingml.Solid(col.WithAlpha(alpha))
}

// gradient returns the gradient of a shaded fill. The angle of the fill
// turns counterclockwise from a vector that points up, from the back color
// to the fill color. The focus moves the fill color from the end of the
// vector: 0 leaves it at the end, 100 puts it at the start, and 50 in the
// middle with the back color at both ends. A negative focus swaps the two
// colors.
func (b *builder) gradient(c chain, col, back drawingml.RGBA, alpha float64) drawingml.Paint {
	col = col.WithAlpha(col.A * alpha)
	g := drawingml.Gradient{Angle: math.Mod(90-fixed(c.value(propFillAngle, 0))+720, 360)}
	if stops := b.shadeColors(c); len(stops) >= 2 {
		g.Stops = stops
	} else {
		focus := float64(int32(c.value(propFillFocus, 0))) / 100
		peak, edge := col, back
		if focus < 0 {
			peak, edge, focus = back, col, -focus
		}
		switch pos := 1 - focus; {
		case pos >= 1:
			g.Stops = []drawingml.Stop{{Pos: 0, Color: edge}, {Pos: 1, Color: peak}}
		case pos <= 0:
			g.Stops = []drawingml.Stop{{Pos: 0, Color: peak}, {Pos: 1, Color: edge}}
		default:
			g.Stops = []drawingml.Stop{{Pos: 0, Color: edge}, {Pos: pos, Color: peak}, {Pos: 1, Color: edge}}
		}
	}
	switch c.value(propFillType, 0) {
	case fillShadeCenter, fillShadeShape, fillShadeTitle:
		g.Path = "rect"
		g.Focus = drawingml.Insets{
			L: unit(fixed(c.value(propFillToLeft, 0))),
			T: unit(fixed(c.value(propFillToTop, 0))),
			R: 1 - unit(fixed(c.value(propFillToRight, 0))),
			B: 1 - unit(fixed(c.value(propFillToBottom, 0))),
		}
		// a path gradient goes from the focus to the edge
		for i, j := 0, len(g.Stops)-1; i < j; i, j = i+1, j-1 {
			g.Stops[i].Color, g.Stops[j].Color = g.Stops[j].Color, g.Stops[i].Color
		}
	}
	return drawingml.Paint{Kind: drawingml.PaintGradient, Gradient: g}
}

// shadeColors reads the stops of fillShadeColors, an array of a color and a
// position of 16.16 bits each.
func (b *builder) shadeColors(c chain) []drawingml.Stop {
	elems := msoArray(c.complex(propFillShadeCols), 8)
	stops := make([]drawingml.Stop, 0, len(elems))
	for _, e := range elems {
		stops = append(stops, drawingml.Stop{
			Color: b.color(c, le.Uint32(e), 2),
			Pos:   unit(fixed(le.Uint32(e[4:]))),
		})
	}
	return stops
}

// msoArray returns the elements of an IMsoArray (MS-ODRAW 2.2.51). size is
// the size of an element when the array says 0xFFF0.
func msoArray(b []byte, size int) [][]byte {
	if len(b) < 6 {
		return nil
	}
	n := int(le.Uint16(b))
	cb := int(le.Uint16(b[4:]))
	if cb == 0xFFF0 {
		cb = size
	}
	if cb <= 0 || cb > 64 {
		return nil
	}
	n = min(n, (len(b)-6)/cb)
	out := make([][]byte, n)
	for i := range n {
		out[i] = b[6+cb*i : 6+cb*(i+1)]
	}
	return out
}

// unit returns v within [0, 1].
func unit(v float64) float64 {
	if math.IsNaN(v) {
		return 0
	}
	return max(0, min(v, 1))
}

// The values of lineDashing (MS-ODRAW 2.4.11), in multiples of the width.
var dashes = map[uint32][]float64{
	1:  {3, 1},
	2:  {1, 1},
	3:  {3, 1, 1, 1},
	4:  {3, 1, 1, 1, 1, 1},
	5:  {1, 3},
	6:  {4, 3},
	7:  {8, 3},
	8:  {4, 3, 1, 3},
	9:  {8, 3, 1, 3},
	10: {8, 3, 1, 3, 1, 3},
}

// The ends of a line of lineStartArrowhead and lineEndArrowhead (MS-ODRAW
// 2.4.10).
var arrows = map[uint32]string{1: "triangle", 2: "stealth", 3: "diamond", 4: "oval", 5: "arrow"}

// line returns the outline of the properties c.
func (b *builder) line(c chain, def bool) drawingml.Line {
	l := drawingml.DefaultLine()
	if !c.flag(propLineBools, lineLine, lineUseLine, def) {
		return l
	}
	col := b.color(c, c.value(propLineColor, 0), 2)
	col = col.WithAlpha(unit(fixed(c.value(propLineOpacity, 0x10000))))
	l.Paint = drawingml.Solid(col)
	l.Width = float64(c.value(propLineWidth, 9525)) / emuPerPoint
	if l.Width > 1000 {
		l.Width = 1000
	}
	l.Dash = dashes[c.value(propLineDashing, 0)]
	if a, ok := arrows[c.value(propLineStartArrow, 0)]; ok {
		l.Head = drawingml.Arrow{Type: a, W: 1, L: 1}
	}
	if a, ok := arrows[c.value(propLineEndArrow, 0)]; ok {
		l.Tail = drawingml.Arrow{Type: a, W: 1, L: 1}
	}
	return l
}

// shadow returns the shadow of the properties c, or nil.
func (b *builder) shadow(c chain) *drawingml.Shadow {
	if !c.flag(propShadowBools, shadowShadow, shadowUseShdw, false) {
		return nil
	}
	return &drawingml.Shadow{
		Color: b.color(c, c.value(propShadowColor, 0x808080), 2),
		DX:    float64(int32(c.value(propShadowOffsetX, 25400))) / emuPerPoint,
		DY:    float64(int32(c.value(propShadowOffsetY, 25400))) / emuPerPoint,
	}
}

// addShape adds the shape sh, with the transform t of its group, to the
// scene.
func (b *builder) addShape(sh *shape, t *transform) {
	c := b.chain(sh)
	if c.flag(propGroupBools, groupHidden, groupUseHidden, false) {
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
	r := t.apply(a)
	x, y := r.left/unitsPerPoint, r.top/unitsPerPoint
	w, h := (r.right-r.left)/unitsPerPoint, (r.bottom-r.top)/unitsPerPoint
	if w < 0 {
		x, w = x+w, -w
	}
	if h < 0 {
		y, h = y+h, -h
	}
	if !(w < 1e6 && h < 1e6) || math.IsNaN(x) || math.IsNaN(y) {
		return
	}
	rot := math.Mod(fixed(c.value(propRotation, 0)), 360)
	if rot < 0 {
		rot += 360
	}
	if (rot >= 45 && rot < 135) || (rot >= 225 && rot < 315) {
		// the anchor of a shape that turns by about 90 degrees is the box of
		// the shape turned
		cx, cy := x+w/2, y+h/2
		w, h = h, w
		x, y = cx-w/2, cy-h/2
	}
	flipH, flipV := sh.flags&fspFlipH != 0, sh.flags&fspFlipV != 0
	f := &drawingml.Figure{
		Transform:     drawingml.ShapeTransform(x, y, w, h, rot, flipH, flipV),
		W:             w,
		H:             h,
		TextTransform: drawingml.TextTransform(x, y, w, h, rot, flipV),
		// a picture has no outline unless it gives one
		Line:   b.line(c, sh.typ != sptPictureFrame),
		Shadow: b.shadow(c),
	}
	var filled bool
	f.Paths, filled = b.geometry(sh, c, w, h)
	if filled {
		f.Fill = b.fill(c)
	} else {
		f.Fill = drawingml.Paint{Kind: drawingml.PaintNone}
	}
	if pib := int(c.value(propPib, 0)); pib > 0 {
		key, hasKey := c.get(propPictureTransparent)
		if pic := b.keyedPicture(pib, key, hasKey); pic != nil {
			f.Fill = drawingml.Paint{Kind: drawingml.PaintPicture, Picture: pic, Opacity: 1, Crop: drawingml.Insets{
				L: fixed(c.value(propCropLeft, 0)),
				T: fixed(c.value(propCropTop, 0)),
				R: fixed(c.value(propCropRight, 0)),
				B: fixed(c.value(propCropBottom, 0)),
			}}
			if sh.typ == 75 {
				// a picture frame shows the picture over its paths
				box, _ := okage.RectFromXYWH(0, 0, float32(max(w, 1e-3)), float32(max(h, 1e-3)))
				f.Paths = []drawingml.Path{{Path: okage.PathFromRect(box), Stroke: true}}
			}
		}
	}
	if tx := b.textOf(sh); tx != nil {
		// the defaults of the document are for new shapes: the text of a
		// shape does not take them, as LibreOffice and PowerPoint show it
		f.Text = b.textBody(tx, c[:len(c)-1])
	}
	b.add(f)
}

// masterShape returns the shape of the master that the shape sh inherits
// from, or nil.
func (b *builder) masterShape(sh *shape) *shape {
	spid, ok := sh.props.get(0x0301)
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

// picture returns the picture of the entry i of the store of pictures,
// from 1, or nil.
func (b *builder) picture(i int) *drawingml.Picture {
	d := b.d
	if p, ok := d.pictures[i]; ok {
		return p
	}
	d.pictures[i] = nil
	if i < 1 || i > len(d.doc.blips) || d.stream.key != nil {
		return nil
	}
	e := d.doc.blips[i-1]
	if !e.ok {
		return nil
	}
	rec := e.embedded
	if rec == nil {
		if uint64(e.foDelay) >= uint64(len(d.picStream)) {
			return nil
		}
		rec = d.picStream[e.foDelay:]
	}
	data := blipData(rec, d.cfg.Limits.MaxMemory)
	if data == nil {
		return nil
	}
	p := drawingml.NewPicture(data, d.cfg)
	d.pictures[i] = p
	d.rawPictures[i] = data
	return p
}

// pictureKey is a picture of the store with a color that it leaves
// transparent.
type pictureKey struct {
	pib int
	key uint32
}

// keyedPicture returns the picture of the entry pib of the store, with the
// color of the OfficeArtCOLORREF key transparent if hasKey is set. Only a
// raster picture takes the color key.
func (b *builder) keyedPicture(pib int, key uint32, hasKey bool) *drawingml.Picture {
	pic := b.picture(pib)
	if pic == nil || !hasKey || key&0xFF000000 != 0 {
		return pic
	}
	d := b.d
	k := pictureKey{pib: pib, key: key}
	if p, ok := d.keyed[k]; ok {
		return p
	}
	d.keyed[k] = pic
	data, ok := colorKey(d.ctx, d.rawPictures[pib], uint8(key), uint8(key>>8), uint8(key>>16), d.cfg.Limits)
	if !ok {
		return pic
	}
	p := drawingml.NewPicture(data, d.cfg)
	d.keyed[k] = p
	return p
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
