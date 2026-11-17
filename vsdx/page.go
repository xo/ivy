package vsdx

import (
	"context"
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/xo/okage"

	"github.com/xo/ivy"
	"github.com/xo/ivy/internal/display"
	"github.com/xo/ivy/internal/drawingml"
	"github.com/xo/ivy/internal/ooxml"
)

// The default size of a page, US Letter, in inches.
const (
	defaultPageWidth  = 8.5
	defaultPageHeight = 11
)

// builder builds the scene of one page.
type builder struct {
	dr    *drawing
	ctx   context.Context
	lim   ivy.Limits
	scene *drawingml.Scene

	theme  *theme
	colors *drawingml.Colors
	// k is the number of points of the paper for an inch of the drawing:
	// 72 times PageScale over DrawingScale.
	k float64
	// pageW and pageH are the size of the page in inches of the drawing.
	pageW, pageH float64
	// page is the sheet of the page that the shapes are on.
	page                       *sheet
	pageNumber, pageCount      int
	pageName                   string
	variantColor, variantStyle int
	// layers holds the indexes of the layers of the page that are hidden.
	hidden map[string]bool

	// shapes and segs count the shapes and the segments of the paths.
	shapes, segs counter
	errs         []error
	// evalDepth is the depth of the formulas that the builder computes,
	// one inside the other.
	evalDepth int
	// deleted holds the master shapes that an instance of a group deletes.
	deleted map[*shape]bool
	// values caches the values of the cells.
	values map[cellKey]cachedValue

	// chains caches the chains of the shapes, links the master shape of
	// each shape, and virtual the children that an instance of a group
	// inherits from its master.
	chains  map[*shape]*[4]chain
	links   map[*shape]*shape
	virtual map[*shape][]*shape
}

// fault records a part of the page that the builder could not draw.
func (b *builder) fault(err error) {
	if len(b.errs) < 16 {
		b.errs = append(b.errs, err)
	}
}

// buildPage builds the scene of the foreground page n.
func (dr *drawing) buildPage(ctx context.Context, n int) (*page, error) {
	ref := dr.pages[dr.fore[n]]
	b := dr.newBuilder(ctx, n)
	// the background pages draw first, the farthest first
	var stack []*pageRef
	seen := map[*pageRef]bool{ref: true}
	for p := ref; p.backPage != "" && len(stack) < dr.lim.MaxDepth; {
		i, ok := dr.pageIndex(p.backPage)
		if !ok || seen[dr.pages[i]] {
			break
		}
		p = dr.pages[i]
		seen[p] = true
		stack = append(stack, p)
	}
	for i := len(stack) - 1; i >= 0; i-- {
		b.drawPage(stack[i])
	}
	b.drawPage(ref)
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("reading page %d: %w", n+1, err)
	}
	pg := &page{scene: b.scene, w: b.pageW * b.k, h: b.pageH * b.k, n: n}
	if len(b.errs) > 0 {
		pg.err = errors.Join(b.errs...)
	}
	return pg, nil
}

