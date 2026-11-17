package xps

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/xo/ivy"
	"github.com/xo/ivy/internal/display"
	"github.com/xo/ivy/internal/font"
	"github.com/xo/okage"
	"github.com/xo/okage/roxmltree"
)

// renderer walks one FixedPage, or the visual of a VisualBrush, and sends
// its marks to a device.
type renderer struct {
	ctx  context.Context
	doc  *Document
	part string
	dev  display.Device
	// ctm is the transform from the user space to the pixels of the device,
	// and stack holds the transforms that save pushed.
	ctm   okage.Transform
	stack []okage.Transform
	// area is the target of the device, in its pixels.
	area okage.Rect
	// visuals is the number of VisualBrush elements that the walk is inside.
	visuals int
	// w is the state that the renderers of the visuals of a page share.
	w *walk
}

// walk is the state of the walk of one page, which the renderers of its
// visuals share.
type walk struct {
	// err is the first error of an element that the walk left out.
	err   error
	count int
	// pixels is the number of pixels that the visuals and the tiles of the
	// brushes drew.
	pixels int64
}

// fail keeps the first error of an element that the renderer leaves out, and
// logs each one.
func (r *renderer) fail(err error) {
	if err == nil {
		return
	}
	r.doc.log.Debug("xps: element left out", "page", r.part, "err", err)
	if r.w.err == nil {
		r.w.err = err
	}
}

// save pushes the state of the device and the current transform.
func (r *renderer) save() {
	r.dev.Save()
	r.stack = append(r.stack, r.ctm)
}

// restore pops the state that the last save pushed.
func (r *renderer) restore() {
	r.dev.Restore()
	if n := len(r.stack); n > 0 {
		r.ctm = r.stack[n-1]
		r.stack = r.stack[:n-1]
	}
}

// concat multiplies the current transform by t, so that t applies first.
func (r *renderer) concat(t okage.Transform) {
	r.dev.Concat(t)
	r.ctm = r.ctm.PreConcat(t)
}

// resources is a resource dictionary, with the dictionary of the parent
// element.
type resources struct {
	m      map[string]roxmltree.Node
	parent *resources
}

// static returns the resource that the value {StaticResource key} names. It
// returns false if s is not such a value, and an error if the key is unknown.
func (res *resources) static(s string) (roxmltree.Node, bool, error) {
	s = strings.TrimSpace(s)
	inner, ok := strings.CutPrefix(s, "{StaticResource")
	if !ok {
		return roxmltree.Node{}, false, nil
	}
	key := strings.TrimSpace(strings.TrimSuffix(inner, "}"))
	for d := res; d != nil; d = d.parent {
		if n, ok := d.m[key]; ok {
			return n, true, nil
		}
	}
	return roxmltree.Node{}, true, fmt.Errorf("static resource %q: %w", key, ivy.ErrFormat)
}

// resourcesOf returns the resources of the element n: its property element
// <n.Resources>, inline or in a remote dictionary, over the resources of its
// parent.
func (r *renderer) resourcesOf(n roxmltree.Node, parent *resources) *resources {
	dict, ok := property(n, "Resources")
	if !ok {
		return parent
	}
	res := &resources{m: make(map[string]roxmltree.Node), parent: parent}
	if src := attr(dict, "Source"); src != "" {
		remote, err := r.doc.pkg.xml(resolve(r.part, src))
		if err != nil {
			r.fail(err)
			return res
		}
		dict = remote.RootElement()
	}
	for _, c := range elements(dict) {
		if key := attr(c, "Key"); key != "" {
			res.m[key] = c
		}
	}
	return res
}

// page walks the FixedPage element root.
func (r *renderer) page(root roxmltree.Node) {
	r.children(root, r.resourcesOf(root, nil), 0)
}

// children walks the child elements of n that are not property elements.
func (r *renderer) children(n roxmltree.Node, res *resources, depth int) {
	for _, c := range elements(n) {
		if strings.Contains(local(c), ".") {
			continue
		}
		if !r.element(c, res, depth+1) {
			return
		}
	}
}

