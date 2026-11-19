// Package drawpage puts the pages of the presentations and the drawings of
// OpenDocument on pages: Impress and Draw, odp, otp, odg and otg.
//
// Each draw:page is one page, at the size of the page layout of its master
// page. A page that a presentation hides is left out. The page draws its
// background, the shapes of its master page and its own shapes onto the
// display list of D10, with the fills, the lines, the pictures and the text
// of internal/drawingml. A custom shape evaluates its enhanced geometry, and
// a shape of a type without a path takes the preset shape of DrawingML that
// looks the same. The text goes through the layout of the ebooks, as the
// text of a slide of PowerPoint does. internal/chart draws the charts of
// the embedded objects with zuzu (D24).
//
// A page shows the footer, the header, the date and the number of its
// master page when its style asks for them, with the texts that the page
// names. A date of the source current-date stays empty. A picture shows the
// part that fo:clip leaves. Shapes draws the shapes of a text of Writer,
// each in a box of its own, for the layout.
//
// The package is apart from the package odf so that its names do not
// collide with the code of the texts and the spreadsheets.
package drawpage

import (
	"context"
	"encoding/base64"
	"fmt"
	"log/slog"
	"math"
	"path"
	"strings"
	"sync"

	"github.com/xo/okage"
	"github.com/xo/okage/roxmltree"

	"github.com/xo/ivy"
	"github.com/xo/ivy/internal/chart"
	"github.com/xo/ivy/internal/display"
	"github.com/xo/ivy/internal/drawingml"
	"github.com/xo/ivy/internal/fontdb"
	"github.com/xo/ivy/internal/ooxml"
)

// The sizes of a page without a page layout, in points: the slide of 28 by
// 15.75 cm of Impress, and the A4 portrait of Draw.
const (
	slideWidth    = 28 * 72 / 2.54
	slideHeight   = 15.75 * 72 / 2.54
	drawingWidth  = 21 * 72 / 2.54
	drawingHeight = 29.7 * 72 / 2.54
)

// maxPictures is the number of pictures that a document keeps decoded.
const maxPictures = 64

// Options are the options of Open.
type Options struct {
	// Config gives the limits, the logger and the fonts.
	Config ivy.Config
	// Read reads an entry of the package, and decrypts it if the file is
	// encrypted.
	Read func(ctx context.Context, name string) ([]byte, error)
	// Presentation leaves out the pages that the presentation hides.
	Presentation bool
}

// Document is the pages of a presentation or a drawing. It is safe for
// concurrent use.
type Document struct {
	opts  Options
	lim   ivy.Limits
	log   *slog.Logger
	st    *styles
	pages []pageRef
	// decls holds the texts of presentation:footer-decl,
	// presentation:header-decl and presentation:date-time-decl, by their
	// local name and their name, such as "footer-decl/ftr1".
	decls map[string]string

	mu       sync.Mutex
	pictures map[string]picture
}

// picture is a picture of the document. raster is set for a picture of
// pixels, whose size in pixels and resolution give its size in points.
type picture struct {
	pic    *drawingml.Picture
	raster bool
}

// pageRef is a page of the document.
type pageRef struct {
	n roxmltree.Node
	// number is the number of the page, from 1, with the hidden pages.
	number int
}

// Open reads content.xml and styles.xml. It returns an error that wraps
// ivy.ErrFormat if the content cannot be read.
func Open(ctx context.Context, opts Options) (d *Document, err error) {
	defer func() {
		if r := recover(); r != nil {
			d, err = nil, fmt.Errorf("reading the pages: panic: %v: %w", r, ivy.ErrFormat)
		}
	}()
	d = &Document{
		opts:     opts,
		lim:      opts.Config.Limits.WithDefaults(),
		log:      opts.Config.Log(),
		st:       newStyles(),
		pictures: map[string]picture{},
		decls:    map[string]string{},
	}
	content, err := d.parse(ctx, "content.xml")
	if err != nil {
		return nil, err
	}
	d.st.add(content.RootElement())
	if styles, err := d.parse(ctx, "styles.xml"); err == nil {
		d.st.add(styles.RootElement())
	} else {
		d.log.Debug("odf: styles left out", "err", err)
	}
	body, ok := firstChild(content.RootElement(), nsOffice, "body")
	if !ok {
		return nil, fmt.Errorf("odf content without a body: %w", ivy.ErrFormat)
	}
	number := 0
	for kind := range body.Children() {
		if !kind.IsElement() {
			continue
		}
		for p := range kind.Children() {
			if uri, _ := p.TagName().Namespace(); p.IsElement() && uri == nsPresentation {
				d.decl(p)
			}
			if !is(p, nsDraw, "page") {
				continue
			}
			number++
			if len(d.pages) >= d.lim.MaxObjects {
				return nil, fmt.Errorf("more than %d pages: %w", d.lim.MaxObjects, ivy.ErrLimit)
			}
			if opts.Presentation && d.hidden(p) {
				continue
			}
			d.pages = append(d.pages, pageRef{n: p, number: number})
		}
	}
	return d, nil
}

