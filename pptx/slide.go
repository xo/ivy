package pptx

import (
	"context"
	"errors"
	"fmt"

	"github.com/xo/okage"
	"github.com/xo/okage/roxmltree"

	"github.com/xo/ivy"
	"github.com/xo/ivy/internal/chart"
	"github.com/xo/ivy/internal/display"
	"github.com/xo/ivy/internal/drawingml"
	"github.com/xo/ivy/internal/fontdb"
	"github.com/xo/ivy/internal/ooxml"
)

// slide is a slide that a page builds: its parts, its theme and its colors.
type slide struct {
	p   *presentation
	ctx context.Context
	ref slideRef
	cfg ivy.Config

	// root, layout and master are the root elements of the parts. layout
	// and master are the zero Node when the slide has none.
	root, layout, master roxmltree.Node
	// pics, layoutPics and masterPics give the pictures of the parts.
	pics, layoutPics, masterPics *partPictures
	colors                       *drawingml.Colors
	scene                        *drawingml.Scene

	// styles holds the list styles of the text, by the kind of
	// placeholder: "title", "body" and "other".
	styles map[string]*drawingml.ListStyle
}

// page builds the scene of the slide n.
func (p *presentation) page(ctx context.Context, n int) (ivy.Page, error) {
	ref := p.slides[n]
	s := &slide{
		p:      p,
		ctx:    ctx,
		ref:    ref,
		cfg:    p.d.cfg,
		scene:  drawingml.NewScene(p.d.cfg),
		styles: make(map[string]*drawingml.ListStyle),
	}
	if err := s.read(); err != nil {
		return nil, fmt.Errorf("reading slide %d: %w", n+1, err)
	}
	s.build()
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("reading slide %d: %w", n+1, err)
	}
	return &page{scene: s.scene, w: p.width, h: p.height, n: n}, nil
}

// read reads the parts of the slide: the slide, its layout, its master and
// its theme. A slide without a layout, a master or a theme draws with what
// it has, and the fault is logged.
func (s *slide) read() error {
	doc, err := s.p.part(s.ref.part)
	if err != nil {
		return err
	}
	s.root = doc.RootElement()
	s.pics = s.p.picturesOf(s.ref.part)
	log := s.cfg.Log()
	var theme *drawingml.Theme
	colorMap := map[string]string(nil)
	if name, ok := s.p.target(s.ref.part, relLayout); ok {
		if doc, err := s.p.part(name); err == nil {
			s.layout = doc.RootElement()
			s.layoutPics = s.p.picturesOf(name)
			if master, ok := s.p.target(name, relMaster); ok {
				if doc, err := s.p.part(master); err == nil {
					s.master = doc.RootElement()
					s.masterPics = s.p.picturesOf(master)
					if e, ok := child(s.master, "clrMap"); ok {
						colorMap = drawingml.ParseColorMap(e)
					}
					if name, ok := s.p.target(master, relTheme); ok {
						if theme, err = s.p.theme(name); err != nil {
							log.Debug("pptx: theme left out", "err", err)
						}
					}
				} else {
					log.Debug("pptx: master left out", "err", err)
				}
			}
		} else {
			log.Debug("pptx: layout left out", "err", err)
		}
	}
	for _, n := range []roxmltree.Node{s.layout, s.root} {
		if !n.IsElement() {
			continue
		}
		ovr, ok := child(n, "clrMapOvr")
		if !ok {
			continue
		}
		if e, ok := child(ovr, "overrideClrMapping"); ok {
			colorMap = drawingml.ParseColorMap(e)
		}
	}
	s.colors = &drawingml.Colors{Theme: theme, Map: colorMap}
	return nil
}

// child returns the first child element of n with the local name.
func child(n roxmltree.Node, name string) (roxmltree.Node, bool) {
	if !n.IsElement() {
		return roxmltree.Node{}, false
	}
	for c := range n.Children() {
		if c.IsElement() && c.TagName().Name() == name {
			return c, true
		}
	}
	return roxmltree.Node{}, false
}