// newBuilder returns the builder of the foreground page n, with the size,
// the scale and the theme of the page.
func (dr *drawing) newBuilder(ctx context.Context, n int) *builder {
	ref := dr.pages[dr.fore[n]]
	b := &builder{
		dr:         dr,
		ctx:        ctx,
		lim:        dr.lim,
		scene:      drawingml.NewScene(dr.cfg),
		page:       &ref.sheet,
		pageNumber: n + 1,
		pageCount:  len(dr.fore),
		pageName:   ref.name,
		hidden:     make(map[string]bool),
		shapes:     counter{max: dr.lim.MaxObjects},
		segs:       counter{max: dr.lim.MaxObjects},
		chains:     make(map[*shape]*[4]chain),
		links:      make(map[*shape]*shape),
		virtual:    make(map[*shape][]*shape),
		deleted:    make(map[*shape]bool),
		values:     make(map[cellKey]cachedValue),
	}
	b.theme = dr.theme
	if ref.part != "" {
		b.theme = dr.pageTheme(ref.part)
	}
	b.colors = &drawingml.Colors{}
	if b.theme != nil {
		b.colors.Theme = b.theme.dml
	}
	b.pageW = parseNum(b.pageCell("PageWidth"), defaultPageWidth)
	b.pageH = parseNum(b.pageCell("PageHeight"), defaultPageHeight)
	ratio := 1.0
	if ps, ds := parseNum(b.pageCell("PageScale"), 1), parseNum(b.pageCell("DrawingScale"), 1); ps > 0 && ds > 0 {
		ratio = ps / ds
	}
	b.k = 72 * ratio
	if !(b.pageW > 0) || !(b.pageH > 0) || b.pageW*b.k > 1e6 || b.pageH*b.k > 1e6 || !(b.k > 0) || b.k > 1e6 {
		b.pageW, b.pageH, b.k = defaultPageWidth, defaultPageHeight, 72
	}
	b.variantColor = int(parseNum(b.pageCell("VariationColorIndex"), 0))
	b.variantStyle = int(parseNum(b.pageCell("VariationStyleIndex"), 0))
	if b.variantColor > 100 {
		b.variantColor = 0
	}
	if b.variantStyle > 100 {
		b.variantStyle = 0
	}
	return b
}

// pageCell returns the value of the cell of the page sheet with the name.
func (b *builder) pageCell(name string) string {
	return b.page.cells[name].v
}

// drawPage draws the shapes of the page p.
func (b *builder) drawPage(p *pageRef) {
	if p.part == "" {
		return
	}
	cnt := &counter{max: b.lim.MaxObjects}
	shapes, err := b.dr.pageContents(p.part, cnt)
	if err != nil {
		b.fault(err)
		return
	}
	if cnt.over() {
		b.fault(fmt.Errorf("page of more than %d objects: %w", b.lim.MaxObjects, ivy.ErrLimit))
	}
	b.readLayers(&p.sheet)
	// the page has its y axis up, from the bottom of the page
	m := affine{a: b.k, d: -b.k, f: b.pageH * b.k}
	for _, s := range shapes {
		if b.ctx.Err() != nil {
			return
		}
		b.drawShape(s, m, nil, 0)
	}
}

// readLayers records the layers of the page that are hidden.
func (b *builder) readLayers(s *sheet) {
	sec, ok := s.sections["Layer"]
	if !ok {
		return
	}
	for key, r := range sec.rows {
		if v, ok := r.cells["Visible"]; ok && v.v == "0" {
			b.hidden[key] = true
		}
	}
}

// isHidden reports whether each layer of the shape is hidden.
func (b *builder) isHidden(s *shape) bool {
	if len(b.hidden) == 0 {
		return false
	}
	v, _ := b.value(s, "LayerMember")
	if v == "" {
		return false
	}
	for l := range strings.SplitSeq(v, ";") {
		if !b.hidden[strings.TrimSpace(l)] {
			return false
		}
	}
	return true
}

// link returns the master shape of the shape s, and the master of the
// children of s. ctx is the master of the parent of s.
func (b *builder) link(s *shape, ctx *master) (*shape, *master) {
	if ms, ok := b.links[s]; ok {
		if s.masterID != "" {
			return ms, b.dr.master(s.masterID)
		}
		return ms, ctx
	}
	var ms *shape
	if s.masterID != "" {
		ctx = b.dr.master(s.masterID)
		if ctx.err != nil {
			b.dr.cfg.Log().Debug("vsdx: master left out", "master", s.masterID, "err", ctx.err)
		}
		switch {
		case s.masterShapeID != "":
			ms = ctx.byID[s.masterShapeID]
		case len(ctx.shapes) > 0:
			ms = ctx.shapes[0]
		}
	} else if s.masterShapeID != "" && ctx != nil {
		ms = ctx.byID[s.masterShapeID]
	}
	if ms == s {
		ms = nil
	}
	b.links[s] = ms
	return ms, ctx
}