// decl reads a declaration of the text of the footer, the header or the
// date of the pages. A date of the source current-date is the date of the
// display, which ivy does not draw: its text is empty.
func (d *Document) decl(n roxmltree.Node) {
	name := n.TagName().Name()
	switch name {
	case "footer-decl", "header-decl":
	case "date-time-decl":
		if at(n, nsPresentation, "source") == "current-date" {
			return
		}
	default:
		return
	}
	if len(d.decls) < d.lim.MaxObjects {
		d.decls[name+"/"+at(n, nsPresentation, "name")] = nodeText(n)
	}
}

// parse reads and parses the entry name.
func (d *Document) parse(ctx context.Context, name string) (*roxmltree.Document, error) {
	buf, err := d.opts.Read(ctx, name)
	if err != nil {
		return nil, err
	}
	doc, err := ooxml.ParseXML(buf, d.lim)
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", name, err)
	}
	return doc, nil
}

// hidden reports whether the style of the page p hides it.
func (d *Document) hidden(p roxmltree.Node) bool {
	pp := d.st.props(d.st.chain("drawing-page", at(p, nsDraw, "style-name")), "drawing-page-properties", nil)
	v := pp.get("presentation:visibility")
	return v == "hidden" || pp.get("presentation:display") == "false"
}

// Count returns the number of pages.
func (d *Document) Count() int {
	return len(d.pages)
}

// Page returns the page n, from 0.
func (d *Document) Page(ctx context.Context, n int) (pg ivy.Page, err error) {
	if n < 0 || n >= len(d.pages) {
		return nil, fmt.Errorf("page %d of %d: %w", n+1, len(d.pages), ivy.ErrPageRange)
	}
	defer func() {
		if r := recover(); r != nil {
			pg, err = nil, fmt.Errorf("reading page %d: panic: %v: %w", n+1, r, ivy.ErrFormat)
		}
	}()
	ref := d.pages[n]
	b := &builder{
		d:      d,
		ctx:    ctx,
		st:     d.st,
		scene:  drawingml.NewScene(d.opts.Config),
		number: ref.number,
		log:    d.log,
	}
	master, hasMaster := d.st.masters[at(ref.n, nsDraw, "master-page-name")]
	if !hasMaster && d.st.hasMaster {
		master, hasMaster = d.st.firstMaster, true
	}
	w, h := drawingWidth, drawingHeight
	if d.opts.Presentation {
		w, h = slideWidth, slideHeight
	}
	if hasMaster {
		if l, ok := d.st.layouts[at(master, nsStyle, "page-layout-name")]; ok {
			if p, ok := firstChild(l, nsStyle, "page-layout-properties"); ok {
				if v, ok := length(at(p, nsFO, "page-width")); ok && v > 0 {
					w = min(v, 1e5)
				}
				if v, ok := length(at(p, nsFO, "page-height")); ok && v > 0 {
					h = min(v, 1e5)
				}
			}
		}
	}
	b.w, b.h = w, h
	b.build(ref.n, master, hasMaster)
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("reading page %d: %w", n+1, err)
	}
	return &page{scene: b.scene, w: w, h: h, n: n}, nil
}

// builder builds the scene of a page.
type builder struct {
	d      *Document
	ctx    context.Context
	st     *styles
	scene  *drawingml.Scene
	number int
	log    *slog.Logger
	// w and h are the size of the page.
	w, h float64
	// shapes counts the shapes of the page.
	shapes int
	// noText leaves out the text of the shapes.
	noText bool
	// page is the draw:page, and pageProps the properties of its style.
	// They give the texts of the fields of a master page.
	page      roxmltree.Node
	pageProps props
}

// build adds the background, the shapes of the master page and the shapes
// of the page p to the scene.
func (b *builder) build(p, master roxmltree.Node, hasMaster bool) {
	dp := b.st.props(b.st.chain("drawing-page", at(p, nsDraw, "style-name")), "drawing-page-properties", nil)
	b.page, b.pageProps = p, dp
	var mp props
	if hasMaster {
		mp = b.st.props(b.st.chain("drawing-page", at(master, nsDraw, "style-name")), "drawing-page-properties", nil)
	}
	switch {
	case dp.get("draw:fill") != "" && dp.get("draw:fill") != "none":
		b.background(dp)
	case hasMaster && dp.get("presentation:background-visible") != "false":
		b.background(mp)
	}
	if hasMaster && dp.get("presentation:background-objects-visible") != "false" {
		b.children(master, 0, true)
	}
	b.children(p, 0, false)
}