// element walks one element. It returns false when the walk must stop: the
// context is done, or a limit is reached.
func (r *renderer) element(n roxmltree.Node, res *resources, depth int) bool {
	r.w.count++
	switch {
	case r.ctx.Err() != nil:
		r.fail(r.ctx.Err())
		return false
	case r.w.count > r.doc.lim.MaxObjects:
		r.fail(fmt.Errorf("page of more than %d elements: %w", r.doc.lim.MaxObjects, ivy.ErrLimit))
		return false
	case depth > r.doc.lim.MaxDepth:
		r.fail(fmt.Errorf("canvases nested deeper than %d: %w", r.doc.lim.MaxDepth, ivy.ErrLimit))
		return true
	}
	switch local(n) {
	case "Canvas":
		r.canvas(n, res, depth)
	case "Path":
		r.path(n, res)
	case "Glyphs":
		r.glyphs(n, res)
	}
	return true
}

// begin saves the state, and applies the RenderTransform, the Clip and the
// OpacityMask of n. An opacity below 1, or an OpacityMask, draws n as a
// group. The caller calls the function that begin returns, to end the group
// and restore the state.
func (r *renderer) begin(n roxmltree.Node, res *resources, opacity float32) func() {
	r.save()
	t, err := r.transformOf(n, "RenderTransform", res)
	r.fail(err)
	r.concat(t)
	if g, ok, err := r.geometryOf(n, "Clip", res); err != nil {
		r.fail(err)
	} else if ok {
		r.dev.ClipPath(g.path, g.rule)
	}
	mask := r.opacityMask(n, res)
	if mask == nil && opacity >= 1 {
		return r.restore
	}
	r.dev.BeginGroup(display.Group{Opacity: opacity, Mask: mask})
	return func() {
		r.dev.EndGroup()
		r.restore()
	}
}

// opacityMask returns the alpha mask of the OpacityMask of n, or nil if n
// has none. The brush of the mask fills the part of the user space that the
// target shows. A mask applies only where n draws, so the result is the same
// as a fill of the bounds of n.
func (r *renderer) opacityMask(n roxmltree.Node, res *resources) *display.Mask {
	if _, ok := property(n, "OpacityMask"); !ok && !hasAttr(n, "OpacityMask") {
		return nil
	}
	b, err := r.brushOf(n, "OpacityMask", res, 1)
	if err != nil {
		// the element draws without its mask
		r.fail(fmt.Errorf("opacity mask: %w", err))
		return nil
	}
	ctm := r.ctm
	var area *okage.Path
	if inv, ok := ctm.Invert(); ok {
		if rect, ok := r.area.Transform(inv); ok {
			area = okage.PathFromRect(rect)
		}
	}
	return &display.Mask{
		Type: okage.MaskTypeAlpha,
		Content: func(d display.Device) {
			// a brush that draws nothing hides the element
			if b != nil && area != nil {
				b.paint(d, ctm, area, okage.FillRuleWinding)
			}
		},
	}
}

// opacityOf returns the Opacity of n.
func opacityOf(n roxmltree.Node) float32 {
	return clamp(number(attr(n, "Opacity"), 1))
}

// canvas walks a Canvas. A canvas with an opacity draws as a group.
func (r *renderer) canvas(n roxmltree.Node, res *resources, depth int) {
	res = r.resourcesOf(n, res)
	end := r.begin(n, res, opacityOf(n))
	defer end()
	r.children(n, res, depth)
}

// path draws a Path: its fill, then its stroke. The Opacity of the path
// multiplies the alpha of its brushes.
func (r *renderer) path(n roxmltree.Node, res *resources) {
	g, ok, err := r.geometryOf(n, "Data", res)
	if err != nil || !ok || g.path == nil {
		r.fail(err)
		return
	}
	end := r.begin(n, res, 1)
	defer end()
	op := opacityOf(n)
	fill, err := r.brushOf(n, "Fill", res, op)
	r.fail(err)
	if fill != nil {
		fill.paint(r.dev, r.ctm, g.path, g.rule)
	}
	stroke, err := r.brushOf(n, "Stroke", res, op)
	r.fail(err)
	if stroke != nil {
		r.stroke(g.path, strokeOf(n), stroke)
	}
}