// shapeTree returns the p:cSld/p:spTree of a part.
func shapeTree(root roxmltree.Node) (roxmltree.Node, bool) {
	cSld, ok := child(root, "cSld")
	if !ok {
		return roxmltree.Node{}, false
	}
	return child(cSld, "spTree")
}

// build adds the background and the shapes of the slide to the scene.
func (s *slide) build() {
	s.background()
	showMaster := attrBool(s.root, "showMasterSp", true)
	if showMaster && attrBool(s.layout, "showMasterSp", true) {
		if tree, ok := shapeTree(s.master); ok {
			s.tree(s.masterPics, true).Draw(tree, okage.IdentityTransform())
		}
	}
	if showMaster {
		if tree, ok := shapeTree(s.layout); ok {
			s.tree(s.layoutPics, true).Draw(tree, okage.IdentityTransform())
		}
	}
	if tree, ok := shapeTree(s.root); ok {
		s.tree(s.pics, false).Draw(tree, okage.IdentityTransform())
	}
}

// attrBool returns the boolean attribute of n with the name, or def.
func attrBool(n roxmltree.Node, name string, def bool) bool {
	if !n.IsElement() {
		return def
	}
	switch ooxml.Attr(n, name) {
	case "0", "false":
		return false
	case "1", "true":
		return true
	}
	return def
}

// tree returns the drawer of a shape tree of the slide, of its layout or of
// its master.
func (s *slide) tree(pics *partPictures, master bool) *drawingml.Tree {
	t := &drawingml.Tree{
		Ctx:              s.ctx,
		Config:           s.cfg,
		Scene:            s.scene,
		Colors:           s.colors,
		TextStyle:        s.textStyle,
		TableStyles:      s.p.tableStyle,
		SkipPlaceholders: master,
		Fields:           drawingml.Fields{SlideNumber: s.ref.number},
		Chart: func(frame roxmltree.Node, m okage.Transform, w, h float64) bool {
			return s.drawChart(frame, pics, m, w, h)
		},
	}
	if pics != nil {
		t.Pictures = pics
		t.Diagram = func(frame roxmltree.Node) (roxmltree.Node, drawingml.Pictures, bool) {
			return s.diagram(frame, pics)
		}
	}
	if !master {
		t.Inherit = s.inherit
	}
	return t
}

// background adds the background of the slide: the first p:bg of the
// slide, of its layout and of its master.
func (s *slide) background() {
	for _, part := range []struct {
		root roxmltree.Node
		pics *partPictures
	}{{s.root, s.pics}, {s.layout, s.layoutPics}, {s.master, s.masterPics}} {
		cSld, ok := child(part.root, "cSld")
		if !ok {
			continue
		}
		bg, ok := child(cSld, "bg")
		if !ok {
			continue
		}
		var pics drawingml.Pictures
		if part.pics != nil {
			pics = part.pics
		}
		pt, ok := s.backgroundPaint(bg, pics)
		if !ok {
			continue
		}
		box, ok := okage.RectFromXYWH(0, 0, float32(s.p.width), float32(s.p.height))
		if !ok {
			return
		}
		s.scene.AddFigure(&drawingml.Figure{
			Transform: okage.IdentityTransform(),
			W:         s.p.width,
			H:         s.p.height,
			Paths:     []drawingml.Path{{Path: okage.PathFromRect(box)}},
			Fill:      pt,
			Line:      drawingml.Line{Paint: drawingml.Paint{Kind: drawingml.PaintNone}},
		})
		return
	}
}