// masterOf returns the master shape of the shape s, or nil.
func (b *builder) masterOf(s *shape) *shape {
	if ms, ok := b.links[s]; ok {
		return ms
	}
	ms, _ := b.link(s, nil)
	return ms
}

// kind returns the type of the shape, such as "Group", from the shape or
// from its master.
func (b *builder) kind(s *shape) string {
	for depth := 0; s != nil && depth <= b.lim.MaxDepth; depth++ {
		if s.kind != "" {
			return s.kind
		}
		s = b.masterOf(s)
	}
	return "Shape"
}

// children returns the shapes of a group: its own, or those of its master
// shape when the group does not list them.
func (b *builder) children(s, ms *shape, depth int) []*shape {
	if s.hasChildren || ms == nil || depth > b.lim.MaxDepth {
		return s.children
	}
	if v, ok := b.virtual[s]; ok {
		return v
	}
	var list []*shape
	for _, c := range b.children(ms, b.masterOf(ms), depth+1) {
		if b.deleted[c] {
			continue
		}
		v := &shape{sheet: sheet{cells: map[string]cell{}, sections: map[string]*section{}}, id: c.id, part: s.part}
		b.links[v] = c
		list = append(list, v)
	}
	b.virtual[s] = list
	return list
}

// chain returns the sheets that the shape inherits the cells of the
// category from, nearest first. A shape inherits from itself, from the
// styles that it sets when they are not the styles of its master, from its
// master shape, and from the styles of its master shape, or from its
// styles when it has no master ([MS-VSDX] 2.2.5.4). A style inherits from
// the style of the same category that it names.
func (b *builder) chain(s *shape, cat int) chain {
	cs := b.chains[s]
	if cs == nil {
		cs = new([4]chain)
		b.chains[s] = cs
	}
	if cs[cat] != nil {
		return cs[cat]
	}
	out := chain{}
	seen := make(map[*sheet]bool)
	add := func(sh *sheet) bool {
		if seen[sh] {
			return false
		}
		seen[sh] = true
		out = append(out, sh)
		return true
	}
	cur := s
	for depth := 0; cur != nil && depth <= b.lim.MaxDepth; depth++ {
		if !add(&cur.sheet) {
			break
		}
		ms := b.masterOf(cur)
		if ms == nil {
			if cat == catOther {
				for c := range catOther {
					b.styleChain(b.effStyle(cur, c), c, add)
				}
			} else {
				b.styleChain(b.effStyle(cur, cat), cat, add)
			}
			break
		}
		if cat != catOther {
			if own := cur.styles[cat]; own != "" && own != b.effStyle(ms, cat) {
				b.styleChain(own, cat, add)
			}
		}
		cur = ms
	}
	cs[cat] = out
	return out
}

// effStyle returns the ID of the style of the category of the shape: its
// own, the style of its master, or the default style of the document.
func (b *builder) effStyle(s *shape, cat int) string {
	for depth := 0; s != nil && depth <= b.lim.MaxDepth; depth++ {
		if s.styles[cat] != "" {
			return s.styles[cat]
		}
		s = b.masterOf(s)
	}
	return b.dr.defaults[cat]
}

// styleChain adds the style id and the styles that it inherits from, for
// the category, to a chain.
func (b *builder) styleChain(id string, cat int, add func(*sheet) bool) {
	for depth := 0; id != "" && depth <= b.lim.MaxDepth; depth++ {
		st, ok := b.dr.styles[id]
		if !ok || !add(st) {
			return
		}
		id = st.styles[cat]
	}
}