// stroke strokes a path with a brush. A brush that is not a shader fills the
// outline of the stroke.
func (r *renderer) stroke(p *okage.Path, s okage.Stroke, b *brush) {
	if b.draw == nil {
		r.dev.StrokePath(p, s, b.shader)
		return
	}
	scale := okage.ComputeResolutionScale(r.ctm)
	if s.Dash != nil {
		dashed, ok := p.Dash(s.Dash, scale)
		if !ok {
			return
		}
		p = dashed
	}
	if outline, ok := p.Stroke(&s, scale); ok {
		b.paint(r.dev, r.ctm, outline, okage.FillRuleWinding)
	}
}

// strokeOf returns the stroke of a Path.
func strokeOf(n roxmltree.Node) okage.Stroke {
	s := okage.DefaultStroke()
	s.Width = number(attr(n, "StrokeThickness"), 1)
	s.MiterLimit = max(number(attr(n, "StrokeMiterLimit"), 10), 1)
	switch attr(n, "StrokeStartLineCap") {
	case "Round":
		s.LineCap = okage.LineCapRound
	case "Square", "Triangle":
		s.LineCap = okage.LineCapSquare
	}
	switch attr(n, "StrokeLineJoin") {
	case "Round":
		s.LineJoin = okage.LineJoinRound
	case "Bevel":
		s.LineJoin = okage.LineJoinBevel
	}
	if dashes, err := numbers(attr(n, "StrokeDashArray")); err == nil && len(dashes) > 0 {
		for i := range dashes {
			dashes[i] *= s.Width
		}
		if len(dashes)%2 == 1 {
			dashes = append(dashes, dashes...)
		}
		offset := number(attr(n, "StrokeDashOffset"), 0) * s.Width
		if dash, ok := okage.NewStrokeDash(dashes, offset); ok {
			s.Dash = dash
		}
	}
	return s
}

// transformOf returns the transform of the property name of n, from its
// attribute, a matrix or a static resource, or from its property element.
// It returns the identity if n has none.
func (r *renderer) transformOf(n roxmltree.Node, name string, res *resources) (okage.Transform, error) {
	var matrix string
	switch {
	case hasAttr(n, name):
		matrix = attr(n, name)
		if node, ok, err := res.static(matrix); ok || err != nil {
			if err != nil {
				return okage.IdentityTransform(), err
			}
			matrix = attr(node, "Matrix")
		}
	default:
		node, ok := property(n, name)
		if !ok {
			return okage.IdentityTransform(), nil
		}
		matrix = attr(node, "Matrix")
	}
	t, err := parseMatrix(matrix)
	if err != nil {
		return okage.IdentityTransform(), err
	}
	return t, nil
}

// geometryOf returns the geometry of the property name of n, from its
// attribute, the abbreviated syntax or a static resource, or from its
// property element. It returns false if n has none.
func (r *renderer) geometryOf(n roxmltree.Node, name string, res *resources) (geometry, bool, error) {
	if hasAttr(n, name) {
		s := attr(n, name)
		if node, ok, err := res.static(s); ok || err != nil {
			if err != nil {
				return geometry{}, false, err
			}
			g, err := r.pathGeometry(node, res)
			return g, err == nil, err
		}
		g, err := parseAbbreviated(s)
		return g, err == nil, err
	}
	node, ok := property(n, name)
	if !ok {
		return geometry{}, false, nil
	}
	g, err := r.pathGeometry(node, res)
	return g, err == nil, err
}

