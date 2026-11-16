package drawingml

import (
	"context"
	"log/slog"

	"github.com/xo/okage"
	"github.com/xo/okage/roxmltree"

	"github.com/xo/ivy"
)

// emuPerPoint is the number of EMU in a point.
const emuPerPoint = 12700

// Placeholder is the placeholder of a shape of PresentationML, from p:ph.
type Placeholder struct {
	// Type is the type, such as "title" or "body". A placeholder without a
	// type is "obj".
	Type string
	// Idx is the index, which matches the placeholder of the layout.
	Idx    int
	HasIdx bool
}

// Inherited is a shape that a placeholder inherits from, such as the
// placeholder of the same type on the layout or on the master.
type Inherited struct {
	// Node is the shape, such as p:sp.
	Node roxmltree.Node
	// Pictures gives the pictures of the part of the shape.
	Pictures Pictures
}

// Tree draws the shapes of a shape tree of DrawingML onto a scene, such as
// the p:spTree of a slide. The format sets the fields, and calls Draw.
type Tree struct {
	// Ctx stops the work.
	Ctx context.Context
	// Config gives the limits and the logger.
	Config ivy.Config
	// Scene receives the figures.
	Scene *Scene
	// Colors resolves the colors.
	Colors *Colors
	// Pictures gives the pictures of the part of the tree.
	Pictures Pictures
	// Inherit returns the shapes that a placeholder inherits from, the
	// nearest first, or is nil.
	Inherit func(ph Placeholder) []Inherited
	// TextStyle returns the list style of the text of a placeholder, or of
	// a shape that is not a placeholder for a nil ph, or is nil.
	TextStyle func(ph *Placeholder) *ListStyle
	// TableStyles gives the table styles of the package, or is nil.
	TableStyles TableStyles
	// SkipPlaceholders leaves out the placeholders, as on a master or a
	// layout.
	SkipPlaceholders bool
	// Fields gives the text of the fields, such as the slide number.
	Fields Fields
	// Chart draws the chart of a graphic frame, and returns false if it
	// drew nothing. It is nil when the format draws no chart.
	Chart func(frame roxmltree.Node, m okage.Transform, w, h float64) bool
	// Diagram returns the shape tree of the drawing of a diagram of
	// SmartArt in a graphic frame, and the pictures of its part. It is nil
	// when the format draws no diagram.
	Diagram func(frame roxmltree.Node) (roxmltree.Node, Pictures, bool)

	// shapes counts the shapes that the tree drew, within the limits.
	shapes int
}

// log returns the logger of the tree.
func (t *Tree) log() *slog.Logger {
	return t.Config.Log()
}

// Draw draws the children of the shape tree n, such as p:spTree, with the
// transform m from the space of the shapes onto the space of the scene.
func (t *Tree) Draw(n roxmltree.Node, m okage.Transform) {
	t.group(n, rootSpace(m), Paint{Kind: PaintNone}, 0, t.Pictures)
}

// space is the space of the children of a shape tree or of a group. The
// boxes of the children map onto the box of the group, so a box changes
// its size and its place, but the width of its outline and the size of its
// text do not scale with the group, as in PowerPoint.
type space struct {
	// m maps the space of the box of the group onto the space of the
	// scene. It holds no scale of the group.
	m okage.Transform
	// sx and sy scale the boxes of the children, and ox and oy are the
	// origin of the space of the children.
	sx, sy, ox, oy float64
}

// rootSpace returns the space of a shape tree with the transform m onto the
// space of the scene.
func rootSpace(m okage.Transform) space {
	return space{m: m, sx: 1, sy: 1}
}

// place returns the box of the transform x in the box of the group.
func (s space) place(x Xfrm) Xfrm {
	if s.sx == 1 && s.sy == 1 && s.ox == 0 && s.oy == 0 {
		return x
	}
	// the box scales before it turns, as in LibreOffice
	cx := (x.X + x.W/2 - s.ox) * s.sx
	cy := (x.Y + x.H/2 - s.oy) * s.sy
	x.W, x.H = x.W*s.sx, x.H*s.sy
	x.X, x.Y = cx-x.W/2, cy-x.H/2
	return x
}