// background adds the fill of the drawing page properties pp over the page.
func (b *builder) background(pp props) {
	paint := b.fill(pp, false)
	if !paint.Visible() {
		return
	}
	box, ok := okage.RectFromXYWH(0, 0, float32(b.w), float32(b.h))
	if !ok {
		return
	}
	b.scene.AddFigure(&drawingml.Figure{
		Transform: okage.IdentityTransform(),
		W:         b.w,
		H:         b.h,
		Paths:     []drawingml.Path{{Path: okage.PathFromRect(box)}},
		Fill:      paint,
		Line:      drawingml.Line{Paint: drawingml.Paint{Kind: drawingml.PaintNone}},
	})
}

// children adds the shapes among the children of n. master leaves out the
// presentation objects of a master page.
func (b *builder) children(n roxmltree.Node, depth int, master bool) {
	if depth > min(b.d.lim.MaxDepth, 256) {
		b.log.Warn("odf: groups nested too deep", "err", ivy.ErrLimit)
		return
	}
	for c := range n.Children() {
		if !c.IsElement() {
			continue
		}
		if b.ctx.Err() != nil {
			return
		}
		uri, _ := c.TagName().Namespace()
		if uri != nsDraw {
			continue
		}
		if b.shapes >= b.d.lim.MaxObjects {
			b.log.Warn("odf: too many shapes", "err", ivy.ErrLimit)
			return
		}
		b.shapes++
		if master && hasAt(c, nsPresentation, "class") && !b.showField(at(c, nsPresentation, "class")) {
			continue
		}
		if at(c, nsPresentation, "placeholder") == "true" {
			// an empty placeholder draws nothing in a slide show
			continue
		}
		b.shape(c, depth, master)
	}
}

// showField reports whether a page shows the presentation object of its
// master page of the class, such as "footer": the page shows a footer, a
// header, a date or its number when the style of the page asks for it,
// with presentation:display-footer and the like.
func (b *builder) showField(class string) bool {
	switch class {
	case "footer", "header", "date-time", "page-number":
		return b.pageProps.get("presentation:display-"+class) == "true"
	}
	return false
}

// field returns the text of a field of a master page, such as
// presentation:footer, for the page: the text of the declaration that the
// page names.
func (b *builder) field(name string) string {
	switch name {
	case "footer", "header", "date-time":
		return b.d.decls[name+"-decl/"+at(b.page, nsPresentation, "use-"+name+"-name")]
	}
	return ""
}

// shape adds the shape n.
func (b *builder) shape(n roxmltree.Node, depth int, master bool) {
	switch n.TagName().Name() {
	case "g":
		b.children(n, depth+1, master)
	case "frame":
		b.frame(n)
	case "rect", "caption", "text-box":
		b.geometric(n, "rect")
	case "ellipse", "circle":
		b.ellipse(n)
	case "custom-shape":
		b.custom(n)
	case "polygon", "polyline", "path", "regular-polygon":
		b.polygon(n)
	case "line", "connector":
		b.line(n)
	case "a":
		// a link around shapes
		b.children(n, depth+1, master)
	default:
		b.log.Debug("odf: shape left out", "element", n.TagName().Name())
	}
}

// box returns the box of the shape n: its size, and the transform of the box
// from the origin onto the page.
func (b *builder) box(n roxmltree.Node) (w, h float64, m okage.Transform) {
	x, _ := length(at(n, nsSVG, "x"))
	y, _ := length(at(n, nsSVG, "y"))
	w, _ = length(at(n, nsSVG, "width"))
	h, _ = length(at(n, nsSVG, "height"))
	w, h = max(w, 0), max(h, 0)
	m = okage.TransformFromTranslate(float32(x), float32(y))
	if t := at(n, nsDraw, "transform"); t != "" {
		m = parseTransform(t).PreConcat(m)
	}
	return w, h, m
}

// parseTransform parses draw:transform, such as "rotate (0.5) translate
// (2cm 3cm)". The transforms apply in the order of the list, and a positive
// rotation turns counterclockwise.
func parseTransform(s string) okage.Transform {
	m := okage.IdentityTransform()
	for range 16 {
		s = strings.TrimSpace(s)
		open := strings.IndexByte(s, '(')
		end := strings.IndexByte(s, ')')
		if open < 0 || end < open {
			break
		}
		name := strings.TrimSpace(s[:open])
		args := strings.FieldsFunc(s[open+1:end], func(r rune) bool { return r == ' ' || r == ',' })
		s = s[end+1:]
		num := func(i int) float64 {
			if i >= len(args) {
				return 0
			}
			if v, ok := length(args[i]); ok {
				return v
			}
			v, _ := float(args[i])
			return v
		}
		var t okage.Transform
		switch name {
		case "rotate":
			t = okage.TransformFromRotate(float32(-num(0) * 180 / math.Pi))
		case "translate":
			t = okage.TransformFromTranslate(float32(num(0)), float32(num(1)))
		case "scale":
			sy := num(0)
			if len(args) > 1 {
				sy = num(1)
			}
			t = okage.TransformFromScale(float32(num(0)), float32(sy))
		case "skewX":
			t = okage.TransformFromSkew(float32(math.Tan(-num(0))), 0)
		case "skewY":
			t = okage.TransformFromSkew(0, float32(math.Tan(-num(0))))
		case "matrix":
			t = okage.TransformFromRow(float32(num(0)), float32(num(1)), float32(num(2)), float32(num(3)), float32(num(4)), float32(num(5)))
		default:
			continue
		}
		if t.IsFinite() {
			m = t.PreConcat(m)
		}
	}
	return m
}