// pathGeometry reads a PathGeometry element: its Figures in the abbreviated
// syntax, its PathFigure elements, its FillRule and its Transform.
func (r *renderer) pathGeometry(n roxmltree.Node, res *resources) (geometry, error) {
	if local(n) != "PathGeometry" {
		return geometry{}, fmt.Errorf("geometry %s: %w", local(n), ivy.ErrFormat)
	}
	g := geometry{rule: okage.FillRuleEvenOdd}
	if attr(n, "FillRule") == "NonZero" {
		g.rule = okage.FillRuleWinding
	}
	b := newPathBuilder()
	if figures := attr(n, "Figures"); figures != "" {
		fg, err := parseAbbreviated(figures)
		if err != nil {
			return g, err
		}
		if fg.path != nil {
			b.pb.PushPath(fg.path)
		}
	}
	for _, fig := range elements(n) {
		if local(fig) != "PathFigure" {
			continue
		}
		if err := figure(b, fig); err != nil {
			return g, err
		}
	}
	g.path = b.finish()
	t, err := r.transformOf(n, "Transform", res)
	if err != nil {
		return g, err
	}
	if g.path != nil && !t.IsIdentity() {
		// the path is new, so it can change in place
		if p, ok := g.path.Transform(t); ok {
			g.path = p
		}
	}
	return g, nil
}

// figure adds a PathFigure to b.
func figure(b *pathBuilder, n roxmltree.Node) error {
	start, err := points(attr(n, "StartPoint"))
	if err != nil || len(start) != 1 {
		return fmt.Errorf("path figure start point: %w", ivy.ErrFormat)
	}
	b.moveTo(start[0][0], start[0][1])
	for _, seg := range elements(n) {
		pts, err := points(attr(seg, "Points"))
		switch local(seg) {
		case "PolyLineSegment":
			if err != nil {
				return err
			}
			for _, p := range pts {
				b.lineTo(p[0], p[1])
			}
		case "PolyBezierSegment":
			if err != nil || len(pts)%3 != 0 {
				return fmt.Errorf("bezier points: %w", ivy.ErrFormat)
			}
			for i := 0; i < len(pts); i += 3 {
				b.cubicTo(pts[i][0], pts[i][1], pts[i+1][0], pts[i+1][1], pts[i+2][0], pts[i+2][1])
			}
		case "PolyQuadraticBezierSegment":
			if err != nil || len(pts)%2 != 0 {
				return fmt.Errorf("quadratic points: %w", ivy.ErrFormat)
			}
			for i := 0; i < len(pts); i += 2 {
				b.quadTo(pts[i][0], pts[i][1], pts[i+1][0], pts[i+1][1])
			}
		case "ArcSegment":
			end, err := points(attr(seg, "Point"))
			size, err2 := points(attr(seg, "Size"))
			if err != nil || err2 != nil || len(end) != 1 || len(size) != 1 {
				return fmt.Errorf("arc segment: %w", ivy.ErrFormat)
			}
			large := attr(seg, "IsLargeArc") == "true"
			sweep := attr(seg, "SweepDirection") == "Clockwise"
			b.arcTo(size[0][0], size[0][1], number(attr(seg, "RotationAngle"), 0), large, sweep, end[0][0], end[0][1])
		}
	}
	if attr(n, "IsClosed") == "true" {
		b.close()
	}
	return nil
}

// glyphs draws a Glyphs element.
func (r *renderer) glyphs(n roxmltree.Node, res *resources) {
	run, err := r.glyphRun(n)
	if err != nil {
		r.fail(err)
		return
	}
	if len(run.Glyphs) == 0 {
		return
	}
	end := r.begin(n, res, 1)
	defer end()
	fill, err := r.brushOf(n, "Fill", res, opacityOf(n))
	if err != nil || fill == nil {
		r.fail(err)
		return
	}
	if fill.draw == nil {
		r.dev.FillGlyphs(run, fill.shader)
	} else if p := run.Path(); p != nil {
		fill.paint(r.dev, r.ctm, p, okage.FillRuleWinding)
	}
	if strings.Contains(attr(n, "StyleSimulations"), "Bold") {
		// a bold simulation widens each glyph by 2 percent of an em
		if p := run.Path(); p != nil {
			stroke := okage.DefaultStroke()
			stroke.Width = run.Size * 0.02
			r.stroke(p, stroke, fill)
		}
	}
}