// value returns the value of the cell of the shape with the name. A cell
// that the shape inherits, with a trivial formula, takes the result of the
// formula for the size of the shape.
func (b *builder) value(s *shape, name string) (string, bool) {
	key := cellKey{s, name}
	if v, ok := b.values[key]; ok {
		return v.v, v.ok
	}
	c, i, ok := b.chain(s, category(name)).cell(name)
	if !ok {
		b.values[key] = cachedValue{}
		return "", false
	}
	// a formula that names its own cell takes the value of the cell
	b.values[key] = cachedValue{c.v, true}
	if i > 0 && c.f != "" && b.evalDepth < maxEvalDepth {
		b.evalDepth++
		v, ok := eval(c.f, b.lookup(s))
		b.evalDepth--
		if ok {
			b.values[key] = cachedValue{formatNum(v), true}
			return formatNum(v), true
		}
	}
	return c.v, true
}

// cellKey is a cell of a shape, for the cache of the values.
type cellKey struct {
	s    *shape
	name string
}

// cachedValue is the value of a cell in the cache.
type cachedValue struct {
	v  string
	ok bool
}

// lookup returns the function that gives the value of a cell of the shape
// to a formula.
func (b *builder) lookup(s *shape) func(string) (float64, bool) {
	return func(name string) (float64, bool) {
		v, ok := b.value(s, name)
		if !ok {
			return 0, false
		}
		f, err := strconv.ParseFloat(v, 64)
		return f, err == nil
	}
}

// num returns the value of the cell of the shape with the name as a
// number, or def.
func (b *builder) num(s *shape, name string, def float64) float64 {
	v, ok := b.value(s, name)
	if !ok {
		return def
	}
	return parseNum(v, def)
}

// rowNum returns the cell of the row with the name as a number, or 0. A
// cell that the shape s inherits, with a trivial formula, takes the result
// of the formula for s.
func (b *builder) rowNum(s *shape, r *rowView, name string) float64 {
	c, i, ok := r.cell(name)
	if !ok {
		return 0
	}
	if i > 0 && c.f != "" && b.evalDepth < maxEvalDepth {
		b.evalDepth++
		v, ok := eval(c.f, b.lookup(s))
		b.evalDepth--
		if ok {
			return v
		}
	}
	return parseNum(c.v, 0)
}

// drawShape draws the shape s, whose parent maps the coordinates of the
// parent of s onto the page. ctx is the master of the parent of s.
func (b *builder) drawShape(s *shape, parent affine, ctx *master, depth int) {
	if depth > b.lim.MaxDepth {
		b.fault(fmt.Errorf("groups deeper than %d: %w", b.lim.MaxDepth, ivy.ErrLimit))
		return
	}
	if !b.shapes.add(1) {
		if b.shapes.n == b.shapes.max+1 {
			b.fault(fmt.Errorf("page of more than %d shapes: %w", b.lim.MaxObjects, ivy.ErrLimit))
		}
		return
	}
	if b.ctx.Err() != nil {
		return
	}
	ms, childCtx := b.link(s, ctx)
	kind := b.kind(s)
	if kind == "Guide" || b.isHidden(s) {
		return
	}
	w, h := b.num(s, "Width", 0), b.num(s, "Height", 0)
	pinX, pinY := b.num(s, "PinX", 0), b.num(s, "PinY", 0)
	locX, locY := b.num(s, "LocPinX", w/2), b.num(s, "LocPinY", h/2)
	fx, fy := 1.0, 1.0
	if v, _ := b.value(s, "FlipX"); isTrue(v) {
		fx = -1
	}
	if v, _ := b.value(s, "FlipY"); isTrue(v) {
		fy = -1
	}
	local := translate(pinX, pinY).mul(rotate(b.num(s, "Angle", 0))).mul(scale(fx, fy)).mul(translate(-locX, -locY))
	m := parent.mul(local)
	if !m.finite() || math.IsNaN(w) || math.IsNaN(h) {
		return
	}
	for _, c := range s.children {
		// a deleted shape of a group names the shape of the master that the
		// instance leaves out
		if c.del && c.masterShapeID != "" && childCtx != nil {
			if d, ok := childCtx.byID[c.masterShapeID]; ok {
				b.deleted[d] = true
			}
		}
	}
	children := b.children(s, ms, 0)
	if kind != "Group" && len(children) == 0 {
		b.figure(s, m, w, h, true, true)
		return
	}
	if b.num(s, "DisplayMode", 2) != 0 {
		b.figure(s, m, w, h, true, false)
	}
	for _, c := range children {
		if !c.del {
			b.drawShape(c, m, childCtx, depth+1)
		}
	}
	b.figure(s, m, w, h, false, true)
}