// shapeStyle returns the chain of the style of the shape n: its graphic
// style, then its presentation style over it.
func (b *builder) shapeStyle(n roxmltree.Node) []roxmltree.Node {
	var chain []roxmltree.Node
	if name := at(n, nsDraw, "style-name"); name != "" {
		chain = b.st.chain("graphic", name)
	}
	if name := at(n, nsPresentation, "style-name"); name != "" {
		chain = append(chain, b.st.chain("presentation", name)...)
	}
	if len(chain) == 0 {
		chain = b.st.chain("graphic", "")
	}
	return chain
}

// outlinePrefix returns the prefix of the styles of the levels of an
// outline object, such as "Focus-outline", from its presentation style.
func (b *builder) outlinePrefix(n roxmltree.Node) string {
	name := at(n, nsPresentation, "style-name")
	for range maxStyleDepth {
		if base, ok := strings.CutSuffix(name, "-outline1"); ok {
			return base + "-outline"
		}
		s, ok := b.st.style("presentation", name)
		if !ok {
			return ""
		}
		name = at(s, nsStyle, "parent-style-name")
	}
	return ""
}

// figure returns the figure of a shape with its style, without its paths.
func (b *builder) figure(n roxmltree.Node, w, h float64, m okage.Transform, frame bool) (*drawingml.Figure, []roxmltree.Node, props) {
	chain := b.shapeStyle(n)
	gp := b.st.props(chain, "graphic-properties", nil)
	f := &drawingml.Figure{
		Transform:     m,
		W:             w,
		H:             h,
		Fill:          b.fill(gp, !frame),
		Line:          b.stroke(gp, !frame),
		Shadow:        b.shadow(gp),
		TextTransform: m,
	}
	return f, chain, gp
}

// textOf returns the text of the shape n, with the style chain and the
// graphic properties gp of the shape. holder is the element that holds the
// paragraphs.
func (b *builder) textOf(n, holder roxmltree.Node, chain []roxmltree.Node, gp props) *drawingml.TextBody {
	if b.noText {
		return nil
	}
	base := &textBase{chain: chain, body: bodyStyle(gp)}
	if name := at(n, nsDraw, "text-style-name"); name != "" {
		base.chain = append(append([]roxmltree.Node(nil), chain...), b.st.chain("paragraph", name)...)
	}
	if at(n, nsPresentation, "class") == "outline" {
		base.outline = b.outlinePrefix(n)
	}
	base.list, base.hasList = listStyleOf(chain)
	tb := b.textBody(holder, base)
	if tb.Empty() {
		return nil
	}
	return tb
}

// bodyStyle returns the style of the text body of a shape from its graphic
// properties gp.
func bodyStyle(gp props) drawingml.BodyStyle {
	var s drawingml.BodyStyle
	all, hasAll := length(gp.get("fo:padding"))
	for i, side := range [...]string{"left", "top", "right", "bottom"} {
		v, ok := length(gp.get("fo:padding-" + side))
		if !ok && hasAll {
			v, ok = all, true
		}
		if !ok {
			v = 0
		}
		s.Insets[i].Set(min(max(v, 0), 1e4))
	}
	switch gp.get("draw:textarea-vertical-align") {
	case "middle":
		s.Anchor.Set("ctr")
	case "bottom":
		s.Anchor.Set("b")
	default:
		s.Anchor.Set("t")
	}
	if gp.get("draw:auto-grow-width") == "true" {
		s.Wrap.Set(false)
	}
	if strings.HasPrefix(gp.get("style:writing-mode"), "tb") {
		s.Vert.Set("vert")
	}
	return s
}