// glyphRun reads the font, the size, the origin and the glyphs of a Glyphs
// element. The Indices give a glyph, an advance in hundredths of an em, and
// offsets for each glyph. A glyph that the Indices do not give comes from the
// UnicodeString and the map of the font.
func (r *renderer) glyphRun(n roxmltree.Node) (display.GlyphRun, error) {
	face, err := r.doc.font(resolve(r.part, attr(n, "FontUri")))
	if err != nil {
		return display.GlyphRun{}, err
	}
	size := number(attr(n, "FontRenderingEmSize"), 0)
	if size <= 0 {
		return display.GlyphRun{}, nil
	}
	run := display.GlyphRun{Face: face, Size: size, Transform: okage.IdentityTransform()}
	if strings.Contains(attr(n, "StyleSimulations"), "Italic") {
		// an italic simulation slants each glyph by 20 degrees
		run.Transform = okage.TransformFromRow(1, 0, -0.36397, 1, 0, 0)
	}
	text := []rune(strings.TrimPrefix(attr(n, "UnicodeString"), "{}"))
	rtl := number(attr(n, "BidiLevel"), 0)
	direction := float32(1)
	if int(rtl)%2 == 1 {
		direction = -1
	}
	x, y := number(attr(n, "OriginX"), 0), number(attr(n, "OriginY"), 0)
	var entries []string
	if indices := strings.TrimSpace(attr(n, "Indices")); indices != "" {
		entries = strings.Split(indices, ";")
	}
	// pos is the next character of the text, and left is the number of
	// glyphs of the current cluster that are still to come
	pos, left := 0, 0
	for i := 0; i < len(entries) || pos < len(text); i++ {
		if i >= r.doc.lim.MaxObjects {
			return run, fmt.Errorf("glyph run of more than %d glyphs: %w", r.doc.lim.MaxObjects, ivy.ErrLimit)
		}
		var entry string
		if i < len(entries) {
			entry = entries[i]
		}
		chars, glyphs, rest := cluster(entry)
		fields := strings.Split(rest, ",")
		var gid font.GID
		var haveGID bool
		if v, err := strconv.ParseUint(strings.TrimSpace(fields[0]), 10, 32); err == nil {
			gid, haveGID = font.GID(v), true
		} else if pos < len(text) {
			gid, haveGID = face.Glyph(text[pos])
		}
		// the first glyph of a cluster takes its characters, and the other
		// glyphs of the cluster take none
		if left == 0 {
			left = glyphs
			pos += chars
		}
		left--
		advance := face.Advance(gid) * size
		if len(fields) > 1 && strings.TrimSpace(fields[1]) != "" {
			advance = number(fields[1], 0) / 100 * size
		}
		var u, v float32
		if len(fields) > 2 {
			u = number(fields[2], 0) / 100 * size
		}
		if len(fields) > 3 {
			v = number(fields[3], 0) / 100 * size
		}
		gx := x + direction*u
		if direction < 0 {
			gx -= advance
		}
		if haveGID {
			run.Glyphs = append(run.Glyphs, display.Glyph{ID: gid, X: gx, Y: y - v})
		}
		x += direction * advance
	}
	return run, nil
}

// cluster reads the cluster map "(chars:glyphs)" at the start of an entry of
// the Indices, and returns the number of characters and of glyphs, and the
// rest of the entry. An entry without a map maps one character to one glyph.
func cluster(entry string) (int, int, string) {
	entry = strings.TrimSpace(entry)
	inner, ok := strings.CutPrefix(entry, "(")
	if !ok {
		return 1, 1, entry
	}
	m, rest, ok := strings.Cut(inner, ")")
	if !ok {
		return 1, 1, entry
	}
	cs, gs, _ := strings.Cut(m, ":")
	chars, err := strconv.Atoi(strings.TrimSpace(cs))
	if err != nil || chars < 0 {
		chars = 1
	}
	glyphs, err := strconv.Atoi(strings.TrimSpace(gs))
	if err != nil || glyphs < 1 {
		glyphs = 1
	}
	return chars, glyphs, rest
}