// group draws the children of a shape tree or of a group.
func (t *Tree) group(n roxmltree.Node, m space, groupFill Paint, depth int, pics Pictures) {
	lim := t.Config.Limits.WithDefaults()
	if depth > lim.MaxDepth || depth > 256 {
		t.log().Warn("drawingml: groups nested too deep", "max", lim.MaxDepth, "err", ivy.ErrLimit)
		return
	}
	for e := range n.Children() {
		if !e.IsElement() {
			continue
		}
		if t.Ctx.Err() != nil {
			return
		}
		if t.shapes >= lim.MaxObjects {
			t.log().Warn("drawingml: too many shapes", "max", lim.MaxObjects, "err", ivy.ErrLimit)
			return
		}
		t.shapes++
		switch e.TagName().Name() {
		case "sp":
			t.shape(e, m, groupFill, pics, true)
		case "cxnSp":
			t.shape(e, m, groupFill, pics, false)
		case "pic":
			t.picture(e, m, pics)
		case "grpSp":
			t.groupShape(e, m, groupFill, depth, pics)
		case "graphicFrame":
			t.frame(e, m, pics)
		case "AlternateContent":
			// the fallback holds the shapes of the first edition of
			// DrawingML, which the package reads. A choice that holds a
			// chart of Office 2016 (chartex) goes first, as the chart draws
			// better than the picture of its fallback.
			drawn := false
			if ch, ok := t.chartexChoice(e); ok {
				// a chart that cannot be read draws nothing, and the
				// fallback draws instead
				n := t.Scene.Len()
				t.group(ch, m, groupFill, depth+1, pics)
				drawn = t.Scene.Len() > n
			}
			if drawn {
				continue
			}
			if fb, ok := child(e, "Fallback"); ok {
				t.group(fb, m, groupFill, depth+1, pics)
			} else if ch, ok := child(e, "Choice"); ok {
				t.group(ch, m, groupFill, depth+1, pics)
			}
		case "nvGrpSpPr", "grpSpPr", "extLst":
		default:
			t.log().Debug("drawingml: shape left out", "element", e.TagName().Name())
		}
	}
}

// chartexChoice returns the mc:Choice of the mc:AlternateContent n that
// holds a chart of Office 2016, when the tree draws charts.
func (t *Tree) chartexChoice(n roxmltree.Node) (roxmltree.Node, bool) {
	if t.Chart == nil {
		return roxmltree.Node{}, false
	}
	for c := range n.Children() {
		if !c.IsElement() || c.TagName().Name() != "Choice" {
			continue
		}
		for d := range c.Descendants() {
			if d.IsElement() && d.TagName().Name() == "graphicData" && hasSuffix(attr(d, "uri", ""), "/chartex") {
				return c, true
			}
		}
	}
	return roxmltree.Node{}, false
}

// Xfrm is the transform of a shape, from a:xfrm: its box, its rotation and
// its flips, in points. A group also has the box of its children.
type Xfrm struct {
	X, Y, W, H   float64
	Rot          float64
	FlipH, FlipV bool
	// ChX, ChY, ChW and ChH are the box of the children of a group.
	ChX, ChY, ChW, ChH float64
	OK                 bool
}

// ParseXfrm reads a:xfrm or p:xfrm.
func ParseXfrm(n roxmltree.Node) Xfrm {
	x := Xfrm{OK: true, Rot: numberAttr(n, "rot", 0) / 60000}
	x.FlipH, _ = boolAttr(n, "flipH")
	x.FlipV, _ = boolAttr(n, "flipV")
	for e := range n.Children() {
		if !e.IsElement() {
			continue
		}
		switch e.TagName().Name() {
		case "off":
			x.X, x.Y = coord(e, "x"), coord(e, "y")
		case "ext":
			x.W, x.H = max(coord(e, "cx"), 0), max(coord(e, "cy"), 0)
		case "chOff":
			x.ChX, x.ChY = coord(e, "x"), coord(e, "y")
		case "chExt":
			x.ChW, x.ChH = max(coord(e, "cx"), 0), max(coord(e, "cy"), 0)
		}
	}
	return x
}

// coord returns the attribute of n with the name as a coordinate in EMU,
// converted to points.
func coord(n roxmltree.Node, name string) float64 {
	v := numberAttr(n, name, 0)
	return min(max(emu(v), -1e7), 1e7)
}