// geometric adds a shape whose geometry is a preset of DrawingML, such as
// draw:rect.
func (b *builder) geometric(n roxmltree.Node, preset string) {
	w, h, m := b.box(n)
	f, chain, gp := b.figure(n, w, h, m, false)
	g, _ := drawingml.Preset(preset)
	var adjust []drawingml.Guide
	if preset == "roundRect" || preset == "rect" && hasAt(n, nsDraw, "corner-radius") {
		g, _ = drawingml.Preset("roundRect")
		r, _ := length(at(n, nsDraw, "corner-radius"))
		if ss := min(w, h); ss > 0 {
			adjust = []drawingml.Guide{drawingml.AdjustValue("adj", min(r/ss*100000, 50000))}
		}
	}
	f.Paths, f.TextRect = drawingml.Evaluated(g, adjust, w, h, b.log)
	// the text of a shape of OpenDocument is in the whole box of the
	// shape, as LibreOffice draws it
	f.TextRect, _ = okage.RectFromXYWH(0, 0, float32(max(w, 1e-3)), float32(max(h, 1e-3)))
	f.Text = b.textOf(n, n, chain, gp)
	b.scene.AddFigure(f)
}

// ellipse adds draw:ellipse or draw:circle, whole or in part.
func (b *builder) ellipse(n roxmltree.Node) {
	kind := at(n, nsDraw, "kind")
	if kind == "" || kind == "full" {
		b.geometric(n, "ellipse")
		return
	}
	w, h, m := b.box(n)
	f, chain, gp := b.figure(n, w, h, m, false)
	start, _ := angleValue(at(n, nsDraw, "start-angle"))
	end, _ := angleValue(at(n, nsDraw, "end-angle"))
	e := &geometry{vw: max(w, 1e-3), vh: max(h, 1e-3), w: w, h: h, names: map[string]int{}}
	pb := &pathBuilder{e: e, sx: 1, sy: 1, pb: okage.NewPathBuilder(), fill: true, stroke: true}
	cx, cy, rx, ry := w/2, h/2, w/2, h/2
	// the angles are counterclockwise on the page
	t0 := -start * math.Pi / 180
	sweep := -(end - start) * math.Pi / 180
	for sweep >= 0 {
		sweep -= 2 * math.Pi
	}
	switch kind {
	case "section":
		pb.moveTo(cx, cy)
		pb.arc(cx, cy, rx, ry, t0, sweep, false)
		pb.close()
	case "cut":
		pb.arc(cx, cy, rx, ry, t0, sweep, true)
		pb.close()
	default:
		pb.arc(cx, cy, rx, ry, t0, sweep, true)
		pb.fill = false
	}
	pb.end()
	f.Paths = pb.paths
	f.Text = b.textOf(n, n, chain, gp)
	b.scene.AddFigure(f)
}

// angleValue parses an angle of OpenDocument in degrees: a number of
// degrees, or a number with the unit deg, rad or grad.
func angleValue(s string) (float64, bool) {
	s = strings.TrimSpace(s)
	scale := 1.0
	switch {
	case strings.HasSuffix(s, "grad"):
		s, scale = strings.TrimSuffix(s, "grad"), 0.9
	case strings.HasSuffix(s, "rad"):
		s, scale = strings.TrimSuffix(s, "rad"), 180/math.Pi
	case strings.HasSuffix(s, "deg"):
		s = strings.TrimSuffix(s, "deg")
	}
	v, ok := float(s)
	return v * scale, ok
}

// custom adds draw:custom-shape: the paths of its enhanced geometry, or the
// preset shape of its type.
func (b *builder) custom(n roxmltree.Node) {
	w, h, m := b.box(n)
	g, ok := firstChild(n, nsDraw, "enhanced-geometry")
	if !ok {
		b.geometric(n, "rect")
		return
	}
	flipH := at(g, nsDraw, "mirror-horizontal") == "true"
	flipV := at(g, nsDraw, "mirror-vertical") == "true"
	if flipH || flipV {
		sx, sy := float32(1), float32(1)
		if flipH {
			sx = -1
		}
		if flipV {
			sy = -1
		}
		m = m.PreTranslate(float32(w/2), float32(h/2)).PreScale(sx, sy).PreTranslate(float32(-w/2), float32(-h/2))
	}
	f, chain, gp := b.figure(n, w, h, m, false)
	if flipV {
		f.TextTransform = f.Transform.PreRotateAt(180, float32(w/2), float32(h/2))
	} else if flipH {
		f.TextTransform = f.Transform.PreTranslate(float32(w), 0).PreScale(-1, 1)
	}
	e := parseGeometry(g, w, h)
	box, _ := okage.RectFromXYWH(0, 0, float32(max(w, 1e-3)), float32(max(h, 1e-3)))
	f.TextRect = box
	if paths, ok := e.paths(); ok {
		f.Paths = paths
		if r, ok := e.textRect(); ok {
			f.TextRect = r
		}
	} else if pg, adjust, ok := presetGeometry(at(g, nsDraw, "type"), e.modifiers); ok {
		f.Paths, f.TextRect = drawingml.Evaluated(pg, adjust, w, h, b.log)
	} else {
		b.log.Debug("odf: custom shape without a path drawn as a rectangle", "type", at(g, nsDraw, "type"))
		f.Paths = []drawingml.Path{{Path: okage.PathFromRect(box), Stroke: true}}
	}
	f.Text = b.textOf(n, n, chain, gp)
	b.scene.AddFigure(f)
}

