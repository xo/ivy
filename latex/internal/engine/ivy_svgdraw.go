// This file of ivy draws SVG onto the display list (D62): the specials of
// a page, such as the drawings of TikZ, and the figures in SVG. Upstream
// interprets them as operators of PDF in svgpdf.go and svgimage.go. ivy
// parses them with the SVG of okage, and walks the tree of usvg as resvg
// does, so that the paths stay vectors on the display list.
//
// The walk draws the fills and the strokes of the paths with their colors
// and gradients, the opacity and the blend mode of a group, and a clip path
// of paths. It leaves out the masks, the filters, the patterns and the
// images in the SVG.

package engine

import (
	"fmt"
	"strings"

	"github.com/xo/ivy"
	"github.com/xo/ivy/internal/display"
	"github.com/xo/okage"
	"github.com/xo/okage/svg"
)

// svgTree is a parsed SVG drawing.
type svgTree struct {
	tree *svg.Tree
}

// The limits of the SVG of a page.
const (
	maxSVGDepth = 256
	// maxSpecialBytes bounds the SVG of the specials of one page.
	maxSpecialBytes = 64 << 20
)

// parseSpecials parses the literals of the specials of a page, which are
// fragments of SVG in the coordinates of the page, in TeX points. A scope
// that the literals leave open is closed at the end.
func parseSpecials(stream string, pw, ph float64) (*svgTree, error) {
	if len(stream) > maxSpecialBytes {
		return nil, fmt.Errorf("latex: specials of %d bytes: %w", len(stream), ivy.ErrLimit)
	}
	open := strings.Count(stream, "<g") - strings.Count(stream, "</g") - strings.Count(stream, "<g/>")
	var b strings.Builder
	w, h := f(pw), f(ph)
	fmt.Fprintf(&b, `<svg xmlns="http://www.w3.org/2000/svg" xmlns:gotex="urn:gotex" width="%s" height="%s" viewBox="0 0 %s %s">`, w, h, w, h)
	b.WriteString(stream)
	for range max(open, 0) {
		b.WriteString("</g>")
	}
	b.WriteString("</svg>")
	return parseSVG([]byte(b.String()))
}

// parseSVG parses an SVG document.
func parseSVG(data []byte) (t *svgTree, err error) {
	defer func() {
		if r := recover(); r != nil {
			t, err = nil, fmt.Errorf("latex: svg: %v: %w", r, ivy.ErrFormat)
		}
	}()
	tree, err := svg.FromData(data, svg.WithMaxDepth(maxSVGDepth))
	if err != nil {
		return nil, fmt.Errorf("latex: svg: %w: %w", ivy.ErrFormat, err)
	}
	return &svgTree{tree: tree}, nil
}

// draw draws the tree in the user space of d, in the units of the size of
// the tree.
func (t *svgTree) draw(d display.Device) {
	drawSVGGroup(d, t.tree.Root(), 0)
}

// drawSVGImage draws an SVG figure into the rectangle from (0, 0) to (w,
// h) of the user space of d.
func drawSVGImage(d display.Device, data []byte, w, h float64) error {
	t, err := parseSVG(data)
	if err != nil {
		return err
	}
	size := t.tree.Size()
	if !(size.Width() > 0 && size.Height() > 0) {
		return nil
	}
	d.Save()
	defer d.Restore()
	d.Concat(okage.TransformFromScale(float32(w)/size.Width(), float32(h)/size.Height()))
	t.draw(d)
	return nil
}

// drawSVGGroup draws a group and its children, as renderGroup of resvg
// does.
func drawSVGGroup(d display.Device, g *svg.Group, depth int) {
	if depth > maxSVGDepth {
		return
	}
	d.Save()
	defer d.Restore()
	if t := g.Transform(); !t.IsIdentity() {
		d.Concat(t)
	}
	if cp, ok := g.ClipPath(); ok {
		if p := clipPathOf(cp); p != nil {
			d.ClipPath(p, okage.FillRuleWinding)
		}
	}
	grouped := g.Opacity().Get() < 1 || g.BlendMode() != svg.BlendModeNormal
	if grouped {
		d.BeginGroup(display.Group{Opacity: g.Opacity().Get(), Blend: blendMode(g.BlendMode()), Isolated: g.Isolate()})
	}
	for _, n := range g.Children() {
		switch c := n.(type) {
		case *svg.Group:
			drawSVGGroup(d, c, depth+1)
		case *svg.Path:
			drawSVGPath(d, c)
		case *svg.Text:
			if fl := c.Flattened(); fl != nil {
				drawSVGGroup(d, fl, depth+1)
			}
		}
	}
	if grouped {
		d.EndGroup()
	}
}

// clipPathOf returns the paths of a clip path as one path in the space of
// the group that it clips. It returns nil when the clip path holds no path.
func clipPathOf(cp *svg.ClipPath) *okage.Path {
	pb := okage.NewPathBuilder()
	var add func(g *svg.Group, t okage.Transform)
	add = func(g *svg.Group, t okage.Transform) {
		t = t.PreConcat(g.Transform())
		for _, n := range g.Children() {
			switch c := n.(type) {
			case *svg.Group:
				add(c, t)
			case *svg.Path:
				if p, ok := display.Clone(c.Data()).Transform(t); ok {
					pb.PushPath(p)
				}
			}
		}
	}
	add(cp.Root(), cp.Transform())
	p, ok := pb.Finish()
	if !ok {
		return nil
	}
	return p
}