// Transform returns the transform of the box of the shape onto the space of
// its parent.
func (x *Xfrm) Transform() okage.Transform {
	return ShapeTransform(x.X, x.Y, x.W, x.H, x.Rot, x.FlipH, x.FlipV)
}

// textTransform returns the transform of the text of the shape onto the
// space of its parent.
func (x *Xfrm) textTransform() okage.Transform {
	return TextTransform(x.X, x.Y, x.W, x.H, x.Rot, x.FlipV)
}

// groupShape draws p:grpSp.
func (t *Tree) groupShape(n roxmltree.Node, m space, groupFill Paint, depth int, pics Pictures) {
	g, ok := child(n, "grpSpPr")
	if !ok {
		t.group(n, m, groupFill, depth+1, pics)
		return
	}
	if pt, ok := t.Colors.PaintIn(g, pics); ok && pt.Kind != PaintGroup {
		groupFill = pt
	}
	x := Xfrm{}
	if e, ok := child(g, "xfrm"); ok {
		x = ParseXfrm(e)
	}
	gm := m
	if x.OK {
		// the children are in the box of the children, which maps onto the
		// box of the group
		x = m.place(x)
		gm = space{m: m.m.PreConcat(x.Transform()), sx: 1, sy: 1, ox: x.ChX, oy: x.ChY}
		if x.ChW > 0 {
			gm.sx = x.W / x.ChW
		}
		if x.ChH > 0 {
			gm.sy = x.H / x.ChH
		}
	}
	t.group(n, gm, groupFill, depth+1, pics)
}

// placeholder returns the placeholder of a shape, from p:nvSpPr/p:nvPr/p:ph
// or the same for a picture.
func placeholder(n roxmltree.Node) (*Placeholder, bool) {
	for e := range n.Children() {
		if !e.IsElement() || len(e.TagName().Name()) < 3 || e.TagName().Name()[:2] != "nv" {
			continue
		}
		nvPr, ok := child(e, "nvPr")
		if !ok {
			continue
		}
		ph, ok := child(nvPr, "ph")
		if !ok {
			return nil, false
		}
		p := &Placeholder{Type: attr(ph, "type", "obj")}
		if v, ok := number(attr(ph, "idx", "")); ok {
			p.Idx, p.HasIdx = int(min(max(v, -1), 1<<31-1)), true
		}
		return p, true
	}
	return nil, false
}

// shapeProps is the p:spPr of a shape and of the shapes that it inherits
// from, the nearest first.
type shapeProps struct {
	own   roxmltree.Node
	hasPr bool
	chain []Inherited
	// style is the p:style of the shape, or of the nearest shape that it
	// inherits from that has one.
	style    roxmltree.Node
	hasStyle bool
	// ownStyle reports whether style is the p:style of the shape itself.
	// It then comes before the p:spPr of the shapes that it inherits from.
	ownStyle bool
}

// props returns the properties of the shape n.
func (t *Tree) props(n roxmltree.Node, ph *Placeholder) *shapeProps {
	sp := &shapeProps{}
	sp.own, sp.hasPr = child(n, "spPr")
	if ph != nil && t.Inherit != nil {
		sp.chain = t.Inherit(*ph)
	}
	sp.style, sp.hasStyle = child(n, "style")
	sp.ownStyle = sp.hasStyle
	for _, in := range sp.chain {
		if sp.hasStyle {
			break
		}
		sp.style, sp.hasStyle = child(in.Node, "style")
	}
	return sp
}

// find returns the first child with the name of the p:spPr of the shape and
// of the shapes that it inherits from, and the pictures of its part.
func (sp *shapeProps) find(name string, pics Pictures) (roxmltree.Node, Pictures, bool) {
	if sp.hasPr {
		if e, ok := child(sp.own, name); ok {
			return e, pics, true
		}
	}
	for _, in := range sp.chain {
		if pr, ok := child(in.Node, "spPr"); ok {
			if e, ok := child(pr, name); ok {
				return e, in.Pictures, true
			}
		}
	}
	return roxmltree.Node{}, nil, false
}