// polygon adds draw:polygon, draw:polyline, draw:path or
// draw:regular-polygon. Their points are in the viewBox, which maps onto
// the box of the shape.
func (b *builder) polygon(n roxmltree.Node) {
	w, h, m := b.box(n)
	f, chain, gp := b.figure(n, w, h, m, false)
	var p *okage.Path
	var ok bool
	switch n.TagName().Name() {
	case "polygon":
		p, ok = pointsPath(points(n), true)
	case "polyline":
		p, ok = pointsPath(points(n), false)
		f.Fill = drawingml.Paint{Kind: drawingml.PaintNone}
	case "path":
		p, ok = svgPath(at(n, nsSVG, "d"))
	case "regular-polygon":
		p, ok = regularPolygon(n, w, h)
	}
	if !ok {
		return
	}
	if n.TagName().Name() != "regular-polygon" {
		if v := numbers(at(n, nsSVG, "viewBox")); len(v) == 4 && v[2] > 0 && v[3] > 0 {
			t := okage.TransformFromScale(float32(w/v[2]), float32(h/v[3])).PreTranslate(float32(-v[0]), float32(-v[1]))
			if q, ok := p.Transform(t); ok {
				p = q
			}
		}
	}
	f.Paths = []drawingml.Path{{Path: p, Stroke: true}}
	f.Text = b.textOf(n, n, chain, gp)
	b.scene.AddFigure(f)
}

// points returns the points of a polygon: draw:points, or svg:points of
// older files.
func points(n roxmltree.Node) string {
	if v := at(n, nsDraw, "points"); v != "" {
		return v
	}
	return at(n, nsSVG, "points")
}

// regularPolygon returns the path of draw:regular-polygon in its box.
func regularPolygon(n roxmltree.Node, w, h float64) (*okage.Path, bool) {
	corners, _ := float(at(n, nsDraw, "corners"))
	k := int(min(max(corners, 3), 1000))
	star := at(n, nsDraw, "concave") == "true"
	sharp, _ := percent(at(n, nsDraw, "sharpness"))
	pb := okage.NewPathBuilder()
	cx, cy := w/2, h/2
	steps := k
	if star {
		steps = 2 * k
	}
	for i := range steps {
		a := -math.Pi/2 + 2*math.Pi*float64(i)/float64(steps)
		r := 1.0
		if star && i%2 == 1 {
			r = 1 - sharp
		}
		x, y := float32(cx+cx*r*math.Cos(a)), float32(cy+cy*r*math.Sin(a))
		if i == 0 {
			pb.MoveTo(x, y)
		} else {
			pb.LineTo(x, y)
		}
	}
	pb.Close()
	return pb.Finish()
}

// line adds draw:line or draw:connector, whose ends are on the page.
func (b *builder) line(n roxmltree.Node) {
	x1, _ := length(at(n, nsSVG, "x1"))
	y1, _ := length(at(n, nsSVG, "y1"))
	x2, _ := length(at(n, nsSVG, "x2"))
	y2, _ := length(at(n, nsSVG, "y2"))
	m := okage.IdentityTransform()
	if t := at(n, nsDraw, "transform"); t != "" {
		m = parseTransform(t)
	}
	x0, y0 := min(x1, x2), min(y1, y2)
	w, h := math.Abs(x2-x1), math.Abs(y2-y1)
	f, chain, gp := b.figure(n, w, h, m, false)
	f.Fill = drawingml.Paint{Kind: drawingml.PaintNone}
	var p *okage.Path
	ok := false
	if d := at(n, nsSVG, "d"); d != "" {
		// the path of a connector is in its viewBox, which is in 1/100 mm
		// of the page
		if q, found := svgPath(d); found {
			if v := numbers(at(n, nsSVG, "viewBox")); len(v) == 4 && v[2] > 0 && v[3] > 0 {
				t := okage.TransformFromTranslate(float32(x0), float32(y0)).
					PreScale(float32(w/v[2]), float32(h/v[3])).PreTranslate(float32(-v[0]), float32(-v[1]))
				if w == 0 || h == 0 {
					t = okage.TransformFromScale(72/2540.0, 72/2540.0)
				}
				p, ok = q.Transform(t)
			}
		}
	}
	if !ok {
		pb := okage.NewPathBuilder()
		pb.MoveTo(float32(x1), float32(y1))
		pb.LineTo(float32(x2), float32(y2))
		p, ok = pb.Finish()
	}
	if !ok {
		return
	}
	f.Paths = []drawingml.Path{{Path: p, Fill: drawingml.FillNone, Stroke: true}}
	f.W, f.H = max(x0+w, 1e-3), max(y0+h, 1e-3)
	f.Text = b.textOf(n, n, chain, gp)
	if f.Text != nil {
		f.TextRect, _ = okage.RectFromXYWH(float32(x0), float32(y0), float32(max(w, 1)), float32(max(h, 1)))
	}
	b.scene.AddFigure(f)
}