// drawSVGPath fills and strokes a path, in the order of its paint.
func drawSVGPath(d display.Device, p *svg.Path) {
	if !p.IsVisible() {
		return
	}
	if p.PaintOrder() == svg.PaintOrderFillAndStroke {
		fillSVGPath(d, p)
		strokeSVGPath(d, p)
		return
	}
	strokeSVGPath(d, p)
	fillSVGPath(d, p)
}

// fillSVGPath fills a path, as fillPath of resvg does.
func fillSVGPath(d display.Device, p *svg.Path) {
	fill, ok := p.Fill()
	if !ok {
		return
	}
	b := p.Data().Bounds()
	if b.Width() == 0 || b.Height() == 0 {
		return
	}
	shader, ok := svgShader(fill.Paint(), fill.Opacity())
	if !ok {
		return
	}
	rule := okage.FillRuleWinding
	if fill.Rule() == svg.FillRuleEvenOdd {
		rule = okage.FillRuleEvenOdd
	}
	d.FillPath(p.Data(), rule, shader)
}

// strokeSVGPath strokes a path, as strokePath of resvg does.
func strokeSVGPath(d display.Device, p *svg.Path) {
	stroke, ok := p.Stroke()
	if !ok {
		return
	}
	shader, ok := svgShader(stroke.Paint(), stroke.Opacity())
	if !ok {
		return
	}
	st := stroke.ToTinySkia()
	if dash, ok := stroke.Dasharray(); ok {
		if sd, ok := okage.NewStrokeDash(dash, stroke.Dashoffset()); ok {
			st.Dash = sd
		}
	}
	d.StrokePath(p.Data(), st, shader)
}

// svgShader returns the shader of a paint: a color or a gradient. A
// pattern takes the color black, as a degraded paint.
func svgShader(paint svg.Paint, opacity svg.Opacity) (okage.Shader, bool) {
	switch c := paint.(type) {
	case svg.Color:
		return okage.SolidColor(okage.ColorFromRGBA8(c.Red, c.Green, c.Blue, opacity.ToU8())), true
	case *svg.LinearGradient:
		mode, stops := svgStops(&c.BaseGradient, opacity)
		return okage.NewLinearGradient(
			okage.PointFromXY(c.X1(), c.Y1()), okage.PointFromXY(c.X2(), c.Y2()),
			stops, mode, c.Transform())
	case *svg.RadialGradient:
		mode, stops := svgStops(&c.BaseGradient, opacity)
		return okage.NewRadialGradient(
			okage.PointFromXY(c.Fx(), c.Fy()), c.Fr().Get(),
			okage.PointFromXY(c.Cx(), c.Cy()), c.R().Get(),
			stops, mode, c.Transform())
	}
	return okage.SolidColor(okage.ColorFromRGBA8(0, 0, 0, opacity.ToU8())), true
}

// svgStops returns the stops and the spread of a gradient.
func svgStops(g *svg.BaseGradient, opacity svg.Opacity) (okage.SpreadMode, []okage.GradientStop) {
	mode := okage.SpreadModePad
	switch g.SpreadMethod() {
	case svg.SpreadMethodReflect:
		mode = okage.SpreadModeReflect
	case svg.SpreadMethodRepeat:
		mode = okage.SpreadModeRepeat
	}
	stops := make([]okage.GradientStop, 0, len(g.Stops()))
	for _, s := range g.Stops() {
		c := s.Color()
		a := s.Opacity().Mul(opacity)
		stops = append(stops, okage.NewGradientStop(s.Offset().Get(), okage.ColorFromRGBA8(c.Red, c.Green, c.Blue, a.ToU8())))
	}
	return mode, stops
}

// blendMode returns the blend mode of okage of a blend mode of SVG.
func blendMode(b svg.BlendMode) okage.BlendMode {
	switch b {
	case svg.BlendModeMultiply:
		return okage.BlendModeMultiply
	case svg.BlendModeScreen:
		return okage.BlendModeScreen
	case svg.BlendModeOverlay:
		return okage.BlendModeOverlay
	case svg.BlendModeDarken:
		return okage.BlendModeDarken
	case svg.BlendModeLighten:
		return okage.BlendModeLighten
	case svg.BlendModeColorDodge:
		return okage.BlendModeColorDodge
	case svg.BlendModeColorBurn:
		return okage.BlendModeColorBurn
	case svg.BlendModeHardLight:
		return okage.BlendModeHardLight
	case svg.BlendModeSoftLight:
		return okage.BlendModeSoftLight
	case svg.BlendModeDifference:
		return okage.BlendModeDifference
	case svg.BlendModeExclusion:
		return okage.BlendModeExclusion
	case svg.BlendModeHue:
		return okage.BlendModeHue
	case svg.BlendModeSaturation:
		return okage.BlendModeSaturation
	case svg.BlendModeColor:
		return okage.BlendModeColor
	case svg.BlendModeLuminosity:
		return okage.BlendModeLuminosity
	}
	return okage.BlendModeSourceOver
}