// figure adds the figure of the shape to the scene: its picture, its
// geometry with its fill, its line and its shadow, when shapes is true, and
// its text, when text is true. m maps the coordinates of the shape onto the
// page, and w by h is its size.
func (b *builder) figure(s *shape, m affine, w, h float64, shapes, text bool) {
	k := b.k
	// the frame of the figure is the box of the shape in points, with its
	// y axis down
	frame := affine{a: k, d: -k, f: h * k}
	toPage := m.mul(affine{a: 1 / k, d: -1 / k, f: h})
	if !toPage.finite() {
		return
	}
	q := b.quick(s)
	f := &drawingml.Figure{
		Transform: toPage.okage(),
		W:         math.Max(w*k, 0),
		H:         math.Max(h*k, 0),
	}
	if b.theme != nil {
		f.Theme = b.theme.dml
	}
	var lineSubs []subpath
	var l drawingml.Line
	var a arrows
	if shapes {
		if fs, ok := b.foreignOf(s); ok {
			b.addPicture(fs, s, w, h, frame, f.Transform)
		}
		var fillSubs []subpath
		fillSubs, lineSubs = b.geometry(s, w, h)
		f.Fill = b.fill(s, q)
		l, a = b.line(s, q)
		f.Line = l
		if p := path(fillSubs, frame, func(sp *subpath) bool { return sp.closed }); p != nil && f.Fill.Visible() {
			f.Paths = append(f.Paths, drawingml.Path{Path: p, Fill: drawingml.FillNorm})
		}
		if p := path(lineSubs, frame, func(*subpath) bool { return true }); p != nil && l.Visible() {
			f.Paths = append(f.Paths, drawingml.Path{Path: p, Fill: drawingml.FillNone, Stroke: true})
		}
		if len(f.Paths) > 0 {
			f.Shadow = b.shadow(s)
		}
	}
	if text {
		if tb := b.textBody(s, q); tb != nil && !tb.Empty() {
			if t, r, ok := b.textFrame(s, m, w, h); ok {
				f.Text, f.TextTransform, f.TextRect = tb, t, r
			}
		}
	}
	if len(f.Paths) == 0 && f.Text == nil {
		return
	}
	b.scene.AddFigure(f)
	if shapes {
		b.addArrows(lineSubs, l, a, frame, f.Transform)
	}
}

// geometry returns the subpaths of the Geometry sections of the shape that
// draw a fill, and those that draw a line.
func (b *builder) geometry(s *shape, w, h float64) (fill, line []subpath) {
	c := b.chain(s, catOther)
	g := &geometry{b: b, w: w, h: h, extent: (b.pageW + b.pageH) * 4}
	num := func(r *rowView, name string) float64 {
		return b.rowNum(s, r, name)
	}
	formula := func(r *rowView, name string) string {
		v, _, _ := r.cell(name)
		if strings.Contains(v.f, "(") {
			return v.f
		}
		return v.v
	}
	for _, key := range c.sectionKeys("Geometry") {
		flag := func(name string) bool {
			v, _ := c.sectionCell(key, name)
			return isTrue(v.v)
		}
		if flag("NoShow") {
			continue
		}
		g.subs = nil
		g.section(c.rows(key, maxRows), num, formula)
		if r := b.num(s, "Rounding", 0); r > 0 {
			for i := range g.subs {
				g.subs[i] = g.subs[i].round(r)
			}
		}
		if !flag("NoFill") {
			fill = append(fill, g.subs...)
		}
		if !flag("NoLine") {
			line = append(line, g.subs...)
		}
		if g.over {
			b.fault(fmt.Errorf("paths of more than %d segments: %w", b.lim.MaxObjects, ivy.ErrLimit))
			break
		}
	}
	return fill, line
}