// frame adds draw:frame: a text box, a picture, a table or an object.
func (b *builder) frame(n roxmltree.Node) {
	w, h, m := b.box(n)
	for c := range n.Children() {
		if !c.IsElement() {
			continue
		}
		switch {
		case is(c, nsDraw, "text-box"):
			f, chain, gp := b.figure(n, w, h, m, true)
			box, _ := okage.RectFromXYWH(0, 0, float32(max(w, 1e-3)), float32(max(h, 1e-3)))
			f.Paths = []drawingml.Path{{Path: okage.PathFromRect(box), Stroke: true}}
			f.Text = b.textOf(n, c, chain, gp)
			b.scene.AddFigure(f)
			return
		case is(c, nsDraw, "image"):
			if b.image(n, c, w, h, m) {
				return
			}
		case is(c, nsTable, "table"):
			b.table(c, w, h, m)
			return
		case is(c, nsDraw, "object"), is(c, nsDraw, "object-ole"):
			if b.drawChart(c, w, h, m) {
				return
			}
			// the picture that replaces the object, which a later child
			// draw:image holds
			b.log.Debug("odf: object left out", "href", at(c, nsXLink, "href"))
		}
	}
}

// drawChart draws the chart of the object of a frame, in the box of w by
// h points of the space m. internal/chart draws it with zuzu (D24), with
// the fonts of the program. It returns false for an object that is not a
// chart, whose replacement picture the frame draws next.
func (b *builder) drawChart(object roxmltree.Node, w, h float64, m okage.Transform) bool {
	href := at(object, nsXLink, "href")
	name := path.Clean(strings.TrimPrefix(href, "./"))
	if name == "." || strings.HasPrefix(name, "..") || strings.HasPrefix(name, "/") || strings.Contains(href, "://") {
		return false
	}
	rect, ok := okage.RectFromXYWH(0, 0, float32(w), float32(h))
	if !ok {
		return false
	}
	buf, err := b.d.opts.Read(b.ctx, name+"/content.xml")
	if err != nil {
		return false
	}
	cfg := b.d.opts.Config
	ch, err := chart.ParseODF(buf, chart.Options{Config: cfg, Fonts: fontdb.Of(cfg), Width: w, Height: h})
	if err != nil {
		b.log.Debug("odf: object that is not a chart", "href", href, "err", err)
		return false
	}
	b.scene.Add(m, func(ctx context.Context, dev display.Device, _ float64) error {
		return ch.Draw(ctx, dev, rect)
	})
	return true
}

// image adds the picture of draw:image in the frame n. It returns false if
// the frame holds no picture that the package reads, so that the next
// child of the frame can draw.
func (b *builder) image(n, img roxmltree.Node, w, h float64, m okage.Transform) bool {
	p := b.picture(img)
	if p.pic == nil {
		return false
	}
	pw, ph, ok := p.pic.Size(b.ctx)
	if !ok {
		return false
	}
	f, _, gp := b.figure(n, w, h, m, true)
	box, _ := okage.RectFromXYWH(0, 0, float32(max(w, 1e-3)), float32(max(h, 1e-3)))
	f.Paths = []drawingml.Path{{Path: okage.PathFromRect(box), Stroke: true}}
	f.Fill = drawingml.Paint{Kind: drawingml.PaintPicture, Picture: p.pic, Opacity: 1}
	if o, ok := percent(gp.get("draw:image-opacity")); ok {
		f.Fill.Opacity = min(max(o, 0), 1)
	}
	if p.raster {
		rx, ry := p.pic.Resolution()
		pw, ph = pw*72/rx, ph*72/ry
	}
	f.Fill.Crop = clip(gp.get("fo:clip"), pw, ph)
	b.scene.AddFigure(f)
	return true
}

// clip returns the part of a picture of w by h points that fo:clip shows,
// such as "rect(1cm, 2cm, 1cm, 2cm)": the insets of its top, its right,
// its bottom and its left, which LibreOffice gives in the size of the
// picture. A value of auto is 0.
func clip(s string, w, h float64) drawingml.Insets {
	body, ok := strings.CutPrefix(strings.TrimSpace(s), "rect(")
	if !ok || !(w > 0) || !(h > 0) {
		return drawingml.Insets{}
	}
	body, _ = strings.CutSuffix(strings.TrimSpace(body), ")")
	f := strings.FieldsFunc(body, func(r rune) bool { return r == ',' || r == ' ' })
	if len(f) != 4 {
		return drawingml.Insets{}
	}
	var v [4]float64
	for i, part := range f {
		if l, ok := length(part); ok {
			v[i] = l
		}
	}
	in := drawingml.Insets{T: v[0] / h, R: v[1] / w, B: v[2] / h, L: v[3] / w}
	// a clip that leaves nothing of the picture is not drawn
	if !(in.L+in.R < 1) || !(in.T+in.B < 1) || min(in.L, in.R, in.T, in.B) < 0 {
		return drawingml.Insets{}
	}
	return in
}