// geometry returns the paths of the geometry of the shape for its box, in
// points, and its text rectangle.
func (t *Tree) geometry(sp *shapeProps, w, h float64, pics Pictures) ([]Path, okage.Rect) {
	g, _ := Preset("rect")
	var adjust []Guide
	if e, _, ok := sp.find("prstGeom", pics); ok {
		if p, ok := Preset(attr(e, "prst", "rect")); ok {
			g = p
		} else {
			t.log().Debug("drawingml: unknown preset shape", "prst", attr(e, "prst", ""))
		}
		if av, ok := child(e, "avLst"); ok {
			adjust, _ = ParseAdjust(av)
		}
	} else if e, _, ok := sp.find("custGeom", pics); ok {
		c, err := ParseCustom(e)
		if err != nil {
			t.log().Debug("drawingml: custom geometry left out", "err", err)
		} else {
			g = c
		}
	}
	return Evaluated(g, adjust, w, h, t.log())
}

// Evaluated evaluates the geometry g with the adjust values for a box of w
// by h points, and returns its paths and its text rectangle in points. The
// guides are computed in EMU, the unit of the geometry. It logs an error
// and returns the box for a geometry that cannot be evaluated.
func Evaluated(g *Geometry, adjust []Guide, w, h float64, log *slog.Logger) ([]Path, okage.Rect) {
	box, _ := okage.RectFromXYWH(0, 0, float32(max(w, 1e-3)), float32(max(h, 1e-3)))
	s, err := g.Evaluate(w*emuPerPoint, h*emuPerPoint, adjust)
	if err != nil {
		log.Debug("drawingml: geometry left out", "err", err)
		return []Path{{Path: okage.PathFromRect(box), Stroke: true}}, box
	}
	scale := okage.TransformFromScale(1.0/emuPerPoint, 1.0/emuPerPoint)
	paths := make([]Path, 0, len(s.Paths))
	for _, p := range s.Paths {
		q, ok := p.Path.Transform(scale)
		if !ok {
			continue
		}
		p.Path = q
		paths = append(paths, p)
	}
	text := box
	if r, ok := okage.RectFromLTRB(s.Text.Left()/emuPerPoint, s.Text.Top()/emuPerPoint,
		s.Text.Right()/emuPerPoint, s.Text.Bottom()/emuPerPoint); ok {
		text = r
	}
	return paths, text
}

// styleRef returns the index and the color of a reference of p:style, such
// as a:fillRef.
func (t *Tree) styleRef(sp *shapeProps, name string) (int, *Colors, bool) {
	if !sp.hasStyle {
		return 0, nil, false
	}
	ref, ok := child(sp.style, name)
	if !ok {
		return 0, nil, false
	}
	idx := int(min(max(numberAttr(ref, "idx", 0), 0), 2000))
	colors := t.Colors
	if v, ok := t.Colors.In(ref); ok {
		colors = t.Colors.WithPlaceholder(v)
	}
	return idx, colors, true
}

// fill returns the fill of the shape.
func (t *Tree) fill(sp *shapeProps, groupFill Paint, pics Pictures) Paint {
	if sp.hasPr {
		if pt, ok := t.Colors.PaintIn(sp.own, pics); ok {
			return t.groupPaint(pt, groupFill)
		}
	}
	if sp.ownStyle {
		if pt, ok := t.styleFill(sp); ok {
			return pt
		}
	}
	for _, in := range sp.chain {
		if pr, ok := child(in.Node, "spPr"); ok {
			if pt, ok := t.Colors.PaintIn(pr, in.Pictures); ok {
				return t.groupPaint(pt, groupFill)
			}
		}
	}
	if !sp.ownStyle {
		if pt, ok := t.styleFill(sp); ok {
			return pt
		}
	}
	return Paint{Kind: PaintNone}
}

// styleFill returns the fill of the a:fillRef of the p:style of the shape.
// A picture of the fill is in the theme part.
func (t *Tree) styleFill(sp *shapeProps) (Paint, bool) {
	if idx, colors, ok := t.styleRef(sp, "fillRef"); ok && idx > 0 {
		if e, ok := t.Colors.Theme.FillStyle(idx); ok {
			return colors.PaintOf(e, t.Colors.Theme.StylePictures())
		}
	}
	return Paint{}, false
}

// groupPaint returns the fill of the group for a:grpFill.
func (t *Tree) groupPaint(pt, groupFill Paint) Paint {
	if pt.Kind == PaintGroup {
		return groupFill
	}
	return pt
}