// foreignOf returns the shape that holds the ForeignData of s: s or its
// master.
func (b *builder) foreignOf(s *shape) (*shape, bool) {
	for depth := 0; s != nil && depth <= b.lim.MaxDepth; depth++ {
		if s.hasForeign {
			return s, true
		}
		s = b.masterOf(s)
	}
	return nil, false
}

// addPicture adds the picture of the ForeignData of the shape fs to the
// scene, in the rectangle of the cells ImgOffsetX, ImgOffsetY, ImgWidth
// and ImgHeight of the shape s.
func (b *builder) addPicture(fs, s *shape, w, h float64, frame affine, t okage.Transform) {
	var id string
	for c := range elements(fs.foreign) {
		if c.TagName().Name() == "Rel" {
			id = ooxml.Attr(c, "id")
		}
	}
	if id == "" {
		return
	}
	data, err := b.dr.picture(fs.part, id)
	if err != nil {
		b.fault(err)
		return
	}
	pic := drawingml.NewPicture(data, b.dr.cfg)
	ox, oy := b.num(s, "ImgOffsetX", 0), b.num(s, "ImgOffsetY", 0)
	iw, ih := b.num(s, "ImgWidth", w), b.num(s, "ImgHeight", h)
	p0, p1 := frame.apply(pt{ox, oy + ih}), frame.apply(pt{ox + iw, oy})
	r, ok := okage.RectFromLTRB(float32(min(p0.x, p1.x)), float32(min(p0.y, p1.y)), float32(max(p0.x, p1.x)), float32(max(p0.y, p1.y)))
	if !ok {
		return
	}
	b.scene.Add(t, func(ctx context.Context, dev display.Device, scale float64) error {
		if err := pic.Draw(ctx, dev, r, drawingml.Insets{}, 1, scale); err != nil {
			return fmt.Errorf("drawing picture: %w", err)
		}
		return nil
	})
}

// page is a page of a Visio drawing.
type page struct {
	scene *drawingml.Scene
	w, h  float64
	n     int
	// err is the fault of the parts of the page that its scene leaves out.
	err error
}

// The interfaces that a page implements.
var _ ivy.Page = (*page)(nil)

// Size satisfies the ivy.Page interface: the size of the page in points.
func (p *page) Size() (float32, float32) {
	return float32(p.w), float32(p.h)
}

// Render satisfies the ivy.Page interface.
func (p *page) Render(ctx context.Context, dst *okage.Pixmap, t okage.Transform) (err error) {
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("rendering page %d: %w", p.n+1, err)
	}
	defer func() {
		if r := recover(); r != nil {
			err = &ivy.PartialError{Err: fmt.Errorf("rendering page %d: panic: %v: %w", p.n+1, r, ivy.ErrFormat)}
		}
	}()
	dev := display.NewOkage(dst, t)
	if box, ok := okage.RectFromXYWH(0, 0, float32(p.w), float32(p.h)); ok {
		dev.ClipPath(okage.PathFromRect(box), okage.FillRuleWinding)
	}
	err = p.scene.Draw(ctx, dev, t)
	if pe, ok := errors.AsType[*ivy.PartialError](err); ok {
		return &ivy.PartialError{Err: fmt.Errorf("rendering page %d: %w", p.n+1, errors.Join(pe.Err, p.err))}
	}
	if err != nil {
		return fmt.Errorf("rendering page %d: %w", p.n+1, err)
	}
	if p.err != nil {
		return &ivy.PartialError{Err: fmt.Errorf("rendering page %d: %w", p.n+1, p.err)}
	}
	return nil
}

// formatNum formats a number as the value of a cell.
func formatNum(v float64) string {
	return strconv.FormatFloat(v, 'g', -1, 64)
}