// isRaster reports whether data is a picture of pixels: a PNG, a JPEG, a
// GIF, a BMP, a TIFF or a WebP file.
func isRaster(data []byte) bool {
	for _, magic := range [...]string{"\x89PNG", "\xff\xd8\xff", "GIF8", "BM", "II*\x00", "MM\x00*"} {
		if strings.HasPrefix(string(data[:min(len(data), 8)]), magic) {
			return true
		}
	}
	return len(data) >= 12 && string(data[:4]) == "RIFF" && string(data[8:12]) == "WEBP"
}

// picture returns the picture of draw:image or draw:fill-image: the entry
// of xlink:href, or office:binary-data. Its pic is nil if the element holds
// no picture that the package reads.
func (b *builder) picture(img roxmltree.Node) picture {
	href := strings.TrimPrefix(at(img, nsXLink, "href"), "./")
	if href == "" {
		data, ok := firstChild(img, nsOffice, "binary-data")
		if !ok {
			return picture{}
		}
		text := strings.Map(func(r rune) rune {
			if r == ' ' || r == '\n' || r == '\r' || r == '\t' {
				return -1
			}
			return r
		}, nodeText(data))
		if int64(len(text)) > b.d.lim.MaxMemory {
			return picture{}
		}
		buf, err := base64.StdEncoding.DecodeString(text)
		if err != nil {
			b.log.Debug("odf: picture left out", "err", err)
			return picture{}
		}
		return picture{pic: drawingml.NewPicture(buf, b.d.opts.Config), raster: isRaster(buf)}
	}
	if strings.Contains(href, ":") {
		// a link out of the package, which ivy never reads (D1)
		return picture{}
	}
	d := b.d
	d.mu.Lock()
	p, ok := d.pictures[href]
	d.mu.Unlock()
	if ok {
		return p
	}
	buf, err := d.opts.Read(b.ctx, href)
	if err != nil {
		b.log.Debug("odf: picture left out", "href", href, "err", err)
		return picture{}
	}
	p = picture{pic: drawingml.NewPicture(buf, d.opts.Config), raster: isRaster(buf)}
	d.mu.Lock()
	defer d.mu.Unlock()
	if len(d.pictures) >= maxPictures {
		clear(d.pictures)
	}
	d.pictures[href] = p
	return p
}

// nodeText returns the text of the element n.
func nodeText(n roxmltree.Node) string {
	var sb strings.Builder
	for c := range n.Children() {
		if c.IsText() {
			t, _ := c.Text()
			sb.WriteString(t)
		}
	}
	return sb.String()
}

// page is a page of a drawing or a presentation.
type page struct {
	scene *drawingml.Scene
	w, h  float64
	n     int
}

// The interfaces that a page implements.
var (
	_ ivy.Page       = (*page)(nil)
	_ display.Drawer = (*page)(nil)
)

// Size satisfies the ivy.Page interface: the size of the page in points.
func (p *page) Size() (float32, float32) {
	return float32(p.w), float32(p.h)
}

// Render satisfies the ivy.Page interface.
func (p *page) Render(ctx context.Context, dst *okage.Pixmap, t okage.Transform) error {
	return p.draw(ctx, display.NewOkage(dst, t), t)
}

// Draw satisfies the display.Drawer interface (D58).
func (p *page) Draw(ctx context.Context, d display.Device, res float32) error {
	d.Save()
	defer d.Restore()
	return p.draw(ctx, d, okage.TransformFromScale(res, res))
}

// draw draws the scene of the page onto dev, whose user space is the space
// of the page in points. t maps that space onto the pixels of the target,
// and sets the resolution of the pictures and the text.
func (p *page) draw(ctx context.Context, dev display.Device, t okage.Transform) (err error) {
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("rendering page %d: %w", p.n+1, err)
	}
	defer func() {
		if r := recover(); r != nil {
			err = &ivy.PartialError{Err: fmt.Errorf("rendering page %d: panic: %v: %w", p.n+1, r, ivy.ErrFormat)}
		}
	}()
	err = p.scene.Draw(ctx, dev, t)
	if pe, ok := asPartial(err); ok {
		return &ivy.PartialError{Err: fmt.Errorf("rendering page %d: %w", p.n+1, pe.Err)}
	}
	if err != nil {
		return fmt.Errorf("rendering page %d: %w", p.n+1, err)
	}
	return nil
}