// backgroundPaint returns the paint of p:bg: its p:bgPr, or the style of
// the theme that its p:bgRef names.
func (s *slide) backgroundPaint(bg roxmltree.Node, pics drawingml.Pictures) (drawingml.Paint, bool) {
	if pr, ok := child(bg, "bgPr"); ok {
		return s.colors.PaintIn(pr, pics)
	}
	ref, ok := child(bg, "bgRef")
	if !ok {
		return drawingml.Paint{}, false
	}
	idx, _ := atoi(ooxml.Attr(ref, "idx"))
	colors := s.colors
	if v, ok := s.colors.In(ref); ok {
		colors = s.colors.WithPlaceholder(v)
	}
	if e, ok := s.colors.Theme.FillStyle(idx); ok {
		return colors.PaintOf(e, pics)
	}
	// a reference without a style of the theme is the color of the
	// reference
	if v, ok := s.colors.In(ref); ok {
		return drawingml.Solid(v), true
	}
	return drawingml.Paint{}, false
}

// styleKind returns the kind of the list style of the text of a
// placeholder: "title", "body" or "other".
func styleKind(ph *drawingml.Placeholder) string {
	if ph == nil {
		return "other"
	}
	switch ph.Type {
	case "title", "ctrTitle":
		return "title"
	case "dt", "ftr", "sldNum", "hdr":
		return "other"
	}
	return "body"
}

// textStyle returns the list style of the text of a placeholder, or of a
// shape that is not a placeholder: the default text style of the
// presentation, with the text style of the master over it.
func (s *slide) textStyle(ph *drawingml.Placeholder) *drawingml.ListStyle {
	kind := styleKind(ph)
	if st, ok := s.styles[kind]; ok {
		return st
	}
	var st drawingml.ListStyle
	if s.p.hasDefaults {
		st.Over(s.colors.ParseListStyle(s.p.defaults))
	}
	if tx, ok := child(s.master, "txStyles"); ok {
		if e, ok := child(tx, kind+"Style"); ok {
			st.Over(s.colors.ParseListStyle(e))
		}
	}
	s.styles[kind] = &st
	return &st
}

// masterType returns the type of the placeholder of the master that a
// placeholder of the type inherits from.
func masterType(typ string) string {
	switch typ {
	case "ctrTitle", "title":
		return "title"
	case "dt", "ftr", "sldNum", "hdr":
		return typ
	}
	return "body"
}

// inherit returns the placeholders of the layout and of the master that a
// placeholder of the slide inherits from.
func (s *slide) inherit(ph drawingml.Placeholder) []drawingml.Inherited {
	var out []drawingml.Inherited
	typ := ph.Type
	if tree, ok := shapeTree(s.layout); ok {
		if n, lph, ok := findPlaceholder(tree, ph, true); ok {
			out = append(out, drawingml.Inherited{Node: n, Pictures: s.layoutPics})
			typ = lph.Type
		}
	}
	if tree, ok := shapeTree(s.master); ok {
		if n, _, ok := findPlaceholder(tree, drawingml.Placeholder{Type: typ}, false); ok {
			out = append(out, drawingml.Inherited{Node: n, Pictures: s.masterPics})
		}
	}
	return out
}

// findPlaceholder returns the shape of the tree with the placeholder that
// matches ph: the same index when byIdx is set and ph has one, else the
// same type, else the same type of the master.
func findPlaceholder(tree roxmltree.Node, ph drawingml.Placeholder, byIdx bool) (roxmltree.Node, drawingml.Placeholder, bool) {
	type candidate struct {
		n  roxmltree.Node
		ph drawingml.Placeholder
	}
	var list []candidate
	for e := range tree.Children() {
		if !e.IsElement() {
			continue
		}
		if p, ok := placeholderOf(e); ok {
			list = append(list, candidate{e, p})
		}
	}
	if byIdx && ph.HasIdx {
		for _, c := range list {
			if c.ph.HasIdx && c.ph.Idx == ph.Idx {
				return c.n, c.ph, true
			}
		}
	}
	for _, c := range list {
		if c.ph.Type == ph.Type {
			return c.n, c.ph, true
		}
	}
	for _, c := range list {
		if masterType(c.ph.Type) == masterType(ph.Type) {
			return c.n, c.ph, true
		}
	}
	return roxmltree.Node{}, drawingml.Placeholder{}, false
}