// line returns the outline of the shape.
func (t *Tree) line(sp *shapeProps, pics Pictures) Line {
	l := DefaultLine()
	style := func() {
		if idx, colors, ok := t.styleRef(sp, "lnRef"); ok && idx > 0 {
			if e, ok := t.Colors.Theme.LineStyle(idx); ok {
				l = colors.ParseLine(e, l, t.Colors.Theme.StylePictures())
			}
		}
	}
	if !sp.ownStyle {
		style()
	}
	for i := len(sp.chain) - 1; i >= 0; i-- {
		if pr, ok := child(sp.chain[i].Node, "spPr"); ok {
			if e, ok := child(pr, "ln"); ok {
				l = t.Colors.ParseLine(e, l, sp.chain[i].Pictures)
			}
		}
	}
	if sp.ownStyle {
		style()
	}
	if sp.hasPr {
		if e, ok := child(sp.own, "ln"); ok {
			l = t.Colors.ParseLine(e, l, pics)
		}
	}
	return l
}

// shadow returns the shadow of the shape.
func (t *Tree) shadow(sp *shapeProps, pics Pictures) *Shadow {
	own := sp.hasPr
	if own {
		_, own = child(sp.own, "effectLst")
	}
	if !own && sp.ownStyle {
		if s, ok := t.styleShadow(sp); ok {
			return s
		}
	}
	if e, _, ok := sp.find("effectLst", pics); ok {
		if names := Effects(e); len(names) > 0 {
			t.log().Debug("drawingml: effects left out", "effects", names)
		}
		s, _ := t.Colors.ParseEffects(e)
		return s
	}
	s, _ := t.styleShadow(sp)
	return s
}

// styleShadow returns the shadow of the a:effectRef of the p:style of the
// shape.
func (t *Tree) styleShadow(sp *shapeProps) (*Shadow, bool) {
	if idx, colors, ok := t.styleRef(sp, "effectRef"); ok && idx > 0 {
		if e, ok := t.Colors.Theme.EffectStyle(idx); ok {
			return colors.ParseEffects(e)
		}
	}
	return nil, false
}

// xfrm returns the transform of the shape: its own, or that of the nearest
// shape that it inherits from.
func (sp *shapeProps) xfrm(pics Pictures) Xfrm {
	if e, _, ok := sp.find("xfrm", pics); ok {
		return ParseXfrm(e)
	}
	return Xfrm{}
}

// shape draws p:sp, or p:cxnSp for a connector without text.
func (t *Tree) shape(n roxmltree.Node, gs space, groupFill Paint, pics Pictures, text bool) {
	ph, isPh := placeholder(n)
	if isPh && t.SkipPlaceholders {
		return
	}
	sp := t.props(n, ph)
	x := sp.xfrm(pics)
	if !x.OK {
		return
	}
	x = gs.place(x)
	m := gs.m
	paths, rect := t.geometry(sp, x.W, x.H, pics)
	f := &Figure{
		Transform:     m.PreConcat(x.Transform()),
		W:             x.W,
		H:             x.H,
		Paths:         paths,
		Fill:          t.fill(sp, groupFill, pics),
		Line:          t.line(sp, pics),
		Shadow:        t.shadow(sp, pics),
		TextRect:      rect,
		TextTransform: m.PreConcat(x.textTransform()),
		Theme:         t.Colors.Theme,
	}
	if b, ok := boolAttr(n, "useBgFill"); ok && b {
		if v, ok := t.Colors.scheme("bg1"); ok {
			f.Fill = Solid(v)
		}
	}
	if text {
		f.Text = t.text(n, sp, ph)
	}
	t.Scene.AddFigure(f)
}