// placeholderOf returns the placeholder of a shape, from its p:nvPr.
func placeholderOf(n roxmltree.Node) (drawingml.Placeholder, bool) {
	for e := range n.Children() {
		if !e.IsElement() {
			continue
		}
		nvPr, ok := child(e, "nvPr")
		if !ok {
			continue
		}
		ph, ok := child(nvPr, "ph")
		if !ok {
			return drawingml.Placeholder{}, false
		}
		p := drawingml.Placeholder{Type: ooxml.Attr(ph, "type")}
		if p.Type == "" {
			p.Type = "obj"
		}
		if v, ok := atoi(ooxml.Attr(ph, "idx")); ok {
			p.Idx, p.HasIdx = v, true
		}
		return p, true
	}
	return drawingml.Placeholder{}, false
}

// drawChart draws the chart of a graphic frame of the part of pics, in the
// box of w by h points of the space m. internal/chart draws it with zuzu
// (D24), with the theme and the fonts of the slide. It returns false for a
// chart that cannot be read, which the tree logs and leaves out.
func (s *slide) drawChart(frame roxmltree.Node, pics *partPictures, m okage.Transform, w, h float64) bool {
	if pics == nil {
		return false
	}
	id := ""
	for e := range frame.Descendants() {
		if e.IsElement() && e.TagName().Name() == "chart" {
			id = ooxml.Attr(e, "id")
			break
		}
	}
	r, ok := pics.rels[id]
	if id == "" || !ok || r.External {
		return false
	}
	rect, ok := okage.RectFromXYWH(0, 0, float32(w), float32(h))
	if !ok {
		return false
	}
	buf, err := s.p.d.doc.ReadPart(r.Target)
	var ch *chart.Chart
	if err == nil {
		ch, err = chart.Parse(buf, chart.Options{
			Theme:  s.colors.Theme,
			Config: s.cfg,
			Fonts:  fontdb.Of(s.cfg),
			Width:  w,
			Height: h,
		})
	}
	if err != nil {
		s.cfg.Log().Debug("pptx: chart left out", "part", r.Target, "err", err)
		return false
	}
	s.scene.Add(m, func(ctx context.Context, dev display.Device, _ float64) error {
		return ch.Draw(ctx, dev, rect)
	})
	return true
}

// diagram returns the shape tree of the drawing of a diagram of SmartArt:
// the part dsp:drawing that the extension dsp:dataModelExt of its data part
// names.
func (s *slide) diagram(frame roxmltree.Node, pics *partPictures) (roxmltree.Node, drawingml.Pictures, bool) {
	var dm string
	for e := range frame.Descendants() {
		if e.IsElement() && e.TagName().Name() == "relIds" {
			for a := range e.Attributes() {
				if a.Name() == "dm" {
					dm = a.Value()
				}
			}
			break
		}
	}
	r, ok := pics.rels[dm]
	if dm == "" || !ok || r.External {
		return roxmltree.Node{}, nil, false
	}
	data, err := s.p.part(r.Target)
	if err != nil {
		return roxmltree.Node{}, nil, false
	}
	var drawingID string
	for e := range data.RootElement().Descendants() {
		if e.IsElement() && e.TagName().Name() == "dataModelExt" {
			drawingID = ooxml.Attr(e, "relId")
			break
		}
	}
	d, ok := pics.rels[drawingID]
	if drawingID == "" || !ok || d.External {
		return roxmltree.Node{}, nil, false
	}
	drawing, err := s.p.part(d.Target)
	if err != nil {
		return roxmltree.Node{}, nil, false
	}
	for e := range drawing.RootElement().Children() {
		if e.IsElement() && e.TagName().Name() == "spTree" {
			return e, s.p.picturesOf(d.Target), true
		}
	}
	return roxmltree.Node{}, nil, false
}

// page is a slide on a page.
type page struct {
	scene *drawingml.Scene
	w, h  float64
	n     int
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