// text returns the text body of the shape, with the styles that it
// inherits.
func (t *Tree) text(n roxmltree.Node, sp *shapeProps, ph *Placeholder) *TextBody {
	body, ok := child(n, "txBody")
	if !ok {
		return nil
	}
	var list ListStyle
	if t.TextStyle != nil {
		if s := t.TextStyle(ph); s != nil {
			list = *s
		}
	}
	var bs BodyStyle
	for i := len(sp.chain) - 1; i >= 0; i-- {
		tx, ok := child(sp.chain[i].Node, "txBody")
		if !ok {
			continue
		}
		if e, ok := child(tx, "lstStyle"); ok {
			list.Over(t.Colors.ParseListStyle(e))
		}
		if e, ok := child(tx, "bodyPr"); ok {
			b := ParseBodyStyle(e)
			bs.Over(&b)
		}
	}
	if e, ok := child(body, "bodyPr"); ok {
		b := ParseBodyStyle(e)
		bs.Over(&b)
	}
	var run RunStyle
	if sp.hasStyle {
		if ref, ok := child(sp.style, "fontRef"); ok {
			switch attr(ref, "idx", "") {
			case "major":
				run.Latin.Set("+mj-lt")
			case "minor":
				run.Latin.Set("+mn-lt")
			}
			if v, ok := t.Colors.In(ref); ok {
				run.Paint.Set(Solid(v))
			}
		}
	}
	return t.Colors.ParseTextBody(body, &list, &run, bs, t.Fields)
}

// picture draws p:pic.
func (t *Tree) picture(n roxmltree.Node, gs space, pics Pictures) {
	ph, isPh := placeholder(n)
	if isPh && t.SkipPlaceholders {
		return
	}
	sp := t.props(n, ph)
	x := sp.xfrm(pics)
	if !x.OK {
		return
	}
	x = gs.place(x)
	m := gs.m
	paths, _ := t.geometry(sp, x.W, x.H, pics)
	f := &Figure{
		Transform: m.PreConcat(x.Transform()),
		W:         x.W,
		H:         x.H,
		Paths:     paths,
		Line:      t.line(sp, pics),
		Shadow:    t.shadow(sp, pics),
	}
	if e, ok := child(n, "blipFill"); ok {
		f.Fill, _ = t.Colors.PaintOf(e, pics)
		if f.Fill.Kind == PaintPicture && f.Fill.Picture == nil {
			t.log().Debug("drawingml: picture not in the package")
		}
	}
	t.Scene.AddFigure(f)
}

// frame draws p:graphicFrame: a table, a chart, a diagram or an object.
func (t *Tree) frame(n roxmltree.Node, gs space, pics Pictures) {
	ph, isPh := placeholder(n)
	if isPh && t.SkipPlaceholders {
		return
	}
	x := Xfrm{}
	if e, ok := child(n, "xfrm"); ok {
		x = ParseXfrm(e)
	} else if ph != nil && t.Inherit != nil {
		for _, in := range t.Inherit(*ph) {
			if pr, ok := child(in.Node, "spPr"); ok {
				if e, ok := child(pr, "xfrm"); ok {
					x = ParseXfrm(e)
					break
				}
			}
		}
	}
	if !x.OK {
		return
	}
	x = gs.place(x)
	m := gs.m
	g, ok := child(n, "graphic")
	if !ok {
		return
	}
	data, ok := child(g, "graphicData")
	if !ok {
		return
	}
	fm := m.PreConcat(x.Transform())
	uri := attr(data, "uri", "")
	switch {
	case hasSuffix(uri, "/table"):
		if tbl, ok := child(data, "tbl"); ok {
			t.table(tbl, fm, x.W, x.H, pics)
		}
	case hasSuffix(uri, "/chart"), hasSuffix(uri, "/chartex"):
		if t.Chart == nil || !t.Chart(n, fm, x.W, x.H) {
			t.log().Debug("drawingml: chart left out", "uri", uri)
		}
	case hasSuffix(uri, "/diagram"):
		if t.Diagram == nil {
			t.log().Debug("drawingml: diagram left out")
			return
		}
		tree, dpics, ok := t.Diagram(n)
		if !ok {
			t.log().Debug("drawingml: diagram without a drawing left out")
			return
		}
		// the shapes of the drawing are in the space of the frame
		t.group(tree, rootSpace(m.PreTranslate(float32(x.X), float32(x.Y))), Paint{Kind: PaintNone}, 1, dpics)
	default:
		// an object, such as OLE, can hold a picture of its content
		found := false
		for e := range data.Descendants() {
			if e.IsElement() && e.TagName().Name() == "pic" {
				t.picture(e, gs, pics)
				found = true
				break
			}
		}
		if !found {
			t.log().Debug("drawingml: graphic frame left out", "uri", uri)
		}
	}
}

// hasSuffix reports whether s ends with suffix.
func hasSuffix(s, suffix string) bool {
	return len(s) >= len(suffix) && s[len(s)-len(suffix):] == suffix
}
